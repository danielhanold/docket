<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0587 — Update tickets automatically after each event](../../changes/archive/2026-11-11-0587-update-tickets-automatically-after-each-event.md)**
<!-- docket:backlink:end -->

# Update tickets automatically after each event — Implementation Plan

> **For agentic workers:** executed by `docket-build` task-by-task (each task routed to a tier agent running `docket-build-task`). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the four `tickets.auto.*` switches their effect. When a switch is on and the change has a ticket, the workflow that just finished that event posts the update through `docket-update-ticket`, without asking. Remove `tickets.auto.change_created`, and add a concepts page on tickets.

**Architecture:** No new operation and no production Go beyond deleting one config leaf. A shared definition in `docket-convention` (*Automatic ticket updates*) holds the three steps every site runs: read `context.ticket`, stay silent, otherwise invoke. Five one-paragraph sites in four workflow skills each name their event and carry their own `DIRECTED to:` direction. `docket-update-ticket` gains an automatic path in a new blocking reference, `references/automatic.md`. Two tests pin the wiring: a block-level site guard in `internal/app` (next to `ticketEvents`), and a whitespace-collapsed prose-contract table in `internal/repoguard`.

**Tech Stack:** Go tests (`go test`), Markdown skills mirrored into `internal/assets/embedded/tree` by `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-11-11-update-tickets-automatically-after-each-event-design.md` (metadata branch; read through the `.docket/` worktree). Decisions 1–9 and *Tests* are normative. Where this plan gives exact copy, it is a starting point. The builder may reword it, but must then update the pinned clause in the same commit.

## Global Constraints

- Repository visibility: shared. No ticket-system call is made from this repository, in tests or otherwise. This repository's `.docket.yml` still declares no `tickets` key (`internal/repoguard/docket_yml_tickets_test.go` stays green, untouched).
- Production Go changes only remove `tickets.auto.change_created`. `context.ticket` (`internal/app/ticket_context.go`) is unchanged, and `ticketEvents` stays exactly `spec_written`, `pr_opened`, `pr_merged`, `results_completed`.
- Skill bodies ship into other repositories. Write no sentence that is only true in this repository, and cite no change or PR in skills or docs (`TestLivingDocsAlignment` enforces this). ADR citations are allowed.
- Docs describe only current behavior. Nothing in `docs/` or skills names `change_created` or says a switch was once planned or dropped.
- Cross-references anchor on a symbol name or a heading, never a line number.
- After any edit under `skills/` or to `.docket.example.yml`, run `go generate ./internal/assets/` and commit the regenerated `internal/assets/embedded/tree/...` files in the same commit.
- Every grown skill file gets its `internal/repoguard/budgets_test.go` row raised to the **exact** new line/word counts, with a note prepended to that row's comment in the house form `0587: <what grew> (<old lines>/<old words> -> <new lines>/<new words>); `. A new skill file gets a new row whose note ends in `0587: new — <what it is>`.
- Mutation probes: back up the file first (`cp f "${TMPDIR:-/tmp}/…"` from a `mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX"` directory) and restore from the backup, never with `git checkout --`. Run every probe with `-count=1`. Confirm each mutation actually landed (before/after `grep -c` on a whitespace-flattened copy) before believing a red or a green.
- In the five site files and in `docket-update-ticket`'s own files, never put the backticked name `` `docket-update-ticket` `` next to an invoke verb except in the five site paragraphs and the convention's definition. The site guard (Task 3) counts every such block.
- The whole suite is `build.test_command` (`go run ./cmd/docket development test`), run once by `docket-build`'s final gate. Per-task verification uses the focused `go test` commands below.
- No ADR work in this plan. The coordinator records the new ADR and ADR-0162's update note through `docket-adr` after the build.

## Review Focus

1. **A site written in another shape:** a passive "`docket-update-ticket` is invoked", or a paragraph hard-wrapped across lines (as in `results-artifact.md`), must still count as a site. Pinned by Task 3's `non_vacuity` subtest.
2. **A prohibition that names the skill:** "the sweep never invokes `docket-update-ticket`" must not count as a site. Pinned by Task 3's `non_vacuity` subtest.
3. **A stack root landing:** a `root-archived` closeout posts for the root only, never for `carried_ids`. A merged-recovery run posts once for each change it closed out. Pinned by Task 3's finalize contract clause.
4. **A retried success** (`already-completed`, `already-closed-out`, a `change.mark-implemented` retry returning its earlier applied outcome) posts like a first success. Pinned by Task 3's implement-next, results-artifact, and finalize contract clauses.
5. **A broken read with every switch off:** a `context.ticket` read that is not `applied` reports one `not checked` line and the run carries on. A non-`shared` visibility posts nothing and reports one line. Pinned by Task 3's convention clauses and Task 2's `automatic.md` clauses.

## Results requirements (for implement-next Step 6.5, not a build task)

