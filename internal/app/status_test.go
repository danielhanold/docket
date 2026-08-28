package app

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// fakeReader is a scriptable StatusReader: every concern returns a canned value
// or a canned error, and the calls it received are recorded so a test can prove
// orchestration threaded the pin verbatim and asked the sources it should have.
type fakeReader struct {
	pin          StatusPin
	pinErr       error
	corpus       []StatusBlob
	corpusErr    error
	facts        domain.BranchFacts
	factsErr     error
	artifacts    map[string]bool           // "source|path" -> exists
	artifactData map[string]StatusArtifact // "source|path" -> read bytes/version
	artifactErr  error

	pinCount     int         // records PinContext calls
	branchAsks   [][]string  // records BranchFacts calls
	artifactAsks []string    // records ArtifactExists/ReadArtifact calls ("source|path")
	seenPins     []StatusPin // every pin threaded into a post-pin call
}

func (f *fakeReader) PinContext(_ context.Context, _ string) (StatusPin, error) {
	f.pinCount++
	return f.pin, f.pinErr
}

func (f *fakeReader) ReadCorpus(_ context.Context, pin StatusPin) ([]StatusBlob, error) {
	f.seenPins = append(f.seenPins, pin)
	return f.corpus, f.corpusErr
}

func (f *fakeReader) BranchFacts(_ context.Context, pin StatusPin, branches []string) (domain.BranchFacts, error) {
	f.seenPins = append(f.seenPins, pin)
	f.branchAsks = append(f.branchAsks, branches)
	return f.facts, f.factsErr
}

func (f *fakeReader) ArtifactExists(_ context.Context, pin StatusPin, source, path string) (bool, error) {
	f.seenPins = append(f.seenPins, pin)
	f.artifactAsks = append(f.artifactAsks, source+"|"+path)
	if f.artifactErr != nil {
		return false, f.artifactErr
	}
	return f.artifacts[source+"|"+path], nil
}

func (f *fakeReader) ReadArtifact(_ context.Context, pin StatusPin, source, path string) (StatusArtifact, error) {
	f.seenPins = append(f.seenPins, pin)
	f.artifactAsks = append(f.artifactAsks, source+"|"+path)
	if f.artifactErr != nil {
		return StatusArtifact{}, f.artifactErr
	}
	art, ok := f.artifactData[source+"|"+path]
	if !ok {
		return StatusArtifact{Found: false}, nil
	}
	return art, nil
}

// --- fixtures -------------------------------------------------------------

func testConfig(t *testing.T) config.Snapshot {
	t.Helper()
	snap, _, err := config.Resolve(nil, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("resolve built-in config: %v", err)
	}
	return *snap
}

func docketPin(t *testing.T) StatusPin {
	t.Helper()
	return StatusPin{
		DefaultBranch:       "main",
		DefaultRevision:     "1111111111111111111111111111111111111111",
		IntegrationBranch:   "main",
		IntegrationRevision: "2222222222222222222222222222222222222222",
		MetadataRevision:    "3333333333333333333333333333333333333333",
		Config:              testConfig(t),
	}
}

func mainPin(t *testing.T) StatusPin {
	t.Helper()
	return StatusPin{
		DefaultBranch:       "main",
		DefaultRevision:     "4444444444444444444444444444444444444444",
		IntegrationBranch:   "main",
		IntegrationRevision: "4444444444444444444444444444444444444444",
		MetadataRevision:    "4444444444444444444444444444444444444444",
		Config:              testConfig(t),
	}
}

// changeBlob builds an active-change StatusBlob from real record bytes.
func changeBlob(id int, slug, ctype, priority string, extra string) StatusBlob {
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: proposed\npriority: %s\ntype: %s\ncreated: 2026-01-02\n%s---\n\nBody of %d.\n",
		id, slug, id, priority, ctype, extra, id)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobchange%04d", id),
		Data:     []byte(fm),
	}
}

func adrBlob(id int, slug string) StatusBlob {
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: ADR %d\nstatus: Accepted\ndate: 2026-01-02\n---\n\nContext.\n", id, slug, id)
	return StatusBlob{
		Kind:     repository.KindADR,
		Location: repository.LocationLedger,
		Path:     fmt.Sprintf("docs/adrs/%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobadr%04d", id),
		Data:     []byte(fm),
	}
}

// --- tests ----------------------------------------------------------------

