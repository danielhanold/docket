<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0122 — Nested keys' scope tags in .docket.example.yml are unguarded](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0122-nested-keys-scope-tags-in-docket-example-yml-are-unguarded.md)**
<!-- docket:backlink:end -->

# Design — guard nested keys' scope tags in `.docket.example.yml` (change 0122)

## Problem

`tests/test_docket_example_yml.sh`'s scope-tag guard anchors on a column-0 key pattern
(`is_active = ($0 ~ /^[A-Za-z_][A-Za-z0-9_]*:/)`, line 624; the `H`/`S` header-vs-scalar split at
line 631 keys off the same anchor). Every nested key in the file is therefore invisible to it, and
a block header's comment window is extended *forward through its nested body*, so any one child's
tag satisfies the header — masking a wrong or missing tag on a sibling. Change 0102 shipped
`finalize.require_pr_approval` with a bespoke annotation asserting the opposite of its real scope
and the suite stayed green.

## Decision — the guard's rule set

The generalized check evaluates keys at **every** indentation depth, under four rules:

1. **Leaves carry the obligation.** A *scalar* key (anything after the colon) must be **covered**.
   A *header* key (mapping opener — nothing after the colon) is never itself required to carry a
   tag; it may *provide* one for its subtree.
2. **Coverage = own tag, else nearest tagged ancestor.** A key is covered if its own comment
   window contains a sanctioned tag, else if any enclosing header block's own window does.
3. **A header's window is its own preceding comment lines only** — never extended forward into its
   body. This is the anti-masking rule, and it is what makes the 0102 regression RED: the
   `finalize:` header carries no tag of its own, so each of its three children must be tagged
   individually, and a child whose window holds only a bespoke note is uncovered.
4. **Same-depth adjacency inheritance is retained.** A scalar key with a genuinely empty window,
   immediately adjacent to the previous key at the same depth, inherits that key's coverage —
   the existing `changes_dir` / `adrs_dir` / `results_dir` shared-comment group.

Sanctioned tag forms are the three the file's legend defines:
`scope: repo-only (coordination-fenced, ADR-0019)`, `scope: any layer`, and **`scope: local-only`**.
The third is accepted forward-looking only: its two occurrences are the legend (line 38) and line
56, which sits above the **commented** `# runtime:` key and is sealed from every active key by the
banner at line 61 — so no active key at any depth carries it today, and adding it to the accepted
set is inert against the current file.

**Consequence, verified against the current file: zero edits to `.docket.example.yml` are
required.** `finalize` / `learnings` / `reclaim` tag each child individually; `auto_capture`,
`runners`, and `skills` tag the block header and their children inherit — both conventions are
legal under rules 1–3, so the file's existing mix is not a defect to normalize.

## What gets built

- Rewrite the `untagged_keys` awk pass in `tests/test_docket_example_yml.sh` to implement rules
  1–4. Track `(line, depth, type)` per key; maintain a depth-indexed stack of the innermost tagged
  header at each level; report failures as **dotted paths** (`finalize.gate`) so the diagnostic
  names the key, not an indented fragment.
- Boundaries for a window stay what they are today, generalized to any depth: a section banner
  (`# ═══`), any active key, or a commented pseudo-key (`# agent_harnesses:` / `# agents:`).
- Add `scope: local-only` to the accepted tag set (and keep the existing three
  `scope tag: <form> present` presence asserts).
- **Population floor — EXACT count, emitted by the guard itself.** The count MUST come from the
  hoisted guard program's own stdout (e.g. a `COUNT <n>` line), never from the qualified extractor
  at `tests/test_docket_example_yml.sh:302-321` — reusing that extractor would keep the floor green
  while the guard's own pass enumerates zero nested keys, which is exactly the
  `correspondence-guard-runs-one-way` vacuity this bullet exists to prevent. Assert the pass
  enumerated exactly **17** keys at depth > 0
  (3 `finalize.*`, 2 `learnings.*`, 2 `reclaim.*`, 2 `auto_capture.*`, `runners.codex` + its 2
  leaves, 5 `skills.*`), with the same "add your key in the same commit" failure message the flat
  read-key floor at `tests/test_docket_example_yml.sh:489-491` already models. An at-least-N floor
  is rejected: `>= 15` is satisfied by the *pre-0102* file and tolerates a regression that silently
  drops both `runners.codex` leaves. (learnings:
  `marker-scoped-guard-needs-a-population-floor` — which closes on exact-count —
  `correspondence-guard-runs-one-way`, `enumerated-floor`.)
- **Guard-the-guard mutation self-test.** Hoist the awk program text into a single shell variable
  so the self-test runs **literally the same program** — a hand-copied second inline copy is the
  `plan-supplied-test-code-is-unverified` vacuity. Assign it with a single-quoted heredoc so `$0`
  is not shell-expanded. Then: copy `$EX` to a temp file, delete the `scope:` line immediately
  above the `  gate:` key in the copy, run the hoisted program over it, assert it reports
  `finalize.gate`. Same for an inheriting child (delete the `scope:` line immediately above
  `skills:` → all five `skills.*` reported). Anchor both deletions **by content, never by line
  number** — the floor assert explicitly anticipates the file gaining keys. Never mutate the real
  file. (learnings:
  `guards-are-code`, `transient-resource-lifecycle` for the temp-file lifecycle.)
- **Update the guard's own prose, label, and failure diagnostic.** Three sites all assert the
  now-false "top-level" framing and must be rewritten in the same commit:
  `tests/test_docket_example_yml.sh:607-617` (documents the forward-extension behavior rule 3
  deletes), the assert label at `:664` ("every ACTIVE **top-level** key"), and the operator-facing
  failure block at `:665-668`, which prints `--- untagged top-level keys ---`.
