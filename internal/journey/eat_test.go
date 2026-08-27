package journey_test

// Eating-track ("Отношения с едой") scenarios: the one-time track consent,
// the mini-menu, the three instruments (EDE-QS / BES / NIAS) question by
// question, the cutoff boundaries, the deterministic Burton Murray reading
// of the NIAS against the EDE-QS, the combined doctor report, and the
// pause / escape / delete matrix.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// --- synthetic eating content ----------------------------------------------
// LoadEating pins the canonical shape (12 EDE-QS items on two scales, the 16
// weighted BES groups, the 9 NIAS statements in three subscales) and the
// published thresholds, so the test set generates a complete synthetic
// bundle with short texts and the canonical numbers.

// eatBesWeights mirrors the canonical BES statement weights: repeated
// weights inside a group and two groups topping out at 2 (max sum 46).
var eatBesWeights = [][]int{
	{0, 0, 1, 3}, {0, 1, 2, 3}, {0, 1, 3, 3}, {0, 0, 0, 2},
	{0, 1, 2, 3}, {0, 1, 3}, {0, 2, 3, 3}, {0, 1, 2, 3},
	{0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3},
	{0, 0, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2},
}

func eatSyntheticEDEQS() string {
	var b strings.Builder
	b.WriteString("instruction_ru: \"E instruction about the last 7 days.\"\n")
	b.WriteString("days_header_ru: \"E DAYS HEADER\"\n")
	b.WriteString("severity_header_ru: \"E SEVERITY HEADER\"\n")
	b.WriteString("scale_days:\n")
	for i := 0; i <= 3; i++ {
		fmt.Fprintf(&b, "  - {score: %d, label: \"D%d\"}\n", i, i)
	}
	b.WriteString("scale_severity:\n")
	for i := 0; i <= 3; i++ {
		fmt.Fprintf(&b, "  - {score: %d, label: \"S%d\"}\n", i, i)
	}
	b.WriteString("items:\n")
	for id := 1; id <= 12; id++ {
		scale := "days"
		if id >= 11 {
			scale = "severity"
		}
		fmt.Fprintf(&b, "  - {id: %d, scale: %s, text: \"E question %d?\"}\n", id, scale, id)
	}
	b.WriteString(`scoring:
  cutoff: 15
  bands:
    - {min: 0, max: 14, id: below}
    - {min: 15, max: 36, id: at_or_above}
attribution_text_original: "EDE-QS original attribution."
`)
	return b.String()
}

func eatSyntheticBES() string {
	var b strings.Builder
	b.WriteString("instruction_ru: \"B instruction: pick one statement per group.\"\n")
	b.WriteString("items:\n")
	for i, weights := range eatBesWeights {
		fmt.Fprintf(&b, "  - id: %d\n    statements:\n", i+1)
		for j, w := range weights {
			fmt.Fprintf(&b, "      - {score: %d, text: \"B group %d statement %d\"}\n", w, i+1, j+1)
		}
	}
	b.WriteString(`scoring:
  bands:
    - {min: 0, max: 17, id: low}
    - {min: 18, max: 26, id: moderate}
    - {min: 27, max: 46, id: severe}
attribution_text_ru: "BES attribution."
`)
	return b.String()
}

func eatSyntheticNIAS() string {
	var b strings.Builder
	b.WriteString("instruction_ru: \"N instruction.\"\n")
	b.WriteString("scale:\n")
	for i := 0; i <= 5; i++ {
		fmt.Fprintf(&b, "  - {score: %d, label: \"N%d\"}\n", i, i)
	}
	b.WriteString("items:\n")
	for id := 1; id <= 9; id++ {
		subscale := []string{"picky", "appetite", "fear"}[(id-1)/3]
		fmt.Fprintf(&b, "  - {id: %d, subscale: %s, text: \"N statement %d\"}\n", id, subscale, id)
	}
	b.WriteString(`scoring:
  subscales:
    - {id: picky, cutoff: 10}
    - {id: appetite, cutoff: 9}
    - {id: fear, cutoff: 10}
attribution_text_ru: "NIAS attribution."
`)
	return b.String()
}

func eatSyntheticModule() string {
	return `meta:
  title: "Food self-check"
  disclaimer: "EAT-DISCLAIMER not a diagnosis"
menu:
  prompt: "Food menu prompt"
  edeqs_button: "Core test"
  bes_button: "Overeating"
  nias_button: "Picky eating"
  resume_edeqs_button: "Resume core test"
  resume_bes_button: "Resume overeating"
  resume_nias_button: "Resume picky eating"
consent:
  title: "Food consent"
  body: "food consent body"
  agree_button: "Got it"
  later_button: "Not now, food"
  declined: "food declined text"
edeqs:
  title: "Core test title"
  results:
    score_line: "EDE-QS: {score} of 36 (cutoff {cutoff})."
    bands:
      below: "EDEQS-BAND-BELOW"
      at_or_above: "EDEQS-BAND-AT-OR-ABOVE"
    attribution_line: "EDEQS-ATTR-LINE"
bes:
  title: "Overeating title"
  progress: "Group {current} of {total}"
  pick_hint: "Send the option number."
  results:
    score_line: "BES: {score} of 46."
    bands:
      low: "BES-BAND-LOW"
      moderate: "BES-BAND-MODERATE"
      severe: "BES-BAND-SEVERE"
    attribution_line: "BES-ATTR-LINE"
nias:
  title: "Picky eating title"
  progress: "Statement {current} of {total}"
  subscales:
    picky: "Picky"
    appetite: "Appetite"
    fear: "Fear"
  results:
    subscale_line: "{name}: {score} of 15 (cutoff {cutoff}) - {verdict}"
    verdict_above: "VERDICT-ABOVE"
    verdict_below: "VERDICT-BELOW"
    contexts:
      restrictive_no_body_image: "CTX-NO-BODY-IMAGE"
      restrictive_body_image: "CTX-BODY-IMAGE"
      restrictive_unknown: "CTX-UNKNOWN"
    attribution_line: "NIAS-ATTR-LINE"
doctor_report:
  lead_in: "food report lead-in"
  heading: "FOOD SUMMARY REPORT"
  edeqs_line: "EDEQS ({date}): {score}/36, cutoff {cutoff} - screen {verdict}"
  verdict_positive: "screen-positive"
  verdict_negative: "screen-negative"
  bes_line: "BES ({date}): {score}/46 - {band}"
  nias_line: "NIAS ({date}): picky {picky}/15 (>={picky_cutoff}), appetite {appetite}/15 (>={appetite_cutoff}), fear {fear}/15 (>={fear_cutoff})"
  nias_contexts:
    restrictive_no_body_image: "RPT-CTX-NO-BODY-IMAGE"
    restrictive_body_image: "RPT-CTX-BODY-IMAGE"
    restrictive_unknown: "RPT-CTX-UNKNOWN"
  fodmap_context_line: "RPT-FODMAP-CONTEXT"
  footer: "FOOD-REPORT-FOOTER"
ui:
  mode_button: "Food self-check"
  results_heading: "Your food result"
  progress: "Question {current} of {total}"
  abandon_confirmed: "food test abandoned"
  delete_confirm_prompt: "delete all food data?"
  delete_confirm_button: "Yes, drop food data"
  delete_cancel_button: "Keep food data"
  delete_done: "food data deleted"
  delete_nothing: "nothing food stored"
`
}

