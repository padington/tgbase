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
phase.stage_choice.prompt: "Recommended for {name}: {recommended}. Pick a volume:"
phase.stage_choice.invalid: "Tap one of the amounts."
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
product.amount_template.pieces: "{value}"
product.amount_template.grams: "{value}g"
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
	runner.Register(journey.NewProductCategoryPhase())
	runner.Register(journey.NewProductChoicePhase())
	runner.Register(journey.NewStageChoicePhase())
	runner.Register(journey.NewStageCheckinPhase())

	return runner, stateStore, sender
}

// lowAmountFor returns the low-stage button label for the seed-test catalog,
// matching the labels rendered by Product.AmountLabel under the test i18n.
func lowAmountFor(t *testing.T, product string) string {
	t.Helper()
	switch product {
	case "Apple":
		return "0.25"
	case "Cashews":
		return "10g"
	default:
		t.Fatalf("lowAmountFor: unexpected product %q", product)
		return ""
	}
}

func mediumAmountFor(t *testing.T, product string) string {
	t.Helper()
	switch product {
	case "Apple":
		return "0.5"
	case "Cashews":
		return "20g"
	default:
		t.Fatalf("mediumAmountFor: unexpected product %q", product)
		return ""
	}
}

func highAmountFor(t *testing.T, product string) string {
	t.Helper()
	switch product {
	case "Apple":
		return "1"
	case "Cashews":
		return "30g"
	default:
		t.Fatalf("highAmountFor: unexpected product %q", product)
		return ""
	}
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

func TestHandleText_PickProductTransitionsToStageChoice(t *testing.T) {
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
	if d.State != state.StateAwaitingStageChoice {
		t.Errorf("expected AwaitingStageChoice, got %q", d.State)
	}
	if d.CurrentProduct != picked {
		t.Errorf("CurrentProduct: got %q, want %q", d.CurrentProduct, picked)
	}
	if d.CurrentStage != "" {
		t.Errorf("CurrentStage should be unset until user picks a volume, got %q", d.CurrentStage)
	}
}

func TestHandleText_StageChoicePicksLowStartsCheckin(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))

	runner.HandleText(sender, newMsg(1, lowAmountFor(t, picked)))

	d := store.Get(1)
	if d.State != state.StateAwaitingStageCheckin {
		t.Errorf("expected AwaitingStageCheckin, got %q", d.State)
	}
	if d.CurrentStage != products.StageLow {
		t.Errorf("expected CurrentStage=low, got %q", d.CurrentStage)
	}
	if d.Products[picked].LastStage != products.StageLow {
		t.Errorf("expected progress LastStage=low, got %q", d.Products[picked].LastStage)
	}
}

func TestHandleText_StageChoicePicksMediumSkipsLow(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))

	runner.HandleText(sender, newMsg(1, mediumAmountFor(t, picked)))

	d := store.Get(1)
	if d.State != state.StateAwaitingStageCheckin {
		t.Errorf("expected AwaitingStageCheckin, got %q", d.State)
	}
	if d.CurrentStage != products.StageMedium {
		t.Errorf("expected CurrentStage=medium, got %q", d.CurrentStage)
	}
}

func TestHandleText_StageChoiceInvalidStays(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))

	runner.HandleText(sender, newMsg(1, "blue"))

	d := store.Get(1)
	if d.State != state.StateAwaitingStageChoice {
		t.Errorf("expected to stay in AwaitingStageChoice, got %q", d.State)
	}
	if d.CurrentStage != "" {
		t.Errorf("CurrentStage should remain unset, got %q", d.CurrentStage)
	}
	if got := sender.lastText(); got != "Tap one of the amounts." {
		t.Errorf("expected invalid prompt, got %q", got)
	}
}

func TestHandleText_StageChoicePromptIncludesRecommendation(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))

	got := sender.lastText()
	if !contains(got, picked) {
		t.Errorf("stage-choice prompt should name the picked product, got %q", got)
	}
}

func TestHandleText_StageYesAdvances(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))
	runner.HandleText(sender, newMsg(1, lowAmountFor(t, picked)))

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
	runner.HandleText(sender, newMsg(1, lowAmountFor(t, picked)))
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

func TestHandleText_StageStartingHighOneYesCompletes(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))
	runner.HandleText(sender, newMsg(1, highAmountFor(t, picked)))

	runner.HandleText(sender, newMsg(1, "yes"))

	d := store.Get(1)
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("starting at high then yes should complete; got state %q", d.State)
	}
	if d.Products[picked].Status != "completed" {
		t.Errorf("expected completed status, got %q", d.Products[picked].Status)
	}
}

