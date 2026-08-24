package screening

// Verdict and gap-hint keys. These map 1:1 onto the results.overall.* /
// results.overall.gap_hints.* templates of the content module. There is
// deliberately NO function that returns a combined score or an "ADHD
// percentage" — the instruments do not work that way and the product
// forbids inventing one.
const (
	VerdictConsistent    = "consistent"
	VerdictPartial       = "partial"
	VerdictNotConsistent = "not_consistent"

	GapNoChildhoodOnset  = "no_childhood_onset"
	GapNoCurrentSymptoms = "no_current_symptoms"
	GapFewDomains        = "few_domains"
)

// answerAt returns answers[i], or 0 when i is out of range. Short slices are
// a programmer error upstream; zero-filling keeps scoring panic-free.
func answerAt(answers []int, i int) int {
	if i < 0 || i >= len(answers) {
		return 0
	}
	return answers[i]
}

// ScorePartA counts part-A answers that fall into the shaded ("significant")
// boxes and reports whether the screen is positive.
//
// answers holds the first 6 answers (scores 0..4, index = id-1). Answer i is
// significant when answers[i] >= PartA.Items[i].SignificantMinScore — the
// per-item thresholds come from the content (items 1–3: >= 2 "Иногда",
// items 4–6: >= 3 "Часто"). positive when significant >=
// PartA.Scoring.PositiveScreenThreshold (4). The 0–24 point sum is NOT
// computed anywhere — the official part-A screen does not use it.
func (a *ASRS) ScorePartA(answers []int) (significant int, positive bool) {
	for i, item := range a.PartA.Items {
		if answerAt(answers, i) >= item.SignificantMinScore {
			significant++
		}
	}
	return significant, significant >= a.PartA.Scoring.PositiveScreenThreshold
}

// ScorePartB counts part-B answers in the shaded boxes. answers holds the 12
// answers to questions 7–18 (index 0 = question 7). Per the official
// checklist instructions part B has NO threshold and NO verdict — only the
// count, as additional material for a clinician.
func (a *ASRS) ScorePartB(answers []int) (significant int) {
	for i, item := range a.PartB.Items {
		if answerAt(answers, i) >= item.SignificantMinScore {
			significant++
		}
	}
	return significant
}

// ScoreWURS sums the 25 answers (0..100); positive when the sum reaches the
// content's primary cutoff (46).
func (w *WURS) ScoreWURS(answers []int) (sum int, positive bool) {
	for i := range w.Items {
		sum += answerAt(answers, i)
	}
	return sum, sum >= w.PrimaryCutoff()
}

// OverallVerdict maps the three key DSM-5-shaped conditions onto a wording
// key (NOT a score):
//
//	A: asrsAPositive — current symptoms reach the part-A screen threshold;
//	B: onsetChildhood — difficulties noticeable before age 12;
//	D: adultDomainCount >= 2 — impairment in at least two current life domains.
//
// All three → VerdictConsistent; none → VerdictNotConsistent; otherwise
// VerdictPartial with a deterministic gapHint (priority !A → !B → !D).
//
// WURS deliberately does not gate the verdict — it is its own line with its
// own cutoff; childhood domains are a reported fact only.
func OverallVerdict(asrsAPositive, onsetChildhood bool, adultDomainCount int) (verdict, gapHint string) {
	domainsOK := adultDomainCount >= 2
	met := 0
	for _, cond := range []bool{asrsAPositive, onsetChildhood, domainsOK} {
		if cond {
			met++
		}
	}
	switch met {
	case 3:
		return VerdictConsistent, ""
	case 0:
		return VerdictNotConsistent, ""
	}
	switch {
	case !asrsAPositive:
		return VerdictPartial, GapNoCurrentSymptoms
	case !onsetChildhood:
		return VerdictPartial, GapNoChildhoodOnset
	default:
		return VerdictPartial, GapFewDomains
	}
}
