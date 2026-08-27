package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// EatMenuPhase owns StateEatMenu — the eating-track mini-menu shown after
// the (one-time) consent: one button per instrument (📋 core test / 🍩
// overeating / 🥄 picky eating), contextual «▶️ Продолжить…» rows for
// unfinished runs on top, and the 🏠 home row (the runner intercepts that
// tap before dispatch, like everywhere else).
//
// An instrument button always starts a FRESH run: with an unfinished run of
// that same instrument the resume row sits directly above, so tapping the
// plain button is an explicit start-over — same contract as the mood menu.
type EatMenuPhase struct {
	c *screening.EatingContent
}

func NewEatMenuPhase(c *screening.EatingContent) *EatMenuPhase {
	return &EatMenuPhase{c: c}
}

func (EatMenuPhase) State() state.StateKind { return state.StateEatMenu }

func (p *EatMenuPhase) Setup(ctx Context) Outcome {
	if !hasEatConsent(ctx.User) {
		return Outcome{NextState: state.StateEatConsent}
	}
	m := p.c.Module
	oc := scrText("🍽 " + m.Meta.Title + "\n" + m.Menu.Prompt)
	var kb [][]string
	// Contextual resume rows first — the most likely next action on top.
	if ctx.User.Edeqs != nil {
		kb = append(kb, []string{m.Menu.ResumeEdeqsButton})
	}
	if ctx.User.Bes != nil {
		kb = append(kb, []string{m.Menu.ResumeBesButton})
	}
	if ctx.User.Nias != nil {
		kb = append(kb, []string{m.Menu.ResumeNiasButton})
	}
	kb = append(kb,
		[]string{m.Menu.EdeqsButton},
		[]string{m.Menu.BesButton},
		[]string{m.Menu.NiasButton},
		[]string{homeLabel(ctx)},
	)
	oc.Keyboard = kb
	return oc
}

func (p *EatMenuPhase) Collect(ctx Context, input string) Outcome {
	m := p.c.Module.Menu
	now := ctx.Now()
	// startFresh wipes the instrument's previous run — the visible resume
	// row above the button makes this an explicit start-over.
	startFresh := func(next state.StateKind, set func(*state.UserData, *state.EatingProgress)) Outcome {
		return Outcome{
			NextState: next,
			Mutate: func(u *state.UserData) {
				set(u, &state.EatingProgress{StartedAt: now})
			},
		}
	}
	switch in := normText(input); {
	case ctx.User.Edeqs != nil && labelIs(in, m.ResumeEdeqsButton):
		return Outcome{NextState: state.StateEatEdeqsQuestion}
	case ctx.User.Bes != nil && labelIs(in, m.ResumeBesButton):
		return Outcome{NextState: state.StateEatBesQuestion}
	case ctx.User.Nias != nil && labelIs(in, m.ResumeNiasButton):
		return Outcome{NextState: state.StateEatNiasQuestion}
	case labelIs(in, m.EdeqsButton):
		return startFresh(state.StateEatEdeqsQuestion,
			func(u *state.UserData, r *state.EatingProgress) { u.Edeqs = r })
	case labelIs(in, m.BesButton):
		return startFresh(state.StateEatBesQuestion,
			func(u *state.UserData, r *state.EatingProgress) { u.Bes = r })
	case labelIs(in, m.NiasButton):
		return startFresh(state.StateEatNiasQuestion,
			func(u *state.UserData, r *state.EatingProgress) { u.Nias = r })
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (EatMenuPhase) Remind(ctx Context) Outcome { return Outcome{} }
