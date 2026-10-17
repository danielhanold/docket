//go:build integration

package gitcli

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationRepoListLocalHeadsCompleteInventory proves one read returns every
// local branch at its exact full object id (oracle read via for-each-ref, never
// the adapter under test), and that no tag or remote-tracking ref leaks in.
func TestIntegrationRepoListLocalHeadsCompleteInventory(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	gitOut(t, r.Invocation, "branch", "feature/x")
	gitOut(t, r.Invocation, "branch", "feature/x-2")
	gitOut(t, r.Invocation, "tag", "v1")
	repo := mustDiscover(t, c, r.Invocation)

	want := map[RefName]ObjectID{}
	for _, line := range strings.Split(gitOut(t, r.Invocation, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		name, oid, _ := strings.Cut(line, " ")
		want[RefName(name)] = ObjectID(oid)
	}
	if _, ok := want["refs/heads/feature/x-2"]; !ok {
		t.Fatalf("oracle missing feature/x-2: %+v", want)
	}

	got, err := c.ListLocalHeads(ctx, repo)
	if err != nil {
		t.Fatalf("ListLocalHeads: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListLocalHeads = %+v, want %+v", got, want)
	}
	for ref := range got {
		if strings.HasPrefix(string(ref), "refs/remotes/") || strings.HasPrefix(string(ref), "refs/tags/") {
			t.Errorf("non-local-branch ref in inventory: %q", ref)
		}
	}
}

// TestIntegrationRepoListLocalHeadsNoBranchesIsEmptyMap proves a repository with no
// branches yields an empty NON-NIL map, never an error or nil.
func TestIntegrationRepoListLocalHeadsNoBranchesIsEmptyMap(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	root := testsupport.TempDir(t)
	inv := filepath.Join(root, "inv")
	gitOut(t, root, "init", "-q", "-b", "main", inv)
	repo := Repository{PrimaryWorktree: inv}

	got, err := c.ListLocalHeads(ctx, repo)
	if err != nil {
		t.Fatalf("ListLocalHeads on empty repo: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty repo = %#v, want empty non-nil map", got)
	}
}

// TestIntegrationRepoListLocalHeadsMalformedLineIsFailure proves any malformed line
// fails the whole read as invalid-output, never a partial map.
func TestIntegrationRepoListLocalHeadsMalformedLineIsFailure(t *testing.T) {
	const oid = "1111111111111111111111111111111111111111"
	cases := map[string]string{
		"short-oid": "deadbeef\trefs/heads/main\n",
		"non-heads": oid + "\trefs/tags/v1\n",
		"no-tab":    oid + " refs/heads/main\n",
	}
	for name, stdout := range cases {
		t.Run(name, func(t *testing.T) {
			c := helperClient(t, "script", "GITCLI_HELPER_STDOUT="+stdout)
			repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}
			got, err := c.ListLocalHeads(context.Background(), repo)
			if got != nil {
				t.Fatalf("malformed line produced a partial map: %+v", got)
			}
			assertKind(t, err, KindInvalidOutput)
		})
	}
}

// TestIntegrationRepoListLocalHeadsDuplicateIsFailure proves a repeated ref is refused.
func TestIntegrationRepoListLocalHeadsDuplicateIsFailure(t *testing.T) {
	const oid = "1111111111111111111111111111111111111111"
	c := helperClient(t, "script",
		"GITCLI_HELPER_STDOUT="+oid+"\trefs/heads/main\n"+oid+"\trefs/heads/main\n")
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}
	got, err := c.ListLocalHeads(context.Background(), repo)
	if got != nil {
		t.Fatalf("duplicate ref produced a map: %+v", got)
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("duplicate ref not reported as *Failure: %v", err)
	}
}

// TestIntegrationRepoListLocalHeadsCommandFailure proves a non-zero exit is
// command-failed, never an empty inventory.
func TestIntegrationRepoListLocalHeadsCommandFailure(t *testing.T) {
	c := helperClient(t, "exit", "GITCLI_HELPER_EXIT=128")
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}
	got, err := c.ListLocalHeads(context.Background(), repo)
	if got != nil {
		t.Fatalf("command failure produced a map: %+v", got)
	}
	assertKind(t, err, KindCommandFailed)
}
