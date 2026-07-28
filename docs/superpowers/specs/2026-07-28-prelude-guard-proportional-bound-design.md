<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0149 — Make the prelude guard's exemption bound proportional, and close the partial-rename gap](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0149-make-the-prelude-guard-s-exemption-bound-proportional-and-cl.md)**
<!-- docket:backlink:end -->

# Make the prelude guard's exemption bound proportional, and close the partial-rename gap — design

Change 0149.

## Problem

Change 0126's correspondence guard in `tests/test_docket_config.sh` defends itself against a
degenerate key set with

```
assert "0126 T: exemptions stay a rounding error (guard not degenerate)" '[ "$t_exempt" -le 5 ]'
```

Measured on the running suite: `TOTALS sites=64 exempt=3 ok=61 viol=0`. Two residual weaknesses
were recorded at 0126's merge gate rather than fixed.

**The bound is absolute, not proportional.** A fixed ceiling of 5 against a real value of 3 leaves
two sites of headroom. As the file grows the headroom does not, so several legitimately-exempt
fixtures landing together trip it. The failure mode when it ages is a loud false red, not a silent
pass — which is why it was parked.

**A suspected partial-rename gap.** The ceiling exists because a *wrong* key set makes every site
"exempt by derivation" (no key can match an empty or renamed `KEY` set) and the guard goes vacuously
green. 0126's review reasoned that a **partial** rename — say 5 of the 28 export keys — would raise
`exempt` only slightly and slip under 5, leaving those sites silently unguarded, and filed that as
"the more interesting half: a silent hole, just a narrow one."

**That half was tested, and the hole is not there.** The evidence is in §2 below. The stub asked
whether it is worth closing; the answer is no, and the reasoning is recorded so it is not
re-derived.

## Decision

One edit, plus a documented negative finding.

### 1. Replace the absolute ceiling with a floor on proven coverage

Swap the ceiling on `exempt` for a **proportional floor on `ok`** — the count of sites the guard
actually proved something about:

```
t_ok="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS .* ok=\([0-9]*\) .*/\1/p')"
assert "0126 T: the guard proved something at >=90% of sites (ok=$t_ok of $t_sites)" \
  '[ $(( t_ok * 10 )) -ge $(( t_sites * 9 )) ]'
```

Today `61 * 10 = 610 >= 64 * 9 = 576` — passes with room for three more non-`ok` sites, and the
room **grows with the file** instead of staying frozen at two. A floor on `ok` is preferred to a
ratio on `exempt` for two reasons: it measures the property that matters (coverage proven) rather
than its complement, and because `viol` must independently be 0, the floor bounds `exempt` without
naming it.

Stated honestly: at today's 64 sites the floor permits 6 non-`ok` sites where the ceiling permitted
5, so this trades one site of immediate slack for slack that **scales**. That trade is the point of
the change; the rejected form below trades away seven.

Note the arithmetic direction: `[ $((t_exempt * 5)) -le "$t_sites" ]` — the form the stub floated —
would permit 12 exempt sites at today's 64, which is **looser** than the absolute 5 it replaces. It
is rejected for that reason.

### 2. The partial-rename gap does not exist as described — do not build for it

The stub asks whether the gap is worth closing "and if so how". It was tested rather than reasoned
about, and the premise does not hold. **A partially-renamed export key is not silent; it is loud.**

Measured against the running suite:

- Renaming **one** emitted key in `scripts/docket-config.sh` (`METADATA_BRANCH` →
  `METADATA_BRANCHZ`) turns **four ordinary asserts red** — and `TOTALS` comes back
  **byte-identical**: `sites=64 exempt=3 ok=61 viol=0`. `exempt` does not rise "slightly"; it does
  not rise **at all**. The reason is worth stating precisely, because it generalises: `$t_keys` is
  derived from the resolver's own output, so the rename *does* swap the new name into the key set
  and drop the old one — the scanner really does lose it. `exempt` holds at 3 because no eval window
  reads that key *exclusively*; any window reading it alongside a still-valid key stays non-exempt.
  A rename therefore has to be near-total before `exempt` moves at all, which is a stronger
  statement of the same conclusion.
