package codexentry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
)

type scriptedTransport struct {
	recv   []string
	sent   []map[string]any
	closed bool
}

func (s *scriptedTransport) Send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	s.sent = append(s.sent, m)
	return nil
}

func (s *scriptedTransport) Recv() (json.RawMessage, error) {
	if len(s.recv) == 0 {
		return nil, io.EOF
	}
	raw := s.recv[0]
	s.recv = s.recv[1:]
	return json.RawMessage(raw), nil
}

func (s *scriptedTransport) Close() error { s.closed = true; return nil }

func validRequest() Request {
	return Request{
		Contract: codex.RoleContract{
			Name:                  "docket-implement-next",
			LaunchPosture:         harness.LaunchRootCoordinator,
			Model:                 "gpt-test",
			Effort:                "max",
			DeveloperInstructions: "ROLE-CONTRACT",
		},
		UserRequest:    "implement docket change 393\nunchanged",
		CWD:            "/repo",
		ApprovalPolicy: "never",
		Sandbox:        "danger-full-access",
		Skills:         []SkillInput{{Name: "docket-implement-next", Path: "/home/u/.agents/skills/docket-implement-next/SKILL.md"}},
	}
}

func TestEnterMapsContractAndWaitsForRootCompletion(t *testing.T) {
	tr := &scriptedTransport{recv: []string{
		`{"jsonrpc":"2.0","id":1,"result":{"userAgent":"test"}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"root-thread"}}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"root-turn","status":"inProgress"}}}`,
		// A child's completion must not terminate the root wait.
		`{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"child-thread","turn":{"id":"child-turn","status":"completed","items":[]}}}`,
		`{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"root-thread","turnId":"root-turn","completedAtMs":1,"item":{"type":"agentMessage","id":"m1","text":"ROOT RESULT"}}}`,
		`{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"root-thread","turn":{"id":"root-turn","status":"completed","items":[]}}}`,
	}}
	c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }}
	got, err := c.Enter(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Enter: %v", err)
	}
	if got.Output != "ROOT RESULT" || got.ThreadID != "root-thread" || got.TurnID != "root-turn" {
		t.Fatalf("result = %+v", got)
	}
	if !tr.closed {
		t.Fatal("transport was not closed")
	}
	if len(tr.sent) != 4 { // initialize, initialized, thread/start, turn/start
		t.Fatalf("sent %d frames, want 4: %#v", len(tr.sent), tr.sent)
	}
	if tr.sent[0]["method"] != "initialize" || tr.sent[1]["method"] != "initialized" || tr.sent[2]["method"] != "thread/start" || tr.sent[3]["method"] != "turn/start" {
		t.Fatalf("method sequence = %#v", tr.sent)
	}
	threadParams := tr.sent[2]["params"].(map[string]any)
	for key, want := range map[string]any{
		"cwd": "/repo", "developerInstructions": "ROLE-CONTRACT", "model": "gpt-test",
		"approvalPolicy": "never", "sandbox": "danger-full-access", "threadSource": "vscode",
	} {
		if got := threadParams[key]; got != want {
			t.Errorf("thread/start %s = %#v, want %#v", key, got, want)
		}
	}
	turnParams := tr.sent[3]["params"].(map[string]any)
	if turnParams["effort"] != "max" || turnParams["threadId"] != "root-thread" {
		t.Fatalf("turn/start params = %#v", turnParams)
	}
	wantInput := []any{
		map[string]any{"type": "skill", "name": "docket-implement-next", "path": "/home/u/.agents/skills/docket-implement-next/SKILL.md"},
		map[string]any{"type": "text", "text": "implement docket change 393\nunchanged"},
	}
	if !reflect.DeepEqual(turnParams["input"], wantInput) {
		t.Fatalf("turn input = %#v, want %#v", turnParams["input"], wantInput)
	}
}

func TestEnterClassifiesProtocolFailures(t *testing.T) {
	cases := []struct {
		name string
		recv []string
		want string
	}{
		{"initialize rpc error", []string{`{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"no init"}}`}, "initialize"},
		{"thread rpc error", []string{`{"jsonrpc":"2.0","id":1,"result":{}}`, `{"jsonrpc":"2.0","id":2,"error":{"code":-2,"message":"no thread"}}`}, "root-thread creation"},
		{"turn rpc error", []string{`{"jsonrpc":"2.0","id":1,"result":{}}`, `{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"t"}}}`, `{"jsonrpc":"2.0","id":3,"error":{"code":-3,"message":"no turn"}}`}, "coordinator turn"},
		{"turn failed", []string{`{"jsonrpc":"2.0","id":1,"result":{}}`, `{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"t"}}}`, `{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"u"}}}`, `{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"t","turn":{"id":"u","status":"failed","error":{"message":"boom"},"items":[]}}}`}, "failed"},
		{"missing output", []string{`{"jsonrpc":"2.0","id":1,"result":{}}`, `{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"t"}}}`, `{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"u"}}}`, `{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"t","turn":{"id":"u","status":"completed","items":[]}}}`}, "without a final agent message"},
		{"malformed frame", []string{`not-json`}, "malformed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := &scriptedTransport{recv: append([]string(nil), tc.recv...)}
			c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }}
			_, err := c.Enter(context.Background(), validRequest())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
			if !tr.closed {
				t.Fatal("transport was not closed after error")
			}
		})
	}
}

func TestEnterSurfacesStartFailure(t *testing.T) {
	c := Client{Start: func(context.Context, string) (Transport, error) { return nil, errors.New("missing codex") }}
	_, err := c.Enter(context.Background(), validRequest())
	if err == nil || !strings.Contains(err.Error(), "starting Codex app-server") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateExecutionContextRejectsUnknownValues(t *testing.T) {
	cases := []struct {
		approval string
		sandbox  string
		want     string
	}{
		{approval: "sometimes", sandbox: "danger-full-access", want: "approval policy"},
		{approval: "never", sandbox: "host-root", want: "sandbox mode"},
	}
	for _, tc := range cases {
		err := ValidateExecutionContext(tc.approval, tc.sandbox)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("ValidateExecutionContext(%q, %q) = %v, want %q", tc.approval, tc.sandbox, err, tc.want)
		}
	}
	for _, approval := range []string{"untrusted", "on-request", "never"} {
		for _, sandbox := range []string{"read-only", "workspace-write", "danger-full-access"} {
			if err := ValidateExecutionContext(approval, sandbox); err != nil {
				t.Fatalf("ValidateExecutionContext(%q, %q): %v", approval, sandbox, err)
			}
		}
	}
}
