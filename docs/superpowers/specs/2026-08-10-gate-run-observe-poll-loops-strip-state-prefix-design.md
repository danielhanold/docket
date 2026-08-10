<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0286 — Caller-authored gate-run --observe poll loops strip the state= prefix and never terminate](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-10-0286-gate-run-observe-poll-loops-strip-state-prefix.md)**
<!-- docket:backlink:end -->

# gate-run --observe caller poll loops — teach the canonical loop, keyed on the printed form

**Change:** 0286 · **Type:** fix · **Priority:** high · **Date:** 2026-08-10 (auto-groom)

## Problem

`gate-run --observe` prints exactly one machine-readable line: `state=<state>` (plus `cause=<cause>`
for `died`). The caller owns the polling loop — deliberately; that is a stated invariant of
`scripts/gate-run.md`. But nothing in the contract or in `skills/docket-build/SKILL.md`'s *Gate
execution posture* shows a caller **what a correct loop looks like**, so agents invent one. A live
invented loop did `awk '{print $1}'` on the observe output and then matched **bare tokens**
(`passed|failed|died|stopped|unavailable|running`) — but the first field *is* `state=passed`, so
every observation fell to the `*)` arm and a finished gate looked unfinished until the
`GATE_OBSERVATION_BUDGET` (30 min) was burned or a human killed the loop.

The helper told the truth; the caller could not read it. The fix therefore lives in the **taught
surface**, not in the helper.

## Design

Three edits, one defect class, no behavior change to `gate-run.sh` itself:

