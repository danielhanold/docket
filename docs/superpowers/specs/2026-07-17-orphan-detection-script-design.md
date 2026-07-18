# Orphan detection script — design

- **Change:** 0092 — orphan detection script (cross-reference change ids in merged commits against archive state)
- **Date:** 2026-07-17 (UTC)
- **Author:** docket-auto-groom (autonomous self-brainstorm; every decision audited below)
- **Status:** build-ready design
- **Related:** #0023 (`board-checks.sh` + ADR-0012 boundary — done; the health-check home this extends), #0083 (terminal-publish gap — auto-groom blocked / needs human; **owns the class-2 detector**, see Scope boundary), beads (`gastownhall/beads`) `bd orphans`/`bd doctor` (the competitive prior art)
- **ADRs cited:** ADR-0012 (script-vs-model boundary — the binding constraint), ADR-0001 (metadata branch model — the branch topology the check reads)

## Problem

docket has no mechanical cross-reference between what the git history says happened to a change
and what docket's own state says. Beads embeds the issue id in commit subjects and ships
`bd orphans` — "issues referenced in commits but still open" — a pure cross-reference that catches
book-keeping drift in one sweep. docket already carries the raw material: commit subjects on the
integration branch reference change ids by convention (`docket(0085): …`, `… (change 0085)`), and
the archive/active split on the metadata branch records each change's terminal state. Nothing
cross-references them, so an orphan — a change whose work merged but whose docket record was never
closed out — is invisible until found by hand.

## Goal

A **deterministic, git-only, offline** detector (constraint from capture, ADR-0012: a script, not
model prose — pure git reads, no network) that cross-references change ids referenced in
integration-branch commits against docket's active/archive state, and reports each divergence as a
`docket-status` health-check finding with the evidence commit. Detection only — it reports, it never
heals.

## Scope boundary — two detection classes; the terminal-publish gap is #0083's, not this change's

The capture enumerated three candidate classes. This design ships **two** and **defers the third to
#0083**:

- **Ship — class 1, `merged-orphan`:** a change id appears in a merged commit subject on the
  integration branch, but the change is still **non-terminal** (in `active/`, not archived) on the
  metadata branch. This is *the* classic orphan (`bd orphans`) and the change's titular purpose.
- **Ship — class 3, `unknown-commit-ref`:** a change id is referenced by an integration-branch
  commit subject but **no change file** with that id exists in `active/` or `archive/` (a typo'd or
  deleted id).
- **Defer to #0083 — class 2, "archived-but-unpublished terminal record":** a change terminal on
  the metadata branch whose record is missing from the integration branch under
  `terminal_publish: true` — the #0043 case.

**Why class 2 is out.** #0083 (auto-groom blocked, needs human) owns the terminal-publish gap. Its
groom abstained with decisive evidence that the one realized instance (#0043) was a **consciously
deferred, human-recommended** choice to leave the integration branch clean for a never-shipped
proposal — not a bug, not a classifier denial. #0083's still-open, human-only question is precisely
*"do you want that class of deferral **detected**, marked, or simply accepted?"* Building a class-2
detector here would answer that question ("detected") — it would **decide #0083's abstained
detector-vs-marker-vs-neither call for it**, which this groom must not do. #0083 also established
(gap census, 2026-07-17) that #0043 was the *only* realized gap and that a naive set-difference
fires on legitimately-pending records forever unless it honors `terminal_publish` and scopes to
terminal records — reinforcing that class 2 is a judgment-laden design a human must shape, not a
default an agent may pick. Classes 1 and 3 have no such entanglement: neither touches the
terminal-publish gap, and both are unambiguous book-keeping-drift findings.

The two changes compose cleanly and additively (see *Design*): if #0083 later decides a class-2
detector is wanted, it lands as another `board-checks.sh` check-id beside these two, no rework.
This resolves the capture's open question "does #0083's detection half collapse into this script?"
as **no — 0092 ships classes 1 & 3; the class-2 detector remains #0083's to decide.**

## Design

### Home — extend `board-checks.sh` (no new script, no `docket-status` edit)

The check(s) live as new check-ids **inside the existing `scripts/board-checks.sh`**, alongside
`broken-spec` / `broken-plan-results` / `stale-in-progress` / `merge-gate-stall` / `dep-cycle`.
This is the conservative, lowest-surface home, and it fits every existing seam:

- **The constraint matches the script's own invariant.** `board-checks.sh` is already declared
  *"Git-only, offline. No network calls, no `gh`."* — byte-identical to 0092's "pure git reads, no
  network." It already runs `git log` / `git cat-file -e` / `git rev-parse`.
- **It already has both branches.** `board-checks.sh --changes-dir DIR --metadata-branch BR
  --integration-branch BR` — the exact inputs the cross-reference needs. `docket-status.sh` already
  invokes it with `--integration-branch origin/<integration_branch>`.
- **Auto-discovery — no `docket-status` wiring change.** `docket-status.sh`'s `health_checks()`
  pipes *whatever* TSV `board-checks.sh` emits, prefixing each line `check <check-id> <change-id>
  <message>`. New check-ids surface through that pipe with **zero** `docket-status.sh` edits (the
  `check-plumbing-auto-discovery` learning: verify the plumbing auto-discovers before planning an
  edit to it — it does here). "Wired in as a `docket-status` health check via the `docket.sh`
  facade" (the capture) is satisfied entirely by adding to `board-checks.sh`.
- **Output/exit contract ride the existing rails unchanged.** Findings accumulate into the same
  list and sort by `(check-id asc, change-id numeric asc)`; `--strict` already exits 1 on any
  finding; a clean tree still prints nothing. No new flags, no exit-code changes.

### Extraction — which ids a commit references (resolves Open Q1)

Parse **commit *subject* lines only** (never bodies) on the integration branch, and count an id
only from the two unambiguous docket-convention forms:

1. a **numeric conventional-commit scope** — `<type>(<id>):` where `<id>` is a 1–4 digit number,
   e.g. `docket(0085): …`, `results(0085): …`; and
2. the **trailing `(change <id>)`** form, e.g. `feat: … (change 0085)`.

Ids are normalized to their integer value (zero-padding tolerated: `0085` → `85`). **Bare `#NNNN`
is deliberately excluded** — it collides with PR numbers (`#95` is a PR, not change 95) and is the
capture's named false-positive hazard. Body-text mentions ("related to change 50") are excluded by
parsing subjects only. This is the conservative floor; widening the grammar later is additive.
(Implementer: escape/anchor the ERE as usual — `escape-ere-metacharacters-in-key`, `pipefail`,
`shell-portability`, `guards-are-code`.)

### History window — full history, stateless (resolves Open Q2)

Scan the **full** commit history of `origin/<integration_branch>` on every run (`git log`
subjects). No high-water mark, no persisted cursor: `board-checks.sh` holds no state, the "pure git
reads" constraint discourages a side file, runs are deterministic, and the repo is small (≈100s of
commits). If integration-branch history ever grows enough to matter, a `--since <ref>` window is a
clean additive follow-up — noted, not built.

