package transaction

import (
	"context"
	"errors"
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

// TestEngineScopeAppliesDespiteUnrelatedError is the progress case: an
// unrelated invalid record A does not veto a scoped write to B, A's bytes stay
// identical on the target ref, and the unrelated finding is not a refusal.
func TestEngineScopeAppliesDespiteUnrelatedError(t *testing.T) {
	for _, topo := range topologies() {
		t.Run(topo.name, func(t *testing.T) {
			r := topo.build(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
			aBlob := r.blobID(t, scopeUnrelatedPath)

			loader := &scopeLoader{}
			res := execScoped(t, eng, r, repo, loader, staticScope(nil, scopeSubjectPath), nil, editSubjectOp())
			if loader.beforeErrors == 0 {
				t.Fatal("fixture is vacuous: the before state carried no error finding")
			}
			if res.Disposition != DispositionApplied {
				t.Fatalf("disposition = %q, want applied (findings %v)", res.Disposition, res.Findings)
			}
			if len(res.Findings) != 0 {
				t.Errorf("applied result carried refusal findings %v", res.Findings)
			}
			if got := r.blobID(t, scopeUnrelatedPath); got != aBlob {
				t.Errorf("unrelated record blob changed: %q -> %q", aBlob, got)
			}
			paths := diffTreePaths(t, r.Origin, res.AppliedCommit)
			if len(paths) != 1 || paths[0] != scopeSubjectPath {
				t.Errorf("committed paths = %v, want [%s]", paths, scopeSubjectPath)
			}
		})
	}
}

// TestEngineScopeRefusesRelevantBeforeErrors proves an error on a subject — the
// root itself, a resolved dependency, or a path the request pins in Expected —
// refuses before the operation is ever consulted.
func TestEngineScopeRefusesRelevantBeforeErrors(t *testing.T) {
	cases := []struct {
		name    string
		seed    func(t *testing.T, r *testRepos)
		scope   []string
		pinA    bool
		wantAt  string
		wantNot string
	}{
		{
			name: "error on root",
			seed: func(t *testing.T, r *testRepos) {
				r.advanceOrigin(t, scopeSubjectPath, corpusChange(1, "other-slug", "proposed"))
			},
			scope:  []string{scopeSubjectPath},
			wantAt: scopeSubjectPath,
		},
		{
			name: "error on resolved dependency",
			seed: func(t *testing.T, r *testRepos) {
				r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
				r.advanceOrigin(t, scopeDependencyPath, corpusChange(2, "other-slug", "proposed"))
			},
			scope:   []string{scopeSubjectPath, scopeDependencyPath},
			wantAt:  scopeDependencyPath,
			wantNot: scopeUnrelatedPath,
		},
		{
			name: "error on an expectation-pinned path",
			seed: func(t *testing.T, r *testRepos) {
				r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
			},
			scope:  []string{scopeSubjectPath},
			pinA:   true,
			wantAt: scopeUnrelatedPath,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			c.seed(t, r)
			base := r.originTip(t)
			var exp []EntityExpectation
			if c.pinA {
				exp = []EntityExpectation{{Path: scopeUnrelatedPath,
					Version: ExpectedVersion{Kind: VersionBlob, ObjectID: r.blobID(t, scopeUnrelatedPath)}}}
			}

			op := editSubjectOp()
			res := execScoped(t, eng, r, repo, &scopeLoader{}, staticScope(nil, c.scope...), exp, op)
			assertScopedRefusal(t, r, res, base)
			if op.calls != 0 {
				t.Errorf("operation consulted %d times before the scoped before-gate refused", op.calls)
			}
			if !hasFindingAt(res.Findings, c.wantAt) {
				t.Errorf("refusal findings %v lack the relevant error at %s", res.Findings, c.wantAt)
			}
			if c.wantNot != "" && hasFindingAt(res.Findings, c.wantNot) {
				t.Errorf("refusal findings %v carry the unrelated error at %s", res.Findings, c.wantNot)
			}
		})
	}
}

// TestEngineScopeUnresolvableScopeIsStrict proves an empty resolved subject set,
// a resolver error, and a nil resolver inside a non-nil scope all fall back to
// strict whole-corpus validation: the unrelated error refuses, never applies.
func TestEngineScopeUnresolvableScopeIsStrict(t *testing.T) {
	cases := []struct {
		name  string
		scope *ValidationScope
	}{
		{"empty resolved set", &ValidationScope{Subjects: func(LoadedState) (map[gitcli.RepoPath]bool, error) {
			return map[gitcli.RepoPath]bool{}, nil
		}}},
		{"only unusable entries resolved", &ValidationScope{Subjects: func(LoadedState) (map[gitcli.RepoPath]bool, error) {
			return map[gitcli.RepoPath]bool{"": true, scopeSubjectPath: false}, nil
		}}},
		{"resolver error", &ValidationScope{Subjects: func(LoadedState) (map[gitcli.RepoPath]bool, error) {
			return map[gitcli.RepoPath]bool{scopeSubjectPath: true}, errors.New("resolver failed")
		}}},
		{"nil resolver", &ValidationScope{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
			base := r.originTip(t)

			op := editSubjectOp()
			res := execScoped(t, eng, r, repo, &scopeLoader{}, c.scope, nil, op)
			assertScopedRefusal(t, r, res, base)
			if op.calls != 0 {
				t.Errorf("operation consulted %d times despite a strict before-gate refusal", op.calls)
			}
			if !hasFindingAt(res.Findings, scopeUnrelatedPath) {
				t.Errorf("strict refusal findings %v lack the corpus error at %s", res.Findings, scopeUnrelatedPath)
			}
		})
	}
}

// TestEngineScopeUnresolvableCandidateIsStrict proves the after-gate falls back
// to strict on its own: when the candidate state resolves to nothing (or the
// resolver errors on it), the unrelated pre-existing error refuses instead of
// being grandfathered under the before state's subjects.
func TestEngineScopeUnresolvableCandidateIsStrict(t *testing.T) {
	cases := []struct {
		name  string
		after func() (map[gitcli.RepoPath]bool, error)
	}{
		{"candidate resolves empty", func() (map[gitcli.RepoPath]bool, error) {
			return map[gitcli.RepoPath]bool{}, nil
		}},
		{"candidate resolves only unusable entries", func() (map[gitcli.RepoPath]bool, error) {
			return map[gitcli.RepoPath]bool{"": true, scopeSubjectPath: false}, nil
		}},
		{"candidate resolver error", func() (map[gitcli.RepoPath]bool, error) {
			return nil, errors.New("resolver failed on candidate")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
			base := r.originTip(t)

			calls := 0
			scope := &ValidationScope{Subjects: func(LoadedState) (map[gitcli.RepoPath]bool, error) {
				calls++
				if calls == 1 { // the before state
					return map[gitcli.RepoPath]bool{scopeSubjectPath: true}, nil
				}
				return c.after()
			}}
			op := editSubjectOp()
			loader := &scopeLoader{}
			res := execScoped(t, eng, r, repo, loader, scope, nil, op)
			assertScopedRefusal(t, r, res, base)
			if op.calls != 1 || loader.afters != 1 {
				t.Errorf("operation consulted %d / candidate loaded %d times, want 1 / 1 (an after-gate refusal)", op.calls, loader.afters)
			}
			if !hasFindingAt(res.Findings, scopeUnrelatedPath) {
				t.Errorf("strict after-gate findings %v lack the corpus error at %s", res.Findings, scopeUnrelatedPath)
			}
		})
	}
}

// TestEngineScopePostPlanRecheckRefusesPlanTouchedError proves a plan that
// declares a path whose record already carried an error widens the scope and
// refuses at the post-plan recheck — before the candidate is ever loaded — even
// when the plan would repair that record.
func TestEngineScopePostPlanRecheckRefusesPlanTouchedError(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
	base := r.originTip(t)

	op := &scriptedOp{files: []FileMutation{
		{Path: scopeSubjectPath, Kind: MutationReplace, Bytes: []byte(editedSubject())},
		{Path: scopeUnrelatedPath, Kind: MutationReplace, Bytes: []byte(corpusChange(7, "mismatch", "proposed"))},
	}}
	loader := &scopeLoader{}
	res := execScoped(t, eng, r, repo, loader, staticScope(nil, scopeSubjectPath), nil, op)
	assertScopedRefusal(t, r, res, base)
	if op.calls != 1 {
		t.Errorf("operation consulted %d times, want 1", op.calls)
	}
	if loader.afters != 0 {
		t.Errorf("candidate loaded %d times; the post-plan recheck must refuse before the after load", loader.afters)
	}
	if !hasFindingAt(res.Findings, scopeUnrelatedPath) {
		t.Errorf("recheck findings %v lack the plan-touched error at %s", res.Findings, scopeUnrelatedPath)
	}
}

// TestEngineScopeCandidateSubjectsAreResolved proves the resolver runs against
// the candidate too: a plan that makes B newly depend on the invalid A pulls A
// into scope, so A's unchanged error refuses rather than being grandfathered.
func TestEngineScopeCandidateSubjectsAreResolved(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
	base := r.originTip(t)

	dependent := strings.Replace(corpusChange(1, "first-change", "proposed"),
		"type: feat\n", "type: feat\ndepends_on: [7]\n", 1)
	op := &scriptedOp{files: []FileMutation{
		{Path: scopeSubjectPath, Kind: MutationReplace, Bytes: []byte(dependent)},
	}}
	loader := &scopeLoader{}
	res := execScoped(t, eng, r, repo, loader, dependencyScope(), nil, op)
	assertScopedRefusal(t, r, res, base)
	if loader.afters != 1 {
		t.Errorf("candidate loaded %d times, want 1 (the after-gate refusal)", loader.afters)
	}
	if !hasFindingAt(res.Findings, scopeUnrelatedPath) {
		t.Errorf("after-gate findings %v lack the newly required record's error at %s", res.Findings, scopeUnrelatedPath)
	}
}

// TestEngineScopeAfterGateVolatility proves the after-gate grandfathers only an
// exact, unchanged pre-existing unrelated error: a changed blob id for its path,
// a new error (even one sharing the old code), or a count increase all refuse.
func TestEngineScopeAfterGateVolatility(t *testing.T) {
	// withFindings rebuilds st's report with extra findings appended.
	withFindings := func(st *LoadedState, extra ...domain.Finding) {
		st.Report = domain.NewValidationReport(append(st.Report.Findings(), extra...))
	}
	unrelatedFinding := func(st *LoadedState) domain.Finding {
		for _, f := range errorFindings(st.Report.Findings()) {
			if f.Entity.Path == scopeUnrelatedPath {
				return f
			}
		}
		t.Fatal("after state lacks the unrelated finding")
		return domain.Finding{}
	}
	cases := []struct {
		name   string
		tamper func(st *LoadedState)
		wantAt string
	}{
		{"unrelated path blob id changed", func(st *LoadedState) {
			st.Blobs[scopeUnrelatedPath] = "1111111111111111111111111111111111111111"
		}, scopeUnrelatedPath},
		{"new unrelated error with an existing code", func(st *LoadedState) {
			f := unrelatedFinding(st)
			f.Entity = domain.EntityRef{Kind: domain.EntityChange, ID: 2, Slug: "second-change", Path: scopeDependencyPath}
			withFindings(st, f)
		}, scopeDependencyPath},
		{"count increase of an existing key", func(st *LoadedState) {
			withFindings(st, unrelatedFinding(st))
		}, scopeUnrelatedPath},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())
			base := r.originTip(t)

			loader := &scopeLoader{tamperAfter: c.tamper}
			res := execScoped(t, eng, r, repo, loader, staticScope(nil, scopeSubjectPath), nil, editSubjectOp())
			assertScopedRefusal(t, r, res, base)
			if loader.afters != 1 {
				t.Errorf("candidate loaded %d times, want 1", loader.afters)
			}
			if !hasFindingAt(res.Findings, c.wantAt) {
				t.Errorf("after-gate findings %v lack the volatile error at %s", res.Findings, c.wantAt)
			}
		})
	}
}

