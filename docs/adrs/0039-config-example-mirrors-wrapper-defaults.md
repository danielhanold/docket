---
id: 39
slug: config-example-mirrors-wrapper-defaults
title: config.yml.example is a documented mirror of the shipped wrapper defaults
status: Accepted
date: 2026-07-15
supersedes: []
reverses: []
relates_to: []
change: 81
---

## Context

Change 0081 adds `config.yml.example` — a committed starter for the global
user-level config (`${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`). To let a
first-time user *see* the otherwise-invisible per-skill model/effort defaults,
its `agents.claude` block restates docket's nine claude built-in per-skill
defaults. Those same values already ship in the `agents/docket-*.md` wrapper
frontmatter, which is what `sync-agents.sh` reads to generate the machine-local
wrappers. So `config.yml.example` duplicates authored values that live in the
wrappers: a discoverability win bought with a second copy that can drift.

## Decision

Accept the duplication for discoverability. `agents/docket-*.md` wrapper
frontmatter is the **single source of truth** for the per-skill model/effort
defaults; `config.yml.example`'s `agents.claude` block is a **documented
mirror** of those values, present only so a first user can see the defaults
without reading nine wrapper files.

The rule: when a shipped default changes in `agents/docket-*.md`,
`config.yml.example` **must** be updated to match. The mirror never leads; the
wrappers do. A reader who finds the two disagreeing trusts the wrappers.

## Consequences

- **Discoverability:** a first user sees docket's built-in per-skill defaults
  in one committed file instead of having to open nine wrappers or run a tool.
- **Sync burden:** the mirror can go stale, and a stale mirror actively
  misleads — it shows a user a default the wrappers no longer carry. The two
  copies must be kept equal by hand at every default change.
- **Mitigation:** change 0081 adds a build-time equality check that fails when
  `config.yml.example`'s mirrored values diverge from the wrapper frontmatter,
  so drift is caught rather than shipped. A fuller automated drift guard is
  possible future work and is out of scope now.
