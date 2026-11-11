package repoguard

import (
	"strings"
	"testing"
)

// autoTicketContract pins the prose of the automatic ticket-update path in one
// file: every present phrase must appear and every absent phrase must not.
// Matching is whitespace-collapsed and file-wide, so a hard-wrapped phrase
// still matches.
type autoTicketContract struct {
	file    string
	present []string
	absent  []string
}

// autoTicketContracts is the automatic ticket-update path: docket-update-ticket
// runs when a workflow invokes it after an event whose tickets.auto switch is
// on, and its references/automatic.md replaces every point where it would ask
// or wait. The convention's *Automatic ticket updates* shared definition and
// the five workflow sites are pinned here too; which files hold a site is
// derived and checked by TestTicketAutoUpdateSites in internal/app.
var autoTicketContracts = []autoTicketContract{
	{file: "skills/docket-update-ticket/SKILL.md",
		present: []string{
			"This skill runs inline, in the session or workflow agent that invokes it",
			"or when a docket workflow invokes it after an event whose `tickets.auto` switch is on",
			"read `references/automatic.md` now (blocking)",
			"When a docket workflow invoked this skill, that workflow has already run the startup check: run none here.",
		},
		absent: []string{
			"Nothing here runs on its own after an event",
			"and only when the person asks: it posts",
		}},
	{file: "skills/docket-update-ticket/references/automatic.md",
		present: []string{
			"The caller supplies the change and the event.",
			"Never ask and never wait.",
			"post nothing and report one line",
			"when `visibility` is anything other than `shared`",
			"automatic updates are not available in a private repository",
			"when no connector for `ticket.provider` is available in this session",
			"an uncertain outcome is reported as uncertain",
			"**`event.switch` is `false`:** post nothing and say nothing, whatever the caller directed.",
			"`ticket.state` is `not-linked`:** post nothing and say nothing",
			"Never retry, and never touch the calling run",
			"no halt, no blocked marker, no changed disposition, and no metadata write",
			"A duplicate comment is acceptable and is deleted by hand.",
			"Never load `references/private.md` on this path.",
			"**No startup check of its own.**",
			"take `metadata_worktree_path` from the caller's `repository.prepare` context",
			"A `refused` or `error` answer from a docket read on this path is never a stop: post nothing, report one line (`Not posted to #<id> (<event>): <reason>.`), and return to the workflow.",
		}},
	{file: "skills/docket-convention/SKILL.md",
		present: []string{
			"### Automatic ticket updates (shared definition)",
			"with `--id <id> --event <event> --json`",
			"when `context.event.switch` is `false` or `context.ticket.state` is `not-linked`",
			"no post, no prompt, no report line",
			"Ticket update for #<id> not checked: <reason>",
			"never changes a run's disposition, writes a halt or blocked marker, or stops a later step",
			"A replayed success posts like a first success",
			"when a stack root lands only the root's ticket is updated",
		}},
	{file: "skills/docket-groom-next/SKILL.md",
		present: []string{
			"the switch is the consent, so nothing is asked",
			"Kill, defer, revise, and re-enable never post.",
		}},
	{file: "skills/docket-auto-groom/SKILL.md",
		present: []string{
			"before step 1 selects the next stub",
			"An abstain never posts.",
		}},
	{file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"(a retry's earlier applied outcome included)",
			"The run's disposition is `advanced` whatever the update's outcome",
		}},
	{file: "skills/docket-implement-next/references/results-artifact.md",
		present: []string{
			"After `change.complete` returns `completed` or `already-completed`",
		}},
	{file: "skills/docket-finalize-change/SKILL.md",
		present: []string{
			"never for a `root-archived` result's `carried_ids`",
			"never after `contended`, `blocked`, or `unknown`",
			"`already-closed-out` for a change this run closed out",
		}},
	// The tickets concepts page: its sections, both reasons ticket creation is
	// on request, the no-setting invariant, the three ways a change gets a
	// ticket, the four switches, and its index lines. These keep the page from
	// losing a part; its accuracy against the skills is checked by reading.
	{file: "docs/concepts/tickets.md",
		present: []string{
			"# Tickets",
			"## The problem it solves",
			"## The moving parts",
			"## The invariants",
			"## Why docket never creates a ticket by itself",
			"### Creating a ticket is a person's action",
			"### There is no one right moment to create it",
			"## Decided in",
			"A ticket is created only when a person asks and approves the draft. No setting changes that.",
			"link an existing ticket to a change",
			"create a ticket from a change",
			"make a change from a ticket",
			"`tickets.auto.spec_written`",
			"`tickets.auto.pr_opened`",
			"`tickets.auto.pr_merged`",
			"`tickets.auto.results_completed`",
			"retry uploads after a timeout",
			"agents report on tickets, and people create them",
			"an automatic update posts nothing and reports one line",
		},
		absent: []string{"change_created", "was once planned", "dropped"}},
	{file: "docs/concepts/README.md",
		present: []string{"[Tickets](./tickets.md)"}},
	{file: "docs/README.md",
		present: []string{"[Tickets](concepts/tickets.md)"}},
	{file: "docs/reference/config-keys.md",
		present: []string{
			"Creating a ticket has no switch: it is always on request",
			"../concepts/tickets.md#why-docket-never-creates-a-ticket-by-itself",
		}},
	{file: "docs/reference/glossary.md",
		present: []string{
			"Docket never creates a ticket by itself",
			"../concepts/tickets.md#why-docket-never-creates-a-ticket-by-itself",
			"or by itself after an event whose switch is on",
			"An automatic update in a private repository posts nothing and reports one line.",
		},
		absent: []string{"to a change's linked ticket when you ask. It posts"}},
	{file: "docs/reference/skills-and-agents.md",
		present: []string{"when you ask, or automatically after an event whose `tickets.auto` switch is on"},
		absent:  []string{"linked ticket when you ask (spec written"}},
}

