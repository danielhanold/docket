<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0339 — Retire the gate-run.sh launch/liveness/stop facade now that the native Go-v1 gate is canonical (collapse the shared docket-liveness.sh seam with runner-dispatch.sh)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-24-0339-retire-the-gate-run-sh-launch-liveness-stop-facade-now-that.md)**
<!-- docket:backlink:end -->

# Retire the gate-run.sh launch/liveness/stop facade — design

**Change:** 0339 · **Date:** 2026-08-23 · **Type:** refactor
**Revision:** 2026-08-23 — supersedes the original 2026-08-23 spec at this same path.

## Revision note (2026-08-23)

A build against the original spec halted twice. Two of its decisions were wrong, and both are
re-settled below. Everything else in the original stands unchanged.

**Defect 1 — the new home was unbuildable.** The original chose
`skills/docket-build/references/gate-execution.md` as the destination for gate-run.md's surviving
content. Building that move produced **258 lines / 2493 words** against that file's budget of
**130 / 1200** (`tests/test_skill_size_budgets.sh`). That budget is not slack that ran out: change
0234 **ratcheted it down** (175/1650 → 130/1200) when it split the file along the
instruction-vs-evidence axis, and the file is governed by a one-directional neutrality invariant —
it is the harness quarantine, holding measured per-harness verdicts and mechanism detail only, and
*"a rule the agent must PERFORM at the moment it starts the gate cannot live there — a rule in a
file read once, ahead of the act, does not intervene at the act."* The caller's loop is exactly
such a rule. Raising the budget to fit would reverse 0234's ratchet and dissolve the charter that
justifies the file's existence. **Re-settled: a new reference file** (decision 4 below), following
change 0271's precedent, which created `references/delegation-execution.md` rather than merging
adapter evidence into gate-execution.md for this same charter reason.

**Defect 2 — the stop verdicts had no stated re-grounding.** The original said the `--stop` token
set *"becomes the native stop's JSON result"* and deferred field spellings to implementation time.
There is no native twin: `docket gate stop` returns a `GateResult` whose envelope `result` is
`applied` / `no-op` / an error, with `state` preserved — a two-axis answer where the shell reported
one token. `skills/docket-build/SKILL.md`'s three died-disposition bullets and
`tests/test_gate_execution_posture.sh`'s **derived** verdict set both key on the shell's three
tokens, so "read the spelling later" left the disposition semantics unspecified and the test's
derivation source about to be deleted. **Re-settled: an explicit mapping table** owned by the new
file (decision 5 below), which both the SKILL bullets and the posture test's derivation bind to.

## Problem

Docket carries two implementations of "launch a long-running child detached, check its liveness,
stop it": the native Go-v1 gate (`docket gate launch|observe|stop|recover`, change 0314 —
`internal/app/gate.go` over the `internal/process` supervisor) and the shell facade
`scripts/gate-run.sh`. Change 0338 retired the facade's `--observe` verb (it now refuses with a
pointer to `docket gate observe --json`), leaving `--launch` and `--stop` as a second
launch/liveness/stop path held in agreement with the native gate only by convention. The shared
liveness predicate `docket_group_alive_and_ours` in `scripts/lib/docket-liveness.sh` (extracted by
change 0284 after real drift) ties the facade to its other consumer, `runner-dispatch.sh`.

`docket-finalize-change` and `docket-implement-next` already run the native gate end-to-end.
`docket-build`'s SKILL.md is the last skill-level caller of `gate-run --launch`/`--stop`.
`runner-dispatch.sh` never calls the facade — it only shares the liveness lib.

## Decisions

Decisions 1–3 are unchanged from the original spec. Decisions 4 and 5 replace the original's
decision 4.

1. **Full retirement, no wrapper.** `scripts/gate-run.sh` and `scripts/gate-run.md` are deleted;
   the `gate-run` entry leaves `WRAPPED_OPS` in `scripts/docket.sh` (and its header comment line),
   so `docket.sh gate-run` fails like any unknown op. No passthrough shim and no deprecation
   window — the same posture 0338 took for `--observe`. A shell entry point kept "for environments
   without the Go binary" was rejected: it preserves exactly the two-spellings drift class 0338
   killed, and the native gate is proven in production.
