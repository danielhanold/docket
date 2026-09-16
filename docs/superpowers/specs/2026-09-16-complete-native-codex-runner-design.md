<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0432 — Complete native Codex runner](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0432-complete-native-codex-runner.md)**
<!-- docket:backlink:end -->

# Complete native Codex runner

## Purpose and authorization boundary

Complete native Codex's existing Docket workflow, including supported continuation,
exact-HEAD certification, native review, results verification, and an open correctly
based PR. Repair the actual caller boundary before attempting acceptance again.

This session ends after approval, publication, and linking of this spec to change 432.
Approval does not launch implementation, planning, acceptance, or a resume of 431.
Subsequent planning and repair are human-directed on the existing
`fix/complete-native-codex-runner` branch. Do not use `docket-implement-next` to bootstrap
its own repair. Native acceptance follows successful repair and rehearsal, under later
explicit authorization.

## Investigated failure

The saved September 16 native transcripts contradict the reported missing-token
diagnosis. At 19:13:45 UTC, `gate.drive.handoff` produced an applied `WAITING` receipt
with nonempty `drive.generation`. Its complete JSON reached the coordinator in the
terminal shell/code-mode response. The coordinator nevertheless reported no token.

The parent then received `gate-continue`. At 19:14:47 UTC, the continuation's
`run.gate-claim` returned `gate-claimed`, `WAITING`, and nonempty top-level `generation`.
That complete response reached the continuation, which still demanded a handoff token
or parent capability and halted.

Source at `ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8` confirms the semantics:
`Driver.Handoff` and `transferDoc` place the handoff credential in `drive.generation`;
`RunGateClaim` consumes that credential through `ClaimSeam.Claim` and returns the fresh
owner generation. The observed failure is caller interpretation and subsequent control
flow, not absent production or lost transport in this final run. Earlier transport
failures remain separate regression cases. This is forensic diagnosis, not a new
execution or red/green reproduction.

The sanitized trace and source locators are in
`docs/changes/research/0432-codex-runner-handoff.md`. Preserve the historical halt report;
correct the live proposal's diagnosis when this spec is attached.

## Design

### Operation-aware consumption of existing receipts

Preserve the wire protocol, operation names, outcome vocabulary, and authority model.
Extend the existing read-only Codex receipt-validation boundary and its caller
instructions to cover the facade continuation response as well as drive responses.
This is checked interpretation of public receipts, not a new workflow controller.

| Operation and successful response | Credential path and meaning | Next action |
|---|---|---|
| `gate.drive.start` / `gate.drive.advance`, `WAITING` | `drive.generation`: current owner | Advance the same drive, or explicitly hand off before departure. |
| `gate.drive.handoff` | `drive.generation`: single-use handoff token | Direct recovery uses `gate.drive.claim`; an outer-gated departure instead returns to the parent for its keyed verdict and supported facade continuation. Never redeem through both paths. |
| `gate.drive.claim` / authorized `gate.drive.takeover` | `drive.generation`: fresh owner | Continue the same drive under that owner. Takeover retains its existing direct-child-return authorization. |
| `run.gate-claim`, `decision: gate-claimed` | top-level `generation`: fresh owner; top-level `drive_id` and `phase` identify recovered work | Redemption already happened. Continue through `gate.drive.advance`; do not claim again or demand the predecessor token or parent capability. |

Validation checks protocol, producing operation, result/disposition, required fields,
credential presence where applicable, and the expected drive/phase when exposed.
Use assignment and existing context/input checks for identity facts absent from the
receipt. No nonexistent envelope fields may be required. The checker does not read
private gate records or claim that string inspection proves current ownership: the
existing backend remains authoritative for stale, consumed, or superseded credentials.

The checker must support the actual `run.gate-claim` envelope and distinguish its
refusal from successful redemption. Its normalized interpretation must distinguish
handoff authority from owner authority, without renaming producer fields. Wire the
checked interpretation into the native caller recipe so correct parser behavior cannot
coexist with instructions to perform a second claim. Update affected generated assets
through their authoritative generators. Fix shared prose or validation only as necessary
for this contract, with cross-harness regression coverage.

