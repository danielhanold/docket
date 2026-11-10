package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdateTicketSkillNamesEveryReadToken: every event, not-happened reason, and
// ticket state context.ticket can report appears in the skill in backticks, so a
// token renamed in Go reddens here instead of leaving the skill unable to act on it.
func TestUpdateTicketSkillNamesEveryReadToken(t *testing.T) {
	path := filepath.Join("..", "..", "skills", "docket-update-ticket", "SKILL.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(raw)
	var tokens []string
	tokens = append(tokens, ticketEvents...)
	tokens = append(tokens, ticketEventReasons...)
	tokens = append(tokens, ticketStates...)
	if len(tokens) != 13 {
		t.Fatalf("context.ticket reports %d tokens, want 13 (4 events, 6 reasons, 3 states); update this guard with the producer", len(tokens))
	}
	for _, tok := range tokens {
		if !strings.Contains(body, "`"+tok+"`") {
			t.Errorf("skills/docket-update-ticket/SKILL.md never names `%s`, which context.ticket can report", tok)
		}
	}
}
