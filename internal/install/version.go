package install

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/assets"
)

// The version tree is the one place a release installation's asset bytes live,
// and everything else — every skill symlink a harness follows — points into it.
// Two properties make that safe to point at:
//
//   - It is published by a single rename, so a directory named
//     versions/<asset-set-id> either does not exist or is a complete tree. A
//     partially extracted bundle is never reachable under that name: extraction
//     happens in a sibling staging directory that is removed on any failure.
//   - Its files are immutable after publication (0o444), so the bytes a symlink
//     resolves to cannot drift under an installation that has already recorded
//     their digests. Directories stay writable (0o755): immutability is a
//     property of the content, and sealing the directories too would only mean
//     neither the user nor docket could ever delete a superseded tree — a
//     read-only directory refuses to unlink its children.
//
// Reuse is therefore an integrity question, never a naming one: an existing tree
// is adopted only after every manifest entry is re-read from disk and matched,
// and a tree that fails is refused outright rather than repaired in place —
// repairing would mean writing into a directory some other installation may
// already be reading.

const (
	// versionFileMode seals the published tree's content; versionDirMode leaves
	// its directories traversable and removable. Both are applied by an explicit
	// Chmod, so neither depends on the process umask.
	versionFileMode = 0o444
	versionDirMode  = 0o755
	// versionsDirMode is the private container the trees are published into.
	versionsDirMode = 0o700
	// stagingWriteMode is what extraction needs before the tree is sealed.
	stagingWriteMode = 0o700
	// versionAssetsDir is the subdirectory of a version root that holds the
	// bundle itself, mirroring UserRoots.VersionDir.
	versionAssetsDir = "assets"
	versionManifest  = "manifest.json"
)

// ErrVersionTreeInvalid is the sentinel every unusable extracted tree wraps:
// incomplete, mutated, or carrying files the manifest does not describe.
var ErrVersionTreeInvalid = errors.New("install: extracted version tree invalid")

// ProvenVersion is an extracted asset tree whose version-root layout,
// manifest, and payload bytes have all been checked without following links.
// Legacy reports a pre-manifest protocol-v1 tree reconstructed from the frozen
// installer-owned v1 inventory.
type ProvenVersion struct {
	Root      string
	AssetsDir string
	Manifest  assets.Manifest
	Legacy    bool
}

