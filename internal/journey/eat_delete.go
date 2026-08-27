package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// EatDeleteConfirmPhase owns StateEatDeleteConfirm — the /food_delete
// confirmation. Confirming wipes ALL eating-track data in one Set: the three
// transient runs, the three stored results, and the track-wide consent
// timestamp (consent will be asked again on the next entry). Cancelling
// leaves everything intact (unfinished runs stay resumable via /food).
type EatDeleteConfirmPhase struct {
	c *screening.EatingContent
}

func NewEatDeleteConfirmPhase(c *screening.EatingContent) *EatDeleteConfirmPhase {
	return &EatDeleteConfirmPhase{c: c}
}

func (EatDeleteConfirmPhase) State() state.StateKind { return state.StateEatDeleteConfirm }

// wipeEatingData clears every persisted trace of the eating track.
func wipeEatingData(u *state.UserData) {
	u.Edeqs, u.EdeqsResult = nil, nil
	u.Bes, u.BesResult = nil, nil
	u.Nias, u.NiasResult = nil, nil
	u.EatConsentAt = nil
}

func (p *EatDeleteConfirmPhase) Setup(ctx Context) Outcome {
	ui := p.c.Module.UI
	oc := scrText(ui.DeleteConfirmPrompt)
	oc.Keyboard = [][]string{{ui.DeleteConfirmButton}, {ui.DeleteCancelButton}}
	return oc
}

func (p *EatDeleteConfirmPhase) Collect(ctx Context, input string) Outcome {
	ui := p.c.Module.UI
	in := normText(input)
	switch {
	case labelIs(in, ui.DeleteConfirmButton):
		oc := scrText(ui.DeleteDone)
		oc.RemoveKeyboard = true
		if _, midTest := eatDeleteResumeTarget(ctx.User); midTest {
			// Confirming mid-test destroys the very position the flow would
			// return to — that run is abandoned, so land home. The FODMAP
			// detour (if any) stays reachable via the landing's diary button.
			oc.NextState = testExitState
			oc.Mutate = wipeEatingData
			return oc
		}
		// Not mid-test: return to the state the command interrupted (the
		// FODMAP question or the landing, recorded by enterDeleteConfirm).
		oc.NextState = deleteReturnState(ctx.User)
		oc.Mutate = func(u *state.UserData) {
			wipeEatingData(u)
			u.ReturnState = ""
		}
		return oc
	case labelIs(in, ui.DeleteCancelButton):
		// Cancelling mid-test returns to the interrupted position (recorded
		// by enterDeleteConfirm into the run's ResumeState) — never eject
		// the user from the run. The marker is consumed here: the question
		// phase derives its position from the answers anyway, and a
		// leftover marker would mislead the next /food_delete.
		if target, midTest := eatDeleteResumeTarget(ctx.User); midTest {
			return Outcome{
				ReplyKey:       "scr.delete.cancelled",
				RemoveKeyboard: true,
				NextState:      target,
				Mutate:         clearEatResumeStates,
			}
		}
		return Outcome{
			ReplyKey:       "scr.delete.cancelled",
			RemoveKeyboard: true,
			NextState:      deleteReturnState(ctx.User),
			Mutate:         clearReturnState,
		}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (EatDeleteConfirmPhase) Remind(ctx Context) Outcome { return Outcome{} }
