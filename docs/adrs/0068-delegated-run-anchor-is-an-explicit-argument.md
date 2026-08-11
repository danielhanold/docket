---
id: 68
slug: delegated-run-anchor-is-an-explicit-argument
title: A delegated run's anchor is an explicit argument defaulting to the main worktree
status: Accepted
date: 2026-08-05
supersedes: []
reverses: []
relates_to: [34]
change: 206
---

## Context

`runner-dispatch.sh` anchored every delegated run at `docket_main_worktree()` (ADR-0034),
exporting that path as `DOCKET_REPO_ROOT` and handing it to each adapter's directory flag
(`codex exec -C`, `cursor` likewise, `opencode run --dir`).

That anchor is correct for the agents delegation had shipped for. `docket-status` and `docket-adr`
are metadata-scoped: they operate on `metadata_branch` through the `.docket/` worktree and belong in
the main tree. It is wrong for a build worker, whose `docket-build-task` contract requires it to run
**inside its feature worktree, on its branch**, performing no docket metadata operations. A
delegated `build-*` worker therefore started in the primary checkout with the integration branch
checked out, and the only thing pointing it at the correct tree was prose in the relayed prompt.

Two observations sharpen the failure mode:

1. Feature worktrees live at `<repo>/.worktrees/<slug>` — *inside* the main worktree — so a
   `workspace-write` sandbox rooted at `DOCKET_REPO_ROOT` already permits writes there. **This is
   not a permission failure.**
2. The exposure is the **starting cwd and the checked-out branch**. A worker that does not
   faithfully follow the prompt's instruction commits code onto the integration branch in the
   shared primary checkout, unattended.

The mismatch predates the opencode adapter — it is a property of the 0079 delegation framework —
but only became reachable when change 0205 shipped the first documented recipe delegating the
`build-*` profile workers to a child harness.

## Decision

A delegated run's anchor is an **explicit argument whose default is the main worktree**; the
delegated agent's **scope** decides which. Metadata-scoped agents (`status`, `adr`) keep the main
worktree implicitly. Feature-scoped agents must name their tree, and the facade **refuses to run
one that did not**.

ADR-0034 stands **unamended**. Nothing resolves an anchor from the caller's CWD; the main worktree
remains the default; the only way off it is an argument someone deliberately wrote.

### Rejected alternatives

**Resolve the caller's CWD** (use CWD when inside the repo, main worktree as fallback). Rejected on
two independent grounds: it reverses ADR-0034's central rule that a stray `cd` must never misdirect
a script; and it is unreliable at the one moment it matters, since a shim runs as a harness agent
making a single Bash call whose cwd is the *session's* cwd — whatever the last `cd` left behind,
not reliably the feature worktree.

**Derive the path inside the facade** from docket state (the change's `branch:` field →
`git worktree list`). Rejected: the facade is deliberately docket-metadata-ignorant and does not
know which change is being built.

**Forbid delegating `build-*` agents**, or merely document the limitation and change no code.
Rejected: selective build delegation is change 0205's stated motivation (cheap build workers,
review native), and documenting a reachable unattended-commit-to-main path is not a fix.

**Require `--worktree` for every delegation.** Rejected: it breaks every shipped `status`/`adr`
shim and *does* amend ADR-0034, for no gain over requiring it where scope demands it.

## Consequences

Enables: a feature-scoped delegated worker starts in the tree its contract requires, without any
adapter change — `codex.sh`, `cursor.sh`, and `opencode.sh` keep reading `DOCKET_REPO_ROOT`
verbatim. That the facade owns the anchor is what makes this cost the adapters nothing.

Cwd-independence is preserved by construction: the resolution routes through `docket_anchor_path`,
so a relative `--worktree .worktrees/<slug>` joins to the **main worktree** and resolves identically
from any cwd. The new argument inherits ADR-0034's cwd-independence rather than reintroducing the
hazard ADR-0034 was written against. Absent the flag, behavior is byte-identical to before for
every currently-shipped shim.

Three loud gates (abort-and-report, matching the facade's posture for an unknown `--runner`):
`--worktree` is required for `build-*` agents; the resolved anchor must be a directory; and it must
belong to this repo's worktree set — the last keeps a child harness running under an auto-approve
grant out of a tree docket does not own. The `build-*` gate is the one piece of agent-family
knowledge the facade gains; it is a runtime requirement, so generation-time enforcement cannot
substitute for it.

`sync-agents.sh` bakes the flag into `build-*` shims as a required slot, leaving every other shim
byte-identical. The cost accepted: the *value* remains prose-supplied one level up. What makes that
acceptable is that an omission is now a loud abort rather than a silent main-tree run on the
integration branch.

## Update — 2026-08-11 (see ADR-0083)

Two statements in *Consequences* above are no longer true, though the decision itself stands.

"The `build-*` gate is the one piece of agent-family knowledge the facade gains" — the facade now
reads a declared `worktree-scope:` for **every** agent, not just the build family. And
`sync-agents.sh` no longer leaves "every other shim byte-identical": the five newly feature-scoped
agents (`docket-rebase-resolver`, `docket-integration-repair`, and the three `docket-review-*` rungs)
gained the required `--worktree` slot too, and the declaration flows into every generated wrapper.

The cause: keying on the name shape `build-*` enumerated one family while three others were equally
feature-scoped, leaving them ungated. ADR-0083 replaces the name-pattern key with a declared
frontmatter fact that both readers gate on.
