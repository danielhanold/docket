//go:build integration

package gitcli

import (
	"context"
	"path/filepath"
	"testing"
)

// TestListRemoteHeadsCompleteAdvertisement proves one ls-remote --heads call
// returns the whole heads advertisement: an origin carrying main, docket, and
// feature/x yields a map with exactly those three fully qualified refs, each at
// the exact full object id the origin's own ref holds (oracle read straight
// from origin, never from the adapter under test).
func TestListRemoteHeadsCompleteAdvertisement(t *testing.T) {
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

// TestListRemoteHeadsEmptyOriginIsEmptyMapNotError proves a bare origin with no
// refs yields a clean empty NON-NIL map, never an error and never a nil map —
// absence of heads is a proven emptiness, not an unknown.
func TestListRemoteHeadsEmptyOriginIsEmptyMapNotError(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)

	root := t.TempDir()
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

// TestListRemoteHeadsMalformedLineIsFailureNotPartial proves a single malformed
// advertisement line (an abbreviated/non-hex object id) fails the whole read as
// invalid-output — never a partial map that would understate the inventory.
func TestListRemoteHeadsMalformedLineIsFailureNotPartial(t *testing.T) {
	ctx := context.Background()
	c := helperClient(t, "script",
		"GITCLI_HELPER_STDOUT=deadbeef\trefs/heads/main\n")
	repo := Repository{PrimaryWorktree: t.TempDir()}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if got != nil {
		t.Fatalf("malformed line produced a partial map: %+v", got)
	}
	assertKind(t, err, KindInvalidOutput)
}

// TestListRemoteHeadsDuplicateRefIsFailure proves the same ref advertised twice
// is refused as a typed *Failure — a duplicated inventory line is never silently
// collapsed.
func TestListRemoteHeadsDuplicateRefIsFailure(t *testing.T) {
	ctx := context.Background()
	const oid = "1111111111111111111111111111111111111111"
	c := helperClient(t, "script",
		"GITCLI_HELPER_STDOUT="+oid+"\trefs/heads/main\n"+oid+"\trefs/heads/main\n")
	repo := Repository{PrimaryWorktree: t.TempDir()}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if got != nil {
		t.Fatalf("duplicate ref produced a map: %+v", got)
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("duplicate ref not reported as *Failure: %v", err)
	}
}

// TestListRemoteHeadsTransportFailureIsError proves a non-zero exit from the
// advertisement command is command-failed — a failed shared inventory is
// unknown, never an empty advertisement the caller could read as "no heads".
func TestListRemoteHeadsTransportFailureIsError(t *testing.T) {
	ctx := context.Background()
	c := helperClient(t, "exit", "GITCLI_HELPER_EXIT=128")
	repo := Repository{PrimaryWorktree: t.TempDir()}

	got, err := c.ListRemoteHeads(ctx, repo, "origin")
	if got != nil {
		t.Fatalf("transport failure produced a map: %+v", got)
	}
	assertKind(t, err, KindCommandFailed)
}
