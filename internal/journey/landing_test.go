package journey_test

// Landing (home menu) scenarios: the universal escape to the landing from
// every family of states, the contextual resume buttons, the return to the
// exact paused position, the report button, and the delete-cancel /
// lost-ResumeState regressions.

import (
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/state"
)

// lastKeyboard returns the reply-keyboard rows of the last sent message
// (nil when the last message carries no reply keyboard).
func lastKeyboard(sender *mockSender) [][]string {
	sent := sender.snapshot()
	if len(sent) == 0 {
		return nil
	}
	m, ok := sent[len(sent)-1].(tgbotapi.MessageConfig)
	if !ok {
		return nil
	}
	kb, ok := m.ReplyMarkup.(tgbotapi.ReplyKeyboardMarkup)
	if !ok {
		return nil
	}
	var rows [][]string
	for _, r := range kb.Keyboard {
		var row []string
		for _, b := range r {
			row = append(row, b.Text)
		}
		rows = append(rows, row)
	}
	return rows
}

func keyboardHas(kb [][]string, label string) bool {
	for _, row := range kb {
		for _, b := range row {
			if b == label {
				return true
			}
		}
	}
	return false
}

// driveToCheckin walks a fresh diary user to the stage check-in and returns
// the picked product name.
func driveToCheckin(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender, id int64) string {
	t.Helper()
	startDiary(runner, sender, id)
	say(runner, sender, id, "2")
	picked := st.Get(id).OfferedProducts[0]
	say(runner, sender, id, picked)
	say(runner, sender, id, lowAmountFor(t, picked))
	if got := st.Get(id).State; got != state.StateAwaitingStageCheckin {
		t.Fatalf("precondition: stage checkin, got %q", got)
	}
	return picked
}

