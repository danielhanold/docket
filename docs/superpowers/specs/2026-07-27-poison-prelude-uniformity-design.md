<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0126 — Apply the poison-value prelude uniformly to every resolver eval in the config suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0126-apply-the-poison-value-prelude-uniformly-to-every-resolver-e.md)**
<!-- docket:backlink:end -->

# Design — Apply the poison-value prelude uniformly to every resolver eval in the config suite (change 0126)

## Problem

`tests/test_docket_config.sh` evaluates the resolver's `--export` output 63 times
(`out="$(run … --export)"; eval "$out"`). An aborted resolver run emits nothing, so a bare
`eval ""` is a no-op that leaves **the previous fixture's** exported variables standing. Any
following assert then reads stale state and can pass vacuously — the `guards-are-code` class (e)
failure the ledger already carries ("any test that `eval`s a command's output must clear the
variables it asserts on first").

The suite applies the discipline unevenly. Two clearing idioms exist today:

- **poison assignment** — `FINALIZE_TEST_COMMAND=__poison__` before the eval (section S, `s4`–`s9`),
- **unset + safe read** — `unset TERMINAL_PUBLISH` before the eval plus `${TERMINAL_PUBLISH-unset}`
  in the assert (the 0064 fence blocks, the 0067 learnings block, the reclaim block).

A mechanical scan (the line immediately preceding each `eval "$out…"` matching neither
`__poison__` nor `^unset `) finds **52 of the 63 `$out` eval sites unprotected**. The `L2`/`N`-family
site named by the stub (`tests/test_docket_config.sh:500`, asserting `BOARD_SURFACES`) is one of them.

**`eval "$out` is a spelling, not a shape, and the 63 is therefore a floor.** Two in-file sites the
spelling misses, both real:

- `tests/test_docket_config.sh:1521` — `eval "$runtime_odd_out"`, no clearing, and the assert two
  lines down reads `$DOCKET_BASH_PATH`, which earlier evals set. A genuine unprotected hazard site.
- `tests/test_docket_config.sh:763` — `AUTO_GROOM=""; eval "$z_sub_shell"`, where the clearing is
  **same-line**. A "preceding line" rule would wrongly flag it as a violation.

## Scope

**One file.** A whole-repo grep for the idiom matches only `tests/test_docket_config.sh` — no
sibling suite evals command output into asserted variables (`tests/test_sync_agents.sh:1112` evals a
captured *remedy command* for execution, not into asserts). That grep is the floor, and the build
re-runs it (case-insensitive, all file types) at reconcile rather than trusting this sentence.

**Discovery key — a shape.** The site set is *"`eval "$V"` where `V` was assigned from a command
substitution"*, with clearing accepted on the same line as well as the preceding one, and with
comment lines and the `assert()` helper's own `eval "$2"` (`:8`) excluded. The counts above are
`grep`-level approximations: the build derives the exact site count with that tokenizer and pins it
then.

## Design

### 1. The per-fixture rule

Clearing is **per-fixture, not a blanket line**. For each eval site, collect the exported variables
read by the asserts in the segment from that eval to the **next eval site** (one rule, not two —
today that boundary coincides with the fixture boundary), and clear **exactly
those** variables immediately before the eval. Copying section S's `FINALIZE_TEST_COMMAND=__poison__`
onto a fixture whose assert reads `BOARD_SURFACES` is decoration.

Eval sites whose following asserts read only `$out`/`$err` text (the `grep -qxF` shape) need no
prelude — the emitted-line asserts are already immune to the stale-value hazard, which is precisely
why block (D) asserts on the emitted line. **That exemption is derived, never enumerated**: a site
needs no prelude exactly when no exported variable name appears in the asserts between it and the
next eval. The same extraction the guard performs computes it, so no exemption list is written or
maintained.

### 2. The idiom

Standardize on the **poison assignment** (`VAR=__poison__`) for newly-protected sites:

- it keeps the variable defined, so existing asserts (`[ "$X" = … ]`) need no `${X-unset}` rewrite
  under `set -u` — a far smaller diff than converting 52 sites to the unset idiom;
- a leftover `__poison__` value is self-describing in a failure message.

Existing `unset`-idiom blocks are **left byte-untouched**: they already satisfy the invariant, they
carry explanatory comments, and rewriting them is churn. Where a variable's expected value could
legitimately be empty (`s4`/`s5` assert `-z`), the poison value is the correct choice anyway, since
`unset` and "resolved empty" would be indistinguishable.

### 3. The mutation demonstration (the completion bar)

The stub sets the bar: prove the hazard rather than assert it. Note that the demonstration cannot
run at the named site — at `:500` the previously-eval'd fixture leaves `BOARD_SURFACES=none`, so an
aborting run there makes the assert (`= "inline github"`) go **red**, not vacuously green. The
stale-value hazard is real but *latent*: it fires only when the stale value coincides with the
expected one. Seeding that coincidence by hand next to the prelude would prove only that the last
assignment wins.

**A natural coincidence already exists in the unmodified file, at the O→P boundary:** `:509` evals
block O leaving `AUTO_GROOM=false` (asserted at `:511`), `:518` evals block P, and `:520` asserts
`[ "$AUTO_GROOM" = false ]` — with nothing writing `AUTO_GROOM` in between. So:

1. make block P's resolver run abort (an unresolvable `origin/<integration_branch>` is a hard config
   error and emits nothing on stdout — every `die` in `scripts/docket-config.sh` precedes the first
   `emit`) and confirm `:520` reports **ok**, passing vacuously on block O's stale value;
