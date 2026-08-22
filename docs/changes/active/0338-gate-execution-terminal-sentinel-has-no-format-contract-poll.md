---
id: 338
slug: gate-execution-terminal-sentinel-has-no-format-contract-poll
title: 'Gate-execution terminal sentinel has no format contract — poll grepping JSON never matches the plain-text state: line and spins forever'
status: proposed
priority: medium
type: fix
created: 2026-08-22
updated: 2026-08-22
depends_on: []
stacked_on:
related: []
discovered_from: [337]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — observed live during the change 0337 `docket-implement-next` run (2026-08-22). The
dispatched run reached the build gate, started the suite correctly, and then its
gate-observation poll spun indefinitely: the run notified its parent three times with "still
waiting / blocking on the terminal notification" while making no new commits, reading exactly like
the ADR-0024 self-notification deadlock. It was not that deadlock. Child-agent notifications
(status, plan-writer, the four build workers, the reviewer) all arrived and were acted on, and the
gate itself was healthy and passed twice. The poll matched **nothing** because it grepped for a
JSON-shaped terminal sentinel (`"state":"..."`) while the actual observation surface emits the
state as a **plain-text** line (`state: running`, `state: <terminal>`). The pattern never matched,
so the loop never exited, and the run only advanced after a human resumed it and told it to stop
arming the poll.

**Root cause** — `docket-build`'s *Gate execution posture* (`skills/docket-build/SKILL.md`, and its
`references/gate-execution.md` Capability 5) requires the gate to "record an unambiguous terminal
result" and defines the four-state vocabulary a caller keys on, but it does **not** specify the
serialization/format of that terminal sentinel. With the format left implicit, the worker that
improvises the poll and the surface that emits the state can drift on shape (JSON vs. plain-text
`key: value`), and the mismatch is silent: a poll that never matches is indistinguishable from a
gate that never finishes. The failure mode masquerades as a wedge / false completion and costs
human resumes to break.

**Distinct from adjacent work** — this is NOT change 0264 (which measures the *forked-mode launch
shape* that lets the gate survive being detached) and NOT ADR-0024 (unreceivable self-notification).
Both of those were the leading hypotheses during the incident and both were wrong. The defect here
is a missing **format contract** for the terminal-state sentinel that the observation poll parses:
the state vocabulary is specified, the state *shape* is not.

## What changes

Pin the terminal-state sentinel's format as part of the gate-execution contract so the emitting
surface and the observing poll cannot drift:

- Specify the exact serialization of the terminal sentinel (the `state:` signal the poll keys on)
  in `skills/docket-build/references/gate-execution.md` alongside the existing four-state vocabulary
  — one canonical shape, named, so both sides parse the same thing.
- Make the observation poll's match key on that specified shape (and, ideally, fail loudly on an
  **unrecognized** state line rather than treating "no match yet" as "still running" forever — an
  unparseable sentinel is a defect, not a poll-again condition).
- Add a guard/regression that a plain-text `state: <terminal>` line is recognized as terminal, so a
  future reshape of the emitter (e.g. to JSON) reddens a test instead of silently re-introducing the
  infinite poll.

Boundary: contract + poll-parser + guard only. No change to the four-state vocabulary itself, no
change to the launch-shape question owned by 0264, and no change to the notification mechanics.

## Auto-groom blocked

### 2026-08-22

`docket-auto-groom` abstained. The default-biased designer pass and the adversarial critic
(independently, from source) agree that this stub cannot be safely groomed autonomously: the
critic returned **needs human context** on three load-bearing decisions, and a spec may be emitted
only when every decision in it is safe to auto-commit.

**The stub's stated root cause is disproved against source.** The stub asserts the terminal-state
sentinel "has no format contract" and proposes adding one to
`skills/docket-build/references/gate-execution.md`. Source says otherwise:

- The serialization is already pinned — in `scripts/gate-run.md`, not `gate-execution.md`.
  `gate-run.md`'s exit table and *The caller's loop* fix the shape as one line, `state=<state>`
  (an `=`-delimited `key=value`, e.g. `state=running`). `gate-execution.md` Capability 5 explicitly
  *delegates* the state vocabulary to `scripts/gate-run.md` — so the stub's proposed fix location is
  the wrong file, and what it proposes to add already exists in the right one.
- The fail-closed-on-unknown behavior the stub asks for already exists: the canonical loop's
  unknown-line arm is `*) state=unavailable; break ;;`, annotated "NEVER a retry arm … A retry
  there is precisely the shape that never terminates" — exactly the spin the incident hit.
- A mutation-reddened regression already exists: `tests/test_gate_run.sh` asserts `state=running` /
  `state=passed` / `state=failed` / `state=died cause=…` and the malformed-record → `state=unavailable`
  fail-closed path.
- All of this landed in change **0286 on 2026-08-10** (commit `53d4d32c`), merged to `main`
  **12 days before** the 0337 incident of 2026-08-22 — so it was live during the incident.
- Aside: the stub's own symptom description even misreports the surface it claims lacks a contract —
  it says the emitter printed `state: running` (colon-space), but the real contract is
  `state=running` (equals). That only strengthens the disproof.

**The genuine defect the incident exposes is different — and it needs a human.** Two gate
implementations ship with two serializations, reconciled only by a prose sentence in
`skills/docket-build/SKILL.md` (~lines 274-289): the `gate-run.sh` shell facade emits `state=<name>`,
while the landed Go-v1 native gate (`docket gate observe`, `internal/app/gate.go`) emits
**protocol-v1 JSON** with a `"state":"…"` field (confirmed by `internal/cli/gate_test.go`). The SKILL
says the caller loop keys on `state=<name>` "not the native JSON" — but the incident happened *despite*
that prose being live, which is evidence that more prose is not the fix.

**Undecidable decisions (why a human is required):**

1. **Which serialization is canonical / whether to converge the two** (native JSON vs shell
   `state=<name>`). protocol-v1 JSON is the docket-wide operation-document protocol with an installed
   base; retiring or reshaping it is an architecture decision no default-biased groom may take.
2. **Scope: markdown-only vs Go runtime.** The stub scopes "contract + poll-parser + guard, markdown
   only," but any fix with teeth touches `internal/app/gate.go` (or the caller-loop parser) — outside
   the stub's boundary and outside auto-groom's markdown-only write scope.
3. **Disposition: build vs re-scope vs kill.** The likely honest outcome is kill or re-scope, and
   kill/defer are never autonomous.

**What a human should supply, and the recommendation:**

- **Strongly consider killing this stub as-written.** The `state=<name>` format contract, the
  fail-closed canonical loop, and its regression guard already exist (change 0286). If the intent
  was "pin the format + fail loud on unknown + add a guard," that is already done.
- **If a real defect remains, re-title and re-scope it** to the two-serialization seam (shell
  `state=<name>` vs native protocol-v1 JSON), decide which serialization is canonical, and expect a
  **Go-runtime** change (converge the native `observe` output onto `state=<name>`, or give the caller
  loop a defined JSON arm, or retire one implementation) — not the markdown-only contract edit the
  stub currently describes.
- To re-arm autonomous grooming after supplying that context, a human flips `auto_groomable` back to
  `true` and deletes this `## Auto-groom blocked` section.

**Couplings.** No *open* dependency or file-collision coupling was found, so no `depends_on:` /
`related:` frontmatter edit was made. Change **0286** (the canonical loop / format contract this stub
duplicates) is already merged — a merged change is never a `depends_on` gate. Change **0264**
(forked-mode launch shape) is named by the stub only as an adjacency it is explicitly *not*, not as a
coupling.
