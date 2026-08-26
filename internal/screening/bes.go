package screening

// BES content (the eating track's binge-eating instrument): types and pure
// scoring. The Russian text is the bot's own translation of the original
// English scale (bes_ru.yaml carries the provenance); the statement weights
// and the severity bands are pinned in Validate.

import (
	"fmt"
	"strings"
)

// Canonical BES band ids over the 0..46 sum.
const (
	BesBandLow      = "low"      // ≤ 17
	BesBandModerate = "moderate" // 18..26
	BesBandSevere   = "severe"   // ≥ 27
)

// BesStatement is one weighted statement inside a BES item group. Several
// statements of a group may share a weight — that is how the original scale
// works, and the weights are pinned by Validate.
type BesStatement struct {
	Score int    `yaml:"score"`
	Text  string `yaml:"text"`
	// TextEN is the English original, kept for traceability only — never
	// rendered to users.
	TextEN string `yaml:"text_en"`
}

// BesItem is one of the 16 groups of statements.
type BesItem struct {
	ID         int            `yaml:"id"`
	Statements []BesStatement `yaml:"statements"`
}

// BES mirrors bes_ru.yaml: the 16 weighted statement groups plus the
// severity bands over the 0..46 sum.
type BES struct {
	Instruction string    `yaml:"instruction_ru"`
	Items       []BesItem `yaml:"items"`
	Scoring     struct {
		Bands []SeverityBand `yaml:"bands"`
	} `yaml:"scoring"`
	Attribution string `yaml:"attribution_text_ru"`
}

// Score sums the sixteen answers (0..46). Answers hold the weight of the
// picked statement, so the sum is the scale's own total. Short/nil slices
// are zero-filled — a programmer error upstream, kept panic-free.
func (b *BES) Score(answers []int) int {
	total := 0
	for i := range b.Items {
		total += answerAt(answers, i)
	}
	return total
}

// Band maps a total score onto its severity-band id, or "" when no band
// covers it (Validate guarantees full 0..46 coverage for loaded content).
func (b *BES) Band(score int) string {
	for _, band := range b.Scoring.Bands {
		if score >= band.Min && score <= band.Max {
			return band.ID
		}
	}
	return ""
}

// MaxScore is the highest reachable sum — 46 for canonical content, because
// two groups top out at 2 rather than 3.
func (b *BES) MaxScore() int {
	total := 0
	for _, item := range b.Items {
		best := 0
		for _, st := range item.Statements {
			if st.Score > best {
				best = st.Score
			}
		}
		total += best
	}
	return total
}

// canonicalBesWeights are the original statement weights of each of the 16
// groups (Gormally et al. 1982). Note the repeated weights (groups 1, 3, 4,
// 7, 13) and the two groups that top out at 2 — that is why the maximum sum
// is 46 and not 48. Foreign weights are refused at startup.
var canonicalBesWeights = [][]int{
	{0, 0, 1, 3},
	{0, 1, 2, 3},
	{0, 1, 3, 3},
	{0, 0, 0, 2},
	{0, 1, 2, 3},
	{0, 1, 3},
	{0, 2, 3, 3},
	{0, 1, 2, 3},
	{0, 1, 2, 3},
	{0, 1, 2, 3},
	{0, 1, 2, 3},
	{0, 1, 2, 3},
	{0, 0, 2, 3},
	{0, 1, 2, 3},
	{0, 1, 2, 3},
	{0, 1, 2},
}

// canonicalBesMax is the maximum reachable sum of the canonical weights.
const canonicalBesMax = 46

// canonicalBesBands are the widely used severity boundaries over that sum.
var canonicalBesBands = []SeverityBand{
	{Min: 0, Max: 17, ID: BesBandLow},
	{Min: 18, Max: 26, ID: BesBandModerate},
	{Min: 27, Max: 46, ID: BesBandSevere},
}

func (c *EatingContent) validateBES() error {
	b := &c.BES
	if len(b.Items) != len(canonicalBesWeights) {
		return fmt.Errorf("want %d items, got %d", len(canonicalBesWeights), len(b.Items))
	}
	for i, item := range b.Items {
		if item.ID != i+1 {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		want := canonicalBesWeights[i]
		if len(item.Statements) != len(want) {
			return fmt.Errorf("item id %d has %d statements, want %d",
				item.ID, len(item.Statements), len(want))
		}
		for j, st := range item.Statements {
			if st.Score != want[j] {
				return fmt.Errorf("item id %d statement %d has weight %d, want %d (foreign content?)",
					item.ID, j+1, st.Score, want[j])
			}
			if strings.TrimSpace(st.Text) == "" {
				return fmt.Errorf("item id %d statement %d has empty text", item.ID, j+1)
			}
		}
	}
	if got := b.MaxScore(); got != canonicalBesMax {
		return fmt.Errorf("maximum reachable score is %d, want %d", got, canonicalBesMax)
	}
	if len(b.Scoring.Bands) != len(canonicalBesBands) {
		return fmt.Errorf("want %d bands, got %d", len(canonicalBesBands), len(b.Scoring.Bands))
	}
	for i, want := range canonicalBesBands {
		if b.Scoring.Bands[i] != want {
			return fmt.Errorf("band %d is %+v, want %+v (foreign content?)", i, b.Scoring.Bands[i], want)
		}
	}
	if strings.TrimSpace(b.Instruction) == "" {
		return fmt.Errorf("empty instruction")
	}
	if strings.TrimSpace(b.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	return nil
}
