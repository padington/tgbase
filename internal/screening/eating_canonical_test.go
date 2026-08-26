package screening

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Tests over the REAL bundled eating-track content (../../screening/
// edeqs_ru.yaml + bes_ru.yaml + nias_ru.yaml + eating_module_ru.yaml).
// They pin the canonical shape, the English originals the Russian texts were
// translated from (the yaml `text_en` fields, which carry the provenance),
// the three instruments' thresholds, and the track's own product rules:
// no weight/calorie figures and no disorder labels anywhere a user can see.

func loadCanonicalEating(t *testing.T) *EatingContent {
	t.Helper()
	c, err := LoadEating(canonicalDir)
	if err != nil {
		t.Fatalf("LoadEating(%s): %v", canonicalDir, err)
	}
	return c
}

// The English EDE-QS as published in S2 File of Gideon et al. (2016) —
// the text the Russian translation was made from.
var canonicalEdeqsEN = map[int]string{
	1:  "Have you been deliberately trying to limit the amount of food you eat to influence your weight or shape (whether or not you have succeeded)?",
	9:  "Have you had a sense of having lost control over your eating (at the time that you were eating)?",
	11: "Has your weight or shape influenced how you think about (judge) yourself as a person?",
	12: "How dissatisfied have you been with your weight or shape?",
}

func TestCanonical_EDEQS(t *testing.T) {
	c := loadCanonicalEating(t)
	e := &c.EDEQS

	if got := len(e.Items); got != 12 {
		t.Fatalf("items: %d", got)
	}
	// Two scales, as in the original form: items 1–10 answer in days,
	// items 11–12 by severity.
	wantDays := []string{"0 дней", "1–2 дня", "3–5 дней", "6–7 дней"}
	for i, opt := range e.ScaleDays {
		if opt.Label != wantDays[i] {
			t.Errorf("scale_days[%d] = %q, want %q", i, opt.Label, wantDays[i])
		}
		if opt.Score != i {
			t.Errorf("scale_days[%d] score = %d, want %d", i, opt.Score, i)
		}
	}
	wantSeverity := []string{"Совсем нет", "Немного", "Умеренно", "Сильно"}
	for i, opt := range e.ScaleSeverity {
		if opt.Label != wantSeverity[i] {
			t.Errorf("scale_severity[%d] = %q, want %q", i, opt.Label, wantSeverity[i])
		}
	}
	daysItems := 0
	for _, item := range e.Items {
		if item.Scale == EdeqsScaleDays {
			daysItems++
		}
		if strings.TrimSpace(item.TextEN) == "" {
			t.Errorf("item %d lost its English original (provenance)", item.ID)
		}
	}
	if daysItems != 10 {
		t.Errorf("%d items on the days scale, want 10", daysItems)
	}
	for id, want := range canonicalEdeqsEN {
		if got := e.Items[id-1].TextEN; got != want {
			t.Errorf("English original of item %d was edited:\ngot  %q\nwant %q", id, got, want)
		}
	}

	// The screening cutoff (Prnjak et al. 2020) and its two readings.
	if e.Cutoff() != 15 {
		t.Errorf("cutoff = %d, want 15", e.Cutoff())
	}
	if e.Positive(14) || !e.Positive(15) {
		t.Error("the cutoff must be ≥ 15, not > 15")
	}
	if !strings.Contains(e.Attribution, "Fairburn") {
		t.Errorf("the form's own attribution was edited: %q", e.Attribution)
	}
}

// The English BES statements as published in the NIMH Data Archive data
// dictionary (structure binge01) — the text the Russian translation was made
// from. Two per group are enough to catch an edit of the source column.
var canonicalBesEN = map[[2]int]string{
	{1, 1}:  "I don't feel self-conscious about my weight or body size when I'm with others.",
	{4, 4}:  "I have a strong habit of eating when I'm bored. Nothing seems to help me break the habit.",
	{16, 3}: "Even though I might know how many calories I should eat, I don't have any idea what is a \"normal\" amount of food for me.",
}

func TestCanonical_BES(t *testing.T) {
	c := loadCanonicalEating(t)
	b := &c.BES

	if got := len(b.Items); got != 16 {
		t.Fatalf("items: %d", got)
	}
	// The original weighting is uneven: some groups repeat a weight and two
	// top out at 2, which is why the maximum is 46 and not 48.
	for i, item := range b.Items {
		want := canonicalBesWeights[i]
		if len(item.Statements) != len(want) {
			t.Fatalf("group %d has %d statements, want %d", item.ID, len(item.Statements), len(want))
		}
		for j, st := range item.Statements {
			if st.Score != want[j] {
				t.Errorf("group %d statement %d weight = %d, want %d", item.ID, j+1, st.Score, want[j])
			}
			if strings.TrimSpace(st.TextEN) == "" {
				t.Errorf("group %d statement %d lost its English original (provenance)", item.ID, j+1)
			}
		}
	}
	if got := b.MaxScore(); got != 46 {
		t.Fatalf("MaxScore = %d, want 46", got)
	}
	for key, want := range canonicalBesEN {
		if got := b.Items[key[0]-1].Statements[key[1]-1].TextEN; got != want {
			t.Errorf("English original of group %d statement %d was edited:\ngot  %q\nwant %q",
				key[0], key[1], got, want)
		}
	}

	// Bands: ≤ 17 / 18–26 / ≥ 27.
	for _, tc := range []struct {
		score int
		want  string
	}{{17, BesBandLow}, {18, BesBandModerate}, {26, BesBandModerate}, {27, BesBandSevere}} {
		if got := b.Band(tc.score); got != tc.want {
			t.Errorf("band(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

// The nine English NIAS statements (Zickgraf & Ellis 2018), in the
// instrument's own order — the text the Russian translation was made from.
var canonicalNiasEN = []string{
	"I am a picky eater.",
	"I dislike most of the foods that other people eat.",
	"The list of foods that I like and will eat is shorter than the list of foods I won't eat.",
	"I am not very interested in eating; I seem to have a smaller appetite than other people.",
	"I have to push myself to eat regular meals throughout the day, or to eat a large enough amount of food at meals.",
	"Even when I am eating a food I really like, it is hard for me to eat a large enough volume at meals.",
	"I avoid or put off eating because I am afraid of GI discomfort, choking, or vomiting.",
	"I restrict myself to certain foods because I am afraid that other foods will cause GI discomfort, choking, or vomiting.",
	"I eat small portions because I am afraid of GI discomfort, choking, or vomiting.",
}

func TestCanonical_NIAS(t *testing.T) {
	c := loadCanonicalEating(t)
	n := &c.NIAS

	if got := len(n.Items); got != 9 {
		t.Fatalf("items: %d", got)
	}
	for i, item := range n.Items {
		if got := item.TextEN; got != canonicalNiasEN[i] {
			t.Errorf("English original of item %d was edited:\ngot  %q\nwant %q",
				i+1, got, canonicalNiasEN[i])
		}
		if want := niasSubscaleOrder[i/3]; item.Subscale != want {
			t.Errorf("item %d subscale = %q, want %q", i+1, item.Subscale, want)
		}
	}
	if got := len(n.Scale); got != 6 {
		t.Fatalf("scale size = %d, want 6 (0..5 Likert)", got)
	}
	// Subscale cutoffs (Burton Murray et al. 2021) — the whole reading rule
	// hangs off these three numbers.
	if n.Cutoff(NiasPicky) != 10 || n.Cutoff(NiasAppetite) != 9 || n.Cutoff(NiasFear) != 10 {
		t.Errorf("cutoffs = %d/%d/%d, want 10/9/10",
			n.Cutoff(NiasPicky), n.Cutoff(NiasAppetite), n.Cutoff(NiasFear))
	}
	// No total score exists by design — the instrument is read per subscale.
	scores := n.Score([]int{5, 5, 5, 5, 5, 5, 5, 5, 5})
	if scores.Picky != 15 || scores.Appetite != 15 || scores.Fear != 15 {
		t.Errorf("subscale maxima = %+v, want 15/15/15", scores)
	}
}

func TestCanonical_EatingModule(t *testing.T) {
	c := loadCanonicalEating(t)
	m := &c.Module

	// Each instrument's compact attribution line credits its source.
	for line, want := range map[string]string{
		m.Edeqs.Results.AttributionLine: "EDE-QS",
		m.Bes.Results.AttributionLine:   "Gormally",
		m.Nias.Results.AttributionLine:  "Zickgraf",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("attribution line %q missing %q", line, want)
		}
	}

	// The doctor report carries every placeholder the renderer fills in.
	dr := &m.DoctorReport
	for name, spec := range map[string][]string{
		"edeqs_line": {"{date}", "{score}", "{cutoff}", "{verdict}"},
		"bes_line":   {"{date}", "{score}", "{band}"},
		"nias_line": {"{date}", "{picky}", "{appetite}", "{fear}",
			"{picky_cutoff}", "{appetite_cutoff}", "{fear_cutoff}"},
	} {
		tpl := map[string]string{"edeqs_line": dr.EdeqsLine, "bes_line": dr.BesLine, "nias_line": dr.NiasLine}[name]
		for _, ph := range spec {
			if !strings.Contains(tpl, ph) {
				t.Errorf("doctor_report.%s missing placeholder %s: %q", name, ph, tpl)
			}
		}
	}
	// The automatic FODMAP context line must name the diet and the reason —
	// that is the whole point of adding it for the doctor.
	for _, want := range []string{"low-FODMAP", "СРК"} {
		if !strings.Contains(dr.FodmapContextLine, want) {
			t.Errorf("fodmap_context_line missing %q: %q", want, dr.FodmapContextLine)
		}
	}
	// The soft button wording is a product rule: no disorder word in the menu.
	for _, label := range []string{m.Menu.EdeqsButton, m.Menu.BesButton, m.Menu.NiasButton, m.UI.ModeButton} {
		if strings.Contains(strings.ToLower(label), "расстройств") {
			t.Errorf("menu label %q must stay free of disorder wording", label)
		}
	}
	// The band wordings route the user to a doctor, never to a diagnosis.
	if !strings.Contains(m.Edeqs.Results.Bands[EdeqsBandAtOrAbove], "врач") {
		t.Errorf("edeqs at_or_above wording must point at a doctor: %q",
			m.Edeqs.Results.Bands[EdeqsBandAtOrAbove])
	}
	if !strings.Contains(m.Bes.Results.Bands[BesBandModerate], "врач") {
		t.Errorf("bes moderate wording must point at a doctor: %q", m.Bes.Results.Bands[BesBandModerate])
	}
	if !strings.Contains(m.Bes.Results.Bands[BesBandSevere], "специалист") {
		t.Errorf("bes severe wording must recommend a specialist: %q", m.Bes.Results.Bands[BesBandSevere])
	}
}

// eatingUserTexts collects every string of the bundle a user can actually
// see in the chat: the three instruments' instructions, headers, scale
// labels, items and statements, plus all rendered module texts (menu,
// consent, results, doctor report, service strings).
func eatingUserTexts(c *EatingContent) map[string]string {
	out := map[string]string{}
	add := func(where, s string) { out[where] = s }

	e := &c.EDEQS
	add("edeqs.instruction", e.Instruction)
	add("edeqs.days_header", e.DaysHeader)
	add("edeqs.severity_header", e.SeverityHeader)
	for i, opt := range e.ScaleDays {
		add("edeqs.scale_days."+strconv.Itoa(i), opt.Label)
	}
	for i, opt := range e.ScaleSeverity {
		add("edeqs.scale_severity."+strconv.Itoa(i), opt.Label)
	}
	for _, item := range e.Items {
		add("edeqs.item."+strconv.Itoa(item.ID), item.Text)
	}

	b := &c.BES
	add("bes.instruction", b.Instruction)
	for _, item := range b.Items {
		for i, st := range item.Statements {
			add("bes.item."+strconv.Itoa(item.ID)+".statement."+strconv.Itoa(i+1), st.Text)
		}
	}

	n := &c.NIAS
	add("nias.instruction", n.Instruction)
	for i, opt := range n.Scale {
		add("nias.scale."+strconv.Itoa(i), opt.Label)
	}
	for _, item := range n.Items {
		add("nias.item."+strconv.Itoa(item.ID), item.Text)
	}

	m := &c.Module
	for where, s := range map[string]string{
		"meta.title":                     m.Meta.Title,
		"meta.disclaimer":                m.Meta.Disclaimer,
		"menu.prompt":                    m.Menu.Prompt,
		"menu.edeqs_button":              m.Menu.EdeqsButton,
		"menu.bes_button":                m.Menu.BesButton,
		"menu.nias_button":               m.Menu.NiasButton,
		"menu.resume_edeqs_button":       m.Menu.ResumeEdeqsButton,
		"menu.resume_bes_button":         m.Menu.ResumeBesButton,
		"menu.resume_nias_button":        m.Menu.ResumeNiasButton,
		"consent.title":                  m.Consent.Title,
		"consent.body":                   m.Consent.Body,
		"consent.agree_button":           m.Consent.AgreeButton,
		"consent.later_button":           m.Consent.LaterButton,
		"consent.declined":               m.Consent.Declined,
		"edeqs.title":                    m.Edeqs.Title,
		"edeqs.results.score_line":       m.Edeqs.Results.ScoreLine,
		"edeqs.results.attribution_line": m.Edeqs.Results.AttributionLine,
		"bes.title":                      m.Bes.Title,
		"bes.progress":                   m.Bes.Progress,
		"bes.pick_hint":                  m.Bes.PickHint,
		"bes.results.score_line":         m.Bes.Results.ScoreLine,
		"bes.results.attribution_line":   m.Bes.Results.AttributionLine,
		"nias.title":                     m.Nias.Title,
		"nias.progress":                  m.Nias.Progress,
		"nias.results.subscale_line":     m.Nias.Results.SubscaleLine,
		"nias.results.attribution_line":  m.Nias.Results.AttributionLine,
		"report.lead_in":                 m.DoctorReport.LeadIn,
		"report.heading":                 m.DoctorReport.Heading,
		"report.edeqs_line":              m.DoctorReport.EdeqsLine,
		"report.bes_line":                m.DoctorReport.BesLine,
		"report.nias_line":               m.DoctorReport.NiasLine,
		"report.fodmap_context_line":     m.DoctorReport.FodmapContextLine,
		"report.footer":                  m.DoctorReport.Footer,
		"ui.mode_button":                 m.UI.ModeButton,
		"ui.results_heading":             m.UI.ResultsHeading,
		"ui.progress":                    m.UI.Progress,
		"ui.abandon_confirmed":           m.UI.AbandonConfirmed,
		"ui.delete_confirm_prompt":       m.UI.DeleteConfirmPrompt,
		"ui.delete_confirm_button":       m.UI.DeleteConfirmButton,
		"ui.delete_cancel_button":        m.UI.DeleteCancelButton,
		"ui.delete_done":                 m.UI.DeleteDone,
		"ui.delete_nothing":              m.UI.DeleteNothing,
	} {
		add("module."+where, s)
	}
	for band, s := range m.Edeqs.Results.Bands {
		add("module.edeqs.bands."+band, s)
	}
	for band, s := range m.Bes.Results.Bands {
		add("module.bes.bands."+band, s)
	}
	for id, s := range m.Nias.Subscales {
		add("module.nias.subscales."+id, s)
	}
	for ctx, s := range m.Nias.Results.Contexts {
		add("module.nias.contexts."+ctx, s)
	}
	for ctx, s := range m.DoctorReport.NiasContexts {
		add("module.report.nias_contexts."+ctx, s)
	}
	return out
}

// figureNeedles catch a number glued to a weight or calorie unit. The bare
// words are allowed (two instruments legitimately talk about calories in
// prose) — what the track must never do is put a figure on the screen.
var figureNeedles = regexp.MustCompile(`(?i)\d+\s*(кг|килограмм|ккал|калори|грамм)`)

// TestCanonical_EatingUserTextsHaveNoWeightNumbersOrDiagnoses is the track's
// canary, in the spirit of the ADHD/mood lean-texts guards: none of the
// three instruments asks for a figure (weight, height, calories, BMI), and
// no user-visible text — including the doctor report — carries a disorder
// label. "Скрин по шкале X положительный" is the allowed wording instead.
func TestCanonical_EatingUserTextsHaveNoWeightNumbersOrDiagnoses(t *testing.T) {
	c := loadCanonicalEating(t)

	diagnosisNeedles := []string{
		"анорекси", "булими", "орторекси", "арфид", "arfid",
		"компульсивн", "рпп", "расстройств", "binge eating disorder",
		"имт", "bmi", "индекс массы тела",
	}
	for where, s := range eatingUserTexts(c) {
		if m := figureNeedles.FindString(s); m != "" {
			t.Errorf("%s carries a weight/calorie figure (%q):\n%s", where, m, s)
		}
		low := strings.ToLower(s)
		for _, needle := range diagnosisNeedles {
			if strings.Contains(low, needle) {
				t.Errorf("%s carries a forbidden label (%q):\n%s", where, needle, s)
			}
		}
	}
}

// TestCanonical_NoMethodologyCaveatsInEatingUserTexts extends the owner's
// lean-texts decision to the eating track: users never see translation or
// validation hedging. Same needles as the ADHD/mood guards.
func TestCanonical_NoMethodologyCaveatsInEatingUserTexts(t *testing.T) {
	c := loadCanonicalEating(t)
	for where, s := range eatingUserTexts(c) {
		low := strings.ToLower(s)
		for _, needle := range []string{
			"неофициальн",
			"валидац", "валидир",
			"не существует",
			"психометрическ",
			"нестандартизиров",
			"перевод",
		} {
			if strings.Contains(low, needle) {
				t.Errorf("%s carries methodology hedging (%q):\n%s", where, needle, s)
			}
		}
	}
}
