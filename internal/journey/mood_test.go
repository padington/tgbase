package journey_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// --- synthetic mood content -------------------------------------------------
// LoadMood demands the full canonical shape (the three instruments with
// their pinned scales/bands plus the v2 module sections), so the test set
// generates a complete synthetic bundle with short texts.

// moodScale maps the synthetic PHQ-9/GAD-7 scale labels to their scores.
var moodScale = []string{"Not at all", "Several days", "More than half", "Nearly every day"}

// moodQ10Scale maps the synthetic functional-item labels to their scores.
var moodQ10Scale = []string{"Not difficult at all", "Somewhat difficult", "Very difficult", "Extremely difficult"}

func moodSyntheticPHQ9() string {
	var b strings.Builder
	b.WriteString("instruction_official_ru: \"Over the last 2 weeks?\"\n")
	b.WriteString("scale:\n")
	for i, l := range moodScale {
		fmt.Fprintf(&b, "  - {score: %d, label: %q}\n", i, l)
	}
	b.WriteString("items:\n")
	for id := 1; id <= 9; id++ {
		crisis := ""
		if id == 9 {
			crisis = ", crisis: true"
		}
		fmt.Fprintf(&b, "  - {id: %d, text: \"M question %d?\"%s}\n", id, id, crisis)
	}
	b.WriteString("functional_item:\n  text: \"Q10 how difficult?\"\n  scale:\n")
	for i, l := range moodQ10Scale {
		fmt.Fprintf(&b, "    - {score: %d, label: %q}\n", i, l)
	}
	b.WriteString(`scoring:
  bands:
    - {min: 0, max: 4, id: minimal}
    - {min: 5, max: 9, id: mild}
    - {min: 10, max: 14, id: moderate}
    - {min: 15, max: 19, id: moderately_severe}
    - {min: 20, max: 27, id: severe}
attribution_text_ru: "PHQ-9 attribution."
`)
	return b.String()
}

// moodWho5Scale maps the synthetic WHO-5 labels to their scores, in the
// official top-down order (best first, 5 → 0).
var moodWho5Scale = []string{
	"All the time", "Most of the time", "More than half the time",
	"Less than half the time", "Some of the time", "At no time",
}

func moodSyntheticWHO5() string {
	var b strings.Builder
	b.WriteString("recall_header_official_ru: \"Over the last two weeks\"\n")
	b.WriteString("scale:\n")
	for i, l := range moodWho5Scale {
		fmt.Fprintf(&b, "  - {score: %d, label: %q}\n", 5-i, l)
	}
	b.WriteString("items:\n")
	for id := 1; id <= 5; id++ {
		fmt.Fprintf(&b, "  - {id: %d, text: \"W statement %d\"}\n", id, id)
	}
	b.WriteString(`scoring:
  multiplier: 4
  bands:
    - {min: 0, max: 28, id: very_low}
    - {min: 29, max: 50, id: low}
    - {min: 51, max: 100, id: ok}
attribution_text_ru: "WHO-5 attribution."
`)
	return b.String()
}

func moodSyntheticGAD7() string {
	var b strings.Builder
	b.WriteString("instruction_official_ru: \"Over the last 14 days?\"\n")
	b.WriteString("scale:\n")
	for i, l := range moodScale {
		fmt.Fprintf(&b, "  - {score: %d, label: %q}\n", i, l)
	}
	b.WriteString("items:\n")
	for id := 1; id <= 7; id++ {
		fmt.Fprintf(&b, "  - {id: %d, text: \"G question %d?\"}\n", id, id)
	}
	b.WriteString(`scoring:
  bands:
    - {min: 0, max: 4, id: minimal}
    - {min: 5, max: 9, id: mild}
    - {min: 10, max: 14, id: moderate}
    - {min: 15, max: 21, id: severe}
attribution_text_ru: "GAD-7 attribution."
`)
	return b.String()
}

func moodSyntheticModule() string {
	return `meta:
  title: "Mood self-check"
  disclaimer: "MOOD-DISCLAIMER not a diagnosis"
menu:
  prompt: "Mood menu prompt"
  who5_button: "Quick check"
  phq9_button: "PHQ-9 test"
  gad7_button: "Anxiety test"
  resume_phq9_button: "Resume PHQ-9 run"
  resume_who5_button: "Resume quick check"
  resume_gad7_button: "Resume anxiety run"
consent:
  title: "Mood consent"
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
  attribution_line: "MOOD-ATTR-LINE"
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
  delete_nothing: "nothing mood stored"
`
}

