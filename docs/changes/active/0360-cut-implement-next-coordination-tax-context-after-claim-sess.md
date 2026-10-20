---
id: 360
slug: 'cut-implement-next-coordination-tax-context-after-claim-sess'
title: 'Stop implement-next tripping over its own bookkeeping'
status: 'in-progress'
priority: 'high'
type: 'feat'
created: '2026-08-27'
updated: '2026-10-20'
depends_on: []
stacked_on:
related: [315, 357, 313, 335, 324, 247, 294, 399, 549, 448, 530]
discovered_from: []
adrs: [3, 47, 20, 66, 92, 95, 129, 126, 106]
spec: 'docs/superpowers/specs/2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess-design.md'
plan: 'docs/superpowers/plans/2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/cut-implement-next-coordination-tax-context-after-claim-sess'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-20T21:34:42Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess-design.md](../../superpowers/specs/2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess-design.md) |
| Plan | [2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess.md](../../superpowers/plans/2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess.md) |
| ADRs | [ADR-0003](../../adrs/0003-convention-reference-loading.md), [ADR-0047](../../adrs/0047-digest-only-read-tier-skips-preflight.md), [ADR-0020](../../adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0066](../../adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0092](../../adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0095](../../adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0129](../../adrs/0129-collision-free-docket-vocabulary.md), [ADR-0126](../../adrs/0126-named-implement-next-skips-unrelated-maintenance-preflight.md), [ADR-0106](../../adrs/0106-implementation-preflight-is-a-deterministic-operation-not-a.md) |
<!-- docket:artifacts:end -->

## Why

Originally captured 2026-08-27 from a run in a downstream Helm-chart repository, where a two-commit change spent most of its tokens on bookkeeping: re-syncing, hunting for the right revision, probing request formats, and fighting gate and evidence config. Re-traced at grooming (2026-10-20) against the Go tree: most of that list was fixed by later changes (#448, #374, #489, #517, #530, #399, #520, #521, #502).

What is still live was measured on the last 15 implement-next runs (2026-10-17 to 2026-10-20):

- The claim receipt's `committed_revision` is a commit id, but the skill says it is the record revision, so 13 of 14 claimed runs failed their first reconcile with `contended`.
- `context.implementation --id` refuses every in-progress change (14 of 14), although the skill routes post-claim re-reads and the documented resume entry through it.
- Runs therefore re-sync and `git rev-parse` before every write: 6.7 `repository.prepare` calls per run, nearly all redundant since #549.
- Four `change.refresh-claim` calls per run protect nothing: reclaim already refuses while the feature branch exists.
- `evidence.verify` fails in 13 of 15 runs because the skill doesn't say the file is the receipt's `block` text, and the call repeats a check `pr.publish` and `mark-implemented` make anyway.
- Smaller: the review-size bump counts a stacked child's parent lines; reconcile has no "nothing changed" wording; Step 7 loads a 109-line edge-case file every run; `docket schema <id>` is refused; `repository.prepare` returns an unused `skills: {}`.

The goal is a run whose writes succeed first time, with no weaker conflict checks.

## What changes

One change (design in the linked spec):

1. **Revision lookups.** Before each post-claim write, the run reads its record's revision from the local metadata checkout (`git rev-parse HEAD:<record path>`). The skill stops claiming receipts carry it. On `contended`: re-sync once and retry once. No new receipt field.
2. **Context after claim.** `context.implementation --id` serves an in-progress change: the full bundle with `claim_eligible: false`, the recorded branch, and a readable halt marker. Selection and every other status are unchanged.
3. **No claim refreshes.** Remove every `change.refresh-claim` call and delete the operation. Claim, reconcile, and resume-halted still stamp `claimed_at`. Fix the convention's lease wording.
4. **Fewer re-syncs.** `repository.prepare` runs at startup, after a dispatched agent writes the change (plan-writer, docket-adr), and once after a `contended` write. The post-claim SHA check and the other re-syncs go.
5. **Reconcile wording.** A "nothing changed" request is id + revision + a one-line log body. The operation writes the date heading.
6. **Evidence wording.** The `--evidence` file is `evidence.record`'s `block` text. The standalone `evidence.verify` call goes.
7. **ADR numbers.** implement-next passes the change to docket-adr, so `adr.record` adds the number to `adrs:` in the same commit; the extra reconcile goes.
8. **Review size.** The 1500-line tier bump diffs against the change's own effective base.
9. **Leftovers.** Drop the dead `skills` field from `repository.prepare`; move PR-body assembly from `edge-paths.md` into Step 7; accept `docket schema <id>` as a plain argument.

## Out of scope

- A new receipt field for the record revision (rejected at grooming; see the spec).
- Weakening any exact-revision check, exact-lease push, claim eligibility, or stacked-base resolution.
- Stamping `claimed_at` from attach-plan/attach-results (considered for the cross-clone reclaim case and declined; see the spec).
- A Go `docket implement` driver.
- A separate gate "couldn't run" result for exit 126/127 (its trigger, agent-supplied argv, no longer exists).
- Changing the parent's check of the plan-writer's work.
- `docket config --repo-dir` reading `.docket.local.yml` from the given folder rather than the main checkout (not hitting any run today).
- Selection: `context.implementation` without `--id` still picks only build-ready proposed changes.
- Auto-detecting non-pytest Python test scripts (`configure-tests --command` covers it).
- Anything already fixed by #448, #374, #489, #517, #530, #399, #520, #521, or #502.

## Reconcile log

### 2026-10-20

Unchanged against main 3f197de6d. The spec was groomed today (2026-10-20) against the current tree; no related change landed since that alters scope, and the effective base is main.
