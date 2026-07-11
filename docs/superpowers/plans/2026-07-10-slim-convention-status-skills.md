# Slim docket-convention + docket-status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Behavior-neutral restructure of `skills/docket-convention/SKILL.md` (380 L / 5,982 w → ≤ ~200 L / ≤ 2,500 w) and `skills/docket-status/SKILL.md` (185 L / 2,820 w → ≤ ~110 L / ≤ 1,600 w) via progressive disclosure into two new one-level-deep reference files, with every doc-sentinel test kept green.

**Architecture:** Two new reference files under `skills/docket-convention/references/` — `terminal-close-out.md` (new single source of the shared archive→re-render→publish→cleanup→board sequence, with a per-caller failure-posture table) and `agent-layer.md` (the Agent-layer configuration deep-dive). The convention keeps byte-stable section headings with compressed inline content behind loud blocking pointers, and gains a new `### Step-0 preamble` section. `docket-status` deletes its board Structure/example prose (the renderer contract is the executable source), rewires sweep steps c–e to the close-out reference, and compresses its Step-0 boilerplate. Test sentinels whose content moves are re-pointed to the reference files; sentinels whose content stays are preserved verbatim in their grammatical location.

**Tech Stack:** Markdown skill files, bash test suite (`tests/test_*.sh`), no scripts or script contracts change.

**Spec:** `docs/superpowers/specs/2026-07-10-docket-skill-slimming-design.md` (on the `docket` branch; read from `.docket/docs/superpowers/specs/` in this clone).

## Global Constraints

