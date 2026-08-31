package journey

import (
	"strconv"
	"strings"
	"time"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// PuMaxTestPhase owns StatePuTest — the max test and, with the identical
// protocol, every retest. It is the only measurement of the track: the
// result becomes the base, and every rep count the user will ever see is
// generated from it.
//
// The file is named pu_maxtest.go on purpose — pu_test.go would be a Go TEST
// file and this code would never ship.
//
// Rules the phase enforces, all content-driven:
//
//   - the rep and stop rules are shown before the number is asked;
//   - the result is capped (test_cap): above the cap a longer set measures
//     endurance of the clock, not strength, so a harder rung is offered;
//   - a result at or below too_low_reps means the rung is too hard — step
//     down and test again at once (or, already on the easiest rung, floor
//     the base);
//   - a retest COUNTS as the week's session: it closes the week instead of
//     adding load on top of it.
type PuMaxTestPhase struct {
	c *screening.PushupContent
}

func NewPuMaxTestPhase(c *screening.PushupContent) *PuMaxTestPhase {
	return &PuMaxTestPhase{c: c}
}

func (PuMaxTestPhase) State() state.StateKind { return state.StatePuTest }

func (p *PuMaxTestPhase) Setup(ctx Context) Outcome {
	prog := ctx.User.Pushups
	if prog == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	if prog.Variation == "" {
		return Outcome{NextState: puEntryState(p.c, ctx.User, ctx.Now())}
	}
	now := ctx.Now()
	if s := puWorkoutSession(p.c, ctx.User, now); s != nil {
		// An unfinished workout is never replaced by a test.
		return Outcome{NextState: puResumeTarget(p.c, ctx.User, now)}
	}

	retest := prog.Base > 0
	kind := state.PushupSessionTest
	intro := p.c.Test.Intro
	if retest {
		kind, intro = state.PushupSessionRetest, p.c.Test.RetestIntro
	}

	lines := make([]string, 0, len(p.c.Test.RepRules)+len(p.c.Test.StopRules)+2)
	lines = append(lines, intro)
	for _, r := range p.c.Test.RepRules {
		lines = append(lines, "• "+r)
	}
	for _, r := range p.c.Test.StopRules {
		lines = append(lines, "• "+r)
	}
	lines = append(lines, p.c.Test.Prompt)

	oc := scrText(strings.Join(lines, "\n"))
	oc.Keyboard = [][]string{{homeLabel(ctx)}}
	if puActiveSession(p.c, ctx.User, now) == nil {
		oc.Mutate = func(u *state.UserData) {
			u.PuSession = &state.PushupSession{
				Kind:      kind,
				StartedAt: now,
			}
		}
	}
	return oc
}

func (p *PuMaxTestPhase) Collect(ctx Context, input string) Outcome {
	prog := ctx.User.Pushups
	if prog == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	now := ctx.Now()
	s := puActiveSession(p.c, ctx.User, now)
	if s == nil || s.Kind == state.PushupSessionWorkout {
		// Nothing to answer (the test session expired, say) — re-enter the
		// track where its data says it should be.
		return Outcome{NextState: puEntryState(p.c, ctx.User, now)}
	}

	reps, ok := parsePuReps(input, p.c.Params.MaxRepsInput)
	if !ok {
		return scrText(p.c.Test.BadInput)
	}

	verdict := p.c.ClassifyTest(reps, p.c.VariationIndex(prog.Variation) > 0)
	if verdict.TooLow && verdict.StepDown {
		// The rung is too hard to measure anything: drop one and retest
		// immediately — re-entering this state builds a fresh test session.
		oc := scrText(p.c.Test.TooLow)
		oc.NextState = state.StatePuTest
		oc.Mutate = func(u *state.UserData) {
			mutatePuProgram(u, func(pr *state.PushupProgram) {
				pr.Variation = p.c.ShiftVariation(pr.Variation, -1)
			})
			u.PuSession = nil
		}
		return oc
	}

	retest := s.Kind == state.PushupSessionRetest
	recorded := reps
	if verdict.Capped {
		recorded = p.c.Params.TestCap
	}
	oldBase, newBase := prog.Base, verdict.Base
	deload := retest && p.c.RetestNeedsDeload(oldBase, newBase)
	if puPauseDays(prog, now) >= p.c.Params.PauseDeloadDays {
		// Back after a long pause: one reduced week, not a rollback.
		deload = true
	}

	var lines []string
	if verdict.Capped {
		lines = append(lines, renderContent(p.c.Test.Capped, map[string]string{
			"reps": strconv.Itoa(recorded),
		}))
	}
	if verdict.TooLow {
		lines = append(lines, renderContent(p.c.Test.FloorNote, map[string]string{
			"base": strconv.Itoa(newBase),
		}))
	}
	lines = append(lines, p.resultLine(retest, oldBase, newBase, recorded))

	source := state.PushupTestInitial
	if retest {
		source = state.PushupTestRetest
	}
	test := state.PushupTest{
		TakenAt:   now,
		Variation: prog.Variation,
		Reps:      recorded,
		Capped:    verdict.Capped,
		Source:    source,
	}

	oc := scrText(strings.Join(lines, "\n"))
	oc.NextState = state.StatePuMenu
	oc.Mutate = func(u *state.UserData) {
		t := test
		u.PuTest = &t
		u.PuSession = nil
		mutatePuProgram(u, func(pr *state.PushupProgram) {
			pr.Base = newBase
			pr.SessionsSinceTest = 0
			pr.RetestPending = false
			if deload {
				pr.Deload = true
			}
			if !retest {
				return
			}
			// A retest IS the session of the day and closes the week: no
			// load is stacked on top of a max effort.
			pr.SessionsDone++
			pr.TotalReps += recorded
			pr.LastSessionAt = now
			pr.NextDueAt = now.Add(time.Duration(p.c.Params.AdvisedHoursBetween) * time.Hour)
			pr.DuePingSent = false
			pr.WeekIdx++
			pr.SessionInWeek = 0
			pr.WeekOutcomes = nil
			pr.RepeatCount = 0
			pr.StreakWeeks++
		})
		if retest {
			u.PuHistory = state.AppendPushupLog(u.PuHistory, state.PushupSessionLog{
				Date:       now,
				Sets:       []int{recorded},
				Planned:    recorded,
				Done:       recorded,
				OpenTarget: recorded,
				OpenActual: recorded,
				Outcome:    screening.PushupOutcomePlan,
				Base:       newBase,
			}, p.c.Params.HistoryCap)
		}
	}
	return oc
}

// resultLine is the one line that reports the measurement: the fresh base
// for a first test, the delta against the previous one for a retest.
func (p *PuMaxTestPhase) resultLine(retest bool, oldBase, newBase, reps int) string {
	args := map[string]string{
		"reps":  strconv.Itoa(reps),
		"base":  strconv.Itoa(newBase),
		"level": strconv.Itoa(p.c.Level(newBase)),
		"old":   strconv.Itoa(oldBase),
		"new":   strconv.Itoa(reps),
	}
	if !retest {
		return renderContent(p.c.Test.ResultLine, args)
	}
	switch {
	case newBase > oldBase:
		return renderContent(p.c.Test.CompareUp, args)
	case newBase < oldBase:
		return renderContent(p.c.Test.CompareDown, args)
	default:
		return renderContent(p.c.Test.CompareSame, args)
	}
}

func (PuMaxTestPhase) Remind(ctx Context) Outcome { return Outcome{} }
