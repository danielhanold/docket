package domain

import (
	"slices"
	"time"
)

// FieldState records how an optional frontmatter field appeared in the stored
// record. The decoder needs the absent/empty/malformed/present distinction to
// report the right validation finding; laundering a malformed value into a
// zero value that later looks valid is exactly what these states prevent.
type FieldState int

// The closed set of field states. FieldAbsent is the zero value, so an unset
// optional reads as "key not present".
const (
	FieldAbsent    FieldState = iota // key not present
	FieldEmpty                       // key present, no value
	FieldMalformed                   // present but unparseable for its type
	FieldPresent                     // present and parsed
)

// OptionalString is an optional text field. Value carries the stored text
// whenever State != FieldAbsent.
type OptionalString struct {
	State FieldState
	Value string
}

// OptionalInt is an optional integer field. Raw carries the stored text
// whenever State != FieldAbsent, so a malformed value stays reportable.
type OptionalInt struct {
	State FieldState
	Value int
	Raw   string
}

// OptionalTime is an optional date or timestamp field. Raw carries the stored
// text whenever State != FieldAbsent; Value is meaningful only when the state
// is FieldPresent.
type OptionalTime struct {
	State FieldState
	Value time.Time
	Raw   string
}

// RecordLocation names where a record was read from. It lives in domain so
// ChangeSpec.Location types cleanly; internal/repository re-exports the
// vocabulary through its own input aliases.
type RecordLocation string

// The closed set of record locations.
const (
	LocationActive   RecordLocation = "active"
	LocationArchive  RecordLocation = "archive"
	LocationLedger   RecordLocation = "ledger"
	LocationArtifact RecordLocation = "artifact"
	LocationDerived  RecordLocation = "derived"
)

// ChangeSpec is the mutable input shape for NewChange. Callers populate it
// field by field; the constructed Change is an opaque immutable copy.
type ChangeSpec struct {
	ID             ChangeID
	Slug           string
	Title          string
	Status         Status
	RawStatus      string // as stored, for diagnostics
	Priority       Priority
	RawPriority    string
	Type           string
	Created        OptionalTime // date-only; Raw keeps stored text
	Updated        OptionalTime
	DependsOn      []ChangeID
	StackedOn      OptionalInt // single integer scalar or absent
	Related        []ChangeID
	DiscoveredFrom []ChangeID
	ADRs           []ADRID
	Spec           OptionalString
	Plan           OptionalString
	Results        OptionalString
	Trivial        bool
	BranchPrefix   OptionalString // per-change mint-prefix override; durable input
	Branch         OptionalString
	ClaimedAt      OptionalTime // second-precision UTC; Raw kept
	PR             OptionalString
	Issue          OptionalString
	BlockedBy      OptionalString
	Reconciled     bool
	Location       RecordLocation
	Path           string
	ArchiveDate    OptionalTime // parsed from the archive filename prefix

	HasRunHalted        bool // "## Run halted" body section present
	HasAutoGroomBlocked bool // "## Auto-groom blocked" present
	HasFinalizeBlocked  bool
	HasPublishDeferred  bool
}

// Change is an immutable change manifest. Every caller-owned slice is copied
// on construction and again on access, so neither the input nor an accessor
// result can reach into the entity.
type Change struct {
	spec ChangeSpec
}

// NewChange returns an immutable Change holding a deep copy of s.
func NewChange(s ChangeSpec) Change {
	s.DependsOn = slices.Clone(s.DependsOn)
	s.Related = slices.Clone(s.Related)
	s.DiscoveredFrom = slices.Clone(s.DiscoveredFrom)
	s.ADRs = slices.Clone(s.ADRs)
	return Change{spec: s}
}

// ID returns the change's numeric identifier.
func (c Change) ID() ChangeID { return c.spec.ID }

// Slug returns the change's slug.
func (c Change) Slug() string { return c.spec.Slug }

// Title returns the change's title.
func (c Change) Title() string { return c.spec.Title }

// Status returns the parsed lifecycle status.
func (c Change) Status() Status { return c.spec.Status }

// RawStatus returns the status text as stored.
func (c Change) RawStatus() string { return c.spec.RawStatus }

// Priority returns the parsed priority.
func (c Change) Priority() Priority { return c.spec.Priority }

// RawPriority returns the priority text as stored.
func (c Change) RawPriority() string { return c.spec.RawPriority }

// Type returns the stored change type token.
func (c Change) Type() string { return c.spec.Type }

// Created returns the optional creation date.
func (c Change) Created() OptionalTime { return c.spec.Created }

// Updated returns the optional update date.
func (c Change) Updated() OptionalTime { return c.spec.Updated }

// DependsOn returns a fresh copy of the dependency IDs.
func (c Change) DependsOn() []ChangeID { return slices.Clone(c.spec.DependsOn) }

// StackedOn returns the optional stack parent ID.
func (c Change) StackedOn() OptionalInt { return c.spec.StackedOn }

// Related returns a fresh copy of the related change IDs.
func (c Change) Related() []ChangeID { return slices.Clone(c.spec.Related) }