// EnsureVersionTree makes the manifest's bundle available as an immutable tree
// under <DataRoot>/versions/<asset-set-id>/assets and returns that directory.
//
// A complete, byte-identical tree that is already there is reused — verified,
// not trusted by name — and open is not called at all in that case. An existing
// directory that fails verification returns ErrVersionTreeInvalid: it is never
// adopted and never silently replaced, because whatever is wrong with it may be
// the only evidence of how it got that way.
func EnsureVersionTree(roots UserRoots, m assets.Manifest, open func(string) ([]byte, error)) (string, bool, error) {
	if open == nil {
		return "", false, errors.New("install: EnsureVersionTree requires an open function")
	}
	// The manifest steers every path this function creates, so it is validated
	// before a single directory exists.
	if err := assets.ValidateManifest(m); err != nil {
		return "", false, fmt.Errorf("install: refusing to extract an invalid bundle: %w", err)
	}

	assetsDir := roots.VersionDir(m.AssetSetID)
	versionRoot := filepath.Dir(assetsDir)

	switch _, err := os.Lstat(versionRoot); {
	case err == nil:
		proven, err := ProveVersionTree(versionRoot)
		if err != nil {
			return "", false, err
		}
		if proven.Manifest.AssetSetID != m.AssetSetID {
			return "", false, fmt.Errorf("%w: %s identifies %s, want %s", ErrVersionTreeInvalid, versionRoot, proven.Manifest.AssetSetID, m.AssetSetID)
		}
		if err := verifyVersionTree(proven.AssetsDir, m); err != nil {
			return "", false, err
		}
		return proven.AssetsDir, true, nil
	case !errors.Is(err, fs.ErrNotExist):
		return "", false, fmt.Errorf("install: inspecting %s: %w", versionRoot, err)
	}

	if err := os.MkdirAll(roots.VersionsDir(), versionsDirMode); err != nil {
		return "", false, fmt.Errorf("install: creating %s: %w", roots.VersionsDir(), err)
	}
	staging, err := os.MkdirTemp(roots.VersionsDir(), ".staging-"+sanitizeSegment(m.AssetSetID)+"-")
	if err != nil {
		return "", false, fmt.Errorf("install: staging a version tree under %s: %w", roots.VersionsDir(), err)
	}
	// Every exit before the publishing rename takes the staging directory with
	// it: a half-extracted tree left under versions/ would be indistinguishable
	// from one a later run could adopt.
	published := false
	defer func() {
		if !published {
			discardTree(staging)
		}
	}()

	stagedAssets := filepath.Join(staging, versionAssetsDir)
	if err := extractTree(stagedAssets, m, open); err != nil {
		return "", false, err
	}
	canonical, err := assets.EncodeCanonical(m)
	if err != nil {
		return "", false, fmt.Errorf("install: encoding version manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, versionManifest), canonical, stagingWriteMode); err != nil {
		return "", false, fmt.Errorf("install: writing version manifest: %w", err)
	}
	// Verified from disk rather than from the bytes just held in memory, so a
	// short or refused write is caught here rather than by the next run.
	if err := sealTree(staging); err != nil {
		return "", false, err
	}
	if err := verifyVersionTree(stagedAssets, m); err != nil {
		return "", false, err
	}

	if err := os.Rename(staging, versionRoot); err != nil {
		// A concurrent installation of the same bundle may have published first.
		// That tree is as good as this one — provided it verifies — so adopt it
		// and let the deferred cleanup discard the staged duplicate.
		if proven, verifyErr := ProveVersionTree(versionRoot); verifyErr == nil && proven.Manifest.AssetSetID == m.AssetSetID {
			if verifyErr := verifyVersionTree(proven.AssetsDir, m); verifyErr == nil {
				return proven.AssetsDir, true, nil
			}
		}
		return "", false, fmt.Errorf("install: publishing %s: %w", versionRoot, err)
	}
	published = true
	return assetsDir, false, nil
}

// ProveVersionTree validates the whole version root rather than merely an
// assets subdirectory. New trees carry a canonical saved manifest; the sole
// compatibility exception is the frozen v1 root shape, whose manifest is
// reconstructed from installer-owned constants and never written back.
func ProveVersionTree(root string) (ProvenVersion, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return ProvenVersion{}, fmt.Errorf("install: inspecting version root %s: %w", root, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != versionDirMode {
		return ProvenVersion{}, fmt.Errorf("%w: version root %s is not a directory", ErrVersionTreeInvalid, root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ProvenVersion{}, fmt.Errorf("install: reading version root %s: %w", root, err)
	}
	assetsDir := filepath.Join(root, versionAssetsDir)
	assetsInfo, err := os.Lstat(assetsDir)
	if err != nil {
		return ProvenVersion{}, fmt.Errorf("%w: version root %s has no assets directory", ErrVersionTreeInvalid, root)
	}
	if !assetsInfo.IsDir() || assetsInfo.Mode()&os.ModeSymlink != 0 {
		return ProvenVersion{}, fmt.Errorf("%w: %s is not a real assets directory", ErrVersionTreeInvalid, assetsDir)
	}

	has := make(map[string]bool, len(entries))
	for _, entry := range entries {
		has[entry.Name()] = true
	}
	if len(entries) == 1 && has[versionAssetsDir] {
		m, err := reconstructLegacyV1Manifest(assetsDir)
		if err != nil {
			return ProvenVersion{}, err
		}
		if err := verifyVersionTree(assetsDir, m); err != nil {
			return ProvenVersion{}, err
		}
		return ProvenVersion{Root: root, AssetsDir: assetsDir, Manifest: m, Legacy: true}, nil
	}
	if len(entries) != 2 || !has[versionAssetsDir] || !has[versionManifest] {
		return ProvenVersion{}, fmt.Errorf("%w: version root %s must contain exactly %s and %s", ErrVersionTreeInvalid, root, versionManifest, versionAssetsDir)
	}
	manifestPath := filepath.Join(root, versionManifest)
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return ProvenVersion{}, fmt.Errorf("install: inspecting %s: %w", manifestPath, err)
	}
	if !manifestInfo.Mode().IsRegular() || manifestInfo.Mode().Perm() != versionFileMode {
		return ProvenVersion{}, fmt.Errorf("%w: %s is not an immutable regular manifest", ErrVersionTreeInvalid, manifestPath)
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return ProvenVersion{}, fmt.Errorf("install: reading %s: %w", manifestPath, err)
	}
	m, err := assets.DecodeManifest(raw)
	if err != nil {
		return ProvenVersion{}, fmt.Errorf("%w: saved manifest: %v", ErrVersionTreeInvalid, err)
	}
	if err := assets.ValidateManifest(m); err != nil {
		return ProvenVersion{}, fmt.Errorf("%w: saved manifest: %v", ErrVersionTreeInvalid, err)
	}
	canonical, err := assets.EncodeCanonical(m)
	if err != nil {
		return ProvenVersion{}, fmt.Errorf("install: encoding saved manifest: %w", err)
	}
	if !bytes.Equal(raw, canonical) {
		return ProvenVersion{}, fmt.Errorf("%w: saved manifest %s is not canonical", ErrVersionTreeInvalid, manifestPath)
	}
	if sanitizeSegment(m.AssetSetID) != filepath.Base(root) {
		return ProvenVersion{}, fmt.Errorf("%w: saved manifest identity %s does not name %s", ErrVersionTreeInvalid, m.AssetSetID, root)
	}
	if err := verifyVersionTree(assetsDir, m); err != nil {
		return ProvenVersion{}, err
	}
	return ProvenVersion{Root: root, AssetsDir: assetsDir, Manifest: m}, nil
}

// extractTree writes every manifest entry beneath dir. Each payload is checked
// against its entry before it is written: a bundle that disagrees with its own
// manifest must not reach disk at all, even inside staging.
func extractTree(dir string, m assets.Manifest, open func(string) ([]byte, error)) error {
	if err := os.MkdirAll(dir, stagingWriteMode); err != nil {
		return fmt.Errorf("install: creating %s: %w", dir, err)
	}
	for _, e := range m.Entries {
		body, err := open(e.Path)
		if err != nil {
			return fmt.Errorf("install: reading bundle entry %s: %w", e.Path, err)
		}
		if int64(len(body)) != e.Size {
			return fmt.Errorf("%w: %s is %d bytes, the manifest says %d",
				assets.ErrManifestInvalid, e.Path, len(body), e.Size)
		}
		if got := hashBytes(body); got != e.SHA256 {
			return fmt.Errorf("%w: %s digests %s, the manifest says %s",
				assets.ErrManifestInvalid, e.Path, got, e.SHA256)
		}
		// The path is safe by ValidateManifest, so the join cannot escape dir.
		full := filepath.Join(dir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(full), stagingWriteMode); err != nil {
			return fmt.Errorf("install: creating %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, body, os.FileMode(e.Mode).Perm()); err != nil {
			return fmt.Errorf("install: writing %s: %w", full, err)
		}
	}
	return nil
}

// verifyVersionTree proves dir is exactly the manifest's bundle: every entry
// present as a regular file with the recorded size and digest, and nothing else
// present at all. The second half matters as much as the first — an extra file
// under a tree the installer hands to a harness is content docket cannot
// account for.
func verifyVersionTree(dir string, m assets.Manifest) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("install: inspecting %s: %w", dir, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is not a real assets directory", ErrVersionTreeInvalid, dir)
	}
	wanted := make(map[string]bool, len(m.Entries))
	wantedDirs := map[string]bool{".": true}
	for _, e := range m.Entries {
		wanted[e.Path] = true
		for parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(e.Path))); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			wantedDirs[parent] = true
		}
		full := filepath.Join(dir, filepath.FromSlash(e.Path))
		info, err := os.Lstat(full)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("%w: %s is missing %s", ErrVersionTreeInvalid, dir, e.Path)
		case err != nil:
			return fmt.Errorf("install: inspecting %s: %w", full, err)
		case !info.Mode().IsRegular():
			return fmt.Errorf("%w: %s is not a regular file", ErrVersionTreeInvalid, full)
		case info.Mode().Perm() != versionFileMode:
			return fmt.Errorf("%w: %s has mode %o, want %o", ErrVersionTreeInvalid, full, info.Mode().Perm(), versionFileMode)
		case info.Size() != e.Size:
			return fmt.Errorf("%w: %s is %d bytes, the manifest says %d",
				ErrVersionTreeInvalid, full, info.Size(), e.Size)
		}
		body, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("install: reading %s: %w", full, err)
		}
		if got := hashBytes(body); got != e.SHA256 {
			return fmt.Errorf("%w: %s digests %s, the manifest says %s",
				ErrVersionTreeInvalid, full, got, e.SHA256)
		}
	}
	return rejectUnmanifestedFiles(dir, wanted, wantedDirs)
}

