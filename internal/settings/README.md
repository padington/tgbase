# internal/settings

Runtime-mutable tunables (reminder timings, default locale). Backend key = `"settings"`.

## Responsibility

Hold a single `Settings` value, persisted as JSON, seeded from YAML on first boot. Consumed by the reminder worker (`ScanInterval`) and the journey runner (`DefecationReminderAfter`, `CheckinInterval`, `DefaultLocale`).

## Public API

```go
type Settings struct {
    DefecationReminderAfter time.Duration
    CheckinInterval         time.Duration
    ScanInterval            time.Duration
    DefaultLocale           string
}

func New(backend store.Backend, seedPath string) (*Store, error)
func (s *Store) Get() Settings              // copy
func (s *Store) Update(next Settings) error // overwrites all fields and persists
```

## Seed YAML shape

Durations are human-readable strings (`"1m"`, `"30s"`); `Update` writes back as JSON nanoseconds.

```yaml
defecation_reminder_after: 1m
checkin_interval: 30m
scan_interval: 10s
default_locale: en
```

## Invariants

- `Get` returns a copy; safe for concurrent reads.
- `Update` replaces ALL fields. There is no per-field setter — callers must pass a complete `Settings`.
- The reminder worker resamples `ScanInterval` every tick (via `IntervalFunc`), so `Update` takes effect on the next tick without a restart.

## When to edit

- **Add a new tunable** → field on `Settings` (with the YAML fallback handler in `yamlShape`), regenerate `proto/settings.proto` to match.
- **Add a per-field setter** → here, with appropriate locking.
- **Change seeding** (e.g. defaults from env) → `New` / `load`.

## Dependencies

`internal/store`. Standard library + `gopkg.in/yaml.v3`.
