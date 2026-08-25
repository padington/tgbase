package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// ScrDeleteConfirmPhase owns StateScrDeleteConfirm — the /adhd_delete
// confirmation. Confirming wipes both the transient progress and the stored
// result in one Set; cancelling leaves everything intact (an unfinished run
// stays resumable via /adhd).
type ScrDeleteConfirmPhase struct {
	c *screening.Content
}

func NewScrDeleteConfirmPhase(c *screening.Content) *ScrDeleteConfirmPhase {
	return &ScrDeleteConfirmPhase{c: c}
}

func (ScrDeleteConfirmPhase) State() state.StateKind { return state.StateScrDeleteConfirm }

func (p *ScrDeleteConfirmPhase) Setup(ctx Context) Outcome {
	ui := p.c.Module.UI
	oc := scrText(ui.DeleteConfirmPrompt)
	oc.Keyboard = [][]string{{ui.DeleteConfirmButton}, {ui.DeleteCancelButton}}
	return oc
}

func (p *ScrDeleteConfirmPhase) Collect(ctx Context, input string) Outcome {
	ui := p.c.Module.UI
	in := normText(input)
	switch {
	case labelIs(in, ui.DeleteConfirmButton):
		oc := scrText(ui.DeleteDone)
		oc.RemoveKeyboard = true
		if s := ctx.User.Screening; s != nil && resumableScrState(s.ResumeState) {
			// Confirming mid-test destroys the very position the flow
			// would return to — that run is abandoned, so land home. The
			// FODMAP detour (if any) stays reachable via the landing's
			// diary button.
			oc.NextState = testExitState
			oc.Mutate = func(u *state.UserData) {
				u.Screening = nil
				u.ScreeningResult = nil
			}
			return oc
		}
		// Not mid-test: return to the state the command interrupted (the
		// FODMAP question or the landing, recorded by enterDeleteConfirm).
		oc.NextState = deleteReturnState(ctx.User)
		oc.Mutate = func(u *state.UserData) {
			u.Screening = nil
			u.ScreeningResult = nil
			u.ReturnState = ""
		}
		return oc
	case labelIs(in, ui.DeleteCancelButton):
		// Cancelling mid-test returns to the interrupted question (recorded
		// by enterDeleteConfirm) — never eject the user from the run. The
		// detour bookkeeping stays for the eventual test exit.
		if s := ctx.User.Screening; s != nil && resumableScrState(s.ResumeState) {
			return Outcome{
				ReplyKey:       "scr.delete.cancelled",
				RemoveKeyboard: true,
				NextState:      s.ResumeState,
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

func (ScrDeleteConfirmPhase) Remind(ctx Context) Outcome { return Outcome{} }