// TestLanding_EscapeFromEveryState is the escape table: from each family of
// states both /start and the 🏠 button label land on the landing WITHOUT
// losing progress.
func TestLanding_EscapeFromEveryState(t *testing.T) {
	rows := []struct {
		name    string
		arrange func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender)
		from    state.StateKind
		check   func(t *testing.T, d state.UserData)
	}{
		{
			name: "fodmap defecation",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				startDiary(runner, sender, 1)
			},
			from: state.StateAwaitingDefecation,
			check: func(t *testing.T, d state.UserData) {
				if d.ReturnState != state.StateAwaitingDefecation {
					t.Errorf("ReturnState not recorded, got %q", d.ReturnState)
				}
			},
		},
		{
			name: "fodmap category",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				st.Set(1, state.UserData{State: state.StateAwaitingProductCategory, Locale: "en"})
			},
			from: state.StateAwaitingProductCategory,
			check: func(t *testing.T, d state.UserData) {
				if d.ReturnState != state.StateAwaitingProductCategory {
					t.Errorf("ReturnState not recorded, got %q", d.ReturnState)
				}
			},
		},
		{
			name: "fodmap product choice",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				startDiary(runner, sender, 1)
				say(runner, sender, 1, "2")
			},
			from: state.StateAwaitingProductChoice,
			check: func(t *testing.T, d state.UserData) {
				if d.ReturnState != state.StateAwaitingProductChoice {
					t.Errorf("ReturnState not recorded, got %q", d.ReturnState)
				}
			},
		},
		{
			name: "fodmap stage choice",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				startDiary(runner, sender, 1)
				say(runner, sender, 1, "2")
				say(runner, sender, 1, st.Get(1).OfferedProducts[0])
			},
			from: state.StateAwaitingStageChoice,
			check: func(t *testing.T, d state.UserData) {
				if d.CurrentProduct == "" {
					t.Error("picked product must survive the escape")
				}
				if d.ReturnState != state.StateAwaitingStageChoice {
					t.Errorf("ReturnState not recorded, got %q", d.ReturnState)
				}
			},
		},
		{
			name: "fodmap stage checkin",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				driveToCheckin(t, runner, st, sender, 1)
			},
			from: state.StateAwaitingStageCheckin,
			check: func(t *testing.T, d state.UserData) {
				if d.CurrentProduct == "" || d.CurrentStage == "" {
					t.Error("active trial must survive the escape")
				}
				if d.Products[d.CurrentProduct].Status != "in_progress" {
					t.Errorf("trial must stay in_progress, got %q", d.Products[d.CurrentProduct].Status)
				}
				if d.ReturnState != state.StateAwaitingStageCheckin {
					t.Errorf("ReturnState not recorded, got %q", d.ReturnState)
				}
			},
		},
		{
			name: "scr consent",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleAdhd(sender, newMsg(1, "/adhd"))
			},
			from: state.StateScrConsent,
			check: func(t *testing.T, d state.UserData) {
				if d.Screening != nil {
					t.Error("no consent given — nothing must be recorded")
				}
			},
		},
		{
			name: "scr asrs question",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleAdhd(sender, newMsg(1, "/adhd"))
				say(runner, sender, 1, "Agree")
				say(runner, sender, 1, "Start")
				say(runner, sender, 1, "Very Often")
				say(runner, sender, 1, "Very Often")
			},
			from: state.StateScrAsrsA,
			check: func(t *testing.T, d state.UserData) {
				if d.Screening == nil || len(d.Screening.AsrsAnswers) != 2 {
					t.Fatalf("answers must survive the escape: %+v", d.Screening)
				}
				if d.Screening.ResumeState != state.StateScrAsrsA {
					t.Errorf("ResumeState not recorded, got %q", d.Screening.ResumeState)
				}
			},
		},
		{
			name: "scr wurs question",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleAdhd(sender, newMsg(1, "/adhd"))
				say(runner, sender, 1, "Agree")
				say(runner, sender, 1, "Start")
				for _, l := range rep("Very Often", 6) {
					say(runner, sender, 1, l)
				}
				say(runner, sender, 1, "Continue")
				for _, l := range rep("Very Often", 12) {
					say(runner, sender, 1, l)
				}
				say(runner, sender, 1, "Continue")
				say(runner, sender, 1, "Masculine")
				say(runner, sender, 1, "W1")
			},
			from: state.StateScrWurs,
			check: func(t *testing.T, d state.UserData) {
				if d.Screening == nil || len(d.Screening.WursAnswers) != 1 {
					t.Fatalf("WURS answers must survive the escape: %+v", d.Screening)
				}
				if d.Screening.ResumeState != state.StateScrWurs {
					t.Errorf("ResumeState not recorded, got %q", d.Screening.ResumeState)
				}
			},
		},
		{
			name: "scr life domains",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				walkToDomains(t, runner, sender, 1)
				say(runner, sender, 1, "Yes")
			},
			from: state.StateScrDomainsAdult,
			check: func(t *testing.T, d state.UserData) {
				if d.Screening == nil || d.Screening.AdultDomainIdx != 1 {
					t.Fatalf("domain cursor must survive the escape: %+v", d.Screening)
				}
				if d.Screening.ResumeState != state.StateScrDomainsAdult {
					t.Errorf("ResumeState not recorded, got %q", d.Screening.ResumeState)
				}
			},
		},
		{
			name: "scr delete confirm",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleAdhd(sender, newMsg(1, "/adhd"))
				say(runner, sender, 1, "Agree")
				say(runner, sender, 1, "Start")
				say(runner, sender, 1, "Very Often")
				runner.HandleAdhdDelete(sender, newMsg(1, "/adhd_delete"))
			},
			from: state.StateScrDeleteConfirm,
			check: func(t *testing.T, d state.UserData) {
				if d.Screening == nil || len(d.Screening.AsrsAnswers) != 1 {
					t.Fatalf("nothing confirmed — progress must survive: %+v", d.Screening)
				}
				if d.Screening.ResumeState != state.StateScrAsrsA {
					t.Errorf("resume must point at the interrupted question, got %q", d.Screening.ResumeState)
				}
			},
		},
		{
			name: "mood consent",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleMood(sender, newMsg(1, "/mood"))
			},
			from: state.StateMoodConsent,
			check: func(t *testing.T, d state.UserData) {
				if d.Mood != nil {
					t.Error("no consent given — nothing must be recorded")
				}
			},
		},
		{
			name: "mood question",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleMood(sender, newMsg(1, "/mood"))
				say(runner, sender, 1, "Begin")
				say(runner, sender, 1, "Several days")
				say(runner, sender, 1, "Several days")
			},
			from: state.StateMoodQuestion,
			check: func(t *testing.T, d state.UserData) {
				if d.Mood == nil || len(d.Mood.Answers) != 2 {
					t.Fatalf("answers must survive the escape: %+v", d.Mood)
				}
				if d.Mood.ResumeState != state.StateMoodQuestion {
					t.Errorf("ResumeState not recorded, got %q", d.Mood.ResumeState)
				}
			},
		},
		{
			name: "mood crisis card",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				runner.HandleMood(sender, newMsg(1, "/mood"))
				say(runner, sender, 1, "Begin")
				for _, l := range rep("Not at all", 8) {
					say(runner, sender, 1, l)
				}
				say(runner, sender, 1, "More than half")
			},
			from: state.StateMoodCrisis,
			check: func(t *testing.T, d state.UserData) {
				if d.Mood == nil || len(d.Mood.Answers) != 9 {
					t.Fatalf("answers must survive the escape: %+v", d.Mood)
				}
				if d.Mood.ResumeState != state.StateMoodCrisis {
					t.Errorf("a run paused on the crisis card must resume on the card, got %q", d.Mood.ResumeState)
				}
			},
		},
		{
			name: "mood delete confirm",
			arrange: func(t *testing.T, runner *journey.Runner, st *state.Store, sender *mockSender) {
				driveMood(t, runner, sender, 1, moodAnswers("Several days", "Not at all"))
				runner.HandleMoodDelete(sender, newMsg(1, "/mood_delete"))
			},
			from: state.StateMoodDeleteConfirm,
			check: func(t *testing.T, d state.UserData) {
				if d.MoodResult == nil {
					t.Error("nothing confirmed — the stored result must survive")
				}
			},
		},
	}

	escapes := []struct {
		name string
		do   func(runner *journey.Runner, sender *mockSender)
	}{
		{"start command", func(runner *journey.Runner, sender *mockSender) {
			runner.HandleStart(sender, newMsg(1, "/start"))
		}},
		{"home button", func(runner *journey.Runner, sender *mockSender) {
			say(runner, sender, 1, "Home")
		}},
	}

	for _, row := range rows {
		for _, esc := range escapes {
			t.Run(row.name+" / "+esc.name, func(t *testing.T) {
				runner, st, sender, _ := setupScr(t)
				row.arrange(t, runner, st, sender)
				if got := st.Get(1).State; got != row.from {
					t.Fatalf("arrange: expected %q, got %q", row.from, got)
				}
				esc.do(runner, sender)
				d := st.Get(1)
				if d.State != state.StateAwaitingModeChoice {
					t.Fatalf("expected the landing, got %q", d.State)
				}
				row.check(t, d)
			})
		}
	}
}

