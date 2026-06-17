# Design: finalize — rebase-onto-base + re-run-tests gate before merge

**Status:** design (brainstormed 2026-06-17 via `docket-groom-next`, with the human)
**Change:** 0015
**Depends on:** none. **Related:** change 0016 (`done`, the agent layer — ADR-0008), change 0017 (`done`, composition wiring — ADR-0009; this change reuses its named-subagent-dispatch pattern).

## 1. Context / problem

`docket-finalize-change` merges an approved PR by trusting the PR's **own** CI — which was green on the PR **head**. `gh pr merge --merge` only blocks *textual* conflicts. So a PR that is **behind base** can pass its own CI and still produce a logically-broken integration branch once merged — a **semantic conflict** git auto-merges cleanly (e.g. base renamed a symbol the PR still calls, or changed a contract the PR relied on). Nothing re-validates the **merged result** before it lands. Finalize's only test step today is a parenthetical *optional* ("verify the merge landed (optionally: tests green on the merged result)"), so the effective gate is "the PR head was green when a human approved it."

This change adds a **rebase-onto-base + re-run-tests gate** to finalize's merge step: bring the feature branch up to base, validate the integrated result, and only merge if green — repairing what's repairable along the way, and otherwise abort-and-report.

## 2. Scope

**In scope.** The gate guards finalize's **merge step** — the `Approved + mergeable but not merged → merge it` path in `docket-finalize-change`'s per-change step 1, which is the **only place docket itself performs a merge**.

**Out of scope:**

