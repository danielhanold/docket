package app

import (
	"os"
	"runtime"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// The publish-lock holder re-exec role (change 0494). TestMain routes it before any
// test-suite setup. The child takes the lock at the holder env path, writes
// "ready" to the ready env path, and blocks until killed. It never closes the
// lock: only process death may free it.
const (
	publishLockHolderEnv = "DOCKET_APP_TEST_PUBLISH_LOCK_HOLDER"
	publishLockReadyEnv  = "DOCKET_APP_TEST_PUBLISH_LOCK_READY"
)

func runPublishLockHolder(lockPath, readyPath string) int {
	f, busy, err := process.TryExclusiveLock(lockPath)
	if err != nil || busy {
		return 3
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
		return 4
	}
	for {
		time.Sleep(time.Hour)
		runtime.KeepAlive(f) // keep the descriptor (and so the lock) reachable until death
	}
}
