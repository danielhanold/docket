// Package testsupport is the shared real-process test fixture (change 0373).
// It is imported ONLY from _test.go files. It replaces bare t.TempDir() in
// packages whose tests spawn real git or supervisor processes: t.TempDir's
// cleanup is one os.RemoveAll with no retry, so a detached writer that
// outlives the last assertion produces "directory not empty" under parallel
// load. This fixture drains registered writers first, then retries removal
// over a bounded tolerance window, and on final failure fails the test
// naming the surviving paths — a genuine leak surfaces as a finding, not an
// opaque teardown crash.
package testsupport

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitbg"
)

// cleanupTolerance bounds the RemoveAll retry window.
// MEASURED (change 0373, Task 9) on Darwin arm64, hw.ncpu=11, gate -j=11
// (`development test` default = runtime.NumCPU()). Four full-gate runs were
// instrumented (temporary retry-count logging in removeAllTolerant, since
// removed): across all 18,796 fixture cleanups NOT ONE needed a retry — the
// drain-before-removal wiring makes the first os.RemoveAll succeed every time,
// so the retry window is a never-exercised safety net. The longest single
// drain-to-removable interval observed under load was 945ms (heaviest sweep
// candidate) and 391ms under the pinned candidate. 4s is >=4x the heaviest
// observation and ~10x the pinned one, clearing the 2s floor. Too tight
// flakes; too loose only costs wall clock on a genuine leak, which fails anyway.
const cleanupTolerance = 4 * time.Second

const cleanupStep = 10 * time.Millisecond

var (
	drainMu sync.Mutex
	drains  = map[testing.TB][]func(){}
)

// DrainOnCleanup registers fn to run before this test's fixture dirs are
// removed. Drains run once, in registration order, from the first removal
// cleanup that fires (t.Cleanup is LIFO and TempDir is called before the
// spawn being drained, so removal cleanups run after any t.Cleanup the test
// registered later — drains still run first because removal invokes them).
func DrainOnCleanup(t testing.TB, fn func()) {
	t.Helper()
	drainMu.Lock()
	defer drainMu.Unlock()
	drains[t] = append(drains[t], fn)
}

func runDrains(t testing.TB) {
	drainMu.Lock()
	fns := drains[t]
	delete(drains, t)
	drainMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// TempDir is the fixture's drop-in for t.TempDir(). The directory name
// carries the test name so a surviving dir on the failure path is
// attributable (diagnostic naming).
func TempDir(t testing.TB) string {
	t.Helper()
	pattern := "docketfix-" + sanitize(t.Name()) + "-*"
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatalf("testsupport: MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		runDrains(t)
		removeAllTolerant(t, dir)
	})
	return dir
}

func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		}
		return '_'
	}, name)
}

// removeAllTolerant retries os.RemoveAll over cleanupTolerance, absorbing
// trailing writes from a draining child. On final failure it fails the test
// and names the surviving paths.
func removeAllTolerant(t testing.TB, dir string) {
	deadline := time.Now().Add(cleanupTolerance)
	var lastErr error
	for {
		lastErr = os.RemoveAll(dir)
		if lastErr == nil {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(cleanupStep)
	}
	var survivors []string
	_ = filepath.WalkDir(dir, func(p string, _ fs.DirEntry, err error) error {
		if err == nil && p != dir {
			survivors = append(survivors, p)
		}
		return nil
	})
	t.Errorf("testsupport: fixture dir not removable after %v: %v; surviving paths: %v",
		cleanupTolerance, lastErr, survivors)
}

// GitEnv returns the env override pointing spawned git processes at a
// per-fixture global config: the synthetic identity plus gitbg.BackgroundOff.
// Pass it (appended last) to any exec'd command that may run git.
func GitEnv(t testing.TB) []string {
	t.Helper()
	dir := TempDir(t)
	cfg := "[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n" + gitbg.BackgroundOff
	path := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatalf("testsupport: write git config: %v", err)
	}
	return []string{"GIT_CONFIG_GLOBAL=" + path}
}

// WaitQuiesced polls probe every step until it reports true or deadline
// elapses; returns whether the probe became true. This is the generalized
// loop of internal/process's quiesceRun ("a free live.lock proves the run
// dir is quiescent"): callers supply the domain-specific probe.
func WaitQuiesced(deadline time.Duration, step time.Duration, probe func() bool) bool {
	end := time.Now().Add(deadline)
	for {
		if probe() {
			return true
		}
		if time.Now().After(end) {
			return false
		}
		time.Sleep(step)
	}
}
