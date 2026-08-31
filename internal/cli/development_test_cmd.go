package cli

// This file is the `docket development test` wiring: it resolves the fully-formed
// suiterunner.Config from the environment and the checkout under test, runs the
// Go-native whole-suite runner, and returns its exit code (change 0318). Unlike
// every other command in the tree, `development test` produces no JSON result
// document — the runner streams its own report to stdout/stderr and the exit code
// carries the verdict — so root.go threads this integer straight to the process
// exit, bypassing the presenter. The file is named development_test_cmd.go, not
// development_test.go: a *_test.go suffix would make this production source a test
// file the toolchain excludes from the build.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/suiterunner"
)

// runDevelopmentTest resolves the run configuration and hands it to
// suiterunner.Run, returning the process exit code. A configuration that cannot
// be resolved (no repository root) fails closed to exit 2 — the runner will not
// start — mirroring suiterunner.Run's own usage-error exit for an unusable input.
func runDevelopmentTest(ctx context.Context, stdout, stderr io.Writer) int {
	repoRoot, err := gitToplevel()
	if err != nil {
		fmt.Fprintf(stderr, "development test: cannot resolve the repository root: %v\n", err)
		return 2
	}

	// The DOCKET_RUNTESTS_* env seams configure the Go runner (TESTS_DIR/BUDGETS
	// select inputs; JOBS/STATE/STRICT tune the run). An unset seam takes the
	// branch-faithful default.
	testsDir := envOr("DOCKET_RUNTESTS_TESTS_DIR", filepath.Join(repoRoot, "tests"))
	budgets := envOr("DOCKET_RUNTESTS_BUDGETS", filepath.Join(repoRoot, "tests", "runtime-budgets.tsv"))

	jobs := runtime.NumCPU()
	if v := os.Getenv("DOCKET_RUNTESTS_JOBS"); v != "" {
		// A malformed value becomes an out-of-range jobs count, which Run rejects
		// as the usage error (exit 2) the shared exit contract names.
		if n, convErr := strconv.Atoi(strings.TrimSpace(v)); convErr == nil {
			jobs = n
		} else {
			jobs = 0
		}
	}

	// The Go runner keeps its OWN advisory budget-state store, never the Bash
	// runner's (a documented intentional deviation). An unresolvable common git
	// dir leaves the path empty, which Load treats as no history (fail-open).
	state := os.Getenv("DOCKET_RUNTESTS_STATE")
	if state == "" {
		if p, derr := suiterunner.DefaultStatePath(repoRoot); derr == nil {
			state = p
		}
	}

	cfg := suiterunner.Config{
		RepoRoot:    repoRoot,
		TestsDir:    testsDir,
		BudgetsPath: budgets,
		Jobs:        jobs,
		// Bash is left unset: suiterunner.Run resolves "bash" on PATH (and its
		// existing unusable-bash exit-2 still fires). The DOCKET_BASH_PATH seam was
		// retired with the frozen Bash control plane (change 0370).
		StatePath:     state,
		Strict:        os.Getenv("DOCKET_RUNTESTS_STRICT") == "1",
		DurationsPath: os.Getenv("DOCKET_RUNTESTS_TEST_DURATIONS"),
		Stdout:        stdout,
		Stderr:        stderr,
	}
	return suiterunner.Run(ctx, cfg)
}

// gitToplevel resolves the worktree root of the checkout the command runs in.
// The runner tests THIS checkout, so discovery and budgets are anchored here
// rather than at any installed asset tree.
func gitToplevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("git rev-parse --show-toplevel returned an empty path")
	}
	return root, nil
}

// envOr returns the environment value for key, or fallback when it is unset or
// empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
