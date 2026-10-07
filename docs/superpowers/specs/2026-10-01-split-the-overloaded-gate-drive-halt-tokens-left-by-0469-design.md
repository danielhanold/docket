<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0481 — Split the overloaded gate-drive halt tokens left by 0469](../../changes/archive/2026-10-01-0481-split-the-overloaded-gate-drive-halt-tokens-left-by-0469.md)**
<!-- docket:backlink:end -->

# Split the overloaded gate-drive halt tokens left by 0469 — design

## Problem

Change 0469 (ADR-0129 rows 68 and 69) renamed two gate-drive halt causes. Each name describes one
condition, but each is also emitted for a second condition it does not describe, so a reader picks
the wrong remedy. A whole-repo trace of every halt-cause emission of the two strings:

| Site | Condition | Token today | Fits? |
|---|---|---|---|
| `driver.go` pass-path fingerprint revalidation | worktree fingerprint moved since drive start | `worktree-changed` | yes |
| `driver.go` `relaunchRefusal` | same fingerprint drift | `worktree-changed` | yes |
| `takeover.go` `scopeIdentityMatch` failure | the drive's repo/branch/worktree/change/task/phase does not match the scope | `worktree-changed` | **no** |
| `driver.go` relaunch, `Launch` error with an unresolved reservation | nothing proved whether the launch happened | `launch-unconfirmed` | yes |
| `driver.go` `haltReservedRelaunch` default | crash-window uncertainty | `launch-unconfirmed` | yes |
| `driver.go` `resolveDriveRun`, `LoadWorktreeExecution` error | slot absent/corrupt/unknown-schema/IO — the drive cannot prove which run it is linked to | `launch-unconfirmed` | **no** |
| `driver.go` `resolveDriveRun`, `slot.ReservationToken != rec.AdmissionToken` | the slot was reassigned — the run link is lost | `launch-unconfirmed` | **no** |

The `ErrLaunchUnconfirmed` *ownership refusal kind* (worktree admission) is a separate surface and is
correct as-is. `resolveDriveRun`'s cause reaches a halt only through `authorizeRelaunch`; its other
callers (`recoveryRunRevoked`, `reconcile.go`) discard it.

## Decision

1. **Takeover scope drift reuses `scope-identity-mismatch`.** In `takeover.go`, the
   `scopeIdentityMatch` failure halts with `string(ErrScopeIdentityMismatch)` instead of
   `"worktree-changed"`. The token already names exactly this condition — `start` refuses with it when
   a drive's identity does not match its prepared scope — and ADR-0129 deliberately kept it (family
   (e), "identity" in its which-record sense). The same takeover function already emits ownership
   kinds as halt causes (`string(ErrUnresolvedLaunchTransition)`, `string(ErrHandoffOutstanding)`),
   and the takeover fingerprint case already halts `string(ErrFingerprintMismatch)`, so this follows
   the established pattern and adds no vocabulary. Broaden `ErrScopeIdentityMismatch`'s doc comment
   in `ownership.go` to say it is also a takeover halt cause.
2. **A lost run link gets one new token, `run-link-lost`.** Add
   `CauseRunLinkLost = "run-link-lost"` beside the other `Cause*` constants in `drive.go`. Both
   slot branches of `resolveDriveRun` (load error and token mismatch) return it. One token, not two:
   both branches have the same meaning (the drive can no longer prove which run it belongs to, so it
   refuses new execution) and the same remedy (cancel the run / start fresh), and an absent slot is
   not "unreadable", so reusing `run-record-unreadable` would mislabel it. The name follows the
   gatedrive package's existing "run-linked drive" wording (`reconcile.go`); it collides with nothing
   in the repo today.

Before / after for an operator reading the halt:

```
takeover … HALTED cause=worktree-changed        → checks the worktree's bytes (wrong)
takeover … HALTED cause=scope-identity-mismatch → checks which scope/drive pairing drifted

relaunch … HALTED cause=launch-unconfirmed → recovers an "unresolved" slot that may be fine and owned by someone else (wrong)
relaunch … HALTED cause=run-link-lost      → knows this drive is orphaned from its run; cancel the run / start fresh
```

## Unchanged

- The four correct emission sites (two `worktree-changed`, two `launch-unconfirmed`).
- The `ErrLaunchUnconfirmed` worktree-admission refusal kind and every skill prose mention of
  `launch-unconfirmed` (`skills/docket-build/references/gate-caller-loop.md`,
  `skills/docket-finalize-change/references/gate-failure.md`, `skills/docket-build-task/SKILL.md`) —
  each describes that refusal, which stays accurate.
- `mapDriveHaltCause` in `internal/app/finalize_rebase.go`: both tokens fall through to
  `GateHaltUnavailable`, which is the correct finalize posture for either condition.
- When the drive halts and how it recovers — this is a token-only change.

## Tests

- `internal/gatedrive/takeover_test.go`: the five "identity mismatch" cases (branch, worktree,
  change, task, phase) expect `string(ErrScopeIdentityMismatch)`.
- `internal/gatedrive/driver_test.go`: relaunch cases for (a) an absent or corrupt worktree slot and
  (b) a slot whose `ReservationToken` no longer equals the drive's `AdmissionToken`, each expecting
  `CauseRunLinkLost`. Retarget an existing case if one already covers the path; otherwise add one.
  Each must redden when its site is reverted to the old token (mutation-test the guard).
- `internal/app/runtracker_verdict_integration_test.go`
  (`TestIntegrationRunVerdictVerdictTakeoverHaltStops`): change the fake takeover cause from
  `worktree-changed` to `scope-identity-mismatch` so the pass-through fixture uses a value the driver
  can actually produce.
- Derive the full site list from a whole-repo grep for both strings (AGENTS.md), sorting prose from
  executable; do not trust this enumeration alone. Run the whole suite at the build gate.

## Prose and decision record

- `docs/reference/glossary.md`, *Worktree changed / certified input changed*: drop the "or when a
  takeover finds a drive whose recorded branch, worktree or change is not the scope's" clause and
  state that this case halts `scope-identity-mismatch`.
- `docs/reference/glossary.md`: add a short `run-link-lost` entry (the drive cannot prove which run
  it belongs to, so it refuses to relaunch; remedy: cancel the run and start fresh) plus its index
  line.
- ADR-0129: append a dated `## Update` (via the docket-adr flow; never an edit to the decision or an
  amendment) recording that rows 68 and 69 now cover only their described conditions, that takeover
  scope drift halts with the retained family-(e) token `scope-identity-mismatch`, and that a lost run
  link halts with the new `run-link-lost`.

## Out of scope

Changing when the gate drive halts or how it recovers; any other ADR-0129 rename row; the leftover
0469 wording ("repair" in `change relink`, "Step 0"), tracked separately; the scoped branch of
`resolveDriveRun` returning `run-record-unreadable` for an unreadable scope.
