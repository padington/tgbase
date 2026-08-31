package journey

import (
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/state"
)

// Runner dispatches incoming user input and reminder ticks to the
// registered phases. It is the top-level orchestrator that the bot
// wiring connects to the router.
type Runner struct {
	phases   map[state.StateKind]Phase
	store    *state.Store
	catalog  *products.Catalog
	settings *settings.Store
	trans    i18n.Translator
	sender   router.Sender
	now      func() time.Time
}

func New(
	store *state.Store,
	sender router.Sender,
	catalog *products.Catalog,
	settingsStore *settings.Store,
	trans i18n.Translator,
) *Runner {
	return &Runner{
		phases:   make(map[state.StateKind]Phase),
		store:    store,
		catalog:  catalog,
		settings: settingsStore,
		trans:    trans,
		sender:   sender,
		now:      time.Now,
	}
}

// Register installs a Phase under its declared State.
func (r *Runner) Register(p Phase) {
	r.phases[p.State()] = p
}

// HandleStart implements router.HandlerFunc for /start and its alias /menu.
// With the screening mode wired it routes the user to the landing (the
// mode-choice phase) from ANY state without losing progress. Nothing is
// interrupted here — the FODMAP mode button of the landing owns the legacy
// interrupt semantics.
func (r *Runner) HandleStart(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	if _, ok := r.phases[state.StateAwaitingModeChoice]; !ok {
		// Screening mode not wired (no content configured) — keep the
		// legacy /start behavior so the bot stays usable.
		r.legacyStart(msg)
		return
	}
	r.routeToLanding(msg.From.ID, msg.Chat.ID, msg.From.LanguageCode)
}

// routeToLanding is the universal escape shared by /start, /menu and the 🏠
// button: it records how to come back — a FODMAP journey state to
// ReturnState, a resumable screening/mood/eating state to the run's ResumeState —
// then shows the landing. Non-resumable states (delete confirmations, the
// consent/intro gates) never overwrite an earlier recorded position.
func (r *Runner) routeToLanding(userID, chatID int64, langCode string) {
	user := r.store.Get(userID)

	cur := user.State
	switch {
	case isScreeningState(cur):
		// Escape mid-screening: "Continue" must land back here.
		if user.Screening != nil && resumableScrState(cur) {
			sc := user.Screening.Clone()
			sc.ResumeState = cur
			user.Screening = sc
		}
	case isMoodState(cur):
		// Escape mid-mood-test: same resume bookkeeping (matters for the
		// crisis card — a paused run must land back on the card).
		if user.Mood != nil && resumableMoodState(cur) {
			mc := user.Mood.Clone()
			mc.ResumeState = cur
			user.Mood = mc
		}
	case isEatingState(cur):
		// Escape mid-eating-test: nothing to record. Every eating question
		// phase derives its position from the recorded answers (like the
		// mood module's WHO-5 and GAD-7), so the run resumes exactly where
		// it paused without a stored marker. EatingProgress.ResumeState is
		// reserved for the delete dialog, which uses it to tell "opened
		// mid-test" from "opened on the landing" — writing it here would
		// make a merely paused run look interrupted later.
		//
		// The other half of that rule: escaping FROM the delete dialog
		// bypasses the cancel button that consumes the marker, so drop it
		// here. A leftover marker would make the next /food_delete —
		// opened from the landing — close back INTO the paused run.
		clearEatResumeStates(&user)
	case isPushupState(cur):
		// Escape mid-session: the landing must offer «Продолжить
		// тренировку: подход N/M» and land back on the exact set or rest.
		// Only positions inside a running session are recorded; the setup
		// chain, the menu and the confirmations are re-entered through the
		// menu, which derives its position from the program.
		if user.PuSession != nil && resumablePuState(cur) {
			ps := user.PuSession.Clone()
			ps.ResumeState = cur
			user.PuSession = ps
		}
		// A pushup state recorded as the delete dialog's way back is stale
		// the moment the user escapes to the landing — dropping it here is
		// the same hygiene the eating track applies to its marker.
		if isPushupState(user.ReturnState) {
			user.ReturnState = ""
		}
	case cur != state.StateAwaitingModeChoice && isFodmapJourneyState(cur):
		// Remember where to return after a detour. A repeated escape from
		// the landing itself is idempotent — ReturnState is kept.
		user.ReturnState = cur
	}
	// The active trial is NOT marked interrupted here — only the FODMAP
	// button does that.

	user.ChatID = chatID
	if user.Locale == "" {
		user.Locale = string(r.detectLocale(langCode))
	}
	user.State = state.StateAwaitingModeChoice
	user.EnteredAt = r.now()
	r.store.Set(userID, user)

	phase := r.phases[state.StateAwaitingModeChoice]
	ctx := r.contextFor(userID, user)
	r.applyOutcome(ctx, phase.Setup(ctx), chatID)
}

