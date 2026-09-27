<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0455 — Document finalize's record-invalid reason in the docket-finalize-change skill](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0455-document-finalize-s-record-invalid-reason-in-the-docket-fina.md)**
<!-- docket:backlink:end -->
# Document the `record-invalid` refusal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: execute with the `docket-build` build role (task-by-task via its profile agents, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Document the `record-invalid` refusal of `finalize.merge` (in `docket-finalize-change` step 8) and `pr.publish` (in `docket-implement-next` "Publish the PR"), and fix the unrelated `gofmt` drift in `internal/githubcli/comment_integration_test.go`.

**Architecture:** Prose-only edits to two distributed skill bodies, appended inside the existing paragraphs so they add no lines. Each edit comes with the matching word-ceiling bump in `internal/repoguard/budgets_test.go` (both files are exactly at their word ceilings today) and a regenerated embedded asset bundle (`internal/assets/embedded/`). The gofmt fix is formatting only.

**Tech Stack:** Markdown skill bodies, Go (`go generate` asset bundle, `TestSkillSizeBudgets`), the toolchain-pinned `gofmt`.

**Spec:** `docs/superpowers/specs/2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina-design.md` (on the `docket` branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina-design.md`)

## Global Constraints

- No behavior change to either operation, to the reason token `record-invalid`, or to the `record-invalid`-before-`already-merged` precedence.
- No new repoguard test that pins skill reason tokens (YAGNI, per the spec's non-goals).
- Skill bodies ship into other repos, so the added prose must not name this repo's source files (`internal/app/...`), line numbers, or local-only facts. Name the operation, the reason token, and the result fields.
- Cross-references anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- Budgets are currently at the ceiling: `docket-finalize-change/SKILL.md` is 238 lines / 5520 words (ceiling 239 / 5520), and `docket-implement-next/SKILL.md` is 214 lines / 8025 words (ceiling 214 / 8025). Add **no new lines**: append to the existing paragraph line. Raise only the word ceiling, and set it to the exact post-edit `wc -w` count.
- Stage explicit paths only. Never `git add -A`.
- Format with the toolchain gofmt, never PATH's: `"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt"`.

## Review Focus

1. **Scope wording.** The prose must say the check covers the change, its `depends_on` targets, and its stack ancestors, and never associative links (`related`, `discovered_from`, ADRs). Wording that suggests any broken record in the corpus can trigger the refusal is wrong.
2. **Remedy wording.** Repair exactly the records the result's `findings` name, then re-run. Nothing may suggest guessing at the culprit, editing unnamed records, or an override flag. None exists.
3. **Merged-outside-docket case.** It must read as expected behavior (closeout would refuse the same defect), and the text must say that after the repair the re-run takes the merged-recovery path.
4. **Budget honesty.** Each ceiling bump equals the measured count and carries a `0455:` comment. Neither file may cross its line ceiling.
5. **Bundle drift.** `go run ./cmd/genassets -check` passes after every skill edit. An edited `skills/` file without a regenerated `internal/assets/embedded/tree/skills/...` copy reddens the suite.

---

### Task 1: Document `record-invalid` in `docket-finalize-change` step 8

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`, section `### 8. Merge exactly once`, first paragraph (the one beginning "The `finalize.merge` operation with `--id <id> --version <version> --head <head>`")
- Modify: `internal/repoguard/budgets_test.go`, the `{"docket-finalize-change/SKILL.md", 239, 5520}` row of `skillBudgets`
- Regenerate: `internal/assets/embedded/` (manifest + `tree/skills/docket-finalize-change/SKILL.md`)
- Decided, no edit: `skills/docket-finalize-change/references/gate-failure.md`. Its abort-and-report set already covers merge refusals generically ("a merge conjunct that fails at the fresh recheck … returns the conjunct's token"). It lists no individual pre-effect refusal token, and the sibling `merge-method-unavailable` is absent too. The spec says not to add a line only for symmetry. Its budget (145/147 lines, 1892/1901 words) also has no real headroom.

**Interfaces:**
- Consumes: nothing.
- Produces: nothing code-facing.

- [ ] **Step 1: Confirm the gap (the failing check)**

Run from the worktree root:
```bash
grep -c -- 'record-invalid' skills/docket-finalize-change/SKILL.md
```
Expected: `0` (exit 1).

- [ ] **Step 2: Append the prose to the step-8 paragraph**

Insert the following text into the same line, directly after the sentence that ends "… it is not `merge-denied` and is never retried with another method." and before "It merges at the exact expected head". Keep a single space on each side and add no newline:

```text
Before any GitHub call it also validates the change and the records it structurally requires — its `depends_on` targets and stack ancestors, never `related`, `discovered_from`, or ADR links — and refuses `blocked` with reason `record-invalid` (`halted`) when any carries a validation error; the result's `findings` name each bad record's code and path. No merge call was made: repair exactly the records `findings` name — never guess, never edit an unnamed record; no override exists — then re-run finalize. Because this check precedes already-merged recovery, a PR merged outside docket whose scope holds a defective record reports `record-invalid`, not `already-merged`; that is expected (closeout would refuse the same defect), and once repaired the re-run takes the merged-recovery path.
```

- [ ] **Step 3: Verify the prose and measure**

```bash
f=skills/docket-finalize-change/SKILL.md
flat=$(tr -s '[:space:]' ' ' < "$f")
grep -qF -- 'refuses `blocked` with reason `record-invalid`' <<<"$flat" && echo scope-ok
grep -qF -- 'repair exactly the records `findings` name' <<<"$flat" && echo remedy-ok
grep -qF -- 'reports `record-invalid`, not `already-merged`' <<<"$flat" && echo merged-ok
wc -l < "$f"; wc -w < "$f"
```
Expected: `scope-ok`, `remedy-ok`, and `merged-ok` all print. The line count is still `238`. Record the word count as `W1` (about 5630).

- [ ] **Step 4: Raise the word ceiling to exactly `W1`**

In `internal/repoguard/budgets_test.go`, change the row `{"docket-finalize-change/SKILL.md", 239, 5520},` to `{"docket-finalize-change/SKILL.md", 239, <W1>},` and put `0455: +record-invalid refusal (structural scope, findings remedy, merged-outside-docket precedence) in step 8 (word ceiling 5520 -> <W1>); ` at the start of that row's trailing comment. Leave the rest of the comment unchanged. Then run the toolchain gofmt on the file:
```bash
"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -w internal/repoguard/budgets_test.go
```

- [ ] **Step 5: Regenerate the embedded bundle**

```bash
go generate ./internal/assets && go run ./cmd/genassets -check
```
Expected: `-check` exits 0. `git status --porcelain` shows only the skill, the budgets file, `internal/assets/embedded/manifest.json`, and `internal/assets/embedded/tree/skills/docket-finalize-change/SKILL.md`.

- [ ] **Step 6: Run the guards**

```bash
go test -count=1 ./internal/repoguard/ ./internal/assets/
```
Expected: PASS. Mutation check: temporarily set the ceiling to `<W1>-1`, rerun `go test -count=1 -run TestSkillSizeBudgets ./internal/repoguard/`, see it fail, then restore it.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md internal/repoguard/budgets_test.go internal/assets/embedded/manifest.json internal/assets/embedded/tree/skills/docket-finalize-change/SKILL.md
git commit -m "docs(skills): document finalize.merge record-invalid refusal in finalize step 8 (change 0455)"
```

### Task 2: Document `record-invalid` in `docket-implement-next` "Publish the PR"

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`, the `**Publish the PR.**` paragraph (the one containing "the `pr.publish` operation with `--id <id> --head <feature head>`")
- Modify: `internal/repoguard/budgets_test.go`, the `{"docket-implement-next/SKILL.md", 214, 8025}` row
- Regenerate: `internal/assets/embedded/` (manifest + `tree/skills/docket-implement-next/SKILL.md`)

**Interfaces:**
- Consumes: the Task 1 commit (the budgets file and manifest already carry its edits).
- Produces: nothing code-facing.

- [ ] **Step 1: Confirm the gap**

```bash
grep -c -- 'record-invalid' skills/docket-implement-next/SKILL.md
```
Expected: `0`.

- [ ] **Step 2: Append the clause to the end of the paragraph line**

After the paragraph's last sentence ("… and the result redacts the body bytes."), on the same line after one space, append:

```text
It refuses `record-invalid` (`invalid-state`) before any GitHub call when the change, a `depends_on` target, or a stack ancestor carries a validation error (associative links are never checked): no PR was created or edited — repair exactly the records the result's `findings` name, then re-publish; the run keeps its existing halt posture for a typed refusal.
```

- [ ] **Step 3: Verify and measure**

```bash
f=skills/docket-implement-next/SKILL.md
flat=$(tr -s '[:space:]' ' ' < "$f")
grep -qF -- 'It refuses `record-invalid` (`invalid-state`) before any GitHub call' <<<"$flat" && echo scope-ok
grep -qF -- 'repair exactly the records the result'"'"'s `findings` name, then re-publish' <<<"$flat" && echo remedy-ok
wc -l < "$f"; wc -w < "$f"
```
Expected: `scope-ok` and `remedy-ok` print. The line count is still `214`. Record the word count as `W2` (about 8080).

- [ ] **Step 4: Raise the word ceiling to exactly `W2`**

Change `{"docket-implement-next/SKILL.md", 214, 8025},` to `{"docket-implement-next/SKILL.md", 214, <W2>},` and put `0455: +pr.publish record-invalid refusal clause (word ceiling 8025 -> <W2>); ` at the start of its trailing comment. Run the toolchain gofmt on the file, as in Task 1 Step 4.

- [ ] **Step 5: Regenerate and check the bundle**

```bash
go generate ./internal/assets && go run ./cmd/genassets -check
```
Expected: exit 0. The changed paths are only the skill, the budgets file, the manifest, and `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md`.

- [ ] **Step 6: Run the guards**

```bash
go test -count=1 ./internal/repoguard/ ./internal/assets/
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/SKILL.md internal/repoguard/budgets_test.go internal/assets/embedded/manifest.json internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md
git commit -m "docs(skills): document pr.publish record-invalid refusal in implement-next (change 0455)"
```

### Task 3: Fix gofmt drift in `internal/githubcli/comment_integration_test.go`

**Files:**
- Modify: `internal/githubcli/comment_integration_test.go` (formatting only)

- [ ] **Step 1: Confirm the drift**

```bash
G="$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt"
"$G" -l internal/githubcli/
```
Expected: prints `internal/githubcli/comment_integration_test.go`.

- [ ] **Step 2: Reformat**

```bash
"$G" -w internal/githubcli/comment_integration_test.go
```

- [ ] **Step 3: Verify the diff is whitespace-only**

```bash
"$G" -l internal/githubcli/          # expected: no output
git diff -w --stat -- internal/githubcli/comment_integration_test.go   # expected: empty (whitespace-only change)
go vet -tags integration ./internal/githubcli/
```
Expected: `gofmt -l` prints nothing, `git diff -w` is empty, and vet passes. If `git diff -w` is not empty (for example, gofmt reordered or aligned something non-whitespace), inspect the diff and confirm it is gofmt's own canonical output before continuing.

- [ ] **Step 4: Commit**

```bash
git add internal/githubcli/comment_integration_test.go
git commit -m "style(githubcli): gofmt comment_integration_test.go (change 0455)"
```

### Build gate (docket-build, after all tasks)

Run the full suite with the command `build.test_command` resolves to from config, entered from source. Read the budget report even when the run is green.
