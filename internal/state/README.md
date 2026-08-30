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
    StateMoodConsent … StateMoodDeleteConfirm StateKind // 11 mood_* states: consent, menu,
    // PHQ-9 (question/crisis/q10/report), WHO-5 (who5_question/offer_phq9),
    // GAD-7 (gad7_question/offer_gad7), delete confirm
    StateEatConsent … StateEatDeleteConfirm StateKind   // 7 eat_* states: consent, menu,
    // one question state per instrument (edeqs/bes/nias), report, delete confirm
    StatePuConsent … StatePuDeleteConfirm StateKind     // 13 pu_* pushup-track states: consent,
    // gate, goal, variation, test, menu, set, rest, effort, week fork, red card,
    // progress, delete confirm — pu_rest is scanned by the reminder loop
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

type MoodProgress struct {       // TRANSIENT unfinished run of ONE mood-module instrument
    Answers []int                // raw per-question answers live ONLY here; PHQ-9 index 9 = functional item
    ResumeState StateKind; ConsentAt (legacy), StartedAt time.Time
}
func (p *MoodProgress) Clone() *MoodProgress   // deep copy for Outcome.Mutate

type MoodResult struct {         // last COMPLETED PHQ-9 run; overwritten by each completion
    TakenAt time.Time
    Score int                    // 0..27 (functional item NOT included)
    Severity string              // applied severity-band id
    Q9Positive bool              // item-9 (self-harm) answered > 0
    Q10Answered bool; Q10Answer int // official functional (10th) item: asked only when any answer > 0
}

type Who5Result struct {         // last COMPLETED WHO-5 quick check
    TakenAt time.Time; Score int // 0..100 (raw 0..25 sum × 4)
    Band string                  // ok | low | very_low
}

type Gad7Result struct {         // last COMPLETED GAD-7 run
    TakenAt time.Time; Score int // 0..21
    Severity string              // minimal | mild | moderate | severe
}

type EatingProgress struct {     // TRANSIENT unfinished run of ONE eating-track instrument
    Answers []int                // raw per-item answers live ONLY here (BES: the picked statement's WEIGHT)
    ResumeState StateKind; StartedAt time.Time
}
func (p *EatingProgress) Clone() *EatingProgress  // deep copy for Outcome.Mutate

type EdeqsResult struct {        // last COMPLETED EDE-QS run
    TakenAt time.Time; Score int // 0..36
    Cutoff int; Positive bool    // applied cutoff (15) + the verdict it produced
}

type BesResult struct {          // last COMPLETED BES run
    TakenAt time.Time; Score int // 0..46
    Band string                  // low | moderate | severe
}

type NiasResult struct {         // last COMPLETED NIAS run — three subscales, NO total
    TakenAt time.Time
    Picky, Appetite, Fear int                       // 0..15 each
    PickyCutoff, AppetiteCutoff, FearCutoff int     // applied cutoffs (10 / 9 / 10)
    PickyPositive, AppetitePositive, FearPositive bool
}
func (r *NiasResult) AnyPositive() bool  // trigger of the NIAS reading rule

// Pushup track (pushups.go) — a PROGRAM plus a HISTORY, not a score.
type PushupProgram struct {  // PERSISTENT program state; nil = track never started
    Variation, Goal string       // ladder rung id; reps | strength
    Base int                     // B — the ONLY generator input (0 until the first test)
    WeekIdx, SessionInWeek int; WeekOutcomes []string; RepeatCount int
    EffortAdj float64; RestBonusSec int
    SessionsDone, SessionsSinceTest, TotalReps int
    LastSessionAt, NextDueAt time.Time; DuePingSent bool   // due-ping scan key + once-per-due guard
    StreakWeeks int; FreezeUsedAt, StartedAt time.Time
    GateIdx, StartStepDown int   // safety-gate cursor + rungs to start below the picked one
    Deload, RetestPending bool   // reduced next session / retest owed first
}
func (p *PushupProgram) Clone() *PushupProgram   // deep copy for Outcome.Mutate

type PushupSession struct {   // TRANSIENT unfinished session (nil when none)
    Kind string                  // workout | test | retest
    DayIdx int; Targets []int    // 4..7 numbers, LAST = the open set's floor
    OpenFloor int; Actual []int  // Actual is index-aligned with Targets, append-only
    Idx, RestSec int; RestUntil, StartedAt time.Time
    ResumeState StateKind        // recorded escape position (pu_set | pu_rest)
}
func (s *PushupSession) Clone() *PushupSession
func (s *PushupSession) Stale(now time.Time, ttl time.Duration) bool  // nil = stale; ttl<=0 → PushupSessionTTL

type PushupTest struct {      // last COMPLETED max test (overwritten by each retest)
    TakenAt time.Time; Variation string; Reps int
    Capped bool; Source string   // hit the ceiling; initial | retest
}

type PushupSessionLog struct { // one finished session in the history ring — numbers only
    Date time.Time; DayIdx int; Sets []int
    Planned, Done, OpenTarget, OpenActual int
    Outcome, Effort string       // over|plan|short; hard|ok|easy
    Base int                     // the base this session was generated from
}
func (l PushupSessionLog) Clone() PushupSessionLog
func AppendPushupLog(history []PushupSessionLog, entry PushupSessionLog, capN int) []PushupSessionLog
func IsPushupPingState(s StateKind) bool   // idle | awaiting_mode_choice | pu_menu

