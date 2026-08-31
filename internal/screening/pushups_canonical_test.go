package screening

// Tests over the REAL bundled pushup content (../../screening/
// pushups_ru.yaml). They pin the generator's canonical parameters, the
// provenance header, and the track's two product canaries: no body figures
// in any user-visible text, and no trace of the commercial program whose
// tables this track deliberately does not reproduce.

import (
	"os"
	"path/filepath"
	"reflect"
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

// pushupBodyFigureRules are the track's hardest canary: the bot also runs an
// eating-habits screening pinned to "no weight, height, BMI or calorie figure
// anywhere", and a neighbouring track may not break that property. The rules
// are written as patterns rather than as a list of phrases because a phrase
// list only catches the phrasings someone thought of — "твой вес" was caught
// while «Сколько ты весишь?» and «свой вес в килограммах» walked straight
// through.
//
// Cyrillic note: Go's \b is an ASCII word boundary and never fires between
// two Cyrillic letters, so word edges are spelled out as character classes.
var pushupBodyFigureRules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"a figure in body or energy units",
		regexp.MustCompile(`(?i)\d+\s*(кг|килограмм|ккал|калори|грамм|см([^а-яё]|$)|сантиметр)`)},
	{"the weight noun",
		regexp.MustCompile(`(?i)(^|[^а-яёa-z])вес(а|у|ом|е|ы|ов|ам|ами|ах)?([^а-яёa-z]|$)`)},
	{"the verb «весить» / «взвешиваться»",
		regexp.MustCompile(`(?i)(^|[^а-яёa-z])(взвес|весиш|весит|весят|весим|весите|весить|весила|весил)`)},
	{"body mass",
		regexp.MustCompile(`(?i)(масс[аыуой][^.,;!?]{0,12}тела|тела[^.,;!?]{0,8}масс|в килограмм)`)},
	{"BMI",
		regexp.MustCompile(`(?i)((^|[^а-яёa-z])(имт|bmi)([^а-яёa-z]|$)|индекс[а-яё]*\s+масс)`)},
	// Height only in the body sense — the track legitimately talks about
	// growth ("Рост происходит между тренировками").
	{"body height",
		regexp.MustCompile(`(?i)((тво[йя]|ваш|свой|у теб[яе]|при)[^.,;!?]{0,12}рост|рост[^.,;!?]{0,12}(в см|сантиметр|метр))`)},
}

// pushupBodyFigure names the rule a text trips, or "" when it is clean.
func pushupBodyFigure(s string) string {
	for _, rule := range pushupBodyFigureRules {
		if rule.re.MatchString(s) {
			return rule.name
		}
	}
	return ""
}

func TestCanonical_PushupTextsHaveNoBodyFigures(t *testing.T) {
	c := loadCanonicalPushups(t)
	for where, s := range c.userTexts() {
		if rule := pushupBodyFigure(s); rule != "" {
			t.Errorf("%s asks about or names a body figure (%s):\n%s", where, rule, s)
		}
	}
}

// TestCanonical_PushupFigureGuardBites tests the canary itself. A guard that
// silently stops matching is worse than no guard, so the natural ways of
// asking for a body figure are pinned as must-catch, and the track's own
// wording — which talks about GROWTH and about metabolic illness — is pinned
// as must-pass.
func TestCanonical_PushupFigureGuardBites(t *testing.T) {
	for _, s := range []string{
		"Отжимания. Сколько ты весишь?",
		"Отжимания. Напиши свой вес в килограммах",
		"Твой вес?",
		"Ваш вес сегодня",
		"Укажи вес тела",
		"Сколько ты весил в начале?",
		"Взвесься перед тестом",
		"Укажи массу тела",
		"Масса тела нужна для расчёта",
		"Это примерно 70 кг нагрузки",
		"За тренировку сожжёшь 200 ккал",
		"Посчитаю ИМТ",
		"Индекс массы тела в норме?",
		"Какой у тебя рост?",
		"Твой рост в сантиметрах",
		"Рост в см?",
		"Обхват груди 100 см",
	} {
		if pushupBodyFigure(s) == "" {
			t.Errorf("the body-figure guard misses %q", s)
		}
	}

	for _, s := range []string{
		"Сегодня отдых. Рост происходит между тренировками.",
		"Есть болезнь сердца, обмена веществ или почек — или при нагрузке бывает боль в груди?",
		"Подход 2/5 — 8 повторов. Сколько получилось?",
		"Всё равно тренироваться",
		"Вверх — до полного выпрямления рук.",
		"Бери тот, где ровной техникой делаешь 5–15 повторов.",
		"Готово: 36 повторов, открытый подход 12 — на 3 больше цели.",
		"Стоя, руки в стену.",
	} {
		if rule := pushupBodyFigure(s); rule != "" {
			t.Errorf("the body-figure guard false-positives on %q (%s)", s, rule)
		}
	}
}

