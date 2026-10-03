package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// overrideSeam swaps a package-level seam for the test and restores it on cleanup.
// Tests that use it must not call t.Parallel.
func overrideSeam[T any](t *testing.T, target *T, v T) {
	t.Helper()
	old := *target
	*target = v
	t.Cleanup(func() { *target = old })
}

// lockedTempFile makes a real publish lock for token in dir. hold keeps it held by
// this process until cleanup; !hold leaves the file present and free.
func lockedTempFile(t *testing.T, dir, token string, hold bool) {
	t.Helper()
	path, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatalf("publishLockPath rejected %q", token)
	}
	f, busy, err := process.TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	if !hold {
		f.Close()
		return
	}
	t.Cleanup(func() { f.Close() })
}

func TestValidPublishLockToken(t *testing.T) {
	good := strings.Repeat("0123456789abcdef", 2)
	if !validPublishLockToken(good) {
		t.Fatalf("%q must be valid", good)
	}
	for _, bad := range []string{"", good[:31], good + "0", strings.ToUpper(good),
		"../../../../../../../../tmp/xx", strings.Repeat("g", 32), good[:30] + "/x"} {
		if validPublishLockToken(bad) {
			t.Fatalf("%q must be invalid", bad)
		}
	}
	if p, ok := publishLockPath("/d", good); !ok || p != filepath.Join("/d", "publish-"+good+".lock") {
		t.Fatalf("publishLockPath = %q %v", p, ok)
	}
	if _, ok := publishLockPath("", good); ok {
		t.Fatal("an empty run dir must yield no path")
	}
	if _, ok := publishLockPath("/d", "../x"); ok {
		t.Fatal("a malformed token must yield no path")
	}
}

// TestClassifyAdmittedMutationMatrix is spec section 2's table, plus Review Focus 1–3.
func TestClassifyAdmittedMutationMatrix(t *testing.T) {
	const op = OperationWorkspacePublish
	pending, abandoned := "mutation-pending:"+op, "mutation-abandoned:"+op
	tokFree := strings.Repeat("a1", 16)
	tokHeld := strings.Repeat("b2", 16)
	tokMissing := strings.Repeat("c3", 16)
	tokDir := strings.Repeat("d4", 16)
	dir := testsupport.TempDir(t)
	lockedTempFile(t, dir, tokFree, false)
	lockedTempFile(t, dir, tokHeld, true)
	dirPath, _ := publishLockPath(dir, tokDir)
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		dir        string
		m          AdmittedMutation
		wantBlocks bool
		wantFind   string
	}{
		{"completed", dir, AdmittedMutation{OpKey: op, Status: mutationStatusCompleted}, false, ""},
		{"completed with a held lock", dir, AdmittedMutation{OpKey: op, Status: mutationStatusCompleted, LockToken: tokHeld}, false, ""},
		{"uncertain", dir, AdmittedMutation{OpKey: op, Status: mutationStatusUncertain}, false, abandoned},
		{"uncertain with a held lock", dir, AdmittedMutation{OpKey: op, Status: mutationStatusUncertain, LockToken: tokHeld}, false, abandoned},
		{"admitted, lock free", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokFree}, false, abandoned},
		{"admitted, lock held", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokHeld}, true, pending},
		{"admitted, lock file missing", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokMissing}, true, pending},
		{"admitted, lock path is a directory", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokDir}, true, pending},
		{"admitted, no token", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted}, true, pending},
		{"admitted, malformed traversal token", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: "../../../../../../../../tmp/xx"}, true, pending},
		{"admitted, uppercase token", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: strings.ToUpper(tokFree)}, true, pending},
		{"admitted, empty run dir", "", AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokFree}, true, pending},
		{"unknown status, lock free", dir, AdmittedMutation{OpKey: op, Status: "bogus", LockToken: tokFree}, true, pending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks, find := classifyAdmittedMutation(tc.dir, tc.m)
			if blocks != tc.wantBlocks || find != tc.wantFind {
				t.Fatalf("classify = (%v, %q), want (%v, %q)", blocks, find, tc.wantBlocks, tc.wantFind)
			}
		})
	}
}

// TestClassifyAdmittedMutationProbeErrorBlocks: a probe error (seam) blocks, and a
// malformed token never reaches the probe at all.
func TestClassifyAdmittedMutationProbeErrorBlocks(t *testing.T) {
	var probed []string
	overrideSeam(t, &publishLockProbe, func(p string) process.LockProbe {
		probed = append(probed, p)
		return process.LockProbeUnknown
	})
	dir := testsupport.TempDir(t)
	tok := strings.Repeat("e5", 16)
	lockedTempFile(t, dir, tok, false)
	m := AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, LockToken: tok}
	if blocks, find := classifyAdmittedMutation(dir, m); !blocks || find != "mutation-pending:"+OperationPRPublish {
		t.Fatalf("probe error classified (%v, %q), want blocking mutation-pending", blocks, find)
	}
	probed = nil
	bad := AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, LockToken: "../x"}
	if blocks, _ := classifyAdmittedMutation(dir, bad); !blocks {
		t.Fatal("malformed token must block")
	}
	if len(probed) != 0 {
		t.Fatalf("a malformed token reached the probe: %v", probed)
	}
}

