package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `artifact backlink` operation: it stamps the deterministic
// docket:backlink managed block at the TOP of a spec/plan/results artifact in the
// FEATURE worktree, pointing home to its change record. It is the one operation
// in this slice that writes a working-tree file rather than driving a metadata
// transaction — the plan-writer commits the file afterward. It renders through
// render.BacklinkContent and rewrites the managed block through internal/document,
// so a malformed backlink marker (dangling/out-of-order/nested) fails the
// document parse and the file is left untouched. The write target is contained:
// an absolute path, a ../ escape, and a symlink escaping the worktree are each
// rejected before any byte is written, and every symlink hop is canonicalised so
// one physical file cannot answer to two names (learning
// canonicalise-every-symlink-hop). Same inputs yield a byte-identical file.

// OperationArtifactBacklink is the operation key `artifact backlink` records in
// its result envelope.
const OperationArtifactBacklink = "artifact.backlink"

// backlinkBlockName and backlinkBlockAnnotation are the managed block's marker
// identity. They mirror render.BacklinkContent's marker spelling exactly, so a
// block this operation inserts round-trips through render on the next run and the
// write is idempotent.
const (
	backlinkBlockName       = "backlink"
	backlinkBlockAnnotation = "generated — do not hand-edit"
)

// The stable machine reasons `artifact backlink` reports for its typed refusals.
// Message text is explanatory and must not be parsed.
const (
	// ReasonBacklinkAbsolutePath: the artifact path is absolute; paths crossing
	// the CLI are canonical repository-relative (Global Constraints).
	ReasonBacklinkAbsolutePath = "absolute-path"
	// ReasonBacklinkPathEscape: the artifact path escapes the worktree with a
	// `..` traversal.
	ReasonBacklinkPathEscape = "path-escape"
	// ReasonBacklinkSymlinkEscape: a symlink hop on the artifact path resolves to
	// a physical location outside the worktree.
	ReasonBacklinkSymlinkEscape = "symlink-escape"
	// ReasonBacklinkArtifactNotFound: the artifact file does not exist. Backlinks
	// are stamped onto an authored artifact, never created from nothing.
	ReasonBacklinkArtifactNotFound = "artifact-not-found"
	// ReasonBacklinkMalformedMarkers: the artifact's managed-block population is
	// malformed (dangling/out-of-order/nested markers); the block is not rewritten
	// and the file is left untouched.
	ReasonBacklinkMalformedMarkers = "malformed-markers"
	// ReasonBacklinkUnknownChange: the --change path names no record in the
	// corpus, so no backlink can be rendered.
	ReasonBacklinkUnknownChange = "unknown-change"
	// ReasonBacklinkRepoUnreadable: the worktree root cannot be canonicalised.
	ReasonBacklinkRepoUnreadable = "repo-unreadable"
	// ReasonBacklinkArtifactUnreadable: the artifact path cannot be resolved on
	// disk for a reason other than plain absence.
	ReasonBacklinkArtifactUnreadable = "artifact-unreadable"
)

// The closed set of dispositions a successful result carries.
const (
	backlinkDispositionRendered  = "rendered"
	backlinkDispositionUnchanged = "unchanged"
)

// ArtifactBacklinkRequest is the closed request: the artifact to stamp and the
// change to point home to, both canonical repository-relative paths. ArtifactPath
// is resolved inside the feature worktree (repoDir); ChangePath keys the change
// record in the pinned corpus.
type ArtifactBacklinkRequest struct {
	ArtifactPath string `json:"artifact_path"`
	ChangePath   string `json:"change_path"`
}

