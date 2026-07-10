# Machine-local config layer (.docket.local.yml) + all-local agent generation — results

Change: #0051 · Branch: feat/global-agents-middle-layer · PR: _(opened by this build)_ · Plan: `docs/superpowers/plans/2026-07-09-global-agents-middle-layer.md` · ADRs: 8, 15, 16, 17, 19, **20 (new)**

Built autonomously via `docket-implement-next` (SDD: 6 plan tasks, per-task spec+quality review, two Important findings fixed pre-review-approval, whole-branch final review). Build-receipt detail (files, test tables) lives in the PR description.

## Verify (human)

- [ ] **Live migration on your real machine:** this repo is tracking-only (no committed wrappers), so the 0048-era migration never fires here. To see it end-to-end, run `bash sync-agents.sh` in any repo of yours that carries committed `.claude/agents/docket-*.md` files — expect: working-tree copies deleted+regenerated, a `# docket:generated` block appended to `.gitignore`, and ONE printed `git rm -r --cached … && git commit` remedy. Run it verbatim, then `sync-agents.sh --check` should be green.
- [ ] **The 0050 bug is actually dead:** in an opted-in repo with a global `~/.config/docket/config.yml` `agents:` value and no repo override, confirm the generated `.claude/agents/docket-*.md` carries the global model (pre-0051 it silently carried the built-in, with a SHADOWED warning).
- [ ] **Post-merge:** re-run `install.sh`/`sync-agents.sh` on this machine so your user-level wrappers regenerate under the new resolver (interface unchanged, but worth the 5 seconds).

## Findings

- **ADR-0020** recorded (on `origin/docket`; rides this change's terminal-publish onto `main` at merge): generated agent artifacts are machine-local, never committed; `.docket.local.yml` completes the four-layer config. Supersedes ADR-0017's committed-generation model (keeping its opt-in gate, full-set rationale, prune scoping); dated `## Update` notes added to ADR-0008 and ADR-0016. The clone-identical-committed-wrapper guarantee is **consciously retired** (solo-first call, Daniel, 2026-07-09).
- **Two Important defects caught by per-task review, both fixed pre-merge:** (1) an unterminated `# docket:generated:start` marker (end line lost to truncation/bad merge) made the awk range logic silently delete user `.gitignore` content after the dangling marker — now detected, loudly warned, and the file left untouched (`b0c1980`); (2) the printed migration remedy chained `git add .gitignore` unconditionally, which fails as-printed in a repo with stale tracked wrappers but no current opt-in (no block written there) — the clause is now conditional on the block actually being maintained (`41d9815`).
- **Spec discrepancy (not "fixed", per learnings #21):** spec §5 names a `sync-agents.md` script contract to update — no such file has ever existed (root-level tools have no `scripts/*.md` contract). The script's header comment, which is its de-facto contract, was rewritten instead. If a real contract file is wanted, that's a human re-scope.
- **Plan assumption wrong, code right:** the plan's smoke expected `--check` rc=1 until `.gitignore` is committed. Actually leg (a) compares on-disk block content, not commit status — rc=0 immediately after a sync; a fresh CI clone still fails until the block is committed (which is the enforcement that matters). Behavior verified correct in a real-clone smoke.
- `--check` also retains a 4th pre-existing CI-meaningful check (legacy bare-agent-key `agents:` shape in the committed `.docket.yml`) alongside the three 0051 legs — docs frame it as a separate note, not a leg; judged non-contradictory by review.

## Follow-ups

- The two remaining 0050 migration-polish items are still open (out of 0051's scope): a directory at `config.yml` + live `agents.yaml` aborts `sync-agents.sh` with a raw unprefixed redirection error; `mv` would clobber a pre-existing `agents.yaml.migrated`.
- After this merges, any repo that committed 0048-era wrappers goes red on `--check` legs (a)/(b) until its one migration commit lands — deliberate, with the remedy printed in the failure output. Known affected: none in this repo (tracking-only); check other adopters.
