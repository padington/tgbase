package journey_test

// Scenario tests of the pushup track, driven over the REAL bundled content
// (../../screening/pushups_ru.yaml) and the REAL ru i18n bundle. Every label
// the tests tap comes from the content, and every expected number is
// computed with the same pure generator the phases use — a content edit can
// change the program without desyncing the tests, but it can never make them
// pass by accident.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/state"
	"github.com/padington/tgbase/internal/store"
)

// --- harness ----------------------------------------------------------------

func setupPushups(t *testing.T) (*journey.Runner, *state.Store, *mockSender, *screening.PushupContent) {
	t.Helper()

	trans, err := i18n.Load(filepath.Join("..", "..", "i18n"), "ru")
	if err != nil {
		t.Fatalf("load bundled i18n: %v", err)
	}
	content, err := screening.LoadPushups(filepath.Join("..", "..", "screening"))
	if err != nil {
		t.Fatalf("load bundled pushup content: %v", err)
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
	runner.Register(journey.NewModeChoicePhaseWithPushups(content))
	registerPushupPhases(runner, content)

	return runner, stateStore, sender, content
}

func registerPushupPhases(runner *journey.Runner, c *screening.PushupContent) {
	runner.Register(journey.NewPuConsentPhase(c))
	runner.Register(journey.NewPuGatePhase(c))
	runner.Register(journey.NewPuGoalPhase(c))
	runner.Register(journey.NewPuVariationPhase(c))
	runner.Register(journey.NewPuMaxTestPhase(c))
	runner.Register(journey.NewPuMenuPhase(c))
	runner.Register(journey.NewPuSetPhase(c))
	runner.Register(journey.NewPuRestPhase(c))
	runner.Register(journey.NewPuEffortPhase(c))
	runner.Register(journey.NewPuWeekForkPhase(c))
	runner.Register(journey.NewPuRedCardPhase(c))
	runner.Register(journey.NewPuProgressPhase(c))
	runner.Register(journey.NewPuDeleteConfirmPhase(c))
}

// --- drive helpers ----------------------------------------------------------

// puOpenTrack walks the entry chain up to the first max test: consent, the
// three safety questions answered "no", the goal and the rung.
func puOpenTrack(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender,
	id int64, c *screening.PushupContent, variation string) {
	t.Helper()
	runner.HandlePushups(sender, newMsg(id, "/pushups"))
	say(runner, sender, id, c.Consent.AgreeButton)
	for range c.Gate.Questions {
		say(runner, sender, id, c.Gate.NoButton)
	}
	say(runner, sender, id, c.Goal.StrengthButton)
	say(runner, sender, id, c.Variation(variation).Name)
	if got := st.Get(id).State; got != state.StatePuTest {
		t.Fatalf("expected the max test, got %q", got)
	}
}

// puStart opens the track and passes the first test with reps.
func puStart(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender,
	id int64, c *screening.PushupContent, reps int) {
	t.Helper()
	puOpenTrack(t, runner, st, sender, id, c, "knees")
	say(runner, sender, id, strconv.Itoa(reps))
	if got := st.Get(id).State; got != state.StatePuMenu {
		t.Fatalf("expected the track menu after the test, got %q", got)
	}
}

// puRewind moves the program's clock-bound fields back, so the next session
// is allowed (the 24 h block is real time, and tests do not sleep).
func puRewind(st *state.Store, id int64, d time.Duration) {
	u := st.Get(id)
	if u.Pushups == nil {
		return
	}
	p := u.Pushups.Clone()
	if !p.LastSessionAt.IsZero() {
		p.LastSessionAt = p.LastSessionAt.Add(-d)
	}
	if !p.NextDueAt.IsZero() {
		p.NextDueAt = p.NextDueAt.Add(-d)
	}
	u.Pushups = p
	st.Set(id, u)
}

// puTrainSession runs one whole session from the menu: every fixed set is
// done exactly, the open set lands openDelta reps above its floor (negative
// = a miss), and the self-report answers with effort. Rests are skipped with
// «Готов раньше» — the timer itself is covered by its own test.
func puTrainSession(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender,
	id int64, c *screening.PushupContent, openDelta int, effort string) {
	t.Helper()
	puRewind(st, id, 50*time.Hour)
	runner.HandlePushups(sender, newMsg(id, "/pushups"))
	if got := st.Get(id).State; got != state.StatePuMenu {
		t.Fatalf("expected the track menu before training, got %q", got)
	}
	say(runner, sender, id, c.UI.TrainButton)
	if st.Get(id).State == state.StatePuRedCard {
		say(runner, sender, id, c.RedCard.NoButton)
	}

	for i := 0; i < 40; i++ {
		u := st.Get(id)
		switch u.State {
		case state.StatePuRest:
			say(runner, sender, id, c.Session.ReadyButton)
		case state.StatePuSet:
			s := u.PuSession
			idx := len(s.Actual)
			reps := s.Targets[idx]
			if idx == len(s.Targets)-1 {
				reps += openDelta
				if reps < 0 {
					reps = 0
				}
			}
			say(runner, sender, id, strconv.Itoa(reps))
		case state.StatePuEffort:
			say(runner, sender, id, effort)
			return
		default:
			t.Fatalf("unexpected state mid-session: %q", u.State)
		}
	}
	t.Fatalf("session did not finish, stuck in %q", st.Get(id).State)
}

// nthLastText returns the n-th message counted from the end (0 = last). The
// track often answers a tap with one message and then re-renders the state
// it moved to, so the interesting line is frequently the one before last.
func nthLastText(sender *mockSender, n int) string {
	texts := sentTexts(sender)
	if len(texts) <= n {
		return ""
	}
	return texts[len(texts)-1-n]
}

// puHomeLabel is the 🏠 label as THIS user's locale renders it (the test
// harness speaks the bundled i18n, the track speaks its own content).
func puHomeLabel(t *testing.T, st *state.Store, id int64) string {
	t.Helper()
	trans, err := i18n.Load(filepath.Join("..", "..", "i18n"), "ru")
	if err != nil {
		t.Fatalf("load bundled i18n: %v", err)
	}
	locale := i18n.Locale(st.Get(id).Locale)
	if locale == "" {
		locale = i18n.Locale("ru")
	}
	return trans.T("button.menu.home", locale, nil)
}

// renderPuResume mirrors the content's resume label, so the test taps the
// exact string the landing rendered.
func renderPuResume(c *screening.PushupContent, s *state.PushupSession) string {
	current := len(s.Actual) + 1
	if current > len(s.Targets) {
		current = len(s.Targets)
	}
	label := strings.ReplaceAll(c.UI.ResumeButton, "{current}", strconv.Itoa(current))
	return strings.ReplaceAll(label, "{total}", strconv.Itoa(len(s.Targets)))
}

// puProgram is the stored program (fails the test when absent).
func puProgram(t *testing.T, st *state.Store, id int64) *state.PushupProgram {
	t.Helper()
	p := st.Get(id).Pushups
	if p == nil {
		t.Fatal("expected a stored pushup program")
	}
	return p
}

// --- entry chain ------------------------------------------------------------

func TestPushups_EntryChainReachesTheFirstTest(t *testing.T) {
	runner, st, sender, c := setupPushups(t)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	if got := st.Get(1).State; got != state.StatePuConsent {
		t.Fatalf("expected the consent gate, got %q", got)
	}
	if st.Get(1).Pushups != nil {
		t.Error("nothing may be stored before consent")
	}

	say(runner, sender, 1, c.Consent.AgreeButton)
	if st.Get(1).PuConsentAt == nil {
		t.Error("consent timestamp not recorded")
	}
	if got := st.Get(1).State; got != state.StatePuGate {
		t.Fatalf("expected the safety gate, got %q", got)
	}
	for i := range c.Gate.Questions {
		if got := puProgram(t, st, 1).GateIdx; got != i {
			t.Fatalf("gate cursor is %d, want %d", got, i)
		}
		say(runner, sender, 1, c.Gate.NoButton)
	}
	if got := st.Get(1).State; got != state.StatePuGoal {
		t.Fatalf("expected the goal question, got %q", got)
	}

	say(runner, sender, 1, c.Goal.RepsButton)
	if got := puProgram(t, st, 1).Goal; got != screening.PushupGoalReps {
		t.Errorf("goal is %q", got)
	}
	if got := st.Get(1).State; got != state.StatePuVariation {
		t.Fatalf("expected the variation picker, got %q", got)
	}

	say(runner, sender, 1, c.Variation("classic").Name)
	if got := puProgram(t, st, 1).Variation; got != "classic" {
		t.Errorf("variation is %q", got)
	}
	if got := st.Get(1).State; got != state.StatePuTest {
		t.Fatalf("expected the max test, got %q", got)
	}
	if s := st.Get(1).PuSession; s == nil || s.Kind != state.PushupSessionTest {
		t.Fatalf("expected an open test session, got %+v", s)
	}
}

func TestPushups_ConsentDeclineStoresNothing(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.Consent.LaterButton)

	u := st.Get(1)
	if u.PuConsentAt != nil || u.Pushups != nil {
		t.Error("declining consent must leave no trace")
	}
	if u.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing, got %q", u.State)
	}
}

