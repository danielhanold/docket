package app

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// preparableFacts returns a healthy-topology, attached, at-tip facts value: the
// remote metadata branch is a published parentless root, no live surface remains,
// and the local .docket worktree is registered and clean at the remote tip. A test
// perturbs one field (and passes the matching sync relationship) to exercise a
// single disposition row in isolation.
func preparableFacts() reposetup.Facts {
	return reposetup.Facts{
		RemoteConfigured:    reposetup.PresencePresent,
		RemoteDefaultBranch: reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "d0"},
		RemoteIntegration:   reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "i0"},
		RemoteMetadata:      reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "m9"},
		MetadataRoot:        reposetup.RootParentless,
		LiveSurface:         reposetup.PresenceAbsent,
		LocalMetadata:       reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "m9"},
		DocketWorktree: reposetup.WorktreeFact{
			Presence:     reposetup.PresencePresent,
			Registered:   reposetup.PresencePresent,
			Clean:        reposetup.PresencePresent,
			Synchronized: reposetup.PresencePresent,
		},
	}
}

// findingCode returns the single verdict finding code, failing the test when the
// verdict carries no finding.
func findingCode(t *testing.T, v prepareVerdict) string {
	t.Helper()
	if v.finding == nil {
		t.Fatalf("verdict %q carries no finding", v.disposition)
	}
	return v.finding.Code
}

// TestRepositoryPrepareFreshRefusesWithInitRemedy — a fresh repository refuses
// with the exact `docket repository init` remedy.
func TestRepositoryPrepareFreshRefusesWithInitRemedy(t *testing.T) {
	f := preparableFacts()
	f.RemoteMetadata = reposetup.BranchFact{Presence: reposetup.PresenceAbsent}
	f.LiveSurface = reposetup.PresenceAbsent

	v := prepareRoute(f, prepareSyncUnknown)
	if v.disposition != PrepareDispositionRefused {
		t.Fatalf("disposition = %q, want refused", v.disposition)
	}
	if v.state != reposetup.StateFresh {
		t.Errorf("state = %q, want fresh", v.state)
	}
	if code := findingCode(t, v); code != "repository-fresh" {
		t.Errorf("finding code = %q, want repository-fresh", code)
	}
	if !strings.Contains(v.finding.Remedy, "docket repository init") {
		t.Errorf("remedy %q must contain exactly \"docket repository init\"", v.finding.Remedy)
	}
	if v.action != prepareActionNone {
		t.Errorf("a refusal must plan no action, got %v", v.action)
	}
}

// TestRepositoryPrepareLegacyRefusesWithMigrateRemedy — a legacy live surface
// refuses with the exact `docket repository migrate` remedy.
func TestRepositoryPrepareLegacyRefusesWithMigrateRemedy(t *testing.T) {
	f := preparableFacts()
	f.RemoteMetadata = reposetup.BranchFact{Presence: reposetup.PresenceAbsent}
	f.LiveSurface = reposetup.PresencePresent

	v := prepareRoute(f, prepareSyncUnknown)
	if v.disposition != PrepareDispositionRefused || v.state != reposetup.StateLegacy {
		t.Fatalf("verdict = %q/%q, want refused/legacy", v.disposition, v.state)
	}
	if code := findingCode(t, v); code != "repository-legacy" {
		t.Errorf("finding code = %q, want repository-legacy", code)
	}
	if !strings.Contains(v.finding.Remedy, "docket repository migrate") {
		t.Errorf("remedy %q must contain exactly \"docket repository migrate\"", v.finding.Remedy)
	}
}

