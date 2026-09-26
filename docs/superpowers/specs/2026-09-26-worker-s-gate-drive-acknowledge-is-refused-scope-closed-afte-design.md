<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0459 — Worker's gate.drive.acknowledge is refused scope-closed after the parent claims its WAITING drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-26-0459-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte.md)**
<!-- docket:backlink:end -->

# Handed-off worker scopes: no false BLOCKED after a parent claim — design

Change 0459. Groomed 2026-09-26.

## Problem

A docket-build worker drives its focused tests under a **recovery scope** (minted by the parent's
`gate.drive.prepare-scope`, authorized by the worker's child capability). When a drive outlives an
observation slice the worker performs `gate.drive.handoff` and returns `WAITING`; the parent
performs `gate.drive.claim`, becomes the drive's owner, and advances it to a terminal disposition.

`Driver.Claim` (`internal/gatedrive/driver.go`) deliberately closes the drive's scope on a normal
claim ("close it best-effort so a parent never later takes over a drive that was already handed off
and claimed cooperatively"). That closure is correct and stays.

The worker contract (`skills/docket-build-task/SKILL.md`, *Sequential drives within your scope*)
then requires the worker, before returning, to perform `gate.drive.acknowledge` on its scope — and
"a failed acknowledgement returns `BLOCKED` with the typed cause, never `COMPLETE`".
`Driver.Acknowledge` (`internal/gatedrive/acknowledge.go`) refuses any scope that is `Closed` and not
`FinalAcked` with `ErrScopeClosed`, and the app layer (`ownershipNextAction` in
`internal/app/gate_drive.go`) renders it as "scope authority was transferred or finished; stop and
return BLOCKED".

Observed on change 0458 (Task 2): the worker handed off its package-test drive, the parent claimed
it and advanced it to `PASSED`, the parent resumed the same worker, the worker made its correct
planned commit (0c2534e1), its acknowledge was refused `scope-closed`, and it returned `BLOCKED` for
completed work. A parent trusting the outcome could re-dispatch or halt.

A second, latent gap: the claim-closed scope also refuses a scoped `gate.drive.start` (`driver.go`,
`ErrScopeClosed` on op `start`), so a continuation that needs any further test drive cannot run one
on the original scope.

Root cause: the worker contract assumes the worker always owns its scope's final drive; after a
handoff + claim it does not, and neither the contract nor the refusal message says so.

## Decision (approach A)

Keep the authority model strict. Fix the two contracts (worker and parent) so a handed-off worker
never tries to use its transferred scope, and make the `docket` binary's refusal name the actual
state instead of directing the worker to `BLOCKED`. No refusal is loosened; nothing new is
authorized.

Rejected: (B) treating acknowledge on a claim-closed scope as idempotent success — it lets a child
capability act on a scope whose authority moved to the parent, and does not fix scoped `start`;
(C) contract-only with the old message — leaves the misleading "return BLOCKED" instruction as a
trap for the next worker that misses the rule.

## Design

### 1. Worker contract — `skills/docket-build-task/SKILL.md`

- A worker that returned `WAITING` (performed `gate.drive.handoff`) and is later resumed or
  continued **does not acknowledge the original scope** and does not start drives on it — its
  authority over that scope ended at the handoff.
- Its outcome rests on the **terminal verdict the continuation supplies** (the handed-off drive id
  and its `PASSED`/`FAILED` disposition) plus its own work: `PASSED` with exactly one task commit →
  `COMPLETE`; `FAILED` → the existing repair discretion, never `COMPLETE` on that verdict.
- If the continuation needs further test drives it uses the **fresh scope bundle** the continuation
  provides, driving and acknowledging that fresh scope under the normal sequential-drive rules. With
  no fresh bundle it cannot run tests and returns `BLOCKED` naming "continuation needs a fresh
  scope" — an honest block.
- The existing rule "a failed acknowledgement returns `BLOCKED`, never `COMPLETE`" remains for
  scopes the worker still owns. A `scope-transferred` refusal (below) is not an acknowledgement
  failure of an owned scope: it means the worker misapplied the continuation rule; it reports on the
  continuation's verdict as above.
- Keep the final drive id and verdict in `VERIFICATION`/`NOTES` as today.

### 2. Parent contract — `skills/docket-build/SKILL.md` (*Task-level WAITING and the continuation*)

- After claiming the handed-off drive and advancing it to a terminal disposition, the continuation
  (same-agent resume or fresh dispatch alike) carries: the drive id, its terminal verdict, and an
  explicit statement that the original scope is closed and must not be acknowledged or reused.
- When the continued task may still need test drives, the parent runs `gate.drive.prepare-scope`
  again for the same change/task/phase/branch/worktree (and dispatch context) and includes the new
  start-ready scope bundle — child capability only; the parent capability stays in notes, as for
  any dispatch.
- Reading the continuation's return is unchanged: `COMPLETE` is verified against git (SHA resolves
  and is an ancestor of the branch tip).

### 3. Binary — distinct typed refusal `scope-transferred`

- New `OwnershipErrorKind` `ErrScopeTransferred = "scope-transferred"` in
  `internal/gatedrive/ownership.go`.
- Returned instead of `ErrScopeClosed` when the scope is `Closed && !FinalAcked` (closed by a claim
  or takeover), **only** on the two child-capability operations:
  - `Driver.Acknowledge` — the closed-scope branch (the byte-identical-repeat no-op for a
    `FinalAcked` scope is unchanged);
  - a scoped `gate.drive.start` (the closed-scope check in `driver.go`'s scoped admission).
- A scope closed by a normal acknowledgement (`FinalAcked`) keeps `ErrScopeClosed` on both.
- Both refusals still write nothing.
- `ownershipNextAction` gains a `scope-transferred` message stating the real state and next action,
  e.g. "the parent claimed or took over this scope's drive; this scope is no longer yours — report on
  the verdict your continuation supplied, and run further tests only under a fresh scope". It never
  says "return BLOCKED". The `scope-closed` message drops the "transferred or" wording (e.g. "this
  scope was already finished by its terminal acknowledgement; stop and return BLOCKED").
- The result-vocabulary mapping for the new kind matches `scope-closed`'s (`invalid-input`), and the
  reason vocabulary/schema surfaces gain `scope-transferred` wherever reason members are enumerated.
- **Unchanged:** `Claim` still closes the scope; the parent-side takeover path
  (`takeover.go`, `takeoverClose`, and the "second detached-crash takeover halts scope-closed" rule
  referenced in `internal/app/rungate_verdict.go`) keeps `ErrScopeClosed`; `bind-scope-change`
  keeps `ErrScopeClosed`.

## Testing

1. **Regression (red today)** — `internal/gatedrive`: scoped start → drive reaches a slice end →
   `Handoff` → `Claim` → `Advance` to `PASSED` → `Acknowledge(scopeID, childCap, driveID, oldGen)`
   returns `ErrScopeTransferred` and writes nothing. Today it returns `ErrScopeClosed`.
2. Same claim-closed scope → a scoped `start` with the child capability returns
   `ErrScopeTransferred`.
3. Non-regression: a `FinalAcked` scope + a non-identical acknowledge still returns
   `ErrScopeClosed`; the byte-identical repeat still returns the recorded document.
4. Existing takeover tests (`takeover_test.go`, `integration_takeover_test.go`, and the rungate
   second-takeover halt) stay green unchanged — proof the parent path is untouched.
5. App/CLI layer: the `gate.drive.acknowledge` JSON envelope for the claim-closed case carries
   reason `scope-transferred` and a message that does not contain "BLOCKED".
6. Contract guards in `internal/repoguard` (house pattern): assert the worker contract states the
   handed-off worker does not acknowledge the original scope and reports on the continuation's
   verdict, and the parent contract states the continuation carries the verdict plus a fresh scope
   bundle when more drives are needed. Mutation-test each guard (strip the clause, watch it redden).

## Out of scope

- Broader handoff/claim redesign; `Claim` keeps closing the scope.
- The parent-side takeover path and its `scope-closed` semantics.
- The `artifact.backlink` absolute-path issue from the same run (change 0460).
