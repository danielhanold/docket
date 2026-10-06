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
