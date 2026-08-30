package journey

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// The session automaton: one set = one message.
//
//	pu_set    "Подход 2/5 — 8 повторов."      → the user answers with a number
//	pu_rest   "Записал 8. Отдых 90 с."        → server-side timer + «Готов раньше»
//	pu_effort "Как ощущалось?"                → the ONE post-session question
//
// The rest timer is deliberately NOT a live countdown: Telegram's rate limit
// makes per-second message edits impossible in any honest sense, and the
// documented pain of the phone apps this track competes with is a timer that
// dies when the app is backgrounded. Here the state simply waits, the
// reminder loop delivers exactly one "go" message when the rest is over, and
// resting LONGER is never counted against the session.

// PuSetPhase owns StatePuSet — "do this many, tell me what you got".
type PuSetPhase struct {
	c *screening.PushupContent
}

func NewPuSetPhase(c *screening.PushupContent) *PuSetPhase { return &PuSetPhase{c: c} }

func (PuSetPhase) State() state.StateKind { return state.StatePuSet }

func (p *PuSetPhase) Setup(ctx Context) Outcome {
	now := ctx.Now()
	s := puActiveSession(p.c, ctx.User, now)
	if s == nil {
		return Outcome{NextState: state.StatePuMenu}
	}
	if s.Kind != state.PushupSessionWorkout {
		return Outcome{NextState: state.StatePuTest}
	}
	idx, total := len(s.Actual), len(s.Targets)
	if idx >= total {
		return Outcome{NextState: state.StatePuEffort}
	}

	var text string
	var row []string
	switch {
	case idx == total-1:
		text = renderContent(p.c.Session.OpenPrompt, map[string]string{
			"current": strconv.Itoa(idx + 1),
			"total":   strconv.Itoa(total),
			"floor":   strconv.Itoa(s.OpenFloor),
		})
		row = puOpenKeyboard(s.OpenFloor)
	default:
		tpl := p.c.Session.SetPrompt
		if idx > 0 {
			// Arriving from a rest (timer, «Готов раньше» or a resume): the
			// short "go" wording, not the full first-set prompt.
			tpl = p.c.Session.RestOver
		}
		text = renderContent(tpl, map[string]string{
			"current": strconv.Itoa(idx + 1),
			"total":   strconv.Itoa(total),
			"reps":    strconv.Itoa(s.Targets[idx]),
		})
		row = puRepsKeyboard(s.Targets[idx])
	}

	oc := scrText(text)
	oc.Keyboard = [][]string{row, {homeLabel(ctx)}}
	oc.Mutate = func(u *state.UserData) {
		mutatePuSession(u, func(s *state.PushupSession) {
			s.ResumeState = state.StatePuSet
			s.RestUntil = time.Time{}
		})
	}
	return oc
}

func (p *PuSetPhase) Collect(ctx Context, input string) Outcome {
	now := ctx.Now()
	if puActiveSession(p.c, ctx.User, now) == nil {
		return Outcome{NextState: state.StatePuMenu}
	}
	reps, ok := parsePuReps(input, p.c.Params.MaxRepsInput)
	if !ok {
		return scrText(p.c.Session.BadInput)
	}
	return puRecordReps(p.c, ctx.User, reps, now)
}

func (PuSetPhase) Remind(ctx Context) Outcome { return Outcome{} }

// PuRestPhase owns StatePuRest — the server-side rest between sets. Its
// Setup renders the "recorded N, rest M s" line with the REMAINING seconds,
// so it reads correctly both right after a set and when a paused user comes
// back to the session hours later.
type PuRestPhase struct {
	c *screening.PushupContent
}

func NewPuRestPhase(c *screening.PushupContent) *PuRestPhase { return &PuRestPhase{c: c} }

func (PuRestPhase) State() state.StateKind { return state.StatePuRest }

func (p *PuRestPhase) Setup(ctx Context) Outcome {
	now := ctx.Now()
	s := puActiveSession(p.c, ctx.User, now)
	if s == nil {
		return Outcome{NextState: state.StatePuMenu}
	}
	if s.Kind != state.PushupSessionWorkout {
		return Outcome{NextState: state.StatePuTest}
	}
	if len(s.Actual) >= len(s.Targets) {
		return Outcome{NextState: state.StatePuEffort}
	}
	if !now.Before(s.RestUntil) {
		return Outcome{NextState: state.StatePuSet}
	}

	last := 0
	if n := len(s.Actual); n > 0 {
		last = s.Actual[n-1]
	}
	oc := scrText(renderContent(p.c.Session.Recorded, map[string]string{
		"reps": strconv.Itoa(last),
		// Rounded UP: the ping lands on the next scan tick, so the honest
		// promise is "not before N seconds".
		"rest": strconv.Itoa(int(math.Ceil(s.RestUntil.Sub(now).Seconds()))),
	}))
	oc.Keyboard = [][]string{{p.c.Session.ReadyButton, p.c.Session.PauseButton}}
	oc.Mutate = func(u *state.UserData) {
		mutatePuSession(u, func(s *state.PushupSession) { s.ResumeState = state.StatePuRest })
	}
	return oc
}

