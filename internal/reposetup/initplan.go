package reposetup

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposeed"
)

// initRootSubject is the commit subject of the orphan metadata root.
const initRootSubject = "docket: initialize metadata branch"

// InitPlan is the pure effect plan for `docket repository init` on a fresh
// repository. It names the six effects the app layer executes — no seed-corpus
// files ride along: the orphan root is committed over the empty tree, and the
// parent-facing surfaces are planned separately via reposeed from SeedInput.
type InitPlan struct {
	RootSubject   string    // orphan root commit subject
	RootTrailers  []Trailer // the OpInitRoot receipt
	MetadataRef   string    // refs/heads/<metadata branch>
	WorktreePath  string    // <primary>/.docket
	GitignorePath string    // .gitignore (edit prepared, unstaged)
	SeedInput     reposeed.PlanInput

	// DocketYMLPath is the repo-relative config path init leaves a pending,
	// unstaged test-policy edit at (human-gated, never staged — the same posture
	// as GitignorePath).
	DocketYMLPath string
	// DocketYMLBytes is the pending `.docket.yml` bytes the test-policy edit
	// produced, or nil when no write applies (already configured, ambiguous, or
	// the file already carries these exact settings).
	DocketYMLBytes []byte
	// TestDiscovery is the setup-time suite discovery outcome; its candidates are
	// reported (never fatal) on an ambiguous result.
	TestDiscovery DiscoveryOutcome
}

// PlanInit is pure: config + facts in, effects out. Callers pre-classify; this
// re-classifies and refuses any non-fresh input as defense in depth, so no
// destructive orphan-root write can ever be planned from an ambiguous or
// legacy state.
func PlanInit(cfg config.Effective, f Facts, tree TestTree, primaryRoot string) (InitPlan, error) {
	if c := Classify(f); c.State != StateFresh {
		return InitPlan{}, fmt.Errorf("reposetup: PlanInit requires a fresh repository, got state %q (reasons %v)", c.State, c.Reasons)
	}
	// Setup-time test discovery over the primary worktree tree, rendered into the
	// pending `.docket.yml` edit. Init never fails on an ambiguous outcome — it
	// reports the candidates and writes nothing (a human resolves it via
	// `docket repository configure-tests`); a probe fault or malformed config is
	// surfaced as an error rather than folded into a false "none".
	existing, err := readTreeFileOptional(tree, docketYMLRel)
	if err != nil {
		return InitPlan{}, err
	}
	bytes, outcome, err := TestPolicyEdit(cfg, existing, tree)
	if err != nil {
		return InitPlan{}, err
	}
	return InitPlan{
		RootSubject:   initRootSubject,
		RootTrailers:  Receipt{Operation: OpInitRoot}.Trailers(),
		MetadataRef:   "refs/heads/" + MetadataBranchName,
		WorktreePath:  filepath.Join(primaryRoot, ".docket"),
		GitignorePath: ".gitignore",
		SeedInput: reposeed.PlanInput{
			WorktreeRoot: primaryRoot,
			Harnesses:    authorizedHarnesses(cfg),
		},
		DocketYMLPath:  docketYMLRel,
		DocketYMLBytes: bytes,
		TestDiscovery:  outcome,
	}, nil
}

// docketYMLRel is the repo-relative config path init and migrate write the
// generated test policy into.
const docketYMLRel = ".docket.yml"

// readTreeFileOptional reads path from tree, returning (nil, nil) for an absent
// file and surfacing any real probe fault (probe-error-is-not-clean-absence).
func readTreeFileOptional(tree TestTree, path string) ([]byte, error) {
	data, err := tree.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

// authorizedHarnesses returns the repository's parent-facing opt-in tokens, but
// ONLY when agent_harnesses was declared explicitly at the repository or
// repository-local layer. Opt-in is a deliberate signal, never file presence: a
// non-explicit (built-in) value or a global-layer declaration carries no
// repository write authority and yields an empty set (the touch-nothing state).
func authorizedHarnesses(cfg config.Effective) []string {
	ah := cfg.AgentHarnesses
	if !ah.Explicit {
		return nil
	}
	switch ah.Provenance.Layer {
	case config.LayerRepository, config.LayerRepositoryLocal:
	default:
		return nil
	}
	if len(ah.Value) == 0 {
		return nil
	}
	out := make([]string, len(ah.Value))
	copy(out, ah.Value)
	return out
}
