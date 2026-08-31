package gitcli

import (
	"context"
	"strings"
)

// Operation label for the complete remote-heads advertisement surface.
const listRemoteHeadsOp Operation = "list-remote-heads"

// ListRemoteHeads reads one complete branch-heads advertisement from the remote
// via `git ls-remote --heads <remote>` (a network READ on the read budget) and
// returns every advertised head as a map from its fully qualified RefName to the
// full ObjectID it points at. A clean exit with no heads returns an empty
// NON-NIL map — a proven emptiness the caller may treat as "the remote has no
// heads". The advertisement is parsed strictly: each line is "<oid>\t<refname>",
// the id is checked with validateObjectID and the name must carry the
// refs/heads/ prefix and pass validateRefName, and no ref may be advertised
// twice. Any malformed or duplicate line is invalid-output and any non-zero exit
// is command-failed — never a partial map, because a partial map would
// understate the shared inventory and let a caller infer a ref's absence from an
// incomplete read.
//
// The error contract is the whole point of a SHARED inventory: a failed shared
// inventory is unknown, not permission to fan out into individual inventory
// probes. A caller that catches an error here must hold the inventory as unknown
// and must never fall back to per-ref remote reads whose collective absence it
// would misread as proof.
func (c *Client) ListRemoteHeads(ctx context.Context, repo Repository, remote RemoteName) (map[RefName]ObjectID, error) {
	if err := validateRemoteName(remote); err != nil {
		return nil, newFailure(listRemoteHeadsOp, KindInvalidRequest, "invalid remote name", err)
	}
	res, f := c.run(ctx, runRequest{
		op:      listRemoteHeadsOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"ls-remote", "--heads", string(remote)},
		network: true,
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(listRemoteHeadsOp, KindCommandFailed, "ls-remote --heads failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}

	// Build the whole advertisement or fail: splitLsRemoteLine, validateObjectID,
	// and validateRefName reject a malformed line, and a repeated key is refused,
	// so the returned map is either the complete inventory or no map at all.
	heads := make(map[RefName]ObjectID)
	for _, line := range stdoutLines(res.stdout) {
		id, name, ok := splitLsRemoteLine(line)
		if !ok {
			return nil, newFailure(listRemoteHeadsOp, KindInvalidOutput, "malformed ls-remote line", nil)
		}
		if err := validateObjectID(id); err != nil {
			return nil, newFailure(listRemoteHeadsOp, KindInvalidOutput, "ls-remote produced a malformed object id", err)
		}
		if !strings.HasPrefix(name, "refs/heads/") {
			return nil, newFailure(listRemoteHeadsOp, KindInvalidOutput, "ls-remote --heads produced a non-heads ref", nil)
		}
		ref := RefName(name)
		if err := validateRefName(ref); err != nil {
			return nil, newFailure(listRemoteHeadsOp, KindInvalidOutput, "ls-remote produced an invalid ref name", err)
		}
		if _, dup := heads[ref]; dup {
			return nil, newFailure(listRemoteHeadsOp, KindInvalidOutput, "ls-remote advertised a ref more than once", nil)
		}
		heads[ref] = id
	}
	return heads, nil
}
