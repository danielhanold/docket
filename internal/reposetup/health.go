package reposetup

// health.go — non-repairing repository health: it maps a Classification (and
// the Facts and frontmatter findings the caller already gathered) to an ordered
// list of Findings with exact, state-branched remedies, and computes the
// `docket repository check` 0/1/2 exit contract.
//
// Every remedy is branched on the same facts that produced its finding
// (learning `printed-remedy-state-validity`): the fresh remedy names
// `docket repository init`, the legacy remedy names `docket repository migrate`,
// a partial remedy names the idempotent continuation, a needs-review remedy
// lists the exact pending paths, and a conflict remedy names a human disposition
// and NEVER a destructive command. Each of these is pinned by a test in that
// exact fixture state.

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"go.yaml.in/yaml/v3"
)

// Severity is a finding's disposition. It is a string so health JSON carries it
// verbatim.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Finding is one non-repairing health diagnosis about the repository. Its JSON
// field names are the protocol-v1 snake_case form, because a Finding is
// serialized verbatim inside the `findings` array of the repository check/op
// results a JSON consumer reads.
type Finding struct {
	Code       string   `json:"code"`                 // stable machine token, e.g. "live-surface-present"
	Severity   Severity `json:"severity"`             //
	Ref        string   `json:"ref,omitempty"`        // path or ref the finding is about, if any
	Message    string   `json:"message"`              //
	Remedy     string   `json:"remedy,omitempty"`     // exact human remedy, branched on the finding's own facts
	Repairable *bool    `json:"repairable,omitempty"` // set only for frontmatter findings; nil otherwise
}

// finding categories, in the deterministic output order the contract fixes:
// remote/topology, then integration-tree, then local worktree, then surface,
// then frontmatter (appended last, from the caller's repair findings).
const (
	catRemoteTopology = iota
	catIntegrationTree
	catLocalWorktree
	catSurface
)

// categoryOf places a classifier reason token into its output category. An
// unrecognized token defaults to remote/topology so it still surfaces first
// rather than being dropped.
func categoryOf(reason string) int {
	switch reason {
	case "no-metadata-no-surface",
		"metadata-root-foreign",
		"postconditions-unmet",
		"remote-configured-unknown", "remote-default-unknown",
		"remote-integration-unknown", "metadata-presence-unknown", "live-surface-unknown":
		return catRemoteTopology
	case "legacy-live-surface",
		"pending-review-paths",
		"metadata-seeded", "metadata-seeded-live-surface":
		return catIntegrationTree
	case "docket-dir-foreign", "metadata-worktree-dirty",
		"local-metadata-diverged", "integration-pruned-attach-incomplete":
		return catLocalWorktree
	case "surfaces-drift":
		return catSurface
	default:
		return catRemoteTopology
	}
}

// EvaluateHealth maps a Classification + Facts (+ frontmatter findings the
// caller gathered) to the ordered finding list. A healthy classification yields
// no topology findings, but the caller-gathered frontmatter findings `fm` are
// still appended: a healthy topology whose metadata corpus carries a
// repairable/unsafe or malformed record surfaces those corpus findings (and
// CheckExit then returns 1 — an outstanding corpus record is a required
// action), while a healthy topology with an empty `fm` stays empty (exit 0).
// Every non-healthy state yields at least one topology finding. Output order is
// deterministic: remote/topology findings, then integration-tree findings, then
// local worktree findings, then surface findings, then frontmatter findings.
func EvaluateHealth(c Classification, f Facts, fm []RepairFinding) []Finding {
	buckets := [4][]Finding{}
	for _, reason := range c.Reasons {
		fnd := findingFor(reason, f)
		cat := categoryOf(reason)
		buckets[cat] = append(buckets[cat], fnd)
	}

	for _, fnd := range supplementalConditionFindings(c, f) {
		buckets[conditionCategory(fnd.Code)] = append(buckets[conditionCategory(fnd.Code)], fnd)
	}

	var out []Finding
	for cat := catRemoteTopology; cat <= catSurface; cat++ {
		out = append(out, buckets[cat]...)
	}
	for _, rf := range fm {
		out = append(out, frontmatterFinding(rf))
	}
	return out
}

