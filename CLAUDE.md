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

## Package boundaries

- `internal/state` has no internal imports — keep it that way.
- `internal/router` imports only `tgbotapi` — no flow or state imports.
- Flow packages (`internal/flows/*`) may import `router` and `state`, but not each other.
- `internal/bot` is the only composition root — it imports everything and wires it together.

## General

- Run `go build ./...` to verify compilation before committing.
- Keep commits small and focused — one logical change, one commit message that says why.
