package repoguard

import (
	"errors"
	"io/fs"
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

// TestLicenseFiles guards the license artifacts of change 0404: LICENSE is the
// verbatim Apache License 2.0 (identifier, date line, and the appendix's
// distinctive boilerplate clause, which pins the complete text) and carries no
// trace of the retired PolyForm license; NOTICE carries the copyright line
// Apache section 4(d) obliges redistributors to preserve; the retired
// additional-permissions file is absent; CONTRIBUTING.md adopts the DCO
// trailer. Fixed-string containment only — no regexp, no network.
func TestLicenseFiles(t *testing.T) {
	license := readRepoFile(t, "LICENSE")
	for _, want := range []string{
		"Apache License",
		"Version 2.0, January 2004",
		`Licensed under the Apache License, Version 2.0 (the "License");`,
	} {
		if !strings.Contains(license, want) {
			t.Errorf("LICENSE does not contain %q", want)
		}
	}
	for _, banned := range []string{
		"PolyForm",
		"LICENSE-ADDITIONAL-PERMISSIONS.md",
	} {
		if strings.Contains(license, banned) {
			t.Errorf("LICENSE still contains %q — the PolyForm-era content must be gone", banned)
		}
	}

	notice := readRepoFile(t, "NOTICE")
	if want := "Copyright 2026 Daniel Hanold"; !strings.Contains(notice, want) {
		t.Errorf("NOTICE does not contain %q", want)
	}

	root, err := Root()
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	_, err = os.Stat(filepath.Join(root, "LICENSE-ADDITIONAL-PERMISSIONS.md"))
	switch {
	case err == nil:
		t.Errorf("LICENSE-ADDITIONAL-PERMISSIONS.md still exists at the repo root; change 0404 deletes it")
	case !errors.Is(err, fs.ErrNotExist):
		t.Fatalf("stat LICENSE-ADDITIONAL-PERMISSIONS.md: %v", err)
	}

	contributing := readRepoFile(t, "CONTRIBUTING.md")
	if want := "Signed-off-by"; !strings.Contains(contributing, want) {
		t.Errorf("CONTRIBUTING.md does not contain %q", want)
	}
}

// TestLicenseReadmeSection guards the README's License section: the heading is
// present and the section links to LICENSE, NOTICE, and CONTRIBUTING.md
// (change 0404). The link targets are matched as markdown link destinations,
// so a rewrite that keeps the words but drops the links reddens.
func TestLicenseReadmeSection(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for _, want := range []string{
		"## License",
		"](LICENSE)",
		"](NOTICE)",
		"](CONTRIBUTING.md)",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not contain %q", want)
		}
	}
}
