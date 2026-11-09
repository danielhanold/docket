package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

const (
	ticketTestURL  = "https://tickets.example.com/browse/CLOUD-123"
	ticketTestURL2 = "https://tickets.example.com/browse/CLOUD-2"
)

// ticketLines renders the three ticket frontmatter lines for a fixture.
func ticketLines(provider, id, url string) string {
	return "ticket_provider: '" + provider + "'\nticket_id: '" + id + "'\nticket_url: '" + url + "'\n"
}

// withTicketLines splices lines into a lifecycleChange record's frontmatter.
func withTicketLines(rec, lines string) string {
	return strings.Replace(rec, "trivial: false\n", "trivial: false\n"+lines, 1)
}

func linkTicketOpFor(surfaces []string, id int, recPath string, t domain.Ticket, replace, unlink bool) changeLinkTicketOp {
	return changeLinkTicketOp{
		changeID:   id,
		path:       recPath,
		ticket:     t,
		replace:    replace,
		unlink:     unlink,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

// linkTicketPlanFor runs one Plan over files, through the same loader the
// lifecycle harness uses.
func linkTicketPlanFor(t *testing.T, files map[string]string, op changeLinkTicketOp) (transaction.MutationPlan, transaction.OperationResult) {
	t.Helper()
	tree := newFakeTree(files)
	before, err := newPlanningLoader(op.eff).Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	if before.Report.HasErrors() {
		t.Fatalf("before-state has errors: %v", before.Report.Findings())
	}
	plan, opRes, err := op.Plan(context.Background(), transaction.AttemptState{Base: tree.Revision(), State: before, Tree: tree})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return plan, opRes
}

func linkTicketReceipt(t *testing.T, plan transaction.MutationPlan) changeLinkTicketReceipt {
	t.Helper()
	var rec changeLinkTicketReceipt
	if err := json.Unmarshal(plan.Receipt, &rec); err != nil {
		t.Fatalf("decoding receipt %q: %v", plan.Receipt, err)
	}
	if rec.Op != OperationChangeLinkTicket {
		t.Errorf("receipt op = %q", rec.Op)
	}
	return rec
}

var cloud123 = domain.Ticket{Provider: "jira", ID: "CLOUD-123", URL: ticketTestURL}

func TestChangeLinkTicketRejectsBadShapeWithoutEngineCall(t *testing.T) {
	id := func(r ChangeLinkTicketRequest) ChangeLinkTicketRequest {
		r.ChangeID, r.Path, r.Revision = 7, groomPath(7, "widget"), blobV
		return r
	}
	cases := []struct {
		name  string
		req   ChangeLinkTicketRequest
		codes []string
	}{
		{"empty request", ChangeLinkTicketRequest{}, []string{"invalid-change_id", "empty-path", "empty-revision"}},
		{"no fields", id(ChangeLinkTicketRequest{}), []string{"empty-request"}},
		{"unlink with id", id(ChangeLinkTicketRequest{Unlink: true, TicketID: "CLOUD-1"}), []string{"unlink-with-ticket"}},
		{"unlink with replace", id(ChangeLinkTicketRequest{Unlink: true, Replace: true}), []string{"unlink-with-ticket"}},
		{"two fields", id(ChangeLinkTicketRequest{TicketProvider: "jira", TicketID: "CLOUD-1"}), []string{"partial-ticket"}},
		{"bad url", id(ChangeLinkTicketRequest{TicketProvider: "jira", TicketID: "CLOUD-1", TicketURL: "javascript:alert(1)"}), []string{"invalid-ticket-url"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := ChangeLinkTicket(context.Background(), PlanningDeps{}, "", c.req)
			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			var got []string
			for _, f := range res.Findings {
				got = append(got, f.Code)
			}
			if strings.Join(got, ",") != strings.Join(c.codes, ",") {
				t.Errorf("codes = %v, want %v", got, c.codes)
			}
		})
	}
}

func TestLinkTicketPlanLinks(t *testing.T) {
	recPath := groomPath(7, "widget")
	files := map[string]string{
		recPath:                 lifecycleChange(7, "widget", "in-progress"),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := linkTicketPlanFor(t, files, linkTicketOpFor([]string{"inline"}, 7, recPath, cloud123, false, false))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:                 transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})
	rec := lifecycleRecordBytes(t, plan, recPath)
	for _, want := range []string{
		"ticket_provider: 'jira'", "ticket_id: 'CLOUD-123'", "ticket_url: '" + ticketTestURL + "'",
		"updated: '2026-08-16'", "| Ticket | [CLOUD-123](" + ticketTestURL + ") |",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("record lacks %q:\n%s", want, rec)
		}
	}
	if got := linkTicketReceipt(t, plan).Disposition; got != LinkTicketDispLinked {
		t.Errorf("receipt disposition = %q, want linked", got)
	}
	if plan.CommitSubject != "change 0007 ticket linked" {
		t.Errorf("commit subject = %q", plan.CommitSubject)
	}
}

