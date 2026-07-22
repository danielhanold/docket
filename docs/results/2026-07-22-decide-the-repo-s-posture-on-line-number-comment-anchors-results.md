# Decide the repo's posture on line-number comment anchors — results

Change: #114 · Branch: feat/decide-the-repo-s-posture-on-line-number-comment-anchors · PR: <url> · Plan: docs/superpowers/plans/2026-07-22-line-number-comment-anchors.md · ADRs: 54

## Verify (human)

Nothing interactive is required — the whole suite is green (54/54 `tests/test_*.sh`) and the guard
was mutation-verified in seven directions (matrix below). **One judgment call needs your ratification
at the merge gate:**

- [ ] **The guard walks one surface beyond the spec's literal scope list.** The spec enumerates the
      in-scope surfaces as `scripts/`, `skills/`, `tests/`, `agents/`, `cursor-rules/`, and root
      `*.md` / `*.yml` — **root `*.sh` appears nowhere**, neither ruled in nor ruled out, even
      though `install.sh`, `link-skills.sh`, `migrate-to-docket.sh`, and `sync-agents.sh` are
      maintained source by any reading. The whole-branch review demonstrated the consequence: a
      one-line comment added to `migrate-to-docket.sh` reintroduced exactly the defect this change
      exists to eliminate, and the guard stayed **green**. The walk was extended to cover root
      shell scripts. This was free — all four are already anchor-clean — but it *is* a deliberate
      step past the spec's enumeration, recorded as `DEVIATION FROM SPEC, RECORDED:` in the guard's
      own header rather than made silently. Confirm you accept the wider walk; the alternative is
      shipping a demonstrated hole to satisfy a list that never considered the surface.

Two things worth your eye but needing no decision:

- **The guard is partial on purpose, and the unguarded half is the worse half.** It enforces only
  the explicit-file form. The bare colon-number and prose forms were converted by hand and are not
  guarded, because they measured false-positive rates with no exception path under the no-allowlist
  rule. The honest cost — the unguarded self-file prose form carries roughly 50% rot density versus
  about 3/24 elsewhere, so the guard catches about half the *demonstrated rot*, not half by count —
  is now stated in the guard header, in `AGENTS.md`, and in ADR-0054. It was previously only in the
  spec.
- **The guard walks version-control-tracked files.** A brand-new, unstaged file carrying an anchor
  is invisible until `git add`. Documented in the header. Uncommitted edits to *tracked* files are
  seen; the gap is strictly new-and-unstaged files, which cannot reach a merge anyway.

## Findings

**Every anchor this change wrote had to be verified against the code it described, and three were
wrong before they were caught.** That is the headline: the change's own generated content kept
reproducing the defect class it exists to eliminate.

**The plan's replacement text was the defect source, not implementer error.** Symbol anchors were
derived semi-mechanically, and the helper reported *the function containing the cited line* rather
than *the function containing the described behavior*. Two shipped and were caught at Task 1 review:
`board-checks.sh` named `sweep_execute_one` as the reader of its TSV findings when the real reader
is `health_checks` (the old numeric anchor had drifted 25 lines and pointed into a different
function entirely), and `scripts/docket-status.md` quoted a header clause that was **not greppable**
— it omitted the `scripts/` prefix the real line carries. A third was caught at Task 2 when the
implementer, explicitly instructed to verify rather than transcribe, found the brief conflating two
different sentences in the finalize SKILL. A fourth surfaced at the whole-branch review: two files
quoted `` `git rev-parse --show-toplevel` `` where the code reads `"$GIT" rev-parse --show-toplevel`,
so the "verbatim" quote matched nothing. **A quoted clause that cannot be found is the same defect
as a stale line number wearing better clothes** — greppability has to be checked, not assumed.

**The whole-branch review found 1 Critical + 3 Important in a guard that had already passed a task
review with a green suite and a four-cell mutation matrix.** All were ways for a green guard to
prove less than it claimed:

- **Critical — a whole surface was outside the walk.** Root `*.sh` was never in the pathspec (see
  *Verify* above). The guard reported "no line-number cross-reference anchors in maintained source"
  while a planted anchor sat in a 400-line maintained script.
- **Important — the population floor was dead slack and half the surfaces were unpinned.** Probes
  covered `scripts/`, `tests/`, and the two root globs; `skills/`, `agents/`, and `cursor-rules/`
  (35 files) were pinned by nothing, and the numeric floor sat at 40 against an actual 145 — so it
  could never fire in a case the probes did not already catch. Narrowing the pathspec to two
  directories left the guard green with every probe passing. This is the same anti-pattern recorded
  in `tests/test_board_checks.sh`, a file this branch edits, which notes a floor that "sits below
  the true count by construction, so it can never catch an under-derivation."
