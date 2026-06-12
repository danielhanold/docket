# Design: `docket-groom-next` — pick the next needs-brainstorm stub and groom it to build-ready

**Status:** design (brainstormed 2026-06-12)
**Change:** 0012
**Related:** `docket-new-change` (whose scan mode produces the stubs this skill consumes, and whose brainstorm/spec mechanics it reuses), `docket-implement-next` (whose selection order it copies and whose queue it feeds), `docket-convention` (gains a sixth operating skill)

## 1. Context / problem

Ideas are captured on the go as scan-mode stubs — `proposed`, no `spec:`, not `trivial: true` — the state the board renders as **needs-brainstorm**. The convention promises these are turned build-ready by "a later brainstorm pass", but no skill owns that pass: `docket-new-change` only mints *new* changes; `docket-implement-next` only selects *build-ready* ones. The backlog currently holds six needs-brainstorm stubs with no skill whose job is to drain them.

`docket-groom-next` is that skill: a "next" skill over the needs-brainstorm queue, mirroring `docket-implement-next`'s shape. The differences: the queue (needs-brainstorm stubs, not build-ready changes), the work (an **interactive** brainstorm with the human, not an autonomous build), and the exit (build-ready `proposed`, not `implemented` with a PR). Selection is autonomous; the design conversation is not — the human is the point.

