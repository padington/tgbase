package state_test

import (
	"sync"
	"testing"

	"github.com/padington/tgbase/internal/state"
)

func TestGet_NewUser(t *testing.T) {
	s := state.NewStore()
	d := s.Get(42)
	if d.State != state.StateIdle {
		t.Fatalf("expected StateIdle, got %q", d.State)
	}
	if d.HowamiAnswer != 0 {
		t.Fatalf("expected HowamiAnswer=0, got %d", d.HowamiAnswer)
	}
}

func TestSetAndGet_RoundTrip(t *testing.T) {
	s := state.NewStore()
	s.Set(1, state.UserData{State: state.StateAwaitingHowamiAnswer, HowamiAnswer: 2})

	d := s.Get(1)
	if d.State != state.StateAwaitingHowamiAnswer {
		t.Fatalf("expected StateAwaitingHowamiAnswer, got %q", d.State)
	}
	if d.HowamiAnswer != 2 {
		t.Fatalf("expected HowamiAnswer=2, got %d", d.HowamiAnswer)
	}
}

func TestGet_IsolatedCopy(t *testing.T) {
	s := state.NewStore()
	s.Set(1, state.UserData{State: state.StateIdle})

	d := s.Get(1)
	d.State = state.StateAwaitingHowamiAnswer // mutate the copy

	// store must be unchanged
	if s.Get(1).State != state.StateIdle {
		t.Fatal("mutating returned value should not affect the store")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := state.NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			s.Set(id, state.UserData{State: state.StateAwaitingHowamiAnswer, HowamiAnswer: int(id % 3) + 1})
			_ = s.Get(id)
		}(int64(i))
	}
	wg.Wait()
}
