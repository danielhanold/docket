<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0436 — test_go_toolchain.sh's gofmt check ignores the pinned toolchain, flip-flopping CI red](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-19-0436-test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch.md)**
<!-- docket:backlink:end -->
# Pinned-Toolchain gofmt Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Check 1 of `tests/test_go_toolchain.sh` run the gofmt shipped with the toolchain declared in `go.mod` (currently `go1.26.5`), never PATH's gofmt, so local and CI formatting verdicts stop flip-flopping.

**Architecture:** All behavior change lands inside the existing Check 1 of the shell wrapper: read the explicit `toolchain` directive from `go.mod`, resolve that toolchain's GOROOT with a command-scoped `GOTOOLCHAIN`, and run `<GOROOT>/bin/gofmt -l` over the existing go-list-derived package dirs, failing closed on every resolution or formatter failure. Regression coverage is behavioral Go tests in `internal/repoguard` that execute a copy of the real wrapper in a temp fixture with fake `go`/`gofmt` tools, so the tests prove which binary ran and with which `GOTOOLCHAIN`, without downloading toolchains or recursing into the suite.

**Tech Stack:** Bash (the wrapper), Go tests in `internal/repoguard` using `internal/testsupport` fixtures, `exec.Command("bash", …)` with fake tools on a prepended PATH.

**Spec:** `docs/superpowers/specs/2026-09-18-test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch-design.md` (synchronized copy under `.docket/`; change 0436)

## Global Constraints

