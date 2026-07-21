---
id: 52
slug: config-key-resolution-boundary
title: A documented config key resolves through docket-config.sh; a model-read of .docket.yml is not a supported shape
status: Accepted
date: 2026-07-21
supersedes: []
reverses: []
relates_to: [48, 19, 12]
change: 102
---

## Context

The config key `finalize.require_pr_approval` shipped documented in three places — the README,
`.docket.example.yml`, and `skills/docket-finalize-change/SKILL.md` — but `scripts/docket-config.sh`
never read it. Its only consumer was the finalize skill body, which parsed `.docket.yml` by eye (a
*model-read*). Consequence: a user setting the key in `.docket.local.yml` or the global
`~/.config/docket/config.yml` got silence — the value was neither honored nor warned-and-ignored,
unlike a coordination-fenced key. The failure mode was the worst shape available: a merge gate the
user believes is armed but is not, discovered only when docket merges an unapproved PR. Nothing in
the repo connected "documented in `.docket.example.yml`" to "resolved by `docket-config.sh`", which
is what let it ship that way and would have let the next key do the same.

## Decision

Every config key documented in `.docket.example.yml` must resolve through `docket-config.sh` and
be emitted in its export block; skills read the exported value from the Step-0 `preflight` block,
never by parsing `.docket.yml` themselves. A model-read of the config file is not a supported
shape. The named exception is a key whose real consumer is another *script* — recorded explicitly
as `elsewhere:<consumer>` in the manifest guard, with the consumer named — never an unclassified
key.

## Enforcement

`tests/test_docket_example_yml.sh` carries a classification manifest: every key documented in the
example classifies as `resolved:<EXPORT_NAME>` (asserted to actually be emitted, AND tied back to
its own leaf key so a copy-pasted arm naming an unrelated export fails) or `elsewhere:<consumer>`
(the named consumer must be one of a declared allowlist of real consumer files and must mention the
key). An unclassified key fails the suite, as does a duplicate leaf key name.

## Consequences

Enables: a documented key's advertised scope is now backed by the resolver, and the
documented-but-unwired bug class fails as a red test rather than shipping silently.

Costs: the manifest is itself a hand-maintained artifact that can go stale — mitigated by making
staleness loud (an unclassified key fails, a `resolved:` entry naming a nonexistent or unrelated
export fails, an `elsewhere:` entry naming a non-consumer fails).

Known residual, worth stating honestly: the `elsewhere:` mention-check proves the key name *occurs*
in the named consumer file, not that it is genuinely *read* there. The declared consumer files
contain enough English prose that a word match can be incidental. Tightening this to prove a real
read is deliberately left as follow-up work.

`finalize.require_pr_approval` is deliberately **not** coordination-fenced (ADR-0019) despite
gating an irreversible shared write: `finalize.gate` — already global-able and gating the very same
merge — is the governing precedent, and splitting the two halves of one merge gate across opposite
scope classes would be harder to explain than the per-machine divergence it permits.

Relates to ADR-0048 (`.docket.example.yml` as a tested canonical reference), ADR-0019 (the
coordination-key fence, which this key sits deliberately outside of), and ADR-0012 (the
script-vs-model boundary — this is that boundary applied to config reads).
