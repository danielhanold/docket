#!/bin/sh
# install.sh — bootstrap docket's development install from this checkout (change 0322).
#
# This is a thin POSIX bootstrapper, NOT the installer. It resolves its own checkout
# directory independent of the caller's CWD, then chooses one of three paths by probing
# for an installed `docket`:
#
#   (a) a compatible installed `docket`  -> delegate to `docket development install`
#   (b) no `docket` on PATH              -> `go run -C <checkout> ./cmd/docket development install`
#   (c) a `docket` present but whose `development install` probe errors, or does not name
#       the install operation            -> refuse non-zero, mutating nothing. This third
#       state must NOT fall through to the go-run path: a broken-but-present docket is a
#       machine to fix, not a case to silently work around.
#
# The heavy lifting (building the binary, linking harnesses, adopting legacy artifacts,
# the journaled install transaction) lives in the Go engine reached through that command.
#
# DOCKET_BOOTSTRAP_DRY_RUN=1 prints the resolved command to stdout and exits 0 without
# running it — the test seam this file's own tests key on.
#
# Runs under the system /bin/sh: no bashisms, and no dependency on any docket runtime, so a
# bare checkout can bootstrap itself before anything docket has been installed.
set -eu

# Resolve <checkout> as the directory holding this script, canonicalised and independent of the
# caller's CWD. pwd -P so a checkout reached through a symlinked parent yields one physical name —
# the same canonicalisation sync-agents.sh's resolve_physical_path applies to the dirs it walks.
SOURCE_ROOT="$(cd "$(dirname "$0")" && pwd -P)"

# docket_is_compatible — tri-state probe of an installed `docket`:
#   0  a usable installed docket that advertises the development-install operation -> delegate
#   1  present but its `development install` probe errored or did not name the operation
#      (the third state — the caller must refuse, never fall through to go run)
#   2  absent from PATH -> the go-run fallback is appropriate
docket_is_compatible() {
  command -v docket >/dev/null 2>&1 || return 2
  # Capture the probe output first, then test the captured text: never pipe a producer into an
  # early-exiting consumer. A non-zero probe is the third state (return 1), not absence.
  probe="$(docket development install --help 2>/dev/null)" || return 1
  case "$probe" in
    *install*) return 0 ;;
    *) return 1 ;;
  esac
}

# emit_or_run — dry-run prints the resolved command on one line; otherwise exec it (replacing this
# shell so the delegated exit status is the caller's, unchanged). The real path is `exec "$@"`, so
# every argument crosses as its own argv word — a checkout or --bin-dir value carrying a space stays
# one word. The dry-run join collapses spacing for a legible single-line echo only.
emit_or_run() {
  if [ "${DOCKET_BOOTSTRAP_DRY_RUN:-}" = 1 ]; then
    printf '%s\n' "$*"
    exit 0
  fi
  exec "$@"
}

# The condition of an `if` is exempt from set -e, so a non-zero return here is captured, not fatal.
if docket_is_compatible; then
  state=0
else
  state=$?
fi

# Passthrough flags — forward only an explicit allowlist to the delegated installer: --bin-dir <path>
# and a repeatable --harness <name>, each preserved as a distinct argv word in the order given.
# Anything else is refused rather than forwarded blind. This rebuilds "$@" in place to hold exactly
# the normalized passthrough words: the classic count-guarded filter — consume the ORIGINAL args from
# the front (their number tracked in `argc`) and append accepted words to the back — so no eval and no
# command string is ever constructed. `argc` bounds the loop to the originals, so an appended word is
# never re-read as a flag, and a value is only taken while an original remains (argc >= 1) — which is
# also how a trailing value-less --bin-dir/--harness is caught.
argc=$#
while [ "$argc" -gt 0 ]; do
  arg="$1"; shift; argc=$((argc - 1))
  case "$arg" in
    --bin-dir=*|--harness=*)
      set -- "$@" "$arg"
      ;;
    --bin-dir|--harness)
      if [ "$argc" -lt 1 ]; then
        printf 'docket: %s requires a value.\n' "$arg" >&2
        exit 2
      fi
      val="$1"; shift; argc=$((argc - 1))
      set -- "$@" "$arg" "$val"
      ;;
    *)
      printf 'docket: unsupported argument: %s\n' "$arg" >&2
      printf 'docket: this bootstrapper forwards only --bin-dir <path> and --harness <name> to the installer.\n' >&2
      exit 2
      ;;
  esac
done

# Build the full delegated argv (base command + validated passthrough) as positional parameters, then
# hand it to emit_or_run as "$@" — never a concatenated string.
case "$state" in
  0)
    set -- docket development install --source "$SOURCE_ROOT" "$@"
    ;;
  2)
    if ! command -v go >/dev/null 2>&1; then
      printf 'docket: no compatible "docket" on PATH and the Go toolchain is required to bootstrap from source.\n' >&2
      printf 'docket: install Go (https://go.dev/dl/) and re-run, or install a compatible docket first.\n' >&2
      exit 1
    fi
    # Anchor `go run` to the checkout with -C: the `./cmd/docket` package spec resolves RELATIVE
    # to the process CWD, and this bootstrapper runs from the caller's arbitrary CWD (it resolved
    # SOURCE_ROOT from $0, never from `pwd`). Without the anchor, a run from any directory other
    # than the checkout fails package resolution. -C stays two distinct argv words, so it crosses
    # `exec "$@"` argv-safe like every other word; --source remains the absolute checkout path.
    set -- go run -C "$SOURCE_ROOT" ./cmd/docket development install --source "$SOURCE_ROOT" "$@"
    ;;
  *)
    printf 'docket: found an incompatible "docket" on PATH; its development-install probe failed.\n' >&2
    printf 'docket: refusing to continue. Install a compatible docket, or remove it from PATH to bootstrap from source.\n' >&2
    exit 1
    ;;
esac

emit_or_run "$@"
