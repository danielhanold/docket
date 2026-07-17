---
id: 62
slug: autonomous-finalize-merge-authorization
title: Autonomous finalize merge — clear the auto-mode Merge-Without-Review soft-deny
status: done
priority: low
created: 2026-07-11
updated: 2026-07-17
depends_on: []
related: [61]
adrs: [11, 42]
spec: docs/superpowers/specs/2026-07-16-autonomous-finalize-merge-authorization-design.md
plan: docs/superpowers/plans/2026-07-16-autonomous-finalize-merge-authorization.md
results: docs/results/2026-07-16-autonomous-finalize-merge-authorization-results.md
trivial: false
auto_groomable: false
branch: feat/autonomous-finalize-merge-authorization
pr: https://github.com/danielhanold/docket/pull/94
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-16-autonomous-finalize-merge-authorization-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-16-autonomous-finalize-merge-authorization-design.md) |
| Plan | [2026-07-16-autonomous-finalize-merge-authorization.md](https://github.com/danielhanold/docket/blob/feat/autonomous-finalize-merge-authorization/docs/superpowers/plans/2026-07-16-autonomous-finalize-merge-authorization.md) |
| Results | [2026-07-16-autonomous-finalize-merge-authorization-results.md](https://github.com/danielhanold/docket/blob/feat/autonomous-finalize-merge-authorization/docs/results/2026-07-16-autonomous-finalize-merge-authorization-results.md) |
| PR | [#94](https://github.com/danielhanold/docket/pull/94) |
| ADRs | [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md), [ADR-0042](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0042-auto-approve-consent-model.md) |
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
6. **Setup guide** in the repo-global `docs/` directory, linked from the root `README.md` — prerequisites (token scopes, repo setting), the one-time setup run, the knob, and the documented limitations.

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
- **2026-07-16** — Ran Task 1's go/no-go spike **attended** (human ran the headless arms in a fresh terminal; scratch PR #93; throwaway workflow `spike-approve-0062.yml`, installed and removed same day; `can_approve_pull_request_reviews` flipped true for the spike and restored to false). Claude Code **2.1.211**. Verdict: **GO — build Tasks 2–6.**
  - **Arm A (control, headless `claude -p --permission-mode auto`):** the "Merge Without Review" soft-deny **did not reproduce** — `permission_denials: []`, `gh pr merge --rebase` executed and failed only at GitHub's policy layer ("base branch policy prohibits the merge"). Re-ran as **Arm A′** with a prompt containing no merge wording at all (bare close-out shape): same result. Under 2.1.211 the merge classifier does not fire headless in either prompt shape — a behavior change since the 2.1.207 findings.
  - **Arm B (the question):** full finalize-shaped chain headless — `gh workflow run` dispatch → poll to `completed` → `reviewDecision: APPROVED` (bot review) → `gh pr merge 93 --rebase` **without `--admin`** → `state: MERGED`. Zero permission denials end-to-end. The Actions-bot approval satisfies branch protection's required review; the design's core claim is verified.
  - **Arm C (probe, non-gating):** terminal-publish-shaped direct push to `main` headless — **not denied**; push landed with only the informational protection-bypass notice. The "Git Push to Default Branch" arm also does not fire headless under 2.1.211. Task 4's publish-degradation path stays as version-defense but is currently dormant.
  - **Counterpoint finding (interactive):** the same afternoon, *this attended session's* classifier denied the `git commit` and push of the approval-granting workflow file (explicit-intent retry did not clear it; human ran both via `!`) and denied the `gh workflow run` dispatch (Self-Approval-shaped), yet later allowed the commit+push *removing* the workflow. Headless auto-mode and interactive auto-mode classifier behavior have diverged; the interactive classifier keys on granting approval capability, not on main-pushes per se. Record the CC version in the Task 5 ADR; classifier semantics are version-dependent and the spike re-opens cheaply.
  - Arm reports preserved verbatim in [the spike-transcripts doc](../../superpowers/specs/2026-07-16-autonomous-finalize-merge-authorization-spike-transcripts.md); to be carried into the results doc at close-out.
- **2026-07-16 (build claim)** — Claimed for implementation via `docket-implement-next`; **Task 1's go/no-go spike is DONE with a GO verdict** (attended, above), so this build covers **Tasks 2–6 only** — no product code exists yet and the spike is not re-run. Reconciled the design against current `main`: verified every integration point still matches the spec. (1) `scripts/docket-config.sh` — `terminal_publish` (a coordination-key-fenced, repo-committed-only `true|false` knob) is the exact precedent for the new `finalize.auto_approve` knob: read from `$CFG` only, added to the Stage-2c fence loop, `true|false`-validated, emitted as `FINALIZE_AUTO_APPROVE`. (2) `skills/docket-finalize-change/SKILL.md` — the auto_approve dispatch slots into *The rebase-retest merge gate* between step 5 (force-push) and step 6 (`gh pr merge`), so the Action approves the exact rebased SHA; `reviewDecision`/`require_pr_approval` are already referenced there and ADR-0011 governs the consent model the Task-5 ADR relates to. (3) `scripts/docket.sh` — the facade adds `setup-auto-approve` to `WRAPPED_OPS` (op name == helper basename), with matching `scripts/docket.md` + `scripts/setup-auto-approve.md` contract and sentinel-test coverage. (4) No `.github/`/`templates/` dir exists yet — `docket-approve.yml` is a new shipped asset the human-run setup script copies onto the integration branch. Related #61 is `done`; nothing done elsewhere overlaps or narrows scope. **This repo runs `terminal_publish: true`**, so the Arm-C publish-degradation path (Task 4) is live-relevant here. ADR (Task 5) must record the CC version the classifier behavior was pinned against (spike: 2.1.211).
