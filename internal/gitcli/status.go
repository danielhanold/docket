package gitcli

import (
	"bytes"
	"context"
	"errors"
)

// changedPathsOp labels the changed-path surface in any Failure.
const changedPathsOp Operation = "changed-paths"

// PathChange is one path differing from HEAD in a worktree. Staged reports
// whether the change is present in the index relative to HEAD (the porcelain v2
// staged column); an unstaged-only working-tree change or an untracked file is
// Staged:false.
type PathChange struct {
	Path   RepoPath
	Staged bool
}

// ChangedPaths returns the exact set of paths differing from HEAD in the
// worktree at dir — index and working tree, tracked and untracked — via
// `status --porcelain=v2 -z --untracked-files=all --no-renames`. Rename
// detection is off so a move surfaces as a delete of the source and an add of
// the destination: a safety predicate must see both sides. Output is read
// NUL-delimited so hostile path bytes never corrupt the parse. One PathChange is
// returned per path.
func (c *Client) ChangedPaths(ctx context.Context, dir string) ([]PathChange, error) {
	if dir == "" {
		return nil, newFailure(changedPathsOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	res, f := c.run(ctx, runRequest{
		op:   changedPathsOp,
		dir:  dir,
		args: []string{"status", "--porcelain=v2", "-z", "--untracked-files=all", "--no-renames"},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(changedPathsOp, KindCommandFailed, "status failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	changes, err := parseStatusV2Z(res.stdout)
	if err != nil {
		return nil, newFailure(changedPathsOp, KindInvalidOutput, "malformed status output", err)
	}
	return changes, nil
}

// parseStatusV2Z parses the NUL-delimited records of
// `status --porcelain=v2 -z --untracked-files=all --no-renames`. Recognized
// record kinds:
//
//	"1 <XY> …<path>"  — an ordinary changed entry; Staged = X != '.'.
//	"u <XY> …<path>"  — an unmerged entry; treated as staged (present in index).
//	"? <path>"        — an untracked file; Staged:false.
//	"! <path>"        — an ignored file; Staged:false (only appears with --ignored).
//
// A "2 …" rename/copy record must never appear because --no-renames is passed;
// its presence is a shape violation and an error (it would also carry a second
// NUL-terminated path field this record-oriented parser does not expect). Each
// "1"/"u" record's path is everything after its fixed header columns, split with
// a bounded count so a path carrying spaces or tabs survives intact.
func parseStatusV2Z(out []byte) ([]PathChange, error) {
	if len(out) == 0 {
		return nil, nil
	}
	recs := bytes.Split(out, []byte{0})
	// A well-formed -z stream terminates every record with NUL, so the final
	// split element is empty; a non-empty tail is a truncated record.
	if len(recs[len(recs)-1]) != 0 {
		return nil, errors.New("gitcli: status output has an unterminated trailing record")
	}
	recs = recs[:len(recs)-1]
	changes := make([]PathChange, 0, len(recs))
	for _, rec := range recs {
		if len(rec) == 0 {
			return nil, errors.New("gitcli: status emitted an empty record")
		}
		switch rec[0] {
		case '1':
			path, staged, err := parseOrdinaryEntry(rec)
			if err != nil {
				return nil, err
			}
			changes = append(changes, PathChange{Path: RepoPath(path), Staged: staged})
		case 'u':
			path, err := parseUnmergedEntry(rec)
			if err != nil {
				return nil, err
			}
			// An unmerged path is present in the index (conflicted).
			changes = append(changes, PathChange{Path: RepoPath(path), Staged: true})
		case '?', '!':
			// "? <path>" / "! <path>": the path is everything after the single
			// space following the marker.
			if len(rec) < 3 || rec[1] != ' ' {
				return nil, errors.New("gitcli: status untracked/ignored record is malformed")
			}
			changes = append(changes, PathChange{Path: RepoPath(rec[2:]), Staged: false})
		case '2':
			return nil, errors.New("gitcli: status emitted a rename/copy record despite --no-renames")
		default:
			return nil, errors.New("gitcli: status record has an unknown kind")
		}
	}
	return changes, nil
}

// parseOrdinaryEntry parses a porcelain v2 "1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>"
// record. The nine fields are single-space separated up to the path, and the
// path may itself contain spaces and tabs, so the split is bounded to keep the
// path intact. Staged is true when the staged column (X) is not '.'.
func parseOrdinaryEntry(rec []byte) (path []byte, staged bool, err error) {
	fields := bytes.SplitN(rec, []byte{' '}, 9)
	if len(fields) != 9 {
		return nil, false, errors.New("gitcli: status ordinary record has too few fields")
	}
	xy := fields[1]
	if len(xy) != 2 {
		return nil, false, errors.New("gitcli: status ordinary record has a malformed XY field")
	}
	path = fields[8]
	if len(path) == 0 {
		return nil, false, errors.New("gitcli: status ordinary record has an empty path")
	}
	return path, xy[0] != '.', nil
}

// parseUnmergedEntry parses a porcelain v2
// "u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>" record: eleven
// space-separated fields before the path. Bounded split keeps a space/tab-laden
// path intact.
func parseUnmergedEntry(rec []byte) (path []byte, err error) {
	fields := bytes.SplitN(rec, []byte{' '}, 11)
	if len(fields) != 11 {
		return nil, errors.New("gitcli: status unmerged record has too few fields")
	}
	path = fields[10]
	if len(path) == 0 {
		return nil, errors.New("gitcli: status unmerged record has an empty path")
	}
	return path, nil
}
