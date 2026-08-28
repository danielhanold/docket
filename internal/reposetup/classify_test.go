package reposetup

import (
	"testing"
)

// healthyFacts returns a Facts value that classifies healthy. Individual test
// cases start from this and flip exactly the fields under test, so a case's
// intent is legible from its mutation and no case accidentally inherits an
// unrelated defect.
func healthyFacts() Facts {
	return Facts{
		RemoteConfigured:    PresencePresent,
		RemoteDefaultBranch: BranchFact{Presence: PresencePresent, Tip: "def0"},
		RemoteIntegration:   BranchFact{Presence: PresencePresent, Tip: "int0"},
		RemoteMetadata:      BranchFact{Presence: PresencePresent, Tip: "meta0"},
		MetadataRoot:        RootParentless,
		LocalMetadata:       BranchFact{Presence: PresencePresent, Tip: "meta0"},
		LiveSurface:         PresenceAbsent,
		LegacyConfigKey:     PresenceAbsent,
		CommittedIgnoreBlock: PresencePresent,
		DocketWorktree: WorktreeFact{
			Presence:     PresencePresent,
			Registered:   PresencePresent,
			Foreign:      false,
			Clean:        PresencePresent,
			Synchronized: PresencePresent,
			HooksOff:     PresencePresent,
		},
		PrimaryClean:         PresencePresent,
		PrimaryOnIntegration: PresencePresent,
		PrimaryAtRemoteTip:   PresencePresent,
		PendingReviewPaths:   nil,
		PartialPhase:         PartialNone,
		SurfacesAuthorized:   true,
		SurfacesAgree:        PresencePresent,
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name        string
		facts       Facts
		wantState   State
		wantReason  string // if non-empty, Reasons must contain this token
		reasonsReq  bool   // Reasons must be non-empty
	}{
		{
			name:       "zero-value classifies unknown (safe default)",
			facts:      Facts{},
			wantState:  StateUnknown,
			reasonsReq: true,
		},
		{
			name: "remote-configured unknown -> unknown",
			facts: func() Facts {
				f := healthyFacts()
				f.RemoteConfigured = PresenceUnknown
				return f
			}(),
			wantState:  StateUnknown,
			wantReason: "remote-configured-unknown",
			reasonsReq: true,
		},
		{
			name: "remote-default unknown -> unknown",
			facts: func() Facts {
				f := healthyFacts()
				f.RemoteDefaultBranch.Presence = PresenceUnknown
				return f
			}(),
			wantState:  StateUnknown,
			wantReason: "remote-default-unknown",
			reasonsReq: true,
		},
		{
			name: "remote-integration unknown -> unknown",
			facts: func() Facts {
				f := healthyFacts()
				f.RemoteIntegration.Presence = PresenceUnknown
				return f
			}(),
			wantState:  StateUnknown,
			wantReason: "remote-integration-unknown",
			reasonsReq: true,
		},
		{
			// The mutation-probe case: RemoteMetadata is the ONLY unknown among
			// the required probes, so flipping the PresenceUnknown check to
			// treat Unknown as Absent must move this off `unknown`.
			name: "metadata presence unknown (sole unknown) -> unknown",
			facts: func() Facts {
				f := healthyFacts()
				f.RemoteMetadata.Presence = PresenceUnknown
				return f
			}(),
			wantState:  StateUnknown,
			wantReason: "metadata-presence-unknown",
			reasonsReq: true,
		},
		{
			name: "live surface unknown when metadata absent -> unknown",
			facts: func() Facts {
				f := healthyFacts()
				f.RemoteMetadata = BranchFact{Presence: PresenceAbsent}
				f.LiveSurface = PresenceUnknown
				return f
			}(),
			wantState:  StateUnknown,
			wantReason: "live-surface-unknown",
			reasonsReq: true,
		},
		{
			name: "metadata absent + live surface absent -> fresh",
			facts: func() Facts {
				f := healthyFacts()
				f.RemoteMetadata = BranchFact{Presence: PresenceAbsent}
				f.MetadataRoot = RootUnknown
				f.LocalMetadata = BranchFact{Presence: PresenceAbsent}
				f.LiveSurface = PresenceAbsent
				return f
			}(),
			wantState: StateFresh,
		},
		{
			name: "metadata absent + live surface present -> legacy",
			facts: func() Facts {
				f := healthyFacts()
				f.RemoteMetadata = BranchFact{Presence: PresenceAbsent}
				f.MetadataRoot = RootUnknown
				f.LocalMetadata = BranchFact{Presence: PresenceAbsent}
				f.LiveSurface = PresencePresent
				return f
			}(),
			wantState: StateLegacy,
		},
		{
			name: "metadata present + root foreign -> conflict metadata-root-foreign",
			facts: func() Facts {
				f := healthyFacts()
				f.MetadataRoot = RootForeign
				return f
			}(),
			wantState:  StateConflict,
			wantReason: "metadata-root-foreign",
			reasonsReq: true,
		},
		{
			name: "metadata present parentless + live surface present -> partial (seeded)",
			facts: func() Facts {
				f := healthyFacts()
				f.LiveSurface = PresencePresent
				return f
			}(),
			wantState:  StatePartial,
			reasonsReq: true,
		},
		{
			// The seed tree does not correspond: the gatherer sets RootForeign,
			// so the seeded-live-surface shape resolves to conflict, not partial.
			name: "metadata present + live surface present but root foreign -> conflict",
			facts: func() Facts {
				f := healthyFacts()
				f.LiveSurface = PresencePresent
				f.MetadataRoot = RootForeign
				return f
			}(),
			wantState:  StateConflict,
			wantReason: "metadata-root-foreign",
			reasonsReq: true,
		},
		{
			name: "metadata topology complete + pending review paths -> needs-review",
			facts: func() Facts {
				f := healthyFacts()
				f.PendingReviewPaths = []string{"docs/changes/active/0001-x.md"}
				return f
			}(),
			wantState: StateNeedsReview,
		},
		{
			name:      "all postconditions -> healthy",
			facts:     healthyFacts(),
			wantState: StateHealthy,
		},
		{
			name: "dirty metadata worktree -> conflict metadata-worktree-dirty",
			facts: func() Facts {
				f := healthyFacts()
				f.DocketWorktree.Clean = PresenceAbsent
				return f
			}(),
			wantState:  StateConflict,
			wantReason: "metadata-worktree-dirty",
			reasonsReq: true,
		},
		{
			name: "ahead metadata worktree -> conflict metadata-worktree-dirty",
			facts: func() Facts {
				f := healthyFacts()
				f.DocketWorktree.Synchronized = PresenceAbsent
				return f
			}(),
			wantState:  StateConflict,
			wantReason: "metadata-worktree-dirty",
			reasonsReq: true,
		},
		{
			name: "foreign .docket/ -> conflict docket-dir-foreign",
			facts: func() Facts {
				f := healthyFacts()
				f.DocketWorktree.Foreign = true
				return f
			}(),
			wantState:  StateConflict,
			wantReason: "docket-dir-foreign",
			reasonsReq: true,
		},
		{
			name: "diverged local metadata branch -> conflict local-metadata-diverged",
			facts: func() Facts {
				f := healthyFacts()
				f.LocalMetadata = BranchFact{Presence: PresencePresent, Tip: "other9"}
				return f
			}(),
			wantState:  StateConflict,
			wantReason: "local-metadata-diverged",
			reasonsReq: true,
		},
		{
			name: "surfaces disagree -> conflict surfaces-drift",
			facts: func() Facts {
				f := healthyFacts()
				f.SurfacesAgree = PresenceAbsent
				return f
			}(),
			wantState:  StateConflict,
			wantReason: "surfaces-drift",
			reasonsReq: true,
		},
		{
			// Surfaces disagreement is only meaningful when authorized: an
			// unauthorized repo with SurfacesAgree unset stays healthy.
			name: "surfaces unauthorized -> not a drift conflict",
			facts: func() Facts {
				f := healthyFacts()
				f.SurfacesAuthorized = false
				f.SurfacesAgree = PresenceUnknown
				return f
			}(),
			wantState: StateHealthy,
		},
		{
			name: "partial integration pruned -> partial",
			facts: func() Facts {
				f := healthyFacts()
				f.PartialPhase = PartialIntegrationPruned
				return f
			}(),
			wantState:  StatePartial,
			reasonsReq: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.facts)
			if got.State != tc.wantState {
				t.Fatalf("state = %q, want %q (reasons=%v)", got.State, tc.wantState, got.Reasons)
			}
			if got.State == "" {
				t.Fatalf("empty State returned for %q", tc.name)
			}
			if tc.reasonsReq && len(got.Reasons) == 0 {
				t.Fatalf("Reasons empty, want non-empty for state %q", got.State)
			}
			if tc.wantReason != "" && !containsReason(got.Reasons, tc.wantReason) {
				t.Fatalf("Reasons %v missing token %q", got.Reasons, tc.wantReason)
			}
		})
	}
}

