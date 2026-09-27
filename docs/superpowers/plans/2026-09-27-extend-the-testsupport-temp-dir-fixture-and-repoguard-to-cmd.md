<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0398 — Extend the testsupport temp-dir fixture and repoguard to cmd/ real-process test packages](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0398-extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd.md)**
<!-- docket:backlink:end -->
# Extend the testsupport Temp-Dir Fixture and Repoguard to `cmd/`: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build (docket's build role) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `TestRealProcessPackagesUseFixtureTempDir` also covers the `cmd/` real-process test packages. Every bare `t.TempDir()` in `cmd/docket` and `cmd/releasepkg` becomes `testsupport.TempDir(t)`, and the private `gateTempDir` helper in `cmd/docket/gate_cli_test.go` is deleted in favor of the shared fixture.

**Architecture:** The guard in `internal/repoguard/tempdir_fixture_test.go` replaces its single `const scanRoot = "internal"` with a named root list (`scanRoots`) walked into one package map. It adds a separate named population-floor list (`realProcFloors`) that now also requires `cmd/docket`. The 12 `cmd/` sites and the 4 `gateTempDir` call sites switch to the existing `internal/testsupport.TempDir`. The fixture itself does not change.

**Tech Stack:** Go 1.26 (stdlib `testing`, `path/filepath`, `slices`); the `internal/testsupport` fixture from change 0373; `internal/repoguard` source-shape guards.

**Spec:** `docs/superpowers/specs/2026-09-27-extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd-design.md` (on the `docket` metadata branch).

## Global Constraints

- The fixture-package exemption stays keyed on `internal/testsupport`.
- The walk roots are one named declaration covering exactly `internal` and `cmd`. There are no other Go test roots today, so widening past these two is out of scope.
- Keep the existing `internal/process` population floor and add a `cmd/docket` floor.
- Import `github.com/danielhanold/docket/internal/testsupport` **unaliased** (the guard rejects an aliased import).
- Comments anchor on symbol names or quoted clauses, never on line numbers (ADR-0054; `TestCommentAnchorStyle`).
- Do not change fixture semantics (`cleanupTolerance`, drain wiring) or the runner concurrency cap (ADR-0108).
- Out of scope: the `internal/` package set, including the twin `gateTempDir` in `internal/cli/gate_test.go`. It uses `os.MkdirTemp`, so the guard does not flag it, and it belongs to 0373's domain. Leave it untouched.
- Every mutation probe and re-verification run uses `go test -count=1`. `cmd/docket` and `cmd/releasepkg` build their binary in `TestMain`, and Go's test cache key does not cover that binary (learning `cached-runner-serves-a-mutated-tree`).
- Mutation restores use a `cp` backup and a `mv -f` back, never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).
- The whole-suite gate is `build.test_command` (`go run ./cmd/docket development test`) and is owned by docket-build's end gate. Read its budget report for `BUDGET WATCH:` and `SERIAL CONFIRMED OVER BUDGET:` lines on `cmd/` packages.

## Review Focus

