package repoguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readRepoFile reads a repo-root-relative file, failing the test on any error:
// a missing or unreadable license artifact is a red result, never a skip.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	root, err := Root()
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// TestLicenseFiles guards the license artifacts of change 0401: LICENSE carries
// the PolyForm Noncommercial 1.0.0 identifier, the Required Notice, and the
// pointer to the additional-permissions file; that file carries its three
// clause headings. Fixed-string containment only — no regexp, no network.
func TestLicenseFiles(t *testing.T) {
	license := readRepoFile(t, "LICENSE")
	for _, want := range []string{
		"PolyForm Noncommercial License 1.0.0",
		"Required Notice: Copyright Daniel Hanold",
		"LICENSE-ADDITIONAL-PERMISSIONS.md",
	} {
		if !strings.Contains(license, want) {
			t.Errorf("LICENSE does not contain %q", want)
		}
	}

	perms := readRepoFile(t, "LICENSE-ADDITIONAL-PERMISSIONS.md")
	for _, want := range []string{
		"## 1. Individual commercial exemption",
		"## 2. Scope over the repository history",
		"## 3. Obtaining a commercial license",
	} {
		if !strings.Contains(perms, want) {
			t.Errorf("LICENSE-ADDITIONAL-PERMISSIONS.md does not contain heading %q", want)
		}
	}
}

// TestLicenseReadmeSection guards the README's License section: the heading is
// present and the section links to both license files (change 0401). The link
// targets are matched as markdown link destinations, so a rewrite that keeps
// the words but drops the links reddens.
func TestLicenseReadmeSection(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for _, want := range []string{
		"## License",
		"](LICENSE)",
		"](LICENSE-ADDITIONAL-PERMISSIONS.md)",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not contain %q", want)
		}
	}
}
