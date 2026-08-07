<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0154 — Audit skill bodies for the stale-restatement class change 0145 closed in one file](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0154-audit-skill-bodies-for-the-stale-restatement-class-change-01.md)**
<!-- docket:backlink:end -->

# Design: audit skill bodies for the stale-restatement class change 0145 closed in one file (change 0154)

Auto-groomed 2026-08-07 (docket-auto-groom; no human in the loop — decisions below are
committed defaults, auditable in `## Assumptions`).

## Problem

Change 0145 removed one stale restatement (a check count + check-id list + hand-run invocation
block) from `skills/docket-status/SKILL.md` and guarded that one section. Its `## Out of scope`
named the residue: no other skill file was audited for the same class. The class is structural —
any skill body that enumerates a closed vocabulary owned by a script contract or lib array (check
ids, report-line tokens, flag lists, exit codes, config-key sets), or restates a count of one, is
an unpinned surface that drifts silently while the 0111 guard stays green.

The class is provably live: since 0145 the check-id vocabulary grew from thirteen to **fifteen**
(`aborted-run`, `scalar-form` in `BOARD_CHECK_IDS`, `scripts/lib/docket-frontmatter.sh:435`), and
`skills/docket-status/SKILL.md:35`'s normal-outcomes enumeration still omits the
`health checks failed <exit>` line change 0144 added (absorbed change 0159).

## Goal

One docs-type PR that (1) inventories the `skills/` tree for the class, (2) applies a single
disposition rule per hit — prefer removal-plus-pointer — and (3) settles the "generalized guard?"
question the stub poses. No vocabulary, exit-code, or script-behavior changes.

## Sweep scope

- **In:** every markdown file under `skills/` — the 12 `SKILL.md` files, all
  `skills/*/references/*.md`, and `skills/docket-convention/github-board-mirror.md` (~2.9k lines).
- **Out:** `scripts/*.md` contracts (they are the *owners*; contract-to-contract duplication is a
  different ownership question — report any found in prose, change nothing), the four surfaces
  0111 pins, and `skills/docket-status/SKILL.md`'s `### Health checks` section (0145 closed and
  guarded it).

## Decision rule (per hit)

**In-scope hit:** an enumeration of a closed set owned elsewhere (script contract, lib array,
generated-file glob), a count of such a set, or a near-verbatim multi-sentence restatement of a
contract's mechanics.

**Exempt (not hits):**
- a *single-item* cross-reference naming one id/token as a pointer (e.g. "the `publish-deferred`
  health check surfaces it") — the 0145 precedent already left `SKILL.md`'s lone out-of-section
  check-id mention alone;
- definitional canon the skill itself owns (the convention's manifest schema, lifecycle table,
  body-section contract, ADR format — the convention IS their source);
- the literal facade invocation a skill runs (`docket.sh docket-status --board-only --must-land`
  is the command, not a restatement);
- the convention's `.docket.yml` schema snippet (config-snippet drift is the 0107/0108 guard
  family; the build verifies a guard covers it and reports in prose if not — no edits here).

**Disposition preference, strict order:**
1. **Delete-and-point** at the owning contract (drift becomes impossible);
2. **Compress to the judgment the skill owns** + pointer (when the block mixes owned posture with
   restated mechanics);
3. **Pin** 0111-style (only if an enumeration must survive verbatim on an always-loaded surface —
   expected count after this sweep: zero).

## Named hits and committed dispositions

Verified live 2026-08-07; the build re-verifies each and completes the inventory.

