<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0471 — Rename the run gate to the run tracker (epoch → run id, gate-* → run-*)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0471-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run.md)**
<!-- docket:backlink:end -->
# Rename the run gate to the run tracker (epoch → run id, gate-* → run-*) — Results

**Human action:** Yes. This is a hard cut with no aliases, so follow the landing procedure below when you merge: merge only with no dispatched run in flight, rebuild the binary right away, restart coordinator sessions, re-run `docket install` in consumer repos, and switch binaries on each machine only when no run is in flight there.

## Outcome

Docket's run-bookkeeping feature used to be called the "run gate", and a run's id was called an "epoch". Both names clashed with other docket terms, so change 0468 (ADR-0129) settled new names. This change applies ADR-0129 family (a), rows 1–38 and 38a–38d, across the whole wire surface, with no aliases:

- **Operations and verbs:** `run.gate-before` / `run.gate-verdict` / `run.gate-claim` are now `run.start` / `run.verdict` / `run.continue`, along with their `docket run …` verbs.
- **Flags:** `--run-epoch` and `run cancel --epoch` are now `--run-id`, `change claim --gate-context` is now `--run-context`, and `agent enter --run-gate-key` is now `--run-key`.
- **Report lines:** `gate-armed` / `gate-unarmed` are now `run-started` / `run-untracked`, each verdict line `gate-*` is now `run-*`, and `gate-claimed` is now `run-continued`.
- **Codes and stages:** the `epoch-*` codes and stages are renamed by meaning, `gate-unavailable` is now `run-tracker-unavailable`, and `gate-context-*` is now `run-context-*`.
- **Keys and env var:** the result key `epoch` is now `run_id`, the claim request key `gate_context` is now `run_context`, and the guardian env var is now `DOCKET_AGENT_GUARDIAN_RUN_ID`.
- **Local stores start empty under new names:** `run-tracker/`, `run-tracker-resume/`, `run.json`, `run.lock`, and `v2` roots for gate admission, gate scopes and gate drives. Nothing is migrated. The claim receipts committed on `docket` keep their `gate_context_hash` key.
- **Renames that follow from the above:** files, integration shards, Go identifiers, test names, skills, the embedded asset tree, generated dispatch material, the AGENTS.md block, the concept page (`docs/concepts/run-tracker.md`), the glossary and the cursor rule (`cursor-rules/run-tracker.md`).
- **A new guard:** a retired-vocabulary table in `internal/repoguard` (`retired_vocabulary_test.go`) blocks the retired spellings at executable sites. It is mutation-tested.

Departures from the plan:

