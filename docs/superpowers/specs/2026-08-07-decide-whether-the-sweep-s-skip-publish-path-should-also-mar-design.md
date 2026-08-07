<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0118 — Decide whether the sweep's skip-publish path should also mark an unpublished terminal record](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0118-decide-whether-the-sweep-s-skip-publish-path-should-also-mar.md)**
<!-- docket:backlink:end -->

# Mark the sweep's skip-publish path too — design (change 0118)

## Decision

Yes, the sweep's `render-change-links skipped-publish` path marks. When the renderer itself
exits non-zero during the merge sweep's close-out (`docket-status.sh` `sweep_execute_one`,
the `sweep-failed <id> render-change-links skipped-publish` branch), and the publish was
**expected** (`terminal_publish: true` AND docket-mode), the sweep appends the same
`## Publish deferred` marker it already writes on the `terminal-publish` failure branch —
best-effort, muted, committed and pushed on `metadata_branch` — before emitting its
unchanged report line. Reason stays `blocked`; the cause lives in `--detail`.

The stub's "nothing published means nothing was deferred yet" rationale does not survive the
code: once archived, the change leaves `active/` and **no later sweep resumes it** (the sweep
scans `active/*.md` only, and self-skips already-archived ids). The gap is permanent until a
human acts — byte-for-byte the #0043 state ADR-0051 exists to make visible. Whether the
publish was *deferred*, *blocked*, or *never reached* is a distinction about cause, not
visibility, and the marker's detail line carries cause fine.

