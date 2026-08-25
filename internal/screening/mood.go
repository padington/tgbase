package screening

// PHQ-9 depression-screening content (the mood self-check mode): types,
// loader, validation, and pure scoring. Kept as a separate bundle from the
// ADHD Content so each mode's content has an independent lifecycle — the
// loading/validation/scoring conventions are shared (same file layout, same
// verbatim-instrument canon, thresholds pinned in Validate).

import (
	"fmt"
	"path/filepath"
	"strings"
)

// File names LoadMood expects inside its content directory.
const (
	phq9File       = "phq9_ru.yaml"
	moodModuleFile = "mood_module_ru.yaml"
)

// Canonical severity-band ids (Kroenke, Spitzer, Williams 2001). These map
// 1:1 onto the results.bands.* templates of the mood module.
const (
	BandMinimal          = "minimal"
	BandMild             = "mild"
	BandModerate         = "moderate"
	BandModeratelySevere = "moderately_severe"
	BandSevere           = "severe"
)

// forbiddenCrisisNumber is the children's helpline that must never appear in
// the crisis contacts of an adult screening — a deterministic content guard.
const forbiddenCrisisNumber = "8-800-2000-122"

// PhqItem is a single PHQ-9 item. Crisis marks the self-harm item (id 9):
// any answer > 0 on it deterministically triggers the crisis card.
type PhqItem struct {
	ID     int    `yaml:"id"`
	Text   string `yaml:"text"`
	Crisis bool   `yaml:"crisis"`
}

// SeverityBand is one score band of the PHQ-9 severity ladder.
type SeverityBand struct {
	Min int    `yaml:"min"`
	Max int    `yaml:"max"`
	ID  string `yaml:"id"`
}

// PHQ9 mirrors phq9_ru.yaml: the official Russian PHQ-9 (verbatim) plus the
// canonical severity bands.
type PHQ9 struct {
	Instruction string        `yaml:"instruction_official_ru"`
	Scale       []ScaleOption `yaml:"scale"`
	Items       []PhqItem     `yaml:"items"`
	Scoring     struct {
		Bands []SeverityBand `yaml:"bands"`
	} `yaml:"scoring"`
	Attribution string `yaml:"attribution_text_ru"`
}

// Score sums the answers (0..27). Short/nil slices are zero-filled — a
// programmer error upstream, kept panic-free like the ADHD scorers.
func (p *PHQ9) Score(answers []int) int {
	total := 0
	for i := range p.Items {
		total += answerAt(answers, i)
	}
	return total
}

// Band maps a total score onto its severity-band id, or "" when no band
// covers it (Validate guarantees full 0..27 coverage for loaded content).
func (p *PHQ9) Band(score int) string {
	for _, b := range p.Scoring.Bands {
		if score >= b.Min && score <= b.Max {
			return b.ID
		}
	}
	return ""
}

// CrisisAnswer returns the answer given to the crisis item (id 9), 0 when
// not answered yet. Any value > 0 must trigger the crisis card.
func (p *PHQ9) CrisisAnswer(answers []int) int {
	for i, item := range p.Items {
		if item.Crisis {
			return answerAt(answers, i)
		}
	}
	return 0
}

