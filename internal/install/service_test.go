// The service tests live in install_test rather than install so they may
// import the real harness adapters: internal/harness imports internal/install,
// so only an external test package can close the loop. Everything they assert
// is therefore part of the package's exported surface, which is the point — the
// operations are what internal/app will hold.
package install_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
	"github.com/danielhanold/docket/internal/install"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// world is one isolated installation universe. The data root sits BESIDE the
// fake home rather than inside it, so a test that asserts "this operation
// touched nothing in the user's home" is not confused by the version tree the
// same operation legitimately extracts under the data root.
type world struct {
	t     *testing.T
	home  string
	data  string
	roots install.UserRoots
}

func newWorld(t *testing.T, harnessDirs ...string) *world {
	t.Helper()
	base := canonical(t, t.TempDir())
	home := filepath.Join(base, "home")
	data := filepath.Join(base, "data")
	mkdirAll(t, home)
	mkdirAll(t, data)
	for _, d := range harnessDirs {
		mkdirAll(t, filepath.Join(home, filepath.FromSlash(d)))
	}
	roots, err := install.ResolveRoots(
		func() (string, error) { return home, nil },
		func(k string) string {
			if k == "XDG_DATA_HOME" {
				return data
			}
			return ""
		},
	)
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	return &world{t: t, home: home, data: data, roots: roots}
}

// allHarnessDirs are the four detection roots, spelled as the adapters read
// them: claude/codex/cursor under the home, opencode under XDG config home.
var allHarnessDirs = []string{".claude", ".codex", ".cursor", ".config/opencode"}

func (w *world) path(parts ...string) string {
	return filepath.Join(append([]string{w.home}, parts...)...)
}

func canonical(t *testing.T, p string) string {
	t.Helper()
	// macOS resolves /tmp through a symlink, so every fixture path is
	// canonicalised once here: an installed link's destination is canonical, and
	// comparing it against an uncanonicalised fixture path would fail for a
	// reason that has nothing to do with the installer.
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("canonicalising %s: %v", p, err)
	}
	return resolved
}

func mkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(body)
}

func embeddedCatalog(t *testing.T) assets.Catalog {
	t.Helper()
	cat, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	return cat
}

// adapterPlanners adapts the four real harness adapters into the closure seam
// the installer consumes. It is the shape internal/app will build.
func adapterPlanners(roots install.UserRoots, agents config.AgentsTable) []install.Planner {
	adapters := []harness.Adapter{claude.New(), codex.New(), cursor.New(), opencode.New()}
	planners := make([]install.Planner, 0, len(adapters))
	for _, a := range adapters {
		a := a
		planners = append(planners, install.Planner{
			Name: a.Name(),
			Detect: func(r install.UserRoots) (bool, string) {
				d := a.Detect(r)
				return d.Present, d.Root
			},
			Plan: func(mode install.Mode, assetsDir string, cat assets.Catalog) ([]install.Target, error) {
				hm := harness.ModeRelease
				if mode == install.ModeDevelopment {
					hm = harness.ModeDevelopment
				}
				return a.Plan(harness.PlanInput{
					Assets:    cat,
					Mode:      hm,
					AssetsDir: assetsDir,
					Roots:     roots,
					Agents:    agents,
				})
			},
		})
	}
	return planners
}

func (w *world) options(agents config.AgentsTable) install.Options {
	return install.Options{
		Roots:       w.roots,
		Planners:    adapterPlanners(w.roots, agents),
		Catalog:     embeddedCatalog(w.t),
		Config:      &config.Snapshot{},
		Info:        buildinfo.Info{Version: "test", Commit: "cafe", BuildDate: "2026-08-13"},
		FS:          install.RealFS{},
		AgentDigest: "sha256:agents-baseline",
	}
}

// toy is a synthetic harness whose plan a test controls exactly. Real adapters
// prove integration; this proves the service's own arithmetic — prunes,
// upgrades, recovery — without a 100-target plan in the way.
type toy struct {
	name  string
	root  string
	files map[string]string // relative path -> content
	links []string          // relative path; destination is <assetsDir>/skills/<base>
}

