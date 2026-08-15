package gitcli

import (
	"bytes"
	"context"
	"errors"
	"strconv"
)

// readBlobsOp labels the batch-blob-read surface in any Failure.
const readBlobsOp Operation = "read-blobs"

// ReadBlobs reads the exact bytes of each requested path at the pinned commit.
// The whole input set is validated before any work: every path must satisfy the
// strict repo-path rules and no path may be requested twice (either is
// invalid-request). Empty input returns an empty result without spawning a
// process.
//
// Resolution and reading are two processes, never one-per-blob: a single
// non-recursive `ls-tree -z --full-tree <commit> -- <paths>` maps each path to
// its object — --full-tree pins resolution to the repository root so a request
// is answered identically whatever subdirectory the caller's cwd sits in —
// then a single `cat-file --batch --buffer` streams every found object's bytes.
// A path absent from the tree is reported Found:false in its slot; a path whose
// entry is a directory (tree) or a gitlink (commit) fails the whole call with
// unexpected-object. Results are returned in request order and each carries a
// fresh copy of its bytes, sharing no backing array with the parse buffer.
func (s *objectSource) ReadBlobs(ctx context.Context, paths []RepoPath) ([]BlobResult, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	seen := make(map[RepoPath]struct{}, len(paths))
	for _, p := range paths {
		if err := validateRepoPath(p, false); err != nil {
			return nil, newFailure(readBlobsOp, KindInvalidRequest, "invalid blob path", err)
		}
		if _, dup := seen[p]; dup {
			return nil, newFailure(readBlobsOp, KindInvalidRequest, "duplicate blob path in request", nil)
		}
		seen[p] = struct{}{}
	}

	// One process resolves every requested path to a typed entry.
	args := append([]string{"ls-tree", "-z", "--full-tree", string(s.rev.Commit), "--"}, repoPathsToStrings(paths)...)
	res, f := s.client.run(ctx, runRequest{op: readBlobsOp, dir: s.repo.PrimaryWorktree, args: args})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(readBlobsOp, KindCommandFailed, "ls-tree failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	byPath, err := parseLsTreeResolve(res.stdout)
	if err != nil {
		return nil, newFailure(readBlobsOp, KindInvalidOutput, "malformed ls-tree output", err)
	}

	results := make([]BlobResult, len(paths))
	// batchOrder holds the object ids fed to cat-file, in request order; slot maps
	// each batch position back to its result index; mode carries the ls-tree mode.
	var batchIDs []ObjectID
	var batchSlots []int
	var batchModes []FileMode
	for i, p := range paths {
		results[i].Path = p
		rec, ok := byPath[p]
		if !ok {
			results[i].Found = false
			continue
		}
		if rec.typ != "blob" {
			// A directory (tree) or gitlink (commit) requested as a blob.
			return nil, newFailure(readBlobsOp, KindUnexpectedObject, "requested path is not a blob", nil)
		}
		batchIDs = append(batchIDs, rec.oid)
		batchSlots = append(batchSlots, i)
		batchModes = append(batchModes, rec.mode)
	}

	if len(batchIDs) == 0 {
		return results, nil
	}

	// One process streams every found object's bytes in request order.
	var stdin bytes.Buffer
	for _, id := range batchIDs {
		stdin.WriteString(string(id))
		stdin.WriteByte('\n')
	}
	batchRes, f := s.client.run(ctx, runRequest{
		op:    readBlobsOp,
		dir:   s.repo.PrimaryWorktree,
		args:  []string{"cat-file", "--batch", "--buffer"},
		stdin: stdin.Bytes(),
	})
	if f != nil {
		return nil, f
	}
	if batchRes.exitCode != 0 {
		return nil, newFailure(readBlobsOp, KindCommandFailed, "cat-file failed: "+stderrExcerpt(batchRes.stderr), nil).withExitCode(batchRes.exitCode)
	}

	blobs, err := parseBatchBlobs(batchRes.stdout, batchIDs)
	if err != nil {
		return nil, newFailure(readBlobsOp, KindInvalidOutput, "malformed cat-file batch output", err)
	}
	for k, blob := range blobs {
		i := batchSlots[k]
		blob.Mode = batchModes[k]
		results[i].Found = true
		results[i].Blob = blob
	}
	return results, nil
}

// repoPathsToStrings converts a repo-path slice to a plain string slice for the
// git argument vector.
func repoPathsToStrings(paths []RepoPath) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = string(p)
	}
	return out
}

