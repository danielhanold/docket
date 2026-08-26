// Package cursor renders Docket's installation for Cursor: skill symlinks,
// one native custom-agent document per agent source, and the dedicated
// dispatch rule at ~/.cursor/rules/docket-dispatch.mdc. It plans only —
// nothing here writes to the filesystem apart from Detect's read-only stat.
package cursor

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
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

	// The authored dispatch payloads, matched by path below the dispatch
	// asset root: a static head, one fragment per agent, and the run gate.
	dispatchRoot     = "cursor-rules"
	dispatchHeadPath = dispatchRoot + "/dispatch.head.md"
	fragmentDir      = dispatchRoot + "/dispatch"

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
	rule, err := assembleDispatchRule(in.Assets, sources)
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

	// Cursor's dispatch surface is a rule file docket owns whole, not a
	// managed block inside a file the user also writes — so it is a KindFile
	// target, and its bytes are the authored payloads passed straight through
	// rather than harness.DispatchInterior's machine-neutral roster.
	targets = append(targets, install.Target{
		Path:    filepath.Join(root, rulesDir, dispatchFile),
		Kind:    install.KindFile,
		Content: rule,
		Role:    roleDispatch,
	})

	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	return targets, nil
}

// assembleDispatchRule concatenates the authored dispatch payloads: the static
// head, every per-agent fragment in sorted order, then the run gate. Every
// payload is opaque — it is Cursor-specific prose docket authors in the bundle
// and never re-derives here, so the rule ships exactly what was reviewed.
//
// Each agent source must have a fragment and each fragment an agent source. A
// missing fragment would ship a rule that leaves one agent silently
// un-dispatched, which is precisely the failure the rule exists to prevent, so
// it is an error rather than a generated stand-in.
func assembleDispatchRule(c assets.Catalog, sources []harness.AgentSource) ([]byte, error) {
	// Every payload is located through the manifest, never fetched by a
	// hard-coded path alone: the manifest is what says the bundle carries it,
	// and a byte reader that happens to answer for an unlisted path would let
	// an incomplete bundle assemble a rule anyway.
	fragments := map[string]bool{}
	hasHead := false
	for _, e := range c.EntriesByRole(assets.RoleDispatch) {
		switch {
		case e.Path == dispatchHeadPath:
			hasHead = true
		case path.Dir(e.Path) == fragmentDir:
			fragments[path.Base(e.Path)] = true
		}
	}
	if !hasHead {
		return nil, fmt.Errorf("%w: the asset bundle carries no %s", ErrRender, dispatchHeadPath)
	}
	head, err := c.Bytes(dispatchHeadPath)
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %w", ErrRender, dispatchHeadPath, err)
	}
	runGate, err := harness.RunGate(c)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.Write(head)
	// Sources arrive in ParseInventory's ascending short-name order, which is
	// the fragments' sorted order too: the fragment basename is the agent name
	// and the agent name is the prefixed short name.
	for _, s := range sources {
		base := s.Name + ".md"
		if !fragments[base] {
			return nil, fmt.Errorf("%w: the asset bundle carries no dispatch fragment %s/%s for agent %s", ErrRender, fragmentDir, base, s.Name)
		}
		delete(fragments, base)
		frag, err := c.Bytes(fragmentDir + "/" + base)
		if err != nil {
			return nil, fmt.Errorf("%w: reading dispatch fragment %s: %w", ErrRender, base, err)
		}
		b.WriteString("\n" + strings.TrimRight(string(frag), "\n") + "\n")
	}
	if len(fragments) != 0 {
		orphans := make([]string, 0, len(fragments))
		for name := range fragments {
			orphans = append(orphans, name)
		}
		sort.Strings(orphans)
		return nil, fmt.Errorf("%w: dispatch fragments with no agent source: %s", ErrRender, strings.Join(orphans, ", "))
	}
	b.WriteString("\n" + strings.TrimRight(string(runGate), "\n") + "\n")
	return []byte(b.String()), nil
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
