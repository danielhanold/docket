package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newRepoDirProbe builds a minimal command carrying the defaulted flag, mirroring
// how the real subcommands declare it.
func newRepoDirProbe() *cobra.Command {
	c := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
	c.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	return c
}

// TestResolveRepoDirExplicit: a non-empty explicit value is returned verbatim,
// with no cleaning, discovery, or substitution.
func TestResolveRepoDirExplicit(t *testing.T) {
	c := newRepoDirProbe()
	if err := c.Flags().Set("repo-dir", "/some/explicit/dir"); err != nil {
		t.Fatal(err)
	}
	got, err := resolveRepoDir(c)
	if err != nil || got != "/some/explicit/dir" {
		t.Fatalf("explicit value not returned verbatim: got=%q err=%v", got, err)
	}
}

// TestResolveRepoDirDefaultsToCwd: an omitted flag resolves through the process
// working directory — the invocation directory, not a Git-discovered root.
func TestResolveRepoDirDefaultsToCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	got, err := resolveRepoDir(newRepoDirProbe())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wd, _ := os.Getwd() // compare against the same canonicalization Getwd applies
	if got != wd {
		t.Fatalf("omitted flag did not resolve to the working directory: got=%q want=%q", got, wd)
	}
}

// TestResolveRepoDirCwdFailure: when the current directory cannot be determined,
// the resolver returns an argument error before any operation runs.
func TestResolveRepoDirCwdFailure(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Skipf("cannot remove cwd on this platform: %v", err)
	}
	// Some platforms (notably darwin) still resolve a removed working directory
	// through the still-open cwd vnode, so os.Getwd never errors and the
	// unresolvable-cwd precondition cannot be manufactured here. Skip in that
	// case, mirroring the sibling remove skip above; where Getwd does fail
	// (e.g. Linux), the assertion below still pins that the resolver propagates
	// the error rather than swallowing it.
	if _, err := os.Getwd(); err == nil {
		t.Skip("platform still resolves a removed working directory; unresolvable-cwd precondition unavailable")
	}
	if _, err := resolveRepoDir(newRepoDirProbe()); err == nil {
		t.Fatal("expected an error when the working directory is unresolvable")
	}
}

// repoDirDefaultMatchesExplicit runs one command twice — once with an explicit
// --repo-dir and once with the flag omitted from that same directory — and
// requires byte-identical documents. Deleting the resolver reverts the no-flag
// run to the empty-path failure, so this assert pins the RESOLVED non-default
// value, not merely "some output appeared".
func repoDirDefaultMatchesExplicit(t *testing.T, args ...string) {
	t.Helper()
	dir := t.TempDir()
	explicit, _, _ := runCLI(t, append(append([]string{}, args...), "--repo-dir", dir, "--json")...)
	t.Chdir(dir)
	defaulted, _, _ := runCLI(t, append(append([]string{}, args...), "--json")...)
	if defaulted != explicit {
		t.Fatalf("omitted --repo-dir diverges from explicit:\nexplicit:  %q\ndefaulted: %q", explicit, defaulted)
	}
	if strings.Contains(defaulted, "invocation path is empty") {
		t.Fatalf("empty repo-dir leaked into the operation: %q", defaulted)
	}
}

// TestContextFinalizeRepoDirDefault: the read-only finalize context reaches
// repository discovery from the working directory (no pr-unknown degrade from an
// empty path).
func TestContextFinalizeRepoDirDefault(t *testing.T) {
	repoDirDefaultMatchesExplicit(t, "context", "finalize")
}

// TestFinalizeMergeRepoDirDefault: the mutating merge verb resolves the same way.
func TestFinalizeMergeRepoDirDefault(t *testing.T) {
	repoDirDefaultMatchesExplicit(t, "finalize", "merge", "--id", "1", "--version", "v", "--head", strings.Repeat("a", 40))
}

// TestStatusRepoDirDefault: at least one NON-finalize command family shares the
// contract, so the resolver cannot regress into a finalize-only special case.
func TestStatusRepoDirDefault(t *testing.T) {
	repoDirDefaultMatchesExplicit(t, "status")
}
