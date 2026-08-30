package journey

import (
	"strconv"
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// PuProgressPhase owns StatePuProgress — the whole history of the track in
// ONE message of at most six lines: base and level, where the week stands,
// the last test, a text sparkline of the recent open sets, the lifetime
// volume and the week streak.
//
// No charts and no images in v1 by decision: the sparkline is two numbers
// and eight blocks, which is enough to see a direction, and pulling in a
// plotting dependency before the track has proven retention would be
// backwards. Note what is NOT here: no body figure of any kind — this track
// shares the bot with an eating self-check that pins exactly that rule.
type PuProgressPhase struct {
	c *screening.PushupContent
}

func NewPuProgressPhase(c *screening.PushupContent) *PuProgressPhase {
	return &PuProgressPhase{c: c}
}

func (PuProgressPhase) State() state.StateKind { return state.StatePuProgress }

func (p *PuProgressPhase) Setup(ctx Context) Outcome {
	prog := ctx.User.Pushups
	if prog == nil || prog.Base <= 0 {
		return Outcome{NextState: puEntryState(p.c, ctx.User, ctx.Now())}
	}

	lines := []string{
		renderContent(p.c.UI.ProgressBase, map[string]string{
			"base":      strconv.Itoa(prog.Base),
			"level":     strconv.Itoa(p.c.Level(prog.Base)),
			"variation": strings.ToLower(puVariationName(p.c, prog.Variation)),
		}),
		renderContent(p.c.UI.ProgressWeek, map[string]string{
			"week":    strconv.Itoa(prog.WeekIdx + 1),
			"session": strconv.Itoa(prog.SessionInWeek + 1),
			"total":   strconv.Itoa(p.c.Params.SessionsPerWeek),
		}),
	}
	if t := ctx.User.PuTest; t != nil {
		lines = append(lines, renderContent(p.c.UI.ProgressTest, map[string]string{
			"date": t.TakenAt.Format(reportDateLayout),
			"reps": strconv.Itoa(t.Reps),
		}))
	}
	if spark := puSparkline(ctx.User.PuHistory, puSparkPoints); spark != "" {
		lines = append(lines, renderContent(p.c.UI.ProgressOpen, map[string]string{
			"sparkline": spark,
		}))
	}
	lines = append(lines,
		renderContent(p.c.UI.ProgressVolume, map[string]string{
			"total": strconv.Itoa(prog.TotalReps),
		}),
		renderContent(p.c.UI.ProgressStreak, map[string]string{
			"weeks": strconv.Itoa(prog.StreakWeeks),
		}),
	)

	oc := scrText(strings.Join(lines, "\n"))
	oc.Keyboard = [][]string{{p.c.UI.TrainButton}, {homeLabel(ctx)}}
	return oc
}

func (p *PuProgressPhase) Collect(ctx Context, input string) Outcome {
	if labelIs(normText(input), p.c.UI.TrainButton) {
		return puTrain(ctx, p.c, false)
	}
	// The screen is read-only: anything else simply reopens the menu.
	return Outcome{NextState: state.StatePuMenu}
}

func (PuProgressPhase) Remind(ctx Context) Outcome { return Outcome{} }
