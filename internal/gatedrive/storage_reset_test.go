package gatedrive

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// The run tracker's gatedrive stores were renamed by RESET, not migrated
// (ADR-0129 row 38 and Decision 3, change 0471): each store whose persisted names
// changed moved to a new root, so the binary never reads retired state. This file
// is excluded from change 0471's textual rename passes because its fixtures spell
// the RETIRED layout on purpose.

func TestRunTrackerResetStoreRoots(t *testing.T) {
	common := testsupport.TempDir(t)
	s := OpenStore(common)
	for name, c := range map[string]struct{ got, want string }{
		"drives":    {s.root, filepath.Join(common, "docket", "gate-drives", "v2")},
		"scopes":    {s.scopeRoot, filepath.Join(common, "docket", "gate-scopes", "v2")},
		"admission": {s.admissionRoot, filepath.Join(common, "docket", "gate-admission", "v2")},
		// The suite-budget store carries no retired key, so it keeps v1.
		"budgets": {s.suiteBudgetRoot, filepath.Join(common, "docket", "gate-suite-budgets", "v1")},
	} {
		if c.got != c.want {
			t.Errorf("%s root = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestRunTrackerResetIgnoresRetiredAdmissionSlot(t *testing.T) {
	common := testsupport.TempDir(t)
	s := OpenStore(common)
	wt := mkWorktree(t)
	canonical, err := filepath.EvalSymlinks(wt)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	// A busy v1 slot, exactly as the pre-0471 binary wrote it: untagged Go field
	// names, owned by a run the new store has never heard of.
	retiredPath := filepath.Join(common, "docket", "gate-admission", "v1", admissionKey(canonical), recordFileName)
	retired := []byte(`{"Generation":"retired-gen","Record":{"SchemaVersion":1,"RepoIdentity":"repo-1","WorktreeRoot":` +
		strconv.Quote(canonical) + `,"State":"executing","ExecutionGen":3,"ReservationToken":"retired-token","RunEpochID":"retired-run","Kind":"scopeless"}}`)
	if err := os.MkdirAll(filepath.Dir(retiredPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(retiredPath, retired, 0o600); err != nil {
		t.Fatal(err)
	}

	token, err := s.ReserveWorktreeExecutionForEpoch("repo-1", wt, "new-run", nil)
	if err != nil || token == "" {
		t.Fatalf("a reservation over a retired v1 slot must be admitted: token=%q err=%v", token, err)
	}
	slot, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if slot.RunEpochID != "new-run" || slot.State != admissionReserved {
		t.Fatalf("slot = (%q, %q), want (new-run, reserved)", slot.RunEpochID, slot.State)
	}
	after, err := os.ReadFile(retiredPath)
	if err != nil || !bytes.Equal(after, retired) {
		t.Fatalf("the retired v1 slot must stay byte-for-byte untouched (err=%v)", err)
	}
}

// TestAdmissionRecordFieldsCarryExplicitJSONTags: the admission record once had
// no tags, so a Go rename silently changed its on-disk key. Every field now names
// its key explicitly.
func TestAdmissionRecordFieldsCarryExplicitJSONTags(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(admissionRecord{}), reflect.TypeOf(storedAdmission{})} {
		seen := map[string]string{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag, ok := f.Tag.Lookup("json")
			name := strings.Split(tag, ",")[0]
			if !ok || name == "" || name == "-" {
				t.Errorf("%s.%s has no explicit json key (tag %q)", typ.Name(), f.Name, tag)
				continue
			}
			if prev, dup := seen[name]; dup {
				t.Errorf("%s: json key %q used by both %s and %s", typ.Name(), name, prev, f.Name)
			}
			seen[name] = f.Name
		}
	}
}

func TestAdmissionRecordPersistsRunIDKey(t *testing.T) {
	b, err := json.Marshal(storedAdmission{Generation: "g", Record: admissionRecord{RunEpochID: "run-1", State: admissionReserved}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"generation":"g"`, `"record":{`, `"run_id":"run-1"`, `"state":"reserved"`} {
		if !strings.Contains(s, want) {
			t.Errorf("persisted slot %s lacks %s", s, want)
		}
	}
	for _, gone := range []string{`"RunEpochID"`, `"Generation"`, `"Record"`} {
		if strings.Contains(s, gone) {
			t.Errorf("persisted slot %s still carries the implicit key %s", s, gone)
		}
	}
}
