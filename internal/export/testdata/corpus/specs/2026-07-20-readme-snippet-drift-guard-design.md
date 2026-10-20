# Guard the README config snippet against `.docket.yml.example` drift — design

Change: 0107 · Date: 2026-07-20 · Status: settled

## Problem

Change 0101 made `.docket.yml.example` the single canonical config reference and deleted the other
all-keys surfaces. Mid-review it found a fourth one still standing: the README carried a full
commented `.docket.yml` sample, already drifted (no `learnings:` block, no `auto` sentinel). It was
cut down to a five-key illustrative snippet plus a pointer to the example — the right shape, but
nothing tests it. The snippet's values can silently diverge from the canonical file's, and the
pointer can rot if the example moves. 0101's results file records this as an accepted, unguarded
residual; this change closes it.

## Decision

Add one new numbered section — **`(8) README SNIPPET CORRESPONDENCE`** — to
`tests/test_docket_yml_example.sh`, placed after the existing `(7) README + dogfooding`.

**Why that file, not a README doc-test of its own** (open question 1): it already owns every
example-mirroring invariant, which is what ADR-0048's must-update rule points at. A second file
guarding the same artifact from the other side is a coordination surface with no payoff; the
existing `(7)` already asserts README facts, so the precedent is set.

## The guard

Extract the fenced YAML block under the README's `### `.docket.yml` — per-repo settings` heading,
flatten it to dotted key paths with values, and for each extracted key assert:

1. the key path exists in `.docket.yml.example`, and
2. the value shown in the README equals the value on the example's corresponding line.

Plus a pointer assert: the README's link target for the canonical reference resolves to a file that
actually exists at that path (guards the rename/relocate case).

### Nesting is handled generically

The snippet already nests (`finalize:` → `gate: local`), and more nested keys are expected over
time, so extraction parses indentation into dotted paths (`finalize.gate`) rather than hardcoding
the one known nested key. Both sides — snippet and example — go through the same flattener, so the
comparison is path-to-path. A shared helper is fine; it need only cover the block-mapping subset
YAML that both files actually use (two levels, scalar and inline-list values). It is a test helper,
not a general YAML parser — do not grow it beyond what the two files contain.

### Direction: forward only, deliberately

The guard iterates the **snippet's** keys (`S ⊆ E` plus value agreement). It does **not** iterate
the example's keys.

This is a conscious departure from the `correspondence-guard-runs-one-way` learning harvested from
0101, which says: name the direction you iterate, then write the other one. Here the reverse loop
*is* a completeness assert — "every key in the example appears in the README" — and that is exactly
the fourth all-keys surface 0101 deleted. The snippet is a deliberate proper subset: a small taste,
not a mirror. Writing the reverse direction would undo the change that motivated this one.

The orphan direction that actually bit 0101 — a documented key that no real surface carries — is
already covered by the forward loop here: a snippet key absent from the example fails assert (1).
The asymmetry is safe *because* the two artifacts stand in a subset relation rather than a
correspondence, which was not true of 0101's export-keys-vs-example guards.

**This reasoning must be written into the test as a comment.** A future reader who has internalized
the learning will otherwise "fix" the missing reverse loop and reintroduce the drift surface. The
comment states: the reverse loop is intentionally absent, and why.

### Non-vacuity floor

The forward loop's real failure mode is iterating an empty set: rename the fence, retitle the
heading, or move the section, and the extractor finds zero keys — the loop runs zero times and the
test passes while proving nothing. `(1)` already carries this defense for its fidelity fixture
(lines 48–55), for the same reason.

So assert the extracted key count **equals the number the snippet actually shows** (currently 5:
`metadata_branch`, `integration_branch`, `board_surfaces`, `finalize`, `finalize.gate` — settle the
exact count against the flattener's output when implementing), not `>= 1`. An exact count is the
right bar: it also catches a snippet that quietly grows toward being a mirror again.

## Completion bar

Mutation-prove each assert before calling it done:

- change a value in the README snippet → the value assert reddens;
- add a key to the snippet that is not in the example → the existence assert reddens;
- break the heading or the fence so extraction yields nothing → the non-vacuity assert reddens;
- point the README's canonical-reference link at a nonexistent path → the pointer assert reddens;
- add a nested key two levels deep to both files consistently → everything stays green (proves the
  flattener is generic, not accidentally passing on the one hardcoded path).

## Out of scope

- Regenerating the snippet from the example. Codegen was rejected for the example itself in
  ADR-0048; the same hand-maintained-mirror trade-off applies, and a five-key taste is not worth a
  generator.
- Auditing the README's other prose claims against the code — broader than this snippet and not
  grep-able (see the `verify-the-claim` finding).
- Any reverse/completeness direction, per the reasoning above.

## No ADR

The direction-of-truth call is a test-design decision scoped to one guard, and it is recorded here
plus in the test's own comment. ADR-0048 already owns the standing rule this sits under.
