package journey

import (
	"strconv"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// asrsPart selects which half of the ASRS checklist a ScrAsrsPhase serves.
type asrsPart int

const (
	asrsPartA asrsPart = iota
	asrsPartB
)

// ScrAsrsPhase is the index-driven question phase for ASRS v1.1. One struct,
// registered twice: part A (questions 1–6, official Russian text) and part B
// (questions 7–18, unofficial translation). The question index is derived
// from len(Screening.AsrsAnswers) — there is no separate cursor to drift.
type ScrAsrsPhase struct {
	c    *screening.Content
	part asrsPart
}

func NewScrAsrsAPhase(c *screening.Content) *ScrAsrsPhase {
	return &ScrAsrsPhase{c: c, part: asrsPartA}
}

func NewScrAsrsBPhase(c *screening.Content) *ScrAsrsPhase {
	return &ScrAsrsPhase{c: c, part: asrsPartB}
}

func (p *ScrAsrsPhase) State() state.StateKind {
	if p.part == asrsPartA {
		return state.StateScrAsrsA
	}
	return state.StateScrAsrsB
}

func (p *ScrAsrsPhase) items() []screening.AsrsItem {
	if p.part == asrsPartA {
		return p.c.ASRS.PartA.Items
	}
	return p.c.ASRS.PartB.Items
}

func (p *ScrAsrsPhase) offset() int {
	if p.part == asrsPartA {
		return 0
	}
	return len(p.c.ASRS.PartA.Items)
}

func (p *ScrAsrsPhase) gate() state.StateKind {
	if p.part == asrsPartA {
		return state.StateScrAsrsAGate
	}
	return state.StateScrAsrsBGate
}

// header renders the block heading shown above question 1 of the part.
func (p *ScrAsrsPhase) header() string {
	ins := p.c.Module.Results.Instruments
	if p.part == asrsPartA {
		// Official WHO instruction: only the first sentence — the second one
		// is tied to the paper form (per the content note).
		return "📋 " + ins.AsrsA.Title + "\n" + screening.FirstSentence(p.c.ASRS.Instruction)
	}
	// Short unofficial-translation caveat — the fixed attribution line of
	// the results block, not the long translation_note.
	return "📋 " + ins.AsrsB.Title + "\n" + ins.AsrsB.Attribution
}

func (p *ScrAsrsPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	li := len(s.AsrsAnswers) - p.offset()
	if li < 0 {
		// Part B entered before part A finished — bounce back.
		return Outcome{NextState: state.StateScrAsrsA}
	}
	items := p.items()
	if li >= len(items) {
		return Outcome{NextState: p.gate()}
	}
	text := ""
	if li == 0 {
		text = p.header() + "\n\n"
	}
	text += progressLine(p.c, li+1, len(items)) + "\n" + items[li].Text
	oc := scrText(text)
	oc.Keyboard = scaleKeyboard(p.c.ASRS.Scale)
	return oc
}

func (p *ScrAsrsPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	li := len(s.AsrsAnswers) - p.offset()
	items := p.items()
	if li < 0 || li >= len(items) {
		// Stale position (e.g. tap on an outdated keyboard) — re-run the
		// Setup guards.
		return Outcome{NextState: p.State()}
	}
	score, ok := matchScale(p.c.ASRS.Scale, normText(input))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}
	next := p.State() // re-fire Setup for the next question (picker pattern)
	if li+1 >= len(items) {
		next = p.gate()
	}
	return Outcome{
		NextState: next,
		Mutate: func(u *state.UserData) {
			if u.Screening == nil {
				return
			}
			sc := u.Screening.Clone()
			sc.AsrsAnswers = append(sc.AsrsAnswers, score)
			u.Screening = sc
		},
	}
}

func (ScrAsrsPhase) Remind(ctx Context) Outcome { return Outcome{} }

// scrGateKind selects which block boundary a ScrGatePhase serves.
type scrGateKind int

const (
	gateAfterAsrsA scrGateKind = iota
	gateAfterAsrsB
	gateAfterWurs
)

// ScrGatePhase is the pause point between blocks. One struct, registered
// three times. The A gate shows the intermediate screener verdict (that part
// is a self-contained instrument); the B and WURS gates deliberately show no
// numbers — those belong to the final report only.
type ScrGatePhase struct {
	c    *screening.Content
	gate scrGateKind
}

func NewScrAsrsAGatePhase(c *screening.Content) *ScrGatePhase {
	return &ScrGatePhase{c: c, gate: gateAfterAsrsA}
}

func NewScrAsrsBGatePhase(c *screening.Content) *ScrGatePhase {
	return &ScrGatePhase{c: c, gate: gateAfterAsrsB}
}

func NewScrWursGatePhase(c *screening.Content) *ScrGatePhase {
	return &ScrGatePhase{c: c, gate: gateAfterWurs}
}

func (p *ScrGatePhase) State() state.StateKind {
	switch p.gate {
	case gateAfterAsrsA:
		return state.StateScrAsrsAGate
	case gateAfterAsrsB:
		return state.StateScrAsrsBGate
	default:
		return state.StateScrWursGate
	}
}

func (p *ScrGatePhase) next() state.StateKind {
	switch p.gate {
	case gateAfterAsrsA:
		return state.StateScrAsrsB
	case gateAfterAsrsB:
		return state.StateScrWursForm
	default:
		return state.StateScrOnset
	}
}

func (p *ScrGatePhase) Setup(ctx Context) Outcome {
	s := ctx.User.Screening
	if s == nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	ui := p.c.Module.UI
	var text string
	switch p.gate {
	case gateAfterAsrsA:
		sig, positive := p.c.ASRS.ScorePartA(s.AsrsAnswers)
		ib := p.c.Module.Results.Instruments.AsrsA
		line := ib.NegativeLine
		if positive {
			line = ib.PositiveLine
		}
		text = ib.Title + "\n" +
			renderContent(ib.ScoreLine, map[string]string{"score": strconv.Itoa(sig)}) + "\n" +
			line + "\n" + ib.Attribution +
			"\n\n" + ui.BlockBoundaries.AfterAsrsA
	case gateAfterAsrsB:
		text = ui.BlockBoundaries.AfterAsrsB
	default:
		text = ui.BlockBoundaries.AfterWurs
	}
	oc := scrText(text)
	oc.Keyboard = [][]string{{ui.ContinueButton}, {ui.PauseButton}}
	return oc
}

func (p *ScrGatePhase) Collect(ctx Context, input string) Outcome {
	ui := p.c.Module.UI
	in := normText(input)
	switch {
	case labelIs(in, ui.ContinueButton):
		return Outcome{NextState: p.next()}
	case labelIs(in, ui.PauseButton):
		gateState := p.State()
		oc := scrText(ui.Paused)
		oc.RemoveKeyboard = true
		oc.NextState = exitState(ctx.User)
		oc.Mutate = func(u *state.UserData) {
			if u.Screening != nil {
				sc := u.Screening.Clone()
				sc.ResumeState = gateState
				u.Screening = sc
			}
			u.ReturnState = ""
		}
		return oc
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (ScrGatePhase) Remind(ctx Context) Outcome { return Outcome{} }