func writeMoodTestContent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"phq9_ru.yaml":        moodSyntheticPHQ9(),
		"who5_ru.yaml":        moodSyntheticWHO5(),
		"gad7_ru.yaml":        moodSyntheticGAD7(),
		"mood_module_ru.yaml": moodSyntheticModule(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func registerMoodPhases(runner *journey.Runner, content *screening.MoodContent) {
	runner.Register(journey.NewMoodConsentPhase(content))
	runner.Register(journey.NewMoodMenuPhase(content))
	runner.Register(journey.NewMoodQuestionPhase(content))
	runner.Register(journey.NewMoodCrisisPhase(content))
	runner.Register(journey.NewMoodQ10Phase(content))
	runner.Register(journey.NewMoodReportPhase(content))
	runner.Register(journey.NewMoodWho5Phase(content))
	runner.Register(journey.NewMoodOfferPhq9Phase(content))
	runner.Register(journey.NewMoodGad7Phase(content))
	runner.Register(journey.NewMoodOfferGad7Phase(content))
	runner.Register(journey.NewMoodDeleteConfirmPhase(content))
}

// --- drive helpers ----------------------------------------------------------

// moodAnswers builds the 9 scale labels: 8 identical answers plus the given
// label for question 9.
func moodAnswers(first8, q9 string) []string {
	return append(rep(first8, 8), q9)
}

// openMoodMenu enters the mood module via /mood, passing the one-time
// consent when it is asked, and asserts the menu is reached.
func openMoodMenu(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64) {
	t.Helper()
	runner.HandleMood(sender, newMsg(id, "/mood"))
	if st.Get(id).State == state.StateMoodConsent {
		say(runner, sender, id, "Begin")
	}
	if got := st.Get(id).State; got != state.StateMoodMenu {
		t.Fatalf("expected the mood menu, got %q", got)
	}
}

// driveMood walks a full PHQ-9 run from /mood to the home landing: consent
// (when fresh), the module menu, the nine answers, the crisis card (when
// q9 > 0), the functional question (when any answer > 0), and declines the
// closing GAD-7 offer.
func driveMood(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64, answers []string) {
	t.Helper()
	openMoodMenu(t, runner, st, sender, id)
	say(runner, sender, id, "PHQ-9 test")
	for _, l := range answers {
		say(runner, sender, id, l)
	}
	if st.Get(id).State == state.StateMoodCrisis {
		say(runner, sender, id, "Continue")
	}
	if st.Get(id).State == state.StateMoodQ10 {
		say(runner, sender, id, "Somewhat difficult")
	}
	if st.Get(id).State == state.StateMoodOfferGad7 {
		say(runner, sender, id, "Skip")
	}
}

// driveWho5 walks a full WHO-5 quick check from the menu with the given
// five answers, leaving the user wherever the result routes (the landing or
// the PHQ-9 offer).
func driveWho5(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64, answers []string) {
	t.Helper()
	openMoodMenu(t, runner, st, sender, id)
	say(runner, sender, id, "Quick check")
	for _, l := range answers {
		say(runner, sender, id, l)
	}
}

func crisisCardShown(sender *mockSender) bool {
	return contains(sender.lastText(), "crisis lead")
}

// --- scenarios --------------------------------------------------------------

func TestMood_ModeForkHasThirdButton(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleStart(sender, newMsg(1, "/start"))
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Fatalf("expected mode fork, got %q", got)
	}
	say(runner, sender, 1, "Mood")
	if got := st.Get(1).State; got != state.StateMoodConsent {
		t.Fatalf("expected mood consent, got %q", got)
	}
	text := sender.lastText()
	for _, want := range []string{"Mood consent", "mood consent body", "MOOD-DISCLAIMER"} {
		if !contains(text, want) {
			t.Errorf("consent missing %q:\n%s", want, text)
		}
	}
}

func TestMood_ConsentDeclineLeavesNoTrace(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	runner.HandleMood(sender, newMsg(1, "/mood"))
	if got := st.Get(1).State; got != state.StateMoodConsent {
		t.Fatalf("expected MoodConsent, got %q", got)
	}
	say(runner, sender, 1, "Not now")
	d := st.Get(1)
	// Every test exit lands on the home landing (never mid-diary, never a
	// dead idle): the decline text is followed by the landing prompt.
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after decline, got %q", d.State)
	}
	if d.Mood != nil || d.MoodConsentAt != nil {
		t.Error("decline must not create any mood data")
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "mood declined text") {
		t.Errorf("declined text not sent, got %q", texts)
	}
	if !contains(sender.lastText(), "Mode?") {
		t.Errorf("landing prompt must follow the decline, got %q", sender.lastText())
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"mood`) {
		t.Errorf("persisted JSON must not mention mood after decline:\n%s", raw)
	}
}

// TestMood_ConsentOnceOpensMenu pins the v2 consent contract: consent is
// asked ONCE for the whole module, opens the mini-menu with the three
// instrument buttons, and is never re-asked on later entries.
func TestMood_ConsentOnceOpensMenu(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleMood(sender, newMsg(1, "/mood"))
	say(runner, sender, 1, "Begin")
	d := st.Get(1)
	if d.State != state.StateMoodMenu {
		t.Fatalf("consent must open the menu, got %q", d.State)
	}
	if d.MoodConsentAt == nil {
		t.Fatal("agreeing must record the module-wide consent")
	}
	menu := sender.lastText()
	if !contains(menu, "Mood menu prompt") {
		t.Errorf("menu prompt missing: %q", menu)
	}
	kb := lastKeyboard(sender)
	for _, want := range []string{"Quick check", "PHQ-9 test", "Anxiety test", "Home"} {
		if !keyboardHas(kb, want) {
			t.Errorf("menu must offer %q, got %v", want, kb)
		}
	}

	// A quick check passes without any further consent…
	driveWho5(t, runner, st, sender, 1, rep("All the time", 5))

	// …and the next /mood goes straight to the menu.
	runner.HandleMood(sender, newMsg(1, "/mood"))
	if got := st.Get(1).State; got != state.StateMoodMenu {
		t.Fatalf("second entry must skip consent, got %q", got)
	}
	consentShown := 0
	for _, txt := range sentTexts(sender) {
		if contains(txt, "mood consent body") {
			consentShown++
		}
	}
	if consentShown != 1 {
		t.Errorf("consent must be asked exactly once, shown %d times", consentShown)
	}
}

func TestMood_QuestionsOneByOne(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	q1 := sender.lastText()
	for _, want := range []string{"PHQ-9 test", "Over the last 2 weeks?", "Question 1 of 9", "M question 1?"} {
		if !contains(q1, want) {
			t.Errorf("first question missing %q:\n%s", want, q1)
		}
	}
	say(runner, sender, 1, "Several days")
	q2 := sender.lastText()
	if !contains(q2, "Question 2 of 9") || !contains(q2, "M question 2?") {
		t.Errorf("second question wrong:\n%s", q2)
	}
	if contains(q2, "M question 1?") || contains(q2, "Over the last 2 weeks?") {
		t.Errorf("second question must not repeat q1 or the header:\n%s", q2)
	}
}

