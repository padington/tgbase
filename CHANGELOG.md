# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Pushup track — «Отжимания» (ru-only v1).** Fifth mode on the `/start`
  landing; direct entry via `/pushups`, deletion via `/pushups_delete`. The
  first mode that is not a questionnaire: it runs a training **program**, and
  it is the first track in the bot with a timer, a session history and a
  reminder that belongs to data rather than to a state.
  - **One number, no tables.** A max test sets the **base** `B`, the only
    program variable. Every number the user ever sees — the sets of a
    session, the floor of the open last set, the rest length, the weekly
    step — is *generated* from `B` by pure functions in `internal/screening`.
    Nothing tabulated is copied or stored: the scheme (test → 3 sessions a
    week → several sets with the last one open → retest) is a method, and
    methods are not protectable, while the tables of the commercial program
    that popularised it are. The generator's parameters (`day_factors`,
    `work_share` / `open_share`, set-count thresholds, rest seconds, volume
    ceilings, progression steps) live as **data** in
    `screening/pushups_ru.yaml`, so the shape of the program is tunable
    without touching logic. The source's branding appears nowhere — in the
    texts, the commands, the yaml or the commit messages — and a canary
    keeps it that way.
  - **Progress by result, not by the calendar.** A week is three sessions
    whenever they happen. Each session classifies as `over` / `plan` /
    `short`; the weekly rule (a development of the 2-for-2 idea) raises the
    base, holds it, or repeats the week with a slightly lower one. **A
    partially completed session still counts** — «short» is a first-class
    outcome, not a failure — and **a repeated week is always announced with
    its base delta**, because a base that moves silently reads as a bug.
    Three repeats in a row open a two-button fork: an easier rung with an
    immediate retest, or +30 s of rest. Continuous base ⇒ no gaps in
    coverage; there are no discrete "columns" to fall between.
  - **Safety in code, not in prose.** A three-question entry gate
    (ACSM/PAR-Q+-shaped): "yes" to the cardio/metabolic/renal question stops
    the track and wipes the half-built program *and* the consent; joint pain
    starts two rungs easier; pregnancy adds a doctor line and the easiest
    rung. Then: a volume ceiling the planner applies silently (2.5 × B for
    the first six sessions, 3.5 × B after), a cap on the test, a hard 24-hour
    block between sessions that «Всё равно тренироваться» cannot buy through
    (that button only overrides the softer 24–48 h warning), and a red-flag
    check-in before the first session of every week that pauses the track on
    **facts** (cola-coloured urine, pain growing after 48 h, arms that will
    not straighten, lopsided swelling), not on a score.
  - **Server-side rest timer** — the one thing a chat does better than the
    mobile apps whose timers die when backgrounded. One set = one message:
    `pu_set` asks, `pu_rest` acknowledges and waits, the reminder loop
    delivers exactly ONE «Поехали» when the rest is over, «Готов раньше»
    does the same immediately, and a number sent during the rest is taken as
    the next set rather than rejected. No per-second `editMessageText` (Bot
    API rate limit), and **resting longer is never counted as a miss**. A
    session that outlives its 24-hour TTL is closed as a partial one,
    recorded by what was actually done, so the automaton is never parked.
  - **Due ping with rules.** «Пора тренироваться» is the first reminder in
    the bot not tied to a state, so it is fenced by the store's own filter
    (`state.Store.AllPushupDue`): only a user idle, on the landing or in the
    track menu is reachable — never one mid-check-in, mid-self-check or
    mid-set — never while an open session is still resumable, and at most
    once per due date. Two messages per cycle maximum (the ping, then one
    «Тренировка ждёт»), then silence until a session moves the date. Rest
    days say nothing.
  - **Nothing about the body is asked or stored** — no weight, height, BMI
    or calorie figure anywhere in the track, and the ladder of variations is
    described as "easier / harder" only, never as a share of body weight.
    The bot hosts an eating self-check that pins exactly this property; the
    new track inherits the pin and enforces it with its own canary over the
    assembled chat surface (every message body and every keyboard label,
    including the landing, `/report` and both pings), which also bans source
    branding, "N reps in M weeks" promises, methodology hedging and
    unrendered `{placeholders}`. The stated goal is to double your own test;
    the track has no finish line.
  - **Persistence**: `PuConsentAt`, `Pushups` (the program), `PuSession`
    (transient, cloned on every mutation), `PuTest` and `PuHistory` — a ring
    of the last 36 sessions (≈ 12 weeks) of numbers only, capped because it
    all lives inside the shared `users.json`. The proto schema mirrors every
    new shape. `/abandon` drops the SESSION only — base, week and history
    stand; `/pushups_delete` wipes program, session, tests, history and the
    consent in one write.
  - Deliberately **not** in v1: no norm bands by age or sex, no images (the
    progress screen draws a text sparkline), no day streaks, and only the
    four basic rungs in the picker.
