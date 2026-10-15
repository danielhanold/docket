---
id: 150
slug: 'docket-install-grants-claude-code-allow-rules-for-finalize-p'
title: 'docket install grants Claude Code allow rules for finalize publish and merge'
status: 'Accepted'
date: '2026-10-15'
supersedes: []
reverses: []
relates_to: [145]
change: 552
---

## Context

Claude Code's auto-mode classifier soft-denies `docket finalize publish` and `docket finalize merge` as a "CI bypass" when finalize reuses implement-next's green suite evidence, stalling an otherwise clean finalize until a human intervenes.

ADR-0145 had the user-level Claude Code settings target carry only content-free triggers (the SessionStart hook commands). Change 0027 earlier rejected a global raw `git push` allow rule because it would authorize far more than docket's own operations.

## Decision

`docket install` carries a narrow permission grant in the user-level Claude Code settings (`~/.claude/settings.json`, `permissions.allow`): `Bash(docket finalize publish:*)` and `Bash(docket finalize merge:*)`.

The grant is owned by the existing Claude hook-entries install target as a second owned list (allow rules) next to the SessionStart hook commands, not as a second target: install state allows one managed target per file.

- Every install converges the list: it re-adds hand-deleted rules, retires rules a newer docket no longer ships, and is a byte-identical no-op otherwise.
- Uninstall removes the rules.
- A settings file docket cannot edit is skipped with a warning naming the rules.
- A test pins each rule to the capability catalog argv for the operation it grants.

The grant is user-level because the rules match only `docket finalize publish|merge`, which refuse outside a prepared docket repository, so a global grant authorizes nothing elsewhere (unlike change 0027's rejected raw `git push` rule). This extends ADR-0145's user-level target from content-free triggers to include this permission grant.

## Consequences

- Finalize publish and merge no longer stall on the auto-mode classifier on machines where docket is installed.
- There is no opt-out knob; a user who does not want the grant must remove docket's install or accept it.
- A hand-typed rule identical to a docket rule is indistinguishable from docket's and is removed on uninstall (the same as hook entries).
- An empty `allow` list or `permissions` object the user wrote is removed if docket's removal empties it.

## Alternatives considered

- Per-repo grants in `.claude/settings.local.json`: needs a write per clone, is missing on fresh clones, and violates the private-repository rule of nothing in the repo root.
- A generic JSON-array install target: a second managed target in the same file is not allowed by install state, and nothing else needs it yet.
- A config knob to disable the grant: no demonstrated need, and the rules authorize nothing outside a docket repository.
- Also covering the `dckt` alias spelling: skills invoke the catalog argv, which spells `docket`, so the alias rule would be dead.
