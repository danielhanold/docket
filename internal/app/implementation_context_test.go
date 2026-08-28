package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// --- fixtures -------------------------------------------------------------

// learningBlob builds an active (retained) learning record from real bytes.
func learningBlob(slug string) StatusBlob {
	fm := fmt.Sprintf("---\nslug: %s\ntitle: Lesson %s\nhook: 'when %s appears do the thing'\npromotion_state: retained\n---\n\nBody of %s.\n",
		slug, slug, slug, slug)
	return StatusBlob{
		Kind:     repository.KindLearning,
		Location: repository.LocationLedger,
		Path:     "docs/changes/learnings/" + slug + ".md",
		Version:  "bloblearn-" + slug,
		Data:     []byte(fm),
	}
}

// liveParentBlob builds an in-progress (claimed) change record carrying a
// recorded feature branch — the live stack parent the facts-backed effective
// base resolver consults (domain rule 4: a live parent resolves to its
// recorded branch only when that branch is present in facts).
func liveParentBlob(id int, slug, branch string) StatusBlob {
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: 'in-progress'\npriority: medium\ntype: feat\ncreated: 2026-01-02\nbranch: '%s'\nclaimed_at: '2026-01-03T00:00:00Z'\nreconciled: true\n---\n\nBody of %d.\n",
		id, slug, id, branch, id)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobchange%04d", id),
		Data:     []byte(fm),
	}
}

// contextDeps wraps a fake reader as the read-only deps the operation consumes.
func contextDeps(fake *fakeReader) PlanningDeps {
	return PlanningDeps{Reader: fake, Clock: testClock()}
}

// --- tests ----------------------------------------------------------------

// TestContextImplementationSelectsByPolicy: with no --id the bundle's change is
// SelectQueue's first build-ready candidate; every fact is exact, and the
// change/spec source bytes are byte-identical to the corpus fixtures.
func TestContextImplementationSelectsByPolicy(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-alpha.md"
	specBytes := []byte("# Spec Alpha\n\nAuthoritative design body.\n")
	corpus := []StatusBlob{
		changeBlob(11, "alpha", "feat", "high", "spec: "+specPath+"\n"),
		changeBlob(12, "beta", "fix", "low", ""), // no spec: needs-brainstorm, not build-ready
	}
	fake := &fakeReader{
		pin:    pin,
		corpus: corpus,
		artifactData: map[string]StatusArtifact{
			sourceMetadata + "|" + specPath: {Found: true, Version: "specblob-alpha", Data: specBytes},
		},
	}

	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
	}
	if got.Context == nil {
		t.Fatal("applied result carried no bundle")
	}
	b := got.Context

	if b.Change.Summary == nil || b.Change.Summary.ID != 11 {
		t.Fatalf("selected change = %+v, want id 11", b.Change.Summary)
	}
	if !bytes.Equal(b.Change.Source, corpus[0].Data) {
		t.Errorf("change source bytes not byte-identical:\n got %q\nwant %q", b.Change.Source, corpus[0].Data)
	}
	if b.Change.Version != corpus[0].Version {
		t.Errorf("change version = %q, want %q", b.Change.Version, corpus[0].Version)
	}
	if b.Change.Path != corpus[0].Path {
		t.Errorf("change path = %q, want %q", b.Change.Path, corpus[0].Path)
	}
	if !bytes.Equal(b.Spec.Source, specBytes) {
		t.Errorf("spec source bytes not byte-identical:\n got %q\nwant %q", b.Spec.Source, specBytes)
	}
	if b.Spec.Version != "specblob-alpha" || b.Spec.Path != specPath {
		t.Errorf("spec entity = %+v", b.Spec)
	}
	if b.MetadataCommit != pin.MetadataRevision {
		t.Errorf("metadata commit = %q, want %q", b.MetadataCommit, pin.MetadataRevision)
	}
	if b.MetadataRef != "docket" {
		t.Errorf("metadata ref = %q, want docket", b.MetadataRef)
	}
	if b.Readiness != string(domain.ReadyBuildReady) {
		t.Errorf("readiness = %q, want build-ready", b.Readiness)
	}
	if !b.ClaimEligible || b.ClaimRefusal != "" {
		t.Errorf("claim eligibility = %v / %q, want eligible", b.ClaimEligible, b.ClaimRefusal)
	}
	if b.EffectiveBase.Kind != string(domain.BaseResolved) || b.EffectiveBase.Branch != "main" {
		t.Errorf("effective base = %+v, want resolved/main", b.EffectiveBase)
	}
	if b.Workflow.IntegrationBranch != "main" ||
		b.Workflow.Remote != "origin" || b.Workflow.FeatureBranch != "feat/alpha" {
		t.Errorf("workflow = %+v", b.Workflow)
	}
	// Nil collections normalized to empty slices, never null.
	buf, _ := json.Marshal(got)
	if strings.Contains(string(buf), "null") {
		t.Errorf("null leaked into protocol document: %s", buf)
	}
}

