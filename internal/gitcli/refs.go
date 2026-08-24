package gitcli

import (
	"bytes"
	"context"
	"strings"
)

// Operation labels for the ref/fetch surface.
const (
	remoteDefaultBranchOp Operation = "remote-default-branch"
	fetchBranchOp         Operation = "fetch-branch"
	resolveRefOp          Operation = "resolve-ref"
	remoteURLOp           Operation = "remote-url"
)

// branchRefPrefix is the fully-qualified local-branch namespace; FetchBranch
// only accepts a branch under it and derives the short name from the remainder.
const branchRefPrefix = "refs/heads/"

// RemoteDefaultBranch reports the branch a remote's own HEAD points at, read
// live from the remote via `ls-remote --symref <remote> HEAD`. It performs no
// set-head, caches nothing, and never guesses: a remote whose HEAD is detached
// (no branch symref) is ref-unavailable, and an unconfigured remote name is
// remote-unavailable (probed before any network access).
func (c *Client) RemoteDefaultBranch(ctx context.Context, repo Repository, remote RemoteName) (RefName, error) {
	if err := validateRemoteName(remote); err != nil {
		return "", newFailure(remoteDefaultBranchOp, KindInvalidRequest, "invalid remote name", err)
	}
	if f := c.ensureRemoteConfigured(ctx, remoteDefaultBranchOp, repo, remote); f != nil {
		return "", f
	}
	res, f := c.run(ctx, runRequest{
		op:      remoteDefaultBranchOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"ls-remote", "--symref", string(remote), "HEAD"},
		network: true,
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(remoteDefaultBranchOp, KindCommandFailed, "ls-remote failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	ref, ok := parseSymrefHead(res.stdout)
	if !ok {
		return "", newFailure(remoteDefaultBranchOp, KindRefUnavailable, "remote HEAD has no branch symref", nil)
	}
	if err := validateRefName(ref); err != nil {
		return "", newFailure(remoteDefaultBranchOp, KindInvalidOutput, "remote symref is not a valid ref", err)
	}
	return ref, nil
}

// FetchBranch fetches exactly one fully-qualified remote branch into its
// tracking ref (`+refs/heads/<b>:refs/remotes/<remote>/<b>`), pulling no tags
// and no submodules, then resolves that tracking ref to the exact commit and
// returns the pinned Revision. FETCH_HEAD is never read. The branch must be
// refs/heads/<name> (else invalid-request); an unconfigured remote is
// remote-unavailable; an absent source branch is ref-unavailable; any other
// fetch failure is command-failed; a post-fetch unresolvable tracking ref is
// ref-unavailable (never a silently accepted stale cached ref).
func (c *Client) FetchBranch(ctx context.Context, repo Repository, remote RemoteName, branch RefName) (Revision, error) {
	if err := validateRemoteName(remote); err != nil {
		return Revision{}, newFailure(fetchBranchOp, KindInvalidRequest, "invalid remote name", err)
	}
	if err := validateRefName(branch); err != nil {
		return Revision{}, newFailure(fetchBranchOp, KindInvalidRequest, "invalid branch ref", err)
	}
	short, ok := branchShortName(branch)
	if !ok {
		return Revision{}, newFailure(fetchBranchOp, KindInvalidRequest, "branch must be fully qualified refs/heads/<name>", nil)
	}
	if f := c.ensureRemoteConfigured(ctx, fetchBranchOp, repo, remote); f != nil {
		return Revision{}, f
	}
	trackingRef := RefName("refs/remotes/" + string(remote) + "/" + short)
	refspec := "+" + string(branch) + ":" + string(trackingRef)
	// The fetch and the failure-classification probe that may follow it are two
	// network processes serving one operation, so they share one network budget
	// rather than each starting its own clock: an unreachable remote must cost a
	// caller a single networkTimeout, not two back to back. Scoped to the pair —
	// the local rev-parse below keeps its own local budget.
	netCtx, cancelNet := context.WithTimeout(ctx, c.networkTimeout)
	defer cancelNet()
	res, f := c.run(netCtx, runRequest{
		op:      fetchBranchOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"fetch", "--no-tags", "--recurse-submodules=no", string(remote), refspec},
		network: true,
	})
	if f != nil {
		return Revision{}, f
	}
	if res.exitCode != 0 {
		return Revision{}, c.classifyFetchFailure(netCtx, repo, remote, branch, res)
	}
	commit, err := c.ResolveRef(ctx, repo, trackingRef)
	if err != nil {
		if fail, ok := AsFailure(err); ok && fail.Kind == KindRefUnavailable {
			return Revision{}, newFailure(fetchBranchOp, KindRefUnavailable, "fetched tracking ref did not resolve to a commit", err)
		}
		return Revision{}, err
	}
	return Revision{Commit: commit, Remote: remote, Ref: branch}, nil
}

// ResolveRef resolves a fully-qualified ref to the exact commit it names via
// `rev-parse --verify --end-of-options <ref>^{commit}` (no network). A ref that
// does not resolve to a commit is ref-unavailable; a non-object-id answer is
// invalid-output.
func (c *Client) ResolveRef(ctx context.Context, repo Repository, ref RefName) (ObjectID, error) {
	if err := validateRefName(ref); err != nil {
		return "", newFailure(resolveRefOp, KindInvalidRequest, "invalid ref name", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   resolveRefOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"rev-parse", "--verify", "--end-of-options", string(ref) + "^{commit}"},
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(resolveRefOp, KindRefUnavailable, "ref does not resolve to a commit", nil).withExitCode(res.exitCode)
	}
	lines := stdoutLines(res.stdout)
	if len(lines) != 1 {
		return "", newFailure(resolveRefOp, KindInvalidOutput, "unexpected rev-parse output", nil)
	}
	id := ObjectID(lines[0])
	if err := validateObjectID(id); err != nil {
		return "", newFailure(resolveRefOp, KindInvalidOutput, "rev-parse produced a malformed object id", err)
	}
	return id, nil
}

// ensureRemoteConfigured verifies a remote name is a configured remote before
// any network access, so an unknown name is distinguished from a transport or
// authentication failure. It reads local config only (`remote get-url`); a
// non-zero exit means the remote is not configured (remote-unavailable).
func (c *Client) ensureRemoteConfigured(ctx context.Context, op Operation, repo Repository, remote RemoteName) *Failure {
	res, f := c.run(ctx, runRequest{
		op:   op,
		dir:  repo.PrimaryWorktree,
		args: []string{"remote", "get-url", string(remote)},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(op, KindRemoteUnavailable, "remote is not configured", nil).withExitCode(res.exitCode)
	}
	return nil
}

// RemoteURL returns the configured URL of a named remote, read from raw local
// config (`git config --get remote.<name>.url`); offline, no network. Raw
// config is deliberate: a url.<base>.insteadOf transport rewrite must not
// perturb the value (change 0341 derives the repository WEB URL from it). An
// unconfigured remote name is remote-unavailable.
func (c *Client) RemoteURL(ctx context.Context, repo Repository, remote RemoteName) (string, error) {
	if err := validateRemoteName(remote); err != nil {
		return "", newFailure(remoteURLOp, KindInvalidRequest, "invalid remote name", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   remoteURLOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"config", "--get", "remote." + string(remote) + ".url"},
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(remoteURLOp, KindRemoteUnavailable, "remote is not configured", nil).withExitCode(res.exitCode)
	}
	lines := stdoutLines(res.stdout)
	if len(lines) != 1 {
		return "", newFailure(remoteURLOp, KindInvalidOutput, "unexpected remote url output", nil)
	}
	return lines[0], nil
}

// classifyFetchFailure distinguishes an absent source branch from a genuine
// transport/command failure after a failed fetch of a known-configured remote.
// It runs one `ls-remote <remote> refs/heads/<b>` probe: a successful probe with
// empty output means the branch is absent (ref-unavailable); a failing probe or
// a non-empty answer means the fetch failed for another reason (command-failed).
//
// ctx is the caller's shared network budget, already part-spent by the fetch;
// the probe inherits whatever remains instead of opening a second full one, so
// FetchBranch costs one networkTimeout end to end. A probe that cannot run
// because that shared budget is spent surfaces as the operation's own
// timed-out/cancelled failure, which is what exhausting it means.
func (c *Client) classifyFetchFailure(ctx context.Context, repo Repository, remote RemoteName, branch RefName, fetchRes runResult) *Failure {
	probe, f := c.run(ctx, runRequest{
		op:      fetchBranchOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"ls-remote", string(remote), string(branch)},
		network: true,
	})
	if f != nil {
		return f
	}
	if probe.exitCode == 0 && len(bytes.TrimSpace(probe.stdout)) == 0 {
		return newFailure(fetchBranchOp, KindRefUnavailable, "remote branch does not exist", nil)
	}
	return newFailure(fetchBranchOp, KindCommandFailed, "git fetch failed: "+stderrExcerpt(fetchRes.stderr), nil).withExitCode(fetchRes.exitCode)
}

// branchShortName strips the refs/heads/ prefix, reporting false when the ref is
// not a local branch or names an empty branch.
func branchShortName(branch RefName) (string, bool) {
	s := string(branch)
	if !strings.HasPrefix(s, branchRefPrefix) {
		return "", false
	}
	short := s[len(branchRefPrefix):]
	if short == "" {
		return "", false
	}
	return short, true
}

// parseSymrefHead extracts the branch a remote's HEAD points at from
// `ls-remote --symref` output. It scans for the "ref: <ref>\tHEAD" symref line
// and returns <ref>; a detached remote HEAD (no such line) reports false.
func parseSymrefHead(out []byte) (RefName, bool) {
	for _, line := range strings.Split(string(out), "\n") {
		rest, ok := strings.CutPrefix(line, "ref: ")
		if !ok {
			continue
		}
		tab := strings.IndexByte(rest, '\t')
		if tab < 0 {
			continue
		}
		if rest[tab+1:] != "HEAD" {
			continue
		}
		return RefName(rest[:tab]), true
	}
	return "", false
}
