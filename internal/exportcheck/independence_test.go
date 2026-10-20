package exportcheck

import (
	"os/exec"
	"strings"
	"testing"
)

const (
	exportPkg      = "github.com/danielhanold/docket/internal/export"
	exportcheckPkg = "github.com/danielhanold/docket/internal/exportcheck"
	documentPkg    = "github.com/danielhanold/docket/internal/document"
)

// deps lists the import paths pkg depends on, itself excluded.
func deps(t *testing.T, pkg string) map[string]bool {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
	}
	set := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" && line != pkg {
			set[line] = true
		}
	}
	return set
}

// TestDetectorSharesNothingWithTransform keeps the self-check independent of the
// transform it checks: sharing code or patterns would make the two agree by
// construction, so the detector could never find anything.
func TestDetectorSharesNothingWithTransform(t *testing.T) {
	checkDeps := deps(t, exportcheckPkg)
	exportDeps := deps(t, exportPkg)
	if checkDeps[exportPkg] {
		t.Errorf("%s depends on %s", exportcheckPkg, exportPkg)
	}
	if exportDeps[exportcheckPkg] {
		t.Errorf("%s depends on %s", exportPkg, exportcheckPkg)
	}
	if checkDeps[documentPkg] {
		t.Errorf("%s depends on %s, whose fence and marker scanning the transform uses", exportcheckPkg, documentPkg)
	}
}
