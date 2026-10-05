package reposetup

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
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

// TestConfigEditDivergentExplicitPairSurvivesUntouched is change 0374's
// own-thesis guard (Task 15): the build and finalize commands are owned
// INDEPENDENTLY, so a file whose explicit build.test_command differs from its
// finalize.test_command survives a re-run byte-for-byte. A configured outcome
// (what an already-explicit pair resolves to) writes nothing — the renderer
// never re-couples a deliberately divergent pair into one shared command.
func TestConfigEditDivergentExplicitPairSurvivesUntouched(t *testing.T) {
	existing := []byte("build:\n  gate: local\n  test_command: make build-suite\n" +
		"finalize:\n  gate: local\n  test_command: make finalize-suite\n")
	out, changed, err := RenderTestConfigEdit(existing, DiscoveryOutcome{Kind: DiscoveryConfigured})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("a configured divergent pair must produce no edit")
	}
	if !bytes.Equal(out, existing) {
		t.Fatalf("divergent build/finalize commands must survive byte-for-byte\n got: %q\nwant: %q", out, existing)
	}
}

// TestPolicyEditDivergentExplicitPairShortCircuitsDiscovery drives the same
// independence claim through the full setup-time pipeline: an explicit divergent
// pair short-circuits discovery (no probing) and yields NO pending edit even
// when the tree WOULD detect a different command if it were probed. This is the
// mutation-sensitive re-coupling guard — dropping the configured short-circuit
// would let discovery detect `go test ./...` and rewrite BOTH divergent
// commands to it.
func TestPolicyEditDivergentExplicitPairShortCircuitsDiscovery(t *testing.T) {
	cfg := buildTestPolicyCfg("local", "make build-suite", "local", "make finalize-suite")
	existing := []byte("build:\n  gate: local\n  test_command: make build-suite\n" +
		"finalize:\n  gate: local\n  test_command: make finalize-suite\n")
	// A tree a probe would detect as `go test ./...` — so a non-nil edit proves
	// discovery was NOT short-circuited.
	tree := mapTree{"go.mod": "module x", "x_test.go": ""}

	edited, outcome, err := TestPolicyEdit(cfg, existing, tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Kind != DiscoveryConfigured {
		t.Fatalf("outcome kind = %q, want configured (an explicit pair short-circuits discovery)", outcome.Kind)
	}
	if edited != nil {
		t.Fatalf("a configured divergent pair must produce no pending edit; got:\n%s", edited)
	}
}

// TestConfigEditPreservesExplicitNonLocalFinalizeGate is the regression guard for
// the migrate/detected clobber: a detected outcome (the migrate copy path) writes
// gate: local into BOTH owners, but a block whose gate is ALREADY explicitly set
// to a non-local value (off/ci/both — a legacy repo that deliberately disabled or
// retargeted the finalize gate) must keep that value. Only a MISSING gate is
// filled with local; an explicit divergent gate is preserved (spec: already-explicit
// new-style settings survive). TestConfigEditInsertsIntoExistingBlock starts from
// gate: local, so it never exercised the rewrite path this guards.
func TestConfigEditPreservesExplicitNonLocalFinalizeGate(t *testing.T) {
	for _, gate := range []string{"off", "ci", "both"} {
		t.Run(gate, func(t *testing.T) {
			existing := []byte("build:\n  gate: local\n  test_command: make suite\n" +
				"finalize:\n  gate: " + gate + "\n  test_command: make suite\n")
			out, changed, err := RenderTestConfigEdit(existing, detectedOutcome("make suite"))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// build already carries the detected settings and finalize's only
			// divergence is its explicit gate, which must be preserved — so the
			// whole render is a no-op.
			if changed {
				t.Fatalf("an explicit finalize.gate %q must be preserved (no edit); got changed=true:\n%s", gate, out)
			}
			if !bytes.Equal(out, existing) {
				t.Fatalf("explicit finalize.gate %q must survive byte-for-byte\n got: %q\nwant: %q", gate, out, existing)
			}
		})
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

// explicitBlock is the exact owner-block text the explicit path writes for cmd.
func explicitBlock(owner, cmd string) string {
	return owner + ":\n  gate: local\n  test_command: " + cmd + "\n"
}

func TestExplicitEditNoFileRendersBothLocalGates(t *testing.T) { // (a)
	out, changed, err := RenderExplicitTestCommandEdit(nil, "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want a fresh file", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditOverwritesOffGates(t *testing.T) { // (b)
	existing := "# top comment\nintegration_branch: main  # inline\n" +
		"build:\n  gate: \"off\"\nfinalize:\n  gate: \"off\"\n"
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit", changed, err)
	}
	want := "# top comment\nintegration_branch: main  # inline\n" +
		explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditReplacesDifferentCommand(t *testing.T) { // (c)
	existing := explicitBlock("build", "make check") + explicitBlock("finalize", "make check")
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditCompletesHalfConfiguredPair(t *testing.T) { // (d)
	existing := explicitBlock("build", "sh ./test.sh") + "finalize:\n  gate: local\n"
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditOverwritesDivergentExplicitGateOnOneOwner(t *testing.T) { // (e)
	existing := explicitBlock("build", "sh ./test.sh") +
		"finalize:\n  gate: \"off\"\n  test_command: sh ./test.sh\n"
	out, changed, err := RenderExplicitTestCommandEdit([]byte(existing), "sh ./test.sh")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want an edit (explicit command overwrites the gate)", changed, err)
	}
	want := explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh")
	if string(out) != want {
		t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

func TestExplicitEditAlreadyEqualIsNoChange(t *testing.T) { // (f)
	existing := []byte("integration_branch: main\n" +
		explicitBlock("build", "sh ./test.sh") + explicitBlock("finalize", "sh ./test.sh"))
	out, changed, err := RenderExplicitTestCommandEdit(existing, "sh ./test.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed || !bytes.Equal(out, existing) {
		t.Fatalf("an already-equal file must be untouched: changed=%v\n got: %q", changed, out)
	}
}

func TestExplicitEditMalformedYAMLErrors(t *testing.T) {
	existing := []byte("build: [unclosed\n")
	out, changed, err := RenderExplicitTestCommandEdit(existing, "sh ./test.sh")
	if err == nil || changed || out != nil {
		t.Fatalf("malformed YAML must error with no edit: out=%q changed=%v err=%v", out, changed, err)
	}
}

func TestExplicitTestCommandRefusesInvalidInput(t *testing.T) {
	for _, raw := range []string{"", "   ", "\t\n", "auto", "  auto  ",
		"make\ntest", "make\ttest", "make\rtest", "a\u2028b", "a\u2029b", "a\u0085b", "\xff"} {
		got, err := ExplicitTestCommand(raw)
		if err == nil {
			t.Errorf("ExplicitTestCommand(%q) = %q, want a refusal", raw, got)
			continue
		}
		if !errors.Is(err, ErrInvalidTestCommand) {
			t.Errorf("ExplicitTestCommand(%q) error %v must wrap ErrInvalidTestCommand", raw, err)
		}
	}
}

func TestExplicitTestCommandTrims(t *testing.T) {
	got, err := ExplicitTestCommand("  sh ./test.sh  ")
	if err != nil || got != "sh ./test.sh" {
		t.Fatalf("ExplicitTestCommand = %q, %v; want the trimmed command", got, err)
	}
}

// TestExplicitCommandRoundTripsThroughConfigResolve is the reader-correspondence
// guard: every accepted command, rendered into .docket.yml, must resolve through
// the REAL config resolver (which types test_command as a !!str leaf) back to
// exactly that command with both gates local. Digit-led tokens such as 123 or
// 2026-10-05 are the cases a plain-charset check renders bare and the YAML
// resolver retypes.
func TestExplicitCommandRoundTripsThroughConfigResolve(t *testing.T) {
	cmds := []string{
		"sh ./test.sh", "123", "1.5", "2026-10-05", "0x10", "1e3",
		"off", "true", "null", "yes", "auto-test",
		`pytest -k "a and b"`, `echo 'x' \ y`, "make test # tail", "npm test -- --watch=false",
		"a: b", "{x}", "[x]", "- dash", "ünï test", "@at", "`tick`", "&anchor", "*alias",
		"!tag", "%pct", "|pipe", ">gt", `bash -c 'set -e; for t in tests/test_*.sh; do bash "$t"; done'`,
	}
	bases := map[string][]byte{
		"no file":  nil,
		"off file": []byte("integration_branch: main\nbuild:\n  gate: \"off\"\nfinalize:\n  gate: \"off\"\n"),
	}
	for _, cmd := range cmds {
		valid, err := ExplicitTestCommand(cmd)
		if err != nil {
			t.Fatalf("ExplicitTestCommand(%q) refused a legal command: %v", cmd, err)
		}
		for name, base := range bases {
			out, _, err := RenderExplicitTestCommandEdit(base, valid)
			if err != nil {
				t.Fatalf("%s / %q: render error %v", name, cmd, err)
			}
			snap, _, err := config.Resolve([]config.Source{{
				Layer: config.LayerRepository, Name: ".docket.yml", Data: out,
			}}, config.ResolveContext{DefaultBranch: "main"})
			if err != nil {
				t.Fatalf("%s / %q: rendered file does not resolve: %v\n%s", name, cmd, err, out)
			}
			eff := snap.Effective
			if eff.Build.TestCommand.Value != cmd || eff.Finalize.TestCommand.Value != cmd {
				t.Errorf("%s / %q: resolved build=%q finalize=%q\n%s", name, cmd,
					eff.Build.TestCommand.Value, eff.Finalize.TestCommand.Value, out)
			}
			if eff.Build.Gate.Value != "local" || eff.Finalize.Gate.Value != "local" {
				t.Errorf("%s / %q: resolved gates build=%q finalize=%q, want local", name, cmd,
					eff.Build.Gate.Value, eff.Finalize.Gate.Value)
			}
		}
	}
}

// unsplicableBases are existing .docket.yml shapes whose leaf or owner spans
// lines the line-splice planner cannot see (a folded or literal block-scalar
// test_command carries no child line nodes) or shares its line with the owner
// key (a flow-style owner mapping). Each must either render to a file that
// resolves to exactly the desired pairs, or refuse with no output.
var unsplicableBases = map[string][]byte{
	"folded test_command":  []byte("build:\n  gate: \"off\"\n  test_command: >-\n    sh ./test.sh\n    make test\nfinalize:\n  gate: \"off\"\n"),
	"literal test_command": []byte("build:\n  gate: local\n  test_command: |\n    sh ./test.sh\n    make test\nfinalize:\n  gate: local\n  test_command: |\n    sh ./test.sh\n"),
	"flow-style build":     []byte("integration_branch: main\nbuild: {gate: \"off\"}\nfinalize:\n  gate: \"off\"\n"),
	"folded last in file":  []byte("finalize:\n  gate: \"off\"\nbuild:\n  test_command: >\n    old\n    command\n"),
}

// TestExplicitCommandOnUnsplicableBasesResolvesOrRefuses is the post-splice
// re-parse guard: an edit that would leave block-scalar continuation lines
// behind (silently changing the command, or breaking the YAML) or delete a
// flow-style owner key must be refused rather than written.
func TestExplicitCommandOnUnsplicableBasesResolvesOrRefuses(t *testing.T) {
	const cmd = "make test"
	for name, base := range unsplicableBases {
		out, changed, err := RenderExplicitTestCommandEdit(base, cmd)
		if err != nil {
			if out != nil || changed {
				t.Errorf("%s: refusal returned out=%q changed=%v; want (nil, false)", name, out, changed)
			}
			continue
		}
		snap, _, rerr := config.Resolve([]config.Source{{
			Layer: config.LayerRepository, Name: ".docket.yml", Data: out,
		}}, config.ResolveContext{DefaultBranch: "main"})
		if rerr != nil {
			t.Errorf("%s: rendered file does not resolve: %v\n%s", name, rerr, out)
			continue
		}
		eff := snap.Effective
		if eff.Build.TestCommand.Value != cmd || eff.Finalize.TestCommand.Value != cmd ||
			eff.Build.Gate.Value != "local" || eff.Finalize.Gate.Value != "local" {
			t.Errorf("%s: resolved build=(%q,%q) finalize=(%q,%q), want (local,%q) for both\n%s", name,
				eff.Build.Gate.Value, eff.Build.TestCommand.Value,
				eff.Finalize.Gate.Value, eff.Finalize.TestCommand.Value, cmd, out)
		}
	}
}

// TestDiscoveryRenderOnUnsplicableBasesResolvesOrRefuses covers the same
// splice core through the discovery render, where gate is preserve-explicit.
func TestDiscoveryRenderOnUnsplicableBasesResolvesOrRefuses(t *testing.T) {
	for name, base := range unsplicableBases {
		out, changed, err := RenderTestConfigEdit(base, detectedOutcome("go test ./..."))
		if err != nil {
			if out != nil || changed {
				t.Errorf("%s: refusal returned out=%q changed=%v; want (nil, false)", name, out, changed)
			}
			continue
		}
		snap, _, rerr := config.Resolve([]config.Source{{
			Layer: config.LayerRepository, Name: ".docket.yml", Data: out,
		}}, config.ResolveContext{DefaultBranch: "main"})
		if rerr != nil {
			t.Errorf("%s: rendered file does not resolve: %v\n%s", name, rerr, out)
			continue
		}
		eff := snap.Effective
		if eff.Build.TestCommand.Value != "go test ./..." || eff.Finalize.TestCommand.Value != "go test ./..." {
			t.Errorf("%s: resolved build=%q finalize=%q\n%s", name,
				eff.Build.TestCommand.Value, eff.Finalize.TestCommand.Value, out)
		}
	}
}
