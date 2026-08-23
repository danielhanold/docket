<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0339 — Retire the gate-run.sh launch/liveness/stop facade now that the native Go-v1 gate is canonical (collapse the shared docket-liveness.sh seam with runner-dispatch.sh)** — `docs/changes/active/0339-retire-the-gate-run-sh-launch-liveness-stop-facade-now-that.md`
<!-- docket:backlink:end -->
# Retire the gate-run.sh Facade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the `gate-run.sh` launch/liveness/stop shell facade so the native Go-v1 gate (`docket gate launch|observe|stop`) is the sole supervisor, moving the orphaned caller guidance into `skills/docket-build/references/gate-execution.md` and leaving `scripts/lib/docket-liveness.sh` with `runner-dispatch.sh` as its sole consumer.

**Architecture:** Content-move first, retarget second, delete last — every commit boundary leaves a green tree. The caller's loop, state vocabulary, and per-platform note move into `gate-execution.md` while `gate-run.md` still exists (transient duplication is the buildable intermediate state); the docket-build SKILL and its posture test then retarget onto the native verbs; the caller-loop fence-execution guard relocates into a new test file; only then do the facade files, their test shards, and the `WRAPPED_OPS` entry die. The liveness lib keeps its code byte-identical and changes ownership prose only.

**Tech Stack:** Bash test shards under `tests/run-tests` discipline, Go (`internal/process`, `internal/cli`, `internal/app` — read-only reference), `cmd/genassets` for the embedded skill tree, jq for protocol-v1 JSON.

**Spec:** `docs/superpowers/specs/2026-08-23-retire-gate-run-facade-design.md` (synchronized metadata copy: `.docket/docs/superpowers/specs/…`; the feature tree does not carry it — read it from the metadata worktree or origin/docket).

## Global Constraints

- **No wrapper, no shim, no deprecation window** — after Task 5, `docket.sh gate-run` fails like any unknown op (spec decision 1).
- **`runner-dispatch.sh` migration is out of scope**; `scripts/lib/docket-liveness.sh` keeps its code unchanged — prose edits only (spec decisions 2–3).
- **Exact protocol-v1 field spellings are read from the code at implementation time** (`internal/app/gate.go` `GateResult`, `internal/cli/gate.go`), never restated from memory (spec § Caller migration).
- **Frozen records stay untouched:** everything under `docs/changes/archive/`, `docs/results/`, `docs/superpowers/plans/` (prior plans), `docs/superpowers/specs/` (prior specs), and `docs/adrs/` keeps its `gate-run` mentions — rewriting them falsifies history (AGENTS.md cross-reference rule).
- **Also untouched, classified non-targets:** `internal/app/evidence_ops.go` `"gate-running"` (evidence-gate reason string, unrelated); `internal/app/finalize_cleanup_test.go` / `finalize_git_test.go` "gate-run" fixture names (native gate *run dirs*, not the facade); `tests/test_docket_facade.sh:162` comment ("`gate-run` (change 0282) drifted exactly that way") — a past-tense historical claim that stays true after the deletion.
- Agent-authored sweeps run under explicit `bash -c` and verify with `command grep` / `git grep`, never the interactive shell's bare `grep` (learnings: *agent-shell-noop-reads-as-success*). Assert on effect (`git diff --stat`, match counts), never on exit 0.
- Skill/reference edits under `skills/` must be mirrored into `internal/assets/embedded/` by running `go run ./cmd/genassets` in the same commit (the embedded manifest guard reddens otherwise).
- The whole suite runs at the build gate per `finalize.test_command`; per-task verification runs the named files only.

## The grep-derived site ledger (authority for every task)

Derived by whole-repo grep at plan time (`git grep -ln 'gate-run\|gate_run'`), sorted per the house rule. Re-derive at Task 6 to prove the sweep closed.

