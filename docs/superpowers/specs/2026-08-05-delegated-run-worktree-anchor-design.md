<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0206 — Delegated runner runs are anchored at the main worktree, not the feature worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0206-delegated-runner-runs-are-anchored-at-the-main-worktree-not.md)**
<!-- docket:backlink:end -->

# Delegated run anchor — an explicit argument with a main-worktree default

## Context

`runner-dispatch.sh` anchors every delegated run at the repo's main worktree:

```sh
REPO_ROOT="$(docket_main_worktree)"
export DOCKET_REPO_ROOT="$REPO_ROOT"
```

Deliberately cwd-independent per ADR-0034, so it returns the repo's primary checkout even when the
caller stands in `.docket/` or `.worktrees/<slug>`. Each adapter hands that path to the child
harness — `codex exec -C "$DOCKET_REPO_ROOT"`, `cursor` likewise, and 0205's
`opencode run --dir "$DOCKET_REPO_ROOT"`.

That anchor is correct for the agents delegation has shipped for until now. `docket-status` and
`docket-adr` are metadata-scoped: they operate on `metadata_branch` through the `.docket/` worktree
and belong in the main tree. It is wrong for a build worker. The `docket-build-task` contract
requires the worker to stay **inside the feature worktree, on its branch**, performing no docket
metadata operations. A delegated `build-economy` therefore starts in the main tree with the
integration branch checked out, holding whatever permission grant its runner was configured with
(`--auto` under opencode, `workspace-write` under codex), and the only thing pointing it at the
correct tree is prose in the relayed prompt.

Two observations sharpen the failure mode:

1. Feature worktrees live at `<repo>/.worktrees/<slug>` — *inside* the main worktree. Codex's
   `--sandbox workspace-write` rooted at `$DOCKET_REPO_ROOT` therefore already permits writes to
   the feature tree. **This is not a permission failure.**
2. The exposure is the **starting cwd and the checked-out branch**. A worker that does not
   faithfully follow the prompt's worktree instruction commits code onto the integration branch in
   the shared primary checkout, unattended.

The mismatch predates 0205 — it is a property of the 0079 delegation framework — but only becomes
reachable now, because 0205 ships the first documented recipe that delegates the four
`docket-build-*` profile workers to a child harness.

## Decision

A delegated run's anchor is an **explicit argument whose default is the main worktree**; the
delegated agent's scope decides which. Metadata-scoped agents keep the main worktree implicitly.
Feature-scoped agents must name their tree, and the facade refuses to run one that did not.

ADR-0034 stands **unamended**. Nothing resolves an anchor from the caller's CWD; the main worktree
remains the default; the only way off it is an argument someone deliberately wrote.

### Rejected alternatives

**Resolve the caller's CWD** (the stub's first option — use CWD when inside the repo, main worktree
as fallback). Rejected on two independent grounds. It reverses ADR-0034's central rule, that a
stray `cd` must never misdirect a script. And it is unreliable at the one moment it matters: the
shim runs as a harness agent making a single Bash call, whose cwd is the *session's* cwd — which
persists across calls and is whatever the last `cd` left behind, not reliably the feature worktree.

**Derive the path inside the facade** from docket state (the change's `branch:` field →
`git worktree list`). Rejected: the facade is deliberately docket-metadata-ignorant, and it does
not know which change is being built.

**Forbid delegating `build-*` agents**, or document the limitation loudly and change no code.
Rejected: selective build delegation is the stated motivation of 0205 (cheap build workers, review
native), and documenting a reachable unattended-commit-to-main path is not a fix.

**Require `--worktree` for every delegation.** Rejected: it breaks every shipped `status`/`adr`
shim and does amend ADR-0034, for no gain over requiring it where scope demands it.

## Design

### 1. Facade — `scripts/runner-dispatch.sh`

A new optional flag, parsed alongside the existing four:

```sh
--worktree <path>
```

The anchor becomes:

```sh
ANCHOR="$(docket_anchor_path "${WORKTREE:-.}")"
export DOCKET_REPO_ROOT="$ANCHOR"
```

Routing through `docket_anchor_path` rather than passing the argument through raw is deliberate: a
relative `--worktree .worktrees/<slug>` joins to the **main worktree**, so it resolves identically
from any cwd. The new argument inherits ADR-0034's cwd-independence instead of quietly
reintroducing the hazard ADR-0034 was written against. An absolute path passes through untouched.
With `--worktree` absent the expression is `docket_anchor_path "."`, which is the main worktree —
byte-identical behavior to today for every currently-shipped shim.

Three gates, all **loud** (`die`, abort-and-report). This matches the facade's posture for an
unknown `--runner`, not its tolerant posture for an unparseable `runners.<name>:` config value:
that tolerance exists because a cosmetic config typo must not fail a live dispatch, whereas each
of these represents a request the facade cannot serve correctly.

| Condition | Diagnostic |
|---|---|
| `--agent build-*` and `--worktree` empty | `--worktree is required for build-* agents (a build worker must run in its feature worktree, not the main tree)` |
| resolved anchor is not a directory | `--worktree <path> is not a directory` |
| resolved anchor's main worktree ≠ this repo's main worktree | `--worktree <path> is not a worktree of this repository` |

