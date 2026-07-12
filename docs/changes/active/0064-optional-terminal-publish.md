---
id: 64
slug: optional-terminal-publish
title: Optional terminal-publish — per-repo opt-out to keep metadata on docket
status: proposed
priority: medium
created: 2026-07-12
updated: 2026-07-12
depends_on: []
related: [2, 26, 40]
adrs: [12, 19]
spec: docs/superpowers/specs/2026-07-12-optional-terminal-publish-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-12-optional-terminal-publish-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-12-optional-terminal-publish-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md) |
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
