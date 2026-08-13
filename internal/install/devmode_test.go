package install_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/install"
)

// ---------------------------------------------------------------------------
// A synthetic source checkout
// ---------------------------------------------------------------------------

// newSource builds a miniature docket checkout: the four authored asset roots
// plus the committed embedded bundle a real checkout carries. It is synthetic
// on purpose — a development install must be provable against a source tree the
// test controls byte for byte, not against the repo the tests happen to run in.
func newSource(t *testing.T) string {
	t.Helper()
	dir := canonical(t, t.TempDir())
	writeFile(t, filepath.Join(dir, "skills", "docket-toy", "SKILL.md"), "# toy skill\n")
	writeFile(t, filepath.Join(dir, "agents", "docket-toy.md"), agentSource("v1"))
	writeFile(t, filepath.Join(dir, "agents", "harness-defaults.yml"), "claude: {}\n")
	writeFile(t, filepath.Join(dir, "cursor-rules", "run-gate.md"), "# run gate\n")
	writeFile(t, filepath.Join(dir, ".docket.example.yml"), "version: 1\n")
	regenerateSource(t, dir)
	return dir
}

func agentSource(marker string) string {
	return "---\nname: docket-toy\ndescription: \"a toy agent\"\n---\n\nbody " + marker + "\n"
}

