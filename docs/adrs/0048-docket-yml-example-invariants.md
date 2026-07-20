---
id: 48
slug: docket-yml-example-invariants
title: .docket.yml.example is a tested canonical config reference — mirror, fidelity, must-update
status: Accepted
date: 2026-07-19
supersedes: [39]
reverses: []
relates_to: [19]
change: 101
---

## Context

ADR-0039 (change 0081) established that `config.yml.example`'s `agents.claude` block is a
**documented mirror** of the per-skill model/effort defaults authored in `agents/docket-*.md`
wrapper frontmatter — the wrappers are the single source of truth, the mirror never leads, and
a build-time equality check (`tests/test_config_example.sh`) catches drift.

Change 0101 deletes both artifacts ADR-0039 named. `config.yml.example` is replaced by a new
root-level `.docket.yml.example` — a Helm-values-style canonical reference where every key ships
**active** at its shipped default, not just the `agents.claude` block — plus a minimal
pointer-only global config scaffolded by `scripts/ensure-global-config.sh`. The equality check is
replaced by `tests/test_docket_yml_example.sh`. ADR-0039's decision was not wrong; it is stated
over artifacts that no longer exist. The mirror rule survives, relocated, and change 0101 adds two
more invariants the surviving rule alone doesn't cover: does the example actually match the
resolver's real defaults, and does it stay current as new keys are added.

## Decision

`.docket.yml.example` is a hand-maintained canonical config reference held trustworthy by three
invariants, enforced together by `tests/test_docket_yml_example.sh`:

1. **The mirror rule survives, relocated.** `.docket.yml.example`'s commented `agents.claude`
   block mirrors `agents/docket-*.md` wrapper frontmatter. The wrappers remain the single source
   of truth; the mirror never leads. A reader who finds the two disagreeing trusts the wrappers.
   This is the relocated ADR-0039 equality check, now run against the new file. The block ships
   **commented**, not active like the rest of the file, because `agents:` and `agent_harnesses:`
   are presence-sensitive: `sync-agents.sh` branches on whether the `^`-anchored key is present at
   all, so an active-but-empty block would change behavior rather than merely document a default.

2. **The example IS the resolver's defaults — test-enforced.** Every other key ships **active**
   at its shipped default, and `tests/test_docket_yml_example.sh` proves byte-fidelity by feeding
   the example file to the real resolver and diffing its `--export` output against the no-config
   export. This is what makes "copy this file" safe advice: an unedited full copy into a repo's
   `.docket.yml` is a no-op. Two keys — `finalize.test_command` and `github_project` — gained a
   new `auto` sentinel (≡ unset) purely so their defaults could ship as active values instead of
   commented notes.

3. **The must-update rule.** Every new config flag lands in `.docket.yml.example` — value, plus
   documentation, plus scope tag (repo-only coordination-fenced vs any-layer) — in the **same PR**
   that introduces it. A canonical reference that lags reality is worse than no canonical
   reference, because it is trusted. This is backed by a completeness guard driven off the
   resolver's actual `--export` surface (a new exported key fails the suite until documented), an
   explicit allowlist for the four schema keys the resolver does not export (`github_project`,
   `agents`, `agent_harnesses`, `finalize.require_pr_approval`), and an inverse orphan-key check so
   the example cannot accrete keys nothing reads.

## Consequences

- The trade-off ADR-0039 accepted — duplication for discoverability over codegen from the
  resolver — is re-accepted at larger scale: `.docket.yml.example` is roughly 300 lines of
  hand-maintained prose asserting facts about `scripts/docket-config.sh` and several skill
  bodies. Codegen was considered and rejected in the spec; manual mirror plus tests remains the
  accepted trade-off.
- The guards are asymmetric in strength. Key **presence** and exported-key **values** are
  mechanically enforced by the test suite; the surrounding **English** describing what a key does
  is not, and this repo has a documented history of prose asserting a false fact about another
  artifact with every grep green. The whole-branch review of change 0101 found exactly this twice
  — both about `github_project` — and both were fixed before merge. This is a known residual
  risk, not a solved problem, and no mechanism in this ADR closes it.
- `finalize.require_pr_approval` and `github_project` are documented in the example with explicit
  NOT-WIRED / layer-resolution-broken annotations rather than a standard scope tag, because both
  are real gaps in the implementation (the former tracked as change 0102) — the example's job is
  to describe what IS, including what is broken, not to launder a gap into looking resolved.

## Update — 2026-07-20

Change 0109 renamed the artifact this ADR governs:

- `.docket.yml.example` → `.docket.example.yml`
- its guard `tests/test_docket_yml_example.sh` → `tests/test_docket_example_yml.sh`

Reason: the `.example` suffix landed after `.yml`, so neither editors nor GitHub recognized the
file as YAML — both rendered the one file whose entire job is to be read as plain text, with no
syntax highlighting, folding, or structural cues. `.docket.example.yml` keeps the "this is an
example" signal while ending in `.yml`, so every YAML-aware tool highlights it.

The decision above is unchanged: all three invariants stand exactly as written, and the file's
contents, key set, ordering, structure, and scope tags are untouched. This was a rename plus a
reference sweep across 14 live files, nothing more. Every reference above to
`.docket.yml.example` and `tests/test_docket_yml_example.sh` should now be read as naming
`.docket.example.yml` and `tests/test_docket_example_yml.sh` respectively.

This ADR's own body above, the generated `docs/adrs/README.md` index, and change 0101's and
0107's archived artifacts deliberately retain the old name as historical records.
