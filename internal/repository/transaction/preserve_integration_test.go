//go:build integration

package transaction

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// TestIntegrationTxnApplyTransactionPreservesDirtyCheckout proves the engine leaves a working tree
// carrying staged, unstaged, AND untracked local edits byte-identical across an
// applied transaction. The engine works only in its private detached worktree and
// pushes to origin, so the invocation clone — dirty user work and all — must not
// move.
func TestIntegrationTxnApplyTransactionPreservesDirtyCheckout(t *testing.T) {
	requireGit(t)
	for _, topo := range topologies() {
		t.Run(topo.name, func(t *testing.T) {
			r := topo.build(t)
			client, err := gitcli.NewClient()
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			eng := newEngine(t, client)
			repo, dir := freshClone(t, client, r, "dirty")

			// Dirty the checkout three ways: a staged edit, an unstaged edit, and an
			// untracked file. The three must all survive byte-for-byte.
			staged := filepath.Join(dir, "staged-edit.md")
			if err := os.WriteFile(staged, []byte("staged local work\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			hgitOut(t, dir, "add", "--", "staged-edit.md")
			unstaged := filepath.Join(dir, "README-or-docket.md")
			if err := os.WriteFile(unstaged, []byte("unstaged local work\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			hgitOut(t, dir, "add", "--", "README-or-docket.md")
			if err := os.WriteFile(unstaged, []byte("unstaged local work\nMORE UNSTAGED\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			untracked := filepath.Join(dir, "untracked.local")
			if err := os.WriteFile(untracked, []byte("untracked local work\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			before := captureCheckouts(t, dir)

			op := createOp(thirdChangePath, thirdChange())
			res, err := eng.Execute(context.Background(), Request{
				Repository: repo, Remote: "origin", TargetRef: r.Target,
				Loader: testLoader{}, Operation: op,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Disposition != DispositionApplied {
				t.Fatalf("disposition = %q, want applied (findings %v)", res.Disposition, res.Findings)
			}

			// The transaction landed on origin, and the dirty checkout is untouched.
			assertCheckoutsUnchanged(t, []string{dir}, before)
			if !transactionsEmpty(t, repo) {
				t.Error("transactions root not empty after apply")
			}
		})
	}
}
