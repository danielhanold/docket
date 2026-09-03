package app

// OperationStatus is the operation name every status protocol document carries
// in its envelope. Later tasks reference this constant rather than the literal.
const OperationStatus = "status"

// StatusContext reports the exact authoritative state the whole read was pinned
// against: the branch names with their full object ids. It carries revisions and
// branch names — the stable identity the spec's host-path rule permits — and
// never a host-absolute path. Go v1 supports one metadata topology (the fixed
// orphan `docket` branch), so no mode-shaped field appears here (change 0363);
// the metadata identity is the revision alone.
type StatusContext struct {
	DefaultBranch         string `json:"default_branch"`          // e.g. "main"
	DefaultBranchRevision string `json:"default_branch_revision"` // full object id
	IntegrationBranch     string `json:"integration_branch"`
	IntegrationRevision   string `json:"integration_revision"`
	MetadataRevision      string `json:"metadata_revision,omitempty"`
}

// StatusSummary carries the counts a reader would otherwise recompute from the
// body. Every count is derived from the assembled arrays so the summary and the
// body can never disagree: displayed reflects the --type/--priority projection,
// while total/active and the finding tallies always cover the complete repo.
type StatusSummary struct {
	TotalChanges     int `json:"total_changes"`     // complete corpus: active + archived
	ActiveChanges    int `json:"active_changes"`    // complete active set
	DisplayedChanges int `json:"displayed_changes"` // after --type/--priority projection
	ReadyChanges     int `json:"ready_changes"`     // len of Ready
	ADRs             int `json:"adrs"`
	Learnings        int `json:"learnings"`
	ErrorFindings    int `json:"error_findings"`
	WarningFindings  int `json:"warning_findings"`
}

// StatusChange is one displayed change: its stored record fields verbatim, plus
// the readiness, dependency, and stack facts the domain layer derived. Status,
// Priority, and Type carry the stored spelling; ReadinessWhy is explanatory
// prose, not a parseable code.
type StatusChange struct {
	ID            int    `json:"id"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Status        string `json:"status"`   // stored spelling
	Priority      string `json:"priority"` // stored spelling
	Type          string `json:"type"`
	Location      string `json:"location"`         // "active" | "archive"
	Path          string `json:"path"`             // repo-relative record path
	Version       string `json:"version"`          // blob object id
	Readiness     string `json:"readiness"`        // domain Readiness kind's named string
	ReadinessWhy  string `json:"readiness_reason"` // explanatory, not parseable
	UnmetDeps     []int  `json:"unmet_dependencies"`
	StackParent   int    `json:"stack_parent,omitempty"`
	EffectiveBase string `json:"effective_base,omitempty"` // resolved base branch name
	Ready         bool   `json:"ready"`                    // member of the ordered ready queue
}

// StatusRecord is one artifact-integrity row over the complete corpus: its
// kind, identity, named domain location, repo-relative path, and blob id. It is
// the corpus inventory the findings reference, independent of the projection.
type StatusRecord struct {
	Kind     string `json:"kind"`     // "change" | "adr" | "learning"
	Identity string `json:"identity"` // "0310", adr id, or learning slug
	Location string `json:"location"` // named domain location string
	Path     string `json:"path"`
	Version  string `json:"version"` // blob object id
}

// StatusFinding is one normalized health finding. It carries the DTO's own
// finding shape — a client of domain.Finding but not that type — so the
// protocol contract is stable independent of the domain's internal spelling.
// Message is explanatory prose; Remedy, when present, must be valid for the
// exact reported state.
type StatusFinding struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"` // "error" | "warning" | "notice"
	Entity   string   `json:"entity_kind,omitempty"`
	Identity string   `json:"entity_identity,omitempty"`
	Field    string   `json:"field,omitempty"`
	Path     string   `json:"path,omitempty"`
	Related  []string `json:"related,omitempty"`
	Message  string   `json:"message"`          // explanatory prose
	Remedy   string   `json:"remedy,omitempty"` // must be valid for the exact reported state
}

// StatusResult is the diagnostic.status / status protocol document both
// presenters consume. It embeds the envelope; Reason and Message are populated
// on failure results only, and never carry a partial report — a failure emits
// exactly one document with empty report arrays.
type StatusResult struct {
	Envelope
	Context StatusContext  `json:"context"`
	Summary StatusSummary  `json:"summary"`
	Changes []StatusChange `json:"changes"`
	Ready   []int          `json:"ready"`
	// Records is the artifact-integrity inventory over the complete corpus.
	// It is computed and marshaled only when StatusOptions.IncludeRecords asks
	// for it (the --records flag): nil means "not requested" and the key is
	// absent; a non-nil empty slice still marshals as []. Absence is the
	// signal — never an empty array (change 0397).
	Records  *[]StatusRecord `json:"records,omitempty"`
	Findings []StatusFinding `json:"findings"`
	// RepositoryState is populated only on the operational gate's typed refusal
	// (change 0363): the classifier's stable repository state (e.g. "legacy"),
	// the same value `repository check` reports for the same facts.
	RepositoryState string `json:"repository_state,omitempty" docket:"refusal-only"`
	Reason          string `json:"reason,omitempty" docket:"refusal-only"` // failure results only
	Message         string `json:"message,omitempty" docket:"refusal-only"`
}

// NewStatusResult stamps the envelope and normalizes nil collections to empty
// slices so the three always-present arrays marshal as [] on every path,
// including failures. Records is not among them (change 0397): it is an opt-in
// pointer left nil unless StatusOptions.IncludeRecords requested it, so the key
// is absent by default and a non-nil empty slice still marshals as [].
func NewStatusResult(result Result, r StatusResult) StatusResult {
	r.Envelope = NewEnvelope(OperationStatus, result)
	if r.Changes == nil {
		r.Changes = []StatusChange{}
	}
	if r.Ready == nil {
		r.Ready = []int{}
	}
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}
