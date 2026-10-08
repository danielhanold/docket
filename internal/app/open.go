package app

// open.go holds `docket open`: its argument grammar, its result document (the
// six targets it accepts, the reasons a refusal carries, the notes a success
// may attach, and OpenResult, the one document every outcome renders), and
// Open, which resolves a target to a GitHub page or a file in the metadata
// checkout and hands it to the platform opener.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/repository"
)

// OperationOpen is the operation id of `docket open`.
const OperationOpen = "open"

// OpenTargets lists what `docket open` can open, in usage order. Every target
// but board takes an optional change id.
var OpenTargets = []string{"board", "change", "spec", "plan", "results", "pr"}

// Target kinds an OpenResult reports: a URL or a local file path.
const (
	OpenTargetKindURL  = "url"
	OpenTargetKindFile = "file"
)

// Reasons an OpenResult carries when it refuses or fails.
const (
	ReasonOpenInvalidTarget      = "invalid-target"
	ReasonOpenUnknownChange      = "unknown-change"
	ReasonOpenAmbiguousChange    = "ambiguous-change"
	ReasonOpenArtifactUnset      = "artifact-unset"
	ReasonOpenTrivialNoSpec      = "trivial-no-spec"
	ReasonOpenBoardDisabled      = "board-disabled"
	ReasonOpenPRNotURL           = "pr-not-url"
	ReasonOpenPathInvalid        = "artifact-path-invalid"
	ReasonOpenHeadDetached       = "head-detached"
	ReasonOpenBranchUnmatched    = "branch-unmatched"
	ReasonOpenBranchAmbiguous    = "branch-ambiguous"
	ReasonOpenCheckoutUnreadable = "checkout-unreadable"
	ReasonOpenFileMissing        = "file-missing"
	ReasonOpenNoOpener           = "no-opener"
	ReasonOpenOpenerFailed       = "opener-failed"
)

// openNoteNotGitHub is the note attached when a GitHub page was wanted but the
// origin is not a GitHub remote, so the local file was opened instead.
const openNoteNotGitHub = "origin is not a GitHub remote — opened the local file instead"

// openNoteStale is the note attached when preparing the metadata checkout
// failed with code: the local file opened anyway, but may be out of date.
func openNoteStale(code string) string {
	return "metadata checkout not synced (" + code + ") — file may be stale"
}

// OpenOptions is one `docket open` request.
type OpenOptions struct {
	RepoDir string
	What    string
	ID      int // 0 = infer from the checked-out branch
	Print   bool
}

// OpenDeps is what Open reads and runs, injected so tests start no real git
// and launch no real application.
type OpenDeps struct {
	Reader   StatusReader
	Checkout func(ctx context.Context, dir string) (gitcli.CheckoutState, error)
	Prepare  func(ctx context.Context, repoDir string) RepositoryPrepareResult
	Opener   Opener
}

// OpenResult is the document `docket open` emits for every outcome. ChangeID is
// omitted for the board; Notes is always a list, possibly empty.
type OpenResult struct {
	Envelope
	What       string   `json:"what"`
	ChangeID   int      `json:"change_id,omitempty"`
	Target     string   `json:"target,omitempty"`
	TargetKind string   `json:"target_kind,omitempty"`
	Launched   bool     `json:"launched"`
	Notes      []string `json:"notes"`
	Reason     string   `json:"reason,omitempty"`
	Message    string   `json:"message,omitempty"`
}

// HumanText renders the result for a terminal: the target alone on success
// (so `--print` composes in command substitution), the resolved target then
// the failure message when a failure follows resolution, or the message alone.
func (r OpenResult) HumanText() string {
	if r.Message == "" {
		return r.Target
	}
	if r.Target != "" {
		return r.Target + "\n" + r.Message
	}
	return r.Message
}

// HumanNotes returns the notes a human renderer writes apart from HumanText.
func (r OpenResult) HumanNotes() []string { return r.Notes }

// openFailure is a refusal or failure on the way to a target, carrying the
// result class, reason, and message the OpenResult reports.
type openFailure struct {
	result  Result
	reason  string
	message string
}

// openIDPattern admits plain decimal ids, leading zeros included; a sign, a
// "#", or a hex prefix is refused.
var openIDPattern = regexp.MustCompile(`^[0-9]+$`)

// ParseOpenArgs parses `open <target> [id]` into the target and the change id,
// 0 when none was given. Every error is a one-line usage message.
func ParseOpenArgs(args []string) (string, int, error) {
	targets := strings.Join(OpenTargets, ", ")
	switch {
	case len(args) == 0:
		return "", 0, fmt.Errorf("missing target — choose one of %s", targets)
	case len(args) > 2:
		return "", 0, fmt.Errorf("too many arguments — usage: open <target> [id], where <target> is one of %s", targets)
	}
	what := args[0]
	if !slices.Contains(OpenTargets, what) {
		return "", 0, fmt.Errorf("unknown target %q — choose one of %s", what, targets)
	}
	if len(args) == 1 {
		return what, 0, nil
	}
	if what == "board" {
		return "", 0, fmt.Errorf("board takes no change id — the targets are %s, and only the last five take an id", targets)
	}
	raw := args[1]
	n, err := strconv.Atoi(raw)
	if !openIDPattern.MatchString(raw) || err != nil || n <= 0 {
		return "", 0, fmt.Errorf("invalid change id %s", raw)
	}
	return what, n, nil
}