func writeEatTestContent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"edeqs_ru.yaml":         eatSyntheticEDEQS(),
		"bes_ru.yaml":           eatSyntheticBES(),
		"nias_ru.yaml":          eatSyntheticNIAS(),
		"eating_module_ru.yaml": eatSyntheticModule(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func registerEatingPhases(runner *journey.Runner, content *screening.EatingContent) {
	runner.Register(journey.NewEatConsentPhase(content))
	runner.Register(journey.NewEatMenuPhase(content))
	runner.Register(journey.NewEatEdeqsPhase(content))
	runner.Register(journey.NewEatBesPhase(content))
	runner.Register(journey.NewEatNiasPhase(content))
	runner.Register(journey.NewEatReportPhase(content))
	runner.Register(journey.NewEatDeleteConfirmPhase(content))
}

// --- answer builders --------------------------------------------------------

// edeqsAnswersFor builds the twelve scale labels summing to target (0..36):
// items 1–10 answer on the days scale, 11–12 on the severity scale.
func edeqsAnswersFor(t *testing.T, target int) []string {
	t.Helper()
	if target < 0 || target > 36 {
		t.Fatalf("edeqsAnswersFor: unreachable target %d", target)
	}
	out := make([]string, 0, 12)
	remaining := target
	for i := 0; i < 12; i++ {
		v := remaining
		if v > 3 {
			v = 3
		}
		prefix := "D"
		if i >= 10 {
			prefix = "S"
		}
		out = append(out, prefix+strconv.Itoa(v))
		remaining -= v
	}
	if remaining != 0 {
		t.Fatalf("edeqsAnswersFor(%d): %d points left over", target, remaining)
	}
	return out
}

// besPicksFor builds the sixteen option numbers whose picked statement
// weights sum to target (0..46). Greedy: each group contributes the largest
// weight it offers that still fits.
func besPicksFor(t *testing.T, target int) []string {
	t.Helper()
	out := make([]string, 0, len(eatBesWeights))
	remaining := target
	for _, group := range eatBesWeights {
		best, bestIdx := 0, 0
		for i, w := range group {
			if w <= remaining && w > best {
				best, bestIdx = w, i
			}
		}
		out = append(out, strconv.Itoa(bestIdx+1))
		remaining -= best
	}
	if remaining != 0 {
		t.Fatalf("besPicksFor(%d): %d points left over", target, remaining)
	}
	return out
}

// niasAnswersFor builds the nine scale labels producing the three subscale
// sums (each 0..15, three items of 0..5 per subscale).
func niasAnswersFor(t *testing.T, picky, appetite, fear int) []string {
	t.Helper()
	out := make([]string, 0, 9)
	for _, sum := range []int{picky, appetite, fear} {
		if sum < 0 || sum > 15 {
			t.Fatalf("niasAnswersFor: unreachable subscale sum %d", sum)
		}
		remaining := sum
		for i := 0; i < 3; i++ {
			v := remaining
			if v > 5 {
				v = 5
			}
			out = append(out, "N"+strconv.Itoa(v))
			remaining -= v
		}
	}
	return out
}

// --- drive helpers ----------------------------------------------------------

// openEatMenu enters the eating track via /food, passing the one-time
// consent when it is asked, and asserts the track menu is reached.
func openEatMenu(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64) {
	t.Helper()
	runner.HandleFood(sender, newMsg(id, "/food"))
	if st.Get(id).State == state.StateEatConsent {
		say(runner, sender, id, "Got it")
	}
	if got := st.Get(id).State; got != state.StateEatMenu {
		t.Fatalf("expected the eating menu, got %q", got)
	}
}

// driveEdeqs runs the core test from the menu to the landing with a total of
// exactly target points.
func driveEdeqs(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64, target int) {
	t.Helper()
	openEatMenu(t, runner, st, sender, id)
	say(runner, sender, id, "Core test")
	for _, l := range edeqsAnswersFor(t, target) {
		say(runner, sender, id, l)
	}
}

// driveBes runs the overeating test from the menu with a total of exactly
// target points.
func driveBes(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64, target int) {
	t.Helper()
	openEatMenu(t, runner, st, sender, id)
	say(runner, sender, id, "Overeating")
	for _, l := range besPicksFor(t, target) {
		say(runner, sender, id, l)
	}
}

// driveNias runs the picky-eating test from the menu with the given subscale
// sums.
func driveNias(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64, picky, appetite, fear int) {
	t.Helper()
	openEatMenu(t, runner, st, sender, id)
	say(runner, sender, id, "Picky eating")
	for _, l := range niasAnswersFor(t, picky, appetite, fear) {
		say(runner, sender, id, l)
	}
}

// eatResultText returns the instrument result message of a just-finished
// run: the chain always ends result → doctor report → landing prompt.
func eatResultText(t *testing.T, sender *mockSender) string {
	t.Helper()
	texts := sentTexts(sender)
	if len(texts) < 3 {
		t.Fatalf("expected result + report + landing, got %q", texts)
	}
	return texts[len(texts)-3]
}

// eatDoctorReportText returns the combined doctor report of a just-finished
// run.
func eatDoctorReportText(t *testing.T, sender *mockSender) string {
	t.Helper()
	texts := sentTexts(sender)
	if len(texts) < 3 {
		t.Fatalf("expected result + report + landing, got %q", texts)
	}
	return texts[len(texts)-2]
}

// --- entry, consent, menu ---------------------------------------------------

func TestEat_LandingHasFourthButton(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleStart(sender, newMsg(1, "/start"))
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Fatalf("expected the landing, got %q", got)
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Food") {
		t.Fatalf("landing must offer the eating track, got %v", kb)
	}
	say(runner, sender, 1, "Food")
	if got := st.Get(1).State; got != state.StateEatConsent {
		t.Fatalf("expected the eating consent, got %q", got)
	}
	text := sender.lastText()
	for _, want := range []string{"Food consent", "food consent body", "EAT-DISCLAIMER"} {
		if !contains(text, want) {
			t.Errorf("consent missing %q:\n%s", want, text)
		}
	}
}