// TestRepositoryPrepareHealthyMissingWorktreeAttaches — a healthy remote with no
// local .docket worktree applies, recording the attach against the pinned remote
// metadata revision.
func TestRepositoryPrepareHealthyMissingWorktreeAttaches(t *testing.T) {
	f := preparableFacts()
	f.DocketWorktree = reposetup.WorktreeFact{Presence: reposetup.PresenceAbsent}
	f.LocalMetadata = reposetup.BranchFact{Presence: reposetup.PresenceAbsent}

	v := prepareRoute(f, prepareSyncUnknown)
	if v.disposition != PrepareDispositionApplied {
		t.Fatalf("disposition = %q, want applied", v.disposition)
	}
	if v.action != prepareActionAttach {
		t.Errorf("action = %v, want attach", v.action)
	}
	if v.targetRev != "m9" {
		t.Errorf("targetRev = %q, want the pinned remote metadata tip m9", v.targetRev)
	}
}

// TestRepositoryPrepareCleanBehindFastForwards — a clean, strictly-behind worktree
// applies, fast-forwarding to the pinned remote metadata revision.
func TestRepositoryPrepareCleanBehindFastForwards(t *testing.T) {
	f := preparableFacts()
	f.LocalMetadata.Tip = "m1" // behind m9
	f.DocketWorktree.Synchronized = reposetup.PresenceAbsent

	v := prepareRoute(f, prepareSyncBehind)
	if v.disposition != PrepareDispositionApplied {
		t.Fatalf("disposition = %q, want applied", v.disposition)
	}
	if v.action != prepareActionFastForward {
		t.Errorf("action = %v, want fast-forward", v.action)
	}
	if v.targetRev != "m9" {
		t.Errorf("targetRev = %q, want the pinned remote metadata tip m9", v.targetRev)
	}
}

// TestRepositoryPrepareCleanCurrentIsNoOp — a worktree already at the pinned
// revision is a no-op with no planned effect.
func TestRepositoryPrepareCleanCurrentIsNoOp(t *testing.T) {
	v := prepareRoute(preparableFacts(), prepareSyncCurrent)
	if v.disposition != PrepareDispositionNoOp {
		t.Fatalf("disposition = %q, want no-op", v.disposition)
	}
	if v.action != prepareActionNone {
		t.Errorf("action = %v, want none", v.action)
	}
	if v.state != reposetup.StateHealthy {
		t.Errorf("state = %q, want healthy", v.state)
	}
}

// TestRepositoryPrepareDirtyRefuses — a dirty .docket worktree refuses without a
// touch.
func TestRepositoryPrepareDirtyRefuses(t *testing.T) {
	f := preparableFacts()
	f.DocketWorktree.Clean = reposetup.PresenceAbsent

	v := prepareRoute(f, prepareSyncCurrent)
	if v.disposition != PrepareDispositionRefused || v.action != prepareActionNone {
		t.Fatalf("verdict = %q/%v, want refused/none", v.disposition, v.action)
	}
	if code := findingCode(t, v); code != "metadata-worktree-dirty" {
		t.Errorf("finding code = %q, want metadata-worktree-dirty", code)
	}
}

// TestRepositoryPrepareAheadRefuses — a local branch ahead of the remote refuses;
// prepare never rewinds or force-updates.
func TestRepositoryPrepareAheadRefuses(t *testing.T) {
	f := preparableFacts()
	f.LocalMetadata.Tip = "mA"
	f.DocketWorktree.Synchronized = reposetup.PresenceAbsent

	v := prepareRoute(f, prepareSyncAhead)
	if v.disposition != PrepareDispositionRefused || v.action != prepareActionNone {
		t.Fatalf("verdict = %q/%v, want refused/none", v.disposition, v.action)
	}
	if code := findingCode(t, v); code != "local-metadata-ahead" {
		t.Errorf("finding code = %q, want local-metadata-ahead", code)
	}
}

// TestRepositoryPrepareDivergedRefuses — a diverged local branch refuses.
func TestRepositoryPrepareDivergedRefuses(t *testing.T) {
	f := preparableFacts()
	f.LocalMetadata.Tip = "mD"
	f.DocketWorktree.Synchronized = reposetup.PresenceAbsent

	v := prepareRoute(f, prepareSyncDiverged)
	if v.disposition != PrepareDispositionRefused {
		t.Fatalf("disposition = %q, want refused", v.disposition)
	}
	if code := findingCode(t, v); code != "local-metadata-diverged" {
		t.Errorf("finding code = %q, want local-metadata-diverged", code)
	}
}