| Class | Sites | Disposition |
|---|---|---|
| Delete | `scripts/gate-run.sh`, `scripts/gate-run.md`, `tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh`, `tests/lib/gate_run_common.sh` | Task 5, after Tasks 1–4 relocate what survives |
| Executable edit | `scripts/docket.sh` (WRAPPED_OPS + header comment), `tests/test_gate_execution_posture.sh`, `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (EXPECTED_TOTAL) | Tasks 3–5 |
| Maintained prose edit | `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-execution.md`, `scripts/docket.md` (table row), `scripts/lib/docket-liveness.sh` (comments), `scripts/runner-dispatch.sh` (comments), `scripts/runner-dispatch.md`, `tests/test_docket_liveness.sh` (comments only, asserts unchanged), `tests/test_skill_size_budgets.sh` (comment block ~lines 923–966 if its asserts or claims go stale) | Tasks 2, 4, 5 |
| Frozen / non-target | listed in Global Constraints | never touched |

---

### Task 1: Move the orphaned caller guidance into gate-execution.md

**Files:**
- Modify: `skills/docket-build/references/gate-execution.md`
- Modify: `tests/test_gate_execution_posture.sh` (retarget the two **reference-side** posture asserts this task's content move reddens — see new Step 6)
- Modify: `internal/assets/embedded/` (regenerated)
- Read-only sources: `scripts/gate-run.md` (§ *The caller's loop* lines 81–141, § *Per-platform capability note* lines 366–405), `internal/process/launch.go`, `docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md`

**Interfaces:**
- Produces: `gate-execution.md` sections `## The caller's loop` (verbatim fence), `## State vocabulary and retryability`, `## Per-platform capability note (shell-era measurement record)`, and an evidence-carryover paragraph. Task 3's test extracts the fence from this file; Task 4's SKILL rewrite and posture retarget point at these headings. **Because this task edits `gate-execution.md`, it also owns the two `test_gate_execution_posture.sh` asserts that read that file** (the `(12d)` *reference:* group — cap5 pointer ~591, mitigation invocation ~606); their retarget lands here so this task's commit boundary is green. The *helper:* asserts (~483, ~494) read `SKILL.md`, which this task does not touch, and stay with Task 4.

- [ ] **Step 1: Copy the caller's loop into gate-execution.md.** Append a new `## The caller's loop` section after `## The six required capabilities`, containing, moved from `gate-run.md` § *The caller's loop*: the intro paragraph (jq is the loop's required dependency; missing jq is a loud terminal diagnostic), the ```bash fence **byte-identical** (it already drives `docket gate observe <run-dir> --json` — a move, not a redesign), and the two follow-on paragraphs ("Never re-derive the state by hand…", "The loop RESOLVES the native spellings…"). Two retargets inside the moved prose only: the sentence naming `tests/test_gate_run.sh` as the fence's executor now names `tests/test_gate_caller_loop.sh` (created in Task 3), and any `scripts/gate-run.md` self-reference becomes a same-page section reference. Do not alter the fence's own comment lines — Task 3 executes them byte-for-byte.

- [ ] **Step 2: Add the state vocabulary and retryability rule.** A short `## State vocabulary and retryability` section: the observed protocol-v1 states (read the authoritative list from `internal/app/gate.go` — the `GateResult` state vocabulary — and say so in the section), the resolution rule (`signaled`/`vanished` resolve to `died`; `died` is never `failed`), and **"only `running` is retryable"**. Then rewrite capability 5's parenthetical (currently "…defined once in [`scripts/gate-run.md`](../../../scripts/gate-run.md)…") to point at this section instead.

- [ ] **Step 3: Move the per-platform capability note as a measurement record, with the carryover note.** Append `## Per-platform capability note (shell-era measurement record)` holding `gate-run.md` § *Per-platform capability note* with verbatim intent — the rung ladder, the measured `script(1)` rejection, the ADR-0080 and evidence-file quotes. Introduce it with the **evidence-carryover note** (spec § New home): the per-harness verdicts above were measured against `gate-run.sh`'s launch shape; the native launcher performs the same session-leader detachment and establishment handshake via `Setsid` in `internal/process/launch.go` ("a Setsid session-leader supervisor with the live lock and a handshake" — quote the clause, per the comment-anchor rule), and per ADR-0095 delivers a **real session on every platform**, so the shell rung-3 narrowing is historical context for the verdicts, not a claim about the native gate — the verdicts carry over without re-probing, and this paragraph is the recorded carryover.

