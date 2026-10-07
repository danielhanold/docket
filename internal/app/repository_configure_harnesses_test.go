package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// TestConfigureHarnessesGuard proves configure-harnesses admits healthy,
// needs-review, and the states that are only init's clean-checkout or at-tip
// preconditions away from those, and refuses every other state with the remedy
// valid there.
func TestConfigureHarnessesGuard(t *testing.T) {
	admitted := map[string]func(*reposetup.Facts){
		"healthy":      func(*reposetup.Facts) {},
		"needs-review": func(f *reposetup.Facts) { f.PendingReviewPaths = []string{".docket.yml"} },
		"needs-review, branch probe unknown": func(f *reposetup.Facts) {
			f.PendingReviewPaths = []string{".docket.yml"}
			f.PrimaryOnIntegration = reposetup.PresenceUnknown
		},
		"dirty primary":  func(f *reposetup.Facts) { f.PrimaryClean = reposetup.PresenceAbsent },
		"behind the tip": func(f *reposetup.Facts) { f.PrimaryAtRemoteTip = reposetup.PresenceAbsent },
	}
	for name, mutate := range admitted {
		t.Run("admits "+name, func(t *testing.T) {
			facts := healthyConfigureFacts()
			mutate(&facts)
			cls, refusal := configureHarnessesGuard(facts)
			if refusal != nil {
				t.Fatalf("state %q refused: %s", cls.State, refusal.HumanText())
			}
		})
	}

	refused := []struct {
		name   string
		mutate func(*reposetup.Facts)
		remedy string
	}{
		{"fresh", func(f *reposetup.Facts) {
			f.RemoteMetadata = reposetup.BranchFact{Presence: reposetup.PresenceAbsent}
		}, "docket repository init"},
		{"legacy", func(f *reposetup.Facts) {
			f.RemoteMetadata = reposetup.BranchFact{Presence: reposetup.PresenceAbsent}
			f.LiveSurface = reposetup.PresencePresent
		}, "docket repository migrate"},
		{"foreign worktree", func(f *reposetup.Facts) { f.DocketWorktree.Foreign = true }, "docket repository check"},
		{"hooks on", func(f *reposetup.Facts) { f.DocketWorktree.HooksOff = reposetup.PresenceAbsent }, "docket repository check"},
		{"healthy off the integration branch", func(f *reposetup.Facts) { f.PrimaryOnIntegration = reposetup.PresenceAbsent }, "docket repository check"},
		{"needs-review off the integration branch", func(f *reposetup.Facts) {
			f.PendingReviewPaths = []string{".docket.yml"}
			f.PrimaryOnIntegration = reposetup.PresenceAbsent
		}, "docket repository check"},
	}
	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			facts := healthyConfigureFacts()
			tc.mutate(&facts)
			cls, refusal := configureHarnessesGuard(facts)
			if refusal == nil {
				t.Fatalf("state %q admitted; want a refusal", cls.State)
			}
			if refusal.Result != ResultInvalidState {
				t.Errorf("Result = %q, want invalid-state", refusal.Result)
			}
			if !strings.Contains(refusal.HumanText(), tc.remedy) {
				t.Errorf("refusal %q does not name %q", refusal.HumanText(), tc.remedy)
			}
		})
	}
}

// Missing and invalid input are refused before ANY repository read: the nil Git
// client would fail the gather if validation ran after it.
func TestRunRepositoryConfigureHarnessesRefusesInputBeforeGather(t *testing.T) {
	cases := map[string]HarnessesOptions{
		"no flag, no chooser": {},
		"bogus":               {Set: true, Tokens: []string{"bogus"}},
		"duplicate":           {Set: true, Tokens: []string{"claude", "claude"}},
		"none with a name":    {Set: true, Tokens: []string{"none", "claude"}},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			res := RunRepositoryConfigureHarnesses(context.Background(), SetupDeps{}, ConfigureHarnessesOptions{Harnesses: opts})
			got, ok := res.(RepositoryOpResult)
			if !ok {
				t.Fatalf("result is %T, want RepositoryOpResult", res)
			}
			if got.Result != ResultInvalidInput {
				t.Errorf("Result = %q (%s), want invalid-input", got.Result, got.HumanText())
			}
		})
	}
}