- **Behavior-neutral:** no contract semantics change. Every deleted sentence must be (a) narration, (b) restated elsewhere inline, (c) moved to a reference file, or (d) covered by a script contract.
- **Byte-stable headings:** every kept `###` section heading in docket-convention/SKILL.md stays byte-identical (including any `(change NNNN)` suffix — cross-refs and tests anchor on them).
- **References one level deep** from SKILL.md; reference files may point at `scripts/*.md` contracts (terminal reads). Any reference file > 100 lines opens with a TOC.
- **Bare provenance pointers survive where tests anchor on them** (`0016` in the Agent-layer heading, `0017` in the Composition lead, `ADR-0015`, `ADR-0012`); provenance *narration* ("change 0043's tiers were rejected", "retired by change 0024", "the #0035 footgun") is cut.
- **Small-model constraint (docket-status):** every remaining step stays an explicit numbered imperative; cuts remove duplication and narration, never step explicitness.
- **Must-preserve phrases stay in their grammatical location** (never relocated to appease a grep — learnings #36/#37).
- **Do not touch:** the other six skills, any `scripts/*.sh` or `scripts/*.md`, frontmatter `description:` lines, `agents/*.md`, `link-skills.sh`/`sync-agents.sh`.
- Run the full suite as: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAIL: $t"; done` — expected output: nothing.
- Work in `/Users/homer/dev/docket/.worktrees/slim-convention-status-skills` on branch `feat/slim-convention-status-skills`. If `origin/main` advances into these files mid-build, rebase **by intent** (learnings #37): a same-file change that merged after divergence supersedes ours — drop our side, don't reflexively keep it.

## Sentinel disposition table (the load-bearing inventory)

Phrases that MUST remain in `docket-convention/SKILL.md` (tests in parens):

| Phrase (grep -F unless noted) | Guarded by |
|---|---|
| `### Configuration` `### Directory layout` `### Change manifest` `### ADR file` `### Lifecycle` `### Build-readiness` `### Bootstrap guard` `### Branch model` | test_convention_extraction (a) |
| `never gitignored`, `proposed ──claim──▶`, `satisfied when it reaches`, `immutable once Accepted`, `live planning surface`, `half-migrated`, `only flow of metadata onto the code line`, `zero-padded to 4 digits`, `PM-altitude proposal`, `must never trail the change files` | test_convention_extraction (b) |
| `### GitHub board mirror` heading + `github-board-mirror.md` pointer; mirror mechanics stay ABSENT (`closed as **not planned**`, `never touches a label it did not mint`) | test_convention_extraction (f) |
| `Board refresh on status writes` | test_board_refresh_on_transition |
| `^metadata_branch: docket` (E), `integration_branch`, `half-migrated\|bootstrap guard\|migrate-to-docket` (Ei), ``pinning `metadata_branch: main` ``, `^results:` (E) | test_docket_metadata_branch |
| `^results:` (E), `results_dir`, `<results_dir>/`, `plan + results + code` | test_results_artifact |
| `### Learnings ledger`, `LEARNINGS.md`, `~300 lines`, `LEARNINGS.md            # curated` (layout block comment, exact spacing), `build-loop memory`, `compression, not destruction` | test_learnings_ledger |
| `/docket-config.sh`, `${DOCKET_SCRIPTS_DIR:?run docket/install.sh}` (literal), `DOCKET_-namespaced` (i), `Skill layer`, `SKILL_BRAINSTORM`, `SKILL_FINISH`, `degrade to auto` (iF) | test_docket_config |
| `finalize.gate|finalize:` (Ei) + `gate` (i), `local…ci…both…off` (Ei, one sentence), `docket-rebase-resolver`, `docket-integration-repair`, `eight` (i), `five .*skills.* get a wrapper` (i) | test_finalize_gate |
| `0017`, `docket-auto-groom-critic`, `no skill` (i), `only .?docket-convention` (Ei); NO match for `(opus|sonnet|haiku|fable)/(low|medium|high|xhigh|max)` (E) | test_composition_wiring |
| `agents:`, `sync-agents.sh`, `repo-local > repo-committed > global > built-in` (i), `abort-and-report` (i), agent-layer heading regex `^#+ .*(agent layer|model/effort|subagent)` (Ei) | test_sync_agents (kept-on-CONV subset) |
| `agent_harnesses`, `agent_harnesses.*\[claude\]|default.*\[claude\]` (E), `config.yml`, `fence` (i) + `per-repo-only` (i), `.docket.local.yml` | test_sync_agents (satisfied by the inline Configuration section) |
| `## Auto-groom blocked`, `` `## Auto-groom blocked` ``, `^auto_groom: false` (E), `^auto_groomable:[[:space:]]+#` (E), `unset ⇒ inherit`, `**effective auto-groomable**`, `**autonomous-eligible**`, `selection bands` | test_auto_groom |
| `render-change-links.sh` | test_change_links_coverage |

Phrases that MUST remain in `docket-status/SKILL.md`:

| Phrase | Guarded by |
|---|---|
| `## Convention (load first — blocking)` + `docket-convention` | test_convention_extraction (c) |
| `/render-board.sh`, `never 3-way merge` (iF) | test_render_board |
| `/board-checks.sh`, `--changes-dir`, `--metadata-branch`, `--integration-branch`, `blocked_by:` (iF), `mirror reachability` (iF), `do not auto-fix` (iF) | test_board_checks |
| `auto-groom blocked — needs you` | test_auto_groom:104 |
| `terminal-publish` (i) | test_docket_metadata_branch:57 |
| `those files legitimately still live on the unmerged` | test_results_artifact:38 |
| `Harvest learnings` + `docket-finalize-change` | test_learnings_ledger:28 |
| `render-change-links.sh`, `abandon the remainder of this change` (Ei), `deliberately divergent from .?docket-finalize-change` (Ei); NO match for `git mv .*active/` (E) | test_closeout (kept subset) |
| `github-board-mirror.md` | test_convention_extraction (f) |
| NO convention-copy sentinels (list in test_convention_extraction (b)) | test_convention_extraction (b) loop |

Test assertions that get RE-POINTED to a reference file (exact edits in Tasks 2 and 4):
`test_sync_agents.sh` lines 407, 414–417, 421–423, 523–525, 791, 800–801 → `references/agent-layer.md`; `test_closeout.sh` lines 374, 375, 376, 384 → `references/terminal-close-out.md`.

---

### Task 1: Create `references/terminal-close-out.md`

**Files:**
- Create: `skills/docket-convention/references/terminal-close-out.md`

**Interfaces:**
- Produces: the reference file Tasks 3–4 point at (path `references/terminal-close-out.md` relative to `skills/docket-convention/SKILL.md`; from `docket-status/SKILL.md` the path is `../docket-convention/references/terminal-close-out.md`). Headings later tasks cite: `## The sequence (docket-mode)`, `## main-mode degradation`, `## Failure posture — per caller`, `## Determinism invariant`.

This is a pure addition — no existing file changes, so the suite stays green by construction. Content is synthesized from the three current restatements: `docket-finalize-change/SKILL.md` per-change steps 3–5 + Terminal publish section, `docket-status/SKILL.md` sweep steps c–e + g, and `docket-implement-next/SKILL.md`'s reconcile-kill. In 0053 only `docket-status` is rewired to it; finalize/implement-next/new-change follow in 0054/0055.

- [ ] **Step 1: Write the file** with exactly this content:

````markdown
# Terminal close-out — the shared per-change sequence

> Single source for the close-out sequence every terminal transition (`done` or `killed`) runs:
> archive → re-render `## Artifacts` → terminal-publish → cleanup → board. Callers —
> `docket-finalize-change` (per-change close-out), `docket-status`'s merge sweep,
> `docket-implement-next`'s reconcile-kill, `docket-new-change`'s proposed-kill — run the SAME
> sequence; only the failure posture differs (table below). This file owns ordering and posture;
> each script's mechanics live in its co-located contract (`scripts/<name>.md`).

## The sequence (docket-mode)

All metadata writes happen in the metadata working tree (`.docket/`), synced to `origin/docket`
before the first read; every commit pushes immediately.

1. **Archive on `docket` first.** Compute the terminal date in **UTC** — the merge commit's date
   for `done` (`gh`'s `mergedAt`, or `TZ=UTC git show -s --date=format-local:%Y-%m-%d <merge-sha>`),
   the kill commit's date for `killed`. Never `now()`. Author the commit message, pass
   `--results <path>` when a `results:` file arrived via the merge, `--reason "<why>"` on a kill:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/archive-change.sh --changes-dir .docket/<changes_dir> \
     --id <id> --outcome <done|killed> --date <UTC-date> [--results <path>] [--reason "<why>"] --message "<msg>"
   ```

   Trust the exit code: `0` ⇒ archived — an idempotent no-op if already archived, including across
   a day boundary (it reuses the existing dated filename). The script commits **the change file
   only** on `metadata_branch`, so the re-render and the board stay separate commits and
   concurrent archivers converge tree-identically (see *Determinism invariant*).

2. **Re-render the `## Artifacts` block — follow-on commit, pushed BEFORE publish.** Regenerate
   the block on the **archived** file (plan/results re-point to the integration branch at
   terminal state; the renderer is the block's sole writer):

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-change-links.sh \
     --change-file .docket/<changes_dir>/archive/<UTC-date>-<id>-<slug>.md --adrs-dir .docket/<adrs_dir>
   ```

   Commit as a separate follow-on metadata commit on `metadata_branch` and push `origin/docket`.
   **Ordering is load-bearing:** `terminal-publish.sh` copies the change file *from
   `origin/docket`* — publishing before this commit lands would publish the stale block onto the
   integration branch, defeating the re-point on the exact surface it targets. Never bundle this
   into the step-1 archive commit (which must stay change-file-only and byte-identical across
   concurrent archivers).

3. **Publish the terminal record.** Reached only after the step-2 commit is on `origin/docket`:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --id <id> --outcome <done|killed> \
     --integration-branch <integration_branch> --metadata-branch docket \
     --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>"
   ```

   Copies the archived change file + its `spec:` (if set) + the **`Accepted`** ADRs in `adrs:`
   from `origin/docket` onto the integration branch in one dedicated commit — the only flow of
   metadata onto the code line. Trust the exit code; its reuse-existing-file idempotency makes two
   drivers racing on the same change a safe no-op.

4. **Clean up the feature branch + worktree.**

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/cleanup-feature-branch.sh --slug <slug>
   ```

   Trust the exit code. The provenance guard lives in the script: only worktrees resolving under
   `.worktrees/<slug>` are removed — never the `.docket/` metadata worktree or any out-of-tree
   path.

5. **Board refresh.** Regenerate each enabled board surface (the Board pass) and commit + push on
   `metadata_branch` — always a **separate commit** from the archive commits above. `BOARD.md` is
   the live planning view and is never published to the integration branch.

## main-mode degradation

In single-branch/`main`-mode the metadata working tree *is* the integration branch, so the step-1
archive commit is itself the terminal record: `terminal-publish.sh` is a no-op (its own mode-guard
fires), and the step-2 renderer still runs once to re-point the block in place, committed before
cleanup. Steps 4–5 are unchanged.

## Failure posture — per caller

The sequence is shared; the posture on a non-zero exit from steps 1–3 is the caller's:

| Caller | Posture |
|---|---|
| `docket-finalize-change` (single-change close-out) | **abort-and-report** — stop this change's close-out, surface the failure |
| `docket-status` merge sweep (bulk janitor) | **log-and-continue** — abandon the remainder of this change's close-out, move to the next change; the next sweep self-heals idempotently |
| `docket-implement-next` reconcile-kill | trust each exit code; a failure aborts the kill and is surfaced before looping back to selection |
| `docket-new-change` proposed-kill | same as reconcile-kill — surface and stop; nothing else is in flight |

**The skip-publish guard (all callers):** a failed step 1 skips steps 2–3; a **failed step-2
commit/push skips step 3** — a stale `## Artifacts` block must never be published. Steps 4–5 are
best-effort everywhere (log and continue; the board self-heals on the next pass).

## Determinism invariant

Two agents both driving the same terminal transition produce a byte-identical step-1 commit
(change-file-only, UTC terminal date, no `now()`); the loser's `pull --rebase` resolves cleanly.
Everything else (re-render, board) is regenerated deterministically from the change files — on a
rebase conflict in generated content, **regenerate, never 3-way merge**.
````

- [ ] **Step 2: Verify the file parses as expected**

Run: `wc -l skills/docket-convention/references/terminal-close-out.md`
Expected: ~115 lines (± 10). It is > 100 lines → confirm the blockquote preamble at top serves as orientation and ADD a TOC line under the H1 if over 100: check with `awk 'NR<=12' skills/docket-convention/references/terminal-close-out.md` — if `wc -l` > 100, insert after the blockquote: `Contents: [The sequence](#the-sequence-docket-mode) · [main-mode degradation](#main-mode-degradation) · [Failure posture](#failure-posture--per-caller) · [Determinism invariant](#determinism-invariant)`

- [ ] **Step 3: Run the full suite (must be untouched-green)**

Run: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAIL: $t"; done`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add skills/docket-convention/references/terminal-close-out.md
git commit -m "docs(0053): add terminal-close-out reference — single source of the shared close-out sequence"
```

---

### Task 2: Create `references/agent-layer.md`; collapse the convention's Agent layer to a stub; re-point moved test sentinels

**Files:**
- Create: `skills/docket-convention/references/agent-layer.md`
- Modify: `skills/docket-convention/SKILL.md:64-184` (the `### Agent layer` section)
- Modify: `tests/test_sync_agents.sh:403-423,520-525,787-801`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: `references/agent-layer.md`; the stub's pointer sentence Task 3 leaves untouched; test variable `AGL="$REPO/skills/docket-convention/references/agent-layer.md"` used by the re-pointed assertions.

- [ ] **Step 1: Create `skills/docket-convention/references/agent-layer.md`**

Content = MOVE of SKILL.md's current Agent-layer deep-dive (lines 70–183: the layered-config paragraph + table, the harness-first prose paragraph, the `agents:` YAML example block, the orthogonality paragraph, the folding/precedence paragraph, the retired-guarantee paragraph, the `sync-agents.sh` on-demand + `--check` paragraph, the harness-portable-IDs paragraph, and the always-full-set + Cursor-dispatch-rule paragraph), verbatim EXCEPT the enumerated provenance-narration cuts below. Open the file with:

```markdown
# Agent layer — configuring model/effort-pinned subagents

> On-demand detail for the convention's *Agent layer*. Read this before configuring
> `agents:` / `agent_harnesses:` in any config layer, or running/debugging `sync-agents.sh`.
> The runtime contract (which skills get wrappers, dispatch semantics, abort-and-report)
> stays in `SKILL.md`'s *Agent layer* stub; this file is the full configuration mechanics.

Contents: [Layered config](#layered-config) · [Harness-first agents: blocks](#harness-first-agents-blocks) · [Generation scope: agent_harnesses](#generation-scope-agent_harnesses) · [Harness-portable model IDs](#harness-portable-model-ids) · [Always-full-set generation + the Cursor dispatch rule](#always-full-set-generation--the-cursor-dispatch-rule) · [sync-agents.sh runs + the --check gate](#sync-agentssh-runs--the---check-gate)
```

then the moved content organized under those `##` headings: `## Layered config` (table + intro paragraph), `## Harness-first agents: blocks` (harness-first prose + YAML example + orthogonality + folding paragraphs), `## Generation scope: agent_harnesses` (the scoping prose from the layered-config and harness-portable paragraphs), `## Harness-portable model IDs` (ADR-0015 paragraph), `## Always-full-set generation + the Cursor dispatch rule` (0048/0051 paragraph + retired-guarantee paragraph), `## sync-agents.sh runs + the --check gate` (on-demand + three-leg `--check` paragraph).

Provenance-narration cuts to apply while moving (everything else moves verbatim):
- `(change 0046)`, `(change 0050)`, `(change 0051)`, `(change 0045, ADR-0015)` → keep `(ADR-0015)`, drop the change number; `(change 0048, extended 0051)` → drop; `; change 0051)` inside parens → drop the clause; `(change 0051 added the repo-local rung)` → drop.
- `— no tier layer (change 0043's tiers were rejected)` → `— no tier layer`.
- `(change 0046 reshaped only how each file's values resolve, per the harness-first Agent layer above) — change 0050 later made a global \`agent_harnesses\` override this presence detection for the user-level pass only` → `— unless the global \`config.yml\` sets \`agent_harnesses:\`, which governs the user-level target list only` (the rule, minus the archaeology; the fuller statement already exists in the moved layered-config paragraph — do not duplicate it, keep whichever single statement reads complete).
- `**The clone-identical-committed-wrapper guarantee is retired.** Before change 0051, …` paragraph → compress to: `Generated files are machine-local: per-repo wrappers were committed before the all-local model, so identical-on-every-clone pinning is retired — a deliberate trade-off; team defaults still live in the committed \`.docket.yml\` \`agents:\` block by convention, without CI-enforced pinning of generated copies.`
- Keep intact (tests grep them here after re-pointing): `default:` + `cursor:` YAML example keys, `full built-in agent set` phrasing (`full (built-in )?(agent )?set` regex), `override-only`, `docket-dispatch.mdc`, `harness-neutral`, `passthrough`, `agent_harnesses` within 500 chars of `ADR-0015`, the `| Global |`…`config.yml` table row, `gitignored, never committed`, `advisory`, `# docket:generated`, `effort: auto` vs omitted comment lines, `repo-local > repo-committed > global > built-in`.

- [ ] **Step 2: Replace SKILL.md's Agent-layer section body with the stub**

In `skills/docket-convention/SKILL.md`, keep the heading line `### Agent layer — model/effort-pinned subagents (change 0016)` byte-identical, and replace everything from the line after it down to (not including) the `### Skill layer — pluggable workflow skills (change 0049)` heading with exactly:

```markdown
Each **autonomous** docket skill can run as a model/effort-pinned **subagent** instead of inline at the session model. Five skills get a wrapper — `docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two **interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline and only surface an **advisory** recommended model/effort at startup (a skill cannot force the session model). `docket-convention` is not an agent — it is injected into every wrapper via `skills:`.

A wrapper is a thin generated file: it pins `model` + `effort` and injects the skill; the skill body stays the single source of behavior. Because a subagent cannot pause to ask a human, every autonomous wrapper carries an **abort-and-report** rule: an unmet precondition or blocking ambiguity is surfaced and stopped on — never turned into an interactive prompt. Wrappers are generated by `sync-agents.sh` from the layered config (precedence: repo-local > repo-committed > global > built-in); an agent with no entry in any layer defaults to `model: inherit` with no `effort`.

**Composition (change 0017).** `docket-implement-next` dispatches the `docket-status` subagent (step 0) and the `docket-adr` subagent (step 6); `docket-auto-groom` dispatches the `docket-auto-groom-critic` subagent for its adversarial gate. These dispatches are **foreground** (the parent suspends until the child returns) and **unconditional**; their contract is **git state** on `origin/docket` (for adr, plus a published ADR on the integration branch), re-read after a re-sync — never an in-context return. `docket-finalize-change` dispatches the `docket-rebase-resolver` and `docket-integration-repair` subagents at its merge gate — also foreground, but their reports flow **back to finalize in-context** to gate the merge, and they act in the feature worktree, not on `origin/docket`. Each dispatched agent runs at the model/effort its own wrapper resolves — literal tiers are never restated in dispatch prose, so an override can never drift from the documentation. Three of the **eight** generated wrappers wrap **no skill** — `docket-auto-groom-critic`, `docket-rebase-resolver`, `docket-integration-repair`; each loads only `docket-convention`, so it inherits no caller bias, and all are auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob. (Five *skills* get a wrapper; these three are wrappers that wrap no skill — eight wrappers, five skills.)

**Configuring the layer** — the harness-first `agents:` blocks (`default:` + per-harness keys), `agent_harnesses` scoping, harness-portable model IDs (ADR-0015), always-full-set generation, the Cursor dispatch rule, `effort: auto` vs omitted, and `sync-agents.sh` / `--check` mechanics — is a separate read: **read [`references/agent-layer.md`](references/agent-layer.md) now (blocking) before configuring `agents:`/`agent_harnesses:` or running/debugging `sync-agents.sh`.**
```

- [ ] **Step 3: Re-point the moved-content assertions in `tests/test_sync_agents.sh`**

At each of the three `CONV=` definition sites (lines 403, 520, 787), add directly below it:
```bash
AGL="$REPO/skills/docket-convention/references/agent-layer.md"
```
Then change ONLY these assertions' target variable from `"$CONV"` to `"$AGL"` (leave the assertion names' text as is, except swap the word `convention` → `agent-layer ref` so failures read honestly): lines 407 (`auto => omit effort`), 414 (`default:` key + Pzoq proximity), 415 (cursor example), 416 (field-level fallback), 417 (non-Claude fallback warning), 421 (full built-in set), 422 (override-only), 423 (docket-dispatch.mdc), 523 (harness-neutral direct model IDs), 524 (passthrough), 525 (ADR-0015 proximity), 791 (Global table row), 800 (gitignored-never-committed), 801 (three-leg --check: advisory + docket:generated).

Directly after line 403's new `AGL=` line, add two new assertions:
```bash
assert "agent-layer reference exists" '[ -f "$AGL" ]'
assert "convention points at the agent-layer reference (blocking)" 'grep -qF "references/agent-layer.md" "$CONV"'
```

Assertions that stay on `"$CONV"` unchanged: 404 (`agents:` — the Configuration sample keeps it), 405 (`sync-agents.sh` — the stub names it), 406 (precedence — Config-layers paragraph keeps it), 408 (abort-and-report), 409 (`0017`), 411 (heading regex), 521–522 (`agent_harnesses` + `[claude]` — Configuration sample), 788–789 (`config.yml`, fence — Config-layers paragraph), 799 (`.docket.local.yml`).

- [ ] **Step 4: Run the targeted tests, then the full suite**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -c '^ok'` then `bash tests/test_sync_agents.sh 2>&1 | grep '^NOT OK'`
Expected: all-ok count, no NOT OK lines.
Run: `bash tests/test_composition_wiring.sh; bash tests/test_finalize_gate.sh; bash tests/test_docket_config.sh` — each must end `exit 0` / print no `NOT OK`.
Run the full suite loop. Expected: no output.

- [ ] **Step 5: Verify the move is a move, not a paraphrase (learnings #20)**

Run: `for p in "override-only" "docket-dispatch.mdc" "harness-neutral" "full built-in" "docket:generated"; do grep -qF "$p" skills/docket-convention/references/agent-layer.md && ! grep -qF "$p" skills/docket-convention/SKILL.md && echo "moved: $p" || echo "CHECK: $p"; done`
Expected: five `moved:` lines. (These five must exist ONLY in the reference now.)

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/references/agent-layer.md skills/docket-convention/SKILL.md tests/test_sync_agents.sh
git commit -m "docs(0053): extract agent-layer deep-dive to references/agent-layer.md; keep runtime contract inline"
```

---

### Task 3: Slim the rest of docket-convention — Step-0 preamble, Skill layer, Learnings ledger, Branch model, narration cuts

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (all sections except the Task-2 Agent-layer stub)

**Interfaces:**
- Consumes: `references/terminal-close-out.md` (Task 1) — the Branch model compression points at it.
- Produces: the `### Step-0 preamble` section (heading exactly `### Step-0 preamble (every operating skill)`) that Task 4 and changes 0054/0055 point at.

- [ ] **Step 1: Insert the new Step-0 preamble section** immediately after the `**`finalize` — the rebase-retest merge gate.**` paragraph (i.e., at the end of the Configuration section, before `### Agent layer`):

```markdown
### Step-0 preamble (every operating skill)

Every operating skill starts identically; skill bodies compress to a pointer here plus one line naming where their writes land.

1. Load this convention (blocking).
2. Resolve config + the bootstrap verdict: `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"` (fail-closed; read-only). Act on `BOOTSTRAP` — `PROCEED` → continue; `STOP_MIGRATE` → refuse and point at `migrate-to-docket.sh`; `CREATE_ORPHAN` → opt into `docket-config.sh --bootstrap` (fresh repo only).
3. Ensure + sync the **metadata working tree**. In `docket`-mode: the persistent `.docket/` worktree parked on `docket` (state-specific create per *Branch model*, idempotent); **sync before any read** — `git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`; pushes target `origin/docket`. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (= `origin/<integration_branch>` there). Skill prose that says "`.docket/`" / "`origin/docket`" reads as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

All metadata reads and writes happen in that tree on `metadata_branch`, pushed to its remote immediately.
```

- [ ] **Step 2: Compress the Skill layer section.** Keep the heading byte-identical and the role table verbatim. Replace the intro paragraph and the four bullets with:

```markdown
docket's five workflow steps are **pluggable roles**: the optional `skills:` map rebinds each to any skill name, or to the sentinel `auto`. An unset key defaults to the superpowers skill — an absent map is byte-identical to pre-0049 behavior.
```

(then the table, unchanged, then:)

```markdown
- **Passthrough.** A value is passed verbatim to the Skill tool — never validated against a registry (the ADR-0015 passthrough philosophy; exactly what lets any third-party or in-repo skill plug in). Unknown *role keys* are warned-and-ignored.
- **`auto` sentinel.** No skill is invoked; the running agent does the step itself at whatever model it already runs at. The per-role fallback defines only the **final artifact / stop-point** (column 4) — never the method.
- **Missing-skill rule — degrade to auto + warn.** If the resolved skill cannot be invoked at runtime, the invoking skill degrades to that role's `auto` fallback and warns prominently — in the run output and (for plan/build/review/finish) in the PR body. Softer than abort-and-report because skill availability is per-machine, not repo state.
- **Resolution** is deterministic via `docket-config.sh --export`, which emits `SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH` (defaulted when unset); skill bodies read the variable, never re-parse YAML. `docket-finalize-change`'s merge gate (`finalize.gate`) still validates regardless of the resolved build method.
```

- [ ] **Step 3: Compress the Learnings ledger section.** Keep the heading + the layout-block comment untouched elsewhere; replace the section body with:

```markdown
`<changes_dir>/LEARNINGS.md` — the project's **build-loop memory**: curated, hand-edited lessons, on `metadata_branch` only (never published to the integration branch; unlike the board it is prose, never regenerated). Flat dated entries, **newest first**, one to three lines with provenance: `- 2026-06-12 (#12, PR #7) — <what happened>. Apply: <the rule>.`

**Writing:** only the harvest at close-out appends (single source: the *Harvest learnings* step in `docket-finalize-change`; `docket-status`'s sweep invokes it by reference). Zero entries is normal; kills are not harvested. **Reading:** `docket-implement-next` at plan time and review; `docket-groom-next` before a brainstorm. **Distilling:** append-only until ~300 lines; the next harvest past the cap also distills — merge near-duplicates, drop entries promoted to CLAUDE.md or this convention. Distillation is **compression, not destruction** (git history keeps everything); durable conventions belong in CLAUDE.md, and promotion removes the entry here.
```

- [ ] **Step 4: Compress the Branch model section.** Replace the single giant paragraph with:

```markdown
Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — the primary working tree on the integration branch in single-branch (`main`) mode, the persistent `.docket/` worktree in `docket`-mode — and is **always pushed to its remote immediately** (the backlog, board, specs, and ADRs stay browsable on the remote at all times).

A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata.

On a terminal transition (`done` *or* `killed`), the driving skill runs the shared **terminal close-out** sequence — archive, re-render, **terminal-publish** (copying the archived change file + its `spec:` + the `Accepted` ADRs in `adrs:` from `origin/docket` onto the integration branch via `git checkout origin/docket -- <paths>`, never a `git merge docket` — the only flow of metadata onto the code line, also refreshing the integration-branch ADR index whenever the commit publishes an ADR), cleanup, board. Ordering, per-caller failure postures, and the `main`-mode degradation live in **[`references/terminal-close-out.md`](references/terminal-close-out.md) — read it before driving any terminal transition.** After a merge lands, both merge sites run the best-effort, FF-only `sync-integration-branch.sh` once at end of run to fast-forward the clone's local `<integration_branch>` checkout (a no-op in `main`-mode and on any non-FF/dirty/feature-branch tree).
```

- [ ] **Step 5: Apply the remaining narration cuts across the file** (each is a deletion or one-clause tightening; do NOT touch the sentinel phrases in the disposition table):
- Configuration section: in the `.docket.yml` sample comments, drop `(change 0045)`, `(change 0046)`, `(change 0049)`, `(change 0050)`, `(change 0051)`, `(change 0015)` and the `Can also be set in .docket.local.yml (change 0051).` trailing sentence (keep `agent_harnesses` + `.docket.local.yml` facts in the Config-layers paragraph). In the Config-layers paragraph drop `(change 0050, extended 0051)` from the bold lead and the legacy-`agents.yaml` migration sentence's `(original renamed .migrated; no dual-read remains)` parenthetical. Keep the misplaced-file and malformed-file warn rules.
- `board_surfaces` paragraph: unchanged except drop nothing — it carries no change numbers. Leave as is.
- Directory layout / Change manifest / Change body sections / ADR file / Lifecycle / Build-readiness / Autonomous grooming / Bootstrap guard: keep intact (test-dense); only cut `(change 0040)` in Branch model (already covered by Step 4's rewrite) and `(change 0022)`/`(change 0023)`-style parentheticals if any remain.
- GitHub board mirror section: keep both paragraphs; in the derived-view paragraph keep `render-change-links.sh` + `ADR-0012` (both load-bearing).

- [ ] **Step 6: Verify sentinels + size, run the suite**

Run: `bash tests/test_convention_extraction.sh 2>&1 | grep 'NOT OK'; bash tests/test_learnings_ledger.sh 2>&1 | grep 'NOT OK'; bash tests/test_docket_config.sh 2>&1 | grep 'NOT OK'; bash tests/test_results_artifact.sh 2>&1 | grep 'NOT OK'; bash tests/test_docket_metadata_branch.sh 2>&1 | grep 'NOT OK'; bash tests/test_board_refresh_on_transition.sh 2>&1 | grep 'NOT OK'`
Expected: no output.
Run: `wc -lw skills/docket-convention/SKILL.md`
Expected: ≤ ~210 lines and ≤ ~2,600 words (hard gate in Task 5 after status edits; if materially over, tighten the Configuration + Config-layers paragraphs further — narration only, never sentinel phrases).
Run the full suite loop. Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md
git commit -m "docs(0053): slim docket-convention — Step-0 preamble section, compressed Skill layer/Learnings/Branch model, narration cuts"
```

---

### Task 4: Slim docket-status — delete board Structure, rewire sweep to the close-out reference, compress Step-0

**Files:**
- Modify: `skills/docket-status/SKILL.md`
- Modify: `tests/test_closeout.sh:371-391`

**Interfaces:**
- Consumes: `references/terminal-close-out.md` (Task 1); the convention's `### Step-0 preamble (every operating skill)` (Task 3).

- [ ] **Step 1: Compress the Step-0 section.** Replace the body of `## Where the board, sweep, and checks operate` (keep the heading) with:

```markdown
Run the convention's *Step-0 preamble* — config export, `BOOTSTRAP` verdict, metadata-working-tree ensure + sync. All three passes read and write in that tree on `metadata_branch`, pushed to its remote immediately; the passes below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case.
```

- [ ] **Step 2: Move the two readiness renderings into the dependency-resolution section.** At the end of `## Shared dependency-resolution pass`, append:

```markdown
Readiness cells the board renders from this pass: a dependency-waiting change shows **⏳ waiting on #N — not yet built** or **⏳ waiting on #N — needs your merge** (never build-ready; waiting takes precedence over a missing spec). A `proposed` change that is not waiting, has no spec, and is not `trivial: true` shows **needs-brainstorm** — or **auto-groom blocked — needs you** when its body carries an `## Auto-groom blocked` section.
```

- [ ] **Step 3: Delete the board Structure + example.** Remove the entire `### Structure (in order)` section and the entire `### Example — abbreviated rendered BOARD.md` section (heading through the closing ````` ```` `````). The Board section keeps: the surface-dispatch paragraph, the full `inline` paragraph (invocation, never-hand-edit, commit discipline, **regenerate, never 3-way merge** rule, dropped-inline tradeoff), the full `github` paragraph, and the **No churny timestamp** line. In the `inline` paragraph, replace `it reproduces the *Structure* below byte-for-byte from the change files` with `its contract (`scripts/render-board.md`) documents the emitted structure`; drop `(change 0022)` references.

- [ ] **Step 4: Rewire sweep steps c–e.** Keep sweep intro, the finalize-only-gate note (keep the phrases `finalize-only`/`never merges`/`only archives already-merged` — one must survive for test_finalize_gate:157), and per-change steps 1–2. Replace steps 3.c–3.e and the failure-posture block with:

```markdown
   c–e. **Close out via the shared sequence** — run the convention's terminal close-out
   (**read `../docket-convention/references/terminal-close-out.md` now — blocking**) with
   `--outcome done` and the UTC merge date from step b, through its **cleanup** step (steps 1–4:
   archive, re-render, terminal-publish onto the integration branch in `docket`-mode, cleanup —
   the reference owns invocations, ordering, and the `main`-mode degradation). Its step 5 (board)
   is covered by this same `docket-status` run's own Board pass — do not re-render per change.

   **Sweep posture (steps c–e):** the sweep is a bulk janitor draining N changes — on any non-zero
   exit, **log it, abandon the remainder of this change's close-out, and continue to the next
   change**; the next sweep self-heals idempotently. A failed `render-change-links.sh` follow-on
   commit **skips publish** (a stale `## Artifacts` block is never published). This posture is
   **deliberately divergent from `docket-finalize-change`'s** abort-and-report — the sequence is
   shared, the failure posture is not. Determinism: concurrent archivers produce byte-identical
   change-file-only commits; `BOARD.md` is regenerated separately, never hand-merged.
```

DELETE step g (cleanup — now inside the reference sequence run by c–e). KEEP step h verbatim (harvest is sweep-side: `Harvest learnings` + `docket-finalize-change` + best-effort). Keep the **Sync the integration checkout** paragraph (trim `(change 0029)`). DELETE the trailing `**Determinism invariant.**` and `**Note:** This archive procedure is identical…` paragraphs (folded into the posture block above / the reference).

- [ ] **Step 5: Health checks — cut the retired-footnote narration.** In the `github mirror reachability` bullet, delete the italic parenthetical starting `*(The paired `inline` board/source-drift check was **retired by change 0024**…` through its closing `)*`. Keep everything else in Health checks verbatim (it is test-dense). Also trim `(change 0023)` from the mechanical-checks lead-in.

- [ ] **Step 6: Re-point the moved sweep assertions in `tests/test_closeout.sh`.** Below line 371 (`STATUS=…`), add:
```bash
TCO="$REPO/skills/docket-convention/references/terminal-close-out.md"
assert "wiring(status): sweep points at the terminal-close-out reference" 'grep -qF "terminal-close-out.md" "$STATUS"'
```
Re-point these from `"$STATUS"` to `"$TCO"` (rename `status` → `close-out ref` in the assertion labels): 374 (`/archive-change.sh`), 375 (`/terminal-publish.sh`), 376 (`/cleanup-feature-branch.sh`), 384 (the render-before-publish ordering awk). Keep on `"$STATUS"` unchanged: 380 (no `git mv`), 386 (`render-change-links.sh`), 389 (`abandon the remainder`), 391 (`deliberately divergent`).

- [ ] **Step 7: Run targeted tests + full suite + size check**

Run: `for t in test_closeout test_render_board test_board_checks test_auto_groom test_convention_extraction test_learnings_ledger test_results_artifact test_docket_metadata_branch test_finalize_gate; do bash tests/$t.sh 2>&1 | grep 'NOT OK' && echo "^^ $t"; done`
Expected: no output.
Run: `wc -lw skills/docket-status/SKILL.md` — Expected: ≤ ~110 lines / ≤ ~1,600 words.
Run the full suite loop. Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-status/SKILL.md tests/test_closeout.sh
git commit -m "docs(0053): slim docket-status — structure prose delegated to render-board contract, sweep rewired to terminal-close-out reference"
```

---

### Task 5: Verification sweep — anchor grep-gate, size gate, behavior-neutrality diff pass, read-only smoke

**Files:**
- Modify: none (verification only; fixes loop back into the file they belong to)

- [ ] **Step 1: Anchor grep-gate.** Every convention section name referenced anywhere must still exist as a heading:

```bash
CONV=skills/docket-convention/SKILL.md
for a in "Configuration" "Step-0 preamble" "Agent layer" "Skill layer" "Directory layout" "Change manifest" "Change body sections" "ADR file" "Lifecycle" "Build-readiness" "Autonomous grooming" "Learnings ledger" "GitHub board mirror" "Bootstrap guard" "Branch model"; do
  grep -q "^### .*$a\|^### $a" "$CONV" || echo "MISSING ANCHOR: $a"
done
grep -rn "convention's \*[A-Za-z]" skills/ agents/ scripts/*.md README.md | grep -o "convention's \*[^*]*\*" | sort -u
```
Expected: no `MISSING ANCHOR`; every listed `convention's *X*` name appears in the heading list above.

- [ ] **Step 2: Size gate (recorded for the PR body).**

```bash
wc -lw skills/docket-convention/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/references/agent-layer.md skills/docket-convention/references/terminal-close-out.md
```
Expected: convention ≤ ~200 L / ≤ 2,500 w; status ≤ ~110 L / ≤ 1,600 w; each reference ≤ ~150 L (TOC present if > 100 — verify with `head -12 <file>`). If a target is missed, tighten narration (never sentinels) and re-run.

- [ ] **Step 3: Behavior-neutrality diff pass.** Read `git diff origin/main -- skills/docket-convention/SKILL.md skills/docket-status/SKILL.md` hunk by hunk; for every deleted sentence confirm one of: (a) narration, (b) restated inline, (c) present in a reference file (`grep -F` the key phrase against `references/*.md`), (d) covered by a script contract (`scripts/render-board.md` for the Structure section). Record any sentence that fails all four and restore it.

- [ ] **Step 4: Read-only smoke of the refactored docket-status.** Follow the refactored `skills/docket-status/SKILL.md` text literally, against this clone's real metadata, WITHOUT writing:

```bash
eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)" && echo "BOOTSTRAP=$BOOTSTRAP"
"${DOCKET_SCRIPTS_DIR:?}"/render-board.sh --changes-dir /Users/homer/dev/docket/.docket/docs/changes > /tmp/smoke-board.md \
  && diff /tmp/smoke-board.md /Users/homer/dev/docket/.docket/docs/changes/BOARD.md && echo BOARD-IDENTICAL
"${DOCKET_SCRIPTS_DIR:?}"/board-checks.sh --changes-dir /Users/homer/dev/docket/.docket/docs/changes \
  --metadata-branch docket --integration-branch origin/main
```
Expected: `BOOTSTRAP=PROCEED`; `BOARD-IDENTICAL` (or a diff explained solely by real backlog movement since the last board commit); board-checks prints nothing (clean) or only pre-existing findings. Confirm the refactored skill text contains every command just run (the imperatives survived the cut). The sweep leg has no merged-`implemented` change to exercise — note that in the results file.

- [ ] **Step 5: Full suite, final.**

Run: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAIL: $t"; done`
Expected: no output. Then `git status --short` — expected: clean (verification made no changes) or only deliberate fixes, which get committed with `docs(0053): verification fixes — <what>`.

---

## Self-review notes

- Spec §1 (convention core+references) → Tasks 2–3. Spec §2 (status) → Task 4. Spec §3 (Step-0 preamble) → Task 3 Step 1 + Task 4 Step 1. Spec §5 verification → Task 5 (grep-gate, neutrality, smoke, sizes). Spec guardrails (byte-stable headings, one-level refs, TOC) → Global Constraints + Task 1 Step 2 / Task 2 Step 1.
- Out of scope respected: no script/contract edits; test edits are confined to re-pointing sentinels whose content moved (tests follow content; the alternative — leaving them on SKILL.md — would force the content to stay, defeating the change).
- The `0017`/`(change 0016)` bare tokens are kept deliberately (test-anchored; spec decision 2 targets narration, not bare pointers).
