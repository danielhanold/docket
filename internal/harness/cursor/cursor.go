// Package cursor renders Docket's installation for Cursor: skill symlinks and
// one native custom-agent document per agent source. It no longer plans a
// user-global dispatch rule (change 0351) — parent-facing routing belongs to a
// repository's own .cursor/rules, not a personal global one — but it still
// exports GlobalDispatchTarget so the installer can retire a leftover a prior
// install owns. It plans only: nothing here writes to the filesystem apart from
// Detect's read-only stat.
package cursor

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
	Name = "cursor"

	// root, and the three destinations under it. Cursor reads user-level
	// skills from ~/.cursor/skills, custom agents from ~/.cursor/agents, and
	// always-apply rules from ~/.cursor/rules.
	rootDir      = ".cursor"
	skillsDir    = "skills"
	agentsDir    = "agents"
	rulesDir     = "rules"
	dispatchFile = "docket-dispatch.mdc"

	// Target roles, as recorded in the installed state.
	roleSkill    = "skill"
	roleAgent    = "agent"
	roleDispatch = "dispatch"

	// skillsPreambleFormat is the sentence that tells a Cursor agent to
	// preload its docket skills. It is spelled per harness rather than
	// templated: the phrasing names this harness's own skills directory, and
	// flattening the spellings into one would erase real variance.
	skillsPreambleFormat = "Before acting, load these docket skills from your Cursor skills directory: %s."
)

// ErrRender is the sentinel for a rendering that cannot be expressed — an
// input whose value would not survive its own serialization, or a bundle
// missing a payload the rendering is assembled from. It is a defect in the
// bundle or in the resolved configuration, never a state of the user's disk.
var ErrRender = errors.New("cursor: cannot render")

type adapter struct{}

// New returns the Cursor adapter.
func New() harness.Adapter { return adapter{} }

func (adapter) Name() string { return Name }

// Detect reports whether this user has Cursor installed, by the presence of
// ~/.cursor as a directory. It stats and nothing else: no shelling out, and no
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

// Plan renders every target a Cursor installation owns. It is pure: the same
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

	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	return targets, nil
}

// GlobalDispatchTarget is the user-global dispatch destination this adapter
// USED to plan and no longer does (change 0351): the dedicated rule file
// ~/.cursor/rules/docket-dispatch.mdc. Unlike the other three harnesses this is
// a whole file docket owns — a KindFile, not a managed block inside a file the
// user also writes. Parent-facing routing instructions now live in a
// repository's own .cursor/rules, never a personal global one, so Plan omits
// this target — but the installer still needs its location and identity to
// RETIRE a leftover a prior install owns. Content is left nil: retirement
// proves ownership from the recorded whole-file digest or the frozen legacy
// reproducer, never from a freshly assembled rule.
func GlobalDispatchTarget(r install.UserRoots) install.Target {
	return install.Target{
		Path: filepath.Join(r.Home, rootDir, rulesDir, dispatchFile),
		Kind: install.KindFile,
		Role: roleDispatch,
	}
}

// dispatchRuleFrontmatter is the always-apply Cursor rule header. Cursor reads a
// `.mdc` rule's YAML frontmatter for its activation policy, so the header is
// Cursor chrome and lives here rather than in harness.DispatchInterior (which is
// the machine-neutral body every harness shares). `alwaysApply: true` keeps the
// routing rule in context for every request against the repository. The
// description is the reviewed wording the retired user-global rule carried; it
// names no personal-global path, so it reads correctly as a repository rule.
const dispatchRuleFrontmatter = "---\n" +
	"description: Docket agents must be dispatched, never run inline. Cursor runs a directly-invoked skill at the current model and outside its wrapper, dropping the wrapper's model/effort pin where one is set and its skill preload and isolation always — so force a dispatch to the matching docket subagent.\n" +
	"alwaysApply: true\n" +
	"---\n\n"

// DispatchRuleContent renders the compact Cursor dispatch rule as a whole `.mdc`
// file: the always-apply frontmatter above, then the shared machine-neutral
// dispatch interior (heading, compact routing rule, run gate) verbatim from
// harness.DispatchInterior. It emits NO per-agent definitions — a Cursor agent
// resolves the named docket subagents through its own registry, exactly as the
// compact rule instructs (change 0334) — so it needs only the run-gate payload,
// not the asset catalog. It replaces the retired global rule's assembler: a
// repository's `.cursor/rules`, not a personal global one, is where parent-facing
// routing now lives (change 0351), and internal/reposeed is its only caller.
func DispatchRuleContent(runGate []byte) []byte {
	return []byte(dispatchRuleFrontmatter + harness.DispatchInterior(runGate))
}

