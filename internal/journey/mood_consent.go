package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodConsentPhase owns StateMoodConsent — the single entry gate of the
// mood module (WHO-5 / PHQ-9 / GAD-7). Consent is asked ONCE for the whole
// module: the short what-this-is + what-gets-stored text plus the
// screening-not-a-diagnosis disclaimer. Agreeing records the module-wide
// consent timestamp and opens the module menu; declining leaves no trace.
// Users with consent already given (or with any stored mood data, which
// implies it) are routed straight to the menu.
type MoodConsentPhase struct {
	c *screening.MoodContent
}

func NewMoodConsentPhase(c *screening.MoodContent) *MoodConsentPhase {
	return &MoodConsentPhase{c: c}
}

func (MoodConsentPhase) State() state.StateKind { return state.StateMoodConsent }

func (p *MoodConsentPhase) Setup(ctx Context) Outcome {
	if hasMoodConsent(ctx.User) {
		return Outcome{NextState: state.StateMoodMenu}
	}
	m := p.c.Module
	text := m.Consent.Title + "\n\n" + m.Consent.Body + "\n\n⚠️ " + m.Meta.Disclaimer
	oc := scrText(text)
	oc.Keyboard = [][]string{{m.Consent.AgreeButton}, {m.Consent.LaterButton}}
	return oc
}

func (p *MoodConsentPhase) Collect(ctx Context, input string) Outcome {
	m := p.c.Module
	switch in := normText(input); {
	case labelIs(in, m.Consent.AgreeButton):
		now := ctx.Now()
		return Outcome{
			NextState: state.StateMoodMenu,
			Mutate: func(u *state.UserData) {
				t := now
				u.MoodConsentAt = &t
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

func (MoodConsentPhase) Remind(ctx Context) Outcome { return Outcome{} }
