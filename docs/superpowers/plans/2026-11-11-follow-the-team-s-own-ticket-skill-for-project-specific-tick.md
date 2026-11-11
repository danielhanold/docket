<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0591 — Follow a companion skill when creating a ticket](../../changes/archive/2026-11-11-0591-follow-the-team-s-own-ticket-skill-for-project-specific-tick.md)**
<!-- docket:backlink:end -->

# Follow a companion skill when creating a ticket Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` (one tier worker per task under the `docket-build-task` contract, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the any-layer config key `tickets.companion_skill` and teach `docket-link-ticket`'s create path to find and follow a companion skill for project-specific ticket details, while removing the "description is never rewritten" rule.

**Architecture:** The key is one more `tickets.*` schema leaf (`internal/config/schema.go`), but `scopeAny` instead of `scopeRepoOnly`, so it gets normal four-layer precedence and the shared-setting guard never touches it. It surfaces through `diagnostic.config` at `effective.tickets.companion_skill.value` like its siblings. The behaviour lives entirely in `skills/docket-link-ticket/references/create.md` (prose, pinned by a new prose-contract row); `SKILL.md` is not edited.

**Tech Stack:** Go (`internal/config`, `internal/app`), Markdown skills, repository guards in `internal/repoguard`, embedded asset twins regenerated with `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-11-11-follow-the-team-s-own-ticket-skill-for-project-specific-tick-design.md` (on the `docket` branch). Read it before either task.

## Global Constraints

- No ticket-system call from this repository in any test, build, review, or results run. Docket's own `.docket.yml` gets no `tickets` block (`TestDocketYMLDeclaresNoTicketsKey` stays unchanged and green).
- Docket names, ships, or describes no particular companion skill, provider-specific field id, or team value anywhere (skill text, example config, docs, tests). Test fixtures use a neutral name such as `team-tickets`.
- `tickets.companion_skill`: `kind: kindString`, `def: ""`, `merge: mergeScalar`, `scope: scopeAny`, `disp: dispSupported`, `validate: singleLineStringLeaf()`. Every other `tickets.*` leaf stays `scopeRepoOnly`.
- The companion skill is used on the create path only. Link, import (`references/import.md`), and `docket-update-ticket` are untouched.
- A missing or misconfigured companion skill is a one-line warning and a fallback, never a stop (no new blocking gate).
- Spec decision 7 overrides the "written once and never rewritten" wording in spec decision 4: after this change `create.md` contains none of `never rewritten`, `cannot be rewritten`, `written once`.
- Skill prose house style: one paragraph or bullet per physical line, so every pinned phrase sits on one line (`scanProse` is a raw `strings.Contains`; learning `phrase-grep-over-wrapped-prose`). Skill bodies ship into other repositories: no sentence about "this repo" (learning `distributed-body-has-no-local-repo`).
- Docs and comments describe current behaviour for a reader who never saw this change: no change numbers in the example config, config-keys doc, or skill text (learning `config-knob-ship-end-to-end`). Change numbers are fine in test comments and budget notes, per those files' conventions.
- After any edit under `skills/` or to `.docket.example.yml`, run `go generate ./internal/assets/` and commit the regenerated `internal/assets/embedded/tree/` twins in the same commit (`TestEmbeddedMatchesAuthored`).
- Budget rows in `internal/repoguard/budgets_test.go` are pinned at the exact measured `wc -l` / `wc -w` counts, with a leading `0591: <what> (old -> new); ` note prepended to the existing comment.
- Every guard added is mutation-tested: strip what it guards, run with `-count=1`, watch it redden, restore from a backup copy (never `git checkout --` on an uncommitted edit; learning `mutation-restore-needs-a-backup-copy`).
- Cross-references in maintained source anchor on symbols or quoted clauses, never line numbers (ADR-0054).
- The results file (written later by implement-next) lists under `## Human actions and testing` one **Important** item: from another repository with a companion skill installed, create a ticket from a change once with `tickets.companion_skill` set and once relying on description matching; confirm the title format, sprint, custom fields, labels, type, and any after-creation actions match the companion skill and that the draft showed them before creation; also confirm the missing-skill warning when the key names a skill that isn't installed.

## Review Focus