The results must list an **Important** live check, done by a person from a shared scratch repository against a sandbox ticket project, never from this repository:

- With each switch on in turn, run the workflow and confirm the comment arrives with working links.
- With the switch off, and with the switch on but no ticket, confirm nothing is posted and nothing is reported.
- Confirm what a dispatched `docket-implement-next` and a forked `docket-auto-groom` do at the post step: they post the comment, or report the "no connector" line.

## Coordinator follow-ups (after the build)

- Record the new ADR and ADR-0162's `## Update` note through `docket-adr` (spec *ADR*).
- Once the new ADR has its number, add it to `docs/concepts/tickets.md`'s *Decided in*, cited by number without a link like the other later ADRs. Task 4 leaves the entry out rather than guessing the number.

---

### Task 1: Retire `tickets.auto.change_created` and describe the four switches

**Build tier:** standard

**Files:**
- Modify: `internal/config/schema.go` (the `tickets.auto.change_created` registry leaf), `internal/config/config.go` (`TicketsAuto.ChangeCreated`), `internal/config/resolve.go` (its `assign` line), `internal/config/defaults.go` (its `builtinValue`), `internal/app/config.go` (its `leafLine` in the human block)
- Modify tests: `internal/config/decode_test.go` (`TestDecodeRejections` plus the per-leaf row table that holds the `tickets.auto.change_created` row), `internal/config/schema_test.go` (both enumerations), `internal/config/setting_paths_test.go`, `internal/config/fixtures_test.go`, `internal/config/defaults_test.go`, `internal/config/resolve_test.go` (`effectiveLeaf` case), `internal/app/config_test.go` (`TestConfigDiagnosticsTicketsSurface`: three places)
- Modify: `.docket.example.yml` (the `auto:` leaf and the comment above the `tickets:` block), `docs/reference/config-keys.md` (the five `tickets.auto.*` rows and the `**`tickets`**` paragraph), `internal/repoguard/prose_contracts_test.go` (the `change_0592_tickets_any_layer` rows)
- Regenerate: `internal/assets/embedded/tree/.docket.example.yml`

**Interfaces:**
- Produces: `config.TicketsAuto` with exactly `SpecWritten`, `PROpened`, `PRMerged`, `ResultsCompleted`. `context.ticket` reads these unchanged.

- [ ] **Step 1: Write the failing test.** Add one row to the `cases` table in `TestDecodeRejections` (`internal/config/decode_test.go`), next to the `"unknown tickets auto switch"` row:

```go
{"retired tickets auto switch", "tickets:\n  auto:\n    change_created: true\n", CodeUnknownKey, "tickets.auto.change_created", SeverityError},
```

- [ ] **Step 2: Run it to verify it fails.** Run: `go test ./internal/config/ -run TestDecodeRejections -count=1`. Expected: FAIL. The key is still accepted.

- [ ] **Step 3: Remove the leaf everywhere.** Derive the sites with a case-sensitive whole-repo search, not from this list: `grep -rnE "change_created|ChangeCreated" --exclude-dir=.git .`. Delete the production leaf from the five Go files. In every test that enumerates the ticket keys, delete the `change_created` entry: the `decode_test.go` per-leaf row, both `schema_test.go` lists, and the `setting_paths_test.go`, `fixtures_test.go`, `defaults_test.go`, and `resolve_test.go` entries. In `TestConfigDiagnosticsTicketsSurface`, delete both `tickets.auto.change_created = …` expectations, the `change_created: true` line from the YAML fixture, the `!tk.Auto.ChangeCreated.Value ||` term, and `` `"change_created":` `` from the JSON-key list. Leave alone:
  - the `"change_created"` entry in `internal/app/ticket_context_test.go` (an unknown-event case, still correct);
  - records under `docs/superpowers/` and `docs/results/`.

- [ ] **Step 4: Rewrite the sample config.** In `.docket.example.yml`:
  - Delete `    change_created: false`.
  - Replace the three-line comment that begins `# The `auto` switches name five events` with:

```
# The four `auto` switches name events: a change's spec is written, its pull request opens, its
# pull request merges, or its results are complete. When one is on and the change has a ticket,
# the workflow that just finished that event posts the update to the ticket as a comment, without
# asking. All are off by default, and off means nothing is posted or reported.
```

- [ ] **Step 5: Rewrite the reference rows.** In `docs/reference/config-keys.md`:
  - Delete the `tickets.auto.change_created` row.
  - Replace the last cell of the other four rows as follows:
    - `spec_written`: `when on, the grooming workflows post an update to the change's ticket after its first spec lands`
    - `pr_opened`: `when on, `docket-implement-next` posts an update to the change's ticket after its pull request opens`
    - `pr_merged`: `when on, `docket-finalize-change` posts an update to the change's ticket after it closes out the merged change`
    - `results_completed`: `when on, `docket-implement-next` posts an update to the change's ticket after a results-only change completes`
  - In the `**`tickets`**` paragraph, replace "The five `auto` switches are reported by the ticket context, but nothing acts on them yet." with: "The four `auto` switches each let the workflow that just finished an event post that event's update to the change's ticket by itself, as a comment, with nothing asked. Off (the default) means nothing is posted or reported, and a switch does nothing for a change with no ticket. In a private repository an automatic update posts nothing and reports one line."
  - Keep the hard wrap near 100 columns.

