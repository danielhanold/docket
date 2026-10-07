package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/reposeed"
	"github.com/danielhanold/docket/internal/testsupport"
)

const (
	testDispatchStart = "<!-- docket:dispatch:start -->"
	testDispatchEnd   = "<!-- docket:dispatch:end -->"
)

func TestStripDispatchBlock(t *testing.T) {
	block := testDispatchStart + "\nrun the dispatcher\n" + testDispatchEnd + "\n"
	cases := []struct {
		name    string
		src     string
		want    string
		remove  bool
		wantErr bool
	}{
		{name: "middle block", src: "# Rules\n\n" + block + "\nKeep this lesson.\n", want: "# Rules\n\n\nKeep this lesson.\n"},
		{name: "block only", src: block, want: "", remove: true},
		{name: "block with blank lines only", src: "\n\n" + block + "\n", want: "\n\n\n", remove: true},
		{name: "no block", src: "# Rules\n\nKeep this lesson.\n", want: "# Rules\n\nKeep this lesson.\n"},
		{name: "malformed markers", src: "# Rules\n" + testDispatchStart + "\nunterminated\n", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, remove, err := stripDispatchBlock([]byte(c.src))
			if c.wantErr {
				if err == nil {
					t.Fatalf("stripDispatchBlock = (%q, %v, nil), want an error", out, remove)
				}
				return
			}
			if err != nil {
				t.Fatalf("stripDispatchBlock: %v", err)
			}
			if string(out) != c.want || remove != c.remove {
				t.Errorf("stripDispatchBlock = (%q, %v), want (%q, %v)", out, remove, c.want, c.remove)
			}
		})
	}
}

func TestDisownWorkingTreeSurfaces(t *testing.T) {
	dir := testsupport.TempDir(t)

	t.Run("absent record", func(t *testing.T) {
		changed, err := disownWorkingTreeSurfaces(filepath.Join(dir, "absent", "install.json"), nil)
		if err != nil || changed {
			t.Fatalf("disownWorkingTreeSurfaces = (%v, %v), want (false, nil)", changed, err)
		}
	})

	t.Run("keeps the git-dir surface", func(t *testing.T) {
		path := filepath.Join(dir, "mixed", "install.json")
		writeTestRecord(t, path, "AGENTS.md", ".git/dckt/AGENTS.md")
		changed, err := disownWorkingTreeSurfaces(path, map[string]bool{"AGENTS.md": true})
		if err != nil || !changed {
			t.Fatalf("disownWorkingTreeSurfaces = (%v, %v), want (true, nil)", changed, err)
		}
		rec, err := reposeed.LoadRecord(path)
		if err != nil || rec == nil {
			t.Fatalf("LoadRecord = (%v, %v)", rec, err)
		}
		if len(rec.Surfaces) != 1 || rec.Surfaces[0].Path != ".git/dckt/AGENTS.md" {
			t.Errorf("surfaces = %+v, want only .git/dckt/AGENTS.md", rec.Surfaces)
		}
		if changed, err := disownWorkingTreeSurfaces(path, map[string]bool{"AGENTS.md": true}); err != nil || changed {
			t.Errorf("second call = (%v, %v), want (false, nil)", changed, err)
		}
	})

	t.Run("removes an emptied record", func(t *testing.T) {
		path := filepath.Join(dir, "shared", "install.json")
		writeTestRecord(t, path, "AGENTS.md", "CLAUDE.md")
		changed, err := disownWorkingTreeSurfaces(path, map[string]bool{"AGENTS.md": true, "CLAUDE.md": true})
		if err != nil || !changed {
			t.Fatalf("disownWorkingTreeSurfaces = (%v, %v), want (true, nil)", changed, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the emptied record is still present (stat err %v)", err)
		}
	})

	t.Run("keeps an untracked working-tree surface owned", func(t *testing.T) {
		path := filepath.Join(dir, "ignored", "install.json")
		writeTestRecord(t, path, "AGENTS.md", cursorRuleRel)
		changed, err := disownWorkingTreeSurfaces(path, map[string]bool{"AGENTS.md": true})
		if err != nil || !changed {
			t.Fatalf("disownWorkingTreeSurfaces = (%v, %v), want (true, nil)", changed, err)
		}
		rec, err := reposeed.LoadRecord(path)
		if err != nil || rec == nil {
			t.Fatalf("LoadRecord = (%v, %v)", rec, err)
		}
		if len(rec.Surfaces) != 1 || rec.Surfaces[0].Path != cursorRuleRel {
			t.Errorf("surfaces = %+v, want only %s, which no commit tracks", rec.Surfaces, cursorRuleRel)
		}
	})
}

// writeTestRecord writes an ownership record listing one managed block per path.
func writeTestRecord(t *testing.T, path string, paths ...string) {
	t.Helper()
	rec := &reposeed.Record{FormatVersion: reposeed.RecordFormatVersion}
	for _, p := range paths {
		rec.Surfaces = append(rec.Surfaces, reposeed.SurfaceRecord{
			Path: p, Kind: install.KindManagedBlock, BlockName: instructionsBlockName, SHA256: "abc", Harnesses: []string{"codex"},
		})
	}
	b, err := reposeed.EncodeRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
