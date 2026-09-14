package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/workspace"
)

func TestValidateResolverEntryEvidenceBindsReservationCommitAndPaths(t *testing.T) {
	stopped := strings.Repeat("a", 40)
	rec := workspace.RebaseReceipt{ResolverBudgetVersion: "1", ResolverLimit: "2", ResolverUsed: "1", ResolverReservationToken: "reservation", ResolverReservationStopped: stopped}
	state := gitcli.RebaseStatus{Disposition: gitcli.RebaseConflicted, UnmergedPaths: []string{"b.go", "a.go"}}
	if err := validateResolverEntryEvidence(rec, "reservation", state, stopped, []string{"a.go", "b.go"}); err != nil {
		t.Fatalf("valid resolver entry: %v", err)
	}
	for name, mutate := range map[string]func(*workspace.RebaseReceipt, *gitcli.RebaseStatus, *string, *[]string){
		"reservation": func(_ *workspace.RebaseReceipt, _ *gitcli.RebaseStatus, token *string, _ *[]string) { *token = "stale" },
		"stopped commit": func(r *workspace.RebaseReceipt, _ *gitcli.RebaseStatus, _ *string, _ *[]string) {
			r.ResolverReservationStopped = strings.Repeat("b", 40)
		},
		"paths": func(_ *workspace.RebaseReceipt, _ *gitcli.RebaseStatus, _ *string, paths *[]string) {
			*paths = []string{"a.go"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, s, token, paths := rec, state, "reservation", []string{"a.go", "b.go"}
			mutate(&r, &s, &token, &paths)
			if validateResolverEntryEvidence(r, token, s, stopped, paths) == nil {
				t.Fatal("accepted mismatched resolver authority")
			}
		})
	}
}
