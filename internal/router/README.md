# internal/router

Telegram update dispatcher. The only package that talks `tgbotapi` directly to handlers.

## Responsibility

Route an incoming `tgbotapi.Update` to the right handler. Two registration kinds:

- `HandleCommand("foo", h)` — fires for `/foo`.
- `HandleText(predicate, h)` — fires when `predicate(msg)` returns true. Tried in registration order, **first match wins**.

## Public API

```go
type Sender interface { Send(c tgbotapi.Chattable) (tgbotapi.Message, error) }
type HandlerFunc func(sender Sender, msg *tgbotapi.Message)
type TextPredicate func(msg *tgbotapi.Message) bool

func New(sender Sender) *Router
func (r *Router) HandleCommand(cmd string, h HandlerFunc)
func (r *Router) HandleText(pred TextPredicate, h HandlerFunc)
func (r *Router) Dispatch(update tgbotapi.Update)
```

## Contract

- Handlers see only `Sender`, never `*tgbotapi.BotAPI`. Tests use mock senders.
- `Dispatch` ignores non-message updates (callbacks, edits, etc).
- Unknown command → logged, dropped.
- No text predicate matches → logged, dropped.

## When to edit

- **Add inline-keyboard / callback support** → here. Today only `update.Message` is consulted.
- **New registration kind** (e.g. by chat type) → extend `Router` and `Dispatch`.
- **Logging policy** → here; handlers stay quiet.

## Dependencies

Imports `tgbotapi` only. Importable by every higher layer.
