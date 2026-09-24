package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
	"time"
)

// --- fakes / fixtures ------------------------------------------------------

const (
	prHead      = "1111111111111111111111111111111111111111"
	prOtherHead = "2222222222222222222222222222222222222222"
)

// fakeGitHub is a scriptable GitHubService: it records DiscoverRepository and
// EnsurePullRequest calls separately, so a test can prove the app layer NEVER
// reached EnsurePullRequest on a broken identity conjunct.
type fakeGitHub struct {
	repo      githubcli.Repository
	repoErr   error
	ensureRes githubcli.EnsureResult
	ensureErr error
	probePRs  []githubcli.PullRequest
	probeErr  error

	discoverCalls int
	ensureCalls   []githubcli.EnsurePullRequestRequest
	probeCalls    []string // head branch of each FindOpenPullRequestsByHead call
}

func (f *fakeGitHub) DiscoverRepository(_ context.Context, _ string) (githubcli.Repository, error) {
	f.discoverCalls++
	return f.repo, f.repoErr
}

func (f *fakeGitHub) EnsurePullRequest(_ context.Context, req githubcli.EnsurePullRequestRequest) (githubcli.EnsureResult, error) {
	f.ensureCalls = append(f.ensureCalls, req)
	return f.ensureRes, f.ensureErr
}

func (f *fakeGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, headBranch string) ([]githubcli.PullRequest, error) {
	f.probeCalls = append(f.probeCalls, headBranch)
	return f.probePRs, f.probeErr
}

// prEvidenceBytes renders the canonical build-evidence block certifying head.
func prEvidenceBytes(t *testing.T, head string) []byte {
	t.Helper()
	rec, err := evidence.NewRecord("go test ./...", head, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	return []byte(evidence.Render(rec))
}

// prSkippedEvidenceBytes renders a truthful skipped (build-gate-off) evidence
// block certifying head — the record a build.gate: off repository publishes in
// place of a green run. Shared by the four green-or-skipped acceptance tests.
func prSkippedEvidenceBytes(t *testing.T, head string) []byte {
	t.Helper()
	rec, err := evidence.NewSkippedRecord(head, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewSkippedRecord: %v", err)
	}
	return []byte(evidence.Render(rec))
}

// prReader builds a fake reader over a single in-progress change 7 (slug widget).
func prReader(t *testing.T) *fakeReader {
	t.Helper()
	return &fakeReader{
		pin:    mainPin(t),
		corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")},
	}
}

// prMatchPR is the verified PR snapshot the fake adapter returns on the happy path.
func prMatchPR(body string) githubcli.PullRequest {
	return githubcli.PullRequest{
		Number:     42,
		URL:        "https://github.com/acme/widget/pull/42",
		State:      githubcli.StateOpen,
		HeadBranch: "feat/widget",
		HeadCommit: prHead,
		BaseBranch: "main",
		Title:      "Add widget",
		Body:       body,
	}
}

func prRepo() githubcli.Repository {
	return githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widget"}
}

// readyService returns a fake workspace service reporting a ready workspace at
// the given local head.
func readyService(head string) *fakeWorkspaceService {
	return &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
	}
}

// --- (1) agreement checks: each conjunct broken ⇒ typed refusal, gh untouched ---

// --- (2) body assembly: prose preserved, evidence replaced, backlink once ---

// --- (3) dispositions surface verbatim; PR snapshot round-trips ------------

// --- (4) redaction: no body bytes in the result JSON or human text ---------

// prSnapshotChange builds the snapshot the operation reads and returns change id.
func prSnapshotChange(t *testing.T, reader *fakeReader, id int) domain.Change {
	t.Helper()
	inputs, _ := parseCorpus(reader.corpus)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: reader.pin.Config.Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		t.Fatalf("change %d not found in snapshot (outcome %v)", id, out)
	}
	return c
}

