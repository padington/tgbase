package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodDeleteConfirmPhase owns StateMoodDeleteConfirm — the /mood_delete
// confirmation. Confirming wipes both the transient progress and the stored
// result in one Set; cancelling leaves everything intact (an unfinished run
// stays resumable via /mood).
type MoodDeleteConfirmPhase struct {
	c *screening.MoodContent
}

func NewMoodDeleteConfirmPhase(c *screening.MoodContent) *MoodDeleteConfirmPhase {
	return &MoodDeleteConfirmPhase{c: c}
}

func (MoodDeleteConfirmPhase) State() state.StateKind { return state.StateMoodDeleteConfirm }

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
		oc.NextState = exitState(ctx.User)
		oc.Mutate = func(u *state.UserData) {
			u.Mood = nil
			u.MoodResult = nil
			u.ReturnState = ""
		}
		return oc
	case labelIs(in, ui.DeleteCancelButton):
		// Cancelling mid-test returns to the interrupted question or crisis
		// card (recorded by enterDeleteConfirm) — never eject the user from
		// the run. The detour bookkeeping stays for the eventual test exit.
		if s := ctx.User.Mood; s != nil && resumableMoodState(s.ResumeState) {
			return Outcome{
				ReplyKey:       "scr.delete.cancelled",
				RemoveKeyboard: true,
				NextState:      s.ResumeState,
			}
		}
		return Outcome{
			ReplyKey:       "scr.delete.cancelled",
			RemoveKeyboard: true,
			NextState:      exitState(ctx.User),
			Mutate:         clearReturnState,
		}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (MoodDeleteConfirmPhase) Remind(ctx Context) Outcome { return Outcome{} }
