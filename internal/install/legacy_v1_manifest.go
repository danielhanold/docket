package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
)

// The complete protocol-v1 extraction inventory belongs to the installer,
// rather than assets.DefaultAllowedRoots, because admitting a historic root
// must never expand when the live bundle generator gains one.
const (
	legacyV1SkillsRoot      = "skills"
	legacyV1AgentsRoot      = "agents"
	legacyV1CursorRulesRoot = "cursor-rules"
	legacyV1ConfigRoot      = ".docket.example.yml"
	legacyV1HarnessDefaults = "harness-defaults.yml"
)

// reconstructLegacyV1Manifest derives a manifest only for a sealed,
// pre-manifest protocol-v1 assets tree. It intentionally classifies paths from
// installer-owned constants instead of consulting the live generator's allowed
// roots.
func reconstructLegacyV1Manifest(assetsDir string) (assets.Manifest, error) {
	info, err := os.Lstat(assetsDir)
	if err != nil {
		return assets.Manifest{}, fmt.Errorf("install: inspecting legacy assets %s: %w", assetsDir, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != versionDirMode {
		return assets.Manifest{}, fmt.Errorf("%w: legacy assets %s is not a directory", ErrVersionTreeInvalid, assetsDir)
	}

	roots := map[string]assets.Role{
		legacyV1SkillsRoot:      assets.RoleSkill,
		legacyV1AgentsRoot:      assets.RoleAgentSource,
		legacyV1CursorRulesRoot: assets.RoleDispatch,
		legacyV1ConfigRoot:      assets.RoleConfigSchema,
	}
	seenRoots := make(map[string]bool, len(roots))
	dirs := make(map[string]bool)
	hasKnownDescendant := make(map[string]bool)
	var entries []assets.Entry

	err = filepath.WalkDir(assetsDir, func(full string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(assetsDir, full)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			dirs[rel] = true
			return nil
		}
		segments := strings.Split(rel, "/")
		role, knownRoot := roots[segments[0]]
		if !knownRoot {
			return fmt.Errorf("%w: legacy tree contains unknown root %s", ErrVersionTreeInvalid, full)
		}
		seenRoots[segments[0]] = true
		fileInfo, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if d.IsDir() {
			if fileInfo.Mode()&os.ModeSymlink != 0 || fileInfo.Mode().Perm() != versionDirMode {
				return fmt.Errorf("%w: legacy tree contains linked directory %s", ErrVersionTreeInvalid, full)
			}
			if len(segments) == 1 && segments[0] == ".docket.example.yml" {
				return fmt.Errorf("%w: legacy config root %s is a directory", ErrVersionTreeInvalid, full)
			}
			dirs[rel] = true
			return nil
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("%w: legacy tree contains non-regular file %s", ErrVersionTreeInvalid, full)
		}
		if fileInfo.Mode().Perm() != versionFileMode {
			return fmt.Errorf("%w: legacy file %s has mode %o, want %o", ErrVersionTreeInvalid, full, fileInfo.Mode().Perm(), versionFileMode)
		}
		if segments[0] == ".docket.example.yml" && rel != ".docket.example.yml" {
			return fmt.Errorf("%w: legacy config root %s is not a file", ErrVersionTreeInvalid, full)
		}
		if segments[0] != ".docket.example.yml" && len(segments) == 1 {
			return fmt.Errorf("%w: legacy root %s is not a directory", ErrVersionTreeInvalid, full)
		}
		if role == assets.RoleAgentSource && path.Base(rel) == legacyV1HarnessDefaults {
			role = assets.RoleHarnessDefaults
		}
		body, readErr := os.ReadFile(full)
		if readErr != nil {
			return readErr
		}
		entries = append(entries, assets.Entry{
			Path:   rel,
			Role:   role,
			Mode:   0o644,
			Size:   int64(len(body)),
			SHA256: hashBytes(body),
		})
		for ancestor := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel))); ancestor != "."; ancestor = filepath.ToSlash(filepath.Dir(ancestor)) {
			hasKnownDescendant[ancestor] = true
		}
		hasKnownDescendant["."] = true
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrVersionTreeInvalid) {
			return assets.Manifest{}, err
		}
		return assets.Manifest{}, fmt.Errorf("install: walking legacy assets %s: %w", assetsDir, err)
	}
	for root := range roots {
		if !seenRoots[root] {
			return assets.Manifest{}, fmt.Errorf("%w: legacy tree is missing protocol-v1 root %s", ErrVersionTreeInvalid, root)
		}
	}
	for dir := range dirs {
		if !hasKnownDescendant[dir] {
			return assets.Manifest{}, fmt.Errorf("%w: legacy tree contains empty directory %s", ErrVersionTreeInvalid, filepath.Join(assetsDir, filepath.FromSlash(dir)))
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	m := assets.Manifest{FormatVersion: assets.ManifestFormatVersion, AssetProtocol: assets.AssetProtocol, Entries: entries}
	id, err := assets.ComputeAssetSetID(m)
	if err != nil {
		return assets.Manifest{}, fmt.Errorf("install: computing legacy manifest identity: %w", err)
	}
	m.AssetSetID = id
	if err := assets.ValidateManifest(m); err != nil {
		return assets.Manifest{}, fmt.Errorf("%w: reconstructed legacy manifest: %v", ErrVersionTreeInvalid, err)
	}
	if sanitizeSegment(m.AssetSetID) != filepath.Base(filepath.Dir(assetsDir)) {
		return assets.Manifest{}, fmt.Errorf("%w: reconstructed legacy identity %s does not name %s", ErrVersionTreeInvalid, m.AssetSetID, assetsDir)
	}
	return m, nil
}
