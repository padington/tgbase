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
	locale := r.localeForUser(user)

	scrLine := r.screeningReportLine(user, locale)

	if len(user.Products) == 0 {
		if scrLine == "" {
			send(s, msg.Chat.ID, r.trans.T("cmd.report.empty", locale, nil))
			return
		}
		send(s, msg.Chat.ID, r.trans.T("cmd.report.heading", locale, nil)+"\n"+scrLine)
		return
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
	lines = append(lines, r.trans.T("cmd.report.heading", locale, nil))
	if user.CurrentProduct != "" {
		lines = append(lines, r.trans.T("cmd.report.in_progress", locale, map[string]any{
			"name":  user.CurrentProduct,
			"stage": user.CurrentStage,
		}))
	}
	if len(completed) > 0 {
		lines = append(lines, r.trans.T("cmd.report.completed", locale, map[string]any{
			"names": strings.Join(completed, ", "),
		}))
	}
	if len(notTolerated) > 0 {
		lines = append(lines, r.trans.T("cmd.report.not_tolerated", locale, map[string]any{
			"names": strings.Join(notTolerated, ", "),
		}))
	}
	if len(interrupted) > 0 {
		lines = append(lines, r.trans.T("cmd.report.interrupted", locale, map[string]any{
			"names": strings.Join(interrupted, ", "),
		}))
	}
	if scrLine != "" {
		lines = append(lines, scrLine)
	}
	send(s, msg.Chat.ID, strings.Join(lines, "\n"))
}

// screeningReportLine renders the /report line for the last completed ADHD
// self-check: per-instrument scores with their applied thresholds — never a
// combined score.
func (r *Runner) screeningReportLine(user state.UserData, locale i18n.Locale) string {
	res := user.ScreeningResult
	if res == nil {
		return ""
	}
	verdictWord := func(positive bool) string {
		if positive {
			return r.trans.T("scr.report.positive", locale, nil)
		}
		return r.trans.T("scr.report.negative", locale, nil)
	}
	return r.trans.T("cmd.report.screening", locale, map[string]any{
		"date":      res.TakenAt.Format(reportDateLayout),
		"asrs_a":    res.AsrsASignificant,
		"a_thr":     res.AsrsAThreshold,
		"a_verdict": verdictWord(res.AsrsAPositive),
		"asrs_b":    res.AsrsBSignificant,
		"wurs":      res.WursScore,
		"w_thr":     res.WursCutoff,
		"w_verdict": verdictWord(res.WursPositive),
		"overall":   r.trans.T("scr.report.overall."+res.Verdict, locale, nil),
	})
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

// HandleAdhd is the direct entry into the ADHD self-check. Mid-screening it
// re-fires the current phase's Setup (redraws the question and keyboard);
// from a FODMAP state it records the detour and routes to consent (which
// itself forwards to the resume intro when an unfinished run exists).
func (r *Runner) HandleAdhd(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	if _, ok := r.phases[state.StateScrConsent]; !ok {
		return // screening mode not wired
	}
	user := r.store.Get(msg.From.ID)
	user.ChatID = msg.Chat.ID
	if user.Locale == "" {
		user.Locale = string(r.detectLocale(msg.From.LanguageCode))
	}

	if isScreeningState(user.State) {
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
	user.State = state.StateScrConsent
	user.EnteredAt = r.now()
	r.store.Set(msg.From.ID, user)

	phase := r.phases[state.StateScrConsent]
	ctx := r.contextFor(msg.From.ID, user)
	r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
}

// HandleAdhdDelete starts the delete-confirmation flow for all stored
// self-check data. With nothing stored it answers immediately and does not
// change state.
func (r *Runner) HandleAdhdDelete(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	c := r.screeningContent()
	if c == nil {
		return // screening mode not wired
	}
	user := r.store.Get(msg.From.ID)

	if user.Screening == nil && user.ScreeningResult == nil {
		send(s, msg.Chat.ID, c.Module.UI.DeleteNothing)
		return
	}

	if isFodmapJourneyState(user.State) {
		user.ReturnState = user.State
	}
	user.ChatID = msg.Chat.ID
	user.State = state.StateScrDeleteConfirm
	r.store.Set(msg.From.ID, user)

	if phase, ok := r.phases[state.StateScrDeleteConfirm]; ok {
		ctx := r.contextFor(msg.From.ID, user)
		r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
	}
}

// HandleAbandon aborts the current activity. Mid-screening it wipes the
// transient raw answers (the previous completed ScreeningResult is kept) and
// returns to the recorded FODMAP state or idle. Otherwise it marks the
// active trial as interrupted and re-routes to the product selection phase,
// exactly as before.
func (r *Runner) HandleAbandon(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	locale := r.localeForUser(user)

	if isScreeningState(user.State) {
		c := r.screeningContent()
		next := state.StateIdle
		if user.ReturnState != "" {
			next = user.ReturnState
		}
		user.Screening = nil
		user.ReturnState = ""
		user.State = next
		r.store.Set(msg.From.ID, user)

		if c != nil {
			out := tgbotapi.NewMessage(msg.Chat.ID, c.Module.UI.AbandonConfirmed)
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
