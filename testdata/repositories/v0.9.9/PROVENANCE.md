# v0.9.9 agent-defaults sidecar

Source: docket's own `agents/harness-defaults.yml` as of change 0473 (rename build profile and
review rung to tiers, and the dispatch fallbacks), 2026-09-30.

Cut from `v0.9.3/agents-harness-defaults.yml` by change 0473. The ONLY file in this tree is
`agents-harness-defaults.yml`, a byte-exact copy of `agents/harness-defaults.yml` at that change.
It differs from the v0.9.3 sidecar in comments only: the build-profile and review-rung vocabulary
moves to "tier" (ADR-0129) — "review rungs" becomes "review tiers", "the cap rung" becomes "the
cap tier", "the lean rung" becomes "the lean tier", "max build profile" becomes "max build tier",
"The profile names" becomes "The tier names", the codex ladder's economy/standard/premium/max
"rung"s become "tier"s, and the opencode block's "build rungs" / "lean review rung" become "build
tiers" / "lean review tier". No key or value changed, so the Go built-in agent table in
`internal/config/defaults.go` is unchanged. Only the agent-defaults parity oracle's `sidecarPath`
advances to this tree; every other frozen reader stays where it was. Older versioned fixture trees
(v0.9.2–v0.9.8, including the v0.9.3 sidecar and its status corpus) are immutable inputs and are
never edited in place.
