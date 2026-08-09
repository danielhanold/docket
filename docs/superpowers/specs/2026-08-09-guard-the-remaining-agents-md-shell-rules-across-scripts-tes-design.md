<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0263 — Guard the remaining AGENTS.md Shell rules across scripts, tests, and agent-executed markdown](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0263-guard-the-remaining-agents-md-shell-rules-across-scripts-tes.md)**
<!-- docket:backlink:end -->

# Design: guard the remaining AGENTS.md Shell rules across scripts, tests, and agent-executed markdown (0263)

## Problem

Four `AGENTS.md` `## Shell` rules are enforced by nothing on some or all of the repo's executable
surfaces. Verified at groom time (2026-08-09, `/usr/bin/grep` probes over `git ls-files` minus
`docs/`):

1. **Producer-pipe under pipefail** — `scripts/`/`tests/` `*.sh` is change 0172's remit (its guard
   `tests/test_pipefail_shape.sh` is specced but not yet built). The **agent-executed markdown
   surface** (`scripts/*.md`, `skills/*/SKILL.md`, `skills/*/references/*.md`) is guarded by
   nothing. Probe: the surface is nearly clean today (one prose mention of the shape, no live
   hazard found by the coarse probe) — this leg is mostly guard extension, not sweep.
2. **Leading-`--` grep pattern must be declared** (`-e`/`--`) — unguarded everywhere. Probe: the
   tree is clean today (every found `"--…"` pattern already sits behind `--` or `-e`; the one
   remaining hit is a comment). Guard-only leg.
3. **awk indent classes `[^[:space:]]`, never `[^ ]`** — unguarded everywhere. Probe: one
   awk-program site carries `[^ ]` today (`tests/test_bash_runtime_routing.sh`, inside an awk
   program matching a path token); a handful of non-awk `[^ ]` uses exist in sed/grep patterns and
   are outside the rule as written.
4. **Single-backslash word-boundary spelling** (absorbed from killed 0262) — 0246's class in
   `tests/test_grep_portability.sh` gates only the two-backslash source spelling; the byte-identical
   single-backslash spelling is unguarded, computed at 48 sites across ~10 test files, with known
   live fail-open carriers (`tests/test_docket_metadata_branch.sh:112`,
   `tests/test_cursor_dispatch_rule.sh:38,:93` — pointers true at groom time; the build re-derives
   the site list). The guard's own negative control deliberately reddens when the class is widened,
   forcing this change to revisit the population.

## What this change does

Three guard homes, one per rule class — never one mega-guard, and never a second restatement of a
hazard taxonomy that already has an owner:

### Leg 1 — producer-pipe, markdown surface only (extends 0172's guard)

