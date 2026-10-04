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

// jsonDecodeHome maps each JSON decode primitive to the single function
// allowed to reference it. decodeRequest may be reached only through
// declareJSONFile, which records the decoded type on the command; the raw
// encoding/json primitives live only inside decodeRequest. An empty home means
// no function may reference the primitive: json.Unmarshal cannot refuse
// unknown fields, so a request read through it would bypass the strict decode.
// The json.* keys name the encoding/json function whatever the file's local
// import name.
var jsonDecodeHome = map[string]string{
	"decodeRequest":         "declareJSONFile",
	"json.NewDecoder":       "decodeRequest",
	"DisallowUnknownFields": "decodeRequest",
	"json.Unmarshal":        "",
}

// encodingJSONName returns the name file uses for encoding/json: "" when the
// file does not import it (or blank-imports it), "." for a dot import.
func encodingJSONName(file *ast.File) string {
	for _, imp := range file.Imports {
		if imp.Path.Value != `"encoding/json"` {
			continue
		}
		if imp.Name == nil {
			return "json"
		}
		if imp.Name.Name == "_" {
			return ""
		}
		return imp.Name.Name
	}
	return ""
}

// scanJSONDecodeSites parses every non-test .go file in dir and reports each
// reference to a decode primitive outside its home function. The scan keys on
// syntactic shape: any identifier decodeRequest (a call, or a value
// reference), any selector .DisallowUnknownFields, and NewDecoder or Unmarshal
// selected from the file's encoding/json import under whatever local name it
// carries. A dot import of encoding/json is refused outright, since it hides
// those selectors. It never consults a list of call sites. A reference at
// package level (outside any function) is always a violation. homed counts the references found in
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
		jsonName := encodingJSONName(file)
		if jsonName == "." {
			violations = append(violations, fmt.Sprintf("%s: dot-imports encoding/json, which hides its decode primitives from this guard; import it by name",
				fset.Position(file.Package)))
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
					} else if pkg, ok := x.X.(*ast.Ident); ok && jsonName != "" && pkg.Name == jsonName {
						switch x.Sel.Name {
						case "NewDecoder", "Unmarshal":
							key = "json." + x.Sel.Name
						}
					}
				case *ast.Ident:
					if x.Name == "decodeRequest" {
						key = "decodeRequest"
					}
				}
				if key == "" {
					return true
				}
				home := jsonDecodeHome[key]
				if owner != "" && owner == home {
					homed[key]++
					return true
				}
				where := owner
				if where == "" {
					where = "package level"
				}
				allowed := "only " + home + " may"
				if home == "" {
					allowed = "nothing may"
				}
				violations = append(violations, fmt.Sprintf("%s: %s references %s; %s (route the decode through declareJSONFile)",
					fset.Position(n.Pos()), where, key, allowed))
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
		if home != "" && homed[key] == 0 {
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
	for key, home := range jsonDecodeHome {
		if home == "" {
			continue
		}
		if homed[key] != 1 {
			t.Errorf("homed[%s] = %d, want 1", key, homed[key])
		}
	}
}

// TestJSONDecodeScanSeesThroughImportSpelling proves the scan keys on the
// encoding/json import, not on the spelling "json": an aliased import's
// NewDecoder and an Unmarshal anywhere are both reported, a dot import is
// refused outright, and a non-encoding/json package spelled json is ignored.
func TestJSONDecodeScanSeesThroughImportSpelling(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"aliased NewDecoder", `package cli

import stdjson "encoding/json"

func stray() { _ = stdjson.NewDecoder(nil) }
`, "stray references json.NewDecoder"},
		{"Unmarshal", `package cli

import "encoding/json"

func stray() { _ = json.Unmarshal(nil, nil) }
`, "stray references json.Unmarshal"},
		{"aliased Unmarshal", `package cli

import j "encoding/json"

func stray() { _ = j.Unmarshal(nil, nil) }
`, "stray references json.Unmarshal"},
		{"dot import", `package cli

import . "encoding/json"

func stray() { _ = Unmarshal(nil, nil) }
`, "dot-imports encoding/json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := testsupport.TempDir(t)
			if err := os.WriteFile(filepath.Join(dir, "stray.go"), []byte(tc.src), 0o600); err != nil {
				t.Fatal(err)
			}
			violations, _, err := scanJSONDecodeSites(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(violations) != 1 || !strings.Contains(violations[0], tc.want) {
				t.Fatalf("violations = %q, want exactly one containing %q", violations, tc.want)
			}
		})
	}

	// A package that is not encoding/json, even one named json, is not a decode primitive.
	dir := testsupport.TempDir(t)
	src := `package cli

import json "example.com/notjson"

func fine() { _ = json.NewDecoder(nil); _ = json.Unmarshal(nil, nil) }
`
	if err := os.WriteFile(filepath.Join(dir, "fine.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, _, err := scanJSONDecodeSites(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %q, want none for a non-encoding/json package", violations)
	}
}
