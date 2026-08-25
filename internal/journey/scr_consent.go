package journey

import (
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// ScrConsentPhase owns StateScrConsent — explicit consent to processing
// health-related answers, shown before any question. Declining leaves no
// trace: the ScreeningProgress is only created on agreement.
type ScrConsentPhase struct {
	c *screening.Content
}

func NewScrConsentPhase(c *screening.Content) *ScrConsentPhase { return &ScrConsentPhase{c: c} }

func (ScrConsentPhase) State() state.StateKind { return state.StateScrConsent }

func (p *ScrConsentPhase) Setup(ctx Context) Outcome {
	// Unfinished run → consent was already given (ConsentAt is set);
	// go straight to the intro, which renders in resume mode.
	if ctx.User.Screening != nil {
		return Outcome{NextState: state.StateScrIntro}
	}
	con := p.c.Module.Consent
	oc := scrText(con.Title + "\n\n" + con.Body)
	oc.Keyboard = [][]string{{con.AgreeButton}, {con.LaterButton}}
	return oc
}

func (p *ScrConsentPhase) Collect(ctx Context, input string) Outcome {
	con := p.c.Module.Consent
	in := normText(input)
	switch {
	case labelIs(in, con.AgreeButton):
		now := ctx.Now()
		return Outcome{
			NextState: state.StateScrIntro,
			Mutate: func(u *state.UserData) {
				u.Screening = &state.ScreeningProgress{ConsentAt: now, StartedAt: now}
			},
		}
	case labelIs(in, con.LaterButton):
		// Nothing was recorded — Screening was never created.
		oc := scrText(con.Declined)
		oc.RemoveKeyboard = true
		oc.NextState = exitState(ctx.User)
		oc.Mutate = clearReturnState
		return oc
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (ScrConsentPhase) Remind(ctx Context) Outcome { return Outcome{} }

// ScrIntroPhase owns StateScrIntro — what the check consists of, plus the
// short screening-not-a-diagnosis disclaimer. With an unfinished run it
// renders in resume mode: Continue / start over / postpone.
type ScrIntroPhase struct {
	c *screening.Content
}

func NewScrIntroPhase(c *screening.Content) *ScrIntroPhase { return &ScrIntroPhase{c: c} }

func (ScrIntroPhase) State() state.StateKind { return state.StateScrIntro }

// resumeMode is decided by the ACTUAL presence of answers, not just by a
// recorded ResumeState — a run that lost its ResumeState (e.g. a cancelled
// /adhd_delete in older versions) must still offer Continue instead of
// silently wiping progress on "Start".
func (p *ScrIntroPhase) resumeMode(u state.UserData) bool {
	s := u.Screening
	return s != nil && (resumableScrState(s.ResumeState) || screeningHasAnswers(s))
}

// resumeTarget is where Continue lands: the recorded position when usable,
// otherwise the state derived from the recorded answers.
func (p *ScrIntroPhase) resumeTarget(s *state.ScreeningProgress) state.StateKind {
	if s != nil && resumableScrState(s.ResumeState) {
		return s.ResumeState
	}
	return screeningResumeState(p.c, s)
}

func (p *ScrIntroPhase) Setup(ctx Context) Outcome {
	if ctx.User.Screening == nil {
		return Outcome{NextState: state.StateScrConsent}
	}
	m := p.c.Module
	if p.resumeMode(ctx.User) {
		text := m.Intro.Title + "\n\n" + m.UI.Resumed + "\n" +
			blockTitle(p.c, p.resumeTarget(ctx.User.Screening))
		oc := scrText(text)
		oc.Keyboard = [][]string{
			{m.UI.ContinueButton},
			{m.Intro.StartButton}, // in resume mode "Start" means "start over"
			{m.Intro.PostponeButton},
		}
		return oc
	}
	text := m.Intro.Title + "\n\n" + m.Intro.Body +
		"\n\n⚠️ " + m.Meta.Disclaimer
	oc := scrText(text)
	oc.Keyboard = [][]string{{m.Intro.StartButton}, {m.Intro.PostponeButton}}
	return oc
}

func (p *ScrIntroPhase) Collect(ctx Context, input string) Outcome {
	m := p.c.Module
	in := normText(input)
	switch {
	case labelIs(in, m.Intro.StartButton):
		// Fresh run — or a restart that wipes previous raw answers. The
		// previous ScreeningResult is left intact until a new completion.
		now := ctx.Now()
		consentAt := now
		if s := ctx.User.Screening; s != nil {
			consentAt = s.ConsentAt
		}
		return Outcome{
			NextState: state.StateScrAsrsA,
			Mutate: func(u *state.UserData) {
				u.Screening = &state.ScreeningProgress{ConsentAt: consentAt, StartedAt: now}
			},
		}
	case labelIs(in, m.UI.ContinueButton):
		return Outcome{NextState: p.resumeTarget(ctx.User.Screening)}
	case labelIs(in, m.Intro.PostponeButton):
		// Progress and consent are kept — /adhd resumes straight away.
		oc := scrText(m.UI.Paused)
		oc.RemoveKeyboard = true
		oc.NextState = exitState(ctx.User)
		oc.Mutate = clearReturnState
		return oc
	default:
		return Outcome{ReplyKey: "scr.invalid_button"}
	}
}

func (ScrIntroPhase) Remind(ctx Context) Outcome { return Outcome{} }