// findingFor builds the single finding for one classifier reason token. The
// remedy is branched on the reason (and, for needs-review, on the pending
// paths), so every printed remedy is valid in exactly the state that produced
// it.
func findingFor(reason string, f Facts) Finding {
	switch reason {
	case "no-metadata-no-surface":
		return Finding{
			Code:     "repository-uninitialized",
			Severity: SeverityError,
			Message:  "Repository is not initialized: no docket metadata branch and no live surface.",
			Remedy:   "Run `docket repository init` to create the docket metadata branch and worktree.",
		}
	case "legacy-live-surface":
		return Finding{
			Code:     "legacy-repository",
			Severity: SeverityError,
			Message:  "Legacy single-branch docket layout: a live surface exists without a docket metadata branch.",
			Remedy:   "Run `docket repository migrate` to convert this repository to the docket metadata topology.",
		}
	case "pending-review-paths":
		paths := strings.Join(f.PendingReviewPaths, ", ")
		return Finding{
			Code:     "pending-review-paths",
			Severity: SeverityWarning,
			Ref:      paths,
			Message:  "Initialization staged integration paths that are not yet reviewed and committed.",
			Remedy:   "Review and commit the pending paths, then re-check: " + paths + ".",
		}
	case "metadata-seeded", "metadata-seeded-live-surface":
		return Finding{
			Code:     "migration-incomplete",
			Severity: SeverityWarning,
			Message:  "A migration seeded the metadata branch but did not finish pruning the integration surface.",
			Remedy:   "The interrupted migration is safe to resume. Re-run `docket repository migrate`; it is idempotent.",
		}
	case "integration-pruned-attach-incomplete":
		return Finding{
			Code:     "attach-incomplete",
			Severity: SeverityWarning,
			Message:  "The remote metadata and integration postconditions are met but the local worktree attach is incomplete.",
			Remedy:   "The interrupted migration is safe to resume. Re-run `docket repository migrate`; it is idempotent.",
		}
	case "metadata-root-foreign":
		return Finding{
			Code:     "metadata-root-foreign",
			Severity: SeverityError,
			Message:  "The remote docket branch is not a docket-created orphan root (foreign or non-corresponding tree).",
			Remedy:   "Inspect the remote docket branch and resolve it with a human before any repository operation; leave it unchanged.",
		}
	case "docket-dir-foreign":
		return Finding{
			Code:     "docket-dir-foreign",
			Severity: SeverityError,
			Ref:      ".docket",
			Message:  "The .docket path is a foreign directory or a conflicting worktree registration.",
			Remedy:   "Inspect the .docket path and resolve it manually with a human before any repository operation.",
		}
	case "metadata-worktree-dirty":
		msg := "The .docket metadata worktree has uncommitted or unsynchronized changes."
		switch {
		case f.DocketWorktree.Clean == PresenceAbsent && f.DocketWorktree.Synchronized == PresenceAbsent:
			msg = "The .docket metadata worktree has uncommitted changes and is not synchronized with the remote docket tip."
		case f.DocketWorktree.Clean == PresenceAbsent:
			msg = "The .docket metadata worktree has uncommitted or untracked changes."
		case f.DocketWorktree.Synchronized == PresenceAbsent:
			msg = "The .docket metadata worktree is not synchronized with the remote docket tip."
		}
		return Finding{
			Code:     "metadata-worktree-dirty",
			Severity: SeverityError,
			Ref:      ".docket",
			Message:  msg,
			Remedy:   "Commit or inspect the changes in the .docket metadata worktree before any repository operation; leave them in place.",
		}
	case "local-metadata-diverged":
		return Finding{
			Code:     "local-metadata-diverged",
			Severity: SeverityError,
			Message:  "The local docket branch has diverged from the remote docket branch.",
			Remedy:   "Reconcile the local and remote docket branches manually with a human before any repository operation.",
		}
	case "surfaces-drift":
		return Finding{
			Code:     "surfaces-drift",
			Severity: SeverityError,
			Message:  "The declared parent-facing surfaces disagree with the seed plan and ownership record.",
			Remedy:   "Re-check the agent_harnesses declaration and reconcile the parent-facing surfaces manually.",
		}
	case "postconditions-unmet":
		return Finding{
			Code:     "postconditions-unmet",
			Severity: SeverityError,
			Message:  "The metadata branch exists but not every health postcondition is satisfied.",
			Remedy:   "Resolve the reported issues, then re-run `docket repository check`.",
		}
	case "remote-configured-unknown", "remote-default-unknown",
		"remote-integration-unknown", "metadata-presence-unknown", "live-surface-unknown":
		return Finding{
			Code:     reason,
			Severity: SeverityWarning,
			Message:  "A required repository probe could not be resolved: " + reason + ".",
			Remedy:   "Ensure the remote is configured and reachable, then re-run `docket repository check`.",
		}
	default:
		return Finding{
			Code:     reason,
			Severity: SeverityWarning,
			Message:  "Unclassified repository condition: " + reason + ".",
			Remedy:   "Re-run `docket repository check` after investigating the repository state.",
		}
	}
}

