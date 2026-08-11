---
id: 118
slug: decide-whether-the-sweep-s-skip-publish-path-should-also-mar
title: Decide whether the sweep's skip-publish path should also mark an unpublished terminal record
status: in-progress
priority: medium
created: 2026-07-21
updated: 2026-08-11
depends_on: []
related: [154, 254]
discovered_from: [83]
adrs: []
spec: docs/superpowers/specs/2026-08-07-decide-whether-the-sweep-s-skip-publish-path-should-also-mar-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/decide-whether-the-sweep-s-skip-publish-path-should-also-mar
claimed_at: 2026-08-11T18:06:02Z
pr:
blocked_by:
reconciled: true
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-decide-whether-the-sweep-s-skip-publish-path-should-also-mar-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-decide-whether-the-sweep-s-skip-publish-path-should-also-mar-design.md) |
<!-- docket:artifacts:end -->

## Why

Change #0083 made a deferred terminal publish visible with the `## Publish deferred` marker
(ADR-0051), and the sweep marks itself on its `terminal-publish` failure branch. One sibling
path was left unmarked: the `render-change-links skipped-publish` branch, on the stated
rationale (`scripts/docket-status.md:186-196`) that *nothing published means nothing was
deferred yet*.

That rationale does not survive the code. Once archived, a change leaves `active/` and **no
sweep ever resumes it** — the gap is permanent until a human acts, byte-for-byte the #0043
state that went unnoticed for eight days. And the marker cannot flap on this leg (only a
successful publish removes it, and nothing automated retries one), so the transient-noise
counter-argument collapses too. Whether the publish was deferred, blocked, or never reached
is a distinction about cause, not visibility.

## What changes

Settled — see the spec (2026-08-07). The skipped-publish branch marks, under the same
expected-publish gate the marker contract requires:

- **`scripts/docket-status.sh`**: in `sweep_execute_one`'s `skipped-publish` branch, when
  `TERMINAL_PUBLISH=true` AND docket-mode, mark (`mark-publish-deferred.sh --mode add
  --reason blocked`, distinct `--detail` telling the human to re-render before publishing)
  plus muted commit+push — best-effort toward the report contract but transactional toward
  the worktree: restore the archived path to HEAD on add/commit failure, retain the local
  commit on push failure; same recovery back-ported to the 0083 mark block. The report
  line and `return 0` are unchanged; the gate is load-bearing because, unlike the 0083
  branch, suppression does not make this branch unreachable.
