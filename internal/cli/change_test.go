package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// runCLIStdin is runCLI with caller-supplied stdin, so a `--request -` command
// can be driven with an in-memory JSON body.
func runCLIStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, strings.NewReader(stdin), &out, &errBuf, devInfo(), hostFacts())
	return out.String(), errBuf.String(), code
}

// TestChangeCommandsRegistered is the registration assertion: `docket change`
// carries exactly the five settled subcommands, each with a required --request
// flag and a --repo-dir flag, and the bare group reports a missing command.
func TestChangeCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"create", "groom", "block", "defer", "kill"} {
		cmd, _, err := root.Find([]string{"change", sub})
		if err != nil || cmd == nil || cmd.Name() != sub {
			t.Fatalf("change %s not registered: cmd=%v err=%v", sub, cmd, err)
		}
		if cmd.Flags().Lookup("request") == nil {
			t.Errorf("change %s: missing --request flag", sub)
		}
		if cmd.Flags().Lookup("repo-dir") == nil {
			t.Errorf("change %s: missing --repo-dir flag", sub)
		}
	}
	// The group itself resolves to a command (not an unknown-command error).
	grp, _, err := root.Find([]string{"change"})
	if err != nil || grp == nil || grp.Name() != "change" {
		t.Fatalf("change group not registered: grp=%v err=%v", grp, err)
	}
}

// TestChangeRequestFlagRequired proves the --request flag is required: omitting
// it fails as an argument error (exit 2) that names the flag, before any
// operation runs.
func TestChangeRequestFlagRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "create")
	if code != 2 || !strings.Contains(errS, "request") {
		t.Fatalf("err=%q code=%d", errS, code)
	}
}

// TestChangeCreateUnknownFieldRejected proves --request decodes with
// DisallowUnknownFields: an unknown JSON field is invalid input, exit 2, one
// document, and no engine is reached.
func TestChangeCreateUnknownFieldRejected(t *testing.T) {
	out, errS, code := runCLIStdin(t, `{"title":"x","bogus_field":1}`, "change", "create", "--request", "-", "--json")
	if code != 2 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("stdout=%q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be one newline-terminated document, got %q", out)
	}
}

// TestChangeCommandsReachOperation is the wiring assertion for all five
// commands: a well-formed (if semantically empty) JSON request read from stdin
// decodes into the operation's own request struct and is handed to that
// operation, which returns exactly one protocol-v1 document naming it. A `{}`
// body fails each operation's up-front request-shape validation, so this reaches
// the operation without needing a live repository.
func TestChangeCommandsReachOperation(t *testing.T) {
	cases := []struct{ sub, op string }{
		{"create", "change.create"},
		{"groom", "change.groom"},
		{"block", "change.block"},
		{"defer", "change.defer"},
		{"kill", "change.kill"},
	}
	for _, c := range cases {
		out, errS, code := runCLIStdin(t, `{}`, "change", c.sub, "--request", "-", "--repo-dir", testsupport.TempDir(t), "--json")
		if errS != "" {
			t.Fatalf("%s: unexpected stderr %q (code=%d)", c.sub, errS, code)
		}
		if !strings.Contains(out, `"operation":"`+c.op+`"`) {
			t.Fatalf("%s: document did not name the operation: %q", c.sub, out)
		}
		if !strings.Contains(out, `"protocol_version":1`) {
			t.Fatalf("%s: missing protocol version: %q", c.sub, out)
		}
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			t.Fatalf("%s: must be exactly one newline-terminated document, got %q", c.sub, out)
		}
	}
}

// TestChangeRequestFromFile proves the --request path form reads a JSON file
// (not only stdin) and reaches the operation.
func TestChangeRequestFromFile(t *testing.T) {
	dir := testsupport.TempDir(t)
	reqPath := filepath.Join(dir, "req.json")
	if err := os.WriteFile(reqPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errS, code := runCLI(t, "change", "create", "--request", reqPath, "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"change.create"`) {
		t.Fatalf("stdout=%q", out)
	}
}

// TestChangeRequestFileMissing proves an unreadable --request path is an
// argument error (exit 2) rather than a panic or a half-formed document.
func TestChangeRequestFileMissing(t *testing.T) {
	_, errS, code := runCLI(t, "change", "create", "--request", filepath.Join(testsupport.TempDir(t), "nope.json"))
	if code != 2 {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(errS, "request") {
		t.Fatalf("err=%q", errS)
	}
}

// TestChangeCommandsAssetIndependent guards the install.go registration: each
// change command must be in the asset-independent set (they read the repository,
// never installed assets), so they are not refused on a machine with no
// installation.
func TestChangeCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"change", "change create", "change groom", "change block", "change defer", "change kill", "change claim", "change refresh-claim", "change reconcile"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}

// TestChangeClaimCommandsRegistered proves claim and refresh-claim are wired as
// change subcommands carrying the scalar --id/--version flags (no --request:
// they carry no authored Markdown).
func TestChangeClaimCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"claim", "refresh-claim"} {
		cmd, _, err := root.Find([]string{"change", sub})
		if err != nil || cmd == nil || cmd.Name() != sub {
			t.Fatalf("change %s not registered: cmd=%v err=%v", sub, cmd, err)
		}
		if cmd.Flags().Lookup("id") == nil {
			t.Errorf("change %s: missing --id flag", sub)
		}
		if cmd.Flags().Lookup("version") == nil {
			t.Errorf("change %s: missing --version flag", sub)
		}
	}
}

