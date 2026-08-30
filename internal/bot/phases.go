package bot

import (
	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/screening"
)

// phasesFor returns every journey Phase this binary should register: the
// always-on FODMAP diary phases plus the phases of each track whose content
// actually loaded.
//
// It is a plain list rather than inline Register calls so the wiring is
// testable: the Runner keys phases by State(), so a forgotten phase is a
// dead-end state and a duplicated one silently shadows its twin — neither
// shows up until a user walks into it.
func phasesFor(scr *screening.Content, mood *screening.MoodContent, eat *screening.EatingContent,
	pu *screening.PushupContent) []journey.Phase {
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
		//
		// It is also the one phase that takes another track's content: the
		// pushup bundle owns that track's landing button and its
		// «подход N/M» resume row (the whole track keeps its texts in one
		// file), so the landing needs the bundle to render them. Without it
		// the landing simply has no pushup row.
		landing := journey.NewModeChoicePhase()
		if pu != nil {
			landing = journey.NewModeChoicePhaseWithPushups(pu)
		}
		phases = append(phases,
			landing,
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
	if pu != nil {
		// The pushup track: entry chain (consent → safety gate → goal →
		// ladder rung → max test), the track menu, the session automaton
		// (set → rest → self-report → week fork) and the two screens that
		// stand outside a session — the red-flag card and progress.
		phases = append(phases,
			journey.NewPuConsentPhase(pu),
			journey.NewPuGatePhase(pu),
			journey.NewPuGoalPhase(pu),
			journey.NewPuVariationPhase(pu),
			journey.NewPuMaxTestPhase(pu),
			journey.NewPuMenuPhase(pu),
			journey.NewPuSetPhase(pu),
			journey.NewPuRestPhase(pu),
			journey.NewPuEffortPhase(pu),
			journey.NewPuWeekForkPhase(pu),
			journey.NewPuRedCardPhase(pu),
			journey.NewPuProgressPhase(pu),
			journey.NewPuDeleteConfirmPhase(pu),
		)
	}
	return phases
}