func TestEat_ConsentDeclineLeavesNoTrace(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	runner.HandleFood(sender, newMsg(1, "/food"))
	if got := st.Get(1).State; got != state.StateEatConsent {
		t.Fatalf("expected EatConsent, got %q", got)
	}
	say(runner, sender, 1, "Not now, food")

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after decline, got %q", d.State)
	}
	if d.EatConsentAt != nil || d.Edeqs != nil || d.Bes != nil || d.Nias != nil {
		t.Error("decline must not create any eating data")
	}
	texts := sentTexts(sender)
	if len(texts) < 2 || !contains(texts[len(texts)-2], "food declined text") {
		t.Errorf("declined text not sent, got %q", texts)
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"edeqs`) ||
		strings.Contains(raw, `"eat_consent_at`) {
		t.Errorf("persisted JSON must not mention the eating track after decline:\n%s", raw)
	}
}

// TestEat_ConsentOnceOpensMenu pins the track's consent contract: one
// consent for all three instruments, asked once, opening the mini-menu.
func TestEat_ConsentOnceOpensMenu(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleFood(sender, newMsg(1, "/food"))
	say(runner, sender, 1, "Got it")

	d := st.Get(1)
	if d.State != state.StateEatMenu {
		t.Fatalf("consent must open the menu, got %q", d.State)
	}
	if d.EatConsentAt == nil {
		t.Fatal("agreeing must record the track-wide consent")
	}
	if !contains(sender.lastText(), "Food menu prompt") {
		t.Errorf("menu prompt missing: %q", sender.lastText())
	}
	kb := lastKeyboard(sender)
	for _, want := range []string{"Core test", "Overeating", "Picky eating", "Home"} {
		if !keyboardHas(kb, want) {
			t.Errorf("menu must offer %q, got %v", want, kb)
		}
	}

	// A full instrument run passes without any further consent…
	driveNias(t, runner, st, sender, 1, 0, 0, 0)

	// …and the next /food goes straight to the menu.
	runner.HandleFood(sender, newMsg(1, "/food"))
	if got := st.Get(1).State; got != state.StateEatMenu {
		t.Fatalf("second entry must skip consent, got %q", got)
	}
	consentShown := 0
	for _, txt := range sentTexts(sender) {
		if contains(txt, "food consent body") {
			consentShown++
		}
	}
	if consentShown != 1 {
		t.Errorf("consent must be asked exactly once, shown %d times", consentShown)
	}
}

// --- questions one by one ---------------------------------------------------

func TestEat_EdeqsQuestionsOneByOne(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	q1 := sender.lastText()
	for _, want := range []string{
		"Core test title", "E instruction about the last 7 days.",
		"E DAYS HEADER", "Question 1 of 12", "E question 1?",
	} {
		if !contains(q1, want) {
			t.Errorf("first question missing %q:\n%s", want, q1)
		}
	}
	if contains(q1, "E SEVERITY HEADER") {
		t.Errorf("the severity block must not be announced up front:\n%s", q1)
	}

	say(runner, sender, 1, "D1")
	q2 := sender.lastText()
	if !contains(q2, "Question 2 of 12") || !contains(q2, "E question 2?") {
		t.Errorf("second question wrong:\n%s", q2)
	}
	for _, unwanted := range []string{"E question 1?", "E DAYS HEADER", "Core test title"} {
		if contains(q2, unwanted) {
			t.Errorf("second question must not repeat %q:\n%s", unwanted, q2)
		}
	}

	// The paper form switches framing after item 10 — the header appears
	// exactly there, once.
	for i := 2; i <= 10; i++ {
		say(runner, sender, 1, "D0")
	}
	q11 := sender.lastText()
	if !contains(q11, "E SEVERITY HEADER") || !contains(q11, "Question 11 of 12") {
		t.Errorf("question 11 must open the severity block:\n%s", q11)
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "S0") || keyboardHas(kb, "D0") {
		t.Errorf("question 11 must switch to the severity keyboard, got %v", kb)
	}
	say(runner, sender, 1, "S0")
	if q12 := sender.lastText(); contains(q12, "E SEVERITY HEADER") {
		t.Errorf("the severity header must be shown once:\n%s", q12)
	}
}

func TestEat_BesGroupsAreNumberedLists(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Overeating")
	g1 := sender.lastText()
	for _, want := range []string{
		"Overeating title", "B instruction: pick one statement per group.",
		"Group 1 of 16", "1) B group 1 statement 1", "4) B group 1 statement 4",
		"Send the option number.",
	} {
		if !contains(g1, want) {
			t.Errorf("first group missing %q:\n%s", want, g1)
		}
	}
	kb := lastKeyboard(sender)
	for _, want := range []string{"1", "2", "3", "4"} {
		if !keyboardHas(kb, want) {
			t.Errorf("group keyboard must offer %q, got %v", want, kb)
		}
	}
	// Group 6 has only three statements — the keyboard must shrink with it.
	for i := 1; i <= 5; i++ {
		say(runner, sender, 1, "1")
	}
	if !contains(sender.lastText(), "Group 6 of 16") {
		t.Fatalf("expected group 6, got:\n%s", sender.lastText())
	}
	if kb := lastKeyboard(sender); keyboardHas(kb, "4") {
		t.Errorf("group 6 has three statements, keyboard offered a fourth: %v", kb)
	}
	say(runner, sender, 1, "9")
	if !contains(sender.lastText(), "Tap a scale button.") {
		t.Errorf("an out-of-range number must be rejected, got:\n%s", sender.lastText())
	}
}

