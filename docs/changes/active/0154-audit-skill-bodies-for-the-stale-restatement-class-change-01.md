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
related: []
discovered_from: [145]
adrs: []
spec:
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

Sweep the `skills/` tree (and, where cheap, the `scripts/*.md` contracts) for the same restatement
class and decide per hit whether to delete-and-point or to pin.

- Enumerate every skill-body occurrence of a closed vocabulary owned elsewhere: check-ids, exit
  codes, report-line tokens, `docket.sh` flag lists, status/state enumerations, counts of any of
  these.
- For each, apply 0145's rule: prefer **removal plus a pointer to the owning contract** over adding
  another pinned surface, since removal makes drift impossible rather than merely detected.
- Where a restatement genuinely must stay (the skill owns the posture, not just the words), pin it
  the way change 0111 pins its four surfaces.
- Consider whether a single generalized guard — "no skill body names a check-id outside a sanctioned
  section" — is worth building, or whether per-site removal is enough.

## Out of scope

- Changing any vocabulary, exit code, or script behavior.
- Re-litigating the four surfaces change 0111 already pins.
- `skills/docket-status/SKILL.md`'s `### Health checks` section, which change 0145 already closed
  and guarded.
