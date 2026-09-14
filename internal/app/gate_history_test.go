package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// legacyFixtureID maps a Task 2 legacy-v2 fixture name to the validated
// (lowercase-hex, 30-char) drive id it is seeded under. The ids carry neither an
// 'a' run (the fixtures' 32-char generation token) nor a '1' run (the fixtures'
// 40-char head_oid), so the redaction assertion cannot false-positive on the id.
var legacyFixtureID = map[string]string{
	"passed":           "042800000000000000000000000001",
	"failed":           "042800000000000000000000000002",
	"halted":           "042800000000000000000000000003",
	"waiting":          "042800000000000000000000000004",
	"schema1":          "042800000000000000000000000005",
	"missing-worktree": "042800000000000000000000000006",
	"corrupt":          "042800000000000000000000000007",
}

// seedLegacyFixtures installs each named Task 2 fixture into a fresh temp
// gitCommonDir at the exact store path gatedrive.OpenStore roots
// (<gitCommonDir>/docket/gate-drives/v1/<id>/record.json) and returns the
// gitCommonDir. It reads the frozen fixtures the gatedrive package owns, so the
// app-layer mapping is exercised against the same records the driver reads.
func seedLegacyFixtures(t *testing.T, names ...string) string {
	t.Helper()
	gitCommonDir := testsupport.TempDir(t)
	root := filepath.Join(gitCommonDir, "docket", "gate-drives", "v1")
	for _, name := range names {
		id, ok := legacyFixtureID[name]
		if !ok {
			t.Fatalf("no id mapping for fixture %q", name)
		}
		buf, err := os.ReadFile(filepath.Join("..", "gatedrive", "testdata", "legacy-v2", name+".json"))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		dir := filepath.Join(root, id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "record.json"), buf, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return gitCommonDir
}

// exePathFor returns an absolute (need-not-exist) executable path — process
// classification of the fixtures' absent run dirs never re-execs it.
func exePathFor(t *testing.T) string {
	t.Helper()
	return filepath.Join(testsupport.TempDir(t), "docket")
}

func TestGateHistoryCleanupAppliedRunMirrorsOutcome(t *testing.T) {
	// passed+failed classify nonblocking (terminal); waiting is nonterminal, halted's
	// run dir is absent (foreign), schema1 is an unknown schema — all three retained.
	gitCommonDir := seedLegacyFixtures(t, "passed", "failed", "waiting", "halted", "schema1")

	res := GateHistoryCleanup(gitCommonDir, exePathFor(t), GateHistoryCleanupRequest{RepoDir: "/repo"})

	if res.Result != ResultApplied {
		t.Fatalf("Result = %q, want %q (reason %q)", res.Result, ResultApplied, res.Reason)
	}
	if res.Operation != OperationGateHistoryCleanup {
		t.Errorf("Operation = %q, want %q", res.Operation, OperationGateHistoryCleanup)
	}
	if res.Checked != 5 || res.Recovered != 0 || res.Recoverable != 0 || res.Retained != 3 {
		t.Errorf("counts = checked %d recovered %d recoverable %d retained %d; want 5/0/0/3",
			res.Checked, res.Recovered, res.Recoverable, res.Retained)
	}
	if len(res.Findings) != 5 {
		t.Fatalf("findings = %d, want 5", len(res.Findings))
	}
	// The findings mirror the outcome verbatim (id/class/reason), keyed by class.
	byClass := map[string]int{}
	for _, f := range res.Findings {
		byClass[f.Class]++
		if f.Reason == "" {
			t.Errorf("finding %q carries an empty reason", f.DriveID)
		}
	}
	if byClass["nonblocking"] != 2 || byClass["retained"] != 3 {
		t.Errorf("class tally = %v, want 2 nonblocking / 3 retained", byClass)
	}

	human := res.HumanText()
	wantHeader := "history cleanup — checked 5: 2 nonblocking, 0 recovered, 0 recoverable, 3 retained"
	if !strings.HasPrefix(human, wantHeader) {
		t.Errorf("HumanText header = %q, want prefix %q", human, wantHeader)
	}
	// One indented line per finding, each naming id, class, reason.
	for _, f := range res.Findings {
		line := "\n  " + f.DriveID + "  " + f.Class + "  " + f.Reason
		if !strings.Contains(human, line) {
			t.Errorf("HumanText missing finding line %q in:\n%s", line, human)
		}
	}
	if !strings.HasSuffix(human, "retained records remain; recovery is not complete") {
		t.Errorf("HumanText must end with the retained-remain line, got:\n%s", human)
	}
	// Never a repository-readiness claim.
	for _, banned := range []string{"ready", "complete recovery", "recovery is complete"} {
		if strings.Contains(human, banned) {
			t.Errorf("HumanText must not claim readiness (%q):\n%s", banned, human)
		}
	}
}

func TestGateHistoryCleanupInvalidDriveID(t *testing.T) {
	gitCommonDir := seedLegacyFixtures(t, "passed")

	res := GateHistoryCleanup(gitCommonDir, exePathFor(t), GateHistoryCleanupRequest{
		RepoDir: "/repo",
		DriveID: "../etc/passwd",
	})

	if res.Result != ResultInvalidInput {
		t.Fatalf("Result = %q, want %q", res.Result, ResultInvalidInput)
	}
	if res.Reason != "invalid-id" {
		t.Errorf("Reason = %q, want %q", res.Reason, "invalid-id")
	}
	if len(res.Findings) != 0 {
		t.Errorf("a refusal carries no findings, got %d", len(res.Findings))
	}
}

func TestGateHistoryCleanupDryRunEchoesIntoDocument(t *testing.T) {
	gitCommonDir := seedLegacyFixtures(t, "passed", "waiting")

	dry := GateHistoryCleanup(gitCommonDir, exePathFor(t), GateHistoryCleanupRequest{RepoDir: "/repo", DryRun: true})
	if dry.Result != ResultApplied {
		t.Fatalf("Result = %q, want %q", dry.Result, ResultApplied)
	}
	if !dry.DryRun {
		t.Errorf("DryRun did not echo into the document")
	}

	wet := GateHistoryCleanup(gitCommonDir, exePathFor(t), GateHistoryCleanupRequest{RepoDir: "/repo", DryRun: false})
	if wet.DryRun {
		t.Errorf("DryRun=false must echo false into the document")
	}
}

func TestGateHistoryCleanupRedactionBound(t *testing.T) {
	// Run over EVERY fixture, then prove the marshaled document leaks none of the
	// sensitive fields the fixtures carry: no command, no env, no head oid, no
	// generation token — findings hold ONLY id/class/reason.
	gitCommonDir := seedLegacyFixtures(t,
		"passed", "failed", "halted", "waiting", "schema1", "missing-worktree", "corrupt")

	res := GateHistoryCleanup(gitCommonDir, exePathFor(t), GateHistoryCleanupRequest{RepoDir: "/repo"})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q, want %q (reason %q)", res.Result, ResultApplied, res.Reason)
	}

	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	doc := string(buf)

	for _, banned := range []string{
		"command", // the fixtures' command argv key
		"env",     // env_hash / any env leakage
		"1111111111111111111111111111111111111111", // fixture head_oid
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",         // fixture generation token
		"/bin/sh",                                  // command argv content
		"deadbeef",                                 // env_hash content
	} {
		if strings.Contains(doc, banned) {
			t.Errorf("result document leaks %q:\n%s", banned, doc)
		}
	}
}
