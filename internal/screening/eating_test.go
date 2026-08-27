package screening

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- synthetic eating content ----------------------------------------------
// LoadEating demands the full canonical shape (pinned scales, statement
// weights, cutoffs), so the generators below produce a complete, valid
// bundle with short texts; each takes a mutate hook so a test can break
// exactly one thing and assert that loading refuses it.

func syntheticEDEQS(mutate func(lines []string) []string) string {
	lines := []string{
		`instruction_ru: "Answer about the past 7 days."`,
		`days_header_ru: "On how many of the past 7 days…"`,
		`severity_header_ru: "Over the past 7 days…"`,
		`scale_days:`,
		`  - {score: 0, label: "0 days"}`,
		`  - {score: 1, label: "1-2 days"}`,
		`  - {score: 2, label: "3-5 days"}`,
		`  - {score: 3, label: "6-7 days"}`,
		`scale_severity:`,
		`  - {score: 0, label: "Not at all"}`,
		`  - {score: 1, label: "Slightly"}`,
		`  - {score: 2, label: "Moderately"}`,
		`  - {score: 3, label: "Markedly"}`,
		`items:`,
	}
	for id := 1; id <= 12; id++ {
		scale := EdeqsScaleDays
		if id >= 11 {
			scale = EdeqsScaleSeverity
		}
		lines = append(lines, fmt.Sprintf(`  - {id: %d, scale: %s, text: "E item %d"}`, id, scale, id))
	}
	lines = append(lines,
		`scoring:`,
		`  cutoff: 15`,
		`  bands:`,
		`    - {min: 0, max: 14, id: below}`,
		`    - {min: 15, max: 36, id: at_or_above}`,
		`attribution_text_original: "EDE-QS attribution."`,
	)
	if mutate != nil {
		lines = mutate(lines)
	}
	return strings.Join(lines, "\n") + "\n"
}

func syntheticBES(mutate func(lines []string) []string) string {
	lines := []string{
		`instruction_ru: "Pick the statement that fits best."`,
		`items:`,
	}
	for i, weights := range canonicalBesWeights {
		lines = append(lines, fmt.Sprintf(`  - id: %d`, i+1), `    statements:`)
		for j, w := range weights {
			lines = append(lines, fmt.Sprintf(`      - {score: %d, text: "B item %d option %d"}`, w, i+1, j+1))
		}
	}
	lines = append(lines,
		`scoring:`,
		`  bands:`,
		`    - {min: 0, max: 17, id: low}`,
		`    - {min: 18, max: 26, id: moderate}`,
		`    - {min: 27, max: 46, id: severe}`,
		`attribution_text_ru: "BES attribution."`,
	)
	if mutate != nil {
		lines = mutate(lines)
	}
	return strings.Join(lines, "\n") + "\n"
}

func syntheticNIAS(mutate func(lines []string) []string) string {
	lines := []string{
		`instruction_ru: "How much is this like you?"`,
		`scale:`,
	}
	for score, label := range []string{"0", "1", "2", "3", "4", "5"} {
		lines = append(lines, fmt.Sprintf(`  - {score: %d, label: "Level %s"}`, score, label))
	}
	lines = append(lines, `items:`)
	for id := 1; id <= 9; id++ {
		lines = append(lines, fmt.Sprintf(`  - {id: %d, subscale: %s, text: "N item %d"}`,
			id, niasSubscaleOrder[(id-1)/3], id))
	}
	lines = append(lines,
		`scoring:`,
		`  subscales:`,
		`    - {id: picky, cutoff: 10}`,
		`    - {id: appetite, cutoff: 9}`,
		`    - {id: fear, cutoff: 10}`,
		`attribution_text_ru: "NIAS attribution."`,
	)
	if mutate != nil {
		lines = mutate(lines)
	}
	return strings.Join(lines, "\n") + "\n"
}