2. add `AUTO_GROOM=__poison__` before `:518` and confirm the same mutation now reports **NOT OK**;
3. revert the mutation.

Record the observed outcomes and the suite's ok / NOT-OK counts in the results file, noting that in
step 1 the sibling asserts at `:519` and `:521` legitimately go NOT OK — the demonstration is about
`:520` alone, and the counts must not be read as "only `:520` changed".

That is the
completion bar; `:500` still receives its prelude, and the remaining sites ride the same argument.

### 4. The enforcement guard

Add one structural guard so the convention is enforced rather than remembered — and make it a
**correspondence** guard, not a presence guard. A presence-only guard is green on exactly the
regression it exists to stop (append a fixture asserting `BOARD_SURFACES`, prepend
`FINALIZE_TEST_COMMAND=__poison__`, hazard live, suite green), which would mechanically bless a wrong
convention — `guards-are-code` classes (b) and (g). Correspondence is mechanically reachable here
because the asserts are single-quoted `[ "$VAR" = … ]` bodies on the lines between evals.

Shape:

- **Discovery keyed on shape** (`eval "$V"` where `V` came from a command substitution), accepting a
  clearing on the same line or the preceding non-blank, non-comment line, in either idiom
  (`VAR=__poison__` / `VAR=""` / `unset VAR …`).
- **Correspondence**: extract the variable names read by the asserts in the segment between this eval
  and the next (matching `${VAR-unset}` as well as `$VAR` — otherwise every existing `unset`-idiom
  assert reads as an empty intersection and is wrongly exempted), intersect them with the resolver's
  exported key set, and require every surviving name to be cleared in that site's prelude. A site
  with an empty intersection is exempt **by derivation** — no list.
  The key set is **derived live at guard runtime** from the LHS of one shell-format `--export` run —
  *not* from the E′ assert at `tests/test_docket_config.sh:460–461`, which pins a **count** (`-eq 28`)
  and names nothing, and *not* from grepping the resolver's `emit` calls, of which there are 29: the
  shell format omits `REPO_ROOT` (`scripts/docket-config.sh:661–663`), so an `emit`-derived set would
  carry a key no `eval "$out"` site can ever define. Deriving the names from the resolver while the
  asserted names come from the test file keeps the two sides of the correspondence genuinely
  independent.
