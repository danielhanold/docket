# Design: extract the shared convention into a `docket-convention` skill

**Status:** design (brainstormed 2026-06-10)
**Change:** 0005
**Related:** change 0004 (touched the same `docket-status` health-check section; its board/source drift tripwire is *unaffected* — see §6); `sync-convention.sh` and `tests/test_sync_convention.sh` (retired by this change)

## 1. Context / problem

Every docket skill embeds the shared `## Convention` block between `<!-- docket:convention:begin/end -->` markers, kept byte-identical across the five skills by `sync-convention.sh` (canonical source: `docket-new-change/SKILL.md`). The duplication was a deliberate design decision: each skill is self-contained and never depends on a centralized template being installed.

Measured cost today: the block is **146 lines of each skill's 202–284 lines** — 52–72% of every skill is the same text five times over. Every convention edit is a 5-file change mediated by the sync script, guarded by `test_sync_convention.sh` and sync-check steps in three other tests.

The proposal: a sixth skill, `docket-convention`, holding the convention once; the five operating skills load it by reference instead of embedding it. The install-coupling objection (someone installs five skills but not the sixth) is explicitly accepted as negligible — the skills ship as a set.

## 2. Risk analysis (the decision record — feeds the build-time ADR)

The central worry: how reliable is "invoke `docket-convention` to learn the convention" as an instruction in the middle of a skill?

**Mid-flow skill invocation is a proven mechanism, not a novel bet.** `docket-new-change` invokes `superpowers:brainstorming` mid-flow; `docket-implement-next` chains `docket-status`, `superpowers:writing-plans`, and `subagent-driven-development`. Skill→skill invocation as an explicit checklist step is the most reliable instruction pattern available in this environment.

**The real failure mode is not "can't" but "thinks it doesn't need to".** A model may believe it already knows the docket convention (from training-adjacent text or an earlier session) and skip a soft "review the convention" suggestion. Mitigations, in force together:

