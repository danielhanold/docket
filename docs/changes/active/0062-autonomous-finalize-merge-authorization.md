---
id: 62
slug: autonomous-finalize-merge-authorization
title: Autonomous finalize merge — clear the auto-mode Merge-Without-Review soft-deny
status: proposed
priority: low
created: 2026-07-11
updated: 2026-07-11
depends_on: []
related: [61]
adrs: [11]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md) |
<!-- docket:artifacts:end -->

## Why

`docket-finalize-change` can't run headless/autonomous on Claude Code because its `gh pr merge` is soft-denied by the auto-mode **"Merge Without Review"** classifier (active under `permissions.defaultMode: auto`), which an autonomous run can't clear — there's no human in the subagent to express merge intent. This is why change 0061 (context:fork parity) had to **exclude** finalize from forking, and it's the reason interactive finalize still hard-stops at the merge gate until the human states intent and retries.

A prior attempt to fix this concluded "the allow-rule doesn't work" — but that test used `permissions.allow` (`Bash(gh pr merge:*)`), which runs **before** the classifier and cannot clear a soft_deny. Docs research (2026-07-11) found the **untried** lever: the separate `autoMode.allow` field **inside** the classifier is documented to override matching `soft_deny` rules. That would let an autonomous finalize merge — and, combined with 0061's mechanism, would make finalize **forkable** (model-pinned + autonomous). But this is a real **safety-policy** decision (standing permission to merge unreviewed PRs), not a mechanical config tweak, so it belongs in its own change.

## What changes

Design and (if chosen) implement a way for autonomous/headless `docket-finalize-change` to clear the "Merge Without Review" soft-deny. Candidate levers to weigh in brainstorm:

- **`autoMode.allow` prose entry** — documented to override the soft_deny. Placement caveat: the classifier does **not** read `autoMode` from committed `.claude/settings.json`; it reads `~/.claude/settings.json`, `.claude/settings.local.json` (docket already manages this via its gitignore block), managed settings, or `--settings`. So `sync-agents.sh`/install could write it to `.docket`-managed `.claude/settings.local.json` per-machine.
- **Satisfy, don't bypass** — the rule fires only for PRs *no human has approved*; require a real GitHub approval (`finalize.require_pr_approval: true`, ADR-0011) so it never triggers. Safest; respects the rule's intent.
- **`autoMode.soft_deny` customization** — remove/narrow the built-in rule (blunter; must splice `"$defaults"`).
- Validate any custom rule with `claude auto-mode critique` / `claude auto-mode config`.

If a lever is adopted, follow-on: let `docket-finalize-change` (and its wrapper) become fork/dispatch-eligible.

## Out of scope

- The context:fork parity fix itself (change 0061).
- Any harness other than Claude Code (Cursor has no such classifier).

## Open questions

- Is autonomous unreviewed auto-merge acceptable at all, or should docket only ever *satisfy* the classifier (require approval) and never bypass it? This is the central policy call.
- Empirically confirm an `autoMode.allow` prose entry actually clears the "Merge Without Review" soft-deny for `gh pr merge` (docs say it should; untested).
- Should the lever be opt-in per-repo (a `.docket.yml`/local knob) rather than a machine-wide standing allow?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
