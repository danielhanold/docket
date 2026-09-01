package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
)

// assetIndependent names every command that runs without reading installed
// assets. It is an allowlist rather than a denylist on purpose: a command
// added without a decision is asset-DEPENDENT and gets refused on a machine
// with no compatible installation, which is a loud failure a test catches,
// where the reverse default would be a command silently reading a version tree
// that is absent or speaks another asset protocol.
//
// Keys are command paths with the root's name removed — "" is the bare
// `docket`. TestAssetIndependentSetExact holds the correspondence with the
// Cobra tree in both directions.
var assetIndependent = map[string]bool{
	"":                           true, // bare `docket`: reports a missing command
	"help":                       true,
	"version":                    true,
	"status":                     true,
	"change":                     true, // the group itself; it reports a missing command
	"change create":              true,
	"change groom":               true,
	"change block":               true,
	"change defer":               true,
	"change kill":                true,
	"change claim":               true,
	"change refresh-claim":       true,
	"change reconcile":           true,
	"change attach-plan":         true,
	"change attach-results":      true,
	"change halt":                true,
	"change resume-halted":       true,
	"change reclaim":             true,
	"change mark-implemented":    true,
	"change repair-identity":     true,
	"context":                    true, // the group itself; it reports a missing command
	"context implementation":     true,
	"context finalize":           true,
	"artifact":                   true, // the group itself; it reports a missing command
	"artifact backlink":          true,
	"workspace":                  true, // the group itself; it reports a missing command
	"workspace prepare":          true,
	"workspace inspect":          true,
	"workspace publish":          true,
	"evidence":                   true, // the group itself; it reports a missing command
	"evidence record":            true,
	"evidence verify":            true,
	"pr":                         true, // the group itself; it reports a missing command
	"pr publish":                 true,
	"run":                        true, // the group itself; it reports a missing command
	"run verify":                 true,
	"run gate-before":            true,
	"run gate-verdict":           true,
	"learning":                   true, // the group itself; it reports a missing command
	"learning record":            true,
	"learning update":            true,
	"adr":                        true, // the group itself; it reports a missing command
	"adr record":                 true,
	"adr supersede":              true,
	"adr reverse":                true,
	"gate":                       true, // the group itself; it reports a missing command
	"gate launch":                true,
	"gate observe":               true,
	"gate stop":                  true,
	"gate recover":               true,
	"gate cleanup":               true,
	"gate drive":                 true, // the group itself; it reports a missing command
	"gate drive start":           true,
	"gate drive advance":         true,
	"gate drive handoff":         true,
	"gate drive claim":           true,
	"finalize":                   true, // the group itself; it reports a missing command
	"finalize retarget-children": true,
	"finalize rebase":            true,
	"finalize rebase-continue":   true,
	"finalize rebase-abort":      true,
	"finalize publish":           true,
	"finalize block":             true,
	"finalize clear-block":       true,
	"finalize merge":             true,
	"finalize closeout":          true,
	"finalize cleanup":           true,
	"maintenance":                true, // the group itself; it reports a missing command
	"maintenance sweep":          true,
	"repository":                 true, // the group itself; it reports a missing command
	"repository init":            true,
	"repository check":           true,
	"repository migrate":         true,
	"repository prepare":         true,
	"repository configure-tests": true,
	"diagnostic":                 true, // the group itself; it reports a missing command
	"diagnostic runtime":         true,
	"diagnostic config":          true,
	"install":                    true,
	"install check":              true,
	"development":                true,
	"development install":        true,
	"development test":           true, // the Go-native whole-suite runner reads this checkout, never installed assets (change 0318)
}

// commandKey is a command's path with the root's own name stripped, which is
// what assetIndependent is keyed on. The root itself keys as "".
func commandKey(c *cobra.Command) string {
	return strings.TrimPrefix(strings.TrimPrefix(c.CommandPath(), c.Root().Name()), " ")
}

// operationName spells a command path the way the protocol names operations:
// `install check` is the operation `install.check`.
func operationName(key string) string {
	if key == "" {
		return "docket"
	}
	return strings.ReplaceAll(key, " ", ".")
}

// InstallRefusal is a refusal carrying the installer's stable machine reason.
// It exists so a refusal computed HERE — an unusable home directory, a guard
// that found no installation — reaches the user classified by exactly the same
// table as one computed by the service, instead of a second opinion assembled
// at the CLI boundary.
type InstallRefusal struct {
	Reason string
	Err    error
}

func (e *InstallRefusal) Error() string { return e.Err.Error() }
func (e *InstallRefusal) Unwrap() error { return e.Err }

// result renders the refusal as the named operation's document.
func (e *InstallRefusal) result(operation string) app.InstallResult {
	return app.NewInstallResult(operation, install.Outcome{Reason: e.Reason, Err: e.Err})
}