The noise counter-argument also collapses — but on permanence, not determinism. The
renderer CAN fail transiently: `render-change-links.sh` unconditionally resolves config via
`docket-config.sh --export` (`render-change-links.sh:45-50`), which runs `git fetch origin`
and dies on fetch failure (`docket-config.sh:260-262`), so a network blip fires exactly this
branch (the renderer's "no network" header comment is stale; the code is authoritative).
Flapping is nonetheless structurally impossible: only `terminal-publish.sh`'s success path
strips the marker, and no sweep ever resumes an archived change — so a marker written here
is stable until a human acts, whatever the cause was. Permanence alone carries the decision.

## What changes

### 1. `scripts/docket-status.sh` — the only code change

In `sweep_execute_one`, inside the existing `skipped-publish` branch (renderer exited
non-zero, currently bare `echo` + `return 0`), insert before the `echo`:

- **Gate:** `[ "${TERMINAL_PUBLISH:-false}" = true ] && [ "${DOCKET_MODE:-}" = docket ]`.
  This gate is load-bearing and is the one structural difference from the 0083 branch: the
  `terminal-publish` failure branch is unreachable under suppression (both suppressions are
  exit-0 no-ops), but a renderer failure fires regardless of the knob. Under
  `terminal_publish: false` or main-mode a skipped publish is *success* — never mark
  (ADR-0051 / `mark-publish-deferred.md` invariant). The residual there remains what the doc
  already says: a stale `## Artifacts` block, fixed by a manual re-render.
- **Mark:** `mark-publish-deferred.sh --mode add --change-file "$archived" --reason blocked
  --detail "sweep: artifacts re-render failed — publish never attempted; re-render before
  publishing" --integration-branch "$INTEGRATION_BRANCH" --id "$id"`, then
  `git add/commit/push` the archived file on `metadata_branch` — every step muted and
  `|| true`-guarded, outcomes never read, exactly the 0083 posture (an uncommitted marker
  would dirty the shared worktree and fail every later pass's `pull --rebase`, which is
  worse than an unmarked gap).
- **Report contract unchanged:** the branch still emits exactly
  `sweep-failed <id> render-change-links skipped-publish` and still `return 0`
  (publish + cleanup stay abandoned — the skip-publish guard itself is out of scope and
  correct). The mark block must be invisible to callers keying on report lines.
- The `--detail` string must not contain the literal renderer/publisher script names —
  `tests/test_closeout.sh`'s call-site scanner greps joined logical lines for
  `terminal-publish.sh` (this invocation carries `--id` without `--enabled`).

### 2. Docs stating the real reason

- `scripts/docket-status.md` (§6, the current lines 186–196): replace the "nothing was
  deferred *yet*" rationale with: the `render-change-links skipped-publish` leg now marks
  under the same expected-publish gate, best-effort like the `terminal-publish` mark; under
  suppression it stays unmarked because a suppressed publish is success, and the residual
  there is the (already-documented) stale-Artifacts-block-needs-manual-re-render note.
- `skills/docket-status/SKILL.md` (~line 90, the sweep-posture paragraph): same correction —
  the `skipped-publish` case is "no longer invisible" alongside the `terminal-publish` case;
  keep the reason-vocabulary cross-check guidance intact.
- `skills/docket-convention/references/terminal-close-out.md`: extend step 3's
  "expected but does NOT complete — mark it" rule to state it applies to **any** path that
  abandons close-out before the publish step while the publish was expected. Scope the two
  legs precisely, because the drivers diverge on the commit/push leg: a failed step-2
  **re-render** abandons publish for all drivers and is marked by all drivers; a failed
  step-2 **commit/push** skips publish (and so is marked) only in the skill-driven drivers —
  the sweep deliberately continues to publish on that leg (change 0075 §5, documented in
  `docket-status.md` §6a), so the rule must not read as obliging the sweep to mark there.
  In the same edit, fix the pre-existing contradiction this exposes: the skip-publish guard
  at `terminal-close-out.md:158-159` says "(all callers) … a failed step-2 commit/push skips
  step 3", which the sweep's continue-to-publish posture (0075 §5) contradicts — carve the
  sweep out of "(all callers)" there, pointing at `docket-status.md` §6a for its deviation.
  Doc-level rule only; the sweep is the only driver getting code in this change (the other
  three drivers are skill-driven abort-and-report flows with a human or report in the loop,
  and their mark duty is discharged by this rule text).

### 3. Explicitly not done

- **No third `--reason` value, no new heading.** `mark-publish-deferred.sh`, its contract,
  `board-checks.sh`, and the `publish-deferred` check are untouched. One heading keeps one
  reader, one removal path (`terminal-publish.sh`'s success-path strip), one health finding.
- **No N-consecutive-failures counter** — rejected; needs sweep state that does not exist,
  to defend against flapping that permanence already makes impossible (nothing retries an
  archived change, so even a transiently-caused marker is stable, not noisy).
- **No change to the `commit-failed`/`push-failed` legs** (step 6a): there the close-out
  continues and the record publishes; the residual stale block is cosmetic, not a missing
  record — nothing to mark.

## Open questions from the stub — answered

1. **Failure frequency:** unknown (no telemetry), but irrelevant — the deciding property is
   *permanence* (no sweep resumes an archived change), not frequency. The stub's premise
   that "the next sweep retries the re-render, so the window is one pass" is factually wrong
   per `docket-status.md:181-184` and the `active/`-only scan in the code.
2. **Same heading?** Yes. The marker's value is its single reader; cause belongs in the
   dated `**blocked** — <detail>` line, which already distinguishes deferral from wall.
   The marker prose ("terminal-publish to `<branch>` not completed") is accurate for
   never-attempted too.
3. **Other archived-but-unpublished paths, audited:** (a) `skipped-publish` — closed by this
   change; (b) `terminal-publish` failure — marked since 0083; (c) step-6a
   commit/push failure — publishes anyway, not a gap; (d) hard crash between archive and
   publish — writes nothing by definition; stays the accepted residual ADR-0051 records
   ("the write side is a rule for drivers, not an enforced code path"); (e) the three
   skill-driven drivers' skip-publish guard — covered by the terminal-close-out.md rule
   extension (doc-level, human/report in the loop).

## Assumptions

1. **Mark, rather than doc-fix-only or decline.** Chosen on **permanence**: no sweep
   resumes an archived change, so the gap is forever until a human acts, and the marker
   cannot flap (nothing auto-removes it on this leg) even though the failure itself can be
   transient (the renderer's config resolution does a `git fetch`). The doc-only and
   decline options were honest only under the false "window is one pass" premise.
   Rejected: doc-only fix; decline-as-fault-to-fix (the marker is cheap, the check already
   exists, and 0083 set the precedent on the sibling branch).
2. **Reuse `--reason blocked` + a distinct `--detail`, not a third reason value.** Minimal
   surface: zero changes to `mark-publish-deferred.sh`, its `.md`, `board-checks.sh`, or
   the convention's marker description; the dated detail line is the cause channel the 0083
   design already provides. Rejected: a `not-attempted` reason (touches script validation,
   contract, tests, and the convention doc for no additional reader); a second heading
   (needs a second check and second removal path).
3. **Gate the mark on `TERMINAL_PUBLISH=true` AND `DOCKET_MODE=docket`.** Required on this
   branch (unlike 0083's, which suppression cannot reach) to honor never-mark-under-
   suppression. Uses the same variables the pass already trusts elsewhere
   (`health_checks`' adr-check gating). Rejected: marking unconditionally (violates
   ADR-0051's success-not-deferral rule); re-deriving config in-branch (duplicates
   resolution the script already did).
4. **Strictly best-effort, report-contract-frozen.** The mark block is muted, `|| true`-
   guarded, includes its own commit+push, and neither adds, removes, nor reorders any
   report line; `skipped-publish` remains the branch's only emission and `return 0` stands.
   Rejected: keying control flow on the mark's outcome (0083 settled the posture, and a
   failed mark must degrade to today's behavior exactly).
5. **Detail text tells the human to re-render before publishing.** The marker's fixed
   `**Re-arm:**` line says "complete the publish" — correct for the 0083 branch but
   incomplete here, since publishing without a re-render would publish the stale block the
   guard exists to stop. The detail line carries that one extra instruction. Rejected:
   teaching `mark-publish-deferred.sh` per-cause re-arm text (surface growth for one line).
6. **Doc scope: three files** (`scripts/docket-status.md`, `skills/docket-status/SKILL.md`,
   `terminal-close-out.md` rule extension); no code in the other three drivers. Rejected:
   wiring a mark call into `docket-finalize-change`/kill flows now — they are interactive
   abort-and-report paths where the failure is surfaced to a human immediately, and the
   convention rule now names their duty.
7. **Couplings:** `related: [154, 254]` — file collisions, not dependencies. 0154's audit
   list names `docket-status.md:180-196` and `skills/docket-status/SKILL.md:90` (both
   paragraphs this change rewrites — the latter is 0154's delete-and-point target, the
   tightest collision: whichever lands second must re-apply its edit against the survivor)
   and `docket-convention/SKILL.md:191` (which this change does NOT touch — its convention
   edit is `references/terminal-close-out.md`); 0254's BSD `mv` sweep touches
   `docket-status.sh` and `mark-publish-deferred.sh`. Whichever lands second re-reads.
   No `depends_on:` — both directions build cleanly in either order.

## Tests

Extend the existing sweep marker coverage in `tests/test_docket_status.sh` (the 0083
`terminal-publish` mark tests), with three frictions the existing fixture imposes:

- The 0083 fixture runs `DOCKET_MODE=main` with `TERMINAL_PUBLISH` unset
  (`test_docket_status.sh:~1444-1448`); the new marking case needs docket-mode, under which
  `docket_metadata_worktree` resolves `.docket` (`lib/docket-root.sh:51-58`) — the test must
  set `METADATA_WORKTREE=.` (or build a docket-mode fixture) rather than reuse the run as-is.
- The mock `mark-publish-deferred.sh` appends a fixed heading and logs its argv
  (`:~1413-1426`): assert the `blocked` reason and the re-render `--detail` **on the logged
  argv**, as the 0083 asserts do — not on rendered file content.
- The existing assertion "a change that never reached the publish step is never marked"
  (`:~1524-1525`) encodes exactly the semantics this change reverses; it only passes today
  because that run is suppression-mode. Rename/re-scope it to say what it now proves
  (never marked **under suppression**), and do not add its change id to a
  `TERMINAL_PUBLISH=true` run.

Cases:

- Renderer forced to exit non-zero, `TERMINAL_PUBLISH=true` + docket-mode: mark call logged
  with `blocked` + re-render detail; report stream byte-identical to today's
  (`sweep-failed <id> render-change-links skipped-publish`, no `swept`/`harvest`); marker
  commit on `metadata_branch`.
- Same failure under `TERMINAL_PUBLISH=false`, and under main-mode: **no** mark call.
- Mark script itself failing: report stream unchanged (best-effort invisibility).
