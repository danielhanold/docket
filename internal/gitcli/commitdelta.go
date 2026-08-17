package gitcli

import (
	"bytes"
	"context"
	"errors"
	"sort"
)

// commitDeltaOp labels the commit-delta surface in any Failure.
const commitDeltaOp Operation = "commit-delta"

// CommitChangedPaths returns the sorted, de-duplicated set of repo-relative
// paths a commit changes relative to its first parent, via
// `diff-tree --no-commit-id --name-only -r -z --no-renames --root <commit>`.
// Rename detection is OFF, so a move surfaces as a delete of the source and an
// add of the destination: a single-artifact predicate must see both sides
// (learning diff-derived-allowlist-needs-no-renames). A root commit (no parent)
// reports every path in its tree (--root). Output is read NUL-delimited so
// hostile path bytes (spaces, tabs, embedded newlines, non-ASCII) and
// core.quotePath=true never corrupt the parse.
func (c *Client) CommitChangedPaths(ctx context.Context, repo Repository, commit ObjectID) ([]RepoPath, error) {
	if err := validateObjectID(commit); err != nil {
		return nil, newFailure(commitDeltaOp, KindInvalidRequest, "invalid commit id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   commitDeltaOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"diff-tree", "--no-commit-id", "--name-only", "-r", "-z", "--no-renames", "--root", string(commit)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(commitDeltaOp, KindCommandFailed, "diff-tree failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	paths, err := parseNulPaths(res.stdout)
	if err != nil {
		return nil, newFailure(commitDeltaOp, KindInvalidOutput, "malformed diff-tree output", err)
	}
	return paths, nil
}

// parseNulPaths parses the NUL-delimited path list `diff-tree --name-only -z`
// emits — each path is terminated by a NUL, so the final split element is empty
// (a non-empty tail is a truncated record). Empty output is no paths. Results
// are sorted by raw path bytes and de-duplicated.
func parseNulPaths(out []byte) ([]RepoPath, error) {
	if len(out) == 0 {
		return nil, nil
	}
	recs := bytes.Split(out, []byte{0})
	if len(recs[len(recs)-1]) != 0 {
		return nil, errors.New("gitcli: diff-tree output has an unterminated trailing record")
	}
	recs = recs[:len(recs)-1]
	paths := make([]RepoPath, 0, len(recs))
	for _, rec := range recs {
		if len(rec) == 0 {
			return nil, errors.New("gitcli: diff-tree emitted an empty path")
		}
		paths = append(paths, RepoPath(rec))
	}
	sort.Slice(paths, func(i, j int) bool {
		return bytes.Compare([]byte(paths[i]), []byte(paths[j])) < 0
	})
	out2 := paths[:0]
	var last RepoPath
	haveLast := false
	for _, p := range paths {
		if haveLast && p == last {
			continue
		}
		out2 = append(out2, p)
		last = p
		haveLast = true
	}
	return out2, nil
}
