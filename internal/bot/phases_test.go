package bot

import (
	"strings"
	"testing"

	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// realContent loads the REAL bundled screening yamls (../../screening) the
// same way bot.New does — so these tests double as the boot check that the
// shipped bundle passes every validator, including the eating track's and
// the pushup track's (whose validator also guards the generator parameters).
func realContent(t *testing.T) (*screening.Content, *screening.MoodContent, *screening.EatingContent,
	*screening.PushupContent) {
	t.Helper()
	scr, err := screening.Load("../../screening")
	if err != nil {
		t.Fatalf("load bundled screening content: %v", err)
	}
	mood, err := screening.LoadMood("../../screening")
	if err != nil {
		t.Fatalf("load bundled mood content: %v", err)
	}
	eat, err := screening.LoadEating("../../screening")
	if err != nil {
		t.Fatalf("load bundled eating content: %v", err)
	}
	pu, err := screening.LoadPushups("../../screening")
	if err != nil {
		t.Fatalf("load bundled pushup content: %v", err)
	}
	return scr, mood, eat, pu
}

func phaseStates(phases []journey.Phase) []state.StateKind {
	out := make([]state.StateKind, len(phases))
	for i, p := range phases {
		out[i] = p.State()
	}
	return out
}

func hasState(states []state.StateKind, want state.StateKind) bool {
	for _, s := range states {
		if s == want {
			return true
		}
	}
	return false
}

// Every eat_* state must own a registered phase. A state the Runner cannot
// resolve is a dead end: the user is parked in it and no input moves them.
func TestPhasesFor_EatingTrackCoversEveryState(t *testing.T) {
	states := phaseStates(phasesFor(realContent(t)))

	want := []state.StateKind{
		state.StateEatConsent,
		state.StateEatMenu,
		state.StateEatEdeqsQuestion,
		state.StateEatBesQuestion,
		state.StateEatNiasQuestion,
		state.StateEatReport,
		state.StateEatDeleteConfirm,
	}
	for _, w := range want {
		if !hasState(states, w) {
			t.Errorf("no phase registered for %q", w)
		}
	}
}

// Same contract for the pushup track, and it bites harder here: this is the
// only track a user can be parked in with a HALF-FINISHED session (mid-set,
// mid-rest), so an unregistered state would strand real reps rather than an
// unanswered questionnaire.
func TestPhasesFor_PushupTrackCoversEveryState(t *testing.T) {
	states := phaseStates(phasesFor(realContent(t)))

	want := []state.StateKind{
		state.StatePuConsent,
		state.StatePuGate,
		state.StatePuGoal,
		state.StatePuVariation,
		state.StatePuTest,
		state.StatePuMenu,
		state.StatePuSet,
		state.StatePuRest,
		state.StatePuEffort,
		state.StatePuWeekFork,
		state.StatePuRedCard,
		state.StatePuProgress,
		state.StatePuDeleteConfirm,
	}
	for _, w := range want {
		if !hasState(states, w) {
			t.Errorf("no phase registered for %q", w)
		}
	}
}

// StatePuRest is scanned by Runner.Remind on every reminder tick: without a
// registered phase the rest timer never fires the "go" message and the
// session hangs until its TTL. The scan is silent about a missing phase (it
// skips the kind), so only this check catches it.
func TestPhasesFor_RestPhaseIsRegisteredForTheReminderScan(t *testing.T) {
	if !hasState(phaseStates(phasesFor(realContent(t))), state.StatePuRest) {
		t.Fatal("pu_rest has no phase: the rest timer would never ping")
	}
}

// The Runner keys phases by State() in a map, so a second phase claiming an
// already-taken state silently replaces the first — a copy-paste slip that
// no runtime error reports.
func TestPhasesFor_NoDuplicateStates(t *testing.T) {
	seen := make(map[state.StateKind]bool)
	for _, s := range phaseStates(phasesFor(realContent(t))) {
		if seen[s] {
			t.Errorf("state %q is claimed by more than one phase", s)
		}
		seen[s] = true
	}
}

// A track without content must contribute no phases at all — the registration
// and the command menu have to agree on what this binary handles.
func TestPhasesFor_UnwiredTrackRegistersNothing(t *testing.T) {
	scr, mood, eat, pu := realContent(t)

	cases := []struct {
		name   string
		phases []journey.Phase
		absent string // state-kind prefix that must not appear
	}{
		{"eating off", phasesFor(scr, mood, nil, pu), "eat_"},
		{"mood off", phasesFor(scr, nil, eat, pu), "mood_"},
		{"screening off", phasesFor(nil, mood, eat, pu), "scr_"},
		{"pushups off", phasesFor(scr, mood, eat, nil), "pu_"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, s := range phaseStates(tc.phases) {
				if strings.HasPrefix(string(s), tc.absent) {
					t.Errorf("unwired track still registered %q", s)
				}
			}
		})
	}
}

// With no screening content there is no mode fork: Runner.HandleStart falls
// back to the legacy direct-to-diary /start, and the diary phases must still
// be there for it to land on.
func TestPhasesFor_NoTracksKeepsDiaryAndDropsLanding(t *testing.T) {
	states := phaseStates(phasesFor(nil, nil, nil, nil))

	if hasState(states, state.StateAwaitingModeChoice) {
		t.Error("landing must not be registered without screening content")
	}
	for _, w := range []state.StateKind{
		state.StateAwaitingDefecation,
		state.StateAwaitingProductCategory,
		state.StateAwaitingProductChoice,
		state.StateAwaitingStageChoice,
		state.StateAwaitingStageCheckin,
	} {
		if !hasState(states, w) {
			t.Errorf("diary phase for %q must be registered unconditionally", w)
		}
	}
}
