package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestSetVisibilityInputValidation proves a bad target, and each going-private
// flag with the shared target, is invalid-input before anything is read: the
// repository directory does not even exist.
func TestSetVisibilityInputValidation(t *testing.T) {
	missing := filepath.Join(testsupport.TempDir(t), "does-not-exist")
	cases := []SetVisibilityOptions{
		{Target: "public"},
		{Target: ""},
		{Target: "shared", MetadataRemote: "/tmp/x.git"},
		{Target: "shared", DeleteSharedBranch: true},
		{Target: "shared", RemoveSharedFiles: true},
	}
	for _, o := range cases {
		res := RunRepositorySetVisibility(context.Background(), SetupDeps{RepoDir: missing}, o)
		if res.Result != ResultInvalidInput {
			t.Errorf("%+v: Result = %q (%s), want invalid-input", o, res.Result, res.HumanText())
		}
		if res.Operation != OperationRepositorySetVisibility {
			t.Errorf("%+v: Operation = %q", o, res.Operation)
		}
		if o.Target == "shared" && !strings.Contains(res.HumanText(), visibilityPrivateFlagsMessage) {
			t.Errorf("%+v: text %q does not name the going-private flags", o, res.HumanText())
		}
	}
}

// TestAbsoluteMetadataLinks proves blob/docket/ and tree/docket/ links match,
// and another branch, another host, and an empty web URL do not.
func TestAbsoluteMetadataLinks(t *testing.T) {
	const web = "https://github.com/acme/app"
	files := map[string][]byte{
		"z/blob.md":    []byte("see https://github.com/acme/app/blob/docket/x.md"),
		"a/tree.md":    []byte("dir https://github.com/acme/app/tree/docket/docs/"),
		"main.md":      []byte("see https://github.com/acme/app/blob/main/x.md"),
		"otherhost.md": []byte("see https://example.com/acme/app/blob/docket/x.md"),
		"otherrepo.md": []byte("see https://github.com/acme/other/blob/docket/x.md"),
		"plain.md":     []byte("no links"),
	}
	got := absoluteMetadataLinks(web, layout.SharedName, files)
	if want := []string{"a/tree.md", "z/blob.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("absoluteMetadataLinks = %v, want %v", got, want)
	}
	if got := absoluteMetadataLinks("", layout.SharedName, files); len(got) != 0 {
		t.Errorf("an empty web URL matched %v", got)
	}
}

// fakeOpenPRGitHub serves PR bodies and states by number.
type fakeOpenPRGitHub struct {
	prs      map[int]githubcli.PullRequest
	batchErr error
	asked    [][]int
}

func (f *fakeOpenPRGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widget"}, nil
}

func (f *fakeOpenPRGitHub) ViewPullRequestsBatch(_ context.Context, _ githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error) {
	f.asked = append(f.asked, numbers)
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	out := map[int]githubcli.BatchPRResult{}
	for _, n := range numbers {
		pr, ok := f.prs[n]
		out[n] = githubcli.BatchPRResult{Found: ok, PR: pr}
	}
	return out, nil
}

func (f *fakeOpenPRGitHub) EditPullRequestBody(context.Context, githubcli.Repository, int, string, string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error) {
	return githubcli.BodyUnknown, githubcli.PullRequest{}, errors.New("set-visibility never edits a pull request")
}

// prChange renders a change record carrying a PR reference.
func prChange(id int, slug, status string, pr int) corpusRecord {
	body := "---\nid: " + strconv.Itoa(id) + "\nslug: " + slug + "\ntitle: Change " + slug + "\nstatus: " + status +
		"\npriority: medium\ntype: feature\ncreated: 2026-01-01\nupdated: 2026-01-02\n" +
		"pr: 'https://github.com/acme/widget/pull/" + strconv.Itoa(pr) + "'\n---\n\nBody.\n"
	return corpusRecord{
		path:     "docs/changes/active/" + fmt.Sprintf("%04d", id) + "-" + slug + ".md",
		bytes:    []byte(body),
		kind:     repository.KindChange,
		location: repository.LocationActive,
	}
}

// TestListDocketTextPRs proves only OPEN pull requests of in-progress and
// implemented changes whose body carries docket text are listed, in one
// batched read; merged, markerless, and other statuses are not.
func TestListDocketTextPRs(t *testing.T) {
	cfg := derivedTestConfig()
	snap, ok := buildCorpusSnapshot(cfg, []corpusRecord{
		prChange(1, "open-marked", "implemented", 11),
		prChange(2, "merged-marked", "implemented", 12),
		prChange(3, "open-plain", "implemented", 13),
		prChange(4, "proposed-marked", "proposed", 14),
	})
	if !ok {
		t.Fatal("buildCorpusSnapshot failed")
	}
	if n := len(snap.Changes()); n != 4 {
		t.Fatalf("snapshot holds %d changes, want 4", n)
	}
	marked := "intro\n<!-- docket:backlink:start -->\nx\n<!-- docket:backlink:end -->\n"
	gh := &fakeOpenPRGitHub{prs: map[int]githubcli.PullRequest{
		11: {Number: 11, URL: "https://github.com/acme/widget/pull/11", State: githubcli.StateOpen, Body: marked},
		12: {Number: 12, URL: "https://github.com/acme/widget/pull/12", State: githubcli.StateMerged, Body: marked},
		13: {Number: 13, URL: "https://github.com/acme/widget/pull/13", State: githubcli.StateOpen, Body: "plain"},
		14: {Number: 14, URL: "https://github.com/acme/widget/pull/14", State: githubcli.StateOpen, Body: marked},
	}}
	got, err := listDocketTextPRs(context.Background(), gh, "/repo", snap.Changes())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"https://github.com/acme/widget/pull/11"}; !reflect.DeepEqual(got, want) {
		t.Errorf("listDocketTextPRs = %v, want %v", got, want)
	}
	if want := [][]int{{11, 12, 13}}; !reflect.DeepEqual(gh.asked, want) {
		t.Errorf("batches asked = %v, want %v", gh.asked, want)
	}

	gh.batchErr = errors.New("rate limited")
	if _, err := listDocketTextPRs(context.Background(), gh, "/repo", snap.Changes()); err == nil {
		t.Error("a failed batch must be an error, never an empty list")
	}
}

