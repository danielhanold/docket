# Design: retire the inline board/source-drift health check (change 0024)

**Status:** design (auto-groomed 2026-07-08, `docket-auto-groom`)
**Change:** 0024
**Depends on:** 0022 — `render-board.sh` made `inline` rendering deterministic (satisfied: 0022 is `done`, PR #35 merged).
**Related:** 0023 — scripted the five mechanical checks into `board-checks.sh` and *deliberately left* the inline drift check model-driven "owned by change 0024"; ADR-0012 (its script-vs-model boundary).

> **Auto-groom note.** This spec was designed autonomously (no human brainstorm) by biasing every
> decision to the conservative default and recording the rejected alternatives below for a deferred
> human audit. It was gated by the `docket-auto-groom-critic` before emission. The one decision a
> human might reasonably override is flagged in **§Assumptions A2** (a future `docket`-branch CI gate).

---

## 1. Context

`docket-status`'s Health-checks section carries a model-driven **"Board/source drift"** check
(`skills/docket-status/SKILL.md`, the last bullet under *Model-driven checks*). For the `inline`
surface it renders `BOARD.md` in-memory from the change files and byte-compares against the committed
`BOARD.md`, warning if they disagree ("a writer skipped the board-refresh invariant"). For the
`github` surface it warns when a change's `issue:` mirror is unreachable (a visibility flag).

The check predates determinism. Change **0022** extracted `inline` rendering into the deterministic,
idempotent `render-board.sh` (same change files ⇒ byte-identical `BOARD.md`). Change 0022's own *Why*
anticipated this follow-up: *"A script removes that whole failure class for `inline`."* Change **0023**
then scripted the five mechanical checks and **left this one decision to 0024** (locking it in place
with two test assertions, below) precisely because it hinges on 0022 landing first — which it now has.

This change decides whether the `inline` drift check still earns its keep, and applies the decision.

## 2. The failure classes the check was defending

The original check conflated two failure modes:

1. **Board rendered *wrong*** — the model, rendering `BOARD.md` non-deterministically, produced a board
   inconsistent with the change files even while believing it had refreshed. **This class is dead.**
   `render-board.sh` is a pure deterministic function of the change files; a script cannot render the
   board "wrong" relative to its inputs.

2. **Board-refresh *skipped*** — a skill wrote a change's `status:` but the mandatory board-refresh
   commit never landed (bug, interrupted run, dropped in a rebase), so the committed `BOARD.md` on
   `origin/docket` lags the change files. **This class survives 0022** — but see §3.

## 3. Why the surviving class is already covered structurally

Two facts make a dedicated check for class (2) vacuous *where it runs*:

- **`docket-status` unconditionally re-renders first.** The Board pass runs at the *top* of every
  `docket-status` invocation and regenerates `BOARD.md` (separate commit) **before** the Health-checks
  pass. So by the time a drift check would run, any staleness has already been healed — the check
  structurally cannot observe the condition it exists to flag. The current prose admits this: *"the
  Board pass in this same `docket-status` run re-renders the enabled surfaces and heals the drift
  regardless."* A check that cannot fire is pure token cost.

- **The board is a derived view; the failure is cosmetic and self-healing.** `BOARD.md` is losslessly
  regenerable from the change files at any moment (convention: *"the board is a derived view"*). A stale
  board loses zero information, and the next status write **or** `docket-status` run heals it. The
  authoritative record is always the change files + git history — never the board.

The real guarantee against class (2) is the convention's **"Board refresh on status writes"** invariant
(every status-writing skill refreshes each enabled surface immediately). That invariant **stays** and is
untouched here — it is the mechanism; the retired check was only a tripwire on top of it.

## 4. Decision

**Retire the `inline` board/source-drift check. Keep the `github` mirror-reachability visibility flag.
No scripted replacement is added now. No new ADR; no convention edit.**

Concretely:

1. **`skills/docket-status/SKILL.md`** — replace the single combined "Board/source drift" bullet with a
   `github`-only **mirror-reachability** bullet (the surviving visibility flag), plus one terse clause
   recording that the `inline` drift check was retired by change 0024 (deterministic render + the
   unconditional Board-pass re-render make it vacuous). Keep the bullet lean — full reasoning lives here,
   not in the skill.
2. **`tests/test_board_checks.sh`** (the assertion at the "keeps the inline board/source drift check
   (owned by change 0024)" line) — replace the *keep-the-check* assertion with a **positive** anchor on
   what survives: `docket-status` still carries the `github` mirror-reachability visibility flag. Do
   **not** add an absence assertion for the string "board/source drift" — the skill's retirement note
   still mentions the phrase, so an absence grep would false-fail (LEARNINGS 2026-06-21 #36: absence
   sentinels self-defeat when the phrase legitimately appears in a contrast/retirement clause).
3. **`tests/test_board_refresh_on_transition.sh`** (assertion **E**, "docket-status has board/source
   drift health check") — remove assertion E and its comment; the tripwire it checked is retired.
   Assertions A–D (the board-refresh *invariant* in the convention + the per-site refreshes) are the
   surviving contract of change 0004 and stay.

**No new ADR.** Retiring a warn-only, now-vacuous check is operational cleanup that *follows from*
0022's determinism (which itself shipped ADR-free). It neither reverses nor supersedes any Accepted ADR;
ADR-0012's script-vs-model boundary is unaffected (this is not "move a check from model to script" — it
is "remove a check that no longer fires"). **No convention edit** — the convention enumerates no such
check; its board-refresh invariant (the real defense) is unchanged.

## 5. Assumptions (auto-groom decision log — deferred human audit)

**A1 — Retire, not downgrade to a scripted staleness check.**
Chosen: retire outright; add no `board-stale` check to `board-checks.sh`.
Rejected alternative (the change body's option 2): a scripted byte-compare (`render-board.sh` output vs
committed `BOARD.md`) folded into `board-checks.sh --strict`.
Why retire: (a) *vacuous where it runs* — `docket-status`'s Board pass heals drift before any check
could see it (§3); (b) the byte-compare's only non-vacuous use is a standalone `--strict` gate, and
**no such gate exists** — building it now is speculative (YAGNI); (c) the failure it guards is cosmetic
and self-healing over a derived view; (d) it honors 0022's stated intent. The retire is cheaply
reversible: if a consumer appears, the scriptable check is easy to add then (see A2).

**A2 — No scripted byte-compare now, but the door is left open (the one human-overridable call).**
The single fact that would flip A1 to *downgrade* is a **planned `docket`-branch CI gate** that runs
`board-checks.sh --strict` without re-rendering. That is private roadmap knowledge a human may hold. If
such a gate is intended, the natural home is a new mechanical `board-stale` check in `board-checks.sh`
that byte-compares `render-board.sh` output against the committed `BOARD.md` (now deterministic, so the
compare is meaningful and false-positive-free) and is emitted under `--strict`. This spec deliberately
does **not** build it (no consumer today); a human wanting the CI gate should say so and this becomes a
one-check addition. Recorded here so the deferral is a conscious, reversible choice, not an oversight.

**A3 — No new ADR; convention untouched.**
Chosen: convention/skill + test edits only; `adrs:` stays `[]`.
Rejected alternatives: (i) a new ADR recording the retirement; (ii) a dated `## Update` note appended to
ADR-0012 (whose line 17 names "inline board/source drift" as an agent-driven example that no longer
exists). Why neither: (i) the retirement is a downstream *consequence* of 0022's determinism, not a
durable architectural decision a future reader must rediscover — it reverses/supersedes nothing. (ii) An
`## Update` append to an immutable ADR drags in the `adrs:`-listing + terminal-publish machinery
(LEARNINGS 2026-06-17 #31) for a cosmetic staleness; ADR-0012 accurately describes the state *as of
change 0023* and readers understand ADRs are point-in-time. Keeping it untouched is the minimal
conservative path. (If a human prefers the audit trail, the `## Update` note is the low-cost option.)

**A4 — Keep the `github` half; do not fold it away.**
The combined bullet currently covers both surfaces. Retiring `inline` must not silently drop the
`github` mirror-reachability flag (a distinct, still-useful best-effort visibility signal). The rewrite
keeps it as its own bullet.

**A5 — Dependency state.** `depends_on: [22]` is satisfied — 0022 is `done` (PR #35 merged), confirmed
by the human dispatching this groom ("change 22 is completed and merged"). The design is not gated; the
implementer's reconcile re-validates at build time.

## 6. Touch-points (exhaustive; enumerated touch-points are a floor — regrep at build)

**Edit (live sources of truth):**
- `skills/docket-status/SKILL.md` — the "Board/source drift" bullet → `github`-only reachability bullet
  + retirement clause (§4.1).
- `tests/test_board_checks.sh` — the "keeps the inline board/source drift check" assertion → positive
  `github`-reachability-survives assertion (§4.2).
- `tests/test_board_refresh_on_transition.sh` — remove assertion E (§4.3).

**Do NOT edit (immutable historical records):** `docs/adrs/0012-*` (Accepted/immutable — see A3),
and every `docs/changes/archive/*`, `docs/superpowers/specs/*`, `docs/superpowers/plans/*` that mentions
"board/source drift" (0004, 0005, 0022, 0023, github-mirror) — build-time records, never re-edited.

**Confirmed no edit:** `skills/docket-convention/SKILL.md` — no check enumeration to change; the
board-refresh invariant (its line ~187) stays. `docket-status` overview/description enumerate "stale
claims, broken links, dependency stalls" — no drift mention, so no edit.

**Build-time regrep (LEARNINGS 2026-07-08 #42 / 2026-06-20 #32):** before finishing, grep the whole
live tree (`skills/`, `tests/`, `scripts/`) for `board/source drift`, `board/source-drift`, and
`Board/source drift` to confirm no *live* (non-archive) consumer beyond the three above was missed.

## 7. Test plan

- **Update** the two assertions above; run `bash tests/test_board_checks.sh` and
  `bash tests/test_board_refresh_on_transition.sh` — both green.
- **Mutation-check the new positive assertion**: deleting the `github` reachability clause from the
  skill must flip `test_board_checks.sh`'s new assertion to NOT OK (prove non-vacuity, LEARNINGS #2).
- **Full suite** green (the retirement touches only docket-status prose + two test files; no script
  behavior changes, so `render-board.sh`/`board-checks.sh` tests are unaffected).

## 8. Out of scope

- The `github`-surface mirror-reachability flag's behavior (kept, not changed).
- `render-board.sh` / `board-checks.sh` code (no behavior change — this is a prose + test edit).
- Any `docket`-branch CI `--strict` gate and the `board-stale` scripted check (deferred — A2).
- The convention's board-refresh invariant (the surviving real defense — untouched).
