//go:build integration

package gitcli

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"path/filepath"
	"reflect"
	"testing"
)

// TestIntegrationListRemoteHeadsCompleteAdvertisement proves one ls-remote --heads call
// returns the whole heads advertisement: an origin carrying main, docket, and
// feature/x yields a map with exactly those three fully qualified refs, each at
// the exact full object id the origin's own ref holds (oracle read straight
// from origin, never from the adapter under test).
func TestIntegrationListRemoteHeadsCompleteAdvertisement(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	r.writerCommit(t, "docket", map[string]string{"d.md": "d\n"})
	r.writerCommit(t, "feature/x", map[string]string{"fx.md": "fx\n"})
	repo := mustDiscover(t, c, r.Invocation)

	want := map[RefName]ObjectID{
		"refs/heads/main":      ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/main")),
		"refs/heads/docket":    ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/docket")),
		"refs/heads/feature/x": ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/feature/x")),
	}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if err != nil {
		t.Fatalf("ListRemoteHeads: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("advertisement has %d refs, want %d: %+v", len(got), len(want), got)
	}
	for ref, wantOID := range want {
		gotOID, ok := got[ref]
		if !ok {
			t.Errorf("advertisement missing %q", ref)
			continue
		}
		if gotOID != wantOID {
			t.Errorf("%q = %q, want %q", ref, gotOID, wantOID)
		}
	}
}

// TestIntegrationListRemoteHeadsEmptyOriginIsEmptyMapNotError proves a bare origin with no
// refs yields a clean empty NON-NIL map, never an error and never a nil map —
// absence of heads is a proven emptiness, not an unknown.
func TestIntegrationListRemoteHeadsEmptyOriginIsEmptyMapNotError(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)

	root := testsupport.TempDir(t)
	origin := filepath.Join(root, "empty.git")
	gitOut(t, root, "init", "--bare", "-b", "main", origin)
	inv := filepath.Join(root, "inv")
	gitOut(t, root, "clone", "-q", origin, inv)
	configRepoIdentity(t, inv)
	repo := Repository{PrimaryWorktree: inv}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if err != nil {
		t.Fatalf("ListRemoteHeads on empty origin: %v", err)
	}
	if got == nil {
		t.Fatal("empty advertisement returned a nil map, want empty non-nil")
	}
	if len(got) != 0 {
		t.Fatalf("empty origin advertised %d refs: %+v", len(got), got)
	}
}

// TestIntegrationListRemoteHeadsMalformedLineIsFailureNotPartial proves a single malformed
// advertisement line (an abbreviated/non-hex object id) fails the whole read as
// invalid-output — never a partial map that would understate the inventory.
func TestIntegrationListRemoteHeadsMalformedLineIsFailureNotPartial(t *testing.T) {
	ctx := context.Background()
	c := helperClient(t, "script",
		"GITCLI_HELPER_STDOUT=deadbeef\trefs/heads/main\n")
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if got != nil {
		t.Fatalf("malformed line produced a partial map: %+v", got)
	}
	assertKind(t, err, KindInvalidOutput)
}

// TestIntegrationListRemoteHeadsDuplicateRefIsFailure proves the same ref advertised twice
// is refused as a typed *Failure — a duplicated inventory line is never silently
// collapsed.
func TestIntegrationListRemoteHeadsDuplicateRefIsFailure(t *testing.T) {
	ctx := context.Background()
	const oid = "1111111111111111111111111111111111111111"
	c := helperClient(t, "script",
		"GITCLI_HELPER_STDOUT="+oid+"\trefs/heads/main\n"+oid+"\trefs/heads/main\n")
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if got != nil {
		t.Fatalf("duplicate ref produced a map: %+v", got)
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("duplicate ref not reported as *Failure: %v", err)
	}
}

// TestIntegrationListRemoteHeadsTransportFailureIsError proves a non-zero exit from the
// advertisement command is command-failed — a failed shared inventory is
// unknown, never an empty advertisement the caller could read as "no heads".
func TestIntegrationListRemoteHeadsTransportFailureIsError(t *testing.T) {
	ctx := context.Background()
	c := helperClient(t, "exit", "GITCLI_HELPER_EXIT=128")
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if got != nil {
		t.Fatalf("transport failure produced a map: %+v", got)
	}
	assertKind(t, err, KindCommandFailed)
}

// TestIntegrationRepoListRemoteRefs proves ListRemoteRefs returns exactly the
// remote refs its patterns name, each at the exact object id the origin holds
// (oracle read straight from origin): docket-named branches and a refs/docket/
// namespace ref match, while an unrelated feature branch does not. An
// unreachable remote is an error, never an empty map a caller could read as
// "no such refs", and a dash-led pattern is refused before git runs.
func TestIntegrationRepoListRemoteRefs(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	r.writerCommit(t, "docket", map[string]string{"d.md": "d\n"})
	r.writerCommit(t, "dckt", map[string]string{"k.md": "k\n"})
	r.writerCommit(t, "feature/q", map[string]string{"q.md": "q\n"})
	gitOut(t, r.Writer, "push", "-q", "origin", "HEAD:refs/docket/x/y")
	repo := mustDiscover(t, c, r.Invocation)

	want := map[RefName]ObjectID{
		"refs/heads/docket": ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/docket")),
		"refs/heads/dckt":   ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/dckt")),
		"refs/docket/x/y":   ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/docket/x/y")),
	}
	got, err := c.ListRemoteRefs(ctx, repo, "origin", []string{"refs/heads/docket", "refs/heads/dckt", "refs/docket/*"})
	if err != nil {
		t.Fatalf("ListRemoteRefs: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListRemoteRefs = %+v, want %+v", got, want)
	}

	none, err := c.ListRemoteRefs(ctx, repo, "origin", []string{"refs/heads/no-such-branch"})
	if err != nil {
		t.Fatalf("ListRemoteRefs with no match: %v", err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("no match = %#v, want an empty non-nil map", none)
	}

	gitOut(t, r.Invocation, "remote", "add", "gone", filepath.Join(testsupport.TempDir(t), "missing.git"))
	unreachable, err := c.ListRemoteRefs(ctx, repo, "gone", []string{"refs/heads/docket"})
	if unreachable != nil {
		t.Fatalf("unreachable remote produced a map: %+v", unreachable)
	}
	assertKind(t, err, KindCommandFailed)

	for _, bad := range [][]string{{"-x"}, {""}, nil} {
		_, err := c.ListRemoteRefs(ctx, repo, "origin", bad)
		assertKind(t, err, KindInvalidRequest)
	}
}
