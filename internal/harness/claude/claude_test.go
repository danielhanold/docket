package claude

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
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
)

// -update rewrites the frozen goldens from the current renderer. It exists so a
// deliberate rendering change is a reviewable diff rather than a hand edit; it
// is never set in CI, and the drift tie that makes the goldens meaningful lives
// upstream (internal/assets' authored→embedded correspondence test), so a
// regenerated golden still cannot launder an unauthored asset.
var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current renderer")

const (
	fakeHome  = "/home/u"
	assetsDir = "/data/versions/sha256-x/assets"
)

// The fixed input table the goldens are frozen under: one agent pinned on both
// fields, one model-only, one carrying both no-pin sentinels. Every other agent
// resolves to no pin at all, which is the fourth case.
func fixtureAgents() config.AgentsTable {
	return config.AgentsTable{
		"claude": {
			"build-standard": {
				Model:  config.Value[string]{Value: "claude-opus-5[1m]"},
				Effort: config.Value[string]{Value: "high"},
			},
			"status": {
				Model: config.Value[string]{Value: "claude-sonnet-4-6"},
			},
			"adr": {
				Model:  config.Value[string]{Value: "inherit"},
				Effort: config.Value[string]{Value: "auto"},
			},
			// `inherit` alongside a real effort pin: sync-agents.sh's emit()
			// drops neither, so neither does this renderer.
			"review-deep": {
				Model:  config.Value[string]{Value: "inherit"},
				Effort: config.Value[string]{Value: "medium"},
			},
		},
		// A sibling harness's pins must never leak into the claude rendering.
		"codex": {
			"build-standard": {
				Model:  config.Value[string]{Value: "gpt-5.5-codex"},
				Effort: config.Value[string]{Value: "low"},
			},
		},
	}
}

