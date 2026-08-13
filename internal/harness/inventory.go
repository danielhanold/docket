package harness

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
)

// AgentSource is one parsed agents/docket-*.md — the shared input every
// harness renderer maps into its own native shape. Every adapter reads this
// inventory rather than carrying a name list of its own, so adding an agent
// source to the bundle propagates without touching adapter code.
type AgentSource struct {
	ShortName   string   // "build-standard"
	Name        string   // "docket-build-standard"
	Description string   //
	Skills      []string // frontmatter `skills:` flow list; nil when the agent preloads none
	Body        string   // markdown body after the frontmatter, verbatim
}

// agentFrontmatter is the decode target: the fields docket owns. Unknown keys
// (`worktree-scope`, whatever a newer docket adds) decode away silently, which
// is internal/document's documented compatibility posture.
type agentFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Skills      []string `yaml:"skills"`
}

// The prefix every agent definition's name carries. The short name is what the
// config agents table and the harness-defaults bundle key on.
const agentNamePrefix = "docket-"

// ParseInventory decodes every RoleAgentSource entry in the catalog, sorted by
// ShortName. An entry that fails to parse is an error, never a skip: a bundle
// carrying an agent source docket cannot read is a corrupt bundle, and silently
// planning without that agent would install an inventory that disagrees with
// the dispatch material generated from it.
func ParseInventory(c assets.Catalog) ([]AgentSource, error) {
	entries := c.EntriesByRole(assets.RoleAgentSource)
	sources := make([]AgentSource, 0, len(entries))
	for _, e := range entries {
		s, err := parseAgentSource(c, e.Path)
		if err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ShortName < sources[j].ShortName })
	for i := 1; i < len(sources); i++ {
		if sources[i].ShortName == sources[i-1].ShortName {
			return nil, fmt.Errorf("harness: duplicate agent short name %q in the asset catalog", sources[i].ShortName)
		}
	}
	return sources, nil
}

func parseAgentSource(c assets.Catalog, p string) (AgentSource, error) {
	body, err := c.Bytes(p)
	if err != nil {
		return AgentSource{}, fmt.Errorf("harness: reading agent source %s: %w", p, err)
	}
	doc, err := document.Parse(body)
	if err != nil {
		return AgentSource{}, fmt.Errorf("harness: parsing agent source %s: %w", p, err)
	}
	if !doc.HasFrontmatter() {
		return AgentSource{}, fmt.Errorf("harness: agent source %s has no frontmatter", p)
	}
	var fm agentFrontmatter
	if err := doc.DecodeFrontmatter(&fm); err != nil {
		return AgentSource{}, fmt.Errorf("harness: decoding agent source %s: %w", p, err)
	}
	if fm.Name == "" {
		return AgentSource{}, fmt.Errorf("harness: agent source %s declares no name", p)
	}
	if fm.Description == "" {
		return AgentSource{}, fmt.Errorf("harness: agent source %s declares no description", p)
	}
	if !strings.HasPrefix(fm.Name, agentNamePrefix) {
		return AgentSource{}, fmt.Errorf("harness: agent source %s names %q, which does not start with %q", p, fm.Name, agentNamePrefix)
	}
	if base := path.Base(p); base != fm.Name+".md" {
		return AgentSource{}, fmt.Errorf("harness: agent source %s names %q, which disagrees with its filename %s", p, fm.Name, base)
	}
	text, err := bodyAfterFrontmatter(doc)
	if err != nil {
		return AgentSource{}, fmt.Errorf("harness: agent source %s: %w", p, err)
	}
	return AgentSource{
		ShortName:   strings.TrimPrefix(fm.Name, agentNamePrefix),
		Name:        fm.Name,
		Description: fm.Description,
		Skills:      fm.Skills,
		Body:        text,
	}, nil
}

// bodyAfterFrontmatter returns everything after the closing frontmatter fence.
// internal/document exposes the parsed frontmatter and the managed blocks but
// no body span, so the split is recomputed here over the same rule the parse
// already validated: the opening fence is the first line, and the body starts
// after the next line that is exactly `---`.
func bodyAfterFrontmatter(d document.Document) (string, error) {
	src := d.Source()
	if !d.HasFrontmatter() {
		return string(src), nil
	}
	offset := 0
	first := true
	for offset < len(src) {
		end := len(src)
		if i := bytes.IndexByte(src[offset:], '\n'); i >= 0 {
			end = offset + i + 1
		}
		line := strings.TrimRight(string(src[offset:end]), "\r\n")
		offset = end
		if first {
			first = false
			continue // the opening fence
		}
		if line == "---" {
			return string(src[offset:]), nil
		}
	}
	return "", fmt.Errorf("frontmatter has no closing fence")
}

// SkillDirs returns the sorted, deduplicated top-level skill directory names in
// the catalog ("docket-build", …). An entry that names no directory below the
// skills root (a stray file directly under it) contributes nothing.
func SkillDirs(c assets.Catalog) []string {
	seen := map[string]bool{}
	for _, e := range c.EntriesByRole(assets.RoleSkill) {
		parts := strings.Split(path.Clean(e.Path), "/")
		if len(parts) < 3 {
			// parts is <root>/<file> at best: no skill directory to link.
			continue
		}
		seen[parts[1]] = true
	}
	out := make([]string, 0, len(seen))
	for dir := range seen {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}

// The no-pin sentinels docket writes rather than a vendor value: `auto` means
// "do not emit an effort line" on every harness, and `inherit` means "do not
// emit a model line" on every harness EXCEPT claude — see ResolvedAgentRaw.
// config.Resolve already folds `auto` to "", so the effort case here is
// belt-and-braces for a table built by hand.
const (
	modelInheritSentinel = "inherit"
	effortAutoSentinel   = "auto"
)

// ResolvedAgentRaw returns the configured model VERBATIM — `inherit`
// included — with only the effort `auto` sentinel folded to "". It exists for
// the one harness where `inherit` is not docket's sentinel: Claude Code
// documents `model: inherit` as a real frontmatter value meaning "run this
// subagent on the parent conversation's model", which is a different runtime
// outcome from omitting the key (Claude Code's own subagent default). The
// effort beside such a model is NOT dropped, mirroring sync-agents.sh's emit(),
// which normalizes `auto` and nothing else.
//
// Every other adapter calls ResolvedAgent instead. That asymmetry is
// deliberate: folding it away is what the 0168 whole-branch review caught
// (IMPORTANT 2) in the shell implementation, and sync-agents.sh still records
// it in the comment above emit().
//
// An unknown harness or agent resolves to no pin at all rather than an error: a
// sparse table is the documented shape.
func ResolvedAgentRaw(t config.AgentsTable, harnessName, shortName string) (model, effort string) {
	setting, ok := t[harnessName][shortName]
	if !ok {
		return "", ""
	}
	model, effort = setting.Model.Value, setting.Effort.Value
	if effort == effortAutoSentinel {
		effort = ""
	}
	return model, effort
}

// ResolvedAgent returns the model and effort a harness should render for one
// agent short name, with both of docket's no-pin sentinels normalized to "".
// This is the resolver for harnesses that have no `inherit` value of their own
// (codex, cursor, opencode); claude uses ResolvedAgentRaw.
func ResolvedAgent(t config.AgentsTable, harnessName, shortName string) (model, effort string) {
	model, effort = ResolvedAgentRaw(t, harnessName, shortName)
	if model == modelInheritSentinel {
		model = ""
	}
	return model, effort
}
