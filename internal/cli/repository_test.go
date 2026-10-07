package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRepositoryCommandsRegistered proves `docket repository` carries exactly the
// settled subcommands, each with a --repo-dir flag, and that the bare
// group and an unknown subcommand both fail rather than silently succeeding.
func TestRepositoryCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"init", "check", "migrate", "prepare", "configure-tests", "configure-harnesses", "repair", "set-visibility"} {
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
	repositoryInitRunner = func(ctx context.Context, d app.SetupDeps, o app.InitOptions) app.OperationResult {
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

// fakeSyncResult is a stub sync OperationResult a stubbed sync-integration
// runner returns so a test can drive the presenter without a real repository.
type fakeSyncResult struct {
	app.Envelope
}

func (r fakeSyncResult) HumanText() string { return "sync-human" }

// TestRepositorySyncIntegrationDispatch proves `docket repository
// sync-integration --repo-dir <tmp>` resolves the invocation directory into
// SetupDeps.RepoDir, dispatches the sync runner with it, and presents the
// runner's result.
func TestRepositorySyncIntegrationDispatch(t *testing.T) {
	tmp := testsupport.TempDir(t)
	var gotRepoDir string
	var calls int
	old := repositorySyncIntegrationRunner
	repositorySyncIntegrationRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		calls++
		gotRepoDir = d.RepoDir
		return fakeSyncResult{Envelope: app.NewEnvelope("repository.sync-integration", app.ResultApplied)}
	}
	defer func() { repositorySyncIntegrationRunner = old }()

	out, _, code := runCLI(t, "repository", "sync-integration", "--repo-dir", tmp)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if calls != 1 {
		t.Fatalf("runner called %d times, want exactly one", calls)
	}
	if gotRepoDir != tmp {
		t.Errorf("runner RepoDir = %q, want the resolved --repo-dir %q", gotRepoDir, tmp)
	}
	if !strings.Contains(out, "sync-human") {
		t.Errorf("presenter did not print the runner result: %q", out)
	}
}

// TestRepositorySyncIntegrationJSONFlowsToPresenter proves --json selects the
// JSON transport carrying the repository.sync-integration operation key.
func TestRepositorySyncIntegrationJSONFlowsToPresenter(t *testing.T) {
	old := repositorySyncIntegrationRunner
	repositorySyncIntegrationRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
		return fakeSyncResult{Envelope: app.NewEnvelope("repository.sync-integration", app.ResultApplied)}
	}
	defer func() { repositorySyncIntegrationRunner = old }()

	out, _, code := runCLI(t, "repository", "sync-integration", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, `"operation":"repository.sync-integration"`) {
		t.Fatalf("--json did not flow to the presenter: %q", out)
	}
}

// TestRepositorySyncIntegrationAnnotation proves the subcommand carries the
// capability id repository.sync-integration with exactly the local-write
// effect, and a --repo-dir flag.
func TestRepositorySyncIntegrationAnnotation(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"repository", "sync-integration"})
	if err != nil || cmd == nil || cmd.Name() != "sync-integration" {
		t.Fatalf("repository sync-integration not registered: cmd=%v err=%v", cmd, err)
	}
	if got := cmd.Annotations[capAnnotationID]; got != "repository.sync-integration" {
		t.Errorf("capability id = %q, want repository.sync-integration", got)
	}
	if got := cmd.Annotations[capAnnotationEffects]; got != string(EffectLocalWrite) {
		t.Errorf("effects = %q, want exactly %q", got, EffectLocalWrite)
	}
	if cmd.Flags().Lookup("repo-dir") == nil {
		t.Errorf("repository sync-integration: missing --repo-dir flag")
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

// fakeRepairResult is a stub repair result whose confirmation state is set by the
// test, so the CLI's prompt decision is exercised without a repository.
type fakeRepairResult struct {
	app.Envelope
	source  string
	confirm bool
}

func (r fakeRepairResult) HumanText() string          { return "repair plan @ " + r.source }
func (r fakeRepairResult) SourceRev() string          { return r.source }
func (r fakeRepairResult) ConfirmationRequired() bool { return r.confirm }

// TestRepositoryRepairRegisteredWithCapability proves `repository repair` is a
// registered leaf carrying --repo-dir, --yes, and --pr-backlinks, the catalog id
// repository.repair, and exactly the external-write and metadata-write effects.
func TestRepositoryRepairRegisteredWithCapability(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"repository", "repair"})
	if err != nil || cmd == nil || cmd.Name() != "repair" {
		t.Fatalf("repository repair not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"repo-dir", "yes", "pr-backlinks"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("repository repair: missing --%s flag", flag)
		}
	}
	if cmd.Flags().Lookup("repair-frontmatter") != nil {
		t.Errorf("repository repair must not carry --repair-frontmatter")
	}
	if got := cmd.Annotations[capAnnotationID]; got != "repository.repair" {
		t.Errorf("capability id = %q, want repository.repair", got)
	}
	if got, want := cmd.Annotations[capAnnotationEffects], "external-write metadata-write"; got != want {
		t.Errorf("effects = %q, want exactly %q", got, want)
	}
}

