package opencode

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
)

// -update rewrites the frozen goldens from the current renderer, exactly as in
// the claude, codex, and cursor adapters: a deliberate rendering change becomes
// a reviewable diff rather than a hand edit, and it is never set in CI.
var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current renderer")

const (
	fakeHome   = "/home/u"
	fakeConfig = "/home/u/.config"
	assetsDir  = "/data/versions/sha256-x/assets"
)

// The fixed input table the goldens are frozen under: one agent pinned on both
// fields, one model-only, one carrying both no-pin sentinels. Every other agent
// resolves to no pin at all, which is the fourth case.
func fixtureAgents() config.AgentsTable {
	return config.AgentsTable{
		"opencode": {
			"build-standard": {
				Model:  config.Value[string]{Value: "openrouter/anthropic/claude-opus-5"},
				Effort: config.Value[string]{Value: "high"},
			},
			"status": {
				Model: config.Value[string]{Value: "openrouter/deepseek/deepseek-v4-flash-0731"},
			},
			"adr": {
				Model:  config.Value[string]{Value: "inherit"},
				Effort: config.Value[string]{Value: "auto"},
			},
		},
		// A sibling harness's pins must never leak into the opencode rendering.
		"claude": {
			"build-standard": {
				Model:  config.Value[string]{Value: "claude-opus-5[1m]"},
				Effort: config.Value[string]{Value: "low"},
			},
		},
	}
}

func fixtureRoots() install.UserRoots {
	return install.UserRoots{
		Home:       fakeHome,
		DataRoot:   fakeHome + "/.local/share/docket",
		ConfigHome: fakeConfig,
		BinDir:     fakeHome + "/.local/bin",
	}
}

func fixtureInput(t *testing.T) harness.PlanInput {
	t.Helper()
	c, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	return harness.PlanInput{
		Assets:    c,
		Mode:      harness.ModeRelease,
		AssetsDir: assetsDir,
		Roots:     fixtureRoots(),
		Agents:    fixtureAgents(),
	}
}

func planFixture(t *testing.T) []install.Target {
	t.Helper()
	targets, err := New().Plan(fixtureInput(t))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return targets
}

func TestOpencodeName(t *testing.T) {
	if got := New().Name(); got != "opencode" {
		t.Fatalf("Name = %q, want opencode", got)
	}
}

func TestOpencodePlanDeterministic(t *testing.T) {
	first := planFixture(t)
	second := planFixture(t)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Plan is not deterministic")
	}
	for i, tg := range first {
		if i > 0 && !(first[i-1].Path < tg.Path) {
			t.Fatalf("plan not strictly sorted by Path at %d: %q then %q", i, first[i-1].Path, tg.Path)
		}
		if !filepath.IsAbs(tg.Path) {
			t.Errorf("target %d path %q is not absolute", i, tg.Path)
		}
	}
}

// Every opencode path roots on UserRoots.ConfigHome, never on ~/.config
// recomputed from Home: an XDG_CONFIG_HOME elsewhere must move the whole
// installation, and a target that quietly rooted on Home would install a tree
// opencode never reads.
func TestOpencodeXDGRoot(t *testing.T) {
	in := fixtureInput(t)
	in.Roots.ConfigHome = "/xdg/cfg"
	targets, err := New().Plan(in)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(targets) == 0 {
		t.Fatalf("Plan produced no targets")
	}
	want := filepath.Join("/xdg/cfg", "opencode")
	for _, tg := range targets {
		if !strings.HasPrefix(tg.Path, want+string(filepath.Separator)) {
			t.Errorf("target %q does not sit under the XDG root %q", tg.Path, want)
		}
		if strings.Contains(tg.Path, fakeHome) {
			t.Errorf("target %q leaks the home directory past the XDG override", tg.Path)
		}
	}
	// The plan under the override differs from the default plan only by root:
	// same count, same relative layout.
	def := planFixture(t)
	if len(def) != len(targets) {
		t.Fatalf("XDG override changed the target count: %d vs %d", len(targets), len(def))
	}
	for i := range def {
		gotRel := strings.TrimPrefix(targets[i].Path, want)
		wantRel := strings.TrimPrefix(def[i].Path, filepath.Join(fakeConfig, "opencode"))
		if gotRel != wantRel {
			t.Errorf("target %d relative path = %q, want %q", i, gotRel, wantRel)
		}
	}
	// Detect follows ConfigHome too.
	if root := New().Detect(install.UserRoots{Home: fakeHome, ConfigHome: "/xdg/cfg"}).Root; root != want {
		t.Errorf("Detect root = %q, want %q", root, want)
	}
}

