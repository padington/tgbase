package journey

import (
	"github.com/padington/tgbase/internal/state"
)

// DefecationPhase owns StateAwaitingDefecation. Setup sends the 1/2/3
// prompt; Collect maps the reply to a DefecationKind; Remind nudges users
// who haven't replied within Settings.DefecationReminderAfter.
type DefecationPhase struct{}

func NewDefecationPhase() *DefecationPhase { return &DefecationPhase{} }

func (DefecationPhase) State() state.StateKind { return state.StateAwaitingDefecation }

func (DefecationPhase) Setup(ctx Context) Outcome {
	return Outcome{
		ReplyKey: "phase.defecation.prompt",
		Buttons:  []string{"1", "2", "3"},
	}
}

func (DefecationPhase) Collect(ctx Context, input string) Outcome {
	var kind state.DefecationKind
	switch input {
	case "1":
		kind = state.DefecationFluid
	case "2":
		kind = state.DefecationNormal
	case "3":
		kind = state.DefecationIssues
	default:
		return Outcome{ReplyKey: "phase.defecation.invalid"}
	}
	return Outcome{
		NextState: state.StateAwaitingProductChoice,
		Mutate: func(u *state.UserData) {
			u.DefecationState = kind
			u.ReminderSent = false
		},
	}
}

func (DefecationPhase) Remind(ctx Context) Outcome {
	if ctx.User.ReminderSent {
		return Outcome{}
	}
	if ctx.Now().Sub(ctx.User.EnteredAt) < ctx.Settings.DefecationReminderAfter {
		return Outcome{}
	}
	return Outcome{
		ReplyKey: "phase.defecation.reminder",
		Mutate: func(u *state.UserData) {
			u.ReminderSent = true
		},
	}
}