// TestRepositoryRepairYesAuthorizesDirectly proves --yes calls the service once,
// authorized, with no preview pass.
func TestRepositoryRepairYesAuthorizesDirectly(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultApplied), source: "abc123"}
	}
	defer func() { repositoryRepairRunner = old }()

	_, _, code := runCLI(t, "repository", "repair", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(calls) != 1 || !calls[0].Authorized {
		t.Fatalf("--yes must call the service exactly once, authorized: %+v", calls)
	}
}

// TestRepositoryRepairNonInteractivePreview proves without --yes and without a
// terminal the service is called once, unauthorized, and never re-invoked.
func TestRepositoryRepairNonInteractivePreview(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultInvalidState), source: "abc123", confirm: true}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return false }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLI(t, "repository", "repair")
	if len(calls) != 1 || calls[0].Authorized {
		t.Fatalf("want exactly one unauthorized preview call: %+v", calls)
	}
}

// TestRepositoryRepairInteractiveConfirmReinvokes proves `y` re-invokes
// authorized, pinned to the previewed revision.
func TestRepositoryRepairInteractiveConfirmReinvokes(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultInvalidState), source: "pinnedtip", confirm: !o.Authorized}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLIStdin(t, "y\n", "repository", "repair")
	if len(calls) != 2 {
		t.Fatalf("runner called %d times, want preview then authorized", len(calls))
	}
	if calls[0].Authorized || !calls[1].Authorized || calls[1].ExpectedSource != "pinnedtip" {
		t.Errorf("calls = %+v, want unauthorized preview then authorized pinned to pinnedtip", calls)
	}
}

// TestRepositoryRepairGitHubOnlyWithPRBacklinks proves the GitHub client is
// built and wired only under --pr-backlinks, and the flag reaches the service.
func TestRepositoryRepairGitHubOnlyWithPRBacklinks(t *testing.T) {
	built := 0
	oldGH := repositoryRepairGitHub
	repositoryRepairGitHub = func() (app.RepairGitHub, error) { built++; return nil, nil }
	var seen []app.RepairOptions
	oldR := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		seen = append(seen, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultNoOp), source: "abc"}
	}
	defer func() { repositoryRepairGitHub = oldGH; repositoryRepairRunner = oldR }()

	runCLI(t, "repository", "repair", "--yes")
	if built != 0 || len(seen) != 1 || seen[0].PRBacklinks {
		t.Fatalf("plain repair built GitHub %d time(s) / opts %+v; want none", built, seen)
	}
	runCLI(t, "repository", "repair", "--pr-backlinks", "--yes")
	if built != 1 || len(seen) != 2 || !seen[1].PRBacklinks || !seen[1].Authorized {
		t.Fatalf("--pr-backlinks: built=%d opts=%+v", built, seen)
	}
}

// TestRepositoryRepairPRBacklinksInteractiveConfirmKeepsFlag proves the
// confirmed re-invocation of a --pr-backlinks preview still selects the
// PR-backlink repair, pinned to the previewed revision.
func TestRepositoryRepairPRBacklinksInteractiveConfirmKeepsFlag(t *testing.T) {
	var calls []app.RepairOptions
	oldGH := repositoryRepairGitHub
	repositoryRepairGitHub = func() (app.RepairGitHub, error) { return nil, nil }
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultInvalidState), source: "pinnedtip", confirm: !o.Authorized}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() {
		repositoryRepairRunner = old
		repositoryConfirmInteractive = oldI
		repositoryRepairGitHub = oldGH
	}()

	_, _, _ = runCLIStdin(t, "y\n", "repository", "repair", "--pr-backlinks")
	if len(calls) != 2 {
		t.Fatalf("runner called %d times, want preview then authorized", len(calls))
	}
	if !calls[0].PRBacklinks || calls[0].Authorized || !calls[1].PRBacklinks || !calls[1].Authorized || calls[1].ExpectedSource != "pinnedtip" {
		t.Errorf("calls = %+v, want a --pr-backlinks preview then an authorized --pr-backlinks run pinned to pinnedtip", calls)
	}
}

