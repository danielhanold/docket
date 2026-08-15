package gitcli

import (
	"context"
	"strings"
)

// Operation labels for the lease-push and reachability surface.
const (
	pushLeaseOp  Operation = "push-lease"
	isAncestorOp Operation = "is-ancestor"
)

// PushDisposition is the structural outcome of a lease push.
type PushDisposition string

const (
	// PushApplied: the ref result line reports success — the remote now holds the
	// pushed commit.
	PushApplied PushDisposition = "applied"
	// PushLeaseLost: the push was structurally rejected AND a follow-up fetch
	// proved the remote moved to a value the pushed commit does not contain — the
	// only outcome the engine retries as contention.
	PushLeaseLost PushDisposition = "lease-lost"
	// PushFailed: any external/unknown failure. Never retried as contention.
	PushFailed PushDisposition = "failed"
)

// PushOutcome is the classified result of PushLease. Remote is the observed
// remote target when a fetch established it (the pushed commit on applied, the
// winner on lease-lost), else the empty id.
type PushOutcome struct {
	Disposition PushDisposition
	Remote      ObjectID
}

// PushLease pushes commit to ref on remote under the literal expected old value
// via `git push --porcelain --force-with-lease=<ref>:<expected> <remote>
// <commit>:<ref>`. Classification is structural, from the machine-readable
// per-ref result line — never from human stderr, and never from the process exit
// status alone. An ok result line is applied. A rejection ('!') triggers a
// follow-up fetch: only a fetch showing the remote at a value != expected that
// the pushed commit is NOT an ancestor of is a lease loss; every other shape —
// remote still at expected, remote already containing the commit, an
// unestablishable remote, or no per-ref result line at all — is a plain failure.
func (c *Client) PushLease(ctx context.Context, repo Repository, remote RemoteName, ref RefName, commit, expected ObjectID) (PushOutcome, error) {
	if err := validateRemoteName(remote); err != nil {
		return PushOutcome{}, newFailure(pushLeaseOp, KindInvalidRequest, "invalid remote name", err)
	}
	if err := validateRefName(ref); err != nil {
		return PushOutcome{}, newFailure(pushLeaseOp, KindInvalidRequest, "invalid ref name", err)
	}
	if err := validateObjectID(commit); err != nil {
		return PushOutcome{}, newFailure(pushLeaseOp, KindInvalidRequest, "invalid commit id", err)
	}
	if err := validateObjectID(expected); err != nil {
		return PushOutcome{}, newFailure(pushLeaseOp, KindInvalidRequest, "invalid expected id", err)
	}

	lease := "--force-with-lease=" + string(ref) + ":" + string(expected)
	refspec := string(commit) + ":" + string(ref)
	res, f := c.run(ctx, runRequest{
		op:      pushLeaseOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"push", "--porcelain", lease, string(remote), refspec},
		network: true,
	})
	if f != nil {
		return PushOutcome{}, f
	}

	flag, found := parsePushRefLine(res.stdout, ref)
	if found && isOkPushFlag(flag) {
		return PushOutcome{Disposition: PushApplied, Remote: commit}, nil
	}
	if !found || flag != "!" {
		// No structural per-ref rejection to attribute to a lease loss (a transport
		// failure prints no ref result line): a plain failure, never lease-lost.
		return PushOutcome{Disposition: PushFailed}, nil
	}

	// A structural rejection. A lease loss requires a follow-up fetch proving the
	// remote moved to a value the pushed commit does not contain. A rejection
	// alone — or a rejection whose remote still equals expected — is never a lease
	// loss.
	rev, err := c.FetchBranch(ctx, repo, remote, ref)
	if err != nil {
		return PushOutcome{Disposition: PushFailed}, nil
	}
	remoteCommit := rev.Commit
	if remoteCommit == expected {
		return PushOutcome{Disposition: PushFailed, Remote: remoteCommit}, nil
	}
	anc, err := c.IsAncestor(ctx, repo, commit, remoteCommit)
	if err != nil {
		return PushOutcome{Disposition: PushFailed, Remote: remoteCommit}, nil
	}
	if !anc {
		return PushOutcome{Disposition: PushLeaseLost, Remote: remoteCommit}, nil
	}
	return PushOutcome{Disposition: PushFailed, Remote: remoteCommit}, nil
}

// IsAncestor reports whether ancestor is reachable from descendant via
// `git merge-base --is-ancestor <ancestor> <descendant>`: exit 0 is true, exit 1
// is false, and any other exit is a typed command-failed *Failure.
func (c *Client) IsAncestor(ctx context.Context, repo Repository, ancestor, descendant ObjectID) (bool, error) {
	if err := validateObjectID(ancestor); err != nil {
		return false, newFailure(isAncestorOp, KindInvalidRequest, "invalid ancestor id", err)
	}
	if err := validateObjectID(descendant); err != nil {
		return false, newFailure(isAncestorOp, KindInvalidRequest, "invalid descendant id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   isAncestorOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"merge-base", "--is-ancestor", string(ancestor), string(descendant)},
	})
	if f != nil {
		return false, f
	}
	switch res.exitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, newFailure(isAncestorOp, KindCommandFailed, "merge-base --is-ancestor failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
}

// okPushFlags are the porcelain per-ref result flags that mean the update
// succeeded: a fast-forward (space), a forced update, a new ref, and an
// up-to-date ref.
var okPushFlags = map[string]bool{" ": true, "+": true, "*": true, "=": true}

// isOkPushFlag reports whether a porcelain ref-result flag denotes a successful
// update.
func isOkPushFlag(flag string) bool { return okPushFlags[flag] }

// parsePushRefLine finds the porcelain per-ref result line for ref and returns
// its leading flag. Each result line is "<flag>\t<from>:<to>\t<summary>"; the
// "To <url>" header and trailing "Done" line are skipped. The destination ref is
// the segment after the single ':' in the "<from>:<to>" field.
func parsePushRefLine(out []byte, ref RefName) (string, bool) {
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || strings.HasPrefix(line, "To ") || line == "Done" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		fromTo := fields[1]
		colon := strings.LastIndexByte(fromTo, ':')
		if colon < 0 {
			continue
		}
		if fromTo[colon+1:] == string(ref) {
			return fields[0], true
		}
	}
	return "", false
}
