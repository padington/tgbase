// Package journey is the multi-phase interaction framework that drives the
// low-FODMAP diet conversation. Each user state is owned by a Phase that
// describes how to enter (Setup), how to handle an input (Collect), and
// how to nudge an idle user (Remind). The Runner dispatches text input
// and reminder ticks to the right phase by user state.
package journey

import (
	"time"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/settings"
	"github.com/padington/tgbase/internal/state"
)

// Outcome describes how the runner should proceed after a phase callback.
// An empty Outcome (zero value) is valid: stay put, no message, no mutation.
type Outcome struct {
	NextState state.StateKind         // empty = stay in current state
	ReplyKey  string                  // i18n key for outgoing message; empty = silent
	ReplyArgs map[string]any          // placeholder substitutions for ReplyKey
	Buttons   []string                // already-resolved keyboard labels (one row)
	Mutate    func(*state.UserData)   // optional mutation applied before persisting
}

// Context is the per-call dependency bag handed to Phase callbacks.
// User is a copy — mutations should be expressed via Outcome.Mutate.
type Context struct {
	UserID   int64
	User     state.UserData
	Catalog  *products.Catalog
	Settings settings.Settings
	Trans    i18n.Translator
	Locale   i18n.Locale
	Now      func() time.Time
}

// Phase represents one user-state's behaviour.
type Phase interface {
	State() state.StateKind
	Setup(ctx Context) Outcome
	Collect(ctx Context, input string) Outcome
	Remind(ctx Context) Outcome
}