// TestPRPublishAcceptsSkippedEvidenceAtExactHead: a build.gate: off repository's
// truthful skipped evidence certifying the exact feature head passes PRPublish's
// evidence conjunct — the operation proceeds PAST it (VerdictSkipped is accepted
// exactly as VerdictVerified). Any later refusal is not the evidence conjunct;
// reverting the green-or-skipped acceptance would refuse here with
// ReasonPREvidenceUnverified, so this pins the verify-site change. (PR-body
// weaving of a skipped block is a separate concern: evidence.Upsert is green-only
// today, so the operation still fails later at body assembly — see the change
// notes.)
func TestPRPublishAcceptsSkippedEvidenceAtExactHead(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	reader := prReader(t)
	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	deps := workspaceDepsFor(t, reader)
	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prSkippedEvidenceBytes(t, prHead)})
	if res.Reason == ReasonPREvidenceUnverified {
		t.Fatalf("skipped evidence at the exact head was refused at the evidence conjunct: %q", res.Message)
	}
}

// TestPRFenceRefusalMessageIsReasonAware pins the review fix (change 0441): the
// human message must be accurate per fence reason. A run-completed fence is a
// SUCCESSFUL closeout, so its message must NOT claim cancellation; a cancelled or
// superseded fence keeps the existing cancelled-or-superseded wording. The
// machine-readable Reason token rides through unchanged in every case.
func TestPRFenceRefusalMessageIsReasonAware(t *testing.T) {
	cancelled := prFenceRefusal(7, ErrRunCancelled)
	if cancelled.Reason != "run-cancelled" {
		t.Fatalf("cancelled reason = %q, want run-cancelled", cancelled.Reason)
	}
	if cancelled.Message != "the run that owns this change was cancelled or superseded; publish nothing" {
		t.Fatalf("cancelled message unexpectedly changed: %q", cancelled.Message)
	}
	if superseded := prFenceRefusal(7, ErrStaleRunEpoch); superseded.Reason != "stale-run-epoch" ||
		superseded.Message != cancelled.Message {
		t.Fatalf("superseded reason=%q message=%q, want stale-run-epoch + the cancelled-or-superseded wording", superseded.Reason, superseded.Message)
	}
	completed := prFenceRefusal(7, ErrRunCompleted)
	if completed.Reason != "run-completed" {
		t.Fatalf("completed reason = %q, want run-completed (token must be unchanged)", completed.Reason)
	}
	if strings.Contains(completed.Message, "cancelled") || strings.Contains(completed.Message, "superseded") {
		t.Fatalf("run-completed message falsely claims cancellation: %q", completed.Message)
	}
	if !strings.Contains(completed.Message, "finished successfully") {
		t.Fatalf("run-completed message is not accurate for a successful closeout: %q", completed.Message)
	}
}