const PushupHistoryCap = 36                // fallback ring size (live value: params.history_cap)
const PushupSessionTTL = 24 * time.Hour    // fallback session lifetime (live: params.session_ttl_hours)
const PushupSessionWorkout/Test/Retest, PushupTestInitial/Retest string

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
    ReturnState StateKind                          // state a detour interrupted: FODMAP position (kept across test exits; consumed by the landing's diary button) or the landing (delete-confirm entry); shared by both modes
    Mood *MoodProgress                             // nil when no PHQ-9 run in progress
    MoodResult *MoodResult                         // nil until first PHQ-9 completion
    MoodConsentAt *time.Time                       // single module-wide consent (nil = not given; wiped by /mood_delete)
    Who5, Gad7 *MoodProgress                       // WHO-5 / GAD-7 transient runs
    Who5Result *Who5Result; Gad7Result *Gad7Result // last completed WHO-5 / GAD-7
    EatConsentAt *time.Time                        // single track-wide eating consent (wiped by /food_delete)
    Edeqs, Bes, Nias *EatingProgress               // eating-track transient runs
    EdeqsResult *EdeqsResult; BesResult *BesResult; NiasResult *NiasResult
    PuConsentAt *time.Time                         // single track-wide pushup consent (wiped by /pushups_delete)
    Pushups *PushupProgram                         // nil = pushup track never started
    PuSession *PushupSession                       // nil when no session is running
    PuTest *PushupTest                             // last max test (nil until the first one)
    PuHistory []PushupSessionLog                   // capped ring of finished sessions
    ChatID int64
}

func NewStoreFromBackend(b store.Backend) *Store
func NewStore(p Persister) *Store                  // legacy constructor
func (s *Store) Get(userID int64) UserData         // copy; returns {State: Idle} for unseen
func (s *Store) Set(userID int64, d UserData)      // persists snapshot
func (s *Store) AllAwaitingDefecation() / AllAwaitingCheckin() / AllAwaiting() (legacy)
func (s *Store) AllPushupResting() map[int64]UserData          // state filter: pu_rest
func (s *Store) AllPushupDue(now time.Time) map[int64]UserData // data-driven scan over NextDueAt
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
- **Privacy invariant (mood):** same contract — raw answers of each mood-module instrument exist only inside its `MoodProgress` (`Mood`/`Who5`/`Gad7`); completing, restarting, `/abandon`, and `/mood_delete` set the progress pointer to nil in the same `Set` that writes the corresponding result. The results keep only totals + applied band ids + dates + the two allowed PHQ-9 per-question facts: the item-9 flag (needed to repeat the support contacts) and the official functional (10th) item answer (not part of the score; surfaced in the doctor report). `/mood_delete` wipes all six mood fields plus `MoodConsentAt`.
- **Privacy invariant (eating):** same contract again — raw answers of each eating-track instrument exist only inside its `EatingProgress` (`Edeqs`/`Bes`/`Nias`); completing, restarting, `/abandon` and `/food_delete` set the progress pointer to nil in the same `Set` that writes the result. The results keep only totals, the applied cutoffs and the verdicts they produced — no per-item answers, and no NIAS total (the instrument is read per subscale). `/food_delete` wipes all seven eating fields plus `EatConsentAt`.
- **Generated-numbers invariant (pushups):** `Pushups.Base` is the only number the program stores — every rep count a user sees is generated from it by `internal/screening`, so no table of loads is ever persisted here. The cosmetic level (`1 + Base/step`) is derived on display, never stored.
- **No-body-figures invariant (pushups):** the pushup shapes hold reps and seconds and nothing else — no weight, height, BMI, calories or numeric age. This extends the eating track's canary to the persisted shape and is pinned by `TestPushupShapes_NoBodyMetricFields`.
- **History-growth invariant:** `PuHistory` is a ring — always append through `AppendPushupLog`, which trims to the newest `capN` entries (`params.history_cap`, falling back to `PushupHistoryCap`). `users.json` is serialised whole on every `Set`, so an unbounded history would grow the write on every set of every session. Log entries hold numbers only, never free text.
- **Due-ping invariant:** `AllPushupDue` is the only reminder scan that is not a state filter, so the anti-spam rules live in the filter itself — at most one ping per due date (`DuePingSent`), never while a resumable session is open (`PuSession.Stale`), and only in a quiet state (`IsPushupPingState`: idle / landing / `pu_menu`). A user mid-check-in, mid-self-check or mid-set is never nudged.
- **Session TTL:** an unfinished session stays resumable for `params.session_ttl_hours` (fallback `PushupSessionTTL`) counted from `StartedAt` — sets may be spread over a day on purpose. Past the TTL `Stale` reports true and the journey closes the session by what was actually done.
- Legacy `users.json` files without the screening/mood/eating/pushup fields load as zero values — no migration needed.
- Mutations of `Screening` / `Mood` / the eating runs / `Pushups` / `PuSession` must go through `Clone()` (pointer fields — `Get`'s struct copy shares the pointee), and `PuHistory` through `AppendPushupLog` (slice field — same aliasing trap).

## When to edit

- **New per-user field** → add to `UserData` (with `omitempty` for transient fields), update relevant tests.
- **New state** → add to the `StateKind` const block; add the matching `Store.AllAwaiting<X>()` filter only if reminders need to scan that state.
- **New reminder scan that is not a state** (like the pushup due ping) → a data-driven `Store.All<X>(now)` filter here, with its anti-spam rules inside the filter rather than in the caller.
- **Change snapshot granularity** (per-user writes) → here.

## Dependencies

`internal/products` (only for `Stage` field type), `internal/store`.
