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

// dispatchPreamble is the machine-neutral routing rule.
//
// It names NO harness. The same interior is written into Claude's CLAUDE.md,
// Codex's AGENTS.md, and OpenCode's AGENTS.md, so any sentence naming one
// harness's binary, root, or model vocabulary would be a false claim on the
// other two — and each adapter's own tests assert that its artifacts never name
// a sibling harness.
//
// Change 0334 dropped the per-agent roster (and, with it, sync-agents.sh's
// interpolated shipped-harness list) from this block: the compact rule below
// defers to the harness's own agent registry instead of restating it. Removing
// the list also retired the one deliberate Go/shell emitter variance — the
// shell mirror used to interpolate HD_SHIPPED_HARNESSES here where this constant
// stated the equivalent quantified claim, so the two generators emitted subtly
// different prose (learnings: consolidation-flattens-caller-variance — the
// divergence was deliberate and was retired, not overlooked).
//
// Change 0371 made this Go emitter the single canonical dispatch policy and
// appended the never-fall-back sentence. The frozen shell mirror (sync-agents.sh)
// that once carried a textual twin of this preamble — and rendered the committed
// AGENTS.md dispatch block from it — was deleted with the frozen facade by change
// 0370, so this Go emitter is now the sole source and the committed AGENTS.md is
// rendered from it.
//
// The machine-neutral rationale above still holds and Task 2's cross-surface
// guards depend on the property it documents: this interior names NO harness, so
// the identical bytes can land on every host surface.
const dispatchPreamble = "When a requested Docket workflow has a registered same-name `docket-*` agent, dispatch that agent\n" +
	"instead of running the workflow inline: the agent carries that workflow's dispatch contract, its\n" +
	"skill preload, and whatever model and reasoning effort your config layers pin for it. Your\n" +
	"harness's native agent registry is authoritative for agent names, descriptions, and availability —\n" +
	"this block does not restate it. If no same-name agent is registered, do not invent one; follow the\n" +
	"workflow's own inline or unavailable-capability contract. Dispatch through the harness's native\n" +
	"named-agent dispatch, and pass the request through unchanged, including any change or ADR id.\n" +
	"Never reroute a registered workflow through a shell runner, another harness, a generic agent, or\n" +
	"an inline reconstruction of its contract — a missing registration is a visible capability\n" +
	"failure, not a fallback trigger."

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
// shares: the heading, the compact routing rule, a blank line, then the
// run-gate payload verbatim.
//
// The rule precedes the run gate's `##` heading so the two sections read as
// siblings — order is structure in a headed markdown document. The block no
// longer lists the agent roster: an agent added to the bundle reaches every
// dispatch surface through the harness's own agent registry, which this rule
// defers to rather than restating (change 0334), so the interior no longer
// depends on the inventory at all.
func DispatchInterior(runGate []byte) string {
	var b strings.Builder
	b.WriteString(DispatchHeading + "\n\n")
	b.WriteString(dispatchPreamble + "\n\n")
	// One trailing newline, whatever the payload carries: the interior digest
	// normalizes a trailing newline away, so this is presentation only, but a
	// stable spelling keeps the frozen goldens honest.
	b.WriteString(strings.TrimRight(string(runGate), "\n") + "\n")
	return b.String()
}

// CodexRootEntryClause is appended only to repository AGENTS.md surfaces for
// repos that opt into Codex. Registered description markers expose the closed
// root, feature, and metadata launch routes without a role-name roster.
const CodexRootEntryClause = "### Codex root-coordinator entry\n\n" +
	"For Codex, every registered Docket role uses the harness's top-level native named-agent dispatch. Description markers remain typed role metadata; `[docket launch: root-coordinator]` and `[docket worktree: feature]` do not select `agent.enter` or another root process. Pass the request and immutable assignment unchanged, including any resume or continuation id and dispatch-context token required by the child contract. Pass the run epoch id to `docket-implement-next` so its scoped child dispatches can bind to the outer run. Keep the caller's gate key and parent capability private.\n\n" +
	"A native dispatch yield carrying a live child identity is a liveness transition, not completion. Retain that exact identity and collect its terminal output through the harness-native observation/wait control. Never launch a replacement watcher or return a completion report while the original child remains live or unobserved. Only after terminal return may implement-next run the parent's keyed `run.gate-verdict` and obey its report. Coordinator prose, notification text, child success text, and process exit alone do not prove gate ownership or completion. If native dispatch or the registration is unavailable, report the configuration failure; do not substitute `agent.enter`, `codex exec`, another harness, a generic agent, or a parent relay."

func CodexDispatchInterior(runGate []byte) string {
	return DispatchInterior(runGate) + "\n" + CodexRootEntryClause + "\n"
}
