# Design: claim leases + reclaim script (change 0089)

**Status:** design (auto-groomed 2026-07-17 by `docket-auto-groom`; default-biased self-brainstorm, critic-gated)
**Change:** 0089 — Claim leases + reclaim script — expired in-progress claims self-heal back to proposed
**Depends on:** none (`depends_on: []`).
**Related:** 0023 (`done` — introduced `board-checks.sh`, the health-check plumbing this extends; the shared `scripts/lib/docket-frontmatter.sh` it sources was introduced by change 0022), 0088 (needs-brainstorm, concurrently groomed — loop continuation; this change makes a crashed loop iteration self-heal but does **not** depend on it).
**Binding capture constraint (human-stated):** the reclaim mechanism MUST be a deterministic script (ADR-0012 script-vs-model boundary), not model prose.

> This spec was authored autonomously. Every design decision, its chosen default, the rejected
> alternatives, and the reasoning are recorded in **§7 Assumptions** — the deferred human audit
> trail. A spec is emitted (build-ready) only because every one of those decisions was defaultable
> to the conservative option from existing precedent + the stub's own framing; none required human
> context. The adversarial critic gate re-checked exactly that before emission.

---

## 1. Context

A crashed `docket-implement-next` run leaves its change stuck at `status: in-progress` forever.
`docket-status`'s `stale-in-progress` health check *flags* long-idle claims but nothing *recovers*
them, and the flag has a blind spot: it keys on the feature **branch's** newest commit age (>3 days)
and requires the branch to exist locally — so a crash in the window **after the claim commit but
before the branch is pushed** is invisible (the explicit carve-out in `board-checks.sh`:
"branch set but the branch does not exist ⇒ not stale"). The documented recovery is itself a
trap docket has hit in practice: resuming `implement-next` without an explicit id silently claims a
*different* change, because Step-1 selection skips `in-progress`.

Beads (the competitive-review source) solves this with a **lease**: `bd claim` stamps a TTL,
`bd reclaim` reverts `in_progress` issues with expired leases back to `ready`, and crashed agents
self-heal. docket has the failure mode but not the healing. This change adds the near-free markdown
equivalent: a `claimed_at:` timestamp stamped at claim time, a config-layered lease TTL, and a
**deterministic reclaim script** that flips expired in-progress claims back to `proposed` so the
queue self-heals.

## 2. Goal / non-goals

**Goal.** Make an expired in-progress claim self-heal: stamp a claim lease, detect expiry
deterministically (independent of whether a branch was ever pushed), and flip the change back to a
build-ready `proposed` state that re-enters the selection pool — dissolving both the
stuck-forever failure and the resume-claims-a-different-change trap.