func TestHandleText_StageNoMarksNotTolerated(t *testing.T) {
	runner, store, sender := setup(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	picked := store.Get(1).OfferedProducts[0]
	runner.HandleText(sender, newMsg(1, picked))
	runner.HandleText(sender, newMsg(1, lowAmountFor(t, picked)))

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
	runner.HandleText(sender, newMsg(1, lowAmountFor(t, picked)))

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

// setupWithCategories returns a runner whose seed declares two
// categories (fruits, legumes) so the category-picker actually renders
// instead of auto-skipping. Used by the tests below that exercise the
// category step + pagination.
func setupWithCategories(t *testing.T) (*journey.Runner, *state.Store, *mockSender) {
	t.Helper()
	dir := t.TempDir()

	i18nDir := filepath.Join(dir, "i18n")
	if err := os.Mkdir(i18nDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(i18nDir, "en.yaml"), []byte(`phase.defecation.prompt: "D?"
phase.defecation.invalid: "1/2/3"
phase.defecation.reminder: "still?"
phase.product.category.prompt: "Pick a category:"
phase.product.category.invalid: "Tap a category."
phase.product.prompt: "Pick a food:"
phase.product.invalid: "Tap a food."
phase.product.exhausted: "All done."
phase.stage_choice.prompt: "Volume for {name}? recommended {recommended}"
phase.stage_choice.invalid: "Tap an amount."
phase.stage.prompt: "Take {description}. Check in {checkin}."
phase.stage.next: "Now take {description}."
phase.stage.checkin: "OK after {description}?"
phase.stage.checkin_invalid: "yes/no"
phase.stage.completed: "{name} done!"
phase.stage.not_tolerated: "{name} skipped."
button.product.back: "Back"
button.product.prev: "Prev"
button.product.next: "Next"
button.yes: "yes"
button.no: "no"
product.measure_template.pieces: "{value} {name}"
product.measure_template.grams: "{value}g {name}"
product.amount_template.pieces: "{value}"
product.amount_template.grams: "{value}g"
about: "info"
cmd.report.empty: "nothing"
cmd.report.heading: "Progress:"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	trans, err := i18n.Load(i18nDir, "en")
	if err != nil {
		t.Fatal(err)
	}

	prodSeed := filepath.Join(dir, "products.yaml")
	// 14 fruits exercises pagination at productPageSize=12; legumes
	// stays small so the multi-category step shows two buttons.
	if err := os.WriteFile(prodSeed, []byte(`categories:
  - id: fruits
    emoji: "F"
    name_localized: { en: Fruits }
  - id: legumes
    emoji: "L"
    name_localized: { en: Legumes }

products:
  - { name: Apple,      category: fruits, emoji: "A", fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Apricot,    category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Banana,     category: fruits,             fodmap: low,  measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Blackberry, category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Cherry,     category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Date,       category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Fig,        category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Grape,      category: fruits,             fodmap: low,  measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Kiwi,       category: fruits,             fodmap: low,  measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Lemon,      category: fruits,             fodmap: low,  measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Mango,      category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Nectarine,  category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Orange,     category: fruits,             fodmap: low,  measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Pear,       category: fruits,             fodmap: high, measure: pieces, stages: { low: 1, medium: 2, high: 3 } }
  - { name: Cashew,     category: legumes,            fodmap: high, measure: grams,  stages: { low: 10, medium: 20, high: 30 } }
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
	runner.Register(journey.NewProductCategoryPhase())
	runner.Register(journey.NewProductChoicePhase())
	runner.Register(journey.NewStageChoicePhase())
	runner.Register(journey.NewStageCheckinPhase())

	return runner, stateStore, sender
}

func TestHandleText_DefecationToCategoryWhenMultipleCategories(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))

	d := store.Get(1)
	if d.State != state.StateAwaitingProductCategory {
		t.Errorf("expected AwaitingProductCategory, got %q", d.State)
	}
	if got := sender.lastText(); got != "Pick a category:" {
		t.Errorf("expected category prompt, got %q", got)
	}
}

func TestHandleText_PickCategoryEntersChoiceWithPickerCategorySet(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))

	runner.HandleText(sender, newMsg(1, "F Fruits"))

	d := store.Get(1)
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("expected AwaitingProductChoice, got %q", d.State)
	}
	if d.PickerCategory != "fruits" {
		t.Errorf("PickerCategory: got %q", d.PickerCategory)
	}
	if d.PickerPage != 0 {
		t.Errorf("PickerPage: got %d", d.PickerPage)
	}
	// Page size = 12, but fruits has 14 entries → first page lists 12.
	if got := len(d.OfferedProducts); got != 12 {
		t.Errorf("OfferedProducts size: got %d", got)
	}
}

