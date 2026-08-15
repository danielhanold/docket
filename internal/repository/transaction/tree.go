package transaction

import (
	"bytes"
	"context"
	"errors"
	"sort"

	"github.com/danielhanold/docket/internal/gitcli"
)

// Tree is the read-only subset a repository loader needs: the base revision it
// reads from, a prefix-scoped tree listing, and batch blob reads. Both a raw
// base tree and a plan-layered overlay implement it, so a loader validates the
// before and after states through one interface.
type Tree interface {
	Revision() gitcli.Revision // overlay reports its base revision
	ListTree(ctx context.Context, prefixes []gitcli.RepoPath) ([]gitcli.TreeEntry, error)
	ReadBlobs(ctx context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error)
}

// baseTree adapts a gitcli.ObjectSource to the Tree interface with no layering.
type baseTree struct {
	src gitcli.ObjectSource
}

// newBaseTree returns a Tree that reads directly from src.
func newBaseTree(src gitcli.ObjectSource) Tree { return &baseTree{src: src} }

func (b *baseTree) Revision() gitcli.Revision { return b.src.Revision() }

func (b *baseTree) ListTree(ctx context.Context, prefixes []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	return b.src.ListTree(ctx, prefixes)
}

func (b *baseTree) ReadBlobs(ctx context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	return b.src.ReadBlobs(ctx, paths)
}

// overlayAction is the resolved effect of one plan file on the base tree.
type overlayAction struct {
	kind  MutationKind
	mode  gitcli.FileMode // create: 100644; replace: base mode; unused for delete
	bytes []byte          // snapshot of plan bytes; nil for delete
}

// overlayTree layers a validated plan over a base tree. Created and replaced
// paths resolve to the plan's bytes (mode 100644 for creates, base mode for
// replaces, ObjectID "" — loaders read bytes, not IDs, from an overlay); deleted
// paths vanish from both ListTree and ReadBlobs. Untouched paths pass through to
// the base unchanged.
type overlayTree struct {
	base    Tree
	actions map[gitcli.RepoPath]overlayAction
}

// isRegularBlobMode reports whether mode names a regular file blob (not a
// symlink 120000 or gitlink 160000).
func isRegularBlobMode(mode gitcli.FileMode) bool {
	return mode == "100644" || mode == "100755"
}