func TestEat_NiasStatementsOneByOne(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Picky eating")
	s1 := sender.lastText()
	for _, want := range []string{"Picky eating title", "N instruction.", "Statement 1 of 9", "N statement 1"} {
		if !contains(s1, want) {
			t.Errorf("first statement missing %q:\n%s", want, s1)
		}
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "N0") || !keyboardHas(kb, "N5") {
		t.Errorf("NIAS keyboard must carry the full 0..5 scale, got %v", kb)
	}
	say(runner, sender, 1, "N5")
	if !contains(sender.lastText(), "Statement 2 of 9") {
		t.Errorf("second statement wrong:\n%s", sender.lastText())
	}
}

// --- cutoffs ----------------------------------------------------------------

// TestEat_EdeqsCutoffBoundary pins the published screening cutoff: ≥ 15, not
// > 15. The stored result carries the cutoff that was applied.
func TestEat_EdeqsCutoffBoundary(t *testing.T) {
	for _, tc := range []struct {
		score    int
		positive bool
		band     string
	}{
		{0, false, "EDEQS-BAND-BELOW"},
		{14, false, "EDEQS-BAND-BELOW"},
		{15, true, "EDEQS-BAND-AT-OR-ABOVE"},
		{16, true, "EDEQS-BAND-AT-OR-ABOVE"},
		{36, true, "EDEQS-BAND-AT-OR-ABOVE"},
	} {
		t.Run(strconv.Itoa(tc.score), func(t *testing.T) {
			runner, st, sender, _ := setupScr(t)
			driveEdeqs(t, runner, st, sender, 1, tc.score)

			res := st.Get(1).EdeqsResult
			if res == nil {
				t.Fatal("completion must store an EdeqsResult")
			}
			if res.Score != tc.score {
				t.Errorf("score = %d, want %d", res.Score, tc.score)
			}
			if res.Cutoff != 15 {
				t.Errorf("applied cutoff = %d, want 15", res.Cutoff)
			}
			if res.Positive != tc.positive {
				t.Errorf("positive = %v, want %v", res.Positive, tc.positive)
			}
			result := eatResultText(t, sender)
			if !contains(result, tc.band) {
				t.Errorf("result must carry %q:\n%s", tc.band, result)
			}
			if !contains(result, "EDE-QS: "+strconv.Itoa(tc.score)+" of 36 (cutoff 15).") {
				t.Errorf("score line wrong:\n%s", result)
			}
			// One short disclaimer plus one compact attribution line — no
			// methodology hedging anywhere.
			for _, want := range []string{"Your food result", "EAT-DISCLAIMER", "EDEQS-ATTR-LINE"} {
				if !contains(result, want) {
					t.Errorf("result missing %q:\n%s", want, result)
				}
			}
		})
	}
}

// TestEat_BesBandBoundaries pins the three severity bands over the 0..46 sum.
func TestEat_BesBandBoundaries(t *testing.T) {
	for _, tc := range []struct {
		score int
		band  string
		line  string
	}{
		{0, "low", "BES-BAND-LOW"},
		{17, "low", "BES-BAND-LOW"},
		{18, "moderate", "BES-BAND-MODERATE"},
		{26, "moderate", "BES-BAND-MODERATE"},
		{27, "severe", "BES-BAND-SEVERE"},
		{46, "severe", "BES-BAND-SEVERE"},
	} {
		t.Run(strconv.Itoa(tc.score), func(t *testing.T) {
			runner, st, sender, _ := setupScr(t)
			driveBes(t, runner, st, sender, 1, tc.score)

			res := st.Get(1).BesResult
			if res == nil {
				t.Fatal("completion must store a BesResult")
			}
			if res.Score != tc.score {
				t.Errorf("score = %d, want %d", res.Score, tc.score)
			}
			if res.Band != tc.band {
				t.Errorf("band = %q, want %q", res.Band, tc.band)
			}
			result := eatResultText(t, sender)
			if !contains(result, tc.line) || !contains(result, "BES: "+strconv.Itoa(tc.score)+" of 46.") {
				t.Errorf("result wrong for score %d:\n%s", tc.score, result)
			}
		})
	}
}

// TestEat_NiasSubscaleCutoffBoundaries pins the three published subscale
// cutoffs (10 / 9 / 10) — each read on its own, with no total anywhere.
func TestEat_NiasSubscaleCutoffBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		picky, appetite, fear int
		wantP, wantA, wantF   bool
	}{
		{"all below", 9, 8, 9, false, false, false},
		{"picky at cutoff", 10, 8, 9, true, false, false},
		{"appetite at cutoff", 9, 9, 9, false, true, false},
		{"fear at cutoff", 9, 8, 10, false, false, true},
		{"all at max", 15, 15, 15, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner, st, sender, _ := setupScr(t)
			driveNias(t, runner, st, sender, 1, tc.picky, tc.appetite, tc.fear)

			res := st.Get(1).NiasResult
			if res == nil {
				t.Fatal("completion must store a NiasResult")
			}
			if res.Picky != tc.picky || res.Appetite != tc.appetite || res.Fear != tc.fear {
				t.Errorf("subscales = %d/%d/%d, want %d/%d/%d",
					res.Picky, res.Appetite, res.Fear, tc.picky, tc.appetite, tc.fear)
			}
			if res.PickyCutoff != 10 || res.AppetiteCutoff != 9 || res.FearCutoff != 10 {
				t.Errorf("applied cutoffs = %d/%d/%d, want 10/9/10",
					res.PickyCutoff, res.AppetiteCutoff, res.FearCutoff)
			}
			if res.PickyPositive != tc.wantP || res.AppetitePositive != tc.wantA || res.FearPositive != tc.wantF {
				t.Errorf("verdicts = %v/%v/%v, want %v/%v/%v",
					res.PickyPositive, res.AppetitePositive, res.FearPositive,
					tc.wantP, tc.wantA, tc.wantF)
			}

			result := eatResultText(t, sender)
			for _, row := range []struct {
				name     string
				score    int
				cutoff   int
				positive bool
			}{
				{"Picky", tc.picky, 10, tc.wantP},
				{"Appetite", tc.appetite, 9, tc.wantA},
				{"Fear", tc.fear, 10, tc.wantF},
			} {
				verdict := "VERDICT-BELOW"
				if row.positive {
					verdict = "VERDICT-ABOVE"
				}
				want := fmt.Sprintf("%s: %d of 15 (cutoff %d) - %s", row.name, row.score, row.cutoff, verdict)
				if !contains(result, want) {
					t.Errorf("result missing %q:\n%s", want, result)
				}
			}
		})
	}
}

