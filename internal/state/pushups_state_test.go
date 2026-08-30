package state

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fullPushupProgram() *PushupProgram {
	return &PushupProgram{
		Variation:         "classic",
		Goal:              "reps",
		Base:              19,
		WeekIdx:           3,
		SessionInWeek:     2,
		WeekOutcomes:      []string{"plan", "over"},
		RepeatCount:       1,
		EffortAdj:         1.1,
		RestBonusSec:      30,
		SessionsDone:      11,
		SessionsSinceTest: 11,
		TotalReps:         1640,
		LastSessionAt:     time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC),
		NextDueAt:         time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC),
		StreakWeeks:       3,
		StartedAt:         time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC),
		GateIdx:           3,
		StartStepDown:     1,
		RetestPending:     true,
	}
}

func fullPushupSession() *PushupSession {
	return &PushupSession{
		Kind:        PushupSessionWorkout,
		DayIdx:      1,
		Targets:     []int{8, 9, 7, 7, 11},
		OpenFloor:   11,
		Actual:      []int{8, 9},
		Idx:         2,
		RestSec:     90,
		RestUntil:   time.Date(2026, 8, 30, 19, 5, 0, 0, time.UTC),
		StartedAt:   time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC),
		ResumeState: StatePuRest,
	}
}

func fullPushupLog() PushupSessionLog {
	return PushupSessionLog{
		Date:       time.Date(2026, 8, 28, 19, 30, 0, 0, time.UTC),
		DayIdx:     2,
		Sets:       []int{9, 10, 8, 8, 13},
		Planned:    48,
		Done:       48,
		OpenTarget: 13,
		OpenActual: 13,
		Outcome:    "plan",
		Effort:     "ok",
		Base:       19,
	}
}

func TestUserData_PushupJSONRoundTrip(t *testing.T) {
	consent := time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)
	in := UserData{
		State:       StatePuRest,
		PuConsentAt: &consent,
		Pushups:     fullPushupProgram(),
		PuSession:   fullPushupSession(),
		PuTest: &PushupTest{
			TakenAt:   time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC),
			Variation: "classic",
			Reps:      17,
			Source:    PushupTestInitial,
		},
		PuHistory: []PushupSessionLog{fullPushupLog()},
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out UserData
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if out.PuConsentAt == nil || !out.PuConsentAt.Equal(consent) {
		t.Errorf("PuConsentAt lost: %v", out.PuConsentAt)
	}
	if !reflect.DeepEqual(out.Pushups, in.Pushups) {
		t.Errorf("program lost:\n got %+v\nwant %+v", out.Pushups, in.Pushups)
	}
	if !reflect.DeepEqual(out.PuSession, in.PuSession) {
		t.Errorf("session lost:\n got %+v\nwant %+v", out.PuSession, in.PuSession)
	}
	if !reflect.DeepEqual(out.PuTest, in.PuTest) {
		t.Errorf("test lost:\n got %+v\nwant %+v", out.PuTest, in.PuTest)
	}
	if !reflect.DeepEqual(out.PuHistory, in.PuHistory) {
		t.Errorf("history lost:\n got %+v\nwant %+v", out.PuHistory, in.PuHistory)
	}
}

func TestUserData_OmitemptyKeepsPushupKeysOut(t *testing.T) {
	raw, err := json.Marshal(UserData{State: StateIdle})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, key := range []string{
		"pu_consent_at", "pushups", "pu_session", "pu_test", "pu_history",
	} {
		if strings.Contains(s, `"`+key+`"`) {
			t.Errorf("empty UserData JSON should not contain %q key: %s", key, s)
		}
	}
}

func TestUserData_LegacyJSONWithoutPushupFields(t *testing.T) {
	legacy := `{"state":"awaiting_defecation","chat_id":100,"locale":"ru"}`
	var d UserData
	if err := json.Unmarshal([]byte(legacy), &d); err != nil {
		t.Fatal(err)
	}
	if d.Pushups != nil || d.PuSession != nil || d.PuTest != nil || d.PuConsentAt != nil {
		t.Errorf("pushup fields should be nil for legacy JSON: %+v", d)
	}
	if d.PuHistory != nil {
		t.Errorf("history should be nil for legacy JSON: %+v", d.PuHistory)
	}
}

// The track's share of the project's no-body-figures pin: the persisted
// shape asks for reps and seconds and nothing else. A field named for weight,
// height, BMI, calories or a numeric age would mean some text upstream asks
// for it — which the eating track's canary forbids bot-wide.
func TestPushupShapes_NoBodyMetricFields(t *testing.T) {
	banned := []string{"weight", "height", "bmi", "calorie", "kcal", "age", "kg", "fat"}
	for _, shape := range []any{
		PushupProgram{}, PushupSession{}, PushupTest{}, PushupSessionLog{},
	} {
		typ := reflect.TypeOf(shape)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.ToLower(f.Name + " " + f.Tag.Get("json"))
			for _, bad := range banned {
				if strings.Contains(name, bad) {
					t.Errorf("%s.%s looks like a body metric (%q)", typ.Name(), f.Name, bad)
				}
			}
		}
	}
}

