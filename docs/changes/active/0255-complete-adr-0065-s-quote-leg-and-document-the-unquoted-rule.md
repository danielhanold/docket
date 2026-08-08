---
id: 255
slug: complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule
title: 'Complete ADR-0065''s quote leg and document the unquoted rule'
status: in-progress
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: [256]
discovered_from: [180, 181]
adrs: []
spec: docs/superpowers/specs/2026-08-07-complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule
claimed_at: 2026-08-08T05:07:10Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0180 and #0181 (2026-08-07 triage): the correctness half and the documentation half of the same ADR-0065 rule, both discovered from 0173.

Verified 2026-08-07:

- **`hd_validate` is missing the quote leg (#0180).** `scripts/lib/harness-defaults.sh:150-159` has only `[ "$v" != "$raw" ]`. With `{model: "claude-opus-5"}`, `hd_field`'s class (`:88`, `[^,}[:space:]]+`) and `hd_field_raw`'s (`:104`, `[^,}]*`) both return the value **with quotes**, so `v == raw` and the entry ships with quote characters in the pin — exactly ADR-0065's table row. The two-legged reference implementation sits at `sync-agents.sh:606-613` (raw≠consumed OR leading-quote case). ADR-0065 is Accepted and says every `field`/`field_raw` validator pair, present and future, needs the leg.
- **The `#`-strip corner (#0180):** `harness_agent_line` strips `#` before the readers run, in both code paths (`sync-agents.sh:436` sed, `:444` `${line%%#*}`), so `{model: c#5}` truncates silently before any validator sees it.
- **The rule is documented nowhere a user looks (#0181).** `grep -rn unquoted README.md skills/docket-convention/ .docket.example.yml` → zero hits. The two README `agents:` examples (README.md:395-397 global, :423-425 repo-local) show flow-map pins with no rule stated. The gate self-describes only once tripped (`sync-agents.sh:611-612`, same string at `harness-defaults.sh:158`: "write model/effort values unquoted and space-free"). Blast radius: `install.sh` runs `sync-agents.sh`, and the global layer is read for every repo on the machine.

## What changes

Settled design in the linked spec (auto-groomed 2026-08-07; critic-gated, one revision round):

- Add the quote leg to `hd_validate`, an inline copy of the sync-agents reference condition (double and single quotes), merged into the existing byte-identical diagnostic; no shared helper — extraction is deferred to #0256.
- Settle the `#`-strip corner: strip order unchanged; a `#` inside an entry's `{…}` flow map is out of contract and **validated** in BOTH validators via a pre-strip view of the entry line, with a distinct diagnostic. Exact firing rule, carve-outs (trailing comments, full-line comments, commented-out maps stay legal), and the hard-abort cost statement are in the spec. Ruling recorded in spec + docs + code comments; no ADR update.
- Document one consistent sentence — unquoted, space-free, no `#` inside the flow map — at five points of use: both README `agents:` examples, docket-convention SKILL.md's schema block comment, `references/agent-layer.md`'s example block, and `.docket.example.yml`'s `agents:` intro.
- Value-level fire/ignore probes in `test_harness_defaults_validator.sh` (quote legs + `#` leg) and `test_sync_agents_validator.sh` (`#` leg; 0173's quote probes untouched).

## Out of scope

- Consolidating the readers themselves — that is the config-reader consolidation change.
- Widening what values are legal (quoting support) — ADR-0065 chose validation, not tolerance.
