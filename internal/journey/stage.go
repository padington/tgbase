package journey

import (
	"strings"

	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/state"
)

// StageCheckinPhase owns StateAwaitingStageCheckin. Setup tells the user
// what volume to take and when to expect a check-in; Remind sends the
// "are you doing OK?" prompt; Collect handles yes/no and either advances
// to the next stage / completes the product / marks not_tolerated.
type StageCheckinPhase struct{}

func NewStageCheckinPhase() *StageCheckinPhase { return &StageCheckinPhase{} }

func (StageCheckinPhase) State() state.StateKind { return state.StateAwaitingStageCheckin }

func (StageCheckinPhase) Setup(ctx Context) Outcome {
	prod, ok := ctx.Catalog.Find(ctx.User.CurrentProduct)
	if !ok {
		return Outcome{NextState: state.StateAwaitingProductChoice}
	}
	desc := prod.StageDescription(ctx.User.CurrentStage, ctx.Locale, ctx.Trans)
	checkin := ctx.Settings.CheckinInterval.String()
	now := ctx.Now()
	return Outcome{
		ReplyKey: "phase.stage.prompt",
		ReplyArgs: map[string]any{
			"description": desc,
			"checkin":     checkin,
		},
		Mutate: func(u *state.UserData) {
			u.StageStartedAt = now
			u.CheckinAsked = false
		},
	}
}

func (StageCheckinPhase) Collect(ctx Context, input string) Outcome {
	yes := ctx.Trans.T("button.yes", ctx.Locale, nil)
	no := ctx.Trans.T("button.no", ctx.Locale, nil)
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case strings.ToLower(yes), "yes":
		return advanceOnYes(ctx)
	case strings.ToLower(no), "no":
		return rejectOnNo(ctx)
	default:
		return Outcome{ReplyKey: "phase.stage.checkin_invalid"}
	}
}

func (StageCheckinPhase) Remind(ctx Context) Outcome {
	if ctx.User.CheckinAsked {
		return Outcome{}
	}
	if ctx.Now().Sub(ctx.User.StageStartedAt) < ctx.Settings.CheckinInterval {
		return Outcome{}
	}
	prod, ok := ctx.Catalog.Find(ctx.User.CurrentProduct)
	if !ok {
		return Outcome{}
	}
	desc := prod.StageDescription(ctx.User.CurrentStage, ctx.Locale, ctx.Trans)
	return Outcome{
		ReplyKey: "phase.stage.checkin",
		ReplyArgs: map[string]any{
			"description": desc,
		},
		Mutate: func(u *state.UserData) {
			u.CheckinAsked = true
		},
	}
}

func advanceOnYes(ctx Context) Outcome {
	prod, ok := ctx.Catalog.Find(ctx.User.CurrentProduct)
	if !ok {
		return Outcome{NextState: state.StateAwaitingProductChoice}
	}
	next, complete := products.NextStage(ctx.User.CurrentStage)
	now := ctx.Now()

	if complete {
		return Outcome{
			NextState: state.StateAwaitingProductChoice,
			ReplyKey:  "phase.stage.completed",
			ReplyArgs: map[string]any{"name": prod.DisplayName(ctx.Locale)},
			Mutate: func(u *state.UserData) {
				if u.Products == nil {
					u.Products = make(map[string]state.ProductProgress)
				}
				prog := u.Products[prod.Name]
				prog.LastStage = products.StageHigh
				prog.Status = "completed"
				prog.UpdatedAt = now
				u.Products[prod.Name] = prog
				u.CurrentProduct = ""
				u.CurrentStage = ""
				u.OfferedProducts = nil
			},
		}
	}

	desc := prod.StageDescription(next, ctx.Locale, ctx.Trans)
	return Outcome{
		ReplyKey: "phase.stage.next",
		ReplyArgs: map[string]any{
			"description": desc,
		},
		Mutate: func(u *state.UserData) {
			u.CurrentStage = next
			u.StageStartedAt = now
			u.CheckinAsked = false
			if u.Products == nil {
				u.Products = make(map[string]state.ProductProgress)
			}
			prog := u.Products[prod.Name]
			prog.LastStage = next
			prog.Status = "in_progress"
			prog.UpdatedAt = now
			u.Products[prod.Name] = prog
		},
	}
}

func rejectOnNo(ctx Context) Outcome {
	prod, ok := ctx.Catalog.Find(ctx.User.CurrentProduct)
	name := ctx.User.CurrentProduct
	if ok {
		name = prod.DisplayName(ctx.Locale)
	}
	now := ctx.Now()
	return Outcome{
		NextState: state.StateAwaitingProductChoice,
		ReplyKey:  "phase.stage.not_tolerated",
		ReplyArgs: map[string]any{"name": name},
		Mutate: func(u *state.UserData) {
			if u.Products == nil {
				u.Products = make(map[string]state.ProductProgress)
			}
			prog := u.Products[u.CurrentProduct]
			prog.LastStage = u.CurrentStage
			prog.Status = "not_tolerated"
			prog.UpdatedAt = now
			u.Products[u.CurrentProduct] = prog
			u.CurrentProduct = ""
			u.CurrentStage = ""
			u.OfferedProducts = nil
		},
	}
}
