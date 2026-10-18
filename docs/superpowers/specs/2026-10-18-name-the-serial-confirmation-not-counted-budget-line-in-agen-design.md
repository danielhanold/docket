# Point AGENTS.md's budget-report rule at tests/README.md instead of listing the lines — design

Change 0558 — Point the budget-report rule in AGENTS.md at tests/README.md instead of listing the lines

Change #558, groomed 2026-10-18.

## Problem

The test suite runner (`docket development test`, `internal/suiterunner`) prints a budget report
whenever a test file's wall-clock time crosses a line, and it prints it on green runs too. No
budget line fails a run (`tests/README.md`: "No budget finding fails a run"), so an agent that
sees a green summary and stops reading misses a real breach. Nothing else surfaces them: no docket
skill, agent file, or gate-driver code reads these lines. An agent sees them only by reading the
suite output.

`AGENTS.md`, which is loaded into every agent session in this repository (`CLAUDE.md` is a symlink
to it), carries this rule in its *Guards and tests* section:

> - Read the budget report even on a green run: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line
>   is a screening finding, and a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative
>   breach to act on. Neither fails the run (a parallel wall-clock number is
>   machine-dependent, so a real breach is confirmed serially; see `tests/README.md`), so
>   nothing else will catch them for you.

The runner prints five line types today, and the bullet names three:

| Line | What it is | Named in AGENTS.md? |
|---|---|---|
| `BUDGET WATCH:` | screening note from a parallel run | yes |
| `PARALLEL-SENSITIVE:` | screening note from a parallel run | yes |
| `SERIAL CONFIRMED OVER BUDGET:` | breach confirmed by a solo re-check | yes |
| `OVER BUDGET:` | breach measured directly: a single-job run (`DOCKET_RUNTESTS_JOBS=1`) or a `serial` budget row | **no** |
| `SERIAL CONFIRMATION NOT COUNTED:` | the solo re-check ran while another suite was running, so the reading was set aside; the test stays due for the next quiet run | **no** |

The bullet is a hand-kept copy of what `tests/README.md` already explains, and it has fallen
behind twice. The missing `OVER BUDGET:` line matters: an agent doing a quiet single-job
re-measure can see one and not know it is a breach (the release-candidate workflow fails a release
on it).

## Decision

The part of the rule that must fire without prompting is the reminder: read the budget report even
on a green run, because nothing fails on it. That stays in `AGENTS.md`. What each line means is
reference material, and an agent that has just been told to read the report knows to look it up.
It moves out of `AGENTS.md` and lives in `tests/README.md`, which the bullet already points to.
The same fix applies to the one other maintained copy of the list kept as a rule rather than as
reference, the header comment of `tests/runtime-budgets.tsv`.

Why not a skill: everything under `skills/` ships to every repository that uses docket, and these
lines come only from docket's own contributor suite (the glossary says none is part of the build
gate in a user's repository). A skill local to this repository would have to be loaded by an
agent that already thinks it is done. That fails the `AGENTS.md` test for what belongs there:
"will the agent know to search for this?" `AGENTS.md` also reaches every harness (Claude, Codex,
Cursor). A skill doesn't.

## What changes

### 1. `AGENTS.md`: replace the bullet

Replace the whole *Read the budget report…* bullet in the *Guards and tests* section with:

> - Read the budget report even on a green run: no budget line fails the run, so nothing else
>   will catch a real breach. `tests/README.md` says which lines are screening notes, which are
>   breaches to act on, and which need nothing.

The bullet names no line prefix. Keep the wrapping and indentation style of the surrounding
bullets. Do not edit `CLAUDE.md` separately; it is a symlink to `AGENTS.md`.

### 2. `tests/README.md`: make the budget paragraph say each line's meaning outright

