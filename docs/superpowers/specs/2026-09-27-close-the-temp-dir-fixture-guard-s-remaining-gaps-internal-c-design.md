<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0462 — Close the temp-dir fixture guard's remaining gaps (internal/cli gateTempDir, scan-root removal)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-27-0462-close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c.md)**
<!-- docket:backlink:end -->

# Close the temp-dir fixture guard's remaining gaps — design

Change 0462 (discovered from 0398). Groomed 2026-09-27.

## Background

`TestRealProcessPackagesUseFixtureTempDir` (`internal/repoguard/tempdir_fixture_test.go`) enforces that
every temp dir in a *real-process* test package (one whose `_test.go` files match the `exec.Command`
shape) comes from `testsupport.TempDir`, whose cleanup drains detached writers and retries removal.
Change 0398 extended it to `cmd/` and reported two residual gaps:

1. `internal/cli/gate_test.go` defines a private `gateTempDir` helper built on `os.MkdirTemp` plus a
   hand-rolled 40×50ms `RemoveAll` retry. The guard's only violation shape is `<ident>.TempDir(`, so
   `os.MkdirTemp` is a blind spot.
2. The walk is bounded by a hand-listed `scanRoots = {"internal","cmd"}`. Deleting a root together
   with its `realProcFloors` entry passes silently.

## Design

### 1. Replace `gateTempDir` with the fixture

Delete `gateTempDir` (and its doc comment) from `internal/cli/gate_test.go`; convert every call site
to `testsupport.TempDir(t)`. The fixture's drain-then-retry removal (4s tolerance window) covers the
supervisor exit-window race the helper's comment describes. Drop now-unused imports.

### 2. Ban `MkdirTemp` in real-process packages unless justified at the site

- Add a violation shape: any executable `<ident>.MkdirTemp(` call (regex keyed on the receiver-call
  shape, like `tempDirCallRe`, not on the `os` spelling), evaluated on the same comment- and
  string-masked view (`maskProse(b, true)`) the `TempDir` check uses.
- A call is exempt only when its own line or the line immediately above carries the marker
  `// tempdir-exempt: <reason>` with a non-empty reason. Read the marker from the **raw** bytes (the
  masked view blanks comments). A marker with an empty/whitespace-only reason does not exempt.
- The fixture package (`internal/testsupport`) stays exempt by construction, as today.
- Violation message names the file and line and says to use `testsupport.TempDir(t)` or add a
  justified `tempdir-exempt` marker.
- Update the file-header comment: the LIMITATION note gains the MkdirTemp shape and the marker rule.

Why a marker rather than a fixture extension: the legitimate `MkdirTemp` sites need lifetimes the
per-test fixture deliberately does not provide (no `t` in `TestMain`; process-lifetime shared dirs
created under `sync.Once`; failure evidence that must survive; a mandated `/tmp` parent). None of them
is a per-test dir deleted at test end, so the teardown race the fixture closes does not apply. Adding
fixture modes for them was considered and rejected (YAGNI; fixture behavior is out of scope).

Sites that receive a marker (at groom time — the build re-derives the list by grep, never trusts this
enumeration):

| Site | Reason class |
|---|---|
| `cmd/docket/main_test.go` `TestMain` | binary built once before any test (no `t`) |
| `cmd/releasepkg/main_test.go` `TestMain` | same |
| `internal/app/finalize_e2e_test.go` `e2eNode` (`sync.Once`) | process-lifetime XDG dir |
| `internal/app/finalize_e2e_test.go` `sharedBinaries` (`sync.Once`) | process-lifetime shared binaries |
| `internal/app/status_git_test.go` `backgroundOffGitEnv` (`sync.Once`) | process-lifetime gitconfig |
| `internal/gatedrive/driver_test.go` `sampleWorktree` (`sync.Once`) | process-lifetime sample dir |
| `internal/release/package_integration_test.go` determinism mismatch | failure evidence must survive |
| `internal/install/references_test.go` `TestDeriveVersionReferencesCanonicalizesTmpAliases` | must live under `/tmp` for the macOS alias |
| `internal/app/gate_drive_test.go` runroot | parent is already a `testsupport.TempDir` |

Each marker's reason is a short, site-specific sentence, not a generic class label.

### 3. Derive the scan population from the whole repo

- Delete `scanRoots`. Build the per-package `_test.go` map from `repoguard.MaintainedFiles(root)`
  filtered to `*_test.go` — the shared walker other repoguard tests already use, with its categorical
  exclusions (`.git`, `.worktrees`, `testdata`, `docs`, `tests/fixtures`, `internal/install/legacydata`).
  Narrowing coverage then requires editing the shared exclusion rules every guard depends on.
- Keep `realProcFloors` (`internal/process`, `cmd/docket`) as the derivation's rot/non-vacuity check.
- Rewrite the header SCOPE note and the `realProcFloors` doc to describe whole-repo derivation (drop
  the "named scanRoots list" rationale).

## Verification (mutation tests — each must redden the guard, then be reverted)

1. Restore `gateTempDir` in `internal/cli/gate_test.go` → red (unmarked `MkdirTemp`).
2. Strip one site's `tempdir-exempt` marker → red.
3. Blank one marker's reason (`// tempdir-exempt:`) → red.
4. Add an unmarked `os.MkdirTemp` helper in a different real-process package → red.
5. Put a `tempdir-exempt` marker on a line two lines above a `MkdirTemp` call → red (adjacency is enforced).
6. Coverage: temporarily make the walk skip `cmd/` (e.g. an extra exclusion) → red via the `cmd/docket`
   floor; confirm there is no guard-local list left to shrink.
7. A `MkdirTemp` spelling inside a comment or string literal → stays green (masking works).

The full suite (`build.test_command`) must pass green after the change.

## Out of scope

- Any change to `testsupport.TempDir`'s behavior or new fixture modes.
- Temp dirs in packages that do not spawn real processes.
- Re-opening 0398's already-converted `cmd/` sites.
- `os.CreateTemp` (temp *files*), and calls through interface values or name-shadowing helpers (the
  existing, asserted limitation stands).
