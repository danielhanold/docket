package app

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/repository"
)

// --- fixtures -------------------------------------------------------------

// ticketRecordBlob builds an active change record with the given status and extra frontmatter lines.
func ticketRecordBlob(id int, slug, status, extra string) StatusBlob {
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: '%s'\npriority: medium\ntype: feat\ncreated: 2026-01-02\n%s---\n\nBody of %d.\n",
		id, slug, id, status, extra, id)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug),
		Revision: fmt.Sprintf("blobchange%04d", id),
		Data:     []byte(fm),
	}
}

const (
	ticketSpecPath    = "docs/superpowers/specs/2026-11-08-x-design.md"
	ticketResultsPath = "docs/superpowers/results/2026-11-08-x-results.md"
	ticketPlanPath    = "docs/superpowers/plans/2026-11-08-x.md"
	ticketPR          = "https://github.com/acme/widgets/pull/9"
	ticketBlobBase    = "https://github.com/acme/widgets/blob/3333333333333333333333333333333333333333/"
)

func ticketPin(t *testing.T) StatusPin {
	t.Helper()
	pin := docketPin(t)
	pin.RepoWebURL = "https://github.com/acme/widgets"
	return pin
}

// readTicket runs the operation over a corpus with the given pin and present
// metadata artifacts.
func readTicket(t *testing.T, pin StatusPin, corpus []StatusBlob, present []string, req TicketContextRequest) (TicketContextResult, *fakeReader) {
	t.Helper()
	arts := map[string]bool{}
	for _, p := range present {
		arts[sourceMetadata+"|"+p] = true
	}
	fake := &fakeReader{pin: pin, corpus: corpus, artifacts: arts}
	return ContextTicketRead(context.Background(), contextDeps(fake), "", req), fake
}

