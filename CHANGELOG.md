# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