// legacyStart is the pre-fork /start: route to the defecation phase, marking
// any in-progress trial as interrupted. Used only when ModeChoicePhase is
// not registered.
func (r *Runner) legacyStart(msg *tgbotapi.Message) {
	user := r.store.Get(msg.From.ID)

	if user.CurrentProduct != "" {
		if user.Products == nil {
			user.Products = make(map[string]state.ProductProgress)
		}
		prog := user.Products[user.CurrentProduct]
		prog.Status = "interrupted"
		prog.LastStage = user.CurrentStage
		prog.UpdatedAt = r.now()
		user.Products[user.CurrentProduct] = prog
		user.CurrentProduct = ""
		user.CurrentStage = ""
	}

	user.ChatID = msg.Chat.ID
	if user.Locale == "" {
		user.Locale = string(r.detectLocale(msg.From.LanguageCode))
	}
	user.State = state.StateAwaitingDefecation
	user.EnteredAt = r.now()
	user.ReminderSent = false
	user.CheckinAsked = false
	user.OfferedProducts = nil
	user.PickerCategory = ""
	user.PickerPage = 0

	r.store.Set(msg.From.ID, user)

	phase, ok := r.phases[state.StateAwaitingDefecation]
	if !ok {
		return
	}
	ctx := r.contextFor(msg.From.ID, user)
	r.applyOutcome(ctx, phase.Setup(ctx), msg.Chat.ID)
}

// HandleText implements router.HandlerFunc for the journey text predicate.
// It dispatches to the phase whose State matches the user's current state.
// A tap on the 🏠 button is honored BEFORE phase dispatch, so the escape to
// the landing works from every journey state, including confirmations.
func (r *Runner) HandleText(s router.Sender, msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	user := r.store.Get(msg.From.ID)
	if r.isHomeEscape(user, msg.Text) {
		r.routeToLanding(msg.From.ID, msg.Chat.ID, msg.From.LanguageCode)
		return
	}
	phase, ok := r.phases[user.State]
	if !ok {
		log.Printf("journey: no phase registered for state %q", user.State)
		return
	}
	ctx := r.contextFor(msg.From.ID, user)
	r.applyOutcome(ctx, phase.Collect(ctx, msg.Text), msg.Chat.ID)
}

// isHomeEscape reports whether the input is a tap on the 🏠 menu button —
// the keyboard twin of /menu. On the landing itself the label falls through
// to the phase (which answers with the invalid-button hint).
func (r *Runner) isHomeEscape(user state.UserData, input string) bool {
	if _, ok := r.phases[state.StateAwaitingModeChoice]; !ok {
		return false
	}
	if user.State == state.StateAwaitingModeChoice {
		return false
	}
	return labelIs(normText(input), r.trans.T("button.menu.home", r.localeForUser(user), nil))
}

// IsJourneyState returns true when the user is currently inside a journey
// phase (so the router can use it as a text-predicate gate).
func (r *Runner) IsJourneyState(userID int64) bool {
	_, ok := r.phases[r.store.Get(userID).State]
	return ok
}

// Remind walks every user the runner cares about and asks each phase
// whether to nudge them. Called by the reminder worker on every tick.
//
// Three state-scoped scans plus one data-driven one: the pushup track's
// "time to train" ping is not tied to the state it would interrupt, so it
// has its own rules (see remindPushupsDue).
func (r *Runner) Remind() {
	for _, kind := range []state.StateKind{
		state.StateAwaitingDefecation,
		state.StateAwaitingStageCheckin,
		state.StatePuRest,
	} {
		phase, ok := r.phases[kind]
		if !ok {
			continue
		}
		var users map[int64]state.UserData
		switch kind {
		case state.StateAwaitingDefecation:
			users = r.store.AllAwaitingDefecation()
		case state.StateAwaitingStageCheckin:
			users = r.store.AllAwaitingCheckin()
		case state.StatePuRest:
			users = r.store.AllPushupResting()
		}
		for userID, user := range users {
			ctx := r.contextFor(userID, user)
			r.applyOutcome(ctx, phase.Remind(ctx), user.ChatID)
		}
	}
	r.remindPushupsDue()
}

