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
- Non-interactive flags on tools that can prompt are load-bearing, not style: BSD `mv` on an
  unwritable destination with a tty prompts, self-answers `n` at EOF, and **exits 0**, so `|| die`
  guards are unreachable and the write is silently lost — always `mv -f` on install/replace paths
  (`git mv` excepted; there `-f` means force-overwrite a tracked target).
- Bare `mktemp` — with or without `-d` — ignores `TMPDIR` on macOS, so a redirect meant to contain
  a script's scratch dir is a no-op. Always pass a template: `"${TMPDIR:-/tmp}/<name>.XXXXXX"`,
  unless the temp file must sit **beside its destination** for a same-filesystem atomic rename, in
  which case template it there instead.

## Frontmatter and generated blocks

- Anchor a frontmatter-field edit to the first `---…---` block, never a bare column-0 line match:
  docket's own change/ADR files discuss `status:`/`updated:` in body prose.
- Quote any YAML scalar carrying a colon-space, a trailing colon, a ` #`, a leading indicator
  character, or a boolean keyword (`on/off/yes/no/true/false`) — whoever writes it, model or
  script. Today's grep/awk reader tolerating it is not evidence it is well-formed. A **script**
  writing free-text prose into frontmatter quotes unconditionally at the write boundary rather
  than predicating on shape, because a conditional is only as good as its enumeration
  (ADR-0071; `mint-stub.sh`'s `title` write is the reference). This rule reaches **scalars only**:
  a flow collection (`depends_on: [3]`, `discovered_from: [234]`, `adrs: [71]`) is not a scalar, so
  the rule does not apply to it and quoting one is a defect — it changes the parsed type from a
  sequence to a string.
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
- Run the whole suite at the build gate, never only the tests the spec enumerated. The suite command is
  whatever `finalize.test_command` resolves to — read it there, never from a second copy. It runs
  the files in parallel with per-job isolation and measures each file against its wall-clock budget.
  A `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding, and a
  `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to act on: neither fails the run by
  default (a parallel wall-clock number is machine-dependent, so a real breach is confirmed serially;
  see `scripts/run-tests.md`), so nothing else will catch them for you. `tests/README.md` covers how
  to run the suite and where a new test belongs.

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

## Rebuild the binary after a merge to main

- Whenever a PR is successfully merged into `main`, rebuild the `docket` binary so the installed
  tool matches source: `docket development install --source /Users/homer/dev/docket`.

<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->
## Docket agents — dispatch, don't run inline

When a requested Docket workflow has a registered same-name `docket-*` agent, dispatch that agent
instead of running the workflow inline: the agent carries that workflow's dispatch contract, its
skill preload, and whatever model and reasoning effort your config layers pin for it. Your
harness's native agent registry is authoritative for agent names, descriptions, and availability —
this block does not restate it. If no same-name agent is registered, do not invent one; follow the
workflow's own inline or unavailable-capability contract. Dispatch through the harness's native
named-agent dispatch, and pass the request through unchanged, including any change or ADR id.

## Run gate — bracket a dispatched implement-next run with the gate facade

A dispatched run that stops early returns a report that reads as success, and a completion
notification is the CHILD's claim, not your report. The gate facade owns attribution, durable
state, and retry accounting — never hand-reimplement them. Docket's helper facade is not on
`PATH`: run each command below verbatim, expansion included.

1. Before dispatching `docket-implement-next`, run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-before implement-next` and keep
   the printed key in your own notes — a shell variable does not survive the next tool call. If it
   prints `gate-unarmed`, you may still dispatch, but the return is keyless (step 2's fallback)
   and can never authorize a re-dispatch.
2. After the run returns — or its detached completion notification arrives — run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-verdict <key>`. Without a key,
   run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-verdict --unattributed`,
   adding any change id the notification names as a trailing hint argument.
3. Obey the facade's `gate-*` report line exactly — never its exit code, and never the child's
   prose.
4. Only `gate-retry-once` authorizes another dispatch: the same `docket-implement-next`, once, for
   the id and unmet conjuncts it names, keeping the same key. Every `gate-stop` and every
   `gate-observe` forbids re-dispatch — `run-halted` means a human is needed, and `run-waiting`
   names a continuation a fresh dispatch would NOT resume: report the handoff id and phase, then
   stop.
5. Never hand-reimplement attribution or infer permission from child prose, launch shape,
   timestamps, ids, or process exit codes.
<!-- docket:dispatch:end -->
