package journey

import (
	"log"
	"sort"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// HandleAbout sends the bot description in the user's locale.
func (r *Runner) HandleAbout(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	locale := r.localeForUser(user)
	text := r.trans.T("about", locale, nil)
	if _, err := s.Send(tgbotapi.NewMessage(msg.Chat.ID, text)); err != nil {
		log.Printf("journey: about send: %v", err)
	}
}

// HandleReport prints a per-user breakdown of completed / in-progress /
// not_tolerated / interrupted products, plus the stored screening summary
// line when a completed ADHD self-check exists.
func (r *Runner) HandleReport(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	send(s, msg.Chat.ID, reportText(r.trans, r.localeForUser(user), user))
}

// reportText assembles the full /report breakdown. A free function so the
// landing's report button can render the same text through a phase Outcome.
func reportText(trans i18n.Translator, locale i18n.Locale, user state.UserData) string {
	var extraLines []string
	if line := screeningReportLine(trans, locale, user); line != "" {
		extraLines = append(extraLines, line)
	}
	if line := moodReportLine(trans, locale, user); line != "" {
		extraLines = append(extraLines, line)
	}
	if line := gad7ReportLine(trans, locale, user); line != "" {
		extraLines = append(extraLines, line)
	}
	if line := who5ReportLine(trans, locale, user); line != "" {
		extraLines = append(extraLines, line)
	}
	extraLines = append(extraLines, eatingReportLines(trans, locale, user)...)

	if len(user.Products) == 0 {
		if len(extraLines) == 0 {
			return trans.T("cmd.report.empty", locale, nil)
		}
		return trans.T("cmd.report.heading", locale, nil) + "\n" +
			strings.Join(extraLines, "\n")
	}

	var completed, notTolerated, interrupted []string
	for _, name := range sortedKeys(user.Products) {
		prog := user.Products[name]
		switch prog.Status {
		case "completed":
			completed = append(completed, name)
		case "not_tolerated":
			notTolerated = append(notTolerated, name)
		case "interrupted":
			interrupted = append(interrupted, name)
		}
	}

	var lines []string
	lines = append(lines, trans.T("cmd.report.heading", locale, nil))
	if user.CurrentProduct != "" {
		lines = append(lines, trans.T("cmd.report.in_progress", locale, map[string]any{
			"name":  user.CurrentProduct,
			"stage": user.CurrentStage,
		}))
	}
	if len(completed) > 0 {
		lines = append(lines, trans.T("cmd.report.completed", locale, map[string]any{
			"names": strings.Join(completed, ", "),
		}))
	}
	if len(notTolerated) > 0 {
		lines = append(lines, trans.T("cmd.report.not_tolerated", locale, map[string]any{
			"names": strings.Join(notTolerated, ", "),
		}))
	}
	if len(interrupted) > 0 {
		lines = append(lines, trans.T("cmd.report.interrupted", locale, map[string]any{
			"names": strings.Join(interrupted, ", "),
		}))
	}
	lines = append(lines, extraLines...)
	return strings.Join(lines, "\n")
}

// screeningReportLine renders the /report line for the last completed ADHD
// self-check: per-instrument scores with their applied thresholds — never a
// combined score.
func screeningReportLine(trans i18n.Translator, locale i18n.Locale, user state.UserData) string {
	res := user.ScreeningResult
	if res == nil {
		return ""
	}
	verdictWord := func(positive bool) string {
		if positive {
			return trans.T("scr.report.positive", locale, nil)
		}
		return trans.T("scr.report.negative", locale, nil)
	}
	return trans.T("cmd.report.screening", locale, map[string]any{
		"date":      res.TakenAt.Format(reportDateLayout),
		"asrs_a":    res.AsrsASignificant,
		"a_thr":     res.AsrsAThreshold,
		"a_verdict": verdictWord(res.AsrsAPositive),
		"asrs_b":    res.AsrsBSignificant,
		"wurs":      res.WursScore,
		"w_thr":     res.WursCutoff,
		"w_verdict": verdictWord(res.WursPositive),
		"overall":   trans.T("scr.report.overall."+res.Verdict, locale, nil),
	})
}

// moodReportLine renders the /report line for the last completed PHQ-9
// self-check: the date and the 0–27 score — lean by design.
func moodReportLine(trans i18n.Translator, locale i18n.Locale, user state.UserData) string {
	res := user.MoodResult
	if res == nil {
		return ""
	}
	return trans.T("cmd.report.mood", locale, map[string]any{
		"date":  res.TakenAt.Format(reportDateLayout),
		"score": res.Score,
	})
}

// gad7ReportLine / who5ReportLine — the same lean per-instrument lines for
// the other two mood-module instruments.
func gad7ReportLine(trans i18n.Translator, locale i18n.Locale, user state.UserData) string {
	res := user.Gad7Result
	if res == nil {
		return ""
	}
	return trans.T("cmd.report.gad7", locale, map[string]any{
		"date":  res.TakenAt.Format(reportDateLayout),
		"score": res.Score,
	})
}

func who5ReportLine(trans i18n.Translator, locale i18n.Locale, user state.UserData) string {
	res := user.Who5Result
	if res == nil {
		return ""
	}
	return trans.T("cmd.report.who5", locale, map[string]any{
		"date":  res.TakenAt.Format(reportDateLayout),
		"score": res.Score,
	})
}

// eatingReportLines renders the /report lines of the eating track: one lean
// line per completed instrument. NIAS prints its three subscales — it has no
// total by design.
func eatingReportLines(trans i18n.Translator, locale i18n.Locale, user state.UserData) []string {
	var out []string
	if res := user.EdeqsResult; res != nil {
		out = append(out, trans.T("cmd.report.edeqs", locale, map[string]any{
			"date":  res.TakenAt.Format(reportDateLayout),
			"score": res.Score,
		}))
	}
	if res := user.BesResult; res != nil {
		out = append(out, trans.T("cmd.report.bes", locale, map[string]any{
			"date":  res.TakenAt.Format(reportDateLayout),
			"score": res.Score,
		}))
	}
	if res := user.NiasResult; res != nil {
		out = append(out, trans.T("cmd.report.nias", locale, map[string]any{
			"date":     res.TakenAt.Format(reportDateLayout),
			"picky":    res.Picky,
			"appetite": res.Appetite,
			"fear":     res.Fear,
		}))
	}
	return out
}

// screeningContent fetches the content bundle from the registered consent
// phase; nil when the screening mode is not wired. Keeps journey.New
// unchanged — phases receive the content via their constructors.
func (r *Runner) screeningContent() *screening.Content {
	if p, ok := r.phases[state.StateScrConsent].(*ScrConsentPhase); ok {
		return p.c
	}
	return nil
}

// moodContent fetches the mood bundle from the registered mood consent
// phase; nil when the mood mode is not wired. Same pattern as
// screeningContent — phases receive the content via their constructors.
func (r *Runner) moodContent() *screening.MoodContent {
	if p, ok := r.phases[state.StateMoodConsent].(*MoodConsentPhase); ok {
		return p.c
	}
	return nil
}

// eatingContent fetches the eating bundle from the registered eating consent
// phase; nil when the eating track is not wired. Same pattern as
// screeningContent / moodContent.
func (r *Runner) eatingContent() *screening.EatingContent {
	if p, ok := r.phases[state.StateEatConsent].(*EatConsentPhase); ok {
		return p.c
	}
	return nil
}

// enterScreeningMode is the shared direct-entry routine of the screening
// commands (/adhd, /mood, /food). Mid-mode it re-fires the current phase's Setup
// (redraws the question and keyboard); from a FODMAP state it records the
// detour and routes to the mode's entry gate (computed from the user's
// data: the ADHD consent, or the mood consent/menu).
func (r *Runner) enterScreeningMode(msg *tgbotapi.Message, entryFor func(state.UserData) state.StateKind, inMode func(state.StateKind) bool) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	entry := entryFor(user)
	if _, ok := r.phases[entry]; !ok {
		return // mode not wired
	}
	user.ChatID = msg.Chat.ID
	if user.Locale == "" {
		user.Locale = string(r.detectLocale(msg.From.LanguageCode))
	}

	if inMode(user.State) {
		r.store.Set(msg.From.ID, user)
		if phase, ok := r.phases[user.State]; ok {
			ctx := r.contextFor(msg.From.ID, user)
			r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
		}
		return
	}

	if isFodmapJourneyState(user.State) {
		user.ReturnState = user.State
	}
	user.State = entry
	user.EnteredAt = r.now()
	r.store.Set(msg.From.ID, user)

	phase := r.phases[entry]
	ctx := r.contextFor(msg.From.ID, user)
	r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
}

