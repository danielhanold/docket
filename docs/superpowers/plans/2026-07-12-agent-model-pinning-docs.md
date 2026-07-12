# Agent Model Pinning Docs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Document docket's two invocation paths into a pinned wrapper (skill-invoke vs agent-dispatch) and teach per-agent model pinning as a first-class idea, guarded by doc sentinels — plus (off-branch) an ADR recording the `context: fork` findings.

**Architecture:** Pure documentation. Three prose surfaces at three altitudes, no duplication: **teaching** in `README.md` (two bold-lead paragraph blocks inside the existing `## Tuning agent models & effort`, plus one *What you get* bullet), **mechanics** in `skills/docket-convention/references/agent-layer.md` (~10 lines), and **the decision** in ADR-0026 — which is authored on the *metadata branch* by the `docket-adr` subagent, **not on this feature branch** (see *Out-of-branch work* at the end). Each prose surface is pinned by a positive-anchor sentinel in the suite that already owns the fork subject matter, `tests/test_skill_fork_dispatch.sh`.

**Tech Stack:** Markdown; bash + `grep` sentinels (`tests/test_*.sh`, `assert`-style harness, no framework).

## Global Constraints

Copied verbatim from the spec and from `docs/changes/LEARNINGS.md` — every task's requirements implicitly include these.

