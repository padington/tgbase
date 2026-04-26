package state_test

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/padington/tgbase/internal/state"
)

func TestFilePersister_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	p := state.NewFilePersister(path)

	in := map[int64]state.UserData{
		1: {State: state.StateAwaitingHowamiAnswer, ChatID: 100, EnteredAt: time.Now().UTC().Truncate(time.Second), ReminderSent: false},
		2: {State: state.StateIdle, HowamiAnswer: 3},
	}
	if err := p.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(in) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(in))
	}
	for k, v := range in {
		if got[k].State != v.State {
			t.Errorf("user %d state: got %q, want %q", k, got[k].State, v.State)
		}
		if got[k].ChatID != v.ChatID {
			t.Errorf("user %d chatID: got %d, want %d", k, got[k].ChatID, v.ChatID)
		}
		if !got[k].EnteredAt.Equal(v.EnteredAt) {
			t.Errorf("user %d EnteredAt: got %v, want %v", k, got[k].EnteredAt, v.EnteredAt)
		}
	}
}

func TestFilePersister_LoadMissingFileReturnsNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	got, err := state.NewFilePersister(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil map for missing file, got %v", got)
	}
}

func TestDebouncedPersister_FlushOnClose(t *testing.T) {
	inner := &countingPersister{}
	d := state.NewDebouncedPersister(inner, 1*time.Hour) // long interval — only Close should flush

	for i := 0; i < 10; i++ {
		_ = d.Save(map[int64]state.UserData{int64(i): {HowamiAnswer: i}})
	}

	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	saves := inner.snapshot()
	if len(saves) != 1 {
		t.Fatalf("expected exactly 1 inner save (final flush), got %d", len(saves))
	}
	if _, ok := saves[0][9]; !ok {
		t.Error("final flush should carry the last submitted snapshot (user 9)")
	}
}

func TestDebouncedPersister_PeriodicFlushCoalesces(t *testing.T) {
	inner := &countingPersister{}
	d := state.NewDebouncedPersister(inner, 30*time.Millisecond)
	defer d.Close()

	for i := 0; i < 100; i++ {
		_ = d.Save(map[int64]state.UserData{1: {HowamiAnswer: i}})
	}

	time.Sleep(120 * time.Millisecond)

	saves := inner.snapshot()
	if len(saves) < 1 {
		t.Errorf("expected at least 1 inner save, got 0")
	}
	if len(saves) > 10 {
		t.Errorf("expected coalescing (<=10 inner saves), got %d", len(saves))
	}
}

func TestDebouncedPersister_NoSavesIfClean(t *testing.T) {
	inner := &countingPersister{}
	d := state.NewDebouncedPersister(inner, 10*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := len(inner.snapshot()); got != 0 {
		t.Errorf("expected 0 inner saves with no Save calls, got %d", got)
	}
}

func TestDebouncedPersister_DoubleCloseSafe(t *testing.T) {
	d := state.NewDebouncedPersister(&countingPersister{}, 1*time.Hour)
	if err := d.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

type countingPersister struct {
	mu    sync.Mutex
	saved []map[int64]state.UserData
}

func (c *countingPersister) Load() (map[int64]state.UserData, error) { return nil, nil }
func (c *countingPersister) Save(m map[int64]state.UserData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.saved = append(c.saved, m)
	return nil
}
func (c *countingPersister) Close() error { return nil }

func (c *countingPersister) snapshot() []map[int64]state.UserData {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]map[int64]state.UserData, len(c.saved))
	copy(out, c.saved)
	return out
}
