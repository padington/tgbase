# internal/products

Runtime-mutable FODMAP catalog. Backend key = `"products"`.

## Responsibility

- Own the `Product` and `Category` Go structs.
- Load + persist a `{categories, products}` wrapper. Accepts legacy `[]Product` arrays from both backend and seed YAML so deployed VPSes keep booting.
- Provide lookup helpers the journey picker needs (`ByCategory`, `AvailableCategories`, `CategoryByID`, `Find`).
- Render localized labels (`Product.DisplayName`, `Category.DisplayName`, stage/amount templates).

## Public API

```go
type FodmapLevel, Measure, Stage string  // closed Go enums (these are NOT data-driven)

type Category struct { ID, Emoji string; NameLocalized map[string]string }
func (c Category) DisplayName(locale i18n.Locale) string

type Product struct {
    Name string
    Fodmap FodmapLevel; Measure Measure
    Stages map[Stage]float64
    Note string
    NameLocalized, NoteLocalized map[string]string
    Category string  // category id; unknown → bucketed under UncategorizedID
    Emoji string     // optional; falls back to category emoji at render time
}
func (p Product) DisplayName(locale) string
func (p Product) DisplayNote(locale) string
func (p Product) StageDescription(s Stage, locale, t i18n.Translator) string
func (p Product) AmountLabel(s Stage, locale, t i18n.Translator) string

const UncategorizedID = "uncategorized"

func NextStage(s Stage) (next Stage, completed bool)

type Catalog struct{ ... }
func New(backend store.Backend, seedPath string) (*Catalog, error)
func (c *Catalog) All() []Product
func (c *Catalog) Find(name string) (Product, bool)
func (c *Catalog) Categories() []Category
func (c *Catalog) CategoryByID(id string) (Category, bool)
func (c *Catalog) ByCategory(id string, exclude map[string]bool, locale i18n.Locale) []Product   // sorted by localized name
func (c *Catalog) AvailableCategories(exclude map[string]bool) []string                          // declaration order, includes "uncategorized" if it has stragglers
func (c *Catalog) Sample(exclude map[string]bool, n int) []Product                               // legacy random sampler
func (c *Catalog) Add(p Product) error
func (c *Catalog) Remove(name string) error
```

## Persisted shape

```json
{ "categories": [{"id":"fruits","emoji":"🍎","name_localized":{"en":"Fruits"}}, ...],
  "products":   [{"name":"Apple","category":"fruits","emoji":"🍎",...}, ...] }
```

Legacy `[Product, ...]` (no wrapper) is also accepted — `parseStored` peeks at the first non-whitespace byte to discriminate.

## Invariants

- All public methods are safe for concurrent use (single mutex).
- `ByCategory` sorts by localized display name (deterministic, paging-friendly).
- `AvailableCategories` returns categories in declaration order; appends `UncategorizedID` last when uncategorized products remain.
- `Add` overwrites by `Name`; `Remove` is a no-op for unknown names.

## When to edit

- **Add a new Product field** → struct here + `proto/products.proto` + regen + bump tests. Seed YAML stays backwards compatible if the field is `omitempty`.
- **Add a Category field** → same; also `internal/journey/category.go` (`categoryLabel`) if it affects rendering.
- **Change persistence format** → `parseStored` + `parseSeedYAML` + tests. Keep legacy fallback.
- **Add a new picker query** → method here, not in the journey package.

## Dependencies

`internal/i18n` (locale + Translator for label rendering), `internal/store`.
