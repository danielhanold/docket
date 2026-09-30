//go:build integration

package gatedrive

import (
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity drives a REAL scope-bound run: a parent
// prepares a recovery scope, a scope-bound Start launches a real slow child, and a
// parent Takeover then supersedes the child owner and advances the SAME run to its
// terminal pass. It proves the takeover continues one stable supervised identity —
// same raw run dir, raw ownership, attempt, and native pid/pgid/sid — with exactly
// one run slot throughout: no relaunch, no duplicate child.
// Race shard (change 0466): a parent takeover supersedes the owner of a live supervised child mid-run.
func TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	store := OpenStore(testsupport.TempDir(t))
	reapSupervisors(t, runRoot)
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	d := newIntDriver(store, svc)

	// A real scope-bound start over a child that outlives several short slices and
	// is still live when the takeover happens.
	req := intStartRequest(mustExe(t), runRoot, testsupport.TempDir(t), "pass-after", "500")
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("scope-bound first slice over a live child must WAIT, got %s (%s)", started.Outcome, started.Cause)
	}

	// Identity BEFORE takeover: the durable raw run identity plus the native
	// manifest pid/pgid/sid (the driver-independent oracle).
	runDir := soleRunDir(t, runRoot)
	recBefore, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	idBefore := readManifestIdentity(t, runDir)

	// Event-authorized takeover: it supersedes the child owner without launching or
	// stopping any process.
	took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if took.Outcome == HALTED {
		t.Fatalf("a valid takeover of a live scope-bound drive must not HALT: %s", took.Cause)
	}
	if took.Generation == "" || took.Generation == started.Generation {
		t.Fatalf("takeover must mint a fresh owner generation distinct from the child's, got %q", took.Generation)
	}

	// The fresh owner drives the SAME live run to its terminal pass.
	term, _ := advanceUntilTerminal(t, d, started.DriveID, took.Generation)
	if term.Outcome != PASSED {
		t.Fatalf("post-takeover terminal %s (cause %q), want PASSED", term.Outcome, term.Cause)
	}

	// Same run throughout: one run slot, identical raw run dir + raw ownership +
	// attempt, identical gate supervisor identity. The takeover CONTINUED the run.
	recAfter, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if got := soleRunDir(t, runRoot); got != runDir {
		t.Fatalf("run dir changed across takeover: %s -> %s", runDir, got)
	}
	if recAfter.RawRunDir != recBefore.RawRunDir || recAfter.RawOwnership != recBefore.RawOwnership {
		t.Fatalf("takeover changed the raw run identity: dir %q->%q ownership %q->%q",
			recBefore.RawRunDir, recAfter.RawRunDir, recBefore.RawOwnership, recAfter.RawOwnership)
	}
	if recAfter.Attempt != recBefore.Attempt {
		t.Fatalf("takeover changed the attempt %d->%d — it relaunched", recBefore.Attempt, recAfter.Attempt)
	}
	if idAfter := readManifestIdentity(t, runDir); idAfter != idBefore {
		t.Fatalf("supervised identity drifted across takeover: %+v -> %+v", idBefore, idAfter)
	}
}
