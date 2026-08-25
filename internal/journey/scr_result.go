package journey

import (
	"strconv"
	"strings"
	"time"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// reportDateLayout formats ScreeningResult.TakenAt for /report and the
// doctor report.
const reportDateLayout = "02.01.2006"

// buildScreeningResult converts the transient progress into the persisted
// result: scores per instrument, the thresholds that were applied (copied
// from the content so the report stays honest even if the content changes
// later), the verdict wording key, and the context facts. Never any raw
// per-question answers. childDomains is passed explicitly so the childhood
// "none" button can finalize with an empty list.
func buildScreeningResult(c *screening.Content, s *state.ScreeningProgress, childDomains []string, now time.Time) state.ScreeningResult {
	var asrsB []int
	if len(s.AsrsAnswers) > len(c.ASRS.PartA.Items) {
		asrsB = s.AsrsAnswers[len(c.ASRS.PartA.Items):]
	}
	aSig, aPos := c.ASRS.ScorePartA(s.AsrsAnswers)
	bSig := c.ASRS.ScorePartB(asrsB)
	wSum, wPos := c.WURS.ScoreWURS(s.WursAnswers)

	onsetChildhood := s.OnsetChild != nil && *s.OnsetChild
	onsetAge := 0
	if !onsetChildhood {
		onsetAge = s.OnsetAge
	}
	verdict, gapHint := screening.OverallVerdict(aPos, onsetChildhood, len(s.AdultDomains))

	return state.ScreeningResult{
		TakenAt:          now,
		AsrsASignificant: aSig,
		AsrsAThreshold:   c.ASRS.PartA.Scoring.PositiveScreenThreshold,
		AsrsAPositive:    aPos,
		AsrsBSignificant: bSig,
		WursScore:        wSum,
		WursCutoff:       c.WURS.PrimaryCutoff(),
		WursPositive:     wPos,
		OnsetChildhood:   onsetChildhood,
		OnsetAge:         onsetAge,
		AdultDomains:     append([]string(nil), s.AdultDomains...),
		ChildDomains:     append([]string(nil), childDomains...),
		Verdict:          verdict,
		GapHint:          gapHint,
	}
}

// domainTitles maps stored domain ids onto the life-phase titles from the
// content. Unknown ids render as-is so a content change never hides a fact.
func domainTitles(c *screening.Content, ids []string, childhood bool) []string {
	byID := make(map[string]screening.DomainItem, len(c.Module.Domains.Items))
	for _, item := range c.Module.Domains.Items {
		byID[item.ID] = item
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if item, ok := byID[id]; ok {
			if childhood {
				out = append(out, item.Childhood.Title)
			} else {
				out = append(out, item.Adult.Title)
			}
			continue
		}
		out = append(out, id)
	}
	return out
}

// onsetFact renders the criterion-B fact line (no scores — facts only).
func onsetFact(c *screening.Content, res *state.ScreeningResult) string {
	if res.OnsetChildhood {
		return c.Module.CriterionB.OnsetFactChildhood
	}
	return renderContent(c.Module.CriterionB.OnsetFactLater, map[string]string{
		"age": strconv.Itoa(res.OnsetAge),
	})
}

// resultMessage assembles the first of the three final messages: heading,
// one lean block per instrument (score + verdict, each with its own
// threshold), the context facts (no scores), the overall wording, and a
// footer of the short disclaimer plus one compact attribution line. There
// is deliberately no combined score anywhere.
func resultMessage(c *screening.Content, res *state.ScreeningResult) string {
	r := c.Module.Results
	var b strings.Builder

	b.WriteString(r.Heading)

	// ASRS part A — its own screen threshold.
	a := r.Instruments.AsrsA
	aLine := a.NegativeLine
	if res.AsrsAPositive {
		aLine = a.PositiveLine
	}
	b.WriteString("\n\n" + a.Title + "\n" +
		renderContent(a.ScoreLine, map[string]string{"score": strconv.Itoa(res.AsrsASignificant)}) + "\n" +
		aLine)

	// ASRS part B — count only, no threshold by design.
	pb := r.Instruments.AsrsB
	b.WriteString("\n\n" + pb.Title + "\n" +
		renderContent(pb.ScoreLine, map[string]string{"score": strconv.Itoa(res.AsrsBSignificant)}) + "\n" +
		pb.Note)

	// WURS-25 — its own cutoff.
	w := r.Instruments.Wurs
	wLine := w.NegativeLine
	if res.WursPositive {
		wLine = w.PositiveLine
	}
	b.WriteString("\n\n" + w.Title + "\n" +
		renderContent(w.ScoreLine, map[string]string{"score": strconv.Itoa(res.WursScore)}) + "\n" +
		wLine)

	// DSM-context facts — booleans and lists, never scores.
	cf := r.ContextFacts
	b.WriteString("\n\n" + cf.Heading + "\n" +
		renderContent(cf.OnsetLine, map[string]string{"onset_fact": onsetFact(c, res)}))
	if len(res.AdultDomains) > 0 {
		b.WriteString("\n" + renderContent(cf.AdultDomainsLine, map[string]string{
			"adult_domains": strings.Join(domainTitles(c, res.AdultDomains, false), ", "),
		}))
	} else {
		b.WriteString("\n" + cf.AdultDomainsEmpty)
	}
	if len(res.ChildDomains) > 0 {
		b.WriteString("\n" + renderContent(cf.ChildDomainsLine, map[string]string{
			"child_domains": strings.Join(domainTitles(c, res.ChildDomains, true), ", "),
		}))
	} else {
		b.WriteString("\n" + cf.ChildDomainsEmpty)
	}

	// Overall wording — a template key, never a number.
	switch res.Verdict {
	case screening.VerdictConsistent:
		b.WriteString("\n\n" + r.Overall.Consistent)
	case screening.VerdictNotConsistent:
		b.WriteString("\n\n" + r.Overall.NotConsistent)
	default:
		b.WriteString("\n\n" + renderContent(r.Overall.Partial, map[string]string{
			"gap_hint": r.Overall.GapHints[res.GapHint],
		}))
	}

	b.WriteString("\n\n⚠️ " + c.Module.Meta.Disclaimer + "\n" + r.AttributionLine)
	return b.String()
}

// doctorReport renders the shareable summary from the persisted result on
// the fly — the rendered text is never stored.
func doctorReport(c *screening.Content, trans i18n.Translator, locale i18n.Locale, res *state.ScreeningResult) string {
	verdictWord := func(positive bool) string {
		if positive {
			return trans.T("scr.report.positive", locale, nil)
		}
		return trans.T("scr.report.negative", locale, nil)
	}
	domainsOr := func(titles []string) string {
		if len(titles) == 0 {
			return trans.T("scr.report.domains_empty", locale, nil)
		}
		return strings.Join(titles, ", ")
	}
	return renderContent(c.Module.Results.DoctorReport.Template, map[string]string{
		"date":           res.TakenAt.Format(reportDateLayout),
		"asrs_a_score":   strconv.Itoa(res.AsrsASignificant),
		"asrs_a_verdict": verdictWord(res.AsrsAPositive),
		"asrs_b_score":   strconv.Itoa(res.AsrsBSignificant),
		"wurs_score":     strconv.Itoa(res.WursScore),
		"wurs_verdict":   verdictWord(res.WursPositive),
		"onset_fact":     onsetFact(c, res),
		"adult_domains":  domainsOr(domainTitles(c, res.AdultDomains, false)),
		"child_domains":  domainsOr(domainTitles(c, res.ChildDomains, true)),
	})
}

// ScrReferralPhase owns StateScrReferral — a transit Setup-only phase that
// sends message two of the final chain: the short route to a specialist.
type ScrReferralPhase struct {
	c *screening.Content
}

func NewScrReferralPhase(c *screening.Content) *ScrReferralPhase {
	return &ScrReferralPhase{c: c}
}

func (ScrReferralPhase) State() state.StateKind { return state.StateScrReferral }

func (p *ScrReferralPhase) Setup(ctx Context) Outcome {
	if ctx.User.ScreeningResult == nil {
		return Outcome{NextState: state.StateIdle}
	}
	r := p.c.Module.Results
	oc := scrText(r.Referral.Heading + "\n" + r.Referral.Body)
	oc.NextState = state.StateScrReport
	return oc
}

func (ScrReferralPhase) Collect(ctx Context, input string) Outcome {
	return Outcome{ReplyKey: "scr.invalid_button"}
}

func (ScrReferralPhase) Remind(ctx Context) Outcome { return Outcome{} }

// ScrReportPhase owns StateScrReport — the final transit phase: sends the
// doctor report and returns the user to the recorded FODMAP state (re-Setup)
// or idle.
type ScrReportPhase struct {
	c *screening.Content
}

func NewScrReportPhase(c *screening.Content) *ScrReportPhase { return &ScrReportPhase{c: c} }

func (ScrReportPhase) State() state.StateKind { return state.StateScrReport }

func (p *ScrReportPhase) Setup(ctx Context) Outcome {
	res := ctx.User.ScreeningResult
	if res == nil {
		return Outcome{NextState: state.StateIdle}
	}
	text := p.c.Module.Results.DoctorReport.LeadIn + "\n\n" +
		doctorReport(p.c, ctx.Trans, ctx.Locale, res)
	oc := scrText(text)
	oc.RemoveKeyboard = true
	oc.NextState = exitState(ctx.User)
	oc.Mutate = clearReturnState
	return oc
}

func (ScrReportPhase) Collect(ctx Context, input string) Outcome {
	return Outcome{ReplyKey: "scr.invalid_button"}
}

func (ScrReportPhase) Remind(ctx Context) Outcome { return Outcome{} }
