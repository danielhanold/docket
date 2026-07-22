# Unpublished-ADR detection — a computed `board-checks` finding, not a marker

**Change:** #0117 · **Date:** 2026-07-21 · **Status:** design settled, ready to build

## 1. Problem

`docket-adr`'s publish-onto-integration path sits behind the same protected-`main` wall that
change #0083 addressed for terminal *change* records — and #0083 deliberately left it unwired,
calling the omission out in its own spec §5 so it would be a decision rather than an oversight.

The result is an asymmetry. After #0083, a deferred or blocked **change** publish is durable and
visible: a `## Publish deferred` marker on the change file plus a `publish-deferred` health-check
finding. A deferred or blocked **ADR** publish is still invisible — it lives only in a chat
thread, which is precisely the #0043 failure mode that went unnoticed for eight days.

### 1a. The path is live, and has already failed

Not hypothetical. ADR-only publishes run regularly on this repo — `ADR-0024`, `ADR-0034`,
`ADR-0053`, plus status-flip re-publishes for `ADR-0017`, `ADR-0039`, `ADR-0042` all appear as
their own commits on `main`. Two failures on that path are recorded in session notes: a
publish-onto-`main` step refused mid-run by Claude Code's auto-mode classifier, and a superseded
ADR silently skipped by finalize's `Accepted` gate, requiring a hand-run
`terminal-publish.sh --adr N` afterwards.

`docket-adr` reaches `terminal-publish.sh --adr NN` on three paths, two of which are ADR-only and
therefore uncovered by #0083's change-file marker:

| Path | Covered by #0083? |
|---|---|
| Change-tied ADR (rides its change's terminal publish) | yes — the marker lands on the change file |
| Standalone ADR (`docket-adr` invoked directly) | **no** |
| Status flip / `## Update` re-publish of an already-published ADR | **no** |

### 1b. Measured state of the ledger

Taken 2026-07-21 against `origin/docket` and `origin/main`:

- 53 ADRs on `docket`, 52 on `main`.
- Of the 52 present on both, **zero** differ byte-for-byte — every status flip to date has been
  re-published correctly.
- Exactly one ADR is absent from `main`: `ADR-0023` (`Accepted`, `change: 44`).

`ADR-0023` is **not** a gap. Change #0044 is `blocked` — never built, never closed out — and
docket's rule is that a change-tied ADR rides its change's terminal publish. So it is correctly
absent, and a check must stay silent about it. This is load-bearing: it is the single data point
that rules out the naive formulation (§3, option i).

## 2. Goal

Make an ADR publish that did not happen **visible**, durably, without requiring the run that
failed to have noticed it failed.

**Non-goals.** No auto-healing or re-publishing. No set-diff over *change* records (#0083's
decline stands there; #0118 owns the adjacent question). No re-opening the `terminal_publish`
knob's semantics, and no attempt to change the branch-protection policy — that wall is the
maintainer's, not docket's.

## 3. Decision: detect, don't mark

**Chosen: a computed `board-checks.sh` finding comparing the ADR set on the metadata branch
against the integration branch.**

### Why not a marker

A marker on the ADR body (extending `mark-publish-deferred.sh`) was the symmetric option, and was
rejected on three grounds:

1. **It only fires if the failing run noticed.** A marker is written by a compliant driver on a
   defer path. #0043's failure mode was that *nobody noticed*; the classifier-denial and
   skipped-gate failures on the ADR path are the same shape. A computed check needs nothing at
   all from the run that went wrong, and additionally catches stale bytes from an un-re-published
   status flip — which a marker structurally cannot.
2. **There is no seam to hang it on.** An ADR file is never moved, so there is no archive moment;
   and an `Accepted` ADR is immutable except its `status:` line, so a body marker bends the
   repo's own rule. The `change:` back-link is not a fallback — a standalone ADR has none, and a
   status flip's producing change is long since archived *and already published*, so marking it
   would itself require a re-publish.
3. **Cost.** A marker needs a writer, a removal path wired into `terminal-publish.sh`'s success
   branch, and the same multi-site check-id registration #0083 had to perform by hand. The
   computed check needs none of it and self-heals by construction.

### Relationship to ADR-0051 — a narrowing, not a reversal

ADR-0051 (*"marker, not branch-diff detector"*, change #0083) declined a detector for change
records. That decline does not bind here, and the distinction should be recorded:

- ADR-0051 declined a **detector *and healer*** that would re-publish what was missing, on the
  grounds that the realized gap was a *conscious human deferral* a healer would have silently
  reversed. This change builds a **read-only report**, heals nothing, and reverses no decision.
- ADR-0051's second ground was the `relax-the-policy-before-building-the-workaround` learning:
  don't build machinery to route *around* a maintainer-owned wall. A check that tells you the
  wall stopped you is not routing around it — it is the visibility fix that learning explicitly
  endorses.
- ADR-0051's own *Consequences* names the residual this change closes: *"a terminal record that
  goes missing via a path that writes NO marker … is still not caught. This is the accepted cost
  of 'mark, don't detect'."* For the ADR corpus, where the marker seam does not exist at all,
  that cost is not worth accepting.

One correction to the record. `board-checks.sh`'s inline comment gives three reasons for rejecting
a set-diff, and the third — that it would *"break the script's git-only/offline invariant"* — does
not hold. The script already runs `git cat-file -e <ref>:<path>` for both link checks. A presence
probe against a local branch ref is the same shape and needs no network. The other two reasons
(the declined detector, and firing forever under `terminal_publish: false`) are real; the second
is handled by the gate in §4.

**Expect one new ADR at build time** recording this boundary: *detect where there is no marker
seam and no healer; mark where a conscious human deferral is the failure mode.* It relates to
ADR-0051 and supersedes nothing.

## 4. Design

### 4.1 Placement — `board-checks.sh`

The check lands in `scripts/board-checks.sh`, for two reasons that both point the same way:

- **Visibility.** `board-checks.sh` runs on every `docket-status` pass. `adr-checks.sh` — the
  topically natural home — is invoked by exactly one caller, `docket-adr`, so it only runs when
  you are already creating or superseding an ADR. A finding there would surface almost never,
  which defeats the purpose of a visibility fix.
- **Dependencies.** The due rule (§4.2) needs change statuses, and `board-checks.sh` already has
  `--changes-dir` and a shared dependency-resolution pass. Placing the check in `adr-checks.sh`
  would mean giving it git access, branch args, a config gate, *and* `--changes-dir` — i.e.
  reconstructing `board-checks.sh`'s signature plus `--adrs-dir`, with a duplicated pass behind
  it.

`board-checks.sh` gains one new argument, `--adrs-dir`, and one gate flag, `--terminal-publish`.

**Considered and declined (2026-07-21, do not re-raise without new evidence):** wiring
`adr-checks.sh` itself into the `docket-status` health pass, so its three existing checks
(numbering gaps, dangling links, status inconsistencies) run on every pass rather than only under
`docket-adr`. Declined because `adr-checks.sh` runs as part of `docket-adr`'s Index/validate step
— i.e. on every ADR create and supersede, which is precisely when those three checks could newly
break. What it misses is drift introduced by some *other* path (a hand-edit, or a change touching
an ADR outside `docket-adr`), which is rare and cosmetic when it happens; a dangling `relates_to:`
does not compound. The visibility argument that decides *this* check's placement is specific to a
publish gap, where being told promptly is the whole point, and does not generalize to
ledger-hygiene checks. No stub was minted for it — this paragraph is the durable record.

### 4.2 The due rule

An ADR's publish is **due** when its publish trigger has already fired:

| ADR shape | Due when |
|---|---|
| No `change:` back-link (standalone), `status: Accepted` | immediately |
| Has a `change:` back-link | that change's status is `done` or `killed` |
| Already present on the integration branch | always (bytes must match, whatever the status) |

The third row is what catches an un-re-published status flip, and it is deliberately status-blind:
a `Superseded by ADR-NN` ADR that was published while `Accepted` must still track its current
bytes. An ADR that is *not* `Accepted` and *not* already on the integration branch is never
expected there — that is the common shape for decisions made and reversed under
`terminal_publish: false`, and flagging it would be a false positive.

Applied to today's ledger, the rule yields **zero findings** (§1b).

### 4.3 The two arms

Both probe local branch refs via `git cat-file`; no network, no `gh`.

- **missing** — the publish is due, but the ADR is absent on the integration branch.
- **stale** — the ADR is present on both branches and the bytes differ.

One check-id, `adr-unpublished`, with two distinct messages. This follows the precedent
`stale-in-progress` already sets (one check-id, two trigger messages), rather than minting two
ids for one condition.

### 4.4 The gate

The check emits nothing unless **both** hold:

- `terminal_publish: true` — under the default `false` the ledger deliberately lives on the
  metadata branch only, and an ungated check would fire on every ADR forever.
- **docket-mode** — in `main`-mode the metadata and integration refs coincide, so the comparison
  is vacuous.

`docket-status.sh` passes `--adrs-dir` and `--terminal-publish` through from resolved config.

**Retroactivity is accepted, not engineered around.** `terminal_publish` is not retroactive —
flipping it from `false` to `true` does not backfill old ADRs, so a repo that flips it may see a
burst of legitimately-never-published ADRs. On this repo that burst is measured at zero (the one
absent ADR is not due). The check is warn-only, `--strict` is opt-in, and no baseline or
suppression mechanism is being built for a problem no repo currently has.

### 4.5 Registration

`adr-unpublished` joins the closed check-id vocabulary, which is pinned in four places by
`tests/test_board_checks.sh`:

1. `BOARD_CHECK_IDS` in `scripts/lib/docket-frontmatter.sh`
2. `scripts/board-checks.sh`'s `--help` header enumeration
3. `scripts/board-checks.md`'s *Check enumeration*
4. `scripts/docket-status.md`'s closed `check <check-id>` set

Missing any one reproduces exactly the drift change #0104 had to repair by hand. Change #0111's
guard enumerates rather than hardcodes, so it should cover the new id for free — verify this at
build time rather than assuming it.

The check is **orthogonal to the `DROPPED`/`EXPLAINED` suppression machinery**: it concerns ADR
files, not change rows, so it can never drop a board row and neither marks `EXPLAINED` nor feeds
`board-row-dropped`.

## 5. Open question for build time

**The `<change-id>` column.** `board-checks.sh` findings are
`<check-id>\t<change-id>\t<message>`, and ADR-0049 (*findings channel — structural columns,
validated values only*) constrains that column. A standalone ADR has no change id to put in it.
Two candidate resolutions: emit the producing change id when the `change:` back-link is present
and a validated placeholder otherwise; or widen the column's documented contract to admit an ADR
reference. Settling this requires reading ADR-0049's actual wording and the `cid` convention
change #0104 introduced — a build-time read, not a design call. The choice must not weaken the
validated-values rule to make the new check convenient.

## 6. Testing

- **Due rule, per row of the §4.2 table** — including the negative cases: a change-tied ADR whose
  change is not terminal (the `ADR-0023` shape) emits nothing; a non-`Accepted` ADR absent from
  the integration branch emits nothing.
- **Both arms** — a due-and-missing ADR, and a present-but-drifted ADR, each with its own message.
- **Both gate legs** — `terminal_publish: false` emits nothing; `main`-mode (coincident refs)
  emits nothing. These are live paths on this repo (`terminal_publish: true` today), so they must
  be tested rather than assumed unreachable.
- **Registration** — the existing four-site pin test must cover `adr-unpublished`.
- **Offline** — the check must work with no network, against local branch refs only.

## 7. Out of scope

- A set-diff or audit over **change** records — ADR-0051's decline stands; #0118 owns the
  adjacent skip-publish question.
- Any healer, re-publisher, or auto-fix. Report only.
- Publishing `ADR-0023`. Under the due rule it is correctly absent.
- Wiring `adr-checks.sh` into the `docket-status` health pass — considered and **declined**, not
  deferred; see §4.1 for the reasoning.
- The classifier / branch-protection / `--admin` policy. Not docket's to change.
