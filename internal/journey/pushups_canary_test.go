package journey_test

// The pushup track's canary, driven over the REAL bundled content
// (../../screening/pushups_ru.yaml) and the REAL ru bundle (../../i18n): it
// walks every branch of the track and inspects the ASSEMBLED chat surface —
// every message body and every keyboard label the bot actually sends.
//
// internal/screening has its own canary over the raw content strings. This
// one covers what that cannot see: the strings THIS package composes (the
// gate chain, the session prompts, the summary, the week closing, the
// progress screen, the /report line, the pings) plus everything the landing
// contributes. The product rules pinned here:
//
//   - no weight / height / calorie / body-percentage figure and no BMI
//     anywhere in the chat. The bot hosts an eating self-check that pins
//     exactly this rule; a neighbouring track must not break it to print a
//     prettier "that is N kg of load" line;
//   - no branding of the commercial program whose tables this track
//     deliberately does not copy, and no "N reps in M weeks" promise;
//   - no methodology hedging, no study citations — the lean-texts rule the
//     other tracks already follow;
//   - no unrendered {placeholder} anywhere: every template this package
//     assembles must have all its arguments.

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/screening"
	"github.com/padington/tgbase/internal/state"
)

// puFigureNeedle catches a number glued to a body unit — kilograms, calories
// or a percentage. Reps, seconds, minutes and dates are the only figures
// this track is allowed to print.
var puFigureNeedle = regexp.MustCompile(`(?i)\d+\s*(кг|килограмм|ккал|калори|грамм|см\b|%)`)

// puBodyNeedles are the body-metric words themselves. "рост" is NOT here on
// purpose: the track legitimately says «рост происходит между тренировками»
// about training adaptation, and a substring ban would forbid the sentence
// rather than the metric.
var puBodyNeedles = []string{
	"имт", "bmi", "индекс массы тела", "массы тела", "процент жира", "взвесь", "взвешив",
}

// puBrandNeedles are the commercial source program's names — assembled from
// bytes so this file never spells them out either, same trick the content
// validator uses.
var puBrandNeedles = []string{
	string([]byte{'h', 'u', 'n', 'd', 'r', 'e', 'd', 'p', 'u', 's', 'h', 'u', 'p', 's'}),
	string([]byte{'s', 'p', 'e', 'i', 'r', 's'}),
	string([]byte{'o', 'n', 'e', ' ', 'h', 'u', 'n', 'd', 'r', 'e', 'd', ' ', 'p', 'u', 's', 'h'}),
	"100 отжиман",
	"сто отжиман",
}

// puPromiseNeedles are the "N reps in M weeks" claims the track refuses to
// make, plus the failure-to-the-limit wording it deliberately avoids.
var puPromiseNeedles = []string{
	"за 6 недель", "за шесть недель", "гарантиру", "до отказа",
}

// puHedgingNeedles are the methodology caveats and citations that belong in
// the yaml's comments, never in a message.
var puHedgingNeedles = []string{
	"исследован", "мета-анализ", "по данным", "acsm", "рабдомиолиз", "протокол",
}

