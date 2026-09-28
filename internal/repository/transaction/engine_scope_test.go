package transaction

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// These tests drive the engine's scoped gate sequence (change 0449) end to end
// through real isolated Git repositories: the before-gate, the post-plan
// recheck, and the after-gate refuse only errors relevant to the resolved
// subject set, grandfather exact unchanged unrelated errors, and fall back to
// strict whole-corpus validation whenever the scope cannot be resolved.

const (
	// scopeSubjectPath is the root record B every scoped operation acts on.
	scopeSubjectPath = "docs/changes/active/0001-first-change.md"
	// scopeDependencyPath is a second corpus record a resolver may name as B's
	// required record (a dependency).
	scopeDependencyPath = "docs/changes/active/0002-second-change.md"
	// scopeUnrelatedPath is the unrelated invalid record A: its filename slug
	// disagrees with its frontmatter slug, an error-severity finding on A alone.
	scopeUnrelatedPath = "docs/changes/active/0007-mismatch.md"
)

func scopeUnrelatedRecord() string { return corpusChange(7, "different-slug", "proposed") }

// editedSubject is B with a body edit — a legal, validation-clean replacement.
func editedSubject() string {
	return strings.Replace(corpusChange(1, "first-change", "proposed"), "Body.", "Body, edited.", 1)
}

// editSubjectOp replaces B with editedSubject.
func editSubjectOp() *scriptedOp {
	return &scriptedOp{files: []FileMutation{
		{Path: scopeSubjectPath, Kind: MutationReplace, Bytes: []byte(editedSubject())},
	}}
}

// scopeLoader wraps testLoader, filling Blobs from the tree listing exactly as
// the production loader does (an overlay-touched path carries ""), counting
// before/after loads, and optionally tampering with the after state so a test
// can present after-gate volatility the overlay cannot produce organically.
type scopeLoader struct {
	mu              sync.Mutex
	befores, afters int
	beforeErrors    int // error findings in the most recent before state
	tamperAfter     func(*LoadedState)
}

func (l *scopeLoader) Load(ctx context.Context, t Tree) (LoadedState, error) {
	st, err := testLoader{}.Load(ctx, t)
	if err != nil {
		return st, err
	}
	entries, err := t.ListTree(ctx, []gitcli.RepoPath{docsPrefix})
	if err != nil {
		return LoadedState{}, err
	}
	st.Blobs = make(map[string]gitcli.ObjectID, len(entries))
	for _, e := range entries {
		st.Blobs[string(e.Path)] = e.ObjectID
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, overlay := t.(*overlayTree); overlay {
		l.afters++
		if l.tamperAfter != nil {
			l.tamperAfter(&st)
		}
	} else {
		l.befores++
		l.beforeErrors = len(errorFindings(st.Report.Findings()))
	}
	return st, nil
}

func (l *scopeLoader) ValidateEvolution(before, after LoadedState) []domain.Finding {
	return testLoader{}.ValidateEvolution(before, after)
}

var _ StateLoader = (*scopeLoader)(nil)

// staticScope resolves the same fixed subject paths in every state and counts
// its invocations.
func staticScope(calls *int, paths ...string) *ValidationScope {
	return &ValidationScope{Subjects: func(LoadedState) (map[gitcli.RepoPath]bool, error) {
		if calls != nil {
			*calls++
		}
		out := make(map[gitcli.RepoPath]bool, len(paths))
		for _, p := range paths {
			out[gitcli.RepoPath(p)] = true
		}
		return out, nil
	}}
}

// dependencyScope resolves B plus the path of every depends_on target B carries
// in the state it is handed — the shape of a production resolver, so a
// dependency the plan ADDS shows up only in the candidate state's resolution.
func dependencyScope() *ValidationScope {
	return &ValidationScope{Subjects: func(st LoadedState) (map[gitcli.RepoPath]bool, error) {
		out := map[gitcli.RepoPath]bool{scopeSubjectPath: true}
		b, found := st.Snapshot.Change(1)
		if found != domain.LookupFound {
			return out, nil
		}
		for _, dep := range b.DependsOn() {
			if c, ok := st.Snapshot.Change(dep); ok == domain.LookupFound {
				out[gitcli.RepoPath(c.Path())] = true
			}
		}
		return out, nil
	}}
}

// execScoped runs one scoped transaction and fails on a Go error.
func execScoped(t *testing.T, eng *Engine, r *testRepos, repo gitcli.Repository, loader StateLoader,
	scope *ValidationScope, exp []EntityExpectation, op SemanticOperation) Result {
	t.Helper()
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Expected: exp, Loader: loader, Scope: scope, Operation: op,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

// hasFindingAt reports whether findings carry one whose Entity path is p.
func hasFindingAt(findings []domain.Finding, p string) bool {
	for _, f := range findings {
		if f.Entity.Path == p {
			return true
		}
	}
	return false
}

// assertScopedRefusal checks a refusal left origin untouched.
func assertScopedRefusal(t *testing.T, r *testRepos, res Result, base gitcli.ObjectID) {
	t.Helper()
	if res.Disposition != DispositionRefused {
		t.Fatalf("disposition = %q, want refused (findings %v)", res.Disposition, res.Findings)
	}
	if len(res.Findings) == 0 {
		t.Error("refusal carried no findings")
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on a refusal")
	}
}

// assertGrandfatheredSurfaced proves an applied or no-op scoped result carries
// the unrelated record A's grandfathered error finding — and only it, at its
// error severity (spec §1 step 5: unrelated health findings travel through the
// result's findings without changing the disposition). Mutation check: drop the
// kept findings from the applied/no-op result in runCandidate and this reddens.
func assertGrandfatheredSurfaced(t *testing.T, findings []domain.Finding) {
	t.Helper()
	if len(findings) == 0 {
		t.Fatal("result dropped the grandfathered unrelated finding; want it surfaced")
	}
	for _, f := range findings {
		if f.Entity.Path != scopeUnrelatedPath || f.Severity != domain.SeverityError {
			t.Errorf("surfaced finding %+v; want only A's error findings at %s", f, scopeUnrelatedPath)
		}
	}
}
