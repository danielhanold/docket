package assets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// authoredRepoRoot returns the repo root holding the authored asset roots, or
// skips: the correspondence tests are meaningful only inside this repo, and a
// future extraction of the package must not silently redden.
func authoredRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	for _, r := range DefaultAllowedRoots() {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(r.Root))); err != nil {
			t.Skipf("authored root %s is not present at %s — skipping the correspondence check", r.Root, root)
		}
	}
	return root
}

// TestEmbeddedMatchesAuthored is the drift guard: the committed bundle must be
// exactly what the generator produces from the authored roots right now, in
// both directions.
func TestEmbeddedMatchesAuthored(t *testing.T) {
	root := authoredRepoRoot(t)

	generated, payload, err := Generate(root, DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("Generate from %s: %v", root, err)
	}
	embedded, err := EmbeddedManifest()
	if err != nil {
		t.Fatalf("EmbeddedManifest: %v", err)
	}

	genBytes, err := EncodeCanonical(generated)
	if err != nil {
		t.Fatalf("encode generated: %v", err)
	}
	embBytes, err := EncodeCanonical(embedded)
	if err != nil {
		t.Fatalf("encode embedded: %v", err)
	}
	if string(genBytes) != string(embBytes) {
		t.Errorf("embedded manifest is stale — run `go generate ./internal/assets/`\ngenerated asset_set_id %s\nembedded  asset_set_id %s",
			generated.AssetSetID, embedded.AssetSetID)
	}

	// Direction 1: every generated path is embedded, byte for byte.
	embeddedPaths := map[string]bool{}
	for _, e := range embedded.Entries {
		embeddedPaths[e.Path] = true
	}
	for _, e := range generated.Entries {
		if !embeddedPaths[e.Path] {
			t.Errorf("authored path %s is missing from the embedded bundle", e.Path)
			continue
		}
		got, err := Open(e.Path)
		if err != nil {
			t.Errorf("Open(%s): %v", e.Path, err)
			continue
		}
		if string(got) != string(payload[e.Path]) {
			t.Errorf("embedded bytes for %s differ from the authored file", e.Path)
		}
	}

	// Direction 2: every embedded path still exists in the authored roots.
	for _, e := range embedded.Entries {
		if _, ok := payload[e.Path]; !ok {
			t.Errorf("embedded path %s is no longer produced from the authored roots", e.Path)
		}
	}
}

func TestEmbeddedValidates(t *testing.T) {
	m, err := EmbeddedManifest()
	if err != nil {
		t.Fatalf("EmbeddedManifest: %v", err)
	}
	if len(m.Entries) == 0 {
		t.Fatal("embedded manifest carries no entries")
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatalf("embedded manifest does not validate: %v", err)
	}
	if err := VerifyPayloads(m, Open); err != nil {
		t.Fatalf("embedded payloads do not verify: %v", err)
	}
	id, err := ComputeAssetSetID(m)
	if err != nil {
		t.Fatalf("ComputeAssetSetID: %v", err)
	}
	if id != m.AssetSetID {
		t.Errorf("asset_set_id %s does not recompute (%s)", m.AssetSetID, id)
	}
	if m.AssetProtocol != AssetProtocol {
		t.Errorf("embedded asset_protocol %d, binary speaks %d", m.AssetProtocol, AssetProtocol)
	}
}

// TestEmbeddedCarriesDotfiles pins the `all:` embed prefix: a bare directory
// pattern drops every dot-prefixed name, which would strip .docket.example.yml
// from the tree while the manifest still names it.
func TestEmbeddedCarriesDotfiles(t *testing.T) {
	m, err := EmbeddedManifest()
	if err != nil {
		t.Fatalf("EmbeddedManifest: %v", err)
	}
	var dotted []string
	for _, e := range m.Entries {
		if len(e.Path) > 0 && e.Path[0] == '.' {
			dotted = append(dotted, e.Path)
		}
	}
	if len(dotted) == 0 {
		t.Skip("the bundle names no dot-prefixed path")
	}
	for _, p := range dotted {
		if _, err := Open(p); err != nil {
			t.Errorf("Open(%s): %v — the embed pattern is dropping dotfiles", p, err)
		}
	}
}

