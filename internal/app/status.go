package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// Stable machine reasons for the status operation's failure results. Message is
// explanatory prose and must not be parsed.
const (
	ReasonStatusInvalidInput  = "invalid-input"
	ReasonStatusExternal      = "external-failed"
	ReasonStatusInterrupted   = "interrupted"
	ReasonStatusInternalError = "internal-error"
)

// The two source names the operation asks an artifact against. Specs live on
// the metadata source; plans and results live on the integration source. The
// Git-backed reader resolves the metadata source to the fixed `docket` revision
// and the integration source to the integration revision — two distinct pinned
// revisions — which is what keeps a spec read pinned to the metadata revision
// (change 0363: Go v1 supports one metadata topology).
const (
	sourceMetadata    = "metadata"
	sourceIntegration = "integration"
)

// ErrStatusInvalidInput and ErrStatusExternal are the sentinel classifications
// the reader wraps its failures in, so Status can map a failure to its protocol
// result without reading message text. Anything the reader returns that is
// neither sentinel — and is not a context cancellation — is a contract
// violation reported as internal-error.
var (
	ErrStatusInvalidInput = errors.New("status: invalid input")
	ErrStatusExternal     = errors.New("status: external failure")
)

// StatusOptions are the operation's arguments, already CLI-validated for
// syntax. The closed-value checks (priority spellings, configured change types)
// happen here, against the resolved configuration, because only it knows the
// closed type set.
type StatusOptions struct {
	RepoDir    string   // invocation directory; "" = cwd
	Types      []string // repeatable --type values (validated against configured change_types)
	Priorities []string // repeatable --priority values (closed domain priority spellings)
	// IncludeRecords opts in to the corpus artifact-integrity inventory
	// (the records array). Off, the inventory is neither computed nor
	// marshaled (change 0397: the 130 KB majority of the payload that no
	// preflight, selection, or human read uses).
	IncludeRecords bool
}

// StatusPin is everything the read was pinned against, resolved once by
// PinContext and threaded verbatim into every later reader call so the whole
// read observes one authoritative revision of each branch.
type StatusPin struct {
	DefaultBranch       string
	DefaultRevision     string
	IntegrationBranch   string
	IntegrationRevision string
	MetadataRevision    string
	// RepoWebURL is the https web base derived from origin's configured URL
	// ("https://github.com/owner/repo"), "" for non-GitHub/unreadable remotes.
	RepoWebURL  string
	Config      config.Snapshot
	ConfigDiags []config.Diagnostic
}

// StatusBlob is one record read from a pinned source: its declared kind and
// location, its repository-relative path, the blob object id, and the raw
// bytes the document layer parses.
type StatusBlob struct {
	Kind     repository.RecordKind
	Location repository.RecordLocation
	Path     string // repo-relative
	Version  string // blob object id
	Data     []byte
}

// StatusArtifact is one artifact read from a pinned source: its blob object id
// and raw bytes, or Found=false when the path is absent at the pinned revision.
// It is the byte-returning companion to ArtifactExists, which reports only
// presence.
type StatusArtifact struct {
	Found   bool
	Version string // blob object id
	Data    []byte
}

// StatusReader is the seam between orchestration and Git. One call per concern;
// the Git-backed implementation arrives in Task 3, and application tests drive
// a fake.
type StatusReader interface {
	// PinContext discovers the repo, resolves origin's default branch, reads
	// .docket.yml from the pinned default-branch source, resolves configuration,
	// fetches and pins the branches the resolved metadata mode requires, and
	// returns everything pinned once. Errors: ErrStatusInvalidInput /
	// ErrStatusExternal wrapped.
	PinContext(ctx context.Context, repoDir string) (StatusPin, error)
	// ReadCorpus lists and reads every configured record (active + archived
	// changes, ADRs, learnings when enabled) from the pinned metadata source.
	ReadCorpus(ctx context.Context, pin StatusPin) ([]StatusBlob, error)
	// BranchFacts fetches/resolves only the distinct feature branches named by
	// live stack relationships and reports which exist on the remote.
	BranchFacts(ctx context.Context, pin StatusPin, branches []string) (domain.BranchFacts, error)
	// ArtifactExists reports whether a repo-relative path exists on the named
	// pinned source ("metadata" for specs, "integration" for plans/results).
	ArtifactExists(ctx context.Context, pin StatusPin, source, path string) (bool, error)
	// ReadArtifact reads a repo-relative path from the named pinned source,
	// returning its exact bytes and blob object id, or Found=false when the
	// path is absent. It reads at the same pinned revision ArtifactExists
	// consults, so a bundle can carry loss-preserving source bytes for an
	// artifact (a spec) that is not a corpus record.
	ReadArtifact(ctx context.Context, pin StatusPin, source, path string) (StatusArtifact, error)
}

