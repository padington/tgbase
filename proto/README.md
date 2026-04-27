# proto

Canonical IDL schemas for persisted shapes.

## Responsibility

Mirror the runtime Go types for `state.UserData`, `products.Product`/`Category`, `settings.Settings`, and `i18n` bundles, so the schema is documented and future migration tools (or alternative consumers) have a contract.

## Files

| Proto | Generated to | Mirrors |
|---|---|---|
| `products.proto` | `internal/products/pb/products.pb.go` | `products.Product`, `products.Category`, plus `FodmapLevel`/`Measure`/`Stage` enums |
| `state.proto` | `internal/state/pb/state.pb.go` | `state.UserData`, `state.StateKind` enum |
| `settings.proto` | `internal/settings/pb/settings.pb.go` | `settings.Settings` |
| `i18n.proto` | `internal/i18n/pb/i18n.pb.go` | i18n bundle shape |

## Important caveat

**The Go runtime does not currently use the generated `.pb.go` types.** Persistence goes through plain `encoding/json` on the Go structs. The protos are kept in sync by hand to serve as schema documentation; if the Go struct changes, the `.proto` should too.

If you ever switch persistence to `protojson`, this becomes load-bearing rather than documentation.

## Regenerating

```bash
make proto
```

Requires `protoc` (homebrew: `brew install protobuf`) and `protoc-gen-go` (`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`). The Makefile PATHs `$(go env GOPATH)/bin` so the plugin is found regardless of shell config.

The generated `*.pb.go` files are committed under `internal/<pkg>/pb/` so contributors don't need protoc locally.

## Commit discipline

`.proto` edits + the regenerated `*.pb.go` go in the **same commit** (per the table in `CLAUDE.md`).

## When to edit

- **Add a field on a persisted struct** → add to both the Go struct (in its package) and the `.proto`, then regen. Two separate commits per the per-package rule: one for proto, one for the package.
- **Add a new message** → here, then add the consuming Go struct in its package.
