# internal/journey

The multi-phase interaction framework. Owns the user-facing flow.

## Responsibility

Hold the `Phase` interface, the `Outcome` value type, the `Runner` that dispatches text + reminder ticks to phases, and the concrete phases (defecation → product category → product choice → stage choice → stage check-in).

## Public API

```go
type Phase interface {
    State() state.StateKind
    Setup(ctx Context) Outcome   // run on entry to the state
    Collect(ctx Context, input string) Outcome  // run on text input
    Remind(ctx Context) Outcome  // run by reminder worker; usually empty
}

type Outcome struct {
    NextState      state.StateKind   // empty = stay put
    ReplyKey       string            // i18n key; empty = silent
    ReplyArgs      map[string]any
    Keyboard       [][]string        // each inner slice = one row
    RemoveKeyboard bool              // dismiss any active Telegram keyboard
    Mutate         func(*state.UserData)
}

type Context struct { UserID int64; User UserData; Catalog *products.Catalog; Settings settings.Settings; Trans i18n.Translator; Locale i18n.Locale; Now func() time.Time }

func New(stateStore *state.Store, sender router.Sender, catalog *products.Catalog,
        settingsStore *settings.Store, trans i18n.Translator) *Runner
func (r *Runner) Register(p Phase)
func (r *Runner) HandleStart(s router.Sender, msg *tgbotapi.Message)   // /start handler
func (r *Runner) HandleText(s router.Sender, msg *tgbotapi.Message)    // generic text dispatcher
func (r *Runner) HandleAbout / HandleReport / HandleAbandon
func (r *Runner) IsJourneyState(userID int64) bool
func (r *Runner) Remind()  // called by reminder.Worker on every tick

// Concrete phases:
func NewDefecationPhase(), NewProductCategoryPhase(), NewProductChoicePhase(),
    NewStageChoicePhase(), NewStageCheckinPhase()
```

## Runner contracts (important)

- **Setup re-fires whenever `Outcome.NextState` is set, including same-state outcomes.** This is how the picker refreshes its keyboard after Prev/Next paging. A phase that wants to "stay put" must NOT set `NextState`.
- `applyOutcome` chains: `Mutate` → store snapshot → send reply → if transitioned, call next phase's Setup with the same chatID. Multiple chained Setups (e.g. category auto-skip → choice) result in a single user-visible message (only the last reply has a `ReplyKey`).
- `Context.User` is a copy. Mutations must go through `Outcome.Mutate`.

## Phase responsibilities

| Phase | State | Notes |
|---|---|---|
| `DefecationPhase` | `StateAwaitingDefecation` | 1/2/3 keyboard; reminder nudge after `Settings.DefecationReminderAfter`. |
| `ProductCategoryPhase` | `StateAwaitingProductCategory` | 2-col category grid; auto-skips when only 1 bucket has products; transitions to Idle (with "exhausted" reply) when none remain. |
| `ProductChoicePhase` | `StateAwaitingProductChoice` | 4×3 paged grid scoped to `User.PickerCategory`, plus Back / Prev / Next. Bounces to category state when category is empty or unset. |
| `StageChoicePhase` | `StateAwaitingStageChoice` | 3-button volume picker (low/med/high). Renders the product's localized note (when set) as a "💡 …" line above the keyboard so the user can read prep / pathway / swap context before committing to a dose. |
| `StageCheckinPhase` | `StateAwaitingStageCheckin` | yes/no; advances stage, completes product, or marks not_tolerated. Reminder prompts the check-in question after `Settings.CheckinInterval`. |

## Picker label rendering

- Category button: `<emoji> <localized name>` from `products.Category` (or raw id when unknown).
- Product button: FODMAP-level indicator + localized name. 🔴 = high, 🟠 = moderate, 🟢 = low, plain name for unspecified.
- `matchOffered` accepts the raw name, the localized name, OR the rendered button label (so taps and typed inputs both work).

## When to edit

- **New step in the conversation** → new phase struct + `state.StateKind` const + register in `bot.go` (and tests' `setup`).
- **Change layout** (rows/cols) → constants at the top of `category.go` / `product.go`.
- **Change paging behavior** → `ProductChoicePhase.Setup` (page clamping) + nav row helper.
- **Change the runner's transition rule** → `applyOutcome` in `runner.go`. Audit every phase's `Outcome` returns first.

## Dependencies

`internal/state, products, settings, i18n, router` + `tgbotapi`. Top of the internal stack — only `bot` imports it.
