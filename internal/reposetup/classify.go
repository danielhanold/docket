package reposetup

// State is the seven-valued repository classification. It is a string so
// results and health JSON can carry it verbatim.
type State string

const (
	StateFresh       State = "fresh"
	StateNeedsReview State = "needs-review"
	StateHealthy     State = "healthy"
	StateLegacy      State = "legacy"
	StatePartial     State = "partial"
	StateConflict    State = "conflict"
	StateUnknown     State = "unknown"
)

// Classification is the classifier's verdict plus one stable machine-readable
// reason token per contributing fact.
type Classification struct {
	State   State
	Reasons []string // one stable machine-readable reason token per contributing fact
}

// Classify maps a fully-gathered Facts value to a Classification. It is pure:
// it never reads disk and never conflates Unknown with Absent. The ladder is
// deliberately ordered — unknown-checks first, then conflict, then partial,
// then legacy/fresh, then needs-review, then healthy — and healthy is a
// fall-through that re-verifies every conjunct rather than a default. The final
// fall-through is a conflict, so no input can ever yield an empty State.
func Classify(f Facts) Classification {
	// 1. Unknown: any required probe not proven. An errored probe arrives here
	// as PresenceUnknown and must never be read as Absent. Required probes:
	// remote configured/default/integration, metadata presence, and the live
	// surface when metadata is absent (the fact that separates fresh from
	// legacy). Each contributing fact contributes one reason token.
	var unknown []string
	if f.RemoteConfigured == PresenceUnknown {
		unknown = append(unknown, "remote-configured-unknown")
	}
	if f.RemoteDefaultBranch.Presence == PresenceUnknown {
		unknown = append(unknown, "remote-default-unknown")
	}
	if f.RemoteIntegration.Presence == PresenceUnknown {
		unknown = append(unknown, "remote-integration-unknown")
	}
	if f.RemoteMetadata.Presence == PresenceUnknown {
		unknown = append(unknown, "metadata-presence-unknown")
	}
	if f.RemoteMetadata.Presence == PresenceAbsent && f.LiveSurface == PresenceUnknown {
		unknown = append(unknown, "live-surface-unknown")
	}
	if len(unknown) > 0 {
		return Classification{State: StateUnknown, Reasons: unknown}
	}

	// 2. Conflict: any refusing fact. Collect every one so the operator sees
	// the full disposition, not just the first. Each is guarded on proven
	// state, never on Unknown.
	var conflict []string
	if f.RemoteMetadata.Presence == PresencePresent && f.MetadataRoot == RootForeign {
		conflict = append(conflict, "metadata-root-foreign")
	}
	if f.DocketWorktree.Foreign {
		conflict = append(conflict, "docket-dir-foreign")
	}
	if f.RemoteMetadata.Presence == PresencePresent && f.DocketWorktree.Presence == PresencePresent &&
		(f.DocketWorktree.Clean == PresenceAbsent || f.DocketWorktree.Synchronized == PresenceAbsent) {
		conflict = append(conflict, "metadata-worktree-dirty")
	}
	if f.RemoteMetadata.Presence == PresencePresent && f.LocalMetadata.Presence == PresencePresent &&
		f.LocalMetadata.Tip != "" && f.RemoteMetadata.Tip != "" && f.LocalMetadata.Tip != f.RemoteMetadata.Tip {
		conflict = append(conflict, "local-metadata-diverged")
	}
	if f.SurfacesAuthorized && f.SurfacesAgree == PresenceAbsent {
		conflict = append(conflict, "surfaces-drift")
	}
	if len(conflict) > 0 {
		return Classification{State: StateConflict, Reasons: conflict}
	}

	// 3. Partial: an interrupted init/migration that is safe to resume. The
	// seeded-but-unpruned shape (remote docket proven parentless, integration
	// live surface still present) resolves here only because the RootForeign
	// case above already claimed the non-corresponding seed tree as conflict.
	if f.RemoteMetadata.Presence == PresencePresent && f.MetadataRoot == RootParentless &&
		f.LiveSurface == PresencePresent {
		return Classification{State: StatePartial, Reasons: []string{"metadata-seeded-live-surface"}}
	}
	if f.PartialPhase == PartialMetadataSeeded {
		return Classification{State: StatePartial, Reasons: []string{"metadata-seeded"}}
	}
	if f.PartialPhase == PartialIntegrationPruned {
		return Classification{State: StatePartial, Reasons: []string{"integration-pruned-attach-incomplete"}}
	}

	// 4. Legacy / fresh: no remote metadata branch at all. Unknown was excluded
	// above, so here RemoteMetadata is proven Absent.
	if f.RemoteMetadata.Presence == PresenceAbsent {
		if f.LiveSurface == PresencePresent {
			return Classification{State: StateLegacy, Reasons: []string{"legacy-live-surface"}}
		}
		return Classification{State: StateFresh, Reasons: []string{"no-metadata-no-surface"}}
	}

	// 5. Needs-review: metadata topology complete but the init-planned
	// integration paths are not yet reviewed and committed.
	if f.RemoteMetadata.Presence == PresencePresent && f.MetadataRoot == RootParentless &&
		len(f.PendingReviewPaths) > 0 {
		return Classification{State: StateNeedsReview, Reasons: []string{"pending-review-paths"}}
	}

	// 6. Healthy: re-verify EVERY postcondition. This is never a default — a
	// single unmet conjunct falls through to the terminal conflict below.
	if f.RemoteMetadata.Presence == PresencePresent &&
		f.MetadataRoot == RootParentless &&
		f.LocalMetadata.Presence == PresencePresent &&
		f.DocketWorktree.Presence == PresencePresent &&
		f.DocketWorktree.Registered == PresencePresent &&
		!f.DocketWorktree.Foreign &&
		f.DocketWorktree.Clean == PresencePresent &&
		f.DocketWorktree.Synchronized == PresencePresent &&
		f.DocketWorktree.HooksOff == PresencePresent &&
		f.CommittedIgnoreBlock == PresencePresent &&
		f.LiveSurface == PresenceAbsent &&
		f.LegacyConfigKey == PresenceAbsent &&
		f.PrimaryClean == PresencePresent &&
		f.PrimaryOnIntegration == PresencePresent &&
		f.PrimaryAtRemoteTip == PresencePresent &&
		(!f.SurfacesAuthorized || f.SurfacesAgree == PresencePresent) &&
		len(f.PendingReviewPaths) == 0 {
		return Classification{State: StateHealthy}
	}

	// Terminal fall-through: metadata exists but not every postcondition is
	// proven and no specific conflict fired. Never return an empty State.
	return Classification{State: StateConflict, Reasons: []string{"postconditions-unmet"}}
}
