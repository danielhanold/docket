package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
)

// --- fixtures -------------------------------------------------------------

// finalizeBlob builds a change record in finalize's population: a chosen status,
// a canonical PR reference, and any extra frontmatter (e.g. stacked_on).
func finalizeBlob(id int, slug, status, priority, prRef, extra string) StatusBlob {
	// A post-claim record carries the branch stamped once at claim time; every
	// finalize-population status is post-claim, so the recorded branch is what
	// each operation consumes (a still-proposed fixture stays branchless).
	branchField := ""
	switch status {
	case "in-progress", "blocked", "implemented", "done", "stacked-merged":
		branchField = fmt.Sprintf("branch: feat/%s\n", slug)
	}
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: %s\npriority: %s\ntype: feat\ncreated: 2026-01-02\npr: %q\n%s%s---\n\nBody of %d.\n",
		id, slug, id, status, priority, prRef, branchField, extra, id)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobfin%04d", id),
		Data:     []byte(fm),
	}
}

// prRef builds the canonical "owner/repo#<n>" reference the manifest records.
func prRefFor(number int) string { return fmt.Sprintf("acme/widgets#%d", number) }

// fakeFinalizeProber is a scriptable FinalizePRProber: it returns canned domain
// facts (or a canned error) keyed by the PR reference, and records that it was
// only ever asked to read.
type fakeFinalizeProber struct {
	facts map[string]domain.PRFacts
	errs  map[string]error
	calls int
}

func (f *fakeFinalizeProber) ProbePR(_ context.Context, _, prRef string) (domain.PRFacts, error) {
	f.calls++
	if e := f.errs[prRef]; e != nil {
		return domain.PRFacts{}, e
	}
	return f.facts[prRef], nil
}

// finalizeDeps wraps a fake reader + fake prober as the read-only deps the
// operation consumes, with a recording engine that must never fire.
func finalizeDeps(fake *fakeReader, prober FinalizePRProber, engine *recordingEngine) FinalizeDeps {
	return FinalizeDeps{
		Planning: PlanningDeps{Reader: fake, Engine: engine, Clock: testClock()},
		PRProber: prober,
	}
}

// withHead stamps the PR's live head branch onto facts so the domain identity
// classifier reconciles it against the branch recorded at claim time. The
// finalize-population fixtures record branch: feat/<slug>, so an actionable
// candidate pairs its facts with the matching feat/<slug> head.
func withHead(f domain.PRFacts, head string) domain.PRFacts {
	f.HeadBranch = head
	return f
}

// openFacts is the domain facts of an open, approved, mergeable PR.
func openFacts(number int, mergeable string, files, lines int) domain.PRFacts {
	return domain.PRFacts{
		Number:       fmt.Sprintf("%d", number),
		Version:      fmt.Sprintf("v%d", number),
		State:        "open",
		Approved:     true,
		Mergeable:    mergeable,
		HeadOID:      fmt.Sprintf("head%d", number),
		BaseRef:      "main",
		ChangedFiles: files,
		DiffLines:    lines,
	}
}

// --- tests ----------------------------------------------------------------