// TestConfigMissingCode is the stable machine token for the test-policy health
// finding: a local test gate cannot run because no command is configured (or a
// legacy `auto` is still declared). Its remedy names `docket repository
// configure-tests` — the setup-time upgrade path that generates the pending edit.
const TestConfigMissingCode = "test-config-missing"

// TestConfigFinding reports the test-policy configuration gap, or nil when the
// resolved test policy is complete. It is CLOSED and keys on EACH local gate
// independently: it fires when the resolved BUILD gate is `local` with an empty
// command, OR the resolved FINALIZE gate is `local` with an empty command, OR
// the committed repository bytes still declare the legacy `auto` spelling under
// either `build.test_command` or `finalize.test_command`. The two local-gate
// disjuncts are independent — a configured finalize command never masks an
// unconfigured build one — so the build-side and finalize-side asserts each
// redden their own mutation. committedYML may be nil (no file) or malformed (an
// unparseable file is tolerated: the finding then rests on the resolved-config
// disjuncts alone, never a panic).
func TestConfigFinding(cfg config.Effective, committedYML []byte) *Finding {
	fires := localGateNeedsCommand(cfg.Build.Gate.Value, cfg.Build.TestCommand.Value) ||
		localGateNeedsCommand(cfg.Finalize.Gate.Value, cfg.Finalize.TestCommand.Value) ||
		committedDeclaresLegacyAuto(committedYML)
	if !fires {
		return nil
	}
	return &Finding{
		Code:     TestConfigMissingCode,
		Severity: SeverityWarning,
		Message:  "A local test gate has no configured command (or a legacy `auto` spelling is still declared); the gate cannot run until a command is set.",
		Remedy:   "Run `docket repository configure-tests` to generate the pending test-policy edit, then review and commit it.",
	}
}

// localGateNeedsCommand reports whether a gate owner is a local gate with no
// resolved command — the configuration gap a setup-time edit must close.
func localGateNeedsCommand(gate, command string) bool {
	return gate == "local" && command == ""
}

