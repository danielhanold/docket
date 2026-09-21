// Package codexentry enters compositional Docket roles as Codex app-server
// threads over the native protocol. It is deliberately narrower than a generic
// Codex runner: one eligible role, one foreground turn, one final message, and
// no fallback transport.
package codexentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"

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

// ParticipantRegistrar registers this entry's native task handle (its thread id)
// as a run-epoch participant BEFORE the coordinator turn starts, so a later
// cancellation knows the task exists. Registration is lifecycle linkage only — it
// confers no attribution and no authority (claim proofs own attribution). The app
// layer wires the implementation; a nil Registrar registers nothing (the honest
// state for an entry with no run linkage).
type ParticipantRegistrar interface {
	RegisterParticipant(handle string) error
}

// TerminalRecorder persists the adapter's exact terminal observation of this
// entry's native task (the thread/turn that terminated) as run-epoch evidence —
// change 0441. It is recorded ONLY after the transport is torn down, so the
// evidence reflects a settled turn rather than a still-open stream. status is
// terminalCompleted or terminalFailed; a terminal failure is termination evidence
// too (RunVerify independently decides implementation success). The app layer
// wires the implementation; a nil Terminal records nothing (the honest state for
// an entry with no run lifecycle).
type TerminalRecorder interface {
	RecordTerminal(handle, turnID, status string) error
}

// terminalCompleted / terminalFailed are the two terminal-observation statuses
// this adapter records. They are literal strings matching the app layer's
// exported ParticipantTerminalCompleted / ParticipantTerminalFailed constants —
// the recorder validates the value and fails closed on any other, so this
// package never imports the app layer for them (change 0441).
const (
	terminalCompleted = "completed"
	terminalFailed    = "failed"
)

// LifecycleCanceller connects a catchable owner Stop (SIGTERM/SIGINT) to the run's
// cancellation path: it fences the run epoch and tears the run down. It is injected
// from the app layer ONLY for an entry that carries cancellation authority (the root
// coordinator); a feature child registers as a participant but receives a nil
// Canceller — the flags register, they do not confer authority.
type LifecycleCanceller interface {
	CancelRun(reason string) error
}