func mustApplied(t *testing.T, got TicketContextResult) *TicketContext {
	t.Helper()
	if got.Result != ResultApplied || got.Context == nil {
		t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
	}
	return got.Context
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// --- tests ----------------------------------------------------------------

func TestContextTicketEventsHappened(t *testing.T) {
	cases := []struct {
		event string
		blob  StatusBlob
	}{
		{TicketEventSpecWritten, ticketRecordBlob(1, "a", "proposed", "spec: "+ticketSpecPath+"\n")},
		{TicketEventPROpened, ticketRecordBlob(1, "a", "implemented", "pr: '"+ticketPR+"'\n")},
		{TicketEventPRMerged, ticketRecordBlob(1, "a", "done", "pr: '"+ticketPR+"'\n")},
		{TicketEventResultsCompleted, ticketRecordBlob(1, "a", "done", "deliverable: results-artifact\nresults: "+ticketResultsPath+"\n")},
	}
	for _, tc := range cases {
		t.Run(tc.event, func(t *testing.T) {
			got, _ := readTicket(t, ticketPin(t), []StatusBlob{tc.blob}, nil, TicketContextRequest{ID: 1, Event: tc.event})
			c := mustApplied(t, got)
			if c.Event == nil || c.Event.Name != tc.event || !c.Event.Happened || c.Event.Reason != "" {
				t.Errorf("event = %+v, want %s happened", c.Event, tc.event)
			}
		})
	}
}

func TestContextTicketEventsNotHappened(t *testing.T) {
	cases := []struct {
		name, event, reason string
		blob                StatusBlob
	}{
		{"spec_written without spec", TicketEventSpecWritten, ReasonTicketNoSpec, ticketRecordBlob(1, "a", "proposed", "")},
		{"pr_opened without pr", TicketEventPROpened, ReasonTicketNoPR, ticketRecordBlob(1, "a", "proposed", "spec: "+ticketSpecPath+"\n")},
		{"pr_merged on implemented", TicketEventPRMerged, ReasonTicketMergeNotRecorded, ticketRecordBlob(1, "a", "implemented", "pr: '"+ticketPR+"'\n")},
		{"pr_merged on proposed", TicketEventPRMerged, ReasonTicketNoPR, ticketRecordBlob(1, "a", "proposed", "")},
		{"results_completed on pr change", TicketEventResultsCompleted, ReasonTicketNotResultsArtifact, ticketRecordBlob(1, "a", "done", "pr: '"+ticketPR+"'\n")},
		{"results_completed in progress", TicketEventResultsCompleted, ReasonTicketNotCompleted,
			ticketRecordBlob(1, "a", "in-progress", "deliverable: results-artifact\nbranch: 'feat/a'\nclaimed_at: '2026-01-03T00:00:00Z'\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := readTicket(t, ticketPin(t), []StatusBlob{tc.blob}, nil, TicketContextRequest{ID: 1, Event: tc.event})
			c := mustApplied(t, got)
			if c.Event == nil || c.Event.Happened || c.Event.Reason != tc.reason {
				t.Errorf("event = %+v, want not happened (%s)", c.Event, tc.reason)
			}
		})
	}
}

func TestContextTicketKilledChange(t *testing.T) {
	cases := []struct {
		event    string
		extra    string
		happened bool
		reason   string
	}{
		{TicketEventPRMerged, "pr: '" + ticketPR + "'\nspec: " + ticketSpecPath + "\n", false, ReasonTicketKilled},
		{TicketEventPRMerged, "spec: " + ticketSpecPath + "\n", false, ReasonTicketKilled},
		{TicketEventResultsCompleted, "deliverable: results-artifact\nspec: " + ticketSpecPath + "\n", false, ReasonTicketKilled},
		{TicketEventSpecWritten, "pr: '" + ticketPR + "'\nspec: " + ticketSpecPath + "\n", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.event+"/"+tc.reason, func(t *testing.T) {
			blob := ticketRecordBlob(1, "a", "killed", tc.extra)
			got, _ := readTicket(t, ticketPin(t), []StatusBlob{blob}, nil, TicketContextRequest{ID: 1, Event: tc.event})
			c := mustApplied(t, got)
			if c.Event == nil || c.Event.Happened != tc.happened || c.Event.Reason != tc.reason {
				t.Errorf("event = %+v, want happened=%t reason=%q", c.Event, tc.happened, tc.reason)
			}
		})
	}
}

func TestContextTicketMergeFacts(t *testing.T) {
	parent := ticketRecordBlob(20, "parent", "implemented", "branch: 'feat/parent'\npr: 'https://github.com/acme/widgets/pull/1'\n")
	cases := []struct {
		name string
		blob StatusBlob
		want *TicketContextMerge
	}{
		{"done", ticketRecordBlob(1, "a", "done", "pr: '"+ticketPR+"'\n"), &TicketContextMerge{DestinationBranch: "main", InIntegration: true}},
		{"stacked-merged", ticketRecordBlob(1, "a", "stacked-merged", "stacked_on: 20\npr: '"+ticketPR+"'\n"), &TicketContextMerge{DestinationBranch: "feat/parent", InIntegration: false}},
		{"implemented", ticketRecordBlob(1, "a", "implemented", "pr: '"+ticketPR+"'\n"), nil},
	}
	for _, tc := range cases {
		for _, event := range []string{"", TicketEventPRMerged} {
			t.Run(tc.name+"/event="+event, func(t *testing.T) {
				got, _ := readTicket(t, ticketPin(t), []StatusBlob{tc.blob, parent}, nil, TicketContextRequest{ID: 1, Event: event})
				c := mustApplied(t, got)
				if !reflect.DeepEqual(c.Facts.Merge, tc.want) {
					t.Errorf("merge = %+v, want %+v", c.Facts.Merge, tc.want)
				}
				if c.Facts.PRURL != ticketPR {
					t.Errorf("pr_url = %q, want %q", c.Facts.PRURL, ticketPR)
				}
			})
		}
	}
}

func TestContextTicketStackedMergedParentMissing(t *testing.T) {
	blob := ticketRecordBlob(1, "a", "stacked-merged", "stacked_on: 99\npr: '"+ticketPR+"'\n")
	got, _ := readTicket(t, ticketPin(t), []StatusBlob{blob}, nil, TicketContextRequest{ID: 1, Event: TicketEventPRMerged})
	c := mustApplied(t, got)
	if c.Event == nil || !c.Event.Happened {
		t.Errorf("event = %+v, want pr_merged happened", c.Event)
	}
	if c.Facts.Merge == nil || c.Facts.Merge.InIntegration || c.Facts.Merge.DestinationBranch != "" {
		t.Errorf("merge = %+v, want {\"\", false}", c.Facts.Merge)
	}
}

func TestContextTicketTicketStates(t *testing.T) {
	cases := []struct {
		name  string
		extra string
		want  TicketContextTicket
	}{
		{"linked", "ticket_provider: jira\nticket_id: 'CLOUD-123'\nticket_url: 'https://acme.atlassian.net/browse/CLOUD-123'\n",
			TicketContextTicket{State: TicketStateLinked, Provider: "jira", ID: "CLOUD-123", URL: "https://acme.atlassian.net/browse/CLOUD-123"}},
		{"partial fields", "ticket_provider: jira\nticket_id: 'CLOUD-123'\n", TicketContextTicket{State: TicketStatePartial}},
		{"invalid url", "ticket_provider: jira\nticket_id: 'CLOUD-123'\nticket_url: 'javascript:x'\n", TicketContextTicket{State: TicketStatePartial}},
		{"not linked", "", TicketContextTicket{State: TicketStateNotLinked}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := readTicket(t, ticketPin(t), []StatusBlob{ticketRecordBlob(1, "a", "proposed", tc.extra)}, nil, TicketContextRequest{ID: 1})
			c := mustApplied(t, got)
			if c.Ticket != tc.want {
				t.Errorf("ticket = %+v, want %+v", c.Ticket, tc.want)
			}
		})
	}
	if s := mustJSON(t, TicketContextTicket{State: TicketStateNotLinked}); s != `{"state":"not-linked"}` {
		t.Errorf("not-linked ticket JSON = %s", s)
	}
}

