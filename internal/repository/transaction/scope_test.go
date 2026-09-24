package transaction

import (
	"slices"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// Record paths the scope tests build their fixture snapshots from. B is the
// named subject; A1..A4 are unrelated changes; D is B's dependency; Bad is a
// record that failed to parse (path-only findings, no snapshot entry).
const (
	scPathB   = "docs/changes/active/0020-b.md"
	scPathD   = "docs/changes/active/0010-d.md"
	scPathD2  = "docs/changes/archive/2026-09-01-0010-d.md"
	scPathA1  = "docs/changes/active/0091-a1.md"
	scPathA2  = "docs/changes/active/0092-a2.md"
	scPathA3  = "docs/changes/active/0093-a3.md"
	scPathA4  = "docs/changes/active/0094-a4.md"
	scPathBad = "docs/changes/active/0077-bad.md"
	scPathADR = "docs/adrs/0005-x.md"
	scPathL   = "docs/learnings/some-lesson.md"
)

// scChange builds one change record at path.
func scChange(id int, path string) domain.Change {
	return domain.NewChange(domain.ChangeSpec{ID: domain.ChangeID(id), Slug: "s", Path: path})
}

// scSnapshot builds a snapshot holding exactly the given changes plus one ADR
// and one learning, so id/slug lookups of every kind have something to find.
func scSnapshot(changes ...domain.Change) domain.Snapshot {
	return domain.NewSnapshot(domain.SnapshotSpec{
		Changes:   changes,
		ADRs:      []domain.ADR{domain.NewADR(domain.ADRSpec{ID: 5, Slug: "x", Path: scPathADR})},
		Learnings: []domain.Learning{domain.NewLearning(domain.LearningSpec{Slug: "some-lesson", Path: scPathL})},
	})
}

// scBaseSnapshot is the healthy corpus most cases share.
func scBaseSnapshot() domain.Snapshot {
	return scSnapshot(scChange(20, scPathB), scChange(10, scPathD), scChange(91, scPathA1),
		scChange(92, scPathA2), scChange(93, scPathA3), scChange(94, scPathA4))
}

// scBlobs is a blob-id table giving every fixture path a distinct, stable id.
func scBlobs() map[string]gitcli.ObjectID {
	return map[string]gitcli.ObjectID{
		scPathB: "b0", scPathD: "d0", scPathA1: "a10", scPathA2: "a20", scPathA3: "a30",
		scPathA4: "a40", scPathBad: "bad0", scPathADR: "adr0", scPathL: "l0",
	}
}

// scState assembles a LoadedState from a snapshot, a blob table, and findings.
func scState(snap domain.Snapshot, blobs map[string]gitcli.ObjectID, findings ...domain.Finding) LoadedState {
	return LoadedState{Snapshot: snap, Report: domain.NewValidationReport(findings), Blobs: blobs}
}

// scScope is the subject set {B}.
func scScope() map[gitcli.RepoPath]bool { return map[gitcli.RepoPath]bool{scPathB: true} }

// scChangeID is an id-only change reference (the cycle-member shape).
func scChangeID(id int) domain.EntityRef { return domain.EntityRef{Kind: domain.EntityChange, ID: id} }

// scPathRef is a path-bearing change reference (the changeRef shape).
func scPathRef(id int, path string) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityChange, ID: id, Slug: "s", Path: path}
}

// scParseFinding is the path-only, detail-free parse-failure finding shape.
func scParseFinding(path string) domain.Finding {
	return domain.Finding{Code: "document-parse", Severity: domain.SeverityError,
		Entity: domain.EntityRef{Kind: domain.EntityChange, Path: path}}
}

// scCycle is an id-only cycle finding attributed to its first member.
func scCycle(members ...int) domain.Finding {
	related := make([]domain.EntityRef, 0, len(members))
	for _, m := range members {
		related = append(related, scChangeID(m))
	}
	return domain.Finding{Code: "change-dependency-cycle", Severity: domain.SeverityError,
		Entity: scChangeID(members[0]), Field: "depends_on", Related: related}
}