- [ ] **Step 6: Update the pinned rows.** In `internal/repoguard/prose_contracts_test.go`, in the `change_0592_tickets_any_layer` row for `docs/reference/config-keys.md`:
  - Delete the present entry ``"| `tickets.auto.change_created` | `false` | any layer |"``.
  - Add absent entries ``"tickets.auto.change_created"``, `"nothing acts on them yet"`, and `"; no effect yet |"`.
  - In the `.docket.example.yml` row, add absent entries `"change_created"` and `"does not act on them yet"`.

- [ ] **Step 7: Regenerate and verify.** Run `go generate ./internal/assets/`, then `go test ./internal/config/ ./internal/app/ ./internal/repoguard/ ./internal/assets/ -count=1`. Expected: PASS, including `TestDecodeRejections`, `TestProseContracts`, and the example-correspondence test. Then re-run the Step 3 grep. Expected: hits only in `internal/app/ticket_context_test.go`, the new `decode_test.go` rejection row, and records under `docs/superpowers/` and `docs/results/`.

- [ ] **Step 8: Mutation check.** Back up `internal/config/schema.go`, re-add the leaf, and run `go test ./internal/config/ -run TestDecodeRejections -count=1`. Expected: FAIL. Restore from the backup and re-run. Expected: PASS.

- [ ] **Step 9: Commit.**

```bash
git add internal/config internal/app/config.go internal/app/config_test.go .docket.example.yml internal/assets/embedded/tree/.docket.example.yml docs/reference/config-keys.md internal/repoguard/prose_contracts_test.go
git commit -m "refactor(config): retire tickets.auto.change_created; describe the four switches"
```

---

### Task 2: The automatic path in `docket-update-ticket`

**Build tier:** standard

**Files:**
- Create: `skills/docket-update-ticket/references/automatic.md`
- Modify: `skills/docket-update-ticket/SKILL.md` (the opening paragraph, plus a routing paragraph right after `## Convention (load first — blocking)`)
- Create: `internal/repoguard/ticket_auto_updates_contracts_test.go`
- Modify: `internal/repoguard/prose_contracts_test.go` (the `change_0585_update_ticket_skill` row for `SKILL.md`), `internal/repoguard/budgets_test.go` (the `docket-update-ticket/SKILL.md` row, plus a new `docket-update-ticket/references/automatic.md` row)
- Regenerate: the embedded mirrors of both skill files

**Interfaces:**
- Produces:
  - `type autoTicketContract struct { file string; present, absent []string }`
  - `var autoTicketContracts []autoTicketContract`
  - `func TestAutoTicketUpdateContracts(t *testing.T)` in package `repoguard`. It matches whitespace-collapsed with the existing `collapseWS` and reads files with the existing `readRepoFile`. Tasks 3 and 4 append rows and raise its population floor.
- Produces: the routing sentence a workflow relies on. When invoked by a workflow after an event, the skill reads `references/automatic.md` first.

- [ ] **Step 1: Write the failing contract test.** Create `internal/repoguard/ticket_auto_updates_contracts_test.go`. The table holds these rows. Matching is whitespace-collapsed, so present and absent are both checked file-wide.
  - `skills/docket-update-ticket/SKILL.md`:
    - present: `"This skill runs inline, in the session or workflow agent that invokes it"`, `"or when a docket workflow invokes it after an event whose `tickets.auto` switch is on"`, ``"read `references/automatic.md` now (blocking)"``
    - absent: `"Nothing here runs on its own after an event"`, `"and only when the person asks: it posts"`
  - `skills/docket-update-ticket/references/automatic.md`:
    - present: `"The caller supplies the change and the event."`, `"Never ask and never wait."`, `"post nothing and report one line"`, `"when `visibility` is anything other than `shared`"`, `"automatic updates are not available in a private repository"`, `"when no connector for `ticket.provider` is available in this session"`, `"an uncertain outcome is reported as uncertain"`, ``"`ticket.state` is `not-linked`:** post nothing and say nothing"``, `"Never retry, and never touch the calling run"`, `"no halt, no blocked marker, no changed disposition, and no metadata write"`, `"A duplicate comment is acceptable and is deleted by hand."`, `"Never load `references/private.md` on this path."`
  - The test runs a population floor (`checks < <total phrases>` → `t.Fatalf`), fails closed on an unreadable file, and has a `non_vacuity` subtest: a missing phrase and a present-forbidden phrase are each detected over a synthetic string, and a phrase split by a newline still matches.

