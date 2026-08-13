package assets

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// hex64 builds a 64-char lowercase hex digest string from a repeated nibble.
func hex64(c byte) string {
	return strings.Repeat(string(c), 64)
}

func validManifest(t *testing.T) Manifest {
	t.Helper()
	m := Manifest{
		FormatVersion: ManifestFormatVersion,
		AssetProtocol: AssetProtocol,
		Entries: []Entry{
			{Path: ".docket.example.yml", Role: RoleConfigSchema, Mode: 0o644, Size: 12, SHA256: hex64('a')},
			{Path: "skills/docket-adr/SKILL.md", Role: RoleSkill, Mode: 0o644, Size: 34, SHA256: hex64('b')},
		},
	}
	id, err := ComputeAssetSetID(m)
	if err != nil {
		t.Fatalf("ComputeAssetSetID: %v", err)
	}
	m.AssetSetID = id
	return m
}

func TestEncodeCanonicalDeterministic(t *testing.T) {
	m := validManifest(t)

	first, err := EncodeCanonical(m)
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	second, err := EncodeCanonical(m)
	if err != nil {
		t.Fatalf("EncodeCanonical (second): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("encoding not byte-stable:\n%s\n---\n%s", first, second)
	}
	if !bytes.HasSuffix(first, []byte("\n")) {
		t.Fatalf("canonical encoding must end with a trailing newline, got %q", first)
	}
	if !bytes.Contains(first, []byte("\n  \"format_version\": 1,")) {
		t.Fatalf("expected two-space indent and struct key order, got:\n%s", first)
	}
	// Struct field order, not alphabetical order.
	if idxFV, idxAP := bytes.Index(first, []byte(`"format_version"`)), bytes.Index(first, []byte(`"asset_protocol"`)); idxFV > idxAP {
		t.Fatalf("keys not in struct order:\n%s", first)
	}

	// A pre-shuffled manifest is rejected by validation, and its sorted
	// counterpart validates and encodes identically to the sorted original.
	shuffled := m
	shuffled.Entries = []Entry{m.Entries[1], m.Entries[0]}
	if err := ValidateManifest(shuffled); err == nil {
		t.Fatalf("ValidateManifest accepted unsorted entries")
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatalf("ValidateManifest rejected the sorted manifest: %v", err)
	}
}

func TestComputeAssetSetIDExcludesSelf(t *testing.T) {
	a := validManifest(t)
	b := a
	b.AssetSetID = "sha256:" + hex64('c')

	idA, err := ComputeAssetSetID(a)
	if err != nil {
		t.Fatalf("ComputeAssetSetID(a): %v", err)
	}
	idB, err := ComputeAssetSetID(b)
	if err != nil {
		t.Fatalf("ComputeAssetSetID(b): %v", err)
	}
	if idA != idB {
		t.Fatalf("asset set id must ignore AssetSetID itself: %s != %s", idA, idB)
	}
	if !strings.HasPrefix(idA, "sha256:") || len(idA) != len("sha256:")+64 {
		t.Fatalf("unexpected asset set id shape: %q", idA)
	}

	changed := a
	changed.Entries = append([]Entry(nil), a.Entries...)
	changed.Entries[0].Size = a.Entries[0].Size + 1
	idChanged, err := ComputeAssetSetID(changed)
	if err != nil {
		t.Fatalf("ComputeAssetSetID(changed): %v", err)
	}
	if idChanged == idA {
		t.Fatalf("digest did not change when an entry changed")
	}
}

func TestValidateManifestAcceptsMinimal(t *testing.T) {
	if err := ValidateManifest(validManifest(t)); err != nil {
		t.Fatalf("ValidateManifest rejected a valid manifest: %v", err)
	}
}

func TestValidateManifestRejects(t *testing.T) {
	// mutate applies a defect to an otherwise valid manifest. Each case that
	// does not itself set AssetSetID gets a recomputed id first, so the only
	// defect under test is the one the case introduces.
	cases := []struct {
		name        string
		mutate      func(m *Manifest)
		keepStaleID bool
	}{
		{name: "unsorted entries", mutate: func(m *Manifest) {
			m.Entries[0], m.Entries[1] = m.Entries[1], m.Entries[0]
		}},
		{name: "duplicate path", mutate: func(m *Manifest) {
			m.Entries[1].Path = m.Entries[0].Path
		}},
		{name: "parent escape", mutate: func(m *Manifest) {
			m.Entries[0].Path = "../escape"
		}},
		{name: "absolute path", mutate: func(m *Manifest) {
			m.Entries[0].Path = "/etc/passwd"
		}},
		{name: "empty segment", mutate: func(m *Manifest) {
			m.Entries[0].Path = "skills//SKILL.md"
		}},
		{name: "dot segment", mutate: func(m *Manifest) {
			m.Entries[0].Path = "./SKILL.md"
		}},
		{name: "empty path", mutate: func(m *Manifest) {
			m.Entries[0].Path = ""
		}},
		{name: "backslash", mutate: func(m *Manifest) {
			m.Entries[0].Path = `a\b`
		}},
		{name: "unknown role", mutate: func(m *Manifest) {
			m.Entries[0].Role = Role("mystery")
		}},
		{name: "executable mode", mutate: func(m *Manifest) {
			m.Entries[0].Mode = 0o755
		}},
		{name: "negative size", mutate: func(m *Manifest) {
			m.Entries[0].Size = -1
		}},
		{name: "short sha", mutate: func(m *Manifest) {
			m.Entries[0].SHA256 = "abc123"
		}},
		{name: "uppercase sha", mutate: func(m *Manifest) {
			m.Entries[0].SHA256 = strings.ToUpper(hex64('a'))
		}},
		{name: "wrong asset set id", keepStaleID: true, mutate: func(m *Manifest) {
			m.AssetSetID = "sha256:" + hex64('f')
		}},
		{name: "unknown format version", keepStaleID: true, mutate: func(m *Manifest) {
			m.FormatVersion = ManifestFormatVersion + 1
		}},
		{name: "unknown asset protocol", keepStaleID: true, mutate: func(m *Manifest) {
			m.AssetProtocol = AssetProtocol + 1
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest(t)
			m.Entries = append([]Entry(nil), m.Entries...)
			tc.mutate(&m)
			if !tc.keepStaleID {
				id, err := ComputeAssetSetID(m)
				if err != nil {
					t.Fatalf("ComputeAssetSetID: %v", err)
				}
				m.AssetSetID = id
			}
			err := ValidateManifest(m)
			if err == nil {
				t.Fatalf("ValidateManifest accepted an invalid manifest (%s)", tc.name)
			}
			if !errors.Is(err, ErrManifestInvalid) {
				t.Fatalf("error not wrapped with ErrManifestInvalid: %v", err)
			}
		})
	}
}