// TestChangeReconcileRegistered proves reconcile is wired as a change subcommand
// carrying the scalar --input request-file flag (authored Markdown rides in the
// JSON body, never shell-escaped flags).
func TestChangeReconcileRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"change", "reconcile"})
	if err != nil || cmd == nil || cmd.Name() != "reconcile" {
		t.Fatalf("change reconcile not registered: cmd=%v err=%v", cmd, err)
	}
	if cmd.Flags().Lookup("input") == nil {
		t.Errorf("change reconcile: missing --input flag")
	}
	if cmd.Flags().Lookup("repo-dir") == nil {
		t.Errorf("change reconcile: missing --repo-dir flag")
	}
}

// TestChangeReconcileInputFlagRequired proves --input is required: omitting it is
// an argument error (exit 2) that names the flag, before any operation runs.
func TestChangeReconcileInputFlagRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "reconcile")
	if code != 2 || !strings.Contains(errS, "input") {
		t.Fatalf("err=%q code=%d", errS, code)
	}
}

// TestChangeReconcileReachesOperation proves reconcile decodes its --input body
// and reaches the operation, which returns exactly one protocol-v1 document
// naming it. A `{}` body fails the up-front shape validation (missing version /
// log entry), so this reaches the operation without a live repository.
func TestChangeReconcileReachesOperation(t *testing.T) {
	out, errS, code := runCLIStdin(t, `{}`, "change", "reconcile", "--input", "-", "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"change.reconcile"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestChangeReconcileUnknownFieldRejected proves --input decodes with
// DisallowUnknownFields: an unknown JSON field is invalid input, exit 2.
func TestChangeReconcileUnknownFieldRejected(t *testing.T) {
	_, errS, code := runCLIStdin(t, `{"id":1,"nope":true}`, "change", "reconcile", "--input", "-", "--json")
	if code != 2 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
}

// TestInputDecodeErrorNamesInputFlag proves a malformed --input body's error
// names the flag the caller actually passed, never --request. The message
// rides the JSON result document on stdout (--json mode), which is why the
// assertion reads stdout rather than stderr.
func TestInputDecodeErrorNamesInputFlag(t *testing.T) {
	out, _, code := runCLIStdin(t, `{not json`, "change", "reconcile", "--input", "-", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout %q)", code, out)
	}
	if !strings.Contains(out, "--input") || strings.Contains(out, "--request") {
		t.Errorf("decode error must name --input and not --request: %q", out)
	}
}

// TestUnknownFieldErrorListsAcceptedKeys proves an unknown-field refusal
// teaches the caller the real key set instead of only naming the bad key.
func TestUnknownFieldErrorListsAcceptedKeys(t *testing.T) {
	out, _, code := runCLIStdin(t, `{"change_id":1}`, "change", "reconcile", "--input", "-", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout %q)", code, out)
	}
	for _, key := range []string{"id", "version", "sections", "spec_sections", "relations", "reconcile_log_entry"} {
		if !strings.Contains(out, key) {
			t.Errorf("unknown-field error must list accepted key %q: %q", key, out)
		}
	}
}

// TestChangeClaimFlagsRequired proves --id and --version are required: omitting
// them is an argument error (exit 2) before any operation runs.
func TestChangeClaimFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "claim")
	if code != 2 || (!strings.Contains(errS, "id") && !strings.Contains(errS, "version")) {
		t.Fatalf("err=%q code=%d", errS, code)
	}
}

// TestChangeClaimCommandsReachOperation proves both claim commands decode their
// flags and reach the operation, which returns exactly one protocol-v1 document
// naming it. A bare tempdir is no docket repo, so the operation fails past its
// shape check — but only after naming itself.
func TestChangeClaimCommandsReachOperation(t *testing.T) {
	cases := []struct{ sub, op string }{
		{"claim", "change.claim"},
		{"refresh-claim", "change.refresh-claim"},
	}
	for _, c := range cases {
		out, errS, code := runCLI(t, "change", c.sub,
			"--id", "7", "--version", "1234123412341234123412341234123412341234",
			"--repo-dir", testsupport.TempDir(t), "--json")
		_ = code
		if errS != "" {
			t.Fatalf("%s: unexpected stderr %q", c.sub, errS)
		}
		if !strings.Contains(out, `"operation":"`+c.op+`"`) {
			t.Fatalf("%s: document did not name the operation: %q", c.sub, out)
		}
		if !strings.Contains(out, `"protocol_version":1`) {
			t.Fatalf("%s: missing protocol version: %q", c.sub, out)
		}
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			t.Fatalf("%s: must be exactly one newline-terminated document, got %q", c.sub, out)
		}
	}
}

