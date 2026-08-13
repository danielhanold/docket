package harness

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/install"
)

// The floors below exist so an emptied or truncated bundle cannot satisfy the
// derived asserts vacuously: every other assertion in this file compares the
// parsed inventory against the catalog it came from, which stays true when the
// catalog is empty.
const (
	agentSourceFloor = 16
	skillDirFloor    = 12
)

func embeddedCatalog(t *testing.T) assets.Catalog {
	t.Helper()
	c, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	return c
}

// syntheticCatalog builds a catalog over hand-written payloads: it is the seam
// for the malformed-source cases the real bundle can never exhibit.
func syntheticCatalog(files map[string]string, role assets.Role) assets.Catalog {
	m := assets.Manifest{FormatVersion: assets.ManifestFormatVersion, AssetProtocol: assets.AssetProtocol}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		m.Entries = append(m.Entries, assets.Entry{Path: p, Role: role, Mode: 0o644, Size: int64(len(files[p]))})
	}
	return assets.NewCatalog(m, func(p string) ([]byte, error) {
		body, ok := files[p]
		if !ok {
			return nil, fmt.Errorf("no such payload %q", p)
		}
		return []byte(body), nil
	})
}

func TestOrderFixed(t *testing.T) {
	want := []string{"claude", "codex", "cursor", "opencode"}
	if !reflect.DeepEqual(Order, want) {
		t.Fatalf("Order = %v, want %v", Order, want)
	}
}

func TestParseInventoryFromEmbedded(t *testing.T) {
	c := embeddedCatalog(t)
	sources, err := ParseInventory(c)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}

	entries := c.EntriesByRole(assets.RoleAgentSource)
	if len(sources) != len(entries) {
		t.Fatalf("parsed %d sources from %d agent-source entries", len(sources), len(entries))
	}
	if len(sources) < agentSourceFloor {
		t.Fatalf("parsed %d sources, want at least %d", len(sources), agentSourceFloor)
	}

	seen := map[string]bool{}
	for i, s := range sources {
		if i > 0 && !(sources[i-1].ShortName < s.ShortName) {
			t.Errorf("sources not strictly sorted by ShortName at %d: %q then %q", i, sources[i-1].ShortName, s.ShortName)
		}
		if seen[s.ShortName] {
			t.Errorf("duplicate short name %q", s.ShortName)
		}
		seen[s.ShortName] = true
		if !strings.HasPrefix(s.Name, "docket-") {
			t.Errorf("%s: name %q lacks the docket- prefix", s.ShortName, s.Name)
		}
		if s.Name != "docket-"+s.ShortName {
			t.Errorf("short name %q does not match name %q", s.ShortName, s.Name)
		}
		if strings.TrimSpace(s.Description) == "" {
			t.Errorf("%s: empty description", s.ShortName)
		}
		if strings.TrimSpace(s.Body) == "" {
			t.Errorf("%s: empty body", s.ShortName)
		}
		if strings.HasPrefix(s.Body, "---") {
			t.Errorf("%s: body still carries the frontmatter fence", s.ShortName)
		}
		if strings.Contains(s.Body, "description: "+s.Description) {
			t.Errorf("%s: body still carries the frontmatter description", s.ShortName)
		}
	}

	byShort := map[string]AgentSource{}
	for _, s := range sources {
		byShort[s.ShortName] = s
	}

	bs, ok := byShort["build-standard"]
	if !ok {
		t.Fatalf("build-standard missing from the inventory")
	}
	if !reflect.DeepEqual(bs.Skills, []string{"docket-build-task"}) {
		t.Errorf("build-standard skills = %v, want [docket-build-task]", bs.Skills)
	}
	if !strings.Contains(bs.Body, "STANDARD profile") {
		t.Errorf("build-standard body does not read like the authored body: %.80q", bs.Body)
	}

	bc, ok := byShort["brainstorm-consultant"]
	if !ok {
		t.Fatalf("brainstorm-consultant missing from the inventory")
	}
	if len(bc.Skills) != 0 {
		t.Errorf("brainstorm-consultant skills = %v, want none", bc.Skills)
	}
}

