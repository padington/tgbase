package screening

// NIAS content (the eating track's restrictive-eating instrument): types,
// pure subscale scoring, and the deterministic reading rule that combines a
// positive subscale with the EDE-QS result. The Russian text is the bot's
// own translation of the English original (nias_ru.yaml carries the
// provenance); the subscale cutoffs are pinned in Validate.

import (
	"fmt"
	"strings"
)

// Canonical NIAS subscale ids, in item order: items 1–3, 4–6, 7–9.
const (
	NiasPicky    = "picky"
	NiasAppetite = "appetite"
	NiasFear     = "fear"
)

// Reading keys of the Burton Murray rule. They map 1:1 onto the
// nias.results.contexts.* / doctor_report.nias_contexts.* templates.
//
// A NIAS subscale above its cutoff does not by itself distinguish
// restrictive eating without shape/weight concern from restriction driven
// by it — that needs a second screen. Burton Murray et al. (2021) pair the
// NIAS with an eating-disorder screen (EDE-Q Global < 2.3 there, the short
// form EDE-QS with its cutoff here).
const (
	NiasCtxNone                   = ""                          // no subscale above its cutoff
	NiasCtxRestrictiveNoBodyImage = "restrictive_no_body_image" // subscale positive + EDE-QS below cutoff
	NiasCtxRestrictiveBodyImage   = "restrictive_body_image"    // subscale positive + EDE-QS at/above cutoff
	NiasCtxRestrictiveUnknown     = "restrictive_unknown"       // subscale positive, EDE-QS not taken
)

// niasSubscaleOrder is the canonical subscale order, also used to render
// the result block.
var niasSubscaleOrder = []string{NiasPicky, NiasAppetite, NiasFear}

// NiasItem is a single NIAS statement with the subscale it belongs to.
type NiasItem struct {
	ID       int    `yaml:"id"`
	Subscale string `yaml:"subscale"`
	Text     string `yaml:"text"`
	// TextEN is the English original, kept for traceability only — never
	// rendered to users.
	TextEN string `yaml:"text_en"`
}

// NiasSubscale is one subscale with its applied cutoff.
type NiasSubscale struct {
	ID     string `yaml:"id"`
	Cutoff int    `yaml:"cutoff"`
}

// NIAS mirrors nias_ru.yaml: the 6-point Likert scale, the nine statements
// grouped into three subscales, and the subscale cutoffs. There is
// deliberately NO total score — the instrument is read per subscale.
type NIAS struct {
	Instruction string        `yaml:"instruction_ru"`
	Scale       []ScaleOption `yaml:"scale"` // 6 options, 0..5
	Items       []NiasItem    `yaml:"items"`
	Scoring     struct {
		Subscales []NiasSubscale `yaml:"subscales"`
	} `yaml:"scoring"`
	Attribution string `yaml:"attribution_text_ru"`
}

// NiasScores holds the three subscale sums (each 0..15).
type NiasScores struct {
	Picky    int
	Appetite int
	Fear     int
}

// Score sums the answers into the three subscales. Short/nil slices are
// zero-filled — a programmer error upstream, kept panic-free like the other
// scorers. No total is returned by design.
func (n *NIAS) Score(answers []int) NiasScores {
	var out NiasScores
	for i, item := range n.Items {
		v := answerAt(answers, i)
		switch item.Subscale {
		case NiasPicky:
			out.Picky += v
		case NiasAppetite:
			out.Appetite += v
		case NiasFear:
			out.Fear += v
		}
	}
	return out
}

// Cutoff returns the applied cutoff of a subscale, 0 when unknown.
func (n *NIAS) Cutoff(subscale string) int {
	for _, s := range n.Scoring.Subscales {
		if s.ID == subscale {
			return s.Cutoff
		}
	}
	return 0
}

// SubscaleOrder is the canonical rendering order of the three subscales.
func (n *NIAS) SubscaleOrder() []string { return niasSubscaleOrder }

// NiasContext applies the Burton Murray reading rule and returns the wording
// key for the result and the doctor report:
//
//	no subscale above its cutoff          → NiasCtxNone (no line at all);
//	subscale positive, EDE-QS below cutoff → restrictive pattern WITHOUT
//	                                         shape/weight concern;
//	subscale positive, EDE-QS at/above     → restriction likely tied to
//	                                         shape/weight concern;
//	subscale positive, no EDE-QS run yet   → neutral wording, no distinction.
//
// Pure and deterministic: the caller passes facts, never content.
func NiasContext(anySubscalePositive, edeqsTaken, edeqsPositive bool) string {
	if !anySubscalePositive {
		return NiasCtxNone
	}
	if !edeqsTaken {
		return NiasCtxRestrictiveUnknown
	}
	if edeqsPositive {
		return NiasCtxRestrictiveBodyImage
	}
	return NiasCtxRestrictiveNoBodyImage
}

// canonicalNiasSubscales are the published subscale cutoffs (Burton Murray
// et al. 2021) in canonical item order — foreign values are refused at
// startup.
var canonicalNiasSubscales = []NiasSubscale{
	{ID: NiasPicky, Cutoff: 10},
	{ID: NiasAppetite, Cutoff: 9},
	{ID: NiasFear, Cutoff: 10},
}

func (c *EatingContent) validateNIAS() error {
	n := &c.NIAS
	if err := validateScale(n.Scale, 6); err != nil {
		return err
	}
	if len(n.Items) != 9 {
		return fmt.Errorf("want 9 items, got %d", len(n.Items))
	}
	for i, item := range n.Items {
		if item.ID != i+1 {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("item id %d has empty text", item.ID)
		}
		// Items are grouped three per subscale, in the canonical order.
		if want := niasSubscaleOrder[i/3]; item.Subscale != want {
			return fmt.Errorf("item id %d is in subscale %q, want %q (foreign content?)",
				item.ID, item.Subscale, want)
		}
	}
	if len(n.Scoring.Subscales) != len(canonicalNiasSubscales) {
		return fmt.Errorf("want %d subscales, got %d",
			len(canonicalNiasSubscales), len(n.Scoring.Subscales))
	}
	for i, want := range canonicalNiasSubscales {
		if n.Scoring.Subscales[i] != want {
			return fmt.Errorf("subscale %d is %+v, want %+v (foreign content?)",
				i, n.Scoring.Subscales[i], want)
		}
	}
	if strings.TrimSpace(n.Instruction) == "" {
		return fmt.Errorf("empty instruction")
	}
	if strings.TrimSpace(n.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	return nil
}
