// Package reminder owns the scan loop that prompts users at the right
// times. The Worker doesn't know the message contents — it just calls a
// scan callback at the configured interval. The callback (typically
// journey.Runner.Remind) decides which users to nudge and what to say.
package reminder

import (
	"context"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/state"
)

// ScanFunc is a single scan pass — typically journey.Runner.Remind.
type ScanFunc func()

// IntervalFunc returns the current scan interval. Called every tick so
// runtime changes to settings.Settings.ScanInterval take effect on the
// next iteration without restarting the worker.
type IntervalFunc func() time.Duration

// Worker drives a ScanFunc on a ticker until its context is cancelled.
type Worker struct {
	scan         ScanFunc
	intervalFunc IntervalFunc

	// Legacy fields used by the deprecated New constructor.
	legacyStore  *state.Store
	legacySender router.Sender
	legacyCfg    Config
}

// Config is the legacy reminder configuration. Deprecated: prefer
// NewWithCallback + settings.Store.
type Config struct {
	ScanInterval  time.Duration
	ReminderAfter time.Duration
}

// NewWithCallback returns a Worker that calls scan once every
// intervalFunc(). Both must be non-nil; intervalFunc must return a
// positive duration on every call.
func NewWithCallback(scan ScanFunc, intervalFunc IntervalFunc) *Worker {
	return &Worker{scan: scan, intervalFunc: intervalFunc}
}

// New builds a Worker around the legacy survey-flow nudge logic.
//
// Deprecated: legacy survey-only entry point. Use NewWithCallback once
// the journey rollout completes.
func New(store *state.Store, sender router.Sender, cfg Config) *Worker {
	return &Worker{legacyStore: store, legacySender: sender, legacyCfg: cfg}
}

// Run scans on a ticker until ctx is cancelled. The interval is sampled
// every tick (via intervalFunc) so settings updates take effect immediately.
func (w *Worker) Run(ctx context.Context) {
	for {
		interval := w.currentInterval()
		ticker := time.NewTicker(interval)
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			ticker.Stop()
			w.Tick()
		}
	}
}

// Tick performs one scan pass.
func (w *Worker) Tick() {
	if w.scan != nil {
		w.scan()
		return
	}
	w.legacyTick()
}

func (w *Worker) currentInterval() time.Duration {
	if w.intervalFunc != nil {
		if d := w.intervalFunc(); d > 0 {
			return d
		}
	}
	if w.legacyCfg.ScanInterval > 0 {
		return w.legacyCfg.ScanInterval
	}
	return 30 * time.Second
}

func (w *Worker) legacyTick() {
	if w.legacyStore == nil || w.legacySender == nil {
		return
	}
	for userID, d := range w.legacyStore.AllAwaiting() {
		if d.ReminderSent {
			continue
		}
		if time.Since(d.EnteredAt) < w.legacyCfg.ReminderAfter {
			continue
		}
		msg := tgbotapi.NewMessage(d.ChatID, "Still there? Please answer 1, 2, or 3.")
		if _, err := w.legacySender.Send(msg); err != nil {
			log.Printf("reminder: send to chat=%d: %v", d.ChatID, err)
			continue
		}
		d.ReminderSent = true
		w.legacyStore.Set(userID, d)
	}
}
