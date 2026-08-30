package state

// Pushup track (pu_*) persisted shapes.
//
// This track is different from the three self-checks next door in two ways
// that the types below encode.
//
// First, it stores a PROGRAM, not a score: PushupProgram carries one real
// variable — Base, the last max-test result — and every rep count the user
// sees is generated from it by internal/screening. Nothing tabulated is
// persisted here, and the cosmetic level (1 + Base/step) is derived on
// display rather than stored.
//
// Second, it is the only track that keeps a HISTORY. users.json is
// serialised whole on every Set, so the history is a capped ring of
// numbers-only entries (AppendPushupLog) — no free text, no unbounded
// growth.
//
// What this track never stores, by the same rule the eating track pins in
// its canary test: weight, height, BMI, calories, age. Reps and seconds are
// the only body figures in the whole shape.

import "time"

// Session kinds of PushupSession.Kind. A test and a retest share the whole
// protocol; they differ only in what they mean for the program.
const (
	PushupSessionWorkout = "workout"
	PushupSessionTest    = "test"
	PushupSessionRetest  = "retest"
)

// Sources of PushupTest.Source — which run produced the stored test.
const (
	PushupTestInitial = "initial"
	PushupTestRetest  = "retest"
)

// PushupHistoryCap is the fallback ring size used when a caller passes a
// non-positive cap to AppendPushupLog. The live value is content-driven
// (`params.history_cap` in screening/pushups_ru.yaml); this constant only
// guarantees the ring is bounded even if the content is misread.
const PushupHistoryCap = 36

// PushupSessionTTL is the fallback lifetime of an unfinished session used by
// PushupSession.Stale when the caller passes a non-positive ttl. The live
// value is content-driven (`params.session_ttl_hours`). Sets may be spread
// over a whole day on purpose — the session only stops being resumable
// after the TTL.
const PushupSessionTTL = 24 * time.Hour

// PushupProgram is the PERSISTENT state of the pushup track — nil means the
// track was never started. It is created when consent is given (with a zero
// Base) and lives until /pushups_delete.
//
// Base is the single input of the generator. WeekIdx / SessionInWeek /
// WeekOutcomes describe the week in progress; the week closes on the FACT of
// its last session, not on a calendar date, so nothing here is a weekday.
type PushupProgram struct {
	Variation string `json:"variation,omitempty"` // ladder rung id (wall … feet_up)
	Goal      string `json:"goal,omitempty"`      // reps | strength — branches the plateau logic for life
	Base      int    `json:"base,omitempty"`      // B: the last max-test result, 0 until the first test

	WeekIdx       int      `json:"week_idx,omitempty"`        // completed weeks
	SessionInWeek int      `json:"session_in_week,omitempty"` // 0..sessions_per_week-1; also the generator's day index
	WeekOutcomes  []string `json:"week_outcomes,omitempty"`   // over|plan|short of the CURRENT week
	RepeatCount   int      `json:"repeat_count,omitempty"`    // consecutive repeated weeks (fork at repeat_fork_after)

	EffortAdj    float64 `json:"effort_adj,omitempty"`     // accumulated self-report factor (0 = "never asked" = 1.0)
	RestBonusSec int     `json:"rest_bonus_sec,omitempty"` // the one-off "+30 s to everything"

	SessionsDone      int `json:"sessions_done,omitempty"`
	SessionsSinceTest int `json:"sessions_since_test,omitempty"` // retest is due at retest_every_sessions
	TotalReps         int `json:"total_reps,omitempty"`          // lifetime volume — a never-decreasing metric

	LastSessionAt time.Time `json:"last_session_at,omitempty"`
	NextDueAt     time.Time `json:"next_due_at,omitempty"` // scan key of the "time to train" ping
	DuePingSent   bool      `json:"due_ping_sent,omitempty"`

	StreakWeeks  int       `json:"streak_weeks,omitempty"`   // weeks, never days — rest days must not break it
	FreezeUsedAt time.Time `json:"freeze_used_at,omitempty"` // reserved for the monthly streak freeze

	StartedAt time.Time `json:"started_at,omitempty"`

	// Setup and regression carry-overs. GateIdx is the cursor of the
	// three-question safety gate (the gate runs once, before the first test);
	// StartStepDown is how many rungs below the picked one the gate asked to
	// start (joint pain, pregnancy). Deload marks the next session as reduced
	// after a dropped retest or a long pause; RetestPending marks a retest
	// owed before the next ordinary session (long pause, joint-pain step
	// down).
	GateIdx       int  `json:"gate_idx,omitempty"`
	StartStepDown int  `json:"start_step_down,omitempty"`
	Deload        bool `json:"deload,omitempty"`
	RetestPending bool `json:"retest_pending,omitempty"`
}

// Clone returns a deep copy (the outcomes slice is copied), so Outcome.Mutate
// can replace the pointer instead of mutating shared data.
func (p *PushupProgram) Clone() *PushupProgram {
	if p == nil {
		return nil
	}
	out := *p
	out.WeekOutcomes = append([]string(nil), p.WeekOutcomes...)
	return &out
}

