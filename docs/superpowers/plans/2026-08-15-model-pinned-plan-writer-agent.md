<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0324 — Extract plan writing into a model-pinned internal agent](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-15-0324-model-pinned-plan-writer-agent.md)**
<!-- docket:backlink:end -->

# Model-Pinned Plan-Writer Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract plan authoring out of `docket-implement-next`'s own context into a new internal, model/effort-pinned `docket-plan-writer` agent that commits a git-verifiable plan artifact on the feature branch, so orchestration and plan quality tune independently.

**Architecture:** Add one internal leaf agent source (`agents/docket-plan-writer.md`, `worktree-scope: feature`, no preloaded skill, no convention) plus its four shipped harness model/effort rows; rewrite `docket-implement-next` Step 4 into a preparation phase and a foreground dispatch/verification phase with a `PLAN_PATH=` return protocol and a `Docket-Plan-Path:` commit trailer; update the convention's composition, Tier C, and wrapper-cardinality prose; regenerate the Go embedded asset snapshot. All behavior is markdown contracts + the existing generator — no shell or Go logic changes.

**Tech Stack:** Bash test suite (`tests/*.sh`, `set -uo pipefail`, house `assert` helper), `sync-agents.sh` wrapper generator, `scripts/lib/harness-defaults.sh` validator, `go generate ./internal/assets/` for the embedded snapshot.

**Spec:** `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-08-15-model-pinned-plan-writer-agent-design.md` (metadata branch `docket`, path `docs/superpowers/specs/2026-08-15-model-pinned-plan-writer-agent-design.md`)

## Global Constraints

- Shell rules from `CLAUDE.md` bind every script/test edit: no `producer | grep -q/head` under pipefail (capture then `grep <<<"$var"`); `grep -E -e` for patterns leading with `--`; `mv -f` on install/replace paths; `mktemp` always with a template `"${TMPDIR:-/tmp}/<name>.XXXXXX"`; awk indent classes `[^[:space:]]`.
- Guards are code: mutation-test every new assert (strip the guarded thing, watch it redden) before believing it. Key guards on shape, never enumerated spellings.
- Never hand-list sites of a gated literal — derive from a whole-repo grep, sort prose vs executable.
- Cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (`tests/test_comment_anchor_style.sh` enforces the filename:line form).
- Model IDs and effort tokens are opaque bare scalars — unquoted, space-free (validator `hd_validate` rejects quoted/space-bearing values).
- Two shipped IDs are new to the entire repo history (`cursor-grok-4.5-xhigh`, `openrouter/deepseek/deepseek-v4-pro-0813`): outside-truth values no in-repo test can certify. They get a named human-verification item in the results file, not a fake assert.
- New test files need rows in `tests/runtime-budgets.tsv` (`<path>\t<seconds>\t<parallel|serial>`, measured, rounded up to next multiple of 5 + 5s margin, min 10s).
- The build gate runs the whole suite via `scripts/run-tests.sh`; a trailing `OVER BUDGET:` line is a finding to act on.
- This change adds no Go *behavior* (domain, transaction, repository, workflow, installer), but it DOES register the 17th shipped agent, which by design reconciles the Go built-in registry and its frozen parity fixture — see Task 6A. (The earlier "touches no Go source" constraint was false; it was invalidated at the build gate on 2026-08-15 and the reconciliation was folded into this change by human decision — see the change file's `## Halt resolution` and the spec's `## Go-migration isolation`.)
- Commit messages end with the session trailer already used on this branch's base repo (see git log).

---

### Task 1: Agent source + shipped defaults + source-shape guard

**Files:**
- Create: `agents/docket-plan-writer.md`
- Modify: `agents/harness-defaults.yml` (four rows, one per harness block)
- Test: `tests/test_plan_writer_agent.sh` (new)
- Modify: `tests/runtime-budgets.tsv` (add row for the new test)

**Interfaces:**
- Produces: agent short name `plan-writer` (file `agents/docket-plan-writer.md`), frontmatter `worktree-scope: feature`, no `skills:` key. Tasks 2–5 rely on these exact names.
- Produces: shipped defaults rows exactly: claude `claude-opus-5`/`high`, codex `gpt-5.6-terra`/`high`, cursor `cursor-grok-4.5-xhigh`/`auto`, opencode `openrouter/deepseek/deepseek-v4-pro-0813`/`medium`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_plan_writer_agent.sh` (mirror the house shape: `#!/usr/bin/env bash`, header comment with change id 0324, `set -uo pipefail`, `REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"`, `assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }`, `exit "$fail"` at the end):