// DiscoveredFrom returns a fresh copy of the discovering change IDs.
func (c Change) DiscoveredFrom() []ChangeID { return slices.Clone(c.spec.DiscoveredFrom) }

// ADRs returns a fresh copy of the recorded ADR IDs.
func (c Change) ADRs() []ADRID { return slices.Clone(c.spec.ADRs) }

// Spec returns the optional spec artifact reference.
func (c Change) Spec() OptionalString { return c.spec.Spec }

// Plan returns the optional plan artifact reference.
func (c Change) Plan() OptionalString { return c.spec.Plan }

// Results returns the optional results artifact reference.
func (c Change) Results() OptionalString { return c.spec.Results }

// Trivial reports whether the change is marked trivial.
func (c Change) Trivial() bool { return c.spec.Trivial }

// BranchPrefix returns the optional per-change mint-prefix override. It is
// durable human input consumed only at claim time; once branch: is populated
// it is informational and inert.
func (c Change) BranchPrefix() OptionalString { return c.spec.BranchPrefix }

// Branch returns the optional feature branch name.
func (c Change) Branch() OptionalString { return c.spec.Branch }

// ClaimedAt returns the optional claim timestamp.
func (c Change) ClaimedAt() OptionalTime { return c.spec.ClaimedAt }

// PR returns the optional pull request reference.
func (c Change) PR() OptionalString { return c.spec.PR }

// Issue returns the optional issue reference.
func (c Change) Issue() OptionalString { return c.spec.Issue }

// BlockedBy returns the optional blocking reason.
func (c Change) BlockedBy() OptionalString { return c.spec.BlockedBy }

// Reconciled reports whether the change was reconciled against reality.
func (c Change) Reconciled() bool { return c.spec.Reconciled }

// Location returns where the record was read from.
func (c Change) Location() RecordLocation { return c.spec.Location }

// Path returns the record's repository-relative path.
func (c Change) Path() string { return c.spec.Path }

// ArchiveDate returns the optional date parsed from an archive filename.
func (c Change) ArchiveDate() OptionalTime { return c.spec.ArchiveDate }

// HasRunHalted reports whether the body carries a "## Run halted" section.
func (c Change) HasRunHalted() bool { return c.spec.HasRunHalted }

// HasAutoGroomBlocked reports whether the body carries an
// "## Auto-groom blocked" section.
func (c Change) HasAutoGroomBlocked() bool { return c.spec.HasAutoGroomBlocked }

// HasFinalizeBlocked reports whether the body carries a
// "## Finalize blocked" section.
func (c Change) HasFinalizeBlocked() bool { return c.spec.HasFinalizeBlocked }

// HasPublishDeferred reports whether the body carries a
// "## Publish deferred" section.
func (c Change) HasPublishDeferred() bool { return c.spec.HasPublishDeferred }

// ADRSpec is the mutable input shape for NewADR.
type ADRSpec struct {
	ID         ADRID
	Slug       string
	Title      string
	Status     ADRStatus
	RawStatus  string
	Date       OptionalTime
	Supersedes []ADRID
	Reverses   []ADRID
	RelatesTo  []ADRID
	Change     OptionalInt // producing change
	Path       string
	ContentID  string // opaque content identity supplied by the decoder
}

// ADR is an immutable architecture decision record.
type ADR struct {
	spec ADRSpec
}

// NewADR returns an immutable ADR holding a deep copy of s.
func NewADR(s ADRSpec) ADR {
	s.Supersedes = slices.Clone(s.Supersedes)
	s.Reverses = slices.Clone(s.Reverses)
	s.RelatesTo = slices.Clone(s.RelatesTo)
	return ADR{spec: s}
}

// ID returns the ADR's numeric identifier.
func (a ADR) ID() ADRID { return a.spec.ID }

// Slug returns the ADR's slug.
func (a ADR) Slug() string { return a.spec.Slug }

// Title returns the ADR's title.
func (a ADR) Title() string { return a.spec.Title }

// Status returns the parsed tagged ADR status.
func (a ADR) Status() ADRStatus { return a.spec.Status }

// RawStatus returns the status text as stored.
func (a ADR) RawStatus() string { return a.spec.RawStatus }

// Date returns the optional decision date.
func (a ADR) Date() OptionalTime { return a.spec.Date }

// Supersedes returns a fresh copy of the superseded ADR IDs.
func (a ADR) Supersedes() []ADRID { return slices.Clone(a.spec.Supersedes) }

// Reverses returns a fresh copy of the reversed ADR IDs.
func (a ADR) Reverses() []ADRID { return slices.Clone(a.spec.Reverses) }

// RelatesTo returns a fresh copy of the related ADR IDs.
func (a ADR) RelatesTo() []ADRID { return slices.Clone(a.spec.RelatesTo) }

// Change returns the optional producing change ID.
func (a ADR) Change() OptionalInt { return a.spec.Change }

// Path returns the record's repository-relative path.
func (a ADR) Path() string { return a.spec.Path }

// ContentID returns the opaque content identity supplied by the decoder.
func (a ADR) ContentID() string { return a.spec.ContentID }