// Status runs the whole read and returns the one protocol document. It composes
// the landed packages — config resolution is already inside the pin, document
// parsing feeds repository.BuildSnapshot, and every readiness/selection/stack
// question is answered by domain — and only translates their results into the
// protocol DTO. No policy is decided here.
func Status(ctx context.Context, reader StatusReader, opts StatusOptions) StatusResult {
	// 1. Pin the authoritative context. A pin failure carries whatever context
	//    was pinned and no report body.
	pin, err := reader.PinContext(ctx, opts.RepoDir)
	if err != nil {
		return statusFailure(ctx, pin, err)
	}

	// Closed-value validation of the filters against the resolved
	// configuration. This precedes any corpus read: a bad flag value is invalid
	// input, not a health finding.
	priorities, err := validateFilters(pin.Config.Effective, opts)
	if err != nil {
		return NewStatusResult(ResultInvalidInput, StatusResult{
			Context: contextFromPin(pin),
			Reason:  ReasonStatusInvalidInput,
			Message: err.Error(),
		})
	}

	// 2. Read and parse the corpus. A blob that fails to parse becomes a
	//    finding; the rest continue.
	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		return statusFailure(ctx, pin, err)
	}
	inputs, parseFindings := parseCorpus(blobs)
	blobByPath := make(map[string]StatusBlob, len(blobs))
	for _, b := range blobs {
		blobByPath[b.Path] = b
	}

	// 3. Build the snapshot and validation report over the COMPLETE corpus,
	//    never the filtered projection.
	build, err := repository.BuildSnapshot(repository.BuildInput{
		Config:    pin.Config.Effective,
		Documents: inputs,
	})
	if err != nil {
		// A build error means the CALL was malformed — a contract violation.
		return statusFailure(ctx, pin, err)
	}
	snap := build.Snapshot

	// 4. Fetch the branch facts effective-base resolution consults: the branch
	//    of every stack ancestor of every change.
	facts, err := reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		return statusFailure(ctx, pin, err)
	}

	// 5. Per active change: readiness, dependencies, stack, projection; and the
	//    ordered ready queue from the same filter.
	ready := domain.SelectQueue(snap, facts, domain.SelectionFilter{Types: opts.Types, Priorities: priorities})
	readySet := make(map[domain.ChangeID]bool, len(ready))
	for _, c := range ready {
		readySet[c.ID()] = true
	}

	displayed := activeChanges(snap, opts.Types, priorities)
	changes := make([]StatusChange, 0, len(displayed))
	// 6. Artifact checks accumulate their findings alongside the change rows.
	var artifactFindings []StatusFinding
	for _, c := range displayed {
		changes = append(changes, statusChange(snap, c, facts, readySet, blobByPath))
		f, ferr := artifactChecks(ctx, reader, pin, c)
		if ferr != nil {
			return statusFailure(ctx, pin, ferr)
		}
		artifactFindings = append(artifactFindings, f...)
	}

	// 7. Assemble findings in their fixed order; the records inventory is
	//    computed only when opted in (change 0397). Off, records stays nil so
	//    the key is absent — never an empty array.
	var records *[]StatusRecord
	if opts.IncludeRecords {
		r := corpusRecords(snap, blobByPath)
		records = &r
	}
	findings := assembleFindings(pin.ConfigDiags, parseFindings, build.Report, artifactFindings)

	readyIDs := make([]int, 0, len(ready))
	for _, c := range ready {
		readyIDs = append(readyIDs, int(c.ID()))
	}

	return NewStatusResult(ResultApplied, StatusResult{
		Context:  contextFromPin(pin),
		Summary:  summarize(snap, changes, readyIDs, findings),
		Changes:  changes,
		Ready:    readyIDs,
		Records:  records,
		Findings: findings,
	})
}

