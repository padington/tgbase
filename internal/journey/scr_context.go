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

// ScrDomainsPhase walks the five life domains (criteria C/D shaped, bot's
// own wording) ONE AT A TIME: each domain is a single short message — title,
// position line, an "e.g.:" line with 2–3 examples, and a yes/no question.
// One struct, registered twice: adulthood and childhood (5–12 years). The
// domain index is driven by the AdultDomainIdx / ChildDomainIdx cursor
// (answered count, yes AND no), following the ASRS/WURS index pattern; only
// the ids answered "yes" are kept — the shape the result stores. The last
// answer of the childhood pass finalizes the whole screening.
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

// idx returns this pass's cursor: how many domains were answered so far.
func (p *ScrDomainsPhase) idx(s *state.ScreeningProgress) int {
	if p.childhood {
		return s.ChildDomainIdx
	}
	return s.AdultDomainIdx
}

func (p *ScrDomainsPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	d := p.c.Module.Domains
	i := p.idx(s)
	if i >= len(d.Items) {
		// All domains of this pass answered (stale keyboard tap / resume
		// after the last answer): move on without re-asking.
		if p.childhood {
			return p.finalize(ctx, s.ChildDomains)
		}
		return Outcome{NextState: state.StateScrDomainsChild}
	}
	prompt, pos := d.AdultPrompt, d.PositionAdult
	if p.childhood {
		prompt, pos = d.ChildhoodPrompt, d.PositionChild
	}
	item := d.Items[i]
	text := ""
	if i == 0 {
		text = prompt + "\n\n"
	}
	text += renderContent(pos, map[string]string{
		"current": strconv.Itoa(i + 1),
		"total":   strconv.Itoa(len(d.Items)),
	}) + "\n" + p.title(item) + "\n" +
		renderContent(d.ExamplesLine, map[string]string{
			"examples": strings.Join(p.examples(item), ", "),
		}) + "\n\n" + d.Question
	oc := scrText(text)
	oc.Keyboard = [][]string{{d.YesButton, d.NoButton}}
	return oc
}

func (p *ScrDomainsPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	d := p.c.Module.Domains
	i := p.idx(s)
	if i >= len(d.Items) {
		return Outcome{NextState: p.State()} // re-run the Setup guards
	}

	var yes bool
	switch in := normText(input); {
	case labelIs(in, d.YesButton):
		yes = true
	case labelIs(in, d.NoButton):
		yes = false
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}

	id := d.Items[i].ID
	childhood := p.childhood
	advance := func(u *state.UserData) {
		if u.Screening == nil {
			return
		}
		sc := u.Screening.Clone()
		if childhood {
			sc.ChildDomainIdx = i + 1
			if yes {
				sc.ChildDomains = append(sc.ChildDomains, id)
			}
		} else {
			sc.AdultDomainIdx = i + 1
			if yes {
				sc.AdultDomains = append(sc.AdultDomains, id)
			}
		}
		u.Screening = sc
	}

	if i+1 < len(d.Items) {
		return Outcome{NextState: p.State(), Mutate: advance} // next domain
	}
	if !p.childhood {
		return Outcome{NextState: state.StateScrDomainsChild, Mutate: advance}
	}
	// Last childhood answer — finalize with the final list (the just-given
	// answer is not in ctx.User yet, so assemble it locally).
	childDomains := append([]string(nil), s.ChildDomains...)
	if yes {
		childDomains = append(childDomains, id)
	}
	return p.finalize(ctx, childDomains)
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