// --- the Burton Murray reading rule ----------------------------------------

// TestEat_NiasReadingAgainstEdeqs is the rule's truth table as the user and
// the doctor see it: a positive subscale means "restrictive without body
// image concern" below the EDE-QS cutoff, "likely tied to body image" at or
// above it, and degrades to the neutral wording when the core test was never
// taken. No positive subscale → no reading line at all.
func TestEat_NiasReadingAgainstEdeqs(t *testing.T) {
	const (
		noBodyImage = "CTX-NO-BODY-IMAGE"
		bodyImage   = "CTX-BODY-IMAGE"
		unknown     = "CTX-UNKNOWN"
	)
	allContexts := []string{noBodyImage, bodyImage, unknown}

	for _, tc := range []struct {
		name       string
		edeqsScore int // -1 = the core test is not taken at all
		picky      int
		want       string // "" = no reading line
	}{
		{"no core test, subscale above cutoff", -1, 15, unknown},
		{"core test below cutoff, subscale above", 14, 15, noBodyImage},
		{"core test at cutoff, subscale above", 15, 15, bodyImage},
		{"core test above cutoff, subscale above", 36, 15, bodyImage},
		{"no core test, all subscales below", -1, 9, ""},
		{"core test below cutoff, all subscales below", 14, 9, ""},
		{"core test at cutoff, all subscales below", 15, 9, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner, st, sender, _ := setupScr(t)
			if tc.edeqsScore >= 0 {
				driveEdeqs(t, runner, st, sender, 1, tc.edeqsScore)
			}
			driveNias(t, runner, st, sender, 1, tc.picky, 0, 0)

			result := eatResultText(t, sender)
			report := eatDoctorReportText(t, sender)
			for _, ctx := range allContexts {
				wantHere := ctx == tc.want
				if got := contains(result, ctx); got != wantHere {
					t.Errorf("result: %q present = %v, want %v:\n%s", ctx, got, wantHere, result)
				}
				if got := contains(report, "RPT-"+ctx); got != wantHere {
					t.Errorf("doctor report: %q present = %v, want %v:\n%s", "RPT-"+ctx, got, wantHere, report)
				}
			}
		})
	}
}

// TestEat_NiasReadingUpgradesAfterTheCoreTest pins the degradation path
// explicitly: a NIAS taken first reads neutrally, and re-reading it after
// the core test resolves the distinction.
func TestEat_NiasReadingUpgradesAfterTheCoreTest(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	driveNias(t, runner, st, sender, 1, 15, 0, 0)
	if first := eatResultText(t, sender); !contains(first, "CTX-UNKNOWN") {
		t.Fatalf("a NIAS without the core test must read neutrally:\n%s", first)
	}

	driveEdeqs(t, runner, st, sender, 1, 14)
	driveNias(t, runner, st, sender, 1, 15, 0, 0)
	second := eatResultText(t, sender)
	if !contains(second, "CTX-NO-BODY-IMAGE") || contains(second, "CTX-UNKNOWN") {
		t.Errorf("the re-read must resolve to the restrictive-without-body-image wording:\n%s", second)
	}
}

// --- the combined doctor report --------------------------------------------

func TestEat_CombinedReportListsEverythingWithDates(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	driveEdeqs(t, runner, st, sender, 1, 15)
	driveBes(t, runner, st, sender, 1, 27)
	driveNias(t, runner, st, sender, 1, 15, 0, 0)

	report := eatDoctorReportText(t, sender)
	date := st.Get(1).EdeqsResult.TakenAt.Format("02.01.2006")
	for _, want := range []string{
		"FOOD SUMMARY REPORT",
		"EDEQS (" + date + "): 15/36, cutoff 15 - screen screen-positive",
		"BES (" + date + "): 27/46 - BES-BAND-SEVERE",
		"NIAS (" + date + "): picky 15/15 (>=10), appetite 0/15 (>=9), fear 0/15 (>=10)",
		"RPT-CTX-BODY-IMAGE",
		"FOOD-REPORT-FOOTER",
	} {
		if !contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}
	if !contains(eatResultText(t, sender), "Your food result") {
		t.Error("the instrument result must precede the combined report")
	}
	// The report is rendered on the fly and never stored.
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, "FOOD SUMMARY REPORT") {
		t.Errorf("the doctor report must never reach the store:\n%s", raw)
	}
	// Every completion ends on the landing.
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("the report transit must land home, got %q", got)
	}
}

// TestEat_FodmapContextLineFollowsRealDiaryActivity pins the automatic
// context line: a clinician reading restraint items must be told that part
// of this person's dietary restriction is medically motivated — but only
// when the user actually keeps the diary.
func TestEat_FodmapContextLineFollowsRealDiaryActivity(t *testing.T) {
	t.Run("no diary activity", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		driveEdeqs(t, runner, st, sender, 1, 15)
		if report := eatDoctorReportText(t, sender); contains(report, "RPT-FODMAP-CONTEXT") {
			t.Errorf("a user without diary activity must not get the FODMAP line:\n%s", report)
		}
	})

	t.Run("recorded product progress", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		d := st.Get(1)
		d.Locale = "en"
		d.Products = map[string]state.ProductProgress{"Apple": {Status: "completed"}}
		st.Set(1, d)

		driveEdeqs(t, runner, st, sender, 1, 15)
		if report := eatDoctorReportText(t, sender); !contains(report, "RPT-FODMAP-CONTEXT") {
			t.Errorf("recorded diary progress must add the FODMAP line:\n%s", report)
		}
	})

	t.Run("active trial", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		driveToCheckin(t, runner, st, sender, 1)
		driveEdeqs(t, runner, st, sender, 1, 15)
		if report := eatDoctorReportText(t, sender); !contains(report, "RPT-FODMAP-CONTEXT") {
			t.Errorf("an active trial must add the FODMAP line:\n%s", report)
		}
	})
}

