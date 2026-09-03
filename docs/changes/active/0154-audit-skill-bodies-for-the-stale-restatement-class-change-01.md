---
id: 154
slug: audit-skill-bodies-for-the-stale-restatement-class-change-01
title: Audit skill bodies for the stale-restatement class change 0145 closed in one file
status: proposed
priority: medium
type: docs
created: 2026-07-28
updated: 2026-08-07
depends_on: []
related: [111, 144, 157, 159]
discovered_from: [145]
adrs: []
spec: docs/superpowers/specs/2026-08-07-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0145 removed a stale check-id restatement from `skills/docket-status/SKILL.md` — a count
word, a five-item check-id list, and a hand-run `docket.sh board-checks` invocation block, all of
which had drifted from the real thirteen-id vocabulary while change 0111's correspondence guard
stayed green (the guard pins four surfaces, and SKILL.md was not one of them).

The removal was scoped deliberately to that one section in that one file, and 0145's `## Out of
scope` named the rest: **no other skill file was audited for the same restatement class.** The
failure is structural rather than a one-off — any skill body that copies a script's flag list,
enumerates a closed vocabulary, or restates a count is an unpinned surface that drifts silently.
0145 also turned up two collateral instances outside the target file: an assert in
`tests/test_results_artifact.sh` pinned prose that lived in the deleted block, and one in
`tests/test_docket_metadata_branch.sh` depended on a phrase that only appeared inside it.

**Absorbed #0159 (2026-08-07 triage).** One named instance of exactly this class: 
`skills/docket-status/SKILL.md:35`'s normal-outcomes enumeration omits the `health checks failed
<exit>` warn-only line that 0144 added — a reader can mistake it for a hard error. The line is
real and documented on the script side (`docket-status.sh:949`, `docket-status.md:219,:448`,
pinned by `test_docket_status.sh:3981-4030`). Fix it under this sweep's rule: either point at the
owning contract or add the one line with the wording `docket-status.md:448` already fixes.

Known live hits to seed the sweep (verified 2026-08-07): `skills/docket-status/SKILL.md:90`
(~400-word restatement of the sweep's failure-reason vocabulary, near-verbatim from
`docket-status.md:180-196`); `skills/docket-convention/SKILL.md:191` (mark-publish-deferred
marker semantics + check id); `skills/docket-convention/SKILL.md:54,62` (coordination-key fence
list and `board_surfaces` token semantics, owned by `docket-config.md`).

## What changes

Groomed 2026-08-07 (auto-groom; two critic passes — all eleven assumptions sound). The linked spec
settles scope, the per-hit decision rule, the named dispositions, and the guard question; this body
stays at proposal altitude.

One docs-type PR sweeping every markdown file under `skills/` (the 12 SKILL.md files plus skill
references; `scripts/*.md` contracts are the *owners* and are out of the sweep — contract-to-contract
duplication is report-only) under one decision rule, disposition preference strictly
**delete-and-point > compress-to-owned-judgment > pin** (expected pins: zero).

- **Named hits, committed:** `skills/docket-status/SKILL.md:35` (outcome + error-cause
  enumerations → delete-and-point, which absorbs killed #0159 by construction — no list, nothing
  to omit) and `:90` (the ~400-word sweep-posture restatement → compress to the judgment kernel
  the skill owns + pointer); `skills/docket-convention/SKILL.md:55` (fence-key list → delete, but
  the surviving sentence keeps `terminal_publish` named beside the fence phrase — two test pins
  constrain it) and `:63` (`board_surfaces` → keep the definitional sentence, repoint/relocate the
  restated resolver behaviors to their actual owners; one behavior is stated nowhere else and must
  be relocated, never deleted).
- **Exempt, verified:** single-item cross-references (e.g. `publish-deferred`,
  `stale-finalize-blocked` mentions) and the convention's Agent-layer wrapper counts, which are an
  **already-pinned surface** (`test_finalize_gate.sh:152-156`, change 0170) — left alone.
- **Generalized guard: NO** — a repo-wide check-id lint needs its own drifting sanction list;
  removal plus the existing guards (0111's four surfaces, 0145's section guard, 0170's count pins)
  covers what survives.
- **Collateral-test protocol, mandatory per edit:** grep `tests/` for a block's distinctive
  phrases before deleting it; retarget pins to surviving text, never to vacuity (0145 precedent).
- The build completes the inventory mechanically (token sets sourced from the owning lib/contracts,
  never hand-copied) plus one full manual read; results recorded in the plan/results artifacts.

## Out of scope

- Changing any vocabulary, exit code, or script behavior.
- Re-litigating the four surfaces change 0111 already pins, 0145's guarded `### Health checks`
  section, or 0170's count pins.
- `scripts/*.md` contract-to-contract duplication and the convention's `.docket.yml` schema
  snippet (verify + report only).

## Open questions

- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: the named hits (scripts/*.md contracts, docket-status.md line cites, 0111/0145/0170 Bash guards) are deleted. Re-derive the hit list against the Go capability catalog (`docket capabilities`) and `internal/config`; docket-status still cites `board-refresh.sh` / `github-mirror.sh` / `render-board.sh`. Guard home is `internal/repoguard/prose_contracts_test.go`. Sweep the sunset `github` mirror / `issue:` prose while there.

