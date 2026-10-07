// Package layout owns every per-repo spelling docket writes and decides the
// visibility mode from STATE (a <git-common-dir>/dckt/ directory means private).
//
// It is a stdlib-only leaf: it imports no internal package, because gitcli and
// config import it.
package layout

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Mode is a repository's visibility mode.
type Mode string

const (
	Shared  Mode = "shared"
	Private Mode = "private"

	SharedName           = "docket"
	PrivateName          = "dckt"
	SharedRemote         = "origin"
	SharedWorktreeDir    = ".docket"
	PrivateConfigFile    = "config.yml"
	PrivateConfigDisplay = ".git/dckt/config.yml"

	// PrivateInstructionsFile is a private repository's parent-facing rules
	// file beneath <common>/dckt; PrivateInstructionsDisplay is its
	// user-facing spelling.
	PrivateInstructionsFile    = "AGENTS.md"
	PrivateInstructionsDisplay = ".git/dckt/AGENTS.md"

	// PrivateLocalKeysFile is the file beneath <common>/dckt that keeps a
	// repository's .docket.local.yml while it is private, so going shared
	// again can return those keys to the clone-local layer.
	PrivateLocalKeysFile = "local-keys.yml"
)

// Detect decides the mode from state; a probe error is returned, never guessed.
func Detect(commonDir string) (Mode, error) {
	p := filepath.Join(commonDir, PrivateName)
	fi, err := os.Lstat(p)
	switch {
	case err == nil && fi.IsDir():
		return Private, nil
	case err == nil:
		return "", fmt.Errorf("layout: %s exists but is not a directory", p)
	case errors.Is(err, fs.ErrNotExist):
		return Shared, nil
	default:
		return "", fmt.Errorf("layout: probing %s: %w", p, err)
	}
}

// StateName serves path helpers with no error channel. It fails CLOSED:
// anything present (or unprobeable) at <common>/dckt selects the private name,
// so a write beneath it fails loudly instead of landing in a docket-named
// folder. Mode-deciding callers use Detect.
func StateName(commonDir string) string {
	if _, err := os.Lstat(filepath.Join(commonDir, PrivateName)); errors.Is(err, fs.ErrNotExist) {
		return SharedName
	}
	return PrivateName
}

// StateDirOf is the per-repo state folder beneath the git common dir.
func StateDirOf(commonDir string) string { return filepath.Join(commonDir, StateName(commonDir)) }

// PrivateConfigPath is the private repository-layer config file.
func PrivateConfigPath(commonDir string) string {
	return filepath.Join(commonDir, PrivateName, PrivateConfigFile)
}

// PrivateInstructionsPath is a private repository's parent-facing rules file,
// <commonDir>/dckt/AGENTS.md.
func PrivateInstructionsPath(commonDir string) string {
	return filepath.Join(commonDir, PrivateName, PrivateInstructionsFile)
}

// PrivateLocalKeysPath is the saved .docket.local.yml of a private
// repository, <commonDir>/dckt/local-keys.yml.
func PrivateLocalKeysPath(commonDir string) string {
	return filepath.Join(commonDir, PrivateName, PrivateLocalKeysFile)
}

// CommonDirOf resolves a working-tree root's common dir from the filesystem
// alone (no git process). ok=false when <root>/.git does not exist.
//
// A <root>/.git directory is the common dir. A .git file names a gitdir on its
// first "gitdir:" line (relative paths resolve against root); that gitdir's
// commondir file (relative paths resolve against the gitdir) names the common
// dir, and a gitdir with no commondir file (a submodule) is its own common dir.
// Every read error other than not-exist is returned.
func CommonDirOf(root string) (string, bool, error) {
	dotGit := filepath.Join(root, ".git")
	fi, err := os.Stat(dotGit)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("layout: probing %s: %w", dotGit, err)
	}
	if fi.IsDir() {
		return dotGit, true, nil
	}

	gitdir, err := readGitdirLine(dotGit)
	if err != nil {
		return "", false, err
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(root, gitdir)
	}
	gitdir = filepath.Clean(gitdir)

	raw, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if errors.Is(err, fs.ErrNotExist) {
		return gitdir, true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("layout: reading commondir of %s: %w", gitdir, err)
	}
	common := strings.TrimSpace(string(raw))
	if common == "" {
		return "", false, fmt.Errorf("layout: %s/commondir is empty", gitdir)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitdir, common)
	}
	return filepath.Clean(common), true, nil
}

