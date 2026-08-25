package screening

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- synthetic mood content -------------------------------------------------

func syntheticPHQ9(mutate func(lines []string) []string) string {
	lines := []string{
		`instruction_official_ru: "How often over the last 2 weeks?"`,
		`scale:`,
		`  - {score: 0, label: "Not at all"}`,
		`  - {score: 1, label: "Several days"}`,
		`  - {score: 2, label: "More than half"}`,
		`  - {score: 3, label: "Nearly every day"}`,
		`items:`,
	}
	for id := 1; id <= 9; id++ {
		crisis := ""
		if id == 9 {
			crisis = ", crisis: true"
		}
		lines = append(lines, fmt.Sprintf(`  - {id: %d, text: "Q%d text"%s}`, id, id, crisis))
	}
	lines = append(lines,
		`functional_item:`,
		`  text: "Q10 how difficult?"`,
		`  scale:`,
		`    - {score: 0, label: "Not difficult at all"}`,
		`    - {score: 1, label: "Somewhat difficult"}`,
		`    - {score: 2, label: "Very difficult"}`,
		`    - {score: 3, label: "Extremely difficult"}`,
		`scoring:`,
		`  bands:`,
		`    - {min: 0, max: 4, id: minimal}`,
		`    - {min: 5, max: 9, id: mild}`,
		`    - {min: 10, max: 14, id: moderate}`,
		`    - {min: 15, max: 19, id: moderately_severe}`,
		`    - {min: 20, max: 27, id: severe}`,
		`attribution_text_ru: "PHQ-9 attribution."`,
	)
	if mutate != nil {
		lines = mutate(lines)
	}
	return strings.Join(lines, "\n") + "\n"
}

func syntheticWHO5(mutate func(lines []string) []string) string {
	lines := []string{
		`recall_header_official_ru: "Over the last two weeks"`,
		`scale:`,
		`  - {score: 5, label: "All the time"}`,
		`  - {score: 4, label: "Most of the time"}`,
		`  - {score: 3, label: "More than half the time"}`,
		`  - {score: 2, label: "Less than half the time"}`,
		`  - {score: 1, label: "Some of the time"}`,
		`  - {score: 0, label: "At no time"}`,
		`items:`,
	}
	for id := 1; id <= 5; id++ {
		lines = append(lines, fmt.Sprintf(`  - {id: %d, text: "W statement %d"}`, id, id))
	}
	lines = append(lines,
		`scoring:`,
		`  multiplier: 4`,
		`  bands:`,
		`    - {min: 0, max: 28, id: very_low}`,
		`    - {min: 29, max: 50, id: low}`,
		`    - {min: 51, max: 100, id: ok}`,
		`attribution_text_ru: "WHO-5 attribution."`,
	)
	if mutate != nil {
		lines = mutate(lines)
	}
	return strings.Join(lines, "\n") + "\n"
}

func syntheticGAD7(mutate func(lines []string) []string) string {
	lines := []string{
		`instruction_official_ru: "How often over the last 14 days?"`,
		`scale:`,
		`  - {score: 0, label: "Not at all"}`,
		`  - {score: 1, label: "Several days"}`,
		`  - {score: 2, label: "More than half"}`,
		`  - {score: 3, label: "Nearly every day"}`,
		`items:`,
	}
	for id := 1; id <= 7; id++ {
		lines = append(lines, fmt.Sprintf(`  - {id: %d, text: "G%d text"}`, id, id))
	}
	lines = append(lines,
		`scoring:`,
		`  bands:`,
		`    - {min: 0, max: 4, id: minimal}`,
		`    - {min: 5, max: 9, id: mild}`,
		`    - {min: 10, max: 14, id: moderate}`,
		`    - {min: 15, max: 21, id: severe}`,
		`attribution_text_ru: "GAD-7 attribution."`,
	)
	if mutate != nil {
		lines = mutate(lines)
	}
	return strings.Join(lines, "\n") + "\n"
}