- [ ] **Step 4: Rename the shipped mitigation.** In the paragraph after the capability list ("Docket ships that mitigation as `gate-run.sh`, reached through the facade as …`docket.sh gate-run` and specified by [`scripts/gate-run.md`]…"), the shipped mitigation becomes `docket gate launch` (`internal/process/launch.go`), the facade sentence is deleted, and the "specified by" pointer becomes the same-page sections above. The "runtime probe" sentence is replaced by the ADR-0095 real-session claim from Step 3, pointed at the measurement-record section.

- [ ] **Step 5: Sweep the page.** `command grep -n 'gate-run' skills/docket-build/references/gate-execution.md` — every remaining hit must be inside the shell-era measurement record or the carryover note, i.e. deliberately historical. Fix any other.

- [ ] **Step 6: Retarget the two reference-side posture asserts** (learnings: *assert-detects-removal-not-replacement*). Steps 2 and 4 above remove `gate-run.md`/`docket.sh gate-run` from the two `gate-execution.md` blocks that `tests/test_gate_execution_posture.sh` reads, so the two asserts on those blocks must retarget in this same commit or Task 1's boundary is red. These read `gate-execution.md` (`$cap5_blk` / `$mitigation_blk`), NOT the SKILL — leave the *helper:* asserts (`$helper_blk`, ~483/~494) alone; Task 4 owns those. First copy the file aside so you can mutation-restore without `git checkout` (learnings: *mutation-restore-needs-a-backup-copy*).
  - The cap5 pointer assert (~590-591, "reference: capability 5 points at the contract that owns the state vocabulary", `grep -qF -- "gate-run.md" <<<"$cap5_blk"`): Step 2 retargets capability 5's parenthetical from `gate-run.md` to the in-page `## State vocabulary and retryability` section. Rewrite as **removal-detecting**: assert cap5 names that in-page section (shape-keyed on the section's title words, e.g. `grep -qiE "state vocabulary" <<<"$cap5_blk"`) **and** `! grep -qF -- "gate-run.md" <<<"$cap5_blk"`. Update the assert message to name the in-page section as the vocabulary owner.
  - The mitigation invocation assert (~605-606, "reference: the mitigation names the facade invocation of its shipped implementation", `grep -qE "docket\.sh gate-run" <<<"$mitigation_blk"`): Step 4 makes the shipped mitigation `docket gate launch`. Rewrite to `grep -qE "docket gate launch" <<<"$mitigation_blk"` **and** the removal-detecting `! grep -qF -- 'gate-run' <<<"$mitigation_blk"` (note the `-F --`, per the AGENTS.md leading-`--` literal-safety rule). Update the assert message. Preserve the existing `mitigation_blk`-slicing awk (`/^One mitigation/`…blank-line) — Step 4's rewrite must keep the paragraph opening with `One mitigation` and unbroken by a blank line, or add a matching-anchor update here.
  - Mutation-test each: re-add the old string to `gate-execution.md` → the retargeted assert reddens; restore from the aside-copy (never `git checkout --` over uncommitted work).

- [ ] **Step 7: Regenerate embedded assets and verify.**

Run: `cd <worktree> && go run ./cmd/genassets && go test ./internal/assets/ && bash tests/test_gate_execution_posture.sh && bash tests/test_gate_run.sh`
Expected: all PASS — the two reference-side asserts are retargeted (Step 6), `SKILL.md` is untouched so the *helper:* asserts and `gate-run.md`'s `CONTRACT` anchors and the fence-executing shard still hold; duplication is the intended intermediate state.

- [ ] **Step 8: Commit** — `git add skills/docket-build/references/gate-execution.md tests/test_gate_execution_posture.sh internal/assets && git commit` (`docs(0339): move caller loop, vocabulary, and platform note into gate-execution.md`).

---

