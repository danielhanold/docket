package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// fakeBacklinkTree serves scripted blobs; a nil entry answers "absent".
type fakeBacklinkTree struct {
	blobs map[string][]byte
}

func (f fakeBacklinkTree) Revision() gitcli.Revision { return gitcli.Revision{} }

func (f fakeBacklinkTree) ListTree(_ context.Context, _ []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	panic("backlink loader must not list the tree — it reads exact paths only")
}

func (f fakeBacklinkTree) ReadBlobs(_ context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	out := make([]gitcli.BlobResult, len(paths))
	for i, p := range paths {
		out[i].Path = p
		if b, ok := f.blobs[string(p)]; ok {
			out[i].Found = true
			out[i].Blob.Bytes = append([]byte(nil), b...)
		}
	}
	return out, nil
}

const backlinkTestArtifact = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
	"> line\n" +
	"<!-- docket:backlink:end -->\n\n# Plan\n\nBody.\n"

// A well-formed frontmatter file that fails document.Parse: unquoted colon-space
// in a scalar — the exact real-world trigger (ADR-0024 on docket's main).
const backlinkTestMalformed = "---\ntitle: uses `context: fork` dispatch\n---\n\n# Doc\n"

func backlinkLoaderFor(paths ...string) transaction.StateLoader {
	return newBacklinkArtifactLoader([]closeoutBacklinkTarget{{artifactPaths: paths, interior: "x"}})
}

// TestBacklinkLoaderScopedToArtifacts: the loader validates ONLY the targeted
// artifact paths — it never lists (fake panics on ListTree), a parse-clean
// artifact yields no error, and an absent artifact is a clean skip.
func TestBacklinkLoaderScopedToArtifacts(t *testing.T) {
	tree := fakeBacklinkTree{blobs: map[string][]byte{
		"docs/superpowers/plans/p.md": []byte(backlinkTestArtifact),
	}}
	l := backlinkLoaderFor("docs/superpowers/plans/p.md", "docs/changes/results/r.md")
	st, err := l.Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Report.HasErrors() {
		t.Fatalf("clean artifacts reported errors: %+v", st.Report.Findings())
	}
	if _, ok := st.Sources["docs/superpowers/plans/p.md"]; !ok {
		t.Errorf("present artifact missing from Sources")
	}
	if _, ok := st.Sources["docs/changes/results/r.md"]; ok {
		t.Errorf("absent artifact conjured Sources bytes")
	}
	if evo := l.ValidateEvolution(st, st); len(evo) != 0 {
		t.Errorf("ValidateEvolution over artifacts returned findings: %+v", evo)
	}
}

// TestBacklinkLoaderRefusesOnTargetedParseFailure: a targeted artifact whose
// bytes fail document.Parse is an error-severity finding naming that path —
// the ONE in-scope condition the leg still gates on.
func TestBacklinkLoaderRefusesOnTargetedParseFailure(t *testing.T) {
	tree := fakeBacklinkTree{blobs: map[string][]byte{
		"docs/superpowers/plans/p.md": []byte(backlinkTestMalformed),
	}}
	st, err := backlinkLoaderFor("docs/superpowers/plans/p.md").Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !st.Report.HasErrors() {
		t.Fatalf("malformed targeted artifact did not report an error")
	}
	fds := st.Report.Findings()
	found := false
	for _, fd := range fds {
		if fd.Entity.Path == "docs/superpowers/plans/p.md" && fd.Code != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("no finding names the malformed artifact path: %+v", fds)
	}
}

// TestBacklinkLegDetailRendersCauses: the shared renderer folds a refusal's
// findings (code + path) and a failure's typed stage/kind/detail into prose;
// a non-failed, non-refused result renders empty.
func TestBacklinkLegDetailRendersCauses(t *testing.T) {
	refused := transaction.Result{Disposition: transaction.DispositionRefused}
	refused.Findings = st0Findings() // helper below
	got := backlinkLegDetail(refused, nil)
	if !strings.Contains(got, "docs/adrs/0099-x.md") {
		t.Errorf("refusal detail does not name the offending path: %q", got)
	}

	failed := transaction.Result{Disposition: transaction.DispositionFailed}
	ferr := &transaction.Failure{Stage: transaction.StagePush, Kind: transaction.KindExternal, Detail: "push rejected"}
	got = backlinkLegDetail(failed, ferr)
	if !strings.Contains(got, string(transaction.StagePush)) || !strings.Contains(got, "push rejected") {
		t.Errorf("failure detail lost stage/detail: %q", got)
	}

	if got := backlinkLegDetail(transaction.Result{Disposition: transaction.DispositionContended}, nil); got != "" {
		t.Errorf("contended rendered a spurious detail: %q", got)
	}
}

func st0Findings() []domain.Finding {
	return []domain.Finding{{
		Code: "parse-failed", Severity: domain.SeverityError,
		Entity: domain.EntityRef{Path: "docs/adrs/0099-x.md"},
	}}
}
