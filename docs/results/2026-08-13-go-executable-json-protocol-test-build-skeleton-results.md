<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0304 — Go executable, JSON protocol, and test/build skeleton](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0304-go-executable-json-protocol-test-build-skeleton.md)**
<!-- docket:backlink:end -->

# Go executable, JSON protocol, and test/build skeleton — results

Change: #0304 · Branch: feat/go-executable-json-protocol-test-build-skeleton · PR: (see change `pr:` field) · Plan: docs/superpowers/plans/2026-08-13-go-executable-json-protocol-test-build-skeleton.md · ADRs: none

## Verify (human)

- [ ] On a machine with the Go toolchain but a cold `<git-common-dir>/docket-go-cache/`, run
  `scripts/run-tests.sh -j 1 tests/test_go_toolchain.sh` twice: the first run may reach the module
  proxy (the one permitted network event); the second must pass with zero downloads. The suite is
  offline-capable only after that first warm run — no automated test can prove the offline property
  from inside an online run.
- [ ] Judgment call to ratify: `docket help <unknown-topic>` in HUMAN mode now exits 2 with
  `docket: unknown help topic "..."` on stderr, instead of Cobra's default exit-0 prose on stdout.
  The spec's "human help remains Cobra-rendered text on stdout with exit 0" is honored for every
  resolvable topic (`help`, `help version`, `help diagnostic runtime`, `--help`); the unresolvable
  case was aligned with the unknown-command error class. If you prefer Cobra's exit-0 behavior,
  only the help command's `RunE` error branch and `TestHumanHelpTopics` need to change.

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
- Go's test cache does not key on the binary `TestMain` builds — a bare `go test ./cmd/docket/`
  can report `ok (cached)` against a mutated tree. Every mutation probe and manual re-verification
  must use `-count=1`.
- `scripts/run-tests.sh --timings` takes an **output path**; passing the test file after it
  truncates that file and launches the whole suite. It bit two workers on this branch. The stale
  `--timings PATH` phrasing sits verbatim in `tests/runtime-budgets.tsv`'s header and
  `tests/test_runtime_budgets.sh`'s comment — see Follow-ups.

## Follow-ups

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