// TestChangeAttachCommandsRegistered proves attach-plan and attach-results are
// wired as change subcommands carrying the scalar --id/--version/--path/--commit
// flags (no --request: they name a verified Git artifact, not authored Markdown).
func TestChangeAttachCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"attach-plan", "attach-results"} {
		cmd, _, err := root.Find([]string{"change", sub})
		if err != nil || cmd == nil || cmd.Name() != sub {
			t.Fatalf("change %s not registered: cmd=%v err=%v", sub, cmd, err)
		}
		for _, flag := range []string{"id", "version", "path", "commit"} {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("change %s: missing --%s flag", sub, flag)
			}
		}
		if !assetIndependent["change "+sub] {
			t.Errorf("change %s is not registered asset-independent", sub)
		}
	}
}

// TestChangeMarkImplementedRegistered proves mark-implemented is wired as a
// change subcommand carrying its scalar identities plus the --evidence file flag,
// and is registered asset-independent.
func TestChangeMarkImplementedRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"change", "mark-implemented"})
	if err != nil || cmd == nil || cmd.Name() != "mark-implemented" {
		t.Fatalf("change mark-implemented not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "head", "pr", "evidence", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("change mark-implemented: missing --%s flag", flag)
		}
	}
	if !assetIndependent["change mark-implemented"] {
		t.Errorf("change mark-implemented is not registered asset-independent")
	}
}

// TestChangeMarkImplementedFlagsRequired proves the identity/evidence flags are
// required: omitting them is an argument error (exit 2) before any operation runs.
func TestChangeMarkImplementedFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "mark-implemented")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
}

// TestChangeMarkImplementedReachesOperation proves the command decodes its flags
// and evidence file and reaches the operation, which returns exactly one
// protocol-v1 document naming it. A prose-only evidence file does not verify,
// which is still one well-formed document that names change.mark-implemented.
func TestChangeMarkImplementedReachesOperation(t *testing.T) {
	dir := testsupport.TempDir(t)
	evFile := filepath.Join(dir, "evidence.md")
	if err := os.WriteFile(evFile, []byte("# just prose, no block\n"), 0o644); err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
	out, errS, _ := runCLI(t, "change", "mark-implemented",
		"--id", "7",
		"--version", "1234123412341234123412341234123412341234",
		"--head", "1111111111111111111111111111111111111111",
		"--pr", "github.com/acme/widget#42",
		"--evidence", evFile,
		"--repo-dir", dir, "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"change.mark-implemented"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestChangeAttachFlagsRequired proves the scalar flags are required: omitting
// them is an argument error (exit 2) before any operation runs.
func TestChangeAttachFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "attach-plan")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
}

// TestChangeAttachCommandsReachOperation proves both attach commands decode their
// flags and reach the operation, which returns exactly one protocol-v1 document
// naming it. A bare tempdir is no docket repo, so the operation fails past its
// shape check — but only after naming itself.
func TestChangeAttachCommandsReachOperation(t *testing.T) {
	cases := []struct{ sub, op string }{
		{"attach-plan", "change.attach-plan"},
		{"attach-results", "change.attach-results"},
	}
	for _, c := range cases {
		out, errS, _ := runCLI(t, "change", c.sub,
			"--id", "7", "--version", "1234123412341234123412341234123412341234",
			"--path", "docs/superpowers/plans/x.md", "--commit", "1234123412341234123412341234123412341234",
			"--repo-dir", testsupport.TempDir(t), "--json")
		if errS != "" {
			t.Fatalf("%s: unexpected stderr %q", c.sub, errS)
		}
		if !strings.Contains(out, `"operation":"`+c.op+`"`) {
			t.Fatalf("%s: document did not name the operation: %q", c.sub, out)
		}
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			t.Fatalf("%s: must be exactly one newline-terminated document, got %q", c.sub, out)
		}
	}
}

// --- change halt / resume-halted -------------------------------------------

// TestChangeHaltRegistered proves `change halt` is wired with the scalar identity
// flags plus the --input request-file flag (the authored report rides in the
// file, never shell-escaped flags), and is asset-independent.
func TestChangeHaltRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"change", "halt"})
	if err != nil || cmd == nil || cmd.Name() != "halt" {
		t.Fatalf("change halt not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "input", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("change halt: missing --%s flag", flag)
		}
	}
	if !assetIndependent["change halt"] {
		t.Errorf("%q is not registered asset-independent", "change halt")
	}
}

