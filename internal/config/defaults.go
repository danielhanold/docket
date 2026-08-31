package config

// This file is the built-in layer: the lowest-precedence configuration docket
// ships with. Two halves live here — the scalar/list defaults, which mirror
// the registry's `def` cells (defaults_test.go proves they cannot drift), and
// the 17x4 agent table, which the registry deliberately does not carry because
// it is a table rather than a cell. The agent table is frozen against
// `agents/harness-defaults.yml` at commit a4d72613 (change 0324, which added the
// seventeenth agent docket-plan-writer), and its copy under
// `testdata/repositories/v0.9.3/` is the parity oracle.

// builtinProvenance is the provenance every default carries.
func builtinProvenance() Provenance {
	return Provenance{Layer: LayerBuiltIn, Source: "built-in"}
}

// builtinValue wraps a default in a Value: built-in provenance, never explicit.
func builtinValue[T any](v T) Value[T] {
	return Value[T]{Value: v, Provenance: builtinProvenance()}
}

// builtinEffective returns the complete default Effective. IntegrationBranch
// carries the raw `auto` sentinel here — resolution replaces it with the
// context's default branch. Explicit is false everywhere.
func builtinEffective() Effective {
	return Effective{
		// No metadata_branch: it is an obsolete tombstone (change 0363), never a
		// resolved default.
		IntegrationBranch: builtinValue("auto"),
		ChangesDir:        builtinValue("docs/changes"),
		ADRsDir:           builtinValue("docs/adrs"),
		ResultsDir:        builtinValue("docs/results"),
		Finalize: Finalize{
			Gate:              builtinValue("local"),
			TestCommand:       builtinValue("auto"),
			RequirePRApproval: builtinValue(false),
		},
		Learnings: Learnings{Enabled: builtinValue(true)},
		Reclaim: Reclaim{
			LeaseTTL: builtinValue(72),
			Auto:     builtinValue(false),
		},
		Review: Review{
			MinFixSeverity: builtinValue("minor"),
			MaxFixTasks:    builtinValue(10),
		},
		GateObservation: builtinValue(30),
		BoardSurfaces:   builtinValue([]string{"inline"}),
		Board: Board{
			SectionOrder: builtinValue(append([]string(nil), BoardSectionTokens...)),
			Sorting:      builtinBoardSorting(),
		},
		ChangeTypes: builtinValue([]string{"chore", "docs", "feat", "fix", "refactor", "perf"}),
		// No built-in default: an absent agent_harnesses is the touch-nothing
		// state, so the built-in value is a nil list that stays non-explicit.
		AgentHarnesses: builtinValue([]string(nil)),
		Agents:         builtinAgents(),
	}
}

// builtinBoardSorting is the built-in per-section sort table: every section
// sorts updated desc, keyed by BoardSectionTokens. A fresh map is built per
// call so a caller layering overrides can never write back into the defaults.
func builtinBoardSorting() map[string]BoardSort {
	m := make(map[string]BoardSort, len(BoardSectionTokens))
	for _, s := range BoardSectionTokens {
		m[s] = BoardSort{By: builtinValue("updated"), Direction: builtinValue("desc")}
	}
	return m
}

// agentDefault is one shipped model/effort pin, written the way
// `agents/harness-defaults.yml` writes it — `auto` and all.
type agentDefault struct {
	model  string
	effort string
}