// ConfigureTestsGapNote names the per-gate configuration gap that
// `docket repository configure-tests` cannot mechanically close, so its no-op
// path can name the gate instead of a bare "already configured; nothing to
// write". It shares localGateNeedsCommand with TestConfigFinding — the health
// finding fires per-gate, but DiscoverTests short-circuits to "configured" as
// soon as EITHER build.test_command or finalize.test_command is set, and
// RenderTestConfigEdit then writes nothing. So a repo with one gate configured
// and the other `gate: local` with an empty command is a dead end: configure-tests
// reports "nothing to write" while `docket repository check` keeps flagging the
// gap. configure-tests cannot fill it automatically — re-probing would clobber
// the already-set command, and copying the other gate's command conflates two
// independent settings — so it names the specific gate(s) and the by-hand
// completion. It returns "" when no local gate is missing a command (the
// fully-configured and gate-off cases are unchanged).
func ConfigureTestsGapNote(cfg config.Effective) string {
	var owners, keys []string
	if localGateNeedsCommand(cfg.Build.Gate.Value, cfg.Build.TestCommand.Value) {
		owners = append(owners, "build")
		keys = append(keys, "`build.test_command`")
	}
	if localGateNeedsCommand(cfg.Finalize.Gate.Value, cfg.Finalize.TestCommand.Value) {
		owners = append(owners, "finalize")
		keys = append(keys, "`finalize.test_command`")
	}
	if len(owners) == 0 {
		return ""
	}
	return fmt.Sprintf("the %s gate is `local` with no command and discovery left it unset (the pair reads as configured because the other gate already has a command); set %s in .docket.yml by hand, then re-run `docket repository check`.",
		strings.Join(owners, " and "), strings.Join(keys, " and "))
}

// committedDeclaresLegacyAuto reports whether the committed repository-layer
// bytes still spell the legacy `auto` sentinel under build.test_command or
// finalize.test_command. Malformed YAML is not an error here (the resolved
// config already carries the authoritative decision); it simply reports false.
func committedDeclaresLegacyAuto(committedYML []byte) bool {
	if len(committedYML) == 0 {
		return false
	}
	var doc struct {
		Build struct {
			TestCommand string `yaml:"test_command"`
		} `yaml:"build"`
		Finalize struct {
			TestCommand string `yaml:"test_command"`
		} `yaml:"finalize"`
	}
	if err := yaml.Unmarshal(committedYML, &doc); err != nil {
		return false
	}
	return doc.Build.TestCommand == "auto" || doc.Finalize.TestCommand == "auto"
}

// frontmatterFinding lifts one caller-gathered RepairFinding into a health
// Finding. It is the only Finding that carries a non-nil Repairable pointer.
func frontmatterFinding(rf RepairFinding) Finding {
	repairable := rf.Repairable
	if rf.Repairable {
		return Finding{
			Code:       string(rf.Code),
			Severity:   SeverityWarning,
			Ref:        rf.Path,
			Message:    rf.Message,
			Remedy:     "Apply the previewed mechanical repair, or edit the record frontmatter manually.",
			Repairable: &repairable,
		}
	}
	return Finding{
		Code:       "frontmatter-manual-review",
		Severity:   SeverityError,
		Ref:        rf.Path,
		Message:    rf.Message,
		Remedy:     "Edit the record frontmatter manually; this shape is outside the mechanical repair roster.",
		Repairable: &repairable,
	}
}

// CheckExit maps a classification and its findings to the `docket repository
// check` exit contract: healthy → 0, unknown → 2, everything else → 1. The
// exit encodes a non-failure — a 1 means diagnosed action is required, not a
// crash, and JSON consumers read `findings`, never the code (learning
// `exit-code-encodes-a-non-failure`). Invalid CLI usage is mapped to 2 by the
// command layer, not here. As a defensive guard, a healthy classification that
// nonetheless carries findings is never reported clean.
func CheckExit(c Classification, findings []Finding) int {
	switch c.State {
	case StateHealthy:
		if len(findings) > 0 {
			return 1
		}
		return 0
	case StateUnknown:
		return 2
	default:
		return 1
	}
}

