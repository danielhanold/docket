<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0170 — Lean Docket-owned whole-branch review skill](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0170-lean-whole-branch-review-skill.md)**
<!-- docket:backlink:end -->

# Lean whole-branch review skill + suite-once evidence chain — results

Change: #0170 · Branch: feat/lean-whole-branch-review-skill · PR: <url> · Plan: docs/superpowers/plans/2026-08-01-lean-whole-branch-review-skill.md · ADRs: 66

## Verify (human)

- [x] **The `review: docket-review` dogfood binding cannot be verified before merge.** **VERIFIED post-merge (2026-08-01):** `docket.sh env` reports `SKILL_REVIEW=docket-review`. `scripts/docket-config.sh` resolves config from `git show "origin/HEAD:.docket.yml"` — the merged default branch — not the working tree, so on this branch `docket.sh env` still prints `SKILL_REVIEW=superpowers:requesting-code-review`. The edit was proven correct in an isolated throwaway fixture (bare origin + clone with this branch's `.docket.yml` and `scripts/` pushed to its `main`, `remote set-head` applied), where the resolver printed `SKILL_REVIEW=docket-review` and a control mutation tracked. **After merging, re-run `"${DOCKET_SCRIPTS_DIR:?}"/docket.sh env | grep SKILL_REVIEW` and confirm it reports `docket-review`.** This is the plan's Task 6 Step 6, deferred to the merge gate because no in-branch run can satisfy it.
- [ ] **The three new wrappers register only after a fresh session.** `install.sh` generates them and a harness registers agents at process start, so `docket-review-lean/-standard/-deep` are not dispatchable in any session that began before this merges. Re-run `install.sh`, then start a fresh session before the first real `docket-review` dispatch.
- [ ] **First live dispatch is unexercised.** Every guard here is static — the suite proves the wrappers generate, carry the right pins, and are named consistently, but nothing has actually *run* a `docket-review` rung end to end. The first `docket-implement-next` run after merge is the real test of rung selection, the evidence handoff, and the finding schema.

## Findings

**ADR-0066 — docket owns the review role; the suite runs in the build gate.** The change's central decision, recorded at build time per the spec: docket ships its own read-only reviewer behind three pinned rungs; the full suite's implementation-phase home is the build gate, not the reviewer, because the suite answers the *build's* question while review asks "is this good?", and because the repair machinery already lives on the build side — a suite inside a reviewer forbidden to fix would recreate the build→review→build loop this design exists to kill. Its consequences are recorded unsoftened, including the two below.

**The whole-branch review found two provably-false prose claims, one of them pinned green by a guard.**

1. The README claimed *"one full-suite run on the clean path, two in the worst case, and never three."* False by the change's own contract: a blocker fix's commits trigger a suite re-run (2), and a later real rebase at finalize adds a third. `tests/test_docket_review.sh` was asserting `grep -qiE "never three"` — so the guard had turned a prose error into a *guarded* prose error. Corrected to the honest arithmetic (one / two / three, with the conditions for each) and the assert re-keyed onto surviving phrases. The same stale claim was corrected in the change body on the `docket` branch.
2. `skills/docket-convention/SKILL.md`'s reworded convention-injection clause asserted that "that contract is the only thing those wrappers load" across all eight exceptions — false for `docket-brainstorm-consultant`, which wraps no skill at all (a fact the *Composition* paragraph two lines below states). Re-scoped to the seven wrapper-bearing exceptions.

This is the repo's own recorded `verify-the-claim` failure mode reappearing inside the change that ships a reviewer — worth noting that the *suite* could not have caught either one, and the whole-branch read did.

**A pre-existing off-by-one was fixed in passing.** The same convention sentence read "every wrapper except **four**" while naming five — left behind by change 0184's fourth build profile. Corrected to eight (brainstorm-consultant + four build workers + three review rungs) rather than captured separately, since this change already had to edit that sentence.

**Rung selection had an undefined input.** The rule takes "the highest profile any task routed or escalated to" and brands itself "never model judgment" — but `build` and `review` are independently bindable and the *shipped default* build skill emits no build record, so a repo setting only `review: docket-review` reached a rule with no input. Now defaults to `docket-review-standard` (matching how `standard` is docket-build's own uncertainty sink), with the diff-size bump still applying.

**The diff-size threshold was ambiguous and this branch sits one line from it.** "More than 1500 changed lines" over `git diff --shortstat`, which emits three numbers. This branch measured 1431 insertions + 58 deletions = 1489 — one honest edit from flipping the rung depending on the reading. Now specified as insertions + deletions.

**Guard hygiene: three near-vacuous asserts were found and tightened, none weakened.** A bare `|halt` alternation that passed before the prose it guarded existed; a `grep -qiE "once|one run"` over the entire 900-line README; and a negated grep over an `awk` range whose haystack goes empty — and therefore permanently green — if the fence markers are renamed. Each now carries a paired non-vacuity anchor, and each tightening was mutation-proven.

**A plan-authoring defect recurred four times.** The plan's hand-written asserts and its suggested prose were written in the same pass and mismatched in Tasks 3, 4, 5 and 6: an `evidence`-within-30-characters proximity regex, a case-sensitive `important`, a bolded `**no-op**` whose asterisks broke its own pattern, and a whole-file README grep. Each was resolved by keeping the assert and fixing the prose (or tightening the assert where it was genuinely wrong) — never by weakening an assert. Worth a learnings finding at harvest: *plan-supplied test code and plan-supplied prose are two independent unverified artifacts, and a plan that supplies both has not checked them against each other.*

**The PR body is trusted input to a merge-gating decision.** Anyone who can edit a PR description can paste a green evidence block at the current HEAD SHA and finalize will merge without running the suite. Nil exposure for a single maintainer; real for repos with outside contributors. The PR body was still the right home (finalize is a cross-session, cross-machine consumer; `.superpowers/` files are gitignored and transient), so the trust boundary is now stated explicitly in the README rather than designed away.

## Follow-ups

**Change #0190 (minted, `feat`) — close the build-evidence value gap.** `docket-implement-next` Step 6.5 commits the results file on the feature branch *after* the build gate mints the evidence, so `head_sha` no longer matches the branch HEAD and finalize's skip never fires. The review measured it against this repo's own history: roughly 73% of archived changes carry a results file, so the headline one-run path is inert on the majority path. Not a safety bug — the predicate fails toward running — but a real value gap. The stub records both candidate routes and recommends investigating the cheaper one first (have Step 7 **re-mint** the evidence after the last post-gate commit, preserving exact SHA equality) before relaxing the consumer with an ancestry-plus-path-allowlist exemption.

**This very results file demonstrates the gap.** Committing it moves HEAD past the evidence minted at the gate, so finalize will run the suite for this change rather than skipping it. That is the correct, safe behavior — and the cleanest possible illustration of why #0190 exists.

**Not captured, reported only:** this machine's `.docket.local.yml` carries a stale `agents.claude` mirror with nine rows and a header comment that has been false since change 0168 ("the values in each `agents/docket-*.md` wrapper" — pins moved to the sidecar). It is gitignored and machine-local, so it is not repo work and cannot be a change; but it will shadow the sidecar for the nine agents it names on this machine only.

## Notable plan deviations

All were reported by their task and judged sound at review:

- **Budget numbers** differ from the plan's placeholders throughout — the plan instructed "measure first, then apply the rounding rule", so this is compliance, not drift. `docket-review` landed at 105/800 against measured 96/746.
- **`.docket.example.yml` mirror rows had to land before `build-economy`, not after `build-max`.** `tests/test_docket_example_yml.sh`'s per-harness slice terminates at the `build-max` anchor, so the naive "keep existing ordering" reading left all 18 mirror asserts red. Any future roster addition to that file must land above the build rows.
- **The plan under-counted the count-guard surfaces.** `tests/test_sync_agents.sh` needed eight sites moved, not the two the plan named; `tests/test_docket_example_yml.sh` needed a mirror-coverage floor raise the plan did not list at all.
- **One plan-supplied assert was structurally broken.** `'for r in lean standard deep; do … || exit 1; done'` runs under `eval` in the current shell, so the bare `exit 1` terminated the whole test run at the first failure, silently dropping every later assert. Wrapped in a subshell; semantics identical.
- **The `location` finding field deviates from the spec** (`path:symbol` plus a verbatim-quoted clause, rather than the spec's `file:line`), following AGENTS.md's cross-reference rule. Kept deliberately — line numbers rot fastest in exactly the prose-heavy files this reviewer reads — with the reasoning now stated inline so a later reader does not revert it.
