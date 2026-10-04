<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0510 — Match reported follow-ups against proposed and deferred changes](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0510-match-reported-follow-ups-against-proposed-and-deferred-chan.md)**
<!-- docket:backlink:end -->

# Match reported follow-ups against proposed and deferred changes — design

## Problem

`docket-implement-next` records out-of-scope work it found but did not fix as entries under the
results file's `## Known issues and follow-ups` section, and repeats it in the final report as
"follow-up work reported for deliberate capture". The governing rule (Step 6.5, *Known issues and
follow-ups*) reads "link an existing change when one is known", and the results template's
placeholder reads "linking an existing change when available". Neither tells the run to search.
The run links a change only when that change is already in its context, so most follow-ups reach
the human unlinked, and the human must recall whether the backlog already owns each one before
capturing it. A missed match becomes a duplicate change.

## Investigation and prior decisions

Design baseline: main `5805127ab03a9413c7ced1c2e5a74080d60bafca`, inspected 2026-10-04.

- The wording lives in maintained skill prose only:
  `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` (Step 6.5 *Known issues and
  follow-ups*; Step 3's reconcile note on adjacent follow-up work; Step 6's review-finding report
  rule; the final-report enumeration), `…/docket-implement-next/references/fix-loop.md`
  (*Beyond-the-branch findings are reported*), `…/docket-implement-next/results-template.md` (the
  `## Known issues and follow-ups` placeholder), and `…/docket-convention/SKILL.md` (*Results
  artifact shape and lifecycle*). The root `skills/` tree is a derived copy of the embedded tree.
  No Go test pins any of these sentences (whole-repo grep for the phrasing, excluding worktrees and
  `.docket`, found only skill prose and frozen merged plans).
- Sampling the twelve most recent results files: 0496 linked #502 and 0493 linked #492, both
  because the change was already in context; every other follow-up entry was unlinked.
- #302 (deferred) recorded the retired `mint-stub.sh` dedup missing a match because it compared
  titles, and a general-form parent shares little wording with a specific discovery. Matching here
  is therefore a reading judgment over each candidate's stated scope, not a title or keyword
  comparison. It stays in the model, inside the skill, which is consistent with ADR-0012 (no model
  call inside a deterministic operation).
- `status --json` already returns every change's `id`, `title`, `status`, and `path`, so the
  candidate list needs no new operation. The record bodies are read from the metadata working tree
  the startup check already prepared.
- implement-next already refuses to mutate a record it does not own ("you never mutate an
  unrelated proposed record"). Change 0509 widens `change.groom` `outcome: revise` to edit any
  `proposed` change, stubs included, without changing its groom state. That gives a human a typed
  path to fold a follow-up into a matched stub instead of hand-editing `.docket`. `revise` still
  refuses `deferred`, which `change.revive` returns to `proposed` first.

## Design

Prose change to implement-next's results consolidation; no new operation, request field, gate, or
halt.

### 1. When

Once, at Step 6.5 checkpoint **(iv) final consolidation**, after the Known-issues entries are
settled and before the implemented transition. Earlier checkpoints do not run it. A halt before
final consolidation leaves entries in today's form.

### 2. Candidates

Run the `status` operation (argv from the capability catalog) with `--json` and keep changes whose
`status` is `proposed` or `deferred`, excluding the change being built. Do not consider
`in-progress`, `blocked`, `implemented`, or `stacked-merged`: those are claimed or built, and
folding new scope into them would confuse the run or finalize that owns them. Do not consider
archived changes.

### 3. Reading

For every candidate, read the `## Why` and `## What changes` sections of the record at its `path`
in the metadata working tree, not only the candidates whose titles look similar. Read a
candidate's linked spec only when those sections leave the match undecided.

### 4. Verdict

Give each follow-up entry exactly one verdict:

- **Fits #N** — #N's stated scope already covers the follow-up, or would with a small extension
  that does not change what #N is for. When several fit, name the closest and mention the others.
- **Related to #N** — it touches the same area, but folding it in would change what #N is for; it
  needs its own change. Name #N so the human can link it under `related:` on capture.
- **No existing change fits** — state the number of `proposed` and `deferred` changes checked.

Entries that are not out-of-scope follow-up work (an unresolved in-scope limitation, an
uncertainty about verification coverage) get no verdict.

### 5. Suggested next action by target state

| Target #N | Suggested next action |
|---|---|
| `proposed`, needs grooming | Edit #N to add it (`docket-groom-next` edit path, `change.groom` `revise`); #N stays needs-grooming. |
| `proposed`, groomed (spec or trivial) | Edit #N's owned sections; if the spec must change, revise it with `docket-groom-next <N>`. |
| `deferred` | Revive #N, then edit it; consider whether the follow-up strengthens the case to revive. |
| Related or no fit | Capture a new change with `docket change create`, linking any related #N under `related:`. |

### 6. Where it appears

Each affected Known-issues entry ends its suggested next action with the verdict and action. The
final report's follow-up list carries the same verdict per item, so the chat summary and the
durable results file agree.

### 7. Failure posture

If the `status` read fails or a candidate record cannot be read, write the entries in today's form
and add one line to `## Verification performed` saying the backlog check did not run (or which
candidates were unreadable). Never halt, retry in a loop, or block the implemented transition on it.

### 8. What it never does

Never edits, creates, revives, defers, or kills any change; writes only the run's own results file
and final report. Never adds a gate.

### 9. Wording edits

- `docket-implement-next/SKILL.md` Step 6.5 *Known issues and follow-ups*: replace "link an existing
  change when one is known" with a pointer to the backlog match at final consolidation; add a short
  *Backlog match* paragraph carrying sections 1–8 compactly. Final-report enumeration: follow-up
  work is reported with its backlog verdict.
- Step 3's adjacent-follow-up note, Step 6's review-finding report rule, and `fix-loop.md`
  *Beyond-the-branch findings are reported*: keep "reported, never minted"; add that the report
  carries the final-consolidation backlog verdict. Do not restate the procedure.
- `results-template.md`: the placeholder names the verdict ("Fits #N", "Related to #N", or "No
  existing change fits (checked K)") and the matching next action.
- `docket-convention/SKILL.md` *Results artifact shape and lifecycle*: "link an existing change
  when one is known" becomes "the run checks proposed and deferred changes and names any match",
  keeping "never mint".
- Regenerate the root `skills/` copy and any other derived copies through the existing generator
  or sync check the suite enforces, never by hand.

## Out of scope

- Writing to any change other than the one being built.
- Matching against claimed, built, or archived changes.
- finalize's `late_findings` in closeout notes.
- A deterministic dedup operation or a title-similarity helper (#302's territory).
- Re-running the match at earlier checkpoints or after a halt.

## Dependencies

Depends on #509: section 5's stub and groomed rows name its widened `revise` edit path. Building
this first would point humans at a path that refuses stubs.

## Testing

Prose-only; no behavior a Go test can drive. Run the whole suite at the build gate for the
skill-sync and embedded-tree checks. If the suite has a skill-prose contract test covering Step
6.5 or the results template, update it to the new wording and mutation-check it (restore the old
sentence; the test must redden).

## Success criteria

- A results file from a run with follow-ups shows, for each one, a verdict naming #N or the count
  of changes checked, and a next action that matches #N's state.
- No implement-next run writes to any record other than its own change.
- A failed backlog read still produces a complete results file, with the skipped check named under
  Verification performed.
