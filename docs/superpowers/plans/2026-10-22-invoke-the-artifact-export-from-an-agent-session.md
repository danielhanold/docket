<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0578 — Invoke the artifact export from an agent session](../../changes/active/0578-invoke-the-artifact-export-from-an-agent-session.md)**
<!-- docket:backlink:end -->

# Invoke the artifact export from an agent session Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an inline `docket-export` skill so an agent session asked to export a docket change, spec, plan, or results file, or to put one anywhere outside docket, runs the real export (and, in a private repository, rewrites and checks what leaves) instead of sending the raw file or cleaning it by hand.

**Architecture:** One new skill directory, `skills/docket-export/SKILL.md`, frontmatter `name` + `description` only (no `context: fork`, no `agent:`, no wrapper under `agents/`). Install plumbing already auto-discovers skills (`internal/assets` walks `skills/`, `harness.SkillDirs` links every skill dir), so the only code-side work is regenerating the embedded bundle, a budget row, prose pins, and doc sites that enumerate inline skills. No change to the export (`internal/app/open_export.go`), its flags, or the capability catalog.

**Tech Stack:** Markdown skill body; Go repo guards in `internal/repoguard` (table-driven, `collapseWS` + `scanProse`); `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-22-invoke-the-artifact-export-from-an-agent-session-design.md` (on the metadata branch; read it beside this plan).

## Global Constraints

- The skill body is distributed: it is installed at user level into every session on the machine. No sentence may be true only of the docket repository (learning `distributed-body-has-no-local-repo`).
- The only docket CLI spelling the skill may hard-code is `docket capabilities --json` (`TestCapabilitySurface`). Every other invocation is written as "the `<op>` argv" — never `docket open …`, `docket repository …`, `docket diagnostic …`. The guard's shape is `docket` + a catalog family word + any lowercase word, so plain English trips it too: "docket change ids", "docket gate runs", "docket install is" all match. Write "change ids from docket", "one of docket's changes", "docket installation" instead (families at planning time: adr, agent, capabilities, change, context, development, diagnostic, evidence, finalize, gate, install, instructions, learning, maintenance, open, pr, repository, run, schema, status, uninstall, version, workspace — derived by the guard from the catalog).
- The skill must not contain the config-file tokens `.docket.yml`, `.docket.local.yml`, or `config.yml` (`TestConfigReadChannel…`); `leak_check.match_word` is read from the `diagnostic.config` operation's JSON (`effective.leak_check.match_word.value`).
- Living docs and skills carry no change or PR citations (`TestLivingDocsAlignment`): no `#561`, `change 0561`, `PR #…`. ADR citations (`ADR-0144`) are allowed. Placeholders like `#N` and example ticket keys like `PROJ-123` are fine.
- The skill names none of the role skills as a backticked token (`superpowers:brainstorming`, `superpowers:writing-plans`, `docket-build`, `docket-review`, `superpowers:finishing-a-development-branch`) — `skill_handoff_sites_test.go` would then demand a `DIRECTED to:` marker.
- Any temp-dir command in the skill is templated: `mktemp -d "${TMPDIR:-/tmp}/docket-export.XXXXXX"`.
- Export invocation is always `<open argv> <what> [<id>] --export --print --json`; `--warnings` only when the person asked for a filter on a file request.
- Budget rows are pinned at the exact measured `wc -l` / `wc -w` counts (no slack), with a one-line `// 0578: …` reason in the row comment, the house style of `skillBudgets`.
- Mutation-test every new pin: back the file up to a scratch copy (`cp`, never `git checkout --`, which restores HEAD and would erase uncommitted work — learning `mutation-restore-needs-a-backup-copy`), delete the pinned clause, run with `-count=1` (learning `cached-runner-serves-a-mutated-tree`), confirm red, restore from the backup with `cp -f`, confirm green. Count the clause on a whitespace-collapsed copy before and after so a mutation that did not land is not read as a surviving guard (learning `phrase-grep-over-wrapped-prose`).
- The whole suite runs at the build gate (`build.test_command`, `go run ./cmd/docket development test`), not only the tests named here.
- The installed skill registry loads at process start: the session that builds this cannot observe the new skill being triggered. Record in the results file's human-verification section that a fresh session is needed to confirm an export request picks up `docket-export` (learning `generated-artifact-loaded-at-process-start`).

