package journey

import (
	"strconv"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// PuConsentPhase owns StatePuConsent — the single entry gate of the pushup
// track. Consent is asked ONCE for the whole track: what the track is, what
// it costs in time, what gets stored (numbers and dates — nothing about the
// body), plus the one short "this is training, not medicine" line. Agreeing
// records the consent timestamp AND creates the program shell, which is what
// the safety gate then fills in; declining leaves no trace at all.
type PuConsentPhase struct {
	c *screening.PushupContent
}

func NewPuConsentPhase(c *screening.PushupContent) *PuConsentPhase {
	return &PuConsentPhase{c: c}
}

func (PuConsentPhase) State() state.StateKind { return state.StatePuConsent }

func (p *PuConsentPhase) Setup(ctx Context) Outcome {
	if hasPuConsent(ctx.User) {
		return Outcome{NextState: puEntryState(p.c, ctx.User, ctx.Now())}
	}
	// The "≈ N minutes" figure is generated like every other number of the
	// track — from a mid-range base through the same planner — so no number
	// in any text is hand-written.
	plan := p.c.Plan(screening.PlanInput{Base: puConsentSampleBase, DayIdx: 1})
	body := renderContent(p.c.Consent.Body, map[string]string{
		"minutes": strconv.Itoa(p.c.EstimateMinutes(plan)),
	})
	text := p.c.Consent.Title + "\n\n" + body + "\n\n⚠️ " + p.c.Meta.Disclaimer
	oc := scrText(text)
	oc.Keyboard = [][]string{{p.c.Consent.AgreeButton}, {p.c.Consent.LaterButton}}
	return oc
}

func (p *PuConsentPhase) Collect(ctx Context, input string) Outcome {
	now := ctx.Now()
	switch in := normText(input); {
	case labelIs(in, p.c.Consent.AgreeButton):
		return Outcome{
			NextState: state.StatePuGate,
			Mutate: func(u *state.UserData) {
				t := now
				u.PuConsentAt = &t
				u.Pushups = &state.PushupProgram{StartedAt: now}
			},
		}
	case labelIs(in, p.c.Consent.LaterButton):
		// Nothing was recorded — the track is simply not started.
		oc := scrText(p.c.Consent.Declined)
		oc.RemoveKeyboard = true
		oc.NextState = testExitState
		return oc
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (PuConsentPhase) Remind(ctx Context) Outcome { return Outcome{} }
