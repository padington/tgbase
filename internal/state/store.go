package state

import (
	"log"
	"sync"
	"time"
)

// StateKind is the finite set of states a user can be in.
type StateKind string

const (
	StateIdle                 StateKind = "idle"
	StateAwaitingHowamiAnswer StateKind = "awaiting_howami_answer"
)

// UserData holds everything persisted per user.
type UserData struct {
	State        StateKind `json:"state"`
	HowamiAnswer int       `json:"howami_answer"`
	ChatID       int64     `json:"chat_id"`
	EnteredAt    time.Time `json:"entered_at"`
	ReminderSent bool      `json:"reminder_sent"`
}

// Store is a thread-safe map of userID → UserData backed by a Persister.
type Store struct {
	mu        sync.Mutex
	data      map[int64]UserData
	persister Persister
}

// NewStore loads existing state from the persister and returns a ready store.
// A nil persister is treated as MemoryPersister{}.
func NewStore(p Persister) *Store {
	if p == nil {
		p = MemoryPersister{}
	}
	s := &Store{
		data:      make(map[int64]UserData),
		persister: p,
	}
	loaded, err := p.Load()
	if err != nil {
		log.Printf("state: load: %v", err)
	} else if loaded != nil {
		s.data = loaded
	}
	return s
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

// Set stores d for userID and triggers a persister save with a fresh snapshot.
func (s *Store) Set(userID int64, d UserData) {
	s.mu.Lock()
	s.data[userID] = d
	snap := snapshot(s.data)
	s.mu.Unlock()
	if err := s.persister.Save(snap); err != nil {
		log.Printf("state: save: %v", err)
	}
}

// AllAwaiting returns a snapshot of users currently in StateAwaitingHowamiAnswer.
func (s *Store) AllAwaiting() map[int64]UserData {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int64]UserData)
	for id, d := range s.data {
		if d.State == StateAwaitingHowamiAnswer {
			out[id] = d
		}
	}
	return out
}

// Close flushes pending writes and releases persister resources.
func (s *Store) Close() error {
	return s.persister.Close()
}

func snapshot(m map[int64]UserData) map[int64]UserData {
	out := make(map[int64]UserData, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