`PASSED`, `FAILED`, and `HALTED` remain distinct. Only a trustworthy suite failure feeds
the existing repair policy. A claimed terminal drive may need an advance to obtain the
full terminal receipt, because the facade claim does not return `raw_run_dir`. That
collects existing state; it must not launch a replacement suite. Preserve ordinary
terminal generations needed for acknowledgement and existing idempotent observation
and acknowledgement behavior. Single-use claims remain single-use; replay grants no
new authority.

### Complete transport and native child observation

Retain the exact native child identity through terminal return. Retain code-mode cell
and shell-session identities separately, collect the original invocation to terminal
exit, accumulate all output chunks, then parse. Empty live output and a tool yield are
not protocol `WAITING` or missing-receipt failures. Process exit or child prose alone
does not establish workflow completion.

Malformed or missing terminal output, an unobservable/lost handle, wrong operation,
or incomplete required fields uses the existing blocked/halt posture. Never replay a
mutating operation to recover its response, infer a token from a locator, or read private
drive records for credentials. Record sanitized stage-specific diagnostics and retain
original responses in protected local storage; do not put credentials in tracked notes.

### Recovered versus uninterrupted scope completion

`Driver.Claim` closes the old recovery scope. `Driver.Acknowledge` intentionally refuses
scopes closed by claim/takeover. Preserve that behavior.

An uninterrupted worker validates active inputs, completes tests, acknowledges its
current terminal scoped drive at the required point, then commits in the existing
contract's order. A recovered controller advances under its fresh owner and consumes
recovered evidence; it must not invoke the old scope's normal final acknowledgement or
reuse that closed scope for later tests. Later tests use fresh quiescent scopes through
the existing path. Rehearsal must prove recovered evidence can reach final verification,
not merely that claiming succeeds. Any newly discovered backend contradiction must be
reproduced before repair; an authority-contract change returns for human design approval.

### Checkpoints, review, and exact-HEAD certification

431's results commit moved HEAD beyond its build evidence, so native review correctly
refused stale inputs. Preserve that check. If any checkpoint moved HEAD before required
review entry, establish evidence for the current head before preparing the review
assignment. An invalid review entry is not a completed review.

After valid native review, preserve the existing fix and no-automatic-re-review policy.
Consolidate substantive review/fix findings in the existing results artifact before
final certification. Record fresh durable evidence through `evidence.record`, into a
new external file, and verify its exact head. Final certification follows the existing
phase's authoritative configuration: the observed final drive used owner `build`,
phase `final`; this does not invoke the separate finalize/merge workflow.

Do not move HEAD beneath a live or transferred drive. Do not rewrite results merely to
paste the final gate SHA or timestamp. Material new findings may require results edits
and recertification. Before publication and implemented transition, verify local head,
remote head, evidence head, attached results, actual review disposition, PR head and
resolved base. A green intermediate gate is not run completion.

## Verification requirements

The later human-directed plan starts with red tests at the observed consumer boundary.
Existing driver tests that already prove token production are controls, not a new red
reproduction. Tests must run real public CLI producers, capture their complete output,
pass it through the same checked consumer path taught to native callers, and assert
the next operation and preserved execution identity. Do not manufacture a failing
producer merely to obtain RED.

Required cases:

- Direct handoff/claim and facade verdict/claim/advance, including the different JSON
  paths and owner rotation; successful facade claim without a predecessor token or
  parent capability in the continuation prompt.
- No redundant claim, takeover, start, retry charge, or reset after successful redemption;
  stale-owner and consumed-token rejection remain backend enforced.
- Initial completion, shell yield, code-mode yield, multiple chunks including split JSON,
  terminal missing/malformed output, wrong operation/drive, refusal, and false success
  inferred from prose or exit status. Collection never repeats the original mutation.
- WAITING to PASSED/FAILED/HALTED, terminal completion while no model is active, recovered
  terminal receipt collection, and closed-scope handling without weakening acknowledgement.
