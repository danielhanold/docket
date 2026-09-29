<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0470 — Install the no-real-git test guard in internal/gatedrive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-29-0470-install-the-no-real-git-test-guard-in-internal-gatedrive.md)**
<!-- docket:backlink:end -->
# Install the No-Real-Git Test Guard in internal/gatedrive Implementation Plan

> **For agentic workers:** Execution is via the `docket-build` role (task-by-task through its profile agents under the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Install the shared no-real-git guard (`testsupport.InstallNoGitGuard`) in `internal/gatedrive`'s default-build test binary, prove it with the three standard proving tests, and retire the prose that calls gatedrive "deliberately unguarded".

**Architecture:** Reapply change 0466's per-package wiring exactly: a `main_test.go` whose `TestMain` installs the guard around `m.Run`, plus a `nogit_guard_test.go` holding the three proving tests that call the shared `testsupport` asserts. The one difference from `internal/workspace` and `internal/repository/transaction`: gatedrive already has a `TestMain` in the integration-tagged `supervisor_integration_test.go`, so the new `main_test.go` must carry `//go:build !integration && !e2e` to avoid a duplicate `TestMain` under `-tags integration`. No production code changes; the guard helper itself is untouched.

**Tech Stack:** Go test binaries (`internal/gatedrive`, `internal/testsupport`); Bash suite shard headers (`tests/*.sh`, comments only).

**Spec:** none — the change is `trivial: true`. Scope source: the change record `docs/changes/active/0470-install-the-no-real-git-test-guard-in-internal-gatedrive.md` on the `docket` metadata branch (readable from the repo root at `.docket/docs/changes/active/0470-install-the-no-real-git-test-guard-in-internal-gatedrive.md`), including its 2026-09-29 Reconcile log.

## Global Constraints

- Mirror `internal/workspace/main_test.go` and `internal/workspace/nogit_guard_test.go` (change 0466). The helper returns an error and the package's `TestMain` owns `os.Exit`; library code never exits (`TestProcessExitSitesAreAllowlisted` in `cmd/docket/exit_sites_test.go`).
- Guard identity for this package: `nogitPkg = "internal/gatedrive"`, `nogitShardGlob = "tests/test_go_integration_gatedrive_*.sh"` (the existing shards are `tests/test_go_integration_gatedrive_process.sh` and `tests/test_go_integration_gatedrive_race.sh`).
- `internal/gatedrive/main_test.go` MUST open with `//go:build !integration && !e2e` on line 1 and a blank line 2 (so Go honors the constraint). `internal/gatedrive/supervisor_integration_test.go` keeps its `//go:build integration` `TestMain` unchanged in code.
- `internal/gatedrive/nogit_guard_test.go` also carries `//go:build !integration && !e2e` (the proving tests would fail under the tagged builds, where the installer is the identity no-op in `internal/testsupport/nogit_install_off.go`).
- Neither new file is named `*_integration_test.go`, and no new test uses a `TestIntegration…` / `TestRaceIntegration…` prefix (`tests/test_go_integration_contract.sh` rules).
- Do not change `internal/testsupport`'s behavior, any other package's guard, or `tests/runtime-budgets.tsv` (out of scope per the change record). Only comments in `internal/testsupport/nogit.go` change.
- Do not edit point-in-time records: `docs/results/…`, archived changes, and prior plans keep their "gatedrive has no no-real-git guard" wording.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (AGENTS.md, ADR-0054).
- Every focused `go test` run passes `-count=1` (learning `cached-runner-serves-a-mutated-tree`): a cached `ok` certifies an older tree, which makes a mutation probe vacuous.
- Mutation probes restore from an explicit backup copy and prove the restore with `cmp`, never from `git checkout --` against uncommitted work (learning `mutation-restore-needs-a-backup-copy`).
- The build gate runs the whole suite through the resolved `build.test_command` (`go run ./cmd/docket development test`), never only the tests named here.

## Review Focus

1. **Duplicate `TestMain` under the integration tag** — if `main_test.go` loses its build constraint (or line 2 is not blank), `go vet -tags integration ./internal/gatedrive/` fails with `TestMain redeclared`. Pinned by Task 1 Step 6 (vet under `integration` and `e2e`).
2. **A default-corpus gatedrive test that already runs git through production code** (a straggler the reconcile grep could not see, e.g. via `gitcli`) — the guard turns the whole package red naming the offender. Expected behavior: move that test behind `//go:build integration` into a `*_integration_test.go` file with a `TestIntegrationGatedrive…` name, never weaken the guard. Pinned by Task 1 Step 5 (full package run under the guard) with the contingency in Step 5.
3. **A guard that is wired but vacuous** — proving tests that stay green with the install removed. Pinned by Task 1 Step 7 (mutation: remove the install, all three proving tests must redden).
4. **A tolerated git exec in an ordinary (non-proving) test** — a test that swallows the git error must still fail the package with the gatedrive diagnostic. Pinned by `TestNoGitGuardFailsTolerantTest` and by the scratch straggler probe in Task 1 Step 8.
5. **Stale "deliberately unguarded" prose surviving in maintained source** — readers of `test_go_race.sh`, the process shard header, `nogit.go`, or the supervisor `TestMain` comment would be told gatedrive has no guard. Pinned by Task 2 Step 3's whole-repo grep.

---

## File Structure

- Create `internal/gatedrive/main_test.go` — the default-build `TestMain` installing the guard (one responsibility: guard installation).
- Create `internal/gatedrive/nogit_guard_test.go` — the three proving tests.
- Modify `internal/gatedrive/supervisor_integration_test.go` — comment above `TestMain` only (the "The default gatedrive build has no TestMain" paragraph becomes false).
- Modify `internal/testsupport/nogit.go` — package header comment only (the "internal/gatedrive is deliberately unguarded" sentence).
- Modify `tests/test_go_race.sh` — header comment only (the "the spec left it without a git guard" sentence).
- Modify `tests/test_go_integration_gatedrive_process.sh` — header comment only (the "internal/gatedrive has no no-real-git guard" sentence).

---

### Task 1: Install the guard in internal/gatedrive and prove it

**Files:**
- Create: `internal/gatedrive/nogit_guard_test.go`
- Create: `internal/gatedrive/main_test.go`

**Interfaces:**
- Consumes (existing, `github.com/danielhanold/docket/internal/testsupport`):
  - `func InstallNoGitGuard(pkg, shardGlob string) (func(code int) int, error)`
  - `func AssertNoGitGuardShadowsGit(t testing.TB)`
  - `func AssertNoGitGuardRefusesBareExec(t testing.TB, pkg string)`
  - `func NoGitGuardTolerantProbe(t *testing.T, pkg, testName string)` — `testName` must be the calling test's exact name (it re-execs the binary with `-test.run=^<testName>$`).
- Produces: package-level test constants `nogitPkg`, `nogitShardGlob` in package `gatedrive` (default build only); tests `TestNoGitGuardShadowsGitOnPath`, `TestNoGitGuardRefusesBareExec`, `TestNoGitGuardFailsTolerantTest`.

- [ ] **Step 1: Write the failing proving tests**

Create `internal/gatedrive/nogit_guard_test.go` with exactly:

```go
//go:build !integration && !e2e

package gatedrive

// The no-real-git guard's proving tests for internal/gatedrive (change 0470). The
// guard lives in internal/testsupport (InstallNoGitGuard) and is installed from
// TestMain in main_test.go. These tests prove it is installed in THIS package's
// binary and fails the package on any real-git exec, even one a test tolerates.
// Real-git and real-process tests live behind //go:build integration in the
// tests/test_go_integration_gatedrive_*.sh shards.

import (
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestNoGitGuardShadowsGitOnPath: every PATH lookup of `git` resolves the shim.
func TestNoGitGuardShadowsGitOnPath(t *testing.T) {
	testsupport.AssertNoGitGuardShadowsGit(t)
}

// TestNoGitGuardRefusesBareExec: a bare git exec gets the guard's exit code and diagnostic.
func TestNoGitGuardRefusesBareExec(t *testing.T) {
	testsupport.AssertNoGitGuardRefusesBareExec(t, nogitPkg)
}

// TestNoGitGuardFailsTolerantTest: a test that swallows the git failure still fails the package.
func TestNoGitGuardFailsTolerantTest(t *testing.T) {
	testsupport.NoGitGuardTolerantProbe(t, nogitPkg, "TestNoGitGuardFailsTolerantTest")
}
```

- [ ] **Step 2: Run the proving tests to verify they fail**

Run (from the feature worktree root):

```bash
go test -count=1 -run '^TestNoGitGuard' ./internal/gatedrive/
```

Expected: build FAIL with `undefined: nogitPkg` (the constant arrives with `main_test.go` in Step 3). That is the red state for a not-yet-installed guard.

- [ ] **Step 3: Write the guard installation**

Create `internal/gatedrive/main_test.go` with exactly (line 1 is the build constraint, line 2 is blank):

```go
//go:build !integration && !e2e

package gatedrive

import (
	"fmt"
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (change 0470, reapplying change 0466's wiring): the default-tag
// internal/gatedrive test corpus never starts a real git; real-git and
// real-process tests live behind //go:build integration in the
// tests/test_go_integration_gatedrive_*.sh shards.
const (
	nogitPkg       = "internal/gatedrive"
	nogitShardGlob = "tests/test_go_integration_gatedrive_*.sh"
)

// TestMain installs the no-real-git guard (testsupport.InstallNoGitGuard) around
// m.Run in the default build. Unlike internal/workspace and
// internal/repository/transaction, this file carries a build constraint: the
// integration build already has a TestMain (supervisor_integration_test.go, which
// routes the supervisor and child re-exec roles), and the tagged corpora exist to
// run real git, so exactly one TestMain compiles for any tag set.
func TestMain(m *testing.M) {
	finish, err := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
	if err != nil {
		// The library returns the setup failure; this TestMain ends the process.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(finish(m.Run()))
}
```

- [ ] **Step 4: Run the proving tests to verify they pass**

```bash
go test -count=1 -run '^TestNoGitGuard' -v ./internal/gatedrive/
```

Expected: `--- PASS` for all three `TestNoGitGuard…` tests, then `ok  github.com/danielhanold/docket/internal/gatedrive`.

- [ ] **Step 5: Run the whole default gatedrive corpus under the guard (plain and race)**

```bash
go test -count=1 ./internal/gatedrive/
go test -count=1 -race ./internal/gatedrive/
```

Expected: `ok` for both. The reconcile grep found no git / `exec.Command` use in gatedrive's default-build test files, so no straggler is expected.

Contingency, only if the run fails with the `internal/gatedrive` no-real-git diagnostic and a `N real-git exec attempt(s)` list: the listed argv/cwd names the offender. Move that test (and only the fixtures it alone uses) into an existing `internal/gatedrive/*_integration_test.go` file (line 1 `//go:build integration`, line 2 blank), renamed to `TestIntegrationGatedrive<OldSuffix>` so the process shard's `^TestIntegrationGatedrive` prefix runs it; re-run both commands above until `ok`, then run `bash tests/test_go_integration_contract.sh` and `bash tests/test_go_integration_gatedrive_process.sh` and expect no `NOT OK` lines. Never weaken or bypass the guard to make a straggler pass. Record any move in the commit message.

- [ ] **Step 6: Vet under every tag set (duplicate-TestMain check)**

```bash
go vet ./internal/gatedrive/
go vet -tags integration ./internal/gatedrive/
go vet -tags e2e ./internal/gatedrive/
gofmt -l internal/gatedrive/main_test.go internal/gatedrive/nogit_guard_test.go
```

Expected: all three `go vet` runs print nothing and exit 0 (no `TestMain redeclared`, no `undefined: nogitPkg`); `gofmt -l` prints nothing.

- [ ] **Step 7: Mutation — remove the install, watch all three proving tests redden**

Back up, mutate, run, restore, and prove the restore:

```bash
BK="$(mktemp "${TMPDIR:-/tmp}/gatedrive-main-test.XXXXXX")"
cp internal/gatedrive/main_test.go "$BK"
cat > internal/gatedrive/main_test.go <<'EOF'
//go:build !integration && !e2e

package gatedrive

import (
	"os"
	"testing"
)

const (
	nogitPkg       = "internal/gatedrive"
	nogitShardGlob = "tests/test_go_integration_gatedrive_*.sh"
)

func TestMain(m *testing.M) { os.Exit(m.Run()) }
EOF
out="$(go test -count=1 -run '^TestNoGitGuard' -v ./internal/gatedrive/ 2>&1)"; echo "exit=$?"
printf '%s\n' "$out" | grep -E -e '^--- (FAIL|PASS): TestNoGitGuard'
cp "$BK" internal/gatedrive/main_test.go && cmp "$BK" internal/gatedrive/main_test.go && rm -f "$BK"
```

Expected: `exit=1` and exactly three `--- FAIL:` lines (`TestNoGitGuardShadowsGitOnPath` with "the no-real-git guard is not installed"; `TestNoGitGuardRefusesBareExec` because real git's exit code is not the guard's; `TestNoGitGuardFailsTolerantTest` because the child exits 0). Any `--- PASS: TestNoGitGuard` line means a proving test is vacuous: stop and investigate, do not commit. After the restore, `cmp` is silent; re-run Step 4 and expect all three to pass again.

- [ ] **Step 8: Straggler probe — a new default-corpus test that tolerates a git exec fails the package**

```bash
cat > internal/gatedrive/zz_nogit_straggler_probe_test.go <<'EOF'
//go:build !integration && !e2e

package gatedrive

import (
	"os/exec"
	"testing"
)

func TestZZNoGitStragglerProbe(t *testing.T) {
	_ = exec.Command("git", "rev-parse", "HEAD").Run() // tolerated on purpose
}
EOF
out="$(go test -count=1 -run '^TestZZNoGitStragglerProbe$' ./internal/gatedrive/ 2>&1)"; echo "exit=$?"
printf '%s\n' "$out" | grep -F -e 'internal/gatedrive' -e 'real-git exec attempt(s)' -e 'rev-parse'
rm -f internal/gatedrive/zz_nogit_straggler_probe_test.go
git status --porcelain internal/gatedrive/
```

Expected: `exit=1`; the output carries the `internal/gatedrive` guard diagnostic, `1 real-git exec attempt(s)`, and the `rev-parse` argv, even though the test itself reported PASS. After `rm`, `git status --porcelain internal/gatedrive/` lists only `?? internal/gatedrive/main_test.go` and `?? internal/gatedrive/nogit_guard_test.go`.

- [ ] **Step 9: Commit**

```bash
git add internal/gatedrive/main_test.go internal/gatedrive/nogit_guard_test.go
git commit -m "test(gatedrive): install the no-real-git guard in the default corpus (change 0470)"
```

(Add any straggler test file moved under Step 5's contingency to the same `git add`, by exact path.)

---

### Task 2: Retire the "gatedrive is unguarded" prose in maintained source

**Files:**
- Modify: `internal/gatedrive/supervisor_integration_test.go` (comment above `func TestMain`)
- Modify: `internal/testsupport/nogit.go` (package header comment)
- Modify: `tests/test_go_race.sh` (header comment)
- Modify: `tests/test_go_integration_gatedrive_process.sh` (header comment)

**Interfaces:**
- Consumes: Task 1's `internal/gatedrive/main_test.go` (`TestMain`, build constraint `!integration && !e2e`).
- Produces: comments only; no code or behavior change.

- [ ] **Step 1: Confirm the stale sites (derived, not hand-listed)**

```bash
git grep -n -E -e 'has no no-real-git guard|deliberately unguarded|without a git guard|default gatedrive build has no TestMain' -- ':!docs/'
```

Expected: exactly four hits — `internal/gatedrive/supervisor_integration_test.go`, `internal/testsupport/nogit.go`, `tests/test_go_integration_gatedrive_process.sh`, `tests/test_go_race.sh`. If the grep finds any other maintained-source hit, fix it the same way in this task. `docs/` is excluded because results files, archived changes, and plans are point-in-time records.

- [ ] **Step 2: Rewrite the four comments**

In `internal/gatedrive/supervisor_integration_test.go`, replace:

```go
// Change 0466 moved this file (formerly integration_test.go) behind the integration
// tag. The default gatedrive build has no TestMain: none of its tests re-execs the
// test binary as a supervisor or child (only this tagged corpus drives the real
// process.Service), so Go's default m.Run is exactly right there.
```

with:

```go
// Change 0466 moved this file (formerly integration_test.go) behind the integration
// tag. The default gatedrive build has its own TestMain in main_test.go (change
// 0470), which installs the no-real-git guard and routes no re-exec roles: none of
// the default tests re-execs the test binary as a supervisor or child (only this
// tagged corpus drives the real process.Service). The two TestMains carry mutually
// exclusive build constraints, so at most one compiles for any tag set.
```

In `internal/testsupport/nogit.go`, replace:

```go
// starts a real `git`. Installed from the TestMain of internal/app,
// internal/repository/transaction, and internal/workspace. internal/gatedrive is
// deliberately unguarded: its moved corpus is mixed real-process and real-git, the
// spec left the guard out, and its budget row in tests/test_go_race.sh is the
// growth detector.
```

with:

```go
// starts a real `git`. Installed from the TestMain of internal/app,
// internal/repository/transaction, internal/workspace (change 0466), and
// internal/gatedrive (change 0470).
```

In `tests/test_go_race.sh`, replace:

```bash
# internal/gatedrive's real-supervisor and real-git tests behind the tag too.
# gatedrive's moved corpus is mixed real-process and real-git; the spec left it
# without a git guard, and this file's budget row is its growth detector. With
# that tail gone, `go test -race`'s GOMAXPROCS-wide race workers do not
```

with:

```bash
# internal/gatedrive's real-supervisor and real-git tests behind the tag too;
# change 0470 installed the guard in internal/gatedrive as well. With
# that tail gone, `go test -race`'s GOMAXPROCS-wide race workers do not
```

In `tests/test_go_integration_gatedrive_process.sh`, replace:

```bash
# build tag, prefix ^TestIntegrationGatedrive. internal/gatedrive has no no-real-git guard (its
# moved corpus is mixed real-process and real-git; the spec omitted the guard): the budget
# row of tests/test_go_race.sh is its growth detector. Declarations only — execution and
# inspection live in
```

with:

```bash
# build tag, prefix ^TestIntegrationGatedrive. The default internal/gatedrive corpus must never
# start real git (testsupport.InstallNoGitGuard, installed from the package's TestMain, change
# 0470). Declarations only — execution and inspection live in
```

(The line that follows, `# tests/lib/go-integration-shard.sh; the completeness contract is`, stays as-is.)

- [ ] **Step 3: Verify no stale site remains and nothing else moved**

```bash
git grep -n -E -e 'has no no-real-git guard|deliberately unguarded|without a git guard|default gatedrive build has no TestMain' -- ':!docs/'; echo "grep-exit=$?"
gofmt -l internal/gatedrive/supervisor_integration_test.go internal/testsupport/nogit.go
bash -n tests/test_go_race.sh && bash -n tests/test_go_integration_gatedrive_process.sh && echo syntax-ok
git diff --stat
```

Expected: no grep hits and `grep-exit=1`; `gofmt -l` prints nothing; `syntax-ok`; `git diff --stat` lists exactly the four files, each with comment-only hunks (read `git diff` to confirm no non-comment line changed).

- [ ] **Step 4: Run the affected suites**

```bash
go test -count=1 ./internal/testsupport/ ./internal/gatedrive/
go vet -tags integration ./internal/gatedrive/
bash tests/test_go_integration_contract.sh
```

Expected: `ok` for both packages; vet silent; the contract script prints only `ok -` lines. The first line of `tests/test_go_integration_gatedrive_process.sh` is still `#!/usr/bin/env bash` and line 2 still `# docket-suite: go` (the edit is below them).

- [ ] **Step 5: Commit**

```bash
git add internal/gatedrive/supervisor_integration_test.go internal/testsupport/nogit.go tests/test_go_race.sh tests/test_go_integration_gatedrive_process.sh
git commit -m "docs(gatedrive): retire the 'gatedrive is unguarded' comments (change 0470)"
```

---

## Build gate

After both tasks, the build role runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`) from the feature worktree. Read the budget report even when green: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line on `tests/test_go_race.sh` would mean the guard's shim setup costs measurable time; the change record keeps the budget rows unchanged unless re-measurement shows that, and a `SERIAL CONFIRMED OVER BUDGET:` line is the only reading that would justify touching `tests/runtime-budgets.tsv` (out of scope here — report it rather than retune).