1. **Canonical poll loop in `gate-run.md`** — a new short subsection under *Usage* ("The caller's
   loop") carrying one copy-paste-correct fenced example that:
   - captures `out=$("$…"/docket.sh gate-run --observe "$run_dir") || true` (capture-then-match,
     never a producer piped into an early-exiting consumer, per the promoted pipefail rule; the
     `|| true` neutralizes `--observe`'s exit-1-on-`unavailable` so an errexit caller still
     reaches the fail-closed arm — callers key on the report line, never the exit code);
   - matches the **whole printed line by its prefix**: `case "$out" in state=running*) …retry…;;
     state=passed*) …;; state=failed*) …;; state=died*|state=stopped*|state=unavailable*)
     …terminal…;; *) …treat as unavailable, stop polling…;; esac`;
   - bounds itself by `GATE_OBSERVATION_BUDGET` and sleeps between observations;
   - states the anti-pattern in one line beside it: *never re-tokenize the report line
     (`awk '{print $1}'`, `cut`, stripping `state=`) and match bare state names — key on the exact
     printed `state=` form.* The unknown-line arm is fail-closed (stop polling, treat as
     `unavailable`), matching the contract's own closed-vocabulary posture — it can never be a
     silent retry arm, which is exactly the observed defect.
2. **`skills/docket-build/SKILL.md` § Gate execution posture** — one added sentence at the "Key the
   wait on the state each observation reports" paragraph: reuse the canonical loop from
   `gate-run.md` verbatim rather than authoring one, and key each `case` arm on the full
   `state=<name>` printed form — a loop that matches bare tokens never terminates.
3. **Guard** — the doc example is executable surface (learnings: `agent-executed-markdown-is-code`),
   so `tests/test_gate_run.sh` gains an assert that extracts the fenced canonical-loop block from
   `gate-run.md` and proves, against stubbed observe outputs (`state=passed`, `state=running` then
   `state=failed`, `state=died cause=vanished`, a malformed line), that the loop terminates with the
   right disposition on every terminal state and retries only `running`. Two mutation keys, each
   matched to the failure it actually produces: (a) change `state=passed*` to bare `passed` — with
   the fail-closed `*)` arm, `state=passed` then lands in `*)` and the loop terminates immediately
   with the **wrong disposition** (`unavailable` instead of `passed`), which is what the assert
   keys on; (b) change the `*)` arm to a retry (the observed defect shape) — the assert reddens on
   **non-termination**, bounded by the fixture's own small budget (a few iterations), never the
   30-min production one. A second, cheaper sentinel pins the SKILL.md sentence to its claim
   (phrase bound to the `state=` requirement, not mere presence — learnings:
   `prose-guard-binds-phrase-to-claim`).

## Assumptions

1. **Teaching + canonical example, not a helper-owned `--wait`.** Weighed: (a) ship a
   copy-paste-correct loop and teach it (chosen); (b) add `gate-run --wait` owning the budgeted
   poll; (c) both. Rejected (b)/(c): "the helper never polls for the caller" is a **stated
   invariant** of `gate-run.md` — every verb is a short call — and the caller's yield-vs-block
   posture (SKILL.md clause 4) depends on who is polling; a helper-internal wait would be a
   foreground call the harness's 600s Bash ceiling kills mid-suite, reintroducing the exact
   problem gate-run exists to solve. Reversing that invariant is a contract/ADR-level decision the
   stub's own open question flags; nothing here forecloses a later human-driven `--wait`.
2. **The canonical loop lives in `gate-run.md`; SKILL.md points at it and restates only the one
   keying rule.** Weighed: full loop duplicated in SKILL.md vs pointer. Chosen pointer + one-line
   rule: `gate-run.md` is the contract callers are already told to read, and a second full copy
   is the restatement class the learnings ledger warns accumulates its own guards. The one
   restated sentence (key on the full printed form) is the load-bearing rule at the site where
   loops are actually authored.
3. **Match shape is prefix-match on the whole line (`state=passed*`), not extraction
   (`${out%% *}` or `${out#state=}`).** Prefix-match keys on the exact printed form, tolerates the
   optional `cause=` suffix (no state name is a prefix of another, and the helper-side closed
   vocabulary blocks the `state=failedX` class), and cannot be silently broken by a re-tokenizing
   edit; extraction re-introduces a parsing step that can drift. **The capture must neutralize the
   exit status** — `out=$(… --observe "$run_dir") || true` (or equivalent) — because `--observe`
   exits 1 on `unavailable` and the contract's own rule is "callers key on the stdout report line,
   never on the exit code"; under the `set -euo pipefail` an agent-authored loop typically carries,
   a bare capture aborts the caller before the fail-closed `*)` arm ever runs. The canonical
   example carries that guard and one line saying why.
4. **The unknown-line arm is terminal (treat as `unavailable`), never a retry.** The contract's
   vocabulary is closed and validated helper-side; a line outside it means something is wrong
   with the invocation or environment, and retrying it is the observed infinite-loop failure mode.
   Fail-closed matches the posture clause 6 of the gate posture already requires.
5. **No change to `gate-run.sh`, its state vocabulary, or the `state=` print format** — callers
   and tests already key on it (stub's out-of-scope, honored).
6. **`runner-dispatch --observe` callers are out of scope.** Same defect class exists for the
   delegation loop, but that surface is being reworked by 0277/0284 with its own vocabulary;
   folding it in would collide. Recorded as `related:` (282, 284 already present; 277 added as an active
   rework of the same runner-dispatch surface). No `depends_on` — nothing here waits on any of them.
7. **Test bound:** the executable-example fixture uses a stub observe function and a
   fixture-local budget of a few iterations, so mutation key (b)'s non-termination reddens in
   milliseconds, never by waiting out a real budget; mutation key (a) reddens as a wrong terminal
   disposition and needs no budget at all.

## Out of scope

- `gate-run.sh` behavior, the six-state vocabulary, the `state=` format.
- A `--wait` verb (human/ADR-level; see assumption 1).
- `runner-dispatch --observe` and the delegation observation loop (0277/0284).
- The budget values themselves (0273).

## Files touched

- `scripts/gate-run.md` — new *The caller's loop* subsection with the canonical example +
  anti-pattern line.
- `skills/docket-build/SKILL.md` — one sentence in *Gate execution posture*.
- `tests/test_gate_run.sh` — executable-example assert + mutation key; SKILL.md sentence sentinel
  (or `tests/test_docket_build_skill.sh` if that is where SKILL.md prose guards already live —
  builder places it beside the existing guards for that file).
