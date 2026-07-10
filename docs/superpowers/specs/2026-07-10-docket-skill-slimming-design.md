# Design: slim docket-convention + docket-status via progressive disclosure

**Date:** 2026-07-10 · **Change:** 0053 · **Depends on:** 0051 (feat/global-agents-middle-layer, PR #60)

## Problem

The docket skill bodies have grown past the point where their size costs more than it buys.
`docket-convention` — loaded as blocking Step 0 by **every** docket skill on **every** run — is
349 lines / ~5,500 words on `main` and grows to **380 lines / ~6,000 words (~8k+ tokens)** once
change 0051 merges. `docket-status` (185 lines / ~2,820 words) runs on every implement cycle
(step-0 dispatch) at a small pinned model. Every token in these two bodies is paid on nearly
every docket operation, competing with the actual work's context.

## Research findings (2026-07-10)

Authoritative guidance, with sources:

- **"Keep SKILL.md body under 500 lines for optimal performance."** Stated three times in
  Anthropic's skill-authoring best practices
  (<https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices>). All docket
  skills pass this — but the agentskills.io spec (<https://agentskills.io/specification>) adds
  **"Instructions (< 5000 tokens recommended)"**, which docket-convention exceeds ~1.7×.
- **"Concise is key. The context window is a public good."** And: **"Default assumption: Claude is
  already very smart. Only add context Claude doesn't already have."** (Anthropic best practices.)
- **Frequently-loaded skills should be far smaller than the 500-line ceiling** — superpowers'
  writing-skills targets **< 200 words** for skills loaded often (<https://github.com/obra/superpowers>).
- **The official recipe for a shared-contract skill** is progressive disclosure: invariants and a
  routing table inline; per-domain detail in **one-level-deep** reference files. "If certain
  contexts are mutually exclusive or rarely used together, keeping the paths separate will reduce
  the token usage" (<https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills>).
- **"Prefer scripts for deterministic operations."** (Anthropic best practices.) docket already
  does this well — 13 helper scripts with co-located `.md` contracts (ADR-0012 boundary). The
  remaining prose is coordination and judgment, which is correct to keep in-model.
- **Named risks of splitting:** links must be loud and explicit or Claude skips them; reference
  chains deeper than one level get partially read (`head -100`); reference files > 100 lines need
  a leading TOC; cuts a frontier model tolerates can degrade a smaller model — "test with all
  models you plan to use." `docket-status`'s wrapper pins a small model, so its imperative steps
  must stay explicit.
- **Degradation mechanism** is attention dilution and instruction-count limits ("frontier thinking
  LLMs can follow ~150-200 instructions with reasonable consistency",
  <https://www.humanlayer.dev/blog/writing-a-good-claude-md>) — trimming redundancy and narration
  raises compliance on what remains.

## Decisions (brainstormed 2026-07-10, human-approved)

1. **Scope:** one change (0053) concretely optimizes `docket-convention` + `docket-status`; the
   spec carries the strategy + a categorization of all eight skills; follow-up stubs are minted
   for the rest (0054, 0055).
2. **Provenance narration is cut, not relocated.** Change-number archaeology and
   rejected-alternative asides ("change 0043's tiers were rejected", "retired by change 0024",
   "the #0035 footgun") are deleted from skill prose. Where a rule genuinely needs a *why*, a bare
   `(ADR-NNNN)` pointer remains. History stays fully recoverable in ADRs, the change archive, and
   git blame.
3. **Structure: core + reference split** (not trim-in-place, not aggressive-minimal-core).
   Behavior-neutral: every rule either stays inline, moves to a one-level-deep reference file
   behind a loud blocking pointer, or already lives in a script contract. **No contract semantics
   change.**

## Design

### 1. docket-convention — core + references

Target: **~380 → ~190 lines, ~6,000 → ~2,400 words.**

**Stays inline** (needed on virtually every run): the `.docket.yml` block + config-layer
resolution summary, the `DOCKET_SCRIPTS_DIR` mechanism, script-contract pointer rule, directory
layout, change manifest, change body sections, ADR format, lifecycle (diagram + table + rules),
build-readiness & selection, autonomous grooming, learnings ledger (compressed), bootstrap verdict
handling, branch model (compressed), and the skill-layer roles table + `auto`/missing-skill rule
(compressed to the table + ~4 bullets).

**Moves to references** (one level deep, loud pointers):

- **`references/agent-layer.md`** — the full Agent-layer deep-dive: harness-first `agents:`
  resolution, `agent_harnesses` scoping (user-level vs per-repo passes), harness-portable model
  IDs, always-full-set generation, the Cursor dispatch rule, `sync-agents.sh` mechanics and
  `--check` legs. SKILL.md keeps ~10 lines: wrappers exist (five skills wrapped, eight wrappers,
  three wrap no skill), the composition/dispatch contract (foreground, git-state-not-in-context
  return, abort-and-report), and the pointer: *"Read `references/agent-layer.md` now (blocking)
  before configuring `agents:`/`agent_harnesses:` or running/debugging `sync-agents.sh`."* No
  runtime skill needs this section; only humans (or agents) configuring the agent layer do —
  the textbook mutually-exclusive context.
- **`references/terminal-close-out.md`** — **new single source** for the shared per-change
  close-out sequence: archive (`archive-change.sh`) → re-render `## Artifacts`
  (`render-change-links.sh`, follow-on commit, push **before** publish) → terminal-publish
  (`terminal-publish.sh`) → cleanup (`cleanup-feature-branch.sh`) → board refresh — plus a
  per-caller **failure-posture table** (finalize: abort-and-report; status sweep:
  log-and-continue with the failed-re-render-skips-publish guard; producer kill and
  implementer reconcile-kill), the archive-first ordering rationale, the UTC-date rule, and the
  `main`-mode degradation. Today this sequence is restated in four skills, each carrying
  "identical — must not diverge" warnings; the duplication *is* the drift risk it warns about.
- **`github-board-mirror.md`** stays as is (already correct progressive disclosure).

**Guardrails:**

- Other skills cite convention sections by name ("per the convention's *Branch model*",
  "*Learnings ledger*", "*Skill layer*", "*GitHub board mirror*"). Every kept section heading
  stays byte-stable; a grep sweep across `skills/` and `agents/` verifies no dangling anchor.
- Any reference file > 100 lines opens with a table of contents.
- References are one level deep from SKILL.md; reference files may point at script contracts
  (`scripts/*.md`) since those are terminal reads.

### 2. docket-status — delegate to executable sources

Target: **~185 → ~100 lines, ~2,820 → ~1,500 words.**

- **Delete the board Structure section + abbreviated rendered example (~55 lines).**
  `render-board.sh` is the executable source of the structure (its contract
  `scripts/render-board.md` documents the output). The skill keeps: the invocation, the commit
  discipline, the regenerate-never-3-way-merge conflict rule, and the readiness-cell semantics
  that health checks share (the dependency-resolution pass section, already compact, stays).
- **Sweep steps c–e collapse** to a pointer into `references/terminal-close-out.md` + ~6 lines of
  sweep-specific posture: per-change log-and-continue, the failed-re-render-skips-publish guard,
  the determinism invariant one-liner.
- Cut the retired-drift-check historical footnote and other narration.
- Compress the Step-0 preamble (see §3).
- **Small-model constraint:** status's wrapper pins a small model; every remaining step stays an
  explicit numbered imperative. Cuts remove duplication and narration — never step explicitness.

### 3. Cross-cutting — the Step-0 preamble becomes convention-owned

The preamble (load convention → `docket-config.sh --export` → act on `BOOTSTRAP` → metadata-tree
ensure/sync + `main`-mode degradation) appears near-verbatim in all seven operating skills
(~15–20 lines each). The convention gains one named section — **"Step-0 preamble"** — as its
single source; each skill's copy compresses to ~3 lines ("Run the convention's *Step-0 preamble*;
this skill's writes land on <X>"). 0053 applies this to `docket-status` (and the convention side);
0054/0055 propagate it to the remaining skills.

### 4. Categorization of the remaining skills

| Skill | Post-0051 size | Optimization potential | Risk of losing intelligence | Verdict |
|---|---|---|---|---|
| docket-finalize-change | 234 L / 3.5k w | **High** — rewire archive/publish prose to `terminal-close-out.md`; compress gate + selection prose | **Medium-high** — the merge gate is docket's highest-blast-radius path; gate flow, sign-off rule, and abort-and-report set must survive verbatim in meaning | Stub **0054** |
| docket-implement-next | 137 L / 2.9k w | **Medium** — the `render-change-links.sh` litany is restated 4×; Step-0/mode boilerplate; reconcile-kill rewires to close-out ref | **Medium** — its repetition is partly deliberate reinforcement for an autonomous agent; dedupe conservatively, keep the branch/metadata discipline section | Stub **0055** |
| docket-new-change | 70 L / 1.4k w | Low-medium — proposed-kill sub-path → close-out ref; Step-0 compression | Low | Fold into 0055 |
| docket-groom-next | 77 L / 1.5k w | Low — Step-0 compression only | Low | Fold into 0055 |
| docket-adr | 88 L / 1.3k w | Low — Step-0 compression only | Low | Fold into 0055 |
| docket-auto-groom | 64 L / 1.2k w | Low — already lean | Low | Fold into 0055 |

### 5. Verification (build-time acceptance)

1. **Anchor grep-gate:** no skill, agent wrapper, or script contract references a convention
   section heading that no longer exists.
2. **Behavior-neutrality review:** a diff-level check that every deleted sentence is (a) narration,
   (b) restated elsewhere inline, (c) moved to a reference file, or (d) covered by a script
   contract. No invariant simply vanishes.
3. **Smoke run:** post-refactor `docket-status` end-to-end (board + sweep + health checks) on this
   repo; `link-skills.sh` / `sync-agents.sh` untouched by construction.
4. **Size targets asserted** in the PR description: convention ≤ ~200 lines / ≤ 2,500 words;
   status ≤ ~110 lines / ≤ 1,600 words; every reference file ≤ ~150 lines with TOC if > 100.

## Out of scope

- Any semantic change to the convention, config resolution, lifecycle, or close-out behavior.
- Editing docket-finalize-change / docket-implement-next / the four small skills (stubs 0054/0055).
- New scripts or changes to existing script contracts.
- Frontmatter `description:` rewrites (triggering is unaffected by this change).