func TestPushupProgram_Clone(t *testing.T) {
	if got := (*PushupProgram)(nil).Clone(); got != nil {
		t.Fatal("Clone of nil should be nil")
	}
	orig := fullPushupProgram()
	cl := orig.Clone()

	cl.WeekOutcomes[0] = "short"
	cl.WeekOutcomes = append(cl.WeekOutcomes, "short")
	cl.Base = 99

	if orig.WeekOutcomes[0] != "plan" || len(orig.WeekOutcomes) != 2 {
		t.Errorf("Clone shares WeekOutcomes with original: %+v", orig.WeekOutcomes)
	}
	if orig.Base != 19 {
		t.Errorf("Clone shares scalar fields with original: base %d", orig.Base)
	}
}

func TestPushupSession_Clone(t *testing.T) {
	if got := (*PushupSession)(nil).Clone(); got != nil {
		t.Fatal("Clone of nil should be nil")
	}
	orig := fullPushupSession()
	cl := orig.Clone()

	cl.Targets[0] = 99
	cl.Actual = append(cl.Actual, 7)
	cl.Idx = 4

	if orig.Targets[0] != 8 {
		t.Error("Clone shares Targets backing array with original")
	}
	if len(orig.Actual) != 2 {
		t.Errorf("Clone shares Actual with original: %+v", orig.Actual)
	}
	if orig.Idx != 2 {
		t.Errorf("Clone shares scalar fields with original: idx %d", orig.Idx)
	}
}

func TestPushupSession_Stale(t *testing.T) {
	start := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	s := &PushupSession{StartedAt: start}

	if !(*PushupSession)(nil).Stale(start, 0) {
		t.Error("a nil session has nothing to resume and must read as stale")
	}
	if s.Stale(start.Add(23*time.Hour), 0) {
		t.Error("a session must stay resumable within the default TTL")
	}
	if !s.Stale(start.Add(PushupSessionTTL), 0) {
		t.Error("a session must go stale at the default TTL")
	}
	if !s.Stale(start.Add(2*time.Hour), time.Hour) {
		t.Error("an explicit ttl must win over the default")
	}
	if (&PushupSession{}).Stale(start, 0) {
		t.Error("a session without StartedAt must be treated as fresh, not dropped")
	}
}

func TestAppendPushupLog_TrimsRing(t *testing.T) {
	var history []PushupSessionLog
	for i := 1; i <= 40; i++ {
		history = AppendPushupLog(history, PushupSessionLog{Base: i}, PushupHistoryCap)
	}
	if len(history) != PushupHistoryCap {
		t.Fatalf("history len = %d, want %d", len(history), PushupHistoryCap)
	}
	if history[0].Base != 40-PushupHistoryCap+1 {
		t.Errorf("oldest kept entry = %d, want %d", history[0].Base, 40-PushupHistoryCap+1)
	}
	if history[len(history)-1].Base != 40 {
		t.Errorf("newest entry = %d, want 40", history[len(history)-1].Base)
	}
}

func TestAppendPushupLog_DefaultsAndSmallCaps(t *testing.T) {
	var history []PushupSessionLog
	for i := 1; i <= PushupHistoryCap+5; i++ {
		history = AppendPushupLog(history, PushupSessionLog{Base: i}, 0)
	}
	if len(history) != PushupHistoryCap {
		t.Fatalf("non-positive cap should fall back to PushupHistoryCap, got len %d", len(history))
	}

	short := AppendPushupLog(history, PushupSessionLog{Base: 999}, 2)
	if len(short) != 2 || short[1].Base != 999 {
		t.Fatalf("shrinking the cap should keep the newest entries: %+v", short)
	}
	if short[0].Base != history[len(history)-1].Base {
		t.Errorf("second-newest entry lost: got %d, want %d", short[0].Base, history[len(history)-1].Base)
	}
}

func TestAppendPushupLog_DoesNotMutateInput(t *testing.T) {
	sets := []int{7, 8, 6}
	orig := []PushupSessionLog{{Base: 1}, {Base: 2}}
	origLen := len(orig)

	out := AppendPushupLog(orig, PushupSessionLog{Base: 3, Sets: sets}, 5)

	if len(orig) != origLen || orig[0].Base != 1 {
		t.Errorf("input history mutated: %+v", orig)
	}
	sets[0] = 99
	if out[2].Sets[0] != 7 {
		t.Error("appended entry shares its Sets backing array with the live session")
	}

	out[0].Base = 42
	if orig[0].Base != 1 {
		t.Error("returned slice shares its backing array with the input history")
	}
}

