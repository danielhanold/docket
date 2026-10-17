package suiterunner

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// holdPeerSign plays "another run": it creates a visible sign in dir, takes the
// flock through a separate open file description (so ProbeLock reports held even
// in this process), and writes content (pass "" for none). release removes the
// sign and closes the file (idempotent); it is also registered with t.Cleanup.
func holdPeerSign(t *testing.T, dir, name, content string) (path string, release func()) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("holdPeerSign: mkdir %s: %v", dir, err)
	}
	path = filepath.Join(dir, name)
	f, busy, err := process.TryExclusiveLock(path)
	if err != nil {
		t.Fatalf("holdPeerSign: lock %s: %v", path, err)
	}
	if busy {
		t.Fatalf("holdPeerSign: %s is already held", path)
	}
	if content != "" {
		if _, err := f.WriteString(content); err != nil {
			f.Close()
			t.Fatalf("holdPeerSign: write %s: %v", path, err)
		}
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			os.Remove(path)
			f.Close()
		})
	}
	t.Cleanup(release)
	return path, release
}

// signNamesIn lists the names of dir's entries that satisfy match.
func signNamesIn(t *testing.T, dir string, match func(string) bool) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if match(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestRunSignDir(t *testing.T) {
	if got := runSignDir(""); got != "" {
		t.Fatalf(`runSignDir("") = %q, want ""`, got)
	}
	if got, want := runSignDir("/a/b/state.tsv"), "/a/b/development-test-runs"; got != want {
		t.Fatalf("runSignDir = %q, want %q", got, want)
	}
}

func TestPutUpRunSignIsHeldAndNamed(t *testing.T) {
	dir := filepath.Join(testsupport.TempDir(t), "not", "yet", runSignDirName)
	s, err := putUpRunSign(dir, "/repo/x")
	if err != nil {
		t.Fatalf("putUpRunSign: %v", err)
	}
	t.Cleanup(s.takeDown)
	path := s.Path()
	if filepath.Dir(path) != dir {
		t.Fatalf("sign %s is not in %s", path, dir)
	}
	if !runSignName.MatchString(filepath.Base(path)) {
		t.Fatalf("sign name %q does not match %s", filepath.Base(path), runSignName)
	}
	if got := process.ProbeLock(path); got != process.LockProbeHeld {
		t.Fatalf("a live sign must probe held, got %v", got)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sign: %v", err)
	}
	for _, want := range []string{"worktree=/repo/x\n", "pid=", "started="} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("sign content %q lacks %q", body, want)
		}
	}
	if temps := signNamesIn(t, dir, runSignTempName.MatchString); len(temps) != 0 {
		t.Fatalf("a published sign leaves no temporary file, found %v", temps)
	}

	s.takeDown()
	if got := process.ProbeLock(path); got != process.LockProbeMissing {
		t.Fatalf("a taken-down sign must be gone, probe = %v", got)
	}

	var nilSign *runSign
	nilSign.takeDown()
	if got := nilSign.Path(); got != "" {
		t.Fatalf("nil sign Path = %q, want empty", got)
	}
}

func TestPutUpRunSignNamesAreUnique(t *testing.T) {
	dir := filepath.Join(testsupport.TempDir(t), runSignDirName)
	a, err := putUpRunSign(dir, "/repo/a")
	if err != nil {
		t.Fatalf("putUpRunSign a: %v", err)
	}
	t.Cleanup(a.takeDown)
	b, err := putUpRunSign(dir, "/repo/b")
	if err != nil {
		t.Fatalf("putUpRunSign b: %v", err)
	}
	t.Cleanup(b.takeDown)
	if a.Path() == b.Path() {
		t.Fatalf("two signs share one path %s", a.Path())
	}
}

