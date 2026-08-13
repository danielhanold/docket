package harness

import (
	"fmt"
	"path"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
)

// DispatchHeading opens every dispatch surface docket writes. It is the anchor
// a reader — and the managed-block inspection — sees first.
const DispatchHeading = "## Docket agents — dispatch, don't run inline"

// dispatchPreamble is the machine-neutral statement of the dispatch rule.
//
// It names NO harness. The same interior is written into Claude's CLAUDE.md,
// Codex's AGENTS.md, and OpenCode's AGENTS.md, so any sentence naming one
// harness's binary, root, or model vocabulary would be a false claim on the
// other two — and each adapter's own tests assert that its artifacts never name
// a sibling harness. sync-agents.sh's assemble_agents_md_dispatch interpolates
// the shipped-harness roster into this paragraph because its block is committed
// into a consumer repo and read by humans; here the roster is replaced by the
// equivalent quantified claim, which stays true as the roster grows.
const dispatchPreamble = "Docket generates an agent definition per docket skill in your harness's own agents directory. When\n" +
	"you are asked to run one of the docket skills below, run the matching **agent** instead of executing\n" +
	"the skill inline at the session model: the agent carries that skill's dispatch contract, its skill\n" +
	"preload, and whatever model and reasoning effort your config layers pin for it. Docket ships a\n" +
	"validated model and reasoning effort for every one of these agents on every harness it ships\n" +
	"defaults for, so they are pinned out of the box there; your config layers override either field per\n" +
	"agent, and set them for any other harness. Dispatch through the hosting harness's native\n" +
	"named-agent dispatch either way — the pin is not the only reason, since the agent also carries the\n" +
	"skill's dispatch contract and preload. Pass the request through unchanged, including any change or\n" +
	"ADR id."

// RunGateAsset is the basename of the dispatch-role payload carrying the run
// gate. It is matched by basename rather than by full path so the bundle's root
// layout can move without every adapter learning the new spelling.
const RunGateAsset = "run-gate.md"

// RunGate returns the run-gate payload from the catalog. A bundle without one
// is an error rather than an empty tail: a dispatch surface silently missing
// its gate is exactly the failure the gate exists to prevent.
func RunGate(c assets.Catalog) ([]byte, error) {
	for _, e := range c.EntriesByRole(assets.RoleDispatch) {
		if path.Base(e.Path) != RunGateAsset {
			continue
		}
		body, err := c.Bytes(e.Path)
		if err != nil {
			return nil, fmt.Errorf("harness: reading the run-gate payload %s: %w", e.Path, err)
		}
		return body, nil
	}
	return nil, fmt.Errorf("harness: the asset bundle carries no %s dispatch payload", RunGateAsset)
}

// DispatchInterior renders the managed-block interior every dispatch surface
// shares: the heading, the preamble, one bullet per agent in inventory order,
// then the run-gate payload verbatim.
//
// The roster follows its own preamble and the gate comes after both. Order is
// structure in a headed markdown document: with the gate's `##` heading spliced
// between the preamble and the bullets, the preamble's "the docket skills
// below" would point across an unrelated section.
//
// The bullets are generated from sources — never from a second name list — so
// an agent added to the bundle reaches every dispatch surface without an edit
// here. Their order is ParseInventory's (short name, ascending).
func DispatchInterior(sources []AgentSource, runGate []byte) string {
	var b strings.Builder
	b.WriteString(DispatchHeading + "\n\n")
	b.WriteString(dispatchPreamble + "\n\n")
	for _, s := range sources {
		fmt.Fprintf(&b, "- **%s** — %s Delegate to the `%s` agent.\n", s.Name, s.Description, s.Name)
	}
	b.WriteString("\n")
	// One trailing newline, whatever the payload carries: the interior digest
	// normalizes a trailing newline away, so this is presentation only, but a
	// stable spelling keeps the frozen goldens honest.
	b.WriteString(strings.TrimRight(string(runGate), "\n") + "\n")
	return b.String()
}
