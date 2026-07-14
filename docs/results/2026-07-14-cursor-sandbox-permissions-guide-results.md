# Cursor sandbox & permissions guide — results
Change: #73 · Branch: feat/cursor-sandbox-permissions-guide · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-07-14-cursor-sandbox-permissions-guide.md · ADRs: 33 (relates_to 29, 20, 27)

## Verify (human)

<!-- The guards prove STRUCTURE only (JSON parses, spellings present, fences byte-identical, entries
     stamped, not-exposed set complete, README links). They CANNOT prove a classifier claim is TRUE.
     The truth check is yours, at the merge gate. -->

- [ ] **Read `docs/cursor/permissions.md` against the spec's `## Appendix — verification log`**
      (on the `docket` branch:
      `docs/superpowers/specs/2026-07-14-cursor-sandbox-permissions-guide-design.md`). Confirm every
      classifier claim and every troubleshooting entry matches what the Cursor 3.11.19 / 2026-07-14
      session actually recorded — nothing exceeds or contradicts the log. (The final whole-branch
      review performed this comparison and found no divergence, but it remains the reviewer-of-record's
      call — a doc sentinel proves a sentence exists, never that it is still true.)
- [ ] **Spot-check the two fragments in a real Cursor config** if convenient: paste
      `docs/cursor/permissions.example.json` into `~/.cursor/permissions.json` (substituting your real
      clone path for `/Users/$USER/dev/docket`) and confirm a facade call (`docket.sh env`) runs
      **outside** the sandbox under Allowlist (with Sandbox). Optional — the appendix already records
      this; only worth it if your Cursor version differs from 3.11.19.

## Findings

- **The plan's own test code carried a byte-level bug (fixed in build).** Task 1's for-loop search
  literals for the two short-form spellings omitted the JSON-escaped `\"`, so they could never
  byte-match the fragment file (which stores the shell quotes JSON-escaped). The implementer caught it,
  proved the JSON semantics were already correct, and added the backslashes to those two literals only.
  Forms 2 and 3 (`/docket.sh` outside vs inside the closing quote) are now genuinely distinguished
  (mutation-confirmed, no cross-match).
- **ADR-0033 recorded** — "Cursor auto-run trust is granted at the facade, not per operation"
  (`change: 73`, `relates_to: [29, 20, 27]`, Accepted). It rides this change's terminal-publish onto
  the integration branch at merge/`done`; it is on `origin/docket` now.
- **Faithfulness cross-checked (final review, opus).** The guide's six troubleshooting entries map 1:1
  onto the appendix's six reproduced failure modes; both cut candidates (protected `.git` writes;
  "network blocked while allowlisted" as stated) correctly do **not** appear; network denial is framed
  only as a property of sandboxed/demoted commands. No over-claim on Run-Mode or network.
- **Three Minor guard findings — one fixed, two left as noted.**
  - *Fixed:* the canonical assertion originally checked only the inner guard token
    `${DOCKET_SCRIPTS_DIR:?run docket/install.sh}`, leaving the surrounding `\"…\"/docket.sh`
    decoration unguarded (a coordinated mangle across the JSON file + the guide fence passed green).
    Tightened to also assert the full decorated spelling, built from the derived token; mutation-confirmed
    (commit `bb4a792`).
  - *Left as noted (pathological, not exercised):* assertion #4 compares raw
    entry-count == stamp-count, not per-entry pairing — a "2 stamps on one entry, 0 on another" edit
    would sum-equal. Current guide is a clean 6:6 one-to-one.
  - *Left as noted (proven no-op):* assertion #5's `$EXPOSED` appends `.sh` to the `preflight`/`env`/
    `bootstrap` verbs (no such scripts); the three phantom names have no intersection with the
    not-exposed raw set, so the subtraction is unaffected.
- **Whole suite green on the feature branch:** 39 existing `tests/test_*.sh` files pass plus the new
  `tests/test_cursor_permissions_docs.sh` (14 assertions). No regression — this change only adds files
  under `docs/cursor/`, one test file, and a README subsection.

## Follow-ups

- **Guard tightenings** (optional, if this test file is next touched): fold assertion #4 into per-entry
  pairing (stamp inside each entry's own block) and drop the phantom verb names from `$EXPOSED`. Both
  are latent-only today; not worth a standalone change.
- **Equivalent permission fragments for other harnesses** (Claude Code, Codex) are explicitly out of
  scope here (this change documents Cursor). A future change could mirror this guide per harness.
