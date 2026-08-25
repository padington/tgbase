package state

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/store"
)

// StateKind enumerates the possible per-user states.
type StateKind string

const (
	StateIdle                    StateKind = "idle"
	StateAwaitingHowamiAnswer    StateKind = "awaiting_howami_answer" // legacy — survey flow, scheduled for removal
	StateAwaitingDefecation      StateKind = "awaiting_defecation"
	StateAwaitingProductCategory StateKind = "awaiting_product_category"
	StateAwaitingProductChoice   StateKind = "awaiting_product_choice"
	StateAwaitingStageChoice     StateKind = "awaiting_stage_choice"
	StateAwaitingStageCheckin    StateKind = "awaiting_stage_checkin"

	// Mode fork shown on /start: FODMAP diary vs ADHD self-check.
	StateAwaitingModeChoice StateKind = "awaiting_mode_choice"

	// ADHD screening states (scr_*). None of these are scanned by the
	// reminder loop — screening has no nudges in v1.
	StateScrConsent       StateKind = "scr_consent"
	StateScrIntro         StateKind = "scr_intro"
	StateScrAsrsA         StateKind = "scr_asrs_a"
	StateScrAsrsAGate     StateKind = "scr_asrs_a_gate"
	StateScrAsrsB         StateKind = "scr_asrs_b"
	StateScrAsrsBGate     StateKind = "scr_asrs_b_gate"
	StateScrWursForm      StateKind = "scr_wurs_form"
	StateScrWurs          StateKind = "scr_wurs"
	StateScrWursGate      StateKind = "scr_wurs_gate"
	StateScrOnset         StateKind = "scr_onset"
	StateScrOnsetAge      StateKind = "scr_onset_age"
	StateScrDomainsAdult  StateKind = "scr_domains_adult"
	StateScrDomainsChild  StateKind = "scr_domains_child"
	StateScrReferral      StateKind = "scr_referral"
	StateScrReport        StateKind = "scr_report"
	StateScrDeleteConfirm StateKind = "scr_delete_confirm"

	// Mood-module states (mood_*: PHQ-9, WHO-5, GAD-7). Like scr_*, none of
	// these are scanned by the reminder loop — screening has no nudges in v1.
	StateMoodConsent       StateKind = "mood_consent"
	StateMoodMenu          StateKind = "mood_menu"
	StateMoodQuestion      StateKind = "mood_question"
	StateMoodCrisis        StateKind = "mood_crisis"
	StateMoodQ10           StateKind = "mood_q10"
	StateMoodReport        StateKind = "mood_report"
	StateMoodWho5Question  StateKind = "mood_who5_question"
	StateMoodOfferPhq9     StateKind = "mood_offer_phq9"
	StateMoodGad7Question  StateKind = "mood_gad7_question"
	StateMoodOfferGad7     StateKind = "mood_offer_gad7"
	StateMoodDeleteConfirm StateKind = "mood_delete_confirm"
)

// DefecationKind captures the user's reply to the defecation question.
type DefecationKind string

const (
	DefecationUnset  DefecationKind = ""
	DefecationFluid  DefecationKind = "fluid"
	DefecationNormal DefecationKind = "normal"
	DefecationIssues DefecationKind = "issues"
)

