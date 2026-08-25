# internal/journey

The multi-phase interaction framework. Owns the user-facing flow.

## Responsibility

Hold the `Phase` interface, the `Outcome` value type, the `Runner` that dispatches text + reminder ticks to phases, and the concrete phases of both modes:

- **FODMAP diary**: defecation → product category → product choice → stage choice → stage check-in.
- **ADHD self-check** (`scr_*` phases): consent → intro → ASRS-A → gate → ASRS-B → gate → WURS wording form → WURS-25 → gate → onset (+age) → life domains ×2 (one short yes/no question per domain) → result → referral → doctor report; plus the delete-confirmation phase.
- **Mood self-check** (`mood_*` phases, PHQ-9): consent (also the resume gate) → 9 questions one by one → deterministic crisis card when item 9 > 0 → result (score, band, retest delta, contacts) → doctor report; plus the delete-confirmation phase.

`/start` lands on `ModeChoicePhase` (the three-way fork). The screening phases receive a `*screening.Content` / `*screening.MoodContent` via their constructors; `journey.New` is unchanged.

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
func (r *Runner) HandleStart(s router.Sender, msg *tgbotapi.Message)   // /start → mode fork (legacy path when the fork is unregistered)
func (r *Runner) HandleText(s router.Sender, msg *tgbotapi.Message)    // generic text dispatcher
func (r *Runner) HandleAbout / HandleReport / HandleAbandon
func (r *Runner) HandleAdhd / HandleAdhdDelete                          // ADHD entry / data deletion (no-ops when unwired)
func (r *Runner) HandleMood / HandleMoodDelete                          // mood entry / data deletion (no-ops when unwired)
func (r *Runner) IsJourneyState(userID int64) bool
func (r *Runner) Remind()  // called by reminder.Worker on every tick

// Concrete phases:
func NewDefecationPhase(), NewProductCategoryPhase(), NewProductChoicePhase(),
    NewStageChoicePhase(), NewStageCheckinPhase()
func NewModeChoicePhase()
// Screening phases (all take *screening.Content):
func NewScrConsentPhase(c), NewScrIntroPhase(c),
    NewScrAsrsAPhase(c), NewScrAsrsAGatePhase(c), NewScrAsrsBPhase(c), NewScrAsrsBGatePhase(c),
    NewScrWursFormPhase(c), NewScrWursPhase(c), NewScrWursGatePhase(c),
    NewScrOnsetPhase(c), NewScrOnsetAgePhase(c),
    NewScrDomainsAdultPhase(c), NewScrDomainsChildPhase(c),
    NewScrReferralPhase(c), NewScrReportPhase(c), NewScrDeleteConfirmPhase(c)
// Mood phases (all take *screening.MoodContent):
func NewMoodConsentPhase(mc), NewMoodQuestionPhase(mc), NewMoodCrisisPhase(mc),
    NewMoodReportPhase(mc), NewMoodDeleteConfirmPhase(mc)
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
| `ModeChoicePhase` | `StateAwaitingModeChoice` | /start fork, three buttons. The FODMAP button owns the legacy /start semantics (interrupt + reset); both self-check buttons leave the diary intact. Silent `Remind` — users parked on the fork get no nudges (accepted trade-off). |
| `ScrConsentPhase` … `ScrDeleteConfirmPhase` | `scr_*` | The ADHD self-check chain. All `Remind`s are empty — the reminder loop never selects `scr_*` states by construction (pinned by test). Texts are assembled from `screening.Content` and sent via the `scr.text` pass-through i18n key. |
| `MoodConsentPhase` | `mood_consent` | Consent for fresh runs (short body + disclaimer); resume gate (Continue / start over / later) when an unfinished `Mood` exists — consent is never re-asked. Declining creates nothing. |
| `MoodQuestionPhase` | `mood_question` | Index-driven series of the 9 official PHQ-9 questions (position derived from `len(Mood.Answers)`), 4-option scale keyboard. An answer > 0 on the crisis item (q9) transitions to `mood_crisis` immediately; the last non-crisis answer finalizes. |
| `MoodCrisisPhase` | `mood_crisis` | Deterministic crisis card: warm lead + adult support contacts, plus one direct line when the answer was 2–3. Not blocking — Continue proceeds to the result. Pause here resumes here. |
| `MoodReportPhase` | `mood_report` | Transit: sends the doctor report, returns to `ReturnState` (re-Setup) or idle. |
| `MoodDeleteConfirmPhase` | `mood_delete_confirm` | `/mood_delete` confirmation; confirm wipes `Mood` + `MoodResult` in one `Set`. |

