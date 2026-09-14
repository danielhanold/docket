package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/process"
)

type gitAgentInputObserver struct{ git *gitcli.Client }

func (o gitAgentInputObserver) ObserveAgentInputs(ctx context.Context, a codexcontract.Assignment, repoDir string) (AgentRootObservation, error) {
	if repoDir == "" {
		repoDir = a.Primary
	}
	if !filepath.IsAbs(repoDir) {
		return AgentRootObservation{}, fmt.Errorf("repo-dir must be absolute")
	}
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		if os.Getenv(name) != "" {
			return AgentRootObservation{}, fmt.Errorf("Git redirect environment %s is set", name)
		}
	}
	primary, err := o.git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: a.Primary})
	if err != nil {
		return AgentRootObservation{}, err
	}
	featureRepo, err := o.git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: a.Feature})
	if err != nil {
		return AgentRootObservation{}, err
	}
	feature, err := o.git.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: a.Feature})
	if err != nil {
		return AgentRootObservation{}, err
	}
	caller, err := o.git.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return AgentRootObservation{}, err
	}
	if primary.PrimaryWorktree != a.Primary || featureRepo.PrimaryWorktree != primary.PrimaryWorktree || featureRepo.CommonDir != primary.CommonDir || feature.Root != a.Feature || (caller.Root != a.Primary && caller.Root != a.Feature) {
		return AgentRootObservation{}, fmt.Errorf("assigned roots do not identify the registered repository")
	}
	worktrees, err := o.git.ListWorktrees(ctx, primary)
	if err != nil {
		return AgentRootObservation{}, err
	}
	var found *gitcli.WorktreeInfo
	for i := range worktrees {
		canon, e := filepath.EvalSymlinks(worktrees[i].Path)
		if e == nil && canon == feature.Root {
			found = &worktrees[i]
			break
		}
	}
	if found == nil {
		return AgentRootObservation{}, fmt.Errorf("feature is not a registered worktree")
	}
	rootIdentity, err := codexcontract.ObserveRootIdentity(feature.Root, feature.GitDir)
	if err != nil {
		return AgentRootObservation{}, err
	}
	branch := strings.TrimPrefix(string(found.Branch), "refs/heads/")
	if a.Mode == "resolver" {
		if !found.Detached {
			return AgentRootObservation{}, fmt.Errorf("resolver workspace is not detached")
		}
		branch = a.Branch
	} else if found.Detached {
		return AgentRootObservation{}, fmt.Errorf("feature workspace is detached")
	}
	clean, err := gatedrive.WorktreeClean(a.Feature)
	if err != nil {
		return AgentRootObservation{}, err
	}
	fingerprint, err := gatedrive.CurrentFingerprint(a.Feature)
	if err != nil {
		return AgentRootObservation{}, err
	}
	changedPaths, err := gatedrive.ChangedPaths(a.Feature)
	if err != nil {
		return AgentRootObservation{}, err
	}
	descendant, err := o.git.IsAncestor(ctx, primary, gitcli.ObjectID(a.EntryHEAD), found.Head)
	if err != nil {
		return AgentRootObservation{}, err
	}
	committed, err := o.git.ChangedPathsBetween(ctx, primary, gitcli.ObjectID(a.EntryHEAD), found.Head)
	if err != nil {
		return AgentRootObservation{}, err
	}
	committedPaths := make([]string, 0, len(committed))
	for _, p := range committed {
		committedPaths = append(committedPaths, string(p))
	}
	worktreesAfter, err := o.git.ListWorktrees(ctx, primary)
	if err != nil {
		return AgentRootObservation{}, err
	}
	stable := false
	for _, wt := range worktreesAfter {
		canon, e := filepath.EvalSymlinks(wt.Path)
		if e == nil && canon == feature.Root && wt.Head == found.Head && wt.Branch == found.Branch && wt.Detached == found.Detached {
			stable = true
			break
		}
	}
	if !stable {
		return AgentRootObservation{}, fmt.Errorf("feature worktree identity changed during validation")
	}
	if a.Mode != "resolver" {
		refHead, err := o.git.ResolveRef(ctx, primary, gitcli.RefName("refs/heads/"+a.Branch))
		if err != nil || refHead != found.Head {
			return AgentRootObservation{}, fmt.Errorf("feature branch moved during validation")
		}
	}
	return AgentRootObservation{Primary: primary.PrimaryWorktree, Feature: feature.Root, CommonDir: primary.CommonDir, Branch: branch, HEAD: string(found.Head), Clean: clean, CallerRoot: caller.Root, RootIdentity: rootIdentity, Fingerprint: &fingerprint, ChangedPaths: changedPaths, CommittedPaths: committedPaths, EntryDescendant: descendant}, nil
}

func NewAgentInputDeps(executable string) (AgentInputDeps, error) {
	git, err := gitcli.NewClient()
	if err != nil {
		return AgentInputDeps{}, err
	}
	observer := gitAgentInputObserver{git: git}
	// Scope storage is selected per assignment at validation time; the CLI wraps
	// this observer with a validator after discovering the assignment common dir.
	_ = executable
	return AgentInputDeps{Observer: observer}, nil
}

func NewAgentScopeValidator(commonDir, executable string) (AgentChildInputValidator, error) {
	proc, err := process.NewService(executable)
	if err != nil {
		return nil, err
	}
	return gatedrive.NewSystemDriver(gatedrive.OpenStore(commonDir), proc), nil
}