func TestMood_HappyPathWithFunctionalItem(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	// 8×"Several days" + q9 "Not at all" → score 8, band mild, no crisis —
	// but positives exist, so the functional (10th) question follows.
	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	for _, l := range moodAnswers("Several days", "Not at all") {
		say(runner, sender, 1, l)
	}

	if got := st.Get(1).State; got != state.StateMoodQ10 {
		t.Fatalf("expected the functional question after nine answers, got %q", got)
	}
	q10 := sender.lastText()
	if !contains(q10, "Q10 how difficult?") {
		t.Errorf("functional question missing:\n%s", q10)
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Somewhat difficult") {
		t.Errorf("functional options missing, got %v", kb)
	}

	say(runner, sender, 1, "Somewhat difficult")
	d := st.Get(1)
	if d.State != state.StateMoodOfferGad7 {
		t.Errorf("expected the GAD-7 offer after the report, got %q", d.State)
	}
	if d.Mood != nil {
		t.Error("Mood must be wiped on completion")
	}
	res := d.MoodResult
	if res == nil {
		t.Fatal("MoodResult must be stored")
	}
	if res.Score != 8 || res.Severity != "mild" || res.Q9Positive {
		t.Errorf("result: %+v", res)
	}
	if !res.Q10Answered || res.Q10Answer != 1 {
		t.Errorf("functional answer must be stored: %+v", res)
	}

	raw := rawUsersJSON(t, backend)
	if strings.Contains(raw, `"answers"`) || strings.Contains(raw, `"mood":`) {
		t.Errorf("raw answers must not survive completion:\n%s", raw)
	}
	if !strings.Contains(raw, "mood_result") {
		t.Error("mood_result missing from persisted JSON")
	}

	texts := sentTexts(sender)
	if len(texts) < 3 {
		t.Fatalf("expected at least result + report + offer, got %d messages", len(texts))
	}
	// The final chain is result → combined doctor report → GAD-7 offer.
	result, report, offer := texts[len(texts)-3], texts[len(texts)-2], texts[len(texts)-1]
	for _, want := range []string{
		"Your mood result", "PHQ-9: 8 of 27.", "band mild",
		"retest in 2-4 weeks", "MOOD-DISCLAIMER", "MOOD-ATTR-LINE",
	} {
		if !contains(result, want) {
			t.Errorf("result missing %q:\n%s", want, result)
		}
	}
	for _, notWant := range []string{"crisis lead", "CONTACT-LINE-1", "support contacts:", "Last time ("} {
		if contains(result, notWant) {
			t.Errorf("first-run no-crisis result must not contain %q:\n%s", notWant, result)
		}
	}
	if contains(result, "{score}") || contains(result, "{ago}") {
		t.Errorf("unrendered placeholder in result:\n%s", result)
	}
	for _, want := range []string{
		"mood report lead-in", "MOOD SUMMARY REPORT",
		"PHQ9 " + res.TakenAt.Format("02.01.2006") + ": 8/27 - band mild",
		"q9: not marked", "q10: Somewhat difficult", "REPORT-FOOTER",
	} {
		if !contains(report, want) {
			t.Errorf("doctor report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"GAD7 ", "WHO5 "} {
		if contains(report, notWant) {
			t.Errorf("report must list only completed instruments (%q):\n%s", notWant, report)
		}
	}
	if !contains(offer, "offer gad7 body") {
		t.Errorf("GAD-7 offer must close the PHQ-9 chain:\n%s", offer)
	}

	// Declining the offer lands home.
	say(runner, sender, 1, "Skip")
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after declining the offer, got %q", got)
	}
}

// TestMood_Q10OnlyWhenAnyPositive pins the official gate: an all-zero run
// skips the functional question entirely — no q10 in the chat, no q10 line
// in the report.
func TestMood_Q10OnlyWhenAnyPositive(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	for _, l := range moodAnswers("Not at all", "Not at all") {
		say(runner, sender, 1, l)
	}

	d := st.Get(1)
	if d.State != state.StateMoodOfferGad7 {
		t.Fatalf("all-zero run must finalize straight to the offer, got %q", d.State)
	}
	res := d.MoodResult
	if res == nil || res.Score != 0 || res.Q10Answered {
		t.Fatalf("all-zero result must have no functional answer: %+v", res)
	}
	for _, txt := range sentTexts(sender) {
		if contains(txt, "Q10 how difficult?") {
			t.Errorf("functional question must not be asked on an all-zero run:\n%s", txt)
		}
		if contains(txt, "q10:") {
			t.Errorf("report must not carry a q10 line on an all-zero run:\n%s", txt)
		}
	}
}

func TestMood_CrisisCardImmediatelyAfterQ9(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	for _, l := range rep("Not at all", 8) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Several days") // q9 = 1

	// The card comes right after the answer — before anything else.
	if got := st.Get(1).State; got != state.StateMoodCrisis {
		t.Fatalf("expected crisis state right after q9 answer, got %q", got)
	}
	card := sender.lastText()
	for _, want := range []string{"crisis lead", "CONTACT-LINE-1", "CONTACT-LINE-2"} {
		if !contains(card, want) {
			t.Errorf("crisis card missing %q:\n%s", want, card)
		}
	}
	if contains(card, "talk to someone today") {
		t.Errorf("urgent line must not appear for answer 1:\n%s", card)
	}
	if contains(card, "Your mood result") {
		t.Errorf("result must not be sent before Continue:\n%s", card)
	}

	// The test is not blocked: Continue proceeds to the functional question
	// (q9 > 0 opens its gate), then the result and the offer.
	say(runner, sender, 1, "Continue")
	if got := st.Get(1).State; got != state.StateMoodQ10 {
		t.Fatalf("continue must proceed to the functional question, got %q", got)
	}
	say(runner, sender, 1, "Not difficult at all")
	d := st.Get(1)
	if d.State != state.StateMoodOfferGad7 {
		t.Errorf("expected the GAD-7 offer after the report, got %q", d.State)
	}
	res := d.MoodResult
	if res == nil || res.Score != 1 || res.Severity != "minimal" || !res.Q9Positive {
		t.Fatalf("result: %+v", res)
	}
	if !res.Q10Answered || res.Q10Answer != 0 {
		t.Fatalf("functional answer must be stored even when zero: %+v", res)
	}

	// The contacts are repeated in the final result despite the minimal score.
	texts := sentTexts(sender)
	result := texts[len(texts)-3]
	if !contains(result, "support contacts:") || !contains(result, "CONTACT-LINE-1") {
		t.Errorf("result must repeat the contacts when q9 > 0:\n%s", result)
	}
	if !contains(result, "band minimal") {
		t.Errorf("result should still carry the score band:\n%s", result)
	}
	// And the doctor report carries both PHQ-9 facts.
	report := texts[len(texts)-2]
	if !contains(report, "q9: marked") {
		t.Errorf("doctor report must mark q9:\n%s", report)
	}
	if !contains(report, "q10: Not difficult at all") {
		t.Errorf("doctor report must carry the functional answer:\n%s", report)
	}
}

func TestMood_CrisisUrgentLineForStrongAnswers(t *testing.T) {
	for _, tc := range []struct {
		q9         string
		wantUrgent bool
	}{
		{"Several days", false},
		{"More than half", true},
		{"Nearly every day", true},
	} {
		t.Run(tc.q9, func(t *testing.T) {
			runner, st, sender, _ := setupScr(t)
			openMoodMenu(t, runner, st, sender, 1)
			say(runner, sender, 1, "PHQ-9 test")
			for _, l := range rep("Not at all", 8) {
				say(runner, sender, 1, l)
			}
			say(runner, sender, 1, tc.q9)

			if got := st.Get(1).State; got != state.StateMoodCrisis {
				t.Fatalf("expected crisis state, got %q", got)
			}
			card := sender.lastText()
			if got := contains(card, "talk to someone today"); got != tc.wantUrgent {
				t.Errorf("urgent line presence = %v, want %v:\n%s", got, tc.wantUrgent, card)
			}
			if !contains(card, "CONTACT-LINE-1") {
				t.Errorf("card must always carry the contacts:\n%s", card)
			}
		})
	}
}

func TestMood_NoCrisisCardWhenQ9Zero(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	for _, l := range rep("Nearly every day", 8) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Not at all") // q9 = 0 → straight to q10 (positives exist)

	if got := st.Get(1).State; got != state.StateMoodQ10 {
		t.Fatalf("expected the functional question without a crisis card, got %q", got)
	}
	say(runner, sender, 1, "Very difficult")

	d := st.Get(1)
	res := d.MoodResult
	if res == nil || res.Score != 24 || res.Severity != "severe" || res.Q9Positive {
		t.Fatalf("result: %+v", res)
	}
	texts := sentTexts(sender)
	result := texts[len(texts)-3]
	if contains(result, "crisis lead") || contains(result, "support contacts:") {
		t.Errorf("no crisis block when q9 == 0, even for a severe score:\n%s", result)
	}
	if !contains(result, "band severe") {
		t.Errorf("severe band line missing:\n%s", result)
	}
	for _, txt := range texts {
		if contains(txt, "crisis lead") {
			t.Errorf("crisis card must not appear when q9 == 0:\n%s", txt)
		}
	}
}

func TestMood_RetestShowsDelta(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	driveMood(t, runner, st, sender, 1, moodAnswers("Several days", "Not at all")) // 8
	first := st.Get(1).MoodResult
	if first == nil || first.Score != 8 {
		t.Fatalf("first result: %+v", first)
	}

	resetSender(sender)
	driveMood(t, runner, st, sender, 1, moodAnswers("More than half", "Not at all")) // 16
	res := st.Get(1).MoodResult
	if res == nil || res.Score != 16 || res.Severity != "moderately_severe" {
		t.Fatalf("second result must overwrite the first: %+v", res)
	}
	found := false
	for _, txt := range sentTexts(sender) {
		if contains(txt, "Last time (today) it was 8, now 16.") {
			found = true
		}
	}
	if !found {
		t.Errorf("delta line missing from the retest result:\n%q", sentTexts(sender))
	}
}

func TestMood_StartMidTestPausesAndResumes(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	say(runner, sender, 1, "Several days")
	say(runner, sender, 1, "Several days")

	runner.HandleStart(sender, newMsg(1, "/start"))
	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Fatalf("expected mode fork, got %q", d.State)
	}
	if d.Mood == nil || d.Mood.ResumeState != state.StateMoodQuestion {
		t.Fatalf("ResumeState not recorded: %+v", d.Mood)
	}

	// Resume via the mood button: the menu offers the run's resume row, and
	// consent is NOT re-asked.
	say(runner, sender, 1, "Mood")
	if got := st.Get(1).State; got != state.StateMoodMenu {
		t.Fatalf("expected the menu (consent already given), got %q", got)
	}
	if contains(sender.lastText(), "mood consent body") {
		t.Error("consent must not be asked again on resume")
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Resume PHQ-9 run") {
		t.Fatalf("menu must offer the resume row, got %v", kb)
	}
	say(runner, sender, 1, "Resume PHQ-9 run")
	if got := sender.lastText(); !contains(got, "Question 3 of 9") {
		t.Errorf("resume should land on question 3, got %q", got)
	}

	// /mood mid-test re-fires the current question.
	runner.HandleMood(sender, newMsg(1, "/mood"))
	if got := sender.lastText(); !contains(got, "Question 3 of 9") {
		t.Errorf("/mood should re-ask the current question, got %q", got)
	}
}

