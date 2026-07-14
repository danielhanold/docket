---
id: 74
slug: bootstrap-facade-verb
title: A `bootstrap` facade verb — retire the last direct-helper carve-out in Step-0
status: proposed
priority: medium
created: 2026-07-14
updated: 2026-07-14
depends_on: [68, 72]
related: [68, 72]
adrs: [29, 30]
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
| ADRs | [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md) |
<!-- docket:artifacts:end -->

## Why

Change 0068 introduced the `docket.sh` facade (finite subcommands, config read from stdout, never
`eval`'d) and change 0072 rewired the seven operating skills and the convention's Step-0 preamble
onto it. After 0072, exactly **one** direct-helper invocation survives in the convention: the
`CREATE_ORPHAN` bootstrap path still calls `docket-config.sh --bootstrap` directly, because the
facade has no verb for it.

That single carve-out is the whole cost. It is the one place a reader of Step-0 must learn a second
command shape, and it is the one hole in the claim change 0073 (Cursor sandbox & permissions guide)
wants to make — that docket's entire runtime surface is two command shapes, and therefore that a
small, stable, copyable permission configuration is possible. A one-verb exception forces the
permission config, and the guide explaining it, to enumerate a second binary.

0072 left this deliberately out of scope (0068 owns facade behavior); ADR-0029 and the 0072 spec's
§Decisions both record it as a future candidate, and the 0072 results file re-raised it at the merge
gate.

## What changes

- Add a `bootstrap` verb to the `docket.sh` facade, routing the `CREATE_ORPHAN` path (the guarded
  orphan-`docket`-branch create) through the facade like every other operation.
- Rewire the convention's Step-0 preamble to invoke the verb, retiring the last
  `docket-config.sh --bootstrap` direct-helper mention from skill prose.
- Extend the 0072 skill-facade wiring guard so the bootstrap carve-out it currently tolerates is no
  longer an accepted exception — the guard should redden if a direct-helper bootstrap invocation
  reappears in skill prose.
- Check whether change 0073's "two command shapes" framing can then be stated without the carve-out
  caveat.

## Out of scope

- Any change to what bootstrap actually *does* (the `¬DOCKET ∧ ¬LIVE` guard, the orphan create, the
  push). This is a routing/surface change, not a behavior change.
- Broadening the facade beyond this one verb, or revisiting ADR-0029's finite-subcommand posture.
- Tightening the 0072 prose guard to forbid all `.sh` tokens — ADR-0030 explicitly rejects that
  over-scope, and it must stay rejected.

## Open questions

- `docket-config.sh --bootstrap` is a **write** path guarded to one 2×2 cell, while every other
  facade verb is read-only or a bookkeeping commit. Does the facade need to surface that asymmetry
  (a confirmation, a distinct exit code), or does the script's own cell guard suffice?
- Does the verb belong in the same `WRAPPED_OPS` inventory as the rest, or does a write-path verb
  need its own dispatch arm and its own inventory sentinel (see ADR-0029's "wrong surface" note in
  the guards-are-code learnings family)?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