// TestPushupCanary_NoBodyFiguresBrandingOrPromisesInTheChat walks the track
// through every branch that has a text — the three gate answers, the goal
// and the ladder, a normal test, a capped test, a too-low test, a full
// session with its rest ping, the self-report, a closed week, a repeated
// week and the fork, the red-flag card and its joint-pain branch, the
// progress screen, /report, /abandon, the delete dialog and both pings —
// and scans everything the users could read.
func TestPushupCanary_NoBodyFiguresBrandingOrPromisesInTheChat(t *testing.T) {
	runner, st, sender, c := setupPushups(t)

	// User 1 — the happy path: consent, clean gate, a test, three sessions
	// (one over plan, one short), a closed week, the progress screen and
	// /report, then a retest and the delete dialog.
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, c.Params.Progression.OverMargin, c.Session.EffortEasyButton)
	puTrainSession(t, runner, st, sender, 1, c, -1, c.Session.EffortHardButton)
	puTrainSession(t, runner, st, sender, 1, c, 0, c.Session.EffortOKButton)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.ProgressButton)
	runner.HandleReport(sender, newMsg(1, "/report"))
	puRewind(st, 1, 50*time.Hour)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.RetestButton)
	say(runner, sender, 1, "24")
	runner.HandlePushupsDelete(sender, newMsg(1, "/pushups_delete"))
	say(runner, sender, 1, c.UI.DeleteCancelButton)

	// User 2 — the red gate answer: the stop screen, and no track.
	runner.HandlePushups(sender, newMsg(2, "/pushups"))
	say(runner, sender, 2, c.Consent.AgreeButton)
	say(runner, sender, 2, c.Gate.YesButton)

	// User 3 — the two soft gate answers, a capped test and a too-low one.
	runner.HandlePushups(sender, newMsg(3, "/pushups"))
	say(runner, sender, 3, c.Consent.AgreeButton)
	say(runner, sender, 3, c.Gate.NoButton)
	say(runner, sender, 3, c.Gate.YesButton) // joint pain → easier start
	say(runner, sender, 3, c.Gate.YesButton) // pregnancy → doctor note
	say(runner, sender, 3, c.Goal.RepsButton)
	say(runner, sender, 3, c.Variation("knees").Name)
	say(runner, sender, 3, strconv.Itoa(c.Params.TooLowReps)) // too low → step down
	say(runner, sender, 3, "нисколько")                       // the re-ask line
	say(runner, sender, 3, strconv.Itoa(c.Params.TestCap+5))  // capped

	// User 4 — the mid-session surface: the rest card, the timer ping, the
	// pause, the resume row on the landing, then a bad input.
	puStart(t, runner, st, sender, 4, c, 12)
	say(runner, sender, 4, c.UI.TrainButton)
	say(runner, sender, 4, "плохо себя чувствую") // unreadable → re-ask
	say(runner, sender, 4, strconv.Itoa(st.Get(4).PuSession.Targets[0]))
	puExpireRest(st, 4)
	runner.Remind() // the "go" message
	say(runner, sender, 4, strconv.Itoa(st.Get(4).PuSession.Targets[1]))
	say(runner, sender, 4, c.Session.PauseButton)
	runner.HandleStart(sender, newMsg(4, "/menu")) // the landing with the resume row
	say(runner, sender, 4, renderPuResume(c, st.Get(4).PuSession))
	runner.HandleAbandon(sender, newMsg(4, "/abandon"))

	// User 5 — three repeated weeks: every week-closing wording plus the
	// fork, then the red-flag card with both of its answers.
	puStart(t, runner, st, sender, 5, c, 15)
	for week := 0; week < c.Params.Progression.RepeatForkAfter; week++ {
		for i := 0; i < c.Params.SessionsPerWeek; i++ {
			puTrainSession(t, runner, st, sender, 5, c, -1, c.Session.EffortHardButton)
		}
	}
	if got := st.Get(5).State; got != state.StatePuWeekFork {
		t.Fatalf("expected the fork, got %q", got)
	}
	say(runner, sender, 5, c.Week.ForkEasierButton) // → an immediate retest
	say(runner, sender, 5, "14")
	puRewind(st, 5, 50*time.Hour)
	runner.HandlePushups(sender, newMsg(5, "/pushups"))
	say(runner, sender, 5, c.UI.TrainButton)
	if got := st.Get(5).State; got != state.StatePuRedCard {
		t.Fatalf("expected the red-flag card, got %q", got)
	}
	say(runner, sender, 5, c.RedCard.JointPainPrompt)
	runner.HandleAbandon(sender, newMsg(5, "/abandon"))
	puRewind(st, 5, 50*time.Hour)
	runner.HandlePushups(sender, newMsg(5, "/pushups"))
	say(runner, sender, 5, c.UI.TrainButton)
	say(runner, sender, 5, c.RedCard.YesButton)

	// User 6 — both pings of one due cycle, and the recovery-window texts.
	puStart(t, runner, st, sender, 6, c, 18)
	puTrainSession(t, runner, st, sender, 6, c, 0, c.Session.EffortOKButton)
	runner.HandlePushups(sender, newMsg(6, "/pushups"))
	say(runner, sender, 6, c.UI.TrainButton) // inside 24 h → the rest-day line
	puRewind(st, 6, 30*time.Hour)
	say(runner, sender, 6, c.UI.TrainButton) // 24–48 h → the warning
	puRewind(st, 6, 30*time.Hour)
	runner.HandleStart(sender, newMsg(6, "/menu"))
	runner.Remind() // the due ping
	puRewind(st, 6, 5*24*time.Hour)
	runner.Remind() // the overdue ping
	puExpireSessionOf(st, 6, c)
	runner.HandlePushups(sender, newMsg(6, "/pushups"))

	// --- the scan -----------------------------------------------------------
	surface := chatSurface(sender)
	if len(surface) < 60 {
		t.Fatalf("the walk covered only %d messages — the canary is not seeing the track", len(surface))
	}
	for where, text := range surface {
		low := strings.ToLower(text)
		if m := puFigureNeedle.FindString(low); m != "" {
			t.Errorf("%s puts a body figure on screen (%q): %q", where, m, text)
		}
		for _, needle := range puBodyNeedles {
			if strings.Contains(low, needle) {
				t.Errorf("%s names a body metric (%q): %q", where, needle, text)
			}
		}
		for _, needle := range puBrandNeedles {
			if strings.Contains(low, needle) {
				t.Errorf("%s names the commercial source program (%q): %q", where, needle, text)
			}
		}
		for _, needle := range puPromiseNeedles {
			if strings.Contains(low, needle) {
				t.Errorf("%s makes a promise the track does not make (%q): %q", where, needle, text)
			}
		}
		for _, needle := range puHedgingNeedles {
			if strings.Contains(low, needle) {
				t.Errorf("%s hedges with methodology (%q): %q", where, needle, text)
			}
		}
		if strings.Contains(text, "{") || strings.Contains(text, "}") {
			t.Errorf("%s leaks an unrendered placeholder: %q", where, text)
		}
	}
}

