package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
)

// fakePRBody is a scriptable FinalizePRBody keyed by PR number. Its revision is
// a digest of the current body, so a body change moves the revision exactly as
// GitHub's does. Shared with the integration tests (untagged file).
type fakePRBody struct {
	bodies  map[int]string
	viewErr error
	editErr error
	editOut githubcli.BodyEditOutcome // "" applies the edit; any other value is returned as-is
	views   int
	edits   int
}

func newFakePRBody(bodies map[int]string) *fakePRBody { return &fakePRBody{bodies: bodies} }

func (f *fakePRBody) rev(n int) string {
	sum := sha256.Sum256([]byte(f.bodies[n]))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (f *fakePRBody) ViewPullRequest(_ context.Context, _ githubcli.Repository, n int) (githubcli.PullRequest, error) {
	f.views++
	if f.viewErr != nil {
		return githubcli.PullRequest{}, f.viewErr
	}
	b, ok := f.bodies[n]
	if !ok {
		return githubcli.PullRequest{}, errors.New("fake: no such pull request")
	}
	return githubcli.PullRequest{Number: n, State: githubcli.StateMerged, Body: b, Revision: f.rev(n)}, nil
}

func (f *fakePRBody) EditPullRequestBody(_ context.Context, _ githubcli.Repository, n int, rev, body string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error) {
	f.edits++
	if f.editErr != nil {
		return githubcli.BodyUnknown, githubcli.PullRequest{}, f.editErr
	}
	if f.editOut != "" {
		return f.editOut, githubcli.PullRequest{}, nil
	}
	if rev != f.rev(n) {
		return githubcli.BodyContended, githubcli.PullRequest{}, nil
	}
	f.bodies[n] = body
	return githubcli.BodyEdited, githubcli.PullRequest{Number: n, Body: body, Revision: f.rev(n)}, nil
}

// prBodyWithActiveBacklink is a PR body as pr.publish leaves it: the backlink
// block (fallback form, naming the ACTIVE record path) at the top, then
// authored prose and a build-evidence-shaped block that must survive.
func prBodyWithActiveBacklink(activePath, authored string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change 0005 — A change** — `" + activePath + "`\n" +
		"<!-- docket:backlink:end -->\n\n" +
		authored + "\n\n" +
		"<!-- docket:build-evidence:start (generated — do not hand-edit) -->\nevidence\n<!-- docket:build-evidence:end -->\n"
}

const (
	tActive   = "docs/changes/active/0005-widget.md"
	tArchive  = "docs/changes/archive/2026-08-18-0005-widget.md"
	tInterior = "> ↩ **Change 0005 — A change** — `" + tArchive + "`"
)

// outsideBlock returns body with the backlink block's interior removed, so two
// bodies can be compared on every byte the repoint must not touch.
func outsideBlock(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, "-->")
	end := strings.Index(body, "<!-- docket:backlink:end")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("no backlink block in %q", body)
	}
	return body[:start] + body[end:]
}

func TestPlanPRBacklinkRepointRepointsActiveLink(t *testing.T) {
	body := prBodyWithActiveBacklink(tActive, "Authored PR prose.")
	updated, current, present, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || !present || !needs {
		t.Fatalf("present=%v needs=%v err=%v, want a repoint", present, needs, err)
	}
	if !strings.Contains(current, tActive) {
		t.Fatalf("current interior = %q, want the active link", current)
	}
	got := string(updated)
	if !strings.Contains(got, tArchive) || strings.Contains(got, tActive) {
		t.Fatalf("updated body not repointed:\n%s", got)
	}
	if outsideBlock(t, got) != outsideBlock(t, body) {
		t.Fatalf("bytes outside the backlink block changed:\nbefore %q\nafter  %q", body, got)
	}
}

func TestPlanPRBacklinkRepointAlreadyArchivedIsNoWork(t *testing.T) {
	// Path-keyed: a block that already names the archive path is a no-op even
	// when its title wording differs from today's render.
	body := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change 0005 — An OLDER title** — `" + tArchive + "`\n" +
		"<!-- docket:backlink:end -->\n\nprose\n"
	_, _, present, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || !present || needs {
		t.Fatalf("present=%v needs=%v err=%v, want present no-work", present, needs, err)
	}
}

func TestPlanPRBacklinkRepointNoBlockIsNeverGivenOne(t *testing.T) {
	body := "A hand-written PR description with no docket block.\n"
	updated, _, present, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || present || needs {
		t.Fatalf("present=%v needs=%v err=%v, want absent no-work", present, needs, err)
	}
	if string(updated) != body {
		t.Fatalf("a block-less body was rewritten")
	}
}

