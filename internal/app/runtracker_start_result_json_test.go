package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRunStartResultRunContextKey (change 0477, ADR-0129 row 38g): the run.start
// result names the run-context token under the key run_context, the same word
// the run-started text line and every flag use. The retired dispatch_context key
// is gone, with no alias.
func TestRunStartResultRunContextKey(t *testing.T) {
	buf, err := json.Marshal(RunStartResult{Started: true, Key: "k", RunContext: "ctx-token"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(buf), `"run_context":"ctx-token"`) {
		t.Errorf("run.start JSON lacks \"run_context\": %s", buf)
	}
	if strings.Contains(string(buf), "dispatch_context") {
		t.Errorf("run.start JSON still carries the retired dispatch_context key: %s", buf)
	}
}

// TestRunStartResultCarriesNoRunID (change 0491): the run id is retired; the
// run.start result names only the key and the run context, and its started line is
// two tokens.
func TestRunStartResultCarriesNoRunID(t *testing.T) {
	res := startedRunResult("k0491", "ctx-token")
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(buf), `"run_id"`) {
		t.Errorf("run.start JSON still carries the retired run_id key: %s", buf)
	}
	if got := strings.SplitN(res.HumanText(), "\n", 2)[0]; got != "run-started k0491 ctx-token" {
		t.Errorf("started line = %q, want %q", got, "run-started k0491 ctx-token")
	}
}

// runStartedWant is the exact text report of a started run: the two-token
// `run-started <key> <run-context>` line, then the stop note naming the run's own
// key (change 0501). It spells the note out literally rather than calling
// runStartStopNote, so a wrong note fails every assert that uses it.
func runStartedWant(key, runContext string) string {
	return "run-started " + key + " " + runContext + "\n" +
		"note: closing this session may not stop this run. To stop it: docket run cancel --key " +
		key + " --reason <why>"
}

// TestRunStartStopNoteNamesTheRunsOwnKey (change 0501): a started report's second
// line is the stop note, and its cancel command carries this run's own key, so a
// human can run it as printed. The report stays exactly two lines.
func TestRunStartStopNoteNamesTheRunsOwnKey(t *testing.T) {
	res := startedRunResult("k0501-own", "ctx-token")
	got := res.HumanText()
	if want := runStartedWant("k0501-own", "ctx-token"); got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
	if !strings.Contains(got, "docket run cancel --key k0501-own --reason <why>") {
		t.Errorf("stop note does not name the run's own key: %q", got)
	}
	if n := strings.Count(got, "\n"); n != 1 {
		t.Errorf("started report has %d newlines, want exactly 1 (two lines): %q", n, got)
	}
}

// TestRunStartTextOmitsBareTokenJSONKeepsIt (change 0501): the bare
// owner-lifecycle-unavailable token no longer appears in the text report, while
// the JSON owner_lifecycle field still carries it for machine readers. The JSON
// half is also the absence assert's non-vacuity companion: the same result still
// holds the token.
func TestRunStartTextOmitsBareTokenJSONKeepsIt(t *testing.T) {
	if ReasonOwnerLifecycleUnavailable != "owner-lifecycle-unavailable" {
		t.Fatalf("ReasonOwnerLifecycleUnavailable = %q, must stay %q", ReasonOwnerLifecycleUnavailable, "owner-lifecycle-unavailable")
	}
	res := startedRunResult("k0501", "ctx-token")
	if strings.Contains(res.HumanText(), ReasonOwnerLifecycleUnavailable) {
		t.Errorf("text report still prints the bare %q token: %q", ReasonOwnerLifecycleUnavailable, res.HumanText())
	}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(buf), `"owner_lifecycle":"owner-lifecycle-unavailable"`) {
		t.Errorf("run.start JSON lost owner_lifecycle=owner-lifecycle-unavailable: %s", buf)
	}
}

// TestRunStartStopNoteGatedOnOwnerLifecycle (change 0501): the note is printed only
// when OwnerLifecycle is set, so text and JSON cannot drift apart. A run-untracked
// report never carries it.
func TestRunStartStopNoteGatedOnOwnerLifecycle(t *testing.T) {
	res := startedRunResult("k", "ctx")
	res.OwnerLifecycle = ""
	if got := res.HumanText(); got != "run-started k ctx" {
		t.Errorf("HumanText without OwnerLifecycle = %q, want %q", got, "run-started k ctx")
	}
	if got := runUntracked(ReasonRunMintFailed).HumanText(); strings.Contains(got, "note:") {
		t.Errorf("run-untracked report carries the stop note: %q", got)
	}
}
