package codexcontract

import "errors"

// ErrRootIdentityUnsupported reports a platform where Docket cannot bind a
// worktree path to a stable filesystem object.
var ErrRootIdentityUnsupported = errors.New("root identity is unsupported on this platform")

// RootIdentity records the feature directory's Device/Inode and the canonical
// per-worktree GitDir path. Device/Inode do not describe the Git directory.
type RootIdentity struct {
	Platform string `json:"platform"`
	Device   uint64 `json:"device"`
	Inode    uint64 `json:"inode"`
	GitDir   string `json:"git_dir"`
}

func (r RootIdentity) Equal(other RootIdentity) bool {
	return r.Platform == other.Platform && r.Device == other.Device && r.Inode == other.Inode && r.GitDir == other.GitDir
}