func TestMood_MenuInstrumentButtonRestartsRun(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	say(runner, sender, 1, "Nearly every day")
	runner.HandleStart(sender, newMsg(1, "/start"))
	say(runner, sender, 1, "Mood") // → menu with the resume row above the fresh-start button

	// The plain instrument button is the explicit start-over.
	say(runner, sender, 1, "PHQ-9 test")
	if got := sender.lastText(); !contains(got, "Question 1 of 9") {
		t.Fatalf("restart should begin from question 1, got %q", got)
	}
	if got := len(st.Get(1).Mood.Answers); got != 0 {
		t.Errorf("restart must wipe answers, got %d", got)
	}

	// Escaping home keeps the run resumable — the landing offers the mood
	// resume button right away.
	runner.HandleStart(sender, newMsg(1, "/start"))
	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing, got %q", d.State)
	}
	if d.Mood == nil {
		t.Error("escape must keep the unfinished run")
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Resume mood") {
		t.Errorf("landing must offer the mood resume button, got %v", kb)
	}
}

func TestMood_PauseOnCrisisResumesOnCard(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	for _, l := range rep("Not at all", 8) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "More than half") // q9 = 2 → crisis card
	if got := st.Get(1).State; got != state.StateMoodCrisis {
		t.Fatalf("precondition: crisis, got %q", got)
	}

	runner.HandleStart(sender, newMsg(1, "/start"))
	d := st.Get(1)
	if d.Mood == nil || d.Mood.ResumeState != state.StateMoodCrisis {
		t.Fatalf("crisis ResumeState not recorded: %+v", d.Mood)
	}

	// The landing's resume button lands straight back on the card.
	say(runner, sender, 1, "Resume mood")
	card := sender.lastText()
	if !contains(card, "crisis lead") || !contains(card, "talk to someone today") {
		t.Fatalf("resume must land back on the crisis card:\n%s", card)
	}
	if got := st.Get(1).State; got != state.StateMoodCrisis {
		t.Fatalf("expected crisis state again, got %q", got)
	}
	say(runner, sender, 1, "Continue") // card → functional question
	say(runner, sender, 1, "Extremely difficult")
	if res := st.Get(1).MoodResult; res == nil || !res.Q9Positive || !res.Q10Answered || res.Q10Answer != 3 {
		t.Fatalf("result after crisis continue: %+v", res)
	}
}

