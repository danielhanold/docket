---
id: 108
slug: guard-the-readme-s-remaining-config-fences-against-key-drift
title: Guard the README's remaining config fences against key drift
status: done
priority: medium
created: 2026-07-20
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [107]
adrs: [53]
spec: docs/superpowers/specs/2026-07-20-readme-config-fence-key-drift-guard-design.md
plan: docs/superpowers/plans/2026-07-21-readme-config-fence-key-drift-guard.md
results: docs/results/2026-07-21-guard-the-readme-s-remaining-config-fences-against-key-drift-results.md
trivial: false
auto_groomable: true
branch: feat/guard-the-readme-s-remaining-config-fences-against-key-drift
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/116
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-readme-config-fence-key-drift-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-readme-config-fence-key-drift-guard-design.md) |
| Plan | [2026-07-21-readme-config-fence-key-drift-guard.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-21-readme-config-fence-key-drift-guard.md) |
| Results | [2026-07-21-guard-the-readme-s-remaining-config-fences-against-key-drift-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-21-guard-the-readme-s-remaining-config-fences-against-key-drift-results.md) |
| PR | [#116](https://github.com/danielhanold/docket/pull/116) |
| ADRs | [ADR-0053](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0053-readme-yaml-fences-guarded-by-default-opt-out-marker-grammar.md) |
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

## Reconcile log

### 2026-07-21 — reconciled against `main` at `5ed3d8c`

The spec was written against `main` at `6cb6be6`; since then changes 0102 (`require_pr_approval`
layer resolution) and 0083 (terminal-publish gap detection) merged. Every substantive claim in the
spec was re-verified empirically against the current tree and **all of them hold**. One class of
drift found, and it is cosmetic:

- **Test-file line numbers moved; the code did not.** The spec cites `flatten_yaml`'s key class at
  `:447` (shape test) and `:450` (value strip). Change 0102 grew `tests/test_docket_example_yml.sh`
  from ~500 to **884** lines by adding the `(2b)` classification manifest, so those two lines are now
  **`:782` and `:785`**. The substance is unchanged: `flatten_yaml` is defined at `:775–792` and
  still carries the hyphen-free key class at exactly **two** occurrences — the shape test
  (`if (line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:/) next`) and the value strip
  (`sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:[[:space:]]*/, "", val)`). §5's two-line widening stands
  exactly as written; the builder must locate the two occurrences **by their code**, not by the
  spec's stale line numbers.

Re-verified as still exactly true on `5ed3d8c`:

- **README fence ground truth — 9 fences at 209, 234, 264, 289, 310, 407, 433, 576, 594**, with 576
  indented two spaces. A whitespace-tolerant regex finds **9**; the column-0 regex finds **8**. §6
  floor 1's literal `9` and §1's whitespace-tolerant opener remain a matched pair.
- **Section `(8)`'s README span is 203–224**, terminated by the `### Reclaiming stale claims` heading
  at **225**. The `reclaim:` fence is at **234** and line **233** is blank, so the `values` marker
  lands outside `(8)`'s span and `snippet_section()` stays unperturbed.
- **`flatten_yaml` widening is behavior-neutral for `(1)`–`(8)`.** `.docket.example.yml` still
  contains **no** hyphenated active key, and `ex_flat` is **30 paths, byte-identical** before and
  after the widening.
- **§5's blocking prerequisite is real and still live.** On fences 289 and 310: raw = **11**,
  hyphen-free flatten = **10** — floor 3 would ship RED on correct prose exactly as predicted.
- **The half-fix trap reproduces exactly.** Widening only the shape test yields `flat=11` (floor 3
  passes, existence passes, `ex_flat` unchanged — nothing reddens) while
  `agents.default.implement-next` comes back carrying **the entire raw line**
  (`    implement-next: { model: claude-opus-4-8, effort: xhigh }`) as its value; the full widening
  yields the correct `{ model: claude-opus-4-8, effort: xhigh }`. The spec's instruction to assert
  the extracted **value**, not only the path count, is what catches this — it is not optional.

One builder note sharpened: §6's fixture-fence note says `(8)` "already parameterizes this way
(`snippet_section()` reads `$README`)". `snippet_section()` in fact reads the **global** `$README`
rather than taking a parameter. The instruction is unaffected — the new fence-scan helper must take
its markdown path as an **argument** so the `ignore`-path fixture fence can be scanned — but the
builder should not expect to find an existing parameterized helper to copy.

No scope change, no obsolescence, no invalidation. `depends_on: []` still holds (A10). Builds against
`main` at `5ed3d8c`.
