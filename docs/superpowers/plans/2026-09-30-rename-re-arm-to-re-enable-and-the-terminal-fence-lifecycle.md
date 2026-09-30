<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0474 — Rename re-arm to re-enable and retire the lifecycle 'terminal' and non-run 'fence' names](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0474-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle.md)**
<!-- docket:backlink:end -->
# Re-enable, final, closing half and shared-setting guard (ADR-0129 family (d)) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repo the plan is executed by `docket-build` (one tier worker per task, sequential, one full-suite gate at the end).

**Goal:** Apply ADR-0129 family (d), rows 53–59 and 59a–59c, as a hard cut with no aliases. The auto-groom "re-arm" becomes "re-enable" (`outcome: re-enable`, `nothing-to-re-enable`). The config coordination-key "fence" becomes the shared-setting guard (`shared-setting-ignored`). Change- and ADR-lifecycle "terminal" becomes "final" (seven wire codes, `Status.Final()`, messages, commit subjects, comments), and "terminal half" becomes "closing half". `references/terminal-close-out.md` becomes `references/close-out.md`. Every other non-run "fence" is renamed, so "fence" means only the run fence. The retired spellings are sealed.

**Architecture:** This is a behavior-preserving rename. Tasks are split by rename family so a reviewer can accept or reject each one on its own: (1) re-enable; (2) the shared-setting guard in `internal/config`; (3) "final" wire codes, Go identifiers, messages and commit subjects; (4) the lifecycle-"terminal" prose and comment sweep, including "closing half"; (5) the close-out reference file rename; (6) the other non-run fences; (7) the seal, plus the closing whole-repo grep and verification. Any task that edits a file under `skills/` regenerates the embedded tree in the same commit. No agent file changes, so the harness goldens must not change.

**Tech Stack:** Go (`internal/app`, `internal/config`, `internal/domain`, `internal/render`, `internal/repository`, `internal/reposetup`, `internal/cli`, `internal/gitcli`, `internal/repoguard`), Markdown skills and docs, `go generate ./internal/assets` (`cmd/genassets`), build tags `integration` and `e2e`, and the Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-design.md` on the `docket` metadata branch (synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-design.md`). Decision record: ADR-0129 rows 53–59 and 59a–59c, edited in place at grooming. The build does **not** touch ADR-0129.

**Worktree:** `/Users/homer/dev/docket/.worktrees/rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle` (branch `refactor/rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle`, base `e3352114cd386e72d04718f1719ae47df7bd9941`). Every command below runs from this directory.

## Global Constraints

- **Behavior is unchanged everywhere.** Every rename keeps behavior identical: `not-final` is reported for exactly the statuses `not-terminal` was, and `Status.Final()` is true for exactly `done` and `killed`. Do not fix the semantics you notice along the way. If a killed change can reach `finalize cleanup`'s default branch, note it in `NOTES` as a follow-up; do not fix it here (spec *Out of scope*).
- **Hard cut, no aliases** (ADR-0129 Decision 2). Old spellings are not accepted, not mapped, and not mentioned in refusal text. `outcome: rearm` must be refused as `invalid-outcome` once this lands.
- **Wire-token map** (hard cut, sealed in Task 7):

  | Row | Old | New |
  |---|---|---|
  | 54 | `change.groom` `outcome: rearm` (`groom_outcomes`) | `re-enable` |
  | 55 | `nothing-to-rearm` (also in `finding_codes`) | `nothing-to-re-enable` |
  | 56 | `fenced-setting-ignored` | `shared-setting-ignored` |
  | 57 | `terminal-backlink-pending` | `final-backlink-pending` |
  | 57 | `terminal-notes-frozen` | `final-notes-frozen` |
  | 58 | `change-terminal-claim-stamp` | `change-final-claim-stamp` |
  | 58 | `drop-terminal-claimed-at` | `drop-final-claimed-at` |
  | 59 | `not-terminal` (finalize cleanup) | `not-final` |
  | 59 | `skipped-terminal` (retarget child outcome) | `skipped-final`, kept exactly as written although it is also emitted for stacked-merged children (D3) |
  | 59 | `adr-update-after-terminal` | `adr-update-after-final` |
  | 59a | `skills/docket-convention/references/terminal-close-out.md` | `skills/docket-convention/references/close-out.md` |

- **Concept map:** re-arm / re-armed / Re-arm → re-enable / re-enabled / Re-enable. A lifecycle "terminal" (a change or ADR end state, or the record, status, transition, shape, write or backlinks at that end state) → "final". "terminal record" → "archived record" where it means the archived record (ADR-0129 row 64). The stretch that *leads to* the end state ("terminal half", "terminal path", "terminal sequence") → "closing half" / "closing path" / "closing sequence" (D2). The config coordination-key fence → the shared-setting guard. Other non-run fences → check / refuse / guard wording.
- **Explicitly kept (spec §D; never rename):**
  - The process-level "terminal": gate runs, supervisors, participants and drive terminals, `terminal_receipt`, the JSON `terminal` / `terminal_status` / `terminal_turn` / `terminal_observed_at` keys, `terminal.json`, the run-record `Terminal` field, `record-terminal`, `incumbent-nonterminal`, "nonterminal", a run's "Terminal disposition" (implement-next, finalize-change), "terminal result/envelope/verdict/observation" (status, build), `mapTerminalDrive`, `runToTerminal`, `TerminalWaiting`, and everything in `internal/gatedrive`, `internal/process`, `internal/codexentry`, `internal/suiterunner` and `internal/app/runtracker*` / `gate*` / `agent_guardian*` / `root_entry*`.
  - The test names the spec keeps: `TestIntegrationRecordOpsClaimTerminalGateRefused`, `TestHealthTerminalFallbackNamesEveryCause`, the githubcli terminal-state PR tests, and `TestIntegrationRepoMigrationNoTerminalPublishResurrection`.
  - A decision procedure's last branch, the sense `TestHealthTerminalFallbackNamesEveryCause` keeps: the reposetup classifier's "terminal conflict" / "Terminal fall-through" / "terminal healthy conjunction", `repository_check.go`'s "terminal conflict", the transaction engine's "terminal outcome", and stack rule 3's "resolves terminally" / "terminal, never recursing". Only the lifecycle clause "done is terminal" in `internal/domain/stack_test.go` becomes "done is final".
  - A GitHub pull request's "terminal state" (closed or merged).
  - `terminal_publish`, `terminalPublish`, and the obsolete feature name "terminal publish" / "terminal publication" / "terminal-publish".
  - TTY and harness senses: "on a terminal", "a crashed terminal", `GIT_TERMINAL_PROMPT`, Cursor's `terminalAllowlist`.
  - The historical Bash array name `DOCKET_STATUSES(_ACTIVE/_TERMINAL)` in the `internal/app/repository_prepare.go` comment. It names a dropped identifier, so it is quoted history.
  - Markdown code fences and `---` frontmatter fences (`fenceRun`, `isBareFence`, `isFrontmatterFence`, `fenceMask`, "pre-fence insertion point", "fence-aware", "unfenced invocation", …) and every run-fence spelling (`runtracker_fence*`, `MutationFenceError`, `fenceRefusalReasonMessage`, `workspaceFenceRefusal`, `prFenceRefusal`, `fenceNextAction`, "stale-run-id fence", `TestIntegrationRunFence*`, `tests/test_go_integration_app_runfence.sh`, "Fenced" meaning a code fence in `TestChangeGroomReEnableFencedMarkerAgreesWithBoard`).
  - The frozen fixture directory `testdata/repositories/v0.9.2/fenced-machine-keys/` (and the string `"fenced-machine-keys"` that loads it), and ADR-0019's filename `0019-global-config-fence-classification.md`.
  - The unrelated `internal/suiterunner/signal.go` "re-arm" (a signal re-arm), and generic "re-enabling" (learnings, recursive self-dispatch). "defence" in `internal/install/txn.go` is an unrelated word.
- **Frozen, never edited:** `docs/changes/**`, `docs/results/**`, `docs/superpowers/**` (except this plan), `docs/adrs/**` (ADR-0129 included), `testdata/repositories/**`, `internal/repository/testdata/**`, `internal/install/legacydata/`, `internal/install/testdata/`, `docs/reference/harness/fixtures/`, and the history comments in `internal/repoguard/budgets_test.go` (for example "0382: +typed rearm exit").
- **Regenerated, never hand-edited:** `internal/assets/embedded/**` (`go generate ./internal/assets`). Run it in the same commit as any `skills/` edit. The harness goldens (`internal/harness/*/testdata/golden/**`) must **not** change: no agent, cursor-rule or `.docket.example.yml` wording changes here. If a harness golden test reddens, stop and investigate. Never run `-update` to make it pass.
- **Site derivation:** derive every site by whole-repo grep (AGENTS.md: "Never hand-list the sites of a literal …"). The site lists below were traced at base `e3352114c` and are starting points. The derivation commands in each task and the closing grep in Task 7 are authoritative.
- **Budgets:** `internal/repoguard/budgets_test.go` ceilings are never raised. Swaps are made in place, word for word (`wc -w` counts `re-enable`, `nothing-to-re-enable` and `close-out.md` as one word each). Measured at base (lines/words vs ceiling): `docket-groom-next/SKILL.md` **78/78, 2082/2082 (at both ceilings)**; `docket-auto-groom/SKILL.md` 66/70, 1625/1627; `docket-convention/SKILL.md` 390/400, 7856/7969; `docket-implement-next/SKILL.md` 214/214, 8167/8175; `docket-implement-next/references/edge-paths.md` 118/118, 1550/1554; `docket-finalize-change/SKILL.md` 238/239, 5643/5647; `docket-new-change/SKILL.md` 59/61, 1674/1675; `docket-status/SKILL.md` 129/140, 2974/3065; `docket-convention/references/terminal-close-out.md` 151/240, 1466/2150. The only allowed edit to `budgets_test.go` is Task 5's row move.
- **Build tags:** 139 test files carry `//go:build integration` and `internal/app/finalize_e2e_test.go` carries `//go:build e2e`. A plain `go test` or `go vet` never compiles them. Every task that renames a Go identifier runs all three: `go vet ./...`, `go vet -tags integration ./...` and `go vet -tags e2e ./internal/app/`. A rename missed in a tagged file only fails in its shard.
- **Integration shards:** each shard script (`tests/test_go_integration_*.sh`) selects tests by `SHARD_PREFIX`. A renamed `TestIntegration*` must keep its shard's prefix (checked in Task 7).
- Go test commands always pass `-count=1` (a cached `ok (cached)` is not evidence). Run `gofmt -l` on every Go file you touch and expect no output.
- Stage explicit paths only (`git add -- <path> …`), never `git add -A` / `git add .`. Use `git mv` for the one file rename.
- Mutation tests restore from a backup copy (`cp f f.bak; mutate; test; mv -f f.bak f`), never `git checkout -- f`. Confirm with a count that each mutation landed before you read the test result (learning *phrase-grep-over-wrapped-prose*: count through a whitespace-flattened copy when the phrase can wrap).
- Shell: the agent's interactive shell is zsh and its `grep` is ugrep. Run multi-line shell steps under `bash -c`, use `git grep` or `command grep` for verification, and capture producer output into a variable before grepping it (AGENTS.md pipefail rule).
- Test code in this plan is unverified until run (learning *plan-supplied-test-code-is-unverified*). Prove each new assert can pass, and mutation-test its key where the task says so.
- The build gate runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`), not only the tests named here. Read its budget report even when it is green.

## Review Focus

1. **A rename missed in an integration- or e2e-tagged test file.** A plain `go test ./...` stays green while the shard (or the e2e matrix) fails to compile at the gate. Expected: every task that renames a Go identifier compiles all three build-tag sets (`go vet`, `go vet -tags integration ./...`, `go vet -tags e2e ./internal/app/`) before it commits. Tasks 1, 2, 3 and 6 carry that step.
2. **A renamed integration test leaves its shard and silently stops running.** Expected: each renamed `TestIntegration*` still starts with its shard's `SHARD_PREFIX`, and the repo's `func Test` count is the base count plus exactly the tests this plan adds. Task 7 Step 5 pins both.
3. **A consumer still names a retired spelling.** For example, a skill tells an agent to send `outcome: rearm` (refused as `invalid-outcome`), to read `terminal-close-out.md` (a missing file), or to expect `terminal-notes-frozen`. Expected: none survives in skills, scripts, agent-instruction files, non-test Go string literals or generator output. Task 7's seal pins this, with a hand mutation proving it reddens. Each task also runs its own derivation grep.
4. **A kept sense gets renamed.** Renaming the run record's `terminal` JSON key, `terminal.json`, `terminal_publish`, the `fenced-machine-keys` fixture directory, `GIT_TERMINAL_PROMPT`, a run-fence identifier or a code-fence helper would break storage, fixtures or config. Expected: the kept spellings are byte-identical to base. Task 7 Step 6 diffs them against base, and the closing grep classifies every remaining hit.
5. **A skill file crosses its budget ceiling.** `docket-groom-next/SKILL.md` sits at both ceilings (78 lines, 2082 words). Expected: swaps stay word-for-word and no ceiling rises. Tasks 1, 2, 3, 4 and 5 run `TestSkillSizeBudgets`, and Task 7 asserts that the only `budgets_test.go` change is the row move.

---

### Task 1: Re-enable (ADR-0129 rows 53–55)

**Build tier:** standard — a cross-file rename of one wire token and its consumers (Go, schema vocabulary, CLI help, three skills, docs), plus a new hard-cut test.

**Files:**
- Modify: `internal/app/change_groom.go`, `internal/app/finding_codes.go`, `internal/app/schema_vocab.go`, `internal/cli/change.go`
- Test: `internal/app/change_groom_test.go`, `internal/app/change_groom_integration_test.go` (`//go:build integration`), `internal/app/schema_vocab_test.go`, `internal/repository/decode_test.go` (comment only), `internal/repoguard/prose_contracts_test.go`
- Modify (skills): `skills/docket-groom-next/SKILL.md`, `skills/docket-convention/SKILL.md`, `skills/docket-auto-groom/SKILL.md`
- Modify (docs): `docs/reference/glossary.md`, `docs/guide/designing-before-building.md`
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `app.GroomReEnable GroomOutcome = "re-enable"` (was `GroomRearm`), `app.FCNothingToReEnable FindingCode = "nothing-to-re-enable"` (was `FCNothingToRearm`), test helper `reEnableRequest() ChangeGroomRequest` (was `rearmRequest`), and the result text `"change %04d re-enabled for auto-groom — %s"`. The `groom_outcomes` vocabulary reads `spec trivial revise abstain re-enable`.

