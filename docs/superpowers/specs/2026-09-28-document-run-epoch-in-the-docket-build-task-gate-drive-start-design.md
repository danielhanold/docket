<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0467 — Scoped gate starts inherit the run epoch; thread it through the build chain](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0467-document-run-epoch-in-the-docket-build-task-gate-drive-start.md)**
<!-- docket:backlink:end -->

# Design — scoped gate starts inherit the run epoch (change 0467)

## Problem

During change 0461's implement-next run, a `docket-build-task` worker's `gate.drive.start` was refused
`stale-run-epoch` until the worker improvised `--run-epoch`. Tracing the chain shows the gap is wider than
the worker argv:

- The parent's arm prints `gate-armed <key> <epoch> <dispatch-context>`, but the managed run-gate block
  (`cursor-rules/run-gate.md`, rendered into `AGENTS.md`/`CLAUDE.md`) tells the parent to copy only the
  dispatch context into the dispatch prompt. The epoch never reaches implement-next.
- `docket-implement-next`, `docket-build`, and `docket-build-task` do not mention the run epoch at all:
  `docket-build`'s `gate.drive.prepare-scope` argv has no `--run-epoch`, its start-ready scope bundle has
  no epoch, and the build-owner `gate.drive.start` calls (implement-next's Step-6 evidence re-mint,
  `docket-build`'s final suite gate) have no `--run-epoch`.
- Code/comment drift in the driver: `ScopeRecord.RunEpochID` (`internal/gatedrive/scope.go`) is documented
  as travelling "onto each scoped start's worktree execution slot so an omitted or stale epoch cannot
  detach the worktree", but `Driver.admitScopedWorktree` builds the slot's admission record from the
  caller's `req.RunEpochID`, ignoring the epoch the scope pinned. A worker that omits the flag presents an
  empty epoch against an epoch-owned slot and is refused by the run-epoch fence in
  `reserveWorktreeExecution`.

## Decision

Make the driver honour its own documented contract — a **scoped** start takes its run epoch from the scope
the parent prepared — and thread the epoch through prose only as far as the calls that actually create a
scope or start a scope-less drive. Workers never handle the epoch.

### 1. Driver: scoped starts inherit the scope's epoch

In the scoped start path (the one that validates `ScopeID` + `ChildCapability` and reaches
`admitScopedWorktree`), resolve the **effective run epoch** once, after the scope record is loaded and
before admission:

| scope `RunEpochID` | presented `--run-epoch` | effective epoch |
|---|---|---|
| non-empty `E` | empty | `E` (inherited) |
| non-empty `E` | `E` | `E` |
| non-empty `E` | `F ≠ E` | refuse — the existing `scope-identity-mismatch` refusal ("use the complete identity bundle from your dispatch prompt"); nothing reserved, nothing charged |
| empty (legacy v2 scope, or prepared with no epoch) | any | the presented value, unchanged from today |

The effective epoch replaces `req.RunEpochID` everywhere the scoped start uses it: the admission record,
the finished-incumbent reconciliation call, the start ticket, and the drive record. The run-epoch fence
itself, `stale-run-epoch` semantics, the settlement seam, and scope-less starts are untouched. Fix the
`ScopeRecord.RunEpochID` / `ScopeRequest.RunEpochID` comments only if they end up inaccurate — the goal is
that the existing comment becomes true.

Tests (in `internal/gatedrive`, beside the existing epoch/admission tests):
- a scope prepared with epoch `E`, then a start with no `--run-epoch` over an `E`-owned released slot, is
  admitted and the slot/drive carry `E` (today: refused `stale-run-epoch` — RED first);
- presenting the same `E` still admits;
- presenting `F ≠ E` refuses `scope-identity-mismatch` and leaves the slot and budget untouched;
- a scope with no epoch keeps today's behaviour (the presented value governs).
Mutation check: revert the inheritance and watch the first test redden.

### 2. Prose: thread the epoch to where scopes and scope-less drives start

- **Parent (managed run-gate block, source `cursor-rules/run-gate.md`)**: step 1 copies the `<epoch>` into
  the implement-next dispatch prompt alongside the `<dispatch-context>`. Regenerate the managed block in
  `AGENTS.md`/`CLAUDE.md` and the embedded copy (`internal/assets/embedded/tree/...`) through the normal
  generation path — never hand-edit the managed block.
- **`docket-implement-next`**: where it already says to pass the dispatch context into every
  `gate.drive.prepare-scope` / `gate.drive.start --gate-context` and the Step-2 claim, add the run epoch:
  pass it as `--run-epoch` to every `gate.drive.prepare-scope` and to its own build-owner
  `gate.drive.start` (the Step-6 evidence re-mint), and hand it to the build role. Omit it only when the
  prompt carried none (a `gate-unarmed` or ungated run).
- **`docket-build`**: its `gate.drive.prepare-scope` argv gains `--run-epoch <epoch>` (from its prompt, when
  present), and its final build-owner `gate.drive.start` gains `--run-epoch`. The start-ready scope bundle
  handed to workers does **not** gain an epoch — add one sentence that the scope carries the run epoch, so
  the worker passes none.
- **`docket-build-task`**: its `gate.drive.start` argv is unchanged; add one clause that the run epoch rides
  on the prepared scope and the worker neither receives nor invents one (a worker that improvises a
  different value is now refused `scope-identity-mismatch`).
- Regenerate every embedded skill copy.

### 3. Guard

A `internal/repoguard` prose-contract test (beside `gatedriver_test.go` / `prose_contracts_test.go`)
asserting, keyed on syntactic shape rather than an enumerated spelling list:
- every `gate.drive.prepare-scope` invocation in `docket-build` and `docket-implement-next` carries
  `--run-epoch`;
- every build-owner `gate.drive.start` (`--owner build`) invocation in those two skills carries
  `--run-epoch`;
- the managed run-gate source tells the parent to copy the epoch into the dispatch prompt.
Derive the invocation sites from a scan of the skill files, not a hand list. Mutation-test it: strip one
`--run-epoch` and watch it redden.

## Alternatives considered

- **Prose-only, full chain** (thread `--run-epoch` into the worker argv too): leaves the driver's scope
  comment false and makes every worker a place the epoch can drop out again; rejected in grooming.
- **Stub as written** (worker argv + scope bundle only): the worker would have no epoch to pass, because
  nothing upstream delivers one.
- **Derive the epoch from the dispatch-context token** (nested `prepare-scope` inherits the outer scope's
  epoch via `--gate-context`, eliminating prose threading entirely): the arm's outer scope is prepared
  without an epoch today, so this needs arm-order and fencing work; YAGNI until the explicit threading proves
  insufficient.

## Out of scope

- Changing the run-epoch fence, `stale-run-epoch` refusal semantics, epoch settlement, or scope-less start
  behaviour.
- Why 0461's refusal named a foreign incumbent epoch: it is the expected consequence of presenting an empty
  epoch against an epoch-owned slot, so no separate defect is pursued.
