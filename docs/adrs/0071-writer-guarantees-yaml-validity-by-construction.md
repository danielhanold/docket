---
id: 71
slug: writer-guarantees-yaml-validity-by-construction
title: A writer guarantees YAML validity by construction; a checker's predicate is detection only
status: Accepted
date: 2026-08-07
supersedes: []
reverses: []
relates_to: [62, 65]
change: 235
---

## Context

Change 0235 repairs six change files whose unquoted `title:` scalars fail a real YAML parse. Its
groomed spec proposed one shared `docket_scalar_needs_quoting` predicate consumed by both
`mint-stub.sh`'s `set_field` (the **writer**) and `board-checks.sh`'s `scalar_form_check` (the
**checker**), with the writer quoting only when the predicate fires. The predicate carried five
syntax legs plus a flow-collection exemption, and the spec's stated stance was
no-key-enumeration — the predicate judges the value, never the field name.

A test-oracle question forced a re-think. The natural way to prove a writer emits valid YAML is to
parse its output — and Python has no stdlib YAML parser. Verified on this machine:
`/opt/homebrew/bin/python3` carries PyYAML 6.0.3 only because something pip-installed it, while
`/usr/bin/python3` raises `ModuleNotFoundError: No module named 'yaml'`. The JSON precedent in
`tests/test_cursor_permissions_docs.sh` falls back to `python3 -m json.tool`, which is safe
*precisely because* `json.tool` is stdlib; there is no YAML equivalent. So a parser-based assert
means a hard third-party dependency on a repo that is otherwise pure bash + git + gh — squarely
against ADR-[[0062]] — and a skip-when-absent assert goes silently vacuous, which is the exact
failure class change 0235 exists to fix.

A conditional-quoting writer *needs* that oracle, because its correctness is a claim about an
open-ended input space: every input class the predicate fails to enumerate is a latent corrupt
file, and only sampling through a parser can look for the classes nobody thought of.

## Decision

Remove the need for a parser oracle rather than acquire the dependency.

1. **The writer always quotes.** `mint-stub.sh`'s `set_field` **unconditionally** single-quotes the
   free-text scalars it writes, instead of quoting conditionally. The rule is **scoped by key**,
   and the scope is tiny: of `set_field`'s seven calls in `mint-stub.sh`, exactly one carries
   free-text prose — `title`. `slug` is slugified; `id`, `created`, `updated`, `type`, and
   `discovered_from` are an integer, two ISO dates, an enum, and a `[…]` list.

2. **Validity holds by construction, not by enumeration.** A single-quoted YAML scalar has exactly
   one escaping rule — an embedded `'` doubles to `''` — with no backslash escapes and no
   interpolation. A correctly-escaped single-quoted scalar therefore cannot be invalid for *any*
   input that already passes `set_field`'s existing control-character check. There is no dangerous
   input class left to enumerate, and so no predicate leg left to omit.

3. **The output shape is fully determined, so a byte-level assert on the emitted line IS an
   assertion of validity** — provable by inspection rather than sampled through a parser. No
   parser, no optional dependency, no vacuum.

4. **The predicate survives on the detection side only.** `board-checks.sh`'s `scalar_form_check`
   inspects hand-authored files it did not write and must judge arbitrary scalars, so it keeps the
   general, key-agnostic predicate. There a false negative costs a missed warn-only finding, never
   a corrupted file.

The generalizable rule: **a component that writes a file guarantees well-formedness by construction
and may scope that guarantee by key, because it knows the provenance of every field it emits. A
component that only inspects files it did not write cannot scope by key and needs a general
predicate — and its predicate is a detector, never a licence for the writer to skip quoting.** Where
both exist, they are asymmetric by design, not two consumers of one shared rule.

The rule is stated for `mint-stub.sh` but binds any docket writer emitting frontmatter.
`archive-change.sh` carries its own `set_field` copy, writing `status`, `updated`,
`claimed_at ""`, and `results` — none free-text prose today, so it needs no change now; it inherits
this rule the moment it ever writes prose.

## Consequences

- **This revises the spec's conditional-quoting design and its no-key-enumeration stance, on the
  write path only** — a deliberate divergence, recorded here rather than discovered later as an
  oversight. Key-scoping is sound for the writer because it knows which of its own fields are
  free-text prose; the checker does not, which is why the general predicate stays on the detection
  side.
- It dissolves the round-two critic finding that leg 5 would fire on `discovered_from: [234]` and
  turn a sequence into a string. The always-quote rule never touches list fields, so the
  flow-collection exemption becomes unnecessary on the write path.
- **`_docket_unwrap_quotes` MUST gain the exact inverse leg** (`''` → `'` inside a single-quoted
  token) or `BOARD.md` ships `manifest''s`. This is now an unconditional obligation rather than an
  edge case, since every newly minted title is quoted — which is a net benefit: the inverse is
  exercised on every mint instead of rarely.
- Raw files become slightly less readable (every minted title carries quotes), and the quoting is
  visible in `git diff` on files that did not need it. Accepted: uniformity is what removes the
  decision, and removing the decision is what removes the oracle.
- docket keeps its no-external-YAML-parser boundary (ADR-[[0062]]) without buying a vacuous test in
  exchange.
- ADR-[[0065]] made the complementary point about *reading* — that a bare-scalar claim built on a
  raw-vs-consumed comparison needs an explicit quote leg. This ADR is its write-side counterpart:
  quoting is now the writer's default rather than a case it must detect.

## Update — 2026-08-07 (change 0235)

The Consequences above say the flow-collection exemption "becomes unnecessary on the write path" but
"stays in the checker's predicate, where an arbitrary hand-authored value may still be a list." That
last clause no longer holds: change 0235's whole-branch review removed the exemption from
`docket_scalar_quote_reason` entirely. Evaluated first, it suppressed all five syntax legs rather
than only the indicator leg, making previously-reachable findings unreachable — and no ordering
rescues it, since a flow map's `key: value` is a colon-space by construction.

The **Decision** is unchanged and unreversed — `mint-stub.sh`'s writer still quotes `title`
unconditionally, validity still holds by construction, and the byte-level assert still stands in for
a parser oracle. Only the checker-side disposition of the exemption changed. See ADR-[[0073]].