- Native worker input validation and commit/ack ordering; results commit invalidating
  earlier evidence; valid review entry; final evidence and correctly based PR verification.

An isolated deterministic rehearsal covers the entire remaining route, not just the
first repaired handoff. Reuse existing fixture facilities; do not create another real
acceptance change. Use real CLI, Git, receipt checking, and evidence paths. Mock only
external GitHub effects and native-host lifecycle where needed for deterministic tests,
and label those limits. Fixture dispatch is not real native dispatch proof. Mutation-test
new guards at their behavioral boundary.

Run the whole configured suite from the repair source through the Go suite runner;
resolve its command from configuration. Inspect BUDGET WATCH/PARALLEL-SENSITIVE findings
and act on serially confirmed budget breaches. Pin a clean candidate with matching
full source SHA, binary identity, embedded assets, generated roles, skills, and references.
Validate all applicable project/global aliases; on-disk hashes do not prove loaded role
instructions. Actual native acceptance requires a fresh correctly configured session.

## Later native completion and branch preservation

432 retains its existing branch, starting at
`ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8`, and `stacked_on: 431`. Its PR targets 431's
branch while that parent remains live. Do not delete/recreate it or substitute main.
Establish later workspace ownership through supported adoption or the human-directed
bootstrap; surface refusals instead of forcing ownership.

Preserve 431's plan `95842609f2a0b1c9c5e8646e8cba8d8a105babd7`, worker
`bbe70004a97f716dfa197d497985ef8930433169`, repair ancestor
`95660e8fee6e08ba1f439a0283dba4f0364e7d0a`, results checkpoint `ed80a72a…`, and test hash
`f0ce49d43b268ba06bb285505f18fb0a4bee8ba60ffec7e8e032faa19144726e`.
Retain real counters, deadlines, scope/drive history, and prior evidence. No repeated
planner or worker implementation solely to recreate completed acceptance work.

After repair, rehearsal, and full-suite verification, later human-directed planning
must resolve candidate delivery to 431: distinguish the pinned external runtime from
source-owned suite code and specify any required source integration and PR consequences
before moving either branch. No automatic merge or unconditional global reinstall is
authorized by this design.

Before later acceptance mutation, account for the halted run through supported
inspection/cancellation. A halt marker is not teardown proof; the old gate-stop grants
no redispatch. Resume only through newly authorized supported admission, retaining
checkpoints and accounting. 431's PR targets its own resolved effective base, currently
425's branch, never itself.

432's compatibility goal is complete only with a real native run proving the remaining
431 workflow: retained planner/worker evidence, actual native review, configured gates,
exact final-head evidence, attached verified results, an open correctly based PR, and
`run-complete`. Deterministic rehearsal and a green suite are prerequisites, not that
proof. No PR is merged automatically.

## Non-goals and deferred findings

No shared orchestration simplification, supervisor 412, new ownership mechanism,
alternate harness, generic-agent substitute, or `agent.enter` routing for native roles.
No weaker validation, reset accounting, recreation of 431, or fresh acceptance change.
Legacy runner retirement and model policy changes remain separate work.

Record shared discoveries in `docs/changes/research/shared-orchestration-simplification.md`
and supervisor findings in `docs/changes/research/0412-supervisor-findings.md`. They are
not prerequisites. Necessary shared defect repairs require cross-harness coverage;
contract changes require separate human design approval.

## Assumptions

- The inspected final-run receipts and pinned source establish the diagnosis; no test
  or acceptance was executed to claim it reproduced during specification.
- Existing producer fields and backend authority semantics are sufficient. Start with
  native caller/checker repair; do not redesign a producer based on the old halt prose.
- Acceptance needs a fresh session whose loaded runtime matches the candidate. The
  current main/global session's older dispatch metadata cannot certify that runtime.
- Human-directed planning will resolve branch ownership and candidate integration
  before implementation/acceptance effects; this spec does not authorize those effects now.
- Spec publication changes metadata only and ends this session's authorized work.
