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

func syntheticMoodModule() string {
	return `meta:
  title: "Mood self-check"
  disclaimer: "DISCLAIMER not a diagnosis"
consent:
  title: "Consent"
  body: "mood consent body"
  agree_button: "Begin"
  later_button: "Not now"
  declined: "mood declined text"
resume:
  body: "resume at question {current} of {total}"
  continue_button: "Continue"
  restart_button: "Start over"
  later_button: "Come back later"
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
doctor_report:
  lead_in: "mood report lead-in"
  q9_marked: "marked"
  q9_not_marked: "not marked"
  template: |-
    MOOD REPORT {date}
    score: {score}/27 - {band}
    q9: {q9_fact}
ui:
  mode_button: "Mood self-check"
  progress: "Question {current} of {total}"
  paused: "mood paused text"
  abandon_confirmed: "mood abandoned"
  delete_confirm_prompt: "delete mood data?"
  delete_confirm_button: "Yes, delete"
  delete_cancel_button: "Keep"
  delete_done: "mood deleted"
  delete_nothing: "nothing mood to delete"
`
}

func writeMoodContent(t *testing.T, phq9, module string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"phq9_ru.yaml":        phq9,
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
	c, err := LoadMood(writeMoodContent(t, syntheticPHQ9(nil), syntheticMoodModule()))
	if err != nil {
		t.Fatalf("LoadMood: %v", err)
	}
	return c
}

// --- loader / validation ----------------------------------------------------

func TestLoadMood_SyntheticBundle(t *testing.T) {
	c := loadSyntheticMood(t)
	if got := len(c.PHQ9.Items); got != 9 {
		t.Errorf("items: %d", got)
	}
	if got := len(c.PHQ9.Scale); got != 4 {
		t.Errorf("scale: %d", got)
	}
	if !c.PHQ9.Items[8].Crisis {
		t.Error("item 9 must carry the crisis flag")
	}
	if got := c.Module.Results.Bands[BandSevere]; got != "band severe" {
		t.Errorf("band severe wording: %q", got)
	}
}

func TestLoadMood_RejectsForeignContent(t *testing.T) {
	cases := map[string]func(lines []string) []string{
		"foreign band boundary": func(lines []string) []string {
			for i, l := range lines {
				if strings.Contains(l, "max: 14") {
					lines[i] = `    - {min: 10, max: 15, id: moderate}`
				}
			}
			return lines
		},
		"crisis flag missing": func(lines []string) []string {
			for i, l := range lines {
				if strings.Contains(l, "crisis: true") {
					lines[i] = `  - {id: 9, text: "Q9 text"}`
				}
			}
			return lines
		},
		"crisis flag on wrong item": func(lines []string) []string {
			for i, l := range lines {
				if strings.Contains(l, `id: 5`) {
					lines[i] = `  - {id: 5, text: "Q5 text", crisis: true}`
				}
			}
			return lines
		},
		"eight items": func(lines []string) []string {
			out := lines[:0]
			for _, l := range lines {
				if strings.Contains(l, `id: 8, text`) {
					continue
				}
				out = append(out, l)
			}
			return out
		},
		"five-option scale": func(lines []string) []string {
			for i, l := range lines {
				if strings.Contains(l, "items:") && !strings.Contains(l, "-") {
					head := append([]string{}, lines[:i]...)
					head = append(head, `  - {score: 4, label: "Extra"}`)
					return append(head, lines[i:]...)
				}
			}
			return lines
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeMoodContent(t, syntheticPHQ9(mutate), syntheticMoodModule())
			if _, err := LoadMood(dir); err == nil {
				t.Error("expected LoadMood to reject the tampered content")
			}
		})
	}
}

func TestLoadMood_RejectsChildrensHelpline(t *testing.T) {
	module := strings.Replace(syntheticMoodModule(),
		"CONTACT-LINE-1", "call 8-800-2000-122 now", 1)
	dir := writeMoodContent(t, syntheticPHQ9(nil), module)
	if _, err := LoadMood(dir); err == nil {
		t.Error("expected LoadMood to reject the children's helpline in crisis contacts")
	}
}

func TestLoadMood_RejectsForbiddenBranding(t *testing.T) {
	needle := strings.ToUpper(forbiddenBranding)
	module := strings.Replace(syntheticMoodModule(),
		"mood consent body", "mood consent body mentions "+needle, 1)
	dir := writeMoodContent(t, syntheticPHQ9(nil), module)
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
		{[]int{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}, 27}, // extra answers ignored
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
