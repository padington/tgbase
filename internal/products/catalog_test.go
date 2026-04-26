package products_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/products"
	"github.com/padington/tgbase/internal/store"
)

const seedYAML = `- name: Apple
  fodmap: high
  measure: pieces
  stages: { low: 0.25, medium: 0.5, high: 1.0 }
  name_localized:
    ru: "Яблоко"
- name: Cashews
  fodmap: high
  measure: grams
  stages: { low: 10, medium: 20, high: 30 }
- name: Honey
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

	raw, _ := backend.Get("products")
	if raw == nil {
		t.Error("seed should have been written back to backend")
	}
}

func TestNew_PrefersBackendOverYAML(t *testing.T) {
	seed := writeSeed(t, seedYAML)
	backend := store.NewMemoryBackend()
	_ = backend.Put("products", []byte(`[{"name":"OnlyOne","fodmap":"low","measure":"pieces","stages":{"low":1,"medium":2,"high":3}}]`))

	cat, err := products.New(backend, seed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	all := cat.All()
	if len(all) != 1 || all[0].Name != "OnlyOne" {
		t.Errorf("backend value should win over yaml seed, got %+v", all)
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

func TestProduct_StageDescription(t *testing.T) {
	dir := t.TempDir()
	writeI18n(t, dir, "en", "product.measure_template.pieces: \"{value} {name}\"\n")
	writeI18n(t, dir, "ru", "product.measure_template.pieces: \"{value} {name}\"\n")
	tr, err := i18n.Load(dir, "en")
	if err != nil {
		t.Fatal(err)
	}

	p := products.Product{
		Name:    "Apple",
		Measure: products.MeasurePieces,
		Stages:  map[products.Stage]float64{products.StageLow: 0.25},
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
