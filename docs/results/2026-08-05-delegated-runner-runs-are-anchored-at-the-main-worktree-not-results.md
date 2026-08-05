<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0206 — Delegated runner runs are anchored at the main worktree, not the feature worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0206-delegated-runner-runs-are-anchored-at-the-main-worktree-not.md)**
<!-- docket:backlink:end -->

# Delegated runner runs are anchored at the main worktree, not the feature worktree — results
Change: #0206 · Branch: feat/delegated-runner-runs-are-anchored-at-the-main-worktree-not · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-05-delegated-run-worktree-anchor-plan.md · ADRs: 34, 68

## Verify (human)

Automated coverage is green across all 78 suite files, so nothing below is a correctness gate — these
are the judgement calls the review left for merge time.

- [ ] Decide whether gate 3's **containment-vs-membership** weakness blocks this merge or ships as
      follow-up #0208. `--worktree <repo root>` currently passes all three gates for a `build-*`
      agent, which anchors the worker in the primary checkout on the integration branch — the exact
      failure this change exists to prevent, reachable only by supplying a wrong value rather than by
      omitting one. The change still strictly improves on the status quo (an omission was silent
      before and is a loud abort now), which is why it was not treated as a blocker.
- [ ] Decide whether the `build-*`-only gate is acceptable for now, or whether #0209 (three more
      feature-scoped agent families, two of which commit) should land with it.
- [ ] Confirm you are content that `runners.<name>:` config keeps resolving from the **main
      worktree** rather than from the named anchor. This is deliberate and defensible — a feature
      branch must not be able to alter a runner's sandbox or permission posture for its own
      delegated run — but it is a real asymmetry.

## Findings

**Became ADR-0068** — *A delegated run's anchor is an explicit argument defaulting to the main
worktree.* The framework rule, with all four rejected alternatives recorded (resolve the caller's
CWD; derive the path inside the facade from docket state; forbid delegating `build-*`; require
`--worktree` for every delegation). ADR-0034 stands **unamended** and received a dated `## Update`
pointing at ADR-0068, delivered atomically via this change's `adrs:` per the `adr-update-delivery`
learning.

**Whole-branch review (rung: `docket-review-deep`)** returned 8 findings — 0 blocker, 3 important,
5 minor. No blocker means no auto-fix round ran. Beyond the two important findings captured as
#0208/#0209 and the valueless-flag hang captured as #0210:

- *No assert covers the central success path* — a `build-*` agent **with** `--worktree` succeeding
  and anchoring at the named tree. Legs (b)/(c) exercise the flag only with `--agent status`; leg (d)
  exercises `build-economy` only in its rejected state. A mutation making `build-*` abort
  unconditionally would leave the suite green. Folded into #0208's scope.
- *Residual cwd dependence* — a fully-qualified absolute `--worktree` is still refused when the
  **caller's** cwd is outside any repo, because the not-in-a-repo gate runs against `$PWD` before the
  anchor is resolved. Minor, but it is cwd affecting the outcome in a change whose thesis is that it
  must not.
- *`<repo>` is now ambiguous* in `runner-dispatch.md`'s Behavior step 3, since the anchor and the
  config root can differ. Documentation gap, not a code defect.
- *0205's results record* still carries the "not usable yet" claim. Deliberately left: results
  records are dated point-in-time artifacts, and rewriting them falsifies history.

**Plan deviations, both in task 3, both to satisfy rules the plan itself restates.** The plan's
specified insertion point placed a fixture-re-minting `mkgitrepo` ahead of the `--check` leg-(c)
block, which still read the previous fixture — it reddened two pre-existing `0079:` asserts; the
block was moved after that block's cleanup with a comment recording why. Separately, the plan's
`grep -F … | grep -qF --` assert is a `producer | early-exiting-consumer` under `set -o pipefail`,
the exact `AGENTS.md` prohibition, and was rewritten to capture-first.

**One plan prediction was wrong** (not a defect). Task 1 Step 6 predicted that removing gate 2 would
un-redden its rc assert; it does not, because gate 3 still rejects a regular-file path. The paired
message assert reddens instead — which is exactly why the plan authored a message assert beside
every rc assert.

**Task 4 was unplanned.** Change 0205 added a "Known limitation — that motivating use is not usable
yet (change #0206)" warning pointing forward at this change. This branch *is* 0206, so shipping the
warning intact would have documented the fix as absent. Cleared from `scripts/runners/opencode.md`,
`README.md`, and `docs/opencode/setup.md`.

## Follow-ups

Auto-captured this run (cap of 3 reached — capture is best-effort and every candidate above the
materiality bar was minted):

- **#0208** `fix` — runner-dispatch `--worktree` gate 3 proves repo containment, not worktree
  membership. Also carries the missing success-path test leg.
- **#0209** `fix` — the `--worktree` requirement covers `build-*` only, leaving `rebase-resolver`,
  `integration-repair`, and the `review-*` trio ungated.
- **#0210** `fix` — a valueless trailing flag hangs `runner-dispatch` forever instead of aborting
  (pre-existing across all five flags; this change added the fifth).

Not minted, recorded here instead: the residual-cwd-dependence and `<repo>`-ambiguity findings above
sit below the "would a human file this as its own change?" bar and are better folded into #0208 or a
future docs pass.
