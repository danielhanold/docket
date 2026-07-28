<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0133 — Centralize shared Bash runtime configuration helpers](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0133-centralize-runtime-config-helpers.md)**
<!-- docket:backlink:end -->

# Centralize shared Bash runtime configuration helpers — results
Change: #0133 · Branch: feat/centralize-runtime-config-helpers · PR: https://github.com/danielhanold/docket/pull/134 · Plan: docs/superpowers/plans/2026-07-28-centralize-runtime-config-helpers.md · ADRs: none

## Verify (human)

- [ ] Run `bash install.sh` once on this machine against a real `~/.config/docket/config.yml`. The
      whole bootstrap chain now sources `scripts/lib/docket-runtime.sh` under the *system* Bash
      before `DOCKET_BASH_PATH` exists. The suite proves this under a real 3.2.57 with fixtures; a
      single live install is the one thing fixtures cannot stand in for.

## Findings

**No new ADR.** The spec's own judgment — the runtime and configuration boundaries are unchanged,
so this is maintenance on change 0132 — was re-confirmed at reconcile and held through the build.
The one design decision worth naming, the reason-token boundary below, is already recorded as a
spec decision ("the library centralizes reusable mechanics only; authority, discovery, writes, and
diagnostics remain caller-owned policy").

**The reason-token boundary is what let the callers keep their diagnostics.** The two validator
copies differed in a way that a naive `return 0/1` helper would have flattened:
`scripts/docket-config.sh` builds **five** distinct user-facing messages, and
`scripts/ensure-global-config.sh` builds **one** for every failure mode. So
`docket_runtime_validate_bash` returns a machine-readable token (`not-absolute`,
`not-executable`, `no-version`, `not-gnu-bash`, `old-major`, `ok`) plus the binary's version line;
the resolver dispatches on it, the installer discards it with `>/dev/null`. `old-major`
deliberately covers both an unparseable major and a major below 4, because the resolver already
collapsed those two into one die — splitting them in the library would have invented a distinction
no caller makes. All five resolver messages were live-exercised against real fixtures and confirmed
byte-identical to the pre-refactor text.

**A real bug in the plan's own supplied test code, caught by the implementer.** The plan's `probe()`
helper used `out="$(cmd; printf 'x')"; rc=$?`, which captures `printf`'s status, not the target
function's — so `rc` was always 0 and every rc assertion in the validator block was vacuous. The
implementer diagnosed it, fixed the **test** only, and left the library exactly as specified; the
reviewer independently reproduced the diagnosis and confirmed the library had been correct all
along. Worth recording that the *same* `printf 'x'` idiom appears in the shipped resolver code and
is correct there — `scripts/docket-config.sh` branches on the reason token, never on `$?`, so the
idiom is doing only the job it is good at (guarding an empty second line from being swallowed by
command substitution's trailing-newline strip).

**The whole-branch review caught something no per-task review structurally could.** The library's
header claimed to be "the ONE implementation of docket's `runtime.bash` mechanics" and
`scripts/docket-config.md` said Bash 4+ validation is "delegated to the shared library". Both were
false: `scripts/docket.sh` and `scripts/ensure-docket-env.sh` still carry their own independent
version checks. No single task's diff contained those files, and the de-duplication sweep keyed on
the *parser's* symbol (`function scalar`), which neither file ever had. The claims were narrowed to
what is true; the surviving duplication is now change #0152.

**A fixture that passed by coincidence.** The marker-exclusion tests never actually exercised the
library's own claim that a managed `runtime:` header cannot leak block state past the closing
marker — the fixture's post-marker block carried its own `runtime:` header, so the value would have
been found either way. A distinguishing fixture was added and mutation-confirmed: moving
`managed { next }` below the `in_runtime` rules reddens it.

**Mutation matrix: 6/6 rows reddened, no holes.** Block constraint, duplicate detection, marker
exclusion, the Bash-major check, serializability, and the empty-marker guard. Four of the six redden
through the *callers'* suites as well as the library's own, which is why no assert was added to
`test_docket_config.sh` or `test_ensure_global_config.sh`: the spec asked for those suites to be
preserved and extended, and extension would have been redundant coverage rather than new coverage.
Reviewers independently re-ran M1, M3, M4, M5 and M6 rather than taking the matrix on report.

**Integration check against a base that moved mid-build.** `origin/main` advanced by change 0130
(BSD grep interval portability) while this branch was building, and 0130 added a *new* repo-wide
portability guard that this branch's suite runs predate. Running that guard against this branch
returns zero findings for `scripts/lib/docket-runtime.sh` and `tests/test_docket_runtime_lib.sh`,
and the two changes touch disjoint file sets, so the rebase is clean.

## Follow-ups

- **#0152** (refactor) — consolidate the two surviving hand-rolled GNU Bash 4+ validator copies in
  `scripts/docket.sh` and `scripts/ensure-docket-env.sh`. Not a mechanical extension of this
  change: `docket.sh`'s check sits in a bootstrap prologue that deliberately runs before anything
  is resolved, so whether it *can* source a library there — and whether doing so defeats the
  prologue's purpose — is the actual design question.
- **#0153** (fix) — decide whether the `runtime.bash` leaf match should be depth-anchored. The awk
  pattern matches `bash:` at any depth under `runtime:`, so `runtime: → nested: → bash: /path`
  resolves as a valid declaration. Inherited verbatim from all three pre-existing copies, so
  tightening it here would have been precisely the silent caller-rewrite this change existed to
  avoid. Note the migration wrinkle recorded in the stub: `runtime.bash` is machine-local, so a
  repo-committed change cannot migrate anyone's existing file.
- **Deferred, not filed.** On a missing config file, `install.sh` now yields an empty
  `DOCKET_BASH_PATH` and aborts a few lines later with `: command not found`, where the old inline
  awk aborted immediately on `awk: can't open file`. Both abort under `set -euo pipefail`; only the
  diagnostic degrades, and the path is unreachable because `ensure-global-config.sh` runs first and
  exits non-zero unless it has written or validated the file. Below the bar for its own change —
  cheap to improve inside #0152 if that change touches the area.
