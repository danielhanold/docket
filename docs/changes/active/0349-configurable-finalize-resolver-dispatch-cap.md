---
id: 349
slug: configurable-finalize-resolver-dispatch-cap
title: Make the finalize rebase-resolver dispatch cap configurable
status: proposed
priority: medium
type: feat
created: 2026-08-26
updated: 2026-08-26
depends_on: []
stacked_on:
related: [334]
discovered_from: [334]
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
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`docket-finalize-change`'s rebase-resolver loop is hard-capped at **two** resolver dispatches
(SKILL.md: *"Resolver loop (skill-enforced ≤2 attempts)"*). The cap exists so a genuinely gnarly
rebase escalates to a human instead of looping forever — a reasonable default. But the count is the
number of *conflicting commits* the resolver can clear, and each `rebase-continue` can surface the
next commit's conflict. So a feature branch whose rebase onto its base carries **3+ conflicting
commits can never complete** under the cap and always halts for a human — even when every conflict is
mechanically resolvable.

Observed finalizing change **334**: the branch had three separable conflict points (the
`runtime-budgets` `EXPECTED_TOTAL` re-sum recurring across commits, and the embedded-asset
`manifest.json` digest). Two resolver dispatches cleared two of them; the third surfaced on a later
commit with the budget spent, so finalize halted despite every conflict being a routine
re-sum / regenerate-derived-artifact. A human then had to rebase by hand. Repos with longer-lived
branches will hit this routinely.

Making the cap configurable (e.g. a `finalize.resolver_max_attempts` knob in `.docket.yml`, default
2) lets a repo raise it where the conflicts are known-mechanical, without weakening the
escalate-to-human default everywhere.

## What changes

- Add a `finalize` config knob for the resolver dispatch cap (default preserves today's `2`),
  resolved through `docket-config.sh --export` like the other `finalize.*` fields.
- Have `docket-finalize-change`'s resolver loop read the resolved value instead of the literal `2`.
- Decide the escalation semantics at the new ceiling (still `halted` on exhaustion; the abort +
  `## Finalize blocked` path is unchanged).

## Out of scope

- Redesigning the resolver contract so it drives the rebase to completion itself (one dispatch, N
  conflicts). That is a larger architectural change; this stub is only about making the existing
  bounded-dispatch count configurable.
- The full merge/gate flow beyond the resolver loop.

## Open questions

- Knob name, shape, and whether it is a plain integer cap or also allows "unbounded / until stuck".
- Coordination-key fencing: is a per-machine override safe here, or is this a per-repo-only field
  (it changes how far an autonomous finalize will grind before halting)?
- **Related doc drift to reconcile in the same design:** the installed
  `.claude/agents/docket-rebase-resolver` description says the resolver *"continues the rebase to
  completion; never runs tests,"* while the skill body says the **skill** drives `rebase-continue`
  and the resolver only *returns a structured report; never runs Git rebase mechanics.* The
  docket-source `agents/docket-rebase-resolver.md` matches the skill body, so the `.claude/` copy is
  stale — but the contradiction is worth resolving deliberately as part of deciding the cap
  semantics (bounded report-and-continue vs. resolver-drives-to-completion).
- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: the cap is still a skill literal (`skills/docket-finalize-change/SKILL.md`, `≤2 attempts`), and `docket-config.sh --export` is deleted. Put the knob in the Go config schema (`internal/config/schema.go` / `defaults.go`, a `finalize.*` field exported via `repository prepare`) and have `finalize rebase-continue --attempt` enforce it. The `.claude/agents` description-drift open question is likely stale (wrappers are generated from `agents/*.md`).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
