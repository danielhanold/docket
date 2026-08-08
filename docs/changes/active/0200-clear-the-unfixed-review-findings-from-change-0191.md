---
id: 200
slug: clear-the-unfixed-review-findings-from-change-0191
title: Board-checks hardening — sanitize LF escape, capture-shape mutation, minor-finding clearance
status: in-progress
priority: medium
type: fix
created: 2026-08-03
updated: 2026-08-08
depends_on: [224]
related: [213, 215, 216, 217, 222]
discovered_from: [191, 202]
adrs: []
spec: docs/superpowers/specs/2026-08-07-clear-the-unfixed-review-findings-from-change-0191-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/clear-the-unfixed-review-findings-from-change-0191
claimed_at: 2026-08-08T03:29:29Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-clear-the-unfixed-review-findings-from-change-0191-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-clear-the-unfixed-review-findings-from-change-0191-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates five stubs that all touch `scripts/board-checks.sh` and its test suite — this one
(0191's three minor findings) plus killed changes 0215 (sanitize LF escape, a real behavior fix),
0216 (mutation G guard), 0217 (0202's three minor findings), and 0213 (the bash 4.4 `mapfile -d`
floor inconsistency). One change avoids five conflicting PRs against the same two files; the type
is `fix` because 0215's sanitize gap is a genuine correctness defect.

**(a) 0191's unfixed minor findings (original 0200).** Change 0191's whole-branch review returned
3 minor findings that were consciously left for merge-time judgment and never fixed; PR #151
merged with all three outstanding. Cosmetic or documentation-level, but each is a real small
defect in shipped code and would otherwise be lost once the results artifact stops being read.

**(b) sanitize misses a raw LF under `-z` (absorbed from 0215, `important` at 0202's review).**
Change 0202 rewrote `branch_only_artifact` to read `git ls-tree -r -z` instead of `--name-only`,
which fixed a false positive on C-quoted paths but silently invalidated a premise `sanitize`
still relies on: it escapes only TAB and CR, justified by a comment asserting every emitted value
arrives via `field`/`fm_field` (which truncate at the first newline). `$ar_hit` is a **git
path**, not a frontmatter field — under `-z` a raw LF reaches `emit`, splitting one finding
across two TSV records and breaking the `sort` determinism downstream.

**(c) The capture-shape constraint is unguarded (absorbed from 0216, `important` at 0202's
review).** A refactor to `boa_list="$(… git ls-tree -r -z …)"` + `done <<<"$boa_list"` keeps
`-z`, keeps `read -r -d ''`, passes `bash -n`, and makes `branch_only_artifact` return 1 for
**every** input — command substitution strips NUL bytes, so `read -d ''` hits EOF and the loop
body never runs. Leg A would go permanently, silently false-negative with a green suite. The
"do not simplify this back" comment has nothing enforcing it — decoration, by this repo's own
guard rule.

**(d) 0202's three minor findings (absorbed from 0217).** Same class 0202 exists to close —
accurate-looking prose sitting beside code that no longer matches it.

**Re-scoped 2026-08-07 (triage).** Three items verified already-resolved or superseded and
dropped: the comment-strip asymmetry (change 0235 moved `blocked_by` to `fm_field_verbatim` and
documented the accessor choice in `board-checks.md:226-235` — the asymmetry as stated no longer
exists); the count-pin provenance comment (now reads "15 since change 0113…" against a `= 15`
assert); and the whole `mapfile -d` leg (e) from 0213 — Daniel ruled 2026-08-07 to raise the
bash floor to 4.4 (change 0222), which makes `mapfile -d` legal and the drift moot. Also: the
mutation letter G is now **taken** (`tests/test_board_checks.sh:2511`, the aborted-run
idle-floor arm) — the new capture-shape mutation below must take the next free letter.

## What changes

0191 findings (a):

1. `scripts/board-checks.sh` — hoist `scalar_form_check(){...}` out of the per-file walk loop (it is
   redefined per file) to sit alongside `renders_row` with the file's other top-level helpers.

Sanitize LF (b):

4. `scripts/board-checks.sh` — add `v="${v//$'\n'/\\n}"` to `sanitize`, and update the comment that
   currently justifies the TAB/CR-only scope so it no longer asserts a premise the `-z` read broke.
5. `tests/test_board_checks.sh` — a fixture whose branch carries a path with an embedded newline,
   asserting the finding stays one TSV record.

Capture-shape mutation (c):

6. `tests/test_board_checks.sh` — add a mutation arm (next free letter — G is taken by the
   aborted-run idle-floor arm) that rewrites the consumption to the capture shape while keeping
   `-z`, and asserts fixture 230 goes GREEN (i.e. the check stops firing), so the constraint the
   comment states is enforced by execution.

0202 minor findings (d):

7. `scripts/board-checks.sh` — `[ -n "$boa_p" ] || continue` in `branch_only_artifact` is
   unreachable under `-z`; remove it, or state why it is kept.
8. `tests/test_board_checks.sh` — the mutation-baseline comment says the baseline "fires exactly
   the three expected findings"; reword to drop the number rather than re-pin it (re-pinning buys
   a maintenance burden with no guard behind it).
9. `docs/superpowers/plans/2026-08-05-clear-the-unfixed-review-findings-from-change-0113.md` —
   Task 5 Step 2's verification pattern (`grep -nE '4013|4050|147 for 143'`) matches the comment
   line that *explains* 4050 is stale. Recommended resolution: rule merged plan files **frozen
   build records** (never edited), record that decision where a future build will read it, and
   leave the plan file untouched.

## Out of scope

- Any behavior change to the `scalar-form` check itself, and fixing change 0121's flagged title.
- Any further change to the `-z` read itself (0202 settled that shape); `branch_only_artifact`'s
  current shape is correct, and mutation F's existing arm stays as-is.
- The bash floor — settled the other way: change 0222 raises it to 4.4 (Daniel, 2026-08-07);
  the former item 10 (`mapfile -d` rewrite) is dropped, not deferred.
- Any change to which shell the suite selects at runtime (change 0150's territory).

## Open questions

None — both former questions are resolved in the spec (2026-08-07 auto-groom): (b)'s premise IS
relied on by other `emit` callers passing non-frontmatter values (branch names, basenames, git
paths), which is exactly why the LF escape lands inside `sanitize` and covers every caller; and
(c)'s hazard exists at no other `-z` read — `branch_only_artifact` is the only NUL-delimited read
in `scripts/`, so it gets one mutation arm (letter O), not a helper-level guard. The spec also
prices two consequences the stub missed: the frozen-plans paragraph forces a minimal raise of the
docket-convention row in `tests/test_skill_size_budgets.sh` (with the change-0201 in-diff
argument), and the `scalar_form_check` hoist forces a redesign of mutation 4's marker-range
extraction plus its missing `bash -n` landed assert.

## Consolidation note

2026-08-05: absorbed changes 0213, 0215, 0216, and 0217 (all killed pointing here); type flipped
chore → fix accordingly. 0217's merged-plan policy question is resolved by recommendation in
item 9: merged plans are frozen build records.