// --- safety gate ------------------------------------------------------------

func TestPushups_GateRedAnswerStopsTheTrack(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.Consent.AgreeButton)

	// Question 1 is the health stop-question, pinned by screening.Validate.
	say(runner, sender, 1, c.Gate.YesButton)

	if got := nthLastText(sender, 1); !strings.Contains(got, c.Gate.StopScreen) {
		t.Errorf("stop screen not shown, got %q", got)
	}
	u := st.Get(1)
	if u.Pushups != nil || u.PuConsentAt != nil || u.PuSession != nil {
		t.Error("a stopped track must store nothing at all")
	}
	if u.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing, got %q", u.State)
	}
}

func TestPushups_GateJointPainStartsLowerOnTheLadder(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.Consent.AgreeButton)
	say(runner, sender, 1, c.Gate.NoButton)  // health
	say(runner, sender, 1, c.Gate.YesButton) // joint pain
	say(runner, sender, 1, c.Gate.NoButton)  // pregnancy
	say(runner, sender, 1, c.Goal.StrengthButton)
	say(runner, sender, 1, c.Variation("classic").Name)

	want := c.ShiftVariation("classic", -c.Params.JointPainStepDown)
	if got := puProgram(t, st, 1).Variation; got != want {
		t.Errorf("variation is %q, want %q (%d rungs easier)", got, want, c.Params.JointPainStepDown)
	}
	if puProgram(t, st, 1).StartStepDown != 0 {
		t.Error("the step-down carry-over must be consumed by the pick")
	}
}

func TestPushups_GatePregnancyStartsOnTheEasiestRung(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.Consent.AgreeButton)
	say(runner, sender, 1, c.Gate.NoButton)
	say(runner, sender, 1, c.Gate.NoButton)
	say(runner, sender, 1, c.Gate.YesButton) // pregnancy
	say(runner, sender, 1, c.Goal.StrengthButton)
	say(runner, sender, 1, c.Variation("classic").Name)

	if got, want := puProgram(t, st, 1).Variation, c.Variations.Ladder[0].ID; got != want {
		t.Errorf("variation is %q, want the easiest rung %q", got, want)
	}
}

// --- the max test -----------------------------------------------------------

func TestPushups_TestSetsTheBaseAndTheMenuShowsThePlan(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 17)

	p := puProgram(t, st, 1)
	if p.Base != 17 {
		t.Errorf("base is %d, want 17", p.Base)
	}
	tst := st.Get(1).PuTest
	if tst == nil || tst.Reps != 17 || tst.Source != state.PushupTestInitial {
		t.Fatalf("test not recorded: %+v", tst)
	}
	if st.Get(1).PuSession != nil {
		t.Error("the test session must be closed by its answer")
	}

	plan := c.Plan(screening.PlanInput{Base: 17, DayIdx: 0})
	last := sender.lastText()
	for _, n := range plan.Targets[:len(plan.Targets)-1] {
		if !strings.Contains(last, strconv.Itoa(n)) {
			t.Errorf("menu does not show the generated plan %v: %q", plan.Targets, last)
		}
	}
	if !strings.Contains(last, strconv.Itoa(plan.OpenFloor())+"+") {
		t.Errorf("menu does not mark the open set: %q", last)
	}
}

func TestPushups_TestCapOffersAHarderVariation(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puOpenTrack(t, runner, st, sender, 1, c, "knees")
	say(runner, sender, 1, strconv.Itoa(c.Params.TestCap+40))

	if got := puProgram(t, st, 1).Base; got != c.Params.TestCap {
		t.Errorf("base is %d, want the cap %d", got, c.Params.TestCap)
	}
	if tst := st.Get(1).PuTest; tst == nil || !tst.Capped {
		t.Error("the capped flag must be recorded")
	}
	if got := nthLastText(sender, 1); !strings.Contains(got, "потолок") {
		t.Errorf("cap not announced: %q", got)
	}
}

