package journey_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/journey"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/state"
	"github.com/padington/tgbase/internal/store"
)

type mockSender struct {
	mu   sync.Mutex
	sent []tgbotapi.Chattable
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, c)
	return tgbotapi.Message{}, nil
}

func (m *mockSender) snapshot() []tgbotapi.Chattable {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]tgbotapi.Chattable, len(m.sent))
	copy(out, m.sent)
	return out
}

func (m *mockSender) lastText() string {
	sent := m.snapshot()
	if len(sent) == 0 {
		return ""
	}
	last, ok := sent[len(sent)-1].(tgbotapi.MessageConfig)
	if !ok {
		return ""
	}
	return last.Text
}

func setup(t *testing.T) (*journey.Runner, *state.Store, *mockSender) {
	t.Helper()
	dir := t.TempDir()

	i18nDir := filepath.Join(dir, "i18n")
	if err := os.Mkdir(i18nDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(i18nDir, "en.yaml"), []byte(`phase.defecation.prompt: "Defecation? 1/2/3"
phase.defecation.invalid: "Choose 1, 2, or 3."
phase.defecation.reminder: "Still there?"
phase.product.prompt: "Pick:"
phase.product.invalid: "Tap a button."
phase.product.exhausted: "All done."
phase.stage.prompt: "Take {description}. Check in {checkin}."
phase.stage.next: "Now take {description}."
phase.stage.checkin: "OK after {description}?"
phase.stage.checkin_invalid: "yes/no"
phase.stage.completed: "{name} done!"
phase.stage.not_tolerated: "{name} skipped."
about: "Bot info"
button.yes: "yes"
button.no: "no"
cmd.report.empty: "nothing"
cmd.report.heading: "Progress:"
cmd.report.in_progress: "active: {name}({stage})"
cmd.report.completed: "done: {names}"
cmd.report.not_tolerated: "skip: {names}"
cmd.report.interrupted: "halt: {names}"
cmd.abandon.confirmed: "abandoned {name}"
cmd.abandon.no_active: "nothing active"
product.measure_template.pieces: "{value} {name}"
product.measure_template.grams: "{value}g {name}"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	trans, err := i18n.Load(i18nDir, "en")
	if err != nil {
		t.Fatal(err)
	}

	prodSeed := filepath.Join(dir, "products.yaml")
	if err := os.WriteFile(prodSeed, []byte(`- name: Apple
  fodmap: high
  measure: pieces
  stages: { low: 0.25, medium: 0.5, high: 1.0 }
- name: Cashews
  fodmap: high
  measure: grams
  stages: { low: 10, medium: 20, high: 30 }
`), 0o600); err != nil {
		t.Fatal(err)
	}

	settingsSeed := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(settingsSeed, []byte(`defecation_reminder_after: 1m
checkin_interval: 30m
scan_interval: 10s
default_locale: en
`), 0o600); err != nil {
		t.Fatal(err)
	}

	backend := store.NewMemoryBackend()
	cat, err := products.New(backend, prodSeed)
	if err != nil {
		t.Fatal(err)
	}
	settingsStore, err := settings.New(backend, settingsSeed)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStoreFromBackend(backend)
	sender := &mockSender{}

	runner := journey.New(stateStore, sender, cat, settingsStore, trans)
	runner.Register(journey.NewDefecationPhase())
	runner.Register(journey.NewProductChoicePhase())
	runner.Register(journey.NewStageCheckinPhase())

	return runner, stateStore, sender
}

func newMsg(userID int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		MessageID: 1,
		From:      &tgbotapi.User{ID: userID, UserName: "tester", LanguageCode: "en"},
		Chat:      &tgbotapi.Chat{ID: userID * 100},
		Text:      text,
	}
}

func TestHandleStart_TransitionsToDefecation(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))

	d := store.Get(1)
	if d.State != state.StateAwaitingDefecation {
		t.Errorf("expected AwaitingDefecation, got %q", d.State)
	}
	if d.Locale != "en" {
		t.Errorf("locale not set: %q", d.Locale)
	}
	if d.ChatID != 100 {
		t.Errorf("ChatID not captured: %d", d.ChatID)
	}
	if sender.lastText() != "Defecation? 1/2/3" {
		t.Errorf("setup prompt not sent, last text: %q", sender.lastText())
	}
}

func TestHandleStart_MarksInProgressAsInterrupted(t *testing.T) {
	runner, store, sender := setup(t)
	store.Set(1, state.UserData{
		State:          state.StateAwaitingStageCheckin,
		CurrentProduct: "Apple",
		CurrentStage:   products.StageLow,
		Products: map[string]state.ProductProgress{
			"Apple": {Status: "in_progress", LastStage: products.StageLow},
		},
	})

	runner.HandleStart(sender, newMsg(1, "/start"))

	d := store.Get(1)
	if d.Products["Apple"].Status != "interrupted" {
		t.Errorf("expected Apple to be interrupted, got %q", d.Products["Apple"].Status)
	}
	if d.CurrentProduct != "" {
		t.Errorf("CurrentProduct should be cleared, got %q", d.CurrentProduct)
	}
}

func TestHandleText_DefecationToProductChoice(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	sender.mu.Lock()
	sender.sent = nil
	sender.mu.Unlock()

	runner.HandleText(sender, newMsg(1, "2"))

	d := store.Get(1)
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("expected AwaitingProductChoice, got %q", d.State)
	}
	if d.DefecationState != state.DefecationNormal {
		t.Errorf("expected DefecationNormal, got %q", d.DefecationState)
	}
	if got := sender.lastText(); got != "Pick:" {
		t.Errorf("expected product prompt, got %q", got)
	}
	if len(d.OfferedProducts) == 0 {
		t.Error("OfferedProducts should be populated after entering choice phase")
	}
}

func TestHandleText_DefecationInvalidStays(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "blue"))

	if got := store.Get(1).State; got != state.StateAwaitingDefecation {
		t.Errorf("expected to stay AwaitingDefecation, got %q", got)
	}
}

func TestHandleText_PickProductTransitionsToStage(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))

	offered := store.Get(1).OfferedProducts
	if len(offered) == 0 {
		t.Fatal("no products offered")
	}
	picked := offered[0]
	runner.HandleText(sender, newMsg(1, picked))

	d := store.Get(1)
	if d.State != state.StateAwaitingStageCheckin {
		t.Errorf("expected AwaitingStageCheckin, got %q", d.State)
	}
	if d.CurrentProduct != picked {
		t.Errorf("CurrentProduct: got %q, want %q", d.CurrentProduct, picked)
	}
	if d.CurrentStage != products.StageLow {
		t.Errorf("CurrentStage: got %q", d.CurrentStage)
	}
}

func TestHandleText_StageYesAdvances(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))

	runner.HandleText(sender, newMsg(1, "yes"))

	d := store.Get(1)
	if d.State != state.StateAwaitingStageCheckin {
		t.Errorf("expected to remain AwaitingStageCheckin, got %q", d.State)
	}
	if d.CurrentStage != products.StageMedium {
		t.Errorf("expected medium stage, got %q", d.CurrentStage)
	}
}

func TestHandleText_StageHighYesCompletes(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))
	runner.HandleText(sender, newMsg(1, "yes")) // → medium
	runner.HandleText(sender, newMsg(1, "yes")) // → high
	runner.HandleText(sender, newMsg(1, "yes")) // → completed

	d := store.Get(1)
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("expected to bounce back to AwaitingProductChoice, got %q", d.State)
	}
	if d.Products[picked].Status != "completed" {
		t.Errorf("expected completed status, got %q", d.Products[picked].Status)
	}
	if d.CurrentProduct != "" {
		t.Errorf("CurrentProduct should clear after completion: %q", d.CurrentProduct)
	}
}

func TestHandleText_StageNoMarksNotTolerated(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))

	runner.HandleText(sender, newMsg(1, "no"))

	d := store.Get(1)
	if d.Products[picked].Status != "not_tolerated" {
		t.Errorf("expected not_tolerated, got %q", d.Products[picked].Status)
	}
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("expected AwaitingProductChoice after rejection, got %q", d.State)
	}
}

func TestHandleAbout_SendsTranslatedText(t *testing.T) {
	runner, _, sender := setup(t)
	runner.HandleAbout(sender, newMsg(1, "/about"))
	if got := sender.lastText(); got != "Bot info" {
		t.Errorf("got %q", got)
	}
}

func TestHandleReport_EmptyForNewUser(t *testing.T) {
	runner, _, sender := setup(t)
	runner.HandleReport(sender, newMsg(1, "/report"))
	if got := sender.lastText(); got != "nothing" {
		t.Errorf("got %q", got)
	}
}

func TestHandleReport_ListsCompleted(t *testing.T) {
	runner, store, sender := setup(t)
	store.Set(1, state.UserData{
		Locale: "en",
		Products: map[string]state.ProductProgress{
			"Apple":   {Status: "completed", LastStage: products.StageHigh},
			"Cashews": {Status: "not_tolerated", LastStage: products.StageMedium},
		},
	})
	runner.HandleReport(sender, newMsg(1, "/report"))

	got := sender.lastText()
	wantSubstrings := []string{"Progress:", "done: Apple", "skip: Cashews"}
	for _, s := range wantSubstrings {
		if !contains(got, s) {
			t.Errorf("missing %q in report:\n%s", s, got)
		}
	}
}

func TestHandleAbandon_MarksInterruptedAndShowsList(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))

	runner.HandleAbandon(sender, newMsg(1, "/abandon"))

	d := store.Get(1)
	if d.Products[picked].Status != "interrupted" {
		t.Errorf("expected interrupted, got %q", d.Products[picked].Status)
	}
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("expected AwaitingProductChoice, got %q", d.State)
	}
}

func TestHandleAbandon_NoActiveTrial(t *testing.T) {
	runner, _, sender := setup(t)
	runner.HandleAbandon(sender, newMsg(1, "/abandon"))
	if got := sender.lastText(); got != "nothing active" {
		t.Errorf("got %q", got)
	}
}

func TestRemind_DefecationNudgesAfterTimeout(t *testing.T) {
	runner, store, sender := setup(t)
	store.Set(1, state.UserData{
		State:        state.StateAwaitingDefecation,
		ChatID:       100,
		EnteredAt:    time.Now().Add(-5 * time.Minute),
		Locale:       "en",
		ReminderSent: false,
	})

	runner.Remind()

	if !store.Get(1).ReminderSent {
		t.Error("expected ReminderSent=true after Remind")
	}
	if len(sender.snapshot()) == 0 {
		t.Error("expected nudge message sent")
	}
}

func TestRemind_StageCheckinPromptsAfterInterval(t *testing.T) {
	runner, store, sender := setup(t)
	store.Set(1, state.UserData{
		State:          state.StateAwaitingStageCheckin,
		ChatID:         100,
		Locale:         "en",
		CurrentProduct: "Apple",
		CurrentStage:   products.StageLow,
		StageStartedAt: time.Now().Add(-1 * time.Hour),
		CheckinAsked:   false,
	})

	runner.Remind()

	d := store.Get(1)
	if !d.CheckinAsked {
		t.Error("expected CheckinAsked=true after Remind")
	}
	if got := sender.lastText(); got == "" {
		t.Error("expected check-in message sent")
	}
}

func TestRemind_DoesNotDoubleNudge(t *testing.T) {
	runner, store, sender := setup(t)
	store.Set(1, state.UserData{
		State:        state.StateAwaitingDefecation,
		ChatID:       100,
		EnteredAt:    time.Now().Add(-1 * time.Hour),
		ReminderSent: true,
	})

	runner.Remind()
	if got := len(sender.snapshot()); got != 0 {
		t.Errorf("expected 0 nudges (already reminded), got %d", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || stringIndex(haystack, needle) != -1)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
