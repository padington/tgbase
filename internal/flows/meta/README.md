# internal/flows/meta

Stateless meta commands that don't touch the journey state machine.

## Responsibility

`/ping`, `/whoami`, `/menu` — sanity-check commands that work regardless of user state.

## Public API

```go
func Ping() router.HandlerFunc
func Whoami(env string) router.HandlerFunc   // env captured at construction
func Menu() router.HandlerFunc               // sends a reply keyboard with [Ping][Whoami]
```

## Behavior

- `Ping` → replies `pong` (with `ReplyToMessageID`).
- `Whoami` → replies env label + hostname + `runtime.GOOS`/`GOARCH`. Used to verify which binary is live on the VPS.
- `Menu` → sends `"Choose an action:"` with a 2-button reply keyboard.

## When to edit

- **New stateless command** → add a constructor here, register in `internal/bot/bot.go`.
- **Anything that needs `state.UserData` / `journey.Runner`** → it doesn't belong here. Add it as a runner method or a journey phase.

## Dependencies

`internal/router` + `tgbotapi`. Nothing else internal.
