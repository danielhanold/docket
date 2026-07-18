---
id: 43
slug: retire-bot-auto-approval-zero-approvals-branch-protection
title: Retire bot auto-approval — branch protection with zero required approvals is the single-maintainer merge path
status: Accepted
date: 2026-07-18
supersedes: []
reverses: [42]
relates_to: [11]
change: 95
---

## Context

ADR-0042 (change 0062) built an opt-in bot-approval mechanism — a repo-controlled
`docket-approve.yml` GitHub Actions workflow that genuinely approves a PR with the built-in
`GITHUB_TOKEN`, so branch protection's required-review count is satisfied without `--admin`.
Its design depended on `docket-finalize-change` being able to dispatch that workflow
(`gh workflow run`) as the first step of the auto-approve chain.

That dispatch is classifier-blocked in practice. Observed 2026-07-18, **Claude Code headless
(non-interactive) mode**, harness version not separately pinned beyond "the 2026-07-18 session" —
during the change-0088 finalize, the `gh workflow run` bot-approve dispatch was soft-denied by
the auto-mode classifier and did not clear on retry. This is a **headless-mode** observation; it
does not describe interactive Claude Code behavior on the same repo, which has separately been
seen to diverge (the finalize-merge classifier block note documents conversational retry clearing
a *different* soft-deny, `gh pr merge`, interactively — the workflow-dispatch soft-deny did not
clear the same way). Classifier behavior is mode- and version-scoped: an unscoped claim here would
be read later as universal, and would be wrong. A chain whose very first step (the dispatch) is
denied can never complete, regardless of how sound the rest of the chain is — the 2.1.211 spike
underlying ADR-0042 validated the chain in an **attended** session (scratch PR #93), which does
not carry over to the headless case this dispatch actually needs to run in.

Separately, and independent of the classifier finding: main branch protection was changed
2026-07-18 to require a PR but **zero** required approvals (`required_approving_review_count: 0`,
`enforce_admins: false`), verified end-to-end on PR #100 — `gh pr merge --rebase` lands with no
bot, no dispatch, and no `--admin`. That branch-protection change was itself prompted by a
recurring pattern across three prior changes of building in-repo workaround machinery for a
merge/approval wall that turned out to be a relaxable, human-controlled GitHub setting.

## Decision

Retire change 0062's bot-approval mechanism entirely: the `docket-approve.yml` workflow, its
template, the `setup-auto-approve.sh` installer and its docs, the `finalize.auto_approve` config
key (parse, validate, and the `FINALIZE_AUTO_APPROVE` resolver export), and the finalize gate step
that dispatched it.

The single-maintainer hands-off merge path is **branch-protection configuration, not pipeline
machinery**: `required_approving_review_count: 0` and `enforce_admins: false` on the integration
branch, followed by a plain `gh pr merge --rebase` — no `--admin`, no bot, no workflow dispatch.
`docket-finalize-change`'s gate becomes a 6-step flow with merge as step 6.

Removing `auto_approve` restores `require_pr_approval: true` to its ADR-0011 meaning: when set, a
required review is a real human review, full stop. There is no longer a bot-satisfiable
in-between state where the flag reads as "approval required" but is satisfiable by docket's own
pipeline.

## Consequences

- **Enables:** a genuinely working one-swoop headless finalize for the single-maintainer default
  (docket's primary use case), with far less machinery than ADR-0042's design — no Actions
  workflow to install, no `can_approve_pull_request_reviews` repo setting to flip, no dispatch to
  poll, no dispatch-classifier failure mode to defend against.
- **Costs:** under `required_approving_review_count: 0`, the repo merges with **no recorded
  review at all** — not even a bot one. ADR-0042's "docket's pipeline authorized the merge" record
  is gone; there is simply no approval artifact. This is accepted and explicit as the solo-maintainer
  trade-off: the correctness gate (rebase + re-test) still runs regardless of approval count, and a
  human still opened and can still inspect the PR before finalize touches it. Team repos that want
  a real human bar keep `require_pr_approval: true` with `required_approving_review_count` at 1 or
  more and a real human reviewer — ADR-0042's bot-satisfiable middle ground is no longer offered as
  an option for that case.
- **Given up:** the property ADR-0042 was built to preserve — "a required review is both satisfied
  *and* recorded as an approval" — is dropped rather than repaired. It was dropped because its
  prerequisite (the workflow dispatch) does not reliably run headless, not because the property was
  judged undesirable in principle.
- **Version dependence, again:** this is an empirical decision, not a permanent contract, for the
  same reason ADR-0042 was — classifier and branch-protection behavior are mode- and
  version-scoped. A future harness change that makes the dispatch reliably clear headless would not
  by itself justify reviving ADR-0042's mechanism, since the zero-approvals path is simpler
  regardless; reviving it would need its own new decision, not a reversal of this one.