func (x toy) planner() install.Planner {
	return install.Planner{
		Name: x.name,
		Detect: func(install.UserRoots) (bool, string) {
			info, err := os.Stat(x.root)
			return err == nil && info.IsDir(), x.root
		},
		Plan: func(mode install.Mode, assetsDir string, _ assets.Catalog) ([]install.Target, error) {
			out := make([]install.Target, 0, len(x.files)+len(x.links))
			for _, rel := range sortedKeys(x.files) {
				out = append(out, install.Target{
					Path:    filepath.Join(x.root, filepath.FromSlash(rel)),
					Kind:    install.KindFile,
					Content: []byte(x.files[rel]),
					Role:    "agent",
				})
			}
			for _, rel := range x.links {
				out = append(out, install.Target{
					Path:       filepath.Join(x.root, filepath.FromSlash(rel)),
					Kind:       install.KindSymlink,
					LinkTarget: filepath.Join(assetsDir, "skills", filepath.Base(rel)),
					Role:       "skill",
				})
			}
			return out, nil
		},
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (w *world) toyOptions(x toy) install.Options {
	o := w.options(nil)
	o.Planners = []install.Planner{x.planner()}
	return o
}

// ---------------------------------------------------------------------------
// Mutation snapshots
// ---------------------------------------------------------------------------

type node struct {
	kind string // "dir" | "file" | "link"
	body string // file bytes or link destination
	mode os.FileMode
}

// snapshot records a whole tree by value, so "this operation changed nothing"
// is an assertion about bytes rather than about which calls a fake observed.
func snapshot(t *testing.T, root string) map[string]node {
	t.Helper()
	out := map[string]node{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			dest, linkErr := os.Readlink(p)
			if linkErr != nil {
				return linkErr
			}
			out[rel] = node{kind: "link", body: dest}
		case d.IsDir():
			out[rel] = node{kind: "dir", mode: info.Mode().Perm()}
		default:
			body, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			out[rel] = node{kind: "file", body: string(body), mode: info.Mode().Perm()}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotting %s: %v", root, err)
	}
	return out
}

func assertUnchanged(t *testing.T, before, after map[string]node, context string) {
	t.Helper()
	for path, want := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("%s: %s disappeared", context, path)
			continue
		}
		if got != want {
			t.Errorf("%s: %s changed\n  before: %+v\n  after:  %+v", context, path, want, got)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("%s: %s appeared", context, path)
		}
	}
}

func actionPaths(out install.Outcome) []string {
	paths := make([]string, 0, len(out.Actions))
	for _, a := range out.Actions {
		paths = append(paths, a.Path)
	}
	return paths
}

func hasAction(out install.Outcome, op, path string) bool {
	_, ok := findAction(out, op, path)
	return ok
}

// findAction returns the reported action for one op and path, so a test can
// assert on its detail as well as its existence.
func findAction(out install.Outcome, op, path string) (install.Action, bool) {
	for _, a := range out.Actions {
		if a.Op == op && a.Path == path {
			return a, true
		}
	}
	return install.Action{}, false
}

func loadState(t *testing.T, roots install.UserRoots) *install.State {
	t.Helper()
	s, err := install.LoadState(roots.StatePath())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	return s
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func TestInstallFreshApplies(t *testing.T) {
	w := newWorld(t, allHarnessDirs...)
	o := w.options(nil)

	out := install.Install(o)
	if out.Err != nil {
		t.Fatalf("Install: %v (reason %q)", out.Err, out.Reason)
	}
	if !out.Applied {
		t.Fatalf("fresh install reported no work")
	}
	if got, want := strings.Join(out.Harnesses, ","), "claude,codex,cursor,opencode"; got != want {
		t.Fatalf("harnesses = %q, want %q", got, want)
	}
	if out.Mode != install.ModeRelease {
		t.Errorf("mode = %q, want release", out.Mode)
	}
	cat := embeddedCatalog(t)
	if out.AssetSetID != cat.Manifest.AssetSetID {
		t.Errorf("asset set id = %q, want %q", out.AssetSetID, cat.Manifest.AssetSetID)
	}

	// Every harness's three artefact shapes landed, in that harness's own
	// native spelling.
	versionAssets := w.roots.VersionDir(cat.Manifest.AssetSetID)
	link := w.path(".claude", "skills", "docket-build")
	dest, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink %s: %v", link, err)
	}
	if want := filepath.Join(versionAssets, "skills", "docket-build"); dest != want {
		t.Errorf("claude skill link -> %s, want %s", dest, want)
	}
	if body := readFile(t, w.path(".claude", "agents", "docket-build-standard.md")); !strings.Contains(body, "docket-build-standard") {
		t.Errorf("claude agent wrapper does not name its agent:\n%s", body)
	}
	if body := readFile(t, w.path(".claude", "CLAUDE.md")); !strings.Contains(body, "docket:dispatch:start") {
		t.Errorf("claude dispatch block missing:\n%s", body)
	}
	// Codex reads skills from $HOME/.agents, not from its own root.
	if _, err := os.Lstat(w.path(".agents", "skills", "docket-build")); err != nil {
		t.Errorf("codex skill link: %v", err)
	}
	if _, err := os.Lstat(w.path(".codex", "agents", "docket-build-standard.toml")); err != nil {
		t.Errorf("codex agent wrapper: %v", err)
	}
	if _, err := os.Lstat(w.path(".cursor", "rules", "docket-dispatch.mdc")); err != nil {
		t.Errorf("cursor dispatch file: %v", err)
	}
	if body := readFile(t, w.path(".config", "opencode", "AGENTS.md")); !strings.Contains(body, "docket:dispatch:start") {
		t.Errorf("opencode dispatch block missing")
	}

	state := loadState(t, w.roots)
	if state == nil {
		t.Fatalf("no state published")
	}
	if state.Mode != install.ModeRelease || state.AssetSetID != cat.Manifest.AssetSetID {
		t.Errorf("state = %+v", state)
	}
	if state.AgentDigest != o.AgentDigest || state.ProductVersion != "test" {
		t.Errorf("state identity = %q/%q", state.AgentDigest, state.ProductVersion)
	}
	if len(state.Targets) != len(out.Actions) {
		t.Errorf("state records %d targets, outcome reported %d actions", len(state.Targets), len(out.Actions))
	}
	for _, rec := range state.Targets {
		if rec.Harness == "" {
			t.Fatalf("record %s carries no harness attribution", rec.Path)
		}
	}

	// Second run: the same inputs must be a no-op, or the installer is not
	// idempotent and every run churns the user's files.
	before := snapshot(t, w.home)
	again := install.Install(w.options(nil))
	if again.Err != nil {
		t.Fatalf("second Install: %v", again.Err)
	}
	if again.Applied || len(again.Actions) != 0 || again.Reason != "" {
		t.Fatalf("second Install did work: applied=%v reason=%q actions=%v", again.Applied, again.Reason, actionPaths(again))
	}
	assertUnchanged(t, before, snapshot(t, w.home), "second install")
}

