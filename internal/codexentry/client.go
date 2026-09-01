// Package codexentry enters compositional Docket roles as Codex root threads
// over the native app-server protocol. It is deliberately narrower than a
// generic Codex runner: one root-coordinator role, one foreground turn, one
// final message, and no fallback transport.
package codexentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
)

// Transport is the newline-framed JSON-RPC connection to one app-server.
// Production supplies a child process; tests supply a deterministic script.
type Transport interface {
	Send(any) error
	Recv() (json.RawMessage, error)
	Close() error
}

type StartFunc func(context.Context, string) (Transport, error)

type Client struct {
	Start StartFunc
}

type SkillInput struct {
	Name string
	Path string
}

type Request struct {
	Contract       codex.RoleContract
	UserRequest    string
	CWD            string
	ApprovalPolicy string
	Sandbox        string
	Skills         []SkillInput
}

type Result struct {
	Output   string
	ThreadID string
	TurnID   string
}

// ValidateExecutionContext keeps Docket's root-entry surface closed over the
// execution-context spellings understood by Codex app-server. Unknown values
// are refused before a process or thread is started.
func ValidateExecutionContext(approvalPolicy, sandbox string) error {
	if !oneOf(approvalPolicy, "untrusted", "on-request", "never") {
		return fmt.Errorf("unsupported approval policy %q", approvalPolicy)
	}
	if !oneOf(sandbox, "read-only", "workspace-write", "danger-full-access") {
		return fmt.Errorf("unsupported sandbox mode %q", sandbox)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (c Client) Enter(ctx context.Context, req Request) (Result, error) {
	if err := ValidateExecutionContext(req.ApprovalPolicy, req.Sandbox); err != nil {
		return Result{}, err
	}
	if req.Contract.LaunchPosture != harness.LaunchRootCoordinator {
		return Result{}, fmt.Errorf("role %q has launch posture %q, not %q", req.Contract.Name, req.Contract.LaunchPosture, harness.LaunchRootCoordinator)
	}
	start := c.Start
	if start == nil {
		start = StartAppServer
	}
	tr, err := start(ctx, req.CWD)
	if err != nil {
		return Result{}, fmt.Errorf("starting Codex app-server: %w", err)
	}
	defer tr.Close()

	if err := sendRequest(tr, 1, "initialize", map[string]any{
		"clientInfo": map[string]any{"name": "docket", "title": "Docket root coordinator entry", "version": "1"},
	}); err != nil {
		return Result{}, fmt.Errorf("sending initialize: %w", err)
	}
	if _, err := waitResponse(tr, 1, "initialize"); err != nil {
		return Result{}, err
	}
	if err := tr.Send(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		return Result{}, fmt.Errorf("sending initialized notification: %w", err)
	}

	threadParams := map[string]any{
		"cwd":                   req.CWD,
		"developerInstructions": req.Contract.DeveloperInstructions,
		"approvalPolicy":        req.ApprovalPolicy,
		"sandbox":               req.Sandbox,
		"threadSource":          "vscode",
	}
	if req.Contract.Model != "" {
		threadParams["model"] = req.Contract.Model
	}
	if err := sendRequest(tr, 2, "thread/start", threadParams); err != nil {
		return Result{}, fmt.Errorf("sending root-thread creation: %w", err)
	}
	raw, err := waitResponse(tr, 2, "root-thread creation")
	if err != nil {
		return Result{}, err
	}
	var thread threadStartResult
	if err := json.Unmarshal(raw, &thread); err != nil || thread.Thread.ID == "" {
		return Result{}, fmt.Errorf("root-thread creation returned a malformed result")
	}

	inputs := make([]map[string]any, 0, len(req.Skills)+1)
	for _, skill := range req.Skills {
		inputs = append(inputs, map[string]any{"type": "skill", "name": skill.Name, "path": skill.Path})
	}
	inputs = append(inputs, map[string]any{"type": "text", "text": req.UserRequest})
	turnParams := map[string]any{"threadId": thread.Thread.ID, "input": inputs}
	if req.Contract.Model != "" {
		turnParams["model"] = req.Contract.Model
	}
	if req.Contract.Effort != "" {
		turnParams["effort"] = req.Contract.Effort
	}
	if err := sendRequest(tr, 3, "turn/start", turnParams); err != nil {
		return Result{}, fmt.Errorf("sending coordinator turn: %w", err)
	}
	raw, err = waitResponse(tr, 3, "coordinator turn")
	if err != nil {
		return Result{}, err
	}
	var turn turnStartResult
	if err := json.Unmarshal(raw, &turn); err != nil || turn.Turn.ID == "" {
		return Result{}, fmt.Errorf("coordinator turn returned a malformed result")
	}

	output, err := waitTurn(tr, thread.Thread.ID, turn.Turn.ID)
	if err != nil {
		return Result{}, err
	}
	return Result{Output: output, ThreadID: thread.Thread.ID, TurnID: turn.Turn.ID}, nil
}

func sendRequest(tr Transport, id int, method string, params any) error {
	return tr.Send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

func waitResponse(tr Transport, id int, phase string) (json.RawMessage, error) {
	want := strconv.Itoa(id)
	for {
		raw, err := tr.Recv()
		if err != nil {
			return nil, fmt.Errorf("%s ended before its response: %w", phase, err)
		}
		var env rpcEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("%s received a malformed JSON-RPC frame: %w", phase, err)
		}
		if string(env.ID) != want {
			continue
		}
		if env.Error != nil {
			return nil, fmt.Errorf("%s rejected by Codex app-server: %s", phase, env.Error.Message)
		}
		if len(env.Result) == 0 {
			return nil, fmt.Errorf("%s returned no result", phase)
		}
		return env.Result, nil
	}
}

func waitTurn(tr Transport, threadID, turnID string) (string, error) {
	var final string
	for {
		raw, err := tr.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("coordinator turn ended before completion: %w", err)
			}
			return "", fmt.Errorf("reading coordinator turn: %w", err)
		}
		var env rpcEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return "", fmt.Errorf("coordinator turn received a malformed JSON-RPC frame: %w", err)
		}
		switch env.Method {
		case "item/completed":
			var p completedItemParams
			if json.Unmarshal(env.Params, &p) == nil && p.ThreadID == threadID && p.TurnID == turnID && p.Item.Type == "agentMessage" {
				final = p.Item.Text
			}
		case "turn/completed":
			var p completedTurnParams
			if err := json.Unmarshal(env.Params, &p); err != nil {
				return "", fmt.Errorf("coordinator turn completion was malformed: %w", err)
			}
			if p.ThreadID != threadID || p.Turn.ID != turnID {
				continue
			}
			for _, item := range p.Turn.Items {
				if item.Type == "agentMessage" {
					final = item.Text
				}
			}
			if p.Turn.Status != "completed" {
				detail := p.Turn.Status
				if p.Turn.Error != nil && p.Turn.Error.Message != "" {
					detail += ": " + p.Turn.Error.Message
				}
				return "", fmt.Errorf("coordinator turn %s", detail)
			}
			if final == "" {
				return "", fmt.Errorf("coordinator turn completed without a final agent message")
			}
			return final, nil
		}
	}
}