func TestMood_AbandonKeepsOldResult(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	driveMood(t, runner, st, sender, 1, moodAnswers("Several days", "Not at all"))
	if st.Get(1).MoodResult == nil {
		t.Fatal("precondition: result stored")
	}

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	say(runner, sender, 1, "Nearly every day")

	runner.HandleAbandon(sender, newMsg(1, "/abandon"))
	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing, got %q", d.State)
	}
	if d.Mood != nil {
		t.Error("Mood must be wiped by /abandon")
	}
	if d.MoodResult == nil {
		t.Error("previous MoodResult must survive /abandon")
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "mood abandoned") {
		t.Errorf("abandon confirmation missing: %q", texts)
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"answers"`) {
		t.Errorf("raw answers must be gone from JSON after abandon:\n%s", raw)
	}
}

// TestMood_AbandonWipesOnlyTheActiveInstrument pins the per-instrument
// abandon: cancelling a WHO-5 run must not touch a paused PHQ-9 run.
func TestMood_AbandonWipesOnlyTheActiveInstrument(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	say(runner, sender, 1, "Several days")
	runner.HandleStart(sender, newMsg(1, "/start")) // pause the PHQ-9 run

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Quick check")
	say(runner, sender, 1, "All the time")

	runner.HandleAbandon(sender, newMsg(1, "/abandon"))
	d := st.Get(1)
	if d.Who5 != nil {
		t.Error("the active WHO-5 run must be wiped")
	}
	if d.Mood == nil || len(d.Mood.Answers) != 1 {
		t.Errorf("the paused PHQ-9 run must survive: %+v", d.Mood)
	}
}

func TestMood_DeleteFlow(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	// Nothing stored yet.
	runner.HandleMoodDelete(sender, newMsg(1, "/mood_delete"))
	if !contains(sender.lastText(), "nothing mood stored") {
		t.Errorf("expected delete_nothing, got %q", sender.lastText())
	}
	if got := st.Get(1).State; got != state.StateIdle {
		t.Errorf("state must not change, got %q", got)
	}

	// Cancel keeps everything.
	driveMood(t, runner, st, sender, 1, moodAnswers("Several days", "Not at all"))
	runner.HandleMoodDelete(sender, newMsg(1, "/mood_delete"))
	if !contains(sender.lastText(), "delete mood data?") {
		t.Fatalf("confirm prompt missing: %q", sender.lastText())
	}
	say(runner, sender, 1, "Keep")
	if st.Get(1).MoodResult == nil {
		t.Error("cancel must keep the result")
	}

	// Confirm wipes every mood field from the persisted JSON — including
	// the module consent — and returns to the landing the command
	// interrupted.
	runner.HandleMoodDelete(sender, newMsg(1, "/mood_delete"))
	say(runner, sender, 1, "Yes, delete")
	d := st.Get(1)
	if d.Mood != nil || d.MoodResult != nil || d.Who5 != nil || d.Who5Result != nil ||
		d.Gad7 != nil || d.Gad7Result != nil || d.MoodConsentAt != nil {
		t.Error("confirm must wipe all mood data")
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "mood deleted") {
		t.Errorf("delete_done missing: %q", texts)
	}
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("confirm must return to the landing it interrupted, got %q", d.State)
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"mood`) ||
		strings.Contains(raw, `"who5`) || strings.Contains(raw, `"gad7`) {
		t.Errorf("persisted JSON still mentions mood data:\n%s", raw)
	}

	// With the consent wiped, the next entry asks it again.
	runner.HandleMood(sender, newMsg(1, "/mood"))
	if got := st.Get(1).State; got != state.StateMoodConsent {
		t.Errorf("consent must be re-asked after a full delete, got %q", got)
	}
}