- Exact toolchain selection applies ONLY to formatter resolution in Check 1. `go list`, `go vet`, `go test`, the cross-build owner check, concurrency limits (`DOCKET_GO_TEST_CONCURRENCY`, change 0373), caller `GOFLAGS` handling, and the git-common-dir cache selection keep their current behavior byte for byte (ADR-0108, changes 0373/0434).
- The four-result-marker contract is preserved: Check 1 keeps exactly ONE `ok - ` / `NOT OK - ` marker; every new failure mode routes into that one assert via the `unformatted` diagnostic variable. `markers_emitted -eq 4` stays.
- Never fall back to PATH's `gofmt`. Absent/ambiguous/unusable toolchain resolution fails the formatting check with an actionable diagnostic; it never silently certifies a different formatter.
- The `GOTOOLCHAIN` value is exactly the declared name (e.g. `go1.26.5`), command-scoped, with no `+auto` suffix.
- Capture GOROOT-resolution stdout as the path and stderr to a separate file in the existing `$scratch` dir — never `2>&1` into a value that becomes arguments (learning: captured-stderr-becomes-arguments; change 0304's stderr correction).
- Formatter failure condition: nonzero exit OR nonempty output. A silent nonzero exit must fail.
- Do NOT edit `go.mod`'s `go`/`toolchain` directives, CI's go-version setting, or add any config/downloader/lane/budget/retry/timeout machinery.
- `internal/repoguard` is a real-process test package: every temp dir MUST come from `internal/testsupport.TempDir(t)`, never `t.TempDir()` (the change-0373 fixture guard in `tempdir_fixture_test.go` reddens otherwise).
- Formatting corrections to repo files are formatting-only, produced by the selected formatter, on the files the corrected check actually reports (re-discovered, not assumed).
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- Mutation verification runs uncached (`go test … -count=1`) and mutates scratch copies or committed-then-restored files, never uncommitted work (learnings: cached-runner-serves-a-mutated-tree, mutation-restore-needs-a-backup-copy).

---

### Task 1: Corrected Check 1 + behavioral regression harness in internal/repoguard

**Files:**
- Modify: `tests/test_go_toolchain.sh` (Check 1 block and the file-header prose about the formatter)
- Create: `internal/repoguard/gofmt_toolchain_test.go`

**Interfaces:**
- Consumes: `Root()` from `internal/repoguard/repoguard.go` (`func Root() (string, error)`, resolves the repo root); `testsupport.TempDir(t)` from `internal/testsupport`.
- Produces: fixture type `gofmtGateFixture` with `newGofmtGateFixture(t *testing.T) *gofmtGateFixture`, method `run(t *testing.T) (marker, out string)`, fields `root, wrapper, fakeBin, fakeGoroot, pkgDir, ambientLog, pinnedLog, goLog string; env []string`, and helpers `writeToolScript(t, path, body string)`, `readLog(t, path string) string`. Task 2's mutation tests reuse all of these.

- [ ] **Step 1: Write the failing regression tests**

Create `internal/repoguard/gofmt_toolchain_test.go`:

```go
package repoguard

// Change 0436: the formatting gate (Check 1 of tests/test_go_toolchain.sh)
// must run the gofmt shipped with the toolchain DECLARED in go.mod, never
// PATH's gofmt. Under GOTOOLCHAIN=auto a newer installed Go is retained, so
// the ambient formatter and the declared one can disagree about identical
// source — which flip-flops CI red (a developer's newer gofmt says clean,
// CI's declared-family gofmt says dirty, or vice versa).
//
// These are behavioral tests: each one executes a COPY of the real wrapper
// (read from the repo at test time, so there is no frozen fixture to drift)
// inside a temp fixture whose PATH serves fake `go` and `gofmt` tools. The
// fakes log every invocation and their observed GOTOOLCHAIN, so the asserts
// pin the MECHANISM (which binary ran, with which toolchain selection), not
// only the marker outcome (learning: assert-pins-outcome-not-mechanism).
// The fakes stub go vet / go test to exit 0 so the regression never runs
// the real suite or downloads a toolchain.
//
// Only the `ok - `/`NOT OK - ` marker whose description mentions gofmt is
// consulted; the fixture deliberately omits cmd/docket/main_test.go, so
// Check 4 fails there and the wrapper's overall exit code is meaningless —
// the marker line carries Check 1's verdict.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// fixtureToolchain is deliberately NOT the repo's real declared toolchain:
// the wrapper must derive the name from the fixture's go.mod, so a wrapper
// that hard-codes the repo's version reddens here.
const fixtureToolchain = "go1.99.7"

type gofmtGateFixture struct {
	root       string // fixture repo root: go.mod, tests/, pkg/
	wrapper    string // fixture copy of tests/test_go_toolchain.sh
	fakeBin    string // prepended PATH dir: fake go, fake ambient gofmt
	fakeGoroot string // fake resolved GOROOT holding bin/gofmt
	pkgDir     string // the one package dir fake `go list` reports
	ambientLog string // every ambient (PATH) gofmt invocation
	pinnedLog  string // every pinned (GOROOT) gofmt invocation
	goLog      string // every fake go invocation, with observed GOTOOLCHAIN
	env        []string
}

// writeToolScript writes an executable fake tool. The explicit Chmod matters:
// a create-time mode is masked by the process umask (learning:
// promised-file-mode-needs-explicit-chmod).
func writeToolScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const fakeGoScript = `printf 'argv:[%s] GOTOOLCHAIN:[%s]\n' "$*" "${GOTOOLCHAIN-<unset>}" >>"$GO_FAKE_LOG"
case "$1" in
  list)
    printf '%s\n' "$FAKE_PKG_DIR"
    ;;
  env)
    if [ -n "${FAKE_GOROOT_FAIL:-}" ]; then
      echo "go: fake toolchain resolution failure" >&2
      exit 1
    fi
    if [ -n "${FAKE_GOROOT_EMPTY:-}" ]; then
      exit 0
    fi
    if [ "${GOTOOLCHAIN-}" != "$EXPECT_GOTOOLCHAIN" ]; then
      echo "fake go: GOTOOLCHAIN=[${GOTOOLCHAIN-<unset>}] want [$EXPECT_GOTOOLCHAIN]" >&2
      exit 1
    fi
    if [ -n "${FAKE_GOROOT_CHATTER:-}" ]; then
      echo "go: downloading $EXPECT_GOTOOLCHAIN (fake chatter)" >&2
    fi
    printf '%s\n' "$FAKE_GOROOT"
    ;;
  vet|test)
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`

// The ambient formatter answers CLEAN for everything — the newer-Go
// disagreement shape this change exists to catch.
const fakeAmbientGofmtScript = `printf 'ambient-gofmt argv:[%s]\n' "$*" >>"$AMBIENT_GOFMT_LOG"
exit 0
`

const fakePinnedGofmtScript = `printf 'pinned-gofmt argv:[%s]\n' "$*" >>"$PINNED_GOFMT_LOG"
case "${PINNED_MODE:-clean}" in
  clean) exit 0 ;;
  dirty) printf '%s\n' "$FAKE_PKG_DIR/dirty.go"; exit 0 ;;
  silentfail) exit 3 ;;