// scDuplicate is a duplicate-id finding on one record, carrying its count.
func scDuplicate(id int, path, count string) domain.Finding {
	return domain.Finding{Code: "change-id-duplicate", Severity: domain.SeverityError,
		Entity: scPathRef(id, path), Field: "id", Detail: map[string]string{"count": count}}
}

// scDangling is a depends_on dangling-reference finding on a record.
func scDangling(id int, path string, target int, lookup string) domain.Finding {
	return domain.Finding{Code: "change-reference-dangling", Severity: domain.SeverityError,
		Entity: scPathRef(id, path), Field: "depends_on", Related: []domain.EntityRef{scChangeID(target)},
		Detail: map[string]string{"lookup": lookup, "target": itoa(target)}}
}

func TestCanonicalFindingKey(t *testing.T) {
	t.Run("detail count difference changes the key", func(t *testing.T) {
		if canonicalFindingKey(scDuplicate(91, scPathA1, "2")) == canonicalFindingKey(scDuplicate(91, scPathA1, "3")) {
			t.Fatal("findings differing only in Detail[count] share a key")
		}
	})
	t.Run("detail lookup flip changes the key", func(t *testing.T) {
		if canonicalFindingKey(scDangling(91, scPathA1, 50, "absent")) == canonicalFindingKey(scDangling(91, scPathA1, 50, "ambiguous")) {
			t.Fatal("findings differing only in Detail[lookup] share a key")
		}
	})
	t.Run("one related member difference changes the key", func(t *testing.T) {
		if canonicalFindingKey(scCycle(91, 92, 93)) == canonicalFindingKey(scCycle(91, 92, 94)) {
			t.Fatal("findings differing only in one Related member share a key")
		}
	})
	t.Run("related input order does not change the key", func(t *testing.T) {
		a := scCycle(91, 92, 93)
		b := a
		b.Related = []domain.EntityRef{scChangeID(93), scChangeID(91), scChangeID(92)}
		if canonicalFindingKey(a) != canonicalFindingKey(b) {
			t.Fatal("Related order changed the key")
		}
	})
	t.Run("related multiplicity changes the key", func(t *testing.T) {
		a := scCycle(91, 92)
		b := scCycle(91, 92, 92)
		if canonicalFindingKey(a) == canonicalFindingKey(b) {
			t.Fatal("a repeated Related member was collapsed")
		}
	})
	t.Run("detail map order does not change the key", func(t *testing.T) {
		mk := func(order []string) domain.Finding {
			d := make(map[string]string)
			for _, k := range order {
				d[k] = "v-" + k
			}
			return domain.Finding{Code: "c", Severity: domain.SeverityError, Entity: scPathRef(91, scPathA1), Detail: d}
		}
		want := canonicalFindingKey(mk([]string{"a", "b", "c", "d", "e", "f", "g", "h"}))
		for i := 0; i < 50; i++ {
			if got := canonicalFindingKey(mk([]string{"h", "g", "f", "e", "d", "c", "b", "a"})); got != want {
				t.Fatalf("Detail iteration order changed the key: %q vs %q", got, want)
			}
		}
	})
	t.Run("severity entity and field each change the key", func(t *testing.T) {
		base := scDuplicate(91, scPathA1, "2")
		variants := []domain.Finding{base, base, base, base, base}
		variants[0].Severity = domain.SeverityWarning
		variants[1].Entity.Path = scPathA2
		variants[2].Entity.ID = 92
		variants[3].Field = "slug"
		variants[4].Code = "other"
		for i, v := range variants {
			if canonicalFindingKey(v) == canonicalFindingKey(base) {
				t.Fatalf("variant %d shares the base key", i)
			}
		}
	})
	t.Run("delimiter-shaped values cannot collide", func(t *testing.T) {
		a := domain.Finding{Code: "c", Entity: scPathRef(91, scPathA1), Detail: map[string]string{"a": "b\",\"c\":\"d"}}
		b := domain.Finding{Code: "c", Entity: scPathRef(91, scPathA1), Detail: map[string]string{"a": "b", "c": "d"}}
		if canonicalFindingKey(a) == canonicalFindingKey(b) {
			t.Fatal("an injected delimiter collided two distinct Detail maps")
		}
	})
}

