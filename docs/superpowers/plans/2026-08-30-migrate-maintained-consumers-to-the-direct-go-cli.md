<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0369 — Migrate retained lifecycle consumers to typed Go operations](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-30-0369-migrate-maintained-consumers-to-the-direct-go-cli.md)**
<!-- docket:backlink:end -->
# Migrate Retained Lifecycle Consumers to Typed Go Operations — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by the **docket-build** skill,
> task-by-task, one dispatched worker per task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route the retained lifecycle consumers — the run-gate parent instructions, the planning
skills' backlink stamping, implement-next's backlog digest, the convention's fresh-repo bootstrap,
and the metadata-only finalize close-out — through the public typed Go CLI that merged change 0318
landed, and delete the one caller-owned follow-up (standalone ADR-index rendering) a Go transaction
already performs atomically.

**Architecture:** This is a CONSUMER ADAPTATION, not a capability expansion. Every edit is an
invocation/structured-output swap `docket.sh <op>` → `docket <verb>` in canonical instruction
prose, plus one Class D removal. No Go source changes except focused tests. The merged state is an
intentionally mixed, independently green transition: migrated Class A/D surfaces use Go directly
while frozen Bash paths (preflight, board, publish, dispatch, stacks) remain for 0370/0371/0372.

**Tech Stack:** Markdown skill prose (agent-executed — treat as code), Bash test files under
`tests/`, the installed/source Go CLI (`docket` / `go run ./cmd/docket`), `go generate
./internal/assets/` (cmd/genassets) for embedded mirrors.

**Spec:** `docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md`
(synchronized copy: `.docket/docs/superpowers/specs/…` on the `docket` branch). The change file's
**Reconcile log** (change 0369) carries the authoritative Class A/D/frozen inventory — do not
re-derive it.

## Global Constraints

Every task's requirements implicitly include all of these.

1. **Abort boundary (HALT, per payload and spec §Preconditions):** if any task finds a mapped
   migration requires a NEW or behaviorally expanded Go verb, a bespoke compatibility adapter
   (anything beyond invocation/structured-output adaptation), native-dispatch generator work, a
   deferred-feature retirement, or a repository-wide absence invariant — STOP the run and report
   for re-grooming. Do not smuggle partial behavior in to make a scan green. Leaving an unmapped
   *variant* of a site unchanged and recording it as a discrepancy is NOT a halt — it is the
   spec-sanctioned outcome ("Unmapped candidates are left unchanged and reported").
2. **Migrate ONLY the Class A/D surface** confirmed in the change file's Reconcile log:
   - Class A: `gate-before`→`docket run gate-before`; `gate-verdict`→`docket run gate-verdict`;
     `render-artifact-backlink`→`docket artifact backlink`; `docket-status --digest-only`→
     `docket status --json`; `cleanup-feature-branch`→`docket finalize cleanup`;
     `archive-change`→`docket finalize closeout`; `bootstrap`→`docket repository init`.
   - Class D: standalone `render-adr-index` follow-up after ADR record/reverse/supersede in
     `skills/docket-adr/SKILL.md` — removed, not replaced, after atomic-ownership proof.
