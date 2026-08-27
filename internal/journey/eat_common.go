package journey

import (
	"strconv"
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// Helpers shared by the eating-track phases (eat_*.go). Like the ADHD scr_*
// chain and the mood module, all texts come from the content bundle
// (screening.EatingContent) and reach the user through the "scr.text"
// pass-through i18n key — nothing user-visible is hardcoded here. Generic
// helpers (normText, labelIs, renderContent, scrText, testExitState,
// deleteReturnState, scaleKeyboard, matchScale) are shared with
// scr_common.go.

// isEatingState reports whether kind is one of the eat_* states. A deliberate
// enumeration, not a lookup over registered phases: the answer must not
// depend on wiring.
func isEatingState(kind state.StateKind) bool {
	switch kind {
	case state.StateEatConsent,
		state.StateEatMenu,
		state.StateEatEdeqsQuestion,
		state.StateEatBesQuestion,
		state.StateEatNiasQuestion,
		state.StateEatReport,
		state.StateEatDeleteConfirm:
		return true
	}
	return false
}

// resumableEatState reports whether kind is a position a paused eating run
// can meaningfully return to: the three question series, not the
// consent/menu gates, the report transit or the delete confirmation. Only
// these are ever recorded as an EatingProgress.ResumeState — and only by
// enterDeleteConfirm, as the "the dialog was opened mid-test" marker.
func resumableEatState(kind state.StateKind) bool {
	return kind == state.StateEatEdeqsQuestion ||
		kind == state.StateEatBesQuestion ||
		kind == state.StateEatNiasQuestion
}

// eatRunFor returns the transient run that owns a question state.
func eatRunFor(u state.UserData, kind state.StateKind) *state.EatingProgress {
	switch kind {
	case state.StateEatEdeqsQuestion:
		return u.Edeqs
	case state.StateEatBesQuestion:
		return u.Bes
	case state.StateEatNiasQuestion:
		return u.Nias
	}
	return nil
}

// setEatRun is the write counterpart of eatRunFor: it stores run back into
// the field the question state owns.
func setEatRun(u *state.UserData, kind state.StateKind, run *state.EatingProgress) {
	switch kind {
	case state.StateEatEdeqsQuestion:
		u.Edeqs = run
	case state.StateEatBesQuestion:
		u.Bes = run
	case state.StateEatNiasQuestion:
		u.Nias = run
	}
}

// clearEatResumeStates drops the mid-test markers enterDeleteConfirm wrote.
// The marker exists only to tell the delete dialog where it was opened — the
// question phases never read it — so it must not outlive the dialog: a
// leftover marker would make a later /food_delete opened from the landing
// close back INTO the paused run instead of onto the landing.
//
// Called from every way out of the dialog: its own «Оставить» button, the
// landing escapes (🏠 / /start / /menu, via routeToLanding) and /abandon —
// plus enterDeleteConfirm itself, which starts each dialog from a clean
// marker and so covers exits nobody enumerated (a jump straight into
// another track's delete dialog, say).
func clearEatResumeStates(u *state.UserData) {
	u.Edeqs = withoutEatResume(u.Edeqs)
	u.Bes = withoutEatResume(u.Bes)
	u.Nias = withoutEatResume(u.Nias)
}

func withoutEatResume(run *state.EatingProgress) *state.EatingProgress {
	if run == nil || run.ResumeState == "" {
		return run
	}
	c := run.Clone()
	c.ResumeState = ""
	return c
}

// hasEatConsent reports whether the single track-wide consent was given.
// Any stored eating data implies consent — nothing is ever recorded without
// it, so a user who somehow has data is never asked again.
func hasEatConsent(u state.UserData) bool {
	return u.EatConsentAt != nil ||
		u.Edeqs != nil || u.EdeqsResult != nil ||
		u.Bes != nil || u.BesResult != nil ||
		u.Nias != nil || u.NiasResult != nil
}

// eatEntryState is where the eating track is entered (the landing button and
// /food): the consent gate for fresh users, the track menu afterwards.
func eatEntryState(u state.UserData) state.StateKind {
	if hasEatConsent(u) {
		return state.StateEatMenu
	}
	return state.StateEatConsent
}

// anyEatProgress reports whether any of the three instruments has an
// unfinished run — the landing's resume-button condition.
func anyEatProgress(u state.UserData) bool {
	return u.Edeqs != nil || u.Bes != nil || u.Nias != nil
}

// eatResumeTarget is where the landing's «Продолжить тест о еде» button
// lands: straight into the single unfinished run, or the track menu when
// several runs are paused (its per-instrument rows disambiguate). Every
// question phase derives its position from the recorded answers, so no
// ResumeState lookup is needed here.
func eatResumeTarget(u state.UserData) state.StateKind {
	var targets []state.StateKind
	if u.Edeqs != nil {
		targets = append(targets, state.StateEatEdeqsQuestion)
	}
	if u.Bes != nil {
		targets = append(targets, state.StateEatBesQuestion)
	}
	if u.Nias != nil {
		targets = append(targets, state.StateEatNiasQuestion)
	}
	if len(targets) == 1 {
		return targets[0]
	}
	return state.StateEatMenu
}

// eatDeleteResumeTarget returns the mid-test position recorded by
// enterDeleteConfirm, if any. Checked in a fixed order — with several paused
// runs the first match wins (both runs stay intact either way).
func eatDeleteResumeTarget(u state.UserData) (state.StateKind, bool) {
	for _, pair := range []struct {
		run  *state.EatingProgress
		kind state.StateKind
	}{
		{u.Edeqs, state.StateEatEdeqsQuestion},
		{u.Bes, state.StateEatBesQuestion},
		{u.Nias, state.StateEatNiasQuestion},
	} {
		if pair.run != nil && pair.run.ResumeState == pair.kind {
			return pair.kind, true
		}
	}
	return "", false
}

// fodmapContextActive reports whether the user has any FODMAP-diary
// activity — an active trial or any recorded product progress. It gates the
// automatic low-FODMAP line of the doctor report: dietary restriction that
// is medically motivated must not read as a screening finding.
func fodmapContextActive(u state.UserData) bool {
	return u.CurrentProduct != "" || len(u.Products) > 0
}

// eatProgressLine renders the "Question N of M" service line of the eating
// track (used by EDE-QS; BES and NIAS have their own wordings below).
func eatProgressLine(c *screening.EatingContent, current, total int) string {
	return renderContent(c.Module.UI.Progress, map[string]string{
		"current": strconv.Itoa(current),
		"total":   strconv.Itoa(total),
	})
}

// besProgressLine / niasProgressLine — BES asks about groups of statements
// and NIAS about statements, so neither is a "question".
func besProgressLine(c *screening.EatingContent, current, total int) string {
	return renderContent(c.Module.Bes.Progress, map[string]string{
		"current": strconv.Itoa(current),
		"total":   strconv.Itoa(total),
	})
}

func niasProgressLine(c *screening.EatingContent, current, total int) string {
	return renderContent(c.Module.Nias.Progress, map[string]string{
		"current": strconv.Itoa(current),
		"total":   strconv.Itoa(total),
	})
}

// numberedStatements renders a BES group as a numbered list. The statements
// are far too long for keyboard buttons (up to a couple of sentences each),
// so the text carries them and the keyboard carries the numbers.
func numberedStatements(sts []screening.BesStatement) string {
	var b strings.Builder
	for i, st := range sts {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(") ")
		b.WriteString(st.Text)
	}
	return b.String()
}

// numberKeyboard renders 1..n as a single row of short buttons.
func numberKeyboard(n int) [][]string {
	row := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		row = append(row, strconv.Itoa(i))
	}
	return [][]string{row}
}

// matchNumber resolves normalized input against 1..n and returns the
// 0-based index. Digits are unambiguous here — unlike the WURS scale, the
// BES groups carry no printed column numbering.
func matchNumber(in string, n int) (idx int, ok bool) {
	v, err := strconv.Atoi(strings.TrimSpace(in))
	if err != nil || v < 1 || v > n {
		return 0, false
	}
	return v - 1, true
}

// eatResultHeading is the shared "Твой результат" heading of the track.
func eatResultHeading(c *screening.EatingContent) string {
	return c.Module.UI.ResultsHeading
}

// eatResultFooter is the shared footer: the one short disclaimer plus the
// instrument's compact attribution line.
func eatResultFooter(c *screening.EatingContent, attribution string) string {
	return "\n\n⚠️ " + c.Module.Meta.Disclaimer + "\n" + attribution
}
