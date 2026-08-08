---
slug: agent-executed-markdown-is-code
hook: "A `.md` file that carries a runnable command block is executable surface — a repo-wide shell guard that scans only `*.sh` leaves every skill body, script contract, and reference doc unguarded."
topics: [testing, sentinels, shell-portability]
changes: [254]
created: 2026-08-08
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply
docket's normative prose ships bash that an agent runs **verbatim**: the fenced block in a
`SKILL.md`, the copy-paste remedy in a `scripts/<name>.md` contract, the command boundary in a
skill reference. Those bytes reach a shell exactly as a `scripts/*.sh` line does. So any guard that
enforces a shell rule — tool-default hardening, portability, pipefail, quoting — must decide its
surface **by who executes the bytes**, not by file extension.

The practical scope, proven in 0254:

- **In scope**: `scripts/*.md`, `skills/*/SKILL.md`, `skills/*/references/*.md` — prose that
  instructs an agent to run something.
- **Out of scope**: `docs/` — archived changes, plans, and results quote the defective forms
  verbatim *as the subject under discussion*, and the convention forbids rewriting them. This is
  the same `docs/`-exclusion line `tests/test_grep_portability.sh` already draws.

Two costs to accept up front rather than discover mid-review:

- **Prose and invocation can be byte-identical.** In markdown a backticked `` `mktemp` `` naming
  the defect is indistinguishable from a backticked invocation of it. When no *shape* separates
  them, ship the residual gap **named in the guard's own header comment** rather than buying
  coverage with an allowlist (ADR-0050 forbids the allowlist; a bare-word match cost 8 prose false
  positives).
- **The duplicated site is the likely one.** 0254's live hit was a bare `mv` in
  `docket-finalize-change`'s SKILL.md — the same operation `scripts/docket-status.sh` had just been
  hardened for, re-spelled as agent-run bash. Where a script and a skill body do the same job, the
  markdown copy is the one no `*.sh` sweep has ever seen.

Related: [[guards-are-code]], [[shell-portability]], [[enumerated-floor]],
[[fix-reintroduces-its-own-defect-class]].

## War story
- 2026-08-08 (#254, PR #180 — merged) — The change swept BSD tool defaults out of `scripts/`:
  templated every bare `mktemp` (which ignores `TMPDIR` on macOS) and converted 16 bare atomic
  `mv` sites to `mv -f` (bare `mv` self-answers its prompt `n` and exits 0, making every `|| die`
  unreachable). Review finding 3 showed the sweep had missed a **live** site entirely outside the
  shell surface: `docket-finalize-change`'s SKILL.md installs the learnings index with a literal
  bare `mv` in the block an agent executes verbatim — the exact operation just hardened in
  `scripts/docket-status.sh`. The fix widened `tests/test_bsd_tool_defaults.sh` from shell-only to
  51 agent-executed markdown files. Two judgment calls landed with it, both documented in the
  guard's header rather than in a decision record: the `docs/` exclusion, and the named backtick
  gap in the mktemp predicate.
