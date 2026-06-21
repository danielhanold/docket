# Consuming-repo script resolution (`DOCKET_SCRIPTS_DIR`) — results
Change: #34 · Branch: feat/consuming-repo-script-resolution · PR: (opened at close of build) · Plan: docs/superpowers/plans/2026-06-21-consuming-repo-script-resolution.md · ADRs: 0012 (relates), 0014 (new)

## Verify (human)

The automated tests sandbox `HOME`, so they prove the injector's *logic* but not the *real-machine* write or the end-to-end consuming-repo fix (real-data smoke — LEARNINGS #22/#35). Recommended manual checks at the merge gate:

- [ ] Run `bash ~/dev/docket/install.sh` on this machine. In a **fresh** shell, confirm `echo "$DOCKET_SCRIPTS_DIR"` prints the docket clone's `scripts/` path, and `jq -r '.env.DOCKET_SCRIPTS_DIR' ~/.claude/settings.json` prints the same. (Profile re-sourcing covers the current Claude session's later Bash calls; the `settings.json` `env` is read at session start, so a session restart picks it up there too.)
- [ ] In a **consuming** repo (e.g. markhaus, migrated 2026-06-04 — the repo where change #43 ran in manual-fallback mode), from that repo's CWD confirm the Step-0 resolution now works: `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)" && echo "$DOCKET_MODE"`. This is the real-world fix the change targets.
- [ ] (Negative) In a shell with `DOCKET_SCRIPTS_DIR` unset, confirm a helper call surfaces the `run docket/install.sh` remedy on stderr rather than a bare `no such file or directory`.

## Findings

- **ADR-0014 records the script-resolution contract** (env var, not vendoring → zero drift; shell-profile `export` is the *primary* injection path because the Bash tool re-sources the profile on every call and so reaches dispatched subagents, while OS process-env inheritance was verified unreliable for subagents; `~/.claude/settings.json` `env` is reinforcement; fail-loud `:?`; `DOCKET_` namespacing). Relates to ADR-0012 (script-vs-model boundary — this restores that layer's reachability).
- **Eval-site fail-loud is loud-but-not-fail-stop.** At the most-run call site — Step 0's `eval "$("${DOCKET_SCRIPTS_DIR:?…}"/docket-config.sh --export)"` — an unset var fires `:?` *inside* the command substitution: the remedy prints to stderr, the substitution yields empty, and the outer `eval ""` exits 0 (even under `set -e`), so a strict shell would continue with empty config. In practice the agent *is* the executor and stops on the stderr remedy. The convention phrasing was tightened to say exactly this, and the drift-guard (`tests/test_consuming_repo_scripts.sh`) covers **both** shapes: the bare-command form (rc≠0) and the eval form (remedy-on-stderr).
- **README "two primitives" contradiction** (it now runs three) was caught at the whole-branch review and fixed — `install.sh`'s header comment had been updated but the user-facing README had not.
- **The injector preserves the user file's permissions** across its atomic `mktemp`+`mv` (portable BSD `stat -f` / GNU `stat -c`, default 644), so re-running `install.sh` never silently narrows `~/.zshenv`/`settings.json` to `mktemp`'s 0600.

## Follow-ups

- **#37** (already proposed, `depends_on: [34]`): relocate the per-skill manual-fallback / script-contract prose into on-demand sibling files (progressive disclosure). This change deliberately left that prose in place and only switched the call-site form — the two passes over every skill body don't collide.
- **Settings-`env` reinforcement is Claude-Code-only** (the resolved per-harness open question). The harness-agnostic profile `export` is the actual guarantee; if `.codex`/`.cursor`/… ever need their own settings-`env`, that is a separate, low-stakes add.
- **ADR-0014's publish onto `main` is deferred to finalize.** It is committed on `origin/docket` and listed in this change's `adrs:`, so `terminal-publish` copies the Accepted ADR onto `main` when the PR merges (LEARNINGS #17 pattern). A direct push to `main` now is both premature (ahead of its producing change) and blocked by the auto-mode classifier outside the PR flow.
- **Pre-existing, out of scope:** `tests/test_docket_metadata_branch.sh` (untouched by this change) emits `grep: warning: stray \ before -` — an unescaped grep in that test. Worth a tiny cleanup someday; not a 0034 regression.
