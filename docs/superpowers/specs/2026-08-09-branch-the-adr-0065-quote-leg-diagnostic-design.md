<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0265 — Branch the ADR-0065 quote-leg diagnostic so it stops claiming a truncation that did not happen](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0265-branch-the-adr-0065-quote-leg-diagnostic-so-it-stops-claimin.md)**
<!-- docket:backlink:end -->

# Branch the ADR-0065 quote-leg diagnostic — design

Change: 0265. Auto-groomed 2026-08-09 (docket-auto-groom; critic-gated).

## Problem

**Leg 1 — the diagnostic.** ADR-0065's two-legged bare-scalar validator emits one message for both
legs: `… value '$raw' is not a bare scalar — the reader consumes only '$consumed'; write
model/effort values unquoted and space-free`. On the *truncation* leg (`consumed != raw`) the two
strings differ and the sentence is the point. On the *pure quote* leg (quoted but space-free:
`consumed == raw`, leading `"`/`'`) both sides print the identical string — the message claims a
truncation that did not happen and reads as a validator bug.

**Leg 2 — the stale contract claim (absorbed from killed #0267).**
`scripts/render-learnings-index.md` ("Dequoting" paragraph) states "`field()` returns the raw
scalar with surrounding quotes intact" — false since change 0138: `field()`/`fm_field()` strip a
matched surrounding quote pair; `field_raw()`/`fm_field_raw()` are the raw accessors, and
`render-learnings-index.sh` in fact reads `hook` via `field_raw` (script comment at the
`dequote()` definition confirms).

## Design

### Leg 1 — branch the message per firing leg

At each of the three diagnostic sites — whose full strings differ (prefix/suffix per file) but
share the byte-identical middle clause — split the single `elif` message on `consumed != raw`:

- **Truncation branch** (`consumed != raw` — embedded space, quoted-with-space, any value the
  class cannot consume whole): message **unchanged, byte-for-byte** — the truncation claim is true
  there, including the quoted-with-space case (`"a b"` → consumed `'"a'`), so no quoted-ness
  check is needed to pick the branch.
- **Quote branch** (`consumed == raw` and the raw value leads with `"` or `'`): new message —
  `… value '$raw' is not a bare scalar — the quotes are ordinary characters to this reader and
  would ride into the emitted pin verbatim; write model/effort values unquoted and space-free`.
  Same prefix (`… value '$raw' is not a bare scalar — `), same remedy tail, no "consumes only"
  clause and no repetition of the value.

Sites (all three stay copies-by-value of one another in the shared clause, per the existing recorded stance that
extraction of a shared helper is #0256's scope, not this change's):

1. `sync-agents.sh` — the awk diagnostic inside `validate_harness_defaults` (the shipped-sidecar
   reader; the `else if (consumed != raw || lead == …)` line): becomes an if/else on
   `consumed != raw` inside the same guard. awk string escapes (`\042`/`\047`) as the surrounding
   program already uses — the new clause contains no literal apostrophe.
2. `sync-agents.sh` — the bash user-config validator `validate_user_agent_values` (the
   `elif [ "$raw" != "$consumed" ] || case …` site): same split.
3. `scripts/lib/harness-defaults.sh` — `hd_validate`'s `elif [ "$v" != "$raw" ] || case …` site:
   same split (`v` plays `consumed`).

Untouched: the firing predicates (both legs still refuse exactly what they refuse today), the
`#`-flow-map leg and its distinct message, the present-but-empty diagnostic, and the runners-shim
site (`runners.$r.$k … is not a bare scalar — write shim_model/shim_effort values unquoted and
space-free`), which never carried a truncation claim.

### Leg 1 — sentinels and tests

- Existing truncation-leg probes keep asserting `consumes only` — still true on that leg.
- Each quote-leg fixture (quoted, space-free value) gains a paired assert: the quote-branch clause
  **present** and `consumes only` **absent** in that firing's stderr — the absent-assert is the
  mutation-detecting half (a revert to the shared message must redden something; per the
  assert-detects-removal-not-replacement learning).
- Files to touch are wherever the existing quote-leg fire/ignore probes live today —
  `tests/test_sync_agents_validator.sh` and `tests/test_harness_defaults_flow_map.sh` — plus a
  check of `tests/test_sync_agents_runners.sh`'s bare-scalar block for any assert that pins the
  old single message on a quote-leg fixture. Plan derives the exact assert list from a repo grep
  for the pinned strings, never from this enumeration (AGENTS.md: never hand-list gated sites).
- Cross-site consistency: if a byte-identity guard over the three copies exists, it must compare
  both branch messages; if none exists, none is added (out of scope — #0256 owns consolidation).

### Leg 2 — contract correction (docs only)

In `scripts/render-learnings-index.md`'s "Dequoting" paragraph, replace the false sentence with
the true one: `field_raw()` returns the raw scalar with surrounding quotes intact (which is why
the renderer reads `hook` through it); `field()`/`fm_field()` strip a matched surrounding quote
pair (change 0138). Rest of the paragraph (matched-pair-only strip, unescape rules) is accurate
and stays. Repo-wide sweep performed at groom time: `git grep` over `scripts/*.md` for
`field()` + quote claims found exactly this one stale site (`render-board.md` and `mint-stub.md`
already state the stripping behavior correctly); the build re-runs the sweep to confirm, but no
other edits are expected. No code, no accessor change, no new guard.

## Out of scope

- Firing predicates, strip order, the `#` leg, which values are accepted.
- Extracting a shared validator helper (that is #0256; this change adds two more branched copies
  it must later consolidate or keep byte-identical — recorded as a forward `related:` link).
- Any wording that changes ADR-0065's or ADR-0076's decision — this is message text only.

## Assumptions

1. **Branch predicate is `consumed != raw`, not quoted-ness.** Alternatives: branch on the
   leading-quote test (wrong: quoted-with-space values genuinely truncate, and would get the
   quote message with a false "no truncation" implication); emit both clauses when both hold
   (noisier, no added information). Chosen: truncation message whenever a truncation actually
   happened; quote message only when `consumed == raw`. Conservative — the existing message is
   kept everywhere it is true.
2. **Quote-branch wording** as given above: shared prefix + shared remedy tail, new middle clause
   naming the actual failure ("quotes are ordinary characters to this reader and would ride into
   the emitted pin verbatim" — lifted from the sites' own comments, so the diagnostic says what
   the code already documents). Alternative: drop the middle clause entirely (prefix + remedy
   only) — rejected as less informative; the clause explains *why* quoting is refused rather than
   tolerated. Exact final phrasing may be tightened at build; the load-bearing properties are: no
   truncation claim, no repeated value, remedy tail byte-identical to the truncation branch's.
3. **All three copies change identically; no helper extraction.** Alternative: extract now —
   rejected; both files carry explicit recorded stances (comments + 0255's spec) deferring
   consolidation to #0256, and 0265 inverting that would enlarge a low-priority fix.
4. **Runners-shim site untouched.** It makes no truncation claim, so the stub's defect does not
   exist there; touching it would widen scope past the stub's stated boundary.
5. **Leg 2 is a one-file edit** unless the build's confirming sweep finds another stale site; the
   sweep pattern is by content (accessor name + quote claim), not a hand list.
6. **No ADR.** Diagnostic wording and a docs correction change no decision. ADR-0065 stays
   Accepted and accurate (it mandates the legs, not the message text); ADR-0076 untouched.
7. **Couplings**: `related:` gains 256 (forward link only — two more copies for the consolidator
   to reconcile); 267 stays (provenance of leg 2). `depends_on:` stays empty — no build-order
   dependency on #0256 in either direction; a rebase collision with an in-flight #0256 branch is
   handled at finalize's rebase gate, not by ordering.
8. **Dependency state**: none — `depends_on: []` and both legs verified live against today's
   tree (grep evidence in the change body and this spec).
