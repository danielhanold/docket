package app

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
)

// permutedBoardPin returns pin with its resolved configuration replaced by one
// whose board presentation is permuted on every axis: a reversed section_order
// and id/asc on every section (the built-in is the six tokens in canonical
// order, updated/desc everywhere). Only Config.Effective.Board changes; every
// other pin field — revisions, branch names, diagnostics — is left identical to
// pin, so any difference the digest shows can only be the board presentation.
func permutedBoardPin(t *testing.T, pin StatusPin) StatusPin {
	t.Helper()
	snap, _, err := config.Resolve([]config.Source{{
		Layer: config.LayerRepository,
		Name:  ".docket.yml",
		Data: []byte("board:\n" +
			"  section_order: [deferred, proposed, groomed, blocked, built, in-progress]\n" +
			"  sorting:\n" +
			"    in-progress: {by: id, direction: asc}\n" +
			"    built:       {by: id, direction: asc}\n" +
			"    blocked:     {by: id, direction: asc}\n" +
			"    groomed:     {by: id, direction: asc}\n" +
			"    proposed:    {by: id, direction: asc}\n" +
			"    deferred:    {by: id, direction: asc}\n"),
	}}, config.ResolveContext{DefaultBranch: pin.DefaultBranch})
	if err != nil {
		t.Fatalf("resolve permuted board config: %v", err)
	}
	// Guard the fixture: the permutation must actually differ from the default,
	// or the isolation assertion would be vacuous.
	def := testConfig(t)
	if reflect.DeepEqual(def.Effective.Board, snap.Effective.Board) {
		t.Fatalf("permuted board config equals the default — the fixture exercises no presentation change")
	}
	pin.Config = *snap
	return pin
}

// TestStatusDigestUnchangedByBoardPresentation is the digest half of the
// projection-isolation claim (change 0367): the machine-readable status result
// is decided by lifecycle facts and the domain's own selection/ordering, and
// board.section_order / board.sorting flow into NONE of it. One corpus is run
// through Status twice against pins that differ ONLY in their resolved board
// presentation (default vs. reversed order + id/asc everywhere); the two whole
// StatusResults — counts, change rows and their order, readiness tokens, the
// ready queue and its order, records, findings — must be deeply equal.
func TestStatusDigestUnchangedByBoardPresentation(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		changeBlob(9, "nine", "feat", "high", "spec: docs/changes/specs/s9.md\n"),
		changeBlob(3, "three", "fix", "critical", "spec: docs/changes/specs/s3.md\n"),
		changeBlob(7, "seven", "chore", "low", ""),
		changeBlob(5, "five", "feat", "medium", "spec: docs/changes/specs/s5.md\n"),
		adrBlob(71, "some-adr"),
	}
	newFake := func(p StatusPin) *fakeReader {
		return &fakeReader{
			pin:    p,
			corpus: corpus,
			facts:  domain.NewBranchFacts(nil),
			artifacts: map[string]bool{
				"metadata|docs/changes/specs/s9.md": true,
				"metadata|docs/changes/specs/s3.md": true,
				"metadata|docs/changes/specs/s5.md": true,
			},
		}
	}

	// Opt into records (change 0397) so the inventory stays part of the digest
	// the projection-isolation claim covers.
	def := Status(context.Background(), newFake(pin), StatusOptions{IncludeRecords: true})
	permuted := Status(context.Background(), newFake(permutedBoardPin(t, pin)), StatusOptions{IncludeRecords: true})

	if def.Result != ResultApplied {
		t.Fatalf("default digest not applied: %q (%s)", def.Result, def.Message)
	}
	// Non-vacuity: the digest must carry a populated body and a non-empty ready
	// queue, or "deeply equal" would be trivially satisfied by two empty results.
	if len(def.Changes) == 0 || len(def.Ready) == 0 || def.Records == nil || len(*def.Records) == 0 {
		t.Fatalf("digest fixture is too thin to be a meaningful isolation check: %+v", def.Summary)
	}
	if !reflect.DeepEqual(def, permuted) {
		t.Fatalf("status digest changed with the board presentation:\n--- default ---\n%+v\n--- permuted ---\n%+v",
			def, permuted)
	}
}

func TestStatusResultEmptyCollectionsMarshalAsArrays(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{})
	buf, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(buf)
	// records is no longer one of the always-[] arrays (change 0397): the three
	// remaining arrays still marshal [], and a document with no requested
	// inventory carries no "records" key at all.
	for _, want := range []string{`"changes":[]`, `"ready":[]`, `"findings":[]`} {
		if !strings.Contains(s, want) {
			t.Errorf("marshalled document missing %s: %s", want, s)
		}
	}
	if strings.Contains(s, `"records"`) {
		t.Errorf("records key present without IncludeRecords: %s", s)
	}
	if strings.Contains(s, "null") {
		t.Errorf("null leaked into protocol document: %s", s)
	}
}

func TestStatusResultEnvelope(t *testing.T) {
	r := NewStatusResult(ResultApplied, StatusResult{})
	env := r.Env()
	if env.ProtocolVersion != ProtocolVersion || env.Operation != "status" || env.Result != ResultApplied {
		t.Errorf("unexpected envelope: %+v", env)
	}
}

func TestStatusResultFailureShapeCarriesNoPartialReport(t *testing.T) {
	r := NewStatusResult(ResultExternalFailed, StatusResult{Reason: "unreachable-ref", Message: "boom"})
	if len(r.Changes)+len(r.Ready) != 0 || r.Records != nil {
		t.Errorf("failure result carried report sections: %+v", r)
	}
}