- **H1 — `skills/docket-status/SKILL.md:35`**, the normal-outcomes token list and the non-zero-exit
  causes list. *Delete-and-point:* replace both enumerations with "every stdout report line the
  contract documents is a normal outcome; non-zero exit is the hard-error channel" plus the pointer
  to `scripts/docket-status.md`'s report-line/exit-code sections. **Keep** the owned posture: trust
  the exit code, the Board-pass-is-narrower caveat (with its named `board …` failure keying, which
  the convention's *Board refresh on status writes* already owns — point there). This **absorbs
  killed change 0159** by construction: no list, nothing to omit.
- **H2 — `skills/docket-status/SKILL.md:90`**, the ~400-word "Sweep posture" restatement of
  `scripts/docket-status.md` §6/§6a. *Compress to the judgment kernel* (~5 lines): reasons differ —
  never read a `sweep-failed render-change-links` line alone as record-unpublished; check the
  reason token and cross-check `swept`/`harvest` for the same id; the abandon-shaped reasons need
  the manual follow-up the contract names; posture is deliberately divergent from finalize's
  abort-and-report. Everything mechanical (per-reason narrative, mark-publish-deferred sequencing,
  change numbers) deletes in favor of the `scripts/docket-status.md` pointer.
- **H3 — `skills/docket-convention/SKILL.md:55`**, the coordination-key fence *list*.
  *Delete-and-point, minus one named key:* the fence is enforced by `docket-config.sh` at resolve
  time, and the paragraph already declares `scripts/docket-config.md`'s per-key table
  authoritative — the inline key list is a second copy of a growing closed set (0190 just added a
  member). Two test pins constrain the surviving sentence: `tests/test_sync_agents_drift_docs.sh:472`
  pins the words "fence" and "per-repo-only" in this file, and `tests/test_docket_config.sh:858`
  asserts `terminal_publish` appears within 2 lines of the phrase "Coordination-key fence" (the
  0064 end-to-end doc guard). So the fence sentence keeps "coordination-key fence",
  "per-repo-only", warned-and-ignored, never fatal, ADR-0019, **and names `terminal_publish` as
  its worked example** (a single-item cross-reference the exemption category permits) — the rest
  of the enumeration deletes in favor of the table pointer. Both tests must stay green.
- **H4 — `skills/docket-convention/SKILL.md:63`**, `board_surfaces` semantics. *Partial:* keep the
  definitional sentence (the board as 0..n derived views; `inline`/`github`; `[]` disables) — the
  convention owns the concept — and handle the three restated resolver behaviors individually:
  the unknown-token warn trims to a pointer at its owner, `scripts/board-refresh.md` (NOT
  `docket-config.md`, whose :120 drop is the different machine-layer stage); mint-on-first-sync
  trims to the existing `github-board-mirror.md` pointer; **"a non-GitHub remote silently drops
  `github`" is stated nowhere else** — the builder verifies it against code and keeps or relocates
  it (to the owning contract, then points), never silently deletes its only statement.
- **Exempt confirmations:** the convention Agent-layer wrapper counts (`SKILL.md:101,:105` —
  "Seven skills", "sixteen generated wrappers", "seven wrapper-bearing exceptions") are an
  **already-pinned surface**, not a member of the class: `tests/test_finalize_gate.sh:152-156`
  pins "sixteen", forbids "thirteen", and pins "seven skills get a wrapper" as deliberate
  non-vacuous count guards (change 0170; cf. `test_skill_size_budgets.sh:132-137`) — leave them
  alone, parallel to the out-of-scope 0111 surfaces. Likewise single-item cross-references:
  `skills/docket-convention/SKILL.md:192` (`publish-deferred`),
  `skills/docket-finalize-change/references/gate-failure.md:31` (`stale-finalize-blocked`), and
  `skills/docket-convention/references/terminal-close-out.md:92,:101` are single-item
  cross-references — keep, verifying wording against the owning contract at build.

## Generalized guard: NO

The stub's "no skill body names a check-id outside a sanctioned section" guard is rejected:
- legitimate single cross-references exist (H5-exempt list above), so the guard needs a sanction
  list — a new closed set that itself drifts, recreating the disease as the cure;
- after this sweep the surviving skill-side vocabulary surfaces are the already-guarded ones —
  0111's four pinned surfaces, 0145's guarded section, and the 0170 count pins
  (`test_finalize_gate.sh:152-156`) — removal handles the unguarded rest;
- prose-shaped mirrors over 12 files are high-false-positive maintenance (every reworded sentence
  becomes a test edit).

If — against expectation — a hit genuinely requires disposition 3 (pin), the build extends the
0145 section-scoped pattern for that one site rather than building anything repo-wide.

## Inventory procedure (build step 1)

Mechanical seeds, each grepped as whole words across `skills/**/*.md`: `BOARD_CHECK_IDS` members
(sourced from the lib, not hand-copied), the report-line first tokens from
`scripts/docket-status.md`'s report table, `DOCKET_STATUSES`, the fenced-key rows of
`scripts/docket-config.md`'s table, and `docket.sh` op/flag names. Then one full manual read of
all in-scope files flagging count words adjacent to enumerations and multi-sentence
contract-mechanics blocks. The inventory (hits, exemptions, dispositions) is recorded in the
plan/results artifacts — not as a new guard.

## Collateral-test protocol (build, before every edit)

0145 found two test asserts pinning prose inside the block it deleted. So: before deleting or
compressing any block, grep `tests/` for its distinctive phrases; any assert pinning deleted prose
is retargeted to surviving text (never weakened to vacuity, never deleted without a replacement
anchor). After H1/H2, re-run `tests/test_board_checks.sh` (0145's section guard — its extractor's
EOF arm assumes `### Health checks` stays file-final; H1/H2 edit earlier sections only, but
verify) and `tests/test_sync_agents_drift_docs.sh` (H3's word pins).

## Out of scope

- Changing any vocabulary, exit code, flag, or script behavior.
- The four 0111-pinned surfaces and 0145's guarded section.
- `scripts/*.md` contract-to-contract duplication (report only).
- The convention's `.docket.yml` schema snippet and its guard status (verify + report only).

## Assumptions

1. **Sweep scope = `skills/` markdown only; `scripts/*.md` excluded.** Rejected: auditing the
   script contracts too (the stub's "where cheap") — contracts are the owners, so their
   duplications pose a different (which-copy-wins) question; folding it in balloons a docs PR.
   Cheap concession kept: report any contract-to-contract duplication found in prose.
2. **Exemption for single-item cross-references.** Rejected: treating every check-id token as a
   hit — 0145 itself deliberately left the lone out-of-section `publish-deferred` mention, and
   pointers-by-name are how removal-plus-pointer works at all.
3. **Disposition order delete > compress > pin, expecting zero pins; pointers must target the
   actual owner.** Rejected: pin-first (0111-style mirrors per file) — 0145's own rationale
   stands: removal makes drift impossible rather than detected, and pins multiply guard surfaces.
   Corollary (from H4): before pointing, verify which contract owns the behavior — the
   unknown-token warn lives in `board-refresh.md`, not `docket-config.md` — and a restated
   behavior documented **nowhere else** is relocated to its owner, never deleted.
4. **No generalized guard** (see section above). Rejected: a repo-wide check-id lint — needs its
   own drifting sanction list; per-site removal plus the existing guards is strictly cheaper.
5. **H1 deletes the outcome enumeration rather than adding the missing 0159 line.** Rejected:
   add-the-one-line (0159's original shape) — it repairs one omission while leaving the class
   armed; the vocabulary has already grown twice since the list was last true.
6. **H2 keeps an in-model judgment kernel** instead of deleting the whole sweep-posture block.
   Rejected: full delete — the never-misread-`render-change-links` rule and the swept/harvest
   cross-check are agent judgment the skill genuinely owns (the contract documents mechanics, not
   what the reading agent must conclude).
7. **H3 deletes the fence-key list but keeps `terminal_publish` named beside the fence phrase.**
   Rejected: pinning the full list against `docket-config.md`'s table — enforcement is script-side
   (warned-and-ignored at resolve), so the list is explanatory and the authoritative table is one
   pointer away. Also rejected: deleting the list wholesale — `test_docket_config.sh:858`
   deliberately pins `terminal_publish` within 2 lines of "Coordination-key fence", and the
   collateral rule forbids retargeting a guard to vacuity. Both known pins (that one and
   `test_sync_agents_drift_docs.sh:472`) stay satisfied by the surviving sentence.
8. **The convention's wrapper counts are exempt as an already-pinned surface.** Rejected: sweeping
   them (the draft's original disposition) — `tests/test_finalize_gate.sh:152-156` pins those
   exact counts as deliberate non-vacuous guards (change 0170), so by the class definition
   ("unpinned surface that drifts silently") they are not members, and removing the numerals would
   redden two guards with no surviving anchor. They sit with the 0111 surfaces: guarded elsewhere,
   out of this sweep's hands.
9. **Collateral-test protocol is mandatory per edit** (grep-first, retarget-never-delete).
   Rejected: fix-tests-when-red — 0145's collateral shows the pins are silent until the exact
   phrase dies; grep-first is cheaper than archaeology.
10. **No dependencies; design-ahead is clean.** `depends_on:` stays empty — 0111 and 0145 are
    `done`; 0144 is **killed-subsumed** (its work landed via change 0157 on a rolled-up branch —
    the `health checks failed <exit>` line verifiably shipped, `docket-status.sh:949`); 0159 is
    killed-absorbed into this change. Couplings recorded as `related: [111, 144, 157, 159]`
    (guard precedent / killed line-origin stub / the change that shipped that line / absorbed
    stub).
11. **Single PR, type docs, tests touched only where they pin deleted prose.** Rejected:
    splitting per-file PRs — every edit is the same class under one rule; one reviewable diff.
