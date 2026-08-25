package screening

// GAD-7 anxiety-screening content (the mood module's anxiety test): types
// and pure scoring. The verbatim Russian text comes from the official
// phqscreeners.com PDF (gad7_ru.yaml); the published severity bands
// (Spitzer et al. 2006) are pinned in Validate.

import (
	"fmt"
	"strings"
)

// Gad7Item is a single GAD-7 item.
type Gad7Item struct {
	ID   int    `yaml:"id"`
	Text string `yaml:"text"`
}

// GAD7 mirrors gad7_ru.yaml: the official Russian GAD-7 (verbatim) plus the
// canonical severity bands. Band ids reuse the shared Band* constants
// (minimal / mild / moderate / severe — GAD-7 has no moderately_severe).
type GAD7 struct {
	Instruction string        `yaml:"instruction_official_ru"`
	Scale       []ScaleOption `yaml:"scale"` // 4 options, same Russian labels as PHQ-9
	Items       []Gad7Item    `yaml:"items"`
	Scoring     struct {
		Bands []SeverityBand `yaml:"bands"`
	} `yaml:"scoring"`
	Attribution string `yaml:"attribution_text_ru"`
}

// Score sums the seven answers (0..21). Short/nil slices are zero-filled —
// a programmer error upstream, kept panic-free like the other scorers.
func (g *GAD7) Score(answers []int) int {
	total := 0
	for i := range g.Items {
		total += answerAt(answers, i)
	}
	return total
}

// Band maps a total score onto its severity-band id, or "" when no band
// covers it (Validate guarantees full 0..21 coverage for loaded content).
func (g *GAD7) Band(score int) string {
	for _, b := range g.Scoring.Bands {
		if score >= b.Min && score <= b.Max {
			return b.ID
		}
	}
	return ""
}

// canonicalGad7Bands are the published GAD-7 severity bands (Spitzer,
// Kroenke, Williams, Löwe 2006) — foreign boundaries are refused at startup.
var canonicalGad7Bands = []SeverityBand{
	{Min: 0, Max: 4, ID: BandMinimal},
	{Min: 5, Max: 9, ID: BandMild},
	{Min: 10, Max: 14, ID: BandModerate},
	{Min: 15, Max: 21, ID: BandSevere},
}

func (c *MoodContent) validateGAD7() error {
	g := &c.GAD7
	if err := validateScale(g.Scale, 4); err != nil {
		return err
	}
	if len(g.Items) != 7 {
		return fmt.Errorf("want 7 items, got %d", len(g.Items))
	}
	for i, item := range g.Items {
		if item.ID != i+1 {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("item id %d has empty text", item.ID)
		}
	}
	if len(g.Scoring.Bands) != len(canonicalGad7Bands) {
		return fmt.Errorf("want %d severity bands, got %d", len(canonicalGad7Bands), len(g.Scoring.Bands))
	}
	for i, want := range canonicalGad7Bands {
		if g.Scoring.Bands[i] != want {
			return fmt.Errorf("severity band %d is %+v, want %+v (foreign content?)", i, g.Scoring.Bands[i], want)
		}
	}
	if strings.TrimSpace(g.Instruction) == "" {
		return fmt.Errorf("empty instruction")
	}
	if strings.TrimSpace(g.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	return nil
}
