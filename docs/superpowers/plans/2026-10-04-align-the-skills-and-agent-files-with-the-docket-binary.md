<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0502 — Align the skills and agent files with the docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0502-align-the-skills-and-agent-files-with-the-docket-binary.md)**
<!-- docket:backlink:end -->
# Align the skills and agent files with the docket binary — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, which routes each `### Task N`
> to one tier worker running the `docket-build-task` contract. Tasks are strictly sequential: each
> builds on the previous task's commit. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every in-scope agent-facing file (`skills/**`, `agents/*.md`, `AGENTS.md`) describes only
what the docket binary does today, the four prose-caused bugs are fixed, and the living-docs guard
covers these files so the drift cannot return.

**Architecture:** Prose edits grouped by subject (one subject per task), each landing in one commit
together with every repo test that pins the edited wording, the regenerated embedded assets, and
(when an `agents/*.md` file changes) the regenerated harness goldens. Structural deletions and
renames come first, then the role/config/name/fact rewrites, then the four bugs, then three
whole-tree sweeps (metadata layout, "loop", citations), and last the guard extension, which can
only go green once the sweeps are done.

**Tech Stack:** Markdown skill bodies; Go 1.x tests in `internal/repoguard`, `internal/assets`,
`internal/harness/*`, `internal/render`, `internal/app`; `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary-design.md`
on the `docket` branch (read it through the synchronized `.docket/` worktree of the primary
checkout: `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-align-the-skills-and-agent-files-with-the-docket-binary-design.md`).
Its **Worklist** section is the authoritative list of defects; every task below names the worklist
items it owns. Treat each worklist item as a hypothesis: re-check it against the code, then fix,
delete, or (if it is not a defect) say so in the task's commit message body so the coordinator can
record it in the results file.

## Global Constraints

Every task's requirements implicitly include all of these.

- **Repo root.** Every command runs from the feature worktree root: `cd "$(git rev-parse --show-toplevel)"`.
- **In scope:** `skills/**` (all twelve skills and their references), `agents/*.md`, `AGENTS.md`
  outside its generated `docket:dispatch` block, the `internal/render/record.go` comments naming a
  deleted template, `docs/release/four-harness-acceptance.md` (receives the run-boundary probe
  scenarios only), regenerated embedded assets, regenerated harness goldens, the guards, and every
  repo test that pins wording on these files.
- **Never edit:** the human-facing docs (`README.md`, `docs/guide`, `docs/install`,
  `docs/concepts`, `docs/reference`), `.docket.example.yml`, `.docket.yml`,
  `agents/harness-defaults.yml`, `scripts/runners/`, Accepted ADRs (`docs/adrs/`), results
  (`docs/results/`), archived changes, specs, plans, `testdata/`, `internal/install/testdata/`,
  `internal/install/legacydata/`. Exception: `internal/harness/*/testdata/golden/*` are rendered
  from `agents/*.md` and MUST be regenerated (never hand-edited) whenever an `agents/*.md` file
  changes — that is the guard's own remedy.
- **Alignment rules (spec, verbatim intent):** (1) describe only current behaviour — the binary is
  the oracle (`internal/config/schema.go`, `docket capabilities --json`, `docket schema`, each
  verb's `--help`, the Go code); (2) removed or refused things never appear — no "this used to…",
  no tombstone, no "not supported" note; (3) no citations of individual changes or PRs (including
  line-wrapped ones) and no "Go v1" / "deferred from Go v1" narration — keep the rule, drop the
  citation; (4) ADR citations stay only where the ADR's decision is still how docket works
  (ADR-0018 is no longer cited by skill prose; ADR-0022 stays); (5) point-in-time records are
  untouched; (6) skill bodies ship into other repositories — a sentence true only in docket's own
  repository stays conditioned on that.
- **Terminology.** "Loop" means `/loop` only. One metadata layout: **the `docket` branch** and
  **the `.docket/` worktree** (never `metadata_branch`, "docket-mode", "repo-mode"). `auto-or-halt`
  becomes `halt`.
- **Fixed role skills:** brainstorm `superpowers:brainstorming`, plan `superpowers:writing-plans`,
  build `docket-build`, review `docket-review`, finish `superpowers:finishing-a-development-branch`.
- **Operations are named by operation id, never by argv** in skill/agent prose: write
  "the `finalize.clear-block` operation", never `docket finalize clear-block`.
  `TestCapabilitySurface` (`internal/repoguard/capability_surface_test.go`) flags any new
  `docket <verb>` spelling and pins the exact counts of four human-typed remedies
  (`docket repository migrate` 6, `docket repository init` 3, `docket repository configure-tests` 3,
  `docket change create` 5). Deleting text may lower a count: update the pin to the measured count
  in the same commit with a trailing comment `// 0502: <what was deleted>`. Never raise a count.
- **Config-file tokens.** `TestConfigReadChannel` requires every `.docket.yml` /
  `.docket.local.yml` / `config.yml` occurrence in `skills/**/*.md` — except
  `skills/docket-convention/SKILL.md` and `skills/docket-convention/references/agent-layer.md` — to
  carry a same-line `<!-- docket:config-read-channel: write-back -->` or
  `<!-- docket:config-read-channel: negative -->` marker. Prefer naming the setting
  (`build.gate`) over the file. The convention body must keep at least 3 config-token occurrences
  and must keep the literal `docket capabilities --json`.
- **Unsupported-key spellings.** Never write an unsupported key as a dotted path (`skills.build`,
  `learnings.cap`, `build.checkpoint`, `skills.<role>`), as `key:` at a line start, or as a code span
  opening with `key:` (`` `skills: [...]` ``). Describe a wrapper's frontmatter field as "the
  wrapper's skills list", never with the `skills:` spelling. The unsupported set is
  `config.SettingPaths()` rows with `Supported == false`: `metadata_branch`, `runtime.bash`,
  `finalize.skip_results_only_delta`, `learnings.cap`, `build.checkpoint`,
  `delegation_observation_budget`, `github_project`, `terminal_publish`, `auto_groom`,
  `auto_capture.*`, `dummy_mode.*`, `skills.*`, `agents.*.*.runner`, `runners.*`.
- **Budget ratchet.** `TestSkillSizeBudgets` (`internal/repoguard/budgets_test.go`, `skillBudgets`)
  holds a line/word ceiling per `skills/**/*.md` file and requires a row for every file. After
  editing a skill file, measure it (`wc -l <f>; wc -w <f>`) and set its row to the exact new counts
  with a trailing comment `// 0502: <reason> (oldL/oldW -> newL/newW)`. Delete the row of a deleted
  file; rename the key of a renamed file.
- **Every commit that edits `skills/` or `agents/`:** run `go generate ./internal/assets/` and stage
  `internal/assets/embedded/` with the authored paths.
- **Every commit that edits `agents/*.md`:** regenerate the harness goldens:
  `for p in claude codex cursor opencode; do go test ./internal/harness/$p/ -count=1 -update; done`,
  then `git diff --stat internal/harness/` must show only the goldens of the agents you edited, and
  each golden diff must contain only your edited text.
- **Focused check (run at the end of every task, all must pass):**
  ```bash
  cd "$(git rev-parse --show-toplevel)"
  go generate ./internal/assets/
  timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/harness/... ./internal/render/
  ```
  Tasks that touch `results-template.md` also run `./internal/app/ -run 'Results'`.
- **Mutation probes** (learnings `mutation-restore-needs-a-backup-copy`,
  `phrase-grep-over-wrapped-prose`, `cached-runner-serves-a-mutated-tree`): always back up with
  `cp "$f" "$f.bak"`, confirm the mutation landed with a whitespace-collapsed count
  (`tr -s '[:space:]' ' ' < "$f" | grep -o -F -- '<phrase>' | wc -l`, before and after), run the
  test with `-count=1`, restore with `mv -f "$f.bak" "$f"`, and re-run green. Never restore with
  `git checkout --`.
- **Pinned tests.** Before editing any file, `git grep -n -F -- '<path or phrase>' internal tests`
  for the path and for each phrase you are about to delete or reword (learning
  `restatement-accumulates-its-own-guards`). Repoint a dependent assert at the text that now owns
  the claim, or delete the row when its subject is gone and its behaviour is covered elsewhere
  (learning `test-premise-deleted-not-regated`). Never re-add deleted text to keep a grep green.
- **New prose rows.** Every `align_0502_*` row shown in a task is appended to `alignmentContracts`
  (created in Task 3, matched whitespace-collapsed by `TestAlignmentContracts`), never to
  `proseContracts`. Edits and deletions of existing `proseContracts` / `docSectionContract` rows stay
  where those rows are.
- **Staging.** Stage explicit paths only (`git add <paths>`), never `git add -A`/`.`. One commit per
  task. Commit message: `docs(skills): <task subject>` (or `test(repoguard): …` for guard tasks),
  body listing any worklist item found not to be a defect, with its evidence.
- **Do not write the results file.** Task workers never edit `docs/results/`.

## Review Focus

The five uncovered conditions most likely to bite a person using this software, most likely first.
Each has a pinning test in the task that owns the code.

1. **`build.gate: off` review.** A repository with `build.gate: off` gets a `skipped` /
   `build-gate-off` evidence record; the reviewer must not raise `unverified-build-state`, while a
   `skipped` record under `local` (or with no gate value in the payload) must still be a blocker.
   Pinned by Task 14's prose row.
2. **Autonomous repair then human sign-off.** `finalize.clear-block` reprobes the published remote
   feature ref; if the autonomous path records the block before publishing the repaired head, the
   human's clear-block refuses `remote-head-mismatch`. Pinned by Task 16's prose row (publish
   before block; human runs `finalize.clear-block`).
3. **Agent wrapper frontmatter.** Every wrapper carries a real skills list in its frontmatter; the
   extended guard must skip it, but a `skills.build` in a wrapper *body* or a skill body must
   redden. Pinned by Task 20's non-vacuity cases and mutation probes.
4. **Wrapped prose and moved headings.** A re-flow or a renamed heading (`(resolver loop)` →
   `(resolver rounds)`) must not silently drop a section-scoped prose row. Pinned by Task 18
   repointing the `docSectionContract` anchors and running them.
5. **A consuming repository reading a skill.** A sentence true only in docket's own repo (its
   `.docket.yml`, its suite command, its `internal/` paths) misleads every other repo. Checked by
   Task 20's acceptance sweep grep for `internal/`, `go run ./cmd/docket`, and "this repo" in
   `skills/` (each hit must be conditioned or removed).

---

### Task 1: Rename `fix-loop.md` → `fix-pass.md` and `gate-caller-loop.md` → `gate-driver.md`

**Build tier:** economy

Spec worklist: *Terminology* (the two file renames, every link updated).

**Files:**
- Rename: `skills/docket-implement-next/references/fix-loop.md` → `skills/docket-implement-next/references/fix-pass.md`
- Rename: `skills/docket-build/references/gate-caller-loop.md` → `skills/docket-build/references/gate-driver.md`
- Modify (links): every file `git grep -l -e 'fix-loop' -e 'gate-caller-loop' -- skills agents AGENTS.md` lists (expected: `skills/docket-build/SKILL.md`, `skills/docket-build/references/task-routing.md`, `skills/docket-build/references/gate-execution.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/edge-paths.md`)
- Modify (tests): `internal/repoguard/budgets_test.go`, `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/gatedrive_json_capture_test.go`, `internal/repoguard/gatecapture_reserved_param_test.go` (comment only)

**Interfaces:**
- Produces: the paths `skills/docket-implement-next/references/fix-pass.md` and `skills/docket-build/references/gate-driver.md`, used by every later task. `sharedContractRel = "skills/docket-build/references/gate-driver.md"`.

- [ ] **Step 1: Repoint the tests first (they will fail until the rename lands)**

In `internal/repoguard/gatedrive_json_capture_test.go`:
```go
const sharedContractRel = "skills/docket-build/references/gate-driver.md"
```
and the exclusion line:
```go
if !isWorkflowMD(rel) || strings.HasSuffix(rel, "docket-build/references/gate-driver.md") {
```
In `internal/repoguard/budgets_test.go` rename the two `skillBudgets` keys to
`"docket-build/references/gate-driver.md"` and `"docket-implement-next/references/fix-pass.md"`
(append `// 0502: renamed from gate-caller-loop.md` / `from fix-loop.md` to their comments).
In `internal/repoguard/prose_contracts_test.go`: the `test_gate_caller_loop` row's `file` →
`skills/docket-build/references/gate-driver.md`; the `test_gate_execution_posture` row's present
phrase `"gate-caller-loop"` → `"gate-driver"`; the `change_0498_fix_loop_condensation` row's
`file` → `skills/docket-implement-next/references/fix-pass.md`. In
`internal/repoguard/gatecapture_reserved_param_test.go` update the comment `(gate-caller-loop.md)`
→ `(gate-driver.md)`.

