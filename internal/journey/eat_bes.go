package journey

import (
	"strconv"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// EatBesPhase owns StateEatBesQuestion — the index-driven series of the 16
// BES statement groups. Unlike the scale-based instruments, each group has
// its own 3–4 weighted statements, far too long for keyboard buttons: the
// message carries them as a numbered list and the keyboard carries the
// numbers. The recorded answer is the picked statement's WEIGHT (several
// statements of a group can share one), so scoring stays a plain sum.
type EatBesPhase struct {
	c *screening.EatingContent
}

func NewEatBesPhase(c *screening.EatingContent) *EatBesPhase {
	return &EatBesPhase{c: c}
}

func (EatBesPhase) State() state.StateKind { return state.StateEatBesQuestion }

func (p *EatBesPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Bes
	if s == nil {
		return Outcome{NextState: state.StateEatMenu}
	}
	items := p.c.BES.Items
	li := len(s.Answers)
	if li >= len(items) {
		// All groups answered (stale keyboard tap) — finalize.
		return besFinalize(p.c, ctx, s.Answers)
	}
	text := ""
	if li == 0 {
		// Block heading + instruction, shown once above the first group.
		text = "🍩 " + p.c.Module.Bes.Title + "\n" + p.c.BES.Instruction + "\n\n"
	}
	text += besProgressLine(p.c, li+1, len(items)) + "\n" +
		numberedStatements(items[li].Statements) + "\n\n" + p.c.Module.Bes.PickHint
	oc := scrText(text)
	oc.Keyboard = numberKeyboard(len(items[li].Statements))
	return oc
}

func (p *EatBesPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Bes
	if s == nil {
		return Outcome{NextState: state.StateEatMenu}
	}
	items := p.c.BES.Items
	li := len(s.Answers)
	if li >= len(items) {
		return Outcome{NextState: state.StateEatBesQuestion} // re-run the Setup guards
	}
	idx, ok := matchNumber(normText(input), len(items[li].Statements))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}
	score := items[li].Statements[idx].Score

	if li+1 >= len(items) {
		// Last group — finalize with the final list (the just-given answer
		// is not in ctx.User yet).
		answers := append(append([]int(nil), s.Answers...), score)
		return besFinalize(p.c, ctx, answers)
	}
	return Outcome{
		NextState: state.StateEatBesQuestion,
		Mutate: func(u *state.UserData) {
			if u.Bes == nil {
				return
			}
			bc := u.Bes.Clone()
			bc.Answers = append(bc.Answers, score)
			u.Bes = bc
		},
	}
}

func (EatBesPhase) Remind(ctx Context) Outcome { return Outcome{} }

// besBandLine maps the stored band id onto the content wording. Unknown ids
// render as-is so a content change never hides the fact.
func besBandLine(c *screening.EatingContent, band string) string {
	if line, ok := c.Module.Bes.Results.Bands[band]; ok {
		return line
	}
	return band
}

// besResultMessage assembles the BES result: heading, the 0–46 score, the
// severity wording (no diagnosis labels) and the shared footer.
func besResultMessage(c *screening.EatingContent, res *state.BesResult) string {
	r := c.Module.Bes.Results
	return eatResultHeading(c) + "\n\n" +
		renderContent(r.ScoreLine, map[string]string{"score": strconv.Itoa(res.Score)}) + "\n" +
		besBandLine(c, res.Band) +
		eatResultFooter(c, r.AttributionLine)
}

// besFinalize completes the overeating test: scores the answers, persists
// the BesResult and wipes the raw answers in the same Set, sends the result,
// and routes to the report transit.
func besFinalize(c *screening.EatingContent, ctx Context, answers []int) Outcome {
	score := c.BES.Score(answers)
	res := state.BesResult{TakenAt: ctx.Now(), Score: score, Band: c.BES.Band(score)}
	oc := scrText(besResultMessage(c, &res))
	oc.RemoveKeyboard = true
	oc.NextState = state.StateEatReport
	oc.Mutate = func(u *state.UserData) {
		r := res
		u.BesResult = &r
		u.Bes = nil
	}
	return oc
}
