package journey_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/state"
	"github.com/padington/tgbase/internal/store"
)

// --- synthetic screening content -------------------------------------------
// The validator demands the full canonical shape (6+12+25 items, 5 domains),
// so the test set generates a complete synthetic bundle with short texts.

// scrMinScores mirrors the canonical per-item ASRS significance thresholds.
var scrMinScores = map[int]int{
	1: 2, 2: 2, 3: 2, 4: 3, 5: 3, 6: 3,
	7: 3, 8: 3, 9: 2, 10: 3, 11: 3, 12: 2, 13: 3, 14: 3, 15: 3, 16: 2, 17: 3, 18: 2,
}

var scrDomainIDs = []string{"work_study", "relationships_family", "social", "leisure", "self_esteem"}

func scrSyntheticASRS() string {
	var b strings.Builder
	b.WriteString("instruction_official_ru: \"Test instruction. Paper-only sentence.\"\n")
	b.WriteString("scale:\n")
	for i, l := range []string{"Never", "Rarely", "Sometimes", "Often", "Very Often"} {
		fmt.Fprintf(&b, "  - {score: %d, label: %q}\n", i, l)
	}
	b.WriteString("part_a:\n  items:\n")
	for id := 1; id <= 6; id++ {
		fmt.Fprintf(&b, "    - {id: %d, domain: d, text: \"A question %d?\", significant_min_score: %d}\n",
			id, id, scrMinScores[id])
	}
	b.WriteString("  scoring:\n    positive_screen_threshold: 4\n")
	b.WriteString("part_b:\n  translation_note: \"Unofficial translation note.\"\n  items:\n")
	for id := 7; id <= 18; id++ {
		fmt.Fprintf(&b, "    - {id: %d, domain: d, text: \"B question %d?\", significant_min_score: %d}\n",
			id, id, scrMinScores[id])
	}
	b.WriteString("attribution_text_ru: \"ASRS attribution.\"\n")
	return b.String()
}

func scrSyntheticWURS() string {
	var b strings.Builder
	b.WriteString("instruction_ru: \"WURS instruction.\"\n")
	b.WriteString("scale:\n")
	for i := 0; i <= 4; i++ {
		fmt.Fprintf(&b, "  - {score: %d, label: \"W%d\"}\n", i, i)
	}
	b.WriteString("items:\n")
	for id := 1; id <= 25; id++ {
		fmt.Fprintf(&b, "  - {id: %d, text_m: \"W%d male\", text_f: \"W%d female\"}\n", id, id, id)
	}
	b.WriteString("scoring:\n  thresholds:\n    - {cutoff: 46, role: primary}\n")
	b.WriteString("attribution_text_ru: \"WURS attribution.\"\n")
	return b.String()
}

