# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Mood self-check mode (PHQ-9, ru-only v1)** — third branch of the `/start`
  mode fork; direct entry via `/mood`, data deletion via `/mood_delete`.
  - Official Russian PHQ-9 («Russian for Russia», phqscreeners.com) verbatim:
    instruction, 9 items, 4-option scale; new content bundle
    `screening/phq9_ru.yaml` + `screening/mood_module_ru.yaml` with startup
    fail-fast validation pinning the texts, the published severity bands
    (0–4/5–9/10–14/15–19/20–27), and the crisis-card contacts.
  - Flow: short consent → 9 questions one at a time → result with the 0–27
    score, a severity wording without diagnosis labels, a
    repeat-in-2–4-weeks line, the delta against the previous run, a compact
    doctor report, and the one-line attribution (PHQ-9 — Spitzer, Williams,
    Kroenke). Pause/resume and the FODMAP detour work like the ADHD mode.
  - Deterministic crisis protocol: any answer > 0 on item 9 shows the
    support-contacts card immediately after the answer (МЧС, Москва 051,
    Красный Крест, 112, findahelpline.com; the children's helpline is
    excluded by validator + tests); answers 2–3 add one direct
    talk-to-someone-today line; the test is never blocked; the contacts
    repeat in the final result regardless of the total score.
  - Privacy: raw answers live only in the unfinished run and are erased in
    the same write that stores the final result (score, band, date, item-9
    flag only); `/report` gains a lean mood summary line.
- **Adult ADHD self-check mode** (ru-only v1) next to the FODMAP diary:
  `/start` now shows a mode fork; direct entry via `/adhd`, data deletion via
  `/adhd_delete`.
  - Staged flow with pauses at block boundaries: consent → intro →
    ASRS v1.1 part A (official Russian WHO text, verbatim) → part B
    (unofficial translation, flagged) → WURS-25 (unofficial translation,
    flagged; masculine/feminine wording choice) → onset question →
    life-domain multi-selects (adulthood + childhood).
  - Per-instrument results with their own published thresholds and
    attributions — no combined score by design; overall wording
    "consistent / partially / not consistent with DSM-5 criteria",
    screening-not-a-diagnosis disclaimer, referral note, and a shareable
    doctor report rendered on the fly.
  - Privacy: raw per-question answers exist only while a run is unfinished
    and are erased in the same write that stores the final result;
    `/adhd_delete` wipes everything after confirmation.
  - New read-only content bundle `screening/*.yaml` baked into the image
    (`bootstrap.screening_dir`); startup fail-fast validation pins the
    canonical texts and thresholds.
- `/report` now includes the last self-check summary line.

### Changed
- `/start` no longer interrupts an active trial by itself — the interrupt
  moved to the "FODMAP diary" button of the mode fork (same semantics).
- `/abandon` mid-screening wipes the unfinished run and returns to the
  saved diary position; diary behavior unchanged.

## [0.1.0] - 2026-04-26

### Added
- Initial bot scaffold using `go-telegram-bot-api/v5`
- Long-polling update loop with graceful shutdown on SIGINT/SIGTERM
- `/ping` command — replies `pong`
- Multi-stage Dockerfile for production VPS deployment
- `.env.example` with `TELEGRAM_BOT_TOKEN` and `BOT_DEBUG` variables