## Review Focus

1. **Prepare fails, so visibility is unknown.** The skill must treat the repository as private (rewrite pass, word check, before → after stop), never as shared. Pinned in Task 1 (`the visibility is unknown, so **treat the repository as private**`).
2. **The export fails (`invalid-state` / `artifact-unset`, no spec yet).** The agent must post nothing and must not fall back to the raw file or a hand cleanup. Pinned in Task 1 (`**A failed export posts nothing.**` plus the `result`/`exported_path` clause and `Never fall back to the raw file or a hand cleanup`).
3. **A private export with leftovers whose rewrite still leaves "docket" in a code block.** The word check covers code and stops before posting. Pinned in Task 1 (`code included` clause).
4. **A non-docket directory (fresh repository or not a git repository).** The skill steps aside in one line instead of failing loudly or treating the content as private docket output. Pinned in Task 1 (`this is not a docket artifact`).
5. **The skill drifting into a forked skill or getting a wrapper.** A forked skill cannot reach the session's MCP posting tools or ask the person. Pinned in Task 1 by `TestExportSkillIsInline` (frontmatter keys exactly `name`, `description`; no `agents/docket-export.md`).

---

### Task 1: The `docket-export` skill, its pins, budget row, and bundle

**Files:**
- Create: `skills/docket-export/SKILL.md`
- Create: `internal/repoguard/export_skill_test.go`
- Modify: `internal/repoguard/prose_contracts_test.go` (append rows to `alignmentContracts`, before its closing `}`)
- Modify: `internal/repoguard/budgets_test.go` (`skillBudgets`: new row, alphabetical between `docket-convention/...` rows and `docket-finalize-change/SKILL.md`)
- Modify: `internal/harness/inventory_test.go` (`TestSkillDirsFromEmbedded`'s `known` slice gains `"docket-export"`)
- Regenerate: `internal/assets/embedded/**` via `go generate ./internal/assets/`

**Interfaces:**
- Produces: the file `skills/docket-export/SKILL.md` whose exact clauses the Task 1 pins quote; Task 2 references the skill by name `docket-export` only.
- Produces: `skillFrontmatterKeys(content string) ([]string, error)` in `export_skill_test.go` — returns the top-level keys (lines matching `^[a-z_]+:` at column 0) between the first `---` line and the next `---` line, in file order; error when the file does not open with `---` or has no closing fence.

- [ ] **Step 1: Write the failing pins**

Append to `alignmentContracts` in `internal/repoguard/prose_contracts_test.go`:

```go
	// change 0578 — the inline export skill: never a hand-built or raw export;
	// a failed export posts nothing; shared repositories get no rewrite pass and
	// no stop; private (or unknown) repositories get the rewrite pass, the word
	// check, and a before → after stop.
	{sentinel: "change_0578_export_skill", file: "skills/docket-export/SKILL.md",
		present: []string{
			"**Never build an export by hand, and never send the raw file.**",
			"`--export --print --json`",
			"**A failed export posts nothing.**",
			"If `result` is not `applied` or there is no `exported_path`, tell the person the `message` and stop",
			"Never fall back to the raw file or a hand cleanup",
			"the visibility is unknown, so **treat the repository as private**",
			"this is not a docket artifact",
			"**A shared repository gets no rewrite pass and no stop.**",
			"**Rewrite pass.** When the export reported any `leftovers` or `mentions`, edit the exported copy in place",
			"Search the rewritten copy for \"docket\" and \"dckt\" as whole words, case-insensitive, code included",
			"show the person every change as before → after and **post only after they say go**",
		},
		absent: []string{"context: fork", "agent: docket-export"}},
```

Create `internal/repoguard/export_skill_test.go`:

```go
func TestExportSkillIsInline(t *testing.T) {
	root := guardRoot(t)
	body := readRepoFile(t, "skills/docket-export/SKILL.md")
	keys, err := skillFrontmatterKeys(body)
	// err == nil; keys == []string{"name", "description"} exactly
	// the name line is "name: docket-export"
	// collapseWS(description) contains "export a docket change, spec, plan, or results file"
	//   and "put one into anything outside docket"
	// os.Stat(filepath.Join(root, "agents", "docket-export.md")) reports fs.ErrNotExist
	//   (any other Stat error fails closed)
	t.Run("non_vacuity", func(t *testing.T) {
		// "---\nname: x\ndescription: y\ncontext: fork\nagent: x\n---\nbody\n"
		//   -> keys [name description context agent]
		// "name: x\n" (no opening fence) -> error
		// "---\nname: x\n" (no closing fence) -> error
	})
}
```

The description check reads the `description:` line's value from the frontmatter (one line; the skill keeps it on one line).

- [ ] **Step 2: Run the pins to verify they fail**

Run: `go test ./internal/repoguard/ -run 'TestAlignmentContracts|TestExportSkillIsInline' -count=1`
Expected: FAIL — `read skills/docket-export/SKILL.md … (fail closed)`.

- [ ] **Step 3: Write `skills/docket-export/SKILL.md`**

Exact copy below. The builder may reword (spec: "the exact description and body wording" are the builder's), but every clause a Step 1 pin quotes must survive verbatim or the pin is repointed in the same commit.

````markdown
---
name: docket-export
description: Use when asked to export a docket change, spec, plan, or results file, or to put one into anything outside docket — a Jira ticket body or comment, a document, a message, an email — so its content comes from docket's own export, never the raw file or a hand cleanup.
---

# docket-export

Docket artifacts carry links, generated blocks, and references that mean nothing outside docket, and the export writes a clean Markdown copy of them. Use it every time a change, spec, plan, or results file leaves docket. **Never build an export by hand, and never send the raw file.**

This skill runs inline, in your own session: you post with your own tools, and you ask the person directly where a step below says to.

## 1. Resolve the operations

Run `docket capabilities --json` as its own call — the one docket spelling this skill hard-codes. Check that `protocol_version` is 1 and take the `argv` of the entries `open`, `repository.prepare`, and `diagnostic.config`. If the binary is missing or an entry is absent, tell the person their docket installation is broken and stop — never a hand-made export.

## 2. Learn the visibility

Run the `repository.prepare` argv with `--repo-dir <repo root> --json` as its own call and read `context.visibility` (`shared` or `private`).

- The directory is not a git repository, or `repository_state` is `fresh` (no docket set up): this is not a docket artifact. Say so in one line and handle the request without this skill.
- Any other refusal or error: the visibility is unknown, so **treat the repository as private**. A leak cannot be undone.

In a private repository, also run the `diagnostic.config` argv with `--repo-dir <repo root> --json` and read `effective.leak_check.match_word.value`. When it cannot be read, treat it as `true`.

## 3. Run the export

Run the `open` argv as `<open argv> <what> [<id>] --export --print --json --repo-dir <repo root>`, where `<what>` is `change`, `spec`, `plan`, or `results` and `<id>` is the change the person named (omit it to use the change whose branch is checked out). Always `--export --print --json`: `--print` opens no viewer — the person may not be at the screen, and can ask you to open the file. Never pass `--warnings` unless the person asked for a filter on a file request; the default reports everything the private pass reads.

- **File request** ("export the spec for 9"): the export's default destination, or `--output-path <place>` when the person names one.
- **Outside-tool request** ("add the spec for 9 as a comment on PROJ-123"): `--output-path` set to a fresh directory from `mktemp -d "${TMPDIR:-/tmp}/docket-export.XXXXXX"`, so the person's downloads folder does not collect one file per post. Leave the file there and report its path.

**A failed export posts nothing.** If `result` is not `applied` or there is no `exported_path`, tell the person the `message` and stop. Never fall back to the raw file or a hand cleanup. This holds in every repository.

## 4. Shared repository

**A shared repository gets no rewrite pass and no stop.** Hand the exported text to the target unchanged; on a file request the file is the result. Any `leftovers` go out as they are and are named in your report.

## 5. Private repository

Nothing that names docket may leave.

1. **Rewrite pass.** When the export reported any `leftovers` or `mentions`, edit the exported copy in place until it holds none of these: the words "docket" or "dckt", docket marker comments, `Docket-` trailers, `.docket` or `dckt` paths, change ids or change references from docket, ADR numbers from docket's ledger, or references to the backlog or board. Every edited line must read naturally to an outside reader.
   - Fix each leftover: remove the marker, unwrap the link, drop the stamp line.
   - Rewrite each line on the `mentions` list so it no longer names docket or dckt ("the docket build gate runs the suite" becomes "the build gate runs the suite"). A mention inside code is generalized or dropped, at your judgment.
   - Rewrite ADR numbers and change numbers from docket into what they refer to, or drop them; the person's team cannot open them. An ADR is docket's when a file for its number exists under `<metadata_worktree_path>/<adrs_dir>`, and a `#N` is one of docket's changes when a record for that id exists under `<metadata_worktree_path>/<changes_dir>` (paths from the prepare context). The team's own ADRs stay, and so does a `#N` that is not one of docket's changes, such as a Jira or pull request number.
   - When `leak_check.match_word` is `false`, the bare word "docket" may stay; "dckt", markers, ids, and ADR numbers are still handled.
   - Change only these lines: no other rewording, summarizing, or fixing.
2. **Word check.** Search the rewritten copy for "docket" and "dckt" as whole words, case-insensitive, code included (the bare word "docket" is exempt when `leak_check.match_word` is `false`). Any hit: stop, show the person the path and the lines, and post nothing until they decide.
3. **Show, then post.** If the rewrite pass changed anything, show the person every change as before → after and **post only after they say go**. If nothing needed rewriting, post straight away. A file request gets the same pass and check — the file goes to outsiders too — and the before → after list comes with the path, with no stop, since nothing is posted.

## 6. Post

Read the file at `exported_path` (after the rewrite pass, in a private repository) and hand its text to the target unchanged. When the target needs another format, such as Jira's own markup, convert the formatting only, never the words. A summary or excerpt the person asks for ("just the Goals section") is made from this copy, never the raw file. When the text is too long for the target, tell the person and ask whether to split it or attach the file. Whatever confirmation your harness already asks before an outward post is unchanged.

## 7. Report

Report the path, whether an existing file was replaced (`replaced`), any warnings, and, in a private repository, the before → after list.

The board and pull requests have no export. A person who explicitly asks for the raw file is asking for something else.
````

- [ ] **Step 4: Implement `skillFrontmatterKeys` and finish `TestExportSkillIsInline`**

In `internal/repoguard/export_skill_test.go`, using `guardRoot`, `readRepoFile`, `collapseWS` from the package (do not add a second whitespace helper).

- [ ] **Step 5: Run the pins to verify they pass**

Run: `go test ./internal/repoguard/ -run 'TestAlignmentContracts|TestExportSkillIsInline|TestCapabilitySurface|TestLivingDocsAlignment|TestConfigReadChannel|TestSkillHandoffSites|TestBSDToolDefaults|TestPipeShapes|TestGrepPortability' -count=1`
Expected: PASS. If `TestCapabilitySurface`, `TestLivingDocsAlignment`, or the config-read-channel guard reddens on the new file, fix the skill text (Global Constraints), never the guard.

- [ ] **Step 6: Add the budget row**

Measure: `wc -lw skills/docket-export/SKILL.md`. Add `{"docket-export/SKILL.md", <lines>, <words>}, // 0578: new — the inline export skill` to `skillBudgets`.
Run: `go test ./internal/repoguard/ -run TestSkillSizeBudgets -count=1`
Expected: PASS (it was red before this step: the reverse direction reddens an unbudgeted `skills/**/*.md`).

- [ ] **Step 7: Regenerate the bundle and prove the skill installs**

Add `"docket-export"` to the `known` slice in `TestSkillDirsFromEmbedded` (`internal/harness/inventory_test.go`). Then:

Run: `go generate ./internal/assets/ && go test ./internal/assets/ ./internal/harness/... -count=1`
Expected: PASS — `TestEmbeddedMatchesAuthored` (bundle matches `skills/`), `TestSkillDirsFromEmbedded` (now knows `docket-export`), and each harness's skill-link test (`TestClaudeSkillLinks`, `TestCursorSkillLinks`, `TestOpencodeSkillLinks`, the codex skills-root test) derive their expectation from `SkillDirs` and so cover the new skill. Confirm `internal/assets/embedded/tree/skills/docket-export/SKILL.md` exists and no file under `agents/` or `internal/assets/embedded/tree/agents/` names `docket-export`.

- [ ] **Step 8: Mutation-test each pin**

For each `present` clause in the `change_0578_export_skill` row and each assertion of `TestExportSkillIsInline` (add `context: fork` to the frontmatter; create an empty `agents/docket-export.md`): apply the mutation per Global Constraints, run `go test ./internal/repoguard/ -run 'TestAlignmentContracts|TestExportSkillIsInline' -count=1`, confirm FAIL naming that clause, restore, confirm PASS. Delete any scratch `agents/docket-export.md` afterwards. Any clause that stays green is a defect in the pin — fix the pin before continuing.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-export/SKILL.md internal/repoguard/export_skill_test.go internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/harness/inventory_test.go internal/assets/embedded
git commit -m "feat: add the docket-export skill so agent sessions run the real export"
```

---

### Task 2: Name `docket-export` wherever docket's skills or inline skills are enumerated

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (*Agent layer* paragraph, the sentence "the two **interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline and run at the session model.")
- Modify: `skills/docket-convention/references/agent-layer.md` (Fork-exclusion sentence "the two interactive skills stay inline, and `docket-finalize-change` stays unforked")
- Modify: `internal/repoguard/budgets_test.go` (raise the two convention rows by the exact measured delta)
- Modify: `docs/reference/skills-and-agents.md` (Skills list)
- Modify: `docs/reference/cli.md` (`docket open` entry)
- Modify: `docs/install/models-and-effort.md` (the "two interactive skills … run at whatever model" paragraph and the **Fork-exclusion principle** paragraph)
- Modify: `docs/reference/glossary.md` (new `### docket-export` entry, its index line, *Interactive vs autonomous skills*, *Fork-exclusion principle* "Used for")
- Modify: `internal/repoguard/prose_contracts_test.go` (three more rows under the same sentinel)
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: the skill name `docket-export` from Task 1.

- [ ] **Step 1: Write the failing pins**

Append to `alignmentContracts`:

```go
	{sentinel: "change_0578_export_skill", file: "skills/docket-convention/SKILL.md",
		present: []string{"(`docket-new-change`, `docket-groom-next`) and `docket-export` stay inline"}},
	{sentinel: "change_0578_export_skill", file: "docs/reference/cli.md",
		present: []string{"Agent sessions reach the export through the `docket-export` skill."}},
	{sentinel: "change_0578_export_skill", file: "docs/reference/skills-and-agents.md",
		present: []string{"- **docket-export** —"}},
```

Run: `go test ./internal/repoguard/ -run TestAlignmentContracts -count=1`
Expected: FAIL on the three new clauses.

- [ ] **Step 2: Find every enumeration site by grep, not from this list**

Run: `grep -rn -i -e 'interactive skills' -e 'interactive\*\* skills' -e 'stay inline' -e 'run inline' -e 'docket-groom-next' --include='*.md' . --exclude-dir=.git --exclude-dir=testdata --exclude-dir=embedded`
Keep only maintained surfaces (drop `docs/changes/`, `docs/superpowers/`, `docs/results/`, `docs/adrs/`, `docs/comparison/` — point-in-time records). Sort the hits into sentences that enumerate skills or inline skills (edit) and ones that merely name a skill (leave). At planning time the enumerating sites were exactly the ones in **Files** above; edit any additional one the grep finds the same way.

- [ ] **Step 3: Edit the sites**

- Convention *Agent layer*: "the two **interactive** skills (`docket-new-change`, `docket-groom-next`) and `docket-export` stay inline and run at the session model."
- `agent-layer.md`: "the two interactive skills and `docket-export` stay inline, and `docket-finalize-change` stays unforked".
- `skills-and-agents.md`, between `docket-convention` and `docket-finalize-change`: "- **docket-export** — export a change, spec, plan, or results file through docket's own export, or put one into another tool; in a private repository it also rewrites docket references out and shows you before posting."
- `cli.md`, `docket open` entry, after "and opens that copy.": "Agent sessions reach the export through the `docket-export` skill."
- `models-and-effort.md`: after the interactive-skills paragraph add "`docket-export` also runs inline at your session's model and effort; it runs one export and needs no particular model." In the Fork-exclusion paragraph, add `docket-export` to the unforked list with its reason: it posts through your session's own tools and may ask you before a private-repository post.
- `glossary.md`: index line `- [docket-export](#docket-export)` between `docket-convention` and `docket-finalize-change`; a `### docket-export` entry in the same place, two short paragraphs in the house shape (what it is; **Used for:**): it runs the `open` operation's export (`--export --print --json`) and hands the clean copy to a file or another tool, never the raw file; in a private repository it rewrites what the export leaves that names docket, checks the result, and shows you before → after before posting; used for putting a spec or other artifact into a ticket, document, or message from an agent session, and it runs inline. *Interactive vs autonomous skills*: add "`docket-export` also stays inline, because it posts with your session's own tools and may ask you first." *Fork-exclusion principle* "Used for": "explaining why `docket-new-change`, `docket-groom-next`, `docket-export`, and `docket-finalize-change` run inline."

No `docket <family> <subcommand>` spelling in the skill-surface edits; no change/PR citations anywhere.

- [ ] **Step 4: Re-baseline the two convention budgets**

Measure `wc -lw skills/docket-convention/SKILL.md skills/docket-convention/references/agent-layer.md`. Set each row's ceilings to the measured counts and prepend to its comment `0578: docket-export joins the inline skills (<old> -> <new>);` in the house `lines/words` form.

- [ ] **Step 5: Regenerate the bundle and run the guards**

Run: `go generate ./internal/assets/ && go test ./internal/assets/ ./internal/repoguard/ -count=1`
Expected: PASS, including `TestAlignmentContracts`, `TestSkillSizeBudgets`, `TestLivingDocsAlignment`, `TestEmbeddedMatchesAuthored`.

- [ ] **Step 6: Mutation-test the three new pins**

Delete each pinned clause (backup, collapsed-count before/after, `-count=1`), confirm FAIL, restore with `cp -f`, confirm PASS.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-convention/references/agent-layer.md internal/repoguard/budgets_test.go internal/repoguard/prose_contracts_test.go docs/reference/skills-and-agents.md docs/reference/cli.md docs/install/models-and-effort.md docs/reference/glossary.md internal/assets/embedded
git commit -m "docs: name docket-export among docket's skills and inline skills"
```