- [ ] **Step 2: Run it to verify it fails.** Run: `go test ./internal/repoguard/ -run TestAutoTicketUpdateContracts -count=1`. Expected: FAIL. `automatic.md` cannot be read, and the opening phrases are missing.

- [ ] **Step 3: Write `references/automatic.md`.** It holds the decision-4 rules, matching the shape of `shared.md` and `private.md`. Starting copy:

```markdown
# Automatic updates

Loaded from `SKILL.md` when a docket workflow invoked this skill after an event whose switch is on. The rules in `SKILL.md` apply except where this file replaces them.

- **The caller supplies the change and the event.** Take both from the workflow's direction. Nothing is worked out from a person's words, and no one is mentioned: skip sections 1 and 5.
- **Never ask and never wait.** Show no draft. Wherever the on-request path would ask the person something, post nothing and report one line.
- **Read first.** Run the `context.ticket` read yourself with `--id`, `--event`, and `--json`, as on request, so the update is composed from one pinned commit.
- **`ticket.state` is `not-linked`:** post nothing and say nothing.
- **Otherwise post nothing and report one line** when `ticket.state` is `partial` (the ticket link is broken); when `event.happened` is `false` (name the missing fact from `event.reason`); when `visibility` is anything other than `shared` (`automatic updates are not available in a private repository`); when no connector for `ticket.provider` is available in this session; or when the post failed or its outcome is uncertain (an uncertain outcome is reported as uncertain).
- **Otherwise post** exactly what an on-request update for this event posts in a shared repository: its row of section 3's table, the editorial standard, and the links. **Read `references/shared.md` now (blocking).** Never load `references/private.md` on this path.
- **Report one line** and return to the workflow: the update as posted, only with the connector's confirmation, naming the ticket and the event (`Posted the PR-opened update to CLOUD-123.`); or what was not posted and why (`Not posted to CLOUD-123 (PR opened): no Jira connector is available in this session.`).
- **Never retry, and never touch the calling run:** no halt, no blocked marker, no changed disposition, and no metadata write. A duplicate comment is acceptable and is deleted by hand.
```

- [ ] **Step 4: Rewrite the `SKILL.md` opening and add the route.**
  - Replace the first body paragraph with: "This skill runs inline, in the session or workflow agent that invokes it: when the person asks, or when a docket workflow invokes it after an event whose `tickets.auto` switch is on. It posts one short comment to a change's linked ticket with that session's own tools."
  - Right after the `## Convention` paragraphs, add: "**Invoked by a workflow.** When a docket workflow invoked this skill after an event, **read `references/automatic.md` now (blocking)** before section 1. It replaces sections 1, 2, and 5, and every point where this skill would ask or wait."
  - Leave the frontmatter `description` unchanged. `TestUpdateTicketSkillIsInline` pins it, and workflows invoke the skill by name.

- [ ] **Step 5: Repoint the old row.** In `prose_contracts_test.go`, in the `change_0585_update_ticket_skill` row for `skills/docket-update-ticket/SKILL.md`, replace the present entry `"This skill runs inline, in your own session, and only when the person asks"` with `"This skill runs inline, in the session or workflow agent that invokes it"`.

- [ ] **Step 6: Budgets and mirror.** Run `go generate ./internal/assets/`. Then run `go test ./internal/repoguard/ -run TestSkillSizeBudgets -count=1` to read the actual counts, and set:
  - the `docket-update-ticket/SKILL.md` row to its new counts (note `0587: the opening covers workflow invocation; the automatic path routes to references/automatic.md`);
  - a new `{"docket-update-ticket/references/automatic.md", <lines>, <words>}` row (note `0587: new — the automatic path`).

- [ ] **Step 7: Verify.** Run: `go test ./internal/repoguard/ ./internal/app/ ./internal/assets/ -count=1`. Expected: PASS, including `TestUpdateTicketSkillIsInline` (which now also sees the `automatic.md` route), `TestUpdateTicketSkillNamesEveryReadToken`, `TestProseContracts`, and `TestAutoTicketUpdateContracts`.

- [ ] **Step 8: Mutation check.** With backups, run each probe with `go test ./internal/repoguard/ -run TestAutoTicketUpdateContracts -count=1`, and restore after each:
  - delete the `Never retry` bullet → FAIL;
  - re-add "Nothing here runs on its own after an event" to `SKILL.md` → FAIL;
  - delete the routing paragraph → FAIL.

- [ ] **Step 9: Commit.**

```bash
git add skills/docket-update-ticket internal/assets/embedded/tree/skills/docket-update-ticket internal/repoguard/ticket_auto_updates_contracts_test.go internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go
git commit -m "feat(skills): post a ticket update when a workflow invokes the update skill"
```

---

### Task 3: The shared definition, the five sites, and the site guard

**Build tier:** premium. Named risk: a vacuous site guard, and an autonomous run that must never stop or change disposition because of an automatic update.

