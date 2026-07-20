# Line-number comment anchors — repo posture, conversion, and guard

Change: 0114 · discovered_from 0106 · 2026-07-20

> Revision 2. The first draft's survey undercounted, one of its four headline staleness findings
> did not hold, and its central "not house style" claim was backwards. All three are corrected
> below; the sweep-not-kill verdict survives on the corrected evidence, re-grounded on rot rate and
> churn alone.

## Verdict

**Not a no-op.** The stub set its own bar — "a survey of one or two argues for closing this; dozens
argues for a sweep." The survey found 27, with a measured rot rate near 15% and every stale anchor
landing in a top-four-churn file. That is the sweep.

But the conversion is **narrower than the first draft proposed**, and the guard is **narrower
still** — it enforces only the one anchor form that can be matched without false positives.

## The survey (evidence)

**27 anchor references across 10 files.**

| File | Refs | Targets |
|---|---|---|
| `scripts/board-checks.sh` | 8 | `render-board.sh` ×6, `docket-status.sh`, `mint-stub.sh` |
| `scripts/docket-config.md` | 4 | `ensure-claude-settings.sh` |
| `scripts/docket-config.sh` | 4 | `ensure-claude-settings.sh` |
| `scripts/github-mirror.md` | 2 | `docket-status.sh`, `docket-config.sh` |
| `.docket.example.yml` | 2 | `docket-config.sh` |
| `tests/test_docket_config.sh` | 2 | `docket-config.sh` |
| `tests/test_board_checks.sh` | 2 | `render-board.sh` |
| `scripts/docket-status.md` | 1 | self-file (`lines 2–19`) |
| `tests/test_finalize_disposition.sh` | 1 | self-file (`line 54`) |
| `docs/adrs/0044-...md` | 1 | `skills/docket-finalize-change/SKILL.md` |

Counting rule (stated, because the first draft applied it inconsistently): **every anchor
reference counts**, including bare continuation forms (`:78`, `:33`/`:38`/`:74`) that share a
referent with a preceding explicit anchor.

**4 of 27 (≈15%) are stale**, each verified against the current tree:

1. `docs/adrs/0044:65` → `skills/docket-finalize-change/SKILL.md:124`. Cited as "the human-present
   close-out"; line 124 is `` `gate == off` → merge trusting the PR's own CI ``.
2. `scripts/github-mirror.md:119` → `docket-status.sh:272`. Cited as where CLI flags populate
   `--project`; line 272 is a bare `}`. The real sites are `docket-status.sh:43` (arg parse) and
   `:318` (the `${PROJECT_FLAG:+--project ...}` invocation).
3. `tests/test_finalize_disposition.sh:80` → "line 54's delegation to the rebase-retest gate";
   line 54 asserts *naming the ids IS the authorization*. The delegation assert is at 86–87.
4. `scripts/board-checks.sh:133` → "the archive table renders from its own pass, `:297+`".
   `render-board.sh:297` is inside the **mermaid** done-node block; the archive section starts at
   `render-board.sh:308`.

**Explicitly not stale:** `.docket.example.yml:62` → `docket-config.sh:353-363`. The first draft
called this rot; it is not. The anchor was written 2026-07-19 (`dab12b0`) and the only subsequent
commit to that file is a rename. The cited range does contain the `ls-tree` probe. Boundary
precision is arguable; drift is not. It is counted as sound.

**Churn on the target files** (commits, last 90 days) — reproduced exactly:

| Target | Commits/90d |
|---|---|
| `skills/docket-finalize-change/SKILL.md` | 54 |
| `scripts/docket-status.sh` | 27 |
| `scripts/docket-config.sh` | 23 |
| `scripts/render-board.sh` | 13 |
| `tests/test_finalize_disposition.sh` | 6 |
| `scripts/mint-stub.sh` | 4 |
| `scripts/ensure-claude-settings.sh` | 1 |

**Three of the four** stale anchors point into the top four; the exception is stale #3, whose
target is `tests/test_finalize_disposition.sh` itself at 6 commits/90d (rank 5). That
correspondence, not the raw count, is the argument: the anchors rot in proportion to how fast their
targets move, and the targets that move fastest are the ones the comments most need to explain.
The one off-pattern case is a *self-file* anchor, where the referring comment and its target move
together in the same commits — a different rot mechanism, and the one the guard does not cover
(see A3).

## What the first draft got wrong about house style

The first draft claimed line-number anchoring "is not the house style" and that the 0106 review's
contrary statement was false. **That was wrong**, and the correction matters enough to record:

- `docs/superpowers/specs/` carries **213** explicit `<file>:<N>` anchors.
- `docs/results/` carries **121**.
- `docs/changes/active/` carries **9**.
- This spec, in its first draft, carried **14** — the document arguing against the idiom was
  written in it.

