package journey

import (
	"strings"
	"time"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// PuGatePhase owns StatePuGate — the three-question safety gate that runs
// once, right after consent and before anything is measured. One yes/no
// question per message, driven by the GateIdx cursor (so a pause resumes on
// the exact question), with the content pinning the questions' order and
// what a "yes" does:
//
//   - health   → STOP. The track does not start at all and nothing is kept:
//     the program shell and the consent are removed again, so a
//     later /pushups starts from a clean sheet rather than from a
//     half-built program.
//   - joint pain → start joint_pain_step_down rungs easier.
//   - pregnancy  → "check with your doctor" + the easiest rung.
//
// Risk-factor profiling is deliberately NOT added: it is a barrier, not a
// benefit, for a bodyweight track.
type PuGatePhase struct {
	c *screening.PushupContent
}

func NewPuGatePhase(c *screening.PushupContent) *PuGatePhase {
	return &PuGatePhase{c: c}
}

func (PuGatePhase) State() state.StateKind { return state.StatePuGate }

func (p *PuGatePhase) Setup(ctx Context) Outcome {
	prog := ctx.User.Pushups
	if prog == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	if prog.GateIdx >= len(p.c.Gate.Questions) {
		return Outcome{NextState: state.StatePuGoal}
	}
	oc := scrText(p.c.Gate.Questions[prog.GateIdx].Text)
	oc.Keyboard = [][]string{{p.c.Gate.YesButton, p.c.Gate.NoButton}}
	return oc
}

func (p *PuGatePhase) Collect(ctx Context, input string) Outcome {
	prog := ctx.User.Pushups
	if prog == nil || prog.GateIdx >= len(p.c.Gate.Questions) {
		return Outcome{NextState: state.StatePuGoal}
	}
	q := p.c.Gate.Questions[prog.GateIdx]

	switch in := normText(input); {
	case labelIs(in, p.c.Gate.NoButton):
		// Re-firing the same state advances the cursor and asks the next
		// question (or hands over to the goal question).
		return Outcome{
			NextState: state.StatePuGate,
			Mutate: func(u *state.UserData) {
				mutatePuProgram(u, func(pr *state.PushupProgram) { pr.GateIdx++ })
			},
		}
	case labelIs(in, p.c.Gate.YesButton):
		switch q.OnYes {
		case screening.PushupGateStop:
			oc := scrText(p.c.Gate.StopScreen)
			oc.RemoveKeyboard = true
			oc.NextState = testExitState
			oc.Mutate = puWipeTrack
			return oc
		case screening.PushupGateEasier:
			oc := scrText(p.c.Gate.EasierNote)
			oc.NextState = state.StatePuGate
			oc.Mutate = func(u *state.UserData) {
				mutatePuProgram(u, func(pr *state.PushupProgram) {
					pr.GateIdx++
					if pr.StartStepDown < p.c.Params.JointPainStepDown {
						pr.StartStepDown = p.c.Params.JointPainStepDown
					}
				})
			}
			return oc
		default: // doctor_note
			oc := scrText(p.c.Gate.DoctorNote)
			oc.NextState = state.StatePuGate
			oc.Mutate = func(u *state.UserData) {
				mutatePuProgram(u, func(pr *state.PushupProgram) {
					pr.GateIdx++
					// Whatever rung is picked, land on the easiest one: the
					// ladder shift clamps at its lower end.
					pr.StartStepDown = len(p.c.Variations.Ladder)
				})
			}
			return oc
		}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (PuGatePhase) Remind(ctx Context) Outcome { return Outcome{} }

// PuRedCardPhase owns StatePuRedCard — the red-flag check-in shown before
// the first session of every week (never before the very first session: the
// entry gate has just run). It fires on FACTS, not on a score, which is why
// it is a fixed list of four signs rather than a questionnaire:
//
//   - "yes, one of these" → the card body (see a doctor today, stop
//     training) and the track goes quiet: the open session is dropped and
//     the due ping is switched off until the user comes back on their own.
//   - "my shoulder/elbow/wrist hurts" → one rung easier, base untouched, and
//     the session starts anyway — pain changes the leverage, not the plan.
//   - "nothing like that" → straight into the session.
type PuRedCardPhase struct {
	c *screening.PushupContent
}

func NewPuRedCardPhase(c *screening.PushupContent) *PuRedCardPhase {
	return &PuRedCardPhase{c: c}
}

func (PuRedCardPhase) State() state.StateKind { return state.StatePuRedCard }

func (p *PuRedCardPhase) Setup(ctx Context) Outcome {
	if ctx.User.Pushups == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	lines := make([]string, 0, len(p.c.RedCard.Signs)+1)
	lines = append(lines, p.c.RedCard.Prompt)
	for _, s := range p.c.RedCard.Signs {
		lines = append(lines, "• "+s)
	}
	oc := scrText(strings.Join(lines, "\n"))
	oc.Keyboard = [][]string{
		{p.c.RedCard.YesButton},
		{p.c.RedCard.JointPainPrompt},
		{p.c.RedCard.NoButton},
	}
	return oc
}

func (p *PuRedCardPhase) Collect(ctx Context, input string) Outcome {
	if ctx.User.Pushups == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	now := ctx.Now()
	switch in := normText(input); {
	case labelIs(in, p.c.RedCard.YesButton):
		oc := scrText(p.c.RedCard.Body)
		oc.RemoveKeyboard = true
		oc.NextState = testExitState
		oc.Mutate = func(u *state.UserData) {
			u.PuSession = nil
			mutatePuProgram(u, func(pr *state.PushupProgram) {
				// Paused: no due pings until the user opens the track again.
				pr.NextDueAt = time.Time{}
				pr.DuePingSent = false
			})
		}
		return oc
	case labelIs(in, p.c.RedCard.JointPainPrompt):
		begin := puBeginWorkout(p.c, ctx.User.Pushups, now)
		oc := scrText(p.c.RedCard.JointPainReply)
		oc.NextState = begin.NextState
		oc.Mutate = func(u *state.UserData) {
			mutatePuProgram(u, func(pr *state.PushupProgram) {
				pr.Variation = p.c.ShiftVariation(pr.Variation, -1)
			})
			begin.Mutate(u)
		}
		return oc
	case labelIs(in, p.c.RedCard.NoButton):
		return puBeginWorkout(p.c, ctx.User.Pushups, now)
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (PuRedCardPhase) Remind(ctx Context) Outcome { return Outcome{} }
