package install

import (
	"embed"
	"path/filepath"
	"strings"
)

// This file is the FROZEN legacy reproducer (change 0322, Part B): it recreates
// the exact bytes the FINAL v0.9.2 Bash installer (`sync-agents.sh`) wrote for a
// user-level native docket agent definition, so an on-disk artifact that matches
// those bytes can be proven a legacy install and adopted rather than reported as
// an ownership conflict (see inspect.go's provenByLegacy — ownership proof
// three).
//
// It is deliberately SELF-CONTAINED and MUST NOT call internal/harness/* (the
// live renderers). A future renderer change must never silently change what
// counts as "a legacy install": the frozen agent source bodies are embedded
// from internal/install/legacydata/ (a snapshot of v0.9.2's agents/docket-*.md),
// and the per-harness frontmatter/pin rendering below is a hand port of the
// v0.9.2 emitters (emit / emit_codex_toml / emit_cursor_md / emit_opencode_md),
// documented against testdata/legacy/README.md's shape table. When the live
// harness renderers drift, this reproducer stays fixed — the corpus wins.

// roleAgent is the Role value every harness adapter stamps on a native
// agent-definition target (internal/harness/*/… roleAgent = "agent"). The
// reproducer keys on it to identify an agent-definition target before parsing
// (harness, agent) out of the path.
const roleAgent = "agent"

// legacydata holds the frozen v0.9.2 wrapper-source bodies for every built-in
// agent — all sixteen, so the reproducer covers every docket-* agent a real
// legacy machine has, not only the two with captured goldens.
//
//go:embed legacydata/*.md
var legacydata embed.FS

// HarnessPin is the resolved (model, effort) pair the legacy installer baked
// for one agent on one harness. The values are the pre-normalization resolved
// outcome fed to the v0.9.2 emitters: the sentinels `inherit` (model) and `auto`
// (effort) are carried through here exactly as the Bash resolver produced them,
// and each emitter applies its own normalization (matching the frozen bytes).
type HarnessPin struct {
	Model  string
	Effort string
}

// AgentPin is one built-in agent's resolved pin on each harness. The pin
// resolves per harness because v0.9.2's shipped agents/harness-defaults.yml
// sidecar assigns different (model, effort) values to different harnesses; a
// harness missing from ByHarness renders unpinned.
type AgentPin struct {
	ByHarness map[string]HarnessPin
}

// LegacyInputs is the closed set of legacy global inputs — and nothing more —
// needed to recreate the user-level bytes the v0.9.2 Bash installer wrote:
// which harnesses it targeted, and the resolved (model, effort) pin for each
// agent on each of them.
type LegacyInputs struct {
	// Harnesses is the harness-token set the legacy install targeted (the
	// resolved `agent_harnesses`). A target under a harness absent from this
	// set is outside the inventory.
	Harnesses []string
	// AgentPins carries the resolved pins for each built-in agent, keyed by
	// agent short-name (the basename with the "docket-" prefix and extension
	// stripped — e.g. "status", "brainstorm-consultant").
	AgentPins map[string]AgentPin
}

// legacyAgentDir maps the on-disk directory that holds a harness's user-level
// agent definitions to the harness token. This is the harness dimension of the
// closed inventory, and it mirrors the real install target paths: claude/codex/
// cursor hang a dotted dir off the home directory, while opencode's root is an
// undotted dir under the XDG config home (internal/harness/opencode rootDir =
// "opencode"). Only these four harnesses have a named v0.9.2 emitter and a
// captured shape; any other token is out of the reproducer inventory by design
// (testdata/legacy/README.md, "Non-shipped harness tokens").
var legacyAgentDir = map[string]string{
	".claude":  "claude",
	".codex":   "codex",
	".cursor":  "cursor",
	"opencode": "opencode",
}

// legacyExt is the file extension the v0.9.2 generator wrote per harness
// (harness_ext: codex -> toml, everything else -> md). A path whose extension
// does not match its harness is not a legacy agent definition.
func legacyExt(harness string) string {
	if harness == "codex" {
		return "toml"
	}
	return "md"
}