Line-number anchoring **is** an established repo idiom for pointing at code from prose. The 0106
review was substantially right, and the first draft reached the opposite conclusion only by
surveying a scope that excluded the surfaces where the idiom dominates.

The case for this change therefore does **not** rest on "the house style is something else." It
rests on a narrower and better-supported claim: **in the maintained source surfaces — `scripts/`,
`tests/`, `skills/` — line-number anchors rot, and they rot fastest where the code moves fastest.**
That is true regardless of what the doc surfaces do, and it is the only claim the conversion needs.

**Two rates, kept distinct.** Across the whole survey: 4 stale / 27 refs ≈ **15%**. Across the
**in-scope** surfaces only (which exclude the `docs/adrs/` anchor): 3 stale / 26 refs ≈ **11.5%**.
The in-scope figure is the one that measures the conversion's own target and is the one the verdict
rests on. 26 refs clears the stub's "one or two vs. dozens" bar, and 11.5% is real rot.

## Scope — decided explicitly

The first draft left the largest surfaces undecided. Every surface is now ruled in or out.

**In scope** (convert, and the guard walks it): `scripts/`, `skills/`, `tests/`, `agents/`,
`cursor-rules/`, root `*.md` / `*.yml`. **26 refs across 9 files.**

**Out of scope, structurally:**

- **`docs/adrs/`** — an `Accepted` ADR is immutable except its `status:` line. A guard cannot
  demand a repair the convention forbids. Excluded from both conversion and the guard's walk.
  (This also resolves the first draft's blocking self-contradiction, which required zero matches
  across a surface it simultaneously refused to edit.)
- **`docs/results/`, `docs/changes/archive/`** — immutable historical records. A pointer true at
  authoring time is a correct record; rewriting it falsifies history.
- **`docs/superpowers/specs/`** (213 refs) — point-in-time design records for a specific change,
  read as the design as-of-then. Same category as `results/`. Converting 213 anchors in frozen
  documents is the manufactured work this change should not do.
- **`docs/changes/active/`** (9 refs) — **the suite cannot see it.** These files live on the
  `docket` metadata branch; `git ls-tree origin/main docs/changes/` returns only `archive`. A guard
  running from the integration-branch checkout has no such path to walk
  ([[metadata-branch-invisible-to-suite]]). Not a preference — a hard constraint.

The four stale anchors in scope (#2, #3, #4) are **repointed to what the prose actually means**,
not mechanically re-anchored to whatever line they currently hit. Stale #1 is in `docs/adrs/` and
is handled below.

## The idiom to standardize on

**A cross-reference anchors on a symbol name or a verbatim-quoted clause.**

Compliant, and already present at the repo's best site (`board-checks.sh:99`, which quotes the
target code inline):

```sh
# render-board.sh skips a row whose id is non-integer:
#   `id="$(int_field "$f" id)"; [ -n "$id" ] || continue`
```

The quoted clause is *mechanically checkable* — grep the target, it is there or it is not. A line
number is *structurally uncheckable*: nothing can decide whether line 76 still means what the
comment says. That asymmetry is the decision. It is the rule the learnings ledger already carries —
a sentinel proves a sentence still **exists** and can never prove it is still **true**
([[verify-the-claim]]).

## The guard

`tests/test_comment_anchor_style.sh`, modeled on `tests/test_script_contracts_coverage.sh` (same
family: repo-wide, mechanical, existence-style; the suite is the de-facto gate — no GitHub Actions
CI).

**The guard enforces one pattern only: the explicit-file form `<file>.<ext>:<N>[-<M>]`.**

This is the FP-free predicate, measured across the in-scope surfaces: **13 matching lines, 13 true
anchors, zero false positives.** It covers roughly half the refs and all of the cross-file ones —
the forms that actually rot across file boundaries.

**The two other forms are converted by hand but deliberately NOT guarded**, because neither can be
matched cleanly, measured:

- **Bare `:<N>`** — a naive pattern gives 86 matches for 13 true anchors (**85% FP**). Tightening
  to require a non-digit, non-`/` prefix gives 19 for 13 (**32% FP**) *and* introduces a false
  negative: in `scripts/docket-config.sh:402`, `reading it at :38/:74`, the `/`-guard that
  suppresses URLs also drops the real `:74` anchor. The obvious mitigation — "require the match to
  sit in a comment line" — is falsified by a line inside a file being converted:
  `PATH_STACK=("${PATH_STACK[@]:0:${#PATH_STACK[@]}-1}")   # pop ...` (`board-checks.sh:320`),
  where the `:0:` slice is in the code half of a line a comment heuristic accepts.
