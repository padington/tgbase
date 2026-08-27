package screening

// EDE-QS content (the eating track's core instrument): types and pure
// scoring. The Russian text is the bot's own translation of the English
// original published with the validation paper (edeqs_ru.yaml carries the
// provenance); the screening cutoff is pinned in Validate.

import (
	"fmt"
	"strings"
)

// Canonical EDE-QS band ids. The instrument has exactly two readings — the
// screening cutoff is the only boundary the literature defines.
const (
	EdeqsBandBelow     = "below"       // score < cutoff
	EdeqsBandAtOrAbove = "at_or_above" // score >= cutoff (Prnjak et al. 2020: ≥ 15)
	EdeqsScaleDays     = "days"        // items 1–10: "on how many of the past 7 days…"
	EdeqsScaleSeverity = "severity"    // items 11–12: "over the past 7 days…"
)

// EdeqsItem is a single EDE-QS item. Scale names which of the two answer
// scales the item uses — the original form switches after item 10.
type EdeqsItem struct {
	ID    int    `yaml:"id"`
	Scale string `yaml:"scale"` // EdeqsScaleDays | EdeqsScaleSeverity
	Text  string `yaml:"text"`
	// TextEN is the English original, kept for traceability only — it is
	// never rendered to users.
	TextEN string `yaml:"text_en"`
}

// EDEQS mirrors edeqs_ru.yaml: the 12 items with their two answer scales
// plus the pinned screening cutoff and the two reading bands.
type EDEQS struct {
	Instruction    string        `yaml:"instruction_ru"`
	DaysHeader     string        `yaml:"days_header_ru"`     // shown once above item 1
	SeverityHeader string        `yaml:"severity_header_ru"` // shown once above item 11
	ScaleDays      []ScaleOption `yaml:"scale_days"`
	ScaleSeverity  []ScaleOption `yaml:"scale_severity"`
	Items          []EdeqsItem   `yaml:"items"`
	Scoring        struct {
		Cutoff int            `yaml:"cutoff"`
		Bands  []SeverityBand `yaml:"bands"`
	} `yaml:"scoring"`
	Attribution string `yaml:"attribution_text_original"`
}

// ScaleFor returns the answer scale of item index i (0-based). Out-of-range
// indexes fall back to the days scale — scoring stays panic-free.
func (e *EDEQS) ScaleFor(i int) []ScaleOption {
	if i >= 0 && i < len(e.Items) && e.Items[i].Scale == EdeqsScaleSeverity {
		return e.ScaleSeverity
	}
	return e.ScaleDays
}

// Score sums the twelve answers (0..36). Short/nil slices are zero-filled —
// a programmer error upstream, kept panic-free like the other scorers.
func (e *EDEQS) Score(answers []int) int {
	total := 0
	for i := range e.Items {
		total += answerAt(answers, i)
	}
	return total
}

// Cutoff is the applied screening cutoff (15). Copied into the stored result
// so a later content change cannot rewrite history.
func (e *EDEQS) Cutoff() int { return e.Scoring.Cutoff }

// Positive reports whether a score reaches the screening cutoff — the "worth
// discussing with a doctor" line, never a diagnosis.
func (e *EDEQS) Positive(score int) bool { return score >= e.Scoring.Cutoff }

// Band maps a total score onto its reading-band id, or "" when no band
// covers it (Validate guarantees full 0..36 coverage for loaded content).
func (e *EDEQS) Band(score int) string {
	for _, b := range e.Scoring.Bands {
		if score >= b.Min && score <= b.Max {
			return b.ID
		}
	}
	return ""
}

// canonicalEdeqsCutoff is the published screening cutoff (Prnjak et al.
// 2020) — foreign values are refused at startup.
const canonicalEdeqsCutoff = 15

// canonicalEdeqsBands are the two readings around that cutoff.
var canonicalEdeqsBands = []SeverityBand{
	{Min: 0, Max: 14, ID: EdeqsBandBelow},
	{Min: 15, Max: 36, ID: EdeqsBandAtOrAbove},
}

// canonicalEdeqsScales pins which scale each of the twelve items uses: the
// original switches from the days scale to the severity scale after item 10.
func canonicalEdeqsScale(id int) string {
	if id >= 11 {
		return EdeqsScaleSeverity
	}
	return EdeqsScaleDays
}

func (c *EatingContent) validateEDEQS() error {
	e := &c.EDEQS
	if err := validateScale(e.ScaleDays, 4); err != nil {
		return fmt.Errorf("scale_days: %w", err)
	}
	if err := validateScale(e.ScaleSeverity, 4); err != nil {
		return fmt.Errorf("scale_severity: %w", err)
	}
	if len(e.Items) != 12 {
		return fmt.Errorf("want 12 items, got %d", len(e.Items))
	}
	for i, item := range e.Items {
		if item.ID != i+1 {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("item id %d has empty text", item.ID)
		}
		if want := canonicalEdeqsScale(item.ID); item.Scale != want {
			return fmt.Errorf("item id %d uses scale %q, want %q (foreign content?)",
				item.ID, item.Scale, want)
		}
	}
	if e.Scoring.Cutoff != canonicalEdeqsCutoff {
		return fmt.Errorf("cutoff is %d, want %d (foreign content?)",
			e.Scoring.Cutoff, canonicalEdeqsCutoff)
	}
	if len(e.Scoring.Bands) != len(canonicalEdeqsBands) {
		return fmt.Errorf("want %d bands, got %d", len(canonicalEdeqsBands), len(e.Scoring.Bands))
	}
	for i, want := range canonicalEdeqsBands {
		if e.Scoring.Bands[i] != want {
			return fmt.Errorf("band %d is %+v, want %+v (foreign content?)", i, e.Scoring.Bands[i], want)
		}
	}
	if strings.TrimSpace(e.Instruction) == "" {
		return fmt.Errorf("empty instruction")
	}
	if strings.TrimSpace(e.DaysHeader) == "" {
		return fmt.Errorf("empty days header")
	}
	if strings.TrimSpace(e.SeverityHeader) == "" {
		return fmt.Errorf("empty severity header")
	}
	if strings.TrimSpace(e.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	return nil
}