func TestPushups_TooLowResultStepsDownAndRetestsAtOnce(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puOpenTrack(t, runner, st, sender, 1, c, "knees")
	say(runner, sender, 1, strconv.Itoa(c.Params.TooLowReps))

	if got, want := puProgram(t, st, 1).Variation, c.ShiftVariation("knees", -1); got != want {
		t.Errorf("variation is %q, want %q", got, want)
	}
	if got := st.Get(1).State; got != state.StatePuTest {
		t.Fatalf("expected an immediate retest, got %q", got)
	}
	if s := st.Get(1).PuSession; s == nil || s.Kind != state.PushupSessionTest {
		t.Fatalf("expected a fresh test session, got %+v", s)
	}
	if puProgram(t, st, 1).Base != 0 {
		t.Error("a too-low result must not become the base")
	}
}

func TestPushups_FreeNumberInputIsForgiving(t *testing.T) {
	cases := []struct {
		in       string
		wantBase int
		reask    bool
		stepDown bool
	}{
		{in: "17", wantBase: 17},
		{in: "12 повторов", wantBase: 12},
		{in: "~15", wantBase: 15},
		{in: "12-13", wantBase: 12},   // a range means the smaller end
		{in: "13 – 12", wantBase: 12}, // …whichever way it is written
		{in: "0", stepDown: true},     // a valid answer, but the rung is too hard
		{in: "дюжина", reask: true},
		{in: "", reask: true},
		{in: "9999", reask: true}, // out of range
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			runner, st, sender, c := setupPushups(t)
			puOpenTrack(t, runner, st, sender, 1, c, "knees")
			say(runner, sender, 1, tc.in)

			if tc.reask {
				if got := st.Get(1).State; got != state.StatePuTest {
					t.Fatalf("expected a re-ask in pu_test, got %q", got)
				}
				if got := sender.lastText(); got != c.Test.BadInput {
					t.Errorf("expected the re-ask line, got %q", got)
				}
				if puProgram(t, st, 1).Base != 0 {
					t.Error("an unreadable answer must not set a base")
				}
				return
			}
			if tc.stepDown {
				if got := sender.lastText(); got == c.Test.BadInput {
					t.Fatal("zero is a valid answer, not a parse error")
				}
				if got, want := puProgram(t, st, 1).Variation, c.ShiftVariation("knees", -1); got != want {
					t.Errorf("variation is %q, want the easier rung %q", got, want)
				}
				return
			}
			if got := puProgram(t, st, 1).Base; got != tc.wantBase {
				t.Errorf("base is %d, want %d", got, tc.wantBase)
			}
		})
	}
}

// --- one session ------------------------------------------------------------

func TestPushups_SessionWalksSetsRestsAndSelfReport(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	plan := c.Plan(screening.PlanInput{Base: 20, DayIdx: 0})

	say(runner, sender, 1, c.UI.TrainButton)
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("expected the first set, got %q", got)
	}
	s := st.Get(1).PuSession
	if s == nil || len(s.Targets) != len(plan.Targets) {
		t.Fatalf("session targets %+v do not match the generated plan %v", s, plan.Targets)
	}
	for i, n := range plan.Targets {
		if s.Targets[i] != n {
			t.Fatalf("session targets %v, want %v", s.Targets, plan.Targets)
		}
	}
	if s.RestSec != plan.RestSec {
		t.Errorf("rest is %d s, want %d s", s.RestSec, plan.RestSec)
	}

	// First set → one message, and it is the rest card with its two buttons.
	before := len(sender.snapshot())
	say(runner, sender, 1, strconv.Itoa(plan.Targets[0]))
	if got := len(sender.snapshot()) - before; got != 1 {
		t.Errorf("one set must cost exactly one message, got %d", got)
	}
	if got := st.Get(1).State; got != state.StatePuRest {
		t.Fatalf("expected the rest state, got %q", got)
	}
	if got := sender.lastText(); !strings.Contains(got, strconv.Itoa(plan.RestSec)) {
		t.Errorf("rest message does not carry the rest length: %q", got)
	}
	if !keyboardHas(lastKeyboard(sender), c.Session.ReadyButton) {
		t.Error("the rest card must offer «Готов раньше»")
	}
	if u := st.Get(1); u.PuSession.RestUntil.IsZero() {
		t.Error("the rest deadline must be recorded server-side")
	}

	// «Готов раньше» → straight to the next set, no waiting.
	say(runner, sender, 1, c.Session.ReadyButton)
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("expected the next set, got %q", got)
	}

	for st.Get(1).State != state.StatePuEffort {
		u := st.Get(1)
		if u.State == state.StatePuRest {
			say(runner, sender, 1, c.Session.ReadyButton)
			continue
		}
		idx := len(u.PuSession.Actual)
		say(runner, sender, 1, strconv.Itoa(u.PuSession.Targets[idx]))
	}
	if got := nthLastText(sender, 1); !strings.Contains(got, "Готово") {
		t.Errorf("expected the session summary, got %q", got)
	}

	say(runner, sender, 1, c.Session.EffortEasyButton)
	u := st.Get(1)
	if u.State != state.StateAwaitingModeChoice {
		t.Errorf("a finished session lands home, got %q", u.State)
	}
	if u.PuSession != nil {
		t.Error("the session must be wiped once it is recorded")
	}
	if len(u.PuHistory) != 1 {
		t.Fatalf("history has %d entries, want 1", len(u.PuHistory))
	}
	log := u.PuHistory[0]
	if log.Outcome != screening.PushupOutcomePlan {
		t.Errorf("outcome is %q, want %q", log.Outcome, screening.PushupOutcomePlan)
	}
	if log.Done != plan.PlannedTotal() {
		t.Errorf("done %d, want %d", log.Done, plan.PlannedTotal())
	}
	p := u.Pushups
	if p.SessionsDone != 1 || p.SessionInWeek != 1 || p.SessionsSinceTest != 1 {
		t.Errorf("counters after one session: %+v", p)
	}
	if p.TotalReps != plan.PlannedTotal() {
		t.Errorf("lifetime volume is %d, want %d", p.TotalReps, plan.PlannedTotal())
	}
	if p.NextDueAt.IsZero() || p.DuePingSent {
		t.Error("the next session must be scheduled and the ping armed")
	}
	if got := c.NextEffortAdj(0, screening.PushupEffortEasy); p.EffortAdj != got {
		t.Errorf("effort adjustment is %v, want %v", p.EffortAdj, got)
	}
}

func TestPushups_OpenSetOverThePlanReadsAsOver(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, c.Params.Progression.OverMargin, c.Session.EffortOKButton)

	h := st.Get(1).PuHistory
	if len(h) != 1 || h[0].Outcome != screening.PushupOutcomeOver {
		t.Fatalf("expected an over-plan session, got %+v", h)
	}
}

