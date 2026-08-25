# internal/flows/meta

Stateless meta commands that don't touch the journey state machine.

## Responsibility

`/ping`, `/whoami` — sanity-check commands that work regardless of user state.
(`/menu` is no longer a meta command: it is an alias of `/start` — the home
landing — wired in `internal/bot` to `journey.Runner.HandleStart`.)

## Public API

```go
func Ping() router.HandlerFunc
func Whoami(env string) router.HandlerFunc   // env captured at construction
```

## Behavior

- `Ping` → replies `pong` (with `ReplyToMessageID`).
- `Whoami` → replies env label + hostname + `runtime.GOOS`/`GOARCH`. Used to verify which binary is live on the VPS.

## When to edit

- **New stateless command** → add a constructor here, register in `internal/bot/bot.go`.
- **Anything that needs `state.UserData` / `journey.Runner`** → it doesn't belong here. Add it as a runner method or a journey phase.

## Dependencies

`internal/router` + `tgbotapi`. Nothing else internal.
