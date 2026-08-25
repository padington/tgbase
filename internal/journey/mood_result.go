package journey

import (
	"strconv"
	"strings"
	"time"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// buildMoodResult converts the raw answers into the persisted result: the
// total score over the nine items, the severity-band id that was applied
// (copied from the content so the report stays honest even if the content
// changes later), and the two allowed per-question facts — the item-9 flag
// and the functional (10th) answer when it was asked (answers[9]). Never
// any raw per-question answers 1..9.
func buildMoodResult(c *screening.MoodContent, answers []int, now time.Time) state.MoodResult {
	score := c.PHQ9.Score(answers)
	res := state.MoodResult{
		TakenAt:    now,
		Score:      score,
		Severity:   c.PHQ9.Band(score),
		Q9Positive: c.PHQ9.CrisisAnswer(answers) > 0,
	}
	if len(answers) > len(c.PHQ9.Items) {
		res.Q10Answered = true
		res.Q10Answer = answers[len(c.PHQ9.Items)]
	}
	return res
}

// moodBandLine maps the stored PHQ-9 band id onto the content wording.
// Unknown ids render as-is so a content change never hides the fact.
func moodBandLine(c *screening.MoodContent, band string) string {
	if line, ok := c.Module.Results.Bands[band]; ok {
		return line
	}
	return band
}

// who5BandLine / gad7BandLine — same contract for the other two instruments.
func who5BandLine(c *screening.MoodContent, band string) string {
	if line, ok := c.Module.Who5.Results.Bands[band]; ok {
		return line
	}
	return band
}

func gad7BandLine(c *screening.MoodContent, band string) string {
	if line, ok := c.Module.Gad7.Results.Bands[band]; ok {
		return line
	}
	return band
}

// funcAnswerLabel renders the stored functional-item answer as the official
// option label the user picked; the bare number is the fallback when the
// content no longer carries the score.
func funcAnswerLabel(c *screening.MoodContent, score int) string {
	for _, opt := range c.PHQ9.FuncItem.Scale {
		if opt.Score == score {
			return opt.Label
		}
	}
	return strconv.Itoa(score)
}

// moodResultMessage assembles the PHQ-9 result message: heading, score +
// severity wording (no diagnosis labels), the retest block (with the delta
// against the previous result when one exists), the support contacts
// whenever the crisis item was answered > 0 — regardless of the total score
// — and a footer of the short disclaimer plus one compact attribution line.
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

// moodCombinedReport renders the shareable doctor summary from whatever
// results exist — PHQ-9 (with the item-9 fact and the functional answer),
// GAD-7, WHO-5 — each with its date. Rendered on the fly, never stored.
func moodCombinedReport(c *screening.MoodContent, u state.UserData) string {
	dr := c.Module.DoctorReport
	lines := []string{dr.Heading}

	if r := u.MoodResult; r != nil {
		lines = append(lines, renderContent(dr.Phq9Line, map[string]string{
			"date":  r.TakenAt.Format(reportDateLayout),
			"score": strconv.Itoa(r.Score),
			"band":  moodBandLine(c, r.Severity),
		}))
		q9 := dr.Q9NotMarked
		if r.Q9Positive {
			q9 = dr.Q9Marked
		}
		lines = append(lines, renderContent(dr.Q9Line, map[string]string{"q9_fact": q9}))
		if r.Q10Answered {
			lines = append(lines, renderContent(dr.Q10Line, map[string]string{
				"answer": funcAnswerLabel(c, r.Q10Answer),
			}))
		}
	}
	if r := u.Gad7Result; r != nil {
		lines = append(lines, renderContent(dr.Gad7Line, map[string]string{
			"date":  r.TakenAt.Format(reportDateLayout),
			"score": strconv.Itoa(r.Score),
			"band":  gad7BandLine(c, r.Severity),
		}))
	}
	if r := u.Who5Result; r != nil {
		lines = append(lines, renderContent(dr.Who5Line, map[string]string{
			"date":  r.TakenAt.Format(reportDateLayout),
			"score": strconv.Itoa(r.Score),
			"band":  who5BandLine(c, r.Band),
		}))
	}

	lines = append(lines, "", dr.Footer)
	return strings.Join(lines, "\n")
}

// moodFinalize completes the PHQ-9 run: scores the answers, persists the
// MoodResult and wipes the raw answers in the same Set (no window where
// both are on disk), and sends the result message — rendering the delta
// against the previous result before it is overwritten. The combined doctor
// report follows via the report phase.
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

// MoodReportPhase owns StateMoodReport — the transit phase after a PHQ-9 or
// GAD-7 completion: sends the combined doctor report (everything completed
// so far, with dates), then routes onward — to the GAD-7 offer when the
// PHQ-9 was the just-completed instrument (the mood-and-anxiety link),
// otherwise to the home landing. A recorded FODMAP detour stays reachable
// there via the contextual «back to the diary» button.
type MoodReportPhase struct {
	c *screening.MoodContent
}

func NewMoodReportPhase(c *screening.MoodContent) *MoodReportPhase {
	return &MoodReportPhase{c: c}
}

func (MoodReportPhase) State() state.StateKind { return state.StateMoodReport }

// phq9JustCompleted reports whether the PHQ-9 result is the most recent
// completion — the deterministic trigger of the GAD-7 offer. The report
// phase is only ever entered right after a PHQ-9 or GAD-7 finalization, so
// comparing the two timestamps identifies the entry path without extra
// state.
func phq9JustCompleted(u state.UserData) bool {
	r := u.MoodResult
	if r == nil {
		return false
	}
	if g := u.Gad7Result; g != nil && g.TakenAt.After(r.TakenAt) {
		return false
	}
	return true
}

func (p *MoodReportPhase) Setup(ctx Context) Outcome {
	u := ctx.User
	if u.MoodResult == nil && u.Gad7Result == nil && u.Who5Result == nil {
		return Outcome{NextState: state.StateIdle}
	}
	text := p.c.Module.DoctorReport.LeadIn + "\n\n" + moodCombinedReport(p.c, u)
	oc := scrText(text)
	oc.RemoveKeyboard = true
	oc.NextState = testExitState
	if phq9JustCompleted(u) {
		oc.NextState = state.StateMoodOfferGad7
	}
	return oc
}

func (MoodReportPhase) Collect(ctx Context, input string) Outcome {
	return Outcome{ReplyKey: "scr.invalid_button"}
}

func (MoodReportPhase) Remind(ctx Context) Outcome { return Outcome{} }
