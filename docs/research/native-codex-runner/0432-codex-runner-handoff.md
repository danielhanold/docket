# Change 432 — fresh-session handoff

Read [change 432](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0432-complete-native-codex-runner.md) first.
This is an approved scope/provenance handoff, not a claim that the remaining defect has
been diagnosed or a detailed implementation plan. No implementation or acceptance was
started during this preparation.

## Three distinct goals

1. Immediate: make native Codex docket-implement-next complete the existing workflow.
2. Deferred: shared machinery simplification, recorded in [its discovery note](../shared-orchestration-simplification.md).
3. Deferred: the supervisor in 412, recorded in [its findings note](../0412-supervisor-findings.md).

Do not bundle goals 2 or 3 into 432, introduce a competing supervisor, or assume 412 is
required. If a contract change proves necessary, surface the dependency for human approval.

## Branch provenance — verified facts

- New local and remote branch: fix/complete-native-codex-runner.
- Source change: 431.
- Source branch: chore/native-codex-acceptance-for-active-worker-validation.
- Source local HEAD and GitHub remote tip matched:
  ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8.
- The new branch was created directly at that exact SHA on 2026-09-16.
- It was NOT cut from main or merely from 425's repair tip.
- New branch is in the /Users/homer/dev/docket clone and published to origin.
- No worktree was created or existing checkout switched.
- Docket relation: stacked_on: 431; the intended source PR base is 431's branch while
  that parent remains live. No PR was opened or merged.

The branch was pre-created at the human's explicit request. Change 432 remains proposed;
its branch frontmatter is not forged as if a claim already occurred. When work begins,
inspect existing branch/ownership and use the supported claim/workspace path or the
human-directed bootstrap approach used for 425. Never delete/recreate this branch from
main to satisfy a default workflow. Stop and surface an ownership/adoption refusal.

## Preserved acceptance checkpoint

431 remains the acceptance case; do not mint another acceptance change or redo completed
implementation. Its original feature checkout is:
 /Users/homer/dev/docket-0425-launch-kit/candidate/acceptance-active-validation/primary/.worktrees/native-codex-acceptance-for-active-worker-validation

Preserve:
- Plan commit: 95842609f2a0b1c9c5e8646e8cba8d8a105babd7.
- Worker commit: bbe70004a97f716dfa197d497985ef8930433169.
- Latest repair already ancestral: 95660e8fee6e08ba1f439a0283dba4f0364e7d0a.
- Results checkpoint/source tip: ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8.
- Tracked test internal/nativeacceptance/value_test.go SHA-256:
  f0ce49d43b268ba06bb285505f18fb0a4bee8ba60ffec7e8e032faa19144726e.
- Results file: docs/results/2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md.
- All real suite counters, drive records, scopes, and historical evidence.

Current metadata records run_halted. The final WAITING drive's actual process state must
be accounted for through supported inspection/cancellation before mutating the acceptance
checkout or starting another certification. A halt marker does not prove process teardown.
The old keyed gate-stop forbids redispatch; future acceptance needs fresh human-authorized
supported admission. This branch creation did not cancel, resume, reset, or adopt that run.

## Investigation and completion checklist

1. Trace the actual final handoff through producer, serialized receipt, transport chunks,
   caller parsing and continuation payload. Distinguish never-produced, omitted, lost,
   and misinterpreted credentials; do not infer the cause from the final symptom.
2. Map native named dispatch, live child observation, tool yields, final child return,
   scope identity, active-input checks, commit/ack order, continuation, results, final
   exact-HEAD certification, durable evidence, native review and PR verification.
3. Reproduce the failures with red tests through real CLI/native boundaries. Cover the
   complete remaining path in an isolated rehearsal before expensive acceptance.
4. Implement only fixes needed to meet the existing contract. Preserve other harnesses;
   shared defects require cross-harness regression coverage. Do not weaken guards or
   replace native role dispatch with shell runners, agent.enter or generic substitutes.
5. Run the full configured suite from source, inspect budget findings, and publish a
   clean pinned candidate with matching runtime assets.
6. Prepare the supported 431 resume preserving its checkpoints; obtain explicit
   authorization where a terminal run requires it. Establish how to test/publish candidate
   repairs against the preserved acceptance branch before merging or moving either tip.
7. Completion requires real native workflow verification: configured gates, exact-HEAD
   evidence, native review, attached results and an open correctly based PR. No automatic
   merge. A green unit suite or an intermediate successful gate is not completion.

