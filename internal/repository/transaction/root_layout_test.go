package transaction

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestTransactionsRootFollowsStateFolder proves the transactions root sits
// beneath the per-repo state folder: <common>/dckt/ when that directory exists,
// <common>/docket/ once it is gone.
func TestTransactionsRootFollowsStateFolder(t *testing.T) {
	common := testsupport.TempDir(t)
	repo := gitcli.Repository{CommonDir: common}
	priv := filepath.Join(common, "dckt")
	if err := os.Mkdir(priv, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := transactionsRoot(repo), filepath.Join(priv, "transactions"); got != want {
		t.Errorf("private transactionsRoot = %q, want %q", got, want)
	}
	if err := os.Remove(priv); err != nil {
		t.Fatal(err)
	}
	if got, want := transactionsRoot(repo), filepath.Join(common, "docket", "transactions"); got != want {
		t.Errorf("shared transactionsRoot = %q, want %q", got, want)
	}
}
