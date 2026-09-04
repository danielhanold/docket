// Package config owns docket's configuration vocabulary: the v0.9.2 schema,
// four-layer resolution with provenance, capability classification, and the
// mutation preflight. It is pure — no Git, no network, no writes.
package config

import "errors"

type LayerKind string

const (
	LayerBuiltIn         LayerKind = "built-in"
	LayerGlobal          LayerKind = "global"
	LayerRepository      LayerKind = "repository"
	LayerRepositoryLocal LayerKind = "repository-local"
)

// Source is one configuration layer's raw bytes. Name is the display path
// used in diagnostics ("built-in", ".docket.yml", ".docket.local.yml", or the
// global config's absolute path).
type Source struct {
	Layer LayerKind
	Name  string
	Data  []byte
}

// ResolveContext supplies facts resolution cannot derive from the layers.
type ResolveContext struct {
	DefaultBranch string // consumed only when integration_branch resolves to "auto"
	// TolerateUnknownKeys reclassifies every unknown-key ERROR diagnostic as a
	// WARNING so an unrecognized setting cannot invalidate the snapshot. It
	// exists for the install path alone: an installer must never be blocked by
	// a configuration written for a newer docket than the one running.
	// Operating commands never set it.
	TolerateUnknownKeys bool
}

type Provenance struct {
	Layer  LayerKind `json:"layer"`
	Source string    `json:"source"`
	Line   int       `json:"line,omitempty"`
	Column int       `json:"column,omitempty"`
}

// Value is one resolved leaf: the typed value, where it came from, and
// whether any honored layer set it explicitly (false = built-in default won).
type Value[T any] struct {
	Value      T          `json:"value"`
	Provenance Provenance `json:"provenance"`
	Explicit   bool       `json:"explicit"`
}

type Classification string

const (
	Supported Classification = "supported"
	Obsolete  Classification = "obsolete"
	Inert     Classification = "inert"
	Deferred  Classification = "deferred"
)

type Capability struct {
	Path           string         `json:"path"`
	Classification Classification `json:"classification"`
	Active         bool           `json:"active"`
	MutationBlock  bool           `json:"mutation_block"`
	Provenance     Provenance     `json:"provenance"`
	Reason         string         `json:"reason"`
	Remedy         string         `json:"remedy,omitempty"`
}

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Diagnostic struct {
	Code           string         `json:"code"`
	Severity       Severity       `json:"severity"`
	Path           string         `json:"path,omitempty"`
	Classification Classification `json:"classification,omitempty"`
	Provenance     *Provenance    `json:"provenance,omitempty"`
	Message        string         `json:"message"`
	Remedy         string         `json:"remedy,omitempty"`
}

// Warnings filters diagnostics to the warning-severity ones. The install path
// surfaces exactly this subset in its result document (change 0392).
func Warnings(diags []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Severity == SeverityWarning {
			out = append(out, d)
		}
	}
	return out
}

// Diagnostic codes — the closed set. Snapshot validity keys on invalidClass.
const (
	CodeInvalidYAML          = "invalid-yaml"
	CodeDuplicateKey         = "duplicate-key"
	CodeUnknownKey           = "unknown-key"
	CodeInvalidType          = "invalid-type"
	CodeInvalidValue         = "invalid-value"
	CodeFencedIgnored        = "fenced-setting-ignored"
	CodeObsoleteSetting      = "obsolete-setting"
	CodeInertSetting         = "inert-setting"
	CodeDeferredSetting      = "deferred-setting"
	CodeDeferredCapRequested = "deferred-capability-requested"
)

// Effective is the typed aggregate of SUPPORTED policy only. Inactive
// deferred/inert companion values surface through Capabilities and
// Diagnostics, never here.
type Effective struct {
	// metadata_branch is gone (change 0363): it is an obsolete tombstone, never
	// resolved policy. The metadata branch is fixed at reposetup.MetadataBranchName.
	IntegrationBranch Value[string]   `json:"integration_branch"` // auto already resolved
	ChangesDir        Value[string]   `json:"changes_dir"`
	ADRsDir           Value[string]   `json:"adrs_dir"`
	ResultsDir        Value[string]   `json:"results_dir"`
	Finalize          Finalize        `json:"finalize"`
	Build             Build           `json:"build"`
	Learnings         Learnings       `json:"learnings"`
	Reclaim           Reclaim         `json:"reclaim"`
	Review            Review          `json:"review"`
	GateObservation   Value[int]      `json:"gate_observation_budget"` // minutes
	BoardSurfaces     Value[[]string] `json:"board_surfaces"`
	Board             Board           `json:"board"`
	ChangeTypes       Value[[]string] `json:"change_types"`
	// AgentHarnesses is the repository's explicit parent-facing dispatch opt-in.
	// Write authority for repository surfaces exists iff it is Explicit AND its
	// provenance is the repository or repository-local layer; a global-layer
	// declaration resolves but is never honored by the installer. Explicit with
	// an empty list is the deliberate retire-everything state; non-explicit is
	// the touch-nothing state.
	AgentHarnesses Value[[]string] `json:"agent_harnesses"`
	Agents         AgentsTable     `json:"agents"`
}

type Finalize struct {
	Gate              Value[string] `json:"gate"`         // local|off (ci/both classify deferred-active)
	TestCommand       Value[string] `json:"test_command"` // "" == unconfigured (legacy `auto` resolves away)
	RequirePRApproval Value[bool]   `json:"require_pr_approval"`
}

// Build is the build role's OWN gate policy (change 0374). It resolves
// independently of Finalize: neither command falls back to the other.
type Build struct {
	Gate        Value[string] `json:"gate"`         // local|off
	TestCommand Value[string] `json:"test_command"` // "" == unconfigured (legacy `auto` resolves away)
}

type Learnings struct {
	Enabled Value[bool] `json:"enabled"`
}

type Reclaim struct {
	LeaseTTL Value[int]  `json:"lease_ttl"` // hours
	Auto     Value[bool] `json:"auto"`
}

type Review struct {
	MinFixSeverity Value[string] `json:"min_fix_severity"` // minor|important|blocker
	MaxFixTasks    Value[int]    `json:"max_fix_tasks"`
}

// Board is the rendered-board presentation policy: a complete section-order
// permutation and one sort per section. Sorting is keyed by
// BoardSectionTokens and always carries all six entries.
type Board struct {
	SectionOrder Value[[]string]      `json:"section_order"`
	Sorting      map[string]BoardSort `json:"sorting"`
}

type BoardSort struct {
	By        Value[string] `json:"by"`        // id | updated | created
	Direction Value[string] `json:"direction"` // asc | desc
}

// AgentsTable: harness → agent short name → resolved model/effort.
// Resolved from built-in + GLOBAL layers only; repository-layer agent
// declarations are deferred-capability requests and never land here.
type AgentsTable map[string]map[string]AgentSetting

type AgentSetting struct {
	Model  Value[string] `json:"model"`
	Effort Value[string] `json:"effort"` // "" == effort pin suppressed (`effort: auto`)
}

type Snapshot struct {
	Effective    Effective    `json:"effective"`
	Capabilities []Capability `json:"capabilities"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}

// Sentinel errors. Resolve wraps these; callers test with errors.Is.
var (
	ErrInvalidConfig            = errors.New("invalid configuration")
	ErrMissingResolutionContext = errors.New("missing resolution context: integration_branch is auto and no default branch was supplied")
	ErrUnsupportedConfig        = errors.New("unsupported configuration: active deferred or dropped capability requested")
)
