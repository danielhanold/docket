# Design — docket-status sweep: delegate archiving to archive-change.sh (remove the double-archive)

Change: #0036 · slug `status-sweep-double-archive` · spec drafted 2026-06-21 (autonomous groom)

## Problem

`docket-status`'s merge sweep archives a merged change **twice**. Its per-change loop runs a
hand-rolled archive in steps c–e (`git mv active/ → archive/<merge-date>`, set
`status: done` / `results:` / `updated:`, re-render the `## Artifacts` block, commit the change
file only, push `origin/docket`), and then step f **re-invokes** `scripts/archive-change.sh`,
which performs the same move + same field writes + same change-file-only commit + same push.
The second pass is idempotent — its reuse-existing-archive probe makes it a no-op once bytes
match — so the sweep is *correct* today. It is just convoluted: two code paths must stay in
lock-step to describe one operation, while `archive-change.sh` already exists to be the single
archive primitive (extracted in change 0026 precisely to remove hand-staging failure modes).
`docket-finalize-change` already delegates its entire archive to the script; the sweep should
too. Surfaced by the #0035 whole-branch review as a pre-existing tidy.

## Decision

Rewrite the sweep's per-change archive sub-procedure (today's steps c–f) so it **delegates the
archive entirely to `archive-change.sh`**, byte-aligned with `docket-finalize-change`'s step 3.
The convergence target already exists and is the single source — finalize step 3 is the model;
this change brings the sweep into line with it. No script changes; this is a skill-body edit to
`skills/docket-status/SKILL.md`.

### Target sweep per-change flow (`implemented` → `done`)

