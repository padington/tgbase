package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// EatConsentPhase owns StateEatConsent — the single entry gate of the eating
// track (EDE-QS / BES / NIAS). Consent is asked ONCE for the whole track:
// the short what-this-is + what-gets-stored text plus the
// screening-not-a-diagnosis disclaimer. Agreeing records the track-wide
// consent timestamp and opens the track menu; declining leaves no trace.
// Users with consent already given (or with any stored eating data, which
// implies it) are routed straight to the menu.
type EatConsentPhase struct {
	c *screening.EatingContent
}

func NewEatConsentPhase(c *screening.EatingContent) *EatConsentPhase {
	return &EatConsentPhase{c: c}
}

func (EatConsentPhase) State() state.StateKind { return state.StateEatConsent }

func (p *EatConsentPhase) Setup(ctx Context) Outcome {
	if hasEatConsent(ctx.User) {
		return Outcome{NextState: state.StateEatMenu}
	}
	m := p.c.Module
	text := m.Consent.Title + "\n\n" + m.Consent.Body + "\n\n⚠️ " + m.Meta.Disclaimer
	oc := scrText(text)
	oc.Keyboard = [][]string{{m.Consent.AgreeButton}, {m.Consent.LaterButton}}
	return oc
}

func (p *EatConsentPhase) Collect(ctx Context, input string) Outcome {
	m := p.c.Module
	switch in := normText(input); {
	case labelIs(in, m.Consent.AgreeButton):
		now := ctx.Now()
		return Outcome{
			NextState: state.StateEatMenu,
			Mutate: func(u *state.UserData) {
				t := now
				u.EatConsentAt = &t
			},
		}
	case labelIs(in, m.Consent.LaterButton):
		// Nothing was recorded. Land home; an active diary position stays
		// reachable from the landing.
		oc := scrText(m.Consent.Declined)
		oc.RemoveKeyboard = true
		oc.NextState = testExitState
		return oc
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (EatConsentPhase) Remind(ctx Context) Outcome { return Outcome{} }
