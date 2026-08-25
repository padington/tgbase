package screening

import (
	"strings"
	"testing"
)

// Tests over the REAL bundled mood content (../../screening/phq9_ru.yaml +
// mood_module_ru.yaml). They pin the canonical shape, the untouchability of
// the official Russian PHQ-9 text, and the deterministic crisis-card
// contract. A diff in any of these means someone edited content that must
// stay verbatim — or broke the crisis protocol.

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

	// The single compact attribution line must credit the three authors.
	for _, want := range []string{"PHQ-9", "Spitzer", "Williams", "Kroenke"} {
		if !strings.Contains(m.Results.AttributionLine, want) {
			t.Errorf("attribution_line missing %q: %q", want, m.Results.AttributionLine)
		}
	}

	// Severity wordings carry no diagnosis labels — screening canon.
	for band, s := range m.Results.Bands {
		if strings.Contains(strings.ToLower(s), "депресс") {
			t.Errorf("band %s wording must not carry a diagnosis label: %q", band, s)
		}
	}

	// Retest guidance is part of the v1 contract.
	if !strings.Contains(m.Results.RetestLine, "2–4") && !strings.Contains(m.Results.RetestLine, "2-4") {
		t.Errorf("retest_line must suggest 2–4 weeks: %q", m.Results.RetestLine)
	}
}

// TestCanonical_NoMethodologyCaveatsInMoodUserTexts extends the owner's
// lean-texts decision to the mood module: users never see methodology
// hedging. Same needles as the ADHD guard.
func TestCanonical_NoMethodologyCaveatsInMoodUserTexts(t *testing.T) {
	c := loadCanonicalMood(t)
	m := &c.Module

	userVisible := map[string]string{
		"meta.disclaimer":          m.Meta.Disclaimer,
		"consent.body":             m.Consent.Body,
		"resume.body":              m.Resume.Body,
		"crisis.lead":              m.Crisis.Lead,
		"crisis.contacts":          m.Crisis.Contacts,
		"crisis.urgent_line":       m.Crisis.UrgentLine,
		"results.heading":          m.Results.Heading,
		"results.score_line":       m.Results.ScoreLine,
		"results.delta_line":       m.Results.DeltaLine,
		"results.retest_line":      m.Results.RetestLine,
		"results.attribution_line": m.Results.AttributionLine,
		"doctor_report.template":   m.DoctorReport.Template,
		"ui.paused":                m.UI.Paused,
	}
	for band, s := range m.Results.Bands {
		userVisible["results.bands."+band] = s
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