// TestMood_DetourFinishLandsHomeDiaryOneTapAway is the PHQ-9 twin of the
// ADHD detour test: finishing a test started mid-diary lands on the home
// landing — never straight into the diary question — and the recorded diary
// position stays one tap away via «▶️ Вернуться к дневнику».
func TestMood_DetourFinishLandsHomeDiaryOneTapAway(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	startDiary(runner, sender, 1)
	say(runner, sender, 1, "2")
	picked := st.Get(1).OfferedProducts[0]
	say(runner, sender, 1, picked)
	say(runner, sender, 1, lowAmountFor(t, picked))
	if got := st.Get(1).State; got != state.StateAwaitingStageCheckin {
		t.Fatalf("precondition: stage checkin, got %q", got)
	}

	driveMood(t, runner, st, sender, 1, moodAnswers("Several days", "Not at all"))

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

// TestMood_FinishFromDefecationLandsHome replays the reported bug for the
// mood test: detour from the defecation question, finish, and the diary
// question must not be re-asked — the user lands home instead.
func TestMood_FinishFromDefecationLandsHome(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	startDiary(runner, sender, 1) // parked on the defecation question

	driveMood(t, runner, st, sender, 1, moodAnswers("Several days", "Not at all"))

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

// --- WHO-5 quick check ------------------------------------------------------

func TestWho5_HighScoreLandsHomeWithoutOffer(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Quick check")
	s1 := sender.lastText()
	for _, want := range []string{"Quick well-being check", "Over the last two weeks", "Statement 1 of 5", "W statement 1"} {
		if !contains(s1, want) {
			t.Errorf("first statement missing %q:\n%s", want, s1)
		}
	}
	// The scale keeps the form order — best option on top.
	if kb := lastKeyboard(sender); len(kb) == 0 || kb[0][0] != "All the time" {
		t.Errorf("WHO-5 keyboard must start with the best option, got %v", kb)
	}

	for _, l := range rep("All the time", 5) {
		say(runner, sender, 1, l)
	}
	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("a normal score must land home without an offer, got %q", d.State)
	}
	if d.Who5 != nil {
		t.Error("Who5 progress must be wiped on completion")
	}
	res := d.Who5Result
	if res == nil || res.Score != 100 || res.Band != "ok" {
		t.Fatalf("result: %+v", res)
	}
	texts := sentTexts(sender)
	result := texts[len(texts)-2] // result → landing prompt
	for _, want := range []string{"WHO-5: 100 of 100.", "wb ok", "MOOD-DISCLAIMER", "WHO5-ATTR-LINE"} {
		if !contains(result, want) {
			t.Errorf("result missing %q:\n%s", want, result)
		}
	}
	for _, txt := range texts {
		if contains(txt, "offer phq9") {
			t.Errorf("no offer for a normal score:\n%s", txt)
		}
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"answers"`) {
		t.Errorf("raw answers must not survive completion:\n%s", raw)
	}
}

func TestWho5_ReducedScoreOffersPhq9(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	// 5 × "Less than half the time" (2) → raw 10 → 40 → low.
	driveWho5(t, runner, st, sender, 1, rep("Less than half the time", 5))

	d := st.Get(1)
	if d.State != state.StateMoodOfferPhq9 {
		t.Fatalf("a reduced score must offer the PHQ-9, got %q", d.State)
	}
	res := d.Who5Result
	if res == nil || res.Score != 40 || res.Band != "low" {
		t.Fatalf("result: %+v", res)
	}
	texts := sentTexts(sender)
	result, offer := texts[len(texts)-2], texts[len(texts)-1]
	if !contains(result, "WHO-5: 40 of 100.") || !contains(result, "wb low") {
		t.Errorf("result wrong:\n%s", result)
	}
	if !contains(offer, "offer phq9 low") || contains(offer, "very low") {
		t.Errorf("expected the regular offer wording:\n%s", offer)
	}

	// One tap starts the PHQ-9 — consent is NOT re-asked.
	say(runner, sender, 1, "Take the PHQ-9")
	if got := st.Get(1).State; got != state.StateMoodQuestion {
		t.Fatalf("offer must start the PHQ-9, got %q", got)
	}
	if got := sender.lastText(); !contains(got, "Question 1 of 9") {
		t.Errorf("PHQ-9 must start from question 1, got %q", got)
	}
	if contains(sender.lastText(), "mood consent body") {
		t.Error("consent must not be re-asked on the offer path")
	}
}

func TestWho5_MarkedReductionInsistentOffer(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	// 5 × "Some of the time" (1) → raw 5 → 20 → very_low.
	driveWho5(t, runner, st, sender, 1, rep("Some of the time", 5))

	d := st.Get(1)
	if d.State != state.StateMoodOfferPhq9 {
		t.Fatalf("expected the PHQ-9 offer, got %q", d.State)
	}
	if res := d.Who5Result; res == nil || res.Score != 20 || res.Band != "very_low" {
		t.Fatalf("result: %+v", res)
	}
	if offer := sender.lastText(); !contains(offer, "offer phq9 very low") {
		t.Errorf("expected the insistent offer wording:\n%s", offer)
	}

	// Declining lands home; the result stays stored.
	say(runner, sender, 1, "Skip")
	d = st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after declining, got %q", d.State)
	}
	if d.Who5Result == nil {
		t.Error("declining the offer must keep the WHO-5 result")
	}
}

// TestWho5_OfferResumesPausedPhq9Run pins the no-data-loss rule: the offer's
// start button continues a paused PHQ-9 run instead of wiping it.
func TestWho5_OfferResumesPausedPhq9Run(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "PHQ-9 test")
	say(runner, sender, 1, "Several days")
	runner.HandleStart(sender, newMsg(1, "/start")) // pause with 1 answer

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Quick check")
	for _, l := range rep("Less than half the time", 5) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Take the PHQ-9")

	d := st.Get(1)
	if d.State != state.StateMoodQuestion {
		t.Fatalf("expected the PHQ-9 question, got %q", d.State)
	}
	if d.Mood == nil || len(d.Mood.Answers) != 1 {
		t.Fatalf("the paused run must be resumed, not wiped: %+v", d.Mood)
	}
	if got := sender.lastText(); !contains(got, "Question 2 of 9") {
		t.Errorf("resume must land on question 2, got %q", got)
	}
}

// --- GAD-7 anxiety test -----------------------------------------------------

func TestGad7_FlowAndCombinedReport(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Anxiety test")
	q1 := sender.lastText()
	for _, want := range []string{"Anxiety check", "Over the last 14 days?", "Question 1 of 7", "G question 1?"} {
		if !contains(q1, want) {
			t.Errorf("first question missing %q:\n%s", want, q1)
		}
	}

	// 7 × "More than half" (2) → 14 → moderate.
	for _, l := range rep("More than half", 7) {
		say(runner, sender, 1, l)
	}

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("GAD-7 completion must land home (no further offer), got %q", d.State)
	}
	if d.Gad7 != nil {
		t.Error("Gad7 progress must be wiped on completion")
	}
	res := d.Gad7Result
	if res == nil || res.Score != 14 || res.Severity != "moderate" {
		t.Fatalf("result: %+v", res)
	}

	texts := sentTexts(sender)
	// The final chain is result → combined doctor report → landing prompt.
	result, report := texts[len(texts)-3], texts[len(texts)-2]
	for _, want := range []string{"GAD-7: 14 of 21.", "g moderate - discuss", "MOOD-DISCLAIMER", "GAD7-ATTR-LINE"} {
		if !contains(result, want) {
			t.Errorf("result missing %q:\n%s", want, result)
		}
	}
	for _, want := range []string{
		"MOOD SUMMARY REPORT",
		"GAD7 " + res.TakenAt.Format("02.01.2006") + ": 14/21 - g moderate - discuss",
		"REPORT-FOOTER",
	} {
		if !contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}
	if contains(report, "PHQ9 ") || contains(report, "WHO5 ") {
		t.Errorf("report must list only completed instruments:\n%s", report)
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"answers"`) {
		t.Errorf("raw answers must not survive completion:\n%s", raw)
	}
}

func TestGad7_SevereBandWording(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openMoodMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Anxiety test")
	for _, l := range rep("Nearly every day", 7) {
		say(runner, sender, 1, l)
	}
	res := st.Get(1).Gad7Result
	if res == nil || res.Score != 21 || res.Severity != "severe" {
		t.Fatalf("result: %+v", res)
	}
	texts := sentTexts(sender)
	if result := texts[len(texts)-3]; !contains(result, "g severe - see a specialist") {
		t.Errorf("severe wording missing:\n%s", result)
	}
}

// --- the offer chain --------------------------------------------------------

// TestMood_OfferChainWho5ToPhq9ToGad7 walks the full link chain: a reduced
// quick check offers the PHQ-9; the PHQ-9 report offers the GAD-7; the
// final combined report carries all three instruments with dates.
func TestMood_OfferChainWho5ToPhq9ToGad7(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	driveWho5(t, runner, st, sender, 1, rep("Less than half the time", 5))
	say(runner, sender, 1, "Take the PHQ-9")
	for _, l := range moodAnswers("Several days", "Not at all") {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Somewhat difficult") // the functional question
	if got := st.Get(1).State; got != state.StateMoodOfferGad7 {
		t.Fatalf("expected the GAD-7 offer, got %q", got)
	}
	say(runner, sender, 1, "Take the GAD-7")
	for _, l := range rep("Several days", 7) {
		say(runner, sender, 1, l)
	}

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("the chain must close on the landing, got %q", d.State)
	}
	if d.Who5Result == nil || d.MoodResult == nil || d.Gad7Result == nil {
		t.Fatal("all three results must be stored")
	}

	texts := sentTexts(sender)
	report := texts[len(texts)-2] // the combined report before the landing
	date := d.Gad7Result.TakenAt.Format("02.01.2006")
	for _, want := range []string{
		"MOOD SUMMARY REPORT",
		"PHQ9 " + date + ": 8/27 - band mild",
		"q9: not marked",
		"q10: Somewhat difficult",
		"GAD7 " + date + ": 7/21 - g mild",
		"WHO5 " + date + ": 40/100 - wb low",
		"REPORT-FOOTER",
	} {
		if !contains(report, want) {
			t.Errorf("combined report missing %q:\n%s", want, report)
		}
	}
}

// --- /report ----------------------------------------------------------------

func TestMood_ReportIncludesAllInstrumentLines(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	taken := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	st.Set(1, state.UserData{
		Locale:     "en",
		MoodResult: &state.MoodResult{TakenAt: taken, Score: 11, Severity: "moderate"},
	})
	runner.HandleReport(sender, newMsg(1, "/report"))
	got := sender.lastText()
	if !contains(got, "Progress:") || !contains(got, "Mood 25.08.2026: 11 of 27") {
		t.Errorf("report missing the mood line:\n%s", got)
	}

	// Alongside the ADHD line, products, and the other two instruments.
	d := st.Get(1)
	d.Products = map[string]state.ProductProgress{"Apple": {Status: "completed"}}
	d.ScreeningResult = &state.ScreeningResult{
		TakenAt: taken, AsrsASignificant: 4, AsrsAThreshold: 4, AsrsAPositive: true,
		WursScore: 47, WursCutoff: 46, WursPositive: true, Verdict: "consistent",
	}
	d.Gad7Result = &state.Gad7Result{TakenAt: taken, Score: 9, Severity: "mild"}
	d.Who5Result = &state.Who5Result{TakenAt: taken, Score: 48, Band: "low"}
	st.Set(1, d)
	runner.HandleReport(sender, newMsg(1, "/report"))
	got = sender.lastText()
	for _, want := range []string{
		"done: Apple", "ADHD 25.08.2026", "Mood 25.08.2026: 11 of 27",
		"Anxiety 25.08.2026: 9 of 21", "WHO5 25.08.2026: 48 of 100",
	} {
		if !contains(got, want) {
			t.Errorf("combined report missing %q:\n%s", want, got)
		}
	}
}

// --- guards -----------------------------------------------------------------

func TestMood_InvalidInputsKeepState(t *testing.T) {
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

	runner.HandleMood(sender, newMsg(1, "/mood"))
	check(state.StateMoodConsent, "Tap a screening button.")

	say(runner, sender, 1, "Begin")
	check(state.StateMoodMenu, "Tap a screening button.")

	say(runner, sender, 1, "PHQ-9 test")
	check(state.StateMoodQuestion, "Tap a scale button.")

	for _, l := range rep("Not at all", 8) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "Nearly every day")
	check(state.StateMoodCrisis, "Tap a screening button.")
	say(runner, sender, 1, "Continue")
	check(state.StateMoodQ10, "Tap a scale button.")
	say(runner, sender, 1, "Not difficult at all")
	check(state.StateMoodOfferGad7, "Tap a screening button.")
	say(runner, sender, 1, "Skip")

	runner.HandleMoodDelete(sender, newMsg(1, "/mood_delete"))
	check(state.StateMoodDeleteConfirm, "Tap a screening button.")
}

