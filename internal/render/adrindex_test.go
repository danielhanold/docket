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

// adrIndexCorpusSnapshot builds a domain.Snapshot from the frozen fixture ADR
// set under testdata/adrindex/adrs, exactly the corpus the frozen golden was
// generated over (see testdata/adrindex/PROVENANCE.md): every Markdown ADR file
// is parsed with document.Parse and fed to repository.BuildSnapshot as a
// KindADR/LocationLedger record, so the renderer reads the same records the Bash
// script did.
func adrIndexCorpusSnapshot(t *testing.T) domain.Snapshot {
	t.Helper()
	dir := filepath.Join("testdata", "adrindex", "adrs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir %s: %v", dir, err)
	}
	var docs []repository.InputDocument
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
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
			Kind:     repository.KindADR,
			Location: domain.LocationLedger,
			Path:     path.Join("docs/adrs", e.Name()),
			Document: doc,
		})
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{Documents: docs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return build.Snapshot
}

// TestADRIndexGolden is the drift guard: the renderer must reproduce the frozen
// Bash-era index bytes over the fixture ADR set.
func TestADRIndexGolden(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "adrindex", "index.golden"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got, err := render.ADRIndex(adrIndexCorpusSnapshot(t))
	if err != nil {
		t.Fatalf("ADRIndex: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("adr index mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestADRIndexEmptyGroupsRenderNone: an empty candidate ADR set renders every
// group's "_None._" placeholder — a band the golden corpus (whose every group is
// non-empty) does not exercise.
func TestADRIndexEmptyGroupsRenderNone(t *testing.T) {
	got, err := render.ADRIndex(domain.NewSnapshot(domain.SnapshotSpec{}))
	if err != nil {
		t.Fatalf("ADRIndex: %v", err)
	}
	want := "# Architecture Decision Records\n\n" +
		"Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.\n" +
		"\n## Active\n\n_None._\n" +
		"\n## Superseded / Reversed\n\n_None._\n" +
		"\n## Deprecated\n\n_None._\n"
	if string(got) != want {
		t.Fatalf("empty-group index mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestADRIndexDeterministic: equal input yields byte-identical output.
func TestADRIndexDeterministic(t *testing.T) {
	snap := adrIndexCorpusSnapshot(t)
	a, err := render.ADRIndex(snap)
	if err != nil {
		t.Fatal(err)
	}
	b, err := render.ADRIndex(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("non-deterministic adr index:\n%s\n---\n%s", a, b)
	}
}