// statusFailure maps a reader error to the one failure document: the pinned
// context so far, the classification reason, and the error's own text. It never
// carries a partial report — a failure emits exactly one document with empty
// report arrays. The operational gate's typed refusal renders as the spec's
// shared invalid-state document: the classified repository state plus the
// classifier's own findings (the same typed values `repository check` reports),
// never a status-specific copy.
func statusFailure(ctx context.Context, pin StatusPin, err error) StatusResult {
	var notOp *errRepositoryNotOperational
	if errors.As(err, &notOp) {
		return NewStatusResult(ResultInvalidState, StatusResult{
			Context:         contextFromPin(pin),
			RepositoryState: string(notOp.State),
			Findings:        refusalFindings(notOp),
			Reason:          refusalReason(notOp),
			Message:         err.Error(),
		})
	}
	result, reason := classifyStatusError(ctx, err)
	return NewStatusResult(result, StatusResult{
		Context: contextFromPin(pin),
		Reason:  reason,
		Message: err.Error(),
	})
}

// refusalReason is the stable machine reason of an operational refusal: the
// first classifier finding's code (for a legacy repository,
// ReasonLegacyRepository) — sourced from the classifier value, never respelled.
func refusalReason(notOp *errRepositoryNotOperational) string {
	if len(notOp.Findings) > 0 {
		return notOp.Findings[0].Code
	}
	return string(notOp.State)
}

// refusalFindings lifts the classifier's health findings into the status DTO's
// finding shape verbatim: code, severity, message, and the state-exact remedy.
func refusalFindings(notOp *errRepositoryNotOperational) []StatusFinding {
	out := make([]StatusFinding, 0, len(notOp.Findings))
	for _, f := range notOp.Findings {
		out = append(out, StatusFinding{
			Code:     f.Code,
			Severity: string(f.Severity),
			Path:     f.Ref,
			Message:  f.Message,
			Remedy:   f.Remedy,
		})
	}
	return out
}

// classifyStatusError maps a reader failure to its protocol result and reason.
// The operational gate's typed refusal is tested first (every ordinary command
// inherits the shared invalid-state / legacy-repository refusal through this
// one mapping), then the sentinels — a wrapped classification is authoritative
// — then a context cancellation, and everything else is a contract violation.
func classifyStatusError(ctx context.Context, err error) (Result, string) {
	var notOp *errRepositoryNotOperational
	switch {
	case errors.As(err, &notOp):
		return ResultInvalidState, refusalReason(notOp)
	case errors.Is(err, ErrStatusInvalidInput):
		return ResultInvalidInput, ReasonStatusInvalidInput
	case errors.Is(err, ErrStatusExternal):
		return ResultExternalFailed, ReasonStatusExternal
	case ctx.Err() != nil:
		return ResultInterrupted, ReasonStatusInterrupted
	default:
		return ResultInternalError, ReasonStatusInternalError
	}
}

// validateFilters checks the flag VALUES against the resolved configuration:
// every priority must be a closed domain spelling, every type a configured
// change type. It returns the parsed priorities for the selection filter.
func validateFilters(eff config.Effective, opts StatusOptions) ([]domain.Priority, error) {
	priorities := make([]domain.Priority, 0, len(opts.Priorities))
	for _, p := range opts.Priorities {
		parsed, ok := domain.ParsePriority(p)
		if !ok {
			return nil, fmt.Errorf("unknown priority %q; valid values are critical, high, medium, low", p)
		}
		priorities = append(priorities, parsed)
	}
	known := eff.ChangeTypes.Value
	for _, ty := range opts.Types {
		if !containsString(known, ty) {
			return nil, fmt.Errorf("unknown change type %q; configured change_types are %s", ty, strings.Join(known, ", "))
		}
	}
	return priorities, nil
}

// parseCorpus parses every blob. A parse failure becomes an error-severity
// finding carrying the typed error's kind as its code; a success is fed to
// BuildSnapshot as an InputDocument. One bad record never aborts the read.
func parseCorpus(blobs []StatusBlob) ([]repository.InputDocument, []StatusFinding) {
	inputs := make([]repository.InputDocument, 0, len(blobs))
	var findings []StatusFinding
	for _, b := range blobs {
		doc, err := document.Parse(b.Data)
		if err != nil {
			findings = append(findings, parseFinding(b, err))
			continue
		}
		inputs = append(inputs, repository.InputDocument{
			Kind:     b.Kind,
			Location: b.Location,
			Path:     b.Path,
			Document: doc,
		})
	}
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Path < findings[j].Path })
	return inputs, findings
}

