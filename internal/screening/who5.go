package screening

// WHO-5 well-being index content (the mood module's quick check): types and
// pure scoring. The verbatim Russian text comes from the official WHO
// publication WHO-UCN-MSD-MHE-2024.01 (who5_ru.yaml); the interpretation
// bands over the 0–100 score are pinned in Validate.

import (
	"fmt"
	"strings"
)

// Canonical WHO-5 interpretation-band ids. These map 1:1 onto the
// who5.results.bands.* templates of the mood module.
const (
	Who5BandOK      = "ok"       // > 50 — well-being in the normal range
	Who5BandLow     = "low"      // ≤ 50 — reduced well-being (standard WHO-5 screening cutoff)
	Who5BandVeryLow = "very_low" // ≤ 28 — markedly reduced well-being
)

// Who5Item is a single WHO-5 statement (positively worded, per the form).
type Who5Item struct {
	ID   int    `yaml:"id"`
	Text string `yaml:"text"`
}

// WHO5 mirrors who5_ru.yaml: the official Russian WHO-5 (verbatim) plus the
// bot's interpretation bands over the 0–100 score.
type WHO5 struct {
	// RecallHeader is the verbatim row header of the paper form («Последние
	// две недели»), shown once above statement 1 — it carries the recall
	// period after the paper-bound instruction sentences are dropped.
	RecallHeader string        `yaml:"recall_header_official_ru"`
	Scale        []ScaleOption `yaml:"scale"` // 6 options, form order: best first (5 → 0)
	Items        []Who5Item    `yaml:"items"`
	Scoring      struct {
		Multiplier int            `yaml:"multiplier"` // 4, per the official scoring instruction
		Bands      []SeverityBand `yaml:"bands"`      // over the 0..100 score
	} `yaml:"scoring"`
	Attribution string `yaml:"attribution_text_ru"`
}

// Score sums the five answers (0..25) and applies the official ×4
// multiplier, yielding the 0..100 well-being score. Short/nil slices are
// zero-filled — a programmer error upstream, kept panic-free like the other
// scorers.
func (w *WHO5) Score(answers []int) int {
	sum := 0
	for i := range w.Items {
		sum += answerAt(answers, i)
	}
	return sum * w.Scoring.Multiplier
}

// Band maps a 0..100 score onto its interpretation-band id, or "" when no
// band covers it (Validate guarantees full 0..100 coverage for loaded
// content).
func (w *WHO5) Band(score int) string {
	for _, b := range w.Scoring.Bands {
		if score >= b.Min && score <= b.Max {
			return b.ID
		}
	}
	return ""
}

// canonicalWho5Bands are the bot's pinned WHO-5 interpretation boundaries:
// the standard ≤ 50 screening cutoff plus the ≤ 28 marked-reduction line.
// Note the raw sum is multiplied by 4, so only multiples of 4 are reachable
// — the 29..50 band in practice means scores 32..48.
var canonicalWho5Bands = []SeverityBand{
	{Min: 0, Max: 28, ID: Who5BandVeryLow},
	{Min: 29, Max: 50, ID: Who5BandLow},
	{Min: 51, Max: 100, ID: Who5BandOK},
}

func (c *MoodContent) validateWHO5() error {
	w := &c.WHO5
	// The WHO-5 scale is stored in the official top-down order (5 → 0), so
	// the generic ascending validateScale does not apply.
	if len(w.Scale) != 6 {
		return fmt.Errorf("scale must have exactly 6 options, got %d", len(w.Scale))
	}
	for i, opt := range w.Scale {
		if want := 5 - i; opt.Score != want {
			return fmt.Errorf("scale option %d has score %d, want %d (form order: 5 → 0)", i, opt.Score, want)
		}
		if strings.TrimSpace(opt.Label) == "" {
			return fmt.Errorf("scale option %d has empty label", i)
		}
	}
	if len(w.Items) != 5 {
		return fmt.Errorf("want 5 items, got %d", len(w.Items))
	}
	for i, item := range w.Items {
		if item.ID != i+1 {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("item id %d has empty text", item.ID)
		}
	}
	if w.Scoring.Multiplier != 4 {
		return fmt.Errorf("multiplier is %d, want 4 (official scoring instruction; foreign content?)", w.Scoring.Multiplier)
	}
	if len(w.Scoring.Bands) != len(canonicalWho5Bands) {
		return fmt.Errorf("want %d bands, got %d", len(canonicalWho5Bands), len(w.Scoring.Bands))
	}
	for i, want := range canonicalWho5Bands {
		if w.Scoring.Bands[i] != want {
			return fmt.Errorf("band %d is %+v, want %+v (foreign content?)", i, w.Scoring.Bands[i], want)
		}
	}
	if strings.TrimSpace(w.RecallHeader) == "" {
		return fmt.Errorf("empty recall header")
	}
	if strings.TrimSpace(w.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	return nil
}