esac
`

const fixtureGoMod = "module fixture\n\ngo 1.26.0\n\ntoolchain " + fixtureToolchain + "\n"

func newGofmtGateFixture(t *testing.T) *gofmtGateFixture {
	t.Helper()
	repoRoot, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	realWrapper, err := os.ReadFile(filepath.Join(repoRoot, "tests", "test_go_toolchain.sh"))
	if err != nil {
		t.Fatal(err)
	}

	base := testsupport.TempDir(t)
	f := &gofmtGateFixture{
		root:       filepath.Join(base, "fixture"),
		fakeBin:    filepath.Join(base, "fakebin"),
		fakeGoroot: filepath.Join(base, "fakegoroot"),
		ambientLog: filepath.Join(base, "ambient-gofmt.log"),
		pinnedLog:  filepath.Join(base, "pinned-gofmt.log"),
		goLog:      filepath.Join(base, "go.log"),
	}
	f.wrapper = filepath.Join(f.root, "tests", "test_go_toolchain.sh")
	f.pkgDir = filepath.Join(f.root, "pkg")

	if err := os.MkdirAll(f.pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(f.wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.wrapper, realWrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "go.mod"), []byte(fixtureGoMod), 0o644); err != nil {
		t.Fatal(err)
	}

	writeToolScript(t, filepath.Join(f.fakeBin, "go"), fakeGoScript)
	writeToolScript(t, filepath.Join(f.fakeBin, "gofmt"), fakeAmbientGofmtScript)
	writeToolScript(t, filepath.Join(f.fakeGoroot, "bin", "gofmt"), fakePinnedGofmtScript)

	// Inherit the ambient environment (bash, awk, grep, mktemp live there),
	// but strip every variable the wrapper or the fakes key on, then pin
	// deterministic values. GOMODCACHE/GOCACHE are pre-set so the wrapper's
	// cache block is a no-op inside the fixture.
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "PATH", "GOTOOLCHAIN", "GOMODCACHE", "GOCACHE", "GOFLAGS",
			"DOCKET_GO_TEST_CONCURRENCY", "GO_FAKE_LOG", "AMBIENT_GOFMT_LOG",
			"PINNED_GOFMT_LOG", "FAKE_PKG_DIR", "FAKE_GOROOT",
			"EXPECT_GOTOOLCHAIN", "PINNED_MODE", "FAKE_GOROOT_FAIL",
			"FAKE_GOROOT_EMPTY", "FAKE_GOROOT_CHATTER":
			continue
		}
		f.env = append(f.env, kv)
	}
	f.env = append(f.env,
		"PATH="+f.fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOMODCACHE="+filepath.Join(base, "gomodcache"),
		"GOCACHE="+filepath.Join(base, "gocache"),
		"GO_FAKE_LOG="+f.goLog,
		"AMBIENT_GOFMT_LOG="+f.ambientLog,
		"PINNED_GOFMT_LOG="+f.pinnedLog,
		"FAKE_PKG_DIR="+f.pkgDir,
		"FAKE_GOROOT="+f.fakeGoroot,
		"EXPECT_GOTOOLCHAIN="+fixtureToolchain,
	)
	return f
}

// setenv appends a scenario toggle for the fake tools.
func (f *gofmtGateFixture) setenv(kv string) { f.env = append(f.env, kv) }

var gofmtMarkerRe = regexp.MustCompile(`(?m)^(ok|NOT OK) - [^\n]*gofmt[^\n]*$`)

// run executes the fixture's wrapper copy and returns Check 1's result
// marker plus the full combined output. The wrapper's exit code is NOT the
// oracle here (the fixture has no cmd/docket/main_test.go, so Check 4 is
// always NOT OK); the gofmt marker line is.
func (f *gofmtGateFixture) run(t *testing.T) (marker, out string) {
	t.Helper()
	cmd := exec.Command("bash", f.wrapper)
	cmd.Dir = f.root
	cmd.Env = f.env
	b, _ := cmd.CombinedOutput()
	out = string(b)
	markers := gofmtMarkerRe.FindAllString(out, -1)
	if len(markers) != 1 {
		t.Fatalf("want exactly one gofmt result marker, got %d in wrapper output:\n%s", len(markers), out)
	}
	return markers[0], out
}

// TestGofmtCheckIgnoresAmbientFormatter is the headline regression: the
// pinned formatter reports a dirty file while the ambient (PATH) formatter
// reports clean. The check must fail, name the file, and never have invoked
// the ambient formatter at all.
func TestGofmtCheckIgnoresAmbientFormatter(t *testing.T) {
	f := newGofmtGateFixture(t)
	f.setenv("PINNED_MODE=dirty")
	marker, out := f.run(t)
	if !strings.HasPrefix(marker, "NOT OK - ") {
		t.Fatalf("dirty pinned formatter must fail the check, got %q\n%s", marker, out)
	}
	if !strings.Contains(out, "dirty.go") {
		t.Fatalf("diagnostic must name the unformatted file:\n%s", out)
	}
	if got := readLog(t, f.ambientLog); got != "" {
		t.Fatalf("ambient PATH gofmt must never run, but it logged:\n%s", got)
	}
	pinned := readLog(t, f.pinnedLog)
	if !strings.Contains(pinned, "-l") || !strings.Contains(pinned, f.pkgDir) {
		t.Fatalf("pinned gofmt must run -l over the go-list dirs, log:\n%s", pinned)
	}
}

// TestGofmtToolchainNameDerivesFromGoMod: the fake go env fails unless the
// command-scoped GOTOOLCHAIN equals the FIXTURE go.mod's declared name
// (go1.99.7 — not the real repo's), so an ok marker proves derivation.
func TestGofmtToolchainNameDerivesFromGoMod(t *testing.T) {
	f := newGofmtGateFixture(t)
	marker, out := f.run(t)
	if !strings.HasPrefix(marker, "ok - ") {
		t.Fatalf("clean pinned formatter with derived toolchain must pass, got %q\n%s", marker, out)
	}
	goLog := readLog(t, f.goLog)
	if !strings.Contains(goLog, "GOTOOLCHAIN:["+fixtureToolchain+"]") {
		t.Fatalf("go env must be invoked with GOTOOLCHAIN=%s, log:\n%s", fixtureToolchain, goLog)
	}
}

// TestGofmtGorootStderrChatterDoesNotContaminatePath: a cold toolchain
// resolution writes download chatter to stderr and still succeeds; the
// captured path must stay pure (learning: captured-stderr-becomes-arguments).
func TestGofmtGorootStderrChatterDoesNotContaminatePath(t *testing.T) {
	f := newGofmtGateFixture(t)
	f.setenv("FAKE_GOROOT_CHATTER=1")
	marker, out := f.run(t)
	if !strings.HasPrefix(marker, "ok - ") {
		t.Fatalf("stderr chatter during a successful resolution must not fail the check, got %q\n%s", marker, out)
	}
	pinned := readLog(t, f.pinnedLog)
	if pinned == "" {
		t.Fatalf("pinned gofmt never ran — the resolved path was contaminated or discarded:\n%s", out)
	}
}

// TestGofmtSilentNonzeroFailureFails: no output plus a nonzero exit must
// still fail — an implementation testing only output emptiness reads this
// as clean.
func TestGofmtSilentNonzeroFailureFails(t *testing.T) {
	f := newGofmtGateFixture(t)
	f.setenv("PINNED_MODE=silentfail")
	marker, out := f.run(t)
	if !strings.HasPrefix(marker, "NOT OK - ") {
		t.Fatalf("silent nonzero formatter exit must fail the check, got %q\n%s", marker, out)
	}
	if !strings.Contains(out, "rc=3") {
		t.Fatalf("diagnostic must surface the formatter exit status:\n%s", out)
	}
}

// TestGofmtUnusableToolchainFailsClosed: every resolution failure fails the
// check with a diagnostic and NEVER falls back to PATH gofmt.
func TestGofmtUnusableToolchainFailsClosed(t *testing.T) {
	cases := map[string]func(t *testing.T, f *gofmtGateFixture){
		"go env exits nonzero": func(t *testing.T, f *gofmtGateFixture) {
			f.setenv("FAKE_GOROOT_FAIL=1")
		},
		"go env prints empty": func(t *testing.T, f *gofmtGateFixture) {
			f.setenv("FAKE_GOROOT_EMPTY=1")
		},
		"resolved GOROOT lacks executable gofmt": func(t *testing.T, f *gofmtGateFixture) {
			if err := os.Remove(filepath.Join(f.fakeGoroot, "bin", "gofmt")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newGofmtGateFixture(t)
			arrange(t, f)
			marker, out := f.run(t)
			if !strings.HasPrefix(marker, "NOT OK - ") {
				t.Fatalf("unusable toolchain resolution must fail closed, got %q\n%s", marker, out)
			}
			if got := readLog(t, f.ambientLog); got != "" {
				t.Fatalf("resolution failure must not fall back to PATH gofmt, but it logged:\n%s", got)
			}
		})
	}
}

// TestGofmtToolchainDirectiveMustBeExactlyOne: an absent or ambiguous
// toolchain directive is an actionable failure, not a guess.
func TestGofmtToolchainDirectiveMustBeExactlyOne(t *testing.T) {
	cases := map[string]string{
		"absent":    "module fixture\n\ngo 1.26.0\n",
		"ambiguous": "module fixture\n\ngo 1.26.0\n\ntoolchain go1.99.7\ntoolchain go1.99.8\n",
	}
	for name, gomod := range cases {
		t.Run(name, func(t *testing.T) {
			f := newGofmtGateFixture(t)
			if err := os.WriteFile(filepath.Join(f.root, "go.mod"), []byte(gomod), 0o644); err != nil {
				t.Fatal(err)
			}
			marker, out := f.run(t)
			if !strings.HasPrefix(marker, "NOT OK - ") {
				t.Fatalf("%s toolchain directive must fail the check, got %q\n%s", name, marker, out)
			}
			if !strings.Contains(out, "toolchain") {
				t.Fatalf("diagnostic must point at the toolchain directive:\n%s", out)
			}
			if got := readLog(t, f.ambientLog); got != "" {
				t.Fatalf("directive failure must not fall back to PATH gofmt, log:\n%s", got)
			}
		})
	}
}
```

