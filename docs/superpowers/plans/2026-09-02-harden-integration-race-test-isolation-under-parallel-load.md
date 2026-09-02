<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0373 — Harden integration/race test isolation under parallel load](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0373-harden-integration-race-test-isolation-under-parallel-load.md)**
<!-- docket:backlink:end -->
# Harden Integration/Race Test Isolation Under Parallel Load — Implementation Plan

> **For agentic workers:** This plan is executed by the docket build role (`docket-build`): one
> profile worker per task under the docket-build-task contract, one full-suite gate
> (`go run ./cmd/docket development test`) at the end. Steps use checkbox (`- [ ]`) syntax for
> tracking. Each task is independently committable with its own focused verification.

**Goal:** Stop unrelated tests from reddening under the full-suite gate's parallel load, by (a) bounding total Go test concurrency at the runner and (b) replacing bare `t.TempDir()` with a draining fixture in every package whose tests spawn real git or supervisor processes — with a fail-closed guard so the fix stays structural.

**Architecture:** Two mechanisms, per the spec's root-cause analysis. Section-1 tasks export one `DOCKET_`-namespaced concurrency cap from `internal/suiterunner`'s sandbox into every target and translate it into `go test -p` / `GOMAXPROCS` in the Go wrappers. Section-2 tasks add a new test-only package `internal/testsupport` (temp dir with drain-then-retry cleanup, git background work off, per-fixture process-registry roots) and adopt it across the twelve real-process packages. A `repoguard` test then bans bare `t.TempDir()` in those packages, with the package list derived by grep at test time. Two constants — the load multiplier and the cleanup tolerance window — are **measured during the build** (Task 9), never guessed.

**Tech Stack:** Go (module toolchain per `go.mod`), bash suite wrappers under `tests/`, the Go-native suite runner `internal/suiterunner`.

**Spec:** `docs/superpowers/specs/2026-09-02-harden-integration-race-test-isolation-under-parallel-load-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` from the primary tree).

## Global Constraints

- Test-infrastructure hardening ONLY: no product behavior change; no test's assertions change — fixture adoption is a substitution, not a rewrite.
- No wrapper receives a `serial` pin in `tests/runtime-budgets.tsv`.
- Every measured value (load multiplier, cleanup tolerance) is measured on this machine during the build, recorded next to its constant with hardware/contention context (learning *tolerance-constant-calibrated-on-one-machine*). No number in this plan that is marked PROVISIONAL may survive to the final commit unmeasured.
- Re-seeded budget rows follow the table's own rule (next multiple of 5 above the measured serial seconds, +5s margin, min 10s); no row over 60s; no ceiling raised without a written reason in the commit message.
- The twelve-package list in this plan is a snapshot; every task that consumes it MUST re-derive it by grep (command given in Task 7) — never hand-copy (repo rule: never hand-list gated sites).
- All `go test` verification runs use `-count=1` (learning *cached-runner-serves-a-mutated-tree*).
- Comment cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Every guard added here is mutation-tested before its task's commit: strip the guarded thing, watch it redden, restore.
- The suite gate is `go run ./cmd/docket development test`, run from the feature worktree root.

---

### Task 1: `internal/testsupport` — draining temp-dir fixture

**Files:**
- Create: `internal/testsupport/testsupport.go`
- Create: `internal/testsupport/testsupport_test.go`