func TestPushups_MissedOpenSetStillCountsAsASession(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortHardButton)

	u := st.Get(1)
	if len(u.PuHistory) != 1 || u.PuHistory[0].Outcome != screening.PushupOutcomeShort {
		t.Fatalf("expected a short session in the history, got %+v", u.PuHistory)
	}
	if u.Pushups.SessionsDone != 1 || u.Pushups.SessionInWeek != 1 {
		t.Error("a short session must still count — the track never blocks on it")
	}
}

// --- rest timer -------------------------------------------------------------

func TestPushups_RestTimerPingsOnceWhenItExpires(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	say(runner, sender, 1, c.UI.TrainButton)
	say(runner, sender, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))
	if got := st.Get(1).State; got != state.StatePuRest {
		t.Fatalf("expected the rest state, got %q", got)
	}

	// Not yet: the deadline is in the future.
	before := len(sender.snapshot())
	runner.Remind()
	if got := len(sender.snapshot()); got != before {
		t.Fatalf("a running rest must be silent, got %d new messages", got-before)
	}

	u := st.Get(1)
	s := u.PuSession.Clone()
	s.RestUntil = time.Now().Add(-time.Second)
	u.PuSession = s
	st.Set(1, u)

	runner.Remind()
	if got := len(sender.snapshot()) - before; got != 1 {
		t.Fatalf("an expired rest must send exactly one message, got %d", got)
	}
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("expected the next set after the ping, got %q", got)
	}
	if got := sender.lastText(); !strings.Contains(got, strconv.Itoa(st.Get(1).PuSession.Targets[1])) {
		t.Errorf("the ping must name the next set: %q", got)
	}

	// And it does not repeat — the state moved on.
	before = len(sender.snapshot())
	runner.Remind()
	if got := len(sender.snapshot()); got != before {
		t.Error("the rest ping must not repeat")
	}
}

// --- pause, resume, escapes -------------------------------------------------

func TestPushups_PauseToLandingAndResumeMidSession(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	say(runner, sender, 1, c.UI.TrainButton)
	say(runner, sender, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))
	say(runner, sender, 1, c.Session.ReadyButton) // now on set 2

	runner.HandleStart(sender, newMsg(1, "/menu"))
	u := st.Get(1)
	if u.State != state.StateAwaitingModeChoice {
		t.Fatalf("expected the landing, got %q", u.State)
	}
	if u.PuSession == nil || u.PuSession.ResumeState != state.StatePuSet {
		t.Fatalf("the escape must record the position, got %+v", u.PuSession)
	}
	if len(u.PuSession.Actual) != 1 {
		t.Errorf("the recorded set must survive the pause: %+v", u.PuSession.Actual)
	}

	resume := renderPuResume(c, u.PuSession)
	if !keyboardHas(lastKeyboard(sender), resume) {
		t.Fatalf("the landing must offer %q, buttons: %v", resume, lastKeyboard(sender))
	}

	say(runner, sender, 1, resume)
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("expected to be back on the set, got %q", got)
	}
	if got := sender.lastText(); !strings.Contains(got, strconv.Itoa(st.Get(1).PuSession.Targets[1])) {
		t.Errorf("resume must re-render the current set: %q", got)
	}

	// And the session still finishes normally after the detour.
	for st.Get(1).State != state.StatePuEffort {
		u := st.Get(1)
		if u.State == state.StatePuRest {
			say(runner, sender, 1, c.Session.ReadyButton)
			continue
		}
		say(runner, sender, 1, strconv.Itoa(u.PuSession.Targets[len(u.PuSession.Actual)]))
	}
	say(runner, sender, 1, c.Session.EffortOKButton)
	if len(st.Get(1).PuHistory) != 1 {
		t.Error("the paused-and-resumed session must be recorded")
	}
}

func TestPushups_PauseButtonDuringRestLandsHomeAndResumesIntoTheRest(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	say(runner, sender, 1, c.UI.TrainButton)
	say(runner, sender, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))
	say(runner, sender, 1, c.Session.PauseButton)

	u := st.Get(1)
	if u.State != state.StateAwaitingModeChoice {
		t.Fatalf("expected the landing, got %q", u.State)
	}
	if u.PuSession.ResumeState != state.StatePuRest {
		t.Errorf("resume position is %q, want the rest", u.PuSession.ResumeState)
	}
	say(runner, sender, 1, renderPuResume(c, u.PuSession))
	if got := st.Get(1).State; got != state.StatePuRest {
		t.Fatalf("expected to be back in the rest, got %q", got)
	}
	// The rest card is re-rendered with the REMAINING seconds, not with a
	// fresh full rest.
	if got := sender.lastText(); !strings.Contains(got, "Отдых") {
		t.Errorf("expected the rest card, got %q", got)
	}
}