### The two checks

- **`merged-orphan`** (class 1): for each id extracted from `origin/<integration_branch>` subjects,
  if a change file for that id exists under `active/` (i.e. it is non-terminal — no `archive/`
  counterpart), emit a finding. Change-id column = the orphan's id; message names the **evidence
  commit** (short sha + subject) and states the change is still active. This is a *git-history*
  signal that **complements** the sweep's *PR-status* detection: it catches orphans the PR-based
  sweep structurally cannot (squash-merge under a differently-named branch, a change whose `pr:` was
  never recorded, or a sweep that never ran). Because `docket-status` runs health checks only on the
  **full pass, after** the sweep (steps 5–6 precede step 7; `--board-only` skips checks entirely), a
  legitimately-just-merged change has already been archived before the check runs; a transient
  orphan (e.g. sweep skipped because `gh` was unavailable) self-clears on the next successful sweep.
  Warn-only, like every board-check.
- **`unknown-commit-ref`** (class 3): for each extracted id with **no** change file in `active/` or
  `archive/`, emit a finding. Change-id column = the referenced id (as `malformed-id` already does
  with a raw string); message names the evidence commit. The conservative extraction grammar is what
  keeps this from firing on PR numbers.

### Touchpoints (for the implementer)

1. **`scripts/board-checks.sh`** — add the two checks. Reuse `lib/docket-frontmatter.sh` and the
   existing `active/`+`archive/` file walk to know which ids exist and their terminal state; add one
   `git log` pass over `origin/<integration_branch>` subjects for the extraction. Keep findings in
   the shared accumulation so sorting/`--strict`/exit codes are inherited.
2. **`scripts/board-checks.md`** — document `merged-orphan` and `unknown-commit-ref` under *Check
   enumeration*, the subject-only extraction grammar, and the full-history window. (Contract lives
   beside the script — ADR-0012 discipline.)
3. **`scripts/docket-status.md`** — extend the `check <check-id> …` readiness/vocabulary note to
   mention the two new ids (the `check` line shape itself is unchanged).
4. **`tests/test_board_checks.sh`** — add hermetic cases using the existing `GIT` mock seam: a
   merged-orphan (id in a fixture integration-branch subject, change still `active/`), a swept
   change (same id but archived → **no** finding), an `unknown-commit-ref` (id with no file), and
   negatives that must **not** fire: a bare `#NNNN` PR number, a body-only mention, and a
   terminal-publish `docket(<id>)` subject for an already-archived change.

## Out of scope

- **Class 2 — the archived-but-unpublished terminal-record detector — is #0083's** (see *Scope
  boundary*). Not built, not decided here.
- **Auto-healing** (publishing missing records, archiving orphans, editing any change file). The
  check reports; remediation is a human/other-change decision. `board-checks.sh` is warn-only by
  invariant and stays so.
- **Enforcing a commit-message id convention going forward.** Detection works over whatever ids the
  grammar parses; tightening the convention is separate.
