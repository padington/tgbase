package journey

import (
	"strconv"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// MoodConsentPhase owns StateMoodConsent — the single entry gate of the
// PHQ-9 mood self-check. Fresh users see the short consent (what the test
// is + what gets stored + the screening-not-a-diagnosis disclaimer);
// declining leaves no trace — MoodProgress is only created on agreement.
// With an unfinished run the phase renders in resume mode instead:
// Continue / start over / later, without re-asking consent.
type MoodConsentPhase struct {
	c *screening.MoodContent
}

func NewMoodConsentPhase(c *screening.MoodContent) *MoodConsentPhase {
	return &MoodConsentPhase{c: c}
}

func (MoodConsentPhase) State() state.StateKind { return state.StateMoodConsent }

// resumeQuestion is the 1-based question a resumed run would land on,
// capped at the item count (a run paused on the crisis card has all
// answers already).
func (p *MoodConsentPhase) resumeQuestion(s *state.MoodProgress) int {
	q := len(s.Answers) + 1
	if total := len(p.c.PHQ9.Items); q > total {
		q = total
	}
	return q
}

func (p *MoodConsentPhase) Setup(ctx Context) Outcome {
	m := p.c.Module
	if s := ctx.User.Mood; s != nil {
		text := renderContent(m.Resume.Body, map[string]string{
			"current": strconv.Itoa(p.resumeQuestion(s)),
			"total":   strconv.Itoa(len(p.c.PHQ9.Items)),
		})
		oc := scrText(text)
		oc.Keyboard = [][]string{
			{m.Resume.ContinueButton},
			{m.Resume.RestartButton},
			{m.Resume.LaterButton},
		}
		return oc
	}
	text := m.Consent.Title + "\n\n" + m.Consent.Body + "\n\n⚠️ " + m.Meta.Disclaimer
	oc := scrText(text)
	oc.Keyboard = [][]string{{m.Consent.AgreeButton}, {m.Consent.LaterButton}}
	return oc
}

func (p *MoodConsentPhase) Collect(ctx Context, input string) Outcome {
	m := p.c.Module
	in := normText(input)

	if s := ctx.User.Mood; s != nil {
		// Resume mode.
		switch {
		case labelIs(in, m.Resume.ContinueButton):
			next := state.StateMoodQuestion
			if isMoodState(s.ResumeState) {
				next = s.ResumeState
			}
			return Outcome{NextState: next}
		case labelIs(in, m.Resume.RestartButton):
			// Start over: wipe the raw answers, keep the original consent
			// time. The previous MoodResult survives until a new completion.
			now := ctx.Now()
			consentAt := s.ConsentAt
			return Outcome{
				NextState: state.StateMoodQuestion,
				Mutate: func(u *state.UserData) {
					u.Mood = &state.MoodProgress{ConsentAt: consentAt, StartedAt: now}
				},
			}
		case labelIs(in, m.Resume.LaterButton):
			// Progress and consent are kept — /mood resumes straight away.
			oc := scrText(m.UI.Paused)
			oc.RemoveKeyboard = true
			oc.NextState = exitState(ctx.User)
			oc.Mutate = clearReturnState
			return oc
		default:
			return Outcome{ReplyKey: "scr.invalid_button"}
		}
	}

	switch {
	case labelIs(in, m.Consent.AgreeButton):
		now := ctx.Now()
		return Outcome{
			NextState: state.StateMoodQuestion,
			Mutate: func(u *state.UserData) {
				u.Mood = &state.MoodProgress{ConsentAt: now, StartedAt: now}
			},
		}
	case labelIs(in, m.Consent.LaterButton):
		// Nothing was recorded — Mood was never created.
		oc := scrText(m.Consent.Declined)
		oc.RemoveKeyboard = true
		oc.NextState = exitState(ctx.User)
		oc.Mutate = clearReturnState
		return oc
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (MoodConsentPhase) Remind(ctx Context) Outcome { return Outcome{} }
