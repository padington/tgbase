# Agent instructions

## Commit discipline

Each commit must touch **one package or concern only**. Never mix changes across package boundaries in a single commit.

| Changed area | Allowed in same commit |
|---|---|
| `internal/router/` | only router files |
| `internal/state/` | only state files |
| `internal/flows/meta/` | only meta flow files |
| `internal/flows/survey/` | only survey flow files |
| `internal/bot/` | only bot wiring files |
| `.github/workflows/` | only CI/CD files |
| `cmd/` | only entry point files |
| `README.md` / `CHANGELOG.md` | docs only |

Cross-package refactors that must touch multiple packages require **one commit per package**, submitted as a sequence, not bundled.

## Test protection

- Run `go test -race ./...` locally before every commit.
- If a change breaks any existing test, **stop and notify the user before proceeding**. Do not commit broken tests under any circumstances.
- When adding a new feature, add its tests in the **same commit** as the feature code (same package commit rule still applies).
- Never delete or weaken a test to make it pass — fix the code instead.

## Flow diagram protection

`README.md` contains Mermaid state and sequence diagrams describing user-facing flows. Treat these diagrams as part of the contract — keep them in sync with the code.

When changing user-facing behavior — new commands, new states, new transitions, changed prompts, changed reminder/timeout logic, changes to what gets persisted on a transition — update the README diagrams in the same change. The diagram update lives in its own docs commit (per the table above), not bundled with the code commit, but it must land in the same branch/PR as the code that motivated it. If diagrams are out of date after your change, the work is not done.

## Package boundaries

- `internal/state` has no internal imports — keep it that way.
- `internal/router` imports only `tgbotapi` — no flow or state imports.
- Flow packages (`internal/flows/*`) may import `router` and `state`, but not each other.
- `internal/bot` is the only composition root — it imports everything and wires it together.

## General

- Run `go build ./...` to verify compilation before committing.
- Keep commits small and focused — one logical change, one commit message that says why.

# Architecture

```
cmd/bot/main.go        entry point: reads env, starts bot, traps SIGINT/SIGTERM
internal/bot/          composition root: builds router + state + handlers
internal/router/       Telegram update dispatcher (commands + text predicates)
internal/state/        thread-safe per-user state map (in-memory today)
internal/flows/meta/   stateless commands: /ping, /whoami, /menu
internal/flows/survey/ stateful /start → "How are you? 1/2/3" flow
.github/workflows/     CI/CD: deploy.yml ships to VPS via self-hosted runner on push to main
```

## State

`state.UserData`:
- `State` — `StateIdle` | `StateAwaitingHowamiAnswer`
- `HowamiAnswer` — 0 if unset, otherwise 1/2/3

`Store` is `sync.Mutex` + `map[int64]UserData` keyed by Telegram user ID. `Get` returns a value copy (zero value with `StateIdle` for unknown users) so callers can mutate freely. `Set` overwrites by value.

## Router contract

- `router.Sender` is the only Telegram surface handlers see (`Send(c Chattable)`); flows never touch `*tgbotapi.BotAPI` directly.
- `HandleCommand("foo", h)` registers `/foo`. `HandleText(predicate, h)` registers a predicate-driven text handler.
- Text handlers are tried in registration order; first predicate match wins.
- Stateful flows gate themselves via predicate (e.g. survey checks `state == StateAwaitingHowamiAnswer`).

## Flow conventions

- A flow exposes `HandlerFunc`-returning constructors that capture the store: `survey.Start(store) router.HandlerFunc`.
- Bot wiring (`internal/bot/bot.go`) registers commands + predicates on the router. Flows never reach into other flows.

## Deployment

- Built into a Docker image, pushed to VPS via self-hosted runner triggered by push to `main`.
- Container runs with `BOT_ENV=vps`, `TELEGRAM_BOT_TOKEN` from CI secret.
- `/whoami` reports env + hostname + os/arch — sanity check that the right binary is live.
