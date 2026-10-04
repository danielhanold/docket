package bashupgrade

// Restore and sandbox helpers shared by the default-tag unit tests and the
// integration-tagged upgrade proof. A saved case (testdata/bash-upgrade/<tag>/, see
// that tree's PROVENANCE.md) is restored into a per-test sandbox: the home.tar
// harness folders become a throwaway HOME, origin.bundle becomes a local bare
// origin, and clone-config.txt is replayed on a fresh clone so the clone looks the
// way the Bash tag left it. Every subprocess runs with upgradeCase.Env, which is
// built from nothing (never os.Environ), so no real HOME, XDG dir or DOCKET_*
// variable can leak in.

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/danielhanold/docket/internal/testsupport"
)

// homeToken stands for the sandbox HOME in every saved home.tar symlink target and
// regular-file body; restoreCase substitutes the per-test home for it.
const homeToken = "@@SANDBOX_HOME@@"

// upgradeCase is one restored saved case. Root is the per-test sandbox root, Home
// the restored home, Origin the local bare origin, Clone the user's repository,
// BinDir $Home/.local/bin, Docket the freshly built binary (not yet in BinDir), and
// Env the complete environment for every subprocess.
type upgradeCase struct {
	Tag, Root, Home, Origin, Clone, BinDir string
	Env                                    []string
	Docket                                 string
}

type cmdResult struct {
	Stdout, Stderr string
	Code           int
}

type findingRef struct {
	Code, Severity string
}

// cloneAction is one parsed clone-config.txt line: Kind "config" (A key, B value),
// Kind "worktree" (A path relative to the clone root, B branch), or Kind
// "hooks-off" (A worktree path relative to the clone root, B empty).
type cloneAction struct {
	Kind, A, B string
}

// substituteHome replaces every homeToken in b with home.
func substituteHome(b []byte, home string) []byte {
	return bytes.ReplaceAll(b, []byte(homeToken), []byte(home))
}

// parseCloneConfig parses clone-config.txt: blank lines and # comments are
// skipped; every other line is exactly `config <key> <value>`,
// `worktree <path> <branch>` or `hooks-off <path>`, with a relative,
// non-escaping worktree path.
//
// hooks-off exists because the Bash tags' disable-worktree-hooks.sh sets a
// WORKTREE-scoped core.hooksPath (pointing at <git-common-dir>/docket/empty-hooks,
// a path inside the clone) that a `git config --local` line cannot express.
// Without it the restored .docket worktree has hooks enabled, a state no Bash
// user's clone was ever in.
func parseCloneConfig(s string) ([]cloneAction, error) {
	var out []cloneAction
	for i, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		f := strings.Fields(trimmed)
		want := map[string]int{"config": 3, "worktree": 3, "hooks-off": 2}[f[0]]
		if want == 0 || len(f) != want {
			return nil, fmt.Errorf("clone-config line %d: %q is not `config <key> <value>`, `worktree <path> <branch>` or `hooks-off <path>`", i+1, line)
		}
		if f[0] != "config" && !localRelPath(f[1]) {
			return nil, fmt.Errorf("clone-config line %d: worktree path %q must be relative and stay inside the clone", i+1, f[1])
		}
		a := cloneAction{Kind: f[0], A: f[1]}
		if len(f) == 3 {
			a.B = f[2]
		}
		out = append(out, a)
	}
	return out, nil
}

// localRelPath reports whether p is a relative slash path that cannot escape its
// base (no leading /, no .. element).
func localRelPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") {
		return false
	}
	for _, el := range strings.Split(p, "/") {
		if el == ".." {
			return false
		}
	}
	return true
}

// findingCodes decodes one JSON document and collects, depth first in document
// order (map keys sorted), every object whose code and severity fields are both
// strings. It does not depend on the wrapper shape of any one command's output.
func findingCodes(t *testing.T, stdout string) []findingRef {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, stdout)
	}
	var out []findingRef
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			code, okCode := x["code"].(string)
			sev, okSev := x["severity"].(string)
			if okCode && okSev {
				out = append(out, findingRef{Code: code, Severity: sev})
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(doc)
	return out
}

