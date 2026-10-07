<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0480 — Report killed changes truthfully in finalize cleanup](../../changes/archive/2026-10-01-0480-check-finalize-cleanup-s-not-final-message-for-killed-change.md)**
<!-- docket:backlink:end -->

# Report killed changes truthfully in finalize cleanup — design

## Problem

`FinalizeCleanup` (`internal/app/finalize_cleanup.go`) switches on the change's status and
handles only `done` (the destructive suffix) and `stacked-merged` (deliberately retained).
Nothing upstream filters by status — `loadCloseoutContext` loads any change — so a `killed`
change falls through to the `default` branch and, absent aborted-rebase scratch, is refused:

```
result: invalid-state   disposition: pending   reason: not-final
"change is not final and carries no aborted-rebase scratch to clear; nothing to clean"
```

`killed` is a final status, so the message is false. Worse, two skill documents promise the
opposite of what the code does:

- `skills/docket-convention/references/close-out.md`, step 4, routes both final transitions
  (`done` and `killed`) through `finalize.cleanup --id <id>` and says "a kill leg whose branch
  never merged keeps its feature ref rather than losing it" — implying cleanup processes a
  killed change.
- `skills/docket-implement-next/references/edge-paths.md` (reconcile-kill) says "The
  reference's cleanup step prunes any feature worktree/branch already created."

Failure scenario: a resumed `in-progress` change with a prepared workspace is reconcile-killed;
the agent follows the close-out reference and runs `finalize.cleanup`; it gets `invalid-state`,
which under the caller's abort-and-report posture surfaces as a failure *after* the kill already
archived — and the workspace and branch remain with no truthful explanation. (A `proposed`-kill
is unaffected: `docket-new-change` already treats the cleanup step as a no-op.)

## Decision

Report a killed change as a deliberate retention, mirroring the existing `stacked-merged` case.
Do not implement killed-change cleanup here — that is change 0483.

### Code

In `FinalizeCleanup`'s status switch, add an explicit case beside `domain.StatusStackedMerged`:

- `case domain.StatusKilled:` → `newCleanupResult(OperationFinalizeCleanup, ResultNoOp, …)`
  with `Disposition: CleanupDispRetained`, a new stable reason constant
  `ReasonCleanupKilledRetained = "killed-retained"`, and a message stating the real condition,
  e.g. "change is killed; finalize cleanup removes only a merged change's resources — its
  workspace and branches are retained".
- Update the `CleanupDispRetained` doc comment to list the killed case alongside stacked-merged
  and gate runs.
- The `default` branch and `ReasonCleanupNotFinal` are unchanged: they keep covering the
  genuinely non-final statuses (including the aborted-rebase scratch exception, which runs
  before the refusal). The killed case does not consult aborted-rebase scratch — a kill is only
  reachable from `proposed`/`in-progress`, never from the `implemented` state where finalize
  rebases run.

`no-op` (not `invalid-state`) is deliberate: the kill-path caller is following the documented
close-out sequence, and nothing is wrong — the resources are retained by policy until 0483 lands.
A `no-op` lets abort-and-report callers proceed without a spurious failure, and when 0483 replaces
the case with real cleanup no caller or doc needs to change shape.

### Tests

- A unit case in `internal/app/finalize_cleanup_test.go` (follow the existing stacked-merged
  retention test's fixture pattern): a `killed` archived change yields result `no-op`,
  disposition `retained`, reason `killed-retained`, and touches no workspace or ref.
- Pin the wire spelling in `TestFinalLifecycleCodeSpellings`:
  `{"ReasonCleanupKilledRetained", …, "killed-retained"}`.
- Mutation check: remove the new `case` and confirm the unit case reddens (it falls through to
  `invalid-state` / `not-final`).

### Docs

Edit the source skill files and keep their embedded copies under
`internal/assets/embedded/tree/skills/…` byte-identical (use whatever sync mechanism/guard the
repo already enforces for embedded assets):

- `docket-convention/references/close-out.md` step 4: state that on the kill path
  `finalize.cleanup` currently retains the change's workspace and branches and reports
  `no-op` / `retained` / `killed-retained` (manual removal until change 0483); remove the
  sentence implying a kill leg's resources are processed. Keep the `done`-path text as is.
- `docket-implement-next/references/edge-paths.md` reconcile-kill paragraph: replace "The
  reference's cleanup step prunes any feature worktree/branch already created" with the truthful
  retained outcome.
- Grep the skills tree (and embedded copy) for any other claim that cleanup prunes a killed
  change's resources and correct it the same way; derive the sites from the grep, don't trust
  this list.

## Out of scope

- Actually removing a killed change's workspace or branches (change 0483).
- Rewording any other lifecycle message, or changing the `not-final` / `stacked-merged` paths.
- Changing which statuses `finalize cleanup` accepts beyond reporting the killed case.

## Verification

The full suite (`build.test_command`) is green, the new unit case passes and reddens under the
mutation above, and `docket finalize cleanup --id <killed id> --json` against a killed archived
change returns `no-op` / `retained` / `killed-retained`.
