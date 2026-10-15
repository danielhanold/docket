package app

import "testing"

// TestCheckoutRefusalDisposition: every result a checkout refusal can carry
// lands in the closed disposition set, so a caller routing on disposition
// never meets an empty one: an unobservable outcome is unknown, a lost race
// contended, and every other refusal blocked.
func TestCheckoutRefusalDisposition(t *testing.T) {
	for result, want := range map[Result]string{
		ResultExternalFailed:    "unknown",
		ResultInternalError:     "unknown",
		ResultInterrupted:       "unknown",
		ResultContended:         "contended",
		ResultInvalidInput:      "blocked",
		ResultInvalidState:      "blocked",
		ResultBlocked:           "blocked",
		ResultUnsupportedConfig: "blocked",
	} {
		r := withCheckoutDisposition(newWorkspaceResult(OperationWorkspaceCheckout, result, WorkspaceOpResult{ID: 7}))
		if r.Disposition != want {
			t.Errorf("%s refusal disposition = %q; want %q", result, r.Disposition, want)
		}
	}
	kept := withCheckoutDisposition(newWorkspaceResult(OperationWorkspaceCheckout, ResultBlocked, WorkspaceOpResult{ID: 7, Disposition: "contended"}))
	if kept.Disposition != "contended" {
		t.Errorf("a set disposition was overwritten: %q", kept.Disposition)
	}
}