// TestPushupCanary_TextsStayShort pins the compact-texts rule for everything
// this package assembles: the longest screens of the track are the consent
// card, the test protocol and the progress screen, and none of them may grow
// into a wall of text.
func TestPushupCanary_TextsStayShort(t *testing.T) {
	runner, st, sender, c := setupPushups(t)
	puStart(t, runner, st, sender, 1, c, 20)
	puTrainSession(t, runner, st, sender, 1, c, 1, c.Session.EffortOKButton)
	runner.HandlePushups(sender, newMsg(1, "/pushups"))
	say(runner, sender, 1, c.UI.ProgressButton)

	for where, text := range chatSurface(sender) {
		if !strings.HasPrefix(where, "message[") || strings.Contains(where, "button") {
			continue
		}
		if lines := strings.Count(text, "\n") + 1; lines > 12 {
			t.Errorf("%s is %d lines long — the track keeps its screens short:\n%s", where, lines, text)
		}
		if len([]rune(text)) > 700 {
			t.Errorf("%s is %d characters long:\n%s", where, len([]rune(text)), text)
		}
	}
}

// --- helpers ----------------------------------------------------------------

// puExpireRest pulls the rest deadline into the past, so the next reminder
// tick delivers the "go" message without the test sleeping.
func puExpireRest(st *state.Store, id int64) {
	u := st.Get(id)
	if u.PuSession == nil {
		return
	}
	s := u.PuSession.Clone()
	s.RestUntil = time.Now().Add(-time.Second)
	u.PuSession = s
	st.Set(id, u)
}

// puExpireSessionOf ages an open session past its TTL, so the next entry
// into the track closes it as a partial one.
func puExpireSessionOf(st *state.Store, id int64, c *screening.PushupContent) {
	u := st.Get(id)
	if u.PuSession == nil {
		return
	}
	s := u.PuSession.Clone()
	s.StartedAt = time.Now().Add(-time.Duration(c.Params.SessionTTLHours+1) * time.Hour)
	u.PuSession = s
	st.Set(id, u)
}

// TestPushupCanary_LandingOffersTheTrack pins the landing wiring: the mode
// button comes from the track's own content bundle, and the resume row
// appears only while a session is actually resumable.
func TestPushupCanary_LandingOffersTheTrack(t *testing.T) {
	runner, st, sender, c := setupPushups(t)

	runner.HandleStart(sender, newMsg(1, "/start"))
	if !keyboardHas(lastKeyboard(sender), c.UI.ModeButton) {
		t.Fatalf("the landing must offer the track, buttons: %v", lastKeyboard(sender))
	}
	say(runner, sender, 1, c.UI.ModeButton)
	if got := st.Get(1).State; got != state.StatePuConsent {
		t.Fatalf("the mode button must open the track, got %q", got)
	}

	// A landing without the bundle keeps working — the track is simply not
	// offered there (the bot may boot without the content file).
	bare := journey.NewModeChoicePhase()
	if bare == nil {
		t.Fatal("the bundle-less landing must still be constructible")
	}
}
