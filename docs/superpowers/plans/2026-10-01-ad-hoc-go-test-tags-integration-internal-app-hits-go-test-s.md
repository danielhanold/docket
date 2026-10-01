<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0479 — Refuse an unfiltered integration-tagged run of internal/app before go test's 10-minute timeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0479-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s.md)**
<!-- docket:backlink:end -->
# Refuse an unfiltered integration-tagged run of internal/app Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: execute this plan with the `docket-build` skill (docket's build role), task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `internal/app`'s integration-tagged test binary refuse an unfiltered run at go test's default 10-minute timeout, right after compile, with a message naming the supported forms, and document those forms in `tests/README.md`.

**Architecture:** A new guard lives in `internal/testsupport` next to the no-real-git guard and copies its build-tag split. An untagged file holds the pure decision, the flag reading (against an injectable `*flag.FlagSet`, so the default build can unit-test it), and the remedy text. An `//go:build integration` file holds the real entry point, and an `//go:build !integration` twin is a no-op. Only `internal/app`'s `TestMain` calls it. `tests/test_go_integration_contract.sh` gains an end-to-end check that runs the real command shape with `-skip .`, so a removed guard costs seconds, not 19 minutes.

**Tech Stack:** Go 1.26 (`go.mod` toolchain go1.26.5), the stdlib `flag` and `testing` packages, bash suite wrappers under `tests/`.

**Spec:** `docs/superpowers/specs/2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s-design.md` (on the `docket` branch)

## Global Constraints

- Refuse only when `test.run` is empty AND `test.list` is empty AND `test.timeout` is exactly 10m. Any other timeout, including `0`, is allowed. Only `-run` counts as a filter; `-skip` does not.
- `internal/app` only. Do not call the guard from `internal/gatedrive`, `internal/repository/transaction`, or `internal/workspace`.
- No suite, shard-runner, or race-gate timeout changes (0465, ADR-0108). The shard runners get no `-timeout` backstop.
- AGENTS.md and the learnings ledger stay unchanged. Merged plans under `docs/superpowers/plans/` are frozen; do not edit them.
- The remedy text must carry the package name, the shard glob (from the passed-in value, never a second literal in Go source), the `-run '^<Prefix>'` form, and the `-timeout 30m` value. The `30m` constant carries a code comment recording its inputs: 39 `internal/app` shard ceilings in `tests/runtime-budgets.tsv` summing to 1125s (about 18.75m) at change 0479, so 30m is about 1.6x.
- A library never ends the process: the guard returns an error and `TestMain` prints it and calls `os.Exit(1)` (`cmd/docket/exit_sites_test.go` `TestProcessExitSitesAreAllowlisted` scans non-test `.go` files).
- A missing test flag (`Lookup` returns nil) or a flag of an unexpected type is a setup error, never a silent allow.
- **Never hand-run `go test -tags integration ./internal/app/` without `-run`.** That is the bug this change fixes (about 19 minutes, then a timeout panic). Every hand-run integration command in this plan carries `-run` or `-list`. The single exception is the guard's own end-to-end probe inside `tests/test_go_integration_contract.sh`. It always carries `-skip .`, which runs zero tests even when the guard is broken.
- Every manual or mutation run uses `-count=1` (learning `cached-runner-serves-a-mutated-tree`). Every mutation restores from a `cp` backup with `mv -f`, never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).
- Comments anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- Build gate: `go run ./cmd/docket development test` (whole suite), entered from the worktree root.

## Review Focus

1. **An explicit `-timeout 10m`** is indistinguishable from the default and is refused. Pinned by Task 1's `"explicit -timeout 10m is the default"` case.
2. **`-skip .` with no `-run`** is still refused, because only `-run` counts. Pinned by Task 1's `"skip alone is not a filter"` case. The Task 2 end-to-end probe depends on this.
3. **`-timeout 0` and a binary run directly** (`go test -c`, no flags, so `test.timeout` is 0) are allowed. Pinned by Task 1's `"timeout 0 allowed"` case.
4. **Re-exec'd children** (the supervisor and death-guardian roles, which re-exec this test binary) must never reach the guard. Task 2 places the call after both routings. Pinned by running `tests/test_go_integration_app_gatelifecycle.sh`, whose guardian tests re-exec the binary, as a regression in Task 2.
5. **A missing or mistyped testing flag** (for example, a future Go renames `test.list`) must fail as a setup error, never as a silent allow. Pinned by Task 1's missing-flag and wrong-type cases.