// TestContextFinalizeSelection: with no id the candidate order matches the
// domain finalize queue — a merged PR surfaces first as merged-recovery, then
// MERGEABLE before CONFLICTING — and the metadata context is pinned exactly once.
func TestContextFinalizeSelection(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(31, "beta", "implemented", "high", prRefFor(31), ""),  // open mergeable
		finalizeBlob(32, "gamma", "implemented", "high", prRefFor(32), ""), // open conflicting
		finalizeBlob(30, "alpha", "implemented", "high", prRefFor(30), ""), // merged (recovery)
	}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(30): {Number: "30", Version: "v30", State: "merged", HeadBranch: "feat/alpha", HeadOID: "h30", BaseRef: "main", MergedAtUTC: "2026-01-03T00:00:00Z", MergeCommit: "m30"},
		prRefFor(31): withHead(openFacts(31, "MERGEABLE", 2, 20), "feat/beta"),
		prRefFor(32): withHead(openFacts(32, "CONFLICTING", 2, 20), "feat/gamma"),
	}}
	engine := &recordingEngine{}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, engine), "", FinalizeContextRequest{})
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
	}
	gotOrder := candidateIDs(got.Candidates)
	wantOrder := []int{30, 31, 32}
	if fmt.Sprint(gotOrder) != fmt.Sprint(wantOrder) {
		t.Fatalf("candidate order = %v, want %v", gotOrder, wantOrder)
	}
	if got.Candidates[0].Band != "merged-recovery" {
		t.Errorf("first candidate band = %q, want merged-recovery", got.Candidates[0].Band)
	}
	if got.Candidates[1].Band != "mergeable" || got.Candidates[2].Band != "conflicting" {
		t.Errorf("bands = %q / %q, want mergeable / conflicting", got.Candidates[1].Band, got.Candidates[2].Band)
	}
	if fake.pinCount != 1 {
		t.Errorf("PinContext called %d times, want exactly 1", fake.pinCount)
	}
	if len(engine.calls) != 0 {
		t.Errorf("read-only operation opened %d transactions, want 0", len(engine.calls))
	}
	// The merged-recovery candidate carries its full merged facts.
	if pr := got.Candidates[0].PR; pr.Verdict != "probed" || pr.State != "merged" || pr.MergeCommit != "m30" {
		t.Errorf("merged candidate PR facts = %+v", pr)
	}
}

// TestContextFinalizeExplicitID: an explicit id inspects exactly that record even
// when skip-reasoned — an unapproved open PR surfaces as a candidate carrying the
// approval-required skip and an override note — while an absent id is refused.
// TestContextFinalizeReportBranchFromRecord proves the candidate report echoes
// the branch RECORDED at claim time — a non-derived name is surfaced verbatim,
// and a record that carries no branch reports the empty string rather than a
// fabricated feat/<slug> (this report only describes; it invents no identity).
func TestContextFinalizeReportBranchFromRecord(t *testing.T) {
	pin := docketPin(t)
	named := finalizeBlob(41, "one", "implemented", "high", prRefFor(41), "")
	named.Data = []byte(strings.Replace(string(named.Data), "branch: feat/one\n", "branch: feature/renamed-head\n", 1))
	branchless := finalizeBlob(42, "two", "implemented", "low", prRefFor(42), "")
	branchless.Data = []byte(strings.Replace(string(branchless.Data), "branch: feat/two\n", "", 1))
	corpus := []StatusBlob{named, branchless}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(41): openFacts(41, "MERGEABLE", 1, 5),
		prRefFor(42): openFacts(42, "MERGEABLE", 1, 5),
	}}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, &recordingEngine{}), "", FinalizeContextRequest{})
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}
	byID := make(map[int]FinalizeCandidateReport, len(got.Candidates))
	for _, c := range got.Candidates {
		byID[c.ID] = c
	}
	if _, ok := byID[41]; !ok {
		t.Fatalf("candidate 41 not surfaced: %v", candidateIDs(got.Candidates))
	}
	if _, ok := byID[42]; !ok {
		t.Fatalf("candidate 42 not surfaced: %v", candidateIDs(got.Candidates))
	}
	if byID[41].Branch != "feature/renamed-head" {
		t.Errorf("recorded branch not echoed: Branch = %q, want feature/renamed-head", byID[41].Branch)
	}
	if byID[42].Branch != "" {
		t.Errorf("branchless record: Branch = %q, want the empty string (no fabricated feat/<slug>)", byID[42].Branch)
	}
}

