package app

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// planningTestConfig builds a resolved configuration carrying the leaves the
// planning loader and board fence consult: the corpus directories, the change
// type set the fixtures use, learnings enabled, and the board surfaces.
func planningTestConfig(surfaces []string) config.Effective {
	var eff config.Effective
	eff.ChangesDir.Value = "docs/changes"
	eff.ADRsDir.Value = "docs/adrs"
	eff.ChangeTypes.Value = []string{"feat", "fix", "chore", "docs"}
	eff.Learnings.Enabled.Value = true
	eff.BoardSurfaces.Value = surfaces
	return eff
}

// fakeTree is an in-memory transaction.Tree for the planning loader unit tests,
// modeled on the staticTree in internal/repository/transaction/loader_test.go.
type fakeTree struct {
	rev     gitcli.Revision
	entries []gitcli.TreeEntry
	blobs   map[gitcli.RepoPath][]byte
}

func newFakeTree(files map[string]string) *fakeTree {
	t := &fakeTree{
		rev:   gitcli.Revision{Commit: "0000000000000000000000000000000000000000"},
		blobs: make(map[gitcli.RepoPath][]byte, len(files)),
	}
	for p, body := range files {
		rp := gitcli.RepoPath(p)
		t.entries = append(t.entries, gitcli.TreeEntry{Path: rp, Mode: "100644", Type: "blob", ObjectID: "a"})
		t.blobs[rp] = []byte(body)
	}
	return t
}

func (t *fakeTree) Revision() gitcli.Revision { return t.rev }

func (t *fakeTree) ListTree(_ context.Context, prefixes []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	if len(prefixes) == 0 {
		return t.entries, nil
	}
	var out []gitcli.TreeEntry
	for _, e := range t.entries {
		ps := string(e.Path)
		for _, pre := range prefixes {
			pfx := string(pre)
			if pre == "" || e.Path == pre || (len(ps) > len(pfx) && ps[:len(pfx)] == pfx && ps[len(pfx)] == '/') {
				out = append(out, e)
				break
			}
		}
	}
	return out, nil
}

func (t *fakeTree) ReadBlobs(_ context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	out := make([]gitcli.BlobResult, len(paths))
	for i, p := range paths {
		b, ok := t.blobs[p]
		out[i] = gitcli.BlobResult{Path: p, Found: ok, Blob: gitcli.Blob{Mode: "100644", ObjectID: "a", Bytes: b}}
	}
	return out, nil
}