// TestEngineScopeLeaseRetryRecomputes proves scope and baseline are derived
// fresh from each attempt's base: after a lease loss, the second base decides —
// a newly relevant error refuses, and a newly present unrelated error is
// grandfathered against the fresh baseline rather than a stale one.
func TestEngineScopeLeaseRetryRecomputes(t *testing.T) {
	const newUnrelatedPath = "docs/changes/active/0008-other-mismatch.md"
	cases := []struct {
		name        string
		advancePath string
		advanceBody string
		want        Disposition
	}{
		{"second base carries a relevant error", scopeDependencyPath, corpusChange(2, "other-slug", "proposed"), DispositionRefused},
		{"second base carries a new unrelated error", newUnrelatedPath, corpusChange(8, "yet-another-slug", "proposed"), DispositionApplied},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())

			op := editSubjectOp()
			op.beforePlan = func(call int) {
				if call == 1 {
					r.advanceOrigin(t, c.advancePath, c.advanceBody)
				}
			}
			var resolverCalls int
			loader := &scopeLoader{}
			res := execScoped(t, eng, r, repo, loader,
				staticScope(&resolverCalls, scopeSubjectPath, scopeDependencyPath), nil, op)
			if res.Disposition != c.want {
				t.Fatalf("disposition = %q, want %q (findings %v)", res.Disposition, c.want, res.Findings)
			}
			if res.Attempts != 2 {
				t.Errorf("attempts = %d, want 2", res.Attempts)
			}
			if loader.befores != 2 {
				t.Errorf("before state loaded %d times, want once per attempt (2)", loader.befores)
			}
			if resolverCalls < 3 {
				t.Errorf("resolver consulted %d times; want it re-run on the second attempt's base", resolverCalls)
			}
			if c.want == DispositionRefused {
				if op.calls != 1 {
					t.Errorf("operation consulted %d times, want 1 (second attempt refuses before planning)", op.calls)
				}
				if !hasFindingAt(res.Findings, scopeDependencyPath) {
					t.Errorf("refusal findings %v lack the second base's relevant error", res.Findings)
				}
			}
		})
	}
}

