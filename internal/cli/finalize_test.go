package cli

import (
	"strings"
	"testing"
)

// TestFinalizeRetargetChildrenRegistered proves the subcommand is wired under the
// finalize group carrying the scalar --id/--version identity flags and the
// --input request-file flag (the authored authorization set rides in the file,
// never shell-escaped flags), plus --repo-dir.
func TestFinalizeRetargetChildrenRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"finalize", "retarget-children"})
	if err != nil || cmd == nil || cmd.Name() != "retarget-children" {
		t.Fatalf("finalize retarget-children not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "input", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("finalize retarget-children: missing --%s flag", flag)
		}
	}
}

// TestFinalizeRetargetChildrenAssetIndependent guards the install.go registration:
// the command reads the repository, never installed assets.
func TestFinalizeRetargetChildrenAssetIndependent(t *testing.T) {
	if !assetIndependent["finalize retarget-children"] {
		t.Errorf("%q is not registered asset-independent", "finalize retarget-children")
	}
}

// TestFinalizeRetargetChildrenFlagsRequired proves --id, --version, and --input are
// required: omitting them is an argument error (exit 2) before any operation runs.
func TestFinalizeRetargetChildrenFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "finalize", "retarget-children")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
	for _, flag := range []string{"id", "version", "input"} {
		if !strings.Contains(errS, flag) {
			t.Errorf("required-flag error does not name %q: %q", flag, errS)
		}
	}
}

// TestFinalizeRetargetChildrenReachesOperation proves the command decodes its flags
// and --input body and reaches the operation, which returns exactly one protocol-v1
// document naming it. A bare tempdir is no docket repo, so the operation fails past
// its shape check — but only after naming itself.
func TestFinalizeRetargetChildrenReachesOperation(t *testing.T) {
	out, errS, _ := runCLIStdin(t, `{"children":[{"id":81,"pr_number":810,"pr_version":"cv810"}]}`,
		"finalize", "retarget-children",
		"--id", "80", "--version", "1234123412341234123412341234123412341234",
		"--input", "-", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.retarget-children"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestFinalizeRetargetChildrenUnknownFieldRejected proves --input decodes with
// DisallowUnknownFields: an unknown JSON key (e.g. a scalar identity that belongs
// on a flag, not in the authored set) is an argument error (exit 2).
func TestFinalizeRetargetChildrenUnknownFieldRejected(t *testing.T) {
	_, errS, code := runCLIStdin(t, `{"id":80,"children":[]}`,
		"finalize", "retarget-children",
		"--id", "80", "--version", "1234123412341234123412341234123412341234",
		"--input", "-", "--json")
	if code != 2 || errS != "" {
		t.Fatalf("err=%q code=%d, want an invalid-input argument error", errS, code)
	}
}

// TestFinalizeGroupRegistered: the top-level `finalize` command group is
// registered and reports a missing command when invoked bare, so the
// terminal-half mutation subcommands added in later tasks have a tree to attach
// to.
func TestFinalizeGroupRegistered(t *testing.T) {
	root := captureTree(t)

	grp, _, err := root.Find([]string{"finalize"})
	if err != nil || grp == nil || grp.Name() != "finalize" {
		t.Fatalf("finalize group not registered: grp=%v err=%v", grp, err)
	}
}

// TestFinalizeGroupBareReportsMissingCommand: `docket finalize` with no
// subcommand is a missing-command argument error (exit 2), not a silent success.
func TestFinalizeGroupBareReportsMissingCommand(t *testing.T) {
	_, errS, code := runCLI(t, "finalize")
	if code != 2 {
		t.Fatalf("bare finalize exit code = %d, want 2", code)
	}
	if !strings.Contains(errS, "missing command") {
		t.Fatalf("bare finalize stderr = %q, want a missing-command error", errS)
	}
}

// TestFinalizeGroupAssetIndependent guards the install.go registration: the
// finalize commands operate on the repository, never installed assets, so the
// group must be asset-independent.
func TestFinalizeGroupAssetIndependent(t *testing.T) {
	if !assetIndependent["finalize"] {
		t.Errorf("%q is not registered asset-independent", "finalize")
	}
}

// TestFinalizeRebaseRegistered proves `finalize rebase` is wired with its scalar
// identity flags (no authored request body) plus --repo-dir.
func TestFinalizeRebaseRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"finalize", "rebase"})
	if err != nil || cmd == nil || cmd.Name() != "rebase" {
		t.Fatalf("finalize rebase not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "head", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("finalize rebase: missing --%s flag", flag)
		}
	}
}