// builtinAgentDefaults is Reference C verbatim: the 17 canonical agents of
// agentShortNames across the four shipped harnesses. Model IDs and effort
// tokens are opaque passthrough (ADR-0015) — docket keeps no vendor allowlist.
var builtinAgentDefaults = map[string]map[string]agentDefault{
	"claude": {
		"adr":                   {"claude-opus-5", "low"},
		"auto-groom":            {"claude-opus-5", "low"},
		"auto-groom-critic":     {"claude-opus-5", "medium"},
		"brainstorm-consultant": {"claude-opus-5", "medium"},
		"build-economy":         {"claude-sonnet-5", "low"},
		"build-standard":        {"claude-opus-5", "low"},
		"build-premium":         {"claude-opus-5", "medium"},
		"build-max":             {"claude-opus-5", "high"},
		"finalize-change":       {"claude-opus-5", "low"},
		"implement-next":        {"claude-opus-5", "medium"},
		"integration-repair":    {"claude-opus-5", "medium"},
		"plan-writer":           {"claude-opus-5", "high"},
		"rebase-resolver":       {"claude-opus-5", "medium"},
		"review-lean":           {"claude-sonnet-5", "high"},
		"review-standard":       {"claude-opus-5", "medium"},
		"review-deep":           {"claude-opus-5", "high"},
		"status":                {"claude-haiku-4-5-20251001", "medium"},
	},
	// Each cursor ID is a complete built-in whose variant is already encoded,
	// so every effort is `auto` — suppressed to "" in the resolved table.
	"cursor": {
		"adr":                   {"cursor-grok-4.5-high", "auto"},
		"auto-groom":            {"cursor-grok-4.5-medium", "auto"},
		"auto-groom-critic":     {"cursor-grok-4.5-high", "auto"},
		"brainstorm-consultant": {"cursor-grok-4.5-high", "auto"},
		"build-economy":         {"cursor-grok-4.5-low", "auto"},
		"build-standard":        {"cursor-grok-4.5-medium", "auto"},
		"build-premium":         {"cursor-grok-4.5-high", "auto"},
		"build-max":             {"claude-opus-5-high", "auto"},
		"finalize-change":       {"cursor-grok-4.5-high-fast", "auto"},
		"implement-next":        {"cursor-grok-4.5-high", "auto"},
		"integration-repair":    {"cursor-grok-4.5-high", "auto"},
		"plan-writer":           {"cursor-grok-4.5-xhigh", "auto"},
		"rebase-resolver":       {"cursor-grok-4.5-high", "auto"},
		"review-lean":           {"cursor-grok-4.5-medium", "auto"},
		"review-standard":       {"cursor-grok-4.5-high", "auto"},
		"review-deep":           {"claude-opus-5-high", "auto"},
		"status":                {"cursor-grok-4.5-low-fast", "auto"},
	},
	"codex": {
		"adr":                   {"gpt-5.6-terra", "xhigh"},
		"auto-groom":            {"gpt-5.6-sol", "low"},
		"auto-groom-critic":     {"gpt-5.6-sol", "medium"},
		"brainstorm-consultant": {"gpt-5.6-sol", "medium"},
		"build-economy":         {"gpt-5.6-luna", "xhigh"},
		"build-standard":        {"gpt-5.6-terra", "medium"},
		"build-premium":         {"gpt-5.6-sol", "low"},
		"build-max":             {"gpt-5.6-sol", "medium"},
		"finalize-change":       {"gpt-5.6-terra", "high"},
		"implement-next":        {"gpt-5.6-sol", "medium"},
		"integration-repair":    {"gpt-5.6-sol", "high"},
		"plan-writer":           {"gpt-5.6-terra", "high"},
		"rebase-resolver":       {"gpt-5.6-sol", "high"},
		"review-lean":           {"gpt-5.6-terra", "medium"},
		"review-standard":       {"gpt-5.6-terra", "high"},
		"review-deep":           {"gpt-5.6-sol", "medium"},
		"status":                {"gpt-5.6-luna", "xhigh"},
	},
	"opencode": {
		"adr":                   {"openrouter/moonshotai/kimi-k3", "medium"},
		"auto-groom":            {"openrouter/deepseek/deepseek-v4-flash-0731", "medium"},
		"auto-groom-critic":     {"openrouter/openai/gpt-5.6-luna", "high"},
		"brainstorm-consultant": {"openrouter/moonshotai/kimi-k3", "medium"},
		"build-economy":         {"openrouter/deepseek/deepseek-v4-flash-0731", "medium"},
		"build-standard":        {"openrouter/deepseek/deepseek-v4-flash-0731", "high"},
		"build-premium":         {"openrouter/moonshotai/kimi-k3", "medium"},
		"build-max":             {"openrouter/moonshotai/kimi-k3", "high"},
		"finalize-change":       {"openrouter/deepseek/deepseek-v4-flash-0731", "high"},
		"implement-next":        {"openrouter/deepseek/deepseek-v4-flash-0731", "high"},
		"integration-repair":    {"openrouter/moonshotai/kimi-k3", "high"},
		"plan-writer":           {"openrouter/deepseek/deepseek-v4-pro-0813", "medium"},
		"rebase-resolver":       {"openrouter/moonshotai/kimi-k3", "high"},
		"review-lean":           {"openrouter/deepseek/deepseek-v4-flash-0731", "high"},
		"review-standard":       {"openrouter/moonshotai/kimi-k3", "medium"},
		"review-deep":           {"openrouter/moonshotai/kimi-k3", "high"},
		"status":                {"openrouter/deepseek/deepseek-v4-flash-0731", "low"},
	},
}

// builtinAgents returns the full 17x4 table with `effort: auto` suppressed to
// "". A fresh table is built per call, so a caller layering global overrides
// on top can never write back into the shipped defaults.
func builtinAgents() AgentsTable {
	out := make(AgentsTable, len(builtinAgentDefaults))
	for harness, agents := range builtinAgentDefaults {
		row := make(map[string]AgentSetting, len(agents))
		for name, def := range agents {
			effort := def.effort
			if effort == "auto" {
				effort = ""
			}
			row[name] = AgentSetting{
				Model:  builtinValue(def.model),
				Effort: builtinValue(effort),
			}
		}
		out[harness] = row
	}
	return out
}
