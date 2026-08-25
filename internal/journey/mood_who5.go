package journey

import (
	"strconv"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodWho5Phase owns StateMoodWho5Question — the index-driven series of the
// five WHO-5 statements, one short message per statement ("Statement N of 5"
// + the official text + the 6-option scale keyboard in the form's top-down
// order). The index is derived from len(Who5.Answers). WHO-5 has no crisis
// item — no crisis logic here by design.
type MoodWho5Phase struct {
	c *screening.MoodContent
}

func NewMoodWho5Phase(c *screening.MoodContent) *MoodWho5Phase {
	return &MoodWho5Phase{c: c}
}

func (MoodWho5Phase) State() state.StateKind { return state.StateMoodWho5Question }

func (p *MoodWho5Phase) Setup(ctx Context) Outcome {
	s := ctx.User.Who5
	if s == nil {
		return Outcome{NextState: state.StateMoodMenu}
	}
	items := p.c.WHO5.Items
	li := len(s.Answers)
	if li >= len(items) {
		// All statements answered (stale keyboard tap) — finalize.
		return who5Finalize(p.c, ctx, s.Answers)
	}
	text := ""
	if li == 0 {
		// Block heading + the verbatim recall row header, shown once above
		// statement 1 (the paper-bound instruction sentences are omitted —
		// see who5_ru.yaml).
		text = "⚡ " + p.c.Module.Who5.Title + "\n" + p.c.WHO5.RecallHeader + "\n\n"
	}
	text += who5ProgressLine(p.c, li+1, len(items)) + "\n" + items[li].Text
	oc := scrText(text)
	oc.Keyboard = scaleKeyboard(p.c.WHO5.Scale)
	return oc
}

func (p *MoodWho5Phase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Who5
	if s == nil {
		return Outcome{NextState: state.StateMoodMenu}
	}
	items := p.c.WHO5.Items
	li := len(s.Answers)
	if li >= len(items) {
		return Outcome{NextState: state.StateMoodWho5Question} // re-run the Setup guards
	}
	score, ok := matchScale(p.c.WHO5.Scale, normText(input))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}

	if li+1 >= len(items) {
		// Last statement — finalize with the final list (the just-given
		// answer is not in ctx.User yet).
		answers := append(append([]int(nil), s.Answers...), score)
		return who5Finalize(p.c, ctx, answers)
	}
	return Outcome{
		NextState: state.StateMoodWho5Question,
		Mutate: func(u *state.UserData) {
			if u.Who5 == nil {
				return
			}
			mc := u.Who5.Clone()
			mc.Answers = append(mc.Answers, score)
			u.Who5 = mc
		},
	}
}

func (MoodWho5Phase) Remind(ctx Context) Outcome { return Outcome{} }

// who5ResultMessage assembles the WHO-5 result: heading, the 0–100 score +
// the interpretation wording, and the footer of the short disclaimer plus
// the one-line WHO attribution.
func who5ResultMessage(c *screening.MoodContent, res *state.Who5Result) string {
	r := c.Module.Who5.Results
	return c.Module.Results.Heading + "\n\n" +
		renderContent(r.ScoreLine, map[string]string{"score": strconv.Itoa(res.Score)}) + "\n" +
		who5BandLine(c, res.Band) +
		"\n\n⚠️ " + c.Module.Meta.Disclaimer + "\n" + r.AttributionLine
}

// who5Finalize completes the quick check: scores the answers (raw sum × 4 →
// 0–100), persists the Who5Result and wipes the raw answers in the same
// Set, and sends the result. A reduced score (≤ 50) routes to the PHQ-9
// offer — more insistent below the ≤ 28 line; a normal score lands home.
func who5Finalize(c *screening.MoodContent, ctx Context, answers []int) Outcome {
	score := c.WHO5.Score(answers)
	res := state.Who5Result{TakenAt: ctx.Now(), Score: score, Band: c.WHO5.Band(score)}
	oc := scrText(who5ResultMessage(c, &res))
	oc.RemoveKeyboard = true
	oc.NextState = testExitState
	if res.Band != screening.Who5BandOK {
		oc.NextState = state.StateMoodOfferPhq9
	}
	oc.Mutate = func(u *state.UserData) {
		r := res
		u.Who5Result = &r
		u.Who5 = nil
	}
	return oc
}

// MoodOfferPhq9Phase owns StateMoodOfferPhq9 — the one-button link shown
// after a reduced WHO-5 result: a short offer to take the full PHQ-9 (the
// consent already covers it), worded more insistently below the ≤ 28 line.
// Declining lands home; an unfinished PHQ-9 run is resumed, never wiped.
type MoodOfferPhq9Phase struct {
	c *screening.MoodContent
}

func NewMoodOfferPhq9Phase(c *screening.MoodContent) *MoodOfferPhq9Phase {
	return &MoodOfferPhq9Phase{c: c}
}

func (MoodOfferPhq9Phase) State() state.StateKind { return state.StateMoodOfferPhq9 }

func (p *MoodOfferPhq9Phase) Setup(ctx Context) Outcome {
	res := ctx.User.Who5Result
	if res == nil {
		return Outcome{NextState: testExitState}
	}
	o := p.c.Module.Who5.Offer
	body := o.BodyLow
	if res.Band == screening.Who5BandVeryLow {
		body = o.BodyVeryLow
	}
	oc := scrText(body)
	oc.Keyboard = [][]string{{o.StartButton}, {o.LaterButton}}
	return oc
}

func (p *MoodOfferPhq9Phase) Collect(ctx Context, input string) Outcome {
	o := p.c.Module.Who5.Offer
	switch in := normText(input); {
	case labelIs(in, o.StartButton):
		if ctx.User.Mood != nil {
			// A paused PHQ-9 run exists — continue it instead of wiping.
			return Outcome{NextState: phq9ResumeTarget(ctx.User.Mood)}
		}
		now := ctx.Now()
		return Outcome{
			NextState: state.StateMoodQuestion,
			Mutate: func(u *state.UserData) {
				u.Mood = &state.MoodProgress{StartedAt: now}
			},
		}
	case labelIs(in, o.LaterButton):
		return Outcome{NextState: testExitState}
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (MoodOfferPhq9Phase) Remind(ctx Context) Outcome { return Outcome{} }
