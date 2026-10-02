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
	if r := startedRunResult("", "ctx-token"); r.Started || r.Reason != ReasonRunMintFailed {
		t.Errorf("a start with no key must fail closed run-untracked mint-failed, got %+v", r)
	}
}
