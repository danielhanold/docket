package transaction

import (
	"context"
	"fmt"
	"github.com/danielhanold/docket/internal/testsupport"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the required concurrency matrix from the spec's acceptance
// boundary. Every scenario runs TWO engines over TWO INDEPENDENT clones of one
// bare origin — never one shared common directory, because two writers that share
// a Git common dir do not model separate machines. Writers are coordinated by
// channels and barriers, never sleeps: the loser's SemanticOperation blocks in
// Plan (after it has already fetched its base) until the winner's Execute returns,
// which makes the lease loss deterministic. Each scenario runs against both
// metadata topologies (main mode and docket mode) and, via the shared
// captureCheckouts/assertCheckoutsUnchanged helpers, also proves both user
// checkouts stay byte-identical — only origin refs/objects and the private
// transactions state may move.

// freshClone makes an independent non-bare clone of r's origin, pins a
// deterministic identity and core.quotePath=true, discovers it as the engine's
// caller would, and returns the repository plus its working-tree directory. Two
// fresh clones share nothing but the bare origin, so a push from one is a genuine
// remote advance to the other.
func freshClone(t *testing.T, client *gitcli.Client, r *testRepos, name string) (gitcli.Repository, string) {
	t.Helper()
	parent := testsupport.TempDir(t)
	dst := filepath.Join(parent, name)
	hgitOut(t, parent, "clone", "-q", r.Origin, dst)
	hconfigIdentity(t, dst)
	hgitOut(t, dst, "config", "core.quotePath", "true")
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: dst})
	if err != nil {
		t.Fatalf("Discover %s: %v", name, err)
	}
	return repo, dst
}

// concHarness bundles the two independent engine clones plus the origin oracle for
// one concurrency scenario. One Engine and one Client are shared by both writers —
// the engine holds no per-attempt state — while repoA/repoB have distinct common
// dirs, so their private transaction roots never collide.
type concHarness struct {
	r      *testRepos
	client *gitcli.Client
	eng    *Engine
	repoA  gitcli.Repository
	dirA   string
	repoB  gitcli.Repository
	dirB   string
}

// newConcHarness builds a topology, two independent engine clones, and one shared
// Engine over a fresh client.
func newConcHarness(t *testing.T, build func(*testing.T) *testRepos) *concHarness {
	t.Helper()
	requireGit(t)
	r := build(t)
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	eng := newEngine(t, client)
	repoA, dirA := freshClone(t, client, r, "writerA")
	repoB, dirB := freshClone(t, client, r, "writerB")
	return &concHarness{r: r, client: client, eng: eng, repoA: repoA, dirA: dirA, repoB: repoB, dirB: dirB}
}

// recordOp is the versatile SemanticOperation the concurrency matrix drives. It
// creates or replaces one change record whose body encodes the number of changes
// it saw in THIS attempt's base — a base-dependent payload, so a replan on a fresh
// base produces different bytes and a reused first-attempt plan is observable.
// When indexPath is set it also (re)generates a derived index file listing every
// change id in the after-state, sorted: the "derived overlap" case, whose loser
// must render the view from fresh state rather than text-merging. barrier, when
// set, runs at the START of each Plan call — after the base snapshot is read — so
// a test can block the loser until the winner has applied.
type recordOp struct {
	id        int
	slug      string
	path      string
	kind      MutationKind // create or replace of the record
	indexPath string       // "" to skip the derived file
	barrier   func(call int)
	calls     int
	planBytes [][]byte // the record bytes produced on each attempt, in order
	idxBytes  [][]byte // the derived index bytes produced on each attempt, in order
}

func (o *recordOp) Key() OperationKey { return "test.op" }

