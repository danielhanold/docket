package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// validManifest builds a fully valid manifest whose ID is the sha256 of its
// feature ref, so writing it into workspaceDir(commonDir, ref) and reading it
// back round-trips. commonDir and path are absolute (canonical form).
func validManifest(commonDir string, ref gitcli.RefName) Manifest {
	return Manifest{
		Schema:     manifestSchemaVersion,
		ID:         workspaceID(ref),
		CommonDir:  commonDir,
		ChangeID:   7,
		Slug:       "fix-the-thing",
		FeatureRef: ref,
		BaseRef:    "refs/heads/main",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		Path:       filepath.Join(commonDir, ".worktrees", "fix-the-thing"),
		Phase:      PhaseReady,
		CreatedUTC: "2026-08-16T12:00:00Z",
		UpdatedUTC: "2026-08-16T12:30:00Z",
	}
}

// TestManifestRoundTrip proves a full manifest survives write→load byte-equal.
func TestManifestRoundTrip(t *testing.T) {
	commonDir := testsupport.TempDir(t)
	ref := gitcli.RefName("refs/heads/feat/fix-the-thing")
	dir := workspaceDir(commonDir, ref)
	want := validManifest(commonDir, ref)

	if err := writeManifest(dir, want); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	got, present, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if !present {
		t.Fatalf("loadManifest reported absent for a written manifest")
	}
	if got != want {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// TestWorkspaceDirHashed proves the directory basename is exactly the hex
// sha256 of the feature ref and differs across refs — no caller-derived branch
// string appears in the path component.
func TestWorkspaceDirHashed(t *testing.T) {
	commonDir := "/some/common"
	ref := gitcli.RefName("refs/heads/feat/x")
	sum := sha256.Sum256([]byte(ref))
	wantBase := hex.EncodeToString(sum[:])

	dir := workspaceDir(commonDir, ref)
	if base := filepath.Base(dir); base != wantBase {
		t.Fatalf("workspace dir base = %q, want %q", base, wantBase)
	}
	if got := filepath.Dir(dir); got != workspacesRoot(commonDir) {
		t.Fatalf("workspace dir parent = %q, want root %q", got, workspacesRoot(commonDir))
	}
	// The slug string never appears literally in the path.
	if got := filepath.Base(dir); got == "x" || got == "feat" {
		t.Fatalf("path component leaks branch string: %q", got)
	}
	// A different ref yields a different directory.
	other := workspaceDir(commonDir, gitcli.RefName("refs/heads/feat/x2"))
	if other == dir {
		t.Fatalf("distinct refs collided onto one dir: %q", dir)
	}
}

// writeRaw writes bytes as the manifest file directly, bypassing writeManifest's
// validation, so a corruption case can be constructed on disk.
func writeRaw(t *testing.T, dir string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), data, 0o600); err != nil {
		t.Fatalf("write raw manifest: %v", err)
	}
}