- [ ] **Step 2: Run the new tests to verify they fail against the current wrapper**

Run: `cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch && go test ./internal/repoguard/ -run 'TestGofmt' -count=1 -v`

Expected: FAIL. `TestGofmtCheckIgnoresAmbientFormatter` fails because the current wrapper invokes bare `gofmt` (ambient log non-empty, marker `ok`); the derivation, chatter, silent-nonzero, fail-closed, and directive tests fail similarly (the current wrapper never resolves a GOROOT, so `f.goLog` has no `env` invocation and pinned gofmt never runs). If instead they fail on fixture plumbing (e.g. `no gofmt result marker`), fix the fixture first — the failure must be the behavioral one.

- [ ] **Step 3: Rewrite Check 1 of the wrapper**

In `tests/test_go_toolchain.sh`, replace the current Check 1 block — everything from the comment line `# Check 1: gofmt reports no unformatted Go source. The directory set is` through the line `assert "gofmt reports no unformatted files" '[ -z "$unformatted" ] || { printf "  unformatted: %s\n" "$unformatted" >&2; false; }'` — with:

```bash
# Check 1: the DECLARED toolchain's gofmt reports no unformatted Go source
# (change 0436). PATH's gofmt is NEVER consulted: under GOTOOLCHAIN=auto a
# newer installed Go is retained, so an ambient formatter and the go.mod
# toolchain's formatter can disagree about identical source — flip-flopping
# the gate between local runs and CI. The formatter is resolved from the
# single explicit `toolchain` directive in go.mod via a command-scoped
# GOTOOLCHAIN (exact name, no +auto), and every resolution failure fails the
# check rather than silently certifying a different formatter. Only formatter
# RESOLUTION is pinned; go list/vet/test keep their existing toolchain
# behavior.
#
# The directory set is DERIVED from the module rather than hand-listed: a
# hand-listed `cmd internal` silently stops checking any package added
# outside those two trees. `go list` is captured and checked on its own so
# its failure cannot be swallowed by an empty gofmt result reading as
# "clean".
#
# `go list` and `go env GOROOT` stderr each go to a FILE, never into the
# captured value via `2>&1`: on a cold cache go writes `go: downloading …`
# to stderr and still exits 0, so folding the streams together feeds that
# chatter into the captured value — as bogus gofmt "directories" for the
# list, or as a corrupt GOROOT path for the resolution. That reddens only on
# the first run after a fresh clone and passes warm, which is precisely the
# failure a suite gate must not have. Diagnostics are replayed from the file
# on each failure path so nothing is lost.
toolchain_names="$(awk '$1=="toolchain"{print $2}' go.mod)"
toolchain_count="$(grep -c . <<<"$toolchain_names")"
pkg_dirs="$(go list -f '{{.Dir}}' ./... 2>"$scratch/go-list.err")"
pkg_dirs_rc=$?
if [ "$pkg_dirs_rc" -ne 0 ]; then
  unformatted="go list failed: $(cat "$scratch/go-list.err" 2>/dev/null)"
elif [ -z "$pkg_dirs" ]; then
  unformatted="go list reported no packages"
elif [ "$toolchain_count" -ne 1 ]; then
  unformatted="go.mod must declare exactly one explicit toolchain directive (found $toolchain_count) — the formatting gate pins gofmt to it; add or dedupe 'toolchain goX.Y.Z' in go.mod"
else
  gofmt_goroot="$(GOTOOLCHAIN="$toolchain_names" go env GOROOT 2>"$scratch/gofmt-goroot.err")"
  gofmt_goroot_rc=$?
  if [ "$gofmt_goroot_rc" -ne 0 ] || [ -z "$gofmt_goroot" ]; then
    unformatted="cannot resolve GOROOT for declared toolchain $toolchain_names (rc=$gofmt_goroot_rc): $(cat "$scratch/gofmt-goroot.err" 2>/dev/null)"
  elif [ ! -x "$gofmt_goroot/bin/gofmt" ]; then
    unformatted="declared toolchain $toolchain_names has no executable gofmt at $gofmt_goroot/bin/gofmt: $(cat "$scratch/gofmt-goroot.err" 2>/dev/null)"
  else
    # shellcheck disable=SC2086 # deliberate word-splitting: one dir per line.
    unformatted="$("$gofmt_goroot/bin/gofmt" -l $pkg_dirs 2>"$scratch/gofmt.err")"
    gofmt_rc=$?
    if [ "$gofmt_rc" -ne 0 ]; then
      # A silent nonzero exit must fail too — emptiness alone reads as clean.
      unformatted="gofmt ($toolchain_names) failed (rc=$gofmt_rc): ${unformatted:-<no output>} $(cat "$scratch/gofmt.err" 2>/dev/null)"
    fi
  fi
fi
assert "the declared toolchain's gofmt reports no unformatted files" '[ -z "$unformatted" ] || { printf "  unformatted: %s\n" "$unformatted" >&2; false; }'
```

