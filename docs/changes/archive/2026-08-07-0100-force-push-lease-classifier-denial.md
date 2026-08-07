---
id: 100
slug: force-push-lease-classifier-denial
title: Force-push-with-lease denied by the auto-mode classifier — unblock finalize's merge gate
status: killed
priority: medium
created: 2026-07-19
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [91]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Discovered while finalizing #0091 (2026-07-19). The rebase-retest merge gate's step 5 is
`git push --force-with-lease` — mandatory whenever the gate rebased, which is the common case for
any PR that has fallen behind `origin/main`. On that run the push was **denied by the Claude Code
auto-mode permission classifier**, with the standard "Blocked by classifier" refusal.

The attended run recovered trivially: the human re-ran the same command via `! cmd` and the gate
continued to a clean merge. That escape hatch does not exist on the path the gate was built for.
An **autonomous** finalize (the `docket-finalize-change` wrapper, a `/loop` drain, cron) has no
human to type `!`, so a classifier denial at step 5 lands on abort-and-report → `halted`, leaving
a rebased-but-unpushed branch, an open PR, and a change stuck at `implemented`. A drain that hits
this on its first rebased candidate stops there, with the suite already run and thrown away.

This is adjacent to but distinct from the known merge-step denials (change 0095, ADR-0043): those
were about `gh pr merge` and the retired bot-approval dispatch. This one is a plain `git push`
against a feature branch, denied for its `--force` shape rather than for anything about branch
protection.

## What changes

**Establish the cheap fix before designing any machinery.** The denial message names the remedy
itself — a Bash permission rule in the user's settings. Per the
`relax-the-policy-before-building-the-workaround` finding (#0095, where three changes of in-repo
workarounds were built before anyone checked whether the wall was human-controlled policy), the
first question is whether a narrowly-scoped allow rule in **user-level** `~/.claude/settings.json`
covers this permanently. Note the trap from `docket-automode-not-read-from-project-settings`:
`autoMode` rules are honored only at user level, never in a repo's `.claude/settings.local.json`,
so where the rule lands is load-bearing and must be verified by a real A/B, not by the docs.

Only if that fails should this change consider alternatives, roughly in ascending cost:

- Scope the allowlist as tightly as the gate actually needs (`--force-with-lease` on `feat/*`
  only), so the grant is not a blanket force-push permission.
- Have the gate detect the denial and degrade honestly — surface the exact command for a human to
  run, mark `## Finalize blocked` with this specific reason, and exit `halted` without discarding
  the green suite result, so a re-run after the human's push resumes rather than re-testing.
- Avoid needing the force-push at all on the common path (e.g. merge via a strategy that does not
  require the rebased branch to be pushed first) — likely a larger redesign of the gate, and the
  reason this is a stub rather than a patch.

Decide, too, whether this reason deserves naming in the abort-and-report set in
`docket-finalize-change`, which today lists a classifier denial only for *the merge itself*.

## Out of scope

- Changing the gate's rebase-then-retest design, or whether it force-pushes at all, unless the
  investigation above concludes that is the actual fix.
- Anything about `gh pr merge` denials or branch protection — settled by #0095 / ADR-0043.

## Open questions

- Is the denial deterministic for this command shape, or was it a one-off? Nothing here should be
  built on a single observation — reproduce it before designing.
- Is the classifier keying on `--force-with-lease`, on `push --force*` generally, or on something
  about the refspec? The narrowest safe allow rule depends on the answer.
- Does the denial behave differently headless vs attended? `harness-behavior-is-mode-and-version-scoped`
  (#0062, #0085, #0095) says assume it does until measured, and that any finding here is scoped to
  a Claude Code version rather than being a durable contract.

## Triage note (2026-07-26, change 0124)

**Materially narrowed by change 0128 — re-scope before grooming.** 0128 shipped the generic remedy
as a convention rule at `skills/docket-convention/SKILL.md:84`: on host sandbox or permission denial
of a required `docket.sh` facade or direct Git command, retry the exact command once through the
harness's native approval/escalation mechanism, then follow the caller's existing failure posture.
That covers the step-5 `--force-with-lease` denial as a class, which was this stub's primary ask.

Two residuals survive and are what remains worth grooming:

1. **The abort-and-report set is still incomplete.** `skills/docket-finalize-change/SKILL.md:166`
   names a classifier denial only for *the merge itself*; the push denial is not in the set. This
   is the question already posed at the end of `## What changes`, now the main event.
2. **The allow-rule option is pointed the other way, not merely unanswered.**
   `scripts/ensure-claude-settings.sh:10` and `README.md:803` state that force-pushes stay
   deliberately guarded; the only granted rule is `Bash(git -C * push origin HEAD:main)`, verified
   in `docs/results/2026-06-19-claude-settings-publish-permission-results.md:15` to *not* match a
   force-push. So "just add an allow rule" would reverse a stated posture and needs to be argued,
   not assumed.

Kept in the human queue: residual 2 is a security-posture reversal, which is a human call.

## Why killed

Consolidated into #0260 at the 2026-08-07 backlog triage: narrowed long before — the generic remedy shipped as the convention's harness-native-recovery section; the surviving residual (the push-denial sentence in gate-failure.md) lands with #0139's posture work. The settings-posture reversal stays rejected.
