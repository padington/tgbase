package journey

import (
	"strconv"
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// EatReportPhase owns StateEatReport — the transit phase after any
// eating-track completion: it sends the combined doctor report (everything
// completed so far, with dates) and lands home. A recorded FODMAP detour
// stays reachable there via the contextual «back to the diary» button.
//
// Unlike the mood module there is no offer chain here: every instrument of
// this track ends the same way — score, reading, report, landing.
type EatReportPhase struct {
	c *screening.EatingContent
}

func NewEatReportPhase(c *screening.EatingContent) *EatReportPhase {
	return &EatReportPhase{c: c}
}

func (EatReportPhase) State() state.StateKind { return state.StateEatReport }

func (p *EatReportPhase) Setup(ctx Context) Outcome {
	u := ctx.User
	if u.EdeqsResult == nil && u.BesResult == nil && u.NiasResult == nil {
		return Outcome{NextState: testExitState}
	}
	oc := scrText(p.c.Module.DoctorReport.LeadIn + "\n\n" + eatCombinedReport(p.c, u))
	oc.RemoveKeyboard = true
	oc.NextState = testExitState
	return oc
}

func (EatReportPhase) Collect(ctx Context, input string) Outcome {
	return Outcome{ReplyKey: "scr.invalid_button"}
}

func (EatReportPhase) Remind(ctx Context) Outcome { return Outcome{} }

// eatCombinedReport renders the shareable doctor summary from whatever
// results exist — EDE-QS (score + applied cutoff + verdict), BES (score +
// band), NIAS (three subscales + the Burton Murray reading) — each with its
// date. Rendered on the fly, never stored.
//
// The low-FODMAP line is appended automatically whenever the user has diary
// activity: a clinician reading restraint items must know that part of this
// person's dietary restriction is medically motivated, or the screen reads
// falsely positive.
func eatCombinedReport(c *screening.EatingContent, u state.UserData) string {
	dr := c.Module.DoctorReport
	lines := []string{dr.Heading}

	if r := u.EdeqsResult; r != nil {
		verdict := dr.VerdictNegative
		if r.Positive {
			verdict = dr.VerdictPositive
		}
		lines = append(lines, renderContent(dr.EdeqsLine, map[string]string{
			"date":    r.TakenAt.Format(reportDateLayout),
			"score":   strconv.Itoa(r.Score),
			"cutoff":  strconv.Itoa(r.Cutoff),
			"verdict": verdict,
		}))
	}
	if r := u.BesResult; r != nil {
		lines = append(lines, renderContent(dr.BesLine, map[string]string{
			"date":  r.TakenAt.Format(reportDateLayout),
			"score": strconv.Itoa(r.Score),
			"band":  besBandLine(c, r.Band),
		}))
	}
	if r := u.NiasResult; r != nil {
		lines = append(lines, renderContent(dr.NiasLine, map[string]string{
			"date":            r.TakenAt.Format(reportDateLayout),
			"picky":           strconv.Itoa(r.Picky),
			"appetite":        strconv.Itoa(r.Appetite),
			"fear":            strconv.Itoa(r.Fear),
			"picky_cutoff":    strconv.Itoa(r.PickyCutoff),
			"appetite_cutoff": strconv.Itoa(r.AppetiteCutoff),
			"fear_cutoff":     strconv.Itoa(r.FearCutoff),
		}))
		if key := niasContextKey(r, u.EdeqsResult); key != screening.NiasCtxNone {
			if line, ok := dr.NiasContexts[key]; ok {
				lines = append(lines, line)
			}
		}
	}
	if fodmapContextActive(u) {
		lines = append(lines, dr.FodmapContextLine)
	}

	lines = append(lines, "", dr.Footer)
	return strings.Join(lines, "\n")
}