func (p *PuRestPhase) Collect(ctx Context, input string) Outcome {
	now := ctx.Now()
	if puActiveSession(p.c, ctx.User, now) == nil {
		return Outcome{NextState: state.StatePuMenu}
	}
	switch in := normText(input); {
	case labelIs(in, p.c.Session.ReadyButton):
		return Outcome{
			NextState: state.StatePuSet,
			Mutate: func(u *state.UserData) {
				mutatePuSession(u, func(s *state.PushupSession) { s.RestUntil = time.Time{} })
			},
		}
	case labelIs(in, p.c.Session.PauseButton):
		// Pause = the landing, with the position recorded. Sets may be
		// spread over the whole day on purpose.
		return Outcome{
			NextState: testExitState,
			Mutate: func(u *state.UserData) {
				mutatePuSession(u, func(s *state.PushupSession) { s.ResumeState = state.StatePuRest })
			},
		}
	default:
		// A number during the rest means the set was done ahead of the
		// timer — record it instead of scolding the user.
		if reps, ok := parsePuReps(input, p.c.Params.MaxRepsInput); ok {
			return puRecordReps(p.c, ctx.User, reps, now)
		}
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

// Remind is the rest timer: on the first tick after RestUntil it moves the
// session to the next set, whose Setup sends the single "go" message. A
// session that outlived its TTL is closed here instead, so the automaton
// never parks a user in a rest forever.
func (p *PuRestPhase) Remind(ctx Context) Outcome {
	now := ctx.Now()
	s := puActiveSession(p.c, ctx.User, now)
	if s == nil {
		return Outcome{NextState: state.StatePuMenu}
	}
	if s.Kind != state.PushupSessionWorkout || now.Before(s.RestUntil) {
		return Outcome{}
	}
	if len(s.Actual) >= len(s.Targets) {
		return Outcome{NextState: state.StatePuEffort}
	}
	return Outcome{NextState: state.StatePuSet}
}

// PuEffortPhase owns StatePuEffort — the ONE question asked after a session.
// Its answer feeds the within-week auto-regulation (EffortAdj) that sits on
// top of the weekly base rule, and it is also where the session is finally
// written: history entry, counters, next due date and, on the third session,
// the week's verdict.
type PuEffortPhase struct {
	c *screening.PushupContent
}

func NewPuEffortPhase(c *screening.PushupContent) *PuEffortPhase { return &PuEffortPhase{c: c} }

func (PuEffortPhase) State() state.StateKind { return state.StatePuEffort }

func (p *PuEffortPhase) Setup(ctx Context) Outcome {
	now := ctx.Now()
	s := puActiveSession(p.c, ctx.User, now)
	if s == nil {
		return Outcome{NextState: state.StatePuMenu}
	}
	if s.Kind != state.PushupSessionWorkout {
		return Outcome{NextState: state.StatePuTest}
	}
	if len(s.Actual) < len(s.Targets) {
		return Outcome{NextState: state.StatePuSet}
	}
	oc := scrText(p.c.Session.EffortPrompt)
	oc.Keyboard = [][]string{{
		p.c.Session.EffortHardButton,
		p.c.Session.EffortOKButton,
		p.c.Session.EffortEasyButton,
	}}
	oc.Mutate = func(u *state.UserData) {
		mutatePuSession(u, func(s *state.PushupSession) { s.ResumeState = state.StatePuEffort })
	}
	return oc
}

func (p *PuEffortPhase) Collect(ctx Context, input string) Outcome {
	now := ctx.Now()
	if puActiveSession(p.c, ctx.User, now) == nil {
		return Outcome{NextState: state.StatePuMenu}
	}
	var answer string
	switch in := normText(input); {
	case labelIs(in, p.c.Session.EffortHardButton):
		answer = screening.PushupEffortHard
	case labelIs(in, p.c.Session.EffortOKButton):
		answer = screening.PushupEffortOK
	case labelIs(in, p.c.Session.EffortEasyButton):
		answer = screening.PushupEffortEasy
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}

	// Same trick as the menu's expiry: run the bookkeeping on a local copy
	// to compose the message, then run it again inside Mutate on the stored
	// user. Both calls share the same clock, so they agree by construction.
	sim := ctx.User
	text, next := puFinishSession(p.c, &sim, answer, now)
	oc := scrText(text)
	oc.NextState = next
	oc.Mutate = func(u *state.UserData) { puFinishSession(p.c, u, answer, now) }
	return oc
}

func (PuEffortPhase) Remind(ctx Context) Outcome { return Outcome{} }

// puFinishSession writes the finished session and returns the closing
// message plus where the user goes next. It is the only place the week can
// close, and the week's verdict is ALWAYS spoken out loud — a base that
// moves silently reads as a bug.
func puFinishSession(c *screening.PushupContent, u *state.UserData, answer string,
	now time.Time) (string, state.StateKind) {
	s := u.PuSession
	if s == nil || u.Pushups == nil {
		return "", state.StatePuMenu
	}
	log := puSessionLog(c, u.Pushups, s, answer, now)
	mutatePuProgram(u, func(p *state.PushupProgram) {
		p.EffortAdj = c.NextEffortAdj(p.EffortAdj, answer)
	})
	puCloseSession(c, u, log, now)

	lines := []string{puEffortReply(c, answer)}
	if puWeekDue(c, u.Pushups) {
		var closing string
		mutatePuProgram(u, func(p *state.PushupProgram) { closing = puCloseWeek(c, p) })
		lines = append(lines, closing)
	}

	next := testExitState
	switch {
	case u.Pushups.RepeatCount >= c.Params.Progression.RepeatForkAfter:
		// Three repeated weeks in a row: stop pretending the program fits
		// and offer the two things that actually change it.
		next = state.StatePuWeekFork
	case puTestDue(c, u.Pushups, now):
		lines = append(lines, c.Week.RetestDue)
	}
	lines = append(lines, renderContent(c.Session.NextDue, map[string]string{
		"hours": strconv.Itoa(c.Params.AdvisedHoursBetween),
	}))
	return strings.Join(lines, "\n"), next
}

// puEffortReply maps the self-report onto its one-line acknowledgement.
func puEffortReply(c *screening.PushupContent, answer string) string {
	switch answer {
	case screening.PushupEffortHard:
		return c.Session.EffortHardReply
	case screening.PushupEffortEasy:
		return c.Session.EffortEasyReply
	default:
		return c.Session.EffortOKReply
	}
}

// PuWeekForkPhase owns StatePuWeekFork — the fork offered after the third
// repeated week in a row. Repeating a fourth time would be the program
// blaming the user; instead it offers the two levers that actually change
// the load: an easier rung (with an immediate retest, so the base is
// re-measured where it now lives) or more rest between sets.
type PuWeekForkPhase struct {
	c *screening.PushupContent
}

func NewPuWeekForkPhase(c *screening.PushupContent) *PuWeekForkPhase {
	return &PuWeekForkPhase{c: c}
}

func (PuWeekForkPhase) State() state.StateKind { return state.StatePuWeekFork }

func (p *PuWeekForkPhase) Setup(ctx Context) Outcome {
	if ctx.User.Pushups == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	oc := scrText(p.c.Week.ForkPrompt)
	oc.Keyboard = [][]string{{p.c.Week.ForkEasierButton}, {p.c.Week.ForkRestButton}}
	return oc
}

func (p *PuWeekForkPhase) Collect(ctx Context, input string) Outcome {
	if ctx.User.Pushups == nil {
		return Outcome{NextState: state.StatePuConsent}
	}
	switch in := normText(input); {
	case labelIs(in, p.c.Week.ForkEasierButton):
		oc := scrText(p.c.Week.ForkEasierReply)
		oc.NextState = state.StatePuTest
		oc.Mutate = func(u *state.UserData) {
			mutatePuProgram(u, func(pr *state.PushupProgram) {
				pr.Variation = p.c.ShiftVariation(pr.Variation, -1)
				pr.RepeatCount = 0
				pr.RetestPending = true
			})
		}
		return oc
	case labelIs(in, p.c.Week.ForkRestButton):
		oc := scrText(renderContent(p.c.Week.ForkRestReply, map[string]string{
			"seconds": strconv.Itoa(p.c.Params.RestBonusStep),
		}))
		oc.NextState = testExitState
		oc.Mutate = func(u *state.UserData) {
			mutatePuProgram(u, func(pr *state.PushupProgram) {
				pr.RestBonusSec += p.c.Params.RestBonusStep
				pr.RepeatCount = 0
			})
		}
		return oc
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (PuWeekForkPhase) Remind(ctx Context) Outcome { return Outcome{} }
