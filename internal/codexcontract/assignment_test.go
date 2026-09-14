package codexcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAssignmentRejectsChangedDuplicateAndUnknownInputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "assignment.json")
	valid := `{"schema_version":1,"change_id":425,"role":"docket-build-standard","phase":"build","task_id":"task-1","mode":"fresh","primary":"/tmp/primary","feature":"/tmp/feature","common_dir":"/tmp/common","branch":"codex/change","entry_head":"0123456789012345678901234567890123456789","metadata_revision":"1123456789012345678901234567890123456789","change_path":"docs/changes/active/0425.md","docket_executable":"/tmp/docket","docket_commit":"2123456789012345678901234567890123456789","resources":[],"read_roots":["/tmp/control"],"write_paths":["internal/x"],"inherited_paths":[]}`
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(valid))
	if _, err := ReadAssignment(path, hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("valid assignment: %v", err)
	}

	for name, body := range map[string]string{
		"duplicate": strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1),
		"unknown":   strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"surprise":true`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(body))
			if _, err := ReadAssignment(path, hex.EncodeToString(digest[:])); err == nil {
				t.Fatal("accepted malformed assignment")
			}
		})
	}
	if err := os.WriteFile(path, []byte(valid+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAssignment(path, hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("accepted bytes changed after digest was pinned")
	}
}

func TestValidateAssignmentRejectsUnsafeRolePathsAndSecrets(t *testing.T) {
	base := Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-build-standard", Phase: "build", TaskID: "task-1", Mode: "fresh", Primary: "/tmp/primary", Feature: "/tmp/feature", CommonDir: "/tmp/common", Branch: "codex/change", EntryHEAD: strings.Repeat("0", 40), MetadataRevision: strings.Repeat("1", 40), ChangePath: "docs/changes/active/0425.md", DocketExecutable: "/tmp/docket", DocketCommit: strings.Repeat("2", 40), ReadRoots: []string{"/tmp/control"}, WritePaths: []string{"internal/x"}}
	if err := ValidateAssignment(base); err != nil {
		t.Fatalf("valid assignment: %v", err)
	}
	mutations := map[string]func(*Assignment){
		"relative feature":    func(a *Assignment) { a.Feature = "feature" },
		"escaping output":     func(a *Assignment) { a.ArtifactPath = "../primary/plan.md" },
		"worker without task": func(a *Assignment) { a.TaskID = "" },
		"review without pins": func(a *Assignment) { a.Mode = "review"; a.Role = "docket-review-standard"; a.TaskID = "" },
		"secret field in resource": func(a *Assignment) {
			a.Resources = []Resource{{LogicalID: "gate_context", Path: "/tmp/control/key", SHA256: strings.Repeat("a", 64), Source: "package:docket"}}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			a := base
			mutate(&a)
			if ValidateAssignment(a) == nil {
				t.Fatal("accepted invalid assignment")
			}
		})
	}
}