func TestInstallNoHarnessDetected(t *testing.T) {
	w := newWorld(t)
	before := snapshot(t, w.home)

	out := install.Install(w.options(nil))
	if out.Reason != install.ReasonNoHarnessDetected {
		t.Fatalf("reason = %q, want %q", out.Reason, install.ReasonNoHarnessDetected)
	}
	if out.Err == nil || out.Applied {
		t.Fatalf("outcome = %+v", out)
	}
	assertUnchanged(t, before, snapshot(t, w.home), "no harness detected")
	if _, err := os.Stat(w.roots.StatePath()); !os.IsNotExist(err) {
		t.Errorf("state published anyway: %v", err)
	}
}

func TestInstallExplicitHarnessCreatesRoot(t *testing.T) {
	w := newWorld(t) // nothing detected
	o := w.options(nil)
	o.Harnesses = []string{"claude"}

	out := install.Install(o)
	if out.Err != nil {
		t.Fatalf("Install: %v (reason %q)", out.Err, out.Reason)
	}
	if got := strings.Join(out.Harnesses, ","); got != "claude" {
		t.Fatalf("harnesses = %q", got)
	}
	if _, err := os.Lstat(w.path(".claude", "agents", "docket-status.md")); err != nil {
		t.Errorf("claude root not created: %v", err)
	}
	if _, err := os.Lstat(w.path(".codex")); !os.IsNotExist(err) {
		t.Errorf("unselected harness root created: %v", err)
	}
}

func TestInstallUnknownHarnessIsInvalidInput(t *testing.T) {
	w := newWorld(t, allHarnessDirs...)
	before := snapshot(t, w.home)
	o := w.options(nil)
	o.Harnesses = []string{"claude", "emacs"}

	out := install.Install(o)
	if !errors.Is(out.Err, install.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", out.Err)
	}
	if out.Reason != install.ReasonUnknownHarness {
		t.Errorf("reason = %q", out.Reason)
	}
	assertUnchanged(t, before, snapshot(t, w.home), "unknown harness")
}

func TestInstallConflictPreservesEverything(t *testing.T) {
	w := newWorld(t, allHarnessDirs...)
	// Somebody else's file where an agent wrapper belongs. Nothing about the
	// name proves ownership, so the whole installation must refuse.
	foreign := w.path(".claude", "agents", "docket-status.md")
	writeFile(t, foreign, "hand-written by the user\n")
	before := snapshot(t, w.home)

	out := install.Install(w.options(nil))
	if out.Reason != install.ReasonOwnershipConflict {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, install.ReasonOwnershipConflict, out.Err)
	}
	if out.Applied {
		t.Fatalf("conflicted install reported applied work")
	}
	action, ok := findAction(out, install.OpConflict, foreign)
	if !ok {
		t.Fatalf("conflict not reported for %s: %v", foreign, out.Actions)
	}
	// There is no --force: the report is the user's only way forward, so it
	// carries the stable reason AND what to do about this particular target.
	for _, want := range []string{install.ReasonOwnershipConflict, "docket did not write", "move or delete", "re-run"} {
		if !strings.Contains(action.Detail, want) {
			t.Errorf("conflict detail = %q, want it to contain %q", action.Detail, want)
		}
	}
	assertUnchanged(t, before, snapshot(t, w.home), "ownership conflict")
	if _, err := os.Stat(w.roots.StatePath()); !os.IsNotExist(err) {
		t.Errorf("state published despite a conflict: %v", err)
	}
}