// TestRepositoryPrepareForeignRefuses — a foreign .docket directory refuses before
// any topology decision.
func TestRepositoryPrepareForeignRefuses(t *testing.T) {
	f := preparableFacts()
	f.DocketWorktree.Foreign = true

	v := prepareRoute(f, prepareSyncCurrent)
	if v.disposition != PrepareDispositionRefused {
		t.Fatalf("disposition = %q, want refused", v.disposition)
	}
	if code := findingCode(t, v); code != "docket-dir-foreign" {
		t.Errorf("finding code = %q, want docket-dir-foreign", code)
	}
}

// TestRepositoryPrepareAmbiguousRegistrationRefuses — a present-but-unregistered
// .docket worktree refuses with its own finding code.
func TestRepositoryPrepareAmbiguousRegistrationRefuses(t *testing.T) {
	f := preparableFacts()
	f.DocketWorktree.Registered = reposetup.PresenceUnknown

	v := prepareRoute(f, prepareSyncCurrent)
	if v.disposition != PrepareDispositionRefused {
		t.Fatalf("disposition = %q, want refused", v.disposition)
	}
	if code := findingCode(t, v); code != "docket-worktree-ambiguous-registration" {
		t.Errorf("finding code = %q, want docket-worktree-ambiguous-registration", code)
	}
}

// TestRepositoryPrepareProbeUnknownRefuses — a healthy topology whose local sync
// relationship could not be proven refuses (never guesses a fast-forward).
func TestRepositoryPrepareProbeUnknownRefuses(t *testing.T) {
	v := prepareRoute(preparableFacts(), prepareSyncUnknown)
	if v.disposition != PrepareDispositionRefused {
		t.Fatalf("disposition = %q, want refused", v.disposition)
	}
	if v.action != prepareActionNone {
		t.Errorf("action = %v, want none", v.action)
	}
	if code := findingCode(t, v); code != "prepare-local-state-unknown" {
		t.Errorf("finding code = %q, want prepare-local-state-unknown", code)
	}
}

// TestRepositoryPrepareProbeErrorIsNotAbsence — an errored required topology probe
// (mapped to Unknown by the gatherer) yields an error disposition and never an
// attach. The fresh-looking absent worktree must not be read as attachable.
func TestRepositoryPrepareProbeErrorIsNotAbsence(t *testing.T) {
	root := setupProberRepoDir(t, "integration_branch: main\n")
	p := &fakeSetupProber{
		repo:       gitcli.Repository{PrimaryWorktree: root},
		defaultRef: gitcli.RefName("refs/heads/main"),
		probeErr:   errors.New("boom: ls-remote failed"), // metadata presence -> Unknown
	}
	f, _, err := gatherSetupFacts(context.Background(), p, root, true)
	if err != nil {
		t.Fatalf("gatherSetupFacts: %v", err)
	}
	if f.RemoteMetadata.Presence != reposetup.PresenceUnknown {
		t.Fatalf("RemoteMetadata.Presence = %v, want Unknown", f.RemoteMetadata.Presence)
	}

	v := prepareRoute(f, prepareSyncUnknown)
	if v.disposition != PrepareDispositionError {
		t.Errorf("disposition = %q, want error (an errored probe is never a clean absence)", v.disposition)
	}
	if v.action == prepareActionAttach {
		t.Errorf("an errored metadata probe must never plan an attach")
	}
	if code := findingCode(t, v); code != "prepare-topology-unresolved" {
		t.Errorf("finding code = %q, want prepare-topology-unresolved", code)
	}
}