// fixtureChange renders a minimal well-formed proposed change record.
func fixtureChange(id int, slug string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoaTest(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A change'\n")
	b.WriteString("status: proposed\n")
	b.WriteString("priority: medium\n")
	b.WriteString("type: feat\n")
	b.WriteString("created: 2026-08-01\n")
	b.WriteString("updated: 2026-08-02\n")
	b.WriteString("---\n\n## Why\n\nBody.\n")
	return b.String()
}

// fixtureADR renders a minimal well-formed Accepted ADR record.
func fixtureADR(id int, slug string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoaTest(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A decision'\n")
	b.WriteString("status: Accepted\n")
	b.WriteString("date: 2026-08-01\n")
	b.WriteString("---\n\n## Decision\n\nBody.\n")
	return b.String()
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func TestNewPlanningLoaderBuildsStateFromCorpus(t *testing.T) {
	tree := newFakeTree(map[string]string{
		"docs/changes/active/0001-first-change.md":  fixtureChange(1, "first-change"),
		"docs/changes/active/0002-second-change.md": fixtureChange(2, "second-change"),
		"docs/adrs/0001-first-decision.md":          fixtureADR(1, "first-decision"),
		// Not a corpus record: excluded by the prefix scoping.
		".docket.yml":           "version: 1\n",
		"docs/adrs/README.md":   "# index\n",
		"docs/changes/BOARD.md": "# board\n",
	})

	loader := newPlanningLoader(planningTestConfig([]string{"inline"}))
	st, err := loader.Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Report.HasErrors() {
		t.Fatalf("clean corpus produced error findings: %v", st.Report.Findings())
	}
	if n := len(st.Snapshot.Changes()); n != 2 {
		t.Errorf("changes = %d, want 2", n)
	}
	if n := len(st.Snapshot.ADRs()); n != 1 {
		t.Errorf("adrs = %d, want 1", n)
	}
	wantPaths := []string{
		"docs/adrs/0001-first-decision.md",
		"docs/changes/active/0001-first-change.md",
		"docs/changes/active/0002-second-change.md",
	}
	for _, p := range wantPaths {
		if _, ok := st.Documents[p]; !ok {
			t.Errorf("Documents missing %q", p)
		}
		if _, ok := st.Sources[p]; !ok {
			t.Errorf("Sources missing %q", p)
		}
	}
	if len(st.Documents) != len(wantPaths) {
		t.Errorf("Documents has %d entries, want %d", len(st.Documents), len(wantPaths))
	}
	// README/BOARD derived views must not be classified as records.
	if _, ok := st.Documents["docs/adrs/README.md"]; ok {
		t.Errorf("derived index README.md classified as a record")
	}
}

func TestNewPlanningLoaderParseFailureIsFinding(t *testing.T) {
	tree := newFakeTree(map[string]string{
		"docs/changes/active/0001-good.md": fixtureChange(1, "good"),
		// A frontmatter block opened and never closed: document.Parse rejects it.
		"docs/changes/active/0002-bad.md": "---\nid: 2\n",
	})

	loader := newPlanningLoader(planningTestConfig([]string{"inline"}))
	st, err := loader.Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("Load must not return a Go error for a parse failure: %v", err)
	}
	if !st.Report.HasErrors() {
		t.Fatal("parse failure did not surface as an error finding")
	}
	var sawParse bool
	for _, f := range st.Report.Findings() {
		if f.Entity.Path == "docs/changes/active/0002-bad.md" {
			sawParse = true
			if f.Code != string(document_KindUnclosedFrontmatter) {
				t.Errorf("finding code = %q, want %q", f.Code, document_KindUnclosedFrontmatter)
			}
		}
	}
	if !sawParse {
		t.Error("no finding for the unparseable record path")
	}
	// The good record still loads.
	if _, ok := st.Documents["docs/changes/active/0001-good.md"]; !ok {
		t.Error("good record dropped when a sibling failed to parse")
	}
	if _, ok := st.Documents["docs/changes/active/0002-bad.md"]; ok {
		t.Error("unparseable record leaked into Documents")
	}
	if n := len(st.Snapshot.Changes()); n != 1 {
		t.Errorf("changes = %d, want 1 (bad record excluded)", n)
	}
}

// document_KindUnclosedFrontmatter mirrors document.KindUnclosedFrontmatter
// without importing the package for one constant in the test.
const document_KindUnclosedFrontmatter = "unclosed-frontmatter"

var digestShape = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type digestPayload struct {
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Deps     []int  `json:"deps"`
}

func TestCanonicalDigestDeterministicAndShaped(t *testing.T) {
	p := digestPayload{Title: "x", Priority: "high", Deps: []int{1, 2}}
	d1, err := canonicalDigest("change.create", p)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := canonicalDigest("change.create", p)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Errorf("digest not deterministic: %q vs %q", d1, d2)
	}
	if !digestShape.MatchString(string(d1)) {
		t.Errorf("digest shape = %q, want sha256:<64 hex>", d1)
	}
}

func TestCanonicalDigestDiffersOnAnyChange(t *testing.T) {
	base := digestPayload{Title: "x", Priority: "high", Deps: []int{1, 2}}
	baseD, _ := canonicalDigest("change.create", base)

	cases := []struct {
		name string
		op   string
		p    digestPayload
	}{
		{"title", "change.create", digestPayload{Title: "y", Priority: "high", Deps: []int{1, 2}}},
		{"priority", "change.create", digestPayload{Title: "x", Priority: "low", Deps: []int{1, 2}}},
		{"deps", "change.create", digestPayload{Title: "x", Priority: "high", Deps: []int{1, 3}}},
		{"operation", "learning.record", base},
	}
	for _, c := range cases {
		d, _ := canonicalDigest(c.op, c.p)
		if d == baseD {
			t.Errorf("%s: digest did not change (%q)", c.name, d)
		}
	}
}

func TestFenceBoardSurface(t *testing.T) {
	inline, err := fenceBoardSurface(planningTestConfig([]string{"inline"}))
	if err != nil || !inline {
		t.Errorf("[inline] => (%v, %v), want (true, nil)", inline, err)
	}

	inline, err = fenceBoardSurface(planningTestConfig([]string{}))
	if err != nil || inline {
		t.Errorf("[] => (%v, %v), want (false, nil)", inline, err)
	}

	_, err = fenceBoardSurface(planningTestConfig([]string{"inline", "github"}))
	if err == nil {
		t.Fatal("[inline github] must be fenced, got nil error")
	}
	var pe *planningError
	if !errors.As(err, &pe) {
		t.Fatalf("fence error is not a *planningError: %v", err)
	}
	if pe.Result != ResultUnsupportedConfig {
		t.Errorf("fence result = %q, want %q", pe.Result, ResultUnsupportedConfig)
	}

	// github alone is also fenced, before any inline decision.
	if _, err := fenceBoardSurface(planningTestConfig([]string{"github"})); err == nil {
		t.Error("[github] must be fenced")
	}
}

func TestMapOutcome(t *testing.T) {
	fail := func(k transaction.Kind) error {
		return &transaction.Failure{Stage: transaction.StageCommit, Kind: k}
	}

	cases := []struct {
		name        string
		res         transaction.Result
		err         error
		refusalKind Result
		want        Result
		replayed    bool
	}{
		{"applied", transaction.Result{Disposition: transaction.DispositionApplied}, nil, ResultInvalidState, ResultApplied, false},
		{"already-applied", transaction.Result{Disposition: transaction.DispositionAlreadyApplied}, nil, ResultInvalidState, ResultApplied, true},
		{"no-op", transaction.Result{Disposition: transaction.DispositionNoOp}, nil, ResultInvalidState, ResultNoOp, false},
		{"contended", transaction.Result{Disposition: transaction.DispositionContended}, nil, ResultInvalidState, ResultContended, false},
		{"refused-state", transaction.Result{Disposition: transaction.DispositionRefused}, nil, ResultInvalidState, ResultInvalidState, false},
		{"refused-input", transaction.Result{Disposition: transaction.DispositionRefused}, nil, ResultInvalidInput, ResultInvalidInput, false},
		{"interrupted", transaction.Result{Disposition: transaction.DispositionInterrupted}, fail(transaction.KindCancelled), ResultInvalidState, ResultInterrupted, false},
		{"failed-invalid-input", transaction.Result{Disposition: transaction.DispositionFailed}, fail(transaction.KindInvalidInput), ResultInvalidState, ResultInvalidInput, false},
		{"failed-invalid-state", transaction.Result{Disposition: transaction.DispositionFailed}, fail(transaction.KindInvalidState), ResultInvalidState, ResultInvalidState, false},
		{"failed-validation", transaction.Result{Disposition: transaction.DispositionFailed}, fail(transaction.KindValidation), ResultInvalidState, ResultInvalidState, false},
		{"failed-external", transaction.Result{Disposition: transaction.DispositionFailed}, fail(transaction.KindExternal), ResultInvalidState, ResultExternalFailed, false},
		{"failed-unknown-result", transaction.Result{Disposition: transaction.DispositionFailed}, fail(transaction.KindUnknownResult), ResultInvalidState, ResultExternalFailed, false},
		{"failed-cancelled", transaction.Result{Disposition: transaction.DispositionFailed}, fail(transaction.KindCancelled), ResultInvalidState, ResultInterrupted, false},
		{"failed-no-failure", transaction.Result{Disposition: transaction.DispositionFailed}, errors.New("bare"), ResultInvalidState, ResultInternalError, false},
		{"unknown-disposition", transaction.Result{Disposition: transaction.Disposition("bogus")}, nil, ResultInvalidState, ResultInternalError, false},
	}
	for _, c := range cases {
		got, replayed := mapOutcome(c.res, c.err, c.refusalKind)
		if got != c.want || replayed != c.replayed {
			t.Errorf("%s: mapOutcome = (%q, %v), want (%q, %v)", c.name, got, replayed, c.want, c.replayed)
		}
	}
}

func TestFailureStatus(t *testing.T) {
	failed := transaction.Result{Disposition: transaction.DispositionFailed}

	cases := []struct {
		name string
		res  transaction.Result
		err  error
		want *FailureStatus
	}{
		{"non-failed-disposition-is-nil",
			transaction.Result{Disposition: transaction.DispositionRefused},
			&transaction.Failure{Kind: transaction.KindInvalidState}, nil},
		{"typed-failure",
			failed,
			&transaction.Failure{Stage: transaction.StageVerifyDelta, Kind: transaction.KindInvalidState, Detail: "an undeclared path changed in the worktree"},
			&FailureStatus{Stage: "verify-delta", Kind: "invalid-state", Detail: "an undeclared path changed in the worktree"}},
		{"typed-failure-folds-wrapped-err",
			failed,
			&transaction.Failure{Stage: transaction.StageLoadAfter, Kind: transaction.KindInvalidState, Detail: "plan violates before/after tree rules", Err: errors.New("boom")},
			&FailureStatus{Stage: "load-after", Kind: "invalid-state", Detail: "plan violates before/after tree rules: boom"}},
		{"non-failure-error-is-internal-error",
			failed, errors.New("bare"),
			&FailureStatus{Kind: "internal-error", Detail: "bare"}},
		{"nil-error-is-contract-violation",
			failed, nil,
			&FailureStatus{Kind: "internal-error", Detail: "failed disposition carried no error (engine contract violation)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := failureStatus(tc.res, tc.err)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("failureStatus = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("failureStatus = nil, want a diagnosis")
			}
			if *got != *tc.want {
				t.Errorf("failureStatus = %+v, want %+v", *got, *tc.want)
			}
		})
	}
}
