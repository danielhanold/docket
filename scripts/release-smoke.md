# release-smoke.sh — native per-tuple smoke driver for a release-candidate bundle

## Purpose

Proves one release-candidate bundle on the **host tuple**, against the **packaged bytes** — never a
rebuild. Introduced in change 0317. The workflow's smoke matrix runs this once per native runner
(one genuine OS/arch each); a developer runs it on their own machine as build evidence for their
tuple.

It answers a single question the other artifacts cannot: *do the bytes this bundle actually ships
verify, install, check clean, upgrade, and converge after an interruption — as executed, not as
described?* Every block runs the real archive, the real rendered downloader
(`internal/release/downloader/install.sh`, stamped into the bundle), and the real extracted
binary. Nothing is mocked and nothing is rebuilt.

## The host-tuple boundary — read this before trusting a green

**A `SMOKE PASS` line is evidence for the host tuple only.** A host process cannot execute a
foreign-tuple binary, so this driver verifies the four archives' *checksums* are all present in the
manifest (Block B requires exactly one manifest line for the host archive; the bundle's own
packager and `checksums.txt` cover the set) but can only *execute* the one archive built for the
machine it runs on. The other three tuples are proven by:

- the **workflow smoke matrix** — four native runners, one real OS/arch each, each printing its own
  `SMOKE PASS <os>/<arch> <version>` line (emulation or cross-compile does not count); and
- the **four-harness fresh-session live acceptance** — external truth routed to a human merge-gate
  checklist (`docs/release/four-harness-acceptance.md`).

Both are external truth (learnings `external-truth-needs-a-human-checkpoint`,
`generated-artifact-loaded-at-process-start`). No in-repo test, and not this script, may claim the
full four-tuple execution from a single host.

## Usage

```
release-smoke.sh --bundle <dir> --version <v> [--base-bundle <dir> --base-version <v>]
```

| Argument | Required | Description |
|---|---|---|
| `--bundle <dir>` | yes | A candidate bundle directory: the four `docket_<v>_<os>_<arch>.tar.gz` archives, `checksums.txt`, and the rendered `install.sh`. |
| `--version <v>` | yes | The bundle's version, exactly as stamped. Names the host archive and is the identity every block asserts. |
| `--base-bundle <dir>` | no | A **distinct prior** candidate bundle. Enables the upgrade block (G). |
| `--base-version <v>` | no | The base bundle's version. Required with `--base-bundle`, refused without it, and vice versa. |

Bash, because it runs on runners and dev machines — **not** inside the downloader's POSIX
constraint. `set -uo pipefail`; no `producer | early-exiting-consumer` pipeline; scratch dirs are
templated `mktemp -d "${TMPDIR:-/tmp}/release-smoke.XXXXXX"`. It needs `curl`, `tar`, and one of
`sha256sum`/`openssl` on `PATH`.

## Behavior

Every block runs against **isolated `HOME` / `XDG_*` / bin roots** the script creates under its own
scratch dir, so nothing touches the invoking user's real dot-directories. Each installing block is a
fresh *session* (its own home + state + bin). The downloader is fed the bundle over a
`file://` `DOCKET_RELEASE_BASE_URL`, so no block reaches the network.

| Block | What it proves |
|---|---|
| A — host tuple | `uname -s`/`-m` map to one of the four approved tuples; anything else fails here. |
| B — verify + extract | The host archive has **exactly one** `checksums.txt` line, its SHA-256 matches, its tar listing is exactly `docket`, and it extracts to one regular file. |
| C — identity | The extracted binary's `version --json` reports the exact expected version; `diagnostic runtime --json` reports the host `go_os`/`go_arch` and `supported_target: true`. |
| D — install | The bundle downloader installs across `--harness claude --harness codex --harness cursor --harness opencode`; a binary and an ownership record naming the version land. |
| E — check | `install check --json` is clean (`applied` or `no-op`, exit 0). |
| F — idempotence | A same-version rerun leaves the dest binary and the record **byte-identical** (hash before == after). |
| G — upgrade | *(only with `--base-bundle`)* Install base, plant a **foreign** sentinel under a harness dir, upgrade to head: the binary and record become head's, the upgrade is non-vacuous (base and head bytes differ), and the sentinel is byte-identical after — the downloader replaces only bytes it owns. |
| H — convergence | A **doctored copy** of the downloader with the single `mv -f "$stage" "$dest"` line replaced by `exit 97` interrupts after the asset install and before the rename; dest is never created and no stage file is left. A rerun of the **real** downloader then converges to a real installed binary and publishes the record. The doctoring is asserted to change exactly one line, so the case cannot pass vacuously (learning `assert-detects-removal-not-replacement`). |

On success, and only then, exactly one line is written to **stdout**:

```
SMOKE PASS <os>/<arch> <version>
```

The workflow summary greps for that exact line. Every other line — progress and diagnostics — goes
to **stderr**, so stdout carries the verdict and nothing else.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every block passed. `SMOKE PASS …` is on stdout. |
| 1 | The first failed block, with a `SMOKE FAIL: <block>: <reason>` diagnostic on stderr. Later blocks do not run. |
| 2 | Usage error (missing/lone flag, non-directory bundle), with usage on stderr. |

## Invariants

- **Host-tuple evidence only.** Executable proof is for the running machine's tuple; the other three
  are the workflow matrix's and the human acceptance's to establish. Stated in full above.
- **Packaged bytes, never a rebuild.** Every block runs the shipped archive, the rendered
  downloader, and the extracted binary. The script never invokes `go`.
- **Isolated roots, no network.** All state lives under one templated scratch dir removed on exit;
  the downloader is driven entirely over `file://`.
- **stdout is the verdict.** Exactly the one `SMOKE PASS` line on success; all else is stderr.
- **Fail closed on the first block.** A failed assertion dies immediately with a named block and
  reason; no later block can turn a real failure green.

## Related

- `internal/release/downloader/install.sh` — the downloader this script drives; its contract note is
  inline in its header.
- `tests/test_release_downloader_converge.sh` — the hermetic Task-9 convergence suite whose
  doctored-copy technique block H inlines against a real tuple.
- `tests/test_release_package.sh` — the suite file (change 0317, Task 11) that statically guards this
  script exists, is executable, and carries the `SMOKE PASS` line and the `--base-bundle` flag.
- `.github/workflows/release-candidate.yml` — the non-publishing candidate workflow whose smoke
  matrix runs this per native tuple.
- `docs/release/four-harness-acceptance.md` — the external-truth live acceptance this driver does
  **not** replace.
