package app

import (
	"fmt"
	"strings"
	"testing"
)

// TestClassifyRunRecordError (change 0491; was change 0463's run-id classifier test):
// every run registry failure maps to a stable protocol result and its own kind as
// the reason token. A corrupt or unreadable record is an internal error. A
// presented value wrapped into the chain never leaks into the reason.
func TestClassifyRunRecordError(t *testing.T) {
	const presented = "0790b760e26444866ef2e156ba383326"
	for _, tc := range []struct {
		kind RunErrorKind
		want Result
	}{
		{ErrRunNotFound, ResultInvalidInput},
		{ErrRunRecordConflict, ResultInvalidInput},
		{ErrRunNotActive, ResultInvalidInput},
		{ErrRunRecordCorrupt, ResultInternalError},
		{ErrRunRecordIO, ResultInternalError},
	} {
		err := fmt.Errorf("refused %s: %w", presented, runErr(tc.kind, "op", nil))
		res, reason, ok := ClassifyRunRecordError(err)
		if !ok || res != tc.want || reason != string(tc.kind) {
			t.Errorf("%s: got (%s, %q, %v), want (%s, %q, true)", tc.kind, res, reason, ok, tc.want, tc.kind)
		}
		if strings.Contains(reason, presented) {
			t.Errorf("%s: reason leaked the presented value", tc.kind)
		}
	}
	if _, _, ok := ClassifyRunRecordError(fmt.Errorf("plain")); ok {
		t.Error("an error with no *RunError must fall through (ok=false)")
	}
}

// TestRunRecordNextAction (change 0491): run-not-found tells the caller to pass the
// run key run.start printed, never the run context; it names no retired run id.
// Other reasons carry no invented message.
func TestRunRecordNextAction(t *testing.T) {
	msg := RunRecordNextAction(string(ErrRunNotFound))
	if !strings.Contains(msg, "--run-key") || !strings.Contains(msg, "run context") {
		t.Fatalf("run-not-found next action must name --run-key and warn off the run context, got %q", msg)
	}
	if strings.Contains(msg, "--run-id") || strings.Contains(msg, "<run-id>") {
		t.Fatalf("the next action names the retired run id: %q", msg)
	}
	if got := RunRecordNextAction(string(ErrRunRecordConflict)); got != "" {
		t.Errorf("run-record-conflict carries no invented message, got %q", got)
	}
}
