// Package settings exposes runtime-mutable knobs (reminder timings, default
// locale) backed by a store.Backend so they can be adjusted without a
// redeploy. On first boot the Store seeds itself from a bundled YAML.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/padington/tgbase/internal/store"
)

// Settings holds tunable parameters consumed by the reminder worker
// and the journey runner.
type Settings struct {
	DefecationReminderAfter time.Duration `json:"defecation_reminder_after"`
	CheckinInterval         time.Duration `json:"checkin_interval"`
	ScanInterval            time.Duration `json:"scan_interval"`
	DefaultLocale           string        `json:"default_locale"`
}

// yamlShape mirrors Settings but accepts duration strings ("1m", "30s") for
// human-readable YAML editing.
type yamlShape struct {
	DefecationReminderAfter string `yaml:"defecation_reminder_after" json:"defecation_reminder_after"`
	CheckinInterval         string `yaml:"checkin_interval" json:"checkin_interval"`
	ScanInterval            string `yaml:"scan_interval" json:"scan_interval"`
	DefaultLocale           string `yaml:"default_locale" json:"default_locale"`
}

func (y yamlShape) toSettings() (Settings, error) {
	parse := func(name, s string) (time.Duration, error) {
		if s == "" {
			return 0, nil
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("parse %s=%q: %w", name, s, err)
		}
		return d, nil
	}
	a, err := parse("defecation_reminder_after", y.DefecationReminderAfter)
	if err != nil {
		return Settings{}, err
	}
	c, err := parse("checkin_interval", y.CheckinInterval)
	if err != nil {
		return Settings{}, err
	}
	sc, err := parse("scan_interval", y.ScanInterval)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		DefecationReminderAfter: a,
		CheckinInterval:         c,
		ScanInterval:            sc,
		DefaultLocale:           y.DefaultLocale,
	}, nil
}

// Store wraps store.Backend with typed access to a single Settings value.
type Store struct {
	mu      sync.Mutex
	backend store.Backend
	cache   Settings
}

const backendKey = "settings"

// New loads settings from backend; if absent, seeds from seedPath and writes
// the seed back so subsequent boots use the backend.
func New(backend store.Backend, seedPath string) (*Store, error) {
	s := &Store{backend: backend}
	if err := s.load(seedPath); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load(seedPath string) error {
	raw, err := s.backend.Get(backendKey)
	if err != nil {
		return fmt.Errorf("backend get %q: %w", backendKey, err)
	}
	if raw != nil {
		var loaded Settings
		if err := json.Unmarshal(raw, &loaded); err != nil {
			return fmt.Errorf("parse backend settings: %w", err)
		}
		s.cache = loaded
		return nil
	}
	if seedPath == "" {
		return nil
	}
	data, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed %s: %w", seedPath, err)
	}
	var ys yamlShape
	if err := yaml.Unmarshal(data, &ys); err != nil {
		return fmt.Errorf("parse seed %s: %w", seedPath, err)
	}
	parsed, err := ys.toSettings()
	if err != nil {
		return err
	}
	s.cache = parsed
	return s.persistLocked()
}

func (s *Store) persistLocked() error {
	raw, err := json.Marshal(s.cache)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.backend.Put(backendKey, raw)
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache
}

// Update overwrites all fields and persists.
func (s *Store) Update(next Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = next
	return s.persistLocked()
}
