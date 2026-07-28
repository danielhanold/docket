<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0144 — A board-checks.sh non-zero exit silently voids the entire health pass](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0144-a-board-checks-sh-non-zero-exit-silently-voids-the-entire-he.md)**
<!-- docket:backlink:end -->

# A board-checks.sh non-zero exit silently voids the entire health pass — design

Change 0144.

## Problem

`health_checks()` in `scripts/docket-status.sh` pipes `board-checks.sh` into a
`while IFS=$'\t' read` loop and never reads the producer's exit status. `board-checks.sh`
accumulates findings into `$FINDINGS` and prints once at the end, so every validation failure
(`exit 2`) emits **zero** TSV lines: the loop body never runs, `health_checks` returns 0, and the
report is byte-indistinguishable from a clean tree.

Change 0117 hit a live instance (a missing `adrs_dir` made `board-checks.sh` exit 2 and dropped
every check) and fixed that trigger — the `[ -d "$mw/$ADRS_DIR" ]` guard now in `health_checks`. The
swallowing itself remains, and 0117's regression test cannot see it: its mock `board-checks.sh`
exits 0 regardless of arguments, so the "still emits check lines" assert passes against both the
fixed and the unfixed code (`green-suite-untested-branch`).

## Decision

**Keep the warn-only posture; make the failure loud.** Capture the checker's output and exit
status, then consume the captured text, and emit one new report line when the checker failed.

```
local out rc=0 check_id change_id message
out="$("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/board-checks.sh \
        --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
        --integration-branch "origin/$INTEGRATION_BRANCH" \
        --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}" \
        ${adr_args[@]+"${adr_args[@]}"} 2>&2)" || rc=$?
while IFS=$'\t' read -r check_id change_id message; do
  [ -n "$check_id" ] || continue
  echo "check $check_id $change_id $message"
done <<<"$out"
[ "$rc" -eq 0 ] || printf 'health checks failed %s\n' "$rc"
return 0
```

Verified against the running code:

- `scripts/docket-status.sh`'s prologue is `set -uo pipefail` — **no `-e`** — so `|| rc=$?` is safe and
  does not abort the pass.
- An empty `$out` yields one blank line from `<<<`, already swallowed by the existing
  `[ -n "$check_id" ] || continue` guard.
- The loop no longer runs in a pipeline subshell, so `check_id`/`change_id`/`message` must be
  declared `local` (they were previously subshell-scoped).
- Capture-then-consume is this file's own idiom: `reclaim_pass` captures then greps a here-string,
  citing change 0067's no-`pipefail`-SIGPIPE rule.

**The diagnostic is a new non-`check` report line, `health checks failed <exit>`.** The
`check <check-id>` vocabulary is closed — `BOARD_CHECK_IDS` in `scripts/lib/docket-frontmatter.sh`
(13 ids post-0117) is pinned in both directions — count and set — by `tests/test_board_checks.sh`,
and "the checker itself broke" is not a finding about a change.

The token deliberately sits **outside the `board ` family**. `skills/docket-status/SKILL.md` is the
one place callers are told to key on report lines, and it teaches `board *-failed` / "no `board …`
line" as meaning **the board step** failed. Naming a health-pass diagnostic `board-checks failed`
would land inside that taught contract and force a disclaimer in a file the misled reader is not
reading. `health checks failed <exit>` names its own pass, matching the existing
`learnings index failed` shape (pass name + `failed`).

Safe against every existing line-consuming matcher, verified:

- `reclaim_pass`'s `RECLAIMABLE_LINE_RE` anchors on `^check stale-in-progress …`.
- `board_classify` matches `case "$line" in "board "*)`.
- SKILL.md's `board *-failed` gloss.

None matches a line beginning `health checks `.

**The exit code alone is the payload; the cause stays on stderr.** `board-checks.sh` already prints
its reason there (`board-checks: adrs dir not found: …`) and `health_checks` passes stderr through
untouched. This matches `board inline failed`, which likewise carries no reason on stdout: the
report line is a *signal that a step failed*, the diagnostic is the stderr text.

**Not retryable; `board_classify` untouched.** The reason is **capture scope**, not flag
exclusivity: `board_pass_must_land` classifies only its own `board_out="$(board_pass)"`, captured
before `health_checks` ever runs. (A bare `--must-land` *does* run the full path — the arg parser
excludes `--must-land` only against `--digest-only` — so the scoping argument must rest on call
ordering, which is why it gets its own test below.)

## Scope

### `scripts/docket-status.sh`

The `health_checks()` rewrite above, plus the stale comment on the function itself
("a clean tree (or a board-checks failure) prints nothing extra").

### `scripts/docket-status.md`

Locate each site by its text, not by line number — the anchors drift:

1. **§7 "Health checks"** — the sentence "a clean tree, or a `board-checks.sh` failure, produces no
   extra output and never aborts the pass" is now false and must be corrected. This is the real
   site.
2. **The `## Output contract` table** — add the `health checks failed <exit>` row, stating it is a
   **health-pass** line.
3. The two *warn-only* restatements (the *Failure postures* health-check bullet and its echo in
   *Invariants*) carry **no** stale claim — warn-only is still exactly right and both stay
   byte-untouched. Editing only *Failure postures*, the obvious target, would miss the real site
   entirely.

### `tests/test_docket_status.sh`