func TestLinkTicketPlanInsertsMissingKeys(t *testing.T) {
	recPath := groomPath(7, "widget")
	files := map[string]string{recPath: lifecycleChange(7, "widget", "in-progress")}
	plan, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, recPath, cloud123, false, false))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	after := newFakeTree(map[string]string{recPath: lifecycleRecordBytes(t, plan, recPath)})
	state, err := newPlanningLoader(planningTestConfig(nil)).Load(context.Background(), after)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if fs := state.Report.Findings(); len(fs) != 0 {
		t.Errorf("reloaded record has findings: %v", fs)
	}
	c, out := state.Snapshot.Change(domain.ChangeID(7))
	if out != domain.LookupFound {
		t.Fatalf("change 7 not found after reload")
	}
	if got, ok := c.Ticket(); !ok || got != cloud123 {
		t.Errorf("Ticket() = %+v, %v; want %+v", got, ok, cloud123)
	}
}

func TestLinkTicketPlanSameLinkIsAlreadyLinked(t *testing.T) {
	assertSameLinkNoOp(t, false)
}

func TestLinkTicketPlanReplaceSameIsAlreadyLinked(t *testing.T) {
	assertSameLinkNoOp(t, true)
}

func assertSameLinkNoOp(t *testing.T, replace bool) {
	t.Helper()
	recPath := groomPath(7, "widget")
	files := map[string]string{
		recPath: withTicketLines(lifecycleChange(7, "widget", "in-progress"), ticketLines("jira", "CLOUD-123", ticketTestURL)),
	}
	plan, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, recPath, cloud123, replace, false))
	if opRes.Refused {
		t.Fatalf("refused: %v", opRes.Findings)
	}
	assertValidNoOpPlan(t, plan, LinkTicketDispAlreadyLinked)
}

// assertValidNoOpPlan checks an empty plan the engine will accept as a no-op:
// no files, but a commit subject and a receipt naming the disposition.
func assertValidNoOpPlan(t *testing.T, plan transaction.MutationPlan, disposition string) {
	t.Helper()
	if len(plan.Files) != 0 {
		t.Errorf("planned %v, want an empty plan", planPaths(plan))
	}
	if plan.CommitSubject == "" {
		t.Error("empty plan has no commit subject; the engine rejects it")
	}
	if got := linkTicketReceipt(t, plan).Disposition; got != disposition {
		t.Errorf("no-op receipt disposition = %q, want %q", got, disposition)
	}
}

func cloud1Record() string {
	return withTicketLines(lifecycleChange(7, "widget", "in-progress"),
		ticketLines("jira", "CLOUD-1", "https://tickets.example.com/browse/CLOUD-1"))
}

var cloud2 = domain.Ticket{Provider: "jira", ID: "CLOUD-2", URL: ticketTestURL2}

func TestLinkTicketPlanDifferentLinkRefused(t *testing.T) {
	recPath := groomPath(7, "widget")
	plan, opRes := linkTicketPlanFor(t, map[string]string{recPath: cloud1Record()},
		linkTicketOpFor(nil, 7, recPath, cloud2, false, false))
	if !opRes.Refused || !hasDomainFindingCode(opRes.Findings, "already-linked-elsewhere") {
		t.Fatalf("refused=%v findings=%v; want already-linked-elsewhere", opRes.Refused, opRes.Findings)
	}
	if len(plan.Files) != 0 {
		t.Errorf("a refusal planned files %v", planPaths(plan))
	}
	msg := opRes.Findings[0].Detail["message"]
	if !strings.Contains(msg, "replace: true") || !strings.Contains(msg, "human") {
		t.Errorf("message does not name replace: true on the human's request: %q", msg)
	}
}

func TestLinkTicketPlanReplace(t *testing.T) {
	recPath := groomPath(7, "widget")
	plan, opRes := linkTicketPlanFor(t, map[string]string{recPath: cloud1Record()},
		linkTicketOpFor(nil, 7, recPath, cloud2, true, false))
	if opRes.Refused {
		t.Fatalf("refused: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "ticket_id: 'CLOUD-2'") || strings.Contains(rec, "CLOUD-1") {
		t.Errorf("record not replaced:\n%s", rec)
	}
	if got := linkTicketReceipt(t, plan).Disposition; got != LinkTicketDispReplaced {
		t.Errorf("receipt disposition = %q, want replaced", got)
	}
}

func TestLinkTicketPlanPartialRecordNeedsReplace(t *testing.T) {
	recPath := groomPath(7, "widget")
	files := map[string]string{
		recPath: withTicketLines(lifecycleChange(7, "widget", "in-progress"), "ticket_id: 'CLOUD-9'\n"),
	}
	_, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, recPath, cloud123, false, false))
	if !opRes.Refused || !hasDomainFindingCode(opRes.Findings, "already-linked-elsewhere") {
		t.Fatalf("partial record without replace: refused=%v findings=%v", opRes.Refused, opRes.Findings)
	}
	plan, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, recPath, cloud123, true, false))
	if opRes.Refused {
		t.Fatalf("partial record with replace refused: %v", opRes.Findings)
	}
	if got := linkTicketReceipt(t, plan).Disposition; got != LinkTicketDispReplaced {
		t.Errorf("receipt disposition = %q, want replaced", got)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "| Ticket | [CLOUD-123](") || strings.Contains(rec, "CLOUD-9") {
		t.Errorf("record not a valid CLOUD-123 link:\n%s", rec)
	}
}

