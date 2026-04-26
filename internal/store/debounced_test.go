package store_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/padington/tgbase/internal/store"
)

type countingBackend struct {
	mu     sync.Mutex
	puts   []putRecord
	closed bool
	store  map[string][]byte
}

type putRecord struct {
	key   string
	value []byte
}

func newCountingBackend() *countingBackend {
	return &countingBackend{store: make(map[string][]byte)}
}

func (c *countingBackend) Get(key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[key]
	if !ok {
		return nil, nil
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (c *countingBackend) Put(key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.puts = append(c.puts, putRecord{key, append([]byte(nil), value...)})
	c.store[key] = append([]byte(nil), value...)
	return nil
}

func (c *countingBackend) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *countingBackend) snapshot() []putRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]putRecord, len(c.puts))
	copy(out, c.puts)
	return out
}

func TestDebouncedBackend_FlushOnClose(t *testing.T) {
	inner := newCountingBackend()
	d := store.NewDebouncedBackend(inner, 1*time.Hour)

	for i := 0; i < 10; i++ {
		_ = d.Put("users", []byte{byte(i)})
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	puts := inner.snapshot()
	if len(puts) != 1 {
		t.Fatalf("expected 1 inner Put (final flush), got %d", len(puts))
	}
	if !bytes.Equal(puts[0].value, []byte{9}) {
		t.Errorf("expected last value 9, got %v", puts[0].value)
	}
	if !inner.closed {
		t.Error("expected inner backend to be closed")
	}
}

func TestDebouncedBackend_GetReadsPendingWrite(t *testing.T) {
	inner := newCountingBackend()
	d := store.NewDebouncedBackend(inner, 1*time.Hour)
	defer d.Close()

	_ = d.Put("k", []byte("hello"))
	got, err := d.Get("k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Errorf("expected to read pending write, got %q", got)
	}
}

func TestDebouncedBackend_PeriodicFlushCoalesces(t *testing.T) {
	inner := newCountingBackend()
	d := store.NewDebouncedBackend(inner, 30*time.Millisecond)
	defer d.Close()

	for i := 0; i < 100; i++ {
		_ = d.Put("k", []byte{byte(i)})
	}
	time.Sleep(120 * time.Millisecond)

	puts := inner.snapshot()
	if len(puts) < 1 {
		t.Errorf("expected at least 1 inner put, got 0")
	}
	if len(puts) > 10 {
		t.Errorf("expected coalescing (<=10 inner puts), got %d", len(puts))
	}
}

func TestDebouncedBackend_NoPutWhenIdle(t *testing.T) {
	inner := newCountingBackend()
	d := store.NewDebouncedBackend(inner, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if got := len(inner.snapshot()); got != 0 {
		t.Errorf("expected 0 puts with no Put calls, got %d", got)
	}
}

func TestDebouncedBackend_DoubleCloseSafe(t *testing.T) {
	d := store.NewDebouncedBackend(newCountingBackend(), 1*time.Hour)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestDebouncedBackend_FlushesMultipleKeys(t *testing.T) {
	inner := newCountingBackend()
	d := store.NewDebouncedBackend(inner, 1*time.Hour)

	_ = d.Put("users", []byte("u"))
	_ = d.Put("products", []byte("p"))
	_ = d.Put("settings", []byte("s"))
	_ = d.Close()

	keys := make(map[string]bool)
	for _, p := range inner.snapshot() {
		keys[p.key] = true
	}
	for _, want := range []string{"users", "products", "settings"} {
		if !keys[want] {
			t.Errorf("missing flushed key: %s", want)
		}
	}
}