const syntheticEatingModule = `meta:
  title: "Food track"
  disclaimer: "screening, not a diagnosis"
menu:
  prompt: "menu prompt"
  edeqs_button: "Core test"
  bes_button: "Overeating"
  nias_button: "Picky eating"
  resume_edeqs_button: "Resume core"
  resume_bes_button: "Resume overeating"
  resume_nias_button: "Resume picky"
consent:
  title: "consent title"
  body: "consent body"
  agree_button: "Begin"
  later_button: "Not now"
  declined: "declined text"
edeqs:
  title: "Core test"
  results:
    score_line: "EDE-QS: {score} of 36 (cutoff {cutoff})"
    bands:
      below: "below the cutoff"
      at_or_above: "at or above the cutoff"
    attribution_line: "EDEQS-ATTR"
bes:
  title: "Overeating"
  progress: "Group {current} of {total}"
  pick_hint: "Send the option number."
  results:
    score_line: "BES: {score} of 46"
    bands:
      low: "band low"
      moderate: "band moderate"
      severe: "band severe"
    attribution_line: "BES-ATTR"
nias:
  title: "Picky eating"
  progress: "Statement {current} of {total}"
  subscales:
    picky: "Picky"
    appetite: "Appetite"
    fear: "Fear"
  results:
    subscale_line: "{name}: {score} of 15 (cutoff {cutoff}) - {verdict}"
    verdict_above: "above"
    verdict_below: "within"
    contexts:
      restrictive_no_body_image: "restrictive without body-image concern"
      restrictive_body_image: "restriction tied to body image"
      restrictive_unknown: "take the core test for the full picture"
    attribution_line: "NIAS-ATTR"
doctor_report:
  lead_in: "report lead-in"
  heading: "FOOD SUMMARY REPORT"
  edeqs_line: "EDE-QS ({date}): {score}/36, cutoff {cutoff} - screen {verdict}"
  verdict_positive: "positive"
  verdict_negative: "negative"
  bes_line: "BES ({date}): {score}/46 - {band}"
  nias_line: "NIAS ({date}): picky {picky}/15 (>={picky_cutoff}), appetite {appetite}/15 (>={appetite_cutoff}), fear {fear}/15 (>={fear_cutoff})"
  nias_contexts:
    restrictive_no_body_image: "subscale above cutoff with EDE-QS below - restrictive pattern without body-image concern"
    restrictive_body_image: "subscale above cutoff with EDE-QS at or above - restriction likely tied to body image"
    restrictive_unknown: "subscale above cutoff, EDE-QS not taken"
  fodmap_context_line: "FODMAP-CONTEXT: low-FODMAP elimination diet for IBS"
  footer: "REPORT-FOOTER"
ui:
  mode_button: "Food track"
  results_heading: "Your result"
  progress: "Question {current} of {total}"
  abandon_confirmed: "eating test abandoned"
  delete_confirm_prompt: "delete eating data?"
  delete_confirm_button: "Yes, delete"
  delete_cancel_button: "Keep"
  delete_done: "eating data deleted"
  delete_nothing: "nothing eating stored"
`

