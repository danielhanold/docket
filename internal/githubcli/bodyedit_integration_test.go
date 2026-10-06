//go:build integration

package githubcli

import (
	"context"
	"strings"
	"testing"
)

// EditPullRequestBody drives probe→act→verify for one merged PR's body through
// the protocol-faithful fake gh. Each case asserts the outcome AND the witness
// log (whether an edit ran, and what it carried on stdin), so a green result
// can never hide an unexercised or an extra external mutation.

const (
	bodyOld = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ old\n<!-- docket:backlink:end -->\n\nAuthored prose.\r\n"
	bodyNew = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ new\n<!-- docket:backlink:end -->\n\nAuthored prose.\r\n"
)

func mergedPRJSON(body string) string {
	return ensPRJSON(7, "MERGED", false, ensHead, ensHeadOid, "main", ensTitle, body)
}

func editRecord(t *testing.T, log *witnessLog) (invocationRecord, int) {
	t.Helper()
	var found invocationRecord
	n := 0
	for _, r := range log.records(t) {
		if len(r.Argv) >= 2 && r.Argv[0] == "pr" && r.Argv[1] == "edit" {
			found = r
			n++
		}
	}
	return found, n
}

func TestIntegrationMergeBodyEditProbeActVerify(t *testing.T) {
	oldRev := mustDecodeOne(t, mergedPRJSON(bodyOld)).Revision

	t.Run("edited", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(0),
			retViewArm(mergedPRJSON(bodyNew), 0),
		}})
		out, pr, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyEdited {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyEdited)
		}
		if pr.Body != bodyNew {
			t.Fatalf("verified body differs from the request")
		}
		rec, n := editRecord(t, log)
		if n != 1 {
			t.Fatalf("pr edit issued %d times, want 1", n)
		}
		if rec.Stdin != bodyNew {
			t.Fatalf("edit stdin = %q, want the exact requested body", rec.Stdin)
		}
		joined := strings.Join(rec.Argv, "\x00")
		for _, want := range []string{"7", "--repo", "acme/widget", "--body-file", "-"} {
			if !strings.Contains(joined, want) {
				t.Errorf("edit argv %v lacks %q", rec.Argv, want)
			}
		}
		for _, a := range rec.Argv {
			if strings.Contains(a, "Authored prose") {
				t.Fatalf("body bytes leaked into argv: %v", rec.Argv)
			}
		}
		for _, forbidden := range []string{"--base", "--title"} {
			if strings.Contains(joined, forbidden) {
				t.Errorf("a body edit must not carry %s: %v", forbidden, rec.Argv)
			}
		}
	})

	t.Run("already", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(mergedPRJSON(bodyNew), 0)}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyAlready {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyAlready)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("pr edit issued %d times on an already-correct body, want 0", n)
		}
	})

	t.Run("contended-revision-drift", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(mergedPRJSON(bodyOld), 0)}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, "sha256:stale", bodyNew)
		if err != nil || out != BodyContended {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyContended)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("pr edit issued %d times on revision drift, want 0", n)
		}
	})

	t.Run("empty-revision-is-contended", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(mergedPRJSON(bodyOld), 0)}})
		out, _, _ := c.EditPullRequestBody(context.Background(), retRepo(), 7, "", bodyNew)
		if out != BodyContended {
			t.Fatalf("outcome = %q, want %q", out, BodyContended)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("an empty revision authorized %d edits, want 0", n)
		}
	})

	t.Run("probe-error-unknown", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm("", 1)}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if out != BodyUnknown || err == nil {
			t.Fatalf("outcome = %q err=%v, want %q with a diagnostic", out, err, BodyUnknown)
		}
		if _, n := editRecord(t, log); n != 0 {
			t.Fatalf("pr edit issued %d times after a probe error, want 0", n)
		}
	})

	t.Run("verify-shows-different-body", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(0),
			retViewArm(mergedPRJSON("a human rewrote it\n"), 0),
		}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyContended {
			t.Fatalf("outcome = %q err=%v, want %q", out, err, BodyContended)
		}
	})

	t.Run("nonzero-edit-exit-resolved-by-verify", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(1),
			retViewArm(mergedPRJSON(bodyNew), 0),
		}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if err != nil || out != BodyEdited {
			t.Fatalf("a landed edit with a lost response must verify as %q, got %q err=%v", BodyEdited, out, err)
		}
	})

	t.Run("verify-error-unknown", func(t *testing.T) {
		c, _ := newFakeClient(t, fakeScenario{Sequential: true, Invocations: []fakeArm{
			retViewArm(mergedPRJSON(bodyOld), 0),
			retEditArm(0),
			retViewArm("", 1),
		}})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 7, oldRev, bodyNew)
		if out != BodyUnknown || err == nil {
			t.Fatalf("outcome = %q err=%v, want %q with a diagnostic", out, err, BodyUnknown)
		}
	})

	t.Run("invalid-number", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{})
		out, _, err := c.EditPullRequestBody(context.Background(), retRepo(), 0, oldRev, bodyNew)
		if out != BodyUnknown || err == nil {
			t.Fatalf("outcome = %q err=%v, want %q with a diagnostic", out, err, BodyUnknown)
		}
		if len(log.records(t)) != 0 {
			t.Fatalf("an invalid request must spawn no gh process")
		}
	})
}
