---
id: 42
slug: auto-approve-consent-model
title: Auto-approve consent model — a bot approval proves docket's pipeline signed off, not human review
status: Reversed by ADR-0043
date: 2026-07-16
supersedes: []
reverses: []
relates_to: [11]
change: 62
---

## Context

`docket-finalize-change` could not merge headless on Claude Code. Two independent walls: the auto-mode "Merge Without Review" classifier soft-denied `gh pr merge` on an unreviewed PR (`permissions.allow` cannot clear a soft-deny), and a solo maintainer — docket's primary use case — cannot approve their own PR, so branch protection's required review was structurally unsatisfiable and merges needed `--rebase --admin`.

Change 0062 adds an opt-in `finalize.auto_approve` knob. A repo-controlled GitHub Actions workflow (`docket-approve.yml`) genuinely **approves** the PR with the built-in `GITHUB_TOKEN` (the established Dependabot auto-approve pattern — a `github-actions[bot]` review counts toward required approvals, though not CODEOWNERS), so branch protection is satisfied **without** `--admin` and the merge is no longer "without review". finalize dispatches the workflow **after** its rebase-retest gate's force-push, so the approval always covers the exact SHA being merged.

A go/no-go spike (attended, Claude Code **2.1.211**, scratch PR #93) validated the full headless chain end-to-end: `gh workflow run` dispatch → poll to completion → `reviewDecision: APPROVED` (bot review) → `gh pr merge --rebase` **without `--admin`** → `MERGED`, with zero permission denials. Under 2.1.211 the "Merge Without Review" classifier did not fire headless at all — a behavior change from the 2.1.207 findings — but the design's value is the GitHub-side win (no `--admin`) regardless of classifier version.

This decision relates to ADR-0011 (the finalize consent model: ambiguity-only prompting + the `require_pr_approval` policy gate). ADR-0011's principle was **"`require_pr_approval` ensures a human authorized the merge"**; `auto_approve` is where that invariant is deliberately relaxed.

## Decision

1. **A bot approval proves "docket's pipeline signed off," not human review.** Under `auto_approve: true`, the GitHub approval attests that docket's own pipeline — the resolved review step **plus** the rebase-retest correctness gate — ran and passed. It is not a human sign-off. Consequently `require_pr_approval: true` combined with `auto_approve: true` is **legal but the approval it requires is satisfiable by the bot's own review**: it no longer guarantees a human authorized the merge. ADR-0011's "a human authorized the merge" invariant is relaxed here to "docket's pipeline authorized the merge." finalize's SKILL.md and `docs/auto-approve-setup.md` cross-reference this ADR at the point they state the interaction.

2. **Any auto-approve failure is abort-and-report; NEVER `--admin`.** If the dispatch is rejected, the run fails or times out, or the approval never materializes, finalize leaves the PR open and surfaces the reason (recorded as a PR comment). It must **never** fall back to `--admin` — that would silently reinstate the two-party-review bypass this design retires.

3. **What survives untouched.** Claude Code's `Self-Approval` classifier and the sensitive-content arm (a) of `Git Push to Default Branch` are unaffected. The Action can grant a *review*; it can neither manufacture human judgment nor push a secret. The capability is a narrow, auditable "satisfy branch-protection's required-review count," not a general merge-guard bypass.

4. **Off by default, triple-gated, fenced.** Auto-approval cannot occur unless three independent human-gated conditions all hold: the `docket-approve.yml` workflow is installed on the integration branch, the repo Actions setting `can_approve_pull_request_reviews=true` is flipped, and `finalize.auto_approve: true` is set in the **committed** `.docket.yml`. The knob is **coordination-key fenced** (repo-committed only; a value in global config or `.docket.local.yml` is warned-and-ignored, and any non-`true`/`false` value fails closed), so no machine-scoped setting can enable it. CODEOWNERS-protected repos are unsupported (a bot approval cannot satisfy them).

## Consequences

- **Enables:** headless / autonomous `docket-finalize-change` for the single-maintainer default (docket's primary use case) without `--admin` and without a structurally-unsatisfiable required review — the capability change 0061 had to exclude finalize from `context: fork` over.
- **Costs / trade-offs:** approval semantics weaken from "a human authorized" to "docket's pipeline authorized" — deliberate, opt-in, and documented. `require_pr_approval: true` becomes bot-satisfiable under `auto_approve: true`. A team that wants a hard human bar must not enable `auto_approve`. The eligibility guards (open, non-draft, non-fork, `feat/*` head) and the correctness gate (rebase + re-test) still run regardless.
- **Version dependence:** classifier behavior is Claude-Code-version-dependent. This decision pins the version probed — **2.1.211** — so a future release that changes classifier semantics re-opens the spike's Arm B cheaply (re-run it). The spike also probed the terminal-publish push arm (Arm C): under 2.1.211 a direct records-push to the integration branch was **not** denied headless, so finalize's publish-degradation path (complete close-out and surface a manual `terminal-publish` follow-up on a denied headless push) is currently dormant version-defense, not a live path.
- **Scope:** this ADR records the consent decision only. It does not change ADR-0011's auto-detect prompt/`require_pr_approval` mechanics, the rebase-retest gate, or terminal-publish. The driver/loop that would invoke finalize headless is separate work (0062 enables the capability; it does not ship the driver).
