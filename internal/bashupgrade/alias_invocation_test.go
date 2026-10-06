package bashupgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/testsupport"
)

// The alias is only useful if the binary behaves the same whatever name it was
// invoked by. This uses the real built binary (shared with the upgrade tests via
// buildDocket's sync.Once, so it costs no extra build) behind a dckt symlink.
func TestAliasInvocationMatchesDocket(t *testing.T) {
	bin := buildDocket(t)
	dir := testsupport.TempDir(t)
	alias := filepath.Join(dir, install.AliasName)
	if err := os.Symlink(bin, alias); err != nil {
		t.Fatal(err)
	}
	run := func(path string) string {
		t.Helper()
		out, err := exec.Command(path, "version", "--json").Output()
		if err != nil {
			t.Fatalf("%s version --json: %v", path, err)
		}
		return string(out)
	}
	if got, want := run(alias), run(bin); got != want {
		t.Fatalf("dckt version --json = %q, docket version --json = %q", got, want)
	}
}
