// Package products holds the FODMAP food catalog and stage progression.
// The Catalog is backed by store.Backend so the list is mutable at runtime;
// on first boot it seeds itself from a bundled YAML file.
package products

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/padington/tgbase/internal/i18n"
	"github.com/padington/tgbase/internal/store"
	"gopkg.in/yaml.v3"
)

type FodmapLevel string

const (
	FodmapUnspecified FodmapLevel = ""
	FodmapLow         FodmapLevel = "low"
	FodmapHigh        FodmapLevel = "high"
	FodmapModerate    FodmapLevel = "moderate"
)

type Measure string

const (
	MeasureUnspecified Measure = ""
	MeasurePieces      Measure = "pieces"
	MeasureGrams       Measure = "grams"
	MeasureML          Measure = "ml"
	MeasureSpoons      Measure = "spoons"
)

type Stage string

const (
	StageUnspecified Stage = ""
	StageLow         Stage = "low"
	StageMedium      Stage = "medium"
	StageHigh        Stage = "high"
)

// NextStage returns the next stage after s and whether s was the final one.
// For an unset stage it returns (StageLow, false) so callers can use it as
// "start a new product at low volume".
func NextStage(s Stage) (Stage, bool) {
	switch s {
	case StageLow:
		return StageMedium, false
	case StageMedium:
		return StageHigh, false
	case StageHigh:
		return "", true
	default:
		return StageLow, false
	}
}

// Category is a browsable bucket declared in products.yaml. Products
// reference a category by ID. Unknown / missing IDs surface as a synthetic
// "uncategorized" group at the end of the picker.
type Category struct {
	ID            string            `yaml:"id" json:"id"`
	Emoji         string            `yaml:"emoji,omitempty" json:"emoji,omitempty"`
	NameLocalized map[string]string `yaml:"name_localized,omitempty" json:"name_localized,omitempty"`
}

// DisplayName returns the localized category name, falling back to ID.
func (c Category) DisplayName(locale i18n.Locale) string {
	if v, ok := c.NameLocalized[string(locale)]; ok && v != "" {
		return v
	}
	return c.ID
}

type Product struct {
	Name          string            `yaml:"name" json:"name"`
	Fodmap        FodmapLevel       `yaml:"fodmap" json:"fodmap"`
	Measure       Measure           `yaml:"measure" json:"measure"`
	Stages        map[Stage]float64 `yaml:"stages" json:"stages"`
	Note          string            `yaml:"note,omitempty" json:"note,omitempty"`
	NameLocalized map[string]string `yaml:"name_localized,omitempty" json:"name_localized,omitempty"`
	NoteLocalized map[string]string `yaml:"note_localized,omitempty" json:"note_localized,omitempty"`
	Category      string            `yaml:"category,omitempty" json:"category,omitempty"`
	Emoji         string            `yaml:"emoji,omitempty" json:"emoji,omitempty"`
}

// DisplayName returns the localized name for locale, falling back to Name.
func (p Product) DisplayName(locale i18n.Locale) string {
	if v, ok := p.NameLocalized[string(locale)]; ok && v != "" {
		return v
	}
	return p.Name
}

// DisplayNote returns the localized note for locale, falling back to Note.
func (p Product) DisplayNote(locale i18n.Locale) string {
	if v, ok := p.NoteLocalized[string(locale)]; ok && v != "" {
		return v
	}
	return p.Note
}

// StageDescription renders a human-readable phrase like "0.25 Apple" via the
// "product.measure_template.<measure>" i18n template.
func (p Product) StageDescription(s Stage, locale i18n.Locale, t i18n.Translator) string {
	return t.T("product.measure_template."+string(p.Measure), locale, map[string]any{
		"value": p.Stages[s],
		"name":  p.DisplayName(locale),
	})
}

// AmountLabel renders just the amount + unit (e.g. "10g", "0.25") via the
// "product.amount_template.<measure>" i18n template. Used for compact
// stage-pick buttons where the product name is already in the prompt.
func (p Product) AmountLabel(s Stage, locale i18n.Locale, t i18n.Translator) string {
	return t.T("product.amount_template."+string(p.Measure), locale, map[string]any{
		"value": p.Stages[s],
	})
}

// UncategorizedID is the synthetic category bucket for products whose
// declared Category does not match any known Category.ID. Surfaces only
// when the catalog actually has stragglers.
const UncategorizedID = "uncategorized"

// catalogData is the persisted JSON / seed YAML shape.
type catalogData struct {
	Categories []Category `yaml:"categories,omitempty" json:"categories,omitempty"`
	Products   []Product  `yaml:"products" json:"products"`
}

// Catalog wraps a store.Backend with typed access to the category +
// product lists. Methods are safe for concurrent use.
type Catalog struct {
	mu         sync.Mutex
	backend    store.Backend
	categories []Category
	products   []Product
	rng        *rand.Rand
}

const backendKey = "products"

