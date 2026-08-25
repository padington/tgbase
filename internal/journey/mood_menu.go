package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodMenuPhase owns StateMoodMenu — the mood-module mini-menu shown after
// the (one-time) consent: one button per instrument (⚡ WHO-5 quick check /
// 📋 PHQ-9 / 😰 GAD-7), contextual «▶️ Продолжить…» rows for unfinished
// runs on top, and the 🏠 home row (the runner intercepts that tap before
// dispatch, like everywhere else).
//
// An instrument button always starts a FRESH run: with an unfinished run of
// that same instrument the resume row sits directly above, so tapping the
// plain button is an explicit start-over (the v1 resume gate's «Начать
// заново», folded into one keyboard).
type MoodMenuPhase struct {
	c *screening.MoodContent
}

func NewMoodMenuPhase(c *screening.MoodContent) *MoodMenuPhase {
	return &MoodMenuPhase{c: c}
}

func (MoodMenuPhase) State() state.StateKind { return state.StateMoodMenu }

func (p *MoodMenuPhase) Setup(ctx Context) Outcome {
	if !hasMoodConsent(ctx.User) {
		return Outcome{NextState: state.StateMoodConsent}
	}
	m := p.c.Module
	oc := scrText("🌤 " + m.Meta.Title + "\n" + m.Menu.Prompt)
	var kb [][]string
	// Contextual resume rows first — the most likely next action on top.
	if ctx.User.Mood != nil {
		kb = append(kb, []string{m.Menu.ResumePhq9Button})
	}
	if ctx.User.Who5 != nil {
		kb = append(kb, []string{m.Menu.ResumeWho5Button})
	}
	if ctx.User.Gad7 != nil {
		kb = append(kb, []string{m.Menu.ResumeGad7Button})
	}
	kb = append(kb,
		[]string{m.Menu.Who5Button},
		[]string{m.Menu.Phq9Button},
		[]string{m.Menu.Gad7Button},
		[]string{homeLabel(ctx)},
	)
	oc.Keyboard = kb
	return oc
}

func (p *MoodMenuPhase) Collect(ctx Context, input string) Outcome {
	m := p.c.Module.Menu
	now := ctx.Now()
	switch in := normText(input); {
	case ctx.User.Mood != nil && labelIs(in, m.ResumePhq9Button):
		// Straight back to the recorded position (the crisis card or the
		// functional question when the run paused there); the question phase
		// derives the index from the recorded answers otherwise.
		return Outcome{NextState: phq9ResumeTarget(ctx.User.Mood)}
	case ctx.User.Who5 != nil && labelIs(in, m.ResumeWho5Button):
		return Outcome{NextState: state.StateMoodWho5Question}
	case ctx.User.Gad7 != nil && labelIs(in, m.ResumeGad7Button):
		return Outcome{NextState: state.StateMoodGad7Question}
	case labelIs(in, m.Who5Button):
		return Outcome{
			NextState: state.StateMoodWho5Question,
			Mutate: func(u *state.UserData) {
				u.Who5 = &state.MoodProgress{StartedAt: now}
			},
		}
	case labelIs(in, m.Phq9Button):
		return Outcome{
			NextState: state.StateMoodQuestion,
			Mutate: func(u *state.UserData) {
				u.Mood = &state.MoodProgress{StartedAt: now}
			},
		}
	case labelIs(in, m.Gad7Button):
		return Outcome{
			NextState: state.StateMoodGad7Question,
			Mutate: func(u *state.UserData) {
				u.Gad7 = &state.MoodProgress{StartedAt: now}
			},
		}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (MoodMenuPhase) Remind(ctx Context) Outcome { return Outcome{} }
