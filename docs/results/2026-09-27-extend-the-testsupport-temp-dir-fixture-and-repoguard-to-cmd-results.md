<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0398 — Extend the testsupport temp-dir fixture and repoguard to cmd/ real-process test packages](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0398-extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd.md)**
<!-- docket:backlink:end -->
# Extend the testsupport temp-dir fixture and repoguard to cmd/ real-process test packages — Results

**Human action:** None required beyond the normal PR review. This change touches only test code, and the widened guard was mutation-tested during the build.

## Outcome

Change 0373 added a shared temp-dir helper for tests that start real processes. That helper, `testsupport.TempDir`, tolerates a slow child process still holding the directory at cleanup. 0373 also added a guard test that rejects bare `t.TempDir()` calls in those packages, but the guard only covered `internal/`. The `cmd/` test packages were left unprotected and kept their own copy of the retry logic.

This change closes that gap:

- **Guard now covers `cmd/`.** The guard (`TestRealProcessPackagesUseFixtureTempDir`) scans an explicit root list of `internal` and `cmd`. A second minimum-coverage check now requires the scan to find `cmd/docket`, so a broken `cmd/` scan fails loudly instead of passing with nothing checked.
- **Bare temp dirs converted.** All 12 bare `t.TempDir()` calls in `cmd/docket` and `cmd/releasepkg` now use `testsupport.TempDir(t)`.
- **Private helper removed.** The `gateTempDir` helper in `cmd/docket/gate_cli_test.go` is deleted, and its four callers now use the shared helper.

## Verification performed

- The guard failed first, as expected, with exactly the 12 `cmd/` violations. It passed after the conversions.
- Mutation probes:
  - Reverting one converted site in `cmd/docket` failed the guard, naming that file.
  - Reverting the converted site in `cmd/releasepkg` failed the guard, naming that file.
  - Removing `cmd` from the root list failed the new `cmd/docket` minimum-coverage check.
- The gate end-to-end tests passed three times in a row after `gateTempDir` was removed. The `cmd/...` and `internal/repoguard` tests also passed.
- The full-suite build gate ran at the final head. Its result is recorded in the build-evidence block of the PR.

## Known issues and follow-ups

### Removing both the scan root and its minimum-coverage check passes silently

If someone deletes `cmd` from the guard's root list and also deletes the `cmd/docket` minimum-coverage entry, the guard still passes and no longer checks `cmd/`. The probe confirmed this. It needs two deliberate edits to the same test, which would show up plainly in review, so it is the accepted remaining gap. No action is needed.

### A second private `gateTempDir` remains in `internal/cli/gate_test.go`

`internal/cli/gate_test.go` defines the same drain-then-retry helper. The guard does not flag it because it creates its directory with `os.MkdirTemp` rather than `t.TempDir()`. This change's scope excludes `internal/`, so the helper was left alone. Suggested next step: capture a small follow-up change to switch it to `testsupport.TempDir`.
