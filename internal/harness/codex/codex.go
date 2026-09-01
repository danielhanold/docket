// Package codex renders Docket's installation for Codex: skill symlinks under
// the harness-neutral agents root and one native TOML agent definition per
// agent source. It no longer plans a user-global dispatch block (change
// 0351) — parent-facing routing belongs to a repository's own AGENTS.md, not a
// personal global one — but it still exports GlobalDispatchTarget so the
// installer can retire a leftover a prior install owns. It plans only: nothing
// here writes to the filesystem apart from Detect's read-only stat.
package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
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

// codexDispatchBoundary is the Codex-specific nested-dispatch boundary
// (change 0365). It is emitted into EVERY generated agent, unconditionally:
// composition is a property of the invoked skill and may change through
// configuration, so an allowlist of today's dispatching wrappers would
// silently miss a future custom binding, while a leaf agent receiving a
// conditional instruction incurs no behavioral change. It is Codex-specific
// tool placement (ADR-0060), so it lives in this renderer — the shared agent
// bodies stay harness-neutral. It closes the false-negative shape a live run
// hit: a parent inspected a nested JavaScript tool inventory, found no
// dispatch entry there, and halted without ever attempting the registered
// dispatch (see the convention's "Dispatch-capability resolution" section for
// the harness-neutral rule this instantiates).
const codexDispatchBoundary = "When your active charter requires another agent, dispatch it with Codex's direct named-agent dispatch from your active top-level tool surface. Nested orchestration inventories — tool lists read from inside another tool — omit top-level collaboration controls, so absence from such a nested inventory cannot establish dispatch unavailability; only a failed direct dispatch attempt or an explicit policy denial does."

// ErrRender is the sentinel for a rendering that cannot be expressed — an
// input whose value would not survive its own serialization. It is a defect in
// the bundle or in the resolved configuration, never a state of the user's
// disk.
var ErrRender = errors.New("codex: cannot render")

type adapter struct{}

// RoleContract is the single typed Codex role representation consumed by both
// ordinary TOML registration and coordinator root entry. Keeping instructions
// and pins here prevents the runtime path from growing a second renderer.
type RoleContract struct {
	Name                  string
	Description           string
	LaunchPosture         harness.LaunchPosture
	Model                 string
	Effort                string
	Skills                []string
	DeveloperInstructions string
}

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

	skillDirs := harness.SkillDirs(in.Assets)
	targets := make([]install.Target, 0, len(sources)+len(skillDirs))

	for _, dir := range skillDirs {
		targets = append(targets, install.Target{
			Path:       filepath.Join(in.Roots.Home, skillsRootDir, skillsDir, dir),
			Kind:       install.KindSymlink,
			LinkTarget: filepath.Join(in.AssetsDir, skillsDir, dir),
			Role:       roleSkill,
		})
	}

	for _, s := range sources {
		contract := roleContract(s, in.Agents)
		targets = append(targets, install.Target{
			Path:    filepath.Join(root, agentsDir, s.Name+".toml"),
			Kind:    install.KindFile,
			Content: renderAgent(contract),
			Role:    roleAgent,
		})
	}

	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	return targets, nil
}

// RoleContractFor resolves one named role from the same catalog and agent-pin
// table Plan consumes. It performs no filesystem reads and returns an error for
// an absent name rather than fabricating a generic role.
func RoleContractFor(in harness.PlanInput, name string) (RoleContract, error) {
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		return RoleContract{}, err
	}
	for _, s := range sources {
		if s.Name == name {
			return roleContract(s, in.Agents), nil
		}
	}
	return RoleContract{}, fmt.Errorf("%w: agent inventory carries no role %q", ErrRender, name)
}

func roleContract(s harness.AgentSource, agents config.AgentsTable) RoleContract {
	model, effort := harness.ResolvedAgent(agents, Name, s.ShortName)
	body := trimBlankEdges(s.Body)
	dev := body
	if len(s.Skills) > 0 {
		dev = fmt.Sprintf(skillsPreambleFormat, strings.Join(s.Skills, ", ")) + "\n\n" + body
	}
	dev = harness.RecursionGuard(s.Name) + "\n\n" + codexDispatchBoundary + "\n\n" + dev
	description := s.Description
	if s.LaunchPosture == harness.LaunchRootCoordinator {
		description = "[docket launch: root-coordinator] " + description
	}
	return RoleContract{
		Name:                  s.Name,
		Description:           description,
		LaunchPosture:         s.LaunchPosture,
		Model:                 model,
		Effort:                effort,
		Skills:                append([]string(nil), s.Skills...),
		DeveloperInstructions: dev,
	}
}

// GlobalDispatchTarget is the user-global dispatch destination this adapter
// USED to plan and no longer does (change 0351): the managed `dispatch` block
// in ~/.codex/AGENTS.md. Parent-facing routing instructions now live in a
// repository's own AGENTS.md, never a personal global one, so Plan omits this
// target — but the installer still needs its location and identity to RETIRE a
// leftover a prior install owns. Content is left nil: retirement proves
// ownership from the recorded interior or the frozen legacy reproducer, never
// from a freshly rendered body.
func GlobalDispatchTarget(r install.UserRoots) install.Target {
	return install.Target{
		Path:       filepath.Join(r.Home, rootDir, dispatchFile),
		Kind:       install.KindManagedBlock,
		BlockName:  dispatchBlockName,
		Annotation: dispatchAnnotation,
		Role:       roleDispatch,
	}
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
func renderAgent(contract RoleContract) []byte {
	var b strings.Builder
	b.WriteString("name = \"" + escapeBasic(contract.Name) + "\"\n")
	b.WriteString("description = \"" + escapeBasic(contract.Description) + "\"\n")
	// Model and effort are opaque vendor scalars (ADR-0015): docket keeps no
	// allowlist and passes whatever resolved through, escaped but otherwise
	// untouched.
	if contract.Model != "" {
		b.WriteString("model = \"" + escapeBasic(contract.Model) + "\"\n")
	}
	if contract.Effort != "" {
		b.WriteString("model_reasoning_effort = \"" + escapeBasic(contract.Effort) + "\"\n")
	}
	// The self-recursion guard is the first paragraph of the instructions, at the
	// one consistent position every renderer uses: immediately after the
	// frontmatter/heading, ahead of the body (skills preamble included) this
	// renderer already emits. harness.RecursionGuard is the shared emitter, so the
	// paragraph is byte-identical across all four harnesses.
	// The dispatch boundary is the SECOND paragraph: the recursion guard keeps
	// its cross-harness first-paragraph position, and the boundary sits ahead
	// of the skills preamble and body so it reads as harness contract, not
	// role prose.
	b.WriteString("developer_instructions = \"\"\"\n" + escapeMultiline(contract.DeveloperInstructions) + "\n\"\"\"\n")

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