// HandleAdhd is the direct entry into the ADHD self-check.
func (r *Runner) HandleAdhd(s router.Sender, msg *tgbotapi.Message) {
	r.enterScreeningMode(msg,
		func(state.UserData) state.StateKind { return state.StateScrConsent },
		isScreeningState)
}

// HandleMood is the direct entry into the mood module (WHO-5 / PHQ-9 /
// GAD-7): the consent gate for fresh users, the module menu afterwards.
func (r *Runner) HandleMood(s router.Sender, msg *tgbotapi.Message) {
	r.enterScreeningMode(msg, moodEntryState, isMoodState)
}

// HandleFood is the direct entry into the eating track (EDE-QS / BES /
// NIAS): the consent gate for fresh users, the track menu afterwards.
func (r *Runner) HandleFood(s router.Sender, msg *tgbotapi.Message) {
	r.enterScreeningMode(msg, eatEntryState, isEatingState)
}

// enterDeleteConfirm is the shared delete-command routine (/adhd_delete,
// /mood_delete, /food_delete): with nothing stored it answers immediately and does not
// change state, otherwise it records the way back — the interrupted FODMAP
// question or the landing itself to ReturnState, a mid-test position to the
// run's ResumeState (so cancelling returns to the exact question) — and
// routes to the mode's confirmation phase. The landing case matters because
// test exits park the user there: closing the dialog must not eject them
// into a stale diary detour.
func (r *Runner) enterDeleteConfirm(s router.Sender, msg *tgbotapi.Message,
	confirm state.StateKind, hasData func(state.UserData) bool, nothingText string) {
	user := r.store.Get(msg.From.ID)

	if !hasData(user) {
		send(s, msg.Chat.ID, nothingText)
		return
	}

	switch {
	case isFodmapJourneyState(user.State), user.State == state.StateAwaitingModeChoice:
		user.ReturnState = user.State
	case user.Screening != nil && resumableScrState(user.State):
		sc := user.Screening.Clone()
		sc.ResumeState = user.State
		user.Screening = sc
	case user.Mood != nil && resumableMoodState(user.State):
		mc := user.Mood.Clone()
		mc.ResumeState = user.State
		user.Mood = mc
	case user.Who5 != nil && user.State == state.StateMoodWho5Question:
		mc := user.Who5.Clone()
		mc.ResumeState = user.State
		user.Who5 = mc
	case user.Gad7 != nil && user.State == state.StateMoodGad7Question:
		mc := user.Gad7.Clone()
		mc.ResumeState = user.State
		user.Gad7 = mc
	case resumableEatState(user.State) && eatRunFor(user, user.State) != nil:
		ec := eatRunFor(user, user.State).Clone()
		ec.ResumeState = user.State
		setEatRun(&user, user.State, ec)
	}
	user.ChatID = msg.Chat.ID
	user.State = confirm
	r.store.Set(msg.From.ID, user)

	if phase, ok := r.phases[confirm]; ok {
		ctx := r.contextFor(msg.From.ID, user)
		r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
	}
}

