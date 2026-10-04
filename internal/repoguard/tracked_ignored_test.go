package repoguard

// Guard: no tracked file is hidden by the repository's own committed ignore
// rules (change 0504). A file that is tracked while a committed .gitignore rule
// still matches it is in the index only because of a one-time `git add -f`; a
// sibling added later is silently skipped. The durable form is a committed
// negation in a nested .gitignore beside the files, outside the managed docket
// block (learning gitignore-guarantee-must-be-committed). The live instance is
// the testdata/repositories/.gitignore negation that keeps the frozen
// .docket.local.yml fixtures committable.
//
// The probe asks git, never a path list: `git ls-files -z --cached --ignored
// --exclude-per-directory=.gitignore`. Only the repository's own .gitignore files
// decide — never --exclude-standard, so a developer's .git/info/exclude or
// user-global core.excludesFile can neither redden nor mask the guard.
//
// RESIDUAL (undetectable, not unprobed): a brand-new file that is ignored and
// never added is absent from every clone's committed state, so no test over the
// repository can see it. This guard catches every TRACKED instance and the
// deletion of any negation that protects one.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile is the committed
// non-vacuity control for TestNoTrackedFileIsIgnoredByCommittedRules: it runs the
// SAME helper over a throwaway repository and proves the probe reports an ignored
// tracked file, honors a nested negation, and ignores machine-local excludes.
func TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile(t *testing.T) {
	dir := testsupport.TempDir(t)
	elsewhere := testsupport.TempDir(t)
	env := []string{
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=docket test",
		"GIT_AUTHOR_EMAIL=test@docket.invalid",
		"GIT_COMMITTER_NAME=docket test",
		"GIT_COMMITTER_EMAIL=test@docket.invalid",
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(scrubGitRepoEnv(os.Environ()), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	probe := func(step string, want []string) {
		t.Helper()
		got, err := trackedButIgnored(dir, env)
		if err != nil {
			t.Fatalf("%s: probe failed: %v", step, err)
		}
		if len(got) == 0 && len(want) == 0 {
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: trackedButIgnored = %q, want %q", step, got, want)
		}
	}

	git("init", "-q")
	write("keep/local.yml", "a: 1\n")
	write("keep/other.txt", "x\n")
	git("add", ".")
	git("commit", "-q", "-m", "seed")
	probe("baseline (no ignore rules)", nil)

	// A committed rule that matches an already-tracked file: the probe must name it.
	write(".gitignore", "local.yml\n")
	git("add", ".gitignore")
	git("commit", "-q", "-m", "ignore local.yml")
	probe("committed rule matches a tracked file", []string{"keep/local.yml"})

	// A committed nested negation beside the file rescues it (the
	// testdata/repositories/.gitignore shape): nothing is reported.
	write("keep/.gitignore", "!local.yml\n")
	git("add", "keep/.gitignore")
	git("commit", "-q", "-m", "negate local.yml beside it")
	probe("nested negation rescues the file", nil)

	// Machine-local excludes never decide: neither .git/info/exclude nor a
	// core.excludesFile matching a tracked file is reported.
	write(".git/info/exclude", "other.txt\n")
	globalIgnore := filepath.Join(elsewhere, "global-ignore")
	if err := os.WriteFile(globalIgnore, []byte("other.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("config", "core.excludesFile", globalIgnore)
	probe("machine-local excludes are not consulted", nil)
}

// TestNoTrackedFileIsIgnoredByCommittedRules asserts that git reports no tracked
// file as ignored by the repository's own committed .gitignore files.
func TestNoTrackedFileIsIgnoredByCommittedRules(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	hidden, err := trackedButIgnored(root, nil)
	if err != nil {
		t.Fatalf("tracked-but-ignored probe failed (a failed probe is never an empty list): %v", err)
	}
	if len(hidden) > 0 {
		t.Fatalf("%d tracked file(s) are ignored by the repository's own committed .gitignore rules:\n  %s\n"+
			"A tracked file that an ignore rule still matches is in the index only because of a one-time "+
			"`git add -f`; a sibling added later is silently skipped. Remedy: add a committed negation "+
			"(for example `!<name>`) in a nested .gitignore beside the files, outside the managed docket "+
			"block, or stop tracking the file (learning: gitignore-guarantee-must-be-committed).",
			len(hidden), strings.Join(hidden, "\n  "))
	}
}

// trackedButIgnored returns, sorted and slash-separated, every tracked file in the
// repository at dir that the repository's own .gitignore files ignore. Only
// per-directory .gitignore files are consulted (never --exclude-standard). The
// output is NUL-delimited (-z), so paths are never C-quoted. Any git failure is
// returned as an error, never as an empty list. extraEnv is appended last.
func trackedButIgnored(dir string, extraEnv []string) ([]string, error) {
	cmd := exec.Command("git", "-C", dir, "ls-files", "-z", "--cached", "--ignored",
		"--exclude-per-directory=.gitignore")
	cmd.Env = append(scrubGitRepoEnv(os.Environ()), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w: %s", dir, err, strings.TrimSpace(stderr.String()))
	}
	var paths []string
	for _, p := range strings.Split(stdout.String(), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// scrubGitRepoEnv drops the variables that would redirect git away from the
// repository named by -C (an outer hook or tool may export them).
func scrubGitRepoEnv(env []string) []string {
	var out []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR":
			continue
		}
		out = append(out, kv)
	}
	return out
}
