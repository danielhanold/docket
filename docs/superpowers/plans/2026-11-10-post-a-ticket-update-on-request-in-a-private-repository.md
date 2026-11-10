<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0586 — Post a ticket update on request in a private repository](../../changes/archive/2026-11-10-0586-post-a-ticket-update-on-request-in-a-private-repository.md)**
<!-- docket:backlink:end -->

# Post a ticket update on request in a private repository Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` (task-by-task, one commit per task, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In a private repository, `docket-update-ticket` stops refusing and delivers the update as cleaned attachments plus a comment, with attachments first and the comment last.

**Architecture:** Prose-only change. A new `skills/docket-update-ticket/references/private.md` (sibling of `references/shared.md`) carries private delivery; `SKILL.md` drops the private refusal and routes `visibility: private` to the new reference. Prose sentinels in `internal/repoguard` pin the load-bearing sentences, the reference-existence check covers every reference `SKILL.md` routes to, budgets are re-pinned, and the embedded asset tree is regenerated. No binary change: `context.ticket` already returns `visibility` and an empty `url` per artifact in a private repository, and `docket-export`'s file-request path already returns the cleaned file and its before → after list.

**Tech Stack:** Markdown skill bodies; Go tests (`internal/repoguard`, `internal/assets`); `go generate`.

**Spec:** `docs/superpowers/specs/2026-11-10-post-a-ticket-update-on-request-in-a-private-repository-design.md` (on the metadata branch; executors read it alongside this plan).

## Global Constraints

- No change to `context.ticket`, `docket-export`, the leak-check patterns, or any Go code outside `internal/repoguard/*_test.go` and the regenerated `internal/assets/embedded` tree.
- Skill bodies ship into other repositories: no sentence may be true only in this repository (no "this repo", no sandbox site or project names).
- No ticket-system call is made from this repository, and no sandbox site or project name is written anywhere in it.
- Everything else in `docket-update-ticket` is unchanged: the read, the other refusals, the per-update table rows, the late-update rule, the editorial standard, the layout guidance, mentions, one connector call at a time.
- Plans never go out (shared and private alike).
- Mutation probes: back the file up with `cp "$f" "$f.bak"`, mutate, run with `go test -count=1`, restore with `mv -f "$f.bak" "$f"`. Never restore with `git checkout --` (it reverts the uncommitted edit under test). Confirm each mutation landed with a before/after count taken on a whitespace-collapsed copy (`tr -s '[:space:]' ' ' < "$f" | grep -o -F -- "<phrase>" | wc -l`), since guarded phrases may wrap.
- Prose guards match on a whitespace-collapsed haystack (`collapseWS`), so re-flowing a paragraph never reddens them; keep every pinned phrase as one clause bound to its claim.
- Every `mktemp` is templated: `"${TMPDIR:-/tmp}/<name>.XXXXXX"`.
- Build gate: `build.test_command` from `.docket.yml` (`go run ./cmd/docket development test`), run from the feature-worktree root. Read the budget report even when green.

## Review Focus

1. **The refusal is gone but its pin was only moved, not inverted.** A reviewer should see the old refusal string in an `absent` list; if it were merely deleted from `present`, re-adding the refusal would stay green. Task 1 puts it in the `change_0585_update_ticket_skill` row's `absent` list and mutation-tests it.
2. **`SKILL.md` routes to a reference that does not exist.** A typo in `references/private.md` leaves a blocking read with nothing to read. Task 1 derives every routed reference from `SKILL.md` itself and stats each one, with a population floor naming both routes.
3. **A rewording keeps the words but drops the rule** (for example "attachments first" survives while "any failed upload posts no comment" is softened to "may"). Task 1 pins each rule as a full clause, not a keyword, and mutation-tests each pin.
4. **The convention paragraph still claims "no docket write"** while private delivery invokes `docket-export`, which writes a temp file and may read `diagnostic.config`. Task 1 pins the new clause and puts the old sentence in `absent`.
5. **Docs keep saying "shared repositories only"** after the skill learned private delivery. Task 2 pins both stale doc phrases as `absent`.

---

### Task 1: Private delivery reference, SKILL.md route, and their guards

