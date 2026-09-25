<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0450 — Typed change.unblock operation to reverse change.block](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0450-typed-change-unblock-operation-to-reverse-change-block.md)**
<!-- docket:backlink:end -->

# Typed `change.unblock` and `change.revive` — design

**Change:** 0450 · **Type:** chore · **Groomed:** 2026-09-24 (interactive, Daniel)

## Problem

`change.block` (`blocked`) and `change.defer` (`deferred`) are typed metadata transactions, but
neither has a typed inverse. Clearing a blocker or reviving a deferred change is a hand edit of the
frontmatter plus a plain `git commit` on `docket`. That bypasses the transaction engine's
exact-version check and replay, leaves `BOARD.md` stale (`board-stale`, repaired only by a
human-typed `docket repository migrate`), and the docket-convention text still endorses it
("clearing a blocker or reviving is a one-line frontmatter edit, no move").

Observed on change 0444 (2026-09-24): halted → `change.block` (blocker: change 0446) → 0446 merged →
hand-edit commit `84074cd2` flipped `status: 'blocked'` → `'in-progress'` and cleared `blocked_by`
→ `change.resume-halted` then succeeded.

## Findings from tracing the existing implementation

The stub proposed a new operation that "restores the status the change had before it was blocked"
and asked whether to record that status at block time or derive it from history. Tracing the code
shows almost everything already exists:

1. **The domain transitions exist and are tested.** `domain.Unblock` (`internal/domain/actions.go`)
   moves `blocked` → `in-progress` and clears `blocked_by`. `domain.Revive` moves `deferred` →
   `proposed`, touching status only. Neither is reachable from any operation. The Go migration's
   planning-mutations design (0312, `2026-08-16-planning-mutations-board-and-adrs-design.md`) listed
   `change block`/`defer`/`kill` as operations and simply did not list their inverses — no spec or
   ADR decided against them.
2. **No pre-block status needs recording or deriving.** `domain.Block` accepts only `in-progress`
   (`requireStatus(c, "block", StatusInProgress)`), so every change blocked through `change.block`
   was `in-progress` before it — `domain.Unblock`'s fixed `in-progress` target is always correct.
   The one other producer of `blocked`, `domain.KillStackParent` (which blocks stack descendants from
   any non-terminal status), is wired to no operation today; see *Out of scope*.
3. **Halted-then-blocked already resumes.** `domain.Block`/`domain.Unblock` change frontmatter only,
   so a `## Run halted` section survives a block and an unblock, and `change.resume-halted` accepts
   the resulting `in-progress` record — exactly the 0444 path, where the hand edit made the same
   field changes `domain.Unblock` makes.
4. **The operation driver exists.** `executeChangeLifecycle` in `internal/app/change_lifecycle.go`
   already provides the exact-blob version pin, the domain legality gate, the `updated:` refresh,
   the `## Artifacts` re-render, and the atomic inline-board re-render for `change.block` and
   `change.defer`.

So this change adds no new mechanism, field, status, or policy: it exposes two existing domain
transitions through the existing lifecycle driver.

## Design

### Operations

| Operation | CLI | Domain call | Transition | Fields written |
|---|---|---|---|---|
| `change.unblock` | `docket change unblock` | `domain.Unblock` | `blocked` → `in-progress` | `status`, `blocked_by` cleared, `updated` |
| `change.revive` | `docket change revive` | `domain.Revive` | `deferred` → `proposed` | `status`, `updated` |

Both also re-render the record's `## Artifacts` block and, when the inline board surface is on,
`BOARD.md` — in the same single metadata commit, via the existing driver.

### Request and result

Each request is the pinned-record shape `change.block` uses, minus the authored field:

```go
type ChangeUnblockRequest struct {
	ChangeID int    `json:"change_id" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Version  string `json:"version" docket:"required"`
}
// ChangeReviveRequest: identical fields.
```

- No `reason` field. The commit subject is the driver's existing `change NNNN → <status>`, and the
  cleared `blocked_by` text remains in git history. (YAGNI — no consumer reads an unblock reason.)
- Result: the existing `ChangeLifecycleResult` (id, resulting status, committed revision,
  findings), unchanged.
- Request-shape validation reuses `validateLifecycleShape("change_id", …)`: non-positive id, empty
  path, empty version each return `invalid-input` with no engine call.

### Behaviour

- `ChangeUnblock` / `ChangeRevive` in `internal/app/change_lifecycle.go` validate shape, then call
  `executeChangeLifecycle(ctx, deps, repoDir, OperationChangeUnblock|OperationChangeRevive, …,
  action, nil)` with `action` wrapping `domain.Unblock` / `domain.Revive` and **no section edits**.
- **Wrong source status is refused, never guessed.** `domain.Unblock` on anything but `blocked`, or
  `domain.Revive` on anything but `deferred`, returns the domain's `requireStatus` failure, which
  the driver maps to `invalid-state` with the domain's reason token — nothing is written.
- Version mismatch / contention behave exactly as for `change.block` (exact-blob expectation,
  exact-lease push).
- **Revive leaves the body alone.** `## Why deferred` is kept: nothing reads its presence, a later
  `change.defer` replaces it (`SectionReplace`), and removing it would need a special case for
  records that lack the section (`SectionRemove` refuses a missing heading).
