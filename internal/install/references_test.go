package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func referenceRoots(t *testing.T) UserRoots {
	t.Helper()
	base := testsupport.TempDir(t)
	return UserRoots{
		Home:       base,
		DataRoot:   filepath.Join(base, "data"),
		ConfigHome: filepath.Join(base, "config"),
		BinDir:     filepath.Join(base, "bin"),
	}
}

// referenceAsset makes a real immediate version root. Reference derivation is
// deliberately less trusting than version-tree adoption, but containment still
// rests on a real, non-link directory immediately below versions/.
func referenceAsset(t *testing.T, roots UserRoots, id, name string) string {
	t.Helper()
	p := filepath.Join(roots.VersionDir(id), name)
	writeFileOrDie(t, p, "asset\n")
	return p
}

func referenceState(assetSetID string, records ...TargetRecord) *State {
	return &State{
		FormatVersion:  StateFormatVersion,
		ProductVersion: "0.1.0-test",
		AssetProtocol:  1,
		AssetSetID:     assetSetID,
		Mode:           ModeRelease,
		AgentDigest:    "sha256:test",
		Targets:        records,
	}
}

func symlinkRecord(path, destination, harness string) TargetRecord {
	return TargetRecord{
		Path:       path,
		Kind:       KindSymlink,
		LinkTarget: destination,
		Role:       "skill",
		Harness:    harness,
	}
}

func requireReferences(t *testing.T, got ReferenceSet, roots UserRoots, ids ...string) {
	t.Helper()
	if len(got) != len(ids) {
		t.Fatalf("reference count = %d, want %d: %#v", len(got), len(ids), got)
	}
	for _, id := range ids {
		root, err := canonicalPath(filepath.Dir(roots.VersionDir(id)))
		if err != nil {
			t.Fatalf("canonical version root %q: %v", id, err)
		}
		if _, ok := got[root]; !ok {
			t.Errorf("references missing %q: %#v", root, got)
		}
	}
}

func TestDeriveVersionReferencesRetainsRecordedAndObservedLinks(t *testing.T) {
	roots := referenceRoots(t)
	_ = referenceAsset(t, roots, "current", "skills/current")
	claudeRecorded := referenceAsset(t, roots, "claude-old", "skills/docket-claude")
	codexRecorded := referenceAsset(t, roots, "codex-old", "skills/docket-codex")
	claudeObserved := referenceAsset(t, roots, "claude-new", "skills/docket-claude")

	claudeTarget := filepath.Join(roots.Home, ".claude", "skills", "docket-claude")
	codexTarget := filepath.Join(roots.Home, ".codex", "skills", "docket-codex")
	// A scoped upgrade has written the new Claude destination, while the
	// carried record and live Codex target still anchor older trees. All three
	// old/live roots must survive the collection calculation.
	symlinkOrDie(t, claudeObserved, claudeTarget)
	symlinkOrDie(t, codexRecorded, codexTarget)
	state := referenceState("current",
		symlinkRecord(claudeTarget, claudeRecorded, "claude"),
		symlinkRecord(codexTarget, codexRecorded, "codex"),
	)
	state.Harnesses = []string{"claude", "codex"}

	got, err := DeriveVersionReferences(roots, state)
	if err != nil {
		t.Fatalf("DeriveVersionReferences: %v", err)
	}
	requireReferences(t, got, roots, "current", "claude-old", "claude-new", "codex-old")
}

func TestDeriveVersionReferencesPreservesMissingAndReplacedRecordedTargets(t *testing.T) {
	roots := referenceRoots(t)
	_ = referenceAsset(t, roots, "current", "skills/current")
	missingDestination := referenceAsset(t, roots, "missing", "skills/missing")
	replacedDestination := referenceAsset(t, roots, "replaced", "skills/replaced")

	missingTarget := filepath.Join(roots.Home, ".claude", "missing")
	replacedTarget := filepath.Join(roots.Home, ".codex", "replaced")
	writeFileOrDie(t, replacedTarget, "a user replaced this symlink\n")
	state := referenceState("current",
		symlinkRecord(missingTarget, missingDestination, "claude"),
		symlinkRecord(replacedTarget, replacedDestination, "codex"),
	)
	state.Harnesses = []string{"claude", "codex"}

	got, err := DeriveVersionReferences(roots, state)
	if err != nil {
		t.Fatalf("DeriveVersionReferences: %v", err)
	}
	requireReferences(t, got, roots, "current", "missing", "replaced")
}

