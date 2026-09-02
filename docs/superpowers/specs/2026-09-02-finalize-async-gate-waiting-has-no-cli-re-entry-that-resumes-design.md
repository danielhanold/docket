<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0396 — finalize async gate WAITING has no CLI re-entry that resumes the same drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0396-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes.md)**
<!-- docket:backlink:end -->

# Finalize gate WAITING resumes through the owned rebase receipt

**Change:** 0396 · **Date:** 2026-09-02 · **Type:** fix · **Priority:** high

## Problem

Finalize's local suite gate is driven in slices (ADR-0098). When a slice ends while the suite is
still running, `finalize.rebase` returns `disposition: waiting` with a continuation (drive id +
owner generation) in `gate.continuation`. Nothing on the CLI accepts that continuation:

- `finalize rebase` and `finalize rebase-continue` expose no continuation flag, so every re-entry
  reaches the gate seam with an empty continuation, mints a fresh run root under
  `os.MkdirTemp("", "docket-finalize-gate-*")`, and calls the driver's `Start`. Each re-run launches
  the whole suite again under a new detached supervisor. Repeated re-entry never converges.
- The generic `gate drive advance --drive-id --owner-gen` does advance the same drive, but it is not
  the finalize seam: a PASSED terminal there mints no finalize evidence block, so the caller has to
  recover evidence out-of-band with `evidence.record --run <raw run dir>`.

The `docket-finalize-change` skill has no `waiting` route at all in its step-3 disposition table
(the word does not appear in the skill body), which is why the ad-hoc "use `gate drive advance`"
workaround grew. Observed live on change 0364: several orphaned gate drives, evidence minted by hand.

The application layer already carries the resume path: `FinalizeRebaseRequest.Continuation` exists,
the receipt-recovery path threads it into `composeLocalGate`, and the production seam calls
`Advance` when it is set. It is unreachable only because the CLI never populates it.

## Root cause and the design constraint

The owner generation is a caller-held secret by design (ADR-0098: only the exact owner advances a
drive; ownership transfers only through a fingerprinted single-use handoff). A fresh process cannot
advance a drive it does not hold the generation for, and the drive store has no
find-live-drive-by-change lookup. So a keyless re-entry is only possible if finalize itself persists
the continuation somewhere owner-private.

