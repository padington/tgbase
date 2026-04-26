package journey

import (
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/state"
)

// ProductChoicePhase owns StateAwaitingProductChoice. Setup samples up to
// 10 products the user hasn't completed or rejected and offers them as
// keyboard buttons; Collect resolves the user's tap back to a canonical
// product name and starts a stage trial. No reminder.
type ProductChoicePhase struct {
	sampleSize int
}

func NewProductChoicePhase() *ProductChoicePhase {
	return &ProductChoicePhase{sampleSize: 10}
}

func (ProductChoicePhase) State() state.StateKind { return state.StateAwaitingProductChoice }

func (p ProductChoicePhase) Setup(ctx Context) Outcome {
	exclude := finishedProducts(ctx.User.Products)
	sample := ctx.Catalog.Sample(exclude, p.sampleSize)

	if len(sample) == 0 {
		return Outcome{
			NextState: state.StateIdle,
			ReplyKey:  "phase.product.exhausted",
			Mutate: func(u *state.UserData) {
				u.OfferedProducts = nil
			},
		}
	}

	names := make([]string, 0, len(sample))
	labels := make([]string, 0, len(sample))
	for _, prod := range sample {
		names = append(names, prod.Name)
		labels = append(labels, prod.DisplayName(ctx.Locale))
	}

	return Outcome{
		ReplyKey: "phase.product.prompt",
		Buttons:  labels,
		Mutate: func(u *state.UserData) {
			u.OfferedProducts = names
		},
	}
}

func (p ProductChoicePhase) Collect(ctx Context, input string) Outcome {
	picked, ok := matchOffered(ctx, input)
	if !ok {
		return Outcome{ReplyKey: "phase.product.invalid"}
	}
	return Outcome{
		NextState: state.StateAwaitingStageChoice,
		Mutate: func(u *state.UserData) {
			u.CurrentProduct = picked.Name
			u.CurrentStage = ""
			u.CheckinAsked = false
		},
	}
}

func (ProductChoicePhase) Remind(ctx Context) Outcome { return Outcome{} }

func finishedProducts(progress map[string]state.ProductProgress) map[string]bool {
	out := make(map[string]bool)
	for name, prog := range progress {
		if prog.Status == "completed" || prog.Status == "not_tolerated" {
			out[name] = true
		}
	}
	return out
}

func matchOffered(ctx Context, input string) (products.Product, bool) {
	for _, name := range ctx.User.OfferedProducts {
		prod, ok := ctx.Catalog.Find(name)
		if !ok {
			continue
		}
		if input == prod.Name || input == prod.DisplayName(ctx.Locale) {
			return prod, true
		}
	}
	return products.Product{}, false
}
