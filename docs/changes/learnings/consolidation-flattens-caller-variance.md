---
slug: consolidation-flattens-caller-variance
hook: "Restatements across N callers are not pure duplication — diff them against each other before templating, or the shared source silently rewrites the callers that differed."
topics: [refactoring, docs, contracts]
changes: [85, 133, 135, 245]
created: 2026-07-17
updated: 2026-08-08
promotion_state: retained
promoted_to:
---

## Apply
Collapsing prose that N callers each restated is the core move of every slimming round, and it rests
on an assumption worth testing before you make it: that the restatements were **duplication**. Often
one or two carried real per-caller variance — a different posture (must-land vs best-effort,
abort-and-report vs continue), a different gate, a different failure path — and applying one template
literally **rewrites those callers**. It lands looking like a docs edit while being a behavior change.

Before consolidating, diff the restatements **against each other**, not just against your template.
Where they genuinely differ, the shared source must **defer to the caller** ("steps 4–5 follow the
caller") rather than pick one posture and flatten the rest. Where the difference is real, keep both
sentences; the two lines you saved are not worth a silently inverted contract.

Two traps specific to this move:

- **The consolidation you are trusting may already have flattened something.** Check the existing
  shared source against its callers *before* extending it — a prior round's flattening reads as
  settled contract.
- **The sentinel net cannot see this.** Grep anchors pin phrases, not postures; no test fails when a
  best-effort caller starts reading as abort-and-report. This class needs a human diff read of the
  before/after per caller, which is exactly the review step a "purely mechanical" framing invites you
  to skip. See [[guards-are-code]] and [[test-premise-deleted-not-regated]].

## War story
- 2026-07-17 (#85, PR #95) — A behavior-neutral slimming round hit this twice on one file,
  `references/terminal-close-out.md`. (a) The brief's literal single abort-and-report template for
  step 5 would have made `docket-status`'s **merge sweep** read as abort-and-report — a real behavior
  change, caught only because a reviewer traced the sweep's call chain by hand and both posture
  sentences were kept instead. (b) Auditing that fix surfaced a **pre-existing** flattening from the
  earlier 0053 consolidation: step 5 lumped "the two kill callers" into abort-and-report, but
  `docket-implement-next`'s reconcile-kill runs its board pass best-effort per its own skill body —
  contradicting the file's own "Steps 4–5 follow the caller" rule, and shipped undetected because no
  test pinned the wording. The same round's other consolidation (six Board-pass litanies → one
  `--must-land` flag) was safe precisely because the variance moved into a *flag* the callers pass,
  not prose a template overwrites.
- 2026-07-28 (#133, PR #134 — merged) — Two entries: how the flattening was **avoided**, and where
  the sweep still missed copies. (a) The two validator copies differed in their diagnostics —
  `scripts/docket-config.sh` builds **five** distinct user-facing messages, `ensure-global-config.sh`
  builds **one** for every failure mode — so a naive `return 0/1` helper would have flattened the
  richer caller. The shared function instead returns a machine-readable **reason token**
  (`not-absolute`, `not-executable`, `no-version`, `not-gnu-bash`, `old-major`, `ok`) plus the
  version line; the resolver dispatches on it, the installer discards it with `>/dev/null`. Deciding
  what the library owns is the move: reusable *mechanics* centralize; authority, discovery, writes,
  and diagnostics stay caller-owned. `old-major` deliberately merges "unparseable major" and "major
  below 4" because the resolver already collapsed them — do not invent a distinction no caller makes.
  All five messages were live-exercised and confirmed byte-identical to the pre-refactor text.
  (b) **The de-duplication sweep keyed on the parser's own symbol** (`function scalar`), which two
  further copies — in `scripts/docket.sh` and `scripts/ensure-docket-env.sh` — never had, so the
  library's header claim to be "the ONE implementation" and the contract's "delegated to the shared
  library" were both false on merge. No single task's diff contained those files; only the
  whole-branch review could see it. A centralization claim has to be verified against the
  **consumers**, never against the key the sweep happened to search on. Claims narrowed to what is
  true; the surviving duplication became #0152.
- 2026-07-28 (#135, PR #127) — Nine Cursor dispatch-rule fragments looked interchangeable and were
  not: `docket-brainstorm-consultant.md` carries **no `Do NOT` line at all** (it states "It performs
  zero docket operations" instead), and the nine split into two structural families. Applying one
  template literally would have deleted a behavioural constraint while reading as a docs edit. The
  fix was to change only each fragment's dispatch-instruction sentence and then **prove** the
  variance survived: a `--word-diff` over `cursor-rules/` shows zero word deletions across all
  nine. That proof is the move worth copying — on a fan-out edit, assert the *deletion count*, since
  no sentinel can tell you what a template silently overwrote.
- 2026-08-08 (#245, PR #185) — The scope line is the finding, not the merge. `sync-agents.sh`'s three
  named emitters (`emit_codex_toml`, `emit_cursor_md`, `emit_opencode_md`) re-derived `name`,
  `description`, `skills`, and the body with byte-identical `sed`/`awk` — genuine duplication, safely
  collapsed into one `parse_wrapper_source()` setting fixed `WSRC_*` globals. What was **left alone**
  is the lesson: serialization, the per-harness skills-preamble sentence, and the `inherit`/`auto`
  sentinel handling are asymmetric *by design* (codex tests at emit position; cursor and opencode
  normalize up front; claude passes through). Folding those in is the exact regression an earlier
  round's review already caught. Draw the consolidation boundary at the code that is
  character-for-character equal, and treat anything requiring a per-caller conditional as evidence the
  variance is real. The gate was **byte-identity** of generated output, verified three ways — the
  project-level pass, the user-level pass (which reaches the second `emit_for_harness` caller), and a
  fixture over all four combinations of `{model: inherit | real id} × {effort: auto | xhigh}` across
  all four harnesses, running the pre-refactor script against the post-refactor one, stderr included.
  Zero diffs, and the four existing generation suites passed unmodified — a stronger proof than any
  assertion a new test could add.