func scrSyntheticModule() string {
	var b strings.Builder
	b.WriteString(`meta:
  title: "Self-check"
  disclaimer: "DISCLAIMER not a diagnosis"
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
	for _, id := range scrDomainIDs {
		fmt.Fprintf(&b, `    - id: %s
      adult:
        title: "%s adult"
        examples: ["%s a-ex1", "%s a-ex2", "%s a-ex3"]
      childhood:
        title: "%s child"
        examples: ["%s c-ex1", "%s c-ex2"]
`, id, id, id, id, id, id, id, id)
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
  attribution_line: "ATTR-LINE"
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
  abandon_confirmed: "screening abandoned"
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

func writeScrContent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"asrs_ru.yaml":       scrSyntheticASRS(),
		"wurs25_ru.yaml":     scrSyntheticWURS(),
		"dsm_module_ru.yaml": scrSyntheticModule(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const scrTestBundle = `phase.defecation.prompt: "Defecation? 1/2/3"
phase.defecation.invalid: "Choose 1, 2, or 3."
phase.defecation.reminder: "Still there?"
phase.product.prompt: "Pick:"
phase.product.invalid: "Tap a button."
phase.product.exhausted: "All done."
phase.stage_choice.prompt: "Recommended for {name}: {recommended}.{note} Pick a volume:"
phase.stage_choice.invalid: "Tap one of the amounts."
phase.stage.prompt: "Take {description}. Check in {checkin}."
phase.stage.next: "Now take {description}."
phase.stage.checkin: "OK after {description}?"
phase.stage.checkin_invalid: "yes/no"
phase.stage.completed: "{name} done!"
phase.stage.not_tolerated: "{name} skipped."
about: "Bot info"
button.yes: "yes"
button.no: "no"
cmd.report.empty: "nothing"
cmd.report.heading: "Progress:"
cmd.report.in_progress: "active: {name}({stage})"
cmd.report.completed: "done: {names}"
cmd.report.not_tolerated: "skip: {names}"
cmd.report.interrupted: "halt: {names}"
cmd.abandon.confirmed: "abandoned {name}"
cmd.abandon.no_active: "nothing active"
product.measure_template.pieces: "{value} {name}"
product.measure_template.grams: "{value}g {name}"
product.amount_template.pieces: "{value}"
product.amount_template.grams: "{value}g"
phase.mode.prompt: "Mode?"
phase.mode.prompt_active_trial: "Mode? active {trial}"
phase.mode.invalid: "Tap a mode."
button.mode.fodmap: "Diary"
button.mode.screening: "Check"
button.mode.mood: "Mood"
button.mode.report: "Report"
button.mode.resume_fodmap: "Resume diary: {trial}"
button.mode.resume_screening: "Resume check"
button.mode.resume_mood: "Resume mood"
button.menu.home: "Home"
scr.text: "{text}"
scr.invalid_button: "Tap a screening button."
scr.invalid_scale: "Tap a scale button."
scr.wurs.form.prompt: "Which wording?"
button.scr.form.m: "Masculine"
button.scr.form.f: "Feminine"
scr.delete.cancelled: "kept everything"
cmd.report.screening: "ADHD {date}: A {asrs_a}/6 (>={a_thr}) {a_verdict}; B {asrs_b}/12; W {wurs}/100 (>={w_thr}) {w_verdict}; {overall}"
cmd.report.mood: "Mood {date}: {score} of 27"
cmd.report.gad7: "Anxiety {date}: {score} of 21"
cmd.report.who5: "WHO5 {date}: {score} of 100"
scr.report.positive: "positive"
scr.report.negative: "negative"
scr.report.domains_empty: "none marked"
scr.report.overall.consistent: "matches DSM-5 pattern"
scr.report.overall.partial: "partial picture"
scr.report.overall.not_consistent: "does not match DSM-5 pattern"
`

// setupScr builds a runner with both the FODMAP phases and the full
// screening chain registered, backed by a synthetic content bundle.
func setupScr(t *testing.T) (*journey.Runner, *state.Store, *mockSender, store.Backend) {
	t.Helper()
	dir := t.TempDir()

	i18nDir := filepath.Join(dir, "i18n")
	if err := os.Mkdir(i18nDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(i18nDir, "en.yaml"), []byte(scrTestBundle), 0o600); err != nil {
		t.Fatal(err)
	}
	trans, err := i18n.Load(i18nDir, "en")
	if err != nil {
		t.Fatal(err)
	}

	prodSeed := filepath.Join(dir, "products.yaml")
	if err := os.WriteFile(prodSeed, []byte(`- name: Apple
  fodmap: high
  measure: pieces
  stages: { low: 0.25, medium: 0.5, high: 1.0 }
- name: Cashews
  fodmap: high
  measure: grams
  stages: { low: 10, medium: 20, high: 30 }
`), 0o600); err != nil {
		t.Fatal(err)
	}

	settingsSeed := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(settingsSeed, []byte(`defecation_reminder_after: 1m
checkin_interval: 30m
scan_interval: 10s
default_locale: en
`), 0o600); err != nil {
		t.Fatal(err)
	}

	content, err := screening.Load(writeScrContent(t))
	if err != nil {
		t.Fatalf("load synthetic screening content: %v", err)
	}
	moodContent, err := screening.LoadMood(writeMoodTestContent(t))
	if err != nil {
		t.Fatalf("load synthetic mood content: %v", err)
	}

	backend := store.NewMemoryBackend()
	cat, err := products.New(backend, prodSeed)
	if err != nil {
		t.Fatal(err)
	}
	settingsStore, err := settings.New(backend, settingsSeed)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStoreFromBackend(backend)
	sender := &mockSender{}

	runner := journey.New(stateStore, sender, cat, settingsStore, trans)
	runner.Register(journey.NewModeChoicePhase())
	runner.Register(journey.NewDefecationPhase())
	runner.Register(journey.NewProductCategoryPhase())
	runner.Register(journey.NewProductChoicePhase())
	runner.Register(journey.NewStageChoicePhase())
	runner.Register(journey.NewStageCheckinPhase())
	registerScreeningPhases(runner, content)
	registerMoodPhases(runner, moodContent)

	return runner, stateStore, sender, backend
}

func registerScreeningPhases(runner *journey.Runner, content *screening.Content) {
	runner.Register(journey.NewScrConsentPhase(content))
	runner.Register(journey.NewScrIntroPhase(content))
	runner.Register(journey.NewScrAsrsAPhase(content))
	runner.Register(journey.NewScrAsrsAGatePhase(content))
	runner.Register(journey.NewScrAsrsBPhase(content))
	runner.Register(journey.NewScrAsrsBGatePhase(content))
	runner.Register(journey.NewScrWursFormPhase(content))
	runner.Register(journey.NewScrWursPhase(content))
	runner.Register(journey.NewScrWursGatePhase(content))
	runner.Register(journey.NewScrOnsetPhase(content))
	runner.Register(journey.NewScrOnsetAgePhase(content))
	runner.Register(journey.NewScrDomainsAdultPhase(content))
	runner.Register(journey.NewScrDomainsChildPhase(content))
	runner.Register(journey.NewScrReferralPhase(content))
	runner.Register(journey.NewScrReportPhase(content))
	runner.Register(journey.NewScrDeleteConfirmPhase(content))
}

// --- drive helpers ---------------------------------------------------------

func say(runner *journey.Runner, sender *mockSender, id int64, text string) {
	runner.HandleText(sender, newMsg(id, text))
}

func rep(label string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = label
	}
	return out
}

func sentTexts(sender *mockSender) []string {
	var out []string
	for _, c := range sender.snapshot() {
		if m, ok := c.(tgbotapi.MessageConfig); ok {
			out = append(out, m.Text)
		}
	}
	return out
}

func resetSender(sender *mockSender) {
	sender.mu.Lock()
	sender.sent = nil
	sender.mu.Unlock()
}

func rawUsersJSON(t *testing.T, backend store.Backend) string {
	t.Helper()
	raw, err := backend.Get("users")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// fullRun describes one complete pass through the screening.
type fullRun struct {
	asrs     []string // 18 scale labels
	wurs     []string // 25 scale labels
	form     string   // "Masculine" | "Feminine"
	onsetYes bool
	age      string   // typed when onsetYes == false
	adult    []string // domain ids answered "Yes" in the adult pass
	child    []string // domain ids answered "Yes" in the childhood pass
}

func happyRun() fullRun {
	return fullRun{
		asrs:     rep("Very Often", 18),
		wurs:     rep("W4", 25),
		form:     "Feminine",
		onsetYes: true,
		adult:    []string{"work_study", "social"},
	}
}

// answerDomains walks one per-domain pass, answering "Yes" for the ids in
// yesIDs and "No" for the rest.
func answerDomains(runner *journey.Runner, sender *mockSender, id int64, yesIDs []string) {
	for _, domain := range scrDomainIDs {
		answer := "No"
		for _, y := range yesIDs {
			if y == domain {
				answer = "Yes"
			}
		}
		say(runner, sender, id, answer)
	}
}

// drive walks a fullRun from /adhd to the final report.
func drive(t *testing.T, runner *journey.Runner, sender *mockSender, id int64, run fullRun) {
	t.Helper()
	runner.HandleAdhd(sender, newMsg(id, "/adhd"))
	say(runner, sender, id, "Agree")
	say(runner, sender, id, "Start")
	for _, l := range run.asrs[:6] {
		say(runner, sender, id, l)
	}
	say(runner, sender, id, "Continue") // A gate
	for _, l := range run.asrs[6:] {
		say(runner, sender, id, l)
	}
	say(runner, sender, id, "Continue") // B gate
	say(runner, sender, id, run.form)
	for _, l := range run.wurs {
		say(runner, sender, id, l)
	}
	say(runner, sender, id, "Continue") // WURS gate
	if run.onsetYes {
		say(runner, sender, id, "Yes, back then")
	} else {
		say(runner, sender, id, "No, later")
		say(runner, sender, id, run.age)
	}
	answerDomains(runner, sender, id, run.adult)
	answerDomains(runner, sender, id, run.child)
}

// --- scenarios -------------------------------------------------------------

func TestScr_AdhdFromIdle_ConsentDeclineLeavesNoTrace(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	if got := st.Get(1).State; got != state.StateScrConsent {
		t.Fatalf("expected ScrConsent, got %q", got)
	}
	if !contains(sender.lastText(), "consent body") {
		t.Errorf("consent text not sent, got %q", sender.lastText())
	}

	say(runner, sender, 1, "Later")
	d := st.Get(1)
	// Every test exit lands on the home landing (never mid-diary, never a
	// dead idle): the decline text is followed by the landing prompt.
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after decline, got %q", d.State)
	}
	if d.Screening != nil {
		t.Error("decline must not create Screening")
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "declined text") {
		t.Errorf("declined text not sent, got %q", texts)
	}
	if !contains(sender.lastText(), "Mode?") {
		t.Errorf("landing prompt must follow the decline, got %q", sender.lastText())
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, "screening") {
		t.Errorf("persisted JSON must not mention screening after decline:\n%s", raw)
	}
}

func TestScr_IntroIsLeanAndShowsFirstQuestion(t *testing.T) {
	runner, _, sender, _ := setupScr(t)

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")

	intro := sender.lastText()
	for _, want := range []string{"intro body", "DISCLAIMER"} {
		if !contains(intro, want) {
			t.Errorf("intro missing %q:\n%s", want, intro)
		}
	}
	// Owner decision: no attribution paragraphs in the intro — the single
	// compact attribution line lives in the result footer only.
	if contains(intro, "ATTR-LINE") {
		t.Errorf("intro must not carry attributions:\n%s", intro)
	}

	say(runner, sender, 1, "Start")
	q1 := sender.lastText()
	for _, want := range []string{"ASRS part A", "Test instruction.", "Question 1 of 6", "A question 1?"} {
		if !contains(q1, want) {
			t.Errorf("first question missing %q:\n%s", want, q1)
		}
	}
	if contains(q1, "Paper-only") {
		t.Errorf("second (paper-only) instruction sentence must be omitted:\n%s", q1)
	}
}

func TestScr_HappyPathEndToEnd(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	drive(t, runner, sender, 1, happyRun())

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after completion, got %q", d.State)
	}
	if d.Screening != nil {
		t.Error("Screening must be wiped on completion")
	}
	res := d.ScreeningResult
	if res == nil {
		t.Fatal("ScreeningResult must be stored")
	}
	if res.AsrsASignificant != 6 || !res.AsrsAPositive || res.AsrsAThreshold != 4 {
		t.Errorf("ASRS-A: got %d/%v/thr=%d", res.AsrsASignificant, res.AsrsAPositive, res.AsrsAThreshold)
	}
	if res.AsrsBSignificant != 12 {
		t.Errorf("ASRS-B significant: got %d", res.AsrsBSignificant)
	}
	if res.WursScore != 100 || !res.WursPositive || res.WursCutoff != 46 {
		t.Errorf("WURS: got %d/%v/cutoff=%d", res.WursScore, res.WursPositive, res.WursCutoff)
	}
	if !res.OnsetChildhood || res.OnsetAge != 0 {
		t.Errorf("onset: got childhood=%v age=%d", res.OnsetChildhood, res.OnsetAge)
	}
	if len(res.AdultDomains) != 2 || len(res.ChildDomains) != 0 {
		t.Errorf("domains: adult=%v child=%v", res.AdultDomains, res.ChildDomains)
	}
	if res.Verdict != "consistent" || res.GapHint != "" {
		t.Errorf("verdict: got %q/%q", res.Verdict, res.GapHint)
	}

	raw := rawUsersJSON(t, backend)
	if strings.Contains(raw, "asrs_answers") || strings.Contains(raw, `"screening":`) {
		t.Errorf("raw answers must not survive completion:\n%s", raw)
	}
	if !strings.Contains(raw, "screening_result") {
		t.Error("screening_result missing from persisted JSON")
	}

	texts := sentTexts(sender)
	if len(texts) < 4 {
		t.Fatalf("expected at least 3 final messages + the landing, got %d", len(texts))
	}
	// The final chain is summary → referral → doctor report → home landing.
	summary, referral, report := texts[len(texts)-4], texts[len(texts)-3], texts[len(texts)-2]
	if !contains(texts[len(texts)-1], "Mode?") {
		t.Errorf("the landing prompt must close the test, got %q", texts[len(texts)-1])
	}
	for _, want := range []string{
		"Your result",
		"ASRS part A", "6 of 6 significant", "screen positive",
		"ASRS part B", "12 of 12 significant", "no formal threshold",
		"WURS-25", "100 of 100", "above cutoff",
		"Context facts", "noticeable before 12", "work_study adult, social adult", "no child domains",
		"overall consistent", "DISCLAIMER", "ATTR-LINE",
	} {
		if !contains(summary, want) {
			t.Errorf("summary missing %q:\n%s", want, summary)
		}
	}
	if contains(summary, "{score}") || contains(summary, "{gap_hint}") {
		t.Errorf("unrendered placeholder in summary:\n%s", summary)
	}
	for _, want := range []string{"Where to go", "referral body"} {
		if !contains(referral, want) {
			t.Errorf("referral missing %q:\n%s", want, referral)
		}
	}
	for _, want := range []string{
		"report lead-in", "REPORT " + res.TakenAt.Format("02.01.2006"),
		"A: 6/6 positive", "B: 12/12", "W: 100/100 positive",
		"onset: noticeable before 12", "adult: work_study adult, social adult", "child: none marked",
	} {
		if !contains(report, want) {
			t.Errorf("doctor report missing %q:\n%s", want, report)
		}
	}
}

func TestScr_WursFeminineWordingIsUsed(t *testing.T) {
	runner, _, sender, _ := setupScr(t)

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	for _, l := range rep("Very Often", 6) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	for _, l := range rep("Very Often", 12) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	say(runner, sender, 1, "Feminine")
	if got := sender.lastText(); !contains(got, "W1 female") {
		t.Errorf("expected feminine wording of item 1, got %q", got)
	}
	say(runner, sender, 1, "W0")
	if got := sender.lastText(); !contains(got, "W2 female") {
		t.Errorf("expected feminine wording of item 2, got %q", got)
	}
}

func TestScr_AsrsAGateShowsIntermediateResult_BGateDoesNot(t *testing.T) {
	runner, _, sender, _ := setupScr(t)

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	for _, l := range rep("Very Often", 6) {
		say(runner, sender, 1, l)
	}
	gateA := sender.lastText()
	for _, want := range []string{"ASRS part A", "6 of 6 significant", "screen positive", "after A boundary"} {
		if !contains(gateA, want) {
			t.Errorf("A gate missing %q:\n%s", want, gateA)
		}
	}

	say(runner, sender, 1, "Continue")
	for _, l := range rep("Very Often", 12) {
		say(runner, sender, 1, l)
	}
	gateB := sender.lastText()
	if !contains(gateB, "after B boundary") {
		t.Errorf("B gate missing boundary text: %q", gateB)
	}
	if contains(gateB, "significant") {
		t.Errorf("B gate must not show scores: %q", gateB)
	}
}

func TestScr_AsrsABoundary_4PositiveVs3Negative(t *testing.T) {
	// Exactly 4 significant part-A answers → positive screen; the full run
	// with childhood onset and 2 domains stays consistent.
	pos := happyRun()
	pos.asrs = append([]string{"Sometimes", "Sometimes", "Sometimes", "Often", "Never", "Never"}, rep("Never", 12)...)
	runner, st, sender, _ := setupScr(t)
	drive(t, runner, sender, 1, pos)
	res := st.Get(1).ScreeningResult
	if res == nil || res.AsrsASignificant != 4 || !res.AsrsAPositive {
		t.Fatalf("want 4/positive, got %+v", res)
	}
	if res.Verdict != "consistent" {
		t.Errorf("verdict: got %q", res.Verdict)
	}

	// Exactly 3 → negative; with childhood onset and 2 domains the overall
	// is partial with the no-current-symptoms hint.
	neg := happyRun()
	neg.asrs = append([]string{"Sometimes", "Sometimes", "Sometimes", "Never", "Never", "Never"}, rep("Never", 12)...)
	runner2, st2, sender2, _ := setupScr(t)
	drive(t, runner2, sender2, 1, neg)
	res2 := st2.Get(1).ScreeningResult
	if res2 == nil || res2.AsrsASignificant != 3 || res2.AsrsAPositive {
		t.Fatalf("want 3/negative, got %+v", res2)
	}
	if res2.Verdict != "partial" || res2.GapHint != "no_current_symptoms" {
		t.Errorf("verdict: got %q/%q", res2.Verdict, res2.GapHint)
	}
	texts := sentTexts(sender2)
	summary := texts[len(texts)-4] // summary → referral → report → landing
	if !contains(summary, "overall partial - no current symptoms hint") {
		t.Errorf("summary missing rendered gap hint:\n%s", summary)
	}
}

func TestScr_WursBoundary_46PositiveVs45Negative(t *testing.T) {
	pos := happyRun()
	pos.wurs = append(append(rep("W4", 11), "W2"), rep("W0", 13)...) // 46
	runner, st, sender, _ := setupScr(t)
	drive(t, runner, sender, 1, pos)
	res := st.Get(1).ScreeningResult
	if res == nil || res.WursScore != 46 || !res.WursPositive {
		t.Fatalf("want 46/positive, got %+v", res)
	}
	texts := sentTexts(sender)
	if !contains(texts[len(texts)-4], "above cutoff") { // summary is 4th from the end
		t.Error("summary should render the positive WURS line")
	}

	neg := happyRun()
	neg.wurs = append(append(rep("W4", 11), "W1"), rep("W0", 13)...) // 45
	runner2, st2, sender2, _ := setupScr(t)
	drive(t, runner2, sender2, 1, neg)
	res2 := st2.Get(1).ScreeningResult
	if res2 == nil || res2.WursScore != 45 || res2.WursPositive {
		t.Fatalf("want 45/negative, got %+v", res2)
	}
	if !contains(sentTexts(sender2)[len(sentTexts(sender2))-4], "below cutoff") {
		t.Error("summary should render the negative WURS line")
	}
}

func TestScr_OnsetAgeValidationAndFact(t *testing.T) {
	run := happyRun()
	run.onsetYes = false
	run.age = "16"
	runner, st, sender, _ := setupScr(t)

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	for _, l := range run.asrs[:6] {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	for _, l := range run.asrs[6:] {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	say(runner, sender, 1, run.form)
	for _, l := range run.wurs {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	say(runner, sender, 1, "No, later")
	if got := sender.lastText(); !contains(got, "what age?") {
		t.Fatalf("age follow-up not asked: %q", got)
	}
	for _, bad := range []string{"abc", "0", "150"} {
		say(runner, sender, 1, bad)
		if got := sender.lastText(); !contains(got, "age as one number") {
			t.Errorf("input %q: expected age_invalid, got %q", bad, got)
		}
		if got := st.Get(1).State; got != state.StateScrOnsetAge {
			t.Errorf("input %q: state moved to %q", bad, got)
		}
	}
	say(runner, sender, 1, "16")
	if got := st.Get(1).State; got != state.StateScrDomainsAdult {
		t.Fatalf("expected domains after valid age, got %q", got)
	}
	answerDomains(runner, sender, 1, run.adult)
	answerDomains(runner, sender, 1, nil)

	res := st.Get(1).ScreeningResult
	if res == nil || res.OnsetChildhood || res.OnsetAge != 16 {
		t.Fatalf("onset fact: got %+v", res)
	}
	texts := sentTexts(sender)
	if !contains(texts[len(texts)-4], "appeared around 16") { // summary is 4th from the end
		t.Errorf("summary missing onset_fact_later:\n%s", texts[len(texts)-4])
	}
}

func TestScr_GatePauseAndResumeWithoutReconsent(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	for _, l := range rep("Very Often", 6) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Pause")

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after pause, got %q", d.State)
	}
	if d.Screening == nil || d.Screening.ResumeState != state.StateScrAsrsAGate {
		t.Fatalf("ResumeState not recorded: %+v", d.Screening)
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "paused text") {
		t.Errorf("paused text not sent: %q", texts)
	}

	resetSender(sender)
	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	resume := sender.lastText()
	if !contains(resume, "resumed text") || !contains(resume, "ASRS part A") {
		t.Errorf("resume intro wrong: %q", resume)
	}
	if contains(resume, "consent body") {
		t.Error("consent must not be asked again on resume")
	}

	say(runner, sender, 1, "Continue")
	gate := sender.lastText()
	if !contains(gate, "6 of 6 significant") || !contains(gate, "after A boundary") {
		t.Errorf("gate not re-shown with intermediate result: %q", gate)
	}
	if got := st.Get(1).State; got != state.StateScrAsrsAGate {
		t.Errorf("expected the A gate again, got %q", got)
	}
}

func TestScr_StartMidScreening_RestartResetsAnswers(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	// First full pass stores a result.
	drive(t, runner, sender, 1, happyRun())
	first := st.Get(1).ScreeningResult
	if first == nil || first.AsrsASignificant != 6 {
		t.Fatalf("first pass result: %+v", first)
	}

	// Second pass, /start midway, restart from scratch.
	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	say(runner, sender, 1, "Very Often")
	say(runner, sender, 1, "Very Often")

	runner.HandleStart(sender, newMsg(1, "/start"))
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Fatalf("expected mode fork, got %q", got)
	}
	say(runner, sender, 1, "Check")
	resume := sender.lastText()
	if !contains(resume, "resumed text") {
		t.Fatalf("expected resume intro, got %q", resume)
	}
	say(runner, sender, 1, "Start")
	if got := sender.lastText(); !contains(got, "Question 1 of 6") {
		t.Fatalf("restart should begin from question 1, got %q", got)
	}
	if res := st.Get(1).ScreeningResult; res == nil || res.AsrsASignificant != 6 {
		t.Fatalf("previous result must survive a restart: %+v", res)
	}

	// Finish with different answers — the stored result is overwritten.
	run := happyRun()
	run.asrs = append([]string{"Sometimes", "Sometimes", "Sometimes", "Never", "Never", "Never"}, rep("Never", 12)...)
	for _, l := range run.asrs[:6] {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	for _, l := range run.asrs[6:] {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	say(runner, sender, 1, run.form)
	for _, l := range run.wurs {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	say(runner, sender, 1, "Yes, back then")
	answerDomains(runner, sender, 1, []string{"work_study", "social"})
	answerDomains(runner, sender, 1, nil)

	res := st.Get(1).ScreeningResult
	if res == nil || res.AsrsASignificant != 3 {
		t.Fatalf("result not overwritten: %+v", res)
	}
}

func TestScr_AbandonMidScreeningKeepsOldResult(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	drive(t, runner, sender, 1, happyRun())
	if st.Get(1).ScreeningResult == nil {
		t.Fatal("precondition: result stored")
	}

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	say(runner, sender, 1, "Very Often")

	runner.HandleAbandon(sender, newMsg(1, "/abandon"))
	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing, got %q", d.State)
	}
	if d.Screening != nil {
		t.Error("Screening must be wiped by /abandon")
	}
	if d.ScreeningResult == nil {
		t.Error("previous ScreeningResult must survive /abandon")
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "screening abandoned") {
		t.Errorf("abandon confirmation missing: %q", texts)
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, "asrs_answers") {
		t.Errorf("raw answers must be gone from JSON after abandon:\n%s", raw)
	}
}

func TestScr_AbandonFodmapUnchanged(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	startDiary(runner, sender, 1)
	say(runner, sender, 1, "2")
	picked := st.Get(1).OfferedProducts[0]
	say(runner, sender, 1, picked)
	say(runner, sender, 1, lowAmountFor(t, picked))

	runner.HandleAbandon(sender, newMsg(1, "/abandon"))
	d := st.Get(1)
	if d.Products[picked].Status != "interrupted" {
		t.Errorf("expected interrupted, got %q", d.Products[picked].Status)
	}
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("expected AwaitingProductChoice, got %q", d.State)
	}
}

func TestScr_AdhdDeleteFlow(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	// Nothing stored yet.
	runner.HandleAdhdDelete(sender, newMsg(1, "/adhd_delete"))
	if !contains(sender.lastText(), "nothing to delete") {
		t.Errorf("expected delete_nothing, got %q", sender.lastText())
	}
	if got := st.Get(1).State; got != state.StateIdle {
		t.Errorf("state must not change, got %q", got)
	}

	// Cancel keeps everything. The command was issued from the landing (a
	// finished test parks the user there), so closing the dialog returns to
	// the landing — never to a dead idle.
	drive(t, runner, sender, 1, happyRun())
	runner.HandleAdhdDelete(sender, newMsg(1, "/adhd_delete"))
	if !contains(sender.lastText(), "delete everything?") {
		t.Fatalf("confirm prompt missing: %q", sender.lastText())
	}
	say(runner, sender, 1, "Keep")
	if st.Get(1).ScreeningResult == nil {
		t.Error("cancel must keep the result")
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "kept everything") {
		t.Errorf("cancel reply missing: %q", texts)
	}
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("cancel must return to the landing it interrupted, got %q", got)
	}

	// Confirm wipes both fields from the persisted JSON and returns to the
	// landing as well.
	runner.HandleAdhdDelete(sender, newMsg(1, "/adhd_delete"))
	say(runner, sender, 1, "Yes, delete")
	d := st.Get(1)
	if d.Screening != nil || d.ScreeningResult != nil {
		t.Error("confirm must wipe screening data")
	}
	texts = sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "deleted") {
		t.Errorf("delete_done missing: %q", texts)
	}
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("confirm must return to the landing it interrupted, got %q", d.State)
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, "screening") {
		t.Errorf("persisted JSON still mentions screening:\n%s", raw)
	}
}

func TestScr_AdhdDeleteMidFodmapReturns(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	drive(t, runner, sender, 1, happyRun())

	startDiary(runner, sender, 1)
	say(runner, sender, 1, "2")
	picked := st.Get(1).OfferedProducts[0]
	say(runner, sender, 1, picked)
	say(runner, sender, 1, lowAmountFor(t, picked))
	if got := st.Get(1).State; got != state.StateAwaitingStageCheckin {
		t.Fatalf("precondition: stage checkin, got %q", got)
	}

	runner.HandleAdhdDelete(sender, newMsg(1, "/adhd_delete"))
	say(runner, sender, 1, "Yes, delete")

	d := st.Get(1)
	if d.State != state.StateAwaitingStageCheckin {
		t.Errorf("expected to return to stage checkin, got %q", d.State)
	}
	if d.ReturnState != "" {
		t.Errorf("ReturnState must be cleared, got %q", d.ReturnState)
	}
	if !contains(sender.lastText(), "Take ") {
		t.Errorf("stage setup should re-fire, got %q", sender.lastText())
	}
}

// walkToDomains drives a fresh user from /adhd to the first adult-domain
// question (childhood onset answered "yes").
func walkToDomains(t *testing.T, runner *journey.Runner, sender *mockSender, id int64) {
	t.Helper()
	runner.HandleAdhd(sender, newMsg(id, "/adhd"))
	say(runner, sender, id, "Agree")
	say(runner, sender, id, "Start")
	for _, l := range rep("Very Often", 6) {
		say(runner, sender, id, l)
	}
	say(runner, sender, id, "Continue")
	for _, l := range rep("Very Often", 12) {
		say(runner, sender, id, l)
	}
	say(runner, sender, id, "Continue")
	say(runner, sender, id, "Masculine")
	for _, l := range rep("W0", 25) {
		say(runner, sender, id, l)
	}
	say(runner, sender, id, "Continue")
	say(runner, sender, id, "Yes, back then")
}

func TestScr_DomainsOneByOneShortMessages(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	walkToDomains(t, runner, sender, 1)

	// Domain 1: lead-in + position + title + one examples line + question.
	first := sender.lastText()
	for _, want := range []string{
		"adult prompt", "Sphere 1 of 5 - now", "work_study adult",
		"E.g.: work_study a-ex1, work_study a-ex2, work_study a-ex3.",
		"Noticeable difficulties?",
	} {
		if !contains(first, want) {
			t.Errorf("domain 1 message missing %q:\n%s", want, first)
		}
	}
	// One domain per message — no other domain, no repeated wall of text.
	if contains(first, "relationships_family adult") {
		t.Errorf("domain 1 message must not list other domains:\n%s", first)
	}

	// An answer advances to the NEXT short message, not a redraw of a list.
	say(runner, sender, 1, "Yes")
	second := sender.lastText()
	if !contains(second, "Sphere 2 of 5 - now") || !contains(second, "relationships_family adult") {
		t.Errorf("domain 2 message wrong:\n%s", second)
	}
	if contains(second, "work_study adult") || contains(second, "adult prompt") {
		t.Errorf("domain 2 must not repeat domain 1 or the lead-in:\n%s", second)
	}

	// Finish the adult pass → childhood pass restarts positions at 1.
	answerDomains(runner, sender, 1, nil) // domains 2..5 = "No" (+1 extra "No" swallowed below)
	// answerDomains sent 5 answers; the 5th landed on childhood domain 1.
	child := sender.lastText()
	if !contains(child, "Sphere 2 of 5 - childhood") {
		t.Fatalf("expected childhood pass underway:\n%s", child)
	}
	if got := st.Get(1).State; got != state.StateScrDomainsChild {
		t.Fatalf("expected childhood domains state, got %q", got)
	}
}

func TestScr_DomainsFewDomainsPartial(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	walkToDomains(t, runner, sender, 1)

	// Exactly one "Yes" domain → criterion D not met → partial.
	answerDomains(runner, sender, 1, []string{"leisure"})
	answerDomains(runner, sender, 1, nil)

	res := st.Get(1).ScreeningResult
	if res == nil || res.Verdict != "partial" || res.GapHint != "few_domains" {
		t.Fatalf("want partial/few_domains, got %+v", res)
	}
	if len(res.AdultDomains) != 1 || res.AdultDomains[0] != "leisure" {
		t.Errorf("adult domains: %v", res.AdultDomains)
	}
	if len(res.ChildDomains) != 0 {
		t.Errorf("child domains: %v", res.ChildDomains)
	}
}

func TestScr_DomainsPauseAndResumeMidSection(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	walkToDomains(t, runner, sender, 1)

	// Answer two domains, then /start away mid-section.
	say(runner, sender, 1, "Yes")
	say(runner, sender, 1, "No")
	runner.HandleStart(sender, newMsg(1, "/start"))
	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Fatalf("expected mode fork, got %q", d.State)
	}
	if d.Screening == nil || d.Screening.ResumeState != state.StateScrDomainsAdult {
		t.Fatalf("ResumeState not recorded: %+v", d.Screening)
	}

	// Resume via the fork → resume intro → Continue lands on domain 3,
	// with the earlier answers intact.
	say(runner, sender, 1, "Check")
	if !contains(sender.lastText(), "resumed text") {
		t.Fatalf("expected resume intro, got %q", sender.lastText())
	}
	say(runner, sender, 1, "Continue")
	got := sender.lastText()
	if !contains(got, "Sphere 3 of 5 - now") || !contains(got, "social adult") {
		t.Errorf("resume should land on domain 3:\n%s", got)
	}

	// /adhd mid-domains re-fires the same question.
	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	if got := sender.lastText(); !contains(got, "Sphere 3 of 5 - now") {
		t.Errorf("/adhd should re-ask the current domain:\n%s", got)
	}
}

// TestScr_DetourFinishLandsHomeDiaryOneTapAway pins the post-landing exit
// contract (the owner-reported bug: right after the test the bot used to
// fire the FODMAP diary question). Finishing a test started mid-diary lands
// on the home landing — never straight into the diary question — and the
// recorded diary position stays one tap away via «▶️ Вернуться к дневнику».
func TestScr_DetourFinishLandsHomeDiaryOneTapAway(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	startDiary(runner, sender, 1)
	say(runner, sender, 1, "2")
	picked := st.Get(1).OfferedProducts[0]
	say(runner, sender, 1, picked)
	say(runner, sender, 1, lowAmountFor(t, picked))
	if got := st.Get(1).State; got != state.StateAwaitingStageCheckin {
		t.Fatalf("precondition failed: %q", got)
	}

	runner.HandleStart(sender, newMsg(1, "/start"))
	if !contains(sender.lastText(), picked) {
		t.Errorf("fork prompt should mention the active product: %q", sender.lastText())
	}
	say(runner, sender, 1, "Check")
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	run := happyRun()
	for _, l := range run.asrs[:6] {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	for _, l := range run.asrs[6:] {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	say(runner, sender, 1, run.form)
	for _, l := range run.wurs {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Continue")
	say(runner, sender, 1, "Yes, back then")
	answerDomains(runner, sender, 1, []string{"work_study", "social"})
	answerDomains(runner, sender, 1, nil)

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after the test, got %q", d.State)
	}
	if d.ReturnState != state.StateAwaitingStageCheckin {
		t.Errorf("diary detour must stay recorded for the landing, got %q", d.ReturnState)
	}
	if d.CurrentProduct != picked {
		t.Errorf("trial must survive the detour, got %q", d.CurrentProduct)
	}
	texts := sentTexts(sender)
	last, prev := texts[len(texts)-1], texts[len(texts)-2]
	if !contains(last, "Mode? active "+picked) {
		t.Errorf("landing prompt (naming the trial) must close the test, got %q", last)
	}
	if contains(last, "Take ") || contains(prev, "Take ") {
		t.Errorf("the diary question must NOT fire right after the test:\n%q\n%q", prev, last)
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Resume diary: "+picked+" (low)") {
		t.Fatalf("landing must offer the diary resume button, got %v", kb)
	}

	// One tap on the contextual button returns to the saved diary phase.
	say(runner, sender, 1, "Resume diary: "+picked+" (low)")
	d = st.Get(1)
	if d.State != state.StateAwaitingStageCheckin {
		t.Errorf("expected to return to stage checkin, got %q", d.State)
	}
	if d.ReturnState != "" {
		t.Errorf("ReturnState must be consumed by the button, got %q", d.ReturnState)
	}
	if got := sender.lastText(); !contains(got, "Take ") {
		t.Errorf("stage Setup should re-fire on return, got %q", got)
	}
}

// TestScr_FinishFromDefecationLandsHome replays the exact reported bug:
// the user was on the defecation question, took the ADHD self-check, and
// right after the report the bot asked the diary question again. Now the
// finish lands on the landing and the diary question is not re-asked.
func TestScr_FinishFromDefecationLandsHome(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	startDiary(runner, sender, 1) // parked on the defecation question

	drive(t, runner, sender, 1, happyRun()) // /adhd detour + full run

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Fatalf("expected the landing after the test, got %q", d.State)
	}
	if d.ReturnState != state.StateAwaitingDefecation {
		t.Errorf("diary detour must stay recorded, got %q", d.ReturnState)
	}
	prompts := 0
	for _, txt := range sentTexts(sender) {
		if contains(txt, "Defecation?") {
			prompts++
		}
	}
	if prompts != 1 {
		t.Errorf("the diary question must not be re-asked after the test: asked %d times", prompts)
	}
	if got := sender.lastText(); !contains(got, "Mode?") {
		t.Errorf("landing prompt must close the test, got %q", got)
	}
}

func TestScr_ReminderIsolation(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	// Screening user with an active product and backdated timers.
	tr := true
	st.Set(1, state.UserData{
		State:          state.StateScrWurs,
		ChatID:         100,
		Locale:         "en",
		CurrentProduct: "Apple",
		CurrentStage:   products.StageLow,
		StageStartedAt: time.Now().Add(-2 * time.Hour),
		EnteredAt:      time.Now().Add(-2 * time.Hour),
		Screening:      &state.ScreeningProgress{WursForm: "m", OnsetChild: &tr},
	})
	// Control user who must get the defecation nudge.
	st.Set(2, state.UserData{
		State:     state.StateAwaitingDefecation,
		ChatID:    200,
		Locale:    "en",
		EnteredAt: time.Now().Add(-2 * time.Hour),
	})
	// Mood-screening user with backdated timers — must get no nudges either.
	st.Set(3, state.UserData{
		State:     state.StateMoodQuestion,
		ChatID:    300,
		Locale:    "en",
		EnteredAt: time.Now().Add(-2 * time.Hour),
		Mood:      &state.MoodProgress{Answers: []int{1, 2}},
	})

	runner.Remind()

	msgs := sender.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("expected exactly the control nudge, got %d messages", len(msgs))
	}
	if m, ok := msgs[0].(tgbotapi.MessageConfig); !ok || m.ChatID != 200 {
		t.Errorf("nudge went to the wrong chat: %+v", msgs[0])
	}
}

func TestScr_ReportIncludesScreeningLine(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	taken := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	st.Set(1, state.UserData{
		Locale: "en",
		ScreeningResult: &state.ScreeningResult{
			TakenAt:          taken,
			AsrsASignificant: 4, AsrsAThreshold: 4, AsrsAPositive: true,
			AsrsBSignificant: 5,
			WursScore:        47, WursCutoff: 46, WursPositive: true,
			OnsetChildhood: true,
			Verdict:        "consistent",
		},
	})

	// Empty Products — the screening line must still appear.
	runner.HandleReport(sender, newMsg(1, "/report"))
	got := sender.lastText()
	for _, want := range []string{
		"Progress:", "ADHD 24.08.2026", "A 4/6 (>=4) positive", "B 5/12",
		"W 47/100 (>=46) positive", "matches DSM-5 pattern",
	} {
		if !contains(got, want) {
			t.Errorf("report missing %q:\n%s", want, got)
		}
	}

	// With products the line is appended to the usual breakdown.
	d := st.Get(1)
	d.Products = map[string]state.ProductProgress{"Apple": {Status: "completed"}}
	st.Set(1, d)
	runner.HandleReport(sender, newMsg(1, "/report"))
	got = sender.lastText()
	if !contains(got, "done: Apple") || !contains(got, "ADHD 24.08.2026") {
		t.Errorf("combined report wrong:\n%s", got)
	}

	// Without a result the old report is untouched.
	st.Set(2, state.UserData{Locale: "en"})
	runner.HandleReport(sender, newMsg(2, "/report"))
	if got := sender.lastText(); got != "nothing" {
		t.Errorf("empty report changed: %q", got)
	}
}

func TestScr_InvalidInputsKeepState(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	check := func(wantState state.StateKind, wantReply string) {
		t.Helper()
		say(runner, sender, 1, "???")
		if got := st.Get(1).State; got != wantState {
			t.Errorf("state after invalid input: got %q, want %q", got, wantState)
		}
		if got := sender.lastText(); !contains(got, wantReply) {
			t.Errorf("invalid reply: got %q, want %q", got, wantReply)
		}
	}

	runner.HandleStart(sender, newMsg(1, "/start"))
	check(state.StateAwaitingModeChoice, "Tap a mode.")

	say(runner, sender, 1, "Check")
	check(state.StateScrConsent, "Tap a screening button.")

	say(runner, sender, 1, "Agree")
	check(state.StateScrIntro, "Tap a screening button.")

	say(runner, sender, 1, "Start")
	check(state.StateScrAsrsA, "Tap a scale button.")

	for _, l := range rep("Very Often", 6) {
		say(runner, sender, 1, l)
	}
	check(state.StateScrAsrsAGate, "Tap a screening button.")

	say(runner, sender, 1, "Continue")
	check(state.StateScrAsrsB, "Tap a scale button.")

	for _, l := range rep("Very Often", 12) {
		say(runner, sender, 1, l)
	}
	check(state.StateScrAsrsBGate, "Tap a screening button.")

	say(runner, sender, 1, "Continue")
	check(state.StateScrWursForm, "Tap a screening button.")

	say(runner, sender, 1, "Masculine")
	check(state.StateScrWurs, "Tap a scale button.")

	for _, l := range rep("W4", 25) {
		say(runner, sender, 1, l)
	}
	check(state.StateScrWursGate, "Tap a screening button.")

	say(runner, sender, 1, "Continue")
	check(state.StateScrOnset, "Tap a screening button.")

	say(runner, sender, 1, "Yes, back then")
	check(state.StateScrDomainsAdult, "Tap a screening button.")

	runner.HandleAdhdDelete(sender, newMsg(1, "/adhd_delete"))
	check(state.StateScrDeleteConfirm, "Tap a screening button.")
}

func TestScr_PassThroughFallsBackPerKey(t *testing.T) {
	// Production shape: ru is the default bundle and carries every scr key;
	// en has only the mode fork. An en user must still receive assembled
	// screening texts via the per-key fallback of the translator.
	dir := t.TempDir()
	i18nDir := filepath.Join(dir, "i18n")
	if err := os.Mkdir(i18nDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(i18nDir, "ru.yaml"), []byte(scrTestBundle), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(i18nDir, "en.yaml"), []byte(`phase.mode.prompt: "Mode-EN?"
button.mode.fodmap: "Diary-EN"
button.mode.screening: "Check-EN"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	trans, err := i18n.Load(i18nDir, "ru")
	if err != nil {
		t.Fatal(err)
	}

	prodSeed := filepath.Join(dir, "products.yaml")
	if err := os.WriteFile(prodSeed, []byte(`- name: Apple
  fodmap: high
  measure: pieces
  stages: { low: 0.25, medium: 0.5, high: 1.0 }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	settingsSeed := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(settingsSeed, []byte("default_locale: ru\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	content, err := screening.Load(writeScrContent(t))
	if err != nil {
		t.Fatal(err)
	}
	backend := store.NewMemoryBackend()
	cat, err := products.New(backend, prodSeed)
	if err != nil {
		t.Fatal(err)
	}
	settingsStore, err := settings.New(backend, settingsSeed)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStoreFromBackend(backend)
	sender := &mockSender{}
	runner := journey.New(stateStore, sender, cat, settingsStore, trans)
	runner.Register(journey.NewModeChoicePhase())
	runner.Register(journey.NewDefecationPhase())
	registerScreeningPhases(runner, content)

	runner.HandleStart(sender, newMsg(1, "/start")) // LanguageCode "en"
	if got := stateStore.Get(1).Locale; got != "en" {
		t.Fatalf("locale: got %q", got)
	}
	if got := sender.lastText(); got != "Mode-EN?" {
		t.Errorf("en mode prompt: got %q", got)
	}
	say(runner, sender, 1, "Check-EN")
	if got := sender.lastText(); !contains(got, "consent body") {
		t.Errorf("scr.text fallback failed for en locale: %q", got)
	}
}

func TestScr_BrokenStateGuards(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	// Question phase with no Screening → funnels back to consent.
	st.Set(1, state.UserData{State: state.StateScrWurs, ChatID: 100, Locale: "en"})
	say(runner, sender, 1, "W4")
	if got := st.Get(1).State; got != state.StateScrConsent {
		t.Errorf("nil-progress guard: got %q", got)
	}
	if !contains(sender.lastText(), "consent body") {
		t.Errorf("expected consent prompt, got %q", sender.lastText())
	}

	// Overfilled answers → straight to the gate.
	st.Set(2, state.UserData{
		State: state.StateScrAsrsA, ChatID: 200, Locale: "en",
		Screening: &state.ScreeningProgress{AsrsAnswers: rep0(30)},
	})
	say(runner, sender, 2, "whatever")
	if got := st.Get(2).State; got != state.StateScrAsrsAGate {
		t.Errorf("overfilled guard: got %q", got)
	}

	// Referral / report with no result → idle, no panic.
	st.Set(3, state.UserData{State: state.StateScrReferral, ChatID: 300, Locale: "en"})
	runner.HandleAdhd(sender, newMsg(3, "/adhd")) // re-fires Setup of the current phase
	if got := st.Get(3).State; got != state.StateIdle {
		t.Errorf("referral guard: got %q", got)
	}
}

func rep0(n int) []int {
	return make([]int, n)
}
