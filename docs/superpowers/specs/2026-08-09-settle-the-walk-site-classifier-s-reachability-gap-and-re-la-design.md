<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0279 — Settle the walk-site classifier's reachability gap and re-land 0258's reverted fixes](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0279-settle-the-walk-site-classifier-s-reachability-gap-and-re-la.md)**
<!-- docket:backlink:end -->

# Settle the walk-site classifier's reachability gap and re-land 0258's reverted fixes — design

Change: #0279 · Groomed autonomously by docket-auto-groom (critic-gated) on 2026-08-09.

## Problem

`tests/test_skip_allowlist_invisibility.sh` limb 2 (section 9) derives every `ls-files` /
`ls-tree` / `find` site under `tests/` and `scripts/` and asks one computed question per site: can
its scope reach a file under `docs/results/`? Change #0258's fourth review fix (`0982b266`) added

```
rp_family="$(git -C "$REPO" ls-files 'tests/test_docket_config*.sh')"
```

to `tests/test_docket_config.sh`, and the site classified **HAZARD** — a third reaching walk
against `EXPECTED_WALK_REACHING=2` — which rolled back all four of #0258's fixes under the fix
loop's all-or-nothing revert rule.

Two classifier behaviors compose into the false positive (verified against the guard's `site()`
function):

1. **Pathspec visibility is keyed on `--`.** The pathspec loop (`for (j …) if (tok[j] != "--")
   continue`) only sees pathspecs after a literal `--`. `0982b266` spelled the call without one,
   so `npos = 0` and the site fell through past the SCOPED check to the HAZARD default.
2. **Any wildcard pathspec is unconditionally reaching.** Even with `--`, the rule
   `if (index(e, "*") > 0) { reach = 1; continue }` treats every glob as able to reach the
   results tree — the literal prefix before the first glob metacharacter is never consulted.
   `tests/test_docket_config*.sh` anchors on `tests/test_docket_config`, so no path it matches
   can begin with `docs/`; the reachability answer is provably NO, but the classifier cannot say
   it.

Gap 1 is fail-closed and correct as designed: an invisible pathspec must not earn a pass. Gap 2 is
the real classification gap this change settles.

## Design

### Leg A — teach the classifier literal-prefix reachability for glob pathspecs

In `site()`'s pathspec loop, replace the unconditional `index(e, "*") > 0 → reach = 1` rule with a
**literal-prefix computation anchored on the RESOLVED pathspec**, never the raw token:

- First resolve the token exactly as the non-glob path does: `r = resolve(e)`. `r == "OTHER"`
  keeps its existing `continue`; `r == ""` from an unresolvable `$` remainder stays **reaching**
  (fail-closed). Only a cleanly resolved pathspec proceeds to the prefix computation. Computing
  from the raw token would let a repovar-prefixed on-chain glob (`-- "$REPO"/docs/res*`) yield the
  bogus literal anchor `$REPO/docs`, fail `onchain()`, and earn a false SCOPED — a fail-open
  regression from today's raw-token `index(e, "*")` rule, which classifies it reaching.
- Compute the resolved pathspec's literal prefix: the substring of `r` up to (excluding) the
  first glob metacharacter (`*`, `?`, `[`).
- Truncate that prefix at its last `/` — the deepest directory the pattern provably anchors in.
  (Truncation is what keeps `docs/res*` reaching: its literal prefix `docs/res` is off-chain as a
  path, but its anchored *directory* is `docs`, which is on-chain, and `docs/res*` matches
  `docs/results`. Only a whole directory component provably constrains the match.)
- If the anchored directory is non-empty and **off the results chain** (fails `onchain()`), the
  pathspec cannot reach the tree — treat it like any other off-chain pathspec (no `reach`).
- Otherwise (empty anchored directory — pattern starts with a glob or anchors only at the repo
  root — or an on-chain anchored directory): **reaching**, exactly as today. The default stays
  fail-closed; only the provable case is carved out.

`onchain()` and the prefix arithmetic already exist and are computed from the literal, so the new
rule inherits the derived-not-spelled property (a retargeted `results_dir` re-aims it
automatically). No new class is introduced: an off-chain glob classifies **SCOPED**, the existing
by-construction shape — deliberately not a new declaration channel (see Assumptions).

