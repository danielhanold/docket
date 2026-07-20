---
slug: correspondence-guard-runs-one-way
hook: "A guard over a correspondence between two sets proves only the direction it iterates — write the reverse loop too, and anchor it on the consuming code, not an allowlist."
topics: [testing, coverage, sentinels]
changes: [101, 107]
created: 2026-07-20
updated: 2026-07-20
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
