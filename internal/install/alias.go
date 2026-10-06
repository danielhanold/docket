package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Every install also places a second, shorter name for the binary beside it:
// AliasName, a symlink to the installed docket. It follows the binary — same
// directory, same ownership posture, left in place by uninstall — and a dckt
// docket did not create is never overwritten. A missing or foreign alias is a
// finding, never a failure: the binary is installed either way.
const (
	AliasName = "dckt"
	// roleBinaryAlias marks the development install's alias in the installed
	// state. Like roleBinary it belongs to the installation, never a harness.
	roleBinaryAlias = "binary-alias"
)

// The alias finding kinds.
const (
	AliasMissing = "missing"
	AliasForeign = "foreign"
)

// AliasFinding is one alias that is not what an install would leave: absent,
// or occupied by something that does not resolve to the binary.
type AliasFinding struct {
	Kind   string
	Path   string
	Binary string
	Remedy string
}

const (
	remedyAliasMissing = "re-run the installer that placed docket here (the release install.sh, or docket development install) to create it"
	remedyAliasForeign = "docket did not create what is at this path and will not replace it; move or delete it, then re-run the installer to get the dckt alias"
)

// AliasPathFor is where the alias for binary lives: beside it.
func AliasPathFor(binary string) string {
	return filepath.Join(filepath.Dir(binary), AliasName)
}

// InspectBinaryAlias classifies the alias beside binary. A nil finding means a
// symlink that resolves — every hop canonicalised — to binary. Absence is
// AliasMissing; anything else at the path (a link elsewhere, a dangling link,
// a file, a directory) is AliasForeign. It only reads. An error means the
// probe itself failed, which is never reported as a clean absence.
func InspectBinaryAlias(binary string) (*AliasFinding, error) {
	path := AliasPathFor(binary)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &AliasFinding{Kind: AliasMissing, Path: path, Binary: binary, Remedy: remedyAliasMissing}, nil
	case err != nil:
		return nil, fmt.Errorf("install: inspecting %s: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		have, err := linkDestination(path)
		if err != nil {
			return nil, err
		}
		want, err := canonicalPath(binary)
		if err != nil {
			return nil, err
		}
		if have == want {
			if _, err := os.Stat(path); err == nil {
				return nil, nil
			}
		}
	}
	return &AliasFinding{Kind: AliasForeign, Path: path, Binary: binary, Remedy: remedyAliasForeign}, nil
}
