package screening

import (
	"strings"
	"testing"
)

// Tests over the REAL bundled mood content (../../screening/phq9_ru.yaml +
// who5_ru.yaml + gad7_ru.yaml + mood_module_ru.yaml). They pin the canonical
// shape, the untouchability of the official Russian instrument texts
// (PHQ-9 incl. the functional item, WHO-5, GAD-7), and the deterministic
// crisis-card contract. A diff in any of these means someone edited content
// that must stay verbatim — or broke the crisis protocol.

func loadCanonicalMood(t *testing.T) *MoodContent {
	t.Helper()
	c, err := LoadMood(canonicalDir)
	if err != nil {
		t.Fatalf("LoadMood(%s): %v", canonicalDir, err)
	}
	return c
}

// The official Russian PHQ-9 ("Russian for Russia", phqscreeners.com),
// byte-for-byte. Keeping the text verbatim is the whole point of using the
// official translation.
const (
	canonicalPhqInstruction = "Как часто за последние 2 недели Вас беспокоили следующие проблемы?"
	canonicalPhqQ1          = "Вам не хотелось ничего делать"
	canonicalPhqQ9          = "Вас посещали мысли о том, что Вам лучше было бы умереть, или о том, чтобы причинить себе какой-нибудь вред"
)

func TestCanonical_PHQ9(t *testing.T) {
	c := loadCanonicalMood(t)
	p := &c.PHQ9

	if got := len(p.Items); got != 9 {
		t.Fatalf("items: %d", got)
	}
	for i, item := range p.Items {
		if item.ID != i+1 {
			t.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
	}

	// Official scale labels, verbatim (note: the official Russian rendering
	// of "More than half the days" is «Более недели»).
	wantScale := []string{"Ни разу", "Несколько дней", "Более недели", "Почти каждый день"}
	if got := len(p.Scale); got != len(wantScale) {
		t.Fatalf("scale size: %d", got)
	}
	for i, opt := range p.Scale {
		if opt.Label != wantScale[i] {
			t.Errorf("scale[%d] = %q, want %q", i, opt.Label, wantScale[i])
		}
		if opt.Score != i {
			t.Errorf("scale[%d] score = %d, want %d", i, opt.Score, i)
		}
	}

	if p.Instruction != canonicalPhqInstruction {
		t.Errorf("official instruction was edited:\ngot  %q\nwant %q",
			p.Instruction, canonicalPhqInstruction)
	}
	if p.Items[0].Text != canonicalPhqQ1 {
		t.Errorf("official question 1 was edited:\ngot  %q\nwant %q",
			p.Items[0].Text, canonicalPhqQ1)
	}
	if p.Items[8].Text != canonicalPhqQ9 {
		t.Errorf("official question 9 was edited:\ngot  %q\nwant %q",
			p.Items[8].Text, canonicalPhqQ9)
	}
	if !p.Items[8].Crisis {
		t.Error("item 9 must carry the crisis flag")
	}

	wantBands := []SeverityBand{
		{0, 4, BandMinimal}, {5, 9, BandMild}, {10, 14, BandModerate},
		{15, 19, BandModeratelySevere}, {20, 27, BandSevere},
	}
	if got := len(p.Scoring.Bands); got != len(wantBands) {
		t.Fatalf("bands: %d", got)
	}
	for i, want := range wantBands {
		if p.Scoring.Bands[i] != want {
			t.Errorf("band %d = %+v, want %+v", i, p.Scoring.Bands[i], want)
		}
	}

	for _, want := range []string{"Spitzer", "Williams", "Kroenke", "Pfizer"} {
		if !strings.Contains(p.Attribution, want) {
			t.Errorf("attribution missing %q: %q", want, p.Attribution)
		}
	}
}

// The official functional (10th) question of the Russian PHQ-9 form,
// byte-for-byte (folded to one line, as the paper line breaks are layout).
const canonicalPhqQ10 = "Если Вы положительно ответили на какие-нибудь пункты, то оцените, насколько трудно Вам было работать, заниматься домашними делами или общаться с людьми из-за этих проблем?"

// TestCanonical_PHQ9FunctionalItem pins the verbatim 10th question and its
// four official option labels.
func TestCanonical_PHQ9FunctionalItem(t *testing.T) {
	c := loadCanonicalMood(t)
	f := &c.PHQ9.FuncItem

	if f.Text != canonicalPhqQ10 {
		t.Errorf("official functional question was edited:\ngot  %q\nwant %q", f.Text, canonicalPhqQ10)
	}
	wantScale := []string{"Совсем не трудно", "Немного трудно", "Очень трудно", "Чрезвычайно трудно"}
	if got := len(f.Scale); got != len(wantScale) {
		t.Fatalf("functional scale size: %d", got)
	}
	for i, opt := range f.Scale {
		if opt.Label != wantScale[i] {
			t.Errorf("functional scale[%d] = %q, want %q", i, opt.Label, wantScale[i])
		}
		if opt.Score != i {
			t.Errorf("functional scale[%d] score = %d, want %d", i, opt.Score, i)
		}
	}
}

// The official Russian WHO-5 (publication WHO-UCN-MSD-MHE-2024.01),
// byte-for-byte: the recall row header, all five statements (punctuation of
// the form preserved, including the missing period on statement 1), and the
// six scale labels in the form's top-down order.
var canonicalWho5Items = []string{
	"Я чувствую себя бодрой(-ым) и в хорошем настроении",
	"Я чувствую себя спокойной(-ым) и раскованной(-ым).",
	"Я чувствую себя активной(-ым) и энергичной(-ым).",
	"Я просыпаюсь и чувствую себя свежей(-им) и отдохнувшей(-им).",
	"Каждый день со мной происходят вещи, представляющие для меня интерес.",
}

func TestCanonical_WHO5(t *testing.T) {
	c := loadCanonicalMood(t)
	w := &c.WHO5

	if w.RecallHeader != "Последние две недели" {
		t.Errorf("recall header edited: %q", w.RecallHeader)
	}

	wantScale := []string{
		"Все время", "Большую часть времени", "Более половины времени",
		"Менее половины времени", "Некоторое время", "Никогда",
	}
	if got := len(w.Scale); got != len(wantScale) {
		t.Fatalf("scale size: %d", got)
	}
	for i, opt := range w.Scale {
		if opt.Label != wantScale[i] {
			t.Errorf("scale[%d] = %q, want %q", i, opt.Label, wantScale[i])
		}
		if opt.Score != 5-i {
			t.Errorf("scale[%d] score = %d, want %d (form order 5 → 0)", i, opt.Score, 5-i)
		}
	}

	if got := len(w.Items); got != len(canonicalWho5Items) {
		t.Fatalf("items: %d", got)
	}
	for i, item := range w.Items {
		if item.Text != canonicalWho5Items[i] {
			t.Errorf("official statement %d was edited:\ngot  %q\nwant %q",
				i+1, item.Text, canonicalWho5Items[i])
		}
	}

	if w.Scoring.Multiplier != 4 {
		t.Errorf("multiplier = %d, want 4", w.Scoring.Multiplier)
	}
	wantBands := []SeverityBand{
		{0, 28, Who5BandVeryLow}, {29, 50, Who5BandLow}, {51, 100, Who5BandOK},
	}
	for i, want := range wantBands {
		if w.Scoring.Bands[i] != want {
			t.Errorf("band %d = %+v, want %+v", i, w.Scoring.Bands[i], want)
		}
	}

	// The verbatim PDF footer credits the WHO collaborating centre.
	if !strings.Contains(w.Attribution, "Psychiatric Research Unit") ||
		!strings.Contains(w.Attribution, "Hillerød") {
		t.Errorf("attribution edited: %q", w.Attribution)
	}
}

// The official Russian GAD-7 (phqscreeners.com), byte-for-byte.
const canonicalGad7Instruction = "Как часто Вас за последние 14 дней беспокоили следующие проблемы?"

var canonicalGad7Items = []string{
	"Вы нервничали, тревожились или испытывали сильный стресс",
	"Вы были неспособны успокоиться или контролировать свое волнение",
	"Вы слишком сильно волновались по различным поводам",
	"Вам было трудно расслабиться",
	"Вы были настолько суетливы, что Вам было тяжело усидеть на месте",
	"Вы легко злились или раздражались",
	"Вы испытывали страх, словно должно произойти нечто ужасное",
}

func TestCanonical_GAD7(t *testing.T) {
	c := loadCanonicalMood(t)
	g := &c.GAD7

	if g.Instruction != canonicalGad7Instruction {
		t.Errorf("official instruction was edited:\ngot  %q\nwant %q",
			g.Instruction, canonicalGad7Instruction)
	}

	// Same official Russian labels as the PHQ-9 of the same family.
	wantScale := []string{"Ни разу", "Несколько дней", "Более недели", "Почти каждый день"}
	for i, opt := range g.Scale {
		if opt.Label != wantScale[i] {
			t.Errorf("scale[%d] = %q, want %q", i, opt.Label, wantScale[i])
		}
	}

	if got := len(g.Items); got != len(canonicalGad7Items) {
		t.Fatalf("items: %d", got)
	}
	for i, item := range g.Items {
		if item.Text != canonicalGad7Items[i] {
			t.Errorf("official question %d was edited:\ngot  %q\nwant %q",
				i+1, item.Text, canonicalGad7Items[i])
		}
	}

	wantBands := []SeverityBand{
		{0, 4, BandMinimal}, {5, 9, BandMild}, {10, 14, BandModerate}, {15, 21, BandSevere},
	}
	if got := len(g.Scoring.Bands); got != len(wantBands) {
		t.Fatalf("bands: %d", got)
	}
	for i, want := range wantBands {
		if g.Scoring.Bands[i] != want {
			t.Errorf("band %d = %+v, want %+v", i, g.Scoring.Bands[i], want)
		}
	}

	for _, want := range []string{"Spitzer", "Williams", "Kroenke", "Pfizer"} {
		if !strings.Contains(g.Attribution, want) {
			t.Errorf("attribution missing %q: %q", want, g.Attribution)
		}
	}
}

// TestCanonical_MoodCrisisCard pins the deterministic crisis-card content:
// the adult crisis lines the card must offer, and the children's helpline it
// must never offer.
func TestCanonical_MoodCrisisCard(t *testing.T) {
	c := loadCanonicalMood(t)
	contacts := c.Module.Crisis.Contacts

	for _, want := range []string{
		"+7 495 989-50-50",  // МЧС, экстренная психологическая помощь
		"051",               // Москва, с городского
		"+7 495 051",        // Москва, с мобильного
		"8-800-250-18-59",   // Красный Крест
		"112",               // угроза жизни
		"findahelpline.com", // не в России
	} {
		if !strings.Contains(contacts, want) {
			t.Errorf("crisis contacts missing %q:\n%s", want, contacts)
		}
	}
	if strings.Contains(contacts, "8-800-2000-122") {
		t.Errorf("crisis contacts must not carry the children's helpline:\n%s", contacts)
	}
	if strings.TrimSpace(c.Module.Crisis.UrgentLine) == "" {
		t.Error("empty crisis.urgent_line")
	}
	if strings.TrimSpace(c.Module.Crisis.ContinueButton) == "" {
		t.Error("empty crisis.continue_button — the test must not be blocked by the card")
	}
}

func TestCanonical_MoodModule(t *testing.T) {
	c := loadCanonicalMood(t)
	m := &c.Module

	// Each instrument's compact attribution line must credit its source.
	for _, want := range []string{"PHQ-9", "Spitzer", "Williams", "Kroenke"} {
		if !strings.Contains(m.Results.AttributionLine, want) {
			t.Errorf("attribution_line missing %q: %q", want, m.Results.AttributionLine)
		}
	}
	for _, want := range []string{"WHO-5", "©"} {
		if !strings.Contains(m.Who5.Results.AttributionLine, want) {
			t.Errorf("who5 attribution_line missing %q: %q", want, m.Who5.Results.AttributionLine)
		}
	}
	for _, want := range []string{"GAD-7", "Spitzer"} {
		if !strings.Contains(m.Gad7.Results.AttributionLine, want) {
			t.Errorf("gad7 attribution_line missing %q: %q", want, m.Gad7.Results.AttributionLine)
		}
	}

	// Severity wordings carry no diagnosis labels — screening canon.
	for band, s := range m.Results.Bands {
		if strings.Contains(strings.ToLower(s), "депресс") {
			t.Errorf("band %s wording must not carry a diagnosis label: %q", band, s)
		}
	}
	for band, s := range m.Gad7.Results.Bands {
		low := strings.ToLower(s)
		if strings.Contains(low, "депресс") || strings.Contains(low, "расстройств") {
			t.Errorf("gad7 band %s wording must not carry a diagnosis label: %q", band, s)
		}
	}

	// Retest guidance is part of the contract.
	if !strings.Contains(m.Results.RetestLine, "2–4") && !strings.Contains(m.Results.RetestLine, "2-4") {
		t.Errorf("retest_line must suggest 2–4 weeks: %q", m.Results.RetestLine)
	}

	// The GAD-7 gradations of the owner's spec: moderate routes to a doctor
	// conversation, severe to a specialist.
	if !strings.Contains(m.Gad7.Results.Bands["moderate"], "врач") {
		t.Errorf("gad7 moderate wording must suggest discussing with a doctor: %q",
			m.Gad7.Results.Bands["moderate"])
	}
	if !strings.Contains(m.Gad7.Results.Bands["severe"], "специалист") {
		t.Errorf("gad7 severe wording must recommend a specialist: %q",
			m.Gad7.Results.Bands["severe"])
	}

	// The WHO-5 link: ≤ 50 offers the PHQ-9, ≤ 28 offers it more insistently
	// — two distinct texts plus one start button.
	if m.Who5.Offer.BodyLow == m.Who5.Offer.BodyVeryLow {
		t.Error("who5 offer must have distinct low / very-low wordings")
	}
	if !strings.Contains(m.Who5.Offer.StartButton, "PHQ") && !strings.Contains(m.Who5.Offer.StartButton, "настроени") {
		t.Errorf("who5 offer start button must name the PHQ-9 mood test: %q", m.Who5.Offer.StartButton)
	}

	// The PHQ-9 → GAD-7 link stays free of clinical terms («часто идут
	// вместе» tone): no diagnosis words in the offer body.
	for _, needle := range []string{"депресс", "расстройств", "коморбид"} {
		if strings.Contains(strings.ToLower(m.Gad7.Offer.Body), needle) {
			t.Errorf("gad7 offer body must stay free of clinical terms (%q): %q", needle, m.Gad7.Offer.Body)
		}
	}

	// The combined doctor report covers all three instruments plus the two
	// PHQ-9 facts, each with a date placeholder where applicable.
	dr := &m.DoctorReport
	for name, tpl := range map[string]string{
		"phq9_line": dr.Phq9Line, "gad7_line": dr.Gad7Line, "who5_line": dr.Who5Line,
	} {
		for _, ph := range []string{"{date}", "{score}", "{band}"} {
			if !strings.Contains(tpl, ph) {
				t.Errorf("doctor_report.%s missing placeholder %s: %q", name, ph, tpl)
			}
		}
	}
	if !strings.Contains(dr.Q9Line, "{q9_fact}") {
		t.Errorf("doctor_report.q9_line missing {q9_fact}: %q", dr.Q9Line)
	}
	if !strings.Contains(dr.Q10Line, "{answer}") {
		t.Errorf("doctor_report.q10_line missing {answer}: %q", dr.Q10Line)
	}
}

// TestCanonical_NoMethodologyCaveatsInMoodUserTexts extends the owner's
// lean-texts decision to the mood module: users never see methodology
// hedging. Same needles as the ADHD guard.
func TestCanonical_NoMethodologyCaveatsInMoodUserTexts(t *testing.T) {
	c := loadCanonicalMood(t)
	m := &c.Module

	userVisible := map[string]string{
		"meta.disclaimer":               m.Meta.Disclaimer,
		"menu.prompt":                   m.Menu.Prompt,
		"consent.body":                  m.Consent.Body,
		"crisis.lead":                   m.Crisis.Lead,
		"crisis.contacts":               m.Crisis.Contacts,
		"crisis.urgent_line":            m.Crisis.UrgentLine,
		"results.heading":               m.Results.Heading,
		"results.score_line":            m.Results.ScoreLine,
		"results.delta_line":            m.Results.DeltaLine,
		"results.retest_line":           m.Results.RetestLine,
		"results.attribution_line":      m.Results.AttributionLine,
		"who5.results.score_line":       m.Who5.Results.ScoreLine,
		"who5.results.attribution_line": m.Who5.Results.AttributionLine,
		"who5.offer.body_low":           m.Who5.Offer.BodyLow,
		"who5.offer.body_very_low":      m.Who5.Offer.BodyVeryLow,
		"gad7.results.score_line":       m.Gad7.Results.ScoreLine,
		"gad7.results.attribution_line": m.Gad7.Results.AttributionLine,
		"gad7.offer.body":               m.Gad7.Offer.Body,
		"doctor_report.heading":         m.DoctorReport.Heading,
		"doctor_report.footer":          m.DoctorReport.Footer,
	}
	for band, s := range m.Results.Bands {
		userVisible["results.bands."+band] = s
	}
	for band, s := range m.Who5.Results.Bands {
		userVisible["who5.results.bands."+band] = s
	}
	for band, s := range m.Gad7.Results.Bands {
		userVisible["gad7.results.bands."+band] = s
	}
	for where, s := range userVisible {
		low := strings.ToLower(s)
		for _, needle := range []string{
			"неофициальн",
			"валидац", "валидир",
			"не существует",
			"психометрическ",
			"нестандартизиров",
		} {
			if strings.Contains(low, needle) {
				t.Errorf("%s carries methodology hedging (%q):\n%s", where, needle, s)
			}
		}
	}
}
