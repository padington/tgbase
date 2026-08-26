package screening

// Eating-track content (EDE-QS + BES + NIAS): the bundle type, its loader,
// the track's own module texts and the shared validation entry point. Kept
// as a separate bundle from the ADHD Content and the mood MoodContent so
// each track's content has an independent lifecycle — the conventions are
// shared (fixed file names, verbatim/traceable instrument texts, thresholds
// pinned in Validate, fail-fast at startup).
//
// The per-instrument types live next door: EDE-QS in edeqs.go, BES in
// bes.go, NIAS in nias.go (including the Burton Murray reading rule).

import (
	"fmt"
	"path/filepath"
	"strings"
)

// File names LoadEating expects inside its content directory.
const (
	edeqsFile        = "edeqs_ru.yaml"
	besFile          = "bes_ru.yaml"
	niasFile         = "nias_ru.yaml"
	eatingModuleFile = "eating_module_ru.yaml"
)

// EatingModule mirrors eating_module_ru.yaml — the bot's own texts around
// the three instruments: the mini-menu, the single track-wide consent, the
// per-instrument result templates, the combined doctor report (including
// the automatic low-FODMAP context line) and service strings. Everything
// here is original bot content.
type EatingModule struct {
	Meta struct {
		Title      string `yaml:"title"`
		Disclaimer string `yaml:"disclaimer"` // the ONLY caveat users see, kept short
	} `yaml:"meta"`
	Menu struct {
		Prompt            string `yaml:"prompt"`
		EdeqsButton       string `yaml:"edeqs_button"`
		BesButton         string `yaml:"bes_button"`
		NiasButton        string `yaml:"nias_button"`
		ResumeEdeqsButton string `yaml:"resume_edeqs_button"`
		ResumeBesButton   string `yaml:"resume_bes_button"`
		ResumeNiasButton  string `yaml:"resume_nias_button"`
	} `yaml:"menu"`
	Consent struct {
		Title       string `yaml:"title"`
		Body        string `yaml:"body"`
		AgreeButton string `yaml:"agree_button"`
		LaterButton string `yaml:"later_button"`
		Declined    string `yaml:"declined"`
	} `yaml:"consent"`
	Edeqs struct {
		Title   string `yaml:"title"`
		Results struct {
			ScoreLine       string            `yaml:"score_line"` // "{score} {cutoff}" template
			Bands           map[string]string `yaml:"bands"`      // below | at_or_above
			AttributionLine string            `yaml:"attribution_line"`
		} `yaml:"results"`
	} `yaml:"edeqs"`
	Bes struct {
		Title    string `yaml:"title"`
		Progress string `yaml:"progress"`  // "Группа {current} из {total}"
		PickHint string `yaml:"pick_hint"` // how to answer a numbered group
		Results  struct {
			ScoreLine       string            `yaml:"score_line"` // "{score}" template
			Bands           map[string]string `yaml:"bands"`      // low | moderate | severe
			AttributionLine string            `yaml:"attribution_line"`
		} `yaml:"results"`
	} `yaml:"bes"`
	Nias struct {
		Title     string            `yaml:"title"`
		Progress  string            `yaml:"progress"`  // "Утверждение {current} из {total}"
		Subscales map[string]string `yaml:"subscales"` // subscale id → user-facing name
		Results   struct {
			SubscaleLine    string            `yaml:"subscale_line"` // "{name} {score} {cutoff} {verdict}"
			VerdictAbove    string            `yaml:"verdict_above"`
			VerdictBelow    string            `yaml:"verdict_below"`
			Contexts        map[string]string `yaml:"contexts"` // NiasCtx* → user wording
			AttributionLine string            `yaml:"attribution_line"`
		} `yaml:"results"`
	} `yaml:"nias"`
	// DoctorReport is the combined report: heading + one line per completed
	// instrument (with dates) + the NIAS reading + the automatic low-FODMAP
	// context line + a fixed footer.
	DoctorReport struct {
		LeadIn            string            `yaml:"lead_in"`
		Heading           string            `yaml:"heading"`
		EdeqsLine         string            `yaml:"edeqs_line"` // "{date} {score} {cutoff} {verdict}"
		VerdictPositive   string            `yaml:"verdict_positive"`
		VerdictNegative   string            `yaml:"verdict_negative"`
		BesLine           string            `yaml:"bes_line"`  // "{date} {score} {band}"
		NiasLine          string            `yaml:"nias_line"` // "{date} {picky} {appetite} {fear} + cutoffs"
		NiasContexts      map[string]string `yaml:"nias_contexts"`
		FodmapContextLine string            `yaml:"fodmap_context_line"`
		Footer            string            `yaml:"footer"`
	} `yaml:"doctor_report"`
	UI struct {
		ModeButton          string `yaml:"mode_button"`
		ResultsHeading      string `yaml:"results_heading"`
		Progress            string `yaml:"progress"` // EDE-QS "Вопрос {current} из {total}"
		AbandonConfirmed    string `yaml:"abandon_confirmed"`
		DeleteConfirmPrompt string `yaml:"delete_confirm_prompt"`
		DeleteConfirmButton string `yaml:"delete_confirm_button"`
		DeleteCancelButton  string `yaml:"delete_cancel_button"`
		DeleteDone          string `yaml:"delete_done"`
		DeleteNothing       string `yaml:"delete_nothing"`
	} `yaml:"ui"`
}