func TestPushups_EscapeMatrixKeepsTheProgram(t *testing.T) {
	// Every pu_* state must survive 🏠 / /start / /menu: the landing takes
	// over, the program is intact, and only positions inside a session are
	// recorded for the resume row.
	steps := []struct {
		name  string
		drive func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, c *screening.PushupContent)
		want  state.StateKind
	}{
		{"consent", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			r.HandlePushups(s, newMsg(1, "/pushups"))
		}, state.StatePuConsent},
		{"gate", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			r.HandlePushups(s, newMsg(1, "/pushups"))
			say(r, s, 1, c.Consent.AgreeButton)
		}, state.StatePuGate},
		{"goal", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			r.HandlePushups(s, newMsg(1, "/pushups"))
			say(r, s, 1, c.Consent.AgreeButton)
			for range c.Gate.Questions {
				say(r, s, 1, c.Gate.NoButton)
			}
		}, state.StatePuGoal},
		{"variation", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			r.HandlePushups(s, newMsg(1, "/pushups"))
			say(r, s, 1, c.Consent.AgreeButton)
			for range c.Gate.Questions {
				say(r, s, 1, c.Gate.NoButton)
			}
			say(r, s, 1, c.Goal.RepsButton)
		}, state.StatePuVariation},
		{"test", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			puOpenTrack(t, r, st, s, 1, c, "knees")
		}, state.StatePuTest},
		{"menu", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			puStart(t, r, st, s, 1, c, 20)
		}, state.StatePuMenu},
		{"set", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			puStart(t, r, st, s, 1, c, 20)
			say(r, s, 1, c.UI.TrainButton)
		}, state.StatePuSet},
		{"rest", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			puStart(t, r, st, s, 1, c, 20)
			say(r, s, 1, c.UI.TrainButton)
			say(r, s, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))
		}, state.StatePuRest},
		{"effort", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			puStart(t, r, st, s, 1, c, 20)
			say(r, s, 1, c.UI.TrainButton)
			for st.Get(1).State != state.StatePuEffort {
				u := st.Get(1)
				if u.State == state.StatePuRest {
					say(r, s, 1, c.Session.ReadyButton)
					continue
				}
				say(r, s, 1, strconv.Itoa(u.PuSession.Targets[len(u.PuSession.Actual)]))
			}
		}, state.StatePuEffort},
		{"progress", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			puStart(t, r, st, s, 1, c, 20)
			say(r, s, 1, c.UI.ProgressButton)
		}, state.StatePuProgress},
		{"delete_confirm", func(t *testing.T, r *journey.Runner, st *state.Store, s *mockSender, c *screening.PushupContent) {
			puStart(t, r, st, s, 1, c, 20)
			r.HandlePushupsDelete(s, newMsg(1, "/pushups_delete"))
		}, state.StatePuDeleteConfirm},
	}

	for _, step := range steps {
		for _, escape := range []string{"/start", "/menu", "home"} {
			t.Run(step.name+"/"+escape, func(t *testing.T) {
				runner, st, sender, c := setupPushups(t)
				step.drive(t, runner, st, sender, c)
				if got := st.Get(1).State; got != step.want {
					t.Fatalf("setup landed in %q, want %q", got, step.want)
				}
				before := st.Get(1)

				if escape == "home" {
					say(runner, sender, 1, puHomeLabel(t, st, 1))
				} else {
					runner.HandleStart(sender, newMsg(1, escape))
				}

				after := st.Get(1)
				if after.State != state.StateAwaitingModeChoice {
					t.Fatalf("escape from %q did not land on the landing: %q", step.name, after.State)
				}
				if (before.Pushups == nil) != (after.Pushups == nil) {
					t.Fatal("the program must survive every escape")
				}
				if before.Pushups != nil && after.Pushups.Base != before.Pushups.Base {
					t.Errorf("base changed on escape: %d → %d", before.Pushups.Base, after.Pushups.Base)
				}
				if before.PuSession != nil && after.PuSession == nil {
					t.Fatal("an open session must survive the escape")
				}
				if after.PuSession != nil && before.PuSession != nil &&
					len(after.PuSession.Actual) != len(before.PuSession.Actual) {
					t.Error("recorded sets must survive the escape")
				}
				if resumablePuStateForTest(step.want) && after.PuSession != nil &&
					after.PuSession.ResumeState != step.want {
					t.Errorf("resume position is %q, want %q", after.PuSession.ResumeState, step.want)
				}
			})
		}
	}
}

// resumablePuStateForTest mirrors the package's own resumable set — the test
// asserts the contract, so it spells it out instead of importing it.
func resumablePuStateForTest(kind state.StateKind) bool {
	return kind == state.StatePuSet || kind == state.StatePuRest || kind == state.StatePuEffort
}

// --- recovery window --------------------------------------------------------

func TestPushups_SecondSessionBlockedWithin24Hours(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)

	// Two hours later: hard block, and «всё равно» cannot buy through it.
	puRewind(st, 1, 2*time.Hour)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.TrainButton)
	if got := sender.lastText(); got != c.Session.RestDay {
		t.Errorf("expected the rest-day line, got %q", got)
	}
	if got := st.Get(1).State; got != state.StatePuMenu {
		t.Errorf("a blocked session must leave the user on the menu, got %q", got)
	}
	if st.Get(1).PuSession != nil {
		t.Fatal("no session may be created inside the recovery window")
	}
	say(runner, sender, 1, c.Session.TrainAnywayButton)
	if st.Get(1).PuSession != nil {
		t.Fatal("«всё равно» must not override the hard 24 h block")
	}

	// Thirty hours in: a warning, and the override works.
	puRewind(st, 1, 28*time.Hour)
	say(runner, sender, 1, c.UI.TrainButton)
	if got := sender.lastText(); !strings.Contains(got, "прошло") {
		t.Errorf("expected the soft warning, got %q", got)
	}
	if !keyboardHas(lastKeyboard(sender), c.Session.TrainAnywayButton) {
		t.Error("the warning must offer the override button")
	}
	say(runner, sender, 1, c.Session.TrainAnywayButton)
	if st.Get(1).State == state.StatePuRedCard {
		say(runner, sender, 1, c.RedCard.NoButton)
	}
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("the override must start the session, got %q", got)
	}
}

// --- the weekly rule --------------------------------------------------------

func TestPushups_TwoShortSessionsRepeatTheWeekOutLoud(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	base := puProgram(t, st, 1).Base

	puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortHardButton)
	puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortHardButton)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)

	closing := nthLastText(sender, 1)
	if !strings.Contains(closing, "Повторяем неделю") {
		t.Errorf("the repeat must be announced out loud, got %q", closing)
	}
	p := puProgram(t, st, 1)
	if p.Base >= base {
		t.Errorf("a repeated week must lower the base: %d → %d", base, p.Base)
	}
	if p.RepeatCount != 1 || p.StreakWeeks != 0 {
		t.Errorf("repeat bookkeeping: repeats=%d streak=%d", p.RepeatCount, p.StreakWeeks)
	}
	if p.SessionInWeek != 0 || len(p.WeekOutcomes) != 0 {
		t.Errorf("the week must reset: %+v", p)
	}
}

func TestPushups_OneShortSessionDoesNotRepeatTheWeek(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	base := puProgram(t, st, 1).Base

	puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortOKButton)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)

	p := puProgram(t, st, 1)
	if p.Base != base {
		t.Errorf("one bad session must leave the base alone: %d → %d", base, p.Base)
	}
	if p.RepeatCount != 0 || p.StreakWeeks != 1 {
		t.Errorf("hold bookkeeping: repeats=%d streak=%d", p.RepeatCount, p.StreakWeeks)
	}
}

func TestPushups_CleanWeekRaisesTheBase(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	base := puProgram(t, st, 1).Base

	for i := 0; i < c.Params.SessionsPerWeek; i++ {
		puTrainSession(t, runner, st, sender, 1, c, c.Params.Progression.OverMargin, c.Session.EffortOKButton)
	}
	want := c.ProgressWeek(base, []string{
		screening.PushupOutcomeOver, screening.PushupOutcomeOver, screening.PushupOutcomeOver,
	})
	p := puProgram(t, st, 1)
	if p.Base != want.NewBase {
		t.Errorf("base is %d, want %d", p.Base, want.NewBase)
	}
	if p.WeekIdx != 1 || p.StreakWeeks != 1 {
		t.Errorf("week bookkeeping: %+v", p)
	}
}

