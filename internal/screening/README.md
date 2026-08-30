# internal/screening

Read-only content + pure scoring for the self-check modes: adult ADHD, the
mood module (PHQ-9 depression screening, WHO-5 well-being quick check,
GAD-7 anxiety screening) and the eating track (EDE-QS core screen, BES
binge-eating scale, NIAS restrictive-eating screen) — plus the pushup
track's content and its program engine (the one non-screening bundle here:
it generates load instead of scoring answers).

## Responsibility

Load and validate the screening content YAMLs (repo root `screening/`,
`/screening` in the image), and provide pure, stateless scoring functions.
No persistence, no Telegram, no state — `internal/journey` phases consume
this package via a `*Content` / `*MoodContent` handed in at construction.

Four independent bundles share the same conventions (verbatim / traceable
instrument texts, thresholds pinned in Validate, fail-fast at startup):

- **ADHD** (`Content`, `Load`): `asrs_ru.yaml`, `wurs25_ru.yaml`,
  `dsm_module_ru.yaml`.
- **Mood** (`MoodContent`, `LoadMood`): `phq9_ru.yaml`, `who5_ru.yaml`,
  `gad7_ru.yaml`, `mood_module_ru.yaml`.
- **Eating** (`EatingContent`, `LoadEating`): `edeqs_ru.yaml`, `bes_ru.yaml`,
  `nias_ru.yaml`, `eating_module_ru.yaml`.
- **Pushups** (`PushupContent`, `LoadPushups`): `pushups_ru.yaml` — texts
  **and** the generator parameters, because for this track the parameters
  *are* the content.

## Content provenance (do not edit the instrument texts)

- **ASRS v1.1 part A (questions 1–6)** — the official Russian text of the
  WHO/Harvard NCS 6-question screener, byte-for-byte. Keeping the text
  verbatim is a condition of using the official translation.
- **ASRS v1.1 part B (questions 7–18)** — no official Russian version exists;
  unofficial translation from the official English Symptom Checklist.
- **WURS-25** — no validated Russian adaptation exists; unofficial
  psytests.org translation, cutoffs validated on the English original only.
- **DSM-context module** — written from scratch for this bot (consent, intro,
  onset question, per-domain life questions, result templates). DSM-5
  criterion texts are paraphrased, never quoted (APA copyright).
