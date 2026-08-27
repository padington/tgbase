package journey_test

// The eating track's canary, driven over the REAL bundled content
// (../../screening/*.yaml) and the REAL ru bundle (../../i18n): it walks
// every branch of the track and inspects the ASSEMBLED chat surface — every
// message body and every keyboard label the bot actually sends.
//
// internal/screening has its own canary over the raw content strings. This
// one covers what that cannot see: the strings this package composes
// (block headings, progress lines, numbered BES groups, result blocks, the
// doctor report) together with the i18n-owned landing buttons and /report
// lines. The product rules pinned here:
//
//   - no weight / height / calorie figure and no BMI anywhere in the chat;
//   - no disorder label anywhere a user can see — the doctor report says
//     "скрин по шкале X положительный" instead;
//   - no methodology hedging (translation status, validation notes).

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/state"
	"github.com/padington/tgbase/internal/store"
)

// setupEatReal builds a ru runner over the bundled eating content and the
// bundled i18n strings — only the landing and the eating track are wired,
// which is all this track needs.
func setupEatReal(t *testing.T) (*journey.Runner, *state.Store, *mockSender, *screening.EatingContent) {
	t.Helper()

	trans, err := i18n.Load(filepath.Join("..", "..", "i18n"), "ru")
	if err != nil {
		t.Fatalf("load bundled i18n: %v", err)
	}
	content, err := screening.LoadEating(filepath.Join("..", "..", "screening"))
	if err != nil {
		t.Fatalf("load bundled eating content: %v", err)
	}

	dir := t.TempDir()
	prodSeed := filepath.Join(dir, "products.yaml")
	if err := os.WriteFile(prodSeed, []byte(`- name: Apple
  fodmap: high
  measure: pieces
  stages: { low: 0.25, medium: 0.5, high: 1.0 }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	settingsSeed := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(settingsSeed, []byte(`defecation_reminder_after: 1m
checkin_interval: 30m
scan_interval: 10s
default_locale: ru
`), 0o600); err != nil {
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
	registerEatingPhases(runner, content)

	return runner, stateStore, sender, content
}

// --- content-driven drive helpers -------------------------------------------

// openEatMenuReal enters the track via /food, passing the consent when it is
// asked. All labels come from the content, so a content edit cannot silently
// desync the test.
func openEatMenuReal(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender,
	id int64, c *screening.EatingContent) {
	t.Helper()
	runner.HandleFood(sender, newMsg(id, "/food"))
	if st.Get(id).State == state.StateEatConsent {
		say(runner, sender, id, c.Module.Consent.AgreeButton)
	}
	if got := st.Get(id).State; got != state.StateEatMenu {
		t.Fatalf("expected the eating menu, got %q", got)
	}
}

// driveEdeqsReal answers all twelve items with option index level on
// whichever of the two scales the item uses.
func driveEdeqsReal(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender,
	id int64, c *screening.EatingContent, level int) {
	t.Helper()
	openEatMenuReal(t, runner, st, sender, id, c)
	say(runner, sender, id, c.Module.Menu.EdeqsButton)
	for i := range c.EDEQS.Items {
		say(runner, sender, id, c.EDEQS.ScaleFor(i)[level].Label)
	}
}

// driveBesReal picks statement number pick (1-based, clamped to the group)
// in every one of the sixteen groups.
func driveBesReal(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender,
	id int64, c *screening.EatingContent, pick int) {
	t.Helper()
	openEatMenuReal(t, runner, st, sender, id, c)
	say(runner, sender, id, c.Module.Menu.BesButton)
	for _, item := range c.BES.Items {
		n := pick
		if n > len(item.Statements) {
			n = len(item.Statements)
		}
		say(runner, sender, id, strconv.Itoa(n))
	}
}

// driveNiasReal answers all nine statements with option index level.
func driveNiasReal(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender,
	id int64, c *screening.EatingContent, level int) {
	t.Helper()
	openEatMenuReal(t, runner, st, sender, id, c)
	say(runner, sender, id, c.Module.Menu.NiasButton)
	for range c.NIAS.Items {
		say(runner, sender, id, c.NIAS.Scale[level].Label)
	}
}

// chatSurface returns everything the user can read: every message body plus
// every keyboard label, keyed by a short origin so a failure names the spot.
func chatSurface(sender *mockSender) map[string]string {
	out := map[string]string{}
	for i, c := range sender.snapshot() {
		m, ok := c.(tgbotapi.MessageConfig)
		if !ok {
			continue
		}
		out["message["+strconv.Itoa(i)+"]"] = m.Text
		kb, ok := m.ReplyMarkup.(tgbotapi.ReplyKeyboardMarkup)
		if !ok {
			continue
		}
		for r, row := range kb.Keyboard {
			for b, btn := range row {
				out["message["+strconv.Itoa(i)+"].button["+strconv.Itoa(r)+"."+strconv.Itoa(b)+"]"] = btn.Text
			}
		}
	}
	return out
}

// figureNeedle catches a number glued to a weight or calorie unit. The bare
// words are allowed — two of the instruments legitimately talk about
// calories in prose. What the track must never do is put a figure on screen.
var figureNeedle = regexp.MustCompile(`(?i)\d+\s*(кг|килограмм|ккал|калори|грамм)`)

// diagnosisNeedles are the disorder labels (and the body-metric index) that
// must never reach the chat, in any track text.
var diagnosisNeedles = []string{
	"анорекси", "булими", "орторекси", "арфид", "arfid",
	"компульсивн", "рпп", "расстройств", "binge eating disorder",
	"имт", "bmi", "индекс массы тела",
}

// hedgingNeedles are the methodology caveats the owner's lean-texts rule
// keeps out of every track.
var hedgingNeedles = []string{
	"неофициальн", "валидац", "валидир", "психометрическ", "нестандартизиров", "перевод",
}

// TestEatCanary_NoWeightFiguresOrDiagnosesInTheChat walks the whole track —
// both readings of every instrument, all three branches of the NIAS rule,
// the combined doctor report with and without the FODMAP context line,
// /report, /abandon and /food_delete — and scans the assembled chat surface.
func TestEatCanary_NoWeightFiguresOrDiagnosesInTheChat(t *testing.T) {
	runner, st, sender, c := setupEatReal(t)

	// User 1 — the low readings, and the NIAS taken BEFORE the core test so
	// its neutral (undecidable) wording is exercised, then re-taken after a
	// below-cutoff core test so the restrictive-without-body-image wording is
	// too.
	driveNiasReal(t, runner, st, sender, 1, c, len(c.NIAS.Scale)-1)
	driveEdeqsReal(t, runner, st, sender, 1, c, 0)
	driveNiasReal(t, runner, st, sender, 1, c, len(c.NIAS.Scale)-1)
	driveBesReal(t, runner, st, sender, 1, c, 1)
	runner.HandleReport(sender, newMsg(1, "/report"))

	// An interrupted run, closed with /abandon.
	openEatMenuReal(t, runner, st, sender, 1, c)
	say(runner, sender, 1, c.Module.Menu.EdeqsButton)
	say(runner, sender, 1, c.EDEQS.ScaleFor(0)[1].Label)
	runner.HandleAbandon(sender, newMsg(1, "/abandon"))

	// The delete dialog, both ways out.
	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
	say(runner, sender, 1, c.Module.UI.DeleteCancelButton)
	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete"))
	say(runner, sender, 1, c.Module.UI.DeleteConfirmButton)
	runner.HandleFoodDelete(sender, newMsg(1, "/food_delete")) // nothing left

	// User 2 — the high readings, a positive core test (so the NIAS reads as
	// body-image related), and diary activity so the automatic low-FODMAP
	// context line lands in the doctor report.
	d := st.Get(2)
	d.Locale = "ru"
	d.Products = map[string]state.ProductProgress{"Apple": {Status: "completed"}}
	st.Set(2, d)

	driveEdeqsReal(t, runner, st, sender, 2, c, len(c.EDEQS.ScaleDays)-1)
	driveNiasReal(t, runner, st, sender, 2, c, len(c.NIAS.Scale)-1)
	driveBesReal(t, runner, st, sender, 2, c, 4)
	runner.HandleStart(sender, newMsg(2, "/start"))

	surface := chatSurface(sender)

	// Positive controls: the walk really did reach the branches whose
	// wording is the risky part. Without these a broken drive would make the
	// scan below vacuously green.
	joined := strings.Join(mapValues(surface), "\n")
	for _, want := range []string{
		c.Module.Nias.Results.Contexts[screening.NiasCtxRestrictiveUnknown],
		c.Module.Nias.Results.Contexts[screening.NiasCtxRestrictiveNoBodyImage],
		c.Module.Nias.Results.Contexts[screening.NiasCtxRestrictiveBodyImage],
		c.Module.DoctorReport.Heading,
		c.Module.DoctorReport.FodmapContextLine,
		c.Module.Edeqs.Results.Bands[screening.EdeqsBandBelow],
		c.Module.Edeqs.Results.Bands[screening.EdeqsBandAtOrAbove],
		c.Module.Bes.Results.Bands[screening.BesBandLow],
		c.Module.Bes.Results.Bands[screening.BesBandSevere],
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the walk never reached %q — the scan below would be vacuous", want)
		}
	}

	for where, s := range surface {
		if m := figureNeedle.FindString(s); m != "" {
			t.Errorf("%s carries a weight/calorie figure (%q):\n%s", where, m, s)
		}
		low := strings.ToLower(s)
		for _, needle := range diagnosisNeedles {
			if strings.Contains(low, needle) {
				t.Errorf("%s carries a forbidden label (%q):\n%s", where, needle, s)
			}
		}
		for _, needle := range hedgingNeedles {
			if strings.Contains(low, needle) {
				t.Errorf("%s carries methodology hedging (%q):\n%s", where, needle, s)
			}
		}
	}
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// TestEatCanary_DoctorReportStaysScreeningWording pins the one place where
// naming a finding IS allowed and what it may look like: the doctor report
// speaks of a positive/negative SCREEN against a named scale, never of a
// condition.
func TestEatCanary_DoctorReportStaysScreeningWording(t *testing.T) {
	runner, st, sender, c := setupEatReal(t)

	driveEdeqsReal(t, runner, st, sender, 1, c, len(c.EDEQS.ScaleDays)-1)
	report := eatDoctorReportText(t, sender)

	if !strings.Contains(report, c.Module.DoctorReport.VerdictPositive) {
		t.Errorf("a maxed core test must read as a positive screen:\n%s", report)
	}
	if !strings.Contains(strings.ToLower(report), "скрин") {
		t.Errorf("the verdict must be worded as a screen:\n%s", report)
	}
	for _, needle := range diagnosisNeedles {
		if strings.Contains(strings.ToLower(report), needle) {
			t.Errorf("the doctor report must not name a condition (%q):\n%s", needle, report)
		}
	}
}