func TestContextTicketSharedPinnedURLs(t *testing.T) {
	pin := ticketPin(t)
	blob := ticketRecordBlob(1, "a", "proposed", "spec: "+ticketSpecPath+"\nresults: "+ticketResultsPath+"\n")
	got, _ := readTicket(t, pin, []StatusBlob{blob}, []string{ticketSpecPath, ticketResultsPath}, TicketContextRequest{ID: 1})
	c := mustApplied(t, got)
	want := []TicketContextArtifact{
		{Role: "spec", Path: ticketSpecPath, URL: ticketBlobBase + ticketSpecPath},
		{Role: "results", Path: ticketResultsPath, URL: ticketBlobBase + ticketResultsPath},
	}
	if !reflect.DeepEqual(c.Artifacts, want) {
		t.Errorf("artifacts = %+v\nwant %+v", c.Artifacts, want)
	}
	if c.MetadataCommit != pin.MetadataRevision {
		t.Errorf("metadata_commit = %q, want %q", c.MetadataCommit, pin.MetadataRevision)
	}
	if c.Visibility != "shared" {
		t.Errorf("visibility = %q, want shared", c.Visibility)
	}
}

func TestContextTicketPrivateEmptyURLs(t *testing.T) {
	pin := ticketPin(t)
	pin.Layout = layout.PrivateLayout("/c", "/r", "/d", "o-r")
	blob := ticketRecordBlob(1, "a", "proposed", "spec: "+ticketSpecPath+"\nresults: "+ticketResultsPath+"\n")
	got, _ := readTicket(t, pin, []StatusBlob{blob}, []string{ticketSpecPath, ticketResultsPath}, TicketContextRequest{ID: 1})
	c := mustApplied(t, got)
	if c.Visibility != "private" {
		t.Errorf("visibility = %q, want private", c.Visibility)
	}
	if len(c.Artifacts) != 2 {
		t.Fatalf("artifacts = %+v, want spec and results", c.Artifacts)
	}
	for _, a := range c.Artifacts {
		if a.URL != "" {
			t.Errorf("artifact %s URL = %q, want empty in a private repository", a.Role, a.URL)
		}
	}
}