- **Population floor**: assert the discovered site count equals a count produced by a
  **structurally different** extractor (a plain grep against the awk site-walker — two counts from one
  parser is tautological), and that it clears a floor of `>= 60`. Do **not** hardcode a hand-count:
  two independent reviews of this file disagreed (64 vs 65) precisely because the literal also appears
  inside a comment at `:131` and inside the `assert()` helper at `:8`. Derive the exact number at
  build time, from a tokenizer that skips comment lines for the count exactly as it does for prelude
  matching, and pin the derived value.
- **Self-reference without a blind spot**: the guard's own patterns contain the literals it scans for,
  so it would discover itself. Do **not** truncate the corpus at an end-of-file marker — anything a
  future author appends below the guard would then be permanently invisible, and the file's tail is
  exactly where new fixtures land. Instead scan the **whole file** and subtract a single
  start/end-marker-delimited block wrapping the guard's own pattern literals, keying on the **first**
  occurrence of each marker (the literal necessarily appears twice — the marker and the pattern that
  searches for it). Optionally assert that nothing but the suite epilogue follows the guard's end
  marker.
- **Mutation-tested in both directions**: deleting one prelude line reddens it; changing a prelude to
  clear the *wrong* variable also reddens it (the property presence-only guards miss); and the
  correspondence extractor is proven to have seen the whole file via the count assert.

Placement: a new section at the end of `tests/test_docket_config.sh`, so the guard travels with the
file it guards. If correspondence proves genuinely infeasible at build time, the honest fallback is
to ship **no** guard and record why in the results file — never a presence-only one.

## Coupling

- **Change 0125** (rung-pair completeness) proposes a structural guard over *this same file* and is
  still unspecified. The two guards are independent claims (prelude presence vs. rung-pair
  coverage), but they collide in the file and in the enumerated-floor tradeoff. 0126 records
  `related: [125]`; whichever lands second rebases onto the first's section rather than re-litigating
  placement.
- Both stubs descend from change 0112 (`discovered_from: [112]`).
- No `depends_on`. 0126 can land independently of 0125.

## Out of scope

- Restructuring section S's fixtures or extracting shared helpers (0106/0112 deliberately preserved
  the per-fixture shape).
- Converting the existing `unset`-idiom blocks to the poison idiom.
- The rung-pair completeness question (change 0125).

## Assumptions

Every decision below was defaulted autonomously; no human was consulted.

1. **Idiom for the 52 unprotected sites — poison assignment.**
   *Rejected:* converting everything to `unset` + `${VAR-unset}` asserts (uniform but rewrites 52
   asserts, and cannot distinguish "resolved empty" from "unset" at the `-z` sites); rejected doing
   both idioms case-by-case (no rule for the next author).
   *Why:* smallest diff, no assert rewrites under `set -u`, self-describing failure output. Stronger
   than diff size: `tests/test_docket_config.sh:4` sets `set -uo pipefail` and the `assert()` helper
   at `:8` is **not** subshell-wrapped, so an `unset` prelude without a `${VAR-unset}` rewrite would
   abort the whole harness on the first unbound read rather than record one NOT OK.

2. **Existing `unset` blocks stay untouched.**
   *Rejected:* normalizing the whole file to one idiom.
   *Why:* they already satisfy the invariant; churn on correct code risks regressions and buries the
   real diff. Cost accepted: the file carries two idioms, which the guard explicitly accepts.

3. **Clearing is per-fixture (clear the variables the following asserts read), not a blanket line.**
   *Rejected:* a single blanket prelude clearing every exported variable before every eval —
   simpler and strictly safer, but it deletes the fixture-local signal about what each block
   actually asserts and would be a much larger, harder-to-review diff.
   *Why:* the stub's own triage note makes correspondence the point.

4. **Exemptions are derived, never enumerated** — a site is exempt exactly when the asserts between
   it and the next eval read no exported variable.
   *Rejected:* protecting every site unconditionally (models the wrong rule: a prelude before an
   emitted-line assert guards nothing); *rejected:* a comment-marked exemption list — a hand-written
   enumeration that ages into the gap it was written to close.
   *Why:* the guard already extracts the asserted variable names, so the exemption falls out of that
   extraction for free.