func TestInstallManagedBlockInvalidRefuses(t *testing.T) {
	w := newWorld(t, ".claude")
	// A start marker with no end: rewriting an unbounded range would eat the
	// rest of the user's file.
	writeFile(t, w.path(".claude", "CLAUDE.md"), "# mine\n\n<!-- docket:dispatch:start -->\nstuff\n")
	before := snapshot(t, w.home)

	o := w.options(nil)
	o.Harnesses = []string{"claude"}
	out := install.Install(o)
	if out.Reason != install.ReasonManagedBlockInvalid {
		t.Fatalf("reason = %q, want %q", out.Reason, install.ReasonManagedBlockInvalid)
	}
	blocked, ok := findAction(out, install.OpConflict, w.path(".claude", "CLAUDE.md"))
	if !ok {
		t.Fatalf("conflict not reported for the managed-block file: %v", out.Actions)
	}
	// Only the user can repair their own markers, so the remedy names them.
	for _, want := range []string{install.ReasonManagedBlockInvalid, "docket:dispatch", "by hand", "re-run"} {
		if !strings.Contains(blocked.Detail, want) {
			t.Errorf("conflict detail = %q, want it to contain %q", blocked.Detail, want)
		}
	}
	assertUnchanged(t, before, snapshot(t, w.home), "managed block invalid")
}

func TestInstallGlobalPinChangeTouchesOnlyAgents(t *testing.T) {
	w := newWorld(t, ".claude")
	o := w.options(nil)
	o.Harnesses = []string{"claude"}
	if out := install.Install(o); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}

	pinned := config.AgentsTable{"claude": {"build-standard": {
		Model:  config.Value[string]{Value: "opus-5", Explicit: true},
		Effort: config.Value[string]{Value: "high", Explicit: true},
	}}}
	o2 := w.options(pinned)
	o2.Harnesses = []string{"claude"}
	out := install.Install(o2)
	if out.Err != nil {
		t.Fatalf("second Install: %v (reason %q)", out.Err, out.Reason)
	}
	if !out.Applied {
		t.Fatalf("a changed pin produced no work")
	}
	for _, a := range out.Actions {
		if !strings.Contains(a.Path, string(filepath.Separator)+"agents"+string(filepath.Separator)) {
			t.Errorf("a pin change touched %s (%s)", a.Path, a.Op)
		}
	}
	if body := readFile(t, w.path(".claude", "agents", "docket-build-standard.md")); !strings.Contains(body, "opus-5") {
		t.Errorf("pin not rendered:\n%s", body)
	}
}

func TestInstallUpgradePrunesOwned(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	v1 := toy{name: "toy", root: root, files: map[string]string{"a.md": "a1\n", "b.md": "b1\n"}}
	if out := install.Install(w.toyOptions(v1)); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}

	v2 := toy{name: "toy", root: root, files: map[string]string{"a.md": "a1\n"}}
	out := install.Install(w.toyOptions(v2))
	if out.Err != nil {
		t.Fatalf("upgrade: %v (reason %q)", out.Err, out.Reason)
	}
	if !hasAction(out, install.OpRemove, filepath.Join(root, "b.md")) {
		t.Errorf("stale target not pruned: %v", out.Actions)
	}
	if _, err := os.Lstat(filepath.Join(root, "b.md")); !os.IsNotExist(err) {
		t.Errorf("pruned file still present: %v", err)
	}
	if readFile(t, filepath.Join(root, "a.md")) != "a1\n" {
		t.Errorf("surviving target was disturbed")
	}
	state := loadState(t, w.roots)
	for _, rec := range state.Targets {
		if strings.HasSuffix(rec.Path, "b.md") {
			t.Errorf("state still records the pruned target")
		}
	}
}

func TestInstallUpgradeDriftBlocks(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	v1 := toy{name: "toy", root: root, files: map[string]string{"a.md": "a1\n", "b.md": "b1\n"}}
	if out := install.Install(w.toyOptions(v1)); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}
	// The user has since edited the target the upgrade would drop. Its bytes
	// are no longer ours to delete.
	writeFile(t, filepath.Join(root, "b.md"), "the user's own notes\n")
	before := snapshot(t, w.home)
	stateBefore := readFile(t, w.roots.StatePath())

	v2 := toy{name: "toy", root: root, files: map[string]string{"a.md": "a2\n"}}
	out := install.Install(w.toyOptions(v2))
	if out.Reason != install.ReasonInstallationDrift {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, install.ReasonInstallationDrift, out.Err)
	}
	if out.Applied {
		t.Fatalf("blocked upgrade reported applied work")
	}
	assertUnchanged(t, before, snapshot(t, w.home), "drifted prune")
	if readFile(t, w.roots.StatePath()) != stateBefore {
		t.Errorf("state republished despite a blocked upgrade")
	}
}