// TestChangeHaltReachesOperation proves halt decodes its --input body and reaches
// the operation, which returns exactly one protocol-v1 document naming it. An
// empty report fails the up-front shape check, reaching the operation without a
// live repository.
func TestChangeHaltReachesOperation(t *testing.T) {
	out, errS, _ := runCLIStdin(t, `{}`, "change", "halt",
		"--id", "3", "--version", "1234123412341234123412341234123412341234", "--input", "-", "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"change.halt"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestChangeResumeHaltedRegistered proves `change resume-halted` is wired with the
// scalar identity flags and the --acknowledge-quiescent gate flag, and is
// asset-independent.
func TestChangeResumeHaltedRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"change", "resume-halted"})
	if err != nil || cmd == nil || cmd.Name() != "resume-halted" {
		t.Fatalf("change resume-halted not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "acknowledge-quiescent", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("change resume-halted: missing --%s flag", flag)
		}
	}
	if !assetIndependent["change resume-halted"] {
		t.Errorf("%q is not registered asset-independent", "change resume-halted")
	}
}

// TestChangeResumeHaltedReachesOperation proves resume-halted decodes its flags
// and reaches the operation. Without --acknowledge-quiescent the operation
// refuses before any effect, naming itself in one protocol-v1 document — so no
// live repository is consulted.
func TestChangeResumeHaltedReachesOperation(t *testing.T) {
	out, errS, _ := runCLI(t, "change", "resume-halted",
		"--id", "3", "--version", "1234123412341234123412341234123412341234", "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"change.resume-halted"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"reason":"quiescence-not-acknowledged"`) {
		t.Fatalf("resume without acknowledgement should refuse: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// --- change reclaim --------------------------------------------------------

// TestChangeReclaimRegistered proves `change reclaim` is wired with the scalar
// identity flags and is asset-independent.
func TestChangeReclaimRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"change", "reclaim"})
	if err != nil || cmd == nil || cmd.Name() != "reclaim" {
		t.Fatalf("change reclaim not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("change reclaim: missing --%s flag", flag)
		}
	}
	if !assetIndependent["change reclaim"] {
		t.Errorf("%q is not registered asset-independent", "change reclaim")
	}
}

// TestChangeReclaimFlagsRequired proves --id and --version are required: omitting
// them is an argument error (exit 2) before any operation runs.
func TestChangeReclaimFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "reclaim")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
}

// TestChangeReclaimReachesOperation proves reclaim decodes its scalar flags and
// reaches the operation, which returns exactly one protocol-v1 document naming
// it. A non-repository repo-dir refuses while pinning context, so no live
// metadata is mutated.
func TestChangeReclaimReachesOperation(t *testing.T) {
	out, errS, _ := runCLI(t, "change", "reclaim",
		"--id", "3", "--version", "1234123412341234123412341234123412341234", "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"change.reclaim"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestChangeRepairIdentityRegistered proves repair-identity is wired as a change
// subcommand carrying its scalar identity, version pin, and the two mode/evidence
// flag sets (no --request: the op writes one frontmatter field, not authored
// Markdown), and is registered asset-independent.
func TestChangeRepairIdentityRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"change", "repair-identity"})
	if err != nil || cmd == nil || cmd.Name() != "repair-identity" {
		t.Fatalf("change repair-identity not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "expect-version", "adopt-pr-head", "expect-pr", "expect-head", "adopt-pr", "expect-branch", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("change repair-identity: missing --%s flag", flag)
		}
	}
	if !assetIndependent["change repair-identity"] {
		t.Errorf("change repair-identity is not registered asset-independent")
	}
}

// TestChangeRepairIdentityFlagsRequired proves --id and --expect-version are
// required: omitting them is an argument error (exit 2) before any operation runs.
func TestChangeRepairIdentityFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "repair-identity")
	if code != 2 || (!strings.Contains(errS, "id") && !strings.Contains(errS, "expect-version")) {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
}

// TestChangeRepairIdentityReachesOperation proves the command decodes its flags
// and reaches the operation, which owns the mode/evidence validation: a request
// naming neither mode is refused as invalid-request in exactly one protocol-v1
// document naming the operation, before any repository is touched.
func TestChangeRepairIdentityReachesOperation(t *testing.T) {
	out, errS, code := runCLI(t, "change", "repair-identity",
		"--id", "3", "--expect-version", "1234123412341234123412341234123412341234",
		"--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"change.repair-identity"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"reason":"invalid-request"`) {
		t.Fatalf("neither-mode request was not refused as invalid-request: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}