Header prose: update the SCOPED class description (limb-2 comment block) to name the
glob-with-off-chain-anchor shape, and extend HONEST LIMITS (f) if the critic-surviving wording
claims less than the mutations prove.

### Leg B — mutation-prove the new rule

Extend section 9's synthetic controls (the `ws1_class`-style table) with, at minimum:

- an off-chain literal-prefix glob (`-- 'tests/test_x*.sh'`) → **SCOPED**;
- an on-chain glob (`-- 'docs/res*'`) → reaching (HAZARD in the synthetic no-declaration run);
- a bare-glob pathspec (`-- '*.md'`) → reaching;
- a glob anchored exactly at the results dir (`-- 'docs/results/*'`) → reaching;
- a repovar-prefixed on-chain glob (`-- "$ROOT"/docs/res*` in a synthetic file where `$ROOT` is
  BASH_SOURCE-derived) → reaching — the false-pass shape the resolve-first anchoring exists to
  deny;
- a glob with an unresolvable `$` remainder after a resolved root (`-- docs/$X*` or
  `-- "$ROOT"/$SUB/res*`) → reaching (the `r == ""` fail-closed arm);
- an unknown-leading-variable glob (`-- "$SOMEWHERE"/x*`) → **SCOPED** — `resolve()` returns
  `OTHER` for a variable not derived from `BASH_SOURCE`, which takes the existing `continue`,
  consistent with existing synthetic control 10 (a walk of another repository classifies SCOPED).
  This is today's behavior, unchanged by this design; the control pins it.

Plus a mutation control in the established style: strip the new prefix rule from a throwaway copy
of the classifier (or run the pre-fix logic) and assert the off-chain glob reddens back to
HAZARD — the guard-is-code rule from AGENTS.md, applied to the guard itself.

### Leg C — re-land #0258's four reverted fixes

Cherry-pick from PR #189's history, in order: `2fa1c162`, `9dad467d`, `7d6e914b`, `0982b266`.
These commits are reachable only via `refs/pull/189/head` (they sit behind revert commits and on
no branch), so the build must `git fetch origin refs/pull/189/head` (or equivalent) before
cherry-picking — a fresh clone will not have them.
Amend `0982b266`'s walk to spell the pathspec **after `--`**:

```
rp_family="$(git -C "$REPO" ls-files -- 'tests/test_docket_config*.sh')"
```

