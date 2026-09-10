package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// launchWithToken launches a helper carrying a caller reservation token and,
// like launchHelper, reaps the supervisor and drains its quiesce on cleanup.
func launchWithToken(t *testing.T, svc *Service, root, token, mode string, extra ...string) *LaunchOutcome {
	t.Helper()
	out, err := svc.Launch(LaunchRequest{
		Root:             root,
		Cwd:              testsupport.TempDir(t),
		Argv:             helperArgv(t, mode, extra...),
		ReservationToken: token,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	reapSupervisor(out.RunDir)
	testsupport.DrainOnCleanup(t, func() { quiesceRun(t, out.RunDir) })
	return out
}

// TestResolveReservationNeverLaunched — a clean, complete, empty census proves
// the token's launch never wrote a run.
func TestResolveReservationNeverLaunched(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	res, err := svc.ResolveReservation(root, "abcabcabcabcabcabcabcabcabcabcab")
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != "never-launched" {
		t.Fatalf("disposition = %q, want never-launched", res.Disposition)
	}
}

// TestResolveReservationIdentifiedRunning — a real launch carrying the token
// resolves to the exact running run.
func TestResolveReservationIdentifiedRunning(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	token := "deadbeefdeadbeefdeadbeefdeadbeef"
	out := launchWithToken(t, svc, root, token, "sleep")
	res, err := svc.ResolveReservation(root, token)
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != "identified" || res.State != StateRunning || res.RunID != out.RunID {
		t.Fatalf("resolve running: %+v (want identified/running/%s)", res, out.RunID)
	}
	killRun(t, out.RunDir)
}

// TestResolveReservationIdentifiedTerminal — a terminated real run carrying the
// token resolves to identified with its exact terminal record.
func TestResolveReservationIdentifiedTerminal(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	token := "cafecafecafecafecafecafecafecafe"
	out := launchWithToken(t, svc, root, token, "exit", "0")
	observeUntilTerminal(t, svc, out.RunDir)
	res, err := svc.ResolveReservation(root, token)
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != "identified" || res.State != StatePassed {
		t.Fatalf("resolve terminal: %+v (want identified/passed)", res)
	}
	if res.Terminal == nil || res.Terminal.ExitCode != 0 {
		t.Fatalf("exact terminal lost: %+v", res.Terminal)
	}
}

// TestResolveReservationAllocatedIsUnresolved — a manifest carrying the token
// but still in the allocated phase (PGID 0) never published an addressable
// group, so its outcome is genuinely unresolved.
func TestResolveReservationAllocatedIsUnresolved(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	token := "0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f"
	id := "11111111111111111111111111111111"
	runDir := filepath.Join(root, id)
	if err := os.Mkdir(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), &manifestRecord{
		Schema: recordSchema, RunID: id, Root: root, RunDir: runDir,
		Token: token, SupervisorPID: 0, PGID: 0, SID: 0, Phase: "allocated",
	}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ResolveReservation(root, token)
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != "unresolved" || res.RunID != id {
		t.Fatalf("allocated resolve: %+v (want unresolved/%s)", res, id)
	}
}

// TestResolveReservationUnreadableEntryIsUnresolved — an unreadable sibling
// slot is not clean absence: the token we seek could be the one we cannot read,
// so a no-match verdict fails closed to unresolved, never never-launched.
func TestResolveReservationUnreadableEntryIsUnresolved(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	token := "abababababababababababababababab"
	sibID := "22222222222222222222222222222222"
	sibDir := filepath.Join(root, sibID)
	if err := os.Mkdir(sibDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicJSON(filepath.Join(sibDir, manifestFile), &manifestRecord{
		Schema: recordSchema, RunID: sibID, Root: root, RunDir: sibDir,
		Token: "ffffffffffffffffffffffffffffffff", SupervisorPID: 1234, PGID: 1234, Phase: "running",
	}); err != nil {
		t.Fatal(err)
	}
	// Deny traversal so the manifest read fails closed. Restore before teardown
	// removes the fixture.
	if err := os.Chmod(sibDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sibDir, 0o700) })

	res, err := svc.ResolveReservation(root, token)
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != "unresolved" {
		t.Fatalf("unreadable sibling resolve: %+v (want unresolved)", res)
	}
}

// TestResolveReservationRejectsBadToken — an empty or non-hex token is refused
// invalid-input; a lost-launch resolution can only ever be sought by a real
// reservation token.
func TestResolveReservationRejectsBadToken(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	for _, bad := range []string{"", "NOTHEX", "0123XYZ"} {
		_, err := svc.ResolveReservation(root, bad)
		if err == nil {
			t.Fatalf("token %q accepted", bad)
		}
		if f, ok := AsFailure(err); !ok || f.Class != FailInvalidInput {
			t.Fatalf("token %q class = %v, want invalid-input", bad, err)
		}
	}
}

// TestLaunchWritesCallerToken — Launch writes the caller-supplied reservation
// token verbatim into the pre-spawn manifest, and it survives establishment.
func TestLaunchWritesCallerToken(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	token := "feedfacefeedfacefeedfacefeedface"
	out := launchWithToken(t, svc, root, token, "sleep")
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil {
		t.Fatalf("read manifest: %v (m=%v)", err, m)
	}
	if m.Token != token {
		t.Fatalf("manifest token = %q, want caller token %q", m.Token, token)
	}
	killRun(t, out.RunDir)
}

// TestLaunchMintsTokenWhenCallerSuppliesNone — with no caller token the manifest
// still carries a minted 32-hex token (the fallback), never an empty one.
func TestLaunchMintsTokenWhenCallerSuppliesNone(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	out := launchHelper(t, svc, root, "sleep")
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil {
		t.Fatalf("read manifest: %v (m=%v)", err, m)
	}
	if !runIDPattern.MatchString(m.Token) {
		t.Fatalf("minted token = %q, want 32-hex", m.Token)
	}
	killRun(t, out.RunDir)
}
