<!-- results-template.md — close-out artifact for a change. -->
# Auto-grant docket's integration-branch push permission — results
Change: #27 · Branch: feat/claude-settings-publish-permission · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-06-19-claude-settings-publish-permission.md · ADRs: none

## Verify (human)

<!-- The test suite proves the helper's behavior and the gitignore string; it cannot apply the
     grant to this already-migrated repo for you (migrate won't re-run here). -->
- [ ] **Self-grant in the docket repo after merge.** docket itself was migrated back in change #2, so `migrate-to-docket.sh` will not re-run to write the grant here. To get frictionless close-outs in *this* repo, run once from the repo root: `bash scripts/ensure-claude-settings.sh` — it writes `Bash(git -C * push origin HEAD:main)` into `.claude/settings.local.json` (now gitignored repo-locally, so it won't be committed). Re-running is a no-op.
- [ ] **(Optional) Confirm the prompt is gone.** On the next terminal-publish close-out (the `git -C <tmp> push origin HEAD:main` step), confirm Claude Code no longer asks for approval. (This change's *own* merge still prompts unless you ran the helper above first — the grant is not applied by merging the PR.)

## Findings

- **No new ADR** (spec §9 says it is optional, not required). The per-repo-not-user-global scoping is a genuine boundary but is recorded in the change body + spec; nothing non-obvious emerged during the build to warrant promoting it. The env-seam and the gitignore addition were design/reconcile decisions, already documented — not novel implementation decisions.
- **Rule narrowness verified against Claude Code's documented matcher** (final review, opus). `Bash(git -C * push origin HEAD:main)` has no trailing `*`, so it is a whole-command match with a single `*` absorbing only the mktemp worktree path; the literal tail ` push origin HEAD:main` must appear contiguously. Force-push (`push --force origin …`), other-branch, `+HEAD:` force-refspec, and `… && rm -rf /` (compound commands split per-subcommand) all correctly fail to match. The rule mirrors `scripts/terminal-publish.sh:108` (`$GIT -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH"`, `REMOTE=origin` default) verbatim.
- **The #26 dependency is consumed for real, not via a vacuous seam** (LEARNINGS test-seam lesson). Test case 6 runs the actual `scripts/docket-config.sh` against a bare-origin fixture (no `DOCKET_INTEGRATION_BRANCH` env) and asserts the `develop` tail; the final review reproduced the wrong-branch mutation and confirmed case 6 flips to NOT OK. The env override (`DOCKET_INTEGRATION_BRANCH`) keeps the other cases hermetic and doubles as a manual override.
- **Reconcile-discovered gap, closed two ways.** `.claude/settings.local.json` was ignored on the build machine only via the *user-global* excludesfile (`~/.config/git/ignore`), not the repo `.gitignore` — so the change's "never committed onto collaborators" guarantee would not hold on a machine without that global ignore. Closed: (a) `migrate-to-docket.sh` step 5 now gitignores the file for every repo it migrates; (b) docket's own `.gitignore` got the entry directly (commit `c9b5110`), since the already-migrated repo won't re-run migrate. `git check-ignore` confirms the entry ignores only `settings.local.json` and leaves `.claude/settings.json` and `.claude/agents/` committable.
- **Test hygiene:** the one bare-origin fixture (`mkrepo`, case 6) silences the empty-clone warning (`git clone … 2>/dev/null`) per LEARNINGS #26, so the suite's stderr stays pristine. 15/15 asserts pass.

## Follow-ups

- Two **Minor** review notes, both explicitly *no fix* (established style, no behavioral risk): (1) `eval "$(docket-config.sh --export)"` imports all 13 config keys into the helper's scope — consistent with the documented `#26` consumption pattern, no collision with the helper's locals; (2) the migrate gitignore matcher (`pat="^${entry%/}/?$"`) leaves `.` as an unescaped ERE metachar — benign and matching the pre-existing `.docket/`/`.worktrees/` style (only absurd strings false-match, and the consequence is a skipped no-op add).

No remaining follow-ups.
