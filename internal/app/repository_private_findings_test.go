package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/testsupport"
)

// visibilityEff builds an Effective whose visibility value came from layer.
// explicit=false models the built-in default.
func visibilityEff(value string, layer config.LayerKind, source string, explicit bool) config.Effective {
	var eff config.Effective
	eff.Visibility = config.Value[string]{
		Value:      value,
		Explicit:   explicit,
		Provenance: config.Provenance{Layer: layer, Source: source},
	}
	return eff
}

func TestVisibilityMismatchOnlyFromRepositoryLevelFiles(t *testing.T) {
	shared := layout.SharedLayout("/r/.git", "/r")
	private := layout.PrivateLayout("/r/.git", "/r", "/data", "o-r")
	cases := []struct {
		name   string
		lay    layout.Layout
		eff    config.Effective
		want   bool
		source string
	}{
		{"shared repo, committed file says private", shared, visibilityEff("private", config.LayerRepository, ".docket.yml", true), true, ".docket.yml"},
		{"private repo, private config says shared", private, visibilityEff("shared", config.LayerRepository, layout.PrivateConfigDisplay, true), true, layout.PrivateConfigDisplay},
		{"shared repo, local file says private", shared, visibilityEff("private", config.LayerRepositoryLocal, ".docket.local.yml", true), true, ".docket.local.yml"},
		{"shared repo, global says private", shared, visibilityEff("private", config.LayerGlobal, "/home/u/.config/docket/config.yml", true), false, ""},
		{"private repo, global says shared", private, visibilityEff("shared", config.LayerGlobal, "/home/u/.config/docket/config.yml", true), false, ""},
		{"shared repo, committed file agrees", shared, visibilityEff("shared", config.LayerRepository, ".docket.yml", true), false, ""},
		{"private repo, private config agrees", private, visibilityEff("private", config.LayerRepository, layout.PrivateConfigDisplay, true), false, ""},
		{"private repo, built-in default", private, visibilityEff("shared", config.LayerBuiltIn, "built-in", false), false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := visibilityMismatchFinding(tc.lay, tc.eff)
			if (got != nil) != tc.want {
				t.Fatalf("visibilityMismatchFinding = %+v, want finding=%v", got, tc.want)
			}
			if got == nil {
				return
			}
			if got.Code != FindingVisibilityMismatch || got.Ref != tc.source {
				t.Errorf("finding = %+v, want code %q ref %q", got, FindingVisibilityMismatch, tc.source)
			}
			if !strings.Contains(got.Message, tc.source) || !strings.Contains(got.Message, "set up "+string(tc.lay.Mode)) {
				t.Errorf("message %q must name the source and the repository's mode", got.Message)
			}
			if !strings.HasPrefix(got.Remedy, "Change visibility in "+tc.source+" to "+string(tc.lay.Mode)) {
				t.Errorf("remedy %q must lead with the edit-the-file option", got.Remedy)
			}
		})
	}
}

// writeCheckout makes a checkout dir under the layout's checkouts folder whose
// .git file names gitdir.
func writeCheckout(t *testing.T, dir, gitdir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanOrphanedCheckouts(t *testing.T) {
	data := testsupport.TempDir(t)
	common := testsupport.TempDir(t)
	lay := layout.PrivateLayout(common, "/clones/mine", data, "o-r")

	liveGitdir := filepath.Join(common, "worktrees", "other")
	if err := os.MkdirAll(liveGitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(testsupport.TempDir(t), "gone", ".git", "worktrees", "x")

	// The own checkout with a missing gitdir is never reported.
	writeCheckout(t, lay.MetadataWorktree, missing)
	other := filepath.Join(lay.CheckoutsDir, "other-11111111")
	writeCheckout(t, other, liveGitdir)
	moved := filepath.Join(lay.CheckoutsDir, "moved-22222222")
	writeCheckout(t, moved, missing)
	// No .git file, and an unparsable one: not proven ours, skipped.
	if err := os.MkdirAll(filepath.Join(lay.CheckoutsDir, "bare-33333333"), 0o755); err != nil {
		t.Fatal(err)
	}
	garbled := filepath.Join(lay.CheckoutsDir, "garbled-44444444")
	if err := os.MkdirAll(garbled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(garbled, ".git"), []byte("not a gitdir line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A stray file beside the checkouts is not a checkout.
	if err := os.WriteFile(filepath.Join(lay.CheckoutsDir, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := scanOrphanedCheckouts(lay)
	if err != nil {
		t.Fatalf("scanOrphanedCheckouts: %v", err)
	}
	if want := []string{moved}; !reflect.DeepEqual(got, want) {
		t.Errorf("orphans = %v, want %v", got, want)
	}

	t.Run("relative gitdir resolves against the checkout", func(t *testing.T) {
		rel := filepath.Join(lay.CheckoutsDir, "rel-55555555")
		writeCheckout(t, rel, "../other-11111111")
		got, err := scanOrphanedCheckouts(lay)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{moved}; !reflect.DeepEqual(got, want) {
			t.Errorf("orphans = %v, want %v (a relative gitdir that exists is kept)", got, want)
		}
		if err := os.RemoveAll(rel); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("shared mode", func(t *testing.T) {
		got, err := scanOrphanedCheckouts(layout.SharedLayout(common, "/clones/mine"))
		if got != nil || err != nil {
			t.Errorf("shared scan = %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("missing checkouts dir", func(t *testing.T) {
		empty := layout.PrivateLayout(common, "/clones/mine", testsupport.TempDir(t), "o-r")
		got, err := scanOrphanedCheckouts(empty)
		if got != nil || err != nil {
			t.Errorf("scan of a missing checkouts dir = %v, %v; want nil, nil", got, err)
		}
	})
}
