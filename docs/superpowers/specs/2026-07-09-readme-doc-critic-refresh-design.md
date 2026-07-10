# README doc-critic refresh — design

**Date:** 2026-07-09
**Change:** 0052
**Depends on:** 0051 (four-layer config + all-local agent generation) — the build runs only after 0051 merges, because 0051's own scope includes a targeted README rewrite for the four-layer story and this change critiques the *final* text, not a moving target.

## Goal

A technical-documentation-critic pass over `README.md`: review and rewrite it for **accuracy against the post-0051 codebase**, **structure & flow**, and **newcomer clarity**. Concision is explicitly NOT a goal — depth stays (and may grow) wherever clarity needs it. Nothing in the current README is sacred; any section may be restructured, retitled, rewritten, or deleted, judged purely against the three goals.

## The critique (spec foundation — findings against the pre-0051 README @ `origin/main`)

### 1. Accuracy: the agent-generation story is invalidated by 0051

Four places describe the pre-0051 committed-wrapper model that 0051 replaces with gitignored all-local generation, a managed `# docket:generated` `.gitignore` block, a migration step, and new `--check` semantics:

- **Install §, `sync-agents.sh` bullet** — "writes committed project-level wrappers for any repo that opts in."
- **The annotated `.docket.yml` block** — `# agent_harnesses: [claude]  # harnesses the per-repo agent pass generates committed wrappers for`.
- **Global config §** — "precedence per-repo > global > built-in" becomes four layers (repo-local > repo-committed > global > built-in); `.docket.local.yml` appears nowhere in the README.
- **"Tuning an agent's model & effort" §** — the whole section: committed project-level wrappers, the clone-identical-committed-wrapper guarantee, the "seeing files appear in your repo after a global edit is not a misfire" paragraph, and `--check` described as diffing committed wrappers.

Beyond 0051, individual claims need an audit (see Build method): e.g. `finalize.require_pr_approval` appears in the README's `.docket.yml` block but not in docket-convention's schema (one side is stale — resolve at build time by checking the scripts); the "Status" section's Markhaus line ("first planned dogfood project") needs a currency check.

### 2. Structure & flow

- The opening is a single ~70-word sentence packed with undefined vocabulary ("groom stubs to build-ready", "the board").
- "What docket is" opens by assuming the reader already knows superpowers.
- "The reconcile superpower" — the differentiator — sits after Install and two config sections; an evaluating reader may never reach it.
- Global-config detail interrupts the narrative between Install and the conceptual sections.
- The eight-skills reference table is good content in a defensible spot (bottom), but only works if the narrative above it carries newcomers.
- No table of contents for a 300-line README.

### 3. Newcomer gaps

- No prerequisites (Claude Code, `gh`, superpowers-optional).
- Jargon used before definition: *harness*, *board*, *terminal records*, *orphan branch*, *build-ready*.
- **No daily-use walkthrough**: after installing, the README never shows what you actually type — the propose → drain → merge → finalize loop.

## Target outline (the README the build produces)

Narrative order: *what → why → try it → configure → internals → reference*.

1. **`# docket`** — rewritten lead: two-three plain sentences (a markdown-file backlog in your repo + agent skills; you brainstorm changes interactively, an autonomous implementer drains them to PRs), then a 3-4 bullet "what you get". No undefined jargon.
2. **Table of contents.**
3. **How it works** — the producer/implementer loop (current table survives, refined) + a compact change-lifecycle glance (one file ≈ one PR; the seven states in one line or small diagram). Defines *change*, *board*, *build-ready* on first use.
4. **Why docket** — the positioning (superpowers = execution without memory; OpenSpec = too heavy) merged with the **reconcile pitch**, promoted here as the differentiator. Current reconcile content survives largely intact, relocated and tightened.
5. **Install** — new *Prerequisites* subsection; the three primitives rewritten to post-0051 reality (`sync-agents.sh` generates local, gitignored wrappers); pointer to `migrate-to-docket.sh`.
6. **Quickstart: the daily loop** (new) — the concrete session flow with actual skill invocations: propose (`docket-new-change`) → drain (`docket-implement-next`) → human merge gate → close out (`docket-finalize-change`), plus where grooming and the board fit.
7. **Configuration** — one consolidated section: four-layer per-key resolution (built-in < global `config.yml` < committed `.docket.yml` < machine-local `.docket.local.yml`), the annotated `.docket.yml` block kept complete and corrected, `.docket.local.yml` introduced, the coordination-key fence, `agents:`/`skills:` shape pointers to docket-convention (single-source rule kept).
8. **docket-mode internals** — two-branch model, artifact table, `.docket/` worktree, terminal-publish, migration, `main`-mode opt-out. Content survives; tightened, jargon introduced in order.
9. **Tuning agent models & effort** — rewritten to 0051's all-local story: four-layer resolution, gitignored generation, the managed `.gitignore` block, new `--check` semantics (block current + no tracked `docket-*` files + advisory staleness), the one-time migration away from committed wrappers.
10. **The eight skills** — reference table, stays near the end.
11. **Status** — claims verified at build time; cut if stale.