// reasonExplains maps each classifier reason token to the health conditions
// it already explains, so a supplemental finding is suppressed exactly when
// an existing finding covers its condition — dedupe is by the condition
// explained, never by state or category. Deliberately NOT listed:
// pending-review-paths does not explain committed-ignore-valid (naming
// .gitignore as pending does not explain its committed defect).
var reasonExplains = map[string][]HealthCondition{
	"metadata-root-foreign":                {CondMetadataRootVerified},
	"docket-dir-foreign":                   {CondWorktreeNotForeign, CondWorktreeRegistered, CondWorktreeClean, CondWorktreeSynchronized, CondWorktreeHooksOff},
	"metadata-worktree-dirty":              {CondWorktreeClean, CondWorktreeSynchronized},
	"local-metadata-diverged":              {CondWorktreeSynchronized},
	"pending-review-paths":                 {CondNoPendingReviewPaths},
	"metadata-seeded":                      {CondLiveSurfaceAbsent},
	"metadata-seeded-live-surface":         {CondLiveSurfaceAbsent},
	"integration-pruned-attach-incomplete": {CondLocalMetadataPresent, CondWorktreePresent, CondWorktreeRegistered},
	"surfaces-drift":                       {CondSurfacesAgree},
}

// supplementalConditionFindings adds one finding per applicable unmet health
// condition an existing reason-based finding does not already explain. It
// runs only when the remote metadata branch is proven present (the same
// boundary RunRepositoryCheck augments behind), so fresh/legacy repositories
// and gather failures keep their existing reports. It never changes the
// classification — it explains it.
func supplementalConditionFindings(c Classification, f Facts) []Finding {
	if f.RemoteMetadata.Presence != PresencePresent {
		return nil
	}
	unmet := UnmetHealthConditions(f)
	if len(unmet) == 0 {
		return nil
	}
	explained := map[HealthCondition]bool{}
	for _, r := range c.Reasons {
		for _, cond := range reasonExplains[r] {
			explained[cond] = true
		}
	}
	var out []Finding
	for _, cond := range unmet {
		if explained[cond] {
			continue
		}
		if fnd := conditionFinding(cond, f); fnd != nil {
			out = append(out, *fnd)
		}
	}
	return out
}

// integrationResolved reports whether the pinned integration commit was
// resolvable — the prerequisite of every committed-tree probe. When it is
// false those probes never ran, and the unknown-authority diagnostics
// already explain the gap.
func integrationResolved(f Facts) bool {
	return f.RemoteIntegration.Presence == PresencePresent && f.RemoteIntegration.Tip != ""
}

// worktreeInspectable reports whether the .docket worktree's dependent facts
// (registration, cleanliness, synchronization, hooks) were meaningfully
// probed: a missing or foreign path is itself the blocking observation.
func worktreeInspectable(f Facts) bool {
	return f.DocketWorktree.Presence == PresencePresent && !f.DocketWorktree.Foreign
}

