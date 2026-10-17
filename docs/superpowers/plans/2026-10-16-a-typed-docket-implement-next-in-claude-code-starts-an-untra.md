<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0556 — A typed /docket-implement-next in Claude Code starts an untracked run](../../changes/archive/2026-10-17-0556-a-typed-docket-implement-next-in-claude-code-starts-an-untra.md)**
<!-- docket:backlink:end -->

# Tracked ways to start implement-next: Implementation Plan

> **For agentic workers:** executed by `docket-build`, task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The maintained docs name the tracked ways to start implement-next (a plain request in the session, or `/loop`). They also say plainly that a typed `/docket-implement-next` in Claude Code runs untracked and what that costs.

**Architecture:** This is a docs-only change. It edits six maintained Markdown pages and adds one prose-contract row to the existing `proseContracts` table in `internal/repoguard/prose_contracts_test.go`. No skill, agent, cursor-rule, harness, run-tracker, or claim code changes.

**Tech Stack:** Markdown; Go `testing` (the repoguard prose-contract table).

**Spec:** `docs/superpowers/specs/2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra-design.md` (on the `docket` branch)

## Global Constraints

- Do not edit anything under `skills/**`, `agents/**`, `cursor-rules/**`, or `internal/assets/**`, and do not touch harness adapters, run-tracker code, or claim code. Write no new ADR and do not edit ADR-0024, ADR-0026, or ADR-0084.
- The skill frontmatter (`context: fork`, `agent: docket-implement-next`) stays unchanged.
- Docs describe current behavior only. No change numbers, PR references, or "previously/now" wording. ADR links are allowed.
- Do not mention or document the `@docket-implement-next` agent mention.
- Keep these sentinel-pinned phrases verbatim: `completed (forked execution)` and `Fork-exclusion principle` in `docs/install/models-and-effort.md`, and `The right model for each step.` in `README.md`. Leave the two existing rows of the *two invocation paths* table in `models-and-effort.md` intact.
- **Canonical sentence.** Every page edited in this change carries this sentence verbatim, **on a single source line** (prose contracts match with `strings.Contains`, so a line wrap inside it reddens the guard):
  `A typed \`/docket-implement-next\` runs untracked.`
  Surrounding wording and placement are the builder's choice, within each task's direction.
- What an untracked run costs, stated in plain words wherever the cost is explained: it still builds to a PR, but if it stops partway there is no automatic retry, resume, or cancel. Its outcome can only be read unattributed (`docket run verdict --unattributed`).
- The tracked ways to start, named the same way everywhere: a plain request in the session (for example "implement 412"), or `/loop docket-implement-next`.

## Review Focus

- **A grep-found page the plan missed.** A maintained page outside the six named here that still presents a typed `/docket-implement-next` or a skill-invoke of it as the way to start. Expected: none remain. Task 2 Step 6 runs the whole-repository grep.
- **The canonical sentence wrapped across two lines** by a reflow. Expected: the guard reddens. Task 1 Step 2 and Task 2 Step 2 prove each `present` check is live before the docs are edited.
- **A misleading read of the exception.** The models-and-effort exception could be read as "implement-next costs a different model or effort when typed". Expected: the page still says both paths run the same pinned model and effort. The only differences are observability and, for implement-next, tracking.
- **Change-number leakage.** A "(change 0556)" or "since 0345" crept into a page. Expected: none. Task 2 Step 6 greps for it.
- **Sentinel-pinned phrases broken by an edit.** Expected: `TestProseContracts` stays green on the pre-existing rows. The full suite in Task 2 Step 7 covers this.

---

### Task 1: Entry-point pages stop steering to the typed command

**Files:**
- Modify: `internal/repoguard/prose_contracts_test.go`. Append rows to the end of the `proseContracts` slice, after the `change_0539_drift_family` row.
- Modify: `README.md`, the quick-start "The five steps" list, item 3.
- Modify: `docs/install/models-and-effort.md`, the **The two invocation paths** section that closes with "**skill-invoke for everything else**."
- Modify: `docs/reference/glossary.md`, sections `### Implement-next / the drainer` and `### Two invocation paths: skill-invoke vs agent-dispatch`.

