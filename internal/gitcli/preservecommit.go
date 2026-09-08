package gitcli

import (
	"bytes"
	"context"
	"strings"
)

// Operation labels for the preservation-proof plumbing. They follow the
// file-local convention the sibling read surfaces use (see history.go's
// sharedAncestryOp/listHistoryOp/treeEntriesOp).
const (
	preserveOp   Operation = "preserve-proof"
	mergeBasesOp Operation = "merge-bases"
)

// ensurePreservationInputs validates both IDs, refuses shallow history, and
// resolves both as commit objects — fetching a missing exact object at most once
// from the established remote. Inability to obtain an object, a non-commit
// object, or a shallow boundary is an observation failure, never an answer: a
// preservation query that cannot see its own operands must error, not report a
// clean "unproven".
func (c *Client) ensurePreservationInputs(ctx context.Context, repo Repository, remote RemoteName, source, target ObjectID) *Failure {
	for _, id := range []ObjectID{source, target} {
		if err := validateObjectID(id); err != nil {
			return newFailure(preserveOp, KindInvalidRequest, "invalid commit id", err)
		}
	}
	res, f := c.run(ctx, runRequest{
		op:   preserveOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"rev-parse", "--is-shallow-repository"},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(preserveOp, KindCommandFailed, "rev-parse --is-shallow-repository failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	if strings.TrimSpace(string(res.stdout)) != "false" {
		return newFailure(preserveOp, KindInvalidRepository, "shallow history cannot answer a preservation query", nil)
	}
	for _, id := range []ObjectID{source, target} {
		if f := c.resolveCommitFetching(ctx, repo, remote, id); f != nil {
			return f
		}
	}
	return nil
}

// resolveCommitFetching guarantees id names a commit reachable in the local
// object store, fetching the exact object once from the established remote when
// it is absent. A present non-commit object (a blob or tree) is an unexpected
// object — a fetch cannot change an object's type, so the network is never
// consulted for it. An object still absent or non-commit after the single fetch
// is an unexpected-object failure naming the id, never a verdict.
func (c *Client) resolveCommitFetching(ctx context.Context, repo Repository, remote RemoteName, id ObjectID) *Failure {
	// Fast path: the id already peels to a commit in the local store.
	peeled, f := c.run(ctx, runRequest{
		op:   preserveOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"cat-file", "-e", string(id) + "^{commit}"},
	})
	if f != nil {
		return f
	}
	if peeled.exitCode == 0 {
		return nil
	}
	// Not resolvable as a commit locally. Distinguish a present non-commit object
	// (fetching cannot fix its type) from a genuinely absent one (fetchable).
	exists, f := c.run(ctx, runRequest{
		op:   preserveOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"cat-file", "-e", string(id)},
	})
	if f != nil {
		return f
	}
	if exists.exitCode == 0 {
		return newFailure(preserveOp, KindUnexpectedObject, "object is not a commit: "+string(id), nil)
	}
	// Absent locally: fetch the exact id once from the established remote, then
	// recheck. The remote must be configured (a network read against a phantom
	// remote is never attempted).
	if f := c.ensureRemoteConfigured(ctx, preserveOp, repo, remote); f != nil {
		return f
	}
	fetched, f := c.run(ctx, runRequest{
		op:      preserveOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"fetch", "--no-tags", "--recurse-submodules=no", string(remote), string(id)},
		network: true,
	})
	if f != nil {
		return f
	}
	if fetched.exitCode != 0 {
		return newFailure(preserveOp, KindUnexpectedObject, "object could not be fetched from remote: "+string(id), nil).withExitCode(fetched.exitCode)
	}
	recheck, f := c.run(ctx, runRequest{
		op:   preserveOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"cat-file", "-e", string(id) + "^{commit}"},
	})
	if f != nil {
		return f
	}
	if recheck.exitCode != 0 {
		return newFailure(preserveOp, KindUnexpectedObject, "fetched object is absent or not a commit: "+string(id), nil)
	}
	return nil
}

// mergeBasesAll returns ALL best common ancestors of a and b via
// `git merge-base --all a b`. Exit 0 lists one id per line; exit 1 with empty
// output is merge-base's documented "no common ancestor" — an empty slice and a
// nil failure, distinct from every other nonzero exit, which is a typed
// command-failed *Failure. Malformed plumbing output is invalid-output.
func (c *Client) mergeBasesAll(ctx context.Context, repo Repository, a, b ObjectID) ([]ObjectID, *Failure) {
	res, f := c.run(ctx, runRequest{
		op:   mergeBasesOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"merge-base", "--all", string(a), string(b)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode == 1 && len(bytes.TrimSpace(res.stdout)) == 0 {
		return nil, nil
	}
	if res.exitCode != 0 {
		return nil, newFailure(mergeBasesOp, KindCommandFailed, "merge-base --all failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	var out []ObjectID
	for _, line := range stdoutLines(res.stdout) {
		id := ObjectID(line)
		if err := validateObjectID(id); err != nil {
			return nil, newFailure(mergeBasesOp, KindInvalidOutput, "malformed merge-base output", err)
		}
		out = append(out, id)
	}
	return out, nil
}
