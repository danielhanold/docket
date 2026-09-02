// This file owns per-target isolation: the child's HOME/TMPDIR/git-config layout
// and the exact environment overrides the former Bash oracle's launch()
// exported. A target runs against a synthetic git identity and no
// interactive prompts, in a private HOME/TMPDIR, so nothing it does touches the
// developer's real home or blocks on a human.
package suiterunner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/danielhanold/docket/internal/gitbg"
)

// goLoadMultNum/goLoadMultDen bound the gate-wide Go package concurrency at or
// under (mult × NumCPU) instead of (-j × NumCPU): with -j targets each running
// Go tests capped at mult*cpus/jobs, the product stays near mult*cpus rather
// than jobs*cpus, so the parallel lane cannot oversubscribe the machine with
// Go test workers (change 0373).
// MEASURED (change 0373, Task 9) on Darwin arm64, hw.ncpu=11, gate -j=11
// (`development test` default = runtime.NumCPU()). Each candidate ran one full
// `go run ./cmd/docket development test`; per-target cap = GoTestConcurrency(11,11).
// Candidate → cap → suite wall clock → BUDGET WATCH rows (measured > 2.5x ceiling):
//
//	1/1 → cap 1 → 288s → 4 rows   (whole-module race/toolchain crawl: 287s/192s)
//	3/2 → cap 1 → 295s → 4 rows   (integer-division cap identical to 1/1 here)
//	2/1 → cap 2 → 210s → 4 rows   (same watch set; race/toolchain 208s/161s)   <- PINNED
//	3/1 → cap 3 → 191s → 6 rows   (oversubscription grows the watch set: +app_concurrency, +app_merge)
//
// No candidate kept every solo-seeded budget row under its ceiling under
// parallel load (that is Task 10's re-seed; the over rows travel in the pinning
// commit body and the change 0373 results file). 2/1 is the pick: it is the
// largest multiplier that does NOT grow the screening watch set beyond the
// cap-1 baseline while recovering the wall clock that cap 1 wastes leaving cores
// idle behind the whole-module wrappers; cap 3 regresses isolation, which is the
// mechanism this constant exists to protect. No candidate produced a
// SERIAL CONFIRMED OVER BUDGET (the authoritative disqualifier). Full
// per-candidate/per-row table: the change 0373 results file.
const (
	goLoadMultNum = 2
	goLoadMultDen = 1
)

// GoTestConcurrency derives the per-target cap the sandbox exports as
// DOCKET_GO_TEST_CONCURRENCY: with -j targets in flight, a per-target cap of
// mult*cpus/jobs bounds the product at mult*cpus concurrent Go test packages.
// Floor 1 (a target always makes progress), ceiling cpus (never asking Go for
// more package parallelism than the machine has cores).
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

// GitBackgroundOff is the exported alias preserved for callers that still spell
// the git-background-off config as suiterunner.GitBackgroundOff (internal/app's
// status_git_test.go). Its single source is gitbg.BackgroundOff; the literal was
// relocated to the dependency-free leaf internal/gitbg (change 0373) so that
// internal/testsupport — imported from suiterunner's own test files — can share
// it without forming an import cycle.
const GitBackgroundOff = gitbg.BackgroundOff

// gitIdentityConfig is the synthetic global git config every target sees: a
// present-but-fake identity (a test that commits must still be able to) and a
// deterministic default branch, with change 0373's git-background-off knobs
// appended. The identity core (through defaultBranch) is byte-for-byte what
// the oracle's launch() wrote; gitbg.BackgroundOff is the 0373 addition.
const gitIdentityConfig = "[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n" + gitbg.BackgroundOff

// Sandbox builds the isolated child environment for one target under jobdir and
// creates the directories and git-config files it references. It returns the
// full env slice: the base os.Environ() with the isolation overrides appended
// last, so exec (which honors the last value for a duplicated key) uses the
// override. The override set is exactly launch()'s: private HOME/TMPDIR/
// XDG_CONFIG_HOME, synthetic GIT_CONFIG_GLOBAL/SYSTEM, and the no-prompt/
// no-pager/no-autoedit git knobs. When goTestConcurrency >= 1 it additionally
// exports DOCKET_GO_TEST_CONCURRENCY (the Go wrappers translate it into
// `go test -p` / GOMAXPROCS); a value of 0 omits the variable, so a solo run
// (bare `go test`, `bash tests/test_X.sh`) sees Go's defaults unchanged
// (change 0373).
func Sandbox(jobdir string, goTestConcurrency int) ([]string, error) {
	home := filepath.Join(jobdir, "home")
	tmp := filepath.Join(jobdir, "tmp")
	configHome := filepath.Join(home, ".config")
	gitGlobal := filepath.Join(home, ".gitconfig")
	gitSystem := filepath.Join(home, ".gitconfig-system")

	for _, d := range []string{configHome, tmp} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("suiterunner: sandbox mkdir %q: %w", d, err)
		}
	}
	// An empty system config, and the synthetic identity as the global config.
	// Written directly (not via `git config`) — it lands at $HOME/.gitconfig so
	// a pre-2.32 git that ignores GIT_CONFIG_GLOBAL still reads exactly this.
	if err := os.WriteFile(gitSystem, nil, 0o644); err != nil {
		return nil, fmt.Errorf("suiterunner: sandbox write system config: %w", err)
	}
	if err := os.WriteFile(gitGlobal, []byte(gitIdentityConfig), 0o644); err != nil {
		return nil, fmt.Errorf("suiterunner: sandbox write global config: %w", err)
	}

	overrides := []string{
		"HOME=" + home,
		"TMPDIR=" + tmp,
		"XDG_CONFIG_HOME=" + configHome,
		"GIT_CONFIG_GLOBAL=" + gitGlobal,
		"GIT_CONFIG_SYSTEM=" + gitSystem,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
		"GIT_EDITOR=true",
		"EDITOR=true",
		"VISUAL=true",
		"GIT_PAGER=cat",
		"PAGER=cat",
		"GIT_MERGE_AUTOEDIT=no",
	}
	if goTestConcurrency >= 1 {
		overrides = append(overrides, "DOCKET_GO_TEST_CONCURRENCY="+strconv.Itoa(goTestConcurrency))
	}
	return append(os.Environ(), overrides...), nil
}