func TestLinkTicketPlanUnlink(t *testing.T) {
	recPath := groomPath(7, "widget")
	files := map[string]string{
		recPath: withTicketLines(lifecycleChange(7, "widget", "in-progress"), ticketLines("jira", "CLOUD-123", ticketTestURL)),
	}
	plan, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, recPath, domain.Ticket{}, false, true))
	if opRes.Refused {
		t.Fatalf("refused: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	for _, want := range []string{"\nticket_provider:\n", "\nticket_id:\n", "\nticket_url:\n"} {
		if !strings.Contains(rec, want) {
			t.Errorf("record lacks valueless %q:\n%s", strings.TrimSpace(want), rec)
		}
	}
	if strings.Contains(rec, "| Ticket |") || strings.Contains(rec, "CLOUD-123") {
		t.Errorf("unlinked record still carries the ticket:\n%s", rec)
	}
	if got := linkTicketReceipt(t, plan).Disposition; got != LinkTicketDispUnlinked {
		t.Errorf("receipt disposition = %q, want unlinked", got)
	}
}

func TestLinkTicketPlanUnlinkNothingToUnlink(t *testing.T) {
	recPath := groomPath(7, "widget")
	for name, lines := range map[string]string{
		"absent":    "",
		"valueless": "ticket_provider:\nticket_id:\nticket_url:\n",
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{recPath: withTicketLines(lifecycleChange(7, "widget", "in-progress"), lines)}
			plan, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, recPath, domain.Ticket{}, false, true))
			if opRes.Refused {
				t.Fatalf("refused: %v", opRes.Findings)
			}
			assertValidNoOpPlan(t, plan, LinkTicketDispNothingToUnlink)
		})
	}
}

func TestLinkTicketPlanArchivedDone(t *testing.T) {
	recPath := "docs/changes/archive/2026-10-01-0007-widget.md"
	files := map[string]string{recPath: lifecycleChange(7, "widget", "done")}
	plan, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, recPath, cloud123, false, false))
	if opRes.Refused {
		t.Fatalf("refused: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{recPath: transaction.MutationReplace})
	if rec := lifecycleRecordBytes(t, plan, recPath); !strings.Contains(rec, "| Ticket | [CLOUD-123](") {
		t.Errorf("archived record lacks the Ticket row:\n%s", rec)
	}
}

func TestLinkTicketPlanPathMismatch(t *testing.T) {
	recPath := groomPath(7, "widget")
	files := map[string]string{recPath: lifecycleChange(7, "widget", "in-progress")}
	_, opRes := linkTicketPlanFor(t, files, linkTicketOpFor(nil, 7, groomPath(7, "other"), cloud123, false, false))
	if !opRes.Refused || !hasDomainFindingCode(opRes.Findings, "path-mismatch") {
		t.Fatalf("refused=%v findings=%v; want path-mismatch", opRes.Refused, opRes.Findings)
	}
}

func TestLinkTicketResultHumanText(t *testing.T) {
	applied := newChangeLinkTicketResult(ResultApplied, ChangeLinkTicketResult{ID: 7, Disposition: LinkTicketDispLinked, Revision: "abc123"})
	if got, want := applied.HumanText(), "change.link-ticket: change 0007 linked — abc123"; got != want {
		t.Errorf("applied = %q, want %q", got, want)
	}
	noop := newChangeLinkTicketResult(ResultNoOp, ChangeLinkTicketResult{ID: 7, Disposition: LinkTicketDispAlreadyLinked})
	if got, want := noop.HumanText(), "change.link-ticket: change 0007 already-linked"; got != want {
		t.Errorf("no-op = %q, want %q", got, want)
	}
	refused := newChangeLinkTicketResult(ResultInvalidState, ChangeLinkTicketResult{Findings: []StatusFinding{
		{Code: string(FCAlreadyLinkedElsewhere), Message: "change 0007 already carries a link."},
	}})
	if got, want := refused.HumanText(), "change.link-ticket: invalid-state\n  change 0007 already carries a link."; got != want {
		t.Errorf("refused = %q, want %q", got, want)
	}
}