// repoRoot returns the module root: this file lives at
// internal/bashupgrade/restore_test.go, two directories below it.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the repo root")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolved repo root %q has no go.mod: %v", root, err)
	}
	return root
}

// savedTags lists the testdata/bash-upgrade/ subdirectories that hold an
// origin.bundle, sorted. It is derived, never hand-listed, so a newly saved tag is
// picked up with no test edit; an empty list fails (population floor).
func savedTags(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "testdata", "bash-upgrade")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read saved cases: %v", err)
	}
	var tags []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "origin.bundle")); err == nil {
			tags = append(tags, e.Name())
		}
	}
	sort.Strings(tags)
	if len(tags) == 0 {
		t.Fatalf("population floor: no saved case with an origin.bundle under %s", dir)
	}
	return tags
}

var (
	buildOnce sync.Once
	buildDir  string
	buildBin  string
	buildErr  error
)

// TestMain removes the process-lifetime build dir buildDocket created.
func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

// buildDocket builds ./cmd/docket from this checkout once per test binary and
// returns the binary's absolute path. A build failure fails every caller.
func buildDocket(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	buildOnce.Do(func() {
		// tempdir-exempt: process-lifetime binary dir shared by every test under sync.Once; TestMain removes it.
		dir, err := os.MkdirTemp("", "docket-bashupgrade-bin-*")
		if err != nil {
			buildErr = err
			return
		}
		buildDir = dir
		bin := filepath.Join(dir, "docket")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/docket")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build ./cmd/docket: %v\n%s", err, out)
			return
		}
		buildBin = bin
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return buildBin
}

// restoreCase restores the saved case for tag into a fresh per-test sandbox.
func restoreCase(t *testing.T, tag string) *upgradeCase {
	t.Helper()
	root, err := filepath.EvalSymlinks(testsupport.TempDir(t))
	if err != nil {
		t.Fatalf("canonicalize sandbox root: %v", err)
	}
	home := filepath.Join(root, "home")
	c := &upgradeCase{
		Tag:    tag,
		Root:   root,
		Home:   home,
		Origin: filepath.Join(root, "origin.git"),
		Clone:  filepath.Join(home, "dev", "sample"),
		BinDir: filepath.Join(home, ".local", "bin"),
	}
	saved := filepath.Join(repoRoot(t), "testdata", "bash-upgrade", tag)

	extractHome(t, filepath.Join(saved, "home.tar"), home)
	assertNoHomeToken(t, home)

	xdg := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
		"XDG_RUNTIME_DIR": filepath.Join(home, ".local", "runtime"),
		"XDG_BIN_HOME":    c.BinDir,
	}
	tmp := filepath.Join(root, "tmp")
	for _, d := range append([]string{tmp}, mapValues(xdg)...) {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("create sandbox dir: %v", err)
		}
	}
	env := []string{
		"HOME=" + home, "USER=fixture", "LOGNAME=fixture", "SHELL=/bin/zsh", "TERM=dumb", "LANG=en_US.UTF-8",
		"TMPDIR=" + tmp,
		"PATH=" + c.BinDir + ":/usr/bin:/bin:/usr/sbin:/sbin",
	}
	for _, k := range sortedKeys(xdg) {
		env = append(env, k+"="+xdg[k])
	}
	c.Env = append(env, testsupport.GitEnv(t)...)
	c.Docket = buildDocket(t)

	c.mustGit(t, root, "init", "--quiet", "--bare", "-b", "main", c.Origin)
	c.mustGit(t, c.Origin, "fetch", "--quiet", filepath.Join(saved, "origin.bundle"), "refs/heads/*:refs/heads/*")
	c.mustGit(t, root, "clone", "--quiet", c.Origin, c.Clone)
	c.mustGit(t, c.Clone, "remote", "set-head", "origin", "main")

	raw, err := os.ReadFile(filepath.Join(saved, "clone-config.txt"))
	if err != nil {
		t.Fatalf("read clone-config.txt: %v", err)
	}
	actions, err := parseCloneConfig(string(raw))
	if err != nil {
		t.Fatalf("%s/clone-config.txt: %v", tag, err)
	}
	for _, a := range actions {
		switch a.Kind {
		case "config":
			c.mustGit(t, c.Clone, "config", "--local", a.A, a.B)
		case "worktree":
			// git's own DWIM creates a missing local branch tracking origin/<branch>.
			c.mustGit(t, c.Clone, "worktree", "add", "--quiet", a.A, a.B)
		case "hooks-off":
			// Mirrors the tags' disable-worktree-hooks.sh: an empty hooks dir under
			// the git common dir, named by the worktree-scoped core.hooksPath. The
			// tag's script also enables extensions.worktreeConfig; clone-config.txt
			// carries that as its own config line, which must come first.
			wt := filepath.Join(c.Clone, filepath.FromSlash(a.A))
			common := strings.TrimSpace(c.mustGit(t, wt, "rev-parse", "--path-format=absolute", "--git-common-dir"))
			empty := filepath.Join(common, "docket", "empty-hooks")
			if err := os.MkdirAll(empty, 0o755); err != nil {
				t.Fatalf("create empty hooks dir: %v", err)
			}
			c.mustGit(t, wt, "config", "--worktree", "core.hooksPath", empty)
		}
	}
	return c
}

