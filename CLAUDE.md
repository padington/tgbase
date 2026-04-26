# Agent instructions

## Commit discipline

Each commit must touch **one package or concern only**. Never mix changes across package boundaries in a single commit.

| Changed area | Allowed in same commit |
|---|---|
| `internal/router/` | only router files |
| `internal/store/` | only store files |
| `internal/state/` | only state files |
| `internal/products/` | only products files |
| `internal/settings/` | only settings files |
| `internal/i18n/` | only i18n files |
| `internal/journey/` | only journey files |
| `internal/reminder/` | only reminder files |
| `internal/flows/meta/` | only meta flow files |
| `internal/bot/` | only bot wiring files |
| `proto/` | proto schemas + Makefile + generated `*.pb.go` |
| `.github/workflows/` | only CI/CD files |
| `cmd/` | only entry point files |
| `README.md` / `CHANGELOG.md` / `CLAUDE.md` | docs only |

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

- `internal/router` imports only `tgbotapi`.
- `internal/store` imports nothing internal — it's the bottom of the persistence stack.
- `internal/state` imports `store` and `products` (for the `Stage` field type).
- `internal/products`, `internal/settings`, `internal/i18n` import `store` (and `i18n` is consumed by `products`).
- `internal/reminder` imports `state` and `router` (legacy survey path) and is otherwise generic over a callback.
- `internal/journey` imports `state`, `products`, `settings`, `i18n`, `router`, `tgbotapi` — it's the orchestrator.
- `internal/flows/meta` imports `router` only.
- `internal/bot` is the only composition root — it imports everything and wires it together.

When in doubt: lower-level packages must not import higher-level ones (no `store → state`, no `state → journey`, no `i18n → anything internal`).

## General

- Run `go build ./...` to verify compilation before committing.
- Keep commits small and focused — one logical change, one commit message that says why.

# Architecture

```
cmd/bot/main.go        entry point: reads env + config.yaml, starts bot, traps SIGINT/SIGTERM
internal/bot/          composition root: backend → typed stores → journey runner → router
internal/router/       Telegram update dispatcher (commands + text predicates)
internal/store/        Backend interface + FileBackend / MemoryBackend / DebouncedBackend
internal/state/        per-user UserData on top of store.Backend (key="users")
internal/products/     FODMAP catalog with metadata, mutable at runtime (key="products")
internal/settings/     reminder/check-in tunables + default locale (key="settings")
internal/i18n/         translator loaded from i18n/<locale>.yaml
internal/journey/      Phase framework: SetupPhase, DefecationPhase, ProductChoicePhase, StageCheckinPhase
internal/reminder/     scan loop with callback (calls journey.Runner.Remind)
internal/flows/meta/   stateless commands: /ping, /whoami, /menu
proto/                 canonical schemas; generated *.pb.go committed under internal/<pkg>/pb/
i18n/                  bundled UI string yamls (en.yaml, ru.yaml)
products.yaml          first-boot product catalog seed
settings.yaml          first-boot settings seed
config.yaml            boot-only paths + state flush interval
.github/workflows/     test.yml (always runs) + deploy.yml (binary/config gated)
```

## Storage abstraction

The persistence stack is layered:

1. `store.Backend` — generic `Get(key) / Put(key, value) / Close()`. `FileBackend` writes one JSON file per key under `DATA_DIR`; `DebouncedBackend` wraps any Backend and coalesces writes; `MemoryBackend` is for tests.
2. Typed stores — `state.Store`, `products.Catalog`, `settings.Store` each own one backend key (`users`, `products`, `settings`) and expose a domain-specific API.
3. Domain code (`journey`, `reminder`, `bot`) consumes only the typed APIs.

Swapping the file backend for Redis/Postgres later means writing one new struct that satisfies `store.Backend` — no domain code changes.

## Runtime mutability

Products and settings live in the backend, not in `config.yaml`. `products.yaml` and `settings.yaml` are bundled into the image only as **first-boot seeds** — once the backend has values for those keys, the YAMLs are never re-read. Future admin commands can call `Catalog.Add` / `Settings.Update` to mutate at runtime without a redeploy.

`config.yaml` shrinks to truly boot-time config: state flush interval and bootstrap paths.

## Journey

A Phase owns one StateKind and exposes three callbacks: `Setup` (run on entry), `Collect` (handle user input), `Remind` (called by reminder worker). Each returns an `Outcome` describing the next state, an i18n message key, optional keyboard buttons, and an optional `UserData` mutation.

The Runner dispatches text input + reminder ticks to the right phase by user state. Adding a new interaction = registering one Phase struct.

## Localization

Two kinds of strings:

- **UI labels / prompts / errors** — `i18n/<locale>.yaml`, baked into the image, versioned with the binary.
- **Product names + notes** — inline `name_localized` / `note_localized` maps on each `Product`, stored in the backend, runtime-mutable.

Phase Outcomes carry i18n keys + args; the runner resolves them to user-locale strings before sending. User locale is detected from `msg.From.LanguageCode` on first `/start` and persisted on `UserData.Locale`.

## Proto IDL

`proto/*.proto` files are the canonical schema for persisted shapes. Storage is JSON via `protojson` so files stay human-readable. Run `make proto` after editing a `.proto` to regenerate the committed `*.pb.go` files; the generated output is committed so contributors don't need protoc locally.

## Router contract

- `router.Sender` is the only Telegram surface handlers see (`Send(c Chattable)`); flows never touch `*tgbotapi.BotAPI` directly.
- `HandleCommand("foo", h)` registers `/foo`. `HandleText(predicate, h)` registers a predicate-driven text handler.
- Text handlers are tried in registration order; first predicate match wins.
- Stateful interactions gate themselves via predicate (the journey predicate matches when `runner.IsJourneyState(userID)`).

## Deployment

- Built into a Docker image, pushed to VPS via self-hosted runner triggered by push to `main` (binary/config-affecting paths only).
- Container runs with `BOT_ENV=vps`, `DATA_DIR=/data`, `TELEGRAM_BOT_TOKEN` from CI secret.
- The `tgbase_data` volume holds `users.json`, `products.json`, `settings.json`.
- `/whoami` reports env + hostname + os/arch — sanity check that the right binary is live.
