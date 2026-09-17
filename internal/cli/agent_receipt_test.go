package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/codexcontract"
)

// The producer and consumer both run through the public CLI. The initial parent
// continuation record is seeded here; the full verdict path belongs to rehearsal.
// Removing facade-envelope support must reject the real, successful claim below.
func TestAgentReceiptFacadeClaimAdvancesExistingExecution(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "build:\n  gate: local\n  test_command: /bin/echo receipt\n")
	root := gateTempDir(t)
	out, _, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build", "--phase", "final")
	if code != 0 {
		t.Fatalf("start: %s", out)
	}
	start := driveDoc(t, decodeOneJSON(t, out))
	id, gen := start["drive_id"].(string), start["generation"].(string)
	out, _, code = runCLI(t, "--json", "gate", "drive", "handoff", "--repo-dir", wt, "--drive-id", id, "--owner-gen", gen)
	if code != 0 {
		t.Fatalf("handoff: %s", out)
	}
	token := driveDoc(t, decodeOneJSON(t, out))["generation"].(string)
	key, err := app.MintGateRecord(wt, app.GateRecord{Target: "docket-implement-next", Retry: app.RetryUnused, AttemptLimit: 2, AttributedID: 432, ContinuationID: "fixture-continuation", ContinuationDrive: id, ContinuationHandoff: token})
	if err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runCLI(t, "--json", "run", "gate-claim", key, "fixture-continuation", "--repo-dir", wt)
	claim := decodeOneJSON(t, out)
	if code != 0 || claim["decision"] != "gate-claimed" || claim["generation"] == token {
		t.Fatalf("producer did not redeem: %s", out)
	}
	checked := checkCapturedReceipt(t, "run.gate-claim", out, stderr, code, root)
	if checked["authority"] != "owner" || checked["next_operation"] != "gate.drive.advance" || checked["drive_id"] != id {
		t.Fatalf("redeemed receipt must advance same drive: %v", checked)
	}
	fresh := checked["generation"].(string)
	out, _, code = runCLI(t, "--json", "gate", "drive", "advance", "--repo-dir", wt, "--drive-id", id, "--owner-gen", fresh)
	advanced := driveDoc(t, decodeOneJSON(t, out))
	if code != 0 || advanced["outcome"] != "PASSED" || advanced["raw_run_dir"] != start["raw_run_dir"] {
		t.Fatalf("claim relaunched/lost terminal work: %s", out)
	}
	// Backend, not the checker, enforces single use. An exit-zero refusal is a halt.
	out, stderr, code = runCLI(t, "--json", "run", "gate-claim", key, "fixture-continuation", "--repo-dir", wt)
	refused := checkCapturedReceipt(t, "run.gate-claim", out, stderr, code, root)
	if refused["classification"] != "halt" || refused["next_operation"] != nil {
		t.Fatalf("refusal allowed continuation: %v", refused)
	}
}

func checkCapturedReceipt(t *testing.T, operation, stdout, stderr string, exitCode int, runRoot string) map[string]any {
	t.Helper()
	root := gateTempDir(t)
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 432, Role: "docket-build-standard", Phase: "build", Mode: "fresh", TaskID: "fixture", Primary: filepath.Join(root, "primary"), Feature: filepath.Join(root, "feature"), CommonDir: filepath.Join(root, "common"), Branch: "fix/fixture", EntryHEAD: strings.Repeat("a", 40), MetadataRevision: strings.Repeat("b", 40), ChangePath: "docs/changes/active/0432.md", DocketExecutable: filepath.Join(root, "docket"), DocketCommit: strings.Repeat("c", 40), ReadRoots: []string{root}, RunRoot: runRoot}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	assignment, outPath, errPath := filepath.Join(root, "assignment.json"), filepath.Join(root, "stdout"), filepath.Join(root, "stderr")
	for p, body := range map[string][]byte{assignment: b, outPath: []byte(stdout), errPath: []byte(stderr)} {
		if err := os.WriteFile(p, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	out, errS, code := runCLI(t, "--json", "agent", "check-receipt", "--assignment", assignment, "--sha256", hex.EncodeToString(sum[:]), "--operation", operation, "--stdout", outPath, "--stderr", errPath, "--exit-code", strconv.Itoa(exitCode))
	if code != 0 {
		t.Fatalf("real %s receipt rejected by checked consumer: %s %s", operation, out, errS)
	}
	receipt, ok := decodeOneJSON(t, out)["receipt"].(map[string]any)
	if !ok {
		t.Fatalf("missing checked receipt: %s", out)
	}
	return receipt
}

func TestAgentReceiptCoordinatorContextChecksIdentity(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "build:\n  gate: local\n  test_command: /bin/echo context\n")
	root := gateTempDir(t)
	out, stderr, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
	if code != 0 {
		t.Fatalf("producer: %s", out)
	}
	id := driveDoc(t, decodeOneJSON(t, out))["drive_id"].(string)
	capture := filepath.Join(gateTempDir(t), "stdout")
	if err := os.WriteFile(capture, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("producer diagnostics: %s", stderr)
	}
	for _, tc := range []struct {
		name, runRoot, expected string
		valid                   bool
	}{
		{"correct", root, id, true}, {"wrong-drive", root, "different", false}, {"wrong-root", filepath.Dir(root), id, false}, {"missing-root", "", id, false}, {"relative-root", "relative", id, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reply, _, rc := runCLI(t, "--json", "agent", "check-receipt", "--operation", "gate.drive.start", "--stdout", capture, "--exit-code", "0", "--run-root", tc.runRoot, "--expected-drive-id", tc.expected)
			if (rc == 0) != tc.valid {
				t.Fatalf("valid=%v checker=%s", tc.valid, reply)
			}
		})
	}
}

func TestAgentReceiptRealFailureRemainsFailure(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "build:\n  gate: local\n  test_command: 'exit 7'\n")
	root := gateTempDir(t)
	out, stderr, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root, "--owner", "build")
	r := checkCapturedReceipt(t, "gate.drive.start", out, stderr, code, root)
	if code == 0 || r["classification"] != "FAILED" || r["next_operation"] != nil {
		t.Fatal("failed execution became continuation or success")
	}
}
