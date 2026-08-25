package state

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fullMoodProgress() *MoodProgress {
	return &MoodProgress{
		Answers:     []int{3, 2, 1, 0, 3},
		ResumeState: StateMoodQuestion,
		ConsentAt:   time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC),
		StartedAt:   time.Date(2026, 8, 25, 10, 1, 0, 0, time.UTC),
	}
}

func fullMoodResult() *MoodResult {
	return &MoodResult{
		TakenAt:     time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC),
		Score:       17,
		Severity:    "moderately_severe",
		Q9Positive:  true,
		Q10Answered: true,
		Q10Answer:   2,
	}
}

func TestUserData_MoodJSONRoundTrip(t *testing.T) {
	in := UserData{
		State:       StateMoodCrisis,
		Mood:        fullMoodProgress(),
		MoodResult:  fullMoodResult(),
		ReturnState: StateAwaitingStageCheckin,
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out UserData
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if out.State != StateMoodCrisis {
		t.Errorf("State: %q", out.State)
	}
	if out.Mood == nil {
		t.Fatal("Mood lost in roundtrip")
	}
	if got := out.Mood.Answers; len(got) != 5 || got[0] != 3 {
		t.Errorf("Answers: %v", got)
	}
	if out.Mood.ResumeState != StateMoodQuestion {
		t.Errorf("ResumeState: %q", out.Mood.ResumeState)
	}
	if out.MoodResult == nil || out.MoodResult.Score != 17 {
		t.Fatalf("MoodResult lost: %+v", out.MoodResult)
	}
	if out.MoodResult.Severity != "moderately_severe" || !out.MoodResult.Q9Positive {
		t.Errorf("MoodResult fields: %+v", out.MoodResult)
	}
	if !out.MoodResult.Q10Answered || out.MoodResult.Q10Answer != 2 {
		t.Errorf("MoodResult functional-item fields: %+v", out.MoodResult)
	}
}

// TestUserData_MoodV2JSONRoundTrip covers the module-wide consent timestamp
// and the WHO-5 / GAD-7 progress+result pairs added in mood v2.
func TestUserData_MoodV2JSONRoundTrip(t *testing.T) {
	consent := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	in := UserData{
		State:         StateMoodGad7Question,
		MoodConsentAt: &consent,
		Who5:          &MoodProgress{Answers: []int{5, 0, 3}, StartedAt: consent},
		Gad7:          &MoodProgress{Answers: []int{1, 2}, ResumeState: StateMoodGad7Question},
		Who5Result:    &Who5Result{TakenAt: consent, Score: 48, Band: "low"},
		Gad7Result:    &Gad7Result{TakenAt: consent, Score: 11, Severity: "moderate"},
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out UserData
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if out.MoodConsentAt == nil || !out.MoodConsentAt.Equal(consent) {
		t.Errorf("MoodConsentAt lost: %v", out.MoodConsentAt)
	}
	if out.Who5 == nil || len(out.Who5.Answers) != 3 || out.Who5.Answers[0] != 5 {
		t.Errorf("Who5 progress lost: %+v", out.Who5)
	}
	if out.Gad7 == nil || out.Gad7.ResumeState != StateMoodGad7Question {
		t.Errorf("Gad7 progress lost: %+v", out.Gad7)
	}
	if out.Who5Result == nil || out.Who5Result.Score != 48 || out.Who5Result.Band != "low" {
		t.Errorf("Who5Result lost: %+v", out.Who5Result)
	}
	if out.Gad7Result == nil || out.Gad7Result.Score != 11 || out.Gad7Result.Severity != "moderate" {
		t.Errorf("Gad7Result lost: %+v", out.Gad7Result)
	}
}

func TestUserData_LegacyJSONWithoutMoodFields(t *testing.T) {
	legacy := `{"state":"awaiting_defecation","chat_id":100,"locale":"ru"}`
	var d UserData
	if err := json.Unmarshal([]byte(legacy), &d); err != nil {
		t.Fatal(err)
	}
	if d.Mood != nil {
		t.Errorf("Mood should be nil for legacy JSON, got %+v", d.Mood)
	}
	if d.MoodResult != nil {
		t.Errorf("MoodResult should be nil for legacy JSON, got %+v", d.MoodResult)
	}
}

func TestUserData_OmitemptyKeepsMoodKeysOut(t *testing.T) {
	raw, err := json.Marshal(UserData{State: StateIdle})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, key := range []string{"mood", "mood_result", "mood_consent_at", "who5", "who5_result", "gad7", "gad7_result"} {
		if strings.Contains(s, `"`+key+`"`) {
			t.Errorf("empty UserData JSON should not contain %q key: %s", key, s)
		}
	}
}

// Erasing Mood must actually remove the raw answers from the persisted JSON
// — the privacy guard for "raw answers live only until the test completes".
// The stored result keeps the score, band, and the single allowed
// per-question fact (the item-9 flag) — never the answers themselves.
func TestUserData_NilMoodErasesRawAnswersFromJSON(t *testing.T) {
	d := UserData{State: StateMoodQuestion, Mood: fullMoodProgress()}
	raw, _ := json.Marshal(d)
	if !strings.Contains(string(raw), `"answers"`) {
		t.Fatalf("precondition: expected raw answers in JSON: %s", raw)
	}

	d.Mood = nil
	d.MoodResult = fullMoodResult()
	raw, _ = json.Marshal(d)
	s := string(raw)
	if strings.Contains(s, `"answers"`) || strings.Contains(s, `"mood":`) {
		t.Errorf("raw answers must disappear from JSON after Mood=nil: %s", s)
	}
	if !strings.Contains(s, "mood_result") || !strings.Contains(s, "q9_positive") {
		t.Errorf("result (with the q9 flag) should persist: %s", s)
	}
}

func TestMoodProgress_Clone(t *testing.T) {
	if got := (*MoodProgress)(nil).Clone(); got != nil {
		t.Fatal("Clone of nil should be nil")
	}
	orig := fullMoodProgress()
	cl := orig.Clone()

	cl.Answers[0] = 9
	cl.ResumeState = StateIdle

	if orig.Answers[0] != 3 {
		t.Error("Clone shares Answers backing array with original")
	}
	if orig.ResumeState != StateMoodQuestion {
		t.Error("Clone shares scalar fields with original (impossible)")
	}
}

func TestStore_MoodRoundTripThroughBackend(t *testing.T) {
	s := NewStore(MemoryPersister{})
	s.Set(7, UserData{State: StateMoodQuestion, Mood: fullMoodProgress(), ReturnState: StateAwaitingDefecation})

	got := s.Get(7)
	if got.Mood == nil || len(got.Mood.Answers) != 5 {
		t.Fatalf("mood progress not stored: %+v", got.Mood)
	}
	if got.ReturnState != StateAwaitingDefecation {
		t.Errorf("ReturnState: %q", got.ReturnState)
	}
}
