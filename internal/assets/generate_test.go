package assets

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fixtureRepo builds a miniature repo with one file under each allowed root and
// returns its directory.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("skills/docket-demo/SKILL.md", "# demo skill\n")
	write("skills/docket-demo/references/notes.md", "notes\n")
	write("agents/docket-demo.md", "---\nname: docket-demo\n---\nbody\n")
	write("agents/harness-defaults.yml", "claude: {}\n")
	write("cursor-rules/dispatch.head.md", "head\n")
	write(".docket.example.yml", "version: 1\n")
	return dir
}

func TestGenerateDeterministic(t *testing.T) {
	dir := fixtureRepo(t)

	m1, p1, err := Generate(dir, DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}
	m2, p2, err := Generate(dir, DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate (second): %v", err)
	}

	if err := ValidateManifest(m1); err != nil {
		t.Fatalf("generated manifest does not validate: %v", err)
	}
	if m1.AssetSetID == "" {
		t.Fatal("generated manifest has an empty asset_set_id")
	}

	b1, err := EncodeCanonical(m1)
	if err != nil {
		t.Fatalf("encode first: %v", err)
	}
	b2, err := EncodeCanonical(m2)
	if err != nil {
		t.Fatalf("encode second: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("two Generate calls produced different manifests:\n%s\n---\n%s", b1, b2)
	}
	if !reflect.DeepEqual(p1, p2) {
		t.Error("two Generate calls produced different payload maps")
	}

	// Every entry has exactly one payload and vice versa.
	if len(p1) != len(m1.Entries) {
		t.Fatalf("payload count %d != entry count %d", len(p1), len(m1.Entries))
	}
	for _, e := range m1.Entries {
		body, ok := p1[e.Path]
		if !ok {
			t.Fatalf("entry %s has no payload", e.Path)
		}
		if int64(len(body)) != e.Size {
			t.Errorf("entry %s: size %d, payload %d bytes", e.Path, e.Size, len(body))
		}
	}
}

func TestGenerateRejectsSymlink(t *testing.T) {
	dir := fixtureRepo(t)
	if err := os.Symlink(filepath.Join(dir, "agents", "docket-demo.md"),
		filepath.Join(dir, "skills", "docket-demo", "link.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, _, err := Generate(dir, DefaultAllowedRoots()); !errors.Is(err, ErrGenerate) {
		t.Fatalf("want ErrGenerate for a symlink under a root, got %v", err)
	}
}

func TestGenerateRejectsSymlinkedDirectory(t *testing.T) {
	dir := fixtureRepo(t)
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "skills", "docket-demo", "escape")); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}

	if _, _, err := Generate(dir, DefaultAllowedRoots()); !errors.Is(err, ErrGenerate) {
		t.Fatalf("want ErrGenerate for a symlinked directory under a root, got %v", err)
	}
}

func TestGenerateRejectsEscape(t *testing.T) {
	dir := fixtureRepo(t)
	cases := map[string][]AllowedRoot{
		"parent":   {{Root: "../outside", Role: RoleSkill}},
		"absolute": {{Root: "/etc", Role: RoleSkill}},
		"dot":      {{Root: "./skills/../skills", Role: RoleSkill}},
		"empty":    {{Root: "", Role: RoleSkill}},
	}
	for name, roots := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Generate(dir, roots); !errors.Is(err, ErrGenerate) {
				t.Fatalf("want ErrGenerate for root %q, got %v", roots[0].Root, err)
			}
		})
	}
}

func TestGenerateRejectsCollision(t *testing.T) {
	dir := fixtureRepo(t)
	roots := []AllowedRoot{
		{Root: "agents", Role: RoleAgentSource},
		{Root: "agents/docket-demo.md", Role: RoleConfigSchema},
	}
	if _, _, err := Generate(dir, roots); !errors.Is(err, ErrGenerate) {
		t.Fatalf("want ErrGenerate for two roots mapping the same path, got %v", err)
	}
}

func TestGenerateRejectsMissingRoot(t *testing.T) {
	dir := fixtureRepo(t)
	roots := []AllowedRoot{{Root: "nope", Role: RoleSkill}}
	if _, _, err := Generate(dir, roots); !errors.Is(err, ErrGenerate) {
		t.Fatalf("want ErrGenerate for an absent root, got %v", err)
	}
}

func TestGenerateRolesAssigned(t *testing.T) {
	dir := fixtureRepo(t)
	m, _, err := Generate(dir, DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	byPath := map[string]Entry{}
	for _, e := range m.Entries {
		byPath[e.Path] = e
	}
	want := map[string]Role{
		"skills/docket-demo/SKILL.md":            RoleSkill,
		"skills/docket-demo/references/notes.md": RoleSkill,
		"agents/docket-demo.md":                  RoleAgentSource,
		"agents/harness-defaults.yml":            RoleHarnessDefaults,
		"cursor-rules/dispatch.head.md":          RoleDispatch,
		".docket.example.yml":                    RoleConfigSchema,
	}
	if len(byPath) != len(want) {
		t.Fatalf("generated %d entries, want %d: %v", len(byPath), len(want), byPath)
	}
	for path, role := range want {
		e, ok := byPath[path]
		if !ok {
			t.Errorf("missing entry %s", path)
			continue
		}
		if e.Role != role {
			t.Errorf("entry %s: role %q, want %q", path, e.Role, role)
		}
		if e.Mode != 0o644 {
			t.Errorf("entry %s: mode %#o, want 0644", path, e.Mode)
		}
	}
}

func TestWriteTreeRoundTrip(t *testing.T) {
	dir := fixtureRepo(t)
	m, payload, err := Generate(dir, DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := filepath.Join(t.TempDir(), "embedded")
	if err := WriteTree(out, m, payload); err != nil {
		t.Fatalf("WriteTree: %v", err)
	}

	encoded, err := EncodeCanonical(m)
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	onDisk, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if string(onDisk) != string(encoded) {
		t.Error("written manifest.json is not the canonical encoding")
	}
	for path, body := range payload {
		got, err := os.ReadFile(filepath.Join(out, "tree", filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read tree/%s: %v", path, err)
		}
		if string(got) != string(body) {
			t.Errorf("tree/%s bytes differ from the payload", path)
		}
	}
}

func TestWriteTreeRejectsPayloadMismatch(t *testing.T) {
	dir := fixtureRepo(t)
	m, payload, err := Generate(dir, DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	payload["skills/docket-demo/SKILL.md"] = []byte("tampered\n")
	out := filepath.Join(t.TempDir(), "embedded")
	if err := WriteTree(out, m, payload); !errors.Is(err, ErrGenerate) {
		t.Fatalf("want ErrGenerate when a payload does not match its entry, got %v", err)
	}
}

func TestWriteTreeRejectsNonEmptyDir(t *testing.T) {
	dir := fixtureRepo(t)
	m, payload, err := Generate(dir, DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := filepath.Join(t.TempDir(), "embedded")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "stale.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	if err := WriteTree(out, m, payload); !errors.Is(err, ErrGenerate) {
		t.Fatalf("want ErrGenerate when the output dir is not fresh, got %v", err)
	}
}

func TestDefaultAllowedRootsExact(t *testing.T) {
	want := []AllowedRoot{
		{Root: "skills", Role: RoleSkill},
		{Root: "agents", Role: RoleAgentSource},
		{Root: "cursor-rules", Role: RoleDispatch},
		{Root: ".docket.example.yml", Role: RoleConfigSchema},
	}
	if got := DefaultAllowedRoots(); !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultAllowedRoots() = %v, want %v", got, want)
	}
}
