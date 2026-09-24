<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0448 — Named implement-next skips unrelated maintenance preflight](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0448-named-implement-next-skips-unrelated-maintenance-preflight.md)**
<!-- docket:backlink:end -->
# Named implement-next skips unrelated maintenance preflight — Results

**Human action:** None. The size-ceiling question raised at review was settled in-branch (see Known issues and follow-ups).

## Outcome

Before this change, running implement-next for one named change (`docket-implement-next 448`) first ran the repository-wide maintenance preflight. That preflight tries closeouts, cleanup, and reclaims for every change. If any other change had a closeout problem, the run halted before it claimed the named change. A broken unrelated change could therefore block every named build.

Now, a run given exactly one change id skips the preflight. It goes straight from repository preparation to that change's own readiness read and claim or resume. This covers first calls, attributed retries, and named resumes. The run still checks everything about the named change itself: readiness, version, dependencies, effective base, claim eligibility, gate context, and resume rules. It never falls back to building a different change.

There is one case the old sweep used to handle for free. The named change may depend on a change whose PR has merged but which has not been closed out yet. For that, the named path now runs the existing `finalize.closeout` operation only on the named change's own unmet dependencies that are `implemented`. If the dependency's PR is not actually merged, the run gives the ordinary "waiting on dependency" refusal. Any other closeout failure halts the run and names that dependency. Closeouts for unrelated changes are never attempted. This was the spec's preferred option in §3, recorded in ADR-0126.

A stacked change whose stack base is unresolved does not trigger a closeout; it keeps its ordinary refusal. The first draft also closed out stack ancestors. Review showed that this can never succeed, because the parent's closeout cannot prove the still-unbuilt child was carried along. So that part was dropped.

Runs with no id, or with a list of two or more ids, keep the preflight exactly as before. Running `maintenance.preflight` or `maintenance.sweep` directly is also unchanged, so unrelated problems still show up there.

This is a workflow-instruction change only; no Go behavior changed. The edits are in `skills/docket-implement-next/SKILL.md` (with the bounded-closeout procedure in `references/edge-paths.md`, behind a blocking pointer) and the convention's *Composition* paragraph, along with their embedded copies. New prose-contract guard rows now fail if the named-path wording is removed or the bounded closeout is widened.

## Verification performed

- Task-level focused gates (`go test ./internal/repoguard/ ./internal/assets/`, plus the app `Preflight|MaintenanceSweep` tests) ran RED before each edit and GREEN after it.
- Mutation checks: two variants turned the new guard red. One restored the unconditional preflight; the other reworded the closeout to be unbounded. Dropping the convention's "skips the preflight" clause also turned it red.
- A standard-rung whole-branch review returned 2 important and 3 minor findings, and all five were fixed in-branch:
  - the unworkable stack-ancestor closeout leg was removed;
  - the prose now names the real `status` field, `unmet_dependencies`;
  - closeout success is keyed on the envelope `result`;
  - a negative guard proves that an id list still runs the preflight;
  - the `docket-status` skill now describes the preflight as selection-path only.
- The whole-suite build gate ran through the Go runner (`build.test_command`) on the certified head. The PR's build-evidence block records the result. Several `BUDGET WATCH` lines appeared for long-running integration test files under parallel load (first overrun streak). None was ### Implement-next skill size ceiling

This affects repository maintainers, not users. The new prose first pushed `skills/docket-implement-next/SKILL.md` past its `TestSkillSizeBudgets` ceiling, and the build raised it to 214 lines / 8270 words, against the file header's rule that ceilings only move down. At the human's direction, the bounded-closeout paragraph then moved into `references/edge-paths.md` (it only applies when a dependency refuses), leaving a one-sentence blocking pointer at its trigger. That lowered the SKILL.md ceiling to 214 / 8025 and raised the `edge-paths.md` ceiling to 118 / 1554 by the relocated text. The prose-contract row for the closeout moved with it and was mutation-checked in its new home. The SKILL.md word count is still above its pre-change 7716, because the named-invocation paragraph itself stays inline.

t paragraph only applies when a dependency refuses.

### Named-path behavior is covered by prose guards only

The named-path behavior lives in skill instructions, not Go code. The tests therefore check the wording, not an end-to-end named run with a failing unrelated closeout. The spec's acceptance tests 1 and 3 are covered this way. A real named run while another change's closeout is failing would confirm the behavior. Change 0449, which is next in the sequence, is a natural place to watch for this.
