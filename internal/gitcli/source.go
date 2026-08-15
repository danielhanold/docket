package gitcli

import (
	"bytes"
	"context"
	"errors"
	"sort"
)

// Operation labels for the object-source surface.
const (
	openSourceOp Operation = "open-source"
	listTreeOp   Operation = "list-tree"
)

// ObjectSource serves read-only tree listings and batch blob reads from one
// exact commit pinned at open time. The pinned commit never moves afterwards, so
// concurrent fetches advancing a tracking ref cannot change what a source sees.
type ObjectSource interface {
	Revision() Revision
	ListTree(ctx context.Context, prefixes []RepoPath) ([]TreeEntry, error)
	// ReadBlobs resolves each requested path at the pinned commit and returns
	// its exact bytes; it is implemented on *objectSource in readblobs.go.
	ReadBlobs(ctx context.Context, paths []RepoPath) ([]BlobResult, error)
}

// objectSource is the concrete ObjectSource: the client and repository it reads
// through and the revision it is pinned to, all stored by value so nothing can
// move the commit after OpenObjectSource returns.
type objectSource struct {
	client *Client
	repo   Repository
	rev    Revision
}

// OpenObjectSource verifies the revision names a commit object present in the
// local object store and returns a source pinned to it. A malformed commit id is
// invalid-request; an absent object is ref-unavailable; an object that exists but
// is not a commit (a blob or tree) is unexpected-object.
func (c *Client) OpenObjectSource(ctx context.Context, repo Repository, rev Revision) (ObjectSource, error) {
	if err := validateObjectID(rev.Commit); err != nil {
		return nil, newFailure(openSourceOp, KindInvalidRequest, "invalid revision commit id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   openSourceOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"cat-file", "-t", string(rev.Commit)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(openSourceOp, KindRefUnavailable, "commit object not present locally", nil)
	}
	lines := stdoutLines(res.stdout)
	if len(lines) != 1 {
		return nil, newFailure(openSourceOp, KindInvalidOutput, "unexpected cat-file -t output", nil)
	}
	if lines[0] != "commit" {
		return nil, newFailure(openSourceOp, KindUnexpectedObject, "revision does not name a commit", nil)
	}
	return &objectSource{client: c, repo: repo, rev: rev}, nil
}

// Revision reports the exact commit the source is pinned to.
func (s *objectSource) Revision() Revision { return s.rev }

// ListTree lists tree leaves at the pinned commit. An empty prefix slice, or a
// slice containing the empty string, lists the whole tree; otherwise output is
// scoped to the given repo-relative prefixes. Overlapping prefixes are legal and
// their results are de-duplicated. Output is read NUL-delimited so hostile path
// bytes (spaces, tabs, embedded newlines, non-ASCII) and core.quotePath=true
// never corrupt it. Any malformed record yields invalid-output with no partial
// result; an absent prefix contributes zero entries, not an error. Results are
// sorted by raw path bytes.
func (s *objectSource) ListTree(ctx context.Context, prefixes []RepoPath) ([]TreeEntry, error) {
	root := len(prefixes) == 0
	for _, p := range prefixes {
		if err := validateRepoPath(p, true); err != nil {
			return nil, newFailure(listTreeOp, KindInvalidRequest, "invalid tree prefix", err)
		}
		if p == "" {
			root = true
		}
	}

	args := []string{"ls-tree", "-r", "-z", "--full-tree", string(s.rev.Commit)}
	if !root {
		args = append(args, "--")
		seen := make(map[RepoPath]struct{}, len(prefixes))
		for _, p := range prefixes {
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			args = append(args, string(p))
		}
	}

	res, f := s.client.run(ctx, runRequest{op: listTreeOp, dir: s.repo.PrimaryWorktree, args: args})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(listTreeOp, KindCommandFailed, "ls-tree failed: "+stderrExcerpt(res.stderr), nil)
	}

	entries, err := parseLsTreeZ(res.stdout)
	if err != nil {
		return nil, newFailure(listTreeOp, KindInvalidOutput, "malformed ls-tree output", err)
	}
	return sortAndDedupeEntries(entries), nil
}

// parseLsTreeZ parses the NUL-delimited records of `ls-tree -r -z` output —
// "<mode> <type> <oid>\t<path>\x00" — into typed entries. The header is split at
// the FIRST tab so a path carrying its own tab bytes stays intact; the header's
// three fields are single-space separated. Every record must be NUL-terminated
// (a non-empty trailing fragment is a truncated record), carry a tab, split into
// exactly three header fields, hold a valid mode/type pair and object id, and a
// non-empty path. Any violation is an error and no entries are returned.
func parseLsTreeZ(out []byte) ([]TreeEntry, error) {
	if len(out) == 0 {
		return nil, nil
	}
	recs := bytes.Split(out, []byte{0})
	// A well-formed -z stream terminates every record with NUL, so the final
	// split element is empty; a non-empty tail is a truncated record.
	if len(recs[len(recs)-1]) != 0 {
		return nil, errors.New("gitcli: ls-tree output has an unterminated trailing record")
	}
	recs = recs[:len(recs)-1]
	entries := make([]TreeEntry, 0, len(recs))
	for _, rec := range recs {
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
		if !validTreeLeaf(mode, typ) {
			return nil, errors.New("gitcli: ls-tree record has an unexpected mode/type")
		}
		if err := validateObjectID(oid); err != nil {
			return nil, err
		}
		entries = append(entries, TreeEntry{Path: RepoPath(path), Mode: mode, Type: typ, ObjectID: oid})
	}
	return entries, nil
}

// validTreeLeaf reports whether a mode/type pair is a legal `ls-tree -r` leaf:
// the three blob modes carry type "blob"; a gitlink (160000) carries type
// "commit". Tree entries never appear under -r, so mode 040000 is rejected.
func validTreeLeaf(mode FileMode, typ ObjectType) bool {
	switch mode {
	case "100644", "100755", "120000":
		return typ == "blob"
	case "160000":
		return typ == "commit"
	}
	return false
}

// sortAndDedupeEntries sorts entries by raw path bytes and drops duplicate paths
// (identical entries selected through overlapping prefixes).
func sortAndDedupeEntries(entries []TreeEntry) []TreeEntry {
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare([]byte(entries[i].Path), []byte(entries[j].Path)) < 0
	})
	out := entries[:0]
	var last RepoPath
	haveLast := false
	for _, e := range entries {
		if haveLast && e.Path == last {
			continue
		}
		out = append(out, e)
		last = e.Path
		haveLast = true
	}
	return out
}
