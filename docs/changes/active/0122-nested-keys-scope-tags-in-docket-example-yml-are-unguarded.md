---
id: 122
slug: nested-keys-scope-tags-in-docket-example-yml-are-unguarded
title: Nested keys' scope tags in .docket.example.yml are unguarded
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [102]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`.docket.example.yml` — docket's canonical, tested config reference — tags every key with its
config-layer scope (`# scope: any layer` or `# scope: repo-only (coordination-fenced, ADR-0019)`).
A test asserts every **ACTIVE TOP-LEVEL** key carries a tag.

Nested keys are structurally invisible to that check. Its `awk` pass keys on
`^[A-Za-z_][A-Za-z0-9_]*:` — zero leading whitespace — so none of the file's nested keys
(`finalize.gate`, `finalize.test_command`, `finalize.require_pr_approval`, `learnings.enabled`,
`learnings.cap`, `reclaim.lease_ttl`, `reclaim.auto`, the five `skills.*`, `runners.codex.sandbox`,
`runners.codex.network`) ever enter the key list. Worse, a block header like `finalize:` has its
comment window satisfied by **any one** child's tag, so a wrong or missing tag on a sibling is
masked.

Change 0102 hit this concretely: `finalize.require_pr_approval` shipped carrying a bespoke
annotation claiming it was repo-committed-only and silently ignored elsewhere — the exact opposite
of its real scope once wired — and nothing in the suite noticed. 0102 fixed that one key and added
two bespoke asserts for it, which are now the *only* automated guard on any nested key's tag.

## What changes

- Extend the scope-tag checker to evaluate nested keys, not only top-level ones — each key's own
  tag, not its parent block's window.
- Decide the inheritance rule deliberately: does a nested key inherit its parent's tag when it has
  no comment of its own, or must every nested key be tagged individually? The current file mixes
  both conventions.
- Retire change 0102's two bespoke `require_pr_approval` tag asserts once the general check covers
  them, so the guard is not maintained in two places.

## Out of scope

- Changing any key's actual scope, or the coordination-key fence itself.
- The manifest classification guard, which is a separate mechanism.