// TestContextFinalizeIdentityMismatchReport: when the branch recorded at claim
// time disagrees with the exact PR's live head, the candidate is surfaced with
// the branch-pr-head-mismatch skip and its report carries BOTH identities — the
// recorded branch verbatim and the PR's actual head branch — so the reader sees
// the exact disagreement rather than a silent misclassification. The mismatch is
// not --id-overridable: an explicit --id selects it but sets no override note.
func TestContextFinalizeIdentityMismatchReport(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{finalizeBlob(45, "one", "implemented", "high", prRefFor(45), "")}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(45): withHead(openFacts(45, "MERGEABLE", 1, 5), "feature/other"),
	}}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, &recordingEngine{}), "", FinalizeContextRequest{ID: 45})
	if got.Result != ResultApplied || len(got.Candidates) != 1 {
		t.Fatalf("result=%q reason=%q candidates=%d", got.Result, got.Reason, len(got.Candidates))
	}
	c := got.Candidates[0]
	if c.SkipReason != "branch-pr-head-mismatch" {
		t.Errorf("skip reason = %q, want branch-pr-head-mismatch", c.SkipReason)
	}
	if c.Branch != "feat/one" {
		t.Errorf("recorded branch = %q, want feat/one (verbatim from the record)", c.Branch)
	}
	if c.PR.HeadBranch != "feature/other" {
		t.Errorf("PR head branch = %q, want feature/other (the exact PR's live head)", c.PR.HeadBranch)
	}
	// An unresolved-identity skip is never --id-overridable.
	if c.OverrideNote != "" {
		t.Errorf("identity mismatch must carry no override note, got %q", c.OverrideNote)
	}
}

func TestContextFinalizeExplicitID(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(41, "one", "implemented", "high", prRefFor(41), ""),
		finalizeBlob(42, "two", "implemented", "low", prRefFor(42), ""),
	}
	unapproved := domain.PRFacts{Number: "41", Version: "v41", State: "open", Approved: false, Mergeable: "MERGEABLE", HeadOID: "h41", BaseRef: "main"}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(41): unapproved,
		prRefFor(42): openFacts(42, "MERGEABLE", 1, 5),
	}}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, &recordingEngine{}), "", FinalizeContextRequest{ID: 41})
	if got.Result != ResultApplied || len(got.Candidates) != 1 {
		t.Fatalf("result=%q reason=%q candidates=%d", got.Result, got.Reason, len(got.Candidates))
	}
	c := got.Candidates[0]
	if c.ID != 41 {
		t.Fatalf("explicit id ignored: got %d, want 41", c.ID)
	}
	if c.SkipReason != "approval-required" {
		t.Errorf("skip reason = %q, want approval-required", c.SkipReason)
	}
	if c.OverrideNote == "" || !strings.Contains(c.OverrideNote, "approval-required") {
		t.Errorf("override note missing for explicit-id approval skip: %q", c.OverrideNote)
	}

	// An absent explicit id is a typed refusal that fabricates no candidate.
	absent := ContextFinalize(context.Background(), finalizeDeps(&fakeReader{pin: pin, corpus: corpus}, prober, &recordingEngine{}), "", FinalizeContextRequest{ID: 999})
	if absent.Result != ResultInvalidInput || absent.Reason != ReasonFinalizeUnknownChange {
		t.Errorf("absent id: result=%q reason=%q, want invalid-input/%s", absent.Result, absent.Reason, ReasonFinalizeUnknownChange)
	}
	if len(absent.Candidates) != 0 {
		t.Errorf("typed refusal fabricated candidates: %+v", absent.Candidates)
	}
}

// TestContextFinalizeProbeErrorIsUnknown: a GitHub probe error surfaces the
// candidate with unknown PR facts and the pr-unknown skip token — never a clean
// absence (the change stays surfaced, not omitted).
func TestContextFinalizeProbeErrorIsUnknown(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{finalizeBlob(50, "solo", "implemented", "high", prRefFor(50), "")}
	prober := &fakeFinalizeProber{errs: map[string]error{prRefFor(50): fmt.Errorf("gh probe timed out")}}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, &recordingEngine{}), "", FinalizeContextRequest{})
	if got.Result != ResultApplied || len(got.Candidates) != 1 {
		t.Fatalf("result=%q candidates=%d", got.Result, len(got.Candidates))
	}
	c := got.Candidates[0]
	if c.SkipReason != "pr-unknown" {
		t.Errorf("skip reason = %q, want pr-unknown", c.SkipReason)
	}
	if c.PR.Verdict != "unknown" {
		t.Errorf("PR verdict = %q, want unknown", c.PR.Verdict)
	}
	if len(got.Warnings) == 0 {
		t.Errorf("an unresolved probe should surface a warning")
	}
}

