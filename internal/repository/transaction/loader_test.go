package transaction

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
)

// testLoader is the engine tests' StateLoader. It is backed by the landed pure
// read model — document.Parse plus repository.BuildSnapshot — never a second
// production composer. Load lists docs/ through the supplied Tree, reads each
// record's exact bytes, parses and classifies it by directory (as a real composer
// would), and builds one snapshot. ValidateEvolution delegates to
// repository.ValidateEvolution with the exact source bytes of both states.
type testLoader struct{}

// docsPrefix scopes the loader's tree read to the metadata records; everything
// outside docs/ (config, code, ignores) is not a record this loader decodes.
const docsPrefix gitcli.RepoPath = "docs"

// Load reads and validates the complete state visible through t. A tree/parse
// failure is a Go error (a loader failure, distinct from a domain refusal); a
// domain-invalid corpus is reported through the returned report, not an error.
func (testLoader) Load(ctx context.Context, t Tree) (LoadedState, error) {
	entries, err := t.ListTree(ctx, []gitcli.RepoPath{docsPrefix})
	if err != nil {
		return LoadedState{}, fmt.Errorf("testLoader: listing docs: %w", err)
	}

	paths := make([]gitcli.RepoPath, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(string(e.Path), ".md") {
			paths = append(paths, e.Path)
		}
	}
	blobs, err := t.ReadBlobs(ctx, paths)
	if err != nil {
		return LoadedState{}, fmt.Errorf("testLoader: reading records: %w", err)
	}
	if len(blobs) != len(paths) {
		return LoadedState{}, fmt.Errorf("testLoader: read %d blobs for %d paths", len(blobs), len(paths))
	}

	in := repository.BuildInput{Config: loaderConfig()}
	documents := make(map[string]document.Document, len(blobs))
	sources := make(map[string][]byte, len(blobs))
	for _, b := range blobs {
		rel := string(b.Path)
		if !b.Found {
			return LoadedState{}, fmt.Errorf("testLoader: accounted path %q vanished", rel)
		}
		doc, perr := document.Parse(b.Blob.Bytes)
		if perr != nil {
			return LoadedState{}, fmt.Errorf("testLoader: parsing %q: %w", rel, perr)
		}
		kind, location := classifyRecordPath(rel)
		in.Documents = append(in.Documents, repository.InputDocument{
			Kind: kind, Location: location, Path: rel, Document: doc,
		})
		documents[rel] = doc
		sources[rel] = append([]byte(nil), b.Blob.Bytes...)
	}

	result, err := repository.BuildSnapshot(in)
	if err != nil {
		return LoadedState{}, fmt.Errorf("testLoader: building snapshot: %w", err)
	}
	return LoadedState{
		Snapshot:  result.Snapshot,
		Report:    result.Report,
		Documents: documents,
		Sources:   sources,
	}, nil
}

// ValidateEvolution runs the landed before→after rules over the two states' exact
// source bytes.
func (testLoader) ValidateEvolution(before, after LoadedState) []domain.Finding {
	return repository.ValidateEvolution(repository.EvolutionInput{
		Before:        repository.BuildResult{Snapshot: before.Snapshot, Report: before.Report},
		After:         repository.BuildResult{Snapshot: after.Snapshot, Report: after.Report},
		BeforeSources: before.Sources,
		AfterSources:  after.Sources,
	})
}

// classifyRecordPath maps a repository-relative record path to the kind and
// location a composer would declare for it, mirroring the corpus classification in
// internal/repository.
func classifyRecordPath(rel string) (repository.RecordKind, repository.RecordLocation) {
	switch {
	case strings.HasPrefix(rel, "docs/adrs/"):
		return repository.KindADR, repository.LocationLedger
	case strings.HasPrefix(rel, "docs/changes/learnings/"):
		return repository.KindLearning, repository.LocationLedger
	case strings.HasPrefix(rel, "docs/changes/archive/"):
		return repository.KindChange, repository.LocationArchive
	default:
		return repository.KindChange, repository.LocationActive
	}
}