**Must fail against the unfixed `health_checks`** (the mutation mandate applies to these three):

- mock exits 2, no output → exactly one `health checks failed 2`; the run still reaches `pass ok`
  and exits 0.
- mock exits 1, no output → `health checks failed 1`.
- mock prints one well-formed TSV finding **then** exits 2 → the `check` line **and**
  `health checks failed 2`. Pins "additive, not replacement"; a naive
  `if rc; then diagnostic; else findings; fi` fails it. **Not hypothetical**: `board-checks.sh`'s
  `--strict` path already prints `$FINDINGS` and *then* exits 1. `health_checks` never passes
  `--strict` today, so the shape is unreachable from this caller — but it is one flag away, not a
  speculative future.

**Non-regression pins, expected to pass both ways — do NOT apply the mutation mandate:**

- mock exits 0 with findings → unchanged output, no `health checks failed` line (the
  no-false-positive direction; `correspondence-guard-runs-one-way`).
- mock emits `stale-in-progress … [reclaimable]` and exits 2 → the reclaim remedy line still
  prints.

**Two structural guards:**

- The capture-scope argument rests on call ordering inside `main()` that a future edit could
  silently break: full path with `--must-land`, board-checks mock exits 2, board mock clean → the
  run still exits 0 and reaches `pass ok`.
- A doc guard that the **stale sentence is gone**, not merely that the new line is documented. The
  existing `assert "status contract documents …"` family pins presence only.

## Out of scope

- `board-checks.sh`'s own exit-2-on-bad-argument rule, which is correct for a hand-run caller.
- Change 0117's specific trigger, already fixed and merged (PR #129).
- The check-id vocabulary and its four pinned surfaces (0111's guard); no id is added.
- `skills/docket-status/SKILL.md`. Renaming the token out of the `board ` family removes the reason
  to touch it — the file's `board *-failed` gloss stays true. Change **0145** owns that file and is
  `in-progress`; leaving it alone keeps the two changes collision-free.

## No ADR

A failure-posture refinement inside one function, expressed through the existing output contract.
No new decision a future reader would need the "why" for beyond this spec.

## Assumptions

1. **Warn-only, not fatal.** *Chosen:* a checker failure reports and the pass continues.
   *Rejected:* exit non-zero — a checker bug must not abort a pass whose real work (board, sweep,
   digest) succeeded. `scripts/docket-status.md`'s *Failure postures* already ships "health checks
   warn-only" as an invariant, so escalating would be a shipped-contract break.

2. **New line, not a `check` line.** *Chosen:* `health checks failed <exit>`. *Rejected:*
   a `check checker-failed …` finding — that would add a 14th id to the closed `BOARD_CHECK_IDS`
   vocabulary pinned across four surfaces, for something that is not a finding about a change.
   *Rejected:* stderr only — invisible to every caller that keys on report lines, which is the
   whole defect.

3. **Capture-then-loop, not `PIPESTATUS`.** *Chosen:* command substitution + here-string.
   *Rejected:* reading `${PIPESTATUS[0]}` after the pipeline — correct but keeps the loop in a
   subshell and diverges from `reclaim_pass`'s established idiom in the same file. *Rejected:*
   `set -e` + trap — far too broad a blast radius for one function.

4. **Sequencing against change 0117 — resolved.** The prior abstain parked on this. PR #129 is
   **merged** and 0117 is archived, so the collision is gone: the `adr_args` block and the
   `[ -d "$mw/$ADRS_DIR" ]` guard are on `main` and are the code this design was re-verified
   against. `depends_on` is therefore cleared on the change file.

   **0145 is the live overlap, and it is resolved by the rename, not by scope avoidance.** 0145 is
   now `in-progress` on `feat/docket-status-skill-md-restates-a-stale-check-count-and-list`, and its
   reconcile log explicitly predicts this collision: a `board-checks failed <exit>` diagnostic would
   have forced 0144 to edit `## Read the report` in the same `skills/docket-status/SKILL.md` 0145 is
   rewriting. Naming the line `health checks failed` leaves SKILL.md's `board *-failed` gloss
   correct, so 0144 has no reason to open the file at all. `related: [145]` stays as subject
   overlap; there is no file collision left.

5. **A new non-`check` report line is an acceptable addition to the output contract.** *Chosen:*
   yes. The contract is a documented table that already grew for `board inline failed` and
   `learnings index failed`; adding a row in the same family and same commit as the doc update is
   the established pattern, and every existing consumer's matcher was verified above not to catch
   it.

6. **The emit-then-fail test is worth writing despite being unreachable from this caller.**
   *Chosen:* keep it. It is the one case that distinguishes "additive" from "replacement", it costs
   three lines, and the implementation it excludes is the one a reasonable person would otherwise
   write. *Rejected:* drop as speculative — `board-checks.sh --strict` already ships the
   print-then-exit-1 shape, so the only thing standing between it and this caller is one flag.

7. **The report line carries the exit code, not the reason.** *Chosen:* exit code only; the cause
   stays on stderr, where `board-checks.sh` already writes it and `health_checks` already passes it
   through. *Rejected:* interpolating the checker's stderr into the stdout line — it would make an
   unbounded, multi-line, model-unfriendly report line out of a fixed-shape contract, and it
   diverges from `board inline failed`, which sets the precedent that a failure line signals *which
   step*, not *why*.
