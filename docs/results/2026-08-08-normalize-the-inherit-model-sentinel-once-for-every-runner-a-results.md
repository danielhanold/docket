<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0140 — Normalize the inherit model sentinel once for every runner adapter](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0140-normalize-the-inherit-model-sentinel-once-for-every-runner-a.md)**
<!-- docket:backlink:end -->

# Normalize the `inherit` model sentinel once for every runner adapter — results

Change: #0140 · Branch: feat/normalize-the-inherit-model-sentinel-once-for-every-runner-a · PR: (see change manifest) · Plan: docs/superpowers/plans/2026-08-08-normalize-the-inherit-model-sentinel-once-for-every-runner-a.md · ADRs: none

## Verify (human)

- [ ] **Read the four normalization sites as a set** — `scripts/runner-dispatch.sh`,
      `scripts/runners/codex.sh`, `scripts/runners/cursor.sh`, `scripts/runners/opencode.sh`. All
      four now read `case "$MODEL" in inherit) MODEL="" ;; esac`. The single thing worth your eyes
      is whether the three adapter comments still read as *twins* rather than as three
      co-equal owners — that attribution is the whole point of the change and it is the one claim
      the suite cannot check (see *Residual risk*).
- [ ] **No live-CLI check is needed.** Every adapter is exercised through a mock binary, and no
      real `codex` / `cursor-agent` / `opencode` process runs in the suite. The sentinel never
      reaching a child is asserted against recorded argv, which is the same fact.

## Findings

**The stub's own root cause was already fixed, and the reconcile said so before a line was written.**
The 2026-07-27 stub blamed `emit_shim` for baking `--model inherit` into generated wrappers. Since
changes 0168 and 0205 (ADR-0067) that is false: `sync-agents.sh`'s `runner_config_error` rejects an
empty-or-`inherit` model for any `runner:`-bearing claude agent at generation time. What survived
was strictly the **hand-invocation** path every adapter contract documents. The build therefore
shipped a smaller change than the stub described, and deliberately did not reopen the
generation-time gate.

**`codex.sh` already normalized the sibling sentinel and missed this one.** One line above the
defect sat `case "$EFFORT" in auto) EFFORT="" ;; esac`, whose comment claims it does this "exactly
as runners/cursor.sh and runners/opencode.sh do". The same file, the same shape, the same author's
intent — and only the model sentinel was missed. Worth recording as a shape: when a file handles
two sibling sentinels, the second one is where the gap hides.

**A comment in `sync-agents.sh` was asserting this change's thesis before it was true.**
`runner_config_error` carries the line *"`inherit` is docket's own no-pin sentinel — every adapter
normalizes it to 'no flag', so accepting it would leave a one-word bypass."* That sentence was
**false for `codex.sh`** at the time it was written, and nothing detected it — prose asserting a
cross-site property is exactly the tell that the property was assumed rather than established.
This branch makes the sentence true. It was not edited.

**The facade assert had to route around the adapter to mean anything.** Once `codex.sh` gained its
twin, any assert dispatching `--runner codex --model inherit` would pass on the strength of *either*
layer, and would have stayed green with the facade's line deleted — an outcome assert wearing a
mechanism assert's name. The dispatch-level asserts therefore go through a throwaway probe adapter
via the `RUNNERS_DIR` seam. Both mutation arms confirmed the isolation: deleting the facade line
reddens only the facade group, deleting the codex line reddens only the codex group.

### Review dispositions

Reviewer rung `docket-review-standard` (highest build profile routed was `standard`; 677-line diff,
under the 1500-line bump threshold). Four findings, all `minor`, no blockers.

| # | Finding | State |
|---|---|---|
| 1 | `opencode.md` stated the sentinel rule without naming its owner — the local-ownership drift this change exists to end | **fixed** — `37459bf2` |
| 2 | The facade assert group had no positive control, so both negated asserts would have gone green on an empty argv | **fixed** — `37459bf2` |
| 3 | One rule, four sites, three syntactic shapes — greppable only by the bare word `inherit`, which returns dozens of unrelated hits | **fixed** — `08cfe43a` |
| 4 | `tests/runtime-budgets.tsv` untouched though the branch adds two `make_fixture` cycles; the green record carried no measurement | **fixed** — verified, no change needed (below) |

Finding 4 was resolved by evidence rather than by an edit: the reviewer could not run the suite and
correctly flagged that the build record proved nothing about the budget. Both full-suite runs
measured `test_runner_dispatch` at **8s against its 10s budget**, and no `OVER BUDGET:` line was
emitted anywhere in either run. The budget is left at 10 deliberately — raising a budget that was
never exceeded would spend the headroom the guard exists to protect.

Finding 2 is the one worth a second look at merge time: its mutation arm reproduced the vacuity
live. With `dispatch_probe` neutered to write an empty argv file, the two negated asserts
(`! grep -qxF -- "--model"`, `! grep -qxF -- "inherit"`) **stayed green** while the new control
went red. That is the defect class AGENTS.md's leading-`--` rule warns about, arriving through a
different door — an empty haystack rather than a mis-parsed option.

## Residual risk

**The ownership attribution is not machine-checked.** The three adapter comments and the three
`.md` `--model` bullets now all assert that `runner-dispatch.sh` owns the normalization and that
the adapter line is a defensive twin. Beyond the specific phrases
`tests/test_cursor_contract_docs.sh` pins, nothing verifies that claim — a future change that moved
ownership elsewhere would leave six sites confidently describing an arrangement that no longer
holds, and the suite would stay green. The mitigation shipped here is uniformity of shape (finding
3): all four sites now match one greppable pattern, so the set can at least be *enumerated*
mechanically even though the prose about them cannot be *validated* mechanically.

## Plan deviations

- **Task 3 took a documented TDD exception.** It changes no executable line, so no test could fail
  before and pass after. Verification was a byte-identical before/after diff of the cursor and
  opencode suite output, plus four house guards — recorded here rather than left implicit.
- **`cursor.md` wording**: the plan's replacement block wrote "the child's own default model"; the
  builder kept the file's pre-existing "the child's own default", per the plan's own instruction to
  preserve surrounding bullet text verbatim. Only the two appended sentences are new.
- **Finding 3 changed `runner-dispatch.sh`'s exit-status residue.** The original
  `[ "$MODEL" = "inherit" ] && MODEL=""` left a non-zero `$?` on the non-sentinel path; the `case`
  form leaves zero. Confirmed inert before shipping: the next construct is the `case "$RUNNER"`
  traversal guard, which does not read `$?`; the file's only `$?` consumer is `rc=$?` immediately
  after the adapter invocation, far downstream; and the script sets no `-e` and no ERR trap.

## Follow-ups

None minted. All four review findings concerned this branch's own diff, so none was eligible for
auto-capture — each was fixed or resolved in-branch. The reconcile pass surfaced no adjacent work
clearing the six admission gates.