- [ ] **Step 2: Run to verify failure**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/`
Expected: FAIL (budgeted file missing / file unreadable for the new paths).

- [ ] **Step 3: Rename and fix every link**

```bash
git mv skills/docket-implement-next/references/fix-loop.md skills/docket-implement-next/references/fix-pass.md
git mv skills/docket-build/references/gate-caller-loop.md skills/docket-build/references/gate-driver.md
git grep -l -e 'fix-loop\.md' -e 'gate-caller-loop' -- skills agents AGENTS.md
```
In each listed file replace `fix-loop.md` → `fix-pass.md` and `gate-caller-loop.md` /
`gate-caller-loop` → `gate-driver.md` / `gate-driver` (link targets and link text). Change the H1 of
`fix-pass.md` from `# fix-loop — repairing review findings in-branch` to
`# fix-pass — repairing review findings in-branch`. Leave every other occurrence of the word "loop"
for Task 18. Re-run the grep: it must print nothing.

- [ ] **Step 4: Run the focused check** (Global Constraints). Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next/references/fix-pass.md skills/docket-build/references/gate-driver.md \
  skills/docket-build/SKILL.md skills/docket-build/references/task-routing.md \
  skills/docket-build/references/gate-execution.md skills/docket-implement-next/SKILL.md \
  skills/docket-implement-next/references/edge-paths.md internal/repoguard/ internal/assets/embedded/
git status --short   # confirm the two old paths show as renamed, nothing unintended staged
git commit -m "docs(skills): rename fix-loop and gate-caller-loop references to fix-pass and gate-driver"
```
(Add any other file Step 3's grep listed.)

---

### Task 2: Delete the gate-execution references; move the run-boundary probe scenarios to the release procedure

**Build tier:** standard

Spec worklist: *Deleted outright* → "The gate-execution references".

**Files:**
- Delete: `skills/docket-build/references/gate-execution.md`, `skills/docket-build/references/gate-execution-evidence.md`
- Modify: `skills/docket-build/SKILL.md` (delete the paragraph that links `references/gate-execution.md` with "read it now (blocking) before starting the gate"), `skills/docket-build/references/gate-driver.md` (delete its intro reference to gate-execution)
- Modify: `docs/release/four-harness-acceptance.md` (new section)
- Modify (tests): `internal/repoguard/budgets_test.go` (delete both rows), `internal/repoguard/prose_contracts_test.go` (delete the `test_gate_execution_posture` row)

**Interfaces:**
- Consumes: `gate-driver.md` path from Task 1.
- Produces: a `## Run-boundary continuation acceptance` section in `docs/release/four-harness-acceptance.md`.

- [ ] **Step 1: Write the failing check**

Delete the two `skillBudgets` rows and the `test_gate_execution_posture` row first, then run:
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run TestSkillSizeBudgets`
Expected: FAIL — `skills/**/*.md files with no budget row` lists the two gate-execution files.

- [ ] **Step 2: Move the acceptance material**

Read `skills/docket-build/references/gate-execution.md` section
`## Run-boundary continuation acceptance — outstanding human verification` and its subsections
`### Pending rows`, `### The probe scenarios`, `### Standing rules`. Append to
`docs/release/four-harness-acceptance.md`, before `## What merely running \`docket version\` in the parent does NOT prove`,
a new `## Run-boundary continuation acceptance` section carrying the pending-rows table, the five
probe scenarios, and the standing rules, rewritten as release-procedure steps: present tense, no
change or PR citations, no "Go v1" narration, no measured-verdict history (that stays in git
history and ADR-0081). Link targets inside the moved text must resolve from `docs/release/`.

- [ ] **Step 3: Delete the references and their pointers**

```bash
git rm skills/docket-build/references/gate-execution.md skills/docket-build/references/gate-execution-evidence.md
git grep -n -e 'gate-execution' -- skills agents AGENTS.md
```
Delete the docket-build paragraph that begins "[`references/gate-execution.md`]" / contains
"read it now (blocking) before" starting the gate, and the intro sentence in `gate-driver.md` that
references gate-execution. If a docket-build sentence states a still-true rule that only that
paragraph carried (for example the yield/never-yield posture), keep the rule in one sentence
without the link. The grep must print nothing.

- [ ] **Step 4: Budgets** — set the `docket-build/SKILL.md` and `docket-build/references/gate-driver.md` rows to their exact new counts (Global Constraints, Budget ratchet).

- [ ] **Step 5: Run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-build/SKILL.md skills/docket-build/references/gate-driver.md \
  docs/release/four-harness-acceptance.md internal/repoguard/budgets_test.go \
  internal/repoguard/prose_contracts_test.go internal/assets/embedded/
git commit -m "docs(skills): delete gate-execution references; move run-boundary probes to the release procedure"
```
(`git rm` already staged the deletions.)

---

### Task 3: Delete dummy mode

**Build tier:** standard

Spec worklist: *Deleted outright* → "Dummy mode" (including in-session plain-language requests — no replacement sentence).

**Files:**
- Delete: `skills/docket-convention/references/dummy-mode.md`
- Modify: `skills/docket-convention/SKILL.md` (delete `### Dummy mode (shared definition)` through the paragraph ending "…and the authoring guidance."), and every consumer pointer: `skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-implement-next/SKILL.md` (the `**Dummy mode:**` paragraph and any other hit), `skills/docket-finalize-change/SKILL.md` (its leftover paragraph), `skills/docket-status/SKILL.md`, `skills/docket-auto-groom/SKILL.md`
- Modify (tests): `internal/repoguard/prose_contracts_test.go` (delete both `test_dummy_mode` rows), `internal/repoguard/budgets_test.go` (delete the `dummy-mode.md` row; re-measure every edited skill)

**Interfaces:** none.

- [ ] **Step 1: Create the alignment table and write the failing check**

`scanProse` in `TestProseContracts` matches raw bytes, so a wrapped phrase never matches — a
present phrase reddens on a re-flow and, worse, an **absent** phrase that wraps passes vacuously
(learning `phrase-grep-over-wrapped-prose`). This change's rows therefore live in their own
whitespace-collapsed table. Append to `internal/repoguard/prose_contracts_test.go` (if the package
already has a whitespace-collapse helper — `git grep -n 'strings.Fields' internal/repoguard/*_test.go` —
reuse it instead of adding `collapseAlignWS`):
```go
// alignmentContracts are the agent-facing alignment rows: each phrase is bound
// to one skill or agent file and matched whitespace-collapsed on both sides, so
// a re-flow never reddens a present phrase and a wrapped retired phrase is
// still caught (phrase-grep-over-wrapped-prose).
var alignmentContracts = []proseContract{
	// Dummy mode is not a docket feature; no skill describes it.
	{sentinel: "align_0502_no_dummy_mode", file: "skills/docket-convention/SKILL.md",
		absent: []string{"Dummy mode", "DUMMY_MODE", "In plain terms"}},
}

func collapseAlignWS(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestAlignmentContracts(t *testing.T) {
	root := guardRoot(t)
	if len(alignmentContracts) == 0 {
		t.Fatalf("population floor: no alignment rows")
	}
	for _, c := range alignmentContracts {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil {
			t.Fatalf("read %s (sentinel %s): %v (fail closed)", c.file, c.sentinel, err)
		}
		present := make([]string, len(c.present))
		for i, p := range c.present {
			present[i] = collapseAlignWS(p)
		}
		absent := make([]string, len(c.absent))
		for i, a := range c.absent {
			absent[i] = collapseAlignWS(a)
		}
		for _, msg := range scanProse(c.file, collapseAlignWS(string(b)), present, absent) {
			t.Errorf("[%s] %s", c.sentinel, msg)
		}
	}
	t.Run("non_vacuity", func(t *testing.T) {
		doc := collapseAlignWS("a wrapped\n    clause here")
		if got := scanProse("x.md", doc, nil, []string{collapseAlignWS("wrapped clause")}); len(got) != 1 {
			t.Errorf("a wrapped absent phrase was not caught: %v", got)
		}
		if got := scanProse("x.md", doc, []string{collapseAlignWS("wrapped\nclause")}, nil); len(got) != 0 {
			t.Errorf("a wrapped present phrase was not matched: %v", got)
		}
	})
}
```
(`os`, `filepath`, `strings` are already imported by that file.) Delete the two `test_dummy_mode`
rows and the `dummy-mode.md` budget row. Run
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run 'TestProseContracts|TestAlignmentContracts|TestSkillSizeBudgets'`.
Expected: FAIL (the convention still carries the section; dummy-mode.md has no budget row).

- [ ] **Step 2: Delete**

```bash
git rm skills/docket-convention/references/dummy-mode.md
git grep -n -i -e 'dummy' -e 'DUMMY_MODE' -e 'in plain terms' -- skills agents AGENTS.md
```
Delete every hit's sentence or paragraph. Where a consumer sentence also carried a still-true rule,
keep that rule without the dummy-mode clause. The grep must print nothing.

- [ ] **Step 3: Budgets** — re-measure every edited skill file and set exact rows.

- [ ] **Step 4: Run the focused check.** Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/ internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/assets/embedded/
git status --short   # only the files edited above, plus the dummy-mode.md deletion
git commit -m "docs(skills): delete dummy mode"
```
(`git add skills/` here stages only tracked edits you made; confirm with `git status --short` that
nothing unexpected is staged.)

---

### Task 4: Delete the change and ADR templates; correct the convention's record blocks

**Build tier:** standard

Spec worklist: *Deleted outright* → "The two templates".

