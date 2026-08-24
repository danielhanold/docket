<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0339 — Retire the gate-run.sh launch/liveness/stop facade now that the native Go-v1 gate is canonical (collapse the shared docket-liveness.sh seam with runner-dispatch.sh)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-24-0339-retire-the-gate-run-sh-launch-liveness-stop-facade-now-that.md)**
<!-- docket:backlink:end -->
# Retire the gate-run.sh Facade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the `gate-run.sh` launch/liveness/stop shell facade so the native Go-v1 gate (`docket gate launch|observe|stop`) is the sole supervisor, moving the surviving caller guidance into a **new** reference file `skills/docket-build/references/gate-caller-loop.md` (NOT `gate-execution.md` — see the revision note below) and leaving `scripts/lib/docket-liveness.sh` with `runner-dispatch.sh` as its sole consumer.

**Architecture:** Create-new-home first, retarget second, delete last — and the **full suite is green at every single commit**. The new reference file lands complete with its own size-budget row in one commit (content duplicated with the still-present `gate-run.md` is the intended intermediate state); `gate-execution.md`, then `docket-build/SKILL.md`, retarget onto the new file and the native verbs, each **in the same commit as the posture-test asserts that read the edited prose**; the caller-loop fence gains its own test shard; only then do the facade files, their shards, and the `WRAPPED_OPS` entry die; liveness-lib ownership prose closes it out.

**Tech Stack:** Bash test shards under `tests/run-tests` discipline, Go (`internal/process`, `internal/cli`, `internal/app` — read-only reference), `cmd/genassets` for the embedded skill tree, jq for protocol-v1 JSON.

**Spec:** `docs/superpowers/specs/2026-08-23-retire-gate-run-facade-design.md` — the **revised** spec (its "Revision note (2026-08-23)" supersedes the original at the same path; the feature tree does not carry it — read it from the metadata worktree or origin/docket).

## Why this is a re-plan (read before Task 1)

A build against the original spec halted **twice**, both times on the same defect class: an edit landed in one commit and the guard that admits it in the next, so a commit boundary left the full suite red. The revised spec re-settles two decisions:

1. **Defect 1 — the old home was unbuildable.** Moving the caller guidance into `gate-execution.md` blew its 130/1200 size budget (measured 258/2493), and that budget is a deliberate one-directional ratchet (change 0234), not slack. The content now goes to a **new file with its own first-row budget**, following change 0271's `delegation-execution.md` precedent.
2. **Defect 2 — the stop verdicts had no native twin.** The shell tokens `stopped`/`already-terminal`/`unavailable` do not exist natively: `docket gate stop` returns a `GateResult` whose envelope `result` is `applied`/`no-op`/an error, with `state` preserved. An explicit **stop mapping table** in the new file re-grounds the three caller dispositions, and both the SKILL bullets and the posture test's derivation bind to it.

The branch was **reset to drop the abandoned Task 1 build commit** (`14cd56c9`); nothing from that attempt is salvaged or re-applied. This plan starts from a content-clean branch.

## Global Constraints