### Task 2: Migrate docket-build's gate posture to the native verbs

**Files:**
- Modify: `skills/docket-build/SKILL.md` (§ *Gate execution posture*, the "shipped implementation" passage ~lines 274–295 and the loop pointer ~line 286)
- Modify: `internal/assets/embedded/` (regenerated)
- Read-only: `internal/cli/gate.go`, `internal/app/gate.go` (exact flag and field spellings)

**Interfaces:**
- Consumes: Task 1's `gate-execution.md` section headings.
- Produces: SKILL prose Task 4's posture-test rewrite anchors on — the passage must name `docket gate launch` with `--root`/`--cwd`/`--json`, `docket gate stop <run-dir> --json`, jq, and the pointer `references/gate-execution.md` § *The caller's loop*; it must contain **no** `gate-run` spelling in any form.

- [ ] **Step 1: Read the native spellings from code.** From `internal/cli/gate.go` and `internal/app/gate.go`, record: the launch invocation shape (`docket gate launch --root <dir> --cwd <dir> --json -- <command…>` — verify flags and whether `--json` is required or default), the launch **failure** shape (what the JSON envelope carries when launch fails — the facade's `launch-failed` stdout token has no native twin; the caller reads the envelope), and the stop verdict vocabulary in the JSON result (the facade tokens `stopped`/`already-terminal`/`unavailable` — confirm the native spellings and field names, e.g. `GateResult`'s state/outcome fields). Never restate from memory.

- [ ] **Step 2: Rewrite the "shipped implementation of clauses 1–3" passage.** Replace the `docket.sh gate-run` sentence: launch is `docket gate launch --root <dir> --cwd <dir> --json -- <command…>` (spellings from Step 1), stop is `docket gate stop <run-dir> --json`. The observe sentences (already native since 0338) keep their meaning; drop "The plain-text state-name observe contract is retired; `gate-run --observe` refuses with a pointer" — the facade it describes is gone. Retarget "**Reuse the canonical loop** in `gate-run.md` § *The caller's loop*" to `references/gate-execution.md` § *The caller's loop* (same file the section already cites at its end).

- [ ] **Step 3: Rewrite "On a failed launch."** The `launch-failed` slash-free-token paragraph becomes: a failed `docket gate launch` is read from the launch's own protocol-v1 JSON envelope with jq (name the discriminating field from Step 1); the disposition is unchanged — abort-and-report per *Halting conditions*, never a retry loop, never observed.

