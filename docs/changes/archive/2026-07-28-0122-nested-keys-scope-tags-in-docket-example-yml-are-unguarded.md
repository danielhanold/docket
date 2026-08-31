---
id: 122
slug: nested-keys-scope-tags-in-docket-example-yml-are-unguarded
title: Nested keys' scope tags in .docket.example.yml are unguarded
status: done
priority: medium
created: 2026-07-21
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [102]
adrs: []
spec: docs/superpowers/specs/2026-07-26-nested-key-scope-tags-design.md
plan: docs/superpowers/plans/2026-07-28-nested-key-scope-tags-plan.md
results: docs/results/2026-07-28-nested-keys-scope-tags-in-docket-example-yml-are-unguarded-results.md
trivial: false
auto_groomable: true
branch: feat/nested-keys-scope-tags-in-docket-example-yml-are-unguarded
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/131
blocked_by:
reconciled: true
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-26-nested-key-scope-tags-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-26-nested-key-scope-tags-design.md) |
| Plan | [2026-07-28-nested-key-scope-tags-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-28-nested-key-scope-tags-plan.md) |
| Results | [2026-07-28-nested-keys-scope-tags-in-docket-example-yml-are-unguarded-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-28-nested-keys-scope-tags-in-docket-example-yml-are-unguarded-results.md) |
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

Rewrite the `untagged_keys` awk pass in `tests/test_docket_example_yml.sh` to evaluate keys at
every indentation depth, under a settled four-rule set (full design in the spec):

- **Scalar keys carry the obligation**; a mapping-opener header never needs a tag of its own, but
  may provide one for its subtree.
- **Coverage = own tag, else nearest tagged ancestor** — the inheritance question is decided in
  favour of inheritance, which keeps both of the file's existing conventions legal and requires
  **zero edits to `.docket.example.yml`**.
- **A header's comment window never extends forward into its body** — the anti-masking rule, and
  the one that makes the 0102 regression RED (verified against the pre-0102 file).
- Same-depth adjacency inheritance is retained for the `changes_dir`/`adrs_dir`/`results_dir` group.

Also: accept `scope: local-only` (the third legend form, currently unaccepted); report failures as
dotted paths; assert an **exact 17-key** nested population floor emitted by the guard's own output;
add two mutation self-tests over a temp copy running the hoisted, identical program; and rewrite
the guard's now-false "top-level" prose, assert label, and failure diagnostic.

**Change 0102's two bespoke asserts are KEPT, not retired.** The stub's retire instruction was
conditional on the general check covering them, and it does not: the tag assert pins the
*specific* `any layer` value, while the general check only proves *some* sanctioned form covers the
key — retiring it would silently accept a relabel to `repo-only`, the exact 0102 bug class. The
second assert is a negative prose guard, orthogonal to tagging.

## Out of scope

- Changing any key's actual scope, or the coordination-key fence itself.
- Normalizing `.docket.example.yml` onto a single tagging convention.
- The manifest classification guard and the `(2c)` orphan-key check, which are separate mechanisms
  (`(2c)` is column-0-anchored too, but anchors on consumers — a separate design question, noted as
  a follow-up observation in the spec).

## Triage note (2026-07-26, change 0124)

Confirmed still live. The column-0 anchor is `tests/test_docket_example_yml.sh:624`:

```awk
is_active = ($0 ~ /^[A-Za-z_][A-Za-z0-9_]*:/)
```

No leading-whitespace alternative, so every nested key is still invisible to the scope-tag check
exactly as described. Line 631 keys its `H`/`S` header-vs-scalar split off the same anchored
pattern, so extending coverage means revisiting both lines together, not just the `is_active` test.

## Reconcile log

### 2026-07-28 — build-time reconcile (claim → plan)

Re-read the change, its spec, `related`/`discovered_from` (0102), the recent ADRs, and the current
code on `origin/main` (tip `f47f480a`). **Design holds unchanged; scope unchanged; no edits to the
spec were required.** Verified point by point:

- **The defect is still live.** `tests/test_docket_example_yml.sh` still anchors its scope-tag pass
  on `is_active = ($0 ~ /^[A-Za-z_][A-Za-z0-9_]*:/)` with no leading-whitespace alternative, and
  the `H`/`S` header-vs-scalar split still keys off the same anchored pattern. The header's window
  still extends forward through its nested body (the masking rule 3 deletes).
- **The exact-17 nested-key floor is confirmed against the current file.** Enumerating
  `^[[:space:]]+<key>:` in `.docket.example.yml` on `origin/main` yields exactly 17 keys —
  3 `finalize.*`, 2 `learnings.*`, 2 `reclaim.*`, 2 `auto_capture.*`, `runners.codex` plus its 2
  leaves, and 5 `skills.*` — matching the spec's enumeration key for key. The floor ships as an
  exact count as designed.
- **Zero edits to `.docket.example.yml` are still required.** The legend still defines all three
  tag forms, and the file's mixed convention (per-child tags on `finalize`/`learnings`/`reclaim`,
  block-header tags on `auto_capture`/`runners`/`skills`) remains legal under rules 1–3.
- **Drift, in docket's favour:** the spec asks for the `scope: local-only` **presence** assert to be
  kept alongside the other two; all three presence asserts already exist on `origin/main`. Only the
  *accepted set inside the awk pass* still omits `local-only` — the awk still tests just the
  `repo-only` and `any layer` forms. Scope is unchanged, one bullet is simply already half-done.
- **0102's two bespoke asserts are intact** (the `any layer` tag assert and the negative stale-prose
  guard) and stay, per assumption 5.
- **No writer contention materialized.** Assumption 9's flagged co-writers of the same test file
  and example file are all still un-started: 0121 is `proposed`/needs-brainstorm, 0103 is
  `proposed`/needs-brainstorm. Siblings 0126 and 0130 touch different test files. Merge risk is
  textual only and currently nil.

Auto-capture: the spec's out-of-scope observation that the `(2c)` orphan-key check is equally
column-0 anchored was minted as its own stub (see below), rather than left as prose.