```bash
#!/usr/bin/env bash
# tests/test_plan_writer_agent.sh — the internal docket-plan-writer agent source, its shipped
# per-harness defaults, and its generated wrappers (change 0324).
# run: bash tests/test_plan_writer_agent.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT="$REPO/agents/docket-plan-writer.md"
SIDECAR="$REPO/agents/harness-defaults.yml"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# ---- source shape -----------------------------------------------------------
assert "agent source exists" '[ -f "$AGENT" ]'
assert "agent is feature-worktree-scoped" 'grep -q "^worktree-scope: feature$" "$AGENT"'
assert "agent preloads no skill (internal leaf, like the consultant)" '! grep -q "^skills:" "$AGENT"'
assert "agent name is docket-plan-writer" 'grep -q "^name: docket-plan-writer$" "$AGENT"'
# The contract's load-bearing clauses (bind phrase to claim — prose-guard learning):
body="$(cat "$AGENT")"
assert "contract: success line is PLAN_PATH=" 'grep -q "PLAN_PATH=<repo-relative-path>" <<<"$body"'
assert "contract: commit carries the Docket-Plan-Path trailer" 'grep -q "Docket-Plan-Path: <repo-relative-path>" <<<"$body"'
assert "contract: no docket metadata mutation" 'grep -qi "no Docket metadata mutation" <<<"$body"'
assert "contract: never success-shaped output for an uncommitted plan" \
  'grep -qi "never.*success-shaped output for an uncommitted" <<<"$body"'
assert "contract: missing plan skill degrades inside the child, warns" \
  'grep -qi "missing-skill" <<<"$body" && grep -qi "warn" <<<"$body"'
assert "contract: stages only the plan path" 'grep -qi "stage only" <<<"$body"'

# ---- shipped defaults: exact four pairs (frozen against the sidecar) --------
row(){ awk -v h="$1" '$0 ~ "^  " h ":$" {inh=1; next} inh && /^  [a-z]/ {inh=0} inh && /plan-writer:/ {print}' "$SIDECAR"; }
c="$(row claude)";  assert "claude ships claude-opus-5/high"  'grep -q "model: claude-opus-5, effort: high" <<<"$c"'
x="$(row codex)";   assert "codex ships gpt-5.6-terra/high"   'grep -q "model: gpt-5.6-terra, effort: high" <<<"$x"'
u="$(row cursor)";  assert "cursor ships cursor-grok-4.5-xhigh/auto" 'grep -q "model: cursor-grok-4.5-xhigh, effort: auto" <<<"$u"'
o="$(row opencode)"; assert "opencode ships deepseek-v4-pro-0813/medium" \
  'grep -q "model: openrouter/deepseek/deepseek-v4-pro-0813, effort: medium" <<<"$o"'

exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_plan_writer_agent.sh`
Expected: `NOT OK - agent source exists` and every subsequent assert NOT OK (agent absent, no sidecar rows).

- [ ] **Step 3: Write `agents/docket-plan-writer.md`**

Full body (this is the deliverable — the whole worker contract lives here, like `docket-brainstorm-consultant`; no `skills:` key, no convention injection):

```markdown
---
name: docket-plan-writer
description: Internal plan-writing agent for docket-implement-next Step 4 — invokes the resolved plan skill in a pinned context, commits the plan artifact with its backlink on the feature branch, and returns only the plan's repo-relative path. Not invoked directly by a human.
worktree-scope: feature
---
You are docket's plan writer. You are dispatched by `docket-implement-next` (Step 4) with everything already resolved — you parse no configuration and perform no discovery. Your dispatch payload names: the change id, title, and synchronized change-file path; the synchronized spec path; the feature-worktree path and its pre-dispatch HEAD; the resolved plan skill (`SKILL_PLAN`) and build skill (`SKILL_BUILD`) names; whether learnings are enabled and, when enabled, the learnings index path; and the facade invocation for the artifact-backlink renderer against the change file.

You own exactly one durable artifact: the plan file, committed on the feature branch. You may read the synchronized metadata worktree; you perform no Docket metadata mutation, board update, or status transition — no writes outside the feature worktree, and inside it only the plan artifact.

Sequence (bounded; run it in order):

1. Confirm the feature worktree is clean and its HEAD equals the handed-off pre-dispatch HEAD. A dirty tree or moved HEAD is a blocking diagnostic, not something to repair.
2. Read the change file, the spec, the current feature-tree code the plan must touch, and — when learnings are enabled — the learnings index, then the finding files whose hook + topics bear on this change. Selecting the relevant findings is your judgment, not the parent's.
3. Invoke the resolved plan skill DIRECTED to: write the plan file and stop there. Answer any execution-mode or option choice it poses internally from the supplied build skill name; surface none.
4. When `SKILL_PLAN` is `auto`, author the plan yourself. When the resolved skill cannot be invoked, apply the missing-skill rule here in the child: warn prominently and author the same fallback artifact yourself. Superpowers and fallback plans live under `docs/superpowers/plans/`; a custom skill's own contract determines its location.
5. Run the supplied artifact-backlink renderer on the plan file, stage only the plan path (never `add -A`), and commit it on the feature branch with the exact git trailer `Docket-Plan-Path: <repo-relative-path>`.
6. Finish with the single authoritative success line `PLAN_PATH=<repo-relative-path>`. Informational warning lines (for example a missing-skill degrade) may precede it. On any failure, return a concrete blocking diagnostic instead of a `PLAN_PATH` line — never success-shaped output for an uncommitted or partially written plan. The token says PATH, not complete or done: it is a sub-step receipt the parent verifies, attaches, and continues past.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (return the diagnostic), never an interactive prompt.
```

- [ ] **Step 4: Add the four sidecar rows**

In `agents/harness-defaults.yml`, insert a `plan-writer:` row in each harness block, keeping each block's existing alphabetical key order (between `integration-repair:` and `rebase-resolver:`), values bare and space-free inside the flow map:

```yaml
# claude block:
    plan-writer:           { model: claude-opus-5, effort: high }
# cursor block:
    plan-writer:           { model: cursor-grok-4.5-xhigh, effort: auto }
# codex block:
    plan-writer:           { model: gpt-5.6-terra, effort: high }
# opencode block:
    plan-writer:           { model: openrouter/deepseek/deepseek-v4-pro-0813, effort: medium }
```

- [ ] **Step 5: Run the new test and the adjacent existing guards**