// TestFinalizeRebaseResolverSubcommandsRegistered proves rebase-continue and
// rebase-abort are wired with the scalar identity (id, attempt) on flags and the
// authored resolver report in --input (never argv).
func TestFinalizeRebaseResolverSubcommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, name := range []string{"rebase-continue", "rebase-abort"} {
		cmd, _, err := root.Find([]string{"finalize", name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("finalize %s not registered: cmd=%v err=%v", name, cmd, err)
		}
		for _, flag := range []string{"id", "attempt", "input", "repo-dir"} {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("finalize %s: missing --%s flag", name, flag)
			}
		}
	}
}

// TestFinalizeRebaseSubcommandsAssetIndependent guards the install.go registration
// for the three rebase subcommands: they read the repository, never installed assets.
func TestFinalizeRebaseSubcommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"finalize rebase", "finalize rebase-continue", "finalize rebase-abort"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}

// TestFinalizeRebaseFlagsRequired proves --id, --version, and --head are required.
func TestFinalizeRebaseFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "finalize", "rebase")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
	for _, flag := range []string{"id", "version", "head"} {
		if !strings.Contains(errS, flag) {
			t.Errorf("required-flag error does not name %q: %q", flag, errS)
		}
	}
}

// TestFinalizeRebaseReachesOperation proves the command decodes its flags and
// reaches the operation, which emits exactly one protocol-v1 document naming it.
func TestFinalizeRebaseReachesOperation(t *testing.T) {
	out, errS, _ := runCLI(t, "finalize", "rebase",
		"--id", "80", "--version", "1234123412341234123412341234123412341234",
		"--head", "abcabcabcabcabcabcabcabcabcabcabcabcabca", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.rebase"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestFinalizeRebaseContinueReachesOperation proves the command decodes the
// resolver report from --input and reaches the operation, emitting one document.
func TestFinalizeRebaseContinueReachesOperation(t *testing.T) {
	out, errS, _ := runCLIStdin(t,
		`{"change_id":80,"attempt":"a1","disposition":"resolved","conflicted_paths":["x.txt"]}`,
		"finalize", "rebase-continue",
		"--id", "80", "--attempt", "a1", "--input", "-", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.rebase-continue"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestFinalizePublishRegistered proves the subcommand is wired under the finalize
// group carrying the scalar --id/--attempt/--head identity flags and the
// --evidence request-file flag (the canonical evidence bytes ride in the file,
// never a shell-escaped flag), plus --repo-dir.
func TestFinalizePublishRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"finalize", "publish"})
	if err != nil || cmd == nil || cmd.Name() != "publish" {
		t.Fatalf("finalize publish not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "attempt", "head", "evidence", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("finalize publish: missing --%s flag", flag)
		}
	}
}

// TestFinalizePublishAssetIndependent guards the install.go registration: the
// command reads the repository, never installed assets.
func TestFinalizePublishAssetIndependent(t *testing.T) {
	if !assetIndependent["finalize publish"] {
		t.Errorf("%q is not registered asset-independent", "finalize publish")
	}
}

// TestFinalizePublishFlagsRequired proves --id, --attempt, --head, and --evidence
// are required: omitting them is an argument error (exit 2) before any operation.
func TestFinalizePublishFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "finalize", "publish")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
	for _, flag := range []string{"id", "attempt", "head", "evidence"} {
		if !strings.Contains(errS, flag) {
			t.Errorf("required-flag error does not name %q: %q", flag, errS)
		}
	}
}

// TestFinalizePublishReachesOperation proves the command decodes its flags and the
// --evidence bytes and reaches the operation, which emits exactly one protocol-v1
// document naming it. A bare tempdir is no docket repo, so the operation fails past
// its shape check — but only after naming itself.
func TestFinalizePublishReachesOperation(t *testing.T) {
	out, errS, _ := runCLIStdin(t, "canonical evidence bytes",
		"finalize", "publish",
		"--id", "80", "--attempt", "a1",
		"--head", "abcabcabcabcabcabcabcabcabcabcabcabcabca",
		"--evidence", "-", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.publish"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestFinalizeMergeRegistered proves the subcommand is wired under the finalize
// group carrying the scalar --id/--version/--head identity flags, the --admin
// override flag, and --repo-dir.
func TestFinalizeMergeRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"finalize", "merge"})
	if err != nil || cmd == nil || cmd.Name() != "merge" {
		t.Fatalf("finalize merge not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "head", "admin", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("finalize merge: missing --%s flag", flag)
		}
	}
}

