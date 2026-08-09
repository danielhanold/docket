---
id: 281
slug: auto-groom-s-critic-verdict-return-channel-fails-under-backg
title: 'Auto-groom''s critic verdict return channel fails under background dispatch'
status: proposed
priority: medium
type: fix
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [247]
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

**Trigger** — observed 7-for-7 during the 2026-08-09 groom campaign (first on change 0247's
critic round, then on 0275, 0261, 0263, 0266, 0277, 0279, 0272, 0265, and 0195): every
`docket-auto-groom-critic` that tried to deliver its verdict back to its dispatcher by
name-addressed agent messaging failed — "No agent named 'docket-auto-groom' is reachable" —
and several critics additionally reported having no agent-listing surface with which to resolve
a ref. Each verdict survived only as the critic's final transcript output, and the campaign
completed only because the coordinating session manually relayed every verdict to the paused
groom — an unmodeled coordinator dependency inside a flow that is autonomous by contract.

**Opportunity** — the critic gate already has one working return path: a groom that dispatches
its critic foreground reads the child's return value directly (several rounds in the same
campaign used it successfully). The failing path is the message-back protocol under background
dispatch: a dispatched groom agent is not registered under its skill name, so name-addressed
delivery cannot resolve, and a groom that yields "waiting for the critic's re-check" waits on a
channel that never delivers. Without a relay the run wedges silently — there is no timeout, no
fallback collect, and no diagnostic.

**Independent value** — unattended throughput and correctness of the auto-groom drain: with the
return channel severed, every backgrounded groom stalls at its first critic round regardless of
how sound its draft is. The defect is in docket's own skill/agent contracts, so it stands
independent of any single harness session.

**Boundary** — settle the critic→dispatcher return-channel contract for both dispatch shapes:
either mandate foreground critic dispatch and ban the message-back protocol, or specify a
return address/instruction that resolves from a background child, and in either case define the
groom-side posture when no verdict arrives (bounded wait, then collect the verdict from the
critic's transcript/report output — never an indefinite yield). The fix lands in the
`docket-auto-groom` skill body, the `docket-auto-groom-critic` agent source, and their
dispatch-contract prose; no scripts are expected to change. Out of scope: the hosting harness's
agent-naming implementation, and the shared-worktree contention family (change 0247).

**Why grooming is needed** — choosing the leg (foreground-only vs resolvable return address vs
collect-on-timeout) depends on the dispatch-capability taxonomy (change 0137 / the convention's
Dispatch-capability resolution section) and on what each supported harness's messaging surface
can actually resolve for a forked child; that judgment call is the spec's job.