// writeEatingContent writes a full synthetic bundle and returns its dir.
func writeEatingContent(t *testing.T, edeqs, bes, nias, module string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		edeqsFile:        edeqs,
		besFile:          bes,
		niasFile:         nias,
		eatingModuleFile: module,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func loadSyntheticEating(t *testing.T) *EatingContent {
	t.Helper()
	dir := writeEatingContent(t, syntheticEDEQS(nil), syntheticBES(nil), syntheticNIAS(nil), syntheticEatingModule)
	c, err := LoadEating(dir)
	if err != nil {
		t.Fatalf("LoadEating: %v", err)
	}
	return c
}

// --- EDE-QS scoring ---------------------------------------------------------

func TestEdeqs_ScoreAndCutoffBoundary(t *testing.T) {
	c := loadSyntheticEating(t)
	e := &c.EDEQS

	if got := e.Cutoff(); got != 15 {
		t.Fatalf("cutoff = %d, want 15", got)
	}

	// 14 → below, 15 → at or above: the only boundary the instrument has.
	below := []int{3, 3, 3, 3, 2, 0, 0, 0, 0, 0, 0, 0} // 14
	at := []int{3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0}    // 15
	if got := e.Score(below); got != 14 {
		t.Fatalf("score(below) = %d, want 14", got)
	}
	if got := e.Score(at); got != 15 {
		t.Fatalf("score(at) = %d, want 15", got)
	}
	if e.Positive(14) {
		t.Error("14 must not be a positive screen")
	}
	if !e.Positive(15) {
		t.Error("15 must be a positive screen")
	}
	if got := e.Band(14); got != EdeqsBandBelow {
		t.Errorf("band(14) = %q, want %q", got, EdeqsBandBelow)
	}
	if got := e.Band(15); got != EdeqsBandAtOrAbove {
		t.Errorf("band(15) = %q, want %q", got, EdeqsBandAtOrAbove)
	}

	// Maximum and panic-free short input.
	all3 := make([]int, 12)
	for i := range all3 {
		all3[i] = 3
	}
	if got := e.Score(all3); got != 36 {
		t.Errorf("max score = %d, want 36", got)
	}
	if got := e.Score(nil); got != 0 {
		t.Errorf("score(nil) = %d, want 0", got)
	}
	if got := e.Score([]int{3, 3}); got != 6 {
		t.Errorf("score(short) = %d, want 6", got)
	}
}

func TestEdeqs_ScaleForSwitchesAfterItem10(t *testing.T) {
	c := loadSyntheticEating(t)
	e := &c.EDEQS
	for i := 0; i < 10; i++ {
		if got := e.ScaleFor(i)[0].Label; got != e.ScaleDays[0].Label {
			t.Errorf("item %d must use the days scale, got %q", i+1, got)
		}
	}
	for i := 10; i < 12; i++ {
		if got := e.ScaleFor(i)[0].Label; got != e.ScaleSeverity[0].Label {
			t.Errorf("item %d must use the severity scale, got %q", i+1, got)
		}
	}
	// Out of range never panics.
	if got := e.ScaleFor(99)[0].Label; got != e.ScaleDays[0].Label {
		t.Errorf("out-of-range index must fall back to the days scale, got %q", got)
	}
}

// --- BES scoring ------------------------------------------------------------

func TestBes_BandBoundaries(t *testing.T) {
	c := loadSyntheticEating(t)
	b := &c.BES

	for _, tc := range []struct {
		score int
		want  string
	}{
		{0, BesBandLow}, {17, BesBandLow},
		{18, BesBandModerate}, {26, BesBandModerate},
		{27, BesBandSevere}, {46, BesBandSevere},
	} {
		if got := b.Band(tc.score); got != tc.want {
			t.Errorf("band(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
	if got := b.MaxScore(); got != 46 {
		t.Errorf("MaxScore = %d, want 46", got)
	}
	// The sum is over the picked statements' weights.
	answers := []int{3, 3, 3, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2}
	if got := b.Score(answers); got != 46 {
		t.Errorf("score(all max) = %d, want 46", got)
	}
	if got := b.Score(nil); got != 0 {
		t.Errorf("score(nil) = %d, want 0", got)
	}
}

// --- NIAS scoring and the Burton Murray rule --------------------------------

func TestNias_SubscaleScoresAndCutoffs(t *testing.T) {
	c := loadSyntheticEating(t)
	n := &c.NIAS

	// picky 9 (below 10), appetite 9 (at 9), fear 10 (at 10).
	got := n.Score([]int{3, 3, 3, 3, 3, 3, 4, 3, 3})
	if got != (NiasScores{Picky: 9, Appetite: 9, Fear: 10}) {
		t.Fatalf("subscale scores = %+v", got)
	}
	if n.Cutoff(NiasPicky) != 10 || n.Cutoff(NiasAppetite) != 9 || n.Cutoff(NiasFear) != 10 {
		t.Fatalf("cutoffs = %d/%d/%d, want 10/9/10",
			n.Cutoff(NiasPicky), n.Cutoff(NiasAppetite), n.Cutoff(NiasFear))
	}
	if got.Picky >= n.Cutoff(NiasPicky) {
		t.Error("picky 9 must stay below its cutoff of 10")
	}
	if got.Appetite < n.Cutoff(NiasAppetite) {
		t.Error("appetite 9 must reach its cutoff of 9")
	}
	if got.Fear < n.Cutoff(NiasFear) {
		t.Error("fear 10 must reach its cutoff of 10")
	}

	// Maximum per subscale is 15; short input is zero-filled.
	max := n.Score([]int{5, 5, 5, 5, 5, 5, 5, 5, 5})
	if max != (NiasScores{Picky: 15, Appetite: 15, Fear: 15}) {
		t.Errorf("max subscale scores = %+v, want 15/15/15", max)
	}
	if n.Score(nil) != (NiasScores{}) {
		t.Error("score(nil) must be all zeros")
	}
	if got := n.Cutoff("nope"); got != 0 {
		t.Errorf("unknown subscale cutoff = %d, want 0", got)
	}
}

func TestNiasContext_BurtonMurrayRule(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		positive, edeqsTaken, edeqsPositive bool
		want                                string
	}{
		{"no subscale above cutoff", false, true, false, NiasCtxNone},
		{"no subscale above cutoff, no edeqs", false, false, false, NiasCtxNone},
		{"positive + edeqs below cutoff", true, true, false, NiasCtxRestrictiveNoBodyImage},
		{"positive + edeqs at or above cutoff", true, true, true, NiasCtxRestrictiveBodyImage},
		{"positive, edeqs not taken", true, false, false, NiasCtxRestrictiveUnknown},
	} {
		if got := NiasContext(tc.positive, tc.edeqsTaken, tc.edeqsPositive); got != tc.want {
			t.Errorf("%s: NiasContext = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// --- Validate: foreign content is refused -----------------------------------

func TestLoadEating_RefusesForeignContent(t *testing.T) {
	replace := func(old, new string) func([]string) []string {
		return func(lines []string) []string {
			for i, l := range lines {
				if strings.Contains(l, old) {
					lines[i] = strings.Replace(l, old, new, 1)
					return lines
				}
			}
			return lines
		}
	}

	for _, tc := range []struct {
		name   string
		edeqs  string
		bes    string
		nias   string
		module string
		want   string
	}{
		{
			name:  "edeqs cutoff moved",
			edeqs: syntheticEDEQS(replace("cutoff: 15", "cutoff: 12")),
			want:  "cutoff is 12",
		},
		{
			name:  "edeqs band boundary moved",
			edeqs: syntheticEDEQS(replace("max: 14, id: below", "max: 13, id: below")),
			want:  "band 0",
		},
		{
			name:  "edeqs item moved to the wrong scale",
			edeqs: syntheticEDEQS(replace("id: 11, scale: severity", "id: 11, scale: days")),
			want:  "uses scale",
		},
		{
			name: "bes statement weight changed",
			bes:  syntheticBES(replace(`{score: 0, text: "B item 1 option 2"}`, `{score: 1, text: "B item 1 option 2"}`)),
			want: "weight 1, want 0",
		},
		{
			name: "bes band boundary moved",
			bes:  syntheticBES(replace("max: 17, id: low", "max: 16, id: low")),
			want: "band 0",
		},
		{
			name: "nias cutoff moved",
			nias: syntheticNIAS(replace("{id: appetite, cutoff: 9}", "{id: appetite, cutoff: 8}")),
			want: "subscale 1",
		},
		{
			name: "nias item in the wrong subscale",
			nias: syntheticNIAS(replace("id: 4, subscale: appetite", "id: 4, subscale: picky")),
			want: "is in subscale",
		},
		{
			name:   "module drops a Burton Murray branch",
			module: strings.Replace(syntheticEatingModule, `      restrictive_no_body_image: "restrictive without body-image concern"`, "", 1),
			want:   "nias.results.contexts.restrictive_no_body_image",
		},
		{
			name:   "module drops the fodmap context line",
			module: strings.Replace(syntheticEatingModule, `  fodmap_context_line: "FODMAP-CONTEXT: low-FODMAP elimination diet for IBS"`, "", 1),
			want:   "doctor_report.fodmap_context_line",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edeqs, bes, nias, module := tc.edeqs, tc.bes, tc.nias, tc.module
			if edeqs == "" {
				edeqs = syntheticEDEQS(nil)
			}
			if bes == "" {
				bes = syntheticBES(nil)
			}
			if nias == "" {
				nias = syntheticNIAS(nil)
			}
			if module == "" {
				module = syntheticEatingModule
			}
			_, err := LoadEating(writeEatingContent(t, edeqs, bes, nias, module))
			if err == nil {
				t.Fatal("expected LoadEating to refuse the content")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestLoadEating_RefusesForbiddenBranding(t *testing.T) {
	needle := strings.ToUpper(forbiddenBranding)
	edeqs := syntheticEDEQS(func(lines []string) []string {
		for i, l := range lines {
			if strings.Contains(l, `text: "E item 1"`) {
				lines[i] = strings.Replace(l, `"E item 1"`, `"E item 1 (`+needle+`)"`, 1)
			}
		}
		return lines
	})
	_, err := LoadEating(writeEatingContent(t, edeqs, syntheticBES(nil), syntheticNIAS(nil), syntheticEatingModule))
	if err == nil || !strings.Contains(err.Error(), "forbidden instrument branding") {
		t.Fatalf("expected the branding guard to fire, got %v", err)
	}
}

func TestLoadEating_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, edeqsFile), []byte(syntheticEDEQS(nil)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEating(dir); err == nil {
		t.Fatal("expected LoadEating to fail on a missing file")
	}
}
