<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0570 — Let a loop carry on past a halt that concerns only one change](../../changes/archive/2026-10-19-0570-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang.md)**
<!-- docket:backlink:end -->

# Let a loop carry on past a halt that concerns only one change — design

Change #570, groomed 2026-10-19.

## Intent

An overnight `/loop docket-implement-next` drain ends at the first halt. The typical halt is one the
build finds mid-run in a single change, such as a logical error in its spec that needs a person. The
change is already marked "run halted — needs you" on the board, so stopping the whole drain for it
throws away a night of work on the other build-ready changes.

After this change, a loop carries on past a halt recorded on a change it picked from the queue and
moves to the next build-ready change. A setup problem would halt every change the same way, so a limit
on halts in a row stops the loop before it walks the whole queue.

## Goals

- A tracked implement-next loop continues after a run that halted with a recorded `## Run halted`
  marker on a change the loop picked, while the count of halts in a row is under the limit.
- The loop stops when the count reaches `run.max_consecutive_halts` (default 2), and the person is
  asked whether to reset the count.
- The loop still stops on any halt that left no marker (every pre-claim halt) and on every other
  `run-stop`, as it does today.
- **Nothing outside a loop changes.** A named run, a resume, a retry or continuation, an untracked
  run, and finalize print exactly today's verdict lines and see no new refusal. *(Human's
  requirement; confirmed by a code review of this design — see *Isolation guarantees*.)*

## Non-goals

- **No halt reason or classification.** The stub proposed a typed halt reason and sorting every halt
  point into "this change" and "every change". Dropped: the marker plus the halt limit is enough. A
  setup-wide problem is caught by the limit rather than predicted per halt point. The cost is a
  couple of halted changes before the loop stops, which the human accepted.
- **No change to `change.halt`**, the `## Run halted` section, `run.verify`, or the board.
- **Finalize is untouched.** Its `## Finalize blocked` note never removes a change from finalize
  selection, so an unscoped finalize loop that carried on would re-pick the same blocked change.
  Carrying a finalize loop past a block is a separate change, captured as its own stub. Finalize is
  not bracketed by the run tracker (`run.start` accepts only `implement-next`), and every finalize
  `halted` still stops the driver.
- **No change to pre-claim halts.** They write nothing and the run tracker reports them as
  `run-done <key> no-attributable-claim`; the child declares `halted` and the loop stops, as today.
- **No loop identity.** Resetting the count when a loop starts was considered and rejected: docket
  cannot see a loop start (`/loop` re-fires the same prompt every iteration), so the reset would rest
  on the agent judging "this is the first iteration"; a wrong judgment resets every iteration and the
  limit never trips. A restarted loop would also get a fresh limit with nothing fixed.

## Anti-goals

- An agent must never reset the halt count on its own. A loop that could clear its own limit
  defeats the backstop.
- Halts under the limit must never be re-dispatched. Carrying on means starting a *new* run for the
  next change, never retrying the halted one.
- No new text in the everyday path beyond one sentence in the dispatch block. The reset prompt is
  carried by the verdict output at the moment it applies.
- No new refusal anywhere: nothing in this change may make `run.start`, `change.claim`,
  `run.verdict`, finalize, `status`, the board, `repository.check`/`repair`, `resume-halted`, or
  `set-visibility` refuse, stop, or report differently for a run that is not a loop-picked halt.

## Design (settled with the human)

### 1. Which runs count: loop-picked vs named

implement-next already has two ways of choosing a change: the **selection path** (no id, or an id
allowlist of two or more ids — what a loop drives, and also a one-off "implement the next change") and
the **named path** (a single explicit id, or a `run.start --resume`).

- **Recording the mode.** Today the claim cannot tell the two apart: `ChangeClaimRequest`
  (`internal/app/change_claim.go`) carries only `ID`, `Revision` and `RunContext`, and both paths call
  `context.implementation --id` then `change.claim --id`. So the selection path passes a new
  **optional** claim input naming its mode.
  - Absent or unrecognised → recorded as `unknown`. It **never refuses a claim**, so an older skill
    copy (a session not restarted after install) keeps claiming exactly as today.
  - It is not part of `claimDigestPayload` or `claimRequestID`, so it never changes claim identity or
    idempotence.
  - It is stored in the committed claim receipt (`changeClaimReceipt`) as an additive field, so it
    survives a lost local binding and the verdict's receipt-recovery path, and is mirrored on the
    binding. A named claim's command line and outcomes are unchanged.
- **A resume is named by construction.** `run.start --resume` binds the attributed id up front and
  never claims (`resumeAttributed()`); the verdict reads such a record as named from the run-tracker
  record itself, never from a claim.
- **Retries and continuations** keep the first claim's mode: the binding is bind-once, and
  `run.continue` does not claim.
- **Three modes, three behaviours:**
  - **loop-picked** (selection path recorded): a halt adds one to the count and may print `run-next`.
  - **named**: a halt never reads or touches the count and prints today's line.
  - **unknown**: a halt **adds one to the count** but prints **today's line** with no note. A loop
    never carries on without positive evidence, and a named run never sees the new line — safe both
    ways.