// TestEngineScopeKeyedReplay proves a scoped keyed request replays
// already-applied through the idempotency scan, before any load or resolution.
func TestEngineScopeKeyedReplay(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	r.advanceOrigin(t, scopeUnrelatedPath, scopeUnrelatedRecord())

	var resolverCalls int
	scope := staticScope(&resolverCalls, scopeSubjectPath)
	loader := &scopeLoader{}
	first, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: loader, Scope: scope, Operation: editSubjectOp(),
	})
	if err != nil || first.Disposition != DispositionApplied {
		t.Fatalf("first Execute: disposition %q err %v (findings %v)", first.Disposition, err, first.Findings)
	}
	loadsAfterFirst, callsAfterFirst := loader.befores+loader.afters, resolverCalls

	replayOp := editSubjectOp()
	second, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: loader, Scope: scope, Operation: replayOp,
	})
	if err != nil {
		t.Fatalf("replay Execute: %v", err)
	}
	if second.Disposition != DispositionAlreadyApplied {
		t.Fatalf("replay disposition = %q, want already-applied", second.Disposition)
	}
	if second.AppliedCommit != first.AppliedCommit {
		t.Errorf("replay commit = %q, want original %q", second.AppliedCommit, first.AppliedCommit)
	}
	if got := loader.befores + loader.afters; got != loadsAfterFirst {
		t.Errorf("replay loaded state %d more times, want 0", got-loadsAfterFirst)
	}
	if resolverCalls != callsAfterFirst || replayOp.calls != 0 {
		t.Errorf("replay consulted resolver %d / operation %d times, want 0", resolverCalls-callsAfterFirst, replayOp.calls)
	}
}
