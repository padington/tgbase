package screening

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests over the REAL bundled content (../../screening/*.yaml). They pin the
// canonical shape and the untouchability of the instrument texts: a diff in
// any of these means someone edited content that must stay verbatim.

// canonicalDir is the bundled content directory relative to this package.
const canonicalDir = "../../screening"

func loadCanonical(t *testing.T) *Content {
	t.Helper()
	c, err := Load(canonicalDir)
	if err != nil {
		t.Fatalf("Load(%s): %v", canonicalDir, err)
	}
	return c
}

// canonicalAsrsQ1 is the first part-A question exactly as published in the
// official Russian WHO/Harvard NCS screener. Byte-for-byte equality guards
// the "official text was not edited" condition of use.
const canonicalAsrsQ1 = "Как часто вам бывает трудно закончить работу после того как самое трудное уже сделано?"

// canonicalAsrsInstructionFirstSentence is the first sentence of the
// official Russian instruction (the second sentence is paper-form-only and
// is deliberately not shown by the bot).
const canonicalAsrsInstructionFirstSentence = "Отметьте клетку с ответом, который лучше всего описывает ваше самочувствие и поведение за последние 6 месяцев."

// canonicalMinScores is the per-item significance ("shaded box") map of the
// ASRS v1.1 checklist, verbatim.
var canonicalMinScores = map[int]int{
	1: 2, 2: 2, 3: 2, 4: 3, 5: 3, 6: 3,
	7: 3, 8: 3, 9: 2, 10: 3, 11: 3, 12: 2, 13: 3, 14: 3, 15: 3, 16: 2, 17: 3, 18: 2,
}

func TestCanonical_ASRS(t *testing.T) {
	c := loadCanonical(t)
	a := &c.ASRS

	if got := len(a.PartA.Items); got != 6 {
		t.Fatalf("part A items: %d", got)
	}
	if got := len(a.PartB.Items); got != 12 {
		t.Fatalf("part B items: %d", got)
	}
	all := append(append([]AsrsItem(nil), a.PartA.Items...), a.PartB.Items...)
	for i, item := range all {
		if item.ID != i+1 {
			t.Errorf("item %d has id %d, want %d", i, item.ID, i+1)
		}
		if want := canonicalMinScores[item.ID]; item.SignificantMinScore != want {
			t.Errorf("item %d significant_min_score = %d, want %d", item.ID, item.SignificantMinScore, want)
		}
	}
	if got := a.PartA.Scoring.PositiveScreenThreshold; got != 4 {
		t.Errorf("part A threshold = %d, want 4", got)
	}

	wantScale := []string{"Никогда", "Редко", "Иногда", "Часто", "Очень часто"}
	if got := len(a.Scale); got != len(wantScale) {
		t.Fatalf("scale size: %d", got)
	}
	for i, opt := range a.Scale {
		if opt.Label != wantScale[i] {
			t.Errorf("scale[%d] = %q, want %q", i, opt.Label, wantScale[i])
		}
	}

	if a.PartA.Items[0].Text != canonicalAsrsQ1 {
		t.Errorf("official part-A question 1 was edited:\ngot  %q\nwant %q",
			a.PartA.Items[0].Text, canonicalAsrsQ1)
	}
	if got := FirstSentence(a.Instruction); got != canonicalAsrsInstructionFirstSentence {
		t.Errorf("instruction first sentence was edited:\ngot  %q\nwant %q",
			got, canonicalAsrsInstructionFirstSentence)
	}
}

func TestCanonical_WURS(t *testing.T) {
	c := loadCanonical(t)
	w := &c.WURS

	if got := len(w.Items); got != 25 {
		t.Fatalf("items: %d", got)
	}
	differs := 0
	for _, item := range w.Items {
		if item.TextM != item.TextF {
			differs++
		}
	}
	if differs == 0 {
		t.Error("text_m and text_f are identical for every item — the two blanks must actually differ")
	}
	if got := w.PrimaryCutoff(); got != 46 {
		t.Errorf("primary cutoff = %d, want 46", got)
	}
	if got := len(w.Scale); got != 5 {
		t.Fatalf("scale size: %d", got)
	}
	if w.Scale[0].Label != "Совсем нет или очень незначительно" || w.Scale[4].Label != "Очень много" {
		t.Errorf("scale labels edited: %q … %q", w.Scale[0].Label, w.Scale[4].Label)
	}
}

func TestCanonical_Module(t *testing.T) {
	c := loadCanonical(t)
	m := &c.Module

	wantIDs := []string{"work_study", "relationships_family", "social", "leisure", "self_esteem"}
	if got := len(m.Domains.Items); got != len(wantIDs) {
		t.Fatalf("domains: %d", got)
	}
	for i, d := range m.Domains.Items {
		if d.ID != wantIDs[i] {
			t.Errorf("domain[%d] id = %q, want %q", i, d.ID, wantIDs[i])
		}
		if strings.TrimSpace(d.Adult.Title) == "" || strings.TrimSpace(d.Childhood.Title) == "" {
			t.Errorf("domain %s: both life-phase titles must be set", d.ID)
		}
	}

	for name, s := range map[string]string{
		"overall.consistent":     m.Results.Overall.Consistent,
		"overall.partial":        m.Results.Overall.Partial,
		"overall.not_consistent": m.Results.Overall.NotConsistent,
	} {
		if strings.TrimSpace(s) == "" {
			t.Errorf("empty results.%s", name)
		}
	}
	for _, hint := range []string{GapNoChildhoodOnset, GapNoCurrentSymptoms, GapFewDomains} {
		if strings.TrimSpace(m.Results.Overall.GapHints[hint]) == "" {
			t.Errorf("empty gap hint %s", hint)
		}
	}
	for name, b := range map[string]InstrumentBlock{
		"asrs_a": m.Results.Instruments.AsrsA,
		"asrs_b": m.Results.Instruments.AsrsB,
		"wurs":   m.Results.Instruments.Wurs,
	} {
		if strings.TrimSpace(b.Title) == "" || strings.TrimSpace(b.ScoreLine) == "" || strings.TrimSpace(b.Attribution) == "" {
			t.Errorf("instrument block %s incomplete", name)
		}
	}
}

// TestNoForbiddenBranding walks the canonical files, every loaded content
// string, and the whole repository (*.go, *.yaml, *.proto, *.md outside
// .git) for the name of the copyrighted third-party interview instrument
// whose foundation forbids chat-bot use. The needle is assembled from bytes
// in content.go so this test's own source does not trip the check.
func TestNoForbiddenBranding(t *testing.T) {
	needle := forbiddenBranding // lowercase ASCII

	// 1. The three canonical files, raw.
	for _, name := range []string{asrsFile, wursFile, moduleFile} {
		raw, err := os.ReadFile(filepath.Join(canonicalDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(raw)), needle) {
			t.Errorf("%s contains forbidden branding", name)
		}
	}

	// 2. The loaded bundle (Validate embeds the same guard — run it anyway).
	c := loadCanonical(t)
	if err := c.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	// 3. Repository-wide walk.
	repoRoot := filepath.Join("..", "..")
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".yaml", ".proto", ".md":
		default:
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(string(raw)), needle) {
			t.Errorf("%s contains forbidden branding", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
