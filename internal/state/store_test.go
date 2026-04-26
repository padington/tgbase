package state_test

import (
	"sync"
	"testing"

	"github.com/padington/tgbase/internal/state"
)

func TestGet_NewUser(t *testing.T) {
	s := state.NewStore(state.MemoryPersister{})
	d := s.Get(42)
	if d.State != state.StateIdle {
		t.Fatalf("expected StateIdle, got %q", d.State)
	}
	if d.HowamiAnswer != 0 {
		t.Fatalf("expected HowamiAnswer=0, got %d", d.HowamiAnswer)
	}
}

func TestSetAndGet_RoundTrip(t *testing.T) {
	s := state.NewStore(state.MemoryPersister{})
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
	s := state.NewStore(state.MemoryPersister{})
	s.Set(1, state.UserData{State: state.StateIdle})

	d := s.Get(1)
	d.State = state.StateAwaitingHowamiAnswer

	if s.Get(1).State != state.StateIdle {
		t.Fatal("mutating returned value should not affect the store")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := state.NewStore(state.MemoryPersister{})
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

func TestStore_LoadsFromPersister(t *testing.T) {
	p := &recordingPersister{
		loadResult: map[int64]state.UserData{
			7: {State: state.StateAwaitingHowamiAnswer, HowamiAnswer: 0},
		},
	}
	s := state.NewStore(p)

	if got := s.Get(7).State; got != state.StateAwaitingHowamiAnswer {
		t.Errorf("expected loaded state to be StateAwaitingHowamiAnswer, got %q", got)
	}
}

func TestStore_SetTriggersSaveSnapshot(t *testing.T) {
	p := &recordingPersister{}
	s := state.NewStore(p)
	s.Set(1, state.UserData{State: state.StateAwaitingHowamiAnswer})

	saves := p.getSaves()
	if len(saves) != 1 {
		t.Fatalf("expected 1 save, got %d", len(saves))
	}
	if saves[0][1].State != state.StateAwaitingHowamiAnswer {
		t.Errorf("save snapshot missing the just-set value")
	}

	saves[0][1] = state.UserData{State: state.StateIdle}
	if s.Get(1).State != state.StateAwaitingHowamiAnswer {
		t.Error("mutating saved snapshot must not affect the store (snapshot is independent)")
	}
}

func TestStore_AllAwaiting(t *testing.T) {
	s := state.NewStore(state.MemoryPersister{})
	s.Set(1, state.UserData{State: state.StateAwaitingHowamiAnswer, ChatID: 100})
	s.Set(2, state.UserData{State: state.StateIdle})
	s.Set(3, state.UserData{State: state.StateAwaitingHowamiAnswer, ChatID: 300})

	got := s.AllAwaiting()
	if len(got) != 2 {
		t.Fatalf("expected 2 awaiting users, got %d", len(got))
	}
	if _, ok := got[1]; !ok {
		t.Error("missing user 1")
	}
	if _, ok := got[3]; !ok {
		t.Error("missing user 3")
	}
	if _, ok := got[2]; ok {
		t.Error("user 2 (idle) should not be in AllAwaiting")
	}
}

type recordingPersister struct {
	mu         sync.Mutex
	saves      []map[int64]state.UserData
	loadResult map[int64]state.UserData
}

func (r *recordingPersister) Load() (map[int64]state.UserData, error) {
	return r.loadResult, nil
}

func (r *recordingPersister) Save(m map[int64]state.UserData) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saves = append(r.saves, m)
	return nil
}

func (r *recordingPersister) Close() error { return nil }

func (r *recordingPersister) getSaves() []map[int64]state.UserData {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]map[int64]state.UserData, len(r.saves))
	copy(out, r.saves)
	return out
}
