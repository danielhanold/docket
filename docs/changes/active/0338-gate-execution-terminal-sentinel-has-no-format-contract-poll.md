---
id: 338
slug: gate-execution-terminal-sentinel-has-no-format-contract-poll
title: 'Gate observe ships two serializations (shell state=name vs native protocol-v1 JSON) reconciled only by prose — converge on JSON, migrate the caller loop, retire the text contract'
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-22
updated: '2026-08-23'
depends_on: []
stacked_on:
related: []
discovered_from: [337]
adrs: []
spec: docs/superpowers/specs/2026-08-22-gate-observe-json-convergence-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: 'feat/gate-execution-terminal-sentinel-has-no-format-contract-poll'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-23T04:37:51Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-22-gate-observe-json-convergence-design.md` |
<!-- docket:artifacts:end -->

## Why

**Trigger** — observed live during the change 0337 `docket-implement-next` run (2026-08-22). The
dispatched run reached the build gate, started the suite correctly, and then its
gate-observation poll spun indefinitely, notifying its parent three times with "still waiting"
while making no new commits — reading like the ADR-0024 self-notification deadlock, but it was not
that. The gate itself was healthy and passed twice; the poll matched **nothing** and never exited,
and the run only advanced after a human resumed it.

**The real defect (revised 2026-08-22 — see `## Rescope`).** The gate's "done yet?" observation
ships in **two serializations for one operation**, reconciled only by a sentence of prose:

- The **shell facade** `"${DOCKET_SCRIPTS_DIR}"/docket.sh gate-run --observe` emits a plain-text
  line, `state=<name>` (`=`-delimited, e.g. `state=running`), specified by `scripts/gate-run.md`.
  This is the format the build worker's **canonical caller loop actually drives today**.
- The **landed Go-v1 native gate** `docket gate observe <run-dir>` (`internal/app/gate.go`) emits
  **protocol-v1 JSON** with a `"state":"…"` field (confirmed by `internal/cli/gate_test.go`).

`skills/docket-build/SKILL.md` glues these together with one prose sentence telling the caller to
key on `state=<name>` "not the native JSON." That prose was **live during the 0337 incident and did
not prevent it** — proof that a comment is not a mechanical guard. Nothing stops a worker from
pointing the `state=<name>` loop at the JSON-emitting `observe`, whereupon every answer is
unrecognized, the loop never sees a terminal state, and it polls until the budget is spent.

**Not adjacent work** — NOT change 0264 (forked-mode launch shape) and NOT ADR-0024 (unreceivable
self-notification). Both were the incident's leading hypotheses and both were wrong.

## What changes

**Human design decision (2026-08-22, Daniel): converge on the native protocol-v1 JSON as the single
serialization.** protocol-v1 JSON is the docket-wide operation-document protocol with an installed
base; the plain-text `state=<name>` line is the older, narrower format. So JSON wins and the text
contract retires — rather than dragging the JSON back to `state=<name>`.

Target — one serialization, one caller loop that reads it:

- Migrate the canonical observe loop (`scripts/gate-run.md` § *The caller's loop*, referenced by
  `docket-build/SKILL.md`) to parse the Go gate's protocol-v1 JSON `"state"` field instead of the
  plain-text `state=<name>` line — preserving the existing fail-closed-on-unknown arm (an
  unrecognized/unparseable observation is a defect, never a poll-again condition).
- Retire the `state=<name>` **text observe contract** and update its regression
  (`tests/test_gate_run.sh`) to assert on the JSON shape, so a future reshape reddens a test.
- Delete the reconciling prose sentence in `docket-build/SKILL.md` — the format is now enforced by
  the parser and its test, not by a comment.

**Boundary — a Go-runtime + shell + test change, NOT markdown-only.** In scope: the observe
*output format* and the one loop that consumes it. Explicitly **out of scope / possible follow-up**:
retiring the whole `gate-run.sh` facade. That facade also carries the detached-launch and
liveness-checking machinery it shares with `runner-dispatch.sh` via `scripts/lib/docket-liveness.sh`
(jobs beyond emitting the observe line); converging the observe *format* on JSON does not require
deleting that shared code, and whether to collapse the two launch/liveness paths is a separable
decision. This change closes the serialization seam only.

## Groomed design (2026-08-22)

Groomed with Daniel; detail in the linked spec. Three decisions settled: (1) the caller loop
invokes the native `docket gate observe` directly — no facade passthrough; (2) `gate-run.sh
--observe` is hard-retired in this change (non-zero refusal with a pointer), so the text
serialization dies when this lands — launch/liveness/stop verbs stay for 0339; (3) the loop
parses the JSON with **jq**, which becomes a documented required dependency of the loop (already
a working docket dependency elsewhere); the fail-closed-on-unknown arm survives and also covers
jq-absent and unparseable-document cases.

## Rescope

### 2026-08-22

Rescoped by Daniel after `docket-auto-groom` abstained. The auto-groom's disproof of the original
framing is preserved here because it is the reason for the rescope.

**Original framing (disproved against source).** The stub asserted the terminal sentinel "has no
format contract" and proposed adding one to `references/gate-execution.md`. That was already false:
the `state=<name>` format **is** pinned in `scripts/gate-run.md` (exit table + *The caller's loop*);
the fail-closed-on-unknown arm already exists (`*) state=unavailable; break ;;`); a mutation-reddened
regression already exists (`tests/test_gate_run.sh`); and all of it landed in change **0286**
(2026-08-10, commit `53d4d32c`), merged to `main` 12 days before the 0337 incident. So "pin the
format + fail loud + add a guard" was already done — killing the stub as-written was on the table.

**Why rescope instead of kill.** A real defect survives the disproof: two serializations for one
observe operation (shell `state=<name>` vs native protocol-v1 JSON), reconciled only by prose that
was demonstrably insufficient. The human decision above (JSON canonical, text retired) resolves the
three questions the auto-groom flagged as needing a human — (1) which serialization is canonical:
JSON; (2) markdown-only vs Go-runtime: Go-runtime + shell + test; (3) build vs re-scope vs kill:
re-scope to this. This is now a build-shaped change with a settled direction; it still wants a
brainstorm/spec to nail the exact JSON-parse mechanics in the caller loop before build.

**Couplings.** No *open* dependency coupling. Change **0286** (the canonical loop this builds on) is
merged — never a `depends_on` gate. Change **0264** (forked-mode launch shape) is an explicit
non-adjacency, not a coupling. If the follow-up to retire the `gate-run.sh` facade is ever cut, it
would couple to `runner-dispatch.sh` / `scripts/lib/docket-liveness.sh` — but that is out of scope
here, so no frontmatter link is set.
