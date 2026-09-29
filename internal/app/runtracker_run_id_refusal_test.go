package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestClassifyRunIDError (change 0463): every run-epoch registry failure maps to
// a stable protocol result and a fixed reason token. A not-found epoch is the named
// unknown-run-id. A presented value wrapped into the error chain never leaks into
// the reason.
func TestClassifyRunIDError(t *testing.T) {
	const presented = "0790b760e26444866ef2e156ba383326"
	cases := []struct {
		kind   RunErrorKind
		res    Result
		reason string
	}{
		{ErrRunNotFound, ResultInvalidInput, "unknown-run-id"},
		{ErrRunIDMismatch, ResultInvalidInput, "stale-run-id"},
		{ErrRunAmbiguous, ResultInvalidInput, "run-ambiguous"},
		{ErrRunNotActive, ResultInvalidInput, "run-not-active"},
		{ErrRunOwnerAmbiguous, ResultInvalidInput, "run-owner-ambiguous"},
		{ErrRunOwnerUnresolved, ResultInvalidInput, "run-owner-unresolved"},
		{ErrRunRecordCorrupt, ResultInternalError, "run-record-corrupt"},
		{ErrRunRecordIO, ResultInternalError, "run-record-io"},
	}
	for _, tc := range cases {
		err := fmt.Errorf("start refused for %s: %w", presented, runErr(tc.kind, "find-dir-by-id", errors.New(presented)))
		res, reason, ok := ClassifyRunIDError(err)
		if !ok || res != tc.res || reason != tc.reason {
			t.Errorf("%s: got (%s, %q, %v), want (%s, %q, true)", tc.kind, res, reason, ok, tc.res, tc.reason)
		}
		if strings.Contains(reason, presented) {
			t.Errorf("%s: reason leaked the presented value: %q", tc.kind, reason)
		}
	}
	if _, _, ok := ClassifyRunIDError(errors.New("plain failure")); ok {
		t.Error("a non-epoch error must not classify")
	}
	if _, _, ok := ClassifyRunIDError(ErrStaleRunID); ok {
		t.Error("a mutation-fence error is not an epoch-registry error")
	}
}

// TestRunIDNextAction (change 0463): unknown-run-id tells the caller which
// run-started field goes where. Its text is distinct from the stale-linkage remedy.
// Other reasons carry no invented message.
func TestRunIDNextAction(t *testing.T) {
	unknown := RunIDNextAction(ReasonUnknownRunID)
	for _, want := range []string{"--run-id", "--gate-context", "run-started <key> <epoch> <dispatch-context>"} {
		if !strings.Contains(unknown, want) {
			t.Errorf("unknown-run-id message must mention %q, got %q", want, unknown)
		}
	}
	stale := RunIDNextAction("stale-run-id")
	if stale == "" || stale == unknown {
		t.Errorf("stale-run-id needs its own message, got %q", stale)
	}
	if got := RunIDNextAction("run-record-io"); got != "" {
		t.Errorf("an unmapped reason must yield no message, got %q", got)
	}
}
