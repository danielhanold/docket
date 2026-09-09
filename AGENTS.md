# AGENTS.md — always-in-context rules for this repo

These rules must fire unprompted — they guide ad hoc agent actions before any repository test
could catch a mistake. Promotion into this file is a human decision; detailed history lives in
the learnings ledger on the `docket` branch, and the docket-convention skill's *Learnings
ledger* section owns the promotion mechanics.

<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->
## Docket agents — dispatch, don't run inline

When a requested Docket workflow has a registered same-name `docket-*` agent, dispatch that agent
instead of running the workflow inline: the agent carries that workflow's dispatch contract, its
skill preload, and whatever model and reasoning effort your config layers pin for it. Your
harness's native agent registry is authoritative for agent names, descriptions, and availability —
this block does not restate it. If no same-name agent is registered, do not invent one; follow the
workflow's own inline or unavailable-capability contract. Dispatch through the harness's native
named-agent dispatch, and pass the request through unchanged, including any change or ADR id.
Never reroute a registered workflow through a shell runner, another harness, a generic agent, or
an inline reconstruction of its contract — a missing registration is a visible capability
failure, not a fallback trigger.

## Run gate — bracket a dispatched implement-next run with the gate facade

A dispatched run that stops early returns a report that reads as success; a completion
notification is the CHILD's claim, not your report. The gate facade owns attribution, durable state,
and retry accounting: never hand-reimplement them, and never infer permission from child prose,
launch shape, timestamps, ids, or exit codes. The `docket` binary is on `PATH`; resolve each
operation below from the capability catalog. If it is missing, the install is broken: surface it,
never rebuild the gate by hand.

