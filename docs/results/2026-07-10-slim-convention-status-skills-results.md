# Slim docket-convention + docket-status via progressive disclosure — results
Change: #53 · Branch: feat/slim-convention-status-skills · PR: (see change `pr:`) · Plan: docs/superpowers/plans/2026-07-10-slim-convention-status-skills.md · ADRs: 12 (cited; none minted)

## Verify (human)

- [ ] Skim `skills/docket-convention/SKILL.md` end-to-end once as a consumer — confirm the compressed contract still reads whole (the automated gates checked invariants, not taste).
- [ ] Confirm you accept the size-target deviation (spec §5.4 asked ~200 L / ≤2,500 w and ~110 L / ≤1,600 w; actuals below) — the remainder is test-pinned core contract, and cutting further means re-pointing sentinel tests at scale or semantic loss.
- [ ] Next `docket-implement-next` run after merge doubles as the live smoke of the slimmed `docket-status` under its small pinned model (the pre-merge smoke was read-only: board render byte-identical, board-checks clean, sweep leg unexercised — no merged `implemented` change existed).

## Findings

- **Sizes**: `docket-convention` 380 L/5,982 w → **258 L/4,176 w**; `docket-status` 185 L/2,820 w → **125 L/1,852 w**; new on-demand references: `agent-layer.md` 130 L/1,401 w, `terminal-close-out.md` 107 L/791 w. Hot-path load (convention + status, every implement cycle) drops ~30–35%; the agent-config deep-dive (~1,400 w) now loads only when configuring the agent layer.
- **The spec's word targets were pinned from below by the test suite**: ~14 test files grep the two skill files for dozens of exact phrases and whole sections ("keep intact — test-dense"). The disposition table in the plan enumerates them; that inventory, not prose appetite, set the floor. Deviation judged honest-and-sufficient by the final whole-branch review.
- Behavior-neutrality audit (full-diff, deleted-sentence classification): **NEUTRAL** — every cut is narration, restated inline, in a reference file, or covered by a script contract/ADR (notably `scripts/docket-config.md` + ADR-0019 for the config-layer compression).
- The final review caught one Important issue the diff-scoped audit structurally could not: the new close-out reference **universalized** claims ("all four callers run the SAME sequence", "steps 4–5 best-effort everywhere") that the three unchanged kill paths contradict. Fixed in `10879b9` by scoping the reference to its two live consumers and deferring kill-caller adoption to 0054/0055.

## Follow-ups

- **For #0054/#0055 (already stubbed):** when rewiring the kill callers onto `references/terminal-close-out.md`, decide whether kills adopt the step-2 re-render + skip-publish gate (today: archive → publish → prune, no re-render) and update the reference's blockquote to drop the adoption caveat.
- **For #0054:** the convention's Step-0 preamble says "state-specific create per *Branch model*" but no create procedure exists in the Branch model section (only `migrate-to-docket.sh:192` creates the `.docket/` worktree) — a fresh clone's agent has no instructions and will improvise the `git worktree add`. Add the one-line create recipe (or a pointer) when compressing the remaining skills.
- **For #0054/#0055 (cosmetic):** `docket-status` sweep step labels now run a, b, c–e, h (old g folded into the reference sequence) — relabel to avoid a small-model "missing f–g?" stall.
- Deferred minors from review, no action required: `agent-layer.md` retains two `change 00NN` parentheticals matching no enumerated cut pattern; the wrapper config keys for the three no-skill agents are derivable but no longer listed inline.
