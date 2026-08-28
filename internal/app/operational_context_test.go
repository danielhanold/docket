package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// These are the shared operational-repository gate tests (change 0363 Task 3).
// The gate lives inside loadOperationalContext — the one loader every ordinary
// command reaches through StatusReader.PinContext — and refuses exactly the
// `legacy` classification with change 0352's own typed finding. Every other
// classifier state the single-pin contract accepted before the gate existed is
// still admitted (the gate contracts nothing but legacy).

// legacyChangeRecord is one valid active change committed on the integration
// branch — the live planning surface that makes a metadata-less repository
// classify as legacy rather than fresh.
func legacyChangeRecord() map[string]string {
	return map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	}
}

// originRefs snapshots every ref on the bare origin, so a refused ordinary
// operation can be proven to have moved nothing.
func originRefs(t *testing.T, origin string) string {
	t.Helper()
	return runGit(t, origin, "for-each-ref")
}

// TestOperationalGateRefusesLegacy proves a legacy fixture makes an ordinary
// command's PinContext return the shared typed refusal, rendered by the status
// operation as the spec's one protocol document, and that a MUTATING ordinary
// operation refused by the same gate moves no ref on the origin.
func TestOperationalGateRefusesLegacy(t *testing.T) {
	repo := newLegacyRepo(t, legacyChangeRecord())
	client := newGitClient(t)
	reader := NewGitStatusReader(client)

	res := Status(context.Background(), reader, StatusOptions{RepoDir: repo.invocation})
	if res.Result != ResultInvalidState {
		t.Fatalf("result = %q, want %q (message %q)", res.Result, ResultInvalidState, res.Message)
	}
	if res.Operation != OperationStatus {
		t.Errorf("operation = %q, want %q (the envelope keeps the attempted operation)", res.Operation, OperationStatus)
	}
	if res.Reason != "legacy-repository" {
		t.Errorf("reason = %q, want legacy-repository", res.Reason)
	}
	if res.RepositoryState != string(reposetup.StateLegacy) {
		t.Errorf("repository_state = %q, want %q", res.RepositoryState, reposetup.StateLegacy)
	}
	if len(res.Findings) == 0 {
		t.Fatal("refusal carries no findings")
	}
	f := res.Findings[0]
	if f.Code != "legacy-repository" {
		t.Errorf("findings[0].code = %q, want legacy-repository", f.Code)
	}
	if f.Severity != string(reposetup.SeverityError) {
		t.Errorf("findings[0].severity = %q, want error", f.Severity)
	}
	if !strings.Contains(f.Remedy, "docket repository migrate") {
		t.Errorf("findings[0].remedy = %q, want it to name `docket repository migrate`", f.Remedy)
	}

	// A mutating ordinary operation inherits the same gate through PinContext
	// and performs no mutation: every origin ref is byte-identical after it.
	before := originRefs(t, repo.origin)
	node := planningDepsFor(t, cloneOrigin(t, repo.origin))
	cres := ChangeCreate(context.Background(), node.deps, node.dir, validChangeCreateRequest())
	if cres.Result != ResultInvalidState {
		t.Fatalf("change create result = %q, want %q", cres.Result, ResultInvalidState)
	}
	if len(cres.Findings) == 0 || cres.Findings[0].Code != "legacy-repository" {
		t.Fatalf("change create findings = %+v, want findings[0].code legacy-repository", cres.Findings)
	}
	if after := originRefs(t, repo.origin); after != before {
		t.Errorf("refused operation moved origin refs:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestOperationalGateFindingIsTheClassifierValue proves the refusal finding is
// the exact typed value `repository check` reports for the same fixture — the
// classifier is the single source, not a command-specific copy.
func TestOperationalGateFindingIsTheClassifierValue(t *testing.T) {
	repo := newLegacyRepo(t, legacyChangeRecord())
	client := newGitClient(t)

	res := Status(context.Background(), NewGitStatusReader(client), StatusOptions{RepoDir: repo.invocation})
	if res.Result != ResultInvalidState || len(res.Findings) == 0 {
		t.Fatalf("status refusal missing: result %q findings %+v", res.Result, res.Findings)
	}
	gateFinding := res.Findings[0]

	check := RunRepositoryCheck(context.Background(), SetupDeps{Git: client, RepoDir: repo.invocation})
	if check.RepositoryState != string(reposetup.StateLegacy) {
		t.Fatalf("repository check state = %q, want legacy", check.RepositoryState)
	}
	var checkFinding *reposetup.Finding
	for i := range check.Findings {
		if check.Findings[i].Code == "legacy-repository" {
			checkFinding = &check.Findings[i]
			break
		}
	}
	if checkFinding == nil {
		t.Fatalf("repository check reported no legacy-repository finding: %+v", check.Findings)
	}
	if gateFinding.Code != checkFinding.Code ||
		gateFinding.Severity != string(checkFinding.Severity) ||
		gateFinding.Message != checkFinding.Message ||
		gateFinding.Remedy != checkFinding.Remedy {
		t.Errorf("gate finding differs from the classifier value:\ngate:  %+v\ncheck: %+v", gateFinding, *checkFinding)
	}
}

// TestOperationalGatePassesHealthy proves a docket-topology fixture pins
// normally: integration resolved from configuration, the metadata revision
// pinned from the remote docket branch, and no refusal.
func TestOperationalGatePassesHealthy(t *testing.T) {
	repo := newWorkingRepo(t, map[string]string{
		"docs/changes/active/0002-beta.md": changeRecord(2, "beta", "Beta"),
	})
	reader := NewGitStatusReader(newGitClient(t))

	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext refused a docket-topology repository: %v", err)
	}
	if pin.IntegrationBranch != "main" {
		t.Errorf("integration branch = %q, want main (resolved from config)", pin.IntegrationBranch)
	}
	if want := originTip(t, repo.origin, "docket"); pin.MetadataRevision != want {
		t.Errorf("metadata revision = %q, want the remote docket tip %q", pin.MetadataRevision, want)
	}
	if pin.MetadataRevision == pin.IntegrationRevision {
		t.Fatal("fixture is not observably source-separated: docket and integration tips are equal")
	}

	res := Status(context.Background(), reader, StatusOptions{RepoDir: repo.invocation})
	if res.Result != ResultApplied {
		t.Fatalf("status result = %q, want applied (message %q)", res.Result, res.Message)
	}
	if res.Summary.ActiveChanges != 1 {
		t.Errorf("active changes = %d, want 1 (corpus read from the docket branch)", res.Summary.ActiveChanges)
	}
}

// TestFailClosedOrdering proves (a) an invalid configuration fails as invalid
// input BEFORE any topology classification — never the legacy remedy — and (b)
// the refusal predicate fires for exactly the legacy state, so unknown or
// conflicting classifications keep change 0352's own disposition and never
// collapse into legacy-repository.
func TestFailClosedOrdering(t *testing.T) {
	t.Run("invalid config precedes classification", func(t *testing.T) {
		repo := newLegacyRepo(t, legacyChangeRecord())
		// Corrupt the committed repository-layer configuration on the origin.
		writeRepoFile(t, repo.writer, ".docket.yml", "not: [valid\n")
		runGit(t, repo.writer, "add", ".docket.yml")
		runGit(t, repo.writer, "commit", "-q", "-m", "corrupt config")
		runGit(t, repo.writer, "push", "-q", "origin", "main")

		res := Status(context.Background(), NewGitStatusReader(newGitClient(t)), StatusOptions{RepoDir: repo.invocation})
		if res.Result != ResultInvalidInput {
			t.Fatalf("result = %q, want %q (invalid config fails closed first)", res.Result, ResultInvalidInput)
		}
		if res.Reason == "legacy-repository" || strings.Contains(res.Message, "repository migrate") {
			t.Errorf("invalid config collapsed into the legacy remedy: reason %q message %q", res.Reason, res.Message)
		}
	})

	t.Run("only legacy refuses", func(t *testing.T) {
		for _, state := range []reposetup.State{
			reposetup.StateFresh, reposetup.StateNeedsReview, reposetup.StateHealthy,
			reposetup.StatePartial, reposetup.StateConflict, reposetup.StateUnknown,
		} {
			if err := operationalRefusal(reposetup.Classification{State: state}, reposetup.Facts{}); err != nil {
				t.Errorf("state %q refused: %v (the gate must refuse only legacy)", state, err)
			}
		}
		err := operationalRefusal(reposetup.Classification{
			State: reposetup.StateLegacy, Reasons: []string{"legacy-live-surface"},
		}, reposetup.Facts{})
		if err == nil {
			t.Fatal("legacy state was not refused")
		}
		var notOp *errRepositoryNotOperational
		if !errors.As(err, &notOp) || notOp.State != reposetup.StateLegacy {
			t.Fatalf("legacy refusal is not the typed error: %v", err)
		}
	})
}