// ArtifactBacklinkResult is the protocol-v1 document the operation returns. It
// names only paths and a disposition — never the artifact's authored bytes
// (redaction).
type ArtifactBacklinkResult struct {
	Envelope
	Artifact    string `json:"artifact,omitempty"`
	Change      string `json:"change,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Message     string `json:"message,omitempty"`
}

// HumanText renders the one-line human summary. It carries paths and disposition
// only, never an authored document body (redaction constraint).
func (r ArtifactBacklinkResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		return fmt.Sprintf("artifact backlink: rendered into %s (change %s)", r.Artifact, r.Change)
	case ResultNoOp:
		return fmt.Sprintf("artifact backlink: %s already current (change %s)", r.Artifact, r.Change)
	default:
		if r.Reason != "" {
			return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
		}
		return fmt.Sprintf("%s: %s", r.Operation, r.Result)
	}
}

// newBacklinkResult stamps the envelope.
func newBacklinkResult(result Result, r ArtifactBacklinkResult) ArtifactBacklinkResult {
	r.Envelope = NewEnvelope(OperationArtifactBacklink, result)
	return r
}

// backlinkRefusal builds a refusing result carrying a stable reason and message.
func backlinkRefusal(result Result, reason, message string) ArtifactBacklinkResult {
	return newBacklinkResult(result, ArtifactBacklinkResult{Reason: reason, Message: message})
}

// ArtifactBacklink stamps the docket:backlink block onto the artifact at
// req.ArtifactPath, pointing home to the change at req.ChangePath. It reads the
// change from one pinned corpus, renders the block deterministically, rewrites
// the managed block through the marker-validating document layer, and writes the
// file only when its bytes change. Every refusal predates the write, so a refused
// call leaves the file byte-identical.
func ArtifactBacklink(ctx context.Context, deps PlanningDeps, repoDir string, req ArtifactBacklinkRequest) ArtifactBacklinkResult {
	// 1. Resolve and contain the write target inside the worktree root.
	root := repoDir
	if strings.TrimSpace(root) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return backlinkRefusal(ResultInternalError, ReasonBacklinkRepoUnreadable, err.Error())
		}
		root = wd
	}
	target, reason := containedArtifactPath(root, req.ArtifactPath)
	if reason != "" {
		result := ResultInvalidInput
		if reason == ReasonBacklinkSymlinkEscape || reason == ReasonBacklinkRepoUnreadable {
			result = ResultInvalidState
		}
		return backlinkRefusal(result, reason,
			fmt.Sprintf("artifact path %q is not a contained repository-relative path", req.ArtifactPath))
	}

	// 2. Read the authored artifact. It must already exist — a backlink is stamped
	//    onto an authored file, never conjured from nothing.
	original, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return backlinkRefusal(ResultInvalidInput, ReasonBacklinkArtifactNotFound,
				fmt.Sprintf("artifact %q does not exist", req.ArtifactPath))
		}
		return backlinkRefusal(ResultInvalidState, ReasonBacklinkArtifactUnreadable, err.Error())
	}

	// 3. Parse the artifact — the whole-population marker validation. A malformed
	//    backlink marker (dangling/out-of-order/nested) fails here and the file is
	//    left untouched.
	doc, err := document.Parse(original)
	if err != nil {
		return backlinkRefusal(ResultInvalidState, ReasonBacklinkMalformedMarkers,
			fmt.Sprintf("artifact %q has a malformed managed-block population: %v", req.ArtifactPath, err))
	}

	// 4. Resolve the target change from one pinned corpus read.
	change, refusal := resolveBacklinkChange(ctx, deps, repoDir, req.ChangePath)
	if refusal != nil {
		return *refusal
	}

	// 5. Render the deterministic backlink block and reduce it to the interior the
	//    document layer manages between the markers it owns.
	block, err := render.BacklinkContent(change.change, change.link)
	if err != nil {
		return backlinkRefusal(ResultInternalError, ReasonStatusInternalError, err.Error())
	}
	interior := backlinkInterior(block)

	// 6. Rewrite (or insert) the managed block.
	var ps document.PatchSet
	if _, ok := doc.Block(backlinkBlockName); ok {
		ps.ReplaceBlock(backlinkBlockName, interior)
	} else {
		at := document.AtDocumentStart
		if doc.HasFrontmatter() {
			at = document.AfterFrontmatter
		}
		ps.InsertBlock(backlinkBlockName, backlinkBlockAnnotation, interior, at)
	}
	updated, err := doc.Apply(ps)
	if err != nil {
		return backlinkRefusal(ResultInvalidState, ReasonBacklinkMalformedMarkers,
			fmt.Sprintf("rewriting the backlink block in %q: %v", req.ArtifactPath, err))
	}

	applied := ArtifactBacklinkResult{Artifact: req.ArtifactPath, Change: change.change.Path()}

	// 7. Idempotent write: unchanged bytes are a no-op, so a re-run yields a
	//    byte-identical file and no needless mtime churn.
	if string(updated) == string(original) {
		applied.Disposition = backlinkDispositionUnchanged
		return newBacklinkResult(ResultNoOp, applied)
	}
	mode := os.FileMode(0o644)
	if fi, statErr := os.Stat(target); statErr == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.WriteFile(target, updated, mode); err != nil {
		return backlinkRefusal(ResultInvalidState, ReasonBacklinkArtifactUnreadable, err.Error())
	}
	applied.Disposition = backlinkDispositionRendered
	return newBacklinkResult(ResultApplied, applied)
}

// backlinkChange bundles a resolved change with the link context its backlink
// renders under.
type backlinkChange struct {
	change domain.Change
	link   render.LinkContext
}

// resolveBacklinkChange pins context once, reads the corpus once, builds the
// snapshot, and returns the change whose canonical path equals changePath. An
// absent change is a typed unknown-change refusal — never a fabricated backlink.
func resolveBacklinkChange(ctx context.Context, deps PlanningDeps, repoDir, changePath string) (backlinkChange, *ArtifactBacklinkResult) {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := backlinkRefusal(result, reason, err.Error())
		return backlinkChange{}, &r
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := backlinkRefusal(result, reason, err.Error())
		return backlinkChange{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		r := backlinkRefusal(ResultInternalError, ReasonStatusInternalError, err.Error())
		return backlinkChange{}, &r
	}
	for _, c := range build.Snapshot.Changes() {
		if c.Path() == changePath {
			return backlinkChange{
				change: c,
				link:   linkContextOf(pin),
			}, nil
		}
	}
	r := backlinkRefusal(ResultInvalidInput, ReasonBacklinkUnknownChange,
		fmt.Sprintf("no change record is present at %q in the corpus", changePath))
	return backlinkChange{}, &r
}

// containedArtifactPath resolves rel against the worktree root and proves the
// physical write target stays inside it. It returns the absolute target on
// success, or a stable refusal reason. Every symlink hop is canonicalised
// (learning canonicalise-every-symlink-hop): an absolute symlink target is still
// a spelling that can traverse another link, so identity is decided by physical
// location, never by a trusted string.
func containedArtifactPath(root, rel string) (string, string) {
	if strings.TrimSpace(rel) == "" {
		return "", ReasonBacklinkPathEscape
	}
	if filepath.IsAbs(rel) {
		return "", ReasonBacklinkAbsolutePath
	}
	// IsLocal rejects any path that escapes the directory it is evaluated in — a
	// leading or interior `..` that climbs out, or an absolute path.
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return "", ReasonBacklinkPathEscape
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", ReasonBacklinkRepoUnreadable
	}
	target := filepath.Join(rootReal, filepath.FromSlash(rel))

	// A symlink leaf is resolved explicitly and must stay contained; a broken or
	// unresolvable symlink leaf fails closed. This closes the gap resolveEveryHop
	// leaves for a symlink whose target does not yet exist.
	if li, lerr := os.Lstat(target); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
		resolved, rerr := filepath.EvalSymlinks(target)
		if rerr != nil || !withinRoot(rootReal, resolved) {
			return "", ReasonBacklinkSymlinkEscape
		}
	}

	// Canonicalise every hop of the deepest existing prefix; an escaping
	// intermediate directory or existing leaf surfaces in the physical path.
	real, err := resolveEveryHop(target)
	if err != nil {
		return "", ReasonBacklinkArtifactUnreadable
	}
	if !withinRoot(rootReal, real) {
		return "", ReasonBacklinkSymlinkEscape
	}
	return target, ""
}

// withinRoot reports whether physical path p is the root itself or lies beneath
// it, comparing canonicalised absolute paths.
func withinRoot(rootReal, p string) bool {
	return p == rootReal || strings.HasPrefix(p, rootReal+string(os.PathSeparator))
}

// resolveEveryHop canonicalises p by following every symlink hop. It resolves the
// deepest existing prefix with filepath.EvalSymlinks (which follows every hop of
// what it resolves) and re-appends the not-yet-existing tail, so a fresh leaf
// under a real directory resolves to a contained path while any symlink hop that
// escapes the tree — intermediate directory or existing leaf — surfaces in the
// returned physical path.
func resolveEveryHop(p string) (string, error) {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real, nil
	}
	parent := filepath.Dir(p)
	if parent == p {
		return "", fmt.Errorf("no existing ancestor for %q", p)
	}
	parentReal, err := resolveEveryHop(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(parentReal, filepath.Base(p)), nil
}