// legacyWrapperSource is the parsed form of a frozen v0.9.2 wrapper source,
// carrying both the raw file text (for the claude stream emitter) and the four
// source-derived fields the named emitters consume.
type legacyWrapperSource struct {
	raw       string // full source bytes, for emitClaude's stream transform
	name      string
	desc      string
	skillsCSV string
	body      string // leading blank lines trimmed, trailing newlines stripped
}

// legacyAgentSources is the parsed frozen source per agent short-name, built
// once at init from the embedded legacydata snapshot.
var legacyAgentSources = loadLegacyAgentSources()

func loadLegacyAgentSources() map[string]legacyWrapperSource {
	entries, err := legacydata.ReadDir("legacydata")
	if err != nil {
		panic("install: reading embedded legacydata: " + err.Error())
	}
	out := make(map[string]legacyWrapperSource, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := legacydata.ReadFile("legacydata/" + e.Name())
		if err != nil {
			panic("install: reading embedded " + e.Name() + ": " + err.Error())
		}
		short := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "docket-"), ".md")
		out[short] = parseLegacyWrapperSource(string(data), short)
	}
	return out
}

// NewLegacyReproducer returns a LegacyReproducer over the given legacy inputs.
// For a KindFile native-agent-definition target whose path resolves to a
// harness in the inventory (present in in.Harnesses, a named-emitter harness)
// and a known built-in agent, it returns the frozen v0.9.2 bytes and true; for
// anything outside that closed inventory it returns (nil, false). The bytes it
// returns are only a CANDIDATE — provenByLegacy still requires a byte-exact
// match against what is on disk, so a wrong pin never causes a false adoption.
func NewLegacyReproducer(in LegacyInputs) LegacyReproducer {
	harnessSet := make(map[string]bool, len(in.Harnesses))
	for _, h := range in.Harnesses {
		harnessSet[h] = true
	}
	pins := in.AgentPins
	return func(t Target) ([]byte, bool) {
		if t.Kind != KindFile || t.Role != roleAgent {
			return nil, false
		}
		harness, agent, ok := parseLegacyAgentPath(t.Path)
		if !ok {
			return nil, false
		}
		if _, named := legacyRender[harness]; !named {
			return nil, false
		}
		if !harnessSet[harness] {
			return nil, false
		}
		src, ok := legacyAgentSources[agent]
		if !ok {
			return nil, false
		}
		pin := pins[agent].ByHarness[harness]
		return legacyRender[harness](src, pin.Model, pin.Effort), true
	}
}

// parseLegacyAgentPath extracts (harness, agent short-name) from a native
// agent-definition target path of the frozen shape `<…>/<harnessDir>/agents/
// docket-<name>.<ext>`, requiring the extension to match the harness. It
// returns ok=false for any path that is not that shape. The path is cleaned so
// stray separators do not change the parse; symlink hops are not resolved here
// because the target path is the installer's own planned destination, a
// spelling the planner already canonicalises (inspect.go canonicalPath).
func parseLegacyAgentPath(p string) (harness, agent string, ok bool) {
	p = filepath.Clean(p)
	base := filepath.Base(p)
	agentsDir := filepath.Dir(p)
	if filepath.Base(agentsDir) != "agents" {
		return "", "", false
	}
	harness, known := legacyAgentDir[filepath.Base(filepath.Dir(agentsDir))]
	if !known {
		return "", "", false
	}
	ext := strings.TrimPrefix(filepath.Ext(base), ".")
	if ext != legacyExt(harness) {
		return "", "", false
	}
	name := strings.TrimSuffix(base, "."+ext)
	if !strings.HasPrefix(name, "docket-") {
		return "", "", false
	}
	agent = strings.TrimPrefix(name, "docket-")
	if agent == "" {
		return "", "", false
	}
	return harness, agent, true
}