// ProductProgress tracks per-product trial state for a user.
type ProductProgress struct {
	LastStage products.Stage `json:"last_stage"`
	Status    string         `json:"status"` // in_progress | completed | not_tolerated | interrupted
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

// ScreeningProgress is the TRANSIENT state of an unfinished ADHD screening
// run — it exists only so the user can resume. nil whenever no screening is
// in progress. It is wiped on completion, on restart ("Начать заново"), on
// /abandon, and on /adhd_delete. Raw per-question answers live ONLY here:
// they reach disk (users.json) only while the test is unfinished and are
// erased in the same Set that persists the final ScreeningResult.
type ScreeningProgress struct {
	AsrsAnswers []int  `json:"asrs_answers,omitempty"` // append-only, scores 0..4; index = question id - 1
	WursAnswers []int  `json:"wurs_answers,omitempty"` // append-only, scores 0..4
	WursForm    string `json:"wurs_form,omitempty"`    // "m" | "f"; transient, never copied to the result
	OnsetChild  *bool  `json:"onset_child,omitempty"`  // nil = not asked yet
	OnsetAge    int    `json:"onset_age,omitempty"`    // >0 when OnsetChild == false
	// Life domains are asked one at a time (yes/no per domain). The *Idx
	// cursors count domains ANSWERED so far in each pass (both yes and no),
	// so a paused run resumes at the right domain; the *Domains lists keep
	// only the ids answered "yes" — the shape the final result stores.
	AdultDomainIdx int       `json:"adult_domain_idx,omitempty"`
	ChildDomainIdx int       `json:"child_domain_idx,omitempty"`
	AdultDomains   []string  `json:"adult_domains,omitempty"`
	ChildDomains   []string  `json:"child_domains,omitempty"`
	ResumeState    StateKind `json:"resume_state,omitempty"`
	ConsentAt      time.Time `json:"consent_at,omitempty"`
	StartedAt      time.Time `json:"started_at,omitempty"`
}

// Clone returns a deep copy (slices and the OnsetChild pointer are copied),
// so Outcome.Mutate can replace the pointer instead of mutating shared data.
func (p *ScreeningProgress) Clone() *ScreeningProgress {
	if p == nil {
		return nil
	}
	out := *p
	out.AsrsAnswers = append([]int(nil), p.AsrsAnswers...)
	out.WursAnswers = append([]int(nil), p.WursAnswers...)
	out.AdultDomains = append([]string(nil), p.AdultDomains...)
	out.ChildDomains = append([]string(nil), p.ChildDomains...)
	if p.OnsetChild != nil {
		v := *p.OnsetChild
		out.OnsetChild = &v
	}
	return &out
}

// MoodProgress is the TRANSIENT state of one unfinished mood-module
// instrument run (PHQ-9 in UserData.Mood, WHO-5 in .Who5, GAD-7 in .Gad7) —
// it exists only so the user can resume. nil whenever no run is in progress.
// It is wiped on completion, on restart, on /abandon, and on /mood_delete.
// Raw per-question answers live ONLY here: they reach disk (users.json) only
// while the test is unfinished and are erased in the same Set that persists
// the final result. For PHQ-9, index 9 (when present) is the answer to the
// functional (10th) item.
type MoodProgress struct {
	Answers     []int     `json:"answers,omitempty"` // append-only; index = question id - 1
	ResumeState StateKind `json:"resume_state,omitempty"`
	ConsentAt   time.Time `json:"consent_at,omitempty"` // legacy v1 per-run consent; module consent is UserData.MoodConsentAt
	StartedAt   time.Time `json:"started_at,omitempty"`
}

// Clone returns a deep copy (the answers slice is copied), so Outcome.Mutate
// can replace the pointer instead of mutating shared data.
func (p *MoodProgress) Clone() *MoodProgress {
	if p == nil {
		return nil
	}
	out := *p
	out.Answers = append([]int(nil), p.Answers...)
	return &out
}

// MoodResult is the last COMPLETED PHQ-9 run (overwritten by each new
// completion). Only the total score, the severity-band id that was applied,
// the crisis-item flag, the functional-item answer, and the date —
// per-question answers 1..9 are never stored here. Q9Positive is one of the
// two per-question facts the privacy policy allows: whether the self-harm
// item (9) was answered above zero, kept so the result and /report can
// repeat the support contacts. The other is the official functional (10th)
// item: asked only when at least one of the nine answers was > 0, NOT part
// of the 0–27 score, surfaced as its own line in the doctor report
// (Q10Answer is meaningful only when Q10Answered).
type MoodResult struct {
	TakenAt     time.Time `json:"taken_at"`
	Score       int       `json:"score"`    // 0..27 (functional item not included)
	Severity    string    `json:"severity"` // applied severity-band id
	Q9Positive  bool      `json:"q9_positive,omitempty"`
	Q10Answered bool      `json:"q10_answered,omitempty"`
	Q10Answer   int       `json:"q10_answer,omitempty"` // 0..3
}

// Who5Result is the last COMPLETED WHO-5 quick check (overwritten by each
// new completion): the 0–100 well-being score (raw 0–25 sum × 4) and the
// interpretation band that was applied — never per-statement answers.
type Who5Result struct {
	TakenAt time.Time `json:"taken_at"`
	Score   int       `json:"score"` // 0..100
	Band    string    `json:"band"`  // ok | low | very_low
}

// Gad7Result is the last COMPLETED GAD-7 run (overwritten by each new
// completion): the total score and the severity-band id that was applied —
// never per-question answers.
type Gad7Result struct {
	TakenAt  time.Time `json:"taken_at"`
	Score    int       `json:"score"`    // 0..21
	Severity string    `json:"severity"` // minimal | mild | moderate | severe
}

// ScreeningResult is the last COMPLETED screening run (overwritten by each
// new completion). Only scores, the thresholds that were applied, domain
// ids, the onset fact/age, and the date — per-question answers are never
// stored here. Thresholds are copied from the content on purpose: the report
// stays honest even if the content file changes its cutoffs later.
type ScreeningResult struct {
	TakenAt          time.Time `json:"taken_at"`
	AsrsASignificant int       `json:"asrs_a_significant"` // 0..6
	AsrsAThreshold   int       `json:"asrs_a_threshold"`   // applied threshold (4)
	AsrsAPositive    bool      `json:"asrs_a_positive"`
	AsrsBSignificant int       `json:"asrs_b_significant"` // 0..12; part B has no threshold
	WursScore        int       `json:"wurs_score"`         // 0..100
	WursCutoff       int       `json:"wurs_cutoff"`        // applied cutoff (46)
	WursPositive     bool      `json:"wurs_positive"`
	OnsetChildhood   bool      `json:"onset_childhood"`
	OnsetAge         int       `json:"onset_age,omitempty"` // 0 when OnsetChildhood
	AdultDomains     []string  `json:"adult_domains,omitempty"`
	ChildDomains     []string  `json:"child_domains,omitempty"`
	Verdict          string    `json:"verdict"`            // consistent | partial | not_consistent
	GapHint          string    `json:"gap_hint,omitempty"` // gap-hint key when partial
}

// UserData holds everything persisted per user.
type UserData struct {
	State        StateKind `json:"state"`
	HowamiAnswer int       `json:"howami_answer,omitempty"` // legacy survey artifact, scheduled for removal

	DefecationState DefecationKind             `json:"defecation_state,omitempty"`
	CurrentProduct  string                     `json:"current_product,omitempty"`
	CurrentStage    products.Stage             `json:"current_stage,omitempty"`
	StageStartedAt  time.Time                  `json:"stage_started_at,omitempty"`
	CheckinAsked    bool                       `json:"checkin_asked,omitempty"`
	OfferedProducts []string                   `json:"offered_products,omitempty"`
	PickerCategory  string                     `json:"picker_category,omitempty"`
	PickerPage      int                        `json:"picker_page,omitempty"`
	Products        map[string]ProductProgress `json:"products,omitempty"`
	Locale          string                     `json:"locale,omitempty"`

	// ADHD screening. Screening is the transient in-progress run (nil when
	// none); ScreeningResult is the last completed run. ReturnState remembers
	// the state a detour interrupted: a FODMAP journey state (recorded when a
	// self-check starts from one; test exits keep it so the home landing can
	// offer the way back) or the landing itself (recorded when a delete
	// confirmation is entered from it).
	Screening       *ScreeningProgress `json:"screening,omitempty"`
	ScreeningResult *ScreeningResult   `json:"screening_result,omitempty"`
	ReturnState     StateKind          `json:"return_state,omitempty"`

	// Mood module (PHQ-9 + WHO-5 + GAD-7). Same contract as the ADHD pair;
	// ReturnState is shared by all detours (only one test runs at a time by
	// state). MoodConsentAt is the single module-wide consent (asked once,
	// covers all three instruments; nil = not given); /mood_delete wipes all
	// six fields. Each instrument keeps its own transient run and its own
	// last completed result — there is no combined index by design.
	Mood          *MoodProgress `json:"mood,omitempty"` // PHQ-9 run
	MoodResult    *MoodResult   `json:"mood_result,omitempty"`
	MoodConsentAt *time.Time    `json:"mood_consent_at,omitempty"`
	Who5          *MoodProgress `json:"who5,omitempty"`
	Who5Result    *Who5Result   `json:"who5_result,omitempty"`
	Gad7          *MoodProgress `json:"gad7,omitempty"`
	Gad7Result    *Gad7Result   `json:"gad7_result,omitempty"`

	ChatID       int64     `json:"chat_id,omitempty"`
	EnteredAt    time.Time `json:"entered_at,omitempty"`
	ReminderSent bool      `json:"reminder_sent,omitempty"`
}

// Store is a thread-safe per-user data store backed by store.Backend.
type Store struct {
	mu      sync.Mutex
	data    map[int64]UserData
	backend store.Backend
}

const backendKey = "users"

// NewStoreFromBackend loads users from backend and returns a ready Store.
// The store will Put back to backend after every Set.
func NewStoreFromBackend(backend store.Backend) *Store {
	if backend == nil {
		backend = store.NewMemoryBackend()
	}
	s := &Store{
		data:    make(map[int64]UserData),
		backend: backend,
	}
	s.load()
	return s
}

// NewStore is the legacy constructor accepting a Persister. Kept for the
// transition; new callers should use NewStoreFromBackend with a store.Backend.
// A nil Persister is treated as MemoryPersister{}.
func NewStore(p Persister) *Store {
	if p == nil {
		p = MemoryPersister{}
	}
	return NewStoreFromBackend(&persisterBackendAdapter{p: p})
}

func (s *Store) load() {
	raw, err := s.backend.Get(backendKey)
	if err != nil {
		log.Printf("state: load: %v", err)
		return
	}
	if raw == nil {
		return
	}
	var loaded map[int64]UserData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		log.Printf("state: parse: %v", err)
		return
	}
	if loaded != nil {
		s.data = loaded
	}
}