// lsResolveEntry is one resolved ls-tree record: the mode, type, and object id
// git reported for a path. Unlike the ListTree leaf parser, this accepts tree
// (040000) and commit (160000) entries too, so ReadBlobs can classify a
// directory or gitlink request as unexpected-object rather than malformed.
type lsResolveEntry struct {
	mode FileMode
	typ  ObjectType
	oid  ObjectID
}

// parseLsTreeResolve parses the NUL-delimited records of a non-recursive
// `ls-tree -z <commit> -- <paths>` into a path-keyed map. The header is split at
// the first tab so a path carrying its own tab stays intact. Every record must
// be NUL-terminated, carry a tab, split into exactly three header fields, hold a
// known mode/type pair and a valid object id, and a non-empty path. Any
// violation is an error and no map is returned.
func parseLsTreeResolve(out []byte) (map[RepoPath]lsResolveEntry, error) {
	m := make(map[RepoPath]lsResolveEntry)
	if len(out) == 0 {
		return m, nil
	}
	recs := bytes.Split(out, []byte{0})
	if len(recs[len(recs)-1]) != 0 {
		return nil, errors.New("gitcli: ls-tree output has an unterminated trailing record")
	}
	for _, rec := range recs[:len(recs)-1] {
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
		if !validResolveEntry(mode, typ) {
			return nil, errors.New("gitcli: ls-tree record has an unexpected mode/type")
		}
		if err := validateObjectID(oid); err != nil {
			return nil, err
		}
		m[RepoPath(path)] = lsResolveEntry{mode: mode, typ: typ, oid: oid}
	}
	return m, nil
}

// validResolveEntry reports whether a mode/type pair is a legal `ls-tree` entry
// for path resolution: the three blob modes carry type "blob"; a directory
// (040000) carries "tree"; a gitlink (160000) carries "commit".
func validResolveEntry(mode FileMode, typ ObjectType) bool {
	switch mode {
	case "100644", "100755", "120000":
		return typ == "blob"
	case "040000":
		return typ == "tree"
	case "160000":
		return typ == "commit"
	}
	return false
}

// parseBatchBlobs parses a `cat-file --batch` stream into one Blob per requested
// id, in order. Each frame is a header line "<oid> <type> <size>\n", exactly
// <size> payload bytes, then a mandatory "\n". The response order, oid, and type
// ("blob") must match the request exactly; a "missing" answer, a size that
// overruns the buffer, an absent trailing newline, or any bytes left after the
// last expected frame is an error with no partial result. Each Blob's bytes are
// a fresh copy owned by the result.
func parseBatchBlobs(out []byte, want []ObjectID) ([]Blob, error) {
	blobs := make([]Blob, len(want))
	i := 0
	for k, expID := range want {
		nl := bytes.IndexByte(out[i:], '\n')
		if nl < 0 {
			return nil, errors.New("gitcli: cat-file frame header not terminated")
		}
		header := out[i : i+nl]
		i += nl + 1
		fields := bytes.Split(header, []byte{' '})
		if len(fields) == 2 && string(fields[1]) == "missing" {
			return nil, errors.New("gitcli: cat-file reported a resolved object missing")
		}
		if len(fields) != 3 {
			return nil, errors.New("gitcli: cat-file frame header is not three fields")
		}
		oid := ObjectID(fields[0])
		typ := string(fields[1])
		if oid != expID {
			return nil, errors.New("gitcli: cat-file frame oid does not match the requested id (reordered or wrong object)")
		}
		if typ != "blob" {
			return nil, errors.New("gitcli: cat-file frame is not a blob")
		}
		size, err := strconv.Atoi(string(fields[2]))
		if err != nil || size < 0 {
			return nil, errors.New("gitcli: cat-file frame has an invalid size")
		}
		if i+size > len(out) {
			return nil, errors.New("gitcli: cat-file frame payload overruns the buffer (truncated or oversized)")
		}
		payload := make([]byte, size)
		copy(payload, out[i:i+size])
		i += size
		if i >= len(out) || out[i] != '\n' {
			return nil, errors.New("gitcli: cat-file frame payload not followed by a newline")
		}
		i++
		blobs[k] = Blob{ObjectID: oid, Bytes: payload}
	}
	if i != len(out) {
		return nil, errors.New("gitcli: cat-file stream has trailing bytes after the last expected frame")
	}
	return blobs, nil
}
