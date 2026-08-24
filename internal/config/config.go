package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level app config loaded from YAML.
type Config struct {
	State     StateConfig     `yaml:"state"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
	Reminder  ReminderConfig  `yaml:"reminder"` // legacy — superseded by settings.Store; kept until bot wiring switches
}

type StateConfig struct {
	FlushInterval Duration `yaml:"flush_interval"`
}

// BootstrapConfig points at the seed files that populate runtime stores
// on first boot (when the backend is empty). After the first boot the
// backend is the source of truth and these paths are no longer consulted.
type BootstrapConfig struct {
	ProductsSeedPath string `yaml:"products_seed_path"`
	SettingsSeedPath string `yaml:"settings_seed_path"`
	I18nDir          string `yaml:"i18n_dir"`
	// ScreeningDir holds the read-only ADHD-screening content YAMLs.
	// Unlike the seeds above it is consulted on every boot (like I18nDir):
	// the content is never copied into the backend. Empty = screening
	// mode disabled.
	ScreeningDir string `yaml:"screening_dir"`
}

// ReminderConfig is the legacy reminder block. Deprecated: settings live in
// settings.Store now. Kept here so existing bot wiring still parses.
type ReminderConfig struct {
	ScanInterval  Duration `yaml:"scan_interval"`
	ReminderAfter Duration `yaml:"reminder_after"`
}

// Duration wraps time.Duration so YAML can decode strings like "200ms" or "1m".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// Load reads YAML from path. If path is empty, defaults to "config.yaml".
func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &c, nil
}