// TestContextImplementationExplicitID: an explicit id is returned even when it
// is not first in the selection queue (attributed-retry support).
func TestContextImplementationExplicitID(t *testing.T) {
	pin := docketPin(t)
	specA := "docs/changes/specs/spec-a.md"
	specB := "docs/changes/specs/spec-b.md"
	corpus := []StatusBlob{
		changeBlob(11, "alpha", "feat", "high", "spec: "+specA+"\n"), // ranks first (high)
		changeBlob(12, "beta", "fix", "low", "spec: "+specB+"\n"),    // ranks second (low)
	}
	fake := &fakeReader{
		pin:    pin,
		corpus: corpus,
		artifactData: map[string]StatusArtifact{
			sourceMetadata + "|" + specA: {Found: true, Version: "sa", Data: []byte("spec a\n")},
			sourceMetadata + "|" + specB: {Found: true, Version: "sb", Data: []byte("spec b\n")},
		},
	}

	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{ID: 12})
	if got.Result != ResultApplied || got.Context == nil {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}
	if got.Context.Change.Summary.ID != 12 {
		t.Errorf("explicit id ignored: got change %d, want 12", got.Context.Change.Summary.ID)
	}
}

// TestContextImplementationFeatureBranchHonorsMintPrefix proves the pre-claim
// context previews the branch the imminent claim will MINT through the single
// constructor — honoring branch_prefix over the change type, never a hardcoded
// feat/<slug>. The context runs before the claim, so no branch is recorded yet;
// the previewed name must equal exactly what claim would stamp.
func TestContextImplementationFeatureBranchHonorsMintPrefix(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-hot.md"
	corpus := []StatusBlob{
		// type fix, with a branch_prefix override of hotfix: the mint is hotfix/<slug>.
		changeBlob(11, "urgent", "fix", "high", "branch_prefix: hotfix\nspec: "+specPath+"\n"),
	}
	fake := &fakeReader{
		pin:    pin,
		corpus: corpus,
		artifactData: map[string]StatusArtifact{
			sourceMetadata + "|" + specPath: {Found: true, Version: "sh", Data: []byte("hot\n")},
		},
	}

	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
	if got.Result != ResultApplied || got.Context == nil {
		t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
	}
	if got.Context.Workflow.FeatureBranch != "hotfix/urgent" {
		t.Errorf("FeatureBranch = %q, want the minted hotfix/urgent (branch_prefix honored over feat and over the fix type)", got.Context.Workflow.FeatureBranch)
	}
}

// TestContextImplementationRevisionConsistency: every fact derives from one
// pinned snapshot — PinContext is called exactly once.
func TestContextImplementationRevisionConsistency(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-a.md"
	fake := &fakeReader{
		pin:    pin,
		corpus: []StatusBlob{changeBlob(11, "alpha", "feat", "high", "spec: "+specPath+"\n")},
		artifactData: map[string]StatusArtifact{
			sourceMetadata + "|" + specPath: {Found: true, Version: "sa", Data: []byte("spec a\n")},
		},
	}
	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}
	if fake.pinCount != 1 {
		t.Errorf("PinContext called %d times, want exactly 1", fake.pinCount)
	}
}