- A one-off unnamed run ("implement the next change") is selection-path, so it counts like a loop
  run. The docs say so. *(Groomer's call, from the review.)*

### 2. The halt count

- The run tracker keeps one count of **halts in a row** per repository, in the per-repo state folder
  under the git common dir, so every worktree and every session sees the same count (a new Claude Code
  session does not reset it). Keep it outside `<state>/run-tracker/` — the precedent is
  `acquireResumeLock`'s `run-tracker-resume/` — so the scanners that walk run-key folders never see it.
- `run.verdict` (keyed mode) updates it when it settles a run:
  - a `run-halted` outcome on a **loop-picked or unknown** change adds one;
  - only a **clean build** resets it to 0: the `run-complete` outcome (a change built to an open PR,
    or a results-artifact change completed), **named or loop-picked alike**, since a clean build
    proves the setup works;
  - every other outcome leaves it unchanged — including `run-unclaimed` and `no-attributable-claim`.
    `no-attributable-claim` covers an empty queue, a lost claim race, *and* every pre-claim halt, and
    the verdict cannot tell them apart; leaving the count alone means a pre-claim setup failure never
    hands the next loop a fresh limit. Also unchanged: `run-incomplete`, `run-continue`,
    `run-tracker-unavailable`, `ambiguous-claims`.
- **Once per key, both ways.** A run's effect on the count — an increment *or* a reset — applies at
  most once, however often `run.verdict` is re-run for that key. Save the decision (`run-next` or
  `run-stop`, and the count it saw) with the counted marker and replay it, so a replay after a later
  reset or halt never flips the line, and replaying an old `run-complete` never re-zeroes the count.
- Do **not** derive the count from persisted verdicts: `PruneRunTrackerRecords` deletes terminal
  records after 7 days and would silently shrink it.
- Unattributed (`--unattributed`) observe mode never reads or writes the count.
- **One loop per stage.** docket supports one implement loop and one finalize loop at a time (they may
  run side by side). Two implement loops at once are unsupported: they would share one count, and a
  healthy loop's clean builds would keep resetting a broken loop's halts. No gate prevents it — the
  docs state the support boundary instead. *(Human's decision.)*
- `repository.set-visibility` moves the state folder, which effectively resets the count. That only
  affects loops; document it.

### 3. The setting

- New config key **`run.max_consecutive_halts`**, integer, default **2**, minimum **1**. It layers
  like every other key (repo-local > repo-committed > global > built-in).
- `1` reproduces today's behaviour for loops: the first halt stops the loop.
- An invalid value (`0`, `-1`, `"two"`) is a config error exactly like any other int key's (for
  example `run.max_attempts: 0` today): it blocks config-reading operations until fixed. That is the
  uniform config rule, not a gate this change introduces; never clamp silently. An older binary only
  warns about the key as unknown.
- Read it at verdict time, or snapshot it at `run.start` the way `run.max_attempts` is — builder's
  call, **subject to *Isolation guarantees***: no record or binding schema bump, and a missing value
  falls back to resolved config or the default.

### 4. The verdict line

- **Loop-picked halt under the limit** (count after this halt < `run.max_consecutive_halts`): a new
  decision, **`run-next <key> run-halted <id>`**. It means: the change needs a person; never
  re-dispatch it; a loop driver may start its next run with a fresh `run.start`; a single dispatch
  stops, as on `run-stop`.
  - It goes through the existing halted branch (`persistRunVerdict`), never `runTrackerCompleteRun`,
    and keeps `terminal: true`. The run stays `active` exactly as a halted run does today
    (ADR-0148), so resume works unchanged.
- **Loop-picked halt at the limit:** **`run-stop <key> run-halted <id>`**, as today, followed by a
  note line telling the coordinating session to ask the person, worded without assuming a loop (for
  example: *"2 changes picked from the queue halted in a row. Reset the halt count to 0?"*), with the
  reset command and the instruction never to run it without a yes.
- **Named or unknown halt:** exactly today's `run-stop <key> run-halted <id>`, no note.
- JSON gains the decision value `run-next` and fields for the count and the limit (for example
  `halts_in_a_row`, `halt_limit`), **on loop-picked verdicts only**.
- No retry is spent on any halt, as today (ADR-0115).

### 5. Resetting the count

- New operation **`run.reset-halts`** (`docket run reset-halts`), effect `local-write`. It sets the
  count to 0 and reports what it was. It is safe to run at any time, typed by a person or run by the
  agent on the person's explicit yes.
- Without a reset, only a clean build (`run-complete`) brings the count back to 0. Consequence the
  human accepted: if a loop stopped at the limit and nobody resets, the next loop — in any session —
  stops at its first halt.
- An overnight loop gets no answer to the prompt. The loop has already stopped; the question waits
  for the person.

### 6. Where the loop logic lives

- **Dispatch block** (`internal/assets/embedded/tree/cursor-rules/run-tracker.md`, step 3): one
  sentence — `run-next` forbids re-dispatching that change; only a loop driver (`/loop`) may start its
  next run with a fresh `run.start`; a single dispatch stops, as on `run-stop`. This is the only
  always-loaded addition. Re-sync installs through the normal generator.
- **Verdict output:** the at-limit note carries the reset prompt, read only when it applies.
- **implement-next driver contract** (*Terminal disposition (driver contract)* in
  `skills/docket-implement-next/SKILL.md`): the `halted` row's driver action becomes "stop + surface;
  a loop whose tracked verdict is `run-next` continues", and the "The driver's decision is binary"
  sentence is edited to match. The four dispositions stay four; the child still declares `halted`. An
  untracked driver (a person re-issuing the request, or a run without `run.start`) has no verdict and
  keeps stopping on `halted`.
- **implement-next selection path:** passes the new claim mode input (§1). The named path passes
  nothing.
- Run `go generate ./internal/assets/` after the skill and asset edits.

## Isolation guarantees (from the design review)

These are requirements, not suggestions; each closes a path by which the counter could affect a run
that is not a loop-picked halt.

1. **No schema bump** of the run-tracker record (`runTrackerSchemaVersion`) or the claim binding
   (`bindingSchemaVersion`). `decodeRunTrackerRecord` and `readRunTrackerClaimBinding` reject any
   other version, so a bump would break every run in flight across the install — named runs, retries
   and continuations included (`run-tracker-unavailable corrupt-record`, `run-context-invalid` claims,
   `binding-unreadable`). New fields are additive and optional, with **no save-time guard** on them
   (unlike `writeRunTrackerRecordAtomic`'s `AttemptLimit < 1` refusal, which a copied guard would turn
   into `report-unpersisted` on old-shape records).
2. **Count faults never change a report line.** Failing to read or write the count or the counted
   marker never produces `run-tracker-unavailable`:
   - on a loop-picked halt, a fault prints today's `run-stop … run-halted <id>`;
   - on `run-complete`, the reset is best-effort and happens only after the `run-done` line is final
     (after the checked save); if the closeout is blocked, nothing resets;
   - named halts do not read the count at all;
   - any lock on the count is bounded and never blocks a verdict.
3. **The claim input never refuses** (§1).
4. **A named run can never see `run-next` or the at-limit note** — named and unknown modes print
   today's line (§1, §4).

Per-path result with these in place:

| Path | Verdict line | Different from today? |
|---|---|---|
| Named run (single id) that halts | `run-stop <key> run-halted <id>` | No — the count is never read |
| Named run that builds cleanly | `run-done … run-complete` | No — the reset is a best-effort side effect |
| `--resume` | named by construction → today's lines | No |
| `run-retry-once` / `run-continue` of a named run | today's lines | No |
| Untracked run | no verdict; stops on `halted` | No |
| `--unattributed` observe | no count access | No |
| Finalize | not run-tracked | No |
| One-off unnamed run that halts | counts; may print `run-next`, and the coordinator stops | Count only |
| Loop-picked halt | `run-next` under the limit, today's line + note at it | Yes — the feature |

## Docs

Describe current behaviour only (no change or PR citations):

- `docs/guide/building-without-supervision.md` (*Draining the queue hands-free*): the loop carries
  past a halted change it picked, up to the limit; which runs count (picked from the queue, including
  a one-off unnamed run; not a named run or a resume); what resets the count; how to reset; that
  docket supports one loop per stage (one implement loop and one finalize loop at a time, side by side
  is fine) and why a second implement loop breaks the halt limit; that `set-visibility` resets the
  count and refuses while halted runs are still active (each must be cancelled first — an existing
  rule, now with up to the limit's worth of halted runs instead of one). Check the other guides and
  concepts for wording that implies concurrent same-stage loops are supported, and correct it.
- `docs/guide/landing-changes.md` (*Closing out hands-free with `/loop`*): it currently says
  implement and finalize share one stop rule — finalize still stops on every `halted`; implement's
  tracked loop may continue on `run-next`.
- `docs/concepts/run-tracker.md`: the verdict flow diagram gains `run-next`; the halt count.
- `docs/reference/config-keys.md`, `docs/reference/cli.md`, `docs/reference/outcomes.md`,
  `docs/reference/glossary.md` as applicable.

## Constraints

- The run tracker owns the accounting; the coordinator never counts halts itself (CLAUDE.md run
  tracker rules).
- Config keys behave uniformly — no single-layer scoping for the new key.
- No new blocking gate: the limit only *stops a loop*, which is today's behaviour on every halt.
- Guards are mutation-tested; the whole suite runs at the build gate. Each isolation guarantee gets a
  test that a named run's verdict line is byte-identical to today's under a count fault, a high count,
  and an old-shape record.

## Left to the builder

- The on-disk form of the count and its counted markers, within §2 and the isolation guarantees.
- The exact name and values of the claim mode input, the JSON field names, and the exact wording of
  the at-limit note, within the meaning above.
- Whether the decision is worth an ADR (likely yes: it amends ADR-0153's "every `halted` stops the
  driver" context and extends ADR-0115's run accounting).
