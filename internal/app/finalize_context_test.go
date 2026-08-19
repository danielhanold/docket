package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// --- fixtures -------------------------------------------------------------

// finalizeBlob builds a change record in finalize's population: a chosen status,
// a canonical PR reference, and any extra frontmatter (e.g. stacked_on).
func finalizeBlob(id int, slug, status, priority, prRef, extra string) StatusBlob {
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: %s\npriority: %s\ntype: feat\ncreated: 2026-01-02\npr: %q\n%s---\n\nBody of %d.\n",
		id, slug, id, status, priority, prRef, extra, id)
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

func (f *fakeFinalizeProber) ProbePR(_ context.Context, _, prRef, _ string) (domain.PRFacts, error) {
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
		prRefFor(30): {Number: "30", Version: "v30", State: "merged", HeadOID: "h30", BaseRef: "main", MergedAtUTC: "2026-01-03T00:00:00Z", MergeCommit: "m30"},
		prRefFor(31): openFacts(31, "MERGEABLE", 2, 20),
		prRefFor(32): openFacts(32, "CONFLICTING", 2, 20),
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
		prRefFor(72): openFacts(72, "MERGEABLE", 1, 1),
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