Notes for the implementer:
- The file runs under `set -uo pipefail` (no `-e`), so `rc=$?` capture after command substitution assignments works as in the existing Checks 2/3. `grep -c .` exiting 1 on zero matches is harmless in an assignment.
- Keep the `Check 2`/`Check 3`/`Check 4` blocks, the cache block, GOFLAGS handling, `go_conc_args`, the brace group, marker replay, and the `markers_emitted -eq 4` self-count untouched.
- Also update the file-header paragraph that begins `# Requires a Go toolchain on PATH (go.mod pins its version); fails loudly if` — append to that paragraph: `Check 1 additionally resolves gofmt from go.mod's explicit toolchain directive (change 0436), so PATH's Go version cannot change the formatting verdict.` Keep the rest of the header intact.
- The mutation anchors Task 2 depends on are the exact substrings `"$gofmt_goroot/bin/gofmt" -l` and `GOTOOLCHAIN="$toolchain_names" ` — keep those spellings.

- [ ] **Step 4: Run the regression tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch && go test ./internal/repoguard/ -run 'TestGofmt' -count=1 -v`

Expected: PASS (all of `TestGofmtCheckIgnoresAmbientFormatter`, `TestGofmtToolchainNameDerivesFromGoMod`, `TestGofmtGorootStderrChatterDoesNotContaminatePath`, `TestGofmtSilentNonzeroFailureFails`, `TestGofmtUnusableToolchainFailsClosed` with its three subtests, `TestGofmtToolchainDirectiveMustBeExactlyOne` with its two subtests).

- [ ] **Step 5: Run the whole repoguard package (shape/hygiene guards watch tests/*.sh and this package's own fixtures)**

Run: `cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch && go test ./internal/repoguard/ -count=1`

Expected: PASS. If `tempdir_fixture_test.go`, `shellshape_test.go`, `test_source_hygiene_test.go`, or `anchors_test.go` redden, fix the new code to satisfy the guard (e.g. only `testsupport.TempDir`, canonical assert helper untouched, no filename:line cross-references) — never weaken the guard.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
git add tests/test_go_toolchain.sh internal/repoguard/gofmt_toolchain_test.go
git commit -m "fix(0436): pin the formatting gate's gofmt to go.mod's declared toolchain"
```

