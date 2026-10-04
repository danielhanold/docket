<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0516 — Remove stale auto_groom comments and fix TestSkillHandoffSites' 'cannot be invoked' match](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0516-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit.md)**
<!-- docket:backlink:end -->
# Remove stale auto_groom comments and fix TestSkillHandoffSites' 'cannot be invoked' match — Results

**Human action:** None needed. The change touches only Go comments and a repository guard test; nothing user-facing behaves differently.

## Outcome

Two leftovers from change 0502 are cleaned up.

- Three Go comments (`internal/domain/entities.go` twice, `internal/app/change_create.go` once) no longer claim that an unset `auto_groomable` inherits a repository `auto_groom` setting. They now say what the code does: unset or `false` means not auto-groomable, and only an explicit `true` opts a stub in. A whole-repo grep found no other maintained site; the remaining hits are frozen plans and specs.
- `TestSkillHandoffSites` now recognises a negation by its shape: a word ending in `not` (`not`, `cannot`), a word ending in the `n't` contraction (straight or typographic apostrophe), or the word `never`. A skill line such as "when `docket-review` cannot be invoked" is now read as a mention rather than an invocation, so it no longer needs the `DIRECTED to:` marker. The test's header comment states what it still misses: negations outside these shapes, such as "unable to invoke", are still read as invocations.

Re-classifying all 29 role-skill lines in `skills/` under the new pattern changed none of them, so no skill prose was edited.

## Verification performed

- Full suite (`build.test_command`) green at the build gate through the gate driver.
- Mutation checks on the guard, each restored afterwards:
  - Old negation pattern restored: red, with the three cannot / can't / can’t cases reported as invocations.
  - Negation pattern made to match every invoke verb: red on the invocation floor ("only 0 marker-checked invocation lines"); with the floor check also switched off, red on the unmarked, marked, and mixed-line invocation cases.
  - `DIRECTED to:` removed from the build-role line in `skills/docket-implement-next/SKILL.md`: the whole-tree scan reddened on that line.
  - `\b` dropped before `never`: red on "`docket-build` runs whenever invoked by the controller."
  - `\b\w*not` widened to `\b\w*not\w*`: red on "`docket-build` is annotated, invoked by the controller."
- Whole-branch review (standard tier): 2 minor findings, both about test cases that could not fail under any mutation; both fixed in-branch.
