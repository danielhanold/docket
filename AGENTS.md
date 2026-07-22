# AGENTS.md — always-in-context rules for this repo

Rules that must fire **unprompted**. This file is the graduation destination for docket's learnings
findings: when a lesson passes the tiering criterion — *"will the agent know to search for this?"* —
a human promotes it here and flips its finding to `promotion_state: promoted`. Everything that is a
*war story* rather than a *rule* stays in `docs/changes/learnings/` on the `docket` branch and is
pulled by relevance, not loaded here.

Promotion is human-gated by construction: the harvest proposes (`candidate`), a human disposes. See
the docket-convention skill's *Learnings ledger* section for the full promotion mechanics — beyond
the tiering criterion above (named here only because it defines what "graduation destination"
means), this file does not restate them.

## Shell

- Never `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`) under `set -o pipefail`
  — the producer takes SIGPIPE and the 141 becomes an intermittent failure. Capture into a variable
  first, then `grep <<<"$var"`.
- `grep` for a pattern that leads with `--` must declare it: `grep -E -e "<pat>"` or
  `grep -qF -- "<pat>"`. A bare leading `--` is parsed as an option (exit 2) — and inside a negated
  assert (`! grep …`), that error inverts into a permanently green, vacuous guard.
- awk indent classes are `[^[:space:]]`, never `[^ ]` — a literal-space class silently drops
  tab-indented input.

## Frontmatter and generated blocks

- Anchor a frontmatter-field edit to the first `---…---` block, never a bare column-0 line match:
  docket's own change/ADR files discuss `status:`/`updated:` in body prose.
- Quote any hand-authored YAML scalar carrying a colon-space or a boolean keyword
  (`on/off/yes/no/true/false`). Today's grep/awk reader tolerating it is not evidence it is
  well-formed.
- Before rewriting a marker-delimited managed block, validate marker **order and balance** — refuse
  on dangling/out-of-order/nested markers and leave the file untouched. Presence alone is not
  enough; an unbounded range consumes to EOF and eats the user's content.

## Guards and tests

- A guard is code: mutation-test it — strip the thing it guards, watch it redden — or it is
  decoration. A mutation that leaves an assert green is a defect until proven otherwise.
- Key a guard on syntactic **shape**, never an enumerated list of spellings. The spelling you miss
  is the target file's own house idiom.
- Never hand-list the sites of a literal or an operation you are gating — derive them from a
  whole-repo grep, then sort them into prose vs executable. Only the executable ones can violate a
  gate, and a docs-shaped reading skips right past them.
- Run the whole suite at the build gate, never only the tests the spec enumerated.

## Comments and cross-references

- A cross-reference in maintained source anchors on a **symbol name** or a **verbatim-quoted
  clause** — never on a line number. A quoted clause is greppable, so drift is mechanically
  visible; a line number is checkable by nothing, and rots fastest in exactly the files that move
  most. `tests/test_comment_anchor_style.sh` rejects the filename-plus-line-number form; the bare
  colon-number and prose "line N" forms are unenforceable without false positives and rest on this
  rule (ADR-0054).
- This binds maintained source only. Point-in-time records — results files, archived changes,
  specs, and Accepted ADRs — keep whatever pointer was true when written; rewriting them falsifies
  history.
