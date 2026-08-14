// Package install owns docket's user-level installation: where its data lives,
// what it has installed, and how that state is published. This file owns root
// resolution only — it reads the environment through injected seams and never
// creates anything on disk.
package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UserRoots are the absolute user-level directories every installation
// operation works within. A root may not exist yet; resolution never creates
// one.
type UserRoots struct {
	Home       string // validated absolute home
	DataRoot   string // <XDG_DATA_HOME|~/.local/share>/docket
	ConfigHome string // XDG_CONFIG_HOME or ~/.config (for opencode)
	BinDir     string // XDG_BIN_HOME or ~/.local/bin (development mode)
}

// ResolveRoots reads the environment through the injected getenv func and the
// home directory through homeFn; production passes os.UserHomeDir and
// os.Getenv. Tests inject both, so no test can reach the developer's real
// home.
//
// An XDG value is honored only when it is set AND absolute — a relative XDG
// value would otherwise anchor an installation to whatever the process's
// working directory happened to be. Each root that already exists must be a
// directory; a root that does not exist yet is fine.
func ResolveRoots(homeFn func() (string, error), getenv func(string) string) (UserRoots, error) {
	if homeFn == nil {
		return UserRoots{}, errors.New("install: ResolveRoots requires a home function")
	}
	if getenv == nil {
		return UserRoots{}, errors.New("install: ResolveRoots requires a getenv function")
	}

	home, err := homeFn()
	if err != nil {
		return UserRoots{}, fmt.Errorf("install: resolving home directory: %w", err)
	}
	if home == "" {
		return UserRoots{}, errors.New("install: home directory is empty")
	}
	if !filepath.IsAbs(home) {
		return UserRoots{}, fmt.Errorf("install: home directory %q is not absolute", home)
	}
	home = filepath.Clean(home)

	roots := UserRoots{
		Home:       home,
		DataRoot:   filepath.Join(xdgOr(getenv, "XDG_DATA_HOME", filepath.Join(home, ".local", "share")), "docket"),
		ConfigHome: xdgOr(getenv, "XDG_CONFIG_HOME", filepath.Join(home, ".config")),
		BinDir:     xdgOr(getenv, "XDG_BIN_HOME", filepath.Join(home, ".local", "bin")),
	}

	for _, p := range []string{roots.Home, roots.DataRoot, roots.ConfigHome, roots.BinDir} {
		if err := requireDirIfPresent(p); err != nil {
			return UserRoots{}, err
		}
	}
	return roots, nil
}

// xdgOr returns the named XDG variable when it is set and absolute, else the
// fallback.
func xdgOr(getenv func(string) string, name, fallback string) string {
	if v := getenv(name); v != "" && filepath.IsAbs(v) {
		return filepath.Clean(v)
	}
	return fallback
}

func requireDirIfPresent(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // not installed yet
		}
		return fmt.Errorf("install: inspecting root %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("install: root %q exists but is not a directory", path)
	}
	return nil
}

// VersionsDir holds one immutable extracted asset tree per asset set.
func (r UserRoots) VersionsDir() string { return filepath.Join(r.DataRoot, "versions") }

// VersionDir is the assets directory of one immutable extracted version.
func (r UserRoots) VersionDir(assetSetID string) string {
	return filepath.Join(r.VersionsDir(), sanitizeSegment(assetSetID), "assets")
}

// TransactionsDir holds one journal directory per in-flight transaction.
func (r UserRoots) TransactionsDir() string { return filepath.Join(r.DataRoot, "transactions") }

// StatePath is the published ownership manifest.
func (r UserRoots) StatePath() string { return filepath.Join(r.DataRoot, "state", "install.json") }

// LockPath is the file carrying the exclusive installation lock. It sits at the
// root of the data tree because it serializes every mutation under it; see
// lock.go for why the lock is an flock rather than the file's existence.
func (r UserRoots) LockPath() string { return filepath.Join(r.DataRoot, installLockName) }

// sanitizeSegment turns an asset-set identifier into exactly one safe path
// segment. The identifier's own shape ("sha256:<hex>") is not trusted to stay
// that way: anything outside the portable set becomes "-", and a segment that
// would still traverse or vanish is prefixed.
func sanitizeSegment(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	seg := b.String()
	if seg == "" || seg == "." || seg == ".." {
		seg = "_" + seg
	}
	return seg
}
