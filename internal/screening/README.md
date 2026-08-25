# internal/screening

Read-only content + pure scoring for the adult ADHD self-check mode.

## Responsibility

Load and validate the three screening content YAMLs (`screening/asrs_ru.yaml`,
`screening/wurs25_ru.yaml`, `screening/dsm_module_ru.yaml` in the repo root,
`/screening` in the image), and provide pure, stateless scoring functions.
No persistence, no Telegram, no state — `internal/journey` phases consume
this package via a `*Content` handed in at construction.

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
- Loaded content must not mention the copyrighted third-party interview
  instrument whose foundation forbids chat-bot use; `Validate` and the
  repository-wide branding test enforce this.

## Dependencies

Standard library + `gopkg.in/yaml.v3`. No internal imports — same layer as
`internal/products`.