**Files:**
- Create: `skills/docket-update-ticket/references/private.md`
- Modify: `skills/docket-update-ticket/SKILL.md` (convention paragraph; section 2 refusal list; section 3 table header plus one note line; section 4)
- Test: `internal/repoguard/prose_contracts_test.go` (`alignmentContracts`: the `change_0585_update_ticket_skill` SKILL.md row, plus two new `change_0586_update_ticket_private` rows)
- Test: `internal/repoguard/update_ticket_skill_test.go` (`TestUpdateTicketSkillIsInline`)

**Interfaces:**
- Consumes: existing helpers `readRepoFile(t, rel) string`, `guardRoot(t) string`, `collapseWS(string) string`, the `proseContract{sentinel, file, present, absent}` row shape in `alignmentContracts`.
- Produces: the file `skills/docket-update-ticket/references/private.md` and the routing sentence ``When `visibility` is `private`, **read `references/private.md` now (blocking)**.`` (Task 3 budgets both files).

- [ ] **Step 1: Write the failing guards**

In `prose_contracts_test.go`, in the `change_0585_update_ticket_skill` row for `skills/docket-update-ticket/SKILL.md`:
- remove ``"`visibility` is `private`: say private repositories are not supported yet",`` from `present`;
- add `"private repositories are not supported yet"` to that row's `absent` list.

Directly after the `change_0585_update_ticket_skill` rows for `references/shared.md`, add (with a comment `// change 0586 — private delivery: the refusal is replaced by a blocking route to references/private.md; attachments first, comment last; any failed upload posts no comment; the docket-export file request; the quiet self-fix; one review stop only on a rewrite.`):

```go
{sentinel: "change_0586_update_ticket_private", file: "skills/docket-update-ticket/SKILL.md",
	present: []string{
		"When `visibility` is `private`, **read `references/private.md` now (blocking)**.",
		"This skill makes no docket metadata write.",
		"in a private repository it also invokes `docket-export`, which writes each exported file to a temp directory and may read `leak_check.match_word` through `diagnostic.config`",
		"The last column names what goes with the update: linked in a shared repository, attached in a private one.",
	},
	absent: []string{"This skill makes no docket write."}},
{sentinel: "change_0586_update_ticket_private", file: "skills/docket-update-ticket/references/private.md",
	present: []string{
		"Loaded from `SKILL.md` when `visibility` is `private`. The rules in `SKILL.md` apply; they are not repeated here.",
		"The PR link stays a link: it is `facts.pr_url`, as in shared delivery.",
		"A PR-opened update attaches the spec even when an earlier update already attached it.",
		"An artifact the update names but `artifacts` does not return is left out, and the comment does not mention it.",
		"invoke the `docket-export` skill as a **file request**",
		"docket-update-ticket.XXXXXX",
		"return the exported path and the before → after list, and post nothing",
		"A failed export posts nothing: no attachment and no comment",
		"A word-check hit inside the export stops the way `docket-export` stops",
		"Write it under the private-repository writing rule",
		"`Spec: attached <filename>`, `Results: attached <filename>`",
		"search the comment and every attachment's file name for \"docket\" and \"dckt\" as whole words, case-insensitive",
		"fix your own text quietly: reword the comment, or rename the exported file in its temp directory",
		"Your own words get no before → after.",
		"If any export rewrote something, stop once: show the before → after list for each attachment and the names of the files to be attached, and post on the person's go.",
		"If nothing was rewritten, post straight away",
		"**Attachments first, comment last.**",
		"Never assume a one-call upload",
		"`uploadAttachmentToJiraIssue` first returns an upload command to run from the shell",
		"If you cannot run that step, the upload has failed.",
		"Never print a credential",
		"**Any failed upload posts no comment.**",
		"A failed or uncertain upload means no comment, and never substitute a local path for an attachment.",
		"which attachments already landed on the ticket (never delete from a ticket)",
		"the exported files' paths so the person can attach them by hand",
		"the update as posted only with the connector's confirmation, naming the ticket, the attachments, and anyone mentioned.",
	}},
```

In `update_ticket_skill_test.go`, replace the single `shared.md` stat in `TestUpdateTicketSkillIsInline` with a derived check: extract every match of `` regexp.MustCompile(`references/([a-z0-9-]+\.md)`) `` from `SKILL.md`, dedupe, and for each name `os.Stat(filepath.Join(guardRoot(t), "skills", name, "references", ref))`, failing with `"%s routes to references/%s, which cannot be read: %v"`. Population floor: the extracted set must contain both `shared.md` and `private.md` (fail with `"%s routes to no references/%s; each visibility needs its delivery reference"`). Update the function's doc comment to say it checks every reference `SKILL.md` routes to.

