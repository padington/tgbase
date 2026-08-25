# internal/screening

Read-only content + pure scoring for the self-check modes: adult ADHD and
mood (PHQ-9 depression screening).

## Responsibility

Load and validate the screening content YAMLs (repo root `screening/`,
`/screening` in the image), and provide pure, stateless scoring functions.
No persistence, no Telegram, no state — `internal/journey` phases consume
this package via a `*Content` / `*MoodContent` handed in at construction.

Two independent bundles share the same conventions (verbatim instrument
texts, thresholds pinned in Validate, fail-fast at startup):

- **ADHD** (`Content`, `Load`): `asrs_ru.yaml`, `wurs25_ru.yaml`,
  `dsm_module_ru.yaml`.
- **Mood** (`MoodContent`, `LoadMood`): `phq9_ru.yaml`, `mood_module_ru.yaml`.

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
  scale, the attribution footer. Verified against the Wayback Machine copies
  of 2016/2022 (identical digest). The paper-form remark «(Ставьте “✔”…)»
  and the functional-impairment follow-up question are deliberately omitted
  (paper-only / not part of the 0–27 score). PHQ-9 is free to use —
  reproduction, translation, display and distribution are permitted (Pfizer
  removed all restrictions; the permission line is quoted in the YAML).
- **Mood module** — written from scratch for this bot (consent, resume,
  crisis card, result templates, doctor report). Severity-band wordings are
  our own and carry no diagnosis labels.

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

type MoodContent struct { PHQ9 PHQ9; Module MoodModule }
func LoadMood(dir string) (*MoodContent, error)  // 2 fixed file names + Validate
func (c *MoodContent) Validate() error           // canonical-shape fail-fast

func (p *PHQ9) Score(answers []int) int          // sum, 0..27
func (p *PHQ9) Band(score int) string            // severity-band id (Band* consts)
func (p *PHQ9) CrisisAnswer(answers []int) int   // answer to item 9, 0 when unanswered
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
  item 9 only, severity bands pinned to the published boundaries
  (0–4 / 5–9 / 10–14 / 15–19 / 20–27, Kroenke 2001). The crisis card's
  contacts must carry the adult crisis lines and must NEVER carry the
  children's helpline 8-800-2000-122 — `Validate` and the canonical tests
  enforce both.
- Loaded content must not mention the copyrighted third-party interview
  instrument whose foundation forbids chat-bot use; `Validate` and the
  repository-wide branding test enforce this.

## Dependencies

Standard library + `gopkg.in/yaml.v3`. No internal imports — same layer as
`internal/products`.
