package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodQuestionPhase owns StateMoodQuestion — the index-driven series of the
// nine PHQ-9 questions, one short message per question ("Question N of 9" +
// the official item text + the 4-option scale keyboard). The question index
// is derived from len(Mood.Answers) — no separate cursor to drift.
//
// Crisis protocol (deterministic, in code): an answer > 0 on the crisis
// item (question 9) transitions to StateMoodCrisis IMMEDIATELY after the
// answer — the crisis card is shown before the result, and the test is not
// blocked (the card carries a Continue button).
type MoodQuestionPhase struct {
	c *screening.MoodContent
}

func NewMoodQuestionPhase(c *screening.MoodContent) *MoodQuestionPhase {
	return &MoodQuestionPhase{c: c}
}

func (MoodQuestionPhase) State() state.StateKind { return state.StateMoodQuestion }

func (p *MoodQuestionPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Mood
	if s == nil {
		return Outcome{NextState: state.StateMoodConsent}
	}
	items := p.c.PHQ9.Items
	li := len(s.Answers)
	if li >= len(items) {
		// All questions answered (stale keyboard tap, or a resume that lost
		// its crisis ResumeState) — finalize without re-asking. The crisis
		// contacts still reach the user via the result when q9 > 0.
		return moodFinalize(p.c, ctx, s.Answers)
	}
	text := ""
	if li == 0 {
		// Block heading + the official instruction, shown once above q1.
		text = "📋 " + p.c.Module.Meta.Title + "\n" + p.c.PHQ9.Instruction + "\n\n"
	}
	text += moodProgressLine(p.c, li+1, len(items)) + "\n" + items[li].Text
	oc := scrText(text)
	oc.Keyboard = scaleKeyboard(p.c.PHQ9.Scale)
	return oc
}

func (p *MoodQuestionPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Mood
	if s == nil {
		return Outcome{NextState: state.StateMoodConsent}
	}
	items := p.c.PHQ9.Items
	li := len(s.Answers)
	if li >= len(items) {
		return Outcome{NextState: state.StateMoodQuestion} // re-run the Setup guards
	}
	score, ok := matchScale(p.c.PHQ9.Scale, normText(input))
	if !ok {
		return Outcome{ReplyKey: "scr.invalid_scale"}
	}

	appendAnswer := func(u *state.UserData) {
		if u.Mood == nil {
			return
		}
		mc := u.Mood.Clone()
		mc.Answers = append(mc.Answers, score)
		u.Mood = mc
	}

	if items[li].Crisis && score > 0 {
		// Crisis card right after this answer, not at the end.
		return Outcome{NextState: state.StateMoodCrisis, Mutate: appendAnswer}
	}
	if li+1 >= len(items) {
		// Last answer without a crisis signal — finalize with the final
		// list (the just-given answer is not in ctx.User yet).
		answers := append(append([]int(nil), s.Answers...), score)
		return moodFinalize(p.c, ctx, answers)
	}
	return Outcome{NextState: state.StateMoodQuestion, Mutate: appendAnswer}
}

func (MoodQuestionPhase) Remind(ctx Context) Outcome { return Outcome{} }

// MoodCrisisPhase owns StateMoodCrisis — the deterministic crisis card shown
// immediately after an answer > 0 on the crisis item: warm lead + the adult
// support contacts, plus one direct talk-to-someone-today line when the
// answer was 2–3. The test is not blocked: Continue proceeds (to the result,
// since the crisis item is the last question).
type MoodCrisisPhase struct {
	c *screening.MoodContent
}

func NewMoodCrisisPhase(c *screening.MoodContent) *MoodCrisisPhase {
	return &MoodCrisisPhase{c: c}
}

func (MoodCrisisPhase) State() state.StateKind { return state.StateMoodCrisis }

func (p *MoodCrisisPhase) Setup(ctx Context) Outcome {
	s := ctx.User.Mood
	if s == nil {
		return Outcome{NextState: state.StateMoodConsent}
	}
	cr := p.c.Module.Crisis
	answer := p.c.PHQ9.CrisisAnswer(s.Answers)
	if answer <= 0 {
		// Crisis state without a crisis answer — broken state, back to the
		// question series.
		return Outcome{NextState: state.StateMoodQuestion}
	}
	text := cr.Lead + "\n\n" + cr.Contacts
	if answer >= 2 {
		text += "\n\n" + cr.UrgentLine
	}
	oc := scrText(text)
	oc.Keyboard = [][]string{{cr.ContinueButton}}
	return oc
}

func (p *MoodCrisisPhase) Collect(ctx Context, input string) Outcome {
	s := ctx.User.Mood
	if s == nil {
		return Outcome{NextState: state.StateMoodConsent}
	}
	if !labelIs(normText(input), p.c.Module.Crisis.ContinueButton) {
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
	if len(s.Answers) >= len(p.c.PHQ9.Items) {
		return moodFinalize(p.c, ctx, s.Answers)
	}
	return Outcome{NextState: state.StateMoodQuestion}
}

func (MoodCrisisPhase) Remind(ctx Context) Outcome { return Outcome{} }