Extend `tests/test_pipefail_shape.sh` (built by 0172) with the agent-executed markdown walk —
the same three-glob surface `test_bsd_tool_defaults.sh` established (`scripts/*.md`,
`skills/*/SKILL.md`, `skills/*/references/*.md`), carried as a **separately-floored** population
(the `md_scope_files` split precedent: folded into one list, a dead glob hides inside the combined
count). The hazard taxonomy, canonical fixes, and `# pipefail-ok:` exemption token are 0172's and
are reused verbatim, not restated. Markdown-specific handling: the scan is a whole-file line scan
with no fence parsing — exactly the `test_bsd_tool_defaults.sh` design — so prose is tolerated **by
shape**: a defective form named in prose is a bare code span without operands, which an
invocation-shaped predicate (pipe + consumer + operand context) never matches, and whole-line
prose/comment mentions that do carry the shape (e.g. `scripts/docket-status.md:332`'s "never
`health_checks | grep -q`", the surface's one hit today) are dropped by the whole-line-comment/
prose-context filter before the offender check runs. For `*.sh` this change adds nothing on top of
0172 (the recorded remit split). Because 0172 names a guard-lands-last fallback, the build's
reconcile pass re-reads 0172's **built** guard file (name, token, taxonomy placement), never its
spec, before extending it.

**This leg creates a real dependency: `depends_on: [172]`** — the guard file and its taxonomy must
exist before they can be extended.

### Leg 2+3 — leading-`--` and awk indent class (one new guard)

New `tests/test_shell_shape_rules.sh` scanning every tracked path minus `docs/` (the
`test_grep_portability.sh` walk: `git ls-files`, NUL-delimited, no extension filter, docs/ excluded
as immutable record), with two classes:

- **Leading-`--` grep pattern**: a `grep` invocation whose first pattern operand is a quoted string
  beginning with `--` (committing to the rule exactly as AGENTS.md states it — a bare leading `--`
  parses as an option; a single-dash-leading pattern is a different, rarer failure and out of
  scope), with no `--` or `-e` between the flags and the operand. Both quoted forms (`"--…"`,
  `'--…'`) are matched; the compliant spellings (`grep -qF -- "--x"`, `grep -E -e "--x"`) are the
  negative controls. Whole-line comments are dropped before the offender check runs (the
  `test_bsd_tool_defaults.sh` precedent) — required on day one: `tests/test_skill_facade_wiring.sh`
  carries the offender form verbatim in a rationale comment. Population floor on the **compliant**
  population (~117 lines today, e.g. throughout `tests/test_docket_status.sh`) so the predicate
  cannot go vacuous while the offender count sits at zero.
- **awk literal-space class**: `[^ ]` in awk program text. This is a deliberate, owned **widening**
  of the AGENTS.md rule (which names indent classes specifically): indent-vs-token cannot be told
  apart statically, so the guard bans the class in any awk program and routes genuinely
  tab-impossible token classes through a reasoned bless token (`# awk-space-ok: <reason>`, honored
  on the site line or the line above, every skip printed — the visible-skip house rule). Detection
  is awk-invoking lines **plus** a bounded in-program tracker (an awk invocation opening a single
  quote that does not close on its line puts the scan in-program until the closing quote) — a
  line-only scan would be vacuous for the one live site, which sits inside a multi-line awk
  program (`tests/test_bash_runtime_routing.sh`). Heredoc-fed and variable-held awk programs are
  outside the tracker and stated as the guard's boundary (the 0172 guard-boundary precedent: the
  guard never claims more than it enforces). Non-awk `[^ ]` (sed/grep token classes such as
  `scripts/docket-status.sh`'s `RECLAIMABLE_LINE_RE`) stays untouched. The live site is converted
  to `[^[:space:]]` (or token-blessed if the tab-impossible argument genuinely holds).

House obligations for the new file: `/usr/bin/grep`-pinned scan (PATH grep is ugrep here — engine
agreement is never assumed), one scan function per class shared by loop and controls, positive +
negative + boundary controls with runtime-assembled fixtures, self-membership handling (this file
embeds the banned shapes, so it is structurally self-excluded per the
`test_comment_anchor_style.sh` precedent, and says so), population floors, mutation-tested at build
time, `tests/runtime-budgets.tsv` row.

### Leg 4 — single-backslash word boundary (extends test_grep_portability.sh)

Policy decision (the fork 0262/the triage left to grooming): **convert by default, bless by
exception** — widening the class while leaving all 48 sites in place would just relabel the defect,
and blessing all 48 contradicts the change's thesis (the known carriers are fail-open guards on a
stock-macOS PATH today).

- Widen the gated class in `tests/test_grep_portability.sh` from the two-backslash spelling to the
  spelling-neutral single-backslash form (which the file already measures as `ONE_BACKSLASH` and
  matches as a superset), update the negative control that deliberately pins the old limitation,
  and retire the now-redundant computed-count report or repurpose it as the exemption count.
  Two comment-population consequences the widening forces, both handled inside the guard's own
  discipline: (a) the guard is self-included and its header/controls carry the single-backslash
  spelling literally in ~8 comment lines — those extend the file's existing `WB_ESC`/`WB_ONE`
  assembled-spelling discipline (banned bytes are assembled at runtime, never written), exactly as
  the two-backslash class already required of itself; (b) rationale comments elsewhere in the tree
  (e.g. `tests/test_docket_build.sh`'s blessing comments) are handled by a whole-line-comment drop
  before the offender check (the `test_bsd_tool_defaults.sh` precedent — a comment cannot reach
  grep), which the class currently lacks because the two-backslash population had no comment
  mentions.
- Re-derive the site list from the guard's own walk (never the stub's figure), and convert each
  defective site to the explicit-class form the guard's remedy already prescribes:
  `(^|[^[:alnum:]_])` / `([^[:alnum:]_]|$)` in place of leading/trailing `\b` (and the `\<`/`\>`
  equivalents). This is **not** byte-equivalent (a class consumes a character; `\b` is zero-width):
  each conversion preserves the assert's verdict, with `-o`/capture sites and adjacent-match cases
  inspected per site rather than converted blind.
- Sites that are deliberate, comment-blessed PATH-grep idiom (the `tests/test_docket_build.sh`
  family) are exempted via a per-site reasoned token (`# word-boundary-ok: <reason>`, honored on
  the site line or the line above), printed visibly when skipped — the per-site-token route,
  chosen over 0246's `elsewhere_shape_exempt` asserted-exact list because a file-and-count list is
  the enumerated-allowlist shape ADR-0050 rules out for a population this size and rots on every
  reflow.
- The toolchain pin/report question stays #0150's (recorded boundary).

## Verification protocol

- Full suite green before and after; final gate backgrounded (the suite sits at the Bash 600s
  ceiling) with exit-code keying.
- Leg-4 conversions: per converted file, assert names and counts byte-identical; converted patterns
  proven against `/usr/bin/grep` (the engine the defect is about), not only PATH grep.
- Every new/extended class mutation-tested: strip the class, watch the control redden; seed a
  synthetic offender in a fixture, watch the gate redden.