func (o *recordOp) Plan(ctx context.Context, st AttemptState) (MutationPlan, OperationResult, error) {
	o.calls++
	call := o.calls

	changes := st.State.Snapshot.Changes()
	body := fmt.Sprintf("changes seen: %d", len(changes))
	record := []byte(corpusChangeBody(o.id, o.slug, "proposed", body))
	o.planBytes = append(o.planBytes, record)

	files := []FileMutation{{Path: gitcli.RepoPath(o.path), Kind: o.kind, Bytes: record}}

	if o.indexPath != "" {
		ids := make([]int, 0, len(changes)+1)
		for _, ch := range changes {
			ids = append(ids, int(ch.ID()))
		}
		ids = append(ids, o.id)
		sort.Ints(ids)
		idx := []byte(renderIndex(ids))
		o.idxBytes = append(o.idxBytes, idx)

		// The derived file may or may not exist in this attempt's base: create it the
		// first time, replace it once a prior writer has published it. Choosing the
		// kind from the observed base is what makes a fresh replan valid.
		kind := MutationCreate
		blobs, err := st.Tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(o.indexPath)})
		if err != nil {
			return MutationPlan{}, OperationResult{}, err
		}
		if len(blobs) == 1 && blobs[0].Found {
			kind = MutationReplace
		}
		files = append(files, FileMutation{Path: gitcli.RepoPath(o.indexPath), Kind: kind, Bytes: idx})
	}

	if o.barrier != nil {
		o.barrier(call)
	}
	return MutationPlan{Files: files, CommitSubject: "test: apply record", Receipt: validReceipt()}, OperationResult{}, nil
}

// corpusChangeBody renders a well-formed change record with a caller-supplied body
// paragraph, so a base-dependent payload can be embedded while the frontmatter
// (whose id/slug must match the filename) stays valid.
func corpusChangeBody(id int, slug, status, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoa(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A change'\n")
	b.WriteString("status: " + status + "\n")
	b.WriteString("priority: medium\n")
	b.WriteString("type: feat\n")
	b.WriteString("created: 2026-08-01\n")
	b.WriteString("updated: 2026-08-02\n")
	b.WriteString("---\n\n## Why\n\n" + body + "\n")
	return b.String()
}

// renderIndex renders a sorted-id derived index, one id per line — deterministic
// bytes a test can predict exactly, so a text-merge artifact would be visible.
func renderIndex(ids []int) string {
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(itoa(id) + "\n")
	}
	return b.String()
}

// originShow returns the exact bytes of path at the target ref's tip on origin.
func originShow(t *testing.T, r *testRepos, path string) string {
	t.Helper()
	return hgitOutRaw(t, r.Origin, "show", string(r.Target)+":"+path)
}

// contendingWriters runs the loser (repoB, op2) and the winner (repoA, op1) so the
// loser deterministically loses its first lease: the loser blocks inside Plan
// after fetching its base, the winner then runs to completion and advances origin,
// and only then is the loser released to push, lose, refetch, and replan. It
// returns both results.
func (h *concHarness) contendingWriters(t *testing.T, exp1, exp2 []EntityExpectation, op1, op2 *recordOp) (Result, Result, error, error) {
	t.Helper()
	w2ready := make(chan struct{})
	w1done := make(chan struct{})
	op2.barrier = func(call int) {
		if call == 1 {
			close(w2ready) // loser has fetched its base and is about to finish planning
			<-w1done       // wait until the winner has applied and advanced origin
		}
	}

	var res2 Result
	var err2 error
	done := make(chan struct{})
	go func() {
		res2, err2 = h.eng.Execute(context.Background(), Request{
			Repository: h.repoB, Remote: "origin", TargetRef: h.r.Target,
			Expected: exp2, Loader: testLoader{}, Operation: op2,
		})
		close(done)
	}()

	<-w2ready
	res1, err1 := h.eng.Execute(context.Background(), Request{
		Repository: h.repoA, Remote: "origin", TargetRef: h.r.Target,
		Expected: exp1, Loader: testLoader{}, Operation: op1,
	})
	close(w1done)
	<-done
	return res1, res2, err1, err2
}

// concTopologies names the two harness builders the matrix runs each scenario
// against.
func concTopologies() []topology { return topologies() }

