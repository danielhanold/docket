<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0465 — test_go_race times out on internal/app in CI (Go's 10m per-package limit)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0465-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa.md)**
<!-- docket:backlink:end -->
# Default internal/app Corpus Never Runs Real Git — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. (In this repository the build runs through `docket-build`: one `### Task N` heading is one worker and one commit.)

**Goal:** Make `tests/test_go_race.sh` reliably pass in CI without weakening the race gate. To do that, enforce change 0333's partition as an invariant: the default-tag `internal/app` test corpus never starts a real `git` process.

**Architecture:** A default-build-only `TestMain` hook, `installNoGitGuard`, puts a refusing `git` shim at the front of `PATH` and fails the package if any exec reached it. It is the authoritative census of offenders. Driven by that census, the 337 real-git, subprocess and process-lifecycle default tests move behind `//go:build integration`, get `TestIntegration…` names, and run in eleven new prefix-scoped plain shards. Genuinely concurrent ones join the existing `TestRaceIntegrationAppConcurrency` race shard. Around the partition: `tests/test_go_race.sh` gets an explicit `-timeout` backstop with a readable overrun message, the suite runner's budget-state key becomes repo-relative so overruns accumulate across worktrees, and every touched budget row is re-measured.

**Tech Stack:** Go 1.27 (`testing`, `os/exec`), bash test wrappers over `tests/lib/go-integration-shard.sh`, the Go suite runner (`internal/suiterunner`), `tests/runtime-budgets.tsv`.

**Spec:** `docs/superpowers/specs/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-design.md` (on the `docket` metadata branch; read-only copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-design.md`).

**Feature worktree:** `/Users/homer/dev/docket/.worktrees/test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa` (branch `fix/test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa`). Every command below runs from this directory unless it says otherwise.

## Global Constraints

- Invariant: **the default-tag `internal/app` test corpus never starts a real `git` process.** Real-git, subprocess and process-lifecycle scenarios live behind `//go:build integration`.
- Never weaken the race gate. Do not drop `-race`, narrow `./...`, or skip `internal/app` in `tests/test_go_race.sh`.
- The guard is keyed on runtime behavior (the exec itself), never on spellings such as `exec.Command("git"` or helper names.
- The guard is compiled only into the **default** build. `internal/app` has two non-default tags today, `integration` and `e2e` (`internal/app/finalize_e2e_test.go` is `//go:build e2e` and runs real git by design). The guard file is `//go:build !integration && !e2e` and its no-op twin is `//go:build integration || e2e`.
- Every moved test file is `internal/app/*_integration_test.go` with line 1 exactly `//go:build integration` and line 2 blank (contract check (1) in `tests/test_go_integration_contract.sh`).
- Renaming rule for a moved test: `<shard prefix>` + the old name with its leading `Test` removed. Example: `TestRunCancelHappyPath` becomes `TestIntegrationGateCancelRunCancelHappyPath`. A race-classified test uses the prefix `TestRaceIntegrationAppConcurrency` instead.
- Moved tests run in **plain (non-race)** shards. A test goes to the race shard only if it genuinely exercises concurrency: it starts goroutines (`go func`/`go f(`), coordinates simultaneous work with `sync.WaitGroup`, errgroup or channels, or holds two live processes/launches against shared state at once. It then gets a one-line rationale comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.`
- New shard prefixes must not be a string prefix of any other `internal/app` shard prefix, and no other prefix may be a prefix of them. The eleven new prefixes below were checked against the 27 existing ones at plan time.
- `tests/test_go_integration_contract.sh` stays green and is **not edited**.
- Every `tests/test_*.sh` needs exactly one row in `tests/runtime-budgets.tsv` (`internal/repoguard` `TestRuntimeBudgetsCorrespondence`). Row value = measured serial (solo) seconds, rounded up to the next multiple of 5, plus 5, minimum 10. Format: `<path><TAB><seconds><TAB>parallel`.
- Never hand-list sites: derive offenders from the guard's census and name references from a whole-repo `git grep -w`. Point-in-time records keep old names: never rewrite `docs/results/**`, `docs/changes/**`, `docs/superpowers/**` or `docs/adrs/**`.
- Every run that observes a verdict defeats Go's test cache (`-count=1`). Mutation probes back up the file and copy it back. Never `git checkout --` an uncommitted edit.
- Shell rules (AGENTS.md): never pipe a producer into `grep -q`/`head`, and capture into a variable first. Write grep patterns as `grep -E -e`. Template every `mktemp` as `"${TMPDIR:-/tmp}/<name>.XXXXXX"`. Use `mv -f`. The Bash tool's shell here is zsh, so run multi-line snippets with `bash -c '…'` or from a script file when they rely on bash word-splitting.
- Stage only the files your task names (`git add <paths>`, never `git add -A`).
- Cross-references in maintained source anchor on symbol names or quoted clauses, never on line numbers (ADR-0054).
- **Expected intermediate state:** after Task 1 the default `go test ./internal/app/` is **red** (337 offenders) and stays red until Task 13 lands. Tasks 3–13 each remove exactly their own offenders. Task 15 proves the package green. Do not "fix" another task's offenders early.

## Review Focus

1. **A default test that tolerates the git failure** passes even though it reached git. The census found four of these: `TestGateLaunchInvalidInput`, `TestGateLaunchOutsideGitUnchanged`, `TestResolveRepoPhaseInvalidExplicitRepoDir` and `TestResolveRepoPhaseOutsideGitIsMachineOnly`. The package must still go red. This is pinned by `TestNoGitGuardFailsTolerantTest` (Task 1), which re-execs the test binary with a test that swallows the failure and asserts a non-zero exit plus the violation listing.
2. **The `e2e` build** (`tests/test_go_finalize_e2e.sh`, `-tags e2e`) must not get the shim, and helpers moved behind `integration` must not break its compile. Pinned by Task 1's e2e runner step and by every move task's `go vet -tags e2e ./internal/app/` step.
3. **An unreadable violation log** must fail closed, not read as "no violations" (learning probe-error-is-not-clean-absence). Pinned by the `unreadable log fails` row of `TestNoGitGuardVerdict` (Task 1).
4. **Budget-state key for a target outside `RepoRoot`, or a symlink-spelled root.** A `DOCKET_RUNTESTS_TESTS_DIR` override outside the repo must keep its absolute key, never a `../` key. A sibling directory whose name merely shares a prefix (`/w/a` vs `/w/ab`) is not "under" the root. A `/var` versus `/private/var` spelling of one checkout must still converge. Pinned by `TestBudgetKeyPathIsRepoRelative` and `TestBudgetKeyPathResolvesSymlinkedRoot` (Task 2).
5. **A new shard prefix that collides with an existing one**, which leaves a test doubly matched or unmatched. Pinned by each move task's prefix-collision step plus the contract's checks (4)/(5), run in every move task.

---

### Task 1: Default-build no-real-git guard in `internal/app` (the census)

**Build profile:** premium

The named risk is that `TestMain` also routes the supervisor and guardian re-exec roles of the test binary. A guard wired in the wrong place breaks every real `GateLaunch`/guardian test, and a guard compiled into the wrong build breaks the integration or e2e corpora.

**Files:**
- Create: `internal/app/nogit_guard_test.go` (`//go:build !integration && !e2e`, the guard plus its proving tests)
- Create: `internal/app/nogit_guard_off_test.go` (`//go:build integration || e2e`, the no-op twin). The name deliberately does **not** end in `_integration_test.go`, because the contract's check (1) requires such files to open with exactly `//go:build integration`.
- Modify: `internal/app/gate_test.go` (`TestMain` only)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces (later tasks rely on these exact names):
  - `func installNoGitGuard() func(code int) int`: default build installs the shim and returns the finisher; tagged builds return the identity.
  - `func nogitVerdict(logPath string, code int, w io.Writer) int`
  - constants `nogitGuardProbeArg = "docket-nogit-guard-probe"`, `nogitGuardDiagnostic = "docket nogit guard: default internal/app tests must not run real git"`, `nogitGuardExit = 97`, `nogitSelfProbeEnv = "DOCKET_NOGIT_GUARD_SELF_PROBE"`
  - package var `nogitGuardDir string` (the shim directory; `""` when not installed)
  - Census output: every default test that reaches git fails with stderr containing `docket nogit guard: default internal/app tests must not run real git`. After `m.Run` the package prints `docket nogit guard: default internal/app tests must not run real git: N real-git exec attempt(s) reached the guard shim …` followed by one `<cwd><TAB><argv>` line per attempt, and exits non-zero.

**Audit already done at plan time (re-verify in Step 1):** production code resolves git only through `PATH`.
- `gitcli.NewClient` runs `exec.LookPath("git")` at construction when `WithExecutable` is empty. No production caller passes `WithExecutable`, and no package-level client or `LookPath` exists.
- `internal/app/rungate_store.go` `gateGitCommonDir` and `internal/gatedrive/fingerprint.go` use bare `exec.Command("git", …)`.
- No `DOCKET_*` override names a git executable.
- Default `internal/app` tests that rewrite `PATH` (`finalize_e2e_test.go` `e2eEnv`, which is e2e-only) append `os.Getenv("PATH")`, so the shim stays in front.

A shim at the front of `PATH`, installed in `TestMain` before `m.Run`, therefore covers every path.

- [ ] **Step 1: Re-verify the git-resolution audit**

Run:
```bash
git grep -nE 'WithExecutable\(' -- internal cmd ':!*_test.go'
git grep -nE '^var .*(NewClient|LookPath)' -- internal
git grep -nE 'exec\.Command(Context)?\([^)]*"git"' -- internal/app internal/gatedrive internal/gitcli ':!*_test.go'
```
Expected: the first two print only the two `func WithExecutable` definitions (gitcli, githubcli) and nothing else. The third prints `internal/app/rungate_store.go` (`gateGitCommonDir`) and `internal/gatedrive/fingerprint.go`, both PATH-resolved. If anything else appears (an absolute git path, a cached `LookPath`, an env override), extend the guard to cover it and say so in your report. Do not document it as a residual (learning residual-is-for-undetectable-not-unprobed).

- [ ] **Step 2: Write the no-op twin and the proving tests with a deliberately inert guard (RED)**

Create `internal/app/nogit_guard_off_test.go`:

```go
//go:build integration || e2e

package app

// installNoGitGuard is the tagged builds' no-op twin of the default-build guard in
// nogit_guard_test.go (change 0465). The integration and e2e corpora exist to run
// real git, so they install no shim and the finisher returns m.Run's code as-is.
// Exactly one of the two files compiles for any tag set, so TestMain stays
// single-sourced.
func installNoGitGuard() func(code int) int { return func(code int) int { return code } }
```

Create `internal/app/nogit_guard_test.go` with the **inert** implementation below. It never touches `PATH` and never fails the package. This is the mutation state, and the tests must be red against it:

```go
//go:build !integration && !e2e

package app

// The default-build no-real-git guard (change 0465). Change 0333 moved the slow
// real-git, subprocess, and process-lifecycle corpus behind `//go:build integration`,
// but nothing stopped new real-git tests landing in the default corpus, which
// tests/test_go_race.sh instruments. This guard makes the partition an enforced
// invariant: the default-tag internal/app test corpus never starts a real `git`.
//
// Mechanism (keyed on the exec itself, never on spellings): installNoGitGuard, called
// from TestMain before m.Run, puts a directory holding a refusing `git` shim at the
// FRONT of PATH. Every route to git (gitcli.NewClient's exec.LookPath, a bare
// exec.Command("git", …), a fixture helper, a child process inheriting PATH) resolves
// the shim. The shim exits nogitGuardExit with the nogitGuardDiagnostic on stderr AND
// appends "<cwd>\t<argv>" to a violation log, so a test that tolerates the failure
// still turns the package red when nogitVerdict reads the log after m.Run.
//
// Only this guard's own proving tests may call the shim without recording a
// violation, by passing nogitGuardProbeArg as the first argument.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

const (
	nogitGuardProbeArg   = "docket-nogit-guard-probe"
	nogitGuardDiagnostic = "docket nogit guard: default internal/app tests must not run real git"
	nogitGuardExit       = 97
	nogitSelfProbeEnv    = "DOCKET_NOGIT_GUARD_SELF_PROBE"
)

// nogitGuardDir is the installed shim directory ("" when the guard is not installed).
var nogitGuardDir string

// INERT (Step 2 only): replaced in Step 4.
func installNoGitGuard() func(code int) int { return func(code int) int { return code } }

// INERT (Step 2 only): replaced in Step 4.
func nogitVerdict(logPath string, code int, w io.Writer) int { return code }

// TestNoGitGuardShadowsGitOnPath: every PATH lookup of `git` (gitcli.NewClient uses
// exec.LookPath) resolves the shim, not a real git.
func TestNoGitGuardShadowsGitOnPath(t *testing.T) {
	if nogitGuardDir == "" {
		t.Fatalf("the no-real-git guard is not installed (nogitGuardDir empty); TestMain must call installNoGitGuard before m.Run")
	}
	p, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath(git): %v", err)
	}
	if filepath.Dir(p) != nogitGuardDir {
		t.Fatalf("git resolves to %q, want the guard shim in %q", p, nogitGuardDir)
	}
}

// TestNoGitGuardRefusesBareExec pins the MECHANISM, not just "it failed": real git
// also fails on an unknown subcommand, so the assert is the guard's exit code AND
// its diagnostic (learning assert-pins-outcome-not-mechanism).
func TestNoGitGuardRefusesBareExec(t *testing.T) {
	out, err := exec.Command("git", nogitGuardProbeArg).CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != nogitGuardExit {
		t.Fatalf("git exec must exit %d from the guard shim, got err=%v output=%q", nogitGuardExit, err, out)
	}
	if !strings.Contains(string(out), nogitGuardDiagnostic) {
		t.Fatalf("git exec output must carry the guard diagnostic %q, got %q", nogitGuardDiagnostic, out)
	}
}

// TestNoGitGuardVerdict covers the post-m.Run verdict over the violation log.
func TestNoGitGuardVerdict(t *testing.T) {
	dir := testsupport.TempDir(t) // bare X.TempDir() is banned by internal/repoguard tempdir_fixture_test.go
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name     string
		logPath  string
		code     int
		want     int
		wantText string
	}{
		{"missing log is clean", filepath.Join(dir, "absent.log"), 0, 0, ""},
		{"empty log is clean", write("empty.log", ""), 0, 0, ""},
		{"one violation fails a green run", write("one.log", "/tmp/x\tstatus --porcelain\n"), 0, 1, "1 real-git exec attempt(s)"},
		{"violation keeps an existing failure code", write("keep.log", "/tmp/x\tlog\n"), 2, 2, "/tmp/x\tlog"},
		{"unreadable log fails closed", dir, 0, 1, "cannot read the violation log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			got := nogitVerdict(tc.logPath, tc.code, &buf)
			if got != tc.want {
				t.Fatalf("nogitVerdict(%q, %d) = %d, want %d; output:\n%s", tc.logPath, tc.code, got, tc.want, buf.String())
			}
			if tc.wantText == "" && buf.Len() != 0 {
				t.Fatalf("clean verdict must print nothing, got:\n%s", buf.String())
			}
			if tc.wantText != "" && !strings.Contains(buf.String(), tc.wantText) {
				t.Fatalf("verdict output must contain %q, got:\n%s", tc.wantText, buf.String())
			}
		})
	}
}

// TestNoGitGuardFailsTolerantTest proves a test that SWALLOWS the git failure still
// fails the package: it re-execs this test binary running only itself in child
// mode, where it runs `git status` and ignores the error, then asserts the child
// binary exits non-zero and lists the violation.
func TestNoGitGuardFailsTolerantTest(t *testing.T) {
	if os.Getenv(nogitSelfProbeEnv) == "1" {
		_ = exec.Command("git", "status").Run() // tolerated on purpose
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNoGitGuardFailsTolerantTest$", "-test.count=1")
	cmd.Env = append(os.Environ(), nogitSelfProbeEnv+"=1")
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() == 0 {
		t.Fatalf("a package whose test tolerated a git exec must exit non-zero, got err=%v output:\n%s", err, out)
	}
	for _, want := range []string{nogitGuardDiagnostic, "1 real-git exec attempt(s)", "\tstatus"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("child output must contain %q, got:\n%s", want, out)
		}
	}
}

// keep imports referenced by the Step 4 implementation compiling in the inert state.
var _ = fmt.Sprintf
var _ fs.FileMode
```

Then change `TestMain` in `internal/app/gate_test.go`. Replace its last line `os.Exit(m.Run())` with the lines below and extend its doc comment:

```go
	// Change 0465: the default build installs the no-real-git guard (nogit_guard_test.go)
	// AFTER the re-exec routing above, so the supervisor and guardian roles behave
	// exactly as before; tagged builds get the no-op twin (nogit_guard_off_test.go).
	finish := installNoGitGuard()
	os.Exit(finish(m.Run()))
```

Append this sentence to the `TestMain` doc comment: `Ordinary runs then install the default-build no-real-git guard (change 0465) around m.Run.`

- [ ] **Step 3: Run the proving tests and confirm they are RED against the inert guard**

Run: `go test -count=1 -run '^TestNoGitGuard' ./internal/app/`
Expected: FAIL.
- `TestNoGitGuardShadowsGitOnPath` fails with "guard is not installed".
- `TestNoGitGuardRefusesBareExec` fails, because real git exits 1 with "not a git command", not 97.
- `TestNoGitGuardVerdict` fails its three non-clean rows.
- `TestNoGitGuardFailsTolerantTest` fails because the child exits 0.

Save this output. It is the mutation evidence for both mutation arms, taken before the guard exists.

- [ ] **Step 4: Replace the two inert functions with the real guard (GREEN)**

In `internal/app/nogit_guard_test.go`, delete the two `// INERT` functions and the two `var _` lines, and add:

```go
// installNoGitGuard installs the refusing git shim at the front of PATH and returns
// the finisher TestMain wraps around m.Run. Setup failure exits the binary non-zero:
// a guard that silently failed to install would certify nothing.
func installNoGitGuard() func(code int) int {
	// tempdir-exempt: TestMain installs the shim for the whole package run; there is no t to own a fixture dir.
	dir, err := os.MkdirTemp("", "docket-app-nogit-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot create the shim directory: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	logPath := filepath.Join(dir, "violations.log")
	if strings.ContainsAny(logPath, "'\n") {
		fmt.Fprintf(os.Stderr, "%s: shim log path %q is not single-quote safe\n", nogitGuardDiagnostic, logPath)
		os.Exit(1)
	}
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(nogitShimScript(logPath)), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot write the shim: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	// Explicit chmod: a create-time mode is masked by the umask.
	if err := os.Chmod(shim, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot chmod the shim: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot prepend the shim to PATH: %v\n", nogitGuardDiagnostic, err)
		os.Exit(1)
	}
	nogitGuardDir = dir
	return func(code int) int {
		verdict := nogitVerdict(logPath, code, os.Stderr)
		_ = os.RemoveAll(dir)
		return verdict
	}
}

// nogitShimScript renders the refusing git: it records every non-probe invocation
// as "<cwd>\t<argv>" in logPath and always exits nogitGuardExit with the diagnostic
// and the remedy on stderr.
func nogitShimScript(logPath string) string {
	return "#!/bin/sh\n" +
		"if [ \"${1-}\" != '" + nogitGuardProbeArg + "' ]; then\n" +
		"  printf '%s\\t%s\\n' \"$PWD\" \"$*\" >> '" + logPath + "'\n" +
		"fi\n" +
		"printf '%s (git %s): move the test behind //go:build integration with a TestIntegration prefix and a tests/test_go_integration_app_*.sh shard (change 0465; partition from change 0333)\\n' '" +
		nogitGuardDiagnostic + "' \"$*\" >&2\n" +
		fmt.Sprintf("exit %d\n", nogitGuardExit)
}

// nogitVerdict folds the violation log into m.Run's exit code. A missing or empty
// log is clean and leaves code unchanged. Any recorded attempt, or a log that exists
// but cannot be read, fails the package: a probe error is never clean absence.
func nogitVerdict(logPath string, code int, w io.Writer) int {
	raw, err := os.ReadFile(logPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(w, "%s: cannot read the violation log %s: %v\n", nogitGuardDiagnostic, logPath, err)
		return nogitFailCode(code)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return code
	}
	fmt.Fprintf(w, "%s: %d real-git exec attempt(s) reached the guard shim (a test that tolerated the failure still counts); <cwd>\\t<argv>:\n", nogitGuardDiagnostic, len(lines))
	for _, l := range lines {
		fmt.Fprintf(w, "  %s\n", l)
	}
	return nogitFailCode(code)
}

func nogitFailCode(code int) int {
	if code == 0 {
		return 1
	}
	return code
}
```

(The `\\t` inside the Go string in `nogitVerdict`'s header is intentional. It prints a literal `\t` as the legend, while the data lines carry a real tab.)

- [ ] **Step 5: Run the proving tests GREEN**

Run: `gofmt -l internal/app && go vet ./internal/app/ && go test -count=1 -run '^TestNoGitGuard' -v ./internal/app/`
Expected: `gofmt -l` prints nothing, vet is clean, and all four `TestNoGitGuard*` tests PASS with exit 0. The proving tests record no violation, because the probe uses `nogitGuardProbeArg`, the verdict rows use their own temp logs, and the tolerant child is a separate process.

- [ ] **Step 6: Mutation probes (backup-copy restore, `-count=1`)**

```bash
bash -c '
set -uo pipefail
f=internal/app/nogit_guard_test.go
bak="$(mktemp "${TMPDIR:-/tmp}/nogit-guard.XXXXXX")"; cp "$f" "$bak"
# (a) strip the PATH prepend: shadow + bare-exec + tolerant-child must go red
perl -0pi -e "s/if err := os\.Setenv\(\"PATH\".*?\n\t\}\n//s" "$f"
grep -c -F -e "os.Setenv(\"PATH\"" "$f"   # expect 0 (proves the mutation landed)
go test -count=1 -run "^TestNoGitGuard" ./internal/app/ ; echo "mutation-a rc=$?"
cp "$bak" "$f"
# (b) make the verdict ignore violations: verdict rows + tolerant-child must go red
perl -0pi -e "s/return nogitFailCode\(code\)\n\}\n\nfunc nogitFailCode/return code\n}\n\nfunc nogitFailCode/s" "$f"
go test -count=1 -run "^TestNoGitGuard" ./internal/app/ ; echo "mutation-b rc=$?"
cp "$bak" "$f"; rm -f "$bak"
go test -count=1 -run "^TestNoGitGuard" ./internal/app/ ; echo "restored rc=$?"
'
```
Expected:
- mutation (a): `grep -c` prints `0`, the run FAILs naming at least `TestNoGitGuardShadowsGitOnPath`, `TestNoGitGuardRefusesBareExec` and `TestNoGitGuardFailsTolerantTest`, and `mutation-a rc=1`.
- mutation (b): FAILs naming `TestNoGitGuardVerdict` and `TestNoGitGuardFailsTolerantTest`, and `mutation-b rc=1`.
- `restored rc=0`.

If a mutation leaves the run green, the guard is decoration: stop and fix it. Also confirm `git diff --stat internal/app/nogit_guard_test.go` shows your Step 4 content, not an empty or HEAD diff. Paste the three `rc=` lines and the failing test names into your report. The results file cites them.

- [ ] **Step 7: Prove the tagged builds are untouched**

Run:
```bash
go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
bash tests/test_go_integration_app_named.sh; echo "named rc=$?"
bash tests/test_go_finalize_e2e.sh; echo "e2e rc=$?"
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
```
Expected: both vets are clean, and all three runners print only `ok - ` lines with `rc=0`. The integration and e2e corpora still reach real git, so no shim is installed there.

- [ ] **Step 8: Take the baseline census (expected RED)**

Run:
```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
printf "top-level failures: %s\n" "$(grep -c -E -e . <<<"$fails")"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the package FAILs with about 333 top-level failures, and the guard summary line reports more attempts than that (several tests exec git more than once). The four tolerant tests show up only in the attempt listing (Review Focus 1). Record both numbers in your report as the Task 1 baseline census. **Do not fix any offender here.** Tasks 3–13 move them.

- [ ] **Step 9: Commit**

```bash
git add internal/app/nogit_guard_test.go internal/app/nogit_guard_off_test.go internal/app/gate_test.go
git commit -m "test(app): default-build no-real-git guard (census) for internal/app (change 0465)"
```

---

### Task 2: Budget-state key on the repo-relative target path

**Files:**
- Modify: `internal/suiterunner/budgetstate.go` (add `budgetKeyPath`, next to `ContextKey`)
- Modify: `internal/suiterunner/run.go` (the `ContextKey(o.Target.Path, …)` call in `Run`'s budget classification loop)
- Test: `internal/suiterunner/budgetstate_test.go`, `internal/suiterunner/run_test.go`

**Interfaces:**
- Consumes: `Config.RepoRoot` (the git toplevel `internal/cli/development_test_cmd.go` passes), `ContextKey`, and the test helpers `writeScript`/`bashPath` (`execute_test.go`), `writeDurations` (`budgetstate_test.go`) and `runCfg` (`run_test.go`).
- Produces: `func budgetKeyPath(repoRoot, path string) string`. `Run` now keys budget state on `budgetKeyPath(cfg.RepoRoot, o.Target.Path)`. `ScreenObs.Path` (the human-readable report and trailing store column) keeps the target path as given.

Today every `.worktrees/<slug>` gets its own record, because the key leads with the absolute path. The streak never reaches the 5-overrun serial confirmation. That is why `test_go_race` at 200–558s against a 60s row was never confirmed. Existing records are orphaned once. That is acceptable: the store is advisory and fail-open, and no schema bump is needed.

- [ ] **Step 1: Write the failing tests**

Append to `internal/suiterunner/budgetstate_test.go`:

```go
// Change 0465: the budget-state key leads with the REPO-RELATIVE target path, so every
// worktree of one repository accumulates one record per target.
func TestBudgetKeyPathIsRepoRelative(t *testing.T) {
	cases := []struct{ name, root, path, want string }{
		{"under the primary checkout", "/w/docket", "/w/docket/tests/test_x.sh", "tests/test_x.sh"},
		{"under a linked worktree", "/w/docket/.worktrees/fix-y", "/w/docket/.worktrees/fix-y/tests/test_x.sh", "tests/test_x.sh"},
		{"already relative", "/w/docket", "tests/test_x.sh", "tests/test_x.sh"},
		{"no repo root", "", "/w/docket/tests/test_x.sh", "/w/docket/tests/test_x.sh"},
		{"outside the root keeps its absolute key", "/w/docket", "/elsewhere/tests/test_x.sh", "/elsewhere/tests/test_x.sh"},
		{"a sibling sharing a name prefix is not under the root", "/w/a", "/w/ab/tests/test_x.sh", "/w/ab/tests/test_x.sh"},
	}
	for _, tc := range cases {
		if got := budgetKeyPath(tc.root, tc.path); got != tc.want {
			t.Errorf("%s: budgetKeyPath(%q, %q) = %q, want %q", tc.name, tc.root, tc.path, got, tc.want)
		}
	}
}

func TestContextKeySameAcrossCheckouts(t *testing.T) {
	a := ContextKey(budgetKeyPath("/Users/x/docket", "/Users/x/docket/tests/test_go_race.sh"), 8, 8, "Darwin", "arm64", 60, ModeParallel)
	b := ContextKey(budgetKeyPath("/Users/x/docket/.worktrees/fix-y", "/Users/x/docket/.worktrees/fix-y/tests/test_go_race.sh"), 8, 8, "Darwin", "arm64", 60, ModeParallel)
	want := "tests/test_go_race.sh|j8|c8|Darwin|arm64|b60|mparallel|s1"
	if a != want || b != want {
		t.Fatalf("keys must converge on %q, got primary=%q worktree=%q", want, a, b)
	}
}

// A symlink-spelled root (macOS /var vs /private/var) must still converge.
func TestBudgetKeyPathResolvesSymlinkedRoot(t *testing.T) {
	real := testsupport.TempDir(t)
	if err := os.MkdirAll(filepath.Join(real, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(real, "tests", "test_x.sh")
	if err := os.WriteFile(target, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(testsupport.TempDir(t), "checkout-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got := budgetKeyPath(link, target); got != "tests/test_x.sh" {
		t.Fatalf("budgetKeyPath(%q, %q) = %q, want tests/test_x.sh", link, target, got)
	}
}
```

Append to `internal/suiterunner/run_test.go`. Add `"path/filepath"` and `"os"` only if they are not already imported there. Both are imported today.

```go
// Change 0465: two checkouts of one repository (a primary and a .worktrees/<slug>)
// sharing one budget-state store accumulate ONE streak for the same target. Before
// the fix each absolute path minted its own record and the second run read 1/5.
func TestRunBudgetStateConvergesAcrossCheckouts(t *testing.T) {
	state := filepath.Join(testsupport.TempDir(t), "state.tsv")
	durations := writeDurations(t, [][3]string{{"test_slow.sh", "1000", "1"}})
	run := func(root string) string {
		t.Helper()
		tests := filepath.Join(root, "tests")
		if err := os.MkdirAll(tests, 0o755); err != nil {
			t.Fatal(err)
		}
		writeScript(t, tests, "slow", "# docket-suite: go\necho 'ok - slow'\n")
		var out, errBuf bytes.Buffer
		cfg := runCfg(t, tests, &out, &errBuf)
		cfg.RepoRoot = root
		cfg.StatePath = state
		cfg.DurationsPath = durations
		if code := Run(context.Background(), cfg); code != 0 {
			t.Fatalf("run in %s exit = %d\nstdout:\n%s\nstderr:\n%s", root, code, out.String(), errBuf.String())
		}
		return out.String()
	}
	primary := filepath.Join(testsupport.TempDir(t), "docket")
	first := run(primary)
	second := run(filepath.Join(primary, ".worktrees", "fix-slow"))
	if !strings.Contains(first, "consecutive parallel-overrun streak 1/5") {
		t.Fatalf("first checkout must open the streak at 1/5:\n%s", first)
	}
	if !strings.Contains(second, "consecutive parallel-overrun streak 2/5") {
		t.Fatalf("second checkout must CONTINUE the same record (2/5), not start its own:\n%s", second)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestBudgetKeyPath|TestContextKeySameAcrossCheckouts|TestRunBudgetStateConvergesAcrossCheckouts' ./internal/suiterunner/`
Expected: a build failure, `undefined: budgetKeyPath`. Once Step 3 adds only the function, `TestRunBudgetStateConvergesAcrossCheckouts` still fails with `streak 1/5` in the second output until the `run.go` call site changes.

- [ ] **Step 3: Implement**

In `internal/suiterunner/budgetstate.go`, directly below `ContextKey`:

```go
// budgetKeyPath renders a target path for the budget-state context key relative to
// the checkout root (change 0465), so every worktree of one repository accumulates
// one record per target and the screen-then-confirm streak can actually reach its
// serial confirmation. An already-relative path, an empty root, or a path outside
// the root (a DOCKET_RUNTESTS_TESTS_DIR override) is returned unchanged; a
// symlink-spelled root is compared after resolving both sides.
func budgetKeyPath(repoRoot, path string) string {
	if repoRoot == "" || !filepath.IsAbs(path) {
		return path
	}
	if rel, ok := relUnderRoot(repoRoot, path); ok {
		return rel
	}
	rr, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return path
	}
	rp, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	if rel, ok := relUnderRoot(rr, rp); ok {
		return rel
	}
	return path
}

// relUnderRoot reports path relative to root when it lies strictly inside root.
func relUnderRoot(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
```

In `internal/suiterunner/run.go`, change the key line in the budget-classification loop from:

```go
			key := ContextKey(o.Target.Path, cfg.Jobs, cpus, osName, arch, ceil, o.Target.Mode)
```
to:
```go
			// Change 0465: key on the repo-relative path so worktrees share one record.
			key := ContextKey(budgetKeyPath(cfg.RepoRoot, o.Target.Path), cfg.Jobs, cpus, osName, arch, ceil, o.Target.Mode)
```

Leave `ScreenObs{…, Path: o.Target.Path, …}` unchanged.

Next, check whether any other `ContextKey(` caller keys on the target path. The strict path, `StrictConfirmCandidates` and `ScheduleConfirmation` all read `o.Key`/records. Run:

```bash
git grep -n -e 'ContextKey(' -- internal ':!*_test.go'
```
Expected: the definition plus the one `run.go` call you just changed. If there is another path-keyed call, route it through `budgetKeyPath` as well and say so in your report.

- [ ] **Step 4: Run the tests to verify they pass, plus the package**

Run: `gofmt -l internal/suiterunner && go test -count=1 ./internal/suiterunner/`
Expected: no gofmt output, and PASS, including the pre-existing `TestContextKeyRendersOracleFormat` and the budget-state suite.

- [ ] **Step 5: Mutation probe**

Back up `internal/suiterunner/run.go` to a `mktemp` copy. Revert the call site to `ContextKey(o.Target.Path, …)` and run `go test -count=1 -run TestRunBudgetStateConvergesAcrossCheckouts ./internal/suiterunner/`. Expected: FAIL (`streak 1/5` in the second output). Copy the backup back and re-run. Expected: PASS. Put both results in your report.

- [ ] **Step 6: Commit**

```bash
git add internal/suiterunner/budgetstate.go internal/suiterunner/run.go internal/suiterunner/budgetstate_test.go internal/suiterunner/run_test.go
git commit -m "fix(suiterunner): key budget state on the repo-relative target path (change 0465)"
```

---
### Task 3: Move the run-gate cancel real-git tests into the `TestIntegrationGateCancel` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/rungate_cancel_test.go` → `internal/app/rungate_cancel_integration_test.go`
- Create: `tests/test_go_integration_app_gatecancel.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_gatecancel.sh`)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_gatecancel.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationGateCancel"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationGateCancel<OldName minus Test>`, e.g. `TestCancelCompletesWhenLaunchObligationsSettle` → `TestIntegrationGateCancelCancelCompletesWhenLaunchObligationsSettle`. If Step 2 race-classifies a test anyway, it becomes `TestRaceIntegrationAppConcurrency<OldName minus Test>` and joins the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 43 offenders: the run-gate cancel, retirement and terminal-repair tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `rungate_cancel_test.go` (all 43): `TestCancelCompletesWhenLaunchObligationsSettle`, `TestCancelConcurrentReplayIsIdempotent`, `TestCancelFencesBeforeStopping`, `TestCancelInterruptedBetweenRetireAndFinalizeConverges`, `TestCancelLeavesUnlinkedEpochlessSlot`, `TestCancelNativeAdapterAbsentIsFindingNotSilence`, `TestCancelNeverChargesOrResets`, `TestCancelNeverTouchesForeignSlot`, `TestCancelPendingOnUncompletedMutation`, `TestCancelPendingWhenRetirementFails`, `TestCancelPendingWhileLaunchObligationUnresolved`, `TestCancelReconcilerUnavailableFailsClosed`, `TestCancelReleaseWriteFailureFailsClosed`, `TestCancelRemovedWorktreeEpochReachesSlotByStoredIdentity`, `TestCancelRepeatResumesCleanup`, `TestCancelRetireRaceWithSuccessorLeavesSuccessor`, `TestCancelRetiresOwnedReleasedSlot`, `TestCancelSettlesUncertainPublicationWithIdenticalRetry`, `TestCancelStaysPendingWithoutCompletedIdenticalRetry`, `TestCancelStopsLinkedEpochlessSlot`, `TestFinalizeGateAdmitsAfterRetirement`, `TestGuardianReapsButNeverRetires`, `TestRawAdmissionStoreWiresEpochSettledResolver`, `TestRawLaunchSettlesSettledEpochReleasedSlot`, `TestRawStaleEpochRefusalStillFencesBusySlot`, `TestRepairChargesNothing`, `TestRepairTerminalEpochRemovedWorktree`, `TestRetirementDoesNotUnfenceOldEpochLaunches`, `TestRetirementSitesConverge`, `TestRunCancelAlreadyCancelled`, `TestRunCancelHappyPath`, `TestRunCancelPendingOnUnprovenStop`, `TestRunCancelPublicEntry`, `TestRunCancelRefusedWrongClaim`, `TestRunCancelRefusedWrongEpoch`, `TestRunCancelRefusedWrongRepo`, `TestRunCancelRefusesCompletedEpoch`, `TestRunCancelWinsFromCompletingEpoch`, `TestTerminalRepairRefusesUnsafeHistories`, `TestTerminalRepairRetiresHistoricalStaleSlot`, `TestTerminalRepairSupersededSlot`, `TestTerminalRepairSupersededThreadsReplacementWorktree`, `TestTerminalRepairTornResumeConverges`

**Race candidates:** none were pre-classified. Apply the Step 2 criterion anyway.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task3.txt"
grep -E -e "^internal/app/(rungate_cancel)_test\.go " "${TMPDIR:-/tmp}/census-task3.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/rungate_cancel_test.go internal/app/rungate_cancel_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_cancel_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_cancel_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationGateCancel"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_gatecancel.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_gatecancel.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_gatecancel.sh — Go integration shard (change 0465, extending change
# 0333's partition): the run-gate cancel, retirement and terminal-repair tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationGateCancel. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationGateCancel"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/rungate_cancel_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_gatecancel.sh; echo "shard rc=$?"
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0`.
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_gatecancel.sh' 2>&1 | tail -4
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_gatecancel.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_gatecancel.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move run-gate cancel real-git tests behind the integration tag (TestIntegrationGateCancel, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 4: Move the run-gate verdict real-git tests into the `TestIntegrationGateVerdict` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/rungate_verdict_test.go` → `internal/app/rungate_verdict_integration_test.go`
- Create: `tests/test_go_integration_app_gateverdict.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_gateverdict.sh`)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_gateverdict.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationGateVerdict"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationGateVerdict<OldName minus Test>`, e.g. `TestVerdictAbsentBindingAdoptsSoleProof` → `TestIntegrationGateVerdictVerdictAbsentBindingAdoptsSoleProof`. If Step 2 race-classifies a test anyway, it becomes `TestRaceIntegrationAppConcurrency<OldName minus Test>` and joins the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 32 offenders: the run-gate verdict and claim-binding tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `rungate_verdict_test.go` (all 32): `TestVerdictAbsentBindingAdoptsSoleProof`, `TestVerdictAmbiguousDrivesStops`, `TestVerdictClaimReplacedStops`, `TestVerdictConfirmedBindingResolvesBoundChange`, `TestVerdictContinuationConsumesNoAttempt`, `TestVerdictContinuationDoesNotRebindScope`, `TestVerdictContinueNeverAuthorizesNewClaim`, `TestVerdictCorruptBindingFailsClosed`, `TestVerdictFreshRunBindsScopeChange`, `TestVerdictHaltPrecedenceOverBudget`, `TestVerdictIncompleteNoGrantLeavesRetryMirrorUnused`, `TestVerdictIncompleteQuiescentStillRetriesOnce`, `TestVerdictIncompleteRepeatObservationDoesNotDoubleGrant`, `TestVerdictIncompleteRespectsAttemptLimit`, `TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry`, `TestVerdictNilProofScannerFailsClosed`, `TestVerdictNoBindingNoProofIsNoAttributableClaim`, `TestVerdictObserveModeNeverTouchesOwnership`, `TestVerdictObservePathStillCannotContinue`, `TestVerdictOwnershipIgnoresBeforeSetAndEpoch`, `TestVerdictProofScanErrorFailsClosed`, `TestVerdictResumeBindingSkipsContinuity`, `TestVerdictRunCompleteBlockedCloseoutStopsWithoutSuccess`, `TestVerdictRunCompleteCancelledEpochNeverReportsSuccess`, `TestVerdictRunCompleteClosesOutEpochOwnership`, `TestVerdictRunCompleteReportPersistFailureIsReported`, `TestVerdictRunCompleteWithoutEpochUnchanged`, `TestVerdictTakeoverHaltStops`, `TestVerdictUnconfirmedReservationRecoversFromExactReceipt`, `TestVerdictUnconfirmedReservationSiblingContextHashIsNoAttributableClaim`, `TestVerdictUnconfirmedReservationWithoutReceiptStops`, `TestVerdictWaitingIsNonterminalContinue`

**Race candidates:** none were pre-classified. Apply the Step 2 criterion anyway.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task4.txt"
grep -E -e "^internal/app/(rungate_verdict)_test\.go " "${TMPDIR:-/tmp}/census-task4.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/rungate_verdict_test.go internal/app/rungate_verdict_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_verdict_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_verdict_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationGateVerdict"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_gateverdict.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_gateverdict.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_gateverdict.sh — Go integration shard (change 0465, extending change
# 0333's partition): the run-gate verdict and claim-binding tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationGateVerdict. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationGateVerdict"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/rungate_verdict_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_gateverdict.sh; echo "shard rc=$?"
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0`.
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_gateverdict.sh' 2>&1 | tail -4
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_gateverdict.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_gateverdict.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move run-gate verdict real-git tests behind the integration tag (TestIntegrationGateVerdict, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 5: Move the run-gate fence and ownership real-git tests into the `TestIntegrationGateFence` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/rungate_fence_test.go` → `internal/app/rungate_fence_integration_test.go`
- Move (whole file, `git mv`): `internal/app/rungate_ownership_test.go` → `internal/app/rungate_ownership_integration_test.go`
- Create: `tests/test_go_integration_app_gatefence.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_gatefence.sh`)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_gatefence.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationGateFence"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationGateFence<OldName minus Test>`, e.g. `TestAdmitWorkflowMutationRefusesCompletingEpoch` → `TestIntegrationGateFenceAdmitWorkflowMutationRefusesCompletingEpoch`. If Step 2 race-classifies a test anyway, it becomes `TestRaceIntegrationAppConcurrency<OldName minus Test>` and joins the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 30 offenders: the run-gate fencing and ownership tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `rungate_fence_test.go` (all 26): `TestAdmitWorkflowMutationRefusesCompletingEpoch`, `TestCompletedEpochExcludedFromAmbientOwnerLookup`, `TestEpochCarryingFencesUnchangedByOwnerSelection`, `TestFenceBlocksEngineMutationAfterCancel`, `TestFenceBlocksPRPublishAfterCancel`, `TestFenceBlocksWorkspacePublishAfterCancel`, `TestFenceMatchesWorktreeAcrossSymlinkAlias`, `TestFenceRefusesSupersededEpochAsStale`, `TestFreshRunClaimBindsEpochWorktreeSoFenceActs`, `TestInFlightMutationReconcilesBeforeCancelled`, `TestOwnerSelectionActiveBeatsCancelledRegardlessOfOrder`, `TestOwnerSelectionCompletedNeverOwns`, `TestOwnerSelectionSoleCancelledStillFences`, `TestOwnerSelectionTwoActiveOwnersAmbiguous`, `TestProductionUncertainThenIdenticalRetryThenCancel`, `TestProductionUnverifiedPRRetryNeverSettles`, `TestProductionUnverifiedWorkspaceRetryNeverSettles`, `TestPRPublishJournalsPublicationIdentity`, `TestSlotNamedEpochUnreadableRefusesLocally`, `TestStandaloneMutationUnfenced`, `TestUnreadableSlotRefusesLocally`, `TestVerdictRecoveryUnresolvedIdentityStopsBeforeConfirm`, `TestVerdictSoleProofAdoptionBindsEpochWorktreeSoFenceActs`, `TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs`, `TestWorkspacePublishJournalsPublicationIdentity`, `TestWorkspacePublishMovedHeadUnderLockIsHeadMismatch`
- `rungate_ownership_test.go` (all 4): `TestLaterVerdictCannotOverwriteBinding`, `TestReplacementClaimBlocksOldGate`, `TestTwoGatesEachVerifyOnlyTheirOwn`, `TestUnrelatedChurnDoesNotMoveOwnership`

**Race candidates:** none were pre-classified. Apply the Step 2 criterion anyway.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task5.txt"
grep -E -e "^internal/app/(rungate_fence|rungate_ownership)_test\.go " "${TMPDIR:-/tmp}/census-task5.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/rungate_fence_test.go internal/app/rungate_fence_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_fence_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_fence_integration_test.go
git mv internal/app/rungate_ownership_test.go internal/app/rungate_ownership_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_ownership_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_ownership_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationGateFence"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_gatefence.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_gatefence.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_gatefence.sh — Go integration shard (change 0465, extending change
# 0333's partition): the run-gate fencing and ownership tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationGateFence. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationGateFence"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/rungate_fence_test.go|internal/app/rungate_ownership_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_gatefence.sh; echo "shard rc=$?"
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0`.
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_gatefence.sh' 2>&1 | tail -4
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_gatefence.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_gatefence.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move run-gate fence and ownership real-git tests behind the integration tag (TestIntegrationGateFence, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 6: Move the run-gate completion and publication real-git tests into the `TestIntegrationGateCompletion` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/rungate_complete_test.go` → `internal/app/rungate_complete_integration_test.go`
- Move (whole file, `git mv`): `internal/app/rungate_production_census_test.go` → `internal/app/rungate_production_census_integration_test.go`
- Split (only the 7 listed tests of 12): `internal/app/rungate_publication_test.go` → new `internal/app/rungate_publication_integration_test.go`
- Split (only the 2 listed tests of 3): `internal/app/rungate_publication_settle_paths_test.go` → new `internal/app/rungate_publication_settle_paths_integration_test.go`
- Create: `tests/test_go_integration_app_gatecompletion.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_gatecompletion.sh`; re-measure the `tests/test_go_integration_app_concurrency.sh` row if a test is race-classified)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_gatecompletion.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationGateCompletion"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationGateCompletion<OldName minus Test>`, e.g. `TestCompleteSuccessfulRunBlocksOnEveryUnsettledObligation` → `TestIntegrationGateCompletionCompleteSuccessfulRunBlocksOnEveryUnsettledObligation`. Race-classified tests become `TestRaceIntegrationAppConcurrency<OldName minus Test>` and join the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 32 offenders: the run-gate completion, production-census, and publication-settlement tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `rungate_complete_test.go` (all 20): `TestCompleteSuccessfulRunBlocksOnEveryUnsettledObligation`, `TestCompleteSuccessfulRunBlocksOnUnverifiedRetry`, `TestCompleteSuccessfulRunDoesNotDuplicateFindings`, `TestCompleteSuccessfulRunForeignSuccessorUntouched`, `TestCompleteSuccessfulRunHappyPath`, `TestCompleteSuccessfulRunIdempotentReplay`, `TestCompleteSuccessfulRunLateParticipantBlocks`, `TestCompleteSuccessfulRunNeverRelabelsCancellation`, `TestCompleteSuccessfulRunReplayAfterRetireBeforeComplete`, `TestCompleteSuccessfulRunSendsNoStops`, `TestCompleteSuccessfulRunSettlesUncertainPublication`, `TestCompleteSuccessfulRunSkipsObservingNonReleasedOwnedSlot`, `TestCompleteSuccessfulRunStillBlocksWithoutRetry`, `TestCompleteThenScratchCleanupThenFinalizeAdmits`, `TestCompletionParticipantDurableProof`, `TestCompletionSlotReleasedOwnedNoReobservation`, `TestCompletionUnreleasedOwnedSlotStillBlocks`, `TestOrdinaryReleaseStillRetainsEpochBetweenDrives`, `TestReadOnlyPathsNeverSettle`, `TestStandaloneFinalizeAdmissionBlockedThenAdmittedAroundCloseout`
- `rungate_production_census_test.go` (all 3): `TestProductionCensusCancelResumeStartsReplacementGate`, `TestProductionCensusCancelThenFinalize`, `TestProductionCensusCompleteThenFinalize`
- `rungate_publication_test.go` (7 of 12; the rest stay in the default file): `TestAdmissionJournalsPublicationDescriptorAndLegacyDecodes`, `TestJournaledRetryOutcomeGatesSettlement`, `TestSettlementInterruptionConverges`, `TestSettlementNeverDowngradesUnderRacingCallback`, `TestSettleUncertainPublicationsDurable`, `TestSettleUncertainPublicationsFailureIsBoundedFinding`, `TestSettleUncertainPublicationsWriteFailureReportsNoSettlement`
- `rungate_publication_settle_paths_test.go` (2 of 3; the rest stay in the default file): `TestReadOnlyVerdictPathsNeverSettleSettleablePair`, `TestVerdictRunCompleteSettlesUncertainPublication`

**Race candidates, pre-classified at plan time** (their bodies start goroutines or hold two live launches at once; confirm against the criterion in Step 2): `TestSettlementNeverDowngradesUnderRacingCallback`.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task6.txt"
grep -E -e "^internal/app/(rungate_complete|rungate_production_census|rungate_publication|rungate_publication_settle_paths)_test\.go " "${TMPDIR:-/tmp}/census-task6.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/rungate_complete_test.go internal/app/rungate_complete_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_complete_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_complete_integration_test.go
git mv internal/app/rungate_production_census_test.go internal/app/rungate_production_census_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_production_census_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_production_census_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

Split files. Create each new file with this header, then **cut** each listed test function, together with its doc comment, out of the source file and paste it below the header. The file's non-`Test` helpers stay where they are:

```go
//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_gatecompletion.sh (prefix ^TestIntegrationGateCompletion).

import (
	// exactly the imports the moved functions use; `go vet` in Step 4 names any gap
)
```
- `internal/app/rungate_publication_test.go` → `internal/app/rungate_publication_integration_test.go`: 7 functions (listed above).
- `internal/app/rungate_publication_settle_paths_test.go` → `internal/app/rungate_publication_settle_paths_integration_test.go`: 2 functions (listed above).

There is no goimports on this machine. Fix imports by hand from the compiler's `imported and not used` / `undefined:` messages in both the source and the new file.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationGateCompletion"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_gatecompletion.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_gatecompletion.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_gatecompletion.sh — Go integration shard (change 0465, extending change
# 0333's partition): the run-gate completion, production-census, and publication-settlement tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationGateCompletion. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationGateCompletion"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/rungate_complete_test.go|internal/app/rungate_production_census_test.go|internal/app/rungate_publication_test.go|internal/app/rungate_publication_settle_paths_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_gatecompletion.sh; echo "shard rc=$?"
bash tests/test_go_integration_app_concurrency.sh; echo "race shard rc=$?"   # only if a test was race-classified
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0` (and `race shard rc=0` when used).
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_gatecompletion.sh' 2>&1 | tail -4
bash -c 'time bash tests/test_go_integration_app_concurrency.sh' 2>&1 | tail -4   # only if a test was race-classified
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_gatecompletion.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes. If a race test joined `tests/test_go_integration_app_concurrency.sh`, re-measure it the same way and raise its row only if the new measurement exceeds it.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_gatecompletion.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move run-gate completion and publication real-git tests behind the integration tag (TestIntegrationGateCompletion, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 7: Move the run-gate epoch and store real-git tests into the `TestIntegrationGateEpoch` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/rungate_epoch_test.go` → `internal/app/rungate_epoch_integration_test.go`
- Move (whole file, `git mv`): `internal/app/rungate_store_test.go` → `internal/app/rungate_store_integration_test.go`
- Move (whole file, `git mv`): `internal/app/rungate_gate_test.go` → `internal/app/rungate_gate_integration_test.go`
- Create: `tests/test_go_integration_app_gateepoch.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_gateepoch.sh`; re-measure the `tests/test_go_integration_app_concurrency.sh` row if a test is race-classified)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_gateepoch.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationGateEpoch"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationGateEpoch<OldName minus Test>`, e.g. `TestCompleteEpochOnlyFromCompleting` → `TestIntegrationGateEpochCompleteEpochOnlyFromCompleting`. Race-classified tests become `TestRaceIntegrationAppConcurrency<OldName minus Test>` and join the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 36 offenders: the run-gate epoch record, gate store and launch-gate tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Task-specific note.** `rungate_store_test.go` and `rungate_epoch_test.go` very likely hold fixtures that other still-default files use (for example gate-drive or status tests). Step 4 is where you find out. Move those helpers into `internal/app/rungate_helpers_test.go` (untagged) rather than leave them behind the tag.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `rungate_epoch_test.go` (all 18): `TestCompleteEpochOnlyFromCompleting`, `TestConfirmGateClaimBindsEpochChange`, `TestConfirmGateClaimNoEpochIsNoop`, `TestEpochRecordCRUD`, `TestEpochSettledResolverStates`, `TestEpochUnknownSchemaFailsClosed`, `TestFenceEpochCompletingFromActive`, `TestFenceEpochCompletingNeverRelabelsTerminalStates`, `TestFenceEpochCompletingRejectsStaleLocator`, `TestGateBeforeMintsEpoch`, `TestLoadEpochNotFound`, `TestNoAdapterReportsLifecycleUnavailable`, `TestRecordEpochParticipantTerminal`, `TestRecordEpochParticipantTerminalAllowedAfterFence`, `TestRecordEpochParticipantTerminalUnknownHandleAndBadInput`, `TestRegisterEpochParticipantRejectedOnCompletingAndCompleted`, `TestRegisterParticipantRejectsNonActive`, `TestSupersedeRefusesCompletingAndCompleted`
- `rungate_store_test.go` (all 11): `TestConfirmGateClaimMirrorsRecord`, `TestConfirmWithoutReservationFails`, `TestConsumeGateRetryLimitOne`, `TestConsumeGateRetryPerAttemptCAS`, `TestFindGateRecordByContextHash`, `TestGateRetryUsageCountsLegacyMarker`, `TestGateSchemaV2RecordFailsClosed`, `TestLoadGateClaimBindingCorruptFailsClosed`, `TestLoadGateRecordRefusesV3`, `TestReserveGateClaimIsBindOnce`, `TestSaveGateRecordRefusesUnstampedLimit`
- `rungate_gate_test.go` (all 7): `TestEpochLaunchGateAdmitsActiveBoundEpoch`, `TestEpochLaunchGatePerformsNoWrite`, `TestEpochLaunchGateRefusalMatrix`, `TestEpochLaunchGateRefusesCompletingAndCompleted`, `TestEpochLaunchGateSerializesWithFence`, `TestEpochRevokedResolverRevokesCompletingAndCompleted`, `TestFindEpochDirByID`

**Race candidates, pre-classified at plan time** (their bodies start goroutines or hold two live launches at once; confirm against the criterion in Step 2): `TestEpochLaunchGateSerializesWithFence`.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task7.txt"
grep -E -e "^internal/app/(rungate_epoch|rungate_store|rungate_gate)_test\.go " "${TMPDIR:-/tmp}/census-task7.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/rungate_epoch_test.go internal/app/rungate_epoch_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_epoch_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_epoch_integration_test.go
git mv internal/app/rungate_store_test.go internal/app/rungate_store_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_store_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_store_integration_test.go
git mv internal/app/rungate_gate_test.go internal/app/rungate_gate_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_gate_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_gate_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationGateEpoch"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_gateepoch.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_gateepoch.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_gateepoch.sh — Go integration shard (change 0465, extending change
# 0333's partition): the run-gate epoch record, gate store and launch-gate tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationGateEpoch. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationGateEpoch"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/rungate_epoch_test.go|internal/app/rungate_store_test.go|internal/app/rungate_gate_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_gateepoch.sh; echo "shard rc=$?"
bash tests/test_go_integration_app_concurrency.sh; echo "race shard rc=$?"   # only if a test was race-classified
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0` (and `race shard rc=0` when used).
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_gateepoch.sh' 2>&1 | tail -4
bash -c 'time bash tests/test_go_integration_app_concurrency.sh' 2>&1 | tail -4   # only if a test was race-classified
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_gateepoch.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes. If a race test joined `tests/test_go_integration_app_concurrency.sh`, re-measure it the same way and raise its row only if the new measurement exceeds it.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_gateepoch.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move run-gate epoch and store real-git tests behind the integration tag (TestIntegrationGateEpoch, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 8: Move the run-gate arm, resume and claim real-git tests into the `TestIntegrationGateArm` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/rungate_before_resume_test.go` → `internal/app/rungate_before_resume_integration_test.go`
- Move (whole file, `git mv`): `internal/app/rungate_before_test.go` → `internal/app/rungate_before_integration_test.go`
- Move (whole file, `git mv`): `internal/app/rungate_claim_test.go` → `internal/app/rungate_claim_integration_test.go`
- Create: `tests/test_go_integration_app_gatearm.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_gatearm.sh`; re-measure the `tests/test_go_integration_app_concurrency.sh` row if a test is race-classified)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_gatearm.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationGateArm"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationGateArm<OldName minus Test>`, e.g. `TestRepeatArmObservesReservation` → `TestIntegrationGateArmRepeatArmObservesReservation`. Race-classified tests become `TestRaceIntegrationAppConcurrency<OldName minus Test>` and join the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 31 offenders: the run-gate arm (gate-before), resume, and gate-claim tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `rungate_before_resume_test.go` (all 14): `TestRepeatArmObservesReservation`, `TestResumeAfterCancelledSupersedesOnce`, `TestResumeCancellingIsPending`, `TestResumeDeniedWhileOldEpochNotQuiescent`, `TestResumeDoesNotResetSuiteBudget`, `TestResumeForeignSlotIsNeutral`, `TestResumeRefusesActiveEpochWithLocator`, `TestResumeRefusesCompletedEpochWithoutSuperseding`, `TestResumeRefusesCompletingEpochWithoutSuperseding`, `TestResumeRetiresStaleSlotThenReservesOnce`, `TestResumeSupersededBranchAccountsScopeLinkedDrives`, `TestResumeSupersededChecksReplacementSlot`, `TestResumeSupersededValidatesBeforeObserve`, `TestResumeTornReplacementConverges`
- `rungate_before_test.go` (all 7): `TestGateBeforeFreshArmSurfacesRunEpoch`, `TestGateBeforeNoTimestampGames`, `TestGateBeforePreparesOuterScope`, `TestGateBeforeResumeBindsOnlyVerifiedInProgress`, `TestGateRecordContinuationTripleRule`, `TestGateRecordSchema1FailsClosed`, `TestMintSnapshotsRunMaxAttempts`
- `rungate_claim_test.go` (all 10): `TestGateClaimCommandError`, `TestGateClaimHaltedCarriesCause`, `TestGateClaimLoadErrorFailsClosed`, `TestGateClaimMismatch`, `TestGateClaimMismatchDifferentLength`, `TestGateClaimNilSeam`, `TestGateClaimNoContinuation`, `TestGateClaimRedactsGeneration`, `TestGateClaimSingleUse`, `TestGateClaimSuccessRedeemsAndClearsTriple`

**Race candidates, pre-classified at plan time** (their bodies start goroutines or hold two live launches at once; confirm against the criterion in Step 2): `TestResumeAfterCancelledSupersedesOnce`.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task8.txt"
grep -E -e "^internal/app/(rungate_before_resume|rungate_before|rungate_claim)_test\.go " "${TMPDIR:-/tmp}/census-task8.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/rungate_before_resume_test.go internal/app/rungate_before_resume_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_before_resume_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_before_resume_integration_test.go
git mv internal/app/rungate_before_test.go internal/app/rungate_before_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_before_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_before_integration_test.go
git mv internal/app/rungate_claim_test.go internal/app/rungate_claim_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/rungate_claim_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/rungate_claim_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationGateArm"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_gatearm.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_gatearm.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_gatearm.sh — Go integration shard (change 0465, extending change
# 0333's partition): the run-gate arm (gate-before), resume, and gate-claim tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationGateArm. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationGateArm"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/rungate_before_resume_test.go|internal/app/rungate_before_test.go|internal/app/rungate_claim_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_gatearm.sh; echo "shard rc=$?"
bash tests/test_go_integration_app_concurrency.sh; echo "race shard rc=$?"   # only if a test was race-classified
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0` (and `race shard rc=0` when used).
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_gatearm.sh' 2>&1 | tail -4
bash -c 'time bash tests/test_go_integration_app_concurrency.sh' 2>&1 | tail -4   # only if a test was race-classified
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_gatearm.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes. If a race test joined `tests/test_go_integration_app_concurrency.sh`, re-measure it the same way and raise its row only if the new measurement exceeds it.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_gatearm.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move run-gate arm, resume and claim real-git tests behind the integration tag (TestIntegrationGateArm, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 9: Move the gate launch lifecycle and guardian real-git tests into the `TestIntegrationGateLifecycle` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/agent_guardian_test.go` → `internal/app/agent_guardian_integration_test.go`
- Split (only the 7 listed tests of 11): `internal/app/gate_test.go` → new `internal/app/gate_integration_test.go`
- Create: `tests/test_go_integration_app_gatelifecycle.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_gatelifecycle.sh`; re-measure the `tests/test_go_integration_app_concurrency.sh` row if a test is race-classified)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_gatelifecycle.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationGateLifecycle"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationGateLifecycle<OldName minus Test>`, e.g. `TestGuardianCannotMutate` → `TestIntegrationGateLifecycleGuardianCannotMutate`. Race-classified tests become `TestRaceIntegrationAppConcurrency<OldName minus Test>` and join the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 12 offenders: the real-process gate launch/stop lifecycle and death-guardian tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Task-specific constraint.** `TestMain` (and its supervisor/guardian re-exec routing plus the Task 1 `installNoGitGuard` call) **stays in the untagged `internal/app/gate_test.go`**, because both builds need it. The moved guardian tests re-exec the test binary as a detached guardian, and they rely on that routing in the integration build too. Only the seven listed test functions leave `gate_test.go`.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `agent_guardian_test.go` (all 5): `TestGuardianCannotMutate`, `TestGuardianCompletionMarkerPreventsCancel`, `TestGuardianEOFFencesEpoch`, `TestGuardianLeavesCompletingEpochForReplay`, `TestGuardianStaleMarkerDoesNotSuppressFence`
- `gate_test.go` (7 of 11; the rest stay in the default file): `TestGateLaunchInsideWorktreeReservesSlot`, `TestGateLaunchInvalidInput`, `TestGateLaunchLegacyInventoryRefusalNamesMatchedDrive`, `TestGateLaunchOutsideGitUnchanged`, `TestGateLaunchSecondRefusedWhileFirstLives`, `TestGateLaunchSettlesFinishedRawIncumbent`, `TestGateStopReleasesRawSlot` (`TestGateLaunchInvalidInput` and `TestGateLaunchOutsideGitUnchanged` *pass* under the guard, because they tolerate the git failure. They appear only in the guard's attempt listing, never as `--- FAIL`, and they still move.)

**Race candidates, pre-classified at plan time** (their bodies start goroutines or hold two live launches at once; confirm against the criterion in Step 2): `TestGateLaunchSecondRefusedWhileFirstLives`.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task9.txt"
grep -E -e "^internal/app/(gate|agent_guardian)_test\.go " "${TMPDIR:-/tmp}/census-task9.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/agent_guardian_test.go internal/app/agent_guardian_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/agent_guardian_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/agent_guardian_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

Split files. Create each new file with this header, then **cut** each listed test function, together with its doc comment, out of the source file and paste it below the header. The file's non-`Test` helpers stay where they are:

```go
//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_gatelifecycle.sh (prefix ^TestIntegrationGateLifecycle).

import (
	// exactly the imports the moved functions use; `go vet` in Step 4 names any gap
)
```
- `internal/app/gate_test.go` → `internal/app/gate_integration_test.go`: 7 functions (listed above).

There is no goimports on this machine. Fix imports by hand from the compiler's `imported and not used` / `undefined:` messages in both the source and the new file.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationGateLifecycle"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_gatelifecycle.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_gatelifecycle.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_gatelifecycle.sh — Go integration shard (change 0465, extending change
# 0333's partition): the real-process gate launch/stop lifecycle and death-guardian tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationGateLifecycle. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationGateLifecycle"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/gate_test.go|internal/app/agent_guardian_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_gatelifecycle.sh; echo "shard rc=$?"
bash tests/test_go_integration_app_concurrency.sh; echo "race shard rc=$?"   # only if a test was race-classified
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0` (and `race shard rc=0` when used).
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_gatelifecycle.sh' 2>&1 | tail -4
bash -c 'time bash tests/test_go_integration_app_concurrency.sh' 2>&1 | tail -4   # only if a test was race-classified
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_gatelifecycle.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes. If a race test joined `tests/test_go_integration_app_concurrency.sh`, re-measure it the same way and raise its row only if the new measurement exceeds it.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_gatelifecycle.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move gate launch lifecycle and guardian real-git tests behind the integration tag (TestIntegrationGateLifecycle, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 10: Move the finalize operations real-git tests into the `TestIntegrationFinalizeOps` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/finalize_reserve_test.go` → `internal/app/finalize_reserve_integration_test.go`
- Move (whole file, `git mv`): `internal/app/finalize_publish_test.go` → `internal/app/finalize_publish_integration_test.go`
- Split (only the 23 listed tests of 29): `internal/app/finalize_rebase_test.go` → new `internal/app/finalize_rebase_ops_integration_test.go`
- Split (only the 3 listed tests of 5): `internal/app/finalize_block_test.go` → new `internal/app/finalize_block_integration_test.go`
- Split (only the 2 listed tests of 3): `internal/app/pr_publish_test.go` → new `internal/app/pr_publish_integration_test.go`
- Create: `tests/test_go_integration_app_finalizeops.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_finalizeops.sh`; re-measure the `tests/test_go_integration_app_concurrency.sh` row if a test is race-classified)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_finalizeops.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationFinalizeOps"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationFinalizeOps<OldName minus Test>`, e.g. `TestFinalizeResolverReserveExhausted` → `TestIntegrationFinalizeOpsFinalizeResolverReserveExhausted`. Race-classified tests become `TestRaceIntegrationAppConcurrency<OldName minus Test>` and join the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 39 offenders: the finalize rebase-continue, resolver-reserve, block, publish, and PR-publish tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `finalize_reserve_test.go` (all 9): `TestFinalizeResolverReserveConcurrent`, `TestFinalizeResolverReserveExhausted`, `TestFinalizeResolverReserveForeignAttempt`, `TestFinalizeResolverReserveLegacy`, `TestFinalizeResolverReserveNonConflicted`, `TestFinalizeResolverReservePending`, `TestFinalizeResolverReserveReserved`, `TestFinalizeResolverReserveStoppedProbeError`, `TestFinalizeResolverReserveWriteFailureNoAdmission`
- `finalize_publish_test.go` (all 2): `TestFinalizePublishAcceptsSkippedEvidence`, `TestFinalizePublishAfterCheckpointResume`
- `finalize_rebase_test.go` (23 of 29; the rest stay in the default file): `TestFinalizeRebaseContinueLegacyRefuses`, `TestFinalizeRebaseContinueMarkerWriteFailureNoRecoveryClaim`, `TestFinalizeRebaseContinueMarksStartedBeforeStaging`, `TestFinalizeRebaseContinueNextConflictExhausted`, `TestFinalizeRebaseContinueNextConflictUnderBudget`, `TestFinalizeRebaseContinueReconcileWriteFailureNextConflict`, `TestFinalizeRebaseContinueReconcileWriteFailurePreserves`, `TestFinalizeRebaseContinueRepeatedConsumedReservation`, `TestFinalizeRebaseContinueReservationMissing`, `TestFinalizeRebaseContinueReservationStaleCommit`, `TestFinalizeRebaseContinueReservationStaleToken`, `TestFinalizeRebaseContinueStartedAdvancedRecoveryWriteFails`, `TestFinalizeRebaseContinueStartedAmbiguousRetains`, `TestFinalizeRebaseContinueStartedCompletedRecovers`, `TestFinalizeRebaseContinueStartedCompletedRecoveryWriteFails`, `TestFinalizeRebaseGateHaltCarriesAdmissionRefusal`, `TestFinalizeRebaseGateHaltGenericUnchanged`, `TestFinalizeRebaseGateOffCreatesNoReceipt`, `TestFinalizeRebaseResolverBudgetClearReloadsForward`, `TestFinalizeRebaseResolverBudgetRecoveryNoResnapshot`, `TestFinalizeRebaseResolverBudgetSnapshot`, `TestFinalizeRebaseResolverBudgetWaitingReloadsForward`, `TestMutateReceiptForAttemptSkipsSuperseded`
- `finalize_block_test.go` (3 of 5; the rest stay in the default file): `TestFinalizeBlockUnrelatedInvalidRecordProgress`, `TestFinalizeBlockUnrelatedInvalidRecordRefusals`, `TestFinalizeClearBlockUnrelatedInvalidRecordProgress`
- `pr_publish_test.go` (2 of 3; the rest stay in the default file): `TestPRPublishAcceptsSkippedEvidenceAtExactHead`, `TestPRPublishPreEffectValidationIsScoped`

**Race candidates, pre-classified at plan time** (their bodies start goroutines or hold two live launches at once; confirm against the criterion in Step 2): `TestFinalizeResolverReserveConcurrent`.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task10.txt"
grep -E -e "^internal/app/(finalize_rebase|finalize_reserve|finalize_block|finalize_publish|pr_publish)_test\.go " "${TMPDIR:-/tmp}/census-task10.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/finalize_reserve_test.go internal/app/finalize_reserve_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/finalize_reserve_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/finalize_reserve_integration_test.go
git mv internal/app/finalize_publish_test.go internal/app/finalize_publish_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/finalize_publish_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/finalize_publish_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

Split files. Create each new file with this header, then **cut** each listed test function, together with its doc comment, out of the source file and paste it below the header. The file's non-`Test` helpers stay where they are:

```go
//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_finalizeops.sh (prefix ^TestIntegrationFinalizeOps).

import (
	// exactly the imports the moved functions use; `go vet` in Step 4 names any gap
)
```
- `internal/app/finalize_rebase_test.go` → `internal/app/finalize_rebase_ops_integration_test.go`: 23 functions (listed above).
- `internal/app/finalize_block_test.go` → `internal/app/finalize_block_integration_test.go`: 3 functions (listed above).
- `internal/app/pr_publish_test.go` → `internal/app/pr_publish_integration_test.go`: 2 functions (listed above).

There is no goimports on this machine. Fix imports by hand from the compiler's `imported and not used` / `undefined:` messages in both the source and the new file.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationFinalizeOps"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_finalizeops.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_finalizeops.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_finalizeops.sh — Go integration shard (change 0465, extending change
# 0333's partition): the finalize rebase-continue, resolver-reserve, block, publish, and PR-publish tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationFinalizeOps. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationFinalizeOps"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/finalize_rebase_test.go|internal/app/finalize_reserve_test.go|internal/app/finalize_block_test.go|internal/app/finalize_publish_test.go|internal/app/pr_publish_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_finalizeops.sh; echo "shard rc=$?"
bash tests/test_go_integration_app_concurrency.sh; echo "race shard rc=$?"   # only if a test was race-classified
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0` (and `race shard rc=0` when used).
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_finalizeops.sh' 2>&1 | tail -4
bash -c 'time bash tests/test_go_integration_app_concurrency.sh' 2>&1 | tail -4   # only if a test was race-classified
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_finalizeops.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes. If a race test joined `tests/test_go_integration_app_concurrency.sh`, re-measure it the same way and raise its row only if the new measurement exceeds it.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_finalizeops.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move finalize operations real-git tests behind the integration tag (TestIntegrationFinalizeOps, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 11: Move the evidence and run verify real-git tests into the `TestIntegrationEvidence` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/evidence_recertify_test.go` → `internal/app/evidence_recertify_integration_test.go`
- Split (only the 3 listed tests of 6): `internal/app/evidence_ops_test.go` → new `internal/app/evidence_ops_integration_test.go`
- Split (only the 4 listed tests of 8): `internal/app/run_verify_test.go` → new `internal/app/run_verify_integration_test.go`
- Create: `tests/test_go_integration_app_evidence.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_evidence.sh`)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_evidence.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationEvidence"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationEvidence<OldName minus Test>`, e.g. `TestBuildLocalGateFailsClosedWithoutBuildCommand` → `TestIntegrationEvidenceBuildLocalGateFailsClosedWithoutBuildCommand`. If Step 2 race-classifies a test anyway, it becomes `TestRaceIntegrationAppConcurrency<OldName minus Test>` and joins the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 26 offenders: the evidence record/recertify and run-verify tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `evidence_recertify_test.go` (all 19): `TestBuildLocalGateFailsClosedWithoutBuildCommand`, `TestBuildLocalGateResolvesBuildCommandOnly`, `TestEvidenceRecertifyAdvancesOneDriveAcrossWaiting`, `TestEvidenceRecertifyEditContended`, `TestEvidenceRecertifyEditFailureThenRetry`, `TestEvidenceRecertifyEditUnrecognizedDisposition`, `TestEvidenceRecertifyGateFailureAndHalt`, `TestEvidenceRecertifyGateOffRecordsSkipped`, `TestEvidenceRecertifyHappyPath`, `TestEvidenceRecertifyRefusesClosedOrMismatchedPR`, `TestEvidenceRecertifyRefusesDirtyAfterGate`, `TestEvidenceRecertifyRefusesDirtyWorkspace`, `TestEvidenceRecertifyRefusesForeignCommandEvidence`, `TestEvidenceRecertifyRefusesHeadMovedUnderGate`, `TestEvidenceRecertifyRefusesNotImplemented`, `TestEvidenceRecertifyRefusesUnconfiguredGate`, `TestEvidenceRecertifyRefusesUnpublishedFollowUp`, `TestEvidenceRecertifyRefusesWrongHeadEvidence`, `TestEvidenceRecertifyShape`
- `evidence_ops_test.go` (3 of 6; the rest stay in the default file): `TestEvidenceRecordBuildGateOffMintsSkipped`, `TestEvidenceRecordRecordsBuildCommandNotFinalize`, `TestEvidenceRecordUnconfiguredBuildCommandIsTypedSetupRefusal`
- `run_verify_test.go` (4 of 8; the rest stay in the default file): `TestRunVerifyAcceptsSkippedEvidenceAtExactHead`, `TestRunVerifyInvalidResultsContentIsUnmetConjunct`, `TestRunVerifyMissingResultsIsUnmetConjunct`, `TestRunVerifyWaitingSurvivesMissingResults`

**Race candidates:** none were pre-classified. Apply the Step 2 criterion anyway.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task11.txt"
grep -E -e "^internal/app/(evidence_recertify|evidence_ops|run_verify)_test\.go " "${TMPDIR:-/tmp}/census-task11.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/evidence_recertify_test.go internal/app/evidence_recertify_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/evidence_recertify_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/evidence_recertify_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

Split files. Create each new file with this header, then **cut** each listed test function, together with its doc comment, out of the source file and paste it below the header. The file's non-`Test` helpers stay where they are:

```go
//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_evidence.sh (prefix ^TestIntegrationEvidence).

import (
	// exactly the imports the moved functions use; `go vet` in Step 4 names any gap
)
```
- `internal/app/evidence_ops_test.go` → `internal/app/evidence_ops_integration_test.go`: 3 functions (listed above).
- `internal/app/run_verify_test.go` → `internal/app/run_verify_integration_test.go`: 4 functions (listed above).

There is no goimports on this machine. Fix imports by hand from the compiler's `imported and not used` / `undefined:` messages in both the source and the new file.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationEvidence"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_evidence.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_evidence.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_evidence.sh — Go integration shard (change 0465, extending change
# 0333's partition): the evidence record/recertify and run-verify tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationEvidence. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationEvidence"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/evidence_recertify_test.go|internal/app/evidence_ops_test.go|internal/app/run_verify_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_evidence.sh; echo "shard rc=$?"
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0`.
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_evidence.sh' 2>&1 | tail -4
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_evidence.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_evidence.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move evidence and run verify real-git tests behind the integration tag (TestIntegrationEvidence, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 12: Move the change and ADR record operations real-git tests into the `TestIntegrationRecordOps` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/change_attach_git_test.go` → `internal/app/change_attach_git_integration_test.go`
- Move (whole file, `git mv`): `internal/app/claim_proof_git_test.go` → `internal/app/claim_proof_git_integration_test.go`
- Split (only the 10 listed tests of 14): `internal/app/change_claim_test.go` → new `internal/app/change_claim_integration_test.go`
- Split (only the 4 listed tests of 11): `internal/app/change_halt_test.go` → new `internal/app/change_halt_integration_test.go`
- Split (only the 3 listed tests of 10): `internal/app/change_repair_test.go` → new `internal/app/change_repair_integration_test.go`
- Split (only the 3 listed tests of 4): `internal/app/change_implemented_test.go` → new `internal/app/change_implemented_integration_test.go`
- Split (only the 2 listed tests of 10): `internal/app/change_reconcile_test.go` → new `internal/app/change_reconcile_integration_test.go`
- Split (only the 2 listed tests of 24): `internal/app/change_lifecycle_test.go` → new `internal/app/change_lifecycle_integration_test.go`
- Split (only the 2 listed tests of 40): `internal/app/change_groom_test.go` → new `internal/app/change_groom_integration_test.go`
- Split (only the 1 listed tests of 16): `internal/app/change_create_test.go` → new `internal/app/change_create_integration_test.go`
- Split (only the 3 listed tests of 21): `internal/app/adr_ops_test.go` → new `internal/app/adr_ops_integration_test.go`
- Create: `tests/test_go_integration_app_recordops.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_recordops.sh`)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_recordops.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationRecordOps"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationRecordOps<OldName minus Test>`, e.g. `TestChangeAttachUnrelatedInvalidRecordProgress` → `TestIntegrationRecordOpsChangeAttachUnrelatedInvalidRecordProgress`. If Step 2 race-classifies a test anyway, it becomes `TestRaceIntegrationAppConcurrency<OldName minus Test>` and joins the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 33 offenders: the change/ADR record-operation tests over real git (unrelated-invalid-record, claim gate-context, and real-git replay families). They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `change_attach_git_test.go` (all 2): `TestChangeAttachUnrelatedInvalidRecordProgress`, `TestChangeAttachUnrelatedInvalidRecordRefusals`
- `claim_proof_git_test.go` (all 1): `TestScanClaimProofsReadsCommittedReceipt`
- `change_claim_test.go` (10 of 14; the rest stay in the default file): `TestChangeClaimUnrelatedDependentsOfBrokenProgress`, `TestChangeClaimUnrelatedInvalidRecordProgress`, `TestChangeClaimUnrelatedInvalidRecordRefusals`, `TestChangeClaimUnrelatedShapesProgress`, `TestChangeRefreshClaimUnrelatedInvalidRecordRefusals`, `TestClaimGateContextConflictRefused`, `TestClaimGateContextInvalidRefusesBeforeTransaction`, `TestClaimGateContextReservesAndConfirms`, `TestClaimTerminalGateRefused`, `TestClaimUngatedUnchanged`
- `change_halt_test.go` (4 of 11; the rest stay in the default file): `TestChangeHaltUnrelatedInvalidRecordProgress`, `TestChangeHaltUnrelatedInvalidRecordRefusals`, `TestChangeResumeHaltedUnrelatedInvalidRecordProgress`, `TestChangeResumeHaltedUnrelatedInvalidRecordRefusals`
- `change_repair_test.go` (3 of 10; the rest stay in the default file): `TestRepairAdoptPRHeadAppliesOnMalformedRecordedBranch`, `TestRepairIdentityUnrelatedInvalidRecordProgress`, `TestRepairIdentityUnrelatedInvalidRecordRefusals`
- `change_implemented_test.go` (3 of 4; the rest stay in the default file): `TestMarkImplementedAcceptsSkippedEvidence`, `TestMarkImplementedUnrelatedInvalidRecordProgress`, `TestMarkImplementedUnrelatedInvalidRecordRefusals`
- `change_reconcile_test.go` (2 of 10; the rest stay in the default file): `TestChangeReconcileUnrelatedInvalidRecordProgress`, `TestChangeReconcileUnrelatedInvalidRecordRefusals`
- `change_lifecycle_test.go` (2 of 24; the rest stay in the default file): `TestChangeLifecycleUnrelatedInvalidRecordProgress`, `TestChangeLifecycleUnrelatedInvalidRecordRefusals`
- `change_groom_test.go` (2 of 40; the rest stay in the default file): `TestChangeGroomAbstainThenRearmRealGit`, `TestChangeGroomReviseSpecVersionContendsRealGit`
- `change_create_test.go` (1 of 16; the rest stay in the default file): `TestChangeCreateNormalizedPrefixReplaysRealGit`
- `adr_ops_test.go` (3 of 21; the rest stay in the default file): `TestADRRecordIndexSurfacesUnparseableUnrelatedADR`, `TestADRUnrelatedInvalidRecordProgress`, `TestADRUnrelatedInvalidRecordRefusals`

**Race candidates:** none were pre-classified. Apply the Step 2 criterion anyway.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task12.txt"
grep -E -e "^internal/app/(change_claim|change_halt|change_repair|change_implemented|change_reconcile|change_lifecycle|change_groom|change_attach_git|change_create|claim_proof_git|adr_ops)_test\.go " "${TMPDIR:-/tmp}/census-task12.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/change_attach_git_test.go internal/app/change_attach_git_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/change_attach_git_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/change_attach_git_integration_test.go
git mv internal/app/claim_proof_git_test.go internal/app/claim_proof_git_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/claim_proof_git_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/claim_proof_git_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

Split files. Create each new file with this header, then **cut** each listed test function, together with its doc comment, out of the source file and paste it below the header. The file's non-`Test` helpers stay where they are:

```go
//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	// exactly the imports the moved functions use; `go vet` in Step 4 names any gap
)
```
- `internal/app/change_claim_test.go` → `internal/app/change_claim_integration_test.go`: 10 functions (listed above).
- `internal/app/change_halt_test.go` → `internal/app/change_halt_integration_test.go`: 4 functions (listed above).
- `internal/app/change_repair_test.go` → `internal/app/change_repair_integration_test.go`: 3 functions (listed above).
- `internal/app/change_implemented_test.go` → `internal/app/change_implemented_integration_test.go`: 3 functions (listed above).
- `internal/app/change_reconcile_test.go` → `internal/app/change_reconcile_integration_test.go`: 2 functions (listed above).
- `internal/app/change_lifecycle_test.go` → `internal/app/change_lifecycle_integration_test.go`: 2 functions (listed above).
- `internal/app/change_groom_test.go` → `internal/app/change_groom_integration_test.go`: 2 functions (listed above).
- `internal/app/change_create_test.go` → `internal/app/change_create_integration_test.go`: 1 functions (listed above).
- `internal/app/adr_ops_test.go` → `internal/app/adr_ops_integration_test.go`: 3 functions (listed above).

There is no goimports on this machine. Fix imports by hand from the compiler's `imported and not used` / `undefined:` messages in both the source and the new file.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationRecordOps"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_recordops.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_recordops.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_recordops.sh — Go integration shard (change 0465, extending change
# 0333's partition): the change/ADR record-operation tests over real git (unrelated-invalid-record, claim gate-context, and real-git replay families) — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationRecordOps. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRecordOps"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/change_claim_test.go|internal/app/change_halt_test.go|internal/app/change_repair_test.go|internal/app/change_implemented_test.go|internal/app/change_reconcile_test.go|internal/app/change_lifecycle_test.go|internal/app/change_groom_test.go|internal/app/change_attach_git_test.go|internal/app/change_create_test.go|internal/app/claim_proof_git_test.go|internal/app/adr_ops_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_recordops.sh; echo "shard rc=$?"
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0`.
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_recordops.sh' 2>&1 | tail -4
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_recordops.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_recordops.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move change and ADR record operations real-git tests behind the integration tag (TestIntegrationRecordOps, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---

### Task 13: Move the repo phase, operational context and sweep session real-git tests into the `TestIntegrationContextProbe` shard

**Files:**
- Move (whole file, `git mv`): `internal/app/repophase_test.go` → `internal/app/repophase_integration_test.go`
- Move (whole file, `git mv`): `internal/app/operational_context_test.go` → `internal/app/operational_context_integration_test.go`
- Move (whole file, `git mv`): `internal/app/sweep_session_test.go` → `internal/app/sweep_session_integration_test.go`
- Split (only the 4 listed tests of 8): `internal/app/named_branch_facts_test.go` → new `internal/app/named_branch_facts_integration_test.go`
- Create: `tests/test_go_integration_app_contextprobe.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row for `tests/test_go_integration_app_contextprobe.sh`)
- Possibly create: `internal/app/<base>_helpers_test.go` (untagged). Only when a helper that a still-default file uses would otherwise end up behind the tag.
- Modify: any maintained file the Step 5 reference grep finds

**Interfaces:**
- Consumes: Task 1's default-build guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`). Every default-build real-git exec fails with stderr `docket nogit guard: default internal/app tests must not run real git`, and the package prints a `real-git exec attempt(s) reached the guard shim` summary. Also the shard executor `tests/lib/go-integration-shard.sh` and the contract `tests/test_go_integration_contract.sh`, used unchanged.
- Produces: runner `tests/test_go_integration_app_contextprobe.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationContextProbe"`, `SHARD_MODE="normal"`). Every moved normal test is renamed `TestIntegrationContextProbe<OldName minus Test>`, e.g. `TestResolveRepoPhaseAbsentKeyNotAuthorized` → `TestIntegrationContextProbeResolveRepoPhaseAbsentKeyNotAuthorized`. If Step 2 race-classifies a test anyway, it becomes `TestRaceIntegrationAppConcurrency<OldName minus Test>` and joins the existing race shard `tests/test_go_integration_app_concurrency.sh`.

**Context.** Task 1 made the default `internal/app` corpus refuse real git. The package is **red** until Tasks 3–13 have all landed, and that is expected. This task removes exactly its own 23 offenders: the repo-phase resolution, operational-context, named-branch-facts, and sweep-session tests. They move behind `//go:build integration` into a new plain (non-race) shard, following change 0333's partition. Do not touch other tasks' offenders.

**Task-specific note.** This is the last move task. After it, Step 7's census should report `guard: no attempts` and no `--- FAIL` at all for `./internal/app/`. If anything remains, name it in your report. Task 15 re-proves this, but a leftover here means an earlier task's census was incomplete.

**Offenders in this task (census snapshot taken at plan time on `3f9813fbc` with a refusing git shim; the live census in Step 1 is authoritative):**

- `repophase_test.go` (all 9): `TestResolveRepoPhaseAbsentKeyNotAuthorized`, `TestResolveRepoPhaseAgentsTableAloneNotAuthorized`, `TestResolveRepoPhaseDiscoversFromRootAndNestedDir`, `TestResolveRepoPhaseGlobalLayerNotAuthorized`, `TestResolveRepoPhaseInvalidExplicitRepoDir`, `TestResolveRepoPhaseOutsideGitIsMachineOnly`, `TestResolveRepoPhaseRetiresDroppedClaudeLink`, `TestResolveRepoPhaseScopedHarnessCarriesUnrelatedRecord`, `TestResolveRepoPhaseToleratesUnknownKeys`
- `operational_context_test.go` (all 5): `TestFailClosedOrdering`, `TestOperationalGateFindingIsTheClassifierValue`, `TestOperationalGatePassesHealthy`, `TestOperationalGateRefusesLegacy`, `TestStatusInvalidConfigDiagnostics`
- `sweep_session_test.go` (all 5): `TestBoundReaderNeverFetches`, `TestPrepareFailedFetchIsErrorNeverStaleFallback`, `TestPrepareIsOneMetadataFetchZeroSetupProbes`, `TestPrepareObservesFreshMetadataTip`, `TestSessionRefusesDifferentRepository`
- `named_branch_facts_test.go` (4 of 8; the rest stay in the default file): `TestChangeClaimProbesOnlyOwnStack`, `TestFinalizeClearBlockProbesOnlyOwnStack`, `TestMergeContextProbesOnlyOwnStack`, `TestWorkspaceContextProbesOnlyOwnStack`
  - In `repophase_test.go`, `TestResolveRepoPhaseInvalidExplicitRepoDir` and `TestResolveRepoPhaseOutsideGitIsMachineOnly` *pass* under the guard, because they tolerate the git failure. They appear only in the guard's attempt listing, never as `--- FAIL`, and they still move.

**Race candidates:** none were pre-classified. Apply the Step 2 criterion anyway.

- [ ] **Step 1: Confirm this task's live offenders from the guard's census**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; printf "%s %s\n" "$f" "$t"; done > "${TMPDIR:-/tmp}/census-task13.txt"
grep -E -e "^internal/app/(repophase|operational_context|named_branch_facts|sweep_session)_test\.go " "${TMPDIR:-/tmp}/census-task13.txt" | LC_ALL=C sort
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out"
'
```
Expected: the listed `--- FAIL` offenders for this task's files match the snapshot above. If the live census differs, the live census wins. Move every live offender in these files (a test added since the snapshot included), leave a snapshot name that no longer fails in place, and report the difference. Write down the attempt count from the summary line. Step 7 must show it smaller.

- [ ] **Step 2: Classify each offender as normal or race**

A test is **race** only if its body (or a helper it calls for the scenario) starts goroutines (`go func`/`go f(`), coordinates simultaneous operations with `sync.WaitGroup`, errgroup or channels, or holds two live processes or launches against shared state at once. Everything else, including sequential "replay"/"idempotent" tests, is **normal**. Race tests keep 0333's rule that race instrumentation is only for tests exercising real concurrency. Each one gets a one-line comment directly above its `func`: `// Race shard (change 0465): <what runs concurrently>.` List the classification in your report.

- [ ] **Step 3: Move the tests behind the tag**

Whole files (every test in the file is an offender):

```bash
bash -c '
set -euo pipefail
git mv internal/app/repophase_test.go internal/app/repophase_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/repophase_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/repophase_integration_test.go
git mv internal/app/operational_context_test.go internal/app/operational_context_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/operational_context_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/operational_context_integration_test.go
git mv internal/app/sweep_session_test.go internal/app/sweep_session_integration_test.go
tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat internal/app/sweep_session_integration_test.go; } > "$tmp" && mv -f "$tmp" internal/app/sweep_session_integration_test.go
'
```
Then check each moved file: line 1 is `//go:build integration` and line 2 is blank (`sed -n '1,3p' <file>`). If the original file opened with its own `//go:build` line, merge the constraints into one line-1 constraint instead of stacking two.

Split files. Create each new file with this header, then **cut** each listed test function, together with its doc comment, out of the source file and paste it below the header. The file's non-`Test` helpers stay where they are:

```go
//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_contextprobe.sh (prefix ^TestIntegrationContextProbe).

import (
	// exactly the imports the moved functions use; `go vet` in Step 4 names any gap
)
```
- `internal/app/named_branch_facts_test.go` → `internal/app/named_branch_facts_integration_test.go`: 4 functions (listed above).

There is no goimports on this machine. Fix imports by hand from the compiler's `imported and not used` / `undefined:` messages in both the source and the new file.

- [ ] **Step 4: Keep every build compiling (helpers stay reachable)**

An untagged `_test.go` file compiles into **every** build, and a tagged one only into `-tags integration`. So any non-`Test` identifier (func, type, var, const) now in a tagged file that an untagged or `e2e` file still references must go back into an untagged file. Put it in `internal/app/<base>_helpers_test.go`, where `<base>` is the source file's base name, with no build constraint. Iterate until all three builds are clean:

```bash
gofmt -l internal/app
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
```
Expected: `gofmt -l` prints nothing and all three vets exit 0.

- [ ] **Step 5: Rename the moved tests and every maintained reference**

Derive references with a whole-repo grep, never a hand list. Point-in-time records keep the old names. Put the normal-classified names in `NORMAL` and the race-classified ones in `RACE`, both as space-separated old names:

```bash
bash -c '
set -uo pipefail
PREFIX="TestIntegrationContextProbe"
NORMAL="<space-separated old names classified normal>"
RACE="<space-separated old names classified race, or empty>"
rename(){ old="$1"; new="$2"
  hits="$(git grep -l -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep -n -w -e "$old" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs")"
  [ -z "$left" ] || { printf "STILL REFERENCED %s:\n%s\n" "$old" "$left"; exit 1; }; }
for old in $NORMAL; do rename "$old" "${PREFIX}${old#Test}"; done
for old in $RACE; do rename "$old" "TestRaceIntegrationAppConcurrency${old#Test}"; done
'
```
Expected: exit 0 and no `STILL REFERENCED` lines. The `\b…\b` word boundary keeps `TestFoo` from rewriting `TestFooBar`. Read `git diff --stat` and inspect every file touched **outside** `internal/app/`. A comment or doc that described the test as a default-corpus test gets its wording corrected, not only its name.

- [ ] **Step 6: Create the shard runner**

Create `tests/test_go_integration_app_contextprobe.sh` with exactly this content, then `chmod +x tests/test_go_integration_app_contextprobe.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_contextprobe.sh — Go integration shard (change 0465, extending change
# 0333's partition): the repo-phase resolution, operational-context, named-branch-facts, and sweep-session tests — real-git tests moved out of the
# default internal/app corpus, which must never start real git (the no-real-git guard
# in internal/app/nogit_guard_test.go) — behind the `integration` build tag, prefix
# ^TestIntegrationContextProbe. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationContextProbe"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 7: Verify: offenders gone, shard green, contract green, no prefix collision**

```bash
bash -c '
census_out="$(go test -count=1 -v ./internal/app/ 2>&1)"
fails="$(grep -E -e "^--- FAIL: " <<<"$census_out" | awk "{print \$3}" | LC_ALL=C sort -u)"
mine=""; for t in $fails; do f="$(grep -l -E -e "^func ${t}\(" internal/app/*_test.go)"; case "$f" in internal/app/repophase_test.go|internal/app/operational_context_test.go|internal/app/named_branch_facts_test.go|internal/app/sweep_session_test.go) mine="$mine $t";; esac; done
printf "remaining offenders in this task files:%s\n" "${mine:- none}"
grep -E -e "real-git exec attempt\(s\) reached the guard shim" <<<"$census_out" || echo "guard: no attempts"
leak="$(go test -list "^Test(Race)?Integration" ./internal/app/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"
'
bash tests/test_go_integration_app_contextprobe.sh; echo "shard rc=$?"
bash tests/test_go_integration_contract.sh; echo "contract rc=$?"
bash -c 'p="$(for r in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" | sed -n "s/^prefix=//p"; done)"; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo prefix-check-done'
```
Expected:
- `remaining offenders in this task files: none`.
- The guard attempt count is lower than in Step 1, or `guard: no attempts` once every task has landed.
- `no leak into the default corpus`.
- Every runner prints only `ok - ` lines, with `shard rc=0` and `contract rc=0`.
- The prefix check prints only `prefix-check-done`.

The default package as a whole stays red until Task 13. That is expected.

- [ ] **Step 8: Measure and register the budget row**

```bash
bash -c 'time bash tests/test_go_integration_app_contextprobe.sh' 2>&1 | tail -4
```
Take the `real` seconds S of the solo run. The row is S rounded up to the next multiple of 5, plus 5, with a minimum of 10. Insert `tests/test_go_integration_app_contextprobe.sh<TAB><row><TAB>parallel` into `tests/runtime-budgets.tsv` directly after the `tests/test_go_integration_app_sync.sh` row, with a literal tab, not spaces. A row above 60 would not fit the parallel lane. Do **not** register it. Return NEEDS_ESCALATION with the measurement, so the shard can be split into two runners with disjoint new prefixes.

Then run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app tests/runtime-budgets.tsv tests/test_go_integration_app_contextprobe.sh   # plus any file outside internal/app that Step 5 rewrote; list it explicitly
git status --porcelain   # expect nothing unstaged or untracked that belongs to this task
git commit -m "test(app): move repo phase, operational context and sweep session real-git tests behind the integration tag (TestIntegrationContextProbe, change 0465)"
```
(`git add -- internal/app` stages this task's renames and splits. Before committing, confirm with `git status --porcelain` that nothing under `internal/app` belongs to another task.)

---
### Task 14: Explicit `-timeout` backstop and readable overrun in `tests/test_go_race.sh`

**Files:**
- Modify: `tests/test_go_race.sh` (header `PARTITION AND LANE` paragraph, a new `BACKSTOP TIMEOUT` paragraph, the `go test -race` invocation, one new assert)
- Create: `internal/repoguard/race_gate_timeout_test.go`

**Interfaces:**
- Consumes: Tasks 3–13 (the post-partition default corpus to measure), and the `internal/repoguard` test helpers `Root()`, `writeToolScript(t, path, body)` and `readLog(t, path)` (defined in `internal/repoguard/gofmt_toolchain_test.go`, same package).
- Produces: `tests/test_go_race.sh` defines `RACE_TIMEOUT="<N>m"` and runs `go test -race $go_conc_args -timeout "$RACE_TIMEOUT" -count=1 ./...`. On overrun it prints the assert line `NOT OK - no package ran past the <N>m -timeout backstop` plus a stderr remedy naming `PARTITION AND LANE`.

The backstop is not a cure. The guard and the budget row detect growth. The timeout only makes an overrun readable, instead of Go's 10m goroutine-dump panic.

- [ ] **Step 1: Measure the post-partition worst package under the CI-equivalent cap**

CI's derived cap on a shared 3-core runner is `-p 2` with `GOMAXPROCS=2`.

```bash
bash -c '
out="$(GOMAXPROCS=2 go test -race -count=1 -p 2 -json ./... 2>/dev/null)"
ranked="$(jq -r "select(.Test==null and (.Action==\"pass\" or .Action==\"fail\")) | \"\(.Elapsed)\t\(.Package)\t\(.Action)\"" <<<"$out" | sort -rn)"
sed -n "1,5p" <<<"$ranked"
'
```
Expected: every package reports `pass`. `github.com/danielhanold/docket/internal/app` should now be well under a minute, down from ~238s at `-p 3`. Let W be the top line's seconds. Compute `RACE_TIMEOUT` as 4 × W rounded **up** to whole minutes, with a minimum of `3m`. If 4 × W exceeds 8 minutes, stop: report NEEDS_ESCALATION with the ranking, because the partition did not bring the gate into range. Record the top five lines and your chosen value in your report.

- [ ] **Step 2: Write the failing behavioral tests**

Create `internal/repoguard/race_gate_timeout_test.go`:

```go
package repoguard

// Change 0465: tests/test_go_race.sh passes an explicit -timeout backstop (below Go's
// 10m per-package default) and turns an overrun into a named, readable NOT OK line
// instead of a bare goroutine-dump panic. These are behavioral tests over a COPY of
// the real wrapper (read at test time, so nothing frozen can drift) with a fake `go`
// on PATH that logs its argv. Same pattern and helpers as gofmt_toolchain_test.go
// (writeToolScript, readLog). The asserts pin the mechanism (the argv go test
// actually received) and also prove the gate is not weakened (-race, -count=1, ./...).

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

const fakeRaceGoScript = `printf 'argv:[%s]\n' "$*" >>"$GO_FAKE_LOG"
if [ "$1" = test ] && [ -n "${FAKE_GO_TIMEOUT:-}" ]; then
  printf 'panic: test timed out after 4m0s\n\ngoroutine 1 [running]:\nFAIL\tfixture/slow\t240.012s\nFAIL\n'
  exit 1
fi
exit 0
`

type raceGateFixture struct {
	root, wrapper, goLog string
	env                  []string
}

func newRaceGateFixture(t *testing.T) *raceGateFixture {
	t.Helper()
	repoRoot, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	realWrapper, err := os.ReadFile(filepath.Join(repoRoot, "tests", "test_go_race.sh"))
	if err != nil {
		t.Fatal(err)
	}
	base := testsupport.TempDir(t)
	f := &raceGateFixture{root: filepath.Join(base, "fixture"), goLog: filepath.Join(base, "go.log")}
	f.wrapper = filepath.Join(f.root, "tests", "test_go_race.sh")
	if err := os.MkdirAll(filepath.Dir(f.wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.wrapper, realWrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(base, "fakebin")
	writeToolScript(t, filepath.Join(fakeBin, "go"), fakeRaceGoScript)
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "PATH", "GOMODCACHE", "GOCACHE", "GOFLAGS", "GOMAXPROCS",
			"DOCKET_GO_TEST_CONCURRENCY", "GO_FAKE_LOG", "FAKE_GO_TIMEOUT":
			continue
		}
		f.env = append(f.env, kv)
	}
	// GOMODCACHE/GOCACHE pre-set so the wrapper's cache block never calls git.
	f.env = append(f.env,
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOMODCACHE="+filepath.Join(base, "gomodcache"),
		"GOCACHE="+filepath.Join(base, "gocache"),
		"GO_FAKE_LOG="+f.goLog,
	)
	return f
}

func (f *raceGateFixture) run(t *testing.T) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", f.wrapper)
	cmd.Dir = f.root
	cmd.Env = f.env
	b, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return string(b), 0
	case errors.As(err, &ee):
		return string(b), ee.ExitCode()
	default:
		t.Fatalf("running the wrapper copy: %v\n%s", err, b)
		return "", -1
	}
}

// raceTestArgv returns the argv of the fake `go test` invocation.
func raceTestArgv(t *testing.T, log string) []string {
	t.Helper()
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "argv:[test ") {
			return strings.Fields(strings.TrimSuffix(strings.TrimPrefix(line, "argv:["), "]"))
		}
	}
	t.Fatalf("the fake go never received a `go test` invocation; log:\n%s", log)
	return nil
}

func TestRaceGatePassesTimeoutBackstopBelowGoDefault(t *testing.T) {
	f := newRaceGateFixture(t)
	out, code := f.run(t)
	if code != 0 {
		t.Fatalf("a green fake run must exit 0, got %d:\n%s", code, out)
	}
	argv := raceTestArgv(t, readLog(t, f.goLog))
	var timeout time.Duration
	found := false
	for i, a := range argv {
		val := ""
		switch {
		case a == "-timeout" && i+1 < len(argv):
			val = argv[i+1]
		case strings.HasPrefix(a, "-timeout="):
			val = strings.TrimPrefix(a, "-timeout=")
		default:
			continue
		}
		d, err := time.ParseDuration(val)
		if err != nil {
			t.Fatalf("-timeout value %q does not parse as a duration: %v", val, err)
		}
		timeout, found = d, true
	}
	if !found {
		t.Fatalf("go test -race must carry an explicit -timeout backstop; argv %q", argv)
	}
	if timeout <= 0 || timeout >= 10*time.Minute {
		t.Fatalf("the backstop %s must be positive and below Go's 10m default", timeout)
	}
	for _, want := range []string{"-race", "-count=1", "./..."} {
		if !slices.Contains(argv, want) {
			t.Fatalf("the race gate must not be weakened: argv %q lacks %q", argv, want)
		}
	}
}

var raceBackstopMarker = regexp.MustCompile(`(?m)^NOT OK - no package ran past the \S+ -timeout backstop$`)

func TestRaceGateNamesTimeoutBackstopOnOverrun(t *testing.T) {
	f := newRaceGateFixture(t)
	f.env = append(f.env, "FAKE_GO_TIMEOUT=1")
	out, code := f.run(t)
	if code == 0 {
		t.Fatalf("an overrun must fail the gate:\n%s", out)
	}
	if !raceBackstopMarker.MatchString(out) {
		t.Fatalf("an overrun must print the named backstop marker, got:\n%s", out)
	}
	for _, want := range []string{"FAIL\tfixture/slow", "PARTITION AND LANE"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the overrun diagnostic must contain %q, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test -count=1 -run 'TestRaceGate' ./internal/repoguard/`
Expected: FAIL. `TestRaceGatePassesTimeoutBackstopBelowGoDefault` fails with "must carry an explicit -timeout backstop", and `TestRaceGateNamesTimeoutBackstopOnOverrun` fails with "must print the named backstop marker".

- [ ] **Step 4: Implement in `tests/test_go_race.sh`**

(a) Replace the whole `# PARTITION AND LANE.` paragraph of the header, from `# PARTITION AND LANE. Change 0333 partitioned` through `# tests/runtime-budgets.tsv like every other file.`, with:

```bash
# PARTITION AND LANE. Change 0333 partitioned the slow real-git, subprocess, and
# process-lifecycle integration corpus of internal/app, internal/githubcli, and
# internal/gitcli behind the `integration` build tag — dedicated shard runners
# (tests/test_go_integration_*.sh) own it, and tests/test_go_integration_contract.sh
# proves that partition is total. This gate therefore covers the FAST default
# corpus only. Change 0465 made that an enforced invariant for internal/app, the
# package whose default corpus had regrown to ~920 tests (337 of them real-git,
# 225s of its 237s under -race): the default-tag internal/app test corpus never
# starts a real `git` process. installNoGitGuard (internal/app/nogit_guard_test.go)
# shadows git on PATH in the default build and fails the package on any attempt,
# so a new real-git test cannot land here unnoticed. With that tail gone, `go test
# -race`'s GOMAXPROCS-wide race workers do not oversubscribe the cores the other
# parallel jobs need (change 0332's reason for the serial lane, and change 0329's
# load-dependent build-gate halt), so this gate rides the PARALLEL lane under an
# ordinary row in tests/runtime-budgets.tsv like every other file.
#
# BACKSTOP TIMEOUT (change 0465). `go test` is given an explicit -timeout
# (RACE_TIMEOUT below): several times the measured post-partition worst package
# under CI's derived cap (-p 2, GOMAXPROCS=2), and below Go's 10m per-package
# default. It is NOT a growth allowance — the guard and the budget row are the
# growth detectors. It exists so an overrun fails with the named
# "no package ran past the … -timeout backstop" assert and the offending FAIL line,
# instead of a 10m goroutine-dump panic. Never raise it to make a slow package fit;
# move the slow tests behind the integration tag instead.
```

(b) Replace the invocation block:

```bash
race_out="$(go test -race $go_conc_args -count=1 ./... 2>&1)"
race_rc=$?
assert "go test -race -count=1 ./... (the whole module) passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'
```
with the following, using your Step 1 value in place of `<N>m`:
```bash
# The backstop — see BACKSTOP TIMEOUT in this header (change 0465).
RACE_TIMEOUT="<N>m"
race_out="$(go test -race $go_conc_args -timeout "$RACE_TIMEOUT" -count=1 ./... 2>&1)"
race_rc=$?
assert "go test -race -count=1 ./... (the whole module) passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'
assert "no package ran past the ${RACE_TIMEOUT} -timeout backstop" '! grep -q -E -e "^panic: test timed out after" <<<"$race_out" || { grep -E -e "^(FAIL[[:space:]]|panic: test timed out)" <<<"$race_out" >&2; printf "%s\n" "tests/test_go_race.sh: a package ran past the ${RACE_TIMEOUT} backstop — the fast default corpus has outgrown this gate; move real-git, subprocess, and process-lifecycle tests behind //go:build integration (see PARTITION AND LANE in this file)" >&2; false; }'
```

(`grep -q` reads a here-string, not a pipe, so the AGENTS.md SIGPIPE rule does not apply. The pattern leads with `^`, not `--`, so the negated assert cannot go vacuous on an option-parse error.)

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -count=1 -run 'TestRaceGate' ./internal/repoguard/ && go test -count=1 ./internal/repoguard/`
Expected: PASS for both. The full repoguard package stays green, including the source-hygiene rules over the edited wrapper.

- [ ] **Step 6: Mutation probes**

Back up `tests/test_go_race.sh` to a `mktemp "${TMPDIR:-/tmp}/race-gate.XXXXXX"` copy.
- (a) Delete ` -timeout "$RACE_TIMEOUT"` from the invocation. Run `go test -count=1 -run TestRaceGatePassesTimeoutBackstopBelowGoDefault ./internal/repoguard/`. Expected: FAIL.
- Copy back.
- (b) Delete the second `assert "no package ran past…"` line. Run `go test -count=1 -run TestRaceGateNamesTimeoutBackstopOnOverrun ./internal/repoguard/`. Expected: FAIL.
- Copy back and re-run both. Expected: PASS.

Report all four results.

- [ ] **Step 7: Run the real gate once**

Run: `bash tests/test_go_race.sh; echo "rc=$?"`
Expected: `ok - ` for all three asserts, `rc=0`.

- [ ] **Step 8: Commit**

```bash
git add tests/test_go_race.sh internal/repoguard/race_gate_timeout_test.go
git commit -m "test(race): explicit -timeout backstop with a readable overrun in test_go_race (change 0465)"
```

---

### Task 15: Honest budget rows and final re-measure

**Files:**
- Modify: `tests/runtime-budgets.tsv` (the rows for `tests/test_go_race.sh`, `tests/test_go_toolchain.sh`, `tests/test_go_integration_app_concurrency.sh`, and the eleven new `tests/test_go_integration_app_*.sh` shards, only where the measurement changes the value)
- Modify: `tests/test_go_integration_app_concurrency.sh` (header comment only, when Tasks 3–13 moved race tests into it)

**Interfaces:**
- Consumes: every earlier task. Specifically, the guard from Task 1, the eleven shard runners from Tasks 3–13 (`tests/test_go_integration_app_{gatecancel,gateverdict,gatefence,gatecompletion,gateepoch,gatearm,gatelifecycle,finalizeops,evidence,recordops,contextprobe}.sh`), and `RACE_TIMEOUT` from Task 14.
- Produces: final rows and the measurement record that the results file cites.

- [ ] **Step 1: The whole default corpus is green with the guard on**

Run:
```bash
bash -c '
out="$(go test -count=1 ./internal/app/ 2>&1)"; rc=$?
printf "%s\n" "$out" | tail -5
grep -E -e "real-git exec attempt\(s\)|docket nogit guard" <<<"$out" || echo "guard: silent"
echo "rc=$rc"
'
```
Expected: `ok  github.com/danielhanold/docket/internal/app`, `guard: silent`, `rc=0`. This is the proof that no default test reaches git. If anything fails, name it in the report and move it with the Tasks 3–13 procedure, into whichever of the eleven shards matches its family.

- [ ] **Step 2: Contract and every app shard green**

```bash
bash -c '
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
for r in tests/test_go_integration_app_*.sh; do bash "$r" >/dev/null 2>&1; echo "$r rc=$?"; done
'
```
Expected: every line ends `rc=0`.

- [ ] **Step 3: Record the headline measurement**

Run: `bash -c 'time GOMAXPROCS=3 go test -race -count=1 -p 3 ./internal/app/' 2>&1 | tail -4`
Expected: `real` of about 15–30s, against the ~238s grooming baseline on the same command. Record it in the report as `internal/app -race @3 CPUs: before ~238s, after <S>s`. If it is above 60s, report `FINDING:` with the `-json` ranking of the slowest tests. Use the Task 14 Step 1 command narrowed to `./internal/app/`, and select `.Test != null` events instead of package events.

- [ ] **Step 4: Re-measure and correct the rows**

Measure each file solo (serial, uncontended) with `bash -c 'time bash <file>' 2>&1 | tail -4`:
- `tests/test_go_race.sh`
- `tests/test_go_toolchain.sh` (its plain `go test ./...` also lost the internal/app real-git tail)
- `tests/test_go_integration_app_concurrency.sh`
- the eleven new shard runners

For each file, compute the rule value: `real` rounded up to the next multiple of 5, plus 5, minimum 10. Set the row to it, raising or lowering. One exception: **never write a parallel row above 60.** If `tests/test_go_race.sh` computes above 60, leave its row at `60` and add a `FINDING:` line to your report. Name the measured seconds and the dominating package from the Task 14 Step 1 ranking. Task 2's repo-relative key now lets the runner's `BUDGET WATCH` → `SERIAL CONFIRMED OVER BUDGET` path surface it across worktrees. Do not silently grant a large ceiling. Then run `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 5: Keep the race shard's header honest**

If Tasks 3–13 moved any `TestRaceIntegrationAppConcurrency…` tests, check with `go test -tags integration -list '^TestRaceIntegrationAppConcurrency' ./internal/app/`. Then update the header comment of `tests/test_go_integration_app_concurrency.sh`. Its current description, "the concurrency-bearing app tests (concurrent planning mutations and gate-retry CAS)", becomes a description that also names the families that joined, for example "…, and the run-gate/launch/finalize-reserve concurrency tests moved out of the default corpus by change 0465". Change only the comment. `SHARD_*` lines are untouched.

- [ ] **Step 6: Commit and report**

```bash
git add tests/runtime-budgets.tsv tests/test_go_integration_app_concurrency.sh
git commit -m "test: re-measure budget rows after the internal/app partition (change 0465)"
```

Your report must carry, for the results file:
- the Task 15 Step 3 before/after numbers
- every row you changed (old → new, with the measured seconds)
- any `FINDING:` line

Also state that **CI acceptance is a human action after merge**: several consecutive green release-candidate source-gate runs on `test_go_race`, with clear headroom under `RACE_TIMEOUT`.