3. **FROZEN — leave byte-untouched, permit in guards:** `preflight` (~22 sites),
   `docket-status --board-only` / the full orchestrator pass, `render-change-links`,
   `terminal-publish`, `mint-stub`, `mark-publish-deferred`, `render-learnings-index`,
   `runner-dispatch`, `stack-base`/`stack-children`/`stack-closeout`, `backfill-change-types`,
   `adr-checks`, and every `scripts/*.sh` file (deletion is 0370's). A file mixing frozen and
   migrated calls (terminal-close-out.md, docket-status SKILL.md) ends partially migrated — that
   is the intended, spec-blessed state.
4. **Canonical vs generated:** edit ONLY canonical sources under `skills/`, `agents/`,
   `cursor-rules/`, `AGENTS.md` (`CLAUDE.md` is a symlink to it — never write CLAUDE.md
   separately), `README.md`. `internal/assets/embedded/tree/**` is a GENERATED mirror —
   regenerate with `go generate ./internal/assets/`, never hand-edit. Managed-block marker order
   and balance are validated before any block rewrite; unrelated content preserved.
5. **Invocation spelling in skill prose:** the bare installed binary, `docket <verb> …` — the
   established pattern (`docket change claim`, `docket context implementation` already in
   docket-implement-next). Never `"${DOCKET_SCRIPTS_DIR…}"/docket.sh` for a migrated op, never a
   `go run` spelling in skill prose. Repository *tests* invoke the CLI from source
   (`go run ./cmd/docket …` or a binary built from the checkout), never the installed binary.
6. **Failure posture:** migrated callers distinguish domain outcomes from process failure, reject
   malformed/incomplete machine output, preserve waiting/halted/conflict/continuation/retry
   semantics, propagate failure with NO Bash fallback, and stop caller-owned secondary mutations
   after a failed transaction.
7. **Tests:** TDD — update/extend the sentinel test first, watch it fail against the unedited
   prose, then edit. Mutation-test every guard (strip the guarded thing, watch it redden), always
   with `-count=1` for Go (`cached-runner-serves-a-mutated-tree`) and with a backup copy of any
   file you mutate — `cp file file.bak && … && mv file.bak file`, never `git checkout -- file`
   over uncommitted work (`mutation-restore-needs-a-backup-copy`). Shell guard patterns: bound
   both sides of every token (`byte-pattern-guard-matches-a-spelling`); `grep -E -e` for
   patterns leading with `--`; never `producer | grep -q` under pipefail; re-check any regex
   under `/usr/bin/grep` (PATH grep is ugrep); no `{0,N}` intervals over 255.
8. **Before deleting any prose, grep `tests/` for the exact phrases being removed**
   (`restatement-accumulates-its-own-guards`) — repoint dependent asserts at the surviving
   canonical content; never re-add deleted text to keep a grep green, never weaken a frozen
   surface's coverage.
9. **Preserve per-caller variance** when rewriting shared references
   (`consolidation-flattens-caller-variance`): terminal-close-out.md's posture table
   (abort-and-report vs log-and-continue vs best-effort) must survive byte-for-byte in meaning.
10. **Suite gate** (final, run by docket-build once): `go run ./cmd/docket development test`.
    `scripts/run-tests.sh` is the frozen parity oracle, not the gate. A new test file needs a
    `tests/runtime-budgets.tsv` row and the `EXPECTED_TOTAL` adjustment in
    `tests/test_runtime_budgets.sh` — that adjustment is the registry's own sanctioned
    registration procedure, not an evasion.
11. **Metadata worktree discipline:** this plan writes NOTHING in `.docket/` — all work is on the
    feature branch `refactor/migrate-maintained-consumers-to-the-direct-go-cli`. Stage by
    explicit path, never `git add -A`.

## File Structure

| File | Role in this change |
|---|---|
| `cursor-rules/run-gate.md` | Canonical single source of the parent run-gate block (change 0242) — Class A gate swap |
| `AGENTS.md` (symlink target of `CLAUDE.md`) | This repo's committed copy of the run-gate block — must stay in step with the single source |
| `skills/docket-new-change/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-groom-next/SKILL.md` | Planning family — `render-artifact-backlink` → `docket artifact backlink` |
| `skills/docket-implement-next/SKILL.md` | Implementation family — digest → `docket status --json` |
| `skills/docket-convention/SKILL.md` | CREATE_ORPHAN path — `bootstrap` → `docket repository init` |
| `skills/docket-convention/references/terminal-close-out.md` | Finalize family — closeout/cleanup/backlink swaps; publish/board/links frozen |
| `skills/docket-adr/SKILL.md` | ADR transactions to Go verbs + Class D index-render removal |
| `internal/app/adr_ops_test.go` | Absorption-proof fill-in (only if an assertion is missing) |
| `tests/test_go_consumer_migration_guard.sh` (new) | Stage-local mutation-tested guard over the migrated Class A/D surface |
| `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` | Register the new test file |
| `tests/test_sync_agents_run_gate.sh`, `tests/test_cursor_dispatch_rule.sh`, `tests/test_sync_agents_claude_surface.sh`, `tests/test_skill_facade_wiring.sh`, `tests/test_artifact_backlink_coverage.sh`, `tests/test_closeout.sh`, `tests/test_results_artifact.sh`, plus whatever the Task-level grep finds | Sentinel repoints (old spelling → new, absence asserts) |
| `internal/assets/embedded/tree/**` | Regenerated mirror (never hand-edited) |

---

### Task 1: Verify every mapped public command exists and is behaviorally equivalent

**Files:**
- Read: `scripts/gate-before.sh`, `scripts/gate-verdict.sh`, `scripts/verify-run.sh`,
  `scripts/docket.sh` (the `WRAPPED_OPS` dispatch), `internal/app/rungate_before.go`,
  `internal/app/rungate_verdict.go`, `internal/app/artifact_backlink.go`,
  `internal/app/finalize_closeout.go`, `internal/app/finalize_cleanup.go`,
  `internal/app/adr_ops.go`, the `repository init` implementation (locate via
  `grep -rn "repository init\|RepositoryInit" internal/cli internal/app`)
- Produce: a verification record (table in this task's commit message and carried into the
  build-evidence/results notes) pinning every mapped verb's exact invocation, plus the
  classification table from the change file's Reconcile log (satisfies acceptance criteria 1–2).
- No production code changes. A scratch fixture under
  `"${TMPDIR:-/tmp}/docket-0369-verify.XXXXXX"` (mktemp WITH template — bare mktemp ignores
  TMPDIR on macOS).

**Interfaces:**
- Produces for later tasks: the pinned invocation exemplars (below) — later tasks copy them
  verbatim; if verification lands on a different flag shape, this task's record is authoritative
  and later tasks follow it.

**PARTICULAR RISK — the gate pair.** The dispatch payload flags this as the primary build-time
risk: `docket run gate-before`/`run gate-verdict` must reproduce the Bash facade's attribution
key, unattributed fallback, and retry-once accounting EXACTLY. If the straight swap turns out to
need a bespoke adapter, **HALT (Global Constraint 1)**.

- [ ] **Step 1: Prove the Bash gate facade owns no behavior.** Read `scripts/gate-before.sh` and
  `scripts/gate-verdict.sh` end to end (they are ~16/18 lines). Expected finding (verified at plan
  time): each is a change-0334 thin delegator — `exec "$DOCKET_BIN" run gate-before "$@"` /
  `exec "$DOCKET_BIN" run gate-verdict "$@"` with `DOCKET_BIN="${DOCKET_BIN:-docket}"` — stdout
  and exit code pass through untouched. Then read `scripts/docket.sh`'s case arm for
  `gate-before`/`gate-verdict` and confirm the facade dispatcher adds no preflight, env mutation,
  or output rewriting around them. If either wrapper owns ANY behavior beyond argv forwarding,
  the swap is not straight — HALT with the diff in the report.
- [ ] **Step 2: Confirm the Go pair is the attribution owner.** Read
  `internal/app/rungate_before.go` (arming: `gate-armed <key>` / `gate-unarmed <reason>`) and
  `internal/app/rungate_verdict.go` (attributed mode with durable-record attribution and atomic
  one-retry accounting; observe-only `--unattributed [<id>…]`). Confirm `scripts/verify-run.sh`
  reaches the same store through the same `DOCKET_BIN` seam. Record the report-line vocabulary
  (`gate-armed`, `gate-unarmed`, `gate-retry-once`, `gate-stop`, `gate-observe`, `gate-done`) —
  the run-gate prose in Task 2 must keep using exactly these tokens.
- [ ] **Step 3: Execute the gate pair against a fixture.** In a scratch git repo (init, one
  commit; if the ops require docket metadata, reuse the fixture recipe from
  `tests/test_gate_facade.sh` — read that file and copy its setup verbatim):
  run `docket run gate-before implement-next`, assert stdout is `gate-armed <key>` or a
  `gate-unarmed <reason>` line with exit 0; run `docket run gate-verdict --unattributed`, assert
  `gate-observe` lines. Also run each once with `--json` and confirm a protocol-v1 envelope.
- [ ] **Step 4: Pin the backlink exemplar for metadata-tree artifacts.** The Bash form was
  `docket.sh render-artifact-backlink --artifact-file .docket/<spec-path> --change-file
  .docket/<changes_dir>/active/<id>-<slug>.md`. The Go op (`internal/app/artifact_backlink.go`)
  takes `--repo-dir <worktree the artifact lives in>` and canonical repo-relative `--artifact` /
  `--change`, refusing absolute paths, `..` escapes, symlink escapes, and malformed markers.
  Candidate exemplar: `docket artifact backlink --repo-dir .docket --artifact <spec-path>
  --change <changes_dir>/active/<id>-<slug>.md`. Verify by executing in a fixture with a
  metadata-worktree-shaped layout (an artifact file carrying an empty/absent `docket:backlink`
  block and a change file): the block is stamped idempotently (run twice, byte-identical file).
  If the op cannot stamp an artifact in the metadata worktree without new capability — HALT.
  Record the working exemplar; Tasks 3, 5 copy it verbatim.
- [ ] **Step 5: Pin the status JSON contract.** Run `docket status --json --repo-dir .` in the
  docket repo and assert (verified at plan time, re-verify): top-level keys include
  `protocol_version`, `operation`, `result`, `ready` (array of ids in selection order),
  `changes` (objects with `id`, `status`, `readiness`, `readiness_reason`, `slug`), `records`,
  `findings`, `summary`. Confirm `readiness_reason` carries the skip vocabulary the
  implement-next prose names (needs-brainstorm, dependency waits) or record the actual
  vocabulary for Task 4's rewrite. Confirm the read is write-free (`git -C .docket status
  --porcelain` unchanged, no board write).
- [ ] **Step 6: Pin the finalize pair.** Read `internal/app/finalize_closeout.go` and
  `internal/app/finalize_cleanup.go` plus their `_test.go`/`_integration_test.go` neighbors.
  Answer and record precisely:
  (a) which outcomes `finalize closeout --id <id> [--input <notes>]` covers — help text says
  "Close out a merged change: mark done and archive, mark stacked-merged, or carry a stack
  root"; if the `killed` outcome is NOT covered, the kill drivers' `archive-change
  --outcome killed` legs are an unmapped VARIANT: they stay frozen and are recorded as a
  discrepancy (NOT a halt);
  (b) exactly which secondary mutations the closeout transaction owns atomically (terminal-date
  computation, archive move, board render, `## Artifacts` re-render, backlink restamp?) — Task
  5's rewrite keeps every frozen step the transaction does NOT own and must not double-run any
  step it DOES own;
  (c) `finalize cleanup --id <id>`'s ownership proof vs the Bash `--slug` guard (only
  `.worktrees/<slug>` removal, never `.docket/`) — semantics must be equal or stricter.
- [ ] **Step 7: Pin `repository init` vs `docket.sh bootstrap`.** Read the Go implementation and
  confirm it performs the guarded CREATE_ORPHAN creation (orphan `docket` branch + persistent
  `.docket` worktree, plumbing-built, push) that `docket-config.sh --bootstrap` performed, with
  fail-closed refusals on a non-fresh repo. Equal-or-stricter refusal surface required;
  anything weaker is a bespoke-adapter smell — HALT.
