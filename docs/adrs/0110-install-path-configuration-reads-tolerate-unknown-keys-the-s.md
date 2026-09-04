---
id: 110
slug: 'install-path-configuration-reads-tolerate-unknown-keys-the-s'
title: 'Install-path configuration reads tolerate unknown keys; the strict typo policy binds operating commands only'
status: 'Accepted'
date: '2026-09-04'
supersedes: []
reverses: []
relates_to: [19, 102]
change: 392
---

## Context

docket's config parser enforces a strict typo policy — an unrecognized key in .docket.yml is a hard `invalid configuration` error (it lands in the resolver's invalidClass), so typos are caught loudly rather than silently ignored. The unintended consequence was a self-hosting bootstrap deadlock: when a merged change extends the .docket.yml schema (change 0374 added a `build:` block), the still-installed pre-schema binary rejected ALL config reads, and because `docket development install` triggers a repository-phase config read at startup, the installer itself could no longer run — the one tool that would rebuild the binary was blocked by the very field it needed to be rebuilt to understand. Recovery was an out-of-band `go build` and a manual binary swap, and CLAUDE.md carried a standing caveat describing it. This recurs for every schema-extending change.

## Decision

Add `config.ResolveContext.TolerateUnknownKeys`, set at exactly one site — the CLI's shared `installOptions`/`installResolveContext` — which reclassifies the `unknown-key` diagnostic, at any depth, from error to warning with a shared remedy naming both plausible causes (a newer docket than the one running, or a typo); the snapshot stays valid and the unknown subtree resolves to defaults. It applies to all three install operations (`install`, `install check`, `development install`) on both of their config reads: the global-only read and the repository-phase read inside `app.ResolveRepoPhase`, which now takes the caller's ResolveContext. Malformed YAML, duplicate keys, wrong types, bad values, and the coordination fence (ADR-0019) stay fatal on the install path; every other command keeps the strict typo policy unchanged. Install-path warning-severity diagnostics are surfaced in the install result (JSON `warnings` plus one human-readable line each) rather than discarded.

## Consequences

A schema-extending change no longer needs an out-of-band rebuild — the tracked `development.install` reinstall works directly with a pre-schema binary, closing the deadlock. The strict policy is now a property of *operating* the repository, not of parsing: an old binary still refuses to operate on an unknown field, and `docket diagnostic config` still shows the strict verdict on demand, so a genuine typo is still caught the moment any real command runs. The cost is a narrow window in which an install-path invocation proceeds over a key it does not understand; the warning surface is what keeps that visible. CLAUDE.md/AGENTS.md's schema-bump rebuild caveat is replaced by a one-line note.

## Alternatives considered

Keep the strict policy everywhere and live with the documented manual `go build` + binary-swap recovery — rejected: it recurs for every schema-extending change and deadlocks the very automation meant to recover it. Make unknown keys tolerant globally — rejected: it discards the typo protection that motivated the strict class in the first place. Version-gate the config schema so an older binary knows to skip newer blocks — rejected as far more machinery than the problem warrants, and it still needs a tolerant read to get at the version marker.
