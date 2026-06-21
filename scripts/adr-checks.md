# adr-checks.sh — ADR-ledger health checks

## Purpose

The ADR-ledger analog of `board-checks.sh`. Walks the ADR files in `--adrs-dir`, emits one
TAB-separated finding per line on stdout for each problem found, and exits 0 unless `--strict`
is passed. Offline and warn-only — it never modifies files. The caller (`docket-adr`) surfaces
the findings. Introduced in change 0030.

## Usage

```
adr-checks.sh --adrs-dir DIR [--strict]
```

| Flag | Required | Description |
|---|---|---|
| `--adrs-dir DIR` | yes | Path to the directory containing the ADR `*.md` files. `README.md` is excluded from all checks. |
| `--strict` | no | Exit 1 if any finding is emitted (a future CI gate). Default: exit 0 regardless of findings. |

**Output format:** every finding is `<check-id>\t<adr-id>\t<message>` on stdout, sorted by
`(check-id asc, adr-id numeric asc)`. A clean ledger produces no output.

## Behavior

### Check enumeration

The script sources `lib/docket-frontmatter.sh` and performs a single scan of every `*.md`
file (excluding `README.md`) to collect `id`, `status`, `supersedes`, `reverses`, and
`relates_to` fields. It then runs three named checks:

**`adr-numbering-gap`** — For every integer `n` from 1 to the highest observed ADR id, if no
file with that id exists, a `adr-numbering-gap` finding is emitted. Files with non-integer ids
are collected separately as `malformed-id` and do not count toward `MAXID` or the gap range.

**`adr-dangling-link`** — For each ADR, every integer referenced in `supersedes:`,
`reverses:`, and `relates_to:` is checked against the set of known ids. A reference to an id
with no file emits a `adr-dangling-link` finding (on the source ADR, not the missing target).

**`adr-status-inconsistent`** — Two arms:

- *Arm (a): non-existent target in status.* If an ADR's `status:` value is
  `"Superseded by ADR-NNNN"` or `"Reversed by ADR-NNNN"` and the referenced ADR has no file,
  a finding is emitted on that ADR.

- *Arm (b): un-flipped back-pointer.* When ADR-X carries a `supersedes: [Y]` edge, ADR-Y's
  `status:` must be `"Superseded by ADR-X"` (exactly, verb and id). When ADR-X carries a
  `reverses: [Y]` edge, ADR-Y's `status:` must be `"Reversed by ADR-X"`. Any mismatch —
  including the right id with the wrong verb — emits a finding on the target ADR-Y. ADR-Y ids
  that are already flagged as `adr-dangling-link` are skipped in arm (b).

**`malformed-id`** — A file whose `id:` field is non-empty but non-integer emits a
`malformed-id` finding (using the raw string as the adr-id column). The file is then skipped
for all other checks.

### Sorting and strict mode

All findings are accumulated and sorted by `(check-id asc, adr-id numeric asc)` before
output. With `--strict`, the script exits 1 if any findings were emitted; otherwise it always
exits 0.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Clean ledger, or findings present without `--strict`. |
| 1 | One or more findings emitted and `--strict` was passed. |
| 2 | Missing or invalid argument (`--adrs-dir` absent/not a directory, unknown flag). |

## Invariants

- **Offline.** No network calls, no `gh`, no git reads. All checks operate on the local file
  set under `--adrs-dir`.
- **Warn-only, never auto-fixes.** The script emits findings and exits; it never modifies ADR
  files or the git index.
- **STDOUT for findings, STDERR for errors.** Callers capture stdout; usage errors go to
  stderr.
- **Deterministic.** Same inputs → identical output. Sorted by `(check-id, adr-id)`.
- **`README.md` is excluded.** The ADR index file is never parsed for frontmatter or checked
  for id integrity.
- **`docket-adr` owns display.** This script is an implementation detail of `docket-adr`;
  it does not surface anything to the user directly.
