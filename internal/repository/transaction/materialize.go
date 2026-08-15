package transaction

// This file materializes a validated MutationPlan into a detached transaction
// worktree through an os.Root anchored there, and reads the result back.
//
// os.Root refuses, by construction, any name that escapes the root and any
// symlink hop that leaves it. This engine goes further than that baseline: the
// metadata tree it writes is trusted-but-verified, so a create/replace must
// never write THROUGH a symlink even one that stays inside the root, and a
// parent that is a symlink or a non-directory is a corrupted checkout, not a
// path to follow. Every parent component is therefore Lstat-walked (Lstat never
// follows the final component) and refused on symlink/non-directory, and the
// target itself is Lstat-checked before any bytes are written. Writes go to a
// sibling temp file in the target's own directory — same filesystem, so the
// rename into place is atomic — with the base file's mode copied onto the temp
// before the rename so an executable stays executable. Existing bytes outside
// the declared paths are never opened.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
)

// createdParentDirMode is the mode of a parent directory the materializer makes
// for a declared create. It is masked by the process umask like any Mkdir; the
// private containment of the whole transactions tree is enforced at its root
// (0700), not here.
const createdParentDirMode os.FileMode = 0o755

// defaultFileMode is the mode a freshly created file lands with when the target
// does not already exist. A replace instead copies the existing file's mode.
const defaultFileMode os.FileMode = 0o644

// materializePlan applies a validated plan inside worktree through an os.Root
// anchored there. For create/replace it writes to a sibling temp file in the
// target's directory, syncs, and renames into place; for delete it removes the
// file. Creates may make missing parent directories, but only beneath the root
// and only for declared file paths. It refuses (typed *Failure, stage
// "materialize") any parent component that is a symlink or non-directory, any
// create/replace target that is a symlink, and — defense in depth over
// validatePlan — any escaping or otherwise unsafe declared path.
func materializePlan(worktree string, plan MutationPlan) error {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "opening worktree root", Err: err}
	}
	defer func() { _ = root.Close() }()

	for _, f := range plan.Files {
		if err := materializeOne(root, f); err != nil {
			return err
		}
	}
	return nil
}

// materializeOne applies one file mutation. It first re-validates the declared
// path's shape (validatePlan already did, but the filesystem write is the last
// line of defense), then dispatches by kind.
func materializeOne(root *os.Root, f FileMutation) error {
	if err := validateRepoPathValue(f.Path); err != nil {
		return &Failure{Stage: StageMaterialize, Kind: KindInvalidInput, Detail: "unsafe path in plan", Err: err}
	}
	switch f.Kind {
	case MutationCreate, MutationReplace:
		return writePlannedFile(root, f)
	case MutationDelete:
		return deletePlannedFile(root, string(f.Path))
	default:
		return &Failure{Stage: StageMaterialize, Kind: KindInvalidInput, Detail: "unknown mutation kind"}
	}
}

// writePlannedFile materializes a create or replace: it verifies (and for a
// create, creates) the parent directories, inspects the target, writes a sibling
// temp file carrying the correct mode, and renames it into place.
func writePlannedFile(root *os.Root, f FileMutation) error {
	rel := string(f.Path)
	dir := path.Dir(rel)
	if err := ensureParentDirs(root, dir, f.Kind); err != nil {
		return err
	}

	// Inspect the target itself. Lstat never follows the final component, so a
	// symlink target is caught here rather than written through.
	mode := defaultFileMode
	if info, err := root.Lstat(rel); err == nil {
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "target is a symlink"}
		case info.IsDir():
			return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "target is a directory"}
		case !info.Mode().IsRegular():
			return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "target is not a regular file"}
		}
		// Preserve the existing file's mode through the replace.
		mode = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "inspecting target", Err: err}
	}

	tmpRel, err := writeTempSibling(root, dir, f.Bytes, mode)
	if err != nil {
		return err
	}
	if err := root.Rename(tmpRel, rel); err != nil {
		_ = root.Remove(tmpRel)
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "renaming into place", Err: err}
	}
	return syncRootDir(root, dir)
}

// deletePlannedFile removes a declared path after verifying its parents are real
// directories and the target itself is a present, non-symlink entry.
func deletePlannedFile(root *os.Root, rel string) error {
	dir := path.Dir(rel)
	if err := ensureParentDirs(root, dir, MutationDelete); err != nil {
		return err
	}
	info, err := root.Lstat(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "delete target is absent"}
		}
		return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "inspecting delete target", Err: err}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "delete target is a symlink"}
	}
	if err := root.Remove(rel); err != nil {
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "removing file", Err: err}
	}
	return syncRootDir(root, dir)
}

