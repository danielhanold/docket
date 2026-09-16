<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0427 — Verdict-path gate recovery never binds the run epoch's worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-16-0427-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt.md)**
<!-- docket:backlink:end -->

# Change 0427: bind the worktree during verdict recovery

## Problem

In `resolveGateOwnership` (`internal/app/rungate_verdict.go`), recovery from an unconfirmed reservation and adoption of a sole committed proof both call `ConfirmGateClaim` with an empty worktree. A fresh epoch recovered exclusively through either path therefore retains an empty `EpochRecord.Worktree`; cancellation cannot locate that worktree for fencing or teardown.

## Fix

After the existing receipt checks select the claimed change, resolve its logical feature path using the existing repository and change-reading seams: `filepath.Join(repo.PrimaryWorktree, ".worktrees", change.Slug)`. This is the normal `ChangeClaim` derivation. Thread the necessary existing context into ownership resolution and pass that path to both recovery calls to `ConfirmGateClaim`. Correct the adjacent comments that currently justify the empty argument.

Use the selected change's authoritative metadata and canonical primary repository identity, never the caller's directory, branch spelling, or a scan for candidate worktrees. The feature directory need not exist yet; retain the current fence's canonicalization at comparison time. Do not create or prepare a workspace during verdict recovery. If repository/change identity cannot be resolved, stop through the existing `gate-stop … gate-unavailable` / `ReasonGateProofUnavailable` path before confirming; do not silently substitute an empty path.

Keep the existing confirmation, epoch-binding, and mutation-fence mechanisms. Preserve receipt matching, ambiguity refusal, continuation handling, retry accounting, and epoch-less behavior. At most a small private helper is needed to share the two recovery calls' path resolution.

## Acceptance

Extend the existing verdict/fence tests with both recovery shapes: an unconfirmed reservation with its exact committed receipt, and no binding with one matching proof. Start with a real fresh epoch whose worktree is empty; neither normal claim confirmation nor fixture setup may pre-bind it.

For each shape, drive `RunGateVerdict` and verify the selected change's expected feature path is stored. Cancel through `RunCancel`, then assert that a workflow mutation from that feature worktree is refused specifically as `run-cancelled`. Also cover recovery before the feature directory exists and an unresolved identity refusing before confirmation. Retain existing sibling-proof and ambiguous-proof rejection tests.

Mutation-check each fixed call separately by restoring its empty argument: the corresponding binding and post-cancel fencing checks must fail. Run probes uncached and restore the fixed source. During implementation, run the complete configured build suite through the Go runner and inspect its budget report.

## Scope

Only the two recovery legs, their directly needed context plumbing/comments, and regression tests. No new subsystem, command, configuration, schema, ledger, migration, generic recovery framework, or redesign of cancellation/retry semantics. Repairing already-confirmed historical bindings and unrelated partial-write recovery are excluded.

No dependencies or stacking are needed. Changes 0375 and 0428 are done; 0422 independently edits retry handling in the same file, so preserve its changes if it lands first. Existing ADR-0107 and ADR-0118 remain unchanged.