// TestConcurrencyUnrelatedWritersConverge proves two writers touching different
// records converge: the loser loses its lease, refetches the winner's base,
// replans from fresh state, and applies — its FIRST plan bytes never appear in the
// winning commit, and origin ends with both records.
func TestConcurrencyUnrelatedWritersConverge(t *testing.T) {
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			h := newConcHarness(t, topo.build)
			before := captureCheckouts(t, h.dirA, h.dirB)

			op1 := &recordOp{id: 5, slug: "writer-x", path: "docs/changes/active/0005-writer-x.md", kind: MutationCreate}
			op2 := &recordOp{id: 6, slug: "writer-y", path: "docs/changes/active/0006-writer-y.md", kind: MutationCreate}
			res1, res2, err1, err2 := h.contendingWriters(t, nil, nil, op1, op2)
			if err1 != nil || res1.Disposition != DispositionApplied {
				t.Fatalf("winner: disposition %q err %v", res1.Disposition, err1)
			}
			if err2 != nil {
				t.Fatalf("loser Execute: %v", err2)
			}
			if res2.Disposition != DispositionApplied {
				t.Fatalf("loser disposition = %q, want applied", res2.Disposition)
			}
			if res2.Attempts != 2 {
				t.Errorf("loser attempts = %d, want 2 (one lease loss then apply)", res2.Attempts)
			}
			if op2.calls != 2 {
				t.Errorf("loser replanned %d times, want 2", op2.calls)
			}

			// Both records converged onto origin.
			names := hgitOut(t, h.r.Origin, "ls-tree", "-r", "--name-only", string(h.r.Target))
			for _, want := range []string{op1.path, op2.path} {
				if !strings.Contains(names, want) {
					t.Errorf("origin missing converged record %q:\n%s", want, names)
				}
			}

			// The loser's FIRST plan bytes (base A, "changes seen: 2") must not appear in
			// the winning commit; the committed blob is the SECOND plan (base B,
			// "changes seen: 3"). This is the proof the retry replanned from fresh state
			// rather than reusing a stale patch.
			committed := originShow(t, h.r, op2.path)
			if committed == string(op2.planBytes[0]) {
				t.Errorf("committed record equals the FIRST (stale) plan — retry reused a patch")
			}
			if committed != string(op2.planBytes[1]) {
				t.Errorf("committed record != the fresh replan bytes:\ncommitted %q\nreplan    %q", committed, op2.planBytes[1])
			}
			if !strings.Contains(committed, "changes seen: 3") {
				t.Errorf("committed record body = %q, want the fresh base's count (3)", committed)
			}

			assertCheckoutsUnchanged(t, []string{h.dirA, h.dirB}, before)
		})
	}
}

// TestConcurrencySameEntityContends proves two writers expecting the same blob do
// NOT both win: the loser's retry sees the winner's new blob rather than the one
// it expected and returns contended with no commit of its own.
func TestConcurrencySameEntityContends(t *testing.T) {
	const rec = "docs/changes/active/0001-first-change.md"
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			h := newConcHarness(t, topo.build)
			before := captureCheckouts(t, h.dirA, h.dirB)
			x1 := h.r.blobID(t, rec)

			// Both writers REPLACE record 1; both expect it at blob X1. The winner sets
			// X2; the loser's retry sees X2 != X1 and contends.
			op1 := &recordOp{id: 1, slug: "first-change", path: rec, kind: MutationReplace}
			op2 := &recordOp{id: 1, slug: "first-change", path: rec, kind: MutationReplace}
			exp := []EntityExpectation{{Path: rec, Version: ExpectedVersion{Kind: VersionBlob, ObjectID: x1}}}

			res1, res2, err1, err2 := h.contendingWriters(t, exp, exp, op1, op2)
			if err1 != nil || res1.Disposition != DispositionApplied {
				t.Fatalf("winner: disposition %q err %v (findings %v)", res1.Disposition, err1, res1.Findings)
			}
			if err2 != nil {
				t.Fatalf("loser Execute: %v", err2)
			}
			if res2.Disposition != DispositionContended {
				t.Fatalf("loser disposition = %q, want contended", res2.Disposition)
			}
			if len(res2.ContendedPaths) != 1 || res2.ContendedPaths[0] != gitcli.RepoPath(rec) {
				t.Errorf("loser contended paths = %v, want [%s]", res2.ContendedPaths, rec)
			}
			if res2.Attempts != 2 {
				t.Errorf("loser attempts = %d, want 2 (one lease loss then a contended re-check)", res2.Attempts)
			}

			// The loser committed nothing: origin's record 1 is the winner's body.
			committed := originShow(t, h.r, rec)
			if committed != string(op1.planBytes[len(op1.planBytes)-1]) {
				t.Errorf("origin record 1 is not the winner's bytes:\n%s", committed)
			}
			// The winner made exactly one commit past the shared base; the loser added none.
			assertCheckoutsUnchanged(t, []string{h.dirA, h.dirB}, before)
		})
	}
}