// Open resolves what (and the change id, or the change the checked-out branch
// records when the id is 0) to a target and opens it. A private repository,
// open.artifacts: local, or a non-GitHub origin opens the file from the
// metadata checkout after preparing it; otherwise the GitHub blob page opens.
// A change's PR always opens its URL. With Print set nothing is launched.
func Open(ctx context.Context, deps OpenDeps, o OpenOptions) OpenResult {
	res := OpenResult{What: o.What, ChangeID: o.ID, Notes: []string{}}
	fail := func(f openFailure) OpenResult {
		res.Envelope = NewEnvelope(OperationOpen, f.result)
		res.Reason, res.Message = f.reason, f.message
		return res
	}
	if !slices.Contains(OpenTargets, o.What) || (o.What == "board" && o.ID != 0) {
		args := []string{o.What}
		if o.ID != 0 {
			args = append(args, strconv.Itoa(o.ID))
		}
		_, _, err := ParseOpenArgs(args)
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, err.Error()})
	}
	if o.RepoDir == "" {
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, "a repository directory is required"})
	}
	// The checkout probe refuses a relative path, so every seam gets an
	// absolute one.
	dir, err := filepath.Abs(o.RepoDir)
	if err != nil {
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, fmt.Sprintf("cannot resolve %s: %v", o.RepoDir, err)})
	}
	branch := ""
	if o.What != "board" && o.ID == 0 { // before any fetch
		b, f := checkoutBranch(ctx, deps.Checkout, dir)
		if f != nil {
			return fail(*f)
		}
		branch = b
	}
	pin, err := deps.Reader.PinContext(ctx, dir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return fail(openFailure{result, reason, err.Error()})
	}
	eff := pin.Config.Effective
	var rel string
	if o.What == "board" {
		if !slices.Contains(eff.BoardSurfaces.Value, boardSurfaceInline) {
			return fail(openFailure{ResultInvalidState, ReasonOpenBoardDisabled, "this repository has no board (board_surfaces has no inline surface)"})
		}
		rel = boardCorpusPath(eff)
	} else {
		c, f := resolveOpenChange(ctx, deps.Reader, pin, o.ID, branch)
		if f != nil {
			return fail(*f)
		}
		res.ChangeID = int(c.ID())
		if o.What == "pr" {
			u, f := changePRURL(c)
			if f != nil {
				return fail(*f)
			}
			res.Target, res.TargetKind = u, OpenTargetKindURL
			return finishOpen(ctx, deps, o, res)
		}
		if rel, f = changeArtifactPath(c, o.What); f != nil {
			return fail(*f)
		}
	}
	// A recorded path that escapes the repository is never opened, stat'ed,
	// or linked.
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return fail(openFailure{ResultInvalidState, ReasonOpenPathInvalid, fmt.Sprintf("%s path %q is not a path inside the repository", o.What, rel)})
	}
	if pin.Layout.Mode != layout.Private && eff.Open.Artifacts.Value != "local" {
		if u := linkContextOf(pin).BlobURL(rel); u != "" {
			res.Target, res.TargetKind = u, OpenTargetKindURL
			return finishOpen(ctx, deps, o, res)
		}
		res.Notes = append(res.Notes, openNoteNotGitHub)
	}
	path, notes, f := localArtifact(ctx, deps, dir, pin, rel)
	res.Notes = append(res.Notes, notes...)
	if f != nil {
		return fail(*f)
	}
	res.Target, res.TargetKind = path, OpenTargetKindFile
	return finishOpen(ctx, deps, o, res)
}

