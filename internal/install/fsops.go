package install

import "os"

// FSOps is the mutation seam every installation write passes through. It exists
// so a test can refuse one operation in the middle of a transaction and observe
// what the engine does next: rollback is the whole safety story of an installer
// that spans harness directories no single rename can cover, and a rollback
// nobody has watched happen is decoration.
//
// Reads are deliberately absent. Only mutations need injecting, and routing
// reads through the seam would invite a fake filesystem that disagrees with the
// real one about what is on disk.
type FSOps interface {
	WriteFile(path string, data []byte, mode os.FileMode) error
	Rename(old, new string) error
	Symlink(target, path string) error
	Remove(path string) error
	MkdirAll(path string, mode os.FileMode) error
}

// RealFS is the production implementation: the stdlib, unadorned.
//
// It does not fsync. The interruption this change defends against is process
// death — the journal is written and renamed before any target is touched, so
// the next run finds it — and defending against power loss instead would need
// durability guarantees at every step, which is a different design with a
// different cost. The boundary is stated here rather than half-implemented.
type RealFS struct{}

func (RealFS) WriteFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}

func (RealFS) Rename(old, new string) error { return os.Rename(old, new) }

func (RealFS) Symlink(target, path string) error { return os.Symlink(target, path) }

func (RealFS) Remove(path string) error { return os.Remove(path) }

func (RealFS) MkdirAll(path string, mode os.FileMode) error { return os.MkdirAll(path, mode) }
