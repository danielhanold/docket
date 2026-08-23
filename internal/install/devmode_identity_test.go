package install_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/install"
)

// gitRun cans git output per subcommand (argv[1]) and records every probe the
// identity seam issues, so tests assert both the values and the argv shapes.
type gitRun struct {
	dirs  []string
	argvs [][]string
	out   map[string]string
	errs  map[string]error
}

func (g *gitRun) runner() func(string, []string) (string, error) {
	return func(dir string, argv []string) (string, error) {
		g.dirs = append(g.dirs, dir)
		g.argvs = append(g.argvs, append([]string(nil), argv...))
		key := ""
		if len(argv) > 1 {
			key = argv[1]
		}
		if err := g.errs[key]; err != nil {
			return "", err
		}
		return g.out[key], nil
	}
}

func TestDevInstallRequiresGitRunner(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	o := w.devOptions(t, src, filepath.Join(w.home, "bin"), &goRun{body: "binary\n"})
	o.GitRunner = nil

	out := install.DevelopmentInstall(o)
	if out.Err == nil || out.Reason != install.ReasonInvalidOptions {
		t.Fatalf("err=%v reason=%q, want a %q refusal", out.Err, out.Reason, install.ReasonInvalidOptions)
	}
}

func TestDefaultGitRunner(t *testing.T) {
	repo := t.TempDir()
	init := [][]string{
		{"git", "init", "-q"},
		{"git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "x"},
	}
	for _, argv := range init {
		if _, err := install.DefaultGitRunner(repo, argv); err != nil {
			t.Fatalf("%v: %v", argv, err)
		}
	}
	out, err := install.DefaultGitRunner(repo, []string{"git", "rev-parse", "HEAD"})
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	// Stdout only, machine-readable: a full SHA and git's own trailing newline.
	if !regexp.MustCompile(`^[0-9a-f]{40}\n$`).MatchString(out) {
		t.Fatalf("rev-parse output = %q, want a bare 40-hex SHA line", out)
	}

	if _, err := install.DefaultGitRunner(t.TempDir(), []string{"git", "rev-parse", "HEAD"}); err == nil {
		t.Fatal("rev-parse outside a repository must error, not succeed")
	}
	if _, err := install.DefaultGitRunner(repo, nil); err == nil {
		t.Fatal("an empty argv must error")
	}
	_ = strings.TrimSpace // keep the import stable for later tasks in this file
}
