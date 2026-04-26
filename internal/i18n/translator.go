// Package i18n provides UI string lookup keyed by locale. Translation tables
// are flat key→string YAML files loaded from disk, one file per locale.
package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Locale is an IETF-style language tag (e.g. "en", "ru"). Empty string means
// "use the default" — Translator falls back accordingly.
type Locale string

// Translator looks up UI strings.
type Translator interface {
	// T returns the translated value for key in locale. If the key is missing
	// in locale, falls back to the default locale; if still missing, returns
	// the key verbatim so the bug is visible.
	// args can be nil; non-nil values replace {placeholder} tokens.
	T(key string, locale Locale, args map[string]any) string
	Has(key string, locale Locale) bool
}

type loader struct {
	bundles       map[Locale]map[string]string
	defaultLocale Locale
}

// Load reads every *.yaml file in dir as a locale bundle. Each file's basename
// (minus the extension) is the locale identifier.
func Load(dir string, defaultLocale Locale) (Translator, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read i18n dir %s: %w", dir, err)
	}
	bundles := make(map[Locale]map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		locale := Locale(strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml"))
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var entries map[string]string
		if err := yaml.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		bundles[locale] = entries
	}
	if _, ok := bundles[defaultLocale]; !ok {
		return nil, fmt.Errorf("default locale %q not found in %s", defaultLocale, dir)
	}
	return &loader{bundles: bundles, defaultLocale: defaultLocale}, nil
}

func (l *loader) T(key string, locale Locale, args map[string]any) string {
	template, ok := l.lookup(key, locale)
	if !ok {
		return key
	}
	return substitute(template, args)
}

func (l *loader) Has(key string, locale Locale) bool {
	_, ok := l.lookup(key, locale)
	return ok
}

func (l *loader) lookup(key string, locale Locale) (string, bool) {
	if locale != "" {
		if bundle, ok := l.bundles[locale]; ok {
			if v, ok := bundle[key]; ok {
				return v, true
			}
		}
	}
	if bundle, ok := l.bundles[l.defaultLocale]; ok {
		if v, ok := bundle[key]; ok {
			return v, true
		}
	}
	return "", false
}

func substitute(template string, args map[string]any) string {
	if len(args) == 0 {
		return template
	}
	out := template
	for k, v := range args {
		out = strings.ReplaceAll(out, "{"+k+"}", fmt.Sprint(v))
	}
	return out
}