- Renaming **five** keys turns 14 asserts red and then kills the run outright:
  `tests/test_docket_config.sh: line 8: LEARNINGS_CAP: unbound variable` under `set -u`. Section (T)
  never executes, so there is no `TOTALS` line to be vacuously green in.

So the failure the ceiling was reaching for — a rename that leaves the guard green while sites go
unchecked — is already caught, twice over, by the ordinary fixtures and by `set -u`. The guard is
not the layer that detects renames, and adding a detector here would restate a property those two
cheaper layers already enforce (`backstop-must-compute-not-reenumerate`).

**The reverse direction was drafted and rejected on evidence.** A pass over `$t_keys` counting keys
no corpus line reads is a real check, but it is a **test-coverage** floor, not a guard-degeneracy
one, and three findings sank it as specified:

- Under the guard's own `\$\{?KEY` read-shape, `t_unread` is **3, not 0** —
  `AUTO_CAPTURE_ENABLED`, `AUTO_CAPTURE_TYPES`, `CHANGE_TYPES` are all heavily tested, but only
  through string-literal assertions (`grep -qxF "AUTO_CAPTURE_ENABLED=false"`,
  `ct_get CHANGE_TYPES …`), never as a dereference. An exact-zero assert would redden the day it
  landed. Broadening to a bare name-occurrence shape does reach 0 — but that abandons the corpus
  discipline the guard is built on, for a check whose fault is already covered.
- It does **not** catch an empty key set: `for (k in KEY)` over an empty array iterates zero times,
  so `unread` is 0 and the assert goes green. The existing `t_keycount >= 20` floor remains the sole
  detector, exactly as 0126 designed it.
- An exact-zero form would couple this guard to **every assert deletion anywhere in the file** —
  which is precisely what changes 0148 and 0151 propose. `DOCKET_BASH_PATH` is dereferenced at
  exactly three lines today, and **0148** — already spec'd — commits to deleting two of them. It
  would survive with a margin of one line, and this spec's own "add the fixture, not the exemption"
  rule would stand in direct conflict with 0148's remedy of deleting meaningless asserts. (0151 is
  unspec'd and explicitly undecided between wiring an eval and deleting.)

Recorded so a future reader does not re-derive it: if a test-coverage floor over export keys is ever
wanted, it is a **separate** change, keyed on bare name occurrence, and it must be reconciled with
whatever assert-deletion work has landed by then.

## Scope — `tests/test_docket_config.sh`, section (T) only

- Extract `t_ok` from the `TOTALS` line alongside the existing `t_sites` / `t_viol` / `t_exempt`.
  Match the existing extractors' shape. **Do not add a field to the `TOTALS` line** — `t_viol`'s
  extractor is end-anchored (`… viol=\([0-9]*\)$`), so appending anything after `viol=` silently
  empties `t_viol` and `[ "" -eq 0 ]` errors the suite red. Nothing here needs a new field.
- Replace the `t_exempt -le 5` assert with the proportional `ok` floor.
- **Keep the `t_exempt` extraction and the existing `TOTALS` print.** `t_exempt` itself is never
  printed — the printed `TOTALS` line carries `exempt=` independently — so after the swap the
  variable becomes diagnostic-only. Do not add a print that does not exist today.
- Replace the retired assert's comment with one recording the measured finding: a renamed export key
  reddens the ordinary fixtures and can abort the run under `set -u`, so this guard is not the
  rename detector and the ceiling should not be reinstated on that reasoning.
- Everything else in section (T) stays byte-untouched.

### Mutation mandate

- Force many sites exempt (neutralise the `need` accumulation in `prelude_report`) → the `ok` floor
  must fire where the old ceiling would have.
- Confirm the floor's slack is what the design claims: `ok` must be allowed to fall to 58 of 64 and
  redden at 57. Assert this by computing, not by re-running the suite 64 times.
- Confirm the retired assert's own mutation still has a detector: with the ceiling gone, an
  all-exempt run must still redden — via the `ok` floor, not via `t_keycount`.
- Re-run the full file and confirm `TOTALS` is unchanged (`sites=64 exempt=3 ok=61 viol=0`) and no
  other assert changes verdict.

