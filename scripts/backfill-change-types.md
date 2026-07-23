# scripts/backfill-change-types.sh — one-time active-backlog type categorization

## Purpose

Apply a **human-approved** `id -> type` mapping to the change files under `<changes-dir>/active/`
(change 0127). This is the deterministic half of the rollout that gives an existing backlog its
`type:` values; the semantic half is a human decision made before the script runs.

The division is the ADR-0012 script-vs-model boundary, in three steps:

1. An interactive agent reads each active change and **proposes** a complete mapping.
2. The human **reviews and approves** that mapping as a single decision.
3. This script **validates and applies** it mechanically — all files or none.

It performs no classification of its own and never guesses a type. It also never reads or edits
`<changes-dir>/archive/`: archived records are intentionally not backfilled, so their bytes must be
identical before and after any run.

Committing and pushing the result is the **caller's** job — the script is a pure file rewriter with
no git, no network, and no knowledge of the metadata branch.

## Usage

```
backfill-change-types.sh --changes-dir DIR --map ID=TYPE[,ID=TYPE...] [--dry-run]
backfill-change-types.sh -h | --help
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | The changes directory whose `active/` holds the backlog (e.g. `.docket/docs/changes`). Must contain an `active/` subdirectory. |
| `--map PAIRS` | yes | Comma-separated `ID=TYPE` assignments. Must cover **every** untyped active change (see *the migration set*). |
| `--dry-run` | no | Run every validation and report how many files would change; write nothing. |
| `-h`, `--help` | no | Print the script header and exit 0. |

Reached from a skill through the facade: `docket.sh backfill-change-types …`.

Find the exact input set first — `docket-status --digest-only --type untyped` is the authoritative
inventory. `--digest-only` is load-bearing: without it the pass commits and pushes `BOARD.md`,
sweeps, archives, and harvests before printing the digest, so a command run to *look* would write.

## Behavior

**1. Parse and validate the mapping — before any write.** Every check below runs to completion
before a single file is rewritten. A helper that validated lazily would fail on entry *N* having
already written entries *1..N-1*, which is exactly the half-migrated backlog the all-or-nothing
contract exists to prevent.

- Each entry must be `ID=TYPE`; `ID` must be a non-negative integer.
- `TYPE` is rejected if it contains a control character (the structural-injection shape: a newline
  would otherwise inject a second frontmatter line), if it is a **reserved** value (`all`,
  `untyped` — a config selector and a query pseudo-value, never legal in a stored manifest), or if
  it does not match `[a-z][a-z0-9-]*`.
- A repeated id is a duplicate assignment and is refused.
- `TYPE` is deliberately **not** checked against the effective `change_types`. The caller resolved
  the taxonomy and made the classification; the script stays usable in a repo whose configured
  taxonomy differs from the one that wrote a given record.

**2. Resolve the active population and the migration set.** Every `*.md` directly under `active/`
with an `id:` is in the population. The **migration set** is the subset whose `type:` is absent or
empty in its **first frontmatter block**. Reads use `fm_field`, never `field`: an unanchored read
would fall through to body prose for a change that has no frontmatter `type:` yet — precisely the
state of every record this script exists to fix.

**3. Refuse, leaving every file untouched, when:**

| Condition | Diagnostic names |
|---|---|
| an id is not an active change (including an **archived** id) | `not an active change` |
| an id already carries a different non-empty type | `already has type` |
| an untyped active change has no assignment | `incomplete mapping` |
| a duplicate assignment | `duplicate assignment` |
| a reserved type | `reserved value` |
| a malformed type | `must match` |

Each refusal is pinned to its own diagnostic in `tests/test_backfill_change_types.sh`, not merely
to "non-zero exit + nothing written": several independent mechanisms can satisfy that weaker
assertion, so removing a guard once left the whole refusal block green.

**4. Apply — staged, then installed.** Each rewrite goes to a scratch directory and is verified
there; only once every file has rewritten cleanly are they moved into place. A failure partway
through therefore cannot leave a half-migrated backlog.

The write is **anchored to the first `---…---` block** (AGENTS.md): an awk counter tracks the
frontmatter delimiters, so a body line beginning `type:` can never be rewritten. An existing
**empty** `type:` placeholder inside that block (the change template's own shape) is filled in
place; otherwise the field is inserted immediately before the block's closing `---`. The value is
passed through awk's `ENVIRON`, never interpolated into a `sed` replacement, so `&` and
backreferences in it cannot be reinterpreted.

**5. Idempotent.** A change whose stored type already equals its assignment is skipped, so rerunning
an applied mapping is a byte-level no-op that still exits 0.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Applied (or a dry run completed, or nothing needed changing). The count is reported on stdout. |
| `1` | Any validation failure, rewrite failure, or install failure. Nothing was installed. |

## Invariants

- **Active-only.** `<changes-dir>/archive/` is never read and never written; its bytes are identical
  across every run, asserted by a hash comparison in the test suite.
- **All files or none.** No partial application is observable, on any failure path.
- **Idempotent.** Rerunning an applied mapping changes nothing and exits 0.
- **First-frontmatter-block anchored**, for both the read and the write.
- **Never classifies.** The type always comes from the caller's approved mapping (ADR-0012).
- **No git, no network.** Committing the migration is the caller's responsibility.