- [ ] **Step 2: Run the guards and watch them fail**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestAlignmentContracts|TestUpdateTicketSkillIsInline'`
Expected: FAIL — `[change_0585_update_ticket_skill]` reports the absent refusal phrase present; `[change_0586_update_ticket_private]` reports `references/private.md` unreadable (fail closed) or the SKILL.md phrases missing; `TestUpdateTicketSkillIsInline` reports no `references/private.md` route.

- [ ] **Step 3: Write `skills/docket-update-ticket/references/private.md`**

Exact copy (the sentinels pin it):

```markdown
# Private delivery

Loaded from `SKILL.md` when `visibility` is `private`. The rules in `SKILL.md` apply; they are not repeated here. In a private repository nothing can be linked: each `artifacts` entry's `url` is empty and the metadata branch is local. So each file the update names goes to the ticket as an attachment, and the comment names it.

- **Which files.** The ones the update's row in `SKILL.md` names, attached instead of linked: the spec, the results file, or the artifacts a revision, blocker, or kill discusses. The PR link stays a link: it is `facts.pr_url`, as in shared delivery. A PR-opened update attaches the spec even when an earlier update already attached it. An artifact the update names but `artifacts` does not return is left out, and the comment does not mention it.
- **Export each file.** For each one, invoke the `docket-export` skill as a **file request** — its rules are its own; do not restate or re-run them — **DIRECTED to:** export `<role> <id>` (`spec` or `results`) with `--output-path` set to a fresh directory from `mktemp -d "${TMPDIR:-/tmp}/docket-update-ticket.XXXXXX"`, return the exported path and the before → after list, and post nothing. You do the uploading and posting. A failed export posts nothing: no attachment and no comment; report the failed export. A word-check hit inside the export stops the way `docket-export` stops: show the person the path and the lines, and post nothing until they decide.
- **Write the comment clean.** Write it under the private-repository writing rule: no change ids or change references (`#612`), no ADR numbers, no backlog or board references, and no "docket" or "dckt". Name each attachment, labeled like a link: `Spec: attached <filename>`, `Results: attached <filename>`.
- **Check your own words.** Before posting, search the comment and every attachment's file name for "docket" and "dckt" as whole words, case-insensitive (the bare word "docket" is exempt when `leak_check.match_word` is `false`, read as `docket-export` reads it). On a hit, fix your own text quietly: reword the comment, or rename the exported file in its temp directory. Your own words get no before → after.
- **When to ask.** If any export rewrote something, stop once: show the before → after list for each attachment and the names of the files to be attached, and post on the person's go. If nothing was rewritten, post straight away; when the person asks to see the update first, show it and post on their go.
- **Attachments first, comment last.** Upload each attachment to `ticket.id`, one connector call at a time, then post the comment. Never assume a one-call upload: some connectors attach in two steps. The Atlassian connector's `uploadAttachmentToJiraIssue` first returns an upload command to run from the shell, then attaches the file id that command returns; it may be reachable only through the connector's discovery tool (`discover`). If you cannot run that step, the upload has failed. Never print a credential: when the upload command carries a token, run it; never echo it into the report.
- **Any failed upload posts no comment.** Every attachment the update names, and whose file exists, is required. A failed or uncertain upload means no comment, and never substitute a local path for an attachment. Report in one plain line the ticket, which upload failed and why, which attachments already landed on the ticket (never delete from a ticket), and the exported files' paths so the person can attach them by hand: `Not posted: the results upload to CLOUD-123 failed (connector error). The spec is already attached there; delete it, or ask again. Exported files: <paths>.` Asking again uploads a landed file a second time; that duplicate is acceptable.
- **Report** the update as posted only with the connector's confirmation, naming the ticket, the attachments, and anyone mentioned.
```

Hard-wrapping is the builder's choice; the guards collapse whitespace but do not strip `**`, so keep bold markers exactly where the pins expect them (the last bullet's pin starts after `**Report**`).

- [ ] **Step 4: Edit `skills/docket-update-ticket/SKILL.md`**

1. Convention paragraph: replace `This skill makes no docket write. Its reads are the `context.ticket` operation and, for a request naming only a ticket, the `status` operation (section 1).` with
   `This skill makes no docket metadata write. Its reads are the `context.ticket` operation and, for a request naming only a ticket, the `status` operation (section 1); in a private repository it also invokes `docket-export`, which writes each exported file to a temp directory and may read `leak_check.match_word` through `diagnostic.config` (section 4).`
   (keeps the 0585-pinned clause "Its reads are the `context.ticket` operation and, for a request naming only a ticket, the `status` operation" intact).
2. Section 2: delete the bullet ``- `visibility` is `private`: say private repositories are not supported yet.``
3. Section 3 table: rename the header cell `Links` to `Goes with it` (header row becomes `| Update | It says | Goes with it |`). Directly under the table add the line: `The last column names what goes with the update: linked in a shared repository, attached in a private one.` Leave every row cell unchanged (0585 pins `| the PR, the spec, and the results file |`).
4. Section 4: rename the heading to `## 4. Links and attachments` and add, after the shared sentence, the line: ``When `visibility` is `private`, **read `references/private.md` now (blocking)**.``

