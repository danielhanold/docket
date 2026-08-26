// Package opencode renders Docket's installation for opencode: skill
// symlinks, one native agent definition per agent source, and the managed
// dispatch block in the user-level AGENTS.md — all under the XDG config root.
// It plans only — nothing here writes to the filesystem apart from Detect's
// read-only stat.
package opencode

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
	Name = "opencode"

	// rootDir is opencode's user-level directory, and the three destinations
	// under it. Unlike the other three harnesses this root hangs off the XDG
	// config home rather than the home directory itself, so an
	// XDG_CONFIG_HOME pointing elsewhere moves the whole installation with it
	// — TestOpencodeXDGRoot pins that.
	rootDir      = "opencode"
	skillsDir    = "skills"
	agentsDir    = "agents"
	dispatchFile = "AGENTS.md"

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

	// skillsPreambleFormat is the sentence that tells an opencode agent to
	// preload its docket skills. It is spelled per harness rather than
	// templated: the phrasing names this harness's own skills directory, and
	// flattening the spellings into one would erase real variance. The
	// lowercase "opencode" is the vendor's own spelling of its name.
	skillsPreambleFormat = "Before acting, load these docket skills from your opencode skills directory: %s."

	// mode is constant for every docket agent: they are dispatched by a
	// primary session, never the primary themselves.
	subagentMode = "subagent"
)

// ErrRender is the sentinel for a rendering that cannot be expressed — an
// input whose value would not survive its own serialization. It is a defect in
// the bundle or in the resolved configuration, never a state of the user's
// disk.
var ErrRender = errors.New("opencode: cannot render")

type adapter struct{}

// New returns the opencode adapter.
func New() harness.Adapter { return adapter{} }

func (adapter) Name() string { return Name }

// Detect reports whether this user has opencode installed, by the presence of
// <ConfigHome>/opencode as a directory. It stats and nothing else: no shelling
// out, and no write, so detection is safe on a machine docket has never
// touched.
func (adapter) Detect(r install.UserRoots) harness.Detection {
	root := filepath.Join(r.ConfigHome, rootDir)
	d := harness.Detection{Root: root}
	if r.ConfigHome == "" {
		return d
	}
	info, err := os.Stat(root)
	d.Present = err == nil && info.IsDir()
	return d
}

// Plan renders every target an opencode installation owns. It is pure: the
// same PlanInput yields byte-identical targets, sorted by path.
func (adapter) Plan(in harness.PlanInput) ([]install.Target, error) {
	// Home is never read here: an opencode path that fell back to
	// ~/.config when an XDG override resolved elsewhere would install a tree
	// opencode does not read, so the resolved ConfigHome is the only root.
	if !filepath.IsAbs(in.Roots.ConfigHome) {
		return nil, fmt.Errorf("%w: config home %q is not an absolute path", ErrRender, in.Roots.ConfigHome)
	}
	if !filepath.IsAbs(in.AssetsDir) {
		return nil, fmt.Errorf("%w: assets dir %q is not absolute", ErrRender, in.AssetsDir)
	}
	root := filepath.Join(in.Roots.ConfigHome, rootDir)

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
		model, effort := harness.ResolvedAgent(in.Agents, Name, s.ShortName)
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
		Content:    []byte(harness.DispatchInterior(runGate)),
		BlockName:  dispatchBlockName,
		Annotation: dispatchAnnotation,
		Role:       roleDispatch,
	})

	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	return targets, nil
}

// renderAgent maps one agent source onto an opencode agent definition,
// mirroring sync-agents.sh's emit_opencode_md exactly:
//
//	frontmatter description: -> description
//	(constant)               -> mode: subagent
//	resolved model           -> model            (omitted when unpinned)
//	resolved effort          -> reasoningEffort  (only alongside a model)
//	skills preamble + body   -> body
//
// There is no `name:` key: opencode identifies an agent by its FILENAME, so a
// name field would be a key the harness does not read. opencode has no
// first-class reasoning-effort field either — it forwards unrecognized
// frontmatter keys to the provider as model options, which is what makes
// `reasoningEffort` a real per-agent effort rather than decoration.
//
// An effort resolved with no model is dropped. That is docket's own policy and
// not an opencode limitation — opencode would honor an orphan effort — but
// docket refuses to pin an effort it cannot attribute to a resolved model. The
// drop is silent here because an adapter is pure computation with no
// diagnostic stream; the shell emitter's WARN has no equivalent to write to.
func renderAgent(s harness.AgentSource, model, effort string) ([]byte, error) {
	// An effort with no model has nowhere to go; normalizing it here keeps
	// the round-trip check below asserting what was actually rendered.
	if model == "" {
		effort = ""
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: " + quoteYAML(s.Description) + "\n")
	b.WriteString("mode: " + subagentMode + "\n")
	// Model and effort are opaque vendor scalars (ADR-0015): docket keeps no
	// allowlist and passes whatever resolved through, bare. IDs reached
	// through OpenRouter are double-prefixed (`openrouter/<vendor>/<model>`)
	// and pass through whole. The round-trip check below is what makes "bare"
	// safe — a value that would not read back as itself is refused rather than
	// shipped.
	if model != "" {
		b.WriteString("model: " + model + "\n")
		if effort != "" {
			b.WriteString("reasoningEffort: " + effort + "\n")
		}
	}
	b.WriteString("---\n\n")

	body := strings.TrimRight(s.Body, "\n")
	if len(s.Skills) > 0 {
		body = fmt.Sprintf(skillsPreambleFormat, strings.Join(s.Skills, ", ")) + "\n\n" + body
	}
	b.WriteString(body + "\n")

	out := []byte(b.String())
	if err := verifyRoundTrip(out, s, model, effort); err != nil {
		return nil, err
	}
	return out, nil
}

// verifyRoundTrip reparses the rendered document and proves every field reads
// back as the value it was rendered from. Quoting correctness is thereby a
// checked property of each write rather than a claim about the escaping
// helper, which is what keeps a hostile description — or an unusual vendor
// model string — from shipping as a file that parses into something else.
func verifyRoundTrip(rendered []byte, s harness.AgentSource, model, effort string) error {
	doc, err := document.Parse(rendered)
	if err != nil {
		return fmt.Errorf("%w: %s does not reparse: %w", ErrRender, s.Name, err)
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Mode        string `yaml:"mode"`
		Model       string `yaml:"model"`
		Effort      string `yaml:"reasoningEffort"`
	}
	if err := doc.DecodeFrontmatter(&fm); err != nil {
		return fmt.Errorf("%w: %s does not decode: %w", ErrRender, s.Name, err)
	}
	got := struct {
		Name        string
		Description string
		Mode        string
		Model       string
		Effort      string
	}{fm.Name, fm.Description, fm.Mode, fm.Model, fm.Effort}
	want := got
	// The empty Name is asserted alongside the rest: a name key reintroduced
	// by a future edit is a rendering defect, not a harmless extra.
	want.Name, want.Description, want.Mode = "", s.Description, subagentMode
	want.Model, want.Effort = model, effort
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("%w: %s round-trips as %+v, want %+v", ErrRender, s.Name, got, want)
	}
	return nil
}

// quoteYAML renders a single-quoted YAML scalar. Quoting is unconditional
// rather than shape-predicated (ADR-0071): a conditional is only as good as
// its enumeration of the indicator characters, and every value passing through
// here is free text docket did not author. Single quotes need one escape — a
// literal quote is doubled — and no other character is special inside them.
func quoteYAML(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
