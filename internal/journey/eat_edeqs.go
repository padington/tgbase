package journey

import (
	"strconv"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// EatEdeqsPhase owns StateEatEdeqsQuestion — the index-driven series of the
// twelve EDE-QS questions, one short message per question. The index is
// derived from len(Edeqs.Answers). The original form switches answer scales
// after item 10 (days → severity), so the keyboard and the block header
// switch with it.
type EatEdeqsPhase struct {
	c *screening.EatingContent
}

func NewEatEdeqsPhase(c *screening.EatingContent) *EatEdeqsPhase {
	return &EatEdeqsPhase{c: c}
}

func (EatEdeqsPhase) State() state.StateKind { return state.StateEatEdeqsQuestion }

func (p *EatEdeqsPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Edeqs
	if s == nil {
		return Outcome{NextState: state.StateEatMenu}
	}
	e := &p.c.EDEQS
	li := len(s.Answers)
	if li >= len(e.Items) {
		// All questions answered (stale keyboard tap) — finalize.
		return edeqsFinalize(p.c, ctx, s.Answers)
	}
	text := ""
	switch {
	case li == 0:
		// Block heading + instruction + the days header, shown once.
		text = "📋 " + p.c.Module.Edeqs.Title + "\n" + e.Instruction + "\n\n" + e.DaysHeader + "\n\n"
	case e.Items[li].Scale == screening.EdeqsScaleSeverity &&
		e.Items[li-1].Scale == screening.EdeqsScaleDays:
		// The form's second block starts here — announce the new framing
		// once, exactly where the paper form does.
		text = e.SeverityHeader + "\n\n"
	}
	text += eatProgressLine(p.c, li+1, len(e.Items)) + "\n" + e.Items[li].Text
	oc := scrText(text)
	oc.Keyboard = scaleKeyboard(e.ScaleFor(li))
	return oc
}

func (p *EatEdeqsPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Edeqs
	if s == nil {
		return Outcome{NextState: state.StateEatMenu}
	}
	e := &p.c.EDEQS
	li := len(s.Answers)
	if li >= len(e.Items) {
		return Outcome{NextState: state.StateEatEdeqsQuestion} // re-run the Setup guards
	}
	score, ok := matchScale(e.ScaleFor(li), normText(input))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}

	if li+1 >= len(e.Items) {
		// Last question — finalize with the final list (the just-given
		// answer is not in ctx.User yet).
		answers := append(append([]int(nil), s.Answers...), score)
		return edeqsFinalize(p.c, ctx, answers)
	}
	return Outcome{
		NextState: state.StateEatEdeqsQuestion,
		Mutate: func(u *state.UserData) {
			if u.Edeqs == nil {
				return
			}
			ec := u.Edeqs.Clone()
			ec.Answers = append(ec.Answers, score)
			u.Edeqs = ec
		},
	}
}

func (EatEdeqsPhase) Remind(ctx Context) Outcome { return Outcome{} }

// edeqsResultMessage assembles the EDE-QS result: heading, the 0–36 score
// with the applied cutoff, the reading (risk wording, no diagnosis labels),
// and the shared footer.
func edeqsResultMessage(c *screening.EatingContent, res *state.EdeqsResult) string {
	r := c.Module.Edeqs.Results
	band := screening.EdeqsBandBelow
	if res.Positive {
		band = screening.EdeqsBandAtOrAbove
	}
	return eatResultHeading(c) + "\n\n" +
		renderContent(r.ScoreLine, map[string]string{
			"score":  strconv.Itoa(res.Score),
			"cutoff": strconv.Itoa(res.Cutoff),
		}) + "\n" +
		edeqsBandLine(c, band) +
		eatResultFooter(c, r.AttributionLine)
}

// edeqsBandLine maps a reading-band id onto the content wording. Unknown ids
// render as-is so a content change never hides the fact.
func edeqsBandLine(c *screening.EatingContent, band string) string {
	if line, ok := c.Module.Edeqs.Results.Bands[band]; ok {
		return line
	}
	return band
}

// edeqsFinalize completes the core test: scores the answers, persists the
// EdeqsResult (with the cutoff that was applied) and wipes the raw answers
// in the same Set, sends the result, and routes to the report transit.
func edeqsFinalize(c *screening.EatingContent, ctx Context, answers []int) Outcome {
	score := c.EDEQS.Score(answers)
	res := state.EdeqsResult{
		TakenAt:  ctx.Now(),
		Score:    score,
		Cutoff:   c.EDEQS.Cutoff(),
		Positive: c.EDEQS.Positive(score),
	}
	oc := scrText(edeqsResultMessage(c, &res))
	oc.RemoveKeyboard = true
	oc.NextState = state.StateEatReport
	oc.Mutate = func(u *state.UserData) {
		r := res
		u.EdeqsResult = &r
		u.Edeqs = nil
	}
	return oc
}
