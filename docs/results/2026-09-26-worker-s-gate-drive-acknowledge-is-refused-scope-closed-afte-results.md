<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0459 — Worker's gate.drive.acknowledge is refused scope-closed after the parent claims its WAITING drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0459-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte.md)**
<!-- docket:backlink:end -->
# Worker's gate.drive.acknowledge is refused scope-closed after the parent claims its WAITING drive — Results

**Human action:** No action is required before merge. Read the behavior change below: several existing gate-drive tests now expect the new `scope-transferred` refusal where they used to expect `scope-closed`.

## Outcome

Before this change, a build worker could report `BLOCKED` for work it had actually finished. The sequence was: the worker's test run outlived one observation slice, so it handed the run off (`WAITING`); the parent claimed the run and finished it; the worker then tried to acknowledge its original test scope. That scope had been closed when the parent claimed it, so the acknowledge was refused with `scope-closed`, and the refusal message told the worker to return `BLOCKED`.

What changed:

- **New refusal, `scope-transferred`.** A child-side acknowledge, scoped test start, or scope reservation on a scope that a parent claim or takeover closed now returns `scope-transferred`. Its message says the parent took over the scope's run and tells the worker to report on the verdict its continuation supplied. It never says "return BLOCKED". A scope that was closed by the worker's own final acknowledgement still returns `scope-closed`. Parent-side paths (takeover, bind-scope-change) are unchanged.
- **Worker contract** (`docket-build-task`): a worker that handed off never acknowledges or reuses its original scope. It reports on the continuation's terminal verdict and runs further tests only under a fresh scope.
- **Parent contract** (`docket-build`): the continuation carries the claimed run's id and verdict, states that the original scope is closed, and includes a freshly prepared scope when more test runs may be needed.
- **Guards**: `internal/repoguard` prose-contract rows pin the new contract sentences (mutation-tested).

Departures from the design:

- The spec said the existing takeover tests would stay unchanged. That was wrong: several tests assert what a *child* sees on a scope closed by takeover or claim, and by the spec's own rule that is now `scope-transferred`. Those assertions were updated (takeover, admission-successor, driver, driver-concurrency, scope, and handoff tests). The takeover path's own result is still `scope-closed`.
- Review fixes widened the change a little: the close race inside the worker's own acknowledge (it loses to a concurrent claim) now also returns `scope-transferred`, and a scoped start checks the child capability before revealing whether the scope was closed.
- The skill size budgets in `internal/repoguard/budgets_test.go` were raised to fit the new contract prose.

## Verification performed

- Each task ran its focused package tests through the gate driver; the claim → advance → acknowledge regression was red before the fix and green after.
- Full suite (`go run ./cmd/docket development test`) passed at 9ff69611 before review: 54/54 files. `BUDGET WATCH` lines were reported for the long integration/race files under parallel load; there was no serial-confirmed breach.
- Whole-branch review (standard rung) returned three minor findings, all fixed in-branch (551b8344, f257fc1d). The final certification suite run is recorded in the PR's build-evidence block.
- Guard mutation probes: deleting or rewording each guarded contract sentence turned `TestProseContracts` red.

## Known issues and follow-ups

### Spec's "takeover tests unchanged" line was inaccurate

This matters when someone reads the spec against the diff. They will see takeover-related tests edited even though the spec said they would not be. Confirmed. Only child-facing assertions changed; the parent takeover path still halts with `scope-closed`. No action is needed beyond knowing this.
