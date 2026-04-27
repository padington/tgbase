package journey

import (
	"strings"

	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/state"
)

// StageChoicePhase owns StateAwaitingStageChoice. Setup tells the user
// the recommended starting dose and offers all three stage volumes as
// short amount-only buttons; Collect resolves the tap to a Stage and
// transitions to the regular check-in phase. No reminder.
type StageChoicePhase struct{}

func NewStageChoicePhase() *StageChoicePhase { return &StageChoicePhase{} }

func (StageChoicePhase) State() state.StateKind { return state.StateAwaitingStageChoice }

var stageOrder = []products.Stage{products.StageLow, products.StageMedium, products.StageHigh}

func (StageChoicePhase) Setup(ctx Context) Outcome {
	prod, ok := ctx.Catalog.Find(ctx.User.CurrentProduct)
	if !ok {
		return Outcome{NextState: state.StateAwaitingProductChoice}
	}
	buttons := make([]string, 0, len(stageOrder))
	for _, s := range stageOrder {
		buttons = append(buttons, prod.AmountLabel(s, ctx.Locale, ctx.Trans))
	}
	back := ctx.Trans.T("button.product.back", ctx.Locale, nil)
	return Outcome{
		ReplyKey: "phase.stage_choice.prompt",
		ReplyArgs: map[string]any{
			"name":        prod.DisplayName(ctx.Locale),
			"recommended": prod.StageDescription(products.StageLow, ctx.Locale, ctx.Trans),
		},
		Keyboard: [][]string{buttons, {back}},
	}
}

func (StageChoicePhase) Collect(ctx Context, input string) Outcome {
	prod, ok := ctx.Catalog.Find(ctx.User.CurrentProduct)
	if !ok {
		return Outcome{NextState: state.StateAwaitingProductChoice}
	}
	normalized := strings.TrimSpace(input)

	if normalized == ctx.Trans.T("button.product.back", ctx.Locale, nil) {
		return Outcome{
			NextState: state.StateAwaitingProductChoice,
			Mutate: func(u *state.UserData) {
				u.CurrentProduct = ""
				u.CurrentStage = ""
			},
		}
	}
	picked := products.Stage("")
	for _, s := range stageOrder {
		if normalized == prod.AmountLabel(s, ctx.Locale, ctx.Trans) {
			picked = s
			break
		}
	}
	if picked == "" {
		return Outcome{ReplyKey: "phase.stage_choice.invalid"}
	}
	now := ctx.Now()
	return Outcome{
		NextState: state.StateAwaitingStageCheckin,
		Mutate: func(u *state.UserData) {
			u.CurrentStage = picked
			u.StageStartedAt = now
			u.CheckinAsked = false
			if u.Products == nil {
				u.Products = make(map[string]state.ProductProgress)
			}
			prog := u.Products[prod.Name]
			prog.LastStage = picked
			prog.Status = "in_progress"
			prog.UpdatedAt = now
			u.Products[prod.Name] = prog
		},
	}
}

func (StageChoicePhase) Remind(ctx Context) Outcome { return Outcome{} }