// countingSetupProber records the gatherRepoFacts probe calls so a test can prove
// that an invalid configuration short-circuits BEFORE any topology/sync probe.
type countingSetupProber struct {
	repo       gitcli.Repository
	defaultRef gitcli.RefName
	fetches    int
	probes     int
	changed    int
	worktrees  int
}

func (p *countingSetupProber) Discover(ctx context.Context, opts gitcli.DiscoverOptions) (gitcli.Repository, error) {
	return p.repo, nil
}
func (p *countingSetupProber) RemoteDefaultBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName) (gitcli.RefName, error) {
	return p.defaultRef, nil
}
func (p *countingSetupProber) FetchBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error) {
	p.fetches++
	return gitcli.Revision{}, nil
}
func (p *countingSetupProber) ProbeRemoteBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName) (gitcli.RemoteRef, error) {
	p.probes++
	return gitcli.RemoteRef{}, nil
}
func (p *countingSetupProber) OpenObjectSource(ctx context.Context, repo gitcli.Repository, rev gitcli.Revision) (gitcli.ObjectSource, error) {
	return nil, errors.New("unreachable")
}
func (p *countingSetupProber) ChangedPaths(ctx context.Context, dir string) ([]gitcli.PathChange, error) {
	p.changed++
	return nil, nil
}
func (p *countingSetupProber) ResolveRef(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName) (gitcli.ObjectID, error) {
	return "", nil
}
func (p *countingSetupProber) ListWorktrees(ctx context.Context, repo gitcli.Repository) ([]gitcli.WorktreeInfo, error) {
	p.worktrees++
	return nil, nil
}

// TestRepositoryPrepareInvalidConfigRefusesBeforeSync — an invalid configuration
// refuses, and no synchronization/topology probe is reached: the gather fails at
// config resolution before gatherRepoFacts runs any probe.
func TestRepositoryPrepareInvalidConfigRefusesBeforeSync(t *testing.T) {
	// A malformed configuration (integration_branch typed as a sequence, not a
	// scalar) fails configuration resolution closed.
	root := setupProberRepoDir(t, "integration_branch:\n  - a\n  - b\n")
	p := &countingSetupProber{
		repo:       gitcli.Repository{PrimaryWorktree: root},
		defaultRef: gitcli.RefName("refs/heads/main"),
	}
	_, _, err := gatherSetupFacts(context.Background(), p, root, true)
	if err == nil {
		t.Fatal("invalid configuration must fail the gather")
	}
	if p.fetches != 0 || p.probes != 0 || p.changed != 0 || p.worktrees != 0 {
		t.Errorf("no topology/sync probe may run on an invalid config: fetches=%d probes=%d changed=%d worktrees=%d",
			p.fetches, p.probes, p.changed, p.worktrees)
	}

	res := prepareGatherFailure(err)
	if res.Disposition != PrepareDispositionRefused {
		t.Errorf("disposition = %q, want refused", res.Disposition)
	}
	if res.Result != ResultUnsupportedConfig {
		t.Errorf("result = %q, want unsupported-config", res.Result)
	}
}