// TestFinalizeMergeAssetIndependent guards the install.go registration: the
// command reads the repository, never installed assets.
func TestFinalizeMergeAssetIndependent(t *testing.T) {
	if !assetIndependent["finalize merge"] {
		t.Errorf("%q is not registered asset-independent", "finalize merge")
	}
}

// TestFinalizeMergeFlagsRequired proves --id, --version, and --head are required:
// omitting them is an argument error (exit 2) before any operation runs. --admin
// is optional.
func TestFinalizeMergeFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "finalize", "merge")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
	for _, flag := range []string{"id", "version", "head"} {
		if !strings.Contains(errS, flag) {
			t.Errorf("required-flag error does not name %q: %q", flag, errS)
		}
	}
}

// TestFinalizeMergeReachesOperation proves the command decodes its flags and
// reaches the operation, which emits exactly one protocol-v1 document naming it.
// A bare tempdir is no docket repo, so the operation fails past its shape check —
// but only after naming itself.
func TestFinalizeMergeReachesOperation(t *testing.T) {
	out, errS, _ := runCLI(t, "finalize", "merge",
		"--id", "80", "--version", "1234123412341234123412341234123412341234",
		"--head", "abcabcabcabcabcabcabcabcabcabcabcabcabca",
		"--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.merge"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// --- finalize block / clear-block ------------------------------------------

// TestFinalizeBlockRegistered proves `finalize block` is wired with the scalar
// identity flags plus the --input request-file flag (the authored report and
// remedy ride in the file, never shell-escaped flags).
func TestFinalizeBlockRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"finalize", "block"})
	if err != nil || cmd == nil || cmd.Name() != "block" {
		t.Fatalf("finalize block not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "pr-number", "attempt", "reason", "head", "input", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("finalize block: missing --%s flag", flag)
		}
	}
	if !assetIndependent["finalize block"] {
		t.Errorf("%q is not registered asset-independent", "finalize block")
	}
}

// TestFinalizeBlockReachesOperation proves the command decodes its flags and
// --input body and reaches the operation, which returns exactly one protocol-v1
// document naming it. An empty report fails the up-front shape check, so this
// reaches the operation without a live repository.
func TestFinalizeBlockReachesOperation(t *testing.T) {
	out, errS, _ := runCLIStdin(t, `{"remedy":"fix it"}`,
		"finalize", "block", "--id", "3", "--version", "1234123412341234123412341234123412341234",
		"--pr-number", "7", "--attempt", "att1", "--reason", "gate-repair-required",
		"--head", "aaaa", "--input", "-", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.block"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestFinalizeBlockFlagsRequired proves the scalar identity and --input flags are
// required: omitting them is an argument error (exit 2) before any operation runs.
func TestFinalizeBlockFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "finalize", "block")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
	for _, flag := range []string{"id", "version", "pr-number", "attempt", "reason", "head", "input"} {
		if !strings.Contains(errS, flag) {
			t.Errorf("required-flag error does not name %q: %q", flag, errS)
		}
	}
}

// TestFinalizeClearBlockRegistered proves `finalize clear-block` is wired with the
// scalar reprobe flags and no authored request body.
func TestFinalizeClearBlockRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"finalize", "clear-block"})
	if err != nil || cmd == nil || cmd.Name() != "clear-block" {
		t.Fatalf("finalize clear-block not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "head", "pr-number", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("finalize clear-block: missing --%s flag", flag)
		}
	}
	if !assetIndependent["finalize clear-block"] {
		t.Errorf("%q is not registered asset-independent", "finalize clear-block")
	}
}

// TestFinalizeClearBlockReachesOperation proves the command decodes its flags and
// reaches the operation, which names itself in one protocol-v1 document (a bare
// tempdir is no docket repo, so it fails past its shape check — after naming itself).
func TestFinalizeClearBlockReachesOperation(t *testing.T) {
	out, errS, _ := runCLI(t, "finalize", "clear-block",
		"--id", "3", "--version", "1234123412341234123412341234123412341234",
		"--head", "aaaa", "--pr-number", "7", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.clear-block"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}