func TestContextTicketNeverReportsPlan(t *testing.T) {
	blob := ticketRecordBlob(1, "a", "proposed", "spec: "+ticketSpecPath+"\nplan: "+ticketPlanPath+"\nresults: "+ticketResultsPath+"\n")
	got, fake := readTicket(t, ticketPin(t), []StatusBlob{blob}, []string{ticketSpecPath, ticketPlanPath, ticketResultsPath}, TicketContextRequest{ID: 1})
	c := mustApplied(t, got)
	for _, a := range c.Artifacts {
		if a.Role == "plan" || a.Path == ticketPlanPath {
			t.Errorf("artifacts report the plan: %+v", a)
		}
	}
	for _, ask := range fake.artifactAsks {
		if strings.Contains(ask, ticketPlanPath) {
			t.Errorf("the plan path was read: %s", ask)
		}
	}
}

func TestContextTicketMissingArtifactOmitted(t *testing.T) {
	blob := ticketRecordBlob(1, "a", "proposed", "spec: "+ticketSpecPath+"\n")
	got, _ := readTicket(t, ticketPin(t), []StatusBlob{blob}, nil, TicketContextRequest{ID: 1, Event: TicketEventSpecWritten})
	c := mustApplied(t, got)
	if c.Event == nil || !c.Event.Happened {
		t.Errorf("event = %+v, want spec_written happened", c.Event)
	}
	if c.Artifacts == nil || len(c.Artifacts) != 0 {
		t.Errorf("artifacts = %#v, want empty non-nil", c.Artifacts)
	}
	if s := mustJSON(t, got); !strings.Contains(s, `"artifacts":[]`) {
		t.Errorf("JSON lacks \"artifacts\":[]: %s", s)
	}
}

func TestContextTicketArchivedChange(t *testing.T) {
	path := "docs/changes/archive/2026-08-16-0042-old.md"
	blob := ticketRecordBlob(42, "old", "done", "pr: '"+ticketPR+"'\n")
	blob.Location = repository.LocationArchive
	blob.Path = path
	got, _ := readTicket(t, ticketPin(t), []StatusBlob{blob}, nil, TicketContextRequest{ID: 42})
	c := mustApplied(t, got)
	if c.Change.Path != path || c.Change.Status != "done" {
		t.Errorf("change = %+v, want path %s status done", c.Change, path)
	}
}

func TestContextTicketNoEventRead(t *testing.T) {
	blob := ticketRecordBlob(1, "a", "done", "spec: "+ticketSpecPath+"\npr: '"+ticketPR+"'\nticket_provider: jira\nticket_id: 'CLOUD-1'\nticket_url: 'https://acme.atlassian.net/browse/CLOUD-1'\n")
	got, _ := readTicket(t, ticketPin(t), []StatusBlob{blob}, []string{ticketSpecPath}, TicketContextRequest{ID: 1})
	c := mustApplied(t, got)
	if c.Event != nil {
		t.Errorf("event = %+v, want nil", c.Event)
	}
	if s := mustJSON(t, got); strings.Contains(s, `"event"`) {
		t.Errorf("JSON carries an event key: %s", s)
	}
	if c.Ticket.State != TicketStateLinked || c.Facts.PRURL != ticketPR || c.Facts.Merge == nil || len(c.Artifacts) != 1 {
		t.Errorf("no-event read dropped facts: ticket=%+v facts=%+v artifacts=%+v", c.Ticket, c.Facts, c.Artifacts)
	}
}