// mustJSON marshals a manifest to bytes without going through validation.
func mustJSON(t *testing.T, m Manifest) []byte {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

// TestLoadManifestThreeOutcomes proves the (m, present, err) contract: clean
// absence is (false, nil); every corruption is an error and NEVER (false, nil).
func TestLoadManifestThreeOutcomes(t *testing.T) {
	commonDir := testsupport.TempDir(t)
	ref := gitcli.RefName("refs/heads/feat/fix-the-thing")

	// Absent directory and absent file both read as cleanly absent.
	t.Run("absent-dir", func(t *testing.T) {
		_, present, err := loadManifest(filepath.Join(commonDir, "nope"))
		if present || err != nil {
			t.Fatalf("absent dir: present=%v err=%v, want false/nil", present, err)
		}
	})
	t.Run("absent-file", func(t *testing.T) {
		dir := testsupport.TempDir(t) // exists but has no manifest
		_, present, err := loadManifest(dir)
		if present || err != nil {
			t.Fatalf("absent file: present=%v err=%v, want false/nil", present, err)
		}
	})

	// Each corruption must error and must not read as clean absence.
	corrupt := map[string][]byte{
		"truncated-json": []byte(`{"schema":1,"id":`),
	}
	badSchema := validManifest(commonDir, ref)
	badSchema.Schema = 99
	corrupt["unknown-schema"] = mustJSON(t, badSchema)

	badCommon := validManifest(commonDir, ref)
	badCommon.CommonDir = ""
	corrupt["empty-common-dir"] = mustJSON(t, badCommon)

	badPhase := validManifest(commonDir, ref)
	badPhase.Phase = "weird"
	corrupt["bad-phase"] = mustJSON(t, badPhase)

	badCommit := validManifest(commonDir, ref)
	badCommit.BaseCommit = "nothex"
	corrupt["bad-base-commit"] = mustJSON(t, badCommit)

	for name, data := range corrupt {
		t.Run(name, func(t *testing.T) {
			dir := testsupport.TempDir(t)
			writeRaw(t, dir, data)
			_, present, err := loadManifest(dir)
			if err == nil {
				t.Fatalf("%s: expected error, got present=%v", name, present)
			}
			if present {
				t.Fatalf("%s: present=true alongside error", name)
			}
		})
	}

	// An unreadable directory is an error, never clean absence.
	t.Run("unreadable-dir", func(t *testing.T) {
		commonDir := testsupport.TempDir(t)
		dir := workspaceDir(commonDir, ref)
		if err := writeManifest(dir, validManifest(commonDir, ref)); err != nil {
			t.Fatalf("writeManifest: %v", err)
		}
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatalf("chmod 000: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		_, present, err := loadManifest(dir)
		if err == nil {
			t.Fatalf("unreadable dir: expected error, got present=%v", present)
		}
		if present {
			t.Fatalf("unreadable dir: present=true alongside error")
		}
	})
}

// TestWriteManifestNoTempSurvives proves an atomic write leaves no *.tmp sibling.
func TestWriteManifestNoTempSurvives(t *testing.T) {
	commonDir := testsupport.TempDir(t)
	ref := gitcli.RefName("refs/heads/feat/fix-the-thing")
	dir := workspaceDir(commonDir, ref)
	if err := writeManifest(dir, validManifest(commonDir, ref)); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != manifestFileName {
			t.Fatalf("unexpected sibling after atomic write: %q", e.Name())
		}
	}
}

// TestManifestModes proves the workspace dir is 0700 and the manifest file is
// 0600 by EXPLICIT chmod, not umask luck: it asserts under a restrictive umask
// (077) and again under a permissive one (022), where a create-mode of 0755/0644
// relying on umask would be observably too open.
func TestManifestModes(t *testing.T) {
	ref := gitcli.RefName("refs/heads/feat/fix-the-thing")
	for _, um := range []int{0o077, 0o022} {
		old := syscall.Umask(um)
		func() {
			defer syscall.Umask(old)
			commonDir := testsupport.TempDir(t)
			dir := workspaceDir(commonDir, ref)
			if err := writeManifest(dir, validManifest(commonDir, ref)); err != nil {
				t.Fatalf("umask %#o: writeManifest: %v", um, err)
			}
			di, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("stat dir: %v", err)
			}
			if got := di.Mode().Perm(); got != 0o700 {
				t.Fatalf("umask %#o: dir mode = %#o, want 0700", um, got)
			}
			fi, err := os.Stat(filepath.Join(dir, manifestFileName))
			if err != nil {
				t.Fatalf("stat file: %v", err)
			}
			if got := fi.Mode().Perm(); got != 0o600 {
				t.Fatalf("umask %#o: file mode = %#o, want 0600", um, got)
			}
		}()
	}
}

// TestPhaseTransitions proves the monotonic phase rules: allocating→ready and
// ready→cleaned advance; cleaned→ready and ready→allocating are refused.
func TestPhaseTransitions(t *testing.T) {
	cases := []struct {
		from, to Phase
		want     bool
	}{
		{PhaseAllocating, PhaseReady, true},
		{PhaseReady, PhaseCleaned, true},
		{PhaseCleaned, PhaseReady, false},
		{PhaseReady, PhaseAllocating, false},
		{PhaseAllocating, PhaseCleaned, false},
		{PhaseCleaned, PhaseCleaned, false},
	}
	for _, c := range cases {
		if got := c.from.canAdvanceTo(c.to); got != c.want {
			t.Errorf("%s→%s canAdvanceTo = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
