package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/workspace"
)

// symlinkRepoFile creates a symlink at a repo-relative path (creating parents)
// pointing at target, so a committed artifact can be a symlink (mode 120000).
func symlinkRepoFile(t *testing.T, root, rel, target string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
}

// These are the real-git pre-transaction verification tests for change
// attach-plan: they drive the real ChangeAttachPlan operation over a real
// prepared workspace and a real bare metadata remote, so the from-Git guards
// (head, descent, single-artifact delta, trailer, tracked/regular file,
// balanced-and-targeted backlink, no placeholder token) are exercised end-to-end
// rather than faked. The guard table is a MUTATION test: every row is the happy
// fixture with exactly one property corrupted, and each asserts its own stable
// reason string — proof the guard reddens for the reason it names, not merely
// that something failed (learning assert-pins-outcome-not-mechanism).

// attachBacklinkBlock renders the docket:backlink block the operation expects at
// the head of an artifact, targeting change id/title at recPath. It mirrors
// render.BacklinkContent's repo-relative shape exactly (no RepoWebURL is
// configured in these fixtures), so a happy plan round-trips through verification.
func attachBacklinkBlock(id int, title, recPath string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		fmt.Sprintf("> ↩ **Change %04d — %s** — `%s`\n", id, title, recPath) +
		"<!-- docket:backlink:end -->\n"
}

// attachHappyPlan renders a well-formed plan artifact: the correct backlink plus
// an authored body carrying no placeholder token.
func attachHappyPlan(id int, title, recPath string) string {
	return attachBacklinkBlock(id, title, recPath) + "\n# Implementation Plan\n\nConcrete steps here.\n"
}

// attachSetup builds a main-mode repo with one in-progress change, prepares its
// feature workspace against the resolved base, and returns everything a
// verification row needs. The workspace sits on the feature ref at the base tip;
// a row commits its own plan variant, then runs ChangeAttachPlan.
type attachFixture struct {
	ctx        context.Context
	deps       PlanningDeps
	wdeps      WorkspaceDeps
	repo       *gitRepo
	invocation string
	wp         string // feature workspace path
	id         int
	slug       string
	recPath    string
	planPath   string
	version    string
	base       string
}

func attachSetup(t *testing.T) *attachFixture {
	t.Helper()
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	repo := newMainModeRepo(t, map[string]string{
		recPath: lifecycleChange(id, slug, "in-progress"),
	})
	version := blobVersionAt(t, repo.origin, "main", recPath)

	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}
	ctx := context.Background()

	prep := WorkspacePrepare(ctx, node.deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Version: version})
	if prep.Result != ResultApplied {
		t.Fatalf("prepare workspace = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
	}
	return &attachFixture{
		ctx: ctx, deps: node.deps, wdeps: wdeps, repo: repo, invocation: repo.invocation,
		wp: prep.Path, id: id, slug: slug, recPath: recPath,
		planPath: "docs/superpowers/plans/2026-08-17-widget-plan.md",
		version:  version, base: prep.BaseCommit,
	}
}

// reset returns the feature workspace to a clean checkout of the prepared base.
func (f *attachFixture) reset(t *testing.T) {
	t.Helper()
	runGit(t, f.wp, "reset", "-q", "--hard", f.base)
	runGit(t, f.wp, "clean", "-fdq")
}

// commitPlan writes files into the workspace and commits them, optionally adding
// the plan-path trailer, and returns the new head.
func (f *attachFixture) commitPlan(t *testing.T, files map[string]string, trailerPath string) string {
	t.Helper()
	for rel, content := range files {
		writeRepoFile(t, f.wp, rel, content)
	}
	runGit(t, f.wp, "add", "-A")
	args := []string{"commit", "-q", "-m", "write plan"}
	if trailerPath != "" {
		args = append(args, "--trailer", "Docket-Plan-Path: "+trailerPath)
	}
	runGit(t, f.wp, args...)
	return runGit(t, f.wp, "rev-parse", "HEAD")
}

// TestChangeAttachPlanGitVerificationHappyPath proves a correctly written plan
// commit passes every from-Git guard and lands the metadata transaction: the
// change record on the remote gains the plan: field, rendered by the engine.
func TestChangeAttachPlanGitVerificationHappyPath(t *testing.T) {
	f := attachSetup(t)
	head := f.commitPlan(t, map[string]string{
		f.planPath: attachHappyPlan(f.id, "A change", f.recPath),
	}, f.planPath)

	res := ChangeAttachPlan(f.ctx, f.deps, f.wdeps, f.invocation,
		ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head})
	if res.Result != ResultApplied {
		t.Fatalf("happy attach = %q (reason %q msg %q findings %v)", res.Result, res.Reason, res.Message, res.Findings)
	}
	if res.Revision == "" || res.Path != f.planPath || res.Kind != attachKindPlan {
		t.Fatalf("applied result malformed: %+v", res)
	}
	final, ok := originFile(t, f.repo.origin, "main", f.recPath)
	if !ok {
		t.Fatalf("change record missing on origin after attach")
	}
	if !strings.Contains(final, "plan: '"+f.planPath+"'") {
		t.Errorf("committed record missing the plan field:\n%s", final)
	}
}