// TestStatusSourceDistinction is the artifact-source-distinction probe target:
// the operation asks the metadata source for a spec and the integration source
// for a plan, so the recorded source names diverge.
func TestStatusSourceDistinction(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-a.md"
	planPath := "docs/changes/plans/plan-a.md"
	corpus := []StatusBlob{
		changeBlob(11, "alpha", "feat", "high",
			fmt.Sprintf("spec: %s\nplan: %s\n", specPath, planPath)),
		changeBlob(12, "beta", "fix", "low", "spec: docs/changes/specs/spec-b.md\n"),
	}
	fake := &fakeReader{
		pin:    pin,
		corpus: corpus,
		facts:  domain.NewBranchFacts(nil),
		artifacts: map[string]bool{
			"metadata|" + specPath:                  true,
			"integration|" + planPath:               true,
			"metadata|docs/changes/specs/spec-b.md": true,
		},
	}

	got := Status(context.Background(), fake, StatusOptions{})
	if got.Result != ResultApplied {
		t.Fatalf("result = %q, want applied; message=%q", got.Result, got.Message)
	}

	var sawMetaSpec, sawIntegrationPlan bool
	for _, ask := range fake.artifactAsks {
		if ask == "metadata|"+specPath {
			sawMetaSpec = true
		}
		if ask == "integration|"+planPath {
			sawIntegrationPlan = true
		}
	}
	if !sawMetaSpec {
		t.Errorf("spec was not checked against the metadata source; asks=%v", fake.artifactAsks)
	}
	if !sawIntegrationPlan {
		t.Errorf("plan was not checked against the integration source; asks=%v", fake.artifactAsks)
	}

	// The pin is threaded verbatim into every post-pin reader call.
	for _, p := range fake.seenPins {
		if !reflect.DeepEqual(p, pin) {
			t.Errorf("reader received a mutated pin: %+v != %+v", p, pin)
		}
	}
	if got.Context.MetadataRevision != pin.MetadataRevision {
		t.Errorf("context did not echo the pin: %+v", got.Context)
	}
}

// TestStatusFiltersCannotSuppressHealth crosses the filter with the unhealthy
// record: filtering to one type drops the unhealthy change from the displayed
// projection, yet its finding and the error tally are unchanged.
func TestStatusFiltersCannotSuppressHealth(t *testing.T) {
	pin := docketPin(t)
	// A "docs"-typed change with an invalid slug — decodes, but validation
	// flags it. It is the one we will filter OUT.
	unhealthy := changeBlob(20, "Bad_Slug", "docs", "medium", "")
	unhealthy.Path = "docs/changes/active/0020-Bad_Slug.md"
	healthy := changeBlob(21, "healthy", "feat", "high", "")
	corpus := []StatusBlob{unhealthy, healthy}

	newFake := func() *fakeReader {
		return &fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil)}
	}

	unfiltered := Status(context.Background(), newFake(), StatusOptions{})
	filtered := Status(context.Background(), newFake(), StatusOptions{Types: []string{"feat"}})

	if filtered.Summary.DisplayedChanges >= unfiltered.Summary.DisplayedChanges {
		t.Errorf("filter did not shrink the displayed projection: %d unfiltered, %d filtered",
			unfiltered.Summary.DisplayedChanges, filtered.Summary.DisplayedChanges)
	}
	if filtered.Summary.ErrorFindings != unfiltered.Summary.ErrorFindings {
		t.Errorf("filter changed the error tally: %d unfiltered, %d filtered",
			unfiltered.Summary.ErrorFindings, filtered.Summary.ErrorFindings)
	}
	if filtered.Summary.ErrorFindings == 0 {
		t.Fatalf("expected the unhealthy record to produce an error finding")
	}
	// The unhealthy change is not in the displayed projection...
	for _, c := range filtered.Changes {
		if c.ID == 20 {
			t.Errorf("filtered-out change still displayed: %+v", c)
		}
	}
	// ...but its finding survived.
	if !hasFindingForEntity(filtered.Findings, "0020") {
		t.Errorf("finding for the filtered-out change was suppressed: %+v", filtered.Findings)
	}
}

func hasFindingForEntity(findings []StatusFinding, identity string) bool {
	for _, f := range findings {
		if f.Identity == identity {
			return true
		}
	}
	return false
}

// TestStatusPartialDamage: one unparseable blob becomes a finding while the
// remaining changes are still evaluated and present.
func TestStatusPartialDamage(t *testing.T) {
	pin := docketPin(t)
	broken := StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     "docs/changes/active/0030-broken.md",
		Version:  "blobbroken",
		Data:     []byte("---\nid: 30\n: not valid yaml :\n---\n"),
	}
	corpus := []StatusBlob{
		broken,
		changeBlob(31, "survivor-a", "feat", "high", ""),
		changeBlob(32, "survivor-b", "fix", "low", ""),
	}
	fake := &fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil)}
	got := Status(context.Background(), fake, StatusOptions{})
	if got.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", got.Result)
	}
	if got.Summary.ErrorFindings == 0 {
		t.Errorf("unparseable blob produced no finding: %+v", got.Findings)
	}
	if len(got.Changes) != 2 {
		t.Errorf("surviving changes not all present: got %d, want 2 (%+v)", len(got.Changes), got.Changes)
	}
}

