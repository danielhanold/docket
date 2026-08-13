package assets

//go:generate go run ../../cmd/genassets -repo ../..

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
)

// ErrGenerate is the sentinel every bundle-generation failure wraps.
var ErrGenerate = errors.New("asset bundle generation failed")

// AllowedRoot names one authored tree (or single file) that may enter the
// bundle, together with the role its files carry.
type AllowedRoot struct {
	Root string // repo-relative, slash-separated: "skills", ".docket.example.yml"
	Role Role
}

// harnessDefaultsBase is the one file under the agents root whose role differs
// from its root's: it configures harnesses rather than defining an agent.
const harnessDefaultsBase = "harness-defaults.yml"

// DefaultAllowedRoots is the authored surface a release bundle freezes.
func DefaultAllowedRoots() []AllowedRoot {
	return []AllowedRoot{
		{Root: "skills", Role: RoleSkill},
		{Root: "agents", Role: RoleAgentSource},
		{Root: "cursor-rules", Role: RoleDispatch},
		{Root: ".docket.example.yml", Role: RoleConfigSchema},
	}
}

// roleFor applies the per-file role exceptions inside an allowed root.
func roleFor(root AllowedRoot, entryPath string) Role {
	if root.Role == RoleAgentSource && path.Base(entryPath) == harnessDefaultsBase {
		return RoleHarnessDefaults
	}
	return root.Role
}

// Generate walks repoDir's allowed roots and returns the manifest plus a
// path->bytes payload map. It fails on a symlink or any other non-regular
// file, a root that escapes the repo, a path collision after normalization, an
// absent root, or an unreadable file. Release bundles carry regular files only,
// so the walk never follows a link.
func Generate(repoDir string, roots []AllowedRoot) (Manifest, map[string][]byte, error) {
	payload := make(map[string][]byte)
	owner := make(map[string]string) // manifest path -> the root that claimed it
	var entries []Entry

	for _, root := range roots {
		if !SafeRelPath(root.Root) {
			return Manifest{}, nil, fmt.Errorf("%w: allowed root %q is not a safe repo-relative path", ErrGenerate, root.Root)
		}
		abs := filepath.Join(repoDir, filepath.FromSlash(root.Root))

		info, err := os.Lstat(abs)
		if err != nil {
			return Manifest{}, nil, fmt.Errorf("%w: allowed root %q: %s", ErrGenerate, root.Root, err)
		}

		add := func(entryPath string, fullPath string, info fs.FileInfo) error {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%w: %s is not a regular file (mode %s) — bundles carry no symlinks or special files", ErrGenerate, entryPath, info.Mode())
			}
			if !SafeRelPath(entryPath) {
				return fmt.Errorf("%w: %s escapes its allowed root", ErrGenerate, entryPath)
			}
			if prior, dup := owner[entryPath]; dup {
				return fmt.Errorf("%w: %s is claimed by both root %q and root %q", ErrGenerate, entryPath, prior, root.Root)
			}
			body, err := os.ReadFile(fullPath)
			if err != nil {
				return fmt.Errorf("%w: read %s: %s", ErrGenerate, entryPath, err)
			}
			sum := sha256.Sum256(body)
			owner[entryPath] = root.Root
			payload[entryPath] = body
			entries = append(entries, Entry{
				Path:   entryPath,
				Role:   roleFor(root, entryPath),
				Mode:   entryMode,
				Size:   int64(len(body)),
				SHA256: hex.EncodeToString(sum[:]),
			})
			return nil
		}

		if !info.IsDir() {
			// A single-file root: the manifest path is the root itself.
			if err := add(root.Root, abs, info); err != nil {
				return Manifest{}, nil, err
			}
			continue
		}

		walkErr := filepath.WalkDir(abs, func(full string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("%w: walk %s: %s", ErrGenerate, root.Root, err)
			}
			rel, relErr := filepath.Rel(abs, full)
			if relErr != nil {
				return fmt.Errorf("%w: %s: %s", ErrGenerate, full, relErr)
			}
			if rel == "." {
				return nil
			}
			entryPath := path.Join(root.Root, filepath.ToSlash(rel))
			fi, statErr := d.Info()
			if statErr != nil {
				return fmt.Errorf("%w: stat %s: %s", ErrGenerate, entryPath, statErr)
			}
			if fi.IsDir() {
				return nil
			}
			// WalkDir stats with Lstat, so a symlinked directory arrives here
			// as an irregular file rather than being descended into.
			return add(entryPath, full, fi)
		})
		if walkErr != nil {
			return Manifest{}, nil, walkErr
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	m := Manifest{
		FormatVersion: ManifestFormatVersion,
		AssetProtocol: AssetProtocol,
		Entries:       entries,
	}
	id, err := ComputeAssetSetID(m)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("%w: %s", ErrGenerate, err)
	}
	m.AssetSetID = id
	if err := ValidateManifest(m); err != nil {
		return Manifest{}, nil, fmt.Errorf("%w: %s", ErrGenerate, err)
	}
	return m, payload, nil
}

