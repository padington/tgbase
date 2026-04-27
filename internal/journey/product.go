package journey

import (
	"strings"

	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/state"
)

const (
	productGridCols = 3
	productGridRows = 4
	productPageSize = productGridCols * productGridRows
)

// ProductChoicePhase owns StateAwaitingProductChoice. Setup pages through
// the user's selected category (PickerCategory) in deterministic
// alphabetical order, showing up to productPageSize products plus a
// Back / Prev / Next nav row. Collect routes nav-row taps in-place and
// resolves product taps to a stage trial. No reminder.
type ProductChoicePhase struct{}

func NewProductChoicePhase() *ProductChoicePhase { return &ProductChoicePhase{} }

func (ProductChoicePhase) State() state.StateKind { return state.StateAwaitingProductChoice }

func (ProductChoicePhase) Setup(ctx Context) Outcome {
	if ctx.User.PickerCategory == "" {
		return Outcome{NextState: state.StateAwaitingProductCategory}
	}

	exclude := finishedProducts(ctx.User.Products)
	available := ctx.Catalog.ByCategory(ctx.User.PickerCategory, exclude, ctx.Locale)
	if len(available) == 0 {
		return Outcome{
			NextState: state.StateAwaitingProductCategory,
			Mutate: func(u *state.UserData) {
				u.PickerCategory = ""
				u.PickerPage = 0
				u.OfferedProducts = nil
			},
		}
	}

	pageCount := (len(available) + productPageSize - 1) / productPageSize
	page := ctx.User.PickerPage
	if page < 0 || page >= pageCount {
		page = 0
	}
	start := page * productPageSize
	end := start + productPageSize
	if end > len(available) {
		end = len(available)
	}
	slice := available[start:end]

	names := make([]string, 0, len(slice))
	keyboard := make([][]string, 0, productGridRows+1)
	var row []string
	for i, prod := range slice {
		names = append(names, prod.Name)
		row = append(row, productLabel(ctx, prod))
		if (i+1)%productGridCols == 0 || i == len(slice)-1 {
			keyboard = append(keyboard, row)
			row = nil
		}
	}
	keyboard = append(keyboard, navRow(ctx, page, pageCount))

	clampedPage := page
	return Outcome{
		ReplyKey: "phase.product.prompt",
		Keyboard: keyboard,
		Mutate: func(u *state.UserData) {
			u.OfferedProducts = names
			u.PickerPage = clampedPage
		},
	}
}

func (ProductChoicePhase) Collect(ctx Context, input string) Outcome {
	trimmed := strings.TrimSpace(input)
	switch trimmed {
	case ctx.Trans.T("button.product.back", ctx.Locale, nil):
		return Outcome{
			NextState: state.StateAwaitingProductCategory,
			Mutate: func(u *state.UserData) {
				u.PickerCategory = ""
				u.PickerPage = 0
				u.OfferedProducts = nil
			},
		}
	case ctx.Trans.T("button.product.prev", ctx.Locale, nil):
		return Outcome{
			NextState: state.StateAwaitingProductChoice,
			Mutate: func(u *state.UserData) {
				if u.PickerPage > 0 {
					u.PickerPage--
				}
			},
		}
	case ctx.Trans.T("button.product.next", ctx.Locale, nil):
		return Outcome{
			NextState: state.StateAwaitingProductChoice,
			Mutate: func(u *state.UserData) {
				u.PickerPage++
			},
		}
	}

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

// productLabel renders "<fodmap-indicator> <localized name>".
// 🔴 = high FODMAP, 🟠 = moderate FODMAP, plain name for low/unspecified.
func productLabel(ctx Context, p products.Product) string {
	name := p.DisplayName(ctx.Locale)
	switch p.Fodmap {
	case products.FodmapHigh:
		return "🔴 " + name
	case products.FodmapModerate:
		return "🟠 " + name
	case products.FodmapLow:
		return "🟢 " + name
	default:
		return name
	}
}

// navRow builds the Back / Prev / Next row beneath the product grid.
// Prev hides on page 0; Next hides on the last page.
func navRow(ctx Context, page, pageCount int) []string {
	row := []string{ctx.Trans.T("button.product.back", ctx.Locale, nil)}
	if page > 0 {
		row = append(row, ctx.Trans.T("button.product.prev", ctx.Locale, nil))
	}
	if page+1 < pageCount {
		row = append(row, ctx.Trans.T("button.product.next", ctx.Locale, nil))
	}
	return row
}

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
		if input == prod.Name || input == prod.DisplayName(ctx.Locale) || input == productLabel(ctx, prod) {
			return prod, true
		}
	}
	return products.Product{}, false
}
