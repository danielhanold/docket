// Repository execution-identity fingerprint for the gate driver.
//
// A PASSED drive certifies the exact repository bytes present when the drive
// began (spec Constraint 5, "Persisted execution identity"): HEAD, staged
// changes, unstaged changes, untracked files, modes, deletion state, rename
// state, and symlink values cannot drift across a continuation. This file
// computes a Fingerprint of those dimensions so the driver can recompute it at
// every ownership boundary and before accepting a terminal pass, and HALT (never
// go red) on any drift.
//
// The fingerprint stores per-dimension HASHES and structural metadata only —
// never file or diff content — so the persisted drive record can never become a
// cache of ambient repository content. Symlinks are hashed BY LINK VALUE and are
// never followed, so a symlink retarget is detected and a dangling symlink still
// fingerprints.
package gatedrive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// GitSeam abstracts the read-only git queries ComputeFingerprint needs, so tests
// can inject a deterministic implementation and production uses realGit. None of
// these queries mutate the repository; each returns raw git output whose bytes
// carry object ids, modes, and paths — never file content.
type GitSeam interface {
	// HeadOID returns HEAD's full object id, or an empty string on an unborn
	// branch (no commit yet). A read failure is returned as an error.
	HeadOID(repoDir string) (string, error)
	// IndexEntries returns `git ls-files --stage -z`: mode, object id, stage, and
	// path for every index entry. It captures staged content (via object id),
	// index-side file modes, and the staged path set (so a staged rename or
	// deletion moves it).
	IndexEntries(repoDir string) ([]byte, error)
	// Status returns `git status --porcelain=v2 -z --untracked-files=all`,
	// capturing per-path staged/unstaged status codes, rename state (including
	// score and both paths), and deletion state.
	Status(repoDir string) ([]byte, error)
	// WorktreePaths returns `git ls-files -z --cached --others --exclude-standard`:
	// the union of tracked and untracked, non-ignored paths, NUL-separated. It is
	// the enumeration ComputeFingerprint walks to hash live worktree bytes,
	// modes, and symlink values.
	WorktreePaths(repoDir string) ([]byte, error)
}

// Fingerprint is the per-dimension digest of a repository's execution identity.
// Every field is a hash or a structural scalar — never content — so two
// fingerprints can be compared for exact-identity without retaining any bytes of
// the worktree. Field names are the structural dimensions; drift in any one
// makes Equal report false.
type Fingerprint struct {
	// Head is HEAD's full object id (empty on an unborn branch).
	Head string `json:"head"`
	// Index is the sha256 of `git ls-files --stage` output: staged content,
	// index modes, and the staged path set.
	Index string `json:"index"`
	// Status is the sha256 of `git status --porcelain=v2` output: staged/unstaged
	// codes, rename state, and deletion state.
	Status string `json:"status"`
	// Worktree is the sha256 of the canonical, path-sorted record of every
	// non-ignored worktree path's kind, mode, and content/link-value hash.
	Worktree string `json:"worktree"`
	// Entries is the count of worktree paths folded into Worktree — structural
	// metadata that makes an added or removed path visible without exposing any
	// path or content.
	Entries int `json:"entries"`
}

// Equal reports whether two fingerprints certify the identical repository
// execution identity across every dimension. Because every field is a comparable
// scalar, exact struct equality is the identity test the driver keys on.
func (a Fingerprint) Equal(b Fingerprint) bool { return a == b }

// ComputeFingerprint builds the repository execution-identity fingerprint for
// repoDir using the injected git seam for the structural git reads and a direct,
// symlink-safe filesystem walk for live worktree bytes. It stores only hashes
// and structural counts; it never retains file or diff content, and it never
// follows a symlink (each is hashed by its link value via os.Lstat/os.Readlink),
// so a dangling symlink still fingerprints.
func ComputeFingerprint(repoDir string, git GitSeam) (Fingerprint, error) {
	head, err := git.HeadOID(repoDir)
	if err != nil {
		return Fingerprint{}, fmt.Errorf("gatedrive: fingerprint head: %w", err)
	}
	index, err := git.IndexEntries(repoDir)
	if err != nil {
		return Fingerprint{}, fmt.Errorf("gatedrive: fingerprint index: %w", err)
	}
	status, err := git.Status(repoDir)
	if err != nil {
		return Fingerprint{}, fmt.Errorf("gatedrive: fingerprint status: %w", err)
	}
	pathsRaw, err := git.WorktreePaths(repoDir)
	if err != nil {
		return Fingerprint{}, fmt.Errorf("gatedrive: fingerprint worktree paths: %w", err)
	}

	worktreeHash, entries, err := hashWorktree(repoDir, splitNUL(pathsRaw))
	if err != nil {
		return Fingerprint{}, err
	}

	return Fingerprint{
		Head:     head,
		Index:    hashBytes(index),
		Status:   hashBytes(status),
		Worktree: worktreeHash,
		Entries:  entries,
	}, nil
}