## Build method (runs post-0051 merge; the reconcile pass re-validates first)

1. **Accuracy audit** — extract every testable claim from the *then-current* README (commands, paths, config keys, described behaviors) into a checklist; verify each against the merged codebase, script contracts, and docket-convention. 0051's reconcile-log/results may have shifted details (e.g. exact `--check` semantics) — the audit trusts the merged code over this spec's snapshot.
2. **Rewrite** to the target outline. Where 0051's own targeted README rewrite already landed text (four-layer story), critique and refine it rather than re-deriving it.
3. **Prose pass** — apply the `humanizer` skill's standards to the rewritten text (if the skill is unavailable, apply its published checklist manually).
4. **Verification** — the results doc carries the claims checklist: each claim → where it was verified (file/line or command output). This is the change's test surface; there is no code.

Single PR touching `README.md` only.

## Out of scope

- Any length cap (concision was explicitly deselected as a goal).
- Changes to `docket-convention`, any skill body, script contracts, or docs other than `README.md`.
- `docs/changes/README.md` (the small static blurb) and other per-directory READMEs.
- New features or behavior changes of any kind.

## Decisions log

- **Depend on 0051, don't subsume it** — 0051 keeps its targeted four-layer README rewrite; 0052 critiques the merged result. Avoids double-editing an in-progress change's scope.
- **Critique-driven single change** (over critique-report-then-apply, or spec-light direct rewrite) — the critique is the spec; the build applies it. One PR.
- **Goals: accuracy, structure/flow, newcomer clarity. Non-goal: concision.**
- **Nothing sacred** — full editorial range over every section.

## Reconcile addendum (2026-07-10 — post-0051 merge)

Reconciled by `docket-implement-next` against `origin/main` @ `d7f4a96` (0051 merged 2026-07-10, PR #60; README now 336 lines). The critique above was written against the pre-0051 text; this addendum maps it onto the merged result.

- **Critique §1 (accuracy / agent-generation story): RESOLVED by 0051's own rewrite.** All four cited spots now carry post-0051 text: the Install `sync-agents.sh` bullet (machine-local gitignored generation), the annotated `.docket.yml` block (`agent_harnesses` comment corrected), a four-layer "Global config" section plus a dedicated `.docket.local.yml` subsection, and a fully rewritten "Tuning an agent's model & effort" (all-local story, managed `.gitignore` block, three-part `--check`, migration path, clone-identical guarantee explicitly retired). Per Build method step 2, the build **critiques and refines** these sections in place; it does not re-derive them. The full-text accuracy audit (Build method step 1) still runs over every claim in the README.
- **`finalize.require_pr_approval`: the README is the correct side.** The key is real — implemented by `docket-finalize-change` (change 0021, ADR-0011: consent model, selection matrix, explicit-id override). docket-convention's schema omission is the stale side and is out of scope here (candidate follow-up stub). The audit keeps the README's line and verifies its wording against the finalize skill's semantics (default `false`; gates the auto-detect path only; explicit id overrides).
- **ADR-0020 supersedes ADR-0017.** The agent-artifact story now rests on ADR-0020 (generated agent artifacts are machine-local); audit the tuning section against ADR-0020 + ADR-0015/0016/0019, not ADR-0017.
- **Critique §2 (structure) and §3 (newcomer gaps): verified still fully valid** against the merged text — the ~70-word lead, no TOC, reconcile pitch still after Install + two config sections, no prerequisites subsection, no daily-loop quickstart, jargon-before-definition unchanged. The target outline stands as written, with one note: the merged README's config material (Install's `.docket.yml` block + "Global config" + "`.docket.local.yml`" sections) is current and consolidates naturally into outline item 7 (Configuration).
- **Concurrent 0053 (skill slimming) is in-progress** on the `docket` branch, unmerged, skill bodies only. Audit baseline stays `origin/main`; re-verify the README's by-name pointers to docket-convention sections ("Agent layer", "Skill layer") at audit time in case 0053 merges first.
- **"Status" section currency:** the Markhaus line ("first planned dogfood project") is unverifiable from this repo; per outline item 11, soften or cut at build time unless the human's own materials confirm it.
