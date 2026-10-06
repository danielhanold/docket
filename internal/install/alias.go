package install

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
// a file, a directory, a link that cannot even be resolved) is AliasForeign. It
// only reads. An error means the probe itself failed — the alias path cannot be
// examined, or the binary's own path cannot be canonicalised — which is never
// reported as a clean absence.
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
		want, err := canonicalPath(binary)
		if err != nil {
			return nil, err
		}
		// Something is at the path; a link docket cannot resolve (a loop, a
		// hop through a file, an unreadable directory) is not provably
		// docket's, so it is foreign rather than a failed probe.
		if have, err := linkDestination(path); err == nil && have == want {
			if _, err := os.Stat(path); err == nil {
				return nil, nil
			}
		}
	}
	return &AliasFinding{Kind: AliasForeign, Path: path, Binary: binary, Remedy: remedyAliasForeign}, nil
}

// planBinaryAlias is the development install's alias step: the symlink target
// it would own beside binary, or — when what is there is not provably docket's
// — no target and a foreign finding. It inspects with the same ownership proofs
// every other target gets, so "already owned" means exactly what it means for a
// harness link: a link already resolving to binary, or one the prior state
// recorded and that still matches its record. A conflict is reported rather
// than returned as a refusal: the binary install must never be failed by the
// alias.
//
// Only a failure to examine the alias path, or to canonicalise the binary's
// own path, is an error. Once something is known to be at the path, any
// failure resolving or reading it (a looping link, a hop through a file, an
// unreadable directory or file) is a foreign finding: what docket cannot
// resolve it cannot prove it owns.
func planBinaryAlias(binary string, prior *State) (*Target, *AliasFinding, error) {
	t := Target{Path: AliasPathFor(binary), Kind: KindSymlink, LinkTarget: binary, Role: roleBinaryAlias}
	foreign := &AliasFinding{Kind: AliasForeign, Path: t.Path, Binary: binary, Remedy: remedyAliasForeign}
	_, lerr := os.Lstat(t.Path)
	if lerr != nil && !errors.Is(lerr, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("install: inspecting %s: %w", t.Path, lerr)
	}
	if _, err := canonicalPath(binary); err != nil {
		return nil, nil, err
	}
	inspection, err := InspectTarget(t, prior, nil)
	if err != nil {
		if lerr == nil {
			return nil, foreign, nil
		}
		return nil, nil, err
	}
	if inspection.Disposition == DispositionConflict {
		return nil, foreign, nil
	}
	return &t, nil, nil
}

// readReleaseBinaryPath returns the absolute binary path the release
// downloader's record names, or "" when there is no record or it names no
// usable absolute path — the downloader itself grants no ownership to such a
// record. A record that exists but cannot be read is an error, not an absence.
func readReleaseBinaryPath(recordPath string) (string, error) {
	if recordPath == "" {
		return "", nil
	}
	f, err := os.Open(recordPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("install: reading %s: %w", recordPath, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "path="); ok {
			if filepath.IsAbs(v) {
				return filepath.Clean(v), nil
			}
			return "", nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("install: reading %s: %w", recordPath, err)
	}
	return "", nil
}

// checkBinaryAliases classifies the alias beside every binary an installer
// owns: the development installation's recorded binary, and the binary the
// release downloader's record names. A binary that is absent is skipped: its
// alias cannot be judged against nothing. It only reads.
func checkBinaryAliases(state *State, roots UserRoots) ([]AliasFinding, error) {
	var binaries []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			binaries = append(binaries, p)
		}
	}
	if state != nil {
		for _, rec := range state.Targets {
			if rec.Role == roleBinary {
				add(rec.Path)
			}
		}
	}
	release, err := readReleaseBinaryPath(roots.ReleaseBinaryRecordPath())
	if err != nil {
		return nil, err
	}
	add(release)

	var findings []AliasFinding
	for _, binary := range binaries {
		// A binary that has vanished is binary drift (or a re-install) to
		// report; docket's own link to it resolving to nothing is no evidence
		// that someone else put it there, so the alias is not probed at all.
		if _, err := os.Lstat(binary); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("install: inspecting %s: %w", binary, err)
		}
		f, err := InspectBinaryAlias(binary)
		if err != nil {
			return nil, err
		}
		if f != nil {
			findings = append(findings, *f)
		}
	}
	return findings, nil
}
