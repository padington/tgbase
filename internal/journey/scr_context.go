package journey

import (
	"strconv"
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// ScrOnsetPhase owns StateScrOnset — the bot's own criterion-B-shaped
// question: were similar difficulties noticeable before age 12?
type ScrOnsetPhase struct {
	c *screening.Content
}

func NewScrOnsetPhase(c *screening.Content) *ScrOnsetPhase { return &ScrOnsetPhase{c: c} }

func (ScrOnsetPhase) State() state.StateKind { return state.StateScrOnset }

func (p *ScrOnsetPhase) Setup(ctx Context) Outcome {
	if ctx.User.Screening == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	cb := p.c.Module.CriterionB
	oc := scrText("📋 " + cb.Title + "\n\n" + cb.Question)
	oc.Keyboard = [][]string{{cb.YesLabel}, {cb.NoLabel}}
	return oc
}

func (p *ScrOnsetPhase) Collect(ctx Context, input string) Outcome {
	cb := p.c.Module.CriterionB
	in := normText(input)
	set := func(v bool) func(*state.UserData) {
		return func(u *state.UserData) {
			if u.Screening == nil {
				return
			}
			sc := u.Screening.Clone()
			val := v
			sc.OnsetChild = &val
			u.Screening = sc
		}
	}
	switch {
	case labelIs(in, cb.YesLabel):
		return Outcome{NextState: state.StateScrDomainsAdult, Mutate: set(true)}
	case labelIs(in, cb.NoLabel):
		return Outcome{NextState: state.StateScrOnsetAge, Mutate: set(false)}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (ScrOnsetPhase) Remind(ctx Context) Outcome { return Outcome{} }

// ScrOnsetAgePhase owns StateScrOnsetAge — free-text follow-up after "no":
// at what age did the difficulties appear? The age is a report fact only, it
// does not affect scoring.
type ScrOnsetAgePhase struct {
	c *screening.Content
}

func NewScrOnsetAgePhase(c *screening.Content) *ScrOnsetAgePhase {
	return &ScrOnsetAgePhase{c: c}
}

func (ScrOnsetAgePhase) State() state.StateKind { return state.StateScrOnsetAge }

func (p *ScrOnsetAgePhase) Setup(ctx Context) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	if s.OnsetChild == nil {
		return Outcome{NextState: state.StateScrOnset}
	}
	oc := scrText(p.c.Module.CriterionB.NoFollowup)
	oc.RemoveKeyboard = true
	return oc
}

func (p *ScrOnsetAgePhase) Collect(ctx Context, input string) Outcome {
	age, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || age < 1 || age > 99 {
		return scrText(p.c.Module.CriterionB.AgeInvalid)
	}
	return Outcome{
		NextState: state.StateScrDomainsAdult,
		Mutate: func(u *state.UserData) {
			if u.Screening == nil {
				return
			}
			sc := u.Screening.Clone()
			sc.OnsetAge = age
			u.Screening = sc
		},
	}
}

func (ScrOnsetAgePhase) Remind(ctx Context) Outcome { return Outcome{} }

// ScrDomainsPhase is the multi-select of life domains (criteria C/D shaped,
// bot's own wording). One struct, registered twice: adulthood and childhood
// (5–12 years). Toggling redraws the message via the same-state re-Setup
// pattern; "Done" on the childhood pass finalizes the whole screening.
type ScrDomainsPhase struct {
	c         *screening.Content
	childhood bool
}

func NewScrDomainsAdultPhase(c *screening.Content) *ScrDomainsPhase {
	return &ScrDomainsPhase{c: c, childhood: false}
}

func NewScrDomainsChildPhase(c *screening.Content) *ScrDomainsPhase {
	return &ScrDomainsPhase{c: c, childhood: true}
}

func (p *ScrDomainsPhase) State() state.StateKind {
	if p.childhood {
		return state.StateScrDomainsChild
	}
	return state.StateScrDomainsAdult
}

func (p *ScrDomainsPhase) title(d screening.DomainItem) string {
	if p.childhood {
		return d.Childhood.Title
	}
	return d.Adult.Title
}

func (p *ScrDomainsPhase) examples(d screening.DomainItem) []string {
	if p.childhood {
		return d.Childhood.Examples
	}
	return d.Adult.Examples
}

func (p *ScrDomainsPhase) selected(s *state.ScreeningProgress) []string {
	if p.childhood {
		return s.ChildDomains
	}
	return s.AdultDomains
}

func (p *ScrDomainsPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	d := p.c.Module.Domains
	prompt := d.AdultPrompt
	if p.childhood {
		prompt = d.ChildhoodPrompt
	}
	sel := make(map[string]bool)
	for _, id := range p.selected(s) {
		sel[id] = true
	}
	var lines []string
	var rows [][]string
	for _, item := range d.Items {
		mark, btn := "▫️", p.title(item)
		if sel[item.ID] {
			mark, btn = "✅", "✅ "+p.title(item)
		}
		ex := p.examples(item)
		if len(ex) > 2 {
			ex = ex[:2] // two examples in the message; full lists stay in content
		}
		lines = append(lines, mark+" "+p.title(item)+" — "+strings.Join(ex, "; "))
		rows = append(rows, []string{btn})
	}
	if len(sel) == 0 {
		rows = append(rows, []string{d.NoneButton})
	} else {
		rows = append(rows, []string{d.DoneButton})
	}
	oc := scrText(prompt + "\n\n" + strings.Join(lines, "\n") + "\n\n" + d.MultiselectHint)
	oc.Keyboard = rows
	return oc
}

func (p *ScrDomainsPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	d := p.c.Module.Domains
	in := normText(input)

	for _, item := range d.Items {
		t := p.title(item)
		if labelIs(in, t) || labelIs(in, "✅ "+t) {
			id := item.ID
			childhood := p.childhood
			return Outcome{
				NextState: p.State(), // re-fire Setup to redraw marks
				Mutate: func(u *state.UserData) {
					if u.Screening == nil {
						return
					}
					sc := u.Screening.Clone()
					if childhood {
						sc.ChildDomains = toggleDomain(sc.ChildDomains, id)
					} else {
						sc.AdultDomains = toggleDomain(sc.AdultDomains, id)
					}
					u.Screening = sc
				},
			}
		}
	}

	switch {
	case labelIs(in, d.NoneButton):
		if p.childhood {
			return p.finalize(ctx, nil)
		}
		return Outcome{
			NextState: state.StateScrDomainsChild,
			Mutate: func(u *state.UserData) {
				if u.Screening == nil {
					return
				}
				sc := u.Screening.Clone()
				sc.AdultDomains = nil
				u.Screening = sc
			},
		}
	case labelIs(in, d.DoneButton):
		if p.childhood {
			return p.finalize(ctx, s.ChildDomains)
		}
		return Outcome{NextState: state.StateScrDomainsChild}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

// finalize completes the screening: scores everything, persists the
// ScreeningResult and wipes the raw answers in the same Set (no window where
// both are on disk), and sends the summary message. The referral and
// doctor-report messages follow via the two Setup-only phases.
func (p *ScrDomainsPhase) finalize(ctx Context, childDomains []string) Outcome {
	s := ctx.User.Screening
	res := buildScreeningResult(p.c, s, childDomains, ctx.Now())
	oc := scrText(resultMessage(p.c, &res))
	oc.RemoveKeyboard = true
	oc.NextState = state.StateScrReferral
	oc.Mutate = func(u *state.UserData) {
		r := res
		u.ScreeningResult = &r
		u.Screening = nil
	}
	return oc
}

func (ScrDomainsPhase) Remind(ctx Context) Outcome { return Outcome{} }

// toggleDomain flips the presence of id in ids, preserving order.
func toggleDomain(ids []string, id string) []string {
	for i, v := range ids {
		if v == id {
			return append(append([]string(nil), ids[:i]...), ids[i+1:]...)
		}
	}
	return append(append([]string(nil), ids...), id)
}
