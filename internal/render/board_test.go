package render_test

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
)

// boardCorpusSnapshot builds a domain.Snapshot from the frozen fixture corpus
// under testdata/board/corpus, exactly the corpus the frozen golden was
// generated over (see testdata/board/PROVENANCE.md): every active/ and archive/
// Markdown file is parsed with document.Parse and fed to
// repository.BuildSnapshot, so the renderer reads the same records the Bash
// script did.
func boardCorpusSnapshot(t *testing.T) domain.Snapshot {
	t.Helper()
	root := filepath.Join("testdata", "board", "corpus")
	parts := []struct {
		sub string
		loc domain.RecordLocation
	}{
		{"active", domain.LocationActive},
		{"archive", domain.LocationArchive},
	}
	var docs []repository.InputDocument
	for _, part := range parts {
		dir := filepath.Join(root, part.sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read corpus dir %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read corpus file %s: %v", e.Name(), err)
			}
			doc, err := document.Parse(data)
			if err != nil {
				t.Fatalf("parse corpus file %s: %v", e.Name(), err)
			}
			docs = append(docs, repository.InputDocument{
				Kind:     repository.KindChange,
				Location: part.loc,
				Path:     path.Join("docs/changes", part.sub, e.Name()),
				Document: doc,
			})
		}
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{Documents: docs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return build.Snapshot
}

// TestBoardGolden is the drift guard: the renderer must reproduce the frozen
// Bash-era board bytes over the fixture corpus.
func TestBoardGolden(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "board", "board.golden"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got, err := render.Board(render.BoardInput{Snapshot: boardCorpusSnapshot(t)})
	if err != nil {
		t.Fatalf("Board: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("board mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBoardDeterministic(t *testing.T) {
	snap := boardCorpusSnapshot(t)
	a, err := render.Board(render.BoardInput{Snapshot: snap})
	if err != nil {
		t.Fatal(err)
	}
	b, err := render.Board(render.BoardInput{Snapshot: snap})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("non-deterministic board:\n%s\n---\n%s", a, b)
	}
}

// proposedChange builds a proposed active change with the given id/slug and
// optional overrides applied by the caller.
func proposedChange(id int, slug, title string) domain.ChangeSpec {
	return domain.ChangeSpec{
		ID:       domain.ChangeID(id),
		Slug:     slug,
		Title:    title,
		Status:   domain.StatusProposed,
		Location: domain.LocationActive,
		Path:     path.Join("docs/changes/active", fmtID(id)+"-"+slug+".md"),
	}
}

func fmtID(id int) string {
	s := "0000" + itoa(id)
	return s[len(s)-4:]
}

func itoa(id int) string {
	if id == 0 {
		return "0"
	}
	var digits []byte
	for id > 0 {
		digits = append([]byte{byte('0' + id%10)}, digits...)
		id /= 10
	}
	return string(digits)
}

func boardFrom(t *testing.T, changes ...domain.Change) []byte {
	t.Helper()
	return boardFromFacts(t, domain.BranchFacts{}, changes...)
}

func boardFromFacts(t *testing.T, facts domain.BranchFacts, changes ...domain.Change) []byte {
	t.Helper()
	snap := domain.NewSnapshot(domain.SnapshotSpec{Changes: changes})
	out, err := render.Board(render.BoardInput{Snapshot: snap, Facts: facts})
	if err != nil {
		t.Fatalf("Board: %v", err)
	}
	return out
}

// TestBoardClassifyEveryBucket pins boardClassify's precedence: exactly one
// rendered section per active change, including the two precedence edges
// (finalize-blocked implemented → Blocked over Built; spec-backed build-ready →
// Groomed while trivial-build-ready-without-spec stays Proposed).
func TestBoardClassifyEveryBucket(t *testing.T) {
	blocked := domain.NewChange(domain.ChangeSpec{
		ID: 1, Slug: "blk", Title: "Blocked", Status: domain.StatusBlocked,
		Location: domain.LocationActive, Path: "docs/changes/active/0001-blk.md",
	})
	finBlocked := domain.NewChange(domain.ChangeSpec{
		ID: 2, Slug: "fin", Title: "Fin blocked", Status: domain.StatusImplemented,
		HasFinalizeBlocked: true,
		Location:           domain.LocationActive, Path: "docs/changes/active/0002-fin.md",
	})
	impl := domain.NewChange(domain.ChangeSpec{
		ID: 3, Slug: "impl", Title: "Impl", Status: domain.StatusImplemented,
		Location: domain.LocationActive, Path: "docs/changes/active/0003-impl.md",
	})
	stacked := domain.NewChange(domain.ChangeSpec{
		ID: 4, Slug: "stk", Title: "Stacked", Status: domain.StatusStackedMerged,
		Location: domain.LocationActive, Path: "docs/changes/active/0004-stk.md",
	})
	inprog := domain.NewChange(domain.ChangeSpec{
		ID: 5, Slug: "wip", Title: "WIP", Status: domain.StatusInProgress,
		Location: domain.LocationActive, Path: "docs/changes/active/0005-wip.md",
	})

	groomedSpec := proposedChange(6, "groomed", "Groomed")
	groomedSpec.Spec = optString("docs/superpowers/specs/groomed-design.md")
	groomed := domain.NewChange(groomedSpec)

	// proposed + spec + unmet dependency → still proposed (readiness ≠ build-ready).
	depTarget := domain.NewChange(proposedChange(100, "dep", "Dep"))
	waiterSpec := proposedChange(7, "waiter", "Waiter")
	waiterSpec.Spec = optString("docs/superpowers/specs/waiter-design.md")
	waiterSpec.DependsOn = []domain.ChangeID{100}
	waiter := domain.NewChange(waiterSpec)

	// proposed + spec + unresolved stack base (zero Facts) → proposed.
	parent := domain.NewChange(domain.ChangeSpec{
		ID: 9, Slug: "parent", Title: "Parent", Status: domain.StatusInProgress,
		Branch:   domain.OptionalString{State: domain.FieldPresent, Value: "feat/parent"},
		Location: domain.LocationActive, Path: "docs/changes/active/0009-parent.md",
	})
	stackChildSpec := proposedChange(8, "child", "Child")
	stackChildSpec.Spec = optString("docs/superpowers/specs/child-design.md")
	stackChildSpec.StackedOn = domain.OptionalInt{State: domain.FieldPresent, Value: 9}
	stackChild := domain.NewChange(stackChildSpec)

	// proposed + trivial, no spec (build-ready) → proposed, because the groomed
	// label means build-ready AND spec-backed.
	trivialSpec := proposedChange(10, "triv", "Trivial")
	trivialSpec.Trivial = true
	trivial := domain.NewChange(trivialSpec)

	needsBrainstorm := domain.NewChange(proposedChange(11, "needs", "Needs brainstorm"))

	agBlockedSpec := proposedChange(12, "agb", "AG blocked")
	agBlockedSpec.HasAutoGroomBlocked = true
	agBlocked := domain.NewChange(agBlockedSpec)

	deferred := domain.NewChange(domain.ChangeSpec{
		ID: 13, Slug: "def", Title: "Deferred", Status: domain.StatusDeferred,
		Location: domain.LocationActive, Path: "docs/changes/active/0013-def.md",
	})

	cases := []struct {
		name   string
		target domain.Change
		extra  []domain.Change
		want   render.BoardSection
	}{
		{"blocked lifecycle", blocked, nil, render.BoardSectionBlocked},
		{"implemented finalize-blocked", finBlocked, nil, render.BoardSectionBlocked},
		{"implemented healthy", impl, nil, render.BoardSectionBuilt},
		{"stacked-merged", stacked, nil, render.BoardSectionBuilt},
		{"in-progress", inprog, nil, render.BoardSectionInProgress},
		{"proposed spec build-ready", groomed, nil, render.BoardSectionGroomed},
		{"proposed spec unmet dependency", waiter, []domain.Change{depTarget}, render.BoardSectionProposed},
		{"proposed spec unresolved stack base", stackChild, []domain.Change{parent}, render.BoardSectionProposed},
		{"proposed trivial no spec", trivial, nil, render.BoardSectionProposed},
		{"proposed needs-brainstorm", needsBrainstorm, nil, render.BoardSectionProposed},
		{"proposed auto-groom-blocked", agBlocked, nil, render.BoardSectionProposed},
		{"deferred lifecycle", deferred, nil, render.BoardSectionDeferred},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changes := append([]domain.Change{tc.target}, tc.extra...)
			snap := domain.NewSnapshot(domain.SnapshotSpec{Changes: changes})
			got, err := render.BoardClassifyForTest(render.BoardInput{Snapshot: snap}, tc.target)
			if err != nil {
				t.Fatalf("classify: %v", err)
			}
			if got != tc.want {
				t.Fatalf("classify = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBoardClassifyNonActiveStatusErrors pins the fail-closed posture: a
// terminal (archive) status has no rendered section and classify errors.
func TestBoardClassifyNonActiveStatusErrors(t *testing.T) {
	done := domain.NewChange(domain.ChangeSpec{
		ID: 14, Slug: "done", Title: "Done", Status: domain.StatusDone,
		Location: domain.LocationArchive, Path: "docs/changes/archive/2026-01-01-0014-done.md",
	})
	snap := domain.NewSnapshot(domain.SnapshotSpec{Changes: []domain.Change{done}})
	if _, err := render.BoardClassifyForTest(render.BoardInput{Snapshot: snap}, done); err == nil {
		t.Fatalf("expected error classifying a non-active status")
	}
}

// TestBoardDefaultPresentation pins the built-in presentation: the default
// permutation with updated desc on every section.
func TestBoardDefaultPresentation(t *testing.T) {
	p := render.DefaultBoardPresentation()
	want := []render.BoardSection{
		render.BoardSectionInProgress, render.BoardSectionBuilt, render.BoardSectionBlocked,
		render.BoardSectionGroomed, render.BoardSectionProposed, render.BoardSectionDeferred,
	}
	if len(p.SectionOrder) != len(want) {
		t.Fatalf("section order = %v, want %v", p.SectionOrder, want)
	}
	for i, s := range want {
		if p.SectionOrder[i] != s {
			t.Fatalf("section order[%d] = %q, want %q", i, p.SectionOrder[i], s)
		}
	}
	for _, s := range want {
		srt, ok := p.Sorting[s]
		if !ok {
			t.Fatalf("missing default sort for %q", s)
		}
		if srt.By != "updated" || srt.Direction != "desc" {
			t.Fatalf("%q default sort = %q %q, want updated desc", s, srt.By, srt.Direction)
		}
	}
}

// TestBoardActiveSectionSortsByNumericID pins the section sort as NUMERIC
// ascending, not string collation: id 10 must follow id 2, and unset
// priority/type render deterministically (empty backticks / `untyped`) without
// perturbing that order.
func TestBoardActiveSectionSortsByNumericID(t *testing.T) {
	// id 10 has a stored priority/type; id 2 has neither.
	two := proposedChange(2, "two", "Two")
	two.Spec = optString("docs/superpowers/specs/two-design.md") // build-ready
	ten := proposedChange(10, "ten", "Ten")
	ten.Priority = domain.PriorityHigh
	ten.RawPriority = "high"
	ten.Type = "feat"
	ten.Spec = optString("docs/superpowers/specs/ten-design.md")

	out := string(boardFrom(t, domain.NewChange(ten), domain.NewChange(two)))

	i2 := strings.Index(out, "[0002](active/")
	i10 := strings.Index(out, "[0010](active/")
	if i2 < 0 || i10 < 0 {
		t.Fatalf("expected both rows present:\n%s", out)
	}
	if i2 > i10 {
		t.Fatalf("numeric sort violated: id 2 rendered after id 10:\n%s", out)
	}
	// id 2 carries no priority and no type: empty backticks and `untyped`.
	if !strings.Contains(out, "| [0002](active/0002-two.md) | Two | `` | `untyped` | build-ready |") {
		t.Fatalf("unset priority/type not rendered deterministically:\n%s", out)
	}
}

// TestBoardReadinessNeedsMerge: a proposed change depending on an IMPLEMENTED
// change reports the needs-your-merge waiting cell — a band the golden corpus
// (whose only dependency is done or not-built) does not exercise.
func TestBoardReadinessNeedsMerge(t *testing.T) {
	impl := domain.NewChange(domain.ChangeSpec{
		ID: 5, Slug: "impl", Title: "Impl", Status: domain.StatusImplemented,
		Location: domain.LocationActive, Path: "docs/changes/active/0005-impl.md",
	})
	waiter := proposedChange(6, "waiter", "Waiter")
	waiter.DependsOn = []domain.ChangeID{5}
	waiter.Spec = optString("docs/superpowers/specs/waiter-design.md")

	out := string(boardFrom(t, impl, domain.NewChange(waiter)))
	if !strings.Contains(out, "⏳ waiting on #5 — needs your merge") {
		t.Fatalf("needs-your-merge cell not rendered:\n%s", out)
	}
}

// TestBoardReadinessAutoGroomBlocked: a proposed, spec-less change carrying the
// retained auto-groom-blocked marker reports the human call to action.
func TestBoardReadinessAutoGroomBlocked(t *testing.T) {
	c := proposedChange(7, "blocked-groom", "Blocked groom")
	c.HasAutoGroomBlocked = true // no spec, not trivial
	out := string(boardFrom(t, domain.NewChange(c)))
	if !strings.Contains(out, "| Blocked groom | `` | `untyped` | auto-groom blocked — needs you |") {
		t.Fatalf("auto-groom-blocked cell not rendered:\n%s", out)
	}
}

// TestBoardReadinessStackBaseUnresolved: a build-ready-but-for-its-stack change
// whose parent branch is absent from the facts reports the padded immediate
// parent, not the ancestor the resolver stopped at.
func TestBoardReadinessStackBaseUnresolved(t *testing.T) {
	parent := domain.NewChange(domain.ChangeSpec{
		ID: 8, Slug: "parent", Title: "Parent", Status: domain.StatusInProgress,
		Branch:   domain.OptionalString{State: domain.FieldPresent, Value: "feat/parent"},
		Spec:     optString("docs/superpowers/specs/parent-design.md"),
		Location: domain.LocationActive, Path: "docs/changes/active/0008-parent.md",
	})
	child := proposedChange(9, "child", "Child")
	child.Spec = optString("docs/superpowers/specs/child-design.md")
	child.StackedOn = domain.OptionalInt{State: domain.FieldPresent, Value: 8}

	// Empty facts: the parent's branch does not exist on the remote.
	out := string(boardFromFacts(t, domain.BranchFacts{}, parent, domain.NewChange(child)))
	if !strings.Contains(out, "⏳ waiting on #0008 — stack base not built") {
		t.Fatalf("stack-base-unresolved cell not rendered:\n%s", out)
	}
}

// TestBoardStackedMergedSection: the stacked-merged section header, emoji, and
// Stack cell — a section the golden corpus does not exercise.
func TestBoardStackedMergedSection(t *testing.T) {
	c := domain.NewChange(domain.ChangeSpec{
		ID: 11, Slug: "merged-child", Title: "Merged child", Status: domain.StatusStackedMerged,
		StackedOn: domain.OptionalInt{State: domain.FieldPresent, Value: 8},
		PR:        domain.OptionalString{State: domain.FieldPresent, Value: "https://github.com/danielhanold/docket/pull/99"},
		Location:  domain.LocationActive, Path: "docs/changes/active/0011-merged-child.md",
	})
	out := string(boardFrom(t, c))
	if !strings.Contains(out, "## 🪆 Stacked-merged (1)") {
		t.Fatalf("stacked-merged section header missing:\n%s", out)
	}
	if !strings.Contains(out, "| [#99](https://github.com/danielhanold/docket/pull/99) | merged into #0008 |") {
		t.Fatalf("stacked-merged PR/Stack cells missing:\n%s", out)
	}
}

// mustDate parses a YYYY-MM-DD test date into a present OptionalTime.
func mustDate(t *testing.T, ymd string) domain.OptionalTime {
	t.Helper()
	tm, err := time.Parse("2006-01-02", ymd)
	if err != nil {
		t.Fatalf("parse test date %q: %v", ymd, err)
	}
	return domain.OptionalTime{State: domain.FieldPresent, Value: tm, Raw: ymd}
}

// datedChange builds a proposed active change carrying present updated/created
// dates, for exercising the per-section comparator directly.
func datedChange(t *testing.T, id int, updated, created string) domain.Change {
	t.Helper()
	spec := proposedChange(id, "s"+fmtID(id), "Change "+fmtID(id))
	spec.Updated = mustDate(t, updated)
	spec.Created = mustDate(t, created)
	return domain.NewChange(spec)
}

// ids extracts the numeric ids of rows in slice order.
func ids(rows []domain.Change) []int {
	out := make([]int, len(rows))
	for i, c := range rows {
		out[i] = int(c.ID())
	}
	return out
}

func idsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSortBoardSectionAllFieldsBothDirections pins the primary key for every
// (field × direction) combination. The five fixtures are chosen so id-order,
// updated-order and created-order are three DIFFERENT permutations — a
// comparator reading the wrong field cannot pass by coincidence.
func TestSortBoardSectionAllFieldsBothDirections(t *testing.T) {
	// id : updated    : created
	//  1 : 2026-01-05 : 2026-01-03
	//  2 : 2026-01-01 : 2026-01-05
	//  3 : 2026-01-04 : 2026-01-01
	//  4 : 2026-01-02 : 2026-01-04
	//  5 : 2026-01-03 : 2026-01-02
	base := []domain.Change{
		datedChange(t, 1, "2026-01-05", "2026-01-03"),
		datedChange(t, 2, "2026-01-01", "2026-01-05"),
		datedChange(t, 3, "2026-01-04", "2026-01-01"),
		datedChange(t, 4, "2026-01-02", "2026-01-04"),
		datedChange(t, 5, "2026-01-03", "2026-01-02"),
	}
	cases := []struct {
		name string
		by   render.BoardSortKey
		dir  render.BoardDirection
		want []int
	}{
		{"id asc", render.BoardSortKeyID, render.BoardDirectionAsc, []int{1, 2, 3, 4, 5}},
		{"id desc", render.BoardSortKeyID, render.BoardDirectionDesc, []int{5, 4, 3, 2, 1}},
		{"updated asc", render.BoardSortKeyUpdated, render.BoardDirectionAsc, []int{2, 4, 5, 3, 1}},
		{"updated desc", render.BoardSortKeyUpdated, render.BoardDirectionDesc, []int{1, 3, 5, 4, 2}},
		{"created asc", render.BoardSortKeyCreated, render.BoardDirectionAsc, []int{3, 5, 1, 4, 2}},
		{"created desc", render.BoardSortKeyCreated, render.BoardDirectionDesc, []int{2, 4, 1, 5, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := append([]domain.Change(nil), base...)
			render.SortBoardSectionForTest(rows, render.BoardSort{By: tc.by, Direction: tc.dir})
			if got := ids(rows); !idsEqual(got, tc.want) {
				t.Fatalf("order = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSortBoardSectionSameDateTiesFollowDirection pins the tie-break: rows
// sharing the primary date order by numeric id in the SAME direction as the
// primary sort.
func TestSortBoardSectionSameDateTiesFollowDirection(t *testing.T) {
	// ids 3 and 7 share 2026-08-30; id 5 is later.
	rows := func() []domain.Change {
		return []domain.Change{
			datedChange(t, 3, "2026-08-30", "2026-01-01"),
			datedChange(t, 7, "2026-08-30", "2026-01-01"),
			datedChange(t, 5, "2026-08-31", "2026-01-01"),
		}
	}

	desc := rows()
	render.SortBoardSectionForTest(desc, render.BoardSort{By: render.BoardSortKeyUpdated, Direction: render.BoardDirectionDesc})
	// desc: newest (id 5) first, then the tie band higher-id first.
	if got := ids(desc); !idsEqual(got, []int{5, 7, 3}) {
		t.Fatalf("desc tie order = %v, want [5 7 3]", got)
	}

	asc := rows()
	render.SortBoardSectionForTest(asc, render.BoardSort{By: render.BoardSortKeyUpdated, Direction: render.BoardDirectionAsc})
	// asc: the tie band lower-id first, then the later date.
	if got := ids(asc); !idsEqual(got, []int{3, 7, 5}) {
		t.Fatalf("asc tie order = %v, want [3 7 5]", got)
	}
}

// TestSortBoardSectionUnknownDatesSortLast pins that rows with an absent or
// malformed primary date land after every dated row in BOTH directions, and
// among themselves order by id in the configured direction.
func TestSortBoardSectionUnknownDatesSortLast(t *testing.T) {
	// A dated row, plus one absent-date and one malformed-date row.
	makeRows := func() []domain.Change {
		dated := datedChange(t, 4, "2026-05-01", "2026-01-01")

		absentSpec := proposedChange(2, "absent", "Absent")
		absentSpec.Updated = domain.OptionalTime{State: domain.FieldAbsent}
		absent := domain.NewChange(absentSpec)

		malformedSpec := proposedChange(9, "malformed", "Malformed")
		malformedSpec.Updated = domain.OptionalTime{State: domain.FieldMalformed, Raw: "not-a-date"}
		malformed := domain.NewChange(malformedSpec)

		// Deliberately not id-sorted on input.
		return []domain.Change{malformed, dated, absent}
	}

	desc := makeRows()
	render.SortBoardSectionForTest(desc, render.BoardSort{By: render.BoardSortKeyUpdated, Direction: render.BoardDirectionDesc})
	// Dated row first; then unknowns by id descending (9 before 2).
	if got := ids(desc); !idsEqual(got, []int{4, 9, 2}) {
		t.Fatalf("desc unknown-date order = %v, want [4 9 2]", got)
	}

	asc := makeRows()
	render.SortBoardSectionForTest(asc, render.BoardSort{By: render.BoardSortKeyUpdated, Direction: render.BoardDirectionAsc})
	// Dated row STILL first (unknowns after valid dates regardless of
	// direction); then unknowns by id ascending (2 before 9).
	if got := ids(asc); !idsEqual(got, []int{4, 2, 9}) {
		t.Fatalf("asc unknown-date order = %v, want [4 2 9]", got)
	}
}

// TestSortBoardSectionIgnoresArrivalOrder pins that the comparator is total:
// the output is identical no matter how the input slice arrives.
func TestSortBoardSectionIgnoresArrivalOrder(t *testing.T) {
	build := []domain.Change{
		datedChange(t, 1, "2026-01-05", "2026-01-03"),
		datedChange(t, 2, "2026-01-01", "2026-01-05"),
		datedChange(t, 3, "2026-01-04", "2026-01-01"),
		datedChange(t, 4, "2026-01-02", "2026-01-04"),
		datedChange(t, 5, "2026-01-03", "2026-01-02"),
	}
	sort := render.BoardSort{By: render.BoardSortKeyUpdated, Direction: render.BoardDirectionDesc}

	// Canonical order from the natural input.
	canon := append([]domain.Change(nil), build...)
	render.SortBoardSectionForTest(canon, sort)
	want := ids(canon)

	// Reversed and rotated inputs must yield the same output.
	reversed := make([]domain.Change, len(build))
	for i, c := range build {
		reversed[len(build)-1-i] = c
	}
	render.SortBoardSectionForTest(reversed, sort)
	if got := ids(reversed); !idsEqual(got, want) {
		t.Fatalf("reversed input order = %v, want %v", got, want)
	}

	rotated := append(append([]domain.Change(nil), build[2:]...), build[:2]...)
	render.SortBoardSectionForTest(rotated, sort)
	if got := ids(rotated); !idsEqual(got, want) {
		t.Fatalf("rotated input order = %v, want %v", got, want)
	}
}
