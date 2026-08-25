// Package screening holds the read-only content and pure scoring logic for
// the adult ADHD self-check mode: the ASRS v1.1 symptom checklist, the
// WURS-25 childhood retrospective, and the bot's own DSM-context module
// (consent, intro, onset question, life domains, result templates).
//
// The package is content-neutral by design: instrument texts are loaded
// verbatim from YAML files in the repository's screening/ directory and are
// never edited in code. Thresholds come from the content too — the code pins
// the expected canonical values in Validate so a foreign or tampered file
// fails fast at startup.
package screening

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// File names Load expects inside its content directory.
const (
	asrsFile   = "asrs_ru.yaml"
	wursFile   = "wurs25_ru.yaml"
	moduleFile = "dsm_module_ru.yaml"
)

// ScaleOption is one point of a 5-option frequency/intensity scale.
type ScaleOption struct {
	Score int    `yaml:"score"`
	Label string `yaml:"label"`
}

// AsrsItem is a single ASRS question with its per-item significance
// threshold (the "shaded box" rule of the official checklist).
type AsrsItem struct {
	ID                  int    `yaml:"id"`
	Domain              string `yaml:"domain"`
	Text                string `yaml:"text"`
	SignificantMinScore int    `yaml:"significant_min_score"`
}

// AsrsPartScoring carries part-level scoring parameters (part A only).
type AsrsPartScoring struct {
	PositiveScreenThreshold int `yaml:"positive_screen_threshold"`
}

// ASRS mirrors asrs_ru.yaml: the official 6-question part A plus the
// 12-question part B of the ASRS v1.1 Symptom Checklist.
type ASRS struct {
	Instruction string        `yaml:"instruction_official_ru"`
	Scale       []ScaleOption `yaml:"scale"`
	PartA       struct {
		Items   []AsrsItem      `yaml:"items"`
		Scoring AsrsPartScoring `yaml:"scoring"`
	} `yaml:"part_a"`
	PartB struct {
		TranslationNote string     `yaml:"translation_note"`
		Items           []AsrsItem `yaml:"items"`
	} `yaml:"part_b"`
	Attribution string `yaml:"attribution_text_ru"`
}

// WursItem is a single WURS-25 statement in masculine and feminine wording.
type WursItem struct {
	ID    int    `yaml:"id"`
	TextM string `yaml:"text_m"`
	TextF string `yaml:"text_f"`
}

// WursThreshold is one published cutoff with its role.
type WursThreshold struct {
	Cutoff int    `yaml:"cutoff"`
	Role   string `yaml:"role"` // primary | alternative_*
}

// WURS mirrors wurs25_ru.yaml.
type WURS struct {
	Instruction string        `yaml:"instruction_ru"`
	Scale       []ScaleOption `yaml:"scale"`
	Items       []WursItem    `yaml:"items"`
	Scoring     struct {
		Thresholds []WursThreshold `yaml:"thresholds"`
	} `yaml:"scoring"`
	Attribution string `yaml:"attribution_text_ru"`
}

// PrimaryCutoff returns the cutoff of the threshold with role "primary",
// or 0 when absent (Validate rejects such content).
func (w *WURS) PrimaryCutoff() int {
	for _, t := range w.Scoring.Thresholds {
		if t.Role == "primary" {
			return t.Cutoff
		}
	}
	return 0
}

// DomainPhaseText is one life-phase rendering of a domain (title + examples).
type DomainPhaseText struct {
	Title    string   `yaml:"title"`
	Examples []string `yaml:"examples"`
}

// DomainItem is one life domain with adulthood and childhood renderings.
type DomainItem struct {
	ID        string          `yaml:"id"`
	Adult     DomainPhaseText `yaml:"adult"`
	Childhood DomainPhaseText `yaml:"childhood"`
}

// InstrumentBlock is one per-instrument result block template. Deliberately
// lean: no per-instrument attributions or translation caveats — the single
// compact attribution line lives in Results.AttributionLine (owner decision:
// users see results, not methodology).
type InstrumentBlock struct {
	Title        string `yaml:"title"`
	ScoreLine    string `yaml:"score_line"`
	PositiveLine string `yaml:"positive_line"`
	NegativeLine string `yaml:"negative_line"`
	Note         string `yaml:"note"`
}

