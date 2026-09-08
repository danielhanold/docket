package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/assets"
)

func TestAgentEnterCommandRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"agent", "enter"})
	if err != nil || cmd == nil || cmd.Name() != "enter" {
		t.Fatalf("agent enter not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"role", "request", "cwd", "approval-policy", "sandbox"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("agent enter: missing --%s flag", flag)
		}
	}
	if !assetIndependent["agent"] {
		t.Error("the bare agent group must remain asset-independent")
	}
	if assetIndependent["agent enter"] || !assetDependent["agent enter"] {
		t.Error("agent enter must be explicitly classified as requiring a compatible installed role registry")
	}
}

// Exercise CLI parsing, stdin, installed-contract selection, process launch and
// final receipt together. The subprocess scripts only Codex's protocol boundary.
func TestAgentEnterCLIPreservesRequestAndReceipt(t *testing.T) {
	pinInstallEnv(t)
	writeInstallState(t, assets.AssetProtocol)
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=^TestAgentEnterServerProcess$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKET_AGENT_TEST_SERVER", "1")
	request := "Please implement change 393.\nDispatch context: opaque-token\nPreserve `literal` and $bytes.\n"
	t.Setenv("DOCKET_AGENT_TEST_REQUEST", request)
	t.Setenv("DOCKET_AGENT_TEST_CWD", dir)
	for _, jsonMode := range []bool{false, true} {
		args := []string{"agent", "enter", "--role", "docket-implement-next", "--request", "-", "--cwd", dir, "--approval-policy", "never", "--sandbox", "workspace-write"}
		if jsonMode {
			args = append(args, "--json")
		}
		var out, stderr bytes.Buffer
		code := Run(args, strings.NewReader(request), &out, &stderr, devInfo(), hostFacts())
		if code != 0 || stderr.Len() != 0 {
			t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
		}
		if jsonMode {
			var res app.AgentEnterResult
			if err := json.Unmarshal(out.Bytes(), &res); err != nil {
				t.Fatal(err)
			}
			if res.Result != app.ResultApplied || res.Operation != "agent.enter" || res.Role != "docket-implement-next" || res.ThreadID != "root" || res.TurnID != "turn" || res.Output != "ROOT RESULT" {
				t.Fatalf("receipt: %+v", res)
			}
		} else if out.String() != "ROOT RESULT\n" {
			t.Fatalf("human receipt: %q", out.String())
		}
	}
}

func TestAgentEnterServerProcess(t *testing.T) {
	if os.Getenv("DOCKET_AGENT_TEST_SERVER") != "1" {
		return
	}
	if got := os.Args[len(os.Args)-2:]; got[0] != "app-server" || got[1] != "--stdio" {
		os.Exit(2)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var msg struct {
			Method string                     `json:"method"`
			Params map[string]json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			os.Exit(3)
		}
		switch msg.Method {
		case "initialize":
			fmt.Println(`{"id":1,"result":{}}`)
		case "thread/start":
			for key, want := range map[string]string{"cwd": os.Getenv("DOCKET_AGENT_TEST_CWD"), "approvalPolicy": "never", "sandbox": "workspace-write", "threadSource": "vscode"} {
				var got string
				_ = json.Unmarshal(msg.Params[key], &got)
				if got != want {
					fmt.Fprintf(os.Stderr, "%s=%q, want %q", key, got, want)
					os.Exit(4)
				}
			}
			var dev string
			_ = json.Unmarshal(msg.Params["developerInstructions"], &dev)
			if !strings.Contains(dev, "docket-implement-next") || !strings.Contains(dev, "Before acting, load these docket skills") {
				os.Exit(5)
			}
			fmt.Println(`{"id":2,"result":{"thread":{"id":"root"}}}`)
		case "turn/start":
			var inputs []struct{ Type, Text, Name, Path string }
			_ = json.Unmarshal(msg.Params["input"], &inputs)
			var text string
			skill := false
			for _, input := range inputs {
				if input.Type == "text" {
					text += input.Text
				}
				if input.Type == "skill" && input.Name == "docket-implement-next" && filepath.IsAbs(input.Path) {
					skill = true
				}
			}
			if text != os.Getenv("DOCKET_AGENT_TEST_REQUEST") || !skill {
				os.Exit(6)
			}
			fmt.Println(`{"id":3,"result":{"turn":{"id":"turn"}}}`)
			fmt.Println(`{"method":"turn/completed","params":{"threadId":"root","turn":{"id":"turn","status":"completed","items":[{"type":"agentMessage","text":"ROOT RESULT"}]}}}`)
		}
	}
	os.Exit(0)
}

func TestAgentEnterRefusesNonCoordinatorRoles(t *testing.T) {
	pinInstallEnv(t)
	writeInstallState(t, assets.AssetProtocol)
	for _, tc := range []struct{ role, reason string }{{"docket-missing", "unknown-role"}, {"docket-plan-writer", "ordinary-child-role"}} {
		out, _, _ := runCLI(t, "agent", "enter", "--role", tc.role, "--request", "-", "--cwd", t.TempDir(), "--approval-policy", "never", "--sandbox", "workspace-write", "--json")
		var res app.AgentEnterResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatal(err)
		}
		if res.Reason != tc.reason {
			t.Fatalf("role %s: %+v", tc.role, res)
		}
	}
}

func TestAgentEnterCapabilitySignature(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := entryByID(entries, "agent.enter")
	want := "--approval-policy <policy> --cwd <dir> --request <file> --role <name> --sandbox <mode>"
	if !ok || entry.Signature != want {
		t.Fatalf("agent.enter signature = %q, present=%v; want %q", entry.Signature, ok, want)
	}
}

func TestAgentEnterRequiresClosedExecutionContext(t *testing.T) {
	pinInstallEnv(t)
	writeInstallState(t, assets.AssetProtocol)

	if _, errS, code := runCLI(t, "agent", "enter"); code != 2 || !strings.Contains(errS, "required") {
		t.Fatalf("missing flags: stderr=%q code=%d", errS, code)
	}

	dir := t.TempDir()
	base := []string{"agent", "enter", "--role", "docket-implement-next", "--request", "-", "--cwd", dir}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown approval", []string{"--approval-policy", "sometimes", "--sandbox", "workspace-write"}, "approval policy"},
		{"unknown sandbox", []string{"--approval-policy", "never", "--sandbox", "host-root"}, "sandbox mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string(nil), base...), tc.args...)
			_, errS, code := runCLI(t, args...)
			if code != 2 || !strings.Contains(errS, tc.want) {
				t.Fatalf("stderr=%q code=%d, want %q", errS, code, tc.want)
			}
		})
	}
}