2. **`runner-dispatch.sh` stays on its shell liveness path.** Migrating the runner-delegation
   subsystem (change 0079) onto the native supervisor is a separate future change.
3. **The liveness lib is kept, single-consumer.** `scripts/lib/docket-liveness.sh` remains, with
   `runner-dispatch.sh` as its sole consumer; **its code is byte-unchanged** and only its ownership
   prose moves. Folding it back inline was rejected as churn with no drift left to prevent.
4. **The surviving guidance moves to a NEW reference file:**
   `skills/docket-build/references/gate-caller-loop.md`. `gate-execution.md` cannot hold it, for
   the ratchet-and-charter reason recorded in the revision note above; `scripts/*.md` cannot hold
   it because those files document `.sh` siblings and a native-command doc family is a larger
   structural decision than this cleanup should smuggle in. The precedent is change 0271's
   `references/delegation-execution.md`: when content does not fit gate-execution.md's charter, the
   answer is a sibling file with its own first-row budget, not a raise.
5. **The stop verdicts are re-grounded by an explicit mapping table in the new file.** The three
   shell tokens have no native twin, so the new file states the mapping from what
   `docket gate stop` actually returns to the three caller dispositions the posture already
   defines. The table becomes the single derivation source for both the SKILL bullets and the
   posture test's verdict set, preserving the derive-never-hand-list property the shell contract's
   token table currently supplies.

## Branch state

The feature branch `feat/retire-the-gate-run-sh-launch-liveness-stop-facade-now-that` is **reset to
drop commit `14cd56c9`** (the Task 1 build against the rejected home), keeping only the plan
commits. The plan is rewritten from this revised spec. The plan writer therefore starts from a
content-clean branch: no partial move exists, and nothing from the abandoned attempt should be
salvaged or re-applied.

## What changes

### The new reference file — `skills/docket-build/references/gate-caller-loop.md`

The file's charter, stated on the page: **the caller-side contract for driving the native gate** —
the loop, the vocabulary it resolves into, the stop mapping, and the measurement record behind the
launch shape. It is a caller contract, not a harness quarantine; that is the axis separating it
from `gate-execution.md`, and the same axis 0271 used.

Contents, in order:

1. **The caller's loop.** The bash fence from `scripts/gate-run.md` § *The caller's loop*, moved
   **byte-identical**. It already drives `docket gate observe <run-dir> --json` and parses with jq
   (change 0338), so nothing inside the fence is edited — this is a move, not a redesign, and the
   0338 boundary forbids touching the observe format. The surrounding prose moves with it, with two
   retargets: the sentence naming the test that extracts and executes the fence now names the new
   test file (below), and the pointer to the caller's disposition policy keeps pointing at
   `skills/docket-build/SKILL.md` § *Gate execution posture*.
2. **The state vocabulary and retryability rule.** The states are read from `internal/app/gate.go`
   (`GateResult` / its state constants) at implementation time; the rules are: `signaled` and
   `vanished` both resolve to `died` (with `cause` carrying the document's own qualifier, possibly
   empty), a `died` run **never finished** so it is never `failed` and only `failed` may feed
   repair work, and **only `running` is retryable** — every other arm is terminal, including the
   fail-closed unknown-document arm.
3. **The caller's verbs.** A three-row table — `launch`, `observe`, `stop` — naming what each verb
   returns to the caller, with an explicit note that `recover` and `cleanup` are operator verbs,
   not caller-loop verbs. This table exists because the posture test derives its verb coverage from
   a published table rather than a hand-list (see *Tests*), and the native CLI registers five
   subcommands of which only three belong to this loop.