func TestOpencodeSkillLinks(t *testing.T) {
	c, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	dirs := harness.SkillDirs(c)
	if len(dirs) == 0 {
		t.Fatalf("the bundle carries no skill dirs; the fixture assumption is stale")
	}
	want := map[string]string{}
	for _, dir := range dirs {
		want[filepath.Join(fakeConfig, "opencode", "skills", dir)] = filepath.Join(assetsDir, "skills", dir)
	}
	got := map[string]string{}
	for _, tg := range planFixture(t) {
		if tg.Kind != install.KindSymlink {
			continue
		}
		got[tg.Path] = tg.LinkTarget
		if len(tg.Content) != 0 {
			t.Errorf("symlink %q carries content", tg.Path)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("skill links = %v, want %v", got, want)
	}
}

func TestOpencodeDetect(t *testing.T) {
	cfg := t.TempDir()
	roots := install.UserRoots{Home: t.TempDir(), ConfigHome: cfg}

	d := New().Detect(roots)
	if d.Present {
		t.Errorf("Detect reported present with no <ConfigHome>/opencode")
	}
	if d.Root != filepath.Join(cfg, "opencode") {
		t.Errorf("Detect root = %q", d.Root)
	}

	// A regular file at the root is not a harness installation.
	if err := os.WriteFile(filepath.Join(cfg, "opencode"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if New().Detect(roots).Present {
		t.Errorf("Detect reported present for a regular file at <ConfigHome>/opencode")
	}
	if err := os.Remove(filepath.Join(cfg, "opencode")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if err := os.Mkdir(filepath.Join(cfg, "opencode"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if !New().Detect(roots).Present {
		t.Errorf("Detect did not report present with <ConfigHome>/opencode a directory")
	}

	// A home with no resolved config home resolves to no detection rather than
	// a bare-relative stat.
	if New().Detect(install.UserRoots{Home: roots.Home}).Present {
		t.Errorf("Detect reported present with no ConfigHome")
	}
	if New().Detect(install.UserRoots{}).Present {
		t.Errorf("Detect reported present for empty roots")
	}
}

// goldenPath is the frozen artifact for one rendered file.
func goldenPath(name string) string { return filepath.Join("testdata", "golden", name) }

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := goldenPath(name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", p, err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading golden %s (regenerate with -update): %v", p, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s does not match its golden.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// agentFiles is every rendered agent definition, keyed by basename.
func agentFiles(t *testing.T) map[string]string {
	t.Helper()
	byName := map[string]string{}
	agentsRoot := filepath.Join(fakeConfig, "opencode", "agents")
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile && filepath.Dir(tg.Path) == agentsRoot {
			byName[filepath.Base(tg.Path)] = string(tg.Content)
		}
	}
	return byName
}

func TestOpencodeGoldenAgents(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	byName := agentFiles(t)
	if len(byName) != len(sources) {
		t.Fatalf("plan rendered %d agent files for %d agent sources", len(byName), len(sources))
	}

	for _, s := range sources {
		content, ok := byName[s.Name+".md"]
		if !ok {
			t.Fatalf("no rendered agent file for %s", s.Name)
		}
		checkGolden(t, s.Name+".md", []byte(content))
	}

	// The golden set must not outlive its inventory.
	entries, err := os.ReadDir(filepath.Join("testdata", "golden"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	frozen := map[string]bool{}
	for _, e := range entries {
		frozen[e.Name()] = true
	}
	for _, s := range sources {
		delete(frozen, s.Name+".md")
	}
	delete(frozen, dispatchGoldenName)
	if len(frozen) != 0 {
		names := make([]string, 0, len(frozen))
		for n := range frozen {
			names = append(names, n)
		}
		sort.Strings(names)
		t.Errorf("goldens with no agent source: %v", names)
	}

	// The four pin cases, asserted against the emitted frontmatter rather than
	// the golden bytes, so a golden regenerated by mistake cannot quietly
	// redefine what "pinned" means.
	cases := []struct {
		file       string
		wantModel  string // "" means the model line must be absent
		wantEffort string // "" means the reasoningEffort line must be absent
	}{
		{"docket-build-standard.md", "openrouter/anthropic/claude-opus-5", "high"},
		{"docket-status.md", "openrouter/deepseek/deepseek-v4-flash-0731", ""},
		{"docket-adr.md", "", ""},                   // inherit / auto sentinels
		{"docket-review-lean.md", "", ""},           // no table entry at all
		{"docket-brainstorm-consultant.md", "", ""}, // no table entry, no skills
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			content, ok := byName[tc.file]
			if !ok {
				t.Fatalf("no rendered file %s", tc.file)
			}
			fm := frontmatterOf(content)
			if tc.wantModel == "" {
				if strings.Contains(fm, "\nmodel:") {
					t.Errorf("%s emits a model line for an unpinned agent:\n%s", tc.file, content)
				}
			} else if want := "\nmodel: " + tc.wantModel + "\n"; !strings.Contains(fm, want) {
				t.Errorf("%s does not carry %q", tc.file, strings.TrimSpace(want))
			}
			if tc.wantEffort == "" {
				if strings.Contains(fm, "reasoningEffort") {
					t.Errorf("%s emits a reasoningEffort line for an unpinned effort:\n%s", tc.file, content)
				}
			} else if want := "\nreasoningEffort: " + tc.wantEffort + "\n"; !strings.Contains(fm, want) {
				t.Errorf("%s does not carry %q", tc.file, strings.TrimSpace(want))
			}
			// Constant for every docket agent: they are dispatched, never primary.
			if !strings.Contains(fm, "\nmode: subagent") {
				t.Errorf("%s does not declare mode: subagent:\n%s", tc.file, content)
			}
			// opencode identifies an agent by FILENAME; a name: key is not part
			// of its schema.
			if strings.Contains(fm, "name:") {
				t.Errorf("%s emits a name key, which opencode does not read:\n%s", tc.file, content)
			}
			if strings.Contains(fm, "\neffort:") {
				t.Errorf("%s emits a bare effort key, which opencode does not read:\n%s", tc.file, content)
			}
			if !strings.HasPrefix(content, "---\ndescription: '") {
				t.Errorf("%s does not open with its description field: %.60q", tc.file, content)
			}
		})
	}
}

// frontmatterOf returns the text between the two fences, so a frontmatter
// assert cannot be satisfied — or defeated — by the agent's prose body.
func frontmatterOf(content string) string {
	rest, ok := strings.CutPrefix(content, "---\n")
	if !ok {
		return ""
	}
	fm, _, _ := strings.Cut(rest, "\n---\n")
	// Fenced on both sides so a `\n<key>: <value>\n` assert matches the first
	// and last frontmatter lines exactly as it does the middle ones.
	return "\n" + fm + "\n"
}

// The rendered document must carry the source's description, its skills
// preamble, and its body verbatim — the mapping that mirrors emit_opencode_md.
func TestOpencodeAgentMirrorsSource(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	byName := agentFiles(t)
	for _, s := range sources {
		content, ok := byName[s.Name+".md"]
		if !ok {
			t.Fatalf("no rendered file for %s", s.Name)
		}
		// The self-recursion guard is the first paragraph after the frontmatter;
		// the skills preamble, when present, follows it. Both anchors move down by
		// the guard paragraph, so they are re-anchored on it here.
		guard := harness.RecursionGuard(s.Name)
		if !strings.Contains(content, "---\n\n"+guard+"\n\n") {
			t.Errorf("%s does not carry the recursion guard right after its frontmatter", s.Name)
		}
		preamble := "Before acting, load these docket skills from your opencode skills directory: " +
			strings.Join(s.Skills, ", ") + "."
		if len(s.Skills) == 0 {
			if strings.Contains(content, "Before acting, load these docket skills") {
				t.Errorf("%s carries a skills preamble for an agent that preloads none", s.Name)
			}
		} else if !strings.Contains(content, guard+"\n\n"+preamble+"\n\n") {
			t.Errorf("%s does not carry the skills preamble %q after the guard", s.Name, preamble)
		}
		if !strings.Contains(content, strings.TrimRight(s.Body, "\n")) {
			t.Errorf("%s does not carry its source body verbatim", s.Name)
		}
	}
}

// An effort with no model is dropped: docket refuses to pin an effort it
// cannot attribute to a resolved model. The drop is silent — a rendering fact,
// not a stream warning, since an adapter is pure computation.
func TestOpencodeEffortDrop(t *testing.T) {
	in := fixtureInput(t)
	in.Agents = config.AgentsTable{
		"opencode": {
			"status": {Effort: config.Value[string]{Value: "xhigh"}},
			// The `inherit` sentinel is a no-pin model, so its effort drops too.
			"adr": {
				Model:  config.Value[string]{Value: "inherit"},
				Effort: config.Value[string]{Value: "high"},
			},
			// A model with an `auto` effort keeps its model and drops nothing else.
			"build-max": {
				Model:  config.Value[string]{Value: "openrouter/x/y"},
				Effort: config.Value[string]{Value: "auto"},
			},
		},
	}
	targets, err := New().Plan(in)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	agentsRoot := filepath.Join(fakeConfig, "opencode", "agents")
	seen := 0
	for _, tg := range targets {
		if tg.Kind != install.KindFile || filepath.Dir(tg.Path) != agentsRoot {
			continue
		}
		name := filepath.Base(tg.Path)
		content := string(tg.Content)
		fm := frontmatterOf(content)
		switch name {
		case "docket-status.md", "docket-adr.md":
			seen++
			if strings.Contains(fm, "\nmodel:") {
				t.Errorf("%s emits a model line for an effort-only pin:\n%s", name, content)
			}
			for _, token := range []string{"reasoningEffort", "xhigh", "high"} {
				if strings.Contains(fm, token) {
					t.Errorf("%s leaks the dropped effort (%q) into its frontmatter:\n%s", name, token, content)
				}
			}
		case "docket-build-max.md":
			seen++
			if !strings.Contains(fm, "\nmodel: openrouter/x/y\n") {
				t.Errorf("%s dropped a model pinned alongside an auto effort:\n%s", name, content)
			}
			if strings.Contains(fm, "reasoningEffort") {
				t.Errorf("%s emits reasoningEffort for the auto sentinel:\n%s", name, content)
			}
		}
	}
	if seen != 3 {
		t.Fatalf("checked %d of the 3 effort-drop cases; the fixture inventory is stale", seen)
	}
}

const dispatchGoldenName = "dispatch-block-interior.md"

func dispatchTarget(t *testing.T) install.Target {
	t.Helper()
	var found []install.Target
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindManagedBlock {
			found = append(found, tg)
		}
	}
	if len(found) != 1 {
		t.Fatalf("plan carries %d managed-block targets, want 1", len(found))
	}
	return found[0]
}

func TestOpencodeDispatchBlockPath(t *testing.T) {
	tg := dispatchTarget(t)
	if want := filepath.Join(fakeConfig, "opencode", "AGENTS.md"); tg.Path != want {
		t.Errorf("dispatch path = %q, want %q", tg.Path, want)
	}
	if tg.Kind != install.KindManagedBlock {
		t.Errorf("dispatch kind = %v, want a managed block", tg.Kind)
	}
	if tg.BlockName != "dispatch" {
		t.Errorf("block name = %q, want dispatch", tg.BlockName)
	}
	if tg.Annotation != "managed by docket — do not hand-edit" {
		t.Errorf("annotation = %q", tg.Annotation)
	}
}

func TestOpencodeDispatchGolden(t *testing.T) {
	tg := dispatchTarget(t)
	interior := string(tg.Content)
	checkGolden(t, dispatchGoldenName, tg.Content)

	// The roster is gone (change 0334): assert its removal by SHAPE — no line is
	// a `- **docket-...` bullet and the delegation clause is absent — never by a
	// spelling list. The compact routing rule carries its load-bearing phrases in
	// the roster's place.
	for _, line := range strings.Split(interior, "\n") {
		if strings.HasPrefix(line, "- **docket-") {
			t.Errorf("dispatch interior still carries a roster bullet: %q", line)
		}
	}
	if strings.Contains(interior, "Delegate to the") {
		t.Errorf("dispatch interior still carries the roster delegation clause")
	}
	for _, phrase := range []string{
		"registered same-name",
		"authoritative for agent names, descriptions, and availability",
		"do not invent one",
	} {
		if !strings.Contains(interior, phrase) {
			t.Errorf("dispatch interior is missing the routing-rule phrase %q", phrase)
		}
	}
	if !strings.HasPrefix(interior, harness.DispatchHeading+"\n") {
		t.Errorf("dispatch interior does not open with the heading: %.60q", interior)
	}

	in := fixtureInput(t)
	runGate, err := in.Assets.Bytes("cursor-rules/run-gate.md")
	if err != nil {
		t.Fatalf("run-gate payload: %v", err)
	}
	if !strings.Contains(interior, strings.TrimRight(string(runGate), "\n")) {
		t.Errorf("dispatch interior does not carry the run-gate asset verbatim")
	}

	// Nothing in an opencode-bound artifact may name another harness or its runner.
	for _, token := range []string{"claude", "codex", "cursor"} {
		if strings.Contains(strings.ToLower(interior), token) {
			t.Errorf("dispatch interior names %q, which belongs to another harness", token)
		}
	}
}

// A hostile description must survive its own serialization: the rendered
// document reparses and every field reads back as the value it was rendered
// from, or the plan fails rather than shipping a file that parses into
// something else.
func TestOpencodeYAMLEscaping(t *testing.T) {
	const short = "hostile"
	const name = "docket-" + short
	files := map[string]string{
		"agents/" + name + ".md": "---\nname: " + name +
			"\ndescription: 'a: b # c ''quoted'' and a back\\slash'\nskills: [docket-build-task]\n---\nBody line.\n",
	}
	targets, err := New().Plan(harness.PlanInput{
		Assets:    syntheticAgentCatalog(files),
		Mode:      harness.ModeRelease,
		AssetsDir: assetsDir,
		Roots:     fixtureRoots(),
		Agents:    fixtureAgents(),
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	var content string
	for _, tg := range targets {
		if filepath.Base(tg.Path) == name+".md" {
			content = string(tg.Content)
		}
	}
	if content == "" {
		t.Fatalf("no rendered agent file")
	}
	want := "description: 'a: b # c ''quoted'' and a back\\slash'\n"
	if !strings.Contains(content, want) {
		t.Errorf("description not quoted as a single-quoted YAML scalar.\ngot:\n%s\nwant line:\n%s", content, want)
	}
}

// syntheticAgentCatalog carries hand-written agent sources plus the dispatch
// assets the plan needs, so a rendering case can be driven by inputs the real
// bundle can never exhibit.
func syntheticAgentCatalog(agentFiles map[string]string) assets.Catalog {
	payload := map[string][]byte{}
	m := assets.Manifest{FormatVersion: assets.ManifestFormatVersion, AssetProtocol: assets.AssetProtocol}

	add := func(p string, role assets.Role, body string) {
		payload[p] = []byte(body)
		m.Entries = append(m.Entries, assets.Entry{Path: p, Role: role, Mode: 0o644, Size: int64(len(body))})
	}
	paths := make([]string, 0, len(agentFiles))
	for p := range agentFiles {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		add(p, assets.RoleAgentSource, agentFiles[p])
	}
	add("cursor-rules/run-gate.md", assets.RoleDispatch, "## Run gate — verify a dispatched implement-next run before you relay it\n\nRead git.\n")
	add("skills/docket-build-task/SKILL.md", assets.RoleSkill, "skill")
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Path < m.Entries[j].Path })

	return assets.NewCatalog(m, func(p string) ([]byte, error) {
		b, ok := payload[p]
		if !ok {
			return nil, fmt.Errorf("no such payload %q", p)
		}
		return b, nil
	})
}

// A bundle with no run-gate payload is a corrupt bundle, never a plan that
// silently ships a dispatch block missing its gate.
func TestOpencodePlanRequiresRunGate(t *testing.T) {
	base := syntheticAgentCatalog(map[string]string{
		"agents/docket-alpha.md": "---\nname: docket-alpha\ndescription: Alpha.\n---\nBody.\n",
	})
	m := base.Manifest
	kept := m.Entries[:0:0]
	for _, e := range m.Entries {
		if e.Path != "cursor-rules/run-gate.md" {
			kept = append(kept, e)
		}
	}
	m.Entries = kept
	_, err := New().Plan(harness.PlanInput{
		Assets:    assets.NewCatalog(m, base.Bytes),
		Mode:      harness.ModeRelease,
		AssetsDir: assetsDir,
		Roots:     fixtureRoots(),
		Agents:    fixtureAgents(),
	})
	if err == nil {
		t.Fatalf("Plan accepted a bundle with no run-gate payload")
	}
}

func TestOpencodePlanRejectsUnusableInput(t *testing.T) {
	in := fixtureInput(t)

	t.Run("relative assets dir", func(t *testing.T) {
		bad := in
		bad.AssetsDir = "relative/assets"
		if _, err := New().Plan(bad); err == nil {
			t.Fatalf("Plan accepted a relative AssetsDir")
		}
	})
	t.Run("empty config home", func(t *testing.T) {
		bad := in
		bad.Roots = install.UserRoots{Home: fakeHome}
		if _, err := New().Plan(bad); err == nil {
			t.Fatalf("Plan accepted roots with no ConfigHome")
		}
	})
	t.Run("relative config home", func(t *testing.T) {
		bad := in
		bad.Roots.ConfigHome = "cfg"
		if _, err := New().Plan(bad); err == nil {
			t.Fatalf("Plan accepted a relative ConfigHome")
		}
	})
}

// Adding an agent source to the bundle must grow the plan by exactly one agent
// file with no adapter edit. Since change 0334 the dispatch block no longer
// restates the roster, so the added agent must NOT surface in the dispatch
// interior — the harness's own registry is the roster. The claude adapter
// carries the same probe; each adapter needs its own because each derives its
// own destinations.
func TestOpencodeInventoryAdditionPropagates(t *testing.T) {
	in := fixtureInput(t)
	before := planFixture(t)
	beforeInterior := string(dispatchTarget(t).Content)

	const extraPath = "agents/docket-zzz-synthetic.md"
	const extraBody = "---\nname: docket-zzz-synthetic\ndescription: A synthetic seventeenth agent.\n---\nSynthetic body.\n"

	base := in.Assets
	m := base.Manifest
	m.Entries = append(append([]assets.Entry(nil), m.Entries...), assets.Entry{
		Path: extraPath, Role: assets.RoleAgentSource, Mode: 0o644, Size: int64(len(extraBody)),
	})
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Path < m.Entries[j].Path })
	grown := assets.NewCatalog(m, func(p string) ([]byte, error) {
		if p == extraPath {
			return []byte(extraBody), nil
		}
		return base.Bytes(p)
	})

	in.Assets = grown
	after, err := New().Plan(in)
	if err != nil {
		t.Fatalf("Plan over the grown catalog: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("plan grew from %d to %d targets, want exactly one more", len(before), len(after))
	}

	wantPath := filepath.Join(fakeConfig, rootDir, agentsDir, "docket-zzz-synthetic.md")
	var found bool
	var afterInterior string
	for _, tg := range after {
		if tg.Path == wantPath && tg.Kind == install.KindFile {
			found = true
		}
		if tg.Kind == install.KindManagedBlock {
			afterInterior = string(tg.Content)
		}
	}
	if !found {
		t.Errorf("the grown plan carries no agent file at %s", wantPath)
	}
	// The dispatch interior is inventory-independent now: adding an agent leaves
	// it byte-for-byte unchanged and never leaks the added agent's name.
	if afterInterior != beforeInterior {
		t.Errorf("adding an agent changed the (now inventory-independent) dispatch interior")
	}
	if strings.Contains(afterInterior, "docket-zzz-synthetic") {
		t.Errorf("the dispatch interior leaked the added agent name")
	}
}