1. Before dispatching `docket-implement-next`, run `run.gate-before` with `implement-next`. It prints
   `gate-armed <key> <dispatch-context>`; keep both (it won't survive the next tool call) and copy
   the `<dispatch-context>` into the dispatch prompt. Add `--resume <id>` to arm for
   resuming an already-in-progress change. `gate-unarmed` still lets you dispatch, but keyless
   (step 2's fallback) and can never authorize a re-dispatch.
2. After the run returns, or its completion notification arrives, run `run.gate-verdict`
   with `<key>`; without a key, run it with `--unattributed` plus any change id the notification
   names. Obey the resulting `gate-*` report line exactly, never its exit code or the child's prose.
3. Only `gate-retry-once` authorizes another dispatch: the same `docket-implement-next`, once, for
   the id and unmet conjuncts it names, keeping the same key. `gate-continue <key> run-waiting
   <change-id> <continuation-id> <phase>` is **nonterminal**: the same attempt still owns tracked
   work, so it keeps the same key, spends no retry, and is distinct from `gate-retry-once` (a
   continuation, not a second attempt). On it, resume the existing implement-next agent,
   or dispatch `docket-implement-next` again with the explicit change id, the continuation id, and the
   same key, and run `run.gate-verdict` with `<key>` again. Every `gate-stop` and every
   `gate-observe` forbids re-dispatch; `run-halted` means a human is needed.

### Codex root-coordinator entry

For Codex, description markers select the native launch over the general named-child wording. `[docket launch: root-coordinator]` takes precedence: foreground catalog-resolved `agent.enter` at the caller cwd. Otherwise `[docket worktree: feature]` requires foreground catalog-resolved `agent.enter` with the owning workflow's exact `--worktree`; an unmarked metadata child uses direct native named-agent dispatch.

For any `agent.enter` route: Write a request file containing the user's request unchanged; for implement-next include the unchanged gate dispatch-context token, labeled for `change.claim --gate-context` and gate-drive operations. Preserve resume/continuation ids and gate keys. Pass `--request`, `--role`, the active absolute caller `--cwd`, approval policy, and sandbox; pass the owning workflow's exact `--worktree` explicitly for feature children. Never omit dispatch context.

A shell-tool yield carrying a live task/session identity is a liveness transition, not completion. You must retain that exact task/session identity and collect its terminal exit and final output through the harness-native observation/wait mechanism. Never re-run `agent.enter`, start a second watcher, or return a completion report while the original task remains live or unobserved. Only after terminal output is collected may implement-next run the parent's keyed `run.gate-verdict` and obey its report. Coordinator prose, thread or turn ids, and process exit alone do not prove gate ownership or completion. Do not substitute `codex exec`, another harness, a generic agent, or a parent relay.
<!-- docket:dispatch:end -->

## Rebuild the binary after a merge to main

- Whenever a PR is successfully merged into `main`, rebuild the installed `docket` binary from
  a source tree proven to contain the merge, never blindly. First run the
  `repository.sync-integration` operation (argv resolved from the capability catalog) with
  `--repo-dir /Users/homer/dev/docket --json`; proceed only on disposition `advanced` or
  `already-current` — a `skipped`, `refused`, or `failed` sync leaves the rebuild incomplete.
  Confirm the checkout is clean on `main` with its full HEAD equal to the sync target, and
  prove each merge landed in it: `git merge-base --is-ancestor <landed-commit> HEAD`, where
  `<landed-commit>` is the commit the merge produced on `main` — the rebased or squashed tip,
  never the PR's feature-branch head. A negative answer and a failed probe are different
  outcomes; neither permits the install. Then resolve the `development.install` operation from
  the capability catalog, run it with `--source /Users/homer/dev/docket`, and confirm the
  installed binary's `version` operation reports the same full, clean commit id as that HEAD —
  a fresh timestamp, a short prefix, or an older running process proves nothing. On any failed
  condition, report `binary rebuild incomplete` naming it, keep the merged change done, and
  never stash, reset, or switch branches to force the rebuild — fix the reported source state,
  re-sync, and repeat.
- A merged change that extends the `.docket.yml` schema no longer blocks this: since change
  0392 the install path tolerates unknown configuration keys (surfaced as warnings), so the
  tracked `development.install` reinstall works directly with the pre-schema binary.

## Shell

- Under `set -o pipefail`, never pipe a producer into an early-exiting consumer such as
  `grep -q`, `head`, or `head -n1`: the producer takes SIGPIPE and the 141 becomes an
  intermittent failure. Capture into a variable first, then `grep <<<"$var"`.
- Declare a grep pattern that leads with `--`: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`.
  A bare leading `--` parses as an option (exit 2), and inside a negated assert (`! grep …`)
  that error inverts into a permanently green, vacuous guard.
- awk indent classes are `[^[:space:]]`, never `[^ ]` — a literal-space class silently drops
  tab-indented input.
- Always `mv -f` on install/replace paths: BSD `mv` on an unwritable destination with a tty
  prompts, self-answers `n` at EOF, and exits 0, so `|| die` guards never fire and the write is
  silently lost. `git mv` is excepted — there `-f` means force-overwrite a tracked target.
- Always template `mktemp`, with or without `-d`: `"${TMPDIR:-/tmp}/<name>.XXXXXX"` — bare
  macOS `mktemp` ignores `TMPDIR`. When the temp file must sit beside its destination for a
  same-filesystem atomic rename, template it there instead.

## Frontmatter and generated blocks

- Anchor a frontmatter-field edit to the first `---…---` block, never a bare column-0 line
  match: change/ADR bodies discuss `status:`/`updated:` in prose.
- The docket writer guarantees YAML validity by construction (ADR-0071), single-quoting every
  unsafe scalar at the write boundary — so trust it rather than pre-quoting fields it owns.
  Hand-editing, the trap is the inverse: a flow collection (`depends_on: [3]`, `adrs: [71]`) is
  a sequence, not a scalar; quoting it is a defect that retypes it to a string.
- Before rewriting a marker-delimited managed block by hand, validate marker order and
  balance — refuse on dangling/out-of-order/nested markers and leave the file untouched.
  Presence alone is not enough; an unbounded range consumes to EOF and eats the user's content.

## Guards and tests

- A guard is code: mutation-test it — strip the thing it guards, watch it redden — or it is
  decoration. A mutation that leaves an assert green is a defect until proven otherwise.
- Key a guard on syntactic shape, never an enumerated list of spellings. The spelling you miss
  is the target file's own house idiom.
- Never hand-list the sites of a literal or an operation you are gating — derive them from a
  whole-repo grep, then sort them into prose vs executable; only the executable ones can
  violate a gate.
- Run the whole suite at the build gate, never only the tests the spec enumerated. The BUILD
  gate's command is whatever `build.test_command` resolves to and finalize's is whatever
  `finalize.test_command` resolves to — read each from config, never from a second copy —
  entered from source so the gate tests the exact checkout under review. The Go runner
  (`internal/suiterunner`) is the sole channel; there is no separate Bash oracle.
  `tests/README.md` covers how to run the suite and where a new test belongs.
- Read the budget report even on a green run: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line
  is a screening finding, and a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative
  breach to act on. Neither fails the run by default (a parallel wall-clock number is
  machine-dependent, so a real breach is confirmed serially; see `tests/README.md`), so
  nothing else will catch them for you.

## Comments and cross-references

- A cross-reference in maintained source anchors on a symbol name or a verbatim-quoted
  clause — never a line number, which nothing can check and which rots fastest in the files
  that move most (ADR-0054). `TestCommentAnchorStyle` (`internal/repoguard/anchors_test.go`)
  rejects the filename-plus-line-number form only; the bare colon-number and prose "line N"
  forms rest on this rule.
- This binds maintained source only. Point-in-time records — results files, archived changes,
  specs, and Accepted ADRs — keep whatever pointer was true when written; rewriting them
  falsifies history.
