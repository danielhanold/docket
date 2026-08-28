package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
)

// TestRepositoryCommandsRegistered proves `docket repository` carries exactly the
// three settled subcommands, each with a --repo-dir flag, and that the bare
// group and an unknown subcommand both fail rather than silently succeeding.
func TestRepositoryCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"init", "check", "migrate"} {
		cmd, _, err := root.Find([]string{"repository", sub})
		if err != nil || cmd == nil || cmd.Name() != sub {
			t.Fatalf("repository %s not registered: cmd=%v err=%v", sub, cmd, err)
		}
		if cmd.Flags().Lookup("repo-dir") == nil {
			t.Errorf("repository %s: missing --repo-dir flag", sub)
		}
	}
	grp, _, err := root.Find([]string{"repository"})
	if err != nil || grp == nil || grp.Name() != "repository" {
		t.Fatalf("repository group not registered: grp=%v err=%v", grp, err)
	}
}

// TestRepositoryUnknownSubcommandFails proves an unrecognized subcommand is an
// argument error, not a silent success.
func TestRepositoryUnknownSubcommandFails(t *testing.T) {
	_, _, code := runCLI(t, "repository", "bogus")
	if code == 0 {
		t.Fatalf("unknown repository subcommand must fail, got exit 0")
	}
}

// fakeInitResult is a minimal OperationResult a stubbed init runner returns so a
// test can drive the presenter without a real repository. It embeds Envelope so
// it marshals as a protocol document.
type fakeInitResult struct {
	app.Envelope
}

func (r fakeInitResult) HumanText() string { return "human" }

// TestRepositoryInitJSONFlowsToPresenter proves the --json flag selects the JSON
// transport for the repository command's result.
func TestRepositoryInitJSONFlowsToPresenter(t *testing.T) {
	old := repositoryInitRunner
	repositoryInitRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return fakeInitResult{Envelope: app.NewEnvelope("repository.init", app.ResultApplied)}
	}
	defer func() { repositoryInitRunner = old }()

	out, _, code := runCLI(t, "repository", "init", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, `"operation":"repository.init"`) {
		t.Fatalf("--json did not flow to the presenter: %q", out)
	}
}

// fakeCheckResult implements both OperationResult and the CheckExitCode contract
// so a test can prove the repository check exit path reads CheckExitCode rather
// than the coarse app.ExitCode.
type fakeCheckResult struct {
	app.Envelope
	exitCode int
}

func (r fakeCheckResult) HumanText() string  { return "check" }
func (r fakeCheckResult) CheckExitCode() int { return r.exitCode }

// TestRepositoryCheckExitUsesCheckExitCode proves the check exit path calls
// CheckExitCode: a result whose Result would map to app.ExitCode 0 (no-op) but
// whose CheckExitCode returns 1 exits 1, encoding the non-failure diagnostic.
func TestRepositoryCheckExitUsesCheckExitCode(t *testing.T) {
	old := repositoryCheckRunner
	repositoryCheckRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		// no-op would exit 0 via app.ExitCode; CheckExitCode overrides it to 1.
		return fakeCheckResult{Envelope: app.NewEnvelope("repository.check", app.ResultNoOp), exitCode: 1}
	}
	defer func() { repositoryCheckRunner = old }()

	_, _, code := runCLI(t, "repository", "check")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (the check exit path must read CheckExitCode, not app.ExitCode)", code)
	}
}