// MoodModule mirrors mood_module_ru.yaml — the bot's own texts around the
// PHQ-9: consent, resume, the crisis card, result templates, doctor report,
// and service strings. Everything here is original bot content.
type MoodModule struct {
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
	Resume struct {
		Body           string `yaml:"body"` // "…вопросе {current} из {total}" template
		ContinueButton string `yaml:"continue_button"`
		RestartButton  string `yaml:"restart_button"`
		LaterButton    string `yaml:"later_button"`
	} `yaml:"resume"`
	Crisis struct {
		Lead           string `yaml:"lead"`
		Contacts       string `yaml:"contacts"`
		UrgentLine     string `yaml:"urgent_line"` // added when the crisis answer is 2–3
		ContinueButton string `yaml:"continue_button"`
	} `yaml:"crisis"`
	Results struct {
		Heading         string            `yaml:"heading"`
		ScoreLine       string            `yaml:"score_line"` // "{score}" template
		Bands           map[string]string `yaml:"bands"`      // band id → user wording
		DeltaLine       string            `yaml:"delta_line"` // "{ago} {prev} {cur}" template
		AgoToday        string            `yaml:"ago_today"`
		AgoDays         string            `yaml:"ago_days"`  // "{n}" template
		AgoWeeks        string            `yaml:"ago_weeks"` // "{n}" template
		RetestLine      string            `yaml:"retest_line"`
		CrisisHeading   string            `yaml:"crisis_heading"` // above the repeated contacts
		AttributionLine string            `yaml:"attribution_line"`
	} `yaml:"results"`
	DoctorReport struct {
		LeadIn      string `yaml:"lead_in"`
		Q9Marked    string `yaml:"q9_marked"`
		Q9NotMarked string `yaml:"q9_not_marked"`
		Template    string `yaml:"template"`
	} `yaml:"doctor_report"`
	UI struct {
		ModeButton          string `yaml:"mode_button"`
		Progress            string `yaml:"progress"`
		Paused              string `yaml:"paused"`
		AbandonConfirmed    string `yaml:"abandon_confirmed"`
		DeleteConfirmPrompt string `yaml:"delete_confirm_prompt"`
		DeleteConfirmButton string `yaml:"delete_confirm_button"`
		DeleteCancelButton  string `yaml:"delete_cancel_button"`
		DeleteDone          string `yaml:"delete_done"`
		DeleteNothing       string `yaml:"delete_nothing"`
	} `yaml:"ui"`
}

// MoodContent is the full mood-screening content bundle.
type MoodContent struct {
	PHQ9   PHQ9
	Module MoodModule
}

// LoadMood reads the two fixed-name content files from dir and validates the
// bundle. Any structural deviation from the canonical shape is an error —
// the bot must fail fast at startup rather than run with wrong instrument
// texts, thresholds, or crisis contacts.
func LoadMood(dir string) (*MoodContent, error) {
	var c MoodContent
	if err := loadYAML(filepath.Join(dir, phq9File), &c.PHQ9); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, moodModuleFile), &c.Module); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate checks the loaded bundle against the canonical shape and pins the
// canonical values: 4-option scale, 9 items with the crisis flag exactly on
// item 9, the published severity-band boundaries, and the crisis-contacts
// guard (adult lines only). An error means "this is not the content this
// code was written for" — the caller must refuse to start.
func (c *MoodContent) Validate() error {
	if err := c.validatePHQ9(); err != nil {
		return fmt.Errorf("screening: phq9: %w", err)
	}
	if err := c.validateMoodModule(); err != nil {
		return fmt.Errorf("screening: mood module: %w", err)
	}
	return nil
}

// canonicalBands are the published PHQ-9 severity bands (Kroenke 2001) —
// foreign boundaries are refused at startup.
var canonicalBands = []SeverityBand{
	{Min: 0, Max: 4, ID: BandMinimal},
	{Min: 5, Max: 9, ID: BandMild},
	{Min: 10, Max: 14, ID: BandModerate},
	{Min: 15, Max: 19, ID: BandModeratelySevere},
	{Min: 20, Max: 27, ID: BandSevere},
}

func (c *MoodContent) validatePHQ9() error {
	p := &c.PHQ9
	if err := validateScale(p.Scale, 4); err != nil {
		return err
	}
	if len(p.Items) != 9 {
		return fmt.Errorf("want 9 items, got %d", len(p.Items))
	}
	for i, item := range p.Items {
		if item.ID != i+1 {
			return fmt.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("item id %d has empty text", item.ID)
		}
		if item.Crisis != (item.ID == 9) {
			return fmt.Errorf("item id %d: crisis flag must be set on item 9 only", item.ID)
		}
	}
	if len(p.Scoring.Bands) != len(canonicalBands) {
		return fmt.Errorf("want %d severity bands, got %d", len(canonicalBands), len(p.Scoring.Bands))
	}
	for i, want := range canonicalBands {
		if p.Scoring.Bands[i] != want {
			return fmt.Errorf("severity band %d is %+v, want %+v (foreign content?)",
				i, p.Scoring.Bands[i], want)
		}
	}
	if strings.TrimSpace(p.Instruction) == "" {
		return fmt.Errorf("empty instruction")
	}
	if strings.TrimSpace(p.Attribution) == "" {
		return fmt.Errorf("empty attribution")
	}
	return nil
}