// RequireCompatibleInstallation is the asset-dependence guard's whole test: is
// there an installation on this machine, and does it speak this binary's asset
// protocol? It only reads.
func RequireCompatibleInstallation(roots install.UserRoots) error {
	state, err := install.LoadState(roots.StatePath())
	if err != nil {
		return &InstallRefusal{Reason: install.ReasonStateInvalid, Err: err}
	}
	if state == nil {
		return &InstallRefusal{Reason: install.ReasonInstallationRequired,
			Err: fmt.Errorf("%w: run `docket install` first", install.ErrNotInstalled)}
	}
	if state.AssetProtocol != assets.AssetProtocol {
		return &InstallRefusal{Reason: install.ReasonAssetProtocolMismatch,
			Err: fmt.Errorf("%w: the installation speaks asset protocol %d, this binary speaks %d; run `docket install`",
				install.ErrDrifted, state.AssetProtocol, assets.AssetProtocol)}
	}
	return nil
}

// installDefaultBranch is the resolution context these operations supply.
//
// Resolving configuration is total or it is nothing: `integration_branch: auto`
// needs a default branch to resolve against, and there is no repository here to
// read one from. Installing into a home directory never consults the setting,
// so the value only has to exist — it steers nothing, and it reaches no
// document these commands emit.
const installDefaultBranch = "main"

// installOptions assembles what the three operations read. The MACHINE half is
// strictly user-level: the home roots, this binary's embedded bundle, and the
// GLOBAL configuration layer alone — a .docket.yml in whatever directory the user
// happens to stand in must never steer what gets written into their home, and in
// particular never reaches Planners' agent table.
//
// When resolveRepo is set (install and development install, never check), it also
// grows the REPOSITORY half: the repository containing repoDir — or the current
// directory when repoDir is empty — is resolved, and its explicit agent_harnesses
// opt-in feeds ONLY the repository phase and the machine-selection union, staying
// clear of the HOME-write path above. Check never resolves a repository: it is a
// read-only, machine-only report with no --repo-dir.
func installOptions(ctx context.Context, harnesses []string, repoDir string, resolveRepo bool, info buildinfo.Info) (install.Options, *InstallRefusal) {
	roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
	if err != nil {
		return install.Options{}, &InstallRefusal{Reason: install.ReasonInvalidOptions, Err: err}
	}
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		return install.Options{}, &InstallRefusal{Reason: install.ReasonAssetManifestInvalid, Err: err}
	}
	sources, err := config.LoadGlobalSource("")
	if err != nil {
		return install.Options{}, &InstallRefusal{Reason: app.ReasonInvalidConfig, Err: err}
	}
	snapshot, _, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: installDefaultBranch})
	if err != nil {
		return install.Options{}, &InstallRefusal{Reason: app.ReasonInvalidConfig, Err: err}
	}
	agents := snapshot.Effective.Agents
	digest, err := app.AgentDigest(agents, app.HarnessNames(harnesses))
	if err != nil {
		return install.Options{}, &InstallRefusal{Reason: install.ReasonInternal, Err: err}
	}
	opts := install.Options{
		Roots:       roots,
		Planners:    app.Planners(roots, agents),
		Harnesses:   harnesses,
		Catalog:     catalog,
		Config:      snapshot,
		Info:        info,
		FS:          install.RealFS{},
		AgentDigest: digest,
	}
	if !resolveRepo {
		return opts, nil
	}

	phase, refusal := resolveRepoPhase(ctx, opts, harnesses, repoDir)
	if refusal != nil {
		return install.Options{}, refusal
	}
	opts.RepoPhase = phase
	opts.HarnessOptIns = repoOptIns(phase)
	return opts, nil
}

// resolveRepoPhase discovers and assembles the repository half of an install. The
// run-gate payload rides from the same embedded bundle the machine plan renders
// from, and the legacy reproducer is the same frozen one the machine transaction
// inspects against, so machine and repository agree on what "unchanged" means.
func resolveRepoPhase(ctx context.Context, opts install.Options, harnesses []string, repoDir string) (*install.RepoPhase, *InstallRefusal) {
	runGate, err := harness.RunGate(opts.Catalog)
	if err != nil {
		return nil, &InstallRefusal{Reason: install.ReasonAssetManifestInvalid, Err: err}
	}
	git, err := gitcli.NewClient()
	if err != nil {
		return nil, &InstallRefusal{Reason: install.ReasonInvalidOptions, Err: err}
	}
	legacy := install.LegacyReproducerFor(opts, harnessNamesForLegacy(harnesses))
	phase, _, err := app.ResolveRepoPhase(ctx, git, repoDir, harnesses, runGate, legacy)
	if err != nil {
		var re *app.RepoResolutionError
		if errors.As(err, &re) {
			return nil, &InstallRefusal{Reason: re.Reason, Err: re.Err}
		}
		return nil, &InstallRefusal{Reason: install.ReasonInternal, Err: err}
	}
	return phase, nil
}

// harnessNamesForLegacy is the harness set the repository legacy reproducer must
// cover: the explicit --harness scope when there is one, else every harness, so
// a surface owned by any harness can be proof-gated against a legacy artifact.
func harnessNamesForLegacy(explicit []string) []string {
	return app.HarnessNames(explicit)
}

// repoOptIns are the opt-in harnesses the machine-selection union widens the
// default detection with. They are the owners the assembled phase actually
// planned surfaces for; an unauthorized or nil phase contributes none.
func repoOptIns(phase *install.RepoPhase) []string {
	if phase == nil || !phase.Authorized {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, owners := range phase.Owners {
		for _, h := range owners {
			if !seen[h] {
				seen[h] = true
				out = append(out, h)
			}
		}
	}
	return out
}