- [ ] **Step 8: Pin the ADR transactions and their index ownership.** Read
  `internal/app/adr_ops.go`: confirm (verified at plan time via doc comments) the record /
  supersede / reverse transactions each atomically land the ADR file, the re-rendered
  `<adrs_dir>/README.md` index ("ADR index is rerendered on every ADR operation,
  unconditionally"), and — when a producing change is supplied — the change's `adrs:` append,
  in ONE metadata commit. Record the JSON request schema for `docket adr record --request -`
  (and supersede/reverse) by reading the request-decoding code and one existing test.
- [ ] **Step 9: Record the verification table and the inventory classification** (Class A rows
  with old→new spelling and file list, the Class D row, the frozen list with reasons — copy from
  the change file's Reconcile log, adding this task's findings such as the killed-outcome
  disposition). Commit the record:

```bash
git add docs/superpowers/plans/2026-08-30-migrate-maintained-consumers-to-the-direct-go-cli.md
git commit -m "docs(0369): Task 1 — mapped-verb verification record and inventory classification" \
  -m "<the table>"
```

  (If nothing in the plan file itself changed, land the table in the commit message of an empty
  `--allow-empty` commit or append a short `## Task 1 findings` section to this plan file — the
  build-evidence record must carry it either way.)

---

### Task 2: Run-gate family — parent instructions to `docket run gate-before` / `gate-verdict`

**Files:**
- Modify: `cursor-rules/run-gate.md` (the single source), `AGENTS.md` (its committed twin in this
  repo — `CLAUDE.md` is a symlink; do not write it separately)
- Test-first: `tests/test_sync_agents_run_gate.sh`; also grep-and-repoint
  `tests/test_cursor_dispatch_rule.sh`, `tests/test_sync_agents_claude_surface.sh`,
  `tests/test_gate_facade.sh` (expected: gate_facade tests the frozen Bash wrappers and needs NO
  change — touching it would weaken a 0370-owned surface)

**Interfaces:**
- Consumes: Task 1's confirmation that the swap is straight and the report-line vocabulary.
- Produces: the new gate-block text below; Task 7's guard asserts its presence.

**Frozen boundary for this task:** `scripts/gate-before.sh`, `scripts/gate-verdict.sh`,
`scripts/docket.sh`, `scripts/verify-run.sh` and their tests stay byte-untouched. The
`docket:dispatch` managed block and `runner-dispatch` remain 0371's. If the rendering pipeline
(`sync-agents.sh`) would need code changes to carry the new text — that is generator work: HALT.

- [ ] **Step 1: Inventory dependents.** Run
  `grep -rn -e "gate-before" -e "gate-verdict" -e "DOCKET_SCRIPTS_DIR" tests/ cursor-rules/ AGENTS.md`
  and list every assert that pins the OLD spelling in the two files being edited. (Known at plan
  time: `tests/test_sync_agents_run_gate.sh` asserts the payload "arms the gate before dispatch"
  via the `gate-before implement-next` line and renders the single source verbatim into consumer
  AGENTS.md blocks.)
- [ ] **Step 2: Write the failing sentinel updates.** In `tests/test_sync_agents_run_gate.sh`,
  repoint the positive asserts to the new spelling — e.g. the arm assert becomes (adapt to the
  file's own `assert` helper style):

```bash
assert "payload arms the gate with the Go verb before dispatch" \
  'grep -qF "docket run gate-before implement-next" "$GATE_SRC"'
assert "payload reads the verdict with the Go verb" \
  'grep -qF "docket run gate-verdict" "$GATE_SRC"'
# assert-detects-removal-not-replacement: the retired facade spelling must be GONE from the source
assert "retired facade gate spelling is gone from the gate source" \
  '! grep -E -e "docket\.sh[[:space:]]+gate-(before|verdict)" "$GATE_SRC"'
```

  Keep every existing NEGATIVE assert about the pre-0334 hand-executed procedure
  (`DISPATCH_EPOCH`, `--with-claimed-at`, detached-dispatch headings) — they guard a different
  removal and stay.
- [ ] **Step 3: Run it to verify the new asserts fail** against the unedited prose:
  `bash tests/test_sync_agents_run_gate.sh` → the new positive asserts are NOT OK, the absence
  assert is NOT OK.
- [ ] **Step 4: Edit `cursor-rules/run-gate.md`.** Replace the two facade invocations:
  - `` `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-before implement-next` ``
    → `` `docket run gate-before implement-next` ``
  - both `gate-verdict` spellings → `` `docket run gate-verdict <key>` `` and
    `` `docket run gate-verdict --unattributed` `` (trailing hint ids unchanged).
  Rewrite the "Docket's helper facade is not on `PATH`: run each command below verbatim,
  expansion included." sentence to the binary reality, preserving the no-fallback posture, e.g.:
  "The `docket` binary is on `PATH`; run each command below verbatim. If the command is not
  found, the install is broken — stop and surface it; never reconstruct the gate by hand."
  Every other sentence keeps its exact semantics: keep the key in your notes; keyless
  `--unattributed` fallback can never authorize a re-dispatch; obey the `gate-*` report line,
  never the exit code; only `gate-retry-once` authorizes one same-id re-dispatch; `gate-stop`/
  `gate-observe` forbid re-dispatch; never hand-reimplement attribution.
- [ ] **Step 5: Mirror the block into `AGENTS.md`.** Apply the same textual change to the
  "## Run gate" section so the two stay in step exactly as they are today (same sentences,
  AGENTS.md's numbered-list formatting preserved). Do NOT touch any other AGENTS.md section.
- [ ] **Step 6: Run the affected shards:**
  `bash tests/test_sync_agents_run_gate.sh && bash tests/test_cursor_dispatch_rule.sh && bash tests/test_sync_agents_claude_surface.sh && bash tests/test_gate_facade.sh`
  → all PASS. If cursor_dispatch_rule or claude_surface pin the old spelling, repoint them the
  same way (relocation, not restoration).
- [ ] **Step 7: Regenerate the embedded mirror and prove correspondence:**
  `go generate ./internal/assets/ && go test ./internal/assets/ -count=1` → PASS
  (`internal/assets/embedded/tree/cursor-rules/run-gate.md` picks up the new text).
- [ ] **Step 8: Commit** (explicit paths):

```bash
git add cursor-rules/run-gate.md AGENTS.md tests/test_sync_agents_run_gate.sh \
  internal/assets/embedded/tree/cursor-rules/run-gate.md
# plus any other sentinel files repointed in steps 2/6
git commit -m "refactor(0369): run-gate parent instructions invoke docket run gate-before/gate-verdict"
```

---

### Task 3: Planning family — `render-artifact-backlink` → `docket artifact backlink`

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (step 2 backlink stamp),
  `skills/docket-auto-groom/SKILL.md` (Spec-exit backlink stamp),
  `skills/docket-groom-next/SKILL.md` (Spec-exit backlink stamp)
- Test-first: `tests/test_skill_facade_wiring.sh` (per-file op inventory),
  `tests/test_artifact_backlink_coverage.sh`; grep-and-repoint any other dependent

**Interfaces:**
- Consumes: Task 1 Step 4's pinned exemplar (below assumes
  `--repo-dir .docket --artifact <spec-path> --change <changes_dir>/active/<id>-<slug>.md`;
  follow Task 1's record if it differs).
- Produces: the migrated invocation text Task 7's guard floors on.

**Frozen boundary for this task:** the adjacent `docket.sh render-change-links` calls (the
`## Artifacts` writer — no Go verb), `docket.sh preflight`, `docket.sh docket-status
--board-only --must-land`, and every push/CAS instruction stay byte-untouched in all three files.
Only the `render-artifact-backlink` invocations move.

- [ ] **Step 1: Inventory dependents:** `grep -rn "render-artifact-backlink" tests/` — list which
  asserts pin the spelling inside the three files above (vs. inside frozen files like
  terminal-close-out.md, whose site Task 5 owns, or `scripts/`, which stay). Read
  `tests/test_skill_facade_wiring.sh`'s SOUND-GUARD header before editing: its Layer-1 sweep
  keys on the `${DOCKET_SCRIPTS_DIR` invocation prefix, so the new bare `docket artifact
  backlink` spelling is inherently permitted; what must change is its per-file **op inventory**
  (drop `render-artifact-backlink` from these three files' expected op sets).
- [ ] **Step 2: Write the failing asserts.** In `tests/test_artifact_backlink_coverage.sh` (or
  the facade-wiring file if inventory lives there), for each of the three skills add:

```bash
assert "docket-new-change stamps the spec backlink via the Go verb" \
  'grep -qF "docket artifact backlink" "$REPO/skills/docket-new-change/SKILL.md"'
assert "docket-new-change facade backlink spelling retired" \
  '! grep -E -e "docket\.sh[[:space:]]+render-artifact-backlink" "$REPO/skills/docket-new-change/SKILL.md"'
```

  (repeat for docket-auto-groom, docket-groom-next). Run the file — new asserts FAIL.
- [ ] **Step 3: Edit the three skills.** Replace each
  `` `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-artifact-backlink --artifact-file .docket/<spec-path> --change-file .docket/<changes_dir>/active/<id>-<slug>.md` ``
  with the Task-1 exemplar, e.g.
  `` `docket artifact backlink --repo-dir .docket --artifact <spec-path> --change <changes_dir>/active/<id>-<slug>.md` ``.
  Keep the surrounding semantics verbatim: the edit rides the same spec-write commit; the
  operation is the sole writer of the `docket:backlink` block; a typed refusal
  (malformed markers, missing artifact) leaves the file untouched — surface it, do not hand-edit
  the block (no-fallback posture).
- [ ] **Step 4: Run the shards:** `bash tests/test_skill_facade_wiring.sh &&
  bash tests/test_artifact_backlink_coverage.sh` → PASS. Fix any inventory assert the sweep
  emits for the three files (drop the migrated op from the expected set — never widen a frozen
  file's set).
- [ ] **Step 5: Regenerate mirrors:** `go generate ./internal/assets/ &&
  go test ./internal/assets/ -count=1` → PASS.
- [ ] **Step 6: Commit:**

```bash
git add skills/docket-new-change/SKILL.md skills/docket-auto-groom/SKILL.md \
  skills/docket-groom-next/SKILL.md tests/test_skill_facade_wiring.sh \
  tests/test_artifact_backlink_coverage.sh \
  internal/assets/embedded/tree/skills/docket-new-change/SKILL.md \
  internal/assets/embedded/tree/skills/docket-auto-groom/SKILL.md \
  internal/assets/embedded/tree/skills/docket-groom-next/SKILL.md
git commit -m "refactor(0369): planning skills stamp backlinks via docket artifact backlink"
```

---

### Task 4: Implementation/maintenance family — digest to `docket status --json`, bootstrap to `docket repository init`

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 1 acquisition prose, currently the
  `docket.sh docket-status --digest-only` paragraph and the exit-status paragraph after it),
  `skills/docket-convention/SKILL.md` (Step-0 preamble item 3: CREATE_ORPHAN path)
- Test-first: `tests/test_skill_facade_wiring.sh` (CREATE_ORPHAN routing assert + op
  inventories); grep-and-repoint dependents of the removed phrases

**Interfaces:**
- Consumes: Task 1 Step 5's JSON contract, Step 7's repository-init verification.
- Produces: migrated invocations Task 7 floors on.

**Frozen boundary for this task:** `docket.sh preflight` everywhere (including the CREATE_ORPHAN
path's "then re-run `docket.sh preflight`"), `docket.sh docket-status --board-only` (best-effort
mid-build refresh and reconcile-kill board pass in docket-implement-next), the docket-status
skill's full orchestrator invocation `docket.sh docket-status [--board-only]` (sweep + board are
Bash-owned until their owners land), and `README.md`'s `docket.sh docket-status --digest-only
--type untyped` example (coupled to the frozen one-off `backfill-change-types` workflow). Two
recorded discrepancies, no edits: (a) `skills/docket-status/SKILL.md` carries NO standalone
`--digest-only` call site (its digest lines are the frozen orchestrator's own output) — the
payload names the file; record the no-op; (b) the README example stays and the Task 7 guard must
permit both.

- [ ] **Step 1: Inventory dependents:** `grep -rn -e "digest-only" -e "docket.sh bootstrap" -e "ready line" tests/`
  — list asserts pinning the old acquisition prose or the bootstrap routing. (Known: the
  facade-wiring test asserts "the CREATE_ORPHAN path routes through the `docket.sh bootstrap`
  facade verb".)
- [ ] **Step 2: Write the failing asserts.** In `tests/test_skill_facade_wiring.sh`: flip the
  CREATE_ORPHAN assert so it detects the retired routing rather than merely confirming new
  wording (`assert-detects-removal-not-replacement`):

```bash
assert "CREATE_ORPHAN routes through docket repository init" \
  'grep -qF "docket repository init" "$REPO/skills/docket-convention/SKILL.md"'
assert "retired bootstrap facade routing is gone from the convention" \
  '! grep -E -e "docket\.sh[[:space:]]+bootstrap" "$REPO/skills/docket-convention/SKILL.md"'
assert "implement-next acquires the digest via docket status --json" \
  'grep -qF "docket status --json" "$REPO/skills/docket-implement-next/SKILL.md"'
assert "retired digest-only facade spelling is gone from implement-next" \
  '! grep -E -e "docket\.sh[[:space:]]+docket-status[[:space:]]+--digest-only" "$REPO/skills/docket-implement-next/SKILL.md"'
```

  Run — the four FAIL. Note the negated greps use `-E -e` (pattern context) and the whole-file
  scope is safe here because the remaining `docket-status --board-only` spellings do not match
  `--digest-only` and `repository init` does not collide with `preflight`.
- [ ] **Step 3: Edit `skills/docket-convention/SKILL.md`** Step-0 item 3: `CREATE_ORPHAN (fresh
  repo, once, human-attended) → run `docket repository init`, then re-run `docket.sh preflight`.`
  Keep the verdict vocabulary, the STOP_MIGRATE arm, and the preflight re-run untouched.
  Check the 2×2 bootstrap-guard section further down: sentences describing the *verdict* stay;
  only an executable `docket.sh bootstrap` instruction (if any besides item 3) migrates.
- [ ] **Step 4: Edit `skills/docket-implement-next/SKILL.md`** Step 1 acquisition:
  - Invocation: `` `docket status --json` `` — still a write-free read, still only after Step
    0's dispatch + re-sync (snapshot caveat preserved).
  - Channel: replace the `ready <id> …` report-line reading with the JSON envelope: validate
    `protocol_version: 1` and `operation: "status"`; the top-level `ready` array IS the
    build-ready queue in the convention's deterministic order (final tie-break lowest id
    unchanged); per-change skip reasons come from `changes[].readiness` /
    `changes[].readiness_reason` (use the vocabulary recorded in Task 1 Step 5).
  - Outcome mapping (preserve every disposition): non-zero exit or unparseable stdout → hard
    error → surface stderr/diagnostic and STOP with the `halted` disposition, never a fallback
    walk of `active/`, never `drained`. Exit 0 with `ready: []` → genuinely empty → `drained`.
    A parseable envelope MISSING the `ready` key is malformed machine output → reject → `halted`
    (this retires the old "no `ready` line → walk `active/` yourself" degradation branch, which
    existed for pre-queue Bash renders; the typed contract always carries `ready`. Record this
    as a sanctioned structured-output adaptation in the results notes.)
  - Leave untouched: the "accelerator, not the sole channel" paragraph and everything about
    `docket context implementation`, `docket change claim`, and the frozen board passes.
- [ ] **Step 5: Run the shards:** `bash tests/test_skill_facade_wiring.sh` plus every file
  Step 1's grep flagged (expect at minimum `tests/test_docket_status.sh` or
  `tests/test_typed_changes_docs.sh` if they quote the acquisition prose — repoint by
  relocation) → PASS.
- [ ] **Step 6: Regenerate mirrors:** `go generate ./internal/assets/ &&
  go test ./internal/assets/ -count=1` → PASS.
- [ ] **Step 7: Commit:**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-convention/SKILL.md \
  tests/test_skill_facade_wiring.sh \
  internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md \
  internal/assets/embedded/tree/skills/docket-convention/SKILL.md
# plus repointed sentinel files
git commit -m "refactor(0369): implement-next digest via docket status --json; CREATE_ORPHAN via docket repository init"
```

---

### Task 5: Metadata-only finalize — closeout, cleanup, and the archived-spec backlink

**Files:**
- Modify: `skills/docket-convention/references/terminal-close-out.md`
- Test-first: grep-and-repoint dependents (`grep -rn -e "archive-change" -e "cleanup-feature-branch" tests/`
  — known candidates: `tests/test_closeout.sh`, `tests/test_results_artifact.sh`,
  `tests/test_docket_status.sh`; asserts over `scripts/archive-change.sh` /
  `scripts/cleanup-feature-branch.sh` themselves are frozen-surface coverage and stay)

**Interfaces:**
- Consumes: Task 1 Step 6's record — outcome coverage, atomic ownership set, flags — and Step
  4's backlink exemplar.
- Produces: migrated step text Task 7 floors on.

**Frozen boundary for this task:** step 2's `render-change-links`, all of step 3
(`terminal-publish`, `mark-publish-deferred`), step 5's board pass, the determinism-invariant
`docket.sh preflight` mention, and — per Task 1's finding — any `killed`-outcome leg
`finalize closeout` does not cover (in that case the kill drivers' step-1 command keeps the
`docket.sh archive-change … --outcome killed` block, explicitly labeled frozen-for-kill, and the
done drivers get the Go verb; record the split as a discrepancy). The per-caller failure-posture
table's MEANING is untouchable (`consolidation-flattens-caller-variance`): sweeps stay
log-and-continue, finalize stays abort-and-report, both kill callers keep their own postures.

- [ ] **Step 1: Inventory dependents** with the grep above; list which asserts quote the step-1/
  step-4 command blocks or their surrounding sentences.
- [ ] **Step 2: Write the failing asserts** (in the file that already covers close-out prose, or
  the Task 7 guard file if none does — prefer extending the topical shard):

```bash
assert "close-out archives the done path via docket finalize closeout" \
  'grep -qF "docket finalize closeout --id" "$REPO/skills/docket-convention/references/terminal-close-out.md"'
assert "close-out cleans up via docket finalize cleanup" \
  'grep -qF "docket finalize cleanup --id" "$REPO/skills/docket-convention/references/terminal-close-out.md"'
assert "retired cleanup facade spelling is gone from close-out" \
  '! grep -E -e "docket\.sh[[:space:]]+cleanup-feature-branch" "$REPO/skills/docket-convention/references/terminal-close-out.md"'
```

  Run — FAIL. (No whole-file absence assert for `archive-change` — the kill contingency may
  legitimately keep it; the Task 7 guard slices the done-path step instead.)
- [ ] **Step 3: Rewrite step 1** per Task 1's record. Done drivers:
  `` `docket finalize closeout --id <id> [--input <notes.json>]` `` — keep/adapt the surrounding
  rules the transaction still needs from the caller (UTC merge-date sourcing moves INSIDE the
  transaction if Task 1 found it computed there — then delete the caller's date computation
  sentence for the done path; otherwise keep it), keep "trust the exit code /
  idempotent-if-already-archived" in typed-outcome form: a typed refusal or process failure
  aborts per the caller's posture, with no partial caller-owned follow-up. Apply Task 1's
  kill-leg disposition (swap if covered; frozen labeled block if not).
- [ ] **Step 4: Rewrite step 2's backlink restamp** to the Go verb with the archived change path:
  `` `docket artifact backlink --repo-dir .docket --artifact <spec-path> --change <changes_dir>/archive/<UTC-date>-<id>-<slug>.md` ``
  (exemplar per Task 1). `render-change-links` in the same step stays Bash. If Task 1 Step 6(b)
  found closeout already restamps the spec backlink atomically, DELETE the caller step for the
  done path instead (a second Class D absorption — cite the transaction test that proves it) and
  keep it only for legs closeout does not drive.
- [ ] **Step 5: Rewrite step 4** to `` `docket finalize cleanup --id <id>` ``, moving the
  provenance sentence to the Go op's ownership proof ("only workspaces proven owned by the
  terminal change are removed — never `.docket/` or out-of-tree paths"), trusting the typed
  outcome.
- [ ] **Step 6: Re-read the whole rewritten file against the four callers** (finalize-change,
  status sweep, reconcile-kill, proposed-kill) and confirm each caller's posture and the
  skip-publish guard sentences read exactly as before for the frozen steps. Run the Step-1/2
  asserts and every repointed shard → PASS.
- [ ] **Step 7: Regenerate mirrors:** `go generate ./internal/assets/ &&
  go test ./internal/assets/ -count=1` → PASS.
- [ ] **Step 8: Commit:**

```bash
git add skills/docket-convention/references/terminal-close-out.md \
  internal/assets/embedded/tree/skills/docket-convention/references/terminal-close-out.md
# plus the test files edited in steps 1–2
git commit -m "refactor(0369): terminal close-out drives done-path archive/cleanup through docket finalize"
```

---

### Task 6: Class D — docket-adr on the Go transactions, redundant index render removed

**Files:**
- Modify: `skills/docket-adr/SKILL.md`
- Test (absorption proof): `internal/app/adr_ops_test.go` — READ first; add a focused assertion
  ONLY if one of the three transactions lacks an index-atomicity assert (fill, never redesign)
- Test-first (prose): grep-and-repoint `tests/test_render_adr_index.sh` and any other dependent
  of the deleted follow-up prose

**Interfaces:**
- Consumes: Task 1 Step 8's transaction/JSON-request record.
- Produces: the `docket adr record|supersede|reverse` invocations Task 7 floors on; the
  absorption proof acceptance criterion 7 requires.

**Frozen boundary for this task:** `terminal-publish` invocations (standalone-ADR and
status-change re-publish — Class C, 0372), `adr-checks` (no verb), the manual "Update note"
body-append flow (no verb), and `scripts/render-adr-index.sh` itself. The Index/validate
section's STANDALONE stale-index repair render is an unmapped candidate: it stays, rewritten to
be explicitly repair-only (never a follow-up to a transaction) — recorded as a discrepancy the
guard permits. If expressing the skill's Create/Supersede/Reverse needs anything the JSON
request cannot carry — HALT.

- [ ] **Step 1: Prove atomic index ownership BEFORE deleting the caller step.** Read
  `internal/app/adr_ops_test.go` (and the ops file's doc comments). For each of record /
  supersede / reverse, identify the existing assert that the re-rendered `<adrs_dir>/README.md`
  lands in the SAME transaction commit as the ADR record, that failure leaves no partial state,
  and that retry/CAS behavior holds. If one of the three lacks the index-atomicity assert, add
  a focused Go test in `adr_ops_test.go` shaped like its siblings (same fixture helpers), run
  `go test ./internal/app/ -run <NewTestName> -count=1` → PASS, and prove it is non-vacuous by
  temporarily asserting a wrong index path and watching it fail (restore after). Do NOT touch
  production transaction code.
- [ ] **Step 2: Inventory prose dependents:** `grep -rn -e "render-adr-index" -e "separate commit" tests/`
  scoped to asserts over `skills/docket-adr/SKILL.md`; list repoints.
- [ ] **Step 3: Write the failing prose asserts** (in the shard covering docket-adr prose, else
  the Task 7 guard file):

```bash
assert "docket-adr records through the Go transaction" \
  'grep -qF "docket adr record" "$REPO/skills/docket-adr/SKILL.md"'
assert "docket-adr supersede/reverse go through the Go transactions" \
  'grep -qF "docket adr supersede" "$REPO/skills/docket-adr/SKILL.md" && grep -qF "docket adr reverse" "$REPO/skills/docket-adr/SKILL.md"'
```

  Run — FAIL.
- [ ] **Step 4: Rewrite the skill's Create section** to the transaction: build the JSON request
  (fields per Task 1 Step 8's schema record — title, body sections, optional producing-change
  id, supersedes/reverses target where applicable) and run
  `` `docket adr record --request -` `` (stdin). The transaction owns, atomically: number
  allocation, the ADR file, graph validation, the re-rendered index, and the producing change's
  `adrs:` append + `## Artifacts` block. DELETE the hand-allocation, template-write, manual
  commit/CAS-rename, and "the `README.md` index is regenerated in a separate commit" steps —
  the CAS-loss guidance becomes "a typed conflict/refusal returns without writing; re-read and
  retry the operation". Keep: returning the number to the caller (read it from the operation's
  result envelope), the publish-on-acceptance flow (frozen terminal-publish).
- [ ] **Step 5: Rewrite Supersede/Reverse** to `` `docket adr supersede --request -` `` /
  `` `docket adr reverse --request -` `` — the transaction lands the new ADR, the old ADR's
  `status:` flip, and the re-rendered index together; DELETE the "regenerate the index in a
  separate commit" instruction. Keep the frozen re-publish invocation and the
  never-edit-an-Accepted-body rule verbatim.
- [ ] **Step 6: Rewrite Index/validate** — state that every ADR transaction re-renders the index
  atomically, so no follow-up render exists; keep the `docket.sh render-adr-index` command ONLY
  under an explicit "stale-index repair (no transaction in flight; no Go verb — frozen)"
  framing, and keep `adr-checks` untouched.
- [ ] **Step 7: Run the shards** from Steps 2–3 plus `bash tests/test_render_adr_index.sh` and
  `go test ./internal/app/ -run 'ADR|Adr' -count=1` → PASS.
- [ ] **Step 8: Regenerate mirrors:** `go generate ./internal/assets/ &&
  go test ./internal/assets/ -count=1` → PASS.
- [ ] **Step 9: Commit:**

```bash
git add skills/docket-adr/SKILL.md internal/app/adr_ops_test.go \
  internal/assets/embedded/tree/skills/docket-adr/SKILL.md
# plus repointed test files
git commit -m "refactor(0369): docket-adr drives record/supersede/reverse through Go transactions; redundant index render removed"
```

---

### Task 7: Stage-local, mutation-tested, shape-derived guard over the migrated surface

**Files:**
- Create: `tests/test_go_consumer_migration_guard.sh`
- Modify: `tests/runtime-budgets.tsv` (new row), `tests/test_runtime_budgets.sh`
  (`EXPECTED_TOTAL` — the registry's own registration procedure)

**Interfaces:**
- Consumes: the migrated files and new spellings from Tasks 2–6.
- Produces: the durable regression net acceptance criterion 14 requires.

**Design rules (all load-bearing):**
- **Stage-local, never repo-wide:** the scanned population is exactly the migrated files (listed
  below). It must NOT assert repo-wide zero `docket.sh` callers, and by construction it never
  scans `docs/` (point-in-time records), `scripts/` (frozen), `internal/assets/embedded/`
  (generated mirror; correspondence is `internal/assets/generate_test.go`'s job —
  `frozen-fixture-corpus-trips-repo-wide-scans`), or the frozen files/ops.
- **Shape, not spellings:** the legacy-invocation discriminator is the facade basename + op
  token with both sides bounded — any path or variable prefix reaching `docket.sh` matches; the
  op token list IS the migrated-surface definition (that is scope data, not a spelling
  enumeration; the spelling hazard — prefix variants — is what the shape absorbs).
- **Population floor** (`marker-scoped-guard-needs-a-population-floor`): every migrated file must
  POSITIVELY carry its new Go invocation, so deleting a file or rewriting it away from the Go
  verb reddens — absence asserts alone go vacuously green on an empty file.
- **Header names the residual:** equivalent prose paraphrases ("run the archive helper") survive
  a byte guard; say so in the header instead of pretending coverage
  (`byte-pattern-guard-matches-a-spelling`).

- [ ] **Step 1: Write the test file.** Skeleton (house style: mirror
  `tests/test_docket_facade.sh`'s boilerplate — `set -uo pipefail`, REPO root, `fail`, inline
  `assert` that evals a command string):

```bash
#!/usr/bin/env bash
# tests/test_go_consumer_migration_guard.sh — change 0369: the migrated Class A/D consumer
# surface stays on the typed Go CLI.
#
# STAGE-LOCAL BY DESIGN: scans ONLY the files 0369 migrated. It asserts nothing about
# preflight, board-only, render-change-links, terminal-publish, mint-stub, runner-dispatch,
# stack-*, adr-checks, render-learnings-index, backfill-change-types, mark-publish-deferred,
# README.md's frozen digest example, scripts/, docs/, or internal/assets/embedded/ — those are
# 0370/0371/0372's (or have no Go verb) and MUST keep passing here untouched. The final
# no-callers seal is explicitly NOT this guard's claim.
# RESIDUAL (named, per byte-pattern-guard-matches-a-spelling): a prose paraphrase that
# re-teaches a legacy op without the docket.sh token survives this guard; review owns that.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# The migrated surface: file → banned legacy op tokens → required new invocation.
# Op tokens are the 0369 migration map (scope data), one shape-keyed pattern for all:
#   <anything>docket.sh <op-token>  with both sides of the token bounded.
banned(){ # $1=file $2=alternation of op tokens
  grep -E -e "docket\.sh[[:space:]]+($2)([^[:alnum:]_-]|$)" "$1"
}

GATE_SRC="$REPO/cursor-rules/run-gate.md"
AGM="$REPO/AGENTS.md"
for f in "$GATE_SRC" "$AGM"; do
  assert "no legacy gate facade call in ${f##*/}" '! banned "$f" "gate-before|gate-verdict"'
  assert "Go gate pair present in ${f##*/} (floor)" \
    'grep -qF "docket run gate-before implement-next" "$f" && grep -qF "docket run gate-verdict" "$f"'
done

for s in docket-new-change docket-auto-groom docket-groom-next; do
  f="$REPO/skills/$s/SKILL.md"
  assert "no legacy backlink call in $s" '! banned "$f" "render-artifact-backlink"'
  assert "Go backlink verb present in $s (floor)" 'grep -qF "docket artifact backlink" "$f"'
done

IMPL="$REPO/skills/docket-implement-next/SKILL.md"
assert "no legacy digest call in implement-next" \
  '! grep -E -e "docket\.sh[[:space:]]+docket-status[[:space:]]+--digest-only" "$IMPL"'
assert "Go status read present in implement-next (floor)" 'grep -qF "docket status --json" "$IMPL"'

CONV="$REPO/skills/docket-convention/SKILL.md"
assert "no legacy bootstrap call in the convention" '! banned "$CONV" "bootstrap"'
assert "Go repository init present in the convention (floor)" 'grep -qF "docket repository init" "$CONV"'

TCO="$REPO/skills/docket-convention/references/terminal-close-out.md"
assert "no legacy cleanup call in terminal-close-out" '! banned "$TCO" "cleanup-feature-branch"'
assert "no legacy backlink call in terminal-close-out" '! banned "$TCO" "render-artifact-backlink"'
assert "Go finalize pair present in terminal-close-out (floor)" \
  'grep -qF "docket finalize closeout --id" "$TCO" && grep -qF "docket finalize cleanup --id" "$TCO"'
# archive-change: done path migrated; a killed-outcome leg may legitimately remain (Task 1
# disposition). Slice the done-path step with a NAMED terminator and assert the terminator
# exists (section-slice-needs-a-named-terminator), then ban the token inside the slice only.
# ADJUST the two anchors to the headings Task 5 actually wrote before landing this file.

ADR="$REPO/skills/docket-adr/SKILL.md"
assert "docket-adr transactions on Go verbs (floor)" \
  'grep -qF "docket adr record" "$ADR" && grep -qF "docket adr supersede" "$ADR" && grep -qF "docket adr reverse" "$ADR"'
# Class D: no render-adr-index follow-up inside the transaction sections. Repair-only mention
# in Index/validate is permitted. Slice Create..Index/validate with named terminators:
adr_txn_span(){ awk '/^### Create/{f=1} /^### Index \/ validate/{f=0} f' "$ADR"; }
assert "Index/validate terminator still exists (slice is bounded)" \
  'grep -qE "^### Index / validate" "$ADR"'
assert "no index-render follow-up inside ADR transaction sections (Class D stays removed)" \
  '! adr_txn_span | grep -E -e "docket\.sh[[:space:]]+render-adr-index"'

exit $fail
```

  Adapt heading anchors/quoting to the real post-Task-5/6 files (the `awk` terminator names must
  match actual headings — and assert each terminator's existence, as above, so a rename reddens
  instead of silently widening the slice). Add the equivalent named-terminator slice + banned
  assert for terminal-close-out's done-path `archive-change` per Task 1's disposition (whole-file
  ban if the kill leg migrated too).
- [ ] **Step 2: Run it green:** `bash tests/test_go_consumer_migration_guard.sh` → all ok.
  Also run it under `/usr/bin/grep` visibility: confirm no `{0,N}` intervals and that every
  pattern used works with BSD grep (`PATH_TO=/usr/bin/grep` spot-run the two `banned` patterns).
- [ ] **Step 3: Mutation-test — restore a representative legacy call in EACH migrated workflow
  family and watch exactly the right assert redden.** For each of: `cursor-rules/run-gate.md`
  (re-add a `"${DOCKET_SCRIPTS_DIR:?x}"/docket.sh gate-before implement-next` line),
  `skills/docket-groom-next/SKILL.md` (re-add a `docket.sh render-artifact-backlink` line),
  `skills/docket-implement-next/SKILL.md` (re-add `docket.sh docket-status --digest-only`),
  `skills/docket-convention/references/terminal-close-out.md` (re-add
  `docket.sh cleanup-feature-branch --slug x`), `skills/docket-adr/SKILL.md` (re-add a
  `docket.sh render-adr-index` line inside the Create section):

```bash
cp <file> <file>.bak
printf '%s\n' '<the legacy line>' >> <file>   # or insert inside the sliced section for the ADR case
bash tests/test_go_consumer_migration_guard.sh; echo "exit=$?"   # expect NOT OK + exit 1
mv <file>.bak <file>
```

  Then the floor direction: delete the `docket run gate-before implement-next` line from a
  backup-protected copy of `cursor-rules/run-gate.md` and confirm the floor assert reddens.
  Then the vacuity probe: against a temp EMPTY file substituted for one scanned path, confirm
  the floor assert (not just the ban) fails. Record each observed red in the commit message.
- [ ] **Step 4: Frozen-surface no-fire proof:** with the tree clean, confirm the guard passes
  while `grep -c "docket\.sh" skills/docket-convention/SKILL.md` (preflight sites) and
  terminal-close-out's `terminal-publish`/`render-change-links`/board lines are still present —
  i.e. the guard is provably permitting the frozen surface it claims to permit.
- [ ] **Step 5: Register the file:** add a `tests/runtime-budgets.tsv` row (budget it like the
  sibling prose-grep guards, e.g. the smallest existing ceiling class) and bump
  `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` by exactly that row's value. Run
  `bash tests/test_runtime_budgets.sh` → PASS.
- [ ] **Step 6: Commit:**

```bash
git add tests/test_go_consumer_migration_guard.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "test(0369): stage-local mutation-tested guard over the migrated Go-consumer surface"
```

---

### Task 8: Deterministic regeneration proof

**Files:**
- Verify-only: `internal/assets/embedded/tree/**`, `internal/assets/generate_test.go`

- [ ] **Step 1: First regeneration on the finished tree:** `go generate ./internal/assets/` then
  `git status --porcelain -- internal/assets/` → EMPTY (Tasks 2–6 already committed their
  mirrors; any diff here means a task skipped its regeneration — commit the correction and find
  which task's commit missed it).
- [ ] **Step 2: Second regeneration, no diff:** run `go generate ./internal/assets/` again;
  `git status --porcelain -- internal/assets/` → EMPTY. Non-empty output is a determinism defect
  in the pipeline — HALT and report (generator work is out of scope).
- [ ] **Step 3: Generation tests from source, cache-defeated:**
  `go test ./internal/assets/ -count=1 -v` → PASS (canonical/embedded correspondence proven by
  the package's own test).
- [ ] **Step 4: Commit only if Step 1 corrected anything**; otherwise record "regeneration
  idempotent, no diff" in the task report (no empty commit needed).

---

### Task 9: Full-suite gate and acceptance sweep

**Files:** none created — verification and evidence only.

- [ ] **Step 1: Full suite from source:** `go run ./cmd/docket development test`. Green required.
  Treat `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines as screening findings to note;
  a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach — confirm serially per
  `tests/README.md` and resolve or record before proceeding. A red unrelated to this branch is
  re-run on the unmodified base before being believed (learnings: environment).
- [ ] **Step 2: Acceptance sweep** against the spec's criteria 1–18 — for each, name the task
  and artifact that satisfies it; explicitly verify the frozen-surface criteria by inspection:
  10 (no `runner-dispatch`/dispatch-block edits in `git diff main...HEAD`), 11–13 (no
  Class C/0370 files touched — `git diff --name-only --no-renames main...HEAD` contains ONLY
  the files this plan names; `--no-renames` per `diff-derived-allowlist-needs-no-renames`),
  16 (the discrepancy list — docket-status no-op site, README digest example, any frozen kill
  leg, the repair-only index render — is recorded for the results notes).
- [ ] **Step 3: Hand the discrepancy list, Task 1's verification table, and the mutation-test
  evidence to the run's results/close-out notes** (the parent run owns writing the results file;
  this task's report must carry the content).

## Task 1 findings

Recorded by Task 1's worker (premium). All eight steps executed against the source tree and a
built `./cmd/docket` binary run over scratch docket-topology fixtures
(`"${TMPDIR:-/tmp}/docket-0369-verify.XXXXXX"`). **Abort boundary: CLEARED** — every mapped verb is
a pre-existing public Go verb reachable by a straight invocation/structured-output swap; none needs
a new/expanded verb or a bespoke adapter. Two frozen discrepancies (below), neither a halt.

### Pinned invocation exemplars (skill-prose spelling — bare installed binary, Global Constraint 5)

Later tasks copy these VERBATIM; this record is authoritative where it refines a Task's candidate.

| Verb | Pinned exemplar | Consumers |
|---|---|---|
| gate-before | `docket run gate-before implement-next` | run-gate.md, AGENTS.md (Task 2) |
| gate-verdict (attributed) | `docket run gate-verdict <key>` | run-gate.md, AGENTS.md (Task 2) |
| gate-verdict (observe) | `docket run gate-verdict --unattributed [<id>…]` | run-gate.md, AGENTS.md (Task 2) |
| artifact backlink (active) | `docket artifact backlink --repo-dir .docket --artifact <spec-path> --change docs/changes/active/<id>-<slug>.md` | planning skills (Task 3), terminal-close-out (Task 5) |
| artifact backlink (archived) | `docket artifact backlink --repo-dir .docket --artifact <spec-path> --change docs/changes/archive/<UTC-date>-<id>-<slug>.md` | terminal-close-out (Task 5) |
| status digest | `docket status --json` (`--repo-dir <dir>` where needed) | implement-next (Task 4) |
| finalize closeout | `docket finalize closeout --id <id> [--input <notes.json>]` | terminal-close-out (Task 5) |
| finalize cleanup | `docket finalize cleanup --id <id>` | terminal-close-out (Task 5) |
| repository init | `docket repository init` (`--repo-dir <dir>` where needed) | convention CREATE_ORPHAN (Task 4) |
| adr record | `docket adr record --request -` | docket-adr (Task 6) |
| adr supersede | `docket adr supersede --request -` | docket-adr (Task 6) |
| adr reverse | `docket adr reverse --request -` | docket-adr (Task 6) |

`--change` / `--artifact` are the FULL canonical repository-relative paths as they appear in the
pinned corpus (`c.Path()` == `docs/changes/active/<id>-<slug>.md`, not a `changes_dir`-relative
tail); `--artifact` and `--change` are both `MarkFlagRequired`. `--input` / `--request` accept `-`
for stdin. All twelve verbs are pre-existing landed public Go commands (`internal/cli/{gate,run,
artifact,change,finalize,repository,adr}.go`).

### Gate pair (PARTICULAR RISK) — straight binary swap, no adapter

- `scripts/gate-before.sh` / `scripts/gate-verdict.sh` are change-0334 thin delegators:
  `DOCKET_BIN="${DOCKET_BIN:-docket}"; exec "$DOCKET_BIN" run gate-before|gate-verdict "$@"` — argv
  forwarded, stdout + exit code passthrough, ZERO owned behavior. `scripts/docket.sh`'s dispatch
  (`WRAPPED_OPS` loop) just `exec`s the wrapper — no preflight, env mutation, or output rewriting.
  So the migrated caller calling `docket run gate-*` directly hits the EXACT same binary entrypoint
  the Bash facade reaches; attribution key, unattributed fallback, and one-retry accounting are all
  owned by the Go binary (`internal/app/rungate_before.go`, `rungate_verdict.go`) and are unchanged
  by the swap. `scripts/verify-run.sh` reaches the Go authority through the same `DOCKET_BIN` seam.
- **Report-line vocabulary** (Task 2 prose must keep these tokens exactly):
  - gate-before: `gate-armed <key>` | `gate-unarmed <reason>` (both exit 0); bad target → usage
    error, exit 2 (verified: `docket run gate-before bogus` → rc 2).
  - gate-verdict attributed: leading decision `gate-done` | `gate-retry-once` | `gate-stop`;
    outcome word ∈ {`no-attributable-claim`, `ambiguous-claims`, `gate-unavailable`,
    `run-complete`, `run-unclaimed`, `run-halted`, `run-waiting`, `run-incomplete`}.
  - gate-verdict observe (`--unattributed`): `gate-observe <outcome>`, outcome ∈ {`no-current-run`,
    `gate-unavailable`, `run-*`}. Observe is a structurally separate render path with NO branch to
    `gate-retry-once` — `--unattributed` can never authorize a retry.
- **Fixture execution (verified, exit 0 throughout):** `gate-before implement-next` →
  `gate-armed implement-next-<utc>-<pid>-<hex>`; `gate-verdict <key>` (fresh process, empty backlog)
  → `gate-done <key> no-attributable-claim`; `gate-verdict --unattributed` → `gate-observe
  no-current-run`. `--json` yields protocol-v1 envelopes: `{"protocol_version":1,"operation":
  "run.gate-before","result":"applied","armed":true,"key":…,"target":"docket-implement-next"}` and
  `{"protocol_version":1,"operation":"run.gate-verdict","result":"applied","observations":[…]}`.
  Cross-process durability + the exactly-one `gate-retry-once` / later `gate-stop` retry accounting
  are covered by `tests/test_gate_facade.sh` cases (c) (read, not re-run).

### artifact backlink — metadata-worktree stamp, idempotent, no new capability

Go op `internal/app/artifact_backlink.go` takes `--repo-dir` (worktree the artifact lives in) and
canonical repo-relative `--artifact`/`--change`; refuses absolute paths, `..` escapes, symlink
escapes, malformed markers, unknown change. Verified in a metadata-worktree-shaped fixture
(a `.docket`-style docket-branch checkout): stamp #1 → `rendered`; stamp #2 → NoOp `unchanged`,
byte-identical file (idempotent). `--json` → `{"…","operation":"artifact.backlink","result":
"no-op","disposition":"unchanged"}`. Refusals exit 2 with typed reasons (`absolute-path`,
`unknown-change`). It stamps a metadata-worktree artifact with NO new capability.

### status --json contract

Top-level keys (verified against the live repo): `protocol_version` (=1), `operation` (=`status`),
`result` (=`applied`), `ready` (array of ids in selection order — the build-ready queue),
`changes`, `records`, `findings`, `summary`, and `context`. Change objects carry `id`, `slug`,
`title`, `status`, `priority`, `type`, `location`, `path`, `version`, `readiness`,
`readiness_reason`, `unmet_dependencies`, `effective_base`, `ready`. **Write-free**: `git -C
.docket status --porcelain` unchanged before/after, no board write. `readiness` vocabulary (Task 4):
`build-ready` ("ready to build"), `needs-brainstorm` ("needs a design brainstorm before it can be
built"), `waiting-dependency` ("waiting on unmet dependencies"), `not-proposed` ("not in proposed
status"), `auto-groom-blocked` ("auto-groom blocked; needs a human design pass"). The implement-next
skip vocabulary ("needs-brainstorm", dependency waits) maps to `needs-brainstorm` and
`waiting-dependency`.

### finalize closeout / cleanup

- **(a) closeout outcome coverage** (`internal/app/finalize_closeout.go`): covers `done-archived`
  (integration-branch merge), `stacked-merged` (in place), `root-archived` (stack-root carry), plus
  the retained/terminal dispositions `already`, `children-retarget-required`, `contended`,
  `blocked`, `unknown`, `failed`. Legal closeout SOURCE statuses are `implemented` or
  `stacked-merged` ONLY (`ReasonCloseoutIllegalSource`). **The `killed` outcome is NOT covered** →
  **DISCREPANCY (frozen, not a halt):** the kill drivers' `docket.sh archive-change … --outcome
  killed` legs stay frozen; Task 5 gives the Go verb to the done drivers and keeps a frozen
  labeled `--outcome killed` block for the kill legs.
- **(b) closeout atomic-ownership set** (done-archived shape, ONE metadata transaction): `MarkDone`
  after a merge-commit reachability proof; `updated` stamped from the verified GitHub `mergedAt`
  (the UTC terminal/archive date is derived INSIDE the transaction — no caller date input); claim
  cleared with historical branch/PR fields preserved; record relocated to the dated archive path
  (create-archive + delete-active in one plan); the artifact `## Artifacts` block re-rendered;
  **every metadata-resident backlink re-rendered (this INCLUDES the spec, which lives on the
  metadata ref)**; the inline board re-rendered. A separate follow-up integration-ref leg
  (`finalize.closeout-backlink`) patches only the merged plan/results `docket:backlink` blocks.
  **Task 5 implication:** closeout OWNS the spec backlink restamp for the done path →
  **second Class D absorption** — Task 5 DELETES the caller's spec-backlink restamp for the done
  path (keep only for legs closeout does not drive; cite `internal/app/finalize_closeout_*_test.go`).
  Task 5 must not double-run any step closeout owns; it keeps only the frozen `render-change-links`
  `## Artifacts` writer (which closeout also re-renders, but that call is the frozen Bash surface —
  left byte-untouched per the change's mixed-state blessing) and publish/board frozen steps.
- **(c) cleanup ownership proof** (`internal/app/finalize_cleanup.go`): manifest-fact-driven
  `workspace.Cleanup` (never a base recomputed from the terminal record); LOCAL feature ref deleted
  only when the exact recorded tip is detached from every worktree AND contained in the verified
  merge chain; REMOTE feature ref deleted only under an exact old-value lease AND after a fresh
  probe proves no open child PR still targets it; "never calls a global worktree prune, force-
  removes a checkout, recursively deletes by pathname, or touches the primary, metadata,
  transaction, sibling, or foreign worktree." → **equal-or-stricter** than the Bash `--slug` guard
  (which only bounded removal to `.worktrees/<slug>`, never `.docket/`). Fail-closed on every
  unanswerable probe (pending/retained dispositions).

### repository init vs docket.sh bootstrap

`internal/app/repository_init.go` (`RunRepositoryInit`) performs the guarded CREATE_ORPHAN
creation: a parentless empty-tree orphan `docket` root with a versioned receipt; a create-only
publish that adopts an already-published exact shape and refuses a foreign one; the local branch +
persistent `.docket` worktree; disabled worktree hooks; the unstaged managed `.gitignore` edit +
(only when authorized) the parent-facing dispatch surfaces; the ownership record. Idempotent on
re-run. Fail-closed refusals (`initGuard`): legacy → `docket repository migrate`; unknown probe →
`docket repository check`; foreign/conflicting `.docket` → `docket repository check`; dirty primary
→ refuse. Never prompts, never reads stdin. → **equal-or-stricter** refusal surface than
`docket-config.sh --bootstrap`. No HALT. (Not executed against the live repo to avoid mutating it;
verified by source + guard classification.)

### ADR transactions and index ownership (Class D)

`internal/app/adr_ops.go`: record / supersede / reverse each drive ONE validated atomic transaction
commit landing the ADR record, the re-rendered index `docs/adrs/README.md` (MutationCreate or
MutationReplace — "rerendered on every ADR operation, unconditionally"), the target's status flip
("Superseded by ADR-<n>" / "Reversed by ADR-<n>") for supersede/reverse, and — when a producing
change is supplied — that change's `adrs:` append + re-rendered artifact block. Index-atomicity
assertions already exist in `internal/app/adr_ops_test.go` (plan is EXACTLY {new ADR, index} and
{new ADR, index, producing change}; index reflects the new ADR; `adrs: [2]` append) → the
standalone `render-adr-index` follow-up in `skills/docket-adr/SKILL.md` is genuinely redundant, so
the **Class D removal is justified** (Task 6). JSON request schemas (`--request -`, stdin):
- **record** (`ADRRecordRequest`): `request_id`, `title`, `context`, `decision`, `consequences`,
  `alternatives`, `relates_to` []int, optional `change` {`id`, `path`, `version`}.
- **supersede / reverse** (`ADRReplaceRequest`): `request_id`, `target` {`id`, `path`, `version`},
  `successor` (a full `ADRRecordRequest`; the successor's own `request_id` is ignored — the outer
  key governs). Decoders use `DisallowUnknownFields`.

### Inventory classification (from the change file's Reconcile log + this task's findings)

**Class A — migrate to a landed public Go verb (invocation/structured-output swap):**

| Old (`docket.sh …`) | New (`docket …`) | Files |
|---|---|---|
| `gate-before` | `run gate-before` | cursor-rules/run-gate.md, AGENTS.md (CLAUDE.md symlink) |
| `gate-verdict` | `run gate-verdict` | cursor-rules/run-gate.md, AGENTS.md |
| `render-artifact-backlink` | `artifact backlink` | docket-new-change, docket-auto-groom, docket-groom-next, terminal-close-out |
| `docket-status --digest-only` | `status --json` | docket-implement-next |
| `cleanup-feature-branch` | `finalize cleanup` | terminal-close-out |
| `archive-change` (done path) | `finalize closeout` | terminal-close-out |
| `bootstrap` | `repository init` | docket-convention (CREATE_ORPHAN) |

**Class D — redundant follow-up absorbed by a transaction (remove, do not replace):** standalone
`render-adr-index` after `adr record/supersede/reverse` in docket-adr. PLUS the second absorption
found here: the done-path spec-backlink restamp in terminal-close-out (closeout owns it atomically).

**Frozen / unmapped (leave byte-untouched, permitted in guards):** `preflight` (~22 sites),
`docket-status --board-only` / full orchestrator, `render-change-links`, `terminal-publish`,
`mint-stub`, `mark-publish-deferred`, `render-learnings-index`, `runner-dispatch`,
`stack-base`/`stack-children`/`stack-closeout`, `backfill-change-types`, `adr-checks`, all
`scripts/*.sh`. A file mixing frozen + migrated calls (terminal-close-out.md, docket-status
SKILL.md) ends partially migrated — the spec-blessed intermediate state.

**Discrepancies (frozen, recorded, NOT halts):** (1) `finalize closeout` does not cover the
`killed` outcome → kill legs keep `archive-change … --outcome killed`. (2) `docket-status/SKILL.md`
carries no standalone `--digest-only` call site (digest lines are the frozen orchestrator's own
output) — no-op. (3) `README.md`'s `docket-status --digest-only --type untyped` example is coupled
to the frozen `backfill-change-types` one-off — stays; the Task 7 guard permits it. (4) the
repair-only `render-adr-index` in docket-adr's Index/validate section stays, reframed repair-only.