1. **Stale cached pass under mutation.** A mutation probe that reports `ok … (cached)` proves nothing. Tasks 1 and 2 use `-count=1` in every run, and each mutation step's expected output names the specific file or floor.
2. **Dropping `cmd` from the roots must redden.** The spec requires the `cmd/docket` floor to fire when `cmd` is removed from the walk. To keep that true, the floors live in a list separate from the roots, so one edit cannot drop both. Mutation M3 in Task 1 pins this.
3. **`cmd/releasepkg` must be derived, not just `cmd/docket`.** The floor covers only `cmd/docket`. Mutation M2 in Task 1 is a positive control showing a regressed `cmd/releasepkg` site is still reported.
4. **Removal after build or install under a fixture dir.** Some tests put `go build` outputs or a `development install` HOME in these dirs. `testsupport.TempDir` removes with `os.RemoveAll` plus retries (a superset of `t.TempDir`'s single removal), so any test that passed before should still pass. Task 1 step 5 runs `./cmd/...` with `-count=1` to confirm.
5. **Supervisor exit window after deleting `gateTempDir`.** The gate end-to-end tests previously relied on a 2s retry. The fixture's 4s tolerant removal subsumes it. Task 2 runs both gate tests with `-count=3` to check teardown under repetition.

---

## File Structure

- Modify: `internal/repoguard/tempdir_fixture_test.go`. The header SCOPE comment, `scanRoot` → `scanRoots` + `realProcFloors`, the multi-root walk, the explicit fixture exemption path, and the floor loop.
- Modify: `cmd/docket/main_test.go` (4 sites), `cmd/docket/config_cli_test.go` (4), `cmd/docket/devinstall_cli_test.go` (2), `cmd/docket/gate_cli_test.go` (1 bare site plus the `gateTempDir` deletion and its 4 call sites), `cmd/releasepkg/main_test.go` (1). Each gains the unaliased `internal/testsupport` import.

No new files.

---

### Task 1: Widen the guard to `cmd/` and convert the 12 bare `t.TempDir()` sites

**Files:**
- Modify: `internal/repoguard/tempdir_fixture_test.go` (file header SCOPE paragraph, the `scanRoot` declaration, the body of `TestRealProcessPackagesUseFixtureTempDir`)
- Modify: `cmd/docket/main_test.go` (`TestInjectedBuildIdentity`, `TestCrossCompileApprovedTargets`, `TestInstallCheckJSONGolden`, `TestInstallCommandsRegistered`)
- Modify: `cmd/docket/config_cli_test.go` (`hermeticEnv`, `sparseRepo`, the fixture-copy helper that builds `dst := filepath.Join(t.TempDir(), "repo")`, `TestConfigNonexistentRepoDir`)
- Modify: `cmd/docket/devinstall_cli_test.go` (`TestDevelopmentInstallFreshRenderHandoff`, `buildWitnesslessStub`)
- Modify: `cmd/docket/gate_cli_test.go` (`gateDriveConfiguredRepo`'s `XDG_CONFIG_HOME` dir only; `gateTempDir` is Task 2)
- Modify: `cmd/releasepkg/main_test.go` (`TestHappyRunPackagesBundle`)

**Interfaces:**
- Consumes: `testsupport.TempDir(t testing.TB) string` from `internal/testsupport` (existing).
- Produces: `var scanRoots []string` and `var realProcFloors []string` in package `repoguard` (test-only). The guard's failure lines keep their existing formats: `<abs file>: bare <recv>.TempDir() — use testsupport.TempDir(t)` and, for a floor, `derivation lost <floor> — real-process set: [...]`.

- [ ] **Step 1: Widen the guard (this is the failing test)**

In `internal/repoguard/tempdir_fixture_test.go`, replace the `SCOPE` paragraph of the file header comment (the paragraph that begins `// SCOPE (change 0373, explicit — see scanRoot below)` and ends `// constant, not a silent one.`) with:

```go
// SCOPE (explicit — see scanRoots and realProcFloors below): this guard
// enforces the fixture rule for the real-process test packages under the
// module's two Go test roots, `internal/` (change 0373) and `cmd/` (change
// 0398). The walk roots are a visible property of the test via the named
// scanRoots list, not an accident of a buried literal, so any later change
// of coverage is a deliberate edit to that list. Each root carries a
// population floor in realProcFloors, kept as a SEPARATE list so that
// dropping a root from the walk reddens its floor instead of silently
// taking the floor with it.
```

Replace the `scanRoot` doc comment and declaration (`// scanRoot names change 0373's deliberate coverage boundary: …` through `const scanRoot = "internal"`) with:

```go
// scanRoots names the guard's deliberate coverage boundary: the module
// subtrees walked for real-process test packages (see the SCOPE note in the
// file header). Widening or narrowing coverage is an explicit edit to this
// list, never an accident of a buried walk-root literal.
var scanRoots = []string{"internal", "cmd"}

// realProcFloors are module-relative (slash-separated) packages the
// derivation must always find — one per scan root. An empty or rotted
// derivation, or a root silently dropped from scanRoots, then fails loudly
// instead of passing vacuously (marker-scoped guards need a population
// floor). Deliberately separate from scanRoots: see the SCOPE note.
var realProcFloors = []string{"internal/process", "cmd/docket"}
```

In `TestRealProcessPackagesUseFixtureTempDir`, replace the single `filepath.WalkDir(filepath.Join(root, scanRoot), …)` call and its `if err != nil { t.Fatal(err) }` with a loop over the roots:

```go
	pkgs := map[string][]string{}
	for _, sr := range scanRoots {
		err = filepath.WalkDir(filepath.Join(root, sr), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, "_test.go") {
				return err
			}
			dir := filepath.Dir(p)
			pkgs[dir] = append(pkgs[dir], p)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	fixtureDir := filepath.Join(root, "internal", "testsupport")
```

(This replaces the old `pkgs := map…` line, the old walk, and the old `fixtureDir := filepath.Join(root, scanRoot, "testsupport")` line. The rest of the derivation loop and `sort.Strings(realProc)` stay as they are.)

Replace the population-floor block (the comment starting `// Population floor (marker-scoped guards need one)` and the `if !slices.Contains(realProc, filepath.Join(root, scanRoot, "process"))` check) with:

```go
	// Population floors (marker-scoped guards need one): the derivation must
	// find each floor package — internal/process, whose supervisor tests
	// motivated the fixture, and cmd/docket, whose built-binary gate tests
	// spawn the real supervisor. A missing floor means the grep shape rotted
	// or a root was dropped from scanRoots, and the guard would otherwise pass
	// vacuously over that root.
	for _, floor := range realProcFloors {
		if !slices.Contains(realProc, filepath.Join(root, filepath.FromSlash(floor))) {
			t.Fatalf("derivation lost %s — real-process set: %v", floor, realProc)
		}
	}
```

After the edit, `scanRoot` must have no remaining reference:

Run: `grep -nw 'scanRoot' /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd/internal/repoguard/tempdir_fixture_test.go`
Expected: no output. `-w` matches whole words only, so `scanRoots` does not match.

- [ ] **Step 2: Run the guard and confirm it reddens on exactly the 12 `cmd/` sites**

First derive the site count from the tree. Do not trust the spec's number:

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && grep -rn '[A-Za-z_]\.TempDir(' cmd/`
Expected: 12 lines, all `t.TempDir()`, in the 5 files listed above. `gateTempDir(` does not match because it has no `.` before `TempDir`.

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && go test -count=1 -run '^TestRealProcessPackagesUseFixtureTempDir$' ./internal/repoguard/`
Expected: FAIL with `real-process packages must use the testsupport fixture:` followed by 12 `…: bare t.TempDir() — use testsupport.TempDir(t)` lines. The violations name `cmd/docket/main_test.go` (4), `cmd/docket/config_cli_test.go` (4), `cmd/docket/devinstall_cli_test.go` (2), `cmd/docket/gate_cli_test.go` (1), and `cmd/releasepkg/main_test.go` (1), with no `internal/` file. If a `derivation lost` line appears instead, the multi-root walk is wrong. Fix that before continuing.

- [ ] **Step 3: Convert the 12 sites to the fixture**

In each of the five files, add the unaliased import. For `cmd/docket/main_test.go`, add it to the existing module import group:

```go
	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/testsupport"
```

For `cmd/releasepkg/main_test.go`, add it to the existing module import group:

```go
	"github.com/danielhanold/docket/internal/release"
	"github.com/danielhanold/docket/internal/testsupport"
```

For `cmd/docket/config_cli_test.go`, `cmd/docket/devinstall_cli_test.go`, and `cmd/docket/gate_cli_test.go` (stdlib-only imports today), add a second import group after the stdlib group:

```go
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)
```

(For `gate_cli_test.go`, keep `"time"` in the stdlib group. The group then ends `"testing"` / `"time"` before the blank line.)

Then rewrite each call, keeping the receiver `t` (every site's receiver is the enclosing `t *testing.T`):

| File | Function | Before | After |
|---|---|---|---|
| `cmd/docket/main_test.go` | `TestInjectedBuildIdentity` | `filepath.Join(t.TempDir(), "docket-injected")` | `filepath.Join(testsupport.TempDir(t), "docket-injected")` |
| `cmd/docket/main_test.go` | `TestCrossCompileApprovedTargets` | `dir := t.TempDir()` | `dir := testsupport.TempDir(t)` |
| `cmd/docket/main_test.go` | `TestInstallCheckJSONGolden` | `home := t.TempDir()` | `home := testsupport.TempDir(t)` |
| `cmd/docket/main_test.go` | `TestInstallCommandsRegistered` | `home := t.TempDir()` | `home := testsupport.TempDir(t)` |
| `cmd/docket/config_cli_test.go` | `hermeticEnv` | `base := t.TempDir()` | `base := testsupport.TempDir(t)` |
| `cmd/docket/config_cli_test.go` | `sparseRepo` | `filepath.Join(t.TempDir(), "repo")` | `filepath.Join(testsupport.TempDir(t), "repo")` |
| `cmd/docket/config_cli_test.go` | fixture-copy helper (`dst := …`) | `dst := filepath.Join(t.TempDir(), "repo")` | `dst := filepath.Join(testsupport.TempDir(t), "repo")` |
| `cmd/docket/config_cli_test.go` | `TestConfigNonexistentRepoDir` | `filepath.Join(t.TempDir(), "no-such-repo")` | `filepath.Join(testsupport.TempDir(t), "no-such-repo")` |
| `cmd/docket/devinstall_cli_test.go` | `TestDevelopmentInstallFreshRenderHandoff` | `home := t.TempDir()` | `home := testsupport.TempDir(t)` |
| `cmd/docket/devinstall_cli_test.go` | `buildWitnesslessStub` | `dir := t.TempDir()` | `dir := testsupport.TempDir(t)` |
| `cmd/docket/gate_cli_test.go` | `gateDriveConfiguredRepo` | `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` | `t.Setenv("XDG_CONFIG_HOME", testsupport.TempDir(t))` |
| `cmd/releasepkg/main_test.go` | `TestHappyRunPackagesBundle` | `outDir := t.TempDir()` | `outDir := testsupport.TempDir(t)` |

Then run `gofmt -l cmd/ internal/repoguard/` from the worktree root. Expected: no output.

- [ ] **Step 4: Run the guard and confirm it passes**

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && grep -rn '[A-Za-z_]\.TempDir(' cmd/ | grep -v 'testsupport\.TempDir('; go test -count=1 -run '^TestRealProcessPackagesUseFixtureTempDir$' ./internal/repoguard/`
Expected: the grep prints nothing, and the test prints `ok  github.com/danielhanold/docket/internal/repoguard` (not `(cached)`).

- [ ] **Step 5: Run the converted packages and the whole repoguard package**

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && go vet ./cmd/... ./internal/repoguard/ && go test -count=1 ./cmd/... ./internal/repoguard/`
Expected: vet clean, and every package `ok` with no `(cached)`. No `testsupport: fixture dir not removable` error appears. That error would mean a converted test leaves unremovable content (Review Focus 4). If it appears, report it as a finding and do not revert the site.

- [ ] **Step 6: Mutation-test the widened guard (restore every mutation from a backup)**

Run each probe from the worktree root with `-count=1`. Restore after each one before starting the next.

M1: a regressed `cmd/docket` site reddens and names its file.

```bash
cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd
f=cmd/docket/config_cli_test.go
cp "$f" "$f.bak"
perl -0pi -e 's/dir := filepath\.Join\(testsupport\.TempDir\(t\), "repo"\)/dir := filepath.Join(t.TempDir(), "repo")/' "$f"
grep -n 'dir := filepath.Join(t.TempDir(), "repo")' "$f"   # must print one line: the mutation landed
go test -count=1 -run '^TestRealProcessPackagesUseFixtureTempDir$' ./internal/repoguard/
mv -f "$f.bak" "$f"
```
Expected: the grep prints one line. The test FAILs with exactly one violation, `…/cmd/docket/config_cli_test.go: bare t.TempDir() — use testsupport.TempDir(t)`.

M2: positive control showing `cmd/releasepkg` is derived.

```bash
f=cmd/releasepkg/main_test.go
cp "$f" "$f.bak"
perl -0pi -e 's/outDir := testsupport\.TempDir\(t\)/outDir := t.TempDir()/' "$f"
grep -n 'outDir := t.TempDir()' "$f"   # must print one line
go test -count=1 -run '^TestRealProcessPackagesUseFixtureTempDir$' ./internal/repoguard/
mv -f "$f.bak" "$f"
```
Expected: FAIL with exactly one violation, naming `…/cmd/releasepkg/main_test.go`. (The `testsupport` import becomes unused in the mutated file, but the guard only reads source bytes. It does not compile `cmd/releasepkg`, so the probe is valid.)

M3: dropping `cmd` from the walk reddens the `cmd/docket` floor.

```bash
f=internal/repoguard/tempdir_fixture_test.go
cp "$f" "$f.bak"
perl -0pi -e 's/var scanRoots = \[\]string\{"internal", "cmd"\}/var scanRoots = []string{"internal"}/' "$f"
grep -n 'var scanRoots = \[\]string{"internal"}' "$f"   # must print one line
go test -count=1 -run '^TestRealProcessPackagesUseFixtureTempDir$' ./internal/repoguard/
mv -f "$f.bak" "$f"
```
Expected: FAIL with `derivation lost cmd/docket — real-process set: [...]`, and the set lists only `internal/` packages.

M4: dropping both the `cmd` root and the `cmd/docket` floor (a deliberate two-list edit) records the guard's residual. It is expected to pass, and it confirms why the floors live in a separate list.

```bash
f=internal/repoguard/tempdir_fixture_test.go
cp "$f" "$f.bak"
perl -0pi -e 's/var scanRoots = \[\]string\{"internal", "cmd"\}/var scanRoots = []string{"internal"}/; s/var realProcFloors = \[\]string\{"internal\/process", "cmd\/docket"\}/var realProcFloors = []string{"internal\/process"}/' "$f"
go test -count=1 -run '^TestRealProcessPackagesUseFixtureTempDir$' ./internal/repoguard/
mv -f "$f.bak" "$f"
```
Expected: PASS. This is the documented residual: removing a root and its floor together is a deliberate two-list edit that the SCOPE comment makes visible, and M3 shows that removing either list entry alone reddens. Record the M4 PASS in the build evidence as expected, not as a defect.

After all four probes, confirm the tree is back to the Step 5 state:

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && ls cmd/docket/*.bak cmd/releasepkg/*.bak internal/repoguard/*.bak 2>/dev/null; go test -count=1 -run '^TestRealProcessPackagesUseFixtureTempDir$' ./internal/repoguard/`
Expected: no `.bak` files listed, and `ok`.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd
git add internal/repoguard/tempdir_fixture_test.go cmd/docket/main_test.go cmd/docket/config_cli_test.go cmd/docket/devinstall_cli_test.go cmd/docket/gate_cli_test.go cmd/releasepkg/main_test.go
git commit -m "test(repoguard): extend the testsupport temp-dir guard to cmd/ real-process packages (change 0398)"
```

---

### Task 2: Delete the private `gateTempDir` helper in `cmd/docket` in favor of the shared fixture

**Files:**
- Modify: `cmd/docket/gate_cli_test.go` (delete `gateTempDir` and its doc comment; `gateDriveConfiguredRepo`, `TestGateDriveEndToEndThroughBuiltBinary`, `TestGateEndToEndThroughBuiltBinary`)

**Interfaces:**
- Consumes: `testsupport.TempDir(t testing.TB) string` (import already added in Task 1).
- Produces: nothing new. `gateTempDir` no longer exists in package `main` under `cmd/docket`.

- [ ] **Step 1: Delete the helper and repoint its call sites**

Delete the whole `gateTempDir` declaration from `cmd/docket/gate_cli_test.go`: its doc comment (from `// gateTempDir is a temp dir whose cleanup tolerates the external supervisor's` through `// fails "directory not empty". The retry loop lets the supervisor finish.`) and the `func gateTempDir(t *testing.T) string { … }` body.

Replace its four call sites:

| Function | Before | After |
|---|---|---|
| `gateDriveConfiguredRepo` | `root := gateTempDir(t)` | `root := testsupport.TempDir(t)` |
| `TestGateDriveEndToEndThroughBuiltBinary` | `root := gateTempDir(t)` | `root := testsupport.TempDir(t)` |
| `TestGateEndToEndThroughBuiltBinary` | `root := gateTempDir(t)` | `root := testsupport.TempDir(t)` |
| `TestGateEndToEndThroughBuiltBinary` | `cwd := gateTempDir(t)` | `cwd := testsupport.TempDir(t)` |

Keep the `"os"` and `"time"` imports. Other code in the file still uses `os.MkdirAll`/`os.WriteFile` and `time.Sleep`. `go vet` in Step 2 confirms this. If either becomes unused, remove only that import.

Do not add a replacement comment that restates the supervisor exit-window rationale. It lives in the fixture's own doc comment on `cleanupTolerance` and `DrainOnCleanup`, and a copy would only drift. Do not touch `internal/cli/gate_test.go`'s separate `gateTempDir` (out of scope; see Global Constraints).

- [ ] **Step 2: Verify the helper is gone and the gate tests pass under repetition**

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && grep -rn 'gateTempDir' cmd/; gofmt -l cmd/docket/ && go vet ./cmd/docket/`
Expected: the grep prints nothing, gofmt prints nothing, and vet is clean.

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && go test -count=3 -run '^(TestGateDriveEndToEndThroughBuiltBinary|TestGateEndToEndThroughBuiltBinary)$' ./cmd/docket/`
Expected: `ok  github.com/danielhanold/docket/cmd/docket` with no `testsupport: fixture dir not removable` error in any of the three repetitions (Review Focus 5). The fixture's 4s tolerant removal replaces the old 2s retry. If a removal failure does appear, register a `testsupport.DrainOnCleanup(t, …)` that waits for the supervisor, as the spec directs. Do not reintroduce a private retry helper.

- [ ] **Step 3: Re-run the guard and the whole package**

Run: `cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd && go test -count=1 ./cmd/docket/ ./internal/repoguard/`
Expected: both `ok`, neither `(cached)`.

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd
git add cmd/docket/gate_cli_test.go
git commit -m "test(cmd/docket): replace private gateTempDir with the testsupport fixture (change 0398)"
```

---

## Build gate (owned by docket-build's end gate)

After both tasks, the whole suite runs through `build.test_command` (`go run ./cmd/docket development test`), entered from the feature worktree. Beyond the pass/fail line, read the budget report. Any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, or `SERIAL CONFIRMED OVER BUDGET:` line naming a shard that runs `cmd/docket`, `cmd/releasepkg`, or `internal/repoguard` is a finding for the build evidence, even on a green run. The fixture's removal adds cost only on the failure path, so no budget movement is expected.