// HandleAdhdDelete starts the delete-confirmation flow for all stored
// ADHD self-check data.
func (r *Runner) HandleAdhdDelete(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	c := r.screeningContent()
	if c == nil {
		return // screening mode not wired
	}
	r.enterDeleteConfirm(s, msg, state.StateScrDeleteConfirm,
		func(u state.UserData) bool { return u.Screening != nil || u.ScreeningResult != nil },
		c.Module.UI.DeleteNothing)
}

// HandleMoodDelete starts the delete-confirmation flow for all stored
// mood-module data (all three instruments plus the module consent).
func (r *Runner) HandleMoodDelete(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	c := r.moodContent()
	if c == nil {
		return // mood mode not wired
	}
	r.enterDeleteConfirm(s, msg, state.StateMoodDeleteConfirm,
		hasMoodConsent, // any stored mood data, incl. the consent timestamp
		c.Module.UI.DeleteNothing)
}

// HandleFoodDelete starts the delete-confirmation flow for all stored
// eating-track data (all three instruments plus the track consent).
func (r *Runner) HandleFoodDelete(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	c := r.eatingContent()
	if c == nil {
		return // eating track not wired
	}
	r.enterDeleteConfirm(s, msg, state.StateEatDeleteConfirm,
		hasEatConsent, // any stored eating data, incl. the consent timestamp
		c.Module.UI.DeleteNothing)
}