// conditionFinding builds the supplemental finding for one unmet condition,
// or nil when a missing prerequisite's own finding already represents it
// (explanatory grouping, never permission to drop an unexplained failure).
// Unknown evidence yields an "unverified" warning; a proven wrong state
// yields an error. Every remedy fits the observed state and preserves local
// work (learning printed-remedy-state-validity).
func conditionFinding(cond HealthCondition, f Facts) *Finding {
	switch cond {
	case CondMetadataBranchPresent:
		return nil // the supplemental gate requires it proven present
	case CondMetadataRootVerified:
		// RootForeign is explained by the metadata-root-foreign reason; here
		// only RootUnknown remains: unresolved, never foreign.
		if f.MetadataRoot != RootUnknown {
			return nil
		}
		return &Finding{
			Code:     "metadata-ownership-unverified",
			Severity: SeverityWarning,
			Message:  "The remote docket branch's ownership proof could not be resolved (unverified, not proven wrong).",
			Remedy:   "Ensure the remote is reachable and its objects fetchable, then re-run `docket repository check`.",
		}
	case CondLocalMetadataPresent:
		if f.LocalMetadata.Presence == PresenceAbsent {
			return &Finding{
				Code:     "local-metadata-missing",
				Severity: SeverityError,
				Message:  "No local docket branch exists for the present remote docket branch.",
				Remedy:   "Run `docket repository migrate` to restore the local metadata attachment; it is idempotent.",
			}
		}
		return &Finding{
			Code:     "local-metadata-unverified",
			Severity: SeverityWarning,
			Message:  "The local docket branch could not be resolved (unverified, not proven absent).",
			Remedy:   "Re-run `docket repository check` once local Git reads succeed.",
		}
	case CondWorktreePresent:
		if f.DocketWorktree.Presence == PresenceAbsent {
			return &Finding{
				Code:     "docket-worktree-missing",
				Severity: SeverityError,
				Ref:      ".docket",
				Message:  "The .docket metadata worktree is missing; its registration, cleanliness, and hooks state cannot be established until it exists.",
				Remedy:   "Run `docket repository migrate` to restore the .docket worktree attachment; it is idempotent.",
			}
		}
		return &Finding{
			Code:     "docket-worktree-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket path could not be inspected (unverified, not proven absent); its dependent state cannot be established.",
			Remedy:   "Restore read access to the .docket path, then re-run `docket repository check`.",
		}
	case CondWorktreeNotForeign:
		return nil // always explained by the docket-dir-foreign conflict reason
	case CondWorktreeRegistered:
		if !worktreeInspectable(f) {
			return nil // the worktree-present/foreign finding is the blocking observation
		}
		if f.DocketWorktree.Registered == PresenceAbsent {
			return &Finding{
				Code:     "docket-worktree-unregistered",
				Severity: SeverityError,
				Ref:      ".docket",
				Message:  "The .docket path exists but is not a registered worktree of this repository.",
				Remedy:   "Inspect the .docket path and resolve its registration manually with a human; leave its contents in place.",
			}
		}
		return &Finding{
			Code:     "docket-worktree-registration-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket worktree registration could not be resolved (unverified, not proven foreign).",
			Remedy:   "Re-run `docket repository check` once `git worktree list` succeeds.",
		}
	case CondWorktreeClean:
		if !worktreeInspectable(f) {
			return nil
		}
		// Absent is explained by the metadata-worktree-dirty reason when the
		// classifier selected it; in states where it did not (it always does
		// for a present worktree), only Unknown remains applicable.
		if f.DocketWorktree.Clean != PresenceUnknown {
			return nil
		}
		return &Finding{
			Code:     "docket-worktree-clean-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket worktree's cleanliness could not be resolved (unverified, not proven dirty).",
			Remedy:   "Re-run `docket repository check` once the .docket status read succeeds.",
		}
	case CondWorktreeSynchronized:
		if !worktreeInspectable(f) {
			return nil
		}
		// Synchronization needs both branch tips; missing prerequisites are
		// represented by the local-metadata finding. Absent is explained by
		// the metadata-worktree-dirty reason.
		return nil
	case CondWorktreeHooksOff:
		if !worktreeInspectable(f) {
			return nil
		}
		if f.DocketWorktree.HooksOff == PresenceAbsent {
			return &Finding{
				Code:     "docket-worktree-hooks-enabled",
				Severity: SeverityError,
				Ref:      ".docket",
				Message:  "Git hooks are not disabled on the .docket metadata worktree (per-worktree core.hooksPath is not set to an existing directory).",
				Remedy:   "Point the .docket worktree's per-worktree core.hooksPath at an existing empty directory, as init leaves it, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "docket-worktree-hooks-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "The .docket worktree's hooks configuration could not be resolved (unverified, not proven enabled).",
			Remedy:   "Re-run `docket repository check` once the .docket config read succeeds.",
		}
	case CondCommittedIgnoreValid:
		if !integrationResolved(f) {
			return nil // the unknown-authority diagnostic explains the skipped read
		}
		if f.CommittedIgnoreBlock == PresenceUnknown {
			return &Finding{
				Code:     "committed-ignore-unverified",
				Severity: SeverityWarning,
				Ref:      ".gitignore",
				Message:  "The committed .gitignore blob could not be read, so the managed ignore block cannot be verified (unverified, not proven absent).",
				Remedy:   "Restore readable committed evidence (fetch the integration objects), then re-run `docket repository check`.",
			}
		}
		return committedIgnoreFinding(f.CommittedIgnoreDetail)
	case CondLiveSurfaceAbsent:
		if f.LiveSurface == PresencePresent {
			return &Finding{
				Code:     "live-surface-present",
				Severity: SeverityError,
				Message:  "The integration tree still carries a live docket surface alongside the metadata branch.",
				Remedy:   "Run `docket repository migrate` to finish pruning the integration surface; it is idempotent.",
			}
		}
		return &Finding{
			Code:     "live-surface-unverified",
			Severity: SeverityWarning,
			Message:  "The integration tree's live-surface state could not be resolved (unverified).",
			Remedy:   "Ensure the remote is reachable, then re-run `docket repository check`.",
		}
	case CondLegacyConfigKeyAbsent:
		if !integrationResolved(f) {
			return nil
		}
		if f.LegacyConfigKey == PresencePresent {
			return &Finding{
				Code:     "legacy-config-key-present",
				Severity: SeverityError,
				Ref:      ".docket.yml",
				Message:  "The committed .docket.yml still declares the legacy top-level metadata_branch key.",
				Remedy:   "Remove the metadata_branch key from .docket.yml, commit, and push the integration branch, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "legacy-config-key-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket.yml",
			Message:  "The committed .docket.yml could not be read, so the legacy metadata_branch key cannot be verified (unverified, not proven present).",
			Remedy:   "Restore readable committed evidence, then re-run `docket repository check`.",
		}
	case CondPrimaryClean:
		if f.PrimaryClean == PresenceAbsent {
			return &Finding{
				Code:     "primary-worktree-dirty",
				Severity: SeverityError,
				Message:  "The primary worktree has uncommitted changes.",
				Remedy:   "Review and commit (or deliberately restore) the changes yourself, preserving local work, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "primary-clean-unverified",
			Severity: SeverityWarning,
			Message:  "The primary worktree's cleanliness could not be resolved (unverified, not proven dirty).",
			Remedy:   "Re-run `docket repository check` once the primary status read succeeds.",
		}
	case CondPrimaryOnIntegration:
		if f.PrimaryOnIntegration == PresenceAbsent {
			return &Finding{
				Code:     "primary-not-on-integration",
				Severity: SeverityError,
				Message:  "The primary worktree is not checked out on the integration branch.",
				Remedy:   "Switch the primary worktree to the integration branch without discarding local work, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "primary-branch-unverified",
			Severity: SeverityWarning,
			Message:  "The primary worktree's branch could not be resolved (unverified).",
			Remedy:   "Re-run `docket repository check` once `git worktree list` succeeds.",
		}
	case CondPrimaryAtRemoteTip:
		if !integrationResolved(f) {
			return nil
		}
		if f.PrimaryAtRemoteTip == PresenceAbsent {
			return &Finding{
				Code:     "primary-behind-remote-tip",
				Severity: SeverityError,
				Message:  "The primary worktree's HEAD does not equal the pinned remote integration tip.",
				Remedy:   "Fetch and fast-forward the integration branch (e.g. `docket repository sync-integration`), preserving local work, then re-run `docket repository check`.",
			}
		}
		return &Finding{
			Code:     "primary-tip-unverified",
			Severity: SeverityWarning,
			Message:  "The primary worktree's position against the remote integration tip could not be resolved (unverified).",
			Remedy:   "Re-run `docket repository check` once the local HEAD read succeeds.",
		}
	case CondSurfacesAgree:
		// Only reached when SurfacesAuthorized (the conjunct is otherwise
		// satisfied); Absent is explained by the surfaces-drift reason.
		if f.SurfacesAgree != PresenceUnknown {
			return nil
		}
		return &Finding{
			Code:     "surfaces-unverified",
			Severity: SeverityWarning,
			Message:  "The authorized parent-facing surface agreement could not be resolved (unverified, not proven drifted).",
			Remedy:   "Re-run `docket repository check` once the surface probe succeeds.",
		}
	case CondNoPendingReviewPaths:
		// Applicable when pending paths exist but the classifier did not
		// select needs-review (e.g. an unverified root shape): reuse the
		// existing finding so the explanation cannot drift.
		fnd := findingFor("pending-review-paths", f)
		return &fnd
	}
	return nil
}

// committedIgnoreFinding renders the preserved ignore detail into the one
// committed-ignore defect finding. The defect is explicitly located in the
// committed integration tree: an uncommitted local fix does not establish
// the guarantee (learning gitignore-guarantee-must-be-committed).
func committedIgnoreFinding(d IgnoreDetail) *Finding {
	fnd := &Finding{
		Code:     "committed-ignore-invalid",
		Severity: SeverityError,
		Ref:      ".gitignore",
	}
	switch d.Defect {
	case IgnoreDefectFileAbsent:
		fnd.Message = "The committed integration tree has no .gitignore file, so the managed docket ignore block is absent."
		fnd.Remedy = "Restore the managed block (e.g. re-run `docket repository migrate`, or add it by hand from the canonical block), review, commit, and push the corrected .gitignore."
	case IgnoreDefectBlockAbsent:
		fnd.Message = "The committed .gitignore does not contain the managed docket ignore block."
		fnd.Remedy = "Restore the managed block, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectLegacyOnly:
		fnd.Message = "The committed .gitignore carries only the legacy managed-block markers; the current-generation block is absent."
		fnd.Remedy = "Upgrade the managed block to the current markers, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectMalformedMarkers:
		fnd.Message = "The committed .gitignore's managed-block markers are malformed (" + d.Generation + " generation): dangling, out-of-order, or nested start/end."
		fnd.Remedy = "Inspect and correct the reported marker structure by hand first, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectMissingEntries:
		fnd.Message = "The committed .gitignore's managed docket block is missing required entries: " + strings.Join(d.MissingEntries, ", ") + "."
		fnd.Remedy = "Restore the missing entries (" + strings.Join(d.MissingEntries, ", ") + ") to the managed block, then review, commit, and push the corrected .gitignore."
	case IgnoreDefectNonCanonical:
		fnd.Message = "The committed .gitignore's managed docket block contains all required entries but differs from the canonical block representation (reordered, extra, or differently-terminated lines)."
		fnd.Remedy = "Rewrite the managed block to the canonical representation, then review, commit, and push the corrected .gitignore."
	default:
		// Absent presence with no preserved detail (an older caller):
		// still a concrete committed-block failure, without invented detail.
		fnd.Message = "The committed .gitignore's managed docket block failed validation in the committed integration tree."
		fnd.Remedy = "Restore the canonical managed block, then review, commit, and push the corrected .gitignore."
	}
	return fnd
}

// conditionCategory places a supplemental finding code into the existing
// deterministic output buckets (same order categoryOf fixes for reasons).
func conditionCategory(code string) int {
	switch code {
	case "metadata-ownership-unverified", "local-metadata-missing", "local-metadata-unverified":
		return catRemoteTopology
	case "committed-ignore-invalid", "committed-ignore-unverified",
		"legacy-config-key-present", "legacy-config-key-unverified",
		"live-surface-present", "live-surface-unverified", "pending-review-paths":
		return catIntegrationTree
	case "surfaces-unverified":
		return catSurface
	default: // every docket-worktree-* and primary-* code
		return catLocalWorktree
	}
}
