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

## Self-check track wiring

`ScreeningDir` non-empty → `screening.Load` + `screening.LoadMood` +
`screening.LoadEating` all run at startup, each fail-fast on any content
deviation (instrument items, thresholds, the crisis-card contacts). Then the
landing (mode-choice) plus every `scr_*`, `mood_*` and `eat_*` phase is
registered and the `/adhd`, `/adhd_delete`, `/mood`, `/mood_delete`, `/food`,
`/food_delete` commands are installed. Empty `ScreeningDir` → none of that
happens and `/start` keeps its legacy direct-to-diary behavior
(`Runner.HandleStart` falls back when the fork phase is unregistered).

One directory feeds all three tracks, so in practice they are wired together
or not at all; the wiring is still expressed per track (`phasesFor` takes one
content pointer each, nil = that track contributes nothing) so a track can be
dropped without touching the others.

`/menu` is registered as an alias of `/start` (`runner.HandleStart`), not as
a meta command — both escape to the landing from any state.

## Phase registration (phases.go)

`phasesFor(scr, mood, eat)` returns the full ordered list of phases `New`
feeds to `runner.Register`: the five FODMAP diary phases unconditionally, the
landing + 16 `scr_*` phases when screening content loaded, 11 `mood_*`
phases, 7 `eat_*` phases. It is a returned list rather than inline `Register`
calls so the wiring is unit-testable — the Runner keys phases by `State()`,
so a forgotten phase is a dead-end state and a duplicated one silently
shadows its twin, and neither surfaces until a user walks into it.

## Command menu self-registration (commands.go)

`New` calls `registerCommands` at the end of wiring: two `setMyCommands`
payloads (default scope with ru = default-locale descriptions, plus a
`language_code="en"` override) built by `menuConfigs` from i18n keys
`cmd.<name>.desc`. The menu lists, in usage-frequency order: `menu`, `adhd`,
`mood`, `food`, `report`, `about`, `abandon`, `adhd_delete`, `mood_delete`,
`food_delete` — the per-track entries only when that track is wired (the
`modes` struct carries which ones are), so the client hint always matches the
running binary. `/start` is omitted (Telegram shows its own Start button);
`/ping` and `/whoami` are operator commands and stay out of the menu.
Registration is **best-effort**: an API error is logged as a warning and
never prevents boot.

## Backend selection (buildBackend)

```
DataDir set     → FileBackend (atomic tmp+rename, one .json per key)
DataPath only   → derive dir, FileBackend
both empty      → MemoryBackend (warns)
StateFlushInterval > 0 → wrap with DebouncedBackend
```

## When to edit

- **Adding a new phase** → add it to `phasesFor` in `phases.go`. Order doesn't matter (phases keyed by `State()`), but the state must be claimed exactly once.
- **New router handler** → add to the same block as `/start`, `/about`, etc.
- **New command in the client menu** → add an entry to `menuCommands` with its `cmd.<name>.desc` key in both i18n bundles; gate it on the owning track's `modes` field if it is track-specific.
- **New typed store** built on top of `store.Backend` → construct here, pass into `journey.New`.
- **Changing backend selection** → `buildBackend` is the only place that picks an implementation.

## Dependencies

Imports everything. Nothing imports `internal/bot`.