- [ ] **Step 1: Retarget the tests first (failing)**

`internal/app/change_groom_test.go`:
- Rename the helper `rearmRequest` → `reEnableRequest`, with the comment "reEnableRequest is a well-formed re-enable request against the fixture at id 2." and `Outcome: GroomReEnable`. Replace every call site in this file and in `change_groom_integration_test.go`.
- Rename the tests, changing `Rearm` → `ReEnable` in each: `TestChangeGroomRearmShapeValidation`, `TestChangeGroomPlanRearmClearsSectionSetsFlagAndBoard`, `TestChangeGroomPlanRearmAppliesSectionEditsInOneRecord`, `TestChangeGroomPlanRearmRefusals`, `TestChangeGroomRearmFencedMarkerAgreesWithBoard` (→ `TestChangeGroomReEnableFencedMarkerAgreesWithBoard`; keep "Fenced", a code fence), `TestChangeGroomResultHumanTextRearm`, `TestChangeGroomPlanRearmRemovesSectionBeforeFollowingSection`. One exception: `TestChangeGroomPlanRearmArmsWithoutABlockedSection` → `TestChangeGroomPlanReEnableEnablesWithoutABlockedSection`.
- Subtest names and messages: `"bare rearm passes"` → `"bare re-enable passes"`, and so on for each `"rearm …"` case (including `"rearm accepts title"` → `"re-enable accepts title"`); "re-arm left …" → "re-enable left …"; "re-arm did not set …" → "re-enable did not set …"; "rearm over a fenced marker" → "re-enable over a fenced marker"; "nothing to re-arm" → "nothing to re-enable"; comments "re-arm"/"rearm" → "re-enable" (including "abstain-append and rearm-removal" → "abstain-append and re-enable-removal").
- Literal expectations: `receipt.Outcome != "rearm"` → `!= "re-enable"`, with its message "want outcome re-enable". Every `"nothing-to-rearm"` → `"nothing-to-re-enable"`. The human text want → `"change 0007 re-enabled for auto-groom — cafe"`.
- Add the hard-cut test right after `TestChangeGroomReEnableShapeValidation`:

```go
// TestChangeGroomRetiredRearmOutcomeRefused pins the hard cut (ADR-0129
// Decision 2, row 54): the retired spelling is not an alias of re-enable, and
// the refusal names only the live outcome set.
func TestChangeGroomRetiredRearmOutcomeRefused(t *testing.T) {
	req := reEnableRequest()
	req.Outcome = GroomOutcome("rearm")
	findings := validateChangeGroomShape(req)
	var msg string
	for _, f := range findings {
		if f.Code == "invalid-outcome" {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("outcome rearm: want invalid-outcome, got %v", findings)
	}
	// The message echoes the offending value via %q ("rearm"), so assert on the
	// listed outcome set only: it must end with the live spelling.
	if !strings.HasSuffix(msg, "must be one of spec, trivial, revise, abstain, re-enable") {
		t.Errorf("invalid-outcome message %q must list exactly the live outcomes ending in re-enable", msg)
	}
}
```

`internal/app/change_groom_integration_test.go`: `TestIntegrationRecordOpsChangeGroomAbstainThenRearmRealGit` → `TestIntegrationRecordOpsChangeGroomAbstainThenReEnableRealGit`. Its comment and messages change "re-arm" → "re-enable" ("stale re-enable = %q …", "a contended re-enable moved …", "re-enable = %q …", "re-enabled record:", "… after re-enable").

`internal/app/schema_vocab_test.go`: `assertVocabMembers(t, v, "groom_outcomes", []string{"spec", "trivial", "revise", "abstain", "re-enable"})`.

`internal/repoguard/prose_contracts_test.go` (the `change_0382_typed_abstain` rows). In the `skills/docket-convention/SKILL.md` row's `present` list, replace the element whose text is `` `outcome: rearm` `` (backticks included) with `` `outcome: re-enable` ``. In the `skills/docket-groom-next/SKILL.md` row's `present` list, replace `` `outcome: rearm` `` with `` `outcome: re-enable` `` and `` `nothing-to-rearm` `` with `` `nothing-to-re-enable` ``. In the comment above them, "the human re-arm" → "the human re-enable" and "hand-edit re-arm" → "hand-edit re-enable".

`internal/repository/decode_test.go`: comment "(rearm)" → "(re-enable)".

- [ ] **Step 2: Run to verify it fails**

Run: `go vet ./internal/app/`
Expected: FAIL, with `undefined: GroomReEnable` (compile error).

- [ ] **Step 3: Rename the implementation**

`internal/app/change_groom.go`:
- The constant block:

```go
	// GroomReEnable re-enables a needs-brainstorm change for autonomous grooming:
	// it sets auto_groomable: true, removes the ## Auto-groom blocked section when
	// present, and applies any owned-section edits (typically the context the
	// abstain asked for) in the same commit. Human-typed or human-attended only.
	GroomReEnable GroomOutcome = "re-enable"
```

- `HumanText`: `if r.Outcome == string(GroomReEnable) { return fmt.Sprintf("change %04d re-enabled for auto-groom — %s", r.ID, r.Revision) }`.
- `validateChangeGroomShape`: `case GroomReEnable:`. Messages: `"spec_markdown is not accepted by the re-enable outcome"`, `"the re-enable outcome removes \"## Auto-groom blocked\" itself; a section edit may not name it"`, and the default `fmt.Sprintf("outcome %q must be one of spec, trivial, revise, abstain, re-enable", req.Outcome)`.
- The plan closure: both `o.req.Outcome == GroomRearm` → `GroomReEnable`; `refuseGroom(string(FCNothingToReEnable), fmt.Sprintf("change %04d has no %s section and is already auto_groomable: true; there is nothing to re-enable", …))`; the comment "there is nothing to re-arm" → "there is nothing to re-enable".
- Comments: "the rearm outcome" → "the re-enable outcome"; "trivial, revise, and rearm outcomes accept it" → "trivial, revise, and re-enable outcomes accept it". Derive the rest with `git grep -n -i -E "re-?arm" -- internal/app/change_groom.go`, which must print nothing when you are done.

`internal/app/finding_codes.go`: `FCNothingToRearm FindingCode = "nothing-to-rearm"` → `FCNothingToReEnable FindingCode = "nothing-to-re-enable"`, and the `AllFindingCodes` entry `FCNothingToRearm,` → `FCNothingToReEnable,`. It keeps its position: "nothing-to-re-enable" still sorts after "not-retitleable" and before "parse-failed". `TestFindingCodeRegistryIntegrity` checks this.

`internal/app/schema_vocab.go`: `string(GroomRearm)` → `string(GroomReEnable)`.

`internal/cli/change.go` (groom `Short`): "(abstain, rearm)" → "(abstain, re-enable)".

Run `gofmt -w` on the touched Go files (the `FindingCode` block realigns).

- [ ] **Step 4: Skills and docs**

Each swap is word for word (Global Constraints, *Budgets*).

`skills/docket-groom-next/SKILL.md` (at both ceilings; add no word):

| Old (quoted) | New |
|---|---|
| description: "a kill, a defer, or a re-arm — or revising" | "a kill, a defer, or a re-enable — or revising" |
| "6. **Re-arm** (abstained or opted-out stubs)" | "6. **Re-enable** (abstained or opted-out stubs)" |
| "with `outcome: rearm`, the pinned" | "with `outcome: re-enable`, the pinned" |
| "A `nothing-to-rearm` refusal" | "A `nothing-to-re-enable` refusal" |
| "of an abstained stub needs no re-arm." | "… needs no re-enable." |
| "5 (revise), and 6 (re-arm) also carry" | "5 (revise), and 6 (re-enable) also carry" |
| "a re-arm returns an abstained row to needs-brainstorm" | "a re-enable returns an abstained row to needs-brainstorm" |

`skills/docket-convention/SKILL.md`:

| Old | New |
|---|---|
| `## Auto-groom blocked` bullet: "(including removal by `outcome: rearm`)" | "(including removal by `outcome: re-enable`)" |
| *Abstain rule*: "Re-arm = a human supplies" | "Re-enable = a human supplies" |
| "applies `change.groom` with `outcome: rearm` (optionally" | "… with `outcome: re-enable` (optionally" |
| "would mislabel a re-armed stub" | "would mislabel a re-enabled stub" |

`skills/docket-auto-groom/SKILL.md`: "the human's re-arm cue" → "the human's re-enable cue".

`docs/reference/glossary.md`:
1. The sections entry: "and\n`rearm` may not name `## Auto-groom blocked`" → "and\n`re-enable` may not name `## Auto-groom blocked`".
2. Heading `### Abstain / re-arm` → `### Abstain / re-enable`. Body: "**Re-arm** is the human supplying" → "**Re-enable** is the human supplying". Example: `"outcome": "rearm"` → `"outcome": "re-enable"`.
3. *Grooming*: "`trivial`, `revise`, `abstain`, or `rearm`." → "… or `re-enable`."
4. *docket-groom-next*: "revise, or\nre-arm." → "revise, or\nre-enable."
5. Vocabulary table row: `` | `groom_outcomes` | `spec` `trivial` `revise` `abstain` `rearm` | `` → `` … `abstain` `re-enable` | ``.
6. Index: `- [Abstain / re-arm](#abstain--re-arm)` → `- [Abstain / re-enable](#abstain--re-enable)`. It keeps its position.