// TestContextFinalizeStackFacts: descendant lifecycles and the open-child PR set
// come from the stacked_on graph, not from any rendered table.
func TestContextFinalizeStackFacts(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(60, "root", "implemented", "high", prRefFor(60), ""),
		finalizeBlob(61, "mid", "implemented", "high", prRefFor(61), "stacked_on: 60\n"),
		finalizeBlob(62, "leaf", "implemented", "high", prRefFor(62), "stacked_on: 61\n"),
	}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(60): openFacts(60, "MERGEABLE", 1, 5),
		prRefFor(61): {Number: "61", Version: "v61", State: "open", Approved: true, Mergeable: "MERGEABLE", HeadOID: "h61", BaseRef: "feat/root"},
		prRefFor(62): {Number: "62", Version: "v62", State: "open", Approved: true, Mergeable: "MERGEABLE", HeadOID: "h62", BaseRef: "feat/mid"},
	}}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, &recordingEngine{}), "", FinalizeContextRequest{})
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}
	var root *FinalizeCandidateReport
	for i := range got.Candidates {
		if got.Candidates[i].ID == 60 {
			root = &got.Candidates[i]
		}
	}
	if root == nil {
		t.Fatalf("root candidate 60 not surfaced: %+v", candidateIDs(got.Candidates))
	}
	if ids := descendantIDs(root.Descendants); fmt.Sprint(ids) != fmt.Sprint([]int{61, 62}) {
		t.Errorf("descendants = %v, want [61 62]", ids)
	}
	if root.Descendants[0].Status != "implemented" {
		t.Errorf("descendant lifecycle = %q, want implemented", root.Descendants[0].Status)
	}
	if root.Descendants[0].PRDestination != "feat/root" {
		t.Errorf("descendant PR destination = %q, want feat/root", root.Descendants[0].PRDestination)
	}
	if fmt.Sprint(root.OpenChildPRs) != fmt.Sprint([]int{61}) {
		t.Errorf("open child PRs = %v, want [61]", root.OpenChildPRs)
	}
}

// TestContextFinalizeTypedReasons: every surfaced candidate carries either a band
// or a closed skip token — nothing omitted or guessed.
func TestContextFinalizeTypedReasons(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(70, "proposed-pr", "proposed", "high", prRefFor(70), ""), // not-implemented
		finalizeBlob(71, "draft-pr", "implemented", "high", prRefFor(71), ""), // draft
		finalizeBlob(72, "ok", "implemented", "high", prRefFor(72), ""),       // mergeable
	}
	prober := &fakeFinalizeProber{facts: map[string]domain.PRFacts{
		prRefFor(70): openFacts(70, "MERGEABLE", 1, 1),
		prRefFor(71): {Number: "71", Version: "v71", State: "open", Draft: true, Approved: true, Mergeable: "MERGEABLE", HeadOID: "h71", BaseRef: "main"},
		prRefFor(72): withHead(openFacts(72, "MERGEABLE", 1, 1), "feat/ok"),
	}}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, prober, &recordingEngine{}), "", FinalizeContextRequest{})
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}
	if len(got.Candidates) != 3 {
		t.Fatalf("candidates surfaced = %d, want 3 (nothing omitted): %v", len(got.Candidates), candidateIDs(got.Candidates))
	}
	skipTokens := map[string]bool{
		"not-implemented": true, "draft": true, "pr-closed": true,
		"approval-required": true, "finalize-blocked": true, "dependency-unmerged": true,
		"malformed": true, "pr-unknown": true,
	}
	bandTokens := map[string]bool{"merged-recovery": true, "mergeable": true, "conflicting": true, "unknown": true}
	for _, c := range got.Candidates {
		switch {
		case c.SkipReason != "":
			if !skipTokens[c.SkipReason] {
				t.Errorf("candidate %d carries non-closed skip token %q", c.ID, c.SkipReason)
			}
		case c.Band != "":
			if !bandTokens[c.Band] {
				t.Errorf("candidate %d carries non-closed band token %q", c.ID, c.Band)
			}
		default:
			t.Errorf("candidate %d carries neither a band nor a skip reason", c.ID)
		}
	}
	// Nil collections normalize to empty, never null.
	buf, _ := json.Marshal(got)
	if strings.Contains(string(buf), "null") {
		t.Errorf("null leaked into protocol document: %s", buf)
	}
}

