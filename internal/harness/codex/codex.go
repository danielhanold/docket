// Package codex renders Docket's installation for Codex: skill symlinks under
// the harness-neutral agents root, one native TOML agent definition per agent
// source, and the managed dispatch block in the user-level ~/.codex/AGENTS.md.
// It plans only — nothing here writes to the filesystem apart from Detect's
// read-only stat.
package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
)

const (
	// Name is the harness token this adapter answers to; it is one of
	// harness.Order's four.
	Name = "codex"

	// rootDir is the Codex home, and agents/AGENTS.md sit under it. Skills do
	// NOT: Codex reads its skills from the harness-neutral ~/.agents/skills
	// root, so linking them under rootDir would install a tree Codex never
	// reads. TestCodexSkillsRootIsAgents pins both halves of that fact.
	rootDir      = ".codex"
	agentsDir    = "agents"
	dispatchFile = "AGENTS.md"

	// The harness-neutral skills root, and the directory below it.
	skillsRootDir = ".agents"
	skillsDir     = "skills"

	// The managed block docket owns in the dispatch file. Both spellings are
	// part of the installed-state identity: install.InspectTarget finds the
	// block by name, and the annotation is what tells a human reading the file
	// that hand edits are lost.
	dispatchBlockName  = "dispatch"
	dispatchAnnotation = "managed by docket — do not hand-edit"

	// Target roles, as recorded in the installed state.
	roleSkill    = "skill"
	roleAgent    = "agent"
	roleDispatch = "dispatch"

	// skillsPreambleFormat is the sentence that tells a Codex agent to preload
	// its docket skills. It is spelled per harness rather than templated: the
	// phrasing names this harness's own skills directory, and flattening the
	// three spellings into one would erase real variance.
	skillsPreambleFormat = "Before acting, load these docket skills from your linked Codex skills directory: %s."
)

// ErrRender is the sentinel for a rendering that cannot be expressed — an
// input whose value would not survive its own serialization. It is a defect in
// the bundle or in the resolved configuration, never a state of the user's
// disk.
var ErrRender = errors.New("codex: cannot render")

type adapter struct{}

// New returns the Codex adapter.
func New() harness.Adapter { return adapter{} }

func (adapter) Name() string { return Name }

// Detect reports whether this user has Codex installed, by the presence of
// ~/.codex as a directory. It stats and nothing else: no shelling out, and no
// write, so detection is safe on a machine docket has never touched.
func (adapter) Detect(r install.UserRoots) harness.Detection {
	root := filepath.Join(r.Home, rootDir)
	d := harness.Detection{Root: root}
	if r.Home == "" {
		return d
	}
	info, err := os.Stat(root)
	d.Present = err == nil && info.IsDir()
	return d
}

// Plan renders every target a Codex installation owns. It is pure: the same
// PlanInput yields byte-identical targets, sorted by path.
func (adapter) Plan(in harness.PlanInput) ([]install.Target, error) {
	if in.Roots.Home == "" {
		return nil, fmt.Errorf("%w: the resolved roots carry no home directory", ErrRender)
	}
	if !filepath.IsAbs(in.AssetsDir) {
		return nil, fmt.Errorf("%w: assets dir %q is not absolute", ErrRender, in.AssetsDir)
	}
	root := filepath.Join(in.Roots.Home, rootDir)

	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		return nil, err
	}
	runGate, err := harness.RunGate(in.Assets)
	if err != nil {
		return nil, err
	}

	skillDirs := harness.SkillDirs(in.Assets)
	targets := make([]install.Target, 0, len(sources)+len(skillDirs)+1)

	for _, dir := range skillDirs {
		targets = append(targets, install.Target{
			Path:       filepath.Join(in.Roots.Home, skillsRootDir, skillsDir, dir),
			Kind:       install.KindSymlink,
			LinkTarget: filepath.Join(in.AssetsDir, skillsDir, dir),
			Role:       roleSkill,
		})
	}

	for _, s := range sources {
		model, effort := harness.ResolvedAgent(in.Agents, Name, s.ShortName)
		targets = append(targets, install.Target{
			Path:    filepath.Join(root, agentsDir, s.Name+".toml"),
			Kind:    install.KindFile,
			Content: renderAgent(s, model, effort),
			Role:    roleAgent,
		})
	}

	targets = append(targets, install.Target{
		Path:       filepath.Join(root, dispatchFile),
		Kind:       install.KindManagedBlock,
		Content:    []byte(harness.DispatchInterior(runGate)),
		BlockName:  dispatchBlockName,
		Annotation: dispatchAnnotation,
		Role:       roleDispatch,
	})

	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	return targets, nil
}

// renderAgent maps one agent source onto a Codex TOML agent document,
// mirroring sync-agents.sh's emit_codex_toml exactly:
//
//	frontmatter name:        -> name
//	frontmatter description: -> description
//	resolved model           -> model                   (omitted when unpinned)
//	resolved effort          -> model_reasoning_effort  (omitted when unpinned)
//	skills preamble + body   -> developer_instructions  (multi-line basic string)
//
// Codex has no `skills:` key of its own — the preload is prose in the
// instructions, pointing at the linked skills directory. The `inherit`/`auto`
// sentinels the shell emitter tests for at emit position are already
// normalized away by harness.ResolvedAgent, which every adapter EXCEPT claude
// shares (Codex has no `inherit` value of its own; Claude Code does, so that
// adapter resolves through harness.ResolvedAgentRaw), so an unpinned field
// simply omits its key and Codex applies its own default.
func renderAgent(s harness.AgentSource, model, effort string) []byte {
	var b strings.Builder
	b.WriteString("name = \"" + escapeBasic(s.Name) + "\"\n")
	b.WriteString("description = \"" + escapeBasic(s.Description) + "\"\n")
	// Model and effort are opaque vendor scalars (ADR-0015): docket keeps no
	// allowlist and passes whatever resolved through, escaped but otherwise
	// untouched.
	if model != "" {
		b.WriteString("model = \"" + escapeBasic(model) + "\"\n")
	}
	if effort != "" {
		b.WriteString("model_reasoning_effort = \"" + escapeBasic(effort) + "\"\n")
	}

	body := trimBlankEdges(s.Body)
	dev := body
	if len(s.Skills) > 0 {
		dev = fmt.Sprintf(skillsPreambleFormat, strings.Join(s.Skills, ", ")) + "\n\n" + body
	}
	b.WriteString("developer_instructions = \"\"\"\n" + escapeMultiline(dev) + "\n\"\"\"\n")

	return []byte(b.String())
}

// escapeBasic escapes a value for a TOML basic (double-quoted) string:
// backslash first, then the double quote. Escaping is unconditional rather
// than shape-predicated — every value passing through here is free text docket
// did not author, and a conditional is only as good as its enumeration.
func escapeBasic(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// escapeMultiline escapes a value for a TOML multi-line basic string. Only two
// sequences can break out: a backslash, and a literal `"""` that would
// terminate the string early. Interior double quotes are legal unescaped, and
// the shell emitter leaves them alone, so this one does too — the two must
// agree byte for byte.
func escapeMultiline(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"""`, `""\"`)
}

// trimBlankEdges drops leading blank lines and trailing newlines, matching what
// parse_wrapper_source's awk and the shell's command substitution do to a
// wrapper body before it reaches the emitter.
func trimBlankEdges(body string) string {
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return strings.TrimRight(strings.Join(lines[i:], "\n"), "\n")
}