- **Important — the positive control did not exercise the code path it certified.** It ran the
  pattern in a separate invocation, so the main loop's own scan and its violation-reporting path
  were asserted by nothing. Neutering the in-loop pattern *with a real anchor planted* left the
  guard at exit 0 with all nine `ok` lines — **including the positive-control line**. Closed by
  extracting a shared `scan_file()` that the loop and both controls now route through.
- **Important — ADR-0054 was cited by two shipped files before it existed.** `AGENTS.md` and the
  guard header both referenced it while the ledger topped out at 0053 and the manifest carried
  `adrs: []`. Had any other change minted an ADR first, the change whose purpose is eliminating
  broken cross-references would have shipped two dangling ones. Minted on the metadata branch and
  verified anchor-clean.

**An earlier task review caught the guard violating a rule stated two headings above the section
this change adds to the same file.** The probe used `printf … | grep -qxF` under `set -o pipefail`,
which `AGENTS.md` explicitly forbids (the producer takes SIGPIPE and the 141 becomes an intermittent
failure). It came from the plan text.

**Two environment traps cost real time and are now recorded in the plan and the guard header**, because
both produce *silent* wrong answers:

1. **`git grep -E` does not support `\b` or `\<`.** Both return zero matches with exit 0. A
   `\b`-anchored survey pattern reported "no prose anchors exist" when five did.
2. **`git grep` output is `path:lineno:content`, and the path ends in `.sh:` / `.md:` — the exact
   shape this guard hunts.** Filtering `git grep` output with the anchor pattern matches the tool's
   own prefix on nearly every line. This is why the guard scans per-file with `grep -n`, whose
   `lineno:` prefix cannot collide. A guard written the obvious way would have been silently
   self-defeating.

**The reconcile pass found the population had doubled** since the spec was written five days
earlier — 13 → 26 explicit-file anchors, as changes 0102/0104/0111/0112 landed. Four *more* bare
anchors had gone stale by exactly 2 lines under 0111's header edits. The idiom accretes faster than
it rots, which is the argument for the guard rather than against it.

## Plan deviations

- **Task 4 was rerouted off the feature branch entirely.** The plan directed the ADR-0044 `## Update`
  into the feature worktree. That would violate the convention's invariant that a feature branch
  "adds only the plan + results + code and **never modifies** docket metadata (the change file,
  `BOARD.md`, ADRs)" — ADRs are authored on the metadata branch, and the copies on the integration
  branch are published mirrors. Both ADR actions were routed to the `docket-adr` subagent on
  `docket`; no feature-branch ADR commit was made. The plan was corrected in place so it and the
  branch agree.
- **ADR-0044's `## Update` was not written.** It is a metadata-branch edit to an `Accepted` ADR,
  outside this branch's remit and outside the guard's walk. ADR-0054 records why `docs/adrs/` is
  unwalked (an Accepted ADR cannot be edited to satisfy a guard), which is the substance of what
  the note was to say. Left for a metadata-branch pass if still wanted.
- **Strict TDD was deliberately not followed for the guard.** A red-first commit would have left the
  suite broken across two commits. Instead the finished guard was run against the pre-conversion
  base commit in a throwaway worktree and required to report exactly the 26 real anchors — stronger
  evidence than a red-first run, because it also pins the count.

## Mutation matrix

Seven directions, all re-verified after the final fix wave; the last two are regression tests for
the Critical and Important findings above.

| Mutation | Expected | Observed |
|---|---|---|
| Reintroduce an anchor in `scripts/board-checks.sh` | red | 1 |
| Empty the walk population (bad pathspec) | red | 1 |
| Neuter the `ANCHOR` pattern | red | 1 |
| Remove the structural self-exclusion | red | 1 |
| Plant an anchor in a `skills/` file | red | 1 |
| **Plant an anchor in a root script** (C1 regression) | red | 1 |
| **Neuter the in-loop scan only, anchor planted** (I3 regression) | red | 1 |
| Restored | green | 0 |

Final guard state: **149 files walked, 148 scanned** (self-excluded), 8 population probes, no
allowlist anywhere.

## Follow-ups

None minted. Two observations were considered against the auto-capture materiality bar and
deliberately not filed as changes:

- **Closing the tracked-files gap** (e.g. a pre-commit hook) is speculative and the gap cannot reach
  a merge — an unstaged file is not part of the repo the gate protects.
- **Re-surveying the unguarded forms later** is already the spec's own stated mitigation for A3
  ("detectable by re-running this survey later") rather than distinct work.