---

### Task 2: Mutation-resistance proof for the regression coverage

**Files:**
- Modify: `internal/repoguard/gofmt_toolchain_test.go` (append two tests)

**Interfaces:**
- Consumes: `newGofmtGateFixture`, `gofmtGateFixture.run`, `gofmtGateFixture.setenv`, `readLog`, `fixtureToolchain`, `gofmtMarkerRe` from Task 1 (same file).
- Produces: nothing consumed later; these tests are the committed proof that Task 1's asserts key on the real mechanism.

- [ ] **Step 1: Write the mutation-shape tests**

Append to `internal/repoguard/gofmt_toolchain_test.go`:

```go
// mutateWrapper rewrites the FIXTURE's wrapper copy (never the repo file —
// learning: mutation-restore-needs-a-backup-copy) by replacing old with new,
// and fails the test if the anchor does not occur exactly once: a vanished
// anchor means the wrapper's spelling drifted and this mutation proof went
// vacuous (learning: assert-detects-removal-not-replacement).
func mutateWrapper(t *testing.T, f *gofmtGateFixture, old, new string) {
	t.Helper()
	b, err := os.ReadFile(f.wrapper)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), old); n != 1 {
		t.Fatalf("mutation anchor %q occurs %d times in the wrapper, want exactly 1 — update the anchor alongside the wrapper", old, n)
	}
	mutated := strings.Replace(string(b), old, new, 1)
	if err := os.WriteFile(f.wrapper, []byte(mutated), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestGofmtMutationBareGofmtIsDetected proves TestGofmtCheckIgnoresAmbientFormatter
// keys on WHICH formatter runs: with the pinned invocation degraded to bare
// `gofmt`, the same dirty-pinned fixture flips to an ok marker and the
// ambient log fills — exactly the divergences that test asserts against.
func TestGofmtMutationBareGofmtIsDetected(t *testing.T) {
	f := newGofmtGateFixture(t)
	f.setenv("PINNED_MODE=dirty")
	mutateWrapper(t, f, `"$gofmt_goroot/bin/gofmt" -l`, `gofmt -l`)
	marker, out := f.run(t)
	if !strings.HasPrefix(marker, "ok - ") {
		t.Fatalf("bare-gofmt mutant should wrongly pass (ambient reports clean), got %q — the mutation did not land\n%s", marker, out)
	}
	if got := readLog(t, f.ambientLog); got == "" {
		t.Fatalf("bare-gofmt mutant must invoke ambient gofmt — the mutation did not land:\n%s", out)
	}
}

// TestGofmtMutationDroppedGotoolchainIsDetected proves the coverage keys on
// the command-scoped GOTOOLCHAIN: with the scoping removed, the fake go env
// observes an unset GOTOOLCHAIN and refuses, flipping the clean fixture's ok
// marker to NOT OK.
func TestGofmtMutationDroppedGotoolchainIsDetected(t *testing.T) {
	f := newGofmtGateFixture(t)
	mutateWrapper(t, f, `GOTOOLCHAIN="$toolchain_names" `, ``)
	marker, out := f.run(t)
	if !strings.HasPrefix(marker, "NOT OK - ") {
		t.Fatalf("dropped-GOTOOLCHAIN mutant must fail resolution, got %q — the mutation did not land\n%s", marker, out)
	}
	if goLog := readLog(t, f.goLog); !strings.Contains(goLog, "GOTOOLCHAIN:[<unset>]") {
		t.Fatalf("mutant's go env must observe an unset GOTOOLCHAIN, log:\n%s", goLog)
	}
}
```