// loaderConfig is a resolved configuration carrying the four leaves repository
// policy may consult, matching internal/repository's own test config.
func loaderConfig() config.Effective {
	var cfg config.Effective
	cfg.IntegrationBranch.Value = "main"
	cfg.ChangeTypes.Value = []string{"feat", "fix", "chore", "docs"}
	cfg.Reclaim.LeaseTTL.Value = 24
	cfg.Learnings.Enabled.Value = true
	return cfg
}

// compile-time interface satisfaction.
var _ StateLoader = testLoader{}

// TestLoaderBuildsCleanStateFromCorpus proves the loader turns the harness corpus
// into a complete, error-free LoadedState reading through a real base tree.
func TestLoaderBuildsCleanStateFromCorpus(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	ctx := context.Background()

	rev, err := client.FetchBranch(ctx, repo, "origin", r.Target)
	if err != nil {
		t.Fatalf("FetchBranch: %v", err)
	}
	src, err := client.OpenObjectSource(ctx, repo, rev)
	if err != nil {
		t.Fatalf("OpenObjectSource: %v", err)
	}

	st, err := testLoader{}.Load(ctx, newBaseTree(src))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Report.HasErrors() {
		t.Fatalf("corpus produced error findings: %v", st.Report.Findings())
	}
	if n := len(st.Snapshot.Changes()); n != 2 {
		t.Errorf("changes = %d, want 2", n)
	}
	if n := len(st.Snapshot.ADRs()); n != 1 {
		t.Errorf("adrs = %d, want 1", n)
	}

	var gotPaths []string
	for p := range st.Sources {
		gotPaths = append(gotPaths, p)
	}
	sort.Strings(gotPaths)
	want := []string{
		"docs/adrs/0001-first-decision.md",
		"docs/changes/active/0001-first-change.md",
		"docs/changes/active/0002-second-change.md",
	}
	sort.Strings(want)
	if strings.Join(gotPaths, "|") != strings.Join(want, "|") {
		t.Errorf("loaded sources = %v, want %v", gotPaths, want)
	}
}

// TestLoaderErrorsOnUnparseableRecord proves a record whose bytes will not parse
// surfaces as a Go error (a loader failure), not a domain finding.
func TestLoaderErrorsOnUnparseableRecord(t *testing.T) {
	overlayBase := &staticTree{
		rev: gitcli.Revision{Commit: fixedBase},
		entries: []gitcli.TreeEntry{
			{Path: "docs/changes/active/0001-x.md", Mode: "100644", Type: "blob", ObjectID: "a"},
		},
		blobs: map[gitcli.RepoPath][]byte{
			// A frontmatter block opened and never closed: document.Parse rejects it.
			"docs/changes/active/0001-x.md": []byte("---\nid: 1\n"),
		},
	}
	if _, err := (testLoader{}).Load(context.Background(), overlayBase); err == nil {
		t.Fatal("Load over an unparseable record: want error, got nil")
	}
}

// staticTree is an in-memory Tree for loader unit tests that do not need real Git.
type staticTree struct {
	rev     gitcli.Revision
	entries []gitcli.TreeEntry
	blobs   map[gitcli.RepoPath][]byte
}

func (s *staticTree) Revision() gitcli.Revision { return s.rev }

func (s *staticTree) ListTree(_ context.Context, prefixes []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	if len(prefixes) == 0 {
		return s.entries, nil
	}
	var out []gitcli.TreeEntry
	for _, e := range s.entries {
		for _, pre := range prefixes {
			ps, pfx := string(e.Path), string(pre)
			if pre == "" || e.Path == pre || (len(ps) > len(pfx) && ps[:len(pfx)] == pfx && ps[len(pfx)] == '/') {
				out = append(out, e)
				break
			}
		}
	}
	return out, nil
}

func (s *staticTree) ReadBlobs(_ context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	out := make([]gitcli.BlobResult, len(paths))
	for i, p := range paths {
		b, ok := s.blobs[p]
		out[i] = gitcli.BlobResult{Path: p, Found: ok, Blob: gitcli.Blob{Mode: "100644", Bytes: b}}
	}
	return out, nil
}
