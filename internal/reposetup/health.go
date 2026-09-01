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
		return Finding{
			Code:     "metadata-worktree-dirty",
			Severity: SeverityError,
			Ref:      ".docket",
			Message:  "The .docket metadata worktree has uncommitted or unsynchronized changes.",
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
