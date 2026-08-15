package gitcli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
)

// Operation labels for the worktree-lifecycle surface.
const (
	worktreeAddOp    Operation = "worktree-add"
	worktreeRemoveOp Operation = "worktree-remove"
	worktreeListOp   Operation = "worktree-list"
)

// WorktreeInfo is one registered worktree as reported by
// `git worktree list --porcelain -z`. Path is the path git reports; callers
// canonicalize (Abs + every symlink hop) before comparing it to their own
// spelling. Branch is empty on a detached worktree.
type WorktreeInfo struct {
	Path     string
	Head     ObjectID
	Detached bool
	Branch   RefName
}

// AddDetachedWorktree registers a detached worktree at path checked out at
// commit via `worktree add --detach <path> <commit>`, run from the primary
// worktree. It never creates or resets a branch: the checkout is detached at the
// exact commit. path must be absolute (the caller has already proven it beneath
// the engine-owned root); a relative path or a malformed commit id is
// invalid-request. A non-zero git exit is command-failed with a redacted,
// bounded stderr excerpt.
func (c *Client) AddDetachedWorktree(ctx context.Context, repo Repository, path string, commit ObjectID) error {
	if !filepath.IsAbs(path) {
		return newFailure(worktreeAddOp, KindInvalidRequest, "worktree path must be absolute", nil)
	}
	if err := validateObjectID(commit); err != nil {
		return newFailure(worktreeAddOp, KindInvalidRequest, "invalid commit id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   worktreeAddOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"worktree", "add", "--detach", "--", path, string(commit)},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(worktreeAddOp, KindCommandFailed, "worktree add failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}

// RemoveWorktree deregisters exactly the worktree registered at path via
// `worktree remove --force <path>`, run from the primary worktree. The force is
// deliberate: a transaction worktree intentionally carries staged/dirty state at
// cleanup time. path must be absolute; an unregistered path is a command-failed
// *Failure (git's own non-zero exit), never a panic.
func (c *Client) RemoveWorktree(ctx context.Context, repo Repository, path string) error {
	if !filepath.IsAbs(path) {
		return newFailure(worktreeRemoveOp, KindInvalidRequest, "worktree path must be absolute", nil)
	}
	res, f := c.run(ctx, runRequest{
		op:   worktreeRemoveOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"worktree", "remove", "--force", "--", path},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(worktreeRemoveOp, KindCommandFailed, "worktree remove failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}

// ListWorktrees parses `git worktree list --porcelain -z` — never human display
// output — into one WorktreeInfo per registered worktree. Records are read
// NUL-delimited so a worktree path carrying hostile bytes cannot corrupt the
// parse. A malformed HEAD object id or a stanza missing its worktree path is
// invalid-output with no partial result.
func (c *Client) ListWorktrees(ctx context.Context, repo Repository) ([]WorktreeInfo, error) {
	res, f := c.run(ctx, runRequest{
		op:   worktreeListOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(worktreeListOp, KindCommandFailed, "worktree list failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	infos, err := parseWorktreeListZ(res.stdout)
	if err != nil {
		return nil, newFailure(worktreeListOp, KindInvalidOutput, "malformed worktree list output", err)
	}
	return infos, nil
}

// parseWorktreeListZ parses the NUL-delimited attribute stream of
// `worktree list --porcelain -z`. Each worktree stanza is a run of
// "label value" (or bare "label") fields terminated by an empty field; the
// "worktree <path>" field opens a stanza. Recognized attributes are worktree,
// HEAD, branch, and detached; bare and any other attribute (locked, prunable,
// …) are tolerated and ignored. A HEAD value must be a valid object id; a
// stanza without a worktree path is malformed.
func parseWorktreeListZ(out []byte) ([]WorktreeInfo, error) {
	var infos []WorktreeInfo
	var cur WorktreeInfo
	inStanza := false
	isBare := false

	flush := func() error {
		if !inStanza {
			return nil
		}
		if cur.Path == "" {
			return errBadWorktreeStanza
		}
		// A bare repository entry carries no HEAD; the caller's WorktreeInfo has
		// no bare field, so only require a valid HEAD for non-bare worktrees.
		if !isBare {
			if err := validateObjectID(cur.Head); err != nil {
				return err
			}
		}
		infos = append(infos, cur)
		cur = WorktreeInfo{}
		inStanza = false
		isBare = false
		return nil
	}

	for _, field := range bytes.Split(out, []byte{0}) {
		if len(field) == 0 {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		label, value := splitWorktreeField(field)
		switch label {
		case "worktree":
			// A new "worktree" field without an intervening empty separator
			// still opens a fresh stanza; flush whatever preceded it.
			if err := flush(); err != nil {
				return nil, err
			}
			cur = WorktreeInfo{Path: value}
			inStanza = true
		case "HEAD":
			cur.Head = ObjectID(value)
		case "branch":
			cur.Branch = RefName(value)
			cur.Detached = false
		case "detached":
			cur.Detached = true
		case "bare":
			isBare = true
		default:
			// locked, prunable, and any future attribute: ignored.
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return infos, nil
}

// errBadWorktreeStanza is the sentinel for a worktree-list stanza that never
// declared its path.
var errBadWorktreeStanza = &worktreeParseError{"worktree stanza missing its path"}

type worktreeParseError struct{ msg string }

func (e *worktreeParseError) Error() string { return "gitcli: " + e.msg }

// splitWorktreeField splits one porcelain field into its label and value at the
// first space; a bare attribute ("detached", "bare") has an empty value.
func splitWorktreeField(field []byte) (label, value string) {
	s := string(field)
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}
