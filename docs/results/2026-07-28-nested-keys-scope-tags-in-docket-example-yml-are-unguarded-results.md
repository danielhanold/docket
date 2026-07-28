<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0122 — Nested keys' scope tags in .docket.example.yml are unguarded](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0122-nested-keys-scope-tags-in-docket-example-yml-are-unguarded.md)**
<!-- docket:backlink:end -->

# Nested keys' scope tags in `.docket.example.yml` are unguarded — results

Change: #0122 · Branch: `feat/nested-keys-scope-tags-in-docket-example-yml-are-unguarded` · PR: (pending) · Plan: `docs/superpowers/plans/2026-07-28-nested-key-scope-tags-plan.md` · ADRs: none

## Verify (human)

Nothing interactive — the guard is a test, and it proves itself. Two optional spot-checks if you want the evidence in front of you rather than in CI:

- [ ] `bash tests/test_docket_example_yml.sh | grep -E 'scope tag:|guard-the-guard'` — expect 11 `ok` lines: 3 tag-form presence, the depth-wide coverage assert, the exact-17 floor, the adjacency-inheritance counter, 3 mutation reports, and 3 one-line-delta asserts.
- [ ] Confirm `.docket.example.yml` is byte-unchanged on this branch (`git diff origin/main -- .docket.example.yml` → empty). Zero edits to the example was a design constraint, not an outcome.

## Findings

**The plan's guard program was verified before the plan was written, and it paid off.** The awk pass and all three expected mutation outputs were executed against the real file during the reconcile/plan pass, so the values in the plan were measured rather than predicted. All three matched on the implementer's first run, and no expectation was adjusted mid-build. Zero findings across all three per-task reviews — the `plan-supplied-test-code-is-unverified` learning's front-loading claim, reproduced.

**The whole-branch review found the one thing the task-level reviews structurally could not.** Three clean task reviews did not mean the branch was clean: the final review (opus, whole-branch) surfaced one Important finding that only exists in the *interaction* between two tasks.

- **Rule 4 plus the floor's failure message was a laundering path.** Rule 4 grants coverage to a scalar with a genuinely empty comment window sitting immediately below a same-depth key — it exists for the `changes_dir`/`adrs_dir`/`results_dir` shared-comment group. A new nested key added with *no comment at all* beneath a tagged sibling therefore inherited coverage silently; only `COUNT` moved, 17 → 18. The exact-17 floor then failed with a message whose remedy was "bump `expected_nested_key_count` in the same commit" — and doing exactly that returned the suite to green with an untagged key shipped. The cheapest way to add a nested key was the one that evaded the guard.
- **Closed in both halves.** The floor's message now leads with "confirm the new key carries its own tag or sits under a tagged header" before mentioning the bump; and the pass emits a second counter, `ADJINHERIT`, for keys covered *via* rule 4, asserted to equal 2 (`adrs_dir` and `results_dir`). The counter is what actually closes it — the re-review confirmed the assert reddens on the mutation *independently of* `COUNT`, so laundering the count to 18 still fails. Adjacency inheritance is now a loud, budgeted event rather than a free one.
- **This is the design decision made during the build**, and it is deliberately recorded here rather than as an ADR: it refines the spec's rule 4 inside one test guard, with no blast radius beyond it. The spec's own architecture decisions (inheritance over per-key tags, and the accepted residual in assumption 11) were settled at design time and needed no revisiting.

**Two smaller review findings, both real:**

- The window scan did not require a matched tag to sit in a *comment*, so prose quoting a sanctioned form granted coverage. `.docket.example.yml`'s own legend contains all three forms verbatim and was out of scope only because a `# ═══` banner seals it — the first active key ever added above that banner would have been vacuously covered by the legend. Now anchored on `^[[:space:]]*#`.
- Widening the key anchor from column 0 to `^[[:space:]]*` means indented non-key text (inside a YAML block scalar, say) would parse as keys and inflate the count. No block scalars exist in the file today and the exact floor makes any inflation loud, so this is recorded as a comment at the anchor rather than defended in code.

**Verification standard met.** All 63 test suites green on a clean tree. The guard program, both mutation helpers, and every verification command were run under `/usr/bin/awk` and `/usr/bin/grep` — never the PATH tools, since PATH `grep` here is ugrep and accepts constructs BSD grep rejects (the `grep-is-ugrep` trap). The reviewers independently reproduced the anti-masking behavior against 15 hand-built fixtures, including a tab-indented case.

**Accepted residual, unchanged from the spec (assumption 11):** a *wrong but well-formed* tag is still never caught — on a header it masks its subtree, on a leaf it masks that leaf. That is the deliberate price of choosing inheritance over per-key tags, and it is a strictly smaller hole than the pre-0122 one, where a child's tag masked the whole block in both directions. Detecting it needs per-key expected values, i.e. the hand-maintained allowlist the spec rejects.

## Follow-ups

- **#0147 — extend the `(2c)` orphan-key check past its column-0 anchor** (auto-captured during reconcile). `(2c)` enumerates keys with the same column-0 anchor this change removed from the scope-tag guard, so all 17 nested keys are invisible to the orphan direction too. Deliberately not folded in: `(2c)` anchors on *consumers*, and nested keys reach theirs through different paths (`runners.*` via the runner adapters, `skills.*` via the `SKILL_*` exports), so it is a design question rather than a mechanical widening.
- **Change 0102's two bespoke asserts were KEPT, not retired.** The stub's retire instruction was conditional on the general check covering them, and it does not: the tag assert pins the *specific* `any layer` value, while the general check only proves *some* sanctioned form covers the key. Retiring it would silently accept a relabel to `repo-only` — the exact 0102 bug class.
- **Co-writer note for whoever lands next:** #0121 rewrites the classification manifest / `elsewhere:` check in this same file, and #0103 may add nested keys to `.docket.example.yml` (which will trip the exact-17 floor — that is the floor working as designed; bump it in that change's commit). Both were still un-started at build time, so the exposure is textual-merge only.
