package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
)

// openChange builds a change blob: active = in-progress under active/,
// archived = done under archive/ with a date prefix. extra is raw frontmatter.
func openChange(id int, slug string, archived bool, extra string) StatusBlob {
	status, loc := "in-progress", repository.LocationActive
	path := fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug)
	if archived {
		status, loc = "done", repository.LocationArchive
		path = fmt.Sprintf("docs/changes/archive/2026-01-03-%04d-%s.md", id, slug)
	}
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: %s\npriority: medium\ntype: feat\ncreated: 2026-01-02\n%s---\n\nBody.\n",
		id, slug, id, status, extra)
	return StatusBlob{Kind: repository.KindChange, Location: loc, Path: path, Revision: fmt.Sprintf("blob%04d%s", id, slug), Data: []byte(fm)}
}

func openSnapshot(t *testing.T, blobs ...StatusBlob) domain.Snapshot {
	t.Helper()
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: testConfig(t).Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return build.Snapshot
}

func TestInferChangeFromBranch(t *testing.T) {
	w := openChange(7, "widget", false, "branch: feat/widget\n")
	none := func(b string) string { return "branch " + b + " belongs to no change — pass a change id" }
	cases := []struct {
		branch      string
		blobs       []StatusBlob
		wantID      int
		reason, msg string
	}{
		{"feat/widget", []StatusBlob{w}, 7, "", ""},
		{"feat/old", []StatusBlob{w, openChange(3, "old", true, "branch: feat/old\n")}, 3, "", ""},              // archived fallback
		{"feat/widget", []StatusBlob{openChange(4, "widget-old", true, "branch: feat/widget\n"), w}, 7, "", ""}, // active wins over archived
		{"main", []StatusBlob{w}, 0, ReasonOpenBranchUnmatched, none("main")},
		{"docket", []StatusBlob{w}, 0, ReasonOpenBranchUnmatched, none("docket")},                                         // metadata checkout
		{"feat/fresh", []StatusBlob{openChange(9, "fresh", false, "")}, 0, ReasonOpenBranchUnmatched, none("feat/fresh")}, // no recorded branch
		{"feat/widget", []StatusBlob{openChange(12, "widget-two", false, "branch: feat/widget\n"), w}, 0,
			ReasonOpenBranchAmbiguous, "branch feat/widget is recorded by changes 7, 12 — pass a change id"},
	}
	for i, tc := range cases {
		c, f := inferChangeFromBranch(openSnapshot(t, tc.blobs...), tc.branch)
		if tc.reason == "" && (f != nil || int(c.ID()) != tc.wantID) {
			t.Errorf("case %d: got (%d, %+v), want change %d", i, c.ID(), f, tc.wantID)
		}
		if tc.reason != "" && (f == nil || f.reason != tc.reason || f.message != tc.msg || f.result != ResultInvalidInput) {
			t.Errorf("case %d: failure = %+v, want %s %q", i, f, tc.reason, tc.msg)
		}
	}
}

func probeReturning(st gitcli.CheckoutState, err error) func(context.Context, string) (gitcli.CheckoutState, error) {
	return func(context.Context, string) (gitcli.CheckoutState, error) { return st, err }
}

func TestCheckoutBranch(t *testing.T) {
	if b, f := checkoutBranch(context.Background(), probeReturning(gitcli.CheckoutState{Branch: "refs/heads/feat/widget"}, nil), "/r"); f != nil || b != "feat/widget" {
		t.Errorf("branch = (%q, %+v)", b, f)
	}
	if _, f := checkoutBranch(context.Background(), probeReturning(gitcli.CheckoutState{Detached: true}, nil), "/r"); f == nil ||
		f.reason != ReasonOpenHeadDetached || f.message != "HEAD is detached — pass a change id" {
		t.Errorf("detached = %+v", f)
	}
}

// TestCheckoutBranchProbeErrorIsNotAbsence (Review Focus 5).
func TestCheckoutBranchProbeErrorIsNotAbsence(t *testing.T) {
	_, f := checkoutBranch(context.Background(), probeReturning(gitcli.CheckoutState{}, errors.New("not a git repository")), "/r")
	if f == nil || f.reason != ReasonOpenCheckoutUnreadable || f.result != ResultExternalFailed {
		t.Fatalf("failure = %+v, want checkout-unreadable external-failed", f)
	}
}