// TestRepositoryRepairInteractiveDeclineDoesNotAuthorize proves `n` never
// re-invokes authorized.
func TestRepositoryRepairInteractiveDeclineDoesNotAuthorize(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultInvalidState), source: "pinnedtip", confirm: true}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLIStdin(t, "n\n", "repository", "repair")
	if len(calls) != 1 || calls[0].Authorized {
		t.Fatalf("a declined preview must never authorize: %+v", calls)
	}
}

// TestRepositoryRepairInteractiveNoPromptWithoutConfirmation proves an
// interactive run whose preview is NOT a confirmation request (a no-op or a
// refusal) is presented directly, with no prompt and no re-invoke (Review Focus 4).
func TestRepositoryRepairInteractiveNoPromptWithoutConfirmation(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultNoOp), source: "tip", confirm: false}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	out, _, _ := runCLIStdin(t, "y\n", "repository", "repair")
	if len(calls) != 1 {
		t.Fatalf("runner called %d times, want one (no prompt, no re-invoke)", len(calls))
	}
	if strings.Contains(out, "repair? [y/N]") {
		t.Errorf("a no-op preview must not prompt; stdout:\n%s", out)
	}
}

// TestRepositoryConfigureTestsCommandFlagFlows proves --command reaches the app
// options as a non-nil pointer whenever the flag is PASSED, including the empty
// value (which the app must refuse, not mistake for "absent"), and stays nil
// when it is not passed.
func TestRepositoryConfigureTestsCommandFlagFlows(t *testing.T) {
	tmp := testsupport.TempDir(t)
	var got app.ConfigureTestsOptions
	old := repositoryConfigureTestsRunner
	repositoryConfigureTestsRunner = func(ctx context.Context, d app.SetupDeps, o app.ConfigureTestsOptions) app.OperationResult {
		got = o
		return fakeSyncResult{Envelope: app.NewEnvelope("repository.configure-tests", app.ResultNoOp)}
	}
	defer func() { repositoryConfigureTestsRunner = old }()

	if _, _, code := runCLI(t, "repository", "configure-tests", "--repo-dir", tmp, "--command", "sh ./test.sh"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got.Command == nil || *got.Command != "sh ./test.sh" {
		t.Fatalf("Command = %v, want pointer to %q", got.Command, "sh ./test.sh")
	}

	got = app.ConfigureTestsOptions{}
	if _, _, code := runCLI(t, "repository", "configure-tests", "--repo-dir", tmp, "--command", ""); code != 0 {
		t.Fatalf("exit = %d, want 0 (the stubbed runner decides)", code)
	}
	if got.Command == nil || *got.Command != "" {
		t.Fatalf("an explicit empty --command must arrive as a non-nil empty pointer, got %v", got.Command)
	}

	got = app.ConfigureTestsOptions{Command: new(string)}
	if _, _, code := runCLI(t, "repository", "configure-tests", "--repo-dir", tmp); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got.Command != nil {
		t.Fatalf("no --command must arrive as nil (run discovery), got %q", *got.Command)
	}
}

func TestRepositoryConfigureTestsHelpNamesCommandFlag(t *testing.T) {
	out, _, _ := runCLI(t, "repository", "configure-tests", "--help")
	if !strings.Contains(out, "--command") {
		t.Errorf("configure-tests --help must document --command:\n%s", out)
	}
}

// TestRepositoryHarnessesFlagAndChooserFlow proves configure-harnesses reads
// --harnesses by Changed (repeated and comma-separated values accumulate, an
// explicit empty value still arrives as Set) and supplies the picker only when
// no flag was given, --json is off, and the session is interactive.
func TestRepositoryHarnessesFlagAndChooserFlow(t *testing.T) {
	tmp := testsupport.TempDir(t)
	var got app.ConfigureHarnessesOptions
	oldRunner := repositoryConfigureHarnessesRunner
	repositoryConfigureHarnessesRunner = func(ctx context.Context, d app.SetupDeps, o app.ConfigureHarnessesOptions) app.OperationResult {
		got = o
		return fakeSyncResult{Envelope: app.NewEnvelope("repository.configure-harnesses", app.ResultNoOp)}
	}
	interactive := false
	oldInteractive := repositoryHarnessesInteractive
	repositoryHarnessesInteractive = func() bool { return interactive }
	defer func() {
		repositoryConfigureHarnessesRunner = oldRunner
		repositoryHarnessesInteractive = oldInteractive
	}()

	run := func(t *testing.T, args ...string) app.HarnessesOptions {
		t.Helper()
		got = app.ConfigureHarnessesOptions{}
		argv := append([]string{"repository", "configure-harnesses", "--repo-dir", tmp}, args...)
		if _, _, code := runCLI(t, argv...); code != 0 {
			t.Fatalf("exit = %d, want 0 (the stubbed runner decides)", code)
		}
		return got.Harnesses
	}

	t.Run("repeated and comma flags accumulate", func(t *testing.T) {
		interactive = false
		h := run(t, "--harnesses", "claude,cursor", "--harnesses", "codex")
		if !h.Set || strings.Join(h.Tokens, "|") != "claude|cursor|codex" || h.Chooser != nil {
			t.Fatalf("got Set=%v Tokens=%v chooser=%v", h.Set, h.Tokens, h.Chooser != nil)
		}
	})
	t.Run("explicit empty value is set", func(t *testing.T) {
		interactive = false
		h := run(t, "--harnesses", "")
		if !h.Set || h.Tokens == nil {
			t.Fatalf("an explicit empty --harnesses must arrive Set with non-nil tokens, got Set=%v Tokens=%#v", h.Set, h.Tokens)
		}
	})
	t.Run("non-interactive without flag", func(t *testing.T) {
		interactive = false
		h := run(t)
		if h.Set || h.Chooser != nil {
			t.Fatalf("got Set=%v chooser=%v, want unset and no chooser", h.Set, h.Chooser != nil)
		}
	})
	t.Run("interactive supplies chooser", func(t *testing.T) {
		interactive = true
		if h := run(t); h.Set || h.Chooser == nil {
			t.Fatalf("got Set=%v chooser=%v, want a chooser", h.Set, h.Chooser != nil)
		}
	})
	t.Run("interactive with json has no chooser", func(t *testing.T) {
		interactive = true
		if h := run(t, "--json"); h.Chooser != nil {
			t.Fatal("--json must never show the picker")
		}
	})
	t.Run("interactive with flag has no chooser", func(t *testing.T) {
		interactive = true
		if h := run(t, "--harnesses", "none"); !h.Set || h.Chooser != nil {
			t.Fatalf("got Set=%v chooser=%v, want set and no chooser", h.Set, h.Chooser != nil)
		}
	})
}

// TestRepositoryInitHarnessesFlagFlows proves init passes --harnesses through to
// the app options.
func TestRepositoryInitHarnessesFlagFlows(t *testing.T) {
	tmp := testsupport.TempDir(t)
	var got app.InitOptions
	old := repositoryInitRunner
	repositoryInitRunner = func(ctx context.Context, d app.SetupDeps, o app.InitOptions) app.OperationResult {
		got = o
		return fakeInitResult{Envelope: app.NewEnvelope("repository.init", app.ResultApplied)}
	}
	defer func() { repositoryInitRunner = old }()

	if _, _, code := runCLI(t, "repository", "init", "--repo-dir", tmp, "--harnesses", "claude", "--harnesses", "cursor"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !got.Harnesses.Set || strings.Join(got.Harnesses.Tokens, "|") != "claude|cursor" {
		t.Fatalf("Harnesses = %+v, want Set with [claude cursor]", got.Harnesses)
	}
}

func TestRepositoryHarnessesHelp(t *testing.T) {
	for _, sub := range []string{"init", "configure-harnesses"} {
		out, _, _ := runCLI(t, "repository", sub, "--help")
		if !strings.Contains(out, "--harnesses") || !strings.Contains(out, "none") {
			t.Errorf("%s --help must document --harnesses and none:\n%s", sub, out)
		}
	}
	if !assetIndependent["repository configure-harnesses"] {
		t.Error(`assetIndependent["repository configure-harnesses"] must be true`)
	}
}

// TestRepositorySetVisibilityRegisteredWithCapability proves `repository
// set-visibility` is a registered leaf taking exactly one argument, carrying its
// flags, the catalog id repository.set-visibility, exactly the local-write,
// metadata-write, and external-write effects, and an asset-independent entry.
func TestRepositorySetVisibilityRegisteredWithCapability(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"repository", "set-visibility"})
	if err != nil || cmd == nil || cmd.Name() != "set-visibility" {
		t.Fatalf("repository set-visibility not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"repo-dir", "yes", "metadata-remote", "delete-shared-branch", "remove-shared-files"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("repository set-visibility: missing --%s flag", flag)
		}
	}
	if got := cmd.Annotations[capAnnotationID]; got != "repository.set-visibility" {
		t.Errorf("capability id = %q, want repository.set-visibility", got)
	}
	if got, want := cmd.Annotations[capAnnotationEffects], "external-write local-write metadata-write"; got != want {
		t.Errorf("effects = %q, want exactly %q", got, want)
	}
	if !assetIndependent["repository set-visibility"] {
		t.Errorf("repository set-visibility must be asset-independent")
	}
	if cmd.Args == nil || cmd.Args(cmd, nil) == nil || cmd.Args(cmd, []string{"a", "b"}) == nil || cmd.Args(cmd, []string{"private"}) != nil {
		t.Errorf("repository set-visibility must take exactly one argument")
	}
}