func TestEmbeddedCorruptionDetected(t *testing.T) {
	base, err := EmbeddedManifest()
	if err != nil {
		t.Fatalf("EmbeddedManifest: %v", err)
	}

	t.Run("flipped hash", func(t *testing.T) {
		m := base
		m.Entries = append([]Entry(nil), base.Entries...)
		flipped := m.Entries[0].SHA256
		if flipped[0] == 'a' {
			flipped = "b" + flipped[1:]
		} else {
			flipped = "a" + flipped[1:]
		}
		m.Entries[0].SHA256 = flipped
		if err := VerifyPayloads(m, Open); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("want ErrManifestInvalid for a flipped digest, got %v", err)
		}
	})

	t.Run("wrong size", func(t *testing.T) {
		m := base
		m.Entries = append([]Entry(nil), base.Entries...)
		m.Entries[0].Size++
		if err := VerifyPayloads(m, Open); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("want ErrManifestInvalid for a wrong size, got %v", err)
		}
	})

	t.Run("missing payload", func(t *testing.T) {
		m := base
		m.Entries = append([]Entry(nil), base.Entries...)
		m.Entries[0].Path = "skills/does-not-exist/SKILL.md"
		if err := VerifyPayloads(m, Open); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("want ErrManifestInvalid for a missing payload, got %v", err)
		}
	})
}

func TestOpenRejectsUnsafePath(t *testing.T) {
	for _, p := range []string{"", "/etc/passwd", "../manifest.json", "skills/../../x"} {
		if _, err := Open(p); !errors.Is(err, ErrManifestInvalid) {
			t.Errorf("Open(%q): want ErrManifestInvalid, got %v", p, err)
		}
	}
}

func TestEmbeddedManifestCopyIsNotAliased(t *testing.T) {
	first, err := EmbeddedManifest()
	if err != nil {
		t.Fatalf("EmbeddedManifest: %v", err)
	}
	first.Entries[0].SHA256 = "tampered"
	second, err := EmbeddedManifest()
	if err != nil {
		t.Fatalf("EmbeddedManifest (second): %v", err)
	}
	if second.Entries[0].SHA256 == "tampered" {
		t.Fatal("EmbeddedManifest hands out the cached entry slice — a caller can corrupt it")
	}
}

func TestEmbeddedCatalog(t *testing.T) {
	c, err := EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}

	roles := map[Role]int{}
	for _, e := range c.Manifest.Entries {
		roles[e.Role]++
	}
	for _, r := range []Role{RoleSkill, RoleAgentSource, RoleHarnessDefaults, RoleDispatch, RoleConfigSchema} {
		byRole := c.EntriesByRole(r)
		if len(byRole) != roles[r] {
			t.Errorf("EntriesByRole(%s) returned %d entries, manifest carries %d", r, len(byRole), roles[r])
		}
		if len(byRole) == 0 {
			t.Errorf("no entry carries role %s", r)
		}
		for _, e := range byRole {
			if e.Role != r {
				t.Errorf("EntriesByRole(%s) returned an entry with role %s", r, e.Role)
			}
		}
	}

	if got := c.EntriesByRole(RoleHarnessDefaults); len(got) != 1 || got[0].Path != "agents/harness-defaults.yml" {
		t.Errorf("harness-defaults role entries = %v, want exactly agents/harness-defaults.yml", got)
	}

	first := c.Manifest.Entries[0]
	body, err := c.Bytes(first.Path)
	if err != nil {
		t.Fatalf("Bytes(%s): %v", first.Path, err)
	}
	if int64(len(body)) != first.Size {
		t.Errorf("Bytes(%s) returned %d bytes, manifest says %d", first.Path, len(body), first.Size)
	}
	if _, err := (Catalog{}).Bytes(first.Path); !errors.Is(err, ErrManifestInvalid) {
		t.Error("a zero Catalog should refuse to serve payloads")
	}
}

func TestDecodeManifestRejectsGarbage(t *testing.T) {
	cases := map[string]string{
		"not json":       "{",
		"unknown field":  `{"format_version":1,"asset_protocol":1,"asset_set_id":"","entries":[],"extra":1}`,
		"trailing value": `{"format_version":1,"asset_protocol":1,"asset_set_id":"","entries":[]} {}`,
	}
	for name, raw := range cases {
		if _, err := DecodeManifest([]byte(raw)); !errors.Is(err, ErrManifestInvalid) {
			t.Errorf("%s: want ErrManifestInvalid, got %v", name, err)
		}
	}
}
