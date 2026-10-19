package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file pins proveCarriedContent, the one per-descendant content proof that
// root closeout and carried-child cleanup share: ancestry in the pinned
// integration history short-circuits, and anything else (a clean false OR an
// ancestry error) falls through to ProvePreserved against the ROOT's merge
// result, never the integration tip.

const (
	carryChildMerge = "1111111111111111111111111111111111111111"
	carryRootMerge  = "2222222222222222222222222222222222222222"
	carryTip        = "3333333333333333333333333333333333333333"
)

type preserveCall struct {
	remote         gitcli.RemoteName
	source, target gitcli.ObjectID
}

// scriptedCarryGit is a recording carryContentGit: it answers IsAncestor and
// ProvePreserved from scripted values and records every call it receives.
type scriptedCarryGit struct {
	ancestor     bool
	ancestorErr  error
	check        gitcli.PreservationCheck
	preserveErr  error
	ancestorRuns int
	preserves    []preserveCall
}

func (g *scriptedCarryGit) IsAncestor(_ context.Context, _ gitcli.Repository, _, _ gitcli.ObjectID) (bool, error) {
	g.ancestorRuns++
	return g.ancestor, g.ancestorErr
}

func (g *scriptedCarryGit) ProvePreserved(_ context.Context, _ gitcli.Repository, remote gitcli.RemoteName, source, target gitcli.ObjectID) (gitcli.PreservationCheck, error) {
	g.preserves = append(g.preserves, preserveCall{remote: remote, source: source, target: target})
	return g.check, g.preserveErr
}

func runCarryContent(t *testing.T, g *scriptedCarryGit, childMerge string) (carriedContent, error) {
	t.Helper()
	return proveCarriedContent(context.Background(), g, gitcli.Repository{}, childMerge,
		gitcli.ObjectID(carryRootMerge), gitcli.ObjectID(carryTip))
}

func TestProveCarriedContentAncestryShortCircuits(t *testing.T) {
	g := &scriptedCarryGit{ancestor: true}
	got, err := runCarryContent(t, g, carryChildMerge)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !got.Proven || got.Category != "" {
		t.Fatalf("got %+v, want proven with no category", got)
	}
	if len(g.preserves) != 0 {
		t.Fatalf("ProvePreserved calls = %d, want 0 on an ancestry short-circuit", len(g.preserves))
	}
}

func TestProveCarriedContentAncestryErrorFallsThrough(t *testing.T) {
	g := &scriptedCarryGit{
		ancestorErr: errors.New("exit 128: object absent locally"),
		check:       gitcli.PreservationCheck{Outcome: gitcli.PreservationProven},
	}
	got, err := runCarryContent(t, g, carryChildMerge)
	if err != nil {
		t.Fatalf("err = %v; an ancestry error must fall through to ProvePreserved, not be returned", err)
	}
	if !got.Proven || got.Category != "" {
		t.Fatalf("got %+v, want proven", got)
	}
	if len(g.preserves) != 1 {
		t.Fatalf("ProvePreserved calls = %d, want exactly 1", len(g.preserves))
	}
	call := g.preserves[0]
	if call.source != carryChildMerge {
		t.Fatalf("ProvePreserved source = %s, want the child merge %s", call.source, carryChildMerge)
	}
	if call.target != carryRootMerge {
		t.Fatalf("ProvePreserved target = %s, want the ROOT merge %s (never the integration tip %s)", call.target, carryRootMerge, carryTip)
	}
	if call.remote != originRemote {
		t.Fatalf("ProvePreserved remote = %q, want %q", call.remote, originRemote)
	}
}

func TestProveCarriedContentUnpreserved(t *testing.T) {
	g := &scriptedCarryGit{
		ancestor: false,
		check: gitcli.PreservationCheck{
			Outcome: gitcli.PreservationUnproven,
			Detail:  gitcli.PreserveEntryDiffers,
			Paths:   []gitcli.RepoPath{"a.txt"},
		},
	}
	got, err := runCarryContent(t, g, carryChildMerge)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Proven {
		t.Fatalf("got proven; an unproven preservation check must not prove the carry")
	}
	if got.Category != CarryFindingUnpreserved {
		t.Fatalf("Category = %q, want %q", got.Category, CarryFindingUnpreserved)
	}
	if got.Check.Detail != gitcli.PreserveEntryDiffers || len(got.Check.Paths) != 1 {
		t.Fatalf("Check = %+v, want the observed check carried through", got.Check)
	}
	if len(g.preserves) != 1 || g.preserves[0].target != carryRootMerge {
		t.Fatalf("preserves = %+v, want one call against the root merge", g.preserves)
	}
}

func TestProveCarriedContentMissingMerge(t *testing.T) {
	g := &scriptedCarryGit{ancestor: true}
	got, err := runCarryContent(t, g, "abc")
	if err != nil {
		t.Fatalf("err = %v; missing merge evidence is a category, not an error", err)
	}
	if got.Proven || got.Category != CarryFindingMissingMerge {
		t.Fatalf("got %+v, want unproven %q", got, CarryFindingMissingMerge)
	}
	if g.ancestorRuns != 0 || len(g.preserves) != 0 {
		t.Fatalf("git calls = ancestor %d, preserve %d; want zero for an unusable merge id", g.ancestorRuns, len(g.preserves))
	}
}

func TestProveCarriedContentObservationError(t *testing.T) {
	g := &scriptedCarryGit{ancestor: false, preserveErr: errors.New("object absent from repo and remote")}
	got, err := runCarryContent(t, g, carryChildMerge)
	if err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("err = %v, want the ProvePreserved observation error returned", err)
	}
	if got.Proven {
		t.Fatalf("got proven on an observation error; it must never read as proven")
	}
}