func TestLanding_ContextualButtons(t *testing.T) {
	t.Run("fresh user: modes and report only", func(t *testing.T) {
		runner, _, sender, _ := setupScr(t)
		runner.HandleStart(sender, newMsg(1, "/start"))
		kb := lastKeyboard(sender)
		for _, want := range []string{"Diary", "Check", "Mood", "Report"} {
			if !keyboardHas(kb, want) {
				t.Errorf("landing must offer %q, got %v", want, kb)
			}
		}
		for _, wrong := range []string{"Resume check", "Resume mood"} {
			if keyboardHas(kb, wrong) {
				t.Errorf("fresh landing must not offer %q, got %v", wrong, kb)
			}
		}
	})

	t.Run("unfinished ADHD run adds a resume button on top", func(t *testing.T) {
		runner, _, sender, _ := setupScr(t)
		runner.HandleAdhd(sender, newMsg(1, "/adhd"))
		say(runner, sender, 1, "Agree")
		say(runner, sender, 1, "Start")
		say(runner, sender, 1, "Very Often")
		runner.HandleStart(sender, newMsg(1, "/start"))
		kb := lastKeyboard(sender)
		if !keyboardHas(kb, "Resume check") {
			t.Fatalf("expected the ADHD resume button, got %v", kb)
		}
		if len(kb) == 0 || kb[0][0] != "Resume check" {
			t.Errorf("resume button must be the first row, got %v", kb)
		}
		if keyboardHas(kb, "Resume mood") {
			t.Errorf("no mood run — no mood resume button, got %v", kb)
		}
	})

	t.Run("unfinished mood run adds a resume button", func(t *testing.T) {
		runner, _, sender, _ := setupScr(t)
		runner.HandleMood(sender, newMsg(1, "/mood"))
		say(runner, sender, 1, "Begin")
		say(runner, sender, 1, "Several days")
		runner.HandleStart(sender, newMsg(1, "/start"))
		kb := lastKeyboard(sender)
		if !keyboardHas(kb, "Resume mood") {
			t.Fatalf("expected the mood resume button, got %v", kb)
		}
	})

	t.Run("active trial adds a named diary resume button and prompt", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		picked := driveToCheckin(t, runner, st, sender, 1)
		runner.HandleStart(sender, newMsg(1, "/start"))
		want := "Resume diary: " + picked + " (low)"
		if kb := lastKeyboard(sender); !keyboardHas(kb, want) {
			t.Fatalf("expected %q, got %v", want, kb)
		}
		if text := sender.lastText(); !contains(text, "active "+picked+" (low)") {
			t.Errorf("landing prompt must name the trial, got %q", text)
		}
	})

	t.Run("all three contexts at once", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		picked := driveToCheckin(t, runner, st, sender, 1)
		runner.HandleAdhd(sender, newMsg(1, "/adhd"))
		say(runner, sender, 1, "Agree")
		say(runner, sender, 1, "Start")
		say(runner, sender, 1, "Very Often")
		runner.HandleStart(sender, newMsg(1, "/start"))
		say(runner, sender, 1, "Mood")
		say(runner, sender, 1, "Begin")
		say(runner, sender, 1, "Several days")
		runner.HandleStart(sender, newMsg(1, "/start"))
		kb := lastKeyboard(sender)
		for _, want := range []string{"Resume diary: " + picked + " (low)", "Resume check", "Resume mood"} {
			if !keyboardHas(kb, want) {
				t.Errorf("expected %q on the landing, got %v", want, kb)
			}
		}
	})
}

