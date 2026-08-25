# internal/screening

Read-only content + pure scoring for the self-check modes: adult ADHD and
the mood module (PHQ-9 depression screening, WHO-5 well-being quick check,
GAD-7 anxiety screening).

## Responsibility

Load and validate the screening content YAMLs (repo root `screening/`,
`/screening` in the image), and provide pure, stateless scoring functions.
No persistence, no Telegram, no state — `internal/journey` phases consume
this package via a `*Content` / `*MoodContent` handed in at construction.

Two independent bundles share the same conventions (verbatim instrument
texts, thresholds pinned in Validate, fail-fast at startup):

- **ADHD** (`Content`, `Load`): `asrs_ru.yaml`, `wurs25_ru.yaml`,
  `dsm_module_ru.yaml`.
- **Mood** (`MoodContent`, `LoadMood`): `phq9_ru.yaml`, `who5_ru.yaml`,
  `gad7_ru.yaml`, `mood_module_ru.yaml`.

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
- Loaded content must not mention the copyrighted third-party interview
  instrument whose foundation forbids chat-bot use; `Validate` and the
  repository-wide branding test enforce this.

## Dependencies

Standard library + `gopkg.in/yaml.v3`. No internal imports — same layer as
`internal/products`.
