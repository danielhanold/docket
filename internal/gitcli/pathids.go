package gitcli

import (
	"bytes"
	"context"
	"encoding/hex"
	"strconv"
	"strings"
)

// ObjectPath identifies a literal path within an immutable root tree.
type ObjectPath struct {
	Tree ObjectID
	Path RepoPath
}
type pathEntry struct {
	id        ObjectID
	directory bool
}

// PathObjectIDs reads tree objects in batches by path depth, caching each tree
// once per call. Absence means no entry in a readable parent tree, never an
// unavailable target object. Leaf IDs (including gitlinks) are preserved without
// dereferencing them, matching ls-tree. Callers still prove complete tree equality.
func (c *Client) PathObjectIDs(ctx context.Context, repo Repository, paths []ObjectPath) ([]ObjectID, error) {
	type cursor struct {
		tree  ObjectID
		parts []string
	}
	cursors := make([]cursor, len(paths))
	for i, p := range paths {
		if err := validateObjectID(p.Tree); err != nil {
			return nil, newFailure(treeEntriesOp, KindInvalidRequest, "invalid root tree", err)
		}
		if err := validateRepoPath(p.Path, false); err != nil {
			return nil, newFailure(treeEntriesOp, KindInvalidRequest, "invalid tree path", err)
		}
		cursors[i] = cursor{p.Tree, strings.Split(string(p.Path), "/")}
	}
	ids := make([]ObjectID, len(paths))
	cache := map[ObjectID]map[string]pathEntry{}
	for {
		var needed []ObjectID
		seen := map[ObjectID]bool{}
		active := false
		for _, cur := range cursors {
			if len(cur.parts) == 0 {
				continue
			}
			active = true
			if _, ok := cache[cur.tree]; !ok && !seen[cur.tree] {
				needed = append(needed, cur.tree)
				seen[cur.tree] = true
			}
		}
		if !active {
			return ids, nil
		}
		if len(needed) > 0 {
			var input strings.Builder
			for _, id := range needed {
				input.WriteString(string(id))
				input.WriteByte('\n')
			}
			res, fail := c.run(ctx, runRequest{op: treeEntriesOp, dir: repo.PrimaryWorktree, args: []string{"cat-file", "--batch", "--buffer"}, stdin: []byte(input.String())})
			if fail != nil {
				return nil, fail
			}
			if res.exitCode != 0 {
				return nil, newFailure(treeEntriesOp, KindCommandFailed, "tree batch failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
			}
			trees, err := parsePathTrees(res.stdout, needed)
			if err != nil {
				return nil, err
			}
			for id, tree := range trees {
				cache[id] = tree
			}
		}
		for i, cur := range cursors {
			if len(cur.parts) == 0 {
				continue
			}
			entry, ok := cache[cur.tree][cur.parts[0]]
			if !ok {
				cursors[i].parts = nil
				continue
			}
			if len(cur.parts) == 1 {
				ids[i] = entry.id
				cursors[i].parts = nil
				continue
			}
			if !entry.directory {
				cursors[i].parts = nil
				continue
			}
			cursors[i] = cursor{entry.id, cur.parts[1:]}
		}
	}
}

// Tree objects encode mode SP name NUL raw-object-id, whose width follows the
// repository's object format. Frame sizes and IDs are checked before parsing;
// a missing/truncated object never yields partial or absent-path results.
func parsePathTrees(out []byte, want []ObjectID) (map[ObjectID]map[string]pathEntry, error) {
	bad := func() (map[ObjectID]map[string]pathEntry, error) {
		return nil, newFailure(treeEntriesOp, KindInvalidOutput, "invalid or unavailable tree batch", nil)
	}
	trees := map[ObjectID]map[string]pathEntry{}
	for _, id := range want {
		nl := bytes.IndexByte(out, '\n')
		if nl < 0 {
			return bad()
		}
		fields := strings.Split(string(out[:nl]), " ")
		if len(fields) != 3 || fields[0] != string(id) || fields[1] != "tree" {
			return bad()
		}
		size, err := strconv.Atoi(fields[2])
		out = out[nl+1:]
		if err != nil || size < 0 || size >= len(out) || out[size] != '\n' {
			return bad()
		}
		body := out[:size]
		out = out[size+1:]
		entries := map[string]pathEntry{}
		for len(body) > 0 {
			space := bytes.IndexByte(body, ' ')
			if space < 0 {
				return bad()
			}
			mode := string(body[:space])
			body = body[space+1:]
			if mode != "40000" && mode != "100644" && mode != "100755" && mode != "120000" && mode != "160000" {
				return bad()
			}
			zero := bytes.IndexByte(body, 0)
			width := len(id) / 2
			if zero <= 0 || len(body)-zero-1 < width {
				return bad()
			}
			name := string(body[:zero])
			if strings.Contains(name, "/") || name == "." || name == ".." {
				return bad()
			}
			if _, exists := entries[name]; exists {
				return bad()
			}
			oid := ObjectID(hex.EncodeToString(body[zero+1 : zero+1+width]))
			entries[name] = pathEntry{oid, mode == "40000"}
			body = body[zero+1+width:]
		}
		trees[id] = entries
	}
	if len(out) != 0 {
		return bad()
	}
	return trees, nil
}
