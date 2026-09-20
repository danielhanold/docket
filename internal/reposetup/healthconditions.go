package reposetup

// healthconditions.go — the healthy conjunction as data. UnmetHealthConditions
// is the ONE evaluation of the classifier's terminal healthy conjunction:
// Classify selects healthy exactly when it returns empty, and EvaluateHealth
// uses the same result to supplement reports, so the diagnostic list can never
// drift from the healthy decision (change 0418; learning
// duplicated-gate-copies-the-whole-predicate). It is pure and adds no
// conditions the classifier does not enforce.

// HealthCondition is a stable identifier for one conjunct of the healthy
// postcondition set.
type HealthCondition string

// The full conjunct roster, in the fixed order the healthy conjunction
// checks them. UnmetHealthConditions reports unmet conditions in this order.
const (
	CondMetadataBranchPresent HealthCondition = "metadata-branch-present"
	CondMetadataRootVerified  HealthCondition = "metadata-root-verified"
	CondLocalMetadataPresent  HealthCondition = "local-metadata-present"
	CondWorktreePresent       HealthCondition = "docket-worktree-present"
	CondWorktreeRegistered    HealthCondition = "docket-worktree-registered"
	CondWorktreeNotForeign    HealthCondition = "docket-worktree-not-foreign"
	CondWorktreeClean         HealthCondition = "docket-worktree-clean"
	CondWorktreeSynchronized  HealthCondition = "docket-worktree-synchronized"
	CondWorktreeHooksOff      HealthCondition = "docket-worktree-hooks-off"
	CondCommittedIgnoreValid  HealthCondition = "committed-ignore-valid"
	CondLiveSurfaceAbsent     HealthCondition = "live-surface-absent"
	CondLegacyConfigKeyAbsent HealthCondition = "legacy-config-key-absent"
	CondPrimaryClean          HealthCondition = "primary-clean"
	CondPrimaryOnIntegration  HealthCondition = "primary-on-integration"
	CondPrimaryAtRemoteTip    HealthCondition = "primary-at-remote-tip"
	CondSurfacesAgree         HealthCondition = "surfaces-agree"
	CondNoPendingReviewPaths  HealthCondition = "no-pending-review-paths"
)

// UnmetHealthConditions returns the healthy-conjunction conjuncts f does not
// satisfy, in fixed order. Empty means the conjunction holds. Unknown and
// Absent are both non-satisfying — a conjunct is met only when PROVEN met —
// but presentation (unknown vs proven-wrong) is EvaluateHealth's job, not
// this evaluation's.
func UnmetHealthConditions(f Facts) []HealthCondition {
	var unmet []HealthCondition
	add := func(ok bool, c HealthCondition) {
		if !ok {
			unmet = append(unmet, c)
		}
	}
	add(f.RemoteMetadata.Presence == PresencePresent, CondMetadataBranchPresent)
	add(f.MetadataRoot == RootParentless, CondMetadataRootVerified)
	add(f.LocalMetadata.Presence == PresencePresent, CondLocalMetadataPresent)
	add(f.DocketWorktree.Presence == PresencePresent, CondWorktreePresent)
	add(f.DocketWorktree.Registered == PresencePresent, CondWorktreeRegistered)
	add(!f.DocketWorktree.Foreign, CondWorktreeNotForeign)
	add(f.DocketWorktree.Clean == PresencePresent, CondWorktreeClean)
	add(f.DocketWorktree.Synchronized == PresencePresent, CondWorktreeSynchronized)
	add(f.DocketWorktree.HooksOff == PresencePresent, CondWorktreeHooksOff)
	add(f.CommittedIgnoreBlock == PresencePresent, CondCommittedIgnoreValid)
	add(f.LiveSurface == PresenceAbsent, CondLiveSurfaceAbsent)
	add(f.LegacyConfigKey == PresenceAbsent, CondLegacyConfigKeyAbsent)
	add(f.PrimaryClean == PresencePresent, CondPrimaryClean)
	add(f.PrimaryOnIntegration == PresencePresent, CondPrimaryOnIntegration)
	add(f.PrimaryAtRemoteTip == PresencePresent, CondPrimaryAtRemoteTip)
	add(!f.SurfacesAuthorized || f.SurfacesAgree == PresencePresent, CondSurfacesAgree)
	add(len(f.PendingReviewPaths) == 0, CondNoPendingReviewPaths)
	return unmet
}