func TestStaleManagedBlockRecordNeverDeletesTheFile(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	notes := filepath.Join(root, "NOTES.md")

	blocky := install.Planner{
		Name:   "toy",
		Detect: func(install.UserRoots) (bool, string) { return true, root },
		Plan: func(install.Mode, string, assets.Catalog) ([]install.Target, error) {
			return []install.Target{{
				Path:       notes,
				Kind:       install.KindManagedBlock,
				BlockName:  "dispatch",
				Annotation: "managed by docket",
				Content:    []byte("hello"),
				Role:       "dispatch",
			}}, nil
		},
	}
	o := w.options(nil)
	o.Planners = []install.Planner{blocky}
	if out := install.Install(o); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}
	body := readFile(t, notes)

	// The block leaves the plan. Deleting the file would take the user's own
	// content with it, so the record is retained and the file is left alone.
	empty := toy{name: "toy", root: root, files: map[string]string{"a.md": "a\n"}}
	o2 := w.options(nil)
	o2.Planners = []install.Planner{empty.planner()}
	out := install.Install(o2)
	if out.Err != nil {
		t.Fatalf("second Install: %v (reason %q)", out.Err, out.Reason)
	}
	if readFile(t, notes) != body {
		t.Errorf("the managed-block file was rewritten or truncated")
	}
	if _, err := os.Stat(notes); err != nil {
		t.Fatalf("the managed-block file was deleted: %v", err)
	}
}

func TestInstallRecoversPendingJournal(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	x := toy{name: "toy", root: root, files: map[string]string{"a.md": "installed\n"}}
	if out := install.Install(w.toyOptions(x)); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}
	target := filepath.Join(root, "a.md")

	// An interrupted run: the journal is on disk with the pre-image captured,
	// the destination has already been half-rewritten, and nobody rolled it
	// back.
	pending := install.Target{Path: target, Kind: install.KindFile, Content: []byte("half-applied\n"), Role: "agent"}
	insp, err := install.InspectTarget(pending, loadState(t, w.roots), nil)
	if err != nil {
		t.Fatalf("InspectTarget: %v", err)
	}
	txn, err := install.BeginTxn(install.RealFS{}, w.roots, []install.Inspection{insp})
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	writeFile(t, target, "half-applied\n")

	out := install.Install(w.toyOptions(x))
	if out.Err != nil {
		t.Fatalf("Install over a pending journal: %v (reason %q)", out.Err, out.Reason)
	}
	if !hasAction(out, install.OpRecover, filepath.Join(w.roots.TransactionsDir(), txn.ID())) {
		t.Errorf("recovery not reported: %v", out.Actions)
	}
	if got := readFile(t, target); got != "installed\n" {
		t.Errorf("recovery did not restore the pre-image: %q", got)
	}
	if _, found, err := install.DetectRecovery(w.roots); err != nil || found {
		t.Errorf("journal survived recovery: found=%v err=%v", found, err)
	}
}

func TestInstallRefusesCorruptState(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	x := toy{name: "toy", root: root, files: map[string]string{"a.md": "a\n"}}
	if out := install.Install(w.toyOptions(x)); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}
	writeFile(t, w.roots.StatePath(), "{not json")
	before := snapshot(t, w.home)

	out := install.Install(w.toyOptions(x))
	if !errors.Is(out.Err, install.ErrStateInvalid) {
		t.Fatalf("err = %v, want ErrStateInvalid", out.Err)
	}
	// Corruption is never "not installed": overwriting here would silently
	// adopt targets nothing can prove docket owns.
	assertUnchanged(t, before, snapshot(t, w.home), "corrupt state")
}

func TestInstallBlockedByDeferredCapability(t *testing.T) {
	w := newWorld(t, allHarnessDirs...)
	before := snapshot(t, w.home)
	o := w.options(nil)
	o.Config = &config.Snapshot{Diagnostics: []config.Diagnostic{{
		Code:    config.CodeDeferredCapRequested,
		Path:    "agents.claude",
		Message: "deferred capability requested",
	}}}

	out := install.Install(o)
	if out.Reason != install.ReasonDeferredCapability {
		t.Fatalf("reason = %q, want %q", out.Reason, install.ReasonDeferredCapability)
	}
	if !errors.Is(out.Err, config.ErrUnsupportedConfig) {
		t.Errorf("err = %v, want ErrUnsupportedConfig", out.Err)
	}
	assertUnchanged(t, before, snapshot(t, w.home), "blocked configuration")
}

