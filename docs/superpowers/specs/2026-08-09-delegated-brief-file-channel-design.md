<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0277 — Delegated task briefs travel through shell argv, a lossy model-performed transformation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0277-delegated-task-briefs-travel-through-shell-argv-a-lossy-mode.md)**
<!-- docket:backlink:end -->

# Delegated task briefs: a brief-file channel instead of lossy shell argv (change 0277)

Groomed autonomously by docket-auto-groom, 2026-08-09. Every decision below is auditable in
`## Assumptions`.

## Problem

A delegated child's entire input travels as shell argv, transformed by a model in one shot with no
verification. Change 0271 fixed the omission half (the payload slot is now required and emphatic);
this change fixes the mangling half: all three runner adapters interpolate caller arguments as
`$*`, which joins on the first `IFS` character, so a multi-line brief passed as several arguments
is flattened to one line — silently. The current mitigation is an instruction ("pass ONE
single-quoted argument"), which is model discipline, not a mechanism.

## Design

### 1. New channel: `--brief-file <path>` on `runner-dispatch`

`runner-dispatch` (both `--launch` and the legacy foreground verb) gains `--brief-file <path>`:
the caller writes the task brief to a file (heredoc — no shell quoting of the content at all) and
passes its path. Validation at the facade: the file must exist, be readable, and be non-empty.

- **`--launch`**: after minting the dispatch dir, the facade copies the brief to `$DDIR/brief`
  (atomic: `brief.partial` + `mv -f`, matching every other write in the dir) and hands the adapter
  the durable copy's path. This removes any dependence on the caller's temp-file lifetime and
  makes the brief part of the dispatch's audit record, beside `launch`, `stdout.log`, `done`.
- **Legacy foreground verb**: no dispatch dir exists; the caller's file is passed through
  directly. Nothing detaches, so there is no lifetime hazard.

### 2. Mutual exclusion: brief file XOR trailing argv — refuse both

If `--brief-file` is present AND arguments follow `--`, the facade dies before minting anything:
"pass the brief in the file OR after `--`, never both." Preferring either silently drops or
duplicates content; concatenation has an undefined ordering. Refuse is the only shape with no
silent-wrong-answer mode. The adapters enforce the same exclusion defensively (they die too),
covering the direct hand invocation the adapter contracts document, which bypasses the facade.

### 3. Empty-payload gate for build workers — verb-neutral

A `build-*` dispatch carrying neither a brief file nor trailing argv **dies** at the facade with a
diagnostic naming the improvise failure mode. The gate sits at the same **pre-verb** validation
point as the existing `build-*` `--worktree` gate (both `--brief-file` and trailing argv are known
there), so it covers `--launch` and the legacy foreground verb alike — the legacy verb serves
hand invocations, the path most likely to be typed task-less. A build worker without a task is
always the silent-improvise defect 0271 documented, and a loud abort is strictly better than a
successful-looking task-less dispatch. Non-build agents (status, adr, …) legitimately dispatch
payload-free; for them an empty payload stays legal and silent.

### 4. Adapters: `$*` → order- and newline-preserving construction, regardless

All three adapters (`scripts/runners/opencode.sh`, `codex.sh`, `cursor.sh`) replace the `$*`
interpolation with an explicit loop over `"$@"` joining on newline. The argv path stops being
lossy even when it is still used (defense in depth; the single-quoted-argument instruction stops
being load-bearing). When `--brief-file` is set, the prompt instead appends the existing
`Additional caller arguments / task context:` heading followed by the file's contents verbatim
(`cat`, no substitution — the brief is model-authored untrusted input and must never pass through
`eval`, `printf %b`, or a double-quoted expansion that could mangle backslashes or `%`).

### 5. Shim template: teach the two-step, keep 0271's emphatic unbracketed treatment

`sync-agents.sh`'s `emit_shim` STEP 1 becomes two sub-steps:

1. **Write the brief.** One foreground Bash call: `mktemp` with an explicit template
   (`"${TMPDIR:-/tmp}/docket-brief.XXXXXX"`, per the AGENTS.md rule), then a quoted-delimiter
   heredoc (`<<'DOCKET_BRIEF_EOF'`) carrying the caller's task text verbatim. The quoted delimiter
   makes every character literal — no quoting, no escaping, no rewording to make quoting easier.
   If the task text could contain the delimiter line, the model is told to pick a different
   delimiter, not to trim the text.
2. **Launch** with `--brief-file <that path>` rendered **unbracketed**, exactly like the
   `<feature worktree>` slot — a required slot the model must fill, with 0271's failure-mode
   narration carried over: the brief is the ONLY way the child learns what to do; omitting it
   fails silently into improvisation; before sending, confirm `--brief-file` is present and the
   file holds the caller's full task text. The escape hatch stays, reworded for the new channel:
   omit `--brief-file` ONLY when your caller handed you no task text at all (and the facade will
   refuse that for build workers).

The trailing-`--` argv path remains a working mechanism at the facade and adapters but is no
longer taught by the shim: one taught path, and it is the non-lossy one. The single-quoting
paragraph and its `'\''` gymnastics are deleted with it — that instruction existed only to patch
the argv channel's lossiness.

### 6. Brief lifecycle

`$DDIR/brief` inherits the dispatch dir's existing retention: `docket_dispatch_prune` already
bounds growth and removes only terminal dispatches past the retention window. No new mechanism.
Sensitivity is unchanged in kind: the dir already durably holds the child's full stdout, which
quotes the same plan text the brief carries.

## Files touched

- `scripts/runner-dispatch.sh` + `scripts/runner-dispatch.md` — `--brief-file` parsing/validation,
  XOR refuse, `$DDIR/brief` spool, build-* empty-payload gate.
- `scripts/runners/{opencode,codex,cursor}.sh` + their `.md` contracts — `--brief-file` option,
  newline-preserving argv join, defensive XOR.
- `sync-agents.sh` (`emit_shim`) — the two-step template.
- Tests: see below. Regenerated agent wrappers ride the normal sync.

## Testing

- **Facade**: brief spooled into `$DDIR/brief` byte-identical; XOR refusal (both channels → die,
  nothing minted); missing/empty brief file → die; build-* with no payload at all → die;
  non-build with no payload → still launches.
- **Adapters** (fixture-driven, mock binary): a multi-line brief file lands in the prompt
  byte-for-byte; multiple post-`--` args join on newline, order preserved; both channels → die.
  Content fixtures include single quotes, backslashes, `%`, backticks, and a `-- --flag`-shaped
  line (model-authored values are untrusted input).
- **Shim sentinels** (`sync-agents` output): the `--brief-file` slot is unbracketed; the heredoc
  write step is present; the bracketed `[-- <caller args>]` spelling does not reappear
  (mutation-test: re-bracket the slot, watch it redden).
- **Named human verification item** (no in-repo oracle): the harness loads agent definitions at
  process start, so the regenerated shim must be exercised in a FRESH session after sync — one
  live delegated dispatch whose `$DDIR/brief` matches the caller's task text. A same-session
  check proves nothing (proven on 0271).

## Out of scope

- Any change to launch/observe semantics, the run gate, or detachment (0271, frozen).
- Runner config locality (0270) and dispatch anchor gates (0208) — file collision noted, concerns
  disjoint.

## Assumptions

1. **Channel = file path, not stdin.** Stdin does not survive detachment (the launch closes stdin
   by design) without being spooled first — so stdin would just be a hidden file write plus an
   extra relay. A path is explicit, testable, and composes with the detached launch. Rejected:
   stdin (hidden spool, breaks the `</dev/null` detach invariant); keeping argv as the primary
   channel with better instructions (model discipline, not mechanism — the defect as filed).
2. **Spool into `$DDIR/brief` at launch; pass the durable copy.** Removes caller-temp lifetime
   races and gives the dispatch record its input alongside its output. Rejected: passing the
   caller's path straight through (works today because the adapter reads at start, but couples
   correctness to an unstated temp-file lifetime and leaves the audit record input-less).
