package journey

import (
	"strconv"
	"time"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// Helpers shared by the mood-module phases (mood_*.go). Like the ADHD scr_*
// chain, all texts come from the content bundle (screening.MoodContent) and
// reach the user through the "scr.text" pass-through i18n key — nothing
// user-visible is hardcoded here. Generic helpers (normText, labelIs,
// renderContent, scrText, testExitState, deleteReturnState, scaleKeyboard,
// matchScale) are shared with scr_common.go.

// isMoodState reports whether kind is one of the mood_* states. A deliberate
// enumeration, not a lookup over registered phases: the answer must not
// depend on wiring.
func isMoodState(kind state.StateKind) bool {
	switch kind {
	case state.StateMoodConsent,
		state.StateMoodMenu,
		state.StateMoodQuestion,
		state.StateMoodCrisis,
		state.StateMoodQ10,
		state.StateMoodReport,
		state.StateMoodWho5Question,
		state.StateMoodOfferPhq9,
		state.StateMoodGad7Question,
		state.StateMoodOfferGad7,
		state.StateMoodDeleteConfirm:
		return true
	}
	return false
}

// resumableMoodState reports whether kind is a PHQ-9 position a paused run
// can meaningfully return to: the question series, the crisis card, or the
// functional (10th) question — not the consent/menu gates, the offers, the
// report transit, or the delete confirmation. Only these are ever recorded
// as Mood.ResumeState. WHO-5 / GAD-7 runs need no recorded position — their
// question phases derive the index from the recorded answers.
func resumableMoodState(kind state.StateKind) bool {
	return kind == state.StateMoodQuestion ||
		kind == state.StateMoodCrisis ||
		kind == state.StateMoodQ10
}

// hasMoodConsent reports whether the single module-wide consent was given.
// Any stored mood data implies consent (nothing is ever recorded without
// it), which also migrates v1 users — they consented per run, so their
// unfinished run or stored result stands in for the missing timestamp.
func hasMoodConsent(u state.UserData) bool {
	return u.MoodConsentAt != nil ||
		u.Mood != nil || u.MoodResult != nil ||
		u.Who5 != nil || u.Who5Result != nil ||
		u.Gad7 != nil || u.Gad7Result != nil
}

// moodEntryState is where the mood module is entered (the landing button and
// /mood): the consent gate for fresh users, the module menu afterwards.
func moodEntryState(u state.UserData) state.StateKind {
	if hasMoodConsent(u) {
		return state.StateMoodMenu
	}
	return state.StateMoodConsent
}

// anyMoodProgress reports whether any of the three instruments has an
// unfinished run — the landing's resume-button condition.
func anyMoodProgress(u state.UserData) bool {
	return u.Mood != nil || u.Who5 != nil || u.Gad7 != nil
}

// phq9ResumeTarget is where an unfinished PHQ-9 run continues: the recorded
// position when usable (the crisis card and the functional question cannot
// be derived from the answer count alone), otherwise the question series,
// which derives its index from the recorded answers.
func phq9ResumeTarget(s *state.MoodProgress) state.StateKind {
	if s != nil && resumableMoodState(s.ResumeState) {
		return s.ResumeState
	}
	return state.StateMoodQuestion
}

// moodResumeTarget is where the landing's «Продолжить тест настроения»
// button lands: straight into the single unfinished run, or the module menu
// when several runs are paused (its per-instrument rows disambiguate).
func moodResumeTarget(u state.UserData) state.StateKind {
	var targets []state.StateKind
	if u.Mood != nil {
		targets = append(targets, phq9ResumeTarget(u.Mood))
	}
	if u.Who5 != nil {
		targets = append(targets, state.StateMoodWho5Question)
	}
	if u.Gad7 != nil {
		targets = append(targets, state.StateMoodGad7Question)
	}
	if len(targets) == 1 {
		return targets[0]
	}
	return state.StateMoodMenu
}

// moodDeleteResumeTarget returns the mid-test position recorded by
// enterDeleteConfirm, if any: the run whose ResumeState carries a state its
// own instrument owns. Checked in a fixed order — with several paused runs
// the first match wins (both runs stay intact either way).
func moodDeleteResumeTarget(u state.UserData) (state.StateKind, bool) {
	if s := u.Mood; s != nil && resumableMoodState(s.ResumeState) {
		return s.ResumeState, true
	}
	if s := u.Who5; s != nil && s.ResumeState == state.StateMoodWho5Question {
		return s.ResumeState, true
	}
	if s := u.Gad7; s != nil && s.ResumeState == state.StateMoodGad7Question {
		return s.ResumeState, true
	}
	return "", false
}

// moodProgressLine renders the "Question N of M" service line from the mood
// module (PHQ-9 and GAD-7).
func moodProgressLine(c *screening.MoodContent, current, total int) string {
	return renderContent(c.Module.UI.Progress, map[string]string{
		"current": strconv.Itoa(current),
		"total":   strconv.Itoa(total),
	})
}

// who5ProgressLine renders the WHO-5 "Statement N of M" service line —
// WHO-5 items are statements, not questions, so the wording differs.
func who5ProgressLine(c *screening.MoodContent, current, total int) string {
	return renderContent(c.Module.Who5.Progress, map[string]string{
		"current": strconv.Itoa(current),
		"total":   strconv.Itoa(total),
	})
}

// moodAgo renders how long ago the previous result was taken, for the
// retest delta line: same day → "today", under a week → days, else weeks.
func moodAgo(c *screening.MoodContent, from, to time.Time) string {
	days := int(to.Sub(from).Hours() / 24)
	switch {
	case days <= 0:
		return c.Module.Results.AgoToday
	case days < 7:
		return renderContent(c.Module.Results.AgoDays, map[string]string{
			"n": strconv.Itoa(days),
		})
	default:
		return renderContent(c.Module.Results.AgoWeeks, map[string]string{
			"n": strconv.Itoa(days / 7),
		})
	}
}
