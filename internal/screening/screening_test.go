package screening

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- synthetic content -----------------------------------------------------

// asrsMinScores mirrors the canonical per-item significance thresholds.
var asrsMinScores = map[int]int{
	1: 2, 2: 2, 3: 2, 4: 3, 5: 3, 6: 3,
	7: 3, 8: 3, 9: 2, 10: 3, 11: 3, 12: 2, 13: 3, 14: 3, 15: 3, 16: 2, 17: 3, 18: 2,
}

func syntheticASRS() string {
	var b strings.Builder
	b.WriteString("instruction_official_ru: \"Test instruction. Second paper-only sentence.\"\n")
	b.WriteString("scale:\n")
	for i, l := range []string{"never", "rarely", "sometimes", "often", "very often"} {
		fmt.Fprintf(&b, "  - {score: %d, label: %q}\n", i, l)
	}
	b.WriteString("part_a:\n  items:\n")
	for id := 1; id <= 6; id++ {
		fmt.Fprintf(&b, "    - {id: %d, domain: d, text: \"A question %d?\", significant_min_score: %d}\n",
			id, id, asrsMinScores[id])
	}
	b.WriteString("  scoring:\n    positive_screen_threshold: 4\n")
	b.WriteString("part_b:\n  translation_note: \"Unofficial translation note.\"\n  items:\n")
	for id := 7; id <= 18; id++ {
		fmt.Fprintf(&b, "    - {id: %d, domain: d, text: \"B question %d?\", significant_min_score: %d}\n",
			id, id, asrsMinScores[id])
	}
	b.WriteString("attribution_text_ru: \"ASRS attribution.\"\n")
	return b.String()
}

func syntheticWURS() string {
	var b strings.Builder
	b.WriteString("instruction_ru: \"WURS instruction.\"\n")
	b.WriteString("scale:\n")
	for i, l := range []string{"not at all", "mildly", "moderately", "quite a bit", "very much"} {
		fmt.Fprintf(&b, "  - {score: %d, label: %q}\n", i, l)
	}
	b.WriteString("items:\n")
	for id := 1; id <= 25; id++ {
		fmt.Fprintf(&b, "  - {id: %d, text_m: \"W%d male\", text_f: \"W%d female\"}\n", id, id, id)
	}
	b.WriteString("scoring:\n  thresholds:\n    - {cutoff: 46, role: primary}\n    - {cutoff: 36, role: alternative}\n")
	b.WriteString("attribution_text_ru: \"WURS attribution.\"\n")
	return b.String()
}

func syntheticModule() string {
	var b strings.Builder
	b.WriteString(`meta:
  title: "Self-check"
  disclaimer: "screening, not a diagnosis"
consent:
  title: "Consent"
  body: "consent body"
  agree_button: "Agree"
  later_button: "Later"
  declined: "declined text"
intro:
  title: "Intro"
  body: "intro body"
  start_button: "Start"
  postpone_button: "Postpone"
criterion_b:
  title: "Onset"
  question: "before 12?"
  yes_label: "Yes, back then"
  no_label: "No, later"
  no_followup: "what age?"
  age_invalid: "age as one number"
  onset_fact_childhood: "noticeable before 12"
  onset_fact_later: "appeared around {age}"
domains:
  adult_prompt: "adult prompt"
  childhood_prompt: "child prompt"
  position_adult: "Sphere {current} of {total} - now"
  position_child: "Sphere {current} of {total} - childhood"
  examples_line: "E.g.: {examples}."
  question: "Noticeable difficulties?"
  yes_button: "Yes"
  no_button: "No"
  items:
`)
	for _, id := range []string{"work_study", "relationships_family", "social", "leisure", "self_esteem"} {
		b.WriteString(domainBlock(id))
	}
	b.WriteString(`results:
  heading: "Your result"
  instruments:
    asrs_a:
      title: "ASRS part A"
      score_line: "{score} of 6 significant"
      positive_line: "screen positive"
      negative_line: "screen negative"
    asrs_b:
      title: "ASRS part B"
      score_line: "{score} of 12 significant"
      note: "no formal threshold"
    wurs:
      title: "WURS-25"
      score_line: "{score} of 100 (cutoff 46)"
      positive_line: "above cutoff"
      negative_line: "below cutoff"
  context_facts:
    heading: "Context facts"
    onset_line: "Onset: {onset_fact}."
    adult_domains_line: "Now: {adult_domains}."
    adult_domains_empty: "no adult domains"
    child_domains_line: "School years: {child_domains}."
    child_domains_empty: "no child domains"
  overall:
    consistent: "overall consistent"
    partial: "overall partial - {gap_hint}"
    not_consistent: "overall not consistent"
    gap_hints:
      no_childhood_onset: "no childhood onset hint"
      no_current_symptoms: "no current symptoms hint"
      few_domains: "few domains hint"
  attribution_line: "ATTR-LINE"
  referral:
    heading: "Where to go"
    body: "referral body"
  doctor_report:
    lead_in: "report lead-in"
    template: |-
      REPORT {date}
      A: {asrs_a_score}/6 {asrs_a_verdict}
      B: {asrs_b_score}/12
      W: {wurs_score}/100 {wurs_verdict}
      onset: {onset_fact}
      adult: {adult_domains}
      child: {child_domains}
ui:
  mode_button: "ADHD self-check"
  progress: "Question {current} of {total}"
  continue_button: "Continue"
  pause_button: "Pause"
  paused: "paused text"
  resumed: "resumed text"
  block_boundaries:
    after_asrs_a: "after A boundary"
    after_asrs_b: "after B boundary"
    after_wurs: "after WURS boundary"
  abandon_confirmed: "abandoned"
  abandon_no_active: "no active test"
  delete_confirm_prompt: "delete everything?"
  delete_confirm_button: "Yes, delete"
  delete_cancel_button: "Keep"
  delete_done: "deleted"
  delete_nothing: "nothing to delete"
  fodmap_guard: "diary untouched"
`)
	return b.String()
}

