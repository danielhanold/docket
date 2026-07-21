---
slug: correspondence-guard-runs-one-way
hook: "A guard over a correspondence between two sets proves only the direction it iterates — write the reverse loop too, and anchor it on the consuming code, not an allowlist."
topics: [testing, coverage, sentinels]
changes: [101, 107, 104, 102]
created: 2026-07-20
updated: 2026-07-21
promotion_state: candidate
promoted_to:
---

## Apply
Whenever a guard enforces that two sets **correspond** — every key the code reads is documented,
every documented flag exists, every exported symbol has a contract, every skill has a wrapper —
name which direction it iterates, then write the other one. A loop over side A proves only
`A ⊆ B`; anything in B with no counterpart in A is **structurally invisible to it**, and it will
sail past the neighbouring guards too, because they were built for the forward direction:

- a **fidelity diff** is blind to an orphan, since a resolver simply *ignores* keys it does not know;
- a **format/scope-tag check** is satisfied by a neighbouring entry's window, not the orphan's own;
- an **allowlist of known exceptions** answers "is this one expected?", never "does this one exist?".

Mutation-test in **both** directions and treat that as the completion bar: delete a real entry from
B and watch the forward loop redden, *and* add a phantom entry to B and watch the reverse loop
redden. Anchor the reverse check on the **consuming code** — grep the scripts that actually read
the key — never on a hand-maintained list of exceptions, because that list is itself an enumerated
floor ([[enumerated-floor]]) and ages directly into the gap it was written to close.

The stakes scale with the artifact: on a file whose entire deliverable is *documentation accuracy*,
the orphan direction is the one that ships a documented flag that does not exist — the drift the
change existed to end ([[verify-the-claim]]).

## War story
- 2026-07-20 (#101, PR #109) — `.docket.yml.example` shipped as the canonical config reference behind
  three guards, all one-directional. Completeness `(2a)` drove its loop off the **resolver's actual
  export surface** and was honestly mutation-proven (a new export key fails until documented); `(2b)`
  allowlisted the four non-exported schema keys (`github_project`, `agents`, `agent_harnesses`,
  `finalize.require_pr_approval`). Together they proved every key the CODE reads is documented.
  Nothing proved the converse. A phantom key added to the example passed **all three**: `(2a)`
  iterates export keys rather than example keys, the fidelity diff is blind because the resolver
  ignores unknown keys, and the scope-tag awk was satisfied by a neighbouring key's comment window.
  Caught by the whole-branch review, not the suite. Closed by a new `(2c)` orphan-key check anchored
  on the consuming scripts rather than on the `(2b)` allowlist — deliberately not "extend the
  allowlist", since the allowlist is the enumerated floor that made the gap. Mutation-verified in
  both directions.
- 2026-07-20 (#107, PR #110) — **The exception that scopes this rule: when the two sets are
  deliberately in a SUBSET relation, the reverse loop is the defect.** The README's five-key
  illustrative snippet is guarded against `.docket.yml.example` forward-only — iterate the snippet's
  keys, assert each exists in the example with a matching value, never iterate the example's keys.
  Writing the reverse loop here would assert the README shows every key, which is exactly the
  fourth all-keys surface #101 existed to delete. So ask first **whether the correspondence is a
  mirror or a proper subset**; this rule's "write the other direction too" binds only on mirrors.
  The obligation does not vanish, it changes shape: with no reverse loop, **vacuity becomes the live
  risk**, so the subset direction must carry its own corpus asserts — here an exact-count assert on
  the extracted keys (a broken heading or fence must redden rather than pass with an empty loop) and
  a filtered-vs-raw cross-check ([[guards-are-code]]). Two costs to accept deliberately: the exact
  count reddens on any legitimate addition to the snippet (chosen over `>= 1` precisely so the
  snippet cannot creep back toward being a mirror — the remedy is inlined in the assert's failure
  message), and the departure must be written **into the test as a comment**, or a later reader
  "fixes" the missing reverse loop and re-creates the surface. Recorded here rather than only in the
  spec because a rule this finding states absolutely will otherwise be applied absolutely.
- 2026-07-20 (#104, PR #113) — The zero-way case. `board-checks.sh`'s emitted check-id set has
  **three** registration surfaces (the script header, `scripts/docket-status.md`'s enumeration, the
  test file) and *nothing* binds any of them to the emitted set — not one direction, let alone two.
  Drift was found in **both** directions in one change: the spec knew the script header omitted
  `malformed-id`, and reconcile found the converse — the `docket-status.md` enumeration omitted
  `stale-finalize-blocked`, which change **0098 shipped without ever registering**. Both repaired,
  but the structural gap recurs on the next check-id added; tracked as change **#111**. The tell
  worth reusing: when a set has three hand-maintained mirrors and no guard, assume drift in every
  direction and *derive* the true set from the emitting code before trusting any mirror.
- 2026-07-21 (#102, PR #115) — **Both directions can be live and the guard still passes, because the
  PAIRING was never asserted.** The documented-but-unwired-key guard iterated correctly in both
  directions and was still defeated by renaming `finalize.require_pr_approval` →
  `finalize.require_approval` in the example and copy-pasting the classify arm: the assert checked
  that a `resolved:`-classified key had *an export*, never that the export was **that key's**. The
  old export still emitted, so the rename reproduced the original documented-but-unwired bug
  verbatim, green. A correspondence guard needs three things, not two — forward, reverse, **and the
  binding between paired elements**; set-membership on each side proves neither element is missing
  while saying nothing about whether they are matched to each other. Closed by tying the export name
  back to its leaf key inside the resolver, and mutation-verified at HEAD rather than reported.
  Three more escapes in the same guard, each an anchor that made the assert unconditionally true:
  (a) `elsewhere:HEADER` was an **unverified escape hatch** — nothing checked a HEADER-classified
  key was a real block opener, so relabeling one key re-opened the whole bug class in a one-word
  edit (now requires a bare opener *with* a more-indented child); (b) `elsewhere:` targets were
  **unconstrained**, so a key could anchor on `.docket.example.yml` — the very file documenting it —
  making the correspondence self-satisfying (targets now constrained to a declared consumer
  allowlist, the one place an allowlist is right: it bounds the *anchor surface*, not the *key set*);
  and (c) `sort -u` **absorbed leaf-name collisions**, so a new key whose leaf matched an
  already-classified one was invisible — `learnings.gate` colliding with the flat-read
  `finalize.gate` passed green, which is also a live mis-resolution hazard for `yaml_get`'s
  `head -n1`. Whenever a guard de-duplicates, ask what real distinctions the de-dup key erases.
