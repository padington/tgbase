package store

import (
	"log"
	"sync"
	"time"
)

// DebouncedBackend coalesces Put calls and flushes to inner at most once per
// interval. Get checks pending writes first so callers see their own writes
// before they reach disk. Close performs a final flush and stops the loop.
type DebouncedBackend struct {
	inner    Backend
	interval time.Duration

	mu      sync.Mutex
	pending map[string][]byte

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// NewDebouncedBackend panics if interval <= 0 (programming error,
// matches time.NewTicker semantics).
func NewDebouncedBackend(inner Backend, interval time.Duration) *DebouncedBackend {
	if interval <= 0 {
		panic("store: DebouncedBackend interval must be > 0")
	}
	d := &DebouncedBackend{
		inner:    inner,
		interval: interval,
		pending:  make(map[string][]byte),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go d.run()
	return d
}

func (d *DebouncedBackend) Get(key string) ([]byte, error) {
	d.mu.Lock()
	if v, ok := d.pending[key]; ok {
		out := make([]byte, len(v))
		copy(out, v)
		d.mu.Unlock()
		return out, nil
	}
	d.mu.Unlock()
	return d.inner.Get(key)
}

func (d *DebouncedBackend) Put(key string, value []byte) error {
	d.mu.Lock()
	stored := make([]byte, len(value))
	copy(stored, value)
	d.pending[key] = stored
	d.mu.Unlock()
	return nil
}

func (d *DebouncedBackend) run() {
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

func (d *DebouncedBackend) flush() {
	d.mu.Lock()
	if len(d.pending) == 0 {
		d.mu.Unlock()
		return
	}
	batch := d.pending
	d.pending = make(map[string][]byte)
	d.mu.Unlock()

	for key, value := range batch {
		if err := d.inner.Put(key, value); err != nil {
			log.Printf("store: debounced put %s: %v", key, err)
		}
	}
}

func (d *DebouncedBackend) Close() error {
	d.stopOnce.Do(func() {
		close(d.stop)
		<-d.done
	})
	return d.inner.Close()
}