**Interfaces:**
- Produces: sentinel id `change_0556_tracked_start`. Task 2 adds more rows under the same id.

- [ ] **Step 1: Add the failing prose-contract rows**

Append to `proseContracts`:

```go
	// Typed /docket-implement-next skips run.start: docs name the tracked starts.
	{sentinel: "change_0556_tracked_start", file: "README.md",
		present: []string{"A typed `/docket-implement-next` runs untracked."},
		absent:  []string{"`docket-implement-next` (or `/loop` it hands-free)"}},
	{sentinel: "change_0556_tracked_start", file: "docs/install/models-and-effort.md",
		present: []string{"A typed `/docket-implement-next` runs untracked."}},
	{sentinel: "change_0556_tracked_start", file: "docs/reference/glossary.md",
		present: []string{"A typed `/docket-implement-next` runs untracked."},
		absent:  []string{"/docket-implement-next 412\n"}},
```

- [ ] **Step 2: Run it and confirm it fails for the right reasons**

Run: `go test ./internal/repoguard -run TestProseContracts -count=1`
Expected: FAIL. There are three `required contract phrase is missing` violations (README, models-and-effort, glossary) and two `retired/forbidden phrase is present` violations (README, glossary). Any other violation means a row is wrong. Fix it before going on.

- [ ] **Step 3: Edit `README.md`**

Rewrite item 3 so it no longer presents the bare command as the way in. It must name a plain request and `/loop`, and it must carry the canonical sentence on one line. For example:

```markdown
3. **Drain** the backlog to open PRs: ask in plain words ("implement the next change"), or
   `/loop docket-implement-next` hands-free. A typed `/docket-implement-next` runs untracked.
```

Optionally link the run-tracker page (`docs/concepts/run-tracker.md`). Keep `The right model for each step.` untouched.

- [ ] **Step 4: Edit `docs/install/models-and-effort.md`**

Directly after the closing "Reach for **agent-dispatch when you want to watch a long run** … **skill-invoke for everything else**." sentence, add a short paragraph with the implement-next exception. Content:
- A skill-invoke of `docket-implement-next` (typed, or the model invoking the skill) goes straight to the implementer. The session never runs `run.start`, so the run tracker does not track it. Include the canonical sentence on one line.
- So start implement-next with a plain request in the session or with `/loop`.
- State the cost (Global Constraints) and link `../concepts/run-tracker.md`.

Do not alter the table rows or the existing sentence.

- [ ] **Step 5: Edit `docs/reference/glossary.md`**

- In `### Implement-next / the drainer`, replace the code block's `/docket-implement-next 412` line with a plain-request example such as `implement 412`. Keep the `/loop docket-implement-next 412 413 414` line. In the **Used for:** paragraph, add the canonical sentence on one line, with the plain-request or `/loop` guidance.
- In `### Two invocation paths: skill-invoke vs agent-dispatch`, after the **Used for:** sentence, add one sentence: implement-next is the exception, because its skill-invoke is untracked, so start it with a plain request or `/loop`. That sentence is not the canonical one. The canonical sentence already appears once in this file.

- [ ] **Step 6: Run the test and confirm it passes**

Run: `go test ./internal/repoguard -run TestProseContracts -count=1`
Expected: PASS.

- [ ] **Step 7: Mutation-check the new rows**

Copy each edited doc to a backup with `cp` (never restore with `git checkout --`, which would discard the uncommitted edit). Do the following one file at a time, then restore from the backup:
- Delete the canonical sentence from the file and run the Step 6 command. Expected: FAIL naming that file.
- For the README and glossary, restore the old line (`docket-implement-next` (or `/loop` it hands-free) / `/docket-implement-next 412`) and run the Step 6 command. Expected: FAIL naming that file.

After restoring, run the Step 6 command again. Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/repoguard/prose_contracts_test.go README.md docs/install/models-and-effort.md docs/reference/glossary.md
git commit -m "docs: name the tracked ways to start implement-next on the entry pages"
```

### Task 2: Explanation pages say why a typed start is untracked

**Files:**
- Modify: `internal/repoguard/prose_contracts_test.go`. Append two rows after Task 1's rows.
- Modify: `docs/guide/building-without-supervision.md`, the paragraph and list that recommend `/loop docket-implement-next`, ending with the link to *The run tracker and attribution*.
- Modify: `docs/concepts/run-tracker.md`, `## The moving parts`, the **Starting** bullet.

