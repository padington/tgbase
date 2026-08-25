package journey

import (
	"github.com/padington/tgbase/internal/state"
)

// ModeChoicePhase owns StateAwaitingModeChoice — the fork shown on /start:
// FODMAP diary, ADHD self-check, or mood self-check (PHQ-9). Picking the
// diary reproduces exactly the legacy /start semantics (interrupt the active
// trial, clear picker state); picking either self-check leaves the diary
// untouched and remembers where to come back via UserData.ReturnState (set
// by HandleStart before entry).
type ModeChoicePhase struct{}

func NewModeChoicePhase() *ModeChoicePhase { return &ModeChoicePhase{} }

func (ModeChoicePhase) State() state.StateKind { return state.StateAwaitingModeChoice }

func (ModeChoicePhase) Setup(ctx Context) Outcome {
	key := "phase.mode.prompt"
	var args map[string]any
	if ctx.User.CurrentProduct != "" {
		name := ctx.User.CurrentProduct
		if prod, ok := ctx.Catalog.Find(name); ok {
			name = prod.DisplayName(ctx.Locale)
		}
		key = "phase.mode.prompt_active_trial"
		args = map[string]any{
			"name":  name,
			"stage": ctx.User.CurrentStage,
		}
	}
	return Outcome{
		ReplyKey:  key,
		ReplyArgs: args,
		Keyboard: [][]string{
			{ctx.Trans.T("button.mode.fodmap", ctx.Locale, nil)},
			{ctx.Trans.T("button.mode.screening", ctx.Locale, nil)},
			{ctx.Trans.T("button.mode.mood", ctx.Locale, nil)},
		},
	}
}

func (ModeChoicePhase) Collect(ctx Context, input string) Outcome {
	in := normText(input)
	switch {
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

// Remind is deliberately silent: users parked on the fork get no nudges
// (documented trade-off — the fork belongs to both modes and neither
// reminder fits).
func (ModeChoicePhase) Remind(ctx Context) Outcome { return Outcome{} }
