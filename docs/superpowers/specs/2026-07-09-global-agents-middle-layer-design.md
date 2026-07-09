# Machine-local config layer + all-local agent generation — design (change 0051)

**Date:** 2026-07-09
**Change:** 0051 (`global-agents-middle-layer`)
**Status:** Approved (groomed interactively with Daniel, 2026-07-09)
**Depends on:** change 0050 (global config layer, PR #59) — this builds on its resolver and docs.

## Problem

Live testing of change 0050 (2026-07-09) showed the global `agents:` block is dead in any
repo that opts into per-repo wrapper generation: change 0048's always-full-set pass commits
ALL wrappers resolved from `.docket.yml` + built-ins only, so agents without a per-repo
override are pinned to built-in Claude IDs, and those committed files shadow the user-level
wrappers that carry the global models. The 0050 docs promise "per-repo > global > built-in,
field-by-field" — false for `agents:` in opted-in repos, exactly the tested case.

Root tension: model/effort choices are **machine** preferences, but 0048 forced them through
**committed** files, where the ADR-0019 fence (no global value may shape committed bytes)
correctly forbids the global layer from participating. Any committed-file design hits some
variant of this (an earlier fall-through draft of this change still had a file-granularity
shadowing wrinkle and an unverified Cursor user-registry dependency).

## Decision summary

Stop committing generated agent artifacts entirely, and add the machine-scoped per-repo
config file that 0050 deferred:

1. **New layer: `.docket.local.yml`** (repo root, gitignored) — machine-and-repo-scoped
   overrides for the global-able key set.
2. **All-local agent generation** — the per-repo pass writes the full agent set (and the
   Cursor dispatch rule) as gitignored local files, resolved per-field across all four
   layers at generation time.
3. **Managed `.gitignore` block** owned by `sync-agents.sh`.
4. **Migration** prunes 0048-era committed wrappers; `--check` is redefined.

Rejected alternatives (from the stub + brainstorm): committed override-only fall-through
(file-granularity shadowing wrinkle; depends on unverified Cursor user+project registry
merging), a seed command (global edits stop propagating), docs-only (bug persists), and a
hybrid committed+local model (two generation modes to document and test).

## 1. `.docket.local.yml` — the machine-local layer

- **Location:** repo root, sibling of `.docket.yml`. Gitignored via the managed block (§3).
- **Accepted keys:** exactly the **global-able set** from ADR-0019 — `skills:`, `agents:`
  (harness-first block, same shape as `.docket.yml`), `auto_groom`, `finalize.gate` /
  `finalize.test_command`, `board_surfaces` minus the `github` token, `agent_harnesses`.
  No new classification is invented: the local file is machine-scoped, so ADR-0019's rule
  applies verbatim. Fenced coordination keys (`metadata_branch`, `integration_branch`,
  `changes_dir`/`adrs_dir`/`results_dir`, `github_project`, the `github` token) set locally
  are **loudly warned-and-ignored** — same posture as the global file.
- **Precedence (per-field, the `.env` pattern):**
  `repo-local > repo-committed > global > built-in`.
  Global = machine-wide taste; committed `.docket.yml` = team defaults; local = a deliberate
  "on this machine, for this repo" override that beats everything.
- **Reader:** `docket-config.sh --export` grows the one extra rung for global-able keys.
  The local file is read from the **working tree** (it is machine-local by definition —
  the origin/HEAD-authoritative read applies only to the committed `.docket.yml`).
  Skills' Step-0 interface (`SKILL_*`, `AUTO_GROOM`, `FINALIZE_*`, …) is unchanged; no
  consuming skill re-parses YAML. A malformed local file warns and is skipped (the 0050
  malformed-global posture); `.docket.local.yml` misplacement guards are not needed (it is
  already at the only sensible path).

## 2. All-local agent generation

`sync-agents.sh`'s per-repo pass changes target and resolution, not shape:

- **Full set, always** (ADR-0017's by-construction dispatch guarantee is kept): for each
  harness H in the **resolved** `agent_harnesses`, write every built-in agent to
  `<repo>/.<H>/agents/docket-*.md` — but the files are **gitignored, never committed**.
- **Resolution is per-field across all four layers in one pass** at generation time:
  `local.agents.H.X → local.agents.default.X → committed.agents.H.X →
  committed.agents.default.X → global.agents.H.X → global.agents.default.X → built-in`,
  independently for `model` and `effort` (existing ADR-0016 harness-first mechanics, two
  more layers). `effort: auto` semantics unchanged (drops the effort line; omitted key
  inherits the next layer).
- **Cursor dispatch rule:** same trigger as today (`cursor` ∈ resolved `agent_harnesses`),
  written to `<repo>/.cursor/rules/docket-dispatch.mdc`, gitignored. Targets resolve by
  construction because the full set is generated alongside it — the Cursor
  user-level-registry question from the stub is **moot** (no partial project-level set ever
  exists).
- **Opt-in signal:** an `agents:` block or `agent_harnesses:` key in *either* the committed
  `.docket.yml` **or** `.docket.local.yml` — a machine can opt a repo in locally without
  touching committed config. Tracking-only repos (neither key in either file) still
  generate nothing.
- **Consequences of going local:** the ADR-0019 fence is moot for generation (no committed
  bytes exist for a global value to corrupt); the fall-through draft's file-granularity
  shadowing wrinkle disappears (no harness file-precedence is involved — the project-level
  file IS the fully resolved artifact); the **PR #59 stopgap shadowing warning is removed
  outright**.
- **User-level pass unchanged:** `~/.claude/agents` (and every present harness root) still
  gets built-in ⊕ global wrappers — it serves tracking-only repos and non-repo contexts.

## 3. Managed `.gitignore` block

`sync-agents.sh` owns a marker-bounded block in the repo's `.gitignore`:

```
# docket:generated:start (managed by sync-agents.sh — do not hand-edit)
.docket.local.yml
.claude/agents/docket-*.md
.cursor/agents/docket-*.md
.cursor/rules/docket-dispatch.mdc
# docket:generated:end
```

(Exact pattern list is emitted from the same harness table generation uses, so a new
harness extends it without a second roster.) Missing `.gitignore` → created; missing or
stale block → rewritten, with a loud "commit this" notice. Patterns are strictly
docket-scoped; nothing outside the markers is ever touched. The block is written for
opted-in repos (same signal as §2) **and** for any repo where a `.docket.local.yml` is
present — a tracking-only repo using the local file for `skills:`/`finalize:` must not risk
committing it; a repo with neither signal never has its `.gitignore` touched.

## 4. Migration and `--check`

- **Migration (0048-era repos):** on the first run in a repo with *tracked*
  `docket-*` wrapper/rule files, `sync-agents.sh` deletes them from the working tree,
  writes the gitignore block, regenerates everything locally, and prints the single
  migration commit the human needs to make. Idempotent; strictly `docket-*`-scoped
  (ADR-0017's prune scoping, extended to tracked files).
- **`--check` redefined**, three legs:
  - (a) gitignore block present and current — CI-meaningful;
  - (b) **no tracked** `docket-*` wrapper/rule files — migration enforcement, CI-meaningful;
  - (c) locally generated files match the resolved four-layer config — per-machine
    staleness, advisory locally, and vacuous on CI (no local file, no generated files).
  Legs (a)+(b) replace the old committed-drift diff as the CI gate's substance.
- After the docket upgrade, an opted-in repo's CI `--check` goes red on legs (a)/(b) until
  the one migration commit lands — deliberate, with the remedy in the failure output.

## 5. Docs and decision record

- **README:** "Global config" + agent-config sections rewritten to the four-layer story;
  new `.docket.local.yml` documentation with a full commented example; state plainly that
  generated agents are machine-local and never committed.
- **docket-convention:** Configuration section (add the local file + precedence), Agent
  layer (all-local generation, gitignore block, new `--check` meaning), change-0048
  paragraph updated. Sample `.docket.yml` comments updated where they reference committed
  wrappers. (Learnings #49 applies: ship the knob end-to-end — sample file, README,
  convention — in the same change.)
- **Script contracts:** `docket-config.md` (local layer), `sync-agents.md` (generation
  target, gitignore block, migration, `--check`).
- **ADR (build time, via docket-adr):** one new ADR — "generated agent artifacts are
  machine-local, never committed; `.docket.local.yml` completes the four-layer config" —
  **superseding ADR-0017's committed-generation model** (keeping its opt-in gate, full-set
  rationale, and prune scoping) and adding dated `## Update` notes to ADR-0008 and
  ADR-0016. The trade-off is recorded explicitly: the clone-identical-committed-wrapper
  reproducibility guarantee is **consciously retired** — team defaults live in the
  committed `agents:` block by convention, without CI-enforced pinning (solo-first call,
  Daniel, 2026-07-09).

## 6. Tests

- Four-layer per-field resolution (local beats committed beats global beats built-in;
  `model` and `effort` independently; tab-indented YAML per learnings #46).
- Fenced keys in `.docket.local.yml` → warned-and-ignored, never honored, never fatal.
- Opt-in via local file alone; tracking-only repos (neither file has either key) generate
  zero files and never gain a gitignore block (learnings #48 regression posture).
- Gitignore block: creation, idempotent rewrite, hand-edit repair, docket-scoped patterns
  only.
- Migration: 0048-era tracked full set → deleted, block written, local set regenerated;
  idempotent second run.
- `--check`: each leg red/green independently; vacuously green on a clean fresh clone of a
  migrated repo.
- Malformed `.docket.local.yml` → warn + skip, repo still works.

## Out of scope

- A seed command (rejected in favor of the local layer).
- New global-able keys beyond the ADR-0019 set, or any fence reclassification.
- Changes to `skills:` runtime semantics beyond the added resolution rung.
- Board/GitHub-mirror behavior changes.
- User-level pass changes.
