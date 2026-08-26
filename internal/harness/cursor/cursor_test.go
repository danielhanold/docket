package cursor

import (
	"flag"
	"fmt"
	"os"
	"path"
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

const dispatchGoldenName = "docket-dispatch.mdc"

func dispatchTarget(t *testing.T) install.Target {
	t.Helper()
	want := filepath.Join(fakeHome, ".cursor", "rules", "docket-dispatch.mdc")
	for _, tg := range planFixture(t) {
		if tg.Path == want {
			return tg
		}
	}
	t.Fatalf("plan carries no dispatch rule at %s", want)
	return install.Target{}
}

// The dispatch rule is a dedicated file assembled from the authored payloads,
// verbatim: head, one fragment per agent source, then the run gate. Count
// equality is asserted BOTH ways against ParseInventory — a fragment with no
// agent source is as much a defect as an agent source with no fragment.
func TestCursorDispatchFileAssembled(t *testing.T) {
	tg := dispatchTarget(t)
	if tg.Kind != install.KindFile {
		t.Errorf("dispatch kind = %v, want a plain file", tg.Kind)
	}
	if tg.BlockName != "" || tg.Annotation != "" {
		t.Errorf("dispatch target carries managed-block fields: %q / %q", tg.BlockName, tg.Annotation)
	}
	content := string(tg.Content)
	checkGolden(t, dispatchGoldenName, tg.Content)

	in := fixtureInput(t)
	head, err := in.Assets.Bytes("cursor-rules/dispatch.head.md")
	if err != nil {
		t.Fatalf("head payload: %v", err)
	}
	if !strings.HasPrefix(content, string(head)) {
		t.Errorf("dispatch rule does not open with the authored head payload verbatim")
	}
	runGate, err := in.Assets.Bytes("cursor-rules/run-gate.md")
	if err != nil {
		t.Fatalf("run-gate payload: %v", err)
	}
	if !strings.HasSuffix(content, strings.TrimRight(string(runGate), "\n")+"\n") {
		t.Errorf("dispatch rule does not end with the run-gate payload verbatim")
	}

	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	fragments := map[string]bool{}
	for _, e := range in.Assets.EntriesByRole(assets.RoleDispatch) {
		if path.Dir(e.Path) == "cursor-rules/dispatch" {
			fragments[path.Base(e.Path)] = true
		}
	}
	if len(fragments) != len(sources) {
		t.Errorf("the bundle carries %d dispatch fragments for %d agent sources", len(fragments), len(sources))
	}
	for _, s := range sources {
		if !fragments[s.Name+".md"] {
			t.Errorf("agent source %s has no dispatch fragment", s.Name)
			continue
		}
		frag, err := in.Assets.Bytes("cursor-rules/dispatch/" + s.Name + ".md")
		if err != nil {
			t.Fatalf("fragment payload for %s: %v", s.Name, err)
		}
		if !strings.Contains(content, strings.TrimRight(string(frag), "\n")) {
			t.Errorf("dispatch rule does not carry %s's fragment verbatim", s.Name)
		}
		delete(fragments, s.Name+".md")
	}
	if len(fragments) != 0 {
		t.Errorf("dispatch fragments with no agent source: %v", fragments)
	}
}

// A bundle missing a dispatch payload is a corrupt bundle, never a rule that
// silently ships without its head, a fragment, or its gate.
func TestCursorPlanRequiresDispatchPayloads(t *testing.T) {
	for _, missing := range []string{
		"cursor-rules/dispatch.head.md",
		"cursor-rules/run-gate.md",
		"cursor-rules/dispatch/docket-alpha.md",
	} {
		t.Run(missing, func(t *testing.T) {
			base := syntheticCatalog()
			m := base.Manifest
			kept := m.Entries[:0:0]
			for _, e := range m.Entries {
				if e.Path != missing {
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
				t.Fatalf("Plan accepted a bundle missing %s", missing)
			}
		})
	}
}

// syntheticCatalog is a minimal well-formed bundle: one agent source with a
// matching fragment, plus the head and the gate.
func syntheticCatalog() assets.Catalog {
	payload := map[string][]byte{}
	m := assets.Manifest{FormatVersion: assets.ManifestFormatVersion, AssetProtocol: assets.AssetProtocol}
	add := func(p string, role assets.Role, body string) {
		payload[p] = []byte(body)
		m.Entries = append(m.Entries, assets.Entry{Path: p, Role: role, Mode: 0o644, Size: int64(len(body))})
	}
	add("agents/docket-alpha.md", assets.RoleAgentSource, "---\nname: docket-alpha\ndescription: Alpha.\n---\nBody.\n")
	add("cursor-rules/dispatch.head.md", assets.RoleDispatch, "head\n")
	add("cursor-rules/dispatch/docket-alpha.md", assets.RoleDispatch, "## docket-alpha\n")
	add("cursor-rules/run-gate.md", assets.RoleDispatch, "## Run gate\n\nRead git.\n")
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

// Adding an agent source to the bundle must grow the plan by one agent file and
// carry that agent's authored dispatch fragment into the rule, with no adapter
// edit — the inventory is the single roster, so a renderer-side name list would
// fail to grow here. Cursor's dispatch surface is assembled from authored
// per-agent fragments rather than a rendered roster, so the probe also proves
// the half a bundle can get wrong: an agent source shipped without its fragment
// is refused rather than silently un-dispatched.
func TestCursorInventoryAdditionPropagates(t *testing.T) {
	in := fixtureInput(t)
	before := planFixture(t)
	beforeRule := string(dispatchTarget(t).Content)

	const extraPath = "agents/docket-zzz-synthetic.md"
	const extraBody = "---\nname: docket-zzz-synthetic\ndescription: A synthetic seventeenth agent.\n---\nSynthetic body.\n"
	const fragPath = "cursor-rules/dispatch/docket-zzz-synthetic.md"
	const fragBody = "- **docket-zzz-synthetic** — A synthetic seventeenth agent. Delegate to the `docket-zzz-synthetic` agent.\n"

	base := in.Assets
	grow := func(extra ...assets.Entry) assets.Catalog {
		m := base.Manifest
		m.Entries = append(append([]assets.Entry(nil), m.Entries...), extra...)
		sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Path < m.Entries[j].Path })
		return assets.NewCatalog(m, func(p string) ([]byte, error) {
			switch p {
			case extraPath:
				return []byte(extraBody), nil
			case fragPath:
				return []byte(fragBody), nil
			}
			return base.Bytes(p)
		})
	}
	agentEntry := assets.Entry{Path: extraPath, Role: assets.RoleAgentSource, Mode: 0o644, Size: int64(len(extraBody))}
	fragEntry := assets.Entry{Path: fragPath, Role: assets.RoleDispatch, Mode: 0o644, Size: int64(len(fragBody))}

	t.Run("agent source without its fragment is refused", func(t *testing.T) {
		bad := in
		bad.Assets = grow(agentEntry)
		if _, err := New().Plan(bad); err == nil {
			t.Fatal("Plan assembled a rule for an agent with no dispatch fragment")
		}
	})

	in.Assets = grow(agentEntry, fragEntry)
	after, err := New().Plan(in)
	if err != nil {
		t.Fatalf("Plan over the grown catalog: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("plan grew from %d to %d targets, want exactly one more", len(before), len(after))
	}

	wantPath := filepath.Join(fakeHome, rootDir, agentsDir, "docket-zzz-synthetic.md")
	var found bool
	var afterRule string
	for _, tg := range after {
		if tg.Path == wantPath && tg.Kind == install.KindFile {
			found = true
		}
		if tg.Path == filepath.Join(fakeHome, rootDir, rulesDir, dispatchFile) {
			afterRule = string(tg.Content)
		}
	}
	if !found {
		t.Errorf("the grown plan carries no agent file at %s", wantPath)
	}
	if !strings.Contains(afterRule, strings.TrimRight(fragBody, "\n")) {
		t.Errorf("the dispatch rule does not carry the added agent's fragment")
	}
	if len(afterRule) <= len(beforeRule) {
		t.Errorf("the dispatch rule did not grow: %d bytes before, %d after", len(beforeRule), len(afterRule))
	}
}
