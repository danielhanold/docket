package repoguard

import (
	"os"
	"path/filepath"
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
