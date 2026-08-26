package reposeed

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/install"
)

// sampleTargets renders one target of each kind under root, mirroring the shapes
// Plan emits: a co-owned AGENTS.md block, a Claude symlink to it, and a Cursor
// rule file. It returns them out of Path order so a test can prove DesiredRecord
// sorts.
func sampleTargets(root string) ([]install.Target, map[string][]string) {
	agents := filepath.Join(root, "AGENTS.md")
	claude := filepath.Join(root, "CLAUDE.md")
	cursor := filepath.Join(root, ".cursor", "rules", "docket-dispatch.mdc")
	targets := []install.Target{
		{
			Path:       claude,
			Kind:       install.KindSymlink,
			LinkTarget: agents,
			Role:       roleDispatch,
		},
		{
			Path:       agents,
			Kind:       install.KindManagedBlock,
			Content:    []byte("interior body\n"),
			BlockName:  dispatchBlockName,
			Annotation: dispatchAnnotation,
			Role:       roleDispatch,
		},
		{
			Path:    cursor,
			Kind:    install.KindFile,
			Content: []byte("cursor rule bytes\n"),
			Role:    roleDispatch,
		},
	}
	owners := map[string][]string{
		filepath.Clean(agents): {harnessOpencode, harnessCodex}, // unsorted on purpose
		filepath.Clean(claude): {harnessClaude},
		filepath.Clean(cursor): {harnessCursor},
	}
	return targets, owners
}

func TestDesiredRecordSortsSurfacesAndOwners(t *testing.T) {
	root := "/repo"
	targets, owners := sampleTargets(root)
	rec, err := DesiredRecord(targets, owners, root)
	if err != nil {
		t.Fatalf("DesiredRecord: %v", err)
	}
	if rec.FormatVersion != RecordFormatVersion {
		t.Errorf("FormatVersion = %d, want %d", rec.FormatVersion, RecordFormatVersion)
	}

	// Surfaces sorted by their worktree-relative, slash-separated Path.
	wantPaths := []string{".cursor/rules/docket-dispatch.mdc", "AGENTS.md", "CLAUDE.md"}
	var gotPaths []string
	for _, s := range rec.Surfaces {
		gotPaths = append(gotPaths, s.Path)
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Errorf("surface paths = %v, want %v", gotPaths, wantPaths)
	}

	// AGENTS.md owners sorted despite unsorted input.
	for _, s := range rec.Surfaces {
		if s.Path == "AGENTS.md" {
			want := []string{harnessCodex, harnessOpencode}
			if !reflect.DeepEqual(s.Harnesses, want) {
				t.Errorf("AGENTS.md harnesses = %v, want %v", s.Harnesses, want)
			}
			if s.Kind != install.KindManagedBlock || s.BlockName != dispatchBlockName {
				t.Errorf("AGENTS.md kind/block = %s/%s", s.Kind, s.BlockName)
			}
			if s.SHA256 == "" {
				t.Error("AGENTS.md surface missing block SHA256")
			}
		}
		if s.Path == "CLAUDE.md" {
			if s.Kind != install.KindSymlink {
				t.Errorf("CLAUDE.md kind = %s, want symlink", s.Kind)
			}
			if s.LinkTarget != "AGENTS.md" {
				t.Errorf("CLAUDE.md link target = %q, want relative AGENTS.md", s.LinkTarget)
			}
		}
	}
}

func TestDesiredRecordRefusesEscapingPath(t *testing.T) {
	root := "/repo"
	// An absolute target path outside the root reduces to a "../x" relative
	// spelling. DesiredRecord must refuse it rather than store an escape.
	targets := []install.Target{
		{Path: "/x", Kind: install.KindFile, Content: []byte("body\n"), Role: roleDispatch},
	}
	owners := map[string][]string{filepath.Clean("/x"): {harnessClaude}}
	if _, err := DesiredRecord(targets, owners, root); err == nil {
		t.Fatal("DesiredRecord accepted a path escaping the worktree root; want error")
	}
}