"Groom" is the Jira-lineage term for exactly this transformation (stubbed-out item → ready). `docket-brainstorm-next` was rejected as ambiguous — "brainstorm" already means designing a *new* change in `docket-new-change`. (`docket-refine`, `docket-spec-next`, `docket-ready-next` were considered and dropped as less evocative; full naming record in change 0012's body.)

## 2. Selection

The queue: every change in `active/` with `status: proposed`, empty `spec:`, and not `trivial: true` — exactly the convention's **needs-brainstorm** definition. Ranking is `docket-implement-next`'s deterministic order, unchanged: `priority` (`critical` > `high` > `medium` > `low`) → age (`created`) → **lowest `id`**. The caller may pass an explicit id to override selection ("groom 0011"); an explicit id that is not needs-brainstorm is an error, not a silent re-pick.

**`depends_on` does not gate selection.** Designing ahead of dependencies is normal — that is what specs are for — and the implementer's reconcile pass re-validates every spec against current reality at build time anyway. Instead of gating, the session **opens by stating each dependency and its status** (`done` / `implemented — needs your merge` / not yet built), so the human designs with eyes open. Decision record: "deps must be done first" was rejected because it serializes design behind builds (often leaving nothing groomable while one change sits at a merge gate); "eligible but deprioritized" was rejected as extra selection machinery for marginal benefit.

Empty queue → report "nothing needs grooming" and stop. (Changes that are `proposed` with a spec are build-ready — `docket-implement-next`'s queue, not this one.)

## 3. Session flow

1. **Step 0 — convention load (blocking).** Invoke `docket-convention` first, same as every operating skill; the body uses convention vocabulary without redefinition.
2. **Sync.** Ensure the metadata working tree (in `docket`-mode the persistent `.docket/` worktree; idempotent create per the convention's Branch model) and sync it to its remote before any read. Run the bootstrap guard as the convention requires.
3. **Select** per §2. No claim is taken (§5).
4. **Scan related context** — neighbouring `active/` changes, recently archived changes, and the ADR index — *before* the brainstorm, so the conversation is informed by adjacent work (same ordering rationale as `docket-new-change`'s step 3). Update `related:`/`adrs:` after the design settles.
5. **Brainstorm with the human** — invoke `superpowers:brainstorming`, seeded with the stub's body and its `## Open questions`. The stub's open questions are the session's starting agenda. **Stop at the spec**: never continue to `superpowers:writing-plans` — planning is build-time (`docket-implement-next` step 5).
6. **Exit** per §4, then bookkeeping per §5.

One stub per invocation, like `docket-implement-next`; loop by re-invoking.

## 4. Exits — four outcomes, no new lifecycle status

| outcome | when | writes |
|---|---|---|
| **Spec** (normal) | real design settled | spec to `docs/superpowers/specs/<UTC-date>-<slug>-design.md` on `metadata_branch`; set `spec:`; refresh body (Why/What/Out-of-scope updated to the settled design; resolved `## Open questions` entries removed); set `updated:` |
| **Trivial verdict** | brainstorm concludes there is no real design question | set `trivial: true`; tighten body; no spec; set `updated:` |
| **Kill** | stub is obsolete, a duplicate, or a bad idea | the existing **proposed-kill sub-path** in `docket-new-change` — referenced, never restated (it already covers `## Why killed`, archive move via terminal-publish, board refresh) |
| **Defer** | right idea, wrong time | `status: deferred` + `## Why deferred`; set `updated:` |

Spec and trivial both land the change **build-ready** — the implementer's queue picks it up from there. The groom session proposes the exit; the human confirms it (the kill and defer exits especially are the human's call, surfaced as a recommendation during the brainstorm).

## 5. Bookkeeping & concurrency

All writes happen in the **metadata working tree** on `metadata_branch` and are pushed to its remote immediately, like every docket skill. Commit shape mirrors `docket-new-change` step 5: the change-file edit + spec in one commit, then the **Board pass as a separate, must-land commit** (the readiness cell flips from `needs-brainstorm` to build-ready, or the row leaves the Proposed section on kill/defer).

**No claim.** Decision record: grooming is human-attended and minutes-long, so concurrent-groomer collisions are improbable, and the existing push discipline already protects the write — the final push's `pull --rebase`-and-retry loop is the CAS. One addition to that loop: if the rebase brought in changes touching the groomed change's file, **re-read it before re-pushing** — if it is no longer needs-brainstorm (someone else groomed or killed it), stop and report rather than overwrite. Rejected alternatives: a `grooming:` frontmatter marker (new field + stale-marker health checks for a race that barely exists) and a full status-based CAS claim (an eighth lifecycle status for a human-attended activity — heavy).

## 6. Touch-ups to existing skills and docs

- **`docket-new-change`** — scan mode's "a later brainstorm pass turns build-ready" now names `docket-groom-next` as that pass. No mechanics change.
- **`docket-convention`** — the operating-skills enumeration (five skills) grows to six; the lifecycle/build-readiness text is already correct and needs no change (groom-next moves a change *within* `proposed`, except the defer/kill exits which reuse existing transitions).
- **`docket-status`** — no change. Board rendering already distinguishes needs-brainstorm from build-ready; no new states exist.
- **Repo plumbing** — `link-skills.sh` needs **no change** (it globs `skills/*/`; verified at reconcile 2026-06-12). The hardcoded skill enumerations in tests do: `tests/test_convention_extraction.sh` (`OPERATING=` array — Step-0/sentinel assertions) and `tests/test_docket_metadata_branch.sh` (`SKILLS=` array) gain `docket-groom-next`.

## 7. New skill file

`skills/docket-groom-next/SKILL.md`, following the post-0005 reference-loading pattern: frontmatter `description` worded for both triggers (a human saying "groom the backlog" / "groom change N", and other skills referring to grooming); blocking Step-0 `docket-convention` load; body uses convention vocabulary without restating it; explicit **never-writes-code, never-branches** framing like `docket-new-change` (it writes markdown only).

## 8. Out of scope

- Building anything — the skill stops at build-ready, exactly where `docket-implement-next` picks up.
- Autonomous (no-human) spec writing — the brainstorm stays interactive.
- Batch grooming — one stub per invocation; loop by re-invoking.
- Re-grooming changes that already have a spec — drift against reality is the reconcile pass's job; a human who wants to redo a design can clear `spec:` by hand (not part of this skill).
- Any new `.docket.yml` knob, frontmatter field, or lifecycle status — the design deliberately adds none.

## 9. Testing

Follow the existing suite's pattern (`tests/`): a structural test asserting the new SKILL.md exists, carries the blocking Step-0 convention load, and never restates convention definitions (the anti-restatement sentinels used since 0005); plus updates to any inventory-style assertions (skill count, link-skills coverage). Behavioral verification of a groom session is a build-time concern for the implementer's plan, not pinned here.