**Files:**
- Delete: `skills/docket-adr/adr-template.md`, `skills/docket-new-change/change-template.md`
- Modify: `skills/docket-convention/SKILL.md` (`### Change manifest` block, the `## Artifacts` bullet's "Seeded empty by the template", `### ADR file` block), any skill that mentions either template (`git grep -n -e 'adr-template' -e 'change-template' -e 'the template' -- skills agents`)
- Modify: `internal/render/record.go` (the `ChangeRecord` and `ADRRecord` doc comments)
- Modify (tests): `internal/repoguard/budgets_test.go` (delete both rows), `internal/repoguard/prose_contracts_test.go` (`test_change_types` row)

**Interfaces:**
- Consumes: `render.ChangeRecord` field order (`id, slug, title, status, priority, type, created, updated, depends_on, stacked_on, related, discovered_from, adrs, spec, plan, results, trivial, auto_groomable, branch_prefix, branch, pr, blocked_by, reconciled`; body `## Artifacts`, `## Why`, `## What changes`, `## Out of scope`) and `render.ADRRecord` (`id, slug, title, status, date, supersedes, reverses, relates_to, change`; body `## Context`, `## Decision`, `## Consequences`, `## Alternatives considered`).

- [ ] **Step 1: Confirm the behavioural invariant lives elsewhere, then retire the template row**

Run `git grep -n -e '"type"' -e 'Alternatives considered' -- internal/render/*_test.go internal/app/*_test.go | head`.
A render or app test must assert the `type` field and the ADR `## Alternatives considered` section;
quote the test names in the commit body. Then delete the `test_change_types` row (its subject, the
template, is deleted; learning `test-premise-deleted-not-regated`) and add:
```go
	// 0502: the convention's record blocks mirror render.ChangeRecord / render.ADRRecord.
	{sentinel: "align_0502_record_blocks", file: "skills/docket-convention/SKILL.md",
		present: []string{"branch_prefix:", "## Alternatives considered"},
		absent:  []string{"Seeded empty by the template"}},
```
Delete the two template budget rows. Run
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run 'TestProseContracts|TestAlignmentContracts|TestSkillSizeBudgets'`.
Expected: FAIL (ADR block lacks `## Alternatives considered`; the templates have no budget row).

- [ ] **Step 2: Rewrite the convention blocks**

Change-manifest block: list the fields in `render.ChangeRecord` order (above) with one-line
comments, then `claimed_at:` last with the comment "stamped at claim; refreshed at phase
boundaries; cleared on leaving in-progress". `auto_groomable:` comment: "`true` makes a stub
auto-groomable; unset or `false` means not" (no repository default — see Task 9). `spec:` comment
names the `docket` branch, not `metadata_branch`. Keep `branch_prefix` before `branch`. The body
sections list states that `change.create` writes `## Artifacts`, `## Why`, `## What changes`, and
`## Out of scope`; `## Open questions` and later sections are added by later operations. Replace
"Seeded empty by the template" with "seeded empty by `change.create`". ADR block: add
`## Alternatives considered — the options rejected, and why` after `## Consequences`.

- [ ] **Step 3: Delete the templates and every pointer**

```bash
git rm skills/docket-adr/adr-template.md skills/docket-new-change/change-template.md
git grep -n -e 'adr-template' -e 'change-template' -- skills agents AGENTS.md internal/render
```
Reword the two `internal/render/record.go` comments so the renderer is the authority, e.g.
"ChangeRecord serializes r as a canonical brand-new proposed change record. This renderer is the
authority for a new record's field names, order, and defaults; frontmatter is emitted through
document.New…" and the same for `ADRRecord` ("This renderer is the authority for a new ADR's field
names, order, defaults, and body sections"). Also correct the `AutoGroomable` field comment
`// nil ⇒ null (inherit the repo's auto_groom)` to `// nil ⇒ null (not auto-groomable)`. The grep
must print nothing.

- [ ] **Step 4: Budgets** — set the `docket-convention/SKILL.md` row (and any other edited skill) to exact counts.

- [ ] **Step 5: Run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md internal/render/record.go internal/repoguard/ internal/assets/embedded/
git commit -m "docs(skills): delete the change and ADR templates; renderers own the record shape"
```
(Add any other skill file Step 3's grep made you edit.)

---

### Task 5: Fixed role skills in the convention and the role skills themselves

**Build tier:** standard

Spec worklist: *Roles are fixed defaults* (convention *Skill layer*, *Dispatch-capability resolution*, docket-build, docket-review, docket-brainstorm, `fix-pass.md`). The `$SKILL_*` sites in implement-next, groom-next, new-change, and the plan-writer belong to Task 6.

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — `### Skill layer — pluggable workflow skills (…)`, the dispatch-capability table and its two following paragraphs, the `**Composition (…)**` paragraph's plan-writer clauses ("unavailable dispatch falls back `auto-or-halt`", "the plan-writer invokes the resolved plan skill at runtime as a passthrough… because `skills.plan` may name any installed skill")
- Modify: `skills/docket-build/SKILL.md` — frontmatter description "(skills.build)", "bound by `skills.build`", the `**\`auto-or-halt\`**` paragraph and its "not registered on this machine" follow-up, the halting-conditions bullet naming `skills.build: auto`, every `skills.review` mention
- Modify: `skills/docket-review/SKILL.md` — every `skills.review` binding mention
- Modify: `skills/docket-brainstorm/SKILL.md` — "bound by `skills.brainstorm`"
- Modify: `skills/docket-implement-next/references/fix-pass.md` — the "explicitly configured `skills.build: auto`" clause
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

**Interfaces:**
- Produces: convention *Skill layer* text naming the five fixed role skills, the missing-skill rule without `auto`, and the dispatch-capability posture named **`halt`**. Later tasks refer to "the convention's *Skill layer*" and "*Dispatch-capability resolution*'s `halt` posture".

- [ ] **Step 1: Write the failing check**

In `internal/repoguard/prose_contracts_test.go` delete the `test_role_skill_self_description` row
(it pins `skills.<role>`, a refused key) and add:
```go
	// 0502: roles are fixed; no auto sentinel, no rebinding, halt posture.
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-convention/SKILL.md",
		present: []string{"never outranks", "DIRECTED to:", "**`halt`**"},
		absent:  []string{"auto-or-halt", "`auto` sentinel", "SKILL_BRAINSTORM", "Passthrough."}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-build/SKILL.md",
		absent: []string{"auto-or-halt", "skills.build", "skills.review"}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-review/SKILL.md",
		absent: []string{"skills.review"}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-brainstorm/SKILL.md",
		absent: []string{"skills.brainstorm"}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-implement-next/references/fix-pass.md",
		absent: []string{"skills.build: auto"}},
```
Run `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 2: Rewrite the convention *Skill layer***

Rename the heading to `### Skill layer — the workflow role skills`. Replace the section body with:
a role table with columns Role | Skill | Invoked by | Final artifact — stop-point, rows brainstorm
`superpowers:brainstorming`, plan `superpowers:writing-plans`, build `docket-build`, review
`docket-review` (keep their current "Invoked by" and stop-point cells). **Finish:** verify with
`git grep -n -e 'finishing-a-development-branch' -e 'finish role' -- skills` whether any skill
invokes the finish role (implement-next Step 7 publishes through `workspace.publish` /
`pr.publish`; finalize uses typed operations). If nothing invokes it, drop the finish row and state
that in the commit body; if something does, keep the row with an accurate "Invoked by" cell. Then
keep these bullets, reworded:
- **Role invocation is a skill invocation** — the named skill is invoked, never a same-name agent;
  nested dispatch belongs to the invoked skill's own contract (keep the existing clause that a
  rejected same-name agent dispatch is the wrong operation, not missing-skill evidence and not
  *Dispatch-capability resolution* evidence).
- **Missing-skill rule** — a role skill that cannot be invoked (for example, the superpowers plugin
  is not installed) is done inline by the running agent, which produces the role's final artifact
  (table column 4) and warns prominently in the run output and, for plan/build/review, in the PR
  body. Skill availability is per-machine, which is why this warns rather than halts.
- **`docket-brainstorm`** — the consultant-authored brainstorm (ADR-0022) runs only when a human
  asks for it in that run; otherwise brainstorm is `superpowers:brainstorming`.
- **Autonomy precedence — pre-specified at the call site** — keep the paragraph, but drop "from
  already-resolved config" → "from the run's own inputs", and keep the words "never outranks" and
  `DIRECTED to:`.
Delete: the `skills:` map/rebinding sentence, **Passthrough** (with its ADR-0015 citation), the
**`auto` sentinel** bullet, the **Resolution** bullet (`SKILL_*` exports), and **Role skill
self-description**.

- [ ] **Step 3: Rewrite *Dispatch-capability resolution*'s table**

Third row: Fallback **`halt` — discipline**; Dispatch: the plan-writer dispatch, the build and
review role skills' required nested dispatches, and the in-branch fix workers (which run the build
role's own contract); Posture: **`halt`** — the run stops: the change stays `in-progress` with
`claimed_at` refreshed and the halt reason recorded through `change.halt`. Keep "No new status, no
new field — the reclaim lease self-heals an abandoned claim." Rewrite the `no-fallback` paragraph's
"`auto-or-halt` presupposes a `skills:` role whose resolved value could carry a human's `auto`
authorization" → "`halt` belongs to the workflow roles, and these dispatches are not a role".
Rewrite the paragraph after it to: the missing-skill rule (a role skill that cannot be invoked → done
inline, warned) and `halt` (an invoked role skill whose required nested dispatch cannot run) are two
conditions with two postures. In the `**Composition**` paragraph: "unavailable dispatch falls back
`halt`"; "the plan-writer invokes `superpowers:writing-plans` at runtime, never as a preload".

- [ ] **Step 4: Role skill bodies**

