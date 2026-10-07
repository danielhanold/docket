// The shared choose-harnesses step that repository init and repository
// configure-harnesses both run, in two halves.
//
// resolveHarnessChoice decides what to write — from the --harnesses flag, an
// interactive chooser, or the caller's policy when neither is available — and
// runs before any repository write, so a cancelled chooser leaves nothing
// behind.
//
// applyHarnessChoice writes a decided selection to the repository's config
// target, then refreshes the agent surfaces through installAuthorizedSurfaces.
// That function re-reads configuration from disk rather than trusting the
// gather-time snapshot, so a key written in this run authorizes this run's
// surfaces: the predicate asks the post-write state, never the pre-write one.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
)

// HarnessChoiceRequest is what an interactive chooser is shown: every
// selectable harness in planning order, the ones to pre-check, and the config
// path the answer is written to.
type HarnessChoiceRequest struct {
	Options     []string
	Preselected []string
	ConfigPath  string
}

// HarnessChooser asks a human which harnesses get docket's dispatch
// instructions. It returns the chosen tokens (empty means none) or
// ErrHarnessChoiceCancelled when the human backs out.
type HarnessChooser func(ctx context.Context, req HarnessChoiceRequest) ([]string, error)

// ErrHarnessChoiceCancelled is the sentinel a HarnessChooser returns when the
// human cancels the choice.
var ErrHarnessChoiceCancelled = errors.New("the harness choice was cancelled")

// errHarnessesFlagRequired is configure-harnesses' refusal when it has neither a
// --harnesses value nor a chooser to ask with.
var errHarnessesFlagRequired = errors.New("no --harnesses value and no terminal to show the checklist on")

// HarnessesOptions carries the caller's harness input: whether --harnesses was
// given, its raw tokens, and the chooser to ask with (nil when the caller has no
// terminal to ask on).
type HarnessesOptions struct {
	Set     bool
	Tokens  []string
	Chooser HarnessChooser
}

// parseHarnessesFlag validates a --harnesses value before any repository read
// or write. An unset flag returns nil, nil; a valid one returns the canonical,
// non-nil selection; an invalid one returns an invalid-input refusal.
func parseHarnessesFlag(operation string, opts HarnessesOptions) ([]string, *RepositoryOpResult) {
	if !opts.Set {
		return nil, nil
	}
	sel, err := reposetup.ParseHarnessSelection(opts.Tokens)
	if err != nil {
		out := newRepositoryOpResult(operation, ResultInvalidInput, RepositoryOpResult{})
		out.human = fmt.Sprintf("%s: %s: %s", operation, ResultInvalidInput, err.Error())
		return nil, &out
	}
	return sel, nil
}

// harnessPolicy is the caller's rule for when neither a flag nor a chooser
// decides, and for whether an existing repository value is kept unasked.
type harnessPolicy int

const (
	harnessPolicyInit      harnessPolicy = iota // keep a repository value; warn when it cannot ask
	harnessPolicyConfigure                      // always ask; refuse when it cannot
)

// harnessChoiceInput is everything resolveHarnessChoice decides from.
type harnessChoiceInput struct {
	flag          []string // parsed --harnesses; nil when not passed
	chooser       HarnessChooser
	policy        harnessPolicy
	cfg           config.Effective
	repoDeclared  bool
	configDisplay string
	detect        func() []string
}

// harnessChoice is resolveHarnessChoice's answer: a decided selection to write,
// or nothing to write plus an optional warning.
type harnessChoice struct {
	decided   bool
	selection []string // non-nil when decided
	warning   string
}

// resolveHarnessChoice decides the selection before any write: the flag wins;
// init keeps a repository-declared value unasked; otherwise a chooser is asked
// with the current value (or detection) pre-checked; with no chooser, init warns
// and configure refuses with errHarnessesFlagRequired.
func resolveHarnessChoice(ctx context.Context, in harnessChoiceInput) (harnessChoice, error) {
	if in.flag != nil {
		return harnessChoice{decided: true, selection: in.flag}, nil
	}
	if in.repoDeclared && in.policy == harnessPolicyInit {
		return harnessChoice{}, nil
	}
	if in.chooser != nil {
		got, err := in.chooser(ctx, HarnessChoiceRequest{
			Options: append([]string(nil), harness.Order...), Preselected: harnessPreselection(in.cfg, in.detect), ConfigPath: in.configDisplay,
		})
		if err != nil {
			return harnessChoice{}, err
		}
		sel := []string{}
		if len(got) > 0 {
			if sel, err = reposetup.ParseHarnessSelection(got); err != nil {
				return harnessChoice{}, fmt.Errorf("the harness chooser returned an invalid selection: %w", err)
			}
		}
		return harnessChoice{decided: true, selection: sel}, nil
	}
	if in.policy == harnessPolicyInit {
		return harnessChoice{warning: fmt.Sprintf("agent_harnesses is not set, so docket wrote no agent dispatch instructions; add a line such as `%s` to %s, or run `%s`",
			reposetup.AgentHarnessesLine([]string{"claude"}), in.configDisplay, reposetup.ConfigureHarnessesCommand)}, nil
	}
	return harnessChoice{}, errHarnessesFlagRequired
}

// harnessRepoDeclared reports whether the repository or repository-local layer
// explicitly declares agent_harnesses — the same predicate that authorizes
// surface writes (Facts.SurfacesAuthorized).
func harnessRepoDeclared(cfg config.Effective) bool {
	return cfg.AgentHarnesses.Explicit && isRepositoryLayer(cfg.AgentHarnesses.Provenance.Layer)
}