## Screening-specific contracts

- **Privacy**: raw per-question answers live only in `UserData.Screening` (transient); completion writes `ScreeningResult` and wipes `Screening` in the same `Set`. The doctor report is rendered on the fly and never stored. The WURS wording form (m/f) never reaches the result.
- **Detour bookkeeping**: `/start` or `/adhd` from a FODMAP state records `ReturnState`; every screening exit (pause, decline, finish, delete, `/abandon`) returns there (re-firing that phase's Setup — which restarts the stage timer, an accepted trade-off pinned by test) or to idle, clearing `ReturnState`.
- **Pause/resume**: gates set `Screening.ResumeState`; `/start` mid-screening records the current state there. `/adhd` resumes via the intro in resume mode without re-asking consent. "Start" on the resume intro means "start over" (wipes raw answers, keeps the previous result until a new completion).
- **Life domains one at a time**: each domain of each pass is a single short message (position line + title + one "e.g.:" line + yes/no), driven by the `AdultDomainIdx` / `ChildDomainIdx` cursor exactly like the ASRS/WURS question series — no multi-select, no redrawn walls of text. Only "yes" ids are kept; the ≥2-domains scoring rule and the stored result shape are unchanged. A `/start` or pause mid-section resumes on the exact domain.
- **Lean texts** (owner decision): no methodology caveats (translation status, validation notes, criterion-E hedging) anywhere user-visible. The intro and result carry only the short screening-not-a-diagnosis disclaimer; the result footer adds the single compact `results.attribution_line`; the referral is a two-line route to a specialist.
- **No combined score**: each instrument renders its own block with its own threshold; the overall wording maps `screening.OverallVerdict` keys onto content templates.
- **Reply keyboards** (v1 compromise): the user's taps stay visible in their Telegram chat history; the bot neither reads nor stores it. Inline buttons + CallbackQuery support in `internal/router` would remove that trace — a v2 privacy improvement, out of scope here.
- **No nudges / no TTL** for unfinished screenings in v1 — a future extension point.

## Mood-specific contracts (PHQ-9)

- **Crisis protocol is deterministic and code-driven**: any answer > 0 on item 9 → the crisis card right after the answer (never delayed to the result); answer 2–3 adds the one direct talk-to-someone-today line; the contacts block is repeated in the final result whenever the item-9 flag is set, regardless of the total score. The test is never blocked by the card. All branches are unit-tested.
- **Privacy**: raw answers live only in `UserData.Mood` (transient); completion writes `MoodResult` (score, band id, date, item-9 flag — the single per-question fact kept) and wipes `Mood` in the same `Set`. The doctor report is rendered on the fly and never stored.
- **Retest dynamics**: on a repeat completion the result renders a delta line against the previous stored result (`сегодня` / days / weeks ago) before overwriting it, plus the fixed repeat-in-2–4-weeks line.
- **Detour and resume**: same bookkeeping as `scr_*` — `ReturnState` for the FODMAP detour, `Mood.ResumeState` for `/start` mid-test (a run paused on the crisis card resumes on the card). The consent phase doubles as the resume gate; "start over" wipes only the raw answers.
- **Command naming**: `/mood` + `/mood_delete` (not `/depression`) — matches the user-facing mode name «Самопроверка настроения», avoids a self-labeling diagnosis word in the command menu, and pairs with `/adhd`/`/adhd_delete`.

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

`internal/state, products, settings, i18n, screening, router` + `tgbotapi`. Top of the internal stack — only `bot` imports it.
