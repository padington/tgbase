# internal/bot

Composition root. The only package that imports every other internal package and wires them together.

## Responsibility

- Build `tgbotapi.BotAPI` from token.
- Build `store.Backend` from `Config.DataDir` (or fall back to memory).
- Construct typed stores (`state.Store`, `products.Catalog`, `settings.Store`).
- Construct `journey.Runner`, register all phases, register router handlers.
- Self-register the Telegram command menu (`setMyCommands`) on boot.
- Drive the update loop and the reminder worker.

## Public API

```go
type Config struct { Token, Env, DataDir, DataPath, I18nDir, ProductsSeedPath, SettingsSeedPath, ScreeningDir string; Debug bool; Timeout int; StateFlushInterval time.Duration }
func New(cfg Config) (*Bot, error)
func (b *Bot) Run(ctx context.Context) error
```

## Screening mode wiring

`ScreeningDir` non-empty → `screening.Load` + `screening.LoadMood` run at
startup (fail-fast on any content deviation, including the crisis-card
contacts), the landing (mode-choice) + all `scr_*` and `mood_*` phases are
registered, and the `/adhd` / `/adhd_delete` / `/mood` / `/mood_delete`
commands are installed. Empty `ScreeningDir` → none of that happens and
`/start` keeps its legacy direct-to-diary behavior (`Runner.HandleStart`
falls back when the fork phase is unregistered).

`/menu` is registered as an alias of `/start` (`runner.HandleStart`), not as
a meta command — both escape to the landing from any state.

## Command menu self-registration (commands.go)

`New` calls `registerCommands` at the end of wiring: two `setMyCommands`
payloads (default scope with ru = default-locale descriptions, plus a
`language_code="en"` override) built by `menuConfigs` from i18n keys
`cmd.<name>.desc`. The menu lists, in usage-frequency order: `menu`, `adhd`,
`mood`, `report`, `about`, `abandon`, `adhd_delete`, `mood_delete` — the
adhd/mood entries only when the corresponding mode is wired, so the client
hint always matches the running binary. `/start` is omitted (Telegram shows
its own Start button); `/ping` and `/whoami` are operator commands and stay
out of the menu. Registration is **best-effort**: an API error is logged as a
warning and never prevents boot.

## Backend selection (buildBackend)

```
DataDir set     → FileBackend (atomic tmp+rename, one .json per key)
DataPath only   → derive dir, FileBackend
both empty      → MemoryBackend (warns)
StateFlushInterval > 0 → wrap with DebouncedBackend
```

## When to edit

- **Adding a new phase** → register it here under `runner.Register(...)`. Order doesn't matter (phases keyed by `State()`).
- **New router handler** → add to the same block as `/start`, `/about`, etc.
- **New typed store** built on top of `store.Backend` → construct here, pass into `journey.New`.
- **Changing backend selection** → `buildBackend` is the only place that picks an implementation.

## Dependencies

Imports everything. Nothing imports `internal/bot`.