// parseFinding normalizes a document.Parse failure into a StatusFinding.
func parseFinding(b StatusBlob, err error) StatusFinding {
	code := string(FCParseFailed)
	var de *document.Error
	if errors.As(err, &de) {
		code = string(de.Kind)
	}
	return StatusFinding{
		Code:     code,
		Severity: string(domain.SeverityError),
		Entity:   entityKindFor(b.Kind),
		Path:     b.Path,
		Message:  err.Error(),
	}
}

// stackBranches collects the recorded branch of every stack ancestor of every
// change — exactly the branch names ResolveEffectiveBase consults through
// BranchFacts. The set is sorted so the reader is asked deterministically.
func stackBranches(snap domain.Snapshot) []string {
	seen := make(map[string]bool)
	for _, c := range snap.Changes() {
		for _, ancestorID := range domain.StackAncestors(snap, c) {
			ancestor, out := snap.Change(ancestorID)
			if out != domain.LookupFound {
				continue
			}
			if b := ancestor.Branch(); b.State == domain.FieldPresent && b.Value != "" {
				seen[b.Value] = true
			}
		}
	}
	branches := make([]string, 0, len(seen))
	for b := range seen {
		branches = append(branches, b)
	}
	sort.Strings(branches)
	return branches
}

// activeChanges returns the active changes passing the type/priority
// projection, in ascending numeric ID order.
func activeChanges(snap domain.Snapshot, types []string, priorities []domain.Priority) []domain.Change {
	var out []domain.Change
	for _, c := range snap.Changes() {
		if c.Location() != domain.LocationActive {
			continue
		}
		if !matchesFilter(c, types, priorities) {
			continue
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// matchesFilter mirrors domain.SelectionFilter's projection: an empty dimension
// matches everything, and both dimensions match the change's STORED value.
func matchesFilter(c domain.Change, types []string, priorities []domain.Priority) bool {
	if len(types) > 0 && !containsString(types, c.Type()) {
		return false
	}
	if len(priorities) > 0 && !containsPriority(priorities, c.Priority()) {
		return false
	}
	return true
}

// statusChange translates one active change into its displayed row: stored
// fields verbatim, plus the readiness, dependency, and stack facts the domain
// derived, and the blob version keyed by path.
func statusChange(snap domain.Snapshot, c domain.Change, facts domain.BranchFacts, readySet map[domain.ChangeID]bool, blobByPath map[string]StatusBlob) StatusChange {
	readiness := domain.EvaluateReadiness(snap, c, facts)

	unmet := make([]int, 0, len(readiness.Dependency.Unmet))
	for _, u := range readiness.Dependency.Unmet {
		unmet = append(unmet, int(u.ID))
	}

	row := StatusChange{
		ID:           int(c.ID()),
		Slug:         c.Slug(),
		Title:        c.Title(),
		Status:       c.RawStatus(),
		Priority:     c.RawPriority(),
		Type:         c.Type(),
		Location:     string(c.Location()),
		Path:         c.Path(),
		Version:      blobByPath[c.Path()].Version,
		Readiness:    string(readiness.Kind),
		ReadinessWhy: readinessReason(readiness),
		UnmetDeps:    unmet,
		Ready:        readySet[c.ID()],
	}
	if parent, out := domain.StackParent(snap, c); out == domain.LookupFound {
		row.StackParent = int(parent.ID())
	}
	if base := domain.ResolveEffectiveBase(snap, c, facts); base.Kind == domain.BaseResolved {
		row.EffectiveBase = base.Branch
	}
	return row
}

// readinessReason renders explanatory prose for a readiness outcome. It is not
// a parseable code — the Readiness kind carries that — so the wording is free.
func readinessReason(r domain.Readiness) string {
	switch r.Kind {
	case domain.ReadyBuildReady:
		return "ready to build"
	case domain.ReadyNeedsBrainstorm:
		return "needs a design brainstorm before it can be built"
	case domain.ReadyAutoGroomBlocked:
		return "auto-groom blocked; needs a human design pass"
	case domain.ReadyWaitingDependency:
		return "waiting on unmet dependencies"
	case domain.ReadyStackBaseUnresolved:
		return fmt.Sprintf("stack base unresolved (%s)", r.StackBase.Kind)
	case domain.ReadyInvalid:
		return "change identity is not usable"
	case domain.ReadyNotProposed:
		return "not in proposed status"
	default:
		return string(r.Kind)
	}
}

// artifactChecks verifies each non-empty artifact link of an active change
// against the source its kind lives on. A missing target is an error finding;
// an empty link produces no finding (a distinct, benign state). An
// ArtifactExists error propagates as an operation failure.
func artifactChecks(ctx context.Context, reader StatusReader, pin StatusPin, c domain.Change) ([]StatusFinding, error) {
	var findings []StatusFinding
	links := []struct {
		field  string
		value  domain.OptionalString
		source string
	}{
		{"spec", c.Spec(), sourceMetadata},
		{"plan", c.Plan(), sourceIntegration},
		{"results", c.Results(), sourceIntegration},
	}
	for _, link := range links {
		if link.value.State != domain.FieldPresent || link.value.Value == "" {
			continue
		}
		exists, err := reader.ArtifactExists(ctx, pin, link.source, link.value.Value)
		if err != nil {
			return nil, err
		}
		if !exists {
			findings = append(findings, StatusFinding{
				Code:     string(FCArtifactMissing),
				Severity: string(domain.SeverityError),
				Entity:   string(domain.EntityChange),
				Identity: changeIdentity(c.ID()),
				Field:    link.field,
				Path:     link.value.Value,
				Message: fmt.Sprintf("change %s references a %s artifact that does not exist: %s",
					changeIdentity(c.ID()), link.field, link.value.Value),
			})
		}
	}
	return findings, nil
}

// corpusRecords is the artifact-integrity inventory over the COMPLETE corpus:
// changes by ascending ID, then ADRs by ascending ID, then learnings by slug —
// kind-then-identity, independent of the filter projection.
func corpusRecords(snap domain.Snapshot, blobByPath map[string]StatusBlob) []StatusRecord {
	records := make([]StatusRecord, 0)

	changes := snap.Changes()
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].ID() < changes[j].ID() })
	for _, c := range changes {
		b := blobByPath[c.Path()]
		records = append(records, StatusRecord{
			Kind:     string(repository.KindChange),
			Identity: changeIdentity(c.ID()),
			Location: string(b.Location),
			Path:     c.Path(),
			Version:  b.Version,
		})
	}

	adrs := snap.ADRs()
	sort.SliceStable(adrs, func(i, j int) bool { return adrs[i].ID() < adrs[j].ID() })
	for _, a := range adrs {
		b := blobByPath[a.Path()]
		records = append(records, StatusRecord{
			Kind:     string(repository.KindADR),
			Identity: fmt.Sprintf("%04d", int(a.ID())),
			Location: string(b.Location),
			Path:     a.Path(),
			Version:  b.Version,
		})
	}

	learnings := snap.Learnings()
	sort.SliceStable(learnings, func(i, j int) bool { return learnings[i].Slug() < learnings[j].Slug() })
	for _, l := range learnings {
		b := blobByPath[l.Path()]
		records = append(records, StatusRecord{
			Kind:     string(repository.KindLearning),
			Identity: l.Slug(),
			Location: string(b.Location),
			Path:     l.Path(),
			Version:  b.Version,
		})
	}
	return records
}

