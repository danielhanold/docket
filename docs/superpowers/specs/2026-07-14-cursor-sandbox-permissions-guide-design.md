# Cursor sandbox & permissions guide — design

- **Change:** 0073 (`cursor-sandbox-permissions-guide`)
- **Date:** 2026-07-14 (interactive groom; decisions settled with the human)
- **Depends on:** 0068 (facade — done), 0072 (skill rewiring — done)
- **Build precondition:** the `## Appendix — verification log` section of THIS file must exist and be
  non-empty before the build starts. See *Provenance* below.

## Problem

Cursor users have no docket-owned guidance for running the skills under Auto-run in Sandbox.
Before change 0068 no such guidance was even possible: docket's Step-0 prose asked agents to compose
`eval`, branching, worktree creation, hook setup, and fetch/pull into multiline shell programs, and
Cursor's classifier demotes an entire program when any single leaf is not allowlisted — so the
permission surface was unboundable. 0068 collapsed docket's agent-facing runtime to one program
behind one canonical spelling, and 0072 rewired every skill and the convention's Step-0 onto it.
The permission configuration is now small, finite, and stable enough to publish.

Two forces shape the guide beyond "write down the config":

1. **The claims rest on observed classifier behavior**, not on documented contract. Cursor's
   auto-run classifier is not specified anywhere docket can cite. Any troubleshooting entry docket
   publishes is an empirical claim about a specific Cursor version at a specific date, and must be
   stamped as such or it is folklore with a shelf life.
2. **Allowlisting the facade is a security decision, not a convenience one.** One permission entry
   authorizes every facade operation — including a sweep that rewrites shared history. The guide
   that hands a user that entry owes them a plain statement of what they just authorized.

## Core decisions