// previewTestState is a visibility state with every phase pending except the
// ones the test sets done.
func previewTestState() visibilityState {
	common, primary := "/repo/.git", "/repo"
	return visibilityState{
		common:        common,
		primary:       primary,
		current:       layout.Shared,
		shared:        layout.SharedLayout(common, primary),
		private:       layout.PrivateLayout(common, primary, "/data", "acme-app"),
		primaryBranch: "main",
		originDocket:  gitcli.RemoteRef{State: gitcli.RemoteRefFound, Commit: "abc"},
		foldSource:    "origin's main .docket.yml",
		headPresentCommitPaths: []string{
			".docket.yml", ".gitignore", "AGENTS.md",
		},
		headSharedFiles:          true,
		sharedStateDir:           true,
		privateVisibilityAligned: true,
	}
}

// TestVisibilityPreviewTextNamesEveryPendingPhase proves the preview names every
// phase with its status, the planned commit's exact subject and branch, and the
// warnings.
func TestVisibilityPreviewTextNamesEveryPendingPhase(t *testing.T) {
	st := previewTestState()
	o := SetVisibilityOptions{Target: "private", DeleteSharedBranch: true, RemoveSharedFiles: true}
	steps := planToPrivate(st, o)
	res := RepositorySetVisibilityResult{
		SourceRevision: "origin-docket=abc dckt=absent origin-default=def",
		Commits:        visibilityPlannedCommits(st, o, steps),
		Warnings:       []string{"a warning"},
	}
	text := visibilityPreviewText(st, o, steps, res)
	for _, want := range []string{
		"docket repository set-visibility private — plan",
		"pinned:     origin-docket=abc dckt=absent origin-default=def",
		"message: " + visibilityRemoveSubject,
		"branch:  main",
		".docket.yml (delete)",
		"- a warning",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("preview lacks %q:\n%s", want, text)
		}
	}
	for _, name := range []string{"metadata-remote", "publish", "config", "state-folder", "ignore", "metadata-worktree", "instructions", "delete-shared-branch", "remove-shared-files"} {
		if !strings.Contains(text, "[pending] "+name+":") {
			t.Errorf("preview does not list pending phase %s:\n%s", name, text)
		}
	}
	if !strings.Contains(text, "[done] align-visibility:") {
		t.Errorf("preview does not list the done phase align-visibility:\n%s", text)
	}

	shared := planToShared(st, SetVisibilityOptions{Target: "shared"})
	sharedText := visibilityPreviewText(st, SetVisibilityOptions{Target: "shared"}, shared, RepositorySetVisibilityResult{})
	for _, s := range shared {
		if !strings.Contains(sharedText, s.name+":") {
			t.Errorf("shared preview does not list phase %s:\n%s", s.name, sharedText)
		}
	}
}

// TestSetVisibilityNilExecutorIsInternalError proves a pending phase with no
// executor stops the run as an internal error, while done phases are skipped.
func TestSetVisibilityNilExecutorIsInternalError(t *testing.T) {
	ran := false
	steps := []visibilityStep{
		{name: "first", done: true},
		{name: "second", run: func(context.Context, *visibilityRun) error { ran = true; return nil }},
		{name: "third"},
	}
	res := RepositorySetVisibilityResult{Phases: visibilityPhaseRows(steps)}
	x := &visibilityRun{res: &res}
	err := runVisibilitySteps(context.Background(), x, steps)
	var internal *visibilityInternal
	if !errors.As(err, &internal) || !strings.Contains(err.Error(), "phase third has no executor") {
		t.Fatalf("err = %v, want the internal no-executor error", err)
	}
	if !ran || res.Phases[1].Status != visibilityPhaseApplied || res.Phases[2].Status != visibilityPhasePending {
		t.Errorf("phases = %+v, want second applied and third still pending", res.Phases)
	}
	if out := visibilityStopped(res, err); out.Result != ResultInternalError {
		t.Errorf("stopped Result = %q, want internal-error", out.Result)
	}
}

// TestSetVisibilityPhaseHookStopsRun proves the afterVisibilityPhase seam fires
// after each applied phase, named, and its error stops the run there.
func TestSetVisibilityPhaseHookStopsRun(t *testing.T) {
	var fired []string
	d := SetupDeps{hooks: setupHooks{afterVisibilityPhase: func(p string) error {
		fired = append(fired, p)
		if p == "b" {
			return errors.New("died after b")
		}
		return nil
	}}}
	noop := func(context.Context, *visibilityRun) error { return nil }
	steps := []visibilityStep{{name: "a", run: noop}, {name: "b", run: noop}, {name: "c", run: noop}}
	res := RepositorySetVisibilityResult{Phases: visibilityPhaseRows(steps)}
	err := runVisibilitySteps(context.Background(), &visibilityRun{d: d, res: &res}, steps)
	if err == nil || !reflect.DeepEqual(fired, []string{"a", "b"}) {
		t.Fatalf("err = %v, fired = %v; want the run stopped after b", err, fired)
	}
}
