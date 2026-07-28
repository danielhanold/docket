<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0145 — docket-status SKILL.md restates a stale check count and list the 0111 guard does not pin](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0145-docket-status-skill-md-restates-a-stale-check-count-and-list.md)**
<!-- docket:backlink:end -->

# Design — `docket-status` SKILL.md restates a stale check count and list (change 0145)

## Problem

`skills/docket-status/SKILL.md`'s `### Health checks` section opens:

> Flag the following (do not auto-fix unless asked). Five mechanical, git-only, warn-only checks run via
> `docket.sh board-checks` against the shared dependency-resolution pass:

and then enumerates exactly five check-ids: `broken-spec`, `broken-plan-results`, `dep-cycle`,
`stale-in-progress`, `merge-gate-stall`.

The real vocabulary is **twelve** on `main` (`BOARD_CHECK_IDS`, `scripts/lib/docket-frontmatter.sh`) — and
**thirteen** once change 0117's PR #129 merges, which adds `adr-unpublished`. So the section is missing
`board-row-dropped`, `field-domain`, `malformed-id`, `merged-orphan`, `publish-deferred`,
`stale-finalize-blocked`, `unknown-commit-ref`, and the count word is wrong by more than a factor of two.

The same block prints a hand-run invocation that omits flags the script has since gained
(`--lease-ttl-hours` on `main`; `--adrs-dir` and `--terminal-publish` once #129 lands).

The staleness is **structural**. Change 0111 built a correspondence guard (in `tests/test_board_checks.sh`)
that pins the check-id vocabulary as a *set* across four surfaces — `BOARD_CHECK_IDS`, `board-checks.sh`'s
own `check-id ∈ {…}` header, `scripts/board-checks.md`'s per-check sections, and `scripts/docket-status.md`'s
`check <check-id>` row. `skills/docket-status/SKILL.md` is not one of them, so every future check-id drifts
there silently while the guard stays green — the repo's own `correspondence-guard-runs-one-way` finding,
re-occurring.

## Decision

**Remove the restatement; do not add a fifth pinned surface.**

The stub already prefers this ("Prefer whichever removes the restatement rather than adding a fifth place to
maintain it"), and it is the right call on the merits: a fifth pinned surface makes every future check-id a
five-file edit and grows the guard, whereas removal makes the drift *impossible* rather than *detected*. The
repo has the same instinct recorded as `verify-the-claim` — a document asserting a fact about another
artifact is not an oracle.

### 1. What SKILL.md keeps

SKILL.md legitimately owns the **posture** and the **agent's obligations**, which are not restatements of any
other surface:

- the checks are mechanical, git-only, and **warn-only** — never auto-fix unless the user asks;
- a one-line **cross-reference** to `## Judgment follow-ups` for the two in-model judgment checks
  (`blocked_by:` re-examination and `github` mirror reachability). The instruction for both already lives in
  full at that section (SKILL.md lines 60 and 62); the Health-checks line is *already* a pointer ("see
  *Judgment follow-ups* above") and stays exactly that — it is not duplicated content to preserve;
- a one-line characterization of what the checks are *about* — stale claims, broken links, dependency
  stalls — matching the verbatim wording already in SKILL.md's frontmatter `description:` ("stale claims,
  broken spec/plan/results links, and dependency stalls") and its `## Overview` paragraph.

### 2. What SKILL.md drops

- The count word ("Five").
- The five-item check-id list.
- The hand-run `docket.sh board-checks` invocation block. This skill does **not** run `board-checks.sh`
  directly — it invokes `docket.sh docket-status`, which runs the checker itself, and SKILL.md already says
  (§"The script owns the mechanics of what it renders, sweeps, and checks — see `scripts/docket-status.md`").
  A copied invocation that no path in the skill executes is a third restatement, and it is already wrong.

Both are replaced by a pointer to the authoritative enumeration: `scripts/board-checks.md`'s per-check
sections for what each check means, `scripts/docket-status.md`'s `check <check-id>` row for the closed set as
it reaches this skill's report.

### 3. A guard so the restatement cannot return

Removal alone leaves nothing stopping a future editor from re-adding a list. Add one assert to
`tests/test_board_checks.sh`, adjacent to the existing 0111 block (that file is where this guard family
lives, and it already derives `$emitted` — the assert reuses it rather than re-deriving):

**Placement.** After the extractor-integrity block and immediately **before** the file's
`PASS`/`exit "$fail"` epilogue — stated structurally, deliberately not as a line number. This matters: 0117's PR #129 has exactly two hunks in this file,
`@@ -994,6 +994,340 @@` and `@@ -1093,8 +1427,10 @@` — the second *is* the count-assert region, so
"adjacent to the 0111 block" would land the new assert inside a hunk 0117 rewrites. The end-of-file position
sits outside both.

**Scoped negative + positive anchor.** Extract only the `### Health checks` section of
`skills/docket-status/SKILL.md` — from that heading to the next `^#{1,3} ` heading **or EOF**. The EOF arm is
the **live** path, not a fallback: `### Health checks` is currently the file's *last* section (line 92 of
107), so an extractor written as "lines between two heading matches" would extract **nothing** and the
negative assert would pass vacuously forever. Then assert:

- *(positive, non-vacuity)* the extracted section is non-empty **and** contains the pointer to
  `scripts/board-checks.md`. Without this the negative assert passes vacuously the moment the heading is
  renamed — the exact trap the 0111 block already documents for its own extractors ("a retitled section head
  must redden, not pass vacuously").
- *(negative)* **no** check-id from `$emitted` appears in that section. The **required** matcher is
  word-boundary (`grep -w` over the extracted section), never a bare substring and **never backtick-anchored**:
  a backtick-only match would miss a list re-added in bare form (`- broken-spec — …`), and since the mutation
  check will naturally be written by copying today's backticked list, a backtick-anchored guard would pass its
  own mutation test while leaving the hole open. There is no false-positive cost — all twelve emitted ids are
  hyphenated compounds that cannot occur in ordinary prose. Accordingly the mutation check must re-add the id
  **unbackticked**.

The negative must be **scoped to the section**, not the file: SKILL.md legitimately names `publish-deferred`
at line 86 (the sweep-posture paragraph, explaining what the mark drives) — the file's only check-id
occurrence outside this section — so a file-wide ban would redden honest prose.

**Named limitation.** Section scoping means the guard stops the restatement from returning *in this section*
only. A future editor who re-adds the list under a **new** heading escapes it — the non-vacuity anchor catches
a *rename of* `### Health checks`, not a *new* section elsewhere. The middle option (file-wide ban with an
occurrence allowance for the single legitimate line-86 mention) was considered and rejected: it pins prose
position and is more brittle than what it buys. The limitation is recorded here so nobody later reads the
guard as stronger than it is.

## Test

The guard above is itself the test. Its own mutation checks:

- re-add one check-id to the `### Health checks` section, **unbackticked** → must redden.
- rename the `### Health checks` heading → must redden on the non-vacuity anchor, not pass silently.
- leave the section as designed → green.

No other test changes. `tests/test_skill_size_budgets.sh` covers SKILL.md's size and can only benefit.

## Out of scope

- The check-id vocabulary itself, and any check's behavior.
- The four already-pinned surfaces, which are correct and guarded.
- Auditing other skills for the same restatement class. Real, but a separate sweep.

## Assumptions

1. **Remove the restatement rather than pin a fifth surface.** The stub states the preference; the merits
   agree (removal eliminates the drift, pinning only detects it, and pinning taxes every future check-id).
   Rejected alternative: add SKILL.md to change 0111's guard — verified that would mean editing the
   `comm -3` block in `tests/test_board_checks.sh` and making every new check-id a five-file edit.
2. **SKILL.md keeps the posture, and keeps its existing one-line cross-reference to
   `## Judgment follow-ups`.** An earlier draft justified this as "content restated nowhere else"; that is
   verifiably wrong — both judgment checks are stated in full at SKILL.md lines 60 and 62, and the
   Health-checks line is already a pointer to them, not a copy. The decision stands on the correct reason:
   the pointer is one line, it is accurate, and it is not a restatement of a machine-owned vocabulary — so
   the removal this change performs does not reach it.
3. **Drop the hand-run invocation block entirely rather than correct it.** Verified the skill's own text
   already delegates mechanics to `scripts/docket-status.md`, and the skill's execution path is
   `docket.sh docket-status`, never a direct `board-checks` call. Rejected alternative: fix the flags — that
   re-creates the same drift with fresher wrong values, and `--lease-ttl-hours` would be the next thing to go
   stale.
4. **The guard is a section-scoped negative with a positive non-vacuity anchor, terminated on
   `^#{1,3} ` OR EOF, matching ids by word boundary (`grep -w`).** Verified the necessity of scoping: `publish-deferred`
   legitimately appears at SKILL.md line 86 (the sweep-posture paragraph) and is the file's only such
   occurrence, so a file-wide ban would redden honest prose. Verified the necessity of the EOF arm:
   `### Health checks` is the file's last section, so a two-match extractor yields the empty set and the
   guard is vacuous from birth — the non-vacuity anchor is what catches that, and the 0111 block already
   applies exactly this pattern to its own extractors. The matcher is `grep -w`, **not** backtick-anchored —
   a backtick-anchored guard would miss a bare-form re-add *and* pass the mutation check written from
   today's backticked list. Rejected alternatives: a bare substring `grep` over the whole file (wrong on
   both scoping and delimiters); a file-wide ban with an occurrence allowance for line 86 (pins prose
   position, more brittle than it is worth — see §3's named limitation).
5. **The guard lives in `tests/test_board_checks.sh`, beside the 0111 block.** Verified that is where the
   correspondence-guard family lives and where `$emitted` is already derived from `board-checks.sh`, so the
   new assert consumes the real emitted set rather than a hand-kept list (the 0111 block's own rule).
   Rejected alternative: `tests/test_docket_status.sh` — that file guards the orchestrator's report contract,
   not the check-id vocabulary.
6. **`related: [117]` — file collision in `tests/test_board_checks.sh`, made additive by placement.**
   Verified 0117 is `status: implemented`, `pr: #129`, unmerged; its branch adds ~340 lines to
   `tests/test_board_checks.sh` in **two hunks** (`@@ -994,6 +994,340 @@` and `@@ -1093,8 +1427,10 @@`),
   the second of which is the count-assert region that raises `BOARD_CHECK_IDS holds the 12 check-ids` to 13.
   It does **not** touch `skills/docket-status/SKILL.md`. The collision is therefore avoidable by placement,
   not merely by good intentions — hence the explicit end-of-file placement rule in §3, outside both hunks;
   `concurrent-edits-compose-at-rebase` then applies cleanly. **Not** `depends_on:` — nothing in 0145 needs
   0117's content, and gating build-readiness behind a human merge would be cost with no benefit. 0117 makes
   this change *more* valuable: it adds the 13th check-id SKILL.md would silently miss.
7. **`related: [144]` — subject overlap; file-disjoint only by *prediction*, and the prediction has a known
   soft spot.** 0144 is `proposed` with **no spec** (auto-groom abstained on it on 2026-07-28;
   `auto_groomable: false`, `## Auto-groom blocked` present), so its file set is inferred from its stub and
   its abstain record, **not** observed from a design or a diff — the word "verified" would be unearned.
   Predicted edits: `scripts/docket-status.sh`, `scripts/docket-status.md`, `tests/test_docket_status.sh` —
   disjoint from 0145's `skills/docket-status/SKILL.md` + `tests/test_board_checks.sh`. **The soft spot:**
   0144's open question is whether the health pass should emit a *distinguishable diagnostic line*, and its
   abstain record's settled design does propose one (`board-checks failed <exit>`).
   `skills/docket-status/SKILL.md`'s `## Read the report` section (lines 39–53) enumerates several report
   lines, including two failure-diagnostic families (it is a subset — it omits the `check`, `board …`,
   `swept`, `sweep-failed`, and `harvest` families), which makes it a plausible rather than certain landing
   site; so a 0144 that lands a new line will very likely edit the same SKILL.md 0145 edits — a
   different section, but the same file. Whoever builds second must re-check. `related:` is the right field
   either way; the reciprocal `related: [145]` already exists on 0144's manifest.
8. **No ADR.** A documentation-accuracy fix plus one guard. The general rule it exemplifies — *do not restate
   a closed vocabulary you do not own* — is already recorded as the `correspondence-guard-runs-one-way` and
   `verify-the-claim` learnings, and is a **learnings** extension at close-out, not a new decision record.
9. **Count as of drafting: 12 on `main`, 13 after #129.** Verified by reading `BOARD_CHECK_IDS`
   (`scripts/lib/docket-frontmatter.sh`) and the `BOARD_CHECK_IDS holds the 12 check-ids` assert. The stub's
   "thirteen" anticipates 0117. Since the fix **removes** the number rather than correcting it, the exact
   value is not load-bearing for the implementation — but the implementer must not "fix" the count and stop.

## Reconcile note — 2026-07-28

Re-verified against `origin/main` at `f804c7b2` when change 0145 was claimed. The design stands
unchanged; two of its *pending* premises have settled:

- **0117 (PR #129) merged.** `BOARD_CHECK_IDS` now holds **thirteen** ids on `main`, so Assumption 9's
  conditional count is a flat 13 — still not load-bearing, because the fix removes the number.
  Assumption 6's file-collision risk in `tests/test_board_checks.sh` is **retired**: 0117's two hunks
  are already on `main`. §3's end-of-file placement rule is retained on its own merits — the
  `PASS`/`exit "$fail"` epilogue is the stable structural anchor, and it keeps the new assert clear of
  the count-assert region 0117 just rewrote.
- **The structural claims re-verify.** `### Health checks` is still the last section of
  `skills/docket-status/SKILL.md` (lines 92–107 of 107), so the extractor's **EOF arm is the live
  path**; line 86 remains the file's only check-id occurrence (`publish-deferred`) outside that
  section, so the negative assert stays **section-scoped**. `$emitted` is still derived at
  `tests/test_board_checks.sh:1447`.

Assumption 7 (change 0144's predicted overlap in this same SKILL.md's `## Read the report` section)
is **unchanged and still open** — 0144 remains `proposed` with no spec.
