package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// ModeChoicePhase owns StateAwaitingModeChoice — the home landing shown on
// /start and /menu: a short greeting plus one button per mode (FODMAP diary,
// ADHD self-check, mood self-check, eating self-check, pushup track) and a
// report button. The landing is
// contextual: an unfinished self-check adds a "resume" button on top, an
// active FODMAP trial adds a "back to the diary" button naming the product
// and stage. Picking the diary MODE button reproduces exactly the legacy
// /start semantics (interrupt the active trial, clear picker state); the
// resume buttons and the four track buttons leave everything untouched.
//
// The pushup track is the one mode whose landing strings come from its own
// content bundle rather than from i18n (`pu`, optional): the bundle already
// owns the button and the «подход N/M» resume label, and keeping them there
// means one place edits the whole track. Without the bundle the landing
// simply has no pushup row — /pushups still works.
type ModeChoicePhase struct {
	pu *screening.PushupContent
}

func NewModeChoicePhase() *ModeChoicePhase { return &ModeChoicePhase{} }

// NewModeChoicePhaseWithPushups is the landing wired with the pushup track:
// its mode button and its contextual resume row.
func NewModeChoicePhaseWithPushups(pu *screening.PushupContent) *ModeChoicePhase {
	return &ModeChoicePhase{pu: pu}
}

func (ModeChoicePhase) State() state.StateKind { return state.StateAwaitingModeChoice }

// homeLabel is the 🏠 button label — appended to the FODMAP keyboards as the
// visible twin of /menu. Runner.HandleText intercepts a tap on it in every
// journey state and routes to the landing.
func homeLabel(ctx Context) string {
	return ctx.Trans.T("button.menu.home", ctx.Locale, nil)
}

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

func (p *ModeChoicePhase) Setup(ctx Context) Outcome {
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
	if anyMoodProgress(ctx.User) {
		kb = append(kb, []string{ctx.Trans.T("button.mode.resume_mood", ctx.Locale, nil)})
	}
	if anyEatProgress(ctx.User) {
		kb = append(kb, []string{ctx.Trans.T("button.mode.resume_eating", ctx.Locale, nil)})
	}
	if s := p.puSession(ctx); s != nil {
		kb = append(kb, []string{puResumeLabel(p.pu, s)})
	}
	kb = append(kb,
		[]string{ctx.Trans.T("button.mode.fodmap", ctx.Locale, nil)},
		[]string{ctx.Trans.T("button.mode.screening", ctx.Locale, nil)},
		[]string{ctx.Trans.T("button.mode.mood", ctx.Locale, nil)},
		[]string{ctx.Trans.T("button.mode.eating", ctx.Locale, nil)},
	)
	if p.pu != nil {
		kb = append(kb, []string{p.pu.UI.ModeButton})
	}
	kb = append(kb, []string{ctx.Trans.T("button.mode.report", ctx.Locale, nil)})
	return Outcome{
		ReplyKey:  key,
		ReplyArgs: args,
		Keyboard:  kb,
	}
}

// puSession is the open pushup session the resume row stands for, or nil
// (no bundle wired, no session, or a session past its TTL).
func (p *ModeChoicePhase) puSession(ctx Context) *state.PushupSession {
	if p.pu == nil {
		return nil
	}
	return puWorkoutSession(p.pu, ctx.User, ctx.Now())
}

func (p *ModeChoicePhase) Collect(ctx Context, input string) Outcome {
	in := normText(input)
	if p.pu != nil {
		if s := p.puSession(ctx); s != nil && labelIs(in, puResumeLabel(p.pu, s)) {
			// Straight back into the open session: the exact set, or the
			// rest when its timer is still running.
			return Outcome{NextState: puResumeTarget(p.pu, ctx.User, ctx.Now())}
		}
		if labelIs(in, p.pu.UI.ModeButton) {
			// The pushup track: consent → safety gate → goal → variation →
			// first test for a fresh user, the track menu afterwards.
			return Outcome{NextState: puEntryState(p.pu, ctx.User, ctx.Now())}
		}
	}
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
	case anyMoodProgress(ctx.User) && labelIs(in, ctx.Trans.T("button.mode.resume_mood", ctx.Locale, nil)):
		// Straight back into the single unfinished run (the crisis card /
		// functional question when the PHQ-9 paused there); with several
		// paused runs the module menu disambiguates via its resume rows.
		return Outcome{NextState: moodResumeTarget(ctx.User)}
	case anyEatProgress(ctx.User) && labelIs(in, ctx.Trans.T("button.mode.resume_eating", ctx.Locale, nil)):
		// Straight back into the single unfinished eating run; with several
		// paused runs the track menu disambiguates via its resume rows.
		return Outcome{NextState: eatResumeTarget(ctx.User)}
	case labelIs(in, ctx.Trans.T("button.mode.report", ctx.Locale, nil)):
		// Render the /report breakdown and stay on the landing.
		return Outcome{
			ReplyKey:  "scr.text",
			ReplyArgs: map[string]any{"text": reportText(ctx.Trans, ctx.Locale, ctx.User, p.pu)},
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
		// The mood module: the one-time consent for fresh users, the module
		// menu (with its per-instrument resume rows) afterwards.
		return Outcome{NextState: moodEntryState(ctx.User)}
	case labelIs(in, ctx.Trans.T("button.mode.eating", ctx.Locale, nil)):
		// The eating track: same shape — one-time consent, then the menu.
		return Outcome{NextState: eatEntryState(ctx.User)}
	default:
		return Outcome{ReplyKey: "phase.mode.invalid"}
	}
}

// Remind is deliberately silent: users parked on the landing get no nudges
// (documented trade-off — the landing belongs to all modes and no single
// reminder fits).
func (ModeChoicePhase) Remind(ctx Context) Outcome { return Outcome{} }
