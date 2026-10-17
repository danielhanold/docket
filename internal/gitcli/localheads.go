package gitcli

import (
	"context"
	"strings"
)

// Operation label for the local branch-heads inventory.
const listLocalHeadsOp Operation = "list-local-heads"

// ListLocalHeads reads every local branch in one local read via
// `git for-each-ref --format=%(objectname)%09%(refname) refs/heads/` and returns
// each as a map from its fully qualified RefName to the full ObjectID it points
// at. It has exactly three outcomes: a complete map; an empty NON-NIL map, a
// proven emptiness meaning the repository has no branches; or nil plus a
// *Failure. It never returns a partial map: a malformed line, a malformed id, a
// name outside refs/heads/ or failing validateRefName, or a repeated ref is
// invalid-output, and any non-zero exit is command-failed. A caller holding an
// error must treat the inventory as unknown, never as "branch absent".
func (c *Client) ListLocalHeads(ctx context.Context, repo Repository) (map[RefName]ObjectID, error) {
	res, f := c.run(ctx, runRequest{
		op:      listLocalHeadsOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"for-each-ref", "--format=%(objectname)%09%(refname)", "refs/heads/"},
		network: false,
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(listLocalHeadsOp, KindCommandFailed, "for-each-ref refs/heads/ failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}

	heads := make(map[RefName]ObjectID)
	for _, line := range stdoutLines(res.stdout) {
		id, name, ok := splitLsRemoteLine(line)
		if !ok {
			return nil, newFailure(listLocalHeadsOp, KindInvalidOutput, "malformed for-each-ref line", nil)
		}
		if err := validateObjectID(id); err != nil {
			return nil, newFailure(listLocalHeadsOp, KindInvalidOutput, "for-each-ref produced a malformed object id", err)
		}
		if !strings.HasPrefix(name, "refs/heads/") {
			return nil, newFailure(listLocalHeadsOp, KindInvalidOutput, "for-each-ref produced a non-heads ref", nil)
		}
		ref := RefName(name)
		if err := validateRefName(ref); err != nil {
			return nil, newFailure(listLocalHeadsOp, KindInvalidOutput, "for-each-ref produced an invalid ref name", err)
		}
		if _, dup := heads[ref]; dup {
			return nil, newFailure(listLocalHeadsOp, KindInvalidOutput, "for-each-ref listed a ref more than once", nil)
		}
		heads[ref] = id
	}
	return heads, nil
}