- **Eating track — «Отношения с едой» (EDE-QS + BES + NIAS, ru-only v1).**
  Fourth mode on the `/start` landing; direct entry via `/food`, data
  deletion via `/food_delete`. Three instruments behind ONE track-wide
  consent and a mini-menu («📋 Основной тест» / «🍩 Переедание» / «🥄
  Избирательность в еде» + resume rows for unfinished runs). Goal of the
  track, stated plainly: a summary worth taking to a doctor.
  - **EDE-QS** — 12 items about the last 7 days, from the S2 File of the
    original PLoS ONE 2016 validation paper (CC BY 4.0). Both answer scales
    of the paper form are reproduced and the framing switches exactly where
    the form switches (days for items 1–10 → severity for 11–12). Sum 0–36
    against the screening cutoff **≥ 15** (Prnjak et al. 2020); only two
    readings, below and at-or-above. The applied cutoff is stored with the
    result, so an old result stays readable if the content ever changes.
  - **BES** — 16 groups of weighted statements (English original and weights
    from the NIMH Data Archive dictionary `binge01`, group splits verified
    against a second independent source). Statements are far too long for
    buttons: the message renders the group as a numbered list and the
    keyboard carries the numbers. The stored answer is the picked
    statement's **weight**, not its index — several statements in a group
    can share a weight, which is why the maximum is 46, not 48. Bands
    ≤ 17 / 18–26 / ≥ 27.
  - **NIAS** — 9 statements, three independent subscales of 0–15
    (избирательность / аппетит / опасения) with cutoffs ≥ 10 / ≥ 9 / ≥ 10
    (Burton Murray et al. 2021). **No total is computed** — the instrument
    is read per subscale, and a total would be meaningless.
  - **Burton Murray cross-reading, deterministic and in code**
    (`screening.NiasContext`): a positive NIAS subscale means different
    things depending on the EDE-QS. Below the cutoff → restriction without
    body-image concern; at or above → restriction likely tied to body
    image; EDE-QS not taken → the bot says outright that the two cannot be
    told apart and invites the core test. The reading upgrades itself as
    soon as the core test is completed — a NIAS result read before it is
    re-read after it.
  - **Combined doctor report** after every completion: each instrument with
    its date, score and the cutoff that was applied, the NIAS subscales and
    their reading. A **low-FODMAP context line is appended automatically**
    whenever the user has diary activity — a clinician reading the restraint
    items has to know that part of this person's dietary restriction is
    medically prescribed, or the screen reads falsely positive. The report
    is rendered on the fly and never stored.
  - **Content safety, enforced not just intended**: no instrument in the
    track asks for a weight, height or calorie figure, and no user-facing
    string names a diagnosis. Pinned twice — canonical tests over the raw
    content in `internal/screening`, and a canary in `internal/journey` that
    walks the whole track for two users and scans the assembled chat surface
    (every message body AND every button label, including the landing and
    `/report` strings i18n contributes). In the doctor report the strongest
    allowed wording is «скрин по шкале X положительный».
  - `/abandon` wipes only the active instrument's run; `/food_delete` wipes
    all three runs, all three results and the track consent in one write;
    `/report` gains one lean line per completed instrument (NIAS prints its
    three subscales). Russian texts are the bot authors' own translations —
    no validated Russian versions of these instruments exist — with
    provenance pinned in the content files and, per the owner's style, no
    methodology caveats in user-facing messages.
  - Deliberately flatter than the mood module, per the track's proof-of-
    concept scope: **no offer chain, no crisis card, no resource cards, no
    diary gates**. Every instrument ends the same way — score, reading,
    doctor report, landing.
  - Pauses need no bookkeeping: all three question phases derive the current
    item from the answer count, so `/start`, `/menu` and 🏠 interrupt
    anywhere and the landing's «▶️ Продолжить тест о еде» resumes on the
    exact next question (the track menu disambiguates when several runs are
    paused). The only thing written to an `EatingProgress.ResumeState` is
    `/food_delete`'s "opened mid-test" marker, and it never outlives that
    dialog — the cancel consumes it, escaping (🏠, `/start`, `/menu`,
    `/abandon`) drops it, and the next `/food_delete` starts clean. A stale
    marker would otherwise make a later `/food_delete` opened from the
    landing close back INTO a merely paused run.
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