func TestLanding_ResumeReturnsToSamePosition(t *testing.T) {
	t.Run("ADHD: same question", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		runner.HandleAdhd(sender, newMsg(1, "/adhd"))
		say(runner, sender, 1, "Agree")
		say(runner, sender, 1, "Start")
		say(runner, sender, 1, "Very Often")
		say(runner, sender, 1, "Very Often")
		runner.HandleStart(sender, newMsg(1, "/start"))
		say(runner, sender, 1, "Resume check")
		if got := st.Get(1).State; got != state.StateScrAsrsA {
			t.Fatalf("expected scr_asrs_a, got %q", got)
		}
		if got := sender.lastText(); !contains(got, "Question 3 of 6") {
			t.Errorf("resume must land on question 3, got %q", got)
		}
	})

	t.Run("mood: same question", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		runner.HandleMood(sender, newMsg(1, "/mood"))
		say(runner, sender, 1, "Begin")
		say(runner, sender, 1, "Several days")
		say(runner, sender, 1, "Several days")
		runner.HandleStart(sender, newMsg(1, "/start"))
		say(runner, sender, 1, "Resume mood")
		if got := st.Get(1).State; got != state.StateMoodQuestion {
			t.Fatalf("expected mood_question, got %q", got)
		}
		if got := sender.lastText(); !contains(got, "Question 3 of 9") {
			t.Errorf("resume must land on question 3, got %q", got)
		}
	})

	t.Run("mood: crisis card again", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		runner.HandleMood(sender, newMsg(1, "/mood"))
		say(runner, sender, 1, "Begin")
		for _, l := range rep("Not at all", 8) {
			say(runner, sender, 1, l)
		}
		say(runner, sender, 1, "More than half")
		runner.HandleStart(sender, newMsg(1, "/start"))
		say(runner, sender, 1, "Resume mood")
		if got := st.Get(1).State; got != state.StateMoodCrisis {
			t.Fatalf("expected the crisis card again, got %q", got)
		}
		if got := sender.lastText(); !contains(got, "crisis lead") {
			t.Errorf("crisis card must be re-shown, got %q", got)
		}
	})

	t.Run("diary: back to the check-in, nothing interrupted", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		picked := driveToCheckin(t, runner, st, sender, 1)
		runner.HandleStart(sender, newMsg(1, "/start"))
		say(runner, sender, 1, "Resume diary: "+picked+" (low)")
		d := st.Get(1)
		if d.State != state.StateAwaitingStageCheckin {
			t.Fatalf("expected stage checkin, got %q", d.State)
		}
		if d.CurrentProduct != picked || d.Products[picked].Status != "in_progress" {
			t.Errorf("trial must survive the round trip: %+v", d.Products[picked])
		}
		if d.ReturnState != "" {
			t.Errorf("ReturnState must be consumed, got %q", d.ReturnState)
		}
		if got := sender.lastText(); !contains(got, "Take ") {
			t.Errorf("stage Setup should re-fire, got %q", got)
		}
	})
}