// autoTicketViolations returns one message per missing present phrase and per
// found absent phrase in body, comparing whitespace-collapsed text.
func autoTicketViolations(c autoTicketContract, body string) []string {
	flat := collapseWS(body)
	var out []string
	for _, p := range c.present {
		if !strings.Contains(flat, collapseWS(p)) {
			out = append(out, c.file+": missing required phrase "+quoteQ(p))
		}
	}
	for _, a := range c.absent {
		if strings.Contains(flat, collapseWS(a)) {
			out = append(out, c.file+": contains forbidden phrase "+quoteQ(a))
		}
	}
	return out
}

func quoteQ(s string) string { return "\"" + s + "\"" }

// TestAutoTicketUpdateContracts checks every autoTicketContracts row. An
// unreadable file fails closed (readRepoFile is fatal), and a population floor
// keeps the table from silently shrinking.
func TestAutoTicketUpdateContracts(t *testing.T) {
	t.Run("non_vacuity", func(t *testing.T) {
		c := autoTicketContract{file: "synthetic",
			present: []string{"must be here", "split across lines"},
			absent:  []string{"must not be here"}}
		if v := autoTicketViolations(c, "must be here; split across\n   lines"); len(v) != 0 {
			t.Fatalf("a present phrase split by a newline must still match; got %v", v)
		}
		if v := autoTicketViolations(c, "split across lines only"); len(v) != 1 || !strings.Contains(v[0], "missing") {
			t.Fatalf("a missing present phrase must be detected; got %v", v)
		}
		if v := autoTicketViolations(c, "must be here, split across lines, must not\nbe here"); len(v) != 1 || !strings.Contains(v[0], "forbidden") {
			t.Fatalf("a present forbidden phrase must be detected; got %v", v)
		}
	})

	const floor = 72
	checks := 0
	for _, c := range autoTicketContracts {
		body := readRepoFile(t, c.file)
		checks += len(c.present) + len(c.absent)
		for _, v := range autoTicketViolations(c, body) {
			t.Error(v)
		}
	}
	if checks < floor {
		t.Fatalf("autoTicketContracts ran %d checks, below the floor of %d", checks, floor)
	}
}