// TestChangeAttachPlanGitVerification is the guard-table mutation test: each row
// corrupts exactly one property of the happy fixture and asserts the operation
// refuses with that guard's stable reason.
func TestChangeAttachPlanGitVerification(t *testing.T) {
	f := attachSetup(t)
	otherPlan := "docs/superpowers/plans/2026-08-17-decoy.md"

	rows := []struct {
		name   string
		build  func(t *testing.T) ChangeAttachRequest // resets, mutates, returns the request
		reason string
	}{
		{
			name: "plan outside the planning root",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				head := f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: "docs/notes/outside.md", Commit: head}
			},
			reason: ReasonAttachPathOutsideRoot,
		},
		{
			name: "commit is not the feature head",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				_ = f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)
				// The base is a real commit that is not the head.
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: f.base}
			},
			reason: ReasonAttachCommitNotHead,
		},
		{
			name: "commit does not descend from the prepared base",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A root (orphan) commit carrying a valid plan: it is the head, but it
				// does not descend from the prepared base.
				runGit(t, f.wp, "checkout", "-q", "--orphan", "_orphan")
				writeRepoFile(t, f.wp, f.planPath, attachHappyPlan(f.id, "A change", f.recPath))
				runGit(t, f.wp, "add", "-A")
				runGit(t, f.wp, "commit", "-q", "-m", "orphan plan", "--trailer", "Docket-Plan-Path: "+f.planPath)
				orphan := runGit(t, f.wp, "rev-parse", "HEAD")
				runGit(t, f.wp, "branch", "-qf", "feat/"+f.slug, "_orphan")
				runGit(t, f.wp, "checkout", "-q", "feat/"+f.slug)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: orphan}
			},
			reason: ReasonAttachCommitNotDescendant,
		},
		{
			name: "artifact path is not tracked at the commit",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// The commit adds a DIFFERENT file; the requested plan path is absent.
				head := f.commitPlan(t, map[string]string{otherPlan: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachUntrackedFile,
		},
		{
			name: "artifact is a symlink at the commit",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				symlinkRepoFile(t, f.wp, f.planPath, "README.md")
				runGit(t, f.wp, "add", "-A")
				runGit(t, f.wp, "commit", "-q", "-m", "symlink plan", "--trailer", "Docket-Plan-Path: "+f.planPath)
				head := runGit(t, f.wp, "rev-parse", "HEAD")
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachSymlinkedPlan,
		},
		{
			name: "writer commit changes two artifacts",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				head := f.commitPlan(t, map[string]string{
					f.planPath: attachHappyPlan(f.id, "A change", f.recPath),
					"docs/superpowers/plans/2026-08-17-extra.md": "# extra\n",
				}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachMultiArtifactDelta,
		},
		{
			name: "writer commit is a rename (two-sided with --no-renames)",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A prior commit places an old file; the writer commit renames it to
				// the plan path. With rename detection off this is a delete + an add.
				old := "docs/superpowers/plans/2026-08-17-old.md"
				f.commitPlan(t, map[string]string{old: attachHappyPlan(f.id, "A change", f.recPath)}, "")
				runGit(t, f.wp, "mv", old, f.planPath)
				runGit(t, f.wp, "commit", "-q", "-m", "rename plan", "--trailer", "Docket-Plan-Path: "+f.planPath)
				head := runGit(t, f.wp, "rev-parse", "HEAD")
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachMultiArtifactDelta,
		},
		{
			name: "writer commit lacks the plan-path trailer",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				head := f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, "")
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachMissingTrailer,
		},
		{
			name: "backlink markers are unbalanced",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A dangling start marker with no partner: the whole-population marker
				// validation fails the parse.
				bad := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ dangling\n\n# Plan\n\nSteps.\n"
				head := f.commitPlan(t, map[string]string{f.planPath: bad}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachUnbalancedBacklink,
		},
		{
			name: "backlink targets another change",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A balanced backlink pointing at a different change.
				bad := attachBacklinkBlock(9, "Another change", "docs/changes/active/0009-other.md") + "\n# Plan\n\nSteps.\n"
				head := f.commitPlan(t, map[string]string{f.planPath: bad}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachBacklinkMismatch,
		},
		{
			name: "plan carries an unresolved placeholder token",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				withToken := attachBacklinkBlock(f.id, "A change", f.recPath) + "\n# Plan\n\nTODO: finish this section.\n"
				head := f.commitPlan(t, map[string]string{f.planPath: withToken}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachPlaceholderToken,
		},
	}

	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			req := row.build(t)
			res := ChangeAttachPlan(f.ctx, f.deps, f.wdeps, f.invocation, req)
			if res.Result == ResultApplied {
				t.Fatalf("%s: attach applied, want a refusal", row.name)
			}
			if res.Reason != row.reason {
				t.Fatalf("%s: reason = %q, want %q (msg %q)", row.name, res.Reason, row.reason, res.Message)
			}
			// A refusal opens no transaction: the remote record keeps no plan field.
			final, ok := originFile(t, f.repo.origin, "main", f.recPath)
			if ok && strings.Contains(final, "plan: '") {
				t.Errorf("%s: a refused attach wrote the plan field to the remote", row.name)
			}
		})
	}
}
