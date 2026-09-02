package install

// legacy_capture_test.go — the guarded generator that (re)freezes the v0.9.2 legacy golden corpus
// under testdata/legacy. It is SKIPPED BY DEFAULT: it runs the FINAL v0.9.2 Bash installer against
// a sandboxed HOME and copies the emitted bytes, so it only runs when explicitly enabled and only
// on a machine that has the clean v0.9.2 checkout. It asserts nothing about the Go reproducer
// (that is B2+); its sole job is deterministic, documented regeneration of the corpus. See
// testdata/legacy/README.md for the inventory and provenance these bytes carry.
//
// Regenerate:
//
//	DOCKET_LEGACY_CAPTURE=1 go test ./internal/install -run TestCaptureLegacyCorpus
//
// Optional env: DOCKET_V092 overrides the v0.9.2 checkout path
// (default /Users/homer/dev/docket-v0.9.2).

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// captureScript runs the v0.9.2 sync-agents.sh once per pin shape under a sandboxed HOME and copies
// the exact emitted user-level bytes (agent definitions, Cursor dispatch rule, managed dispatch
// block) into $DEST. It mutates nothing under the real $HOME and nothing in the v0.9.2 checkout.
const captureScript = `
set -euo pipefail
: "${V092:?}"; : "${DEST:?}"; : "${WORK:?}"
HARNESSES="claude codex cursor opencode"
PAIR="status brainstorm-consultant"
START='<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->'
END='<!-- docket:dispatch:end -->'
ext(){ case "$1" in codex) printf 'toml';; *) printf 'md';; esac; }
# Remove only GENERATED content; never the hand-authored README.md.
rm -rf "$DEST/claude" "$DEST/codex" "$DEST/cursor" "$DEST/opencode" "$DEST/dispatch-block" "$DEST/_inputs"
mkdir -p "$DEST/_inputs"
run_shape(){
  local shape="$1" frag="$2" home repo h e a
  home="$WORK/$shape/home"; repo="$WORK/$shape/repo"
  mkdir -p "$home/.config/docket" "$repo"
  { printf 'agent_harnesses: [claude, codex, cursor, opencode]\n'; [ -n "$frag" ] && cat "$frag"; :; } \
    > "$home/.config/docket/config.yml"
  cp "$home/.config/docket/config.yml" "$DEST/_inputs/config-$shape.yml"
  printf 'agent_harnesses: [claude, codex, opencode]\n' > "$repo/.docket.yml"
  ( cd "$repo" && env -u XDG_CONFIG_HOME HOME="$home" DOCKET_HARNESS_ROOT="$home" \
      bash "$V092/sync-agents.sh" ) >"$WORK/$shape.log" 2>&1
  for h in $HARNESSES; do
    e="$(ext "$h")"; mkdir -p "$DEST/$h/$shape/agents"
    for a in $PAIR; do cp "$home/.$h/agents/docket-$a.$e" "$DEST/$h/$shape/agents/docket-$a.$e"; done
  done
}
mk_frag(){ cat > "$1" <<EOF
agents:
  default:
    status: { model: $2, effort: $3 }
    brainstorm-consultant: { model: $2, effort: $3 }
EOF
}
mk_frag "$WORK/frag-full.yml" legacy-pinned-model high
mk_frag "$WORK/frag-partial.yml" legacy-pinned-model auto
mk_frag "$WORK/frag-unpinned.yml" inherit auto
run_shape default ""
run_shape fully-pinned "$WORK/frag-full.yml"
run_shape partially-pinned "$WORK/frag-partial.yml"
run_shape unpinned "$WORK/frag-unpinned.yml"
mkdir -p "$DEST/cursor" "$DEST/dispatch-block"
cp "$WORK/default/home/.cursor/rules/docket-dispatch.mdc" "$DEST/cursor/docket-dispatch.mdc"
awk -v s="$START" -v e="$END" '$0==s{inb=1} inb{print} $0==e{inb=0}' "$WORK/default/repo/AGENTS.md" > "$DEST/dispatch-block/block.md"
awk -v s="$START" -v e="$END" '$0==e{inb=0} inb{print} $0==s{inb=1}' "$WORK/default/repo/AGENTS.md" > "$DEST/dispatch-block/interior.md"
git -C "$V092" rev-parse HEAD > "$DEST/_inputs/PROVENANCE-v0.9.2-HEAD.txt"
cp "$V092/agents/harness-defaults.yml" "$DEST/_inputs/harness-defaults.yml"
`

func TestCaptureLegacyCorpus(t *testing.T) {
	if os.Getenv("DOCKET_LEGACY_CAPTURE") == "" {
		t.Skip("guarded generator; set DOCKET_LEGACY_CAPTURE=1 to (re)freeze testdata/legacy from the v0.9.2 checkout")
	}
	v092 := os.Getenv("DOCKET_V092")
	if v092 == "" {
		v092 = "/Users/homer/dev/docket-v0.9.2"
	}
	if _, err := os.Stat(filepath.Join(v092, "sync-agents.sh")); err != nil {
		t.Fatalf("v0.9.2 checkout not found at %s (set DOCKET_V092): %v", v092, err)
	}
	dest, err := filepath.Abs(filepath.Join("testdata", "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", captureScript)
	cmd.Env = append(os.Environ(),
		"V092="+v092,
		"DEST="+dest,
		"WORK="+testsupport.TempDir(t),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("capture failed: %v\n%s", err, out)
	}
}
