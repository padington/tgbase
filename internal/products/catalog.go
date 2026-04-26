// Package products holds the FODMAP food catalog and stage progression.
// The Catalog is backed by store.Backend so the list is mutable at runtime;
// on first boot it seeds itself from a bundled YAML file.
package products

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
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

type Product struct {
	Name          string            `yaml:"name" json:"name"`
	Fodmap        FodmapLevel       `yaml:"fodmap" json:"fodmap"`
	Measure       Measure           `yaml:"measure" json:"measure"`
	Stages        map[Stage]float64 `yaml:"stages" json:"stages"`
	Note          string            `yaml:"note,omitempty" json:"note,omitempty"`
	NameLocalized map[string]string `yaml:"name_localized,omitempty" json:"name_localized,omitempty"`
	NoteLocalized map[string]string `yaml:"note_localized,omitempty" json:"note_localized,omitempty"`
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

// Catalog wraps a store.Backend with typed access to a list of products.
// Methods are safe for concurrent use.
type Catalog struct {
	mu      sync.Mutex
	backend store.Backend
	cache   []Product
	rng     *rand.Rand
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
		var loaded []Product
		if err := json.Unmarshal(raw, &loaded); err != nil {
			return fmt.Errorf("parse backend products: %w", err)
		}
		c.cache = loaded
		return nil
	}
	if seedPath == "" {
		c.cache = []Product{}
		return nil
	}
	data, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed %s: %w", seedPath, err)
	}
	var seeded []Product
	if err := yaml.Unmarshal(data, &seeded); err != nil {
		return fmt.Errorf("parse seed %s: %w", seedPath, err)
	}
	c.cache = seeded
	return c.persistLocked()
}

func (c *Catalog) persistLocked() error {
	raw, err := json.Marshal(c.cache)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.backend.Put(backendKey, raw)
}

func (c *Catalog) All() []Product {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Product, len(c.cache))
	copy(out, c.cache)
	return out
}

func (c *Catalog) Find(name string) (Product, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.cache {
		if p.Name == name {
			return p, true
		}
	}
	return Product{}, false
}

// Sample returns up to n products not in exclude, in random order.
func (c *Catalog) Sample(exclude map[string]bool, n int) []Product {
	c.mu.Lock()
	available := make([]Product, 0, len(c.cache))
	for _, p := range c.cache {
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
	for i, existing := range c.cache {
		if existing.Name == p.Name {
			c.cache[i] = p
			return c.persistLocked()
		}
	}
	c.cache = append(c.cache, p)
	return c.persistLocked()
}

// Remove deletes the product with name. Returns nil even if the name is unknown.
func (c *Catalog) Remove(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range c.cache {
		if p.Name == name {
			c.cache = append(c.cache[:i], c.cache[i+1:]...)
			return c.persistLocked()
		}
	}
	return nil
}
