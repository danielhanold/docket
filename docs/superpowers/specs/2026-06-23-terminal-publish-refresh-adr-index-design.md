# Design — terminal-publish: refresh the integration-branch ADR index when it publishes an ADR

Change: #0040 · slug `terminal-publish-refresh-adr-index` · spec drafted 2026-06-23 (autonomous groom)

## Problem

`terminal-publish.sh` is the single sanctioned flow of docket metadata onto the integration
branch. When it publishes one or more `Accepted` ADR *files* (change-publish `--id` with ADRs in
the change's `adrs:`, or ADR-only `--adr`), it copies those files but **never** regenerates the
ADR index `<adrs_dir>/README.md` on the integration branch. The index is a derived view that
`render-adr-index.sh` regenerates and commits on `metadata_branch` only; on the integration
branch it is whatever was last hand-published. So as ADRs publish to the integration branch the
index there **silently drifts**: ADR files accumulate while the index keeps pointing at an old
high-water mark, leaving newer ADR files present-but-unlisted.

This is dogfooded in docket's own repo today: `origin/main` carries **14** ADR files (through
ADR-0014), but `origin/main:docs/adrs/README.md` still lists only through **ADR-0002**. The same
drift was observed in `markhaus` (files through 0051, index stalled at 0042) and has twice been
patched by a manual regenerate-and-push to `main`. A reader browsing the integration branch sees
an index that disagrees with the ADR files beside it, and every absent row is a decision the code
line silently fails to surface.

## Decision

Make `terminal-publish.sh` regenerate the integration-branch ADR index **from the integration
branch's own ADR files** and include it in the **same publish commit**, whenever (and only when)
the publish actually copies one or more ADR files onto that branch. This keeps the integration
index consistent with the ADR files published beside it, automatically, for every caller —
`docket-finalize-change` (`done`), the kill paths (`killed`), and `docket-adr`'s standalone /
status-change `--adr` publishes — with no per-caller wiring. A no-op in `main`-mode.

> **This spec presupposes model (b) of change #0033, decided by the owner.** #0033
> ("adr-index-main-maintenance") posed the open question *should a generated ADR index live on the
> integration branch at all* — model **(a)** delete it (treat it like `BOARD.md`, docket-only) vs
> model **(b)** keep it on the integration branch, re-rendered from that branch's own ADR set.
> docket-auto-groom abstained on #0033 on 2026-06-20 because (a)-vs-(b) turns on owner intent. The
> owner then authored #0040 on 2026-06-23 (three days later), framed entirely as (b) — "make the
> integration-branch ADR index stay consistent" — and even encoding the correct-(b) implementation
> (#0033's abstain spelled out: "a correct (b) would re-render from main's own ADR set in a
> dedicated main-side pass"; #0040's body says "regenerate from the integration branch's ADR files
> … must reflect only the ADR files actually present there"). #0040 is thus the owner supplying the
> intent #0033 was missing, scoped to the additive tooling only (no convention ADR). **Audit note
> for the human:** emitting this spec effectively resolves #0033's (a)-vs-(b) in favor of (b);
> #0033 may now warrant a kill or a narrowing to its remaining sub-question ("should the resulting
> convention be recorded in a new ADR?"). This groom does not touch #0033 (kill/defer/edit of
> another stub is never autonomous). See Assumption **A2**. `related: [33]` is recorded on #0040.

### Target flow inside `terminal-publish.sh`

The script already, after assembling the copy-set: provisions a transient `pub` worktree on
`origin/<int_branch>`, `checkout`s the copy-set from `origin/<metadata_branch>` into `pub`,
guard-commits, and CAS-pushes `HEAD:<int_branch>` with a fetch-and-assert postcondition. Three
insertions, all confined to the copy-and-push region (no change to copy-set assembly, the mode
guard, or argument handling):

1. **Track whether an ADR was published.** During copy-set assembly set a boolean
   `adr_published` — **true** in `--adr` mode (the lone copy-set entry is an ADR), and in `--id`
   mode true iff ≥1 ADR passed the `Accepted` gate (i.e. ≥1 entry under `<adrs_dir>` was appended
   to the copy-set). This is bookkeeping over logic that already runs.

