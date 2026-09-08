package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/testsupport"
)

func TestAgentEnterCommandRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"agent", "enter"})
	if err != nil || cmd == nil || cmd.Name() != "enter" {
		t.Fatalf("agent enter not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"role", "request", "cwd", "approval-policy", "sandbox", "worktree"} {
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
	seedAgentInstallation(t)
	dir := testsupport.TempDir(t)
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
	t.Setenv("DOCKET_AGENT_TEST_ROLE", "docket-implement-next")
	t.Setenv("DOCKET_AGENT_TEST_SKILL", "docket-implement-next")
	t.Setenv("DOCKET_AGENT_TEST_DEVELOPER", installedAgentTestContract(t, "docket-implement-next").DeveloperInstructions)
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

// This reaches the app-server protocol through the CLI and demonstrates that
// the target feature worktree, rather than the dispatcher's checkout, becomes
// both the server process cwd and thread/start cwd.
func TestAgentEnterCLIUsesVerifiedFeatureWorktree(t *testing.T) {
	paths := newAgentEntryWorktrees(t)
	seedAgentInstallation(t)
	dir := testsupport.TempDir(t)
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
	request := "Resolve this rebase exactly.\nOpaque dispatch context: `unchanged`.\n"
	t.Setenv("DOCKET_AGENT_TEST_REQUEST", request)
	t.Setenv("DOCKET_AGENT_TEST_CWD", paths.b)
	t.Setenv("DOCKET_AGENT_TEST_ROLE", "docket-rebase-resolver")
	t.Setenv("DOCKET_AGENT_TEST_SKILL", "docket-convention")
	t.Setenv("DOCKET_AGENT_TEST_DEVELOPER", installedAgentTestContract(t, "docket-rebase-resolver").DeveloperInstructions)

	var out, stderr bytes.Buffer
	code := Run([]string{"agent", "enter", "--role", "docket-rebase-resolver", "--request", "-", "--cwd", paths.a, "--worktree", paths.b, "--approval-policy", "never", "--sandbox", "workspace-write", "--json"}, strings.NewReader(request), &out, &stderr, devInfo(), hostFacts())
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
	}
	var result app.AgentEnterResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Result != app.ResultApplied || result.Role != "docket-rebase-resolver" || result.ThreadID != "root" || result.TurnID != "turn" {
		t.Fatalf("receipt: %+v", result)
	}
}

// Codex resolves a registered role from the thread's repository before the
// user-global registry. agent.enter must seed the root thread from that same
// definition, and feature entry must anchor the lookup in the verified target
// worktree rather than the coordinator's checkout.
func TestAgentEnterCLIUsesEffectiveRepositoryRoleBeforeGlobal(t *testing.T) {
	paths := newAgentEntryWorktrees(t)
	home := seedAgentInstallation(t)
	writeAgentTestRole(t, home, "docket-implement-next", "global-root-model", "minimal", "GLOBAL ROOT DEVELOPER")
	writeAgentTestRole(t, home, "docket-rebase-resolver", "global-feature-model", "medium", "GLOBAL FEATURE DEVELOPER")
	dir := testsupport.TempDir(t)
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
	request := "Preserve repository role precedence.\n"
	t.Setenv("DOCKET_AGENT_TEST_REQUEST", request)

	for _, tc := range []struct {
		name, role, caller, worktree, skill, model, effort, developer string
	}{
		{"root uses caller repository", "docket-implement-next", paths.a, "", "docket-implement-next", "repo-root-model", "high", "REPOSITORY ROOT DEVELOPER"},
		{"feature uses target worktree repository", "docket-rebase-resolver", paths.a, paths.b, "docket-convention", "repo-feature-model", "low", "REPOSITORY FEATURE DEVELOPER"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			effective := tc.caller
			if tc.worktree != "" {
				effective = tc.worktree
				writeAgentTestRole(t, tc.caller, tc.role, "wrong-caller-model", "minimal", "WRONG CALLER DEVELOPER")
			}
			writeAgentTestRole(t, effective, tc.role, tc.model, tc.effort, tc.developer)
			t.Setenv("DOCKET_AGENT_TEST_CWD", effective)
			t.Setenv("DOCKET_AGENT_TEST_ROLE", tc.role)
			t.Setenv("DOCKET_AGENT_TEST_SKILL", tc.skill)
			t.Setenv("DOCKET_AGENT_TEST_DEVELOPER", tc.developer)
			t.Setenv("DOCKET_AGENT_TEST_MODEL", tc.model)
			t.Setenv("DOCKET_AGENT_TEST_EFFORT", tc.effort)
			args := []string{"agent", "enter", "--role", tc.role, "--request", "-", "--cwd", tc.caller, "--approval-policy", "never", "--sandbox", "workspace-write", "--json"}
			if tc.worktree != "" {
				args = append(args, "--worktree", tc.worktree)
			}
			var out, stderr bytes.Buffer
			code := Run(args, strings.NewReader(request), &out, &stderr, devInfo(), hostFacts())
			if code != 0 || stderr.Len() != 0 {
				t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
			}
			var result app.AgentEnterResult
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Result != app.ResultApplied {
				t.Fatalf("receipt: %+v", result)
			}
		})
	}
}