// WriteTree writes manifest.json plus tree/<path> under outDir, which must be
// absent or empty. Every payload is re-verified against its manifest entry
// before it is written, so a mismatched pair can never reach the tree.
func WriteTree(outDir string, m Manifest, payload map[string][]byte) error {
	if err := ValidateManifest(m); err != nil {
		return fmt.Errorf("%w: %s", ErrGenerate, err)
	}
	if len(payload) != len(m.Entries) {
		return fmt.Errorf("%w: manifest has %d entries but %d payloads were supplied", ErrGenerate, len(m.Entries), len(payload))
	}
	for _, e := range m.Entries {
		body, ok := payload[e.Path]
		if !ok {
			return fmt.Errorf("%w: no payload for entry %s", ErrGenerate, e.Path)
		}
		if int64(len(body)) != e.Size {
			return fmt.Errorf("%w: %s: payload is %d bytes, manifest says %d", ErrGenerate, e.Path, len(body), e.Size)
		}
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != e.SHA256 {
			return fmt.Errorf("%w: %s: payload digest %s does not match manifest %s", ErrGenerate, e.Path, got, e.SHA256)
		}
	}

	if existing, err := os.ReadDir(outDir); err == nil {
		if len(existing) > 0 {
			return fmt.Errorf("%w: output directory %s is not empty", ErrGenerate, outDir)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: read output directory %s: %s", ErrGenerate, outDir, err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("%w: create output directory: %s", ErrGenerate, err)
	}

	encoded, err := EncodeCanonical(m)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGenerate, err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), encoded, 0o644); err != nil {
		return fmt.Errorf("%w: write manifest.json: %s", ErrGenerate, err)
	}

	treeDir := filepath.Join(outDir, "tree")
	for _, e := range m.Entries {
		dest := filepath.Join(treeDir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("%w: create directory for %s: %s", ErrGenerate, e.Path, err)
		}
		if err := os.WriteFile(dest, payload[e.Path], fs.FileMode(e.Mode)); err != nil {
			return fmt.Errorf("%w: write %s: %s", ErrGenerate, e.Path, err)
		}
	}
	return nil
}

// TreePaths lists every regular file under a generated tree directory as
// slash-separated manifest paths. It is the reverse view -check needs to prove
// the committed tree carries nothing the manifest does not name.
func TreePaths(treeDir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(treeDir, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(treeDir, full)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: walk %s: %s", ErrGenerate, treeDir, err)
	}
	sort.Strings(out)
	return out, nil
}

// DiffTree compares a freshly generated bundle against a committed tree and
// returns one human-readable line per difference, sorted. An empty result means
// the committed tree is exactly what the generator produces today.
func DiffTree(committedDir string, m Manifest, payload map[string][]byte) ([]string, error) {
	var diffs []string

	encoded, err := EncodeCanonical(m)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrGenerate, err)
	}
	committedManifest, err := os.ReadFile(filepath.Join(committedDir, "manifest.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		diffs = append(diffs, "manifest.json: missing")
	case err != nil:
		return nil, fmt.Errorf("%w: read committed manifest.json: %s", ErrGenerate, err)
	case string(committedManifest) != string(encoded):
		diffs = append(diffs, "manifest.json: differs")
	}

	treeDir := filepath.Join(committedDir, "tree")
	for _, e := range m.Entries {
		got, err := os.ReadFile(filepath.Join(treeDir, filepath.FromSlash(e.Path)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			diffs = append(diffs, "tree/"+e.Path+": missing")
		case err != nil:
			return nil, fmt.Errorf("%w: read committed tree/%s: %s", ErrGenerate, e.Path, err)
		case string(got) != string(payload[e.Path]):
			diffs = append(diffs, "tree/"+e.Path+": differs")
		}
	}

	// A missing tree needs no reverse scan: every entry was already reported
	// above as missing.
	if _, statErr := os.Stat(treeDir); statErr == nil {
		committedPaths, err := TreePaths(treeDir)
		if err != nil {
			return nil, err
		}
		for _, p := range committedPaths {
			if _, ok := payload[p]; !ok {
				diffs = append(diffs, "tree/"+p+": not in the generated bundle")
			}
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: stat %s: %s", ErrGenerate, treeDir, statErr)
	}

	sort.Strings(diffs)
	return diffs, nil
}