// New loads the catalog from backend; if backend is empty for the products
// key, seeds from the YAML file at seedPath and writes the seed back so
// subsequent boots use the backend as source of truth.
func New(backend store.Backend, seedPath string) (*Catalog, error) {
	c := &Catalog{
		backend: backend,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	if err := c.load(seedPath); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) load(seedPath string) error {
	raw, err := c.backend.Get(backendKey)
	if err != nil {
		return fmt.Errorf("backend get %q: %w", backendKey, err)
	}
	if raw != nil {
		cats, prods, err := parseStored(raw)
		if err != nil {
			return fmt.Errorf("parse backend products: %w", err)
		}
		c.categories = cats
		c.products = prods
		return nil
	}
	if seedPath == "" {
		c.products = []Product{}
		return nil
	}
	data, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed %s: %w", seedPath, err)
	}
	cats, prods, err := parseSeedYAML(data)
	if err != nil {
		return fmt.Errorf("parse seed %s: %w", seedPath, err)
	}
	c.categories = cats
	c.products = prods
	return c.persistLocked()
}

// parseSeedYAML decodes either the new wrapper shape or a bare list of
// products. Test fixtures and pre-categories deployments use the bare
// list, so we keep accepting it.
func parseSeedYAML(data []byte) ([]Category, []Product, error) {
	var wrapped catalogData
	if err := yaml.Unmarshal(data, &wrapped); err == nil && (len(wrapped.Products) > 0 || len(wrapped.Categories) > 0) {
		return wrapped.Categories, wrapped.Products, nil
	}
	var legacy []Product
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil, nil, err
	}
	return nil, legacy, nil
}

// parseStored decodes either the new wrapper shape or the legacy flat
// []Product array (so already-deployed backends keep working).
func parseStored(raw []byte) ([]Category, []Product, error) {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var legacy []Product
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return nil, nil, err
		}
		return nil, legacy, nil
	}
	var loaded catalogData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, nil, err
	}
	return loaded.Categories, loaded.Products, nil
}

func (c *Catalog) persistLocked() error {
	raw, err := json.Marshal(catalogData{
		Categories: c.categories,
		Products:   c.products,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.backend.Put(backendKey, raw)
}

func (c *Catalog) All() []Product {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Product, len(c.products))
	copy(out, c.products)
	return out
}

func (c *Catalog) Find(name string) (Product, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.products {
		if p.Name == name {
			return p, true
		}
	}
	return Product{}, false
}

// Categories returns a copy of the declared categories in declaration
// order. Does not include the synthetic uncategorized bucket.
func (c *Catalog) Categories() []Category {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Category, len(c.categories))
	copy(out, c.categories)
	return out
}

// CategoryByID returns the declared category with id, or false. The
// synthetic uncategorized id is NOT returned here — callers should
// special-case it.
func (c *Catalog) CategoryByID(id string) (Category, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cat := range c.categories {
		if cat.ID == id {
			return cat, true
		}
	}
	return Category{}, false
}

// ByCategory returns products in the given category id (or uncategorized,
// for products whose Category doesn't match any declared category) that
// are not in exclude, sorted alphabetically by their localized display
// name. Deterministic and paging-friendly.
func (c *Catalog) ByCategory(id string, exclude map[string]bool, locale i18n.Locale) []Product {
	c.mu.Lock()
	known := make(map[string]bool, len(c.categories))
	for _, cat := range c.categories {
		known[cat.ID] = true
	}
	out := make([]Product, 0, len(c.products))
	for _, p := range c.products {
		if exclude[p.Name] {
			continue
		}
		productCat := p.Category
		if !known[productCat] {
			productCat = UncategorizedID
		}
		if productCat == id {
			out = append(out, p)
		}
	}
	c.mu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].DisplayName(locale) < out[j].DisplayName(locale)
	})
	return out
}

// AvailableCategories returns category IDs with at least one product not
// in exclude, in declaration order. The synthetic uncategorized id is
// appended last when there are uncategorized leftovers.
func (c *Catalog) AvailableCategories(exclude map[string]bool) []string {
	c.mu.Lock()
	known := make(map[string]bool, len(c.categories))
	for _, cat := range c.categories {
		known[cat.ID] = true
	}
	counts := make(map[string]int)
	uncategorized := 0
	for _, p := range c.products {
		if exclude[p.Name] {
			continue
		}
		if known[p.Category] {
			counts[p.Category]++
		} else {
			uncategorized++
		}
	}
	c.mu.Unlock()

	out := make([]string, 0, len(c.categories)+1)
	for _, cat := range c.categories {
		if counts[cat.ID] > 0 {
			out = append(out, cat.ID)
		}
	}
	if uncategorized > 0 {
		out = append(out, UncategorizedID)
	}
	return out
}

// Sample returns up to n products not in exclude, in random order.
func (c *Catalog) Sample(exclude map[string]bool, n int) []Product {
	c.mu.Lock()
	available := make([]Product, 0, len(c.products))
	for _, p := range c.products {
		if !exclude[p.Name] {
			available = append(available, p)
		}
	}
	c.mu.Unlock()

	c.rng.Shuffle(len(available), func(i, j int) {
		available[i], available[j] = available[j], available[i]
	})
	if n > len(available) {
		n = len(available)
	}
	return available[:n]
}

// Add inserts p, replacing any existing product with the same Name.
func (c *Catalog) Add(p Product) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.products {
		if existing.Name == p.Name {
			c.products[i] = p
			return c.persistLocked()
		}
	}
	c.products = append(c.products, p)
	return c.persistLocked()
}

// Remove deletes the product with name. Returns nil even if the name is unknown.
func (c *Catalog) Remove(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range c.products {
		if p.Name == name {
			c.products = append(c.products[:i], c.products[i+1:]...)
			return c.persistLocked()
		}
	}
	return nil
}