// assembleFindings concatenates the findings in their fixed order: config
// diagnostics, then parse findings, then the validation report, then the
// artifact/status-read findings. The validation report is already
// deterministically ordered; the parse and artifact groups were sorted by their
// producers.
func assembleFindings(diags []config.Diagnostic, parse []StatusFinding, report domain.ValidationReport, artifact []StatusFinding) []StatusFinding {
	findings := make([]StatusFinding, 0, len(diags)+len(parse)+len(artifact))
	for _, d := range diags {
		findings = append(findings, configFinding(d))
	}
	findings = append(findings, parse...)
	for _, f := range report.Findings() {
		findings = append(findings, validationFinding(f))
	}
	sortableArtifact := append([]StatusFinding(nil), artifact...)
	sort.SliceStable(sortableArtifact, func(i, j int) bool {
		if sortableArtifact[i].Identity != sortableArtifact[j].Identity {
			return sortableArtifact[i].Identity < sortableArtifact[j].Identity
		}
		return sortableArtifact[i].Field < sortableArtifact[j].Field
	})
	findings = append(findings, sortableArtifact...)
	return findings
}

// configFinding normalizes a configuration diagnostic. Its Path is a SETTING
// path (e.g. "finalize.gate"), not a file path, so it maps to Field; the
// provenance source is deliberately dropped, since a global config path is
// host-absolute and the protocol context forbids one.
func configFinding(d config.Diagnostic) StatusFinding {
	return StatusFinding{
		Code:     d.Code,
		Severity: normalizeSeverity(string(d.Severity)),
		Field:    d.Path,
		Message:  d.Message,
		Remedy:   d.Remedy,
	}
}

