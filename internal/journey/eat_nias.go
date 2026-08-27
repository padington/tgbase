package journey

import (
	"strconv"
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// EatNiasPhase owns StateEatNiasQuestion — the index-driven series of the
// nine NIAS statements (6-option Likert). The index is derived from
// len(Nias.Answers). The result is read per subscale — there is no total —
// and a positive subscale is interpreted against the EDE-QS result by the
// deterministic Burton Murray rule (screening.NiasContext).
type EatNiasPhase struct {
	c *screening.EatingContent
}

func NewEatNiasPhase(c *screening.EatingContent) *EatNiasPhase {
	return &EatNiasPhase{c: c}
}

func (EatNiasPhase) State() state.StateKind { return state.StateEatNiasQuestion }

func (p *EatNiasPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Nias
	if s == nil {
		return Outcome{NextState: state.StateEatMenu}
	}
	items := p.c.NIAS.Items
	li := len(s.Answers)
	if li >= len(items) {
		// All statements answered (stale keyboard tap) — finalize.
		return niasFinalize(p.c, ctx, s.Answers)
	}
	text := ""
	if li == 0 {
		text = "🥄 " + p.c.Module.Nias.Title + "\n" + p.c.NIAS.Instruction + "\n\n"
	}
	text += niasProgressLine(p.c, li+1, len(items)) + "\n" + items[li].Text
	oc := scrText(text)
	oc.Keyboard = scaleKeyboard(p.c.NIAS.Scale)
	return oc
}

func (p *EatNiasPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Nias
	if s == nil {
		return Outcome{NextState: state.StateEatMenu}
	}
	items := p.c.NIAS.Items
	li := len(s.Answers)
	if li >= len(items) {
		return Outcome{NextState: state.StateEatNiasQuestion} // re-run the Setup guards
	}
	score, ok := matchScale(p.c.NIAS.Scale, normText(input))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}

	if li+1 >= len(items) {
		answers := append(append([]int(nil), s.Answers...), score)
		return niasFinalize(p.c, ctx, answers)
	}
	return Outcome{
		NextState: state.StateEatNiasQuestion,
		Mutate: func(u *state.UserData) {
			if u.Nias == nil {
				return
			}
			nc := u.Nias.Clone()
			nc.Answers = append(nc.Answers, score)
			u.Nias = nc
		},
	}
}

func (EatNiasPhase) Remind(ctx Context) Outcome { return Outcome{} }

// buildNiasResult converts the raw answers into the persisted result: the
// three subscale sums plus the cutoffs that were applied to them (copied
// from the content so the report stays honest if the content changes later).
func buildNiasResult(c *screening.EatingContent, answers []int, ctx Context) state.NiasResult {
	s := c.NIAS.Score(answers)
	res := state.NiasResult{
		TakenAt:        ctx.Now(),
		Picky:          s.Picky,
		Appetite:       s.Appetite,
		Fear:           s.Fear,
		PickyCutoff:    c.NIAS.Cutoff(screening.NiasPicky),
		AppetiteCutoff: c.NIAS.Cutoff(screening.NiasAppetite),
		FearCutoff:     c.NIAS.Cutoff(screening.NiasFear),
	}
	res.PickyPositive = res.Picky >= res.PickyCutoff
	res.AppetitePositive = res.Appetite >= res.AppetiteCutoff
	res.FearPositive = res.Fear >= res.FearCutoff
	return res
}

// niasSubscaleFacts returns the per-subscale (score, cutoff, positive) rows
// in the canonical rendering order.
func niasSubscaleFacts(res *state.NiasResult) []struct {
	ID       string
	Score    int
	Cutoff   int
	Positive bool
} {
	return []struct {
		ID       string
		Score    int
		Cutoff   int
		Positive bool
	}{
		{screening.NiasPicky, res.Picky, res.PickyCutoff, res.PickyPositive},
		{screening.NiasAppetite, res.Appetite, res.AppetiteCutoff, res.AppetitePositive},
		{screening.NiasFear, res.Fear, res.FearCutoff, res.FearPositive},
	}
}

// niasContextKey applies the Burton Murray reading rule to the stored
// results: a positive subscale means something different depending on
// whether the EDE-QS was taken and how it came out. Without an EDE-QS result
// the neutral wording is used — the distinction cannot be made.
func niasContextKey(res *state.NiasResult, edeqs *state.EdeqsResult) string {
	return screening.NiasContext(res.AnyPositive(), edeqs != nil, edeqs != nil && edeqs.Positive)
}

// niasResultMessage assembles the NIAS result: heading, one line per
// subscale with its own cutoff and verdict, the reading line when a subscale
// is positive, and the shared footer.
func niasResultMessage(c *screening.EatingContent, res *state.NiasResult, edeqs *state.EdeqsResult) string {
	r := c.Module.Nias.Results
	var b strings.Builder
	b.WriteString(eatResultHeading(c))
	b.WriteString("\n")
	for _, f := range niasSubscaleFacts(res) {
		verdict := r.VerdictBelow
		if f.Positive {
			verdict = r.VerdictAbove
		}
		b.WriteString("\n" + renderContent(r.SubscaleLine, map[string]string{
			"name":    c.Module.Nias.Subscales[f.ID],
			"score":   strconv.Itoa(f.Score),
			"cutoff":  strconv.Itoa(f.Cutoff),
			"verdict": verdict,
		}))
	}
	if key := niasContextKey(res, edeqs); key != screening.NiasCtxNone {
		if line, ok := r.Contexts[key]; ok {
			b.WriteString("\n\n" + line)
		}
	}
	b.WriteString(eatResultFooter(c, r.AttributionLine))
	return b.String()
}

// niasFinalize completes the picky-eating test: scores the subscales,
// persists the NiasResult and wipes the raw answers in the same Set, sends
// the result (with the Burton Murray reading), and routes to the report
// transit.
func niasFinalize(c *screening.EatingContent, ctx Context, answers []int) Outcome {
	res := buildNiasResult(c, answers, ctx)
	oc := scrText(niasResultMessage(c, &res, ctx.User.EdeqsResult))
	oc.RemoveKeyboard = true
	oc.NextState = state.StateEatReport
	oc.Mutate = func(u *state.UserData) {
		r := res
		u.NiasResult = &r
		u.Nias = nil
	}
	return oc
}