`docs/guide/designing-before-building.md`: "or re-arm it so the autonomous groomer picks it up" → "or re-enable it so the autonomous groomer picks it up".

- [ ] **Step 5: Regenerate the embedded tree**

Run: `go generate ./internal/assets`
Then: `git status --short -- internal/assets/embedded`
Expected: exactly the three skill twins under `internal/assets/embedded/tree/skills/` (docket-groom-next, docket-convention, docket-auto-groom SKILL.md), plus `internal/assets/embedded/manifest.json` if it records digests.

- [ ] **Step 6: Run the focused tests and compile every tag set**

```bash
bash -c '
set -e
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 -run "ChangeGroom|SchemaVocab|VocabularyConst|FindingCode|ShapeValidatorCodes" ./internal/app/
go test -count=1 -tags integration -run "^TestIntegrationRecordOpsChangeGroomAbstainThenReEnableRealGit$" ./internal/app/
go test -count=1 -run "TestProseContracts|TestSkillSizeBudgets" ./internal/repoguard/
go test -count=1 ./internal/assets/ ./internal/harness/...
gofmt -l internal/app internal/cli
'
```

Expected: every command passes. `gofmt -l` prints nothing, and the harness goldens are unchanged.

- [ ] **Step 7: Mutation-test the hard-cut assert**

Make `validateChangeGroomShape` accept the old spelling, which is exactly the alias this change forbids:

```bash
bash -c '
f=internal/app/change_groom.go; cp "$f" "$f.bak"
perl -0pi -e "s/case GroomReEnable:/case GroomReEnable, \"rearm\":/" "$f"
command grep -c "case GroomReEnable, \"rearm\":" "$f"
go test -count=1 -run "^TestChangeGroomRetiredRearmOutcomeRefused$" ./internal/app/; echo "exit=$?"
mv -f "$f.bak" "$f"
go test -count=1 -run "^TestChangeGroomRetiredRearmOutcomeRefused$" ./internal/app/
'
```

Expected: the count prints `1` (the mutation landed). The mutated run FAILS with "outcome rearm: want invalid-outcome"; the restored run PASSES. Record both in `NOTES`.

- [ ] **Step 8: Derivation grep for this family**

```bash
bash -c '
git grep -n -I -i -E "re-?arm" -- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":!testdata/repositories" ":!internal/repository/testdata" ":!internal/install/legacydata" ":!internal/install/testdata" || echo "(none)"
'
```

Expected: exactly these, and nothing else: `internal/suiterunner/signal.go` (the kept signal re-arm), `internal/repoguard/budgets_test.go` (the frozen "0382: +typed rearm exit" history comment), and `internal/app/change_groom_test.go` (the hard-cut test). Fix any other hit.

- [ ] **Step 9: Commit**

```bash
git add -- internal/app/change_groom.go internal/app/finding_codes.go internal/app/schema_vocab.go internal/cli/change.go internal/app/change_groom_test.go internal/app/change_groom_integration_test.go internal/app/schema_vocab_test.go internal/repository/decode_test.go internal/repoguard/prose_contracts_test.go skills/docket-groom-next/SKILL.md skills/docket-convention/SKILL.md skills/docket-auto-groom/SKILL.md docs/reference/glossary.md docs/guide/designing-before-building.md internal/assets/embedded
git commit -m "refactor(0474): re-arm -> re-enable (outcome re-enable, nothing-to-re-enable); hard-cut rearm"
```

---

### Task 2: The shared-setting guard in `internal/config` (rows 56 and 59c, config part)

**Build tier:** standard — a package-wide identifier and comment rename in the config resolver. Behavior must stay byte-identical, and the frozen `fenced-machine-keys` fixture must stay untouched.

**Files:**
- Modify: `internal/config/config.go`, `internal/config/schema.go`, `internal/config/resolve.go`, `internal/config/decode.go`, `internal/config/capability.go`, `internal/app/install.go` (comment only)
- Test: `internal/config/resolve_test.go`, `internal/config/schema_test.go`, `internal/config/fixtures_test.go`, `internal/config/capability_test.go`, `internal/config/decode_test.go`, `internal/config/obsolete_metadata_branch_test.go`
- Modify (skills): `skills/docket-implement-next/references/edge-paths.md`
- Modify (docs): `docs/reference/glossary.md` (the *Coordination key / scope tag / shared-setting guard* entry)
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: nothing.
- Produces: `config.CodeSharedSettingIgnored = "shared-setting-ignored"` (was `CodeFencedIgnored`), `scopeRepoOnly` (was `scopeRepoFenced`), and `(*resolution).applySharedSettingGuard`, `keepUnguardedSurfaces` and `ignoreSharedSetting` (were `applyFence`, `keepUnfencedSurfaces` and `fenced`).

- [ ] **Step 1: Retarget the tests first (failing)**

Rename by table. Derive every site with `git grep -n -i "fenc" -- internal/config ':!internal/config/testdata'`:

| Old | New |
|---|---|
| `CodeFencedIgnored` | `CodeSharedSettingIgnored` |
| `scopeRepoFenced` | `scopeRepoOnly` |
| `fenceCase` / `fenceCases` | `guardCase` / `guardCases` |
| locals `fenced`, `wantFenced`, `repoFenced`, `wantRepoFenced` | `guarded`, `wantGuarded`, `repoOnly`, `wantRepoOnly` |
| `TestEveryFence` | `TestEverySharedSettingGuard` |
| `TestEveryFenceCoversTheRegistry` | `TestEverySharedSettingGuardCoversTheRegistry` |
| `TestRegistryFencedSet` | `TestRegistryRepoOnlySet` |
| `TestFencedPathStillReachesEffective` | `TestGuardedPathStillReachesEffective` |
| `TestRuntimeBashCommittedFence` | `TestRuntimeBashCommittedGuard` |
| `TestBoardGithubTokenMachineFence` | `TestBoardGithubTokenMachineGuard` |
| `TestTolerateUnknownKeysLeavesFenceIntact` | `TestTolerateUnknownKeysLeavesGuardIntact` |
| `TestFixtureFencedMachineKeys` | `TestFixtureGuardedMachineKeys` (it still loads `mustResolveFixture(t, "fenced-machine-keys")`: that frozen directory keeps its name) |
| `TestClassifyFencedDeclarationsAreNotCapabilities` | `TestClassifyGuardedDeclarationsAreNotCapabilities` |

Literal expectations: `"warning/fenced-setting-ignored/runtime.bash"` and `"warning/fenced-setting-ignored/terminal_publish"` (`resolve_test.go`) → `"warning/shared-setting-ignored/…"`. Every message string that says `fenced-setting-ignored` → `shared-setting-ignored`. Message and comment prose: "repo-fenced" → "repo-only", "fence case" → "guard case", "fenced" → "guarded", "fence" → "guard" (for example `"scopeRepoFenced set mismatch"` → `"scopeRepoOnly set mismatch"`, "an obsolete setting is not a fenced setting" → "… not a guarded setting", "registry row %q is repo-fenced but no fence case covers it" → "registry row %q is repo-only but no guard case covers it").

- [ ] **Step 2: Run to verify it fails**

Run: `go vet ./internal/config/`
Expected: FAIL with `undefined: CodeSharedSettingIgnored` (and `scopeRepoOnly`).

- [ ] **Step 3: Rename the implementation**

- `config.go`: `CodeSharedSettingIgnored = "shared-setting-ignored"` (gofmt realigns the block).
- `schema.go`: `scopeRepoOnly // machine-layer declaration → shared-setting-ignored, excluded`, and every `scope: scopeRepoFenced` → `scope: scopeRepoOnly`. The `scopeLocalOnly` comment: "committed-layer fence is enforced at DECODE" → "committed-layer guard is enforced at DECODE"; "never reaches applyFence" → "never reaches applySharedSettingGuard"; "the resolution-time fence back in applyFence" → "the resolution-time guard back in applySharedSettingGuard"; "when that fence is stripped" → "when that guard is stripped"; "(scopeLocalOnly, committed-fenced at decode)" → "(scopeLocalOnly, committed-guarded at decode)"; "before any fence runs" → "before any guard runs".
- `resolve.go`: `applyFence` → `applySharedSettingGuard` (definition, doc comment and call); `keepUnfencedSurfaces` → `keepUnguardedSurfaces`; `func (r *resolution) fenced(decl leafDecl, path, why, remedy string)` → `ignoreSharedSetting(…)` (both call sites); `Code: CodeSharedSettingIgnored`. Comments: "Fences first, then precedence: a fenced declaration is not a lower-precedence declaration" → "Guards first, then precedence: a guarded declaration is not a lower-precedence declaration"; "the ones a fence excluded" → "the ones a guard excluded"; "after fence exclusion" → "after guard exclusion"; "INCLUDING fenced-away ones" → "INCLUDING guarded-away ones".
- `decode.go`: "Fences, precedence, defaults" → "Shared-setting guards, precedence, defaults"; "rather than fenced (a fence would imply some layer could honor it)" → "rather than guarded (a guard would imply some layer could honor it)"; the "fenced." at line ~153 → "guarded.".
- `capability.go`: "Only the fenced `github` token" → "Only the guarded `github` token"; "the fence stripped it" → "the guard stripped it".
- `internal/app/install.go` (the `Warnings` field comment): "tolerated unknown keys, fenced\n\tsettings" → "tolerated unknown keys, guarded\n\tsettings".

When you are done, `git grep -n -i "fenc" -- internal/config ':!internal/config/testdata'` must print only lines naming the frozen fixture directory `fenced-machine-keys` (the `mustResolveFixture` argument and any comment that names that path).

- [ ] **Step 4: Skill and glossary**

`skills/docket-implement-next/references/edge-paths.md`: "(repo-fenced, off by default)" → "(repo-only, off by default)". It matches the documented `repo-only` scope tag and keeps the word count.

`docs/reference/glossary.md` (*Coordination key / scope tag / shared-setting guard*): after "The **shared-setting guard** ignores (with a warning) a coordination key set in\nany other layer." insert the sentence "The warning's code is `shared-setting-ignored`." Keep the rest of the entry unchanged.

- [ ] **Step 5: Regenerate, run the focused tests and compile every tag set**

```bash
bash -c '
set -e
go generate ./internal/assets
git status --short -- internal/assets/embedded
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/config/
go test -count=1 -run "TestSkillSizeBudgets|TestProseContracts" ./internal/repoguard/
go test -count=1 ./internal/assets/ ./internal/harness/...
gofmt -l internal/config internal/app
'
```

Expected: the embedded status shows only the `edge-paths.md` twin (plus `manifest.json` if it records digests); every test passes; `gofmt -l` prints nothing.

- [ ] **Step 6: Mutation-test the pinned code**

```bash
bash -c '
f=internal/config/config.go; cp "$f" "$f.bak"
perl -pi -e "s/\"shared-setting-ignored\"/\"shared-setting-ignoredX\"/" "$f"
command grep -c "shared-setting-ignoredX" "$f"
go test -count=1 -run "TestEverySharedSettingGuard|TestFixtureGuardedMachineKeys|TestTolerateUnknownKeysLeavesGuardIntact|TestResolve" ./internal/config/ >/dev/null; echo "exit=$?"
mv -f "$f.bak" "$f"
go test -count=1 ./internal/config/ >/dev/null; echo "restored exit=$?"
'
```

Expected: count `1`, mutated `exit=1` (at minimum the literal `warning/shared-setting-ignored/…` expectations redden), `restored exit=0`. Record in `NOTES`.

- [ ] **Step 7: Commit**

