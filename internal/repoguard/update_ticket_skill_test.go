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
// linking, and uses the ticket vocabulary only; and checks that the reference
// SKILL.md routes to exists.
func TestUpdateTicketSkillIsInline(t *testing.T) {
	const name = "docket-update-ticket"
	assertInlineSkill(t, name, []string{
		"post an update or comment about a change to its ticket",
		"not for creating or linking a ticket",
	})
	body := readRepoFile(t, "skills/"+name+"/SKILL.md")
	desc := strings.ToLower(collapseWS(frontmatterValue(body, "description")))
	for _, word := range []string{"issue", "work item"} {
		if strings.Contains(desc, word) {
			t.Errorf("%s description uses %q; the item is a ticket: %q", name, word, desc)
		}
	}
	// SKILL.md routes shared delivery to this file; a missing one leaves a
	// blocking read with nothing to read.
	p := filepath.Join(guardRoot(t), "skills", name, "references", "shared.md")
	if _, err := os.Stat(p); err != nil {
		t.Errorf("%s routes to references/shared.md, which cannot be read: %v", name, err)
	}
}

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