// renderAgent maps one agent source onto a Cursor custom-agent document,
// mirroring sync-agents.sh's emit_cursor_md exactly:
//
//	frontmatter name:        -> name
//	frontmatter description: -> description
//	resolved model + effort  -> model: <model>[effort=<effort>]
//	resolved model only      -> model: <model>
//	neither                  -> no model line
//	skills preamble + body   -> body
//
// Cursor documents five frontmatter fields and encodes reasoning effort INSIDE
// the model value, so there is no effort key: an effort resolved with no model
// has nowhere to go and is dropped. That drop is docket's own policy — docket
// refuses to pin an effort it cannot attribute to a resolved model — and it is
// silent here because an adapter is pure computation with no diagnostic
// stream; the shell emitter's WARN has no equivalent to write to. docket sets
// neither `readonly:` nor `is_background:`, leaving Cursor's own defaults.
func renderAgent(s harness.AgentSource, model, effort string) ([]byte, error) {
	var b strings.Builder
	b.WriteString("---\n")
	// name: is a bare scalar, not quoteYAML'd: Cursor builds its Task
	// subagent_type enum from this field's raw token, quotes included, so a
	// quoted name is rejected under its own unquoted spelling (ADR-0060).
	// The name is a docket-authored docket-* identifier, not free text, so
	// ADR-0071's always-quote rule does not reach it; as with the model line
	// below, the round-trip check is what makes "bare" safe — a name that
	// would not read back as itself is refused rather than shipped.
	b.WriteString("name: " + s.Name + "\n")
	b.WriteString("description: " + quoteYAML(s.Description) + "\n")
	// Model and effort are opaque vendor scalars (ADR-0015): docket keeps no
	// allowlist and passes whatever resolved through, bare. The round-trip
	// check below is what makes "bare" safe — a value that would not read back
	// as itself is refused rather than shipped.
	rendered := renderModel(model, effort)
	if rendered != "" {
		b.WriteString("model: " + rendered + "\n")
	}
	b.WriteString("---\n\n")

	body := strings.TrimRight(s.Body, "\n")
	if len(s.Skills) > 0 {
		body = fmt.Sprintf(skillsPreambleFormat, strings.Join(s.Skills, ", ")) + "\n\n" + body
	}
	// The self-recursion guard is its own paragraph at the one consistent
	// position every renderer uses: immediately after the frontmatter, ahead of
	// the body (skills preamble included) this renderer already emits.
	// harness.RecursionGuard is the shared emitter, so the paragraph is
	// byte-identical across all four harnesses.
	body = harness.RecursionGuard(s.Name) + "\n\n" + body
	b.WriteString(body + "\n")

	out := []byte(b.String())
	if err := verifyRoundTrip(out, s, rendered); err != nil {
		return nil, err
	}
	return out, nil
}

// renderModel is the whole model-line decision in one place: both fields, the
// model alone, or nothing at all.
func renderModel(model, effort string) string {
	switch {
	case model == "":
		return "" // an effort with no model is dropped
	case effort == "":
		return model
	default:
		return model + "[effort=" + effort + "]"
	}
}

// verifyRoundTrip reparses the rendered document and proves every field reads
// back as the value it was rendered from. Quoting correctness is thereby a
// checked property of each write rather than a claim about the escaping
// helper, which is what keeps a hostile description — or an unusual vendor
// model string — from shipping as a file that parses into something else.
func verifyRoundTrip(rendered []byte, s harness.AgentSource, model string) error {
	doc, err := document.Parse(rendered)
	if err != nil {
		return fmt.Errorf("%w: %s does not reparse: %w", ErrRender, s.Name, err)
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Model       string `yaml:"model"`
	}
	if err := doc.DecodeFrontmatter(&fm); err != nil {
		return fmt.Errorf("%w: %s does not decode: %w", ErrRender, s.Name, err)
	}
	got := struct {
		Name        string
		Description string
		Model       string
	}{fm.Name, fm.Description, fm.Model}
	want := got
	want.Name, want.Description, want.Model = s.Name, s.Description, model
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