// legacyRender dispatches to the frozen port of the v0.9.2 per-harness emitter.
// Its keys are exactly the four harnesses with a named emitter — membership
// here is what distinguishes an inventory harness from an unmapped token.
var legacyRender = map[string]func(src legacyWrapperSource, model, effort string) []byte{
	"claude":   func(s legacyWrapperSource, m, e string) []byte { return emitClaude(s.raw, m, e) },
	"codex":    emitCodexTOML,
	"cursor":   emitCursorMD,
	"opencode": emitOpencodeMD,
}

// --- source parsing (port of parse_wrapper_source) ---------------------------

// splitAwkLines splits s into the records an awk `getline`/main loop would see:
// on '\n', with a single trailing empty record (the file's final newline)
// dropped. It is the shared line model for every port below.
func splitAwkLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// isFrontmatterFence reports whether a line is a `---` frontmatter fence,
// matching the emitters' `^---[[:space:]]*$`.
func isFrontmatterFence(line string) bool {
	return strings.TrimRight(line, " \t\n\r\f\v") == "---"
}

// firstFieldValue returns the value of the first line beginning `<key>:`, with
// the `<key>:` prefix and any following blank space stripped — the behaviour of
// the emitters' `sed -n '/^<key>:/{s/^<key>:[[:space:]]*//;p;q;}'`.
func firstFieldValue(lines []string, key string) (string, bool) {
	prefix := key + ":"
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return strings.TrimLeft(l[len(prefix):], " \t\n\r\f\v"), true
		}
	}
	return "", false
}

// parseLegacyWrapperSource ports parse_wrapper_source: name/description/skills
// from the frontmatter, and the body (everything after the second fence, with
// leading blank lines trimmed and trailing newlines stripped). short is the
// agent short-name, used only for the name fallback.
func parseLegacyWrapperSource(raw, short string) legacyWrapperSource {
	lines := splitAwkLines(raw)

	name, ok := firstFieldValue(lines, "name")
	if !ok || name == "" {
		name = "docket-" + short
	}
	desc, _ := firstFieldValue(lines, "description")

	skillsCSV := ""
	if v, ok := firstFieldValue(lines, "skills"); ok {
		// sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//'
		v = strings.TrimPrefix(v, "[")
		v = strings.TrimRight(v, " \t\n\r\f\v")
		v = strings.TrimSuffix(v, "]")
		v = strings.TrimRight(v, " \t\n\r\f\v")
		skillsCSV = v
	}

	// Body = lines after the second frontmatter fence.
	d := 0
	var body []string
	for _, l := range lines {
		if d < 2 && isFrontmatterFence(l) {
			d++
			continue
		}
		if d >= 2 {
			body = append(body, l)
		}
	}
	// Trim leading whitespace-only lines (awk `NF{p=1} p{print}`).
	i := 0
	for i < len(body) && strings.TrimSpace(body[i]) == "" {
		i++
	}
	body = body[i:]
	// Strip trailing newlines the way `$(...)` command substitution does.
	bodyStr := strings.TrimRight(strings.Join(body, "\n"), "\n")

	return legacyWrapperSource{raw: raw, name: name, desc: desc, skillsCSV: skillsCSV, body: bodyStr}
}

// --- claude emitter (port of emit) -------------------------------------------

