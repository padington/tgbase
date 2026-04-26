package router_test

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/router"
)

// mockSender records every Send call.
type mockSender struct {
	sent []tgbotapi.Chattable
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sent = append(m.sent, c)
	return tgbotapi.Message{}, nil
}

func commandUpdate(cmd string) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "/" + cmd,
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: len(cmd) + 1},
			},
			From: &tgbotapi.User{UserName: "tester"},
			Chat: &tgbotapi.Chat{ID: 1},
		},
	}
}

func textUpdate(text string) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: text,
			From: &tgbotapi.User{UserName: "tester"},
			Chat: &tgbotapi.Chat{ID: 1},
		},
	}
}

func TestDispatch_Command_Known(t *testing.T) {
	s := &mockSender{}
	r := router.New(s)
	r.HandleCommand("ping", func(sender router.Sender, msg *tgbotapi.Message) {
		sender.Send(tgbotapi.NewMessage(msg.Chat.ID, "pong"))
	})

	r.Dispatch(commandUpdate("ping"))

	if len(s.sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(s.sent))
	}
}

func TestDispatch_Command_Unknown(t *testing.T) {
	s := &mockSender{}
	r := router.New(s)

	r.Dispatch(commandUpdate("unknown"))

	if len(s.sent) != 0 {
		t.Fatalf("expected 0 sends, got %d", len(s.sent))
	}
}

func TestDispatch_Text_FirstMatchWins(t *testing.T) {
	s := &mockSender{}
	r := router.New(s)

	calls := 0
	always := func(msg *tgbotapi.Message) bool { return true }
	r.HandleText(always, func(sender router.Sender, msg *tgbotapi.Message) { calls++ })
	r.HandleText(always, func(sender router.Sender, msg *tgbotapi.Message) { calls++ })

	r.Dispatch(textUpdate("hello"))

	if calls != 1 {
		t.Fatalf("expected 1 handler call, got %d", calls)
	}
}

func TestDispatch_Text_NoMatch(t *testing.T) {
	s := &mockSender{}
	r := router.New(s)
	r.HandleText(func(msg *tgbotapi.Message) bool { return false }, func(sender router.Sender, msg *tgbotapi.Message) {
		t.Fatal("handler should not be called")
	})

	r.Dispatch(textUpdate("hello"))
}

func TestDispatch_NilMessage(t *testing.T) {
	s := &mockSender{}
	r := router.New(s)

	// must not panic
	r.Dispatch(tgbotapi.Update{UpdateID: 99, Message: nil})
}