// TestContextImplementationTypedAbsence: each absence/refusal is a typed closed
// outcome that fabricates no bundle.
func TestContextImplementationTypedAbsence(t *testing.T) {
	pin := docketPin(t)
	specA := "docs/changes/specs/spec-a.md"

	cases := []struct {
		name       string
		corpus     []StatusBlob
		artifacts  map[string]StatusArtifact
		req        ImplementationContextRequest
		wantResult Result
		wantReason string
	}{
		{
			name:       "no candidate",
			corpus:     []StatusBlob{changeBlob(1, "a", "feat", "high", "")}, // no spec => not build-ready
			req:        ImplementationContextRequest{},
			wantResult: ResultNoOp,
			wantReason: ReasonContextNoCandidate,
		},
		{
			name:       "unknown id",
			corpus:     []StatusBlob{changeBlob(1, "a", "feat", "high", "spec: "+specA+"\n")},
			artifacts:  map[string]StatusArtifact{sourceMetadata + "|" + specA: {Found: true, Data: []byte("x")}},
			req:        ImplementationContextRequest{ID: 999},
			wantResult: ResultInvalidInput,
			wantReason: ReasonContextUnknownChange,
		},
		{
			name: "duplicate id",
			corpus: []StatusBlob{
				changeBlob(13, "dup-one", "feat", "high", "spec: "+specA+"\n"),
				changeBlob(13, "dup-two", "feat", "high", "spec: "+specA+"\n"),
			},
			req:        ImplementationContextRequest{ID: 13},
			wantResult: ResultInvalidState,
			wantReason: ReasonContextAmbiguousID,
		},
		{
			name:       "missing spec file",
			corpus:     []StatusBlob{changeBlob(14, "needs-spec", "feat", "high", "spec: "+specA+"\n")},
			artifacts:  map[string]StatusArtifact{}, // spec link present but file absent
			req:        ImplementationContextRequest{ID: 14},
			wantResult: ResultInvalidState,
			wantReason: ReasonContextMissingArtifact,
		},
		{
			name:       "not ready",
			corpus:     []StatusBlob{changeBlob(15, "no-design", "feat", "high", "")}, // no spec, not trivial
			req:        ImplementationContextRequest{ID: 15},
			wantResult: ResultInvalidState,
			wantReason: "not-ready-" + string(domain.ReadyNeedsBrainstorm),
		},
		{
			name: "unresolved effective base",
			corpus: []StatusBlob{
				changeBlob(20, "parent", "feat", "high", "spec: "+specA+"\n"),
				changeBlob(21, "child", "feat", "high", "spec: "+specA+"\nstacked_on: 20\n"),
			},
			artifacts:  map[string]StatusArtifact{sourceMetadata + "|" + specA: {Found: true, Data: []byte("x")}},
			req:        ImplementationContextRequest{ID: 21},
			wantResult: ResultInvalidState,
			wantReason: "not-ready-" + string(domain.ReadyStackBaseUnresolved),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeReader{pin: pin, corpus: tc.corpus, artifactData: tc.artifacts}
			got := ContextImplementation(context.Background(), contextDeps(fake), "", tc.req)
			if got.Result != tc.wantResult {
				t.Errorf("result = %q, want %q (reason %q)", got.Result, tc.wantResult, got.Reason)
			}
			if got.Reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tc.wantReason)
			}
			if got.Context != nil {
				t.Errorf("typed absence fabricated a bundle: %+v", got.Context)
			}
		})
	}
}

