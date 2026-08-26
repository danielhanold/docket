package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
	"github.com/danielhanold/docket/internal/install"
)

// The install operations are the composition point: internal/install owns what
// happens, internal/harness owns what it would look like, and this file joins
// them to the protocol — one result document per operation, classified from the
// service's stable reason rather than from any error text.

// InstallResult is the document `install`, `install check`, and
// `development install` all return. One shape across three operations is
// deliberate: they answer the same question about the same installation, and a
// consumer that can read one can read all three.
type InstallResult struct {
	Envelope
	Mode          string           `json:"mode"`
	Harnesses     []string         `json:"harnesses"`
	AssetProtocol int              `json:"asset_protocol"`
	AssetSetID    string           `json:"asset_set_id"`
	StatePath     string           `json:"state_path"`
	AppliedWork   bool             `json:"applied_work"`
	Actions       []install.Action `json:"actions"`
	Reason        string           `json:"reason,omitempty"`
	Message       string           `json:"message,omitempty"`
	// RepoDir and RepoHarnesses report the repository this run reconciled and the
	// opted-in harnesses whose surfaces it touched, so a scoped or default
	// repository run is visible rather than inferred from changed files (change
	// 0351). Both are empty for a machine-only run; the not-authorized action
	// already names an unreconciled repository.
	RepoDir       string   `json:"repo_dir,omitempty"`
	RepoHarnesses []string `json:"repo_harnesses,omitempty"`

	// relayed marks a development-install parent result whose candidate already
	// printed the sole document to the shared stdout. It is unexported so it
	// never enters the protocol: the presenter reads it through Relay and emits
	// nothing for it rather than a second document. relayExit is the code the
	// CLI exits with in that case.
	relayed   bool
	relayExit int
}

// Relay reports whether this is a development-install parent relay: when it is,
// the candidate subprocess has already written the one result document to the
// shared stdout, so the CLI must present nothing and exit with the returned
// code.
func (r InstallResult) Relay() (int, bool) { return r.relayExit, r.relayed }

// The three operation names. They are protocol, so they are spelled once.
const (
	OperationInstall            = "install"
	OperationInstallCheck       = "install.check"
	OperationDevelopmentInstall = "development.install"
)

// RunInstall performs a release installation.
func RunInstall(o install.Options) InstallResult {
	return withRepoReporting(NewInstallResult(OperationInstall, install.Install(o)), o.RepoPhase)
}

// RunInstallCheck reports on the installation without writing anything. Check is
// a user-level, machine-only operation, so it carries no repository reporting.
func RunInstallCheck(o install.Options) InstallResult {
	return NewInstallResult(OperationInstallCheck, install.Check(o))
}

// RunDevelopmentInstall installs from a contributor's checkout.
func RunDevelopmentInstall(o install.DevOptions) InstallResult {
	return withRepoReporting(NewInstallResult(OperationDevelopmentInstall, install.DevelopmentInstall(o)), o.RepoPhase)
}

// withRepoReporting stamps the scope-visibility fields from the reconciled
// repository phase: the selected working tree and the sorted opt-in harnesses
// whose surfaces were reconciled. An unauthorized or nil phase leaves both empty
// — the outcome's not-authorized action already names an unreconciled repository
// — so the two lines appear only when a repository was actually acted on.
func withRepoReporting(r InstallResult, phase *install.RepoPhase) InstallResult {
	if phase == nil || !phase.Authorized {
		return r
	}
	r.RepoDir = phase.Worktree
	seen := map[string]bool{}
	var harnesses []string
	for _, owners := range phase.Owners {
		for _, h := range owners {
			if !seen[h] {
				seen[h] = true
				harnesses = append(harnesses, h)
			}
		}
	}
	sort.Strings(harnesses)
	r.RepoHarnesses = harnesses
	return r
}

// NewInstallResult renders one service outcome as a protocol document. It is
// exported because the CLI layer computes a few refusals of its own — an
// unusable home directory, an asset-dependent command run against no
// installation — and those must reach the user classified by exactly the same
// table as the service's own, not by a second opinion.
func NewInstallResult(operation string, out install.Outcome) InstallResult {
	r := InstallResult{
		Envelope:      NewEnvelope(operation, classifyInstall(out)),
		Mode:          string(out.Mode),
		Harnesses:     nonNilStrings(out.Harnesses),
		AssetProtocol: out.AssetProtocol,
		AssetSetID:    out.AssetSetID,
		StatePath:     out.StatePath,
		AppliedWork:   out.Applied,
		Actions:       nonNilActions(out.Actions),
		Reason:        out.Reason,
		relayed:       out.Relayed,
		relayExit:     out.RelayExitCode,
	}
	if out.Err != nil {
		r.Message = out.Err.Error()
	}
	return r
}

