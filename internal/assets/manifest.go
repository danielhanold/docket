// Package assets models the frozen asset bundle that ships inside the docket
// binary: a canonical manifest describing every embedded file, its role, and
// its digest. This file owns the manifest model only — no payload access and
// no filesystem access.
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ManifestFormatVersion is the version of the manifest document shape itself.
const ManifestFormatVersion = 1

// AssetProtocol is the version of the asset contract between the generator
// that freezes the bundle and the installer that extracts it.
const AssetProtocol = 1

// ErrManifestInvalid is the sentinel every ValidateManifest failure wraps.
var ErrManifestInvalid = errors.New("asset manifest invalid")

// Role classifies what an embedded asset is, which decides where the
// installer places it and which harness adapters consume it.
type Role string

const (
	RoleSkill           Role = "skill"            // skills/<skill>/**
	RoleAgentSource     Role = "agent-source"     // agents/docket-*.md
	RoleHarnessDefaults Role = "harness-defaults" // agents/harness-defaults.yml
	RoleDispatch        Role = "dispatch"         // cursor-rules/**
	RoleConfigSchema    Role = "config-schema"    // .docket.example.yml
)

// Entry describes one embedded file.
type Entry struct {
	Path   string `json:"path"`   // slash-separated, relative, no ".."/"."/empty segments
	Role   Role   `json:"role"`   //
	Mode   uint32 `json:"mode"`   // portable policy mode: 0o644 files only in v1
	Size   int64  `json:"size"`   //
	SHA256 string `json:"sha256"` // lowercase hex
}

// Manifest is the frozen description of an asset bundle.
type Manifest struct {
	FormatVersion int     `json:"format_version"`
	AssetProtocol int     `json:"asset_protocol"`
	AssetSetID    string  `json:"asset_set_id"`
	Entries       []Entry `json:"entries"`
}

// entryMode is the only file mode permitted in asset protocol v1: release
// bundles carry regular, non-executable files exclusively.
const entryMode uint32 = 0o644

// EncodeCanonical renders the manifest as canonical JSON: two-space indent,
// keys in struct order, entries sorted by Path, trailing newline.
func EncodeCanonical(m Manifest) ([]byte, error) {
	out := m
	out.Entries = append([]Entry(nil), m.Entries...)
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].Path < out.Entries[j].Path })

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("encode asset manifest: %w", err)
	}
	// json.Encoder.Encode already appends the trailing newline.
	return buf.Bytes(), nil
}

// ComputeAssetSetID digests the canonical encoding with AssetSetID forced to
// "", so the identifier never depends on itself.
func ComputeAssetSetID(m Manifest) (string, error) {
	bare := m
	bare.AssetSetID = ""
	encoded, err := EncodeCanonical(bare)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// SafeRelPath reports whether p is a safe slash-separated relative path: not
// empty, not absolute, no backslash, and no "..", "." or empty segment.
func SafeRelPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func knownRole(r Role) bool {
	switch r {
	case RoleSkill, RoleAgentSource, RoleHarnessDefaults, RoleDispatch, RoleConfigSchema:
		return true
	}
	return false
}

func lowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

// ValidateManifest checks structure only — it never reads asset payloads.
func ValidateManifest(m Manifest) error {
	if m.FormatVersion != ManifestFormatVersion {
		return fmt.Errorf("%w: unknown format_version %d (want %d)", ErrManifestInvalid, m.FormatVersion, ManifestFormatVersion)
	}
	if m.AssetProtocol != AssetProtocol {
		return fmt.Errorf("%w: unknown asset_protocol %d (want %d)", ErrManifestInvalid, m.AssetProtocol, AssetProtocol)
	}

	for i, e := range m.Entries {
		if !SafeRelPath(e.Path) {
			return fmt.Errorf("%w: entry %d: unsafe path %q", ErrManifestInvalid, i, e.Path)
		}
		if i > 0 && m.Entries[i-1].Path >= e.Path {
			return fmt.Errorf("%w: entry %d: paths must be strictly ascending (%q after %q)", ErrManifestInvalid, i, e.Path, m.Entries[i-1].Path)
		}
		if !knownRole(e.Role) {
			return fmt.Errorf("%w: entry %d (%s): unknown role %q", ErrManifestInvalid, i, e.Path, e.Role)
		}
		if e.Mode != entryMode {
			return fmt.Errorf("%w: entry %d (%s): mode must be %#o, got %#o", ErrManifestInvalid, i, e.Path, entryMode, e.Mode)
		}
		if e.Size < 0 {
			return fmt.Errorf("%w: entry %d (%s): negative size %d", ErrManifestInvalid, i, e.Path, e.Size)
		}
		if !lowerHex64(e.SHA256) {
			return fmt.Errorf("%w: entry %d (%s): sha256 must be 64 lowercase hex characters, got %q", ErrManifestInvalid, i, e.Path, e.SHA256)
		}
	}

	want, err := ComputeAssetSetID(m)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrManifestInvalid, err)
	}
	if m.AssetSetID != want {
		return fmt.Errorf("%w: asset_set_id %q does not match computed %q", ErrManifestInvalid, m.AssetSetID, want)
	}
	return nil
}