// fakeVisibilityResult is a stub set-visibility result.
type fakeVisibilityResult struct {
	app.Envelope
	source  string
	confirm bool
}

func (r fakeVisibilityResult) HumanText() string          { return "visibility plan @ " + r.source }
func (r fakeVisibilityResult) SourceRev() string          { return r.source }
func (r fakeVisibilityResult) ConfirmationRequired() bool { return r.confirm }

// stubSetVisibility replaces the runner and the GitHub constructor for one test.
func stubSetVisibility(t *testing.T, confirm func(o app.SetVisibilityOptions) bool) *[]app.SetVisibilityOptions {
	t.Helper()
	var calls []app.SetVisibilityOptions
	old := repositorySetVisibilityRunner
	repositorySetVisibilityRunner = func(ctx context.Context, d app.SetupDeps, o app.SetVisibilityOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeVisibilityResult{Envelope: app.NewEnvelope("repository.set-visibility", app.ResultInvalidState), source: "pin", confirm: confirm(o)}
	}
	oldGH := repositorySetVisibilityGitHub
	repositorySetVisibilityGitHub = func() (app.RepairGitHub, error) { return nil, errors.New("no gh") }
	t.Cleanup(func() { repositorySetVisibilityRunner = old; repositorySetVisibilityGitHub = oldGH })
	return &calls
}