- [ ] **Step 2: Run the mutation tests**

Run: `cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch && go test ./internal/repoguard/ -run 'TestGofmtMutation' -count=1 -v`

Expected: PASS.

- [ ] **Step 3: Manually mutation-test the REAL wrapper (uncached), then restore**

Task 1's commit is already in; `git checkout --` restores to it safely.

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
# Mutation A: bare gofmt
perl -pi -e 's/"\$gofmt_goroot\/bin\/gofmt" -l/gofmt -l/' tests/test_go_toolchain.sh
go test ./internal/repoguard/ -run 'TestGofmt' -count=1
git checkout -- tests/test_go_toolchain.sh
# Mutation B: dropped GOTOOLCHAIN scoping
perl -pi -e 's/GOTOOLCHAIN="\$toolchain_names" //' tests/test_go_toolchain.sh
go test ./internal/repoguard/ -run 'TestGofmt' -count=1
git checkout -- tests/test_go_toolchain.sh
git status --porcelain   # must be clean
```

Expected: BOTH `go test` runs FAIL (each mutation reddens at least `TestGofmtCheckIgnoresAmbientFormatter` or `TestGofmtToolchainNameDerivesFromGoMod` respectively), and the final `git status` is clean. If a mutant run stays green, the coverage is decoration — fix the tests before proceeding.

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
git add internal/repoguard/gofmt_toolchain_test.go
git commit -m "test(0436): committed mutation-resistance proof for the pinned-gofmt gate"
```

---

### Task 3: Formatting-only repair of the files the corrected check reports

**Files:**
- Modify: whichever files the corrected check reports — re-discovered in Step 1; expected today: `internal/githubcli/comment_integration_test.go` (trailing-comment alignment in `TestIntegrationEnsureCommentIdempotent`).

**Interfaces:**
- Consumes: the corrected Check 1 behavior from Task 1 (declared-toolchain gofmt).
- Produces: a tree the corrected gate certifies clean.

- [ ] **Step 1: Re-discover the unformatted set with the declared formatter (never assume the groomed list)**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
declared="$(awk '$1=="toolchain"{print $2}' go.mod)"
gofmt_goroot="$(GOTOOLCHAIN="$declared" go env GOROOT)"
"$gofmt_goroot/bin/gofmt" -l $(go list -f '{{.Dir}}' ./...)
```

Expected: a (possibly empty) list of absolute file paths; today it should print exactly `…/internal/githubcli/comment_integration_test.go`. Whatever it prints is the repair set — if it is empty, skip Steps 2 and 3 and commit nothing (record that in the results notes later).

- [ ] **Step 2: Apply the declared formatter, formatting-only**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
"$gofmt_goroot/bin/gofmt" -w <each file Step 1 printed>
git diff --stat
git diff
```

Expected: the diff touches only the reported files and only whitespace/alignment (for the known file: trailing-comment alignment inside `TestIntegrationEnsureCommentIdempotent`). Any non-formatting hunk is a stop-and-investigate.