---

## File Structure

- Create `internal/testsupport/unfiltered.go` (untagged): `goTestDefaultTimeout`, `WholeCorpusTimeout`, `refuseUnfilteredRun` (the pure decision), `UnfilteredRunRemedy` (the text), `checkUnfilteredRun` (flag reading against a `*flag.FlagSet`).
- Create `internal/testsupport/unfiltered_guard.go` (`//go:build integration`): the real `RefuseUnfilteredIntegrationRun`.
- Create `internal/testsupport/unfiltered_guard_off.go` (`//go:build !integration`): the no-op twin.
- Create `internal/testsupport/unfiltered_test.go` (untagged): unit tests of the decision and the flag reading.
- Modify `internal/app/gate_test.go`: the `TestMain` call and the shared-constants comment.
- Modify `tests/test_go_integration_contract.sh`: new check (11), the end-to-end refusal probe, plus its header line. If the file no longer fits its 15s row, use the fallback sibling `tests/test_go_app_integration_guard.sh` instead (Task 2, Step 6).
- Modify `tests/README.md`: new `### Running integration-tagged Go tests by hand` subsection.

---

### Task 1: The unfiltered-run guard in internal/testsupport

**Files:**
- Create: `internal/testsupport/unfiltered.go`
- Create: `internal/testsupport/unfiltered_guard.go`
- Create: `internal/testsupport/unfiltered_guard_off.go`
- Test: `internal/testsupport/unfiltered_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces (Task 2 relies on exactly this):
  - `func RefuseUnfilteredIntegrationRun(pkg, shardGlob string) error`: nil in every non-`integration` build. In the `integration` build it returns nil to allow, or a non-nil error whose `Error()` is the full remedy text (refusal) or a setup diagnostic.
  - `const WholeCorpusTimeout = "30m"`
  - `func UnfilteredRunRemedy(pkg, shardGlob string) string`: its output contains `shardGlob` verbatim, the line `or run it whole: add -timeout 30m`, and the phrase `default 10m timeout`.

- [ ] **Step 1: Write the failing tests**

Create `internal/testsupport/unfiltered_test.go`:

```go
package testsupport

import (
	"flag"
	"strings"
	"testing"
	"time"
)

// TestRefuseUnfilteredRunDecision pins every clause of the change-0479 decision:
// refuse only when run AND list are empty AND the timeout is go test's 10m
// default. Deleting any one clause of refuseUnfilteredRun reddens at least one
// "allow" case below.
func TestRefuseUnfilteredRunDecision(t *testing.T) {
	cases := []struct {
		name    string
		run     string
		list    string
		timeout time.Duration
		refuse  bool
	}{
		{"unfiltered at the default timeout", "", "", 10 * time.Minute, true},
		{"run set", "^TestIntegrationRunRecord", "", 10 * time.Minute, false},
		{"list set", "", "^Test", 10 * time.Minute, false},
		{"explicit 30m", "", "", 30 * time.Minute, false},
		{"timeout 0", "", "", 0, false},
		{"just under the default", "", "", 10*time.Minute - time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refuseUnfilteredRun(tc.run, tc.list, tc.timeout); got != tc.refuse {
				t.Fatalf("refuseUnfilteredRun(%q, %q, %v) = %v, want %v", tc.run, tc.list, tc.timeout, got, tc.refuse)
			}
		})
	}
}