2. **Render the index after the copy-set is checked out, into the same staged commit.** After
   `git -C "$pub" checkout "$metaref" -- "${copyset[@]}"` (so the newly-published ADR file(s) are
   now present in `pub` *alongside* the integration branch's pre-existing ADR files), and only
   when `adr_published` is true:

   ```bash
   "$(dirname "$0")/render-adr-index.sh" --adrs-dir "$pub/$ADRS_DIR" > "$pub/$ADRS_DIR/README.md"
   git -C "$pub" add "$ADRS_DIR/README.md"
   ```

   The render reads `$pub/<adrs_dir>` — the **integration branch's** ADR files with this publish's
   ADR(s) overlaid — so the emitted index reflects exactly the ADRs present on the branch after
   this publish; every link resolves, and it incidentally re-lists any previously-published-but-
   unindexed ADRs (incremental self-heal). The existing guarded `diff --cached --quiet || commit`
   then captures the copy-set **and** the index in **one** commit; a no-op publish where both the
   copy-set bytes and the index already match still creates no commit.

3. **Re-render in the CAS retry path.** The push-reject branch already re-checkouts the copy-set
   before `rebase --continue`; mirror the render+add there (guarded by the same `adr_published`)
   so a concurrent push that moved the integration tip is resolved by deterministic regeneration,
   never a hand-merge — the same regenerate-don't-3-way-merge rule `render-adr-index.md` states.

Optionally (recommended hardening, consistent with the script's fail-closed ethos): when
`adr_published` is true, add `<adrs_dir>/README.md` to the post-push self-verify so a publish that
silently failed to land the index also exits non-zero. The index is rendered locally (not part of
`copyset`), so this is a separate one-line assertion, not a `copyset` membership change — keeping
`copyset` semantics (checkout-from-metaref + existing postcondition) untouched.

## Open question 1 — in `terminal-publish.sh`, or a separate step each caller runs after publish?

**In-script.** `terminal-publish.sh` is already "the single executor of both publish shapes";
every terminal transition delegates to it and none restate its mechanics. Putting the refresh
in-script means all callers — finalize, the kill paths, the `docket-status` sweep, and
`docket-adr`'s `--adr` publishes — get a consistent, **atomic** (same-commit) index refresh for
free, with zero per-caller wiring and no window where ADR files are on the branch but the index
is not. A separate per-caller step would duplicate the logic across ≥4 call sites and reopen the
exact drift this change closes the first time one caller forgot it. This matches ADR-0012's
script-vs-model boundary: deterministic publish mechanics belong in the script, not in
agent-constructed `gh`/`git` sequences.

## Open question 2 — what does the integration index reflect, and is trailing the metadata index correct?

**It reflects exactly the ADR files on the integration branch — by rendering from `pub`'s own
`<adrs_dir>`, never from the metadata branch's superset.** This is load-bearing for correctness:
docket regenerates its index at ADR *accept* time, but ADR files reach the integration branch only
at their change's *terminal* publish, so copying the metadata index verbatim would emit rows
linking to ADR files **not yet published** → dangling links. Rendering from the integration
branch's set guarantees every row links to a file that is actually there.

A consequence — intended and correct — is that the integration index **trails** the metadata index
whenever an ADR is `Accepted` but its change has not yet reached a terminal state: that ADR file
is not on the branch, so it is not in the branch's index. "The index reflects what is on the
branch" is the intended contract precisely because it keeps every link resolvable. Each subsequent
ADR-bearing publish full-re-renders and converges the branch index toward the metadata index as
the underlying ADR files land.

## Scope guards

- **Back-fill of already-drifted branches stays out of scope** (per the stub). The refresh fires
  only when this publish copies an ADR; it does not run on no-ADR change-publishes. Because each
  fire is a *full* re-render from the branch's ADR set, prior drift (e.g. docket's own
  ADR-0003..0014 currently unlisted on `main`) self-heals on the **next** ADR publish — but if a
  human wants it healed *before* the next natural publish, that remains a manual
  `render-adr-index.sh` + push, exactly as today. This change *prevents future* drift; it does not
  promise to retroactively repair the present gap on the spot.
- **No other derived view is published.** `BOARD.md` stays `metadata_branch`-only (unchanged).
- **The copy-set is otherwise unchanged** — change file, spec, Accepted ADRs; this adds only the
  rendered index.
- **`main`-mode is a no-op** — the script's existing mode guard (`META_BRANCH == INT_BRANCH`)
  early-exits before any of this; in `main`-mode the metadata tree *is* the integration branch and
  `docket-adr` already writes the index there directly.

## What the implementer edits

- `scripts/terminal-publish.sh` — the three insertions above (the `adr_published` flag in copy-set
  assembly; render+add after the `pub` checkout, guarded by the flag; the mirrored render+add in
  the CAS retry path; optional index entry in the self-verify). No change to copy-set assembly,
  the mode guard, argument parsing, or the `--adr`/`--id` split.
- `scripts/terminal-publish.md` — document the index refresh: when it fires (`adr_published`), that
  it renders from the integration branch's own ADR set (the dangling-link rationale), that it rides
  the same publish commit, and the `main`-mode no-op. Add an invariant mirroring the existing
  `BOARD.md is never published` line, e.g. *the ADR index is refreshed only from the integration
  branch's published ADR files, and only when an ADR is published.*
- Reuse `render-adr-index.sh` **as-is** — it already takes `--adrs-dir DIR`, reads a local dir, and
  emits to stdout; pointing it at `$pub/<adrs_dir>` is its intended use. No renderer change.
- One-line touch to `docket-convention`'s terminal-publish copy-set description is optional and a
  build-time call — the convention says terminal-publish copies "the change file, spec, and
  Accepted ADRs"; noting it also refreshes the integration ADR index keeps it accurate. Not
  required for correctness (the behavior is fully specified in `terminal-publish.md`).
- **No ADR.** This is additive, reversible tooling that applies existing decisions (ADR-0012
  script-vs-model boundary; render-adr-index's sole-writer/regenerate rules); it makes no new
  architectural decision. `adrs: []` stays. (This is also what keeps #0040 clear of #0033's second
  abstain ground — "an autonomous pipeline should not cement a meta-convention ADR.")

## Testing approach (implementer's TDD call)

`terminal-publish.sh`'s functional coverage lives in `tests/test_closeout.sh` (real git fixtures);
`tests/test_terminal_publish.sh` holds only arg-validation guards. Extend the functional suite —
floor of assertions:

1. **Change-publish (`--id`) with an Accepted ADR** → after publish, `origin/<int>:README.md`
   lists that ADR, and **every** linked file in the index exists on `origin/<int>` (no dangling
   rows). The index and the ADR file land in the **same** commit.
2. **ADR-only publish (`--adr`)** → same: the index on `origin/<int>` now includes the published
   ADR and only branch-present ADRs.
3. **Renders from the branch set, not the metadata superset** — with an `Accepted` ADR on
   `metadata_branch` whose change is *not* terminal (so its file is not on `<int>`), a publish of a
   *different* ADR must produce an index that does **not** list the not-yet-published one (mutation
   test the dangling-link guard).
4. **No-ADR change-publish** → the index on `<int>` is **unchanged** (no spurious back-fill commit).
5. **`main`-mode** (`--metadata-branch == --integration-branch`) → unchanged early no-op; no index
   write.
6. **Idempotent re-run** → re-publishing the same records creates no new commit (byte-stable index).

`render-adr-index.sh` already has full unit coverage in `tests/test_render_adr_index.sh` and needs
none added — this change adds no renderer behavior.

## Assumptions (autonomous-groom audit trail)

Every decision below was defaulted by the groomer without a human; each is gated by the adversarial
critic. Recorded so a human can audit the deferred design.

- **A1 — Spec, not trivial, not abstain.** Real implementation design content: sequencing the
  render into the existing copy/CAS-push region (incl. the retry path), the fire-condition, the
  render-from-branch-set correctness constraint, and the same-commit-vs-separate-commit
  reconciliation (A6). Above the "mechanical, no design questions" bar for `trivial`.
  *Rejected — trivial:* would make the implementer re-derive the dangling-link footgun and the
  retry-path render. *Rejected — abstain:* see A2 — the one owner-intent decision (a-vs-b) is
  already supplied by the owner; nothing else needs human context.

- **A2 — (a-vs-b) is the owner's decision, already made (load-bearing).** #0040 presupposes model
  (b) of #0033. I do **not** default that decision — the owner did, by authoring #0040: same owner
  (danny@danielhanold.com), three days *after* #0033's 2026-06-20 abstain that named (a-vs-b) as
  needs-human-context; #0040's entire framing is (b) ("make the integration-branch ADR index stay
  consistent"), and it independently reproduces #0033's spelled-out correct-(b) implementation
  (re-render from the branch's own ADR set, not the superset). That precise overlap is the human
  supplying the missing intent. #0040 is *narrower* than #0033 — it is the additive (b) tooling
  with **no** convention ADR — so it also clears #0033's second abstain ground (don't let an
  autonomous pipeline cement a meta-convention ADR). #0033's own critic noted "(b) is additive
  tooling," i.e. the conservative, reversible direction.
  *Rejected — abstain (re-defer to #0033):* would refuse work the owner explicitly scoped and
  marked `auto_groomable: true`, on a decision the owner has visibly resolved; the abstain rule
  guards undecidable-by-the-agent intent, not intent the owner has written into the stub. *Audit
  flag (not a blocker):* emitting this resolves #0033's (a-vs-b) toward (b); the human may want to
  kill or narrow #0033. This groom records `related: [33]` on #0040 and touches #0033 in no other
  way (kill/defer/edit of another stub is never autonomous).

- **A3 — In-script, not per-caller (Open Q1).** Conservative and well-precedented: the convention
  names `terminal-publish.sh` THE single publish executor; in-script gives every caller an atomic,
  consistent refresh with no duplicated logic and no forgot-a-caller drift. Matches ADR-0012.
  *Rejected:* a separate post-publish step in each of ≥4 callers — duplicated, drift-prone, the
  exact failure mode this change closes.

- **A4 — Render from the integration branch's ADR files, never the metadata superset (Open Q2).**
  Forced by correctness, not a free choice: the metadata index lists `Accepted` ADRs whose files
  have not yet reached the branch, so copying it verbatim yields dangling links. Rendering from
  `$pub/<adrs_dir>` after the copy-set checkout is the only link-resolvable source.
  *Rejected:* copy `metadata:README.md` into the copy-set (dangling links — the documented #0033
  footgun).

- **A5 — Fire only when an ADR is published (`adr_published`); full re-render; back-fill stays out
  of scope.** Matches the stub ("whenever … publishes one or more Accepted ADRs … also
  regenerate") and avoids spurious index commits on no-ADR change-publishes. Because each fire is a
  full re-render, prior drift self-heals on the next ADR publish — without this change claiming the
  on-the-spot back-fill the stub explicitly scoped out.
  *Rejected — render on every publish:* would emit back-fill commits on no-ADR publishes (out of
  scope) and add noise. *Rejected — append only this ADR's row:* non-deterministic vs the
  sole-writer renderer; loses the self-heal.

- **A6 — Same publish commit, not a separate index commit.** `render-adr-index`'s/`docket-adr`'s
  "index in a **separate** commit" rule exists to stop **concurrent ADR creates on
  `metadata_branch`** racing on the shared index. The integration-branch publish is a different
  context: a single serialized CAS push, where atomicity (ADR file + its index row landing
  together) is the goal and the CAS retry loop already deterministically re-derives on conflict
  (A-retry). So bundling the index into the publish commit is correct here and does not violate the
  metadata-branch rule.
  *Rejected:* a second publish commit just for the index — no concurrency benefit on the
  integration branch and breaks same-commit atomicity.

- **A7 — Re-render in the CAS retry path.** The retry already re-checkouts the copy-set before
  `rebase --continue`; the index must be regenerated there too (regenerate-don't-3-way-merge), or a
  concurrent push could strand a stale/conflicted index. Low-cost mirror of an existing line.

- **A8 — `main`-mode no-op.** Forced by the script's existing mode guard, which early-exits before
  the copy/push region; in `main`-mode `docket-adr` already maintains the index in place. No new
  branch.

- **A9 — Optional self-verify of the index is hardening, not required.** Recommended for fail-closed
  symmetry but presentation-level; the core copy-set postcondition is unchanged either way. Left to
  build time.

- **A10 — `depends_on` stays `[]`; `related: [33]` added.** #0040 is independently implementable —
  (b)'s tooling needs nothing from #0033 reaching `done`; if anything, this spec moots part of
  #0033. The cross-link is recorded for the human audit (A2), not as a build gate.