- Portability: POSIX awk only — no `gensub`, no GNU extensions; depth via
  `match($0, /^[ ]*/)`. Re-verify with `/usr/bin/awk` and `/usr/bin/grep`, not the PATH tools
  (learnings: `shell-portability`, `agent-shell-noop-reads-as-success`; the ugrep/`{0,600}` trap).

## Out of scope

- Changing any key's actual scope, or the coordination-key fence.
- Normalizing the file onto one tagging convention (rules 1–3 make both legal).
- The manifest classification guard and the `(2c)` orphan-key check — separate mechanisms. (Noted:
  `(2c)`'s own key enumeration is column-0 anchored too, so nested keys are equally invisible to
  the orphan-key direction. That is a *different* guard with a different consumer anchor; it is
  recorded here as a follow-up observation, not folded into this change.)

## Assumptions

Each decision below was defaulted autonomously; alternatives and rationale are the deferred audit
trail.

1. **Inheritance rule — nested keys inherit an ancestor block's tag; they need not each be tagged.**
   Rejected: *every nested key must carry its own tag* — it forces ~10 new tag lines into
   `auto_capture`, `runners`, and `skills`, duplicating a scope that is genuinely block-level, and
   turns a guard change into a content churn. Rejected: *tags only ever on the block header* —
   would force deleting the per-child tags on `finalize`/`learnings`/`reclaim`, discarding the
   finer granularity that the 0102 fix deliberately added. Chosen rule keeps the file byte-stable
   and is strictly more sensitive than today's.

2. **A header's window never extends forward into its body.** This is the load-bearing half: it is
   what un-masks a sibling. Rejected: keeping the forward extension and adding a separate
   per-child check — two overlapping mechanisms, and the extension would keep granting coverage
   the child check is trying to deny.

3. **Header keys are exempt from carrying a tag.** Rejected: requiring one — `finalize:`,
   `learnings:`, `reclaim:`, and `runners.codex:` carry none today, so it would demand edits and
   assert a scope on a container that has none of its own.

4. **`scope: local-only` joins the accepted set — forward-looking, inert today.** The form is one
   of the three the file's legend (line 38) defines, and the guard's accepted set should match the
   legend rather than the subset currently in use. It closes no live hole: its only other
   occurrence (line 56) documents the *commented* `# runtime:` key, which the guard never
   enumerates. Rejected: leaving it out — the first active local-only key to land would then fail a
   guard for carrying a legend-sanctioned tag. Widening acceptance can only reduce false failures.

5. **0102's two bespoke asserts: KEEP both, do not retire.** The stub's retire instruction is
   conditional on the general check *covering* them, and it does not. The tag assert (line 517)
   pins `require_pr_approval` to the **specific** `any layer` value; the general check by design
   only proves *some* sanctioned tag covers the key, so retiring it would silently accept a
   relabel to `repo-only` — the exact 0102 bug class. The second assert (line 519) is a negative
   guard on removed prose, orthogonal to tagging entirely. Rejected: retiring the tag assert for
   single-location maintenance — the two asserts test different propositions, so this is not
   duplication. Rejected: generalizing the guard to check tag *values* against a per-key expected
   table — that is a hand-maintained allowlist, the thing `(2c)` was written to avoid.

6. **Extend the existing awk in place rather than extracting a script.** Rejected: a new
   `scripts/check-scope-tags.sh` — the guard has exactly one consumer, and extraction adds a
   contract file and a facade op for no reuse. (learnings: `skill-extraction-and-stub-pointer`,
   same economics.)

7. **Failure output is dotted paths.** Rejected: raw key names — `enabled` alone is ambiguous
   between `learnings.enabled` and `auto_capture.enabled`.

8. **The `(2c)` orphan-key check is left column-0-anchored.** Rejected: extending it in the same
   change — it anchors on *consumers*, and nested keys reach their consumers through different
   paths (`runners.*` via the runner adapters, `skills.*` via `SKILL_*` exports), so it is a
   separate design question, not a mechanical widening. Recorded above as an observation.

9. **Dependency state:** `depends_on` is empty; nothing gates this. Siblings 0126 (`eval`
   poison-value hygiene in `tests/test_docket_config.sh`) and 0130 (a BSD-portability interval in
   `tests/test_finalize_disposition.sh:186`) touch different files. But this change is **not** the
   only open writer of `tests/test_docket_example_yml.sh`: **0121** rewrites the classification
   manifest / `elsewhere:` check in the same file (`:324-330`, `:522-554`) — a disjoint region, so
   the exposure is textual-merge risk only, not a design conflict. **0103** may wire
   `github_project` (`.docket.example.yml:175`); if it adds nested keys it will trip the exact-17
   floor, which is the floor working as designed (update the count in that change's commit).
   Reconcile re-validates all of this at build time.

10. **Verification standard:** the change is done when the new pass is green on the unmodified
    file, both mutation self-tests are RED-on-mutation, the **exact** 17-key nested floor holds,
    the guard's own prose and assert label no longer describe the deleted behavior, and the suite
    runs clean under `/usr/bin/awk` + `/usr/bin/grep`.

11. **Accepted residual (raised by the critic, deliberately not designed away):** a *wrong but
    well-formed* tag is never caught — on a header it masks the whole subtree, on a leaf it masks
    that leaf. Same root cause, same rejected remedy. This is the direct cost
    of choosing inheritance (assumption 1) over per-key tags, and it is a strictly smaller hole
    than today's (where a *child's* tag masked the whole block in both directions). Detecting a
    wrong-but-well-formed tag needs per-key expected values, i.e. the hand-maintained allowlist
    assumption 5 rejects.
