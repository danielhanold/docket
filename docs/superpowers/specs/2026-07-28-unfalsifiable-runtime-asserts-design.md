<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0148 — Two unfalsifiable -z asserts in the config suite sit in eval-free blocks](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0148-two-unfalsifiable-z-asserts-in-the-config-suite-sit-in-eval.md)**
<!-- docket:backlink:end -->

# Two unfalsifiable -z asserts in the config suite sit in eval-free blocks — design

Change 0148.

## Problem

`tests/test_docket_config.sh` carries two asserts of the form

```
assert "0132 runtime invalid $runtime_case: captured value remains clear" '[ -z "$DOCKET_BASH_PATH" ]'
assert "0132 runtime absent: captured value remains clear"                '[ -z "$DOCKET_BASH_PATH" ]'
```

Each is preceded, a few lines above, by a bare `DOCKET_BASH_PATH=""`. **That seed is the whole
defect**: it forces the asserted value to the value the assert demands, so neither assert can ever
fail. They are green for a reason that has nothing to do with the property they name.

Two corrections to the stub's framing, both verified empirically against the running suite:

**The vacuity is not "nothing can change the variable."** The nearest preceding `eval` — the
`require_pr_approval` fixture's — *does* set `DOCKET_BASH_PATH` (to the valid runtime
`ensure_test_runtime` seeds). Remove the `DOCKET_BASH_PATH=""` line and the assert **reddens**. The
seed, not the absence of an eval, is what makes it unfalsifiable.

**Change 0126's guard does see these asserts.** Its need-windows *tile the file* — a site's window
runs to the **next** eval site, not to the end of its own block:

```awk
lo = SL[k]; hi = (k < ns ? SL[k+1] - 1 : n)
```

The `require_pr_approval` site's window extends all the way down to the odd-runtime block, sweeping
in both `[ -z "$DOCKET_BASH_PATH" ]` reads. The file records this in a comment above that site,
which the stub's framing missed:

> `DOCKET_BASH_PATH` is poisoned here not because this `require_pr_approval` fixture cares about the
> bash runtime, but because the guard's windows are asymmetric… **Load-bearing; do not delete.**

Deleting the poison from an unmodified copy produces `SITE … viol DOCKET_BASH_PATH` and a FAIL;
deleting the two asserts first makes the same removal green. So that poison exists **solely** to
satisfy these two vacuous asserts, and it becomes dead code the moment they go — along with a
six-line comment that would then be a lie. That is the same defect class this change exists to
remove, so it is in scope here, not left behind.

What 0126 genuinely could not do was *detect* the vacuity: the guard proves each site clears the
vars its window's asserts read, and a `VAR=""` clear satisfies it. The file already names this
limitation in-file — "`VAR=""` does NOT count… the assert stays exactly as vacuous."

## The property, and who already proves it

Both sites sit on a **fail-closed** path. The resolver is expected to abort and emit **nothing**,
and each site already asserts exactly that, one line above:

```
assert "0132 runtime invalid $runtime_case: resolver aborts"   '[ "$runtime_invalid_rc" != 0 ]'
assert "0132 runtime invalid $runtime_case: export is empty"   '[ -z "$runtime_invalid_out" ]'
```

`export is empty` is the **sole channel**. If the export is empty, then a caller's `eval "$out"` is
`eval ""`, and no exported variable — `DOCKET_BASH_PATH` or any other — can possibly be set. The
per-variable claim is therefore *implied* by the assert directly above it. There is no state in
which `export is empty` passes and a correctly-written `DOCKET_BASH_PATH` assert fails.

## Decision

**Delete both asserts.** Replace each with a one-line comment naming the sole-channel argument:
`export is empty` is the proof, and a per-variable restatement on a totally-empty export is
implied, not additive.