func syntheticMoodModule() string {
	return `meta:
  title: "Mood self-check"
  disclaimer: "DISCLAIMER not a diagnosis"
menu:
  prompt: "mood menu prompt"
  who5_button: "Quick check"
  phq9_button: "PHQ-9 test"
  gad7_button: "Anxiety test"
  resume_phq9_button: "Resume PHQ-9 run"
  resume_who5_button: "Resume quick check"
  resume_gad7_button: "Resume anxiety run"
consent:
  title: "Consent"
  body: "mood consent body"
  agree_button: "Begin"
  later_button: "Not now"
  declined: "mood declined text"
crisis:
  lead: "crisis lead"
  contacts: |-
    CONTACT-LINE-1
    CONTACT-LINE-2
  urgent_line: "talk to someone today"
  continue_button: "Continue"
results:
  heading: "Your mood result"
  score_line: "PHQ-9: {score} of 27."
  bands:
    minimal: "band minimal"
    mild: "band mild"
    moderate: "band moderate"
    moderately_severe: "band moderately severe"
    severe: "band severe"
  delta_line: "Last time ({ago}) it was {prev}, now {cur}."
  ago_today: "today"
  ago_days: "{n} d ago"
  ago_weeks: "{n} w ago"
  retest_line: "retest in 2-4 weeks"
  crisis_heading: "support contacts:"
  attribution_line: "PHQ-9 - Spitzer, Williams, Kroenke"
who5:
  title: "Quick well-being check"
  progress: "Statement {current} of {total}"
  results:
    score_line: "WHO-5: {score} of 100."
    bands:
      ok: "wb ok"
      low: "wb low"
      very_low: "wb very low"
    attribution_line: "WHO5-ATTR-LINE"
  offer:
    body_low: "offer phq9 low"
    body_very_low: "offer phq9 very low"
    start_button: "Take the PHQ-9"
    later_button: "Skip"
gad7:
  title: "Anxiety check"
  results:
    score_line: "GAD-7: {score} of 21."
    bands:
      minimal: "g minimal"
      mild: "g mild"
      moderate: "g moderate - discuss"
      severe: "g severe - see a specialist"
    attribution_line: "GAD7-ATTR-LINE"
  offer:
    body: "offer gad7 body"
    start_button: "Take the GAD-7"
    later_button: "Skip"
doctor_report:
  lead_in: "mood report lead-in"
  heading: "MOOD SUMMARY REPORT"
  phq9_line: "PHQ9 {date}: {score}/27 - {band}"
  q9_line: "q9: {q9_fact}"
  q9_marked: "marked"
  q9_not_marked: "not marked"
  q10_line: "q10: {answer}"
  gad7_line: "GAD7 {date}: {score}/21 - {band}"
  who5_line: "WHO5 {date}: {score}/100 - {band}"
  footer: "REPORT-FOOTER"
ui:
  mode_button: "Mood self-check"
  progress: "Question {current} of {total}"
  abandon_confirmed: "mood abandoned"
  delete_confirm_prompt: "delete mood data?"
  delete_confirm_button: "Yes, delete"
  delete_cancel_button: "Keep"
  delete_done: "mood deleted"
  delete_nothing: "nothing mood to delete"
`
}

