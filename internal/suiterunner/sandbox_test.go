package suiterunner

import (
	"os"
	"strings"
	"testing"
)

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