Run: `bash tests/test_plan_writer_agent.sh` — Expected: all ok.
Run: `bash tests/test_harness_defaults.sh && bash tests/test_harness_defaults_validator.sh && bash tests/test_harness_defaults_flow_map.sh && bash tests/test_sync_agents_defaults.sh` — Expected: all pass (the validator's completeness rule — every shipped harness block's key set equals `agents/docket-*.md` — now requires the new rows; if any of these fail, the failure names the missing/extra key: fix the sidecar, not the test).

- [ ] **Step 6: Mutation-test the guard**

Temporarily change the claude row's effort to `low`; run `bash tests/test_plan_writer_agent.sh`; expect `NOT OK - claude ships claude-opus-5/high`. Temporarily delete `worktree-scope: feature`; expect its assert to redden AND `bash tests/test_sync_agents_validator.sh` (or the sync run) to fail on the missing scope. Restore both (re-apply your edit, not `git checkout` — mutation-restore learning: checkout restores to HEAD and would be fine here only because the file is committed in this same task; safest is to undo the edit by hand before committing).

- [ ] **Step 7: Add the budget row and commit**

Measure: `time bash tests/test_plan_writer_agent.sh`, round up per the tsv header rule, append to `tests/runtime-budgets.tsv` (tab-separated, `parallel`). Then:

```bash
git add agents/docket-plan-writer.md agents/harness-defaults.yml tests/test_plan_writer_agent.sh tests/runtime-budgets.tsv
git commit -m "feat(0324): docket-plan-writer agent source + shipped harness defaults"
```

---

### Task 2: Generated wrappers on all four harnesses