func TestContextTicketSwitchReported(t *testing.T) {
	pin := ticketPin(t)
	pin.Config.Effective.Tickets.Auto.PROpened.Value = true
	blob := ticketRecordBlob(1, "a", "implemented", "spec: "+ticketSpecPath+"\npr: '"+ticketPR+"'\n")
	got, _ := readTicket(t, pin, []StatusBlob{blob}, nil, TicketContextRequest{ID: 1, Event: TicketEventPROpened})
	if c := mustApplied(t, got); !c.Event.Switch {
		t.Errorf("pr_opened switch = false, want true")
	}
	got, _ = readTicket(t, pin, []StatusBlob{blob}, nil, TicketContextRequest{ID: 1, Event: TicketEventSpecWritten})
	if c := mustApplied(t, got); c.Event.Switch {
		t.Errorf("spec_written switch = true, want false")
	}
}

func TestContextTicketRefusals(t *testing.T) {
	cases := []struct {
		name       string
		corpus     []StatusBlob
		id         int
		wantResult Result
		wantReason string
	}{
		{"missing id", []StatusBlob{ticketRecordBlob(1, "a", "proposed", "")}, 0, ResultInvalidInput, ReasonTicketMissingID},
		{"absent", []StatusBlob{ticketRecordBlob(1, "a", "proposed", "")}, 404, ResultInvalidInput, ReasonTicketNotFound},
		{"ambiguous", []StatusBlob{ticketRecordBlob(7, "one", "proposed", ""), ticketRecordBlob(7, "two", "proposed", "")}, 7, ResultInvalidState, ReasonContextAmbiguousID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := readTicket(t, ticketPin(t), tc.corpus, nil, TicketContextRequest{ID: tc.id})
			if got.Result != tc.wantResult || got.Reason != tc.wantReason || got.Context != nil {
				t.Errorf("got %q/%q context=%v, want %q/%q and no context", got.Result, got.Reason, got.Context, tc.wantResult, tc.wantReason)
			}
			if got.Operation != OperationContextTicket {
				t.Errorf("operation = %q", got.Operation)
			}
		})
	}
}

func TestContextTicketUnknownEventRefusedBeforeRead(t *testing.T) {
	for _, event := range []string{"PR_OPENED", "pr-opened", " pr_opened", "change_created"} {
		t.Run(event, func(t *testing.T) {
			got, fake := readTicket(t, ticketPin(t), []StatusBlob{ticketRecordBlob(1, "a", "implemented", "pr: '"+ticketPR+"'\n")}, nil,
				TicketContextRequest{ID: 1, Event: event})
			if got.Result != ResultInvalidInput || got.Reason != ReasonTicketUnknownEvent || got.Context != nil {
				t.Errorf("got %q/%q, want invalid-input/unknown-event", got.Result, got.Reason)
			}
			if fake.pinCount != 0 {
				t.Errorf("PinContext called %d times, want 0", fake.pinCount)
			}
		})
	}
}

func TestContextTicketNoNullAndNoBody(t *testing.T) {
	ticketURL := "https://acme.atlassian.net/browse/CLOUD-5"
	blob := ticketRecordBlob(5, "five", "implemented", "spec: "+ticketSpecPath+"\npr: '"+ticketPR+"'\nticket_provider: jira\nticket_id: 'CLOUD-5'\nticket_url: '"+ticketURL+"'\n")
	got, _ := readTicket(t, ticketPin(t), []StatusBlob{blob}, []string{ticketSpecPath}, TicketContextRequest{ID: 5, Event: TicketEventPROpened})
	mustApplied(t, got)
	if s := mustJSON(t, got); strings.Contains(s, "null") {
		t.Errorf("JSON contains null: %s", s)
	}
	h := got.HumanText()
	for _, banned := range []string{"Change 5", "Body of", ticketURL, ticketPR} {
		if strings.Contains(h, banned) {
			t.Errorf("HumanText %q contains %q", h, banned)
		}
	}
	if !strings.Contains(h, "0005 five") || !strings.Contains(h, "ticket linked") || !strings.Contains(h, "event pr_opened happened") {
		t.Errorf("HumanText %q lacks identity or verdicts", h)
	}
}