// extractHome unpacks home.tar under home, substituting homeToken in symlink
// targets and regular-file bodies. A member name that is absolute, escapes with
// .., or would be written through an already-restored symlink is refused.
func extractHome(t *testing.T, tarPath, home string) {
	t.Helper()
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("open home.tar: %v", err)
	}
	defer f.Close()
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(f)
	members := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read home.tar: %v", err)
		}
		name := strings.TrimSuffix(h.Name, "/")
		if !localRelPath(name) || name == "." {
			t.Fatalf("home.tar member %q is absolute or escapes the home", h.Name)
		}
		dst := filepath.Join(home, filepath.FromSlash(name))
		mustNotTraverseSymlink(t, home, path.Dir(name))
		mode := fs.FileMode(h.Mode).Perm()
		switch h.Typeflag {
		case tar.TypeDir:
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(dst, mode); err != nil {
				t.Fatalf("restore dir %s: %v", name, err)
			}
		case tar.TypeReg:
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read member %s: %v", name, err)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, substituteHome(body, home), mode); err != nil {
				t.Fatalf("restore file %s: %v", name, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(string(substituteHome([]byte(h.Linkname), home)), dst); err != nil {
				t.Fatalf("restore symlink %s: %v", name, err)
			}
		default:
			t.Fatalf("home.tar member %q has unsupported type %q", h.Name, h.Typeflag)
		}
		members++
	}
	if members == 0 {
		t.Fatalf("home.tar %s is empty", tarPath)
	}
}

// mustNotTraverseSymlink fails when any existing component of rel (under home) is
// a symlink, so no member is written outside the home through a restored link.
func mustNotTraverseSymlink(t *testing.T, home, rel string) {
	t.Helper()
	if rel == "." || rel == "" {
		return
	}
	cur := home
	for _, el := range strings.Split(rel, "/") {
		cur = filepath.Join(cur, el)
		fi, err := os.Lstat(cur)
		if err != nil {
			return // not created yet; MkdirAll makes a real directory
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			t.Fatalf("home.tar writes through the restored symlink %s", cur)
		}
	}
}

