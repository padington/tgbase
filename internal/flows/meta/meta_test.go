package meta_test

import (
	"runtime"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/flows/meta"
	"github.com/padington/tgbase/internal/router"
)

type mockSender struct {
	sent []tgbotapi.Chattable
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sent = append(m.sent, c)
	return tgbotapi.Message{}, nil
}

func newMsg() *tgbotapi.Message {
	return &tgbotapi.Message{
		MessageID: 1,
		From:      &tgbotapi.User{ID: 10, UserName: "tester"},
		Chat:      &tgbotapi.Chat{ID: 100},
	}
}

func sentText(c tgbotapi.Chattable) string {
	v, _ := c.(tgbotapi.MessageConfig)
	return v.Text
}

func TestPing(t *testing.T) {
	s := &mockSender{}
	meta.Ping()(s, newMsg())

	if len(s.sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(s.sent))
	}
	if sentText(s.sent[0]) != "pong" {
		t.Fatalf("expected 'pong', got %q", sentText(s.sent[0]))
	}
}

func TestWhoami_ContainsEnvAndOS(t *testing.T) {
	s := &mockSender{}
	meta.Whoami("test-env")(s, newMsg())

	if len(s.sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(s.sent))
	}
	text := sentText(s.sent[0])
	if !strings.Contains(text, "test-env") {
		t.Errorf("expected env label in reply, got %q", text)
	}
	if !strings.Contains(text, runtime.GOOS) {
		t.Errorf("expected GOOS in reply, got %q", text)
	}
}

func TestWhoami_EmptyEnvDefaultsToUnknown(t *testing.T) {
	s := &mockSender{}
	meta.Whoami("")(s, newMsg())
	if !strings.Contains(sentText(s.sent[0]), "unknown") {
		t.Errorf("expected 'unknown' label, got %q", sentText(s.sent[0]))
	}
}

func TestMenu_SendsKeyboardWithTwoButtons(t *testing.T) {
	s := &mockSender{}
	meta.Menu()(s, newMsg())

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
	if len(kb.Keyboard) != 1 || len(kb.Keyboard[0]) != 2 {
		t.Fatalf("expected 1 row with 2 buttons, got %v", kb.Keyboard)
	}

	// verify we expose the right interface to satisfy the compiler
	var _ router.HandlerFunc = meta.Menu()
}