- **Prose `line <N>`** — 5 matches, 2 true (**60% FP**). The three FPs in `tests/test_render_board.sh`
  refer to "line 2" of a constructed test fixture: correct, permanent prose that is not a
  cross-file anchor and has no exception path under a no-allowlist rule. The pattern would also
  need to match an **en-dash** (`lines 2–19` in `scripts/docket-status.md:30`).

Accepting a partial guard is the deliberate trade. ADR-0031 already bounds source-syntax scanning
in this repo, and the precedent guard states the same posture in its own header — mechanical
prose-vs-bash checking is "flaky/gameable" and stays out of scope. A guard with a 32–85% false
positive rate would be disabled within a month; a clean one covering the cross-file forms will
survive. The unguarded forms rest on the convention plus review, which is where the repo already
puts claims it cannot mechanically check.

**No allowlist**, and none needed: the in-scope Pattern-1 count is zero after conversion, and the
excluded surfaces are excluded by the walk, not by exception entries. An allowlist answers "is this
one expected?" and never "does this one exist?", and ages into the gap it was written to close
([[correspondence-guard-runs-one-way]], [[enumerated-floor]]).

**Mutation-test in both directions**, and test the guard's **population**, not only its
suppression — a guard iterating an empty file list is green and worthless
([[agent-shell-noop-reads-as-success]], [[backstop-must-compute-not-reenumerate]]). Concretely:
introduce an anchor into a live in-scope file and watch it redden; assert the walked file count is
non-zero and includes a known in-scope file.

**Document the authoring rule in `AGENTS.md`** so it fires unprompted at authoring time — the
guard covers half the forms, so the convention has to carry the rest.

### Build-time traps (found at review; do not rediscover these)

1. **The guard self-matches.** `tests/test_comment_anchor_style.sh` lives in `tests/` and *walks*
   `tests/`, so its own pattern literals redden it. No other guard in this family has the problem —
   `test_skill_facade_wiring.sh` and `test_docket_facade.sh` live in `tests/` but scan `skills/`.
   **Exclude the guard from its own walk structurally** (skip by `BASH_SOURCE` basename), never by
   an allowlist entry. ADR-0030 is the nearest precedent for discriminating structurally.
2. **`AGENTS.md` is inside the walked surface** (root `*.md`), and the spec requires documenting
   the rule there — any *illustrative* non-compliant example in that prose reddens the guard. Write
   the rule without a literal counter-example, or fence the example so the pattern cannot match it.
3. **The new ADR must not itself carry an explicit-file anchor.** It lands in `docs/adrs/`, which
   is unwalked, so nothing would catch it — hygiene, not enforcement, and precisely the failure
   ADR-0044 already committed.

## ADR

**Mint a new ADR** recording the posture: cross-references in maintained source anchor on symbols
or quoted clauses; the guard enforces the explicit-file form only, and why the other two forms are
deliberately unguarded.

The first draft proposed instead to extend ADR-0044 by a dated `## Update`. That was a stretch.
ADR-0044 is *"Autonomy precedence is enforced by pre-specification at the call site"*; its
corollary governs how a **runtime direction to a vendored role skill** is phrased, and its rot
mechanism is *wholesale plugin replacement on upgrade*. Its guard,
`tests/test_skill_handoff_precedence.sh`, derives its population from `$SKILL_*` under `skills/`.
The proposed guard walks `scripts/` and `tests/`. Zero population overlap, different subject,
different mechanism. A comment pointing at `render-board.sh:76` is not a direction to any skill.

The repo mints ADRs for rules narrower than this (ADR-0030, ADR-0032, ADR-0050). A repo-wide
authoring rule plus a new suite guard is squarely the house pattern for a new one. The new ADR
should `relates_to` **ADR-0031** (the bound of source-syntax scanning — directly governs why the
guard is partial) and **ADR-0050** (compute, don't re-enumerate — governs the guard's shape).

**ADR-0044's own stale anchor (#1)** is repaired the one way the convention permits: leave the body
byte-untouched and append a dated `## Update` note restating the corollary with a stable anchor. No
edit to Decision or Corollaries. Its body anchor persists — which is exactly why `docs/adrs/` is
outside the guard's walk.

## Out of scope

- `tests/test_docket_config.sh` beyond its anchor comments — the 0106 fixtures stay
  mutation-verified and byte-identical otherwise.
- Any behavioral change to `scripts/docket-config.sh`.
- `docs/results/`, `docs/changes/archive/`, `docs/superpowers/specs/`, `docs/changes/active/`,
  `docs/adrs/` — see *Scope*.
- Guarding the bare-`:N` and prose-`line N` forms.

## Assumptions