func (s *Store) save(snap map[int64]UserData) {
	raw, err := json.Marshal(snap)
	if err != nil {
		log.Printf("state: marshal: %v", err)
		return
	}
	if err := s.backend.Put(backendKey, raw); err != nil {
		log.Printf("state: save: %v", err)
	}
}

// Get returns a copy of the UserData for userID.
// Returns a zero-value UserData with StateIdle if the user has never been seen.
func (s *Store) Get(userID int64) UserData {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.data[userID]
	if !ok {
		return UserData{State: StateIdle}
	}
	return d
}

// Set stores d for userID and persists a snapshot.
func (s *Store) Set(userID int64, d UserData) {
	s.mu.Lock()
	s.data[userID] = d
	snap := snapshot(s.data)
	s.mu.Unlock()
	s.save(snap)
}

// AllAwaiting returns users in StateAwaitingHowamiAnswer (legacy survey flow).
// Deprecated: use AllAwaitingDefecation or AllAwaitingCheckin once survey is removed.
func (s *Store) AllAwaiting() map[int64]UserData {
	return s.filterByState(StateAwaitingHowamiAnswer)
}

// AllAwaitingDefecation returns a snapshot of users in StateAwaitingDefecation.
func (s *Store) AllAwaitingDefecation() map[int64]UserData {
	return s.filterByState(StateAwaitingDefecation)
}

// AllAwaitingCheckin returns a snapshot of users in StateAwaitingStageCheckin.
func (s *Store) AllAwaitingCheckin() map[int64]UserData {
	return s.filterByState(StateAwaitingStageCheckin)
}

func (s *Store) filterByState(want StateKind) map[int64]UserData {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int64]UserData)
	for id, d := range s.data {
		if d.State == want {
			out[id] = d
		}
	}
	return out
}

// Close flushes pending writes and releases backend resources.
func (s *Store) Close() error {
	return s.backend.Close()
}

func snapshot(m map[int64]UserData) map[int64]UserData {
	out := make(map[int64]UserData, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