func TestPushups_ThirdRepeatOffersTheFork(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)

	for week := 0; week < c.Params.Progression.RepeatForkAfter; week++ {
		for i := 0; i < c.Params.SessionsPerWeek; i++ {
			puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortHardButton)
		}
	}
	if got := st.Get(1).State; got != state.StatePuWeekFork {
		t.Fatalf("expected the fork after %d repeats, got %q",
			c.Params.Progression.RepeatForkAfter, got)
	}
	if !keyboardHas(lastKeyboard(sender), c.Week.ForkRestButton) {
		t.Fatalf("the fork must offer both levers, buttons: %v", lastKeyboard(sender))
	}

	say(runner, sender, 1, c.Week.ForkRestButton)
	p := puProgram(t, st, 1)
	if p.RestBonusSec != c.Params.RestBonusStep {
		t.Errorf("rest bonus is %d s, want %d s", p.RestBonusSec, c.Params.RestBonusStep)
	}
	if p.RepeatCount != 0 {
		t.Error("the fork must clear the repeat counter")
	}
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing after the fork, got %q", got)
	}
}

// --- retest -----------------------------------------------------------------

func TestPushups_RetestCountsAsTheWeeksSession(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puRewind(st, 1, 50*time.Hour)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.RetestButton)
	if got := st.Get(1).State; got != state.StatePuTest {
		t.Fatalf("expected the retest, got %q", got)
	}
	if s := st.Get(1).PuSession; s == nil || s.Kind != state.PushupSessionRetest {
		t.Fatalf("expected a retest session, got %+v", s)
	}

	before := puProgram(t, st, 1)
	say(runner, sender, 1, "26")

	p := puProgram(t, st, 1)
	if p.Base != 26 {
		t.Errorf("base is %d, want the retest result 26", p.Base)
	}
	if p.SessionsDone != before.SessionsDone+1 {
		t.Error("a retest counts as a session")
	}
	if p.SessionsSinceTest != 0 {
		t.Error("the retest counter must reset")
	}
	if p.WeekIdx != before.WeekIdx+1 || p.SessionInWeek != 0 {
		t.Errorf("a retest closes the week: %+v", p)
	}
	if p.LastSessionAt.IsZero() {
		t.Error("a retest must arm the recovery window like any session")
	}
	if tst := st.Get(1).PuTest; tst == nil || tst.Source != state.PushupTestRetest {
		t.Fatalf("retest not recorded: %+v", tst)
	}
	if len(st.Get(1).PuHistory) != 1 {
		t.Error("the retest belongs in the history — it is the session of the day")
	}
}

// TestPushups_RetestCannotBuyThroughTheRecoveryBlock: «Перетест» is a full
// session — it moves the base, counts as the day's work, closes the week and
// writes history — so it goes through the same hard 24 h block as a workout.
// It used to walk straight past it, which also let a tap a minute inflate the
// week counter and the streak.
func TestPushups_RetestCannotBuyThroughTheRecoveryBlock(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)
	before := *puProgram(t, st, 1)

	// Two hours after a session: the same rest-day line the train button gets.
	puRewind(st, 1, 2*time.Hour)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.RetestButton)

	if got := sender.lastText(); got != c.Session.RestDay {
		t.Errorf("expected the rest-day line, got %q", got)
	}
	if got := st.Get(1).State; got != state.StatePuMenu {
		t.Errorf("a blocked retest must leave the user on the menu, got %q", got)
	}
	if s := st.Get(1).PuSession; s != nil {
		t.Fatalf("no test session may open inside the recovery window: %+v", s)
	}
	after := puProgram(t, st, 1)
	if after.SessionsDone != before.SessionsDone || after.WeekIdx != before.WeekIdx ||
		after.StreakWeeks != before.StreakWeeks || after.TotalReps != before.TotalReps {
		t.Errorf("a blocked retest changed the program: %+v → %+v", before, *after)
	}

	// Past the window the button works again.
	puRewind(st, 1, 48*time.Hour)
	say(runner, sender, 1, c.UI.RetestButton)
	if got := st.Get(1).State; got != state.StatePuTest {
		t.Fatalf("outside the recovery window the retest must open, got %q", got)
	}
}

func TestPushups_RetestBecomesDueAfterTheCycle(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)

	u := st.Get(1)
	p := u.Pushups.Clone()
	p.SessionsSinceTest = c.Params.RetestEverySessions
	p.SessionsDone = c.Params.RetestEverySessions
	u.Pushups = p
	st.Set(1, u)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	if got := sender.lastText(); !strings.Contains(got, c.Week.RetestDue) {
		t.Errorf("the menu must announce the due retest, got %q", got)
	}
	say(runner, sender, 1, c.UI.TrainButton)
	if got := st.Get(1).State; got != state.StatePuTest {
		t.Fatalf("a due retest replaces the workout, got %q", got)
	}
}

// --- housekeeping -----------------------------------------------------------

func TestPushups_StaleSessionClosesAsPartial(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	say(runner, sender, 1, c.UI.TrainButton)
	say(runner, sender, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))

	// A day passes with the session open.
	u := st.Get(1)
	s := u.PuSession.Clone()
	s.StartedAt = time.Now().Add(-time.Duration(c.Params.SessionTTLHours+1) * time.Hour)
	u.PuSession = s
	u.State = state.StatePuMenu
	st.Set(1, u)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	if got := sender.lastText(); !strings.Contains(got, c.Session.Expired) {
		t.Errorf("the expired session must be announced, got %q", got)
	}
	after := st.Get(1)
	if after.PuSession != nil {
		t.Fatal("a stale session must be closed")
	}
	if len(after.PuHistory) != 1 || after.PuHistory[0].Outcome != screening.PushupOutcomeShort {
		t.Fatalf("a partial session is recorded by what was done: %+v", after.PuHistory)
	}
	if after.Pushups.SessionsDone != 1 {
		t.Error("a partial session still counts as done")
	}
}

// puStaleSession makes the user's open session outlive its TTL and parks the
// user on the track menu, which is where housekeeping runs.
func puStaleSession(st *state.Store, id int64, c *screening.PushupContent) {
	u := st.Get(id)
	s := u.PuSession.Clone()
	s.StartedAt = time.Now().Add(-time.Duration(c.Params.SessionTTLHours+1) * time.Hour)
	u.PuSession = s
	u.State = state.StatePuMenu
	st.Set(id, u)
}