Rejected alternative, and why it matters that it is rejected: **do not** insert an
`eval "$runtime_invalid_out"` to make the asserts falsifiable and pull them into 0126's guard. On
this path `$out` is asserted empty, so the inserted eval is a **provable no-op added solely to
satisfy a guard's site-detection heuristic**. That is guard-gaming
(`guard-remedy-must-not-teach-the-evasion`): it would raise 0126's site count and its "ok" tally
while adding zero real coverage, and it would leave a reader of the file unable to tell a
load-bearing eval from a decorative one.

The two remaining fail-closed asserts at each site (`resolver aborts`, `export is empty`) are
retained unchanged and are the whole of the coverage.

## Scope

### `tests/test_docket_config.sh`

- Delete the `captured value remains clear` assert inside the `for runtime_case in relative missing
  nonexec legacy notbash` loop (5 cases × 1 assert).
- Delete the `0132 runtime absent: captured value remains clear` assert.
- Delete the two now-purposeless `DOCKET_BASH_PATH=""` seed lines that existed only to set up those
  asserts. `grep -nE '^[[:space:]]*[A-Z_][A-Z0-9_]*=""'` over this file returns exactly these two.
  That regex is narrower than the defect class — it would miss `VAR=''` and bare `VAR=` — so
  re-run it in all three spellings before declaring the sweep complete. (None of the other two
  spellings occurs in the file today, so the count is expected to stay at two.)
- **Delete the `DOCKET_BASH_PATH=__poison__` CLAUSE in the `require_pr_approval` fixture, and its
  six-line "Load-bearing; do not delete" comment.** Both exist only to satisfy the asserts being
  removed. **Clause, not line** — the line is compound:
  `DOCKET_BASH_PATH=__poison__; FINALIZE_REQUIRE_PR_APPROVAL=__poison__`, and the second clause is
  load-bearing for this fixture's own assert. Deleting the whole line reddens the guard with
  `SITE … viol FINALIZE_REQUIRE_PR_APPROVAL` (the preceding site's poison is outside the cleared
  window). Keep `FINALIZE_REQUIRE_PR_APPROVAL=__poison__` standing. Leaving a dead poison behind a comment that misstates why it is there reproduces this
  change's own defect one section up. (If a later edit gives that site a real need for the key, the
  comment must be rewritten to say so — not preserved as-is.)
- Add, at each deletion site, a short comment recording the sole-channel reasoning so the asserts
  do not get re-added by someone noticing a "missing" per-variable check.

### `tests/test_docket_config.sh` — 0126 guard invariants

The correct post-condition is **not** a `t_exempt` tripwire. Measured on the running suite,
`t_exempt` is 3 before and 3 after — it does not move, so a tripwire on it would pass while
certifying nothing. Assert the real invariants instead:

- `t_viol` stays 0.
- The `require_pr_approval` site retains a **non-empty need set** after its `DOCKET_BASH_PATH`
  poison is removed — it does (`FINALIZE_REQUIRE_PR_APPROVAL`, an emitted key), so the site does not
  fall into the exempt bucket and `t_exempt` legitimately stays 3.
- The existing floors still hold: `t_sites >= 60`, `t_keycount >= 20`, `t_exempt <= 5`.

### Verification

- Run `tests/test_docket_config.sh` before and after: the assert count drops by exactly 6, no
  assert changes verdict, and the guard's `TOTALS` line stays `sites=64 exempt=3 ok=61 viol=0`.
- Re-run the poison-deletion mutation: with the asserts gone, removing the `require_pr_approval`
  site's `DOCKET_BASH_PATH` poison must be **green** (it is now genuinely unneeded) — which is the
  positive proof that the deletion is complete rather than merely tolerated.
- Mutation-test the survivors: make the resolver emit a non-empty export on the invalid path and
  confirm `export is empty` reddens for every one of the five cases plus the absent case. This is
  the assert now carrying the whole load, so it must be shown to carry it
  (`plan-supplied-test-code-is-unverified`).

## Out of scope

- Change 0126's correspondence guard logic, which is working as designed.
- The guard's `t_exempt` bound shape — change **0149** owns that and edits the same guard block in
  the same file.
