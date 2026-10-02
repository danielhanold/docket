//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationRunStartStorageResetIgnoresRetiredRoots pins ADR-0129 Decision 3
// (change 0471): the run tracker's local stores were renamed by RESET, not
// migrated. A repository still holding the retired roots (an ACTIVE run record
// under rungate/<key>/epoch.json bound to change 5 and this worktree, plus a
// resume lock under rungate-resume/5/epoch.lock) is invisible to the new binary:
// a fresh start mints under run-tracker/<key>/, no lookup finds the retired
// record, and the retired files stay byte-for-byte untouched. This file is
// excluded from change 0471's textual rename passes because its fixtures spell
// the RETIRED layout on purpose.
func TestIntegrationRunStartStorageResetIgnoresRetiredRoots(t *testing.T) {
	repo := newRunTrackerRepo(t)
	common, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	canon, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	writeRetired := func(path string, content []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	retiredRecord, err := json.Marshal(map[string]any{
		"generation": "retired-gen",
		"record": map[string]any{
			"schema_version": runSchemaVersion,
			"gate_key":       "retired-key",
			"change_id":      "5",
			"worktree":       canon,
			"state":          string(RunActive),
			"epoch_id":       "retired-epoch",
			"created_at":     "2026-09-01T00:00:00Z",
			"updated_at":     "2026-09-01T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	retiredRecordPath := filepath.Join(common, "docket", "rungate", "retired-key", "epoch.json")
	retiredLockPath := filepath.Join(common, "docket", "rungate-resume", "5", "epoch.lock")
	writeRetired(retiredRecordPath, retiredRecord)
	writeRetired(retiredLockPath, nil)

	deps := PlanningDeps{Reader: runStartReader(t, runStartCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	res := RunStart(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Started || res.Key == "" {
		t.Fatalf("a fresh start over the retired roots must succeed: Started=%v Key=%q reason=%q", res.Started, res.Key, res.Reason)
	}

	keyDir := filepath.Join(common, "docket", "run-tracker", res.Key)
	runRecord, err := os.ReadFile(filepath.Join(keyDir, "run.json"))
	if err != nil {
		t.Fatalf("the new run record must live at run-tracker/<key>/run.json: %v", err)
	}
	trackerRecord, err := os.ReadFile(filepath.Join(keyDir, "record.json"))
	if err != nil {
		t.Fatalf("the tracker record must live beside it: %v", err)
	}
	for _, c := range []struct {
		doc, want, gone string
	}{
		{string(runRecord), `"state":`, `"epoch_id"`},
		// change 0491: the run tracker's run id is retired; a fresh run.json has none.
		{string(runRecord), `"state":`, `"run_id"`},
		{string(runRecord), `"run_key":`, `"gate_key"`},
		{string(trackerRecord), `"dispatched_at":`, `"dispatch_epoch"`},
	} {
		if !strings.Contains(c.doc, c.want) || strings.Contains(c.doc, c.gone) {
			t.Errorf("record %s: want key %s and no %s", c.doc, c.want, c.gone)
		}
	}

	if _, _, found, err := FindRunByChange(repo, "5"); err != nil || found {
		t.Fatalf("the retired record must be invisible by change: found=%v err=%v", found, err)
	}
	if _, found, err := findRunByWorktree(repo, canon); err != nil || found {
		t.Fatalf("the retired record must be invisible by worktree: found=%v err=%v", found, err)
	}
	if got, err := os.ReadFile(retiredRecordPath); err != nil || !bytes.Equal(got, retiredRecord) {
		t.Fatalf("the retired run record must stay byte-for-byte untouched (err=%v)", err)
	}
	if got, err := os.ReadFile(retiredLockPath); err != nil || len(got) != 0 {
		t.Fatalf("the retired resume lock must stay untouched (err=%v, %d bytes)", err, len(got))
	}
}