func TestDeriveVersionReferencesResolvesRelativeDanglingAndMultiHopLinks(t *testing.T) {
	roots := referenceRoots(t)
	_ = referenceAsset(t, roots, "current", "skills/current")
	relativeDestination := referenceAsset(t, roots, "relative", "skills/relative")
	multiHopDestination := referenceAsset(t, roots, "multi-hop", "skills/multi-hop")
	danglingRoot := filepath.Join(roots.VersionsDir(), "dangling")
	if err := os.MkdirAll(danglingRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll dangling root: %v", err)
	}
	danglingDestination := filepath.Join(danglingRoot, "assets", "skills", "not-yet-extracted")

	relativeTarget := filepath.Join(roots.Home, ".claude", "relative")
	danglingTarget := filepath.Join(roots.Home, ".claude", "dangling")
	multiHopTarget := filepath.Join(roots.Home, ".claude", "multi-hop")
	symlinkOrDie(t, filepath.Join("..", "data", "versions", "relative", "assets", "skills", "relative"), relativeTarget)
	symlinkOrDie(t, danglingDestination, danglingTarget)
	firstHop := filepath.Join(roots.Home, ".claude", "first-hop")
	secondHop := filepath.Join(roots.Home, ".claude", "second-hop")
	symlinkOrDie(t, secondHop, firstHop)
	symlinkOrDie(t, multiHopDestination, secondHop)
	symlinkOrDie(t, firstHop, multiHopTarget)

	state := referenceState("current",
		symlinkRecord(relativeTarget, relativeDestination, "claude"),
		symlinkRecord(danglingTarget, danglingDestination, "claude"),
		symlinkRecord(multiHopTarget, multiHopDestination, "claude"),
	)
	state.Harnesses = []string{"claude"}
	got, err := DeriveVersionReferences(roots, state)
	if err != nil {
		t.Fatalf("DeriveVersionReferences: %v", err)
	}
	requireReferences(t, got, roots, "current", "relative", "dangling", "multi-hop")
}

func TestDeriveVersionReferencesCanonicalizesTmpAliases(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("/tmp and /private/tmp are aliases on macOS")
	}
	base, err := os.MkdirTemp("/tmp", "docket-reference-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	roots := UserRoots{Home: base, DataRoot: filepath.Join(base, "data")}
	_ = referenceAsset(t, roots, "current", "skills/current")
	destination := referenceAsset(t, roots, "old", "skills/old")
	target := filepath.Join(base, "target")
	privateDestination := filepath.Join("/private", destination)
	symlinkOrDie(t, privateDestination, target)
	state := referenceState("current", symlinkRecord(target, privateDestination, ""))

	got, err := DeriveVersionReferences(roots, state)
	if err != nil {
		t.Fatalf("DeriveVersionReferences: %v", err)
	}
	requireReferences(t, got, roots, "current", "old")
}

func TestDeriveVersionReferencesRefusesUncertainOrUnsafeShapes(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, roots UserRoots) *State
	}{
		{
			name: "recorded link target outside versions",
			setup: func(t *testing.T, roots UserRoots) *State {
				_ = referenceAsset(t, roots, "current", "skills/current")
				return referenceState("current", symlinkRecord(filepath.Join(roots.Home, "target"), filepath.Join(roots.Home, "outside"), ""))
			},
		},
		{
			name: "immediate version root is a symlink",
			setup: func(t *testing.T, roots UserRoots) *State {
				_ = referenceAsset(t, roots, "current", "skills/current")
				external := filepath.Join(roots.Home, "external")
				writeFileOrDie(t, filepath.Join(external, "assets", "skill"), "asset\n")
				linkedRoot := filepath.Join(roots.VersionsDir(), "linked")
				if err := os.MkdirAll(filepath.Dir(linkedRoot), 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.Symlink(external, linkedRoot); err != nil {
					t.Fatalf("Symlink: %v", err)
				}
				return referenceState("current", symlinkRecord(filepath.Join(roots.Home, "target"), filepath.Join(linkedRoot, "assets", "skill"), ""))
			},
		},
		{
			name: "live link loop",
			setup: func(t *testing.T, roots UserRoots) *State {
				_ = referenceAsset(t, roots, "current", "skills/current")
				target := filepath.Join(roots.Home, "target")
				symlinkOrDie(t, target, target)
				return referenceState("current", symlinkRecord(target, referenceAsset(t, roots, "recorded", "skills/recorded"), ""))
			},
		},
		{
			name: "structurally invalid state",
			setup: func(t *testing.T, roots UserRoots) *State {
				_ = referenceAsset(t, roots, "current", "skills/current")
				state := referenceState("current")
				state.Targets = []TargetRecord{{Path: "relative", Kind: KindFile, SHA256: "digest", Role: "agent"}}
				return state
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roots := referenceRoots(t)
			_, err := DeriveVersionReferences(roots, tc.setup(t, roots))
			if err == nil {
				t.Fatal("DeriveVersionReferences succeeded, want refusal")
			}
		})
	}
}

func TestDeriveVersionReferencesRefusesUnreadableAncestor(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can traverse a chmod(000) directory")
	}
	roots := referenceRoots(t)
	_ = referenceAsset(t, roots, "current", "skills/current")
	blocked := filepath.Join(roots.Home, "blocked")
	if err := os.MkdirAll(blocked, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })
	state := referenceState("current", symlinkRecord(filepath.Join(roots.Home, "target"), filepath.Join(blocked, "asset"), ""))
	_, err := DeriveVersionReferences(roots, state)
	if err == nil || (!errors.Is(err, fs.ErrPermission) && !strings.Contains(err.Error(), "permission")) {
		t.Fatalf("DeriveVersionReferences error = %v, want unreadable-ancestor refusal", err)
	}
}

func TestDeriveVersionReferencesIgnoresDevelopmentSourceOutsideVersions(t *testing.T) {
	roots := referenceRoots(t)
	source := filepath.Join(roots.Home, "checkout")
	writeFileOrDie(t, filepath.Join(source, "README.md"), "source\n")
	state := referenceState("source-digest")
	state.Mode = ModeDevelopment
	state.SourceRoot = source
	state.SourceDigest = "source-digest"

	got, err := DeriveVersionReferences(roots, state)
	if err != nil {
		t.Fatalf("DeriveVersionReferences: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("references = %#v, want no release version roots", got)
	}
}