- **No new `###` headings in `## Tuning agent models & effort`.** That section (README L385–433) today contains **zero** `###` subsections — it is a flat run of bold-lead paragraph blocks. A heading inserted mid-section would silently swallow every paragraph below it (including the unrelated *clone-identical guarantee* closer) into the new subsection. **Match the `**bold-lead**` paragraph idiom instead; the section stays flat.**
- **Teaching prose names tiers as *cheap / mid / top* — never literal model ids** (spec A7). Literal tiers live in `.docket.yml`; restating them in prose is how docs drift from an override. (The ADR is exempt — it records a dated observation, not current config.)
- **The on-disk transcript path is documented as an observed Claude Code internal, NOT an interface** (spec A4) — version-stamped (2.1.207), flagged as liable to move. No code may depend on it.
- **This branch touches NO docket metadata** — no change file, no `BOARD.md`, no `docs/adrs/`. Code + plan only. (The ADR and the `adrs:` field are metadata-branch work; see *Out-of-branch work*.)
- **Doc sentinels use a POSITIVE anchor on the meaningful framing** (LEARNINGS #36/#37) — never a blunt `! grep` or a `grep -q` over a literal that can legitimately appear elsewhere in the same doc.
- **One assert owns exactly ONE clause** (LEARNINGS #21). If a pattern could be satisfied from two independent locations, split it. **Mutation-test each assert in isolation** — delete its clause, watch that assert (and only that assert) go red.
- **The full suite is the gate, not the enumerated sentinels** (LEARNINGS #54/#52). Final task runs every `tests/test_*.sh`.
- **README's TOC needs no edit** — no headings are added or renamed.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `README.md` | The repo's single prose home (teaching altitude) | Two bold-lead blocks inside `## Tuning agent models & effort`; one bullet in *What you get* |
| `skills/docket-convention/references/agent-layer.md` | Canonical agent-layer reference (mechanics altitude) | One bold-lead paragraph in `## Always-full-set generation + the Cursor dispatch rule` |
| `tests/test_skill_fork_dispatch.sh` | Owns the fork-dispatch invariant + (now) its doc sentinels | Append a change-0065 sentinel block |
| `docs/superpowers/plans/2026-07-12-agent-model-pinning-docs.md` | This plan | Created |

Task order is **test-first per surface**: each task writes its sentinel, watches it fail, writes the prose, watches it pass, mutation-tests it, commits. Tasks 1–3 are independently reviewable and independently revertible; Task 4 is the whole-suite gate.

---

### Task 1: README — the two invocation paths

The core deliverable: the fact whose absence cost a user an afternoon.

**Files:**
- Modify: `README.md` — insert **between** the `**Two mechanisms for one inline quirk.**` paragraph and the `**The clone-identical guarantee is retired.**` paragraph (currently adjacent, at the end of `## Tuning agent models & effort`)
- Test: `tests/test_skill_fork_dispatch.sh`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: the `README`/`AGENT_LAYER` shell variables and the `# --- change 0065: doc sentinels ---` block header that Tasks 2 and 3 append their asserts to. Exact names: `README="$REPO/README.md"`, `AGENT_LAYER="$REPO/skills/docket-convention/references/agent-layer.md"`.

- [ ] **Step 1: Write the failing sentinels**

Append to `tests/test_skill_fork_dispatch.sh`, immediately **before** the closing `if [ "$fail" = 0 ]; then echo "PASS"; ...` block (that summary block must stay last):

```bash
# --- change 0065: doc sentinels -----------------------------------------------------------------
# Positive anchors on the MEANINGFUL FRAMING of the invocation-path / model-pinning docs, not on
# incidental wording (LEARNINGS #36/#37). Each assert owns exactly ONE clause in ONE file, so it can
# be mutation-tested in isolation (LEARNINGS #21) and the prose stays freely rewritable.
README="$REPO/README.md"
AGENT_LAYER="$REPO/skills/docket-convention/references/agent-layer.md"

assert "README names both invocation paths into the pinned wrapper" \
  'grep -qi "skill-invoke" "$README" && grep -qi "agent-dispatch" "$README"'
assert "README contrasts them by observability (forked run opaque, dispatch drillable)" \
  'grep -qiF "completed (forked execution)" "$README" && grep -qi "drillable" "$README"'
assert "README names the fork transcript path as the escape hatch" \
  'grep -qF "subagents/agent-" "$README"'
assert "README carries the process-start registration caveat" \
  'grep -qiE "register(ed)? at .{0,4}process start" "$README" && grep -qi "restart" "$README"'
```

- [ ] **Step 2: Run the suite to verify the new asserts fail**

Run: `bash tests/test_skill_fork_dispatch.sh`

Expected: the four pre-existing fork-invariant assertions still print `ok - …`; the four new ones print `NOT OK - …`, and the file ends `FAIL` (exit 1). If any new assert is already green, the anchor is vacuous — it is matching prose that already exists. Fix the anchor before writing a word of README.

- [ ] **Step 3: Write the README block**

In `README.md`, insert this **between** the `**Two mechanisms for one inline quirk.** …` paragraph and the `**The clone-identical guarantee is retired.** …` paragraph. Blank line above and below; **no `###` heading**.

```markdown
**The two invocation paths.** Both mechanisms above land a directly-invoked skill on the *same* pinned wrapper, so the model and effort it runs at are identical either way. What differs is what **you** see while it runs:

| Path | How | You get | You give up |
|---|---|---|---|
| **Skill-invoke** | `/docket-status`, or the model auto-invoking the skill | The pinned run, forked — cheapest, no dispatch turn | Observability: it returns as `completed (forked execution)`, with no box to drill into in the TUI |
| **Agent-dispatch** | `@docket-status`, or a `Task` dispatch naming the wrapper | The **identical** pinned run, drillable live in the TUI | One dispatch turn of overhead |

Reach for **agent-dispatch when you want to watch a long run** — a build you intend to babysit — and **skill-invoke for everything else**. A forked run is not lost, only unobservable in the TUI: Claude Code still writes its full transcript to `~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl`. Treat that path as an **observed internal, not an interface** — it was accurate on Claude Code 2.1.207, it may move, and docket depends on it for nothing. Cursor users are always on the drillable path: the generated dispatch rule routes a direct invocation through a real `Task` dispatch.

**Restart your session after changing an agent or a skill.** Skills and agents are **registered at process start**. After you run `sync-agents.sh`, or edit a skill's frontmatter, an already-open session keeps running the *old* definitions — so a freshly-added fork appears to do nothing, and a healthy pin looks broken. Restart the harness process (a new session — clearing the context is not enough) and re-invoke.
```

- [ ] **Step 4: Run the suite to verify the new asserts pass**

Run: `bash tests/test_skill_fork_dispatch.sh`
Expected: every assert prints `ok - …`; the file ends `PASS` (exit 0).

- [ ] **Step 5: Mutation-test each new assert in isolation**

For each of the four asserts, delete only the clause it owns from `README.md`, re-run `bash tests/test_skill_fork_dispatch.sh`, confirm **that assert alone** flips to `NOT OK`, then restore the clause (`git checkout -- README.md`). Concretely:

| Assert | Delete | Expect red |
|---|---|---|
| names both paths | the table's two `\| **Skill-invoke** \|` / `\| **Agent-dispatch** \|` rows | that assert only |
| contrasts by observability | the `completed (forked execution)` cell text | that assert only |
| names the transcript path | the `~/.claude/projects/…/subagents/agent-<id>.jsonl` sentence | that assert only |
| process-start caveat | the whole `**Restart your session…**` paragraph | that assert only |

A deletion that reddens **two** asserts means they share a clause — split or re-anchor them. A deletion that reddens **none** means the anchor is satisfied from somewhere else in the README — re-anchor it.

- [ ] **Step 6: Verify no heading was introduced and the TOC still matches**

Run:
```bash
awk '/^## Tuning agent models & effort$/,/^## The eight skills$/' README.md | grep -c '^### ' 
```
Expected: `0`. (Any other number means a `###` slipped in and is now swallowing the paragraphs below it.)

- [ ] **Step 7: Commit**

```bash
git add README.md tests/test_skill_fork_dispatch.sh
git commit -m "docs(0065): README — the two invocation paths + restart caveat, with sentinels"
```

---

### Task 2: README — why pin a model per agent

**Files:**
- Modify: `README.md` — a block between the `## Tuning agent models & effort` heading and its existing lead paragraph (`Each **autonomous** docket skill runs as a model/effort-pinned subagent…`); plus one bullet in the *What you get* list near the top of the file
- Test: `tests/test_skill_fork_dispatch.sh`

**Interfaces:**
- Consumes: the `README` variable and the `# --- change 0065: doc sentinels ---` block from Task 1.
- Produces: nothing for later tasks.

- [ ] **Step 1: Write the failing sentinels**

Append inside the change-0065 sentinel block created in Task 1 (still above the closing `if [ "$fail" = 0 ]` summary):

```bash
assert "README teaches model-per-task over model-per-session" \
  'grep -qiE "one session, one model" "$README" && grep -qi "cheap tier" "$README"'
assert "README's What you get list surfaces per-agent model pinning" \
  'grep -qF "**The right model for each step.**" "$README"'
```

- [ ] **Step 2: Run the suite to verify both fail**

Run: `bash tests/test_skill_fork_dispatch.sh`
Expected: the two new asserts print `NOT OK - …`; Task 1's asserts stay `ok`; the file ends `FAIL`.

- [ ] **Step 3: Write the motivation block**

In `README.md`, insert **between** the `## Tuning agent models & effort` heading line and the existing `Each **autonomous** docket skill runs as a model/effort-pinned subagent …` lead paragraph. **No `###` heading** — a heading here would nest the entire 1/2/3 how-to under "motivation".

```markdown
**Why pin a model per agent.** Most harnesses invite one mental model: *one session, one model.* You choose a tier when you start, and everything you do that hour runs at it. That is how you end up paying top-tier prices to regenerate a board — and thinking at the cheap tier while designing a build. Both are the same mistake, in opposite directions: the model was matched to the **session** instead of to the **task**.

docket's unit of work is the **skill**, so the tier is a property of the skill, not of your session. A `docket-status` sweep is mechanical file bookkeeping — cheap tier, low effort. A `docket-implement-next` build is the deepest reasoning in the loop — top tier, high effort. A design pass sits between them. They run in the **same session**, minutes apart, each at its own model, and you never pick one.

A single afternoon's loop spans all three: groom a stub at the mid tier, build it at the top tier, sweep the merged PR at the cheap tier. The `agents:` block below is how you *express* that; the generated wrapper is how it is *enforced*; and `context: fork` (Claude Code) and the generated dispatch rule (Cursor) are how the pin survives even a direct `/docket-status` invocation. Tune the tiers to your budget — docket's built-in defaults are a starting point, not a contract.
```

- [ ] **Step 4: Add the *What you get* bullet**

In `README.md`, append this as the **last** bullet of the top-of-file `What you get:` list (immediately after the `- **No new infrastructure.** …` bullet):

```markdown
- **The right model for each step.** Every autonomous skill is pinned to its own model and effort, so a board refresh runs at a cheap tier while a build runs at a top one — in the same session, with no model choice from you. See [Tuning agent models & effort](#tuning-agent-models--effort).
```

- [ ] **Step 5: Run the suite to verify both pass**

Run: `bash tests/test_skill_fork_dispatch.sh`
Expected: every assert `ok - …`; ends `PASS`.

- [ ] **Step 6: Mutation-test both new asserts in isolation**

| Assert | Delete | Expect red |
|---|---|---|
| model-per-task teaching | the `**Why pin a model per agent.**` block (all three paragraphs) | that assert only |
| What-you-get bullet | the `- **The right model for each step.**` bullet | that assert only |

Restore with `git checkout -- README.md` after each. Note the *What you get* assert deliberately anchors on the bullet's bold lead rather than on the section link `(#tuning-agent-models--effort)` — that link **already exists in the Table of contents**, so a link-anchored assert would be green from the TOC alone and prove nothing (LEARNINGS #21, double-guarded sentinel).

- [ ] **Step 7: Verify prose names no literal model ids**

Run:
```bash
awk '/^## Tuning agent models & effort$/,/^## The eight skills$/' README.md \
  | grep -nEi 'opus|haiku|sonnet' 
```
Expected: matches **only** inside the pre-existing *Finding model IDs* table region and the `agents:`-shape prose that already mentioned them — **no match inside the new `**Why pin a model per agent.**` block**. The teaching prose must say *cheap / mid / top* (Global Constraints, spec A7). If a literal id appears in the new block, replace it with a tier word.

- [ ] **Step 8: Commit**

```bash
git add README.md tests/test_skill_fork_dispatch.sh
git commit -m "docs(0065): README — why pin a model per agent, with sentinels"
```

---

### Task 3: `references/agent-layer.md` — the mechanics propagate

The reference is a **blocking read** for anyone configuring the agent layer, and it is the exact file whose staleness change 0061's review caught. Mechanics only — the teaching stays in README, the decision stays in the ADR.

**Files:**
- Modify: `skills/docket-convention/references/agent-layer.md` — in `## Always-full-set generation + the Cursor dispatch rule`, inserted **after** the long `**Always-full-set generation, now machine-local, + the Cursor dispatch rule.**` paragraph and **before** the `Generated files are machine-local: …` closing paragraph
- Test: `tests/test_skill_fork_dispatch.sh`

**Interfaces:**
- Consumes: the `AGENT_LAYER` variable and the change-0065 sentinel block from Task 1.
- Produces: nothing for later tasks.

- [ ] **Step 1: Write the failing sentinel**

Append inside the change-0065 sentinel block (still above the closing summary):

```bash
assert "agent-layer reference states both paths land on the same pinned wrapper" \
  'grep -qiE "[Bb]oth invocation paths land on the same pinned wrapper" "$AGENT_LAYER"'
```

- [ ] **Step 2: Run the suite to verify it fails**

Run: `bash tests/test_skill_fork_dispatch.sh`
Expected: the new assert prints `NOT OK - agent-layer reference states both paths land on the same pinned wrapper`; ends `FAIL`.

- [ ] **Step 3: Write the reference paragraph**

Insert into `skills/docket-convention/references/agent-layer.md` at the position named above:

```markdown
**Both invocation paths land on the same pinned wrapper.** A forked skill-invoke (`/docket-status`)
and an explicit agent dispatch (`@docket-status`, or a `Task` naming the wrapper) resolve to the
*same* generated wrapper and run at the *same* resolved model/effort. They differ only in
**observability** — the dispatch is drillable in the TUI, while the fork returns as `completed
(forked execution)` with its transcript reachable only on disk — and in **cost**: the dispatch
spends a turn, the fork does not. Verified on Claude Code 2.1.207, together with the composition
question ADR-0024 left open: a wrapper whose `skills:` preloads the very skill that forks into it
does **not** recurse (preload is content injection at startup; the fork fires on invocation).
**Caveat: skills and agents register at process start** — after `sync-agents.sh` or a
skill-frontmatter edit, an already-open session still runs the old definitions, so restart the
harness process or a healthy fork will look broken.
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `bash tests/test_skill_fork_dispatch.sh`
Expected: every assert `ok - …`; ends `PASS`.

- [ ] **Step 5: Mutation-test the assert**

Delete the `**Both invocation paths land on the same pinned wrapper.**` paragraph from `skills/docket-convention/references/agent-layer.md`, re-run `bash tests/test_skill_fork_dispatch.sh`, confirm **that assert alone** goes red, then `git checkout -- skills/docket-convention/references/agent-layer.md`.

- [ ] **Step 6: Verify the two-mechanism story did not get contradicted**

The reference already says Claude Code fixes the inline quirk natively while Cursor uses a generated rule. The new paragraph must **extend** that, not restate or contradict it. Read the section end to end once and confirm it reads as one argument, with no duplicated sentence.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/references/agent-layer.md tests/test_skill_fork_dispatch.sh
git commit -m "docs(0065): agent-layer reference — both paths land on the same pinned wrapper"
```

---

### Task 4: Whole-suite gate

The sentinel list is a **floor**, never the gate (LEARNINGS #54/#52 — a goal-scoped change passes its own audit while an out-of-goal dimension slips). `skills/` is symlinked into the harness, and the convention reference is read by other suites, so an edit there can redden a test nobody anticipated.

**Files:**
- Modify: none (verification only)

**Interfaces:**
- Consumes: everything from Tasks 1–3.
- Produces: a green suite — the precondition for review and PR.

- [ ] **Step 1: Run every test in the suite**

Run this as **one foreground command** (the suite takes several minutes — do not background it):

```bash
cd /Users/homer/dev/docket/.worktrees/agent-model-pinning-docs && \
fails=""; for t in tests/test_*.sh; do
  if bash "$t" >/tmp/0065-$(basename "$t").log 2>&1; then
    echo "PASS  $t"
  else
    echo "FAIL  $t"; fails="$fails $t"
  fi
done; echo "---"; [ -z "$fails" ] && echo "SUITE GREEN" || { echo "RED:$fails"; exit 1; }
```

Expected: `SUITE GREEN`. Every `tests/test_*.sh` exits 0.

- [ ] **Step 2: If any suite is red, root-cause before touching it**

A red test outside `test_skill_fork_dispatch.sh` is the signal this task exists to catch. Do **not** weaken the test. Read its failure (`/tmp/0065-<name>.log`), find which of this change's edits reddened it, and fix the *edit*. Two likely candidates, both worth checking even when green:
- `tests/test_convention_extraction.sh` — asserts things about `skills/docket-convention/`, which Task 3 edited.
- `tests/test_script_contracts_coverage.sh` / `test_change_links_coverage.sh` — grep skill and doc prose for required mentions; a paragraph inserted mid-section can move a line they anchor on.

- [ ] **Step 3: Confirm the branch touched no docket metadata**

Run:
```bash
git diff --name-only origin/main...HEAD
```
Expected — exactly these four paths, and nothing else:
```
README.md
docs/superpowers/plans/2026-07-12-agent-model-pinning-docs.md
skills/docket-convention/references/agent-layer.md
tests/test_skill_fork_dispatch.sh
```
Any `docs/changes/…`, `docs/adrs/…`, or `BOARD.md` path here is a **branch-discipline violation** — the feature branch must never carry docket metadata. Remove it and re-commit.

- [ ] **Step 4: Commit (only if Step 2 required a fix)**

```bash
git add -A
git commit -m "docs(0065): fix suite regression surfaced by the whole-suite gate"
```

---

## Out-of-branch work (metadata branch — NOT part of this plan's commits)

Recorded here so the executor does **not** attempt it in the feature worktree. `docket-implement-next` performs these in its own steps, in the `.docket/` metadata working tree:

- **ADR-0026** — *accept fork opacity; two invocation paths; no tooling.* Authored at step 6 by the **`docket-adr` subagent** (it assigns the id, updates the index, and commits on `origin/docket`). `relates_to: [8, 17, 20, 24]`, `change: 65`. Its `## Context` carries the five verified findings (Claude Code 2.1.207, 2026-07-12); its `## Decision` is *accept the opacity, document two paths, add no tooling* — **not** a test report (spec A2).
- **A dated `## Update` note on ADR-0024** pointing forward to ADR-0026 (the index renders no back-links, so without it a reader of 0024 never learns its open question was closed). Decision text untouched — an `## Update` note is the convention's sanctioned move for a non-reversing context change on an `Accepted` ADR.
- **The dispatch is scoped to `origin/docket` only.** Publishing an ADR onto `main` mid-run is soft-denied by the auto-mode classifier; the ids go in the change's `adrs:` and finalize publishes them at merge.
- **`adrs: [24, 26]` on change 0065** — set in the metadata tree per the field-write rule. **Listing 24 is load-bearing, not bookkeeping:** terminal close-out publishes the `Accepted` ADRs named in `adrs:`, so 24 is the only thing that carries 0024's new `## Update` note to `main`. Do not "tidy" it out.

## Self-review

**Spec coverage.** §1 ADR-0026 + the 0024 `## Update` → *Out-of-branch work* (correctly not a branch task). §2 README two invocation paths → Task 1. §3 README why-pin-models + *What you get* bullet → Task 2. §4 `references/agent-layer.md` → Task 3. §Tests (three required sentinels: two-paths+transcript, restart caveat, agent-layer both-paths) → Tasks 1 and 3, plus two beyond-spec sentinels in Task 2 guarding §3, which would otherwise ship with no guard at all. Scope table's `docs/adrs/README.md` (regenerated index) is script-owned and handled by `docket-adr`. No gap.

**Placeholder scan.** Every step carries its literal content — the exact markdown to insert, the exact `assert` lines, the exact commands and expected output. No TBD, no "similar to Task N", no "add appropriate handling".

**Consistency.** `README` and `AGENT_LAYER` are defined once (Task 1) and reused by name in Tasks 2 and 3. The sentinel block header is introduced once and appended to. Every assert string is unique, so a `NOT OK - …` line identifies exactly one clause. Insertion anchors are named by their existing bold-lead text, not by line number, so they survive the earlier tasks' edits shifting the file.
