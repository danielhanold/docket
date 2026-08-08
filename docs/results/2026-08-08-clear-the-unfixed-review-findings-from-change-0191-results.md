<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0200 — Board-checks hardening — sanitize LF escape, capture-shape mutation, minor-finding clearance](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0200-clear-the-unfixed-review-findings-from-change-0191.md)**
<!-- docket:backlink:end -->

# Board-checks hardening — sanitize LF escape, capture-shape mutation, minor-finding clearance — results

Change: #0200 · Branch: feat/clear-the-unfixed-review-findings-from-change-0191 · PR: (see change manifest) · Plan: docs/superpowers/plans/2026-08-08-clear-the-unfixed-review-findings-from-change-0191-plan.md · ADRs: none

## Verify (human)

- [ ] Nothing manual. Every claim this branch makes is pinned by an assert in the suite, and the suite is green at HEAD. This section is deliberately empty rather than padded with re-checks of fixed findings.

## Findings

**The plan was wrong three times, and the builders caught it each time.** Recorded because the
pattern matters more than the instances — plan-supplied test code is unverified code, and this
branch is a live demonstration.

1. **Mutation O's `armO_z` count.** The plan asserted 2; the real count is **1**, because the awk
   *replaces* the `done < <(…)` line rather than adding beside it, leaving the inserted capture
   line as the only occurrence. Derived empirically on a hand-built mutant. Not weakened to
   `-ge 1` — mutation F reads 0 through the same grep, so the exact count is what separates the two
   arms.
2. **Mutation 4's walk anchor.** The plan assumed one top-level `for f in ` walk; there are **two**
   (a second one near the end of the file). Pinned as a before/after pair (`2 && 2`) instead of a
   bare literal, so adding a third walk later does not falsify it.
3. **Mutation 4's non-vacuity assert was wrong for its own arm.** The plan wrote `[ -n "$m4out" ]`,
   but the MUT fixture repo is built so scalar-form is the only check that fires — an empty
   `$m4out` is the *correct* result, and the assert failed on first run. Replaced with a re-run
   capturing **stderr** (`2>&1 >/dev/null`) demanding empty stderr plus rc 0 — which is exactly
   what `mrun`'s `2>/dev/null` hides.

**The hoist reproduced the vacuous-green defect live.** Plan Step 3 predicted that after the hoist
and before the mutation-4 rewrite, the file would fail on mutation 4. It did **not** — it stayed at
419/419. That green was the defect itself: applying the old single-range awk to the hoisted script
yields `scalar_form_check` count 0 (landed assert satisfied), a `bash -n` syntax error on an
orphaned `done`, and every "goes GREEN" assert passing against a script that never ran. The arm was
measuring nothing, and only the redesign's new asserts can see it.

**The build gate went red on a cross-task failure no focused test could see.** Mutation O's
original awk embedded a verbatim `ls-tree … --full-tree` string, which
`tests/test_skip_allowlist_invisibility.sh` classified as two unbounded tree walks reaching the
results tree. The repair re-scoped rather than declared or budget-raised: mutation O now **derives**
its capture line from the `done < <(…)` feed already in the script under mutation, via a two-pass
awk. That removed the literal at the source, removed a drift risk between the test's copy and the
real command, and made the arm's capture assert an equality check against the derived command
rather than a grep for a restated prefix.

**Review returned 5 findings, 0 blockers; all 5 were fixed in-branch.** Dispositions are in the PR
body. Two are worth surfacing here:

- The new frozen-artifact rule as first written was **falsified by docket's own publisher** —
  `terminal-publish.sh`'s `restamp_build_artifacts` re-renders the `docket:backlink` block on
  merged `plan:`/`results:` files *after* the merge. The rule now carves out that generated block,
  mirroring how `## Artifacts` is already scoped.
- Mutation O had the **same vacuity hole** its sibling arm had just been fixed for: its GREEN
  assert is an absence over output that is legitimately empty, so it passed whether the defect was
  reproduced or the mutant never ran. Now pinned with an exit-code + no-abort-diagnostic assert.
  Note it deliberately does *not* demand silent stderr — bash prints `ignored null byte in input`
  for the capture, so mutation 4's silence clause would be permanently red here.

**Latent tripwire for future maintainers.** Mutation 4's `scalar_form_check` count assert (3 → 0)
counts lines containing the function name across the *whole* script. Any prose mention of the name
outside the two deleted regions breaks it — this bit the F4 fix once, when the parameter contract
was documented below the `# --- end scalar-form helper ---` marker.

No decision in this build was architectural, so no ADR was produced. The frozen-plans rule was
ruled a docs-lifecycle convention (not an architecture decision) by the spec, and the remaining
choices were test-design calls local to this branch.

## Follow-ups

- **Change 0264** (auto-captured, `docs`): measure the `claude` harness's **forked-mode** gate
  verdict and pin a launch shape that survives it. `references/gate-execution.md` records claude as
  `supported — interactive session, two foreground calls; forked mode unmeasured`, yet forked mode
  is docket's default path. During this build the reference's own recommended detached launch
  (`nohup setsid … >/dev/null 2>&1 </dev/null &` + `disown`, every stream redirected) produced **no
  process and no output file at all**; the gate only ran when relaunched through the harness's
  native background mechanism. Per the reference's own rule that is *inconclusive*, not a verdict —
  which is why it needs a real probe rather than a note.
