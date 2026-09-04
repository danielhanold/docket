---
id: 407
slug: 'keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent'
title: 'Keyed gate-verdict misattributes its verdict to a concurrent loop''s change id under parallel implement-next runs'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-04'
updated: '2026-09-04'
depends_on: []
stacked_on:
related: [345, 405]
discovered_from: [403]
adrs: [75]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0075](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0075-run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code.md) |
<!-- docket:artifacts:end -->

## Why

A run-gate verdict resolved against a real dispatch key still names change ids the caller never dispatched. Observed live 2026-09-04: `docket run gate-before implement-next` was armed for change 403, its key copied into the dispatch-context, and `docket-implement-next` built 403 fully to an open PR (origin/docket status: implemented, PR #277 OPEN, base main). But `docket run gate-verdict <key>` returned `gate-stop <key> ambiguous-claims 261 402` — naming two OTHER changes the caller never dispatched (402 was implemented/PR #278; 261 in-progress/no PR), both moved by concurrent implement-next loops during the dispatch epoch. The mirror also reproduces: a keyed verdict for change 402 returned `gate-retry-once ... 261 not-implemented remote-head-mismatch pr-unverified evidence-unverified`, again naming 261. This is distinct from change 345 (slash-command dispatch that never captured a before-set/epoch and so is correctly unattributed) and from change 405 (the gate.drive prepare-scope->start handshake seam): here the dispatch WAS keyed and attributed via gate-before, yet the verdict-resolution logic resolves the unmet in-progress target to the newest / highest-priority in-progress claim rather than to the id the key was armed for. Under docket's normal mode of running multiple implement-next loops concurrently, this makes gate-verdict routinely emit gate-stop/gate-retry-once naming sibling-loop ids, degrading it from a mechanical authority to something a human must cross-check against origin/docket before trusting. ADR-0075 says the gate attributes a claim conservatively — a verdict naming an id outside the keyed dispatch's own before-set/after-set is a violation of that invariant.

## What changes

Investigate and fix the verdict-resolution path so a verdict armed with a specific gate-before key attributes only to the change(s) that key's dispatch actually claimed — i.e. resolve the unmet in-progress target against the keyed dispatch's own before-set and dispatch epoch, not against the global set of currently in-progress claims. When the dispatched id completed cleanly (implemented + PR open) the verdict must report success (gate-done/gate-complete) for that id, never a gate-stop/gate-retry-once that names a concurrent loop's id. If some legitimate ambiguity remains under concurrency, the verdict must still never name an id outside the keyed dispatch's attributable set; at most it reports its own inability to attribute in terms scoped to that key. Add a regression test that arms a key for one change, simulates a concurrent loop flipping a DIFFERENT change's status mid-epoch, and asserts the keyed verdict resolves to the dispatched id (and never names the sibling). Reconcile with ADR-0075's conservative-attribution invariant.

## Out of scope

The slash-command unattributed-dispatch gap (change 345) and the gate.drive prepare-scope->start handshake seam (change 405) — both are separate seams. Reworking the disposition vocabulary beyond what the root cause requires. The unrelated flaky release-determinism test (change 406). Changing how concurrent implement-next loops themselves are scheduled or serialized.
