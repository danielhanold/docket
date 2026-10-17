---
id: 541
slug: 'the-implement-next-child-can-claim-without-run-context-so-a'
title: 'The implement-next child can claim without --run-context, so a tracked run loses attribution'
status: 'killed'
priority: 'medium'
type: 'fix'
created: '2026-10-08'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [540, 345, 407]
discovered_from: [540]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

A tracked implement-next run is attributed to the change it built only when the child passes the dispatch prompt's run-context token to `change.claim` as `--run-context`. The flag is optional (a claim without it is a supported untracked claim), and the only thing that makes the child pass it is one sentence in the implement-next skill. Children drop it. Evidence gathered while grooming change 540 on 2026-10-07: in a private test repository the parent ran `run start` and dispatched `docket-implement-next` with the run-context token in its prompt, but the child ran `docket change claim --id <id> --revision <revision> --json` with no `--run-context`, and the claim commit's result carries `gate_context_hash: ""`. The child built the change to a PR and `run verify` said run-complete, but the parent's keyed verdict could only say `run-done <key> no-attributable-claim`. The same shape happened in this repository: change 0366's claim on 2026-10-04 22:34Z has an empty context hash, and its run (`implement-next-20261004t222959z-96475-7c95`) ended no-attributable-claim. Every other claim since 2026-10-04 in this repository carries a context hash, so the drop is intermittent. Not the same gap as change 345: there the parent never arms a run; here the parent armed it correctly and the child omitted the token. A lost attribution means no retry authority, no continuation, and a run the tracker cannot fence or cancel (change 540 handles the symptom of the run staying active).

## What changes

Make it impossible for a tracked child to claim untracked by accident, without blocking legitimate untracked claims. Candidate directions for grooming: (1) `change.claim` requires an explicit choice, either `--run-context <token>` or an explicit untracked flag, so omission is a typed refusal rather than a silent untracked claim; (2) the implement-next context bundle (`context.implementation`) or another typed read carries the run context so the claim does not depend on the agent copying a token from its prompt; (3) the claim detects an armed, undispatched run for this repository and refuses or warns when the token is missing. Apply the same treatment to the other places the child must pass the token (`gate.drive.start --run-context`). Add a regression test for whichever path is chosen.

## Out of scope

Settling a run that already ended without attribution (change 540). The command-launched dispatch gap where the parent never arms a run (change 345). Inferring attribution from timestamps, launch shape, or child prose, which the run tracker forbids.

## Why killed

Killed by human decision on 2026-10-17 after tracing. Nothing in the code is broken, and every fix the stub proposed would risk breaking a supported path.

- **The two examples from this repository were misread.** For 0366 and 0544, the dispatched implement-next agent received the run context, stopped before claiming because each was a human-attended release, and claimed nothing; the parent's verdict correctly said `no-attributable-claim`. After the human chose to proceed attended, the main session claimed each change by hand. That is a supported untracked claim made after the tracked run had already ended, not a dropped token.
- **There was one real miss, and it failed safe.** In a private test repository on 2026-10-07 (change 1), the agent's two-line prompt carried `run-context: <token>`, yet it wrote "Claim (no run context token given)" and claimed without it. The PR was still built and `run verify` said run-complete. Only attribution was lost, so nothing retried, and the keyed verdict now settles the leftover run record (change 0540).
- **The track record does not justify new machinery.** Every implement-next agent in this repository that received a token passed it to the claim: 41 of 41 under `--gate-context` and 65 of 65 under `--run-context`, through 2026-10-17. Across all repositories, about 110 tokened claims show that single miss. The other empty-hash claims here were untracked by design: typed slash-command runs (change 0556), or no run was started.
- **Each candidate fix breaks or weakens something.** Requiring an explicit tracked-or-untracked choice on `change.claim` breaks today's supported untracked claims. Having the binary supply the token, or refuse or warn while a tracked run is waiting, needs it to guess which run a claim belongs to, which is impossible under concurrent loops and forbidden by the run tracker. Binding a claim to a run afterwards would weaken the claim-time ownership proof (ADR-0111). A wording-only tweak to the dispatch instructions (write the token as an explicit "pass --run-context <token> to change.claim and every gate.drive.start" line) was considered and declined, because it cannot be shown to prevent a one-off misread.

Revisit only if dispatched agents start dropping the token repeatedly. The evidence would be an empty `gate_context_hash` on a claim made by a dispatched agent whose prompt carried a token.