// localArtifact prepares the metadata checkout and returns rel's path inside
// it. A prepare that does not succeed never blocks the open: it adds a
// staleness note, and the layout's metadata worktree stands in for prepare's
// own path. A file absent from the checkout is refused.
func localArtifact(ctx context.Context, deps OpenDeps, dir string, pin StatusPin, rel string) (string, []string, *openFailure) {
	pr := deps.Prepare(ctx, dir)
	root, code := pin.Layout.MetadataWorktree, ""
	var notes []string
	if pr.Disposition == PrepareDispositionApplied || pr.Disposition == PrepareDispositionNoOp {
		if pr.Context != nil && pr.Context.MetadataWorktreePath != "" {
			root = pr.Context.MetadataWorktreePath
		}
	} else {
		code = prepareFindingCode(pr)
		notes = append(notes, openNoteStale(code))
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err != nil {
		msg := fmt.Sprintf("%s is not in the metadata checkout", path)
		if !errors.Is(err, fs.ErrNotExist) {
			msg = fmt.Sprintf("could not read %s: %v", path, err)
		}
		if code != "" {
			msg += fmt.Sprintf(" (metadata checkout not synced: %s)", code)
		}
		return "", notes, &openFailure{ResultInvalidState, ReasonOpenFileMissing, msg}
	}
	return path, notes, nil
}

// resolveOpenChange reads the pinned corpus and returns change id, or, when id
// is 0, the change whose recorded branch is branch.
func resolveOpenChange(ctx context.Context, reader StatusReader, pin StatusPin, id int, branch string) (domain.Change, *openFailure) {
	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return domain.Change{}, &openFailure{result, reason, err.Error()}
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return domain.Change{}, &openFailure{ResultInternalError, ReasonStatusInternalError, err.Error()}
	}
	if id == 0 {
		return inferChangeFromBranch(build.Snapshot, branch)
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	switch out {
	case domain.LookupFound:
		return c, nil
	case domain.LookupAmbiguous:
		return domain.Change{}, &openFailure{ResultInvalidState, ReasonOpenAmbiguousChange,
			fmt.Sprintf("more than one record claims change %d — refusing to choose", id)}
	}
	return domain.Change{}, &openFailure{ResultInvalidInput, ReasonOpenUnknownChange, fmt.Sprintf("no change %d", id)}
}

// changeArtifactPath returns the repository-relative path what names for c: the
// record itself, or its spec, plan, or results. A trivial change has no spec;
// an unset or malformed field is refused as not there yet.
func changeArtifactPath(c domain.Change, what string) (string, *openFailure) {
	var field domain.OptionalString
	switch what {
	case "change":
		return c.Path(), nil
	case "spec":
		if c.Trivial() {
			return "", &openFailure{ResultInvalidState, ReasonOpenTrivialNoSpec,
				fmt.Sprintf("change %d is trivial — it has no spec", int(c.ID()))}
		}
		field = c.Spec()
	case "plan":
		field = c.Plan()
	case "results":
		field = c.Results()
	}
	if field.State != domain.FieldPresent || strings.TrimSpace(field.Value) == "" {
		return "", &openFailure{ResultInvalidState, ReasonOpenArtifactUnset,
			fmt.Sprintf("change %d has no %s yet", int(c.ID()), what)}
	}
	return field.Value, nil
}

// changePRURL returns c's recorded pull-request URL, refusing an unset value
// and anything that is not an absolute http(s) URL.
func changePRURL(c domain.Change) (string, *openFailure) {
	pr := c.PR()
	if pr.State != domain.FieldPresent || strings.TrimSpace(pr.Value) == "" {
		return "", &openFailure{ResultInvalidState, ReasonOpenArtifactUnset, fmt.Sprintf("change %d has no PR yet", int(c.ID()))}
	}
	u, err := url.Parse(pr.Value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", &openFailure{ResultInvalidState, ReasonOpenPRNotURL,
			fmt.Sprintf("change %d pr: %s is not a URL", int(c.ID()), pr.Value)}
	}
	return pr.Value, nil
}

// prepareFindingCode names why a prepare did not succeed: its first finding's
// code, else the repository state, else the disposition.
func prepareFindingCode(pr RepositoryPrepareResult) string {
	if len(pr.Findings) > 0 && pr.Findings[0].Code != "" {
		return pr.Findings[0].Code
	}
	if pr.RepositoryState != "" {
		return pr.RepositoryState
	}
	return pr.Disposition
}

// finishOpen launches the resolved target through the opener, or, with Print,
// only reports it. On failure the target stays set so HumanText prints it
// first.
func finishOpen(ctx context.Context, deps OpenDeps, o OpenOptions, res OpenResult) OpenResult {
	if o.Print {
		res.Envelope = NewEnvelope(OperationOpen, ResultApplied)
		return res
	}
	opener := deps.Opener
	var err error
	if opener == nil {
		err = &NoOpenerError{GOOS: runtime.GOOS}
	} else {
		err = opener.Open(ctx, res.Target)
	}
	var none *NoOpenerError
	switch {
	case errors.As(err, &none):
		res.Envelope = NewEnvelope(OperationOpen, ResultExternalFailed)
		res.Reason, res.Message = ReasonOpenNoOpener, none.Error()
	case err != nil:
		res.Envelope = NewEnvelope(OperationOpen, ResultExternalFailed)
		res.Reason, res.Message = ReasonOpenOpenerFailed, fmt.Sprintf("could not open %s: %v", res.Target, err)
	default:
		res.Envelope = NewEnvelope(OperationOpen, ResultApplied)
		res.Launched = true
	}
	return res
}
