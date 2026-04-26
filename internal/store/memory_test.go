package store_test

import (
	"bytes"
	"testing"

	"github.com/padington/tgbase/internal/store"
)

func TestMemoryBackend_PutGetRoundTrip(t *testing.T) {
	b := store.NewMemoryBackend()
	if err := b.Put("k1", []byte("v1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := b.Get("k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, []byte("v1")) {
		t.Errorf("got %q, want %q", got, "v1")
	}
}

func TestMemoryBackend_GetAbsentReturnsNilNil(t *testing.T) {
	b := store.NewMemoryBackend()
	got, err := b.Get("missing")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for absent key, got %v", got)
	}
}

func TestMemoryBackend_PutCopiesValue(t *testing.T) {
	b := store.NewMemoryBackend()
	v := []byte("original")
	if err := b.Put("k", v); err != nil {
		t.Fatalf("Put: %v", err)
	}
	v[0] = 'X'

	got, _ := b.Get("k")
	if !bytes.Equal(got, []byte("original")) {
		t.Errorf("backend held a reference to caller's slice: got %q", got)
	}
}
