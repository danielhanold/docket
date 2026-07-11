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
spec: docs/superpowers/specs/2026-07-11-autonomous-finalize-merge-authorization-design.md
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
| Spec | [2026-07-11-autonomous-finalize-merge-authorization-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-11-autonomous-finalize-merge-authorization-design.md) |
| ADRs | [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md) |
<!-- docket:artifacts:end -->

## Why

`docket-finalize-change` can't run headless on Claude Code because its `gh pr merge` is soft-denied by the auto-mode **"Merge Without Review"** classifier (active under `permissions.defaultMode: auto`). A `soft_deny` clears only on explicit human merge intent, which an autonomous run (dispatched wrapper, or finalize inside a `/loop`) has no way to express — so it hard-stops. This is why change 0061 had to **exclude** finalize from `context: fork`.

It bites hardest for docket's primary use case, a **single maintainer**: GitHub forbids self-approving your own PR, so the "just require a GitHub approval" path (ADR-0011) is *structurally unavailable*. The bypass is the only route to headless finalize for a solo repo.

The prior "the allow-rule doesn't work" attempt used `permissions.allow`, which runs **before** the classifier and can't clear a `soft_deny`. The correct, untried lever is **`autoMode.allow`** — a field read *inside* the classifier that overrides matching `soft_deny` rules (confirmed against the Claude Code docs, 2026-07-11). Granting standing permission to merge unreviewed code is a real safety decision, so it is opt-in, repo-local, and per-machine.

## What changes

Add an **opt-in `autoMode.allow` bypass** that lets an autonomous finalize clear the merge soft-deny, materialized into the repo's gitignored, machine-local `.claude/settings.local.json` (the file `ensure-claude-settings.sh` already manages). Shape:

- **Extend `ensure-claude-settings.sh`** with `--enable-autonomous-merge` / `--disable-autonomous-merge` flags that idempotently add/remove the `autoMode.allow` prose rule. Opting in is a **deliberate imperative act** per machine — never a committed or global config value, so it can't silently spread to collaborators. Default (no-flag) behavior is unchanged.
- The rule covers `gh pr merge`, and — pending a build-time spike — the terminal-publish push onto the integration branch if the classifier soft-denies it too (docket's own "ADR main-publish classifier block" experience suggests it might; this is separate from the `permissions.allow` push grant already written).
- **No subagents** (they inherit parent settings — can't scope a bypass), **no skill split**, **no `context: fork`** on finalize (would regress the interactive human-merge path 0061 protected). Finalize's flow is untouched; the bypass transparently stops the soft-deny. With the rule absent, autonomous finalize still abort-and-reports exactly as today.
- A new ADR extends ADR-0011's consent model: a third authorization proof (a standing machine-local bypass) beside GitHub-approval and explicit-id.

**Accepted cost:** because `autoMode.allow` rules are prose, not command-scoped, the rule can't be limited to `gh pr merge` — while present, *every* session in this repo on this machine (interactive included) skips the merge soft-deny. Bounded to one repo, one machine; the price of a simple, persistent, docket-owned opt-in.

A **build-time spike runs first** to confirm the rule actually clears the soft-deny (the whole approach rests on it) and to settle the terminal-publish question. Full design, mechanics, and rejected alternatives are in the linked spec.

## Out of scope

- The autonomous **dispatcher/loop** that would drive finalize headless — this enables the capability; the driver is separate work.
- The context:fork parity fix itself (change 0061).
- Any harness other than Claude Code (Cursor has no such classifier; the flag is a no-op there).
- A declarative `.docket.local.yml` knob + config-fence class (considered, rejected for a low-priority change in favor of the simpler flag).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
