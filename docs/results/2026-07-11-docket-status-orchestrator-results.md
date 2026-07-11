# docket-status orchestrator — results
Change: #58 · Branch: feat/docket-status-orchestrator · PR: <set on open> · Plan: docs/superpowers/plans/2026-07-11-docket-status-orchestrator.md · ADRs: 21 (relates ADR-0012)

## Verify (human)

- [ ] Optional live check: run `"${DOCKET_SCRIPTS_DIR}"/docket-status.sh` (full pass) against a repo with a *merged* `implemented` change and confirm it emits `swept <id> <date>` + `harvest …` and archives it — the automated tests mock `gh`, so this is the only end-to-end exercise of the real batched GraphQL detection against live GitHub. (The `--board-only` fast path was smoke-run live during the build: `board inline clean`, exit 0.)

## Findings

- **ADR-0021 recorded** (Accepted, relates to ADR-0012): a deterministic pipeline script may author formulaic templated commit messages and mutate state along an already-blessed script sequence; judgment-bearing prose stays model-authored. This is what lets `scripts/docket-status.sh` run the whole pipeline in one invocation.
- **Batched detection uses one `gh api graphql` aliased query** (Open Question resolved at plan/build time) — `pN: repository(...) { pullRequest(number) { number mergedAt state } }`, with a `gh pr list --head` fallback for `pr:`-unset changes. A final-review pass caught (and fixed) a jq path that read one level too shallow (`.data.pN.mergedAt` vs `.data.pN.pullRequest.mergedAt`) — the bug had been masked by a test mock using a flattened shape `gh` never emits; the mock was corrected to the real nested shape so the test now genuinely exercises the production path.
- Board pass hardened against the LEARNINGS #57/#51 data-loss class on **both** render paths (initial + rebase-conflict regenerate): render to a tmp file, verify exit-code + non-empty, then `mv`; on a failed regenerate the rebase is aborted and the run reports `push-failed` with `BOARD.md` intact.
- Sweep cleanup-failure semantics: a `cleanup-feature-branch.sh` failure still emits `swept`+`harvest` (the terminal transition is already durable after `terminal-publish`), plus a `sweep-failed <id> cleanup` warning.

## Follow-ups

- **Merge-gate awareness (not a blocker):** the sweep self-heals idempotently only for a failure *before* the archive step. A `sweep-failed` at `render-change-links` or `terminal-publish` (i.e. after `archive-change.sh` has already set the change to `done` in `archive/`) leaves the change **archived but its terminal record unpublished**, and no later sweep resumes it (detection only scans `active/*.md` for `implemented`). Recovery is a manual `terminal-publish.sh --id N`. The skill and `scripts/docket-status.md` wording were corrected to state this accurately.
- Follow-on opportunity (out of scope here): apply the same one-invocation orchestrator pattern to other high-turn-count skills, and per-harness model/effort re-pinning (`agents:`), both already noted in the change's Out-of-scope.