- A general "assert reads an exported key in a block with no eval" checker — see assumption 4.
- Change **0151**'s sweep-and-widen framing, of which this change discharges the concrete half; see
  assumption 5.

## No ADR

Deleting two implied asserts and one now-dead poison from one test file. The reasoning is recorded in-file at the deletion
sites, which is where a future reader will look.

## Assumptions

1. **The property is genuinely implied, not merely similar.** *Chosen:* treat `export is empty` as
   a complete proof. The export is the sole channel by which the resolver can set
   `DOCKET_BASH_PATH` in a caller — `docket-config.sh --export` writes shell assignments to stdout
   and nothing else, and the convention forbids sourcing it. An empty stdout therefore admits no
   variable. *Rejected:* the resolver might set the variable some other way — it cannot; a
   subprocess has no channel into the parent's environment.

2. **Delete rather than repair.** *Chosen:* deletion. *Rejected:* rewriting to
   `DOCKET_BASH_PATH=__poison__; eval "$out"; [ "$DOCKET_BASH_PATH" = __poison__ ]` — the file's
   real poison-prelude idiom. It is falsifiable in form, but on a provably-empty export it is still
   implied by `export is empty`, so it buys a guard-site rather than coverage. *Rejected:* keeping
   them with a comment saying "redundant but harmless" — a green assert that cannot fail is exactly
   the thing this change exists to remove; leaving it annotated preserves the misleading signal.

3. **The five loop cases collapse to one deletion.** *Chosen:* delete the single assert inside the
   loop, removing all five instances at once. *Rejected:* preserving one case as a spot-check — the
   argument for deletion is structural and does not vary by case.

4. **No general "eval-free block" checker.** *Chosen:* fix the two sites, build nothing generic.
   *Rejected:* a sibling to 0126's guard that flags any assert reading an exported key inside a
   block with no eval. Against **two** known instances, both of which this change removes, such a
   checker is over-fitting: it would need an allowlist for every legitimately eval-free assert (the
   `rc != 0`, `-z "$out"`, and stderr-diagnostic asserts that surround every fail-closed fixture
   read no exported key but sit in the same blocks), and an allowlist answers "is this expected?"
   rather than "does this exist?". Revisit if a third instance appears — three is a pattern, two is
   a pair.

5. **0151 is a superset of 0148, not a duplicate of it.** Both were minted from change 0126's
   review and both name the same two asserts, but 0151 additionally asks for a **sweep** ("identify
   *every* assert that reads a resolver-exported variable inside a block that performs no resolver
   eval") and for a **guard-widening decision** ("consider whether the 0126 guard can be widened to
   detect the class"). 0148 discharges 0151's concrete half — and assumption 4 answers its
   guard-widening question in the negative, with the sweep's result (`grep` returns exactly two
   instances, both deleted here) as the evidence. So after 0148 lands, nothing of 0151 remains
   *undone*, but that is a conclusion about its residual, not an equivalence.
   **0151 is not folded in or killed autonomously** — that is a backlog-composition call. It is
   abstained with a recommendation. This spec is unaffected by whichever way it goes.

6. **The 0149 interaction is a check, not a dependency.** *Chosen:* no `depends_on`. 0149 changes
   the *shape of the exempt bound*; this change moves `t_exempt` not at all (3 → 3, measured), so
   neither gates the other. *Rejected:* an earlier draft's `t_exempt` tripwire as the safety check —
   it would have passed while certifying a wrong model of the guard, since the asserts being deleted
   *were* inside the guard's view and `t_exempt` still did not move. The real checks are
   `t_viol == 0` and the `require_pr_approval` site keeping a non-empty need set.
   `related: [149, 151]` is to be recorded on the change file at spec-write time, and all three edit
   `tests/test_docket_config.sh` — keep each additive and reconcile at rebase
   (`concurrent-edits-compose-at-rebase`).