- New guard registers its budgets row or `test_runtime_budgets.sh` goes red.
- Two build obligations from the critic's re-check: (a) leg 1's mutation test seeds a prose-quoted
  **full invocation with operands** into a markdown fixture, forcing the build to implement a real
  line-local prose-context heuristic rather than leaning on the operand-shaped predicate alone;
  (b) leg 3's guard states the `'\''` re-quoting idiom inside a single-quoted awk program as a
  false-negative edge of the tracker (it closes the tracked quote early), recorded in the guard's
  boundary alongside heredoc/variable-held programs.

## Out of scope

- The producer-pipe rule on `*.sh` (0172's remit — the recorded split).
- The `Frontmatter and generated blocks` and `Guards and tests` AGENTS.md sections (stub boundary).
- `docs/` in every walk (immutable point-in-time records; the exclusion
  `test_grep_portability.sh` already documents).
- The toolchain pin/report (#0150).
- The prose-anchor house-pattern work (#0253) — it rewrites two of the same test files leg 4
  touches (`test_docket_build.sh`, `test_docket_review.sh`): a textual collision, orderable either
  way → `related: [253]`, no dependency.

## Assumptions

- **A1 — the markdown producer-pipe leg extends 0172's `test_pipefail_shape.sh`, not 0254's
  `test_bsd_tool_defaults.sh`.** The hazard taxonomy (which consumers exit early, the canonical
  fixes, the `pipefail-ok:` token) lives with the pipefail guard; putting the markdown half in the
  BSD-defaults file would duplicate that taxonomy across two guards that must then agree (the
  duplicated-gate learning). Cost: a real `depends_on: [172]`. Rejected: extend 0254's walk (wrong
  class home, taxonomy duplication); a third standalone pipefail guard (same duplication, worse).
- **A2 — `depends_on: [172]` is recorded even though it delays this change.** The dependency is
  structural (the file must exist), not stylistic. Rejected: building the markdown leg first with
  its own patterns and letting 0172 merge into it later (guarantees a collision and a taxonomy
  fork).
- **A3 — legs 2+3 share one new guard file over the tracked-minus-docs walk.** Same walk, same
  floors-and-controls machinery, two independent classes — the `test_grep_portability.sh`
  "N classes, one walk" precedent. Rejected: one file per rule (duplicated walk plumbing, two
  budget rows for trivial scans); folding them into `test_grep_portability.sh` (that file's
  contract is byte-pattern *portability*; argument parsing and awk semantics are a different class
  family, and the file is already 345 lines).
- **A4 — the awk class is a deliberate widening of the rule (any `[^ ]` in awk program text, not
  only indent classes), scoped to awk and detected via a bounded in-program tracker.** Indent
  intent is not statically distinguishable; the bless token absorbs the widening, and the tracker
  is required because the one live site is inside a multi-line program a line-grep cannot see —
  a line-only guard would be vacuous for exactly its motivating site. Heredoc/variable-held
  programs are a stated boundary, never a silent gap. Rejected: whole-repo `[^ ]` ban (reddens
  sed/grep classes the rule never covered); claiming exact indent-class scoping (unimplementable
  statically — the claim would overstate the guard); line-only scan (vacuous where it matters).
- **A5 — leg 4 policy is convert-by-default, per-site reasoned bless token for deliberate
  PATH-grep idiom.** The known carriers are live fail-open defects; blessing the whole population
  keeps them. The token route (`# word-boundary-ok:`) over 0246's asserted-exact list: 48 sites is
  allowlist territory ADR-0050 forbids, and 0172's `pipefail-ok:` sets the house token precedent.
  Rejected: bless-all with an exact list (keeps the defect, rots on reflow); convert-all with no
  exemption path (forces bad conversions on deliberately engine-dependent sites). The widened
  class additionally requires the comment-population handling stated in the leg (assembled
  spellings in the guard's own header; whole-line-comment drop for rationale comments elsewhere) —
  without it the widening reddens the guard on its own file and on comments documenting the rule.
- **A6 — leg-4 conversions are verdict-preserving but not byte-equivalent, so `-o`/capture and
  adjacent-match sites get per-site inspection.** A class consumes a character where `\b` is
  zero-width; blind conversion can change what `-o` emits or whether adjacent tokens both match.
  Rejected: treating the conversion as mechanical (the exact fail-open direction this family of
  changes exists to close).
- **A7 — probes above are groom-time evidence only; every site list is re-derived at build time
  from the guard's own walk** (the AGENTS.md never-hand-list rule; the stub's line-number pointers
  are recorded as true-at-groom-time). Rejected: freezing the probe results into the plan.
- **A8 — couplings recorded in frontmatter (forward link only): `depends_on: [172]`,
  `related: [262, 253]`.** 253 is a file collision (two shared test files), 262 is provenance of
  leg 4; 0150 and 0254 stay prose-level (boundary and evidence respectively, no open coupling —
  0254 is archived/done). Rejected: prose-only couplings (the owner wants them in fields).
