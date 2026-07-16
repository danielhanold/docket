---
id: 62
slug: autonomous-finalize-merge-authorization
title: Autonomous finalize merge — clear the auto-mode Merge-Without-Review soft-deny
status: proposed
priority: low
created: 2026-07-11
updated: 2026-07-16
depends_on: []
related: [61]
adrs: [11]
spec: docs/superpowers/specs/2026-07-16-autonomous-finalize-merge-authorization-design.md
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-16-autonomous-finalize-merge-authorization-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-16-autonomous-finalize-merge-authorization-design.md) |
| ADRs | [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md) |
<!-- docket:artifacts:end -->

## Why

`docket-finalize-change` can't merge headless on Claude Code. Two independent walls: the auto-mode **"Merge Without Review"** classifier soft-denies `gh pr merge` on an unreviewed PR (`permissions.allow` cannot clear a soft-deny — spike-proven; only explicit human merge intent in the live conversation does, which a headless run cannot express); and on GitHub itself a solo maintainer — docket's primary use case — cannot approve their own PR, so branch protection's required review is structurally unsatisfiable and merges need `--rebase --admin`. This is why change 0061 had to exclude finalize from `context: fork`.

The first design (an `autoMode.allow` bypass in the repo's `.claude/settings.local.json`) was **disproven by its own mandated build-time spike** (2026-07-13): Claude Code 2.1.207 honors `autoMode` only from user-level `~/.claude/settings.json` — machine-global, the inverse of the intended repo-bounded safety envelope. The old spec is preserved with a `⛔ DISPROVEN` banner and its 7 verified findings are carried by the new design.

The re-groomed direction (2026-07-16) attacks the problem from the GitHub side instead of bypassing the guard: a repo-controlled GitHub Actions workflow **genuinely approves** the PR (the established Dependabot auto-approve pattern — a `github-actions[bot]` review counts toward required approvals), so branch protection is satisfied without `--admin` and the merge is no longer "without review" — giving the classifier nothing to fire on. Real GitHub state instead of talking the guard out of firing; the classifier half of that claim is unverified and gates the build.

## What changes

Design: [2026-07-16 spec](../../superpowers/specs/2026-07-16-autonomous-finalize-merge-authorization-design.md). Docket-shipped and opt-in per repo:

1. **Go/no-go spike first** (no product code before it): headless auto-mode probe of whether "Merge Without Review" still fires on a genuinely APPROVED PR. Still fires → stop, clear `spec:`, back to needs-brainstorm (the untested `--settings` route is the remaining candidate). A third, non-gating arm probes the terminal-publish push denial to shape the degradation wording.
2. **`docket-approve.yml` workflow template** — `workflow_dispatch` with a `pr` input, `pull-requests: write`, eligibility guards (open, non-draft, non-fork, `feat/*` head), then `gh pr review --approve` with `GITHUB_TOKEN`.
3. **`setup-auto-approve.sh`** (human-attended, one-time, via the `docket.sh` facade) — installs the workflow onto the integration branch and flips the repo's "Allow Actions to create and approve pull requests" setting via `gh api`.
4. **`finalize.auto_approve` knob** (default `false`; coordination-key fenced, per-repo-only) — when `true`, finalize dispatches the workflow *after* the rebase-retest gate's force-push (so the approval covers the merged SHA), polls, verifies `reviewDecision: APPROVED`, merges **without** `--admin`; any failure is abort-and-report, never an `--admin` fallback. On `terminal_publish: true` repos a headless publish denial degrades to a surfaced manual follow-up, never a failed run.
5. **ADR** (relates to ADR-0011): under `auto_approve`, approval proves "docket's pipeline signed off," not human review — `require_pr_approval: true` is incompatible-in-spirit; `Self-Approval` and the sensitive-content push arm survive untouched.

`auto_groomable: false` was deliberate: granting standing permission to merge unreviewed code is a safety-policy decision — this stub was groomed by a human (2026-07-16).

## Out of scope

- The autonomous **driver/loop** that invokes finalize headless — this change enables the capability; the driver is separate work.
- The context:fork parity fix itself (change 0061).
- Harnesses other than Claude Code (Cursor has no such classifier; the GitHub half works there for free).
- CODEOWNERS-protected repos (a bot approval cannot satisfy them; documented limitation).
- The `--settings <file>` launch-flag route — recorded as the fallback candidate if the spike no-gos.
- Any redesign of terminal-publish (e.g. records-via-PR) to duck the push classifier.

## Open questions

<!-- Resolved 2026-07-16 during re-groom; see the spec's "Decisions taken during grooming". -->

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- **2026-07-13** — Ran the spec's mandated build-time spike *before* any code. It **disproved fact (3)** (project-level `.claude/settings.local.json` is not honored for `autoMode`), which the whole design rested on. Followed the spec's own instruction (*"stop and reconvene"*): cleared `spec:`, set `auto_groomable: false`, returned the change to **needs-brainstorm**. Nothing was claimed, branched, or built; no code exists. Old spec retained with a `⛔ DISPROVEN` banner. Full probe transcript and the two false conclusions that preceded the real one are recorded in that banner and in the spec body.