// EatingContent is the full eating-track content bundle.
type EatingContent struct {
	EDEQS  EDEQS
	BES    BES
	NIAS   NIAS
	Module EatingModule
}

// LoadEating reads the four fixed-name content files from dir and validates
// the bundle. Any structural deviation from the canonical shape is an error
// — the bot must fail fast at startup rather than run with wrong instrument
// texts or thresholds.
func LoadEating(dir string) (*EatingContent, error) {
	var c EatingContent
	if err := loadYAML(filepath.Join(dir, edeqsFile), &c.EDEQS); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, besFile), &c.BES); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, niasFile), &c.NIAS); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, eatingModuleFile), &c.Module); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate checks the loaded bundle against the canonical shape and pins the
// canonical values: the three instruments' scales/items/thresholds and the
// module's mandatory texts. An error means "this is not the content this
// code was written for" — the caller must refuse to start.
func (c *EatingContent) Validate() error {
	if err := c.validateEDEQS(); err != nil {
		return fmt.Errorf("screening: edeqs: %w", err)
	}
	if err := c.validateBES(); err != nil {
		return fmt.Errorf("screening: bes: %w", err)
	}
	if err := c.validateNIAS(); err != nil {
		return fmt.Errorf("screening: nias: %w", err)
	}
	if err := c.validateEatingModule(); err != nil {
		return fmt.Errorf("screening: eating module: %w", err)
	}
	return nil
}

