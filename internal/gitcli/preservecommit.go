package gitcli

import (
	"bytes"
	"context"
	"errors"
	"strings"
)

// Operation labels for the preservation-proof plumbing. They follow the
// file-local convention the sibling read surfaces use (see history.go's
// sharedAncestryOp/listHistoryOp/treeEntriesOp).
const (
	preserveOp    Operation = "preserve-proof"
	mergeBasesOp  Operation = "merge-bases"
	sourceDeltaOp Operation = "source-delta"
)

// deltaEntry is one changed tracked entry from base to source, rename detection
// disabled: a rename surfaces as a delete of the old path plus an add of the
// new one, never a single rename record. Only the source-side (final) mode and
// oid are retained — a preservation proof asks what the source's tree holds, so
// a source deletion carries the "000000" mode and the all-zero oid (git's
// sentinel for an absent side).
type deltaEntry struct {
	Path    RepoPath // repo-relative path, verbatim bytes
	Status  byte     // 'A','M','D','T' — diff-tree raw status first byte under --no-renames
	NewMode FileMode // final mode at source ("000000" when Status=='D')
	NewOID  ObjectID // final oid at source (all-zero when Status=='D')
}

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

// commitDelta returns every tracked entry the source commit changes relative to
// base, via `diff-tree -r -z --no-renames --full-index base source`. Rename
// detection is OFF so a move is a delete plus an add (never a rename record);
// -z keeps the records NUL-safe against hostile path bytes (spaces, tabs,
// embedded newlines, non-UTF8), and --full-index pins complete object ids for
// the exact-content comparison. A nonzero exit is a typed command-failed
// *Failure; malformed plumbing output is invalid-output.
func (c *Client) commitDelta(ctx context.Context, repo Repository, base, source ObjectID) ([]deltaEntry, *Failure) {
	if err := validateObjectID(base); err != nil {
		return nil, newFailure(sourceDeltaOp, KindInvalidRequest, "invalid base commit id", err)
	}
	if err := validateObjectID(source); err != nil {
		return nil, newFailure(sourceDeltaOp, KindInvalidRequest, "invalid source commit id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   sourceDeltaOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"diff-tree", "-r", "-z", "--no-renames", "--full-index", string(base), string(source)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(sourceDeltaOp, KindCommandFailed, "diff-tree failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	entries, err := parseDiffTreeRawZ(res.stdout)
	if err != nil {
		return nil, newFailure(sourceDeltaOp, KindInvalidOutput, "malformed diff-tree raw output", err)
	}
	return entries, nil
}

// parseDiffTreeRawZ parses the NUL-delimited raw records of a
// `diff-tree -r -z --no-renames --full-index base source` stream into the
// changed source-side entries. Each change is two NUL-terminated fields — a
// ":<oldmode> <newmode> <oldoid> <newoid> <status>" header then the path — so
// the byte stream is header/path pairs ending in a trailing NUL. It mirrors
// parseLsTreeEntriesZ's record discipline: every record is NUL-terminated, the
// header begins with ':' and splits into exactly five space-separated fields,
// both object ids are valid full hex, the status is a single byte in the
// --no-renames set {A,D,M,T}, and the all-zero oid is accepted only as a
// deletion's (vanished) new side. Any violation is an error and no entries are
// returned; an empty stream is no entries and no error.
func parseDiffTreeRawZ(out []byte) ([]deltaEntry, error) {
	if len(out) == 0 {
		return nil, nil
	}
	recs := bytes.Split(out, []byte{0})
	if len(recs[len(recs)-1]) != 0 {
		return nil, errors.New("gitcli: diff-tree raw output has an unterminated trailing record")
	}
	recs = recs[:len(recs)-1]
	if len(recs)%2 != 0 {
		return nil, errors.New("gitcli: diff-tree raw output has a header without a path")
	}
	entries := make([]deltaEntry, 0, len(recs)/2)
	for i := 0; i < len(recs); i += 2 {
		header := recs[i]
		path := recs[i+1]
		if len(path) == 0 {
			return nil, errors.New("gitcli: diff-tree raw record has an empty path")
		}
		if len(header) == 0 || header[0] != ':' {
			return nil, errors.New("gitcli: diff-tree raw header does not begin with ':'")
		}
		fields := bytes.Split(header[1:], []byte{' '})
		if len(fields) != 5 {
			return nil, errors.New("gitcli: diff-tree raw header is not five fields")
		}
		newMode := FileMode(fields[1])
		oldOID := ObjectID(fields[2])
		newOID := ObjectID(fields[3])
		statusField := fields[4]
		if err := validateObjectID(oldOID); err != nil {
			return nil, err
		}
		if err := validateObjectID(newOID); err != nil {
			return nil, err
		}
		if len(statusField) == 0 {
			return nil, errors.New("gitcli: diff-tree raw record has an empty status")
		}
		status := statusField[0]
		switch status {
		case 'A', 'D', 'M', 'T':
		default:
			return nil, errors.New("gitcli: diff-tree raw status outside the --no-renames set {A,D,M,T}")
		}
		// The all-zero oid is git's sentinel for an absent side; under
		// --no-renames it is legal only as a deletion's vanished new side.
		if isAllZeroOID(newOID) && status != 'D' {
			return nil, errors.New("gitcli: diff-tree raw all-zero new oid on a non-deletion record")
		}
		entries = append(entries, deltaEntry{
			Path:    RepoPath(path),
			Status:  status,
			NewMode: newMode,
			NewOID:  newOID,
		})
	}
	return entries, nil
}

// PreservationOutcome is the closed top-level verdict of a preservation query.
type PreservationOutcome string

const (
	// PreservationProven: source's work is demonstrably present in target.
	PreservationProven PreservationOutcome = "proven"
	// PreservationUnproven: the query completed but could not demonstrate
	// preservation. Uncertainty is unproven — never a fabricated proof, never an
	// error.
	PreservationUnproven PreservationOutcome = "unproven"
)

// PreservationKind names HOW a proven verdict was established; it is set only
// when the outcome is PreservationProven.
type PreservationKind string

const (
	// PreservationByAncestry: source is an ancestor of target, so every one of
	// source's objects is reachable from target unchanged.
	PreservationByAncestry PreservationKind = "ancestry"
	// PreservationByContent: source is not an ancestor, but the complete
	// base->source tracked-entry delta reproduces exactly in target (oid+mode,
	// deletions absent).
	PreservationByContent PreservationKind = "exact-content"
)

// Closed unproven-detail tokens — stable machine strings a caller may branch on.
// Detail carries exactly one of these when the outcome is unproven; it is "" on
// a proven verdict.
const (
	// PreserveNoCommonBase: source and target share no common ancestor, so there
	// is no base against which to compare a delta.
	PreserveNoCommonBase = "no-common-base"
	// PreserveMultipleBases: source and target have more than one merge base
	// (criss-cross history); picking an arbitrary base could manufacture a false
	// proof, so the query refuses rather than choose.
	PreserveMultipleBases = "multiple-bases"
	// PreserveEmptyDelta: source changes nothing relative to the sole base, so
	// there is no work to prove preserved; a proof from an empty population would
	// be vacuous.
	PreserveEmptyDelta = "empty-delta"
	// PreserveEntryDiffers: at least one base->source changed entry is absent or
	// differs (by oid or mode) in target; the differing paths are in Paths.
	PreserveEntryDiffers = "entry-differs"
)

// PreservationCheck is the full result of a preservation query: the verdict, how
// a proof was reached, a closed detail token when unproven, and a bounded set of
// differing diagnostic paths.
type PreservationCheck struct {
	Outcome PreservationOutcome
	Kind    PreservationKind // set only when proven
	Detail  string           // closed token when unproven; "" when proven
	Paths   []RepoPath       // bounded diagnostic (<= 8 differing paths)
}

// maxDifferingPaths bounds the diagnostic Paths slice; a mismatch beyond the cap
// is still recorded as a mismatch (the outcome stays unproven), only its path is
// omitted from the diagnostic.
const maxDifferingPaths = 8

// ProvePreserved reports whether source's work (source = a child's authoritative
// merge-result commit) is preserved in target. Ancestry proves outright:
// everything source holds is reachable from target. Otherwise the FULL
// base->source tracked-entry delta must reproduce exactly in target — every
// changed entry present at the same oid and mode, and every source deletion
// absent — where base is the SINGLE merge base of source and target. Zero, or
// more than one, merge base is an unproven refusal (never an arbitrary pick), as
// is an empty delta. Uncertainty is unproven; inability to observe an operand
// (invalid, missing, non-commit, or shallow) is an error, never a verdict. The
// comparison reads Git objects only — no merge drivers, filters, or rename
// detection can manufacture a proof.
func (c *Client) ProvePreserved(ctx context.Context, repo Repository, remote RemoteName, source, target ObjectID) (PreservationCheck, error) {
	if f := c.ensurePreservationInputs(ctx, repo, remote, source, target); f != nil {
		return PreservationCheck{}, f
	}
	ok, err := c.IsAncestor(ctx, repo, source, target)
	if err != nil {
		return PreservationCheck{}, err
	}
	if ok {
		return PreservationCheck{Outcome: PreservationProven, Kind: PreservationByAncestry}, nil
	}
	bases, f := c.mergeBasesAll(ctx, repo, source, target)
	if f != nil {
		return PreservationCheck{}, f
	}
	if len(bases) == 0 {
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveNoCommonBase}, nil
	}
	if len(bases) > 1 {
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveMultipleBases}, nil
	}
	delta, f := c.commitDelta(ctx, repo, bases[0], source)
	if f != nil {
		return PreservationCheck{}, f
	}
	if len(delta) == 0 {
		// Conservative: never manufacture proof from an empty population.
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveEmptyDelta}, nil
	}
	paths := make([]RepoPath, 0, len(delta))
	for _, d := range delta {
		paths = append(paths, d.Path)
	}
	entries, err := c.TreeEntryIDs(ctx, repo, target, paths)
	if err != nil {
		return PreservationCheck{}, err
	}
	// A directory-valued path (a file->directory transition) yields a single
	// `tree` entry whose mode 040000 can never equal a blob's NewMode, so the
	// comparison below stays conservative for it.
	at := make(map[RepoPath]TreeEntry, len(entries))
	for _, e := range entries {
		at[e.Path] = e
	}
	// Track "a mismatch exists" separately from the bounded diagnostic slice: the
	// path cap must never shorten the verdict — even a mismatch past the cap keeps
	// the outcome unproven.
	mismatched := false
	var differing []RepoPath
	for _, d := range delta {
		e, present := at[d.Path]
		match := false
		if d.Status == 'D' {
			match = !present // a source deletion requires absence in target
		} else {
			match = present && e.ObjectID == d.NewOID && e.Mode == d.NewMode
		}
		if !match {
			mismatched = true
			if len(differing) < maxDifferingPaths {
				differing = append(differing, d.Path)
			}
		}
	}
	if mismatched {
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveEntryDiffers, Paths: differing}, nil
	}
	return PreservationCheck{Outcome: PreservationProven, Kind: PreservationByContent}, nil
}

// isAllZeroOID reports whether id is the all-zero object id git emits for an
// absent side of a raw diff record (40 or 64 zeros); it is legal on a source
// deletion only.
func isAllZeroOID(id ObjectID) bool {
	s := string(id)
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}
