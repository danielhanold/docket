package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/testsupport"
)

// The alias is only useful if the binary behaves the same whatever name it was
// invoked by. This puts the binary TestMain builds once (binPath) behind a dckt
// symlink, so it costs no extra build.
func TestAliasInvocationMatchesDocket(t *testing.T) {
	bin := binPath
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