// TestContextImplementationLearningsCapability: disabled => an explicit warning
// and no entries; enabled => the index entries are present.
func TestContextImplementationLearningsCapability(t *testing.T) {
	specPath := "docs/changes/specs/spec-a.md"
	art := map[string]StatusArtifact{sourceMetadata + "|" + specPath: {Found: true, Data: []byte("spec\n")}}
	newCorpus := func() []StatusBlob {
		return []StatusBlob{
			changeBlob(11, "alpha", "feat", "high", "spec: "+specPath+"\n"),
			learningBlob("some-lesson"),
		}
	}

	t.Run("disabled", func(t *testing.T) {
		fake := &fakeReader{pin: learningsDisabledPin(), corpus: newCorpus(), artifactData: art}
		got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
		if got.Result != ResultApplied || got.Context == nil {
			t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
		}
		if len(got.Context.Learnings) != 0 {
			t.Errorf("learnings listed while capability disabled: %+v", got.Context.Learnings)
		}
		if !warnsAboutLearnings(got.Context.Warnings) {
			t.Errorf("no learnings-capability warning: %+v", got.Context.Warnings)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		fake := &fakeReader{pin: docketPin(t), corpus: newCorpus(), artifactData: art}
		got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
		if got.Result != ResultApplied || got.Context == nil {
			t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
		}
		if len(got.Context.Learnings) == 0 {
			t.Fatalf("no learning entries surfaced while enabled")
		}
		if got.Context.Learnings[0].Slug != "some-lesson" {
			t.Errorf("learning entry = %+v, want slug some-lesson", got.Context.Learnings[0])
		}
		if warnsAboutLearnings(got.Context.Warnings) {
			t.Errorf("learnings warning present while capability enabled: %+v", got.Context.Warnings)
		}
	})
}

func warnsAboutLearnings(warnings []string) bool {
	for _, w := range warnings {
		if strings.Contains(strings.ToLower(w), "learning") {
			return true
		}
	}
	return false
}

// TestContextImplementationDoesNotEchoAuthoredBody: the protocol JSON carries
// the source bytes (redaction constraint applies to logs/human text), but
// HumanText never includes an authored document body.
func TestContextImplementationDoesNotEchoAuthoredBody(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-a.md"
	marker := "SECRET_AUTHORED_BODY_MARKER"
	specBytes := []byte("# spec\n\n" + marker + "\n")
	corpus := []StatusBlob{changeBlob(11, "alpha", "feat", "high", "spec: "+specPath+"\n")}
	fake := &fakeReader{
		pin:    pin,
		corpus: corpus,
		artifactData: map[string]StatusArtifact{
			sourceMetadata + "|" + specPath: {Found: true, Version: "sa", Data: specBytes},
		},
	}
	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
	if got.Result != ResultApplied || got.Context == nil {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}

	if strings.Contains(got.HumanText(), marker) {
		t.Errorf("HumanText echoed an authored body: %q", got.HumanText())
	}
	// The JSON document does carry the bytes (base64-encoded []byte).
	buf, _ := json.Marshal(got)
	if !strings.Contains(string(buf), base64.StdEncoding.EncodeToString(specBytes)) {
		t.Errorf("protocol JSON did not carry the spec source bytes")
	}
}

// TestContextImplementationReportsHalt proves the bundle surfaces the durable
// halt checkpoints — a build-ready change carrying a historical "## Run halted"
// section (a prior aborted run) reports Halt.RunHalted so the implementer knows a
// run was halted rather than re-dispatching blind.
func TestContextImplementationReportsHalt(t *testing.T) {
	pin := docketPin(t)
	specA := "docs/changes/specs/spec-a.md"
	src := "---\nid: 11\nslug: alpha\ntitle: Alpha\nstatus: proposed\npriority: high\ntype: feat\ncreated: 2026-01-02\nspec: " + specA +
		"\n---\n\nBody.\n\n## Run halted\n\n### 2026-08-14\n\nPrior run paused.\n"
	corpus := []StatusBlob{{
		Kind: repository.KindChange, Location: repository.LocationActive,
		Path: "docs/changes/active/0011-alpha.md", Version: "v11", Data: []byte(src),
	}}
	fake := &fakeReader{
		pin: pin, corpus: corpus,
		artifactData: map[string]StatusArtifact{sourceMetadata + "|" + specA: {Found: true, Version: "sa", Data: []byte("spec a\n")}},
	}
	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{ID: 11})
	if got.Result != ResultApplied || got.Context == nil {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}
	if !got.Context.Halt.RunHalted {
		t.Errorf("bundle did not surface the durable run-halted checkpoint: %+v", got.Context.Halt)
	}
}