// TestPRPublishPreEffectValidationIsScoped pins change 0449's pre-effect rule
// for PR publication (spec: "B must pass relevant validation before an external
// effect, while A's unrelated findings cannot veto it"): an error on B itself or
// on a structural subject of B (a depends_on target) refuses before
// EnsurePullRequest, while unrelated records carrying errors — one parseable but
// invalid, one unparseable — never veto the publication.
func TestPRPublishPreEffectValidationIsScoped(t *testing.T) {
	requireRealGit(t)
	badType := func(src string) string {
		out := strings.Replace(src, "type: feat\n", "type: 'Not A Token'\n", 1)
		if out == src {
			t.Fatal("defect fixture did not rewrite the record's type; the fixture shape changed")
		}
		return out
	}
	b := inProgressChangeBlob(7, "widget", "v7", "")
	unrelatedInvalid := StatusBlob{
		Kind: repository.KindChange, Location: repository.LocationActive,
		Path: groomPath(30, "a-invalid"), Version: "v30",
		Data: []byte(badType(lifecycleChange(30, "a-invalid", "proposed"))),
	}
	unrelatedBroken := StatusBlob{
		Kind: repository.KindChange, Location: repository.LocationActive,
		Path: unrelatedBrokenPath, Version: "v99", Data: []byte(unrelatedBrokenBytes),
	}
	badB := b
	badB.Data = []byte(badType(string(b.Data)))
	depPath := "docs/changes/archive/2026-08-01-0008-dep.md"
	withDep := b
	withDep.Data = []byte(strings.Replace(string(b.Data), "depends_on: []\n", "depends_on: [8]\n", 1))
	if string(withDep.Data) == string(b.Data) {
		t.Fatal("dependency fixture did not rewrite depends_on; the fixture shape changed")
	}
	badDep := StatusBlob{
		Kind: repository.KindChange, Location: repository.LocationArchive,
		Path: depPath, Version: "v8", Data: []byte(badType(fixtureArchivedDone(8, "dep"))),
	}

	publish := func(t *testing.T, corpus []StatusBlob) (PRPublishResult, *fakeGitHub) {
		t.Helper()
		reader := &fakeReader{pin: mainPin(t), corpus: corpus}
		gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
		res := PRPublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
			newWorkingRepo(t, nil).invocation,
			PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prEvidenceBytes(t, prHead)})
		return res, gh
	}

	t.Run("unrelated-errors-do-not-veto", func(t *testing.T) {
		corpus := []StatusBlob{b, unrelatedInvalid, unrelatedBroken}
		// Non-vacuity: the unrelated parseable record genuinely carries an error
		// finding in the built report, so only the relevance rule lets B through.
		inputs, _ := parseCorpus(corpus)
		build, err := repository.BuildSnapshot(repository.BuildInput{Config: mainPin(t).Config.Effective, Documents: inputs})
		if err != nil {
			t.Fatalf("BuildSnapshot: %v", err)
		}
		poisoned := false
		for _, f := range build.Report.Findings() {
			if f.Severity == domain.SeverityError && f.Entity.Path == unrelatedInvalid.Path {
				poisoned = true
			}
		}
		if !poisoned {
			t.Fatalf("the unrelated record %s carries no error finding; the non-veto row would be vacuous", unrelatedInvalid.Path)
		}
		res, gh := publish(t, corpus)
		if res.Result != ResultApplied || len(gh.ensureCalls) != 1 {
			t.Fatalf("publish beside unrelated invalid records = %q (reason %q msg %q findings %+v, %d ensure calls), want applied with one ensure",
				res.Result, res.Reason, res.Message, res.Findings, len(gh.ensureCalls))
		}
	})

	for _, tc := range []struct {
		name   string
		corpus []StatusBlob
		names  string
	}{
		{"defective-B-refuses-before-effect", []StatusBlob{badB, unrelatedInvalid, unrelatedBroken}, b.Path},
		{"defective-dependency-refuses-before-effect", []StatusBlob{withDep, badDep, unrelatedInvalid, unrelatedBroken}, depPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, gh := publish(t, tc.corpus)
			if res.Result == ResultApplied || res.Result == ResultNoOp {
				t.Fatalf("publish applied despite a relevant defect (%s): %q", tc.names, res.Result)
			}
			if res.Reason != ReasonPRRecordInvalid {
				t.Fatalf("reason = %q (msg %q), want %q", res.Reason, res.Message, ReasonPRRecordInvalid)
			}
			if len(gh.ensureCalls) != 0 {
				t.Fatalf("EnsurePullRequest invoked %d time(s) despite a relevant defect; want 0", len(gh.ensureCalls))
			}
			named := false
			for _, f := range res.Findings {
				if f.Path == unrelatedInvalid.Path || f.Path == unrelatedBrokenPath {
					t.Errorf("refusal carries an unrelated record's finding %+v", f)
				}
				if f.Path == tc.names {
					named = true
				}
			}
			if !named {
				t.Errorf("refusal does not name the defective record %s: findings %+v", tc.names, res.Findings)
			}
		})
	}
}
