//go:build integration

package gitcli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationRepoInitBareAndAddRemote proves InitBare creates missing
// parents, is idempotent on an existing bare repository, and refuses a
// non-bare one; and that AddRemote registers a remote RemoteURL reads back
// while refusing to overwrite an existing remote of the same name.
func TestIntegrationRepoInitBareAndAddRemote(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	bare := filepath.Join(testsupport.TempDir(t), "store", "o-r", "remote.git")
	if err := c.InitBare(ctx, bare); err != nil {
		t.Fatalf("InitBare (fresh, missing parents): %v", err)
	}
	if err := c.InitBare(ctx, bare); err != nil {
		t.Fatalf("InitBare (idempotent): %v", err)
	}
	if got := strings.TrimSpace(gitOut(t, bare, "rev-parse", "--is-bare-repository")); got != "true" {
		t.Fatalf("is-bare-repository = %q, want true", got)
	}

	if err := c.AddRemote(ctx, repo, "dckt", bare); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	url, err := c.RemoteURL(ctx, repo, "dckt")
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if url != bare {
		t.Fatalf("RemoteURL = %q, want %q", url, bare)
	}
	if err := c.AddRemote(ctx, repo, "dckt", bare); err == nil {
		t.Fatalf("second AddRemote with the same name succeeded, want failure")
	}

	if err := c.InitBare(ctx, r.Writer); err == nil {
		t.Fatalf("InitBare on a non-bare repository succeeded, want failure")
	}
}

// TestIntegrationRepoRemoveRemote proves RemoveRemote deletes a configured
// remote and its tracking refs, after which RemoteURL reports it unconfigured,
// and that removing an unconfigured remote is remote-unavailable.
func TestIntegrationRepoRemoveRemote(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	bare := filepath.Join(testsupport.TempDir(t), "remote.git")
	if err := c.InitBare(ctx, bare); err != nil {
		t.Fatalf("InitBare: %v", err)
	}
	if err := c.AddRemote(ctx, repo, "dckt", bare); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	gitOut(t, r.Invocation, "push", "-q", "dckt", "main")
	gitOut(t, r.Invocation, "fetch", "-q", "dckt")
	if _, err := gitTry(r.Invocation, "rev-parse", "--verify", "refs/remotes/dckt/main"); err != nil {
		t.Fatalf("tracking ref missing before removal: %v", err)
	}

	if err := c.RemoveRemote(ctx, repo, "dckt"); err != nil {
		t.Fatalf("RemoveRemote: %v", err)
	}
	_, err := c.RemoteURL(ctx, repo, "dckt")
	assertKind(t, err, KindRemoteUnavailable)
	if _, err := gitTry(r.Invocation, "rev-parse", "--verify", "refs/remotes/dckt/main"); err == nil {
		t.Fatalf("tracking ref refs/remotes/dckt/main survived RemoveRemote")
	}

	assertKind(t, c.RemoveRemote(ctx, repo, "dckt"), KindRemoteUnavailable)
	assertKind(t, c.RemoveRemote(ctx, repo, "-x"), KindInvalidRequest)
}