4. **The stop mapping table.** Native `docket gate stop` returns a `GateResult`: the envelope's
   `result` is `applied` or `no-op` (or an error), and `state` is preserved — `internal/app/gate.go`
   states it directly: *"termination is applied; an already-terminal no-op carries the preserved
   state (consumers read state; the stop performed nothing)."* The table maps that two-axis answer
   onto the three caller dispositions:

   | Native stop outcome | What it means | Caller disposition |
   |---|---|---|
   | `no-op`, `state` preserved | the run was already finished; the stop performed nothing | **re-observe and key on the preserved `state`** — this is the ordinary outcome of stopping a live child |
   | `applied` | we terminated it; the run produced no verdict of its own | **one relaunch only**, and only where the child is idempotent |
   | an error / the run unreachable | nothing can be proven about what survives | **abort and report loudly**; never relaunch |

   Exact field spellings and enum values are **read from `internal/app/gate.go` and
   `internal/cli/gate.go` at implementation time**, never restated from memory; if a spelling
   differs from the ones written above, the source wins and this table is written to match it.
   The mapping's *semantics* — which outcome earns which disposition — are settled here and are
   not implementation-time discretion.
5. **The per-platform capability note**, moved from `scripts/gate-run.md` § *Per-platform
   capability note*, carried as a **shell-era measurement record**: the setsid(1) / script(1) /
   `set -m` ladder, the measured rejection of `script(1)`, and the ADR-0080 and
   `gate-execution-evidence.md` quotes, all verbatim, labelled on the page as measurements taken
   against the retired shell launch shape.
6. **The evidence-carryover paragraph**, immediately after (5). It states that the native launcher
   is not bound by the shell-era rung-3 narrowing: ADR-0095 records that the native per-run
   supervisor delivers a **genuine new session on both Darwin and Linux**, and the Go launcher's
   own comment in `internal/process/launch.go` describes it *"as a Setsid session-leader supervisor
   with the live lock and a handshake"* (clause quoted verbatim from source at implementation time,
   ADR-0054 anchoring — symbol or quoted clause, never a line number). The native guarantee is
   therefore **at least as strong** as the shape the per-harness verdicts were measured under, so
   those verdicts **carry over without re-probing** — and that carryover is recorded on the page
   rather than left silent.

**Path-spelling constraint.** This file is a skill body, so `tests/test_consuming_repo_scripts.sh`
forbids any bare `scripts/<name>.sh` path in it (a skill ships into a consuming repo that has no
`scripts/` directory). The retired facade is referred to by name — *the retired `gate-run` shell
facade* — never by path; the liveness lib is not named at all.

**Its budget row.** The file gets its own **first row** in `tests/test_skill_size_budgets.sh`'s
`BUDGETS` table, set per that file's rounding rule from the **measured post-write actuals**: lines
to the next multiple of 5, words to the next multiple of 50, and if either lands within the 25-word
(resp. zero-line) near-zero band, the multiple after. The ledger comment above the table records
the first-row derivation and — per the table's own naming requirement — names
`gate-execution.md` as the home considered and rejected, with the ratchet-and-charter argument,
plus `gate-execution-evidence.md` (evidence of probes, not a caller contract) as the second.

### `gate-execution.md` — minimal retargets only, budget unchanged

The file keeps its **130 / 1200** budget and its charter. It gains nothing; two existing pointers
move:

- The **mitigation paragraph** names `docket gate launch` (implemented in
  `internal/process/launch.go`) as the shipped implementation, replacing the facade invocation of
  the retired helper. The runtime-probe narrowing sentence is rewritten to match ADR-0095 rather
  than the shell ladder: the native launcher delivers a genuine session on every supported
  platform, so the page no longer needs the "narrows honestly per platform" hedge and no longer
  points at a per-platform note it does not own.
- **Capability 5's pointer** retargets from `scripts/gate-run.md` to `gate-caller-loop.md` as the
  owner of the state vocabulary and its retryability rule.

No harness row (`### claude|cursor|codex|opencode`) is touched — no verdict is rewritten or
re-probed. Current actuals are **126 / 1131** against 130 / 1200: four lines and 69 words of
headroom. If the retargets exceed it, the paragraph is **trimmed to fit, never raised** — raising
this row is what defect 1 was.

### Caller migration — `skills/docket-build/SKILL.md`

