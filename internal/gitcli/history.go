package gitcli

import (
	"bytes"
	"context"
	"errors"
)

// Operation labels for the narrow history-read surface (shared-ancestry,
// whole-history tree walk, non-recursive tree-entry read).
const (
	sharedAncestryOp Operation = "shared-ancestry"
	listHistoryOp    Operation = "list-history-trees"
	treeEntriesOp    Operation = "tree-entry-ids"
)

// HasSharedAncestry reports whether a and b share any common ancestor via
// `git merge-base <a> <b>`: exit 0 (a merge base exists) is true, exit 1 (no
// merge base) is a clean false, and any other exit — an absent object, a
// transport or repository fault — is a typed command-failed *Failure. An error
// is never collapsed into false: unprovable disjointness is an error, not a
// negative answer.
func (c *Client) HasSharedAncestry(ctx context.Context, repo Repository, a, b ObjectID) (bool, error) {
	if err := validateObjectID(a); err != nil {
		return false, newFailure(sharedAncestryOp, KindInvalidRequest, "invalid object id a", err)
	}
	if err := validateObjectID(b); err != nil {
		return false, newFailure(sharedAncestryOp, KindInvalidRequest, "invalid object id b", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   sharedAncestryOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"merge-base", string(a), string(b)},
	})
	if f != nil {
		return false, f
	}
	switch res.exitCode {
	case 0:
		return true, nil
	case 1:
		// merge-base's documented "no merge base found" exit: a genuine clean
		// false, distinct from every other nonzero exit (absent object, fault).
		return false, nil
	default:
		return false, newFailure(sharedAncestryOp, KindCommandFailed, "merge-base failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
}

// HistoryEntry pairs a commit with its own root tree OID.
type HistoryEntry struct {
	Commit ObjectID
	Tree   ObjectID
}

// ListHistoryTrees walks the COMPLETE reachable history from tip
// (`git rev-list --format=%H %T <tip>`) — no depth window, no first-parent — and
// returns each commit paired with its root tree OID, newest-first (rev-list's
// default topological/date order). The batched read the legacy-equivalence
// dedupe keys on. rev-list --format prefixes each commit with a "commit <oid>"
// header line; those are skipped and only the "%H %T" payload lines are parsed.
func (c *Client) ListHistoryTrees(ctx context.Context, repo Repository, tip ObjectID) ([]HistoryEntry, error) {
	if err := validateObjectID(tip); err != nil {
		return nil, newFailure(listHistoryOp, KindInvalidRequest, "invalid tip id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   listHistoryOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"rev-list", "--format=%H %T", string(tip)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(listHistoryOp, KindCommandFailed, "rev-list --format failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	lines := stdoutLines(res.stdout)
	entries := make([]HistoryEntry, 0, len(lines))
	for _, line := range lines {
		// rev-list --format emits a "commit <oid>" header before each payload.
		if len(line) >= 7 && line[:7] == "commit " {
			continue
		}
		sp := indexByteString(line, ' ')
		if sp < 0 {
			return nil, newFailure(listHistoryOp, KindInvalidOutput, "rev-list payload line has no space separator", nil)
		}
		commit := ObjectID(line[:sp])
		tree := ObjectID(line[sp+1:])
		if err := validateObjectID(commit); err != nil {
			return nil, newFailure(listHistoryOp, KindInvalidOutput, "rev-list produced a malformed commit id", err)
		}
		if err := validateObjectID(tree); err != nil {
			return nil, newFailure(listHistoryOp, KindInvalidOutput, "rev-list produced a malformed tree id", err)
		}
		entries = append(entries, HistoryEntry{Commit: commit, Tree: tree})
	}
	return entries, nil
}

// TreeEntryIDs reads the NON-recursive tree entries of commit at the given
// repo-relative paths (`git ls-tree -z --full-tree <commit> -- <paths...>`),
// returning both tree AND blob entries with mode/type/OID/path. Unlike
// ObjectSource.ListTree (which lists recursive leaves and therefore rejects
// tree entries), this preserves a directory as a single `tree` entry so a caller
// can compare a subtree by its OID. An absent path simply yields no entry — never
// an error. With no paths the whole root tree's top level is listed.
func (c *Client) TreeEntryIDs(ctx context.Context, repo Repository, commit ObjectID, paths []RepoPath) ([]TreeEntry, error) {
	if err := validateObjectID(commit); err != nil {
		return nil, newFailure(treeEntriesOp, KindInvalidRequest, "invalid commit id", err)
	}
	args := []string{"ls-tree", "-z", "--full-tree", string(commit)}
	if len(paths) > 0 {
		args = append(args, "--")
		for _, p := range paths {
			if err := validateRepoPath(p, false); err != nil {
				return nil, newFailure(treeEntriesOp, KindInvalidRequest, "invalid tree path", err)
			}
			args = append(args, string(p))
		}
	}
	res, f := c.run(ctx, runRequest{op: treeEntriesOp, dir: repo.PrimaryWorktree, args: args})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(treeEntriesOp, KindCommandFailed, "ls-tree failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	entries, err := parseLsTreeEntriesZ(res.stdout)
	if err != nil {
		return nil, newFailure(treeEntriesOp, KindInvalidOutput, "malformed ls-tree output", err)
	}
	return entries, nil
}

// parseLsTreeEntriesZ parses the NUL-delimited records of a NON-recursive
// `ls-tree -z` stream — "<mode> <type> <oid>\t<path>\x00" — into typed entries,
// accepting `tree` entries (mode 040000) as well as blob/gitlink leaves. It
// mirrors parseLsTreeZ's record discipline: every record must be
// NUL-terminated, carry a tab, split into exactly three header fields, hold a
// valid mode/type pair and object id, and a non-empty path. Any violation is an
// error and no entries are returned.
func parseLsTreeEntriesZ(out []byte) ([]TreeEntry, error) {
	if len(out) == 0 {
		return nil, nil
	}
	recs := bytes.Split(out, []byte{0})
	if len(recs[len(recs)-1]) != 0 {
		return nil, errors.New("gitcli: ls-tree output has an unterminated trailing record")
	}
	recs = recs[:len(recs)-1]
	entries := make([]TreeEntry, 0, len(recs))
	for _, rec := range recs {
		if len(rec) == 0 {
			return nil, errors.New("gitcli: ls-tree emitted an empty record")
		}
		tab := bytes.IndexByte(rec, '\t')
		if tab < 0 {
			return nil, errors.New("gitcli: ls-tree record has no tab separator")
		}
		header := rec[:tab]
		path := rec[tab+1:]
		if len(path) == 0 {
			return nil, errors.New("gitcli: ls-tree record has empty path")
		}
		fields := bytes.Split(header, []byte{' '})
		if len(fields) != 3 {
			return nil, errors.New("gitcli: ls-tree header is not three fields")
		}
		mode := FileMode(fields[0])
		typ := ObjectType(fields[1])
		oid := ObjectID(fields[2])
		if !validTreeEntry(mode, typ) {
			return nil, errors.New("gitcli: ls-tree record has an unexpected mode/type")
		}
		if err := validateObjectID(oid); err != nil {
			return nil, err
		}
		entries = append(entries, TreeEntry{Path: RepoPath(path), Mode: mode, Type: typ, ObjectID: oid})
	}
	return entries, nil
}

// validTreeEntry reports whether a mode/type pair is a legal NON-recursive
// ls-tree entry: the three blob modes carry type "blob", a gitlink (160000)
// carries type "commit", and a subtree (040000) carries type "tree".
func validTreeEntry(mode FileMode, typ ObjectType) bool {
	switch mode {
	case "100644", "100755", "120000":
		return typ == "blob"
	case "160000":
		return typ == "commit"
	case "040000":
		return typ == "tree"
	}
	return false
}

// indexByteString returns the index of the first b in s, or -1.
func indexByteString(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
