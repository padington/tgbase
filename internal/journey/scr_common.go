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

// exitState is where a screening exit (pause, decline, finish, delete)
// lands: the recorded FODMAP detour state, or idle. Callers must clear
// ReturnState via Mutate on the same outcome.
func exitState(u state.UserData) state.StateKind {
	if u.ReturnState != "" {
		return u.ReturnState
	}
	return state.StateIdle
}

// clearReturnState is the Mutate counterpart of exitState.
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
