// Package gitcli is a typed adapter over controlled Git command execution:
// repository discovery, remote inspection, targeted fetch, and an immutable
// revision-pinned object source with NUL-safe batch reads. Git execution is
// package-private; no exported generic command runner is offered.
package gitcli

import (
	"errors"
	"strings"
)

// ObjectID is a normalized full-hex Git object id (SHA-1 or SHA-256 length),
// compared by exact string equality; never truncated.
type ObjectID string

// RemoteName is a configured remote name (e.g. "origin").
type RemoteName string

// RefName is a fully qualified ref (refs/heads/..., refs/remotes/...).
type RefName string

// RepoPath is a repo-relative byte path carried as opaque bytes in a Go string.
type RepoPath string

// Revision pins one exact commit reached through a remote and ref.
type Revision struct {
	Commit ObjectID
	Remote RemoteName
	Ref    RefName
}

// FileMode is a Git octal mode text: "100644", "100755", "120000", "160000", "040000".
type FileMode string

// ObjectType is a Git object type: "blob", "tree", "commit".
type ObjectType string

// TreeEntry is one entry from a tree listing.
type TreeEntry struct {
	Path     RepoPath
	Mode     FileMode
	Type     ObjectType
	ObjectID ObjectID
}

// BlobResult is the outcome of reading one requested path.
type BlobResult struct {
	Path  RepoPath
	Found bool
	Blob  Blob
}

// Blob holds an object's mode, id, and exact bytes.
type Blob struct {
	Mode     FileMode
	ObjectID ObjectID
	Bytes    []byte
}

// validateRepoPath rejects NUL bytes, a leading "/", a trailing "/", empty
// components ("a//b"), "." and ".." components anywhere, and the empty string —
// except when allowEmptyRootPrefix is true, where "" is legal (root tree
// listing only).
func validateRepoPath(p RepoPath, allowEmptyRootPrefix bool) error {
	s := string(p)
	if s == "" {
		if allowEmptyRootPrefix {
			return nil
		}
		return errors.New("gitcli: empty repo path")
	}
	if strings.IndexByte(s, 0) >= 0 {
		return errors.New("gitcli: repo path contains NUL")
	}
	if strings.HasPrefix(s, "/") {
		return errors.New("gitcli: repo path has leading slash")
	}
	if strings.HasSuffix(s, "/") {
		return errors.New("gitcli: repo path has trailing slash")
	}
	for _, c := range strings.Split(s, "/") {
		if c == "" {
			return errors.New("gitcli: repo path has empty component")
		}
		if c == "." || c == ".." {
			return errors.New("gitcli: repo path has . or .. component")
		}
	}
	return nil
}

// validateRemoteName requires a non-empty name and rejects NUL, whitespace, a
// leading "-" (option smuggling), and "/".
func validateRemoteName(r RemoteName) error {
	s := string(r)
	if s == "" {
		return errors.New("gitcli: empty remote name")
	}
	if strings.HasPrefix(s, "-") {
		return errors.New("gitcli: remote name has leading dash")
	}
	if strings.ContainsAny(s, "/") {
		return errors.New("gitcli: remote name contains slash")
	}
	if strings.IndexByte(s, 0) >= 0 {
		return errors.New("gitcli: remote name contains NUL")
	}
	if strings.ContainsAny(s, " \t\r\n\v\f") {
		return errors.New("gitcli: remote name contains whitespace")
	}
	return nil
}

// validateRefName requires a "refs/"-prefixed name with at least two
// components and rejects NUL, whitespace, a leading "-", empty components,
// "."/".." components, "@{", "\\", a trailing ".lock" component, and "*".
func validateRefName(r RefName) error {
	s := string(r)
	if s == "" {
		return errors.New("gitcli: empty ref name")
	}
	if strings.HasPrefix(s, "-") {
		return errors.New("gitcli: ref name has leading dash")
	}
	if strings.IndexByte(s, 0) >= 0 {
		return errors.New("gitcli: ref name contains NUL")
	}
	if strings.ContainsAny(s, " \t\r\n\v\f") {
		return errors.New("gitcli: ref name contains whitespace")
	}
	if strings.Contains(s, "@{") {
		return errors.New("gitcli: ref name contains @{")
	}
	if strings.Contains(s, "\\") {
		return errors.New("gitcli: ref name contains backslash")
	}
	if strings.Contains(s, "*") {
		return errors.New("gitcli: ref name contains *")
	}
	if !strings.HasPrefix(s, "refs/") {
		return errors.New("gitcli: ref name must begin refs/")
	}
	comps := strings.Split(s, "/")
	if len(comps) < 3 {
		return errors.New("gitcli: ref name needs at least two components after refs/")
	}
	for _, c := range comps {
		if c == "" {
			return errors.New("gitcli: ref name has empty component")
		}
		if c == "." || c == ".." {
			return errors.New("gitcli: ref name has . or .. component")
		}
		if strings.HasPrefix(c, ".") {
			return errors.New("gitcli: ref name has component with leading dot")
		}
		if strings.Contains(c, "..") {
			return errors.New("gitcli: ref name has .. sequence")
		}
		if strings.HasSuffix(c, ".lock") {
			return errors.New("gitcli: ref name has .lock component")
		}
	}
	return nil
}

// validateObjectID requires a non-empty, all-lowercase-hex id of length 40 or
// 64 (SHA-1 and SHA-256 full representation; never truncated).
func validateObjectID(id ObjectID) error {
	s := string(id)
	if s == "" {
		return errors.New("gitcli: empty object id")
	}
	if len(s) != 40 && len(s) != 64 {
		return errors.New("gitcli: object id must be 40 or 64 hex chars")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return errors.New("gitcli: object id has non-lowercase-hex char")
		}
	}
	return nil
}
