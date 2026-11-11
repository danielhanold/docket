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
// or wait.
var autoTicketContracts = []autoTicketContract{
	{file: "skills/docket-update-ticket/SKILL.md",
		present: []string{
			"This skill runs inline, in the session or workflow agent that invokes it",
			"or when a docket workflow invokes it after an event whose `tickets.auto` switch is on",
			"read `references/automatic.md` now (blocking)",
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
			"`ticket.state` is `not-linked`:** post nothing and say nothing",
			"Never retry, and never touch the calling run",
			"no halt, no blocked marker, no changed disposition, and no metadata write",
			"A duplicate comment is acceptable and is deleted by hand.",
			"Never load `references/private.md` on this path.",
		}},
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

	const floor = 17
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
