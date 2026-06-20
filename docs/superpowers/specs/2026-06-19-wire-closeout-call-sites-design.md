# Design: wire the close-out call sites to the extracted scripts (change 0028)

**Status:** design (brainstormed 2026-06-19)
**Change:** 0028
**Depends on:** — (the scripts already exist on `main`; 0025 is `done`)
**Builds on:** 0025 (`archive-change.sh` / `terminal-publish.sh` / `cleanup-feature-branch.sh`, the scripts this change finally wires in)
**Precedent:** 0011 (`github-mirror.sh`) · 0022 (`render-board.sh`) — both extractions *did* land their skill rewire (`docket-status` names them inline); this change brings the close-out scripts to parity.
**ADRs:** 0007 (script-owns-*how* / skill-owns-*when*) — the decision this change applies.

---

## 1. Context

Change **0025** built and tested the three deterministic close-out scripts and its
*What changes* explicitly scoped **"rewire the four call sites … to *invoke* the
scripts rather than restate the bash."** That rewire is **not present** in the shipped
skills: a grep of `skills/` for `archive-change.sh` / `terminal-publish.sh` /
`cleanup-feature-branch.sh` returns **nothing**, while the same grep for
`render-board.sh` / `github-mirror.sh` finds them named inline in
`docket-status`. So the close-out call sites still carry the full manual git dance,
and every close-out re-derives and re-executes it by hand.

That hand-execution is not just costly — it is **incorrect under partial failure**.
At the close-out of change 0026 (the run that triggered this change), the hand-rolled
archive staged the `git mv` rename but **dropped the follow-on `status: done`
frontmatter edit**: the `git add` listed the already-moved `active/` path beside the
`archive/` path, the non-matching pathspec aborted the whole `git add` (staging
nothing), and the rename-only commit — carrying `status: implemented` — then rode
terminal-publish onto `main`. It took a corrective commit on `docket` and a
re-publish to fix `main`. `archive-change.sh` performs the dated `git mv`, the
frontmatter set, and the change-file-only commit as one fail-closed primitive, so it
cannot exhibit this failure mode. The fix is to **use the script that already exists.**

This change carries no new behavior. It is the prose-rewire half of 0025 that never
landed, plus the learnings it distills: the close-out path is error-prone by hand
(LEARNINGS #22 trailing-newline, #25 unanchored-sed / mktemp-symlink / CAS-coverage,
#26 dropped-status-edit), the hardened scripts already neutralize every one of those
traps, and the only thing left is to route the live path through them.

## 2. Goal / non-goals

**Goal.** Every close-out call site invokes the extracted scripts instead of restating
the bash. After this change, no docket skill contains a hand-written `git mv … active
… archive`, terminal-publish `pub-<T>` worktree dance, or feature-branch/worktree
teardown — those mechanics live only in `scripts/` and `docket-convention` points at
them as the single source.

**Non-goals.**
- **No script behavior changes.** `archive-change.sh` / `terminal-publish.sh` /
  `cleanup-feature-branch.sh` and `test_closeout.sh` are consumed as-is.
- **No new regression guard / CI check** (e.g. a `--check` that the skills reference
  the scripts). Considered and explicitly declined for this scope; revisit only if the
  rewire regresses a second time.
- **No finalize post-archive verification gate** (asserting published status matches
  the merge outcome). A separate idea, not this change.
- **`migrate-to-docket.sh`'s duplicated helpers** — out of scope (LEARNINGS #26 tracks
  the twin).

## 3. What changes — the four call sites

Each call site shrinks from a bash block to the **0025 contract: the model authors the
commit message and passes it as `--message`; the script owns the deterministic
plumbing and the CAS-retry loops and is fail-closed; the skill trusts the exit code —
proceed on `0`, abort-and-report on non-zero.** The skill prose keeps owning *when*
(selection, the merge gate, sign-off, the bootstrap/mode guards) — only the mechanical
*how* moves to the call. This mirrors how `docket-status` already names
`render-board.sh` / `github-mirror.sh` inline; it is **not** N duplicated edits.

1. **`docket-finalize-change`** — the single source for three of the four:
   - *Per-change step 3 (Archive)* → `archive-change.sh` (done outcome).
   - *Terminal publish (docket-mode)* procedure → `terminal-publish.sh`. This section
     is the shared single source other skills reference, so rewiring it here rewires
     them by reference.
   - *Per-change step 4 (Clean up)* → `cleanup-feature-branch.sh`.
2. **`docket-status`** — its merge-sweep archive loop → `archive-change.sh` +
   the terminal-publish call, by reference to finalize's procedure (the sweep already
   invokes that procedure by reference; the rewire keeps that indirection).
3. **`docket-new-change`** — the *proposed-kill* sub-path → `archive-change.sh` /
   `terminal-publish.sh` with the **killed** outcome (`## Why killed`, no plan/results).
4. **`docket-implement-next`** — the *reconcile-kill* path → the same kill primitive.

The canonical description of the mechanics stays centralized: `docket-finalize-change`
owns the terminal-publish procedure and `docket-convention`'s *Branch model* /
terminal-publish references point at the scripts, so the per-skill edits are reference
updates, not re-descriptions.

## 4. Open question (recorded, not resolved here)

**Why did 0025's rewire never land?** Two hypotheses: (a) it was silently descoped
during 0025's build (scripts + tests shipped, prose rewire dropped), or (b) it landed
and was later clobbered — most plausibly by change 0026, which edited every skill's
Step 0 for config resolution and could have overwritten close-out prose on a bad
rebase. This change does **not** investigate that (the chosen scope is the rewire
itself), but the implementer's reconcile pass should `git log -p` the relevant skill
sections to decide **re-apply vs. write-fresh** — the end state is identical either
way. A short post-mortem of the root cause is being written separately from this spec.

## 5. Verification

- **Existing coverage suffices for the mechanics.** `tests/test_closeout.sh` already
  exercises `archive-change.sh` / `terminal-publish.sh` / `cleanup-feature-branch.sh`
  hermetically; this change adds no script code, so it adds no script tests.
- **The rewire is prose**, verified two ways: (1) a grep of the close-out skills now
  finds the script names where the manual bash used to be, and the manual `git mv …
  active … archive` / `pub-<T>` / worktree-teardown blocks are gone; (2) the **next
  live close-out dogfoods it** end-to-end (this repo is docket-mode, so the very next
  `docket-finalize-change` run exercises the rewired path).
- **Acceptance:** a finalize run archives, publishes, and cleans up entirely through
  the scripts, with the published `archive/<date>-<id>-<slug>.md` carrying the correct
  terminal `status:` — the exact property the 0026 hand-roll violated.

## 6. Risk

Low. No runtime/script behavior changes; the scripts are already merged and tested and
in daily use is only blocked by the skills not calling them. The main risk is an
**incomplete rewire** (a call site left half-manual), mitigated by the grep check in §5
and by the convention centralizing the mechanics so there are few edit sites.
