// Package transaction is the semantic-operation transaction engine: it executes
// durable Docket metadata mutations inside private detached worktrees, validates
// complete before/after state, commits only declared paths, and pushes under an
// exact expected-ref lease. This file holds the request/plan vocabulary and all
// input validation the engine runs before any Git or filesystem work.
package transaction

import (
	"errors"
	"unicode"
	"unicode/utf8"

	"github.com/danielhanold/docket/internal/gitcli"
)

// OperationKey names a semantic operation. It must match ^[a-z][a-z0-9.-]*$ so
// it can serve as a stable, greppable trailer value and directory-safe token.
type OperationKey string

// VersionKind distinguishes the two states an entity expectation can pin.
type VersionKind string

// The closed set of version kinds.
const (
	VersionBlob   VersionKind = "blob"
	VersionAbsent VersionKind = "absent"
)

// ExpectedVersion pins one entity's expected state on the fetched base tree: a
// full-hex blob id, or provable absence.
type ExpectedVersion struct {
	Kind     VersionKind
	ObjectID gitcli.ObjectID // required for blob (full hex, exact); must be empty for absent
}

// EntityExpectation binds a repo-relative path to its expected version.
type EntityExpectation struct {
	Path    gitcli.RepoPath
	Version ExpectedVersion
}

// RequestDigest is "sha256:" followed by 64 lowercase hex characters.
type RequestDigest string

// IdempotencyKey names a caller request so a replay can be detected from remote
// history. RequestID is caller-chosen; Digest binds the request's content.
type IdempotencyKey struct {
	RequestID string        // 8–128 ASCII, ^[A-Za-z0-9][A-Za-z0-9._-]*$, case-sensitive
	Digest    RequestDigest // "sha256:" + 64 lowercase hex
}

// MutationKind names the effect of one file mutation.
type MutationKind string

// The closed set of mutation kinds.
const (
	MutationCreate  MutationKind = "create"
	MutationReplace MutationKind = "replace"
	MutationDelete  MutationKind = "delete"
)

// FileMutation describes one declared change to exactly one repo-relative path.
type FileMutation struct {
	Path  gitcli.RepoPath
	Kind  MutationKind
	Bytes []byte // must be nil/empty for delete; may be empty for an intentionally empty file
}

// MutationPlan is a closed description of an operation's effect for one attempt:
// the exact final bytes of every touched path, the commit subject, and the
// canonical receipt to record.
type MutationPlan struct {
	Files         []FileMutation
	CommitSubject string // one non-empty UTF-8 line, no control chars, <= 200 bytes
	Receipt       []byte // canonical compact JSON, <= 4096 bytes
}

// maxCommitSubjectBytes bounds a plan's commit subject.
const maxCommitSubjectBytes = 200

// validateOperationKey enforces ^[a-z][a-z0-9.-]*$ over ASCII bytes; a non-ASCII
// byte fails the character class and is rejected.
func validateOperationKey(k OperationKey) error {
	s := string(k)
	if s == "" {
		return errors.New("transaction: empty operation key")
	}
	if c := s[0]; !(c >= 'a' && c <= 'z') {
		return errors.New("transaction: operation key must begin with a lowercase letter")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '-') {
			return errors.New("transaction: operation key has a character outside [a-z0-9.-]")
		}
	}
	return nil
}

// validateRepoPathValue rejects the empty string, NUL, a leading or trailing
// "/", empty components, "."/".." components anywhere, and any ".git" component
// (which covers both ".git" and ".git/x"). Bytes are treated as opaque; a
// non-ASCII path is legal so long as it has no forbidden structure.
func validateRepoPathValue(p gitcli.RepoPath) error {
	s := string(p)
	if s == "" {
		return errors.New("transaction: empty repo path")
	}
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return errors.New("transaction: repo path contains NUL")
		}
	}
	if s[0] == '/' {
		return errors.New("transaction: repo path has leading slash")
	}
	if s[len(s)-1] == '/' {
		return errors.New("transaction: repo path has trailing slash")
	}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '/' {
			comp := s[start:i]
			switch comp {
			case "":
				return errors.New("transaction: repo path has empty component")
			case ".", "..":
				return errors.New("transaction: repo path has . or .. component")
			case ".git":
				return errors.New("transaction: repo path has .git component")
			}
			start = i + 1
		}
	}
	return nil
}