- **`scripts/mark-publish-deferred.sh`**: generalize the fixed body prose ("Close-out
  steps 1–2 … landed"), which is factually false on the new path where the re-render is
  what failed; heading, interface, and contract shape untouched.
- **Docs state the real reason:** rewrite `scripts/docket-status.md` §6's rationale and
  `skills/docket-status/SKILL.md`'s sweep-posture paragraph; extend
  `terminal-close-out.md`'s step-3 mark rule to any expected-publish path abandoned before
  publish (scoped per driver — the sweep's continue-to-publish deviation on the commit/push
  leg stays, and the guard's "(all callers)" sentence gets the sweep carved out).
- **Not done:** no third `--reason` value, no new heading, no failure counter, no change to
  the `commit-failed`/`push-failed` legs, no code in the three skill-driven drivers.

## Out of scope

- Re-opening #0083's declined branch-diff detector/healer, or the `terminal_publish` knob.
- The `## Publish deferred` marker's format, writer, or removal semantics (ADR-0051).
- The skip-publish guard itself — a failed artifacts re-render must still never publish.

## Open questions

None — resolved in the spec: frequency is irrelevant (permanence, not frequency, decides —
and the failure can even be transient via the renderer's config-resolution `git fetch`);
the marker shares the existing heading with cause in the detail line; the close-out
failure-branch audit is in the spec (hard-crash residual stays accepted per ADR-0051).

## Carry-forward from #0247 (2026-08-11)

Change 0247 landed on this surface and spent its budget headroom. Before adding to
`scripts/docket-status.sh`, `tests/test_docket_status.sh`, or `skills/docket-status/SKILL.md`, read
these two numbers as measured at 0247's close-out:

- `tests/test_docket_status.sh` — roughly **3s of margin** against its 45s row in
  `tests/runtime-budgets.tsv`.
- `skills/docket-status/SKILL.md` — **22 words** of headroom against its size budget.

The next edit to either trips a budget. The remedy is already settled and should not be re-derived:
apply **change 0137's rounding rule** (next multiple of 5 plus a 5s margin, computed from the worst
*standalone serial* reading, never the contended run-of-the-day number) and carry **change 0201's
in-diff argument** for the word budget. **#0268 is queued against the same surface**, so whichever of
the two lands second inherits whatever margin the first leaves. See the learnings finding
`budget-headroom-is-spent-before-it-is-breached`.

## Reconcile log

### 2026-08-11 — reconciled against main @ 4e75b2e2

**The decision is not re-litigated.** The title reads as an open question, but it was settled by
the 2026-08-07 spec (§Decision: *yes, the skipped-publish leg marks*), on **permanence** — no
sweep resumes an archived change, so the gap is forever until a human acts. None of the nine
changes merged since touched that leg, so nothing reopened or pre-empted the decision. This run
builds what the spec settled.

**Scope adjustment from #0247 (74895565), which landed on this exact surface.** 0247 taught
`sweep_execute_one`'s artifacts-refresh block two rules for committing into the *shared* metadata
worktree: scope `add`/`commit` with `--`, and refuse when the tree is mid-rebase/merge
(`_docket_tree_wedged`, `scripts/lib/docket-preflight.sh`), reporting `blocked-wedged-tree`. The
mark block this change adds — and the 0083 block it back-ports recovery to — are the **third and
fourth** exposed commits into that same tree, and 0247 did not reach them. So the spec's
"precondition: the archived path is clean" is **widened to include the wedge probe**: a wedged
tree skips the mark entirely. This is not scope creep but a correctness requirement of the
spec's own transactional posture — the spec's recovery step is `checkout HEAD -- "$archived"`,
and inside a rebase `HEAD` is the rebase's *detached* HEAD, so the recovery itself would corrupt
the file it exists to restore. Adding the probe is what makes the recovery sound.

**Docs targets re-read; the spec's line numbers have drifted, the paragraphs have not.**
`scripts/docket-status.md` §6 still carries the "nothing published means nothing was deferred
*yet*" rationale verbatim (now further down the file, past 0247's §6a rewrite), and
`skills/docket-status/SKILL.md`'s sweep-posture paragraph still says the `skipped-publish` case
is unmarked — both now also mention 0247's `blocked-wedged-tree` reason, which these rewrites
preserve.

**§2's whole-repo grep re-run, as the spec required.** `Close-out steps 1` appears in exactly one
maintained file — `scripts/mark-publish-deferred.sh:174`. The three other hits are frozen
point-in-time records (two archived specs and the merged 0083 plan) and are correctly left
untouched.

**Budget facts confirmed as measured, not assumed.** `skills/docket-status/SKILL.md` is at
2478/2500 words (22 words of headroom, 102/118 lines); `tests/test_docket_status.sh` holds the
45s row. Both are expected to trip and will be raised in-diff with measured numbers per 0137's
rounding rule and 0201's argument requirement.

**Unaffected by the rest of the drain.** 0260/0284/0281/0275/0208/0277/0286/0270 concern dispatch
gating, liveness probing, critic return channels, attribution, worktree scope, brief delivery,
poll-loop shape, and runner-config locality — none intersects the sweep's close-out chain.

**Couplings re-checked (spec Assumption 7).** #0154 and #0268 both still target
`skills/docket-status/SKILL.md` and `scripts/docket-status.sh`; neither has landed, so this
change is the first mover and the diff is kept tight to its own spec.