func fixtureRoots() install.UserRoots {
	return install.UserRoots{
		Home:       fakeHome,
		DataRoot:   fakeHome + "/.local/share/docket",
		ConfigHome: fakeHome + "/.config",
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

func TestClaudeName(t *testing.T) {
	if got := New().Name(); got != "claude" {
		t.Fatalf("Name = %q, want claude", got)
	}
}

func TestClaudePlanDeterministic(t *testing.T) {
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

// Every target must sit under the claude root, never under a sibling harness's.
func TestClaudePlanStaysInClaudeRoot(t *testing.T) {
	root := filepath.Join(fakeHome, ".claude")
	for _, tg := range planFixture(t) {
		if !strings.HasPrefix(tg.Path, root+string(filepath.Separator)) {
			t.Errorf("target %q escapes the claude root %q", tg.Path, root)
		}
	}
}

func TestClaudeSkillLinks(t *testing.T) {
	c, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	want := map[string]string{}
	for _, dir := range harness.SkillDirs(c) {
		want[filepath.Join(fakeHome, ".claude", "skills", dir)] = filepath.Join(assetsDir, "skills", dir)
	}
	if len(want) == 0 {
		t.Fatalf("the bundle carries no skill dirs; the fixture assumption is stale")
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

func TestClaudeDetect(t *testing.T) {
	home := t.TempDir()
	roots := install.UserRoots{Home: home}

	d := New().Detect(roots)
	if d.Present {
		t.Errorf("Detect reported present with no ~/.claude")
	}
	if d.Root != filepath.Join(home, ".claude") {
		t.Errorf("Detect root = %q", d.Root)
	}

	// A regular file at the root is not a harness installation.
	if err := os.WriteFile(filepath.Join(home, ".claude"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if New().Detect(roots).Present {
		t.Errorf("Detect reported present for a regular file at ~/.claude")
	}
	if err := os.Remove(filepath.Join(home, ".claude")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if !New().Detect(roots).Present {
		t.Errorf("Detect did not report present with ~/.claude a directory")
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

func TestClaudeGoldenAgents(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}

	byPath := map[string][]byte{}
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile {
			byPath[tg.Path] = tg.Content
		}
	}
	if len(byPath) != len(sources) {
		t.Fatalf("plan rendered %d agent files for %d agent sources", len(byPath), len(sources))
	}

	for _, s := range sources {
		p := filepath.Join(fakeHome, ".claude", "agents", s.Name+".md")
		content, ok := byPath[p]
		if !ok {
			t.Fatalf("no rendered agent file at %s", p)
		}
		checkGolden(t, s.Name+".md", content)
	}

	// The golden set must not outlive its inventory: a stale file left behind
	// by a removed agent would keep passing forever, unread.
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
}

// The three pin cases the fixed table exercises, asserted against the parsed
// frontmatter rather than the golden bytes, so a golden regenerated by mistake
// cannot quietly redefine what "pinned" means.
func TestClaudeAgentPinCases(t *testing.T) {
	byName := map[string][]byte{}
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile {
			byName[filepath.Base(tg.Path)] = tg.Content
		}
	}

	cases := []struct {
		file         string
		model, efrt  string
		wantModelSet bool
		wantEfrtSet  bool
	}{
		{"docket-build-standard.md", "claude-opus-5[1m]", "high", true, true},
		{"docket-status.md", "claude-sonnet-4-6", "", true, false},
		// `inherit` is a real Claude Code frontmatter value, not a docket
		// sentinel on this harness, so it renders verbatim; `auto` is a docket
		// sentinel on every harness and drops.
		{"docket-adr.md", "inherit", "", true, false},
		{"docket-review-deep.md", "inherit", "medium", true, true},
		{"docket-review-lean.md", "", "", false, false},           // no table entry at all
		{"docket-brainstorm-consultant.md", "", "", false, false}, // no table entry, no skills
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			content, ok := byName[tc.file]
			if !ok {
				t.Fatalf("no rendered file %s", tc.file)
			}
			doc, err := document.Parse(content)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			var fm struct {
				Name        string   `yaml:"name"`
				Description string   `yaml:"description"`
				Skills      []string `yaml:"skills"`
				Model       string   `yaml:"model"`
				Effort      string   `yaml:"effort"`
			}
			if err := doc.DecodeFrontmatter(&fm); err != nil {
				t.Fatalf("DecodeFrontmatter: %v", err)
			}
			if fm.Model != tc.model {
				t.Errorf("model = %q, want %q", fm.Model, tc.model)
			}
			if fm.Effort != tc.efrt {
				t.Errorf("effort = %q, want %q", fm.Effort, tc.efrt)
			}
			if _, has := doc.Field("model"); has != tc.wantModelSet {
				t.Errorf("model key present = %v, want %v", has, tc.wantModelSet)
			}
			if _, has := doc.Field("effort"); has != tc.wantEfrtSet {
				t.Errorf("effort key present = %v, want %v", has, tc.wantEfrtSet)
			}
			if fm.Name != strings.TrimSuffix(tc.file, ".md") {
				t.Errorf("name = %q, want %q", fm.Name, strings.TrimSuffix(tc.file, ".md"))
			}
		})
	}
}

// The rendered file must carry the source's own description, skills, and body
// unchanged — that mapping is what mirrors sync-agents.sh's emit().
func TestClaudeAgentMirrorsSource(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	byName := map[string][]byte{}
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile {
			byName[filepath.Base(tg.Path)] = tg.Content
		}
	}
	for _, s := range sources {
		content, ok := byName[s.Name+".md"]
		if !ok {
			t.Fatalf("no rendered file for %s", s.Name)
		}
		doc, err := document.Parse(content)
		if err != nil {
			t.Fatalf("%s: Parse: %v", s.Name, err)
		}
		var fm struct {
			Name        string   `yaml:"name"`
			Description string   `yaml:"description"`
			Skills      []string `yaml:"skills"`
		}
		if err := doc.DecodeFrontmatter(&fm); err != nil {
			t.Fatalf("%s: DecodeFrontmatter: %v", s.Name, err)
		}
		if fm.Name != s.Name {
			t.Errorf("%s: name = %q", s.Name, fm.Name)
		}
		if fm.Description != s.Description {
			t.Errorf("%s: description round-trip = %q, want %q", s.Name, fm.Description, s.Description)
		}
		if len(s.Skills) == 0 {
			if _, has := doc.Field("skills"); has {
				t.Errorf("%s: skills key emitted for an agent that preloads none", s.Name)
			}
		} else if !reflect.DeepEqual(fm.Skills, s.Skills) {
			t.Errorf("%s: skills = %v, want %v", s.Name, fm.Skills, s.Skills)
		}
		if !strings.HasSuffix(string(content), s.Body) {
			t.Errorf("%s: body is not carried verbatim at the end of the file", s.Name)
		}
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

func TestClaudeDispatchGolden(t *testing.T) {
	tg := dispatchTarget(t)
	if want := filepath.Join(fakeHome, ".claude", "CLAUDE.md"); tg.Path != want {
		t.Errorf("dispatch path = %q, want %q", tg.Path, want)
	}
	if tg.BlockName != "dispatch" {
		t.Errorf("block name = %q, want dispatch", tg.BlockName)
	}
	if tg.Annotation != "managed by docket — do not hand-edit" {
		t.Errorf("annotation = %q", tg.Annotation)
	}

	interior := string(tg.Content)
	checkGolden(t, dispatchGoldenName, tg.Content)

	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	for _, s := range sources {
		bullet := fmt.Sprintf("- **%s** — %s Delegate to the `%s` agent.", s.Name, s.Description, s.Name)
		if !strings.Contains(interior, bullet) {
			t.Errorf("dispatch interior is missing the bullet for %s", s.Name)
		}
	}
	if got := strings.Count(interior, "\n- **docket-"); got != len(sources) {
		t.Errorf("dispatch interior carries %d bullets for %d sources", got, len(sources))
	}
	if !strings.HasPrefix(interior, "## Docket agents — dispatch, don't run inline\n") {
		t.Errorf("dispatch interior does not open with the preamble heading: %.60q", interior)
	}
	if !strings.Contains(interior, "## Run gate — bracket a dispatched implement-next run with the gate facade") {
		t.Errorf("dispatch interior does not carry the run-gate heading")
	}

	// The run-gate payload is passthrough, never re-authored.
	runGate, err := in.Assets.Bytes("cursor-rules/run-gate.md")
	if err != nil {
		t.Fatalf("run-gate payload: %v", err)
	}
	if !strings.Contains(interior, strings.TrimRight(string(runGate), "\n")) {
		t.Errorf("dispatch interior does not carry the run-gate asset verbatim")
	}

	// Nothing in a claude-bound artifact may name another harness or its runner.
	for _, token := range []string{"codex", "cursor", "opencode", "runner"} {
		if strings.Contains(strings.ToLower(interior), token) {
			t.Errorf("dispatch interior names %q, which belongs to another harness", token)
		}
	}
}

// A description carrying YAML indicator characters must still render a document
// that reparses to the same description — the write-boundary quoting rule.
func TestClaudeEscaping(t *testing.T) {
	hostile := []string{
		"a: b # c",
		"trailing colon:",
		"*anchor and &alias and [flow] and {map}",
		"quotes ' and \" and a backslash \\",
		"yes",
		"- leading dash",
		"  padded  ",
	}
	for i, desc := range hostile {
		body := fmt.Sprintf("Body %d.\n", i)
		files := map[string]string{}
		short := fmt.Sprintf("hostile%d", i)
		name := "docket-" + short
		// The fixture is written as a single-quoted YAML scalar (a literal
		// quote is doubled), so the hostile description reaches ParseInventory
		// intact; the assertion below is on what the RENDERER writes back.
		fmDesc := "'" + strings.ReplaceAll(desc, "'", "''") + "'"
		files["agents/"+name+".md"] = "---\nname: " + name + "\ndescription: " + fmDesc + "\nskills: [docket-build-task]\n---\n" + body

		c := syntheticAgentCatalog(files)
		targets, err := New().Plan(harness.PlanInput{
			Assets:    c,
			Mode:      harness.ModeRelease,
			AssetsDir: assetsDir,
			Roots:     fixtureRoots(),
			Agents:    fixtureAgents(),
		})
		if err != nil {
			t.Fatalf("%q: Plan: %v", desc, err)
		}
		var content []byte
		for _, tg := range targets {
			if tg.Kind == install.KindFile && strings.HasSuffix(tg.Path, name+".md") {
				content = tg.Content
			}
		}
		if content == nil {
			t.Fatalf("%q: no rendered agent file", desc)
		}
		doc, err := document.Parse(content)
		if err != nil {
			t.Fatalf("%q: rendered file does not parse: %v\n%s", desc, err, content)
		}
		var fm struct {
			Name        string   `yaml:"name"`
			Description string   `yaml:"description"`
			Skills      []string `yaml:"skills"`
		}
		if err := doc.DecodeFrontmatter(&fm); err != nil {
			t.Fatalf("%q: DecodeFrontmatter: %v", desc, err)
		}
		if fm.Description != desc {
			t.Errorf("description round-trip = %q, want %q", fm.Description, desc)
		}
		if fm.Name != name {
			t.Errorf("name round-trip = %q, want %q", fm.Name, name)
		}
		if !reflect.DeepEqual(fm.Skills, []string{"docket-build-task"}) {
			t.Errorf("skills round-trip = %v", fm.Skills)
		}
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

// Adding an agent source to the bundle must grow the plan by one agent file and
// one dispatch bullet with no adapter edit — the inventory is the single roster.
func TestInventoryAdditionPropagates(t *testing.T) {
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

	wantPath := filepath.Join(fakeHome, ".claude", "agents", "docket-zzz-synthetic.md")
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
	if strings.Count(afterInterior, "\n- **docket-") != strings.Count(beforeInterior, "\n- **docket-")+1 {
		t.Errorf("the dispatch interior did not grow by exactly one bullet")
	}
	if !strings.Contains(afterInterior, "- **docket-zzz-synthetic** — A synthetic seventeenth agent. Delegate to the `docket-zzz-synthetic` agent.") {
		t.Errorf("the dispatch interior carries no bullet for the added agent")
	}
}

// A bundle with no run-gate payload is a corrupt bundle, never a plan that
// silently ships a dispatch block missing its gate.
func TestClaudePlanRequiresRunGate(t *testing.T) {
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
	stripped := assets.NewCatalog(m, base.Bytes)

	_, err := New().Plan(harness.PlanInput{
		Assets:    stripped,
		Mode:      harness.ModeRelease,
		AssetsDir: assetsDir,
		Roots:     fixtureRoots(),
		Agents:    fixtureAgents(),
	})
	if err == nil {
		t.Fatalf("Plan accepted a bundle with no run-gate payload")
	}
	if !strings.Contains(err.Error(), "run-gate") {
		t.Fatalf("error %q does not name the missing run-gate payload", err)
	}
}

func TestClaudePlanRejectsUnusableInput(t *testing.T) {
	in := fixtureInput(t)

	t.Run("relative assets dir", func(t *testing.T) {
		bad := in
		bad.AssetsDir = "relative/assets"
		if _, err := New().Plan(bad); err == nil {
			t.Fatalf("Plan accepted a relative AssetsDir")
		}
	})
	t.Run("empty home", func(t *testing.T) {
		bad := in
		bad.Roots = install.UserRoots{}
		if _, err := New().Plan(bad); err == nil {
			t.Fatalf("Plan accepted empty roots")
		}
	})
}
