package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// writeRunFixture writes a storedRun envelope for key in state under stateDir's
// run-tracker root and returns the key's directory.
func writeRunFixture(t *testing.T, stateDir, key string, state runState) string {
	t.Helper()
	buf, err := json.Marshal(storedRun{Generation: "g", Record: RunRecord{
		SchemaVersion: runSchemaVersion, RunKey: key, ChangeID: "7", State: state,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return writeRunFile(t, stateDir, key, buf)
}

func writeRunFile(t *testing.T, stateDir, key string, buf []byte) string {
	t.Helper()
	dir := filepath.Join(stateDir, runTrackerDirName, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, runRecordFileName), buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLiveRunsUnderReportsLiveAndUnreadable(t *testing.T) {
	stateDir := testsupport.TempDir(t)
	writeRunFixture(t, stateDir, "k-active", RunActive)
	writeRunFixture(t, stateDir, "k-completing", RunCompleting)
	writeRunFixture(t, stateDir, "k-cancelling", RunCancelling)
	writeRunFixture(t, stateDir, "k-completed", RunCompleted)
	writeRunFixture(t, stateDir, "k-cancelled", RunCancelled)
	writeRunFixture(t, stateDir, "k-superseded", RunSuperseded)
	badDir := writeRunFile(t, stateDir, "k-broken", []byte("{not json"))
	// A key directory without run.json (a run-tracker record only) is no run.
	if err := os.MkdirAll(filepath.Join(stateDir, runTrackerDirName, "k-norun"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := liveRunsUnder(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	cancel := func(k string) string { return "docket run cancel --key " + k + " --reason <why>" }
	want := []liveRunLocator{
		{Key: "k-active", State: string(RunActive), ChangeID: "7", Remedy: cancel("k-active")},
		{Key: "k-broken", State: "unreadable", Remedy: "inspect or remove " + badDir + " by hand"},
		{Key: "k-cancelling", State: string(RunCancelling), ChangeID: "7",
			Remedy: "re-run the same " + cancel("k-cancelling") + " until it reports cancelled"},
		{Key: "k-completing", State: string(RunCompleting), ChangeID: "7", Remedy: cancel("k-completing")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("liveRunsUnder =\n%#v\nwant\n%#v", got, want)
	}
}

// A state this binary does not know is live (liveness unknown), and run cancel
// refuses it, so its remedy is the by-hand one.
func TestLiveRunsUnderUnknownStateIsLive(t *testing.T) {
	stateDir := testsupport.TempDir(t)
	dir := writeRunFixture(t, stateDir, "k-future", runState("paused"))
	got, err := liveRunsUnder(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "k-future" || got[0].State != "paused" ||
		!strings.Contains(got[0].Remedy, dir) {
		t.Fatalf("liveRunsUnder = %#v", got)
	}
}

func TestLiveRunsUnderMissingRoot(t *testing.T) {
	got, err := liveRunsUnder(testsupport.TempDir(t))
	if err != nil || got != nil {
		t.Fatalf("missing root = %#v, %v; want nil, nil", got, err)
	}
}