5. **The mutation demonstration runs at the natural O→P coincidence (`AUTO_GROOM`), not at `:500`.**
   *Rejected:* the literal reading of the stub ("abort the resolver at `L2`, watch the assert pass"),
   which is not reproducible — the preceding fixture leaves `BOARD_SURFACES=none`, so the assert
   reddens; *rejected:* seeding `BOARD_SURFACES="inline github"` next to the prelude, which is
   degenerate (it proves only that the last assignment wins); *rejected:* reordering fixtures to
   manufacture a coincidence.
   *Why:* blocks O and P already leave and read the same `AUTO_GROOM=false` with nothing in between,
   so the vacuous pass is demonstrable on the unmodified real file. `:500` still gets its prelude —
   it is a latent site, not the demonstration site.

6. **Build the enforcement guard, and make it a correspondence guard.**
   *Rejected:* leaving enforcement to review (the regression that created this change would recur
   with the next appended fixture); *rejected:* a presence-only guard, which is green on precisely
   the failure mode the stub's triage note calls the point — a prelude clearing the wrong variable —
   and would convert a remembered convention into a mechanically-blessed wrong one; *rejected:*
   folding the guard into change 0125 (different claim; 0126 would ship a fix with no floor).
   *Why:* correspondence is reachable — a prototype run of the algorithm over the real file resolves
   every site (3 exempt by derivation, ~12 already compliant, ~49 violations, no false positives on
   the existing `unset`-idiom blocks that assumption 2 leaves untouched). Not scope creep — the stub explicitly puts the
   enforce-vs-remember decision in scope. If correspondence proves infeasible at build time the
   fallback is no guard, recorded in the results file.

7. **Scope is `tests/test_docket_config.sh` only, but the in-file inventory is derived from a shape.**
   *Rejected:* a broader audit of every `eval` in `tests/`; *rejected:* keying discovery on the
   literal `eval "$out` — a spelling, not a shape, which already misses `:1521`
   (`eval "$runtime_odd_out"`, unprotected, assert reads `$DOCKET_BASH_PATH`) and mis-flags `:763`
   (same-line `AUTO_GROOM=""` clearing).
   *Why:* the repo grep finds no sibling file using the idiom, so the file scope holds; the in-file
   count does not. Two independent reviews hand-counted 64 and 65 — the disagreement is itself the
   argument: the literal appears in a comment (`:131`) and in the `assert()` helper (`:8`), so the
   number is **derived at build time by the comment-skipping tokenizer and pinned then**, never
   hand-written into this spec, with a conservative `>= 60` floor underneath it.

8. **Guard lives at the end of `tests/test_docket_config.sh`; the whole file is the corpus, minus a
   marker-delimited block around the guard's own pattern literals.**
   *Rejected:* a new `tests/test_config_suite_hygiene.sh`; rejected `tests/test_comment_anchor_style.sh`;
   *rejected:* co-location with no marker — the guard's own patterns contain the literals it scans
   for, so it would discover itself and read tautologically; *rejected:* truncating the corpus at an
   end-of-file marker, which permanently hides anything appended below the guard — and the file's
   tail is exactly where new fixtures land, so that trades self-reference for silent discovery loss.
   *Why:* subtracting a bounded self-block gets the same self-reference protection with no blind
   spot; the count assert (from a structurally different extractor) catches a mis-bound corpus. If
   0125 lands a second structural guard over the file first, the two sit in adjacent sections.

9. **No ADR.** This is a test-hygiene fix inside an existing, already-decided convention
   (`guards-are-code`), not a new architectural rule.
   *Rejected:* an ADR fixing the poison idiom as a repo-wide convention — premature at one call site.

10. **`related: [125]` is recorded in frontmatter, not only in prose**, per the coupling rule; the
    reciprocal edit on 0125 is left to whoever grooms it, so this run touches only its own stub.