func TestParseInventoryDeterministic(t *testing.T) {
	c := embeddedCatalog(t)
	first, err := ParseInventory(c)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	second, err := ParseInventory(c)
	if err != nil {
		t.Fatalf("ParseInventory (second): %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("ParseInventory is not deterministic")
	}
}

func TestParseInventoryRejects(t *testing.T) {
	const good = "---\nname: docket-alpha\ndescription: Alpha agent.\nskills: [docket-build-task]\n---\nBody.\n"

	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"no frontmatter", map[string]string{"agents/docket-alpha.md": "Body only.\n"}, "frontmatter"},
		{"invalid yaml", map[string]string{"agents/docket-alpha.md": "---\nname: [unclosed\n---\nBody.\n"}, "agents/docket-alpha.md"},
		{"missing name", map[string]string{"agents/docket-alpha.md": "---\ndescription: Alpha.\n---\nBody.\n"}, "name"},
		{"missing description", map[string]string{"agents/docket-alpha.md": "---\nname: docket-alpha\n---\nBody.\n"}, "description"},
		{"name lacks prefix", map[string]string{"agents/alpha.md": "---\nname: alpha\ndescription: Alpha.\n---\nBody.\n"}, "docket-"},
		{"name disagrees with filename", map[string]string{"agents/docket-beta.md": good}, "docket-beta.md"},
		{"duplicate short name", map[string]string{
			"agents/docket-alpha.md":   good,
			"agents/x/docket-alpha.md": good,
		}, "duplicate"},
		{"unreadable payload", map[string]string{"agents/docket-alpha.md": good}, "no such payload"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := syntheticCatalog(tc.files, assets.RoleAgentSource)
			if tc.name == "unreadable payload" {
				// Keep the manifest, drop the payload accessor's content.
				c = assets.NewCatalog(c.Manifest, func(p string) ([]byte, error) {
					return nil, fmt.Errorf("no such payload %q", p)
				})
			}
			_, err := ParseInventory(c)
			if err == nil {
				t.Fatalf("ParseInventory accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestParseInventoryAcceptsSynthetic(t *testing.T) {
	c := syntheticCatalog(map[string]string{
		"agents/docket-alpha.md": "---\nname: docket-alpha\ndescription: \"Alpha: does things.\"\nskills: [docket-build-task, docket-convention]\n---\nFirst line.\n\nSecond line.\n",
		"agents/docket-zeta.md":  "---\nname: docket-zeta\ndescription: Zeta.\n---\nZeta body.\n",
	}, assets.RoleAgentSource)

	got, err := ParseInventory(c)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}
	want := []AgentSource{
		{ShortName: "alpha", Name: "docket-alpha", Description: "Alpha: does things.",
			Skills: []string{"docket-build-task", "docket-convention"},
			Body:   "First line.\n\nSecond line.\n"},
		{ShortName: "zeta", Name: "docket-zeta", Description: "Zeta.", Body: "Zeta body.\n"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseInventory = %#v, want %#v", got, want)
	}
}

func TestSkillDirsFromEmbedded(t *testing.T) {
	c := embeddedCatalog(t)
	got := SkillDirs(c)

	// Derive the expectation from the catalog by an independent route so the
	// assert tracks the bundle instead of restating the implementation.
	want := map[string]bool{}
	for _, e := range c.EntriesByRole(assets.RoleSkill) {
		parts := strings.Split(path.Clean(e.Path), "/")
		if len(parts) < 2 {
			t.Fatalf("skill entry %q has no top-level directory", e.Path)
		}
		want[parts[1]] = true
	}
	if len(got) != len(want) {
		t.Fatalf("SkillDirs returned %d dirs, want %d", len(got), len(want))
	}
	if len(got) < skillDirFloor {
		t.Fatalf("SkillDirs returned %d dirs, want at least %d", len(got), skillDirFloor)
	}
	for i, d := range got {
		if !want[d] {
			t.Errorf("SkillDirs returned %q, which no skill entry names", d)
		}
		if i > 0 && !(got[i-1] < d) {
			t.Errorf("SkillDirs not strictly sorted at %d: %q then %q", i, got[i-1], d)
		}
	}
	for _, known := range []string{"docket-build", "docket-build-task", "docket-convention"} {
		if !want[known] {
			t.Fatalf("bundle no longer carries the %q skill; the fixture assumption is stale", known)
		}
	}
}

func TestSkillDirsIgnoresRootLevelEntries(t *testing.T) {
	c := syntheticCatalog(map[string]string{
		"skills/beta/SKILL.md":            "x",
		"skills/alpha/SKILL.md":           "x",
		"skills/alpha/references/note.md": "x",
		"skills/README.md":                "x",
	}, assets.RoleSkill)
	got := SkillDirs(c)
	if !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("SkillDirs = %v, want [alpha beta]", got)
	}
}

func TestResolvedAgentSentinels(t *testing.T) {
	table := config.AgentsTable{
		"claude": {
			"build-standard": {Model: config.Value[string]{Value: "claude-opus-5"}, Effort: config.Value[string]{Value: "low"}},
			"status":         {Model: config.Value[string]{Value: "inherit"}, Effort: config.Value[string]{Value: "high"}},
			"adr":            {Model: config.Value[string]{Value: "claude-opus-5"}, Effort: config.Value[string]{Value: ""}},
			"review-deep":    {Model: config.Value[string]{Value: "claude-opus-5"}, Effort: config.Value[string]{Value: "auto"}},
		},
	}

	cases := []struct {
		name                string
		harness, short      string
		wantModel, wantEfrt string
	}{
		{"pinned passthrough", "claude", "build-standard", "claude-opus-5", "low"},
		{"model inherit suppressed", "claude", "status", "", "high"},
		{"effort already suppressed", "claude", "adr", "claude-opus-5", ""},
		{"effort auto suppressed", "claude", "review-deep", "claude-opus-5", ""},
		{"unknown agent", "claude", "nope", "", ""},
		{"unknown harness", "nope", "build-standard", "", ""},
		{"nil table", "claude", "build-standard", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := table
			if tc.name == "nil table" {
				in = nil
			}
			model, effort := ResolvedAgent(in, tc.harness, tc.short)
			if model != tc.wantModel || effort != tc.wantEfrt {
				t.Fatalf("ResolvedAgent = (%q, %q), want (%q, %q)", model, effort, tc.wantModel, tc.wantEfrt)
			}
		})
	}
}

// ResolvedAgentRaw keeps the model exactly as configured — `inherit` included,
// because Claude Code reads it as a real value — while still folding the
// effort `auto` sentinel away, which every harness treats as "no pin".
func TestResolvedAgentRawKeepsInherit(t *testing.T) {
	table := config.AgentsTable{
		"claude": {
			"status":      {Model: config.Value[string]{Value: "inherit"}, Effort: config.Value[string]{Value: "high"}},
			"adr":         {Model: config.Value[string]{Value: "inherit"}, Effort: config.Value[string]{Value: "auto"}},
			"review-deep": {Model: config.Value[string]{Value: "claude-opus-5"}, Effort: config.Value[string]{Value: "low"}},
		},
	}

	cases := []struct {
		name                string
		harness, short      string
		wantModel, wantEfrt string
	}{
		{"inherit kept, effort kept", "claude", "status", "inherit", "high"},
		{"inherit kept, effort auto dropped", "claude", "adr", "inherit", ""},
		{"ordinary pin passthrough", "claude", "review-deep", "claude-opus-5", "low"},
		{"unknown agent", "claude", "nope", "", ""},
		{"unknown harness", "nope", "status", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, effort := ResolvedAgentRaw(table, tc.harness, tc.short)
			if model != tc.wantModel || effort != tc.wantEfrt {
				t.Fatalf("ResolvedAgentRaw = (%q, %q), want (%q, %q)", model, effort, tc.wantModel, tc.wantEfrt)
			}
			// The normalizing resolver differs on exactly one field.
			nm, ne := ResolvedAgent(table, tc.harness, tc.short)
			want := model
			if want == "inherit" {
				want = ""
			}
			if nm != want {
				t.Errorf("ResolvedAgent model = %q, want %q", nm, want)
			}
			if ne != effort {
				t.Errorf("ResolvedAgent effort = %q, want %q", ne, effort)
			}
		})
	}
}