// --- pause / escape matrix --------------------------------------------------

// TestEat_EscapeMatrix is the track's escape table: from every eat_* state
// both /start and the 🏠 button land on the landing without losing progress,
// and a merely paused run is never marked as interrupted.
func TestEat_EscapeMatrix(t *testing.T) {
	rows := []struct {
		name    string
		arrange func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender)
		from    state.StateKind
		check   func(t *testing.T, d state.UserData)
	}{
		{
			name: "eat consent",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleFood(sender, newMsg(1, "/food"))
			},
			from: state.StateEatConsent,
			check: func(t *testing.T, d state.UserData) {
				if d.EatConsentAt != nil {
					t.Error("no consent given — nothing must be recorded")
				}
			},
		},
		{
			name: "eat menu",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				openEatMenu(t, runner, st, sender, 1)
			},
			from: state.StateEatMenu,
			check: func(t *testing.T, d state.UserData) {
				if d.EatConsentAt == nil {
					t.Error("the track consent must survive the escape")
				}
			},
		},
		{
			name: "edeqs question",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				openEatMenu(t, runner, st, sender, 1)
				say(runner, sender, 1, "Core test")
				say(runner, sender, 1, "D3")
				say(runner, sender, 1, "D2")
			},
			from: state.StateEatEdeqsQuestion,
			check: func(t *testing.T, d state.UserData) {
				if d.Edeqs == nil || len(d.Edeqs.Answers) != 2 {
					t.Fatalf("answers must survive the escape: %+v", d.Edeqs)
				}
				if d.Edeqs.ResumeState != "" {
					t.Errorf("a paused run needs no marker, got %q", d.Edeqs.ResumeState)
				}
			},
		},
		{
			name: "bes question",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				openEatMenu(t, runner, st, sender, 1)
				say(runner, sender, 1, "Overeating")
				say(runner, sender, 1, "4")
			},
			from: state.StateEatBesQuestion,
			check: func(t *testing.T, d state.UserData) {
				if d.Bes == nil || len(d.Bes.Answers) != 1 {
					t.Fatalf("answers must survive the escape: %+v", d.Bes)
				}
			},
		},
		{
			name: "nias question",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				openEatMenu(t, runner, st, sender, 1)
				say(runner, sender, 1, "Picky eating")
				say(runner, sender, 1, "N5")
				say(runner, sender, 1, "N4")
				say(runner, sender, 1, "N3")
			},
			from: state.StateEatNiasQuestion,
			check: func(t *testing.T, d state.UserData) {
				if d.Nias == nil || len(d.Nias.Answers) != 3 {
					t.Fatalf("answers must survive the escape: %+v", d.Nias)
				}
			},
		},
		{
			name: "eat delete confirm",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				driveNias(t, runner, st, sender, 1, 0, 0, 0)
				runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
			},
			from: state.StateEatDeleteConfirm,
			check: func(t *testing.T, d state.UserData) {
				if d.NiasResult == nil {
					t.Error("escaping the delete dialog must not delete anything")
				}
			},
		},
	}

	for _, row := range rows {
		for _, escape := range []struct {
			name string
			do   func(runner *journey.Runner, sender *mockSender)
		}{
			{"/start", func(runner *journey.Runner, sender *mockSender) {
				runner.HandleStart(sender, newMsg(1, "/start"))
			}},
			{"home button", func(runner *journey.Runner, sender *mockSender) {
				say(runner, sender, 1, "Home")
			}},
		} {
			t.Run(row.name+" via "+escape.name, func(t *testing.T) {
				runner, st, sender, _ := setupScr(t)
				row.arrange(t, runner, st, sender)
				if got := st.Get(1).State; got != row.from {
					t.Fatalf("precondition: expected %q, got %q", row.from, got)
				}
				escape.do(runner, sender)
				d := st.Get(1)
				if d.State != state.StateAwaitingModeChoice {
					t.Fatalf("escape must land on the landing, got %q", d.State)
				}
				row.check(t, d)
			})
		}
	}
}

// TestEat_ResumeFromLandingReturnsToTheNextQuestion pins the contextual
// resume row: a single paused run is re-entered directly, at the exact
// question it paused on (derived from the recorded answers).
func TestEat_ResumeFromLandingReturnsToTheNextQuestion(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	for i := 0; i < 4; i++ {
		say(runner, sender, 1, "D1")
	}
	say(runner, sender, 1, "Home")

	if kb := lastKeyboard(sender); !keyboardHas(kb, "Resume food") {
		t.Fatalf("landing must offer the eating resume row, got %v", kb)
	}
	say(runner, sender, 1, "Resume food")

	if got := st.Get(1).State; got != state.StateEatEdeqsQuestion {
		t.Fatalf("resume must re-enter the core test, got %q", got)
	}
	if !contains(sender.lastText(), "Question 5 of 12") {
		t.Errorf("resume must land on question 5:\n%s", sender.lastText())
	}
	if d := st.Get(1); len(d.Edeqs.Answers) != 4 {
		t.Errorf("resume must not touch the answers: %+v", d.Edeqs)
	}
}

// TestEat_TwoPausedRunsResumeViaTheMenu: with several paused runs the single
// landing row cannot disambiguate, so it opens the menu, whose per-instrument
// resume rows do.
func TestEat_TwoPausedRunsResumeViaTheMenu(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	say(runner, sender, 1, "D3")
	say(runner, sender, 1, "Home")

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Picky eating")
	say(runner, sender, 1, "N5")
	say(runner, sender, 1, "Home")

	say(runner, sender, 1, "Resume food")
	if got := st.Get(1).State; got != state.StateEatMenu {
		t.Fatalf("two paused runs must land on the menu, got %q", got)
	}
	kb := lastKeyboard(sender)
	for _, want := range []string{"Resume core test", "Resume picky eating"} {
		if !keyboardHas(kb, want) {
			t.Errorf("menu must offer %q, got %v", want, kb)
		}
	}
	if keyboardHas(kb, "Resume overeating") {
		t.Errorf("no BES run is paused — its resume row must be absent: %v", kb)
	}

	say(runner, sender, 1, "Resume picky eating")
	if got := st.Get(1).State; got != state.StateEatNiasQuestion {
		t.Fatalf("expected the picky-eating run, got %q", got)
	}
	if !contains(sender.lastText(), "Statement 2 of 9") {
		t.Errorf("resume must land on statement 2:\n%s", sender.lastText())
	}
}