// harnessPreselection is what the chooser pre-checks: the resolved value when
// any layer declares it, else what detect finds (nothing when detect is nil).
func harnessPreselection(cfg config.Effective, detect func() []string) []string {
	if cfg.AgentHarnesses.Explicit {
		return append([]string{}, cfg.AgentHarnesses.Value...)
	}
	if detect == nil {
		return []string{}
	}
	return detect()
}

// detectHarnesses names, in planning order, every harness whose user-level
// root is present in the invoking user's home. It is a hint only: an
// unresolvable home detects nothing.
func detectHarnesses() []string {
	found := []string{}
	roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
	if err != nil {
		return found
	}
	for _, p := range Planners(roots, config.AgentsTable{}) {
		if p.Detect == nil {
			continue
		}
		if present, _ := p.Detect(roots); present {
			found = append(found, p.Name)
		}
	}
	return found
}

// harnessApplied reports what applyHarnessChoice changed: the config's pending
// path (if any), whether the config was written, the surface paths pending
// review, whether any surface changed, and the warnings to show.
type harnessApplied struct {
	pendingConfig  string
	wroteConfig    bool
	surfacePending []string
	wroteSurfaces  bool
	warnings       []string
}

// harnessConfigError marks a failure writing or re-reading agent_harnesses in
// the config at display, so harnessApplyFailure can tell it from a surface
// failure.
type harnessConfigError struct {
	display string
	err     error
}

// Error names the config path and the underlying failure.
func (e *harnessConfigError) Error() string {
	return fmt.Sprintf("writing agent_harnesses to %s: %v", e.display, e.err)
}

// Unwrap exposes the underlying failure.
func (e *harnessConfigError) Unwrap() error { return e.err }

// applyHarnessChoice writes a decided selection, ALWAYS refreshes the surfaces
// (a kept value still installs them; dropped harnesses lose theirs), and warns
// when .docket.local.yml overrides the write — keyed on provenance.
func applyHarnessChoice(ctx context.Context, git *gitcli.Client, sc setupContext, choice harnessChoice) (harnessApplied, error) {
	var out harnessApplied
	_, display, _ := repoConfigTarget(sc)
	if choice.decided {
		pending, wrote, err := writeTargetConfig(sc, func(existing []byte) ([]byte, error) {
			edited, changed, rerr := reposetup.RenderAgentHarnessesEdit(existing, choice.selection)
			if rerr != nil || !changed {
				return nil, rerr
			}
			return edited, nil
		})
		if err != nil {
			return out, &harnessConfigError{display: display, err: err}
		}
		out.pendingConfig, out.wroteConfig = pending, wrote
	}
	var err error
	if out.surfacePending, out.wroteSurfaces, err = installAuthorizedSurfaces(ctx, git, sc.repo.PrimaryWorktree); err != nil {
		return out, err
	}
	if choice.warning != "" {
		out.warnings = append(out.warnings, choice.warning)
	}
	if choice.decided && sc.layout.Mode != layout.Private {
		eff, rerr := resolveSetupConfig(sc.repo.PrimaryWorktree, sc.defaultBranch)
		if rerr != nil {
			return out, &harnessConfigError{display: display, err: rerr}
		}
		if eff.AgentHarnesses.Provenance.Layer == config.LayerRepositoryLocal {
			out.warnings = append(out.warnings, fmt.Sprintf("`.docket.local.yml` also sets agent_harnesses and overrides %s in this clone; the applied value is `%s`",
				display, reposetup.FormatHarnessList(eff.AgentHarnesses.Value)))
		}
	}
	return out, nil
}

// harnessApplyFailure maps an applyHarnessChoice error to operation's result: a
// config write failure is an internal error naming the config path; anything
// else is a surface failure.
func harnessApplyFailure(operation string, state reposetup.State, err error) RepositoryOpResult {
	var ce *harnessConfigError
	if errors.As(err, &ce) {
		return repositoryInternalFailure(operation, state, "writing agent_harnesses to "+ce.display, ce.err)
	}
	return mapSurfaceFailureFor(operation, state, err)
}

// harnessChoiceCancelled is the interrupted result for a cancelled chooser;
// resolveHarnessChoice runs before any write, so nothing was written.
func harnessChoiceCancelled(operation string, state reposetup.State) RepositoryOpResult {
	out := newRepositoryOpResult(operation, ResultInterrupted, RepositoryOpResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s: the harness choice was cancelled; nothing was written", operation, ResultInterrupted)
	return out
}

// appendPending appends each path to dst, skipping "" and any path already
// present.
func appendPending(dst []string, paths ...string) []string {
	for _, p := range paths {
		if p == "" || slices.Contains(dst, p) {
			continue
		}
		dst = append(dst, p)
	}
	return dst
}

// setHarnessResult records the harness outcome on out: the selection when one
// was decided, and the warnings, each also appended to the human text.
func setHarnessResult(out *RepositoryOpResult, choice harnessChoice, applied harnessApplied) {
	if choice.decided {
		sel := append([]string{}, choice.selection...)
		out.AgentHarnesses = &sel
	}
	if len(applied.warnings) == 0 {
		return
	}
	out.Warnings = append(out.Warnings, applied.warnings...)
	human := out.HumanText()
	for _, w := range applied.warnings {
		human += "\nwarning: " + w
	}
	out.human = human
}
