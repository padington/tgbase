package i18n_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/padington/tgbase/internal/i18n"
)

func writeBundle(t *testing.T, dir, locale, content string) {
	t.Helper()
	path := filepath.Join(dir, locale+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_LooksUpInRequestedLocale(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "en", "greeting: Hello\n")
	writeBundle(t, dir, "ru", "greeting: Привет\n")

	tr, err := i18n.Load(dir, "en")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := tr.T("greeting", "ru", nil); got != "Привет" {
		t.Errorf("ru greeting: got %q", got)
	}
	if got := tr.T("greeting", "en", nil); got != "Hello" {
		t.Errorf("en greeting: got %q", got)
	}
}

func TestLoad_FallsBackToDefaultOnMissingKey(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "en", "greeting: Hello\nbye: Goodbye\n")
	writeBundle(t, dir, "ru", "greeting: Привет\n")

	tr, _ := i18n.Load(dir, "en")
	if got := tr.T("bye", "ru", nil); got != "Goodbye" {
		t.Errorf("expected fallback to en, got %q", got)
	}
}

func TestLoad_ReturnsKeyWhenAllMiss(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "en", "greeting: Hello\n")

	tr, _ := i18n.Load(dir, "en")
	if got := tr.T("absent", "en", nil); got != "absent" {
		t.Errorf("expected key verbatim, got %q", got)
	}
}

func TestLoad_DefaultLocaleMissingIsError(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "ru", "greeting: Привет\n")

	if _, err := i18n.Load(dir, "en"); err == nil {
		t.Fatal("expected error when default locale bundle missing")
	}
}

func TestT_SubstitutesPlaceholders(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "en", `prompt: "Take {value} {unit} of {name}."`+"\n")

	tr, _ := i18n.Load(dir, "en")
	args := map[string]any{"value": 0.25, "unit": "piece", "name": "Apple"}
	got := tr.T("prompt", "en", args)
	want := "Take 0.25 piece of Apple."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestT_NilArgsLeavesPlaceholdersIntact(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "en", `greet: "Hi {name}"`+"\n")

	tr, _ := i18n.Load(dir, "en")
	got := tr.T("greet", "en", nil)
	if got != "Hi {name}" {
		t.Errorf("expected literal placeholder, got %q", got)
	}
}

func TestHas_DistinguishesPresenceFromFallback(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "en", "greeting: Hello\n")
	writeBundle(t, dir, "ru", "greeting: Привет\n")

	tr, _ := i18n.Load(dir, "en")
	if !tr.Has("greeting", "ru") {
		t.Error("ru should have greeting")
	}
	if tr.Has("absent", "ru") {
		t.Error("ru should not have absent key (Has must not fall back)")
	}
}
