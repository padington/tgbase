# internal/store

Generic key-value persistence. Bottom of the stack — imports nothing internal.

## Responsibility

Provide a `Backend` interface plus three implementations. Domain packages (`state`, `products`, `settings`) wrap a `Backend` with their own typed API so the underlying storage can be swapped (file → Redis → Postgres) without touching domain code.

## Public API

```go
type Backend interface {
    Get(key string) ([]byte, error)   // (nil, nil) means absent — not an error
    Put(key string, value []byte) error
    Close() error
}

func NewFileBackend(dir string) (*FileBackend, error)   // <dir>/<key>.json, atomic tmp+rename
func NewMemoryBackend() *MemoryBackend                  // tests + no-DATA_DIR boots
func NewDebouncedBackend(inner Backend, interval time.Duration) *DebouncedBackend
```

## Contracts implementations must keep

- `Get` of an absent key returns `(nil, nil)`, never an error.
- All methods are safe for concurrent use.
- `Close` performs any final flush (matters for `DebouncedBackend`).

## DebouncedBackend specifics

- `Put` only updates an in-memory pending map; flush happens on a ticker.
- `Get` checks `pending` first so callers see their own writes pre-flush (read-after-write within the process).
- `Close` triggers a final flush, then closes the inner backend.
- Panics if interval ≤ 0 (matches `time.NewTicker`).

## When to edit

- **Add a Redis/Postgres backend** → new struct satisfying `Backend`. No domain code changes.
- **Change file format** (e.g. compress, encrypt) → `FileBackend.Put`/`Get`.
- **Tune debounce semantics** (e.g. max-pending, sync-on-close-only) → `DebouncedBackend`.

## Dependencies

Standard library only. No internal imports.