// HandleAbandon aborts the current activity. Mid-screening it wipes the
// transient raw answers (the previous completed ScreeningResult is kept) and
// lands the user on the home landing — a recorded FODMAP detour stays
// reachable there via the diary button. Otherwise it marks the active trial
// as interrupted and re-routes to the product selection phase, exactly as
// before.
func (r *Runner) HandleAbandon(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	locale := r.localeForUser(user)

	if isScreeningState(user.State) || isMoodState(user.State) || isEatingState(user.State) {
		// Abandon the active self-check: wipe the transient raw answers
		// (the previous completed result is kept) and land home. The
		// FODMAP detour in ReturnState is kept for the landing's diary
		// button (see testExitState).
		var confirmText string
		if isEatingState(user.State) {
			// Only the run the current state belongs to is wiped — a paused
			// run of another instrument stays resumable.
			switch user.State {
			case state.StateEatEdeqsQuestion:
				user.Edeqs = nil
			case state.StateEatBesQuestion:
				user.Bes = nil
			case state.StateEatNiasQuestion:
				user.Nias = nil
			}
			if c := r.eatingContent(); c != nil {
				confirmText = c.Module.UI.AbandonConfirmed
			}
		} else if isMoodState(user.State) {
			// Only the run the current state belongs to is wiped — a paused
			// run of another instrument stays resumable.
			switch user.State {
			case state.StateMoodWho5Question:
				user.Who5 = nil
			case state.StateMoodGad7Question:
				user.Gad7 = nil
			case state.StateMoodQuestion, state.StateMoodCrisis, state.StateMoodQ10:
				user.Mood = nil
			}
			if c := r.moodContent(); c != nil {
				confirmText = c.Module.UI.AbandonConfirmed
			}
		} else {
			user.Screening = nil
			if c := r.screeningContent(); c != nil {
				confirmText = c.Module.UI.AbandonConfirmed
			}
		}
		next := testExitState
		user.State = next
		r.store.Set(msg.From.ID, user)

		if confirmText != "" {
			out := tgbotapi.NewMessage(msg.Chat.ID, confirmText)
			out.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
			if _, err := s.Send(out); err != nil {
				log.Printf("journey: abandon send: %v", err)
			}
		}
		if phase, ok := r.phases[next]; ok {
			ctx := r.contextFor(msg.From.ID, user)
			r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
		}
		return
	}

	if user.CurrentProduct == "" {
		send(s, msg.Chat.ID, r.trans.T("cmd.abandon.no_active", locale, nil))
		return
	}

	abandoned := user.CurrentProduct
	displayName := abandoned
	if prod, ok := r.catalog.Find(abandoned); ok {
		displayName = prod.DisplayName(locale)
	}

	if user.Products == nil {
		user.Products = make(map[string]state.ProductProgress)
	}
	prog := user.Products[abandoned]
	prog.Status = "interrupted"
	prog.LastStage = user.CurrentStage
	prog.UpdatedAt = r.now()
	user.Products[abandoned] = prog
	user.CurrentProduct = ""
	user.CurrentStage = ""
	user.OfferedProducts = nil
	user.State = state.StateAwaitingProductChoice
	r.store.Set(msg.From.ID, user)

	send(s, msg.Chat.ID, r.trans.T("cmd.abandon.confirmed", locale, map[string]any{
		"name": displayName,
	}))

	if phase, ok := r.phases[state.StateAwaitingProductChoice]; ok {
		ctx := r.contextFor(msg.From.ID, user)
		r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
	}
}

func (r *Runner) localeForUser(u state.UserData) i18n.Locale {
	if u.Locale != "" {
		return i18n.Locale(u.Locale)
	}
	return i18n.Locale(r.settings.Get().DefaultLocale)
}

func send(s router.Sender, chatID int64, text string) {
	if _, err := s.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		log.Printf("journey: send: %v", err)
	}
}

func sortedKeys(m map[string]state.ProductProgress) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