func TestDesiredRecordAcceptsContainedPath(t *testing.T) {
	// The mutation-test partner of the escape guard: a legitimately contained
	// path must be accepted, so the guard is proven to reject only escapes.
	root := "/repo"
	targets := []install.Target{
		{Path: filepath.Join(root, "sub", "AGENTS.md"), Kind: install.KindFile, Content: []byte("body\n"), Role: roleDispatch},
	}
	owners := map[string][]string{filepath.Clean(filepath.Join(root, "sub", "AGENTS.md")): {harnessCodex}}
	rec, err := DesiredRecord(targets, owners, root)
	if err != nil {
		t.Fatalf("DesiredRecord refused a contained path: %v", err)
	}
	if rec.Surfaces[0].Path != "sub/AGENTS.md" {
		t.Errorf("contained path stored as %q, want sub/AGENTS.md", rec.Surfaces[0].Path)
	}
}

func TestEncodeLoadRoundTrip(t *testing.T) {
	root := "/repo"
	targets, owners := sampleTargets(root)
	rec, err := DesiredRecord(targets, owners, root)
	if err != nil {
		t.Fatalf("DesiredRecord: %v", err)
	}

	data, err := EncodeRecord(rec)
	if err != nil {
		t.Fatalf("EncodeRecord: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("EncodeRecord output must end in a trailing newline")
	}

	path := filepath.Join(t.TempDir(), "docket", "install.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadRecord(path)
	if err != nil {
		t.Fatalf("LoadRecord: %v", err)
	}
	if !reflect.DeepEqual(got, rec) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, rec)
	}
}

func TestLoadRecordAbsentIsNotInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docket", "install.json")
	got, err := LoadRecord(path)
	if err != nil {
		t.Fatalf("LoadRecord on absent file: %v", err)
	}
	if got != nil {
		t.Errorf("absent record loaded as %+v, want nil", got)
	}
}

func TestLoadRecordCorruptIsError(t *testing.T) {
	// A present-but-unreadable record is always an error — never silently
	// adopted as "not installed", mirroring install.LoadState. Adopting
	// corruption would let a later publish overwrite surfaces it cannot prove
	// it owns.
	path := filepath.Join(t.TempDir(), "install.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRecord(path)
	if err == nil {
		t.Fatal("LoadRecord on corrupt JSON returned no error")
	}
	if got != nil {
		t.Errorf("corrupt record returned %+v, want nil record with the error", got)
	}
}

func TestToStateProducesAbsoluteCleanedPaths(t *testing.T) {
	root := "/repo"
	targets, owners := sampleTargets(root)
	rec, err := DesiredRecord(targets, owners, root)
	if err != nil {
		t.Fatalf("DesiredRecord: %v", err)
	}

	// A relocated worktree: ToState joins the stored relative paths under the
	// CURRENT root, so history survives a move.
	newRoot := "/moved/repo"
	state := rec.ToState(newRoot)
	if state.FormatVersion != install.StateFormatVersion {
		t.Errorf("state FormatVersion = %d, want %d", state.FormatVersion, install.StateFormatVersion)
	}
	if len(state.Targets) != len(rec.Surfaces) {
		t.Fatalf("state has %d targets, want %d", len(state.Targets), len(rec.Surfaces))
	}
	for _, tr := range state.Targets {
		if !filepath.IsAbs(tr.Path) {
			t.Errorf("target path %q is not absolute", tr.Path)
		}
		if tr.Path != filepath.Clean(tr.Path) {
			t.Errorf("target path %q is not cleaned", tr.Path)
		}
		if !strings.HasPrefix(tr.Path, newRoot+string(filepath.Separator)) {
			t.Errorf("target path %q not rooted under %q", tr.Path, newRoot)
		}
		if tr.Kind == install.KindSymlink {
			if !filepath.IsAbs(tr.LinkTarget) {
				t.Errorf("symlink target %q is not absolute", tr.LinkTarget)
			}
			if tr.LinkTarget != filepath.Join(newRoot, "AGENTS.md") {
				t.Errorf("symlink target = %q, want %q", tr.LinkTarget, filepath.Join(newRoot, "AGENTS.md"))
			}
		}
	}
}

func TestRecordPath(t *testing.T) {
	if got, want := RecordPath("/some/.git"), filepath.Join("/some/.git", "docket", "install.json"); got != want {
		t.Errorf("RecordPath = %q, want %q", got, want)
	}
}
