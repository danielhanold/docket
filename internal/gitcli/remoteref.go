package gitcli

import "context"

// Operation label for the remote-branch probe surface.
const probeRemoteBranchOp Operation = "probe-remote-branch"

// RemoteRefState is a three-outcome probe result. An errored probe is an error
// return, NEVER RemoteRefAbsent (learnings: probe-error-is-not-clean-absence).
type RemoteRefState string

const (
	// RemoteRefFound: the remote holds the ref at exactly one commit.
	RemoteRefFound RemoteRefState = "found"
	// RemoteRefAbsent: the remote answered cleanly and does not hold the ref.
	RemoteRefAbsent RemoteRefState = "absent"
)

// RemoteRef is the classified outcome of ProbeRemoteBranch. Commit carries the
// full object id when found; it is the empty id when absent.
type RemoteRef struct {
	State  RemoteRefState
	Commit ObjectID
}

// ProbeRemoteBranch asks the remote authoritatively whether a fully qualified
// branch ref exists, via `git ls-remote <remote> <ref>` (a network operation on
// the fetch deadline). The remote is confirmed configured first, so an unknown
// remote name is remote-unavailable rather than a masked clean absence. Exactly
// one line whose refname is byte-equal to the requested ref => found plus the
// full ObjectID; zero matching lines on a clean exit => absent; any non-zero
// exit is command-failed, and a malformed line, an abbreviated id, or more than
// one matching line is invalid-output — an errored probe never reports absence.
func (c *Client) ProbeRemoteBranch(ctx context.Context, repo Repository, remote RemoteName, ref RefName) (RemoteRef, error) {
	if err := validateRemoteName(remote); err != nil {
		return RemoteRef{}, newFailure(probeRemoteBranchOp, KindInvalidRequest, "invalid remote name", err)
	}
	if err := validateRefName(ref); err != nil {
		return RemoteRef{}, newFailure(probeRemoteBranchOp, KindInvalidRequest, "invalid ref name", err)
	}
	if f := c.ensureRemoteConfigured(ctx, probeRemoteBranchOp, repo, remote); f != nil {
		return RemoteRef{}, f
	}
	res, f := c.run(ctx, runRequest{
		op:      probeRemoteBranchOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"ls-remote", string(remote), string(ref)},
		network: true,
	})
	if f != nil {
		return RemoteRef{}, f
	}
	if res.exitCode != 0 {
		return RemoteRef{}, newFailure(probeRemoteBranchOp, KindCommandFailed, "ls-remote failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}

	// Each ls-remote line is "<full-hex>\t<refname>". A fully qualified pattern
	// already restricts the answer to the exact ref, but the refname is checked
	// byte-for-byte so a prefix collision (refs/heads/feat/x vs refs/heads/feat/x2)
	// can never leak the wrong id, and the id shape is validated so an abbreviated
	// or otherwise malformed answer is invalid-output, not a silent found.
	var matched []ObjectID
	for _, line := range stdoutLines(res.stdout) {
		id, name, ok := splitLsRemoteLine(line)
		if !ok {
			return RemoteRef{}, newFailure(probeRemoteBranchOp, KindInvalidOutput, "malformed ls-remote line", nil)
		}
		if name != string(ref) {
			continue
		}
		if err := validateObjectID(id); err != nil {
			return RemoteRef{}, newFailure(probeRemoteBranchOp, KindInvalidOutput, "ls-remote produced a malformed object id", err)
		}
		matched = append(matched, id)
	}
	switch len(matched) {
	case 0:
		return RemoteRef{State: RemoteRefAbsent}, nil
	case 1:
		return RemoteRef{State: RemoteRefFound, Commit: matched[0]}, nil
	default:
		return RemoteRef{}, newFailure(probeRemoteBranchOp, KindInvalidOutput, "multiple matching ls-remote lines", nil)
	}
}

// splitLsRemoteLine splits one ls-remote line "<hex>\t<refname>" into its id and
// refname on the single separating TAB, reporting false when the line has no
// TAB (a refname never contains a TAB, so a missing TAB is a malformed line).
func splitLsRemoteLine(line string) (ObjectID, string, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] == '\t' {
			return ObjectID(line[:i]), line[i+1:], true
		}
	}
	return "", "", false
}
