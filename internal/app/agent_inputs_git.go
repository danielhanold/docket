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
	branch := strings.TrimPrefix(string(found.Branch), "refs/heads/")
	clean, err := gatedrive.WorktreeClean(a.Feature)
	if err != nil {
		return AgentRootObservation{}, err
	}
	return AgentRootObservation{Primary: primary.PrimaryWorktree, Feature: feature.Root, CommonDir: primary.CommonDir, Branch: branch, HEAD: string(found.Head), Clean: clean, CallerRoot: caller.Root}, nil
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