// Module mirrors dsm_module_ru.yaml — the bot's own content: consent, intro,
// onset question, life domains, result templates, and service strings.
// The traceability section of the file is docs-only and not parsed.
type Module struct {
	Meta struct {
		Title      string `yaml:"title"`
		Disclaimer string `yaml:"disclaimer"` // the ONLY caveat users see, kept short
	} `yaml:"meta"`
	Consent struct {
		Title       string `yaml:"title"`
		Body        string `yaml:"body"`
		AgreeButton string `yaml:"agree_button"`
		LaterButton string `yaml:"later_button"`
		Declined    string `yaml:"declined"`
	} `yaml:"consent"`
	Intro struct {
		Title          string `yaml:"title"`
		Body           string `yaml:"body"`
		StartButton    string `yaml:"start_button"`
		PostponeButton string `yaml:"postpone_button"`
	} `yaml:"intro"`
	CriterionB struct {
		Title              string `yaml:"title"`
		Question           string `yaml:"question"`
		YesLabel           string `yaml:"yes_label"`
		NoLabel            string `yaml:"no_label"`
		NoFollowup         string `yaml:"no_followup"`
		AgeInvalid         string `yaml:"age_invalid"`
		OnsetFactChildhood string `yaml:"onset_fact_childhood"`
		OnsetFactLater     string `yaml:"onset_fact_later"`
	} `yaml:"criterion_b"`
	Domains struct {
		AdultPrompt     string       `yaml:"adult_prompt"`     // one-line lead-in above domain 1 of the adult pass
		ChildhoodPrompt string       `yaml:"childhood_prompt"` // same for the childhood pass
		PositionAdult   string       `yaml:"position_adult"`   // "Сфера {current} из {total} · …" template
		PositionChild   string       `yaml:"position_child"`
		ExamplesLine    string       `yaml:"examples_line"` // "Например: {examples}." template
		Question        string       `yaml:"question"`      // the per-domain yes/no question
		YesButton       string       `yaml:"yes_button"`
		NoButton        string       `yaml:"no_button"`
		Items           []DomainItem `yaml:"items"`
	} `yaml:"domains"`
	Results struct {
		Heading     string `yaml:"heading"`
		Instruments struct {
			AsrsA InstrumentBlock `yaml:"asrs_a"`
			AsrsB InstrumentBlock `yaml:"asrs_b"`
			Wurs  InstrumentBlock `yaml:"wurs"`
		} `yaml:"instruments"`
		ContextFacts struct {
			Heading           string `yaml:"heading"`
			OnsetLine         string `yaml:"onset_line"`
			AdultDomainsLine  string `yaml:"adult_domains_line"`
			AdultDomainsEmpty string `yaml:"adult_domains_empty"`
			ChildDomainsLine  string `yaml:"child_domains_line"`
			ChildDomainsEmpty string `yaml:"child_domains_empty"`
		} `yaml:"context_facts"`
		Overall struct {
			Consistent    string            `yaml:"consistent"`
			Partial       string            `yaml:"partial"`
			NotConsistent string            `yaml:"not_consistent"`
			GapHints      map[string]string `yaml:"gap_hints"`
		} `yaml:"overall"`
		// AttributionLine is the single compact attribution rendered in the
		// result footer (ASRS © WHO/Kessler; WURS-25 — Ward et al.; DSM-5).
		AttributionLine string `yaml:"attribution_line"`
		Referral        struct {
			Heading string `yaml:"heading"`
			Body    string `yaml:"body"`
		} `yaml:"referral"`
		DoctorReport struct {
			LeadIn   string `yaml:"lead_in"`
			Template string `yaml:"template"`
		} `yaml:"doctor_report"`
	} `yaml:"results"`
	UI struct {
		ModeButton      string `yaml:"mode_button"`
		Progress        string `yaml:"progress"`
		ContinueButton  string `yaml:"continue_button"`
		PauseButton     string `yaml:"pause_button"`
		Paused          string `yaml:"paused"`
		Resumed         string `yaml:"resumed"`
		BlockBoundaries struct {
			AfterAsrsA string `yaml:"after_asrs_a"`
			AfterAsrsB string `yaml:"after_asrs_b"`
			AfterWurs  string `yaml:"after_wurs"`
		} `yaml:"block_boundaries"`
		AbandonConfirmed    string `yaml:"abandon_confirmed"`
		AbandonNoActive     string `yaml:"abandon_no_active"`
		DeleteConfirmPrompt string `yaml:"delete_confirm_prompt"`
		DeleteConfirmButton string `yaml:"delete_confirm_button"`
		DeleteCancelButton  string `yaml:"delete_cancel_button"`
		DeleteDone          string `yaml:"delete_done"`
		DeleteNothing       string `yaml:"delete_nothing"`
		FodmapGuard         string `yaml:"fodmap_guard"`
	} `yaml:"ui"`
}