// TestContextImplementationStackedLiveParentUsesRemoteFacts: a proposed,
// designed child stacked on a live parent whose recorded branch IS in the
// reader's facts gets an applied bundle — build-ready, claim-eligible, and an
// effective base resolved to the parent's recorded branch — through BOTH
// automatic selection and explicit-id inspection. The reader is asked for
// exactly the deterministic stackBranches set, once, with the original pin.
func TestContextImplementationStackedLiveParentUsesRemoteFacts(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-child.md"
	corpus := []StatusBlob{
		liveParentBlob(20, "parent", "feat/parent"),
		changeBlob(21, "child", "feat", "high", "spec: "+specPath+"\nstacked_on: 20\n"),
	}
	for name, req := range map[string]ImplementationContextRequest{
		"automatic-selection": {},
		"explicit-id":         {ID: 21},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeReader{
				pin:    pin,
				corpus: corpus,
				facts:  domain.NewBranchFacts(map[string]bool{"feat/parent": true}),
				artifactData: map[string]StatusArtifact{
					sourceMetadata + "|" + specPath: {Found: true, Version: "sc", Data: []byte("spec child\n")},
				},
			}
			got := ContextImplementation(context.Background(), contextDeps(fake), "", req)
			if got.Result != ResultApplied || got.Context == nil {
				t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
			}
			b := got.Context
			if b.Change.Summary == nil || b.Change.Summary.ID != 21 {
				t.Fatalf("selected change = %+v, want the stacked child 21", b.Change.Summary)
			}
			if b.Readiness != string(domain.ReadyBuildReady) {
				t.Errorf("readiness = %q, want build-ready", b.Readiness)
			}
			if !b.ClaimEligible || b.ClaimRefusal != "" {
				t.Errorf("claim eligibility = %v / %q, want eligible", b.ClaimEligible, b.ClaimRefusal)
			}
			if b.EffectiveBase.Kind != string(domain.BaseResolved) || b.EffectiveBase.Branch != "feat/parent" {
				t.Errorf("effective base = %+v, want resolved/feat/parent (the parent's recorded branch, not the integration branch)", b.EffectiveBase)
			}
			// The reader was asked exactly once, for the deterministic
			// stackBranches(snapshot) set, with the original pin threaded.
			if len(fake.branchAsks) != 1 {
				t.Fatalf("BranchFacts called %d times, want exactly 1 (asks: %v)", len(fake.branchAsks), fake.branchAsks)
			}
			if want := []string{"feat/parent"}; !reflect.DeepEqual(fake.branchAsks[0], want) {
				t.Errorf("BranchFacts asked for %v, want %v", fake.branchAsks[0], want)
			}
			for i, seen := range fake.seenPins {
				if !reflect.DeepEqual(seen, pin) {
					t.Errorf("post-pin call %d threaded a different pin", i)
				}
			}
		})
	}
}

// TestContextImplementationStackedParentBranchAbsent: the SAME stacked child
// with the parent branch absent from the returned facts remains refused —
// an empty fact set is a valid "branch does not exist" answer, and docket
// must not claim a child from an invented fallback base. Explicit id refuses
// as not-ready-stack-base-unresolved; automatic selection skips it entirely.
func TestContextImplementationStackedParentBranchAbsent(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-child.md"
	corpus := []StatusBlob{
		liveParentBlob(20, "parent", "feat/parent"),
		changeBlob(21, "child", "feat", "high", "spec: "+specPath+"\nstacked_on: 20\n"),
	}
	art := map[string]StatusArtifact{
		sourceMetadata + "|" + specPath: {Found: true, Version: "sc", Data: []byte("spec child\n")},
	}

	t.Run("explicit-id", func(t *testing.T) {
		fake := &fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil), artifactData: art}
		got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{ID: 21})
		if got.Result != ResultInvalidState || got.Reason != "not-ready-"+string(domain.ReadyStackBaseUnresolved) {
			t.Errorf("result=%q reason=%q, want invalid-state/not-ready-stack-base-unresolved", got.Result, got.Reason)
		}
		if got.Context != nil {
			t.Errorf("refusal fabricated a bundle: %+v", got.Context)
		}
	})

	t.Run("automatic-selection", func(t *testing.T) {
		fake := &fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil), artifactData: art}
		got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
		if got.Result != ResultNoOp || got.Reason != ReasonContextNoCandidate {
			t.Errorf("result=%q reason=%q, want no-op/no-candidate (the unresolved child must not be selected)", got.Result, got.Reason)
		}
	})
}

// TestContextImplementationBranchFactsFailure: a failed facts lookup is a
// typed operation failure through classifyStatusError, never an empty fact
// set — an observation failure must not be misreported as proven branch
// absence (learning probe-error-is-not-clean-absence). No partial bundle.
func TestContextImplementationBranchFactsFailure(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-child.md"
	fake := &fakeReader{
		pin: pin,
		corpus: []StatusBlob{
			changeBlob(11, "alpha", "feat", "high", "spec: "+specPath+"\n"),
		},
		factsErr: fmt.Errorf("git ls-remote: connection reset: %w", ErrStatusExternal),
		artifactData: map[string]StatusArtifact{
			sourceMetadata + "|" + specPath: {Found: true, Version: "sa", Data: []byte("spec a\n")},
		},
	}
	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
	if got.Result != ResultExternalFailed || got.Reason != ReasonStatusExternal {
		t.Errorf("result=%q reason=%q, want external-failed/external-failed", got.Result, got.Reason)
	}
	if got.Context != nil {
		t.Errorf("failed facts read fabricated a bundle: %+v", got.Context)
	}
}