// readGitdirLine returns the path on the first "gitdir:" line of a .git file.
func readGitdirLine(dotGit string) (string, error) {
	f, err := os.Open(dotGit)
	if err != nil {
		return "", fmt.Errorf("layout: reading %s: %w", dotGit, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "gitdir:"); ok {
			if p := strings.TrimSpace(rest); p != "" {
				return p, nil
			}
			break
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("layout: reading %s: %w", dotGit, err)
	}
	return "", fmt.Errorf("layout: %s names no gitdir", dotGit)
}

// DataHome is ${XDG_DATA_HOME:-$HOME/.local/share}; XDG_DATA_HOME counts only
// when absolute (the installer's rule in install.ResolveRoots). No XDG and a
// home error (or empty home) → error.
func DataHome(getenv func(string) string, home func() (string, error)) (string, error) {
	if x := getenv("XDG_DATA_HOME"); x != "" && filepath.IsAbs(x) {
		return x, nil
	}
	h, err := home()
	if err != nil {
		return "", fmt.Errorf("layout: resolving the home directory: %w", err)
	}
	if h == "" {
		return "", errors.New("layout: resolving the home directory: empty home")
	}
	return filepath.Join(h, ".local", "share"), nil
}

// OwnerRepo derives <owner>-<repo> from origin's URL: the last two segments
// (split on / : \, trailing "/" and ".git" removed), lowercased, with
// non-alphanumeric runs collapsed to "-".
func OwnerRepo(originURL string) (string, error) {
	s := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(originURL), "/"), ".git")
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == ':' || r == '\\' })
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.Join(parts, "-")) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		return "", fmt.Errorf("layout: cannot derive an owner/repo name from origin URL %q", originURL)
	}
	return slug, nil
}

// CloneID is <basename>-<first 8 hex of sha256(abs path)>; the basename keeps
// [A-Za-z0-9._-] and maps anything else to '-'.
func CloneID(primaryWorktree string) string {
	sum := sha256.Sum256([]byte(primaryWorktree))
	base := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, filepath.Base(primaryWorktree))
	return base + "-" + hex.EncodeToString(sum[:])[:8]
}

// Layout is one repository's resolved per-repo spellings.
type Layout struct {
	Mode                                                       Mode
	StateDir, MetadataRemote, MetadataBranch, MetadataWorktree string
	ConfigPath, StoreDir, DefaultBareRemote, CheckoutsDir      string // private only
}

// SharedLayout is the layout of a shared repository: the docket branch on
// origin, checked out at <primary>/.docket.
func SharedLayout(commonDir, primaryWorktree string) Layout {
	return Layout{
		Mode:             Shared,
		StateDir:         filepath.Join(commonDir, SharedName),
		MetadataRemote:   SharedRemote,
		MetadataBranch:   SharedName,
		MetadataWorktree: filepath.Join(primaryWorktree, SharedWorktreeDir),
	}
}

// PrivateLayout is the layout of a private repository: the dckt branch on a
// bare dckt remote under <dataHome>/dckt/<ownerRepo>, with this clone's
// metadata checkout under the store's checkouts folder.
func PrivateLayout(commonDir, primaryWorktree, dataHome, ownerRepo string) Layout {
	store := filepath.Join(dataHome, PrivateName, ownerRepo)
	checkouts := filepath.Join(store, "checkouts")
	return Layout{
		Mode:              Private,
		StateDir:          filepath.Join(commonDir, PrivateName),
		MetadataRemote:    PrivateName,
		MetadataBranch:    PrivateName,
		MetadataWorktree:  filepath.Join(checkouts, CloneID(primaryWorktree)),
		ConfigPath:        PrivateConfigPath(commonDir),
		StoreDir:          store,
		DefaultBareRemote: filepath.Join(store, "remote.git"),
		CheckoutsDir:      checkouts,
	}
}

// MetadataRef is the local metadata branch ref.
func (l Layout) MetadataRef() string { return "refs/heads/" + l.MetadataBranch }

// TrackingRef is the remote-tracking ref of the metadata branch.
func (l Layout) TrackingRef() string {
	return "refs/remotes/" + l.MetadataRemote + "/" + l.MetadataBranch
}
