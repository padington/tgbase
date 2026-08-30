package screening

// Tests over the REAL bundled pushup content (../../screening/
// pushups_ru.yaml). They pin the generator's canonical parameters, the
// provenance header, and the track's two product canaries: no body figures
// in any user-visible text, and no trace of the commercial program whose
// tables this track deliberately does not reproduce.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func loadCanonicalPushups(t *testing.T) *PushupContent {
	t.Helper()
	c, err := LoadPushups(canonicalDir)
	if err != nil {
		t.Fatalf("LoadPushups(%s): %v", canonicalDir, err)
	}
	return c
}

func rawPushupContent(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(canonicalDir, pushupsFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestCanonical_PushupParams pins the numbers the whole program is generated
// from. They are the bot's own choice, not a copy of anyone's table — but
// they are a choice with a safety margin behind it (the volume ceilings, the
// test ceiling, the recovery window), so changing one must be deliberate.
func TestCanonical_PushupParams(t *testing.T) {
	p := &loadCanonicalPushups(t).Params

	for _, tc := range []struct {
		name      string
		got, want float64
	}{
		{"work_share", p.WorkShare, 0.40},
		{"open_share", p.OpenShare, 0.55},
		{"small_base_open_share", p.SmallBaseOpenShare, 0.60},
		{"volume_cap_early", p.VolumeCapEarly, 2.5},
		{"volume_cap", p.VolumeCap, 3.5},
		{"deload_factor", p.DeloadFactor, 0.85},
		{"retest_deload_share", p.RetestDeloadShare, 0.80},
		{"effort hard step", p.EffortAdj.Hard, -0.10},
		{"effort easy step", p.EffortAdj.Easy, 0.10},
		{"effort clamp min", p.EffortAdj.Min, 0.85},
		{"effort clamp max", p.EffortAdj.Max, 1.15},
		{"strong week share", p.Progression.StrongShare, 0.08},
		{"steady week share", p.Progression.SteadyShare, 0.04},
		{"repeat week drop share", p.Progression.DropShare, 0.08},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	for _, tc := range []struct {
		name      string
		got, want int
	}{
		{"min_base", p.MinBase, 3},
		{"too_low_reps", p.TooLowReps, 2},
		{"test_cap", p.TestCap, 60},
		{"max_reps_input", p.MaxRepsInput, 200},
		{"small_base_max", p.SmallBaseMax, 6},
		{"min_set_reps", p.MinSetReps, 2},
		{"small_base_min_set_reps", p.SmallBaseMinSetReps, 1},
		{"open_gap", p.OpenGap, 2},
		{"small_base_open_gap", p.SmallBaseOpenGap, 1},
		{"rest_floor_seconds", p.RestFloorSeconds, 45},
		{"rest_cut_seconds", p.RestCutSeconds, 15},
		{"rest_cut_base", p.RestCutBase, 45},
		{"rest_bonus_step", p.RestBonusStep, 30},
		{"early_sessions", p.EarlySessions, 6},
		{"sessions_per_week", p.SessionsPerWeek, 3},
		{"retest_every_sessions", p.RetestEverySessions, 12},
		{"joint_pain_step_down", p.JointPainStepDown, 2},
		{"min_hours_between", p.MinHoursBetween, 24},
		{"advised_hours_between", p.AdvisedHoursBetween, 48},
		{"pause_retest_days", p.PauseRetestDays, 10},
		{"pause_deload_days", p.PauseDeloadDays, 21},
		{"session_ttl_hours", p.SessionTTLHours, 24},
		{"history_cap", p.HistoryCap, 36},
		{"level_step", p.LevelStep, 5},
		{"level_max", p.LevelMax, 20},
		{"over_margin", p.Progression.OverMargin, 2},
		{"strong_over_count", p.Progression.StrongOverCount, 2},
		{"repeat_short_count", p.Progression.RepeatShortCount, 2},
		{"repeat_fork_after", p.Progression.RepeatForkAfter, 3},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}

	wantDays := []float64{0.85, 1.00, 1.15}
	for i, want := range wantDays {
		if p.DayFactors[i] != want {
			t.Errorf("day_factors[%d] = %v, want %v", i, p.DayFactors[i], want)
		}
	}
	wantRest := []int{60, 90, 120}
	for i, want := range wantRest {
		if p.RestSeconds[i] != want {
			t.Errorf("rest_seconds[%d] = %d, want %d", i, p.RestSeconds[i], want)
		}
	}
	wantThresholds := []int{7, 25, 45}
	for i, want := range wantThresholds {
		if p.SetCountThresholds[i] != want {
			t.Errorf("set_count_thresholds[%d] = %d, want %d", i, p.SetCountThresholds[i], want)
		}
	}
	// Four to seven sets, the open one included — the last set is always the
	// open one, so a template has n-1 offsets.
	wantCounts := []int{4, 5, 6, 7}
	for i, want := range wantCounts {
		if p.SetCounts[i] != want {
			t.Errorf("set_counts[%d] = %d, want %d", i, p.SetCounts[i], want)
		}
	}
	for _, n := range wantCounts {
		offsets := p.SetOffsets[n]
		if len(offsets) != n-1 {
			t.Fatalf("set_offsets[%d] has %d entries, want %d", n, len(offsets), n-1)
		}
		if offsets[0] != 0 || offsets[1] != 1 {
			t.Errorf("set_offsets[%d] starts with %v, want the second set to be the heaviest fixed one", n, offsets[:2])
		}
	}
}

// TestCanonical_PushupProvenanceHeader keeps the licence-critical statement
// in the file: the numbers are generated, no table is copied. The header is
// the first thing a contributor reads before editing the parameters.
func TestCanonical_PushupProvenanceHeader(t *testing.T) {
	low := strings.ToLower(rawPushupContent(t))
	for _, needle := range []string{
		"порождаются формулой",
		"ни одной заимствованной таблицы",
		"собственная реализация",
	} {
		if !strings.Contains(low, needle) {
			t.Errorf("the provenance header lost %q", needle)
		}
	}
}

// pushupFigureNeedles catch a number glued to a body/energy unit. The track
// generates load from ONE number — the test result — and must never ask for
// or print weight, height or calories: the bot also runs an eating-habits
// screening track pinned to exactly that property, and a neighbouring track
// may not break it.
var pushupFigureNeedles = regexp.MustCompile(`(?i)\d+\s*(кг|килограмм|ккал|калори|грамм|см\b|сантиметр)`)

func TestCanonical_PushupTextsHaveNoBodyFigures(t *testing.T) {
	c := loadCanonicalPushups(t)

	forbidden := []string{
		"имт", "bmi", "индекс массы тела", "массы тела", "твой вес", "ваш вес",
		"вес тела", "взвес", "твой рост", "ваш рост", "калорий",
	}
	for where, s := range c.userTexts() {
		if m := pushupFigureNeedles.FindString(s); m != "" {
			t.Errorf("%s carries a body/energy figure (%q):\n%s", where, m, s)
		}
		low := strings.ToLower(s)
		for _, needle := range forbidden {
			if strings.Contains(low, needle) {
				t.Errorf("%s asks about or names a body figure (%q):\n%s", where, needle, s)
			}
		}
	}
}

// TestCanonical_PushupContentHasNoSourceBranding is the licence canary. The
// scheme is ours; the commercial program that popularised it is not
// mentioned anywhere in the file (branding, its promise, its numbers), and
// no text promises a fixed result in a fixed number of weeks.
func TestCanonical_PushupContentHasNoSourceBranding(t *testing.T) {
	c := loadCanonicalPushups(t)
	raw := strings.ToLower(rawPushupContent(t))

	// Latin branding: the whole file, comments included.
	for _, needle := range pushupSourceBranding {
		if strings.Contains(raw, needle) {
			t.Errorf("%s names the commercial source program (%q)", pushupsFile, needle)
		}
	}
	if strings.Contains(raw, forbiddenBranding) {
		t.Errorf("%s contains forbidden instrument branding", pushupsFile)
	}

	// Promises borrowed from that program: a round number of reps, or a
	// deadline in weeks. Neither may appear in a user-visible text.
	promises := []string{
		"100 отжим", "сто отжим", "100 подряд", "100 push",
		"за 6 недель", "за шесть недель", "за 8 недель", "за месяц сможешь",
	}
	for where, s := range c.userTexts() {
		low := strings.ToLower(s)
		for _, needle := range promises {
			if strings.Contains(low, needle) {
				t.Errorf("%s promises a borrowed result (%q):\n%s", where, needle, s)
			}
		}
	}
}

// TestCanonical_PushupTextsAreLeanAndPlain is the house-style guard: short
// messages, no methodology hedging, no study citations. The reasoning behind
// the numbers belongs in the yaml comments, never on the user's screen.
func TestCanonical_PushupTextsAreLeanAndPlain(t *testing.T) {
	c := loadCanonicalPushups(t)

	hedging := []string{
		"исследован", "мета-анализ", "рандомизирован", "доказательн",
		"валидац", "методолог", "по данным наук", "эффективность программы",
		"до отказа", // the track deliberately never uses failure wording
	}
	for where, s := range c.userTexts() {
		low := strings.ToLower(s)
		for _, needle := range hedging {
			if strings.Contains(low, needle) {
				t.Errorf("%s carries methodology hedging (%q):\n%s", where, needle, s)
			}
		}
		limit := 200
		if where == "consent.body" {
			limit = 400 // the one long text: what the track is, up front
		}
		if n := utf8.RuneCountInString(s); n > limit {
			t.Errorf("%s is %d characters, above the %d-character budget:\n%s", where, n, limit, s)
		}
	}
}

// TestCanonical_PushupSafetyTexts pins what the track must SAY, not only
// what it must not: the gate stops the track on the health question, the
// pain answer offers an easier rung instead of "push through", the red-flag
// card sends the user to a doctor, and the repeated week is announced out
// loud with the base change in it.
func TestCanonical_PushupSafetyTexts(t *testing.T) {
	c := loadCanonicalPushups(t)

	if !strings.Contains(strings.ToLower(c.Gate.StopScreen), "врач") {
		t.Errorf("the gate stop screen must send the user to a doctor: %q", c.Gate.StopScreen)
	}
	if !strings.Contains(strings.ToLower(c.Gate.DoctorNote), "врач") {
		t.Errorf("the pregnancy note must say to check with a doctor: %q", c.Gate.DoctorNote)
	}
	if !strings.Contains(strings.ToLower(c.RedCard.Body), "врач") {
		t.Errorf("the red-flag card must send the user to a doctor: %q", c.RedCard.Body)
	}
	if !strings.Contains(strings.ToLower(c.Gate.EasierNote), "боль") {
		t.Errorf("the pain note must name pain as a stop signal: %q", c.Gate.EasierNote)
	}
	// The repeat is always explained, with both bases in the line.
	for _, ph := range []string{"{old}", "{new}"} {
		if !strings.Contains(c.Week.Repeat, ph) {
			t.Errorf("the repeat line must show %s: %q", ph, c.Week.Repeat)
		}
	}
	// The open set is described as "until form breaks", not "to failure".
	if !strings.Contains(strings.ToLower(c.Session.OpenPrompt), "техник") {
		t.Errorf("the open-set prompt must talk about form: %q", c.Session.OpenPrompt)
	}
	// The rest-day line explains why, in one sentence.
	if !strings.Contains(strings.ToLower(c.Session.RestDay), "отдых") {
		t.Errorf("the rest-day line is off: %q", c.Session.RestDay)
	}
}