so the classifier can see it and classify it SCOPED under leg A. Each cherry-pick is re-verified by
its original mutation proof (all four were mutation-proved by execution before the revert — see
#0258's results file); conflicts, if any, resolve by intent per the repo's
concurrent-edits-compose-at-rebase learning.

### Budgets

- `EXPECTED_WALK_REACHING` stays **2** — the whole point: the new site is SCOPED.
- `EXPECTED_WALK_FILTERED` / `EXCLUDED` / `WRAPPED` / `DECLARED` unchanged.
- `WALK_FLOOR=80` is a floor; the new site only adds to the population. Not touched unless the
  build-time live measurement says otherwise (floors are measured live, never copied — the file's
  own rule).

## Out of scope

- The skip-allowlist gate in `docket-finalize-change`, `FINALIZE_SKIP_RESULTS_ONLY_DELTA`
  semantics, and limb 1 — untouched.
- The runtime-budget regime for `tests/test_docket_config.sh` — owned by #0251. This change adds
  the four fixes back to that file but does not touch `tests/runtime-budgets.tsv`.
- `find`-site glob handling: `find` pathspecs go through `resolve()`, not the git pathspec loop,
  and no false positive exists there today. Not extended speculatively.

## Coupling

- **#0251** (`related:`) — same test file (`tests/test_docket_config.sh`); both changes are
  corpus-indifferent by prior agreement (0258's spec assumptions 7/9): whichever lands second
  rebases. No `depends_on` — neither blocks the other.
- **#0258** (`discovered_from:`, already set) — done/archived; source of the four cherry-picks.

## Assumptions (autonomous-groom audit trail)

1. **Classifier extension over a declaration channel or a budget bump.** Three options weighed:
   (a) extend SCOPED with literal-prefix reachability for globs; (b) add a cheaper hand-declared
   "narrow walk" table; (c) bump `EXPECTED_WALK_REACHING` to 3 and declare the site. Chose (a):
   the stub itself frames the need as "shape-countable but provably cannot reach" — a *computable*
   property, and the guard's whole design keys passes on computed shape, not enumeration
   (AGENTS.md: "Key a guard on syntactic shape, never an enumerated list"). (b) adds a fourth
   hand-declared pass — the widest-surface option, and the guard's own header prices declaration
   channels as the ones to minimize. (c) admits a non-reaching walk into the reaching budget,
   permanently misclassifying it and re-arguing at every future narrow walk — exactly the
   merge-time argument the stub says must end.
2. **Off-chain anchored-directory globs classify SCOPED, not a new class.** SCOPED is defined as
   "the walk PROVABLY cannot reach" — the literal-prefix argument is a proof of that same
   property, so a new class would split one property across two names and add a budget with no
   discriminating power. Rejected: a distinct `NARROW` class.
3. **Anchor truncation at the last `/`.** A glob's literal prefix constrains the match only up to
   its last completed directory component (`docs/res*` matches `docs/results`). Truncating is the
   conservative reading; the alternative (comparing the raw prefix) would misclassify
   `docs/res*` as off-chain — a real hole. Chose truncation.
4. **The `--`-visibility rule stays as-is; the re-land adapts to it.** Making the classifier parse
   pathspecs without `--` was rejected: a bare operand after `ls-files` is ambiguous with options
   and revs, and widening the parser risks misreading sites that today fail closed. Cheaper and
   safer to spell `--` at the one call site — also better git hygiene.
5. **Glob metacharacter set is `*`, `?`, `[`.** These are the wildmatch specials git pathspecs
   honor; brace expansion is a shell feature, already expanded before git sees the argument. A
   spelling outside the set leaves the pathspec literal, which the existing `resolve()`/`onchain()`
   path already handles.
6. **On-chain, root-anchored, and unresolvable globs stay reaching (fail-closed).** Only a
   provably-off-chain anchor of a **cleanly resolved** pathspec earns SCOPED; every ambiguous
   shape — including a glob whose variable portion cannot be resolved (`r == ""`) — keeps today's
   reaching verdict. The prefix is computed from `resolve(e)`, never the raw token, so a
   repovar-prefixed on-chain glob cannot launder itself through a `$VAR`-polluted literal anchor.
   This preserves the guard's inverted-predicate design: unanticipated shapes land on
   HAZARD/reaching, never on a pass. (Revised after critic round 1, which found the raw-token
   formulation fail-open for exactly that shape. Critic round 2 confirmed the resolve-first
   formulation sound; its one remaining finding — a mislabeled expected class on one Leg B
   control (`$SOMEWHERE` resolves to OTHER → SCOPED, not `r == ""` → reaching) — was corrected
   verbatim per the critic's own prescription, which fully determined the fix from `resolve()`'s
   three return cases.)
7. **All four fixes re-land together.** They were authored as one review-fix arc and reverted as
   one; the first three (`2fa1c162`, `9dad467d`, `7d6e914b`) never broke anything and carry their
   own mutation proofs. Re-landing only `0982b266` would strand three proven fixes for no reason.
   Rejected: splitting into two changes (needless coordination overhead for four small commits in
   one file).
8. **Dependency state.** #0251 is `proposed`/build-ready and has not landed; the family glob
   `tests/test_docket_config*.sh` currently resolves to one file. Both the guard changes and the
   cherry-picks are written corpus-indifferent to #0251's split, so ordering does not matter;
   whichever lands second rebases.
9. **No `tests/runtime-budgets.tsv` change.** #0258's A/B measured its branch at +1.7s on a file
   already over budget on `main`; the retune is #0251's. The re-land re-adds roughly the same
   cost; the `OVER BUDGET:` advisory line, if it appears, is recorded in results and left to
   #0251 — the same disposition #0258 shipped with.