- **PHQ-9** — the official Russian version («Russian for Russia») from
  phqscreeners.com, byte-for-byte: instruction, all 9 items, the 4-option
  scale, the attribution footer, and (v2) the functional-impairment 10th
  question with its four option labels. Verified against the Wayback Machine
  copies of 2016/2022 (identical digest). The paper-form remark
  «(Ставьте “✔”…)» is deliberately omitted (paper-only). The functional item
  is asked only when at least one answer is > 0 (the form's own instruction)
  and never enters the 0–27 score. PHQ-9 is free to use — reproduction,
  translation, display and distribution are permitted (Pfizer removed all
  restrictions; the permission line is quoted in the YAML).
- **WHO-5** — the official Russian translation from the WHO publication
  WHO-UCN-MSD-MHE-2024.01 (who.int, CC BY-NC-SA 3.0 IGO; translation by the
  Psychiatric Research Unit, WHO Collaborating Centre, Hillerød),
  byte-for-byte: the recall row header, all 5 statements (form punctuation
  preserved), the 6-option scale in the form's top-down order (5 → 0), the
  attribution footer. The paper-bound instruction sentences (circle a digit,
  the example about digit 3 in a square) are omitted — quoted in the YAML
  header for traceability. Verified against the Wayback Machine copy
  (identical md5). Scoring is the form's own: raw 0–25 sum × 4 → 0–100.
  The interpretation bands (> 50 ok / ≤ 50 low / ≤ 28 very low) are the
  bot's product decision on top of the standard ≤ 50 screening cutoff.
- **GAD-7** — the official Russian version («Russian for Russia») from
  phqscreeners.com (same PHQ family, free to use), byte-for-byte:
  instruction, all 7 items, the 4-option scale (same Russian labels as
  PHQ-9), the attribution footer. Verified against the Wayback Machine copy
  (identical md5). Severity bands pinned to Spitzer et al. 2006
  (0–4 / 5–9 / 10–14 / 15–21).
- **Mood module** — written from scratch for this bot (menu, single
  module-wide consent, crisis card, per-instrument result templates, the two
  link offers, combined doctor report). Band wordings are our own and carry
  no diagnosis labels.
- **EDE-QS** — no official Russian version exists; the Russian text is our
  own translation of the English original published as S2 File of Gideon et
  al. 2016 (PLoS ONE, CC BY 4.0; url + md5 in the yaml header — the
  supplementary file itself has no Wayback snapshot, so the article page
  snapshot is pinned next to the digest). Every item keeps its `text_en`
  source string, which the canonical test pins. The paper form's
  Name/Date/**Weight/Height** fields are deliberately NOT reproduced. Both
  answer scales of the original are kept: items 1–10 answer in days, items
  11–12 by severity. Cutoff ≥ 15 (Prnjak et al. 2020).
- **BES** — freely reproduced scale (Gormally et al. 1982); the English
  statements and their weights come from the NIMH Data Archive data
  dictionary `binge01` (url + md5 + Wayback in the yaml), cross-checked
  against the MOH Malaysia obesity CPG appendix for the group/statement
  split. Russian text is our own translation; every statement keeps its
  `text_en`. The uneven original weighting is pinned in `Validate`
  (`canonicalBesWeights`) — that is why the maximum is 46, not 48.
- **NIAS** — English items taken from open-access articles that reproduce
  them verbatim (Biglari et al. 2026 for the item numbering, Koomar et al.
  2021 CC BY as cross-check; urls + md5 in the yaml). Russian text is our
  own translation. Subscale cutoffs ≥ 10 / ≥ 9 / ≥ 10 (Burton Murray et al.
  2021).
- **Eating module** — written from scratch for this bot (menu, single
  track-wide consent, per-instrument result templates, the combined doctor
  report incl. the automatic low-FODMAP context line). Band wordings are our
  own and carry no diagnosis labels.
- **Pushup track** — written from scratch for this bot, texts **and**
  numbers. The scheme (max test → individual load → several sets with an
  open last one → retest) is a widely known training pattern and, as a
  method, is not copyrightable; the *tables* of the commercial programs that
  popularised it are. So this track copies no table: every number a user
  sees is generated by the formulas in `pushups.go` from the parameters in
  `pushups_ru.yaml` (`params:`) and the user's own test result. The
  commercial source's branding and its "N reps in N weeks" promise appear
  nowhere — `TestCanonical_PushupContentHasNoSourceBranding` walks the raw
  file and every user text, and `Validate` carries the same guard so the bot
  refuses to boot on content that leaks it.

**Provenance notes are docs-only** (owner decision): translation-status and
validation caveats live in the instrument YAMLs' non-rendered fields and in
comments, never in user-visible texts. Users see exactly one short
screening-not-a-diagnosis disclaimer (`meta.disclaimer`) and one compact
attribution line (`results.attribution_line`) in the result footer — pinned
by `TestCanonical_NoMethodologyCaveatsInUserTexts`.

Content files are **not** backend-seeded: they are read-only at every boot,
like i18n bundles. Changing wording = changing the file = a redeploy.

## Public API

```go
type Content struct { ASRS ASRS; WURS WURS; Module Module }
func Load(dir string) (*Content, error)   // 3 fixed file names + Validate
func (c *Content) Validate() error        // canonical-shape fail-fast
func FirstSentence(s string) string       // up to and incl. the first period

func (a *ASRS) ScorePartA(answers []int) (significant int, positive bool)
func (a *ASRS) ScorePartB(answers []int) (significant int)   // no threshold by design
func (w *WURS) ScoreWURS(answers []int) (sum int, positive bool)
func (w *WURS) PrimaryCutoff() int        // 46
func OverallVerdict(asrsAPositive, onsetChildhood bool, adultDomainCount int) (verdict, gapHint string)

type MoodContent struct { PHQ9 PHQ9; WHO5 WHO5; GAD7 GAD7; Module MoodModule }
func LoadMood(dir string) (*MoodContent, error)  // 4 fixed file names + Validate
func (c *MoodContent) Validate() error           // canonical-shape fail-fast

func (p *PHQ9) Score(answers []int) int          // sum over the 9 items, 0..27 (q10 never counted)
func (p *PHQ9) Band(score int) string            // severity-band id (Band* consts)
func (p *PHQ9) CrisisAnswer(answers []int) int   // answer to item 9, 0 when unanswered
func (p *PHQ9) AnyPositive(answers []int) bool   // any of the 9 answers > 0 — the q10 gate

func (w *WHO5) Score(answers []int) int          // raw 0..25 sum × 4 → 0..100
func (w *WHO5) Band(score int) string            // Who5BandOK | Who5BandLow | Who5BandVeryLow

func (g *GAD7) Score(answers []int) int          // sum, 0..21
func (g *GAD7) Band(score int) string            // BandMinimal | BandMild | BandModerate | BandSevere

type EatingContent struct { EDEQS EDEQS; BES BES; NIAS NIAS; Module EatingModule }
func LoadEating(dir string) (*EatingContent, error) // 4 fixed file names + Validate
func (c *EatingContent) Validate() error            // canonical-shape fail-fast

func (e *EDEQS) Score(answers []int) int         // sum over the 12 items, 0..36
func (e *EDEQS) Cutoff() int                     // 15 (copied into the stored result)
func (e *EDEQS) Positive(score int) bool         // score >= cutoff
func (e *EDEQS) Band(score int) string           // EdeqsBandBelow | EdeqsBandAtOrAbove
func (e *EDEQS) ScaleFor(i int) []ScaleOption    // days scale for items 1–10, severity for 11–12

func (b *BES) Score(answers []int) int           // sum of the picked statements' weights, 0..46
func (b *BES) Band(score int) string             // BesBandLow | BesBandModerate | BesBandSevere
func (b *BES) MaxScore() int                     // 46 for canonical content

func (n *NIAS) Score(answers []int) NiasScores   // three subscale sums, each 0..15 — no total
func (n *NIAS) Cutoff(subscale string) int       // 10 | 9 | 10
func (n *NIAS) SubscaleOrder() []string          // picky, appetite, fear

// The Burton Murray reading rule, pure and content-free:
func NiasContext(anySubscalePositive, edeqsTaken, edeqsPositive bool) string

type PushupContent struct { /* texts */ ; Params PushupParams }
func LoadPushups(dir string) (*PushupContent, error) // 1 fixed file name + Validate
func (c *PushupContent) Validate() error             // canonical-shape + parameter sanity

