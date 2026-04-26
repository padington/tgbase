package state

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Persister loads and saves the state map. Save may be asynchronous; callers
// that need durability before exit must call Close.
type Persister interface {
	Load() (map[int64]UserData, error)
	Save(map[int64]UserData) error
	Close() error
}

// MemoryPersister is a no-op persister used in tests and when no DataPath is set.
type MemoryPersister struct{}

func (MemoryPersister) Load() (map[int64]UserData, error) { return nil, nil }
func (MemoryPersister) Save(map[int64]UserData) error     { return nil }
func (MemoryPersister) Close() error                      { return nil }

// FilePersister stores the map as JSON on disk, written atomically via tmp+rename.
type FilePersister struct {
	path string
}

func NewFilePersister(path string) *FilePersister {
	return &FilePersister{path: path}
}

func (f *FilePersister) Load() (map[int64]UserData, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", f.path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var m map[int64]UserData
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", f.path, err)
	}
	return m, nil
}

func (f *FilePersister) Save(m map[int64]UserData) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("rename %s: %w", f.path, err)
	}
	return nil
}

func (f *FilePersister) Close() error { return nil }

// DebouncedPersister coalesces Save calls and flushes to inner at most once
// per interval. Close performs a final flush and stops the background loop.
type DebouncedPersister struct {
	inner    Persister
	interval time.Duration

	mu      sync.Mutex
	pending map[int64]UserData
	dirty   bool

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// NewDebouncedPersister panics if interval <= 0 (programming error,
// matches time.NewTicker semantics).
func NewDebouncedPersister(inner Persister, interval time.Duration) *DebouncedPersister {
	if interval <= 0 {
		panic("state: DebouncedPersister interval must be > 0")
	}
	d := &DebouncedPersister{
		inner:    inner,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go d.run()
	return d
}

func (d *DebouncedPersister) Load() (map[int64]UserData, error) {
	return d.inner.Load()
}

func (d *DebouncedPersister) Save(m map[int64]UserData) error {
	d.mu.Lock()
	d.pending = m
	d.dirty = true
	d.mu.Unlock()
	return nil
}

func (d *DebouncedPersister) run() {
	defer close(d.done)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.flush()
		case <-d.stop:
			d.flush()
			return
		}
	}
}

func (d *DebouncedPersister) flush() {
	d.mu.Lock()
	if !d.dirty {
		d.mu.Unlock()
		return
	}
	m := d.pending
	d.pending = nil
	d.dirty = false
	d.mu.Unlock()
	if err := d.inner.Save(m); err != nil {
		log.Printf("state: debounced save: %v", err)
	}
}

func (d *DebouncedPersister) Close() error {
	d.stopOnce.Do(func() {
		close(d.stop)
		<-d.done
	})
	return d.inner.Close()
}
