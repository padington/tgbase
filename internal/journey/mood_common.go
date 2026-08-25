package journey

import (
	"strconv"
	"time"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// Helpers shared by the mood-screening phases (mood_*.go). Like the ADHD
// scr_* chain, all texts come from the content bundle (screening.MoodContent)
// and reach the user through the "scr.text" pass-through i18n key — nothing
// user-visible is hardcoded here. Generic helpers (normText, labelIs,
// renderContent, scrText, exitState, scaleKeyboard, matchScale) are shared
// with scr_common.go.

// isMoodState reports whether kind is one of the mood_* states. A deliberate
// enumeration, not a lookup over registered phases: the answer must not
// depend on wiring.
func isMoodState(kind state.StateKind) bool {
	switch kind {
	case state.StateMoodConsent,
		state.StateMoodQuestion,
		state.StateMoodCrisis,
		state.StateMoodReport,
		state.StateMoodDeleteConfirm:
		return true
	}
	return false
}

// moodProgressLine renders the "Question N of M" service line from the mood
// module.
func moodProgressLine(c *screening.MoodContent, current, total int) string {
	return renderContent(c.Module.UI.Progress, map[string]string{
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