// TestRepositoryPrepareContextFieldsTyped — the applied result's context is a
// closed typed structure: its JSON carries no DOCKET_ key and no flat string map,
// and it surfaces the resolved non-default configuration so the wiring is visible.
func TestRepositoryPrepareContextFieldsTyped(t *testing.T) {
	cfg := config.Effective{}
	cfg.IntegrationBranch.Value = "develop"
	cfg.ChangesDir.Value = "planning/changes"
	cfg.ADRsDir.Value = "planning/adrs"
	cfg.ResultsDir.Value = "planning/results"
	cfg.Finalize.Gate.Value = "local"
	cfg.Finalize.TestCommand.Value = "go run ./cmd/docket development test"
	cfg.Finalize.RequirePRApproval.Value = true

	sc := setupContext{
		cfg:               cfg,
		repo:              gitcli.Repository{PrimaryWorktree: "/repo"},
		defaultBranch:     "develop",
		integrationBranch: "develop",
	}
	f := preparableFacts()

	pc := buildPrepareContext(cfg, sc, f, "git@github.com:acme/widget.git")
	res := RepositoryPrepareResult{
		Envelope:        NewEnvelope(OperationRepositoryPrepare, ResultApplied),
		Disposition:     PrepareDispositionApplied,
		RepositoryState: string(reposetup.StateHealthy),
		Context:         pc,
	}

	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "DOCKET_") {
		t.Errorf("result JSON must carry no DOCKET_ key: %s", raw)
	}

	// The context decodes into the closed typed shape (never a generic map), with
	// the resolved non-default values wired through.
	var decoded struct {
		Context struct {
			RepoRoot                  string `json:"repo_root"`
			OriginURL                 string `json:"origin_url"`
			IntegrationBranch         string `json:"integration_branch"`
			IntegrationBranchRevision string `json:"integration_branch_revision"`
			MetadataBranch            string `json:"metadata_branch"`
			MetadataBranchRevision    string `json:"metadata_branch_revision"`
			MetadataWorktreePath      string `json:"metadata_worktree_path"`
			ChangesDir                string `json:"changes_dir"`
			AdrsDir                   string `json:"adrs_dir"`
			ResultsDir                string `json:"results_dir"`
			Finalize                  struct {
				Gate              string `json:"gate"`
				TestCommand       string `json:"test_command"`
				RequirePRApproval bool   `json:"require_pr_approval"`
			} `json:"finalize"`
		} `json:"context"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	c := decoded.Context
	if c.IntegrationBranch != "develop" {
		t.Errorf("integration_branch = %q, want the resolved non-default develop", c.IntegrationBranch)
	}
	if c.IntegrationBranchRevision != "i0" {
		t.Errorf("integration_branch_revision = %q, want the pinned i0", c.IntegrationBranchRevision)
	}
	if c.MetadataBranch != reposetup.MetadataBranchName {
		t.Errorf("metadata_branch = %q, want %q", c.MetadataBranch, reposetup.MetadataBranchName)
	}
	if c.MetadataBranchRevision != "m9" {
		t.Errorf("metadata_branch_revision = %q, want the pinned m9", c.MetadataBranchRevision)
	}
	if c.MetadataWorktreePath != filepath.Join("/repo", docketWorktreeName) {
		t.Errorf("metadata_worktree_path = %q, want the fixed .docket path", c.MetadataWorktreePath)
	}
	if c.ChangesDir != "planning/changes" || c.AdrsDir != "planning/adrs" || c.ResultsDir != "planning/results" {
		t.Errorf("resolved dirs not wired through: %+v", c)
	}
	if c.Finalize.TestCommand != "go run ./cmd/docket development test" || c.Finalize.Gate != "local" || !c.Finalize.RequirePRApproval {
		t.Errorf("finalize not mirrored from config: %+v", c.Finalize)
	}
	if c.OriginURL != "git@github.com:acme/widget.git" {
		t.Errorf("origin_url = %q, want the resolved origin", c.OriginURL)
	}
	if c.RepoRoot != "/repo" {
		t.Errorf("repo_root = %q, want /repo", c.RepoRoot)
	}
}

// TestRepositoryPrepareResultOmitsContextOnRefusal — a refusal carries the
// diagnosis finding and no context.
func TestRepositoryPrepareResultOmitsContextOnRefusal(t *testing.T) {
	f := preparableFacts()
	f.RemoteMetadata = reposetup.BranchFact{Presence: reposetup.PresenceAbsent}
	v := prepareRoute(f, prepareSyncUnknown)
	res := prepareDiagnosisResult(ResultInvalidState, PrepareDispositionRefused, v, nil)
	if res.Context != nil {
		t.Errorf("a refusal must carry no context, got %+v", res.Context)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("a refusal must carry exactly one finding, got %d", len(res.Findings))
	}
}