- [ ] **Step 5: Run the guards and watch them pass**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestAlignmentContracts|TestUpdateTicketSkillIsInline|TestUpdateTicketSkillExamplesUseNoTypedMention'`
Expected: PASS. (`TestSkillSizeBudgets` and `internal/assets` `TestEmbeddedMatchesAuthored` are expected red until Task 3; do not run the full suite here.)

- [ ] **Step 6: Mutation-test each new guard**

Each probe: `cp "$f" "$f.bak"`; apply the mutation; confirm it landed (whitespace-collapsed count before ≥1, after 0, or the file is gone); run `go test -count=1 ./internal/repoguard/ -run '<test>'`; expect FAIL naming the probe; `mv -f "$f.bak" "$f"`; re-run, expect PASS.

| # | File | Mutation | Must redden |
|---|---|---|---|
| a | `SKILL.md` | re-add the section-2 bullet ``- `visibility` is `private`: say private repositories are not supported yet.`` | `TestAlignmentContracts` `[change_0585_update_ticket_skill]` absent |
| b | `SKILL.md` | delete the private routing sentence | `TestAlignmentContracts` `[change_0586…]` and `TestUpdateTicketSkillIsInline` floor |
| c | `SKILL.md` | change `references/private.md` in the routing sentence to `references/privat.md` | `TestUpdateTicketSkillIsInline` stat |
| d | `references/private.md` | move it aside (`mv -f` to the backup) | `TestUpdateTicketSkillIsInline` stat and `TestAlignmentContracts` fail-closed read |
| e | `SKILL.md` | revert the convention sentence to `This skill makes no docket write.` | `[change_0586…]` absent and present |
| f | `references/private.md` | for each pinned phrase in turn (at minimum: attachments first/comment last; any failed upload posts no comment; never substitute a local path; never assume a one-call upload; the file-request call; the quiet self-fix; the single review stop; posted only on confirmation), weaken it (e.g. `means no comment` → `may still post a comment`) | `[change_0586…]` present, naming that phrase |

Record each probe's before/after count and red/green in the commit body or the task report.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-update-ticket/SKILL.md skills/docket-update-ticket/references/private.md internal/repoguard/prose_contracts_test.go internal/repoguard/update_ticket_skill_test.go
git commit -m "feat(skills): deliver ticket updates as attachments in a private repository"
```

### Task 2: Reference docs describe private delivery

**Files:**
- Modify: `docs/reference/skills-and-agents.md` (the `- **docket-update-ticket** —` bullet)
- Modify: `docs/reference/glossary.md` (`### docket-update-ticket` entry)
- Test: `internal/repoguard/prose_contracts_test.go` (`alignmentContracts`)

**Interfaces:**
- Consumes: Task 1's behavior (attachments in a private repository).
- Produces: nothing later tasks use.

- [ ] **Step 1: Write the failing guards**

Add after Task 1's rows:

```go
{sentinel: "change_0586_update_ticket_private", file: "docs/reference/skills-and-agents.md",
	present: []string{"in a private repository it attaches cleaned copies of the spec and results instead"},
	absent:  []string{"shared repositories only for now"}},
{sentinel: "change_0586_update_ticket_private", file: "docs/reference/glossary.md",
	present: []string{"In a private repository it attaches cleaned copies of the spec and results instead of linking them, and posts the comment only after every attachment lands."},
	absent:  []string{"In a private repository it posts nothing yet."}},
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test -count=1 ./internal/repoguard/ -run TestAlignmentContracts`
Expected: FAIL on both `[change_0586_update_ticket_private]` doc rows (absent phrase present, present phrase missing).