// validationFinding normalizes a domain validation finding, synthesizing its
// message from the code and detail (the domain finding carries no prose).
func validationFinding(f domain.Finding) StatusFinding {
	related := make([]string, 0, len(f.Related))
	for _, r := range f.Related {
		related = append(related, entityIdentity(r))
	}
	if len(related) == 0 {
		related = nil
	}
	return StatusFinding{
		Code:     f.Code,
		Severity: string(f.Severity),
		Entity:   string(f.Entity.Kind),
		Identity: entityIdentity(f.Entity),
		Field:    f.Field,
		Path:     f.Entity.Path,
		Related:  related,
		Message:  findingMessage(f),
	}
}

// findingMessage renders a domain finding as deterministic prose: the code,
// plus its detail pairs in sorted key order when present.
func findingMessage(f domain.Finding) string {
	if len(f.Detail) == 0 {
		return f.Code
	}
	keys := make([]string, 0, len(f.Detail))
	for k := range f.Detail {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+f.Detail[k])
	}
	return f.Code + " (" + strings.Join(parts, ", ") + ")"
}

// entityIdentity renders an entity reference's identity, matching the record
// inventory's convention: a numeric id (changes, ADRs) is the zero-padded id;
// a non-numeric entity (learnings) is its slug; the repository itself has none.
func entityIdentity(e domain.EntityRef) string {
	if e.ID != 0 {
		return fmt.Sprintf("%04d", e.ID)
	}
	if e.Slug != "" {
		return e.Slug
	}
	return ""
}

// entityKindFor maps a record kind to its finding entity-kind string.
func entityKindFor(k repository.RecordKind) string {
	switch k {
	case repository.KindChange:
		return string(domain.EntityChange)
	case repository.KindADR:
		return string(domain.EntityADR)
	case repository.KindLearning:
		return string(domain.EntityLearning)
	default:
		return ""
	}
}

// summarize computes every count from the assembled arrays, so the summary can
// never disagree with the body. Total/active and the finding tallies cover the
// complete repository; displayed reflects the projection.
func summarize(snap domain.Snapshot, changes []StatusChange, ready []int, findings []StatusFinding) StatusSummary {
	total, active := 0, 0
	for _, c := range snap.Changes() {
		total++
		if c.Location() == domain.LocationActive {
			active++
		}
	}
	errs, warns := 0, 0
	for _, f := range findings {
		switch f.Severity {
		case string(domain.SeverityError):
			errs++
		case string(domain.SeverityWarning):
			warns++
		}
	}
	return StatusSummary{
		TotalChanges:     total,
		ActiveChanges:    active,
		DisplayedChanges: len(changes),
		ReadyChanges:     len(ready),
		ADRs:             len(snap.ADRs()),
		Learnings:        len(snap.Learnings()),
		ErrorFindings:    errs,
		WarningFindings:  warns,
	}
}

// contextFromPin echoes the pin into the protocol context verbatim.
func contextFromPin(pin StatusPin) StatusContext {
	return StatusContext{
		DefaultBranch:         pin.DefaultBranch,
		DefaultBranchRevision: pin.DefaultRevision,
		IntegrationBranch:     pin.IntegrationBranch,
		IntegrationRevision:   pin.IntegrationRevision,
		MetadataRevision:      pin.MetadataRevision,
	}
}

// normalizeSeverity maps a configuration severity onto the DTO's closed set:
// info becomes notice; error and warning pass through.
func normalizeSeverity(s string) string {
	if s == string(config.SeverityInfo) {
		return "notice"
	}
	return s
}

// changeIdentity is a change id's canonical four-digit identity string.
func changeIdentity(id domain.ChangeID) string { return fmt.Sprintf("%04d", int(id)) }

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func containsPriority(list []domain.Priority, want domain.Priority) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