func TestStatusFailureMapping(t *testing.T) {
	pin := docketPin(t)
	cases := []struct {
		name   string
		err    error
		want   Result
		reason string
	}{
		{"invalid", fmt.Errorf("bad path: %w", ErrStatusInvalidInput), ResultInvalidInput, "invalid-input"},
		{"external", fmt.Errorf("no ref: %w", ErrStatusExternal), ResultExternalFailed, "external-failed"},
		{"internal", fmt.Errorf("contract violated"), ResultInternalError, "internal-error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeReader{pin: pin, pinErr: tc.err}
			got := Status(context.Background(), fake, StatusOptions{})
			if got.Result != tc.want {
				t.Errorf("result = %q, want %q", got.Result, tc.want)
			}
			if got.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", got.Reason, tc.reason)
			}
			if len(got.Changes)+len(got.Ready)+len(got.Records) != 0 {
				t.Errorf("failure carried report sections: %+v", got)
			}
			if got.Message == "" {
				t.Errorf("failure carried no message")
			}
		})
	}
}

func TestStatusInterrupted(t *testing.T) {
	pin := docketPin(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeReader{pin: pin, pinErr: ctx.Err()}
	got := Status(ctx, fake, StatusOptions{})
	if got.Result != ResultInterrupted {
		t.Errorf("result = %q, want interrupted", got.Result)
	}
	if got.Reason != "interrupted" {
		t.Errorf("reason = %q, want interrupted", got.Reason)
	}
}

// TestStatusInvalidFilterValues: an unknown priority or type spelling is
// invalid input, reported before any corpus read.
func TestStatusInvalidFilterValues(t *testing.T) {
	pin := docketPin(t)
	for _, tc := range []struct {
		name string
		opts StatusOptions
	}{
		{"priority", StatusOptions{Priorities: []string{"bogus"}}},
		{"type", StatusOptions{Types: []string{"not-a-configured-type"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeReader{pin: pin, corpus: []StatusBlob{changeBlob(1, "a", "feat", "high", "")}}
			got := Status(context.Background(), fake, tc.opts)
			if got.Result != ResultInvalidInput {
				t.Errorf("result = %q, want invalid-input", got.Result)
			}
			if len(fake.artifactAsks) != 0 {
				t.Errorf("invalid input still read artifacts: %v", fake.artifactAsks)
			}
		})
	}
}

// TestStatusStableOrdering: shuffled corpus input yields changes by numeric ID
// ascending, records by kind-then-identity, and two runs marshal identically.
func TestStatusStableOrdering(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		adrBlob(71, "z-adr"),
		changeBlob(9, "nine", "feat", "high", "spec: docs/changes/specs/s9.md\n"),
		adrBlob(54, "a-adr"),
		changeBlob(3, "three", "fix", "low", "spec: docs/changes/specs/s3.md\n"),
		changeBlob(7, "seven", "feat", "medium", "spec: docs/changes/specs/s7.md\n"),
	}
	newFake := func() *fakeReader {
		return &fakeReader{
			pin:    pin,
			corpus: corpus,
			facts:  domain.NewBranchFacts(nil),
			artifacts: map[string]bool{
				"metadata|docs/changes/specs/s9.md": true,
				"metadata|docs/changes/specs/s3.md": true,
				"metadata|docs/changes/specs/s7.md": true,
			},
		}
	}

	got := Status(context.Background(), newFake(), StatusOptions{})
	var ids []int
	for _, c := range got.Changes {
		ids = append(ids, c.ID)
	}
	if len(ids) != 3 || ids[0] != 3 || ids[1] != 7 || ids[2] != 9 {
		t.Errorf("changes not sorted by ascending ID: %v", ids)
	}
	// records: changes (by id) then adrs (by id)
	wantKinds := []string{"change", "change", "change", "adr", "adr"}
	if len(got.Records) != len(wantKinds) {
		t.Fatalf("record count = %d, want %d (%+v)", len(got.Records), len(wantKinds), got.Records)
	}
	for i, k := range wantKinds {
		if got.Records[i].Kind != k {
			t.Errorf("record[%d].Kind = %q, want %q", i, got.Records[i].Kind, k)
		}
	}

	a, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(Status(context.Background(), newFake(), StatusOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("two runs not byte-identical:\n%s\n%s", a, b)
	}
	if strings.Contains(string(a), "null") {
		t.Errorf("null leaked into protocol document: %s", a)
	}
}

// TestStatusEmptyStatesExplicit: an empty ready queue and a zero-match filter
// still produce an applied document with empty (never null) arrays.
func TestStatusEmptyStatesExplicit(t *testing.T) {
	pin := docketPin(t)
	// Neither change is build-ready (no spec), so the ready queue is empty.
	fake := &fakeReader{
		pin:    pin,
		corpus: []StatusBlob{changeBlob(1, "a", "feat", "high", ""), changeBlob(2, "b", "fix", "low", "")},
		facts:  domain.NewBranchFacts(nil),
	}
	got := Status(context.Background(), fake, StatusOptions{Types: []string{"chore"}})
	if got.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", got.Result)
	}
	if got.Summary.DisplayedChanges != 0 {
		t.Errorf("zero-match filter displayed %d changes", got.Summary.DisplayedChanges)
	}
	if got.Ready == nil || got.Changes == nil || got.Records == nil || got.Findings == nil {
		t.Errorf("a collection was nil: %+v", got)
	}
	buf, _ := json.Marshal(got)
	for _, want := range []string{`"ready":[]`, `"changes":[]`} {
		if !strings.Contains(string(buf), want) {
			t.Errorf("missing %s in %s", want, buf)
		}
	}
}

// TestStatusReadySubsetOfDisplayed guards the invariant that every ID in a
// filtered result's Ready slice has a corresponding row in the same result's
// Changes slice — i.e. ready ⊆ displayed. Ready comes from domain.SelectQueue
// (domain's own predicate); displayed comes from matchesFilter, a hand-written
// mirror in status.go. If the two ever diverge in the narrowing direction, this
// reddens. Two build-ready changes of two types with a single-type filter make
// the projection a genuine (non-vacuous) subset: displayed and ready must both
// drop the non-matching change.
func TestStatusReadySubsetOfDisplayed(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		changeBlob(41, "feat-one", "feat", "high", "spec: docs/changes/specs/s41.md\n"),
		changeBlob(42, "fix-two", "fix", "critical", "spec: docs/changes/specs/s42.md\n"),
	}
	fake := &fakeReader{
		pin:    pin,
		corpus: corpus,
		facts:  domain.NewBranchFacts(nil),
		artifacts: map[string]bool{
			"metadata|docs/changes/specs/s41.md": true,
			"metadata|docs/changes/specs/s42.md": true,
		},
	}

	got := Status(context.Background(), fake, StatusOptions{Types: []string{"feat"}})
	if got.Result != ResultApplied {
		t.Fatalf("result = %q, want applied; message=%q", got.Result, got.Message)
	}

	// Non-vacuous preconditions: the filter genuinely projected a subset (one of
	// two changes displayed), and the ready queue is not empty.
	if got.Summary.DisplayedChanges != 1 {
		t.Fatalf("expected the single-type filter to display exactly 1 of 2 changes, got %d (%+v)",
			got.Summary.DisplayedChanges, got.Changes)
	}
	if len(got.Ready) == 0 {
		t.Fatalf("expected a non-empty ready queue so the subset assertion is non-vacuous; ready=%v", got.Ready)
	}

	displayedIDs := make(map[int]bool, len(got.Changes))
	for _, c := range got.Changes {
		displayedIDs[c.ID] = true
	}
	for _, id := range got.Ready {
		if !displayedIDs[id] {
			t.Errorf("ready ID %d has no row in the displayed Changes projection: ready=%v, displayed=%v",
				id, got.Ready, displayedIDs)
		}
	}
}

// TestStatusReadyQueueOrder: build-ready changes populate Ready in selector
// order (priority band first), and each is marked Ready in the change rows.
func TestStatusReadyQueueOrder(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		changeBlob(1, "low-one", "feat", "low", "spec: docs/changes/specs/s1.md\n"),
		changeBlob(2, "crit-two", "fix", "critical", "spec: docs/changes/specs/s2.md\n"),
	}
	fake := &fakeReader{
		pin:    pin,
		corpus: corpus,
		facts:  domain.NewBranchFacts(nil),
		artifacts: map[string]bool{
			"metadata|docs/changes/specs/s1.md": true,
			"metadata|docs/changes/specs/s2.md": true,
		},
	}
	got := Status(context.Background(), fake, StatusOptions{})
	if len(got.Ready) != 2 || got.Ready[0] != 2 || got.Ready[1] != 1 {
		t.Errorf("ready order = %v, want [2 1] (critical before low)", got.Ready)
	}
	if got.Summary.ReadyChanges != 2 {
		t.Errorf("ready count = %d, want 2", got.Summary.ReadyChanges)
	}
}