- [ ] **Step 3: Verify the corrected check is now clean and still detects dirt**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
"$gofmt_goroot/bin/gofmt" -l $(go list -f '{{.Dir}}' ./...)   # expected: no output
scratch="$(mktemp -d "${TMPDIR:-/tmp}/docket-0436-fmt.XXXXXX")"
printf 'package p\n\nfunc  Ugly(   ) {\n\treturn}\n' > "$scratch/ugly.go"
"$gofmt_goroot/bin/gofmt" -l "$scratch"   # expected: prints ugly.go
rm -rf "$scratch"
go test ./internal/githubcli/ -count=1    # formatting-only: behavior unchanged
```

Expected: first command silent; second prints the scratch file; the package test passes.

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
git add <each file Step 1 printed, repo-relative>
git commit -m "style(0436): reformat per the declared go.mod toolchain's gofmt"
```

---

### Task 4: tests/README.md formatting remedy

**Files:**
- Modify: `tests/README.md`

**Interfaces:**
- Consumes: the corrected Check 1 behavior (declared-toolchain gofmt).
- Produces: user-facing remedy documentation; nothing else builds on it.

- [ ] **Step 1: Add the remedy note**

In `tests/README.md`, find the section that describes the Go gate / `test_go_toolchain.sh` (search for `test_go_toolchain`); if none names it, add the note under the section on running the suite. Insert:

```markdown
### Formatting failures from the Go gate

`tests/test_go_toolchain.sh` checks formatting with the gofmt shipped by the
toolchain declared in `go.mod` (its explicit `toolchain` directive), never
PATH's gofmt — a newer ambient Go can disagree with the declared one about
identical source, which used to flip-flop CI. To reformat the files the gate
reports, run from the repo root:

```bash
"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -w <files…>
```

The version is derived from `go.mod` at run time — do not copy a literal
`goX.Y.Z` into scripts or docs.
```

(Adjust the fence nesting to the file's house style if it already uses fenced blocks inside sections — the outer block above is illustrative, not part of the inserted text.)

- [ ] **Step 2: Verify the remedy command verbatim (learning: printed-remedy-state-validity)**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
scratch="$(mktemp -d "${TMPDIR:-/tmp}/docket-0436-remedy.XXXXXX")"
printf 'package p\n\nfunc  Ugly(   ) {\n\treturn}\n' > "$scratch/ugly.go"
"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -w "$scratch/ugly.go"
cat "$scratch/ugly.go"
rm -rf "$scratch"
```

Expected: the file comes back properly formatted (`func Ugly() {` etc.) — the command works exactly as printed.

- [ ] **Step 3: Check prose guards still pass (repoguard scans docs too)**

Run: `cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch && go test ./internal/repoguard/ -count=1`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch
git add tests/README.md
git commit -m "docs(0436): formatting remedy derives the gofmt version from go.mod"
```

---

## Suite gate (owned by the build workflow, not a task)

The build's final gate runs whatever `build.test_command` resolves to, from source, over the whole suite — never a hand-picked subset. Read the budget report even on green: Check 1 now adds one `GOTOOLCHAIN=<declared> go env GOROOT` call to `test_go_toolchain.sh` (fast when the toolchain is already downloaded — it is on this host — but the first run on a fresh machine downloads the declared toolchain), and `internal/repoguard` gains roughly a dozen sub-second bash wrapper runs. If a `BUDGET WATCH:` line appears for either, note it in the results file; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to act on. No `tests/runtime-budgets.tsv` edits are planned (budget changes are out of scope for 0436).

## Self-Review (performed while writing)

- Spec coverage: toolchain-directive read with exactly-one enforcement (Task 1 Step 3 + `TestGofmtToolchainDirectiveMustBeExactlyOne`); command-scoped GOTOOLCHAIN GOROOT resolution with separated stderr, rc/nonempty checks, executable `bin/gofmt` requirement, no PATH fallback (Task 1 Step 3 + fail-closed tests); quoted formatter `-l` over go-list dirs failing on nonzero-OR-nonempty (Task 1 + silent-nonzero test); preserved go-list checks, four-marker contract, concurrency/GOFLAGS/caches (Global Constraints + untouched blocks); formatting repair with re-discovery (Task 3); repoguard regression fixture with fake tools, ambient-ignored, go.mod-derived version, stderr chatter, clean/dirty/silent-nonzero/unusable cases (Task 1); mutation resistance for bare gofmt and dropped GOTOOLCHAIN, both as committed shape tests and as uncached mutations of the real wrapper (Task 2); README remedy deriving the version from go.mod (Task 4). Out-of-scope items honored: no go.mod/CI/config/downloader/lane/budget edits anywhere.
- Placeholder scan: none — every step carries the exact code, command, or text.
- Type consistency: `gofmtGateFixture`, `newGofmtGateFixture`, `run`, `setenv`, `readLog`, `mutateWrapper`, `fixtureToolchain`, and the two mutation anchor strings are spelled identically in Tasks 1 and 2; the wrapper variable names (`toolchain_names`, `gofmt_goroot`) match between the wrapper code and the mutation anchors.
