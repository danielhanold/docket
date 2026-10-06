---
id: 145
slug: 'private-repository-parent-facing-rules-load-through-content'
title: 'Private repository parent-facing rules load through content-free user-level triggers'
status: 'Accepted'
date: '2026-10-06'
supersedes: []
reverses: []
relates_to: [36, 78]
change: 535
---

## Context

A private repository must keep docket out of its working tree, so its parent-facing rules (the managed docket:dispatch block plus promoted lessons) cannot live in a repository instructions file. This narrowly revisits change 351's retirement of user-level parent-facing writes. That retirement was about drift between a user-level copy of rule text and the per-repository source; triggers that carry no rule text and no "docket" cannot reintroduce that drift, and the single source of truth stays per repository. A spike showed Claude Code moves hook output over its per-hook ~10,000-character context cap into a file with only a 2KB preview, and that packaging the hook as a Claude plugin does not lift the cap.

## Decision

A private repository's parent-facing rules (the managed docket:dispatch block plus promoted lessons) live in `<git-common-dir>/dckt/AGENTS.md`, never in the working tree or `.git/info/exclude`. They reach the agent through content-free user-level triggers installed once per machine by `docket install`, each running the read-only `dckt instructions` (which prints nothing outside a private repository):

- two Claude Code `SessionStart` hooks in `~/.claude/settings.json`, split by meaning (dispatch block, lessons) so each fits Claude Code's per-hook ~10,000-character context cap;
- a Cursor `sessionStart` hook in `~/.cursor/hooks.json`;
- an OpenCode system-prompt plugin `~/.config/opencode/plugins/dckt-instructions.js`;
- a Codex `dckt:private-instructions` pointer block in `~/.codex/AGENTS.md` (best effort; following it is up to the model).

A hooks file docket cannot edit in place (a symlink, or unparseable content) is left untouched and reported as a warning rather than failing the install.

## Consequences

Private repositories get their dispatch contract and lessons without any docket file in the working tree or its exclude list. The triggers are installed once per machine and are inert in non-private repositories. The rule text keeps one per-repository source, so the drift that retired user-level copies cannot recur. Claude Code needs two hooks, and each half must stay under the per-hook cap. Codex delivery depends on model compliance with a pointer. An uneditable hooks file degrades to a warning, leaving that harness without private instructions until the human fixes it.

## Alternatives considered

- Per-repository rule files listed in `.git/info/exclude`: rejected, because excluded files still live in the private working tree.
- A plain pointer for every harness: rejected, because model compliance is unreliable for Claude and OpenCode on trivial prompts.
- A single Claude Code hook: rejected, because the combined output exceeds the per-hook context cap.
- Capping the private file's size: rejected, because it would limit the rules rather than fix the delivery.