## Out of scope

- The guard's clearing-window semantics and its mirror-vs-subset ruling, both settled in 0126.
- The `VAR=""` does-not-count rule.
- The `t_keycount >= 20` vacuity floor, the `t_sites >= 60` population floor, the independent grep
  extractor cross-check, and the self-block bound — all untouched.
- A test-coverage floor over export keys; see the evidence above.
- Anchoring the export key set against a second independent source — change **0123** owns that.

## No ADR

One bound-shape correction inside one test section. The durable finding — that a rename is caught by
the ordinary fixtures and by `set -u`, not by this guard — belongs in the in-file comment beside the
replaced assert, where 0126's other rationales already live, so the ceiling is not reintroduced by
someone re-deriving the original worry.

## Assumptions

1. **Floor on `ok`, not a ratio on `exempt`.** *Chosen:* `t_ok * 10 >= t_sites * 9`. *Rejected:*
   `t_exempt * 5 <= t_sites`, which at today's 64 sites permits 12 exemptions — looser than the
   absolute 5 it would replace, so it would weaken the guard while appearing to modernise it.
   *Rejected:* keeping an absolute ceiling and just raising it — that is the drift the stub filed.

2. **90% as the threshold.** *Chosen:* 90%, against today's 95.3% (61/64). It absorbs three more
   non-`ok` sites now and scales. *Rejected:* 95% — one legitimately-exempt fixture would trip it,
   reproducing today's brittleness. *Rejected:* 75% — enough slack to hide a substantial partial
   degeneracy. The number is a judgment call within a range where nothing breaks; it is recorded
   here so it is re-argued rather than re-guessed.

3. **Do not build a partial-rename detector here.** *Chosen:* answer the stub's "is it worth
   closing?" with **no**, on evidence, and record the measurements so the question is closed rather
   than reopened. *Rejected:* the reverse key-coverage pass — drafted in full, then refuted on three
   counts: the wrong count under the guard's own read-shape (3, not 0), blindness to the empty key
   set, and a standing conflict with changes 0148/0151. *Rejected:* anchoring `$t_keys` against a
   second independent source — it would make this guard depend on another artifact's accuracy and
   duplicates work change **0123** already owns (machine-checking the `docket-config.md` export list
   against the resolver). Two guards, one anchoring each; not one guard anchoring twice.

4. **Shipping the smaller change is the right outcome.** *Chosen:* 0149 lands as the proportional
   bound alone, plus the in-file comment recording why the ceiling is not coming back. *Rejected:*
   building the reverse pass anyway so the change matches the ambition of its title — shipping a
   check whose stated purpose is already served by two cheaper layers is how a suite accretes guards
   that cost maintenance and prove nothing. The title's second clause is answered in the negative,
   not left unaddressed, and the change file's body is rewritten at this groom's exit to state the
   negative finding rather than the refuted premise.

5. **The remaining edit is small enough to stand alone.** *Chosen:* one change — one assert swapped,
   `t_ok` extracted, one comment added. *Rejected:* folding it into a neighbouring change in the
   same file (0148 or 0151) — it is a distinct decision with its own rationale, and the file already
   has three active changes whose edits must stay individually reviewable.

6. **File-collision couplings.** `tests/test_docket_config.sh` is touched by **0148** and **0151**
   (deleting vacuous asserts in the 0132 runtime blocks) and by **0125** (rung-pair fixtures in
   section S, which may add eval *sites*); **0147** merely references whichever proportional form
   lands here — one-way, imposing no ordering.

   Because the surviving bound is a **ratio**, edits elsewhere in the file cannot invalidate it:
   deletions and additions move `t_sites` and `t_ok` together. That is precisely what the absolute
   ceiling could not say, and it is the reason 0125's possible new sites are harmless here. (The
   stronger claim "no deletion can affect it" would be wrong — a deletion that removes an eval site
   does move both counts.) No claim is made about what `TOTALS` reads after 0148 lands; nothing here
   depends on it. `related: [123, 125, 147, 148, 151]`, no `depends_on`. Keep edits additive and
   reconcile at rebase (`concurrent-edits-compose-at-rebase`).
