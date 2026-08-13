package config

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestParseLayer(t *testing.T) {
	src := func(data string) Source {
		return Source{Layer: LayerRepository, Name: ".docket.yml", Data: []byte(data)}
	}
	cases := []struct {
		name      string
		data      string
		wantNil   bool
		wantCodes []string // codes of returned diagnostics, in order
	}{
		{"empty file", "", true, nil},
		{"whitespace only", "\n\n   \n", true, nil},
		{"comments only", "# just a comment\n", true, nil},
		{"null document", "---\n", true, nil},
		{"explicit null root", "null\n", true, nil},
		{"simple mapping", "metadata_branch: docket\n", false, nil},
		{"flow mapping", "{metadata_branch: docket}\n", false, nil},
		{"empty flow mapping", "{}\n", false, nil},
		{"nested mapping", "finalize:\n  gate: local\n", false, nil},
		{"malformed", "a: [unclosed\n", true, []string{CodeInvalidYAML}},
		{"two documents", "a: 1\n---\nb: 2\n", true, []string{CodeInvalidYAML}},
		{"sequence root", "- a\n- b\n", true, []string{CodeInvalidType}},
		{"scalar root", "just-a-string\n", true, []string{CodeInvalidType}},
		{"alias", "a: &x 1\nb: *x\n", true, []string{CodeInvalidYAML}},
		{"merge key", "base: &b {x: 1}\nout:\n  <<: *b\n", true, []string{CodeInvalidYAML}},
		// An inline merge key carries no alias, so this case — and only this
		// case — exercises the merge-key guard on its own.
		{"merge key without alias", "out:\n  <<: {x: 1}\n", true, []string{CodeInvalidYAML}},
		{"duplicate top-level key", "a: 1\na: 2\n", true, []string{CodeDuplicateKey}},
		{"duplicate nested key", "m:\n  a: 1\n  a: 2\n", true, []string{CodeDuplicateKey}},
		{"duplicate key inside sequence element", "l:\n  - a: 1\n    a: 2\n", true, []string{CodeDuplicateKey}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, diags := parseLayer(src(tc.data))
			if tc.wantNil && node != nil {
				t.Fatalf("parseLayer(%q): want nil node, got kind %v", tc.data, node.Kind)
			}
			if !tc.wantNil && node == nil {
				t.Fatalf("parseLayer(%q): want a mapping node, got nil (diags %+v)", tc.data, diags)
			}
			if node != nil && node.Kind != yaml.MappingNode {
				t.Fatalf("parseLayer(%q): want a mapping root node, got kind %v", tc.data, node.Kind)
			}
			if len(diags) != len(tc.wantCodes) {
				t.Fatalf("parseLayer(%q): want %d diagnostics %v, got %d: %+v",
					tc.data, len(tc.wantCodes), tc.wantCodes, len(diags), diags)
			}
			for i, want := range tc.wantCodes {
				got := diags[i]
				if got.Code != want {
					t.Errorf("diag[%d].Code = %q, want %q", i, got.Code, want)
				}
				if got.Severity != SeverityError {
					t.Errorf("diag[%d].Severity = %q, want %q", i, got.Severity, SeverityError)
				}
				if got.Message == "" {
					t.Errorf("diag[%d].Message is empty", i)
				}
				if got.Provenance == nil {
					t.Fatalf("diag[%d].Provenance is nil", i)
				}
				if got.Provenance.Source != ".docket.yml" {
					t.Errorf("diag[%d].Provenance.Source = %q, want %q", i, got.Provenance.Source, ".docket.yml")
				}
				if got.Provenance.Layer != LayerRepository {
					t.Errorf("diag[%d].Provenance.Layer = %q, want %q", i, got.Provenance.Layer, LayerRepository)
				}
				if got.Provenance.Line < 1 {
					t.Errorf("diag[%d].Provenance.Line = %d, want >= 1", i, got.Provenance.Line)
				}
			}
		})
	}
}

func TestParseLayerProvenanceLine(t *testing.T) {
	src := Source{Layer: LayerGlobal, Name: "/cfg/config.yml", Data: []byte("a: 1\nb: 2\na: 3\n")}
	node, diags := parseLayer(src)
	if node != nil {
		t.Errorf("want nil node on duplicate key, got kind %v", node.Kind)
	}
	if len(diags) != 1 || diags[0].Code != CodeDuplicateKey {
		t.Fatalf("want exactly one duplicate-key diagnostic, got %+v", diags)
	}
	prov := diags[0].Provenance
	if prov == nil {
		t.Fatal("duplicate-key diagnostic has nil provenance")
	}
	if prov.Line != 3 {
		t.Errorf("Provenance.Line = %d, want 3 (the second occurrence)", prov.Line)
	}
	if prov.Column != 1 {
		t.Errorf("Provenance.Column = %d, want 1", prov.Column)
	}
	if prov.Layer != LayerGlobal || prov.Source != "/cfg/config.yml" {
		t.Errorf("Provenance layer/source = %q/%q, want %q/%q",
			prov.Layer, prov.Source, LayerGlobal, "/cfg/config.yml")
	}
}
