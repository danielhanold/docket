# tests/lib/sync_agents_common.sh — the prologue every tests/test_sync_agents*.sh shard sources
# (change 0227). NOT matched by the tests/test_*.sh discovery glob, so it never runs as a test.
#
# This file is sourced, so BASH_SOURCE points at tests/lib/ — REPO needs TWO levels up, where the
# unsharded test needed one. That is the ONLY line that differs from the prologue it replaces.
set -uo pipefail
unset XDG_CONFIG_HOME   # hermetic: the script reads ${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}; pin global to the sandbox
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# True when any occurrence of $2 is followed by $3 within $4 characters in $1.
# Use Perl rather than grep -Pz: BSD grep lacks -P and caps interval bounds at 255.
within(){
  perl -0e '
    my ($file, $first, $second, $limit) = @ARGV;
    open my $fh, "<", $file or exit 1;
    local $/;
    my $text = <$fh>;
    my $offset = 0;
    while (($offset = index($text, $first, $offset)) >= 0) {
      exit 0 if index(substr($text, $offset, length($first) + $limit), $second) >= 0;
      ++$offset;
    }
    exit 1;
  ' "$1" "$2" "$3" "$4"
}

# Extract a single-line frontmatter scalar value from a markdown file.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | sed -n 1p | sed 's/[[:space:]]*$//'; }

# Body = everything after the frontmatter's closing fence. Change 0168 made emit() strip-and-insert
# the resolved model/effort, so a generated wrapper is no longer byte-identical to its source; what
# must still hold is that the BODY comes through verbatim and only the pin is injected.
body_of(){ awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$1"; }

# The shipped default store (change 0168). Asserts about what an UNCONFIGURED wrapper is pinned to
# read it from here rather than from the wrapper source's frontmatter.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"

AGENTS="$REPO/agents"
AUTONOMOUS="docket-implement-next docket-auto-groom docket-finalize-change docket-status docket-adr"
SYNC="$REPO/sync-agents.sh"

# Helper: a fresh fake harness root + repo for an isolated generator run.
make_sandbox(){ SBX="$(mktemp -d)"; mkdir -p "$SBX/.claude" "$SBX/.agents"; }   # .cursor/.codex/.kiro/.windsurf absent on purpose

# Count the parser-heavy external commands used by one real generation pass. The shims deliberately
# exec the pre-PATH absolute tools so this measures the generator rather than replacing its behavior.
parser_subprocess_count(){  # $1=generator path; sets FORK_COUNT + FORK_RC
  local generator="$1" shim_dir fork_log harness_root tool real_tool
  local real_sed real_head real_awk real_grep
  real_sed="$(command -v sed)"
  real_head="$(command -v head)"
  real_awk="$(command -v awk)"
  real_grep="$(command -v grep)"
  shim_dir="$(mktemp -d)"
  fork_log="$(mktemp)"
  harness_root="$(mktemp -d)"
  mkdir -p "$harness_root/.claude"
  : > "$fork_log"
  for tool in sed head awk grep; do
    case "$tool" in
      sed) real_tool="$real_sed" ;;
      head) real_tool="$real_head" ;;
      awk) real_tool="$real_awk" ;;
      grep) real_tool="$real_grep" ;;
    esac
    printf '%s\n' '#!/usr/bin/env bash' \
      "printf '%s\\n' '$tool' >> \"\${DOCKET_175_FORK_LOG:?}\"" \
      "exec $real_tool \"\$@\"" > "$shim_dir/$tool"
    chmod +x "$shim_dir/$tool"
  done
  make_sandbox
  PATH="$shim_dir:$PATH" DOCKET_175_FORK_LOG="$fork_log" DOCKET_HARNESS_ROOT="$harness_root" \
    bash "$generator" >/dev/null 2>&1
  FORK_RC=$?
  FORK_COUNT="$(wc -l < "$fork_log" | tr -d '[:space:]')"
  rm -rf "$SBX" "$shim_dir" "$fork_log" "$harness_root"
}

# git-repo fixture: sandbox repo with identity + one commit (for ls-files-based legs).
# Defined here (rather than at first historical use, further down) so the change-0057
# widened-trigger tests — which need a real docket branch — can use it too.
mkgitrepo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
}

fm_key_count(){  # $1=file $2=key -> occurrences of `^<key>:` inside the first --- block
  awk -v k="$2" '/^---[[:space:]]*$/{ d++; if (d>=2) exit; next }
                 d==1 && $0 ~ "^"k"[[:space:]]*:" { n++ }
                 END { print n+0 }' "$1"
}

# Frontmatter-ANCHORED read. fm() above is first-match-anywhere, so on a wrapper with no pin it
# scans into the body and can return prose — a false green for an assert whose whole point is
# "the pin is present" (AGENTS.md: anchor a frontmatter read to the first ---…--- block).
fm_anchored(){  # $1=file $2=key
  awk -v k="$2" '/^---[[:space:]]*$/ { d++; if (d>=2) exit; next }
                 d==1 && $0 ~ "^"k"[[:space:]]*:" { sub("^"k"[[:space:]]*:[[:space:]]*",""); sub(/[[:space:]]+$/,""); print; exit }' "$1"
}