- [ ] **Step 4: Re-key "On the died state."** The three stop-token bullets keep their exact dispositions (already-terminal → re-observe and key on the state; stopped → relaunch once; unavailable → abort loudly) but each token is named as the native stop's JSON verdict per Step 1's spellings, and `--stop` spellings become `docket gate stop <run-dir>`. "Abandoning a live child" likewise (`--stop` → `docket gate stop <run-dir>`; the fence's trailing comment in Task 1 already says this — keep the two consistent).

- [ ] **Step 5: Verify no facade spelling survives.** `command grep -n 'gate-run' skills/docket-build/SKILL.md` — expect zero hits.

- [ ] **Step 6: Regenerate assets; run the neighbor guards.**

Run: `go run ./cmd/genassets && go test ./internal/assets/ && bash tests/test_skill_size_budgets.sh && bash tests/test_gate_execution_posture.sh; echo "posture-exit=$?"`
Expected: assets and size budgets PASS (if a size-budget ceiling or its justifying comment (~lines 923–966) reddens or goes stale, adjust the comment to the new pointers — the ceilings themselves should hold since the rewrite is size-neutral). **The posture test is expected RED here** — its `docket\.sh gate-run` asserts (its own comments name the anchor: "the posture must name" the helper invocation) now fail. That red is Task 4's subject; note it, do not fix it in this commit unless the two tasks are folded — **fold decision:** to keep every commit green, perform Task 4 (posture-test rewrite) in this same commit. Treat Steps 1–5 here plus all of Task 4 as one commit.

- [ ] **Step 7: Commit** (after Task 4's steps) — `git add skills/docket-build/SKILL.md tests/test_gate_execution_posture.sh internal/assets && git commit` (`feat(0339): docket-build gate posture on native gate verbs`).

---

### Task 3: Relocate the caller-loop fence-execution guard to its own shard

**Files:**
- Create: `tests/test_gate_caller_loop.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` + ledger comment)
- Read-only source: `tests/test_gate_run.sh` (the caller-loop leg, ~lines 455–679: fence extraction, scripted-stub arm proofs, real-native-gate leg)

**Interfaces:**
- Consumes: Task 1's `gate-execution.md` § *The caller's loop* (fence source).
- Produces: a standalone shard proving the published fence's arms and real-document behavior; Task 5 deletes the old shard against this coverage.

- [ ] **Step 1: Extract the leg.** Copy from `tests/test_gate_run.sh` the entire caller-loop block — the section-slicing awk that locates `### The caller's loop` and its fence, the non-vacuity anchors ("the caller-loop section was located", "the canonical loop fence was located… -ge 15"), the scripted-stub gate (`OBS_SCRIPT` stub with its independent hard stop — learnings *mutation-target-needs-a-forced-exit*), every arm assert including the `|| true` errexit mutation key and the jq-check mutation key, and the real-gate leg (`go build -o … ./cmd/docket`, `docket gate launch --root … --cwd … --json -- …`, `await_native_terminal`, the five terminal-state reads).

- [ ] **Step 2: Retarget and make it self-contained.** The fence source becomes `skills/docket-build/references/gate-execution.md` (the section-terminator awk must name its terminator per the house guard style — anchor on the next `## ` heading Task 1 created, and assert the terminator exists). The new file carries its own prologue (strict mode, `assert` helper, `mktemp` sandbox with an explicit template `"${TMPDIR:-/tmp}/gate-caller-loop.XXXXXX"`, `REPO` resolution) copied from the old shard's usage — it must **not** source `tests/lib/gate_run_common.sh`, which Task 5 deletes; the barrier machinery there belongs to the dead script and is not copied. Header comment states what the file guards: "the published caller loop in gate-execution.md, executed byte-unmodified against scripted and real protocol-v1 documents."

- [ ] **Step 3: Run it, then mutation-probe the relocation.**

Run: `bash tests/test_gate_caller_loop.sh`
Expected: PASS. Then mutate `gate-execution.md`'s fence (delete the `|| true` on the observe capture), re-run, expect the errexit-key assert RED; restore the file from your uncommitted edit by re-applying the deletion's inverse — **never** `git checkout --` over uncommitted work (learnings: *mutation-restore-needs-a-backup-copy*: copy the file aside before mutating).

- [ ] **Step 4: Budget row.** Measure serially (`/usr/bin/time -p bash tests/test_gate_caller_loop.sh`, three readings, worst wins); size per the table's rule (next multiple of 5 above worst, plus 5s margin, min 10s — expect ~15–20s: one Go build plus five short real runs). Add the row `tests/test_gate_caller_loop.sh<TAB><N><TAB>parallel`. Recompute `EXPECTED_TOTAL` by **computing, never hand-adding**: `awk -F'\t' '$1 ~ /^tests\// {s += $2} END {print s}' tests/runtime-budgets.tsv`, set the constant, and append a ledger comment beside it naming this as the new-file case with the measurement.

- [ ] **Step 5: Verify the budget guard and commit.**

Run: `bash tests/test_runtime_budgets.sh && bash tests/test_gate_caller_loop.sh`
Expected: PASS. `git add tests/test_gate_caller_loop.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh && git commit` (`test(0339): relocate caller-loop fence guard to its own shard`).

---

### Task 4: Retarget the posture test onto the native spellings (folded into Task 2's commit)

**Files:**
- Modify: `tests/test_gate_execution_posture.sh` (~lines 449–616, the helper/contract section)

**Interfaces:**
- Consumes: Task 2's SKILL prose, Task 1's gate-execution.md sections.
- Produces: guards Task 5 relies on — after the deletion these must still pin the posture without touching `scripts/gate-run.md`.

- [ ] **Step 1: Sort each block by what it GUARDS** (learnings: *test-premise-deleted-not-regated*):
  - `CONTRACT="$REPO/scripts/gate-run.md"` and the `verbs=` extraction cross-checking the SKILL against the contract's `--launch/--stop` verb list — **premise dies with the file.** Replace with the native equivalent: assert the SKILL's helper block names `docket gate launch` and `docket gate stop` (shape-keyed: `grep -qE 'docket gate (launch|stop)'` per verb), anchored the way the existing comments demand ("A bare `grep -F gate-run` is NOT enough" — same rigor, new spelling).
  - **Scope note:** the two *reference:* asserts that read `gate-execution.md` — the cap5 pointer (~591) and the mitigation invocation (~606) — moved to **Task 1 Step 6** (they redden one commit earlier, when Task 1 edits that file). This task owns only the *helper:* asserts, which read `SKILL.md`.
  - The `docket\.sh gate-run` assert on the helper block (~483, `$helper_blk`) — rewrite as a **removal-detecting** assert (learnings: *assert-detects-removal-not-replacement*): the helper block must name the native invocation AND must **not** contain `gate-run` in any spelling (`! grep -qF -- 'gate-run' <<<"$helper_blk"` — note the `-F --`, the pattern leads with nothing but the negated form still follows the AGENTS.md leading-`--` rule for literal safety).
  - The `gate-run.md` pointer assert on the helper block (~494, `$helper_blk`) — retarget to `references/gate-execution.md` § *The caller's loop* pointers.
  - The existing native-observe asserts (~519) and gate-execution.md asserts (1–443) — premise survives, unchanged except where a comment names `gate-run.md` as the vocabulary owner (retarget the comment to the in-page section).
- [ ] **Step 2: Mutation-test each rewritten assert:** re-add the string `docket.sh gate-run` to the SKILL's helper paragraph → the negative assert reddens; delete `docket gate launch` from the paragraph → the positive assert reddens; break the `references/gate-execution.md` pointer → pointer assert reddens. Restore from the aside-copies.
- [ ] **Step 3: Run** `bash tests/test_gate_execution_posture.sh` — PASS — then commit together with Task 2 (see Task 2 Step 7).

---

### Task 5: Delete the facade, its contract, its shards, and the WRAPPED_OPS entry

**Files:**
- Delete: `scripts/gate-run.sh`, `scripts/gate-run.md`, `tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh`, `tests/lib/gate_run_common.sh`
- Modify: `scripts/docket.sh` (WRAPPED_OPS list line, `#   gate-run [args] …` header comment line), `scripts/docket.md` (the `gate-run` table row), `tests/runtime-budgets.tsv` (delete the two rows + append a ledger comment; the header's own `test_gate_run.sh STAYS at 20` comment block is superseded — delete it, it justifies a row that no longer exists), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` recomputed; append a ledger entry naming the two deleted rows — prior ledger entries stay, they are history)

**Interfaces:**
- Consumes: Tasks 1–4 (nothing maintained points at the deleted files any more except the sites this task edits).

- [ ] **Step 1: Confirm coverage before deleting** (learnings: *test-premise-deleted-not-regated* — "once you have confirmed its coverage lives elsewhere"). The two shards' subject-mechanics guards die with the script; their native equivalents are `internal/process/launch_test.go`, `stop_test.go`, `observe_test.go`, `terminal_test.go`, `failure_test.go`, `no_leak_test.go` and `internal/cli/gate_test.go`. Spot-verify the two named-in-comments claims: the `--observe` refusal comment in `test_gate_run.sh` (~line 238) itself says the behavior "…the native gate (internal/process/observe_test.go, internal/cli/gate_test.go)"; and confirm a stop-verdict test exists in `internal/process/stop_test.go` covering the already-terminal and unowned-survivor branches (`command grep -n 'already\|terminal\|unavail\|owner' internal/process/stop_test.go | head`). If a mechanics guard has **no** native twin, stop and record it in the results file as a finding — do not port shell fixtures to Go in this change.

- [ ] **Step 2: Delete and edit.** `git rm scripts/gate-run.sh scripts/gate-run.md tests/test_gate_run.sh tests/test_gate_run_stop.sh tests/lib/gate_run_common.sh`. In `scripts/docket.sh`: remove `gate-run` from the `WRAPPED_OPS` string and delete the `#   gate-run [args] …` header line. In `scripts/docket.md`: delete the `| gate-run | gate-run.sh | …` table row. In `tests/runtime-budgets.tsv`: delete the `tests/test_gate_run.sh` and `tests/test_gate_run_stop.sh` rows and the header comment block that re-justified the 20s row. Recompute `EXPECTED_TOTAL` with the same awk as Task 3 Step 4 and ledger the move ("two rows re-cut to zero by deletion, change 0339; the caller-loop leg's coverage moved to tests/test_gate_caller_loop.sh with its own row").

- [ ] **Step 3: Verify the facade refuses.**

Run: `bash -c 'cd <worktree> && DOCKET_SCRIPTS_DIR=scripts scripts/docket.sh gate-run --launch 2>&1; echo "exit=$?"'`
Expected: the unknown-op diagnostic, non-zero exit — same failure shape as any unlisted op (check `test_docket_facade.sh`'s unknown-op fixture for the expected wording).

- [ ] **Step 4: Run the neighbors.**

Run: `bash tests/test_docket_facade.sh && bash tests/test_runtime_budgets.sh && bash tests/test_gate_execution_posture.sh && bash tests/test_gate_caller_loop.sh && bash tests/test_docket_liveness.sh`
Expected: all PASS. If `test_docket_facade.sh` reddens on a WRAPPED_OPS↔scripts correspondence it is because one side of the pair survived — fix by completing the deletion, never by re-adding the entry.

- [ ] **Step 5: Commit** — `git add -u && git add tests/runtime-budgets.tsv && git commit` (`feat(0339): delete the gate-run facade; native gate is sole supervisor`). (`git add -u` here stages only tracked modifications/deletions from Step 2 — acceptable because Step 2's edit set is exactly this task's file list; verify with `git status --porcelain` before committing.)

---

### Task 6: Liveness-lib ownership prose and the closing sweep

**Files:**
- Modify: `scripts/lib/docket-liveness.sh` (comments only — lines 3, 5, 12, 30, 41, 52, 83 region), `scripts/runner-dispatch.sh` (comments at ~67–69, ~572, ~928–929, ~1286), `scripts/runner-dispatch.md` (~500, ~524, ~529), `tests/test_docket_liveness.sh` (comments at 3, 19, 65, 79 — **asserts byte-unchanged**, honoring the spec's "unchanged" while closing the prose-drift risk its Risks section names)

**Interfaces:**
- Consumes: nothing later; last task.

- [ ] **Step 1: Rewrite the lib's ownership header** (learnings: *shared-resource-keeps-first-owner-assumptions* — the incumbent's prose is exactly what goes stale). `scripts/lib/docket-liveness.sh` line 3 becomes sole-consumer prose: sourced by `scripts/runner-dispatch.sh`; never executed directly. The "ONE PREDICATE, TWO CONSUMERS" block (lines 5–12) becomes past-tense provenance: extracted by change 0284 when `gate-run.sh` (retired by change 0339, native gate `internal/process`) and `runner-dispatch.sh` each carried a diverged copy — keep the empty-token-conjunct war story, it is why the lib exists. Present-tense consumer claims (lines 30, 41, 52, 83: "gate-run.sh does, at `SPAWN_IDENT=`", "a caller for whom a false dead is cheap (gate-run.sh…)") become past-tense or generic-caller phrasing; **no executable line changes** — verify with `git diff` that every changed line starts with `#`.

- [ ] **Step 2: Same rewrite in runner-dispatch.** `scripts/runner-dispatch.sh` ~67–69 ("shared with gate-run.sh") → sole-consumer + provenance; ~572, ~928–929, ~1286 — mentions of `gate-run.sh`'s posture/copy become "the retired gate-run.sh facade (change 0339)" past-tense where the sentence is historical, or are re-keyed on the native gate where the sentence claims a present-day contrast (line ~1286's "gate-run.sh is right to read any non-zero as not alive" — the native gate's supervisor holds that posture now; re-key on it and quote `internal/process` by symbol, not line number). `scripts/runner-dispatch.md` ~500/524/529 likewise. Comment-only for the `.sh`; `git diff` check as in Step 1.

- [ ] **Step 3: Retouch the liveness-test comments.** `tests/test_docket_liveness.sh` lines 3/19/65/79: consumer lists go sole-consumer/past-tense as above. Assert bodies stay byte-identical — confirm with `git diff -U0 tests/test_docket_liveness.sh | command grep -v '^[+-]#' | command grep '^[+-]' | command grep -v '^[+-][+-]'` producing nothing.

- [ ] **Step 4: The closing whole-repo sweep** (the spec's named top risk). Re-derive the ledger:

Run: `bash -c 'cd <worktree> && git grep -ln "gate-run\|gate_run" || true'`
Expected: hits ONLY in the frozen classes (docs/changes/archive, docs/results, docs/superpowers/plans + specs, docs/adrs), the classified non-targets (`internal/app/evidence_ops.go`, `internal/app/finalize_*_test.go`, `tests/test_docket_facade.sh` history comment), deliberate historical prose written by this change (liveness/runner-dispatch/gate-execution.md provenance, budget ledgers, this plan), and `tests/runtime-budgets.tsv`'s ledger comments. Sort EVERY hit against the table in this plan's header; an unclassified hit is a finding to fix, not to wave through. Count the hits and record the sorted list in the build evidence.

- [ ] **Step 5: Run the liveness and dispatch guards.**

Run: `bash tests/test_docket_liveness.sh && bash tests/test_comment_anchor_style.sh`
Expected: PASS (the anchor-style guard checks the new comments quote clauses, not line numbers).

- [ ] **Step 6: Commit** — `git add scripts/lib/docket-liveness.sh scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_docket_liveness.sh && git commit` (`docs(0339): liveness lib ownership prose — runner-dispatch sole consumer`).

---

## Suite gate

After Task 6, docket-build runs the full suite per `finalize.test_command` (never a second copy of the command). Expected: green; the deleted rows and the new row keep `test_runtime_budgets.sh` consistent. Any `SERIAL CONFIRMED OVER BUDGET:` line on `tests/test_gate_caller_loop.sh` means Task 3's row was cut too tight — re-measure serially and re-size per the table's rule, ledgering the correction.

## Self-review notes (performed at plan time)

- Spec coverage: Retirement → Task 5; caller migration → Tasks 2+4; new home + carryover → Task 1; liveness prose → Task 6; test sort/delete → Tasks 3+5; embedded regen → Tasks 1–2. Posture-test retarget is split by the file each assert reads: the reference-side asserts (cap5 ~591, mitigation ~606, which read `gate-execution.md`) land in **Task 1 Step 6** so its content-move commit is green in isolation; the helper-side asserts (~483, ~494, which read `SKILL.md`) stay in **Task 4**, folded into Task 2's SKILL-rewrite commit. This is the amendment that cleared the 2026-08-23 halt (option b in the change's `## Run halted` marker). Spec's anticipated `docket-finalize-change`/`gate-failure.md`/`docket-convention`/`check-test-source-hygiene` retargets: the plan-time grep found **zero** `gate-run` references in any of them (the grep is the authority the spec itself names), so no task exists for them; Task 6 Step 4 re-proves it.
- The 0338 boundary is respected: no observe-format edits anywhere; the fence and observe prose move or stay byte-identical.
- Type/name consistency: `tests/test_gate_caller_loop.sh` is spelled identically in Tasks 1, 3, 5, and the suite-gate note; the gate-execution.md heading `### The caller's loop` / `## The caller's loop` — Task 1 Step 1 fixes it as `## The caller's loop` (top-level section of that page); Task 3's awk anchors on that spelling.