**Non-goals (from the stub's Out of scope, carried verbatim in intent):**
- No heartbeat daemon — docket has no resident process; lease refresh (if any) rides existing
  status-transition commits.
- No killing/rewinding the crashed run's feature branch — **reclaim touches metadata only**.
- No loop-continuation (#0088) or parallel-drain (#0008) semantics — this makes them safer, it does
  not implement them.

## 3. Design overview

Four pieces, each defaulted in §7:

1. **`claimed_at:` frontmatter field** — a UTC-8601 timestamp stamped by `docket-implement-next`'s
   existing claim commit (Step 2), and **re-stamped at the phase-boundary metadata commits
   implement-next already makes** (reconcile, `implemented`, `pr:` …) as a zero-cost poor-man's
   heartbeat. Cleared when the change leaves `in-progress` (terminal transition or reclaim).
2. **A `reclaim:` config block** — `lease_ttl` (generous default; §7-D) and `auto` (default
   `false`; §7-E), resolved by `docket-config.sh --export` (as `RECLAIM_LEASE_TTL` /
   `RECLAIM_AUTO`) exactly like `finalize:` / `learnings:` / `auto_groom`, shipped end-to-end
   (sample `.docket.yml` + README + prose) per the `config-knob-ship-end-to-end` learning.
3. **A new deterministic script `scripts/reclaim-claims.sh`** (with its co-located
   `scripts/reclaim-claims.md` contract) — the constraint's "script, not model prose." It sweeps
   `active/*.md` for `status: in-progress` changes whose `claimed_at:` lease is expired **and that
   have no existing feature branch** (no `feat/<slug>` ref on origin or locally) — precisely the
   crashed-before-push blind spot §1 identifies, and the one case where reclaim is provably
   collision-free and orphan-free. For each such change it: appends a dated `## Reclaim log` entry,
   flips `status:` back to `proposed`, clears `branch:` and `claimed_at:`, resets
   `reconciled: false`, refreshes the `## Artifacts` block, and commits + pushes a mechanical
   reclaim commit on `metadata_branch` (ADR-0021 pipeline-script-authored mechanical commit) under
   the standard docket CAS/re-read discipline. Sourced from `scripts/lib/docket-frontmatter.sh`
   like its sibling `board-checks.sh`. An expired in-progress change **with** a pushed branch is
   never auto-reclaimed (it may carry real work) — the upgraded `stale-in-progress` flag surfaces
   it for a human, and adopt-or-supersede of that branch is a recommended follow-up (§7-C).
4. **`docket-status` wiring, opt-in for mutation.** Detection/recommendation is **always on**: the
   `stale-in-progress` health check is upgraded to *also* key on `claimed_at:`+TTL expiry (catching
   the crashed-before-branch case), and `docket-status` prints a state-valid recommended reclaim
   command (`printed-remedy-state-validity` learning). **Mutation** runs only when the repo opts in
   (`reclaim.auto: true`) or a human runs `docket.sh reclaim` explicitly — the default
   `docket-status` pass stays warn-only / non-mutating, preserving ADR-0012 ("scripts never mutate
   state autonomously") and `board-checks.sh`'s "warn-only, never auto-fixes" invariant.

## 4. Reclaim mechanics (the deterministic script)

For each `active/*.md` at `status: in-progress`:

- **Eligibility probe (fail-safe, two conditions).** Reclaimable iff **(1)** `claimed_at:` is present
  AND `NOW - claimed_at > lease_ttl`, **AND (2)** no feature branch exists (no `feat/<slug>` ref
  resolves on origin or locally). A change with **no** `claimed_at:` (pre-migration, or an anomaly)
  is **never** reclaimed — there is no positive evidence of expiry (`idempotency-keying`: key on the
  state you actually have). A change **with** a pushed branch is **never** reclaimed by this script —
  it may carry real commits, and clearing `branch:` would orphan the branch and collide with the next
  claimant's Step-4 `git worktree add -b feat/<slug>` (see §7-C). Both excluded cases stay surfaced
  by the upgraded `stale-in-progress` flag for a human. `NOW` and `GIT` are mock seams, mirroring
  `board-checks.sh`.
- **State transition (no-branch case only).** Append a dated `## Reclaim log` entry recording the
  reclaim (claim age, that no branch ref was found); then `status: in-progress → proposed`; clear
  `branch:` (populated with `feat/<slug>` at claim Step 2 — reclaim reads it to derive the ref name
  it probes in eligibility condition (2), then clears it here) and `claimed_at:`; set
  `reconciled: false` (fresh reconcile on re-claim — hygiene, since the world moved); set
  `updated: <UTC today>`. `spec:`/
  `trivial:` are untouched, so the change is build-ready again. (`presence-encoded-state`: every
  field encoding "claimed" is removed on the transition out.) Because only the no-branch (pre–Step-4)
  window is ever reclaimed, there is no half-built branch to absorb — the reconcile-adoption problem
  is designed out, not deferred into a pass that cannot solve it.
- **Concurrency (CAS).** Commit + push on `metadata_branch`; on non-fast-forward, re-sync and
  **re-read** — reclaim a change only if it *still* meets both eligibility conditions after the
  re-read (a concurrently re-claimed, advanced, or now-branched change is skipped). Mirrors the
  implement-next claim loop and ADR-0004's final-push-CAS stance.
- **Report.** One structured line per reclaimed (or skipped-because-raced) change, surfaced by
  `docket.sh`/`docket-status`.

## 5. Why this dissolves the resume trap

Once reclaimed, the change is `proposed` and build-ready again, so a bare `docket-implement-next`
re-selects it through normal Step-1 selection — no explicit id needed, no risk of silently claiming
a different change. The trap existed only because Step-1 skips `in-progress`; reclaim removes the
change from that state.

## 6. Build checklist (proposal altitude — the plan owns the breakdown)

- `claimed_at:` added to the change manifest (convention doc) + stamped/refreshed by
  `docket-implement-next` (claim + phase-boundary commits) + cleared on terminal close-out.
- **Convention lifecycle:** sanction the new `in-progress → proposed` reverse edge (reclaim) in the
  convention's "Lifecycle — seven states" section (diagram + rules prose), so the transition is a
  documented part of the contract, not a silent one (§7-B).
- `## Reclaim log` added to the convention's "Change body sections" list.
- `reclaim:` block in `docket-config.sh` (+ `--export` of `RECLAIM_LEASE_TTL` / `RECLAIM_AUTO`),
  the commented sample `.docket.yml`, and README.
- `scripts/reclaim-claims.sh` (eligibility = expired lease **AND** no existing feature branch) +
  `scripts/reclaim-claims.md` + `docket.sh reclaim` facade routing.
- `stale-in-progress` upgraded (claimed_at+TTL) in `board-checks.sh`; `docket-status` prints the
  recommended reclaim command; `docket-status` invokes reclaim mutation only under `reclaim.auto`.
- `tests/test_reclaim_claims.sh` (hermetic, via the `NOW`/`GIT` seams), matching
  `tests/test_board_checks.sh`; regression tests asserting (a) a non-opted-in repo mutates nothing,
  and (b) an expired lease **with** a pushed branch is left untouched (no orphan, no collision).

## 7. Assumptions (the deferred human audit trail)

Each decision an interactive brainstorm would have raised: the chosen default, the rejected
alternatives, and why. All were defaultable — none needed human context (see §8).

**A. Lease field shape — `claimed_at:` timestamp only, no claim identity.**
Chosen: a single `claimed_at:` UTC-8601 frontmatter field. Rejected: (a) also stamping a claim
identity (agent/host id), (b) a richer nested lease object. Why: reclaim keys on time-since-claim,
not *who* claimed; docket already resolves concurrent claimants by final-push CAS (ADR-0004), so an
identity adds machinery for no reclaim benefit. A dedicated field (not the existing `updated:`,
which every edit touches) is required for a reliable claim-age signal.

**B. Reclaim target = `proposed`, no new lifecycle status — but the reverse edge is sanctioned in
the convention.** Chosen: flip back to `proposed` with `spec:` intact (build-ready again),
`reconciled: false`, and **document the new `in-progress → proposed` reverse edge** in the
convention's seven-state lifecycle (diagram + rules), since the current diagram has no such edge and
a silent transition would be a contract gap (build checklist §6). Rejected: a distinct
`reclaimed`/`ready` status or marker field. Why: the stub itself states "docket's equivalent is
`proposed` with spec intact"; an eighth status adds machinery + stale-state cleanup for no benefit —
exactly the trade ADR-0004 rejected for grooming. This adds a transition to the existing state, not a
new state.

**C. Reclaim is narrowed to the no-existing-branch case; the has-branch case is deferred.**
*(Revised after the critic gate — the original "reconcile absorbs the half-built attempt" default
was factually wrong and is retracted.)* Chosen: reclaim mutates **only** an expired-lease change
that has **no** feature branch (no `feat/<slug>` on origin or locally). Rejected defaults: (a) the
original claim that implement-next's reconcile adopts the prior branch — **false**: reconcile
(Step 3) refreshes spec/design drift only and never reads the feature branch, which is created at
Step 4; (b) clearing `branch:` on a change with a pushed branch — this **orphans** that branch and
makes the next claimant's Step-4 `git worktree add -b feat/<slug>` **collide** on the existing
branch, hard-aborting the next autonomous build on exactly the highest-value case (a crash *after*
real work landed). Why the narrowing is safe and sufficient: §1's whole motivation is the
crashed-*before*-branch blind spot that the current branch-age check explicitly misses, and that
case has no branch to orphan and no branch to collide with — reclaim is provably safe there. An
expired lease *with* a branch is already flagged by the (upgraded) `stale-in-progress` health check
for a human; bringing branch **adopt-or-supersede** into the autonomous path is genuine judgment and
is a **recommended follow-up change**, not part of this one (and out of this stub's scope: "reclaim
touches metadata only … killing or rewinding the crashed run's feature branch" is a non-goal).

**D. Lease TTL — a single generous config default, not per-priority.**
Chosen: one `reclaim.lease_ttl`, config-layered, default generous and **≥ the existing 3-day
`stale-in-progress` window** so reclaim never fires before the health check even flags (proposed
default: 72h, matching that precedent). Rejected: per-priority TTLs (YAGNI — no demonstrated need,
adds config surface). Why defaultable: it is a config knob the human can override, and — with auto
OFF by default (§7-E) — the number only shifts *warn-only* detection, so it is a reversible starting
point, not a one-way door. *(Corrected after the critic gate:)* the false-reclaim hazard is bounded
not by "phase-boundary refresh makes the number irrelevant" — the build phase (Step 5) makes **zero**
metadata commits and so gets **no** heartbeat — but by the §7-C **no-branch narrowing**: only the
short pre–Step-4 (claim → reconcile) window is ever reclaimed, and *any* change that reached the
build phase necessarily has a feature branch and is therefore outside reclaim's scope entirely on the
clone running reclaim (the cross-machine local-only-ref window is the documented §7-H residual). The
phase-boundary re-stamp (§3-1) still helps for the claim→reconcile window; it is no longer leaned on
as the safety argument.

**E. Auto-reclaim posture — OFF by default; flag + recommend by default, mutate only on opt-in.**
Chosen: default `docket-status` detects + recommends (warn-only, unchanged posture); actual
mutation runs only under `reclaim.auto: true` or an explicit `docket.sh reclaim`. Rejected:
unconditional auto-reclaim inside every `docket-status` sweep. Why: ADR-0012 mandates scripts
"never mutate state autonomously" and `board-checks.sh` is "warn-only, never auto-fixes"; defaulting
mutation ON would be a posture reversal needing human blessing, whereas defaulting OFF is strictly
additive to today's behavior and needs none. `opt-in-signal-not-file-presence`: gate mutation on an
explicit key, never on config-file presence.

**F. New `## Reclaim log` body section (not reuse of `## Reconcile log`).**
Chosen: a new dated body section parallel to `## Reconcile log` / `## Auto-groom blocked`, appended
by the reclaim script. Rejected: appending to `## Reconcile log`. Why: the convention scopes
reconcile-log entries to "the implementer's reconcile pass" (a different author); a dedicated
section keeps authorship clean and is the established pattern for dated body logs. Cost: one added
line in the convention's section list (shipped in this change).

**G. Detection keys on `claimed_at:`+TTL; the existing branch-age check is complemented, not
replaced.** Chosen: reclaim expiry = `claimed_at:`+TTL (catches the crashed-before-branch case the
current check misses); `stale-in-progress` is upgraded to also consider `claimed_at:` while keeping
its branch-age signal. Rejected: keying reclaim on branch age (inherits the no-branch blind spot).
Why: the whole point is to catch a crash before the branch exists.

**H. Config knobs are behavioral, not coordination-fenced.**
Chosen: `reclaim.lease_ttl` / `reclaim.auto` resolve through the normal per-field config layering
(repo-local > repo-committed > global > built-in), like `auto_groom` / `finalize.gate` /
`learnings.*`. Rejected: fencing them per-repo (ADR-0019). Why: they do not write shared,
non-re-derivable state; they are the same category as `auto_groom` (which governs autonomous writes
and is not fenced). Residual coordination consideration (noted for the human): a global
`reclaim.auto: true` reclaims *destructively* (clears `branch:`) where `auto_groom` only writes
additively, and the §7-C no-branch narrowing removes *most* of the teeth — but not all. Eligibility
condition (2) probes git **refs**, and a build on machine A between claim (Step 4, the `feat/<slug>`
ref is created **locally**) and push (Step 7) has a **local-only** ref that another machine B's check
cannot see — so B could in principle reclaim A's live-but-unpushed build if its lease exceeded the
TTL. This is a **documented residual** for multi-machine `reclaim.auto: true`, *contained* (not
eliminated) by three things: the auto-off default, a generous TTL that a normal build pushes within,
and the CAS re-read. Setting `reclaim.auto` on the committed repo layer keeps the policy consistent
across clones. Single-clone operation has no such window (A's ref is local to the same clone running
reclaim). This residual is why `auto: true` stays an explicit, deliberate opt-in.

**I. The reclaim script authors its own mechanical commit.**
Chosen: like `archive-change.sh` / `board-refresh.sh`, the reclaim script commits + pushes its own
mechanical reclaim commit (ADR-0021). Rejected: a model-authored commit. Why: reclaim is purely
mechanical (no judgment), so ADR-0012's "model authors commit messages" (reserved for judgment
passes) does not apply; the deterministic-script constraint points the other way.

## 8. Why this is a spec, not an abstain

The abstain rule fires only when a decision **cannot be safely defaulted**. Every decision above
had a conservative default anchored in existing precedent (ADR-0012, ADR-0004, ADR-0019, ADR-0021,
the 3-day stale window, the layered-config system, the derived-view script family) and the stub's
own strong framing ("`proposed` with spec intact", "new manifest fields", "rides existing
status-transition commits", "reclaim touches metadata only"). The riskiest decision — whether to
mutate state autonomously — is defaulted to the strictly-safe side (OFF; flag + recommend only),
which needs no human blessing because it is additive to today's warn-only behavior. No decision's
conservative default could silently cause harm if built without a human, so the design is safe to
auto-commit as build-ready.

## 9. Open questions — resolved

The stub's three open questions are settled here: TTL default/unit → §7-D + §3; refresh at phase
boundaries → §3(1) + §7-D; reclaim-to-`proposed` vs marker → §7-B; runs unconditionally vs
gated → §7-E. None remain blocking.
