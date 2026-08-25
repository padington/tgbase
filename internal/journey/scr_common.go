package journey

import (
	"strconv"
	"strings"

	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// Helpers shared by the ADHD-screening phases (scr_*.go). All screening
// texts come from the screening.Content bundle and reach the user through
// the "scr.text" pass-through i18n key — nothing user-visible is hardcoded
// here.

// normText normalizes user input for button-label matching, following the
// stage.go pattern.
func normText(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// labelIs reports whether normalized input in is a tap on label.
func labelIs(in, label string) bool {
	return in == strings.ToLower(label)
}

// renderContent substitutes {placeholder} tokens in a content template.
func renderContent(tpl string, args map[string]string) string {
	out := tpl
	for k, v := range args {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// scrText wraps an already-assembled content string into the scr.text
// pass-through outcome. Callers set NextState / Keyboard / Mutate on top.
func scrText(text string) Outcome {
	return Outcome{
		ReplyKey:  "scr.text",
		ReplyArgs: map[string]any{"text": text},
	}
}

// testExitState is where every exit from a self-check lands — finish (the
// result/report chain), gate pause, consent decline, postpone, /abandon and
// mid-test delete: the home landing. The recorded FODMAP detour
// (ReturnState) is deliberately KEPT: the landing offers it through the
// contextual «back to the diary» button (diaryResumeState) instead of
// auto-dropping the user into a mid-diary question — that auto-drop was the
// pre-landing legacy behavior and read as a non-sequitur right after a test.
const testExitState = state.StateAwaitingModeChoice

// deleteReturnState is where a closed delete-confirmation returns when it
// was NOT entered mid-test: the state the command interrupted — recorded to
// ReturnState by enterDeleteConfirm (a FODMAP question or the landing
// itself) — or idle. Callers must clear ReturnState via Mutate on the same
// outcome.
func deleteReturnState(u state.UserData) state.StateKind {
	if u.ReturnState != "" {
		return u.ReturnState
	}
	return state.StateIdle
}

// clearReturnState is the Mutate counterpart of deleteReturnState; also used
// by the landing's resume-diary button, which consumes the detour.
func clearReturnState(u *state.UserData) {
	u.ReturnState = ""
}

// isScreeningState reports whether kind is one of the scr_* states.
// A deliberate enumeration, not a lookup over registered phases: the answer
// must not depend on wiring. StateAwaitingModeChoice is NOT a screening
// state — the fork belongs to both modes.
func isScreeningState(kind state.StateKind) bool {
	switch kind {
	case state.StateScrConsent,
		state.StateScrIntro,
		state.StateScrAsrsA,
		state.StateScrAsrsAGate,
		state.StateScrAsrsB,
		state.StateScrAsrsBGate,
		state.StateScrWursForm,
		state.StateScrWurs,
		state.StateScrWursGate,
		state.StateScrOnset,
		state.StateScrOnsetAge,
		state.StateScrDomainsAdult,
		state.StateScrDomainsChild,
		state.StateScrReferral,
		state.StateScrReport,
		state.StateScrDeleteConfirm:
		return true
	}
	return false
}

// resumableScrState reports whether kind is a scr_* state a paused run can
// meaningfully return to: the question, gate and follow-up states — not the
// consent/intro gates themselves and not the delete confirmation. Only these
// are ever recorded as Screening.ResumeState.
func resumableScrState(kind state.StateKind) bool {
	return isScreeningState(kind) &&
		kind != state.StateScrConsent &&
		kind != state.StateScrIntro &&
		kind != state.StateScrDeleteConfirm
}

// screeningHasAnswers reports whether the run holds any actual progress —
// the resume-mode signal that survives a lost ResumeState (e.g. after a
// cancelled /adhd_delete in older versions).
func screeningHasAnswers(s *state.ScreeningProgress) bool {
	return s != nil && (len(s.AsrsAnswers) > 0 || s.WursForm != "" ||
		len(s.WursAnswers) > 0 || s.OnsetChild != nil ||
		s.AdultDomainIdx > 0 || s.ChildDomainIdx > 0)
}

// screeningResumeState derives the state a run should continue at purely
// from the recorded answers — the fallback when no usable ResumeState is
// recorded. Gates are skipped: their intermediate texts re-appear in the
// final summary anyway.
func screeningResumeState(c *screening.Content, s *state.ScreeningProgress) state.StateKind {
	switch {
	case s == nil:
		return state.StateScrAsrsA
	case len(s.AsrsAnswers) < len(c.ASRS.PartA.Items):
		return state.StateScrAsrsA
	case len(s.AsrsAnswers) < len(c.ASRS.PartA.Items)+len(c.ASRS.PartB.Items):
		return state.StateScrAsrsB
	case s.WursForm == "":
		return state.StateScrWursForm
	case len(s.WursAnswers) < len(c.WURS.Items):
		return state.StateScrWurs
	case s.OnsetChild == nil:
		return state.StateScrOnset
	case !*s.OnsetChild && s.OnsetAge == 0:
		return state.StateScrOnsetAge
	case s.AdultDomainIdx < len(c.Module.Domains.Items):
		return state.StateScrDomainsAdult
	default:
		return state.StateScrDomainsChild
	}
}

// isFodmapJourneyState reports whether kind is a FODMAP-diary journey state
// worth returning to after a screening detour.
func isFodmapJourneyState(kind state.StateKind) bool {
	switch kind {
	case state.StateAwaitingDefecation,
		state.StateAwaitingProductCategory,
		state.StateAwaitingProductChoice,
		state.StateAwaitingStageChoice,
		state.StateAwaitingStageCheckin:
		return true
	}
	return false
}

// scaleKeyboard renders a 5-option scale as five vertical one-button rows,
// so long labels are not cut off by Telegram.
func scaleKeyboard(scale []screening.ScaleOption) [][]string {
	rows := make([][]string, 0, len(scale))
	for _, opt := range scale {
		rows = append(rows, []string{opt.Label})
	}
	return rows
}

// matchScale resolves normalized input against scale labels (labels only —
// digits are NOT accepted: the 0–4 scores collide with the 1–5 column
// numbering of the paper WURS blank).
func matchScale(scale []screening.ScaleOption, in string) (score int, ok bool) {
	for _, opt := range scale {
		if labelIs(in, opt.Label) {
			return opt.Score, true
		}
	}
	return 0, false
}

// progressLine renders the "Question N of M" service line.
func progressLine(c *screening.Content, current, total int) string {
	return renderContent(c.Module.UI.Progress, map[string]string{
		"current": strconv.Itoa(current),
		"total":   strconv.Itoa(total),
	})
}

// blockTitle names the block a resume would land in, for the resume intro.
func blockTitle(c *screening.Content, kind state.StateKind) string {
	switch kind {
	case state.StateScrAsrsA, state.StateScrAsrsAGate:
		return c.Module.Results.Instruments.AsrsA.Title
	case state.StateScrAsrsB, state.StateScrAsrsBGate:
		return c.Module.Results.Instruments.AsrsB.Title
	case state.StateScrWursForm, state.StateScrWurs, state.StateScrWursGate:
		return c.Module.Results.Instruments.Wurs.Title
	case state.StateScrOnset, state.StateScrOnsetAge:
		return c.Module.CriterionB.Title
	case state.StateScrDomainsAdult, state.StateScrDomainsChild:
		return c.Module.Results.ContextFacts.Heading
	default:
		return c.Module.Results.Instruments.AsrsA.Title
	}
}
