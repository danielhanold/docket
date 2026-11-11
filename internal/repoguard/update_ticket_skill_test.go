package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestUpdateTicketSkillIsInline pins docket-update-ticket as an inline skill whose
// description names its ask in conversational words, disclaims creating or
// linking, and uses the ticket vocabulary only; and checks that every
// reference SKILL.md routes to exists.
func TestUpdateTicketSkillIsInline(t *testing.T) {
	const name = "docket-update-ticket"
	assertInlineSkill(t, name, []string{
		"post an update or comment about a change to its ticket",
		"or when a docket workflow invokes it after an event whose `tickets.auto` switch is on",
		"not for creating or linking a ticket",
	})
	body := readRepoFile(t, "skills/"+name+"/SKILL.md")
	desc := strings.ToLower(collapseWS(frontmatterValue(body, "description")))
	for _, word := range []string{"issue", "work item"} {
		if strings.Contains(desc, word) {
			t.Errorf("%s description uses %q; the item is a ticket: %q", name, word, desc)
		}
	}
	// Every reference SKILL.md routes to must exist: a missing one leaves a
	// blocking read with nothing to read. Each visibility needs its own
	// delivery reference, so both routes must be present.
	routed := map[string]bool{}
	for _, m := range referenceRoute.FindAllStringSubmatch(body, -1) {
		routed[m[1]] = true
	}
	for _, ref := range []string{"shared.md", "private.md"} {
		if !routed[ref] {
			t.Errorf("%s routes to no references/%s; each visibility needs its delivery reference", name, ref)
		}
	}
	for ref := range routed {
		p := filepath.Join(guardRoot(t), "skills", name, "references", ref)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s routes to references/%s, which cannot be read: %v", name, ref, err)
		}
	}
}

// referenceRoute matches a route from a skill body to one of its references.
var referenceRoute = regexp.MustCompile(`references/([a-z0-9-]+\.md)`)

// typedMention matches an at-sign directly before a name: the plain-text
// stand-in for a mention, which notifies nobody.
var typedMention = regexp.MustCompile(`@\p{L}`)

// TestUpdateTicketSkillExamplesUseNoTypedMention keeps the skill's example
// comments from teaching the typed mention its Mentions rule forbids: no bold
// at-mention anywhere, and no at-sign before a name on any example
// (blockquote) line.
func TestUpdateTicketSkillExamplesUseNoTypedMention(t *testing.T) {
	body := readRepoFile(t, "skills/docket-update-ticket/SKILL.md")
	if strings.Contains(body, "**@") {
		t.Errorf("skills/docket-update-ticket/SKILL.md contains a bold at-mention (`**@`); an example mention must be a placeholder for the connector's real mention")
	}
	for i, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		if typedMention.MatchString(line) {
			t.Errorf("skills/docket-update-ticket/SKILL.md example line %d types a mention as plain text: %q", i+1, line)
		}
	}
}