func writeMoodContent(t *testing.T, phq9, who5, gad7, module string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"phq9_ru.yaml":        phq9,
		"who5_ru.yaml":        who5,
		"gad7_ru.yaml":        gad7,
		"mood_module_ru.yaml": module,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func loadSyntheticMood(t *testing.T) *MoodContent {
	t.Helper()
	c, err := LoadMood(writeMoodContent(t,
		syntheticPHQ9(nil), syntheticWHO5(nil), syntheticGAD7(nil), syntheticMoodModule()))
	if err != nil {
		t.Fatalf("LoadMood: %v", err)
	}
	return c
}

// --- loader / validation ----------------------------------------------------

func TestLoadMood_SyntheticBundle(t *testing.T) {
	c := loadSyntheticMood(t)
	if got := len(c.PHQ9.Items); got != 9 {
		t.Errorf("phq9 items: %d", got)
	}
	if got := len(c.PHQ9.Scale); got != 4 {
		t.Errorf("phq9 scale: %d", got)
	}
	if !c.PHQ9.Items[8].Crisis {
		t.Error("item 9 must carry the crisis flag")
	}
	if got := c.PHQ9.FuncItem.Text; got != "Q10 how difficult?" {
		t.Errorf("functional item text: %q", got)
	}
	if got := len(c.PHQ9.FuncItem.Scale); got != 4 {
		t.Errorf("functional item scale: %d", got)
	}
	if got := len(c.WHO5.Items); got != 5 {
		t.Errorf("who5 items: %d", got)
	}
	if got := len(c.WHO5.Scale); got != 6 {
		t.Errorf("who5 scale: %d", got)
	}
	if got := c.WHO5.Scale[0].Score; got != 5 {
		t.Errorf("who5 scale must keep the form order (5 first), got %d", got)
	}
	if got := len(c.GAD7.Items); got != 7 {
		t.Errorf("gad7 items: %d", got)
	}
	if got := c.Module.Results.Bands[BandSevere]; got != "band severe" {
		t.Errorf("band severe wording: %q", got)
	}
	if got := c.Module.Who5.Results.Bands[Who5BandLow]; got != "wb low" {
		t.Errorf("who5 low wording: %q", got)
	}
	if got := c.Module.Gad7.Results.Bands[BandModerate]; got != "g moderate - discuss" {
		t.Errorf("gad7 moderate wording: %q", got)
	}
}

func TestLoadMood_RejectsForeignContent(t *testing.T) {
	type bundle struct{ phq9, who5, gad7 string }
	fresh := func() bundle {
		return bundle{syntheticPHQ9(nil), syntheticWHO5(nil), syntheticGAD7(nil)}
	}
	replaceIn := func(t *testing.T, doc, old, new string) string {
		t.Helper()
		if !strings.Contains(doc, old) {
			t.Fatalf("test needle not found: %s", old)
		}
		return strings.Replace(doc, old, new, 1)
	}

	cases := map[string]func(t *testing.T, b *bundle){
		"phq9 foreign band boundary": func(t *testing.T, b *bundle) {
			b.phq9 = replaceIn(t, b.phq9, "max: 14", "max: 15")
		},
		"phq9 crisis flag missing": func(t *testing.T, b *bundle) {
			b.phq9 = replaceIn(t, b.phq9, `, crisis: true`, ``)
		},
		"phq9 crisis flag on wrong item": func(t *testing.T, b *bundle) {
			b.phq9 = replaceIn(t, b.phq9, `{id: 5, text: "Q5 text"}`, `{id: 5, text: "Q5 text", crisis: true}`)
		},
		"phq9 eight items": func(t *testing.T, b *bundle) {
			b.phq9 = replaceIn(t, b.phq9, "  - {id: 8, text: \"Q8 text\"}\n", "")
		},
		"phq9 functional item empty text": func(t *testing.T, b *bundle) {
			b.phq9 = replaceIn(t, b.phq9, `  text: "Q10 how difficult?"`, `  text: ""`)
		},
		"phq9 functional scale short": func(t *testing.T, b *bundle) {
			b.phq9 = replaceIn(t, b.phq9, "    - {score: 3, label: \"Extremely difficult\"}\n", "")
		},
		"who5 four items": func(t *testing.T, b *bundle) {
			b.who5 = replaceIn(t, b.who5, "  - {id: 5, text: \"W statement 5\"}\n", "")
		},
		"who5 broken scale order": func(t *testing.T, b *bundle) {
			b.who5 = replaceIn(t, b.who5, `{score: 5, label: "All the time"}`, `{score: 0, label: "All the time"}`)
		},
		"who5 foreign multiplier": func(t *testing.T, b *bundle) {
			b.who5 = replaceIn(t, b.who5, "multiplier: 4", "multiplier: 5")
		},
		"who5 foreign band boundary": func(t *testing.T, b *bundle) {
			b.who5 = replaceIn(t, b.who5, "max: 50", "max: 60")
		},
		"gad7 six items": func(t *testing.T, b *bundle) {
			b.gad7 = replaceIn(t, b.gad7, "  - {id: 7, text: \"G7 text\"}\n", "")
		},
		"gad7 foreign band boundary": func(t *testing.T, b *bundle) {
			b.gad7 = replaceIn(t, b.gad7, "max: 14", "max: 15")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b := fresh()
			mutate(t, &b)
			dir := writeMoodContent(t, b.phq9, b.who5, b.gad7, syntheticMoodModule())
			if _, err := LoadMood(dir); err == nil {
				t.Error("expected LoadMood to reject the tampered content")
			}
		})
	}
}