// hashWorktree walks the given repo-relative paths under repoDir and folds each
// one's kind, mode, and content/link-value hash into a single canonical digest.
// Paths are sorted so the digest is independent of git's enumeration order.
// Symlinks are read by value (never followed); regular files are hashed by their
// bytes; a path listed in the index but absent from disk (an unstaged deletion)
// folds in as a deletion marker so removal is detected.
func hashWorktree(repoDir string, paths []string) (string, int, error) {
	sort.Strings(paths)
	h := sha256.New()
	count := 0
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		kind, mode, contentHash, err := fingerprintPath(repoDir, rel)
		if err != nil {
			return "", 0, err
		}
		// Canonical, NUL-delimited record: path, kind, mode, content/value hash.
		// The kind byte prefixes the content hash's own domain (file vs link) so
		// a regular file whose bytes equal a symlink's target cannot collide.
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00", rel, kind, mode, contentHash)
		count++
	}
	return hex.EncodeToString(h.Sum(nil)), count, nil
}

// fingerprintPath classifies one repo-relative worktree path and returns its
// kind, mode string, and content/link-value hash. It uses os.Lstat so a symlink
// is inspected as a link rather than followed, and treats a missing path as a
// deletion rather than an error.
func fingerprintPath(repoDir, rel string) (kind, mode, contentHash string, err error) {
	abs := filepath.Join(repoDir, rel)
	fi, lerr := os.Lstat(abs)
	if lerr != nil {
		if os.IsNotExist(lerr) {
			// Index-tracked path removed from the worktree: an unstaged deletion.
			return "D", "", "", nil
		}
		return "", "", "", fmt.Errorf("gatedrive: fingerprint lstat %q: %w", rel, lerr)
	}

	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		target, rerr := os.Readlink(abs)
		if rerr != nil {
			return "", "", "", fmt.Errorf("gatedrive: fingerprint readlink %q: %w", rel, rerr)
		}
		// Hashed BY LINK VALUE; the link is never followed, so a dangling target
		// is fine. Symlink permission bits are not portable and git ignores them,
		// so the value alone is the identity.
		return "L", "", hashBytes([]byte(target)), nil
	case fi.Mode().IsRegular():
		content, rerr := os.ReadFile(abs)
		if rerr != nil {
			return "", "", "", fmt.Errorf("gatedrive: fingerprint read %q: %w", rel, rerr)
		}
		return "F", octalPerm(fi.Mode()), hashBytes(content), nil
	default:
		// A gitlink (submodule) or other non-file entry: record its kind and mode
		// without content. Its identity still moves if it appears or disappears.
		return "O", octalPerm(fi.Mode()), "", nil
	}
}

// octalPerm renders the permission bits of a mode as a fixed 4-digit octal
// string, so a 0644->0755 executable-bit change is visible in the digest.
func octalPerm(m os.FileMode) string {
	return "0" + strconv.FormatUint(uint64(m.Perm()), 8)
}

// hashBytes returns the hex sha256 of b. It is the single hashing primitive for
// every dimension, so no dimension can accidentally retain raw bytes.
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// splitNUL splits NUL-separated git output into records, dropping the empty
// trailing record left by the final terminator.
func splitNUL(b []byte) []string {
	parts := strings.Split(string(b), "\x00")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// realGit is the production GitSeam. It shells out to the git executable in the
// target directory under a sanitized, non-interactive environment; each query is
// read-only and returns raw output whose bytes carry object ids, modes, and
// paths — never file content.
type realGit struct{}

// runGit executes one read-only git command in repoDir and returns its stdout.
func (realGit) runGit(repoDir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	// Non-interactive, no optional locks, no system config surprises. This is a
	// read path, so no repository-redirection or credential variables are set.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gatedrive: git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (g realGit) HeadOID(repoDir string) (string, error) {
	// --verify fails on an unborn branch; treat that as "no HEAD yet", not an
	// error, so a fresh repository still fingerprints.
	out, err := g.runGit(repoDir, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (g realGit) IndexEntries(repoDir string) ([]byte, error) {
	return g.runGit(repoDir, "ls-files", "--stage", "-z")
}

func (g realGit) Status(repoDir string) ([]byte, error) {
	return g.runGit(repoDir, "status", "--porcelain=v2", "-z", "--untracked-files=all")
}

func (g realGit) WorktreePaths(repoDir string) ([]byte, error) {
	return g.runGit(repoDir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
}