3. **Both channels present ⇒ refuse.** The stub itself observed refusal is the only option with
   no silent-wrong-answer mode; prefer-one drops content silently, concatenate invents an
   ordering. Enforced at the facade AND defensively in the adapters (direct invocation bypasses
   the facade).
4. **Adapters switch to a newline-preserving `"$@"` join regardless.** Cheap, removes the
   lossiness from the surviving argv path, and makes no behavioral change for today's
   single-argument instruction. Rejected: leaving `$*` because the file path is now preferred —
   the argv path remains reachable, and a reachable lossy path is the defect class this change
   exists to close (fix-reintroduces-its-own-defect-class).
5. **Build-* empty payload dies, verb-neutrally; other agents stay silent.** Mirrors the existing
   build-* `--worktree` gate at its own site: that gate is verb-neutral (pre-verb argument
   validation), so this one is too — scoping it to `--launch` would leave the hand-invocation
   foreground verb free to run a task-less build worker silently. For a build worker, a task-less
   dispatch is always the improvise defect. Rejected: warn-only for build-* (the launch call is
   foreground and the shim does read its stderr, but a warning still lets the improvise run
   proceed — die is the posture that stops it); die-for-all (status/adr legitimately dispatch
   payload-free — a false abort on a working path).
6. **Shim teaches ONE path (heredoc → `--brief-file`), unbracketed and emphatic.** Two taught
   paths would let the model pick the lossy one; a bracketed rendering re-opens 0271's omission
   defect in a new spelling — the stub names this constraint and it is treated as binding. The
   added step count is the change's main risk; it is mitigated by (a) the quoted heredoc removing
   the entire quoting burden that made the one-step form error-prone, (b) the pre-send
   confirmation line, and (c) the facade's hard gate converting a build-worker omission from
   silent to loud — the omission mode 0271 could only narrate is now partly mechanical.
7. **Retention = the dispatch dir's existing prune; no separate cleanup.** A second lifecycle for
   one file in a dir that already has one would be a duplicated gate. The sensitivity argument is
   moot in kind: `stdout.log` already persists the same material.
8. **Dependency state**: `depends_on` stays empty — 0271 is `done` (merged). File collisions with
   active 0208 and 0270 (both edit `scripts/runner-dispatch.sh`, disjoint concerns) are recorded
   as `related:`; whichever lands second reconciles at rebase by intent
   (concurrent-edits-compose-at-rebase).