func TestLoadMood_RejectsChildrensHelpline(t *testing.T) {
	module := strings.Replace(syntheticMoodModule(),
		"CONTACT-LINE-1", "call 8-800-2000-122 now", 1)
	dir := writeMoodContent(t, syntheticPHQ9(nil), syntheticWHO5(nil), syntheticGAD7(nil), module)
	if _, err := LoadMood(dir); err == nil {
		t.Error("expected LoadMood to reject the children's helpline in crisis contacts")
	}
}

func TestLoadMood_RejectsForbiddenBranding(t *testing.T) {
	needle := strings.ToUpper(forbiddenBranding)
	module := strings.Replace(syntheticMoodModule(),
		"mood consent body", "mood consent body mentions "+needle, 1)
	dir := writeMoodContent(t, syntheticPHQ9(nil), syntheticWHO5(nil), syntheticGAD7(nil), module)
	if _, err := LoadMood(dir); err == nil {
		t.Error("expected LoadMood to reject forbidden branding")
	}
}

// --- scoring ----------------------------------------------------------------

func TestPHQ9_Score(t *testing.T) {
	c := loadSyntheticMood(t)
	p := &c.PHQ9

	cases := []struct {
		answers []int
		want    int
	}{
		{nil, 0},
		{[]int{3, 3, 3, 3, 3, 3, 3, 3, 3}, 27},
		{[]int{1, 0, 2, 0, 3, 0, 1, 0, 2}, 9},
		{[]int{1, 1}, 2},                             // short slice zero-filled
		{[]int{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}, 27}, // extra answers (incl. q10) ignored
	}
	for _, tc := range cases {
		if got := p.Score(tc.answers); got != tc.want {
			t.Errorf("Score(%v) = %d, want %d", tc.answers, got, tc.want)
		}
	}
}

func TestPHQ9_BandBoundaries(t *testing.T) {
	c := loadSyntheticMood(t)
	p := &c.PHQ9

	cases := map[int]string{
		0: BandMinimal, 4: BandMinimal,
		5: BandMild, 9: BandMild,
		10: BandModerate, 14: BandModerate,
		15: BandModeratelySevere, 19: BandModeratelySevere,
		20: BandSevere, 27: BandSevere,
	}
	for score, want := range cases {
		if got := p.Band(score); got != want {
			t.Errorf("Band(%d) = %q, want %q", score, got, want)
		}
	}
	if got := p.Band(28); got != "" {
		t.Errorf("Band(28) = %q, want empty", got)
	}
	if got := p.Band(-1); got != "" {
		t.Errorf("Band(-1) = %q, want empty", got)
	}
}

func TestPHQ9_CrisisAnswer(t *testing.T) {
	c := loadSyntheticMood(t)
	p := &c.PHQ9

	cases := []struct {
		answers []int
		want    int
	}{
		{nil, 0},
		{[]int{3, 3, 3, 3, 3, 3, 3, 3}, 0}, // q9 not answered yet
		{[]int{0, 0, 0, 0, 0, 0, 0, 0, 0}, 0},
		{[]int{0, 0, 0, 0, 0, 0, 0, 0, 1}, 1},
		{[]int{0, 0, 0, 0, 0, 0, 0, 0, 2}, 2},
		{[]int{3, 3, 3, 3, 3, 3, 3, 3, 3}, 3},
	}
	for _, tc := range cases {
		if got := p.CrisisAnswer(tc.answers); got != tc.want {
			t.Errorf("CrisisAnswer(%v) = %d, want %d", tc.answers, got, tc.want)
		}
	}
}