**Files:**
- Create: `internal/app/ticket_auto_sites_test.go`
- Modify:
  - `skills/docket-convention/SKILL.md`: a new section `### Automatic ticket updates (shared definition)`, placed after `### Autonomous grooming (shared definition)` and before `### Discovered work`.
  - `skills/docket-groom-next/SKILL.md`: Step 5.
  - `skills/docket-auto-groom/SKILL.md`: Step 5, plus the Step 6 report.
  - `skills/docket-implement-next/SKILL.md`: Step 7, between the **Mark implemented.** and **Verify the run.** paragraphs.
  - `skills/docket-implement-next/references/results-artifact.md`: section 6, after the numbered list and before "End the run `advanced`".
  - `skills/docket-finalize-change/SKILL.md`: step 9, after the disposition bullets.
- Modify: `internal/repoguard/ticket_auto_updates_contracts_test.go` (append rows and raise the floor), `internal/repoguard/budgets_test.go` (six rows)
- Regenerate: the embedded mirrors of the six skill files

**Interfaces:**
- Consumes: `ticketEvents` and the `TicketEvent*` constants (`internal/app/ticket_context.go`). Also the routing from Task 2: the update skill reads `references/automatic.md` when a workflow invokes it.
- Produces (test-only, package `app`):
  - `var ticketSiteFiles = map[string]string` mapping each repo-relative site file to its event constant:
    - `skills/docket-groom-next/SKILL.md` → `TicketEventSpecWritten`
    - `skills/docket-auto-groom/SKILL.md` → `TicketEventSpecWritten`
    - `skills/docket-implement-next/SKILL.md` → `TicketEventPROpened`
    - `skills/docket-implement-next/references/results-artifact.md` → `TicketEventResultsCompleted`
    - `skills/docket-finalize-change/SKILL.md` → `TicketEventPRMerged`
  - `const ticketDefinitionFile = "skills/docket-convention/SKILL.md"`
  - `type ticketInvocation struct { rel, text string; tokens []string; directed bool }`
  - `func scanTicketInvocations(files map[string]string) []ticketInvocation`: pure, over repo-relative path → content.
  - `func TestTicketAutoUpdateSites(t *testing.T)`