- [ ] **Step 3: Edit the docs**

- `skills-and-agents.md`: in the `docket-update-ticket` bullet, replace `; shared repositories only for now.` with `; in a private repository it attaches cleaned copies of the spec and results instead.`
- `glossary.md`: replace the sentence `In a private repository it posts nothing yet.` with `In a private repository it attaches cleaned copies of the spec and results instead of linking them, and posts the comment only after every attachment lands.` Keep the paragraph hard-wrapped at the file's width. Describe current behavior only; no change numbers.

- [ ] **Step 4: Run and watch them pass**

Run: `go test -count=1 ./internal/repoguard/ -run TestAlignmentContracts`
Expected: PASS.

- [ ] **Step 5: Mutation-test**

With the `cp`/`mv -f` backup idiom and `-count=1`: restore each old phrase in turn (`shared repositories only for now`; `In a private repository it posts nothing yet.`) and confirm `TestAlignmentContracts` reddens on the matching `absent`; delete each new phrase and confirm it reddens on `present`. Restore and re-run green.

- [ ] **Step 6: Commit**

```bash
git add docs/reference/skills-and-agents.md docs/reference/glossary.md internal/repoguard/prose_contracts_test.go
git commit -m "docs: describe private-repository ticket updates"
```

### Task 3: Budgets, embedded tree, and the full gate

**Files:**
- Modify: `internal/repoguard/budgets_test.go` (`skillBudgets`)
- Regenerate: `internal/assets/embedded/` (via `go generate ./internal/assets/`)

**Interfaces:**
- Consumes: the final bytes of `skills/docket-update-ticket/SKILL.md` and `references/private.md` from Task 1.

- [ ] **Step 1: Watch the budget guard fail**

Run: `go test -count=1 ./internal/repoguard/ -run TestSkillSizeBudgets`
Expected: FAIL — `references/private.md` has no budget row (reverse correspondence), and `SKILL.md` may exceed `98/1521`.

- [ ] **Step 2: Measure and pin**

Run: `wc -lw skills/docket-update-ticket/SKILL.md skills/docket-update-ticket/references/private.md`
In `skillBudgets`:
- `docket-update-ticket/SKILL.md`: set both ceilings to the measured counts and prepend to its comment `0586: the private refusal becomes a blocking route to references/private.md; the convention paragraph names the docket-export call; the last table column covers attachments (98/1521 -> <L>/<W>); `.
- Add, between the `SKILL.md` row and the `references/shared.md` row (alphabetical), `{"docket-update-ticket/references/private.md", <L>, <W>}, // 0586: new — private delivery`, at the measured counts. Align the comment column with gofmt.

- [ ] **Step 3: Watch it pass; mutation-test the new row**

Run: `go test -count=1 ./internal/repoguard/ -run TestSkillSizeBudgets` → PASS.
Probe: lower the new `private.md` word ceiling by one → FAIL `over its <W-1>-word budget`; delete the row → FAIL reverse correspondence for `docket-update-ticket/references/private.md`. Restore (backup copy), re-run PASS.

- [ ] **Step 4: Regenerate the embedded tree**

Run from the feature-worktree root: `go generate ./internal/assets/`
Then: `go test -count=1 ./internal/assets/` → PASS (`TestEmbeddedMatchesAuthored` sees `references/private.md` and the edited `SKILL.md`). `git status --porcelain internal/assets/` shows the regenerated files, including a new embedded copy of `private.md`.

- [ ] **Step 5: Run the full build gate**

Run from the feature-worktree root: `go run ./cmd/docket development test`
Expected: green. Read the budget report even though it is green; act on any breach line per `tests/README.md`.

- [ ] **Step 6: Commit**

```bash
git add internal/repoguard/budgets_test.go internal/assets/
git commit -m "chore(assets): budget the private delivery reference and regenerate the embedded tree"
```

## Results-file item (not a build task)

The results file lists one **Important** human live check, which no in-repo test can certify: from a private scratch repository (its `tickets.*` keys in `.git/dckt/config.yml`) linked to a sandbox Jira project, ask for a spec-written update and a PR-opened update; confirm the attachments appear before the comment, the comment names them, neither the comment nor the attachments contain docket machinery, and a forced upload failure (for example, a file the connector rejects) posts no comment and reports what already landed. No ticket-system call is made from this repository, and no sandbox site or project name is written in it.