// TestClassifyReasonsNonEmptyForFlaggedStates asserts the invariant that
// conflict/partial/unknown always carry at least one reason token, across the
// whole table.
func TestClassifyReasonsNonEmptyForFlaggedStates(t *testing.T) {
	flagged := map[State]bool{StateConflict: true, StatePartial: true, StateUnknown: true}
	probes := []Facts{
		{}, // unknown
		func() Facts { f := healthyFacts(); f.MetadataRoot = RootForeign; return f }(),         // conflict
		func() Facts { f := healthyFacts(); f.PartialPhase = PartialIntegrationPruned; return f }(), // partial
	}
	for i, f := range probes {
		c := Classify(f)
		if flagged[c.State] && len(c.Reasons) == 0 {
			t.Fatalf("probe %d: state %q has empty Reasons", i, c.State)
		}
	}
}

// TestClassifyEveryStateReachable proves each of the seven States is produced
// by some input and that no input yields an empty State.
func TestClassifyEveryStateReachable(t *testing.T) {
	reach := map[State]Facts{
		StateUnknown:     {},
		StateFresh:       func() Facts { f := healthyFacts(); f.RemoteMetadata = BranchFact{Presence: PresenceAbsent}; f.MetadataRoot = RootUnknown; f.LocalMetadata = BranchFact{Presence: PresenceAbsent}; f.LiveSurface = PresenceAbsent; return f }(),
		StateLegacy:      func() Facts { f := healthyFacts(); f.RemoteMetadata = BranchFact{Presence: PresenceAbsent}; f.MetadataRoot = RootUnknown; f.LocalMetadata = BranchFact{Presence: PresenceAbsent}; f.LiveSurface = PresencePresent; return f }(),
		StateConflict:    func() Facts { f := healthyFacts(); f.MetadataRoot = RootForeign; return f }(),
		StatePartial:     func() Facts { f := healthyFacts(); f.LiveSurface = PresencePresent; return f }(),
		StateNeedsReview: func() Facts { f := healthyFacts(); f.PendingReviewPaths = []string{"p"}; return f }(),
		StateHealthy:     healthyFacts(),
	}
	seen := map[State]bool{}
	for want, f := range reach {
		got := Classify(f)
		if got.State == "" {
			t.Fatalf("empty State for target %q", want)
		}
		if got.State != want {
			t.Fatalf("target %q reached %q instead", want, got.State)
		}
		seen[got.State] = true
	}
	for _, s := range []State{StateFresh, StateNeedsReview, StateHealthy, StateLegacy, StatePartial, StateConflict, StateUnknown} {
		if !seen[s] {
			t.Fatalf("State %q was not reached", s)
		}
	}
}

func containsReason(rs []string, want string) bool {
	for _, r := range rs {
		if r == want {
			return true
		}
	}
	return false
}