// prURLFor builds the full-URL pr: reference the board requires — the form the
// pre-0344 prober could not parse.
func prURLFor(number int) string {
	return fmt.Sprintf("https://github.com/acme/widgets/pull/%d", number)
}

// TestContextFinalizeURLFormPRRef: a change whose pr: is the board-required
// full-URL form flows through the PRODUCTION prober (which parses the ref
// before contacting GitHub) and surfaces as a probed merged-recovery candidate
// — never pr-unknown. Before 0344 the prober refused the ref with "carries no
// parseable number" and the selector read pr-unknown, making the change
// un-finalizable through the binary.
func TestContextFinalizeURLFormPRRef(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{finalizeBlob(90, "urlform", "implemented", "high", prURLFor(235), "")}
	gh := &fakeCloseoutGitHub{
		repo: githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widgets"},
		merged: map[int]closeoutProbe{
			235: {outcome: githubcli.MergeAlreadyMerged, facts: githubcli.MergedFacts{
				Version: "v235", HeadBranch: "feat/urlform", HeadOID: "h235", BaseRef: "main",
				MergedAtUTC: "2026-08-24T00:00:00Z", MergeCommit: "m235",
			}},
		},
	}
	fake := &fakeReader{pin: pin, corpus: corpus}

	got := ContextFinalize(context.Background(), finalizeDeps(fake, NewGitHubFinalizeProber(gh), &recordingEngine{}), "", FinalizeContextRequest{})
	if got.Result != ResultApplied || len(got.Candidates) != 1 {
		t.Fatalf("result=%q reason=%q candidates=%d", got.Result, got.Reason, len(got.Candidates))
	}
	c := got.Candidates[0]
	if c.SkipReason == "pr-unknown" {
		t.Fatalf("URL-form pr: still reads as pr-unknown — the prober refused the reference")
	}
	if c.PR.Verdict != "probed" || c.PR.State != "merged" || c.PR.Number != "235" {
		t.Errorf("PR report = verdict %q state %q number %q, want probed/merged/235", c.PR.Verdict, c.PR.State, c.PR.Number)
	}
	if c.Band != "merged-recovery" {
		t.Errorf("band = %q, want merged-recovery", c.Band)
	}
}

// --- production prober: exact-number identity -----------------------------

// fakeProberGitHub is a recording FinalizeGitHub for the production
// githubFinalizeProber tests. It scripts DiscoverRepository, the merged reprobe,
// and the exact-number view, and COUNTS head-search calls so a test can assert
// the exact-number probe path never falls back to FindOpenPullRequestsByHead —
// the assert that reddens if head discovery is reintroduced on this path.
type fakeProberGitHub struct {
	repo            githubcli.Repository
	merged          map[int]closeoutProbe
	views           map[int]githubcli.PullRequest
	viewErrs        map[int]error
	headSearchCalls int
}

func (f *fakeProberGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}
func (f *fakeProberGitHub) ProbeMerged(_ context.Context, _ githubcli.Repository, number int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	if p, ok := f.merged[number]; ok {
		return p.outcome, p.facts, nil
	}
	return githubcli.MergeNotMergeable, githubcli.MergedFacts{}, nil
}
func (f *fakeProberGitHub) ViewPullRequest(_ context.Context, _ githubcli.Repository, number int) (githubcli.PullRequest, error) {
	if e := f.viewErrs[number]; e != nil {
		return githubcli.PullRequest{}, e
	}
	return f.views[number], nil
}
func (f *fakeProberGitHub) FindOpenPullRequestsByHead(context.Context, githubcli.Repository, string) ([]githubcli.PullRequest, error) {
	f.headSearchCalls++
	return nil, nil
}
func (f *fakeProberGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: prober must not call this")
}
func (f *fakeProberGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: prober must not call this")
}
func (f *fakeProberGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: prober must not call this")
}
func (f *fakeProberGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: prober must not call this")
}

