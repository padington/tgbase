package survey_test

import (
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/flows/survey"
	"github.com/padington/tgbase/internal/state"
)

type mockSender struct {
	sent []tgbotapi.Chattable
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sent = append(m.sent, c)
	return tgbotapi.Message{}, nil
}

func newMsg(userID int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		MessageID: 1,
		From:      &tgbotapi.User{ID: userID, UserName: "tester"},
		Chat:      &tgbotapi.Chat{ID: 100},
		Text:      text,
	}
}

func sentText(c tgbotapi.Chattable) string {
	v, _ := c.(tgbotapi.MessageConfig)
	return v.Text
}

func TestStart_SetsStateAndSendsKeyboard(t *testing.T) {
	store := state.NewStore()
	s := &mockSender{}

	before := time.Now()
	survey.Start(store)(s, newMsg(1, "/start"))
	after := time.Now()

	d := store.Get(1)
	if d.State != state.StateAwaitingHowamiAnswer {
		t.Fatal("expected StateAwaitingHowamiAnswer after /start")
	}
	if d.ChatID != 100 {
		t.Errorf("expected ChatID=100, got %d", d.ChatID)
	}
	if d.EnteredAt.Before(before) || d.EnteredAt.After(after) {
		t.Errorf("EnteredAt %v outside expected window [%v, %v]", d.EnteredAt, before, after)
	}
	if d.ReminderSent {
		t.Error("expected ReminderSent=false on fresh /start")
	}

	if len(s.sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(s.sent))
	}
	msg, ok := s.sent[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatal("expected MessageConfig")
	}
	kb, ok := msg.ReplyMarkup.(tgbotapi.ReplyKeyboardMarkup)
	if !ok {
		t.Fatal("expected ReplyKeyboardMarkup")
	}
	if len(kb.Keyboard[0]) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(kb.Keyboard[0]))
	}
}

func TestStart_ResetsReminderSentOnRestart(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{
		State:        state.StateIdle,
		ReminderSent: true,
		EnteredAt:    time.Now().Add(-1 * time.Hour),
	})
	s := &mockSender{}

	survey.Start(store)(s, newMsg(1, "/start"))

	d := store.Get(1)
	if d.ReminderSent {
		t.Error("expected ReminderSent to reset to false on /start")
	}
	if time.Since(d.EnteredAt) > time.Second {
		t.Errorf("expected EnteredAt to be refreshed, got %v ago", time.Since(d.EnteredAt))
	}
}

func TestAnswerPredicate_TrueWhenAwaiting(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{State: state.StateAwaitingHowamiAnswer})

	pred := survey.AnswerPredicate(store)
	if !pred(newMsg(1, "2")) {
		t.Fatal("expected predicate to return true when awaiting")
	}
}

func TestAnswerPredicate_FalseWhenIdle(t *testing.T) {
	store := state.NewStore()

	pred := survey.AnswerPredicate(store)
	if pred(newMsg(1, "2")) {
		t.Fatal("expected predicate to return false when idle")
	}
}

func TestAnswerHandler_PersistsAndResetsState(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{State: state.StateAwaitingHowamiAnswer})
	s := &mockSender{}

	survey.AnswerHandler(store)(s, newMsg(1, "2"))

	d := store.Get(1)
	if d.State != state.StateIdle {
		t.Fatalf("expected StateIdle after answer, got %q", d.State)
	}
	if d.HowamiAnswer != 2 {
		t.Fatalf("expected HowamiAnswer=2, got %d", d.HowamiAnswer)
	}
	if len(s.sent) != 1 {
		t.Fatalf("expected 1 confirmation send, got %d", len(s.sent))
	}
}

func TestAnswerHandler_InvalidInput_StaysInAwaitingState(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{State: state.StateAwaitingHowamiAnswer})
	s := &mockSender{}

	survey.AnswerHandler(store)(s, newMsg(1, "5"))

	d := store.Get(1)
	if d.State != state.StateAwaitingHowamiAnswer {
		t.Fatal("expected state to remain StateAwaitingHowamiAnswer on invalid input")
	}
	if d.HowamiAnswer != 0 {
		t.Fatalf("expected HowamiAnswer to stay 0, got %d", d.HowamiAnswer)
	}
}