**Interfaces:**
- Consumes: sentinel id `change_0556_tracked_start` and the canonical sentence from Task 1.

- [ ] **Step 1: Add the failing rows**

```go
	{sentinel: "change_0556_tracked_start", file: "docs/guide/building-without-supervision.md",
		present: []string{"A typed `/docket-implement-next` runs untracked."}},
	{sentinel: "change_0556_tracked_start", file: "docs/concepts/run-tracker.md",
		present: []string{"A typed `/docket-implement-next` runs untracked."}},
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/repoguard -run TestProseContracts -count=1`
Expected: FAIL. There are exactly two `required contract phrase is missing` violations, one for each of these files.

- [ ] **Step 3: Edit `docs/guide/building-without-supervision.md`**

In the `/loop` section, near the existing run-tracker link, say:
- Start implement-next with a plain request in the session or with `/loop`. The run tracker tracks both.
- In Claude Code, a typed `/docket-implement-next` goes straight to the implementer, because the skill runs forked. Include the canonical sentence on one line.
- State the cost (Global Constraints).

- [ ] **Step 4: Edit `docs/concepts/run-tracker.md`**

Extend the **Starting** bullet, or add a short paragraph right after it inside *The moving parts*. Say:
- A tracked start needs the session to take a turn first, so that it can run `docket run start implement-next` and put the run context in the dispatch prompt.
- In Claude Code, a typed `/docket-implement-next` skips that turn and goes straight to the implementer. Include the canonical sentence on one line.
- That run has no key, so it can only be read with `docket run verdict --unattributed`, which never authorizes a re-dispatch.

ADR-0084 may be cited by link. It is already the page's style to cite ADRs in *Decided in*. Add an ADR-0084 entry there only if the section does not already list it.

- [ ] **Step 5: Run the test and mutation-check it**

Run: `go test ./internal/repoguard -run TestProseContracts -count=1`
Expected: PASS.

Then, using a `cp` backup, delete the canonical sentence from each of the two files in turn and re-run. Expected: FAIL naming that file. Restore and re-run. Expected: PASS.

- [ ] **Step 6: Grep for missed pages and leakage**

Run from the worktree root:

```bash
md=$(git ls-files '*.md' | grep -v -E -e '^(skills|agents|cursor-rules|internal|testdata|docs/adrs|docs/changes|docs/superpowers|docs/results|docs/release|docs/codex)/')
printf '%s\n' "$md" | xargs grep -n -E -e '/docket-implement-next|skill-invoke'
grep -n -E -e '(change|#) ?0?(556|345|541)([^0-9]|$)' README.md docs/install/models-and-effort.md docs/reference/glossary.md docs/guide/building-without-supervision.md docs/concepts/run-tracker.md
```

Expected for the first grep: every `/docket-implement-next` hit is either the canonical sentence, a `/loop docket-implement-next` line, or a sentence explaining the untracked typed path. Every `skill-invoke` hit either is about `docket-status`-style skills or carries the implement-next exception. Expected for the second grep: no hits.

If the first grep turns up another maintained page that presents the typed command or a skill-invoke of implement-next as the way to start, give it the same treatment: tracked starts, the canonical sentence, the cost. Add a `change_0556_tracked_start` present row for it and mutation-check that row. If a hit is inside the embedded asset set (`skills/`, `agents/`, `cursor-rules/`), do not edit it. Record it as a follow-up in the results file instead.

- [ ] **Step 7: Run the full suite**

Run: `go run ./cmd/docket development test`
Expected: green. Read the budget report even on green. Treat any `SERIAL CONFIRMED OVER BUDGET:` line as a finding.

- [ ] **Step 8: Commit**

```bash
git add internal/repoguard/prose_contracts_test.go docs/guide/building-without-supervision.md docs/concepts/run-tracker.md
git commit -m "docs: explain why a typed implement-next start is untracked"
```

Add any grep-found page to this commit's `git add`.