func TestSharedAncestorOwnership(t *testing.T) {
	w := newWorld(t, ".claude", ".codex")
	if out := install.Install(w.options(nil)); out.Err != nil {
		t.Fatalf("Install: %v", out.Err)
	}
	state := loadState(t, w.roots)

	claudeRoot := w.path(".claude")
	codexRoots := []string{w.path(".codex"), w.path(".agents")}
	for _, rec := range state.Targets {
		switch rec.Harness {
		case "claude":
			for _, r := range codexRoots {
				if strings.HasPrefix(rec.Path, r+string(filepath.Separator)) {
					t.Errorf("claude claims %s, which sits under codex's %s", rec.Path, r)
				}
			}
		case "codex":
			if strings.HasPrefix(rec.Path, claudeRoot+string(filepath.Separator)) {
				t.Errorf("codex claims %s, which sits under claude's root", rec.Path)
			}
		default:
			t.Errorf("unexpected harness %q in state", rec.Harness)
		}
	}

	// Re-running for one harness only must not sweep the other's targets: a
	// selection is a scope, never an uninstall of everything outside it.
	codexBefore := map[string]node{}
	for _, r := range codexRoots {
		for k, v := range snapshot(t, r) {
			codexBefore[r+"::"+k] = v
		}
	}
	pinned := config.AgentsTable{"claude": {"status": {Model: config.Value[string]{Value: "opus-5", Explicit: true}}}}
	o := w.options(pinned)
	o.Harnesses = []string{"claude"}
	out := install.Install(o)
	if out.Err != nil {
		t.Fatalf("scoped Install: %v (reason %q)", out.Err, out.Reason)
	}
	for _, a := range out.Actions {
		for _, r := range codexRoots {
			if strings.HasPrefix(a.Path, r+string(filepath.Separator)) {
				t.Errorf("a claude-scoped install acted on %s (%s)", a.Path, a.Op)
			}
		}
	}
	codexAfter := map[string]node{}
	for _, r := range codexRoots {
		for k, v := range snapshot(t, r) {
			codexAfter[r+"::"+k] = v
		}
	}
	assertUnchanged(t, codexBefore, codexAfter, "codex material under a claude-scoped install")

	after := loadState(t, w.roots)
	if !contains(after.Harnesses, "codex") {
		t.Errorf("state forgot codex: %v", after.Harnesses)
	}
	if countHarness(after, "codex") != countHarness(state, "codex") {
		t.Errorf("codex records changed: %d -> %d", countHarness(state, "codex"), countHarness(after, "codex"))
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func countHarness(s *install.State, name string) int {
	n := 0
	for _, rec := range s.Targets {
		if rec.Harness == name {
			n++
		}
	}
	return n
}

func TestRepoLayerNeverLoaded(t *testing.T) {
	w := newWorld(t, ".claude")
	o := w.options(nil)
	o.Harnesses = []string{"claude"}
	if out := install.Install(o); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}
	before := snapshot(t, w.home)
	stateBefore := readFile(t, w.roots.StatePath())

	// A repository configuration sitting in the working directory is not an
	// input to this operation: the service takes a resolved snapshot, never a
	// path, so there is nothing for a repo layer to reach.
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".docket.yml"), "agents:\n  claude:\n    build-standard:\n      model: sonnet-9\n")
	t.Chdir(repo)

	out := install.Install(w.options(nil))
	if out.Err != nil {
		t.Fatalf("second Install: %v (reason %q)", out.Err, out.Reason)
	}
	if out.Applied || len(out.Actions) != 0 {
		t.Fatalf("a repo-local .docket.yml changed the installation: %v", actionPaths(out))
	}
	assertUnchanged(t, before, snapshot(t, w.home), "repo-local config")
	if readFile(t, w.roots.StatePath()) != stateBefore {
		t.Errorf("state changed under a repo-local config")
	}
}

// ---------------------------------------------------------------------------
// The exclusive installation lock
// ---------------------------------------------------------------------------