// TestEat_MenuInstrumentButtonStartsOver: the plain instrument button always
// starts a FRESH run — the resume row sitting directly above it makes that
// an explicit choice.
func TestEat_MenuInstrumentButtonStartsOver(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	for i := 0; i < 4; i++ {
		say(runner, sender, 1, "D1")
	}
	say(runner, sender, 1, "Home")
	openEatMenu(t, runner, st, sender, 1)
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Resume core test") {
		t.Fatalf("the menu must offer the resume row above the button, got %v", kb)
	}

	say(runner, sender, 1, "Core test")
	if d := st.Get(1); d.Edeqs == nil || len(d.Edeqs.Answers) != 0 {
		t.Fatalf("the plain button must start over: %+v", d.Edeqs)
	}
	if !contains(sender.lastText(), "Question 1 of 12") {
		t.Errorf("start-over must show question 1:\n%s", sender.lastText())
	}
}

// TestEat_FoodCommandMidTestRedrawsTheQuestion: /food inside the track is a
// redraw, never a restart.
func TestEat_FoodCommandMidTestRedrawsTheQuestion(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Overeating")
	say(runner, sender, 1, "4")
	say(runner, sender, 1, "4")

	runner.HandleFood(sender, newMsg(1, "/food"))
	if got := st.Get(1).State; got != state.StateEatBesQuestion {
		t.Fatalf("/food mid-test must stay in the run, got %q", got)
	}
	if d := st.Get(1); len(d.Bes.Answers) != 2 {
		t.Fatalf("/food must not touch the answers: %+v", d.Bes)
	}
	if !contains(sender.lastText(), "Group 3 of 16") {
		t.Errorf("/food must redraw the current group:\n%s", sender.lastText())
	}
}

// TestEat_AbandonWipesOnlyTheCurrentRun: /abandon mid-test drops the raw
// answers of THAT run, keeps a paused run of another instrument and every
// completed result, and lands home.
func TestEat_AbandonWipesOnlyTheCurrentRun(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	driveNias(t, runner, st, sender, 1, 0, 0, 0) // a completed result to protect
	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	say(runner, sender, 1, "D3")
	say(runner, sender, 1, "Home")

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Overeating")
	say(runner, sender, 1, "4")
	runner.HandleAbandon(sender, newMsg(1, "/abandon"))

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Fatalf("/abandon must land home, got %q", d.State)
	}
	if d.Bes != nil {
		t.Error("/abandon must wipe the run it interrupted")
	}
	if d.Edeqs == nil || len(d.Edeqs.Answers) != 1 {
		t.Errorf("a paused run of another instrument must survive: %+v", d.Edeqs)
	}
	if d.NiasResult == nil {
		t.Error("a completed result must survive /abandon")
	}
	found := false
	for _, txt := range sentTexts(sender) {
		if contains(txt, "food test abandoned") {
			found = true
		}
	}
	if !found {
		t.Error("the track's own abandon confirmation must be sent")
	}
}

// --- /food_delete -----------------------------------------------------------

func TestEat_DeleteWithNothingStored(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleStart(sender, newMsg(1, "/start"))
	before := st.Get(1).State
	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))

	if got := st.Get(1).State; got != before {
		t.Errorf("nothing stored must not change the state: %q → %q", before, got)
	}
	if !contains(sender.lastText(), "nothing food stored") {
		t.Errorf("expected the nothing-stored answer, got %q", sender.lastText())
	}
}

func TestEat_DeleteConfirmWipesTheWholeTrack(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	driveEdeqs(t, runner, st, sender, 1, 15)
	driveBes(t, runner, st, sender, 1, 27)
	driveNias(t, runner, st, sender, 1, 15, 0, 0)

	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
	if got := st.Get(1).State; got != state.StateEatDeleteConfirm {
		t.Fatalf("expected the delete confirmation, got %q", got)
	}
	if !contains(sender.lastText(), "delete all food data?") {
		t.Errorf("confirmation prompt missing:\n%s", sender.lastText())
	}
	say(runner, sender, 1, "Yes, drop food data")

	d := st.Get(1)
	if d.EdeqsResult != nil || d.BesResult != nil || d.NiasResult != nil ||
		d.Edeqs != nil || d.Bes != nil || d.Nias != nil || d.EatConsentAt != nil {
		t.Errorf("confirm must wipe every eating field: %+v", d)
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"edeqs_result`) ||
		strings.Contains(raw, `"bes_result`) || strings.Contains(raw, `"nias_result`) {
		t.Errorf("wiped results must be gone from disk too:\n%s", raw)
	}
	// Consent is asked again on the next entry.
	runner.HandleFood(sender, newMsg(1, "/food"))
	if got := st.Get(1).State; got != state.StateEatConsent {
		t.Errorf("after a wipe the consent must be asked again, got %q", got)
	}
}

// TestEat_DeleteCancelMidTestReturnsToTheQuestion pins the mid-test cancel:
// «Keep» must never eject the user from the run they were in.
func TestEat_DeleteCancelMidTestReturnsToTheQuestion(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	for i := 0; i < 3; i++ {
		say(runner, sender, 1, "D2")
	}

	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
	if got := st.Get(1).State; got != state.StateEatDeleteConfirm {
		t.Fatalf("expected the delete confirmation, got %q", got)
	}
	say(runner, sender, 1, "Keep food data")

	d := st.Get(1)
	if d.State != state.StateEatEdeqsQuestion {
		t.Fatalf("cancel must return to the interrupted question, got %q", d.State)
	}
	if d.Edeqs == nil || len(d.Edeqs.Answers) != 3 {
		t.Fatalf("cancel must keep the answers: %+v", d.Edeqs)
	}
	if !contains(sender.lastText(), "Question 4 of 12") {
		t.Errorf("cancel must redraw the interrupted question:\n%s", sender.lastText())
	}
	// The mid-test marker is consumed by the dialog it belongs to.
	if d.Edeqs.ResumeState != "" {
		t.Errorf("the mid-test marker must not outlive the dialog, got %q", d.Edeqs.ResumeState)
	}
}

// TestEat_DeleteCancelAfterAPauseReturnsToTheLanding is the regression twin
// of the test above: a run that is merely PAUSED (the user escaped to the
// landing) must not make the delete dialog think it was opened mid-test —
// closing it there belongs on the landing, not inside the run.
func TestEat_DeleteCancelAfterAPauseReturnsToTheLanding(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	say(runner, sender, 1, "D2")
	say(runner, sender, 1, "Home")

	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
	if got := st.Get(1).State; got != state.StateEatDeleteConfirm {
		t.Fatalf("expected the delete confirmation, got %q", got)
	}
	say(runner, sender, 1, "Keep food data")

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Fatalf("cancel from the landing must return to the landing, got %q", d.State)
	}
	if d.Edeqs == nil || len(d.Edeqs.Answers) != 1 {
		t.Errorf("the paused run must stay resumable: %+v", d.Edeqs)
	}
}

