---
id: 64
slug: optional-terminal-publish
title: Optional terminal-publish — per-repo opt-out to keep metadata on docket
status: in-progress
priority: medium
created: 2026-07-12
updated: 2026-07-13
depends_on: []
related: [2, 26, 40]
adrs: [12, 19, 27]
spec: docs/superpowers/specs/2026-07-12-optional-terminal-publish-design.md
plan: docs/superpowers/plans/2026-07-12-optional-terminal-publish.md
results:
trivial: false
auto_groomable:
branch: feat/optional-terminal-publish
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-12-optional-terminal-publish-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-12-optional-terminal-publish-design.md) |
| Plan | [2026-07-12-optional-terminal-publish.md](https://github.com/danielhanold/docket/blob/feat/optional-terminal-publish/docs/superpowers/plans/2026-07-12-optional-terminal-publish.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0027](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0027-terminal-publish-repo-scoped-script-gated.md) |
<!-- docket:artifacts:end -->

## Why

Terminal-publish copies a closed change's metadata record — the archived change file, its `spec:`,
and its `Accepted` ADRs — from `origin/docket` onto the integration branch via a direct commit
(`git checkout origin/docket -- <paths>`), never a PR. In private repos where direct commits to the
integration branch are tolerable, that is the desired behavior. But in repos where the integration
branch is `main` and every merge is expected to go through a pull request, this auto-copy fights the
workflow: docket writes to `main` outside the PR gate.

The code a change produces is unaffected — the feature branch's code, `plan:`, and `results:` reach
the integration branch through the normal PR merge. Only the metadata record is force-copied. So the
gap is narrow: give a repo a way to suppress that metadata copy and keep every change file, spec,
and ADR on `docket`, while PRs continue to carry the code.

## What changes

Add a per-repo `terminal_publish` knob (built-in default `true`, so existing repos are unchanged).
Setting `terminal_publish: false` in a repo's committed `.docket.yml` makes the terminal-publish
step a no-op: the change file, spec, Accepted ADRs, and the integration-branch ADR-index refresh all
stay on `docket` only. The rest of the close-out sequence (archive, `## Artifacts` re-render,
feature-branch cleanup, board refresh) is unchanged.

The knob is **per-repo-only** (coordination-key fenced, like the `github` board surface): honored
only in `.docket.yml`, warned-and-ignored in the global config and machine-local `.docket.local.yml`
— because the autonomous `docket-status` merge sweep can run headless where those files don't exist,
so the policy must live in the committed repo file to hold for every agent.

The gate lives in `terminal-publish.sh` (a new `--enabled <true|false>` flag), which already
self-guards to a no-op in `main`-mode — one guard covers all four close-out drivers with no change
to skill bodies. `docket-config.sh` reads the leaf, applies the fence, and emits `TERMINAL_PUBLISH`.
Design detail, testing, and the component breakdown live in the linked spec.

The gate sits **before the `--id`/`--adr` mode dispatch**, so it also covers `docket-adr`'s
standalone ADR publish (on acceptance, and on a `Superseded`/`Reversed`/`Deprecated` status flip) —
the second path that commits metadata straight to the integration branch. Both `docket-adr` call
sites pass the flag. Without this, ADRs would keep landing on `main` under `terminal_publish: false`
and the knob would not deliver its promise (folded in at reconcile — see the log below).

## Out of scope

- Per-artifact granularity (e.g. suppress the change file but still publish ADRs) — all-or-nothing.
- Any change to how code / plans / results reach the integration branch (the PR flow is untouched).
- A retroactive un-publish of records already copied to the integration branch by prior runs.
- Making the knob settable from the global or machine-local config layers (deliberately fenced out).

## Open questions

<!-- None outstanding; design settled in brainstorm. The produced ADR (fence classification +
     conditional-publish rule) is authored at build time via docket-adr, not pre-minted here. -->

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-12 — reconciled before build (docket-implement-next)

Re-read the change + spec against `related` (2, 26, 40), the cited ADRs (12, 19), the recently
archived changes, and current code (`scripts/terminal-publish.sh`, `scripts/docket-config.sh`,
`skills/docket-convention/references/terminal-close-out.md`, `skills/docket-adr/SKILL.md`).

**Scope widened — the ADR publish path.** The spec accounted for only one of
`terminal-publish.sh`'s two publish shapes. The script is also the executor of `docket-adr`'s
standalone `--adr` publish (`skills/docket-adr/SKILL.md:57` on acceptance, `:65` on a status flip),
which likewise commits metadata directly onto the integration branch — exactly the write this change
exists to suppress. Gating only the close-out (`--id`) call sites would have left ADRs landing on
`main` under `terminal_publish: false`, contradicting this change's own promise that Accepted ADRs
stay on `docket` only. Folded in: the guard is placed **before the `--id`/`--adr` mode dispatch** (so
one guard still covers everything, per the spec's original single-guard intent), and both
`docket-adr` call sites now pass the flag. (Shipped form: the skill/reference prose passes the
`--enabled <terminal_publish>` placeholder, matching the `<integration_branch>`/`<changes_dir>`
idiom of the same code blocks — and fail-closed, since an unsubstituted placeholder kills the
script; the executable call site in `docket-status.sh` passes `--enabled "${TERMINAL_PUBLISH:-true}"`.)
Spec updated: new decision 5, a new
"Affected" bullet, a `docket-adr` mechanism section, and two new test cases (an `--adr --enabled
false` no-op, and a structural check that every call site passes the flag).

**Design otherwise intact.** The core decisions — built-in default `true`, coordination-key fence
(per-repo-only), all-or-nothing suppression, guard in the script rather than the skill bodies — all
still hold against current code. `docket-config.sh`'s Stage-2c fence loop and `--export` emit are
exactly the seams the spec assumed; `terminal-publish.sh`'s existing `main`-mode guard is where the
new guard slots in beside.

**Noted, not fixed (out of scope → follow-up).** `terminal-close-out.md`'s preamble still says the
kill callers "are still governed by their own skill bodies … until changes 0054/0055 rewire them
onto this file". 0054 and 0055 are both `done` (archived 2026-07-11) and all four drivers now route
through the reference. Since this change edits that file's step 3 and the stale sentence directly
concerns which callers the new gate covers, the preamble's caller-coverage claim is corrected in
passing; no broader rewrite of the reference is in scope.

### 2026-07-13 — re-reconciled on resume (docket-implement-next)

The prior run crashed after the build completed but before review/PR. Per the resume-safety guard,
reconcile re-ran because `origin/main` advanced since the 2026-07-12 pass. **Scope confirmed
unchanged — no edit to this change or its spec.**

**What moved.** `origin/main` gained exactly one change: **0065** (agent-model-pinning docs, PR #74,
merged 2026-07-12) — docs-only (`README.md`, `skills/docket-convention/references/agent-layer.md`,
`tests/test_skill_fork_dispatch.sh`). It is orthogonal to `terminal_publish`: it touches neither the
publish path nor the config resolver. Verified directly — **no commit on `origin/main` since
2026-07-11 touches any core file this change modifies** (`terminal-publish.sh`, `docket-config.sh`,
`docket-status.sh`, `terminal-close-out.md`, `docket-adr/SKILL.md`). The feature branch was rebased
onto `origin/main` (base `ec1846e`) and the full suite is green there, so the design holds against
current code as built.

**New ADRs since the last pass: 0025** (worktrees disable git hooks) and **0026** (fork-dispatch
opacity) — both unrelated to the publish path. The cited ADRs (0012 script-vs-model boundary, 0019
fence classification) are unchanged and still the governing precedents; the ADR this change produces
is authored at review time via `docket-adr`.

**Merge-order note (not a scope change).** Two changes sit `implemented` with open, unmerged PRs:
**0044** (PR #69) and **0066** (PR #73). 0044's PR overlaps this branch's files — `README.md`,
`scripts/docket-config.{sh,md}`, `skills/docket-convention/SKILL.md`, `tests/test_docket_config.sh`
— so whichever merges second will need a rebase. That is a merge-time concern the finalize gate
(rebase-onto-base + re-test) already owns; it is not a design conflict — 0044 adds a `skills:`-layer
model knob, this change adds an orthogonal publish knob, and neither redefines the other's seam.
0066 (auto-groom critic) does not overlap the publish path at all. New stub **0067** (learnings
promotion) is unrelated.