// pushupUserTextExceptions are the bundle's machine-readable strings: ids and
// the yes-actions of the gate. Everything else in the file is shown to a user
// and therefore has to be walked by the guards.
var pushupUserTextExceptions = map[string]bool{
	"Meta.Module":            true,
	"Meta.Language":          true,
	"Variations.DefaultID":   true,
	"Gate.Questions[].ID":    true,
	"Gate.Questions[].OnYes": true,
	"Variations.Ladder[].ID": true,
}

// collectPushupStrings walks the bundle and returns every string in it,
// keyed by its structural path (slice indices collapsed to "[]").
func collectPushupStrings(v reflect.Value, path string, out map[string][]string) {
	switch v.Kind() {
	case reflect.String:
		out[path] = append(out[path], v.String())
	case reflect.Pointer:
		if !v.IsNil() {
			collectPushupStrings(v.Elem(), path, out)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			collectPushupStrings(v.Index(i), path+"[]", out)
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if !t.Field(i).IsExported() || t.Field(i).Name == "Params" {
				continue
			}
			name := t.Field(i).Name
			if path != "" {
				name = path + "." + name
			}
			collectPushupStrings(v.Field(i), name, out)
		}
	}
}

// TestCanonical_PushupUserTextsCoverTheBundle closes the hole under every
// canary above: userTexts() is written out by hand, so a text added to the
// yaml and to the struct but forgotten there would be invisible to the
// figure, branding and house-style guards — and would fail nothing.
func TestCanonical_PushupUserTextsCoverTheBundle(t *testing.T) {
	c := loadCanonicalPushups(t)

	covered := map[string]bool{}
	for _, s := range c.userTexts() {
		covered[s] = true
	}

	found := map[string][]string{}
	collectPushupStrings(reflect.ValueOf(*c), "", found)
	if len(found) == 0 {
		t.Fatal("the bundle walk found no strings at all")
	}
	for path, values := range found {
		if pushupUserTextExceptions[path] {
			continue
		}
		for _, s := range values {
			if strings.TrimSpace(s) == "" {
				t.Errorf("%s is empty — Validate should have refused the bundle", path)
				continue
			}
			if !covered[s] {
				t.Errorf("%s is not listed in userTexts(), so no canary sees it:\n%s", path, s)
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

	for where, s := range c.userTexts() {
		if rule := pushupPromise(s); rule != "" {
			t.Errorf("%s promises a borrowed result (%s):\n%s", where, rule, s)
		}
	}
}

// pushupPromiseRules catch the two halves of the promise this track refuses
// to make: a round rep count ("сто повторов", "100 подряд") and a deadline
// ("за 6 недель", "за три месяца"). Patterns again, not a phrase list —
// «100 повторов за три месяца» used to pass a list built around "100 отжим".
var pushupPromiseRules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"a round rep count",
		regexp.MustCompile(`(?i)(^|[^а-яёa-z0-9])(\d{3,}|сто|сотн[а-яё]*)\s*[-—]?\s*(отжим|повтор|раз|подряд|push)`)},
	{"a deadline",
		regexp.MustCompile(`(?i)(^|[^а-яёa-z])за\s+(\d+|одн[уи]|дв[еа]|три|четыре|пять|шесть|семь|восемь|девять|десять|пару|несколько)?\s*(недел|месяц)`)},
}

// pushupPromise names the rule a text trips, or "" when it is clean.
func pushupPromise(s string) string {
	for _, rule := range pushupPromiseRules {
		if rule.re.MatchString(s) {
			return rule.name
		}
	}
	return ""
}

// TestCanonical_PushupPromiseGuardBites tests that guard the same way: the
// promise the source program is famous for, in the shapes it is usually
// written in, versus the track's own honest wording.
func TestCanonical_PushupPromiseGuardBites(t *testing.T) {
	for _, s := range []string{
		"100 отжиманий за 6 недель",
		"Сто отжиманий подряд",
		"100 повторов за три месяца",
		"100 подряд — цель трека",
		"Дойдёшь до 100 повторов",
		"Сотня отжиманий за месяц",
		"За шесть недель удвоишь максимум",
		"За 8 недель дойдём до цели",
		"Программа за 2 месяца",
	} {
		if pushupPromise(s) == "" {
			t.Errorf("the promise guard misses %q", s)
		}
	}

	for _, s := range []string{
		"Цель — удвоить свой сегодняшний максимум.",
		"Три тренировки в неделю, примерно по 12 минут.",
		"Раз в месяц сверяемся. Сегодня тест на максимум.",
		"Он же засчитается за третью тренировку недели.",
		"Бери тот, где ровной техникой делаешь 5–15 повторов.",
		"Готово: 36 повторов, открытый подход 12.",
		"Пришли число повторов, например 12.",
	} {
		if rule := pushupPromise(s); rule != "" {
			t.Errorf("the promise guard false-positives on %q (%s)", s, rule)
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