func writeAgentTestRole(t *testing.T, repo, role, model, effort, developer string) {
	t.Helper()
	dir := filepath.Join(repo, ".codex", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("name = %q\ndescription = %q\nmodel = %q\nmodel_reasoning_effort = %q\ndeveloper_instructions = %q\n", role, "repository role", model, effort, developer)
	if err := os.WriteFile(filepath.Join(dir, role+".toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func installedAgentTestContract(t *testing.T, role string) codex.RoleContract {
	t.Helper()
	opts, refusal := installOptions(context.Background(), []string{"codex"}, "", false, devInfo())
	if refusal != nil {
		t.Fatal(refusal)
	}
	contract, err := codex.RoleContractFor(harness.PlanInput{Assets: opts.Catalog, Agents: opts.Config.Effective.Agents}, role)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := os.ReadFile(filepath.Join(opts.Roots.Home, ".codex", "agents", role+".toml"))
	if err != nil {
		t.Fatal(err)
	}
	contract, err = codex.RoleContractFromDefinition(definition, contract)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func seedAgentInstallation(t *testing.T) string {
	t.Helper()
	home := pinInstallEnv(t)
	t.Chdir(testsupport.TempDir(t))
	if out, stderr, code := runCLI(t, "install", "--harness", "codex", "--json"); code != 0 {
		t.Fatalf("install: code=%d out=%s stderr=%s", code, out, stderr)
	}
	return home
}

func TestAgentEnterRejectsInstalledContractDrift(t *testing.T) {
	for _, mutate := range []string{"missing-role", "edited-role", "edited-skill"} {
		t.Run(mutate, func(t *testing.T) {
			home := seedAgentInstallation(t)
			// No real Codex process may run in this refusal fixture, even if
			// the contract guard is removed by a mutation.
			t.Setenv("PATH", testsupport.TempDir(t))
			role := filepath.Join(home, ".codex", "agents", "docket-implement-next.toml")
			switch mutate {
			case "missing-role":
				if err := os.Remove(role); err != nil {
					t.Fatal(err)
				}
			case "edited-role":
				if err := os.WriteFile(role, []byte("name = \"different-role\"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "edited-skill":
				p := filepath.Join(home, ".agents", "skills", "docket-implement-next", "SKILL.md")
				if err := os.Chmod(p, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("# Stale contract\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out, _, _ := runCLI(t, "agent", "enter", "--role", "docket-implement-next", "--request", "-", "--cwd", testsupport.TempDir(t), "--approval-policy", "never", "--sandbox", "workspace-write", "--json")
			var res app.AgentEnterResult
			if err := json.Unmarshal([]byte(out), &res); err != nil {
				t.Fatal(err)
			}
			if res.Reason != "role-contract-unavailable" {
				t.Fatalf("%s: %+v", mutate, res)
			}
		})
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
			processCWD, err := os.Getwd()
			if err != nil || processCWD != os.Getenv("DOCKET_AGENT_TEST_CWD") {
				os.Exit(4)
			}
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
			if dev != os.Getenv("DOCKET_AGENT_TEST_DEVELOPER") {
				os.Exit(5)
			}
			if want := os.Getenv("DOCKET_AGENT_TEST_MODEL"); want != "" {
				var model string
				_ = json.Unmarshal(msg.Params["model"], &model)
				if model != want {
					os.Exit(5)
				}
			}
			fmt.Println(`{"id":2,"result":{"thread":{"id":"root"}}}`)
		case "turn/start":
			if want := os.Getenv("DOCKET_AGENT_TEST_EFFORT"); want != "" {
				var effort string
				_ = json.Unmarshal(msg.Params["effort"], &effort)
				if effort != want {
					os.Exit(5)
				}
			}
			var inputs []struct{ Type, Text, Name, Path string }
			_ = json.Unmarshal(msg.Params["input"], &inputs)
			var text string
			skill := false
			for _, input := range inputs {
				if input.Type == "text" {
					text += input.Text
				}
				if input.Type == "skill" && input.Name == os.Getenv("DOCKET_AGENT_TEST_SKILL") && filepath.IsAbs(input.Path) {
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
	for _, tc := range []struct{ role, reason string }{{"docket-missing", "unknown-role"}, {"docket-brainstorm-consultant", "ordinary-child-role"}} {
		out, _, _ := runCLI(t, "agent", "enter", "--role", tc.role, "--request", "-", "--cwd", testsupport.TempDir(t), "--approval-policy", "never", "--sandbox", "workspace-write", "--json")
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
	want := "--approval-policy <policy> --cwd <dir> --request <file> --role <name> --sandbox <mode> [--worktree <dir>]"
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

	dir := testsupport.TempDir(t)
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
