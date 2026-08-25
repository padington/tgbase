# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Mood module v2 — WHO-5 quick check, GAD-7, PHQ-9 functional item.** The
  mood mode becomes a three-instrument hub behind one consent and a
  mini-menu («⚡ Быстрый чек (1 мин)» / «📋 Настроение (PHQ-9)» / «😰 Тревога
  (GAD-7)» + resume rows for unfinished runs).
  - **WHO-5** (official Russian text from WHO publication
    WHO-UCN-MSD-MHE-2024.01, verbatim, provenance pinned): 5 statements,
    6-option scale in the form's order, raw sum × 4 → 0–100. > 50 —
    «самочувствие в норме»; ≤ 50 — «снижено» + a one-button offer to take
    the full PHQ-9; ≤ 28 — a more insistent offer. One-line © WHO
    attribution. No crisis item by design.
  - **GAD-7** (official Russian version from phqscreeners.com, verbatim,
    provenance pinned): 7 items, the PHQ-9 scale, 0–21 with the published
    0–4/5–9/10–14/15–21 gradations (10–14 suggests discussing with a
    doctor, 15–21 recommends a specialist). No crisis item.
  - **PHQ-9 functional (10th) question** — the official form's follow-up
    («насколько трудно Вам было работать…», 4 options, verbatim from the
    already-pinned PDF): asked only when at least one of the nine answers
    is > 0, never part of the 0–27 score, stored and surfaced as its own
    line in the doctor report.
  - **Links**: the PHQ-9 report offers the GAD-7 with one button (no
    clinical terms); offers resume paused runs instead of wiping them.
    Each instrument keeps its own result and band wording — no combined
    index.
  - **Combined doctor report**: everything completed (PHQ-9 + item-9 fact +
    functional answer, GAD-7, WHO-5) with dates, rendered on the fly.
  - One consent per module (`MoodConsentAt`; stored mood data implies it,
    so v1 users are never re-asked); `/mood_delete` wipes all three
    instruments plus the consent; `/abandon` wipes only the active
    instrument's run; `/report` gains lean GAD-7/WHO-5 lines; the `/mood`
    command-menu description becomes «самопроверка настроения и тревоги».

### Changed (mood v2)
- The v1 per-run consent/resume gate is folded into the module menu: resume
  rows continue at the exact position (crisis card and functional question
  included); tapping an instrument button with its own run paused is the
  explicit start-over.
- **Command-menu self-registration** — on boot the bot calls `setMyCommands`
  (default scope, ru descriptions as default + a `language_code="en"`
  variant from the i18n bundles) so the client command menu always matches
  the deployed binary; the manual BotFather step is gone. Registration
  failure is a logged warning, never a boot error.
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
