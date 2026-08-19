package githubcli

import (
	"context"
	"strings"
	"testing"
)

// EnsureComment drives the marker-keyed idempotent comment through the fake gh.
// The idempotency key is the live comment whose body starts with the Docket-owned
// attempt marker, re-derived from a fresh probe on every call — never a local
// proxy. The authored body crosses only on stdin.

const (
	cmtMarker = "<!-- docket:finalize-attempt:0316:7f3a -->"
	cmtBody   = cmtMarker + "\n\nFinalize blocked: unretargeted open child PR.\n"
	cmtURL    = "https://github.com/acme/widget/pull/7#issuecomment-100"
)

func cmtRepo() Repository { return Repository{Host: "github.com", Owner: "acme", Name: "widget"} }

// cmtViewArm scripts a `pr view --json comments` response.
func cmtViewArm(stdout string, exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "view"}, Stdout: stdout, Exit: exit}
}

func cmtCommentArm(exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "comment"}, Exit: exit}
}

func TestEnsureCommentIdempotent(t *testing.T) {
	t.Run("creates-when-absent", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{
			Sequential: true,
			Invocations: []fakeArm{
				cmtViewArm(prCommentsJSON(), 0), // probe: no comment
				cmtCommentArm(0),                // act: pr comment
				cmtViewArm(prCommentsJSON(commentObj(cmtBody, cmtURL)), 0), // verify: present
			},
		})
		out, url, err := c.EnsureComment(context.Background(), cmtRepo(), 7, cmtMarker, cmtBody)
		if err != nil {
			t.Fatalf("EnsureComment: %v", err)
		}
		if out != CommentCreated {
			t.Fatalf("outcome = %q, want %q", out, CommentCreated)
		}
		if url != cmtURL {
			t.Fatalf("url = %q, want %q", url, cmtURL)
		}
		recs := log.records(t)
		if n := countArgv(recs, "pr", "comment"); n != 1 {
			t.Fatalf("pr comment issued %d times, want 1", n)
		}
		// The authored body must travel on stdin, never argv.
		var sawStdin bool
		for _, r := range recs {
			if hasArgvPrefix(r.Argv, []string{"pr", "comment"}) {
				if r.Stdin != cmtBody {
					t.Fatalf("comment stdin = %q, want %q", r.Stdin, cmtBody)
				}
				sawStdin = true
				for _, a := range r.Argv {
					if strings.Contains(a, "Finalize blocked") {
						t.Fatal("body leaked into argv")
					}
				}
			}
		}
		if !sawStdin {
			t.Fatal("no pr comment invocation witnessed")
		}
	})

	t.Run("adopts-when-present", func(t *testing.T) {
		// A comment already carries the marker: found, already, same URL, no create.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			cmtViewArm(prCommentsJSON(commentObj("unrelated chatter", "u"), commentObj(cmtBody, cmtURL)), 0),
		}})
		out, url, err := c.EnsureComment(context.Background(), cmtRepo(), 7, cmtMarker, cmtBody)
		if err != nil {
			t.Fatalf("EnsureComment: %v", err)
		}
		if out != CommentAlready {
			t.Fatalf("outcome = %q, want %q", out, CommentAlready)
		}
		if url != cmtURL {
			t.Fatalf("url = %q, want %q", url, cmtURL)
		}
		if n := countArgv(log.records(t), "pr", "comment"); n != 0 {
			t.Fatalf("pr comment issued %d times when the marker already existed, want 0", n)
		}
	})

	t.Run("probe-failure-unknown", func(t *testing.T) {
		// The comment-list probe failed: unknown, NO create (never claim a comment
		// exists, never post a possible duplicate).
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{cmtViewArm("", 1)}})
		out, _, err := c.EnsureComment(context.Background(), cmtRepo(), 7, cmtMarker, cmtBody)
		if out != CommentUnknown {
			t.Fatalf("outcome = %q, want %q", out, CommentUnknown)
		}
		if err == nil {
			t.Fatal("unknown outcome must carry a diagnostic error")
		}
		if n := countArgv(log.records(t), "pr", "comment"); n != 0 {
			t.Fatalf("pr comment issued %d times after a probe failure, want 0", n)
		}
	})

	t.Run("marker-must-prefix-body", func(t *testing.T) {
		// If the marker is not the leading line of the body, EnsureComment's own
		// idempotency key would never re-find what it posts. Refuse before any gh
		// call — invalid input, no probe, no create.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{cmtViewArm(prCommentsJSON(), 0)}})
		out, _, err := c.EnsureComment(context.Background(), cmtRepo(), 7, cmtMarker, "body without the marker line")
		if err == nil {
			t.Fatal("expected an invalid-input error")
		}
		if out != CommentUnknown {
			t.Fatalf("outcome = %q, want %q", out, CommentUnknown)
		}
		if len(log.records(t)) != 0 {
			t.Fatalf("gh was invoked %d times on invalid input, want 0", len(log.records(t)))
		}
	})
}

func TestFindCommentThreeOutcomes(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			cmtViewArm(prCommentsJSON(commentObj(cmtBody, cmtURL)), 0),
		}})
		found, url, err := c.FindComment(context.Background(), cmtRepo(), 7, cmtMarker)
		if err != nil || !found || url != cmtURL {
			t.Fatalf("found=%v url=%q err=%v; want true, %q, nil", found, url, err, cmtURL)
		}
	})
	t.Run("cleanly-absent", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
			cmtViewArm(prCommentsJSON(commentObj("nothing here", "u")), 0),
		}})
		found, _, err := c.FindComment(context.Background(), cmtRepo(), 7, cmtMarker)
		if err != nil || found {
			t.Fatalf("found=%v err=%v; want false, nil", found, err)
		}
	})
	t.Run("probe-error", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{cmtViewArm("", 1)}})
		found, _, err := c.FindComment(context.Background(), cmtRepo(), 7, cmtMarker)
		if err == nil || found {
			t.Fatalf("found=%v err=%v; want false and a non-nil error (probe error is not clean absence)", found, err)
		}
	})
}
