package process

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ensurePrivateDir creates path (and parents) and enforces 0700 with an
// explicit chmod — a create-time mode is a request the umask can mask.
func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return failf(FailExternal, "ensure-dir", "creating %s: %v", filepath.Base(path), err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return failf(FailExternal, "ensure-dir", "chmod 0700 %s: %v", filepath.Base(path), err)
	}
	return nil
}

// writeAtomicJSON writes v as JSON at path via a same-directory temp file,
// fsync, chmod 0600, atomic rename, and directory fsync. A reader sees a
// complete old or new document, never a partial one.
func writeAtomicJSON(path string, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return failf(FailExternal, "atomic-write", "encoding %s: %v", filepath.Base(path), err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return failf(FailExternal, "atomic-write", "temp for %s: %v", filepath.Base(path), err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return failf(FailExternal, "atomic-write", "writing %s: %v", filepath.Base(path), err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return failf(FailExternal, "atomic-write", "syncing %s: %v", filepath.Base(path), err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return failf(FailExternal, "atomic-write", "chmod %s: %v", filepath.Base(path), err)
	}
	if err := tmp.Close(); err != nil {
		return failf(FailExternal, "atomic-write", "closing %s: %v", filepath.Base(path), err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return failf(FailExternal, "atomic-write", "renaming into %s: %v", filepath.Base(path), err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return failf(FailExternal, "atomic-write", "opening dir for sync: %v", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return failf(FailExternal, "atomic-write", "dir sync: %v", err)
	}
	return nil
}