## Session recommendation

Use a fresh Codex Local session at gpt-6-astra / high. Start in /Users/homer/dev/docket,
read this handoff and current AGENTS.md, and prepare an isolated worktree for the existing
fix/complete-native-codex-runner branch before editing code. Keep registered role model/
effort pins unchanged. Do not invoke the broken implement-next workflow to bootstrap its
own repair; use the human-directed repair posture until the integration tests are green.

Progress reports should identify the current Codex blocker, verified progress, next
acceptance condition, and deferred findings written to the other two notes.

## Boundary investigation, 2026-09-16 — correction to the reported diagnosis

Read-only investigation compared the saved native transcripts with source at
`ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8` (including candidate repair
`95660e8fee6e08ba1f439a0283dba4f0364e7d0a`). No gate was resumed, claimed,
advanced, cancelled, or replayed during this investigation. No private drive credential
was recovered. The existing repair branch still pointed to its approved starting commit.

The final handoff did **not** omit its credential:

| Boundary | Observed evidence |
|---|---|
| Producer and serialization | At 19:13:45 UTC, `gate.drive.handoff` returned protocol 1, `result: applied`, a `WAITING` drive, and nonempty `drive.generation`. `Driver.Handoff` passes `receipt.HandoffGeneration` to `transferDoc`, which serializes that value as `generation`. |
| Shell and code-mode transport | The coordinator transcript contains the same complete JSON inside the terminal shell response, exit 0. The credential field reached the caller. |
| First caller interpretation | The coordinator's final report nevertheless said the receipt did not expose a handoff token. This contradicted its captured response. |
| Outer continuation | At 19:14:08 UTC, the parent received `gate-continue`, `run-waiting`, phase `final`, with a continuation id. |
| Continuation redemption | At 19:14:47 UTC, `run.gate-claim` returned `gate-claimed`, `WAITING`, and a nonempty **top-level** `generation`; the continuation's tool output retained that response. |
| Second caller interpretation | The continuation still demanded a handoff token or parent capability and halted. `RunGateClaim` had already consumed the handoff through `ClaimSeam.Claim`; its returned generation was the new owner's authority to advance the same drive. A second claim or takeover was unnecessary. |

Sanitized source locators: native session transcripts beginning
`rollout-2026-09-16T14-59-03` (coordinator), `rollout-2026-09-16T14-55-46`
(parent), and `rollout-2026-09-16T15-14-25` (continuation), under the local
Codex sessions directory for 2026-09-16. The observations above preserve the relevant
facts without copying gate keys, tokens, or capability values. The earlier halt report
remains a historical record of what the controller believed, not an authoritative
description of the emitted protocol.

The demonstrated failing boundary is **receipt interpretation and continuation
control flow**, twice, rather than missing credential production or transport loss.
This does not disprove earlier transport failures or prove the unexecuted remainder
of the workflow correct. Existing driver and facade tests cover token production and
redemption; passing those alone cannot prove that a native caller chooses the right
next operation. The current Codex receipt checker handles `gate.drive.*` but does not
handle the different `run.gate-claim` envelope. The skill names required token concepts
without consistently spelling out their operation-specific JSON paths and next action.
These are repair candidates to exercise at the public CLI/caller boundary during the
subsequent human-directed red/green work, not evidence of an implemented fix.

Two adjacent completion conditions need explicit rehearsal:

- Results publication moved HEAD from the build-certified commit to `ed80a72a…`.
  The reviewer rejected stale evidence. Re-establish exact-HEAD evidence before review
  entry whenever a checkpoint commit has moved HEAD, and after final material results
  consolidation before PR publication. Do not bypass the reviewer check or endlessly
  edit results to add the latest evidence timestamp.
- `Driver.Claim` closes the old recovery scope; `Driver.Acknowledge` intentionally
  rejects a scope closed by claim/takeover. A recovered drive is advanced under its
  new owner and its terminal evidence consumed through the recovery path. Do not apply
  the uninterrupted worker's final-ack recipe or reuse the closed scope for a new test.
  Prove recovered completion through evidence and run verification in rehearsal.

Current-session boundary: investigate and approve/publish the 432 spec only. Planning,
code repair, red/green execution, full-suite verification, and any later native acceptance
belong to subsequent human-directed work. Shared simplification and supervisor work
remain deferred in their linked notes.
