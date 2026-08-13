// Package claude renders Docket's installation for Claude Code: skill
// symlinks, one native agent definition per agent source, and the managed
// dispatch block in the user-level CLAUDE.md. It plans only — nothing here
// writes to the filesystem apart from Detect's read-only stat.
package claude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
)

const (
	// Name is the harness token this adapter answers to; it is one of
	// harness.Order's four.
	Name = "claude"

	// root, and the three destinations under it. Claude Code reads user-level
	// skills from ~/.claude/skills, agent definitions from ~/.claude/agents,
	// and the always-in-context instructions from ~/.claude/CLAUDE.md.
	rootDir      = ".claude"
	skillsDir    = "skills"
	agentsDir    = "agents"
	dispatchFile = "CLAUDE.md"

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
)

// ErrRender is the sentinel for a rendering that cannot be expressed — an
// input whose value would not survive its own serialization. It is a defect in
// the bundle or in the resolved configuration, never a state of the user's
// disk.
var ErrRender = errors.New("claude: cannot render")

type adapter struct{}

// New returns the Claude adapter.
func New() harness.Adapter { return adapter{} }

func (adapter) Name() string { return Name }

// Detect reports whether this user has Claude Code installed, by the presence
// of ~/.claude as a directory. It stats and nothing else: no shelling out, and
// no write, so detection is safe on a machine docket has never touched.
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

// Plan renders every target a Claude installation owns. It is pure: the same
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
			Path:       filepath.Join(root, skillsDir, dir),
			Kind:       install.KindSymlink,
			LinkTarget: filepath.Join(in.AssetsDir, skillsDir, dir),
			Role:       roleSkill,
		})
	}

	for _, s := range sources {
		// Raw, not ResolvedAgent: `model: inherit` is a real Claude Code value
		// on this harness and must survive to renderAgent. See renderAgent's
		// field-mapping comment.
		model, effort := harness.ResolvedAgentRaw(in.Agents, Name, s.ShortName)
		content, err := renderAgent(s, model, effort)
		if err != nil {
			return nil, err
		}
		targets = append(targets, install.Target{
			Path:    filepath.Join(root, agentsDir, s.Name+".md"),
			Kind:    install.KindFile,
			Content: content,
			Role:    roleAgent,
		})
	}

	targets = append(targets, install.Target{
		Path:       filepath.Join(root, dispatchFile),
		Kind:       install.KindManagedBlock,
		Content:    []byte(harness.DispatchInterior(sources, runGate)),
		BlockName:  dispatchBlockName,
		Annotation: dispatchAnnotation,
		Role:       roleDispatch,
	})

	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	return targets, nil
}

// renderAgent maps one agent source onto a Claude Code agent definition,
// mirroring the field mapping of sync-agents.sh's generic emit():
//
//	frontmatter name:        -> name
//	frontmatter description: -> description
//	frontmatter skills:      -> skills          (preserved; omitted when empty)
//	resolved model           -> model           (omitted when unpinned)
//	resolved effort          -> effort          (omitted when unpinned)
//	body                     -> body verbatim
//
// One difference from emit() is deliberate: emit() is a stream transform, so it
// carries every other key the source happens to have (worktree-scope, which is
// docket's own generation-time metadata and means nothing to Claude Code); this
// renders the mapped fields only.
//
// `model: inherit` is NOT normalized away here, matching emit(): Claude Code
// documents `inherit` as a real frontmatter value meaning "run on the parent
// conversation's model", a different runtime outcome from omitting the key. The
// caller therefore resolves through harness.ResolvedAgentRaw rather than the
// harness.ResolvedAgent the other three adapters share, and an effort pinned
// beside an `inherit` model is emitted too — emit() folds `auto` and nothing
// else. TestClaudeAgentPinCases and the docket-adr/docket-review-deep goldens
// pin both halves.
//
// Claude reads `skills:` natively, so unlike the codex/cursor/opencode
// renderers there is no skills preamble in the body.
func renderAgent(s harness.AgentSource, model, effort string) ([]byte, error) {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + quoteYAML(s.Name) + "\n")
	b.WriteString("description: " + quoteYAML(s.Description) + "\n")
	if len(s.Skills) > 0 {
		quoted := make([]string, 0, len(s.Skills))
		for _, sk := range s.Skills {
			quoted = append(quoted, quoteYAML(sk))
		}
		b.WriteString("skills: [" + strings.Join(quoted, ", ") + "]\n")
	}
	// Model and effort are opaque vendor scalars (ADR-0015): docket keeps no
	// allowlist and passes whatever resolved through, bare. The round-trip
	// check below is what makes "bare" safe — a value that would not read back
	// as itself is refused rather than shipped.
	if model != "" {
		b.WriteString("model: " + model + "\n")
	}
	if effort != "" {
		b.WriteString("effort: " + effort + "\n")
	}
	b.WriteString("---\n")

	body := s.Body
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	b.WriteString(body)

	out := []byte(b.String())
	if err := verifyRoundTrip(out, s, model, effort); err != nil {
		return nil, err
	}
	return out, nil
}

// verifyRoundTrip reparses the rendered document and proves every field reads
// back as the value it was rendered from. Quoting correctness is thereby a
// checked property of each write rather than a claim about the escaping helper,
// which is what keeps a hostile description — or an unusual vendor model
// string — from shipping as a file that parses into something else.
func verifyRoundTrip(rendered []byte, s harness.AgentSource, model, effort string) error {
	doc, err := document.Parse(rendered)
	if err != nil {
		return fmt.Errorf("%w: %s does not reparse: %w", ErrRender, s.Name, err)
	}
	var fm struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Skills      []string `yaml:"skills"`
		Model       string   `yaml:"model"`
		Effort      string   `yaml:"effort"`
	}
	if err := doc.DecodeFrontmatter(&fm); err != nil {
		return fmt.Errorf("%w: %s does not decode: %w", ErrRender, s.Name, err)
	}
	switch {
	case fm.Name != s.Name:
		return fmt.Errorf("%w: %s round-trips its name as %q", ErrRender, s.Name, fm.Name)
	case fm.Description != s.Description:
		return fmt.Errorf("%w: %s round-trips its description as %q", ErrRender, s.Name, fm.Description)
	case fm.Model != model:
		return fmt.Errorf("%w: %s round-trips model %q as %q", ErrRender, s.Name, model, fm.Model)
	case fm.Effort != effort:
		return fmt.Errorf("%w: %s round-trips effort %q as %q", ErrRender, s.Name, effort, fm.Effort)
	case len(s.Skills) > 0 && !reflect.DeepEqual(fm.Skills, s.Skills):
		return fmt.Errorf("%w: %s round-trips skills %v as %v", ErrRender, s.Name, s.Skills, fm.Skills)
	}
	return nil
}

// quoteYAML renders a single-quoted YAML scalar. Quoting is unconditional
// rather than shape-predicated (ADR-0071): a conditional is only as good as its
// enumeration of the indicator characters, and every value passing through here
// is free text docket did not author. Single quotes need one escape — a literal
// quote is doubled — and no other character is special inside them.
func quoteYAML(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