func domainBlock(id string) string {
	return fmt.Sprintf(`    - id: %s
      adult:
        title: "%s adult"
        examples: ["%s a-ex1", "%s a-ex2", "%s a-ex3"]
      childhood:
        title: "%s child"
        examples: ["%s c-ex1", "%s c-ex2"]
`, id, id, id, id, id, id, id, id)
}

// writeContentDir writes the three synthetic files, applying per-file text
// replacements first (old → new), and returns the dir.
func writeContentDir(t *testing.T, mutate map[string][2]string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"asrs_ru.yaml":       syntheticASRS(),
		"wurs25_ru.yaml":     syntheticWURS(),
		"dsm_module_ru.yaml": syntheticModule(),
	}
	for name, body := range files {
		if m, ok := mutate[name]; ok {
			if !strings.Contains(body, m[0]) {
				t.Fatalf("mutation %q not found in %s", m[0], name)
			}
			body = strings.Replace(body, m[0], m[1], 1)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// --- parser ---------------------------------------------------------------

func TestLoad_ValidSyntheticContent(t *testing.T) {
	c, err := Load(writeContentDir(t, nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(c.ASRS.PartA.Items); got != 6 {
		t.Errorf("part A items: %d", got)
	}
	if got := len(c.ASRS.PartB.Items); got != 12 {
		t.Errorf("part B items: %d", got)
	}
	if got := len(c.WURS.Items); got != 25 {
		t.Errorf("WURS items: %d", got)
	}
	if got := c.WURS.PrimaryCutoff(); got != 46 {
		t.Errorf("primary cutoff: %d", got)
	}
	if got := c.ASRS.PartA.Scoring.PositiveScreenThreshold; got != 4 {
		t.Errorf("part A threshold: %d", got)
	}
	if got := len(c.Module.Domains.Items); got != 5 {
		t.Errorf("domains: %d", got)
	}
	if c.Module.Results.Overall.GapHints[GapFewDomains] == "" {
		t.Error("gap hint missing")
	}
	if c.ASRS.PartB.Items[0].ID != 7 {
		t.Errorf("part B first id: %d", c.ASRS.PartB.Items[0].ID)
	}
	d := c.Module.Domains
	if d.PositionAdult == "" || d.PositionChild == "" || d.ExamplesLine == "" ||
		d.Question == "" || d.YesButton == "" || d.NoButton == "" {
		t.Errorf("per-domain flow fields not parsed: %+v", d)
	}
	if c.Module.Results.AttributionLine != "ATTR-LINE" {
		t.Errorf("attribution_line: %q", c.Module.Results.AttributionLine)
	}
}

func TestLoad_RejectsBrokenContent(t *testing.T) {
	cases := []struct {
		name   string
		mutate map[string][2]string
	}{
		{"missing part A item", map[string][2]string{
			"asrs_ru.yaml": {"    - {id: 6, domain: d, text: \"A question 6?\", significant_min_score: 3}\n", ""},
		}},
		{"missing part B item", map[string][2]string{
			"asrs_ru.yaml": {"    - {id: 18, domain: d, text: \"B question 18?\", significant_min_score: 2}\n", ""},
		}},
		{"id gap in part A", map[string][2]string{
			"asrs_ru.yaml": {"{id: 3, domain: d", "{id: 30, domain: d"},
		}},
		{"significant_min_score out of range", map[string][2]string{
			"asrs_ru.yaml": {"text: \"A question 1?\", significant_min_score: 2", "text: \"A question 1?\", significant_min_score: 4"},
		}},
		{"part A threshold pinned to 4", map[string][2]string{
			"asrs_ru.yaml": {"positive_screen_threshold: 4", "positive_screen_threshold: 3"},
		}},
		{"instruction without period", map[string][2]string{
			"asrs_ru.yaml": {"instruction_official_ru: \"Test instruction. Second paper-only sentence.\"", "instruction_official_ru: \"no period here\""},
		}},
		{"missing WURS item", map[string][2]string{
			"wurs25_ru.yaml": {"  - {id: 25, text_m: \"W25 male\", text_f: \"W25 female\"}\n", ""},
		}},
		{"no primary WURS threshold", map[string][2]string{
			"wurs25_ru.yaml": {"role: primary", "role: something"},
		}},
		{"primary cutoff pinned to 46", map[string][2]string{
			"wurs25_ru.yaml": {"{cutoff: 46, role: primary}", "{cutoff: 45, role: primary}"},
		}},
		{"empty text_f", map[string][2]string{
			"wurs25_ru.yaml": {"text_f: \"W7 female\"", "text_f: \"\""},
		}},
		{"not 5 domains", map[string][2]string{
			"dsm_module_ru.yaml": {domainBlock("self_esteem"), ""},
		}},
		{"empty domain id", map[string][2]string{
			"dsm_module_ru.yaml": {"    - id: self_esteem", "    - id_off: self_esteem"},
		}},
		{"duplicate domain id", map[string][2]string{
			"dsm_module_ru.yaml": {"    - id: self_esteem", "    - id: social"},
		}},
		{"empty gap hint", map[string][2]string{
			"dsm_module_ru.yaml": {"few_domains: \"few domains hint\"", "few_domains: \"\""},
		}},
		{"empty consent body", map[string][2]string{
			"dsm_module_ru.yaml": {"body: \"consent body\"", "body: \"\""},
		}},
		{"empty per-domain question", map[string][2]string{
			"dsm_module_ru.yaml": {"question: \"Noticeable difficulties?\"", "question: \"\""},
		}},
		{"empty yes button", map[string][2]string{
			"dsm_module_ru.yaml": {"yes_button: \"Yes\"", "yes_button: \"\""},
		}},
		{"empty attribution line", map[string][2]string{
			"dsm_module_ru.yaml": {"attribution_line: \"ATTR-LINE\"", "attribution_line: \"\""},
		}},
		{"more than 3 examples breaks the one-line canon", map[string][2]string{
			"dsm_module_ru.yaml": {"examples: [\"social a-ex1\", \"social a-ex2\", \"social a-ex3\"]",
				"examples: [\"social a-ex1\", \"social a-ex2\", \"social a-ex3\", \"social a-ex4\"]"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(writeContentDir(t, tc.mutate)); err == nil {
				t.Fatalf("Load: expected error, got nil")
			}
		})
	}
}

func TestLoad_MissingFileAndDir(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing dir")
	}
	dir := writeContentDir(t, nil)
	if err := os.Remove(filepath.Join(dir, "wurs25_ru.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFirstSentence(t *testing.T) {
	cases := []struct{ in, want string }{
		{"One. Two.", "One."},
		{"Only one sentence.", "Only one sentence."},
		{"no period", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := FirstSentence(tc.in); got != tc.want {
			t.Errorf("FirstSentence(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- scoring --------------------------------------------------------------

func loadSynthetic(t *testing.T) *Content {
	t.Helper()
	c, err := Load(writeContentDir(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestScorePartA(t *testing.T) {
	c := loadSynthetic(t)
	cases := []struct {
		name        string
		answers     []int
		significant int
		positive    bool
	}{
		{"all very often", []int{4, 4, 4, 4, 4, 4}, 6, true},
		{"exactly 4 significant is positive", []int{2, 2, 2, 3, 1, 0}, 4, true},
		{"exactly 3 significant is negative", []int{2, 2, 2, 1, 1, 1}, 3, false},
		{"sometimes counts for 1-3 not 4-6", []int{2, 2, 2, 2, 2, 2}, 3, false},
		{"often counts everywhere", []int{3, 3, 3, 3, 3, 3}, 6, true},
		{"all rarely", []int{1, 1, 1, 1, 1, 1}, 0, false},
		{"short slice zero-filled", []int{2, 2}, 2, false},
		{"nil slice", nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sig, pos := c.ASRS.ScorePartA(tc.answers)
			if sig != tc.significant || pos != tc.positive {
				t.Errorf("got (%d, %v), want (%d, %v)", sig, pos, tc.significant, tc.positive)
			}
		})
	}
}

func TestScorePartB_PerItemThresholds(t *testing.T) {
	c := loadSynthetic(t)

	// "Sometimes" (2) is significant only for questions 9, 12, 16, 18.
	allSometimes := make([]int, 12)
	for i := range allSometimes {
		allSometimes[i] = 2
	}
	if got := c.ASRS.ScorePartB(allSometimes); got != 4 {
		t.Errorf("all sometimes: got %d significant, want 4", got)
	}

	allOften := make([]int, 12)
	for i := range allOften {
		allOften[i] = 3
	}
	if got := c.ASRS.ScorePartB(allOften); got != 12 {
		t.Errorf("all often: got %d, want 12", got)
	}

	if got := c.ASRS.ScorePartB(nil); got != 0 {
		t.Errorf("nil answers: got %d, want 0", got)
	}

	// Question 9 is answers index 2; question 7 is index 0.
	one := make([]int, 12)
	one[2] = 2
	if got := c.ASRS.ScorePartB(one); got != 1 {
		t.Errorf("sometimes on q9: got %d, want 1", got)
	}
	one = make([]int, 12)
	one[0] = 2
	if got := c.ASRS.ScorePartB(one); got != 0 {
		t.Errorf("sometimes on q7: got %d, want 0", got)
	}
}

func TestScoreWURS(t *testing.T) {
	c := loadSynthetic(t)

	mk := func(total int) []int {
		out := make([]int, 25)
		for i := range out {
			v := total - i*4
			switch {
			case v >= 4:
				out[i] = 4
			case v > 0:
				out[i] = v
			}
		}
		return out
	}

	cases := []struct {
		name     string
		answers  []int
		sum      int
		positive bool
	}{
		{"all zeros", make([]int, 25), 0, false},
		{"all fours", mk(100), 100, true},
		{"exactly 46 is positive", mk(46), 46, true},
		{"exactly 45 is negative", mk(45), 45, false},
		{"nil slice", nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sum, pos := c.WURS.ScoreWURS(tc.answers)
			if sum != tc.sum || pos != tc.positive {
				t.Errorf("got (%d, %v), want (%d, %v)", sum, pos, tc.sum, tc.positive)
			}
		})
	}
}

func TestOverallVerdict_FullMatrix(t *testing.T) {
	cases := []struct {
		a, b    bool
		domains int
		verdict string
		hint    string
	}{
		{true, true, 2, VerdictConsistent, ""},
		{true, true, 5, VerdictConsistent, ""},
		{false, false, 0, VerdictNotConsistent, ""},
		{false, false, 1, VerdictNotConsistent, ""}, // 1 domain → D not met
		{false, true, 2, VerdictPartial, GapNoCurrentSymptoms},
		{false, true, 0, VerdictPartial, GapNoCurrentSymptoms},
		{false, false, 2, VerdictPartial, GapNoCurrentSymptoms},
		{true, false, 2, VerdictPartial, GapNoChildhoodOnset},
		{true, false, 0, VerdictPartial, GapNoChildhoodOnset},
		{true, true, 1, VerdictPartial, GapFewDomains}, // boundary: 1 domain
		{true, true, 0, VerdictPartial, GapFewDomains},
	}
	for _, tc := range cases {
		verdict, hint := OverallVerdict(tc.a, tc.b, tc.domains)
		if verdict != tc.verdict || hint != tc.hint {
			t.Errorf("OverallVerdict(%v, %v, %d) = (%q, %q), want (%q, %q)",
				tc.a, tc.b, tc.domains, verdict, hint, tc.verdict, tc.hint)
		}
	}
}
