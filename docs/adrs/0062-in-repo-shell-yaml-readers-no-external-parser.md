---
id: 62
slug: in-repo-shell-yaml-readers-no-external-parser
title: YAML and frontmatter are parsed by in-repo shell readers — no external YAML parser
status: Accepted
date: 2026-07-28
supersedes: []
reverses: []
relates_to: [57, 58]
change: 18
---

## Context

docket's shell scripts read YAML config and markdown frontmatter with hand-rolled `sed`/`awk`/`grep`. The readers as they exist today are:

- `sync-agents.sh` (repo root) — `section_body()`, `field_of()`, `harness_agent_line()`
- `scripts/docket-config.sh` — the config resolver
- `scripts/lib/docket-frontmatter.sh` — `field_raw`, `field`, `fm_field`, `list_field`, `int_field`
- `scripts/lib/docket-runtime.sh` — the `runtime.bash` declaration reader (changes 0133/0152)

Adopting `yq` was tracked as an open question from 2026-06-16, in change 0018's original framing. It was never adopted. docket then invested in the opposite direction: centralizing frontmatter reading into `scripts/lib/docket-frontmatter.sh`, and writing ADR-0057 and ADR-0058 *about* those readers — design work that would be nonsense had adoption still been open.

So the question is closed by conduct, but was never written down. That is what this ADR fixes. Zero `yq` invocations exist repo-wide, and `scripts/docket-config.sh` notes "(no yq)" as the reason its `.docket.yml` reader is a flat scalar reader.

## Decision

docket parses YAML and markdown frontmatter with in-repo shell readers and takes **no dependency on `yq` or any external YAML parser**.

The boundary is narrow and exact. This is **not** a claim that docket has no external requirements at all — change 0132 established that docket validates a configured GNU Bash 4+ runtime. The rule bans an external *YAML parser*, nothing wider.

Three reasons that actually held:

1. **The install pitch is clone-and-run.** A YAML parser is a binary that every user and every CI runner would have to acquire before docket works at all.
2. **The two `yq` forks are incompatible binaries.** `mikefarah`'s Go implementation and `kislyuk`'s Python one share a name and not a CLI. Adoption is therefore not "use yq" — it is "pin one fork and detect the other," and being wrong about which one is installed fails silently.
3. **The parsed surface is a documented block-style subset, not arbitrary YAML.** docket's config and frontmatter are flat scalars and single-line lists by design. A general parser buys generality the format does not use.

## Consequences

- The hand readers handle a **subset**. Top-level flow-style mappings (e.g. `agents: {…}`) are silently ignored rather than parsed — `section_body()` matches only a bare header, so a flow-style mapping never enters the block at all. This is accepted, not unnoticed.
- Quoting, spacing, and multi-line scalar forms outside the documented subset are likewise unsupported.
- ADR-0057 (a frontmatter read must be anchored when the key may be absent) and ADR-0058 (the two-tier `field()` vs `field_raw()` reader split) decide *how* the in-repo readers behave, and only make sense under this stance. This ADR therefore carries `relates_to: [57, 58]`; theirs stays `[]` — the link is one-way by construction.
- Re-opening adoption is a **new ADR superseding this one**, never an edit to it. And it would be all-or-nothing across every reader — never one bilingual script alongside the others.
- `scripts/lib/docket-runtime.sh` must additionally run under macOS system Bash 3.2, which tightens the stance rather than loosening it.
