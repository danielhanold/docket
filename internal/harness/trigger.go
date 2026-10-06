package harness

import "github.com/danielhanold/docket/internal/install"

// User-level triggers: content-free by design — no rule text, no "docket".
//
// A private repository keeps its parent-facing rules in its own git directory,
// and each harness reaches them through one trigger installed once per machine.
// A trigger only runs `dckt instructions`, which prints nothing outside a
// private repository, so a trigger can sit on every machine without carrying a
// rule that could drift or a name that could leak.
// TestTriggersCarryNoRuleTextAndNoDocket renders every byte below through its
// producer and enforces both properties.
const (
	// TriggerRole is the installed-state role every trigger target carries.
	TriggerRole = "trigger"

	// The Claude Code SessionStart commands, split by section so each output
	// fits Claude Code's per-hook character cap.
	ClaudeDispatchHookCommand = "dckt instructions --hook claude --section dispatch"
	ClaudeLessonsHookCommand  = "dckt instructions --hook claude --section lessons"

	// CursorHookCommand is Cursor's single sessionStart command.
	CursorHookCommand = "dckt instructions --hook cursor"

	// OpenCodePluginFile is the plugin's base name under opencode's plugins
	// directory, and OpenCodePlugin its exact bytes.
	OpenCodePluginFile = "dckt-instructions.js"
	OpenCodePlugin     = "// Adds a private repository's instructions to the system prompt.\n" +
		"// `dckt instructions` prints nothing outside a private repository.\n" +
		"export const DcktInstructions = async ({ $, directory }) => ({\n" +
		"  \"experimental.chat.system.transform\": async (_input, output) => {\n" +
		"    const text = await $`dckt instructions`.cwd(directory).quiet().nothrow().text()\n" +
		"    if (text.trim()) output.system.push(text)\n" +
		"  },\n" +
		"})\n"

	// The Codex pointer: a managed block in the neutral dckt: namespace asking
	// the model to run `dckt instructions` once per session (best effort).
	PointerBlockName       = "dckt:private-instructions"
	PointerBlockAnnotation = "managed — do not hand-edit"
	PointerInterior        = "## Private repository instructions\n\n" +
		"At the start of a session inside a git repository, run `dckt instructions` once.\n" +
		"If it prints anything, treat that output as this repository's own AGENTS.md and\n" +
		"follow it for the rest of the session. If it prints nothing, ignore this section.\n"
)

// ClaudeHookTarget is the Claude Code trigger: both SessionStart commands as
// one hook-entries target in settings.json (one target per hooks file).
func ClaudeHookTarget(settingsPath string) install.Target {
	return install.Target{Path: settingsPath, Kind: install.KindHookEntries, HookDialect: install.HookDialectClaude,
		HookCommands: []string{ClaudeDispatchHookCommand, ClaudeLessonsHookCommand}, Role: TriggerRole}
}

// CursorHookTarget is the Cursor trigger: the sessionStart command in hooks.json.
func CursorHookTarget(hooksPath string) install.Target {
	return install.Target{Path: hooksPath, Kind: install.KindHookEntries, HookDialect: install.HookDialectCursor,
		HookCommands: []string{CursorHookCommand}, Role: TriggerRole}
}

// OpenCodePluginTarget is the OpenCode trigger: a wholly owned plugin file.
func OpenCodePluginTarget(pluginPath string) install.Target {
	return install.Target{Path: pluginPath, Kind: install.KindFile, Content: []byte(OpenCodePlugin), Role: TriggerRole}
}

// CodexPointerTarget is the Codex trigger: the pointer block in AGENTS.md.
func CodexPointerTarget(agentsPath string) install.Target {
	return install.Target{Path: agentsPath, Kind: install.KindManagedBlock, BlockName: PointerBlockName,
		Annotation: PointerBlockAnnotation, Content: []byte(PointerInterior), Role: TriggerRole}
}