- **Revive keeps `branch:` and `claimed_at:`**, per `domain.Revive`'s existing contract ("its
  recorded branch and claim stamp stay readable for whoever revives it"). A later `change.claim`
  re-mints `branch:` and re-stamps `claimed_at:`; lease/reclaim logic only acts on `in-progress`.
- Update the `change_lifecycle.go` file header and `lifecycleFieldValue` comments, which currently
  describe only block and defer.

### Wiring (every site that lists `change.defer` today)

- `internal/app/change_lifecycle.go` — `OperationChangeUnblock = "change.unblock"`,
  `OperationChangeRevive = "change.revive"`, the two request types and entry points.
- `internal/cli/change.go` — `unblock` and `revive` subcommands via `changeSubcommand(…,
  EffectMetadataWrite)`, decoding the request with `decodeRequestFlag`, registered alongside `block`
  / `defer` (so they appear in the capability catalog).
- `internal/app/schema_registry.go` — two rows with `ChangeLifecycleResult{}` as result.
- `internal/cli/install.go` — `"change unblock"` and `"change revive"` in `assetIndependent`
  (`TestAssetIndependentSetExact` enforces the correspondence with the Cobra tree).
- Tests that enumerate operations: `internal/app/schema_tags_test.go`,
  `internal/app/shadow_test.go`, `internal/cli/change_test.go`.

### Documentation

- `skills/docket-convention/SKILL.md`, *Lifecycle → Rules*: replace "clearing a blocker or reviving
  is a one-line frontmatter edit, no move" with wording that names the operations — clear a blocker
  with `change.unblock` (→ `in-progress`), revive with `change.revive` (→ `proposed`); neither moves
  the file. Keep the lifecycle diagram's edges; they already show `blocked ──clears──▶ in-progress`
  and `revive → proposed`.
- Before editing, grep the shipped skill/agent text and README for any other hand-edit instruction
  for unblocking or reviving (at grooming time the convention sentence was the only one found).

## Testing

Extend the existing `change.block` / `change.defer` test sets rather than creating new harnesses:

- **Request shape** (`change_lifecycle_test.go`): non-positive id, empty path, empty version →
  `invalid-input`, no engine call — for both operations.
- **Plan behaviour** (`change_lifecycle_test.go`, the `baseLifecycleOp` pattern): unblock of a
  `blocked` record yields `in-progress` with `blocked_by` null and `updated` refreshed; revive of a
  `deferred` record yields `proposed` with `## Why deferred`, `branch`, and `claimed_at` byte-intact;
  the artifact block and board are re-rendered.
- **Refusals**: unblock of each non-`blocked` status and revive of each non-`deferred` status →
  `invalid-state` carrying the domain reason; nothing written.
- **Integration** (`change_integration_test.go`): one applied unblock and one applied revive
  through the real engine, asserting the receipt, commit subject, and board row.
- **Resume regression** (the 0444 path): `change.halt` → `change.block` → `change.unblock` →
  `change.resume-halted` succeeds and removes `## Run halted`.
- Schema-tag, shadow, CLI, and asset-independence tests updated for the two new ids.

## Out of scope

- **Automatic unblocking** when a dependency lands. `blocked_by` is free text; judging whether it
  has cleared is a human/model call (ADR-0012). Machine-trackable dependencies already use
  `depends_on:`, which clears itself.
- **Stack-kill descendants.** `domain.KillStackParent` is not wired to any operation, and its
  descendants' exits are re-scope, re-parent, or kill — not unblock. It also blocks from statuses
  other than `in-progress`, so a bare unblock would restore the wrong status. Whoever wires stack
  kill owns that recovery.
- **`finalize.block` / `finalize.clear-block`** — a different mechanism (`## Finalize blocked` on an
  `implemented` change) with its own typed clear.
- **Changing `change.block` or `domain.Revive` semantics** — including clearing `branch:` /
  `claimed_at:` on revive.
