package settings_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/store"
)

func writeSeed(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNew_SeedsFromYAMLWhenBackendEmpty(t *testing.T) {
	seed := writeSeed(t, `defecation_reminder_after: 2m
checkin_interval: 45m
scan_interval: 5s
default_locale: en
`)
	backend := store.NewMemoryBackend()

	s, err := settings.New(backend, seed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := s.Get()
	if got.DefecationReminderAfter != 2*time.Minute {
		t.Errorf("DefecationReminderAfter: got %v", got.DefecationReminderAfter)
	}
	if got.CheckinInterval != 45*time.Minute {
		t.Errorf("CheckinInterval: got %v", got.CheckinInterval)
	}
	if got.ScanInterval != 5*time.Second {
		t.Errorf("ScanInterval: got %v", got.ScanInterval)
	}
	if got.DefaultLocale != "en" {
		t.Errorf("DefaultLocale: got %q", got.DefaultLocale)
	}

	if raw, _ := backend.Get("settings"); raw == nil {
		t.Error("seed should have been written back to backend")
	}
}

func TestNew_PrefersBackendOverSeed(t *testing.T) {
	seed := writeSeed(t, "checkin_interval: 30m\n")
	backend := store.NewMemoryBackend()
	_ = backend.Put("settings", []byte(`{"checkin_interval":900000000000,"default_locale":"ru"}`))

	s, err := settings.New(backend, seed)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if got.CheckinInterval != 15*time.Minute {
		t.Errorf("expected backend value (15m), got %v", got.CheckinInterval)
	}
	if got.DefaultLocale != "ru" {
		t.Errorf("expected backend locale (ru), got %q", got.DefaultLocale)
	}
}

func TestUpdate_PersistsAndIsReadable(t *testing.T) {
	seed := writeSeed(t, "checkin_interval: 30m\n")
	backend := store.NewMemoryBackend()
	s, _ := settings.New(backend, seed)

	next := settings.Settings{
		DefecationReminderAfter: 30 * time.Second,
		CheckinInterval:         5 * time.Minute,
		ScanInterval:            1 * time.Second,
		DefaultLocale:           "ru",
	}
	if err := s.Update(next); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := s.Get().CheckinInterval; got != 5*time.Minute {
		t.Errorf("in-memory cache stale: got %v", got)
	}

	s2, _ := settings.New(backend, seed)
	if got := s2.Get().CheckinInterval; got != 5*time.Minute {
		t.Errorf("update did not persist: got %v", got)
	}
	if got := s2.Get().DefaultLocale; got != "ru" {
		t.Errorf("locale did not persist: got %q", got)
	}
}

func TestSettingsSeed_DefaultLocaleIsRu(t *testing.T) {
	s, err := settings.New(store.NewMemoryBackend(), "../../settings.yaml")
	if err != nil {
		t.Fatalf("loading real settings.yaml: %v", err)
	}
	if got := s.Get().DefaultLocale; got != "ru" {
		t.Errorf("settings.yaml default_locale: got %q, want %q", got, "ru")
	}
}

func TestNew_BadDurationFails(t *testing.T) {
	seed := writeSeed(t, "checkin_interval: not-a-duration\n")
	if _, err := settings.New(store.NewMemoryBackend(), seed); err == nil {
		t.Fatal("expected error for bad duration")
	}
}
