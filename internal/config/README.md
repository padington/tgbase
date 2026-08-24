# internal/config

Boot-time YAML config loader.

## Responsibility

Load `config.yaml` (or `$CONFIG_PATH`) and decode into a `Config` struct. Used **only** by `cmd/bot/main.go` to populate `bot.Config`.

## Public API

```go
type Config struct {
    State     StateConfig
    Bootstrap BootstrapConfig
    Reminder  ReminderConfig  // legacy; superseded by settings.Store
}

type StateConfig struct      { FlushInterval Duration }
type BootstrapConfig struct  { ProductsSeedPath, SettingsSeedPath, I18nDir, ScreeningDir string }
type ReminderConfig struct   { ScanInterval, ReminderAfter Duration }   // deprecated

type Duration time.Duration  // YAML decodes "200ms" / "1m" via UnmarshalYAML

func Load(path string) (*Config, error)  // path="" → "config.yaml"
```

## What lives here vs `settings.Store`

- **Boot-only** values (state flush interval, seed paths) → here. Consumed once and never reread.
- **Runtime-mutable** values (reminder timings, default locale) → `internal/settings`.

`ScreeningDir` (like `I18nDir`) is not a seed: the screening content YAMLs
are read-only and reloaded on every boot, never copied into the backend.
Empty `screening_dir` disables the screening mode.

The `Reminder` block is a vestige from the legacy survey flow. Treat it as deprecated; new tunables go in `settings.Settings`.

## When to edit

- **New boot path / interval** → add a field, update the YAML schema docs in the top-level README.
- **New runtime knob** → it goes in `internal/settings`, not here.

## Dependencies

Standard library + `gopkg.in/yaml.v3`. No internal imports.
