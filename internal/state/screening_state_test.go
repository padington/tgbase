package state

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func boolPtr(v bool) *bool { return &v }

func fullProgress() *ScreeningProgress {
	return &ScreeningProgress{
		AsrsAnswers:  []int{4, 3, 2, 1, 0, 4},
		WursAnswers:  []int{0, 1, 2, 3, 4},
		WursForm:     "f",
		OnsetChild:   boolPtr(false),
		OnsetAge:     16,
		AdultDomains: []string{"work_study", "social"},
		ChildDomains: []string{"self_esteem"},
		ResumeState:  StateScrWurs,
		ConsentAt:    time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		StartedAt:    time.Date(2026, 8, 24, 10, 1, 0, 0, time.UTC),
	}
}

func fullResult() *ScreeningResult {
	return &ScreeningResult{
		TakenAt:          time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC),
		AsrsASignificant: 5,
		AsrsAThreshold:   4,
		AsrsAPositive:    true,
		AsrsBSignificant: 7,
		WursScore:        52,
		WursCutoff:       46,
		WursPositive:     true,
		OnsetChildhood:   true,
		AdultDomains:     []string{"work_study", "social"},
		ChildDomains:     []string{"self_esteem"},
		Verdict:          "consistent",
	}
}

func TestUserData_ScreeningJSONRoundTrip(t *testing.T) {
	for _, onset := range []*bool{nil, boolPtr(false), boolPtr(true)} {
		in := UserData{
			State:           StateScrWurs,
			Screening:       fullProgress(),
			ScreeningResult: fullResult(),
			ReturnState:     StateAwaitingStageCheckin,
		}
		in.Screening.OnsetChild = onset

		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		var out UserData
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}

		if out.ReturnState != StateAwaitingStageCheckin {
			t.Errorf("ReturnState: %q", out.ReturnState)
		}
		if out.Screening == nil {
			t.Fatal("Screening lost in roundtrip")
		}
		switch {
		case onset == nil && out.Screening.OnsetChild != nil:
			t.Error("OnsetChild nil not preserved")
		case onset != nil && (out.Screening.OnsetChild == nil || *out.Screening.OnsetChild != *onset):
			t.Errorf("OnsetChild %v not preserved", *onset)
		}
		if got := out.Screening.AsrsAnswers; len(got) != 6 || got[0] != 4 {
			t.Errorf("AsrsAnswers: %v", got)
		}
		if out.Screening.ResumeState != StateScrWurs {
			t.Errorf("ResumeState: %q", out.Screening.ResumeState)
		}
		if out.ScreeningResult == nil || out.ScreeningResult.WursScore != 52 {
			t.Errorf("ScreeningResult lost: %+v", out.ScreeningResult)
		}
		if out.ScreeningResult.Verdict != "consistent" {
			t.Errorf("Verdict: %q", out.ScreeningResult.Verdict)
		}
	}
}

func TestUserData_LegacyJSONWithoutScreeningFields(t *testing.T) {
	legacy := `{"state":"awaiting_defecation","chat_id":100,"locale":"ru"}`
	var d UserData
	if err := json.Unmarshal([]byte(legacy), &d); err != nil {
		t.Fatal(err)
	}
	if d.Screening != nil {
		t.Errorf("Screening should be nil for legacy JSON, got %+v", d.Screening)
	}
	if d.ScreeningResult != nil {
		t.Errorf("ScreeningResult should be nil for legacy JSON, got %+v", d.ScreeningResult)
	}
	if d.ReturnState != "" {
		t.Errorf("ReturnState should be empty for legacy JSON, got %q", d.ReturnState)
	}
	if d.State != StateAwaitingDefecation {
		t.Errorf("State: %q", d.State)
	}
}

func TestUserData_OmitemptyKeepsScreeningKeysOut(t *testing.T) {
	raw, err := json.Marshal(UserData{State: StateIdle})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, key := range []string{"screening", "screening_result", "return_state"} {
		if strings.Contains(s, `"`+key+`"`) {
			t.Errorf("empty UserData JSON should not contain %q key: %s", key, s)
		}
	}
}

// Erasing Screening must actually remove the raw answers from the persisted
// JSON — this is the privacy guard for "raw answers live only until the
// test completes".
func TestUserData_NilScreeningErasesRawAnswersFromJSON(t *testing.T) {
	d := UserData{State: StateScrWurs, Screening: fullProgress()}
	raw, _ := json.Marshal(d)
	if !strings.Contains(string(raw), "asrs_answers") {
		t.Fatalf("precondition: expected raw answers in JSON: %s", raw)
	}

	d.Screening = nil
	d.ScreeningResult = fullResult()
	raw, _ = json.Marshal(d)
	s := string(raw)
	if strings.Contains(s, "asrs_answers") || strings.Contains(s, "wurs_answers") || strings.Contains(s, `"screening":`) {
		t.Errorf("raw answers must disappear from JSON after Screening=nil: %s", s)
	}
	if !strings.Contains(s, "screening_result") {
		t.Errorf("result should persist: %s", s)
	}
}

func TestScreeningProgress_Clone(t *testing.T) {
	if got := (*ScreeningProgress)(nil).Clone(); got != nil {
		t.Fatal("Clone of nil should be nil")
	}
	orig := fullProgress()
	cl := orig.Clone()

	cl.AsrsAnswers[0] = 9
	cl.AdultDomains[0] = "changed"
	*cl.OnsetChild = true
	cl.WursForm = "m"

	if orig.AsrsAnswers[0] != 4 {
		t.Error("Clone shares AsrsAnswers backing array with original")
	}
	if orig.AdultDomains[0] != "work_study" {
		t.Error("Clone shares AdultDomains with original")
	}
	if *orig.OnsetChild {
		t.Error("Clone shares OnsetChild pointer with original")
	}
	if orig.WursForm != "f" {
		t.Error("Clone shares scalar fields with original (impossible)")
	}
}

func TestStore_ScreeningRoundTripThroughBackend(t *testing.T) {
	s := NewStore(MemoryPersister{})
	s.Set(7, UserData{State: StateScrOnset, Screening: fullProgress(), ReturnState: StateAwaitingDefecation})

	got := s.Get(7)
	if got.Screening == nil || got.Screening.WursForm != "f" {
		t.Fatalf("screening progress not stored: %+v", got.Screening)
	}
	if got.ReturnState != StateAwaitingDefecation {
		t.Errorf("ReturnState: %q", got.ReturnState)
	}
}
