package journey

import (
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// PuGoalPhase owns StatePuGoal — the single goal question, asked once,
// before the first test. It is not cosmetic: the goal branches what happens
// at a plateau for the whole life of the track (more reps → more sets and
// shorter rests; strength and form → a harder rung later), and mixing the
// two logics is exactly what makes a program stall.
type PuGoalPhase struct {
	c *screening.PushupContent
}

func NewPuGoalPhase(c *screening.PushupContent) *PuGoalPhase {
	return &PuGoalPhase{c: c}
}

func (PuGoalPhase) State() state.StateKind { return state.StatePuGoal }

func (p *PuGoalPhase) Setup(ctx Context) Outcome {
	if ctx.User.Pushups == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	oc := scrText(p.c.Goal.Prompt)
	oc.Keyboard = [][]string{{p.c.Goal.RepsButton}, {p.c.Goal.StrengthButton}}
	return oc
}

func (p *PuGoalPhase) Collect(ctx Context, input string) Outcome {
	pick := func(goal string) Outcome {
		// Silent: the picked goal's one-line consequence is the first line
		// of the variation screen, so the tap costs one message, not two.
		return Outcome{
			NextState: state.StatePuVariation,
			Mutate: func(u *state.UserData) {
				mutatePuProgram(u, func(pr *state.PushupProgram) { pr.Goal = goal })
			},
		}
	}
	switch in := normText(input); {
	case labelIs(in, p.c.Goal.RepsButton):
		return pick(screening.PushupGoalReps)
	case labelIs(in, p.c.Goal.StrengthButton):
		return pick(screening.PushupGoalStrength)
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (PuGoalPhase) Remind(ctx Context) Outcome { return Outcome{} }

// PuVariationPhase owns StatePuVariation — picking the rung of the ladder
// the track starts on. Only the rungs marked `offered` are shown (v1 keeps
// the harder half of the ladder for regressions and for v2's progressions);
// each is described in words — "easier / harder", never as a share of body
// weight, which would mean asking for a weight this bot must never ask for.
//
// A safety-gate answer can bias the start: StartStepDown moves the pick that
// many rungs down (joint pain, pregnancy) and is consumed here.
type PuVariationPhase struct {
	c *screening.PushupContent
}

func NewPuVariationPhase(c *screening.PushupContent) *PuVariationPhase {
	return &PuVariationPhase{c: c}
}

func (PuVariationPhase) State() state.StateKind { return state.StatePuVariation }

func (p *PuVariationPhase) Setup(ctx Context) Outcome {
	prog := ctx.User.Pushups
	if prog == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	offered := p.c.OfferedVariations()
	lines := make([]string, 0, len(offered)+3)
	if note := p.goalNote(prog.Goal); note != "" {
		lines = append(lines, note)
	}
	lines = append(lines, p.c.Variations.Prompt, p.c.Variations.Hint)
	for _, v := range offered {
		lines = append(lines, "• "+v.Name+" — "+v.Hint)
	}
	oc := scrText(strings.Join(lines, "\n"))
	kb := make([][]string, 0, len(offered))
	for _, v := range offered {
		kb = append(kb, []string{v.Name})
	}
	oc.Keyboard = kb
	return oc
}

// goalNote is the one-line consequence of the goal picked a message earlier.
func (p *PuVariationPhase) goalNote(goal string) string {
	switch goal {
	case screening.PushupGoalReps:
		return p.c.Goal.RepsNote
	case screening.PushupGoalStrength:
		return p.c.Goal.StrengthNote
	default:
		return ""
	}
}

func (p *PuVariationPhase) Collect(ctx Context, input string) Outcome {
	in := normText(input)
	for _, v := range p.c.OfferedVariations() {
		if !labelIs(in, v.Name) {
			continue
		}
		id := v.ID
		return Outcome{
			NextState: state.StatePuTest,
			Mutate: func(u *state.UserData) {
				mutatePuProgram(u, func(pr *state.PushupProgram) {
					pr.Variation = p.c.ShiftVariation(id, -pr.StartStepDown)
					pr.StartStepDown = 0
				})
			},
		}
	}
	return Outcome{ReplyKey: "scr.invalid_button"}
}

func (PuVariationPhase) Remind(ctx Context) Outcome { return Outcome{} }
