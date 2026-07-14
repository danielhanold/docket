# Facade skill rewiring — retire the eval preamble — results
Change: #0072 · Branch: feat/facade-skill-rewiring · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-07-14-facade-skill-rewiring.md · ADRs: 30

## Verify (human)

Automated coverage is green (38/38 suite, incl. the new `tests/test_skill_facade_wiring.sh`, mutation-verified). The items below are judgment calls for the merge gate — validate the two design interpretations this build settled non-interactively.

- [ ] **Tokenizer strictness — invocation-guard reading (load-bearing).** The wiring guard flags non-facade *invocations* (discriminator: the `${DOCKET_SCRIPTS_DIR` prefix + the retired shapes `eval "$(` / `fetch origin` / `pull --rebase` in code spans), NOT every `.sh` code-span token. Descriptive NOUN mentions (`` `board-refresh.sh` ``, `` `sync-agents.sh` ``, `` `render-board.sh` ``) are permitted. This was resolved from spec §3's own tiebreaker — it says `references/agent-layer.md` is "rewired only if grep finds old shapes," yet that file is full of descriptive `sync-agents.sh` code spans, so nouns cannot be violations or that file would always need rewiring. Empirically confirmed: `agent-layer.md` and `docket-brainstorm/SKILL.md` pass the sweep with zero edits. Confirm this is the intended strictness (the stricter "no `.sh` token at all" reading would force a heavy rewrite of a reference doc about `sync-agents.sh`, contradicting §Out-of-scope).
- [ ] **docket-implement-next Step 4 feature-branch fetch.** The legitimate `git fetch origin <integration_branch>` (feature-line freshness, NOT a metadata sync, so it must stay direct — not routed through `preflight`) was reworded to "run a direct `git fetch` for `<integration_branch>` from `origin`" to avoid the guard's blunt `fetch origin` string-match. Meaning is preserved and the line is explicitly annotated as feature-line plumbing. Confirm the rewording reads acceptably; if you'd rather keep the verbatim `git fetch origin <integration_branch>` command, narrow the guard's `fetch origin` assert to the metadata-sync shape instead.

## Findings

- **Scope was larger than the 36-invocation headline in the direction of *test* migration, not prose.** The invocation swap is mechanical (op = helper basename), but 21 pre-existing test asserts across 9 test files anchored the OLD `/<helper>.sh` spellings and were narrowed follow-the-call to `docket.sh <op>` (never loosened — e.g. `tests/test_closeout.sh`'s render-change-links→terminal-publish *ordering* assert was re-anchored, ordering check intact). Verified: full suite 38/38 green after migration.
- **The wiring guard caught two legitimate-but-blunt-match sites, both handled without weakening the guard:** the feature-branch fetch (above) and `terminal-close-out.md`'s concurrent-writer note "the loser's `pull --rebase` resolves cleanly" (reworded to "the loser re-runs `docket.sh preflight` and the rebase resolves cleanly").
- **`render-board.sh` / `disable-worktree-hooks.sh` are now internal to `board-refresh` / `preflight`;** their descriptive noun mentions remain in convention prose (permitted, describing mechanics) — the guard does not force their removal, and they are not runtime invocations anywhere.
- No facade/script/`scripts/*.md`/README change (diff touches only `skills/` + `tests/`). No behavior drift — the skill diff is a 50-insert/50-delete 1:1 line balance (invocation swaps + the Step-0/sync rewordings).

## Follow-ups

- **Candidate stub (not minted — grooming/implement never mint ids):** a `bootstrap` facade verb so even the CREATE_ORPHAN path routes through `docket.sh`, retiring the single `docket-config.sh --bootstrap` direct-helper carve-out in the convention Step-0. Deliberately out of scope here (0068 owns facade behavior); ADR-0029 and spec §Decisions record it as a future candidate.
- **Change 0073 (Cursor sandbox & permissions guide)** consumes this change's two-command surface (`docket.sh` + the once-per-repo bootstrap carve-out); it remains `proposed` and can now cite the merged facade wiring.

## Build-process note (transparency for the gate)

The build ran via `superpowers:subagent-driven-development` (per-task implementer subagents on `opus`/`sonnet` + controller review + mutation-verification).

**The final whole-branch review (opus) found and fixed two real issues** — committed as `93833ca`, controller-verified before this results commit:
- **Important (guards-are-code "wrong unit"):** `extract_code_units` toggled the fence state only on **column-0** fences (`/^```/`), so invocation blocks inside **indented** fenced code (numbered-list items in `terminal-close-out.md` and `docket-adr/SKILL.md`) were silently dropped from the sweep — a latent vacuity that would have let a *missed* indented-fence invocation ship falsely green. Fixed to `/^[[:space:]]*```/`. Controller-re-verified: current prose stays green (59/0), and reverting one indented-fence `docket.sh archive-change` back to a raw `archive-change.sh` now correctly reddens (invisible under the old awk).
- **Minor:** a config-resolver pointer in the convention still described `docket-config.sh --export` as a Step-0 *invocation*; reworded to note the resolver is reached in skill runtime through the `docket.sh preflight`/`env` verbs (the `docket-config.sh --export` noun remains as the resolver's own documented interface per ADR-0029).

Controller review independently covered the same five scrutiny areas against the actual files: guard soundness (read + mutation-tested all three families + the reviewer's fence fix), completeness (`git grep`: 30 `docket.sh` invocations + 1 bootstrap carve-out + 0 eval preambles in scope), sentinel-migration integrity (closeout narrowing verified follow-the-call), behavior-drift (50/50 line balance; all removed lines are swaps or reworded-not-dropped), out-of-scope (diff touches only `skills/`+`tests/`). Full suite 38/38 after the fix. The human merge gate remains the final review.
