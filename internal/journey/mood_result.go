package journey

import (
	"strconv"
	"strings"
	"time"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// buildMoodResult converts the raw answers into the persisted result: the
// total score, the severity-band id that was applied (copied from the
// content so the report stays honest even if the content changes later),
// and the single allowed per-question fact — the item-9 flag. Never any raw
// per-question answers.
func buildMoodResult(c *screening.MoodContent, answers []int, now time.Time) state.MoodResult {
	score := c.PHQ9.Score(answers)
	return state.MoodResult{
		TakenAt:    now,
		Score:      score,
		Severity:   c.PHQ9.Band(score),
		Q9Positive: c.PHQ9.CrisisAnswer(answers) > 0,
	}
}

// moodBandLine maps the stored band id onto the content wording. Unknown ids
// render as-is so a content change never hides the fact.
func moodBandLine(c *screening.MoodContent, band string) string {
	if line, ok := c.Module.Results.Bands[band]; ok {
		return line
	}
	return band
}

// moodResultMessage assembles the result message: heading, score + severity
// wording (no diagnosis labels), the retest block (with the delta against
// the previous result when one exists), the support contacts whenever the
// crisis item was answered > 0 — regardless of the total score — and a
// footer of the short disclaimer plus one compact attribution line.
func moodResultMessage(c *screening.MoodContent, res, prev *state.MoodResult) string {
	r := c.Module.Results
	var b strings.Builder

	b.WriteString(r.Heading)
	b.WriteString("\n\n" +
		renderContent(r.ScoreLine, map[string]string{"score": strconv.Itoa(res.Score)}) + "\n" +
		moodBandLine(c, res.Severity))

	b.WriteString("\n\n")
	if prev != nil {
		b.WriteString(renderContent(r.DeltaLine, map[string]string{
			"ago":  moodAgo(c, prev.TakenAt, res.TakenAt),
			"prev": strconv.Itoa(prev.Score),
			"cur":  strconv.Itoa(res.Score),
		}) + "\n")
	}
	b.WriteString(r.RetestLine)

	if res.Q9Positive {
		b.WriteString("\n\n" + r.CrisisHeading + "\n" + c.Module.Crisis.Contacts)
	}

	b.WriteString("\n\n⚠️ " + c.Module.Meta.Disclaimer + "\n" + r.AttributionLine)
	return b.String()
}

// moodDoctorReport renders the shareable summary from the persisted result
// on the fly — the rendered text is never stored.
func moodDoctorReport(c *screening.MoodContent, res *state.MoodResult) string {
	dr := c.Module.DoctorReport
	q9 := dr.Q9NotMarked
	if res.Q9Positive {
		q9 = dr.Q9Marked
	}
	return renderContent(dr.Template, map[string]string{
		"date":    res.TakenAt.Format(reportDateLayout),
		"score":   strconv.Itoa(res.Score),
		"band":    moodBandLine(c, res.Severity),
		"q9_fact": q9,
	})
}

// moodFinalize completes the mood screening: scores the answers, persists
// the MoodResult and wipes the raw answers in the same Set (no window where
// both are on disk), and sends the result message — rendering the delta
// against the previous result before it is overwritten. The doctor report
// follows via the Setup-only report phase.
func moodFinalize(c *screening.MoodContent, ctx Context, answers []int) Outcome {
	res := buildMoodResult(c, answers, ctx.Now())
	oc := scrText(moodResultMessage(c, &res, ctx.User.MoodResult))
	oc.RemoveKeyboard = true
	oc.NextState = state.StateMoodReport
	oc.Mutate = func(u *state.UserData) {
		r := res
		u.MoodResult = &r
		u.Mood = nil
	}
	return oc
}

// MoodReportPhase owns StateMoodReport — the final transit phase: sends the
// doctor report and lands the user on the home landing. A recorded FODMAP
// detour stays reachable there via the contextual «back to the diary»
// button — the test finish never drops the user into a mid-diary question.
type MoodReportPhase struct {
	c *screening.MoodContent
}

func NewMoodReportPhase(c *screening.MoodContent) *MoodReportPhase {
	return &MoodReportPhase{c: c}
}

func (MoodReportPhase) State() state.StateKind { return state.StateMoodReport }

func (p *MoodReportPhase) Setup(ctx Context) Outcome {
	res := ctx.User.MoodResult
	if res == nil {
		return Outcome{NextState: state.StateIdle}
	}
	text := p.c.Module.DoctorReport.LeadIn + "\n\n" + moodDoctorReport(p.c, res)
	oc := scrText(text)
	oc.RemoveKeyboard = true
	oc.NextState = testExitState
	return oc
}

func (MoodReportPhase) Collect(ctx Context, input string) Outcome {
	return Outcome{ReplyKey: "scr.invalid_button"}
}

func (MoodReportPhase) Remind(ctx Context) Outcome { return Outcome{} }
