package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// ScrWursFormPhase owns StateScrWursForm. WURS-25 exists as masculine and
// feminine blanks (text_m / text_f), so the bot asks which wording to use.
// The question is a bot string (i18n), not instrument content; the choice is
// transient — kept only in ScreeningProgress, never in ScreeningResult.
type ScrWursFormPhase struct {
	c *screening.Content
}

func NewScrWursFormPhase(c *screening.Content) *ScrWursFormPhase {
	return &ScrWursFormPhase{c: c}
}

func (ScrWursFormPhase) State() state.StateKind { return state.StateScrWursForm }

func (p *ScrWursFormPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	if s.WursForm != "" {
		return Outcome{NextState: state.StateScrWurs}
	}
	return Outcome{
		ReplyKey: "scr.wurs.form.prompt",
		Keyboard: [][]string{
			{ctx.Trans.T("button.scr.form.m", ctx.Locale, nil)},
			{ctx.Trans.T("button.scr.form.f", ctx.Locale, nil)},
		},
	}
}

func (p *ScrWursFormPhase) Collect(ctx Context, input string) Outcome {
	in := normText(input)
	var form string
	switch {
	case labelIs(in, ctx.Trans.T("button.scr.form.m", ctx.Locale, nil)):
		form = "m"
	case labelIs(in, ctx.Trans.T("button.scr.form.f", ctx.Locale, nil)):
		form = "f"
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
	return Outcome{
		NextState: state.StateScrWurs,
		Mutate: func(u *state.UserData) {
			if u.Screening == nil {
				return
			}
			sc := u.Screening.Clone()
			sc.WursForm = form
			u.Screening = sc
		},
	}
}

func (ScrWursFormPhase) Remind(ctx Context) Outcome { return Outcome{} }

// ScrWursPhase owns StateScrWurs — the 25 WURS items in the chosen wording,
// index-driven off len(Screening.WursAnswers).
type ScrWursPhase struct {
	c *screening.Content
}

func NewScrWursPhase(c *screening.Content) *ScrWursPhase { return &ScrWursPhase{c: c} }

func (ScrWursPhase) State() state.StateKind { return state.StateScrWurs }

func (p *ScrWursPhase) itemText(item screening.WursItem, form string) string {
	if form == "f" {
		return item.TextF
	}
	return item.TextM
}

func (p *ScrWursPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	items := p.c.WURS.Items
	li := len(s.WursAnswers)
	if li >= len(items) {
		return Outcome{NextState: state.StateScrWursGate}
	}
	if s.WursForm == "" {
		return Outcome{NextState: state.StateScrWursForm}
	}
	text := ""
	if li == 0 {
		// Block heading + instruction, both from content.
		text = "📋 " + p.c.Module.Results.Instruments.Wurs.Title + "\n" +
			p.c.WURS.Instruction + "\n\n"
	}
	text += progressLine(p.c, li+1, len(items)) + "\n" + p.itemText(items[li], s.WursForm)
	oc := scrText(text)
	oc.Keyboard = scaleKeyboard(p.c.WURS.Scale)
	return oc
}

func (p *ScrWursPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	items := p.c.WURS.Items
	li := len(s.WursAnswers)
	if li >= len(items) || s.WursForm == "" {
		return Outcome{NextState: state.StateScrWurs} // re-run Setup guards
	}
	score, ok := matchScale(p.c.WURS.Scale, normText(input))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}
	next := state.StateScrWurs
	if li+1 >= len(items) {
		next = state.StateScrWursGate
	}
	return Outcome{
		NextState: next,
		Mutate: func(u *state.UserData) {
			if u.Screening == nil {
				return
			}
			sc := u.Screening.Clone()
			sc.WursAnswers = append(sc.WursAnswers, score)
			u.Screening = sc
		},
	}
}

func (ScrWursPhase) Remind(ctx Context) Outcome { return Outcome{} }