func TestHandleText_CategoryInvalidStays(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))

	runner.HandleText(sender, newMsg(1, "blah"))

	if got := store.Get(1).State; got != state.StateAwaitingProductCategory {
		t.Errorf("expected to stay in category, got %q", got)
	}
}

func TestHandleText_NextPagingAdvancesAndShrinksOfferedTail(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	runner.HandleText(sender, newMsg(1, "F Fruits"))

	runner.HandleText(sender, newMsg(1, "Next"))

	d := store.Get(1)
	if d.PickerPage != 1 {
		t.Errorf("PickerPage: got %d", d.PickerPage)
	}
	// 14 fruits - 12 first page = 2 on second page.
	if got := len(d.OfferedProducts); got != 2 {
		t.Errorf("OfferedProducts size on page 2: got %d", got)
	}
}

func TestHandleText_PrevPagingDecrements(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	runner.HandleText(sender, newMsg(1, "F Fruits"))
	runner.HandleText(sender, newMsg(1, "Next"))

	runner.HandleText(sender, newMsg(1, "Prev"))

	if got := store.Get(1).PickerPage; got != 0 {
		t.Errorf("PickerPage: got %d", got)
	}
}

func TestHandleText_BackReturnsToCategory(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	runner.HandleText(sender, newMsg(1, "F Fruits"))

	runner.HandleText(sender, newMsg(1, "Back"))

	d := store.Get(1)
	if d.State != state.StateAwaitingProductCategory {
		t.Errorf("expected category state after Back, got %q", d.State)
	}
	if d.PickerCategory != "" {
		t.Errorf("PickerCategory should clear after Back, got %q", d.PickerCategory)
	}
}

func TestHandleText_PickProductFromCategoryFlowsToStageChoice(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	runner.HandleText(sender, newMsg(1, "F Fruits"))

	// Per-product emoji on Apple is "A"; the rendered button label is
	// "A Apple". Tap the rendered label so we exercise the productLabel
	// match path.
	runner.HandleText(sender, newMsg(1, "A Apple"))

	d := store.Get(1)
	if d.State != state.StateAwaitingStageChoice {
		t.Errorf("expected AwaitingStageChoice, got %q", d.State)
	}
	if d.CurrentProduct != "Apple" {
		t.Errorf("CurrentProduct: got %q", d.CurrentProduct)
	}
}

func TestHandleText_ProductLabelFallsBackToCategoryEmoji(t *testing.T) {
	// Banana has no per-product emoji; should fall back to the category
	// emoji ("L" for legumes is wrong; banana is in fruits with emoji
	// "F" — verify "F Banana" matches).
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	runner.HandleText(sender, newMsg(1, "F Fruits"))

	runner.HandleText(sender, newMsg(1, "F Banana"))

	if got := store.Get(1).CurrentProduct; got != "Banana" {
		t.Errorf("CurrentProduct: got %q", got)
	}
}

func TestHandleText_CategoryBackReturnsToDefecation(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2")) // → category

	runner.HandleText(sender, newMsg(1, "Back"))

	d := store.Get(1)
	if d.State != state.StateAwaitingDefecation {
		t.Errorf("expected AwaitingDefecation after Back, got %q", d.State)
	}
	if got := sender.lastText(); got != "D?" {
		t.Errorf("expected defecation prompt, got %q", got)
	}
}

func TestHandleText_StageChoiceBackReturnsToProductChoice(t *testing.T) {
	runner, store, sender := setupWithCategories(t)
	runner.HandleStart(sender, newMsg(1, "/start"))
	runner.HandleText(sender, newMsg(1, "2"))
	runner.HandleText(sender, newMsg(1, "F Fruits"))
	runner.HandleText(sender, newMsg(1, "A Apple")) // → stage choice

	runner.HandleText(sender, newMsg(1, "Back"))

	d := store.Get(1)
	if d.State != state.StateAwaitingProductChoice {
		t.Errorf("expected AwaitingProductChoice after Back, got %q", d.State)
	}
	if d.CurrentProduct != "" {
		t.Errorf("CurrentProduct should be cleared after Back, got %q", d.CurrentProduct)
	}
	if d.CurrentStage != "" {
		t.Errorf("CurrentStage should be cleared after Back, got %q", d.CurrentStage)
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