// TestRepositorySetVisibilityFlagsReachOptions proves the argument and every
// flag reach the service, and --yes authorizes directly with no preview pass.
func TestRepositorySetVisibilityFlagsReachOptions(t *testing.T) {
	calls := stubSetVisibility(t, func(app.SetVisibilityOptions) bool { return false })
	_, _, _ = runCLI(t, "repository", "set-visibility", "private", "--yes",
		"--metadata-remote", "/tmp/backup.git", "--delete-shared-branch", "--remove-shared-files")
	if len(*calls) != 1 {
		t.Fatalf("runner called %d times, want once", len(*calls))
	}
	got := (*calls)[0]
	want := app.SetVisibilityOptions{Target: "private", Authorized: true, MetadataRemote: "/tmp/backup.git", DeleteSharedBranch: true, RemoveSharedFiles: true}
	if got != want {
		t.Errorf("options = %+v, want %+v", got, want)
	}
}

// TestRepositorySetVisibilityInteractiveConfirmReinvokes proves `y` re-invokes
// authorized, pinned to the previewed state, keeping the target and flags.
func TestRepositorySetVisibilityInteractiveConfirmReinvokes(t *testing.T) {
	calls := stubSetVisibility(t, func(o app.SetVisibilityOptions) bool { return !o.Authorized })
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLIStdin(t, "y\n", "repository", "set-visibility", "private", "--remove-shared-files")
	if len(*calls) != 2 {
		t.Fatalf("runner called %d times, want preview then authorized", len(*calls))
	}
	p, a := (*calls)[0], (*calls)[1]
	if p.Authorized || !a.Authorized || a.ExpectedSource != "pin" || a.Target != "private" || !a.RemoveSharedFiles {
		t.Errorf("calls = %+v, want an unauthorized preview then an authorized run pinned to pin with the same flags", *calls)
	}
}

// TestRepositorySetVisibilityNonInteractivePreview proves without --yes and
// without a terminal the service is called once, unauthorized.
func TestRepositorySetVisibilityNonInteractivePreview(t *testing.T) {
	calls := stubSetVisibility(t, func(app.SetVisibilityOptions) bool { return true })
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return false }
	defer func() { repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLI(t, "repository", "set-visibility", "shared")
	if len(*calls) != 1 || (*calls)[0].Authorized || (*calls)[0].Target != "shared" {
		t.Fatalf("want exactly one unauthorized shared preview: %+v", *calls)
	}
}
