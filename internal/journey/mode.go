package journey

import (
	"github.com/padington/tgbase/internal/state"
)

// ModeChoicePhase owns StateAwaitingModeChoice — the home landing shown on
// /start and /menu: a short greeting plus one button per mode (FODMAP diary,
// ADHD self-check, mood self-check) and a report button. The landing is
// contextual: an unfinished self-check adds a "resume" button on top, an
// active FODMAP trial adds a "back to the diary" button naming the product
// and stage. Picking the diary MODE button reproduces exactly the legacy
// /start semantics (interrupt the active trial, clear picker state); the
// resume buttons and both self-check buttons leave everything untouched.
type ModeChoicePhase struct{}

func NewModeChoicePhase() *ModeChoicePhase { return &ModeChoicePhase{} }

func (ModeChoicePhase) State() state.StateKind { return state.StateAwaitingModeChoice }

// trialLabel renders the active trial as "Name" or "Name (stage)" for the
// landing prompt and the resume-diary button.
func trialLabel(ctx Context) string {
	name := ctx.User.CurrentProduct
	if prod, ok := ctx.Catalog.Find(name); ok {
		name = prod.DisplayName(ctx.Locale)
	}
	if ctx.User.CurrentStage != "" {
		return name + " (" + string(ctx.User.CurrentStage) + ")"
	}
	return name
}

// diaryResumeState is where the resume-diary button returns: the recorded
// detour state when it is a diary state, otherwise the state the active
// trial lives in.
func diaryResumeState(u state.UserData) state.StateKind {
	if isFodmapJourneyState(u.ReturnState) {
		return u.ReturnState
	}
	if u.CurrentStage != "" {
		return state.StateAwaitingStageCheckin
	}
	return state.StateAwaitingStageChoice
}

func (ModeChoicePhase) Setup(ctx Context) Outcome {
	key := "phase.mode.prompt"
	var args map[string]any
	if ctx.User.CurrentProduct != "" {
		key = "phase.mode.prompt_active_trial"
		args = map[string]any{"trial": trialLabel(ctx)}
	}

	var kb [][]string
	// Contextual resume rows first — the most likely next action on top.
	if ctx.User.CurrentProduct != "" {
		kb = append(kb, []string{ctx.Trans.T("button.mode.resume_fodmap", ctx.Locale,
			map[string]any{"trial": trialLabel(ctx)})})
	}
	if ctx.User.Screening != nil {
		kb = append(kb, []string{ctx.Trans.T("button.mode.resume_screening", ctx.Locale, nil)})
	}
	if ctx.User.Mood != nil {
		kb = append(kb, []string{ctx.Trans.T("button.mode.resume_mood", ctx.Locale, nil)})
	}
	kb = append(kb,
		[]string{ctx.Trans.T("button.mode.fodmap", ctx.Locale, nil)},
		[]string{ctx.Trans.T("button.mode.screening", ctx.Locale, nil)},
		[]string{ctx.Trans.T("button.mode.mood", ctx.Locale, nil)},
		[]string{ctx.Trans.T("button.mode.report", ctx.Locale, nil)},
	)
	return Outcome{
		ReplyKey:  key,
		ReplyArgs: args,
		Keyboard:  kb,
	}
}

func (ModeChoicePhase) Collect(ctx Context, input string) Outcome {
	in := normText(input)
	switch {
	case ctx.User.CurrentProduct != "" && labelIs(in, ctx.Trans.T("button.mode.resume_fodmap",
		ctx.Locale, map[string]any{"trial": trialLabel(ctx)})):
		// Back to the paused diary trial: nothing is interrupted, the
		// detour bookkeeping is consumed.
		return Outcome{
			NextState: diaryResumeState(ctx.User),
			Mutate:    clearReturnState,
		}
	case ctx.User.Screening != nil && labelIs(in, ctx.Trans.T("button.mode.resume_screening", ctx.Locale, nil)):
		// Straight back to the recorded question. Without a usable
		// ResumeState the consent gate routes to the resume intro, which
		// derives the position from the recorded answers.
		next := state.StateScrConsent
		if resumableScrState(ctx.User.Screening.ResumeState) {
			next = ctx.User.Screening.ResumeState
		}
		return Outcome{NextState: next}
	case ctx.User.Mood != nil && labelIs(in, ctx.Trans.T("button.mode.resume_mood", ctx.Locale, nil)):
		// Straight back to the recorded position (the crisis card when the
		// run paused there); the question phase derives the index from the
		// recorded answers when no usable ResumeState exists.
		next := state.StateMoodQuestion
		if resumableMoodState(ctx.User.Mood.ResumeState) {
			next = ctx.User.Mood.ResumeState
		}
		return Outcome{NextState: next}
	case labelIs(in, ctx.Trans.T("button.mode.report", ctx.Locale, nil)):
		// Render the /report breakdown and stay on the landing.
		return Outcome{
			ReplyKey:  "scr.text",
			ReplyArgs: map[string]any{"text": reportText(ctx.Trans, ctx.Locale, ctx.User)},
		}
	case labelIs(in, ctx.Trans.T("button.mode.fodmap", ctx.Locale, nil)):
		// Exactly the legacy /start flow: interrupt the active trial,
		// reset transient picker state, drop any recorded detour. An
		// unfinished Screening is NOT reset — it stays reachable via /adhd.
		now := ctx.Now()
		return Outcome{
			NextState: state.StateAwaitingDefecation,
			Mutate: func(u *state.UserData) {
				if u.CurrentProduct != "" {
					if u.Products == nil {
						u.Products = make(map[string]state.ProductProgress)
					}
					prog := u.Products[u.CurrentProduct]
					prog.Status = "interrupted"
					prog.LastStage = u.CurrentStage
					prog.UpdatedAt = now
					u.Products[u.CurrentProduct] = prog
					u.CurrentProduct = ""
					u.CurrentStage = ""
				}
				u.ReturnState = ""
				u.EnteredAt = now
				u.ReminderSent = false
				u.CheckinAsked = false
				u.OfferedProducts = nil
				u.PickerCategory = ""
				u.PickerPage = 0
			},
		}
	case labelIs(in, ctx.Trans.T("button.mode.screening", ctx.Locale, nil)):
		// Consent phase redirects to the resume intro by itself when an
		// unfinished Screening exists.
		return Outcome{NextState: state.StateScrConsent}
	case labelIs(in, ctx.Trans.T("button.mode.mood", ctx.Locale, nil)):
		// The mood consent phase renders in resume mode by itself when an
		// unfinished Mood run exists.
		return Outcome{NextState: state.StateMoodConsent}
	default:
		return Outcome{ReplyKey: "phase.mode.invalid"}
	}
}

// Remind is deliberately silent: users parked on the landing get no nudges
// (documented trade-off — the landing belongs to all modes and no single
// reminder fits).
func (ModeChoicePhase) Remind(ctx Context) Outcome { return Outcome{} }
