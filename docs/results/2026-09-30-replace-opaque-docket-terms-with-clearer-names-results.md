<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0469 — Replace opaque docket terms with clearer names](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0469-replace-opaque-docket-terms-with-clearer-names.md)**
<!-- docket:backlink:end -->
# Replace opaque docket terms with clearer names — Results

**Human action:** Needed at merge. Decide whether two gate-drive halts should get their own tokens (see "Two halt tokens cover conditions they do not name" below). Nothing else blocks the merge.

## Outcome

Docket's run logs, verdict lines, skills and docs used several terms a newcomer could not read without the glossary. This change renames them to plain names (ADR-0129 family (e), rows 67-86), as one hard cut with no aliases:

- Readiness `needs-brainstorm` is now `needs-grooming`, matching the project's own word for that step.
- Gate-drive halt causes: `identity-mismatch` is now `worktree-changed`, and `unresolved-execution` is now `launch-unconfirmed`.
- The CLI verb `docket change repair-identity` is now `docket change relink`, and finalize's "repair checkpoint" is now its "link check".
- Finalize merge conditions: `pr-identity-mismatch` is now `pr-link-mismatch`, and recertify's `identity-drift` is now `certified-input-changed`.
- "Conjunct" is now "condition" everywhere, including the Go type `MergeConditions` and the generated AGENTS.md dispatch block ("unmet conditions").
- Concept renames in prose and comments: worktree slot, moved to background, gate supervisor, gate run, marker section, allowed values, conflict-checked write, read on demand.
- "Step-0 preamble" is now the "startup check". The bootstrap verdict names and `--digest-only` are retired to the glossary's Obsolete terms.
- The retired-vocabulary seal now covers the family (e) wire tokens, so an old spelling that creeps back fails the suite.

Anyone running an old, already-loaded skill that passes a retired token or `change repair-identity` gets a loud unknown-command or unknown-value failure, never a silent fallback.

A whole-branch review found six issues. Four were fixed on the branch before the PR opened:

- Short "Step-0" pointers to the renamed startup check in eight skills were renamed. A skill's own "Step 0" step label was left as it is.
- A gate-drive test fixture still created a scratch directory under the old halt name, so one of its history records was never seen as live. It now builds the directory list from the same cause list as the records.
- Variants of "closed … vocabulary" and "execution slot" in Go comments and two operator messages were reworded. A leftover code block and a circular definition in the glossary were fixed.
- The Go identifiers and files for `change relink` still said "repair" (`change_repair.go`, `RepairRelinkedBranch` and others). They now say "relink". No wire value changed.

## Human actions and testing

### Important — decide on the two borrowed halt tokens

This is a naming decision that the rename rules (ADR-0129) leave to a human. It matters before the old tokens' meaning settles into run logs. If you skip it, two halts keep a token whose remedy text points the reader at the wrong cause.

1. Read "Two halt tokens cover conditions they do not name" below.
2. Choose between keeping the shared tokens and giving each case its own token.
   Expected: if you choose separate tokens, capture a follow-up with `docket change create`. That change needs an ADR-0129 `## Update`.

## Verification performed

- Every plan task ran its focused tests through the gate driver before its commit. The rename tasks also ran mutation checks on the new retired-vocabulary seal rows.
- The full suite (`go run ./cmd/docket development test`) passed after the build. It passed again after the review fixes, on the head that carries this file. The PR's build-evidence block records that head.
- The four Codex repoguard tests that halted the first run (fixed on `main` by bc56f96fa, merged into this branch) pass here without a skip.

## Known issues and follow-ups

### Two halt tokens cover conditions they do not name

Two gate-drive halts borrowed a renamed token for a condition it does not describe. Behavior is the same as before this change, where both already shared the old tokens. Confirmed.

- A takeover whose scope no longer matches the drive (a different repo, branch, worktree, change, task or phase) halts with `worktree-changed`. Nothing in the worktree changed, so a reader following the glossary's remedy ("undo the stray edit") looks for an edit that does not exist.
- `resolveDriveRun` halts with `launch-unconfirmed` when the worktree slot can't be read or its reservation token no longer matches. That is lost linkage, not "nothing proved whether a launch happened".

Suggested next action: the human decision above. One option is to reuse the kept `scope-identity-mismatch` for the takeover halt and add a lost-linkage cause for `resolveDriveRun`.

### A few "repair" and "Step-0" wordings remain

Three explanatory messages from `change relink` still say "repair", for example "the repair would orphan it". Some Go comments, one shell test and the `repository prepare` help text still say "Step 0". None of these is a wire token, and no rename row names them. Confirmed; low impact. Suggested next action: tidy them in a later wording pass if wanted.

### "CAS" kept in Go comments

Around 150 Go comments still say "CAS". There it names the `Store.CAS` / `scopeCAS` / `admissionCAS` functions, which no rename row covers. The prose "CAS" in skills, docs and the glossary now reads "conflict-checked write". Confirmed as a deliberate choice; no action needed unless a later change renames those functions.

### Skill word budgets are nearly full

Several skills now sit within a few words of their size ceilings, for example `skills/docket-build/references/gate-caller-loop.md`, which is exactly at its limit. The next edit to these files will have to cut words somewhere else. Confirmed; no action needed now.
