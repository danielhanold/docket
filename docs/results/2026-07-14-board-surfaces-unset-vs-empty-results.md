# Encode the disabled board positively — results
Change: #71 · Branch: feat/board-surfaces-unset-vs-empty · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-07-14-board-surfaces-unset-vs-empty.md · ADRs: 32

## Verify (human)

No interactive checks required — the invariants are all asserted by the suite (38 files, green, zero failures). Listed for the merge gate only because the central promise is destructive if it ever regresses:

- [ ] `board_surfaces: []` still leaves a pre-existing `BOARD.md` **byte-identical** — `bash tests/test_board_refresh.sh | grep "truncation trap"` (2 asserts, plus 2 exit-2 twins proving the *rejection* paths don't truncate either).
- [ ] A disabled repo's `docket-status` stdout is unchanged (`board off`).

## Findings

**The polarity reversal is ADR-0032** — *a deliberate off-state must be encoded positively; absence and emptiness are reserved for error.* It reverses change 0059's "an explicit empty value means no surfaces configured." `relates_to: [28, 30, 31]`.

Four things the build surfaced that the spec did not have:

1. **The report channel was not total — the branch violated its own thesis.** The whole-branch review found that after collapsing the callers onto a stdout report line ("key on the line, never the exit code"), two exit-0 paths through `docket-status.sh` emitted **no `board …` line at all**: an unknown/typo'd surface token (deliberately warn-and-ignore) and an inline render failure. A must-land caller seeing no retryable line would conclude "terminal → the board landed" and proceed on a silently stale board — the exact defect class 0071 exists to kill, relocated from the script boundary into the caller contract, and *less* loud than before (previously a direct `board-refresh.sh` invocation surfaced a non-zero exit at the agent's Bash call). Fixed both ways: every surface path now emits a positive line (`board inline failed`, `board <tok> unknown`), and the contract states that **no line at all — or a non-zero exit — is a FAILURE**. This is ADR-0028's "silence is not evidence" applied one level up, and it is recorded in ADR-0032's consequences.

2. **`board inline clean` could mask an unpushed board commit** (a real bug in the orchestrator, pre-existing). After a failed push the board commit already exists *locally* and the working tree is clean — so re-invoking (the must-land remedy) re-rendered, found no diff, and reported the terminal-success line `board inline clean` while the board had never reached the remote. `board inline changed pushed` was unreachable after any push failure. `board_pass_inline` now also checks for an unpushed commit touching `BOARD.md` (`git rev-list --count @{u}..HEAD -- <path>`, degrading to "nothing to push" when there is no upstream) and falls through into the existing push/rebase loop.

3. **Two vacuous asserts shipped and were caught by review — and one of them was mandated by the plan itself.**
   - `! grep -qxF "BOARD_SURFACES="` stayed **green with the fix removed**: bash's `%q` renders an empty value as `BOARD_SURFACES=''`, never as a bare trailing `=`. Fixed by asserting per-format (bare-empty against `--format plain`, quoted-empty against the default shell format).
   - `! grep -qF "$FLAG" "$f"` with `FLAG='--surfaces'` makes grep parse the pattern as an **option**, error with exit 2 — and the leading `!` inverts that error into a false `ok`. The assert could never redden. Fixed with `grep -qF -- "$pattern"`. This one was copied verbatim from the plan's own snippet.
   Both were found only because every new assert was mutation-tested against the real tree.

4. **The must-land retry loop could not terminate for a `github`-only repo.** The first draft of the contract listed `board inline changed pushed` / `board inline clean` / `board off` as the terminal lines — but a legitimate `board_surfaces: [github]` repo prints only `board github ok`, matching none of them, and an agent would re-invoke forever. The rule was **inverted**: `board inline changed push-failed` is the *only* retryable line; every other line is terminal — total by construction, and bounded to 3 attempts.

**On guard discipline.** Every new assert in this branch was mutation-tested against the *real* tree (not a fixture): the empty-value guards, the `none` exclusivity guards (both token orders), the never-empty resolver, the unpushed-commit check, and all clauses of the structural sentinel — including an indented-fence mutation proving the code-unit extractor still sees indented fences. Two review-added mutations (a bare-delegation prose mutation; the retired invocation in *un-backticked* prose) each found a hole the implementer's own battery missed — evidence for the LEARNINGS rule that a fixture battery only samples shapes you already thought of.

## Follow-ups

- **The `$SKILL_*` family has the same surviving-trigger shape.** `SKILL_BRAINSTORM`/`PLAN`/`BUILD`/`REVIEW`/`FINISH` are still model-consumed shell variables, and change 0072's mitigation ("carry printed values forward as literals") is an *instruction an agent must remember to apply*, not an enforcement — exactly the gap 0071 just closed for `--surfaces` by removing the value from the boundary entirely. Explicitly out of scope here; worth a change of its own. Note the same defence is not available (a skill *name* must cross the boundary), so the fix is a guard, not a consolidation.
- **Whitespace-only surfaces** was closed as defence-in-depth (`" "` previously slipped both empty-guards and then iterated zero tokens, printing *nothing at all*). Not reachable from `docket-config.sh` today, but it is the same "no line at all" shape as finding 1 — kept as a guard.
- **The Layer-3 sentinel's corpus was widened** to `skills/*/*.md` + `skills/*/references/*.md` (its own independent `SCOPE3`, leaving Layers 1–2 untouched per ADR-0031). The original corpus missed `skills/docket-convention/github-board-mirror.md` — the one reference doc *about* board surfaces, and the likeliest place for a 9th call site to appear.
