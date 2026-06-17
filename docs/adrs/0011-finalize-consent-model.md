---
id: 11
slug: finalize-consent-model
title: Finalize consent model — ambiguity-only prompt + `require_pr_approval` policy gate
status: Accepted
date: 2026-06-17
supersedes: []
reverses: []
relates_to: [10]
change: 21
---

## Context

`docket-finalize-change`'s no-arg (auto-detect) path unconditionally prompted before merging a mergeable-but-unmerged PR ("merging is a deliberate act"). The prompt's real job is guarding the **bulk-merge blast radius** — a no-arg run can match several `implemented` changes at once and must not blanket-merge them. But in the common case of exactly one obvious target the human deliberately invoked finalize on, the prompt is pure friction. The friction is sharpest for docket's **primary use case, a single human**, who pushes their own PRs and therefore cannot approve them on GitHub at all — the PR is always unapproved, yet the human running finalize *is* the authorization. There was also no repo-level way to require human sign-off on the auto-detect path for teams that do want it. ADR-0010's gate validates *correctness* (rebase + re-test); nothing validated *who authorized the merge*.

## Decision

A two-part consent model for `docket-finalize-change`, both governing only the **auto-detect** path:

1. **Ambiguity-only prompting.** No-arg finalize runs the full flow (gate + merge + finalize) with **no prompt** when exactly **one** eligible candidate exists; it prompts **only when more than one** eligible candidate would be merged. "Eligible" = git-mergeable AND (`require_pr_approval: false` OR approved); the ambiguity count is over eligible candidates only. Selection's "surface, do not merge" covers states the rebase-retest gate can't act on (draft/closed/flatly un-mergeable); rebaseable conflicts stay the gate's job.

2. **`finalize.require_pr_approval` repo policy knob (default `false`),** nested beside `gate:`/`test_command:`. `gate` validates correctness; `require_pr_approval` validates human sign-off. Default `false` ⇒ approval is never a selection-time blocker (single-human-friendly). `true` ⇒ the auto-detect path refuses to merge an unapproved PR (`reviewDecision != APPROVED`), surfacing it instead.

3. **Explicit id always overrides the approval gate.** Passing an explicit `docket-finalize-change <id>` is itself the human authorization, so it proceeds even on an unapproved PR under `require_pr_approval: true`; it never prompts. The rebase-retest correctness gate still runs regardless of which proof was used.

The principle it reduces to: **`require_pr_approval` ensures a human authorized the merge — on the auto-detect path that proof is a GitHub approval; an explicit id is that proof by another means. Correctness is checked regardless.**

## Consequences

- **Enables:** zero-friction single-target finalize for the single-human default; opt-in strict approval for teams; the bulk-merge blast-radius guard is preserved exactly where it matters (>1 eligible target still prompts).
- **Costs / trade-offs:** the no-arg path will now merge a single unapproved-but-mergeable PR silently under the default `false` — acceptable because invoking finalize is itself a deliberate act and the correctness gate still runs; a team that wants a hard approval bar sets `require_pr_approval: true`. The explicit-id override means an unapproved PR can always be merged by being explicit about it — a deliberate, logged act, not reachable by a bare no-arg run.
- **Scope:** behavior + docs only (no change to the rebase-retest gate/CI logic, no `--yes`/`all` bypass flag, no kill-path/terminal-publish change). The knob is documented in finalize's own SKILL.md (following `gate`/`test_command`'s precedent), with a commented entry in the repo `.docket.yml` for discoverability.

Note for records: the change 0021 spec's §3 stated the convention's `.docket.yml` example "does not enumerate finalize:"; in fact the convention example lists `gate`/`test_command` but not the new `require_pr_approval` — a deliberate scope call (finalize owns its config doc), flagged as a documentation follow-up for the human at the merge gate. This does not affect the decision above.