**A1 — This is a conversion, not a kill.** *Chosen:* sweep + partial guard. *Rejected:* abstain
recommending KILL. 27 refs against the stub's own "one or two vs. dozens" bar, ~15% clear rot, and
all four stale anchors landing in top-four-churn targets. *Grounded on rot rate and churn only* —
the first draft's "not house style" argument is withdrawn as false (see above). *Risk if wrong:*
hours spent on comment-only edits. Low.

**A2 — Anchor on symbol/quoted clause; drop line numbers in the in-scope surfaces.** *Rejected:*
the lenient rule (line number retained as a parenthetical convenience). The lenient rule's guard
must judge whether an adjacent quoted clause is "close enough" to a number — fuzzy adjacency
parsing that becomes its own drift surface. Note the first draft justified this by claiming
strictness buys "an exact, zero-exception predicate"; **that justification was false** — the FP
problem lives in the bare and prose forms and is independent of strict-vs-lenient. The choice now
rests on the narrower true claim: for the *explicit-file* form, strict yields a clean predicate.
*Risk if wrong:* bare symbol anchors prove harder to follow. Reversible via a guard-pattern edit.

**A3 — The guard covers Pattern 1 only; the other two forms are converted but unguarded.**
*Rejected:* (a) guarding all three — measured 32–85% and 60% FP rates, no exception path under
no-allowlist, and one unavoidable false negative; (b) guarding none — discards the one predicate
that is provably clean. This is the assumption most likely to draw an objection, and it is a
deliberate acceptance of partial coverage over a guard that would be switched off. *Risk if wrong:*
new bare-`:N` anchors accrete unguarded. Mitigated by the `AGENTS.md` rule; detectable by re-running
this survey later.

*Known limitations, recorded rather than argued away:*

- **"Half the refs" is true of count, not of rot.** Of the four stale anchors, two are
  explicit-form (guarded), one bare (`:297+`), one prose (`line 54`) — the guard catches **half the
  demonstrated rot**, not more. Sharper still: both prose-form anchors are *self-file* references
  and one of the two is stale — ~50% rot density against 3/24 elsewhere. **The highest-density form
  is the one left unguarded.** That is the real cost of A3, and it is accepted only because the
  alternative predicate carries a 60% false-positive rate with no exception path.
- **Bare `:<N>` is largely parasitic on a guarded seed.** It almost always continues a referent
  named by an explicit anchor earlier in the same comment block (`board-checks.sh:102`'s `:78`
  follows `:101`'s explicit form; `docket-config.sh:402`'s `:33`/`:38`/`:74` follow `:401`'s;
  `test_docket_config.sh:1031`'s `:194` follows its own `:201`). So guarding the seed form
  suppresses most continuations without ever matching them. `test_board_checks.sh:597` is a
  counter-example, which makes this a tendency to rely on, not a law to claim.

**A4 — Mint a new ADR rather than extending ADR-0044.** *Rejected:* the `## Update` extension —
zero population overlap with ADR-0044's guard, different subject and rot mechanism. *Risk if
wrong:* one more ADR in a 50-ADR ledger. Negligible.

**A5 — Every doc surface is ruled in or out explicitly.** `docs/adrs/` out (immutability — a guard
cannot demand a forbidden repair); `results/`, `archive/`, `specs/` out (immutable records);
`docs/changes/active/` out (**not visible to the suite** — it lives on the metadata branch and is
absent from `origin/main`). This is the one place the standing "prefer comprehensive" preference is
deliberately not followed, and the reasons are structural rather than discretionary in three of the
four cases. *Risk if wrong:* a reader follows a stale pointer out of a frozen document. Acceptable —
those are read as history.

**A6 — The guard is a new standalone test file.** `tests/test_comment_anchor_style.sh`, matching
the one-file-per-guard-concern family across 52 existing test files, with
`test_script_contracts_coverage.sh` as the model. *Rejected:* folding it into `board-checks.sh`,
which is board-scoped. (The first draft additionally justified this by a claimed concurrent edit;
that premise was false — see A7 — but the scope argument stands alone.) *Risk if wrong:* one more
suite file. Negligible.

**A7 — No conflict with changes 0111 / 0115 / 0116.** The first draft asserted these were
"concurrently editing" `scripts/board-checks.sh` and built a fallback around it. **False:** all
three are `proposed` stubs with empty `spec:` and empty `branch:` — ungroomed and unstarted.
Nothing is in flight in that file. 0114 is comment-only and can land **first**; the first draft's
"convert `board-checks.sh` last" fallback inverted the real ordering advantage and is withdrawn.

**A8 — `depends_on` stays empty.** Nothing gates 0114: comment-only, produces no interface, and a
dependency is satisfied only at `done` — encoding the 0111/0115/0116 overlap would stall an
independent change behind three undesigned ones.
