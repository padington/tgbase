package reminder

import (
	"context"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/state"
)

// Config controls how often the worker scans and how long a user must sit
// in StateAwaitingHowamiAnswer before getting nudged.
type Config struct {
	ScanInterval  time.Duration
	ReminderAfter time.Duration
}

// Worker periodically nudges users stuck in StateAwaitingHowamiAnswer.
type Worker struct {
	store  *state.Store
	sender router.Sender
	cfg    Config
}

func New(store *state.Store, sender router.Sender, cfg Config) *Worker {
	return &Worker{store: store, sender: sender, cfg: cfg}
}

// Run scans every cfg.ScanInterval until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Tick()
		}
	}
}

// Tick performs a single scan + nudge pass. Exported so tests and ad-hoc
// triggers can drive a scan without spinning up Run.
func (w *Worker) Tick() {
	for userID, d := range w.store.AllAwaiting() {
		if d.ReminderSent {
			continue
		}
		if time.Since(d.EnteredAt) < w.cfg.ReminderAfter {
			continue
		}
		msg := tgbotapi.NewMessage(d.ChatID, "Still there? Please answer 1, 2, or 3.")
		if _, err := w.sender.Send(msg); err != nil {
			log.Printf("reminder: send to chat=%d: %v", d.ChatID, err)
			continue
		}
		d.ReminderSent = true
		w.store.Set(userID, d)
	}
}
