# Design: headless finalize — the finalize-side disposition contract (change 0087)

**Status:** design (interactively groomed 2026-07-18, with the human, in-session)
**Change:** 0087 — Headless finalize — the finalize-side disposition contract, mirroring 0088
**Depends on:** none (`depends_on: []`).
**Related:** 0088 (`done` — the implement-side contract this mirrors), 0008 (`proposed` —
concurrent fan-out; complementary, not a dependency), 0095 (`done` — retired the `auto_approve`
subsystem and reversed ADR-0042, which is what removed this change's original hard part).
**ADRs:** cites ADR-0043 (zero-approvals branch protection as the supported merge path) and
ADR-0011 (the consent model). No new ADR anticipated — see §5.

---

## 1. Context

**Nothing invokes `docket-finalize-change` hands-off today.** Verified 2026-07-18: no driver
surface exists in `scripts/` or `skills/`. A human who wants "close out the merge gate, walk away,
come back to merged PRs" has no way to get it.

This change was originally framed as *"ship a consumer for 0062's `auto_approve` capability"* —
dispatch a bot-approval workflow, poll it, verify `reviewDecision`, then merge. **Change 0095
deleted that subsystem entirely** and reversed ADR-0042. Branch protection requiring a PR with
**zero approvals** now lets a plain `gh pr merge --rebase` land with no `--admin`, no bot, and
nothing for a permission classifier to deny. The approval chain — the part with all the failure
modes — is gone. What remains is invocation.

**Change 0088 already solved the shape of this problem on the implement side**, and its own spec
names this change as the finalize half of the partition (0088 serial self-continuation, #0008
concurrent fan-out, 0087 finalize). Its answer was a driver-agnostic re-invocation contract with
deliberately **no loop primitive and no new entry surface**. This change mirrors it.

**`/loop` is confirmed working** on the implement side (maintainer, 2026-07-18, **CC 2.1.214**):
`/loop docket-implement-next <id-set>` drives real backlog changes cleanly. That retires 0088's
deferred §6 composition spike as a blocker here. Scope the claim honestly — an id-set run ends by
exhausting its set rather than on an empty-backlog `drained`, and neither `contended` (needs two
agents racing) nor `halted` (needs a failure) was exercised. Per the
`harness-behavior-is-mode-and-version-scoped` learning the observation is pinned to 2.1.214, not
to `/loop` in perpetuity.

## 2. Decision

**D1 — Mirror 0088's four dispositions verbatim, including `contended`.**

| Disposition | Finalize meaning | Driver action |
|---|---|---|
| `advanced` | Merged one change → closed out. | continue |
| `contended` | Another writer got there first (the `docket-status` sweep archived it between selection and close-out); the archive is an idempotent no-op, **nothing merged**. | continue — re-select next |
| `drained` | No eligible `implemented` change in scope. | **stop** |
| `halted` | Any abort-and-report point, **or** an eligible set in which every member needs a human (§3.4). | **stop + surface** |

There is no claim CAS here, so `contended`'s *mechanism* differs — but its driver-facing meaning
is identical ("someone else got there, nothing to do, keep going"). Vocabulary parity is the
point: a driver that must know which skill it is running is no longer driver-agnostic, which was
the property justifying 0088's design.

**D2 — One merge per invocation ("single"). Never a batch.** A run merges exactly one change and
exits `advanced`. Consecutive close-outs come from the driver re-invoking. This preserves
finalize's existing blast-radius posture: its Selection matrix already refuses to batch without an
interactive confirmation, and a headless run cannot answer that prompt.

**D3 — Order by *mergeability*, not priority.** The goal is to close out as many changes as
possible per drain, so selection maximizes the chance each attempt succeeds. Ordering keys, in
order:

1. **`depends_on` order — a hard correctness constraint, not a preference.** A dependency is
   satisfied only at `done`, so a dependent can never merge ahead of its dependency regardless of
   how mergeable it looks.
2. **GitHub's `mergeable` field.** `CONFLICTING` is excluded from selection entirely (§3.4).
3. **Smallest diff first** — `changedFiles`, then `additions + deletions`. Smaller changes are
   cheaper to re-test, less likely to break the suite after rebase, and closing them first
   maximizes the count that lands before any halt.
4. **Existing deterministic order — priority → `created` → lowest id — as the final tiebreak.**
   Priority is demoted from primary key to tiebreak rather than deleted: it still encodes human
   intent and it guarantees a total, reproducible order.

**Re-selection replaces sequencing.** Because each invocation re-derives "best next" against the
**current** integration branch, no precomputed order exists to go stale. This is the direct answer
to the moving-base problem — every merge moves the base, so an order authored before the first
merge is a prediction about a base that no longer exists by the third. It also means `mergeable`
is re-queried per invocation, which is exactly when it is accurate.

**D4 — A gate failure is marked with a dated `## Finalize blocked` body section, NOT a new
status.** Mirrors the proven `## Auto-groom blocked` pattern: presence drives a distinct board
cell, the reason lives in prose, and it is cleared on recovery. Rejected alternatives:

- **Reusing `blocked`** would erase true information — the change really *is* `implemented` with
  an open PR, while `blocked` sits in the `in-progress` neighborhood of the lifecycle.
- **An eighth status** would flatten six distinct abort reasons (ambiguous rebase conflict · no
  detectable suite · repair stuck after ≤2 attempts · red/absent CI · rejected
  `--force-with-lease` · autonomous repair sign-off) into one label and drop the part a human
  actually needs, while forcing changes to the lifecycle diagram, the board renderer, the GitHub
  mirror's seven-state mapping, and the health checks.

**D5 — Driver-agnostic; docket owns the contract, not the driver.** No loop primitive, no
`docket-drain` skill, no new entry surface. `/loop`, cron, a scheduled agent, or a human
re-typing the command are all equally valid. Confirms 0088's answer for this side.

## 3. What ships

### 3.1 Terminal disposition contract (skill prose)

A *Terminal disposition (driver contract)* subsection on `docket-finalize-change/SKILL.md`,
structurally parallel to the one in `docket-implement-next/SKILL.md`: the D1 table, the binary
continue/stop rule, and a final report enumerating what was merged, what was skipped and why, and
which disposition ended the run.

Every existing abort-and-report point maps to `halted` — this **names** existing behavior rather
than adding any. The abort set is already enumerated in the skill and does not change.

### 3.2 Id-set scoping

Generalize the existing explicit-id argument to an allowlist: `docket-finalize-change 90,92,94`.
The set bounds **which changes are eligible**; the run still merges only the best-ordered one
(D2). Unset ⇒ every eligible `implemented` change is a candidate. A scoped member that is not
eligible is skipped with its reason in the report.

The explicit-id override of `require_pr_approval` (existing behavior) extends to an id set: naming
the ids *is* the authorization the interactive batch prompt would have collected.

### 3.3 Selection (per invocation)

1. Collect candidates: `status: implemented`, PR open, all `depends_on` at `done`, within the id
   set if one was given.
2. **Exclude any change carrying a `## Finalize blocked` section** — without this, a re-run
   re-selects the same known-bad change forever and the drain never progresses past it.
3. Query `mergeable` per candidate (§3.5), excluding `CONFLICTING`.
4. Sort by the D3 keys; take the head; merge it.

### 3.4 `## Finalize blocked` (the marker)

Written when a candidate cannot proceed without a human — a gate abort on the change being
finalized, or a `CONFLICTING` PR encountered during selection. Shape follows
`## Auto-groom blocked`: a dated entry naming **which** of the six reasons fired and what the
human must do.

- Encountering `CONFLICTING` candidates during selection marks them (cheap, idempotent metadata
  write) so the board surfaces every change needing attention, not only the one this run touched.
- **A non-empty candidate set in which every member is blocked is `halted`, not `drained`** —
  there *is* work, it just needs a human. `drained` must keep meaning "genuinely nothing to do,"
  or the driver's stop signal loses its meaning.
- Board: a distinct cell (proposed wording `finalize blocked — needs you`), parallel to
  `auto-groom blocked — needs you`.
- Clearing: unlike auto-groom's human-only re-arm, a **successful finalize removes the section**
  automatically — the condition is machine-verifiable (the gate passed), so requiring a human to
  delete it would strand stale markers.

### 3.5 Mergeability probe — mechanics

`gh pr view <n> --json mergeable,mergeStateStatus,changedFiles,additions,deletions`.

**GitHub computes `mergeable` lazily: the first query returns `UNKNOWN` and only *triggers* the
computation.** Empirically confirmed 2026-07-18 — all five open docket PRs returned
`UNKNOWN`/`UNKNOWN` on first query and resolved on re-query. The probe must therefore **poll,
bounded**, and treat a still-`UNKNOWN` result as "attempt it" (the rebase-retest gate is the real
arbiter; a wrong guess costs one gate run, not correctness).

**Do NOT build pairwise file-overlap ranking.** The obvious elaboration — compute which PRs share
changed files, merge the least-entangling first — was measured against this repo's real backlog on
2026-07-18 and **discriminates nothing**: the four mergeable PRs (#89, #96, #97, #98) have
**completely disjoint** file sets, so the ranking is a no-op at O(n) extra `gh` calls per
invocation, O(n²) across a drain. Revisit only on evidence.

Note the limit of the signal: file-disjointness proves only the absence of a **textual** conflict.
#96 (`render-board.sh`) and #98 (`board-checks.sh`, `docket-status.md`) are file-disjoint but
semantically adjacent — merging one can still redden the other's suite. Catching that is exactly
what the rebase-retest gate exists for; no static probe substitutes for it.

### 3.6 Documentation

A README subsection beside 0088's *Draining hands-free with `/loop`*, framed identically —
recommended, confirm-in-your-harness — so both halves read as one system.

## 4. Scope boundaries

- **Not** a loop primitive, `docket-drain` skill, or new entry surface (D5).
- **Not** concurrent/parallel fan-out — #0008 owns it, and can key on this vocabulary as it can
  on 0088's.
- **Not** a revision of the rebase-retest gate, `require_pr_approval`, terminal-publish, or the
  consent model (ADR-0011/ADR-0043). This change consumes them.
- **Not** a re-opening of the retired `auto_approve` mechanism — deleted, ADR reversed, no bot
  chain to drive.
- **Not** a change to the seven-state lifecycle (D4 exists precisely to avoid one).

## 5. Open questions — resolved

- **Single vs. drain** → single (D2), maintainer decision.
- **`contended` for finalize** → adopt verbatim (D1), maintainer decision.
- **Ordering** → mergeability-first, priority demoted to tiebreak (D3), maintainer decision.
- **Marking gate failures** → `## Finalize blocked` section, not a status (D4), maintainer
  accepted the recommendation.
- **`/loop` verification** → confirmed live at CC 2.1.214; no spike change needed.
- **docket-owned contract?** → yes, contract only, not the driver (D5).
- **ADR needed?** → not anticipated. Following 0088's precedent, these are design-time decisions
  captured in this spec, not fresh non-obvious implementation-time decisions; with
  `terminal_publish: true` the spec is published to the integration branch at close-out, so the
  rationale is preserved without a restating ADR. If the build surfaces a genuinely new decision,
  the normal `docket-adr` dispatch applies.

**Still open, for the build to settle:** whether the six abort reasons need distinguishing on the
**board** or only inside the section body; and the exact section/cell wording.

## 6. Risk & build-time reconcile item

**Collision risk on the board renderer — check at reconcile.** §3.4's board cell touches the
render path, and two open PRs are in that neighborhood right now: **#96** (change 0093,
archive-decay-digest) modifies `scripts/render-board.sh` + `tests/test_render_board.sh`, and
**#98** (change 0092, orphan-detection-script) modifies `scripts/board-checks.sh` +
`scripts/docket-status.md`. Whichever lands first moves this change's base. Reconcile against
whatever is on the integration branch at build time and compose rather than choose (learning:
`concurrent-edits-compose-at-rebase`).

**`mergeable` is advisory, not a guarantee.** It reflects textual mergeability against the base at
query time; the base moves after every merge, and semantic breakage is invisible to it. The gate
remains the arbiter — a mergeability miss costs one wasted gate run.

**Harness version scoping.** The CC 2.1.214 `/loop` observation and any classifier posture are
version-scoped (learning: `harness-behavior-is-mode-and-version-scoped`). The old CC 2.1.211 pin
lived in ADR-0042, which ADR-0043 reversed — if this change relies on a pinned posture, it needs a
new home for the pin. Whether the driver re-verifies posture per run, or simply `halted`s on
denial, is a build-time call.

**Skill size budget.** `tests/test_skill_size_budgets.sh` carries a row for
`skills/docket-finalize-change/SKILL.md`; §3.1/§3.2 prose will likely exceed it. The guard
explicitly permits an in-diff raise — raise it in the same diff, as 0088 did.

## 7. Testing / verification

- **Sentinels** (`tests/test_finalize_disposition.sh`, mirroring `tests/test_loop_continuation.sh`):
  all four disposition words present in the skill; the binary continue/stop rule stated; every
  abort-and-report point mapped to `halted`; id-set scoping documented; the
  `## Finalize blocked` section named. Sentinels sample vocabulary — they do not parse meaning, so
  pair with a whole-branch review (learning: `foundational-test-discipline`).
- **Each sentinel must be non-vacuous** — written to flip to `NOT OK` if the clause it guards is
  removed (learning: `guards-are-code`), and mutation-verified at build.
- **Board cell**: a render test asserting a change carrying `## Finalize blocked` renders the
  distinct cell, and one without it does not.
- **Selection ordering**: a fixture-driven test over synthetic candidates asserting `depends_on`
  order dominates mergeability, and that diff size orders within `MERGEABLE`.
- **Whole suite green** at the build gate, not only the tests this spec names (AGENTS.md).
- **Human verification** at the merge gate: drive `/loop docket-finalize-change` against a real
  multi-change backlog and confirm it merges one per iteration, stops on `drained`, and surfaces a
  `halted` reason on the PR.
