---
id: 40
slug: terminal-publish-default-opt-in
title: terminal_publish defaults to false — publishing is opt-in
status: Accepted
date: 2026-07-16
supersedes: []
reverses: []
relates_to: [27]
change: 84
---

## Context

`terminal_publish` defaulted to `true`, so every repo that never set the key had direct machine
commits pushed onto its integration branch at close-out. That fail-open posture caused real,
recurring friction: direct pushes trip branch protection on protected/PR-only branches; agent
permission classifiers deny the push mid-run and hard-stop autonomous loops; and a failed publish
can gap silently — a documented incident had a terminal record missing from `main` for eight days
with nothing noticing (changes #0083 / #0043). Writing to a repo's code line should be a conscious
act, not something a repo inherits by never setting a key.

ADR-0027 already decided *where* the knob may be set (per-repo-only, coordination-key fenced) and
*where* the gate lives (once inside `terminal-publish.sh`, ahead of the mode dispatch) — that
decision, and its text, stand unchanged. What ADR-0027 did not decide, and what this ADR settles,
is the *default value* of the knob when a repo's committed `.docket.yml` never sets it.

## Decision

**Unset ⇒ `false`, at every layer that encodes the default:** the config resolver
(`docket-config.sh`), the `terminal-publish.sh` `--enabled` flag fallback, and the
`docket-status.sh` merge-sweep fallback. Publishing requires the repo's committed `.docket.yml` to
say `terminal_publish: true` explicitly.

A closely related sub-decision: `terminal-publish.sh` now has **no default at all** for
`--enabled` — the flag itself must be passed. An **omitted** flag is treated as disabled but
no-ops **loudly**: a prominent stderr WARNING, exit 0. A caller that forgot the flag is a bug,
not a decision, and needs to be seen — whereas an explicit `--enabled false` remains a silent,
intentional no-op, because that caller made the choice on purpose. Exit 0 on the omitted path is
deliberate: callers trust the exit code, and a missing flag must never abort a close-out — the
warning, not a non-zero exit, is what keeps a skipped publish from hiding (the #0043 silent-gap
failure mode).

## Consequences

Repos that relied on the implicit default silently stop publishing until they pin
`terminal_publish: true` — the migration posture is deliberately docs-only, with no runtime
notice when the key is unset, though an opted-out repo does log a
`terminal_publish: false — skipping publish…` line to stderr at each close-out. Hand invocations
of `terminal-publish.sh` now need an explicit `--enabled true`. The docket repo itself pins
`true`, so its own archive-parity practice is unchanged, now explicit rather than inherited.

The knob remains never-retroactive (ADR-0027): already-published records stay, and opting in does
not back-fill. This narrows, but does not replace, #0083's gap-detection health check.