// holdInstallLock takes the installation lock the way a second docket process
// would: an independent descriptor on the same file, flocked exclusively. An
// flock belongs to the open file description rather than to the process, so
// this contends with the installer's own acquisition even from inside the same
// test binary — which is what makes a concurrency defect testable without
// forking one.
func holdInstallLock(t *testing.T, roots install.UserRoots) func() {
	t.Helper()
	mkdirAll(t, roots.DataRoot)
	f, err := os.OpenFile(roots.LockPath(), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("opening %s: %v", roots.LockPath(), err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("flocking %s: %v", roots.LockPath(), err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	t.Cleanup(release)
	return release
}

// A journal on disk says an installation was interrupted; it does not say by
// whom. Without the lock a second run reads a LIVE run's journal as wreckage
// and rolls it back mid-apply, leaving a world neither journal describes. The
// lock is what tells the two apart.
func TestInstallRefusesWhileAnotherRunHoldsTheLock(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	x := toy{name: "toy", root: root, files: map[string]string{"a.md": "installed\n"}}
	if out := install.Install(w.toyOptions(x)); out.Err != nil {
		t.Fatalf("first Install: %v", out.Err)
	}
	target := filepath.Join(root, "a.md")

	// Another run, mid-apply: its journal is published, its pre-image captured,
	// and its destination already rewritten. On disk this is indistinguishable
	// from an interrupted run — the holder of the lock is the difference.
	pending := install.Target{Path: target, Kind: install.KindFile, Content: []byte("half-applied\n"), Role: "agent"}
	insp, err := install.InspectTarget(pending, loadState(t, w.roots), nil)
	if err != nil {
		t.Fatalf("InspectTarget: %v", err)
	}
	txn, err := install.BeginTxn(install.RealFS{}, w.roots, []install.Inspection{insp})
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	writeFile(t, target, "half-applied\n")
	release := holdInstallLock(t, w.roots)
	before := snapshot(t, w.home)

	out := install.Install(w.toyOptions(x))
	if out.Reason != install.ReasonInstallInProgress {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, install.ReasonInstallInProgress, out.Err)
	}
	if !errors.Is(out.Err, install.ErrInstallLocked) {
		t.Errorf("err = %v, want ErrInstallLocked", out.Err)
	}
	if out.Applied {
		t.Errorf("a refused install reported applied work")
	}
	assertUnchanged(t, before, snapshot(t, w.home), "install under a held lock")
	if id, found, err := install.DetectRecovery(w.roots); err != nil || !found || id != txn.ID() {
		t.Fatalf("the live run's journal was rolled back: found=%v id=%q err=%v", found, id, err)
	}

	// Once the holder is gone the very same journal IS orphaned — an flock dies
	// with its process — and the next run recovers it.
	release()
	out = install.Install(w.toyOptions(x))
	if out.Err != nil {
		t.Fatalf("Install after the lock was released: %v (reason %q)", out.Err, out.Reason)
	}
	if !hasAction(out, install.OpRecover, filepath.Join(w.roots.TransactionsDir(), txn.ID())) {
		t.Errorf("recovery not reported: %v", out.Actions)
	}
	if got := readFile(t, target); got != "installed\n" {
		t.Errorf("recovery did not restore the pre-image: %q", got)
	}
}

// A run killed while holding the lock leaves the lock FILE behind and takes the
// flock with it. Were the file's existence the lock, that crash would refuse
// every installation from then on.
func TestInstallProceedsOverAStaleLockFile(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	x := toy{name: "toy", root: root, files: map[string]string{"a.md": "installed\n"}}
	writeFile(t, w.roots.LockPath(), "left behind by a killed run\n")

	out := install.Install(w.toyOptions(x))
	if out.Err != nil {
		t.Fatalf("Install over a stale lock file: %v (reason %q)", out.Err, out.Reason)
	}
	if !out.Applied {
		t.Errorf("the install did no work")
	}
	if readFile(t, filepath.Join(root, "a.md")) != "installed\n" {
		t.Errorf("the target was not written")
	}
}

// The lock spans one run, not one machine: two installs back to back must both
// proceed, including the second one whose only outcome is a no-op.
func TestSequentialInstallsReleaseTheLock(t *testing.T) {
	w := newWorld(t)
	root := w.path(".toy")
	mkdirAll(t, root)
	x := toy{name: "toy", root: root, files: map[string]string{"a.md": "installed\n"}}

	for i, want := range []bool{true, false} {
		out := install.Install(w.toyOptions(x))
		if out.Err != nil {
			t.Fatalf("Install %d: %v (reason %q)", i+1, out.Err, out.Reason)
		}
		if out.Applied != want {
			t.Errorf("Install %d applied = %v, want %v", i+1, out.Applied, want)
		}
		// A lock the previous run leaked would be indistinguishable from a run
		// still in flight, so the release is asserted from outside.
		holdInstallLock(t, w.roots)()
	}
}

// ---------------------------------------------------------------------------
// Check
// ---------------------------------------------------------------------------

// panicFS refuses every mutation. Check must be read-only, and the guard is
// double: this seam catches a write routed through FSOps, and the snapshot
// compare below catches one that bypasses it.
type panicFS struct{}

func (panicFS) WriteFile(path string, _ []byte, _ os.FileMode) error {
	panic("install check wrote " + path)
}
func (panicFS) Chmod(path string, _ os.FileMode) error {
	panic("install check chmodded " + path)
}
func (panicFS) Rename(old, _ string) error   { panic("install check renamed " + old) }
func (panicFS) Symlink(_, path string) error { panic("install check linked " + path) }
func (panicFS) Remove(path string) error     { panic("install check removed " + path) }
func (panicFS) MkdirAll(path string, _ os.FileMode) error {
	panic("install check created " + path)
}

func TestCheckReadOnly(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T, w *world) // runs after a healthy install unless installed is false
		install bool
		reason  string
	}{
		{
			name:    "healthy installation",
			install: true,
			reason:  "",
		},
		{
			name:    "nothing installed",
			install: false,
			reason:  install.ReasonInstallationRequired,
		},
		{
			name:    "drifted target",
			install: true,
			arrange: func(t *testing.T, w *world) {
				p := w.path(".claude", "agents", "docket-status.md")
				writeFile(t, p, readFile(t, p)+"\nedited by hand\n")
			},
			reason: install.ReasonInstallationDrift,
		},
		{
			name:    "pending journal",
			install: true,
			arrange: func(t *testing.T, w *world) {
				target := install.Target{
					Path:    w.path(".claude", "agents", "docket-status.md"),
					Kind:    install.KindFile,
					Content: []byte("pending\n"),
					Role:    "agent",
				}
				insp, err := install.InspectTarget(target, nil, nil)
				if err != nil {
					t.Fatalf("InspectTarget: %v", err)
				}
				insp.Disposition = install.DispositionUpdate
				if _, err := install.BeginTxn(install.RealFS{}, w.roots, []install.Inspection{insp}); err != nil {
					t.Fatalf("BeginTxn: %v", err)
				}
			},
			reason: install.ReasonTransactionRecoveryRequired,
		},
		{
			name:    "asset protocol mismatch",
			install: true,
			arrange: func(t *testing.T, w *world) {
				body := readFile(t, w.roots.StatePath())
				writeFile(t, w.roots.StatePath(),
					strings.Replace(body, `"asset_protocol": 1`, `"asset_protocol": 99`, 1))
			},
			reason: install.ReasonAssetProtocolMismatch,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, ".claude")
			if tc.install {
				o := w.options(nil)
				o.Harnesses = []string{"claude"}
				if out := install.Install(o); out.Err != nil {
					t.Fatalf("arranging the installation: %v (reason %q)", out.Err, out.Reason)
				}
			}
			if tc.arrange != nil {
				tc.arrange(t, w)
			}

			homeBefore := snapshot(t, w.home)
			dataBefore := snapshot(t, w.data)

			o := w.options(nil)
			o.Harnesses = []string{"claude"}
			o.FS = panicFS{}
			out := install.Check(o)

			if out.Reason != tc.reason {
				t.Errorf("reason = %q, want %q (err %v)", out.Reason, tc.reason, out.Err)
			}
			if out.Applied {
				t.Errorf("check reported applied work")
			}
			if tc.reason == "" && out.Err != nil {
				t.Errorf("healthy check returned %v", out.Err)
			}
			if tc.reason != "" && out.Err == nil {
				t.Errorf("failing check returned no error")
			}
			assertUnchanged(t, homeBefore, snapshot(t, w.home), "check over the home")
			assertUnchanged(t, dataBefore, snapshot(t, w.data), "check over the data root")
		})
	}
}

