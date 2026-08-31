---
id: 168
slug: cursor-profile-routed-build-support
title: Cursor support for profile-routed Docket builds
status: done
priority: medium
type: feat
created: 2026-07-30
updated: 2026-07-31
depends_on: [167]
related: [135, 142, 164, 169]
discovered_from: [167]
adrs: [15, 16, 48, 60, 63, 64]
spec: docs/superpowers/specs/2026-07-30-cursor-profile-routed-build-support-design.md
plan: docs/superpowers/plans/2026-07-31-cursor-profile-routed-build-support.md
results: docs/results/2026-07-31-cursor-profile-routed-build-support-results.md
trivial: false
auto_groomable:
branch: feat/cursor-profile-routed-build-support
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/140
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-30-cursor-profile-routed-build-support-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-30-cursor-profile-routed-build-support-design.md) |
| Plan | [2026-07-31-cursor-profile-routed-build-support.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-31-cursor-profile-routed-build-support.md) |
| Results | [2026-07-31-cursor-profile-routed-build-support-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-31-cursor-profile-routed-build-support-results.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0048](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0048-docket-yml-example-invariants.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0064](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md) |
<!-- docket:artifacts:end -->

## Why

Change 0167 shipped Docket's lean, profile-routed build with Claude defaults only. Cursor can
translate and dispatch the workers, but without native defaults it inherits a foreign model ID or
requires every user to supply the same overrides. The wrapper sources also double as the Claude
default store, so adding one-off Cursor exceptions would leave shipped defaults split across two
mechanisms.

## What changes

- Add one harness-indexed shipped-default sidecar: complete for all twelve Claude agents and sparse
  for Cursor's three build workers. Make the wrapper sources behavior-only templates.
- Ship Cursor build mappings for `economy` (`cursor-grok-4.5-medium`), `standard`
  (`cursor-grok-4.5-high`), and `premium` (`claude-opus-5-high`), each with its effort already
  encoded in the Cursor model ID.
- Preserve field-level user overrides and keep shipped native defaults out of delegated
  child-runner flags.
- Make unsupported harness/agent combinations inherit their own harness default rather than a
  Claude source value; change 0169 will add Codex defaults to the same sidecar.
- Certify explicit routing, automatic routing, and one bounded escalation in the Cursor IDE.
- Make the README skill catalog complete under a count-free `## Skills` heading and update the
  build documentation for shipped Claude and Cursor support.
- Supersede ADR-0048 with the new sidecar ADR, restating its three example-file invariants with the
  mirror target re-pointed from the wrapper frontmatter to `agents/harness-defaults.yml`.
- Rewrite the two generated-wrapper byte-identity assertions that the source/sidecar split makes
  structurally impossible, and re-point the example-mirror equality loop at the sidecar.

## Out of scope

- Changing the shared task-worker contract established by change 0167.
- Shipping defaults for Cursor's other nine agents or any Codex agents.
- Runtime model discovery, Cursor CLI certification, or replacement of the whole-branch review
  skill.
- Redesigning the unmapped-harness generation warning beyond keeping it honest under the new
  resolution order; change 0142 owns making that gap loud.

## Reconcile log

### 2026-07-31

Re-read against `origin/main` at `ad55a3cd`, the cited ADRs, and changes 0135 / 0142 / 0164 / 0169 /
0170. The design holds — no scope inversion, no work done elsewhere. Five adjustments:

1. **ADR-0048 collision (material; spec was silent).** The spec's architecture-decision section
   named ADR-0015/0016/0060/0063 and claimed the new ADR supersedes nothing. But ADR-0048
   (`Accepted`, itself superseding ADR-0039) states as its first invariant that
   `.docket.example.yml`'s commented `agents.claude` block mirrors **`agents/docket-*.md` wrapper
   frontmatter**, and that "the wrappers remain the single source of truth." Relocating shipped
   defaults into `agents/harness-defaults.yml` falsifies that invariant's target. The new ADR
   therefore **supersedes ADR-0048**, restating all three of its invariants with the mirror
   re-pointed at the sidecar — exactly the relocation pattern by which 0048 superseded 0039.
   Added 48 to `adrs:`.

2. **Two byte-identity assertions break by construction**, not by tuning:
   `tests/test_sync_agents_cursor.sh` and `tests/test_sync_agents_codex.sh` each `diff` the
   generated Claude wrapper against its `agents/` source to prove the harness split preserves the
   Claude side. Once the source drops `model:`/`effort:` and the generator injects them from the
   sidecar, file-level identity is impossible. They get rewritten to assert behavior/skills
   identity plus injected model/effort — the spec's "byte-equivalent" verification item is about
   resolved values, not file bytes. Likewise `tests/test_docket_example_yml.sh`'s twelve-agent
   mirror-equality loop reads the wrapper frontmatter directly and must re-point at the sidecar.

3. **Claude default values verified byte-exact.** All twelve `model`/`effort` pairs in the spec's
   sidecar table match the live `agents/docket-*.md` frontmatter, including
   `status: claude-haiku-4-5-20251001 / medium`. The preserve-every-current-value goal is
   mechanically checkable with no drift to resolve first.

4. **README surface is wider than the spec's one heading.** The catalog heading is
   `## The eight skills`, with anchor links in the table of contents and in a later footnote whose
   hand-maintained arithmetic ("eleven directories … lists eight") also dies with the count. A
   separate paragraph asserts "the only model IDs docket actually ships are the Claude ones under
   `agents.claude`" and explicitly promises this state holds "until changes 0168 and 0169 land
   validated mappings" — falsified by this change. All four sites move together; `skills/` holds
   eleven packages.

5. **Warning semantics shift, deliberately bounded.** `warn_fallback_model()` today fires when a
   non-Claude harness inherits a model from the default/built-in layer. Afterwards Cursor's nine
   non-build wrappers resolve to *no* model at all, so the trigger becomes "no harness-specific
   value" rather than "inherited a foreign ID." This change keeps the warning honest under the new
   order and stops there; change 0142 (still a stub) owns making the gap loud, and is already in
   `related:`.

Dependency 0167 confirmed `done` (PR #139 merged). Tier 2 Cursor IDE certification is a
human-at-the-IDE checklist this autonomous run cannot execute; per the spec the results file and PR
body carry **Cursor IDE certification pending** and the merge gate stays blocked on it. Auto-capture
is enabled; every discovery above folds into this change's own scope or an existing stub, so no
stubs were minted.