func (c *MoodContent) validateMoodModule() error {
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
		"resume.body", m.Resume.Body,
		"resume.continue_button", m.Resume.ContinueButton,
		"resume.restart_button", m.Resume.RestartButton,
		"resume.later_button", m.Resume.LaterButton,
		"crisis.lead", m.Crisis.Lead,
		"crisis.contacts", m.Crisis.Contacts,
		"crisis.urgent_line", m.Crisis.UrgentLine,
		"crisis.continue_button", m.Crisis.ContinueButton,
		"results.heading", m.Results.Heading,
		"results.score_line", m.Results.ScoreLine,
		"results.delta_line", m.Results.DeltaLine,
		"results.ago_today", m.Results.AgoToday,
		"results.ago_days", m.Results.AgoDays,
		"results.ago_weeks", m.Results.AgoWeeks,
		"results.retest_line", m.Results.RetestLine,
		"results.crisis_heading", m.Results.CrisisHeading,
		"results.attribution_line", m.Results.AttributionLine,
		"doctor_report.lead_in", m.DoctorReport.LeadIn,
		"doctor_report.q9_marked", m.DoctorReport.Q9Marked,
		"doctor_report.q9_not_marked", m.DoctorReport.Q9NotMarked,
		"doctor_report.template", m.DoctorReport.Template,
		"ui.mode_button", m.UI.ModeButton,
		"ui.progress", m.UI.Progress,
		"ui.paused", m.UI.Paused,
		"ui.abandon_confirmed", m.UI.AbandonConfirmed,
		"ui.delete_confirm_prompt", m.UI.DeleteConfirmPrompt,
		"ui.delete_confirm_button", m.UI.DeleteConfirmButton,
		"ui.delete_cancel_button", m.UI.DeleteCancelButton,
		"ui.delete_done", m.UI.DeleteDone,
		"ui.delete_nothing", m.UI.DeleteNothing,
	); err != nil {
		return err
	}

	for _, band := range []string{BandMinimal, BandMild, BandModerate, BandModeratelySevere, BandSevere} {
		if strings.TrimSpace(m.Results.Bands[band]) == "" {
			return fmt.Errorf("empty results.bands.%s", band)
		}
	}

	// Crisis-contacts guard: the children's helpline must never be offered
	// in the adult screening.
	if strings.Contains(m.Crisis.Contacts, forbiddenCrisisNumber) {
		return fmt.Errorf("crisis.contacts carries the children's helpline %s", forbiddenCrisisNumber)
	}

	return c.validateMoodBranding()
}

// validateMoodBranding rejects mood content mentioning the forbidden
// third-party instrument, mirroring the ADHD bundle's guard.
func (c *MoodContent) validateMoodBranding() error {
	check := func(where, s string) error {
		if strings.Contains(strings.ToLower(s), forbiddenBranding) {
			return fmt.Errorf("%s contains forbidden instrument branding", where)
		}
		return nil
	}
	for _, item := range c.PHQ9.Items {
		if err := check(fmt.Sprintf("phq9 item %d", item.ID), item.Text); err != nil {
			return err
		}
	}
	for where, s := range map[string]string{
		"phq9 instruction":  c.PHQ9.Instruction,
		"phq9 attribution":  c.PHQ9.Attribution,
		"module title":      c.Module.Meta.Title,
		"module consent":    c.Module.Consent.Body,
		"module disclaimer": c.Module.Meta.Disclaimer,
		"module crisis":     c.Module.Crisis.Contacts,
	} {
		if err := check(where, s); err != nil {
			return err
		}
	}
	return nil
}
