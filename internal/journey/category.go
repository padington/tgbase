package journey

import (
	"strings"

	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/state"
)

const categoryGridCols = 2

// ProductCategoryPhase owns StateAwaitingProductCategory. Setup renders
// a grid of category buttons (or auto-skips when only one bucket has
// remaining products); Collect routes a tap to the chosen category and
// transitions to the product picker. Empty / single-category states
// short-circuit so the user never sees a useless prompt.
type ProductCategoryPhase struct{}

func NewProductCategoryPhase() *ProductCategoryPhase { return &ProductCategoryPhase{} }

func (ProductCategoryPhase) State() state.StateKind { return state.StateAwaitingProductCategory }

func (ProductCategoryPhase) Setup(ctx Context) Outcome {
	exclude := finishedProducts(ctx.User.Products)
	available := ctx.Catalog.AvailableCategories(exclude)

	if len(available) == 0 {
		return Outcome{
			NextState: state.StateIdle,
			ReplyKey:  "phase.product.exhausted",
			Mutate: func(u *state.UserData) {
				u.PickerCategory = ""
				u.PickerPage = 0
				u.OfferedProducts = nil
			},
		}
	}

	if len(available) == 1 {
		only := available[0]
		return Outcome{
			NextState: state.StateAwaitingProductChoice,
			Mutate: func(u *state.UserData) {
				u.PickerCategory = only
				u.PickerPage = 0
			},
		}
	}

	return Outcome{
		ReplyKey: "phase.product.category.prompt",
		Keyboard: categoryKeyboard(ctx, available),
		Mutate: func(u *state.UserData) {
			u.PickerCategory = ""
			u.PickerPage = 0
			u.OfferedProducts = nil
		},
	}
}

func (ProductCategoryPhase) Collect(ctx Context, input string) Outcome {
	trimmed := strings.TrimSpace(input)
	exclude := finishedProducts(ctx.User.Products)
	available := ctx.Catalog.AvailableCategories(exclude)

	for _, id := range available {
		if trimmed == categoryLabel(ctx, id) {
			picked := id
			return Outcome{
				NextState: state.StateAwaitingProductChoice,
				Mutate: func(u *state.UserData) {
					u.PickerCategory = picked
					u.PickerPage = 0
				},
			}
		}
	}
	return Outcome{ReplyKey: "phase.product.category.invalid"}
}

func (ProductCategoryPhase) Remind(ctx Context) Outcome { return Outcome{} }

// categoryLabel renders "<emoji> <localized name>" for the given category
// id. Unknown ids (including the synthetic uncategorized bucket when
// products.yaml doesn't declare it) fall back to the raw id so the user
// can still pick the bucket.
func categoryLabel(ctx Context, id string) string {
	cat, ok := ctx.Catalog.CategoryByID(id)
	if !ok {
		if id == products.UncategorizedID {
			return id
		}
		return id
	}
	name := cat.DisplayName(ctx.Locale)
	if cat.Emoji != "" {
		return cat.Emoji + " " + name
	}
	return name
}

func categoryKeyboard(ctx Context, ids []string) [][]string {
	rows := make([][]string, 0, (len(ids)+categoryGridCols-1)/categoryGridCols)
	for i := 0; i < len(ids); i += categoryGridCols {
		end := i + categoryGridCols
		if end > len(ids) {
			end = len(ids)
		}
		row := make([]string, 0, end-i)
		for _, id := range ids[i:end] {
			row = append(row, categoryLabel(ctx, id))
		}
		rows = append(rows, row)
	}
	return rows
}
