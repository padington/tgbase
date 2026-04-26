package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level app config loaded from YAML.
type Config struct {
	State    StateConfig    `yaml:"state"`
	Reminder ReminderConfig `yaml:"reminder"`
}

type StateConfig struct {
	FlushInterval Duration `yaml:"flush_interval"`
}

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
