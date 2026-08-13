package install

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sampleState() *State {
	return &State{
		FormatVersion:  StateFormatVersion,
		ProductVersion: "0.1.0-dev",
		AssetProtocol:  1,
		AssetSetID:     "sha256:deadbeef",
		Mode:           ModeRelease,
		Harnesses:      []string{"claude", "codex"},
		AgentDigest:    "sha256:cafe",
		// Canonical order: sorted by Path, matching what WriteStateAtomic
		// publishes, so a round trip is comparable field for field.
		Targets: []TargetRecord{
			{
				Path:      "/home/u/.claude/CLAUDE.md",
				Kind:      KindManagedBlock,
				BlockName: "dispatch",
				SHA256:    "def",
				Role:      "dispatch",
			},
			{
				Path:   "/home/u/.claude/agents/docket-adr.md",
				Kind:   KindFile,
				SHA256: "abc",
				Role:   "agent-source",
			},
			{
				Path:       "/home/u/.claude/skills/docket-build",
				Kind:       KindSymlink,
				LinkTarget: "/data/versions/sha256-x/assets/skills/docket-build",
				Role:       "skill",
			},
		},
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "install.json")
	want := sampleState()

	if err := WriteStateAtomic(path, want); err != nil {
		t.Fatalf("WriteStateAtomic: %v", err)
	}

	// The state directory is private.
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat state dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("state dir mode = %#o, want 0700", perm)
	}

	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got == nil {
		t.Fatal("LoadState returned nil for a written state")
	}
	if !reflect.DeepEqual(*got, *want) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", *got, *want)
	}
}

// Bytes on disk must be a function of the state's content only, so an
// unchanged installation never rewrites a differently-ordered document.
func TestWriteStateCanonicalOrder(t *testing.T) {
	dir := t.TempDir()
	ordered := sampleState()
	shuffled := sampleState()
	shuffled.Harnesses = []string{"codex", "claude"}
	shuffled.Targets = []TargetRecord{shuffled.Targets[2], shuffled.Targets[1], shuffled.Targets[0]}

	pathA := filepath.Join(dir, "a", "install.json")
	pathB := filepath.Join(dir, "b", "install.json")
	if err := WriteStateAtomic(pathA, ordered); err != nil {
		t.Fatalf("WriteStateAtomic(a): %v", err)
	}
	if err := WriteStateAtomic(pathB, shuffled); err != nil {
		t.Fatalf("WriteStateAtomic(b): %v", err)
	}
	a, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("ReadFile(a): %v", err)
	}
	b, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("ReadFile(b): %v", err)
	}
	if string(a) != string(b) {
		t.Errorf("shuffled input produced different bytes:\n a = %s\n b = %s", a, b)
	}
	if !strings.HasSuffix(string(a), "\n") {
		t.Error("state document must end with a newline")
	}

	// The caller's slices must not be reordered underneath it.
	if shuffled.Harnesses[0] != "codex" {
		t.Errorf("WriteStateAtomic mutated the caller's Harnesses slice: %v", shuffled.Harnesses)
	}
}

func TestLoadStateAbsent(t *testing.T) {
	got, err := LoadState(filepath.Join(t.TempDir(), "state", "install.json"))
	if err != nil {
		t.Fatalf("LoadState(absent): unexpected error %v", err)
	}
	if got != nil {
		t.Fatalf("LoadState(absent) = %+v, want nil", got)
	}
}

func TestLoadStateMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState(malformed): want error, got nil")
	}
}

func TestLoadStateUnknownFormatVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install.json")
	if err := os.WriteFile(path, []byte(`{"format_version":99}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := LoadState(path)
	if err == nil {
		t.Fatal("LoadState(unknown format_version): want error, got nil")
	}
	if !errors.Is(err, ErrStateInvalid) {
		t.Errorf("error %v does not wrap ErrStateInvalid", err)
	}
}

// A failed publish must leave the previous installation record complete: the
// window between "temp written" and "rename done" is the only place torn state
// could appear, so the rename seam is the failure injection point.
func TestWriteStateAtomicNoTorn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "install.json")

	first := sampleState()
	if err := WriteStateAtomic(path, first); err != nil {
		t.Fatalf("WriteStateAtomic(first): %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	boom := errors.New("injected rename failure")
	saved := renameFn
	renameFn = func(oldPath, newPath string) error { return boom }
	t.Cleanup(func() { renameFn = saved })

	second := sampleState()
	second.AssetSetID = "sha256:replacement"
	err = WriteStateAtomic(path, second)
	if err == nil {
		t.Fatal("WriteStateAtomic: want the injected rename error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error %v does not wrap the injected failure", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after failed write: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("state file changed under a failed publish:\nbefore = %s\nafter  = %s", before, after)
	}

	// The abandoned temp file must not be left beside the destination.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "install.json" {
			t.Errorf("leftover file %q in the state directory after a failed publish", e.Name())
		}
	}
}
