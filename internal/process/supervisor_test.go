package process

import (
	"testing"
)

func TestEnvWithout(t *testing.T) {
	in := []string{"A=1", supervisorRunDirEnv + "=/x", "B=2", supervisorArgvEnv + `=["y"]`}
	out := envWithout(in, supervisorRunDirEnv, supervisorArgvEnv)
	if len(out) != 2 || out[0] != "A=1" || out[1] != "B=2" {
		t.Fatalf("envWithout = %v", out)
	}
}

func TestSupervisorRequested(t *testing.T) {
	if SupervisorRequested() {
		t.Fatal("requested with env unset")
	}
	t.Setenv(supervisorRunDirEnv, "/somewhere")
	if !SupervisorRequested() {
		t.Fatal("not requested with env set")
	}
}
