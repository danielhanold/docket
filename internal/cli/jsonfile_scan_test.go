package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// jsonDecodeHome maps each strict-JSON decode primitive to the single function
// allowed to reference it. decodeRequest may be reached only through
// declareJSONFile, which records the decoded type on the command; the raw
// encoding/json primitives live only inside decodeRequest.
var jsonDecodeHome = map[string]string{
	"decodeRequest":         "declareJSONFile",
	"json.NewDecoder":       "decodeRequest",
	"DisallowUnknownFields": "decodeRequest",
}

// scanJSONDecodeSites parses every non-test .go file in dir and reports each
// reference to a decode primitive outside its home function. The scan keys on
// syntactic shape: any identifier decodeRequest (a call, or a value
// reference), any selector .DisallowUnknownFields, and json.NewDecoder. It
// never consults a list of call sites. A reference at package level (outside
// any function) is always a violation. homed counts the references found in
// their home, so a caller can prove the scan still sees the real decode path.
func scanJSONDecodeSites(dir string) (violations []string, homed map[string]int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	homed = map[string]int{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, nil, err
		}
		for _, decl := range file.Decls {
			owner := "" // package level
			var root ast.Node = decl
			if fd, ok := decl.(*ast.FuncDecl); ok {
				if fd.Body == nil {
					continue
				}
				owner, root = fd.Name.Name, fd.Body
			}
			ast.Inspect(root, func(n ast.Node) bool {
				key := ""
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if x.Sel.Name == "DisallowUnknownFields" {
						key = "DisallowUnknownFields"
					} else if pkg, ok := x.X.(*ast.Ident); ok && pkg.Name == "json" && x.Sel.Name == "NewDecoder" {
						key = "json.NewDecoder"
					}
				case *ast.Ident:
					if x.Name == "decodeRequest" {
						key = "decodeRequest"
					}
				}
				if key == "" {
					return true
				}
				if owner != "" && owner == jsonDecodeHome[key] {
					homed[key]++
					return true
				}
				where := owner
				if where == "" {
					where = "package level"
				}
				violations = append(violations, fmt.Sprintf("%s: %s references %s; only %s may (route the decode through declareJSONFile)",
					fset.Position(n.Pos()), where, key, jsonDecodeHome[key]))
				return true
			})
		}
	}
	return violations, homed, nil
}

// TestJSONFileDecodesGoThroughTheRegisteringHelper is the bypass guard: no
// production file in this package may strictly decode JSON except through
// declareJSONFile, so every JSON request file an operation reads is declared
// on its command (where TestPublishedRequestIsTheDecodedJSONFile compares it
// with the published schema). The homed floor proves the scan still reaches
// the real decode path; a rename that blinds it reddens here.
func TestJSONFileDecodesGoThroughTheRegisteringHelper(t *testing.T) {
	violations, homed, err := scanJSONDecodeSites(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Error(v)
	}
	for key, home := range jsonDecodeHome {
		if homed[key] == 0 {
			t.Errorf("no reference to %s found inside %s; the scan no longer sees the decode path, so it guards nothing", key, home)
		}
	}
}

// TestJSONDecodeScanReportsAStrayDecode is the scan's positive control: a
// scratch package with one stray decodeRequest call and one package-level
// value reference reports exactly those two, and nothing for the homed uses.
func TestJSONDecodeScanReportsAStrayDecode(t *testing.T) {
	dir := testsupport.TempDir(t)
	src := `package cli

import "encoding/json"

func declareJSONFile() { _ = decodeRequest(nil, "", "", nil) }

func decodeRequest(a, b, c, d any) error {
	dec := json.NewDecoder(nil)
	dec.DisallowUnknownFields()
	return nil
}

func stray() { _ = decodeRequest(nil, "--x", "-", nil) }

var leaked = decodeRequest
`
	if err := os.WriteFile(filepath.Join(dir, "stray.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, homed, err := scanJSONDecodeSites(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 || !strings.Contains(violations[0], "stray references decodeRequest") || !strings.Contains(violations[1], "package level references decodeRequest") {
		t.Fatalf("violations = %q, want exactly the stray call and the package-level reference", violations)
	}
	for key := range jsonDecodeHome {
		if homed[key] != 1 {
			t.Errorf("homed[%s] = %d, want 1", key, homed[key])
		}
	}
}