- **A `--since` history window / persisted high-water mark** — additive follow-up if history grows.
- **Widening the extraction grammar** (bare `#NNNN`, merge-subject branch-name parsing) — deferred
  as the false-positive-prone options; the conservative floor ships first.

## Testing considerations

- Drive the new `git log` reads through the existing `GIT="${GIT:-git}"` mock seam so cases stay
  hermetic (no real integration-branch history required).
- Assert deterministic ordering: new findings sort into the shared `(check-id, change-id)` order.
- The negative cases (PR-number `#NNNN`, body mention, terminal-publish subject of an archived
  change) are the load-bearing false-positive guards — they encode Open Q1's resolution.
- Confirm the full existing `test_board_checks.sh` stays green (the two checks are purely additive).

## Dependency state

`depends_on: []` — nothing gates this change; build-ready now. `related: [23, 83]`: #0023
(`board-checks.sh` + ADR-0012) is `done` — the home and boundary this builds on. #0083 is
`proposed` but **auto-groom blocked (needs human)** and is *not* being groomed concurrently; this
change neither depends on nor waits for it, and deliberately excludes #0083's class-2 territory so
the two never collide. The implementer's reconcile pass should re-check #0083's state at build time
in case a human has since decided the class-2 question.

## Assumptions (deferred-human audit trail)

Each decision below is one an interactive brainstorm would have raised; the chosen default is
conservative and rooted in existing docket convention, and every rejected alternative is recorded.

1. **Ship classes 1 & 3; defer class 2 (terminal-publish gap) to #0083.** *Chosen* because class 2
   is exactly the detector #0083 abstained on — its human-only question is whether that class of
   *consciously deferred* record should be detected at all, marked, or accepted (evidence: #0043 was
   a deliberate deferral, not a bug). Building it would decide #0083's call for it, which the
   binding scope forbids ("do not touch or decide for #0083"). Classes 1 & 3 have no such
   entanglement and are the change's titular purpose. *Rejected:* (a) shipping all three — pre-empts
   #0083's undecided detector-vs-marker-vs-neither decision and risks firing on legitimate
   deferrals forever; (b) abstaining on the whole change — wastes a coherent, valuable, fully
   defaultable core (classic orphan detection) that is independent of #0083. The fold-in of class 2
   remains #0083's (and the human's) to decide and is not foreclosed.

2. **Home = extend `board-checks.sh`; no new script.** *Chosen:* it is already git-only/offline
   (matching the constraint verbatim), already receives `--metadata-branch` + `--integration-branch`,
   and `docket-status.sh` already auto-discovers its output — so "wired in as a `docket-status`
   health check" needs **zero** `docket-status.sh` change (verified against the running code, not
   just prose — `verify-the-claim`; `check-plumbing-auto-discovery`). *Rejected:* a standalone
   `orphan-check.sh` invoked separately by `docket-status.sh` — more surface (new script + contract +
   wiring) for a check that is mechanically identical in shape to the existing five, with no
   separation-of-concerns benefit since `board-checks.sh` already reads git history and both branches.

3. **Extraction grammar = numeric conventional-commit scope + trailing `(change N)`; exclude bare
   `#N` and bodies.** *Chosen* as the conservative floor that bounds the capture's named
   false-positive (bare `#N` ↔ PR numbers) while covering the two forms docket actually emits.
   Subjects only, to exclude free-text mentions. *Rejected:* including bare `#N` (collides with PR
   numbers — the exact hazard the capture flagged); parsing merge-subject branch names and bodies
   (higher recall, materially higher false-positive rate). Widening is additive and left as a noted
   follow-up.

4. **History window = full history, stateless, every run.** *Chosen:* `board-checks.sh` persists no
   state, the "pure git reads" constraint discourages a cursor side-file, full scans are
   deterministic, and the repo is small. *Rejected:* a persisted high-water mark / `--since` cursor
   — introduces state and a staleness class for a performance problem that does not exist yet; kept
   as an additive follow-up for if/when history grows.

5. **Detection only, warn-only, no auto-fix.** *Chosen:* the capture scopes this to detection and
   `board-checks.sh` is warn-only by invariant; healing an orphan (sweeping it to `done`) or
   publishing a missing record are terminal-transition side effects that belong to the sweep /
   `docket-finalize-change` / #0083, not a health check (ADR-0012: shared terminal-transition
   mechanics are never re-implemented per-caller). *Rejected:* auto-sweeping detected orphans —
   couples a warn-only checker to terminal-transition mechanics and can act on a transient
   just-merged state.

6. **`merged-orphan` accepts transient false positives from a skipped sweep; relies on post-sweep
   placement.** *Chosen:* `docket-status` runs health checks only on the full pass, after the sweep,
   so a legitimately-just-merged change is already archived when the check runs; a residual orphan
   from a `gh`-unavailable sweep is a real advisory signal that self-clears next pass. Warn-only
   makes an occasional transient acceptable. *Rejected:* suppressing the check unless the sweep
   succeeded (couples two independent passes and hides genuine orphans when `gh` is flaky) — the
   whole point is a git-only signal that does not depend on the PR-status path.
