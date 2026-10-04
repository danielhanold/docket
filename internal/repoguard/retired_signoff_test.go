package repoguard

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// retiredSignoffToken is the finalize repair sign-off block reason that change
// 0515 retired: a green repair merges, so no maintained skill or agent may
// name the reason.
const retiredSignoffToken = "repair-needs-signoff"

// retiredSignoffRoots are walked whole, so a new skill or agent file is in
// scope the moment it exists — no hand-listed file population.
var retiredSignoffRoots = []string{"skills", "agents"}

// signoffTokenHits walks each root under repo and returns every file (repo
// relative, slash form) that contains the retired token.
func signoffTokenHits(repo string, roots []string) ([]string, int, error) {
	var hits []string
	scanned := 0
	for _, r := range roots {
		err := filepath.WalkDir(filepath.Join(repo, r), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			scanned++
			if strings.Contains(string(b), retiredSignoffToken) {
				rel, _ := filepath.Rel(repo, p)
				hits = append(hits, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return nil, scanned, err
		}
	}
	return hits, scanned, nil
}

func TestRetiredRepairSignoffAbsentFromSkillsAndAgents(t *testing.T) {
	root := guardRoot(t)
	hits, scanned, err := signoffTokenHits(root, retiredSignoffRoots)
	if err != nil {
		t.Fatalf("walk skills/ and agents/: %v (fail closed)", err)
	}
	if scanned == 0 {
		t.Fatalf("population floor: no files under %v", retiredSignoffRoots)
	}
	for _, h := range hits {
		t.Errorf("%s names the retired %q block reason; a green finalize repair merges", h, retiredSignoffToken)
	}

	t.Run("non_vacuity", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		if err := os.MkdirAll(filepath.Join(dir, "skills", "x", "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		planted := filepath.Join(dir, "skills", "x", "references", "deep.md")
		if err := os.WriteFile(planted, []byte("reason `"+retiredSignoffToken+"`\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, _, err := signoffTokenHits(dir, []string{"skills"})
		if err != nil || len(got) != 1 || got[0] != "skills/x/references/deep.md" {
			t.Errorf("a planted nested occurrence was not caught: %v %v", got, err)
		}
	})
}
