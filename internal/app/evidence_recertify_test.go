package app

import (
	"context"
	"testing"
)

// buildVsFinalizeYAML declares DIVERGENT build/finalize commands so a test can
// prove which owner's command a gate resolved (acceptance: differing commands
// prove only the BUILD command runs).
const buildVsFinalizeYAML = "build:\n  gate: local\n  test_command: go test ./build-only\nfinalize:\n  test_command: make finalize-only\n"

// TestBuildLocalGateResolvesBuildCommandOnly: the BUILD-owned production gate
// resolves build.test_command; the finalize twin resolves finalize.test_command
// from the same pin. Deleting the owner branch in buildDriveService reddens one
// of the two arms.
func TestBuildLocalGateResolvesBuildCommandOnly(t *testing.T) {
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), buildVsFinalizeYAML)
	ctx := context.Background()

	bg := NewBuildLocalGate(deps, wdeps).(*processFinalizeGate)
	bsvc, ok := bg.buildDriveService(ctx, repoDir)
	if !ok || bsvc.command != "go test ./build-only" {
		t.Fatalf("build gate resolved (ok=%v, command=%q); want build.test_command %q", ok, commandOf(bsvc), "go test ./build-only")
	}
	fg := NewFinalizeGate(deps, wdeps).(*processFinalizeGate)
	fsvc, ok := fg.buildDriveService(ctx, repoDir)
	if !ok || fsvc.command != "make finalize-only" {
		t.Fatalf("finalize gate resolved (ok=%v, command=%q); want finalize.test_command %q", ok, commandOf(fsvc), "make finalize-only")
	}
}

// commandOf tolerates a nil service in a failure message.
func commandOf(svc *GateDriveService) string {
	if svc == nil {
		return "<nil>"
	}
	return svc.command
}

// TestBuildLocalGateFailsClosedWithoutBuildCommand: a config with ONLY
// finalize.test_command set fails the build-owned gate closed (ok=false → the
// caller halts, never a fabricated red) while the finalize twin still resolves.
// This pins the guard's keying on the owner's OWN config key.
func TestBuildLocalGateFailsClosedWithoutBuildCommand(t *testing.T) {
	yaml := "finalize:\n  test_command: make finalize-only\n"
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), yaml)
	ctx := context.Background()

	bg := NewBuildLocalGate(deps, wdeps).(*processFinalizeGate)
	if _, ok := bg.buildDriveService(ctx, repoDir); ok {
		t.Fatalf("build-owned gate resolved a drive service with no build.test_command; must fail closed")
	}
	fg := NewFinalizeGate(deps, wdeps).(*processFinalizeGate)
	if _, ok := fg.buildDriveService(ctx, repoDir); !ok {
		t.Fatalf("finalize-owned gate must still resolve from finalize.test_command")
	}
}