```bash
git add -- internal/config/config.go internal/config/schema.go internal/config/resolve.go internal/config/decode.go internal/config/capability.go internal/app/install.go internal/config/resolve_test.go internal/config/schema_test.go internal/config/fixtures_test.go internal/config/capability_test.go internal/config/decode_test.go internal/config/obsolete_metadata_branch_test.go skills/docket-implement-next/references/edge-paths.md docs/reference/glossary.md internal/assets/embedded
git commit -m "refactor(0474): config fence -> shared-setting guard (shared-setting-ignored, scopeRepoOnly)"
```

---

### Task 3: "Final" wire codes, Go identifiers, messages and commit subjects (rows 57–59, 59b code)

**Build tier:** standard — mechanical but cross-package (domain, render, repository, reposetup, app finalize and maintenance), touching finalize close-out codes that the finalize skill quotes. Every rename must keep the same status set.

**Files:**
- Modify: `internal/domain/types.go`, `internal/domain/actions.go`, `internal/domain/finalize.go`, `internal/render/board.go`, `internal/repository/evolution.go`, `internal/repository/validate.go`, `internal/reposetup/repair.go`, `internal/app/finalize_closeout.go`, `internal/app/finalize_cleanup.go`, `internal/app/finalize_block.go`, `internal/app/finalize_context.go`, `internal/app/finalize_retarget.go`, `internal/app/maintenance_assess.go`
- Test: `internal/domain/types_test.go`, `internal/repository/evolution_test.go`, `internal/repository/validate_test.go`, `internal/reposetup/repair_test.go`, `internal/app/finalize_retarget_test.go`, `internal/app/finalize_cleanup_test.go`, `internal/app/finalize_cleanup_integration_test.go` (`//go:build integration`)
- Modify (skills): `skills/docket-finalize-change/SKILL.md` (the two quoted codes)
- Modify (docs): `docs/reference/glossary.md` (*Retarget children*)
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: nothing.
- Produces: `domain.Status.Final() bool` (was `Terminal()`, same set: done and killed), `render.boardFinalStatuses`, `repository.isFinalADRStatus`, `repository.CodeADRUpdateAfterFinal = "adr-update-after-final"`, `repository.CodeChangeFinalClaimStamp = "change-final-claim-stamp"`, `reposetup.finalStatus`, `reposetup.RepairDropClaimedAt = "drop-final-claimed-at"`, `app.ReasonCleanupNotFinal = "not-final"`, `app.ReasonCleanupBacklinkPending` / `app.ReasonCloseoutBacklinkPending = "final-backlink-pending"`, `app.ReasonCloseoutNotesFrozen = "final-notes-frozen"`, `app.childOutcomeSkippedDone = "skipped-final"`, `app.closeoutNotesMatchArchived`, and `resolveBlockTarget(…, allowFinal bool)`.

- [ ] **Step 1: Write the failing tests**

`internal/domain/types_test.go`: `TestStatusTerminal` → `TestStatusFinal`. Keep the case table unchanged: the same eight statuses and the same wants, so the status set is pinned. Call `tc.in.Final()`, message `"%q.Final() = %v; want %v"`.

Spelling pins. Add each to the named existing test file:

`internal/app/finalize_cleanup_test.go`, directly after the `// --- TestFinalizeCleanupOnlyAfterTerminal ---…` separator (rename that separator to `// --- TestFinalizeCleanupOnlyAfterFinal ---` and keep its dash padding to the same width):

```go
// TestFinalLifecycleCodeSpellings pins the ADR-0129 rows 57 and 59 wire
// spellings (change 0474). Drivers and skills key on these strings, not on the
// constant names.
func TestFinalLifecycleCodeSpellings(t *testing.T) {
	for _, c := range []struct{ name, got, want string }{
		{"ReasonCloseoutBacklinkPending", string(ReasonCloseoutBacklinkPending), "final-backlink-pending"},
		{"ReasonCleanupBacklinkPending", string(ReasonCleanupBacklinkPending), "final-backlink-pending"},
		{"ReasonCloseoutNotesFrozen", string(ReasonCloseoutNotesFrozen), "final-notes-frozen"},
		{"ReasonCleanupNotFinal", string(ReasonCleanupNotFinal), "not-final"},
		{"childOutcomeSkippedDone", string(childOutcomeSkippedDone), "skipped-final"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}
```

`internal/repository/validate_test.go`, at the end of the file:

```go
// TestFinalLifecycleFindingSpellings pins ADR-0129 rows 58 and 59 (change 0474).
func TestFinalLifecycleFindingSpellings(t *testing.T) {
	if got := string(CodeChangeFinalClaimStamp); got != "change-final-claim-stamp" {
		t.Errorf("CodeChangeFinalClaimStamp = %q", got)
	}
	if got := string(CodeADRUpdateAfterFinal); got != "adr-update-after-final" {
		t.Errorf("CodeADRUpdateAfterFinal = %q", got)
	}
}
```

`internal/reposetup/repair_test.go`, at the end of the file:

```go
// TestRepairDropClaimedAtSpelling pins ADR-0129 row 58 (change 0474).
func TestRepairDropClaimedAtSpelling(t *testing.T) {
	if got := string(RepairDropClaimedAt); got != "drop-final-claimed-at" {
		t.Errorf("RepairDropClaimedAt = %q, want drop-final-claimed-at", got)
	}
}
```