// testingFlagSet registers the four testing flags the guard reads or must
// ignore, under their real names and types, and parses args the way the
// generated test main would receive them from `go test`.
func testingFlagSet(t *testing.T, args ...string) *flag.FlagSet {
	t.Helper()
	fs := flag.NewFlagSet("unfiltered", flag.ContinueOnError)
	fs.String("test.run", "", "")
	fs.String("test.list", "", "")
	fs.String("test.skip", "", "")
	fs.Duration("test.timeout", 0, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return fs
}

// TestCheckUnfilteredRunReadsTestingFlags drives the decision through real flag
// parsing, with the argv shapes `go test` hands the binary (verified at grooming:
// no -timeout arrives as -test.timeout=10m0s, -timeout 0 as 0s).
func TestCheckUnfilteredRunReadsTestingFlags(t *testing.T) {
	const pkg, glob = "internal/app", "tests/test_go_integration_app_*.sh"
	cases := []struct {
		name   string
		args   []string
		refuse bool
	}{
		{"default timeout, no filter", []string{"-test.timeout=10m0s"}, true},
		{"explicit -timeout 10m is the default", []string{"-test.timeout=10m"}, true},
		{"skip alone is not a filter", []string{"-test.timeout=10m0s", "-test.skip=."}, true},
		{"run filter allowed", []string{"-test.timeout=10m0s", "-test.run=^TestIntegrationRunRecord"}, false},
		{"list probe allowed", []string{"-test.timeout=10m0s", "-test.list=^Test"}, false},
		{"whole run at 30m allowed", []string{"-test.timeout=30m0s"}, false},
		{"timeout 0 allowed", []string{"-test.timeout=0s"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUnfilteredRun(testingFlagSet(t, tc.args...), pkg, glob)
			if !tc.refuse {
				if err != nil {
					t.Fatalf("args %v: got %v, want nil", tc.args, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("args %v: got nil, want the refusal", tc.args)
			}
			if err.Error() != UnfilteredRunRemedy(pkg, glob) {
				t.Fatalf("refusal must be exactly the remedy text, got:\n%s", err)
			}
		})
	}
}

// TestCheckUnfilteredRunSetupErrors: a missing or mistyped flag, or empty
// arguments, is a setup error, never a silent allow and never the remedy text.
func TestCheckUnfilteredRunSetupErrors(t *testing.T) {
	const pkg, glob = "internal/app", "tests/test_go_integration_app_*.sh"
	for _, missing := range []string{"test.run", "test.list", "test.timeout"} {
		t.Run("missing "+missing, func(t *testing.T) {
			fs := flag.NewFlagSet("partial", flag.ContinueOnError)
			for _, n := range []string{"test.run", "test.list"} {
				if n != missing {
					fs.String(n, "", "")
				}
			}
			if missing != "test.timeout" {
				fs.Duration("test.timeout", 10*time.Minute, "")
			}
			err := checkUnfilteredRun(fs, pkg, glob)
			if err == nil || !strings.Contains(err.Error(), missing) || err.Error() == UnfilteredRunRemedy(pkg, glob) {
				t.Fatalf("missing %s: got %v, want a setup error naming it", missing, err)
			}
		})
	}
	t.Run("timeout of the wrong type", func(t *testing.T) {
		fs := flag.NewFlagSet("mistyped", flag.ContinueOnError)
		fs.String("test.run", "", "")
		fs.String("test.list", "", "")
		fs.String("test.timeout", "10m0s", "")
		err := checkUnfilteredRun(fs, pkg, glob)
		if err == nil || !strings.Contains(err.Error(), "test.timeout") || err.Error() == UnfilteredRunRemedy(pkg, glob) {
			t.Fatalf("mistyped test.timeout: got %v, want a setup error naming it", err)
		}
	})
	for _, tc := range []struct{ name, pkg, glob string }{{"empty package", "", glob}, {"empty glob", pkg, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkUnfilteredRun(testingFlagSet(t, "-test.timeout=10m0s"), tc.pkg, tc.glob); err == nil {
				t.Fatalf("%s: got nil, want a setup error", tc.name)
			}
		})
	}
}

// TestUnfilteredRunRemedyCarriesTheSupportedForms pins the spec's required
// content: the package, the shard glob AS PASSED (never a second literal), the
// -run form, and the -timeout value.
func TestUnfilteredRunRemedyCarriesTheSupportedForms(t *testing.T) {
	got := UnfilteredRunRemedy("internal/app", "tests/x_*.sh")
	for _, want := range []string{
		"internal/app:",
		"default 10m timeout",
		"bash <one of tests/x_*.sh>",
		"go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/",
		"-timeout " + WholeCorpusTimeout,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remedy missing %q:\n%s", want, got)
		}
	}
	if WholeCorpusTimeout != "30m" {
		t.Fatalf("WholeCorpusTimeout = %q, want 30m (spec; recompute from tests/runtime-budgets.tsv before changing)", WholeCorpusTimeout)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'Unfiltered' ./internal/testsupport/`
Expected: FAIL (build error: `undefined: refuseUnfilteredRun`, `checkUnfilteredRun`, `UnfilteredRunRemedy`, `WholeCorpusTimeout`).

- [ ] **Step 3: Write the untagged implementation**

Create `internal/testsupport/unfiltered.go`:

```go
package testsupport

// The unfiltered-run guard (change 0479). `go test -tags integration ./internal/app/`
// with no -run filter runs that package's whole integration corpus in one process,
// which outlasts go test's default 10m per-package timeout and dies in a
// goroutine-dump panic. The suite never does this (every shard runner passes
// -run "^${SHARD_PREFIX}"); hand-typed and plan-prescribed runs did. The guard
// refuses that run shape right after compile and names the supported forms.
//
// Build split, mirroring InstallNoGitGuard: the decision, the flag reading, and
// the remedy text live here, untagged, so the default build unit-tests them.
// RefuseUnfilteredIntegrationRun is real only under `//go:build integration`
// (unfiltered_guard.go) and a no-op otherwise (unfiltered_guard_off.go).

import (
	"errors"
	"flag"
	"fmt"
	"time"
)

// goTestDefaultTimeout is the -test.timeout `go test` passes when the caller gives
// no -timeout. An explicit -timeout 10m arrives identically and is treated the same.
const goTestDefaultTimeout = 10 * time.Minute

// WholeCorpusTimeout is the -timeout the remedy offers for a deliberate whole-package
// run. Inputs at change 0479: the 39 tests/test_go_integration_app_*.sh ceilings in
// tests/runtime-budgets.tsv sum to 1125s (about 18.75m). A whole-package run compiles
// once and runs the same tests, so that sum bounds its wall from above; 30m is about
// 1.6x it. Recompute from tests/runtime-budgets.tsv when the shard ceilings move.
const WholeCorpusTimeout = "30m"

// refuseUnfilteredRun is the decision: refuse only an unfiltered (-run empty),
// non-listing (-list empty) run at go test's default timeout. -skip is not a filter.
func refuseUnfilteredRun(run, list string, timeout time.Duration) bool {
	return run == "" && list == "" && timeout == goTestDefaultTimeout
}

// UnfilteredRunRemedy is the refusal text for package pkg (module-relative dir),
// naming its shard runners by shardGlob.
func UnfilteredRunRemedy(pkg, shardGlob string) string {
	return fmt.Sprintf("%s: the integration-tagged corpus outlasts go test's default 10m timeout when run whole (change 0479).\n"+
		"Run one shard:   bash <one of %s>\n"+
		"or filter:       go test -tags integration -count=1 -run '^<Prefix>' ./%s/\n"+
		"or run it whole: add -timeout %s",
		pkg, shardGlob, pkg, WholeCorpusTimeout)
}

// checkUnfilteredRun reads test.run, test.list, and test.timeout from fs and
// returns the remedy as an error on refusal, nil to allow. A missing or mistyped
// flag, or an empty pkg/shardGlob, is a setup error: a guard that cannot read its
// inputs must never silently allow.
func checkUnfilteredRun(fs *flag.FlagSet, pkg, shardGlob string) error {
	if pkg == "" || shardGlob == "" {
		return fmt.Errorf("unfiltered-run guard: package %q and shard glob %q must both be non-empty", pkg, shardGlob)
	}
	lookup := func(name string) (*flag.Flag, error) {
		f := fs.Lookup(name)
		if f == nil {
			return nil, fmt.Errorf("%s: unfiltered-run guard: testing flag %s is not registered", pkg, name)
		}
		return f, nil
	}
	runF, err := lookup("test.run")
	if err != nil {
		return err
	}
	listF, err := lookup("test.list")
	if err != nil {
		return err
	}
	timeoutF, err := lookup("test.timeout")
	if err != nil {
		return err
	}
	getter, ok := timeoutF.Value.(flag.Getter)
	if !ok {
		return fmt.Errorf("%s: unfiltered-run guard: testing flag test.timeout is not readable as a duration", pkg)
	}
	timeout, ok := getter.Get().(time.Duration)
	if !ok {
		return fmt.Errorf("%s: unfiltered-run guard: testing flag test.timeout is not a duration (got %T)", pkg, getter.Get())
	}
	if refuseUnfilteredRun(runF.Value.String(), listF.Value.String(), timeout) {
		return errors.New(UnfilteredRunRemedy(pkg, shardGlob))
	}
	return nil
}
```

Note on the "wrong type" case: a `fs.String` flag's value implements `flag.Getter` and returns a `string`, so it reaches the second check (`not a duration (got string)`). Both messages name `test.timeout`.

- [ ] **Step 4: Write the two tagged entry points**

Create `internal/testsupport/unfiltered_guard.go`:

```go
//go:build integration

package testsupport

import "flag"

// RefuseUnfilteredIntegrationRun is the integration build's real unfiltered-run
// guard (change 0479; see unfiltered.go). Call it from TestMain after any re-exec
// routing and before m.Run. It parses the testing flags if they are not parsed yet
// (the documented TestMain pattern; m.Run skips a second parse) and returns the
// remedy as an error on refusal. The caller prints it and exits non-zero; a
// library never ends the process.
func RefuseUnfilteredIntegrationRun(pkg, shardGlob string) error {
	if !flag.Parsed() {
		flag.Parse()
	}
	return checkUnfilteredRun(flag.CommandLine, pkg, shardGlob)
}
```

Create `internal/testsupport/unfiltered_guard_off.go`:

```go
//go:build !integration

package testsupport

// RefuseUnfilteredIntegrationRun is the no-op twin of the integration build's guard
// (unfiltered_guard.go, change 0479). The default and e2e builds never refuse.
// Exactly one of the two files compiles for any tag set.
func RefuseUnfilteredIntegrationRun(pkg, shardGlob string) error { return nil }
```

- [ ] **Step 5: Run the tests to verify they pass, in both builds**

Run: `go test -count=1 -run 'Unfiltered' -v ./internal/testsupport/`
Expected: PASS for `TestRefuseUnfilteredRunDecision`, `TestCheckUnfilteredRunReadsTestingFlags`, `TestCheckUnfilteredRunSetupErrors`, `TestUnfilteredRunRemedyCarriesTheSupportedForms`, and all their subtests.

Run: `go vet ./internal/testsupport/ && go vet -tags integration ./internal/testsupport/ && go vet -tags e2e ./internal/testsupport/`
Expected: no output, exit 0. This proves exactly one `RefuseUnfilteredIntegrationRun` compiles per tag set.

Run: `go test -tags integration -count=1 -run 'Unfiltered' ./internal/testsupport/`
Expected: PASS. The untagged tests also run in the integration build.

- [ ] **Step 6: Mutation-test each clause of the decision**

For each mutation, back up, mutate, run, restore:

```bash
cp internal/testsupport/unfiltered.go "${TMPDIR:-/tmp}/unfiltered.go.bak"
# mutation A: delete `run == "" && ` from refuseUnfilteredRun's return
# mutation B: delete `list == "" && `
# mutation C: delete ` && timeout == goTestDefaultTimeout`
go test -count=1 -run 'Unfiltered' ./internal/testsupport/
mv -f "${TMPDIR:-/tmp}/unfiltered.go.bak" internal/testsupport/unfiltered.go
```

Expected: each of A, B, and C reddens at least one case. A reddens "run set" and "run filter allowed". B reddens "list set" and "list probe allowed". C reddens "explicit 30m", "timeout 0", and their flag-set twins. If a mutation stays green, the table is decoration: fix the table before going on. After the last restore, rerun Step 5's first command and expect PASS.

- [ ] **Step 7: Format and commit**

```bash
"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -l internal/testsupport/
# expect no output; if any, rerun with -w on the listed files
git add internal/testsupport/unfiltered.go internal/testsupport/unfiltered_guard.go internal/testsupport/unfiltered_guard_off.go internal/testsupport/unfiltered_test.go
git commit -m "feat(testsupport): unfiltered integration-run guard (change 0479)"
```

---

### Task 2: Wire the guard into internal/app, prove it end to end, document the forms

**Files:**
- Modify: `internal/app/gate_test.go` (`TestMain` and the comment over `nogitPkg`/`nogitShardGlob`)
- Modify: `tests/test_go_integration_contract.sh` (header list; new check (11) before the final `exit "$fail"`)
- Modify: `tests/README.md` (new subsection under `## Running it`, after `### Formatting failures from the Go gate`, before `## Where new tests go`)
- Fallback only (Step 6): Create `tests/test_go_app_integration_guard.sh`; Modify `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes from Task 1: `testsupport.RefuseUnfilteredIntegrationRun(pkg, shardGlob string) error`. On refusal its error text contains the passed shard glob, `default 10m timeout`, and `-timeout 30m`.
- Produces: nothing other tasks consume.

- [ ] **Step 1: Record the contract test's baseline wall time**

Run: `time bash tests/test_go_integration_contract.sh`
Expected: all `ok -` lines, exit 0. Write down the `real` time. This is the solo baseline against its 15s row in `tests/runtime-budgets.tsv`. Run it twice and keep the second (warm cache) number.

- [ ] **Step 2: Add the failing end-to-end check (11) to the contract test**

In `tests/test_go_integration_contract.sh`, append this item to the header's numbered check list, directly after the `(10)` item and before the blank `#` line that precedes `# FAIL-CLOSED.`:

```bash
#   (11) internal/app's integration-tagged binary refuses an unfiltered run at go
#        test's default 10m timeout and names the supported forms (change 0479).
#        The probe passes -skip . so a broken or unwired guard runs zero tests and
#        reddens in seconds instead of running the ~19-minute corpus.
```

Insert this block immediately before the file's final `exit "$fail"`:

```bash
# (11) change 0479: the unfiltered-run guard, proved on the real command shape.
# No -run (the shape the guard refuses) plus -skip . (so with the guard or its
# TestMain call removed, every test is skipped and the run passes quickly, which
# reddens this assert rather than starting the whole corpus). The assert pins the
# MECHANISM, not just a non-zero exit (learning assert-pins-outcome-not-mechanism):
# a compile failure also exits non-zero, but it does not print the remedy.
guard_out="$(go test -tags integration -count=1 -skip . ./internal/app/ 2>&1)"; guard_rc=$?
assert "an unfiltered integration-tagged internal/app run is refused with the remedy (change 0479)" \
  '[ "$guard_rc" -ne 0 ] && grep -qF -- "default 10m timeout" <<<"$guard_out" && grep -qF -- "tests/test_go_integration_app_*.sh" <<<"$guard_out" && grep -qF -- "-timeout 30m" <<<"$guard_out" || { printf "%s\n" "$guard_out" >&2; false; }'
```

(The assert body is single-quoted for `eval`, so it holds no apostrophes. Every pattern goes through `grep -qF --` because `-timeout 30m` leads with `-`. The output is captured into a variable first, never piped into `grep -q`, because of AGENTS.md's pipefail rule.)

- [ ] **Step 3: Run the contract test to verify check (11) fails**

Run: `bash tests/test_go_integration_contract.sh`
Expected: exit 1, with `NOT OK - an unfiltered integration-tagged internal/app run is refused with the remedy (change 0479)`. `-skip .` skips everything, so stderr shows an `ok  github.com/danielhanold/docket/internal/app` line and the run takes seconds, not minutes. Every other line stays `ok -`.

- [ ] **Step 4: Wire the guard into TestMain**

In `internal/app/gate_test.go`, replace the comment above the `const` block:

```go
// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (testsupport.InstallNoGitGuard): its diagnostic and its remedy text.
```

with:

```go
// nogitPkg and nogitShardGlob name this package to the shared test guards
// (testsupport.InstallNoGitGuard and testsupport.RefuseUnfilteredIntegrationRun):
// their diagnostics and their remedy text.
```

In `TestMain`, insert this block after the `GuardianRequested()` routing block and before the `// Change 0465 (hoisted by change 0466)` comment and the `InstallNoGitGuard` call:

```go
	// Change 0479: the integration-tagged build refuses an unfiltered run at go
	// test's default 10m timeout (the whole corpus outlasts it) and names the
	// supported forms; every other build gets testsupport's no-op twin. It sits
	// AFTER the supervisor and guardian re-exec routing (re-exec'd children never
	// parse test flags) and before m.Run.
	if err := testsupport.RefuseUnfilteredIntegrationRun(nogitPkg, nogitShardGlob); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
```

Also extend the doc comment above `func TestMain` by one sentence after "...around m.Run.": `Integration-tagged runs first pass the unfiltered-run guard (change 0479).`

- [ ] **Step 5: Run the contract test to verify it passes, and measure it**

Run: `time bash tests/test_go_integration_contract.sh` (twice; keep the second number)
Expected: every line `ok -`, exit 0, including check (11). Its `-list` probes on `internal/app` set `test.list`, so the guard allows them.

Compare the `real` time to the 15s row. If it is at or under 15s, skip Step 6. Record the before and after numbers and the remaining margin (for example "11.2s -> 12.0s of 15s, 3.0s margin") in the commit message body (learning `budget-headroom-is-spent-before-it-is-breached`).

- [ ] **Step 6 (fallback, only if Step 5 measured over 15s): move check (11) to a sibling file**

Do not raise the contract row's ceiling (`tests/README.md`: "Never grow a file past its budget and raise the number"). Remove the check (11) block and its header item from `tests/test_go_integration_contract.sh`, then create `tests/test_go_app_integration_guard.sh`. The name must NOT match `tests/test_go_integration_*.sh`, because the contract test and `tests/test_go_race.sh` discover that glob as shard runners:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_app_integration_guard.sh — change 0479: internal/app's
# integration-tagged test binary refuses an unfiltered run at go test's default 10m
# timeout and names the supported forms. Split from tests/test_go_integration_contract.sh
# for budget. Deliberately NOT named test_go_integration_*.sh: the contract test and
# tests/test_go_race.sh discover that glob as shard runners. The probe passes -skip .
# so a broken or unwired guard runs zero tests and reddens in seconds.
#
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the CACHES
# note in that file's header).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'this guard check cannot certify anything without a Go toolchain\n' >&2
  exit 1
fi

export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
  common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
  if [ -n "$common_git_dir" ]; then
    case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
    cache_root="$common_git_dir/docket-go-cache"
    if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
      export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
      export GOCACHE="${GOCACHE:-$cache_root/build}"
    fi
  fi
fi

# Assert pins the MECHANISM (the remedy text), not only a non-zero exit
# (learning assert-pins-outcome-not-mechanism).
guard_out="$(go test -tags integration -count=1 -skip . ./internal/app/ 2>&1)"; guard_rc=$?
assert "an unfiltered integration-tagged internal/app run is refused with the remedy (change 0479)" \
  '[ "$guard_rc" -ne 0 ] && grep -qF -- "default 10m timeout" <<<"$guard_out" && grep -qF -- "tests/test_go_integration_app_*.sh" <<<"$guard_out" && grep -qF -- "-timeout 30m" <<<"$guard_out" || { printf "%s\n" "$guard_out" >&2; false; }'

exit "$fail"
```

The `assert` line must match the tree's canonical helper byte for byte (`internal/repoguard` source-hygiene rule (a)). Copy it from `tests/test_go_integration_contract.sh`, not from this plan, if they differ. Measure `time bash tests/test_go_app_integration_guard.sh` solo (second run). Add a row to `tests/runtime-budgets.tsv` next to the `tests/test_go_integration_contract.sh` row: `tests/test_go_app_integration_guard.sh<TAB><measured, rounded up to the next multiple of 5, plus 5, minimum 10><TAB>parallel`. Use a real tab. Then run `go test -count=1 -run 'TestRuntimeBudgetsCorrespondence' ./internal/repoguard/` and expect PASS. In Steps 7 and 8 below, substitute this file for the contract test.

- [ ] **Step 7: Mutation-test the wiring**

```bash
cp internal/app/gate_test.go "${TMPDIR:-/tmp}/gate_test.go.bak"
# delete the whole `if err := testsupport.RefuseUnfilteredIntegrationRun(...) { ... }` block from TestMain
bash tests/test_go_integration_contract.sh; echo "exit=$?"
mv -f "${TMPDIR:-/tmp}/gate_test.go.bak" internal/app/gate_test.go
bash tests/test_go_integration_contract.sh; echo "exit=$?"
```

Expected: the mutated run prints `NOT OK - an unfiltered integration-tagged internal/app run is refused with the remedy (change 0479)` and `exit=1`, and finishes in seconds (proof that `-skip .` keeps a broken guard cheap). The restored run is all `ok -` with `exit=0`. Confirm with `git diff --stat internal/app/gate_test.go` that the restore kept your Step 4 edit.

Repeat once with the guard itself mutated: back up `internal/testsupport/unfiltered_guard.go`, change its body to `return nil`, rerun the contract test, expect the same `NOT OK`, and restore with `mv -f`.

- [ ] **Step 8: Regression-check the shapes that must keep working**

Run: `bash tests/test_go_integration_app_gatelifecycle.sh`
Expected: all `ok -`. Its guardian tests re-exec this test binary, which proves the re-exec routing still runs before the guard (Review Focus 4).

Run: `bash tests/test_go_integration_app_runrecord.sh`
Expected: all `ok -`. This is a normal filtered shard: `-run "^${SHARD_PREFIX}"` is allowed.

Run: `go test -tags integration -count=1 -run '^TestIntegrationRunRecord' ./internal/app/`
Expected: `ok`. This is the hand-run filter form the remedy recommends.

Run: `go test -count=1 -run '^TestMapObservationTable$' ./internal/app/`
Expected: `ok`. The default (untagged) build gets the no-op twin.

- [ ] **Step 9: Document the supported forms in tests/README.md**

Insert this subsection after the `### Formatting failures from the Go gate` subsection (after its closing paragraph "The version is derived from `go.mod` at run time ...") and before `## Where new tests go`:

````markdown
### Running integration-tagged Go tests by hand

The real-git, subprocess, and process-lifecycle Go tests sit behind the `integration` build tag
(change 0333). The suite runs them only through the `tests/test_go_integration_*.sh` shard
runners, each filtered to one test-name prefix, so no single `go test` process runs a whole
package's corpus.

To run some by hand, run a shard runner, or filter to a prefix yourself:

```bash
bash tests/test_go_integration_app_runrecord.sh
go test -tags integration -count=1 -run '^<Prefix>' <pkg>
```

Do not drop `-run` on `./internal/app/`: its whole integration corpus takes about 19 minutes,
longer than go test's default 10-minute per-package timeout. Its test binary refuses an unfiltered
run at that default timeout (change 0479) and prints these forms. For a deliberate whole-package
run, add `-timeout 30m`.
````

- [ ] **Step 10: Format, verify, and commit**

```bash
"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -l internal/app/gate_test.go
# expect no output
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
git add internal/app/gate_test.go tests/test_go_integration_contract.sh tests/README.md
# fallback only: git add tests/test_go_app_integration_guard.sh tests/runtime-budgets.tsv
git commit -m "feat(app): refuse an unfiltered integration-tagged internal/app run (change 0479)" \
  -m "Contract test wall: <before>s -> <after>s of its 15s row (<margin>s margin)."
```

Fill in the measured numbers in the second `-m`; do not commit the placeholder text.

---

## Build gate

After both tasks, the build role runs the whole suite once with the configured gate command, from the worktree root:

```bash
go run ./cmd/docket development test
```

Expected: exit 0. Read the budget report even on green. A `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, or `SERIAL CONFIRMED OVER BUDGET:` line naming `tests/test_go_integration_contract.sh` (or the fallback sibling) is a finding to act on, not to ignore. Every shard passes `-run`, the contract's `-list` probes are allowed, and the e2e lane builds without the `integration` tag, so no existing caller should change behavior.
