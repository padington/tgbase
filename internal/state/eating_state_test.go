package state

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fullEatingProgress() *EatingProgress {
	return &EatingProgress{
		Answers:     []int{3, 2, 0, 1},
		ResumeState: StateEatEdeqsQuestion,
		StartedAt:   time.Date(2026, 8, 26, 10, 1, 0, 0, time.UTC),
	}
}

func fullNiasResult() *NiasResult {
	return &NiasResult{
		TakenAt:          time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
		Picky:            12,
		Appetite:         4,
		Fear:             10,
		PickyCutoff:      10,
		AppetiteCutoff:   9,
		FearCutoff:       10,
		PickyPositive:    true,
		FearPositive:     true,
		AppetitePositive: false,
	}
}

func TestUserData_EatingJSONRoundTrip(t *testing.T) {
	consent := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	in := UserData{
		State:        StateEatBesQuestion,
		EatConsentAt: &consent,
		Edeqs:        fullEatingProgress(),
		EdeqsResult:  &EdeqsResult{TakenAt: consent, Score: 15, Cutoff: 15, Positive: true},
		Bes:          &EatingProgress{Answers: []int{3, 1}, ResumeState: StateEatBesQuestion},
		BesResult:    &BesResult{TakenAt: consent, Score: 18, Band: "moderate"},
		Nias:         &EatingProgress{Answers: []int{5}},
		NiasResult:   fullNiasResult(),
		ReturnState:  StateAwaitingStageCheckin,
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out UserData
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if out.EatConsentAt == nil || !out.EatConsentAt.Equal(consent) {
		t.Errorf("EatConsentAt lost: %v", out.EatConsentAt)
	}
	if out.Edeqs == nil || len(out.Edeqs.Answers) != 4 || out.Edeqs.ResumeState != StateEatEdeqsQuestion {
		t.Errorf("Edeqs progress lost: %+v", out.Edeqs)
	}
	if out.EdeqsResult == nil || out.EdeqsResult.Score != 15 ||
		out.EdeqsResult.Cutoff != 15 || !out.EdeqsResult.Positive {
		t.Errorf("EdeqsResult lost: %+v", out.EdeqsResult)
	}
	if out.BesResult == nil || out.BesResult.Score != 18 || out.BesResult.Band != "moderate" {
		t.Errorf("BesResult lost: %+v", out.BesResult)
	}
	if out.NiasResult == nil || out.NiasResult.Picky != 12 || out.NiasResult.FearCutoff != 10 {
		t.Errorf("NiasResult lost: %+v", out.NiasResult)
	}
	if !out.NiasResult.PickyPositive || out.NiasResult.AppetitePositive || !out.NiasResult.FearPositive {
		t.Errorf("NiasResult subscale verdicts lost: %+v", out.NiasResult)
	}
	if out.ReturnState != StateAwaitingStageCheckin {
		t.Errorf("ReturnState: %q", out.ReturnState)
	}
}

func TestNiasResult_AnyPositive(t *testing.T) {
	if (*NiasResult)(nil).AnyPositive() {
		t.Error("nil result must not report a positive subscale")
	}
	if (&NiasResult{}).AnyPositive() {
		t.Error("all-negative result must not report a positive subscale")
	}
	for _, r := range []*NiasResult{
		{PickyPositive: true}, {AppetitePositive: true}, {FearPositive: true},
	} {
		if !r.AnyPositive() {
			t.Errorf("%+v must report a positive subscale", r)
		}
	}
}

func TestUserData_LegacyJSONWithoutEatingFields(t *testing.T) {
	legacy := `{"state":"awaiting_defecation","chat_id":100,"locale":"ru"}`
	var d UserData
	if err := json.Unmarshal([]byte(legacy), &d); err != nil {
		t.Fatal(err)
	}
	if d.Edeqs != nil || d.Bes != nil || d.Nias != nil {
		t.Errorf("eating runs should be nil for legacy JSON: %+v %+v %+v", d.Edeqs, d.Bes, d.Nias)
	}
	if d.EdeqsResult != nil || d.BesResult != nil || d.NiasResult != nil || d.EatConsentAt != nil {
		t.Error("eating results/consent should be nil for legacy JSON")
	}
}

func TestUserData_OmitemptyKeepsEatingKeysOut(t *testing.T) {
	raw, err := json.Marshal(UserData{State: StateIdle})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, key := range []string{
		"eat_consent_at", "edeqs", "edeqs_result", "bes", "bes_result", "nias", "nias_result",
	} {
		if strings.Contains(s, `"`+key+`"`) {
			t.Errorf("empty UserData JSON should not contain %q key: %s", key, s)
		}
	}
}

// The privacy guard of the eating track: erasing the run must remove the raw
// answers from the persisted JSON in the same Set that writes the result,
// which keeps only totals, applied cutoffs and verdicts.
func TestUserData_NilEatingRunErasesRawAnswersFromJSON(t *testing.T) {
	d := UserData{State: StateEatEdeqsQuestion, Edeqs: fullEatingProgress()}
	raw, _ := json.Marshal(d)
	if !strings.Contains(string(raw), `"answers"`) {
		t.Fatalf("precondition: expected raw answers in JSON: %s", raw)
	}

	d.Edeqs = nil
	d.EdeqsResult = &EdeqsResult{TakenAt: time.Now(), Score: 15, Cutoff: 15, Positive: true}
	raw, _ = json.Marshal(d)
	s := string(raw)
	if strings.Contains(s, `"answers"`) || strings.Contains(s, `"edeqs":`) {
		t.Errorf("raw answers must disappear from JSON after Edeqs=nil: %s", s)
	}
	if !strings.Contains(s, "edeqs_result") || !strings.Contains(s, `"cutoff":15`) {
		t.Errorf("result (with the applied cutoff) should persist: %s", s)
	}
}

func TestEatingProgress_Clone(t *testing.T) {
	if got := (*EatingProgress)(nil).Clone(); got != nil {
		t.Fatal("Clone of nil should be nil")
	}
	orig := fullEatingProgress()
	cl := orig.Clone()

	cl.Answers[0] = 9
	cl.ResumeState = StateIdle

	if orig.Answers[0] != 3 {
		t.Error("Clone shares Answers backing array with original")
	}
	if orig.ResumeState != StateEatEdeqsQuestion {
		t.Error("Clone shares scalar fields with original (impossible)")
	}
}

func TestStore_EatingRoundTripThroughBackend(t *testing.T) {
	s := NewStore(MemoryPersister{})
	s.Set(9, UserData{State: StateEatNiasQuestion, Nias: fullEatingProgress(), NiasResult: fullNiasResult()})

	got := s.Get(9)
	if got.Nias == nil || len(got.Nias.Answers) != 4 {
		t.Fatalf("nias progress not stored: %+v", got.Nias)
	}
	if got.NiasResult == nil || got.NiasResult.Picky != 12 {
		t.Fatalf("nias result not stored: %+v", got.NiasResult)
	}
}