// TestConcurrencyDerivedOverlapReplansView proves two writers that both regenerate
// one derived index file converge WITHOUT a text merge: the loser replans the
// derived bytes from fresh state, so the final index reflects both primary
// changes, byte-for-byte the deterministic rendering.
func TestConcurrencyDerivedOverlapReplansView(t *testing.T) {
	const indexPath = "records-index.txt"
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			h := newConcHarness(t, topo.build)
			before := captureCheckouts(t, h.dirA, h.dirB)

			op1 := &recordOp{id: 5, slug: "writer-x", path: "docs/changes/active/0005-writer-x.md", kind: MutationCreate, indexPath: indexPath}
			op2 := &recordOp{id: 6, slug: "writer-y", path: "docs/changes/active/0006-writer-y.md", kind: MutationCreate, indexPath: indexPath}
			res1, res2, err1, err2 := h.contendingWriters(t, nil, nil, op1, op2)
			if err1 != nil || res1.Disposition != DispositionApplied {
				t.Fatalf("winner: disposition %q err %v", res1.Disposition, err1)
			}
			if err2 != nil || res2.Disposition != DispositionApplied {
				t.Fatalf("loser: disposition %q err %v", res2.Disposition, err2)
			}
			if res2.Attempts != 2 {
				t.Errorf("loser attempts = %d, want 2", res2.Attempts)
			}

			// The final derived view reflects BOTH primary changes, rendered fresh: the
			// two base records (1,2) plus the winner's 5 and the loser's 6, sorted. A
			// text merge would leave conflict markers or drop one side; the exact
			// deterministic rendering proves neither happened.
			want := renderIndex([]int{1, 2, 5, 6})
			got := originShow(t, h.r, indexPath)
			if got != want {
				t.Errorf("derived index = %q, want %q (both primary changes, no merge artifact)", got, want)
			}
			// The loser's FIRST derived bytes (base A: 1,2,6) never reached origin.
			first := string(op2.idxBytes[0])
			if got == first {
				t.Errorf("derived index equals the loser's stale first render %q", first)
			}

			assertCheckoutsUnchanged(t, []string{h.dirA, h.dirB}, before)
		})
	}
}

// TestConcurrencyFourLeaseLossesContend proves the attempt cap: a writer whose
// origin is advanced before every one of its pushes makes exactly four attempts,
// returns contended, and leaves its transactions root empty — every candidate was
// cleaned.
func TestConcurrencyFourLeaseLossesContend(t *testing.T) {
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			requireGit(t)
			r := topo.build(t)
			client, err := gitcli.NewClient()
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			eng := newEngine(t, client)
			repo, dir := freshClone(t, client, r, "loser")
			before := captureCheckouts(t, dir)

			op := &recordOp{id: 7, slug: "loser", path: "docs/changes/active/0007-loser.md", kind: MutationCreate}
			// On every attempt, advance origin (via the independent writer clone) AFTER
			// the engine has fetched its base but BEFORE it pushes, so the lease always
			// loses. Execute runs in THIS goroutine, so advanceOrigin's t.Fatalf is safe.
			op.barrier = func(call int) {
				adv := call + 10
				r.advanceOrigin(t, fmt.Sprintf("docs/changes/active/00%02d-adv%d.md", adv, call),
					corpusChange(adv, fmt.Sprintf("adv%d", call), "proposed"))
			}

			res, err := eng.Execute(context.Background(), Request{
				Repository: repo, Remote: "origin", TargetRef: r.Target,
				Loader: testLoader{}, Operation: op,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Disposition != DispositionContended {
				t.Fatalf("disposition = %q, want contended", res.Disposition)
			}
			if res.Attempts != maxAttempts {
				t.Errorf("attempts = %d, want %d", res.Attempts, maxAttempts)
			}
			if op.calls != maxAttempts {
				t.Errorf("operation planned %d times, want %d", op.calls, maxAttempts)
			}
			if !transactionsEmpty(t, repo) {
				t.Error("transactions root not empty after four cleaned lease losses")
			}
			// The loser's own record never reached origin.
			names := hgitOut(t, r.Origin, "ls-tree", "-r", "--name-only", string(r.Target))
			if strings.Contains(names, op.path) {
				t.Errorf("loser's record reached origin despite contention:\n%s", names)
			}
			assertCheckoutsUnchanged(t, []string{dir}, before)
		})
	}
}