// ensureParentDirs walks dir component by component. Each prefix is Lstat-checked
// (so a symlink or non-directory component is refused rather than followed); a
// missing component is created only for a create mutation, and is a refusal for
// replace/delete. dir may be "." (a top-level file), in which case there is
// nothing to walk.
func ensureParentDirs(root *os.Root, dir string, kind MutationKind) error {
	if dir == "." || dir == "" {
		return nil
	}
	cur := ""
	for _, comp := range splitSlash(dir) {
		if cur == "" {
			cur = comp
		} else {
			cur = cur + "/" + comp
		}
		info, err := root.Lstat(cur)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if kind != MutationCreate {
					return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "parent directory is missing"}
				}
				if err := root.Mkdir(cur, createdParentDirMode); err != nil {
					return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "creating parent directory", Err: err}
				}
				continue
			}
			return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "inspecting parent component", Err: err}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "parent component is a symlink"}
		}
		if !info.IsDir() {
			return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "parent component is not a directory"}
		}
	}
	return nil
}

// splitSlash splits a non-empty slash path into its components. It exists so the
// walk never depends on the OS separator (repo paths are always slash-formed).
func splitSlash(p string) []string {
	var comps []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			comps = append(comps, p[start:i])
			start = i + 1
		}
	}
	comps = append(comps, p[start:])
	return comps
}

// writeTempSibling creates a uniquely named temp file in dir (within root),
// writes data, copies mode onto it (defeating the umask and carrying an
// executable bit), syncs, and closes it. It returns the temp file's root-relative
// path for the caller to rename into place.
func writeTempSibling(root *os.Root, dir string, data []byte, mode os.FileMode) (string, error) {
	for attempt := 0; attempt < 64; attempt++ {
		suffix, err := randomHex(8)
		if err != nil {
			return "", &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "generating temp name", Err: err}
		}
		name := ".docket-txn." + suffix + ".tmp"
		tmpRel := name
		if dir != "." && dir != "" {
			tmpRel = dir + "/" + name
		}
		file, err := root.OpenFile(tmpRel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "creating temp file", Err: err}
		}
		if err := writeSyncClose(file, data, mode); err != nil {
			_ = root.Remove(tmpRel)
			return "", err
		}
		return tmpRel, nil
	}
	return "", &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "exhausted temp file name attempts"}
}

// writeSyncClose writes data to file, forces its mode with an explicit fchmod
// (so an executable bit survives the ambient umask), fsyncs the bytes, and closes
// the descriptor.
func writeSyncClose(file *os.File, data []byte, mode os.FileMode) error {
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "writing temp file", Err: err}
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "setting temp file mode", Err: err}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "syncing temp file", Err: err}
	}
	if err := file.Close(); err != nil {
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "closing temp file", Err: err}
	}
	return nil
}

// syncRootDir fsyncs the directory dir (within root) so a rename into or a
// removal from it is durable across a crash. dir may be "." for the worktree
// root itself.
func syncRootDir(root *os.Root, dir string) error {
	name := dir
	if name == "" {
		name = "."
	}
	d, err := root.Open(name)
	if err != nil {
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "opening directory for sync", Err: err}
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "syncing directory", Err: err}
	}
	return nil
}

// randomHex returns n random bytes as a lowercase-hex string.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// verifyMaterialized re-reads every created/replaced path through an os.Root and
// requires exact byte equality with the plan, and requires every deleted path to
// be absent. It is the readback half of materialization: a create/replace whose
// on-disk bytes drift from the plan, or a delete that did not take, is a
// *Failure at stage "materialize".
func verifyMaterialized(worktree string, plan MutationPlan) error {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "opening worktree root", Err: err}
	}
	defer func() { _ = root.Close() }()

	for _, f := range plan.Files {
		rel := string(f.Path)
		switch f.Kind {
		case MutationCreate, MutationReplace:
			info, err := root.Lstat(rel)
			if err != nil {
				return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "materialized file is missing", Err: err}
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "materialized path is a symlink"}
			}
			if !info.Mode().IsRegular() {
				return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "materialized path is not a regular file"}
			}
			got, err := root.ReadFile(rel)
			if err != nil {
				return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "reading materialized file back", Err: err}
			}
			if !bytes.Equal(got, f.Bytes) {
				return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "materialized bytes differ from the plan"}
			}
		case MutationDelete:
			if _, err := root.Lstat(rel); err == nil {
				return &Failure{Stage: StageMaterialize, Kind: KindInvalidState, Detail: "deleted path is still present"}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return &Failure{Stage: StageMaterialize, Kind: KindExternal, Detail: "checking deleted path", Err: err}
			}
		}
	}
	return nil
}