// validateExpectations checks every expectation's path and version shape and
// rejects duplicate paths.
func validateExpectations(exps []EntityExpectation) error {
	seen := make(map[gitcli.RepoPath]struct{}, len(exps))
	for _, e := range exps {
		if err := validateRepoPathValue(e.Path); err != nil {
			return err
		}
		switch e.Version.Kind {
		case VersionBlob:
			if err := validateFullObjectID(e.Version.ObjectID); err != nil {
				return err
			}
		case VersionAbsent:
			if e.Version.ObjectID != "" {
				return errors.New("transaction: absent expectation must have empty object id")
			}
		default:
			return errors.New("transaction: expectation has unknown version kind")
		}
		if _, dup := seen[e.Path]; dup {
			return errors.New("transaction: duplicate expectation path")
		}
		seen[e.Path] = struct{}{}
	}
	return nil
}

// validateFullObjectID requires a non-empty, all-lowercase-hex id of length 40
// or 64 (SHA-1 / SHA-256 full form; never truncated).
func validateFullObjectID(id gitcli.ObjectID) error {
	s := string(id)
	if s == "" {
		return errors.New("transaction: empty object id")
	}
	if len(s) != 40 && len(s) != 64 {
		return errors.New("transaction: object id must be 40 or 64 hex chars")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return errors.New("transaction: object id has a non-lowercase-hex char")
		}
	}
	return nil
}

// validateIdempotencyKey validates a request key's ID and digest. A nil key
// means "no idempotency" and is valid.
func validateIdempotencyKey(k *IdempotencyKey) error {
	if k == nil {
		return nil
	}
	id := k.RequestID
	if len(id) < 8 || len(id) > 128 {
		return errors.New("transaction: request id must be 8–128 bytes")
	}
	if c := id[0]; !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
		return errors.New("transaction: request id must begin with an alphanumeric")
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '.' || c == '_' || c == '-') {
			return errors.New("transaction: request id has a character outside [A-Za-z0-9._-]")
		}
	}
	return validateRequestDigest(k.Digest)
}

// validateRequestDigest requires "sha256:" + exactly 64 lowercase hex chars.
func validateRequestDigest(d RequestDigest) error {
	const prefix = "sha256:"
	s := string(d)
	if len(s) != len(prefix)+64 {
		return errors.New("transaction: request digest has wrong length")
	}
	if s[:len(prefix)] != prefix {
		return errors.New("transaction: request digest missing sha256: prefix")
	}
	for i := len(prefix); i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return errors.New("transaction: request digest has a non-lowercase-hex char")
		}
	}
	return nil
}

// validatePlan checks a plan's file set (paths, kinds, delete-byte rule,
// duplicate paths), its commit subject, and its receipt. It performs no Git or
// filesystem work.
func validatePlan(p MutationPlan) error {
	seen := make(map[gitcli.RepoPath]struct{}, len(p.Files))
	for _, f := range p.Files {
		if err := validateRepoPathValue(f.Path); err != nil {
			return err
		}
		switch f.Kind {
		case MutationCreate, MutationReplace:
			// Bytes may be empty (intentionally empty file).
		case MutationDelete:
			if len(f.Bytes) > 0 {
				return errors.New("transaction: delete mutation must carry no bytes")
			}
		default:
			return errors.New("transaction: file mutation has unknown kind")
		}
		if _, dup := seen[f.Path]; dup {
			return errors.New("transaction: duplicate mutation path")
		}
		seen[f.Path] = struct{}{}
	}
	if err := validateCommitSubject(p.CommitSubject); err != nil {
		return err
	}
	return validateReceipt(p.Receipt)
}

// validateCommitSubject requires a non-empty, valid-UTF-8 single line of at most
// 200 bytes with no control characters.
func validateCommitSubject(s string) error {
	if s == "" {
		return errors.New("transaction: empty commit subject")
	}
	if len(s) > maxCommitSubjectBytes {
		return errors.New("transaction: commit subject exceeds 200 bytes")
	}
	if !utf8.ValidString(s) {
		return errors.New("transaction: commit subject is not valid UTF-8")
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return errors.New("transaction: commit subject contains a control character")
		}
	}
	return nil
}
