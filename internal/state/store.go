package state

import "sync"

// StateKind is the finite set of states a user can be in.
type StateKind string

const (
	StateIdle                 StateKind = "idle"
	StateAwaitingHowamiAnswer StateKind = "awaiting_howami_answer"
)

// UserData holds everything persisted per user.
type UserData struct {
	State        StateKind
	HowamiAnswer int // 0=unset, 1/2/3
}

// Store is a thread-safe in-memory map of userID → UserData.
type Store struct {
	mu   sync.Mutex
	data map[int64]UserData
}

func NewStore() *Store {
	return &Store{data: make(map[int64]UserData)}
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

// Set stores d for userID (value copy — caller's struct is not aliased).
func (s *Store) Set(userID int64, d UserData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[userID] = d
}
