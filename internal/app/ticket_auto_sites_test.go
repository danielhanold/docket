package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The automatic ticket-update sites. A tickets.auto.<event> switch has an
// effect only where a workflow skill invokes docket-update-ticket for that
// event, so the sites are derived from a scan of every Markdown file under
// skills/ — never hand-listed — and checked against ticketEvents, which is
// unexported and so is checked here, beside the skill test for the ticket read.
//
// A block (a blank-line-separated paragraph, whitespace collapsed so a
// hard-wrapped paragraph is one string) is an INVOCATION when it puts the
// backticked `docket-update-ticket` within 40 characters of an un-negated
// invoke verb, in either order, so a passive "is invoked" counts and a
// prohibition ("never invokes") does not. An invocation that names an event
// token (tickets.auto.<token> or --event <token>) is a SITE. The convention's
// shared definition invokes the skill with no token; it is the only such
// invocation allowed.

// ticketSiteFiles maps each repo-relative site file to the event it posts.
var ticketSiteFiles = map[string]string{
	"skills/docket-groom-next/SKILL.md":                           TicketEventSpecWritten,
	"skills/docket-auto-groom/SKILL.md":                           TicketEventSpecWritten,
	"skills/docket-implement-next/SKILL.md":                       TicketEventPROpened,
	"skills/docket-implement-next/references/results-artifact.md": TicketEventResultsCompleted,
	"skills/docket-finalize-change/SKILL.md":                      TicketEventPRMerged,
}

// ticketDefinitionFile holds the shared definition every site follows.
const ticketDefinitionFile = "skills/docket-convention/SKILL.md"

// ticketInvocation is one block that invokes docket-update-ticket.
type ticketInvocation struct {
	rel, text string
	tokens    []string
	directed  bool
}

var (
	ticketBlockSplitRe = regexp.MustCompile(`\n[ \t]*\n`)
	// ticketInvokeVerbRe is the invoke verb. Each occurrence is tested on its
	// own, so a negated verb earlier in a block never hides a later one.
	ticketInvokeVerbRe = regexp.MustCompile(`(?i)\binvok(?:e|es|ed|ing)\b`)
	// The backticked name within 40 characters of the verb, in either order:
	// after it ("invoke `docket-update-ticket`") or before it
	// ("`docket-update-ticket` is invoked").
	ticketNameAfterRe  = regexp.MustCompile("^[^.]{0,40}`docket-update-ticket`")
	ticketNameBeforeRe = regexp.MustCompile("`docket-update-ticket`[^.]{0,40}$")
	// ticketNegatedVerbRe is the negation shape of negatedInvokeRe in
	// internal/repoguard/skill_handoff_sites_test.go (a word ending in "not",
	// "never", or an n't contraction, within two words of the verb), anchored
	// so the negated verb is the matched verb itself and not an earlier one.
	ticketNegatedVerbRe = regexp.MustCompile(`(?i)(?:\b\w*not|\bnever|\w+n['’]t)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)$`)
	ticketTokenRe       = regexp.MustCompile(`tickets\.auto\.([a-z0-9_]+)|--event\s+([a-z0-9_]+)`)
)

const ticketDirectedMarker = "DIRECTED to:"

// ticketNegationLookback is how far before a match a negation may start.
const ticketNegationLookback = 30

// scanTicketInvocations returns every invocation block in files, a map of
// repo-relative path to content, sorted by path then position.
func scanTicketInvocations(files map[string]string) []ticketInvocation {
	rels := make([]string, 0, len(files))
	for rel := range files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	var out []ticketInvocation
	for _, rel := range rels {
		for _, raw := range ticketBlockSplitRe.Split(files[rel], -1) {
			block := strings.Join(strings.Fields(raw), " ")
			if !hasUnnegatedTicketInvoke(block) {
				continue
			}
			var tokens []string
			for _, m := range ticketTokenRe.FindAllStringSubmatch(block, -1) {
				tok := m[1] + m[2]
				if !slices.Contains(tokens, tok) {
					tokens = append(tokens, tok)
				}
			}
			out = append(out, ticketInvocation{
				rel:      rel,
				text:     block,
				tokens:   tokens,
				directed: strings.Contains(block, ticketDirectedMarker),
			})
		}
	}
	return out
}

// hasUnnegatedTicketInvoke reports whether block holds an invoke verb with
// the backticked name near it, in either order, whose own verb is not negated.
func hasUnnegatedTicketInvoke(block string) bool {
	for _, loc := range ticketInvokeVerbRe.FindAllStringIndex(block, -1) {
		if !ticketNameAfterRe.MatchString(block[loc[1]:]) && !ticketNameBeforeRe.MatchString(block[:loc[0]]) {
			continue
		}
		if !ticketNegatedVerbRe.MatchString(block[max(loc[0]-ticketNegationLookback, 0):loc[1]]) {
			return true
		}
	}
	return false
}

