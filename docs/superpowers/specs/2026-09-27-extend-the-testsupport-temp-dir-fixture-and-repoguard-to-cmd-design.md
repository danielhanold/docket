<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0398 — Extend the testsupport temp-dir fixture and repoguard to cmd/ real-process test packages](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0398-extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd.md)**
<!-- docket:backlink:end -->

# Extend the testsupport temp-dir fixture and repoguard to `cmd/` — design

## Context (verified against `main` @ e244dfd26, 2026-09-27)

Change 0373 added `internal/testsupport` (`TempDir`, `DrainOnCleanup`, tolerant
removal with a 4s `cleanupTolerance`) and the guard
`TestRealProcessPackagesUseFixtureTempDir` in
`internal/repoguard/tempdir_fixture_test.go`. The guard derives the real-process
package set by walking `scanRoot` (`const scanRoot = "internal"`) for test
packages whose `_test.go` files match `exec\.Command`, then rejects any bare
`<recv>.TempDir(` call (comments and string literals masked) and any aliased
`internal/testsupport` import in those packages. Its SCOPE header comment names
the `cmd/` gap as deliberate deferred follow-up — this change.

Current `cmd/` state:

- `cmd/docket` and `cmd/releasepkg` both contain `exec.Command` in test files, so
  both would be derived real-process packages once `cmd/` is walked.
  `cmd/genassets` has no such tests.
- 12 bare `t.TempDir()` sites: `cmd/docket/main_test.go` (4),
  `cmd/docket/config_cli_test.go` (4), `cmd/docket/devinstall_cli_test.go` (2),
  `cmd/docket/gate_cli_test.go` (1), `cmd/releasepkg/main_test.go` (1).
- `cmd/docket/gate_cli_test.go` defines a private `gateTempDir` (MkdirTemp +
  40×50ms ≈ 2s RemoveAll retry) with four call sites. The fixture's 4s tolerant
  removal strictly subsumes it.

Risk note: outside the gate tests, `cmd/` tests spawn only synchronous processes
(`go build`, run-to-completion binary invocations), so present flake exposure is
low. The value is (a) guard coverage so future `cmd/` real-process tests cannot
regress, and (b) removing the duplicated drain-then-retry helper.

## Design

1. **Widen the guard's walk roots.** Replace `const scanRoot = "internal"` with a
   named, explicit root list covering `internal` and `cmd` (e.g.
   `var scanRoots = []string{"internal", "cmd"}`), walked in sequence into the
   same package map. Keep it a single named declaration so the scope stays a
   visible, deliberate property (the reason 0373 used a named const). The
   fixture-package exemption stays keyed on `internal/testsupport`.
2. **Population floors.** Keep the existing `internal/process` floor and add a
   second floor asserting the derivation contains `cmd/docket`, so a broken
   `cmd/` walk cannot pass vacuously.
3. **Update the SCOPE header comment** to describe the widened scope and drop
   the "deferred follow-up" language (anchor on symbol names, never line
   numbers — ADR-0054).
4. **Adopt the fixture in `cmd/`.** Convert every bare `t.TempDir()` in the
   derived `cmd/` packages to `testsupport.TempDir(t)`, importing
   `github.com/danielhanold/docket/internal/testsupport` unaliased. The guard is
   package-scoped, so this includes files that do not themselves exec (e.g. the
   `XDG_CONFIG_HOME` temp dirs). Where a site was previously the `t.TempDir()`
   of a `testing.TB` other than `t`, pass that receiver.
5. **Delete `gateTempDir`** and point its call sites at `testsupport.TempDir(t)`.
   Its doc comment's rationale (supervisor exit window racing a single-shot
   RemoveAll) is already covered by the fixture's tolerant removal; if any gate
   test needs to wait for a supervisor explicitly, use `DrainOnCleanup` rather
   than re-growing a private helper.

## Verification

- `go test ./internal/repoguard/ ./cmd/...` green; then the full suite at the
  build gate (`build.test_command`).
- **Mutation test the widened guard** (AGENTS.md "A guard is code"): temporarily
  revert one converted `cmd/` site to bare `t.TempDir()` and confirm the guard
  reddens naming that file; temporarily drop `cmd` from the root list and
  confirm the new `cmd/docket` floor reddens. Restore both.
- Read the suite budget report for `BUDGET WATCH:` / `SERIAL CONFIRMED OVER
  BUDGET:` lines on the `cmd/` packages.

## Out of scope

- The `internal/` package set (already covered by 0373).
- Fixture semantics (`cleanupTolerance`, drain wiring) and the runner concurrency
  cap (ADR-0108).
- Widening the guard beyond `internal/` and `cmd/` (there are no other Go test
  roots today).
