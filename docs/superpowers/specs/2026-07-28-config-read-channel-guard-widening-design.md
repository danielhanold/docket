<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0146 — Widen the config read-channel guard to the sibling config layers it does not match](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0146-widen-the-config-read-channel-guard-to-the-sibling-config-la.md)**
<!-- docket:backlink:end -->

# Design — Widen the config read-channel guard to the sibling config layers (change 0146)

## Problem

`tests/test_config_read_channel.sh` (change 0120, **on PR #130's branch — not yet on `main`**) is the
prose-side enforcer for ADR-0052: every occurrence of the config filename in `skills/**/*.md` must carry a
same-line class marker (`write-back` or `negative`) or the suite fails.

Two bounded limitations, both named by 0120's whole-branch review:

1. **Sibling config layers are invisible — a genuine fail-open.** The scanned token is exactly `.docket.yml`.
   docket documents two more layers a skill could just as wrongly be instructed to read: the machine-local
   `<repo>/.docket.local.yml` and the user-level `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`.
   Reproduced end-to-end on 0120's branch: with the suite green, appending an unmarked *"Read
   `.docket.local.yml` yourself and parse the `finalize:` block"* line to `skills/docket-status/SKILL.md`
   leaves the suite **PASSing**; the same is true for an unmarked user-level `config.yml` read instruction.
   ADR-0052's rule is about the config *file*, not one of its three filenames.
2. **The occurrence test is a substring match.** `case "$line" in *"$TOKEN"*)` and `grep -oF` count
   `myconfig.docket.yml.bak` as an occurrence.

## Decision

### 1. Widen the token to a three-token set — the fail-open, closed

Replace the single `TOKEN` with a set:

- `.docket.yml`
- `.docket.local.yml`
- `config.yml`

Per line, sum the occurrences across all three tokens and require the same-line marker count to equal that
sum — the existing per-line equal-count rule, unchanged in shape.

**Both match sites must widen.** `scan_tree` has *two*: the per-line prefilter
(`case "$line" in *"$TOKEN"*) ;; *) continue ;;`) and the counting `grep -oF`. Widening only the counter while
leaving the prefilter single-token **preserves the exact fail-open being closed** — a line mentioning only
`.docket.local.yml` never reaches the counter. Fixtures (h)/(i) below would catch it, but the trap is named
here so it is not rediscovered.

**Exactness of summing.** The counts sum *exactly* only if no two tokens can match **overlapping regions** of
a line. (Overlap would only ever *over*-count, never under-count, so this guards precision rather than
safety — but exactness is worth an assert: an over-count demands a phantom marker from an author who did
nothing wrong.) Pairwise non-substring is *necessary but not sufficient* — a future token whose prefix is another
token's suffix would satisfy non-substring and still double-count. The invariant to assert is the
no-co-match/overlap property, and it is additionally pinned by a ground-truth fixture (below) rather than by
the structural assert alone.

### 2. The third token is bare `config.yml`, not `docket/config.yml`

This settles the stub's first open question — and it reverses an earlier draft of this spec, which proposed
the path-qualified `docket/config.yml` on the belief that every occurrence in the tree carries the `docket/`
segment. **That belief is false.** `skills/docket-convention/references/agent-layer.md` refers to the layer
bare, twice — line 23 (``Every one of `config.yml`'s, the repo's committed `.docket.yml`'s…``) and line 86
(``global `config.yml` sets `agent_harnesses:` ``). "The global `config.yml`" is docket's **own house
phrasing**, so it is the *likeliest* spelling a future skill author would use. A path-qualified token would
have widened the guard to a layer while missing the way that layer is actually named in-house — the same
class of narrowness this change exists to fix.

Bare `config.yml` subsumes both spellings, and it restores the summing invariant: `config.yml` is a substring
of `docket/config.yml`, so a set containing both would double-count every path-qualified occurrence.

Cost, accepted deliberately: `config.yml` is a generic filename, so honest prose about some unrelated tool's
config would now require a marker. Bounded by the scanned population — `skills/**` only, docket's own skill
prose, where a `config.yml` mention is overwhelmingly the docket layer — and measured at **zero** occurrences
outside the exclusions today (§5). If a genuine unrelated mention ever appears, it takes one `negative`
marker, which is the design's normal cost, not a defect.

### 3. The three filenames share **one** class-marker vocabulary

The stub's second open question. `write-back` and `negative` describe *what the line says about the file*,
not *which file* — the classes are orthogonal to the layer, and ADR-0052 draws no per-layer distinction
(verified against the ADR text). A machine-local-only class would encode the layer into the marker, forcing
a rename whenever a line's layer changes. One vocabulary; the marker syntax and admissible set stay exactly
as 0120 shipped them (out of scope per the stub).

### 4. The occurrence test is **not** tightened — reasoned rejection

The stub sketches tightening the substring match. Rejected, on two independent grounds (the third, below, is
supporting only):

- **The current behavior is fail-safe.** A superstring match *over*-reports: it demands a marker on a line
  that arguably needs none. It can never admit an unmarked real occurrence, which is the only failure
  ADR-0052 cares about.
- **It is unreachable today.** Verified on 0120's branch:
  `git grep -E '[A-Za-z0-9_-]\.docket\.yml|\.docket\.yml[A-Za-z0-9_-]|\.docket\.yml\.' -- skills` returns
  nothing. There is no superstring occurrence to fix.
- *(Supporting, narrower than it first appears.)* The **obvious** tightening introduces a real fail-open:
  boundary-anchored `grep -oE '(^|[^A-Za-z0-9_.-])\.docket\.yml($|[^A-Za-z0-9_-])'` **consumes the boundary
  character**, so `see .docket.yml .docket.yml here` counts **1**, not 2 (measured under both `grep`
  implementations available here). An undercount makes `markers == occ` satisfiable with fewer markers than
  occurrences — the per-line fail-open 0120's review Finding 1 closed. Two honest caveats: this bites only on
  whitespace/EOL adjacency (the realistic backtick-delimited shape `` `.docket.yml` and `.docket.yml` ``
  still counts 2), and a non-consuming counter (lookaround, or iterative consume) has no such flaw. So this
  ground establishes "*this* tightening is worse", not "tightening is worse" — the first two grounds carry
  the conclusion on their own.

A trailing-boundary rule additionally cannot distinguish `.docket.yml.bak` from a sentence-final
`.docket.yml.` without more machinery than the problem is worth. The limitation is recorded in the test
file's header comment instead, so the next reader does not mistake it for an oversight.

### 5. Audit result — the widening reclassifies nothing

The stub anticipates "the same shape of work 0120 did, at the wider scope." **Verified false.** Every
occurrence of `.docket.local.yml` and `config.yml` in `skills/**` on 0120's branch sits inside the two files
0120 already declared as exclusions:

| file | `.docket.local.yml` | `config.yml` |
|---|---|---|
| `skills/docket-convention/SKILL.md` | 1 | 1 |
| `skills/docket-convention/references/agent-layer.md` | 7 | 4 |

Thirteen occurrences, **all excluded**; zero elsewhere in `skills/**`. So the scanned population gains zero
new occurrences and requires zero new markers. The widening is pure fail-open closure.

Two obligations attach to that result:

- **Re-verify at reconcile.** Both `main` and 0120's branch will have moved (`moving-base`). The count is a
  finding, not a premise the build may lean on. (An earlier draft of this spec stated "five"; the real number
  is thirteen — a wrong count under the word "verified" is precisely what makes a spec untrustworthy.)
- **The zero-cost result is load-bearing on 0120's two exclusions**, which this change puts out of scope. Any
  future narrowing of those exclusions turns this change's cost from zero markers to thirteen. Whoever
  narrows them owns that.

## Test

Fixtures alongside the existing `(a)`–`(g)` set, all driven through `scan_tree` — never a re-implementation
(the file's own stated rule):

- **(h)** an unmarked `.docket.local.yml` occurrence → REJECTED. This is the exact regression reproduced
  above; it must now redden.
- **(i)** an unmarked bare `config.yml` occurrence → REJECTED.
- **(j)** a marked occurrence of each new token → classified `ok`, proving both new tokens reach the
  admissible arm and are not reject-only.
- **(k)** a line carrying **two different tokens** with only one marker → REJECTED. Pins that the equal-count
  rule sums across the token set rather than short-circuiting on the first token that matches.
- **(l) ground truth for summing**: a line containing `.docket.local.yml` **once**, with exactly one marker →
  classified `ok`. This is the direct test that the token set counts it **once, not twice**, and it is what
  actually proves the overlap property rather than asserting a proxy for it.
- **(m)** a line whose only match is `docket/config.yml` → counted **once** (the bare token matches inside the
  path), with one marker → `ok`. Pins decision §2's subsumption.
- **Overlap invariant**: assert directly that no token in the set can co-match an overlapping region of a
  line — not merely that no token is a substring of another, which is necessary but insufficient.
- **Population floors extended**: keep the existing `files >= 10` / `oks >= 4` floors; add a floor asserting
  the token set has three members, so an accidental truncation cannot read as a clean tree
  (`backstop-must-compute-not-reenumerate` — a scan that reaches nothing is byte-identical to green).

## Out of scope

- ADR-0052 itself, and 0120's two declared `docket-convention` exclusions.
- The marker syntax, the admissible class set, and the equal-count rule — all settled by 0120.
- `tests/test_docket_example_yml.sh`, ADR-0052's *key*-side enforcer (see `related: [147]`).

## Assumptions

1. **Widen to a three-token set; keep the per-line equal-count rule unchanged; widen BOTH match sites.**
   Conservative default: the fail-open is in *what is matched*, not in *how it is classified*, so only the
   token set moves. The prefilter/counter split is named explicitly because widening one and not the other
   silently preserves the bug. The summing invariant is the **overlap/co-match** property — pairwise
   non-substring is necessary but not sufficient (a future token whose prefix is another's suffix would pass
   a non-substring assert and still double-count), so the assert states the stronger property and a
   ground-truth fixture backs it. Rejected alternative: a single regex alternation — equivalent in effect,
   but it makes the soundness condition implicit rather than assertable.
2. **The third token is bare `config.yml`, not `docket/config.yml`.** Settles the stub's first open question,
   and **reverses this spec's own earlier draft**, whose justification ("every occurrence carries the
   `docket/` segment") was verified false: `agent-layer.md` lines 23 and 86 use the bare form, making it
   docket's in-house phrasing and therefore the likeliest evasion. Bare `config.yml` subsumes both spellings
   and keeps the token set overlap-free (`config.yml` is a substring of `docket/config.yml`, so a set holding
   both would double-count). Accepted cost: a generic filename now requires a marker in skill prose —
   bounded by the `skills/**` population, measured at zero occurrences outside the exclusions today, and
   remediable with one `negative` marker if a genuine unrelated mention ever appears.
3. **One shared class vocabulary for all three filenames.** Settles the stub's second open question. Verified
   against ADR-0052's text: the classes describe what the line *says*, which is layer-independent, and the
   ADR draws no per-layer distinction. Rejected alternative: a machine-local-only class — encodes the layer
   into the marker and forces renames on layer changes.
4. **Do NOT tighten the substring occurrence test.** Overrides the stub's sketch. Carried by two verified
   grounds — the current behavior over-reports (fail-safe, cannot admit an unmarked occurrence) and no
   superstring occurrence exists in the tree. The boundary-`grep -oE` undercount is real (measured: 1 instead
   of 2 on space-adjacent occurrences) but is **supporting only**, since it bites just on whitespace
   adjacency and a non-consuming counter avoids it — so it argues against *that* tightening, not against
   tightening as a class.
5. **The widening reclassifies nothing — thirteen occurrences, all inside the two exclusions.** Verified by
   counting on 0120's branch (table in §5). Recorded as a finding to re-verify at reconcile, not a premise;
   and its zero cost is explicitly load-bearing on exclusions this change does not own.
6. **`depends_on: [120]` — a build-time impossibility, not a sequencing preference.** Verified
   `git ls-tree main -- tests/test_config_read_channel.sh` is empty: the file **does not exist on `main`**.
   It lives only on `origin/feat/docket-finalize-change-claims-integration-branch-is-read-fro`; 0120 is
   `status: implemented` with `pr: #130` (**not** #128 — that is change 0115's PR; an earlier draft of this
   spec repeated the wrong number from its dispatch brief). Since docket cuts feature branches from
   `origin/<integration_branch>`, this change cannot be built until #130 merges. Unlike an ordinary ordering
   preference, no judgment is involved, so `depends_on` is the correct instrument and the board's
   "waiting on #120 — needs your merge" cell is accurate rather than a self-imposed park.
7. **`related: [147]` — subject overlap on ADR-0052's *other* enforcer.** Verified 0147 exists and matches:
   it widens `tests/test_docket_example_yml.sh`'s `(2c)` orphan-key check past its column-0 anchor —
   different file, different direction (keys, not prose), same ADR, and the same failure shape (a guard whose
   match scope is narrower than the rule it enforces). Neither gates the other; not a file collision.
   Forward link only; 0147's own `related:` is left untouched per the drain's linking rule.
8. **No ADR — but ADR-0052 needs a dated `## Update`.** Verified ADR-0052's `## Decision` is generic
   ("skills read the exported value from the Step-0 `preflight` block, never by parsing `.docket.yml`
   themselves. A model-read of the config file is not a supported shape") and its `## Context` already names
   `.docket.local.yml` and `~/.config/docket/config.yml` as affected layers — so the rule's subject is the
   config, and widening the enforcer to match it is not a new decision. However, ADR-0052's 2026-07-27
   `## Update` describes the enforcer as requiring *every `.docket.yml` occurrence* to carry a marker; that
   record goes stale here. An Accepted ADR is immutable except its status line, so the instrument is a **new
   dated `## Update`**, never an edit — and this change should list `52` in its `adrs:` so the note ships
   with it.
