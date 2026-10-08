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

// writeTrackerRecordFixture writes a schema-current record.json into key's
// directory under stateDir (no git: Repo is left empty, which only the
// repository-identity check reads).
func writeTrackerRecordFixture(t *testing.T, stateDir, key string, rec RunTrackerRecord) {
	t.Helper()
	rec.Schema = runTrackerSchemaVersion
	buf, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	writeKeyFile(t, stateDir, key, runTrackerRecordFileName, buf)
}

// writeBindingFixture writes claim-binding.json into key's directory.
func writeBindingFixture(t *testing.T, stateDir, key string, b RunTrackerClaimBinding) {
	t.Helper()
	b.Schema = bindingSchemaVersion
	buf, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	writeKeyFile(t, stateDir, key, runTrackerClaimBindingName, buf)
}

func writeKeyFile(t *testing.T, stateDir, key, name string, buf []byte) {
	t.Helper()
	dir := filepath.Join(stateDir, runTrackerDirName, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf, 0o600); err != nil {
		t.Fatal(err)
	}
}

// cancellableRun gives key the authority run cancel accepts: a parent
// capability and a confirmed binding for change 7 (writeRunFixture's change).
func cancellableRun(t *testing.T, stateDir, key string) {
	t.Helper()
	writeTrackerRecordFixture(t, stateDir, key, RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeBindingFixture(t, stateDir, key, RunTrackerClaimBinding{ChangeID: 7, RequestID: "r", Confirmed: true})
}

func TestLiveRunsUnderReportsLiveAndUnreadable(t *testing.T) {
	stateDir := testsupport.TempDir(t)
	writeRunFixture(t, stateDir, "k-active", RunActive)
	writeRunFixture(t, stateDir, "k-completing", RunCompleting)
	writeRunFixture(t, stateDir, "k-cancelling", RunCancelling)
	writeRunFixture(t, stateDir, "k-completed", RunCompleted)
	writeRunFixture(t, stateDir, "k-cancelled", RunCancelled)
	writeRunFixture(t, stateDir, "k-superseded", RunSuperseded)
	cancellableRun(t, stateDir, "k-active")
	cancellableRun(t, stateDir, "k-completing")
	cancellableRun(t, stateDir, "k-cancelling")
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

// TestLiveRunsUnderRemedyFollowsCancelAuthority: each live run's remedy is a
// command that will act on it in its current state (change 0540). Cancel is named
// only when runCancelOwner accepts; a readable run cancel would refuse gets the
// keyed verdict, which retires it; anything neither command can load is by hand.
// The verdict remedy is qualified: the scan cannot tell a finished unclaimed run
// from a live dispatch that has not claimed yet, and a verdict on the latter ends it.
func TestLiveRunsUnderRemedyFollowsCancelAuthority(t *testing.T) {
	stateDir := testsupport.TempDir(t)
	cancel := func(k string) string { return "docket run cancel --key " + k + " --reason <why>" }
	verdict := func(k string) string {
		return "once its dispatch has returned, run `docket run verdict " + k + "`"
	}
	byHand := func(dir string) string { return "inspect or remove " + dir + " by hand" }

	// The incident: a record with no parent capability and no claim (no binding file).
	writeRunFixture(t, stateDir, "a-noclaim", RunActive)
	writeTrackerRecordFixture(t, stateDir, "a-noclaim", RunTrackerRecord{ChildContextHash: "h", AttemptLimit: 2, Terminal: true})

	// A capability but only an unconfirmed reservation.
	writeRunFixture(t, stateDir, "b-reserved", RunActive)
	writeTrackerRecordFixture(t, stateDir, "b-reserved", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeBindingFixture(t, stateDir, "b-reserved", RunTrackerClaimBinding{ChangeID: 7, RequestID: "r"})

	// A confirmed binding but no parent capability: cancel refuses, the verdict does not need it.
	writeRunFixture(t, stateDir, "c-nocap", RunCompleting)
	writeTrackerRecordFixture(t, stateDir, "c-nocap", RunTrackerRecord{AttemptLimit: 2})
	writeBindingFixture(t, stateDir, "c-nocap", RunTrackerClaimBinding{ChangeID: 7, RequestID: "r", Confirmed: true})

	// Cancellable: confirmed binding and capability.
	writeRunFixture(t, stateDir, "d-owned", RunActive)
	cancellableRun(t, stateDir, "d-owned")

	// No record.json at all: neither cancel nor verdict can load the run.
	eDir := writeRunFixture(t, stateDir, "e-norecord", RunActive)

	// Unreadable record.json.
	fDir := writeRunFixture(t, stateDir, "f-badrecord", RunActive)
	writeKeyFile(t, stateDir, "f-badrecord", runTrackerRecordFileName, []byte("{not json"))

	// Corrupt binding: cancel refuses claim-unreadable, verdict stops binding-unreadable.
	gDir := writeRunFixture(t, stateDir, "g-badbinding", RunActive)
	writeTrackerRecordFixture(t, stateDir, "g-badbinding", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeKeyFile(t, stateDir, "g-badbinding", runTrackerClaimBindingName, []byte("{not json"))

	// Cancelling with no claim (the death guardian fenced it): no command settles it.
	hDir := writeRunFixture(t, stateDir, "h-cancelling-noclaim", RunCancelling)
	writeTrackerRecordFixture(t, stateDir, "h-cancelling-noclaim", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})

	// Cancelling and cancellable: re-run the cancel.
	writeRunFixture(t, stateDir, "i-cancelling-owned", RunCancelling)
	cancellableRun(t, stateDir, "i-cancelling-owned")

	// Owner differs from the run's change: cancel refuses claim-mismatch; the verdict still acts.
	writeRunFixture(t, stateDir, "j-mismatch", RunActive)
	writeTrackerRecordFixture(t, stateDir, "j-mismatch", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeBindingFixture(t, stateDir, "j-mismatch", RunTrackerClaimBinding{ChangeID: 8, RequestID: "r", Confirmed: true})

	got, err := liveRunsUnder(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	remedies := map[string]string{}
	for _, l := range got {
		remedies[l.Key] = l.Remedy
	}
	want := map[string]string{
		"a-noclaim":            verdict("a-noclaim"),
		"b-reserved":           verdict("b-reserved"),
		"c-nocap":              verdict("c-nocap"),
		"d-owned":              cancel("d-owned"),
		"e-norecord":           byHand(eDir),
		"f-badrecord":          byHand(fDir),
		"g-badbinding":         byHand(gDir),
		"h-cancelling-noclaim": byHand(hDir),
		"i-cancelling-owned":   "re-run the same " + cancel("i-cancelling-owned") + " until it reports cancelled",
		"j-mismatch":           verdict("j-mismatch"),
	}
	if !reflect.DeepEqual(remedies, want) {
		t.Fatalf("remedies =\n%#v\nwant\n%#v", remedies, want)
	}
}
