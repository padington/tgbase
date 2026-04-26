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
