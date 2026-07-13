# Optional terminal-publish — results

Change: #0064 · Branch: feat/optional-terminal-publish · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-07-12-optional-terminal-publish.md · ADRs: 0027 (produced), 0012 + 0019 (cited)

## Verify (human)

The automated suite covers the mechanism end-to-end (35 test files green, including a hermetic
docket/main fixture that drives the real sweep with the real `terminal-publish.sh`). These are the
checks a human may still want at the merge gate, because they exercise the knob against *this* repo's
live remote rather than a fixture:

- [ ] **This repo's own behavior is unchanged.** docket ships `terminal_publish` commented out in
      `.docket.yml`, so the default (`true`) stands and terminal records keep publishing to `main`.
      Confirm on the next close-out that the archived change file + spec + Accepted ADRs still land on
      `main` exactly as before this change. (Back-compat is the whole safety story: a repo that sets
      nothing must be byte-identical to pre-0064.)
- [ ] **The fence warns where expected.** Put `terminal_publish: false` in a scratch
      `.docket.local.yml` and run any docket skill's Step 0. Expect a loud warn-and-ignore and
      `TERMINAL_PUBLISH=true` still in the export — never a fatal error, never an honored value.
- [ ] **A live suppressed publish.** In a scratch repo with `terminal_publish: false`, close out a
      change and confirm the integration branch receives only code/plan/results via the PR, the
      archived record stays on `docket`, and the sweep logs the suppression as *success* (no
      `sweep-failed`, exit 0, cleanup still runs).

## Findings

**The plan under-enumerated the call sites — the one it missed was the dangerous one.** Task 3 listed
the skill/reference call sites but not `scripts/docket-status.sh:305`, the *executable* invocation in
the headless merge sweep. That is precisely the agent the fence exists to serve, so a miss there would
have left `terminal_publish: false` still publishing to `main` on every sweep — the exact failure the
change exists to prevent. Caught during the build (commit `7e8de35`) and now guarded by a structural
sentinel (`find_ungated_terminal_publish_call_sites`, tests/test_closeout.sh) that fails the build if
*any* call site omits `--enabled`.

**Two false-pass holes in that sentinel (fixed, commit `6aab339`).** The first version grepped
per-line, so a logical line carrying two invocations (a gated `--id` shape and an ungated `--adr`
shape side by side — which `docket-finalize-change/SKILL.md` actually has) was whitewashed by the one
`--enabled` present. It now splits per invocation before filtering. The second: the fence tests
`eval`'d the config export without clearing `TERMINAL_PUBLISH` first — and an aborting run emits
nothing, so `eval ""` would silently leave the *previous* case's value in place and the assertion
would pass on stale state. Both were verified by mutation (strip the feature, watch the test go red).

**Two false factual claims in prose — every grep sentinel green through both (fixed, commit
`8df7d5a`).** This repo *is* docket, so `skills/**` and `scripts/*.md` are source, and the
whole-branch review caught two assertions that no test could:
1. `docket-status.md` / `docket-status/SKILL.md` claimed a `terminal_publish: false` repo "can never
   produce" a post-archive sweep failure. The knob narrows only the `terminal-publish` leg;
   `render-change-links` can still fail there (`sweep-failed <id> render-change-links skipped-publish`)
   and still strands a stale `## Artifacts` block that no later sweep resumes. An agent reading the
   contract would have concluded the state was impossible and done nothing.
2. `docket-adr/SKILL.md` claimed the integration branch "carries no ADR files and no ADR index" under
   the knob — true only for a repo that set it before ever publishing. The knob is explicitly
   non-retroactive (the README says so three sections up), so a mid-life flip keeps everything already
   published.

   This is the LEARNINGS ledger's own lesson landing again, one change later: **a doc sentinel proves
   a sentence exists; it can never prove the sentence is true.** Both sentences were pinned by green
   greps. Only a reviewer re-deriving each claim against the shipped code found them.

**Decisions recorded as ADR-0027** — the coordination-key classification (per-repo-only; the decisive
argument is that the headless sweep can run on any machine, so a machine-scoped value would split the
integration branch by *which agent swept*), and the single script-side guard placed ahead of the
`--id`/`--adr` mode dispatch, where a suppressed publish must exit **0** because all four drivers
treat non-zero as abort.

**Deliberate deviation from the plan (an improvement).** The plan specified skill prose passing
`--enabled "$TERMINAL_PUBLISH"`; the shipped prose passes the `--enabled <terminal_publish>`
placeholder instead, matching the `<integration_branch>` / `<changes_dir>` idiom of the surrounding
code blocks. It is also fail-closed: an unsubstituted placeholder kills the script rather than
publishing. The one executable call site (`docket-status.sh`) passes the shell variable.

**Process note — this change was built across a crash.** A prior implement run completed all 4 plan
tasks and then died before review/PR, leaving the plan's checkboxes unticked and its review-response
fixes uncommitted. The work was adopted, verified against the commits (not the checkboxes), rebased
onto current `main`, and re-reconciled. Worth knowing when reading the branch's 10-commit history.

## Follow-ups

- **`terminal-close-out.md`'s caller-coverage preamble** still referenced changes 0054/0055 as
  pending rewiring; both are `done` and all four drivers now route through the reference. Corrected in
  passing here (the stale sentence directly concerned which callers the new gate covers), but the
  reference has had no broader pass since — worth one.
- **Merge-order coordination.** Change 0044 (PR #69) is `implemented` and touches the same files this
  branch does — `README.md`, `scripts/docket-config.{sh,md}`, `skills/docket-convention/SKILL.md`,
  `tests/test_docket_config.sh`. Whichever merges second needs a rebase; the finalize gate
  (rebase-onto-base + re-test) owns this, but do not merge both blind.
- **No per-artifact granularity** (suppress the change file but still publish ADRs) — deliberately
  out of scope, all-or-nothing. If a repo ever wants ADRs on `main` but change files on `docket`, that
  is a new change, not a knob widening.
