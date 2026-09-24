package app

import (
	"fmt"
	"slices"
	"testing"

	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// scopeChangeBlob is a change record whose frontmatter carries exactly the
// structural/associative edges a subject-scope test needs.
func scopeChangeBlob(id int, slug, extra string) StatusBlob {
	return changeBlob(id, slug, "feat", "high", extra)
}

// scopeArchivedBlob is a done change record at its archive path, the shape a
// closeout candidate state holds after it moves the root active/ -> archive/.
func scopeArchivedBlob(id int, slug, extra string) StatusBlob {
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: done\npriority: high\ntype: feat\ncreated: 2026-01-02\n%s---\n\nBody of %d.\n",
		id, slug, id, extra, id)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationArchive,
		Path:     fmt.Sprintf("docs/changes/archive/2026-08-16-%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobarchive%04d", id),
		Data:     []byte(fm),
	}
}

// scopeState builds the LoadedState the engine would hand a resolver for the
// given corpus: only the snapshot matters to subject resolution.
func scopeState(t *testing.T, blobs ...StatusBlob) transaction.LoadedState {
	t.Helper()
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: planningTestConfig(nil), Documents: inputs})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	return transaction.LoadedState{Snapshot: build.Snapshot, Report: build.Report}
}

// resolveScope runs the scope's resolver against st and returns the sorted
// resolved paths.
func resolveScope(t *testing.T, scope *transaction.ValidationScope, st transaction.LoadedState) []string {
	t.Helper()
	if scope == nil || scope.Subjects == nil {
		t.Fatal("scope has no resolver")
	}
	set, err := scope.Subjects(st)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var out []string
	for p, ok := range set {
		if ok {
			out = append(out, string(p))
		}
	}
	slices.Sort(out)
	return out
}

func scopeRecPath(id int, slug string) string {
	return fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug)
}

// scopeFixture is root B (20) depending on 10, stacked on 5 (itself stacked on
// 3), with descendant 30 stacked on B, and an unrelated A (99) that B cites
// only associatively (related, discovered_from).
func scopeFixture() []StatusBlob {
	return []StatusBlob{
		scopeChangeBlob(3, "grand", ""),
		scopeChangeBlob(5, "parent", "stacked_on: 3\n"),
		scopeChangeBlob(10, "dep", ""),
		scopeChangeBlob(20, "root", "depends_on: [10]\nstacked_on: 5\nrelated: [99]\ndiscovered_from: [99]\n"),
		scopeChangeBlob(30, "child", "stacked_on: 20\n"),
		scopeChangeBlob(99, "unrelated", ""),
	}
}

func TestChangeScopeStructuralClosure(t *testing.T) {
	st := scopeState(t, scopeFixture()...)
	got := resolveScope(t, changeScope(20, scopeRecPath(20, "root"), false), st)
	want := []string{
		scopeRecPath(3, "grand"),
		scopeRecPath(5, "parent"),
		scopeRecPath(10, "dep"),
		scopeRecPath(20, "root"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("resolved set = %v, want %v", got, want)
	}
}

// TestChangeScopeExcludesAssociativeReferences is the ADR-0093 boundary: a
// related/discovered_from citation never pulls the cited record into scope.
func TestChangeScopeExcludesAssociativeReferences(t *testing.T) {
	st := scopeState(t, scopeFixture()...)
	for _, withDesc := range []bool{false, true} {
		got := resolveScope(t, changeScope(20, scopeRecPath(20, "root"), withDesc), st)
		if slices.Contains(got, scopeRecPath(99, "unrelated")) {
			t.Fatalf("withDescendants=%v: associative citation 99 resolved into scope: %v", withDesc, got)
		}
	}
}

func TestChangeScopeDescendantsOnlyWhenRequested(t *testing.T) {
	st := scopeState(t, scopeFixture()...)
	child := scopeRecPath(30, "child")
	without := resolveScope(t, changeScope(20, scopeRecPath(20, "root"), false), st)
	if slices.Contains(without, child) {
		t.Fatalf("descendant in scope without withDescendants: %v", without)
	}
	with := resolveScope(t, changeScope(20, scopeRecPath(20, "root"), true), st)
	if !slices.Contains(with, child) {
		t.Fatalf("descendant missing with withDescendants: %v", with)
	}
}

// TestChangeScopeResolvesRootByIdentity: mid-closeout the candidate holds B at
// its archive path while recPath still names active/; the resolver must find B
// by id and carry both paths plus B's structural closure from the archived
// record.
func TestChangeScopeResolvesRootByIdentity(t *testing.T) {
	archived := scopeArchivedBlob(20, "root", "depends_on: [10]\n")
	st := scopeState(t, scopeChangeBlob(10, "dep", ""), archived, scopeChangeBlob(99, "unrelated", ""))
	got := resolveScope(t, changeScope(20, scopeRecPath(20, "root"), false), st)
	for _, p := range []string{scopeRecPath(20, "root"), archived.Path, scopeRecPath(10, "dep")} {
		if !slices.Contains(got, p) {
			t.Fatalf("resolved set %v missing %s", got, p)
		}
	}
	if slices.Contains(got, scopeRecPath(99, "unrelated")) {
		t.Fatalf("unrelated record in scope: %v", got)
	}
}

func TestChangeScopeExtrasPassThroughAndNeverEmpty(t *testing.T) {
	st := scopeState(t)
	rec := scopeRecPath(20, "root")
	got := resolveScope(t, changeScope(20, rec, false, "docs/adrs/0007-x.md", "docs/adrs/README.md", ""), st)
	want := []string{"docs/adrs/0007-x.md", "docs/adrs/README.md", rec}
	if !slices.Equal(got, want) {
		t.Fatalf("empty-snapshot resolved set = %v, want %v", got, want)
	}
}

// TestChangeScopeDuplicateRootIDsAllInScope: an ambiguous root id picks no
// winner, so every record carrying it is a subject (fail closed).
func TestChangeScopeDuplicateRootIDsAllInScope(t *testing.T) {
	dup := scopeChangeBlob(20, "twin", "")
	st := scopeState(t, scopeChangeBlob(20, "root", ""), dup)
	got := resolveScope(t, changeScope(20, scopeRecPath(20, "root"), false), st)
	if !slices.Contains(got, dup.Path) {
		t.Fatalf("duplicate-id record missing from scope: %v", got)
	}
}

func TestADRScopeResolvesOwnTargetsAndIndex(t *testing.T) {
	st := scopeState(t, adrBlob(1, "old"), adrBlob(2, "unrelated"))
	got := resolveScope(t, adrScope("docs/adrs/0003-new.md", []string{"docs/adrs/0001-old.md"}, "docs/adrs/README.md"), st)
	want := []string{"docs/adrs/0001-old.md", "docs/adrs/0003-new.md", "docs/adrs/README.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("adr scope = %v, want %v", got, want)
	}
}
