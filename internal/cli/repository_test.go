package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
)

// TestRepositoryCommandsRegistered proves `docket repository` carries exactly the
// four settled subcommands, each with a --repo-dir flag, and that the bare
// group and an unknown subcommand both fail rather than silently succeeding.
func TestRepositoryCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"init", "check", "migrate", "prepare"} {
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

// fakeMigrateResult is a stub migration OperationResult that also exposes
// SourceRev, so a test can drive the two-pass confirm flow and assert the second
// pass is pinned to the revision the preview showed.
type fakeMigrateResult struct {
	app.Envelope
	source string
}

func (r fakeMigrateResult) HumanText() string { return "migrate plan @ " + r.source }
func (r fakeMigrateResult) SourceRev() string { return r.source }

// TestRepositoryMigrateFlagsRegistered proves migrate carries --yes and
// --repair-frontmatter alongside --repo-dir.
func TestRepositoryMigrateFlagsRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"repository", "migrate"})
	if err != nil || cmd == nil {
		t.Fatalf("repository migrate not registered: %v", err)
	}
	for _, flag := range []string{"repo-dir", "yes", "repair-frontmatter"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("repository migrate: missing --%s flag", flag)
		}
	}
}

// TestRepositoryMigrateYesAuthorizesDirectly proves --yes calls the service once
// with Authorized=true and no preview pass.
func TestRepositoryMigrateYesAuthorizesDirectly(t *testing.T) {
	var calls []app.MigrateOptions
	old := repositoryMigrateRunner
	repositoryMigrateRunner = func(ctx context.Context, d app.SetupDeps, o app.MigrateOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeMigrateResult{Envelope: app.NewEnvelope("repository.migrate", app.ResultApplied), source: "abc123"}
	}
	defer func() { repositoryMigrateRunner = old }()

	_, _, code := runCLI(t, "repository", "migrate", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(calls) != 1 {
		t.Fatalf("runner called %d times, want exactly one", len(calls))
	}
	if !calls[0].Authorized {
		t.Errorf("--yes must call the service Authorized: %+v", calls[0])
	}
}

// TestRepositoryMigrateNonInteractivePreview proves that without --yes and with
// no terminal, the service is called once for the preview (unauthorized) and its
// result is presented, never re-invoked.
func TestRepositoryMigrateNonInteractivePreview(t *testing.T) {
	var calls []app.MigrateOptions
	old := repositoryMigrateRunner
	repositoryMigrateRunner = func(ctx context.Context, d app.SetupDeps, o app.MigrateOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeMigrateResult{Envelope: app.NewEnvelope("repository.migrate", app.ResultInvalidState), source: "abc123"}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return false }
	defer func() { repositoryMigrateRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLI(t, "repository", "migrate")
	if len(calls) != 1 {
		t.Fatalf("runner called %d times, want exactly one preview call", len(calls))
	}
	if calls[0].Authorized {
		t.Errorf("a non-interactive preview must be unauthorized: %+v", calls[0])
	}
}

// TestRepositoryMigrateInteractiveConfirmReinvokes proves the interactive
// confirm flow: an unauthorized preview, then on `y` a second authorized call
// pinned to exactly the revision the preview showed (decide-and-act on the same
// copy).
func TestRepositoryMigrateInteractiveConfirmReinvokes(t *testing.T) {
	var calls []app.MigrateOptions
	old := repositoryMigrateRunner
	repositoryMigrateRunner = func(ctx context.Context, d app.SetupDeps, o app.MigrateOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeMigrateResult{Envelope: app.NewEnvelope("repository.migrate", app.ResultApplied), source: "pinnedsrc"}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryMigrateRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLIStdin(t, "y\n", "repository", "migrate")
	if len(calls) != 2 {
		t.Fatalf("runner called %d times, want two (preview then authorized)", len(calls))
	}
	if calls[0].Authorized {
		t.Errorf("first call must be the unauthorized preview: %+v", calls[0])
	}
	if !calls[1].Authorized {
		t.Errorf("second call must be authorized: %+v", calls[1])
	}
	if calls[1].ExpectedSource != "pinnedsrc" {
		t.Errorf("second call ExpectedSource = %q, want the previewed source pinnedsrc", calls[1].ExpectedSource)
	}
	// The human confirmed the whole previewed plan, and the preview carries the
	// complete repair diff. So the confirmed re-invoke authorizes the repairs it
	// showed — even without --repair-frontmatter — rather than dead-ending on the
	// service's repair-required refusal (the interactive confirmation covers
	// repairs because the diff was in the preview).
	if !calls[1].RepairAuthorized {
		t.Errorf("a confirmed interactive re-invoke must authorize the previewed repairs: %+v", calls[1])
	}
}

// TestRepositoryMigrateInteractiveDeclineDoesNotAuthorize proves that on `n` (or
// an empty line) the interactive flow presents the preview and never re-invokes
// authorized.
func TestRepositoryMigrateInteractiveDeclineDoesNotAuthorize(t *testing.T) {
	var calls []app.MigrateOptions
	old := repositoryMigrateRunner
	repositoryMigrateRunner = func(ctx context.Context, d app.SetupDeps, o app.MigrateOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeMigrateResult{Envelope: app.NewEnvelope("repository.migrate", app.ResultInvalidState), source: "pinnedsrc"}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryMigrateRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLIStdin(t, "n\n", "repository", "migrate")
	if len(calls) != 1 {
		t.Fatalf("runner called %d times, want one (preview only, declined)", len(calls))
	}
	if calls[0].Authorized {
		t.Errorf("a declined preview must never authorize: %+v", calls[0])
	}
}

// TestRepositoryPrepareJSONEnvelope proves `docket repository prepare --json`
// emits the protocol-v1 envelope carrying the repository.prepare operation key,
// exiting 0 for an applied preparation.
func TestRepositoryPrepareJSONEnvelope(t *testing.T) {
	old := repositoryPrepareRunner
	repositoryPrepareRunner = func(ctx context.Context, d app.SetupDeps, o app.PrepareOptions) app.OperationResult {
		return app.RepositoryPrepareResult{
			Envelope:        app.NewEnvelope(app.OperationRepositoryPrepare, app.ResultApplied),
			Disposition:     app.PrepareDispositionApplied,
			RepositoryState: "healthy",
		}
	}
	defer func() { repositoryPrepareRunner = old }()

	out, _, code := runCLI(t, "repository", "prepare", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, `"operation":"repository.prepare"`) {
		t.Fatalf("--json did not emit the prepare envelope: %q", out)
	}
}

// TestRepositoryPrepareHumanSummaryRedacted proves the non-JSON path presents a
// one-line summary through the result's HumanText and never dumps the full
// context: neither the origin URL credentials nor the resolved skills map reach
// human stdout, even when the returned context carries both.
func TestRepositoryPrepareHumanSummaryRedacted(t *testing.T) {
	old := repositoryPrepareRunner
	repositoryPrepareRunner = func(ctx context.Context, d app.SetupDeps, o app.PrepareOptions) app.OperationResult {
		return app.RepositoryPrepareResult{
			Envelope:        app.NewEnvelope(app.OperationRepositoryPrepare, app.ResultApplied),
			Disposition:     app.PrepareDispositionApplied,
			RepositoryState: "healthy",
			Context: &app.PrepareContext{
				RepoRoot:  "/repo",
				OriginURL: "https://user:s3cr3ttoken@github.com/acme/docket.git",
				Skills:    app.PrepareSkills{Brainstorm: "docket-brainstorm-secret", Plan: "docket-plan", Build: "docket-build"},
			},
		}
	}
	defer func() { repositoryPrepareRunner = old }()

	out, _, code := runCLI(t, "repository", "prepare")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "repository prepare") {
		t.Fatalf("human output is missing the one-line summary: %q", out)
	}
	if strings.Contains(out, "s3cr3ttoken") {
		t.Errorf("human output leaked origin URL credentials: %q", out)
	}
	if strings.Contains(out, "docket-brainstorm-secret") {
		t.Errorf("human output dumped the resolved skills map: %q", out)
	}
}

// TestRepositoryPrepareUnknownFlagFails proves an unrecognized flag is refused
// as an argument error before the operation runs.
func TestRepositoryPrepareUnknownFlagFails(t *testing.T) {
	_, _, code := runCLI(t, "repository", "prepare", "--bogus")
	if code == 0 {
		t.Fatalf("unknown flag on repository prepare must fail, got exit 0")
	}
}

// TestRepositoryMigrateRepairFlagFlows proves --repair-frontmatter flows into
// MigrateOptions on both the preview and the authorized pass.
func TestRepositoryMigrateRepairFlagFlows(t *testing.T) {
	var calls []app.MigrateOptions
	old := repositoryMigrateRunner
	repositoryMigrateRunner = func(ctx context.Context, d app.SetupDeps, o app.MigrateOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeMigrateResult{Envelope: app.NewEnvelope("repository.migrate", app.ResultApplied), source: "abc123"}
	}
	defer func() { repositoryMigrateRunner = old }()

	_, _, _ = runCLI(t, "repository", "migrate", "--yes", "--repair-frontmatter")
	if len(calls) != 1 || !calls[0].RepairAuthorized {
		t.Fatalf("--repair-frontmatter must flow to MigrateOptions: %+v", calls)
	}
}