- **The merge *mode*** — merge vs squash vs rebase-merge stays the team's `gh` flag choice (unchanged).
- **The `docket-status` bulk sweep** — it only **archives PRs that are already merged** (a human merged via the GitHub button); it never performs a merge, so a *pre-merge* gate has nothing to act on there. The gate is inherently **finalize-only**. (A one-line note is added to the sweep docs stating this; GitHub-button merges bypass the gate by nature — outside docket's control.)
- **Asserting the resolver/repair agents' *quality*** — conflict-resolution correctness and repair correctness are judgment, governed by the agents' `opus/xhigh` tier; the test suite asserts the gate's **mechanics** (config parse, mode dispatch, abort paths), not "did it merge correctly."
- **The already-merged finalize path** — if the PR is already merged when finalize runs, there is nothing to gate (the merge landed); straight to archive as today.

## 3. Configuration — new `.docket.yml` keys

```yaml
finalize:
  gate: local          # local (default) | ci | both | off
  test_command:        # OPTIONAL override; unset ⇒ the agent auto-detects the suite
```

- **`gate` default is `local`** — the gate is **on by default**, validating against the repo's local suite (most repos, including docket itself, run tests locally; docket has no GitHub CI). `ci` validates against GitHub checks; `both` requires local **and** CI green; **`off`** restores today's behavior exactly (merge trusting the PR's CI, with the existing optional post-merge test) for a repo that wants to opt out.
- **`test_command` is normally unset** — the finalize agent **auto-detects** how to run the suite by inspecting the repo (Makefile, `package.json` scripts, a `tests/` dir, CI config, etc.). The override exists only as an escape hatch when auto-detection guesses wrong; when set it is used verbatim.
- This change sets **docket's own `.docket.yml`** to `finalize: { gate: local }` to dogfood the gate.

Backward-compat note: a repo that has been finalizing happily until now will, on upgrade, start rebasing+validating by default. That is the intended safety upgrade; `gate: off` is the documented opt-out.

## 4. The gate flow

Runs inside finalize's merge step, **before** `gh pr merge`. Operates in the change's feature worktree (`.worktrees/<slug>`) if it still exists, else a transient worktree on `feat/<slug>` (provisioned and torn down like terminal-publish's `pub-<T>` tree).

```
gate == off ?  ──yes──▶  merge as today (no rebase, no re-test)
   │ no
   ▼
1. git rebase feat/<slug> onto origin/<integration_branch>
     ├─ clean ───────────────────────────────────────────────┐
     └─ conflict → dispatch docket-rebase-resolver (ability ①)│
          ├─ resolves every hunk → rebase completes ──────────┤
          └─ ambiguous → git rebase --abort → ABORT-AND-REPORT │
                                                               ▼
2. determine the suite: test_command override, else auto-detect
     └─ local/both and no suite found → ABORT-AND-REPORT
3. validate per `gate`:
     local → run the suite in the worktree (BEFORE any push)
     ci    → push --force-with-lease, poll `gh pr checks`
     both  → local first, then push + CI
     ├─ green ─────────────────────────────────────────────────┐
     └─ red → dispatch docket-integration-repair (ability ②)    │
          ├─ reaches green in ≤2 attempts → SIGN-OFF GATE (§6)   │
          └─ stuck / can't reach green → ABORT-AND-REPORT        │
                                                                 ▼
4. push --force-with-lease (if rebased and not already pushed)
     └─ lease rejected (concurrent push) → ABORT-AND-REPORT
5. gh pr merge --merge  →  existing close-out
     (harvest → archive → terminal-publish → cleanup → board)
```

**Ordering rationale.** `local` runs the suite **before** the force-push, so a broken rebase is never force-pushed; `ci` necessarily validates **after** the push (CI runs on the pushed branch); `both` = local-before-push **and** CI-after. The rebase makes the feature sit on top of base, so the eventual `gh pr merge --merge` is conflict-free — validating the rebased branch is validating what actually lands.

## 5. The two agents (split at rebase-completion)

Conflict resolution and semantic repair are **different shapes** — a bounded *reconciliation* vs an open-ended *debugging + implementation* — so they are two dedicated wrappers with a crisp boundary: **the rebase completing.** Both `opus/xhigh`, both wrap **no skill**, both load only `docket-convention`; both auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob (config keys `rebase-resolver`, `integration-repair`; no generator edit). Both carry abort-and-report.

### ① `agents/docket-rebase-resolver.md` — resolve conflicts, during the rebase

Dispatched (foreground) only when `git rebase` stops with conflict markers. Charter: reconcile each conflicted hunk using merge-intent judgment (keep one side, or synthesize both — *what did base change, what does the PR intend*), `git add` + `git rebase --continue` through every conflicted commit until the rebase completes. **It does not run tests.** Edits are confined to conflicted regions. Abort-and-report (`git rebase --abort`) only when a conflict is genuinely ambiguous (it cannot tell which intent is correct without guessing). It makes the rebase **land**.

### ② `agents/docket-integration-repair.md` — make the suite pass, after the rebase

Dispatched (foreground) only when, after the rebase has landed, the suite is **red**. Charter: own **every** red-test outcome regardless of cause — genuine base drift *or* a bad ① resolution it can see in the git state — using systematic-debugging discipline: find the root cause, write a **minimal** fix, never game/weaken the tests, re-run. Bounded to **≤2 attempts**; if it cannot reach green it aborts-and-reports with a diagnosis. It makes the suite **pass**. Because its output is code the human's PR review never saw, a successful repair is gated by §6.

Each agent's report distinguishes the work: `conflicts_resolved` (①) vs `authored_repair` (②) — `authored_repair: true` is what fires the §6 sign-off.

## 6. Sign-off on auto-authored repairs (resolves open-question #4)

A ② repair is code the human's approval predated, so it **never merges unseen**. How "wait for sign-off" resolves depends on how finalize was invoked — reconciling with ADR-0008's *abort-and-report for autonomous subagents* rule:

- **Interactive finalize** (human-attended session, e.g. a person running `/docket-finalize-change`): force-push `--with-lease` the repaired branch, **report the repair diff + what broke**, and **prompt** for go-ahead before `gh pr merge`.
- **Autonomous finalize** (running as its own `sonnet/medium` subagent, no human to ask): it **cannot** prompt, so it **force-pushes the repair and aborts-and-reports** — STOP, do not merge. The human reviews the pushed repair on the PR and re-runs finalize to merge.

Pure ① conflict resolution does **not** trigger this gate — it is completing the merge the human already intended; it flows through finalize's normal merge path (which, under auto-detect, already prompts before merging).

## 7. abort-and-report points (the full set)

Each leaves the **PR open** and the **change `implemented`**, and surfaces a clear reason: ambiguous rebase conflict (① gives up) · `local`/`both` with no detectable suite and no `test_command` override · ② cannot reach green in ≤2 attempts · `ci`/`both` with red or absent CI checks · `push --force-with-lease` rejected by a concurrent push · any ② repair under **autonomous** finalize (§6).

## 8. Touches / testing

**Touches:**
- `skills/docket-finalize-change/SKILL.md` — the gate (the new flow inside step 1), the two agent dispatches, the §6 sign-off, the abort-and-report set.
- `.docket.yml` — the example `finalize:` block + docket's own `gate: local`.
- `skills/docket-convention/SKILL.md` — document the `finalize.gate`/`test_command` config; extend the Agent-layer + Composition sections for the two new wrappers; bump wrapper-count prose **6 → 8** (five skills + the critic + these two; the "five *skills* get a wrapper" line stays exact — these wrap no skill).
- `skills/docket-status/SKILL.md` — one-line "the rebase-retest gate is finalize-only; the sweep only archives already-merged PRs" note.
- `agents/docket-rebase-resolver.md`, `agents/docket-integration-repair.md` — the two new wrappers.
- `tests/test_sync_agents.sh` — wrapper count **6 → 8** (both `= "6"` asserts → `8`) + structural asserts for both wrappers (opus/xhigh; `skills:` includes `docket-convention`, excludes any docket skill; abort-and-report present; per-repo override + `--check` for at least one key).
- `tests/test_finalize_gate.sh` (new) — sentinels that finalize's merge step gates on `finalize.gate`, dispatches `docket-rebase-resolver` on conflict and `docket-integration-repair` on red tests, runs local tests before the force-push, and names every abort-and-report path; plus config-parse coverage for the four `gate` modes and the `off` backward-compat path.

**Testing strategy (per LEARNINGS):** sentinels are sampling, not parsing — pair with the whole-branch review (#5). Prove each new assert non-vacuous (#2). Adding two wrappers is a stale-count trap — grep the repo for the old count words (#5/#14). Honor the SIGPIPE-safe capture-then-grep idiom and `--force-with-lease`/here-string patterns (#11/#16). Don't restate model/effort literals in the skill-body dispatch prose — name the source (the #17 lesson); guard with the same regex assert.

## 9. ADR

Two decisions here are non-obvious enough to record at build (decide Update-vs-new at build, per the #16/#17 precedent):

- **The gate splits conflict-resolution from semantic-repair into two pinned agents** with a rebase-completion boundary — likely a **new ADR** (a distinct architectural choice with its own consequences: two wrappers, the boundary rule, ② owning all failures).
- **finalize may author a repair, gated by sign-off (interactive prompt / autonomous abort-and-report)** — possibly folded into the same ADR, as it is the same "finalize gains a judgment-tier subagent" decision; it extends ADR-0008's abort-and-report rule rather than reversing it (record as an Update to ADR-0008 if it reads that way at build).

Append the resulting number(s) to this change's `adrs:`.

## 10. Decisions (resolved at brainstorm, with the human)

- **Gate default is `local`** (on by default); `off` is the documented opt-out; `ci`/`both` for repos with GitHub CI.
- **Test command is auto-detected**, with an optional `test_command` override.
- **Mechanism is rebase + `--force-with-lease`** (the human's normal manual flow), not a local merge-preview; the gate validates the rebased branch.
- **Two dedicated agents** (`docket-rebase-resolver` ①, `docket-integration-repair` ②), split at rebase-completion; ② owns every red-test outcome regardless of cause.
- **finalize repairs broken features** (≤2 bounded attempts), but an auto-authored repair **never merges unseen** — interactive prompt or autonomous abort-and-report (§6).
- **The gate is finalize-only** — the sweep never merges, so it has nothing to gate.
- **Out of scope:** merge mode, the sweep's archive path, asserting agent *quality* (governed by `opus/xhigh`).