// Content is the full screening content bundle.
type Content struct {
	ASRS   ASRS
	WURS   WURS
	Module Module
}

// Load reads the three fixed-name content files from dir and validates the
// resulting bundle. Any structural deviation from the canonical shape is an
// error — the bot must fail fast at startup rather than run with wrong
// instrument texts or thresholds.
func Load(dir string) (*Content, error) {
	var c Content
	if err := loadYAML(filepath.Join(dir, asrsFile), &c.ASRS); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, wursFile), &c.WURS); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, moduleFile), &c.Module); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func loadYAML(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("screening: read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("screening: parse %s: %w", path, err)
	}
	return nil
}

// FirstSentence returns the substring of s up to and including the first
// period. Empty when s contains no period.
func FirstSentence(s string) string {
	if i := strings.Index(s, "."); i >= 0 {
		return s[:i+1]
	}
	return ""
}

// Validate checks the loaded bundle against the canonical shape and pins the
// canonical thresholds. Returning an error means "this is not the content
// this code was written for" — the caller must refuse to start.
func (c *Content) Validate() error {
	if err := c.validateASRS(); err != nil {
		return fmt.Errorf("screening: asrs: %w", err)
	}
	if err := c.validateWURS(); err != nil {
		return fmt.Errorf("screening: wurs: %w", err)
	}
	if err := c.validateModule(); err != nil {
		return fmt.Errorf("screening: module: %w", err)
	}
	return nil
}

func validateScale(scale []ScaleOption) error {
	if len(scale) != 5 {
		return fmt.Errorf("scale must have exactly 5 options, got %d", len(scale))
	}
	for i, opt := range scale {
		if opt.Score != i {
			return fmt.Errorf("scale option %d has score %d, want %d", i, opt.Score, i)
		}
		if strings.TrimSpace(opt.Label) == "" {
			return fmt.Errorf("scale option %d has empty label", i)
		}
	}
	return nil
}

func validateAsrsItems(items []AsrsItem, wantCount, firstID int) error {
	if len(items) != wantCount {
		return fmt.Errorf("want %d items, got %d", wantCount, len(items))
	}
	for i, item := range items {
		if item.ID != firstID+i {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, firstID+i)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("item id %d has empty text", item.ID)
		}
		if item.SignificantMinScore != 2 && item.SignificantMinScore != 3 {
			return fmt.Errorf("item id %d has significant_min_score %d, want 2 or 3",
				item.ID, item.SignificantMinScore)
		}
	}
	return nil
}

