---
id: 255
slug: complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule
title: 'Complete ADR-0065''s quote leg and document the unquoted rule'
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [180, 181]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates #0180 and #0181 (2026-08-07 triage): the correctness half and the documentation half of the same ADR-0065 rule, both discovered from 0173.

Verified 2026-08-07:

- **`hd_validate` is missing the quote leg (#0180).** `scripts/lib/harness-defaults.sh:150-159` has only `[ "$v" != "$raw" ]`. With `{model: "claude-opus-5"}`, `hd_field`'s class (`:88`, `[^,}[:space:]]+`) and `hd_field_raw`'s (`:104`, `[^,}]*`) both return the value **with quotes**, so `v == raw` and the entry ships with quote characters in the pin — exactly ADR-0065's table row. The two-legged reference implementation sits at `sync-agents.sh:606-613` (raw≠consumed OR leading-quote case). ADR-0065 is Accepted and says every `field`/`field_raw` validator pair, present and future, needs the leg.
- **The `#`-strip corner (#0180):** `harness_agent_line` strips `#` before the readers run, in both code paths (`sync-agents.sh:436` sed, `:444` `${line%%#*}`), so `{model: c#5}` truncates silently before any validator sees it.
- **The rule is documented nowhere a user looks (#0181).** `grep -rn unquoted README.md skills/docket-convention/ .docket.example.yml` → zero hits. The two README `agents:` examples (README.md:395-397 global, :423-425 repo-local) show flow-map pins with no rule stated. The gate self-describes only once tripped (`sync-agents.sh:611-612`, same string at `harness-defaults.sh:158`: "write model/effort values unquoted and space-free"). Blast radius: `install.sh` runs `sync-agents.sh`, and the global layer is read for every repo on the machine.

## What changes

- Add the quote leg to `hd_validate`, copied from the sync-agents reference shape; fire/ignore probes.
- Settle the `#`-strip corner: state that a `#`-bearing value is out of contract and validate it (default per #0180), rather than re-ordering the strip.
- Document the unquoted-space-free rule at the point of use: both README `agents:` examples, the docket-convention schema block comment, and `.docket.example.yml` if its `agents:` sketch warrants it — reusing the existing diagnostic's wording.

## Out of scope

- Consolidating the readers themselves — that is the config-reader consolidation change.
- Widening what values are legal (quoting support) — ADR-0065 chose validation, not tolerance.
