<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0334 — Consolidate the docket dispatch block — subagent guard + single per-repo source](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0334-claude-md-global-and-project-copies-out-of-sync-on-finalize-a.md)**
<!-- docket:backlink:end -->

# Consolidate the docket dispatch block: subagent guard + single per-repo source — design (change 0334)

## Problem

The "Docket agents — dispatch, don't run inline" block instructs a session: when asked to run a
docket skill, dispatch the matching model/effort-pinned agent instead of executing the skill inline.
This is correct for a **top-level human session** — it is what preserves each agent's pinned model,
reasoning effort, dispatch contract, and skill preload (the whole point of the generated agent
layer, ADR-0016).

The block has **no subagent guard**. So when a docket-* agent is itself dispatched, the same block
is in *its* context, and the agent obeys it a second time — reading "when asked to run docket-status,
dispatch the docket-status agent" and dispatching **itself** again instead of executing its
preloaded skill. This recurses until some depth happens to run the skill inline.

Observed live during change 0317's four-harness fresh-session acceptance (Claude harness):

```
docket-status            (human dispatched the named agent)
└ docket-status          (re-dispatched, per agent-a9d7ed65 — its transcript carries its own
  └ docket-status         subagent_type:"docket-status" Task call)     (agent-a78d23de, same)
    └ …                  (agent-a51d5e2b finally ran preflight + the real status pass)
```

Two of the four `subagents/agent-*.jsonl` transcripts from that session each contain their own
`subagent_type:"docket-status"` dispatch before one child finally executed the skill. The recursion
is wasteful (N nested agents for one status read) and obscures the acceptance signal (the tree looks
like a runaway rather than a clean single dispatch).

There are **two independent trigger sites** for the same defect:

1. **The shared block template** (`sync-agents.sh`, the `## Docket agents — dispatch, don't run
   inline` heredoc, ~line 1793) is written into every synced repo's own `CLAUDE.md` via
   `claude_surface_target` (~line 1853) — and into `AGENTS.md` for the `codex`/`opencode` harnesses.
   This per-repo copy **also lacks the guard**, so a synced Claude repo (docket's own repo included)
   recurses too. This is the primary defect.
2. **A hand-authored global `~/.claude/CLAUDE.md` copy.** The 0317 acceptance fixture (`347-test`)
   was never `sync-agents`'d — it has no per-repo `CLAUDE.md` — so *only* the global copy was in
   play there. The maintainer dislikes carrying this rule globally, and the global copy is exactly
   the source of change 0334's original complaint: it drifts from the per-repo copy (the finalize
   agents `docket-integration-repair` / `docket-rebase-resolver` are described with stale wording
   globally vs. current wording in-repo).

**These two are one problem.** The drift 0334 first reported exists *because* there are two
hand-maintained copies. Eliminate the second copy — make the per-repo, sync-agents-generated block
the single source and retire the global one — and the drift cannot recur. Add the guard to that
single source and the recursion is fixed everywhere at once.

## Decision

Fold the recursion fix and the global-retirement into change 0334 as one "consolidate the dispatch
block" change.

### 1. Add an identity-based subagent guard to the shared block template

Edit the single shared heredoc in `sync-agents.sh` (the `## Docket agents — dispatch, don't run
inline` block, sole source for every render site) to open with an identity guard, mirroring
superpowers' `<SUBAGENT-STOP>` pattern:

> **If you are one of the `docket-*` agents listed below** — i.e. this instruction has reached you
> *inside* your own dispatched agent definition — then you have already been dispatched: execute
> your preloaded skill **inline** and do **NOT** re-dispatch. The dispatch rule below is for the
> **top-level session only.**

Detection is **identity-based prose**, not new machinery: a dispatched docket-* agent already knows
its own identity from its agent definition (its `name:` and preloaded `skills:`), the same basis on
which superpowers' subagents recognize themselves. No sentinel injection, no new wiring across the
17 generated wrappers.

The guard is content-only and machine-neutral (no model IDs), so it rides the existing block-writer
path unchanged and is delivered to every surface `sync-agents.sh` already writes.

### 2. Make the per-repo block the single source; retire the global copy

The delivery mechanism to Claude **already exists**: `claude_surface_target` unconditionally writes
the managed `docket:dispatch` block into each repo's `CLAUDE.md` (documented always-loaded surface),
and 0242's `CLAUDE.md → AGENTS.md` symlink covers AGENTS.md-native repos. So "deliver per-repo for
all harnesses including Claude" is largely built; this change **verifies** that a Claude-only repo
receives the guarded block and closes any gap found.

With the guarded block reliably delivered per-repo, docket **no longer expects the rule to live in
the global `~/.claude/CLAUDE.md`**. The maintainer removes the hand-authored global copy by hand
(docket cannot edit a user's personal global config, and must not try). Retiring it also resolves
0334's original drift: with one generated source, the finalize-agent descriptions cannot disagree
between a global and a project copy — there is no global copy.

### 3. Do NOT remove the rule outright

Deleting the block would make "run docket-status" execute the skill **inline at the session model**
(e.g. Opus) instead of via the pinned agent (e.g. haiku) — discarding the model pin, the dispatch
contract, and the skill preload the agent layer exists to enforce. The fix is a guard scoped to
depth ≥ 1, never a deletion.

## Consequences

- **Fixes** the recursion at every render site (per-repo `CLAUDE.md`/`AGENTS.md` and the
  global-equivalent) via one template edit — a docket-* agent dispatched by a human runs its skill
  inline instead of re-dispatching itself.
- **Resolves** 0334's original global/project drift by construction: one generated source, no second
  hand-maintained copy to drift.
- **Preserves** the model-pin/dispatch-contract behavior for top-level human sessions (the reason
  the block exists).
- **Requires a one-time manual step** from the maintainer: delete the global `~/.claude/CLAUDE.md`
  dispatch block after confirming the per-repo guarded block is delivered. Docket cannot and must
  not automate edits to a user's personal global config.
- **Verification cost:** the guard's effect is behavioral (an agent's choice not to re-dispatch),
  which no in-repo unit test can assert directly. The template edit itself is testable (the block
  contains the guard clause; markers balance; block is machine-neutral). The behavioral proof is a
  fresh-session dispatch showing a single-level tree — the same external-truth acceptance shape that
  surfaced the bug; fold that check into the 0317-style harness procedure rather than a unit test.

## Open questions

- Does a Claude-**only** repo (no codex/opencode) actually receive the block today via
  `claude_surface_target`, or is there a gap to close? Build must verify against a fresh synced
  Claude-only fixture, not assume.
- Exact placement of the guard clause within the block — first line before the roster, so it is read
  before the "dispatch the matching agent" instruction it qualifies.
- Whether any existing `sync-agents.sh` test (`test_sync_agents_*`) should assert the guard clause is
  present in the emitted block (shape assertion, not behavior) to prevent silent removal.
