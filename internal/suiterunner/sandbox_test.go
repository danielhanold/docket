package suiterunner

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestSandboxGitConfigDisablesBackgroundWork(t *testing.T) {
	jobdir := testsupport.TempDir(t) // suiterunner is itself adopted in Task 6; leave for now
	env, err := Sandbox(jobdir, 0)
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
	// Under the gate the runner exports DOCKET_GO_TEST_CONCURRENCY into this
	// very test process, and Sandbox returns os.Environ()+overrides — so the
	// ambient value would leak into the result and mask whether Sandbox itself
	// added one. Clear it so the cap-0 assertion measures Sandbox's own
	// contribution, not what it inherited (change 0373).
	if v, ok := os.LookupEnv("DOCKET_GO_TEST_CONCURRENCY"); ok {
		os.Unsetenv("DOCKET_GO_TEST_CONCURRENCY")
		t.Cleanup(func() { os.Setenv("DOCKET_GO_TEST_CONCURRENCY", v) })
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