func TestPlanPRBacklinkRepointPreservesCRLFProse(t *testing.T) {
	body := strings.ReplaceAll(prBodyWithActiveBacklink(tActive, "Line one.\nLine two."), "\n", "\r\n")
	updated, _, _, needs, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior)
	if err != nil || !needs {
		t.Fatalf("needs=%v err=%v, want a repoint of a CRLF body", needs, err)
	}
	if outsideBlock(t, string(updated)) != outsideBlock(t, body) {
		t.Fatalf("CRLF bytes outside the block changed:\nbefore %q\nafter  %q", body, string(updated))
	}
}

func TestPlanPRBacklinkRepointMalformedMarkersIsError(t *testing.T) {
	body := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ dangling\n\nprose\n"
	if _, _, _, _, err := planPRBacklinkRepoint([]byte(body), tArchive, tInterior); err == nil {
		t.Fatal("a dangling backlink marker must be an error (unknown), never a clean no-op")
	}
}

func TestRepointPRBacklinkEditsOnceThenIsIdempotent(t *testing.T) {
	ed := newFakePRBody(map[int]string{7: prBodyWithActiveBacklink(tActive, "prose")})
	r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkRepointed || ed.edits != 1 {
		t.Fatalf("first = %+v edits=%d, want repointed once", r, ed.edits)
	}
	r = repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkAlready || ed.edits != 1 {
		t.Fatalf("second = %+v edits=%d, want already with no new edit", r, ed.edits)
	}
	if prBacklinkFinding(5, 7, r) != nil {
		t.Fatal("an already-correct PR must produce no finding")
	}
}

func TestRepointPRBacklinkMalformedNeverEdits(t *testing.T) {
	ed := newFakePRBody(map[int]string{7: "<!-- docket:backlink:start (generated — do not hand-edit) -->\nbad\n"})
	r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkUnknown || ed.edits != 0 {
		t.Fatalf("malformed = %+v edits=%d, want unknown with no edit", r, ed.edits)
	}
	if f := prBacklinkFinding(5, 7, r); f == nil || f.Code != ReasonPRBacklinkPending {
		t.Fatalf("malformed must surface a %s finding, got %+v", ReasonPRBacklinkPending, f)
	}
}

func TestRepointPRBacklinkFailuresArePendingFindings(t *testing.T) {
	for name, ed := range map[string]*fakePRBody{
		"view-error": {bodies: map[int]string{}, viewErr: errors.New("gh: network down")},
		"edit-error": {bodies: map[int]string{7: prBodyWithActiveBacklink(tActive, "p")}, editErr: errors.New("gh: 502")},
		"contended":  {bodies: map[int]string{7: prBodyWithActiveBacklink(tActive, "p")}, editOut: githubcli.BodyContended},
	} {
		t.Run(name, func(t *testing.T) {
			r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
			f := prBacklinkFinding(5, 7, r)
			if f == nil || f.Code != ReasonPRBacklinkPending || !strings.Contains(f.Message, "#7") {
				t.Fatalf("%s: finding = %+v, want a %s finding naming PR #7", name, f, ReasonPRBacklinkPending)
			}
			if strings.Contains(f.Message, "prose") || strings.Contains(f.Message, "Authored") {
				t.Fatalf("%s: the finding leaked PR body bytes: %q", name, f.Message)
			}
		})
	}
}

func TestRepointPRBacklinkNoBlockNoEditNoFinding(t *testing.T) {
	ed := newFakePRBody(map[int]string{7: "hand-written\n"})
	r := repointPRBacklink(context.Background(), ed, githubcli.Repository{}, 7, tArchive, tInterior)
	if r.outcome != prBacklinkNoBlock || ed.edits != 0 || prBacklinkFinding(5, 7, r) != nil {
		t.Fatalf("no-block = %+v edits=%d, want no edit and no finding", r, ed.edits)
	}
}

func TestPRBodyEditorFallsBackToTheGitHubClient(t *testing.T) {
	if prBodyEditor(FinalizeDeps{GitHub: &githubcli.Client{}}) == nil {
		t.Fatal("a *githubcli.Client in deps.GitHub must serve as the PR-body editor (production wiring)")
	}
	if prBodyEditor(FinalizeDeps{}) != nil {
		t.Fatal("no editor is wired when neither PRBody nor a capable GitHub client is present")
	}
	ed := newFakePRBody(nil)
	if prBodyEditor(FinalizeDeps{PRBody: ed}) != FinalizePRBody(ed) {
		t.Fatal("an explicit PRBody seam wins")
	}
}