**Interfaces:**
- Consumes: nothing (leaf package; stdlib only).
- Produces (later tasks rely on these exact names):
  - `func TempDir(t testing.TB) string` — drop-in replacement for `t.TempDir()`.
  - `func DrainOnCleanup(t testing.TB, fn func())` — registers a drain to run before this test's fixture dirs are removed.
  - `func WaitQuiesced(deadline time.Duration, step time.Duration, probe func() bool) bool` — generic bounded quiesce poll (the generalization of `quiesceRun`'s loop).
  - `const cleanupTolerance` — the bounded RemoveAll retry window (unexported; PROVISIONAL until Task 9).

- [ ] **Step 1: Write the failing tests**

Create `internal/testsupport/testsupport_test.go`:

```go
package testsupport

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A dir with a transient post-test writer must still be removed: simulate a
// trailing writer by recreating a file once from a cleanup registered AFTER
// TempDir (LIFO: it runs before the removal cleanup? No — registered after,
// so it runs FIRST; the file it leaves is the "trailing write" removal must
// absorb via retry).
func TestTempDirRemovalAbsorbsTrailingWrite(t *testing.T) {
	var dir string
	t.Run("inner", func(t *testing.T) {
		dir = TempDir(t)
		// Registered after TempDir => runs before the removal cleanup,
		// planting a file the first RemoveAll pass may race with.
		t.Cleanup(func() {
			if err := os.WriteFile(filepath.Join(dir, "straggler"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("fixture dir survived cleanup: %v", err)
	}
}

func TestDrainRunsBeforeRemoval(t *testing.T) {
	var order []string
	var dir string
	t.Run("inner", func(t *testing.T) {
		dir = TempDir(t)
		DrainOnCleanup(t, func() { order = append(order, "drain") })
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("fixture dir survived cleanup")
	}
	if len(order) != 1 || order[0] != "drain" {
		t.Fatalf("drain did not run before removal: %v", order)
	}
}

func TestWaitQuiescedBounded(t *testing.T) {
	start := time.Now()
	if WaitQuiesced(50*time.Millisecond, time.Millisecond, func() bool { return false }) {
		t.Fatal("probe never true, but WaitQuiesced reported quiesced")
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("returned before deadline")
	}
	n := 0
	if !WaitQuiesced(time.Second, time.Millisecond, func() bool { n++; return n >= 3 }) {
		t.Fatal("probe became true, but WaitQuiesced reported timeout")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd <worktree> && go test -count=1 ./internal/testsupport/`
Expected: FAIL (package does not compile — `TempDir` undefined).

- [ ] **Step 3: Implement**

Create `internal/testsupport/testsupport.go`:

```go
// Package testsupport is the shared real-process test fixture (change 0373).
// It is imported ONLY from _test.go files. It replaces bare t.TempDir() in
// packages whose tests spawn real git or supervisor processes: t.TempDir's
// cleanup is one os.RemoveAll with no retry, so a detached writer that
// outlives the last assertion produces "directory not empty" under parallel
// load. This fixture drains registered writers first, then retries removal
// over a bounded tolerance window, and on final failure fails the test
// naming the surviving paths — a genuine leak surfaces as a finding, not an
// opaque teardown crash.
package testsupport

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// cleanupTolerance bounds the RemoveAll retry window.
// PROVISIONAL: 5s pending Task 9's measurement under full-gate load; the
// final value must carry its measurement (machine, -j level, longest
// observed drain) in this comment. Too tight flakes; too loose only costs
// wall clock on a genuine leak, which fails anyway.
const cleanupTolerance = 5 * time.Second

const cleanupStep = 10 * time.Millisecond

var (
	drainMu sync.Mutex
	drains  = map[testing.TB][]func(){}
)

// DrainOnCleanup registers fn to run before this test's fixture dirs are
// removed. Drains run once, in registration order, from the first removal
// cleanup that fires (t.Cleanup is LIFO and TempDir is called before the
// spawn being drained, so removal cleanups run after any t.Cleanup the test
// registered later — drains still run first because removal invokes them).
func DrainOnCleanup(t testing.TB, fn func()) {
	t.Helper()
	drainMu.Lock()
	defer drainMu.Unlock()
	drains[t] = append(drains[t], fn)
}

func runDrains(t testing.TB) {
	drainMu.Lock()
	fns := drains[t]
	delete(drains, t)
	drainMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// TempDir is the fixture's drop-in for t.TempDir(). The directory name
// carries the test name so a surviving dir on the failure path is
// attributable (diagnostic naming).
func TempDir(t testing.TB) string {
	t.Helper()
	pattern := "docketfix-" + sanitize(t.Name()) + "-*"
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatalf("testsupport: MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		runDrains(t)
		removeAllTolerant(t, dir)
	})
	return dir
}

func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		}
		return '_'
	}, name)
}

// removeAllTolerant retries os.RemoveAll over cleanupTolerance, absorbing
// trailing writes from a draining child. On final failure it fails the test
// and names the surviving paths.
func removeAllTolerant(t testing.TB, dir string) {
	deadline := time.Now().Add(cleanupTolerance)
	var lastErr error
	for {
		lastErr = os.RemoveAll(dir)
		if lastErr == nil {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(cleanupStep)
	}
	var survivors []string
	_ = filepath.WalkDir(dir, func(p string, _ fs.DirEntry, err error) error {
		if err == nil && p != dir {
			survivors = append(survivors, p)
		}
		return nil
	})
	t.Errorf("testsupport: fixture dir not removable after %v: %v; surviving paths: %v",
		cleanupTolerance, lastErr, survivors)
}

// WaitQuiesced polls probe every step until it reports true or deadline
// elapses; returns whether the probe became true. This is the generalized
// loop of internal/process's quiesceRun ("a free live.lock proves the run
// dir is quiescent"): callers supply the domain-specific probe.
func WaitQuiesced(deadline time.Duration, step time.Duration, probe func() bool) bool {
	end := time.Now().Add(deadline)
	for {
		if probe() {
			return true
		}
		if time.Now().After(end) {
			return false
		}
		time.Sleep(step)
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test -count=1 ./internal/testsupport/` — Expected: PASS.
Run: `gofmt -l internal/testsupport/ && go vet ./internal/testsupport/` — Expected: no output, no findings.

- [ ] **Step 5: Verify the failure diagnostic (manual mutation)**

Temporarily hold a file open past tolerance is machine-fiddly; instead verify the survivor-naming branch directly: in a scratch `_test.go` (not committed), call `removeAllTolerant` on a dir containing a file made undeletable (`chmod 0` on the parent, restore after). Confirm the error names the surviving path. Delete the scratch file. If sandboxing blocks chmod games, verifying via a temporary `cleanupTolerance = 0` mutation plus a directory seeded with an open-and-recreating writer is acceptable — either way, observe the "surviving paths" message once before committing.

- [ ] **Step 6: Commit**

```bash
git add internal/testsupport/
git commit -m "test(0373): add internal/testsupport draining temp-dir fixture"
```

---

### Task 2: Git background work off — one source, both consumers

**Files:**
- Modify: `internal/suiterunner/sandbox.go` (the `gitIdentityConfig` const and its doc)
- Modify: `internal/suiterunner/sandbox_test.go` if present, else the suiterunner test file covering `Sandbox` (locate via `grep -rn "Sandbox(" internal/suiterunner/*_test.go`)
- Modify: `internal/testsupport/testsupport.go` (add `GitEnv`)
- Modify: `internal/testsupport/testsupport_test.go`

**Interfaces:**
- Produces: `suiterunner.GitBackgroundOff` (exported const, the config snippet); `testsupport.GitEnv(t testing.TB) []string` returning `{"GIT_CONFIG_GLOBAL=<per-fixture path>"}`.
- Consumed by: Task 3–6 adoptions (any test helper that builds env for a spawned git); the runner sandbox (every gate target).

Single-source rationale: the spec requires the same four knobs per fixture AND in the runner sandbox. Duplicating the list invites divergence (learning *duplicated-gate-copies-the-whole-predicate*); `internal/testsupport` (test-only) importing `internal/suiterunner` (product) creates no cycle, so the snippet lives once in `sandbox.go`.

- [ ] **Step 1: Write failing tests**

In the suiterunner test file covering `Sandbox`, add:

```go
func TestSandboxGitConfigDisablesBackgroundWork(t *testing.T) {
	jobdir := t.TempDir() // suiterunner is itself adopted in Task 6; leave for now
	env, err := Sandbox(jobdir)
	if err != nil {
		t.Fatal(err)
	}
	var global string
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_CONFIG_GLOBAL=") {
			global = strings.TrimPrefix(kv, "GIT_CONFIG_GLOBAL=")
		}
	}
	b, err := os.ReadFile(global)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"autoDetach = false", "auto = 0", "[maintenance]", "fsmonitor = false"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("sandbox global git config missing %q:\n%s", want, b)
		}
	}
}
```

In `internal/testsupport/testsupport_test.go`, add:

```go
func TestGitEnvPointsAtBackgroundOffConfig(t *testing.T) {
	env := GitEnv(t)
	if len(env) != 1 || !strings.HasPrefix(env[0], "GIT_CONFIG_GLOBAL=") {
		t.Fatalf("GitEnv = %v", env)
	}
	b, err := os.ReadFile(strings.TrimPrefix(env[0], "GIT_CONFIG_GLOBAL="))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), suiterunner.GitBackgroundOff) {
		t.Fatalf("fixture git config does not embed suiterunner.GitBackgroundOff:\n%s", b)
	}
	if !strings.Contains(string(b), "name = docket test") {
		t.Fatalf("fixture git config lost the synthetic identity:\n%s", b)
	}
}
```

- [ ] **Step 2: Run to verify failures**

`go test -count=1 ./internal/suiterunner/ ./internal/testsupport/` — Expected: FAIL (missing knobs / `GitEnv`, `GitBackgroundOff` undefined).

- [ ] **Step 3: Implement**

In `sandbox.go`, split the snippet out and extend `gitIdentityConfig`:

```go
// GitBackgroundOff disables every git mechanism that detaches a child which
// can outlive the invoking command and keep writing into the repository —
// the t.TempDir() "directory not empty" mechanism (change 0373). One source:
// the runner sandbox appends it to the synthetic global config, and
// internal/testsupport embeds it in each fixture's GIT_CONFIG_GLOBAL, so
// gate runs and solo runs agree.
const GitBackgroundOff = "[gc]\n\tauto = 0\n\tautoDetach = false\n[maintenance]\n\tauto = false\n[core]\n\tfsmonitor = false\n"

const gitIdentityConfig = "[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n" + GitBackgroundOff
```

Note the existing `gitIdentityConfig` doc comment says "Byte-for-byte the oracle's launch() writes this" — update that sentence (it is no longer byte-for-byte; say the identity core is the oracle's, plus change 0373's background-off knobs). Check for any existing test asserting the old exact bytes of `gitIdentityConfig` (`grep -rn "gitIdentityConfig\|defaultBranch" internal/suiterunner/*_test.go`) and update it to the new expectation — do not weaken it to a substring-only check if it was byte-exact.

In `testsupport.go` add (import `docket/internal/suiterunner` — confirm the module path prefix from `go.mod` first):

```go
// GitEnv returns the env override pointing spawned git processes at a
// per-fixture global config: the synthetic identity plus
// suiterunner.GitBackgroundOff. Pass it (appended last) to any exec'd
// command that may run git.
func GitEnv(t testing.TB) []string {
	t.Helper()
	dir := TempDir(t)
	cfg := "[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n" + suiterunner.GitBackgroundOff
	path := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatalf("testsupport: write git config: %v", err)
	}
	return []string{"GIT_CONFIG_GLOBAL=" + path}
}
```

- [ ] **Step 4: Run to verify pass**

`go test -count=1 ./internal/suiterunner/ ./internal/testsupport/` — Expected: PASS.

- [ ] **Step 5: Mutation-test**

Delete `+ GitBackgroundOff` from `gitIdentityConfig`; run the suiterunner test; Expected: RED. Restore. Delete `suiterunner.GitBackgroundOff` from `GitEnv`'s cfg; run the testsupport test; Expected: RED. Restore. Re-run both green.

- [ ] **Step 6: Commit**

```bash
git add internal/suiterunner/ internal/testsupport/
git commit -m "test(0373): disable git background work in sandbox and fixture git config"
```

---

### Task 3: Adopt the fixture in `internal/process` (generalize quiesceRun; per-fixture registry roots)

**Files:**
- Modify: `internal/process/launch_test.go` (the `quiesceRun` function and every `t.TempDir()` site)
- Modify: every other `internal/process/*_test.go` with bare `t.TempDir()` (derive: `grep -rn "\.TempDir()" internal/process/*_test.go`)

**Interfaces:**
- Consumes: `testsupport.TempDir`, `testsupport.DrainOnCleanup`, `testsupport.WaitQuiesced`, `testsupport.GitEnv` (only where git is spawned).
- Produces: nothing new; behavior-preserving substitution.

- [ ] **Step 1: Baseline**

Run: `go test -count=1 ./internal/process/` — Expected: PASS (record the wall clock).

- [ ] **Step 2: Rewrite `quiesceRun` over the fixture primitive**

Keep `quiesceRun`'s domain logic (flock probe, terminal/failure-record reads, group SIGKILL for a still-live idle child) in `internal/process` — it uses unexported symbols and cannot move. Replace only its hand-rolled poll loop with `testsupport.WaitQuiesced`, preserving the existing semantics exactly (same 30s deadline, same 10ms step, same kill-once condition):

```go
func quiesceRun(t *testing.T, runDir string) {
	t.Helper()
	lockPath := filepath.Join(runDir, liveLockFile)
	killed := false
	testsupport.WaitQuiesced(30*time.Second, 10*time.Millisecond, func() bool {
		held, ans := probeFlock(lockPath)
		if !held && ans == probeAbsent {
			return true // supervisor gone; no further writes will land
		}
		if held && !killed {
			term, _ := readTerminal(runDir)
			fr, _ := readFailureRecord(runDir)
			if term == nil && fr == nil {
				if m, err := readManifest(runDir); err == nil && m != nil && m.PGID > 1 {
					_ = signalGroup(m.PGID, syscall.SIGKILL)
					killed = true
				}
			}
		}
		return false
	})
}
```

Keep the full existing doc comment on `quiesceRun` (it explains WHY a free lock proves quiescence); adjust only the last line to note the loop is `testsupport.WaitQuiesced`.

- [ ] **Step 3: Route quiesce through the drain hook**

In `launchHelper`, replace `t.Cleanup(func() { quiesceRun(t, out.RunDir) })` with `testsupport.DrainOnCleanup(t, func() { quiesceRun(t, out.RunDir) })`, and likewise at the other two `t.Cleanup(func() { quiesceRun(...) })` sites (`grep -n "quiesceRun" internal/process/*_test.go`). Update the comment above the `launchHelper` site: the ordering guarantee now comes from the fixture's drain-before-removal contract, not LIFO reasoning.

- [ ] **Step 4: Substitute temp dirs and isolate registry/scan roots**

Replace every bare `t.TempDir()` in `internal/process/*_test.go` with `testsupport.TempDir(t)`. This is a mechanical substitution — do NOT change any assertion. Then close sighting 5 (shared-`$TMPDIR` pollution of the recover-scan): grep the package's tests for any root derived from `os.TempDir()` or `os.Getenv("TMPDIR")` (`grep -rn "os.TempDir\|TMPDIR" internal/process/*_test.go`) and re-root each under `testsupport.TempDir(t)` so a concurrent product supervisor under the real `$TMPDIR` can never appear in a test's scan set. If the grep finds no such site, the pollution entered through a root that defaults inside product code when the test passes "" — find the defaulting path (start from the `Recover`/scan entry point the 0396 results file names) and make the test pass an explicit fixture root instead. Record which of the two shapes it was in the commit message.

- [ ] **Step 5: Verify**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.
Run: `go test -race -count=1 ./internal/process/` — Expected: PASS (stub 381's folded-in sighting lives here: `TestObserveRunningThenTerminal` under `-race`).
Run: `grep -c "t.TempDir()" internal/process/*_test.go` — Expected: 0 matches (exit 1).

- [ ] **Step 6: Commit**

```bash
git add internal/process/
git commit -m "test(0373): adopt testsupport fixture in internal/process; isolate registry roots"
```

---

### Task 4: Adopt the fixture in the git-heavy packages (`gitcli`, `repository`, `repository/transaction`)

**Files:**
- Modify: `internal/gitcli/*_test.go`, `internal/repository/*_test.go`, `internal/repository/transaction/*_test.go` (every bare `t.TempDir()` site; derive with `grep -rln "\.TempDir()" <pkg>/*_test.go`)

**Interfaces:**
- Consumes: `testsupport.TempDir`, `testsupport.GitEnv`.

- [ ] **Step 1: Baseline**

`go test -count=1 ./internal/gitcli/ ./internal/repository/ ./internal/repository/transaction/` — Expected: PASS. Also run the tagged integration corpus these packages carry: `go test -tags integration -count=1 ./internal/gitcli/` — Expected: PASS.

- [ ] **Step 2: Substitute**

Replace every bare `t.TempDir()` (and any `b.TempDir()`) with `testsupport.TempDir(t)` across the three packages. Substitution only — no assertion changes.

- [ ] **Step 3: Wire `GitEnv` at the git-spawn seams**

Find each package's env-construction seam for spawned git (`grep -rn "exec.Command\|Env =\|Environ()" <pkg>/*_test.go` plus the non-test helpers tests call). Where a test (or a test helper) builds the environment for a real `git` child, append `testsupport.GitEnv(t)...` last so the per-fixture config wins. Where tests run git through product code that inherits the test process env, set the variable process-wide for that test via `t.Setenv("GIT_CONFIG_GLOBAL", …)` using the path from `GitEnv` (split the single `KEY=VALUE` entry). Do not force `GitEnv` into tests that spawn no git. Sighting 3 (`internal/gitcli` teardown) and sighting 4 (`internal/repository/transaction` `TestKeyedCommitCarriesFiveTrailers/keyed`) are the regression cases this task closes.

- [ ] **Step 4: Verify**

`go test -count=1 ./internal/gitcli/ ./internal/repository/ ./internal/repository/transaction/` and `go test -tags integration -count=1 ./internal/gitcli/` — Expected: PASS.
`grep -rc "t.TempDir()" internal/gitcli/*_test.go internal/repository/*_test.go internal/repository/transaction/*_test.go` — Expected: every count 0.
Load probe for the two sightings: `go test -count=5 -run 'TestKeyedCommitCarriesFiveTrailers' ./internal/repository/transaction/` concurrently with `go test -tags integration -count=2 ./internal/gitcli/` (two background shells) — Expected: both PASS. A green probe is evidence about the probe, not proof of absence (learning *groomed-root-cause-is-a-hypothesis*, its non-reproduction corollary) — record the result either way for the results file.

- [ ] **Step 5: Commit**

```bash
git add internal/gitcli/ internal/repository/
git commit -m "test(0373): adopt testsupport fixture in gitcli, repository, repository/transaction"
```

---

### Task 5: Adopt the fixture in `internal/app`

**Files:**
- Modify: `internal/app/*_test.go` (largest population; includes the `integration`- and `e2e`-tagged corpus)

**Interfaces:**
- Consumes: `testsupport.TempDir`, `testsupport.GitEnv` (same seam rules as Task 4).

- [ ] **Step 1: Baseline**

`go test -count=1 ./internal/app/`, `go test -tags integration -count=1 ./internal/app/`, and `go test -tags e2e -run TestE2E -count=1 ./internal/app/` — Expected: PASS (the e2e matrix is slow; its wrapper budget is 25s solo — record wall clocks).

- [ ] **Step 2: Substitute and wire**

Same mechanical substitution as Task 4 (`t.TempDir()`/`b.TempDir()` → `testsupport.TempDir(t)`), same `GitEnv` seam rule at env-construction sites for spawned git. `internal/app` tests commonly build env slices for a compiled `docket` binary or git children — append `testsupport.GitEnv(t)...` last at those seams.

- [ ] **Step 3: Verify**

Re-run all three Step-1 commands with `-count=1` — Expected: PASS, wall clocks within the same order as baseline (this package's timeout was sighting 2; a large regression here is a finding, not noise).
`grep -c "t.TempDir()" internal/app/*_test.go` — Expected: 0.

- [ ] **Step 4: Commit**

```bash
git add internal/app/
git commit -m "test(0373): adopt testsupport fixture in internal/app"
```

---

### Task 6: Adopt the fixture in the remaining real-process packages

**Files:**
- Modify: `*_test.go` in the remaining derived packages — snapshot list: `internal/cli`, `internal/gatedrive`, `internal/workspace`, `internal/install`, `internal/suiterunner`, `internal/repoguard`, `internal/release`. RE-DERIVE first:

```bash
grep -rlE 'exec\.Command' internal --include='*_test.go' | xargs -n1 dirname | sort -u
```

Every package that command prints and Tasks 3–5 did not cover belongs to this task (minus `internal/testsupport` itself, if its own tests spawn a writer).

**Interfaces:**
- Consumes: `testsupport.TempDir`, `testsupport.GitEnv`, `testsupport.DrainOnCleanup` (where a package's tests launch detached children — check `internal/gatedrive` for supervisor spawns and register a drain mirroring Task 3's pattern if so).

- [ ] **Step 1: Baseline** — `go test -count=1 ./internal/cli/ ./internal/gatedrive/ ./internal/workspace/ ./internal/install/ ./internal/suiterunner/ ./internal/repoguard/ ./internal/release/` (adjust to the re-derived list) — Expected: PASS.

- [ ] **Step 2: Substitute and wire** — same rules as Task 4. Note for `internal/suiterunner`: Task 2's `TestSandboxGitConfigDisablesBackgroundWork` deliberately left a `t.TempDir()`; convert it here too. `internal/testsupport` is excluded — it is the fixture, and its own tests may use bare `t.TempDir()` where they must observe un-fixtured behavior.

- [ ] **Step 3: Verify** — re-run Step 1 (`-count=1`): PASS. `grep -rc "t.TempDir()"` over the task's packages: all 0.

- [ ] **Step 4: Commit**

```bash
git add internal/
git commit -m "test(0373): adopt testsupport fixture in remaining real-process packages"
```

---

### Task 7: Fail-closed repoguard — no bare `t.TempDir()` in real-process packages

**Files:**
- Create: `internal/repoguard/tempdir_fixture_test.go`

**Interfaces:**
- Consumes: the repository source tree (repoguard tests read files; follow the repo-root discovery pattern of the existing `internal/repoguard/*_test.go` files — reuse their helper if one exists).

- [ ] **Step 1: Write the guard (failing is proven by mutation, not by TDD order — the tree is already clean after Tasks 3–6)**

```go
package repoguard

// Change 0373's fail-closed fixture guard. A real-process package is one
// whose _test.go files spawn subprocesses (shape: exec.Command — derived
// here at test time, never hand-listed). In those packages every temp dir
// must come from internal/testsupport, whose cleanup drains detached
// writers and retries removal; a bare <ident>.TempDir() call reintroduces
// the "directory not empty" teardown race this change closes.
//
// LIMITATION (asserted, per the byte-pattern-guard learning): this guard
// matches the receiver-call shape `<ident>.TempDir(`. It cannot see a call
// through an interface value or a helper that shadows the name; the
// aliased-import check below closes the one cheap evasion (import
// testsupport under another name and the receiver test goes vacuous).

var execCallRe = regexp.MustCompile(`\bexec\.Command`)
var tempDirCallRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.TempDir\(`)
var testsupportAliasRe = regexp.MustCompile(`(?m)^\s*(?:([A-Za-z_][A-Za-z0-9_]*)\s+)?"[^"]*internal/testsupport"`)

func TestRealProcessPackagesUseFixtureTempDir(t *testing.T) {
	root := repoRoot(t) // the package's existing repo-root helper
	pkgs := map[string][]string{}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d fs.DirEntry, err error) error {
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
	fixtureDir := filepath.Join(root, "internal", "testsupport")
	var realProc []string
	for dir, files := range pkgs {
		if dir == fixtureDir {
			continue // the fixture itself is exempt by construction
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if execCallRe.Match(b) {
				realProc = append(realProc, dir)
				break
			}
		}
	}
	sort.Strings(realProc)
	// Population floor (marker-scoped guards need one): the derivation must
	// find the package whose supervisor tests motivated the fixture. An
	// empty or process-less derivation means the grep shape rotted, and the
	// guard would pass vacuously.
	if !slices.Contains(realProc, filepath.Join(root, "internal", "process")) {
		t.Fatalf("derivation lost internal/process — real-process set: %v", realProc)
	}
	var violations []string
	for _, dir := range realProc {
		for _, f := range pkgs[dir] {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range testsupportAliasRe.FindAllStringSubmatch(string(b), -1) {
				if m[1] != "" && m[1] != "testsupport" && m[1] != "_" {
					violations = append(violations, fmt.Sprintf("%s: testsupport imported under alias %q", f, m[1]))
				}
			}
			for _, m := range tempDirCallRe.FindAllStringSubmatch(string(b), -1) {
				if m[1] != "testsupport" {
					violations = append(violations, fmt.Sprintf("%s: bare %s.TempDir() — use testsupport.TempDir(t)", f, m[1]))
				}
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("real-process packages must use the testsupport fixture:\n%s", strings.Join(violations, "\n"))
	}
}
```

Adjust imports/`repoRoot` to the package's existing house pattern (read a neighboring `internal/repoguard/*_test.go` first and reuse its root discovery verbatim).

- [ ] **Step 2: Run green**

`go test -count=1 -run TestRealProcessPackagesUseFixtureTempDir ./internal/repoguard/` — Expected: PASS.

- [ ] **Step 3: Mutation-test (required; the guard is decoration until it reddens)**

1. In `internal/process/launch_test.go`, change one `testsupport.TempDir(t)` back to `t.TempDir()` (uncommitted edit). Run the guard with `-count=1`. Expected: FAIL naming the file. Restore via `git checkout -- internal/process/launch_test.go` (safe: Task 3 is committed).
2. Alias mutation: change `internal/process`'s testsupport import to `tsx "….../internal/testsupport"` (and one call site). Run the guard. Expected: FAIL on the alias. Restore.
3. Population mutation: temporarily change `execCallRe` to something matching nothing. Expected: FAIL on the population floor. Restore.
Re-run green after restores.

- [ ] **Step 4: Commit**

```bash
git add internal/repoguard/tempdir_fixture_test.go
git commit -m "test(0373): fail-closed guard — real-process packages must use the testsupport fixture"
```

---

### Task 8: Bound total Go test load at the runner

**Files:**
- Modify: `internal/suiterunner/sandbox.go` (derivation const + function + env export)
- Modify: `internal/suiterunner/execute.go` (`ExecuteTarget` threads the cap), `internal/suiterunner/schedule.go` (passes it), `internal/suiterunner/run.go` (computes it from `cfg.Jobs` and `runtime.NumCPU()`)
- Modify: `tests/lib/go-integration-shard.sh`, `tests/test_go_toolchain.sh`, `tests/test_go_race.sh`, `tests/test_go_finalize_e2e.sh`
- Modify: the suiterunner test file covering `Sandbox`/`ExecuteTarget`

**Interfaces:**
- Produces: env var `DOCKET_GO_TEST_CONCURRENCY=<n>` in every target's sandbox; `func GoTestConcurrency(jobs, cpus int) int` and consts `goLoadMultNum`, `goLoadMultDen` in `sandbox.go`.
- Contract: absent variable ⇒ Go defaults apply unchanged (solo `bash tests/test_X.sh`, bare `go test`).

- [ ] **Step 1: Failing Go test**

```go
func TestGoTestConcurrencyDerivation(t *testing.T) {
	// cap = clamp(mult*cpus/jobs, 1, cpus): the whole gate then runs at most
	// jobs*cap ≈ mult*cpus concurrent Go test packages.
	if got := GoTestConcurrency(11, 11); got < 1 || got > 11 {
		t.Fatalf("cap out of range: %d", got)
	}
	if got := GoTestConcurrency(1, 8); got > 8 {
		t.Fatalf("solo-jobs cap must not exceed cpus: %d", got)
	}
	if got := GoTestConcurrency(1000, 8); got != 1 {
		t.Fatalf("floor is 1: %d", got)
	}
}

func TestSandboxExportsGoTestConcurrency(t *testing.T) {
	env, err := Sandbox(testsupport.TempDir(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, "DOCKET_GO_TEST_CONCURRENCY=3") {
		t.Fatal("sandbox did not export the cap")
	}
	env, err = Sandbox(testsupport.TempDir(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "DOCKET_GO_TEST_CONCURRENCY=") {
			t.Fatal("cap 0 must omit the variable (Go defaults apply)")
		}
	}
}
```

Run: `go test -count=1 ./internal/suiterunner/` — Expected: FAIL (signature/function missing).

- [ ] **Step 2: Implement runner side**

In `sandbox.go`:

```go
// goLoadMultNum/goLoadMultDen: the gate-wide Go package concurrency stays at
// or under (mult × NumCPU) instead of (-j × NumCPU).
// PROVISIONAL 2/1 — Task 9 sweeps candidates on the reference machine,
// records per-candidate wall clock, and pins the smallest multiplier that
// keeps every Go budget row under its ceiling; the final value must carry
// that measurement here (machine, -j, per-candidate table pointer).
const (
	goLoadMultNum = 2
	goLoadMultDen = 1
)

// GoTestConcurrency derives the per-target cap the sandbox exports as
// DOCKET_GO_TEST_CONCURRENCY: with -j targets in flight, a per-target cap of
// mult*cpus/jobs bounds the product at mult*cpus concurrent Go test
// packages. Floor 1, ceiling cpus.
func GoTestConcurrency(jobs, cpus int) int {
	if jobs < 1 {
		jobs = 1
	}
	n := goLoadMultNum * cpus / (goLoadMultDen * jobs)
	if n < 1 {
		n = 1
	}
	if n > cpus {
		n = cpus
	}
	return n
}
```

Extend `Sandbox(jobdir string, goTestConcurrency int)`: when the value is >= 1, append `"DOCKET_GO_TEST_CONCURRENCY=" + strconv.Itoa(goTestConcurrency)` to the overrides; when 0, omit. Thread the value: `run.go` computes `conc := GoTestConcurrency(cfg.Jobs, runtime.NumCPU())` (it already has `cpus := runtime.NumCPU()` in the budget-classification block — compute the cap where the schedule config is built, before targets launch); pass it through the schedule config into `ExecuteTarget` and on to `Sandbox`. VERIFY the plumbing before editing (learning *check-plumbing-auto-discovery*): `ExecuteTarget(ctx, cfg.Bash, t, cfg.Work, reg, cfg.ExtraEnv)` is called from `schedule.go` ("mirroring the former Bash oracle's `-j` bound" file); extend the schedule `Config` struct with `GoTestConcurrency int` and `ExecuteTarget` with a trailing parameter — fix every caller the compiler names, including tests (pass 0 where a test does not care).

- [ ] **Step 3: Wrapper translation**

In `tests/lib/go-integration-shard.sh`, inside `run_integration_shard` before the `-list` probe:

```bash
  # Change 0373: under the suite runner, DOCKET_GO_TEST_CONCURRENCY bounds
  # this child's share of the machine (go test package parallelism and
  # runtime procs). Absent (solo run), Go's defaults apply unchanged.
  go_conc_args=""
  if [ -n "${DOCKET_GO_TEST_CONCURRENCY:-}" ]; then
    go_conc_args="-p ${DOCKET_GO_TEST_CONCURRENCY}"
    export GOMAXPROCS="${DOCKET_GO_TEST_CONCURRENCY}"
  fi
```

and add `$go_conc_args` (unquoted, like `$race_flag`) to BOTH `go test` invocations (the `-list` probe and the executing run). Do not touch `shard_inspect_maybe` — the contract test parses its output. Apply the same block to the three whole-module wrappers, adding `$go_conc_args` to: `test_go_toolchain.sh`'s `go test ./...` (Check 3), `test_go_race.sh`'s `go test -race -count=1 ./...`, and `test_go_finalize_e2e.sh`'s `go test -tags e2e …` — place the block after each file's GOFLAGS/cache setup, and keep each `assert` description string unchanged unless it interpolates the new args (it must not; assert descriptions are stable markers).

- [ ] **Step 4: Verify**

`go test -count=1 ./internal/suiterunner/` — Expected: PASS.
Solo-absence check: `bash tests/test_go_integration_gitcli_setuptree.sh` with the variable unset — Expected: exit 0, and `DOCKET_GO_TEST_CONCURRENCY` never referenced (Go defaults). Then `DOCKET_GO_TEST_CONCURRENCY=2 bash tests/test_go_integration_gitcli_setuptree.sh` — Expected: exit 0.
Shard contract intact: `bash tests/test_go_integration_contract.sh` — Expected: exit 0.
Mutation: in `run_integration_shard`, drop `$go_conc_args` from the executing `go test` only; there is no cheap red assert for this (the run still passes), so instead verify wiring positively: `DOCKET_GO_TEST_CONCURRENCY=1 bash tests/test_go_integration_gitcli_setuptree.sh` and confirm via the restored line that `-p 1` reaches the command (e.g. temporarily `set -x` around it and read the trace). This is a wiring check, not a guard; the enforcement that matters is Task 9's measured gate.

- [ ] **Step 5: Commit**

```bash
git add internal/suiterunner/ tests/lib/go-integration-shard.sh tests/test_go_toolchain.sh tests/test_go_race.sh tests/test_go_finalize_e2e.sh
git commit -m "feat(0373): runner-exported DOCKET_GO_TEST_CONCURRENCY cap, honored by Go wrappers"
```

---

### Task 9: Measure the two constants (multiplier sweep; cleanup tolerance)

**Files:**
- Modify: `internal/suiterunner/sandbox.go` (`goLoadMultNum`/`goLoadMultDen` + measurement comment)
- Modify: `internal/testsupport/testsupport.go` (`cleanupTolerance` + measurement comment)

This task is measurement, not guesswork (learning *optimization-needs-a-measured-oracle*: the oracle is wall clock, so the acceptance is the recorded table). Machine context to record with every number: `sysctl -n hw.ncpu`, `uname -ms`, and the gate's `-j` (the default `development test` parallelism).

- [ ] **Step 1: Sweep the multiplier**

For each candidate M in {1/1, 3/2, 2/1, 3/1}: set `goLoadMultNum/goLoadMultDen`, rebuild nothing else, run `go run ./cmd/docket development test` once, and record (a) total suite wall clock, (b) every Go-wrapper row's measured seconds vs its ceiling from the run's report, (c) any `BUDGET WATCH` / `PARALLEL-SENSITIVE` / `SERIAL CONFIRMED OVER BUDGET` lines. Between candidates nothing else may change (same head, no other load on the machine you can avoid). Keep the raw table in a scratch file — it goes into the results file at gate time (spec section 4).

- [ ] **Step 2: Pick and pin**

Pick the SMALLEST M whose run keeps every Go budget row under its ceiling (a `SERIAL CONFIRMED OVER BUDGET` disqualifies; screening lines are findings to note, not disqualifiers — but prefer the candidate with none). If no candidate keeps every row under, pick the best and carry the over rows into Task 10's re-seed with the written reason. Replace the PROVISIONAL comment with the measurement: machine, cpus, -j, the winning M, and the per-candidate wall clocks (or a one-line summary plus "full table in the change 0373 results file").

- [ ] **Step 3: Calibrate `cleanupTolerance`**

During the winning candidate's gate run (or one more run), measure the longest observed drain-to-removable interval: temporarily instrument `removeAllTolerant` to log retry counts (uncommitted edit), run the gate, and take the max observed interval across all jobs; set `cleanupTolerance` to a comfortable multiple (at least 4x the observed max, min 2s) — the loose direction here only costs wall clock on a genuine leak, which fails anyway (same asymmetry as the 0325 barrier ceiling). Remove the instrumentation. Record the measurement in the constant's comment: observed max, machine, -j, chosen multiple.

- [ ] **Step 4: Verify and commit**

`go test -count=1 ./internal/suiterunner/ ./internal/testsupport/` — Expected: PASS (the derivation test bounds, not pins, M — it must still pass).

```bash
git add internal/suiterunner/sandbox.go internal/testsupport/testsupport.go
git commit -m "test(0373): pin measured load multiplier and cleanup tolerance"
```

(The commit message body must carry the per-candidate wall-clock table and the observed drain max — the durable home is the results file, but the constants' provenance travels with the commit that set them.)

---

### Task 10: Re-seed affected budget rows

**Files:**
- Modify: `tests/runtime-budgets.tsv`

- [ ] **Step 1: Identify affected rows**

Affected = every Go wrapper whose wall clock the cap or the fixture changed: the three whole-module wrappers (`tests/test_go_toolchain.sh`, `tests/test_go_race.sh`, `tests/test_go_finalize_e2e.sh`) and any `tests/test_go_integration_*.sh` row whose Task-9 measurements moved it out of its seeded band (compare the winning candidate's per-row seconds against current ceilings).

- [ ] **Step 2: Measure serially and re-seed**

For each affected row, take its SOLO measurement (run the file alone: `bash tests/<file>.sh` timed, or read the runner's solo confirmation figures from Task 9's runs — solo, because the table's WHY block says rows are "Seeded from a measured serial run"). Apply the table's rule verbatim: round up to the next multiple of 5, add 5s margin, min 10s. Constraints: no row over 60s (if a measurement forces one over, STOP and surface it — that is a sharding problem out of scope, changes 280/296); any raised ceiling gets its reason in the commit message; keep `parallel` mode on every row (Global Constraint: no `serial` pins).

- [ ] **Step 3: Verify**

`go run ./cmd/docket development test` — Expected: green, with the correspondence guard (`internal/repoguard` budgets tests) passing and no `SERIAL CONFIRMED OVER BUDGET`.

- [ ] **Step 4: Commit**

```bash
git add tests/runtime-budgets.tsv
git commit -m "test(0373): re-seed Go wrapper budget rows from post-fix measurements"
```

---

### Task 11: Evidence — stability runs and regression cases

**Files:** none (evidence collection; the build role writes the results file at its gate — this task produces the material).

- [ ] **Step 1: Regression cases, serially and under the gate**

Run each named sighting serially with `-count=1` and confirm green:
1. `go test -race -count=1 -run 'TestObserveRunningThenTerminal' ./internal/process/` (sightings 1 + folded stub 381)
2. `go test -count=1 ./internal/app/` (sighting 2 — the package that blew the per-package timeout)
3. `go test -tags integration -count=1 ./internal/gitcli/` (sighting 3)
4. `go test -count=1 -run 'TestKeyedCommitCarriesFiveTrailers' ./internal/repository/transaction/` (sighting 4)
5. Sighting 5 (shared-`$TMPDIR` recover-scan pollution): re-run the `internal/process` recover tests (`go test -count=1 -run 'Recover' ./internal/process/`) while a decoy run dir sits under the real `$TMPDIR` (create one shaped like a supervisor run dir by hand from the fixtures Task 3 touched); Expected: the tests never see it — PASS.

- [ ] **Step 2: Five consecutive green full gates**

At one head (no commits between runs), run `go run ./cmd/docket development test` five times consecutively. Expected: five greens, zero unrelated reds. A single red on ANY run voids the streak: diagnose (learning *systematic-debugging* discipline — serial-confirm, then root-cause; do not simply re-run to green), fix, commit, and restart the five-count at the new head. Record for the results file: the head SHA, five wall clocks, and any budget-clause lines.

- [ ] **Step 3: Hand the evidence to the build role's gate**

Collect into one scratch summary for the results file: the five-run table, the Task-9 per-candidate multiplier table and winning M, the cleanup-tolerance measurement, the before/after budget rows, and the five regression-case outcomes (including any honest non-reproduction notes from Task 4 Step 4). No commit from this task unless Step 2 forced a fix.

---

## Self-Review (performed)

- **Spec coverage:** §1 runner cap + wrapper translation + measured multiplier + re-seeded rows → Tasks 8, 9, 10. §2 fixture (drain primitive from `quiesceRun`, bounded retry over measured tolerance, survivor diagnostics, git background off per fixture AND in `gitIdentityConfig`, per-fixture registry root, twelve-package adoption by re-derived grep) → Tasks 1–6. §3 fail-closed mutation-tested repoguard, no serial pins → Task 7 + Global Constraints. §4 evidence → Tasks 9–11. Stub 381 → nothing planned (already killed at reconcile); its test is Task 11 regression case 1.
- **Plan-time choices the spec delegated:** fixture package name `internal/testsupport`; env var `DOCKET_GO_TEST_CONCURRENCY`; derivation `clamp(M×cpus/jobs, 1, cpus)` with M as a ratio constant in `sandbox.go`.
- **Type consistency:** `testsupport.TempDir/DrainOnCleanup/WaitQuiesced/GitEnv` and `suiterunner.GitBackgroundOff/GoTestConcurrency` are spelled identically at definition (Tasks 1, 2, 8) and every consumption site (Tasks 3–8).
- **Placeholders:** the only unfixed values are the two constants, which are explicitly PROVISIONAL with Task 9 as their required resolution — that is the spec's instruction, not a gap.