func TestIsPushupPingState(t *testing.T) {
	quiet := []StateKind{StateIdle, StateAwaitingModeChoice, StatePuMenu}
	for _, s := range quiet {
		if !IsPushupPingState(s) {
			t.Errorf("%q must accept the due ping", s)
		}
	}
	busy := []StateKind{
		StateAwaitingDefecation, StateAwaitingStageCheckin, StateScrAsrsA,
		StateMoodQuestion, StateEatBesQuestion, StatePuSet, StatePuRest,
		StatePuTest, StatePuDeleteConfirm,
	}
	for _, s := range busy {
		if IsPushupPingState(s) {
			t.Errorf("%q must stay silent — the user is mid-flow", s)
		}
	}
}

func TestStore_PushupRoundTripThroughBackend(t *testing.T) {
	s := NewStore(MemoryPersister{})
	s.Set(9, UserData{
		State:     StatePuSet,
		Pushups:   fullPushupProgram(),
		PuSession: fullPushupSession(),
		PuHistory: []PushupSessionLog{fullPushupLog()},
	})

	got := s.Get(9)
	if got.Pushups == nil || got.Pushups.Base != 19 || len(got.Pushups.WeekOutcomes) != 2 {
		t.Fatalf("program not stored: %+v", got.Pushups)
	}
	if got.PuSession == nil || len(got.PuSession.Targets) != 5 || got.PuSession.Idx != 2 {
		t.Fatalf("session not stored: %+v", got.PuSession)
	}
	if len(got.PuHistory) != 1 || got.PuHistory[0].OpenActual != 13 {
		t.Fatalf("history not stored: %+v", got.PuHistory)
	}
}

func TestStore_AllPushupResting(t *testing.T) {
	s := NewStore(MemoryPersister{})
	s.Set(1, UserData{State: StatePuRest, PuSession: fullPushupSession()})
	s.Set(2, UserData{State: StatePuSet, PuSession: fullPushupSession()})
	s.Set(3, UserData{State: StateAwaitingStageCheckin})

	got := s.AllPushupResting()
	if len(got) != 1 {
		t.Fatalf("expected exactly the resting user, got %v", got)
	}
	if _, ok := got[1]; !ok {
		t.Errorf("user 1 (pu_rest) missing: %v", got)
	}
}

func TestStore_AllPushupDue(t *testing.T) {
	due := time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC)
	now := due.Add(time.Minute)

	program := func(mut func(*PushupProgram)) *PushupProgram {
		p := &PushupProgram{Base: 19, NextDueAt: due}
		if mut != nil {
			mut(p)
		}
		return p
	}

	s := NewStore(MemoryPersister{})
	// Included: idle, landing and the track menu are the three quiet states.
	s.Set(1, UserData{State: StateIdle, Pushups: program(nil)})
	s.Set(2, UserData{State: StateAwaitingModeChoice, Pushups: program(nil)})
	s.Set(3, UserData{State: StatePuMenu, Pushups: program(nil)})
	// Included: a session that outlived its TTL no longer suppresses the ping.
	s.Set(4, UserData{
		State:     StateIdle,
		Pushups:   program(nil),
		PuSession: &PushupSession{StartedAt: now.Add(-2 * PushupSessionTTL)},
	})
	// Excluded: not due yet.
	s.Set(5, UserData{State: StateIdle, Pushups: program(func(p *PushupProgram) {
		p.NextDueAt = now.Add(time.Hour)
	})})
	// Excluded: already pinged for this due date.
	s.Set(6, UserData{State: StateIdle, Pushups: program(func(p *PushupProgram) {
		p.DuePingSent = true
	})})
	// Excluded: no due date at all (setup unfinished).
	s.Set(7, UserData{State: StateIdle, Pushups: program(func(p *PushupProgram) {
		p.NextDueAt = time.Time{}
	})})
	// Excluded: no program.
	s.Set(8, UserData{State: StateIdle})
	// Excluded: mid-flow in another track.
	s.Set(9, UserData{State: StateEatBesQuestion, Pushups: program(nil)})
	// Excluded: a live escaped session — the landing offers it instead.
	s.Set(10, UserData{
		State:     StateAwaitingModeChoice,
		Pushups:   program(nil),
		PuSession: &PushupSession{StartedAt: now.Add(-time.Hour)},
	})

	got := s.AllPushupDue(now)

	want := map[int64]bool{1: true, 2: true, 3: true, 4: true}
	if len(got) != len(want) {
		t.Fatalf("due set = %v, want ids %v", keysOf(got), want)
	}
	for id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("user %d should be due: %v", id, keysOf(got))
		}
	}
}

func TestStore_AllPushupDue_ReturnsCopies(t *testing.T) {
	due := time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC)
	s := NewStore(MemoryPersister{})
	s.Set(1, UserData{State: StateIdle, Pushups: &PushupProgram{Base: 19, NextDueAt: due}})

	got := s.AllPushupDue(due)
	d := got[1]
	d.State = StatePuSet
	if s.Get(1).State != StateIdle {
		t.Fatal("mutating the returned snapshot must not affect the store")
	}
}

func keysOf(m map[int64]UserData) []int64 {
	out := make([]int64, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out
}