Rewrite the "shipped implementation of clauses 1–3" passage of § *Gate execution posture* to the
native verbs:

- Launch: `docket gate launch --root <dir> --cwd <dir> -- <command…>`; a failed launch is read from
  the launch's protocol-v1 JSON envelope with jq (already the documented caller-loop dependency
  since 0338). The disposition is unchanged: **abort and report, never a retry loop**.
- Stop: `docket gate stop <run-dir>`.
- The canonical-loop pointer retargets from `gate-run.md` to `gate-caller-loop.md`. The keying
  rules 0286 and 0338 added stay: the loop is bound to the JSON document and its jq extraction, a
  hand-rolled reading of the document is the drift that spins the gate, and no `state=` keying
  reappears.
- **The three died-disposition bullets are re-keyed on the mapping table's rows**, one bullet per
  row, each bullet opening with its row's native outcome in a code span so each token still owns a
  disposition of its own. The posture's meanings are preserved exactly: `died` is never a red suite
  and mints no repair work; the one relaunch is licensed by **idempotence**, not by the state, and
  is **gated on what the stop reported**; a non-idempotent child keeps its site's existing posture;
  a second `died` is abort-and-report. The "ordinary live-child stop" naming survives the
  re-grounding — it is now the **`no-op` / preserved-state** row, and that row re-observes and keys
  on what returns.
- *Abandoning a live child* is unchanged except for the stop invocation's spelling.
- The two-vocabulary disambiguation the section carries (`stopped` as both a stop token and an
  observe state, with opposite dispositions) is **resolved rather than restated**: after
  re-grounding, the stop axis no longer produces `stopped`, so the ambiguity the sentence existed
  to defuse is gone. Its removal is deliberate, not an oversight, and the plan records it as such.

`docket-finalize-change` and `docket-implement-next` are already native; they take only the
doc-pointer retarget below.

### Cross-reference sweep

Every reference to `gate-run` in **maintained** source retargets or dies. The site list is
**derived from a whole-repo grep at build time** and every hit is sorted explicitly into
executable / maintained prose / frozen record — never hand-listed (house rule). Known-likely
carriers: `skills/docket-finalize-change/SKILL.md` and its `references/gate-failure.md`,
`skills/docket-convention/SKILL.md`, `scripts/docket.sh`, `tests/check-test-source-hygiene*`,
`tests/README.md`. **Frozen records — archived changes, results files, merged plans, and Accepted
ADRs — stay untouched**; rewriting them falsifies history.

### Liveness lib ownership prose

- `scripts/lib/docket-liveness.sh` header: rewrite "shared with gate-run.sh" to name
  `runner-dispatch.sh` as sole consumer. **Code byte-unchanged.**
- Same rewrite in `scripts/runner-dispatch.sh` comments and `scripts/runner-dispatch.md` prose.

### Tests

Every guard below is sorted by **what it guards** before its premise is deleted
(learnings: *test-premise-deleted-not-regated*). A guard whose subject dies is deleted; a guard
whose *property* survives is re-keyed onto the surviving carrier, never dropped.

- **`tests/test_gate_caller_loop.sh` — new.** The fence-extraction-and-execution harness currently
  in `tests/test_gate_run.sh` (scripted-document arms plus the real-gate arm) **moves here**,
  retargeted to extract the fence from `gate-caller-loop.md`. This is the executable coverage of
  the loop; it must not vanish with the contract that used to host it. A new test file mirroring
  the new reference 1:1 is chosen over folding the harness into the posture test, which is
  prose-shaped by construction and should stay that way. `tests/README.md` gains its row if that
  file indexes suites.
