package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ReferenceSet is the set of extracted version roots that installation state
// still reaches. Its keys are canonical paths to immediate children of
// UserRoots.VersionsDir, never asset paths or the uncanonical spelling a
// symlink happened to carry.
type ReferenceSet map[string]struct{}

// DeriveVersionReferences returns every release version tree that the state
// names directly or indirectly. It deliberately preserves both halves of a
// changed link: the recorded destination is the ownership history, while a
// current symlink destination is a live filesystem reference. Either can keep
// an immutable tree reachable until a later transaction has reconciled them.
//
// Reference collection is a cleanup safety boundary. A path that cannot be
// resolved enough to prove its containment is an error, rather than an excuse
// to collect no trees. Ordinary target paths live outside versions/ and are
// therefore non-candidates; a release symlink destination is expected to name
// a version tree and is refused if it does not.
func DeriveVersionReferences(roots UserRoots, state *State) (ReferenceSet, error) {
	if state == nil {
		return nil, fmt.Errorf("%w: cannot derive references from nil state", ErrStateInvalid)
	}
	if err := ValidateState(state); err != nil {
		return nil, fmt.Errorf("%w: cannot derive references: %v", ErrStateInvalid, err)
	}

	versions, err := canonicalPath(roots.VersionsDir())
	if err != nil {
		return nil, fmt.Errorf("install: canonicalising versions directory: %w", err)
	}
	refs := ReferenceSet{}

	// Development installs point directly into a contributor checkout. That
	// source is deliberately outside versions/ and must not make a release
	// collector retain a tree merely because the two trees share a spelling.
	if state.Mode == ModeDevelopment {
		source, err := canonicalPath(state.SourceRoot)
		if err != nil {
			return nil, fmt.Errorf("install: canonicalising development source %s: %w", state.SourceRoot, err)
		}
		if _, within, err := versionRootFor(versions, source); err != nil {
			return nil, err
		} else if within {
			return nil, fmt.Errorf("install: development source %s is inside versions", source)
		}
		return refs, nil
	}

	if state.AssetSetID == "" {
		return nil, fmt.Errorf("%w: release state has no asset set id", ErrStateInvalid)
	}
	if err := addRequiredReference(refs, versions,
		filepath.Join(versions, sanitizeSegment(state.AssetSetID), versionAssetsDir), "state asset set"); err != nil {
		return nil, err
	}

	for _, record := range state.Targets {
		// Target paths are normally user configuration paths, but treating a
		// target within versions/ as a reference prevents a future target shape
		// from accidentally making its own backing tree collectible.
		if err := addOptionalReference(refs, versions, record.Path); err != nil {
			return nil, fmt.Errorf("install: resolving recorded target %s: %w", record.Path, err)
		}
		if record.LinkTarget != "" {
			if err := addRequiredReference(refs, versions, record.LinkTarget, "recorded link destination"); err != nil {
				return nil, err
			}
		}

		info, err := os.Lstat(record.Path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// A missing target must not erase the recorded link destination.
			continue
		case err != nil:
			return nil, fmt.Errorf("install: inspecting recorded target %s: %w", record.Path, err)
		case info.Mode()&fs.ModeSymlink == 0:
			// A replaced link similarly leaves only the recorded destination.
			continue
		}

		live, err := linkDestination(record.Path)
		if err != nil {
			return nil, err
		}
		if err := addRequiredReference(refs, versions, live, "live link destination"); err != nil {
			return nil, err
		}
	}
	return refs, nil
}

func addRequiredReference(refs ReferenceSet, versions, candidate, source string) error {
	root, within, err := versionRootFor(versions, candidate)
	if err != nil {
		return err
	}
	if !within {
		return fmt.Errorf("install: %s %s is outside versions directory %s", source, candidate, versions)
	}
	refs[root] = struct{}{}
	return nil
}

func addOptionalReference(refs ReferenceSet, versions, candidate string) error {
	root, within, err := versionRootFor(versions, candidate)
	if err != nil {
		return err
	}
	if within {
		refs[root] = struct{}{}
	}
	return nil
}

// versionRootFor returns the one immediate, real version root that contains
// candidate. The candidate must be strictly below that root: versions/<id>
// itself is container metadata, not a reachable asset path. A symlinked root,
// a missing root, or a malformed shallow path cannot establish containment.
func versionRootFor(versions, candidate string) (string, bool, error) {
	canonical, err := canonicalPath(candidate)
	if err != nil {
		return "", false, err
	}
	rel, err := filepath.Rel(versions, canonical)
	if err != nil {
		return "", false, fmt.Errorf("install: comparing %s with versions directory %s: %w", canonical, versions, err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false, nil
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 || parts[0] == "" || parts[0] == "." || parts[0] == ".." {
		return "", false, fmt.Errorf("install: reference %s is not a strict descendant of a version root", candidate)
	}
	root := filepath.Join(versions, parts[0])
	info, err := os.Lstat(root)
	if err != nil {
		return "", false, fmt.Errorf("install: inspecting version root %s: %w", root, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return "", false, fmt.Errorf("install: version root %s is not a non-symlink directory", root)
	}
	return root, true, nil
}
