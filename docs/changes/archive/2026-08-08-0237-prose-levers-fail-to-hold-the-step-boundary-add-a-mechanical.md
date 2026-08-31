---
id: 237
slug: prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical
title: "Prose levers fail to hold the step boundary — give the disposition contract a consumer"
status: done
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: [219]
related: [96, 113, 212, 235, 236, 242]
discovered_from: [235]
adrs: [69, 75]
spec: docs/superpowers/specs/2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical-design.md
plan: docs/superpowers/plans/2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical.md
results: docs/results/2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical-results.md
trivial: false
auto_groomable: false
branch: feat/prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/176
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical-design.md) |
| Plan | [2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical.md) |
| Results | [2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical-results.md) |
| ADRs | [ADR-0069](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0069-mode-conditioned-clause-discriminates-on-provenance.md), [ADR-0075](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0075-run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code.md) |
<!-- docket:artifacts:end -->

## Why

An autonomous `docket-implement-next` run is defined by seven steps ending in an open PR. Six times
it has executed some prefix of them and reported success — 0109, 0194 (×2), 0206, 0231, and 0235.

Every remedy docket has shipped for this family is **prose addressed to the agent that is failing**:
0096's call-site pre-specification (ADR-0044), 0113's split §5 sentence and the *Step postconditions*
table, 0212's mode-conditioned scoping clause (ADR-0069), and the four-value terminal-disposition
contract itself. The last two instances each occurred with every applicable lever present, correctly
worded, and applicable by its own terms.

Instance 6 is the sharpest datum, because it is the first live run to exercise 0212's fixed path.
`/docket-implement-next 235` ran Steps 0–5 in full — six build commits, suite green — then ended its
turn at the Step 5/6 boundary with the branch unpushed, no review, no PR, and the manifest still
`in-progress`. Its closing line was *"Build disposition (role-scoped): green"* — `docket-build`'s
vocabulary, not the driver's, which is exactly the independently-checkable tell 0212 introduced.
Nothing checked it.

The structural gap, stated once: **the terminal-disposition contract has a producer and no
consumer.** `advanced` is claimable only when Step 7's postcondition holds — a statement entirely
readable from git, that no code reads.

It matters because the failure mode is a half-run that reports success: the caller sees `completed`,
the board shows `in-progress`, the branch is unpushed, and only a human reading the transcript
notices.

## What changes

Build the missing consumer, and call it from a seam docket owns. Three pieces:

- **`docket.sh verify-run <id>`** — a git-only pure reader that evaluates Step 7's postcondition for
  one change (status advanced, `pr:` recorded, branch pushed) and reports one verdict line:
  `run-complete` / `run-halted` / `run-incomplete` / `run-unclaimed`. No network, no `gh`, no status
  writes, no time floor.
- **`runner-dispatch.sh` calls it.** The facade currently `exec`s its adapter and never regains
  control; it becomes a call-and-return that, for `implement-next` delegations only, diffs the
  in-progress set across the hand-off to identify this run's claim, verifies it, and gives an
  unfinished run **one** bounded re-dispatch before aborting loudly. This is the seam docket owns —
  one edit covers `codex`, `cursor`, `opencode`, and every future adapter with no harness
  cooperation, no hook, and no `settings.json`.
- **`## Run halted`** — a dated, presence-encoded change-body section in the same family as
  `## Auto-groom blocked`. A run that legitimately cannot proceed clears the gate by *writing this
  and committing it*, which makes a `halted` disposition verifiable without trusting the worker's
  self-report.

Full design, including the snapshot-diff discriminator and the rejected alternatives, is in the
linked spec.

## Out of scope

- **Re-fixing any step boundary with more prose.** That is what 0096, 0113 and 0212 each did.
- **A Claude Code `Stop` / `SubagentStop` hook**, and all `settings.json` and installer work. It was
  investigated and confirmed workable, but it covers exactly one harness and is the only candidate
  whose code docket does not own. Filed as its own stub — it is a small wiring job onto the oracle
  this change builds — filed as change 0242.
- **Any change to `board-checks.sh`'s legs or floors.** A board pass cannot distinguish a stopped run
  from a live one, so its floors are correct; and change 0219 rewrote that block hours before this
  design. `board-checks.md` gains one pointer sentence, nothing more.
- Any new config knob, status flip, or claim release.

## Reconcile log

### 2026-08-07 — reconcile (implement-next)

Verified against current `origin/docket` + `origin/main`:

- **Dependencies and lineage hold.** 0219 is archived `done` (leg D shipped, `detect_orphan_pr`
  landed); 0236 is archived `killed` into this change, exactly as the spec records. 0242 exists as a
  `proposed` stub `waiting-on-237`, so the deferred Claude-hook scope stays tracked elsewhere.
- **Nothing in scope has been built elsewhere.** `scripts/verify-run.sh` and
  `scripts/verify-run.md` do not exist; `scripts/runner-dispatch.sh` still ends at the
  `exec "$DOCKET_BASH_PATH" "$ADAPTER" …` hand-off (line 124), so §2's call-and-return conversion is
  untouched work. `verify-run` is absent from `WRAPPED_OPS` in `scripts/docket.sh` and from the
  operations table in `scripts/docket.md`.
- **`board-checks.sh` remains untouched** per §4, and change 0219's rewrite of the `aborted-run`
  block is the current state — confirming the spec's rejection of a delegation refactor.
- **One drift, scope-adjusting only.** The spec directs a pointer sentence into
  `scripts/board-checks.md`'s `## Not covered` paragraph. That heading **does not exist** in the
  file today; the document's headings are Purpose / Usage / Behavior / Exit codes / Invariants, and
  the not-covered material lives inside the `aborted-run` narrative as its *Known residual* /
  *surviving residual* prose. The sentence therefore lands in that residual prose, pointing at
  `verify-run` as the floor-free check available at a dispatch seam. Same content, same
  documentation-only scope, correct anchor.

No other scope change. Design intact; proceeding to plan.