- **`tests/test_gate_execution_posture.sh` — retargeted**, in the same commits as the prose it
  guards:
  - `CONTRACT="$REPO/scripts/gate-run.md"` → the new reference path; the non-vacuity line-count
    anchor is re-set against the new file.
  - The **verb derivation** (`grep -oE '^gate-run\.sh --[a-z-]+'` over the contract's Usage block)
    → derived from the new file's **caller's-verbs table**, same awk table-slice shape the stop
    tokens use. Not from `internal/cli/gate.go`: that registers five subcommands, two of which are
    operator verbs the posture correctly does not discuss, so deriving from the CLI would demand
    coverage of verbs that do not belong to the loop.
  - The **stop-token derivation** (the `awk -F'|'` slice of the contract's token table) → the same
    slice over the new file's **stop mapping table**, keyed on the table's first column. Each
    derived row must still open a disposition bullet of its own in the SKILL body — the shape
    assert stays, because presence-anywhere was measured insufficient when this guard was written.
  - `helper: the posture names the facade invocation` (`docket\.sh gate-run`) → the native
    invocation. Same for the reference's mitigation assert in group (12d).
  - `reference: capability 5 points at the contract that owns the state vocabulary`
    (`grep -qF "gate-run.md"`) → the new file's name.
  - The per-harness `! grep -qF "gate-run"` negative asserts: the **property** they guard is "no
    verdict was rewritten to name the shipped implementation". Their literal premise dies with the
    facade, which would leave them permanently, vacuously green
    (learnings: *assert-detects-removal-not-replacement*). They are **re-keyed onto the current
    implementation's names** (`docket gate launch`, `gate-caller-loop.md`), not deleted.
- **`tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh` — deleted**, after the sort: the
  subject-mechanics guards die with the script (native equivalents live in `internal/process` and
  `internal/cli` Go tests); the fence harness has already moved to the new test file.
- **`tests/test_skill_size_budgets.sh`** — one new `BUDGETS` row plus its ledger comment.
- **`tests/test_docket_liveness.sh`** — unchanged.

**Mutation-proof each retargeted assert.** Restoring the old spelling, or deleting the row the
derivation reads, must redden it. Take a **backup copy of the file before mutating and restore from
that copy** — never re-type the original from memory
(learnings: *mutation-restore-needs-a-backup-copy*). Confirm each edit landed by re-reading the
file; a shell edit that silently no-ops exits 0 and reads as success
(learnings: *agent-shell-noop-reads-as-success*).

## Commit boundaries — green at every commit

This is the defect class that halted the build twice: an edit landed in one commit and the guard
that admits it in the next. **The full suite** — `finalize.test_command`, including
`test_skill_size_budgets.sh` and `test_gate_execution_posture.sh` — **must be green at every
commit.** Two hard rules, and a task shape that satisfies them:

- **(a)** `gate-caller-loop.md`'s `BUDGETS` row lands **in the same commit that creates the file**.
  The budget test's completeness guard reddens the instant any `skills/**/*.md` exists without a
  row.
- **(b)** Each posture-test retarget lands **in the same commit as the prose edit it guards**.
  Neither ordering survives alone: an old assert against new prose is red, and a new assert against
  old prose is red.

Additionally, every commit touching `skills/` regenerates the embedded assets
(`go run ./cmd/genassets`) **in that same commit**.

Suggested task boundaries, each one commit and each independently green:

1. **Create `gate-caller-loop.md` + its budget row + ledger comment + assets regen.** Content is
   duplicated with the still-present `gate-run.md` at this commit; nothing reads the new file yet,
   so the suite is green.
2. **Retarget `gate-execution.md`'s two pointers + the posture test's group (12d) + assets regen.**
   Verify the file's measured actuals against the unchanged 130 / 1200.
3. **Rewrite `docket-build` SKILL.md § *Gate execution posture* to the native verbs and the mapping
   table's rows + retarget the posture test's groups (12a)/(12b) — `CONTRACT`, the verb
   derivation, the stop-token derivation, the invocation asserts + assets regen.** `gate-run.sh`
   and `gate-run.md` still exist and are still self-tested here, so `test_gate_run*.sh` stay green.
4. **Create `tests/test_gate_caller_loop.sh` carrying the fence harness** (and `tests/README.md` if
   it indexes suites). The fence now has executable coverage in two places for one commit — which
   is the point: coverage overlaps rather than gaps.
5. **Sweep the remaining cross-references** derived by whole-repo grep + assets regen.
6. **Delete `scripts/gate-run.sh`, `scripts/gate-run.md`, `tests/test_gate_run.sh`,
   `tests/test_gate_run_stop.sh`, and the `gate-run` `WRAPPED_OPS` entry and header comment line.**
   Atomic: the script, its contract, and its tests go together.
7. **Liveness-lib ownership prose** in the three files. Code byte-unchanged.

Nothing may delete `gate-run.md` before tasks 2–4 have retargeted every derivation and pointer that
reads it. Tasks 1–4 are the ordered spine; 5 and 7 are order-free after their predecessors.

## Out of scope

- Migrating `runner-dispatch.sh` onto the native supervisor — a future change.
- Any change to the protocol-v1 document schema, and any edit to the observe format (0338's
  boundary). The fence moves byte-identical.
- Re-probing the per-harness gate-execution verdicts — the carryover paragraph covers them.
- Any change to `scripts/lib/docket-liveness.sh`'s code.
- The forked-mode launch question (change 0264) and ADR-0024 — explicit non-adjacencies, as in
  0338.
- Any new ADR. ADR-0095 already records the native supervisor's session guarantee; this change
  consumes that record rather than adding to it.

## Risks

- **Green-per-commit.** The named failure mode, addressed by rules (a) and (b) and the task shape
  above. Mitigation is procedural, so the plan must carry the rules per-task, not only here.
- **A derivation silently loses its subject.** Deleting `gate-run.md` while a test still derives
  from it turns a real guard into an empty loop that passes. Mitigation: the ordering constraint,
  plus a non-vacuity anchor on every retargeted derivation (`n_verbs >= 2`, `n_tokens >= 3`) that
  reddens when the slice locates nothing.
- **Prose drift sweep.** The mechanical deletion is small; a missed `gate-run` reference in
  maintained source is the real surface. Mitigation: whole-repo grep, every hit sorted explicitly.
- **`gate-execution.md` headroom.** Four lines and 69 words. Mitigation: trim to fit; a raise is
  out of bounds by decision 4.

## Assumptions

Judgment calls made in authoring this revision, in place of asking:

1. **The caller's-verbs table is added to the new file** as the posture test's verb-derivation
   source. The human's settled design named the loop, the vocabulary, the stop mapping, and the
   capability note; it did not name a verb table. It is required because the existing derivation
   reads `gate-run.md`'s Usage block, and the native CLI's five registered subcommands are not the
   right substitute — `recover` and `cleanup` are operator verbs the posture does not and should
   not discuss.
2. **The fence's executable coverage moves to a new `tests/test_gate_caller_loop.sh`.** The settled
   design did not say where `test_gate_run.sh`'s fence harness goes, and the property it guards
   plainly survives the move. A dedicated file mirroring the reference is chosen over folding it
   into the prose-shaped posture test.
3. **The two-vocabulary disambiguation sentence in the SKILL posture is removed, not restated** —
   after re-grounding, the stop axis no longer emits `stopped`, so the collision it defused no
   longer exists.
4. **`gate-execution.md`'s runtime-probe narrowing sentence is rewritten to ADR-0095's uniform
   session guarantee** rather than left pointing at a per-platform narrowing that now lives on
   another page as a historical record.
5. **The evidence carryover is argued a-fortiori**: the native launcher's session guarantee is
   *stronger* than the shell-era shape the verdicts were measured under (ADR-0095), so the verdicts
   hold. The original spec argued "the same detachment"; that is now known to understate it.
6. **Exact field spellings, enum values, and state names are read from source at implementation
   time.** The mapping table's *semantics* are settled; its literal spellings are not, and source
   wins over this document if they differ.
7. **Budget numbers for the new row are not fixed here** — they are set from the measured actuals
   per the ledger's rounding rule, because the file does not exist yet.
8. **The new file names the retired facade without a path spelling**, to satisfy
   `test_consuming_repo_scripts.sh`'s skill-body rule.
9. **`tests/README.md` gains a row for the new suite** if and only if it indexes suites
   individually; verified by reading it at build time.
10. **No ADR is minted or amended** by this change.
