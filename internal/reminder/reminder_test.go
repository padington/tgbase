package reminder_test

import (
	"context"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/reminder"
	"github.com/padington/tgbase/internal/state"
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

func chatID(c tgbotapi.Chattable) int64 {
	if mc, ok := c.(tgbotapi.MessageConfig); ok {
		return mc.ChatID
	}
	return 0
}

func TestTick_NudgesStaleAwaitingUser(t *testing.T) {
	store := state.NewStore()
	store.Set(7, state.UserData{
		State:     state.StateAwaitingHowamiAnswer,
		ChatID:    700,
		EnteredAt: time.Now().Add(-5 * time.Minute),
	})
	sender := &mockSender{}

	w := reminder.New(store, sender, reminder.Config{ReminderAfter: 1 * time.Minute})
	w.Tick()

	sent := sender.snapshot()
	if len(sent) != 1 {
		t.Fatalf("expected 1 nudge, got %d", len(sent))
	}
	if got := chatID(sent[0]); got != 700 {
		t.Errorf("expected nudge to chat=700, got chat=%d", got)
	}
	if !store.Get(7).ReminderSent {
		t.Error("expected ReminderSent=true after nudge")
	}
}

func TestTick_SkipsRecentUser(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{
		State:     state.StateAwaitingHowamiAnswer,
		ChatID:    100,
		EnteredAt: time.Now(),
	})
	sender := &mockSender{}

	w := reminder.New(store, sender, reminder.Config{ReminderAfter: 1 * time.Minute})
	w.Tick()

	if got := len(sender.snapshot()); got != 0 {
		t.Errorf("expected 0 nudges for recent user, got %d", got)
	}
	if store.Get(1).ReminderSent {
		t.Error("expected ReminderSent to stay false")
	}
}

func TestTick_SkipsAlreadyReminded(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{
		State:        state.StateAwaitingHowamiAnswer,
		ChatID:       100,
		EnteredAt:    time.Now().Add(-1 * time.Hour),
		ReminderSent: true,
	})
	sender := &mockSender{}

	w := reminder.New(store, sender, reminder.Config{ReminderAfter: 1 * time.Minute})
	w.Tick()

	if got := len(sender.snapshot()); got != 0 {
		t.Errorf("expected 0 nudges (already reminded), got %d", got)
	}
}

func TestTick_SkipsIdleUser(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{
		State:     state.StateIdle,
		ChatID:    100,
		EnteredAt: time.Now().Add(-1 * time.Hour),
	})
	sender := &mockSender{}

	w := reminder.New(store, sender, reminder.Config{ReminderAfter: 1 * time.Minute})
	w.Tick()

	if got := len(sender.snapshot()); got != 0 {
		t.Errorf("idle users should never be nudged, got %d sends", got)
	}
}

func TestTick_NudgesOnlyStaleSubset(t *testing.T) {
	store := state.NewStore()
	store.Set(1, state.UserData{State: state.StateAwaitingHowamiAnswer, ChatID: 100, EnteredAt: time.Now().Add(-1 * time.Hour)})
	store.Set(2, state.UserData{State: state.StateAwaitingHowamiAnswer, ChatID: 200, EnteredAt: time.Now()})
	store.Set(3, state.UserData{State: state.StateIdle, ChatID: 300})
	store.Set(4, state.UserData{State: state.StateAwaitingHowamiAnswer, ChatID: 400, EnteredAt: time.Now().Add(-1 * time.Hour), ReminderSent: true})

	sender := &mockSender{}
	w := reminder.New(store, sender, reminder.Config{ReminderAfter: 1 * time.Minute})
	w.Tick()

	sent := sender.snapshot()
	if len(sent) != 1 {
		t.Fatalf("expected exactly 1 nudge (only user 1 qualifies), got %d", len(sent))
	}
	if got := chatID(sent[0]); got != 100 {
		t.Errorf("expected nudge to chat=100, got chat=%d", got)
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	store := state.NewStore()
	sender := &mockSender{}
	w := reminder.New(store, sender, reminder.Config{ScanInterval: 10 * time.Millisecond, ReminderAfter: 1 * time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not exit within 500ms after ctx cancel")
	}
}