func (c *Content) validateASRS() error {
	a := &c.ASRS
	if err := validateScale(a.Scale); err != nil {
		return err
	}
	if err := validateAsrsItems(a.PartA.Items, 6, 1); err != nil {
		return fmt.Errorf("part_a: %w", err)
	}
	if err := validateAsrsItems(a.PartB.Items, 12, 7); err != nil {
		return fmt.Errorf("part_b: %w", err)
	}
	if a.PartA.Scoring.PositiveScreenThreshold != 4 {
		return fmt.Errorf("part_a positive_screen_threshold is %d, want 4 (foreign content?)",
			a.PartA.Scoring.PositiveScreenThreshold)
	}
	if strings.TrimSpace(a.Instruction) == "" {
		return fmt.Errorf("empty instruction")
	}
	if FirstSentence(a.Instruction) == "" {
		return fmt.Errorf("instruction has no first sentence (no period)")
	}
	if strings.TrimSpace(a.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	if strings.TrimSpace(a.PartB.TranslationNote) == "" {
		return fmt.Errorf("empty part_b translation_note")
	}
	return nil
}

func (c *Content) validateWURS() error {
	w := &c.WURS
	if err := validateScale(w.Scale); err != nil {
		return err
	}
	if len(w.Items) != 25 {
		return fmt.Errorf("want 25 items, got %d", len(w.Items))
	}
	for i, item := range w.Items {
		if item.ID != i+1 {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		if strings.TrimSpace(item.TextM) == "" {
			return fmt.Errorf("item id %d has empty text_m", item.ID)
		}
		if strings.TrimSpace(item.TextF) == "" {
			return fmt.Errorf("item id %d has empty text_f", item.ID)
		}
	}
	primary := 0
	for _, t := range w.Scoring.Thresholds {
		if t.Role == "primary" {
			primary++
			if t.Cutoff != 46 {
				return fmt.Errorf("primary cutoff is %d, want 46 (foreign content?)", t.Cutoff)
			}
		}
	}
	if primary != 1 {
		return fmt.Errorf("want exactly one primary threshold, got %d", primary)
	}
	if strings.TrimSpace(w.Instruction) == "" {
		return fmt.Errorf("empty instruction")
	}
	if strings.TrimSpace(w.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	return nil
}

func (c *Content) validateModule() error {
	m := &c.Module

	named := func(pairs ...string) error {
		for i := 0; i+1 < len(pairs); i += 2 {
			if strings.TrimSpace(pairs[i+1]) == "" {
				return fmt.Errorf("empty %s", pairs[i])
			}
		}
		return nil
	}

	if err := named(
		"meta.title", m.Meta.Title,
		"meta.disclaimer", m.Meta.Disclaimer,
		"consent.title", m.Consent.Title,
		"consent.body", m.Consent.Body,
		"consent.agree_button", m.Consent.AgreeButton,
		"consent.later_button", m.Consent.LaterButton,
		"consent.declined", m.Consent.Declined,
		"intro.title", m.Intro.Title,
		"intro.body", m.Intro.Body,
		"intro.start_button", m.Intro.StartButton,
		"intro.postpone_button", m.Intro.PostponeButton,
		"criterion_b.title", m.CriterionB.Title,
		"criterion_b.question", m.CriterionB.Question,
		"criterion_b.yes_label", m.CriterionB.YesLabel,
		"criterion_b.no_label", m.CriterionB.NoLabel,
		"criterion_b.no_followup", m.CriterionB.NoFollowup,
		"criterion_b.age_invalid", m.CriterionB.AgeInvalid,
		"criterion_b.onset_fact_childhood", m.CriterionB.OnsetFactChildhood,
		"criterion_b.onset_fact_later", m.CriterionB.OnsetFactLater,
		"domains.adult_prompt", m.Domains.AdultPrompt,
		"domains.childhood_prompt", m.Domains.ChildhoodPrompt,
		"domains.position_adult", m.Domains.PositionAdult,
		"domains.position_child", m.Domains.PositionChild,
		"domains.examples_line", m.Domains.ExamplesLine,
		"domains.question", m.Domains.Question,
		"domains.yes_button", m.Domains.YesButton,
		"domains.no_button", m.Domains.NoButton,
	); err != nil {
		return err
	}

	if len(m.Domains.Items) != 5 {
		return fmt.Errorf("want exactly 5 domains, got %d", len(m.Domains.Items))
	}
	seen := make(map[string]bool)
	for _, d := range m.Domains.Items {
		if strings.TrimSpace(d.ID) == "" {
			return fmt.Errorf("domain with empty id")
		}
		if seen[d.ID] {
			return fmt.Errorf("duplicate domain id %q", d.ID)
		}
		seen[d.ID] = true
		if strings.TrimSpace(d.Adult.Title) == "" {
			return fmt.Errorf("domain %s: empty adult title", d.ID)
		}
		if strings.TrimSpace(d.Childhood.Title) == "" {
			return fmt.Errorf("domain %s: empty childhood title", d.ID)
		}
		// Each domain renders as ONE short message per pass — 1..3 examples
		// keep the "e.g.:" line a single line (the lean-UX canon).
		if n := len(d.Adult.Examples); n == 0 || n > 3 {
			return fmt.Errorf("domain %s: want 1..3 adult examples, got %d", d.ID, n)
		}
		if n := len(d.Childhood.Examples); n == 0 || n > 3 {
			return fmt.Errorf("domain %s: want 1..3 childhood examples, got %d", d.ID, n)
		}
	}

	res := &m.Results
	instr := func(name string, b InstrumentBlock, needVerdictLines, needNote bool) error {
		if strings.TrimSpace(b.Title) == "" {
			return fmt.Errorf("empty results.instruments.%s.title", name)
		}
		if strings.TrimSpace(b.ScoreLine) == "" {
			return fmt.Errorf("empty results.instruments.%s.score_line", name)
		}
		if needVerdictLines && (strings.TrimSpace(b.PositiveLine) == "" || strings.TrimSpace(b.NegativeLine) == "") {
			return fmt.Errorf("empty results.instruments.%s positive/negative line", name)
		}
		if needNote && strings.TrimSpace(b.Note) == "" {
			return fmt.Errorf("empty results.instruments.%s.note", name)
		}
		return nil
	}
	if err := instr("asrs_a", res.Instruments.AsrsA, true, false); err != nil {
		return err
	}
	if err := instr("asrs_b", res.Instruments.AsrsB, false, true); err != nil {
		return err
	}
	if err := instr("wurs", res.Instruments.Wurs, true, false); err != nil {
		return err
	}

	if err := named(
		"results.heading", res.Heading,
		"results.context_facts.heading", res.ContextFacts.Heading,
		"results.context_facts.onset_line", res.ContextFacts.OnsetLine,
		"results.context_facts.adult_domains_line", res.ContextFacts.AdultDomainsLine,
		"results.context_facts.adult_domains_empty", res.ContextFacts.AdultDomainsEmpty,
		"results.context_facts.child_domains_line", res.ContextFacts.ChildDomainsLine,
		"results.context_facts.child_domains_empty", res.ContextFacts.ChildDomainsEmpty,
		"results.overall.consistent", res.Overall.Consistent,
		"results.overall.partial", res.Overall.Partial,
		"results.overall.not_consistent", res.Overall.NotConsistent,
		"results.attribution_line", res.AttributionLine,
		"results.referral.heading", res.Referral.Heading,
		"results.referral.body", res.Referral.Body,
		"results.doctor_report.lead_in", res.DoctorReport.LeadIn,
		"results.doctor_report.template", res.DoctorReport.Template,
	); err != nil {
		return err
	}
	for _, hint := range []string{GapNoChildhoodOnset, GapNoCurrentSymptoms, GapFewDomains} {
		if strings.TrimSpace(res.Overall.GapHints[hint]) == "" {
			return fmt.Errorf("empty results.overall.gap_hints.%s", hint)
		}
	}

	ui := &m.UI
	if err := named(
		"ui.progress", ui.Progress,
		"ui.continue_button", ui.ContinueButton,
		"ui.pause_button", ui.PauseButton,
		"ui.paused", ui.Paused,
		"ui.resumed", ui.Resumed,
		"ui.block_boundaries.after_asrs_a", ui.BlockBoundaries.AfterAsrsA,
		"ui.block_boundaries.after_asrs_b", ui.BlockBoundaries.AfterAsrsB,
		"ui.block_boundaries.after_wurs", ui.BlockBoundaries.AfterWurs,
		"ui.abandon_confirmed", ui.AbandonConfirmed,
		"ui.delete_confirm_prompt", ui.DeleteConfirmPrompt,
		"ui.delete_confirm_button", ui.DeleteConfirmButton,
		"ui.delete_cancel_button", ui.DeleteCancelButton,
		"ui.delete_done", ui.DeleteDone,
		"ui.delete_nothing", ui.DeleteNothing,
	); err != nil {
		return err
	}

	if err := c.validateBranding(); err != nil {
		return err
	}
	return nil
}

// forbiddenBranding is the tool name whose text must never appear in this
// repository or its content. Assembled from bytes so the check's own source
// does not trip the repository-wide guard.
var forbiddenBranding = string([]byte{'d', 'i', 'v', 'a'})

// validateBranding rejects content mentioning the forbidden third-party
// instrument (its foundation prohibits chat-bot use of its text).
func (c *Content) validateBranding() error {
	check := func(where, s string) error {
		if strings.Contains(strings.ToLower(s), forbiddenBranding) {
			return fmt.Errorf("%s contains forbidden instrument branding", where)
		}
		return nil
	}
	for _, item := range c.ASRS.PartA.Items {
		if err := check(fmt.Sprintf("asrs part_a item %d", item.ID), item.Text); err != nil {
			return err
		}
	}
	for _, item := range c.ASRS.PartB.Items {
		if err := check(fmt.Sprintf("asrs part_b item %d", item.ID), item.Text); err != nil {
			return err
		}
	}
	for _, item := range c.WURS.Items {
		if err := check(fmt.Sprintf("wurs item %d", item.ID), item.TextM); err != nil {
			return err
		}
		if err := check(fmt.Sprintf("wurs item %d", item.ID), item.TextF); err != nil {
			return err
		}
	}
	for where, s := range map[string]string{
		"asrs instruction":  c.ASRS.Instruction,
		"asrs attribution":  c.ASRS.Attribution,
		"wurs instruction":  c.WURS.Instruction,
		"wurs attribution":  c.WURS.Attribution,
		"module title":      c.Module.Meta.Title,
		"module intro":      c.Module.Intro.Body,
		"module consent":    c.Module.Consent.Body,
		"module disclaimer": c.Module.Meta.Disclaimer,
	} {
		if err := check(where, s); err != nil {
			return err
		}
	}
	return nil
}