func TestFindingPaths(t *testing.T) {
	st := scState(scBaseSnapshot(), scBlobs())
	cases := []struct {
		name   string
		f      domain.Finding
		want   []gitcli.RepoPath
		wantOK bool
	}{
		{"path-only entity is itself", scParseFinding(scPathBad), []gitcli.RepoPath{scPathBad}, true},
		{"change id resolves through the snapshot",
			domain.Finding{Code: "c", Entity: scChangeID(91)}, []gitcli.RepoPath{scPathA1}, true},
		{"id-only cycle resolves every member", scCycle(91, 20), []gitcli.RepoPath{scPathB, scPathA1}, true},
		{"adr id resolves", domain.Finding{Code: "c", Entity: domain.EntityRef{Kind: domain.EntityADR, ID: 5}},
			[]gitcli.RepoPath{scPathADR}, true},
		{"learning slug resolves", domain.Finding{Code: "c", Entity: domain.EntityRef{Kind: domain.EntityLearning, Slug: "some-lesson"}},
			[]gitcli.RepoPath{scPathL}, true},
		{"absent related member fails", scDangling(91, scPathA1, 50, "absent"), nil, false},
		{"absent entity id fails", domain.Finding{Code: "c", Entity: scChangeID(50)}, nil, false},
		{"absent adr fails", domain.Finding{Code: "c", Entity: domain.EntityRef{Kind: domain.EntityADR, ID: 6}}, nil, false},
		{"absent learning fails", domain.Finding{Code: "c", Entity: domain.EntityRef{Kind: domain.EntityLearning, Slug: "nope"}}, nil, false},
		{"kind-only entity fails", domain.Finding{Code: "c", Entity: domain.EntityRef{Kind: domain.EntityChange}}, nil, false},
		{"repository entity without a path fails", domain.Finding{Code: "c", Entity: domain.EntityRef{Kind: domain.EntityRepo}}, nil, false},
		{"artifact entity without a path fails", domain.Finding{Code: "c", Entity: domain.EntityRef{Kind: domain.EntityArtifact, Slug: "x"}}, nil, false},
		{"path wins over id", domain.Finding{Code: "c", Entity: scPathRef(50, scPathA1)}, []gitcli.RepoPath{scPathA1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := findingPaths(tc.f, st)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (paths %v)", ok, tc.wantOK, got)
			}
			if tc.wantOK && !slices.Equal(got, tc.want) {
				t.Fatalf("paths = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("ambiguous related member fails", func(t *testing.T) {
		dup := scState(scSnapshot(scChange(20, scPathB), scChange(91, scPathA1), scChange(91, scPathA2)), scBlobs())
		if _, ok := findingPaths(scCycle(20, 91), dup); ok {
			t.Fatal("an ambiguous Related id resolved")
		}
	})
	t.Run("record with no path fails", func(t *testing.T) {
		noPath := scState(scSnapshot(scChange(91, "")), scBlobs())
		if _, ok := findingPaths(domain.Finding{Code: "c", Entity: scChangeID(91)}, noPath); ok {
			t.Fatal("a record with an empty path resolved")
		}
	})
}

func TestFindingRelevant(t *testing.T) {
	before := scState(scBaseSnapshot(), scBlobs())

	t.Run("id-only cycle attributed to A with B in Related is relevant", func(t *testing.T) {
		if !findingRelevant(scCycle(91, 92, 20), scScope(), &before, nil) {
			t.Fatal("a cycle whose non-primary member is B escaped attribution")
		}
	})
	t.Run("finding on unrelated A is not relevant", func(t *testing.T) {
		if findingRelevant(scParseFinding(scPathA1), scScope(), &before, &before) {
			t.Fatal("an unrelated finding was judged relevant")
		}
		if findingRelevant(scCycle(91, 92), scScope(), &before, &before) {
			t.Fatal("an unrelated cycle was judged relevant")
		}
	})
	t.Run("finding on B is relevant", func(t *testing.T) {
		if !findingRelevant(scParseFinding(scPathB), scScope(), &before, nil) {
			t.Fatal("a finding on B was judged unrelated")
		}
	})
	t.Run("unresolvable finding is relevant", func(t *testing.T) {
		if !findingRelevant(scDangling(91, scPathA1, 50, "absent"), scScope(), &before, nil) {
			t.Fatal("an unresolvable finding was judged unrelated")
		}
	})
	t.Run("empty or nil scope makes everything relevant", func(t *testing.T) {
		for _, s := range []map[gitcli.RepoPath]bool{nil, {}} {
			if !findingRelevant(scParseFinding(scPathA1), s, &before, &before) {
				t.Fatal("an empty scope disabled relevance")
			}
		}
	})
	t.Run("no state to resolve against is relevant", func(t *testing.T) {
		if !findingRelevant(scCycle(91, 92), scScope(), nil, nil) {
			t.Fatal("a finding with no state to resolve in was judged unrelated")
		}
	})

	// D is B's dependency and therefore a subject in the before state.
	depScope := map[gitcli.RepoPath]bool{scPathB: true, scPathD: true}
	t.Run("dependency moved in the candidate keeps its before-state obligation", func(t *testing.T) {
		after := scState(scSnapshot(scChange(20, scPathB), scChange(10, scPathD2), scChange(91, scPathA1)), scBlobs())
		f := scCycle(10, 91)
		if findingRelevant(f, depScope, nil, &after) {
			t.Fatal("fixture invalid: the finding must be unrelated in the candidate alone")
		}
		if !findingRelevant(f, depScope, &before, &after) {
			t.Fatal("a before-state dependency obligation was erased by the candidate")
		}
	})
	t.Run("dependency removed in the candidate stays relevant", func(t *testing.T) {
		after := scState(scSnapshot(scChange(20, scPathB), scChange(91, scPathA1)), scBlobs())
		if !findingRelevant(scCycle(10, 91), depScope, &before, &after) {
			t.Fatal("a removed dependency's finding was judged unrelated")
		}
	})
	t.Run("id resolving to a subject only in the candidate is relevant", func(t *testing.T) {
		// 92 lives at A2 before but the candidate relocates it onto B's path.
		after := scState(scSnapshot(scChange(92, scPathB)), scBlobs())
		f := domain.Finding{Code: "c", Entity: scChangeID(92)}
		if findingRelevant(f, scScope(), &before, nil) {
			t.Fatal("fixture invalid: the finding must be unrelated in the before state alone")
		}
		if !findingRelevant(f, scScope(), &before, &after) {
			t.Fatal("a candidate-state subject resolution was ignored")
		}
	})
}

func TestBaselineErrors(t *testing.T) {
	before := scState(scBaseSnapshot(), scBlobs(),
		scParseFinding(scPathBad), scParseFinding(scPathBad), // identical pair: count 2
		scParseFinding(scPathB), // relevant: excluded
		scCycle(91, 92),         // unrelated id-only
		domain.Finding{Code: "w", Severity: domain.SeverityWarning, Entity: scPathRef(93, scPathA3)}, // warning: excluded
	)
	base := baselineErrors(before, scScope())
	if got := base.counts[canonicalFindingKey(scParseFinding(scPathBad))]; got != 2 {
		t.Fatalf("identical pair count = %d, want 2", got)
	}
	if _, ok := base.counts[canonicalFindingKey(scParseFinding(scPathB))]; ok {
		t.Fatal("a relevant error entered the grandfather baseline")
	}
	if len(base.counts) != 2 {
		t.Fatalf("baseline keys = %d, want 2 (%v)", len(base.counts), base.counts)
	}
	if got := base.paths[canonicalFindingKey(scCycle(91, 92))]; !slices.Equal(got, []gitcli.RepoPath{scPathA1, scPathA2}) {
		t.Fatalf("cycle baseline paths = %v", got)
	}
	if empty := baselineErrors(before, nil); len(empty.counts) != 0 {
		t.Fatalf("an empty scope produced a grandfather baseline: %v", empty.counts)
	}
}

func TestScopedAfterErrors(t *testing.T) {
	snap := scBaseSnapshot()
	keys := func(fs []domain.Finding) []string {
		out := make([]string, 0, len(fs))
		for _, f := range fs {
			out = append(out, canonicalFindingKey(f))
		}
		slices.Sort(out)
		return out
	}
	blobsWith := func(path string, id gitcli.ObjectID) map[string]gitcli.ObjectID {
		b := scBlobs()
		if id == "-" {
			delete(b, path)
		} else {
			b[path] = id
		}
		return b
	}

	t.Run("identical unrelated finding on unchanged blobs is grandfathered", func(t *testing.T) {
		before := scState(snap, scBlobs(), scParseFinding(scPathBad), scCycle(91, 92), scDuplicate(93, scPathA3, "2"))
		after := scState(snap, scBlobs(), scParseFinding(scPathBad), scCycle(91, 92), scDuplicate(93, scPathA3, "2"))
		if got := scopedAfterErrors(before, after, scScope()); len(got) != 0 {
			t.Fatalf("grandfathered findings refused: %v", got)
		}
	})
	t.Run("warnings never refuse", func(t *testing.T) {
		w := domain.Finding{Code: "w", Severity: domain.SeverityWarning, Entity: scPathRef(20, scPathB)}
		if got := scopedAfterErrors(scState(snap, scBlobs()), scState(snap, scBlobs(), w), scScope()); len(got) != 0 {
			t.Fatalf("a warning refused: %v", got)
		}
	})
	t.Run("relevant error refuses even when identical in before", func(t *testing.T) {
		f := scParseFinding(scPathB)
		got := scopedAfterErrors(scState(snap, scBlobs(), f), scState(snap, scBlobs(), f), scScope())
		if !slices.Equal(keys(got), keys([]domain.Finding{f})) {
			t.Fatalf("refused = %v, want the B finding", got)
		}
	})

	blobCases := []struct {
		name          string
		before, after map[string]gitcli.ObjectID
	}{
		{"changed blob id refuses", scBlobs(), blobsWith(scPathBad, "bad1")},
		{"overlay-cleared after id refuses", scBlobs(), blobsWith(scPathBad, "")},
		{"missing after id refuses", scBlobs(), blobsWith(scPathBad, "-")},
		{"missing before id refuses", blobsWith(scPathBad, "-"), scBlobs()},
		{"empty before id refuses", blobsWith(scPathBad, ""), blobsWith(scPathBad, "")},
		{"nil blob maps refuse", nil, nil},
	}
	for _, tc := range blobCases {
		t.Run(tc.name, func(t *testing.T) {
			f := scParseFinding(scPathBad)
			got := scopedAfterErrors(scState(snap, tc.before, f), scState(snap, tc.after, f), scScope())
			if !slices.Equal(keys(got), keys([]domain.Finding{f})) {
				t.Fatalf("refused = %v, want the unchanged-key finding on a touched path", got)
			}
		})
	}
	t.Run("one touched member of a multi-path finding refuses", func(t *testing.T) {
		f := scCycle(91, 92)
		got := scopedAfterErrors(scState(snap, scBlobs(), f), scState(snap, blobsWith(scPathA2, "a21"), f), scScope())
		if len(got) != 1 {
			t.Fatalf("refused = %v, want the cycle", got)
		}
	})
	t.Run("count increase refuses exactly the surplus", func(t *testing.T) {
		f := scParseFinding(scPathBad)
		got := scopedAfterErrors(scState(snap, scBlobs(), f, f), scState(snap, scBlobs(), f, f, f), scScope())
		if len(got) != 1 || canonicalFindingKey(got[0]) != canonicalFindingKey(f) {
			t.Fatalf("refused = %v, want exactly one surplus", got)
		}
	})
	t.Run("changed duplicate count refuses", func(t *testing.T) {
		before := scState(snap, scBlobs(), scDuplicate(91, scPathA1, "2"), scDuplicate(91, scPathA2, "2"))
		after := scState(snap, scBlobs(), scDuplicate(91, scPathA1, "3"), scDuplicate(91, scPathA2, "3"))
		if got := scopedAfterErrors(before, after, scScope()); len(got) != 2 {
			t.Fatalf("refused = %v, want both changed-count findings", got)
		}
	})
	t.Run("dangling lookup flip absent to ambiguous refuses", func(t *testing.T) {
		before := scState(snap, scBlobs(), scDangling(91, scPathA1, 50, "absent"))
		after := scState(snap, scBlobs(), scDangling(91, scPathA1, 50, "ambiguous"))
		if got := scopedAfterErrors(before, after, scScope()); len(got) != 1 {
			t.Fatalf("refused = %v, want the flipped dangling finding", got)
		}
	})
	t.Run("disappearance is not a license for a different error", func(t *testing.T) {
		gone := scParseFinding(scPathA1)
		kept := scCycle(93, 94)
		fresh := domain.Finding{Code: "change-status-invalid", Severity: domain.SeverityError, Entity: scPathRef(92, scPathA2), Field: "status"}
		got := scopedAfterErrors(scState(snap, scBlobs(), gone, kept), scState(snap, scBlobs(), kept, fresh), scScope())
		if !slices.Equal(keys(got), keys([]domain.Finding{fresh})) {
			t.Fatalf("refused = %v, want only the new error", got)
		}
	})
	t.Run("compareFindings ties match by full key not position", func(t *testing.T) {
		// fx and fy tie on Code/Entity/Field; only Related differs.
		fx := scCycle(91, 92)
		fy := scCycle(91, 93)
		fz := scCycle(91, 94)
		before := scState(snap, scBlobs(), fx, fy)
		if got := scopedAfterErrors(before, scState(snap, scBlobs(), fy, fy), scScope()); !slices.Equal(keys(got), keys([]domain.Finding{fy})) {
			t.Fatalf("refused = %v, want the second fy only", got)
		}
		if got := scopedAfterErrors(before, scState(snap, scBlobs(), fz, fx), scScope()); !slices.Equal(keys(got), keys([]domain.Finding{fz})) {
			t.Fatalf("refused = %v, want the tie-ordered newcomer fz", got)
		}
	})
	t.Run("empty scope refuses every error", func(t *testing.T) {
		f := scParseFinding(scPathBad)
		for _, s := range []map[gitcli.RepoPath]bool{nil, {}} {
			if got := scopedAfterErrors(scState(snap, scBlobs(), f), scState(snap, scBlobs(), f), s); len(got) != 1 {
				t.Fatalf("empty scope grandfathered: refused = %v", got)
			}
		}
	})
	t.Run("finding moved onto a subject in the candidate refuses", func(t *testing.T) {
		f := scCycle(91, 92)
		after := scState(scSnapshot(scChange(20, scPathB), scChange(91, scPathA1), scChange(92, scPathB)), scBlobs(), f)
		if got := scopedAfterErrors(scState(snap, scBlobs(), f), after, scScope()); len(got) != 1 {
			t.Fatalf("refused = %v, want the candidate-relevant cycle", got)
		}
	})
}
