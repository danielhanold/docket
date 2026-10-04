<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0516 — Remove stale auto_groom comments and fix TestSkillHandoffSites' 'cannot be invoked' match](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0516-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit.md)**
<!-- docket:backlink:end -->

# Change 0516 — stale auto_groom comments and the handoff guard's negation match

## Summary

Two leftovers from change 0502, both small and both behavior-neutral for docket users:

1. Three Go comments still say an unset `auto_groomable` inherits a repository `auto_groom`
   setting. Nothing does that: only an explicit `auto_groomable: true` opts a stub in.
2. `TestSkillHandoffSites` misreads "cannot be invoked" (and "can't be invoked") as an
   invocation, so a wrapper-backed skill line that names a role skill next to that phrase is
   wrongly required to carry the `DIRECTED to:` marker. 0502 reworded skill text to avoid it.

## Part 1 — comments

Current truth, traced: the only consumer of the flag, the auto-groom eligibility check in
`internal/app/change_groom.go`, counts a stub as auto-groomable only when
`AutoGroomable().State == domain.FieldPresent && .Value`. Unset and `false` behave identically.
The tri-state survives only so the record writer round-trips what the human wrote (an unset field
stays unset; nothing writes a `false` nobody typed).

Rewrite these comments to say exactly that — unset or `false` means not auto-groomable, only
`true` opts in — with no mention of a repository setting:

- `internal/domain/entities.go` — the `AutoGroomable` field comment on the change spec struct
  ("per-change auto-groom override; unset ⇒ inherit auto_groom").
- `internal/domain/entities.go` — the doc comment on `func (c Change) AutoGroomable()`
  ("Unset (absent or valueless) means the repository's auto_groom knob applies.").
- `internal/app/change_create.go` — the `AutoGroomable *bool` request-field comment
  ("nil leaves the record unset (inherit the repo's auto_groom)").

Leave the generic `OptionalBool` type comment ("absent or valueless (inherit a default) stays
distinguishable from an explicit false") as is: it describes the type, not this field, and is
true. Before editing, re-run a whole-repo grep for the stale claim (maintained Go source and
prose) and fix every executable-adjacent site it finds, not only these three.

## Part 2 — the negation pattern in `TestSkillHandoffSites`

File: `internal/repoguard/skill_handoff_sites_test.go`. The classifier
(`classifyHandoffSite`) calls a line an invocation when its invoke-verb matches (`invokeRe`)
outnumber its prohibition matches (`negatedInvokeRe`). Today `negatedInvokeRe` is

    (?i)\b(?:not|never)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b

`\bnot` needs a word boundary before `not`, so "cannot" never matches, and a contraction
("can't", "won't", "isn't") has no `not` at all.

### Design — key on the negation's shape, not a list of spellings

A negation is any of three shapes, within the same two-word window before the verb:

- a word ending in `not` — covers `not` and `cannot` (`\b\w*not`);
- `never` as a word (`\bnever`) — unchanged;
- a word ending in the `n't` contraction, straight or typographic apostrophe
  (`\w+n['’]t`) — covers `can't`, `won't`, `isn't`, `doesn't`.

Candidate pattern (verified by hand against the cases below while grooming):

    (?i)(?:\b\w*not|\bnever|\w+n['’]t)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b

`never` keeps its leading word boundary on purpose, so "whenever invoked" is not a negation.
The class does not try to understand meaning; per the learnings finding
*byte-pattern-guard-matches-a-spelling*, state the residual in the header comment: a negation
outside these shapes (for example "unable to invoke", "fails to invoke") is still read as an
invocation. Do not widen the pattern to cover those speculatively (YAGNI); widen it when a real
line needs it.

Update the file's header comment ("# Classes, each keyed on shape") so the prohibition
examples include the `cannot` / `n't` shapes, and the residual is stated.

### Tests — extend the existing `non_vacuity` subtest

Add cases to the `non_vacuity` subtest that exercise `classifyHandoffSite` directly:

| Line | Expected |
|---|---|
| "When `docket-review` cannot be invoked, review the branch inline." | mention |
| "When `docket-review` can't be invoked, review the branch inline." | mention |
| "When `docket-review` can’t be invoked, review the branch inline." (typographic apostrophe) | mention |
| "Invoke `docket-build`; when `docket-build` cannot be invoked, run the plan inline." (no marker) | invocation — 2 invoke verbs vs 1 negation, so it must still be caught |
| "`docket-build` is invoked whenever the plan is ready." | invocation (no false negation from "whenever") |

The existing cases (unmarked, marked, mention, prohibited "do NOT invoke", tier, framed) stay
and stay green.

### Mutation check (required, both directions)

- Revert `negatedInvokeRe` to the old pattern: the new "cannot"/"can't" cases must redden.
- Make `negatedInvokeRe` match everything an `invokeRe` matches (or delete the subtraction in
  `classifyHandoffSite`): the unmarked-invocation and mixed-line cases must redden.
- Remove the `DIRECTED to:` marker from one live invocation line in a scratch copy (for
  example `skills/docket-implement-next/SKILL.md`'s build-role line): the whole-tree scan must
  still redden. Revert afterwards.

Record each mutation and its red result in the results file.

## Live-tree impact (checked while grooming)

Re-classifying every wrapper-backed role-skill line in `skills/` under the candidate pattern
changed none of them: the same three lines in `docket-implement-next` stay invocations (all
carry `DIRECTED to:`), and every mention stays a mention. The invocation floor
(`invocations < 3`) is met exactly and keeps holding. If the build finds a line whose class
flips, stop and report it rather than editing skill prose to fit.

## Out of scope

- Restoring 0502's skill wording ("when the `docket-review` skill is missing") to "cannot be
  invoked". The current wording reads fine; this change only makes the guard correct.
- The `auto_groom` row in `internal/config/schema.go` (a deferred setting: `false` is inert,
  `true` blocks writes). Retiring it is a behavior change; the human chose to leave it.
- Any auto-groom selection behavior.
- The frozen fixture `internal/render/testdata/records/PROVENANCE.md`.

## Verification

- `go test ./internal/repoguard/ -run TestSkillHandoffSites` green after the change, red under
  each mutation above.
- Full suite via `build.test_command` green at the build gate.