func (c *EatingContent) validateEatingModule() error {
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
		"menu.edeqs_button", m.Menu.EdeqsButton,
		"menu.bes_button", m.Menu.BesButton,
		"menu.nias_button", m.Menu.NiasButton,
		"menu.resume_edeqs_button", m.Menu.ResumeEdeqsButton,
		"menu.resume_bes_button", m.Menu.ResumeBesButton,
		"menu.resume_nias_button", m.Menu.ResumeNiasButton,
		"consent.title", m.Consent.Title,
		"consent.body", m.Consent.Body,
		"consent.agree_button", m.Consent.AgreeButton,
		"consent.later_button", m.Consent.LaterButton,
		"consent.declined", m.Consent.Declined,
		"edeqs.title", m.Edeqs.Title,
		"edeqs.results.score_line", m.Edeqs.Results.ScoreLine,
		"edeqs.results.attribution_line", m.Edeqs.Results.AttributionLine,
		"bes.title", m.Bes.Title,
		"bes.progress", m.Bes.Progress,
		"bes.pick_hint", m.Bes.PickHint,
		"bes.results.score_line", m.Bes.Results.ScoreLine,
		"bes.results.attribution_line", m.Bes.Results.AttributionLine,
		"nias.title", m.Nias.Title,
		"nias.progress", m.Nias.Progress,
		"nias.results.subscale_line", m.Nias.Results.SubscaleLine,
		"nias.results.verdict_above", m.Nias.Results.VerdictAbove,
		"nias.results.verdict_below", m.Nias.Results.VerdictBelow,
		"nias.results.attribution_line", m.Nias.Results.AttributionLine,
		"doctor_report.lead_in", m.DoctorReport.LeadIn,
		"doctor_report.heading", m.DoctorReport.Heading,
		"doctor_report.edeqs_line", m.DoctorReport.EdeqsLine,
		"doctor_report.verdict_positive", m.DoctorReport.VerdictPositive,
		"doctor_report.verdict_negative", m.DoctorReport.VerdictNegative,
		"doctor_report.bes_line", m.DoctorReport.BesLine,
		"doctor_report.nias_line", m.DoctorReport.NiasLine,
		"doctor_report.fodmap_context_line", m.DoctorReport.FodmapContextLine,
		"doctor_report.footer", m.DoctorReport.Footer,
		"ui.mode_button", m.UI.ModeButton,
		"ui.results_heading", m.UI.ResultsHeading,
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

	for _, band := range []string{EdeqsBandBelow, EdeqsBandAtOrAbove} {
		if strings.TrimSpace(m.Edeqs.Results.Bands[band]) == "" {
			return fmt.Errorf("empty edeqs.results.bands.%s", band)
		}
	}
	for _, band := range []string{BesBandLow, BesBandModerate, BesBandSevere} {
		if strings.TrimSpace(m.Bes.Results.Bands[band]) == "" {
			return fmt.Errorf("empty bes.results.bands.%s", band)
		}
	}
	for _, id := range niasSubscaleOrder {
		if strings.TrimSpace(m.Nias.Subscales[id]) == "" {
			return fmt.Errorf("empty nias.subscales.%s", id)
		}
	}
	// Every reading branch of the Burton Murray rule must have a wording,
	// both for the user-facing result and for the doctor report — a missing
	// one would silently drop the very distinction the rule exists for.
	for _, ctx := range []string{
		NiasCtxRestrictiveNoBodyImage,
		NiasCtxRestrictiveBodyImage,
		NiasCtxRestrictiveUnknown,
	} {
		if strings.TrimSpace(m.Nias.Results.Contexts[ctx]) == "" {
			return fmt.Errorf("empty nias.results.contexts.%s", ctx)
		}
		if strings.TrimSpace(m.DoctorReport.NiasContexts[ctx]) == "" {
			return fmt.Errorf("empty doctor_report.nias_contexts.%s", ctx)
		}
	}

	return c.validateEatingBranding()
}

// validateEatingBranding rejects eating content mentioning the forbidden
// third-party instrument, mirroring the ADHD and mood bundles' guard.
func (c *EatingContent) validateEatingBranding() error {
	check := func(where, s string) error {
		if strings.Contains(strings.ToLower(s), forbiddenBranding) {
			return fmt.Errorf("%s contains forbidden instrument branding", where)
		}
		return nil
	}
	for _, item := range c.EDEQS.Items {
		if err := check(fmt.Sprintf("edeqs item %d", item.ID), item.Text); err != nil {
			return err
		}
	}
	for _, item := range c.BES.Items {
		for i, st := range item.Statements {
			if err := check(fmt.Sprintf("bes item %d statement %d", item.ID, i+1), st.Text); err != nil {
				return err
			}
		}
	}
	for _, item := range c.NIAS.Items {
		if err := check(fmt.Sprintf("nias item %d", item.ID), item.Text); err != nil {
			return err
		}
	}
	for where, s := range map[string]string{
		"edeqs instruction":  c.EDEQS.Instruction,
		"bes instruction":    c.BES.Instruction,
		"nias instruction":   c.NIAS.Instruction,
		"module title":       c.Module.Meta.Title,
		"module menu":        c.Module.Menu.Prompt,
		"module consent":     c.Module.Consent.Body,
		"module disclaimer":  c.Module.Meta.Disclaimer,
		"report heading":     c.Module.DoctorReport.Heading,
		"report fodmap line": c.Module.DoctorReport.FodmapContextLine,
	} {
		if err := check(where, s); err != nil {
			return err
		}
	}
	return nil
}
