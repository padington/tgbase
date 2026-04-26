package store_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/padington/tgbase/internal/store"
)

func TestFileBackend_PutGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	b, err := store.NewFileBackend(dir)
	if err != nil {
		t.Fatalf("NewFileBackend: %v", err)
	}

	if err := b.Put("users", []byte(`{"hello":"world"}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := b.Get("users")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, []byte(`{"hello":"world"}`)) {
		t.Errorf("got %q", got)
	}
}

func TestFileBackend_GetMissingReturnsNilNil(t *testing.T) {
	b, _ := store.NewFileBackend(t.TempDir())
	got, err := b.Get("absent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestFileBackend_PutAtomicTmpRename(t *testing.T) {
	dir := t.TempDir()
	b, _ := store.NewFileBackend(dir)
	if err := b.Put("k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("tmp file leaked: %s", e.Name())
		}
	}
}

func TestFileBackend_CreatesDirIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	if _, err := store.NewFileBackend(dir); err != nil {
		t.Fatalf("expected to create %s, got %v", dir, err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("dir not created: %v", err)
	}
}

func TestFileBackend_OverwriteReplaces(t *testing.T) {
	b, _ := store.NewFileBackend(t.TempDir())
	_ = b.Put("k", []byte("first"))
	_ = b.Put("k", []byte("second"))
	got, _ := b.Get("k")
	if string(got) != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}
