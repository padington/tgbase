package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ParsesDurations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := writeFile(path, `
state:
  flush_interval: 250ms
reminder:
  scan_interval: 5s
  reminder_after: 90s
`); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := time.Duration(cfg.State.FlushInterval), 250*time.Millisecond; got != want {
		t.Errorf("FlushInterval = %v, want %v", got, want)
	}
	if got, want := time.Duration(cfg.Reminder.ScanInterval), 5*time.Second; got != want {
		t.Errorf("ScanInterval = %v, want %v", got, want)
	}
	if got, want := time.Duration(cfg.Reminder.ReminderAfter), 90*time.Second; got != want {
		t.Errorf("ReminderAfter = %v, want %v", got, want)
	}
}

func TestLoad_BadDuration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := writeFile(path, "state:\n  flush_interval: not-a-duration\n"); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load: expected error for invalid duration, got nil")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("Load: expected error for missing file, got nil")
	}
}

func TestLoad_ParsesBootstrap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := writeFile(path, `
state:
  flush_interval: 200ms
bootstrap:
  products_seed_path: /products.yaml
  settings_seed_path: /settings.yaml
  i18n_dir: /i18n
`); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bootstrap.ProductsSeedPath != "/products.yaml" {
		t.Errorf("products path: got %q", cfg.Bootstrap.ProductsSeedPath)
	}
	if cfg.Bootstrap.SettingsSeedPath != "/settings.yaml" {
		t.Errorf("settings path: got %q", cfg.Bootstrap.SettingsSeedPath)
	}
	if cfg.Bootstrap.I18nDir != "/i18n" {
		t.Errorf("i18n dir: got %q", cfg.Bootstrap.I18nDir)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
