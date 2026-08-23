package install_test

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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
}

const (
	testSHA     = "84a10275ffe1aa1242e33386da5be2bd52806b2b"
	buildinfoPk = "github.com/danielhanold/docket/internal/buildinfo"
)

func TestDevInstallStampsIdentity(t *testing.T) {
	cases := []struct {
		name, describe string
		wantVersion    string
		wantCommit     string
	}{
		{"tagged clean", "v0.3.0\n", "v0.3.0", testSHA},
		{"past a tag, clean", "v0.3.0-12-g84a1027\n", "v0.3.0-12-g84a1027", testSHA},
		{"no tags, dirty", "84a1027-dirty\n", "84a1027-dirty", testSHA + "-dirty"},
		{"tagged, dirty", "v0.3.0-dirty\n", "v0.3.0-dirty", testSHA + "-dirty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			mkdirAll(t, w.path(".toy"))
			src := newSource(t)
			g := &goRun{body: "binary\n"}
			git := &gitRun{out: map[string]string{
				"describe":  tc.describe,
				"rev-parse": testSHA + "\n",
			}}
			o := w.devOptions(t, src, filepath.Join(w.home, "bin"), g)
			o.GitRunner = git.runner()

			out := install.DevelopmentInstall(o)
			if out.Err != nil {
				t.Fatalf("DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
			}

			// Both probes ran in the canonical source root, argv-shaped.
			wantProbes := [][]string{
				{"git", "describe", "--tags", "--always", "--dirty"},
				{"git", "rev-parse", "HEAD"},
			}
			if len(git.argvs) != len(wantProbes) {
				t.Fatalf("git probes = %q, want %q", git.argvs, wantProbes)
			}
			for i, want := range wantProbes {
				if strings.Join(git.argvs[i], " ") != strings.Join(want, " ") {
					t.Errorf("probe %d = %q, want %q", i, git.argvs[i], want)
				}
				if git.dirs[i] != src {
					t.Errorf("probe %d ran in %s, want %s", i, git.dirs[i], src)
				}
			}

			// The build argv gained exactly one flag pair before -o.
			if len(g.argv) != 7 || g.argv[0] != "go" || g.argv[1] != "build" ||
				g.argv[2] != "-ldflags" || g.argv[4] != "-o" || g.argv[6] != "./cmd/docket" {
				t.Fatalf("argv = %q", g.argv)
			}
			wantPrefix := "-X " + buildinfoPk + ".Version=" + tc.wantVersion +
				" -X " + buildinfoPk + ".Commit=" + tc.wantCommit +
				" -X " + buildinfoPk + ".BuildDate="
			if !strings.HasPrefix(g.argv[3], wantPrefix) {
				t.Fatalf("ldflags = %q, want prefix %q", g.argv[3], wantPrefix)
			}
			stamped := strings.TrimPrefix(g.argv[3], wantPrefix)
			if _, err := time.Parse(time.RFC3339, stamped); err != nil || !strings.HasSuffix(stamped, "Z") {
				t.Fatalf("BuildDate = %q, want UTC RFC3339 (parse err %v)", stamped, err)
			}
		})
	}
}

func TestDevInstallUnstampedOnGitFailure(t *testing.T) {
	probeErr := errors.New("git failed")
	cases := []struct {
		name string
		git  *gitRun
	}{
		{"describe errors", &gitRun{errs: map[string]error{"describe": probeErr},
			out: map[string]string{"rev-parse": testSHA + "\n"}}},
		{"rev-parse errors", &gitRun{errs: map[string]error{"rev-parse": probeErr},
			out: map[string]string{"describe": "v0.3.0\n"}}},
		{"empty describe output", &gitRun{out: map[string]string{
			"describe": "\n", "rev-parse": testSHA + "\n"}}},
		{"garbage with embedded space", &gitRun{out: map[string]string{
			"describe": "fatal: not a git repository\n", "rev-parse": testSHA + "\n"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			mkdirAll(t, w.path(".toy"))
			src := newSource(t)
			g := &goRun{body: "binary\n"}
			o := w.devOptions(t, src, filepath.Join(w.home, "bin"), g)
			o.GitRunner = tc.git.runner()

			out := install.DevelopmentInstall(o)
			if out.Err != nil {
				t.Fatalf("a git failure must never fail the install: %v (reason %q)", out.Err, out.Reason)
			}
			// All three or none: the bare five-element build, no -ldflags at all.
			if len(g.argv) != 5 {
				t.Fatalf("argv = %q, want the bare 5-element build", g.argv)
			}
			for _, a := range g.argv {
				if strings.Contains(a, "-ldflags") || strings.Contains(a, "buildinfo") {
					t.Fatalf("argv = %q carries a partial stamp", g.argv)
				}
			}
		})
	}
}
