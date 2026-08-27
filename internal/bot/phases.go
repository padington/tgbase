package bot

import (
	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/screening"
)

// phasesFor returns every journey Phase this binary should register: the
// always-on FODMAP diary phases plus the phases of each self-check track
// whose content actually loaded.
//
// It is a plain list rather than inline Register calls so the wiring is
// testable: the Runner keys phases by State(), so a forgotten phase is a
// dead-end state and a duplicated one silently shadows its twin — neither
// shows up until a user walks into it.
func phasesFor(scr *screening.Content, mood *screening.MoodContent, eat *screening.EatingContent) []journey.Phase {
	phases := []journey.Phase{
		journey.NewDefecationPhase(),
		journey.NewProductCategoryPhase(),
		journey.NewProductChoicePhase(),
		journey.NewStageChoicePhase(),
		journey.NewStageCheckinPhase(),
	}
	if scr != nil {
		// The home landing rides along with the ADHD track: with no
		// screening content there is no mode fork at all and /start keeps
		// its legacy direct-to-diary behavior.
		phases = append(phases,
			journey.NewModeChoicePhase(),
			journey.NewScrConsentPhase(scr),
			journey.NewScrIntroPhase(scr),
			journey.NewScrAsrsAPhase(scr),
			journey.NewScrAsrsAGatePhase(scr),
			journey.NewScrAsrsBPhase(scr),
			journey.NewScrAsrsBGatePhase(scr),
			journey.NewScrWursFormPhase(scr),
			journey.NewScrWursPhase(scr),
			journey.NewScrWursGatePhase(scr),
			journey.NewScrOnsetPhase(scr),
			journey.NewScrOnsetAgePhase(scr),
			journey.NewScrDomainsAdultPhase(scr),
			journey.NewScrDomainsChildPhase(scr),
			journey.NewScrReferralPhase(scr),
			journey.NewScrReportPhase(scr),
			journey.NewScrDeleteConfirmPhase(scr),
		)
	}
	if mood != nil {
		phases = append(phases,
			journey.NewMoodConsentPhase(mood),
			journey.NewMoodMenuPhase(mood),
			journey.NewMoodQuestionPhase(mood),
			journey.NewMoodCrisisPhase(mood),
			journey.NewMoodQ10Phase(mood),
			journey.NewMoodReportPhase(mood),
			journey.NewMoodWho5Phase(mood),
			journey.NewMoodOfferPhq9Phase(mood),
			journey.NewMoodGad7Phase(mood),
			journey.NewMoodOfferGad7Phase(mood),
			journey.NewMoodDeleteConfirmPhase(mood),
		)
	}
	if eat != nil {
		phases = append(phases,
			journey.NewEatConsentPhase(eat),
			journey.NewEatMenuPhase(eat),
			journey.NewEatEdeqsPhase(eat),
			journey.NewEatBesPhase(eat),
			journey.NewEatNiasPhase(eat),
			journey.NewEatReportPhase(eat),
			journey.NewEatDeleteConfirmPhase(eat),
		)
	}
	return phases
}
