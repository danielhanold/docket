package codex

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
// the claude adapter: a deliberate rendering change becomes a reviewable diff
// rather than a hand edit, and it is never set in CI.
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
		"codex": {
			"build-standard": {
				Model:  config.Value[string]{Value: "gpt-5.5-codex"},
				Effort: config.Value[string]{Value: "high"},
			},
			"status": {
				Model: config.Value[string]{Value: "gpt-5.5"},
			},
			"adr": {
				Model:  config.Value[string]{Value: "inherit"},
				Effort: config.Value[string]{Value: "auto"},
			},
		},
		// A sibling harness's pins must never leak into the codex rendering.
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

func TestCodexName(t *testing.T) {
	if got := New().Name(); got != "codex" {
		t.Fatalf("Name = %q, want codex", got)
	}
}

func TestCodexPlanDeterministic(t *testing.T) {
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

// Codex reads its skills from a harness-neutral root that docket links into
// $HOME/.agents/skills, NOT from ~/.codex/skills. Both sides of that fact are
// asserted here, because writing the links under the codex root would be a
// silent no-op installation Codex never reads.
func TestCodexSkillsRootIsAgents(t *testing.T) {
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
		want[filepath.Join(fakeHome, ".agents", "skills", dir)] = filepath.Join(assetsDir, "skills", dir)
	}
	if _, ok := want[filepath.Join(fakeHome, ".agents", "skills", "docket-build")]; !ok {
		t.Fatalf("the bundle carries no docket-build skill; the fixture assumption is stale")
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

	// The negative half: nothing in the whole plan may sit under ~/.codex/skills.
	bad := filepath.Join(fakeHome, ".codex", "skills")
	for _, tg := range planFixture(t) {
		if strings.HasPrefix(tg.Path, bad+string(filepath.Separator)) || tg.Path == bad {
			t.Errorf("target %q sits under the codex skills root, which Codex does not read", tg.Path)
		}
	}
}

func TestCodexDetect(t *testing.T) {
	home := t.TempDir()
	roots := install.UserRoots{Home: home}

	d := New().Detect(roots)
	if d.Present {
		t.Errorf("Detect reported present with no ~/.codex")
	}
	if d.Root != filepath.Join(home, ".codex") {
		t.Errorf("Detect root = %q", d.Root)
	}

	// A regular file at the root is not a harness installation.
	if err := os.WriteFile(filepath.Join(home, ".codex"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if New().Detect(roots).Present {
		t.Errorf("Detect reported present for a regular file at ~/.codex")
	}
	if err := os.Remove(filepath.Join(home, ".codex")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if err := os.Mkdir(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if !New().Detect(roots).Present {
		t.Errorf("Detect did not report present with ~/.codex a directory")
	}

	// Empty roots resolve to no detection rather than a bare-relative stat.
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

func TestCodexGoldenAgents(t *testing.T) {
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
		p := filepath.Join(fakeHome, ".codex", "agents", s.Name+".toml")
		content, ok := byPath[p]
		if !ok {
			t.Fatalf("no rendered agent file at %s", p)
		}
		checkGolden(t, s.Name+".toml", content)
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
		delete(frozen, s.Name+".toml")
	}
	if len(frozen) != 0 {
		names := make([]string, 0, len(frozen))
		for n := range frozen {
			names = append(names, n)
		}
		sort.Strings(names)
		t.Errorf("goldens with no agent source: %v", names)
	}
}

func renderedByName(t *testing.T) map[string]string {
	t.Helper()
	byName := map[string]string{}
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile {
			byName[filepath.Base(tg.Path)] = string(tg.Content)
		}
	}
	return byName
}

// The three pin cases the fixed table exercises, asserted against the emitted
// TOML keys rather than the golden bytes, so a golden regenerated by mistake
// cannot quietly redefine what "pinned" means.
func TestCodexAgentPinCases(t *testing.T) {
	byName := renderedByName(t)
	cases := []struct {
		file       string
		wantModel  string // "" means the key must be absent
		wantEffort string
	}{
		{"docket-build-standard.toml", "gpt-5.5-codex", "high"},
		{"docket-status.toml", "gpt-5.5", ""},
		{"docket-adr.toml", "", ""},                   // inherit / auto sentinels
		{"docket-review-lean.toml", "", ""},           // no table entry at all
		{"docket-brainstorm-consultant.toml", "", ""}, // no table entry, no skills
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			content, ok := byName[tc.file]
			if !ok {
				t.Fatalf("no rendered file %s", tc.file)
			}
			modelLine := "model = \"" + tc.wantModel + "\"\n"
			if tc.wantModel == "" {
				if strings.Contains(content, "\nmodel = ") || strings.HasPrefix(content, "model = ") {
					t.Errorf("%s emits a model key for an unpinned agent", tc.file)
				}
			} else if !strings.Contains(content, modelLine) {
				t.Errorf("%s does not carry %q", tc.file, strings.TrimRight(modelLine, "\n"))
			}
			effortLine := "model_reasoning_effort = \"" + tc.wantEffort + "\"\n"
			if tc.wantEffort == "" {
				if strings.Contains(content, "model_reasoning_effort") {
					t.Errorf("%s emits a model_reasoning_effort key for an unpinned effort", tc.file)
				}
			} else if !strings.Contains(content, effortLine) {
				t.Errorf("%s does not carry %q", tc.file, strings.TrimRight(effortLine, "\n"))
			}
			name := strings.TrimSuffix(tc.file, ".toml")
			if !strings.HasPrefix(content, "name = \""+name+"\"\n") {
				t.Errorf("%s does not open with its name key: %.60q", tc.file, content)
			}
		})
	}
}

// Codex has no equivalent of Claude Code's `inherit`, so the sentinel
// normalizes to "no model key" here — while the effort beside it survives, the
// two being tested independently in emit_codex_toml. This is the asymmetry the
// claude adapter deliberately does not share.
func TestCodexInheritDropsModelKeepsEffort(t *testing.T) {
	in := fixtureInput(t)
	in.Agents = config.AgentsTable{
		"codex": {
			"adr": {
				Model:  config.Value[string]{Value: "inherit"},
				Effort: config.Value[string]{Value: "high"},
			},
		},
	}
	targets, err := New().Plan(in)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := filepath.Join(fakeHome, ".codex", "agents", "docket-adr.toml")
	var content string
	for _, tg := range targets {
		if tg.Path == want {
			content = string(tg.Content)
		}
	}
	if content == "" {
		t.Fatalf("no rendered file %s", want)
	}
	if strings.Contains(content, "model = ") {
		t.Errorf("docket-adr.toml emits a model key for the inherit sentinel:\n%s", content)
	}
	if !strings.Contains(content, "model_reasoning_effort = \"high\"\n") {
		t.Errorf("docket-adr.toml drops the effort pin beside an inherit model:\n%s", content)
	}
}

// The rendered document must carry the source's description, its skills
// preamble, and its body — the mapping that mirrors emit_codex_toml.
func TestCodexAgentMirrorsSource(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	byName := renderedByName(t)
	for _, s := range sources {
		content, ok := byName[s.Name+".toml"]
		if !ok {
			t.Fatalf("no rendered file for %s", s.Name)
		}
		if !strings.Contains(content, "description = \""+s.Description+"\"\n") {
			t.Errorf("%s does not carry its source description", s.Name)
		}
		preamble := "Before acting, load these docket skills from your linked Codex skills directory: " +
			strings.Join(s.Skills, ", ") + "."
		if len(s.Skills) == 0 {
			if strings.Contains(content, "Before acting, load these docket skills") {
				t.Errorf("%s carries a skills preamble for an agent that preloads none", s.Name)
			}
		} else if !strings.Contains(content, preamble) {
			t.Errorf("%s does not carry the skills preamble %q", s.Name, preamble)
		}
		if !strings.HasPrefix(content, "name = \""+s.Name+"\"\n") {
			t.Errorf("%s does not open with its name key", s.Name)
		}
		if !strings.Contains(content, "developer_instructions = \"\"\"\n") ||
			!strings.HasSuffix(content, "\n\"\"\"\n") {
			t.Errorf("%s does not carry a multi-line developer_instructions string", s.Name)
		}
		// The body survives into the instructions. Compare on the first
		// non-blank body line, which no escaping in the bundle touches.
		for _, line := range strings.Split(strings.TrimSpace(s.Body), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !strings.Contains(content, line) {
				t.Errorf("%s: body line %q is missing from developer_instructions", s.Name, line)
			}
			break
		}
	}
}

// TOML basic-string escaping for the scalar keys, and the multi-line
// terminator defence for the instructions body.
func TestCodexTOMLEscaping(t *testing.T) {
	const short = "hostile"
	const name = "docket-" + short
	desc := `a "quoted" thing and a back\slash`
	body := "Body with a \"\"\" terminator and a back\\slash.\n"

	files := map[string]string{
		"agents/" + name + ".md": "---\nname: " + name +
			"\ndescription: '" + strings.ReplaceAll(desc, "'", "''") + "'\nskills: [docket-build-task]\n---\n" + body,
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
		if tg.Kind == install.KindFile && strings.HasSuffix(tg.Path, name+".toml") {
			content = string(tg.Content)
		}
	}
	if content == "" {
		t.Fatalf("no rendered agent file")
	}

	wantDesc := "description = \"a \\\"quoted\\\" thing and a back\\\\slash\"\n"
	if !strings.Contains(content, wantDesc) {
		t.Errorf("description not escaped as a TOML basic string.\ngot:\n%s\nwant line:\n%s", content, wantDesc)
	}
	if !strings.Contains(content, "Body with a \"\"\\\" terminator and a back\\\\slash.") {
		t.Errorf("developer_instructions did not escape the \"\"\" terminator or the backslash.\ngot:\n%s", content)
	}
	// The only bare """ left in the document are the two delimiters.
	if got := strings.Count(content, "\"\"\"\n"); got != 2 {
		t.Errorf("document carries %d unescaped triple-quote runs at line end, want exactly the 2 delimiters:\n%s", got, content)
	}
}

// Change 0351 retired the user-global dispatch surface: Plan no longer emits a
// managed block into ~/.codex/AGENTS.md. Parent-facing routing now lives in a
// repository's own AGENTS.md, never a personal global one.
func TestCodexPlanHasNoGlobalDispatch(t *testing.T) {
	for _, tg := range planFixture(t) {
		if tg.Role == "dispatch" {
			t.Errorf("plan still carries a dispatch-role target at %q", tg.Path)
		}
		if tg.Kind == install.KindManagedBlock {
			t.Errorf("plan still carries a managed-block target at %q", tg.Path)
		}
	}
}

// GlobalDispatchTarget still names the historical destination so the installer
// can retire a leftover a prior install owns — location and identity only, no
// Content.
func TestCodexGlobalDispatchTarget(t *testing.T) {
	tg := GlobalDispatchTarget(fixtureRoots())
	if want := filepath.Join(fakeHome, ".codex", "AGENTS.md"); tg.Path != want {
		t.Errorf("dispatch path = %q, want %q", tg.Path, want)
	}
	if tg.Kind != install.KindManagedBlock {
		t.Errorf("dispatch kind = %v, want a managed block", tg.Kind)
	}
	if tg.BlockName != "dispatch" {
		t.Errorf("block name = %q, want dispatch", tg.BlockName)
	}
	if tg.Role != "dispatch" {
		t.Errorf("role = %q, want dispatch", tg.Role)
	}
	if tg.Content != nil {
		t.Errorf("historical target carries content: %q", tg.Content)
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

// TestCodexNestedDispatchBoundary — change 0365: every rendered Codex agent
// must carry the nested-dispatch boundary, because composition is a property
// of the invoked skill and may change through configuration; an allowlist of
// today's dispatching wrappers would silently miss a future binding. The
// expected clauses are LITERAL BEHAVIORAL EXPECTATIONS derived independently
// from the spec — deliberately NOT the renderer's own constant, so deleting
// the renderer paragraph reddens this test instead of both sides drifting
// together.
func TestCodexNestedDispatchBoundary(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("inventory is empty — universality would be vacuous")
	}

	// The spec's five composition families must be present in the inventory,
	// so "every source carries the paragraph" cannot be satisfied by a
	// shrunken agent set. This is a sampling floor, not the coverage boundary
	// — the loop below runs over ALL sources.
	byName := map[string]bool{}
	for _, s := range sources {
		byName[s.Name] = true
	}
	families := []string{
		"docket-implement-next",     // implement-next composition
		"docket-plan-writer",        // the pinned Step 4 dispatch that failed live
		"docket-build-standard",     // profile-routed build
		"docket-review-standard",    // rung-routed review
		"docket-auto-groom-critic",  // auto-groom critic
		"docket-rebase-resolver",    // finalize resolver
		"docket-integration-repair", // finalize repair
	}
	for _, want := range families {
		if !byName[want] {
			t.Errorf("composition-family agent %s missing from inventory", want)
		}
	}

	byPath := map[string][]byte{}
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile {
			byPath[tg.Path] = tg.Content
		}
	}

	// The three semantic clauses, as literal behavioral text.
	clauses := []string{
		"direct named-agent dispatch",           // (1) how to dispatch
		"active top-level tool surface",         // (1) from where
		"omit top-level collaboration controls", // (2) what nested inventories lack
		"cannot establish dispatch unavailability", // (3) what absence proves: nothing
	}
	for _, s := range sources {
		p := filepath.Join(fakeHome, ".codex", "agents", s.Name+".toml")
		content, ok := byPath[p]
		if !ok {
			t.Fatalf("no rendered agent file at %s", p)
		}
		for _, c := range clauses {
			if !strings.Contains(string(content), c) {
				t.Errorf("agent %s: rendered wrapper missing nested-dispatch clause %q", s.Name, c)
			}
		}
	}
}

func TestCodexPlanRejectsUnusableInput(t *testing.T) {
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

// Adding an agent source to the bundle must grow the plan by exactly one agent
// file with no adapter edit. Since change 0351 the plan carries no global
// dispatch surface, so the growth is purely the added agent wrapper. The claude
// adapter carries the same probe; each adapter needs its own because each
// derives its own destinations.
func TestCodexInventoryAdditionPropagates(t *testing.T) {
	in := fixtureInput(t)
	before := planFixture(t)

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

	wantPath := filepath.Join(fakeHome, rootDir, agentsDir, "docket-zzz-synthetic.toml")
	var found bool
	for _, tg := range after {
		if tg.Path == wantPath && tg.Kind == install.KindFile {
			found = true
		}
		if tg.Role == "dispatch" || tg.Kind == install.KindManagedBlock {
			t.Errorf("the grown plan carries a global dispatch surface at %q", tg.Path)
		}
	}
	if !found {
		t.Errorf("the grown plan carries no agent file at %s", wantPath)
	}
}
