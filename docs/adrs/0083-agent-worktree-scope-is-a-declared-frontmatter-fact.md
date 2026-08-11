---
id: 83
slug: agent-worktree-scope-is-a-declared-frontmatter-fact
title: An agent's worktree scope is a declared frontmatter fact, not a name pattern
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [34, 68]
change: 208
---

## Context

Change 0206 gave `scripts/runner-dispatch.sh` a gate requiring `--worktree` for `build-*` agents, and
`sync-agents.sh`'s `emit_shim` a matching required slot. Both keyed on the agent-name shape
`build-*`, which enumerated exactly one family.

Three more delegatable families are equally feature-scoped and match no build shape:
`docket-rebase-resolver` (runs `git add` and `git rebase --continue` mid-rebase),
`docket-integration-repair` (writes and commits a fix), and the three `docket-review-*` rungs
(read-only, but a wrong tree means findings about the wrong diff, silently). A `runner:` on any of
them yielded exactly the silent main-tree-on-the-integration-branch anchor the gate exists to
eliminate, under whatever auto-approve grant the runner carries.

The obvious repair — a second name list — would have been two case statements in two scripts,
drifting against each other from the day the next agent shipped.

## Decision

An agent's worktree scope is a **declared fact in its source frontmatter**, and both delegation
gates key on the declaration rather than on any name pattern.

- Every built-in agent source `agents/docket-*.md` carries a required frontmatter key
  `worktree-scope: feature|metadata`. Nine are `feature` (the four build profiles,
  `rebase-resolver`, `integration-repair`, the three review rungs); seven are `metadata`.
- `sync-agents.sh` validates the key on every source it processes and refuses generation loudly when
  it is absent or invalid. Generation is where a new agent gets wired, so it is the seam at which an
  undeclared agent is still preventable. `emit_shim`'s required `--worktree` slot keys on the
  declaration.
- `scripts/runner-dispatch.sh` reads the same key at runtime through a
  `${DOCKET_AGENTS_SRC:-$SELF_DIR/../agents}` seam and gates on it: `feature` makes `--worktree`
  required and additionally rejects an anchor equal to the main worktree. The read is tolerant
  per-file — a missing file or key is metadata scope, so the adapter keeps its more specific
  unknown-agent diagnostic — but refuses loudly when the sources **directory** itself is missing or
  holds no agent sources, since a misdirected seam would otherwise silently disarm both gates for
  every agent.
- Both readers share one anchored implementation, `scripts/lib/docket-agent-scope.sh`, which parses
  only the first `---…---` frontmatter block. That lib deliberately does not delegate to
  `docket-frontmatter.sh`'s `fm_field`: `sync-agents.sh` must run under macOS system Bash 3.2, where
  that file's `declare -gA` aborts at source time.
- One deliberate exception remains. The empty-payload refusal (`a build-* dispatch carries no task`)
  stays keyed on `build-*`, because its reasoning is build-specific. `validate_agent_scopes`
  therefore also fails a `docket-build-*` source declaring anything but `feature` — a consistency
  bond between the two readings, not a new enumerated floor.

## Consequences

A future feature-scoped agent cannot ship ungated by failing to match a pattern: the gate is keyed
on something the agent's author must write down, and generation refuses when they did not. Adding
the fifth or sixth feature-scoped family costs a frontmatter line, not an edit to two case statements
in two scripts.

The declaration also flows verbatim into every generated Claude wrapper. That is harmless, but
wrapper bytes changed, so an existing install must re-run `sync-agents.sh` once before `--check`
drift assertions settle.

A residual stands: a **linked** worktree checked out on the integration branch is not caught. No
branch predicate is viable here, because `rebase-resolver` dispatches mid-rebase on a detached HEAD.
The gate rejects the main worktree by path and requires repo containment; branch state remains
outside what it can assert.