// stubAdapter proves the Adapter interface is satisfiable with the exact
// signatures the four adapter packages will implement.
type stubAdapter struct{ name string }

func (s stubAdapter) Name() string { return s.name }
func (s stubAdapter) Detect(r install.UserRoots) Detection {
	return Detection{Present: r.Home != "", Root: r.Home}
}
func (s stubAdapter) Plan(in PlanInput) ([]install.Target, error) {
	return []install.Target{{Path: in.AssetsDir, Kind: install.KindFile, Content: []byte("x"), Role: "skill"}}, nil
}

func TestAdapterInterfaceShape(t *testing.T) {
	var a Adapter = stubAdapter{name: "claude"}
	if a.Name() != "claude" {
		t.Fatalf("Name = %q", a.Name())
	}
	if d := a.Detect(install.UserRoots{Home: "/home/u"}); !d.Present || d.Root != "/home/u" {
		t.Fatalf("Detect = %+v", d)
	}
	in := PlanInput{
		Assets:    embeddedCatalog(t),
		Mode:      ModeRelease,
		AssetsDir: "/data/versions/sha256-x/assets",
		Roots:     install.UserRoots{Home: "/home/u", ConfigHome: "/home/u/.config"},
		Agents:    config.AgentsTable{},
	}
	targets, err := a.Plan(in)
	if err != nil || len(targets) != 1 {
		t.Fatalf("Plan = %v, %v", targets, err)
	}
	if ModeRelease != "release" || ModeDevelopment != "development" {
		t.Fatalf("install mode constants drifted: %q %q", ModeRelease, ModeDevelopment)
	}
}
