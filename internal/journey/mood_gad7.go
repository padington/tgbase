package journey

import (
	"strconv"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodGad7Phase owns StateMoodGad7Question — the index-driven series of the
// seven GAD-7 questions, one short message per question ("Question N of 7" +
// the official item text + the 4-option scale keyboard). The index is
// derived from len(Gad7.Answers). GAD-7 has no crisis item — no crisis
// logic here by design.
type MoodGad7Phase struct {
	c *screening.MoodContent
}

func NewMoodGad7Phase(c *screening.MoodContent) *MoodGad7Phase {
	return &MoodGad7Phase{c: c}
}

func (MoodGad7Phase) State() state.StateKind { return state.StateMoodGad7Question }

func (p *MoodGad7Phase) Setup(ctx Context) Outcome {
	s := ctx.User.Gad7
	if s == nil {
		return Outcome{NextState: state.StateMoodMenu}
	}
	items := p.c.GAD7.Items
	li := len(s.Answers)
	if li >= len(items) {
		// All questions answered (stale keyboard tap) — finalize.
		return gad7Finalize(p.c, ctx, s.Answers)
	}
	text := ""
	if li == 0 {
		// Block heading + the official instruction, shown once above q1.
		text = "😰 " + p.c.Module.Gad7.Title + "\n" + p.c.GAD7.Instruction + "\n\n"
	}
	text += moodProgressLine(p.c, li+1, len(items)) + "\n" + items[li].Text
	oc := scrText(text)
	oc.Keyboard = scaleKeyboard(p.c.GAD7.Scale)
	return oc
}

func (p *MoodGad7Phase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Gad7
	if s == nil {
		return Outcome{NextState: state.StateMoodMenu}
	}
	items := p.c.GAD7.Items
	li := len(s.Answers)
	if li >= len(items) {
		return Outcome{NextState: state.StateMoodGad7Question} // re-run the Setup guards
	}
	score, ok := matchScale(p.c.GAD7.Scale, normText(input))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}

	if li+1 >= len(items) {
		// Last question — finalize with the final list (the just-given
		// answer is not in ctx.User yet).
		answers := append(append([]int(nil), s.Answers...), score)
		return gad7Finalize(p.c, ctx, answers)
	}
	return Outcome{
		NextState: state.StateMoodGad7Question,
		Mutate: func(u *state.UserData) {
			if u.Gad7 == nil {
				return
			}
			mc := u.Gad7.Clone()
			mc.Answers = append(mc.Answers, score)
			u.Gad7 = mc
		},
	}
}

func (MoodGad7Phase) Remind(ctx Context) Outcome { return Outcome{} }

// gad7ResultMessage assembles the GAD-7 result: heading, the 0–21 score +
// the severity wording (no diagnosis labels), and the footer of the short
// disclaimer plus the one-line attribution.
func gad7ResultMessage(c *screening.MoodContent, res *state.Gad7Result) string {
	r := c.Module.Gad7.Results
	return c.Module.Results.Heading + "\n\n" +
		renderContent(r.ScoreLine, map[string]string{"score": strconv.Itoa(res.Score)}) + "\n" +
		gad7BandLine(c, res.Severity) +
		"\n\n⚠️ " + c.Module.Meta.Disclaimer + "\n" + r.AttributionLine
}

// gad7Finalize completes the anxiety test: scores the answers, persists the
// Gad7Result and wipes the raw answers in the same Set, sends the result,
// and routes to the report transit (which renders the combined doctor
// report and lands home — GAD-7 is the end of the offer chain).
func gad7Finalize(c *screening.MoodContent, ctx Context, answers []int) Outcome {
	score := c.GAD7.Score(answers)
	res := state.Gad7Result{TakenAt: ctx.Now(), Score: score, Severity: c.GAD7.Band(score)}
	oc := scrText(gad7ResultMessage(c, &res))
	oc.RemoveKeyboard = true
	oc.NextState = state.StateMoodReport
	oc.Mutate = func(u *state.UserData) {
		r := res
		u.Gad7Result = &r
		u.Gad7 = nil
	}
	return oc
}

// MoodOfferGad7Phase owns StateMoodOfferGad7 — the one-button link shown
// after the PHQ-9 report: mood and anxiety often go together, so the bot
// offers the short GAD-7 (no clinical terms, consent already covers it).
// Declining lands home; an unfinished GAD-7 run is resumed, never wiped.
type MoodOfferGad7Phase struct {
	c *screening.MoodContent
}

func NewMoodOfferGad7Phase(c *screening.MoodContent) *MoodOfferGad7Phase {
	return &MoodOfferGad7Phase{c: c}
}

func (MoodOfferGad7Phase) State() state.StateKind { return state.StateMoodOfferGad7 }

func (p *MoodOfferGad7Phase) Setup(ctx Context) Outcome {
	if ctx.User.MoodResult == nil {
		// The offer only makes sense right after a PHQ-9 completion.
		return Outcome{NextState: testExitState}
	}
	o := p.c.Module.Gad7.Offer
	oc := scrText(o.Body)
	oc.Keyboard = [][]string{{o.StartButton}, {o.LaterButton}}
	return oc
}

func (p *MoodOfferGad7Phase) Collect(ctx Context, input string) Outcome {
	o := p.c.Module.Gad7.Offer
	switch in := normText(input); {
	case labelIs(in, o.StartButton):
		if ctx.User.Gad7 != nil {
			// A paused GAD-7 run exists — continue it instead of wiping.
			return Outcome{NextState: state.StateMoodGad7Question}
		}
		now := ctx.Now()
		return Outcome{
			NextState: state.StateMoodGad7Question,
			Mutate: func(u *state.UserData) {
				u.Gad7 = &state.MoodProgress{StartedAt: now}
			},
		}
	case labelIs(in, o.LaterButton):
		return Outcome{NextState: testExitState}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (MoodOfferGad7Phase) Remind(ctx Context) Outcome { return Outcome{} }
