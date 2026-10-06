// This file is package harness_test so its rule-population helper is shared
// with cross_harness_test.go, which must live outside package harness (it
// plans with all four adapter packages, each of which imports harness).
package harness_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
)

// rulePopulation is every line of parent-facing rule text docket renders into a
// dispatch surface: the shared dispatch interior over the real run tracker,
// plus the Codex root-entry clause. A line is a trimmed line of at least 20
// characters, so headings and prose both count while blank and bracket-only
// lines do not. The population is computed from the producers, never written
// down, so a rule added to either producer joins it automatically.
func rulePopulation(t *testing.T) []string {
	t.Helper()
	c, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	rt, err := harness.RunTracker(c)
	if err != nil {
		t.Fatalf("RunTracker: %v", err)
	}
	var lines []string
	for _, src := range []string{harness.DispatchInterior(rt), harness.CodexRootEntryClause} {
		for _, line := range strings.Split(src, "\n") {
			if line = strings.TrimSpace(line); len(line) >= 20 {
				lines = append(lines, line)
			}
		}
	}
	// Non-vacuity: a population that shrank to nothing would let every
	// rendering pass. It must be substantial and must include the routing
	// rule's first line, named through the producer rather than respelled.
	first := strings.TrimSpace(strings.SplitN(harness.DispatchPreambleForTest, "\n", 2)[0])
	if len(lines) < 10 {
		t.Fatalf("the rule population holds %d lines; the guard would be vacuous", len(lines))
	}
	found := false
	for _, l := range lines {
		if l == first {
			found = true
		}
	}
	if !found {
		t.Fatalf("the rule population lacks the routing rule's first line %q", first)
	}
	return lines
}

// assertNoRuleText fails when rendered carries any rule-population line.
func assertNoRuleText(t *testing.T, label, rendered string, population []string) {
	t.Helper()
	for _, line := range population {
		if strings.Contains(rendered, line) {
			t.Errorf("%s carries rule text %q", label, line)
		}
	}
}

// renderedTriggers renders every trigger byte through the producer that writes
// it, keyed by a label: the three hook commands, the hooks file install creates
// for each dialect, the OpenCode plugin, and the Codex pointer block as the
// installer would insert it.
func renderedTriggers(t *testing.T) map[string]string {
	t.Helper()
	claudeT := harness.ClaudeHookTarget("/h/.claude/settings.json")
	cursorT := harness.CursorHookTarget("/h/.cursor/hooks.json")

	doc, err := document.Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	var p document.PatchSet
	p.InsertBlock(harness.PointerBlockName, harness.PointerBlockAnnotation, harness.PointerInterior, document.AtDocumentStart)
	pointer, err := doc.Apply(p)
	if err != nil {
		t.Fatalf("rendering the pointer block: %v", err)
	}
	if !strings.Contains(string(pointer), "## Private repository instructions") {
		t.Fatalf("the rendered pointer block lost its interior:\n%s", pointer)
	}

	return map[string]string{
		"claude dispatch hook command": harness.ClaudeDispatchHookCommand,
		"claude lessons hook command":  harness.ClaudeLessonsHookCommand,
		"cursor hook command":          harness.CursorHookCommand,
		"claude hooks file":            string(install.NewHookFileBytes(claudeT.HookDialect, claudeT.HookCommands)),
		"cursor hooks file":            string(install.NewHookFileBytes(cursorT.HookDialect, cursorT.HookCommands)),
		"opencode plugin":              string(harness.OpenCodePluginTarget("/h/.config/opencode/plugins/x.js").Content),
		"codex pointer block":          string(pointer),
	}
}

// TestTriggersCarryNoRuleTextAndNoDocket is the content-free property of every
// user-level trigger: each one only runs `dckt instructions`, so none may carry
// a line of the rules it delivers, and none may say "docket" (a trigger lands
// on every machine, private repository or not).
func TestTriggersCarryNoRuleTextAndNoDocket(t *testing.T) {
	population := rulePopulation(t)
	rendered := renderedTriggers(t)
	for label, body := range rendered {
		if body == "" {
			t.Errorf("%s rendered empty; the guard would be vacuous", label)
		}
		if strings.Contains(strings.ToLower(body), "docket") {
			t.Errorf("%s says docket:\n%s", label, body)
		}
		assertNoRuleText(t, label, body, population)
	}
}

// TestTriggerTargets pins each trigger target's shape: the kind, the dialect,
// the commands in order, the content, and the shared role.
func TestTriggerTargets(t *testing.T) {
	c := harness.ClaudeHookTarget("/h/.claude/settings.json")
	if c.Kind != install.KindHookEntries || c.HookDialect != install.HookDialectClaude || c.Role != harness.TriggerRole ||
		len(c.HookCommands) != 2 || c.HookCommands[0] != harness.ClaudeDispatchHookCommand || c.HookCommands[1] != harness.ClaudeLessonsHookCommand {
		t.Errorf("claude hook target = %+v", c)
	}
	u := harness.CursorHookTarget("/h/.cursor/hooks.json")
	if u.Kind != install.KindHookEntries || u.HookDialect != install.HookDialectCursor || u.Role != harness.TriggerRole ||
		len(u.HookCommands) != 1 || u.HookCommands[0] != harness.CursorHookCommand {
		t.Errorf("cursor hook target = %+v", u)
	}
	o := harness.OpenCodePluginTarget("/h/p.js")
	if o.Kind != install.KindFile || string(o.Content) != harness.OpenCodePlugin || o.Role != harness.TriggerRole {
		t.Errorf("opencode plugin target = %+v", o)
	}
	x := harness.CodexPointerTarget("/h/.codex/AGENTS.md")
	if x.Kind != install.KindManagedBlock || x.BlockName != harness.PointerBlockName || x.Annotation != harness.PointerBlockAnnotation ||
		string(x.Content) != harness.PointerInterior || x.Role != harness.TriggerRole {
		t.Errorf("codex pointer target = %+v", x)
	}
	if filepath.Base(harness.OpenCodePluginFile) != harness.OpenCodePluginFile {
		t.Errorf("OpenCodePluginFile %q is not a bare file name", harness.OpenCodePluginFile)
	}
}
