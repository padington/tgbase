package journey

import (
	"strconv"
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// PuMenuPhase owns StatePuMenu — the track's home screen: where the program
// stands ("week 2, session 1 of 3"), what the next session looks like, and
// the four things one can do here (train, resume an open session, look at
// the progress screen, retest early). The 🏠 row is the usual escape (the
// runner intercepts that tap before dispatch).
//
// Two pieces of housekeeping live here because this is the state every entry
// into the track passes through: an unfinished session that outlived its TTL
// is closed as a partial one, and the plan of the NEXT session is rendered
// from the current base, so the menu doubles as the "what am I in for" card.
type PuMenuPhase struct {
	c *screening.PushupContent
}

func NewPuMenuPhase(c *screening.PushupContent) *PuMenuPhase {
	return &PuMenuPhase{c: c}
}

func (PuMenuPhase) State() state.StateKind { return state.StatePuMenu }

func (p *PuMenuPhase) Setup(ctx Context) Outcome {
	if !hasPuConsent(ctx.User) {
		return Outcome{NextState: state.StatePuConsent}
	}
	now := ctx.Now()
	// Work on a local copy first: the expiry has to be reflected in the text
	// this very message renders, and applying it twice (here and in Mutate)
	// with the same clock is deterministic.
	sim := ctx.User
	expired := puExpireSession(p.c, &sim, now)
	applyExpiry := func(u *state.UserData) { puExpireSession(p.c, u, now) }

	prog := sim.Pushups
	if prog == nil || prog.Base <= 0 {
		return Outcome{NextState: puEntryState(p.c, sim, now)}
	}

	var lines []string
	if expired.Text != "" {
		lines = append(lines, expired.Text)
	}
	if expired.Fork {
		// The expiry closed the third repeated week in a row. The verdict has
		// just been spoken; the fork is what owes the user next, exactly as
		// after a session finished by hand.
		oc := scrText(strings.Join(lines, "\n"))
		oc.NextState = state.StatePuWeekFork
		oc.Mutate = applyExpiry
		return oc
	}
	lines = append(lines, renderContent(p.c.UI.MenuPrompt, map[string]string{
		"week":    strconv.Itoa(prog.WeekIdx + 1),
		"session": strconv.Itoa(prog.SessionInWeek + 1),
		"total":   strconv.Itoa(p.c.Params.SessionsPerWeek),
	}))

	open := puWorkoutSession(p.c, sim, now)
	var kb [][]string
	switch {
	case open != nil:
		// The most likely next action on top, exactly like the landing.
		kb = append(kb, []string{puResumeLabel(p.c, open)})
	case puTestDue(p.c, prog, now):
		lines = append(lines, p.c.Week.RetestDue)
	default:
		lines = append(lines, puPlanLine(p.c, prog))
	}
	kb = append(kb, []string{p.c.UI.TrainButton}, []string{p.c.UI.ProgressButton})
	if open == nil {
		kb = append(kb, []string{p.c.UI.RetestButton})
	}
	kb = append(kb, []string{homeLabel(ctx)})

	oc := scrText(strings.Join(lines, "\n"))
	oc.Keyboard = kb
	if expired.Text != "" {
		oc.Mutate = applyExpiry
	}
	return oc
}

func (p *PuMenuPhase) Collect(ctx Context, input string) Outcome {
	now := ctx.Now()
	open := puWorkoutSession(p.c, ctx.User, now)
	in := normText(input)
	switch {
	case open != nil && labelIs(in, puResumeLabel(p.c, open)):
		return Outcome{NextState: puResumeTarget(p.c, ctx.User, now)}
	case labelIs(in, p.c.UI.TrainButton):
		return puTrain(ctx, p.c, false)
	case labelIs(in, p.c.Session.TrainAnywayButton):
		// Only the soft "it has been less than 48 h" warning is overridden;
		// the hard 24 h block is not for sale.
		return puTrain(ctx, p.c, true)
	case labelIs(in, p.c.UI.ProgressButton):
		return Outcome{NextState: state.StatePuProgress}
	case open == nil && labelIs(in, p.c.UI.RetestButton):
		// A retest is a session of the week, so it passes the same recovery
		// gate — the button is not a way around the 24 h block.
		return puRetest(ctx, p.c)
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (PuMenuPhase) Remind(ctx Context) Outcome { return Outcome{} }
