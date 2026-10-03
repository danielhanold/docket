//go:build integration

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationRunCancelPublishLockFreedWhenPublisherIsSIGKILLed (change 0494,
// spec test 8): a real child process holds a publish lock (a live publisher), so the
// classifier blocks with mutation-pending. After SIGKILL, with no handler possible,
// the kernel frees the lock, the classifier reports mutation-abandoned, and the lock
// file still exists.
func TestIntegrationRunCancelPublishLockFreedWhenPublisherIsSIGKILLed(t *testing.T) {
	dir := testsupport.TempDir(t)
	token := strings.Repeat("9f", 16)
	lockPath, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatal("publishLockPath rejected a valid token")
	}
	ready := filepath.Join(dir, "ready")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = append(os.Environ(), publishLockHolderEnv+"="+lockPath, publishLockReadyEnv+"="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start holder: %v", err)
	}
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder never reported ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	m := AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: token}
	if blocks, f := classifyAdmittedMutation(dir, m); !blocks || f != "mutation-pending:"+OperationWorkspacePublish {
		t.Fatalf("live publisher classified (%v, %q), want blocking mutation-pending", blocks, f)
	}

	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL: %v", err)
	}
	_ = cmd.Wait() // reaped: the kernel has released the child's lock
	reaped = true

	if blocks, f := classifyAdmittedMutation(dir, m); blocks || f != "mutation-abandoned:"+OperationWorkspacePublish {
		t.Fatalf("killed publisher classified (%v, %q), want non-blocking mutation-abandoned", blocks, f)
	}
	if blocked, fs := accountAdmittedMutations(dir, []AdmittedMutation{m}); blocked || len(fs) != 1 {
		t.Fatalf("accountAdmittedMutations = (%v, %v), want one non-blocking finding", blocked, fs)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("the lock file must survive its holder's death: %v", err)
	}
}
