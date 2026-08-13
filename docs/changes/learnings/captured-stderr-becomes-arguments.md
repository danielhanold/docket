---
slug: captured-stderr-becomes-arguments
hook: "Never capture a command with 2>&1 when the captured value becomes ARGUMENTS — a fetch-capable tool writes progress to stderr and still exits 0, so the contamination is invisible on every warm run and only appears on a cold cache."
topics: [shell, scripts, caching]
changes: [304]
created: 2026-08-13
updated: 2026-08-13
promotion_state: candidate
promoted_to:
---

## Apply
`out=$(cmd 2>&1)` is the reflex for "capture everything so the failure path can print it". It is
correct when `$out` is destined for a **message**. It is a defect when `$out` is destined to be
**word-split into arguments** — a directory list, a file set, a flag vector. The two uses look
identical at the call site and diverge only under an input the developer never runs.

The trap has three parts, and all three must hold before it fires — which is why it hides:

1. The tool writes **non-error progress to stderr**: `go: downloading …`, `Cloning into …`,
   `Fetching …`, npm/pip/cargo progress. Every fetch-capable tool does this.
2. It **still exits 0**, so an `|| die` or `rc` check certifies nothing about the *contents*.
3. The chatter appears **only on the first run against a cold cache**. Every subsequent run is
   silent, so the local gate, the PR's CI on a warm image, and the whole build loop stay green.

The failure surfaces as a nonsense diagnostic about a filename that is really a fragment of a
sentence (`lstat go:: no such file or directory`), which reads as a broken tool rather than a
broken capture.

**The rule:** decide what the captured value *is* before choosing the redirect.

- Value becomes **arguments** → capture stdout only, and route stderr to a file inside the
  script's existing scratch dir. Replay that file on the failure path so the diagnostics are not
  lost — the reason `2>&1` was reached for in the first place is still owed
  ([[transient-resource-lifecycle]]).
- Value becomes a **message** → `2>&1` is fine.

An exit code is not a validator for a captured *value*; if the value has a shape, check the shape.
And note the ordinary green suite proves nothing here: a warm cache is a different input class
from a cold one, so this belongs on a results file as a named human verify item until a cold-cache
run is part of the gate ([[external-truth-needs-a-human-checkpoint]]).

## War story
- 2026-08-13 (#304, PR #204) — `tests/test_go_toolchain.sh` Check 1 derived the gofmt target set
  from the module itself rather than hand-listing `cmd internal` (itself a review fix), via
  `go list ./... 2>&1`. Every run in the build loop and every reviewer's run was warm, so the
  capture was pure package paths and the check passed — 6/6 asserts, repeatedly. The human's
  cold-cache verify was the first execution against an empty module cache: `go list` emitted
  `go: downloading github.com/spf13/cobra v1.10.2` on stderr, exited 0, and the chatter was
  word-split into gofmt's argument list, reddening the check with
  `lstat go:: no such file or directory`. Fixed by sending `go list` stderr to a file in the
  existing scratch dir and replaying it on the failure path; re-verified with an isolated
  `GOMODCACHE`/`GOCACHE` (cold run green) and then mutation-tested with a deliberately unformatted
  `internal/app/zz_mutation_probe.go` to prove the fix had not hollowed the check out
  ([[guards-are-code]]). The verify item that caught it had been written to certify a *different*
  property — that the gate is offline-capable after the first fetch — and found a real defect on
  the way.