// classifyInstall maps the service's stable reason to the protocol result.
//
// The default is deliberately internal-error rather than a plausible-looking
// failure: a reason no row names is a reason this layer was never taught, which
// is docket's defect and not the user's, and reporting it as invalid-state
// would send someone off to inspect a filesystem that is fine.
func classifyInstall(out install.Outcome) Result {
	switch out.Reason {
	case "":
		if out.Err != nil {
			return ResultInternalError
		}
		if out.Applied {
			return ResultApplied
		}
		return ResultNoOp

	case install.ReasonNoHarnessDetected,
		install.ReasonOwnershipConflict,
		install.ReasonManagedBlockInvalid,
		install.ReasonInstallationRequired,
		install.ReasonInstallationDrift,
		install.ReasonTransactionRecoveryRequired,
		install.ReasonAssetProtocolMismatch,
		install.ReasonSourceAssetsDrifted,
		install.ReasonStateInvalid,
		// Another docket process holds the installation lock. Nothing about
		// the user's input or their filesystem is wrong — the installation is
		// momentarily not in a state this operation may act on, and the
		// message says so — so it reads as invalid state rather than as an
		// external failure, which would send someone hunting a fault.
		install.ReasonInstallInProgress,
		// An asset bundle that does not describe itself consistently is the
		// same fact whether it was embedded in this binary or read from a
		// checkout: what is on hand cannot be installed from. It reports as
		// invalid state so the message names the bundle, rather than as an
		// internal error, which would name docket for a checkout the user can
		// regenerate.
		install.ReasonAssetManifestInvalid:
		return ResultInvalidState

	case install.ReasonDeferredCapability:
		return ResultUnsupportedConfig

	case install.ReasonUnknownHarness,
		install.ReasonInvalidOptions,
		install.ReasonInvalidSourceRoot,
		install.ReasonInvalidRepoDir,
		ReasonInvalidConfig:
		return ResultInvalidInput

	case install.ReasonBuildFailed,
		// The build succeeded but the candidate installation it handed off to
		// failed to launch or exited non-zero. It is an execution failure like
		// a build failure, not a defect in the user's input or filesystem.
		install.ReasonHandoffFailed,
		install.ReasonFilesystemFailed:
		return ResultExternalFailed

	default:
		return ResultInternalError
	}
}

// legacyNotAdoptedNote is the one aggregate sentence an ownership conflict adds
// beyond the per-target remedies the service already composed into each
// Action's Detail. An exact-match legacy Bash install — bytes byte-identical to
// what v0.9.2's sync-agents.sh wrote — is now adopted in place by ownership
// proof three (see install.LegacyReproducer), so an ownership conflict no longer
// means "a legacy install docket refuses to touch". It means the paths above are
// neither a prior docket install nor an exact legacy one, so docket cannot prove
// they are its to overwrite. The note says exactly that and defers to the
// per-path remedies; it must never tell the person to move an adoptable legacy
// tree aside, because that tree is adopted automatically now.
const legacyNotAdoptedNote = "note: an exact legacy Bash install is adopted automatically; the path(s) above " +
	"are not one, so docket changed nothing there — follow each path's remedy above, then re-run."