// regenerateSource rewrites the source's committed bundle, which is what
// `go generate ./internal/assets/` does in a real checkout.
func regenerateSource(t *testing.T, dir string) {
	t.Helper()
	m, payload, err := assets.Generate(dir, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := filepath.Join(dir, "internal", "assets", "embedded")
	if err := os.RemoveAll(out); err != nil {
		t.Fatalf("clearing %s: %v", out, err)
	}
	if err := assets.WriteTree(out, m, payload); err != nil {
		t.Fatalf("WriteTree: %v", err)
	}
}

func sourceDigest(t *testing.T, dir string) string {
	t.Helper()
	m, _, err := assets.Generate(dir, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	id, err := assets.ComputeAssetSetID(m)
	if err != nil {
		t.Fatalf("ComputeAssetSetID: %v", err)
	}
	return id
}

// devPlanner renders from whatever catalog it is handed, so a test can prove
// which catalog a development install actually planned with.
func devPlanner(t *testing.T, root string) install.Planner {
	return install.Planner{
		Name: "toy",
		Detect: func(install.UserRoots) (bool, string) {
			info, err := os.Stat(root)
			return err == nil && info.IsDir(), root
		},
		Plan: func(mode install.Mode, assetsDir string, cat assets.Catalog) ([]install.Target, error) {
			if mode != install.ModeDevelopment {
				t.Errorf("planner called with mode %q, want development", mode)
			}
			body, err := cat.Bytes("agents/docket-toy.md")
			if err != nil {
				return nil, err
			}
			return []install.Target{
				{
					Path:    filepath.Join(root, "agents", "docket-toy.md"),
					Kind:    install.KindFile,
					Content: body,
					Role:    "agent",
				},
				{
					Path:       filepath.Join(root, "skills", "docket-toy"),
					Kind:       install.KindSymlink,
					LinkTarget: filepath.Join(assetsDir, "skills", "docket-toy"),
					Role:       "skill",
				},
			}, nil
		},
	}
}

// goRun captures what the build seam was asked to run and stands in for the
// toolchain by writing the bytes a real `go build` would have produced.
type goRun struct {
	dir     string
	argv    []string
	calls   int
	body    string
	failure error
}

func (g *goRun) runner() func(string, []string) error {
	return func(dir string, argv []string) error {
		g.calls++
		g.dir = dir
		g.argv = append([]string(nil), argv...)
		if g.failure != nil {
			return g.failure
		}
		out, err := outputPath(argv)
		if err != nil {
			return err
		}
		return os.WriteFile(out, []byte(g.body), 0o755)
	}
}

func outputPath(argv []string) (string, error) {
	for i, a := range argv {
		if a == "-o" && i+1 < len(argv) {
			return argv[i+1], nil
		}
	}
	return "", errors.New("no -o in argv")
}

func (w *world) devOptions(t *testing.T, src, bin string, g *goRun) install.DevOptions {
	o := w.options(nil)
	o.Planners = []install.Planner{devPlanner(t, w.path(".toy"))}
	return install.DevOptions{Options: o, SourceRoot: src, BinDir: bin, GoRunner: g.runner()}
}

// ---------------------------------------------------------------------------
// Development install
// ---------------------------------------------------------------------------

func TestDevInstallLinksToSource(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	bin := filepath.Join(w.home, ".local", "bin")
	g := &goRun{body: "#!/bin/sh\necho docket\n"}

	out := install.DevelopmentInstall(w.devOptions(t, src, bin, g))
	if out.Err != nil {
		t.Fatalf("DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
	}
	if !out.Applied || out.Mode != install.ModeDevelopment {
		t.Fatalf("outcome = %+v", out)
	}

	// The skill link resolves into the checkout, hop by hop.
	link := w.path(".toy", "skills", "docket-toy")
	dest, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if want := filepath.Join(src, "skills", "docket-toy"); dest != want {
		t.Errorf("link -> %s, want %s", dest, want)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("resolving %s: %v", link, err)
	}
	if !strings.HasPrefix(resolved, src+string(filepath.Separator)) {
		t.Errorf("link resolves to %s, outside the checkout %s", resolved, src)
	}

	// The wrapper is rendered from the SOURCE's asset, not from the running
	// binary's embedded bundle.
	wrapper := filepath.Join(w.path(".toy"), "agents", "docket-toy.md")
	if got, want := readFile(t, wrapper), agentSource("v1"); got != want {
		t.Errorf("wrapper = %q, want the source asset %q", got, want)
	}

	// The built binary is an ordinary owned target, and it is executable.
	installed := filepath.Join(bin, "docket")
	if got := readFile(t, installed); got != g.body {
		t.Errorf("installed binary = %q, want the built bytes", got)
	}
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("stat %s: %v", installed, err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("installed binary mode = %v, want 0755", info.Mode().Perm())
	}

	state := loadState(t, w.roots)
	if state.Mode != install.ModeDevelopment || state.SourceRoot != src {
		t.Errorf("state = %+v", state)
	}
	if state.SourceDigest != sourceDigest(t, src) {
		t.Errorf("source digest = %q, want %q", state.SourceDigest, sourceDigest(t, src))
	}
	var sawBinary bool
	for _, rec := range state.Targets {
		if rec.Path == installed {
			sawBinary = true
		}
	}
	if !sawBinary {
		t.Errorf("the installed binary is not recorded in the state")
	}

	// An edited-and-regenerated source re-renders the wrapper: the source is
	// the truth in development mode.
	writeFile(t, filepath.Join(src, "agents", "docket-toy.md"), agentSource("v2"))
	regenerateSource(t, src)
	if out := install.DevelopmentInstall(w.devOptions(t, src, bin, g)); out.Err != nil {
		t.Fatalf("second DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
	}
	if got, want := readFile(t, wrapper), agentSource("v2"); got != want {
		t.Errorf("wrapper after a source edit = %q, want %q", got, want)
	}
}

func TestDevInstallRecordsSourceDigest(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	g := &goRun{body: "binary\n"}

	out := install.DevelopmentInstall(w.devOptions(t, src, filepath.Join(w.home, "bin"), g))
	if out.Err != nil {
		t.Fatalf("DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
	}
	want := sourceDigest(t, src)
	if out.AssetSetID != want {
		t.Errorf("outcome asset set id = %q, want the source digest %q", out.AssetSetID, want)
	}
	if got := loadState(t, w.roots).SourceDigest; got != want {
		t.Errorf("state source digest = %q, want %q", got, want)
	}
}

func TestDevInstallDriftRefuses(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	// An authored file edited without regenerating: the committed bundle no
	// longer describes the checkout, so nothing may be installed from it.
	writeFile(t, filepath.Join(src, "skills", "docket-toy", "SKILL.md"), "# toy skill edited\n")
	homeBefore := snapshot(t, w.home)
	srcBefore := snapshot(t, src)
	g := &goRun{body: "binary\n"}

	out := install.DevelopmentInstall(w.devOptions(t, src, filepath.Join(w.home, "bin"), g))
	if out.Reason != install.ReasonSourceAssetsDrifted {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, install.ReasonSourceAssetsDrifted, out.Err)
	}
	if g.calls != 0 {
		t.Errorf("a drifted source was still built")
	}
	assertUnchanged(t, homeBefore, snapshot(t, w.home), "drifted source")
	assertUnchanged(t, srcBefore, snapshot(t, src), "the source checkout")
	if len(out.Actions) == 0 {
		t.Errorf("drift reported no differing paths")
	}
}

func TestDevInstallProtocolMismatch(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	manifest := filepath.Join(src, "internal", "assets", "embedded", "manifest.json")
	body := readFile(t, manifest)
	writeFile(t, manifest, strings.Replace(body, `"asset_protocol": 1`, `"asset_protocol": 99`, 1))
	homeBefore := snapshot(t, w.home)
	g := &goRun{body: "binary\n"}

	out := install.DevelopmentInstall(w.devOptions(t, src, filepath.Join(w.home, "bin"), g))
	if out.Reason != install.ReasonAssetProtocolMismatch {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, install.ReasonAssetProtocolMismatch, out.Err)
	}
	assertUnchanged(t, homeBefore, snapshot(t, w.home), "protocol mismatch")
}

func TestDevInstallBuildFailureNoPublish(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	bin := filepath.Join(w.home, "bin")
	homeBefore := snapshot(t, w.home)
	g := &goRun{failure: errors.New("compile error")}

	out := install.DevelopmentInstall(w.devOptions(t, src, bin, g))
	if !errors.Is(out.Err, install.ErrBuildFailed) {
		t.Fatalf("err = %v, want ErrBuildFailed", out.Err)
	}
	if out.Reason != install.ReasonBuildFailed {
		t.Errorf("reason = %q", out.Reason)
	}
	if _, err := os.Stat(filepath.Join(bin, "docket")); !os.IsNotExist(err) {
		t.Errorf("a binary was installed despite a failed build: %v", err)
	}
	if _, err := os.Stat(w.roots.StatePath()); !os.IsNotExist(err) {
		t.Errorf("state published despite a failed build: %v", err)
	}
	assertUnchanged(t, homeBefore, snapshot(t, w.home), "failed build")
}

func TestDevInstallBuildsViaArgv(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	g := &goRun{body: "binary\n"}

	out := install.DevelopmentInstall(w.devOptions(t, src, filepath.Join(w.home, "bin"), g))
	if out.Err != nil {
		t.Fatalf("DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
	}
	if g.calls != 1 {
		t.Fatalf("build ran %d times", g.calls)
	}
	if g.dir != src {
		t.Errorf("build ran in %s, want the canonical source root %s", g.dir, src)
	}
	if len(g.argv) != 5 || g.argv[0] != "go" || g.argv[1] != "build" || g.argv[2] != "-o" || g.argv[4] != "./cmd/docket" {
		t.Fatalf("argv = %q", g.argv)
	}
	// An argument vector, never a shell string: no element may smuggle a
	// second command past the exec boundary.
	for _, a := range g.argv {
		if strings.ContainsAny(a, ";|&$`") || strings.Contains(a, " ") {
			t.Errorf("argv element %q carries shell syntax", a)
		}
	}
	// The build writes into a staging path the installer owns, never straight
	// into the user's bin directory.
	if strings.HasPrefix(g.argv[3], filepath.Join(w.home, "bin")+string(filepath.Separator)) {
		t.Errorf("the build wrote directly into the destination: %q", g.argv[3])
	}
}

func TestDevInstallMissingSource(t *testing.T) {
	good := newSource(t)

	cases := []struct {
		name   string
		mangle func(t *testing.T) string
	}{
		{
			name:   "absent",
			mangle: func(t *testing.T) string { return filepath.Join(canonical(t, t.TempDir()), "nowhere") },
		},
		{
			name: "not a directory",
			mangle: func(t *testing.T) string {
				p := filepath.Join(canonical(t, t.TempDir()), "file")
				writeFile(t, p, "not a checkout\n")
				return p
			},
		},
		{
			name: "missing an allowed root",
			mangle: func(t *testing.T) string {
				dir := filepath.Join(canonical(t, t.TempDir()), "copy")
				if err := os.CopyFS(dir, os.DirFS(good)); err != nil {
					t.Fatalf("copy: %v", err)
				}
				if err := os.RemoveAll(filepath.Join(dir, "cursor-rules")); err != nil {
					t.Fatalf("remove: %v", err)
				}
				return dir
			},
		},
		{
			name:   "empty",
			mangle: func(t *testing.T) string { return "" },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			mkdirAll(t, w.path(".toy"))
			homeBefore := snapshot(t, w.home)
			g := &goRun{body: "binary\n"}
			o := w.devOptions(t, tc.mangle(t), filepath.Join(w.home, "bin"), g)

			out := install.DevelopmentInstall(o)
			if !errors.Is(out.Err, install.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", out.Err)
			}
			if out.Reason != install.ReasonInvalidSourceRoot {
				t.Errorf("reason = %q", out.Reason)
			}
			if g.calls != 0 {
				t.Errorf("an invalid source was still built")
			}
			assertUnchanged(t, homeBefore, snapshot(t, w.home), "invalid source root")
		})
	}
}

func TestDevInstallIsIdempotent(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	bin := filepath.Join(w.home, "bin")
	g := &goRun{body: "binary\n"}
	if out := install.DevelopmentInstall(w.devOptions(t, src, bin, g)); out.Err != nil {
		t.Fatalf("first DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
	}
	before := snapshot(t, w.home)

	out := install.DevelopmentInstall(w.devOptions(t, src, bin, g))
	if out.Err != nil {
		t.Fatalf("second DevelopmentInstall: %v", out.Err)
	}
	if out.Applied || len(out.Actions) != 0 {
		t.Fatalf("second run did work: %v", actionPaths(out))
	}
	assertUnchanged(t, before, snapshot(t, w.home), "second development install")
}

func TestCheckDevelopmentSourceDrift(t *testing.T) {
	w := newWorld(t)
	mkdirAll(t, w.path(".toy"))
	src := newSource(t)
	g := &goRun{body: "binary\n"}
	dev := w.devOptions(t, src, filepath.Join(w.home, "bin"), g)
	if out := install.DevelopmentInstall(dev); out.Err != nil {
		t.Fatalf("DevelopmentInstall: %v (reason %q)", out.Err, out.Reason)
	}

	healthy := dev.Options
	healthy.FS = panicFS{}
	if out := install.Check(healthy); out.Reason != "" || out.Err != nil {
		t.Fatalf("healthy development check: reason %q err %v", out.Reason, out.Err)
	}

	// The checkout moved on: the recorded source digest no longer describes it.
	writeFile(t, filepath.Join(src, "skills", "docket-toy", "SKILL.md"), "# changed\n")
	regenerateSource(t, src)
	homeBefore := snapshot(t, w.home)
	out := install.Check(healthy)
	if out.Reason != install.ReasonSourceAssetsDrifted {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, install.ReasonSourceAssetsDrifted, out.Err)
	}
	assertUnchanged(t, homeBefore, snapshot(t, w.home), "development check")
}