type Client struct {
	Start StartFunc
	// Registrar, when non-nil, registers this entry's thread as a run-epoch
	// participant before the turn starts.
	Registrar ParticipantRegistrar
	// Canceller, when non-nil, is invoked once if a SIGTERM/SIGINT reaches this
	// owner while it waits on the turn — the signal-connected cancellation path.
	Canceller LifecycleCanceller
	// Terminal, when non-nil, records this entry's exact-turn termination as
	// run-epoch evidence after the turn settles and the transport is closed.
	Terminal TerminalRecorder
	// signalSource, when non-nil, replaces OS signal notification during the turn
	// wait so tests drive the cancellation path deterministically. Production leaves
	// it nil and the wait subscribes to real SIGTERM/SIGINT.
	signalSource func() (<-chan os.Signal, func())
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
	// TerminalRecordFailed is set when the turn produced terminal evidence but
	// persisting it via the TerminalRecorder failed. A record failure never fakes
	// evidence and never flips a successful run to failure — the caller surfaces
	// it so a downstream closeout knows the evidence is missing (change 0441).
	TerminalRecordFailed bool
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
	if req.Contract.LaunchPosture != harness.LaunchRootCoordinator && !(req.Contract.LaunchPosture == harness.LaunchChild && req.Contract.WorktreeScope == harness.WorktreeScopeFeature) {
		return Result{}, fmt.Errorf("role %q is not eligible for app-server entry", req.Contract.Name)
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

	// Register the native task handle (the thread id) as a run-epoch participant
	// BEFORE the turn starts, so a cancellation that races the turn already knows the
	// task exists. A registration failure is fatal — an unregistered turn cannot be
	// reached by a later cancellation, so it must not start (fail closed).
	if c.Registrar != nil {
		if err := c.Registrar.RegisterParticipant(thread.Thread.ID); err != nil {
			return Result{}, fmt.Errorf("registering lifecycle participant: %w", err)
		}
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

	output, waitErr := c.waitTurn(tr, thread.Thread.ID, turn.Turn.ID)

	// Terminal observation (change 0441): record the exact turn's termination as
	// run-epoch evidence, but ONLY after the transport's terminal response and
	// teardown are accounted — close it explicitly here, before the idempotent
	// deferred Close, so the record reflects a settled turn, not an open stream.
	// waitTurn success is terminalCompleted; a turnFailedError is terminalFailed
	// (a terminal failure IS termination evidence). Transport loss and every other
	// plain error are NOT termination evidence and record nothing (AC4).
	_ = tr.Close()
	res := Result{ThreadID: thread.Thread.ID, TurnID: turn.Turn.ID}
	var status string
	var turnFailed turnFailedError
	switch {
	case waitErr == nil:
		status = terminalCompleted
	case errors.As(waitErr, &turnFailed):
		status = terminalFailed
	}
	if status != "" && c.Terminal != nil {
		if recErr := c.Terminal.RecordTerminal(thread.Thread.ID, turn.Turn.ID, status); recErr != nil {
			// A record failure must not fake evidence and must not flip a
			// successful run to failure: surface it, keep the run's own result.
			res.TerminalRecordFailed = true
		}
	}
	if waitErr != nil {
		return res, waitErr
	}
	res.Output = output
	return res, nil
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
		if err := rejectInteractiveRequest(env); err != nil {
			return nil, err
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

// recvFrame carries one transport read across the receiver goroutine boundary so
// the wait can select between an incoming frame and a delivered Stop signal.
type recvFrame struct {
	raw json.RawMessage
	err error
}

// turnFailedError marks a TURN-TERMINAL non-completed outcome: the coordinator
// turn reached a turn/completed frame with a non-"completed" status. It is
// termination evidence (change 0441) — the run failed, but the exact turn
// terminated, so Enter records it. EOF, malformed frames, an interactive-request
// rejection, and "completed without a final agent message" stay plain errors and
// are NOT termination evidence.
type turnFailedError struct {
	status, detail string
}

func (e turnFailedError) Error() string {
	return "coordinator turn " + e.detail
}

// waitTurn observes the coordinator turn to completion. When a Canceller is wired
// it also watches for a catchable Stop (SIGTERM/SIGINT): the FIRST such signal
// invokes the run's cancellation path exactly once and then the wait KEEPS
// observing this exact turn until it produces terminal output — a transport kill
// alone is not a receipt, so the cancellation never truncates the turn's own record.
func (c Client) waitTurn(tr Transport, threadID, turnID string) (string, error) {
	frames := make(chan recvFrame, 8)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			raw, err := tr.Recv()
			select {
			case frames <- recvFrame{raw: raw, err: err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	var sigCh <-chan os.Signal
	if c.Canceller != nil {
		var stop func()
		if c.signalSource != nil {
			sigCh, stop = c.signalSource()
		} else {
			ch := make(chan os.Signal, 4)
			signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
			sigCh, stop = ch, func() { signal.Stop(ch) }
		}
		if stop != nil {
			defer stop()
		}
	}

	var final string
	cancelled := false
	for {
		select {
		case s := <-sigCh:
			// A nil sigCh never fires; a real one fires at most usefully once. Keep
			// observing the turn after cancelling — the transport is not the receipt.
			if !cancelled {
				cancelled = true
				_ = c.Canceller.CancelRun(signalReason(s))
			}
		case fr := <-frames:
			if fr.err != nil {
				if errors.Is(fr.err, io.EOF) {
					return "", fmt.Errorf("coordinator turn ended before completion: %w", fr.err)
				}
				return "", fmt.Errorf("reading coordinator turn: %w", fr.err)
			}
			var env rpcEnvelope
			if err := json.Unmarshal(fr.raw, &env); err != nil {
				return "", fmt.Errorf("coordinator turn received a malformed JSON-RPC frame: %w", err)
			}
			if err := rejectInteractiveRequest(env); err != nil {
				return "", err
			}
			switch env.Method {
			case "item/completed":
				var p completedItemParams
				if json.Unmarshal(env.Params, &p) == nil && p.ThreadID == threadID && p.TurnID == turnID && isFinalMessage(p.Item.Type, p.Item.Phase) {
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
					if isFinalMessage(item.Type, item.Phase) {
						final = item.Text
					}
				}
				if p.Turn.Status != "completed" {
					detail := p.Turn.Status
					if p.Turn.Error != nil && p.Turn.Error.Message != "" {
						detail += ": " + p.Turn.Error.Message
					}
					return "", turnFailedError{status: p.Turn.Status, detail: detail}
				}
				if final == "" {
					return "", fmt.Errorf("coordinator turn completed without a final agent message")
				}
				return final, nil
			}
		}
	}
}

// signalReason maps a delivered Stop signal to the bounded human reason recorded on
// the cancellation. It names only the signal, never argv, environment, or output.
func signalReason(s os.Signal) string {
	return "owner received " + s.String() + "; cancelling the run"
}

// Older app-server versions omit phase. Explicit commentary is never the
// coordinator's final return, even if it is the last message in the turn.
func isFinalMessage(kind, phase string) bool {
	return kind == "agentMessage" && (phase == "" || phase == "final_answer")
}

// A server request needs a reply before the turn can proceed. This foreground
// adapter has no interactive approval/input channel; silently ignoring the
// request would hang forever. Notifications carry no id and remain supported.
func rejectInteractiveRequest(env rpcEnvelope) error {
	if env.Method != "" && len(env.ID) != 0 && string(env.ID) != "null" {
		return fmt.Errorf("Codex app-server requires interactive request %q; foreground root entry has no approval/input channel", env.Method)
	}
	return nil
}
