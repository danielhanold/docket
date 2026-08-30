// Package reposetup owns everything about repository initialization,
// migration, and health that is decidable without I/O: the three-valued probe
// model and the seven-state classifier live here, gitcli- and app-free. Every
// repository fact is Present, Absent, or Unknown; an errored or unattempted
// probe is NEVER collapsed into absence (learning
// probe-error-is-not-clean-absence), and the zero value of every probe type is
// its SAFE value so a gatherer that forgets a field can never authorize a
// destructive write.
package reposetup

// Presence is a three-valued probe result. The zero value, PresenceUnknown, is
// the safe value: it means "not proven", never "proven absent".
type Presence int

const (
	PresenceUnknown Presence = iota // zero value is the SAFE value
	PresencePresent
	PresenceAbsent
)

// RootShape describes what the remote metadata branch's root ancestry proves.
// The zero value, RootUnknown, is safe.
type RootShape int

const (
	RootUnknown    RootShape = iota // incomplete or unreadable evidence — never collapsed into foreign
	RootParentless                  // a verified docket seed root with permitted descendants and merges sharing that root; the root need not equal the tip
	RootForeign                     // readable, exhausted evidence, no ownership proof (no valid seed receipt, no legacy-equivalent tree, or >1 root)
)

// BranchFact carries a branch's presence and, when Present, its tip object id.
type BranchFact struct {
	Presence Presence
	Tip      string // object id when Present, else ""
}

// WorktreeFact carries the probed state of the persistent .docket/ metadata
// worktree.
type WorktreeFact struct {
	Presence     Presence // .docket/ path state: absent, or present-and-probed
	Registered   Presence // registered as a linked worktree of THIS repo on the metadata branch
	Foreign      bool     // present but a foreign dir / escaping link / conflicting registration
	Clean        Presence
	Synchronized Presence // local tip == remote metadata tip
	HooksOff     Presence
}

// Facts is the complete classifier input. Every field defaults to the safe
// Unknown/zero value; gatherers must set what they proved, and ONLY what they
// proved.
type Facts struct {
	RemoteConfigured     Presence
	RemoteDefaultBranch  BranchFact
	RemoteIntegration    BranchFact
	RemoteMetadata       BranchFact // the remote `docket` branch
	MetadataRoot         RootShape  // meaningful only when RemoteMetadata is Present
	LocalMetadata        BranchFact
	LiveSurface          Presence // active dir or BOARD.md in the AUTHORITATIVE integration tree
	LegacyConfigKey      Presence // top-level metadata_branch key in the pinned .docket.yml bytes
	CommittedIgnoreBlock Presence // managed block valid in the integration COMMIT tree
	DocketWorktree       WorktreeFact
	PrimaryClean         Presence
	PrimaryOnIntegration Presence
	PrimaryAtRemoteTip   Presence
	PendingReviewPaths   []string // init-planned integration-worktree paths not yet committed
	PartialPhase         PartialPhase
	SurfacesAuthorized   bool     // agent_harnesses explicitly declared at repo/repo-local layer
	SurfacesAgree        Presence // 0351 plan vs bytes+ownership record; only meaningful when authorized
}

// PartialPhase names how far an interrupted init/migration got. The zero
// value, PartialNone, is safe.
type PartialPhase int

const (
	PartialNone              PartialPhase = iota
	PartialMetadataSeeded                 // remote docket proven, integration live surface still present
	PartialIntegrationPruned              // both remote postconditions proven, local attach incomplete
)
