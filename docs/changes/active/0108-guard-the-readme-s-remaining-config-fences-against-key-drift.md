---
id: 108
slug: guard-the-readme-s-remaining-config-fences-against-key-drift
title: Guard the README's remaining config fences against key drift
status: in-progress
priority: medium
created: 2026-07-20
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [107]
adrs: []
spec: docs/superpowers/specs/2026-07-20-readme-config-fence-key-drift-guard-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/guard-the-readme-s-remaining-config-fences-against-key-drift
claimed_at: 2026-07-21T16:29:36Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-readme-config-fence-key-drift-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-readme-config-fence-key-drift-guard-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0107 added `(8) README SNIPPET CORRESPONDENCE` to `tests/test_docket_example_yml.sh`,
guarding the README's per-repo-settings `.docket.yml` snippet against drift from
`.docket.example.yml`. That guard is deliberately scoped to **one fence** — the section's single
worked example — and its whole-branch review surfaced that the README carries **eight other config
fences that nothing guards at all**:

- `auto_capture: true` (README:264)
- `terminal_publish: true` (README:407)
- `metadata_branch: main` (README:433)
- the `reclaim:` block (README:234)
- the global `config.yml` sample (README:289) and the `.docket.local.yml` sample (README:310)
- the `skills:` binding example (README:576 — **indented**, inside a list item) and the
  runner-delegation sample (README:594)

Each is a place a documented key name or value can rot exactly the way the per-repo snippet could
before 0107 — a key renamed in the resolver, or a key that never existed, would sit in the README
indefinitely.

*(Grooming note: this list is the corrected one. As filed, the stub omitted the `reclaim:` fence and
mis-stated two filenames — `tests/test_docket_yml_example.sh` and `.docket.yml.example` do not
exist. That the hand-written list was already wrong on arrival is the change's own argument for
deriving the fence set rather than enumerating it.)*

The reason 0107 did not simply extend its loop is recorded in its own test comment: those fences
**deliberately show NON-default values** to illustrate opting in, so 0107's value-equality assert
would go spuriously RED against them for being correct. Guarding them needs a different assert —
key **existence** in `.docket.example.yml` without value comparison — which is a real design call,
not a mechanical copy of the existing section.

## What changes

Add a new section `(9) README CONFIG FENCE KEY CORRESPONDENCE` to `tests/test_docket_example_yml.sh`
(note: the stub originally named this file `test_docket_yml_example.sh` — that file does not exist).
Section `(8)` is left byte-untouched.

- **The fence set is derived, never enumerated.** The check scans `README.md` for ```` ```yaml ````
  fences and puts every one in scope by default. The stub's own hand-written fence list was already
  wrong when it was filed — it omits the `reclaim:` fence — which is the argument for deriving.
  `README.md` carries **nine** such fences; one of them (the `skills:` example at README:576) is
  **indented**, so the discovery regex must be whitespace-tolerant. A column-0 regex silently misses
  it, which is exactly how the design's own first draft miscounted them as eight.
- **Anchored on `.docket.example.yml`, one hop.** Sections `(2a)`/`(2b)`/`(2c)` already bind the
  example to the resolver in both directions, so the README inherits resolver coverage transitively
  without a second competing anchor.
- **Existence-only by default.** This is what makes one check applicable to all nine fences, and it
  dissolves the stub's third open question: a fence showing deliberately non-default values never
  has to declare anything. Value equality stays where it is sound — `(8)`'s fence — plus an opt-in
  marker (`<!-- docket:config-fence: values -->`) applied to the `reclaim:` fence, whose `72`/`false`
  are shipped defaults. A matching `ignore` marker exempts a future non-config yaml fence; an
  unknown or malformed marker token is a hard fail, never warned-and-ignored.
- **A blocking prerequisite:** `flatten_yaml`'s key class excludes hyphens, so it silently drops
  `implement-next:` from two README fences. It must be widened at **both** occurrences (the shape
  test and the value strip) — verified behavior-neutral for sections `(1)`–`(8)`.
- **Non-vacuity is the live risk** and carries explicit floors (exact fence count, per-fence
  non-empty flatten, raw-vs-flattened cross-check) plus four required mutation tests, because a
  guard that discovers zero fences passes green while proving nothing.

## Out of scope

- Re-litigating 0107's forward-only direction, or adding any reverse/completeness loop over the
  example's keys — that is the all-keys surface change 0101 deleted.
- Auditing the README's non-config prose claims (see the `verify-the-claim` finding).
- Any change to `.docket.example.yml`'s content, or to sections `(1)`–`(8)`.