1. **Blocking Step-0 phrasing** — the load is the first numbered obligation of every skill, not a "see also".
2. **Undefined-terms forcing function** — the slimmed skills keep *using* convention vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` probes, the bootstrap 2×2) without redefining any of it. A skill body that is not executable without the reference makes skipping the load self-defeating. Design rule: **slimmed skills reference, never restate** — defensive paraphrase would erode the forcing function and must be rejected in review.

**Risks the extraction removes.** Drift becomes structurally impossible: `sync-convention.sh`, `test_sync_convention.sh`, and the sync-check steps in other tests exist only because of the duplication. The 5-file editing friction disappears. Cross-skill version skew (one skill's block updated, another's stale) is eliminated — one source.

**Residual risks, accepted.**
- *Context compaction* in long autonomous runs could summarize away the loaded convention mid-build — but an embedded block has exactly the same exposure; this is a wash, not a regression.
- *Human readability*: a skill's SKILL.md is no longer self-contained reading; the Step-0 stub links the reference.
- *Subagents*: `subagent-driven-development` task subagents never carried the skill context and never needed the convention — they execute code tasks. Invariant, stated here so it survives: **docket bookkeeping stays in the orchestrator context, never in task subagents.**
- *Spurious invocation*: `docket-convention` appears in the skill list and may be invoked by a human question ("how does docket work?") — harmless, arguably a feature.

**Alternatives considered.** (a) *Hybrid* — keep a ~15-line inline "survival kernel" per skill plus the reference: rejected because it keeps the sync machinery alive for a murkier boundary (what is kernel-worthy?) and dilutes the undefined-terms forcing function. (b) *Status quo* — works correctly today, but pays 146×5 lines and 5-file edits forever; rejected given (1)+(2) make the reference pattern sound.

## 3. The new skill — `skills/docket-convention/SKILL.md`

A **pure reference skill**: no procedure, no reads or writes, no git. Body = the current convention block verbatim, with exactly one substantive edit: the sentence claiming the block "is kept byte-identical across the five skills by `sync-convention.sh` (canonical source: `docket-new-change/SKILL.md`)" is replaced by one stating that **this skill is the single source of the convention; the operating skills load it at startup and never restate it**. The `<!-- docket:convention:begin/end -->` markers are dropped — nothing syncs anymore.

Frontmatter `description` serves both audiences: the operating skills' mandatory Step-0 load, and a human asking how docket tracks work. Final wording (settled 2026-06-10):

> Use when any docket skill runs — docket-new-change, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.

*(Build note 2026-06-10: the original settled wording used a colon after "how docket tracks work"; a `: ` inside a plain YAML scalar is invalid, and the repo's skill frontmatter style is unquoted scalars, so the colon became an em-dash at build time. Semantics unchanged.)*

## 4. The five operating skills slim down

Each replaces its 146-line embedded block with a ~6-line blocking Step 0, shaped like:

> ## Convention (load first — blocking)
>
> Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

Existing per-skill prose that points at the convention ("per the convention's Branch model", "the Board pass") stays valid — it now resolves against the loaded skill instead of the block above. The build must sweep each skill for accidental restatements of convention content outside the old markers and remove or repoint them, per the reference-never-restate rule.

## 5. Tooling and tests

- **Retire** `sync-convention.sh` and `tests/test_sync_convention.sh`.
- **Update** the three tests that run sync checks (`test_board_refresh_on_transition.sh`, `test_results_artifact.sh`, `test_docket_metadata_branch.sh`) to drop those steps.
- **Add** `tests/test_convention_extraction.sh` asserting: (a) `skills/docket-convention/SKILL.md` exists and contains the convention's key section headers (Configuration, Directory layout, Change manifest, ADR file, Lifecycle, Build-readiness, Bootstrap guard, Branch model); (b) no operating skill contains a convention copy — grep (fixed-string) for the old markers `<!-- docket:convention:begin -->` / `<!-- docket:convention:end -->` and for the sentinel sentences below; (c) every operating skill contains the Step-0 load line naming `docket-convention`.

  **Anti-copy sentinels** (settled 2026-06-10; one per convention section, each verified absent from every operating skill's non-convention text today, so a hit can only mean convention content crept back in):

  | Convention section | Sentinel (fixed string) |
  |---|---|
  | Configuration | `never gitignored` |
  | Lifecycle diagram | `proposed ──claim──▶` |
  | Lifecycle rules | `satisfied when it reaches` |
  | ADR file | `immutable once Accepted` |
  | Bootstrap guard (probe) | `live planning surface` |
  | Bootstrap guard (2×2) | `half-migrated` |
  | Branch model | `only flow of metadata onto the code line` |
  | Directory layout | `zero-padded to 4 digits` |
  | Change body sections | `PM-altitude proposal` |
  | Lifecycle rules (board) | `must never trail the change files` |

  *(The last three were added at build time after the whole-branch review found the original six left the manifest/body/board-rule sections uncovered; all three verified collision-free the same way.)*

  Rejected candidate: `` never a `git merge docket` `` — collides with `docket-adr`'s own body text. The build must re-run the collision scan if it rewords any skill body, and each sentinel must of course still appear in `docket-convention/SKILL.md` itself (the test asserts both directions: present in the reference, absent in the operating skills).
- `link-skills.sh` globs `skills/*` and picks up the sixth skill automatically; the new test verifies the glob covers it.

## 6. Explicitly untouched

Change 0004's **board/source drift tripwire** in `docket-status` health checks concerns `BOARD.md` vs change-file drift — unrelated to convention sync — and stays as-is. Historical documents (archived changes, old specs/plans/results) that mention `sync-convention.sh` are immutable records and are not edited.

## 7. ADR

The build records one ADR — *reference-loading over embedding for the docket convention* — whose Context/Decision/Consequences distill §2. `change: 5` back-link; the change's `adrs:` field is set when the ADR is minted at build time.

## 8. Acceptance

- All shell tests pass, including the new `test_convention_extraction.sh`.
- Manual behavioral check: in a fresh session, run `docket-status` and verify the transcript shows `docket-convention` invoked before any pass touches the metadata worktree; same for `docket-new-change`'s trivial path.
- Convention edit drill: a sample edit to the convention now touches exactly one file.
