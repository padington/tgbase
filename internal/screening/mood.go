package screening

// Mood-module content (PHQ-9 + WHO-5 + GAD-7): types, loader, validation,
// and the PHQ-9 scoring. Kept as a separate bundle from the ADHD Content so
// each mode's content has an independent lifecycle — the loading/validation/
// scoring conventions are shared (same file layout, same verbatim-instrument
// canon, thresholds pinned in Validate). WHO-5 lives in who5.go, GAD-7 in
// gad7.go; the shared module texts (menu, consent, offers, combined doctor
// report) are here.

import (
	"fmt"
	"path/filepath"
	"strings"
)

// File names LoadMood expects inside its content directory.
const (
	phq9File       = "phq9_ru.yaml"
	who5File       = "who5_ru.yaml"
	gad7File       = "gad7_ru.yaml"
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

// PhqFuncItem is the official functional-impairment (10th) question of the
// paper form: shown only when at least one of the nine answers is > 0, never
// part of the 0–27 score, surfaced as its own line in the doctor report.
type PhqFuncItem struct {
	Text  string        `yaml:"text"`
	Scale []ScaleOption `yaml:"scale"` // 4 options («Совсем не трудно» … «Чрезвычайно трудно»)
}

// PHQ9 mirrors phq9_ru.yaml: the official Russian PHQ-9 (verbatim) plus the
// canonical severity bands and the functional (10th) item.
type PHQ9 struct {
	Instruction string        `yaml:"instruction_official_ru"`
	Scale       []ScaleOption `yaml:"scale"`
	Items       []PhqItem     `yaml:"items"`
	FuncItem    PhqFuncItem   `yaml:"functional_item"`
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

// AnyPositive reports whether at least one of the nine item answers is > 0 —
// the official gate for the functional (10th) question («Если Вы
// положительно ответили на какие-нибудь пункты…»). Only the first
// len(Items) answers are considered, so a recorded functional answer at
// index 9 never influences the gate.
func (p *PHQ9) AnyPositive(answers []int) bool {
	for i := range p.Items {
		if answerAt(answers, i) > 0 {
			return true
		}
	}
	return false
}

// MoodOffer is one instrument-link offer (a short lead + one start button +
// one decline button).
type MoodOffer struct {
	Body        string `yaml:"body"`          // GAD-7 offer (after the PHQ-9 result)
	BodyLow     string `yaml:"body_low"`      // WHO-5 → PHQ-9 offer, score ≤ 50
	BodyVeryLow string `yaml:"body_very_low"` // WHO-5 → PHQ-9 offer, score ≤ 28 (more insistent)
	StartButton string `yaml:"start_button"`
	LaterButton string `yaml:"later_button"`
}

// MoodModule mirrors mood_module_ru.yaml — the bot's own texts around the
// three instruments: the module menu, the single module-wide consent, the
// crisis card (PHQ-9 only), per-instrument result templates, the two link
// offers, the combined doctor report, and service strings. Everything here
// is original bot content.
type MoodModule struct {
	Meta struct {
		Title      string `yaml:"title"`
		Disclaimer string `yaml:"disclaimer"` // the ONLY caveat users see, kept short
	} `yaml:"meta"`
	Menu struct {
		Prompt           string `yaml:"prompt"`
		Who5Button       string `yaml:"who5_button"`
		Phq9Button       string `yaml:"phq9_button"`
		Gad7Button       string `yaml:"gad7_button"`
		ResumePhq9Button string `yaml:"resume_phq9_button"`
		ResumeWho5Button string `yaml:"resume_who5_button"`
		ResumeGad7Button string `yaml:"resume_gad7_button"`
	} `yaml:"menu"`
	Consent struct {
		Title       string `yaml:"title"`
		Body        string `yaml:"body"`
		AgreeButton string `yaml:"agree_button"`
		LaterButton string `yaml:"later_button"`
		Declined    string `yaml:"declined"`
	} `yaml:"consent"`
	// Resume is the v1 per-run resume gate. Deprecated: the v2 menu carries
	// the per-instrument resume rows; kept only until the journey flow stops
	// referencing it (not validated, absent from the bundled yaml).
	Resume struct {
		Body           string `yaml:"body"`
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
	// Results are the PHQ-9 result texts (historic name kept for the yaml
	// key); WHO-5 and GAD-7 carry their own blocks below.
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
	Who5 struct {
		Title    string `yaml:"title"`
		Progress string `yaml:"progress"` // "Утверждение {current} из {total}" template
		Results  struct {
			ScoreLine       string            `yaml:"score_line"` // "{score}" template
			Bands           map[string]string `yaml:"bands"`      // ok | low | very_low → wording
			AttributionLine string            `yaml:"attribution_line"`
		} `yaml:"results"`
		Offer MoodOffer `yaml:"offer"` // the WHO-5 → PHQ-9 link (body_low / body_very_low)
	} `yaml:"who5"`
	Gad7 struct {
		Title   string `yaml:"title"`
		Results struct {
			ScoreLine       string            `yaml:"score_line"` // "{score}" template
			Bands           map[string]string `yaml:"bands"`      // minimal … severe → wording
			AttributionLine string            `yaml:"attribution_line"`
		} `yaml:"results"`
		Offer MoodOffer `yaml:"offer"` // the PHQ-9 → GAD-7 link (body)
	} `yaml:"gad7"`
	// DoctorReport is the combined report: heading + one line per completed
	// instrument (with dates) + the PHQ-9 facts + a fixed footer.
	DoctorReport struct {
		LeadIn      string `yaml:"lead_in"`
		Heading     string `yaml:"heading"`
		Phq9Line    string `yaml:"phq9_line"` // "{date} {score} {band}" template
		Q9Line      string `yaml:"q9_line"`   // "{q9_fact}" template
		Q9Marked    string `yaml:"q9_marked"`
		Q9NotMarked string `yaml:"q9_not_marked"`
		Q10Line     string `yaml:"q10_line"`  // "{answer}" template (the chosen option label)
		Gad7Line    string `yaml:"gad7_line"` // "{date} {score} {band}" template
		Who5Line    string `yaml:"who5_line"` // "{date} {score} {band}" template
		Footer      string `yaml:"footer"`
		// Template is the v1 single-instrument report. Deprecated: kept only
		// until the journey flow stops referencing it (not validated, absent
		// from the bundled yaml).
		Template string `yaml:"template"`
	} `yaml:"doctor_report"`
	UI struct {
		ModeButton string `yaml:"mode_button"`
		Progress   string `yaml:"progress"` // PHQ-9 / GAD-7 "Вопрос {current} из {total}"
		// Paused is the v1 pause confirmation. Deprecated: kept only until
		// the journey flow stops referencing it (not validated, absent from
		// the bundled yaml).
		Paused              string `yaml:"paused"`
		AbandonConfirmed    string `yaml:"abandon_confirmed"`
		DeleteConfirmPrompt string `yaml:"delete_confirm_prompt"`
		DeleteConfirmButton string `yaml:"delete_confirm_button"`
		DeleteCancelButton  string `yaml:"delete_cancel_button"`
		DeleteDone          string `yaml:"delete_done"`
		DeleteNothing       string `yaml:"delete_nothing"`
	} `yaml:"ui"`
}

// MoodContent is the full mood-module content bundle.
type MoodContent struct {
	PHQ9   PHQ9
	WHO5   WHO5
	GAD7   GAD7
	Module MoodModule
}

// LoadMood reads the four fixed-name content files from dir and validates
// the bundle. Any structural deviation from the canonical shape is an error
// — the bot must fail fast at startup rather than run with wrong instrument
// texts, thresholds, or crisis contacts.
func LoadMood(dir string) (*MoodContent, error) {
	var c MoodContent
	if err := loadYAML(filepath.Join(dir, phq9File), &c.PHQ9); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, who5File), &c.WHO5); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, gad7File), &c.GAD7); err != nil {
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
// canonical values: the three instruments' scales/items/bands (PHQ-9 incl.
// the functional item, WHO-5, GAD-7) and the crisis-contacts guard (adult
// lines only). An error means "this is not the content this code was
// written for" — the caller must refuse to start.
func (c *MoodContent) Validate() error {
	if err := c.validatePHQ9(); err != nil {
		return fmt.Errorf("screening: phq9: %w", err)
	}
	if err := c.validateWHO5(); err != nil {
		return fmt.Errorf("screening: who5: %w", err)
	}
	if err := c.validateGAD7(); err != nil {
		return fmt.Errorf("screening: gad7: %w", err)
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
	if strings.TrimSpace(p.FuncItem.Text) == "" {
		return fmt.Errorf("empty functional_item text")
	}
	if err := validateScale(p.FuncItem.Scale, 4); err != nil {
		return fmt.Errorf("functional_item: %w", err)
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
		"menu.prompt", m.Menu.Prompt,
		"menu.who5_button", m.Menu.Who5Button,
		"menu.phq9_button", m.Menu.Phq9Button,
		"menu.gad7_button", m.Menu.Gad7Button,
		"menu.resume_phq9_button", m.Menu.ResumePhq9Button,
		"menu.resume_who5_button", m.Menu.ResumeWho5Button,
		"menu.resume_gad7_button", m.Menu.ResumeGad7Button,
		"consent.title", m.Consent.Title,
		"consent.body", m.Consent.Body,
		"consent.agree_button", m.Consent.AgreeButton,
		"consent.later_button", m.Consent.LaterButton,
		"consent.declined", m.Consent.Declined,
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
		"who5.title", m.Who5.Title,
		"who5.progress", m.Who5.Progress,
		"who5.results.score_line", m.Who5.Results.ScoreLine,
		"who5.results.attribution_line", m.Who5.Results.AttributionLine,
		"who5.offer.body_low", m.Who5.Offer.BodyLow,
		"who5.offer.body_very_low", m.Who5.Offer.BodyVeryLow,
		"who5.offer.start_button", m.Who5.Offer.StartButton,
		"who5.offer.later_button", m.Who5.Offer.LaterButton,
		"gad7.title", m.Gad7.Title,
		"gad7.results.score_line", m.Gad7.Results.ScoreLine,
		"gad7.results.attribution_line", m.Gad7.Results.AttributionLine,
		"gad7.offer.body", m.Gad7.Offer.Body,
		"gad7.offer.start_button", m.Gad7.Offer.StartButton,
		"gad7.offer.later_button", m.Gad7.Offer.LaterButton,
		"doctor_report.lead_in", m.DoctorReport.LeadIn,
		"doctor_report.heading", m.DoctorReport.Heading,
		"doctor_report.phq9_line", m.DoctorReport.Phq9Line,
		"doctor_report.q9_line", m.DoctorReport.Q9Line,
		"doctor_report.q9_marked", m.DoctorReport.Q9Marked,
		"doctor_report.q9_not_marked", m.DoctorReport.Q9NotMarked,
		"doctor_report.q10_line", m.DoctorReport.Q10Line,
		"doctor_report.gad7_line", m.DoctorReport.Gad7Line,
		"doctor_report.who5_line", m.DoctorReport.Who5Line,
		"doctor_report.footer", m.DoctorReport.Footer,
		"ui.mode_button", m.UI.ModeButton,
		"ui.progress", m.UI.Progress,
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
	for _, band := range []string{Who5BandOK, Who5BandLow, Who5BandVeryLow} {
		if strings.TrimSpace(m.Who5.Results.Bands[band]) == "" {
			return fmt.Errorf("empty who5.results.bands.%s", band)
		}
	}
	for _, band := range []string{BandMinimal, BandMild, BandModerate, BandSevere} {
		if strings.TrimSpace(m.Gad7.Results.Bands[band]) == "" {
			return fmt.Errorf("empty gad7.results.bands.%s", band)
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
	for _, item := range c.WHO5.Items {
		if err := check(fmt.Sprintf("who5 item %d", item.ID), item.Text); err != nil {
			return err
		}
	}
	for _, item := range c.GAD7.Items {
		if err := check(fmt.Sprintf("gad7 item %d", item.ID), item.Text); err != nil {
			return err
		}
	}
	for where, s := range map[string]string{
		"phq9 instruction":     c.PHQ9.Instruction,
		"phq9 functional item": c.PHQ9.FuncItem.Text,
		"phq9 attribution":     c.PHQ9.Attribution,
		"who5 recall header":   c.WHO5.RecallHeader,
		"who5 attribution":     c.WHO5.Attribution,
		"gad7 instruction":     c.GAD7.Instruction,
		"gad7 attribution":     c.GAD7.Attribution,
		"module title":         c.Module.Meta.Title,
		"module menu":          c.Module.Menu.Prompt,
		"module consent":       c.Module.Consent.Body,
		"module disclaimer":    c.Module.Meta.Disclaimer,
		"module crisis":        c.Module.Crisis.Contacts,
	} {
		if err := check(where, s); err != nil {
			return err
		}
	}
	return nil
}