func TestCheckNamesTheDriftedTarget(t *testing.T) {
	w := newWorld(t, ".claude")
	o := w.options(nil)
	o.Harnesses = []string{"claude"}
	if out := install.Install(o); out.Err != nil {
		t.Fatalf("Install: %v", out.Err)
	}
	drifted := w.path(".claude", "agents", "docket-status.md")
	writeFile(t, drifted, "replaced\n")

	o.FS = panicFS{}
	out := install.Check(o)
	if out.Reason != install.ReasonInstallationDrift {
		t.Fatalf("reason = %q", out.Reason)
	}
	action, ok := findAction(out, install.OpDrift, drifted)
	if !ok {
		t.Fatalf("drift action missing for %s: %v", drifted, out.Actions)
	}
	// A conflict is a dead end whichever operation found it, so `check` states
	// the same way forward `install` would have.
	for _, want := range []string{install.ReasonOwnershipConflict, "no longer matches the recorded install", "re-run"} {
		if !strings.Contains(action.Detail, want) {
			t.Errorf("drift detail = %q, want it to contain %q", action.Detail, want)
		}
	}
}

func TestCheckDetectsMissingVersionTree(t *testing.T) {
	w := newWorld(t, ".claude")
	o := w.options(nil)
	o.Harnesses = []string{"claude"}
	if out := install.Install(o); out.Err != nil {
		t.Fatalf("Install: %v", out.Err)
	}
	tree := filepath.Dir(w.roots.VersionDir(embeddedCatalog(t).Manifest.AssetSetID))
	if err := os.RemoveAll(tree); err != nil {
		t.Fatalf("removing the version tree: %v", err)
	}

	o.FS = panicFS{}
	out := install.Check(o)
	if out.Reason != install.ReasonInstallationDrift {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, install.ReasonInstallationDrift, out.Err)
	}
}

// Check is read-only, and the installation lock is a write lock. Taking it
// would make `install check` mutate the data root — and would make a report
// impossible for exactly the person most likely to want one: someone watching
// an install that is still running.
func TestCheckNeverTakesTheLock(t *testing.T) {
	w := newWorld(t, ".claude")
	o := w.options(nil)
	o.Harnesses = []string{"claude"}
	if out := install.Install(o); out.Err != nil {
		t.Fatalf("Install: %v (reason %q)", out.Err, out.Reason)
	}
	// The install left the lock file behind; removing it makes any reappearance
	// unambiguously check's doing.
	if err := os.Remove(w.roots.LockPath()); err != nil {
		t.Fatalf("removing the lock file: %v", err)
	}

	o.FS = panicFS{}
	if out := install.Check(o); out.Reason != "" || out.Err != nil {
		t.Fatalf("healthy check: reason %q err %v", out.Reason, out.Err)
	}
	if _, err := os.Lstat(w.roots.LockPath()); !os.IsNotExist(err) {
		t.Errorf("check created the installation lock: %v", err)
	}

	// And a check taken while another run holds the lock still answers, rather
	// than refusing or waiting.
	holdInstallLock(t, w.roots)
	if out := install.Check(o); out.Reason != "" || out.Err != nil {
		t.Errorf("check under a held lock: reason %q err %v", out.Reason, out.Err)
	}
}
