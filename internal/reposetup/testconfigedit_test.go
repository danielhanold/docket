package reposetup

import (
	"bytes"
	"strings"
	"testing"
)

// detectedOutcome/noneOutcome are the two edit-producing discovery outcomes.
func detectedOutcome(cmd string) DiscoveryOutcome {
	return DiscoveryOutcome{Kind: DiscoveryDetected, Command: cmd,
		Candidates: []DetectedSuite{{Family: "go", Command: cmd, Evidence: "go.mod"}}}
}

func noneOutcome() DiscoveryOutcome { return DiscoveryOutcome{Kind: DiscoveryNone} }

func TestConfigEditNilDetectedRendersMinimalFile(t *testing.T) {
	out, changed, err := RenderTestConfigEdit(nil, detectedOutcome("go test ./..."))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true for a fresh file")
	}
	want := "build:\n  gate: local\n  test_command: go test ./...\n" +
		"finalize:\n  gate: local\n  test_command: go test ./...\n"
	if !bytes.Equal(out, []byte(want)) {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestConfigEditNilNoneWritesQuotedOffNoCommand(t *testing.T) {
	out, changed, err := RenderTestConfigEdit(nil, noneOutcome())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	want := "build:\n  gate: \"off\"\nfinalize:\n  gate: \"off\"\n"
	if !bytes.Equal(out, []byte(want)) {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
	// Guard: the gate value MUST be the quoted scalar "off" (bare off is a YAML
	// boolean keyword — AGENTS.md). Asserted on the literal bytes.
	if !strings.Contains(string(out), `gate: "off"`) {
		t.Fatalf("gate off must be written quoted as `gate: \"off\"`; got:\n%s", out)
	}
	if strings.Contains(string(out), "test_command") {
		t.Fatalf("none must write NO test_command key; got:\n%s", out)
	}
}

func TestConfigEditPreservesUnrelatedKeysAndComments(t *testing.T) {
	existing := "# docket config\nintegration_branch: main\nchanges_dir: docs/changes   # inline\n"
	out, changed, err := RenderTestConfigEdit([]byte(existing), detectedOutcome("make test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	// Every original line is preserved verbatim as a prefix.
	if !bytes.HasPrefix(out, []byte(existing)) {
		t.Fatalf("unrelated keys/comments not byte-preserved as a prefix\n got: %q\nwant prefix: %q", out, existing)
	}
	for _, want := range []string{
		"# docket config", "integration_branch: main", "changes_dir: docs/changes   # inline",
		"build:\n  gate: local\n  test_command: make test\n",
		"finalize:\n  gate: local\n  test_command: make test\n",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
}

func TestConfigEditIsIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  DiscoveryOutcome
	}{
		{"detected", detectedOutcome("go test ./...")},
		{"none", noneOutcome()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, changed, err := RenderTestConfigEdit(nil, tc.out)
			if err != nil || !changed {
				t.Fatalf("first render: changed=%v err=%v", changed, err)
			}
			second, changed2, err := RenderTestConfigEdit(first, tc.out)
			if err != nil {
				t.Fatalf("second render error: %v", err)
			}
			if changed2 {
				t.Fatalf("second render must report changed=false (idempotent); got true\n%s", second)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf("second render must be byte-identical\nfirst:  %q\nsecond: %q", first, second)
			}
		})
	}
}

func TestConfigEditExistingMatchingSettingsUnchanged(t *testing.T) {
	// Existing file already carries the exact detected settings → no edit.
	existing := "build:\n  gate: local\n  test_command: go test ./...\n" +
		"finalize:\n  gate: local\n  test_command: go test ./...\n"
	out, changed, err := RenderTestConfigEdit([]byte(existing), detectedOutcome("go test ./..."))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false when settings already present")
	}
	if !bytes.Equal(out, []byte(existing)) {
		t.Fatalf("existing bytes must be returned unchanged\n got: %q\nwant: %q", out, existing)
	}
}

func TestConfigEditInsertsIntoExistingBlock(t *testing.T) {
	existing := "finalize:\n  gate: local\n"
	out, changed, err := RenderTestConfigEdit([]byte(existing), detectedOutcome("go test ./..."))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	// test_command inserted into the existing finalize block, after its gate line.
	if !strings.Contains(string(out), "finalize:\n  gate: local\n  test_command: go test ./...\n") {
		t.Fatalf("test_command not spliced into the existing finalize block:\n%s", out)
	}
	// The missing build block is appended.
	if !strings.Contains(string(out), "build:\n  gate: local\n  test_command: go test ./...\n") {
		t.Fatalf("missing build block not appended:\n%s", out)
	}
}

func TestConfigEditConfiguredAndAmbiguousAreNoOps(t *testing.T) {
	existing := []byte("integration_branch: main\n")
	for _, kind := range []DiscoveryKind{DiscoveryConfigured, DiscoveryAmbiguous} {
		out, changed, err := RenderTestConfigEdit(existing, DiscoveryOutcome{Kind: kind})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", kind, err)
		}
		if changed {
			t.Fatalf("%s: expected changed=false", kind)
		}
		if !bytes.Equal(out, existing) {
			t.Fatalf("%s: existing bytes must be returned unchanged", kind)
		}
	}
}

func TestConfigEditMalformedYAMLErrorsFileUntouched(t *testing.T) {
	bad := []byte("foo: [1, 2, 3\n")
	out, changed, err := RenderTestConfigEdit(bad, detectedOutcome("go test ./..."))
	if err == nil {
		t.Fatalf("expected an error for malformed YAML; got out=%q changed=%v", out, changed)
	}
	if out != nil || changed {
		t.Fatalf("error return must be (nil,false,err); got out=%q changed=%v", out, changed)
	}
}
