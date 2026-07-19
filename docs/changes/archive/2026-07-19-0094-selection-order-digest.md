---
id: 94
slug: selection-order-digest
title: Selection-order backlog digest — implement-next selects from a digest instead of walking active/
status: done
priority: medium
created: 2026-07-17
updated: 2026-07-19
depends_on: []
related: [69, 85, 88, 93]
adrs: [12, 47]
spec: docs/superpowers/specs/2026-07-19-selection-order-digest-design.md
plan: docs/superpowers/plans/2026-07-19-selection-order-digest-plan.md
results: docs/results/2026-07-19-selection-order-digest-results.md
trivial: false
auto_groomable: false
branch: feat/selection-order-digest
claimed_at: 
pr: 108
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-19-selection-order-digest-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-selection-order-digest-design.md) |
| Plan | [2026-07-19-selection-order-digest-plan.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-19-selection-order-digest-plan.md) |
| Results | [2026-07-19-selection-order-digest-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-19-selection-order-digest-results.md) |
| PR | 108 |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0047](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0047-digest-only-read-tier-skips-preflight.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` Step 1 selects by having the **model** walk `active/`: read every change
file's frontmatter, filter to build-ready, rank by priority → created → id. That is a per-run cost
that grows with the backlog, and the ranking is model-executed rather than deterministic.

**#0088 (merged 2026-07-18) turned that from a once-per-invocation cost into a per-iteration one.**
Its loop-continuation contract is driver-agnostic prose, and `/loop docket-implement-next` — the
recommended driver, confirmed working — re-forks the skill per iteration with **fresh context**. So
draining N changes now re-walks `active/` N times. 0088 shipped no digest consumption of any kind;
Step 1 is byte-unchanged in how it acquires state.

This change gives that walk a deterministic, single-read replacement. It is deliberately **not** the
original "docket-prime" framing, which is dead: #0069 already ships the stdout-only
`render-board.sh --format digest` (backlog rollups + one line per active change), and #0093 shipped
archive decay. What survives is the narrow, genuinely missing piece — **selection order** — plus the
plumbing needed for anything to actually consume it.

Two blockers found while re-scoping, which is why this is not a pure one-script extension:

- **`render-board` is not reachable from a skill.** The `docket.sh` facade exposes `preflight env
  bootstrap docket-status board-refresh archive-change …` — no `render-board`. The digest exists
  only as internal report output inside `docket-status.sh`.
- **The one path that emits it, `docket-status --board-only`, also commits and pushes `BOARD.md`.**
  A selection read must not be a write.

Without addressing those, a selection-order queue ships with no way to reach it.

## What changes

1. **One `ready` line** in `render-board.sh --format digest` — build-ready ids in selection order
   (priority → created → id), emitted after the existing `change …` lines, which stay untouched and
   id-ascending. One line regardless of backlog size. **Always emitted**, bare when the queue is
   empty, so absence means *no queue was produced* rather than *nothing is ready*.
2. **`docket-status.sh --digest-only`** — a write-free read: config, rollups, `change` lines, `ready`
   line, exit. No sweep, no board render, no commit, no push, and no `board …` report line.
3. **Rewire `docket-implement-next` Step 1** to take its ordered candidate set from the `ready` line
   and confirm build-readiness by reading that one change file before claiming. The digest is an
   **accelerator, not the sole channel** — the change files stay authoritative, so a stale digest
   costs a re-pick, never a bad build. No `ready` line at all → fall back to walking `active/` and
   report the degradation.

The claim-age date from the original scope is **dropped** — no named consumer, and `board-checks.sh`
plus `reclaim-claims.sh` already own the claim-lease signal.

Full design, the rejected alternatives, and the prose-posture guardrails for the Step 1 rewrite are
in the linked spec.

## Out of scope

- **ADR index titles and learnings-index hooks in the digest** — they require reading sources
  `render-board` must not own (ADR-0012) and would *add* per-digest tokens, contradicting the
  purpose. Explicitly dropped at the 2026-07-17 groom and still dropped.
- Replacing `BOARD.md` (the human-facing board stays; this is the agent-facing projection).
- A new committed or cached surface — the digest stays stdout-only and always-fresh.
- Semantic/embedding relevance ranking of anything.
- Rewiring any skill other than `docket-implement-next`. The full `docket-status` report picks the
  `ready` line up automatically (same projection, no new call site) — a free readability win, but no
  other skill's selection path changes here.
- Changing *what* any skill does with the state — only how it acquires it.

## Open questions

_All four resolved at the 2026-07-19 groom — see the linked spec §2. The entry point is a write-free
`docket-status --digest-only`, not a facade exposure of `render-board` (which would re-open the
`> BOARD.md` gate-bypass); the queue is one always-emitted `ready` line, not per-entry lines and not
an in-place reorder; the digest is an accelerator, so the convention's selection definition stays
authoritative in Step 1's prose; claim-age is dropped for want of a consumer._

## Reconcile log

### 2026-07-19 — build claim (docket-implement-next)

Reconciled against `origin/main` @ `a75995f`. **Scope unchanged; design holds in full.** Every
premise the spec rests on was re-verified in current code rather than taken on trust:

- **`render-board.sh --format digest` ships as described** — the digest block emits `backlog …`
  rollups then one `change <id> <status> <readiness> <slug>` per active change, id-ascending, and
  `exit 0`s before the markdown path. `digest_readiness()` delegates to `readiness()` in
  `lib/docket-frontmatter.sh`, which returns the `build-ready` token. Confirmed: the `ready` line
  needs **no new readiness logic and no new file reads** (spec §2.2).
- **Both blockers are still real.** `docket.sh`'s `WRAPPED_OPS` still has no `render-board`, and
  `docket-status.sh --board-only` still commits + pushes `BOARD.md`. The `--digest-only` entry
  point (§2.4) remains the necessary plumbing, not an optional nicety.
- **No classifier change needed.** `board_classify()` keys only on lines matching `board `; its
  own comment names the digest as ignored. A `ready` line cannot be mistaken for a board verdict.
- **Related work is all landed** — 0069 (digest projection), 0085 (board classify/must-land), 0088
  (loop continuation), 0093 (archive decay) are archived `done`; ADR-0012 (script-vs-model
  boundary) is still `Accepted` and still governs: the script owns the ordering, the model does
  not re-derive it.
- **The budget raise is confirmed, not contingent.** `tests/test_skill_size_budgets.sh` carries
  `skills/docket-implement-next/SKILL.md 140 2845`; the file is at **129 lines / 2833 words** —
  11 lines and 12 words of headroom. The Step 1 rewrite will need an in-diff raise of that row.

**One implementation constraint surfaced that the spec does not state.** `docket-status.sh`'s
`main()` opens with `docket_preflight`, which **fetches and `pull --rebase`s the metadata
worktree**. That is a working-tree mutation and can move `HEAD`, which would contradict §5's
`--digest-only` test ("`BOARD.md` byte-unchanged, working tree clean, `HEAD` unmoved"). So
`--digest-only` must resolve config **without** the full preflight sync — config export only. This
is consistent rather than a compromise: §3 already orders the digest read *after* Step 0's
preflight, so the tree is freshly synced and a second sync would be redundant. Recorded here
because it is the one place the build could silently satisfy the prose and fail the posture.

**Escape hatches: neither fired.** The change is not obsolete and the design is not invalidated.
