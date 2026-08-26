package cursor

import (
	"flag"
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
// the claude and codex adapters: a deliberate rendering change becomes a
// reviewable diff rather than a hand edit, and it is never set in CI.
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
		"cursor": {
			"build-standard": {
				Model:  config.Value[string]{Value: "gpt-5.5-cursor"},
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
		// A sibling harness's pins must never leak into the cursor rendering.
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

func TestCursorName(t *testing.T) {
	if got := New().Name(); got != "cursor" {
		t.Fatalf("Name = %q, want cursor", got)
	}
}

func TestCursorPlanDeterministic(t *testing.T) {
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

func TestCursorSkillLinks(t *testing.T) {
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
		want[filepath.Join(fakeHome, ".cursor", "skills", dir)] = filepath.Join(assetsDir, "skills", dir)
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

func TestCursorDetect(t *testing.T) {
	home := t.TempDir()
	roots := install.UserRoots{Home: home}

	d := New().Detect(roots)
	if d.Present {
		t.Errorf("Detect reported present with no ~/.cursor")
	}
	if d.Root != filepath.Join(home, ".cursor") {
		t.Errorf("Detect root = %q", d.Root)
	}

	// A regular file at the root is not a harness installation.
	if err := os.WriteFile(filepath.Join(home, ".cursor"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if New().Detect(roots).Present {
		t.Errorf("Detect reported present for a regular file at ~/.cursor")
	}
	if err := os.Remove(filepath.Join(home, ".cursor")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if err := os.Mkdir(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if !New().Detect(roots).Present {
		t.Errorf("Detect did not report present with ~/.cursor a directory")
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

// agentFiles is every rendered agent definition, keyed by basename. The
// dispatch rule is a KindFile target too, so it is excluded by path.
func agentFiles(t *testing.T) map[string]string {
	t.Helper()
	byName := map[string]string{}
	agentsRoot := filepath.Join(fakeHome, ".cursor", "agents")
	for _, tg := range planFixture(t) {
		if tg.Kind == install.KindFile && filepath.Dir(tg.Path) == agentsRoot {
			byName[filepath.Base(tg.Path)] = string(tg.Content)
		}
	}
	return byName
}

func TestCursorGoldenAgents(t *testing.T) {
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
	// redefine what "pinned" means. Cursor encodes effort INSIDE the model
	// value — there is no effort key at all.
	cases := []struct {
		file      string
		wantModel string // "" means the model line must be absent
	}{
		{"docket-build-standard.md", "gpt-5.5-cursor[effort=high]"},
		{"docket-status.md", "gpt-5.5"},
		{"docket-adr.md", ""},                   // inherit / auto sentinels
		{"docket-review-lean.md", ""},           // no table entry at all
		{"docket-brainstorm-consultant.md", ""}, // no table entry, no skills
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			content, ok := byName[tc.file]
			if !ok {
				t.Fatalf("no rendered file %s", tc.file)
			}
			if tc.wantModel == "" {
				if strings.Contains(content, "\nmodel: ") {
					t.Errorf("%s emits a model line for an unpinned agent:\n%s", tc.file, content)
				}
			} else if want := "\nmodel: " + tc.wantModel + "\n"; !strings.Contains(content, want) {
				t.Errorf("%s does not carry %q", tc.file, strings.TrimSpace(want))
			}
			if strings.Contains(content, "\neffort:") {
				t.Errorf("%s emits a bare effort key, which Cursor does not read", tc.file)
			}
			for _, key := range []string{"readonly:", "is_background:"} {
				if strings.Contains(content, "\n"+key) {
					t.Errorf("%s emits %s, which docket does not set", tc.file, key)
				}
			}
			// name: is a bare scalar — Cursor registers the Task subagent_type
			// enum token verbatim, quotes included, so a quoted name is
			// undispatchable by its own spelling (ADR-0060). description: stays
			// single-quoted free text (ADR-0071); verifyRoundTrip is what keeps
			// the bare form safe.
			name := strings.TrimSuffix(tc.file, ".md")
			if !strings.HasPrefix(content, "---\nname: "+name+"\n") {
				t.Errorf("%s does not open with a bare name scalar: %.60q", tc.file, content)
			}
			if !strings.Contains(content, "\ndescription: '") {
				t.Errorf("%s no longer single-quotes its description", tc.file)
			}
		})
	}
}

// The rendered document must carry the source's description, its skills
// preamble, and its body verbatim — the mapping that mirrors emit_cursor_md.
func TestCursorAgentMirrorsSource(t *testing.T) {
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
		preamble := "Before acting, load these docket skills from your Cursor skills directory: " +
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

// An effort with no model is dropped: Cursor encodes effort inside the model
// value, so there is nowhere to put it. The drop is silent — a rendering fact,
// not a stream warning.
func TestCursorEffortWithoutModelDropped(t *testing.T) {
	in := fixtureInput(t)
	in.Agents = config.AgentsTable{
		"cursor": {
			"status": {Effort: config.Value[string]{Value: "xhigh"}},
			// The `inherit` sentinel is a no-pin model, so its effort drops too.
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
	agentsRoot := filepath.Join(fakeHome, ".cursor", "agents")
	for _, tg := range targets {
		if tg.Kind != install.KindFile || filepath.Dir(tg.Path) != agentsRoot {
			continue
		}
		name := filepath.Base(tg.Path)
		if name != "docket-status.md" && name != "docket-adr.md" {
			continue
		}
		content := string(tg.Content)
		if strings.Contains(content, "\nmodel: ") {
			t.Errorf("%s emits a model line for an effort-only pin:\n%s", name, content)
		}
		for _, token := range []string{"effort", "xhigh", "high"} {
			if strings.Contains(frontmatterOf(content), token) {
				t.Errorf("%s leaks the dropped effort (%q) into its frontmatter:\n%s", name, token, content)
			}
		}
	}
}

// frontmatterOf returns the text between the two fences, so an effort assert
// cannot be satisfied — or defeated — by the agent's prose body.
func frontmatterOf(content string) string {
	rest, ok := strings.CutPrefix(content, "---\n")
	if !ok {
		return ""
	}
	fm, _, _ := strings.Cut(rest, "\n---\n")
	return fm
}

// Change 0351 retired the user-global dispatch surface: Plan no longer emits the
// ~/.cursor/rules/docket-dispatch.mdc rule. Parent-facing routing now lives in a
// repository's own .cursor/rules, never a personal global one.
func TestCursorPlanHasNoGlobalDispatch(t *testing.T) {
	globalRule := filepath.Join(fakeHome, ".cursor", "rules", "docket-dispatch.mdc")
	for _, tg := range planFixture(t) {
		if tg.Role == "dispatch" {
			t.Errorf("plan still carries a dispatch-role target at %q", tg.Path)
		}
		if tg.Path == globalRule {
			t.Errorf("plan still carries the global dispatch rule at %q", tg.Path)
		}
	}
}

// GlobalDispatchTarget still names the historical destination so the installer
// can retire a leftover a prior install owns. Cursor's rule is a whole file
// docket owns — a KindFile, not a managed block. Location and identity only, no
// Content.
func TestCursorGlobalDispatchTarget(t *testing.T) {
	tg := GlobalDispatchTarget(fixtureRoots())
	if want := filepath.Join(fakeHome, ".cursor", "rules", "docket-dispatch.mdc"); tg.Path != want {
		t.Errorf("dispatch path = %q, want %q", tg.Path, want)
	}
	if tg.Kind != install.KindFile {
		t.Errorf("dispatch kind = %v, want a plain file", tg.Kind)
	}
	if tg.BlockName != "" || tg.Annotation != "" {
		t.Errorf("historical target carries managed-block fields: %q / %q", tg.BlockName, tg.Annotation)
	}
	if tg.Role != "dispatch" {
		t.Errorf("role = %q, want dispatch", tg.Role)
	}
	if tg.Content != nil {
		t.Errorf("historical target carries content: %q", tg.Content)
	}
}

func TestCursorPlanRejectsUnusableInput(t *testing.T) {
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
// dispatch rule, so the growth is purely the added agent wrapper.
func TestCursorInventoryAdditionPropagates(t *testing.T) {
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
	in.Assets = assets.NewCatalog(m, func(p string) ([]byte, error) {
		if p == extraPath {
			return []byte(extraBody), nil
		}
		return base.Bytes(p)
	})

	after, err := New().Plan(in)
	if err != nil {
		t.Fatalf("Plan over the grown catalog: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("plan grew from %d to %d targets, want exactly one more", len(before), len(after))
	}

	wantPath := filepath.Join(fakeHome, rootDir, agentsDir, "docket-zzz-synthetic.md")
	var found bool
	globalRule := filepath.Join(fakeHome, rootDir, rulesDir, dispatchFile)
	for _, tg := range after {
		if tg.Path == wantPath && tg.Kind == install.KindFile {
			found = true
		}
		if tg.Role == "dispatch" || tg.Path == globalRule {
			t.Errorf("the grown plan carries a global dispatch surface at %q", tg.Path)
		}
	}
	if !found {
		t.Errorf("the grown plan carries no agent file at %s", wantPath)
	}
}
