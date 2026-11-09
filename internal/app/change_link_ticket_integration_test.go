//go:build integration

package app

// Real-git end-to-end test for change.link-ticket, run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"strings"
	"testing"
)

func TestIntegrationRecordOpsChangeLinkTicketEndToEnd(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(7, "widget")
	archived := "docs/changes/archive/2026-10-01-0008-gadget.md"
	repo := newWorkingRepo(t, map[string]string{
		recPath:  lifecycleChange(7, "widget", "in-progress"),
		archived: lifecycleChange(8, "gadget", "done"),
	})
	node := planningDepsFor(t, repo.invocation)
	link := func(id int, p, revision string, unlink bool) ChangeLinkTicketResult {
		req := ChangeLinkTicketRequest{ChangeID: id, Path: p, Revision: revision, Unlink: unlink}
		if !unlink {
			req.TicketProvider, req.TicketID, req.TicketURL = "jira", "CLOUD-123", ticketTestURL
		}
		return ChangeLinkTicket(context.Background(), node.deps, node.dir, req)
	}

	// Link: applied, linked, exactly one new metadata commit.
	tip0 := originTip(t, repo.origin, "docket")
	rev0 := blobRevisionAt(t, repo.origin, "docket", recPath)
	res := link(7, recPath, rev0, false)
	if res.Result != ResultApplied || res.Disposition != LinkTicketDispLinked {
		t.Fatalf("link = %q %q (findings %v), want applied linked", res.Result, res.Disposition, res.Findings)
	}
	tip1 := originTip(t, repo.origin, "docket")
	if parent := runGit(t, repo.origin, "rev-parse", tip1+"^"); parent != tip0 {
		t.Errorf("link did not advance the metadata branch by exactly one commit: %s^ = %s, want %s", tip1, parent, tip0)
	}
	if rec, _ := originFile(t, repo.origin, "docket", recPath); !strings.Contains(rec, "ticket_id: 'CLOUD-123'") {
		t.Errorf("linked record on origin lacks the ticket:\n%s", rec)
	}

	// The same link again at the new revision: no-op, tip unchanged.
	rev1 := blobRevisionAt(t, repo.origin, "docket", recPath)
	res = link(7, recPath, rev1, false)
	if res.Result != ResultNoOp || res.Disposition != LinkTicketDispAlreadyLinked {
		t.Fatalf("repeat link = %q %q (findings %v), want no-op already-linked; failure %+v", res.Result, res.Disposition, res.Findings, res.Failure)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip1 {
		t.Errorf("a no-op moved the metadata branch %s -> %s", tip1, got)
	}

	// The old revision is stale.
	if res = link(7, recPath, rev0, false); res.Result != ResultContended {
		t.Fatalf("stale-revision link = %q (findings %v), want contended", res.Result, res.Findings)
	}

	// Unlink.
	res = link(7, recPath, rev1, true)
	if res.Result != ResultApplied || res.Disposition != LinkTicketDispUnlinked {
		t.Fatalf("unlink = %q %q (findings %v), want applied unlinked", res.Result, res.Disposition, res.Findings)
	}

	// An archived done record links the same way.
	res = link(8, archived, blobRevisionAt(t, repo.origin, "docket", archived), false)
	if res.Result != ResultApplied || res.Disposition != LinkTicketDispLinked {
		t.Fatalf("archived link = %q %q (findings %v), want applied linked", res.Result, res.Disposition, res.Findings)
	}
	if rec, _ := originFile(t, repo.origin, "docket", archived); !strings.Contains(rec, "| Ticket | [CLOUD-123](") {
		t.Errorf("archived record on origin lacks the Ticket row:\n%s", rec)
	}
}
