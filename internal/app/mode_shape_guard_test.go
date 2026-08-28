package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Change 0363 Task 7: a structural guard against production code that
// reintroduces the removed metadata-mode topology. Go v1 supports one metadata
// topology (the fixed orphan `docket` branch), so mode-shaped production
// constructs must never come back. The guard keys on SYNTACTIC SHAPE — a struct
// field's json tag value and a declared identifier's name — never on a
// hand-listed spelling of one call site, and it excludes point-in-time records
// (it walks only maintained Go source). It is mutation-tested by TestModeShapeGuardIsFalsifiable.

// removedProtocolJSONKeys is the closed set of mode-shaped JSON keys change 0363
// removed from public protocol output. A production struct field must never
// carry one again. metadata_branch is intentionally NOT here: it survives as a
// legitimate render parameter (render.LinkContext.MetadataBranch) — this guard
// scopes to the internal/app protocol surface, where the mode keys lived.
var removedProtocolJSONKeys = map[string]bool{
	"metadata_mode": true,
	"repo_mode":     true,
}

// removedModeIdentifiers is the closed set of mode-selector identifiers change
// 0363 deleted. Their reappearance in production code re-establishes a
// mode-shaped selector.
var removedModeIdentifiers = map[string]bool{
	"metadataModeMain":   true,
	"metadataModeDocket": true,
}

// modeShapeScanRoots are the maintained-source roots the guard walks.
var modeShapeScanRoots = []string{"."}

// scanModeShapeViolations parses every non-test .go file under the given root and
// returns the violations found plus the count of files actually visited (the
// falsifiability floor: an empty walk is decoration).
func scanModeShapeViolations(t *testing.T, root string) (violations []string, visited int) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", root, err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			visited++
			base := filepath.Base(name)
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.Field:
					if node.Tag == nil {
						return true
					}
					tag := reflect.StructTag(strings.Trim(node.Tag.Value, "`"))
					jsonTag := tag.Get("json")
					if idx := strings.IndexByte(jsonTag, ','); idx >= 0 {
						jsonTag = jsonTag[:idx]
					}
					if removedProtocolJSONKeys[jsonTag] {
						violations = append(violations, base+": struct field carries removed mode-shaped json key "+jsonTag)
					}
				case *ast.Ident:
					if removedModeIdentifiers[node.Name] {
						violations = append(violations, base+": references removed mode identifier "+node.Name)
					}
				}
				return true
			})
		}
	}
	return violations, visited
}

func TestNoModeShapedProductionCode(t *testing.T) {
	total := 0
	for _, root := range modeShapeScanRoots {
		violations, visited := scanModeShapeViolations(t, root)
		total += visited
		for _, v := range violations {
			t.Errorf("mode-shaped production code reintroduced: %s", v)
		}
	}
	if total == 0 {
		t.Fatalf("mode-shape guard visited zero files — an unfalsifiable walker is decoration")
	}
}

// TestModeShapeGuardIsFalsifiable mutation-tests the guard by planting each
// violation shape into a temp source tree and confirming the scanner catches it,
// then confirming an empty tree trips the visited-count floor. A guard that
// cannot redden is decoration (AGENTS.md).
func TestModeShapeGuardIsFalsifiable(t *testing.T) {
	// Arm (a): a struct field carrying a removed mode-shaped json key.
	dirA := t.TempDir()
	writeGoFile(t, dirA, "a.go", "package p\n\ntype T struct {\n\tX string `json:\"metadata_mode\"`\n}\n")
	if v, _ := scanModeShapeViolations(t, dirA); len(v) == 0 {
		t.Errorf("guard did not detect a planted metadata_mode json field")
	}

	// Arm (b): a removed mode-selector identifier reintroduced.
	dirB := t.TempDir()
	writeGoFile(t, dirB, "b.go", "package p\n\nconst metadataModeMain = \"main\"\n")
	if v, _ := scanModeShapeViolations(t, dirB); len(v) == 0 {
		t.Errorf("guard did not detect a planted metadataModeMain identifier")
	}

	// Arm (c): a repo_mode json field.
	dirC := t.TempDir()
	writeGoFile(t, dirC, "c.go", "package p\n\ntype W struct {\n\tR string `json:\"repo_mode\"`\n}\n")
	if v, _ := scanModeShapeViolations(t, dirC); len(v) == 0 {
		t.Errorf("guard did not detect a planted repo_mode json field")
	}

	// Floor: an empty tree visits zero files — the run-level TestNoModeShapedProductionCode
	// fails on that, proving the walker is falsifiable.
	dirEmpty := t.TempDir()
	if _, visited := scanModeShapeViolations(t, dirEmpty); visited != 0 {
		t.Errorf("empty tree reported %d visited files, want 0", visited)
	}

	// Control: a clean file trips nothing.
	dirClean := t.TempDir()
	writeGoFile(t, dirClean, "ok.go", "package p\n\ntype Ok struct {\n\tMeta string `json:\"metadata_revision\"`\n}\n")
	if v, _ := scanModeShapeViolations(t, dirClean); len(v) != 0 {
		t.Errorf("guard flagged a clean file: %v", v)
	}
}

func writeGoFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