// TestPHQ9_AnyPositive pins the official gate of the functional (10th)
// question: at least one of the NINE item answers > 0; a recorded functional
// answer at index 9 must not influence the gate.
func TestPHQ9_AnyPositive(t *testing.T) {
	c := loadSyntheticMood(t)
	p := &c.PHQ9

	cases := []struct {
		answers []int
		want    bool
	}{
		{nil, false},
		{[]int{0, 0, 0, 0, 0, 0, 0, 0, 0}, false},
		{[]int{1, 0, 0, 0, 0, 0, 0, 0, 0}, true},
		{[]int{0, 0, 0, 0, 0, 0, 0, 0, 3}, true},
		{[]int{0, 0, 0}, false},                      // short slice zero-filled
		{[]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 3}, false}, // index 9 (q10) must not count
	}
	for _, tc := range cases {
		if got := p.AnyPositive(tc.answers); got != tc.want {
			t.Errorf("AnyPositive(%v) = %v, want %v", tc.answers, got, tc.want)
		}
	}
}

// TestWHO5_Score pins the official ×4 scoring: raw 0–25 sum → 0–100.
func TestWHO5_Score(t *testing.T) {
	c := loadSyntheticMood(t)
	w := &c.WHO5

	cases := []struct {
		answers []int
		want    int
	}{
		{nil, 0},
		{[]int{0, 0, 0, 0, 0}, 0},
		{[]int{5, 5, 5, 5, 5}, 100},
		{[]int{3, 3, 3, 3, 0}, 48}, // raw 12 → 48 (≤ 50: reduced)
		{[]int{3, 3, 3, 3, 1}, 52}, // raw 13 → 52 (> 50: ok)
		{[]int{2, 2, 2, 1, 0}, 28}, // raw 7 → 28 (≤ 28: marked reduction)
		{[]int{5, 5}, 40},          // short slice zero-filled
	}
	for _, tc := range cases {
		if got := w.Score(tc.answers); got != tc.want {
			t.Errorf("Score(%v) = %d, want %d", tc.answers, got, tc.want)
		}
	}
}

// TestWHO5_BandBoundaries pins the interpretation boundaries: > 50 ok,
// ≤ 50 low, ≤ 28 very_low — including the exact 28/29 and 50/51 edges
// (real ×4 scores land on 28/32 and 48/52).
func TestWHO5_BandBoundaries(t *testing.T) {
	c := loadSyntheticMood(t)
	w := &c.WHO5

	cases := map[int]string{
		0:   Who5BandVeryLow,
		28:  Who5BandVeryLow,
		29:  Who5BandLow,
		32:  Who5BandLow,
		48:  Who5BandLow,
		50:  Who5BandLow,
		51:  Who5BandOK,
		52:  Who5BandOK,
		100: Who5BandOK,
	}
	for score, want := range cases {
		if got := w.Band(score); got != want {
			t.Errorf("Band(%d) = %q, want %q", score, got, want)
		}
	}
	if got := w.Band(101); got != "" {
		t.Errorf("Band(101) = %q, want empty", got)
	}
}

func TestGAD7_Score(t *testing.T) {
	c := loadSyntheticMood(t)
	g := &c.GAD7

	cases := []struct {
		answers []int
		want    int
	}{
		{nil, 0},
		{[]int{3, 3, 3, 3, 3, 3, 3}, 21},
		{[]int{1, 0, 2, 0, 1, 0, 1}, 5},
		{[]int{2, 2}, 4},                       // short slice zero-filled
		{[]int{3, 3, 3, 3, 3, 3, 3, 3, 3}, 21}, // extra answers ignored
	}
	for _, tc := range cases {
		if got := g.Score(tc.answers); got != tc.want {
			t.Errorf("Score(%v) = %d, want %d", tc.answers, got, tc.want)
		}
	}
}

// TestGAD7_BandBoundaries pins the published cutoffs at the exact 4/5, 9/10
// and 14/15 edges.
func TestGAD7_BandBoundaries(t *testing.T) {
	c := loadSyntheticMood(t)
	g := &c.GAD7

	cases := map[int]string{
		0: BandMinimal, 4: BandMinimal,
		5: BandMild, 9: BandMild,
		10: BandModerate, 14: BandModerate,
		15: BandSevere, 21: BandSevere,
	}
	for score, want := range cases {
		if got := g.Band(score); got != want {
			t.Errorf("Band(%d) = %q, want %q", score, got, want)
		}
	}
	if got := g.Band(22); got != "" {
		t.Errorf("Band(22) = %q, want empty", got)
	}
}