func proberRepo() githubcli.Repository {
	return githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widgets"}
}

// TestProbePRReadsExactNumber: the recorded pr: parses to 7, the exact-number
// view returns an open PR, and its state and (renamed) head flow into the facts
// — while the head-search method is NEVER called. That last assert reddens if
// FindOpenPullRequestsByHead is reintroduced to identify this change's PR.
func TestProbePRReadsExactNumber(t *testing.T) {
	gh := &fakeProberGitHub{
		repo: proberRepo(),
		views: map[int]githubcli.PullRequest{
			7: {Number: 7, State: githubcli.StateOpen, HeadBranch: "feature/renamed-head", HeadCommit: "h7", BaseBranch: "main", Version: "v7"},
		},
	}
	facts, err := NewGitHubFinalizeProber(gh).ProbePR(context.Background(), "", prRefFor(7))
	if err != nil {
		t.Fatalf("ProbePR: %v", err)
	}
	if facts.State != "open" {
		t.Errorf("State = %q, want open", facts.State)
	}
	if facts.HeadBranch != "feature/renamed-head" {
		t.Errorf("HeadBranch = %q, want feature/renamed-head", facts.HeadBranch)
	}
	if facts.Number != "7" {
		t.Errorf("Number = %q, want 7", facts.Number)
	}
	if gh.headSearchCalls != 0 {
		t.Errorf("FindOpenPullRequestsByHead called %d times; the exact-number path must never fall back to head discovery", gh.headSearchCalls)
	}
}

// TestProbePRClosedOnlyFromCleanExactRead: a closed state is reported only when
// the exact-number view cleanly returns it.
func TestProbePRClosedOnlyFromCleanExactRead(t *testing.T) {
	gh := &fakeProberGitHub{
		repo: proberRepo(),
		views: map[int]githubcli.PullRequest{
			7: {Number: 7, State: githubcli.StateClosed, HeadBranch: "feature/renamed-head", HeadCommit: "h7", BaseBranch: "main", Version: "v7"},
		},
	}
	facts, err := NewGitHubFinalizeProber(gh).ProbePR(context.Background(), "", prRefFor(7))
	if err != nil {
		t.Fatalf("ProbePR: %v", err)
	}
	if facts.State != "closed" {
		t.Errorf("State = %q, want closed", facts.State)
	}
	if gh.headSearchCalls != 0 {
		t.Errorf("FindOpenPullRequestsByHead called %d times; closed comes only from the exact-number view", gh.headSearchCalls)
	}
}

// TestProbePRUnknownOnViewError: an errored exact-number view is folded into
// unknown facts (carrying only the parsed number token) — never a fabricated
// closed state.
func TestProbePRUnknownOnViewError(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{finalizeBlob(80, "solo", "implemented", "high", prRefFor(7), "")}
	reader := &fakeReader{pin: pin, corpus: corpus}
	c := prSnapshotChange(t, reader, 80)

	gh := &fakeProberGitHub{
		repo:     proberRepo(),
		viewErrs: map[int]error{7: fmt.Errorf("gh pr view timed out")},
	}
	facts, unresolved := probeFinalizeFacts(context.Background(), NewGitHubFinalizeProber(gh), "", c)
	if !unresolved {
		t.Fatalf("a view error must surface as unresolved")
	}
	if facts.State != "unknown" {
		t.Errorf("State = %q, want unknown (a view error must not read as closed)", facts.State)
	}
	if facts.Number != "7" {
		t.Errorf("Number = %q, want the parsed token 7", facts.Number)
	}
}

