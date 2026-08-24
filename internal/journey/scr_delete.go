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
		oc.NextState = exitState(ctx.User)
		oc.Mutate = func(u *state.UserData) {
			u.Screening = nil
			u.ScreeningResult = nil
			u.ReturnState = ""
		}
		return oc
	case labelIs(in, ui.DeleteCancelButton):
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

func (ScrDeleteConfirmPhase) Remind(ctx Context) Outcome { return Outcome{} }
