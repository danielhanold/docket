package assets

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
)

// DecodeManifest parses a canonical manifest document strictly: an unknown key
// or trailing content is a manifest defect, not something to tolerate.
func DecodeManifest(raw []byte) (Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%w: decode: %s", ErrManifestInvalid, err)
	}
	if dec.More() {
		return Manifest{}, fmt.Errorf("%w: trailing content after the manifest document", ErrManifestInvalid)
	}
	return m, nil
}

// The bundle frozen by cmd/genassets. The tree pattern carries the `all:`
// prefix because the bundle contains dotfiles (".docket.example.yml") and a
// bare directory pattern silently skips every name beginning with "." or "_" —
// which would drop payloads the manifest still names.
//
//go:embed embedded/manifest.json
//go:embed all:embedded/tree
var embeddedFS embed.FS

const (
	embeddedManifestPath = "embedded/manifest.json"
	embeddedTreeDir      = "embedded/tree"
)

// Open returns the embedded payload for a manifest path.
func Open(p string) ([]byte, error) {
	if !SafeRelPath(p) {
		return nil, fmt.Errorf("%w: unsafe asset path %q", ErrManifestInvalid, p)
	}
	body, err := embeddedFS.ReadFile(path.Join(embeddedTreeDir, p))
	if err != nil {
		return nil, fmt.Errorf("%w: embedded payload %s: %s", ErrManifestInvalid, p, err)
	}
	return body, nil
}

// VerifyPayloads checks every manifest entry against the bytes open returns:
// the payload must exist and its size and digest must match the entry.
func VerifyPayloads(m Manifest, open func(string) ([]byte, error)) error {
	for _, e := range m.Entries {
		body, err := open(e.Path)
		if err != nil {
			return fmt.Errorf("%w: %s: %s", ErrManifestInvalid, e.Path, err)
		}
		if int64(len(body)) != e.Size {
			return fmt.Errorf("%w: %s: payload is %d bytes, manifest says %d", ErrManifestInvalid, e.Path, len(body), e.Size)
		}
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != e.SHA256 {
			return fmt.Errorf("%w: %s: payload digest %s does not match manifest %s", ErrManifestInvalid, e.Path, got, e.SHA256)
		}
	}
	return nil
}

// embeddedTreePaths lists every path present in the embedded tree, so the
// runtime check can also run the reverse direction: a payload the manifest does
// not name is as much a corruption as a missing one.
func embeddedTreePaths() ([]string, error) {
	var out []string
	err := fs.WalkDir(embeddedFS, embeddedTreeDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, strings.TrimPrefix(p, embeddedTreeDir+"/"))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var embeddedOnce = sync.OnceValues(loadEmbeddedManifest)

func loadEmbeddedManifest() (Manifest, error) {
	raw, err := embeddedFS.ReadFile(embeddedManifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: read %s: %s", ErrManifestInvalid, embeddedManifestPath, err)
	}
	m, err := DecodeManifest(raw)
	if err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifest(m); err != nil {
		return Manifest{}, err
	}
	if err := VerifyPayloads(m, Open); err != nil {
		return Manifest{}, err
	}

	named := make(map[string]bool, len(m.Entries))
	for _, e := range m.Entries {
		named[e.Path] = true
	}
	present, err := embeddedTreePaths()
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: walk embedded tree: %s", ErrManifestInvalid, err)
	}
	for _, p := range present {
		if !named[p] {
			return Manifest{}, fmt.Errorf("%w: embedded tree carries %s, which the manifest does not name", ErrManifestInvalid, p)
		}
	}
	return m, nil
}

// EmbeddedManifest parses, validates, and verifies every embedded payload's
// size and digest. Every failure wraps ErrManifestInvalid, which the app layer
// reports as the stable reason asset-manifest-invalid.
func EmbeddedManifest() (Manifest, error) {
	m, err := embeddedOnce()
	if err != nil {
		return Manifest{}, err
	}
	// Hand out a copy: the cached entries must not be mutable through a caller.
	out := m
	out.Entries = append([]Entry(nil), m.Entries...)
	return out, nil
}

// Catalog is the read view harness adapters consume: a validated manifest plus
// the payload accessor it was verified against.
type Catalog struct {
	Manifest Manifest
	open     func(string) ([]byte, error)
}

// NewCatalog pairs a manifest with a payload accessor. It is the seam tests and
// future non-embedded sources (an extracted version tree) use.
func NewCatalog(m Manifest, open func(string) ([]byte, error)) Catalog {
	return Catalog{Manifest: m, open: open}
}

// EmbeddedCatalog is the catalog over the bundle frozen into this binary.
func EmbeddedCatalog() (Catalog, error) {
	m, err := EmbeddedManifest()
	if err != nil {
		return Catalog{}, err
	}
	return NewCatalog(m, Open), nil
}

// EntriesByRole returns the manifest entries carrying role r, in manifest order
// (ascending by path).
func (c Catalog) EntriesByRole(r Role) []Entry {
	var out []Entry
	for _, e := range c.Manifest.Entries {
		if e.Role == r {
			out = append(out, e)
		}
	}
	return out
}

// Bytes returns the payload for one manifest path.
func (c Catalog) Bytes(p string) ([]byte, error) {
	if c.open == nil {
		return nil, fmt.Errorf("%w: catalog has no payload accessor", ErrManifestInvalid)
	}
	return c.open(p)
}
