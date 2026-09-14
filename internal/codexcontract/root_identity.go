package codexcontract

import "errors"

// ErrRootIdentityUnsupported reports a platform where Docket cannot bind a
// worktree path to a stable filesystem object.
var ErrRootIdentityUnsupported = errors.New("root identity is unsupported on this platform")

// RootIdentity binds the canonical feature root and its per-worktree Git dir
// to the filesystem object observed during preparation.
type RootIdentity struct {
	Platform string `json:"platform"`
	Device   uint64 `json:"device"`
	Inode    uint64 `json:"inode"`
	GitDir   string `json:"git_dir"`
}

func (r RootIdentity) Equal(other RootIdentity) bool {
	return r.Platform == other.Platform && r.Device == other.Device && r.Inode == other.Inode && r.GitDir == other.GitDir
}
