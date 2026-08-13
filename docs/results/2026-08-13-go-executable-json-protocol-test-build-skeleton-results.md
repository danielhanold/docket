<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0304 — Go executable, JSON protocol, and test/build skeleton](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-13-0304-go-executable-json-protocol-test-build-skeleton.md)**
<!-- docket:backlink:end -->

# Go executable, JSON protocol, and test/build skeleton — results

Change: #0304 · Branch: feat/go-executable-json-protocol-test-build-skeleton · PR: (see change `pr:` field) · Plan: docs/superpowers/plans/2026-08-13-go-executable-json-protocol-test-build-skeleton.md · ADRs: none

## Verify (human)

- [x] Cold-cache verification run by the human on 2026-08-13 — and it **failed the first run**,
  which turned out to be a real defect rather than the anticipated one-time proxy fetch. On a cold
  module cache `go list` writes `go: downloading github.com/spf13/cobra v1.10.2` to stderr and
  still exits 0; Check 1 captured that stream with `2>&1`, so the download chatter was
  word-split into gofmt's directory arguments and the check reddened with
  `lstat go:: no such file or directory`. Every warm run passed, which is why the build-time gate
  never saw it — the failure was reachable only from a fresh clone or a cold CI image. Fixed on
  this branch by sending `go list` stderr to a file inside the existing scratch dir (diagnostics
  are replayed from it on the failure path). Re-verified with an isolated `GOMODCACHE`/`GOCACHE`:
  cold run green, 6/6 asserts. Guard mutation-tested afterwards — a deliberately unformatted
  `internal/app/zz_mutation_probe.go` reddens Check 1 and names the file, so the fix did not
  hollow the check out.
- [x] Ratified 2026-08-13: keep exit 2. `docket help <unknown-topic>` in HUMAN mode exits 2 with
  `docket: unknown help topic "..."` on stderr, instead of Cobra's default exit-0 prose on stdout.
  The spec's "human help remains Cobra-rendered text on stdout with exit 0" is honored for every
  resolvable topic (`help`, `help version`, `help diagnostic runtime`, `--help`); the unresolvable
  case was aligned with the unknown-command error class. Rationale accepted: a mistyped help topic
  is user error, exit 0 would report success to a calling script for a run that printed nothing,
  and routing every topic through the one error path is what makes the JSON-mode conflict correct
  by construction instead of by a second special case. Observed on the built binary —
  `help version` → stdout, exit 0; `help bogus` → empty stdout, `docket: unknown help topic
  "bogus"` on stderr, exit 2; `--json help bogus` → `json-help-conflict` envelope, exit 2.

## Findings

- Review (docket-review-deep) returned 9 findings — 1 blocker, 4 important, 4 minor — all fixed
  in-branch; none became ADRs (the approved spec is the decision record for this change). The
  blocker: `--json help <unknown-topic>` leaked Cobra usage prose onto the protocol stream at
  exit 0, because Cobra's default help command bypasses `SetHelpFunc` for unresolvable topics.
- Cobra v1.10.2 injects the hidden `__complete`/`__completeNoDesc` commands *before* honoring
  `CompletionOptions.DisableDefaultCmd`; they are now rejected at the CLI boundary ahead of Cobra.
  Anyone adding commands should know the completion machinery is reachable unless fenced.
- pflag accepts `--json=1`/`--json=TRUE` etc.; the mode is now resolved from the Cobra-bound flag
  whenever the parse reached it (`Flag.Changed`), with the bounded three-spelling pre-scan kept
  solely as the parse-failure fallback the spec designed it to be.
- Never capture a command with `2>&1` when the captured value becomes ARGUMENTS rather than a
  message. `go list`, like most fetch-capable tools, writes progress to stderr and still exits 0,
  so the rc check certifies nothing about the contents; the contamination is invisible on every
  warm run and only appears on a cold cache. Route stderr to a file and replay it on the failure
  path. (Found by the human's cold-cache verify, after the whole build ran green on warm caches.)
- Go's test cache does not key on the binary `TestMain` builds — a bare `go test ./cmd/docket/`
  can report `ok (cached)` against a mutated tree. Every mutation probe and manual re-verification
  must use `-count=1`.
- `scripts/run-tests.sh --timings` takes an **output path**; passing the test file after it
  truncates that file and launches the whole suite. It bit two workers on this branch. The stale
  `--timings PATH` phrasing sits verbatim in `tests/runtime-budgets.tsv`'s header and
  `tests/test_runtime_budgets.sh`'s comment — see Follow-ups.

## Follow-ups

- `tests/test_bash_runtime_routing.sh`'s whole-repo inventory assert is **cwd-dependent** and fails
  from anywhere but the repo root. It searches an absolute `"$REPO"` while spelling its exclusions
  as `--glob '!tests/**'`; rg resolves a relative glob against the process cwd, not the search root,
  so launching the suite by absolute path from another directory leaves `tests/**` unexcluded and
  the assert drowns in test-file matches. Verified pre-existing — the file is byte-identical to
  `main` and untouched by this change — and confirmed both ways: `-j 1` from the repo root passes
  25/25, the identical invocation from another cwd reddens the one assert. The fix is `!**/tests/**`
  (likewise for the `docs`/`.superpowers` globs), which is cwd-independent. Not fixed here: out of
  scope for 0304, and it belongs with whoever next owns the routing test.
- **Budget margins (record the number, not "did not trip"):** `tests/test_go_toolchain.sh` row is
  20s; measured 12s cold-cache serial, **2s warm** after the fix-loop cache change — 18s of warm
  margin for changes 0305–0318, which all grow this file's compile set. The whole-suite runs on
  this branch reported pre-existing files OVER BUDGET under parallel contention:
  `test_sync_agents_runners` (201s vs 60s ceiling, both runs) and `test_docket_config` (second run
  only). Neither is touched by this branch; change **0280** (shard/re-budget the over-budget
  files) already covers them — no new stub minted.
- The misleading `--timings PATH` sizing-command phrasing in the budget table header and
  `test_runtime_budgets.sh` predates this branch and was left as-is (out of scope for a fix
  worker); worth folding into 0280 or a docs touch-up.
- Human-mode `docket help __complete` still resolves the hidden command and prints its Cobra help
  at exit 0 (JSON mode correctly conflicts). Cosmetic; note for whichever of 0305–0318 next
  touches the CLI adapter.
- `tests/README.md`'s opening "86 standalone Bash files" is stale (117 budget rows now). Pre-existing
  drift, no assert depends on it.