// TestProbePRMergedCarriesHeadBranch: a merged reprobe carries its head branch
// through into the merged facts, short-circuiting the view.
func TestProbePRMergedCarriesHeadBranch(t *testing.T) {
	gh := &fakeProberGitHub{
		repo: proberRepo(),
		merged: map[int]closeoutProbe{
			7: {outcome: githubcli.MergeAlreadyMerged, facts: githubcli.MergedFacts{
				Version: "v7", HeadBranch: "feature/merged-head", HeadOID: "h7", BaseRef: "main",
				MergedAtUTC: "2026-08-24T00:00:00Z", MergeCommit: "m7",
			}},
		},
	}
	facts, err := NewGitHubFinalizeProber(gh).ProbePR(context.Background(), "", prRefFor(7))
	if err != nil {
		t.Fatalf("ProbePR: %v", err)
	}
	if facts.State != "merged" {
		t.Errorf("State = %q, want merged", facts.State)
	}
	if facts.HeadBranch != "feature/merged-head" {
		t.Errorf("HeadBranch = %q, want feature/merged-head", facts.HeadBranch)
	}
	if gh.headSearchCalls != 0 {
		t.Errorf("FindOpenPullRequestsByHead called %d times; the merged path must never touch head discovery", gh.headSearchCalls)
	}
}

// --- helpers --------------------------------------------------------------

func candidateIDs(cs []FinalizeCandidateReport) []int {
	out := make([]int, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func descendantIDs(ds []FinalizeDescendant) []int {
	out := make([]int, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}

// TestParsePRNumberForms pins parsePRNumber's accepted PR-reference grammar: the
// full GitHub URL in every real shape (canonical, trailing slash, query,
// fragment, deeper sub-page) and the "owner/repo#N" shorthand both yield the
// positive number, while a non-numeric/missing/zero/signed URL segment and a
// non-positive or garbage shorthand return (0, false). The url rows are the bug
// this change fixes — the prober previously could not read a full-URL pr: ref.
func TestParsePRNumberForms(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want int
		ok   bool
	}{
		{"shorthand", "acme/widgets#42", 42, true},
		{"url canonical", "https://github.com/acme/widgets/pull/235", 235, true},
		{"url trailing slash", "https://github.com/acme/widgets/pull/235/", 235, true},
		{"url query", "https://github.com/acme/widgets/pull/235?w=1", 235, true},
		{"url fragment", "https://github.com/acme/widgets/pull/235#discussion_r1", 235, true},
		{"url sub-page", "https://github.com/acme/widgets/pull/235/files", 235, true},
		{"url non-numeric", "https://github.com/acme/widgets/pull/abc", 0, false},
		{"url missing number", "https://github.com/acme/widgets/pull/", 0, false},
		{"url zero", "https://github.com/acme/widgets/pull/0", 0, false},
		{"url signed", "https://github.com/acme/widgets/pull/+42", 0, false},
		{"shorthand zero", "acme/widgets#0", 0, false},
		{"shorthand negative", "acme/widgets#-1", 0, false},
		{"shorthand signed", "acme/widgets#+42", 0, false},
		{"shorthand plus-only", "acme/widgets#+", 0, false},
		{"garbage", "not a pr ref", 0, false},
		{"empty", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, ok := parsePRNumber(tc.ref)
			if n != tc.want || ok != tc.ok {
				t.Errorf("parsePRNumber(%q) = (%d, %v), want (%d, %v)", tc.ref, n, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestPRNumberTokenForms pins prNumberToken's string projection over the same
// grammar: it emits the canonical number for both accepted forms (URL and
// shorthand, including a deeper sub-page URL) and "" for any reference with no
// parseable positive number, proving the token stays in lockstep with
// parsePRNumber via the shared parsePRRef extractor.
func TestPRNumberTokenForms(t *testing.T) {
	cases := []struct {
		ref  string
		want string
	}{
		{"acme/widgets#42", "42"},
		{"https://github.com/acme/widgets/pull/235", "235"},
		{"https://github.com/acme/widgets/pull/235/files", "235"},
		{"acme/widgets#0", ""},
		{"acme/widgets#-7", ""},
		{"acme/widgets#+42", ""},
		{"https://github.com/acme/widgets/pull/abc", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := prNumberToken(tc.ref); got != tc.want {
			t.Errorf("prNumberToken(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}