- `docket-build/SKILL.md`: description → "Use as docket's build role — executes an implementation
  plan…" (drop "(skills.build)"); opening line → "docket's build role." Replace the
  `auto-or-halt` paragraph with: a tier agent that cannot be dispatched (established per the
  convention's *Dispatch-capability resolution*, never from a tool name) is the **`halt`**
  posture — halt per *Halting conditions*; never execute tasks inline. Fix the halting-conditions
  bullet (drop the `skills.build: auto` clause). `skills.review` → "the review role (`docket-review`)".
- `docket-review/SKILL.md`: replace binding mentions with "docket's review role".
- `docket-brainstorm/SKILL.md`: "bound by `skills.brainstorm`" → "run when a human asks for a
  consultant-authored brainstorm in this run".
- `fix-pass.md`: the fix workers run the build role's contract; an undispatchable fix worker is the
  `halt` posture (drop the `skills.build: auto` sentence).
Then `git grep -n -e 'skills\.' -e 'auto-or-halt' -e '`auto`' -- skills/docket-convention/SKILL.md skills/docket-build skills/docket-review skills/docket-brainstorm skills/docket-implement-next/references/fix-pass.md`
must show no role-binding hit (a hit inside implement-next/groom-next/new-change is Task 6's).

- [ ] **Step 5: Budgets** — exact counts for every edited skill file.

- [ ] **Step 6: Run the focused check.** Expected: PASS (`TestSkillHandoffSites` still passes: implement-next keeps its sigils until Task 6).

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-build/SKILL.md skills/docket-review/SKILL.md \
  skills/docket-brainstorm/SKILL.md skills/docket-implement-next/references/fix-pass.md \
  internal/repoguard/ internal/assets/embedded/
git commit -m "docs(skills): fixed role skills; undispatchable required dispatch halts"
```

---

### Task 6: Replace the `$SKILL_*` sites; re-key `TestSkillHandoffSites`; plans live under `docs/superpowers/plans`

**Build tier:** standard

Spec worklist: *Roles are fixed defaults* (implement-next, groom-next, new-change, `agents/docket-plan-writer.md`), *Wrong facts* → "Plans location", *Guards* → re-key `TestSkillHandoffSites`.

**Files:**
- Modify: `internal/repoguard/skill_handoff_sites_test.go`
- Modify: `skills/docket-implement-next/SKILL.md` (Step 4 preparation + payload + verification + continue; Step 5; Step 6 review block)
- Modify: `skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md` (brainstorm invocation; delete the `auto` branch)
- Modify: `agents/docket-plan-writer.md` (+ regenerated goldens)
- Modify (tests): `internal/repoguard/budgets_test.go`

**Interfaces:**
- Consumes: Task 5's convention *Skill layer* (missing-skill rule) and `halt` posture.
- Produces: `roleSkillNames`, `roleSkillRe`, `invokeRe`, `negatedInvokeRe`, `classifyHandoffSite(text string) handoffClass` in package `repoguard`.

- [ ] **Step 1: Re-key the guard (write it to fail on today's tree)**

In `internal/repoguard/skill_handoff_sites_test.go`, replace `skillSigilRe` and the classifier:
```go
// roleSkillNames are docket's fixed role skills (convention, *Skill layer*).
var roleSkillNames = []string{
	"superpowers:brainstorming",
	"superpowers:writing-plans",
	"docket-build",
	"docket-review",
	"superpowers:finishing-a-development-branch",
}

// roleSkillRe matches a role-skill name as a whole backticked token, so a
// tier agent such as `docket-build-economy` or `docket-review-lean` never
// matches.
var roleSkillRe = func() *regexp.Regexp {
	q := make([]string, len(roleSkillNames))
	for i, n := range roleSkillNames {
		q[i] = regexp.QuoteMeta(n)
	}
	return regexp.MustCompile("`(?:" + strings.Join(q, "|") + ")`")
}()

var (
	// invokeRe is the invocation verb the house idiom puts on every genuine
	// role invocation ("is invoked **DIRECTED to:**", "invokes").
	invokeRe = regexp.MustCompile(`(?i)\binvok(?:e|ed|es|ing)\b`)
	// negatedInvokeRe is a prohibition ("do NOT invoke", "never invoked"),
	// which pre-specifies nothing and needs no marker.
	negatedInvokeRe = regexp.MustCompile(`(?i)\b(?:not|never)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b`)
	skillFramedRe   = regexp.MustCompile(`(?i)long [a-z-]+ dispatch`)
)
```
`discoverHandoffSites` matches `roleSkillRe` instead of `skillSigilRe`. `classifyHandoffSite`:
```go
func classifyHandoffSite(text string) handoffClass {
	if strings.Contains(strings.ToLower(text), "human is present") {
		return handoffException
	}
	if len(invokeRe.FindAllStringIndex(text, -1)) > len(negatedInvokeRe.FindAllStringIndex(text, -1)) {
		return handoffInvocation
	}
	return handoffMention
}
```
Keep the wrapper floor (>= 10), `checked >= 4`, `invocations >= 3`, `exceptions == 0`, the
belongs-to check, and the framing ban. Rewrite the file's header comment: sites are every line in
`skills/` markdown naming a fixed role skill as a backticked token; an invocation line is one
whose invocation verbs outnumber its prohibitions; a mention needs no marker. Replace the
`non_vacuity` fixtures with:
```go
		const unmarked = "The build role `docket-build` is invoked to execute the plan."
		const marked = "`docket-build` is invoked **DIRECTED to:** execute the plan task-by-task and stop."
		const mention = "For the `docket-review` role, select the review tier from the build record."
		const prohibited = "do NOT invoke `superpowers:brainstorming` with a simulated human answerer"
		const tier = "Dispatch `docket-build-economy` by name, foreground."
		const framed = "Refresh the claim before this long build dispatch — `docket-build` is invoked **DIRECTED to:** execute the plan."
		if classifyHandoffSite(unmarked) != handoffInvocation || strings.Contains(unmarked, skillMarker) {
			t.Errorf("an unmarked invocation was not classified as a violating invocation")
		}
		if classifyHandoffSite(marked) != handoffInvocation || !strings.Contains(marked, skillMarker) {
			t.Errorf("a marked invocation was not classified as an invocation carrying the marker")
		}
		if classifyHandoffSite(mention) != handoffMention {
			t.Errorf("a role mention was classified as an invocation")
		}
		if classifyHandoffSite(prohibited) != handoffMention {
			t.Errorf("a prohibition was classified as an invocation")
		}
		if !roleSkillRe.MatchString(marked) || roleSkillRe.MatchString(tier) {
			t.Errorf("role-skill discovery must match a backticked role skill and never a tier agent name")
		}
		if !skillFramedRe.MatchString(framed) {
			t.Errorf("framing ban missed the framed-as-dispatch shape")
		}
		if skillFramedRe.MatchString("Dispatch the selected tier wrapper by name, foreground") {
			t.Errorf("framing ban wrongly caught genuine nested-dispatch prose")
		}
```
Run `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites`.
Expected: the `non_vacuity` subtest PASSES (it exercises the new classifier directly). The main
scan may pass on today's tree too, because today's invocation lines already carry the backticked
default (`` (default `docket-build`) ``) beside "invoked **DIRECTED to:**". That is fine: the binding
proof that the re-keyed scan guards real lines is Step 4's mutation, run after the rewrite.

- [ ] **Step 2: Rewrite implement-next's role sites**

- Step 4 *Preparation*: "Resolve nothing new: `$SKILL_PLAN`, `$SKILL_BUILD`, learnings enablement…"
  → "Resolve nothing new: learnings enablement and every repo path come from the startup-check
  prepared context."
- Step 4 payload line (inside the `docket:feature-dispatch` block; keep the block's line shape and
  the `Feature worktree:` line unchanged): drop "resolved `$SKILL_PLAN` and `$SKILL_BUILD`"; the
  child "invokes `superpowers:writing-plans` **DIRECTED to:** write the plan file under
  `docs/superpowers/plans/` and stop there (when the skill cannot be invoked it authors the plan
  itself, warning prominently — carry that warning into the run report and PR body)…".
- Step 4 *Verification*: replace the "deliberately **no directory allowlist**…" sentence with "The
  plan must live under `docs/superpowers/plans/` — `change.attach-plan` accepts no other location."
  (verify `plansPlanningRoot` in `internal/app/change_attach.go` first).
- Step 4 *Continue*: the dispatch fallback is **`halt`** — plan-writer dispatch unavailable
  (established per the convention's *Dispatch-capability resolution*, never from a tool name) halts:
  the change stays `in-progress` with `claimed_at` refreshed and the halt reason recorded. Delete the
  `SKILL_PLAN=auto` inline branch.
- Step 5: "The **resolved build skill** — `$SKILL_BUILD` from the startup-check config export
  (default `docket-build`) — is invoked **DIRECTED to:**" → "The build role — `docket-build` — is
  invoked **DIRECTED to:**" (one line; keep "**DIRECTED to:**" on the same line as the backticked
  name and "invoked"). Delete the "A resolved `skills.build` value…custom build skill…" sentences
  except the rejected-same-name-agent rule; delete "On `auto` or unavailability, apply the build
  auto-fallback…"; replace with the missing-skill rule (build inline when `docket-build` cannot be
  invoked, warning prominently). Required nested dispatch unavailable → **`halt`**.
- Step 6 review block (inside its `docket:feature-dispatch` block): "The review role —
  `docket-review` — is invoked **DIRECTED to:** review the whole branch…"; it dispatches the
  selected tier foreground (keep the payload lines). Delete the `$SKILL_REVIEW` names-any-other-skill
  branch and the `auto` fallback; missing-skill rule instead; an undispatchable tier → **`halt`**.
  The tier-selection paragraph: "For the `docket-review` role…"; drop "any build role rebound away
  from `docket-build`… `superpowers:subagent-driven-development` among them" → "When the build
  emits no build record (for example, it ran inline under the missing-skill rule), the tier
  defaults to `docket-review-standard`".

- [ ] **Step 3: groom-next, new-change, plan-writer**

- `docket-groom-next/SKILL.md` and `docket-new-change/SKILL.md`: "run the **resolved brainstorm
  skill** — `$SKILL_BRAINSTORM` from the startup-check config export (default
  `superpowers:brainstorming`) —" → "invoke `superpowers:brainstorming` —" (or `docket-brainstorm`
  when the human asked for a consultant-authored spec in this run). Delete the "If it resolves to
  `auto`…" branch; when `superpowers:brainstorming` cannot be invoked, run the brainstorm inline with
  the human, warning prominently.
- `agents/docket-plan-writer.md`: description → "…invokes `superpowers:writing-plans` in a pinned
  context…". Body: the payload names no plan or build skill (drop "the resolved plan skill
  (`SKILL_PLAN`) and build skill (`SKILL_BUILD`) names"); step 3 "Invoke `superpowers:writing-plans`
  DIRECTED to: write the plan file under `docs/superpowers/plans/` and stop there. Answer any
  execution-mode or option choice it poses internally: the plan is executed by `docket-build`;
  surface none."; step 4 "When `superpowers:writing-plans` cannot be invoked, apply the missing-skill
  rule: warn prominently and author the same plan yourself under `docs/superpowers/plans/`." (delete
  the `auto` and custom-skill-location clauses).
- Regenerate goldens (Global Constraints) and inspect the diff.

Then: `git grep -n -e 'SKILL_' -- skills agents AGENTS.md` must print nothing.

- [ ] **Step 4: Run, then mutation-test the guard**

Run `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run TestSkillHandoffSites`. Expected: PASS.
Mutation (Global Constraints procedure): in `skills/docket-implement-next/SKILL.md` delete the
`**DIRECTED to:**` on the Step 5 build-invocation line (`perl -0pi -e 's/(`docket-build` — is invoked )\*\*DIRECTED to:\*\*/$1/' "$f"`,
confirm the flattened count of "is invoked **DIRECTED to:**" dropped by one). Expected: FAIL with
"autonomous role invocation does not pre-specify its outcome". Restore with `mv -f`, re-run: PASS.

- [ ] **Step 5: Budgets** — exact counts for every edited skill file.

- [ ] **Step 6: Run the focused check.** Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-groom-next/SKILL.md skills/docket-new-change/SKILL.md \
  agents/docket-plan-writer.md internal/harness/ internal/repoguard/ internal/assets/embedded/
git status --short   # only the plan-writer goldens changed under internal/harness/
git commit -m "docs(skills): invoke the fixed role skills by name; re-key TestSkillHandoffSites"
```

---

### Task 7: Rewrite the convention's configuration contract from the schema

**Build tier:** standard

Spec worklist: *Facts* → "Configuration", "Prepare context"; *Deleted outright* → the convention's `.docket.yml` sample, `finalize.skip_results_only_delta`, `finalize.gate: ci`/`both`, the legacy `agents.yaml` auto-migration; *Wrong facts* → "Convention, configuration".

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — `### Configuration — \`.docket.yml\` …` (sample, the paragraph after it, **Config layers**, the resolution paragraph, **`finalize` — the finalize gate**), *Startup check* step 3's context list
- Modify: `skills/docket-implement-next/references/edge-paths.md` (`skip_results_only_delta`), any other hit of `skip_results_only_delta`, `ci` / `both` finalize gate values, `origin/HEAD`, `agents.yaml` in `skills/`
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`, `internal/repoguard/capability_surface_test.go` (only if a pinned remedy count changes)

**Interfaces:**
- Produces: a convention sample that a verbatim copy never blocks writes with.

- [ ] **Step 1: Write the failing check**

```go
	// 0502: the convention's configuration contract matches the schema.
	{sentinel: "align_0502_config_contract", file: "skills/docket-convention/SKILL.md",
		present: []string{"never from `origin/HEAD`", "# local | off", "max_attempts: 4"},
		absent: []string{"fallback main", "repair `origin/HEAD`", "`ci` polls GitHub checks",
			"skip_results_only_delta", "agents.yaml", "terminal_publish:", "auto_capture:", "auto_groom:"}},
	{sentinel: "align_0502_config_contract", file: "skills/docket-implement-next/references/edge-paths.md",
		absent: []string{"skip_results_only_delta"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 2: Rewrite the sample**

Replace the YAML sample with supported keys only (verify every default against
`internal/config/schema.go` / `docket diagnostic config --help` before writing it; fix any value
that differs):
```yaml
# .docket.yml — committed in the primary worktree; every key is optional
integration_branch: auto     # auto (origin's HEAD branch) | main | develop — where code lands
changes_dir: docs/changes
adrs_dir: docs/adrs
results_dir: docs/results
change_types: [chore, docs, feat, fix, refactor, perf]  # a higher layer replaces this list
board_surfaces: [inline]     # inline (BOARD.md); [] = no board
build:                       # the build role's own gate
  gate: local                # local | off
  test_command: ""           # "" = unconfigured; `docket repository configure-tests`
  max_attempts: 4
finalize:                    # rebase onto the base and re-test before merge
  gate: local                # local | off
  test_command: ""
  require_pr_approval: false
  resolver_max_attempts: 10
  repair_max_attempts: 6
run:
  max_attempts: 2
review:
  min_fix_severity: minor
  max_fix_tasks: 10
reclaim:
  lease_ttl: 72
  auto: false
learnings:
  enabled: true              # false = read/write gate, never a purge
gate_observation_budget: 30  # minutes; enforced by the gate driver
agent_harnesses: [claude]    # harnesses whose repository dispatch blocks install writes
```
Keep `board.section_order` / `board.sorting.<section>.by|direction` as one prose sentence after the
sample (supported, optional). Count `docket repository configure-tests` occurrences before/after; if
the count changed, update the `capabilityExemptions` pin (Global Constraints).

- [ ] **Step 3: Rewrite the facts around it**

- Location: configuration is read from the primary worktree's `.docket.yml` and
  `.docket.local.yml` plus the global `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` — never from
  `origin/HEAD`. `integration_branch: auto` resolves origin's HEAD branch; an unresolvable remote
  HEAD is an error, not a fallback to `main`. Delete "`metadata_branch` resolves where PM commits
  land" (Task 17 owns the remaining layout wording, but this sentence goes now).
- **Config layers:** repo-local > repo-committed > global > built-in, per field. Agent model/effort
  pins (`agents.<harness>.<agent>.model|effort`) are honoured **from the global config only**; in
  `.docket.yml` or `.docket.local.yml` an agent pin blocks writes. Any explicit `skills.*` value
  blocks writes (describe this without the dotted spelling: "any explicit role-skill value"). Only
  `docket install` downgrades an unknown key to a warning; every other read treats it as an error.
  Keep the shared-setting guard sentence (ADR-0019) only if `internal/config` still implements it
  (grep `SharedSetting\|shared-setting` in `internal/config`); delete the legacy `agents.yaml`
  clause.
- Resolution paragraph: "This resolution — read configuration, apply defaults, resolve
  `integration_branch` — runs inside the **`repository.prepare`** operation (the *startup check*)…"
  (no "repair `origin/HEAD`").
- **`finalize` — the finalize gate:** `local` (default) runs the repo's suite locally; `off` merges
  trusting the PR's own CI. Delete `ci` and `both`.
- *Startup check* step 3 context list: add "the `build` configuration (`gate`, `test_command`,
  `max_attempts`)" beside the finalize configuration (verify `PrepareContext` in
  `internal/app/repository_prepare.go`).
- `edge-paths.md`: delete the `skip_results_only_delta` clause (keep the surrounding rule).

- [ ] **Step 4: Budgets** — exact counts for every edited skill file.

- [ ] **Step 5: Run the focused check.** Expected: PASS (`TestConfigReadChannel`'s reader-liveness floor needs ≥3 config tokens left in the convention).

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-implement-next/references/edge-paths.md \
  internal/repoguard/ internal/assets/embedded/
git commit -m "docs(skills): rewrite the convention's configuration contract from the schema"
```

---

### Task 8: Delete the startup-export reads that never arrive

**Build tier:** standard

Spec worklist: *Facts* → "Prepare context", "Where the other values live"; *Deleted outright* → "Auto-capture", "`build.checkpoint: true`".

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (`### Discovered work (auto-capture deferred)`)
- Modify: `skills/docket-implement-next/SKILL.md` (`REVIEW_MIN_FIX_SEVERITY` in Step 6 triage), `skills/docket-implement-next/references/fix-pass.md` (`REVIEW_*` and `## The severity threshold`)
- Modify: `skills/docket-build/SKILL.md` (`GATE_OBSERVATION_BUDGET` ×2, `BUILD_CHECKPOINT` ×2 and the ledger mode it governs)
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`, `internal/repoguard/capability_surface_test.go` (if `docket change create` count changes)

**Interfaces:** none.

- [ ] **Step 1: Write the failing check**

```go
	// 0502: no skill reads a value the prepared context does not carry.
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-convention/SKILL.md",
		present: []string{"reported in the run's final report"},
		absent:  []string{"AUTO_CAPTURE", "auto-capture deferred"}},
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-build/SKILL.md",
		absent: []string{"GATE_OBSERVATION_BUDGET", "BUILD_CHECKPOINT"}},
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"`review.min_fix_severity`"},
		absent:  []string{"REVIEW_MIN_FIX_SEVERITY", "REVIEW_MAX_FIX_TASKS"}},
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-implement-next/references/fix-pass.md",
		present: []string{"`diagnostic.config`"},
		absent:  []string{"REVIEW_MIN_FIX_SEVERITY", "REVIEW_MAX_FIX_TASKS"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 2: Rewrite**

- Convention: rename the heading to `### Discovered work`; delete the first paragraph (the
  `auto_capture` map, `AUTO_CAPTURE_*` exports, "deferred" narration). Keep: work an autonomous
  skill discovers mid-run is **reported in the run's final report, never silently minted or
  discarded**; a human captures reported work deliberately with `docket change create`. Delete the
  "explicit request to capture, or an enabled `auto_capture` key… frozen Bash scripts…" sentence.
- implement-next Step 6 triage and `fix-pass.md`: the severity floor is `review.min_fix_severity`
  (default `minor`) and the non-blocker cap is `review.max_fix_tasks` (default 10), both read through
  the `diagnostic.config` operation (verify the operation's output carries them:
  `docket diagnostic config --help`, `docket schema`); blockers are fixed regardless.
- docket-build: the observation budget is enforced by the gate driver itself (it fails closed when
  spent); no skill reads or passes `gate_observation_budget` — delete both export sentences and keep
  the halting condition "the driver's observation budget is exhausted with no terminal gate result".
  Delete the `BUILD_CHECKPOINT` read and the `build.checkpoint: true` ledger mode entirely (keep
  the *Build-findings checkpoint* section — that is the results-file checkpoint, a different thing).
- Count `docket change create` on the workflow surface before/after; update the pin if it moved.

- [ ] **Step 3: Budgets** — exact counts.

- [ ] **Step 4: Run the focused check.** Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-implement-next/SKILL.md \
  skills/docket-implement-next/references/fix-pass.md skills/docket-build/SKILL.md \
  internal/repoguard/ internal/assets/embedded/
git commit -m "docs(skills): read configuration values from where they live, not from phantom exports"
```

---

### Task 9: Terminal publish, learnings, and auto-groom selection

**Build tier:** standard

Spec worklist: *Deleted outright* → "Terminal publish", "`learnings.cap`"; *Facts* → "Learnings", "Auto-groom selection"; Bash-era table row "human curation only" learnings.

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (*Directory layout* learnings lines, `## Publish deferred` bullet, "re-stamping … at terminal publish" in the frozen-records paragraph, *Learnings ledger*, *Autonomous grooming*, *Branch model*'s terminal-publication sentences)
- Modify: `skills/docket-convention/references/close-out.md` (step 3 and every terminal-publish line), `skills/docket-convention/references/learnings.md`, `skills/docket-adr/SKILL.md` (`## How an ADR reaches the integration branch (deferred)`), `skills/docket-new-change/SKILL.md`, `skills/docket-implement-next/SKILL.md` (terminal-publication sentences in Step 4 and Step 6.5), `skills/docket-implement-next/references/fix-pass.md` (cap), `skills/docket-status/SKILL.md` (`### Learnings`), `skills/docket-auto-groom/SKILL.md` (description and selection), `skills/docket-groom-next/SKILL.md` (selection bands)
- Modify (tests): `internal/repoguard/prose_contracts_test.go` (delete the `test_docket_metadata_branch` row for `skills/docket-new-change/SKILL.md` that pins "terminal publication is deferred from Go v1"; keep `test_learnings_ledger`), `internal/repoguard/budgets_test.go`

**Interfaces:** none.

- [ ] **Step 1: Verify the facts**

```bash
git grep -n -e 'learnings/README' -e 'learning_ops' -- internal/app | head
docket learning record --help; docket learning update --help
```
Confirm `learning.record` / `learning.update` write findings, `learnings.enabled` gates them, and
nothing refreshes `learnings/README.md` (the operations never touch the index —
`internal/app/learning_ops.go` header). Confirm auto-groom selection: grep `AutoGroomable` in
`internal/app` / `internal/domain` — only `auto_groomable: true` is auto-groomable.

- [ ] **Step 2: Write the failing check**

```go
	// 0502: no terminal publish, no learnings cap, no repository auto_groom default.
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-convention/SKILL.md",
		present: []string{"`learning.record`", "`learning.update`", "### Learnings ledger", "will the agent know to search for this?"},
		absent:  []string{"human curation only", "terminal publish", "Terminal publication", "learnings.cap", "repo's `auto_groom`"}},
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-convention/references/close-out.md",
		absent: []string{"terminal publish", "Terminal publication", "terminal_publish"}},
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-convention/references/learnings.md",
		present: []string{"`learning.record`"},
		absent:  []string{"learnings.cap", "## Capacity", "human curation"}},
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-adr/SKILL.md",
		absent: []string{"(deferred)", "terminal publish"}},
```
Delete the new-change `test_docket_metadata_branch` row. Run
`… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 3: Rewrite**

- Terminal publish: delete every narration (`git grep -n -i -e 'terminal publi' -e 'terminal_publish' -e 'publish-deferred' -e 'Publish deferred' -- skills agents`),
  including close-out step 3 (renumber the remaining steps and every cross-reference to them),
  the convention's `## Publish deferred` body-section bullet, and docket-adr's deferred section.
  State the current fact once where needed: archived records and ADRs stay on the `docket` branch;
  the integration branch gets code, plans, and results through PRs only (verify in
  `internal/app/finalize_closeout.go`). Frozen-records paragraph: the backlink re-stamp happens only
  through `artifact.backlink` (drop "at terminal publish").
- Learnings: writers are the `learning.record` and `learning.update` operations, gated by
  `learnings.enabled`; `learnings/README.md` is a derived index that no operation refreshes
  (describe only that); delete the cap, the active-findings capacity section in `learnings.md`
  (`## Capacity`), every "human curation only" / "harvest deferred" sentence, and the `#NNN`
  citations in `learnings.md`. Promotion and consolidation stay human acts.
- Auto-groom: only a per-change `auto_groomable: true` makes a stub auto-groomable; unset means
  not. Convention *Autonomous grooming*: delete "else the repo's `auto_groom` knob". auto-groom
  description: "Use when individual stubs opted into autonomous grooming (`auto_groomable: true`)…".
  groom-next bands: band (2) is "`auto_groomable` unset or `false`", band (3) "`auto_groomable:
  true`" (drop "effective").

- [ ] **Step 4: Budgets** — exact counts.

- [ ] **Step 5: Run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/ internal/repoguard/ internal/assets/embedded/
git status --short   # only the files listed above
git commit -m "docs(skills): delete terminal publish and the learnings cap; auto-groom is per-change only"
```

---

### Task 10: Replace Bash-era names with typed operations; correct the `## Run halted` and hooks facts

**Build tier:** standard

Spec worklist: *Bash-era names → typed operations* (all rows except the status/adr health checks, which are Task 11); *Facts* → "Stacks", "Hooks", "Scripts", "Run tracker"; *Wrong facts* → "Convention, *Change body sections*".

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (*Reaching docket's operations* script sentences, **Script contracts** paragraph, *Build-readiness* `stack-base.sh`, `## Run halted` bullet, *Composition* `verify-run`, *Branch model* `disable-worktree-hooks.sh`)
- Modify: `skills/docket-convention/references/close-out.md` (`scripts/<name>.md` contracts, `board off`, `promote-failed`, `stack-carried-failed`)
- Modify: `skills/docket-convention/references/stacked-changes.md` (`verify-run`, `fm_field`, hand `git worktree add`, hand `gh pr edit`, `stack-invalid`, `stack-parent-killed`)
- Modify: `skills/docket-implement-next/SKILL.md` (`reclaim-claims`, `run-halt`), `skills/docket-implement-next/references/edge-paths.md` (`verify-run`)
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

**Interfaces:** none.

- [ ] **Step 1: Verify the facts**

```bash
ls scripts/
git grep -n -e 'RunHaltedSection' -e 'OwnedRemovals' -- internal/domain internal/app | head
git grep -n -e 'core.hooksPath' -- internal/gitcli | head
docket workspace prepare --help; docket finalize retarget-children --help; docket run verify --help
```
Expected: `scripts/` holds only `release-smoke.sh` and its contract; `## Run halted` is written by
`change.halt` and removed both by `change.resume-halted` and by the claim (`domain.Claim` reports
it as an owned removal); `repository prepare` turns hooks off.

- [ ] **Step 2: Write the failing check**

```go
	// 0502: typed operations replace the Bash-era names.
	{sentinel: "align_0502_typed_ops", file: "skills/docket-convention/SKILL.md",
		present: []string{"`stack-base-unresolved`", "`change.resume-halted`"},
		absent:  []string{"stack-base.sh", "disable-worktree-hooks", "verify-run", "scripts/<name>.md"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-convention/references/stacked-changes.md",
		present: []string{"`workspace.prepare`", "`finalize.retarget-children`", "`change-stack-cycle`"},
		absent:  []string{"verify-run", "fm_field", "git worktree add", "gh pr edit", "stack-invalid", "stack-parent-killed"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-convention/references/close-out.md",
		absent: []string{"scripts/<name>.md", "board off", "promote-failed", "stack-carried-failed"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-implement-next/SKILL.md",
		absent: []string{"reclaim-claims"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-implement-next/references/edge-paths.md",
		absent: []string{"verify-run"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 3: Rewrite, per the spec's table**

- `stack-base.sh` → the change's readiness is `stack-base-unresolved` until the binary resolves its
  effective base; `disable-worktree-hooks.sh` → `repository prepare` turns the shared hooks off in
  the `.docket/` worktree; delete the **Script contracts** paragraph and the "deterministic helper
  scripts this convention still names…" sentence (operations are reached through the capability
  catalog and the `schema` operation); `verify-run` → `run.verify`; `fm_field` → the record's
  fields from the `status` operation with `--json`; `reclaim-claims` → the `change.reclaim`
  operation / the maintenance sweep's reclaim; `run-halt` → `run-halted` (only where it means the
  verdict); stacked worktrees come from `workspace.prepare`; child PRs are retargeted by
  `finalize.retarget-children`; delete the health-check names and tokens the binary does not emit,
  naming only `change-stack-cycle` for stacks.
- `## Run halted` bullet: written by the `change.halt` operation (bare heading; the date is in the
  body); `run.verify` reports `run-halted` while it is present; removed by the
  `change.resume-halted` operation, and by the claim when a halted change is claimed again.
  Delete the claim that implement-next's Step 2 alone owns removal unless the code says so.

- [ ] **Step 4: Budgets** — exact counts.

- [ ] **Step 5: Run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-convention/references/close-out.md \
  skills/docket-convention/references/stacked-changes.md skills/docket-implement-next/SKILL.md \
  skills/docket-implement-next/references/edge-paths.md internal/repoguard/ internal/assets/embedded/
git commit -m "docs(skills): name the typed operations, not the Bash-era scripts"
```

---

### Task 11: docket-status and docket-adr name only findings the binary emits

**Build tier:** standard

Spec worklist: *Facts* → "Findings and checks"; *Wrong facts* → "docket-status (SKILL.md and the `agents/docket-status.md` description)"; Bash-era table checks row for docket-adr and status.

**Files:**
- Modify: `skills/docket-status/SKILL.md` (frontmatter description, `## Run the pass`, `## Read the report`, `### Health checks`, any `publish-deferred`/`adr-unpublished`/stale-claim/dependency-stall text)
- Modify: `skills/docket-adr/SKILL.md` (`adr-unpublished`, `publish-deferred`, health-check mentions)
- Modify: `agents/docket-status.md` (description, body "run the sweep + health checks") (+ regenerated goldens)
- Modify (tests): `internal/repoguard/prose_contracts_test.go` (delete the `test_docket_metadata_branch` row pinning `adr-unpublished` in `skills/docket-adr/SKILL.md`), `internal/repoguard/budgets_test.go`

**Interfaces:** none.

- [ ] **Step 1: Verify which findings exist**

```bash
docket status --help; docket repository check --help; docket maintenance sweep --help
git grep -n -e '"artifact-missing"' -e '"branch-malformed"' -e '"parse-failed"' -e 'board-stale' -e 'waiting-dependency' -- internal | grep -v _test | head -20
```
Expected (spec facts): `status` reports configuration diagnostics, `parse-failed`, record
validation codes, `artifact-missing`, `branch-malformed`; `repository check` reports
`board-stale`, `adr-index-stale`, `artifact-links-stale`, `docket-worktree-hooks-*`, …; stale
claims surface as `maintenance sweep` reclaim items; stalled dependencies surface as the
`waiting-dependency` readiness with `unmet_dependencies`.

- [ ] **Step 2: Write the failing check**

```go
	// 0502: status and adr name only findings the binary emits.
	{sentinel: "align_0502_real_findings", file: "skills/docket-status/SKILL.md",
		present: []string{"`waiting-dependency`", "`artifact-missing`"},
		absent:  []string{"publish-deferred", "adr-unpublished", "dependency stalls", "stale claims,"}},
	{sentinel: "align_0502_real_findings", file: "skills/docket-adr/SKILL.md",
		absent: []string{"adr-unpublished", "publish-deferred"}},
	{sentinel: "align_0502_real_findings", file: "agents/docket-status.md",
		absent: []string{"dependency stalls"}},
```
Delete the `adr-unpublished` presence row. Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 3: Rewrite**

docket-status description (skill and agent alike): "…by refreshing docket state, sweeping merged
changes to done, and reporting configuration, record, and artifact-link findings." `### Health
checks`: list only the real findings, and say where stale claims (the sweep's reclaim items) and
stalled dependencies (`waiting-dependency` with `unmet_dependencies`) surface. Keep the
`change_0389_sweep_scope`, `change_0397_preflight_op`, and `change_0448_named_preflight_skip`
pinned phrases intact (grep them first). docket-adr: delete the unpublished/deferred health-check
text; an ADR stays on the `docket` branch. Regenerate goldens for `docket-status` only.

- [ ] **Step 4: Budgets** — exact counts.

- [ ] **Step 5: Run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-status/SKILL.md skills/docket-adr/SKILL.md agents/docket-status.md \
  internal/harness/ internal/repoguard/ internal/assets/embedded/
git commit -m "docs(skills): docket-status and docket-adr name only findings the binary emits"
```

---

### Task 12: Rewrite the agent layer around user-level wrappers and global-only pins

**Build tier:** standard

Spec worklist: *Deleted outright* → "Repo-layer `agents:` pins…", "The legacy `agents.yaml` auto-migration"; *Facts* → "Agent wrappers"; *Wrong facts* → "Agent layer".

**Files:**
- Modify: `skills/docket-convention/references/agent-layer.md` (rewrite)
- Modify: `skills/docket-convention/SKILL.md` (`### Agent layer — model/effort-pinned subagents (…)` two paragraphs and the blocking-read pointer paragraph)
- Modify (tests): `internal/repoguard/budgets_test.go`; keep `TestCommittedCodexLaunchMatrix`-style pins in `internal/repoguard/root_entry_dispatch_test.go` green (its `agent-layer.md` present/absent clauses)

**Interfaces:** none.

- [ ] **Step 1: Verify the facts and the pinned clauses**

```bash
sed -n 1,20p agents/harness-defaults.yml
git grep -n -e 'agent-layer.md' -- internal tests
sed -n 120,170p internal/repoguard/root_entry_dispatch_test.go
git grep -n -e 'func ' -- internal/reposeed/plan.go | head -20
docket install --help; docket install check --help
```
Record the three verbatim `agent-layer.md` clauses `root_entry_dispatch_test.go` requires; they
stay byte-identical (including their line breaks). Confirm: wrappers install at user level only;
install writes no per-repository agent definitions (`internal/reposeed/plan.go`); the repository
surfaces install writes are the managed dispatch blocks (`CLAUDE.md`, `AGENTS.md`,
`.cursor/rules/docket-dispatch.mdc`) for the harnesses `agent_harnesses` opts into; the built-in
model/effort table is compiled into the binary (`internal/config/defaults.go`, kept equal to
`agents/harness-defaults.yml` by a test); only the global config overrides it, per agent and per
field; `docket install check` is a machine-only report.

- [ ] **Step 2: Write the failing check**

```go
	// 0502: wrappers are user-level; pins are global-only.
	{sentinel: "align_0502_agent_layer", file: "skills/docket-convention/references/agent-layer.md",
		present: []string{"from the global configuration only", "installed at user level"},
		absent:  []string{"agents.yaml", "drift-check gate", "per-repo agent pass", "change 0"}},
	{sentinel: "align_0502_agent_layer", file: "skills/docket-convention/SKILL.md",
		absent: []string{"per-repo agent pass", "generates wrapper files for"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 3: Rewrite `agent-layer.md`**

Sections: what a wrapper is (a thin agent definition carrying model/effort and the skills list in
its frontmatter — never spell the field as `skills:`); where wrappers live (wrappers are installed
at user level by `docket install`, never per repository — use the exact words "installed at user
level"); the compiled built-in model/effort table and global-only overrides
(`agents.<harness>.<agent>.model|effort` overrides come from the global configuration only — use
those exact words; in `.docket.yml` /
`.docket.local.yml` a pin blocks writes; overrides apply per agent and per field; model IDs are
opaque passthrough values, ADR-0015); `agent_harnesses` as the opt-in for a repository's dispatch
blocks; launch posture (keep the Codex launch-matrix material and the three pinned clauses
verbatim); `docket install check` is a machine-only report. Delete repo-layer `agents:` pins,
per-repository wrapper generation, the Bash validator, the generic emitter, the three-leg drift gate,
the `agents.yaml` migration, and every change citation.
Convention *Agent layer*: drop "(change 0016)"; "A wrapper is a thin installed file… The Go
install generates each wrapper from the compiled built-in table plus global overrides…"; delete
the "per-repo agent pass generates wrapper files" claim and the "via `skills: [<skill>,
docket-convention]`" spelling (→ "lists the skill and `docket-convention` in its skills list").
The blocking-read pointer: "before configuring agent model/effort pins or `agent_harnesses`, or
debugging the agent install".

- [ ] **Step 4: Budgets** — exact counts (both files).

- [ ] **Step 5: Run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/references/agent-layer.md skills/docket-convention/SKILL.md \
  internal/repoguard/ internal/assets/embedded/
git commit -m "docs(skills): agent layer describes user-level wrappers and global-only pins"
```

---

### Task 13: Bug 1 — auto-groom drafts outside `.docket/`

**Build tier:** standard

Spec worklist: *The four bugs* → 1.

**Files:**
- Modify: `skills/docket-auto-groom/SKILL.md` (Step 2 "Draft the spec to `.docket/docs/superpowers/specs/…`", Step 4 Spec exit, Step 5 contended path)
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

**Interfaces:** none.

- [ ] **Step 1: Write the failing check**

```go
	// 0502 bug 1: the draft never touches .docket/ (a dirty metadata worktree
	// makes the next repository.prepare refuse metadata-worktree-dirty).
	{sentinel: "align_0502_autogroom_draft", file: "skills/docket-auto-groom/SKILL.md",
		present: []string{"never inside `.docket/`", "`spec_markdown`", "discard the draft"},
		absent:  []string{".docket/docs/superpowers/specs/", "delete any just-drafted spec markdown"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 2: Fix**

Step 2: "Draft the spec in session scratch space — never inside `.docket/` — as the Markdown body
that Step 4 sends as `spec_markdown` in the `change.groom` request file, with an `## Assumptions`
block: …" (match how `skills/docket-groom-next/SKILL.md` Step 4 phrases its request file). Step 4
Spec exit: the request file carries `spec_markdown`; the operation writes the spec file. Step 5
contended path: "…DISCARD this iteration's draft (discard the draft from scratch space) and return to
step 1" (keep "DISCARD"; "loop" wording is Task 18's, but do not introduce new "loop" text).

- [ ] **Step 3: Mutation probe** — restore the old Step 2 sentence in a backup-copy probe (Global Constraints): the row must redden; restore and re-run green.

- [ ] **Step 4: Budgets; run the focused check.** Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-auto-groom/SKILL.md internal/repoguard/ internal/assets/embedded/
git commit -m "fix(skills): auto-groom keeps its spec draft out of the .docket worktree"
```

---

### Task 14: Bug 2 — docket-review accepts gate-off evidence under `build.gate: off`

**Build tier:** standard

Spec worklist: *The four bugs* → 2. Review Focus line 1.

**Files:**
- Modify: `skills/docket-review/SKILL.md` (`## Verifying the build evidence`)
- Modify: `skills/docket-implement-next/SKILL.md` (Step 6 review payload line "It also contains the branch and base ref, …"; the triage paragraph's "The reviewer's `unverified-build-state` blocker is the one exception…"; the `build_gate: off` spellings → `build.gate: off`)
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

**Interfaces:**
- Produces: the review dispatch payload names "the resolved `build.gate` value"; docket-review keys its evidence rule on it.

- [ ] **Step 1: Write the failing check**

```go
	// 0502 bug 2: under build.gate off a skipped record is the expected build
	// state; under local (or an unstated gate) green is still required.
	{sentinel: "align_0502_review_gate_off", file: "skills/docket-review/SKILL.md",
		present: []string{
			"a `skipped` record carrying `reason: build-gate-off` is the expected build state, not a blocker",
			"When the payload names no `build.gate` value, treat it as `local`",
		}},
	{sentinel: "align_0502_review_gate_off", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"the resolved `build.gate` value"},
		absent:  []string{"build_gate: off"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 2: Fix**

docket-review evidence rule: the record must be present and parseable and carry a `head_sha`
equal to the reviewed HEAD; then: "When the dispatch payload names `build.gate: off`, a `skipped`
record carrying `reason: build-gate-off` is the expected build state, not a blocker. Under `local`
it must carry `result: green`. When the payload names no `build.gate` value, treat it as `local`."
Missing, malformed, stale, red, or a `skipped` record under `local` → the `unverified-build-state`
blocker as today. Keep "build-evidence" and "abort-and-report" (pinned by `test_docket_review`).
implement-next Step 6 payload line: "…relevant learnings hooks, the evidence record, and the
resolved `build.gate` value (from the startup-check prepared context's `build` block)." Fix the
`build_gate` spellings to `build.gate`.

- [ ] **Step 3: Mutation probe** — delete the gate-off sentence in a backup-copy probe: the row reddens; restore.

- [ ] **Step 4: Budgets; run the focused check.** Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-review/SKILL.md skills/docket-implement-next/SKILL.md internal/repoguard/ internal/assets/embedded/
git commit -m "fix(skills): docket-review accepts gate-off evidence when build.gate is off"
```

---

### Task 15: Bug 3 — finalize re-gates a repaired head through the gate driver

**Build tier:** premium (the spec names this as a gate-rules risk)

Spec worklist: *The four bugs* → 3.

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (`### 5. Repair a red gate`)
- Modify: `skills/docket-finalize-change/references/gate-failure.md` (`## The two agents …` item 2)
- Modify: `agents/docket-integration-repair.md` (the "the controller re-runs the gate on your repaired head through the `gate.launch`/`observe` operations" sentence) (+ regenerated goldens)
- Modify: `skills/docket-build/references/gate-driver.md` (caller list: add the finalize repair re-gate as a direct caller; the raw-verbs section stays)
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

**Interfaces:**
- Consumes: `gate.drive.start` (`--repo-dir --run-root --owner finalize --change-id --json`), `gate.drive.advance` (`--drive-id --owner-gen`), `evidence.record` (`--id --run --head`).

- [ ] **Step 1: Verify the driver path**

```bash
docket gate drive start --help; docket gate drive advance --help; docket evidence record --help
git grep -n -e '"finalize"' -- internal/app/gate_drive.go internal/cli/gate.go
```
Confirm `--owner finalize` resolves `finalize.test_command` and does not charge the build attempt
budget. Read whether `evidence.record` accepts a finalize-owned drive's raw run dir
(`git grep -n -e 'Owner' -- internal/app/evidence*.go`). If it refuses one, STOP the task as
BLOCKED and report it — never fall back to raw verbs.

- [ ] **Step 2: Write the failing check**

```go
	// 0502 bug 3: a repaired head is re-gated through the driver, never raw verbs.
	{sentinel: "align_0502_finalize_regate", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"`--owner finalize`", "`gate.drive.advance`"},
		absent:  []string{"`gate.launch` with `--root"}},
	{sentinel: "align_0502_finalize_regate", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"`--owner finalize`"},
		absent:  []string{"`gate.launch`/`observe` and records"}},
	{sentinel: "align_0502_finalize_regate", file: "agents/docket-integration-repair.md",
		present: []string{"gate driver"},
		absent:  []string{"`gate.launch`/`observe` operations"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 3: Fix**

Finalize step 5 (after the dispatch block's payload lines — keep the `docket:feature-dispatch`
block's shape; place the re-gate text immediately after `<!-- docket:feature-dispatch:end -->` as
its own paragraph if it reads better): "Then re-gate the repaired head through the gate driver:
the `gate.drive.start` operation with `--repo-dir <feature worktree> --owner finalize --change-id
<id> --run-root <dir> --json` — capture that first JSON response into `gate_reply` and the drive id
and owner generation from it, never a zsh read-only special parameter such as `status` — then the
`gate.drive.advance` operation with `--drive-id <id> --owner-gen <gen>`, one slice per call, until a
terminal disposition, under `docket-build`'s gate-run posture (a dispatched agent never yields). A
`PASSED` drive whose head equals the repaired head feeds the `evidence.record` operation with
`--id <id> --run <raw run dir from the PASSED document> --head <repaired head>`; `FAILED` returns to
repair within `finalize.repair_max_attempts`; `HALTED` is `halted`. The `--owner finalize` drive runs
`finalize.test_command` and does not charge the build attempt budget. The raw `gate.launch` /
`gate.observe` operations are operator primitives, never this re-gate." Mirror the same flow in
`gate-failure.md` item 2 and in `agents/docket-integration-repair.md` ("the controller re-gates
your repaired head through the gate driver (`--owner finalize`) and records the exact-head evidence
through the `evidence.record` operation"). `gate-driver.md`'s caller list names this finalize
re-gate as a direct caller. Regenerate goldens for `docket-integration-repair` only.

- [ ] **Step 4: Run the shape guards**

`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/`.
Expected: PASS (the gate.drive JSON-capture guard now counts the finalize site, which must carry
`--json`; the feature-dispatch guard still parses the step-5 dispatch block; the reserved-parameter
guard rejects a `status` capture variable).

- [ ] **Step 5: Budgets; run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md \
  agents/docket-integration-repair.md skills/docket-build/references/gate-driver.md \
  internal/harness/ internal/repoguard/ internal/assets/embedded/
git commit -m "fix(skills): finalize re-gates a repaired head through the gate driver"
```

---

### Task 16: Bug 4 — repair sign-off is the human's `finalize.clear-block`

**Build tier:** standard

Spec worklist: *The four bugs* → 4. Review Focus line 2.

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (`### 6. Sign-off on an authored repair`)
- Modify: `skills/docket-finalize-change/references/gate-failure.md` (`## Sign-off on auto-authored repairs`; the `## Finalize blocked` marker lifecycle bullet "A successful finalize removes the section…")
- Modify (tests): `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

**Interfaces:**
- Consumes: `finalize.publish` (`--id --attempt --head --evidence`), `finalize.block`, `finalize.clear-block` (`--id --revision --head --pr-number`).

- [ ] **Step 1: Verify**

```bash
docket finalize clear-block --help; docket finalize publish --help
sed -n 1,60p internal/app/finalize_block.go
git grep -n -e 'clear-block' -e 'ClearBlock' -- internal/app/finalize_closeout.go internal/app/finalize_merge.go | head
```
Confirm: `finalize.block` pushes nothing; `finalize.clear-block` refuses unless the remote feature
ref names the expected head (`remote-head-mismatch`/`remote-feature-absent`) and valid gate evidence
exists; `finalize.publish` has no finalize-blocked precondition. Determine whether anything other
than `finalize.clear-block` removes the section (merge? closeout?) and state only what the code does.

- [ ] **Step 2: Write the failing check**

```go
	// 0502 bug 4: a relayed sign-off is not authority; the human runs
	// finalize.clear-block on the published repaired head, then re-runs finalize.
	{sentinel: "align_0502_signoff", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"`finalize.clear-block`", "First publish the repaired head"},
		absent:  []string{"the human reviews the pushed repair on the PR and re-runs finalize"}},
	{sentinel: "align_0502_signoff", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"signs off by running the `finalize.clear-block` operation"},
		absent:  []string{"the retry clears the"}},
```
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 3: Fix**

Finalize step 6 autonomous bullet: "cannot prompt. First publish the repaired head (step 7's
`finalize.publish`), so the human can review it on the PR and `finalize.clear-block` can confirm
the published head. Then record the sign-off requirement durably and **stop**: the `finalize.block`
operation with … `--reason repair-needs-signoff --head <repaired head> …`. Disposition `halted`.
The human reviews the pushed repair, signs off by running the `finalize.clear-block` operation
themselves (`--id <id> --revision <revision> --head <repaired head> --pr-number <n>`), then re-runs
finalize. A re-run alone never clears the block, and a sign-off relayed through an agent's prompt
is not authority." Attended bullet: unchanged in substance (publish, prompt, on go-ahead clear and
merge). Mirror in `gate-failure.md` *Sign-off on auto-authored repairs*, and correct the marker
lifecycle bullet to what Step 1 found (for `repair-needs-signoff` the section is removed only by the
human's `finalize.clear-block`). Do not spell `docket finalize clear-block` (Global Constraints).

- [ ] **Step 4: Mutation probe** — reinstate the old autonomous sentence in a backup-copy probe; the row reddens; restore.

- [ ] **Step 5: Budgets; run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md \
  internal/repoguard/ internal/assets/embedded/
git commit -m "fix(skills): repair sign-off is the human running finalize.clear-block"
```

---

### Task 17: One metadata layout — the `docket` branch and the `.docket/` worktree

**Build tier:** standard

Spec worklist: *Terminology* → "One metadata layout" (about 40 sites).

**Files:**
- Modify: every in-scope file `git grep -l -E -e 'metadata_branch|docket-mode|repo-mode|In .docket.-mode' -- skills agents AGENTS.md` lists (expected: convention SKILL.md and close-out/learnings references, implement-next and edge-paths, adr, new-change, groom-next, auto-groom, status, finalize)
- Modify (tests): `internal/repoguard/budgets_test.go`, plus any test the pre-edit grep finds pinning a phrase you change (for example `TestProseContracts` rows naming "metadata working tree")

**Interfaces:** none.

- [ ] **Step 1: Write the failing check**

Append one row per affected skill file to `alignmentContracts`, sentinel `align_0502_one_layout`, each with
`absent: []string{"metadata_branch", "docket-mode", "repo-mode"}` (list the files from the grep).
Run `… -run 'TestProseContracts|TestAlignmentContracts'`. Expected: FAIL.

- [ ] **Step 2: Rewrite each site**

`metadata_branch` → "the `docket` branch" (or "`origin/docket`" where a ref is meant); "the metadata
working tree" may stay or become "the `.docket/` worktree" — check pinned phrases first
(`git grep -n -F -- 'metadata working tree' internal`). "In `docket`-mode all of the above lives on
the `docket` branch…" → "All of the above lives on the `docket` branch…". The *Bootstrap guard*
heading "(`docket`-mode first-run safety)" → "(first-run safety)" and "when `metadata_branch ==
docket`" → drop the condition. "`metadata_branch` only redirects bookkeeping commits" → "the
`docket` branch only receives bookkeeping commits". close-out's `## The sequence (docket-mode)` →
`## The sequence`. Re-run the grep (whitespace-collapsed:
`for f in $(git ls-files 'skills/*' agents AGENTS.md); do tr -s '[:space:]' ' ' < "$f" | grep -o -E -e 'metadata_branch|docket-mode|repo-mode' | sed "s|^|$f: |"; done`): no output.

- [ ] **Step 3: Budgets; run the focused check.** Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add skills/ internal/repoguard/ internal/assets/embedded/
git status --short   # only skill files with layout edits, tests, generated assets
git commit -m "docs(skills): one metadata layout — the docket branch and the .docket worktree"
```

---

### Task 18: "Loop" means `/loop` only

**Build tier:** standard

Spec worklist: *Terminology* → "Loop is reserved for `/loop`" (fix loop → fix pass; build loop / build-loop memory → builds / the learnings ledger; resolver loop → resolver rounds; repair loop → repair attempts; retry / push-retry loop → retries; poll / observe loop → polling; drain loop → drain; "loop to step 1" → "return to step 1"). Review Focus line 4.

**Files:**
- Modify: every in-scope file the sweep in Step 2 lists (expected: implement-next and its references, finalize and gate-failure, docket-build and its references, auto-groom, convention, close-out, groom-next, new-change, brainstorm, review)
- Modify (tests): `internal/repoguard/prose_contracts_test.go` — the `change_0411_recovery_exception_skill` section anchor `"### 3. Rebase onto the effective base (resolver loop)"` and the gate-failure present phrase `"an operator remedy, not an autonomous retry loop"`, plus any other pinned phrase containing "loop"; `internal/repoguard/budgets_test.go`; `internal/repoguard/gatedriver_test.go` only if a pinned heading changes

**Interfaces:**
- Produces: finalize heading `### 3. Rebase onto the effective base (resolver rounds)`; gate-driver heading `## The raw verbs are primitive/operator APIs, not caller verbs`.

- [ ] **Step 1: Inventory**

```bash
for f in $(git ls-files 'skills/*' 'agents/*.md' AGENTS.md); do
  tr -s '[:space:]' ' ' < "$f" | grep -o -i -E -e '[^ ]{0,30} loops?[^ ]{0,20}|[^ ]*-loop[^ ]*' | grep -v -F -e '/loop' | sed "s|^|$f: |"
done
git grep -n -i -e 'loop' -- internal/repoguard/*_test.go | grep -v -e 'for ' -e 'Loop(' | head -40
```
The second grep lists pinned phrases containing "loop"; each one you reword gets repointed.

- [ ] **Step 2: Repoint the pinned anchors first (failing)**

Change the `docSectionContract` section anchor to `"### 3. Rebase onto the effective base (resolver rounds)"`
and the gate-failure phrase to `"an operator remedy, not an autonomous retry"`. Run
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run 'TestRebaseRecoveryDocContracts'`.
Expected: FAIL (section heading not found).

- [ ] **Step 3: Rename every non-`/loop` sense**

Apply the spec's mapping. Shell-sense loops: "a **loop over per-file commands**" → "a **sequence
of per-file commands**"; "never a shell observe loop" → "never a hand-rolled polling script";
"launch/observe/sleep loop" → "launch/observe/sleep sequence". The convention's learnings
"build-loop memory" → "the learnings ledger". `fix-pass.md`: "fix loop" → "fix pass" throughout
(including its intro). implement-next Step 6: "a bounded **fix loop**" → "a bounded **fix pass**".
Keep every `/loop` mention. Re-run the Step 1 inventory: only `/loop` survives.

- [ ] **Step 4: Run the anchored rows, then mutation-probe one**

`… -run 'TestRebaseRecoveryDocContracts|TestResultsReviewPlacementDocContracts|TestProseContracts|TestAlignmentContracts'`: PASS.
Probe: in a backup copy of `skills/docket-finalize-change/SKILL.md`, rename the heading back to
`(resolver loop)` — `TestRebaseRecoveryDocContracts` must FAIL; restore with `mv -f`.

- [ ] **Step 5: Budgets; run the focused check.** Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add skills/ agents/ internal/harness/ internal/repoguard/ internal/assets/embedded/
git status --short   # agents/ and goldens only if an agent file had a non-/loop "loop"
git commit -m "docs(skills): reserve \"loop\" for /loop"
```
(If an `agents/*.md` file changed, regenerate its goldens first — Global Constraints.)

---

### Task 19: Drop change/PR citations and "Go v1" narration; keep only current ADR citations

**Build tier:** standard

Spec worklist: *Citations and history narration* (about 55 change citations, 2 `#NNN` in `learnings.md`, about 34 "Go v1" narrations, `AGENTS.md`'s line-wrapped "since change 0392"); alignment rule 4.

**Files:**
- Modify: every in-scope file the Step 1 scan lists, including `AGENTS.md` (the "Rebuild the binary after a merge to main" second bullet) and `skills/docket-implement-next/results-template.md` (its leading HTML comment cites changes)
- Modify (tests): `internal/repoguard/budgets_test.go`; any pinned phrase that carried a citation (for example `TestProseContracts` rows whose present phrase includes "(change 0136)"); `internal/app` results tests if `results-template.md` changes

**Interfaces:**
- Consumes: `scanCitations(rel, content string) []string` and `maskCode` from `internal/repoguard/docs_alignment_test.go` (unchanged here).

- [ ] **Step 1: Scan with the guard's own matcher (expected: many hits)**

Add a temporary test file `internal/repoguard/zz_citation_scan_test.go` (delete it before commit):
```go
package repoguard

import (
	"strings"
	"testing"
)

func TestZZCitationScan(t *testing.T) {
	root := guardRoot(t)
	files, err := livingDocFiles(root, []string{"skills", "agents", "AGENTS.md"})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range files {
		content := readMaintained(t, root, rel)
		for _, v := range scanCitations(rel, content) {
			t.Error(v)
		}
		flat := strings.Join(strings.Fields(content), " ")
		if n := strings.Count(flat, "Go v1"); n > 0 {
			t.Errorf("%s: %d Go v1 narration(s)", rel, n)
		}
	}
}
```
Run `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run TestZZCitationScan`. Expected: FAIL with the full worklist.

- [ ] **Step 2: Remove each citation and narration**

Keep the rule a citation carried; drop only the citation (`(change 0137)`, `changes 0064/0084`,
`(#219)`, "since change\n0392"). Delete "Go v1" narration sentences outright when they only narrate
history; when they also state a current fact, keep the fact in present tense. `AGENTS.md`'s bullet
becomes: "A merged change that extends the `.docket.yml` schema does not block this: the install
path tolerates unknown configuration keys (surfaced as warnings), so the tracked
`development.install` reinstall works directly with the pre-schema binary." (Edit only outside the
`docket:dispatch` block.) For every `ADR-NNNN` cited in an in-scope file, open
`docs/adrs/NNNN-*.md`: drop the citation when the ADR is not `Accepted` or its decision describes a
deferred or refused capability (ADR-0018 is dropped; ADR-0022 stays). Pinned phrases that carried a
citation are repointed to the de-cited wording in the same commit. If `results-template.md`
changed, also run `timeout --kill-after=10s 10m go test -count=1 ./internal/app/ -run 'Results'`.

- [ ] **Step 3: Re-run the scan** — `TestZZCitationScan` PASS. Delete `internal/repoguard/zz_citation_scan_test.go`.

- [ ] **Step 4: Budgets; regenerate goldens if an agent file changed; run the focused check.** Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add AGENTS.md skills/ agents/ internal/harness/ internal/repoguard/ internal/app/ internal/assets/embedded/
git status --short   # no zz_citation_scan_test.go; only intended files
git commit -m "docs(skills): drop change citations and Go v1 narration from the agent-facing files"
```

---

### Task 20: Extend `TestLivingDocsAlignment` to `skills/`, `agents/`, and `AGENTS.md`; acceptance sweep

**Build tier:** standard

Spec: *Guards and pinned tests* (extend the guard; key check skips YAML frontmatter only; mutation-tested; population floor) and *Acceptance*. Review Focus lines 3 and 5.

**Files:**
- Modify: `internal/repoguard/docs_alignment_test.go`
- Modify: any in-scope file the sweep or the extended guard still flags (residue fixes)

**Interfaces:**
- Produces: `blankFrontmatter(content string) string`; `livingDocRoots` gains `"skills"`, `"agents"`, `"AGENTS.md"`.

- [ ] **Step 1: Acceptance sweep (record the output for the coordinator's results file)**

```bash
cd "$(git rev-parse --show-toplevel)"
files=$(git ls-files 'skills/*.md' 'skills/**/*.md' 'agents/*.md' AGENTS.md | sort -u)
for f in $files; do
  flat=$(tr -s '[:space:]' ' ' < "$f")
  for pat in 'SKILL_[A-Z]' 'DUMMY_MODE' 'AUTO_CAPTURE' 'REVIEW_MIN_FIX_SEVERITY' 'REVIEW_MAX_FIX_TASKS' \
             'GATE_OBSERVATION_BUDGET' 'BUILD_CHECKPOINT' 'metadata_branch' 'docket-mode' 'repo-mode' \
             'stack-base\.sh' 'disable-worktree-hooks' 'verify-run' 'fm_field' 'reclaim-claims' \
             'run-halt([^e]|$)' 'publish-deferred' 'adr-unpublished' 'stack-invalid' 'stack-parent-killed' \
             'board off' 'promote-failed' 'stack-carried-failed' 'Go v1' 'auto-or-halt' 'scripts/<name>' \
             'git worktree add' 'gh pr edit'; do
    n=$(grep -o -E -e "$pat" <<<"$flat" | wc -l | tr -d ' ')
    [ "$n" != 0 ] && echo "$f: $pat x$n"
  done
  grep -o -i -E -e '[^ ]*loop[^ ]*' <<<"$flat" | grep -v -F -e '/loop' | sed "s|^|$f: loop-word |"
done
# Links: every relative link in the in-scope files resolves.
for f in $files docs/release/four-harness-acceptance.md; do
  d=$(dirname "$f")
  links=$(grep -o -E -e '\]\([^)#[:space:]]+' "$f" | sed 's/^](//')
  for l in $links; do
    case "$l" in http*|mailto:*) continue;; esac
    [ -e "$d/$l" ] || echo "BROKEN $f -> $l"
  done
done
# Nothing links to a deleted or renamed file (point-in-time records excluded).
git grep -n -E -e 'dummy-mode\.md|gate-execution(-evidence)?\.md|fix-loop\.md|gate-caller-loop\.md|adr-template\.md|change-template\.md' \
  -- . ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' ':!**/testdata/**' ':!internal/install/legacydata/**' ':!internal/repoguard/budgets_test.go'
# A consuming repo must not read docket-only facts (Review Focus 5).
git grep -n -E -e 'internal/[a-z]|go run \./cmd/docket|this repo' -- skills
```
Expected: no output from the first three blocks (fix any residue in this task). Every hit of the
last grep must be conditioned on docket's own repository or removed. Save the full output in your
task report.

- [ ] **Step 2: Extend the guard (failing first)**

In `internal/repoguard/docs_alignment_test.go`:
```go
var livingDocRoots = []string{
	"README.md",
	"docs/README.md",
	"docs/guide",
	"docs/install",
	"docs/concepts",
	"docs/reference",
	// Agent-facing: skill bodies and references, agent wrappers, and the
	// always-loaded AGENTS.md (its generated dispatch block included).
	"skills",
	"agents",
	"AGENTS.md",
}

// blankFrontmatter blanks a leading YAML frontmatter block — the first line
// is exactly "---" and a later line is exactly "---" — keeping every newline
// so reported line numbers stay true. Agent wrappers and skills carry a real
// skills list there; nothing else is skipped. A file without a closed leading
// block is returned unchanged and scanned in full (fail-safe).
func blankFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if strings.TrimRight(lines[0], "\r") != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			for j := 0; j <= i; j++ {
				lines[j] = ""
			}
			return strings.Join(lines, "\n")
		}
	}
	return content
}
```
In the main loop: `scanUnsupportedKeys(rel, blankFrontmatter(content), shapes)` (citations still
scan the whole file). Update the header comment: the guard covers the living human-facing docs and
the agent-facing files; the key check skips leading YAML frontmatter only; `docs/release/` stays
excluded. Raise `livingDocFloor` to the measured file count (print it once with a temporary
`t.Logf("%d", len(files))`, then remove the log). Add to `non_vacuity`:
```go
		if got := scanUnsupportedKeys("agents/x.md", blankFrontmatter("---\nname: x\nskills: [docket-review]\n---\nbody\n"), shapes); len(got) != 0 {
			t.Errorf("a wrapper's frontmatter skills list was flagged: %v", got)
		}
		if len(scanUnsupportedKeys("agents/x.md", blankFrontmatter("---\nname: x\n---\nbind `skills.build` to it\n"), shapes)) == 0 {
			t.Errorf("an unsupported key in a wrapper body was not flagged")
		}
		if len(scanUnsupportedKeys("x.md", blankFrontmatter("intro\n---\nskills:\n  build: x\n---\n"), shapes)) == 0 {
			t.Errorf("a frontmatter-shaped block that does not open the file was skipped")
		}
		if len(scanUnsupportedKeys("x.md", blankFrontmatter("---\nskills:\n  build: x\n"), shapes)) == 0 {
			t.Errorf("an unclosed leading block was skipped")
		}
		if got := scanUnsupportedKeys("x.md", blankFrontmatter("---\na: 1\n---\nset `skills.build` here\n"), shapes); len(got) != 1 || !strings.Contains(got[0], "x.md:4:") {
			t.Errorf("blanking moved line numbers: %v", got)
		}
		if len(scanCitations("x.md", "---\ndescription: see change 0363\n---\n")) == 0 {
			t.Errorf("a citation in frontmatter was not flagged")
		}
		for _, r := range []string{"skills", "agents", "AGENTS.md"} {
			if !slices.Contains(livingDocRoots, r) {
				t.Errorf("living doc roots lost the agent-facing root %q", r)
			}
		}
```
(add `"slices"` to the imports). Run
`timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run TestLivingDocsAlignment`.
If it fails on real files, fix that residue in the named files (re-measure their budgets), never by
widening the guard. Expected after fixes: PASS.

- [ ] **Step 3: Mutation probes (each must redden, then restore green)**

1. Citation in a skill body: `f=skills/docket-status/SKILL.md; cp "$f" "$f.bak"; printf '\nSee change 0363 for history.\n' >> "$f"` → FAIL; `mv -f "$f.bak" "$f"`.
2. Registry-derived key in an agent wrapper body: `f=agents/docket-status.md; cp "$f" "$f.bak"; printf '\nBind `skills.build` here.\n' >> "$f"` → FAIL; restore.
3. Same key in agent frontmatter does not redden: the real wrappers already carry a skills list in
   frontmatter and the guard is green. Prove the skip is what keeps it green:
   `f=internal/repoguard/docs_alignment_test.go; cp "$f" "$f.bak"`, change
   `scanUnsupportedKeys(rel, blankFrontmatter(content), shapes)` to
   `scanUnsupportedKeys(rel, content, shapes)` in the main loop → FAIL on `agents/*.md`
   frontmatter; `mv -f "$f.bak" "$f"`.
4. Population floor: temporarily drop `"agents"` from `livingDocRoots` → FAIL (floor or the roots non-vacuity check); restore.
Each probe uses the backup-copy procedure and `-count=1`.

- [ ] **Step 4: Run the focused check and the whole-suite smoke**

Focused check (Global Constraints): PASS. Then
`timeout --kill-after=10s 10m go test -count=1 ./internal/app/ -run 'Results'`: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repoguard/docs_alignment_test.go internal/repoguard/budgets_test.go
git add <every residue-fixed file> internal/assets/embedded/
git status --short
git commit -m "test(repoguard): extend the living-docs guard to skills, agents, and AGENTS.md"
```