func TestLanding_ReportButton(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleStart(sender, newMsg(1, "/start"))
	say(runner, sender, 1, "Report")
	if got := sender.lastText(); got != "nothing" {
		t.Errorf("empty report expected, got %q", got)
	}
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("report must not leave the landing, got %q", got)
	}

	driveMood(t, runner, sender, 1, moodAnswers("Several days", "Not at all"))
	runner.HandleStart(sender, newMsg(1, "/start"))
	say(runner, sender, 1, "Report")
	if got := sender.lastText(); !contains(got, "Mood ") || !contains(got, "of 27") {
		t.Errorf("report must include the mood line, got %q", got)
	}
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Errorf("report must not leave the landing, got %q", got)
	}
}

// TestScr_DeleteCancelMidTestReturnsToQuestion pins the fixed trap: «Keep»
// on /adhd_delete mid-test returns to the interrupted question instead of
// ejecting the user (where a later "Start" used to wipe progress silently).
func TestScr_DeleteCancelMidTestReturnsToQuestion(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	runner.HandleAdhd(sender, newMsg(1, "/adhd"))
	say(runner, sender, 1, "Agree")
	say(runner, sender, 1, "Start")
	say(runner, sender, 1, "Very Often")
	say(runner, sender, 1, "Very Often")

	runner.HandleAdhdDelete(sender, newMsg(1, "/adhd_delete"))
	if got := st.Get(1).State; got != state.StateScrDeleteConfirm {
		t.Fatalf("expected delete confirm, got %q", got)
	}
	say(runner, sender, 1, "Keep")

	d := st.Get(1)
	if d.State != state.StateScrAsrsA {
		t.Fatalf("cancel must return to the question, got %q", d.State)
	}
	if d.Screening == nil || len(d.Screening.AsrsAnswers) != 2 {
		t.Fatalf("progress must survive the cancel: %+v", d.Screening)
	}
	if got := sender.lastText(); !contains(got, "Question 3 of 6") {
		t.Errorf("the interrupted question must be re-asked, got %q", got)
	}
}

func TestMood_DeleteCancelMidCrisisReturnsToCard(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	// A previous completed run gives /mood_delete something to offer even
	// mid-test; then a fresh run paused on the crisis card.
	driveMood(t, runner, sender, 1, moodAnswers("Several days", "Not at all"))
	runner.HandleMood(sender, newMsg(1, "/mood"))
	say(runner, sender, 1, "Begin")
	for _, l := range rep("Not at all", 8) {
		say(runner, sender, 1, l)
	}
	say(runner, sender, 1, "More than half")
	if got := st.Get(1).State; got != state.StateMoodCrisis {
		t.Fatalf("precondition: crisis, got %q", got)
	}

	runner.HandleMoodDelete(sender, newMsg(1, "/mood_delete"))
	say(runner, sender, 1, "Keep")

	d := st.Get(1)
	if d.State != state.StateMoodCrisis {
		t.Fatalf("cancel must return to the crisis card, got %q", d.State)
	}
	if got := sender.lastText(); !contains(got, "crisis lead") {
		t.Errorf("crisis card must be re-shown, got %q", got)
	}
}