// PushupSession is the TRANSIENT unfinished session — nil whenever none is
// running. It is wiped on completion, on /abandon, on /pushups_delete, and
// when it goes Stale.
//
// Targets is variable-length (4..7: the set count grows with the base) and
// its LAST element is the floor of the open set, which is why there are no
// per-set fields. Actual is index-aligned with Targets and append-only, so a
// session abandoned halfway still classifies (missing entries read as zero).
type PushupSession struct {
	Kind        string    `json:"kind,omitempty"`    // workout | test | retest
	DayIdx      int       `json:"day_idx,omitempty"` // 0..sessions_per_week-1
	Targets     []int     `json:"targets,omitempty"`
	OpenFloor   int       `json:"open_floor,omitempty"` // mirrors the last element of Targets
	Actual      []int     `json:"actual,omitempty"`
	Idx         int       `json:"idx,omitempty"`      // cursor: the set being performed
	RestSec     int       `json:"rest_sec,omitempty"` // rest planned between the sets of THIS session
	RestUntil   time.Time `json:"rest_until,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	ResumeState StateKind `json:"resume_state,omitempty"` // recorded escape position (pu_set | pu_rest)
}

// Clone returns a deep copy (both int slices are copied), so Outcome.Mutate
// can replace the pointer instead of mutating shared data.
func (s *PushupSession) Clone() *PushupSession {
	if s == nil {
		return nil
	}
	out := *s
	out.Targets = append([]int(nil), s.Targets...)
	out.Actual = append([]int(nil), s.Actual...)
	return &out
}

// Stale reports whether the session is no longer resumable: it has outlived
// ttl (PushupSessionTTL when ttl <= 0) counted from StartedAt. A nil session
// is Stale by definition — there is nothing to resume. A session without a
// StartedAt is treated as fresh rather than instantly dropped.
func (s *PushupSession) Stale(now time.Time, ttl time.Duration) bool {
	if s == nil {
		return true
	}
	if s.StartedAt.IsZero() {
		return false
	}
	if ttl <= 0 {
		ttl = PushupSessionTTL
	}
	return now.Sub(s.StartedAt) >= ttl
}

// PushupTest is the last COMPLETED max test (overwritten by each retest).
// Kept apart from the program for the same reason ScreeningResult is kept
// apart from ScreeningProgress: the test is a reproducible measurement on a
// named rung, the program is the mutable load built on top of it.
type PushupTest struct {
	TakenAt   time.Time `json:"taken_at"`
	Variation string    `json:"variation,omitempty"` // the rung the test was taken on
	Reps      int       `json:"reps"`
	Capped    bool      `json:"capped,omitempty"` // hit the test ceiling → a harder rung is offered
	Source    string    `json:"source,omitempty"` // initial | retest
}

// PushupSessionLog is one finished session in the history ring: numbers
// only, never free text. Base records the base the session was generated
// from, so the history stays readable after the base moves.
type PushupSessionLog struct {
	Date       time.Time `json:"date"`
	DayIdx     int       `json:"day_idx,omitempty"`
	Sets       []int     `json:"sets,omitempty"`    // what was actually done, per set
	Planned    int       `json:"planned,omitempty"` // fixed sets + the open set's floor
	Done       int       `json:"done,omitempty"`    // actual volume
	OpenTarget int       `json:"open_target,omitempty"`
	OpenActual int       `json:"open_actual,omitempty"`
	Outcome    string    `json:"outcome,omitempty"` // over | plan | short
	Effort     string    `json:"effort,omitempty"`  // hard | ok | easy
	Base       int       `json:"base,omitempty"`
}

// Clone returns a copy with its own Sets backing array.
func (l PushupSessionLog) Clone() PushupSessionLog {
	l.Sets = append([]int(nil), l.Sets...)
	return l
}

// AppendPushupLog appends entry to history and trims the result to the most
// recent capN entries (PushupHistoryCap when capN <= 0). It never mutates
// the input slice — Store.Get hands out a struct copy that still shares the
// history's backing array — and it clones the entry, whose Sets slice
// usually still belongs to the live session.
func AppendPushupLog(history []PushupSessionLog, entry PushupSessionLog, capN int) []PushupSessionLog {
	if capN <= 0 {
		capN = PushupHistoryCap
	}
	keep := history
	if len(keep) >= capN {
		keep = keep[len(keep)-capN+1:]
	}
	out := make([]PushupSessionLog, 0, len(keep)+1)
	out = append(out, keep...)
	return append(out, entry.Clone())
}

// IsPushupPingState reports whether a "time to train" ping may reach a user
// in this state. The due ping is the first reminder in the bot that is NOT
// tied to the state it interrupts, so the rule is deliberately narrow: only
// a user sitting idle, on the landing, or in the track's own menu can be
// nudged. Anyone mid-check-in, mid-self-check or mid-set stays untouched.
func IsPushupPingState(s StateKind) bool {
	switch s {
	case StateIdle, StateAwaitingModeChoice, StatePuMenu:
		return true
	}
	return false
}

// AllPushupResting returns a snapshot of users parked in StatePuRest — the
// within-session rest timer. The caller compares PuSession.RestUntil with
// the clock; the store only narrows the scan.
func (s *Store) AllPushupResting() map[int64]UserData {
	return s.filterByState(StatePuRest)
}

// AllPushupDue returns a snapshot of users whose next session is due at now:
// a data-driven scan over Pushups.NextDueAt rather than a state filter,
// because "time to train" is not a state.
//
// The filter itself enforces the three rules that keep this ping from
// becoming spam: at most one ping per due date (DuePingSent), never while a
// resumable session is open (an escaped session is offered by the landing
// instead), and only in a quiet state (IsPushupPingState).
func (s *Store) AllPushupDue(now time.Time) map[int64]UserData {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int64]UserData)
	for id, d := range s.data {
		p := d.Pushups
		switch {
		case p == nil, p.NextDueAt.IsZero(), p.DuePingSent:
			continue
		case now.Before(p.NextDueAt):
			continue
		case !d.PuSession.Stale(now, 0):
			continue
		case !IsPushupPingState(d.State):
			continue
		}
		out[id] = d
	}
	return out
}