1. **Trust is granted at the facade, not per operation.** The published fragment allowlists the one
   canonical `docket.sh` invocation; every operation runs unprompted. The facade *is* the trust
   boundary (0068's central argument, ADR-0029) — re-litigating that boundary as 13 per-op entries
   would produce a fragment that drifts with every new operation and buys control the facade was
   designed to make unnecessary. The price is that the entry authorizes destructive operations
   unprompted; the guide states that price loudly rather than hiding behind granularity.
   **Recorded as an ADR** (see *ADR* below).
2. **Both fragments ship.** A `permissions.json` fragment (command approval) and a `sandbox.json`
   fragment (a read path to the docket clone, which lives outside the workspace). They are not
   substitutes for one another — see decision 3 and *Cursor Run Mode mapping* below.
3. **The load-bearing claim: docket ops must run OUTSIDE the sandbox.** Docket's runtime needs an
   out-of-workspace program (`$DOCKET_SCRIPTS_DIR/docket.sh`) and the network (`preflight` fetches
   and rebases; skills push). A sandbox denies both. No `sandbox.json` filesystem entry can
   substitute for a terminal permission — the guide says this explicitly, because the natural
   assumption ("I granted filesystem access, why is it still failing") is the failure mode.
   The recommended operator mode is **Auto-review (with Sandbox)** with a
   `permissions.json` `terminalAllowlist` entry for the facade — not "sandbox mode" as an
   alternative to the permissions file (a common misreading of the pre-3.5/3.6 UI).
4. **Nothing is asserted that the verification log does not record.** The troubleshooting section is
   a transcription of the appendix, not a reconstruction from docket's code or from Cursor's docs. A
   failure mode that does not reproduce in the session is **cut**, not softened.
5. **docket never edits the user's Cursor configuration.** Copy-paste, human-applied — the ADR-0020
   posture for generated agent artifacts, applied to permission config.

## Cursor Run Mode mapping (product facts the guide must get right)

Observed in Cursor **3.11.19** (2026-07-14) against the desktop UI, the client logic in
`workbench.*.main.js`, and the public docs
([Run Modes](https://cursor.com/docs/agent/security/run-modes),
[permissions.json](https://cursor.com/docs/reference/permissions)). The public docs understate a
mode-lock interaction that the UI and client enforce.

Settings → Agents → Approvals & Execution exposes four options (Allowlist's optional sandbox is
two rows):

| UI label | Docs mode | Allowlist | Sandbox | Classifier |
|---|---|---|---|---|
| **Allowlist** | Allowlist | Yes | No | No |
| **Allowlist (with Sandbox)** | Allowlist + sandbox | Yes | Yes | No |
| **Auto-review (with Sandbox)** | Auto-review | Yes (when mode available) | Yes | Yes |
| **Run Everything (Unsandboxed)** | Run Everything | N/A | No | No |

**`permissions.json` allowlists lock the mode to Allowlist.** When `~/.cursor/permissions.json`
(or the per-repo file) defines a non-empty `terminalAllowlist` or `mcpAllowlist`, the client sets
`permissionsFileConstrainsUnrestrictedMode` and:

- **Run Everything** is disabled (banner text says this).
- **Auto-review (with Sandbox) is also non-selectable** (UI still shows the row; click does
  nothing / `isDisabled` — tooltip copy says "disabled by your admin" even when the cause is the
  local permissions file). Effective selectable modes: **Allowlist** and **Allowlist (with Sandbox)** only.

This is the operator-visible truth behind the common misreading "sandbox auto-run **or** the
permissions file." Publishing a `terminalAllowlist` fragment without an escape hatch forces
Allowlist modes; it does **not** compose with Auto-review the way the public docs imply.

**Rejected escape hatch (observed 2026-07-14, Cursor 3.11.19):** writing
`approvalMode: "unrestricted"` into `permissions.json` (parsed by the client but absent from the
public field table) emptied the Run Mode dropdown — **no options selectable**. Removed
immediately; do **not** publish or recommend `approvalMode` in the guide. With the key omitted
and non-empty allowlists present, the constrain flag stays on (Allowlist modes only).

**`sandbox.json` is still complementary** for filesystem/network limits of sandboxed commands.
It does not unlock Auto-review and does not substitute for a terminal allowlist entry.

**Guide target mode:** **Allowlist (with Sandbox)** + the facade `terminalAllowlist` fragment +
the sandbox read-path fragment. That is the composition a published allowlist file can actually
select. Auto-review remains available only when the permissions file does **not** define
allowlists (IDE-managed allowlist + optional `autoRun` only) — out of scope for the published
docket fragment. `autoRun` instructions are inert under Allowlist modes.

*Product-context for the guide and verification session — not an ADR. Correct this section from
the verification-log appendix if Cursor changes the lock.*

## Provenance — how the guide earns its claims

The guide's claims about classifier behavior come from a **live Cursor verification session**, run
by the human, **before the build**. The autonomous implementer is a Claude Code subagent: it cannot
drive Cursor's UI, answer an Auto-run prompt, or read a sandbox denial. Verification is therefore
work only the human can do, and the build consumes its output.

**The artifact.** After the session the human appends a `## Appendix — verification log` section to
this spec file, on the `docket` branch, recording:

- the Cursor version and the observation date;
- each command form submitted (canonical guarded-expansion spelling, resolved-absolute path, any
  variant tried);
- what the classifier actually did (auto-ran / prompted / demoted the program / denied);
- each failure mode reproduced, and each that did **not** reproduce.

Being an appendix of the spec rather than a separate file, it needs no new link field, it reaches
the integration branch with the spec at close-out via terminal-publish, and the implementer — which
reads the spec anyway — needs no second artifact.

**The gate.** Change 0073 is left at `status: blocked` (`blocked_by:` naming this appendix) at groom
time, so no autonomous loop can select and build it while the appendix is missing. When the session
is done and the appendix is committed, the human flips `status: blocked` → `proposed`, making the
change build-ready.

**The fail-closed backstop.** The reconcile pass MUST abort (abort-and-report, per the autonomous
wrapper rule) if this spec has no `## Appendix — verification log` heading, or the section is empty.
A premature status flip must fail, never invent observations.

## What ships

All three artifacts are user-facing documentation and live on the integration branch, built on the
feature branch like any code:

| Path | What it is |
|---|---|
| `docs/cursor/permissions.md` | The guide. |
| `docs/cursor/permissions.example.json` | Copyable `permissions.json` fragment: the canonical guarded-expansion spelling **and** the resolved-absolute form (a template — the clone path is machine-specific). |
| `docs/cursor/sandbox.example.json` | Copyable `sandbox.json` fragment: read path to the docket clone. |
| `README.md` | A pointer to the guide (a knob nobody can find is not shipped — LEARNINGS 2026-07-09 #49). |

### Guide outline

1. **The three gates, and why they are independent** — command approval (`permissions.json`),
   filesystem access (`sandbox.json`), network. Plus reload behavior (when a config edit takes
   effect), as observed.
2. **Why docket must run outside the sandbox** — decision 3 above: out-of-workspace program +
   network. The sandbox config is not a substitute for terminal permissions.
3. **The fragments** — both, copy-paste ready, with the machine-specific substitution called out.
4. **Trust tiers** — docket's shell surface, classified:
   - **Daily operations, behind the facade** — allowlisted. The one entry.
   - **The human-initiated tier** — `install.sh`, `migrate-to-docket.sh`, `sync-agents.sh`,
     `ensure-docket-env.sh`, `ensure-claude-settings.sh`: one-time setup and migration tools a human
     runs deliberately. Never allowlisted, never reachable through the facade.
   - **Internals the facade must not expose** — `docket-config.sh`, `disable-worktree-hooks.sh`,
     `render-board.sh`, `lib/`, tests.
   This classification is **derived from `scripts/docket.md`'s Subcommand inventory and Not-exposed
   sections**, which already declare themselves the permission inventory — not hand-copied
   (LEARNINGS 2026-06-12 → 2026-07-13 #64: never hand-list a set; derive it).
5. **The security consequences, stated plainly** — allowlisting the facade authorizes, unprompted:
   `docket-status`'s guarded sweep (archives merged changes, publishes terminal records onto the
   integration branch, deletes merged feature branches and worktrees), `terminal-publish`'s direct
   push to the integration branch, `github-mirror`'s external writes to GitHub Issues/Projects, and
   `cleanup-feature-branch`'s provenance-guarded branch and worktree deletion. These are shared-history
   and external writes, and they are the deal being accepted for one line of config. The guarded/
   provenance-checked nature of each is a mitigation, not a reason to omit the statement.
6. **Why the workarounds are not acceptable** — arbitrary `eval`, blanket `bash`, a bootstrap-command
   prefix, or a generic command-runner subcommand: each erases the trust boundary the facade exists to
   draw, and returns the permission surface to unbounded (0068's rejected alternatives).
7. **Scope statement** — the facade stabilizes docket's own metadata and lifecycle operations.
   Build-time commands in a consuming repo (feature-branch git, the test suite, `gh`) remain that
   repo's own permission surface: documented as such, not hidden and not silently covered by
   docket's fragment.
8. **Troubleshooting** — the observed failure modes, transcribed from the appendix, each stamped with
   the Cursor version and observation date. The candidate list from the stub (to be confirmed,
   corrected, or cut by the session): invalid JSON silently disabling the whole file; spelling
   mismatches in the guarded expansion; one unmatched leaf demoting a compound program; protected
   `.git` writes; network fetches still blocked by `sandbox.json`.

## Verification

`tests/test_cursor_permissions_docs.sh` — every assertion mutation-tested (strip the thing it guards,
watch it redden) before it is trusted; a guard that has not been mutation-tested is decoration
(LEARNINGS, the guards-are-code family):

1. **Both example JSON files parse.** Invalid JSON silently disabling the entire permissions file is
   itself one of the failure modes the guide warns about — shipping a malformed fragment would be
   self-refuting.
2. **The fragment carries the canonical facade spelling**, derived from the same source the facade's
   own tests enforce (`scripts/docket.md` / the canonical-spelling sentinel), never retyped into the
   test. Both published forms — guarded-expansion and resolved-absolute — are checked.
3. **The guide's fenced JSON blocks are byte-identical to the example files**, so the copyable text a
   reader pastes and the file the parser validates cannot drift apart.
4. **Every troubleshooting entry carries a provenance stamp** — assert the entry count equals the
   stamp count, not merely that some stamp exists somewhere (a `grep -q` over a literal that can
   legitimately appear elsewhere is the classic false-green).
5. **The never-allowlist list matches `scripts/docket.md`'s Not-exposed set**, derived by grep from
   that file at test time.
6. **README links the guide.**

**What these guards do NOT prove.** They prove structure — that a claim is stamped, that a set is
complete, that JSON parses. They cannot prove a claim is *true*: a doc sentinel proves a sentence
still exists, never that it is still correct (LEARNINGS 2026-07-13 #65, where every sentinel was
green over factually false prose). The truth of each classifier claim is validated by exactly one
thing: the human reading the guide against the verification-log appendix at the merge gate. The spec
states this so the review is not skipped on the strength of a green suite.

## ADR

**Cursor auto-run trust is granted at the facade, not per operation.** Recorded at build time via
`docket-adr`. Context: the classifier demotes a whole program on one unmatched leaf, and the facade
was built (0068) precisely to make the permission surface finite. Decision: allowlist the single
canonical `docket.sh` invocation rather than 13 per-operation entries. Consequences: the fragment
stays one stable line as operations come and go, and the trust boundary matches the one the code
already draws — at the cost of authorizing `docket-status`'s sweep, `terminal-publish`, `github-mirror`,
and `cleanup-feature-branch` unprompted, which the guide must state plainly. Relates to ADR-0029
(facade routing and config presentation), ADR-0020 (generated agent artifacts stay machine-local and
human-applied — docket never writes the user's Cursor config), ADR-0027 (terminal-publish gating).

## Out of scope

- Any change to scripts or skills — 0068 and 0072 own those surfaces.
- Automatically editing `~/.cursor/permissions.json` or `~/.cursor/sandbox.json` during `install.sh`
  (ADR-0020).
- Equivalent permission fragments for other harnesses (Claude Code, Codex). This change documents
  Cursor.
- Any facade operation, escape hatch, or blanket approval that would execute caller-supplied shell.

## Rejected alternatives

- **Per-operation allowlist entries (13 rows).** Buys prompt-on-destructive-ops control, but drifts
  with every new operation and contradicts the boundary 0068 established. Rejected in favour of one
  entry plus a loud statement of consequences.
- **Writing the guide from docket's code and Cursor's public docs, marking claims unverified.** An
  unverified guide is one careless merge away from shipping as fact; the change's own requirement is
  provenance.
- **Deferring the troubleshooting section to a follow-up change.** The troubleshooting is the part a
  user actually needs at 11pm when nothing runs; a guide without it is the easy half.
- **A separate observation-log file beside the spec.** A second artifact to link and to publish, for
  no gain over an appendix in the spec the implementer already reads.

## Appendix — verification log

- **Cursor version:** 3.11.19 (`bf249e6efb5b097f23d7e21d7283429f0760b740`, arm64)
- **Observation date:** 2026-07-14
- **Operator mode during successful facade tests:** **Allowlist (with Sandbox)**
- **Machine:** `DOCKET_SCRIPTS_DIR=/Users/$USER/dev/docket/scripts`; sandbox.json already granted
  `additionalReadonlyPaths: ["/Users/$USER/dev/docket"]` and `github.com` in network allow.

Harness observation channel: agent Shell tool footers reporting either
"ran outside the sandbox (… matched the user's command allowlist)" or sandboxed execution with
network/filesystem limits. No separate human approval prompt was required for allowlisted facade
calls under Allowlist (with Sandbox).

### A. Run Mode × `permissions.json` lock (product facts)

| Configuration | Selectable Run Modes | Notes |
|---|---|---|
| Non-empty `terminalAllowlist` and/or `mcpAllowlist` in `~/.cursor/permissions.json` | **Allowlist**, **Allowlist (with Sandbox)** only | **Auto-review (with Sandbox)** appears in the menu but is **not selectable**. Banner text only mentions Run Everything being disabled. Client sets `permissionsFileConstrainsUnrestrictedMode` when allowlists are non-empty. |
| `autoRun` only (allowlists removed) | Auto-review becomes selectable again | Confirmed. |
| `approvalMode: "unrestricted"` added alongside allowlists | **Broke Run Mode UI** — no options shown | **Rejected.** Removed immediately. Do not publish. |

**Guide implication:** publish for **Allowlist (with Sandbox)** + facade `terminalAllowlist`. Do not
claim Auto-review + file allowlists compose; that contradicts observed 3.11.19 behavior (and the
public docs). Treat the Auto-review lock as a Cursor bug/docs mismatch to cite, not to work around
with undocumented keys.

### B. Command forms submitted

| # | Form | Config | Where it ran | Result |
|---|---|---|---|---|
| B1 | `/Users/$USER/dev/docket/scripts/docket.sh env` | facade (+ day-to-day) allowlist | **Outside** sandbox (allowlist) | Success; env printed; `BOOTSTRAP=PROCEED` |
| B2 | `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh env` | same | **Outside** sandbox | Success |
| B3 | `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` | same | **Outside** sandbox | Success; fetched `origin/docket`; `BOOTSTRAP=PROCEED` |
| B4 | Same as B1–B3 | **`autoRun` only** (no allowlists); Auto-review available | **Inside** sandbox | `docket-config: cannot reach origin (git fetch failed)`; preflight exit 1 |
| B5 | B1–B3 again after restoring allowlists | Allowlist (with Sandbox) | **Outside** sandbox | Success (contrast with B4) |
| B6 | B1–B3 with **facade-only** docket entries (legacy per-helper `DOCKET_SCRIPTS_DIR/*` lines removed; only the two `docket.sh` spellings remain among docket entries) | Allowlist (with Sandbox) | **Outside** sandbox | Success — published fragment shape is sufficient |

### C. Failure modes — reproduced

| Mode | How tested | Observed |
|---|---|---|
| **Sandbox ≠ terminal permission** | B4: `autoRun` only under Auto-review | Commands auto-ran but stayed sandboxed; network/`git fetch` to origin failed. `sandbox.json` read path + network allowlist did **not** substitute for a terminal allowlist entry that runs the facade outside the sandbox. |
| **Spelling mismatch** | `"${DOCKET_SCRIPTS_DIR:?run docket/instal.sh}"/docket.sh env` (typo `instal`) | Demoted to sandbox; origin fetch failed (exit 1). |
| **Compound demotion (matched + unmatched leaf)** | `"…"/docket.sh env; eval true` | **Whole program** sandboxed despite the facade leaf being allowlisted. |
| **Unreachable unmatched leaf still demotes** | `if false; then eval true; fi; "…"/docket.sh env` | Whole program sandboxed. |
| **Direct helper bypass** | `"…"/docket-config.sh --export --format plain` after removing per-helper allowlist rows | Sandboxed; origin fetch failed (exit 1). Not covered by facade-only allowlist. |
| **Invalid JSON silently disables allowlist** | Truncated trailing `}` in `permissions.json` | Absolute `docket.sh env` demoted to sandbox (allowlist ignored). Restoring valid JSON restored outside-sandbox allowlist match within ~2s (file watcher; no IDE restart). |

### D. Failure modes — cut / not separately reproduced

| Candidate from stub | Verdict |
|---|---|
| Protected `.git` writes under sandbox | **Not separately exercised** this session. Do not assert a dedicated troubleshooting entry unless a later session stamps it. (Sandbox path already implies restricted `.git` / out-of-workspace writes.) |
| Network blocked by `sandbox.json` while command is allowlisted | **Does not apply as stated** when the allowlist match runs **outside** the sandbox (B1–B3/B5–B6 had working `git fetch`). Network denial was observed only for **sandboxed** demotions (B4, C rows). Guide should say: allowlisted facade runs unsandboxed; sandboxed demotions still hit network/filesystem limits. |

### E. Reload behavior

Edits to `~/.cursor/permissions.json` were picked up without restarting Cursor (allowlist strip/restore,
invalid JSON / restore) within a short sleep (~1–2s) before the next Shell call. Settings UI reflected
file-controlled allowlist (read-only Command Allowlist sourced from the file).

### F. Session conclusion for the build

1. Ship the guide targeting **Allowlist (with Sandbox)** + the two facade spellings in
   `terminalAllowlist` + sandbox read-path fragment.
2. State plainly that a `permissions.json` allowlist currently precludes selecting Auto-review
   (Cursor 3.11.19) — cite this appendix.
3. Troubleshooting entries to include (all stamped 3.11.19 / 2026-07-14): sandbox≠terminal;
   spelling mismatch; compound / unreachable-leaf demotion; invalid JSON; helper bypass.
4. Do not document `approvalMode` as a workaround.
