---
id: 18
slug: yq-yaml-parsing
title: Record the pure-bash YAML/frontmatter parsing stance as an ADR
status: proposed
priority: low
created: 2026-06-16
updated: 2026-07-26
depends_on: [16]
related: [11, 127]
adrs: [57, 58]
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
type: docs
---

## Why

docket's shell scripts parse YAML and markdown frontmatter with hand-rolled
`sed`/`awk`/`grep` — `sync-agents.sh` (change 0016), `scripts/github-mirror.sh`
(change 0011), and the frontmatter readers sprinkled through the tooling. In
`sync-agents.sh` the config-reading helpers (`entry_line`/`field_of`/`block_names`)
are dense regex over YAML, raised as a readability concern (it is run manually by a
human). `yq` would make those ~40 lines more readable and more *robust* (real YAML:
flow-vs-block mappings, quoting, spacing — the hand parser only handles the
documented block-style subset, and silently ignores top-level flow-style
`agents: {…}`).

The decision today (2026-06-16) was to **keep the scripts as-is** — but the
tradeoff is worth a deliberate future review rather than leaving it implicit. This
stub captures that.

## Re-scoped 2026-07-26 (change 0124 triage)

**The evaluate-yq question is closed — decided "no" by conduct, never written down.** Since this
stub was filed, docket did not merely decline `yq`; it invested in the opposite direction and then
formalized that investment:

- Frontmatter reading was **centralized** into `scripts/lib/docket-frontmatter.sh`
  (`field` / `field_raw` / `fm_field` / `list_field` / `int_field`).
- **ADR-0057** (anchored reads for optionally-absent keys) and **ADR-0058** (the two-tier
  `field` vs `field_raw` reader split) are Accepted decisions *about the hand-rolled readers* —
  design work that would be nonsense had adoption still been open.
- `scripts/docket-config.sh:97` states "(no yq)" as a standing property.
- Zero `yq` invocations exist repo-wide.

So the "if yes" branch is foreclosed, and this change is reduced to the "if no" branch's single
unmet deliverable: **write the ADR**. The original title and `type: refactor` were retired
accordingly — nothing is being refactored.

One premise of the original framing is also stale and should not be repeated in the ADR: change
0132 established that docket *does* validate a runtime dependency (a configured Bash 4+ runtime),
so "zero external deps" is no longer literally true. The honest stance is narrower — no external
*YAML parser* — and the ADR should say that rather than overclaiming.

## What changes

Record a short ADR stating that docket parses YAML and markdown frontmatter with in-repo shell
readers and does not take a dependency on `yq` or any external YAML parser.

- State the decision and its real boundary (no external YAML parser — not "zero dependencies",
  per 0132).
- Give the reasons that actually held: the install pitch is clone-and-run; the two `yq` forks
  (`mikefarah` Go vs `kislyuk` Python) are incompatible binaries, so adoption means pinning one;
  and the parsed surface is a documented block-style subset, not arbitrary YAML.
- Name the accepted consequence: the hand readers handle a subset, and top-level flow-style
  mappings (e.g. `agents: {…}`) are silently ignored rather than parsed.
- Relate it to ADR-0057 and ADR-0058, which decide *how* the in-repo readers behave and only make
  sense under this stance.

## Out of scope

- Re-opening adoption. If it is ever re-opened, that is a new ADR superseding this one, not an
  edit — and it would be all-or-nothing, never one bilingual script.
- Changing any reader's behavior, or touching `sync-agents.sh` / `scripts/github-mirror.sh`
  parsing. This change writes a record; it moves no code.
- `sync-agents.sh`'s `emit()` frontmatter rewrite — markdown-with-frontmatter is not pure YAML and
  stays `awk` under any stance.

## Open questions

- None. The decision is made; this records it.

## Reconcile log
