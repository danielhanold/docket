# board-refresh.sh — gated, atomic writer for the `inline` board surface

## Purpose

Owns the **write decision** for `BOARD.md`. `render-board.sh` is a pure renderer (stdout only,
no git, no filesystem writes); `board-refresh.sh` composes on top of it, unchanged, and decides
*whether* `BOARD.md` gets written at all, keyed on the caller's already-resolved
`board_surfaces` config (`$BOARD_SURFACES`). A disabled or GitHub-only configuration must never
create, write, truncate, or delete `BOARD.md` — this script is the single choke point that
enforces that. Introduced in change 0059.

## Usage

```
board-refresh.sh --changes-dir DIR --surfaces "TOKENS" [--repo OWNER/REPO]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the changes directory (`active/`, `archive/`, and `BOARD.md` are children). Forwarded to `render-board.sh` unchanged. |
| `--surfaces "TOKENS"` | **yes, as a flag** — its *value* may be the empty string | The caller's already-resolved `$BOARD_SURFACES`, verbatim: space-separated tokens (e.g. `"inline"`, `"inline github"`, `"github"`, or `""`). The flag being **absent** is a wiring bug (exit 2); the flag present with an **empty value** is a valid "no surfaces configured" (no-op, exit 0). These two cases are tracked separately — never conflate "not supplied" with "supplied empty". |
| `--repo OWNER/REPO` | no | Forwarded verbatim to `render-board.sh` (builds `pr:` hyperlinks; see `render-board.md`). |

`-h` / `--help` prints this script's leading comment block.

Mock seam: `RENDER_BOARD="${RENDER_BOARD:-<sibling>/render-board.sh}"` — override in tests to
inject a stub renderer (mirrors `render-board.sh`'s `GIT="${GIT:-git}"` seam). Defaults to the
`render-board.sh` sibling of this script.

## Behavior

**Token gate.** `--surfaces` is split on whitespace. The write decision keys **only** on the
exact token `inline`:

| Tokens (example) | `inline` present? | Action |
|---|---|---|
| `"inline"` | yes | Render via `render-board.sh` and replace `BOARD.md`. |
| `"inline github"` | yes | Same as above — `github` is irrelevant to this script. |
| `"github"` | no | No-op: `BOARD.md` is left completely untouched. |
| `""` (empty) | no | No-op: `BOARD.md` is left completely untouched. |
| `"bogus"` (unknown token) | no | Warns on stderr; still a no-op (not an inline token, not an abort). |
| `"inline bogus"` | yes | Warns on stderr for `bogus`, but still renders — a typo never blocks or aborts the `inline` write. |

Any token other than `inline` or `github` is treated as unknown: it is reported on stderr
(`board-refresh: unknown surface token ignored: <token>`) and otherwise ignored. Unknown tokens
never cause a non-zero exit and never prevent `inline` (if also present) from taking effect.

**Rendering.** When `inline` is present, invokes `render-board.sh --changes-dir DIR [--repo
OWNER/REPO]` exactly as documented in `render-board.md` — this script does not reimplement or
alter any rendering logic.

**Stale-board no-op decision.** When `inline` is absent, the script returns before doing
anything filesystem-related: no temp file is created, `BOARD.md` is not opened, truncated, or
deleted, even if it already exists from a previous run. This is the fix for the truncation trap
where a bare `render-board.sh … > BOARD.md` redirect would truncate the file before the renderer
even ran.

## Diagnostics

| Condition | Stream | Message |
|---|---|---|
| Inline render succeeds | stdout | `board-refresh: inline rendered <changes-dir>/BOARD.md` |
| Inline disabled (no-op) | stdout | `board-refresh: inline disabled — no-op` |
| Unknown surface token | stderr | `board-refresh: unknown surface token ignored: <token>` |
| `render-board.sh` fails | stderr | `board-refresh: render-board.sh failed (exit N); BOARD.md left untouched` |
| Missing `--changes-dir` | stderr | `board-refresh: missing --changes-dir` |
| Invalid `--changes-dir` | stderr | `board-refresh: changes dir not found: <dir>` |
| Missing `--surfaces` flag | stderr | `board-refresh: missing --surfaces (pass --surfaces "" for none)` |
| Unknown CLI argument | stderr | `board-refresh: unknown argument: <arg>` |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Either `BOARD.md` was rendered and written (`inline` present), or the run was a deliberate no-op (`inline` absent) — both are success. |
| 2 | Argument/wiring error: `--changes-dir` missing or not a directory, `--surfaces` flag absent entirely, or an unrecognized flag. |
| *(other)* | Propagated verbatim from `render-board.sh` when the inline render fails; `BOARD.md` is left untouched in this case. |

## Invariants

- **Sole write gate for the inline surface.** Skills call this script instead of redirecting
  `render-board.sh` output to `BOARD.md` directly. The write decision lives here exactly once.
- **Atomic write.** When enabled, the render is captured in a temp file created *inside*
  `--changes-dir` (via `mktemp "$CHANGES_DIR/.board-refresh.XXXXXX"`), guaranteeing the final
  `mv` onto `BOARD.md` is a same-filesystem rename, not a copy. The move happens only after
  `render-board.sh` exits 0, and the `mv` itself is checked: if the rename fails the script exits
  non-zero rather than falsely reporting success. The temp file is created only on the enabled
  path (after argument validation and the disabled-no-op return), so its `trap … EXIT` cleanup
  covers exactly that path's exits — a successful write (where the rename already consumed the
  file, making the `rm -f` a harmless no-op) or a renderer/`mv` failure. On an early argument
  error no temp file exists yet, so the net "no `.board-refresh.*` file is ever left behind"
  guarantee holds regardless. Because `mktemp` creates the temp at `0600`, the script `chmod
  644`s it immediately before the `mv`, so a successful write deterministically leaves `BOARD.md`
  at `0644` — the git-tracked, pushed board's mode — rather than propagating the restrictive
  temp-file mode.
- **No git operations.** This script never runs `git add`, `git commit`, `git push`, or touches
  the index in any way. Callers own all git discipline (staging, committing, and each caller's
  own must-land or best-effort push posture).
- **render-board.sh is unchanged.** This script is a pure composition layer; it does not alter,
  wrap the output of, or reimplement any part of `render-board.sh`'s rendering contract.
- **Disabled means untouched, not deleted.** "Inline disabled" is a true no-op: an existing
  `BOARD.md` from a prior run (or from a since-changed surface configuration) is left exactly as
  it was — never truncated, rewritten, or removed.
