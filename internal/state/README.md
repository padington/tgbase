# internal/state

Per-user data on top of `store.Backend`. Backend key = `"users"`.

## Responsibility

Own the `UserData` struct and a thread-safe `Store` for `Get`/`Set`/state-filter lookups. Persists the entire user map as one JSON blob on every `Set`.

## Public API

```go
type StateKind string
const (
    StateIdle, StateAwaitingHowamiAnswer (legacy), StateAwaitingDefecation,
    StateAwaitingProductCategory, StateAwaitingProductChoice,
    StateAwaitingStageChoice, StateAwaitingStageCheckin,
    StateAwaitingModeChoice,                       // /start mode fork
    StateScrConsent … StateScrDeleteConfirm StateKind  // 16 scr_* screening states
    StateMoodConsent … StateMoodDeleteConfirm StateKind // 5 mood_* (PHQ-9) states
)

type DefecationKind string  // "", fluid, normal, issues
type ProductProgress struct { LastStage products.Stage; Status string; UpdatedAt time.Time }

type ScreeningProgress struct {  // TRANSIENT unfinished screening run (resume only)
    AsrsAnswers, WursAnswers []int   // raw per-question answers live ONLY here
    WursForm string; OnsetChild *bool; OnsetAge int
    AdultDomainIdx, ChildDomainIdx int   // domains answered so far (yes AND no) per pass
    AdultDomains, ChildDomains []string  // only the ids answered "yes"
    ResumeState StateKind; ConsentAt, StartedAt time.Time
}
func (p *ScreeningProgress) Clone() *ScreeningProgress  // deep copy for Outcome.Mutate

type ScreeningResult struct {    // last COMPLETED run; overwritten by each new completion
    TakenAt time.Time
    AsrsASignificant, AsrsAThreshold int; AsrsAPositive bool
    AsrsBSignificant int                     // no threshold by design
    WursScore, WursCutoff int; WursPositive bool
    OnsetChildhood bool; OnsetAge int
    AdultDomains, ChildDomains []string      // domain ids
    Verdict string; GapHint string           // wording keys, never numbers
}

type MoodProgress struct {       // TRANSIENT unfinished PHQ-9 run (resume only)
    Answers []int                // raw per-question answers (0..3) live ONLY here
    ResumeState StateKind; ConsentAt, StartedAt time.Time
}
func (p *MoodProgress) Clone() *MoodProgress   // deep copy for Outcome.Mutate

type MoodResult struct {         // last COMPLETED PHQ-9 run; overwritten by each completion
    TakenAt time.Time
    Score int                    // 0..27
    Severity string              // applied severity-band id
    Q9Positive bool              // item-9 (self-harm) answered > 0 — the only per-question fact kept
}

type UserData struct {
    State, Locale string-ish
    DefecationState, CurrentProduct, CurrentStage  // active-trial fields
    StageStartedAt, EnteredAt time.Time
    CheckinAsked, ReminderSent bool
    OfferedProducts []string                       // last keyboard slice (gate for matchOffered)
    PickerCategory string; PickerPage int          // transient picker state
    Products map[string]ProductProgress
    Screening *ScreeningProgress                   // nil when no screening in progress
    ScreeningResult *ScreeningResult               // nil until first completion
    ReturnState StateKind                          // FODMAP state to restore after a screening detour (shared by both modes)
    Mood *MoodProgress                             // nil when no mood test in progress
    MoodResult *MoodResult                         // nil until first completion
    ChatID int64
}

func NewStoreFromBackend(b store.Backend) *Store
func NewStore(p Persister) *Store                  // legacy constructor
func (s *Store) Get(userID int64) UserData         // copy; returns {State: Idle} for unseen
func (s *Store) Set(userID int64, d UserData)      // persists snapshot
func (s *Store) AllAwaitingDefecation() / AllAwaitingCheckin() / AllAwaiting() (legacy)
func (s *Store) Close() error
```

## Persisted shape

`{"<userID>": UserData, ...}` — one JSON map under the `users` backend key.

## Invariants

- `Get` returns a copy. Mutating it does NOT affect the store.
- `Set` snapshots the whole map and persists. Per-user persistence is not granular.
- `OfferedProducts` is the gate for `journey.matchOffered` — only names in this list are accepted as input.
- `PickerCategory` + `PickerPage` are meaningful only while in `StateAwaitingProductChoice` / `StateAwaitingProductCategory`. Cleared by `/start`, `/abandon`, completion.
- **Privacy invariant:** raw per-question screening answers exist only inside `Screening` (`ScreeningProgress`). Completing, restarting, `/abandon`, and `/adhd_delete` set `Screening = nil`, and `omitempty` removes the key — and the raw answers — from the persisted JSON in the same `Set`. `ScreeningResult` carries only scores + applied thresholds + facts, never answers. The WURS wording form (m/f) is never copied into the result.
- **Privacy invariant (mood):** same contract — raw PHQ-9 answers exist only inside `Mood` (`MoodProgress`); completing, restarting, `/abandon`, and `/mood_delete` set `Mood = nil` in the same `Set` that writes `MoodResult`. The result keeps only the total score, the band id, the date, and the single allowed per-question fact: the item-9 flag (needed to repeat the support contacts).
- Legacy `users.json` files without the screening/mood fields load as zero values — no migration needed.
- Mutations of `Screening` / `Mood` must go through `Clone()` (pointer fields — `Get`'s struct copy shares the pointee).

## When to edit

- **New per-user field** → add to `UserData` (with `omitempty` for transient fields), update relevant tests.
- **New state** → add to the `StateKind` const block; add the matching `Store.AllAwaiting<X>()` filter only if reminders need to scan that state.
- **Change snapshot granularity** (per-user writes) → here.

## Dependencies

`internal/products` (only for `Stage` field type), `internal/store`.
