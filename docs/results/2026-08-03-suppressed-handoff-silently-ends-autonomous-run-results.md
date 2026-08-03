<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0113 — A suppressed hand-off can silently end an autonomous run — make step completion verifiable, not narrated](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0113-suppressed-handoff-silently-ends-autonomous-run.md)**
<!-- docket:backlink:end -->

# Verifiable step completion — the `aborted-run` check — results

Change: #0113 · Branch: feat/suppressed-handoff-silently-ends-autonomous-run · PR: (see change file) · Plan: docs/superpowers/plans/2026-08-03-verifiable-step-completion.md · ADRs: 24, 44

## Verify (human)

- [ ] **ADR-0044's dated `## Update` note reaches `main`.** It was written on `docket`
      (`5402dc10`) and ships atomically via this change's `adrs: [24, 44]` at terminal publish —
      deliberately *not* pushed standalone, which would race the change's own publish
      (`adr-update-delivery`). Confirm after merge that
      `docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md` on `main` carries
      `## Update — 2026-08-03` and that `status:` is still `Accepted`.
- [ ] **The check fires against real repo history, not only fixtures.** The suite is hermetic — it
      sees temp fixtures and the integration-branch checkout, never the metadata branch
      (`metadata-branch-invisible-to-suite`). After merge, run
      `docket.sh docket-status` on this repo and confirm the `aborted-run` findings it reports are
      the ones you expect for whatever is genuinely `in-progress` at that moment. Note change 0190
      is `in-progress` and may legitimately trip leg B.
- [ ] **The 12h window is the right number for how you actually work.** It is hardcoded by design
      (house precedent: `stale-finalize-blocked`'s 72h, `stale-in-progress`'s 3-day branch idle).
      A genuinely long build will trip it; the finding is free and self-clearing, but if it proves
      noisy in practice that is a tuning decision, not a defect.

## Findings

Reviewed at the **deep** rung (`docket-review-deep`): the build routed no task above `standard`,
which selects `docket-review-standard`, and the whole-branch diff of 1748 changed lines bumped it
one step. **0 blockers.** Two `important` and four `minor` findings were left unfixed by
merge-time judgment and are captured as #0202 and #0203 — full text in the PR body.

The two that bear on the change's own thesis:

- **The prose half under-delivers where the deterministic half succeeds.** The §5 rider now says
  "the step is not complete until its git-state postcondition holds", but `git-state postcondition`
  occurs exactly once across `skills/` — in that new sentence — and nothing defines it. Meanwhile
  **Step 4 got no rider at all**, and Step 4 is where both originating incidents stopped. The
  change file itself argued the check must be "per-step and uniform"; that reasoning was applied to
  the oracle and not to the prose. Captured as **#0203**.
- **The guard proves one of its four anchored-read legs.** All four optional-key reads
  (`plan`/`results`/`branch`/`claimed_at`) correctly use `fm_field`, and mutation D proves the
  anchoring is load-bearing — but only for `plan`. Swapping `fm_field "$f" results` to `field`
  produces the same silent false negative with a green suite. This is 0113's own thesis turned back
  on 0113's own additions (`fix-reintroduces-its-own-defect-class`). Captured as **#0202**.

Two build-time discoveries worth recording:

- **The plan's mutation B would have been a false green.** As written it deleted only the `emit`
  line of leg A's results arm, leaving `if …; then` immediately followed by `fi` — a bash **syntax
  error**. The mutated script would have died before running any check, so every fixture would have
  gone green *for the wrong reason* and the "arm survives" assert would have failed confusingly.
  The worker caught it, replaced the mutation with one that removes the whole `if`/`emit`/`fi` arm,
  and added two `bash -n "$ARMSCRIPT"` guards (mutations B and E) so a future deletion-shaped
  mutation that produces an unparseable script fails loudly instead of masquerading as a
  well-behaved all-green mutation. This is `plan-supplied-test-code-is-unverified` landing exactly
  as that finding predicts — test code a plan hands you is unverified code, not an oracle.
- **`branch_ref` gained a second owner.** It was inlined in `stale-in-progress` and is now shared
  with `aborted-run`. The refactor is semantically identical (empty-branch short-circuit, heads
  then remotes, same order), and fixture 214 — where both legs fire on one change — is the
  multi-owner exercise `shared-resource-keeps-first-owner-assumptions` asks for.

No ADR was authored: the one non-obvious interface choice (`--results-dir` is **repo-relative**
while `--changes-dir` and `--adrs-dir` are **filesystem** paths, because it is addressed through
`<ref>:<path>` and `ls-tree --full-tree`) is documented in `scripts/board-checks.md` rather than
promoted to a decision record.

## Follow-ups

- **#0202** (chore) — clear the five unfixed review findings: the untested caller-side
  `--results-dir` wiring, the three unpinned anchored reads, two prose claims in the test file that
  no assert covers, the C-quoted-pathname false positive in `branch_only_artifact`
  (`ls-tree -z` is the one-flag fix), and the missing measured-actual figure in the budget comment.
- **#0203** (docs) — settle and state the per-step git-state postcondition `docket-implement-next`
  now names but never defines, and give Step 4 the treatment the incident record most demands.
  Note `skills/docket-implement-next/SKILL.md` has only 37 words of budget margin left after this
  change raised it 3950 → 4050.

**Plan deviations:** none material. Three adaptations to real file text were reported by workers
and preserved intent — a missing closing `---` in one fixture heredoc, a Usage bullet rendered as a
table row to match `board-checks.md`'s house shape, and the mutation-B replacement above. The
plan's "add the row to the check-id table at the top of `board-checks.md`" step was a no-op: no
head-of-document check-id list exists there.
