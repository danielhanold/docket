package codexentry

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// terminalCall records one RecordTerminal invocation.
type terminalCall struct{ handle, turn, status string }

// recordingTerminal is a fake TerminalRecorder. When tr is set it asserts the
// transport was already Close()d at record time, so a test can prove the record
// happens AFTER transport teardown (Task 9, change 0441).
type recordingTerminal struct {
	tr             *scriptedTransport
	err            error
	calls          []terminalCall
	closedAtRecord bool
}

func (r *recordingTerminal) RecordTerminal(handle, turnID, status string) error {
	if r.tr != nil && r.tr.closed {
		r.closedAtRecord = true
	}
	r.calls = append(r.calls, terminalCall{handle, turnID, status})
	return r.err
}

func successTurnFrames() []string {
	return []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"root-thread"}}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"root-turn"}}}`,
		`{"method":"item/completed","params":{"threadId":"root-thread","turnId":"root-turn","item":{"type":"agentMessage","phase":"final_answer","text":"OK"}}}`,
		`{"method":"turn/completed","params":{"threadId":"root-thread","turn":{"id":"root-turn","status":"completed","items":[]}}}`,
	}
}

func TestEnterRecordsTerminalCompletionForExactTurn(t *testing.T) {
	tr := &scriptedTransport{recv: successTurnFrames()}
	rec := &recordingTerminal{tr: tr}
	c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }, Terminal: rec}
	got, err := c.Enter(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Enter: %v", err)
	}
	if got.Output != "OK" || got.TerminalRecordFailed {
		t.Fatalf("result = %+v", got)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("RecordTerminal called %d times, want 1: %+v", len(rec.calls), rec.calls)
	}
	if rec.calls[0] != (terminalCall{"root-thread", "root-turn", "completed"}) {
		t.Fatalf("terminal call = %+v", rec.calls[0])
	}
	if !rec.closedAtRecord {
		t.Fatal("terminal evidence recorded before the transport was closed")
	}
}

func TestEnterRecordsTerminalFailureAsEvidence(t *testing.T) {
	tr := &scriptedTransport{recv: []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"t"}}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"u"}}}`,
		`{"method":"turn/completed","params":{"threadId":"t","turn":{"id":"u","status":"failed","error":{"message":"boom"},"items":[]}}}`,
	}}
	rec := &recordingTerminal{tr: tr}
	c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }, Terminal: rec}
	_, err := c.Enter(context.Background(), validRequest())
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("Enter error = %v, want a turn-failed error", err)
	}
	if len(rec.calls) != 1 || rec.calls[0] != (terminalCall{"t", "u", "failed"}) {
		t.Fatalf("terminal calls = %+v; want one {t u failed}", rec.calls)
	}
}

func TestEnterTransportLossRecordsNothing(t *testing.T) {
	// EOF mid-turn (no turn/completed frame): transport loss is NOT termination
	// evidence (AC4) — nothing is recorded.
	tr := &scriptedTransport{recv: []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"t"}}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"u"}}}`,
	}}
	rec := &recordingTerminal{tr: tr}
	c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }, Terminal: rec}
	_, err := c.Enter(context.Background(), validRequest())
	if err == nil || !strings.Contains(err.Error(), "ended before completion") {
		t.Fatalf("Enter error = %v, want transport loss", err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("RecordTerminal called %d times on transport loss; want 0: %+v", len(rec.calls), rec.calls)
	}
}

func TestEnterRecordFailureSurfacesWithoutFakingEvidence(t *testing.T) {
	tr := &scriptedTransport{recv: successTurnFrames()}
	rec := &recordingTerminal{tr: tr, err: errors.New("record boom")}
	c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }, Terminal: rec}
	got, err := c.Enter(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Enter must not flip a successful run to failure: %v", err)
	}
	if got.Output != "OK" {
		t.Fatalf("output = %q, want OK", got.Output)
	}
	if !got.TerminalRecordFailed {
		t.Fatal("a record failure on the success path must surface as TerminalRecordFailed")
	}
}
