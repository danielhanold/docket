package render_test

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

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