- [ ] **Step 1: Write the site guard.** In `internal/app/ticket_auto_sites_test.go`:
  - **Scan.** `scanTicketInvocations` does the following:
    1. Split each file into blocks on blank lines (`\n[ \t]*\n`), and collapse each block's whitespace with `strings.Join(strings.Fields(b), " ")`. A hard-wrapped paragraph is then one string.
    2. A block is an **invocation** when it holds at least one un-negated match of `nearRe`: the backticked name `` `docket-update-ticket` `` within 40 characters of an invoke verb, in either order:
       - ``(?i)\binvok(?:e|es|ed|ing)\b[^.]{0,40}`docket-update-ticket` ``
       - `` `docket-update-ticket`[^.]{0,40}\binvok(?:e|es|ed|ing)\b ``
    3. A match is **negated** when its span, plus the 30 characters before it, matches ``(?i)(?:\b\w*not|\bnever|\w+n['’]t)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b``. This is the same negation shape as `negatedInvokeRe` in `internal/repoguard/skill_handoff_sites_test.go`.
    4. **Tokens** are the distinct captures of `` tickets\.auto\.([a-z0-9_]+)|--event\s+([a-z0-9_]+) ``, so `--event <event>` and `tickets.auto.<event>` capture nothing.
    5. `directed` is `strings.Contains(block, "DIRECTED to:")`.
  - **Test.** `TestTicketAutoUpdateSites` walks `../../skills` for `*.md`, keys each file as `skills/<rel>` with forward slashes, scans, and asserts:
    1. a **site** is an invocation with at least one token, and the set of files holding a site equals the keys of `ticketSiteFiles`, with exactly one site per file;
    2. every site has `directed` and exactly one token;
    3. every site's token is in `ticketEvents`; every `ticketEvents` entry has at least one site; and the site's token equals `ticketSiteFiles[rel]`;
    4. exactly one invocation has no token, and it is in `ticketDefinitionFile`;
    5. population floor: there are exactly 5 sites.
  - A `non_vacuity` subtest feeds `scanTicketInvocations` synthetic files and asserts:
    - a passive "`docket-update-ticket` is invoked **DIRECTED to:** … `tickets.auto.pr_opened`" block is an invocation with token `pr_opened`;
    - the same text hard-wrapped over three lines still yields that invocation;
    - "the sweep never invokes `docket-update-ticket`" is not an invocation;
    - a block naming `tickets.auto.spec_written` and `tickets.auto.pr_opened` yields two tokens;
    - `--event <event>` yields none.

- [ ] **Step 2: Run it to verify it fails.** Run: `go test ./internal/app/ -run TestTicketAutoUpdateSites -count=1`. Expected: FAIL. The top-level test reports zero sites and no definition, and `non_vacuity` passes.

- [ ] **Step 3: Write the convention's shared definition.** Add `### Automatic ticket updates (shared definition)` to `skills/docket-convention/SKILL.md`. Starting copy:

```markdown
A `tickets.auto.<event>` switch lets the workflow that just finished an event post that event's update to the change's ticket by itself. Each workflow's own site names its event and the point it fires at. Every site does these three things, in order:

1. **Read.** Run the `context.ticket` operation (argv from the capability catalog) with `--id <id> --event <event> --json`.
2. **Stay silent** when `context.event.switch` is `false` or `context.ticket.state` is `not-linked`: do nothing more and say nothing — no post, no prompt, no report line.
3. **Otherwise invoke `docket-update-ticket`** with the site's own `DIRECTED to:` direction, which names the change and the event and says the update is automatic.

A read that does not return `result: applied` costs one report line, `Ticket update for #<id> not checked: <reason>`, and the run carries on. An automatic update never prompts, waits, queues, retries, or records an update as owed. Its outcome never changes a run's disposition, writes a halt or blocked marker, or stops a later step: a failed or impossible post is one report line. A replayed success posts like a first success, so a retried run may post a duplicate, which is deleted by hand. Only the sites post. A closeout run as housekeeping (the maintenance sweep, a status refresh, or implement-next's closeout of a named change's merged dependencies) posts nothing, and when a stack root lands only the root's ticket is updated.
```

- [ ] **Step 4: Write the five site paragraphs.** Each one is its own blank-line-separated paragraph and names its event once, as `tickets.auto.<event>`. Each carries this direction, adapted only where noted:

> **Ticket update.** <when it fires>, follow the convention's *Automatic ticket updates* for `tickets.auto.<event>`; at its third step, invoke `docket-update-ticket` **DIRECTED to:** post the <event words> update for this change as an automatic update; nobody is present, so anything the skill would ask is answered by posting nothing and reporting one line; then return here. <site-specific sentences>

Site-specific content (decision 1, decision 5):
- **groom-next.**
  - When it fires: "After a spec exit's transaction lands".
  - Add: "The person being in the session changes nothing: the switch is the consent, so nothing is asked. Kill, defer, revise, and re-enable never post."
  - Placement: move Step 5's closing "STOP — grooming never implements." to after this paragraph.
- **auto-groom.**
  - When it fires: "After a spec exit's transaction lands, before step 1 selects the next stub".
  - Add: "An abstain never posts."
  - Placement: before Step 5's "Return to step 1." (split that sentence out).
  - In Step 6, add "and each ticket-update line" to the report.
- **implement-next Step 7.**
  - When it fires: "After `change.mark-implemented` returns its applied outcome (a retry's earlier applied outcome included)".
  - Add: "The run's disposition is `advanced` whatever the update's outcome, and its line goes in the final report."
- **results-artifact.md §6.**
  - When it fires: "After `change.complete` returns `completed` or `already-completed`".
  - Add: "The run still ends `advanced`, and the line goes in the final report."
  - Keep this file's hard wrap.
- **finalize-change step 9.**
  - When it fires: "After `finalize.closeout` returns `done-archived`, `stacked-merged`, `root-archived`, or `already-closed-out` for a change this run closed out".
  - Add: "Post once for each change a closeout call was made for, never for a `root-archived` result's `carried_ids`, and never after `contended`, `blocked`, or `unknown`. The line goes in the final report, and the update's outcome never changes the run's disposition."

- [ ] **Step 5: Run the guard.** Run: `go test ./internal/app/ -run 'TestTicketAutoUpdateSites|TestUpdateTicketSkillNamesEveryReadToken' -count=1`. Expected: PASS.

- [ ] **Step 6: Append contract rows.** Append to `autoTicketContracts`, and raise its floor to the new total:
  - `skills/docket-convention/SKILL.md`:
    - present: `"### Automatic ticket updates (shared definition)"`, ``"with `--id <id> --event <event> --json`"``, ``"when `context.event.switch` is `false` or `context.ticket.state` is `not-linked`"``, `"no post, no prompt, no report line"`, ``"Ticket update for #<id> not checked: <reason>"``, `"never changes a run's disposition, writes a halt or blocked marker, or stops a later step"`, `"A replayed success posts like a first success"`, `"when a stack root lands only the root's ticket is updated"`
  - `skills/docket-groom-next/SKILL.md`:
    - present: `"the switch is the consent, so nothing is asked"`, `"Kill, defer, revise, and re-enable never post."`
  - `skills/docket-auto-groom/SKILL.md`:
    - present: `"before step 1 selects the next stub"`, `"An abstain never posts."`
  - `skills/docket-implement-next/SKILL.md`:
    - present: `"(a retry's earlier applied outcome included)"`, ``"The run's disposition is `advanced` whatever the update's outcome"``
  - `skills/docket-implement-next/references/results-artifact.md`:
    - present: ``"After `change.complete` returns `completed` or `already-completed`"``
  - `skills/docket-finalize-change/SKILL.md`:
    - present: ``"never for a `root-archived` result's `carried_ids`"``, ``"never after `contended`, `blocked`, or `unknown`"``, ``"`already-closed-out` for a change this run closed out"``

- [ ] **Step 7: Budgets and mirror.** Run `go generate ./internal/assets/`. Read the counts from `TestSkillSizeBudgets`, then raise the six rows:
  - `docket-convention/SKILL.md`: `0587: the Automatic ticket updates shared definition`
  - `docket-groom-next/SKILL.md`, `docket-auto-groom/SKILL.md`, `docket-implement-next/SKILL.md`, `docket-implement-next/references/results-artifact.md`, `docket-finalize-change/SKILL.md`: each `0587: the automatic ticket-update site`

- [ ] **Step 8: Verify.** Run: `go test ./internal/app/ ./internal/repoguard/ ./internal/assets/ -count=1`. Expected: PASS. This includes `TestSkillHandoffSites`-family guards, `TestLivingDocsAlignment`, `TestAutoTicketUpdateContracts`, and `TestSkillSizeBudgets`.

- [ ] **Step 9: Mutation check.** With backups, run `go test ./internal/app/ -run TestTicketAutoUpdateSites -count=1` after each probe, restoring each time. Each must FAIL:
  - delete the finalize site;
  - delete `DIRECTED to:` from the implement-next site;
  - swap `tickets.auto.pr_opened` → `tickets.auto.pr_merged` in implement-next;
  - copy the auto-groom site into `skills/docket-status/SKILL.md`;
  - name `tickets.auto.change_created` in the groom-next site;
  - delete the convention's step 3 line.

  Then delete each pinned convention clause one at a time and run `go test ./internal/repoguard/ -run TestAutoTicketUpdateContracts -count=1`. Expected: FAIL each time. Finish with every file restored and both tests PASS.

- [ ] **Step 10: Commit.**

```bash
git add skills/docket-convention/SKILL.md skills/docket-groom-next/SKILL.md skills/docket-auto-groom/SKILL.md skills/docket-implement-next/SKILL.md skills/docket-implement-next/references/results-artifact.md skills/docket-finalize-change/SKILL.md internal/assets/embedded/tree/skills internal/app/ticket_auto_sites_test.go internal/repoguard/ticket_auto_updates_contracts_test.go internal/repoguard/budgets_test.go
git commit -m "feat(skills): post ticket updates automatically after each event whose switch is on"
```

---

### Task 4: The tickets concepts page, its indexes, and the doc entries

**Build tier:** premium. Named risk: every statement on the page must match the skills and the binary as built.

**Files:**
- Create: `docs/concepts/tickets.md`
- Modify:
  - `docs/concepts/README.md` and `docs/README.md` (one index entry each, appended after *Learnings and ADRs as memory*);
  - `docs/reference/config-keys.md` (the pointer sentence in the `**`tickets`**` paragraph);
  - `docs/reference/glossary.md` (a pointer sentence in `### docket-link-ticket`, plus the `### docket-update-ticket` entry);
  - `docs/reference/skills-and-agents.md` (the `docket-update-ticket` bullet);
  - `internal/repoguard/ticket_auto_updates_contracts_test.go` (append rows and raise the floor).

**Interfaces:**
- Consumes: the behavior Tasks 1–3 built. The page's sources of truth are:
  - `skills/docket-link-ticket/SKILL.md` and its `references/create.md` and `references/import.md`;
  - `skills/docket-update-ticket/` (all three references);
  - the convention's **Ticket link.** paragraph and *Automatic ticket updates*;
  - `ContextTicketRead` (`internal/app/ticket_context.go`);
  - the `tickets` rows of `docs/reference/config-keys.md`.

- [ ] **Step 1: Write the failing contract rows.** Append to `autoTicketContracts`, and raise the floor:
  - `docs/concepts/tickets.md`:
    - present: `"# Tickets"`, `"## The problem it solves"`, `"## The moving parts"`, `"## The invariants"`, `"## Why docket never creates a ticket by itself"`, `"### Creating a ticket is a person's action"`, `"### There is no one right moment to create it"`, `"## Decided in"`, `"A ticket is created only when a person asks and approves the draft. No setting changes that."`, `"link an existing ticket to a change"`, `"create a ticket from a change"`, `"make a change from a ticket"`, ``"`tickets.auto.spec_written`"``, ``"`tickets.auto.pr_opened`"``, ``"`tickets.auto.pr_merged`"``, ``"`tickets.auto.results_completed`"``, `"retry uploads after a timeout"`, `"agents report on tickets, and people create them"`, `"an automatic update posts nothing and reports one line"`
    - absent: `"change_created"`, `"was once planned"`, `"dropped"`
  - `docs/concepts/README.md`: present `"[Tickets](./tickets.md)"`
  - `docs/README.md`: present `"[Tickets](concepts/tickets.md)"`
  - `docs/reference/config-keys.md`: present `"Creating a ticket has no switch: it is always on request"`, `"../concepts/tickets.md#why-docket-never-creates-a-ticket-by-itself"`
  - `docs/reference/glossary.md`:
    - present: `"Docket never creates a ticket by itself"`, `"../concepts/tickets.md#why-docket-never-creates-a-ticket-by-itself"`, `"or by itself after an event whose switch is on"`
    - absent: `"to a change's linked ticket when you ask. It posts"`
  - `docs/reference/skills-and-agents.md`:
    - present: ``"when you ask, or automatically after an event whose `tickets.auto` switch is on"``
    - absent: `"linked ticket when you ask (spec written"`

- [ ] **Step 2: Run them to verify they fail.** Run: `go test ./internal/repoguard/ -run TestAutoTicketUpdateContracts -count=1`. Expected: FAIL. `tickets.md` cannot be read.

- [ ] **Step 3: Write `docs/concepts/tickets.md`.**
  - Structure, in order: *The problem it solves*; *The moving parts* (it opens with an ASCII diagram like `reconcile.md`'s, then covers each part decision 9 lists: the link, the three ways, updates on request and automatic, the four events as one table, what goes with an update in shared and private repositories, the connector, the settings, and when something goes wrong); *The invariants* (decision 9's seven); *Why docket never creates a ticket by itself* (decision 9's drafted section, with every point and the worked example kept); and *Decided in*.
  - *Decided in* cites ADR-0162, ADR-0161, and ADR-0044 by number with no link, like the other docs citing ADRs whose files are not on the integration branch. The new ADR is a coordinator follow-up.
  - Rules for the page (decision 9): no change or PR citation; no mention of a removed or planned switch; private repositories are described as they behave now; no ticket-system claim that is not true of ticket systems in general. Hard-wrap near 80 columns, like the other concepts pages.

- [ ] **Step 4: Check each statement against the sources by reading.** Check every sentence on the page against the sources listed under *Interfaces*. Where a line in decision 9 disagrees with a skill or the binary, the skill or the binary wins: fix the page, and record the disagreement in the commit message body. Then re-read the doc entries edited in Steps 5–6 the same way. Any sentence they carry about where `docket-update-ticket` runs must match Task 2's opening: it runs in whichever session or workflow agent invokes it.

- [ ] **Step 5: Index entries and pointers.**
  - Index entries:
    - `docs/concepts/README.md`: `- [Tickets](./tickets.md) — how a change links to a ticket in your team's ticket system, and what docket posts there.`
    - `docs/README.md`: the same line, with the link written as `concepts/tickets.md`.
  - In `config-keys.md`'s `**`tickets`**` paragraph, after the switches sentence, add: "Creating a ticket has no switch: it is always on request ([why](../concepts/tickets.md#why-docket-never-creates-a-ticket-by-itself))."
  - In the glossary's `### docket-link-ticket` **Used for:** paragraph, add: "Docket never creates a ticket by itself ([why](../concepts/tickets.md#why-docket-never-creates-a-ticket-by-itself))."

- [ ] **Step 6: The `docket-update-ticket` entries.**
  - Glossary: rewrite the first sentence to "The skill an agent session uses to post an update to a change's linked ticket, when you ask or by itself after an event whose switch is on."
  - Keep the pinned private-repository sentence. Add: "An automatic update in a private repository posts nothing and reports one line."
  - `skills-and-agents.md`: replace "when you ask (spec written, …)" with "when you ask, or automatically after an event whose `tickets.auto` switch is on (spec written, …)", keeping the rest of the bullet.
  - Keep every phrase the `change_0585_update_ticket_skill` and `change_0586_update_ticket_private` rows pin.

- [ ] **Step 7: Verify.** Run: `go test ./internal/repoguard/ -count=1`. Expected: PASS, including `TestAutoTicketUpdateContracts`, `TestProseContracts`, and `TestLivingDocsAlignment`. Then render-check the anchor: the heading `## Why docket never creates a ticket by itself` slugs to `#why-docket-never-creates-a-ticket-by-itself`.

- [ ] **Step 8: Mutation check.** With backups, run `go test ./internal/repoguard/ -run TestAutoTicketUpdateContracts -count=1` after each probe, restoring each time. Each must FAIL:
  - delete the *There is no one right moment to create it* subsection;
  - delete the no-setting invariant;
  - drop the `docs/README.md` index line;
  - add "tickets.auto.change_created" to the page.

  Then add "see change 587" to the page and run `go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`. Expected: FAIL. Restore.

- [ ] **Step 9: Commit.**

```bash
git add docs/concepts/tickets.md docs/concepts/README.md docs/README.md docs/reference/config-keys.md docs/reference/glossary.md docs/reference/skills-and-agents.md internal/repoguard/ticket_auto_updates_contracts_test.go
git commit -m "docs(concepts): explain how ticket work fits together and why creation is on request"
```
