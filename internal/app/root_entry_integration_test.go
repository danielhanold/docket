//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/codexentry"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
)

// The transport scripts only the model/server boundary. The received request
// drives real claim, completion, publication, evidence and claim-proof scanning
// through runClaimToImplemented. GitHub uses that fixture's stateful gh process.
// The parent asks the gate only AFTER the foreground root returns; no result
// text supplies either ownership or success. The second row strips the dispatch
// context at turn/start and must lose attribution despite completing the change.
func TestIntegrationWorkflowLifecycleRootEntryRunAttribution(t *testing.T) {
	gh := buildFakeGH(t)
	for _, drop := range []bool{false, true} {
		name := "context-preserved"
		if drop {
			name = "context-removed"
		}
		t.Run(name, func(t *testing.T) {
			runClaimToImplemented(t, planRepoModes()[0], gh, func(node realNode, wdeps WorkspaceDeps, complete func(string) GitHubDeps) {
				ctx := context.Background()
				scope := &fakeScopePrep{grant: sampleScopeGrant()}
				started := RunStart(ctx, node.deps, wdeps, scope.deps(), node.dir, "implement-next", 0)
				if !started.Started || started.RunContext == "" {
					t.Fatalf("start: %+v", started)
				}
				catalog, err := assets.EmbeddedCatalog()
				if err != nil {
					t.Fatal(err)
				}
				contract, err := codex.RoleContractFor(harness.PlanInput{Assets: catalog}, "docket-implement-next")
				if err != nil {
					t.Fatal(err)
				}
				var gdeps GitHubDeps
				completed := false
				tr := &workflowRootTransport{dropContext: drop, complete: func(request string) {
					var token string
					for _, line := range strings.Split(request, "\n") {
						if value, ok := strings.CutPrefix(line, "Run context: "); ok {
							token = value
						}
					}
					gdeps = complete(token)
					completed = true
				}}
				// Wire the run lifecycle registration a real root-coordinator
				// entry carries (internal/cli/agent.go's runParticipantRegistrar /
				// runTerminalRecorder): the coordinator thread registers as a native
				// participant before the turn and records its exact-turn terminal
				// observation after it settles. Without this the successful-run closeout
				// (change 0441) would observe an absent coordinator rather than the
				// terminal-observed one a production run establishes.
				lifecycle := runLifecycleFixture{repo: node.dir, key: started.Key}
				client := codexentry.Client{
					Start:     func(context.Context, string) (codexentry.Transport, error) { return tr, nil },
					Registrar: lifecycle,
					Terminal:  lifecycle,
				}
				_, err = client.Enter(ctx, codexentry.Request{
					Contract: contract, CWD: node.dir, ApprovalPolicy: "never", Sandbox: "workspace-write",
					UserRequest: "Please implement change 3.\nRun context: " + started.RunContext + "\n",
				})
				if err != nil || !completed || !tr.closed {
					t.Fatalf("foreground root: err=%v completed=%v closed=%v", err, completed, tr.closed)
				}
				wdeps.ClaimProofs = NewClaimProofScanner(node.deps)
				verdict := RunVerdict(ctx, node.deps, wdeps, gdeps, node.dir, started.Key)
				want := "run-done " + started.Key + " run-complete 3"
				if drop {
					want = "run-done " + started.Key + " no-attributable-claim"
				}
				if got := verdict.HumanText(); got != want {
					t.Fatalf("claim-binding bridge: got %q, want %q", got, want)
				}
			})
		})
	}
}

type workflowRootTransport struct {
	frames              []json.RawMessage
	request             string
	dropContext, closed bool
	complete            func(string)
}

func (tr *workflowRootTransport) Send(value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var msg struct {
		Method string `json:"method"`
		Params struct {
			Input []struct {
				Text string `json:"text"`
			} `json:"input"`
		} `json:"params"`
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		return err
	}
	var frame string
	switch msg.Method {
	case "initialize":
		frame = `{"id":1,"result":{}}`
	case "thread/start":
		frame = `{"id":2,"result":{"thread":{"id":"root"}}}`
	case "turn/start":
		for _, input := range msg.Params.Input {
			tr.request += input.Text
		}
		if tr.dropContext {
			tr.request = "Please implement change 3.\n"
		}
		frame = `{"id":3,"result":{"turn":{"id":"turn"}}}`
	}
	if frame != "" {
		tr.frames = append(tr.frames, json.RawMessage(frame))
	}
	return nil
}

func (tr *workflowRootTransport) Recv() (json.RawMessage, error) {
	if len(tr.frames) != 0 {
		frame := tr.frames[0]
		tr.frames = tr.frames[1:]
		return frame, nil
	}
	if tr.complete == nil {
		return nil, io.EOF
	}
	tr.complete(tr.request)
	tr.complete = nil
	// Deliberately misleading prose must never be an attribution source.
	return json.RawMessage(`{"method":"turn/completed","params":{"threadId":"root","turn":{"id":"turn","status":"completed","items":[{"type":"agentMessage","text":"Completed sibling change 999"}]}}}`), nil
}

func (tr *workflowRootTransport) Close() error { tr.closed = true; return nil }

// runLifecycleFixture mirrors the production root-coordinator lifecycle seams
// (internal/cli/agent.go): as a codexentry.ParticipantRegistrar it registers the
// coordinator thread as a native run participant before the turn, and as a
// codexentry.TerminalRecorder it stamps the exact-turn terminal observation after
// the turn settles. It lets the end-to-end fixture establish the same terminal
// evidence a real run does, so the successful-run closeout (change 0441) observes a
// terminal-observed coordinator instead of an absent one.
type runLifecycleFixture struct{ repo, key string }

func (e runLifecycleFixture) RegisterParticipant(handle string) error {
	return RegisterRunParticipant(e.repo, e.key,
		RunParticipant{Kind: participantKindCoordinator, NativeHandle: handle})
}

func (e runLifecycleFixture) RecordTerminal(handle, turnID, status string) error {
	return RecordRunParticipantTerminal(e.repo, e.key, handle, turnID, status)
}