// TestPushups_StaleSessionThatClosesTheWeekAnnouncesIt: a session dying of
// old age can be the week's third one, so the expiry is the second place the
// week can close — and the verdict is spoken out loud there too. It used to
// close in silence: the base moved down and the only line on screen was
// "прошлая тренировка осталась незаконченной".
func TestPushups_StaleSessionThatClosesTheWeekAnnouncesIt(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)

	// Two short sessions, then a third left open until it goes stale.
	puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortHardButton)
	puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortHardButton)
	base := puProgram(t, st, 1).Base

	puRewind(st, 1, 50*time.Hour)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.TrainButton)
	if st.Get(1).State == state.StatePuRedCard {
		say(runner, sender, 1, c.RedCard.NoButton)
	}
	say(runner, sender, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))
	puStaleSession(st, 1, c)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	text := sender.lastText()
	if !strings.Contains(text, c.Session.Expired) {
		t.Errorf("the expired session must still be announced, got %q", text)
	}
	if !strings.Contains(text, "Повторяем неделю") {
		t.Errorf("a week closed by an expiry must announce its verdict too, got %q", text)
	}
	p := puProgram(t, st, 1)
	if p.Base >= base {
		t.Errorf("the repeated week lowered nothing: %d → %d", base, p.Base)
	}
	for _, n := range []int{base, p.Base} {
		if !strings.Contains(text, strconv.Itoa(n)) {
			t.Errorf("the announcement must carry both bases (%d → %d), got %q", base, p.Base, text)
		}
	}
	if p.RepeatCount != 1 || p.SessionInWeek != 0 || len(p.WeekOutcomes) != 0 {
		t.Errorf("week bookkeeping after an expiry: %+v", *p)
	}
}

// TestPushups_StaleSessionOnTheThirdRepeatOffersTheFork: the same path owes
// the user the fork. Three repeated weeks in a row is the program admitting
// it does not fit; arriving there through an expiry is no reason to hand out
// a fourth identical week instead.
func TestPushups_StaleSessionOnTheThirdRepeatOffersTheFork(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)

	// Two repeated weeks already behind, two short sessions into the third.
	u := st.Get(1)
	p := u.Pushups.Clone()
	p.RepeatCount = c.Params.Progression.RepeatForkAfter - 1
	p.WeekOutcomes = []string{screening.PushupOutcomeShort, screening.PushupOutcomeShort}
	p.SessionInWeek = c.Params.SessionsPerWeek - 1
	p.SessionsDone = 2
	u.Pushups = p
	st.Set(1, u)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.TrainButton)
	if st.Get(1).State == state.StatePuRedCard {
		say(runner, sender, 1, c.RedCard.NoButton)
	}
	say(runner, sender, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))
	puStaleSession(st, 1, c)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	if got := st.Get(1).State; got != state.StatePuWeekFork {
		t.Fatalf("expected the fork after the third repeat, got %q", got)
	}
	if !keyboardHas(lastKeyboard(sender), c.Week.ForkEasierButton) ||
		!keyboardHas(lastKeyboard(sender), c.Week.ForkRestButton) {
		t.Errorf("the fork must offer both levers, buttons: %v", lastKeyboard(sender))
	}
	if closing := nthLastText(sender, 1); !strings.Contains(closing, "Повторяем неделю") {
		t.Errorf("the repeat that led to the fork must be announced, got %q", closing)
	}
	say(runner, sender, 1, c.Week.ForkRestButton)
	if got := puProgram(t, st, 1).RepeatCount; got != 0 {
		t.Errorf("the fork must clear the repeat counter, got %d", got)
	}
}

func TestPushups_AbandonDropsTheSessionNotTheProgram(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	say(runner, sender, 1, c.UI.TrainButton)
	say(runner, sender, 1, strconv.Itoa(st.Get(1).PuSession.Targets[0]))

	runner.HandleAbandon(sender, newMsg(1, "/abandon"))
	u := st.Get(1)
	if u.PuSession != nil {
		t.Error("/abandon drops the session")
	}
	if u.Pushups == nil || u.Pushups.Base != 20 {
		t.Error("/abandon must keep the program")
	}
	if u.State != state.StateAwaitingModeChoice {
		t.Errorf("expected the landing, got %q", u.State)
	}
	if len(u.PuHistory) != 0 {
		t.Error("an abandoned session is not a session")
	}
}

func TestPushups_DeleteWipesTheWholeTrack(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)

	runner.HandlePushupsDelete(sender, newMsg(1, "/pushups_delete"))
	if got := st.Get(1).State; got != state.StatePuDeleteConfirm {
		t.Fatalf("expected the confirmation, got %q", got)
	}
	say(runner, sender, 1, c.UI.DeleteCancelButton)
	if st.Get(1).Pushups == nil {
		t.Fatal("cancelling must keep everything")
	}

	runner.HandlePushupsDelete(sender, newMsg(1, "/pushups_delete"))
	say(runner, sender, 1, c.UI.DeleteConfirmButton)
	u := st.Get(1)
	if u.Pushups != nil || u.PuSession != nil || u.PuTest != nil ||
		len(u.PuHistory) != 0 || u.PuConsentAt != nil {
		t.Errorf("the whole track must be gone: %+v", u)
	}

	// And the track starts from consent again.
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	if got := st.Get(1).State; got != state.StatePuConsent {
		t.Errorf("expected the consent gate again, got %q", got)
	}
}

func TestPushups_DeleteFromInsideASessionLandsHome(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	say(runner, sender, 1, c.UI.TrainButton)

	runner.HandlePushupsDelete(sender, newMsg(1, "/pushups_delete"))
	say(runner, sender, 1, c.UI.DeleteCancelButton)
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("cancelling must return into the session, got %q", got)
	}

	runner.HandlePushupsDelete(sender, newMsg(1, "/pushups_delete"))
	say(runner, sender, 1, c.UI.DeleteConfirmButton)
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("confirming inside a session lands home, got %q", got)
	}
	if st.Get(1).ReturnState != "" {
		t.Error("the recorded way back must be consumed")
	}
}

func TestPushups_DeleteWithNothingStoredSaysSo(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	runner.HandlePushupsDelete(sender, newMsg(1, "/pushups_delete"))
	if got := sender.lastText(); got != c.UI.DeleteNothing {
		t.Errorf("expected the nothing-to-delete line, got %q", got)
	}
	if got := st.Get(1).State; got == state.StatePuDeleteConfirm {
		t.Error("no confirmation dialog without data")
	}
}

// --- due ping ---------------------------------------------------------------