Steps **a** and **b** are unchanged (the sweep's own pre-gate); steps **g**, **h**, and the
once-per-run integration sync are unchanged. Only c–f collapse:

- **a.** `pull --rebase` on the metadata working tree; re-read `status`. Already `done` (or
  already under `archive/`) → no-op, continue. *(unchanged — the sweep's own short-circuit; it
  is cheaper than letting the script discover the no-op, and it gates the whole loop body.)*
- **b.** Compute the merge date in UTC (`gh mergedAt`, or `TZ=UTC git show -s
  --date=format-local:%Y-%m-%d <merge-sha>`). Never `now()`. *(unchanged)*
- **Archive (delegated).** Author the commit message, determine whether a `results:` file
  exists (it arrived via the PR merge), then invoke:
  ```
  scripts/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> \
      --outcome done --date <merge-date> [--results <path>] --message "<msg>"
  ```
  The script owns the dated `archive/<merge-date>-<id>-<slug>.md` move, the
  `status: done` / `updated: <merge-date>` / `results:` writes, the **change-file-only** commit,
  and the push-with-rebase-retry on `origin/docket`. **Trust the exit code:** `0` ⇒ archived
  (idempotent no-op if already archived, including across a day boundary); non-zero ⇒ apply the
  **Per-change failure posture** below (the sweep is a best-effort safety net — it does NOT
  inherit finalize's abort-and-report; see Assumptions A6/A9).
- **Re-render the block (follow-on commit).** After `archive-change.sh` returns 0, regenerate
  the `## Artifacts` block on the **archived** file:
  ```
  scripts/render-change-links.sh --change-file \
      .docket/<changes_dir>/archive/<merge-date>-<id>-<slug>.md --adrs-dir .docket/<adrs_dir>
  ```
  Commit this as a **follow-on metadata commit** on `docket` and **push `origin/docket`** —
  plan/results re-point to the integration branch at `done`; the renderer is the sole writer of
  the block (ADR-0012). This commit **must land on `origin/docket` before** terminal-publish.
- **Publish the terminal record.** `scripts/terminal-publish.sh … --outcome done …` copies the
  now-re-pointed terminal records from `origin/docket` onto the integration branch. *(unchanged
  invocation; only its inputs are now produced by the delegated steps above.)* Reached **only if
  the renderer follow-on commit landed on `origin/docket`** — a failed re-render must skip publish
  (per the Per-change failure posture below), or it would publish the stale block (#0035).
- **g / h / sync** — cleanup feature branch + worktree, harvest learnings, and the
  once-per-run integration-checkout sync are all **unchanged**.

**Per-change failure posture (all three delegated steps).** The sweep is a **bulk best-effort
safety net** run unattended — its existing later steps (harvest, integration-sync) are
already explicitly "log and continue … never abort the sweep" (cleanup step g says "Trust the
exit code"). The three now-separated archive
steps take the **same** posture, stated explicitly for each (the current sweep states none for
archive/publish — it only says "Trust the exit code" — so this fills a genuine gap, it does not
"preserve" a prior choice): on a non-zero exit from `archive-change.sh`, the renderer follow-on
commit/push, **or** `terminal-publish.sh`, **log it, abandon the remainder of THIS change's
close-out, and continue to the NEXT change.** Critically, "abandon the remainder" means a failed
archive skips the renderer and publish, and a **failed renderer commit skips publish** — never
barrel on to publish a stale block (#0035). The next sweep self-heals idempotently: each script
is a reuse-existing / byte-identical no-op on the already-done portion and re-attempts the rest.
This posture is **deliberately divergent from finalize step 3**, whose `non-zero ⇒
abort-and-report` fits a deliberate single-change close-out (and the convention's autonomous-
subagent stop-on-precondition rule), not a janitor draining N changes — so any single-source-by-
reference of finalize step 3 (Assumption A9) must reference the *sequence* and **override the
failure posture** with this paragraph, never re-import abort-and-report.

After this edit, the sweep's archive sub-procedure and finalize step 3 describe the identical
operation, keeping the two "must not diverge" notes — one each in `skills/docket-status/SKILL.md`
and `skills/docket-finalize-change/SKILL.md` (the convention carries no such note) — accurate.

## Open question 1 — does collapsing to the script silently drop any c–e behavior?

**No.** Field-by-field, every manual behavior has a script equivalent, and the script adds one:

| Manual step c–e behavior | `archive-change.sh` equivalent |
|---|---|
| `git mv active/ → archive/<date>` + reuse-existing idempotency | steps (1) reuse-existing probe + (2) dated move |
| `status: done` | `set_field … status done` |
| `updated: <merge-date>` | `set_field … updated <date>` |
| `results:` link (when a results file exists) | `set_field … results` when caller passes `--results` (caller still decides existence — unchanged from today's step f) |
| commit **change-file-only** + push `origin/docket` with rebase-retry | step (4) change-file-only commit + `cas_push` rebase-retry |
| `## Artifacts` re-render (#0035) | **not in the script** → preserved by the explicit follow-on renderer commit above |
| *(none)* | step (5) **fail-closed self-verification** — a strict gain the manual path lacked |

The only manual behavior the script does not carry is the #0035 renderer call, preserved by
sequencing. The `results:` existence determination was already the caller's job in today's step
f (`[--results <path>]`) and stays so. Nothing is dropped; the change is behavior-preserving
plus the script's added postcondition checks.

## Open question 2 — where does the renderer re-render sit in the delegated flow?

**As a follow-on metadata commit, after `archive-change.sh` returns 0 and before
`terminal-publish.sh`** — exactly how finalize step 3 sequences it, and exactly what the #0035
learning requires: *any close-out step that mutates a change file's derived content must
commit+push to `origin/docket` before terminal-publish copies from it.* terminal-publish copies
the archived change file *from `origin/docket`*; if the re-render landed after publish (or never
reached `origin/docket` first), the **stale** block would be published onto the integration
branch — defeating the re-point on the exact public surface it targets. This is why the renderer
cannot be bundled into the archive commit either: that commit is owned by `archive-change.sh`
and must stay change-file-only and byte-identical across concurrent archivers (the determinism
invariant). The two-commit shape (archive commit, then renderer follow-on commit) is mandatory,
not stylistic.

## What the implementer edits

- `skills/docket-status/SKILL.md` — the merge-sweep section: replace today's steps c–f with the
  delegated flow above. Recommended (a presentation call, open at build time): rather than
  restating finalize step 3's archive+render+publish sequence, have the sweep **reference**
  finalize's step 3 / *Terminal publish (docket-mode)* as the single source for that sequence —
  the sweep already references finalize for terminal-publish mechanics, so this extends an
  existing single-source pattern and removes the divergence surface the "must not diverge" note
  warns about. The floor (if not referencing) is byte-aligning the prose. **Either way, the
  Per-change failure posture stays in the sweep's own prose** — reference finalize step 3 for the
  *sequence*, but override its `abort-and-report` with the sweep's log-and-continue, since the two
  skills legitimately differ on failure handling (A6/A9).
- No change to `archive-change.sh`, `terminal-publish.sh`, `render-change-links.sh`, or the
  convention. The two "must not diverge" notes live one each in `skills/docket-status/SKILL.md`
  and `skills/docket-finalize-change/SKILL.md` (not the convention) and stay accurate once the
  sweep matches finalize.
- No ADR — this applies existing decisions (ADR-0012 script-vs-model boundary; the #0026
  archive-primitive extraction; the #0035 renderer-ordering learning), it does not make a new
  one. `adrs: []` stays.

## Testing approach

Skill-body procedure edit → skill-body sentinel tests (the repo's `grep -Eqi` pattern, e.g.
`tests/test_composition_wiring.sh`, `tests/test_convention_extraction.sh`), each mutation-tested
for non-vacuity, plus a whole-branch review for meaning. Floor of assertions:

1. The sweep invokes `archive-change.sh` for the `done` transition (delegation present).
2. The sweep's renderer call (`render-change-links.sh`) is ordered **after** the
   `archive-change.sh` invocation and **before** `terminal-publish.sh` (anchor to the unique
   "before … terminal-publish" phrasing per the #0021/#0015 ordering-sentinel lessons — assert
   the order, not mere presence).
3. The **old manual `git mv active/ → archive/` step is gone** from the sweep (mutation-test:
   re-introducing it must flip the assertion to NOT OK).
4. The two "must not diverge" notes (one each in `docket-status` and `docket-finalize-change`
   SKILL.md — not the convention) remain present and accurate.
5. The sweep's per-change failure posture is log-and-continue (anchor to its unique phrasing,
   e.g. "abandon the remainder of this change's close-out"), distinct from finalize step 3's
   abort-and-report — mutation-test that swapping in "abort-and-report" flips the assertion.

The script mechanics (`archive-change.sh`, `terminal-publish.sh`) already have full coverage in
`tests/test_closeout.sh` and need none added — this change adds no script behavior.

## Assumptions (autonomous-groom audit trail)

Every decision below was defaulted by the groomer without a human; each survived the adversarial
critic gate. Recorded so a human can audit the deferred design.

- **A1 — Spec, not trivial.** The change carries two real open questions (a field-by-field diff
  of two code paths; a correctness-critical renderer ordering with a demonstrated footgun in
  LEARNINGS #0035). That is design content worth recording for the implementer, above the
  "mechanical, no design questions" bar for `trivial: true`.
  *Rejected — trivial:* would force the implementer to re-derive the #0035 footgun from the
  ledger. *Rejected — abstain:* no decision needs human context; both open questions resolve
  deterministically from existing code (finalize step 3 + the diff).

- **A2 — Delegate to the script; do not keep the manual path.** Conservative and
  well-precedented: the convention names `archive-change.sh` THE archive primitive (#0026),
  finalize already delegates, and reviving hand-staging would revive the #0026 dropped-status
  footgun.
  *Rejected:* drop step f and keep the manual archive — wrong direction. *Rejected:* extract a
  new shared "archive+render+publish" wrapper both skills call — larger blast radius, out of
  scope for a doc-convergence tidy; noted as a possible future follow-up, not done here.

- **A3 — Renderer ordering (Open Q2): follow-on commit, after the script, before publish.**
  Pinned by finalize step 3 and the #0035 learning. The two-commit shape is mandatory because
  the archive commit is script-owned and must stay change-file-only/byte-identical.
  *Rejected:* bundle the renderer into the archive commit (breaks the determinism invariant) or
  run it after publish (reintroduces the exact #0035 stale-block bug).

- **A4 — Field-by-field diff (Open Q1): nothing dropped.** Verified against the live
  `archive-change.sh` source (see the table above). The script is a strict superset (adds
  fail-closed self-verification). The load-bearing safety check; re-verified by the critic.

- **A5 — `results:` determination stays in the caller.** Unchanged from today's step f, which
  already passes `[--results <path>]`; `archive-change.sh` writes the field only when `--results`
  is non-empty. No new behavior.

- **A6 — On any delegated-step failure the sweep logs-and-continues to the next change (does NOT
  abort-and-report).** *(Revised after critic.)* This is a genuine gap-fill, not a preservation:
  the current sweep states **no** failure posture for archive/publish (it only says "Trust the
  exit code"; only the later cleanup/harvest/sync steps are explicitly best-effort). The default
  is grounded in the convention, not invented: the sweep is the **bulk best-effort safety net**
  run unattended (`docket-implement-next` step 0; the explicit `docket-status` run), whereas
  finalize's `abort-and-report` is the posture of a **deliberate single-change close-out** and of
  the convention's autonomous-subagent stop-on-precondition rule — not of a janitor draining N
  changes. The posture is stated explicitly for **all three** now-separated steps (archive /
  renderer-commit / publish), and "abandon the remainder of this change's close-out" carries the
  #0035 guard: a failed renderer commit skips publish, so a stale block is never published. The
  next sweep self-heals idempotently.
  *Rejected — abort-and-report:* would halt the whole safety-net sweep (or strand later changes)
  on one transient failure, contrary to the sweep's existing best-effort ethos. *Why not abstain:*
  the choice is deterministically derivable from the convention's sweep-vs-finalize roles; no human
  context is missing.

- **A7 — Dependency #0035 is satisfied.** #0035 is `done` (PR #44 merged, archived
  2026-06-21), so the renderer call already exists in both the sweep and finalize. No
  design-ahead gap — the prerequisite is in place and the change is build-ready w.r.t. its
  `depends_on`.

- **A8 — No ADR.** Applies ADR-0012 and the #0026 / #0035 precedents; introduces no new
  architectural decision. `adrs: []` unchanged.

- **A9 — Single-source vs byte-align is a presentation call left to build time.** The
  recommended single-source-by-reference approach reduces future drift, but byte-alignment also
  satisfies convergence; the implementer may choose either. Not a blocker for build-readiness.
  *(Revised after critic — two corrections folded in:)* (1) the option is **not independent of
  A6**: referencing finalize step 3 must reference the *sequence only* and **override** its
  `abort-and-report` with the sweep's log-and-continue (A6), or the "recommended" option would
  silently reverse the failure posture — the spec's flow and *What the implementer edits* now make
  this carve-out explicit. (2) The two "must not diverge" notes live one each in
  `skills/docket-status/SKILL.md` and `skills/docket-finalize-change/SKILL.md`, **not** the
  convention (originally misattributed); all references corrected.
