package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodDeleteConfirmPhase owns StateMoodDeleteConfirm — the /mood_delete
// confirmation. Confirming wipes ALL mood-module data in one Set: the three
// transient runs, the three stored results, and the module-wide consent
// timestamp (consent will be asked again on the next entry). Cancelling
// leaves everything intact (unfinished runs stay resumable via /mood).
type MoodDeleteConfirmPhase struct {
	c *screening.MoodContent
}

func NewMoodDeleteConfirmPhase(c *screening.MoodContent) *MoodDeleteConfirmPhase {
	return &MoodDeleteConfirmPhase{c: c}
}

func (MoodDeleteConfirmPhase) State() state.StateKind { return state.StateMoodDeleteConfirm }

// wipeMoodData clears every persisted trace of the mood module.
func wipeMoodData(u *state.UserData) {
	u.Mood, u.MoodResult = nil, nil
	u.Who5, u.Who5Result = nil, nil
	u.Gad7, u.Gad7Result = nil, nil
	u.MoodConsentAt = nil
}

func (p *MoodDeleteConfirmPhase) Setup(ctx Context) Outcome {
	ui := p.c.Module.UI
	oc := scrText(ui.DeleteConfirmPrompt)
	oc.Keyboard = [][]string{{ui.DeleteConfirmButton}, {ui.DeleteCancelButton}}
	return oc
}

func (p *MoodDeleteConfirmPhase) Collect(ctx Context, input string) Outcome {
	ui := p.c.Module.UI
	in := normText(input)
	switch {
	case labelIs(in, ui.DeleteConfirmButton):
		oc := scrText(ui.DeleteDone)
		oc.RemoveKeyboard = true
		if _, midTest := moodDeleteResumeTarget(ctx.User); midTest {
			// Confirming mid-test destroys the very position the flow would
			// return to — that run is abandoned, so land home. The FODMAP
			// detour (if any) stays reachable via the landing's diary button.
			oc.NextState = testExitState
			oc.Mutate = wipeMoodData
			return oc
		}
		// Not mid-test: return to the state the command interrupted (the
		// FODMAP question or the landing, recorded by enterDeleteConfirm).
		oc.NextState = deleteReturnState(ctx.User)
		oc.Mutate = func(u *state.UserData) {
			wipeMoodData(u)
			u.ReturnState = ""
		}
		return oc
	case labelIs(in, ui.DeleteCancelButton):
		// Cancelling mid-test returns to the interrupted position (recorded
		// by enterDeleteConfirm into the run's ResumeState) — never eject
		// the user from the run. The detour bookkeeping stays for the
		// eventual test exit.
		if target, midTest := moodDeleteResumeTarget(ctx.User); midTest {
			return Outcome{
				ReplyKey:       "scr.delete.cancelled",
				RemoveKeyboard: true,
				NextState:      target,
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

func (MoodDeleteConfirmPhase) Remind(ctx Context) Outcome { return Outcome{} }