- **Two more string literals.** The seal caught two Go strings the renames had missed. The cancel finding token `replacement-epoch-unreadable:<key>` is now `replacement-run-record-unreadable:<key>` (row 28's family), and the store's error prefix `rungate store` is now `run-tracker store`.
- **A narrower check for the `--gate-context` row.** The gate drive's own `--gate-context` flag is not an ADR-0129 row, so it keeps its name. For that reason the seal checks row 12 by the nearest command noun on the line rather than across the whole line.
- **Task 7 landed as two commits.** A `git add` problem split it: `7f3416dae` holds only the cursor-rule file rename, and `3b37c3158` holds the content. The tree at `7f3416dae` alone is inconsistent, but the branch head is not.

## Human actions and testing

### Important — Land with no run in flight, then rebuild and reinstall

**Why this matters.** The retired spellings stop working the moment the new binary is installed. Any coordinator or consumer repo still using them fails loudly, and a run already in flight cannot be finished from the new binary, because the new binary does not read the old `v1` drive roots. Nothing automated checks for runs in flight.

**Prerequisites.** Check that no `docket-implement-next` or `docket-finalize-change` run is active in any docket repo on this machine. Drain the running ones or cancel them with `docket run cancel`.

1. Merge the PR.
   Expected: the PR lands on `main`.
2. Run the post-merge binary rebuild described in AGENTS.md: `repository.sync-integration`, then `development.install --source /Users/homer/dev/docket`.
   Expected: `docket version` reports the merged commit, and `docket capabilities --json` lists `run.start`, `run.verdict` and `run.continue`, with no `run.gate-*` entries.
3. Restart every open Claude Code or Codex coordinator session.
   Expected: the CLAUDE.md or AGENTS.md they load names `run.start` / `run.verdict`.
4. Re-run `docket install` in each consumer repo.
   Expected: the generated dispatch material has no `run.gate-*` or `--run-epoch` tokens.
5. On any other machine, switch binaries only when no run is in flight there.
   Expected: the first `docket run start implement-next` prints `run-started <key> <run-id> <run-context>`.

**Cleanup (optional).** You can delete the old roots by hand: `.git/docket/rungate`, `.git/docket/rungate-resume`, `.git/docket/gate-admission/v1`, `.git/docket/gate-scopes/v1` and `.git/docket/gate-drives/v1`.

**Recovery if a step was skipped.** Use the new binary to run `docket run verdict`, `docket run start --resume <id>` or `docket run cancel --run-id`, then re-dispatch.

## Verification performed

- **Every task:** each task ran its focused checks through task-owned gate drives, which all passed: build, vet with and without the integration tag, unit tests of the packages it touched, and focused integration tests.
- **Task 1 (store reset):** mutation probes showed that the explicit JSON tags, the kept `gate_context_hash` receipt key, and the new root names are each guarded by a test.
- **Task 4 (shard renames):** each integration shard has the same test count before and after the rename, and the integration contract script passes.
- **Task 9 (the seal):** eight mutation probes each turned the seal red, and each went green again after restoring the file.
- **Whole-branch review:** a deep review returned 3 important and 3 minor findings, and 5 of them were fixed in the branch:
  - `e9df8b755`: identifiers where "gate" means a checkpoint, which the rename had changed, were restored.
  - `0ec6dfaa5`: two remedy messages that the rename had garbled were rewritten, and a test now asserts each one.
  - `0314ddd75`: the row-12 seal now treats every `--gate-context` as retired unless it is bound to a gate-drive command. It is mutation-tested against the old skill wording.
  - `da95bac8c`: leftover prose misfires were fixed ("run run", wrong articles, "dispatch context", "keyed gate"), and the bundle-sentence pin was tightened.
  - The sixth finding, an ADR note for a token outside the table, is deferred (see below).
  - There was no second review round after the fixes.
- **Full suite:** the certifying gate result for the final head is recorded in the PR's build-evidence block.

## Known issues and follow-ups

### The gate drive's own `--gate-context` flag keeps its old name

**When it occurs:** `gate drive start` and `gate drive prepare-scope` still take `--gate-context`, so a caller passes the same token as `--run-context` to `change claim` and as `--gate-context` to the gate drive.

**Impact:** only the naming is inconsistent; nothing breaks.

**Status:** confirmed, and left alone on purpose, because ADR-0129 row 12 covers only `change claim`.

**Next step:** a human can decide whether to capture a follow-up change for it.

### ADR-0129 does not yet record the extra token rename

**What it is:** the cancel finding token `replacement-epoch-unreadable:` is not one of ADR-0129's rows, but this change renamed it to `replacement-run-record-unreadable:`, following the pattern of row 28. ADR-0129's deviations clause asks for a dated `## Update` note when a family change departs from the table.

**Impact:** the ADR, which is the authority for naming, does not list this rename.

**Status:** confirmed. It was left for a human because changing an Accepted ADR is a deliberate metadata write.

**Next step:** add a dated `## Update` note to ADR-0129, or record why none is needed.

### `DOCKET_AGENT_GUARDIAN_GATE_KEY` is not renamed

**What it is:** this env var is not an ADR-0129 row, so it keeps its name.

**Next step:** consider it alongside the `--gate-context` follow-up above.
