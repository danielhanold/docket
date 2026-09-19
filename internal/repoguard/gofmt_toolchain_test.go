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
	// ambient GOROOT that fake `go env` returns for an empty GOTOOLCHAIN when
	// FAKE_GOROOT_AMBIENT_ON_EMPTY is set; its bin/gofmt is the ambient
	// (clean) formatter and logs to ambientLog.
	fakeGorootAmbient string
	pkgDir            string // the one package dir fake `go list` reports
	ambientLog        string // every ambient (PATH) gofmt invocation
	pinnedLog         string // every pinned (GOROOT) gofmt invocation
	goLog             string // every fake go invocation, with observed GOTOOLCHAIN
	env               []string
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
    # An empty/unset GOTOOLCHAIN models the ambient resolution the real go
    # performs when no command-scoped toolchain is pinned: it succeeds and
    # yields the ambient GOROOT (whose bin/gofmt is the ambient formatter),
    # NOT a fail-closed error. Only the count-guard mutation test opts in via
    # FAKE_GOROOT_AMBIENT_ON_EMPTY so default fixture behavior is unchanged.
    if [ -z "${GOTOOLCHAIN-}" ] && [ -n "${FAKE_GOROOT_AMBIENT_ON_EMPTY:-}" ]; then
      printf '%s\n' "$FAKE_GOROOT_AMBIENT"
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
		root:              filepath.Join(base, "fixture"),
		fakeBin:           filepath.Join(base, "fakebin"),
		fakeGoroot:        filepath.Join(base, "fakegoroot"),
		fakeGorootAmbient: filepath.Join(base, "fakegorootambient"),
		ambientLog:        filepath.Join(base, "ambient-gofmt.log"),
		pinnedLog:         filepath.Join(base, "pinned-gofmt.log"),
		goLog:             filepath.Join(base, "go.log"),
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
	writeToolScript(t, filepath.Join(f.fakeGorootAmbient, "bin", "gofmt"), fakeAmbientGofmtScript)

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
			"FAKE_GOROOT_EMPTY", "FAKE_GOROOT_CHATTER",
			"FAKE_GOROOT_AMBIENT", "FAKE_GOROOT_AMBIENT_ON_EMPTY":
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
		"FAKE_GOROOT_AMBIENT="+f.fakeGorootAmbient,
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

// TestGofmtMutationCountGuardIsDetected proves the exactly-one-toolchain count
// guard is load-bearing, not decoration. In a real tree, an absent toolchain
// directive means `GOTOOLCHAIN="" go env GOROOT` resolves to the ambient
// GOROOT — reintroducing the PATH-gofmt bug this change exists to prevent — so
// the count guard's `elif [ "$toolchain_count" -ne 1 ]` branch must intercept
// that case. The fake `go env` models that ambient resolution only when
// FAKE_GOROOT_AMBIENT_ON_EMPTY is set (default fixture behavior is unchanged).
// With the guard neutralized, the absent-directive fixture falls through to the
// ambient formatter, which reports clean — so the marker wrongly flips to ok
// and the ambient log fills. The unmutated absent case
// (TestGofmtToolchainDirectiveMustBeExactlyOne) fails closed instead, so the
// pairing is the mutation proof.
func TestGofmtMutationCountGuardIsDetected(t *testing.T) {
	f := newGofmtGateFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, "go.mod"), []byte("module fixture\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.setenv("FAKE_GOROOT_AMBIENT_ON_EMPTY=1")
	mutateWrapper(t, f, `elif [ "$toolchain_count" -ne 1 ]; then`, `elif false; then`)
	marker, out := f.run(t)
	if !strings.HasPrefix(marker, "ok - ") {
		t.Fatalf("count-guard mutant should wrongly pass via ambient resolution, got %q — the guard's removal reddened nothing, so the guard is unproven\n%s", marker, out)
	}
	if got := readLog(t, f.ambientLog); got == "" {
		t.Fatalf("count-guard mutant must invoke the ambient formatter — the mutation did not land:\n%s", out)
	}
}
