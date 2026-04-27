# internal/reminder

Scan loop that drives time-based nudges.

## Responsibility

Call a `ScanFunc` once every `IntervalFunc()` until `ctx` is cancelled. The worker is intentionally generic — it doesn't know who to nudge or what to say. The scan callback (typically `journey.Runner.Remind`) walks per-user state and decides.

## Public API

```go
type ScanFunc func()
type IntervalFunc func() time.Duration

func NewWithCallback(scan ScanFunc, intervalFunc IntervalFunc) *Worker
func (w *Worker) Run(ctx context.Context)   // blocks until ctx done
func (w *Worker) Tick()                     // one scan pass; useful in tests

// Legacy: New(store, sender, Config) — survey-flow nudge logic kept until
// callers migrate. Deprecated; use NewWithCallback.
```

## Contracts

- `IntervalFunc` is sampled **every iteration**, so a runtime change to `settings.Settings.ScanInterval` takes effect on the next tick — no restart needed.
- If `IntervalFunc` returns ≤ 0, the worker falls back to `Config.ScanInterval` (legacy) or 30s.
- `Run` exits cleanly on `ctx.Done()`.

## When to edit

- **Add a new nudge category** → it goes in the journey runner's `Remind`, not here. The worker is generic by design.
- **Add a max-frequency throttle** → here.
- **Replace the ticker with cron-like scheduling** → here.

## Dependencies

`internal/router` + `internal/state` (legacy survey path only) + `tgbotapi`. The `NewWithCallback` path uses none of them.
