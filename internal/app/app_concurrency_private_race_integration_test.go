//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
)

// seedBuildReadyPrivate creates one change through node and grooms it to
// build-ready with a spec, both through the real engine against the private
// store. It returns the change id and its record revision on the store.
func seedBuildReadyPrivate(t *testing.T, node realNode) (id int, rev string) {
	t.Helper()
	ctx := context.Background()
	store := resolvedPrivateLayout(t, node.dir).DefaultBareRemote
	created := ChangeCreate(ctx, node.deps, node.dir, validChangeCreateRequest())
	if created.Result != ResultApplied {
		t.Fatalf("change create = %q (%s, findings %v)", created.Result, created.HumanText(), created.Findings)
	}
	groom := ChangeGroom(ctx, node.deps, node.dir, ChangeGroomRequest{
		ChangeID:     created.ID,
		Path:         created.Path,
		Revision:     blobRevisionAt(t, store, privateFixtureBranch, created.Path),
		Outcome:      GroomSpec,
		SpecMarkdown: "# Widget: design\n\nBuild the widget.\n",
	})
	if groom.Result != ResultApplied {
		t.Fatalf("change groom = %q (%s, findings %v)", groom.Result, groom.HumanText(), groom.Findings)
	}
	return created.ID, blobRevisionAt(t, store, privateFixtureBranch, created.Path)
}

// clonePrivate makes a second clone of r's origin and runs init --private in
// it, so it adopts the store clone A already published.
func clonePrivate(t *testing.T, r *initRepo) string {
	t.Helper()
	dir := r.freshClone(t)
	second := &initRepo{root: r.root, origin: r.origin, writer: r.writer, invocation: dir}
	if res := second.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("second clone init --private = %q (%s), want applied", res.Result, res.HumanText())
	}
	return dir
}

// race shard (change 0531, acceptance 2): two clones of one private repository
// claim the same build-ready change at the same revision at the same moment.
// The private store is the shared authority and the lease push to it is the
// compare-and-swap: exactly one claim applies and the other contends, writing
// nothing. Each contender carries its own run context: identical ungated claims
// share claimRequestID and digest, so they would be an idempotent replay rather
// than a contention. The run context is part of the claim's request id, so the
// loser's retry after its lost lease misses the winner's receipt and the
// exact-revision expectation reports it contended. -race guards the shared
// adapter/transaction paths.
func TestRaceIntegrationAppConcurrencyPrivateClaimsOneAppliesOneContends(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("clone A init --private = %q (%s), want applied", res.Result, res.HumanText())
	}
	nodeA := planningDepsFor(t, r.invocation)
	id, rev := seedBuildReadyPrivate(t, nodeA)

	b := clonePrivate(t, r)
	nodeB := planningDepsFor(t, b)
	store := resolvedPrivateLayout(t, r.invocation).DefaultBareRemote
	if got := runGit(t, b, "config", "--get", "remote.dckt.url"); got != store {
		t.Fatalf("clone B's dckt remote = %q, want the shared store %q", got, store)
	}

	// Each run context is minted in its own clone and lands under that clone's
	// private state folder, never a docket-named one.
	tokens := [2]string{"ctx-a", "ctx-b"}
	for i, node := range []realNode{nodeA, nodeB} {
		mintRunTrackerWithHash(t, node.dir, runTrackerHashToken(tokens[i]), false)
		common := runGit(t, node.dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if _, err := os.Stat(filepath.Join(common, layout.PrivateName, runTrackerDirName)); err != nil {
			t.Fatalf("clone %d's run tracker is not under .git/%s: %v", i, layout.PrivateName, err)
		}
		if _, err := os.Lstat(filepath.Join(common, "docket")); !os.IsNotExist(err) {
			t.Fatalf("clone %d has a .git/docket state folder (err=%v)", i, err)
		}
	}

	before := originTip(t, store, privateFixtureBranch)
	var (
		wg      sync.WaitGroup
		start   = make(chan struct{})
		results [2]ChangeClaimResult
	)
	for i, node := range []realNode{nodeA, nodeB} {
		i, node := i, node
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = ChangeClaim(context.Background(), node.deps, node.dir,
				ChangeClaimRequest{ID: id, Revision: rev, RunContext: tokens[i]})
		}()
	}
	close(start)
	wg.Wait()

	applied, contended := 0, 0
	for i, res := range results {
		switch {
		case res.Result == ResultApplied && res.Disposition == ClaimDispositionApplied:
			applied++
		case res.Result == ResultContended && res.Disposition == ClaimDispositionContended:
			contended++
		default:
			t.Errorf("contender %d = (%q, %q), want applied or contended (findings %v, failure %+v)", i, res.Result, res.Disposition, res.Findings, res.Failure)
		}
	}
	if applied != 1 || contended != 1 {
		t.Fatalf("concurrent private claims: applied=%d contended=%d, want exactly one of each", applied, contended)
	}

	// Exactly one claim commit landed on the store, and origin never saw it.
	if n := runGit(t, store, "rev-list", "--count", before+".."+privateFixtureBranch); n != "1" {
		t.Errorf("the store carries %s claim commits past the seed, want exactly 1", n)
	}
	for _, name := range []string{privateFixtureBranch, "docket"} {
		if _, err := tryGit(r.origin, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
			t.Errorf("origin has refs/heads/%s; private metadata never reaches origin", name)
		}
	}
	for _, dir := range []string{r.invocation, b} {
		assertNoDocketPathUnder(t, dir)
	}
}