- **THE FULL SUITE IS GREEN AT EVERY COMMIT.** Every task ends by running the whole suite — the command `finalize.test_command` resolves to, `scripts/run-tests.sh`, never a second copy — **including `tests/test_skill_size_budgets.sh` and `tests/test_gate_execution_posture.sh`**, and requiring green before the commit. This is the defect class that halted the build twice; it is a per-task gate here, not an end-of-build one. (docket-build's single end gate still runs in addition.)
- **Rule (a):** `gate-caller-loop.md`'s `BUDGETS` row in `tests/test_skill_size_budgets.sh` lands **in the same commit that creates the file** — the budget test's completeness guard reddens the instant any `skills/**/*.md` exists without a row.
- **Rule (b):** each `tests/test_gate_execution_posture.sh` retarget lands **in the same commit as the prose edit it guards**. Neither ordering survives alone: an old assert against new prose is red, and a new assert against old prose is red.
- **Every commit touching `skills/` regenerates the embedded assets** (`go run ./cmd/genassets`) in that same commit — the manifest drift guard reddens otherwise.
- **Ordering spine:** Tasks 1–4 are strictly ordered; nothing may delete `gate-run.md` (Task 6) before Tasks 2–4 have retargeted **every** derivation and pointer that reads it. Tasks 5 and 7 are order-free after their predecessors; this plan runs them in numeric order.
- **No wrapper, no shim, no deprecation window** — after Task 6, `docket.sh gate-run` fails like any unknown op (spec decision 1).
- **`runner-dispatch.sh` migration is out of scope**; `scripts/lib/docket-liveness.sh` keeps its **code byte-unchanged** — prose edits only (spec decisions 2–3).
- **The 0338 boundary:** no edit to the observe format or the protocol-v1 schema anywhere; the caller-loop fence moves **byte-identical**.
- **Exact field spellings, enum values, and state names are read from source at implementation time** (`internal/app/gate.go`, `internal/cli/gate.go`, `internal/process`), never restated from memory; where this plan writes a spelling, source wins. The mapping table's *semantics* (which outcome earns which disposition) are settled by the spec and are not implementation-time discretion.
- **`gate-execution.md` keeps its 130/1200 budget.** Current actuals 126/1131 — four lines, 69 words of headroom. If Task 2's retargets exceed it, **trim to fit, never raise** — raising that row is what defect 1 was.
- **Consuming-repo path rule:** the new reference file is a skill body, so `tests/test_consuming_repo_scripts.sh` forbids any bare `scripts/<name>.sh` path in it. The retired facade is referred to **by name** — *the retired `gate-run` shell facade* — never by path; the liveness lib is not named at all.
- **No ADR is minted or amended.** ADR-0095 already records the native supervisor's session guarantee; this change consumes it.
- **Frozen records stay untouched:** everything under `docs/changes/archive/`, `docs/results/`, `docs/superpowers/plans/` (prior plans), `docs/superpowers/specs/` (prior specs), and `docs/adrs/` keeps its `gate-run` mentions — rewriting them falsifies history. The size-budget test's historical ledger comments (the change-0282/0234 entries discussing `gate-run.md`) are point-in-time records of past raises and stay as written; only the **new** row's ledger entry is added.
- **Classified non-targets (never touched):** `internal/app/evidence_ops.go` `"gate-running"` (evidence-gate reason string, unrelated); `internal/app/finalize_cleanup_test.go` / `finalize_git_test.go` "gate-run" fixture names (native gate *run dirs*, not the facade); `tests/test_docket_facade.sh` comment ("`gate-run` (change 0282) drifted exactly that way") — a past-tense historical claim that stays true after the deletion.
- Agent-authored sweeps run under explicit `bash -c` and verify with `command grep` / `git grep`, never the interactive shell's bare `grep` (learnings: *agent-shell-noop-reads-as-success*). Assert on effect (`git diff --stat`, match counts), never on exit 0.
- **Mutation-proof each retargeted assert**, and take a **backup copy of the file before mutating, restoring from that copy** — never `git checkout --` over uncommitted work and never re-typing from memory (learnings: *mutation-restore-needs-a-backup-copy*). Confirm each edit landed by re-reading the file (learnings: *agent-shell-noop-reads-as-success*).

## The grep-derived site ledger (authority for every task)

Derived by whole-repo grep at plan time (`git grep -ln 'gate-run\|gate_run'`), sorted per the house rule. Re-derive at Task 5 to prove the sweep closes.

| Class | Sites | Disposition |
|---|---|---|
| Delete | `scripts/gate-run.sh`, `scripts/gate-run.md`, `tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh`, `tests/lib/gate_run_common.sh` | Task 6, after Tasks 1–4 relocate what survives |
| Executable edit | `scripts/docket.sh` (WRAPPED_OPS + header comment), `tests/test_gate_execution_posture.sh` (Tasks 2–3), `tests/runtime-budgets.tsv` + `tests/test_runtime_budgets.sh` (Task 4 adds a row; Task 6 deletes two), `tests/test_skill_size_budgets.sh` (Task 1 adds a row) | Tasks 1–4, 6 |
| Maintained prose edit | `skills/docket-build/SKILL.md` (Task 3), `skills/docket-build/references/gate-execution.md` (Task 2), `scripts/docket.md` table row (Task 6), `scripts/check-test-source-hygiene.sh` + `.md` (their three-example `tests/lib/*_common.sh` lists name `gate_run_common.sh` — Task 6), `tests/README.md` file count (Tasks 4, 6), `scripts/lib/docket-liveness.sh` comments, `scripts/runner-dispatch.sh` comments, `scripts/runner-dispatch.md`, `tests/test_docket_liveness.sh` (comments only, asserts unchanged) (Task 7) | Tasks 2–4, 6, 7 |
| Frozen / non-target | listed in Global Constraints | never touched |

Plan-time grep found **zero** `gate-run` hits in `skills/docket-finalize-change/**`, `skills/docket-convention/**`, or `skills/docket-implement-next/**` — the spec's "known-likely carriers" list overestimates; the grep is the authority the spec itself names. Task 5 re-proves it.

---

### Task 1: Create `gate-caller-loop.md` + its budget row (one commit, rule (a))

**Files:**
- Create: `skills/docket-build/references/gate-caller-loop.md`
- Modify: `tests/test_skill_size_budgets.sh` (one new `BUDGETS` row + its ledger comment)
- Modify: `internal/assets/embedded/` (regenerated)
- Read-only sources: `scripts/gate-run.md` (§ *The caller's loop*, § *Per-platform capability note*), `internal/app/gate.go`, `internal/cli/gate.go`, `internal/process/launch.go`, `docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md`

**Interfaces:**
- Produces: the new reference file with top-level sections, in this order and with these exact headings — `## The caller's loop`, `## State vocabulary and retryability`, `## The caller's verbs`, `## The stop mapping table`, `## Per-platform capability note (shell-era measurement record)` — plus the evidence-carryover paragraph immediately after the last. Task 2's cap5 pointer, Task 3's SKILL pointer and posture derivations, and Task 4's fence-extraction awk all anchor on these headings and on the two tables' first columns. Nothing reads the file at this commit, so duplication with the still-present `gate-run.md` keeps the suite green.

- [ ] **Step 1: Write the file's charter and the caller's loop.** Open with the charter, stated on the page (spec § The new reference file): **the caller-side contract for driving the native gate** — the loop, the vocabulary it resolves into, the stop mapping, and the measurement record behind the launch shape. It is a caller contract, not a harness quarantine; that axis separates it from `gate-execution.md` (the same axis change 0271 used for `delegation-execution.md`). Then `## The caller's loop`: move from `gate-run.md` § *The caller's loop* the intro paragraph (jq is the loop's **required dependency**; a missing jq is a loud terminal diagnostic, never a silent spin), the ```bash fence **byte-identical** (it already drives `docket gate observe <run-dir> --json` and parses with jq — a move, not a redesign; the 0338 boundary forbids touching anything inside the fence, including its comment lines), and the two follow-on paragraphs ("Never re-derive the state by hand…", "The loop RESOLVES the native spellings…"). Three prose-only retargets in the surrounding text: (1) the sentence naming the fence's executor now names `tests/test_gate_caller_loop.sh` (created in Task 4; nothing asserts the name before then); (2) the pointer to the caller's disposition policy keeps pointing at `skills/docket-build/SKILL.md` § *Gate execution posture*; (3) the intro's parenthetical citing `scripts/ensure-docket-env.sh` and `scripts/docket-status.sh` as jq precedents is **dropped** — the consuming-repo path rule forbids `scripts/<name>.sh` spellings in a skill body, so say "already a docket dependency elsewhere" without the paths.

- [ ] **Step 2: Write the state vocabulary and retryability rule.** `## State vocabulary and retryability`: the observed protocol-v1 states, **read from `internal/app/gate.go` (`GateResult` / `GateState`, which mirrors `internal/process.State`'s spellings) at implementation time** and said so on the page. The rules, verbatim in intent from the spec: `signaled` and `vanished` both resolve to `died` (with `cause` carrying the document's own qualifier, possibly empty); a `died` run **never finished**, so it is never `failed`, and only `failed` may feed repair work; **only `running` is retryable** — every other arm is terminal, including the fail-closed unknown-document arm.

- [ ] **Step 3: Write the caller's-verbs table.** `## The caller's verbs`: a three-row table — first column one code-span verb per row (`launch`, `observe`, `stop`) — naming what each verb returns to the caller (launch: the run-dir handle in a protocol-v1 JSON envelope, or a failure envelope; observe: one protocol-v1 JSON document per call; stop: a `GateResult` per the mapping table below). Add the explicit note that `recover` and `cleanup` are **operator verbs, not caller-loop verbs** — the native CLI registers five subcommands and only these three belong to the loop. This table exists because Task 3's posture test derives its verb coverage from a published table rather than a hand-list.

- [ ] **Step 4: Write the stop mapping table.** `## The stop mapping table`: state what native `docket gate stop` returns — a `GateResult` whose envelope `result` is `applied` or `no-op` (or an error), with `state` preserved; quote `internal/app/gate.go`'s own clause verbatim: *"termination is applied; an already-terminal no-op carries the preserved state (consumers read state; the stop performed nothing)."* Then the table mapping the two-axis answer onto the three caller dispositions. **Table shape is load-bearing:** the first column of each row **opens with exactly one code-span token**, because Task 3's posture-test derivation slices this column and Task 3's SKILL bullets each open with their row's token:

  | Native stop outcome | What it means | Caller disposition |
  |---|---|---|
  | `no-op` — `state` preserved | the run was already finished; the stop performed nothing | **re-observe and key on the preserved `state`** — this is the ordinary outcome of stopping a live child |
  | `applied` | we terminated it; the run produced no verdict of its own | **one relaunch only**, and only where the child is idempotent |
  | `error` — any error result, or the run unreachable | nothing can be proven about what survives | **abort and report loudly**; never relaunch |

  `applied`/`no-op` are read from `internal/app/result.go` (`ResultApplied`/`ResultNoOp`) — verify at implementation time; if a spelling differs, source wins and the table is written to match. `error` is the caller-vocabulary label for the whole error family (the envelope has many error results and no single spelling); the row's second cell says so. The mapping's semantics are settled here.

- [ ] **Step 5: Move the per-platform capability note as a measurement record, then the carryover paragraph.** `## Per-platform capability note (shell-era measurement record)`: `gate-run.md` § *Per-platform capability note* with verbatim intent — the setsid(1)/script(1)/`set -m` rung ladder, the measured `script(1)` rejection with its captured bytes, and the ADR-0080 and `gate-execution-evidence.md` quotes — introduced with a sentence labelling the whole section as **measurements taken against the retired `gate-run` shell facade's launch shape** (by name, never by path). Immediately after, the **evidence-carryover paragraph**: the native launcher is not bound by the shell-era rung-3 narrowing — ADR-0095 records that the native per-run supervisor delivers a **genuine new session on both Darwin and Linux**, and the Go launcher's own comment in `internal/process/launch.go` describes it (quote the actual clause verbatim from source at implementation time — the spec's paraphrase is *"a Setsid session-leader supervisor with the live lock and a handshake"*; anchor on the symbol/quoted clause, never a line number, per ADR-0054). The native guarantee is therefore **at least as strong** as the shape the per-harness verdicts were measured under, so those verdicts **carry over without re-probing** — and that carryover is recorded here rather than left silent.

- [ ] **Step 6: Sweep the page for path spellings and stray facade references.** `bash -c 'command grep -nE "scripts/[a-z-]+\.(sh|md)" skills/docket-build/references/gate-caller-loop.md'` — expect **zero** hits. `bash -c 'command grep -n "gate-run" skills/docket-build/references/gate-caller-loop.md'` — every hit must be the by-name historical form inside the measurement record or carryover paragraph, with no path spelling. Fix any other.

- [ ] **Step 7: Add the budget row + ledger comment.** Measure: `wc -l -w skills/docket-build/references/gate-caller-loop.md`. Size per the table's own rounding rule (stated in its header comment): lines up to the next multiple of 5, words to the next multiple of 50 — and if either lands within the 25-word (resp. zero-line) near-zero band, take the multiple after. Insert the row alphabetically in the `BUDGETS` heredoc, between `references/delegation-execution.md` and `references/gate-execution-evidence.md`:

  ```
  skills/docket-build/references/gate-caller-loop.md          <L> <W>
  ```

  (align columns with the neighbors; `<L>`/`<W>` from the measured actuals per the rule). Above the table, append the ledger entry: a NEW row added by change 0339, first-row derivation from the measured actuals (state the numbers), and — per the table's own naming requirement — name **`gate-execution.md`** as the home considered and rejected (its 130/1200 budget is change 0234's one-directional ratchet and its charter is the harness quarantine; a caller-side rule cannot live there) plus **`gate-execution-evidence.md`** as the second (evidence of probes, not a caller contract).

- [ ] **Step 8: Regenerate embedded assets.** `go run ./cmd/genassets && go test ./internal/assets/`.

- [ ] **Step 9: Run the FULL suite.**

Run: `scripts/run-tests.sh`
Expected: green. `test_skill_size_budgets.sh` passes (row + file in one commit), `test_consuming_repo_scripts.sh` passes (no path spellings), `test_gate_run.sh` and the posture test are untouched and still read the still-present `gate-run.md`. Any red is fixed **in this task** before committing.

- [ ] **Step 10: Commit.**

```bash
git add skills/docket-build/references/gate-caller-loop.md tests/test_skill_size_budgets.sh internal/assets
git commit -m "docs(0339): new gate-caller-loop reference with its size-budget row"
```

---

### Task 2: Retarget `gate-execution.md`'s two pointers + posture group (12d) (one commit, rule (b))

**Files:**
- Modify: `skills/docket-build/references/gate-execution.md` (capability 5's parenthetical; the "One mitigation…" paragraph)
- Modify: `tests/test_gate_execution_posture.sh` (the *(12d) reference:* asserts and the per-harness negative asserts — the asserts that read `gate-execution.md`; **nothing that reads `SKILL.md` or `CONTRACT`**, which stay with Task 3)
- Modify: `internal/assets/embedded/` (regenerated)

**Interfaces:**
- Consumes: Task 1's file and headings.
- Produces: `gate-execution.md` whose capability 5 points at `gate-caller-loop.md` as the vocabulary owner and whose mitigation paragraph names `docket gate launch`; posture asserts retargeted to match. Task 3's SKILL rewrite relies on nothing here; Task 6's deletion relies on `gate-execution.md` no longer referencing `gate-run.md`.

- [ ] **Step 1: Retarget capability 5's pointer.** In `## The six required capabilities` item 5, the parenthetical "…defined once in [`scripts/gate-run.md`](../../../scripts/gate-run.md)…" now names [`gate-caller-loop.md`](gate-caller-loop.md) as the owner of the state vocabulary and its retryability rule. Keep item 5's numbered-item shape (the test's `cap5_blk` awk slices `/^5\. /` up to the next numbered item or heading).

- [ ] **Step 2: Rewrite the mitigation paragraph.** The paragraph opening `One mitigation satisfied all of them…`: the shipped implementation becomes **`docket gate launch`** (implemented in `internal/process/launch.go` — symbol anchor, no line number), replacing the facade invocation sentence ("Docket ships that mitigation as `gate-run.sh`, reached through the facade as …`docket.sh gate-run` and specified by [`scripts/gate-run.md`]…"). The runtime-probe narrowing sentence ("**What its detachment delivers is decided by a runtime probe…** …in that page's *Per-platform capability note*") is rewritten to ADR-0095's uniform guarantee: the native launcher delivers a genuine session on every supported platform, so the page drops the "narrows honestly per platform" hedge and no longer points at a per-platform note it does not own. **Slice-anchor constraints:** the paragraph must keep opening with `One mitigation` at column 0 and contain no internal blank line (the test's `mitigation_blk` awk is `/^One mitigation/`…blank-line); if the rewrite must change that anchor, update the awk in the same commit.

- [ ] **Step 3: Verify the file's size against its UNCHANGED budget.** `wc -l -w skills/docket-build/references/gate-execution.md` — must stay ≤ 130 lines / 1200 words (currently 126/1131). Over budget ⇒ **trim the paragraph to fit, never raise the row** (Global Constraints). No harness row (`### claude|cursor|codex|opencode`) is touched — no verdict rewritten or re-probed. Confirm the residue: `bash -c 'command grep -n "gate-run" skills/docket-build/references/gate-execution.md'` — expect zero hits.

- [ ] **Step 4: Retarget the (12d) asserts and re-key the harness negatives** (copy `tests/test_gate_execution_posture.sh` aside first for mutation-restores). Three edits, each **removal-detecting** (learnings: *assert-detects-removal-not-replacement*):
  - The cap5 pointer assert (`"reference: capability 5 points at the contract that owns the state vocabulary"`, currently `grep -qF -- "gate-run.md" <<<"$cap5_blk"`) becomes: `grep -qF -- "gate-caller-loop.md" <<<"$cap5_blk"` **and** `! grep -qF -- "gate-run.md" <<<"$cap5_blk"`. Update the message to name `gate-caller-loop.md` as the vocabulary owner.
  - The mitigation invocation assert (`"reference: the mitigation names the facade invocation of its shipped implementation"`, currently `grep -qE "docket\.sh gate-run" <<<"$mitigation_blk"`) becomes: `grep -qE "docket gate launch" <<<"$mitigation_blk"` **and** `! grep -qF -- 'gate-run' <<<"$mitigation_blk"` (note `-F --`: the AGENTS.md leading-`--` literal-safety rule; a bare pattern error inside a negated assert inverts into a vacuous green). Rewrite the assert's long comment: the consuming-repo-guard rationale for requiring the facade spelling is replaced by the native-invocation rationale (the shipped implementation is now a native subcommand, which `test_consuming_repo_scripts.sh` permits).
  - The per-harness negative asserts (`"verdicts: '$h' row names no helper — no verdict was rewritten or re-probed"`, `! grep -qF -- "gate-run" <<<"$h_blk"`): their literal premise dies with the facade, which would leave them permanently, vacuously green. **Re-key onto the current implementation's names**: `! grep -qF -- "docket gate launch" <<<"$h_blk"` **and** `! grep -qF -- "gate-caller-loop" <<<"$h_blk"` (keep the old `! grep -qF -- "gate-run"` conjunct too — it stays meaningful until Task 6 and is harmless after). Update the comment to say what the property is: no verdict row was rewritten to name the shipped implementation.

- [ ] **Step 5: Mutation-test each retarget.** Re-add `gate-run.md` to cap5's parenthetical → cap5 negative reddens; delete `docket gate launch` from the mitigation paragraph → positive reddens; re-add `docket.sh gate-run` to the paragraph → negative reddens; add `docket gate launch` to the claude harness row → that row's re-keyed negative reddens. Restore each from the aside copies; re-read the file to confirm restoration landed.

- [ ] **Step 6: Regenerate embedded assets.** `go run ./cmd/genassets && go test ./internal/assets/`.

- [ ] **Step 7: Run the FULL suite.**

Run: `scripts/run-tests.sh`
Expected: green — the posture test's (12d) group and the reference agree in this commit; (12a)/(12b) and `CONTRACT` still read the untouched `SKILL.md` and `gate-run.md`.

- [ ] **Step 8: Commit.**

```bash
git add skills/docket-build/references/gate-execution.md tests/test_gate_execution_posture.sh internal/assets
git commit -m "docs(0339): gate-execution.md points at gate-caller-loop; native launch named as the mitigation"
```

---

### Task 3: Rewrite the SKILL posture to the native verbs + posture groups (12a)/(12b) (one commit, rule (b))

**Files:**
- Modify: `skills/docket-build/SKILL.md` (§ *Gate execution posture* — the "shipped implementation" passage, "On a failed launch", "On the died state", "Abandoning a live child")
- Modify: `tests/test_gate_execution_posture.sh` (`CONTRACT`, the verb derivation, the stop-token derivation, the helper invocation/pointer asserts, the (12b) prose asserts)
- Modify: `internal/assets/embedded/` (regenerated)
- Read-only: `internal/cli/gate.go`, `internal/app/gate.go`, `internal/app/result.go` (exact flag and field spellings)

**Interfaces:**
- Consumes: Task 1's headings and the two tables (verbs table first column: `launch`/`observe`/`stop`; stop mapping table first column tokens: `no-op`/`applied`/`error`).
- Produces: SKILL prose with **no `gate-run` spelling in any form**, naming `docket gate launch --root <dir> --cwd <dir> -- <command…>` and `docket gate stop <run-dir>`, pointing at `references/gate-caller-loop.md` § *The caller's loop*, and carrying three died-disposition bullets each opening with a code-span mapping-table token. The posture test derives verbs and stop tokens from `gate-caller-loop.md`'s tables. Task 6's deletion relies on the posture test no longer reading `gate-run.md`.

- [ ] **Step 1: Read the native spellings from code.** From `internal/cli/gate.go`: the launch usage (`launch --root <dir> --cwd <dir> -- <argv...>`), the stop usage (`stop <run-dir>`), and how JSON output is selected (whether `--json` is a root/persistent flag or per-command — read, don't assume). From `internal/app/gate.go` + `result.go`: the launch failure shape (the envelope's `result`/`reason` on a failed launch — the shell's `launch-failed` token has no native twin) and the stop verdict fields (`result` = `applied`/`no-op`, `state` preserved). Record what you found in the build evidence; if any spelling differs from Task 1's table, **fix the table in this commit** (source wins) and note it.

- [ ] **Step 2: Rewrite the "shipped implementation of clauses 1–3" passage.** Launch is `docket gate launch --root <dir> --cwd <dir> -- <command…>` (spelling per Step 1); stop is `docket gate stop <run-dir>`. The observe sentences (native since 0338) keep their meaning; drop "The plain-text state-name observe contract is retired; `gate-run --observe` refuses with a pointer" — the facade it describes is going. Retarget "**Reuse the canonical loop** in `gate-run.md` § *The caller's loop*" to `references/gate-caller-loop.md` § *The caller's loop*. **Keep intact**, because the posture test's shape asserts pin them: the state-keyed-never-marker-keyed sentence; "only `running` is retryable"; the `docket gate observe <run-dir> --json` + jq binding; the "hand-rolled reading … spun the 0337 gate" clause; and keep the `**Reuse the canonical loop**` bold **mid-line**, never at column 0 — the test's `para()` closes its slice at a column-0 `**`, so reflowing it truncates `helper_blk` and reddens three asserts against a file where the sentence is plainly present (the test's own comment says so).

- [ ] **Step 3: Rewrite "On a failed launch."** A failed `docket gate launch` is read from the launch's own protocol-v1 JSON envelope with jq (name the discriminating field from Step 1); the disposition is unchanged — **abort and report** per *Halting conditions*, never a retry loop, never observed (no handle exists to observe). The slash-free-token prose dies with the shell contract.

- [ ] **Step 4: Re-key "On the died state" on the mapping table's rows.** Three bullets, one per row of `gate-caller-loop.md`'s stop mapping table, each opening column-0 `- ` with its row's native outcome in a code span (`no-op`, `applied`, `error`) so each token still owns a disposition of its own. The posture's meanings are preserved exactly:
  - `` `no-op` `` — the **ordinary** outcome of stopping a live child (the "ordinary" naming survives the re-grounding on this row); the state is preserved, so **re-observe and key on what returns**: an observed `passed`/`failed` keeps that verdict, a `died` resolution takes the one relaunch, anything else never relaunches.
  - `` `applied` `` — we terminated it; the run produced no verdict of its own. **Relaunch once**, and only where the child is idempotent.
  - `` `error` `` — **abort and report loudly, without relaunching**: what survives could not be proven to be this run's.
  Keep, verbatim in intent: `died` is never a red suite and mints no repair work; the one relaunch is licensed by **idempotence**, not by the state, and is **gated on what the stop reported**; a non-idempotent child keeps its site's existing posture; a second `died` is abort-and-report, never a third attempt. The stop invocation spelling is `docket gate stop <run-dir>` throughout, including *Abandoning a live child* (which is otherwise unchanged). **Delete, deliberately:** the two-vocabulary disambiguation sentence ("Two vocabularies overlap here … `stopped` is a stop token *and* an observe state") — after re-grounding, the stop axis no longer produces `stopped`, so the collision it defused is gone. Its removal is a settled spec decision (assumption 3), not an oversight.

- [ ] **Step 5: Verify no facade spelling survives and the size budget holds.** `bash -c 'command grep -n "gate-run" skills/docket-build/SKILL.md'` — expect zero hits. `wc -l -w skills/docket-build/SKILL.md` ≤ 380/3750 (the rewrite is roughly size-neutral; if over, trim — do not raise).

- [ ] **Step 6: Retarget the posture test's (12a)/(12b) machinery** (aside-copy first). Each edit sorted by what it GUARDS (learnings: *test-premise-deleted-not-regated*):
  - `CONTRACT="$REPO/scripts/gate-run.md"` → `CONTRACT="$REPO/skills/docket-build/references/gate-caller-loop.md"` (rename the variable if clarity wants it — e.g. `LOOP_REF` — consistently). Re-set the non-vacuity line-count anchor (`-ge 100`) against the new file's measured length (pick a floor comfortably below the actual, e.g. `-ge 60` for a ~100-line file — state the actual in the comment).
  - The **verb derivation** (`grep -oE '^gate-run\.sh --[a-z-]+'` over the contract's Usage block) → an `awk -F'|'` slice of the new file's `## The caller's verbs` table, taking each row's first-column code-span token (same table-slice shape the stop tokens use; strip non-`[a-z-]` after extracting the first backtick span). Keep the floor as a non-vacuity anchor: `[ "$n_verbs" -ge 2 ]` (the table has 3; a fourth row still reddens the per-verb loop only if the SKILL misses it — which is the property). The per-verb loop asserts each derived verb appears in `$helper_blk` as `docket gate <verb>` — shape-keyed (`grep -qE "docket gate <verb>"`), not bare-token.
  - The **helper invocation assert** (`"helper: the posture names the facade invocation"`, `grep -qE "docket\.sh gate-run"`) → removal-detecting: `grep -qE "docket gate launch" <<<"$helper_blk"` and `grep -qE "docket gate stop" <<<"$helper_blk"` **and** `! grep -qF -- 'gate-run' <<<"$helper_blk"`. Update the comment: the measured "bare `grep -F` is not enough" rigor carries over — the posture must name the invocations a worker actually runs.
  - The **helper pointer assert** (`"helper: the posture points at the helper contract for the state vocabulary"`, `grep -qF -- "gate-run.md"`) → `grep -qF -- "gate-caller-loop.md" <<<"$helper_blk"`.
  - The **stop-token derivation** (the `awk -F'|'` over `| Token | Produced when |`) → the same slice over the new file's stop mapping table, keyed on its actual header row (`| Native stop outcome |`…), extracting the first code-span token of column one (`no-op`, `applied`, `error`). Keep `[ "$n_tokens" -ge 3 ]`. The shape assert stays: each derived token must open a disposition bullet of its own in `$died_blk` (`grep -qE "^- .$t. "` — presence-anywhere was measured insufficient when this guard was written).
  - The **(12b) prose asserts** re-bind to the new vocabulary, preserving each guarded property: "already-terminal is named as the ordinary live-child stop" → `no-op` is named ordinary (`grep -qiE "no-op[^.]{0,100}ordinary|ordinary[^.]{0,100}no-op"`); "the already-terminal leg re-observes" → the `no-op` leg re-observes (`grep -qiE "no-op[^.]{0,220}observ"`); "the unavailable leg aborts WITHOUT relaunching" → the `error` leg (`grep -qiE "error[^.]{0,80}abort[^.]{0,120}(without|never|no)[^.]{0,60}relaunch"`). Unchanged (their prose survives): died-not-red/no-repair, idempotence-scoped relaunch, non-idempotent-keeps-posture, relaunch-gated-on-stop-report, second-died-aborts, the state-vs-marker rule, only-running-retryable, JSON+jq binding, `! grep state=`, hand-rolled-drift, canonical-loop pointer, all of (12c).
  - Comments naming `gate-run.md` as the vocabulary owner inside groups (12a)/(12b) retarget to the new file.
- [ ] **Step 7: Mutation-test the rewritten machinery.** Delete the verbs table's `observe` row in `gate-caller-loop.md` → the derivation's per-verb loop shrinks and the non-vacuity floor still holds, so instead verify the reverse: add a fourth row `probe` → the per-verb loop reddens ("the posture gives the '--probe'/'probe' verb a role" has no SKILL counterpart). Delete the stop mapping table's header row → `n_tokens` floor reddens. Delete the SKILL's `error` bullet → its shape assert reddens. Re-add `docket.sh gate-run` to the helper paragraph → the negative reddens. Restore everything from the aside copies; re-read to confirm.

- [ ] **Step 8: Regenerate embedded assets.** `go run ./cmd/genassets && go test ./internal/assets/`.

- [ ] **Step 9: Run the FULL suite.**

Run: `scripts/run-tests.sh`
Expected: green. `test_gate_run.sh` and `test_gate_run_stop.sh` still pass — `gate-run.sh`/`gate-run.md` still exist and are still self-tested; nothing in the posture test reads them any more.

- [ ] **Step 10: Commit.**

```bash
git add skills/docket-build/SKILL.md tests/test_gate_execution_posture.sh internal/assets
git commit -m "feat(0339): docket-build gate posture on native verbs and the stop mapping table"
```

---

### Task 4: Create `tests/test_gate_caller_loop.sh` carrying the fence harness

**Files:**
- Create: `tests/test_gate_caller_loop.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` + ledger comment), `tests/README.md` (the standalone-file count in its opening sentence)
- Read-only source: `tests/test_gate_run.sh` (the caller-loop leg: the section/fence-slicing awk, the scripted-stub arm proofs, the real-native-gate leg)

**Interfaces:**
- Consumes: Task 1's `gate-caller-loop.md` § *The caller's loop* (fence source, heading `## The caller's loop`, next heading `## State vocabulary and retryability` as the named terminator).
- Produces: standalone executable coverage of the published fence — the fence now has coverage in **two** places for two commits (here and `test_gate_run.sh`), which is the point: coverage overlaps rather than gaps until Task 6 deletes the old shard against this one.

- [ ] **Step 1: Extract the leg.** Copy from `tests/test_gate_run.sh` the entire caller-loop block: the fence-locating awk, the non-vacuity anchors ("the caller-loop section was located", "the canonical loop fence was located … -ge 15"), the prose asserts (jq documented as a required dependency; the unknown-document arm terminal, never a retry; disposition policy deferred to *Gate execution posture*; the mandatory stop on the abandon-while-running leg), the `LOOPBOX` stub-`docket` fixture with its 200-observation hard stop (learnings: *mutation-target-needs-a-forced-exit*), `run_loop()` with its simulated clock and jq/nojq PATH modes, every arm assert including the `|| true` errexit mutation key and the jq-check key, and the real-gate leg (`go build -o … ./cmd/docket`, `docket gate launch --root … --cwd … -- …`, the await-terminal helper, the terminal-state reads).

- [ ] **Step 2: Retarget and make it self-contained.** The fence source becomes `skills/docket-build/references/gate-caller-loop.md`; the section awk anchors on `/^## The caller's loop$/` and terminates on the **named** next heading `## State vocabulary and retryability`, asserting the terminator exists (learnings: *section-slice-needs-a-named-terminator*); fence-in-section extraction is unchanged. The file carries its own prologue — strict mode, `assert` helper, `REPO` resolution, `mktemp -d "${TMPDIR:-/tmp}/gate-caller-loop.XXXXXX"` sandbox (template mandatory on macOS) — and must **NOT** source `tests/lib/gate_run_common.sh`, which Task 6 deletes; the barrier machinery there belongs to the dead script and is not copied. Header comment: "guards the published caller loop in skills/docket-build/references/gate-caller-loop.md, executed byte-unmodified against scripted and real protocol-v1 documents; relocated from tests/test_gate_run.sh by change 0339."

- [ ] **Step 3: Run it, then mutation-probe the relocation.**

Run: `bash tests/test_gate_caller_loop.sh`
Expected: PASS. Then copy `gate-caller-loop.md` aside, delete the fence's `|| true` on the observe capture, re-run → the errexit-key assert (failed-document arm) reddens; restore from the aside copy, re-read to confirm, re-run → PASS.

- [ ] **Step 4: Budget row.** Measure serially: `/usr/bin/time -p bash tests/test_gate_caller_loop.sh`, three readings, worst wins. Size per the tsv's rule (next multiple of 5 above the worst reading, minimum sensible margin — expect ~15–25s: one Go build plus short real runs). Append the row (tab-separated): `tests/test_gate_caller_loop.sh<TAB><N><TAB>parallel`. Recompute `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` by **computing, never hand-adding**: `awk -F'\t' '$1 ~ /^tests\// {s += $2} END {print s}' tests/runtime-budgets.tsv`; set the constant and append a ledger comment beside it naming this as the new-file case with the three measurements. Update `tests/README.md`'s opening "123 standalone Bash files" count to the new actual (`ls tests/test_*.sh | wc -l`); it indexes no per-suite rows (verified at plan time — its shard discussion is thematic), so no row is added.

- [ ] **Step 5: Run the FULL suite.**

Run: `scripts/run-tests.sh`
Expected: green, including `test_runtime_budgets.sh` (row + file + total in one commit) and both fence-coverage shards.

- [ ] **Step 6: Commit.**

```bash
git add tests/test_gate_caller_loop.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh tests/README.md
git commit -m "test(0339): caller-loop fence guard relocated to its own shard"
```

---

### Task 5: The cross-reference sweep

**Files:**
- Modify: whatever the re-derived grep proves is still unowned — expected **none**; this task is the proof, and it commits only if it edits.

**Interfaces:**
- Consumes: Tasks 1–4. Produces: the verified ledger Task 6's deletion stands on.

- [ ] **Step 1: Re-derive the ledger.**

Run: `bash -c 'cd <worktree> && git grep -ln "gate-run\|gate_run" || true'`
Expected: every hit sorts into exactly these classes — (a) the five files Task 6 deletes; (b) the sites Task 6 edits (`scripts/docket.sh`, `scripts/docket.md`, `scripts/check-test-source-hygiene.sh` + `.md`, `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` ledger); (c) the sites Task 7 edits (`scripts/lib/docket-liveness.sh`, `scripts/runner-dispatch.sh`, `scripts/runner-dispatch.md`, `tests/test_docket_liveness.sh`); (d) frozen records and classified non-targets (Global Constraints), including the size-budget test's historical ledger and this plan; (e) deliberate historical by-name prose written by Tasks 1–3 (`gate-caller-loop.md`'s measurement record, the embedded mirror copies). Sort EVERY hit explicitly against the header ledger table; record the sorted list in the build evidence.

- [ ] **Step 2: Fix any unclassified hit.** An unclassified maintained-source hit is a finding to fix here (retarget or make deliberately historical), never to wave through. If the fix touches `skills/`, run `go run ./cmd/genassets` in the same commit. Then run the FULL suite (`scripts/run-tests.sh`, expect green) and commit:

```bash
git add <exactly the files edited>
git commit -m "docs(0339): close the gate-run cross-reference sweep"
```

If the sweep proves clean (the expected case), make **no commit** — record the derivation and verdict in the build evidence and move on.

---

### Task 6: Delete the facade, its contract, its shards, and the WRAPPED_OPS entry (one atomic commit)

**Files:**
- Delete: `scripts/gate-run.sh`, `scripts/gate-run.md`, `tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh`, `tests/lib/gate_run_common.sh`
- Modify: `scripts/docket.sh` (drop `gate-run` from the `WRAPPED_OPS` string; delete the `#   gate-run [args] …` header comment line), `scripts/docket.md` (delete the `| gate-run | gate-run.sh | …` table row), `tests/runtime-budgets.tsv` (delete the `tests/test_gate_run.sh` and `tests/test_gate_run_stop.sh` rows AND the header comment block that re-justifies the 20s row — it justifies a row that no longer exists), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` recomputed; append a ledger entry naming the two deleted rows — prior entries stay, they are history), `scripts/check-test-source-hygiene.sh` (the two comment sites listing `tests/lib/gate_run_common.sh` among the assert-helper examples — drop it from the list, "those three definitions" becomes two) and `scripts/check-test-source-hygiene.md` (same list in prose), `tests/README.md` (file count down by two)

**Interfaces:**
- Consumes: Tasks 1–5 — nothing maintained reads the deleted files any more except the sites this task edits.

- [ ] **Step 1: Confirm coverage before deleting** (learnings: *test-premise-deleted-not-regated*, *compensating-assert-must-exist-when-cited*). The shards' subject-mechanics guards die with the script; their native equivalents live in `internal/process/` (`launch_test.go`, `stop_test.go`, `observe_test.go`, `terminal_test.go`, `failure_test.go`, `no_leak_test.go`) and `internal/cli/gate_test.go`. Spot-verify: `bash -c 'command grep -n "already\|terminal\|unavail\|owner" internal/process/stop_test.go' | head` shows the already-terminal and ownership branches covered; the fence harness already lives in `tests/test_gate_caller_loop.sh` (Task 4, committed and green). If a mechanics guard has **no** native twin, stop and record it in the results file as a finding — do not port shell fixtures to Go in this change.

- [ ] **Step 2: Delete and edit.** `git rm scripts/gate-run.sh scripts/gate-run.md tests/test_gate_run.sh tests/test_gate_run_stop.sh tests/lib/gate_run_common.sh`. Make the edits in the Files list above. Recompute `EXPECTED_TOTAL` with the same awk as Task 4 Step 4; the ledger entry: "two rows re-cut to zero by deletion (change 0339 retires the gate-run facade); the caller-loop leg's coverage moved to tests/test_gate_caller_loop.sh with its own row." Update `tests/README.md`'s count to the new `ls tests/test_*.sh | wc -l` actual.

- [ ] **Step 3: Verify the facade refuses.**

Run: `bash -c 'cd <worktree> && scripts/docket.sh gate-run --launch 2>&1; echo "exit=$?"'`
Expected: the unknown-op diagnostic and a non-zero exit — the same failure shape as any unlisted op (compare `tests/test_docket_facade.sh`'s unknown-op fixture wording). No wrapper, no shim, no pointer.

- [ ] **Step 4: Run the FULL suite.**

Run: `scripts/run-tests.sh`
Expected: green. `test_docket_facade.sh`'s WRAPPED_OPS↔scripts correspondence holds because both sides of the pair left together; the runtime-budget registry holds because rows and files left together; the posture test reads only `gate-caller-loop.md` and `SKILL.md` (Tasks 2–3); the fence coverage lives in `test_gate_caller_loop.sh` (Task 4). If the facade test reddens on a surviving half of any pair, complete the deletion — never re-add an entry.

- [ ] **Step 5: Commit.**

```bash
git add -u
git commit -m "feat(0339): delete the gate-run facade; the native gate is the sole supervisor"
```

(`git add -u` stages only tracked modifications/deletions, which is exactly this task's edit set — verify with `git status --porcelain` that nothing untracked or unexpected is present before committing.)

---

### Task 7: Liveness-lib ownership prose — runner-dispatch is the sole consumer

**Files:**
- Modify: `scripts/lib/docket-liveness.sh` (comments only), `scripts/runner-dispatch.sh` (comments only), `scripts/runner-dispatch.md` (prose), `tests/test_docket_liveness.sh` (comments only — **asserts byte-unchanged**, honoring the spec's "unchanged")

**Interfaces:**
- Consumes: nothing later; last task.

- [ ] **Step 1: Rewrite the lib's ownership prose** (learnings: *shared-resource-keeps-first-owner-assumptions* — the incumbent's prose is exactly what goes stale). In `scripts/lib/docket-liveness.sh`: the header's "Sourced by scripts/gate-run.sh and scripts/runner-dispatch.sh" becomes sole-consumer prose (sourced by `scripts/runner-dispatch.sh`; never executed directly). The "ONE PREDICATE, TWO CONSUMERS" block becomes past-tense provenance: extracted by change 0284 when `gate-run.sh` (retired by change 0339; the native gate in `internal/process` owns that side now) and `runner-dispatch.sh` each carried a diverged copy — **keep the empty-token-conjunct war story**; it is why the lib exists. Every present-tense consumer claim ("gate-run.sh does, at `SPAWN_IDENT=`", "a caller for whom a false dead is cheap (gate-run.sh: one bounded relaunch)", the recorded-vs-ambient aside) goes past-tense or generic-caller. **No executable line changes** — verify: `git diff scripts/lib/docket-liveness.sh` shows every changed line starting with `#`.

- [ ] **Step 2: Same rewrite in runner-dispatch.** `scripts/runner-dispatch.sh`: the "shared with gate-run.sh (change 0284)" header comment → sole-consumer + provenance; the barrier-shape comparison and empty-token-drift comments become "the retired gate-run.sh facade (change 0339)" past-tense where the sentence is historical; the false-dead contrast comment ("gate-run.sh is right to read any non-zero as not alive") re-keys on the native gate's supervisor, anchored on an `internal/process` symbol or quoted clause, never a line number. `scripts/runner-dispatch.md`: the same three regions (the shared-predicate sentence, the "shared with `gate-run.sh`" clause, the false-dead cost comparison) likewise. Comment-only for the `.sh` — same `git diff` check as Step 1.

- [ ] **Step 3: Retouch the liveness-test comments.** `tests/test_docket_liveness.sh`: its four comment mentions of `gate-run.sh` (the header's consumer list, the rung-3 construction note, the always-0 assignment note, the non-zero-reading note) go sole-consumer/past-tense the same way. Assert bodies stay byte-identical — confirm: `git diff -U0 tests/test_docket_liveness.sh | command grep '^[+-]' | command grep -v '^[+-][+-]' | command grep -v '^[+-][[:space:]]*#'` produces nothing.

- [ ] **Step 4: Run the anchor-style guard, then the FULL suite.**

Run: `bash tests/test_comment_anchor_style.sh && scripts/run-tests.sh`
Expected: green — the new comments quote clauses or symbols, never line numbers; the liveness asserts are untouched.

- [ ] **Step 5: Commit.**

```bash
git add scripts/lib/docket-liveness.sh scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_docket_liveness.sh
git commit -m "docs(0339): liveness lib ownership prose — runner-dispatch sole consumer"
```

---

## Suite gate

docket-build's end gate runs the full suite per `finalize.test_command` (never a second copy) — by this plan's per-task rule it has already run green at every commit, so this is confirmation, not discovery. Any `SERIAL CONFIRMED OVER BUDGET:` line on `tests/test_gate_caller_loop.sh` means Task 4's row was cut too tight — re-measure serially and re-size per the table's rule, ledgering the correction.

## Self-review notes (performed at plan time)

- **Spec coverage:** revision-note defect 1 (new home) → Task 1; defect 2 (stop mapping) → Tasks 1+3; decision 1 (full retirement) → Task 6; decisions 2–3 (liveness lib) → Task 7; decisions 4–5 (new file + mapping) → Tasks 1–3; `gate-execution.md` minimal retargets → Task 2; caller migration → Task 3; cross-reference sweep → Tasks 5–6; tests section (new shard, posture retargets, deletions, budget rows, liveness unchanged) → Tasks 2–4, 6; commit-boundary rules (a)/(b) → Tasks 1–3 structurally, full-suite-per-commit globally; spec assumptions 1–10 are each carried in the task that consumes them (verbs table T1, new shard T4, disambiguation removal T3, ADR-0095 rewrite T2, a-fortiori carryover T1, spellings-from-source T1/T3, measured budget T1, by-name facade T1, README verified-no-index T4, no ADR global).
- **Green-at-every-commit audit, the twice-halted defect class:** T1 creates file+budget-row together and nothing else reads the file; T2 pairs the `gate-execution.md` edits with the only asserts that read that file ((12d) + harness negatives); T3 pairs the SKILL rewrite with the only asserts that read `SKILL.md` and with the `CONTRACT`/derivation retargets, while `test_gate_run*.sh` still self-test the untouched facade; T4 adds shard+row+total together; T6 deletes files with their rows, WRAPPED_OPS entry, and prose in one commit, after nothing else reads them; T7 is comment-only. Each task runs the full suite before its commit, so a missed pairing is caught in-task, not at the gate.
- **The 0338 boundary:** the fence and every observe sentence move byte-identical or keep their meaning; no observe-format or schema edit anywhere.
- **Name/type consistency:** `skills/docket-build/references/gate-caller-loop.md` and `tests/test_gate_caller_loop.sh` are spelled identically in Tasks 1–6 and the header; the heading set (`## The caller's loop` / `## State vocabulary and retryability` / `## The caller's verbs` / `## The stop mapping table` / `## Per-platform capability note (shell-era measurement record)`) is fixed in Task 1 and consumed verbatim by Tasks 2 (pointer), 3 (derivations), 4 (slice terminator); the token set `no-op`/`applied`/`error` is fixed in Task 1 Step 4 and consumed by Task 3 Steps 4/6, with the source-wins rule resolving any spelling drift in Task 3 Step 1.