// HumanText renders the same facts as the JSON document, in the order a person
// reads them: what happened, to what, and then every action by path.
func (r InstallResult) HumanText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", r.Operation, r.Result)
	if r.Mode != "" {
		fmt.Fprintf(&b, "mode: %s\n", r.Mode)
	}
	if len(r.Harnesses) > 0 {
		fmt.Fprintf(&b, "harnesses: %s\n", strings.Join(r.Harnesses, ", "))
	}
	if r.AssetSetID != "" {
		fmt.Fprintf(&b, "asset set: %s (protocol %d)\n", r.AssetSetID, r.AssetProtocol)
	}
	if r.StatePath != "" {
		fmt.Fprintf(&b, "state: %s\n", r.StatePath)
	}
	if r.RepoDir != "" {
		fmt.Fprintf(&b, "repository: %s\n", r.RepoDir)
	}
	if len(r.RepoHarnesses) > 0 {
		fmt.Fprintf(&b, "repository harnesses: %s\n", strings.Join(r.RepoHarnesses, ", "))
	}
	if r.Reason != "" {
		fmt.Fprintf(&b, "reason: %s\n", r.Reason)
		if r.Message != "" {
			fmt.Fprintf(&b, "message: %s\n", r.Message)
		}
		if r.Reason == install.ReasonOwnershipConflict {
			b.WriteString(legacyNotAdoptedNote + "\n")
		}
	}
	if len(r.Actions) > 0 {
		b.WriteString("actions:\n")
		for _, a := range r.Actions {
			fmt.Fprintf(&b, "  %-8s %s", a.Op, a.Path)
			if a.Detail != "" {
				fmt.Fprintf(&b, "  (%s)", a.Detail)
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// Planners adapts every harness adapter into the closure seam the installer
// consumes, in harness.Order.
//
// The adaptation lives here rather than in either package because it is the
// only place that may know both: internal/harness imports internal/install, so
// an installer that held harness.Adapter would close an import cycle. The
// roots and the resolved agent table are closed over — they are properties of
// this invocation — while the catalog and the assets directory arrive per call,
// because a development install renders from the contributor's checkout rather
// than from the bytes this binary shipped with.
func Planners(roots install.UserRoots, agents config.AgentsTable) []install.Planner {
	adapters := []harness.Adapter{claude.New(), codex.New(), cursor.New(), opencode.New()}
	// The user-global dispatch destination each adapter USED to plan (change
	// 0351). Plan no longer emits it, but the installer still needs its location
	// and identity to retire a leftover a prior install owns. It is a
	// package-level function per adapter, not a method on Adapter, so it is
	// mapped by name here rather than reached through the interface.
	globalDispatch := map[string]func(install.UserRoots) install.Target{
		claude.Name:   claude.GlobalDispatchTarget,
		codex.Name:    codex.GlobalDispatchTarget,
		cursor.Name:   cursor.GlobalDispatchTarget,
		opencode.Name: opencode.GlobalDispatchTarget,
	}
	planners := make([]install.Planner, 0, len(adapters))
	for _, a := range adapters {
		adapter := a
		planners = append(planners, install.Planner{
			Name: adapter.Name(),
			Detect: func(r install.UserRoots) (bool, string) {
				d := adapter.Detect(r)
				return d.Present, d.Root
			},
			Plan: func(mode install.Mode, assetsDir string, catalog assets.Catalog) ([]install.Target, error) {
				return adapter.Plan(harness.PlanInput{
					Assets:    catalog,
					Mode:      planMode(mode),
					AssetsDir: assetsDir,
					Roots:     roots,
					Agents:    agents,
				})
			},
			GlobalDispatchTarget: globalDispatch[adapter.Name()],
		})
	}
	return planners
}

// planMode translates the installer's mode into the adapters'. The two
// vocabularies are spelled identically today; converting through a switch keeps
// a future divergence a compile-time question rather than a silent one.
func planMode(m install.Mode) harness.InstallMode {
	switch m {
	case install.ModeDevelopment:
		return harness.ModeDevelopment
	default:
		return harness.ModeRelease
	}
}

// agentDigestRow is the digested shape of one agent's resolved settings. Only
// the winning values are digested — never provenance — because the digest
// identifies what was RENDERED, and two configurations that pin the same model
// from different layers render the same files.
type agentDigestRow struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

// AgentDigest identifies the resolved agent settings a plan was rendered from:
// sha256 of the canonical JSON of {harness: {agent: {model, effort}}}, narrowed
// to the named harnesses. encoding/json sorts map keys, so the encoding is
// canonical without a second sort, and a harness the caller did not select
// cannot move the digest.
func AgentDigest(agents config.AgentsTable, harnesses []string) (string, error) {
	selected := make(map[string]bool, len(harnesses))
	for _, name := range harnesses {
		selected[name] = true
	}
	table := map[string]map[string]agentDigestRow{}
	for harnessName, row := range agents {
		if !selected[harnessName] {
			continue
		}
		rendered := make(map[string]agentDigestRow, len(row))
		for agentName, setting := range row {
			rendered[agentName] = agentDigestRow{
				Model:  setting.Model.Value,
				Effort: setting.Effort.Value,
			}
		}
		table[harnessName] = rendered
	}
	encoded, err := json.Marshal(table)
	if err != nil {
		return "", fmt.Errorf("app: digesting the agent table: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// HarnessNames returns the harnesses a run may install into: the caller's
// explicit selection when there is one, else every harness docket plans for.
// It is what AgentDigest is narrowed to, so the digest describes the settings
// this invocation could have rendered from.
func HarnessNames(explicit []string) []string {
	if len(explicit) == 0 {
		return append([]string(nil), harness.Order...)
	}
	names := append([]string(nil), explicit...)
	sort.Strings(names)
	return names
}

// nonNilStrings and nonNilActions keep the always-present arrays from
// marshalling as JSON null: a consumer iterating `harnesses` or `actions`
// should find an empty list, never a missing type.
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func nonNilActions(v []install.Action) []install.Action {
	if v == nil {
		return []install.Action{}
	}
	return v
}