1. **A global or `.docket.local.yml` `tickets:` block that mixes repo-only leaves with `companion_skill`.** The person expects the companion skill to apply and the repo-only siblings to be warned about and dropped, per leaf. Pinned by `TestTicketsCompanionSkillAnyLayer` (Task 1, mixed-block case).
2. **`companion_skill` set to a multi-line string or a non-string (`[a]`).** Expected: rejected with the same codes its siblings get, never stored. Pinned by the new rows in `TestTicketsStringSingleLine` and the schema validation table (Task 1).
3. **The never-rewritten rule creeping back under another spelling** (`cannot be rewritten`, `written once`). Pinned by the `absent` list of the `change_0591_companion_skill` row (Task 2), mutation-tested by re-inserting the old step-8 sentence.
4. **No key and no matching skill.** The person expects the ticket to be created exactly as before. Pinned by the `None: create the ticket without one` phrase in the Task 2 contract; with the key unset, no existing config test's expectations change (Task 1 adds rows only).
5. **A companion skill that asks to replace the description or skip the approval.** Expected: docket's rule stands and the draft says so in one line. Pinned by the `docket's rule stands` and `Docket always keeps …` phrases (Task 2).

---

### Task 1: The `tickets.companion_skill` config key, end to end

**Build tier:** standard

**Files:**
- Modify: `internal/config/schema.go` (new leaf after `tickets.project`; the ticket-leaf comment above `tickets.provider`)
- Modify: `internal/config/config.go` (`Tickets` struct and its comment)
- Modify: `internal/config/defaults.go` (`builtinEffective`, `Tickets` literal)
- Modify: `internal/config/resolve.go` (`assemble`, after the `tickets.project` assign)
- Modify: `internal/app/config.go` (the human `leafLine` block, after `tickets.project`)
- Modify: `.docket.example.yml` (Tickets section)
- Modify: `docs/reference/config-keys.md` (new row after `tickets.project`)
- Regenerate: `internal/assets/embedded/tree/.docket.example.yml`
- Test: `internal/config/schema_test.go`, `internal/config/defaults_test.go`, `internal/config/decode_test.go`, `internal/config/resolve_test.go`, `internal/config/fixtures_test.go`, `internal/config/setting_paths_test.go`, `internal/app/config_test.go`

**Interfaces:**
- Produces: `config.Tickets.CompanionSkill Value[string]` with JSON tag `companion_skill`, so `diagnostic.config --json` reports `effective.tickets.companion_skill.value`; human inspection line `tickets.companion_skill = <value>  [<layer>]` directly after `tickets.project`. Task 2's skill text reads exactly that path.

- [ ] **Step 1: Derive every enumeration site.** Run `grep -rn 'tickets\.project\|Tickets\.Project' internal cmd docs .docket.example.yml` (exclude `internal/assets/embedded/`). Sort hits into whole-registry enumerations (add the new leaf after `tickets.project`) and repo-only enumerations (`TestRegistryRepoOnlySet`, `TestRepoOnlyPaths`, `guardCases`, `TestTicketsKeysRepoOnly`: leave unchanged, because the new leaf is not repo-only and those whole-set compares must keep excluding it).

- [ ] **Step 2: Write the failing tests.**
  - `schema_test.go`: insert `"tickets.companion_skill"` after `"tickets.project"` in the registry path list and in the defaults map (`"tickets.companion_skill": ""`); add validation rows `{"tickets companion skill ok", "tickets.companion_skill", "team-tickets", "team-tickets", ""}` and `{"tickets companion skill not a string", "tickets.companion_skill", "[a]", nil, CodeInvalidType}`.
  - `defaults_test.go`, `fixtures_test.go`, `resolve_test.go` `effectiveLeaf`: add the `tickets.companion_skill` entry mapped to `eff.Tickets.CompanionSkill`.
  - `decode_test.go`: row with `block: "tickets:\n  companion_skill: team-tickets\n"`, `flow: "tickets: {companion_skill: team-tickets}\n"`, `value: "team-tickets"`.
  - `setting_paths_test.go` `TestSettingPathsSupportSplit`: add `"tickets.companion_skill"` to the supported list.
  - `resolve_test.go` `TestTicketsStringSingleLine`: add `"tickets:\n  companion_skill: \"a\\nb\"\n"`.
  - `resolve_test.go` new `TestTicketsCompanionSkillAnyLayer`, modelled on `TestOpenArtifactsIsAnOrdinaryLayeredKey`, with `doc(v) = "tickets:\n  companion_skill: " + v + "\n"`:
    - nothing declared → `""`, `LayerBuiltIn`, not explicit;
    - `{srcG(a)}` → `a` from `LayerGlobal`; `{srcL(a)}` → `LayerRepositoryLocal`; `{srcG(a), srcR(b)}` → `b` from `LayerRepository`; `{srcR(b), srcL(c)}` → `c` from `LayerRepositoryLocal`; `{srcG(a), srcL(c)}` → `c` from `LayerRepositoryLocal`; each explicit;
    - for every global/local case, `diagPathSet(mustSnapshot(t, srcs...), CodeSharedSettingIgnored)["tickets.companion_skill"]` is false;
    - mixed block: `srcG("tickets:\n  provider: jira\n  companion_skill: team-tickets\n")` → `tickets.provider` IS in the shared-setting-ignored set and stays `""`, while `CompanionSkill` is `team-tickets` from `LayerGlobal`.
  - `internal/app/config_test.go` `TestConfigDiagnosticsTicketsSurface`: expect `"tickets.companion_skill = (unset)  [built-in]"` in the default block; add `  companion_skill: team-tickets\n` to the declared repository doc (after `project: CLOUD`) and expect `tk.CompanionSkill.Value == "team-tickets"`, the human line `"tickets.companion_skill = team-tickets"`, and `"companion_skill":` in the effective JSON. Adjust the `[repository .docket.yml:2]` expectation only if the inserted line moves `provider` (it does not, if inserted after `project`).

- [ ] **Step 3: Run them to verify they fail**

Run: `go test ./internal/config/... ./internal/app/ -run 'Registry|Defaults|Decode|Tickets|SettingPaths|Fixture|ConfigDiagnostics' -count=1`
Expected: FAIL (compile error `eff.Tickets.CompanionSkill undefined`).

- [ ] **Step 4: Implement the leaf.**
  - `schema.go`: add the leaf (Global Constraints values) after `tickets.project`. Rewrite the comment above `tickets.provider` to say: the ticket settings name which ticket system a repository links to, where new tickets go, and the per-event switches, and belong to the repository (a machine layer's declaration is ignored with a warning); the one exception is `companion_skill`, which names the person's own tooling (a skill usually installed for the user, in every repository) and so is set at any layer.
  - `config.go`: add `CompanionSkill Value[string] \`json:"companion_skill"\`` after `Project`; replace "Every leaf is repository-only." with a sentence naming `CompanionSkill` as the one any-layer leaf and why (it names the person's tooling, not the repository).
  - `defaults.go`: `CompanionSkill: builtinValue("")`.
  - `resolve.go`: `set(assign(&eff.Tickets.CompanionSkill, r.declared, "tickets.companion_skill"))` after the `tickets.project` line.
  - `internal/app/config.go`: `leafLine("tickets.companion_skill", textValue(eff.Tickets.CompanionSkill.Value), eff.Tickets.CompanionSkill.Provenance)` after the `tickets.project` line.

- [ ] **Step 5: Document the key.**
  - `.docket.example.yml`: inside the `tickets:` block, after `project: ""`, add a commented paragraph, its own `  # scope: any layer` tag, and `  companion_skill: ""`. The paragraph says, for a user deciding whether to set it: `companion_skill` names a skill you already have for your ticket project (from your company, your team, or yourself) by the name your session's skill list shows; when a ticket is created from a change, docket follows it for the project's title format, sprint, fields, labels, and ticket type; empty (the default) means docket looks for a skill whose description covers the project, and creates the ticket as usual when there is none; unlike the other ticket settings it may be set in the global config, since a skill is usually installed for you rather than for one repository. Change the block's closing sentence "These settings belong to the repository: a value in the global config or `.docket.local.yml` is ignored with a warning." so it excepts `companion_skill`. Name no real skill.
  - `docs/reference/config-keys.md`: after the `tickets.project` row add exactly `| \`tickets.companion_skill\` | \`""\` (unset) | any layer | the skill to follow for project-specific details when creating a ticket (title format, sprint, fields, labels, type) |`.
  - Run `go generate ./internal/assets/`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/config/... ./internal/app/ ./internal/assets/... ./internal/repoguard/ -count=1`
Expected: PASS, including `TestExampleSchemaCorrespondence` (directions A–D: the verbatim example still resolves with no warning), `TestEmbeddedMatchesAuthored`, `TestRegistryRepoOnlySet`, `TestRepoOnlyPaths`, `TestTicketsKeysRepoOnly`, `TestEverySharedSettingGuardCoversTheRegistry`, `TestDocketYMLDeclaresNoTicketsKey` unchanged.

- [ ] **Step 7: Mutation-test the any-layer guarantee.** Back up `schema.go`, change the new leaf's `scope` to `scopeRepoOnly`, run `go test ./internal/config/ -run 'TestTicketsCompanionSkillAnyLayer|TestRegistryRepoOnlySet' -count=1`: both must FAIL. Restore from the backup and re-run: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config/ internal/app/config.go internal/app/config_test.go .docket.example.yml docs/reference/config-keys.md internal/assets/embedded/tree/.docket.example.yml
git commit -m "feat(config): add tickets.companion_skill, settable at any layer"
```

---

### Task 2: The create path finds and follows a companion skill; tickets may be edited

**Build tier:** standard

**Files:**
- Modify: `skills/docket-link-ticket/references/create.md`
- Modify: `internal/repoguard/prose_contracts_test.go` (the `change_0584_link_ticket_skill` create.md row; new `change_0591_companion_skill` rows)
- Modify: `internal/repoguard/budgets_test.go` (the `docket-link-ticket/references/create.md` row)
- Regenerate: `internal/assets/embedded/tree/skills/docket-link-ticket/references/create.md`

**Interfaces:**
- Consumes: Task 1's read path `effective.tickets.companion_skill.value` and the config-keys row text.

- [ ] **Step 1: Write the failing contract.**
  - In the `change_0584_link_ticket_skill` row for `create.md`, delete the present string `"The description is written once and never rewritten."` and add nothing that forbids edits. Every other 0584 phrase stays and must stay verbatim in `create.md` (including the step-7 "Show the person, in one message, …" sentence and "Create only on their go.").
  - Add, after the 0585 rows, a comment `// change 0591 — the create path finds a companion skill before its first ticket-system call, follows it for type, title, fields, after-creation actions, and pacing, keeps docket's own rules, shows everything in the draft, and records even when a later field or action fails; the never-rewritten rule is gone.` and a row `{sentinel: "change_0591_companion_skill", file: "skills/docket-link-ticket/references/create.md", present: […], absent: []string{"never rewritten", "cannot be rewritten", "written once"}}` whose `present` list is exactly:
    - `"Before the first ticket-system call, look for a companion skill"`
    - ``"Read `tickets.companion_skill` from `diagnostic.config` at `effective.tickets.companion_skill.value`."``
    - `"Do not check whether it covers this project: the key is the person's choice."`
    - ``"(`tickets.companion_skill names <name>, which isn't available in this session`), then look for one as if the key were unset."``
    - `"look through the skills available in this session for one whose description says it handles this ticket project"`
    - `"One match: load it and follow it. Several: ask the person which one. None: create the ticket without one"`
    - `"A companion skill decides the ticket type, only when it states one; the title format, filled with the change's title;"`
    - `"anything it does after the ticket exists; and how the calls are made"`
    - `"Docket always keeps the already-linked and broken-link stops, the readiness check, the description body, the person's approval of the draft, and recording the link only after the connector confirms the ticket."`
    - `"a companion skill's own description template never replaces it"`
    - `"docket's rule stands"`
    - `"When the companion skill states a ticket type, use it."`
    - `"Never infer a type from the companion skill's examples"`
    - `"each field it sets (the type, the sprint, custom fields, labels, and any other field), and each action it will take after the ticket exists"`
    - `"nothing from the companion skill reaches the ticket unless the draft showed it"`
    - `"Then set the companion skill's remaining fields with edits, and run its after-creation actions, in the order and pace it prescribes."`
    - `"a field or action that fails afterwards never stops the record: the ticket stays, the link is still recorded"`
    - `"the report names each field or action that did not land"`
    - `"a branch link stays current after a later spec revision"`
  - A second row: `{sentinel: "change_0591_companion_skill", file: "docs/reference/config-keys.md", present: []string{"| \`tickets.companion_skill\` | \`\"\"\` (unset) | any layer |"}}`.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: FAIL listing the missing 0591 create.md phrases and the three present `absent` strings (`never rewritten`, `cannot be rewritten`, `written once`); the config-keys row passes (Task 1).

- [ ] **Step 3: Rewrite `create.md`.** Keep the header paragraph and steps 1–3 unchanged. New numbering, each step one physical line (sub-bullets one line each):
  - **4. Companion skill** (new). Opens: "Before the first ticket-system call, look for a companion skill: a skill the person already has for this ticket project, from their company, their team, or themselves, that knows its title format, sprint, fields, labels, ticket type, and call limits." then the key-read sentence (pinned). Three bullets: **Set, and the skill is available in this session:** load it and follow it, then the pinned "Do not check …" sentence; **Set, but the skill is not available:** say so in one line with the pinned warning text and fallthrough; **Unset:** the pinned description-matching clause, naming `tickets.project` or the project the person gave for this run, "Its description decides which projects it covers.", then the pinned One/Several/None sentence ending "…without one, as the steps below describe." Then one continuation paragraph (indented 3 spaces, one line) with: the pinned "A companion skill decides …" sentence continuing "the field values, such as the sprint, custom fields, labels, assignee, and priority, with any lookups it prescribes; anything it does after the ticket exists; and how the calls are made, such as one ticket-system call at a time."; the pinned "Docket always keeps …" sentence; "The description body is docket's export of the change plus the spec line or attachment; a companion skill's own description template never replaces it."; and "When the companion skill asks for something that conflicts with one of those, docket's rule stands: say so in one line in the draft (`<skill> asks for <X>; docket keeps <Y>`) and do not do it."
  - **5. Ticket type** (old 4): prepend the pinned "When the companion skill states a ticket type, use it." and "Otherwise", keep the existing text, and add "Never infer a type from the companion skill's examples or its defaults for other fields."
  - **6. Export the body** (old 5): unchanged.
  - **7. Spec** (old 6): shared bullet ends "That is a branch link, not one pinned to a commit: a branch link stays current after a later spec revision." The private bullet's references become "(as in step 6)" and "(step 10)".
  - **8. Draft and approval** (old 7): keep the "Show the person, in one message, …" sentence verbatim, then add "With a companion skill, the draft also names it and shows the formatted title, each field it sets (the type, the sprint, custom fields, labels, and any other field), and each action it will take after the ticket exists; nothing from the companion skill reaches the ticket unless the draft showed it, and any conflict line from step 4 appears here." Keep "Create only on their go." and the word-check sentence.
  - **9. Create** (old 8): "the summary is the export's H1, or the companion skill's title format with the export's H1 as its summary part, and the description is the rest." Keep the markup sentence. Delete "The description is written once and never rewritten." Add "Set every companion field the create call accepts." then the pinned "Then set …" sentence, then "Once the connector confirms the ticket exists, a field or action that fails afterwards never stops the record: the ticket stays, the link is still recorded (step 11), and the report names each field or action that did not land, with what the person needs to finish it by hand."
  - **10. Attach** (old 9): "when step 7 chose an attachment"; the failure clause becomes "If the upload fails, the ticket stays: report the failure and the exported file's path so the person can attach it by hand."
  - **11. Record** (old 10): unchanged.
  - **12. Report** (old 11): "Report the ticket link, its type, whether the spec was linked or attached, the companion skill used, if any, each companion field or action that did not land, and any before → after list."
  - Then `go generate ./internal/assets/`.

- [ ] **Step 4: Raise the budget.** Measure `wc -l -w skills/docket-link-ticket/references/create.md`. Set the `docket-link-ticket/references/create.md` row to those exact counts and prepend `0591: the companion-skill step, its draft and create fields, and the never-rewritten rule removed (35/853 -> <L>/<W>); ` to its comment.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/repoguard/ ./internal/assets/... -count=1`
Expected: PASS (`TestProseContracts`, `TestSkillSizeBudgets`, `TestLinkTicketSkillIsInline`, `TestEmbeddedMatchesAuthored`).

- [ ] **Step 6: Mutation-test the contract.** For each probe, back up `create.md`, apply the mutation, run `go test ./internal/repoguard/ -run TestProseContracts -count=1`, expect FAIL naming `change_0591_companion_skill`, restore from the backup:
  1. Re-append "The description is written once and never rewritten." to step 9.
  2. Replace "the ticket stays:" in step 10 with "the ticket stays, since its description cannot be rewritten:".
  3. Delete the key-read sentence from step 4.
  4. Delete the "Docket always keeps …" sentence.
  5. Delete the "a field or action that fails afterwards …" sentence.
  After restoring, re-run: PASS, and `git diff --stat` shows only the intended files.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-link-ticket/references/create.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/assets/embedded/tree/skills/docket-link-ticket/references/create.md
git commit -m "feat(skills): follow a companion skill when creating a ticket"
```

---

## Final gate

Run the whole suite through the configured `build.test_command` (docket-build's gate), not only the tests named above, and read the budget report even when green.