func TestMood_BrokenStateGuards(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	// Question phase with no Mood and no consent → funnels back to consent.
	st.Set(1, state.UserData{State: state.StateMoodQuestion, ChatID: 100, Locale: "en"})
	say(runner, sender, 1, "Several days")
	if got := st.Get(1).State; got != state.StateMoodConsent {
		t.Errorf("nil-progress guard: got %q", got)
	}
	if !contains(sender.lastText(), "mood consent body") {
		t.Errorf("expected consent prompt, got %q", sender.lastText())
	}

	// Crisis state without a crisis answer → back to the question series.
	st.Set(2, state.UserData{
		State: state.StateMoodCrisis, ChatID: 200, Locale: "en",
		Mood: &state.MoodProgress{Answers: []int{1, 1}},
	})
	runner.HandleMood(sender, newMsg(2, "/mood")) // re-fires Setup of the current phase
	if got := st.Get(2).State; got != state.StateMoodQuestion {
		t.Errorf("crisis guard: got %q", got)
	}

	// Functional question with an incomplete run → back to the questions.
	st.Set(3, state.UserData{
		State: state.StateMoodQ10, ChatID: 300, Locale: "en",
		Mood: &state.MoodProgress{Answers: []int{1, 1, 1}},
	})
	runner.HandleMood(sender, newMsg(3, "/mood"))
	if got := st.Get(3).State; got != state.StateMoodQuestion {
		t.Errorf("q10 guard: got %q", got)
	}

	// Overfilled answers → finalize without re-asking; the PHQ-9 completion
	// routes to the GAD-7 offer as usual.
	st.Set(4, state.UserData{
		State: state.StateMoodQuestion, ChatID: 400, Locale: "en",
		Mood: &state.MoodProgress{Answers: rep0(12)},
	})
	runner.HandleMood(sender, newMsg(4, "/mood"))
	d := st.Get(4)
	if d.State != state.StateMoodOfferGad7 || d.MoodResult == nil {
		t.Errorf("overfilled guard: state %q, result %+v", d.State, d.MoodResult)
	}

	// Report with no result → idle, no panic.
	st.Set(5, state.UserData{State: state.StateMoodReport, ChatID: 500, Locale: "en"})
	runner.HandleMood(sender, newMsg(5, "/mood"))
	if got := st.Get(5).State; got != state.StateIdle {
		t.Errorf("report guard: got %q", got)
	}

	// WHO-5 / GAD-7 question states without a run → back to the menu (the
	// stored result implies consent, so the menu renders).
	taken := time.Now()
	st.Set(6, state.UserData{
		State: state.StateMoodWho5Question, ChatID: 600, Locale: "en",
		Who5Result: &state.Who5Result{TakenAt: taken, Score: 80, Band: "ok"},
	})
	runner.HandleMood(sender, newMsg(6, "/mood"))
	if got := st.Get(6).State; got != state.StateMoodMenu {
		t.Errorf("who5 guard: got %q", got)
	}
	st.Set(7, state.UserData{
		State: state.StateMoodGad7Question, ChatID: 700, Locale: "en",
		Gad7Result: &state.Gad7Result{TakenAt: taken, Score: 3, Severity: "minimal"},
	})
	runner.HandleMood(sender, newMsg(7, "/mood"))
	if got := st.Get(7).State; got != state.StateMoodMenu {
		t.Errorf("gad7 guard: got %q", got)
	}
}

func TestMood_AdhdAndMoodDataIndependent(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	drive(t, runner, sender, 1, happyRun())
	driveMood(t, runner, st, sender, 1, moodAnswers("Several days", "Not at all"))

	d := st.Get(1)
	if d.ScreeningResult == nil || d.MoodResult == nil {
		t.Fatal("both results must coexist")
	}

	// Deleting mood data leaves the ADHD result intact, and vice versa.
	runner.HandleMoodDelete(sender, newMsg(1, "/mood_delete"))
	say(runner, sender, 1, "Yes, delete")
	d = st.Get(1)
	if d.MoodResult != nil {
		t.Error("mood result must be wiped")
	}
	if d.ScreeningResult == nil {
		t.Error("ADHD result must survive /mood_delete")
	}
}
