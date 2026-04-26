package state_test

import (
	"sync"
	"testing"
	"time"

	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/state"
	"github.com/padington/tgbase/internal/store"
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

func TestNewStoreFromBackend_RoundTrip(t *testing.T) {
	backend := store.NewMemoryBackend()
	s := state.NewStoreFromBackend(backend)
	s.Set(7, state.UserData{
		State:           state.StateAwaitingProductChoice,
		DefecationState: state.DefecationNormal,
		ChatID:          700,
		Locale:          "ru",
	})

	s2 := state.NewStoreFromBackend(backend)
	got := s2.Get(7)
	if got.State != state.StateAwaitingProductChoice {
		t.Errorf("state lost across reload: %q", got.State)
	}
	if got.DefecationState != state.DefecationNormal {
		t.Errorf("defecation lost: %q", got.DefecationState)
	}
	if got.Locale != "ru" {
		t.Errorf("locale lost: %q", got.Locale)
	}
}

func TestUserData_PickerFieldsRoundTrip(t *testing.T) {
	backend := store.NewMemoryBackend()
	s := state.NewStoreFromBackend(backend)
	s.Set(9, state.UserData{
		State:          state.StateAwaitingProductCategory,
		PickerCategory: "fruits",
		PickerPage:     2,
	})

	s2 := state.NewStoreFromBackend(backend)
	got := s2.Get(9)
	if got.State != state.StateAwaitingProductCategory {
		t.Errorf("state: got %q", got.State)
	}
	if got.PickerCategory != "fruits" {
		t.Errorf("picker category: got %q", got.PickerCategory)
	}
	if got.PickerPage != 2 {
		t.Errorf("picker page: got %d", got.PickerPage)
	}
}

func TestStore_AllAwaitingDefecation(t *testing.T) {
	s := state.NewStoreFromBackend(store.NewMemoryBackend())
	s.Set(1, state.UserData{State: state.StateAwaitingDefecation, ChatID: 100})
	s.Set(2, state.UserData{State: state.StateIdle})
	s.Set(3, state.UserData{State: state.StateAwaitingProductChoice})
	s.Set(4, state.UserData{State: state.StateAwaitingDefecation, ChatID: 400})

	got := s.AllAwaitingDefecation()
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	for _, want := range []int64{1, 4} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing user %d", want)
		}
	}
}

func TestStore_AllAwaitingCheckin(t *testing.T) {
	s := state.NewStoreFromBackend(store.NewMemoryBackend())
	s.Set(1, state.UserData{State: state.StateAwaitingStageCheckin, CurrentProduct: "Apple", CurrentStage: products.StageLow})
	s.Set(2, state.UserData{State: state.StateAwaitingDefecation})
	s.Set(3, state.UserData{State: state.StateAwaitingStageCheckin, CurrentProduct: "Cashews", CurrentStage: products.StageMedium})

	got := s.AllAwaitingCheckin()
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[1].CurrentStage != products.StageLow {
		t.Errorf("user 1 stage: got %q", got[1].CurrentStage)
	}
	if got[3].CurrentStage != products.StageMedium {
		t.Errorf("user 3 stage: got %q", got[3].CurrentStage)
	}
}

func TestUserData_ProductsRoundTrip(t *testing.T) {
	backend := store.NewMemoryBackend()
	s := state.NewStoreFromBackend(backend)

	now := time.Now().UTC().Truncate(time.Second)
	s.Set(1, state.UserData{
		State: state.StateAwaitingProductChoice,
		Products: map[string]state.ProductProgress{
			"Apple":   {LastStage: products.StageHigh, Status: "completed", UpdatedAt: now},
			"Cashews": {LastStage: products.StageLow, Status: "in_progress", UpdatedAt: now},
		},
	})

	s2 := state.NewStoreFromBackend(backend)
	got := s2.Get(1).Products
	if len(got) != 2 {
		t.Fatalf("expected 2 products, got %d", len(got))
	}
	if got["Apple"].Status != "completed" {
		t.Errorf("apple status: %q", got["Apple"].Status)
	}
	if got["Cashews"].LastStage != products.StageLow {
		t.Errorf("cashews stage: %q", got["Cashews"].LastStage)
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