// newOverlayTree validates the plan's intrinsic shape and its before/after mode
// rules against base, then returns a Tree layering the plan over base. It
// rejects a plan whose replace/delete target is absent from base, whose create
// target already exists in base, or whose replace/delete target is not a regular
// blob (a symlink or gitlink). These checks run once here so every engine
// attempt inherits the exact before/after mode rules. Plan bytes are snapshotted
// so later caller mutation of the plan cannot change what the overlay serves.
//
// Base membership and modes are read through base.ListTree; the interface's
// methods take a context, but construction is synchronous and uses
// context.Background so the constructor stays context-free per its signature.
func newOverlayTree(base Tree, plan MutationPlan) (Tree, error) {
	if err := validatePlan(plan); err != nil {
		return nil, err
	}

	entries, err := base.ListTree(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	baseModes := make(map[gitcli.RepoPath]gitcli.FileMode, len(entries))
	for _, e := range entries {
		baseModes[e.Path] = e.Mode
	}

	actions := make(map[gitcli.RepoPath]overlayAction, len(plan.Files))
	for _, f := range plan.Files {
		baseMode, present := baseModes[f.Path]
		switch f.Kind {
		case MutationCreate:
			if present {
				return nil, errors.New("transaction: overlay create target already exists in base")
			}
			actions[f.Path] = overlayAction{kind: MutationCreate, mode: "100644", bytes: cloneBytes(f.Bytes)}
		case MutationReplace:
			if !present {
				return nil, errors.New("transaction: overlay replace target absent from base")
			}
			if !isRegularBlobMode(baseMode) {
				return nil, errors.New("transaction: overlay replace target is not a regular blob")
			}
			actions[f.Path] = overlayAction{kind: MutationReplace, mode: baseMode, bytes: cloneBytes(f.Bytes)}
		case MutationDelete:
			if !present {
				return nil, errors.New("transaction: overlay delete target absent from base")
			}
			if !isRegularBlobMode(baseMode) {
				return nil, errors.New("transaction: overlay delete target is not a regular blob")
			}
			actions[f.Path] = overlayAction{kind: MutationDelete}
		default:
			// validatePlan already rejected unknown kinds; defense in depth.
			return nil, errors.New("transaction: overlay file has unknown mutation kind")
		}
	}

	return &overlayTree{base: base, actions: actions}, nil
}

// cloneBytes returns a fresh copy of b (nil for empty input).
func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func (o *overlayTree) Revision() gitcli.Revision { return o.base.Revision() }

// ListTree merges the base listing (minus deletes, with replaces re-moded and
// their ids cleared) with created entries matching the requested prefixes, then
// returns the union in deterministic path-sorted order.
func (o *overlayTree) ListTree(ctx context.Context, prefixes []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	baseEntries, err := o.base.ListTree(ctx, prefixes)
	if err != nil {
		return nil, err
	}

	out := make([]gitcli.TreeEntry, 0, len(baseEntries)+len(o.actions))
	for _, e := range baseEntries {
		act, ok := o.actions[e.Path]
		if !ok {
			out = append(out, e)
			continue
		}
		switch act.kind {
		case MutationDelete:
			// vanishes from the listing
		case MutationReplace:
			out = append(out, gitcli.TreeEntry{Path: e.Path, Mode: e.Mode, Type: "blob", ObjectID: ""})
		default:
			// A create colliding with a base entry is impossible after validation.
			out = append(out, e)
		}
	}

	// Add created paths that fall within the requested prefixes.
	for path, act := range o.actions {
		if act.kind != MutationCreate {
			continue
		}
		if !overlayMatchesPrefixes(path, prefixes) {
			continue
		}
		out = append(out, gitcli.TreeEntry{Path: path, Mode: act.mode, Type: "blob", ObjectID: ""})
	}

	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare([]byte(out[i].Path), []byte(out[j].Path)) < 0
	})
	return out, nil
}

// ReadBlobs serves created/replaced paths from the plan's snapshotted bytes,
// reports deleted paths as not found, and delegates every untouched path to the
// base in one batch. Results are returned in request order.
func (o *overlayTree) ReadBlobs(ctx context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	out := make([]gitcli.BlobResult, len(paths))

	var baseWant []gitcli.RepoPath
	var baseSlots []int
	for i, p := range paths {
		out[i].Path = p
		act, ok := o.actions[p]
		if !ok {
			baseWant = append(baseWant, p)
			baseSlots = append(baseSlots, i)
			continue
		}
		switch act.kind {
		case MutationDelete:
			out[i].Found = false
		case MutationCreate, MutationReplace:
			out[i].Found = true
			out[i].Blob = gitcli.Blob{Mode: act.mode, ObjectID: "", Bytes: cloneBytes(act.bytes)}
		}
	}

	if len(baseWant) > 0 {
		baseRes, err := o.base.ReadBlobs(ctx, baseWant)
		if err != nil {
			return nil, err
		}
		if len(baseRes) != len(baseWant) {
			return nil, errors.New("transaction: base ReadBlobs returned a mismatched result count")
		}
		for k, r := range baseRes {
			out[baseSlots[k]] = r
		}
	}

	return out, nil
}

// overlayMatchesPrefixes replicates git's `-- <prefix>` scoping: empty prefixes
// (or one containing "") match everything; otherwise a path matches a prefix
// when it equals it or sits directly beneath it.
func overlayMatchesPrefixes(path gitcli.RepoPath, prefixes []gitcli.RepoPath) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, pre := range prefixes {
		if pre == "" || path == pre {
			return true
		}
		ps, pfx := string(path), string(pre)
		if len(ps) > len(pfx) && ps[:len(pfx)] == pfx && ps[len(pfx)] == '/' {
			return true
		}
	}
	return false
}
