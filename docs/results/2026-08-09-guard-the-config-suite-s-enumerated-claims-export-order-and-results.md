<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0258 — Guard the config-suite's enumerated claims: export order and rung pairs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-09-0258-guard-the-config-suite-s-enumerated-claims-export-order-and.md)**
<!-- docket:backlink:end -->

# Guard the config-suite's enumerated claims: export order and rung pairs — results
Change: #0258 · Branch: feat/guard-the-config-suite-s-enumerated-claims-export-order-and · PR: <url> · Plan: docs/superpowers/plans/2026-08-08-guard-the-config-suite-s-enumerated-claims-export-order-and.md · ADRs: none

## Verify (human)

- [ ] Nothing manual for the shipped guards — they are self-verifying and the green suite is the
      receipt. See **Follow-ups** for the four review fixes that were authored, verified, and then
      rolled back by the suite gate; deciding whether to re-land any of them is a human call.

## Findings

**The guard caught a real defect on merged code, on its first contact with it.** Change 0271 merged
into `main` while this branch was open. It added `DELEGATION_OBSERVATION_BUDGET` to the resolver's
emission (`scripts/docket-config.sh`) and documented the key in `scripts/docket-config.md`'s
config-key table and exit-code table — but never added it to that file's `### Emit` fence. On rebase,
leg 1's whole-sequence equality assert reddened. The per-key *presence* greps that leg 1 replaces all
stayed green, which is exactly the gap this change was written to close. Commit `a0484a2f` adds the
key to the fence and moves the count prose from 33/34 to 34/35.

That is also a **plan deviation worth naming**: Task 3 Step 4 expected `scripts/docket-config.md` to
be unchanged by this branch. It is changed, by one added fence line and one prose line, for the
reason above.

**A guard-on-guard collision the fix loop surfaced.** The last review-fix commit rewrote leg 2's
marker collection to use `git -C "$REPO" ls-files 'tests/test_docket_config*.sh'`. That is a
*tree-walk site*, and `tests/test_skip_allowlist_invisibility.sh` (a different change's guard) budgets
how many walk sites in `tests/` and `scripts/` can reach the results tree:

```
HAZARD	tests/test_docket_config.sh	3119	rp_family="$(git -C "$REPO" ls-files 'tests/test_docket_config*.sh')"
NOT OK - exactly 2 walk sites reach the results chain at all
  found 3 (excluded 0, filtered 1, declared 1, hazard 1), budget 2.
```

The pathspec is narrow and cannot actually reach `docs/results/`, so this reads as a classifier gap
rather than a genuine hazard — but establishing that is beyond-the-branch work, and the fix loop's
gate is bounded at two suite runs. All four fix commits were reverted per that rule and the branch
returned to the tree that was green (`HEAD^{tree}` byte-identical to `a0484a2f^{tree}`).

**Runtime budget — measured, not estimated.** `tests/test_docket_config.sh` appears in the suite's
advisory `OVER BUDGET:` list. An uncontended single-file A/B run settles the attribution:

| Tree | `tests/test_docket_config.sh` wall |
|---|---|
| `main` @ `324d2268` (before this branch) | **65.2s** |
| this branch @ `a0484a2f` | **66.8s** |

The file was already ~10s past its 55s ceiling on `main`; this change adds **1.7s (2.6%)**. Under the
suite's parallel scheduling the same file reports 136–161s depending on machine contention, which is
the regime the 2.5x slack factor exists for. The plan's own Task 3 Step 3 decision rule therefore
applies: leg 1's `l1` fixture is not the cause, so the measurement is recorded here and the retune is
left to **#0251**, which owns the budget regime for this file. `tests/runtime-budgets.tsv` is
deliberately untouched — raising the number is forbidden by `tests/README.md`.

One reviewer-suggested remedy was checked and is **invalid**: dropping the `l1` fixture's
`git commit`/`git push` does not work, because `scripts/docket-config.sh` reads the committed config
via `git show "origin/HEAD:.docket.yml"`, so the push is load-bearing.

## Follow-ups

- **Four authored-and-reverted review fixes.** Each was mutation-proved by execution before it was
  committed; all four were rolled back together by the gate's revert rule after the fourth breached
  another guard's budget. They remain in history and can be cherry-picked:
  - `2fa1c162` — bind each `RUNG_PAIR` marker to a fixture (attachment + count-equality floors)
  - `9dad467d` — derive the layer set by case-arm shape, not by spelling, with a
    `CONFIG_LINES_<LAYER>` cross-check
  - `7d6e914b` — bind the count-prose check to the `### Emit` section slice, reflow-proof
  - `0982b266` — collect `RUNG_PAIR` markers from git-tracked files only ← **the one that broke the
    gate**; re-landing it needs the walk-site question in `test_skip_allowlist_invisibility.sh`
    settled first.
- **#0251** owns the runtime re-baseline / shard for `tests/test_docket_config.sh`. Nothing new was
  minted for it — a duplicate stub would be noise.
- Both shipped guards are written corpus-indifferent for #0251's split of this file: the rung-pair
  collection iterates the `tests/test_docket_config*.sh` family glob, never a `${BASH_SOURCE[0]}`
  whole-file scan.