func TestPushups_DuePingOnlyReachesQuietStates(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)
	puRewind(st, 1, 50*time.Hour) // the next session is due

	// Mid-set: the ping must not interrupt.
	u := st.Get(1)
	u.State = state.StatePuSet
	st.Set(1, u)
	before := len(sender.snapshot())
	runner.Remind()
	if got := len(sender.snapshot()); got != before {
		t.Fatalf("the due ping must not reach a user mid-session, got %d new", got-before)
	}

	// Mid another track's test: same rule.
	u = st.Get(1)
	u.State = state.StateEatEdeqsQuestion
	st.Set(1, u)
	runner.Remind()
	if got := len(sender.snapshot()); got != before {
		t.Fatalf("the due ping must not reach a user mid-self-check, got %d new", got-before)
	}

	// On the landing: one message.
	u = st.Get(1)
	u.State = state.StateAwaitingModeChoice
	st.Set(1, u)
	runner.Remind()
	if got := len(sender.snapshot()) - before; got != 1 {
		t.Fatalf("expected exactly one due ping, got %d", got)
	}
	if got := sender.lastText(); !strings.Contains(got, "тренироваться") {
		t.Errorf("unexpected ping text: %q", got)
	}

	// It does not repeat on the next tick.
	before = len(sender.snapshot())
	runner.Remind()
	if got := len(sender.snapshot()); got != before {
		t.Fatal("the due ping must be sent at most once per due date")
	}

	// Four days later the last message of the cycle arrives — and then the
	// track goes quiet for good.
	puRewind(st, 1, 5*24*time.Hour)
	runner.Remind()
	if got := len(sender.snapshot()) - before; got != 1 {
		t.Fatalf("expected the overdue message, got %d", got)
	}
	if !st.Get(1).Pushups.DuePingSent {
		t.Error("the overdue message must latch the ping off")
	}
	before = len(sender.snapshot())
	puRewind(st, 1, 10*24*time.Hour)
	runner.Remind()
	if got := len(sender.snapshot()); got != before {
		t.Error("no third message in one cycle")
	}
}

// --- progress and report ----------------------------------------------------

func TestPushups_ProgressScreenAndReportLine(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, 1, c.Session.EffortOKButton)
	puTrainSession(t, runner, st, sender, 1, c, 3, c.Session.EffortOKButton)

	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.ProgressButton)
	screen := sender.lastText()
	if lines := strings.Count(screen, "\n") + 1; lines > 8 {
		t.Errorf("the progress screen must stay under 8 lines, got %d:\n%s", lines, screen)
	}
	for _, want := range []string{"База", "Неделя", "Последний тест", "Открытые подходы", "Всего повторов"} {
		if !strings.Contains(screen, want) {
			t.Errorf("progress screen misses %q:\n%s", want, screen)
		}
	}
	if !strings.ContainsAny(screen, "▁▂▃▄▅▆▇█") {
		t.Errorf("expected a text sparkline:\n%s", screen)
	}

	runner.HandleReport(sender, newMsg(1, "/report"))
	line := sender.lastText()
	if !strings.Contains(line, "Отжимания") || !strings.Contains(line, "база") {
		t.Errorf("/report misses the track line: %q", line)
	}
	if !strings.Contains(line, strconv.Itoa(st.Get(1).Pushups.TotalReps)) {
		t.Errorf("/report must carry the lifetime volume: %q", line)
	}
}

// TestPushups_ProgressScreenHonoursTheOverrideItOffers: the progress screen
// has a train button, so the soft "less than 48 h" warning can be rendered
// while the user stands on it — and puTrain answers that warning without
// moving the state. The override button therefore has to be handled here as
// well; it used to fall into the read-only default and drop the user back on
// the menu with no session started.
func TestPushups_ProgressScreenHonoursTheOverrideItOffers(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)

	// Thirty hours later: inside the advised window, outside the hard block.
	puRewind(st, 1, 30*time.Hour)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.ProgressButton)
	if got := st.Get(1).State; got != state.StatePuProgress {
		t.Fatalf("expected the progress screen, got %q", got)
	}

	say(runner, sender, 1, c.UI.TrainButton)
	if got := sender.lastText(); !strings.Contains(got, "прошло") {
		t.Errorf("expected the soft warning on the progress screen, got %q", got)
	}
	if !keyboardHas(lastKeyboard(sender), c.Session.TrainAnywayButton) {
		t.Fatalf("the warning must offer the override, buttons: %v", lastKeyboard(sender))
	}
	if got := st.Get(1).State; got != state.StatePuProgress {
		t.Fatalf("the warning must not move the user off the screen, got %q", got)
	}

	say(runner, sender, 1, c.Session.TrainAnywayButton)
	if st.Get(1).State == state.StatePuRedCard {
		say(runner, sender, 1, c.RedCard.NoButton)
	}
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("the override must start the session, got %q", got)
	}
	if st.Get(1).PuSession == nil {
		t.Error("the override started no session")
	}
}

// --- red card ---------------------------------------------------------------

func TestPushups_RedCardPausesTheTrack(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	for i := 0; i < c.Params.SessionsPerWeek; i++ {
		puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)
	}
	// A new week starts with the red-flag check-in.
	puRewind(st, 1, 50*time.Hour)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.TrainButton)
	if got := st.Get(1).State; got != state.StatePuRedCard {
		t.Fatalf("expected the red-flag check-in, got %q", got)
	}

	say(runner, sender, 1, c.RedCard.YesButton)
	u := st.Get(1)
	if u.State != state.StateAwaitingModeChoice {
		t.Errorf("a red flag stops the session, got %q", u.State)
	}
	if u.PuSession != nil {
		t.Error("no session may be running after a red flag")
	}
	if !u.Pushups.NextDueAt.IsZero() {
		t.Error("a paused track must not ping")
	}
	if u.Pushups == nil || u.Pushups.Base <= 0 || len(u.PuHistory) == 0 {
		t.Error("the program and its history are kept — the track is paused, not deleted")
	}
	before := len(sender.snapshot())
	runner.Remind()
	if got := len(sender.snapshot()); got != before {
		t.Error("a paused track must stay silent")
	}
}

func TestPushups_RedCardJointPainStepsDownAndTrains(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	for i := 0; i < c.Params.SessionsPerWeek; i++ {
		puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)
	}
	puRewind(st, 1, 50*time.Hour)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.TrainButton)

	before := puProgram(t, st, 1)
	say(runner, sender, 1, c.RedCard.JointPainPrompt)
	after := puProgram(t, st, 1)
	if want := c.ShiftVariation(before.Variation, -1); after.Variation != want {
		t.Errorf("variation is %q, want one rung easier (%q)", after.Variation, want)
	}
	if after.Base != before.Base {
		t.Error("pain changes the leverage, not the base")
	}
	if got := st.Get(1).State; got != state.StatePuSet {
		t.Fatalf("the session still starts, got %q", got)
	}
}