// The generator. One number in (the base = the last test result), one
// session out. Pure and deterministic.
func (c *PushupContent) Plan(in PlanInput) PushupPlan
func (p PushupPlan) Fixed() []int        // every set but the open one
func (p PushupPlan) OpenFloor() int      // the open set's minimum ("F+")
func (p PushupPlan) PlannedTotal() int   // fixed sets + the open floor
func (c *PushupContent) RestSeconds(base, dayIdx int, goal string, bonusSec int) int
func (c *PushupContent) EstimateMinutes(plan PushupPlan) int
func (c *PushupContent) Level(base int) int   // cosmetic 1 + base/5, never stored

// Reading a session and a week.
func (c *PushupContent) ClassifySession(targets, actual []int) string // over|plan|short
func (c *PushupContent) ProgressWeek(base int, outcomes []string) PushupWeek
func (c *PushupContent) ClassifyTest(reps int, canStepDown bool) PushupTestVerdict
func (c *PushupContent) RetestNeedsDeload(oldBase, newBase int) bool
func (c *PushupContent) NextEffortAdj(current float64, answer string) float64

// The difficulty ladder (content-driven).
func (c *PushupContent) VariationIndex(id string) int
func (c *PushupContent) Variation(id string) *PushupVariation
func (c *PushupContent) ShiftVariation(id string, steps int) string // clamped
func (c *PushupContent) OfferedVariations() []PushupVariation
```

## Key invariants

- **Thresholds come from the content, not from code constants** — but
  `Validate` pins the canonical values (part-A threshold 4, per-item
  significance thresholds in {2,3}, WURS primary cutoff 46). A file with
  different numbers is treated as foreign content and refused at startup.
- **Life domains are one-short-message-per-domain**: each of the 5 domains
  carries a title and 1..3 short examples per life phase (`Validate`
  enforces the 1..3 bound so the "e.g.:" line stays a single line), plus
  shared position templates and one yes/no question.
- **There is no combined score.** Each instrument is scored separately with
  its own threshold; `OverallVerdict` returns a wording key
  (`consistent|partial|not_consistent` + gap hint), never a number. No
  function returning an "ADHD percentage" may be added — product invariant.
- **ASRS part B has no threshold and no verdict by API** (`ScorePartB`
  returns only the count). The per-item list of strong part-B answers that
  the content's `suggested_bot_output` describes is deliberately NOT
  implemented: it would require persisting per-question answers, which the
  privacy policy forbids.
- Scoring functions are pure and panic-free: short/nil answer slices are
  zero-filled (a programmer error upstream, guarded by tests).
- **PHQ-9 canon**: 4-option scale (0–3), 9 items with the `crisis` flag on
  item 9 only, a 4-option functional item, severity bands pinned to the
  published boundaries (0–4 / 5–9 / 10–14 / 15–19 / 20–27, Kroenke 2001).
  The crisis card's contacts must carry the adult crisis lines and must
  NEVER carry the children's helpline 8-800-2000-122 — `Validate` and the
  canonical tests enforce both.
- **WHO-5 canon**: 6-option scale stored in the form's top-down order
  (scores 5 → 0 — `Validate` pins the order), 5 statements, multiplier 4,
  interpretation bands pinned to 0–28 / 29–50 / 51–100. Since only ×4
  multiples are reachable, the practical boundaries are 28 → very_low and
  48 / 52 around the ≤ 50 cutoff.
- **GAD-7 canon**: 4-option scale (0–3), 7 items, severity bands pinned to
  0–4 / 5–9 / 10–14 / 15–21 (Spitzer 2006). No crisis item — the crisis
  protocol belongs to PHQ-9 only.
- **One consent, separate results**: the module texts carry a single consent
  covering all three instruments; each instrument renders its own result
  with its own score and band — no combined mood index may be added
  (product invariant, same as the ADHD rule).
- **EDE-QS canon**: 12 items, two 4-option scales (days for 1–10, severity
  for 11–12 — `Validate` pins which item uses which), cutoff 15 and the two
  reading bands 0–14 / 15–36. `Positive` is `>= cutoff`, so 14 is negative
  and 15 is positive.
- **BES canon**: 16 statement groups with the ORIGINAL uneven weights
  (`canonicalBesWeights`: repeated weights in groups 1, 3, 4, 7, 13; groups
  4 and 16 top out at 2), maximum sum 46, bands 0–17 / 18–26 / 27–46.
  Answers hold the picked statement's weight, not its position.
- **NIAS canon**: 6-option Likert (0–5), 9 items grouped three per subscale
  in the order picky / appetite / fear, cutoffs 10 / 9 / 10. **There is no
  NIAS total score** — the instrument is read per subscale, and no function
  returning one may be added (product invariant, same family as the "no ADHD
  percentage" and "no combined mood index" rules).
- **The NIAS reading rule lives in code, not in content**: `NiasContext`
  maps (any subscale positive, EDE-QS taken, EDE-QS positive) onto one of
  three wording keys; `Validate` refuses content missing any of the three,
  in the result AND in the doctor report.
- **No figures in eating texts**: none of the three instruments asks for
  weight, height or calorie numbers, and the EDE-QS paper form's
  Weight/Height fields are dropped. Pinned by
  `TestCanonical_EatingUserTextsHaveNoWeightNumbersOrDiagnoses`, which also
  refuses disorder labels in every user-visible string — including the
  doctor report, where "скрин по шкале X положительный" is the allowed
  wording.
- Loaded content must not mention the copyrighted third-party interview
  instrument whose foundation forbids chat-bot use; `Validate` and the
  repository-wide branding test enforce this.
- **Pushups: the base is the only state.** Sets, reps, rest, the weekly step
  — all of it is a function of one integer (the last test result) plus the
  day index and the effort adjustment. There are no columns and therefore no
  coverage holes: `TestPushupPlan_ShapeHoldsForEveryBase` walks every base
  from 3 to 100 across both goals, all three days, the whole effort clamp and
  both ceiling regimes.
- **Pushups: the volume ceiling lives in code, not in a message.** A session
  is capped at `volume_cap_early` × base for the first `early_sessions` and
  `volume_cap` × base after that; the generator drops the second-to-last
  fixed set until the plan fits. The cap may stay unmet only once the plan is
  down to the smallest set count, which happens only in the small-base corner
  (there the whole session is ~10 reps). The test result itself is capped at
  `test_cap`: a bigger number is answered with a harder variation, never with
  a longer set.
- **Pushups: a bigger base never means smaller numbers.** The working number
  and the open set's floor are monotone in the base, and so is the planned
  volume whenever the set count stays put. Volume may step *down* when the
  set count changes — that is the ceiling trading sets for size.
- **Pushups: progression follows the result, not the calendar.** Two or more
  "over" and no "short" raise the base by ≥ `strong_min`; a clean week raises
  it by ≥ `steady_min`; exactly one "short" holds it and does **not** repeat
  the week; two or more repeat the week with a lowered base — and a repeat is
  always announced out loud (`week.repeat` carries both bases).
- **Pushups: no body figures, ever.** The track asks for one number — reps —
  and never for weight, height, calories or a BMI, and it never prints one.
  This is the same product invariant the eating track is pinned to, extended
  to the neighbour: `TestCanonical_PushupTextsHaveNoBodyFigures`. The ladder
  of variations is described in words ("easier"/"harder"); the body-mass
  reasoning behind its order stays in the yaml comments.

## Dependencies

Standard library + `gopkg.in/yaml.v3`. No internal imports — same layer as
`internal/products`.