// emitClaude streams the raw wrapper source, inserting the resolved model/effort
// pin at the closing frontmatter fence and dropping any model:/effort: line the
// source still carries. `auto` effort is dropped; `inherit` model passes through
// verbatim (a real Claude Code frontmatter value).
func emitClaude(raw, model, effort string) []byte {
	if effort == "auto" {
		effort = ""
	}
	var b strings.Builder
	d := 0
	infm := false
	for _, l := range splitAwkLines(raw) {
		if isFrontmatterFence(l) {
			d++
			if d == 1 {
				b.WriteString(l)
				b.WriteByte('\n')
				infm = true
				continue
			}
			if d == 2 && infm {
				if model != "" {
					b.WriteString("model: " + model + "\n")
				}
				if effort != "" {
					b.WriteString("effort: " + effort + "\n")
				}
				infm = false
				b.WriteString(l)
				b.WriteByte('\n')
				continue
			}
			b.WriteString(l)
			b.WriteByte('\n')
			continue
		}
		if infm && isFrontmatterKeyLine(l, "model") {
			continue
		}
		if infm && isFrontmatterKeyLine(l, "effort") {
			continue
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// isFrontmatterKeyLine reports whether line matches `^<key>[[:space:]]*:`.
func isFrontmatterKeyLine(line, key string) bool {
	if !strings.HasPrefix(line, key) {
		return false
	}
	rest := strings.TrimLeft(line[len(key):], " \t")
	return strings.HasPrefix(rest, ":")
}

// --- codex emitter (port of emit_codex_toml) ---------------------------------

func emitCodexTOML(src legacyWrapperSource, model, effort string) []byte {
	dev := src.body
	if src.skillsCSV != "" {
		dev = "Before acting, load these docket skills from your linked Codex skills directory: " + src.skillsCSV + ".\n\n" + src.body
	}
	var b strings.Builder
	b.WriteString("name = \"" + tomlEscapeBasic(src.name) + "\"\n")
	b.WriteString("description = \"" + tomlEscapeBasic(src.desc) + "\"\n")
	if model != "" && model != "inherit" {
		b.WriteString("model = \"" + tomlEscapeBasic(model) + "\"\n")
	}
	if effort != "" && effort != "auto" {
		b.WriteString("model_reasoning_effort = \"" + tomlEscapeBasic(effort) + "\"\n")
	}
	b.WriteString("developer_instructions = \"\"\"\n" + tomlEscapeMultiline(dev) + "\n\"\"\"\n")
	return []byte(b.String())
}

// tomlEscapeBasic ports toml_escape_basic: backslash then double-quote.
func tomlEscapeBasic(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// tomlEscapeMultiline ports the developer_instructions escape: backslash, then
// a literal `"""` defused to `""\"`.
func tomlEscapeMultiline(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"""`, `""\"`)
	return s
}

// --- cursor emitter (port of emit_cursor_md) ---------------------------------

func emitCursorMD(src legacyWrapperSource, model, effort string) []byte {
	if model == "inherit" {
		model = ""
	}
	if effort == "auto" {
		effort = ""
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + src.name + "\n")
	b.WriteString("description: " + src.desc + "\n")
	if model != "" {
		if effort != "" {
			b.WriteString("model: " + model + "[effort=" + effort + "]\n")
		} else {
			b.WriteString("model: " + model + "\n")
		}
	}
	// A resolved effort with no model is dropped (the v0.9.2 emitter only warns);
	// it is structurally unreachable from the captured shapes and reproduces no
	// extra bytes here either.
	b.WriteString("---\n\n")
	if src.skillsCSV != "" {
		b.WriteString("Before acting, load these docket skills from your Cursor skills directory: " + src.skillsCSV + ".\n\n")
	}
	b.WriteString(src.body + "\n")
	return []byte(b.String())
}

// --- opencode emitter (port of emit_opencode_md) -----------------------------

func emitOpencodeMD(src legacyWrapperSource, model, effort string) []byte {
	if model == "inherit" {
		model = ""
	}
	if effort == "auto" {
		effort = ""
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: " + src.desc + "\n")
	b.WriteString("mode: subagent\n")
	if model != "" {
		b.WriteString("model: " + model + "\n")
		if effort != "" {
			b.WriteString("reasoningEffort: " + effort + "\n")
		}
	}
	// As with cursor, a model-less effort is dropped (warn only) and unreachable
	// from the captured shapes.
	b.WriteString("---\n\n")
	if src.skillsCSV != "" {
		b.WriteString("Before acting, load these docket skills from your opencode skills directory: " + src.skillsCSV + ".\n\n")
	}
	b.WriteString(src.body + "\n")
	return []byte(b.String())
}
