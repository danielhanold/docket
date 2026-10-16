package repoguard

// Guard: every folder directly under testdata/repositories/ is named for what
// it holds (configs, documents, status-corpus), never for a release or a date.
// Where a sample came from belongs in its folder's PROVENANCE.md, and tagging a
// release creates nothing here (testdata/README.md).
//
// Shape: a name that starts with a digit, optionally after a "v", looks like a
// version or a date — `^v?[0-9]`.
//
// Mutation-tested: an empty folder named v0.9.14 directly under
// testdata/repositories/ reddens TestSampleFoldersNamedForWhatTheyHold.

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSampleFolderNameAllowed(t *testing.T) {
	for _, name := range []string{"configs", "documents", "status-corpus", "visibility-modes"} {
		if !sampleFolderNameAllowed(name) {
			t.Errorf("sampleFolderNameAllowed(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"v0.9.14", "v1.0.0-rc.1", "1.0.0", "2026-10-11"} {
		if sampleFolderNameAllowed(name) {
			t.Errorf("sampleFolderNameAllowed(%q) = true, want false", name)
		}
	}
}

// versionOrDateName matches a folder name shaped like a version or a date.
var versionOrDateName = regexp.MustCompile(`^v?[0-9]`)

// sampleFolderNameAllowed reports whether name is acceptable for a folder
// directly under testdata/repositories/.
func sampleFolderNameAllowed(name string) bool {
	return !versionOrDateName.MatchString(name)
}

func TestSampleFoldersNamedForWhatTheyHold(t *testing.T) {
	dir := filepath.Join(guardRoot(t), "testdata", "repositories")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	folders := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folders++
		if !sampleFolderNameAllowed(e.Name()) {
			t.Errorf("testdata/repositories/%s/ is named like a version or a date. "+
				"Name sample folders for what they hold, and record the release or date in the folder's PROVENANCE.md. "+
				"See testdata/README.md.", e.Name())
		}
	}
	if folders == 0 {
		t.Fatalf("no folders under %s: the guard has nothing to check", dir)
	}
}