func readSkillMarkdown(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "skills")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(p) != ".md" {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files["skills/"+filepath.ToSlash(rel)] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) < 20 {
		t.Fatalf("population floor: only %d skill Markdown files found under %s", len(files), root)
	}
	return files
}

func TestTicketAutoUpdateSites(t *testing.T) {
	t.Run("non_vacuity", func(t *testing.T) {
		passive := "**Ticket update.** Then `docket-update-ticket` is invoked **DIRECTED to:** post the update for `tickets.auto.pr_opened`."
		wrapped := "**Ticket update.** Then `docket-update-ticket`\n   is invoked **DIRECTED to:** post the\nupdate for `tickets.auto.pr_opened`."
		got := scanTicketInvocations(map[string]string{
			"a.md": "Intro.\n\n" + passive + "\n\nOutro.",
			"b.md": wrapped,
			"c.md": "Housekeeping: the sweep never invokes `docket-update-ticket`.",
			"d.md": "Invoke `docket-update-ticket` **DIRECTED to:** for `tickets.auto.spec_written` and `tickets.auto.pr_opened`.",
			"e.md": "Run it with `--event <event>`, then invoke `docket-update-ticket`.",
		})
		byRel := map[string]ticketInvocation{}
		for _, inv := range got {
			byRel[inv.rel] = inv
		}
		for _, rel := range []string{"a.md", "b.md"} {
			inv, ok := byRel[rel]
			if !ok || !inv.directed || !slices.Equal(inv.tokens, []string{TicketEventPROpened}) {
				t.Errorf("%s: a passive invocation (hard-wrapped in b.md) must be a directed invocation with token pr_opened; got %+v (found %v)", rel, inv, ok)
			}
		}
		if inv, ok := byRel["c.md"]; ok {
			t.Errorf("a prohibition must not be an invocation; got %+v", inv)
		}
		if inv := byRel["d.md"]; !slices.Equal(inv.tokens, []string{TicketEventSpecWritten, TicketEventPROpened}) {
			t.Errorf("two named switches must yield two tokens; got %+v", inv)
		}
		if inv, ok := byRel["e.md"]; !ok || len(inv.tokens) != 0 {
			t.Errorf("`--event <event>` must be an invocation with no token; got %+v (found %v)", inv, ok)
		}
		// A negation of an earlier verb must not negate a later invocation.
		mixed := scanTicketInvocations(map[string]string{"f.md": "Do not invoke it twice; invoke `docket-update-ticket` once."})
		if len(mixed) != 1 {
			t.Errorf("an earlier negated verb must not negate a later invocation; got %+v", mixed)
		}
	})

	invocations := scanTicketInvocations(readSkillMarkdown(t))

	var sites, bare []ticketInvocation
	for _, inv := range invocations {
		if len(inv.tokens) > 0 {
			sites = append(sites, inv)
		} else {
			bare = append(bare, inv)
		}
	}

	perFile := map[string]int{}
	covered := map[string]bool{}
	for _, s := range sites {
		perFile[s.rel]++
		want, isSiteFile := ticketSiteFiles[s.rel]
		if !isSiteFile {
			t.Errorf("%s: an automatic ticket-update site outside the five site files: %q", s.rel, s.text)
		}
		if !s.directed {
			t.Errorf("%s: the site does not carry %q: %q", s.rel, ticketDirectedMarker, s.text)
		}
		if len(s.tokens) != 1 {
			t.Errorf("%s: the site names %d event tokens %v, want exactly one", s.rel, len(s.tokens), s.tokens)
			continue
		}
		tok := s.tokens[0]
		if !slices.Contains(ticketEvents, tok) {
			t.Errorf("%s: the site names %q, which is not a context.ticket event %v", s.rel, tok, ticketEvents)
		}
		if isSiteFile && tok != want {
			t.Errorf("%s: the site names %q, want %q", s.rel, tok, want)
		}
		covered[tok] = true
	}
	for rel := range ticketSiteFiles {
		if perFile[rel] != 1 {
			t.Errorf("%s: %d automatic ticket-update sites, want exactly one", rel, perFile[rel])
		}
	}
	for _, ev := range ticketEvents {
		if !covered[ev] {
			t.Errorf("event %q has no automatic ticket-update site", ev)
		}
	}
	if len(bare) != 1 || bare[0].rel != ticketDefinitionFile {
		t.Errorf("want exactly one invocation with no event token, the shared definition in %s; got %d: %+v", ticketDefinitionFile, len(bare), bare)
	}
	if len(sites) != len(ticketSiteFiles) {
		t.Fatalf("population: %d automatic ticket-update sites, want exactly %d", len(sites), len(ticketSiteFiles))
	}
}
