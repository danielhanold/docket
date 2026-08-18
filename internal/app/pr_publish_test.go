package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
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

func TestPRPublishAgreementChecks(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation

	baseReq := func() PRPublishRequest {
		return PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prEvidenceBytes(t, prHead)}
	}

	cases := []struct {
		name       string
		req        func() PRPublishRequest
		svc        *fakeWorkspaceService
		gh         *fakeGitHub
		wantResult Result
		wantReason string
	}{
		{
			name:       "control-all-agree",
			req:        baseReq,
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("b")}},
			wantResult: ResultApplied,
		},
		{
			name: "head-invalid",
			req: func() PRPublishRequest {
				r := baseReq()
				r.Head = "not-a-full-oid"
				return r
			},
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidInput,
			wantReason: ReasonPRHeadInvalid,
		},
		{
			name: "evidence-stale-head",
			req: func() PRPublishRequest {
				r := baseReq()
				r.EvidenceRecord = prEvidenceBytes(t, prOtherHead)
				return r
			},
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidState,
			wantReason: ReasonPREvidenceUnverified,
		},
		{
			name: "evidence-missing",
			req: func() PRPublishRequest {
				r := baseReq()
				r.EvidenceRecord = []byte("just prose, no block\n")
				return r
			},
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidState,
			wantReason: ReasonPREvidenceUnverified,
		},
		{
			name:       "local-head-mismatch",
			req:        baseReq,
			svc:        readyService(prOtherHead), // workspace head differs from requested
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidState,
			wantReason: ReasonPRLocalHeadMismatch,
		},
		{
			name:       "repository-unresolved",
			req:        baseReq,
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repoErr: errors.New("gh repo view failed")},
			wantResult: ResultExternalFailed,
			wantReason: ReasonPRRepositoryUnresolved,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := workspaceDepsFor(t, prReader(t))
			res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: tc.svc}, GitHubDeps{Service: tc.gh},
				repoDir, tc.req())

			if res.Result != tc.wantResult {
				t.Fatalf("result = %q, want %q (reason %q msg %q)", res.Result, tc.wantResult, res.Reason, res.Message)
			}
			if tc.wantReason != "" && res.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", res.Reason, tc.wantReason)
			}
			if tc.name == "control-all-agree" {
				if len(tc.gh.ensureCalls) != 1 {
					t.Fatalf("control: EnsurePullRequest called %d times, want 1", len(tc.gh.ensureCalls))
				}
				return
			}
			// Every broken conjunct must refuse BEFORE EnsurePullRequest.
			if len(tc.gh.ensureCalls) != 0 {
				t.Fatalf("%s: EnsurePullRequest invoked on a broken conjunct (%d calls)", tc.name, len(tc.gh.ensureCalls))
			}
		})
	}
}

// --- (2) body assembly: prose preserved, evidence replaced, backlink once ---

func TestPRPublishBodyAssembly(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	reader := prReader(t)

	// Authored prose already carrying a STALE build-evidence block. Publishing
	// must preserve the prose, replace the evidence block deterministically, and
	// insert the backlink exactly once.
	staleRec, _ := evidence.NewRecord("old command", prOtherHead, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	authored := "Authored prose before.\n\n" + evidence.Render(staleRec) + "\n\nAuthored prose after.\n"

	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	deps := workspaceDepsFor(t, reader)
	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: authored, EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (reason %q)", res.Result, res.Reason)
	}
	if len(gh.ensureCalls) != 1 {
		t.Fatalf("EnsurePullRequest called %d times, want 1", len(gh.ensureCalls))
	}
	gotBody := gh.ensureCalls[0].Body

	// Independent golden: parse authored, insert the backlink at the top, upsert
	// the fresh evidence — the exact deterministic composition the operation owns.
	change := prSnapshotChange(t, reader, 7)
	backlink, err := render.BacklinkContent(change, render.LinkContext{MetadataBranch: "main"})
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	doc, err := document.Parse([]byte(authored))
	if err != nil {
		t.Fatalf("parse authored: %v", err)
	}
	var ps document.PatchSet
	ps.InsertBlock("backlink", "generated — do not hand-edit", backlinkInterior(backlink), document.AtDocumentStart)
	withBacklink, err := doc.Apply(ps)
	if err != nil {
		t.Fatalf("insert backlink: %v", err)
	}
	freshRec, _ := evidence.NewRecord("go test ./...", prHead, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	wantBody, err := evidence.Upsert(withBacklink, freshRec)
	if err != nil {
		t.Fatalf("upsert evidence: %v", err)
	}
	if gotBody != string(wantBody) {
		t.Fatalf("assembled body mismatch:\n got: %q\nwant: %q", gotBody, string(wantBody))
	}

	// Spot invariants that the full-byte golden also encodes.
	if !strings.Contains(gotBody, "Authored prose before.") || !strings.Contains(gotBody, "Authored prose after.") {
		t.Errorf("authored prose not preserved: %q", gotBody)
	}
	if strings.Count(gotBody, "<!-- docket:backlink:start") != 1 {
		t.Errorf("backlink block not inserted exactly once: %q", gotBody)
	}
	if strings.Contains(gotBody, prOtherHead) {
		t.Errorf("stale evidence head survived the replace: %q", gotBody)
	}
	if !strings.Contains(gotBody, prHead) {
		t.Errorf("fresh evidence head absent from body: %q", gotBody)
	}
}

// --- (3) dispositions surface verbatim; PR snapshot round-trips ------------

func TestPRPublishThroughFakeGH(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation

	cases := []struct {
		name       string
		disp       githubcli.EnsureDisposition
		withPR     bool
		wantResult Result
	}{
		{"created", githubcli.EnsureCreated, true, ResultApplied},
		{"adopted", githubcli.EnsureAdopted, true, ResultNoOp},
		{"unknown", githubcli.EnsureUnknown, false, ResultExternalFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ens := githubcli.EnsureResult{Disposition: tc.disp}
			if tc.withPR {
				ens.PR = prMatchPR("body")
			}
			gh := &fakeGitHub{repo: prRepo(), ensureRes: ens}
			deps := workspaceDepsFor(t, prReader(t))
			res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
				repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "prose\n", EvidenceRecord: prEvidenceBytes(t, prHead)})

			if res.Result != tc.wantResult {
				t.Fatalf("result = %q, want %q (reason %q)", res.Result, tc.wantResult, res.Reason)
			}
			if res.Disposition != string(tc.disp) {
				t.Errorf("disposition = %q, want %q (carried verbatim)", res.Disposition, tc.disp)
			}
			if tc.withPR {
				if res.Number != 42 || res.Head != prHead || res.Base != "main" {
					t.Errorf("PR snapshot did not round-trip: %+v", res)
				}
				if res.URL != "https://github.com/acme/widget/pull/42" {
					t.Errorf("PR url mismatch: %q", res.URL)
				}
				if res.Reference != "github.com/acme/widget#42" {
					t.Errorf("PR reference = %q, want github.com/acme/widget#42", res.Reference)
				}
			}
		})
	}
}

// --- (4) redaction: no body bytes in the result JSON or human text ---------

func TestPRPublishRedaction(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	const secret = "SECRET-PR-BODY-CONTENT-do-not-leak"

	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR(secret)}}
	deps := workspaceDepsFor(t, prReader(t))
	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: secret + "\n", EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}

	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("result JSON leaked the PR body bytes: %s", raw)
	}
	if strings.Contains(res.HumanText(), secret) {
		t.Fatalf("HumanText leaked the PR body bytes: %q", res.HumanText())
	}
}

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