Renames in existing tests:
- `internal/app/finalize_retarget_test.go`: `TestRetargetChildrenSkipsTerminalChildren` → `TestRetargetChildrenSkipsFinalChildren` (and its comment's name); both `want skipped-terminal` messages → `want skipped-final`.
- `internal/app/finalize_cleanup_integration_test.go`: `TestIntegrationFinalizeCleanupOnlyAfterTerminal` → `TestIntegrationFinalizeCleanupOnlyAfterFinal`; subtest `"non-terminal-refused"` → `"non-final-refused"`.
- `internal/repository/evolution_test.go`: `TestValidateEvolutionRejectsUpdateAfterTerminalStatus` → `TestValidateEvolutionRejectsUpdateAfterFinalStatus`; local `terminal := minimalADR(…)` → `final := minimalADR(…)`; `CodeADRUpdateAfterTerminal` → `CodeADRUpdateAfterFinal`; case names `"terminal reopened"` / `"terminal re-aimed"` → `"final reopened"` / `"final re-aimed"`.
- `internal/repository/validate_test.go`: `CodeChangeTerminalClaimStamp` → `CodeChangeFinalClaimStamp`; case names `"terminal status left in active"` → `"final status left in active"`, `"archived terminal record still holding a claim stamp"` → `"archived final record still holding a claim stamp"`.
- `internal/reposetup/repair_test.go`: case `"non-terminal archived record"` → `"non-final archived record"`.

- [ ] **Step 2: Run to verify it fails**

Run: `go vet ./internal/domain/ ./internal/repository/ ./internal/reposetup/ ./internal/app/`
Expected: FAIL with undefined `Final`, `CodeChangeFinalClaimStamp`, `CodeADRUpdateAfterFinal` and `ReasonCleanupNotFinal`.

- [ ] **Step 3: Rename identifiers and codes**

- `internal/domain/types.go`: `// Final reports whether the status is a final state — done or killed.` / `func (s Status) Final() bool`. Replace every caller's `.Terminal()` on a `domain.Status` with `.Final()`: `internal/domain/actions.go`, `internal/domain/finalize.go`, `internal/app/finalize_block.go` (×2), `internal/app/finalize_closeout.go`, `internal/app/finalize_context.go`, `internal/reposetup/repair.go`, `internal/repository/validate.go`. Derive them with `git grep -n "\.Terminal()" -- '*.go'`. A run-record `.Terminal` **field** (no parentheses) is the kept process sense; leave it.
- `internal/render/board.go`: `boardTerminalStatuses` → `boardFinalStatuses` (definition and three uses); its comment "is the terminal group in display order" → "is the final group in display order".
- `internal/repository/evolution.go`: `CodeADRUpdateAfterFinal = "adr-update-after-final"` with the comment "CodeADRUpdateAfterFinal marks an update section appended to an ADR …"; `isTerminalADRStatus` → `isFinalADRStatus` (definition, doc comment, call).
- `internal/repository/validate.go`: `CodeChangeFinalClaimStamp = "change-final-claim-stamp"` (and its comment's name); local `terminal := c.Status().Final()` → `final := c.Status().Final()` and its three uses.
- `internal/reposetup/repair.go`: `RepairDropClaimedAt RepairCode = "drop-final-claimed-at"`; the roster comment line `drop-terminal-claimed-at   claimed_at removed from an already-terminal` → `drop-final-claimed-at      claimed_at removed from an already-final` (keep the column alignment); "planClaimedAt decides the drop-final-claimed-at roster entry."; `terminalStatus` → `finalStatus` (definition, doc comment "finalStatus reports whether the record's decoded status is a final end …", call); "only for a terminal-status record" → "only for a final-status record".
- `internal/app/finalize_cleanup.go`: `ReasonCleanupNotTerminal = "not-terminal"` → `ReasonCleanupNotFinal = "not-final"` (definition and all uses); `ReasonCleanupBacklinkPending = "final-backlink-pending"`.
- `internal/app/finalize_closeout.go`: `ReasonCloseoutBacklinkPending = "final-backlink-pending"`, `ReasonCloseoutNotesFrozen = "final-notes-frozen"`; `closeoutNotesMatchTerminal` → `closeoutNotesMatchArchived` (definition, doc comment "closeoutNotesMatchArchived reports whether the archived record already …", and both calls).
- `internal/app/finalize_retarget.go`: `childOutcomeSkippedDone = "skipped-final" // stacked-merged/done/killed: does not block, not edited; also emitted for stacked-merged children (the child no longer needs its PR retargeted)`. Keep the constant name (spec §B).
- `internal/app/finalize_block.go`: parameter `allowTerminal` → `allowFinal` in `resolveBlockTarget` and its doc comment ("allowFinal is false for `finalize block`").

- [ ] **Step 4: Messages and commit subjects (spec §C)**

| File | Old | New |
|---|---|---|
| `finalize_block.go` (×2) | `"change %04d is terminal; there is no finalize attempt to block"` | `"change %04d is final; there is no finalize attempt to block"` |
| `finalize_context.go` | `"… is not in finalize's population (terminal or without a PR reference)"` | `"… (final or without a PR reference)"` |
| `finalize_cleanup.go` | `"change is not terminal and carries no aborted-rebase scratch to clear; nothing to clean"` | `"change is not final and carries no aborted-rebase scratch to clear; nothing to clean"` |
| `finalize_cleanup.go` | `"the terminal backlink interior could not be rendered; retry cleanup"` | `"the final backlink interior could not be rendered; retry cleanup"` |
| `finalize_cleanup.go` | subject `"change " + itoa(o.rootID) + " terminal backlinks verified (cleanup)"` | `" final backlinks verified (cleanup)"` |
| `finalize_closeout.go` | `CommitSubject: fmt.Sprintf("change %04d terminal backlinks retargeted to archive", o.rootID)` | `"change %04d final backlinks retargeted to archive"` |
| `maintenance_assess.go` | `"the record is not a terminal done/stacked-merged record"` | `"the record is neither done nor stacked-merged"` |
| `maintenance_assess.go` | `"the record's terminal backlink could not be rendered"` | `"the record's final backlink could not be rendered"` |
| `maintenance_assess.go` | `"artifact "+p+" carries a malformed terminal backlink block"` | `"… carries a malformed final backlink block"` |

Re-check that no code parses the old subjects or messages: `git grep -n -E "backlinks (retargeted|verified)|is terminal; there|terminal backlink" -- '*.go' '*.sh'`. Expected: only comment lines, which Task 4 sweeps.

Also change the comments that sit on the lines you edit to use the renamed codes: `finalize_backlink_loader.go`, `finalize_closeout.go` and `finalize_cleanup.go` comments that spell `terminal-backlink-pending` → `final-backlink-pending`. Test comments that spell the old codes (`finalize_closeout_integration_test.go`, `finalize_git_test.go`) → the new codes. Derive them with `git grep -n -E "terminal-backlink-pending|terminal-notes-frozen|not-terminal|skipped-terminal|adr-update-after-terminal|change-terminal-claim-stamp|drop-terminal-claimed-at" -- '*.go'`, which must print nothing when you are done.

- [ ] **Step 5: Skill and glossary consumers**

`skills/docket-finalize-change/SKILL.md`: "are refused (`terminal-notes-frozen`)" → "are refused (`final-notes-frozen`)"; "with a `terminal-backlink-pending` finding" → "with a `final-backlink-pending` finding".

`docs/reference/glossary.md` (*Retarget children*): after "When a stack parent lands, finalize repoints its stacked children's PRs at the new base." add: "Each child reports an outcome. `skipped-final` is also emitted for stacked-merged children (the child no longer needs its PR retargeted)."

Run `go generate ./internal/assets`.

- [ ] **Step 6: Run the focused tests and compile every tag set**

```bash
bash -c '
set -e
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/domain/ ./internal/render/ ./internal/repository/ ./internal/reposetup/
go test -count=1 -run "Finalize|Retarget|Closeout|Cleanup|Maintenance|Assess|FinalLifecycle" ./internal/app/
go test -count=1 -tags integration -run "^TestIntegrationFinalizeCleanup" ./internal/app/
go test -count=1 -run "TestSkillSizeBudgets|TestProseContracts" ./internal/repoguard/
go test -count=1 ./internal/assets/ ./internal/harness/...
gofmt -l internal/domain internal/render internal/repository internal/reposetup internal/app
'
```

Expected: every command passes; `gofmt -l` prints nothing. `TestStatusFinal`'s unchanged table proves that the status set is unchanged.

- [ ] **Step 7: Mutation-test one pin**

```bash
bash -c '
f=internal/app/finalize_retarget.go; cp "$f" "$f.bak"
perl -pi -e "s/\"skipped-final\"/\"skipped-finalX\"/" "$f"
command grep -c "skipped-finalX" "$f"
go test -count=1 -run "^(TestFinalLifecycleCodeSpellings|TestRetargetChildrenSkipsFinalChildren)$" ./internal/app/ >/dev/null; echo "exit=$?"
mv -f "$f.bak" "$f"
'
```

Expected: count `1`, `exit=1`. Record in `NOTES`.

- [ ] **Step 8: Commit**

```bash
git add -- internal/domain/types.go internal/domain/actions.go internal/domain/finalize.go internal/domain/types_test.go internal/render/board.go internal/repository/evolution.go internal/repository/validate.go internal/repository/evolution_test.go internal/repository/validate_test.go internal/reposetup/repair.go internal/reposetup/repair_test.go internal/app/finalize_closeout.go internal/app/finalize_cleanup.go internal/app/finalize_block.go internal/app/finalize_context.go internal/app/finalize_retarget.go internal/app/maintenance_assess.go internal/app/finalize_backlink_loader.go internal/app/finalize_retarget_test.go internal/app/finalize_cleanup_test.go internal/app/finalize_cleanup_integration_test.go internal/app/finalize_closeout_integration_test.go internal/app/finalize_git_test.go skills/docket-finalize-change/SKILL.md docs/reference/glossary.md internal/assets/embedded
git commit -m "refactor(0474): lifecycle terminal -> final (codes, Status.Final, messages, commit subjects)"
```

(Drop from `git add` any listed test file you did not need to touch. Add any extra file the derivation grep made you touch.)

---

### Task 4: Lifecycle-"terminal" prose and comments, including "closing half" (row 59b text)

**Build tier:** standard — judgment work. Every remaining "terminal" hit is classified as lifecycle (rename) or kept, using the rubric below. Wording only, with no identifier or behavior change.

TDD exception: **why unsuitable:** comments, CLI help strings and prose carry no behavior, and a guard on passing mentions is out of scope (spec *Seal*; 0468/0473 precedent). **What replaced it:** the derivation grep below, the closing grep in Task 7, and the full suite at the gate. **Residual risk:** a misclassified sense, which review reads.

**Files (from the grooming trace; the derivation grep is authoritative):**
- CLI help: `internal/cli/finalize.go`, `internal/cli/maintenance.go`, `internal/cli/change.go` (comment)
- Go comments (lifecycle sense): `internal/domain/actions.go`, `internal/domain/stack_test.go` (the "done is terminal" message only), `internal/render/board.go`, `internal/render/board_test.go`, `internal/repository/evolution.go`, `internal/repository/validate.go`, `internal/reposetup/repair.go`, `internal/app/change_kill.go`, `internal/app/finalize_block.go`, `internal/app/finalize_cleanup.go`, `internal/app/finalize_closeout.go`, `internal/app/finalize_context.go`, `internal/app/finalize_merge.go`, `internal/app/finalize_retarget.go`, `internal/app/maintenance.go`, `internal/app/maintenance_assess.go`, `internal/cli/finalize_test.go`, and the test comments in `internal/app/finalize_cleanup_test.go`, `finalize_closeout_test.go`, `finalize_closeout_integration_test.go`, `finalize_git_test.go`, `finalize_merge_test.go`, `finalize_e2e_test.go`, `finalize_state_integration_test.go`, `maintenance_assess_test.go`, `maintenance_test.go`, `maintenance_traffic_integration_test.go`, `repomigration_integration_test.go`, `status_corpus_test.go`
- Skills: `skills/docket-finalize-change/SKILL.md` (line 10, "through its terminal half")
- Shell: `tests/test_go_finalize_e2e.sh` (header comment)
- Docs: `docs/reference/cli.md`, `docs/reference/config-keys.md`
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: Task 3's renamed identifiers (the comments you edit sit beside them).
- Produces: the CLI `Short` strings "Sequence a change's closing half: rebase, publish, merge, and closeout", "Reclaim docket's closing half in batch (docket status stays read-only)", and "Close out merged changes, retry close-out cleanup, and reclaim expired claims in one pass".

**Classification rubric** (apply to every hit):
1. It names a change or ADR end state, or the record, status, transition, shape, write or backlinks at that end state ("a terminal (done or killed) descendant", "terminal record", "terminal archive", "terminal shapes", "never a terminal write", "the promised terminal state", "done or killed child is terminal", "all terminal (status: done)") → **final**. Use "archived record" where "terminal record" means the archived record (row 64).
2. It names the stretch that leads to the end state ("terminal half", "terminal-half operations", "terminal path", "terminal sequence", "terminal metadata transaction" meaning the closeout transaction) → **closing** ("closing half", "closing-half operations", "closing path", "closing sequence", "closing metadata transaction").
3. It is a kept sense (Global Constraints, *Explicitly kept*) → leave it.
4. A lifecycle phrase about stacked-merged, which is non-final: "stacked-merged (a terminal-but-retained state)" → "stacked-merged (a merged-but-retained state)". Never call stacked-merged "final".

- [ ] **Step 1: CLI help and its docs mirror**

`internal/cli/finalize.go`: `Short: "Sequence a change's closing half: rebase, publish, merge, and closeout"`. Comments: "The terminal-half mutation subcommands" → "The closing-half mutation subcommands"; "repairs the terminal backlinks" → "repairs the final backlinks"; "one verified terminal shape (done-archived, stacked-merged, or root carry)" → "one verified closeout shape (done-archived, stacked-merged, or root carry)". Stacked-merged is non-final, so use "closeout shape", not "final shape".
`internal/cli/maintenance.go`: `Short: "Reclaim docket's closing half in batch (docket status stays read-only)"` and `Short: "Close out merged changes, retry close-out cleanup, and reclaim expired claims in one pass"`. Comments: "terminal half is reached only through" → "closing half is reached only through"; "retries terminal backlink repair" → "retries final backlink repair".
`internal/cli/change.go`: "the other terminal-half operations use" → "the other closing-half operations use".
`internal/cli/finalize_test.go`: "terminal-half mutation subcommands" → "closing-half mutation subcommands".
`docs/reference/cli.md`: "sequence a change's terminal half:" → "sequence a change's closing half:"; "reclaim docket's terminal half in batch" → "reclaim docket's closing half in batch".
`docs/reference/config-keys.md`: "the terminal-half sequencer's knobs" → "the closing-half sequencer's knobs".

- [ ] **Step 2: Skill and shell header**

`skills/docket-finalize-change/SKILL.md`: "drives a verified `implemented` change through its terminal half:" → "… through its closing half:". Leave `## Terminal disposition (driver contract)`, "a terminal disposition" and "`passed` terminal observation": they are the kept run sense.
`tests/test_go_finalize_e2e.sh`: "# whole terminal half of the workflow" → "# whole closing half of the workflow".
Then run `go generate ./internal/assets`.

- [ ] **Step 3: Go comment sweep**

Derive the candidates:

```bash
bash -c '
EX=(":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":!testdata/repositories" ":!internal/repository/testdata" ":!internal/install/legacydata" ":!internal/install/testdata" ":!docs/reference/harness/fixtures" ":!internal/assets/embedded")
PROC=(":!internal/gatedrive" ":!internal/process" ":!internal/codexentry" ":!internal/suiterunner" ":!internal/app/runtracker*" ":!internal/app/gate*" ":!internal/app/root_entry*" ":!internal/app/agent_*" ":!internal/githubcli" ":!internal/cli/agent*" ":!internal/cli/gate*")
out=$(git grep -n -I -i "terminal" -- "*.go" "*.sh" "${EX[@]}" "${PROC[@]}")
command grep -v -i -E "terminal_publish|terminal[- ]publi(sh|cation)|terminalPublish|GIT_TERMINAL_PROMPT|terminalAllowlist|nonterminal|terminal\.json|terminalFile|TerminalWaiting|mapTerminalDrive|runToTerminal" <<<"$out"
'
```

Apply the rubric to each hit. Known lifecycle sites from the trace (starting points, not a closed list):
- `internal/domain/actions.go`: "deferred change is not terminal" → "not final"; "terminal (done or killed) descendant" → "final (done or killed) descendant".
- `internal/domain/stack_test.go`: the message "(ADR-0092: done is terminal)" → "(ADR-0092: done is final)". Keep rule 3's "terminal/terminally" (decision-procedure sense, kept).
- `internal/render/board.go`: "then the terminal done/killed groups", "terminal labels are the stored …", "A non-active (terminal) status", "boardEmoji is the terminal (archive) status's …" → "final". `internal/render/board_test.go`: "terminal (archive) status", "Archive terminal counts", "terminal <details> archive block" → "final".
- `internal/repository/evolution.go`: "only to a parseable terminal" → "only to a parseable final". `validate.go`: "an archived record that still …" (already says archived; fix any leftover "terminal").
- `internal/reposetup/repair.go`: any leftover "terminal".
- `internal/app/change_kill.go`: "a non-allocating terminal" → "a non-allocating final".
- `internal/app/finalize_block.go`: "ReasonBlockNotBlockable: the change is terminal" → "is final"; "or a terminal (non-blockable) change" → "or a final (non-blockable) change".
- `internal/app/finalize_cleanup.go`: "destructive suffixes of the terminal half" → "closing half"; "terminal backlinks first when needed" → "final backlinks …"; "terminal record, no live lock/group" and "a durable terminal record" → "archived record"; "Leg 1: repair the terminal backlinks" → "final backlinks".
- `internal/app/finalize_closeout.go`: "the atomic terminal metadata transaction" → "the atomic closing metadata transaction"; "Three verified terminal shapes:" → "Three verified closeout shapes:" (one is stacked-merged, which is non-final); "the promised terminal state" → "final state"; "never permits a terminal write" and "never a terminal write" → "final write"; "on a terminal archive" → "on a final archive"; "the one verified terminal shape" → "closeout shape"; "into the terminal bytes" → "into the archived bytes"; "the terminal in-place record's own bytes" → "the archived in-place record's own bytes"; "would the terminal backlink block's bytes change" (×2) and "the terminal backlink legs" → "final backlink". **Keep** "not terminal publishing" (the obsolete feature name).
- `internal/app/finalize_context.go`: "every terminal-half operation", "the terminal-half operations" (×3) → "closing-half"; "terminal, or carrying no PR" → "final, or carrying no PR". Keep "observes it to a terminal within the observation budget" (a run).
- `internal/app/finalize_merge.go`: "in the terminal path" → "in the closing path".
- `internal/app/finalize_retarget.go`: "a done or killed child is terminal" → "is final".
- `internal/app/maintenance.go`: "docket's terminal half" → "closing half"; "one terminal (or completed-stack) record" → "one final (or completed-stack) record".
- `internal/app/maintenance_assess.go`: "already-correct terminal backlinks", "the terminal-backlink leg", "the terminal backlink leg" → "final".
- Test comments: "terminal path" → "closing path" (`finalize_git_test.go`, `finalize_merge_test.go`, `finalize_e2e_test.go` including "terminal-path gate" and "conflict/repair terminal path"); "the WHOLE terminal half" → "closing half" (`finalize_e2e_test.go`); "reruns the terminal sequence" → "closing sequence"; "Closeout is the atomic terminal metadata" → "closing metadata" (`finalize_closeout_test.go`); "the terminal metadata transaction writes only" → "closing metadata transaction" (`finalize_state_integration_test.go`); "the terminal transaction lands once" → "the closeout transaction lands once" (`finalize_git_test.go`); "ProbeMerged (the terminal reprobe)" → "ProbeMerged (the closing reprobe)" (`finalize_cleanup_test.go`); "stacked-merged (a terminal-but-retained state)" → "(a merged-but-retained state)"; "terminal-backlink interior", "already-correct terminal …", "a stale terminal backlink", "a malformed/unbalanced terminal …" → "final" (`maintenance_assess_test.go`); "docket's terminal half" and "terminal backlink repair" → "closing half" / "final backlink repair" (`maintenance_test.go`); "valid terminal (done) record", "terminal done records", "terminal (done)", "the terminal (done) population" → "final" (`maintenance_traffic_integration_test.go`); "ARCHIVED (terminal) change record" → "ARCHIVED (final)" (`repomigration_integration_test.go`); "all terminal (status: done)" → "all final" (`status_corpus_test.go`). Keep `maintenance_traffic_integration_test.go`'s "terminal success" if it describes the sweep run's result (a run sense). Read it and decide.

When you are done, re-run the derivation. Every remaining hit must be a kept sense. List them in `NOTES` grouped by file, with the rubric class next to each.

- [ ] **Step 4: Focused tests**

```bash
bash -c '
set -e
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 ./internal/cli/ ./internal/domain/ ./internal/render/
go test -count=1 -run "TestSkillSizeBudgets|TestProseContracts" ./internal/repoguard/
go test -count=1 ./internal/assets/ ./internal/harness/...
gofmt -l internal
'
```

Expected: every command passes; `gofmt -l` prints nothing.

- [ ] **Step 5: Commit**

Stage exactly the files the sweep changed (list them with `git status --short`), plus `internal/assets/embedded`:

```bash
git add -- internal/cli/finalize.go internal/cli/maintenance.go internal/cli/change.go internal/cli/finalize_test.go skills/docket-finalize-change/SKILL.md tests/test_go_finalize_e2e.sh docs/reference/cli.md docs/reference/config-keys.md internal/assets/embedded <each Go file the Step 3 sweep changed>
git commit -m "refactor(0474): lifecycle terminal prose -> final; terminal half -> closing half"
```

---

### Task 5: Rename `terminal-close-out.md` to `close-out.md` (row 59a)

**Build tier:** economy — one `git mv`, one budget-row move and four path references. A miss fails loudly: `TestSkillSizeBudgets` checks both directions, and Task 7's seal catches any leftover reference.

**Files:**
- Rename: `skills/docket-convention/references/terminal-close-out.md` → `skills/docket-convention/references/close-out.md`
- Modify: `internal/repoguard/budgets_test.go` (the row only)
- Modify (skills): `skills/docket-convention/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-status/SKILL.md`
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: nothing.
- Produces: the reference path `skills/docket-convention/references/close-out.md`, which Task 7's seal row names as the replacement.

- [ ] **Step 1: Move the budget row (failing test)**

In `internal/repoguard/budgets_test.go`, delete `{"docket-convention/references/terminal-close-out.md", 240, 2150},` and insert, between the `agent-layer.md` and `dummy-mode.md` rows (sorted position):

```go
	{"docket-convention/references/close-out.md", 240, 2150}, // 0474: reference renamed (ADR-0129 row 59a); ceilings unchanged
```

Do not name the old filename in the comment (Task 7's closing grep expects no residue here). Keep every other row and history comment byte-identical.

- [ ] **Step 2: Run to verify it fails**

Run: `go test -count=1 -run '^TestSkillSizeBudgets$' ./internal/repoguard/`
Expected: FAIL, with "budgeted skill file missing/unreadable skills/docket-convention/references/close-out.md" **and** "skills/**/*.md files with no budget row … skills/docket-convention/references/terminal-close-out.md".

- [ ] **Step 3: Rename the file and its references**

```bash
git mv skills/docket-convention/references/terminal-close-out.md skills/docket-convention/references/close-out.md
```

The file's own title (`# Close-out — the shared per-change sequence`) and its "terminal publication" / `terminal_publish` lines stay (kept senses).

| File | Old | New |
|---|---|---|
| `skills/docket-convention/SKILL.md` | "live in **[`references/terminal-close-out.md`](references/terminal-close-out.md) — read it" | "live in **[`references/close-out.md`](references/close-out.md) — read it" |
| `skills/docket-implement-next/SKILL.md` | "(**read `../docket-convention/references/terminal-close-out.md` now — blocking**)" | "(**read `../docket-convention/references/close-out.md` now — blocking**)" |
| `skills/docket-new-change/SKILL.md` | "Follow `references/terminal-close-out.md`'s **kill path**" | "Follow `references/close-out.md`'s **kill path**" |
| `skills/docket-status/SKILL.md` | "close-out sequence (`terminal-close-out.md`)" | "close-out sequence (`close-out.md`)" |

- [ ] **Step 4: Regenerate and verify**

```bash
bash -c '
set -e
go generate ./internal/assets
test -f internal/assets/embedded/tree/skills/docket-convention/references/close-out.md && echo "embedded new: ok"
test ! -e internal/assets/embedded/tree/skills/docket-convention/references/terminal-close-out.md && echo "embedded old: gone"
m=$(cat internal/assets/embedded/manifest.json)
command grep -c "references/close-out.md" <<<"$m"
command grep -c "terminal-close-out" <<<"$m" || true
git grep -n "terminal-close-out" -- skills internal/assets/embedded internal/repoguard/budgets_test.go || echo "(no residue)"
go test -count=1 -run "^TestSkillSizeBudgets$" ./internal/repoguard/
go test -count=1 ./internal/assets/ ./internal/harness/...
'
```

Expected: `embedded new: ok`, `embedded old: gone`, a manifest count ≥ 1 for the new path and `0` for the old one, `(no residue)`, and PASS for all tests. Skills install as directory links into the asset tree, so an installed skill directory carries exactly the files in `internal/assets/embedded/tree/skills/docket-convention/`. The post-merge install check is in the results notes at the end of this plan.

- [ ] **Step 5: Commit**

```bash
git add -- internal/repoguard/budgets_test.go skills/docket-convention/references/terminal-close-out.md skills/docket-convention/references/close-out.md skills/docket-convention/SKILL.md skills/docket-implement-next/SKILL.md skills/docket-new-change/SKILL.md skills/docket-status/SKILL.md internal/assets/embedded
git commit -m "refactor(0474): references/terminal-close-out.md -> references/close-out.md"
```

---

### Task 6: The other non-run fences (row 59c)

**Build tier:** standard — identifier renames that fail loudly on a miss, plus comment rewording that needs sense judgment (run fence and code fence stay).

**Files:**
- Modify: `internal/app/planning.go` (definition of `fenceBoardSurface`), every caller (`internal/app/change_attach.go`, `change_claim.go`, `change_create.go`, `change_groom.go`, `change_halt.go`, `change_implemented.go`, `change_kill.go`, `change_lifecycle.go`, `change_reclaim.go`, `change_reconcile.go`, `change_repair.go`, `finalize_block.go`, `finalize_closeout.go`), `internal/app/learning_ops.go`, `internal/cli/change.go`, `internal/cli/learning.go`, `internal/gitcli/rebase.go` (comments)
- Test: `internal/app/planning_test.go`, `change_create_test.go`, `change_groom_test.go`, `change_kill_test.go`, `change_lifecycle_test.go`, `change_reconcile_test.go`, `learning_ops_test.go`, `config_test.go` (comment), `finalize_e2e_test.go` (`//go:build e2e`), `finalize_state_integration_test.go` (comment), `internal/gitcli/rebase_integration_test.go` (`//go:build integration`), `internal/repoguard/mirror_writeback_test.go` (comment)

**Interfaces:**
- Consumes: nothing.
- Produces: `app.resolveBoardSurface(eff config.Effective) (inline bool, err error)` (was `fenceBoardSurface`), `app.haltPreflight(ctx context.Context, op string, deps PlanningDeps, repoDir string, id int) (StatusPin, config.Effective, bool, *HaltResult)` (was `haltPinAndFence`), and `app.requireLearningsEnabled(eff config.Effective) error` (was `fenceLearningsEnabled`).

- [ ] **Step 1: Retarget the tests first (failing)**

| Old | New |
|---|---|
| `TestFenceBoardSurface` (`planning_test.go`, and its calls to `fenceBoardSurface`) | `TestResolveBoardSurface` (calling `resolveBoardSurface`) |
| `TestChange{Create,Groom,Kill,Block,Defer,Unblock,Revive}FencesGithubBoardSurface` | `TestChange{…}RefusesGithubBoardSurface` |
| `TestLearningRecordFencesWhenLearningsDisabled`, `TestLearningUpdateFencesWhenLearningsDisabled` | `…RefusesWhenLearningsDisabled` |
| `TestChangeReconcileOwnedFieldFence` (and its `// --- … ---` separator and doc comment) | `TestChangeReconcileRefusesNonOwnedSection` |
| `TestE2EUnsupportedConfigFence` (and its separator and doc comment) | `TestE2EUnsupportedConfigRefused` |
| `TestIntegrationRepoOwnedRefFence` (its doc comment says "TestOwnedRefFence proves …") | `TestIntegrationRepoOwnedRefRefusesForeignRef` (doc comment "TestIntegrationRepoOwnedRefRefusesForeignRef proves …") |

Test messages: `"engine called despite a fenced board surface"` (in `change_create_test.go`, `change_groom_test.go`, `change_kill_test.go` and `change_lifecycle_test.go` ×4) → `"engine called despite a refused github board surface"`. In `finalize_e2e_test.go`'s `TestE2EUnsupportedConfigRefused`: "the capability requests that fenced Docket" → "that refused Docket"; "the fence is a config-layer property" → "the refusal is a config-layer property"; "reads never fence" → "reads are never refused"; local `fenced := []struct` → `refused := []struct` (and its loop); messages "a fenced operation moved …" / "changed the PR state" → "a refused operation …"; "before the fenced attempts" → "before the refused attempts"; leave the JSON literal `{"report":"fenced"}` as it is (test data only; its value is never read for meaning). In the header comment: "no deferred-capability fence" → "no deferred-capability refusal".

- [ ] **Step 2: Run to verify it fails**

Run: `go vet ./internal/app/`
Expected: FAIL with `undefined: resolveBoardSurface`.

- [ ] **Step 3: Rename the implementation**

- `planning.go`: `func resolveBoardSurface(eff config.Effective) (inline bool, err error)`. Doc comment: "resolveBoardSurface reads the resolved board_surfaces … so a `[inline github]` configuration is refused rather than silently enabling inline." Other comments in the file: "the board-surface preflight fence" → "the board-surface preflight check"; the "(… fence)" at the taxonomy comment → "(… check)"; "boardSurfaceGitHub is the fenced surface token" → "the refused surface token"; "a github-fenced" → "a github-refused".
- Every caller: `fenceBoardSurface(eff)` → `resolveBoardSurface(eff)` (derive with `git grep -n fenceBoardSurface`). Caller comments: "fence the board surface" → "check the board surface"; "Board-surface fence: a github surface is an unsupported configuration" → "Board-surface check: a github surface is an unsupported configuration"; "the board fence consult(s)" → "the board-surface check consult(s)"; "fenced board surface" → "refused board surface" (`change_reconcile.go`); "Owned-section fence:" → "Owned-section check:" (`change_reconcile.go`); "folds a board-surface fence planning error" → "folds a board-surface check planning error" (`finalize_block.go`).
- `change_halt.go`: `haltPinAndFence` → `haltPreflight` (definition and both calls). Doc comment: "haltPreflight pins context, runs the deferred-capability preflight, and resolves the inline board-surface check — the shared pre-transaction plumbing …".
- `learning_ops.go`: `fenceLearningsEnabled` → `requireLearningsEnabled` (definition, doc comment "requireLearningsEnabled refuses at preflight when learnings.enabled is not true: …", both calls); "so no board fence runs here" → "so no board-surface check runs here".
- `internal/cli/change.go`: "board fence — belongs to internal/app" → "board-surface check — belongs to internal/app". `internal/cli/learning.go`: "the learnings.enabled fence" → "the learnings.enabled check".
- `internal/gitcli/rebase.go`: "ownedRefRequiredPrefix fences the owned-ref primitives" → "ownedRefRequiredPrefix guards the owned-ref primitives"; "the fenced update-ref" → "the prefix-guarded update-ref".
- `internal/app/config_test.go`: "pins the Go v1 capability fence's verdict" → "pins the Go v1 capability check's verdict".
- `internal/app/finalize_state_integration_test.go`: "every effect is fenced by the exact old-value it read" → "every effect is guarded by the exact old value it read".
- `internal/repoguard/mirror_writeback_test.go`: "fenceBoardSurface (in internal/app/planning.go)" → "resolveBoardSurface (in internal/app/planning.go)".

- [ ] **Step 4: Derivation grep**

```bash
bash -c '
EX=(":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":!testdata/repositories" ":!internal/repository/testdata" ":!internal/install/legacydata" ":!internal/install/testdata" ":!docs/reference/harness/fixtures" ":!internal/assets/embedded")
out=$(git grep -n -I -i -E "board[- ]?(surface )?fence|fenceBoardSurface|haltPinAndFence|fenceLearningsEnabled|learnings(\.enabled)? fence|capability fence|owned-(section|field|ref) fence|OwnedRefFence|OwnedFieldFence|FencesGithub|FencesWhenLearnings|UnsupportedConfigFence|fenced board|github-fenced" -- . "${EX[@]}")
echo "${out:-(none)}"
'
```

Expected: `(none)`.

- [ ] **Step 5: Focused tests and every tag set**

```bash
bash -c '
set -e
go vet ./... && go vet -tags integration ./... && go vet -tags e2e ./internal/app/
go test -count=1 -run "BoardSurface|Learning|ChangeReconcile|ChangeHalt|ResumeHalted|ChangeKill|ChangeCreate|ChangeGroom|ChangeBlock|ChangeDefer|ChangeUnblock|ChangeRevive" ./internal/app/
go test -count=1 -tags integration -run "^TestIntegrationRepoOwnedRefRefusesForeignRef$" ./internal/gitcli/
go test -count=1 -tags e2e -run "^TestE2EUnsupportedConfigRefused$" ./internal/app/
go test -count=1 ./internal/repoguard/ ./internal/cli/
gofmt -l internal
'
```

Expected: every command passes; `gofmt -l` prints nothing. The e2e run builds the binary and takes longer.

- [ ] **Step 6: Commit**

```bash
git add -- internal/app/planning.go internal/app/change_attach.go internal/app/change_claim.go internal/app/change_create.go internal/app/change_groom.go internal/app/change_halt.go internal/app/change_implemented.go internal/app/change_kill.go internal/app/change_lifecycle.go internal/app/change_reclaim.go internal/app/change_reconcile.go internal/app/change_repair.go internal/app/finalize_block.go internal/app/finalize_closeout.go internal/app/learning_ops.go internal/cli/change.go internal/cli/learning.go internal/gitcli/rebase.go internal/app/planning_test.go internal/app/change_create_test.go internal/app/change_groom_test.go internal/app/change_kill_test.go internal/app/change_lifecycle_test.go internal/app/change_reconcile_test.go internal/app/learning_ops_test.go internal/app/config_test.go internal/app/finalize_e2e_test.go internal/app/finalize_state_integration_test.go internal/gitcli/rebase_integration_test.go internal/repoguard/mirror_writeback_test.go
git commit -m "refactor(0474): non-run fences -> check/refuse wording (resolveBoardSurface, haltPreflight, requireLearningsEnabled)"
```

(Drop any listed path you did not change. `git add` of an unchanged path is harmless, but list what you touched.)

---

### Task 7: The seal, the closing grep and the final checks

**Build tier:** standard — test-code changes to a repo guard, with mutation proof and whole-repo verification.

**Files:**
- Modify: `internal/repoguard/retired_vocabulary_test.go`

**Interfaces:**
- Consumes: every rename from Tasks 1–6 (the seal is green only once they all landed), and Task 5's `close-out.md` path.
- Produces: 11 new `retiredVocabulary` rows (table size 78 → 89) and the closing-grep evidence for the results file.

- [ ] **Step 1: Append family (d)'s rows, raise the floor, extend the header**

At the end of `retiredVocabulary` (after the last `44a` row):

```go
	// Family (d) — re-enable, final, shared-setting guard (change 0474): rows
	// 54-59 and 59a. Row 53 is the concept; rows 59b and 59c are Go names and
	// prose, which the seal does not scan (a guard keyed on passing mentions
	// would violate the guard rule; the results file records the closing grep).
	{Row: "54", Kind: kindToken, Old: "rearm", New: "re-enable"},
	{Row: "55", Kind: kindToken, Old: "nothing-to-rearm", New: "nothing-to-re-enable"},
	{Row: "56", Kind: kindToken, Old: "fenced-setting-ignored", New: "shared-setting-ignored"},
	{Row: "57", Kind: kindToken, Old: "terminal-backlink-pending", New: "final-backlink-pending"},
	{Row: "57", Kind: kindToken, Old: "terminal-notes-frozen", New: "final-notes-frozen"},
	{Row: "58", Kind: kindToken, Old: "change-terminal-claim-stamp", New: "change-final-claim-stamp"},
	{Row: "58", Kind: kindToken, Old: "drop-terminal-claimed-at", New: "drop-final-claimed-at"},
	{Row: "59", Kind: kindToken, Old: "not-terminal", New: "not-final"},
	{Row: "59", Kind: kindToken, Old: "skipped-terminal", New: "skipped-final"},
	{Row: "59", Kind: kindToken, Old: "adr-update-after-terminal", New: "adr-update-after-final"},
	{Row: "59a", Kind: kindToken, Old: "terminal-close-out.md", New: "close-out.md (skills/docket-convention/references/)"},
```

In `testRetiredTableIntegrity`: `const floor = 78` → `const floor = 89`.

In the header comment, after the family (b) paragraph (before `// LIMITATION (byte-pattern-guard-matches-a-spelling)`), add:

```go
// Family (d), re-enable / final / shared-setting guard (change 0474), appends
// rows 54-59 and 59a, all kindToken: the change.groom outcome rearm and its
// nothing-to-rearm refusal, the config guard's fenced-setting-ignored warning,
// the seven lifecycle codes spelled with "terminal", and the renamed close-out
// reference file (row 59a, the first path row). rearm also matches inside
// nothing-to-rearm; the non-vacuity check counts only the planted row's own hits.
```

- [ ] **Step 2: Negative controls for the kept namesakes**

Append to `cleanText` in `testRetiredNegativeControls`:

```go
		// Change 0474 — the new spellings and the kept namesakes (spec §D).
		"apply `change.groom` with `outcome: re-enable`; a `nothing-to-re-enable` refusal writes nothing",
		"`final-backlink-pending`, `final-notes-frozen`, `not-final`, `skipped-final`, `adr-update-after-final`",
		"`change-final-claim-stamp` and `drop-final-claimed-at`; the warning is `shared-setting-ignored`",
		"read `../docket-convention/references/close-out.md` now — blocking",
		"terminal_publish: false stays parseable; terminal publication is deferred from Go v1",
		"a run-continue is nonterminal; incumbent-nonterminal; the run reached a terminal disposition",
		"the frozen fixture testdata/repositories/v0.9.2/fenced-machine-keys/ keeps its name",
		"a signal re-arm escalation is drained and ignored",
		"`skipped-not-open` for a non-final child with no open PR",
```

Append to `cleanGo`:

```go
		{"internal/app/change_groom.go", "package p\nconst o = \"re-enable\"\n"},
		{"internal/config/config.go", "package p\nconst c = \"shared-setting-ignored\"\n"},
		{"internal/app/finalize_retarget.go", "package p\nconst c = \"skipped-final\"\n"},
		{"internal/config/fixtures_test.go", "package p\nvar d = \"fenced-machine-keys\"\n"},
```

- [ ] **Step 3: Run the seal**

Run: `go test -count=1 -run '^TestRetiredVocabularySeal$' -v ./internal/repoguard/ 2>&1 | tail -20`
Expected: PASS for all seven subtests (`catalog_vocabulary`, `table_integrity`, `non_vacuity`, `negative_controls`, `maintained_surfaces`, `generator_output`, `schema_walk`). If `maintained_surfaces` names a site, the spelling survives in a consumer: fix the site in the file that owns it (regenerate the embedded tree if it is under `skills/`), not the seal.

- [ ] **Step 4: Hand mutations for the results file (spec *Seal*)**

Mutation A (the spec's required one: `outcome: rearm` in a skill line):

```bash
bash -c '
f=skills/docket-groom-next/SKILL.md; cp "$f" "$f.bak"
perl -pi -e "s/with \`outcome: re-enable\`, the pinned/with \`outcome: rearm\`, the pinned/" "$f"
command grep -c "outcome: rearm" "$f"
out=$(go test -count=1 -run "^TestRetiredVocabularySeal$/^maintained_surfaces$" ./internal/repoguard/ 2>&1); echo "exit=$?"
command grep -E "row 54: retired \"rearm\" — use re-enable" <<<"$out"
mv -f "$f.bak" "$f"
go test -count=1 -run "^TestRetiredVocabularySeal$" ./internal/repoguard/ >/dev/null; echo "restored exit=$?"
'
```

Expected: count `1`; `exit=1`; a matched line `skills/docket-groom-next/SKILL.md:<n>: ADR-0129 row 54: retired "rearm" — use re-enable: …`; `restored exit=0`.

Mutation B (a Go string literal in non-test source):

```bash
bash -c '
f=internal/app/finalize_retarget.go; cp "$f" "$f.bak"
perl -pi -e "s/\"skipped-final\"/\"skipped-terminal\"/" "$f"
command grep -c "\"skipped-terminal\"" "$f"
out=$(go test -count=1 -run "^TestRetiredVocabularySeal$/^maintained_surfaces$" ./internal/repoguard/ 2>&1); echo "exit=$?"
command grep -E "row 59: retired \"skipped-terminal\" — use skipped-final" <<<"$out"
mv -f "$f.bak" "$f"
'
```

Expected: count `1`; `exit=1`; one matched line. Copy both outputs into `NOTES`.

- [ ] **Step 5: Shard membership and test population (Review Focus 2)**

```bash
bash -c '
B=e3352114cd386e72d04718f1719ae47df7bd9941
for t in TestIntegrationRecordOpsChangeGroomAbstainThenReEnableRealGit TestIntegrationFinalizeCleanupOnlyAfterFinal TestIntegrationRepoOwnedRefRefusesForeignRef; do
  hit=""
  for s in tests/test_go_integration_*.sh; do
    p=$(sed -n "s/^SHARD_PREFIX=\"\(.*\)\"$/\1/p" "$s"); k=$(sed -n "s/^SHARD_PKG=\"\(.*\)\"$/\1/p" "$s")
    case "$t" in "$p"*) hit="$hit $s($k)";; esac
  done
  echo "$t ->${hit:- NO SHARD}"
done
base=$(git grep -h -E "^func Test[A-Za-z0-9_]*\(t \*testing\.T\)" $B -- "*.go" | wc -l)
head=$(git grep -h -E "^func Test[A-Za-z0-9_]*\(t \*testing\.T\)" -- "*.go" | wc -l)
echo "base=$base head=$head delta=$((head-base))"
'
```

Expected: each renamed test maps to a shard whose `SHARD_PKG` is its own package (`./internal/app` for the first two, `./internal/gitcli` for the third), never `NO SHARD`. `delta=4`: the tests this plan adds are `TestChangeGroomRetiredRearmOutcomeRefused`, `TestFinalLifecycleCodeSpellings`, `TestFinalLifecycleFindingSpellings` and `TestRepairDropClaimedAtSpelling`. Every other change is a rename. Any other delta means a test was lost or duplicated: find it.

- [ ] **Step 6: Frozen paths, kept spellings and the budgets file (Review Focus 4, 5)**

```bash
bash -c '
B=e3352114cd386e72d04718f1719ae47df7bd9941
echo "-- frozen paths touched (must be empty):"
git diff --name-only $B -- docs/changes docs/results docs/adrs testdata/repositories internal/repository/testdata internal/install/legacydata internal/install/testdata docs/reference/harness/fixtures
echo "-- docs/superpowers touched (only this plan):"
git diff --name-only $B -- docs/superpowers
echo "-- kept spellings added or removed (must be empty):"
d=$(git diff $B -- . ":!internal/repoguard/retired_vocabulary_test.go" ":!docs/superpowers")
command grep -E "^[-+][^-+].*(json:\"terminal|terminal\.json|terminal_publish|terminalPublish|fenced-machine-keys|GIT_TERMINAL_PROMPT|terminalAllowlist|incumbent-nonterminal|record-terminal|isBareFence|isFrontmatterFence|fenceRun|MutationFenceError)" <<<"$d" || echo "(none)"
echo "-- budgets_test.go diff (only the close-out row move):"
git diff $B -- internal/repoguard/budgets_test.go
echo "-- harness goldens touched (must be empty):"
git diff --name-only $B -- internal/harness
'
```

Expected: the frozen list is empty; `docs/superpowers` shows only `docs/superpowers/plans/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle.md`; kept spellings `(none)`. If a line appears only because an edited line keeps such a spelling unchanged beside a renamed word, confirm the kept token is identical on the `-` and `+` sides and record that. The budgets diff is exactly one removed row and one added row. The harness list is empty.

- [ ] **Step 7: The closing whole-repo grep (spec *Seal* and *Testing*; record the command and its full output)**

The embedded tree is **inside** the scope on purpose, as proof that regeneration ran:

```bash
bash -c '
EX=(":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":!internal/install/legacydata" ":!internal/install/testdata" ":!testdata/repositories" ":!internal/repository/testdata" ":!docs/reference/harness/fixtures")
echo "== (1) retired wire tokens and path"
git grep -n -I -i -E "rearm|fenced-setting-ignored|terminal-backlink-pending|terminal-notes-frozen|change-terminal-claim-stamp|drop-terminal-claimed-at|not-terminal|skipped-terminal|adr-update-after-terminal|terminal-close-out" -- . "${EX[@]}" || echo "(none)"
echo "== (2) re-arm"
git grep -n -I -i "re-arm" -- . "${EX[@]}" || echo "(none)"
echo "== (3) fence outside the run fence and code/frontmatter fences"
RUN=(":!internal/app/runtracker*" ":!internal/app/agent_guardian*" ":!internal/gatedrive" ":!internal/process" ":!tests/test_go_integration_app_runfence.sh")
o3=$(git grep -n -I -i "fenc" -- . "${EX[@]}" "${RUN[@]}")
command grep -v -i -E "run fence|mutation[- ]fence|admission fence|fences? the run|run (was|is) fenced|fenced run|fence is durably|stale-run-id fence|fenceRun|isBareFence|FrontmatterFence|frontmatter fence|code fence|fenced code|inFence|fenceChar|fenceLine|fenceLen|fenceMask|codeFenceRE|UnterminatedFence|FinalFence|bareFence|scanRawFenced|\`\`\`|~~~|runfence|RunFence|fenceNextAction|MutationFenceError|fenceRefusalReasonMessage|workspaceFenceRefusal|prFenceRefusal|errRunFence|FenceRunCompleting|findingRunFenced|incumbent-run-fenced|unfenceable|fenceable" <<<"$o3" || echo "(none)"
echo "== (4) terminal outside the process-sense packages and the kept spellings"
PROC=(":!internal/gatedrive" ":!internal/process" ":!internal/codexentry" ":!internal/suiterunner" ":!internal/app/runtracker*" ":!internal/app/gate*" ":!internal/app/root_entry*" ":!internal/app/agent_*" ":!internal/githubcli" ":!internal/cli/agent*" ":!internal/cli/gate*")
o4=$(git grep -n -I -i "terminal" -- . "${EX[@]}" "${PROC[@]}")
command grep -v -i -E "terminal_publish|terminal[- ]publi(sh|cation)|terminalPublish|GIT_TERMINAL_PROMPT|terminalAllowlist|nonterminal|terminal\.json|terminalFile|TerminalWaiting|mapTerminalDrive|runToTerminal" <<<"$o4" || echo "(none)"
'
```

Expected:
- (1) exactly: `internal/app/change_groom_test.go` (the hard-cut test's `"rearm"`), `internal/repoguard/retired_vocabulary_test.go` (the seal rows and header), and `internal/repoguard/budgets_test.go` (the frozen "0382: +typed rearm exit" history comment). Nothing under `skills/`, `internal/assets/embedded/`, `docs/` or non-test Go.
- (2) exactly: `internal/suiterunner/signal.go` (the kept signal re-arm) and the `retired_vocabulary_test.go` negative control "a signal re-arm escalation".
- (3) only these classes, each listed per file in `NOTES`: a Markdown code fence or frontmatter fence (document, render, repository decode, results/plan content, harness inventory, install legacy, repoguard markdown scanners, `TestChangeGroomReEnableFencedMarkerAgreesWithBoard`, `"fenced-tilde"` fixtures, "unfenced invocation"); the run fence (including `internal/app/gate.go`'s "stale-run-id fence", `gate_drive.go`, `pr_publish.go`, `workspace_ops.go`, `change_claim.go`'s run-context fence comments, `internal/cli/run.go`'s `run cancel` help, `internal/repoguard/gatelaunch_admission_test.go`); the frozen fixture name `fenced-machine-keys` (`internal/config/fixtures_test.go`, `retired_vocabulary_test.go`); ADR-0019's filename link (`docs/concepts/config-layers.md`); "defence" (`internal/install/txn.go`); and the seal rows. No board-surface, learnings, capability, owned-section, owned-ref or config-guard fence may remain.
- (4) only kept senses, listed per file in `NOTES` with their rubric class: process/run (build and status skills, `docs/reference/glossary.md` run entries, `evidence_ops*.go`, `evidence_recertify*.go`, `finalize_rebase*.go`, `run_verify*.go`, `run_waiting.go`, `workspace/*`, `cmd/docket/gate_cli_test.go`, `inline_role_stop_test.go`, `root_entry_dispatch_test.go`, `prose_contracts_test.go` run-sense sentinels, `.docket.example.yml` observation-budget comments, the Terminal-disposition headings, `AGENTS.md`'s "collect its terminal exit"); the decision-procedure sense (`internal/reposetup/classify.go`, `healthconditions.go`, `health_test.go`, `internal/app/repository_check.go`, `internal/repository/transaction/*`, `internal/domain/stack.go`, `stack_test.go` rule-3 lines, `internal/workspace/prepare_integration_test.go`); TTY (`internal/cli/repository*.go`); PR terminal state; the historical `DOCKET_STATUSES(_ACTIVE/_TERMINAL)`; kept test names; and the budgets history comments. Any lifecycle-sense hit is residue: fix it in the file that owns it (regenerate the embedded tree if it is under `skills/`) and re-run this step.

Paste the command block and its full output into `NOTES`, grouped by class, for the results file.

- [ ] **Step 8: Focused tests**

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/harness/... ./internal/config/`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -- internal/repoguard/retired_vocabulary_test.go
git commit -m "test(0474): seal ADR-0129 family (d) retired spellings (rows 54-59, 59a)"
```

(If Step 3 or Step 7 required residue fixes in earlier-task files, add those paths, plus `internal/assets/embedded` when regenerated, to this commit.)

---

## Whole-suite gate (controller-owned, after Task 7)

`docket-build`'s final gate runs the whole suite through `build.test_command`: `go run ./cmd/docket development test`, entered from this worktree so the gate tests this exact checkout. Workers do not run it themselves. Expected: green. Read the budget report even when it is green. A `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding to record, and a `SERIAL CONFIRMED OVER BUDGET:` line is a breach to act on. The integration shards whose tests this plan renames (`test_go_integration_app_recordops.sh`, `test_go_integration_app_cleanup.sh`, `test_go_integration_gitcli_repo.sh`) and the e2e matrix (`test_go_finalize_e2e.sh`) must each report their renamed tests as run.

## Notes for the results file (carried by the final task's NOTES)

- The closing grep command and its full output, classified (Task 7 Step 7). The mutation evidence: the hard-cut assert (Task 1 Step 7), the config code pin (Task 2 Step 6), the `skipped-final` pin (Task 3 Step 7), and the seal's two hand mutations (Task 7 Step 4).
- Spec deviations to record: sites the grooming trace missed and this build renamed — `internal/config` locals `repoFenced` / `wantRepoFenced`; `skills/docket-implement-next/references/edge-paths.md` "repo-fenced" → "repo-only"; `internal/app/install.go` "fenced settings"; `internal/gitcli/rebase.go`'s owned-ref comments; `internal/app/finalize_state_integration_test.go`'s "fenced by the exact old-value"; `resolveBlockTarget`'s `allowTerminal` parameter; the `DOCKET_STATUSES(_ACTIVE/_TERMINAL)` history comment kept as a quoted dropped identifier; the decision-procedure "terminal" kept alongside the spec's `TestHealthTerminalFallbackNamesEveryCause`. Also record any site the closing grep surfaced.
- Follow-up (spec *Out of scope*, not fixed here): if a killed change can reach `finalize cleanup`'s default branch, its "change is not final …" message is as inaccurate as the old "not terminal" one.
- **Human action (Required, after merge):** consumer repos must re-run `docket install`. Their installed skills still name `outcome: rearm` (which the new binary refuses as `invalid-outcome`) and `references/terminal-close-out.md` (which no longer exists). The AGENTS.md post-merge rebuild closes the same window for this repo: until it runs, a groom run with the new skill against the old binary gets a loud `invalid-outcome`.
- **Post-merge install check (spec *Testing*):** after the AGENTS.md rebuild (`development.install`), confirm `ls ~/.claude/skills/docket-convention/references/` lists `close-out.md` and no `terminal-close-out.md`. Skills install as directory links into the version's asset tree, so there is no per-file stale copy to prune. Record the observed listing.
- Landing: no drain (D7). No retired token is persisted in metadata-branch records, `.git/docket` stores or JSON keys.
