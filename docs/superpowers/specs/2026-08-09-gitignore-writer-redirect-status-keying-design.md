<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0272 — De-duplicate the gitignore-block writer's second copy of the write orchestration](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0272-de-duplicate-the-gitignore-block-writer-s-second-copy-of-the.md)**
<!-- docket:backlink:end -->

# De-duplicate the gitignore-block writer's false-success — redirect-status keying (change 0272)

## Problem

`ensure_docket_gitignore_block` in `scripts/lib/docket-gitignore-block.sh` carries its own copy of
the write orchestration that change 0242 corrected in `ensure_managed_block`. Its step-(4) rewrite
(`{ … } > "$gi"`) ignores the redirect's fate, then unconditionally logs
`UPDATED/UPGRADED … — COMMIT THIS`. On an unwritable target — read-only checkout, permissions,
`.gitignore` resolving to a directory, full disk — the caller is told to commit bytes that were
never written, at exactly the moments (migrate, bootstrap, sync) when the user has the least
context to catch it.

## Decision

Give `ensure_docket_gitignore_block`'s write the same redirect-status keying `ensure_managed_block`
got from 0242, expressed in this function's own reporting protocol (a stderr log line, not a stdout
status word):

1. Key step (4) on the redirect: `{ … } > "$gi" || { _docket_gi_log "WARN FAILED to write $gi — the
   target is not writable (see the shell diagnostic above); nothing was changed there. Fix the
   path/permissions and re-run."; return 0; }`. The shell's own `cannot create` diagnostic has
   already reached stderr; the log line owns the human-facing claim.
2. The `UPDATED`/`UPGRADED … COMMIT THIS` lines and the trailing dedup advisory now run only after
   the redirect succeeded.
3. The function still returns 0 on the failure path — matching `ensure_managed_block`'s
   print-`failed`-return-0 posture, and keeping the three `set -e` callers (migrate-to-docket.sh,
   docket-config.sh --bootstrap, sync-agents.sh) from turning a diagnostic into a mid-run abort.
4. Update the stale clause in `ensure_managed_block`'s header comment — "`ensure_docket_gitignore_block`
   above has its own, deliberately un-refactored copy of this orchestration and is unaffected" — to
   record that both `ensure` writers now key on the redirect. Scope the comment to those two:
   `remove_managed_block`'s write (`> "$f"` before printing `removed`) remains unkeyed and outside
   this stub's boundary — a capture candidate, not a claim this change may retire. Comment only;
   the function body is untouched.

## Not chosen: folding onto ensure_managed_block

The stub's first-listed option is structurally blocked by its own boundary. The `.gitignore` writer
must strip the **legacy** `docket:generated` marker pair from the outside-bytes before rewriting;
`ensure_managed_block` computes `rest` from the file itself and knows exactly one marker pair, so a
fold either leaves the legacy block behind (breaking the one-time upgrade) or requires a new
parameter/shape on `ensure_managed_block` — which the boundary forbids ("`ensure_managed_block` is
not reopened"). Two `ensure` writers remain, but both now carry the same one-line keying idiom —
the duplicated hazard the stub names. (`remove_managed_block`'s unkeyed write is a separate,
out-of-boundary residue; see Decision point 4.)

## Tests — `tests/test_docket_gitignore_block.sh` only

- **Unwritable-target fixture**: make `<root>/.gitignore` a **directory** (deterministic — fails
  even for root, unlike `chmod`; `_docket_gi_malformed`'s `[ -f ]` guard treats it as absent, so the
  write is attempted and the redirect fails). Assert: (a) no `COMMIT THIS` / `UPDATED` / `UPGRADED`
  claim in the log, bound to this run's output; (b) the `WARN FAILED to write` line is present
  (pin the mechanism, not just "no success line"); (c) exit status 0.
- **Regression hold**: existing happy-path and idempotence asserts stay green unchanged.
- **Mutation test** (build-time evidence, per AGENTS.md "a guard is code"): remove the `|| { … }`
  keying, confirm both new asserts redden, restore.

## Assumptions

1. **Option B over fold** — chosen because the fold cannot be done inside the stated boundary (see
   "Not chosen"); rejected alternative: widen `ensure_managed_block`'s signature (violates the
   stub's boundary), or pre-strip the legacy block with a first write (two writes, worse failure
   modes).
2. **stderr WARN, not a stdout `failed` word** — a deliberate deviation from the stub's literal
   second option ("a 'failed' word its caller handles"), which the boundary would permit. Grounds:
   all three call sites invoke the function fire-and-forget with stdout uncaptured
   (`migrate-to-docket.sh`, `docket-config.sh --bootstrap`, `sync-agents.sh`), and
   `docket-config.sh`'s stdout is a machine-read stream (preflight parses its `KEY=value` block),
   so an uncaptured status word would leak into machine-parsed output unless every caller grew
   capture code — a larger, riskier diff for zero behavioral gain over the function's existing
   stderr reporting channel, which already carries the success claims being corrected. Rejected:
   stdout word + per-caller capture (above), non-zero return (aborts `set -e` callers
   mid-migration).
3. **`docket-config.sh --bootstrap`'s unconditional "seeded … COMMIT THIS" line stays** — it is a
   caller's own diagnostic, excluded by the boundary. On a failed write the library's WARN now
   appears directly above it, so the user is no longer misled silently. A follow-up may key that
   line; out of scope here.
4. **No atomic-write hardening** — `>` still truncates on open, so a mid-write failure (disk full)
   can leave a truncated `.gitignore`. Accepted for parity: 0242 made the same call for
   `ensure_managed_block`, and hardening one twin here would reopen the other's design. The
   `atomic-generated-write` learning is noted; if pursued, it is its own change covering both
   writers.
5. **Dedup advisory skipped on the failure path** — nothing changed, and the advisory concerns
   outside-bytes the run did not touch; a failed run's log should carry one message, the failure.
6. **Couplings** — none found: no active change touches `docket-gitignore-block.sh` or its test
   file; `discovered_from: [242]` already records provenance, and 0242 is merged, so
   `depends_on:`/`related:` gain no entries.