// assertNoHomeToken fails when any regular file body or symlink target under home
// still holds homeToken: a surviving placeholder makes every later assert a test
// of a broken home.
func assertNoHomeToken(t *testing.T, home string) {
	t.Helper()
	var left []string
	files, links := 0, 0
	err := filepath.WalkDir(home, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			links++
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			if strings.Contains(target, homeToken) {
				left = append(left, p+" -> "+target)
			}
		case d.Type().IsRegular():
			files++
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if bytes.Contains(b, []byte(homeToken)) {
				left = append(left, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk restored home: %v", err)
	}
	if files == 0 || links == 0 {
		t.Fatalf("restored home has %d files and %d symlinks; the placeholder check would be vacuous", files, links)
	}
	if len(left) > 0 {
		t.Fatalf("%s survived restore in:\n%s", homeToken, strings.Join(left, "\n"))
	}
}

// run executes name with args in dir under c.Env. A bare name is resolved on
// c.Env's PATH (not the test process's), so the case's own tools are used.
func (c *upgradeCase) run(t *testing.T, dir string, name string, args ...string) cmdResult {
	t.Helper()
	bin := name
	if !strings.Contains(name, "/") {
		bin = c.lookPath(t, name)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = c.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	res := cmdResult{}
	err := cmd.Run()
	res.Stdout, res.Stderr = stdout.String(), stderr.String()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.Code = exitErr.ExitCode()
	default:
		t.Fatalf("run %s %v: %v", name, args, err)
	}
	return res
}

// mustGit runs git in dir and fails the test on a non-zero exit.
func (c *upgradeCase) mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	r := c.run(t, dir, "git", args...)
	if r.Code != 0 {
		t.Fatalf("git %s (in %s) exited %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), dir, r.Code, r.Stdout, r.Stderr)
	}
	return r.Stdout
}

func (c *upgradeCase) lookPath(t *testing.T, name string) string {
	t.Helper()
	for _, kv := range c.Env {
		if !strings.HasPrefix(kv, "PATH=") {
			continue
		}
		for _, d := range filepath.SplitList(strings.TrimPrefix(kv, "PATH=")) {
			p := filepath.Join(d, name)
			if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0 {
				return p
			}
		}
	}
	t.Fatalf("%s is not on the case PATH", name)
	return ""
}

// readRecords returns the saved records.txt inventory for tag (comments and blank
// lines dropped), in file order.
func readRecords(t *testing.T, tag string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "bash-upgrade", tag, "records.txt"))
	if err != nil {
		t.Fatalf("read records.txt: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// listRecords computes the records.txt inventory from the case's origin: every
// change record, ADR, spec and learning on the docket and main branches, prefixed
// with its branch and sorted bytewise (LC_ALL=C sort). Ledger dirs come from
// .docket.yml on main when it sets them, else the defaults.
func listRecords(t *testing.T, c *upgradeCase) []string {
	t.Helper()
	changes, adrs := "docs/changes", "docs/adrs"
	if r := c.run(t, c.Origin, "git", "show", "main:.docket.yml"); r.Code == 0 {
		var cfg struct {
			ChangesDir string `yaml:"changes_dir"`
			AdrsDir    string `yaml:"adrs_dir"`
		}
		if err := yaml.Unmarshal([]byte(r.Stdout), &cfg); err != nil {
			t.Fatalf("parse main:.docket.yml: %v", err)
		}
		if cfg.ChangesDir != "" {
			changes = strings.TrimSuffix(cfg.ChangesDir, "/")
		}
		if cfg.AdrsDir != "" {
			adrs = strings.TrimSuffix(cfg.AdrsDir, "/")
		}
	}
	patterns := []string{
		changes + "/active/*.md",
		changes + "/archive/*.md",
		changes + "/learnings/*.md",
		adrs + "/[0-9]*.md",
		"docs/superpowers/specs/*.md",
	}
	var out []string
	for _, branch := range []string{"docket", "main"} {
		for _, p := range strings.Split(c.mustGit(t, c.Origin, "ls-tree", "-r", "--name-only", branch), "\n") {
			if p == "" {
				continue
			}
			if base := path.Base(p); base == "README.md" || base == "BOARD.md" {
				continue
			}
			for _, pat := range patterns {
				if ok, _ := path.Match(pat, p); ok {
					out = append(out, branch+":"+p)
					break
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mapValues(m map[string]string) []string {
	var out []string
	for _, k := range sortedKeys(m) {
		out = append(out, m[k])
	}
	return out
}