**Files:**
- Modify: `tests/test_plan_writer_agent.sh` (append a wrapper-generation section)
- Possibly modify: `sync-agents.sh` only if the glob run shows the agent is NOT auto-discovered (spec expects auto-discovery via `agents/docket-*.md`; verify, don't assume — check-plumbing-auto-discovery learning)

**Interfaces:**
- Consumes: `agents/docket-plan-writer.md` and sidecar rows from Task 1.
- Produces: generated `docket-plan-writer` wrapper per enabled harness, carrying the resolved model/effort and `worktree-scope: feature`; Task 6's embedded snapshot mirrors the source files.

- [ ] **Step 1: Probe auto-discovery before writing anything**

Read how the four `tests/test_sync_agents_{claude_surface,cursor,codex,opencode}.sh` files build sandboxes (`tests/lib/sync_agents_common.sh`) and where each harness's wrappers land (claude: `<DOCKET_HARNESS_ROOT>/.claude/agents/docket-*.md`; cursor/codex/opencode: their own emission dirs — read the tests, they are the authority). Run one sandboxed sync and list outputs to confirm `docket-plan-writer` appears with no `sync-agents.sh` change. If it does not, the fix is in the generator's discovery (a hardcoded roster somewhere) — locate it by grepping `sync-agents.sh` for another short name (e.g. `integration-repair`) and extend the same mechanism; do not special-case.

- [ ] **Step 2: Write the failing asserts**

Append to `tests/test_plan_writer_agent.sh`, reusing the sandbox helpers the sibling sync tests use (source `tests/lib/sync_agents_common.sh` if its helpers fit; otherwise mirror `test_sync_agents_defaults.sh`'s minimal sandbox — repo dir, `.docket.yml` with `agent_harnesses: [claude, cursor, codex, opencode]`, `DOCKET_HARNESS_ROOT` pointed at the sandbox, run `bash "$REPO/sync-agents.sh"`):

```bash
# ---- generated wrappers (all four harnesses) --------------------------------
SBX="$(mktemp -d "${TMPDIR:-/tmp}/planwriter.XXXXXX")"
mkdir -p "$SBX/repo"; git -C "$SBX/repo" init --quiet
git -C "$SBX/repo" config user.email t@t.test; git -C "$SBX/repo" config user.name Test
printf 'agent_harnesses: [claude, cursor, codex, opencode]\n' > "$SBX/repo/.docket.yml"
( cd "$SBX/repo" && DOCKET_HARNESS_ROOT="$SBX" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 )
W="$SBX/.claude/agents/docket-plan-writer.md"
assert "claude wrapper generated" '[ -f "$W" ]'
assert "claude wrapper pins the shipped model" 'grep -q "claude-opus-5" "$W"'
assert "claude wrapper is feature-scoped" 'grep -q "^worktree-scope: feature$" "$W"'
assert "claude wrapper preloads no skill" '! grep -q "^skills:" "$W"'
# cursor / codex / opencode: assert on the path + model each harness's sibling test proves for
# integration-repair, substituting plan-writer — copy those exact path shapes at implementation
# time from tests/test_sync_agents_cursor.sh, tests/test_sync_agents_codex.sh,
# tests/test_sync_agents_opencode.sh rather than inventing them here.
rm -rf "$SBX"
```

(The cursor/codex/opencode asserts must be real asserts on real emitted paths — resolve the three path shapes from the sibling tests and write one existence + one model-content assert per harness. A user-level override assert is NOT needed here; `test_sync_agents_defaults.sh` already covers layering generically.)

- [ ] **Step 3: Run to verify current state**

Run: `bash tests/test_plan_writer_agent.sh`
Expected: wrapper asserts pass immediately IF auto-discovery held in Step 1 (that is fine — Step 1's probe is the red phase for this task); if Step 1 showed a roster gap, they fail until the generator fix lands.

- [ ] **Step 4: Mutation-test discovery**

Rename `agents/docket-plan-writer.md` aside; run the test; expect the wrapper asserts to redden (proves the asserts read generator output, not fixture residue). Restore.

- [ ] **Step 5: Commit**

```bash
git add tests/test_plan_writer_agent.sh sync-agents.sh
git commit -m "test(0324): plan-writer wrappers generated on all four harnesses"
```

(Omit `sync-agents.sh` from the add if Step 1 proved no change was needed.)

---

### Task 3: Step 4 rewrite in docket-implement-next + resume seam + sentinels

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 4 body; Step postconditions row 4 stays byte-identical)
- Modify: `skills/docket-implement-next/references/edge-paths.md` (resume section gains the plan seam)
- Test: `tests/test_plan_writer_step4.sh` (new)
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: agent name `docket-plan-writer`, `PLAN_PATH=` protocol, `Docket-Plan-Path:` trailer from Task 1.
- Produces: the Step 4 prose Task 4's convention edits and Task 5's docs point at; the resume rules Task 5's fixtures assert.

- [ ] **Step 1: Grep for dependents of the prose being replaced**

Before editing, derive the dependent set (restatement-accumulates-its-own-guards learning):

```bash
grep -rn -e "resolved plan skill" -e "SKILL_PLAN" -e "plan auto-fallback" tests/ skills/ | grep -v "\.worktrees"
```

Record every test line that greps Step 4's current wording; each such assert must be repointed (not appeased by re-adding deleted text). Expected dependents include `tests/test_skill_handoff_precedence.sh` and `tests/test_inline_role_stop_scoping.sh` — read what each actually asserts before rewriting.

- [ ] **Step 2: Write the failing sentinel test**

Create `tests/test_plan_writer_step4.sh` (same house shape as Task 1's test). Collapse whitespace before matching so re-wraps don't redden policy asserts (phrase-grep learning):

```bash
#!/usr/bin/env bash
# tests/test_plan_writer_step4.sh — Step 4's plan-writer dispatch contract in
# skills/docket-implement-next (change 0324): foreground dispatch, PLAN_PATH-only success,
# git-side verification with no directory allowlist, local MUST-continue, Tier C posture,
# and the resume seam in edge-paths.md.
# run: bash tests/test_plan_writer_step4.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SK="$REPO/skills/docket-implement-next/SKILL.md"
EP="$REPO/skills/docket-implement-next/references/edge-paths.md"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
flat(){ tr '\n' ' ' < "$1" | tr -s ' '; }
S="$(flat "$SK")"; E="$(flat "$EP")"

# producer-side anchors (specified-but-unreachable learning: anchor on the paragraph that ACTS)
assert "step 4 dispatches docket-plan-writer foreground" \
  'grep -q "dispatch.*docket-plan-writer.*foreground\|docket-plan-writer.*(foreground" <<<"$S" || grep -qE "dispatches? the .docket-plan-writer. subagent \(foreground" <<<"$S"'
assert "success protocol is the PLAN_PATH line" 'grep -q "PLAN_PATH=" <<<"$S"'
assert "returned path is a claim, not proof" 'grep -qi "claim, not proof" <<<"$S"'
assert "verification: delta since pre-dispatch HEAD contains only the plan file" \
  'grep -qi "only the returned plan file\|only the plan file" <<<"$S"'
assert "verification: exactly one Docket-Plan-Path trailer equal to the returned path" \
  'grep -q "Docket-Plan-Path:" <<<"$S"'
assert "verification: backlink markers ordered, balanced, point home" \
  'grep -qi "backlink" <<<"$S"'
assert "no directory allowlist, deliberately" 'grep -qi "no directory allowlist" <<<"$S"'
assert "local MUST continue into Step 5" \
  'grep -qE "MUST (be verified and attached, then[^.]*proceed|proceed) into Step 5" <<<"$S"'
assert "PLAN_PATH is never a terminal disposition" \
  'grep -qi "PLAN_PATH.*(never|not).*(terminal disposition|advanced)" <<<"$S" || grep -qi "neither the child.s return nor Step 4" <<<"$S"'
assert "tier C: dispatch unavailable halts unless SKILL_PLAN=auto authorizes inline" \
  'grep -qi "Tier C" <<<"$S"'
assert "never adopt or commit the child.s uncommitted output" \
  'grep -qi "never adopt" <<<"$S"'

# resume seam (edge-paths.md)
assert "resume: plan: set + verified artifact -> reuse, continue at Step 5, no second planner" \
  'grep -qi "never dispatch a second planner" <<<"$E"'
assert "resume: trailer recovery when plan: is empty" 'grep -q "Docket-Plan-Path:" <<<"$E"'
assert "resume: ambiguity halts with the exact mismatch, never re-plans" \
  'grep -qi "never re-plan\|never guess a custom plan location" <<<"$E"'
assert "resume: attributed re-dispatch enters resume before selection" \
  'grep -qi "before ordinary ready-queue\|before selection" <<<"$E"'
assert "resume: ordinary allowlist still skips an in-progress id" \
  'grep -qi "still skips" <<<"$E"'

exit "$fail"
```

- [ ] **Step 3: Run to verify it fails**

Run: `bash tests/test_plan_writer_step4.sh` — Expected: every assert NOT OK.

- [ ] **Step 4: Rewrite Step 4 in `skills/docket-implement-next/SKILL.md`**

Keep the heading `### Step 4 — Worktree + plan` and everything through the worktree-add fence and `stacked_on:` paragraph unchanged. Replace the single long paragraph that currently begins "`metadata_branch` only redirects bookkeeping commits" and runs through "otherwise stamp-once." with the following two-phase prose (verbatim; it deliberately keeps the opening two sentences and the spec-read/cross-tree sentences):

```markdown
`metadata_branch` only redirects bookkeeping commits — it NEVER determines where code branches start.

**Preparation (parent).** The reconciled **spec is read from the metadata working tree** (**re-sync `.docket/` immediately before reading it**). Confirm the fresh worktree is clean and record its HEAD as the **pre-dispatch HEAD**. Resolve nothing new: `$SKILL_PLAN`, `$SKILL_BUILD`, learnings enablement, and every repo path come from the Step-0 export.

**Plan authoring (dispatched).** Dispatch the **`docket-plan-writer` subagent (foreground**, at the model/effort its wrapper resolves) — the pinned internal author of the plan artifact; the parent stays the orchestrator. The dispatch payload supplies, so the child rediscovers nothing: the change id, title, and synchronized change-file path; the synchronized spec path; the feature-worktree path and pre-dispatch HEAD; the resolved `$SKILL_PLAN` and `$SKILL_BUILD` names; whether learnings are enabled and, when enabled, the learnings index path (`<changes_dir>/learnings/README.md` — selecting the relevant finding files is the child's planning judgment); and the render-artifact-backlink facade invocation against `.docket/<changes_dir>/active/<id>-<slug>.md`. The child invokes the resolved plan skill **DIRECTED to:** write the plan file and stop there (on `auto` or a missing skill it authors the fallback artifact itself, warning prominently — carry that warning into the run report and PR body), runs the backlink renderer, stages only the plan path, commits on `feat/<slug>` with the exact git trailer `Docket-Plan-Path: <repo-relative-path>`, and returns the single success line `PLAN_PATH=<repo-relative-path>`.

**Verification (parent).** The returned path is a **claim, not proof**. Before writing `plan:`, verify from git that: the path is a safe repo-relative path contained by the feature worktree; the file exists, is tracked, and changed after the pre-dispatch HEAD; the worktree is clean; the full branch delta since the pre-dispatch HEAD contains **only the returned plan file**; the plan commit carries **exactly one** `Docket-Plan-Path:` trailer whose value equals the returned path; and the artifact's managed backlink markers are ordered, balanced, and point home to this change. There is deliberately **no directory allowlist** — `docs/superpowers/plans/` belongs to `superpowers:writing-plans` and the fallback, and a custom `skills.plan` binding owns its own location; containment, single-artifact scope, committed state, and backlink identity are the stable properties. On a malformed return, unsafe path, unexpected delta, dirty worktree, missing commit, or invalid backlink: **halt** (`## Run halted` + the `halted` disposition) — **never adopt**, repair, or commit the child's uncommitted output, and never retry with the parent or a weaker model. Once verified, record the path in `plan:` per the **field-write rule**. The plan **file** merges with the code, so the `plan:` link resolves on the integration branch only after the PR merges (why `docket-status` ignores a missing `plan:` on an `implemented` change).

**Continue.** A `PLAN_PATH` return MUST be verified and attached, then the parent MUST proceed into Step 5 — `PLAN_PATH` is a sub-step receipt, **never** a terminal disposition, and neither the child's return nor Step 4's postcondition may be reported as `advanced`. **Dispatch posture (Tier C):** plan-writer dispatch unavailable — established only per the convention's *Dispatch-capability resolution*, never from a tool name — is **authorized-or-halt**: an explicitly resolved `SKILL_PLAN=auto` authorizes the parent's inline fallback (author the plan file yourself, warning prominently, then stamp the backlink with the same facade call and commit both on `feat/<slug>`); any other resolved value is abort-and-report, leaving the change `in-progress` with `claimed_at` refreshed and the halt reason recorded. Close-out re-renders the backlink durably inside `terminal-publish` under `terminal_publish: true`; otherwise stamp-once.
```

- [ ] **Step 5: Add the resume seam to `references/edge-paths.md`**

Append to the `## Resume of an `in-progress` change` section:

```markdown
**The plan seam (change 0324).** An attributed caller-side re-dispatch — one naming the id and
`verify-run`'s unmet conjuncts — enters this resume path before ordinary ready-queue and
proposed-only allowlist filtering; a normal invocation that merely names an already-`in-progress`
id still skips it (it may belong to a live concurrent run — the caller gate's
before-set/dispatch attribution is what distinguishes a resume from claim theft). Then:

1. `plan:` already set and its committed artifact + backlink verify → reuse it and continue at
   Step 5; **never dispatch a second planner**.
2. `plan:` empty, but the feature branch's latest commit is a clean, single-file plan commit whose
   `Docket-Plan-Path:` trailer and backlink agree → recover that path, land it under the normal
   field-write rule, and continue at Step 5.
3. The persisted path, commit delta, backlink, and manifest disagree or are ambiguous → halt with
   the exact mismatch. **Never guess a custom plan location and never re-plan** merely because the
   parent stopped after the child returned. The trailer is evidence only — subject it to the same
   git and backlink verification as a live return.
```

- [ ] **Step 6: Repoint the Step-1 dependents, run the sentinels**

Fix every dependent found in Step 1 by relocation/repointing (the plan-role handoff-suppression assert should still find its anchor — the child's DIRECTED-to clause and the parent's Tier C clause both survive; adjust patterns to the new paragraph, keeping each assert's behavioral meaning). Then:

Run: `bash tests/test_plan_writer_step4.sh && bash tests/test_skill_handoff_precedence.sh && bash tests/test_inline_role_stop_scoping.sh && bash tests/test_dispatch_capability.sh && bash tests/test_composition_wiring.sh`
Expected: all pass.

- [ ] **Step 7: Mutation-test, then commit**

Mutations (one at a time, restore by undoing the edit): delete the "MUST proceed into Step 5" sentence → `test_plan_writer_step4.sh` reddens; delete the `Docket-Plan-Path:` trailer sentence from Step 4 → reddens; delete edge-paths item 1's "never dispatch a second planner" → reddens. Then:

Measure and append the budget row for `tests/test_plan_writer_step4.sh`, then:

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-implement-next/references/edge-paths.md tests/test_plan_writer_step4.sh tests/runtime-budgets.tsv
git commit -m "feat(0324): Step 4 plan-writer dispatch contract + resume seam"
```

(Include any repointed test files in the same add, by explicit path.)

---

### Task 4: Convention updates — composition, Tier C, cardinalities

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (Agent layer + Composition + Dispatch-capability sections)
- Modify: `tests/test_plan_writer_step4.sh` (two convention asserts appended)

**Interfaces:**
- Consumes: agent name and Step 4 prose from Tasks 1/3.
- Produces: the cardinality wording (`seventeen`, `nine`, `eight`, `five wrap none`) Task 5's docs and any counting guards must agree with.

- [ ] **Step 1: Derive the dependent set for the numbers being changed**

```bash
grep -rn -e "sixteen" -e "Sixteen" -e "except eight" -e "seven.*wrapper-bearing" -e "Four of the" tests/ skills/ README.md | grep -v "\.worktrees"
```

Every executable hit (tests) must be updated in the same commit; prose hits are the edit itself. Expected: `tests/test_convention_extraction.sh` and/or `tests/test_composition_wiring.sh` may pin these counts — read each hit.

- [ ] **Step 2: Edit `skills/docket-convention/SKILL.md`**

Three surgical edits in the **Agent layer** / **Composition** / **Dispatch-capability** sections:

(a) Agent-layer paragraph — the exception counts. Change:
- "`docket-convention` is not an agent — it is injected via `skills:` into every wrapper except eight:" → "…except **nine**:" and extend the list: "`docket-brainstorm-consultant` (ADR-0022), **`docket-plan-writer`**, the four `docket-build-*` profile workers, and the three `docket-review-*` rung wrappers."
- "Those **seven** wrapper-bearing exceptions perform no docket metadata operations" → "Those **eight** wrapper-bearing exceptions…" (plan-writer performs no docket metadata operations; its contract states the boundary normatively, satisfying the same clause).

(b) Composition paragraph. After the sentence naming the step-0/step-6 dispatches, insert:

```markdown
`docket-implement-next` also dispatches the `docket-plan-writer` subagent (step 4) — foreground and
unconditional on the same blocking terms, but a hybrid contract: its receipt (`PLAN_PATH=<path>`)
flows back in-context, while the proof is **git state on `feat/<slug>`** (the committed plan +
backlink + `Docket-Plan-Path:` trailer), which the parent verifies before attaching; unavailable
dispatch is Tier C, per *Dispatch-capability resolution*.
```

And update the tally: "Four of the **sixteen** generated wrappers wrap **no skill**" → "**Five** of the **seventeen** generated wrappers wrap **no skill**" adding `docket-plan-writer` to the list (it loads no convention either — it invokes the resolved plan skill at runtime as a passthrough, never as a preload, because `skills.plan` may name any installed skill). Update the closing parenthetical: "(Seventeen wrappers: five wrap the five autonomous skills, four share the `docket-build-task` contract, three share the `docket-review` contract, five wrap none.)"

(c) Dispatch-capability Tier C row. In the table's Tier C **Dispatch** cell, change "the `build` and `review` role skills, plus the in-branch fix workers" → "the **plan-writer dispatch**, the `build` and `review` role skills, plus the in-branch fix workers"; in the Posture cell the existing authorized-or-halt wording already covers it (`SKILL_PLAN=auto` is the plan role's explicitly configured `auto`).

- [ ] **Step 3: Append the convention asserts**

Append to `tests/test_plan_writer_step4.sh` (before `exit "$fail"`):

```bash
CV="$REPO/skills/docket-convention/SKILL.md"
C="$(tr '\n' ' ' < "$CV" | tr -s ' ')"
assert "convention: step-4 plan-writer composition dispatch is named" \
  'grep -q "docket-plan-writer.*(step 4)\|dispatches the .docket-plan-writer. subagent (step 4)" <<<"$C"'
assert "convention: plan-writer is in the Tier C dispatch cell" \
  'grep -qi "plan-writer dispatch.*build.*review" <<<"$C"'
assert "convention: seventeen wrappers, five wrap none" \
  'grep -qi "seventeen" <<<"$C" && grep -qiE "five.{0,40}wrap (no skill|none)" <<<"$C"'
```

- [ ] **Step 4: Run and mutation-test**

Run: `bash tests/test_plan_writer_step4.sh && bash tests/test_convention_extraction.sh && bash tests/test_composition_wiring.sh && bash tests/test_dispatch_capability.sh` — Expected: pass (after Step-1 dependents are fixed). Mutation: revert the Tier C cell edit → the Tier C assert reddens; restore.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_plan_writer_step4.sh
git commit -m "feat(0324): convention — plan-writer composition, Tier C, wrapper cardinalities"
```

(Plus any Step-1 dependent test files, by explicit path.)

---

### Task 5: Resume/attribution/external-gate fixtures

**Files:**
- Modify: `tests/test_verify_run.sh` (one fixture appended) — or a new sibling if that file's budget row is near its ceiling (check `tests/runtime-budgets.tsv` first; budget-headroom learning)
- Modify: `tests/test_plan_writer_step4.sh` (resume asserts already landed in Task 3; this task adds nothing there)

**Interfaces:**
- Consumes: `verify-run`'s existing report-line vocabulary (`run-incomplete` etc.) — read `scripts/verify-run.md` (or the script) for the exact conjunct wording before asserting.

- [ ] **Step 1: Read the existing verify-run test's fixture helpers**

`tests/test_verify_run.sh` already builds changes in various states. Identify its helper for minting an `in-progress` change with a branch, and how it asserts a report line.

- [ ] **Step 2: Write the failing external-gate fixture**

Append a case: an `in-progress` change whose feature branch carries a committed plan file (any file, plus a commit with the `Docket-Plan-Path:` trailer for realism), no `pr:` set, no `## Run halted` section. Assert `verify-run <id>` reports `run-incomplete` — the external oracle that a parent stopped after planning is re-dispatched by the caller gate. Use the file's own helper idioms; the assert keys on the report line, never the exit code.

Note: if the existing suite already has an equivalent "in-progress, no PR → run-incomplete" case, this fixture's *distinct* value is the committed-plan variant (proving a plan commit does not flip the verdict); name it that way in the assert text. `verify-run` itself needs **no code change** — if the fixture fails, that is a finding to investigate, not a license to edit the script.

- [ ] **Step 3: Run to verify it passes for the right reason**

Run: `bash tests/test_verify_run.sh` — Expected: pass. Mutation: point the fixture's change at a state that should NOT be incomplete (set `pr:` and `status: implemented` with a merged-shape) and confirm the assert would fail — i.e. prove the assert reads the verdict, not a constant. Restore.

- [ ] **Step 4: Commit**

```bash
git add tests/test_verify_run.sh
git commit -m "test(0324): external gate classifies a plan-only stop as run-incomplete"
```

---

### Task 6: Docs surface — README, `.docket.example.yml`

**Files:**
- Modify: `README.md` (agent tables / tuning examples)
- Modify: `.docket.example.yml` (commented `agents:` example gains a `plan-writer` line)

**Interfaces:**
- Consumes: cardinalities and names from Tasks 1/4.

- [ ] **Step 1: Locate the surfaces**

```bash
grep -n -e "integration-repair" -e "rebase-resolver" README.md .docket.example.yml
```

Every table/example listing the agent roster or per-agent tuning is a surface; `plan-writer` joins each with one line. Also grep README for any "sixteen"/wrapper-count prose (Task 4 Step 1's grep already surfaced these).

- [ ] **Step 2: Edit**

README: add `docket-plan-writer` to the agent/wrapper table(s) with a one-line description written for a user deciding whether to tune it (config-knob-ship-end-to-end learning — lead with the payoff): e.g. "Internal Step-4 plan author for `docket-implement-next` — tune `agents.<harness>.plan-writer` to give plan writing a stronger (or cheaper) model than orchestration; `skills.plan` still selects the *method*." `.docket.example.yml`: add a commented `#   plan-writer:  { model: ..., effort: ... }` line inside the commented `agents:` block, alongside its siblings.

- [ ] **Step 3: Run the docs guards**

Run: `bash tests/test_docket_example_yml.sh && bash tests/test_readme_skill_catalog.sh && bash tests/test_sync_agents_drift_docs.sh` — Expected: pass; fix any roster-derived guard that now counts the new agent.

- [ ] **Step 4: Commit**

```bash
git add README.md .docket.example.yml
git commit -m "docs(0324): plan-writer in README agent tables and example config"
```

---

### Task 6A: Go built-in registry reconciliation (17th shipped agent → v0.9.3)

Registering `docket-plan-writer` in `agents/harness-defaults.yml` (Task 1) breaks the Go parity
oracle change 0305 established. This task moves the four coupled Go sites so `go test ./...` is green,
under the human decisions recorded in the change file's `## Halt resolution` (fold into 0324; release
version `0.9.3`; sparse fixture tree). The count-guard sweep and asset regen a prior BLOCKED worker
left uncommitted in this worktree were **not** adopted — re-derive everything here under this task's
own authorship; do not resurrect uncommitted files.

**Files:**
- Modify: `internal/config/defaults.go` (`builtinAgents()` — add a `docket-plan-writer` row per harness).
- Modify: `internal/config/defaults_test.go` (`sidecarPath` const → `v0.9.3`).
- Create: `testdata/repositories/v0.9.3/agents-harness-defaults.yml` (byte copy of the current live
  17-agent `agents/harness-defaults.yml`) and `testdata/repositories/v0.9.3/PROVENANCE.md` (tree-wide,
  per `testdata/README.md`: source repo, commit, date, redaction=none; note the tree is sparse and
  extends nothing on `v0.9.2/`).
- Modify: `TestBuiltinAgentsShape`'s canonical-name expectation (16 → 17, `docket-plan-writer`).
- Refreeze: goldens in `internal/harness/{claude,codex,cursor,opencode}` via `go test -update`.
- Sweep: any OTHER site whose assertion hardcodes the agent count/name set — **derive from a
  whole-repo grep, never a hand-list** (CLAUDE.md rule): `grep -rn` for the count literal and the
  canonical-name list across `*.go` and `tests/*.sh`, then move only the executable assertions.

**Interfaces:**
- Consumes: the 17-agent live `agents/harness-defaults.yml` from Task 1 (the byte source of truth).
- The frozen `v0.9.2/` tree stays immutable; only `sidecarPath` moves. Every non-agent-defaults
  frozen reader stays on `v0.9.2/` — do not touch them.

- [ ] **Step 1: Watch it red first (TDD baseline)**

Run: `go test ./internal/config/ -run 'BuiltinAgents' -count=1` — Expected: RED
(`TestBuiltinAgentsParityWithFrozenSidecar` + `TestBuiltinAgentsShape` fail: live=17 vs frozen=16 /
built-ins=16). This failing state IS the test that drives the task; do not add a new bespoke test.

- [ ] **Step 2: Cut the sparse v0.9.3 fixture tree**

Byte-copy the current live sidecar to `testdata/repositories/v0.9.3/agents-harness-defaults.yml`
(`cp` — assert byte-equality after) and author `testdata/repositories/v0.9.3/PROVENANCE.md`. Re-point
`sidecarPath` in `defaults_test.go` to the `v0.9.3` path. Do NOT edit anything under `v0.9.2/`.

- [ ] **Step 3: Add the built-in row + bump the shape count**

Add the `docket-plan-writer` row (model + effort, all four harnesses) to `builtinAgents()` byte-parity
with the live sidecar; move `TestBuiltinAgentsShape` 16 → 17. Then the grep-derived count/name sweep.

- [ ] **Step 4: Refreeze harness goldens**

Run: `go test ./internal/harness/... -update -count=1`, then `go test ./internal/harness/... -count=1`
— Expected: green. Inspect the golden diff: it must contain ONLY `plan-writer` wrapper additions.

- [ ] **Step 5: Verify the reconciliation is green (cache-defeated)**

Run: `go test ./... -count=1` — Expected: green. (`-count=1` defeats the Go result cache.)

- [ ] **Step 6: Commit**

```bash
git add internal/config/ testdata/repositories/v0.9.3/ internal/harness/
# plus any grep-derived sweep sites, added by explicit path
git commit -m "feat(0324): reconcile Go built-in registry for 17th agent; cut v0.9.3 frozen sidecar"
```

---

### Task 7: Embedded asset snapshot regeneration

**Files:**
- Modify (generated): `internal/assets/embedded/tree/...`, `internal/assets/embedded/manifest.json`, possibly `internal/assets/embedded.go`

**Interfaces:**
- Consumes: every authored file from Tasks 1–6 that the asset bundle mirrors (`agents/`, `skills/`, `.docket.example.yml`, …).

- [ ] **Step 1: Regenerate**

Run: `go generate ./internal/assets/` (equivalently `go run ./cmd/genassets` — the drift test's own remedy line). Commit only mechanical outputs.

- [ ] **Step 2: Verify drift is closed (cache-defeated)**

Run: `bash tests/test_asset_bundle_drift.sh` and `go test ./... -count=1` — Expected: pass (`-count=1` defeats the Go result cache; cached-runner learning).

- [ ] **Step 3: Commit**

```bash
git add internal/assets/
git commit -m "chore(0324): regenerate embedded asset snapshot for plan-writer"
```

---

### Task 8: Whole-suite gate

**Files:** none new.

- [ ] **Step 1: Run the resolved suite**

Run: `bash scripts/run-tests.sh` (the `finalize.test_command` resolution — read it from the Step-0 export, never a second copy).
Expected: green. Treat any trailing `OVER BUDGET:` line as a finding: re-measure the offending file and adjust its budget row (or the test) deliberately, in its own commit.

- [ ] **Step 2: Fix-forward anything red**

Any failure traces to Tasks 1–7's edits; fix in the owning task's file set, mutation-test any changed guard, commit per fix with explicit paths.

---

## Results-file obligations (for Step 6.5, recorded here so the build carries them)

The results file IS warranted for this change (criterion (a) + (b)):

1. **Human verification — outside-truth model IDs:** `cursor-grok-4.5-xhigh` and `openrouter/deepseek/deepseek-v4-pro-0813` have no prior occurrence in repo history; no in-repo test can certify they are real, currently-served IDs. Certify by one dispatched run per harness (or vendor docs check); a wrong ID typically falls back to a house default silently.
2. **Human verification — start-time-loaded artifact:** generated wrappers are loaded at harness process start; the session that generates them cannot runtime-validate the pin. Name the restart precondition; the hermetic generator tests are what the run can honestly claim.
3. The expected ADR (spec §Expected ADR) — recorded at review time via the docket-adr dispatch.

## Self-review notes

- Spec coverage: internal agent (T1), parent/child boundary + payload (T3), contract + protocol (T1/T3), verification + no-allowlist (T3), continuation/resume (T3/T5), failure posture Tier C (T3/T4), defaults table (T1), docs (T4/T6), guards/tests (T1–T5), Go registry reconciliation for the 17th agent + v0.9.3 frozen tree (T6A), assets (T7), suite (T8). Success criteria all land: independent pins (T1/T2), custom-location support (no allowlist, T3), no-trust verification (T3), Tier C (T3/T4), external `run-incomplete` classification (T5), current generated outputs (T7).
- The exact grep patterns in Tasks 3–4's sentinel snippets are anchored to prose this plan also writes; if an implementer adjusts wording, adjust pattern and prose **together** and re-run the mutation checks — the mutation step, not the green run, is the proof.
- Type/name consistency: `plan-writer` (short key), `docket-plan-writer` (agent/file/wrapper name), `PLAN_PATH=<repo-relative-path>`, `Docket-Plan-Path:` (trailer), `worktree-scope: feature` — used identically in every task.