func TestScanPeers(t *testing.T) {
	const (
		peerA = "run-424242-00000000000000ab.sign"
		peerB = "run-424243-00000000000000cd.sign"
	)
	newDir := func(t *testing.T) string {
		dir := filepath.Join(testsupport.TempDir(t), runSignDirName)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	want := func(t *testing.T, got, want peerScan) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("scanPeers = %+v, want %+v", got, want)
		}
	}

	t.Run("missing dir", func(t *testing.T) {
		dir := filepath.Join(testsupport.TempDir(t), "absent")
		want(t, scanPeers(dir, ""), peerScan{})
	})

	t.Run("empty dir", func(t *testing.T) {
		want(t, scanPeers(newDir(t), ""), peerScan{})
	})

	t.Run("held peer", func(t *testing.T) {
		dir := newDir(t)
		holdPeerSign(t, dir, peerA, "worktree=/w/y\n")
		want(t, scanPeers(dir, ""), peerScan{worktrees: []string{"/w/y"}})
	})

	t.Run("two held peers", func(t *testing.T) {
		dir := newDir(t)
		holdPeerSign(t, dir, peerA, "pid=424242\nworktree=/w/z\n")
		holdPeerSign(t, dir, peerB, "worktree=/w/a\n")
		want(t, scanPeers(dir, ""), peerScan{worktrees: []string{"/w/a", "/w/z"}})
	})

	t.Run("self excluded", func(t *testing.T) {
		dir := newDir(t)
		self, _ := holdPeerSign(t, dir, peerA, "worktree=/w/self\n")
		want(t, scanPeers(dir, self), peerScan{})
	})

	t.Run("dead visible sign", func(t *testing.T) {
		dir := newDir(t)
		dead := filepath.Join(dir, peerA)
		if err := os.WriteFile(dead, []byte("worktree=/w/dead\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		want(t, scanPeers(dir, ""), peerScan{})
		if _, err := os.Lstat(dead); !os.IsNotExist(err) {
			t.Fatalf("a dead sign must be deleted by the scan, lstat err = %v", err)
		}
	})

	t.Run("held temp ignored", func(t *testing.T) {
		dir := newDir(t)
		holdPeerSign(t, dir, ".tmp-"+peerA, "worktree=/w/tmp\n")
		want(t, scanPeers(dir, ""), peerScan{})
	})

	t.Run("non-matching name ignored", func(t *testing.T) {
		dir := newDir(t)
		holdPeerSign(t, dir, "notes.txt", "worktree=/w/notes\n")
		want(t, scanPeers(dir, ""), peerScan{})
	})

	t.Run("directory with a sign name ignored", func(t *testing.T) {
		dir := newDir(t)
		if err := os.Mkdir(filepath.Join(dir, "run-1-0000000000000001.sign"), 0o755); err != nil {
			t.Fatal(err)
		}
		want(t, scanPeers(dir, ""), peerScan{})
	})

	t.Run("held sign without worktree line", func(t *testing.T) {
		dir := newDir(t)
		holdPeerSign(t, dir, peerA, "")
		want(t, scanPeers(dir, ""), peerScan{worktrees: []string{peerA}})
	})

	t.Run("unreadable dir", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		dir := newDir(t)
		holdPeerSign(t, dir, peerA, "worktree=/w/y\n")
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		if got := scanPeers(dir, ""); !got.unknown {
			t.Fatalf("an unreadable sign folder must be can't-tell, got %+v", got)
		}
	})

	t.Run("unreadable sign", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses file permissions")
		}
		dir := newDir(t)
		sign := filepath.Join(dir, peerA)
		if err := os.WriteFile(sign, []byte("worktree=/w/y\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(sign, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(sign, 0o600) })
		if got := scanPeers(dir, ""); !got.unknown {
			t.Fatalf("an unprobeable sign must be can't-tell, got %+v", got)
		}
		if _, err := os.Lstat(sign); err != nil {
			t.Fatalf("an unprobeable sign must be left in place, lstat err = %v", err)
		}
	})
}

func TestScanPeersSweepsStaleTemps(t *testing.T) {
	dir := filepath.Join(testsupport.TempDir(t), runSignDirName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-(runSignTempMaxAge + time.Minute))

	stale := filepath.Join(dir, ".tmp-run-11-0000000000000011.sign")
	if err := os.WriteFile(stale, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	young := filepath.Join(dir, ".tmp-run-12-0000000000000012.sign")
	if err := os.WriteFile(young, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	heldOld, _ := holdPeerSign(t, dir, ".tmp-run-13-0000000000000013.sign", "worktree=/w/publishing\n")
	if err := os.Chtimes(heldOld, old, old); err != nil {
		t.Fatal(err)
	}

	if got := scanPeers(dir, ""); !reflect.DeepEqual(got, peerScan{}) {
		t.Fatalf("temporary files never count, got %+v", got)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp swept: an old unlocked temp must be removed, lstat err = %v", err)
	}
	if _, err := os.Lstat(young); err != nil {
		t.Fatalf("young temp kept: a fresh temp must survive, lstat err = %v", err)
	}
	if _, err := os.Lstat(heldOld); err != nil {
		t.Fatalf("a held temp must survive however old, lstat err = %v", err)
	}
}

func TestPeerScanDescribe(t *testing.T) {
	cases := []struct {
		name string
		scan peerScan
		want string
	}{
		{"worktrees", peerScan{worktrees: []string{"/a", "/b"}}, "another suite was running (/a, /b)"},
		{"unknown", peerScan{unknown: true}, "the runner could not check for other suites"},
		{"both", peerScan{worktrees: []string{"/a"}, unknown: true}, "another suite was running (/a); could not check for other suites"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.scan.describe(); got != c.want {
				t.Fatalf("describe = %q, want %q", got, c.want)
			}
			if !c.scan.seen() {
				t.Fatalf("%+v must read as seen", c.scan)
			}
		})
	}
	if (peerScan{}).seen() {
		t.Fatal("the zero scan must not read as seen")
	}

	t.Run("merge", func(t *testing.T) {
		got := peerScan{worktrees: []string{"/b", "/a"}}.merge(peerScan{worktrees: []string{"/a", "/c"}, unknown: true})
		want := peerScan{worktrees: []string{"/a", "/b", "/c"}, unknown: true}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("merge = %+v, want %+v", got, want)
		}
		if got := (peerScan{}).merge(peerScan{}); !reflect.DeepEqual(got, peerScan{}) {
			t.Fatalf("merging two clean scans = %+v, want the zero scan", got)
		}
	})
}