The third gate catches a path outside the repo set, which would otherwise hand a child harness —
running under an auto-approve permission grant — a tree docket does not own. It is implemented as
`docket_main_worktree "$ANCHOR"` compared against `docket_main_worktree`; the empty result from a
non-repo path fails the comparison, so the not-a-repo case is covered by the same check.

The `build-*` gate is the one piece of agent-family knowledge the facade gains. It is a runtime
requirement — the worktree path is runtime data — so generation-time enforcement in
`sync-agents.sh` cannot substitute for it. One `case` statement is the whole cost.

Order: parse → existing validation (required flags, runner-name traversal guard, adapter exists) →
anchor resolution → the three gates → config resolution → handoff. The gates sit before the
`runners.<name>:` read so an anchor failure is reported without depending on config parsing.

### 2. Adapters — contracts only

`codex.sh`, `cursor.sh`, and 0205's `opencode.sh` are **unchanged**. Each keeps reading
`DOCKET_REPO_ROOT` verbatim into its own directory flag. That the facade owns the anchor is
precisely what makes this cost three adapters nothing.

Each adapter contract's env table changes one row, from *"absolute main-worktree path"* to
*"absolute run anchor — the main worktree unless the caller named a feature worktree"*, so a
contributor reading `codex.md` alone does not conclude the value is always the primary checkout.

`runner-dispatch.md` gains `--worktree` to its Usage block, a Behavior step 2 rewrite covering the
default and the three gates, and an Invariants entry: *the anchor is never resolved from the
caller's CWD; absent `--worktree` it is the main worktree.*

### 3. Callers — `sync-agents.sh`'s `emit_shim`

`emit_shim` receives the agent name (`$5`), so the requirement is **baked into each generated
file** rather than left to generic prose that applies to every shim equally. For a `build-*`
agent the emitted dispatch line carries the flag as a required slot:

```
docket.sh runner-dispatch --runner <r> --agent build-economy --worktree <feature worktree> [-- <caller args>]
```

with an accompanying instruction: take the path from the feature worktree your caller named; if
your caller did not name one, abort-and-report rather than guessing or omitting the flag. Shims
for every other agent are byte-identical to today — no flag, no instruction.

`skills/docket-build/SKILL.md`'s *Dispatching a task* section gains one sentence: a delegated
worker receives its worktree through the `--worktree` flag, not only through the prompt body.

This does leave the *value* prose-supplied, one level up. The facade's `build-*` gate is what
makes that acceptable: an omission is now a loud abort, where today it is a silent main-tree run
on the integration branch.

### 4. Ledger

A new ADR records the framework rule — *a delegated run's anchor is an explicit argument defaulting
to the main worktree; the delegated agent's scope decides which* — with `relates_to: [34]`. It is a
rule future adapters and future delegated agents need, and ADR-0034 does not cover it.

ADR-0034 additionally receives a dated `## Update` note pointing at the new ADR, so a reader who
lands on the repo-root anchor decision learns the delegation exception exists. This is a
non-reversing context change, which the convention delivers as an `## Update`, never an edit to the
decision body.

Both ids go in this change's `adrs:` so they land atomically — per the `adr-update-delivery`
learning, an ADR body update is delivered by listing that ADR id in the producing change's
frontmatter, never as a standalone push.

### 5. Testing

`tests/test_runner_dispatch.sh` extends, using the existing `RUNNERS_DIR` and `GIT` mock seams:

- `--worktree` absent → anchor is the main worktree (regression guard on today's behavior).
- `--worktree` with an absolute feature-worktree path → that path becomes `DOCKET_REPO_ROOT`.
- `--worktree` with a **relative** path, invoked from a foreign cwd → resolves against the main
  worktree, proving the new argument did not reintroduce CWD dependence. This is the assert that
  distinguishes this design from the rejected CWD-resolution option, so it is the one that must
  not be dropped.
- `--agent build-economy` with no `--worktree` → nonzero, diagnostic names the flag.
- `--worktree` pointing at a non-directory → nonzero.
- `--worktree` pointing at a directory outside the repo set → nonzero.
- `--agent status` with no `--worktree` → still succeeds (the gate is scoped to `build-*`, not
  applied globally).

`sync-agents.sh` generation tests assert that a `build-*` shim's dispatch line contains
`--worktree` and a non-`build-*` shim's does not — the guard runs in both directions, so a future
change that widens the flag to every shim, or drops it from build shims, goes red either way.

## Dependencies

`depends_on: [205]`. This change edits `opencode.md`'s env table and builds on the framework state
0205 establishes; `opencode.sh` and `opencode.md` do not exist on the integration branch until
0205 merges. The facade and the two shipped adapters are buildable without it, but splitting the
contract edits across the merge boundary would leave `opencode.md` describing an anchor rule its
own adapter no longer follows.

## Out of scope

- The opencode adapter's flag mapping and permission gate (shipped and settled in 0205).
- Any change to how `docket-build` selects a profile or routes a task.
- Delegating orchestrator agents (`docket-implement-next`, `docket-build` itself). 0205's recipe
  rule — delegate leaves, not orchestrators — is unaffected.
- `ensure-claude-settings.sh`'s remaining `--show-toplevel` use, which ADR-0034 already records as
  a known out-of-scope residual.