// TestEat_DeleteConfirmMidTestLandsHome: confirming mid-test destroys the
// very position a cancel would return to, so that run is abandoned and the
// user lands on the landing.
func TestEat_DeleteConfirmMidTestLandsHome(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Picky eating")
	say(runner, sender, 1, "N5")

	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
	say(runner, sender, 1, "Yes, drop food data")

	d := st.Get(1)
	if d.State != state.StateAwaitingModeChoice {
		t.Fatalf("mid-test delete confirm must land home, got %q", d.State)
	}
	if d.Nias != nil || d.EatConsentAt != nil {
		t.Errorf("confirm must wipe the track: %+v", d)
	}
}

// TestEat_DeleteFromADiaryQuestionReturnsToIt pins the detour bookkeeping:
// /food_delete issued mid-diary closes back into the diary question.
func TestEat_DeleteFromADiaryQuestionReturnsToIt(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	driveNias(t, runner, st, sender, 1, 0, 0, 0) // something to delete
	driveToCheckin(t, runner, st, sender, 1)

	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
	if got := st.Get(1).State; got != state.StateEatDeleteConfirm {
		t.Fatalf("expected the delete confirmation, got %q", got)
	}
	say(runner, sender, 1, "Keep food data")

	d := st.Get(1)
	if d.State != state.StateAwaitingStageCheckin {
		t.Fatalf("cancel must return to the diary question, got %q", d.State)
	}
	if d.ReturnState != "" {
		t.Errorf("the consumed detour must be cleared, got %q", d.ReturnState)
	}
	if d.NiasResult == nil {
		t.Error("cancel must keep the stored result")
	}
}

// --- privacy ----------------------------------------------------------------

// TestEat_RawAnswersNeverOutliveTheRun: raw per-question answers are
// transient — completion persists totals, cutoffs and verdicts only, in the
// same Set that wipes the run.
func TestEat_RawAnswersNeverOutliveTheRun(t *testing.T) {
	runner, st, sender, backend := setupScr(t)

	openEatMenu(t, runner, st, sender, 1)
	say(runner, sender, 1, "Core test")
	say(runner, sender, 1, "D3")
	if raw := rawUsersJSON(t, backend); !strings.Contains(raw, `"answers"`) {
		t.Fatalf("an unfinished run keeps its answers on disk:\n%s", raw)
	}
	for _, l := range edeqsAnswersFor(t, 15)[1:] {
		say(runner, sender, 1, l)
	}

	d := st.Get(1)
	if d.Edeqs != nil {
		t.Error("completion must wipe the transient run")
	}
	if d.EdeqsResult == nil {
		t.Fatal("completion must store the result")
	}
	if raw := rawUsersJSON(t, backend); strings.Contains(raw, `"answers"`) {
		t.Errorf("raw answers must not survive the completion:\n%s", raw)
	}
}

// --- /report ----------------------------------------------------------------

// TestEat_ReportOneLeanLinePerInstrument: /report gains one line per
// completed instrument, NIAS printing its three subscales (it has no total).
func TestEat_ReportOneLeanLinePerInstrument(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleReport(sender, newMsg(1, "/report"))
	if !contains(sender.lastText(), "nothing") {
		t.Fatalf("a fresh user has nothing to report, got %q", sender.lastText())
	}

	driveEdeqs(t, runner, st, sender, 1, 15)
	driveBes(t, runner, st, sender, 1, 18)
	driveNias(t, runner, st, sender, 1, 10, 8, 9)

	runner.HandleReport(sender, newMsg(1, "/report"))
	report := sender.lastText()
	date := st.Get(1).EdeqsResult.TakenAt.Format("02.01.2006")
	for _, want := range []string{
		"EDEQS " + date + ": 15 of 36",
		"BES " + date + ": 18 of 46",
		"NIAS " + date + ": 10/8/9 of 15",
	} {
		if !contains(report, want) {
			t.Errorf("/report missing %q:\n%s", want, report)
		}
	}

	// The landing's report button renders exactly the same text.
	runner.HandleStart(sender, newMsg(1, "/start"))
	say(runner, sender, 1, "Report")
	if got := sender.lastText(); got != report {
		t.Errorf("the landing report button must render the same text:\n%s\n---\n%s", got, report)
	}
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("the report button must stay on the landing, got %q", got)
	}
}

// --- invalid input ----------------------------------------------------------

func TestEat_InvalidInputIsRejectedWithoutProgress(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleFood(sender, newMsg(1, "/food"))
	say(runner, sender, 1, "nonsense")
	if !contains(sender.lastText(), "Tap a screening button.") {
		t.Errorf("consent must reject free text, got %q", sender.lastText())
	}

	say(runner, sender, 1, "Got it")
	say(runner, sender, 1, "nonsense")
	if !contains(sender.lastText(), "Tap a screening button.") {
		t.Errorf("the menu must reject free text, got %q", sender.lastText())
	}

	say(runner, sender, 1, "Core test")
	say(runner, sender, 1, "S3") // the severity scale is not open yet
	if !contains(sender.lastText(), "Tap a scale button.") {
		t.Errorf("a foreign scale label must be rejected, got %q", sender.lastText())
	}
	if d := st.Get(1); len(d.Edeqs.Answers) != 0 {
		t.Errorf("a rejected answer must not be recorded: %+v", d.Edeqs)
	}
}