Finalize already has exactly that place: the owned rebase receipt (`rebase-receipt.json` in the
workspace meta dir under the repository's git common dir). It is written before the first git
mutation, is never committed, is the durable record of "this attempt", and is what `finalize
publish` and `finalize cleanup` already key on. The 0342 driver spec's *Finalize* section states the
contract this change fulfils: "its caller persists or carries the opaque continuation and re-enters
the same local-gate phase after a slice."

## Goals

1. A WAITING finalize gate is resumed by re-running the **identical** `finalize.rebase` invocation
   (`--id --version --head`, the values the caller already passed). No new operation, no new flag.
2. Re-entry advances the **same** drive and, on PASSED, mints the finalize evidence block through the
   finalize seam and returns `rebased`/`unchanged` exactly as a single-slice run does today.
3. A bare re-run never mints a second run root while a drive for this attempt is live.
4. The `docket-finalize-change` skill routes `waiting` explicitly.

## Non-goals

- Changing gate supervisor or driver mechanics (`gate.drive.*`, `internal/gatedrive`). In
  particular #0375 (`gate drive start` idempotency) is a sibling, not a dependency: this design
  sidesteps it for finalize by never calling `Start` while a receipt-recorded drive exists.
- Changing the resolver/repair two-agent flow or build's gate.
- Cleaning up run roots orphaned by earlier finalize runs under `$TMPDIR/docket-finalize-gate-*`.
- The repair path's post-repair re-gate. The skill's step 5 today re-gates through raw
  `gate.launch` / `gate.observe` rather than the driver, which is at odds with ADR-0098's
  "executable workflows compose the driver layer, never the raw gate primitives". **Discovered
  work** — report it for capture as its own change; do not fold it in here.

## Design

### 1. Re-entry contract

The WAITING re-entry for finalize's local gate is `finalize.rebase` with the same `--id`,
`--version`, and `--head` as the original call. The rebase itself is recovered idempotently from the
receipt (today's `recoverFromReceipt` path: receipt present, base unchanged, no rebase in progress,
head descends the base), and only the gate advances.

Dispositions on re-entry:

| Drive state at re-entry | Disposition |
|---|---|
| still running | `waiting` again (continuation retained) |
| PASSED | `rebased` / `unchanged`, `gate.evidence` populated, receipt continuation cleared |
| FAILED | `failed` / `gate-failed` (repair work), continuation cleared |
| HALTED, or record unreadable / owner superseded | `blocked` / `gate-halted` with the mapped halt cause, continuation cleared |

The `waiting` document keeps `gate.continuation.drive_id` for observability and diagnostics and
**drops `generation`**: the owner generation is receipt-private from this change on, and exposing a
second copy is what invited the `gate drive advance` misuse. `GateContinuation` stays the seam-level
type; only the JSON projection narrows.

### 2. Receipt schema

`workspace.RebaseReceipt` gains two optional scalar string fields:

```
gate_drive_id          string  `json:"gate_drive_id,omitempty"`
gate_owner_generation  string  `json:"gate_owner_generation,omitempty"`
```

`validateRebaseReceipt` enforces the pair rule: both empty or both non-empty; a half-set pair is
refused on write and on read like any other malformed field. Every field stays a scalar string so
the receipt remains a comparable value (`workspace/rewrite.go` asserts on-disk equality with the
receipt it was handed; publish reads and passes the same receipt in one call, so a receipt with gate
fields present is still byte-equal to itself).

Lifecycle of the pair:

- **Set** when the gate seam returns WAITING: the receipt is rewritten through the existing
  crash-safe `WriteRebaseReceipt` with every other field byte-identical and the pair populated.
- **Cleared** in the same `composeLocalGate` call that maps any terminal — PASSED, FAILED, and every
  HALTED cause including an unreadable drive record or a superseded owner — by rewriting the receipt
  with the pair empty. A halt keeps today's `blocked` disposition (a human is needed) but does not
  wedge the receipt: the next deliberate re-run starts a fresh drive.
- `finalize cleanup` already removes the whole receipt; `finalize rebase-abort` already clears it.
  Neither needs to know about the pair.

Rationale for clear-on-any-terminal: a terminal drive is idempotent in the driver (Advance returns
the recorded verdict) but its run root is removed at the terminal, so a later resume against it could
never mint evidence again. Retaining a dead continuation only adds a wedged state.

### 3. Application changes (`internal/app/finalize_rebase.go`)

- `FinalizeRebaseRequest.Continuation` is **retired**. The CLI never set it; the one integration
  test that does (`finalize_rebase_integration_test.go`, the waiting-then-resume case) moves to the
  receipt-driven flow: two bare `FinalizeRebase` calls with identical requests.
- `composeLocalGate` no longer takes a continuation parameter. It resolves the continuation from the
  receipt it is composing under:
  - **recovery path** (`recoverFromReceipt`): the receipt's pair, which may be empty (first re-entry
    after a crash before WAITING, or after a cleared terminal) → fresh drive.
  - **fresh attempt** (`FinalizeRebase` after `BeginRebase`): the receipt was just written with an
    empty pair → fresh drive.
  - **resolver path** (`mapContinuedRebase`): the head changed, so always a fresh drive; the pair is
    empty there by construction (a WAITING can only be reached after the rebase completed, and the
    resolver path only runs on a mid-conflict receipt).