func TestAccountAdmittedMutationsAggregates(t *testing.T) {
	dir := testsupport.TempDir(t)
	tok := strings.Repeat("f6", 16)
	lockedTempFile(t, dir, tok, false)
	muts := []AdmittedMutation{
		{OpKey: OperationPRPublish, Status: mutationStatusCompleted},
		{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok},
		{OpKey: OperationPRPublish, Status: mutationStatusUncertain},
	}
	blocked, findings := accountAdmittedMutations(dir, muts)
	want := []string{"mutation-abandoned:" + OperationWorkspacePublish, "mutation-abandoned:" + OperationPRPublish}
	if blocked || strings.Join(findings, ",") != strings.Join(want, ",") {
		t.Fatalf("account = (%v, %v), want (false, %v)", blocked, findings, want)
	}
	blocked, findings = accountAdmittedMutations(dir, append(muts, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted}))
	if !blocked || findings[len(findings)-1] != "mutation-pending:"+OperationPRPublish {
		t.Fatalf("a lockless admitted entry must block: (%v, %v)", blocked, findings)
	}
}

// TestAccountAdmittedMutationsReloadsBeforeAbandoned is the read-then-probe race:
// the journal was read with the entry admitted, then its publisher wrote completed
// and released its lock before the probe. The re-read sees completed, so no
// mutation-abandoned is reported. Every other re-read answer keeps today's finding.
func TestAccountAdmittedMutationsReloadsBeforeAbandoned(t *testing.T) {
	const op = OperationPRPublish
	abandoned := []string{"mutation-abandoned:" + op}
	dir := testsupport.TempDir(t)
	tok := strings.Repeat("a7", 16)
	lockedTempFile(t, dir, tok, false) // the publisher released its lock
	read := []AdmittedMutation{{OpKey: op, Status: mutationStatusAdmitted, LockToken: tok}}
	stored := func(status, token string) func() ([]AdmittedMutation, error) {
		return func() ([]AdmittedMutation, error) {
			return []AdmittedMutation{{OpKey: op, Status: status, LockToken: token, Verified: status == mutationStatusCompleted}}, nil
		}
	}
	cases := []struct {
		name   string
		reload func() ([]AdmittedMutation, error)
		want   []string
	}{
		{"now completed", stored(mutationStatusCompleted, tok), nil},
		{"now uncertain", stored(mutationStatusUncertain, tok), abandoned},
		{"still admitted", stored(mutationStatusAdmitted, tok), abandoned},
		{"completed under another token", stored(mutationStatusCompleted, strings.Repeat("b8", 16)), abandoned},
		{"entry gone", func() ([]AdmittedMutation, error) { return nil, nil }, abandoned},
		{"reload error", func() ([]AdmittedMutation, error) { return nil, errors.New("unreadable") }, abandoned},
		{"no reload", nil, abandoned},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocked, findings := accountAdmittedMutationsReloading(dir, read, tc.reload)
			if blocked || strings.Join(findings, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("account = (%v, %v), want (false, %v)", blocked, findings, tc.want)
			}
		})
	}
}

func TestAcquirePublishLock(t *testing.T) {
	dir := testsupport.TempDir(t)
	tok := acquirePublishLock(dir)
	if !validPublishLockToken(tok) {
		t.Fatalf("acquire = %q, want a valid token and a held lock", tok)
	}
	path, _ := publishLockPath(dir, tok)
	if got := process.ProbeLock(path); got != process.LockProbeHeld {
		t.Fatalf("acquired lock probed %v, want held", got)
	}
	releasePublishLock(tok)
	if got := process.ProbeLock(path); got != process.LockProbeFree {
		t.Fatalf("released lock probed %v, want free (and the file still present)", got)
	}
	releasePublishLock(tok) // idempotent
	releasePublishLock("")

	t.Run("mint failure", func(t *testing.T) {
		overrideSeam(t, &publishLockMintToken, func() (string, error) { return "", errors.New("no entropy") })
		if tok := acquirePublishLock(dir); tok != "" {
			t.Fatalf("mint failure = %q, want \"\"", tok)
		}
	})
	t.Run("acquire error", func(t *testing.T) {
		overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, false, errors.New("EIO") })
		if tok := acquirePublishLock(dir); tok != "" {
			t.Fatalf("acquire error = %q, want \"\"", tok)
		}
	})
	t.Run("busy on a fresh token", func(t *testing.T) {
		overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, true, nil })
		if tok := acquirePublishLock(dir); tok != "" {
			t.Fatalf("busy = %q, want \"\"", tok)
		}
	})
	t.Run("empty dir", func(t *testing.T) {
		if tok := acquirePublishLock(""); tok != "" {
			t.Fatalf("empty dir = %q, want \"\"", tok)
		}
	})
}

// assertLockSurvivesGC collects garbage repeatedly, giving any finalizer time to
// run, and fails the moment the lock at path stops probing held. An unreleased
// publish lock must stay held until the process exits, never until GC.
func assertLockSurvivesGC(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 20; i++ {
		runtime.GC()
		runtime.GC()
		time.Sleep(5 * time.Millisecond)
		if got := process.ProbeLock(path); got != process.LockProbeHeld {
			t.Fatalf("after GC round %d the unreleased lock probed %v, want held (a dropped handle read as a dead publisher)", i, got)
		}
	}
}

// TestAcquirePublishLockSurvivesDroppedHandle: dropping everything acquirePublishLock
// returned (only the token is kept) never frees the lock; only releasePublishLock does.
func TestAcquirePublishLockSurvivesDroppedHandle(t *testing.T) {
	dir := testsupport.TempDir(t)
	tok := acquirePublishLock(dir)
	if !validPublishLockToken(tok) {
		t.Fatalf("acquire token = %q, want a valid token", tok)
	}
	path, _ := publishLockPath(dir, tok)
	assertLockSurvivesGC(t, path)
	releasePublishLock(tok)
	if got := process.ProbeLock(path); got != process.LockProbeFree {
		t.Fatalf("released lock probed %v, want free", got)
	}
}