// LearningSpec is the mutable input shape for NewLearning.
type LearningSpec struct {
	Slug       string
	Hook       string
	Topics     []string
	Changes    []ChangeID
	Created    OptionalTime
	Updated    OptionalTime
	Promotion  PromotionState
	PromotedTo OptionalString
	Content    string
	Path       string
}

// Learning is an immutable learnings-ledger entry.
type Learning struct {
	spec LearningSpec
}

// NewLearning returns an immutable Learning holding a deep copy of s.
func NewLearning(s LearningSpec) Learning {
	s.Topics = slices.Clone(s.Topics)
	s.Changes = slices.Clone(s.Changes)
	return Learning{spec: s}
}

// Slug returns the learning's slug.
func (l Learning) Slug() string { return l.spec.Slug }

// Hook returns the learning's one-line hook.
func (l Learning) Hook() string { return l.spec.Hook }

// Topics returns a fresh copy of the learning's topics.
func (l Learning) Topics() []string { return slices.Clone(l.spec.Topics) }

// Changes returns a fresh copy of the related change IDs.
func (l Learning) Changes() []ChangeID { return slices.Clone(l.spec.Changes) }

// Created returns the optional creation date.
func (l Learning) Created() OptionalTime { return l.spec.Created }

// Updated returns the optional update date.
func (l Learning) Updated() OptionalTime { return l.spec.Updated }

// Promotion returns the learning's promotion state.
func (l Learning) Promotion() PromotionState { return l.spec.Promotion }

// PromotedTo returns the optional promotion destination.
func (l Learning) PromotedTo() OptionalString { return l.spec.PromotedTo }

// Content returns the learning's authored body.
func (l Learning) Content() string { return l.spec.Content }

// Path returns the record's repository-relative path.
func (l Learning) Path() string { return l.spec.Path }

// ArtifactKind names the role an authored Markdown artifact plays.
type ArtifactKind string

// The closed set of artifact kinds. ArtifactSpecKind is spelled with the Kind
// suffix because ArtifactSpec is the constructor's input struct.
const (
	ArtifactSpecKind ArtifactKind = "spec"
	ArtifactPlan     ArtifactKind = "plan"
	ArtifactResults  ArtifactKind = "results"
	ArtifactOther    ArtifactKind = "other"
)

// ArtifactSpec is the mutable input shape for NewArtifact.
type ArtifactSpec struct {
	Path              string
	Kind              ArtifactKind
	ContentID         string
	HasBacklinkMarker bool
}

// Artifact is an immutable authored Markdown artifact — a spec, plan, results
// file, or other document — carried by path and kind with no domain
// interpretation of its prose.
type Artifact struct {
	spec ArtifactSpec
}

// NewArtifact returns an immutable Artifact holding a copy of s.
func NewArtifact(s ArtifactSpec) Artifact { return Artifact{spec: s} }

// Path returns the artifact's repository-relative path.
func (a Artifact) Path() string { return a.spec.Path }

// Kind returns the artifact's role.
func (a Artifact) Kind() ArtifactKind { return a.spec.Kind }

// ContentID returns the opaque content identity supplied by the decoder.
func (a Artifact) ContentID() string { return a.spec.ContentID }

// HasBacklinkMarker reports whether the artifact carries the managed backlink
// marker block.
func (a Artifact) HasBacklinkMarker() bool { return a.spec.HasBacklinkMarker }

// DerivedViewKind names a generated view's role.
type DerivedViewKind string

// The closed set of derived view kinds.
const (
	DerivedBoard          DerivedViewKind = "board"
	DerivedADRIndex       DerivedViewKind = "adr-index"
	DerivedLearningsIndex DerivedViewKind = "learnings-index"
	DerivedOther          DerivedViewKind = "other"
)

// DerivedViewSpec is the mutable input shape for NewDerivedView.
type DerivedViewSpec struct {
	Path string
	Kind DerivedViewKind
}

// DerivedView is an immutable generated view — a board or index — retained for
// accounting but never consulted as authority.
type DerivedView struct {
	spec DerivedViewSpec
}

// NewDerivedView returns an immutable DerivedView holding a copy of s.
func NewDerivedView(s DerivedViewSpec) DerivedView { return DerivedView{spec: s} }

// Path returns the view's repository-relative path.
func (d DerivedView) Path() string { return d.spec.Path }

// Kind returns the view's role.
func (d DerivedView) Kind() DerivedViewKind { return d.spec.Kind }

// RepositoryPolicy carries the configuration facts domain policy is allowed to
// consult, copied out of resolved configuration by the repository layer.
type RepositoryPolicy struct {
	IntegrationBranch string
	ChangeTypes       []string
	ReclaimTTLHours   int
	LearningsEnabled  bool
}

// NewRepositoryPolicy returns a deep copy of p, so neither the caller's slice
// nor the returned one can reach the other.
func NewRepositoryPolicy(p RepositoryPolicy) RepositoryPolicy {
	p.ChangeTypes = slices.Clone(p.ChangeTypes)
	return p
}