The new bullet promises the README sorts every line into one of three kinds, so the README has to
say so plainly. Today it describes `SERIAL CONFIRMED OVER BUDGET:` as an authoritative breach, but
for `OVER BUDGET:` it only says "a crossing prints `OVER BUDGET:`". For
`SERIAL CONFIRMATION NOT COUNTED:` it says the reading is not counted and the test stays due, but
not that the reader has nothing to do. Edit the budget paragraph (the two bullets after "Budgets
are checked two ways.") so that:

- `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` are called screening notes (the current "advisory
  screening finding" wording is fine).
- `SERIAL CONFIRMED OVER BUDGET:` and `OVER BUDGET:` are both called authoritative breaches to act
  on, i.e. bring the file back under its ceiling. The paragraph after "**Never grow a file past
  its budget and raise the number.**" already says how, so point to it instead of restating it.
- `SERIAL CONFIRMATION NOT COUNTED:` is said to need no action, because the runner re-checks the
  test on the next quiet run.

Keep the existing facts (thresholds, when a solo re-check is scheduled, single-job and `serial`
measurement). Add only the missing classification words. Describe current behavior only, with no
change numbers (house rule for maintained docs).

### 3. `tests/runtime-budgets.tsv`: cut the header comment's line list to a pointer

The header comment's *ENFORCEMENT* paragraph has a third maintained copy of the same three-line
list ("BUDGET WATCH / PARALLEL-SENSITIVE are screening findings; SERIAL CONFIRMED OVER BUDGET is an
authoritative breach"). Replace that parenthetical with a pointer to `tests/README.md` for what
each budget line means, the same fix as `AGENTS.md`. Keep the rest of the comment. Its other
sentences, including the change numbers already in it, are out of scope here.

### 4. `docs/reference/glossary.md`: name `OVER BUDGET:`

The entry *Budget watch / serially confirmed breach* already names four lines and leaves out
`OVER BUDGET:`. Add one clause saying an `OVER BUDGET:` line, measured directly (by a single-job
run or for a `serial` budget row), is also an authoritative breach to act on. Do not rename the
entry heading; other docs may link its anchor. The glossary keeps its list because it is the
reference that defines these terms for human readers, not a rule copied into always-loaded agent
context. (Designer's call.)

## Out of scope

- Any change to the runner, its output wording, or which lines exist.
- `.github/workflows/release-candidate.yml`. Its greps and comment already treat `OVER BUDGET:`
  and `SERIAL CONFIRMED OVER BUDGET:` as authoritative and leave NOT COUNTED out on purpose.
- Old plans, specs, results, and release records that quote the old bullet or name only three
  lines. Those are point-in-time records and stay as written.
- Adding a guard test for `AGENTS.md` wording. No test guards this bullet today and none is
  wanted. The rule is that a line list must not be copied into `AGENTS.md`, not that some exact
  text must be present.

## Verification

- `git diff` touches only `AGENTS.md`, `tests/README.md`, `tests/runtime-budgets.tsv`, and
  `docs/reference/glossary.md` (plus the spec copy the branch carries).
- `git grep -nE "BUDGET WATCH|PARALLEL-SENSITIVE|OVER BUDGET|NOT COUNTED" -- tests/runtime-budgets.tsv`
  returns nothing, and the registry correspondence guard (`TestRuntimeBudgetsCorrespondence`)
  stays green. The edit touches comment lines only, never a row.
- `git grep -nE "BUDGET WATCH|PARALLEL-SENSITIVE|OVER BUDGET|NOT COUNTED" -- AGENTS.md` returns
  nothing.
- In `tests/README.md`, each of the five prefixes appears in the budget paragraph next to its
  kind: screening, breach to act on, or needs nothing.
- The whole suite passes at the build gate (`build.test_command`). No test reads these files'
  budget wording, so a red here is unrelated and is handled the usual way.

## Left to the builder

Exact sentence wording in `tests/README.md`, the `tests/runtime-budgets.tsv` comment, and the
glossary, within the constraints above.
