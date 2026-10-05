package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestStillRegisteredEachCheck pins each of stillRegistered's three checks on
// its own: every true row matches exactly one check, so removing any single
// check reddens its row.
func TestStillRegisteredEachCheck(t *testing.T) {
	const featureRef = gitcli.RefName("refs/heads/feat/x")
	root := testsupport.TempDir(t)

	// A recorded path that does not exist: only the lexical check can match it,
	// because canonicalization fails on a missing path.
	missing := filepath.Join(root, "missing-workspace")

	// A symlinked registration: the lexical path is the link, the recorded path
	// is the canonical target, so only the canonical check can match it.
	realDir := filepath.Join(root, "real-workspace")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link-workspace")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	canonicalReal, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatal(err)
	}
	if abs, _ := filepath.Abs(link); filepath.Clean(abs) == canonicalReal {
		t.Fatalf("fixture invalid: link %q lexically equals canonical %q", link, canonicalReal)
	}

	unrelated := filepath.Join(root, "gone-elsewhere")

	cases := []struct {
		name  string
		infos []gitcli.WorktreeInfo
		path  string
		want  bool
	}{
		{
			name:  "attached on the feature ref at an unrelated path",
			infos: []gitcli.WorktreeInfo{{Path: unrelated, Branch: featureRef}},
			path:  missing,
			want:  true,
		},
		{
			name:  "detached at the recorded path lexically only",
			infos: []gitcli.WorktreeInfo{{Path: missing, Detached: true}},
			path:  missing,
			want:  true,
		},
		{
			name:  "detached at a symlink to the recorded path (canonical only)",
			infos: []gitcli.WorktreeInfo{{Path: link, Detached: true}},
			path:  canonicalReal,
			want:  true,
		},
		{
			name: "unrelated unresolvable registration on another branch",
			infos: []gitcli.WorktreeInfo{
				{Path: unrelated, Branch: gitcli.RefName("refs/heads/other")},
				{Path: filepath.Join(root, "also-gone"), Detached: true},
			},
			path: missing,
			want: false,
		},
		{
			name:  "detached registration does not match on its branch field",
			infos: []gitcli.WorktreeInfo{{Path: unrelated, Detached: true, Branch: featureRef}},
			path:  missing,
			want:  false,
		},
		{
			name:  "no registrations",
			infos: nil,
			path:  missing,
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stillRegistered(tc.infos, tc.path, featureRef); got != tc.want {
				t.Errorf("stillRegistered = %v; want %v", got, tc.want)
			}
		})
	}
}