// remindPushupsDue sends the pushup track's "time to train" ping. It is the
// first reminder in the bot that belongs to a PROGRAM rather than to a
// state, so it is fenced in three ways, all enforced by the store's own
// filter (state.Store.AllPushupDue): only a user sitting idle, on the
// landing or in the track's menu is reachable, never one mid-check-in,
// mid-self-check or mid-set; never while a session is still resumable (the
// landing offers that one instead); and at most once per due date.
//
// Exactly two messages can be sent per due cycle: the ping at NextDueAt,
// and — if the session still has not happened — one final "still waiting"
// message puOverdueAfter later. Then the track goes quiet until a session
// moves the date.
func (r *Runner) remindPushupsDue() {
	c := r.pushupContent()
	if c == nil {
		return
	}
	now := r.now()
	for userID, user := range r.store.AllPushupDue(now) {
		p := user.Pushups
		plan := puPlan(c, p)
		args := map[string]string{
			"sets":    strconv.Itoa(plan.SetCount()),
			"minutes": strconv.Itoa(c.EstimateMinutes(plan)),
		}
		// The rescheduling below is the only thing that can push NextDueAt
		// past the session's own due date — which makes it the marker of
		// "this is the second and last message of the cycle".
		firstDue := p.LastSessionAt.Add(time.Duration(c.Params.AdvisedHoursBetween) * time.Hour)
		overdue := !p.LastSessionAt.IsZero() && p.NextDueAt.After(firstDue.Add(time.Minute))
		tpl, final := c.UI.DuePing, false
		if overdue {
			tpl, final = c.UI.OverduePing, true
		}
		oc := scrText(renderContent(tpl, args))
		oc.Mutate = func(u *state.UserData) {
			mutatePuProgram(u, func(pr *state.PushupProgram) {
				if final {
					pr.DuePingSent = true
					return
				}
				// Not sent yet, but not due again either: the next (and
				// last) message of this cycle waits out puOverdueAfter.
				pr.NextDueAt = pr.NextDueAt.Add(puOverdueAfter)
			})
		}
		ctx := r.contextFor(userID, user)
		r.applyOutcome(ctx, oc, user.ChatID)
	}
}

func (r *Runner) contextFor(userID int64, user state.UserData) Context {
	locale := i18n.Locale(user.Locale)
	if locale == "" {
		locale = i18n.Locale(r.settings.Get().DefaultLocale)
	}
	return Context{
		UserID:   userID,
		User:     user,
		Catalog:  r.catalog,
		Settings: r.settings.Get(),
		Trans:    r.trans,
		Locale:   locale,
		Now:      r.now,
	}
}

func (r *Runner) applyOutcome(ctx Context, oc Outcome, chatID int64) {
	user := r.store.Get(ctx.UserID)
	if oc.Mutate != nil {
		oc.Mutate(&user)
	}
	// Re-fire Setup whenever NextState is set, including same-state
	// outcomes — that's how the picker refreshes its keyboard after
	// Prev / Next paging.
	transitioned := oc.NextState != ""
	if oc.NextState != "" {
		user.State = oc.NextState
	}
	r.store.Set(ctx.UserID, user)

	if oc.ReplyKey != "" && chatID != 0 {
		text := r.trans.T(oc.ReplyKey, ctx.Locale, oc.ReplyArgs)
		out := tgbotapi.NewMessage(chatID, text)
		if len(oc.Keyboard) > 0 {
			out.ReplyMarkup = buildKeyboard(oc.Keyboard)
		} else if oc.RemoveKeyboard {
			out.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		}
		if _, err := r.sender.Send(out); err != nil {
			log.Printf("journey: send: %v", err)
		}
	}

	if transitioned {
		next, ok := r.phases[user.State]
		if !ok {
			return
		}
		ctx2 := r.contextFor(ctx.UserID, user)
		r.applyOutcome(ctx2, next.Setup(ctx2), chatID)
	}
}

func (r *Runner) detectLocale(lang string) i18n.Locale {
	lang = strings.ToLower(lang)
	if lang == "" {
		return i18n.Locale(r.settings.Get().DefaultLocale)
	}
	if r.trans.Has("phase.defecation.prompt", i18n.Locale(lang)) {
		return i18n.Locale(lang)
	}
	return i18n.Locale(r.settings.Get().DefaultLocale)
}

func buildKeyboard(rows [][]string) tgbotapi.ReplyKeyboardMarkup {
	out := make([][]tgbotapi.KeyboardButton, 0, len(rows))
	for _, row := range rows {
		rb := make([]tgbotapi.KeyboardButton, 0, len(row))
		for _, l := range row {
			rb = append(rb, tgbotapi.NewKeyboardButton(l))
		}
		out = append(out, rb)
	}
	kb := tgbotapi.NewReplyKeyboard(out...)
	kb.ResizeKeyboard = true
	return kb
}
