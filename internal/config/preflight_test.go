package config

import (
	"errors"
	"reflect"
	"testing"
)

// blockedSnapshot is Task 6's four-blocker fixture: two layers, four active
// deferred capabilities. Reconstructed here rather than shared so a change to
// the classifier's fixture cannot silently weaken the preflight's evidence.
func blockedSnapshot(t *testing.T) *Snapshot {
	t.Helper()
	return mustSnapshot(t,
		srcG("auto_capture:\n  enabled: true\n"),
		srcR("build:\n  checkpoint: true\nterminal_publish: true\nfinalize:\n  skip_results_only_delta: true\n"),
	)
}

func TestPreflightAllowed(t *testing.T) {
	snap := mustSnapshot(t, srcR("metadata_branch: docket\n"))
	got := PreflightMutation(snap)
	if !got.Allowed {
		t.Errorf("Allowed = false, want true; blockers %v", got.Blockers)
	}
	if len(got.Blockers) != 0 {
		t.Errorf("Blockers = %+v, want none", got.Blockers)
	}
}

func TestPreflightBlockedComplete(t *testing.T) {
	snap := blockedSnapshot(t)
	got := PreflightMutation(snap)
	if got.Allowed {
		t.Fatalf("Allowed = true, want false")
	}
	want := []string{
		"auto_capture.enabled",
		"build.checkpoint",
		"finalize.skip_results_only_delta",
		"terminal_publish",
	}
	var paths []string
	for _, d := range got.Blockers {
		if d.Code != CodeDeferredCapRequested {
			t.Errorf("blocker %s carries code %s, want %s", d.Path, d.Code, CodeDeferredCapRequested)
		}
		paths = append(paths, d.Path)
	}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("blocker paths = %v, want %v", paths, want)
	}
}

// TestGuardMutationRefusesContinuation is the seam's whole point: a blocked
// configuration must not reach the mutation, not merely report an error after
// it ran.
func TestGuardMutationRefusesContinuation(t *testing.T) {
	snap := blockedSnapshot(t)
	sentinel := false
	err := GuardMutation(snap, func() error {
		sentinel = true
		return nil
	})
	if !errors.Is(err, ErrUnsupportedConfig) {
		t.Errorf("err = %v, want it to wrap ErrUnsupportedConfig", err)
	}
	if sentinel {
		t.Errorf("continuation ran under a blocked configuration")
	}
}

func TestGuardMutationRunsWhenAllowed(t *testing.T) {
	snap := mustSnapshot(t, srcR("metadata_branch: docket\n"))
	sentinel := false
	err := GuardMutation(snap, func() error {
		sentinel = true
		return nil
	})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if !sentinel {
		t.Errorf("continuation did not run under an allowed configuration")
	}
}

func TestGuardMutationPropagatesContinuationError(t *testing.T) {
	snap := mustSnapshot(t, srcR("metadata_branch: docket\n"))
	want := errors.New("the mutation itself failed")
	err := GuardMutation(snap, func() error { return want })
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want the continuation's own error", err)
	}
	if errors.Is(err, ErrUnsupportedConfig) {
		t.Errorf("err = %v, must not be reported as an unsupported configuration", err)
	}
}