// rejectUnmanifestedFiles walks the tree and refuses anything the manifest does
// not name. A directory must be an ancestor of a manifest entry; this excludes
// both unexpected and empty directories as well as every unmanifested file.
func rejectUnmanifestedFiles(dir string, wanted, wantedDirs map[string]bool) error {
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if d.IsDir() {
			if !wantedDirs[relSlash] || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != versionDirMode {
				return fmt.Errorf("%w: unexpected directory %s", ErrVersionTreeInvalid, p)
			}
			return nil
		}
		if !wanted[relSlash] || !info.Mode().IsRegular() {
			return fmt.Errorf("%w: %s is not described by the manifest", ErrVersionTreeInvalid, p)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrVersionTreeInvalid) {
			return err
		}
		return fmt.Errorf("install: reading %s: %w", dir, err)
	}
	return nil
}

// sealTree makes the staged tree's files read-only before it is published, so
// the rename hands over something already immutable rather than something that
// becomes immutable a moment later. Directories are normalized to
// versionDirMode instead: they stay writable, because immutability is carried
// by the file modes plus the single publishing rename, and a sealed directory
// would only cost the tree its removability.
func sealTree(root string) error {
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := os.FileMode(versionFileMode)
		if d.IsDir() {
			mode = versionDirMode
		}
		return os.Chmod(p, mode)
	})
	if err != nil {
		return fmt.Errorf("install: sealing %s: %w", root, err)
	}
	return nil
}

// discardTree removes a staged tree that will never be published. Every
// directory it can contain is writable — stagingWriteMode before sealing,
// versionDirMode after — so a plain removal suffices even though the staged
// files themselves may already be 0o444. Failure is not reported: the caller is
// already returning the error that mattered, and a leftover staging directory
// is inert — it is never adopted, because adoption only ever looks at
// versions/<asset-set-id>.
func discardTree(root string) {
	_ = os.RemoveAll(root)
}
