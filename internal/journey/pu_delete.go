package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// PuDeleteConfirmPhase owns StatePuDeleteConfirm — the /pushups_delete
// confirmation. Confirming wipes the WHOLE track in one Set: the program,
// the unfinished session, the last test, the history ring and the track-wide
// consent (which will be asked again on the next entry). Cancelling leaves
// everything intact — an open session stays resumable.
//
// Where the dialog closes to is recorded by enterDeleteConfirm in
// ReturnState. Confirming from inside the track destroys the very position
// the cancel would return to, so that case lands home instead.
type PuDeleteConfirmPhase struct {
	c *screening.PushupContent
}

func NewPuDeleteConfirmPhase(c *screening.PushupContent) *PuDeleteConfirmPhase {
	return &PuDeleteConfirmPhase{c: c}
}

func (PuDeleteConfirmPhase) State() state.StateKind { return state.StatePuDeleteConfirm }

func (p *PuDeleteConfirmPhase) Setup(ctx Context) Outcome {
	oc := scrText(p.c.UI.DeleteConfirmPrompt)
	oc.Keyboard = [][]string{{p.c.UI.DeleteConfirmButton}, {p.c.UI.DeleteCancelButton}}
	return oc
}

func (p *PuDeleteConfirmPhase) Collect(ctx Context, input string) Outcome {
	in := normText(input)
	switch {
	case labelIs(in, p.c.UI.DeleteConfirmButton):
		oc := scrText(p.c.UI.DeleteDone)
		oc.RemoveKeyboard = true
		oc.NextState = deleteReturnState(ctx.User)
		if isPushupState(oc.NextState) {
			// The state the dialog was opened from no longer exists.
			oc.NextState = testExitState
		}
		oc.Mutate = func(u *state.UserData) {
			puWipeTrack(u)
			u.ReturnState = ""
		}
		return oc
	case labelIs(in, p.c.UI.DeleteCancelButton):
		// Back to whatever the command interrupted — including a set or a
		// rest: cancelling must never eject the user from a live session.
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

func (PuDeleteConfirmPhase) Remind(ctx Context) Outcome { return Outcome{} }