// TestScr_ResumeSurvivesLostResumeState pins the answers-based resume
// detection: a run whose ResumeState was lost (old delete-cancel bug) still
// offers Continue, and Continue derives the position from the answers.
func TestScr_ResumeSurvivesLostResumeState(t *testing.T) {
	t.Run("mid-ASRS", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		now := time.Now()
		st.Set(1, state.UserData{
			State:  state.StateIdle,
			Locale: "en",
			Screening: &state.ScreeningProgress{
				AsrsAnswers: []int{4, 4, 4},
				ConsentAt:   now, StartedAt: now,
			},
		})
		runner.HandleAdhd(sender, newMsg(1, "/adhd"))
		if got := sender.lastText(); !contains(got, "resumed text") {
			t.Fatalf("expected the resume intro, got %q", got)
		}
		say(runner, sender, 1, "Continue")
		if got := st.Get(1).State; got != state.StateScrAsrsA {
			t.Fatalf("expected scr_asrs_a, got %q", got)
		}
		if got := sender.lastText(); !contains(got, "Question 4 of 6") {
			t.Errorf("derived resume must land on question 4, got %q", got)
		}
	})

	t.Run("mid-domains", func(t *testing.T) {
		runner, st, sender, _ := setupScr(t)
		now := time.Now()
		onset := true
		st.Set(1, state.UserData{
			State:  state.StateIdle,
			Locale: "en",
			Screening: &state.ScreeningProgress{
				AsrsAnswers:    rep4(18),
				WursForm:       "m",
				WursAnswers:    rep4(25),
				OnsetChild:     &onset,
				AdultDomainIdx: 2,
				AdultDomains:   []string{"work_study"},
				ConsentAt:      now, StartedAt: now,
			},
		})
		runner.HandleAdhd(sender, newMsg(1, "/adhd"))
		say(runner, sender, 1, "Continue")
		if got := st.Get(1).State; got != state.StateScrDomainsAdult {
			t.Fatalf("expected scr_domains_adult, got %q", got)
		}
		if got := sender.lastText(); !contains(got, "Sphere 3 of 5") {
			t.Errorf("derived resume must land on domain 3, got %q", got)
		}
	})
}

// rep4 builds n scores of 4 for synthetic ScreeningProgress fixtures.
func rep4(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = 4
	}
	return out
}

// TestLanding_HomeButtonOnFodmapKeyboards pins the 🏠 button presence on
// every FODMAP keyboard (defecation, product picker, stage choice) and its
// escape behavior when tapped.
func TestLanding_HomeButtonOnFodmapKeyboards(t *testing.T) {
	runner, st, sender, _ := setupScr(t)

	startDiary(runner, sender, 1)
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Home") {
		t.Errorf("defecation keyboard must offer 🏠, got %v", kb)
	}

	say(runner, sender, 1, "2")
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Home") {
		t.Errorf("product picker keyboard must offer 🏠, got %v", kb)
	}

	say(runner, sender, 1, st.Get(1).OfferedProducts[0])
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Home") {
		t.Errorf("stage choice keyboard must offer 🏠, got %v", kb)
	}

	say(runner, sender, 1, "Home")
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Fatalf("🏠 tap must land on the landing, got %q", got)
	}
	if d := st.Get(1); d.CurrentProduct == "" || d.ReturnState != state.StateAwaitingStageChoice {
		t.Errorf("🏠 must preserve the picked product and record the way back: %+v", d.State)
	}
}

// TestLanding_HomeButtonOnCategoryKeyboard needs a multi-category catalog —
// the two-product default auto-skips the category step.
func TestLanding_HomeButtonOnCategoryKeyboard(t *testing.T) {
	runner, st, sender := setupWithCategories(t)
	startDiary(runner, sender, 1)
	say(runner, sender, 1, "2")
	if got := st.Get(1).State; got != state.StateAwaitingProductCategory {
		t.Fatalf("precondition: category state, got %q", got)
	}
	if kb := lastKeyboard(sender); !keyboardHas(kb, "Home") {
		t.Errorf("category keyboard must offer 🏠, got %v", kb)
	}
	say(runner, sender, 1, "Home")
	if got := st.Get(1).State; got != state.StateAwaitingModeChoice {
		t.Fatalf("🏠 tap must land on the landing, got %q", got)
	}
}

// TestLanding_NoRemindersOnLanding pins the reminder isolation: a user who
// escaped mid-diary to the landing gets no nudges while parked there.
func TestLanding_NoRemindersOnLanding(t *testing.T) {
	runner, st, sender, _ := setupScr(t)
	driveToCheckin(t, runner, st, sender, 1)
	runner.HandleStart(sender, newMsg(1, "/start"))

	resetSender(sender)
	runner.Remind()
	if sent := sender.snapshot(); len(sent) != 0 {
		t.Fatalf("no reminders may reach a user on the landing, got %d messages", len(sent))
	}
}
