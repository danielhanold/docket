package repoguard

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// The convention references tell an agent where a private repository's
// parent-facing rules live and how they reach a session: promoted lessons land
// in `.git/dckt/AGENTS.md`, and user-level triggers (never a working-tree rule
// file) deliver that file. These rows keep both statements in their sections.
var privateInstructionsDocContracts = []docSectionContract{
	{change: "private_instructions_delivery", file: "skills/docket-convention/references/agent-layer.md",
		section: "## Repository dispatch blocks: agent_harnesses", terminator: "## Launch posture",
		present: []string{
			"dckt instructions --hook claude --section dispatch",
			"dckt instructions --hook cursor",
			"dckt-instructions.js",
			"dckt:private-instructions",
		},
		absent: []string{".cursor/rules/dckt-dispatch.mdc"}},
}

// privatePromotionSection bounds the learnings promotion rule.
var privatePromotionSection = docSectionContract{
	change: "private_instructions_promotion", file: "skills/docket-convention/references/learnings.md",
	section: "## Promotion — the shrink valve", terminator: "## Off switch",
}

var paragraphBreak = regexp.MustCompile(`\n[ \t]*\n`)

// scanPrivatePromotion requires one paragraph of the promotion section to name
// both the private file and a private repository, so the destination stays
// bound to the repository kind it applies to.
func scanPrivatePromotion(content string, c docSectionContract) []string {
	si := strings.Index(content, c.section)
	if si < 0 {
		return []string{fmt.Sprintf("%s: missing section heading %q", c.file, c.section)}
	}
	rest := content[si+len(c.section):]
	ti := strings.Index(rest, c.terminator)
	if ti < 0 {
		return []string{fmt.Sprintf("%s: section %q missing terminator %q", c.file, c.section, c.terminator)}
	}
	for _, para := range paragraphBreak.Split(rest[:ti], -1) {
		p := collapseWS(para)
		if strings.Contains(p, ".git/dckt/AGENTS.md") && strings.Contains(p, "private repository") {
			return nil
		}
	}
	return []string{fmt.Sprintf("%s §%q: no paragraph binds `.git/dckt/AGENTS.md` to a private repository", c.file, c.section)}
}

func TestPrivateInstructionsDocContracts(t *testing.T) {
	root := guardRoot(t)
	for _, c := range privateInstructionsDocContracts {
		for _, v := range scanDocSection(readMaintained(t, root, c.file), c) {
			t.Errorf("[%s] %s", c.change, v)
		}
	}
	for _, v := range scanPrivatePromotion(readMaintained(t, root, privatePromotionSection.file), privatePromotionSection) {
		t.Errorf("[%s] %s", privatePromotionSection.change, v)
	}

	t.Run("non_vacuity", func(t *testing.T) {
		c := privatePromotionSection
		ok := c.section + "\nIn a private repository it lands in\n`.git/dckt/AGENTS.md`.\n" + c.terminator
		if got := scanPrivatePromotion(ok, c); len(got) != 0 {
			t.Errorf("a bound paragraph was rejected: %v", got)
		}
		split := c.section + "\nIn a private repository.\n\nIt lands in `.git/dckt/AGENTS.md`.\n" + c.terminator
		if got := scanPrivatePromotion(split, c); len(got) != 1 {
			t.Errorf("an unbound pair across paragraphs was accepted: %v", got)
		}
		if got := scanPrivatePromotion(c.section+"\nIn a private repository `.git/dckt/AGENTS.md`.\n", c); len(got) != 1 {
			t.Errorf("a missing terminator was accepted: %v", got)
		}
	})
}