- After `RunLocalGate` returns:
  - WAITING → write the pair. A receipt-write failure returns `blocked` with the existing
    `receipt-write-failed` reason and a message naming the drive id, so a human can locate the
    still-running suite. (The suite keeps running; that is one wasted run, never a fabricated red.)
  - any terminal → clear the pair (best-effort ordering: map the outcome first, then rewrite; a
    clear failure is reported in `message` but does not change the disposition, since the next
    re-run's Advance on a terminal drive halts and clears it then).
- `LocalGateRequest.Continuation` and the production seam's Start-vs-Advance branch are unchanged.

### 4. CLI (`internal/cli/finalize.go`)

No flag changes. `finalize rebase` keeps `--id --version --head`. The result document's
`gate.continuation` marshals `drive_id` only (see §1).

### 5. Skill contract (`skills/docket-finalize-change/SKILL.md`, `references/gate-failure.md`)

Step 3's disposition table gains one route:

> `waiting` with `reason: gate-waiting` — the local suite is still running under the detached
> supervisor. Re-run the **identical** `finalize.rebase` invocation (same `--id --version --head`);
> it recovers the completed rebase from the owned receipt and advances the **same** drive. Never
> re-enter through `gate drive advance`, never mint a run root, never carry the continuation
> yourself — the receipt does. Repeat until a terminal disposition; the driver's observation budget
> bounds the loop and a budget expiry surfaces as `blocked` / `gate-halted`.

The same disposition is reachable from `finalize.rebase-continue` (the resolver path composes the
gate on completion); its re-entry is also `finalize.rebase`, not another `rebase-continue`.
`references/gate-failure.md` gets the same one-line route where it enumerates dispositions. The
embedded asset bundle is regenerated with the skill edit.

### 6. Attempt-token claim — verify, do not assume

The stub asserts that the `finalize.rebase` JSON truncates the attempt token to an 8-character base
suffix while the receipt holds 12, causing an `attempt-token-mismatch` on `rebase-continue`. The
code does not show this: `newRebaseAttempt` mints `<stamp>-<12 hex>` and every result path copies
`Attempt` verbatim into the document. The claim is **unverified** (learnings:
`groomed-root-cause-is-a-hypothesis`).

Required: a CLI-level round-trip test that runs `finalize rebase --json` through to a receipt and
asserts the document's `attempt` equals the on-disk receipt's `attempt` byte for byte. Fix only if it
reddens; otherwise the test stands as the guard and the reconcile log records that the claim did not
reproduce (the 0364 token was most likely mis-copied by the operator).

### 7. ADR

Record a short ADR refining ADR-0098 for finalize: *the finalize local gate's continuation is
persisted in the owned rebase receipt and never carried by the caller; the owner generation is
receipt-private and does not appear in any CLI document.* `relates_to: [98]`, `change: 396`.

## Testing

Seam-level, with the existing `fakeGate` / `seqGate` fakes and the workspace receipt on a temp
common dir:

- WAITING persists the pair into the receipt; all other receipt fields are byte-identical.
- A bare second `FinalizeRebase` with the identical request resumes: the seam receives the recorded
  `DriveID`/`Generation`, the rebase is not repeated, PASSED mints evidence, disposition is
  `rebased`/`unchanged`, and the pair is cleared.
- WAITING → WAITING keeps the pair; a third call still resumes the same drive id.
- FAILED and every HALTED cause clear the pair; the halted call is `blocked` / `gate-halted`; the
  following call starts a fresh drive (seam sees an empty continuation).
- A receipt-write failure on WAITING returns `blocked` / `receipt-write-failed` naming the drive id.
- `validateRebaseReceipt` rejects a half-set pair on both write and read.
- `finalize publish` still authorizes a rewrite from a receipt that carries the pair (and from one
  that does not).
- CLI: the `waiting` document carries `gate.continuation.drive_id` and no `generation` key; the
  §6 attempt round-trip test.
- Skill guard: the existing prose-contract tests over `docket-finalize-change` are extended so the
  `waiting` route is asserted present and bound to `finalize.rebase` (not `gate drive advance`),
  mutation-tested per AGENTS.md.

Whole suite at the build gate as usual (`build.test_command`).

## Files touched (expected)

- `internal/workspace/rebasereceipt.go` (+ tests) — pair fields and validation.
- `internal/app/finalize_rebase.go` (+ unit and integration tests) — continuation from receipt;
  persist/clear; request field retired; envelope projection.
- `internal/cli/finalize.go` (+ tests) — no flag change; round-trip test.
- `skills/docket-finalize-change/SKILL.md`, `references/gate-failure.md`, regenerated embedded
  assets and goldens.
- `internal/repoguard/prose_contracts_test.go` (or the nearest existing guard) — waiting-route guard.
- New ADR under `docs/adrs/`.
