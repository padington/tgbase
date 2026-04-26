package products_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/store"
)

const seedYAML = `categories:
  - id: fruits
    emoji: "🍎"
    name_localized: { en: Fruits, ru: Фрукты }
  - id: legumes
    emoji: "🥜"
    name_localized: { en: Legumes, ru: Бобовые }
  - id: sweets
    emoji: "🍯"
    name_localized: { en: Sweets, ru: Сладкое }

products:
  - name: Apple
    category: fruits
    emoji: "🍎"
    fodmap: high
    measure: pieces
    stages: { low: 0.25, medium: 0.5, high: 1.0 }
    name_localized:
      ru: "Яблоко"
  - name: Cashews
    category: legumes
    fodmap: high
    measure: grams
    stages: { low: 10, medium: 20, high: 30 }
  - name: Honey
    category: sweets
    emoji: "🍯"
    fodmap: high
    measure: spoons
    stages: { low: 0.5, medium: 1, high: 2 }
`

func writeSeed(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "products.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeI18n(t *testing.T, dir string, locale, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, locale+".yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNew_SeedsFromYAMLWhenBackendEmpty(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	backend := store.NewMemoryBackend()

	cat, err := products.New(backend, seed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(cat.All()) != 3 {
		t.Errorf("expected 3 seeded products, got %d", len(cat.All()))
	}
	if len(cat.Categories()) != 3 {
		t.Errorf("expected 3 seeded categories, got %d", len(cat.Categories()))
	}

	raw, _ := backend.Get("products")
	if raw == nil {
		t.Error("seed should have been written back to backend")
	}
}

func TestNew_PrefersBackendOverYAML(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	backend := store.NewMemoryBackend()
	_ = backend.Put("products", []byte(`{"products":[{"name":"OnlyOne","fodmap":"low","measure":"pieces","stages":{"low":1,"medium":2,"high":3}}]}`))

	cat, err := products.New(backend, seed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	all := cat.All()
	if len(all) != 1 || all[0].Name != "OnlyOne" {
		t.Errorf("backend value should win over yaml seed, got %+v", all)
	}
}

func TestNew_AcceptsLegacyFlatArray(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	backend := store.NewMemoryBackend()
	// Pre-existing deployments wrote a flat []Product array; ensure we
	// can still load that without forcing a migration step.
	_ = backend.Put("products", []byte(`[{"name":"LegacyApple","fodmap":"high","measure":"pieces","stages":{"low":0.25}}]`))

	cat, err := products.New(backend, seed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := cat.All(); len(got) != 1 || got[0].Name != "LegacyApple" {
		t.Errorf("legacy array not accepted, got %+v", got)
	}
}

func TestFind_KnownAndUnknown(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	if _, ok := cat.Find("Apple"); !ok {
		t.Error("expected to find Apple")
	}
	if _, ok := cat.Find("nonsense"); ok {
		t.Error("did not expect to find nonsense")
	}
}

func TestSample_RespectsExclude(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	got := cat.Sample(map[string]bool{"Apple": true, "Cashews": true}, 10)
	if len(got) != 1 {
		t.Fatalf("expected 1 (only Honey left), got %d", len(got))
	}
	if got[0].Name != "Honey" {
		t.Errorf("expected Honey, got %s", got[0].Name)
	}
}

func TestSample_CapsAtAvailable(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	got := cat.Sample(nil, 100)
	if len(got) != 3 {
		t.Errorf("expected at most 3 (catalog size), got %d", len(got))
	}
}

func TestCategoryByID_KnownAndUnknown(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	got, ok := cat.CategoryByID("fruits")
	if !ok || got.Emoji != "🍎" {
		t.Errorf("fruits lookup: got %+v ok=%v", got, ok)
	}
	if _, ok := cat.CategoryByID("nope"); ok {
		t.Error("unknown id should return ok=false")
	}
}

func TestByCategory_FiltersAndSorts(t *testing.T) {
	seed := writeSeed(t, seedYAML+`  - name: Pear
    category: fruits
    fodmap: low
    measure: pieces
    stages: { low: 0.25, medium: 0.5, high: 1.0 }
    name_localized:
      ru: "Груша"
`)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	got := cat.ByCategory("fruits", nil, "en")
	if len(got) != 2 {
		t.Fatalf("expected 2 fruits, got %d", len(got))
	}
	if got[0].Name != "Apple" || got[1].Name != "Pear" {
		t.Errorf("expected alphabetical Apple,Pear; got %s,%s", got[0].Name, got[1].Name)
	}

	got = cat.ByCategory("fruits", map[string]bool{"Apple": true}, "en")
	if len(got) != 1 || got[0].Name != "Pear" {
		t.Errorf("exclude not honored: %+v", got)
	}

	// Russian locale sorts by Russian display name (Груша < Яблоко
	// alphabetically in Cyrillic, so Pear comes first).
	got = cat.ByCategory("fruits", nil, "ru")
	if got[0].Name != "Pear" {
		t.Errorf("ru sort: expected Pear first (Груша), got %s", got[0].Name)
	}
}

func TestByCategory_UncategorizedBucket(t *testing.T) {
	seed := writeSeed(t, seedYAML+`  - name: Mystery
    fodmap: high
    measure: pieces
    stages: { low: 1, medium: 2, high: 3 }
`)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	got := cat.ByCategory(products.UncategorizedID, nil, "en")
	if len(got) != 1 || got[0].Name != "Mystery" {
		t.Errorf("uncategorized bucket: got %+v", got)
	}
}

func TestAvailableCategories_OmitsEmpty(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	got := cat.AvailableCategories(nil)
	wantIn := map[string]bool{"fruits": true, "legumes": true, "sweets": true}
	if len(got) != len(wantIn) {
		t.Fatalf("expected %d, got %d (%v)", len(wantIn), len(got), got)
	}
	for _, id := range got {
		if !wantIn[id] {
			t.Errorf("unexpected id %q", id)
		}
	}

	// Exclude every fruit → fruits should drop out.
	got = cat.AvailableCategories(map[string]bool{"Apple": true})
	for _, id := range got {
		if id == "fruits" {
			t.Error("fruits should be empty after excluding Apple")
		}
	}
}

func TestAvailableCategories_AppendsUncategorized(t *testing.T) {
	seed := writeSeed(t, seedYAML+`  - name: Mystery
    fodmap: high
    measure: pieces
    stages: { low: 1, medium: 2, high: 3 }
`)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	got := cat.AvailableCategories(nil)
	if got[len(got)-1] != products.UncategorizedID {
		t.Errorf("expected uncategorized appended last, got %v", got)
	}
}

func TestAdd_NewProductPersists(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	backend := store.NewMemoryBackend()
	cat, _ := products.New(backend, seed)

	newP := products.Product{Name: "Pear", Fodmap: products.FodmapLow, Measure: products.MeasurePieces, Stages: map[products.Stage]float64{products.StageLow: 0.5, products.StageMedium: 1, products.StageHigh: 2}}
	if err := cat.Add(newP); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, ok := cat.Find("Pear"); !ok {
		t.Error("Pear missing from cache after Add")
	}

	cat2, _ := products.New(backend, seed)
	if _, ok := cat2.Find("Pear"); !ok {
		t.Error("Pear did not persist to backend")
	}
	if len(cat2.Categories()) != 3 {
		t.Errorf("categories should round-trip through persist, got %d", len(cat2.Categories()))
	}
}

func TestAdd_OverwritesByName(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	updated := products.Product{Name: "Apple", Fodmap: products.FodmapLow, Measure: products.MeasurePieces, Stages: map[products.Stage]float64{}}
	_ = cat.Add(updated)
	got, _ := cat.Find("Apple")
	if got.Fodmap != products.FodmapLow {
		t.Errorf("expected overwrite to set Fodmap=low, got %q", got.Fodmap)
	}
	if len(cat.All()) != 3 {
		t.Errorf("expected catalog size unchanged after overwrite, got %d", len(cat.All()))
	}
}

func TestRemove_DeletesAndPersists(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	backend := store.NewMemoryBackend()
	cat, _ := products.New(backend, seed)

	if err := cat.Remove("Apple"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cat.Find("Apple"); ok {
		t.Error("Apple still present after Remove")
	}

	cat2, _ := products.New(backend, seed)
	if _, ok := cat2.Find("Apple"); ok {
		t.Error("Apple resurrected after reload — not persisted")
	}
}

func TestRemove_UnknownIsNoop(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	cat, _ := products.New(store.NewMemoryBackend(), seed)

	before := len(cat.All())
	if err := cat.Remove("nope"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := len(cat.All()); got != before {
		t.Errorf("size changed after no-op remove: %d → %d", before, got)
	}
}

func TestNextStage(t *testing.T) {
	cases := []struct {
		in           products.Stage
		wantNext     products.Stage
		wantComplete bool
	}{
		{products.StageUnspecified, products.StageLow, false},
		{products.StageLow, products.StageMedium, false},
		{products.StageMedium, products.StageHigh, false},
		{products.StageHigh, "", true},
	}
	for _, tc := range cases {
		next, complete := products.NextStage(tc.in)
		if next != tc.wantNext || complete != tc.wantComplete {
			t.Errorf("NextStage(%q) = (%q, %v), want (%q, %v)", tc.in, next, complete, tc.wantNext, tc.wantComplete)
		}
	}
}

func TestProduct_DisplayNameFallback(t *testing.T) {
	p := products.Product{Name: "Apple", NameLocalized: map[string]string{"ru": "Яблоко"}}
	if got := p.DisplayName("ru"); got != "Яблоко" {
		t.Errorf("ru DisplayName: got %q", got)
	}
	if got := p.DisplayName("fr"); got != "Apple" {
		t.Errorf("fallback DisplayName: got %q", got)
	}
}

func TestCategory_DisplayNameFallback(t *testing.T) {
	c := products.Category{ID: "fruits", NameLocalized: map[string]string{"ru": "Фрукты"}}
	if got := c.DisplayName("ru"); got != "Фрукты" {
		t.Errorf("ru: got %q", got)
	}
	if got := c.DisplayName("fr"); got != "fruits" {
		t.Errorf("fallback to id: got %q", got)
	}
}

func TestProduct_StageDescription(t *testing.T) {
	dir := t.TempDir()
	writeI18n(t, dir, "en", "product.measure_template.pieces: \"{value} {name}\"\n")
	writeI18n(t, dir, "ru", "product.measure_template.pieces: \"{value} {name}\"\n")
	tr, err := i18n.Load(dir, "en")
	if err != nil {
		t.Fatal(err)
	}

	p := products.Product{
		Name:          "Apple",
		Measure:       products.MeasurePieces,
		Stages:        map[products.Stage]float64{products.StageLow: 0.25},
		NameLocalized: map[string]string{"ru": "Яблоко"},
	}
	if got := p.StageDescription(products.StageLow, "en", tr); got != "0.25 Apple" {
		t.Errorf("en: got %q", got)
	}
	if got := p.StageDescription(products.StageLow, "ru", tr); got != "0.25 Яблоко" {
		t.Errorf("ru: got %q", got)
	}
}

func TestProduct_AmountLabel(t *testing.T) {
	dir := t.TempDir()
	writeI18n(t, dir, "en", "product.amount_template.grams: \"{value}g\"\nproduct.amount_template.pieces: \"{value}\"\n")
	writeI18n(t, dir, "ru", "product.amount_template.grams: \"{value}г\"\nproduct.amount_template.pieces: \"{value}\"\n")
	tr, err := i18n.Load(dir, "en")
	if err != nil {
		t.Fatal(err)
	}

	cashews := products.Product{
		Name:    "Cashews",
		Measure: products.MeasureGrams,
		Stages:  map[products.Stage]float64{products.StageLow: 10, products.StageMedium: 20, products.StageHigh: 30},
	}
	if got := cashews.AmountLabel(products.StageLow, "en", tr); got != "10g" {
		t.Errorf("en low: got %q", got)
	}
	if got := cashews.AmountLabel(products.StageHigh, "ru", tr); got != "30г" {
		t.Errorf("ru high: got %q", got)
	}

	apple := products.Product{
		Name:    "Apple",
		Measure: products.MeasurePieces,
		Stages:  map[products.Stage]float64{products.StageMedium: 0.5},
	}
	if got := apple.AmountLabel(products.StageMedium, "en", tr); got != "0.5" {
		t.Errorf("pieces: got %q", got)
	}
}
