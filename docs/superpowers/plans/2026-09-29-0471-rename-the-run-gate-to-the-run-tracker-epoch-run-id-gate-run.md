<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0471 — Rename the run gate to the run tracker (epoch → run id, gate-* → run-*)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0471-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run.md)**
<!-- docket:backlink:end -->
# Rename the Run Gate to the Run Tracker Implementation Plan

> **For agentic workers:** Execution is via the `docket-build` role (task-by-task through its profile agents under the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply ADR-0129 family (a), rows 1–38 and 38a–38d, as a hard cut: the run gate becomes the **run tracker** across its operation ids, CLI verbs, flags, report lines, codes, failure stages, Go identifiers, file names, local storage, skills, generated dispatch material and docs, sealed by a mutation-tested retired-vocabulary table.

**Architecture:** One behavioral change and a large mechanical rename. Task 1 resets the run tracker's local storage to new roots and keys; this is the only runtime behavior change, and it is test-first. Tasks 2–3 hard-cut the wire tokens with a bounded token engine applied to a grep-derived file set. Task 4 renames files and integration-shard prefixes. Task 5 renames Go identifiers with an AST census, a rule-driven rename map, and an AST-only rewriter. Tasks 6–8 rewrite prose in Go comments and strings, in agent-facing markdown (with the regenerated `AGENTS.md` block), and in docs. Task 9 adds the retired-vocabulary seal. Task 10 runs the residual audit and the full suite. Every task leaves the tree compiling and its focused tests green.

**Tech Stack:** Go 1.27 (`go/ast`, `go/scanner`, cobra CLI), Perl 5 for the text engines, bash, `cmd/genassets` (`go generate ./internal/assets/`), harness golden tests (`-update`), the Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-29-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run-design.md` on the `docket` metadata branch (readable at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-29-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run-design.md`). The naming authority is ADR-0129, family (a) table (`/Users/homer/dev/docket/.docket/docs/adrs/0129-collision-free-docket-vocabulary.md`). The spec's §5 carries the landing procedure the results file's `**Human action:**` line and the PR body must state. This plan does not write the results file.

## Global Constraints

Worktree (every command runs here; use absolute paths): `/Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run` (below: `$W`). Base commit: `e68669728ed736d5b5195a0807e5734fd6e3f81e`. Tool directory (below: `$T`): `"${TMPDIR:-/tmp}/docket-0471"`. It is scratch space; nothing in it is ever committed.

- **Rows owned by this change:** ADR-0129 family (a), rows 1–38 and 38a–38d, exactly as the ADR spells them. Rows 39–66 belong to changes 0472–0474 and 0468. Do not touch `--version`, "change version", "build profile", "review rung", dispatch tiers, `rearm` / "re-arm" (auto-groom), `fenced-setting-ignored`, or any `terminal-*` code.
- **Hard cut, no aliases** (ADR-0129 Decision 2). An old spelling is refused, never accepted beside the new one. No hidden flags, no alias verbs, no dual-spelling parsing.
- **Kept names — never rename:**
  - "gate" in the checkpoint sense: the `docket gate …` CLI noun, the gate drive, the gate run, `gatedrive-*`, `idempotent-suite-gate`, the launch gate inside `run-launch-gate`, finalize/build/suite gates.
  - The suite-gate evidence reasons `gate-failed`, `gate-running`, `gate-stopped`, `gate-signaled`, `gate-vanished`, and the `gate-scope` run participant kind.
  - `tests/test_go_integration_app_gatelifecycle.sh` and its `TestIntegrationGateLifecycle*` prefix; `tests/test_go_integration_gatedrive_process.sh`; `tests/test_go_integration_gatedrive_race.sh`; every `repocheck` / `RepoCheck` name ("r-epoch-eck" is not "epoch").
  - Unix-epoch meanings: `cmd/releasepkg --source-epoch`, `SourceEpoch`, `SOURCE_DATE_EPOCH`, everything under `internal/release/**`, `cmd/releasepkg/**`, `internal/gitcli/**`, `.github/**`, and the phrases "Unix epoch" and "epoch seconds".
  - Committed state (Decision 3): the claim receipts' JSON key `gate_context_hash` and the claim idempotency digest payload. Their Go fields are renamed (Task 5). The JSON tag stays `gate_context_hash`.
  - Wire names that are not ADR-0129 rows: `run.cancel`, `run.verify`, `--key`; the reason tokens without "gate" or "epoch" (`run-waiting`, `run-halted`, `takeover-ambiguous`, `no-attributable-claim`, `resume-active-run`, `owner-lifecycle-unavailable`, …); the `run.start` result keys `key`, `dispatch_context`, `owner_lifecycle`; the env vars `DOCKET_AGENT_GUARDIAN_GATE_KEY`, `DOCKET_AGENT_GUARDIAN_REPO_DIR`, `DOCKET_AGENT_GUARDIAN_MARKER`.
  - **The gate drive's own `--gate-context` flag** on `gate drive start` and `gate drive prepare-scope`. Row 12 retires `change claim --gate-context` only. Note this as a follow-up candidate in your task report; do not rename it.
  - Config keys, agent names, frontmatter fields, the tracker's `record.json` / `claim-binding.json` / retry-marker file names, and the `gate-suite-budgets/v1` store (it carries no retired key).
- **Never edit (point-in-time records, frozen corpora, generated trees):** `docs/changes/**`, `docs/results/**`, `docs/superpowers/**` other than this plan, `docs/adrs/**` (ADR-0129 included), any `testdata/**` (except harness goldens regenerated with `-update`), `tests/fixtures/**`, `internal/install/legacydata/**`, `internal/install/legacy*.go` (legacy-install recognizers), `docs/codex/fixtures/**`, `docs/reference/harness/fixtures/**`. `internal/gatedrive/testdata/**` keeps its old `gate_context_hash` keys: those fixtures stand for records the new binary never reads.
- **Generated, never hand-edited:**
  - `internal/assets/embedded/**`: after editing anything under `skills/`, `agents/`, `cursor-rules/` or `.docket.example.yml`, run `go generate ./internal/assets/` and commit the result in the same commit (`TestEmbeddedMatchesAuthored`).
  - Harness goldens: after `go generate`, run `go test -count=1 ./internal/harness/claude/ ./internal/harness/codex/ ./internal/harness/cursor/ ./internal/harness/opencode/ -update`, then re-run without `-update`.
  - The `AGENTS.md` managed `docket:dispatch` block (and so `CLAUDE.md`, its symlink): regenerate it with the helper below, after `go generate`. Never edit it by hand.
- **The installed `docket` is the old binary.** Never verify a renamed verb or flag with the `docket` on `PATH`. Use `go run ./cmd/docket …` from `$W`. The build's own gate drive keeps using the installed binary, and that is fine.
- **Integration tests** carry `//go:build integration`: run them with `-tags integration`. Every verification and every mutation probe uses `-count=1` (learning `cached-runner-serves-a-mutated-tree`).
- **Mutation probes** back up the file and restore from the backup (`cp f "$T/f.bak"; <mutate>; <run>; cp "$T/f.bak" f`). Never restore with `git checkout --` (learning `mutation-restore-needs-a-backup-copy`). Confirm each mutation landed (a before/after count) before reading its result.
- **Shell hygiene:** run scans under an explicit `bash -c`, use `command git`, and judge a scan by the lines it prints, never by its exit status. Never pipe a producer into an early-exiting consumer (`grep -q`, `head`). Capture into a variable first.
- **Staging:** `git add` explicit paths only, never `-A` or `.`. Use `git mv` for every rename. Commit messages end with `(change 0471)`.
- **Skill budgets:** after editing any `skills/**` file, run `go test -count=1 ./internal/repoguard/ -run 'TestSkillSizeBudgets|TestDispatchBlockBudget'`. The renames are word-neutral or shorter. If a ceiling still breaks, re-baseline only that row, at the exact new count, with a `0471: ADR-0129 run-tracker rename` note. `dispatchBudget` may never be raised (its ceiling must stay below `dispatchOld`).
- **Final gate:** the whole suite through `build.test_command`: `go run ./cmd/docket development test`, entered from `$W`.

### Shared tooling

Install these tools at the start of every task that uses them. Paste the block verbatim; it is idempotent.

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0471"; mkdir -p "$T"

cat > "$T/fileset.sh" <<'"'"'EOF'"'"'
#!/usr/bin/env bash
# fileset.sh <pathspec>... : NUL-separated tracked files under the pathspecs, minus
# point-in-time records, frozen corpora, generated trees, the managed AGENTS.md /
# CLAUDE.md pair, legacy-install recognizers, the Unix-epoch paths, and the two
# storage-reset tests whose fixtures spell the RETIRED layout on purpose.
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
exec git ls-files -z -- "$@" \
  ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" \
  ":!docs/codex/fixtures" ":!docs/reference/harness/fixtures" \
  ":!internal/assets/embedded" ":!testdata" ":!**/testdata/**" ":!tests/fixtures" \
  ":!internal/install/legacydata" ":!internal/install/legacy*.go" \
  ":!internal/release" ":!cmd/releasepkg" ":!internal/gitcli" ":!.github" \
  ":!AGENTS.md" ":!CLAUDE.md" \
  ":!internal/gatedrive/storage_reset_test.go" \
  ":!internal/app/runtracker_storage_reset_integration_test.go"
EOF

cat > "$T/scan.sh" <<'"'"'EOF'"'"'
#!/usr/bin/env bash
# scan.sh <perl-regex> [pathspec...] : every case-insensitive, whitespace-tolerant
# match in the editable fileset (default pathspec ".") plus AGENTS.md, printed as
# file:line: match. A phrase wrapped across a line break is still found.
here="$(cd "$(dirname "$0")" && pwd)"; re="$1"; shift
[ "$#" -eq 0 ] && set -- .
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
{ "$here/fileset.sh" "$@"; printf "AGENTS.md\0"; } | RE="$re" xargs -0 perl -0777 -ne '"'"'
  while (/$ENV{RE}/gi) {
    my $l = 1 + (substr($_, 0, $-[0]) =~ tr/\n//);
    (my $m = $&) =~ s/\s+/ /g;
    print "$ARGV:$l: $m\n";
  }'"'"'
EOF

cat > "$T/tokens.pl" <<'"'"'EOF'"'"'
#!/usr/bin/perl
# tokens.pl MAP FILE... : bounded literal token replacement, in place. MAP is a TSV of
# OLD<TAB>NEW. OLD matches only where the bytes on both sides are outside
# [A-Za-z0-9_-]: gate-stop never matches gate-stopped, gate_context never matches
# gate_context_hash, --epoch never matches --source-epoch. Longer OLD values win.
# DRY=1 counts without writing.
use strict; use warnings;
my $map = shift @ARGV; my %m;
open my $mf, "<", $map or die "map $map: $!";
while (<$mf>) { chomp; next if /^\s*(#|$)/; my ($o, $n) = split /\t/, $_, 2;
  die "bad map line: $_\n" unless defined $n && length $o; $m{$o} = $n; }
my $alt = join "|", map { quotemeta } sort { length($b) <=> length($a) } keys %m;
my $re = qr/(?<![A-Za-z0-9_-])($alt)(?![A-Za-z0-9_-])/;
my ($files, $hits) = (0, 0);
for my $file (@ARGV) {
  open my $in, "<", $file or die "read $file: $!"; local $/; my $s = <$in>; close $in;
  my $c = ($s =~ s/$re/$m{$1}/g); next unless $c;
  $files++; $hits += $c;
  print STDERR "  $file: $c\n" if $ENV{DRY};
  next if $ENV{DRY};
  open my $out, ">", $file or die "write $file: $!"; print $out $s; close $out;
}
print STDERR "tokens: $hits replacement(s) in $files file(s)" . ($ENV{DRY} ? " (dry run)" : "") . "\n";
EOF

cat > "$T/prose.pl" <<'"'"'EOF'"'"'
#!/usr/bin/perl
# prose.pl FILE... : ADR-0129 family (a) prose rules, in place. PROSE_MODE=safe
# applies only the unambiguous phrase rules (rows 1, 4, 5, 6, "epoch id",
# "epoch record", the report-line placeholders). PROSE_MODE=full (the default) also
# maps every remaining "run epoch" / "epoch" to "run" (the Decision 4 default sense
# for code comments). ARM=1 adds the row-2 arm -> start rules; pass it ONLY for
# files where every "arm" belongs to the run tracker. Protected spellings are masked first
# and restored last, so no rule can touch them.
use strict; use warnings;
my $full = ($ENV{PROSE_MODE} // "full") eq "full";
my @protect = ("Unix epoch", "unix epoch", "epoch seconds", "source epoch", "source-epoch",
  "SOURCE_DATE_EPOCH", "gate_context_hash", "dispatch_context",
  "DOCKET_AGENT_GUARDIAN_GATE_KEY", "epoch.json", "epoch.lock", "rungate");
my @rules = (
  [qr/<epoch>/, "<run-id>"], [qr/<dispatch-context>/, "<run-context>"],
  [qr/\bRun epoch id(s?)\b/, "Run id%1"], [qr/\brun epoch id(s?)\b/, "run id%1"],
  [qr/\bEpoch id(s?)\b/, "Run id%1"], [qr/\bepoch id(s?)\b/, "run id%1"],
  [qr/\bEpoch record(s?)\b/, "Run record%1"], [qr/\bepoch record(s?)\b/, "run record%1"],
  [qr/\bEpoch fence(s?)\b/, "Run fence%1"], [qr/\bepoch fence(s?)\b/, "run fence%1"],
  [qr/\b[Ee]poch-?less\b/, "no-run-record"],
  [qr/\bRun gates\b/, "Run trackers"], [qr/\brun gates\b/, "run trackers"],
  [qr/\bRun gate(?![\w-])/, "Run tracker"], [qr/\brun gate(?![\w-])/, "run tracker"],
  [qr/(?<![\w\/-])Run-gate(?![\w.-])/, "Run-tracker"], [qr/(?<![\w\/-])run-gate(?![\w.-])/, "run-tracker"],
  [qr/\bGate facade\b/, "Run tracker"], [qr/\bgate facade\b/, "run tracker"],
  [qr/\bGate key(s?)\b/, "Run key%1"], [qr/\bgate key(s?)\b/, "run key%1"],
  [qr/(?<![\w-])gate-key(?![\w-])/, "run-key"],
  [qr/\bDispatch context(s?)\b/, "Run context%1"], [qr/\bdispatch context(s?)\b/, "run context%1"],
  [qr/(?<![\w-])dispatch-context(?![\w-])/, "run-context"],
);
my @full_rules = (
  [qr/(?<![\w-])Run-epoch(?![\w-])/, "Run"], [qr/(?<![\w-])run-epoch(?![\w-])/, "run"],
  [qr/\bRun epochs\b/, "Runs"], [qr/\brun epochs\b/, "runs"],
  [qr/\bRun epoch\b/, "Run"], [qr/\brun epoch\b/, "run"],
  [qr/\bAn epoch\b/, "A run"], [qr/\ban epoch\b/, "a run"],
  [qr/(?<![A-Za-z0-9_])Epochs(?![A-Za-z0-9_])/, "Runs"], [qr/(?<![A-Za-z0-9_])epochs(?![A-Za-z0-9_])/, "runs"],
  [qr/(?<![A-Za-z0-9_])Epoch(?![A-Za-z0-9_])/, "Run"], [qr/(?<![A-Za-z0-9_])epoch(?![A-Za-z0-9_])/, "run"],
);
my @arm_rules = (
  [qr/(?<![\w-])re-arming(?![\w-])/, "restarting"], [qr/(?<![\w-])re-armed(?![\w-])/, "restarted"],
  [qr/(?<![\w-])re-arms(?![\w-])/, "restarts"], [qr/(?<![\w-])re-arm(?![\w-])/, "restart"],
  [qr/(?<![\w-])Unarmed(?![\w-])/, "Untracked"], [qr/(?<![\w-])unarmed(?![\w-])/, "untracked"],
  [qr/(?<![\w-])Arming(?![\w-])/, "Starting"], [qr/(?<![\w-])arming(?![\w-])/, "starting"],
  [qr/(?<![\w-])Armed(?![\w-])/, "Started"], [qr/(?<![\w-])armed(?![\w-])/, "started"],
  [qr/(?<![\w-])Arms(?![\w-])/, "Starts"], [qr/(?<![\w-])arms(?![\w-])/, "starts"],
  [qr/(?<![\w-])Arm(?![\w-])/, "Start"], [qr/(?<![\w-])arm(?![\w-])/, "start"],
);
my @all = (@rules, ($full ? @full_rules : ()), ($ENV{ARM} ? @arm_rules : ()),
  [qr/\ban run\b/, "a run"], [qr/\bAn run\b/, "A run"]);
my $changed = 0;
for my $file (@ARGV) {
  open my $in, "<", $file or die "read $file: $!"; local $/; my $s = <$in>; close $in;
  my $orig = $s; my @saved;
  for my $p (@protect) { my $q = quotemeta $p;
    $s =~ s/(?<![A-Za-z0-9_])$q/push @saved, $p; "\x01P" . $#saved . "\x01"/ge; }
  for my $r (@all) { my ($re, $rep) = @$r;
    $s =~ s/$re/my $g = defined $1 ? $1 : ""; (my $x = $rep) =~ s{%1}{$g}; $x/ge; }
  $s =~ s/\x01P(\d+)\x01/$saved[$1]/g;
  next if $s eq $orig;
  open my $out, ">", $file or die "write $file: $!"; print $out $s; close $out;
  $changed++;
}
print STDERR "prose: rewrote $changed file(s)\n";
EOF

cat > "$T/protected.sh" <<'"'"'EOF'"'"'
#!/usr/bin/env bash
# protected.sh : every kept namesake must keep its base-commit count.
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
BASE=e68669728ed736d5b5195a0807e5734fd6e3f81e; bad=0
check(){ local tok="$1"; shift
  local b n; b=$(command git grep -o -F -e "$tok" "$BASE" -- "$@" | wc -l | tr -d " ")
  n=$(command git grep -o -F -e "$tok" -- "$@" | wc -l | tr -d " ")
  if [ "$b" = "$n" ]; then echo "ok   $tok ($n) in $*"; else echo "DIFF $tok base=$b now=$n in $*"; bad=1; fi; }
check gate_context_hash internal/app/change_claim.go internal/app/claim_proof.go
check gate_context_hash internal/gatedrive/testdata
check source-epoch cmd/releasepkg
check gate-failed .
check gate-stopped .
check gate-scope .
check idempotent-suite-gate .
check TestIntegrationGateLifecycle .
check gatelifecycle .
check DOCKET_AGENT_GUARDIAN_GATE_KEY .
check dispatch_context .
check "\"gate-context\"" internal/cli/gate.go
exit $bad
EOF

chmod +x "$T/fileset.sh" "$T/scan.sh" "$T/protected.sh"
echo "tools installed in $T"
'
```

Positive control for `scan.sh`: before trusting an empty scan, run `"$T/scan.sh" 'implement-next'` once; it must print many lines.

**`AGENTS.md` regeneration helper.** Run it only after `go generate ./internal/assets/`, because it renders from the embedded catalog exactly as `repoguard.TestCommittedCodexDispatchMatchesGenerator` does. It lives inside the module for one command and is deleted in the same command:

```bash
bash -c '
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
mkdir -p cmd/zz-0471-regen-agents
cat > cmd/zz-0471-regen-agents/main.go <<'"'"'EOF'"'"'
// Temporary change-0471 helper, never committed: rewrites AGENTS.md'"'"'s managed
// dispatch block from the embedded run-tracker payload.
package main

import (
	"fmt"
	"os"
	"path"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
)

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "regen-agents:", err)
		os.Exit(1)
	}
}

func main() {
	cat, err := assets.EmbeddedCatalog()
	check(err)
	var payload []byte
	for _, e := range cat.EntriesByRole(assets.RoleDispatch) {
		if b := path.Base(e.Path); b == "run-gate.md" || b == "run-tracker.md" {
			payload, err = cat.Bytes(e.Path)
			check(err)
		}
	}
	if payload == nil {
		check(fmt.Errorf("no run-tracker dispatch payload in the embedded catalog"))
	}
	src, err := os.ReadFile("AGENTS.md")
	check(err)
	doc, err := document.Parse(src)
	check(err)
	var patch document.PatchSet
	patch.ReplaceBlock("dispatch", harness.CodexDispatchInterior(payload))
	out, err := doc.Apply(patch)
	check(err)
	check(os.WriteFile("AGENTS.md", out, 0o644))
}
EOF
go run ./cmd/zz-0471-regen-agents; rc=$?
rm -rf cmd/zz-0471-regen-agents
exit $rc
'
```

Verify with `go test -count=1 ./internal/repoguard/ -run 'TestCommittedCodexDispatch'`, and confirm `command git status --porcelain -- cmd/` prints nothing.

## Review Focus

1. **A coordinator or consumer repo still sends an old spelling after the merge** (`run gate-before`, `--run-epoch`, `run cancel --epoch`, `change claim --gate-context`). A person expects a loud refusal (unknown command or unknown flag, exit 2), never a silent alias or an ignored flag. Pinned by `TestRunTrackerVocabularyHardCut` in Task 2.
2. **A repository still holds the retired storage roots at upgrade** (an `active` run record under `rungate/`, a `gate-admission/v1` slot owned by that run, a `rungate-resume` lock). A person expects the new binary to start a fresh run in that worktree, ignore the old files, and leave them byte-for-byte untouched. Pinned by `TestIntegrationGateArmStorageResetIgnoresRetiredRoots` and `TestRunTrackerResetIgnoresRetiredAdmissionSlot` in Task 1.
3. **A committed claim receipt written before the rename** (key `gate_context_hash`). A person expects it to decode, and claim proof to still find it, after the Go field is renamed. Pinned by `TestClaimReceiptKeepsCommittedGateContextHashKey` in Task 1, whose tag mutation reddens it, and re-run after Task 5's identifier sweep.
4. **A kept namesake renamed by accident**: the gate drive's `--gate-context`, `gate-failed` / `gate-stopped`, `gate-scope`, `gate_context_hash`, `--source-epoch`, `DOCKET_AGENT_GUARDIAN_GATE_KEY`, the `dispatch_context` key, the `gatelifecycle` shard. The binary and consumers expect them unchanged. Pinned by `"$T/protected.sh"` (Tasks 2–10) and by the seal's negative controls in Task 9.
5. **An integration shard silently drops tests after its prefix rename.** A person expects every renamed shard to run exactly as many tests as its old shard did. Pinned by Task 4's before/after `go test -list` counts and `tests/test_go_integration_contract.sh`.

---

### Task 1: Reset the run tracker's local storage to new roots and keys

**Build profile:** premium

**Files:**
- Create: `internal/gatedrive/storage_reset_test.go`
- Create: `internal/app/runtracker_storage_reset_integration_test.go`
- Create: `internal/app/claim_receipt_key_test.go`
- Modify: `internal/app/rungate_store.go` (new root constants; `dispatch_epoch` tag; every `"rungate"` join)
- Modify: `internal/app/rungate_epoch.go` (record and lock file names; `gate_key` / `epoch_id` tags; joins; layout comments)
- Modify: every other non-test Go file that joins `"rungate"` or `"rungate-resume"`. At base these are `internal/app/rungate_before.go`, `rungate_epoch_refusal.go`, `rungate_fence.go`, `rungate_gate.go`, `agent_guardian.go` and `gate_drive.go`; re-derive them in Step 3.
- Modify: `internal/gatedrive/store.go` (three roots `v1` → `v2`), `internal/gatedrive/admission.go` (explicit JSON tags), `internal/gatedrive/scope.go` and `internal/gatedrive/drive.go` (keys)
- Modify: tests that spell the old storage paths or keys (derived by grep in Step 4)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: constants `runTrackerDirName = "run-tracker"` and `runTrackerResumeDirName = "run-tracker-resume"` in package `app`; the record file `run.json` and lock `run.lock`; persisted keys `run_id`, `run_key`, `dispatched_at` (app) and `run_id`, `run_context_hash` (gatedrive); `gate-drives/v2`, `gate-scopes/v2`, `gate-admission/v2`. The two storage-reset test files are excluded from every later textual pass (`fileset.sh` already excludes them); Task 4 renames their test prefix and Task 5 renames their identifiers with the AST-only tool.

- [ ] **Step 1: Write the failing tests**

`internal/gatedrive/storage_reset_test.go`:

```go
package gatedrive

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// The run tracker's gatedrive stores were renamed by RESET, not migrated
// (ADR-0129 row 38 and Decision 3, change 0471): each store whose persisted names
// changed moved to a new root, so the binary never reads retired state. This file
// is excluded from change 0471's textual rename passes because its fixtures spell
// the RETIRED layout on purpose.

func TestRunTrackerResetStoreRoots(t *testing.T) {
	common := testsupport.TempDir(t)
	s := OpenStore(common)
	for name, c := range map[string]struct{ got, want string }{
		"drives":    {s.root, filepath.Join(common, "docket", "gate-drives", "v2")},
		"scopes":    {s.scopeRoot, filepath.Join(common, "docket", "gate-scopes", "v2")},
		"admission": {s.admissionRoot, filepath.Join(common, "docket", "gate-admission", "v2")},
		// The suite-budget store carries no retired key, so it keeps v1.
		"budgets": {s.suiteBudgetRoot, filepath.Join(common, "docket", "gate-suite-budgets", "v1")},
	} {
		if c.got != c.want {
			t.Errorf("%s root = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestRunTrackerResetIgnoresRetiredAdmissionSlot(t *testing.T) {
	common := testsupport.TempDir(t)
	s := OpenStore(common)
	wt := mkWorktree(t)
	canonical, err := filepath.EvalSymlinks(wt)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	// A busy v1 slot, exactly as the pre-0471 binary wrote it: untagged Go field
	// names, owned by a run the new store has never heard of.
	retiredPath := filepath.Join(common, "docket", "gate-admission", "v1", admissionKey(canonical), recordFileName)
	retired := []byte(`{"Generation":"retired-gen","Record":{"SchemaVersion":1,"RepoIdentity":"repo-1","WorktreeRoot":` +
		strconv.Quote(canonical) + `,"State":"executing","ExecutionGen":3,"ReservationToken":"retired-token","RunEpochID":"retired-run","Kind":"scopeless"}}`)
	if err := os.MkdirAll(filepath.Dir(retiredPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(retiredPath, retired, 0o600); err != nil {
		t.Fatal(err)
	}

	token, err := s.ReserveWorktreeExecutionForEpoch("repo-1", wt, "new-run", nil)
	if err != nil || token == "" {
		t.Fatalf("a reservation over a retired v1 slot must be admitted: token=%q err=%v", token, err)
	}
	slot, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if slot.RunEpochID != "new-run" || slot.State != admissionReserved {
		t.Fatalf("slot = (%q, %q), want (new-run, reserved)", slot.RunEpochID, slot.State)
	}
	after, err := os.ReadFile(retiredPath)
	if err != nil || !bytes.Equal(after, retired) {
		t.Fatalf("the retired v1 slot must stay byte-for-byte untouched (err=%v)", err)
	}
}

// TestAdmissionRecordFieldsCarryExplicitJSONTags: the admission record once had
// no tags, so a Go rename silently changed its on-disk key. Every field now names
// its key explicitly.
func TestAdmissionRecordFieldsCarryExplicitJSONTags(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(admissionRecord{}), reflect.TypeOf(storedAdmission{})} {
		seen := map[string]string{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag, ok := f.Tag.Lookup("json")
			name := strings.Split(tag, ",")[0]
			if !ok || name == "" || name == "-" {
				t.Errorf("%s.%s has no explicit json key (tag %q)", typ.Name(), f.Name, tag)
				continue
			}
			if prev, dup := seen[name]; dup {
				t.Errorf("%s: json key %q used by both %s and %s", typ.Name(), name, prev, f.Name)
			}
			seen[name] = f.Name
		}
	}
}

func TestAdmissionRecordPersistsRunIDKey(t *testing.T) {
	b, err := json.Marshal(storedAdmission{Generation: "g", Record: admissionRecord{RunEpochID: "run-1", State: admissionReserved}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"generation":"g"`, `"record":{`, `"run_id":"run-1"`, `"state":"reserved"`} {
		if !strings.Contains(s, want) {
			t.Errorf("persisted slot %s lacks %s", s, want)
		}
	}
	for _, gone := range []string{`"RunEpochID"`, `"Generation"`, `"Record"`} {
		if strings.Contains(s, gone) {
			t.Errorf("persisted slot %s still carries the implicit key %s", s, gone)
		}
	}
}
```

`internal/app/runtracker_storage_reset_integration_test.go`:

```go
//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationGateArmStorageResetIgnoresRetiredRoots pins ADR-0129 Decision 3
// (change 0471): the run tracker's local stores were renamed by RESET, not
// migrated. A repository still holding the retired roots (an ACTIVE run record
// under rungate/<key>/epoch.json bound to change 5 and this worktree, plus a
// resume lock under rungate-resume/5/epoch.lock) is invisible to the new binary:
// a fresh start mints under run-tracker/<key>/, no lookup finds the retired
// record, and the retired files stay byte-for-byte untouched. This file is
// excluded from change 0471's textual rename passes because its fixtures spell
// the RETIRED layout on purpose.
func TestIntegrationGateArmStorageResetIgnoresRetiredRoots(t *testing.T) {
	repo := newGateRepo(t)
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	canon, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	writeRetired := func(path string, content []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	retiredRecord, err := json.Marshal(map[string]any{
		"generation": "retired-gen",
		"record": map[string]any{
			"schema_version": epochSchemaVersion,
			"gate_key":       "retired-key",
			"change_id":      "5",
			"worktree":       canon,
			"state":          string(EpochActive),
			"epoch_id":       "retired-run-id",
			"created_at":     "2026-09-01T00:00:00Z",
			"updated_at":     "2026-09-01T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	retiredRecordPath := filepath.Join(common, "docket", "rungate", "retired-key", "epoch.json")
	retiredLockPath := filepath.Join(common, "docket", "rungate-resume", "5", "epoch.lock")
	writeRetired(retiredRecordPath, retiredRecord)
	writeRetired(retiredLockPath, nil)

	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	res := RunGateBefore(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Armed || res.Key == "" {
		t.Fatalf("a fresh start over the retired roots must succeed: Armed=%v Key=%q reason=%q", res.Armed, res.Key, res.Reason)
	}

	keyDir := filepath.Join(common, "docket", "run-tracker", res.Key)
	runRecord, err := os.ReadFile(filepath.Join(keyDir, "run.json"))
	if err != nil {
		t.Fatalf("the new run record must live at run-tracker/<key>/run.json: %v", err)
	}
	trackerRecord, err := os.ReadFile(filepath.Join(keyDir, "record.json"))
	if err != nil {
		t.Fatalf("the tracker record must live beside it: %v", err)
	}
	for _, c := range []struct {
		doc, want, gone string
	}{
		{string(runRecord), `"run_id":`, `"epoch_id"`},
		{string(runRecord), `"run_key":`, `"gate_key"`},
		{string(trackerRecord), `"dispatched_at":`, `"dispatch_epoch"`},
	} {
		if !strings.Contains(c.doc, c.want) || strings.Contains(c.doc, c.gone) {
			t.Errorf("record %s: want key %s and no %s", c.doc, c.want, c.gone)
		}
	}

	if _, _, found, err := FindEpochByChange(repo, "5"); err != nil || found {
		t.Fatalf("the retired record must be invisible by change: found=%v err=%v", found, err)
	}
	if _, found, err := findEpochByWorktree(repo, canon); err != nil || found {
		t.Fatalf("the retired record must be invisible by worktree: found=%v err=%v", found, err)
	}
	if got, err := os.ReadFile(retiredRecordPath); err != nil || !bytes.Equal(got, retiredRecord) {
		t.Fatalf("the retired run record must stay byte-for-byte untouched (err=%v)", err)
	}
	if got, err := os.ReadFile(retiredLockPath); err != nil || len(got) != 0 {
		t.Fatalf("the retired resume lock must stay untouched (err=%v, %d bytes)", err, len(got))
	}
}
```

`internal/app/claim_receipt_key_test.go`:

```go
package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClaimReceiptKeepsCommittedGateContextHashKey pins ADR-0129 Decision 3
// (change 0471): committed state keeps its spelling. Claim receipts on the docket
// branch carry gate_context_hash, and the claim idempotency digest hashes that key
// name, so the Go fields may be renamed but the JSON key may not.
func TestClaimReceiptKeepsCommittedGateContextHashKey(t *testing.T) {
	const raw = `{"gate_context_hash":"hash-1"}`
	var receipt changeClaimReceipt
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil || receipt.GateContextHash != "hash-1" {
		t.Fatalf("changeClaimReceipt decode = %q (err %v), want hash-1", receipt.GateContextHash, err)
	}
	var proof claimProofReceipt
	if err := json.Unmarshal([]byte(raw), &proof); err != nil || proof.GateContextHash != "hash-1" {
		t.Fatalf("claimProofReceipt decode = %q (err %v), want hash-1", proof.GateContextHash, err)
	}
	var digest claimDigestPayload
	if err := json.Unmarshal([]byte(raw), &digest); err != nil || digest.GateContextHash != "hash-1" {
		t.Fatalf("claimDigestPayload decode = %q (err %v), want hash-1", digest.GateContextHash, err)
	}
	for name, v := range map[string]any{
		"changeClaimReceipt": changeClaimReceipt{GateContextHash: "hash-1"},
		"claimDigestPayload": claimDigestPayload{GateContextHash: "hash-1"},
		"claimProofReceipt":  claimProofReceipt{GateContextHash: "hash-1"},
	} {
		b, err := json.Marshal(v)
		if err != nil || !strings.Contains(string(b), `"gate_context_hash":"hash-1"`) {
			t.Errorf("%s marshals %s (err %v), want the committed gate_context_hash key", name, b, err)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
go test -count=1 ./internal/gatedrive/ -run 'TestRunTrackerReset|TestAdmissionRecord'
go test -count=1 -tags integration ./internal/app/ -run 'TestIntegrationGateArmStorageResetIgnoresRetiredRoots'
go test -count=1 ./internal/app/ -run 'TestClaimReceiptKeepsCommittedGateContextHashKey'
```
Expected: the store-roots test fails naming `v1` roots. The tag guard fails for every `admissionRecord` field. The persisted-key test fails on `"run_id"`. The retired-slot test fails with a worktree-busy refusal. The app storage-reset test fails, either on the `run-tracker/<key>/run.json` read or on `FindEpochByChange` finding the retired record. The claim-receipt test **passes**: it is a characterization pin for Task 5, and Step 6 proves it bites.

- [ ] **Step 3: Implement the storage reset**

In `internal/app/rungate_store.go`, after the `gateRecordFileName` constant block, add:

```go
// The run tracker's local storage roots under <git-common-dir>/docket/
// (ADR-0129 row 38, change 0471). They were renamed by RESET, not migrated: the
// binary never reads the retired rungate/ and rungate-resume/ roots, which stay
// inert on disk and may be deleted by hand.
const (
	runTrackerDirName       = "run-tracker"
	runTrackerResumeDirName = "run-tracker-resume"
)
```

Replace every `"rungate"` path element with `runTrackerDirName`, and every `"rungate-resume"` with `runTrackerResumeDirName`. Derive the sites (never hand-list them):

```bash
bash -c 'cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run && command git grep -n -F -e "\"rungate\"" -e "\"rungate-resume\"" -- "*.go"'
```

Non-test sites use the constant. Tests in package `app` use the constant. Tests in other packages (for example `internal/cli/gate_test.go`) use the literal `"run-tracker"` and `"run.json"`. The file `internal/app/runtracker_storage_reset_integration_test.go` keeps its retired literals on purpose.

In `internal/app/rungate_epoch.go`: set `epochRecordFileName = "run.json"` and `epochLockFileName = "run.lock"`. In `EpochRecord`, change the tags to `json:"run_key"` (field `GateKey`) and `json:"run_id"` (field `EpochID`). In `internal/app/rungate_store.go` `GateRecord`, change `DispatchEpoch`'s tag to `json:"dispatched_at"`, and update its comment: it is a Unix timestamp. Keep every record-level `schema` / `SchemaVersion` value unchanged.

In `internal/gatedrive/store.go`, change the three roots to `v2` and keep the suite-budget root at `v1`:

```go
		root:            filepath.Join(gitCommonDir, "docket", "gate-drives", "v2"),
		scopeRoot:       filepath.Join(gitCommonDir, "docket", "gate-scopes", "v2"),
		suiteBudgetRoot: filepath.Join(gitCommonDir, "docket", "gate-suite-budgets", "v1"),
		admissionRoot:   filepath.Join(gitCommonDir, "docket", "gate-admission", "v2"),
```

In `internal/gatedrive/admission.go`, give every field an explicit tag. Keep the existing field comments; they are omitted here only for brevity. Replace "Like admissionRecord it carries no json tags." with "Like admissionRecord it names every JSON key explicitly (change 0471)."

```go
type admissionRecord struct {
	SchemaVersion int            `json:"schema_version"`
	RepoIdentity  string         `json:"repo_identity"`
	WorktreeRoot  string         `json:"worktree_root"`
	State         admissionState `json:"state"`
	ExecutionGen  int            `json:"execution_gen"`

	ReservationToken string `json:"reservation_token"`

	DriveID    string `json:"drive_id"`
	ScopeID    string `json:"scope_id"`
	RunEpochID string `json:"run_id"`

	RawRunID  string `json:"raw_run_id"`
	RawRunDir string `json:"raw_run_dir"`

	Kind string `json:"kind"`

	LegacyInventoried bool      `json:"legacy_inventoried"`
	LegacyInventoryAt time.Time `json:"legacy_inventory_at"`

	ReservedAt time.Time `json:"reserved_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type storedAdmission struct {
	Generation string          `json:"generation"`
	Record     admissionRecord `json:"record"`
}
```

In `internal/gatedrive/scope.go`: `GateContextHash` → tag `json:"run_context_hash,omitempty"`; `RunEpochID` → tag `json:"run_id,omitempty"`. In `internal/gatedrive/drive.go`: `GateContextHash` → tag `json:"run_context_hash,omitempty"`. Do **not** touch the claim receipts' `gate_context_hash` tags in `internal/app/change_claim.go` or `internal/app/claim_proof.go`.

Update the layout comments these edits falsify (the `WHERE:` header of `rungate_epoch.go`, the store-root comments in `store.go`, `admission.go`, `scope.go`): find them with `command git grep -n -E -e 'rungate/|epoch\.json|epoch\.lock|/v1/' -- internal/app/rungate_*.go internal/app/agent_guardian.go internal/gatedrive/*.go ':!*_test.go'`.

Finally, check every other persisted struct in these stores for a retired key the table above misses: `command git grep -n -E -e 'json:"[^"]*(epoch|gate_key|gate_context)[^"]*"' -- 'internal/app/*.go' 'internal/gatedrive/*.go' ':!*_test.go'`. Expected: only the claim receipts' `gate_context_hash` (kept) and the change-claim request key `gate_context` (a wire key, renamed in Task 2). Anything else is a store the spec says must move to a new root. Report it and stop rather than guessing.

- [ ] **Step 4: Update test fixtures that spell the old layout or keys**

Derive the sites:

```bash
bash -c 'cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run && command git grep -n -E -e "\"gate-(drives|scopes|admission)\", \"v1\"" -e "dispatch_epoch|run_epoch_id|\"epoch_id\"|epoch_id\\\\?\"|\"gate_key\"" -e "epoch\.json" -e "\{\"Generation\"" -- "*_test.go" ":!internal/gatedrive/storage_reset_test.go" ":!internal/app/runtracker_storage_reset_integration_test.go"'
bash -c 'cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run && command git grep -n -F -e gate_context_hash -- "*_test.go"'
```

Rules:
- `"gate-drives", "v1"`, `"gate-scopes", "v1"`, `"gate-admission", "v1"` become `"v2"`.
- Raw JSON fixtures for the run record, tracker record, scope or drive take the new keys (`run_id`, `run_key`, `dispatched_at`, `run_context_hash`).
- A raw admission document takes the new snake-case keys. For example, `internal/gatedrive/admission_test.go`'s schema-refusal seed becomes `{"generation":"x","record":{"schema_version":99}}`. Go's case-insensitive decode would otherwise read `"Generation"`, but never `"SchemaVersion"`, so the test would pass for the wrong reason.
- `gate_context_hash` in `internal/gatedrive/*_test.go` names a drive or scope record and becomes `run_context_hash`. In `internal/app/*_test.go`, it stays wherever it names a **claim receipt**. Classify each hit by the struct or document it builds.
- `internal/gatedrive/testdata/**` is frozen: never edit it.

- [ ] **Step 5: Run the focused tests to verify they pass**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
gofmt -l internal/ cmd/
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/gatedrive/ ./internal/app/ ./internal/cli/ ./cmd/docket/
go test -count=1 -tags integration -timeout 30m ./internal/app/ ./internal/gatedrive/ ./internal/cli/
```
Expected: `gofmt -l` prints nothing, and every package reports `ok`. `TestIntegrationGateArmEpochlessResumeEndToEnd0382` stays green: a change `in-progress` with no run record still resumes through the change-0463 path.

- [ ] **Step 6: Mutation-test the three new guards**

For each probe: back up, mutate, confirm the mutation landed, run with `-count=1`, see RED, restore from the backup, then re-run and see GREEN.
- (a) Delete the `json:"run_id"` tag from `admissionRecord.RunEpochID` in `internal/gatedrive/admission.go`. `TestAdmissionRecordFieldsCarryExplicitJSONTags` and `TestAdmissionRecordPersistsRunIDKey` go red.
- (b) Change `claimDigestPayload`'s tag to `json:"run_context_hash"` in `internal/app/change_claim.go`. `TestClaimReceiptKeepsCommittedGateContextHashKey` goes red.
- (c) Set `runTrackerDirName = "rungate"`. `TestIntegrationGateArmStorageResetIgnoresRetiredRoots` goes red (with `-tags integration`).

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
command git add internal/gatedrive/storage_reset_test.go internal/app/runtracker_storage_reset_integration_test.go internal/app/claim_receipt_key_test.go <each modified path, explicitly>
command git commit -m "feat: reset run-tracker storage to new roots and keys (change 0471)"
```

### Task 2: Hard-cut the operation ids, CLI verbs, flags, keys and env var (rows 7–12, 13's JSON key, 38a–38d)

**Build profile:** standard

**Files:**
- Create: `internal/cli/run_tracker_rename_test.go`
- Modify: `internal/cli/run.go`, `internal/cli/agent.go`, `internal/cli/gate.go`, `internal/cli/change.go`, `internal/cli/change_test.go`, `internal/cli/install.go`
- Modify: `internal/app/rungate_before.go` (result tags), `internal/app/schema_registry.go`, `internal/app/change_claim.go`, `internal/app/agent_guardian.go`, `internal/harness/dispatch.go`
- Modify: every other maintained site of the Task 2 tokens (derived by `tokens.pl` over `fileset.sh`), `cursor-rules/run-gate.md`, skills, docs
- Regenerate: `internal/assets/embedded/**`, `internal/harness/*/testdata/golden/*`, the `AGENTS.md` dispatch block

**Interfaces:**
- Consumes: Task 1's storage (unaffected here).
- Produces: operation ids `run.start`, `run.verdict`, `run.continue`; CLI `docket run start|verdict|continue`; flags `--run-id` (agent enter, gate drive start, gate drive prepare-scope, run cancel), `--run-context` (change claim), `--run-key` (agent enter); the `run.start` result keys `started` and `run_id`; the `change.claim` request key `run_context`; env `DOCKET_AGENT_GUARDIAN_RUN_ID`. Go identifiers keep their old names until Task 5.

- [ ] **Step 1: Install the shared tools** (Global Constraints → Shared tooling).

- [ ] **Step 2: Write the failing test** — `internal/cli/run_tracker_rename_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestRunTrackerVocabularyHardCut pins ADR-0129 Decision 2 for family (a)
// (change 0471): the renamed verbs and flags exist and the retired spellings are
// gone. No alias, no hidden flag.
func TestRunTrackerVocabularyHardCut(t *testing.T) {
	root := captureTree(t)
	find := func(path ...string) *cobra.Command {
		t.Helper()
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		return cmd
	}
	names := map[string]bool{}
	for _, c := range find("run").Commands() {
		names[c.Name()] = true
		for _, a := range c.Aliases {
			names[a] = true
		}
	}
	for _, want := range []string{"start", "verdict", "continue", "cancel", "verify"} {
		if !names[want] {
			t.Errorf("docket run %s is missing", want)
		}
	}
	for _, gone := range []string{"gate-before", "gate-verdict", "gate-claim"} {
		if names[gone] {
			t.Errorf("docket run %s survives the hard cut", gone)
		}
	}
	for _, f := range []struct {
		path       []string
		want, gone string
	}{
		{[]string{"run", "cancel"}, "run-id", "epoch"},
		{[]string{"change", "claim"}, "run-context", "gate-context"},
		{[]string{"agent", "enter"}, "run-key", "run-gate-key"},
		{[]string{"agent", "enter"}, "run-id", "run-epoch"},
		{[]string{"gate", "drive", "start"}, "run-id", "run-epoch"},
		{[]string{"gate", "drive", "prepare-scope"}, "run-id", "run-epoch"},
	} {
		cmd := find(f.path...)
		if cmd.Flags().Lookup(f.want) == nil {
			t.Errorf("%v lacks --%s", f.path, f.want)
		}
		if cmd.Flags().Lookup(f.gone) != nil {
			t.Errorf("%v still registers the retired --%s", f.path, f.gone)
		}
	}
	// The gate drive's own --gate-context is not an ADR-0129 row and stays.
	for _, p := range [][]string{{"gate", "drive", "start"}, {"gate", "drive", "prepare-scope"}} {
		if find(p...).Flags().Lookup("gate-context") == nil {
			t.Errorf("%v must keep its --gate-context flag", p)
		}
	}
	// An old spelling is refused at the command line (exit 2), not ignored.
	if _, _, code := runCLI(t, "--json", "run", "gate-before", "implement-next"); code != 2 {
		t.Errorf("docket run gate-before exited %d, want 2 (unknown command)", code)
	}
	if _, _, code := runCLI(t, "--json", "run", "cancel", "--key", "k", "--epoch", "e", "--reason", "r"); code != 2 {
		t.Errorf("docket run cancel --epoch exited %d, want 2 (unknown flag)", code)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test -count=1 ./internal/cli/ -run TestRunTrackerVocabularyHardCut`
Expected: FAIL, listing the missing `start` / `verdict` / `continue` verbs and the missing flags.

- [ ] **Step 4: Hand-edit the sites a bulk token pass cannot express**

Do these **before** the bulk pass:
- `internal/cli/run.go`:
  - `Use: "start <target>"`, `Short: "Start a tracked run for a dispatched workflow and print run-started <key> <run-id> <run-context>"`.
  - `Use: "verdict <key> | --unattributed [<id>...]"`, `Short: "Report the run tracker's verdict for a dispatched workflow (attributed or observe-only)"`.
  - `Use: "continue <key> <continuation-id>"`.
  - The cancel command: `Short: "Cancel a dispatched run: fence it, tear it down, and report the disposition"`; `GetString("run-id")`; `cancel.Flags().String("run-id", "", "expected run `id` (required)")`; `MarkFlagRequired("run-id")`; the `key` flag usage becomes "durable run `key` locating the run to cancel (required)".
- `internal/cli/agent.go`: `StringVar(&runGateKey, "run-key", "", "run `key` for lifecycle registration (optional; locator, not a credential)")` and `StringVar(&runEpoch, "run-id", "", "run `id` for lifecycle registration (optional; public locator, not a credential)")`.
- `internal/cli/gate.go`: the two `run-epoch` flag usages become "workflow run `id` recorded on the worktree slot (a locator, not a credential)" and "workflow run `id` every drive under this scope carries; makes the takeover run-revocation gate live (a locator, not a credential)". The flag names themselves change in the bulk pass.
- `internal/cli/change.go`: `req.GateContext, _ = c.Flags().GetString("run-context")` and `claim.Flags().String("run-context", "", "run-context `token` from run start, binding this claim to its started run (optional; omitted for an untracked claim)")`. In `internal/cli/change_test.go` `TestChangeClaimGateContextFlag`, look up `run-context` (both assertions and messages).
- `internal/app/rungate_before.go`: `Armed bool `json:"started"`` and `Epoch string `json:"run_id,omitempty"``. In `internal/app/rungate_before_resume_integration_test.go`, the result-JSON assertion `"epoch":"` becomes `"run_id":"`. Find every other `"armed"` / `"epoch":` JSON assertion with `command git grep -n -E -e '\\?"armed\\?"|\\?"epoch\\?":' -- '*.go'`.
- **Row 12, change-claim sites only.** List every `--gate-context` / `"gate-context"` site: `command git grep -n -F -e gate-context -- . ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs' ':!internal/assets/embedded'`. Rename only where the command is `change claim` / `change.claim`; keep every `gate drive start` / `gate drive prepare-scope` / `gate.drive.*` site, and every `gate-context-invalid` / `gate-context-conflict` (Task 3). At base the change-claim sites are:
  - `internal/cli/change.go` and `change_test.go` (above);
  - `docs/reference/glossary.md` (the `change.claim … --gate-context` sentences and the `docket change claim … --gate-context` example);
  - `skills/docket-implement-next/SKILL.md` (the `change.claim` step);
  - `internal/repoguard/prose_contracts_test.go` ("pass it to the claim as --gate-context");
  - `internal/harness/dispatch.go` `CodexRootEntryClause`: "labeled for `change.claim --gate-context` and gate-drive operations" becomes "labeled for `change.claim --run-context` and gate-drive `--gate-context`";
  - the matching `good` / `old` pin strings in `internal/repoguard/gatedrive_run_epoch_thread_test.go`.
- `cursor-rules/run-gate.md`: nothing by hand. The bulk pass renames its op ids and flags.

- [ ] **Step 5: Run the bulk token pass**

```bash
bash -c '
T="${TMPDIR:-/tmp}/docket-0471"
printf "%s\t%s\n" \
  run.gate-before run.start  run.gate-verdict run.verdict  run.gate-claim run.continue \
  "run gate-before" "run start"  "run gate-verdict" "run verdict"  "run gate-claim" "run continue" \
  gate-before "run start"  gate-verdict "run verdict"  gate-claim "run continue" \
  --run-epoch --run-id  "\"run-epoch\"" "\"run-id\""  --run-gate-key --run-key  "\"run-gate-key\"" "\"run-key\"" \
  --epoch --run-id  gate_context run_context  DOCKET_AGENT_GUARDIAN_EPOCH DOCKET_AGENT_GUARDIAN_RUN_ID \
  > "$T/task2.tsv"
cat "$T/task2.tsv"
"$T/fileset.sh" . | DRY=1 xargs -0 perl "$T/tokens.pl" "$T/task2.tsv"
'
```
Read the dry-run file list. Every file should be a run-tracker site: Go, skills, `cursor-rules/run-gate.md`, docs, repoguard pins, CLI tests. Then run the same command without `DRY=1`.

- [ ] **Step 6: Regenerate and verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
go generate ./internal/assets/
go test -count=1 ./internal/harness/claude/ ./internal/harness/codex/ ./internal/harness/cursor/ ./internal/harness/opencode/ -update
# then run the AGENTS.md regeneration helper (Global Constraints), then:
gofmt -l internal/ cmd/
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/cli/ ./internal/app/ ./internal/harness/... ./internal/repoguard/ ./internal/assets/ ./internal/codexentry/ ./cmd/docket/
go test -count=1 -tags integration -timeout 30m ./internal/app/ ./internal/cli/ ./internal/gatedrive/
go run ./cmd/docket capabilities --json > "${TMPDIR:-/tmp}/docket-0471/caps.json"
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/protected.sh"
"$T/scan.sh" "run\.gate-|(?<![\w-])gate-(before|verdict|claim)(?![\w-])|--run-epoch|--run-gate-key|(?<![\w-])--epoch\b|DOCKET_AGENT_GUARDIAN_EPOCH|\bgate_context\b"
"$T/scan.sh" "change[ .]claim[^\n]*--gate-context"'
```
Expected:
- The new test and every package pass.
- `caps.json` lists `run.start` / `run.verdict` / `run.continue` and no `run.gate-*`.
- `protected.sh` prints only `ok` lines.
- Both scans print nothing. (`\bgate_context\b` cannot match the kept `gate_context_hash`, because `_` is a word byte.)
- `TestSkillSizeBudgets` and `TestDispatchBlockBudget` pass (they run inside `./internal/repoguard/`).

- [ ] **Step 7: Commit**

```bash
command git add internal/cli/run_tracker_rename_test.go internal/assets/embedded AGENTS.md <each modified path, explicitly>
command git commit -m "feat!: hard-cut run-tracker op ids, verbs, flags, keys and env var (change 0471)"
```

### Task 3: Hard-cut the report lines, codes and failure stages (rows 13–37)

**Build profile:** standard

**Files:**
- Modify: `internal/app/rungate_before.go`, `rungate_verdict.go`, `rungate_claim.go`, `rungate_continuation.go`, `rungate_store.go`, `rungate_epoch.go` (codes, stages, `Error()` prefix), `rungate_cancel.go`, `rungate_complete.go`, `rungate_fence.go`, `rungate_gate.go`, `rungate_epoch_refusal.go`, `gate.go`, `gate_drive.go`, `change_claim.go`; `internal/gatedrive/drive.go`, `incumbent.go`, `ownership.go`, `admission.go`, `admission_retire.go`
- Modify: every maintained site of these tokens (derived), including skills, `cursor-rules/run-gate.md`, docs, tests and pins
- Regenerate: embedded, goldens, the `AGENTS.md` block

**Interfaces:**
- Consumes: Task 2's verbs and flags.
- Produces: report lines `run-started <key> <run-id> <run-context>` / `run-untracked <reason>`, verdict lines `run-retry-once|run-continue|run-done|run-stop|run-observe`, the continue decision `run-continued`, codes and stages per rows 16–37, error-text prefix `run <stage>: <kind>`. The stored `Disposition` and `LastCause` text uses the new tokens from now on.

- [ ] **Step 1: Install the shared tools.**

- [ ] **Step 2: Update the pinning tests first (the failing test)**

The existing report-line tests are the specification: for example `TestIntegrationGateArmGateBeforeFreshArmSurfacesRunEpoch`'s `"gate-armed "` want-string, and the verdict-line tests. Find the code-constant definitions (the "implementation" side):

```bash
bash -c 'cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run && command git grep -n -E -e "= \"(gate-(armed|unarmed|retry-once|continue|done|stop|observe|claimed|unavailable|context-invalid|context-conflict)|epoch-[a-z-]+|incumbent-epoch-fenced|resume-epoch-unreadable|stale-run-epoch|unknown-run-epoch)\"" -e "\"gate-armed \"|\"gate-unarmed \"" -- "*.go" ":!*_test.go"'
```

Write the Task 3 map (Step 3). Apply it to the **test files only** first:

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/fileset.sh" "*_test.go" | xargs -0 perl "$T/tokens.pl" "$T/task3.tsv"'
```

Run: `go test -count=1 ./internal/app/ ./internal/gatedrive/ ./internal/cli/`
Expected: FAIL: tests now want `run-started`, `run-record-io`, `mint-run`, … while the code still emits the old tokens.

- [ ] **Step 3: The Task 3 map and the bulk pass**

```bash
bash -c '
T="${TMPDIR:-/tmp}/docket-0471"
printf "%s\t%s\n" \
  gate-armed run-started  gate-unarmed run-untracked \
  gate-retry-once run-retry-once  gate-continue run-continue  gate-done run-done \
  gate-stop run-stop  gate-observe run-observe  gate-claimed run-continued \
  epoch-not-found run-not-found  epoch-corrupt run-record-corrupt  epoch-exists run-exists \
  epoch-not-active run-not-active  epoch-mismatch run-id-mismatch  epoch-not-cancelled run-not-cancelled \
  epoch-ambiguous run-ambiguous  epoch-owner-ambiguous run-owner-ambiguous \
  epoch-owner-unresolved run-owner-unresolved  epoch-io run-record-io \
  epoch-participant-unknown run-participant-unknown  epoch-state-unknown run-state-unknown \
  epoch-unreadable run-record-unreadable  incumbent-epoch-fenced incumbent-run-fenced \
  resume-epoch-unreadable resume-run-record-unreadable  stale-run-epoch stale-run-id \
  unknown-run-epoch unknown-run-id  gate-unavailable run-tracker-unavailable \
  gate-context-invalid run-context-invalid  gate-context-conflict run-context-conflict \
  mint-epoch mint-run  find-epoch find-run  complete-epoch complete-run  supersede-epoch supersede-run \
  write-epoch write-run-record  load-epoch load-run-record  epoch-cas run-record-cas \
  bind-epoch-change bind-run-change  bind-epoch-worktree bind-run-worktree \
  epoch-launch-gate run-launch-gate \
  reserve-worktree-execution-epoch reserve-worktree-execution-run \
  retire-worktree-execution-epoch retire-worktree-execution-run \
  > "$T/task3.tsv"
"$T/fileset.sh" . | DRY=1 xargs -0 perl "$T/tokens.pl" "$T/task3.tsv"
'
```
Read the dry-run list, then run it without `DRY=1`. The boundaries keep `gate-stopped`, `gate-failed` and `docket-finalize-gate-*` untouched; confirm with `protected.sh` in Step 5.

Then hand-edit what the map cannot express:
- The glob `gate-*` means "the run tracker's report lines" only at `cursor-rules/run-gate.md` ("Obey the resulting `gate-*` report line") and the `GateRecord.Disposition` comment in `internal/app/rungate_store.go` ("latest gate-* report line"): make both `run-*`. The `"gate-*" reason` comment in `internal/app/evidence_ops.go` names the **suite-gate evidence reasons** and stays.
- The error-text prefix (row 37): in `internal/app/rungate_epoch.go`, `EpochError.Error()` formats become `"run %s: %s: %v"` and `"run %s: %s"`. Update every test asserting the old prefix: `command git grep -n -F -e 'run epoch ' -- '*_test.go'`. Classify each hit: only error-text assertions change here; prose is Task 6.

- [ ] **Step 4: Regenerate**

Run `go generate ./internal/assets/`, the harness `-update` run, and the `AGENTS.md` helper (Global Constraints).

- [ ] **Step 5: Verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
gofmt -l internal/ cmd/
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/app/ ./internal/gatedrive/ ./internal/cli/ ./internal/harness/... ./internal/repoguard/ ./internal/assets/ ./internal/codexentry/ ./cmd/docket/
go test -count=1 -tags integration -timeout 30m ./internal/app/ ./internal/gatedrive/ ./internal/cli/
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/protected.sh"; "$T/scan.sh" "(?<![\w-])(gate-(armed|unarmed|retry-once|continue|done|stop|observe|claimed|unavailable|context-invalid|context-conflict)|epoch-[a-z]+(-[a-z]+)*|[a-z]+-epoch(-[a-z]+)*)(?![\w-])"'
```
Expected: every package passes, and `protected.sh` prints only `ok`. The scan still prints three kinds of hit, all expected: test fixture **values** (for example `"epoch-e1"`, `"successor-epoch"`); prose adjectives (for example "run-epoch mutation fence", "epoch-revocation"), which Task 6 rewrites; and the kept `source-epoch`. No machine token (a code, stage, report line or verdict line from the Task 3 map) may appear.

- [ ] **Step 6: Commit**

```bash
command git add internal/assets/embedded AGENTS.md <each modified path, explicitly>
command git commit -m "feat!: hard-cut run-tracker report lines, codes and stages (change 0471)"
```

### Task 4: Rename the run-tracker files and integration shards

**Build profile:** standard

**Files:**
- Rename (git mv): the 36 `internal/app/rungate_*` files, 4 `internal/gatedrive` tests, 1 `internal/repoguard` test, and 6 shard runners (table in Step 2)
- Modify: `tests/runtime-budgets.tsv`; every maintained reference to a renamed path; the six test-function prefixes

**Interfaces:**
- Consumes: Tasks 1–3.
- Produces: `internal/app/runtracker_*.go` (Task 5 reads its declared identifiers from this glob); shard prefixes `TestIntegrationRunStart`, `TestIntegrationRunCancel`, `TestIntegrationRunCompletion`, `TestIntegrationRunRecord`, `TestIntegrationRunFence`, `TestIntegrationRunVerdict`.

- [ ] **Step 1: Record each shard's test count (the baseline the renamed shards must match)**

```bash
bash -c '
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
T="${TMPDIR:-/tmp}/docket-0471"; mkdir -p "$T"; : > "$T/shard-before.txt"
for p in TestIntegrationGateArm TestIntegrationGateCancel TestIntegrationGateCompletion TestIntegrationGateEpoch TestIntegrationGateFence TestIntegrationGateVerdict TestIntegrationGateLifecycle; do
  out=$(go test -tags integration -list "^$p" ./internal/app/) || { echo "list failed for $p"; exit 1; }
  printf "%s %s\n" "$p" "$(grep -c -E -e "^$p" <<<"$out")" >> "$T/shard-before.txt"
done
cat "$T/shard-before.txt"'
```
Expected at base: 39, 44, 31, 36, 30, 32, 11. Task 1 added one `TestIntegrationGateArm*` test, so GateArm now reads 40.

- [ ] **Step 2: git mv every file (paths from the spec §3)**

```bash
bash -c '
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
T="${TMPDIR:-/tmp}/docket-0471"
cat > "$T/renames.txt" <<EOF
internal/app/rungate_before.go internal/app/runtracker_start.go
internal/app/rungate_before_integration_test.go internal/app/runtracker_start_integration_test.go
internal/app/rungate_before_resume_integration_test.go internal/app/runtracker_start_resume_integration_test.go
internal/app/rungate_cancel.go internal/app/runtracker_cancel.go
internal/app/rungate_cancel_helpers_test.go internal/app/runtracker_cancel_helpers_test.go
internal/app/rungate_cancel_integration_test.go internal/app/runtracker_cancel_integration_test.go
internal/app/rungate_claim.go internal/app/runtracker_continue.go
internal/app/rungate_claim_integration_test.go internal/app/runtracker_continue_integration_test.go
internal/app/rungate_complete.go internal/app/runtracker_complete.go
internal/app/rungate_complete_helpers_test.go internal/app/runtracker_complete_helpers_test.go
internal/app/rungate_complete_integration_test.go internal/app/runtracker_complete_integration_test.go
internal/app/rungate_continuation.go internal/app/runtracker_continuation.go
internal/app/rungate_epoch.go internal/app/runtracker_run_record.go
internal/app/rungate_epoch_helpers_test.go internal/app/runtracker_run_record_helpers_test.go
internal/app/rungate_epoch_integration_test.go internal/app/runtracker_run_record_integration_test.go
internal/app/rungate_epoch_refusal.go internal/app/runtracker_run_id_refusal.go
internal/app/rungate_epoch_refusal_test.go internal/app/runtracker_run_id_refusal_test.go
internal/app/rungate_epochless_resume_e2e_integration_test.go internal/app/runtracker_no_run_record_resume_e2e_integration_test.go
internal/app/rungate_fence.go internal/app/runtracker_fence.go
internal/app/rungate_fence_helpers_test.go internal/app/runtracker_fence_helpers_test.go
internal/app/rungate_fence_integration_test.go internal/app/runtracker_fence_integration_test.go
internal/app/rungate_gate.go internal/app/runtracker_launch_gate.go
internal/app/rungate_gate_integration_test.go internal/app/runtracker_launch_gate_integration_test.go
internal/app/rungate_ownership_integration_test.go internal/app/runtracker_ownership_integration_test.go
internal/app/rungate_production_census_integration_test.go internal/app/runtracker_production_census_integration_test.go
internal/app/rungate_publication.go internal/app/runtracker_publication.go
internal/app/rungate_publication_integration_test.go internal/app/runtracker_publication_integration_test.go
internal/app/rungate_publication_settle_paths_integration_test.go internal/app/runtracker_publication_settle_paths_integration_test.go
internal/app/rungate_publication_settle_paths_test.go internal/app/runtracker_publication_settle_paths_test.go
internal/app/rungate_publication_test.go internal/app/runtracker_publication_test.go
internal/app/rungate_store.go internal/app/runtracker_store.go
internal/app/rungate_store_helpers_test.go internal/app/runtracker_store_helpers_test.go
internal/app/rungate_store_integration_test.go internal/app/runtracker_store_integration_test.go
internal/app/rungate_verdict.go internal/app/runtracker_verdict.go
internal/app/rungate_verdict_helpers_test.go internal/app/runtracker_verdict_helpers_test.go
internal/app/rungate_verdict_integration_test.go internal/app/runtracker_verdict_integration_test.go
internal/gatedrive/driver_epochfence_test.go internal/gatedrive/driver_runfence_test.go
internal/gatedrive/epoch_gate_test.go internal/gatedrive/run_launch_gate_test.go
internal/gatedrive/epoch_test.go internal/gatedrive/slot_run_fence_test.go
internal/gatedrive/takeover_epoch_test.go internal/gatedrive/takeover_run_test.go
internal/repoguard/gatedrive_run_epoch_thread_test.go internal/repoguard/gatedrive_run_id_thread_test.go
tests/test_go_integration_app_gatearm.sh tests/test_go_integration_app_runstart.sh
tests/test_go_integration_app_gatecancel.sh tests/test_go_integration_app_runcancel.sh
tests/test_go_integration_app_gatecompletion.sh tests/test_go_integration_app_runcompletion.sh
tests/test_go_integration_app_gateepoch.sh tests/test_go_integration_app_runrecord.sh
tests/test_go_integration_app_gatefence.sh tests/test_go_integration_app_runfence.sh
tests/test_go_integration_app_gateverdict.sh tests/test_go_integration_app_runverdict.sh
EOF
[ "$(wc -l < "$T/renames.txt" | tr -d " ")" = 47 ] || { echo "expected 47 renames"; exit 1; }
while read -r old new; do command git mv "$old" "$new" || exit 1; done < "$T/renames.txt"
ls internal/app/rungate_* 2>/dev/null && { echo "rungate_ files remain"; exit 1; }
echo renamed'
```
Expected: `renamed`, and no `rungate_` file left.

- [ ] **Step 3: Rewrite the references to renamed paths and the shard prefixes**

```bash
bash -c '
T="${TMPDIR:-/tmp}/docket-0471"
# Path references: full path and basename of every rename, as bounded tokens.
perl -ne "chomp; my (\$o, \$n) = split / /; print \"\$o\t\$n\n\"; (my \$bo = \$o) =~ s{.*/}{}; (my \$bn = \$n) =~ s{.*/}{}; print \"\$bo\t\$bn\n\";" "$T/renames.txt" | sort -u > "$T/task4-paths.tsv"
"$T/fileset.sh" . | DRY=1 xargs -0 perl "$T/tokens.pl" "$T/task4-paths.tsv"
"$T/fileset.sh" . | xargs -0 perl "$T/tokens.pl" "$T/task4-paths.tsv"
# Test-function prefixes: an identifier PREFIX (followed by more identifier bytes).
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
{ "$T/fileset.sh" "*.go" "tests/*.sh" tests/runtime-budgets.tsv; printf "internal/app/runtracker_storage_reset_integration_test.go\0"; } | xargs -0 perl -0777 -pi -e "
  s/(?<![A-Za-z0-9_])TestIntegrationGateArm/TestIntegrationRunStart/g;
  s/(?<![A-Za-z0-9_])TestIntegrationGateCancel/TestIntegrationRunCancel/g;
  s/(?<![A-Za-z0-9_])TestIntegrationGateCompletion/TestIntegrationRunCompletion/g;
  s/(?<![A-Za-z0-9_])TestIntegrationGateEpoch/TestIntegrationRunRecord/g;
  s/(?<![A-Za-z0-9_])TestIntegrationGateFence/TestIntegrationRunFence/g;
  s/(?<![A-Za-z0-9_])TestIntegrationGateVerdict/TestIntegrationRunVerdict/g;"
'
```
The `perl -pi` pass writes every file it reads, but only content matching a prefix changes. `git diff --stat` shows the real set. `TestIntegrationGateLifecycle` is untouched by construction.

Residual check: `command git grep -n -E -e 'rungate_|app_gate(arm|cancel|completion|epoch|fence|verdict)|TestIntegrationGate(Arm|Cancel|Completion|Epoch|Fence|Verdict)|driver_epochfence|epoch_gate_test|takeover_epoch_test|gatedrive_run_epoch_thread' -- . ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs'` prints nothing. A hit is a missed site: fix it by hand.

- [ ] **Step 4: Verify every renamed shard runs exactly its old count**

```bash
bash -c '
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
T="${TMPDIR:-/tmp}/docket-0471"
while read -r old n; do
  case "$old" in
    TestIntegrationGateArm) new=TestIntegrationRunStart ;; TestIntegrationGateCancel) new=TestIntegrationRunCancel ;;
    TestIntegrationGateCompletion) new=TestIntegrationRunCompletion ;; TestIntegrationGateEpoch) new=TestIntegrationRunRecord ;;
    TestIntegrationGateFence) new=TestIntegrationRunFence ;; TestIntegrationGateVerdict) new=TestIntegrationRunVerdict ;;
    *) new=$old ;;
  esac
  out=$(go test -tags integration -list "^$new" ./internal/app/) || { echo "list failed: $new"; exit 1; }
  got=$(grep -c -E -e "^$new" <<<"$out")
  [ "$got" = "$n" ] && echo "ok   $new $got" || echo "DIFF $new before=$n now=$got"
done < "$T/shard-before.txt"'
go vet -tags integration ./internal/app/ ./internal/gatedrive/ ./internal/repoguard/
go test -count=1 ./internal/repoguard/ ./internal/gatedrive/
bash tests/test_go_integration_contract.sh
bash tests/test_go_integration_app_runstart.sh && bash tests/test_go_integration_app_runrecord.sh
```
Expected: seven `ok` lines, repoguard green (including the runtime-budget row contract), the contract script ending with no `NOT OK`, and both shards passing.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
command git diff --name-only
command git diff --name-only -z | xargs -0 git add --
command git status --porcelain
command git commit -m "refactor: rename run-tracker files and integration shards (change 0471)"
```
The `git mv` renames are already staged. Read the `git diff --name-only` list before staging it: every path must be a reference fix or prefix rename from this task, and nothing else.

### Task 5: Rename the run-tracker Go identifiers

**Build profile:** standard

**Files:**
- Modify: every Go file with an identifier the rename map changes (about 130 files, derived), plus CamelCase mentions of those identifiers in maintained markdown
- Tool (scratch, never committed): `$T/idtool/`, `$T/mkmap.pl`, `$T/applymap.pl`

**Interfaces:**
- Consumes: Task 4's `internal/app/runtracker_*.go` file names.
- Produces (the names later tasks use): `RunStart` / `RunStartResult` (fields `Started`, `RunID`, `RunContext`), `RunVerdict` / `RunVerdictResult` / `RunVerdictObserve`, `RunContinue` / `RunContinueResult`, `OperationRunStart|Verdict|Continue`, `RunRecord`, `LoadRunRecord`, `MintRunRecord`, `FindRunByChange`, `findRunByWorktree`, `RunTrackerRecord`, `RunTrackerStoreError`, `ErrRunTrackerUnavailable`, `RunLaunchGate`, `ClassifyRunIDError`, `RunIDNextAction`, `CheckRunIDExists`, `CheckRunIDLinkage`, `RunContextHash`, gatedrive `RunID` fields, `ReserveWorktreeExecutionForRun`, harness `RunTracker(catalog)` and `RunTrackerAsset` (value still `"run-gate.md"` until Task 7), `runTrackerDirName`, `runRecordFileName`, `runLockFileName`. The map is rule-generated; the full list is printed in Step 3 and goes into the task report.

- [ ] **Step 1: Build the toolkit**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0471"; mkdir -p "$T/idtool"
cat > "$T/idtool/go.mod" <<EOF
module idtool

go 1.22
EOF
cat > "$T/idtool/main.go" <<'"'"'EOF'"'"'
// idtool is change 0471'"'"'s AST identifier toolkit (scratch; never committed).
//
//	idtool census REGEX         (NUL-separated .go paths on stdin) distinct identifiers matching REGEX
//	idtool decls  REGEX FILE... distinct identifiers DECLARED in FILE... matching REGEX
//	idtool rename MAP   FILE... rewrite every ast.Ident named OLD to NEW (TSV MAP) in place;
//	                            comments and string literals are never touched
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

func die(err error) {
	fmt.Fprintln(os.Stderr, "idtool:", err)
	os.Exit(2)
}

func parse(path string) (*token.FileSet, *ast.File, []byte) {
	src, err := os.ReadFile(path)
	if err != nil {
		die(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		die(err)
	}
	return fset, f, src
}

func printSorted(set map[string]bool) {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	for _, k := range out {
		fmt.Println(k)
	}
}

func main() {
	if len(os.Args) < 3 {
		die(fmt.Errorf("usage: idtool census|decls|rename ARG [FILE...]"))
	}
	switch os.Args[1] {
	case "census":
		re := regexp.MustCompile(os.Args[2])
		in, err := io.ReadAll(os.Stdin)
		if err != nil {
			die(err)
		}
		set := map[string]bool{}
		for _, p := range bytes.Split(in, []byte{0}) {
			if len(p) == 0 || !strings.HasSuffix(string(p), ".go") {
				continue
			}
			_, f, _ := parse(string(p))
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && re.MatchString(id.Name) {
					set[id.Name] = true
				}
				return true
			})
		}
		printSorted(set)
	case "decls":
		re := regexp.MustCompile(os.Args[2])
		set := map[string]bool{}
		for _, p := range os.Args[3:] {
			_, f, _ := parse(p)
			ast.Inspect(f, func(n ast.Node) bool {
				var names []*ast.Ident
				switch x := n.(type) {
				case *ast.FuncDecl:
					names = append(names, x.Name)
				case *ast.TypeSpec:
					names = append(names, x.Name)
				case *ast.ValueSpec:
					names = append(names, x.Names...)
				case *ast.Field:
					names = append(names, x.Names...)
				case *ast.AssignStmt:
					if x.Tok == token.DEFINE {
						for _, l := range x.Lhs {
							if id, ok := l.(*ast.Ident); ok {
								names = append(names, id)
							}
						}
					}
				}
				for _, id := range names {
					if re.MatchString(id.Name) {
						set[id.Name] = true
					}
				}
				return true
			})
		}
		printSorted(set)
	case "rename":
		mf, err := os.Open(os.Args[2])
		if err != nil {
			die(err)
		}
		m := map[string]string{}
		sc := bufio.NewScanner(mf)
		for sc.Scan() {
			if f := strings.SplitN(sc.Text(), "\t", 2); len(f) == 2 && f[1] != "" {
				m[f[0]] = f[1]
			}
		}
		changed := 0
		for _, p := range os.Args[3:] {
			fset, f, src := parse(p)
			type hit struct {
				off      int
				old, new string
			}
			var hits []hit
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if nw, ok := m[id.Name]; ok {
						hits = append(hits, hit{fset.Position(id.Pos()).Offset, id.Name, nw})
					}
				}
				return true
			})
			if len(hits) == 0 {
				continue
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].off > hits[j].off })
			out := src
			for _, h := range hits {
				out = append(append(append([]byte{}, out[:h.off]...), h.new...), out[h.off+len(h.old):]...)
			}
			if err := os.WriteFile(p, out, 0o644); err != nil {
				die(err)
			}
			changed++
		}
		fmt.Fprintf(os.Stderr, "idtool rename: rewrote %d file(s)\n", changed)
	default:
		die(fmt.Errorf("unknown subcommand %q", os.Args[1]))
	}
}
EOF
( cd "$T/idtool" && go build -o "$T/idtool.bin" . )

cat > "$T/mkmap.pl" <<'"'"'EOF'"'"'
#!/usr/bin/perl
# mkmap.pl : reads Go identifiers (one per line) on stdin and prints OLD<TAB>NEW for
# every identifier the ADR-0129 family (a) naming rules change. TRACKER (env) names
# a file of the identifiers DECLARED in the run-tracker files; only those take the
# tracker-sense Gate -> RunTracker rule.
use strict; use warnings;
my %keep = map { $_ => 1 } qw(gatedriveClaimSeam gatedriveContinuationSeam
  participantKindGateScope startFinalizeGate gatedWaitingReader gate);
my %override = (
  Epoch => "RunID", epoch => "runID", epochs => "runIDs", Armed => "Started",
  DispatchEpoch => "DispatchedAt", dispatchEpoch => "dispatchedAt",
  runGateKey => "runKey", expectEpoch => "expectRunID",
  guardianEpochEnv => "guardianRunIDEnv", errGuardianEpochMismatch => "errGuardianRunIDMismatch",
  runGateEpochCopyRe => "runTrackerRunIDCopyRe",
  TestRunGateCopiesEpochIntoDispatchPrompt => "TestRunTrackerCopiesRunIDIntoDispatchPrompt",
  codexRequestEpochRe => "codexRequestRunIDRe",
  epochGateFixture => "runLaunchGateFixture", recordingEpochGate => "recordingRunLaunchGate",
  GateClaimDecisionClaimed => "RunContinueDecisionContinued", GateClaimOutcome => "RunContinueOutcome",
  newGateClaimStop => "newRunContinueStop", persistGateClaimStop => "persistRunContinueStop",
  armedGateResult => "startedRunResult", gateUnarmed => "runUntracked", gateUnarmedMsg => "runUntrackedMsg",
  gateMintArmed => "runTrackerMintStarted", gateMintArmedScoped => "runTrackerMintStartedScoped",
  ReasonGateRunCancelled => "ReasonRunCancelled", ReasonGateEpochUnreadable => "ReasonRunRecordUnreadable",
  ReasonGateResumeEpochUnreadable => "ReasonResumeRunRecordUnreadable", ReasonGateStaleRunEpoch => "ReasonStaleRunID",
  CauseEpochUnreadable => "CauseRunRecordUnreadable",
  ErrEpochCorrupt => "ErrRunRecordCorrupt", ErrEpochIO => "ErrRunRecordIO",
  ErrEpochMismatch => "ErrRunIDMismatch", ErrEpochUnresolved => "ErrRunRecordUnresolved",
  epochCAS => "runRecordCAS",
);
my %tracker;
if ($ENV{TRACKER}) { open my $t, "<", $ENV{TRACKER} or die "TRACKER: $!"; while (<$t>) { chomp; $tracker{$_} = 1 if length } }
while (my $old = <STDIN>) {
  chomp $old; next unless length $old; next if $keep{$old};
  my $n = $override{$old};
  if (!defined $n) {
    $n = $old;
    $n =~ s/RunGateBefore/RunStart/g;    $n =~ s/(?<![A-Za-z])runGateBefore/runStart/g;
    $n =~ s/GateBefore/RunStart/g;       $n =~ s/(?<![A-Za-z])gateBefore/runStart/g;
    $n =~ s/RunGateVerdict/RunVerdict/g; $n =~ s/GateVerdict/RunVerdict/g; $n =~ s/(?<![A-Za-z])gateVerdict/runVerdict/g;
    $n =~ s/RunGateClaim/RunContinue/g;
    $n =~ s/RunGate/RunTracker/g;        $n =~ s/(?<![A-Za-z])runGate/runTracker/g;
    $n =~ s/(?<![A-Za-z])rungate/runTracker/g; $n =~ s/Rungate/RunTracker/g;
    $n =~ s/GateKey/RunKey/g;            $n =~ s/(?<![A-Za-z])gateKey/runKey/g;
    $n =~ s/GateContext/RunContext/g;    $n =~ s/(?<![A-Za-z])gateContext/runContext/g;
    $n =~ s/DispatchContext/RunContext/g; $n =~ s/(?<![A-Za-z])dispatchContext/runContext/g;
    $n =~ s/Epochless/NoRunRecord/g;     $n =~ s/(?<![A-Za-z])epochless/noRunRecord/g;
    $n =~ s/RunEpochID/RunID/g;          $n =~ s/(?<![A-Za-z])runEpochID/runID/g;
    $n =~ s/RunEpoch/RunID/g;            $n =~ s/(?<![A-Za-z])runEpoch/runID/g;
    $n =~ s/EpochRecord/RunRecord/g;     $n =~ s/(?<![A-Za-z])epochRecord/runRecord/g;
    $n =~ s/EpochIDs/RunIDs/g;           $n =~ s/(?<![A-Za-z])epochIDs/runIDs/g;
    $n =~ s/EpochID/RunID/g;             $n =~ s/(?<![A-Za-z])epochID/runID/g;
    $n =~ s/EpochUnreadable/RunRecordUnreadable/g;
    $n =~ s/Epochs/Runs/g;               $n =~ s/(?<![A-Za-z])epochs/runs/g;
    $n =~ s/Epoch/Run/g;                 $n =~ s/(?<![A-Za-z])epoch/run/g;
    if ($tracker{$old}) {
      $n =~ s/GateDecision/RunDecision/g; $n =~ s/GateOutcome/RunOutcome/g;
      $n =~ s/GateReason/RunReason/g;     $n =~ s/ReasonGate/ReasonRun/g;
      $n =~ s/GateObservation/RunObservation/g;
      $n =~ s/Unarmed/Untracked/g;        $n =~ s/Armed/Started/g;
      $n =~ s/(?<!Launch)(?<!Finalize)(?<!Build)(?<!Suite)Gate/RunTracker/g;
      $n =~ s/(?<![A-Za-z])gate/runTracker/g;
    }
  }
  print "$old\t$n\n" if $n ne $old;
}
EOF

cat > "$T/applymap.pl" <<'"'"'EOF'"'"'
#!/usr/bin/perl
# applymap.pl MAP FILE... : rewrites each FILE in place, replacing every whole
# identifier OLD (bounded by bytes outside [A-Za-z0-9_]) with NEW. Used for the
# CamelCase/mixed map entries only, so their mentions in comments, strings and
# markdown move with them (intended).
use strict; use warnings;
my $map = shift @ARGV; my %m;
open my $mf, "<", $map or die "map $map: $!";
while (<$mf>) { chomp; my ($o, $n) = split /\t/; $m{$o} = $n if defined $n && length $n }
my $alt = join "|", map { quotemeta } sort { length($b) <=> length($a) } keys %m;
my $re = qr/(?<![A-Za-z0-9_])($alt)(?![A-Za-z0-9_])/;
my $changed = 0;
for my $file (@ARGV) {
  open my $in, "<", $file or die "read $file: $!"; local $/; my $s = <$in>; close $in;
  my $c = ($s =~ s/$re/$m{$1}/g); next unless $c;
  open my $out, ">", $file or die "write $file: $!"; print $out $s; close $out; $changed++;
}
print STDERR "applymap: rewrote $changed file(s)\n";
EOF
echo toolkit ready'
```

- [ ] **Step 2: Pre-resolve the one known name collision**

A planning-time dry run of this exact procedure found one collision: in `internal/app/gate_drive_test.go`, `TestBudgetedBuildAdvisoryReconcilesWithScopeEpoch` declares both `runID` (a gate run id) and a local `epoch` that the sweep maps to `runID`. Rename that file's `epoch` identifiers first (AST-only, so comments and strings stay):

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run && printf "epoch\townerRunID\n" > "$T/pre.tsv" && "$T/idtool.bin" rename "$T/pre.tsv" internal/app/gate_drive_test.go && go vet ./internal/app/'
```

- [ ] **Step 3: Generate and review the map**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0471"
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
"$T/idtool.bin" decls "[Gg]ate" internal/app/runtracker_*.go > "$T/decls.txt"
grep -v -E -e "^Test" "$T/decls.txt" > "$T/tracker.txt" || true
{ "$T/fileset.sh" "*.go"; printf "internal/gatedrive/storage_reset_test.go\0internal/app/runtracker_storage_reset_integration_test.go\0"; } \
  | "$T/idtool.bin" census "[Ee]poch|[Rr]un[Gg]ate|[Rr]ungate|GateKey|gateKey|GateContext|gateContext|DispatchContext|dispatchContext|GateBefore|gateBefore|GateVerdict|gateVerdict|^Armed\$" > "$T/idents.txt"
cat "$T/tracker.txt" >> "$T/idents.txt"; sort -u -o "$T/idents.txt" "$T/idents.txt"
TRACKER="$T/tracker.txt" perl "$T/mkmap.pl" < "$T/idents.txt" > "$T/idmap.tsv"
awk -F"\t" "\$1 ~ /^[a-z]+\$/ || \$1 == \"Epoch\" || \$1 == \"Armed\"" "$T/idmap.tsv" > "$T/idmap-ast.tsv"
awk -F"\t" "!(\$1 ~ /^[a-z]+\$/ || \$1 == \"Epoch\" || \$1 == \"Armed\")" "$T/idmap.tsv" > "$T/idmap-text.tsv"
wc -l "$T/idmap.tsv" "$T/idmap-ast.tsv" "$T/idmap-text.tsv"
echo "--- many-to-one targets (must never share a scope; the compiler decides):"
cut -f2 "$T/idmap.tsv" | sort | uniq -d
cat "$T/idmap.tsv"'
```
Expected: about 550 map lines, with `idmap-ast.tsv` holding exactly the dictionary-word entries `epoch`, `epochs`, `epochless`, `Epoch`, `Armed`. They are renamed only as AST identifiers: a textual pass would also rewrite them in comments and strings. The many-to-one list is `runContext`, `RunContext`, `runID`, `RunID`, `runIDs`, `runTrackerRoot`, or a subset. Paste the full map into the task report. A name that reads wrong by ADR-0129 Decision 4 (id → run id, stored file → run record, run/state → run) gets an entry in `%override` and a regenerated map, never a hand edit afterward.

- [ ] **Step 4: Apply the map**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0471"
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
"$T/fileset.sh" "*.go" | xargs -0 "$T/idtool.bin" rename "$T/idmap-ast.tsv"
"$T/fileset.sh" "*.go" "*.md" "*.sh" "*.yml" | xargs -0 perl "$T/applymap.pl" "$T/idmap-text.tsv"
# The storage-reset tests: identifiers only (their string fixtures spell the retired layout).
"$T/idtool.bin" rename "$T/idmap.tsv" internal/gatedrive/storage_reset_test.go internal/app/runtracker_storage_reset_integration_test.go
command git diff --name-only -z -- "*.go" | xargs -0 gofmt -w
go build ./...'
```
If `go build` or Step 5's vet reports a redeclaration or a shadowed function, a new collision was introduced after planning (Tasks 1–4 added code). Resolve it the way Step 2 did: AST-rename the colliding local in that one file. Report it.

- [ ] **Step 5: Verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
go vet ./... && go vet -tags integration ./...
go test -count=1 ./...
go test -count=1 -tags integration -timeout 30m ./internal/app/ ./internal/gatedrive/ ./internal/cli/ ./internal/codexentry/
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
{ "$T/fileset.sh" "*.go"; printf "internal/gatedrive/storage_reset_test.go\0internal/app/runtracker_storage_reset_integration_test.go\0"; } | "$T/idtool.bin" census "[Ee]poch|[Rr]un[Gg]ate|[Rr]ungate|GateKey|gateKey|GateContext|gateContext|DispatchContext|dispatchContext|GateBefore|GateVerdict"
"$T/protected.sh"'
go generate ./internal/assets/ && command git status --porcelain -- internal/assets/embedded
```
Expected: vet and every test pass; the census prints nothing; `protected.sh` prints only `ok`. If `go generate` changed the embedded tree (a skill mentioned a renamed identifier), commit it with this task. If the change reached `agents/` or `cursor-rules/`, also run the harness `-update` pass and the `AGENTS.md` helper (Global Constraints). `TestClaimReceiptKeepsCommittedGateContextHashKey` (now spelled with `RunContextHash` fields) still passes: the committed key survived the field rename.

- [ ] **Step 6: Commit**

```bash
command git add <each modified path, explicitly>
command git commit -m "refactor: rename run-tracker Go identifiers (change 0471)"
```

### Task 6: Rewrite run-tracker prose in Go comments and strings (rows 1–6)

**Build profile:** standard

**Files:**
- Modify: Go comments and human-readable strings across the maintained Go tree, **excluding** `internal/repoguard/**`, `internal/harness/**`, and the Go tests that pin markdown prose (these move with their markdown in Task 7): `internal/cli/agent_test.go`, `internal/gatedrive/launch_sites_guard_test.go`, `internal/install/devmode_test.go`, `internal/reposeed/plan_test.go`, `internal/app/runtracker_no_run_record_resume_e2e_integration_test.go`. The storage-reset tests stay excluded (`fileset.sh`).

**Interfaces:**
- Consumes: Tasks 2–5. The prose engine's full mode assumes no wire token remains (checked in Step 2).
- Produces: Go comments and messages in the run-tracker vocabulary. Human messages change wording, never a machine token.

- [ ] **Step 1: Install the shared tools.**

- [ ] **Step 2: Precondition — no wire token is left for the prose rules to mangle**

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/scan.sh" "(?<![\w-])(--run-epoch|--epoch|--run-gate-key|gate-(before|verdict|claim|armed|unarmed|retry-once|continue|done|stop|observe|claimed|unavailable|context-invalid|context-conflict)|(mint|find|complete|supersede|write|load)-epoch|epoch-(cas|launch-gate|not-found|corrupt|exists|not-active|mismatch|not-cancelled|ambiguous|owner-ambiguous|owner-unresolved|io|participant-unknown|state-unknown|unreadable)|stale-run-epoch|unknown-run-epoch)(?![\w-])" "*.go"'
```
Expected: no output. Any hit is a Task 2/3 miss: fix it in its own commit first.

- [ ] **Step 3: Characterize, then run the prose pass**

Record the Go test baseline: `go test -count=1 ./... 2>&1 | tail -5` must be green before you touch prose. Then:

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0471"
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
EXCL=(":!internal/repoguard" ":!internal/harness" ":!internal/cli/agent_test.go" ":!internal/gatedrive/launch_sites_guard_test.go" ":!internal/install/devmode_test.go" ":!internal/reposeed/plan_test.go" ":!internal/app/runtracker_no_run_record_resume_e2e_integration_test.go")
# Files where every "arm" is the run tracker'"'"'s: full mode plus the arm rules.
"$T/fileset.sh" "internal/app/runtracker_*.go" internal/cli/run.go "internal/app/agent_guardian*.go" internal/app/root_entry_integration_test.go "internal/app/claim_proof*.go" "internal/app/change_claim*.go" "internal/codexentry/*.go" "${EXCL[@]}" | ARM=1 xargs -0 perl "$T/prose.pl"
# Every other Go file: full mode, no arm rules.
"$T/fileset.sh" "*.go" "${EXCL[@]}" | xargs -0 perl "$T/prose.pl"'
```

- [ ] **Step 4: Review what the rules cannot decide**

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"
"$T/scan.sh" "\b(re-?)?arm(s|ed|ing)?\b|\bunarmed\b" "*.go" ":!internal/repoguard" ":!internal/harness"
"$T/scan.sh" "\b(run|Run) runs?\b|\ba run id id\b|run tracker tracker|\brun key key\b|\bstarted (gate|run tracker) (record|key)\b" "*.go"
"$T/scan.sh" "\bthe (armed )?gate record\b|\bthe gate\b" "internal/app/runtracker_*.go"'
```
- **Arm list:** in a run-tracker context ("the armed gate", "arm time", "an arm"), rewrite by hand to start / started / starting. Leave auto-groom's `rearm` / "re-arm" (family (d)), the suite runner's budget "armed", the workspace/githubcli senses, and `TestIntegration…Arm…` test names as they are.
- **Grammar list:** fix each hit by hand.
- **"The gate" in the run-tracker files:** where it means the run tracker, say "the run tracker". Where it means the per-key record, say "the run-tracker record". Where it means a suite gate, gate drive, or the launch gate, keep it.

- [ ] **Step 5: Verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
gofmt -l internal/ cmd/
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./...
go test -count=1 -tags integration -timeout 30m ./internal/app/ ./internal/gatedrive/ ./internal/cli/ ./internal/repository/...
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/protected.sh"; "$T/scan.sh" "\bepochs?\b|run[ -]gate|gate facade|gate keys?\b|dispatch[ -]context" "*.go" ":!internal/repoguard" ":!internal/harness" ":!internal/cli/agent_test.go" ":!internal/gatedrive/launch_sites_guard_test.go" ":!internal/install/devmode_test.go" ":!internal/reposeed/plan_test.go" ":!internal/app/runtracker_no_run_record_resume_e2e_integration_test.go"'
```
Expected: all green; `protected.sh` all `ok`. The scan prints only protected spellings: "Unix epoch", "epoch seconds", `gate_context_hash`, `dispatch_context`, the gate drive's `--gate-context`, and literal `rungate` / `epoch.json` inside a storage comment that explains the reset. Classify each remaining line in the task report.

- [ ] **Step 6: Commit**

```bash
command git add <each modified path, explicitly>
command git commit -m "docs: run-tracker vocabulary in Go comments and messages (change 0471)"
```

### Task 7: Agent-facing markdown, the cursor rule, pinned prose and the `AGENTS.md` block

**Build profile:** standard

**Files:**
- Rename: `cursor-rules/run-gate.md` → `cursor-rules/run-tracker.md` (git mv)
- Modify: `internal/harness/dispatch.go` (`RunTrackerAsset` value; `CodexRootEntryClause`), every `internal/harness/**` test naming `cursor-rules/run-gate.md`, `internal/install/devmode_test.go`, `internal/reposeed/plan_test.go`, `internal/repoguard/*_test.go` pins, `internal/cli/agent_test.go`, `internal/gatedrive/launch_sites_guard_test.go`, `internal/app/runtracker_no_run_record_resume_e2e_integration_test.go`
- Modify: `skills/**`, `agents/**`, `.docket.example.yml`, `tests/*.sh` comments, `internal/repoguard/budgets_test.go` (comments; a ceiling only if the rule in Global Constraints allows)
- Regenerate: embedded tree and `manifest.json`, harness goldens, `AGENTS.md` block

**Interfaces:**
- Consumes: Task 5's `harness.RunTracker`, `harness.RunTrackerAsset`, `harness.CodexDispatchInterior`.
- Produces: the dispatch payload `cursor-rules/run-tracker.md` (heading `## Run tracker — …`), which every consumer repo receives on `docket install`. Consumer repos are unaffected by the file rename: Cursor always installs the composed rule as `docket-dispatch.mdc`.

- [ ] **Step 1: Install the shared tools.**

- [ ] **Step 2: Rename the cursor rule and its path references**

```bash
bash -c '
T="${TMPDIR:-/tmp}/docket-0471"
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
command git mv cursor-rules/run-gate.md cursor-rules/run-tracker.md
printf "%s\t%s\n" cursor-rules/run-gate.md cursor-rules/run-tracker.md "\"run-gate.md\"" "\"run-tracker.md\"" > "$T/task7-paths.tsv"
"$T/fileset.sh" . | xargs -0 perl "$T/tokens.pl" "$T/task7-paths.tsv"
command git grep -n -F -e run-gate.md -- . ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":!internal/assets/embedded" ":!internal/install/legacydata" ":!internal/install/legacy*.go" ":!**/testdata/**"'
```
Expected: the last grep prints only `docs/` hits (Task 8), or nothing. `const RunTrackerAsset = "run-tracker.md"` now holds in `internal/harness/dispatch.go`. Fix its doc comment ("the dispatch-role payload carrying the run tracker").

- [ ] **Step 3: Write the payload's final text**

Replace the whole content of `cursor-rules/run-tracker.md` with exactly:

```markdown
## Run tracker — bracket a dispatched implement-next run with the run tracker

A dispatched run that stops early returns a report that reads as success; a completion
notification is the CHILD's claim, not your report. The run tracker owns attribution, durable
state, and retry accounting: never hand-reimplement them, and never infer permission from child
prose, launch shape, timestamps, ids, or exit codes. The `docket` binary is on `PATH`; resolve each
operation below from the capability catalog. If it is missing, the install is broken: surface it,
never rebuild the run tracker by hand.

1. Before dispatching `docket-implement-next`, run `run.start` with `implement-next`. It prints
   `run-started <key> <run-id> <run-context>`; keep all three (they won't survive the next tool
   call) and copy the `<run-context>` and the `<run-id>` into the dispatch prompt. The `<run-id>`
   is the id you thread into `run.cancel --run-id` (below) and every `--run-id` dispatch
   flag (`agent.enter`, `gate drive start`, `gate drive prepare-scope`). Add `--resume <id>` to
   start a run that resumes an already-in-progress change. `run-untracked` still lets you dispatch,
   but keyless (step 2's fallback) and can never authorize a re-dispatch.
2. After the run returns, or its completion notification arrives, run `run.verdict`
   with `<key>`; without a key, run it with `--unattributed` plus any change id the notification
   names. Obey the resulting `run-*` report line exactly, never its exit code or the child's prose.
3. Only `run-retry-once` authorizes another dispatch: the same `docket-implement-next`, once, for
   the id and unmet conjuncts it names, keeping the same key. `run-continue <key> run-waiting
   <change-id> <continuation-id> <phase>` is **nonterminal**: the same attempt still owns tracked
   work, so it keeps the same key, spends no retry, and is distinct from `run-retry-once` (a
   continuation, not a second attempt). On it, resume the existing implement-next agent,
   or dispatch `docket-implement-next` again with the explicit change id, the continuation id, and the
   same key, and run `run.verdict` with `<key>` again. Every `run-stop` and every
   `run-observe` forbids re-dispatch; `run-halted` means a human is needed.

## Stopping a dispatched run — there is no automatic Stop button

A run you dispatched has **no automatic Stop**: closing a tab, interrupting the coordinator, or
killing a process does not tell the run tracker the run is over, and `run.start` says so (it
reports the honest owner-lifecycle caveat). To stop a dispatched run deliberately, invoke the
explicit `run.cancel` operation (argv resolved from the capability catalog) with the key and run id
`run.start` gave you, plus a human reason — `--key <key> --run-id <id> --reason <why>`.

It fences the run so nothing new can attach to it, then stops the run's registered native tasks and
processes and reports one disposition:

- `cancelled` — the run was fenced and everything the cancel tracks is accounted for.
- `cancellation-pending` — the fence is durably held but teardown is not fully accounted for yet
  (a process or in-flight action still resolving). Cleanup is safe to resume: **re-run the same
  cancel** to finish it; a repeat never restores the run.
- `already-cancelled` — the run was already cancelled (or the cancel already completed); a no-op.
- `refused` — the key, run id, or repository did not match; nothing was touched.

Cancelling is never a test failure and never earns a retry: it charges no suite attempt and resets
no deadline, budget, or retry state. Completed work is never rolled back.

## Resuming after a stop or interruption

Start a resume with `run.start … --resume <id>`. Because one worktree carries at most one live
run, `run.start` refuses to start a second run over one that has not verifiably stopped, and tells
you what to do instead:

- `resume-active-run` — the prior run is still **active** (nothing has confirmed it
  stopped). `run.start` prints a locator naming the change, run id, and key, plus the exact remedy:
  **cancel it** via the `run.cancel` operation (`--key <key> --run-id <id> --reason <why>`) and
  resume after confirmed cancellation, **or** continue the live run via `run.verdict`. Do not
  force a fresh claim over a run that may still be live.
- `cancellation-pending` — a cancellation is still finishing. The resume only observes that
  cleanup; it does not admit a replacement. Finish the cancel (re-run it until `cancelled`), then
  resume.
- `resume-replacement-reserved` — the prior run was already confirmed-cancelled and superseded,
  and **exactly one** replacement dispatch is reserved. A repeat start (or one recovering a lost
  response) returns that same reserved key rather than minting a second run — dispatch the reserved
  replacement; never start a parallel one.

Resume admits **exactly one** replacement, and only after cancellation is confirmed. `run-continue`
is unchanged by any of this: it stays nonterminal, keeps the same key, spends no retry, and resumes
the existing agent (or re-dispatches with the change id and continuation id) as in step 3 above.
```

This text has the same word count as the base payload (785). In `internal/harness/dispatch.go`, set the second and third paragraphs of `CodexRootEntryClause` to exactly these strings (the first paragraph is unchanged):

```go
	"For any `agent.enter` route: Write a request file containing the user's request unchanged; for implement-next include the unchanged run-context token, labeled for `change.claim --run-context` and gate-drive `--gate-context`, and the unchanged run id, labeled for `--run-id` on prepare-scope and build-owned starts. Preserve resume/continuation ids and run keys. Pass `--request`, `--role`, the active absolute caller `--cwd`, approval policy, and sandbox; pass the owning workflow's exact `--worktree` explicitly for feature children. Never omit run context.\n\n" +
	"A shell-tool yield carrying a live task/session identity is a liveness transition, not completion. You must retain that exact task/session identity and collect its terminal exit and final output through the harness-native observation/wait mechanism. Never re-run `agent.enter`, start a second watcher, or return a completion report while the original task remains live or unobserved. Only after terminal output is collected may implement-next run the parent's keyed `run.verdict` and obey its report. Coordinator prose, thread or turn ids, and process exit alone do not prove run ownership or completion. Do not substitute `codex exec`, another harness, a generic agent, or a parent relay."
```

- [ ] **Step 4: Rewrite the rest of the agent-facing markdown and the deferred Go files**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0471"
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
# Markdown, YAML and shell comments: SAFE mode only (bare "epoch" is decided by hand below).
"$T/fileset.sh" "skills/**" "agents/**" "cursor-rules/**" .docket.example.yml "tests/*.sh" tests/README.md README.md CONTRIBUTING.md "scripts/**" | PROSE_MODE=safe xargs -0 perl "$T/prose.pl"
# The Go files Task 6 deferred: full mode (their pinned strings are realigned by hand below).
"$T/fileset.sh" "internal/repoguard/*.go" "internal/harness/**/*.go" internal/cli/agent_test.go internal/gatedrive/launch_sites_guard_test.go internal/install/devmode_test.go internal/reposeed/plan_test.go ":!internal/harness/dispatch.go" | xargs -0 perl "$T/prose.pl"
ARM=1 perl "$T/prose.pl" internal/app/runtracker_no_run_record_resume_e2e_integration_test.go'
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/scan.sh" "\bepochs?\b|\b(re-?)?arm(s|ed|ing)?\b|\bunarmed\b|\bgate record\b|\bthe gate\b" "skills/**" "agents/**" "cursor-rules/**" .docket.example.yml "tests/*.sh"'
```
Rewrite each printed line by hand, by meaning (ADR-0129 Decision 4):
- A value that is threaded, passed, copied or carried ("pass the run epoch your repair dispatch payload carried", "the bundle carries no run epoch") → **run id**.
- The stored file → **run record**. The run or its state ("a fenced epoch", "the prior epoch is active") → **run**.
- Run-tracker arm / arming / armed / unarmed → start / starting / started / untracked; a run-tracker re-arm → "start again".
- Auto-groom's "arm a stub" / "re-arm" is family (d): leave it.
- "the gate" / "gate record", where it means the run tracker, → "the run tracker" / "the run-tracker record". "Gate key" was already handled by the safe rules.

Keep each edit minimal; re-wrap a line only when it becomes longer than its neighbors (learning `phrase-grep-over-wrapped-prose`).

- [ ] **Step 5: Regenerate, then realign the pins with the final text**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
go generate ./internal/assets/
go test -count=1 ./internal/harness/claude/ ./internal/harness/codex/ ./internal/harness/cursor/ ./internal/harness/opencode/ -update
# run the AGENTS.md regeneration helper (Global Constraints)
go test -count=1 ./internal/repoguard/ ./internal/harness/... ./internal/assets/ ./internal/install/ ./internal/reposeed/ ./internal/cli/ ./internal/gatedrive/
```
A red pin that encodes a retired phrase gets its **expected text updated to the new phrase**. It is never deleted or loosened (learning `restatement-accumulates-its-own-guards`: relocate, don't restore). Known pins at planning time:
- `runTrackerRunIDCopyRe` becomes ``"copy the [^.]{0,60}`<run-id>` into the dispatch prompt"``, and its non-vacuity `good` / `old` strings use `<run-context>` / `<run-id>`.
- `codexRequestRunIDRe` becomes ``"Write a request file (?:[^.]|\\.\\S){0,400}run id(?:[^.]|\\.\\S){0,80}`--run-id`"``, and its `good` / `old` strings match the new `CodexRootEntryClause`.
- `TestRunTrackerCopiesRunIDIntoDispatchPrompt`'s path list reads `cursor-rules/run-tracker.md` and `internal/assets/embedded/tree/cursor-rules/run-tracker.md`.

Mutation-check each pin you realign: delete the pinned phrase from its source (backup and restore), see RED, restore, see GREEN.

- [ ] **Step 6: Verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
go vet ./... && go test -count=1 ./...
go test -count=1 -tags integration -timeout 30m -run 'TestIntegrationRunStartNoRunRecordResume|TestIntegrationRunStart' ./internal/app/
go test -count=1 ./internal/repoguard/ -run 'TestSkillSizeBudgets|TestDispatchBlockBudget|TestCommittedCodexDispatch|TestEmbeddedMatchesAuthored'
command git status --porcelain -- cmd/
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/protected.sh"; "$T/scan.sh" "\bepochs?\b|run[ -]gate|gate facade|gate keys?\b|dispatch[ -]context|\bunarmed\b" "skills/**" "agents/**" "cursor-rules/**" .docket.example.yml "tests/*.sh" "internal/repoguard/*.go" "internal/harness/**/*.go"'
```
Expected: all green; no `cmd/` residue from the helper; `protected.sh` all `ok`. The scan prints only kept spellings (`gate_context_hash`, the gate drive's `--gate-context`, `dispatch_context`), each classified in the report.

- [ ] **Step 7: Commit**

```bash
command git add cursor-rules/run-tracker.md internal/assets/embedded AGENTS.md <each modified path, explicitly>
command git commit -m "docs: run-tracker vocabulary in skills, dispatch payload and AGENTS.md (change 0471)"
```

### Task 8: Docs — the concept page, the glossary and the guides

**Build profile:** standard

**Files:**
- Rename: `docs/concepts/run-gate.md` → `docs/concepts/run-tracker.md` (git mv)
- Modify: `docs/README.md`, `docs/concepts/README.md`, `docs/guide/building-without-supervision.md` (inbound links), `docs/concepts/build-profiles-and-gate.md`, `docs/comparison/ai-native-sdlc-playbook.md`, `docs/reference/harness/validation-runbook.md`, `docs/reference/glossary.md`; `internal/repoguard` pins on `validation-runbook.md` if a pinned sentence changes

**Interfaces:**
- Consumes: the final vocabulary from Tasks 2–7.
- Produces: `docs/concepts/run-tracker.md`; glossary entries for rows 1–38 and 38a–38d in the new names, with **no** old → new mapping (ADR-0129 Decision 10).

- [ ] **Step 1: Install the shared tools, then write the anchor checker (the failing check)**

```bash
bash -c '
T="${TMPDIR:-/tmp}/docket-0471"; mkdir -p "$T"
cat > "$T/anchors.pl" <<'"'"'EOF'"'"'
#!/usr/bin/perl
# anchors.pl GLOSSARY [FILE...] : GitHub-slugs GLOSSARY'"'"'s headings, then prints every
# in-page (#frag) link in GLOSSARY and every glossary.md#frag link in FILE... that
# names no heading. Last line: "N dangling".
use strict; use warnings;
my $g = shift @ARGV; open my $fh, "<", $g or die "$g: $!"; my @lines = <$fh>; close $fh;
my %slug; my $fence = 0;
for (@lines) {
  $fence = !$fence if /^\s*(```|~~~)/; next if $fence;
  next unless /^#{1,6}\s+(.*?)\s*$/;
  (my $s = lc $1) =~ s/[^\p{L}\p{N} _-]//g; $s =~ s/ /-/g; $slug{$s} = 1;
}
my $bad = 0;
my $n = 0; for (@lines) { $n++; while (/\]\(#([^)\s]+)\)/g) { next if $slug{$1}; print "$g:$n: #$1\n"; $bad++ } }
for my $f (@ARGV) { open my $h, "<", $f or next; my $l = 0;
  while (<$h>) { $l++; while (/glossary\.md#([^)\s]+)/g) { next if $slug{$1}; print "$f:$l: glossary.md#$1\n"; $bad++ } } }
print "$bad dangling\n";
EOF
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run && "$T/fileset.sh" . | xargs -0 perl "$T/anchors.pl" docs/reference/glossary.md'
```
Expected at this point: `0 dangling` (the base is consistent). It becomes the regression check for the heading renames below. Prove it bites: temporarily rename the heading `### Epoch fence` to `### X` (backup and restore); the checker must report the in-page links to `#epoch-fence`.

- [ ] **Step 2: Rename the concept page and fix its inbound links**

```bash
bash -c '
T="${TMPDIR:-/tmp}/docket-0471"
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run || exit 2
command git mv docs/concepts/run-gate.md docs/concepts/run-tracker.md
printf "%s\t%s\n" concepts/run-gate.md concepts/run-tracker.md ./run-gate.md ./run-tracker.md > "$T/task8-paths.tsv"
"$T/fileset.sh" docs | xargs -0 perl "$T/tokens.pl" "$T/task8-paths.tsv"
command git grep -n -F -e run-gate.md -- docs ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs"'
```
Expected: the final grep prints nothing. The link texts "The run gate and attribution" become "The run tracker and attribution" in Step 3.

- [ ] **Step 3: Rewrite the docs prose**

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/fileset.sh" docs | PROSE_MODE=safe xargs -0 perl "$T/prose.pl"
"$T/scan.sh" "\bepochs?\b|\b(re-?)?arm(s|ed|ing)?\b|\bunarmed\b|\bthe gate\b|gate record" docs'
```
Rewrite each printed line by meaning, as in Task 7 Step 4. Leave `rearm` / "re-arm" and "arm a stub" (auto-groom, family (d)). In `docs/reference/glossary.md`:
- Section `The run gate` → `The run tracker`; entry `Run gate` → `Run tracker`; entry `Arm / gate key / run epoch / dispatch context` → `Start / run key / run id / run context`; entry `Epoch fence` → `Run fence`; the `gate-context-invalid` / `gate-context-conflict` entry heading → `run-context-invalid` / `run-context-conflict`. Update the table of contents and the alphabetical index to the new names.
- **Delete** index aliases that point from a retired name to a new entry (for example `[Gate facade](#run-gate) — see Run gate`). ADR-0129 Decision 10 forbids an old → new mapping in the glossary.
- Fix every in-page link to a renamed heading. Re-run the checker until it prints `0 dangling`.

- [ ] **Step 4: Verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/fileset.sh" . | xargs -0 perl "$T/anchors.pl" docs/reference/glossary.md; "$T/protected.sh"; "$T/scan.sh" "\bepochs?\b|run[ -]gate|gate facade|gate keys?\b|dispatch[ -]context|\bunarmed\b|gate-(before|verdict|claim|armed)" docs'
go test -count=1 ./internal/repoguard/
```
Expected: `0 dangling`; `protected.sh` all `ok`; the scan prints only kept spellings (the gate drive's `--gate-context`, `gate_context_hash`, "Unix epoch"), each classified in the report; repoguard green (its `validation-runbook.md` pins realigned the Task 7 way if a pinned sentence changed).

- [ ] **Step 5: Commit**

```bash
command git add docs/concepts/run-tracker.md <each modified path, explicitly>
command git commit -m "docs: run-tracker concept page, glossary and guides (change 0471)"
```

### Task 9: The retired-vocabulary table and its absence seal

**Build profile:** premium

**Files:**
- Create: `internal/repoguard/retired_vocabulary_test.go`

**Interfaces:**
- Consumes: `harness.RunTracker`, `harness.CodexDispatchInterior`, `cursor.DispatchRuleContent`, `harness.PlanInput` / `harness.Order` / adapters, and `crossAgentsTable`, `stripHashComment`, `isMarkdownSurface`, `execPop`, `alwaysLoadedPop`, `maintainedPop`, `readMaintained`, `guardRoot` (repoguard test helpers).
- Produces: `retiredVocabulary []retiredToken`, the table that ADR-0129 families (b) and (d) append to (never a second table).

- [ ] **Step 1: Write the seal**

`internal/repoguard/retired_vocabulary_test.go`:

```go
package repoguard

import (
	"fmt"
	"go/scanner"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
	"github.com/danielhanold/docket/internal/install"
)

// This file is the retired-vocabulary table (ADR-0129 Decision 10) and the
// absence seal it drives. Change 0471 (family (a), the run tracker) created it;
// each later ADR-0129 family change APPENDS its rows to retiredVocabulary and
// never creates a second table. Every seal failure names the replacement.
//
// SCOPE, stated where the population is defined (learning
// byte-pattern-guard-matches-a-spelling): the seal scans the maintained
// executable surface (repoguard.ExecutableSurface: shell, exec-bit files,
// scripts/ and skills/ markdown), the always-loaded agent-instruction surface
// (repoguard.AlwaysLoadedSurface: AGENTS.md, CLAUDE.md, agents/**,
// cursor-rules/**), the STRING LITERALS of non-test Go source, and generator
// output. Markdown and shell are scanned line by line, with a `#` comment
// stripped from shell/config lines; markdown prose IS scanned, because a skill's
// prose names catalog operations an agent resolves and runs. Go comments and
// *_test.go files are NOT scanned: a comment is a passing mention, and a test
// may legitimately assert that an old spelling is refused. docs/, testdata/,
// tests/fixtures/ and internal/install/legacydata/ are excluded by
// MaintainedFiles itself (point-in-time records and frozen corpora).
//
// LIMITATION: the match is a bounded spelling, not the property. A retired
// token re-assembled at runtime from fragments (e.g. "gate-" + "armed") escapes
// it; that is accepted, as for every repoguard spelling seal.

// retiredKind selects which sites a retired row matches.
type retiredKind int

const (
	// kindToken: the bounded token anywhere in a scanned line or Go string
	// literal. Leading boundary: start or a non-[A-Za-z0-9_] byte (so a flag's
	// leading "--" and an op id's "run." prefix still match); trailing boundary:
	// end or a non-[A-Za-z0-9_-] byte (so gate-stop never matches gate-stopped,
	// and gate_context never matches the kept gate_context_hash).
	kindToken retiredKind = iota
	// kindGoFlag: a non-test Go string literal whose whole content equals Old —
	// a flag-definition or flag-lookup name ("run-epoch", "epoch").
	kindGoFlag
	// kindJSONKey: a non-test Go struct tag naming Old as its json key.
	kindJSONKey
	// kindGoPrefix: a non-test Go string literal that starts with Old — the
	// retired error-text prefix.
	kindGoPrefix
)

// retiredToken is one row of the retired-vocabulary table.
type retiredToken struct {
	Row  string      // ADR-0129 rename-table row, e.g. "7", "38c"
	Kind retiredKind // which sites match
	Old  string      // the retired spelling
	New  string      // its replacement, named in every failure
	// Scope, when non-nil, narrows a match to lines (or, for Go literals, to
	// files) where the retired spelling is the retired flag rather than a kept
	// namesake: row 12 retires `change claim --gate-context` only; the gate
	// drive's own --gate-context is not an ADR-0129 row and stays.
	Scope func(rel, line string) bool
}

var changeClaimLineRe = regexp.MustCompile(`change[ .]claim`)

// changeClaimScope confines row 12 to change-claim sites.
func changeClaimScope(rel, line string) bool {
	return rel == "internal/cli/change.go" || changeClaimLineRe.MatchString(line)
}

// retiredVocabulary is the table. Append-only across ADR-0129 families.
var retiredVocabulary = []retiredToken{
	// Family (a) — run tracker (change 0471): rows 7-37, 38 (storage names), 38a-38d.
	{Row: "7", Kind: kindToken, Old: "gate-before", New: "run.start / docket run start"},
	{Row: "8", Kind: kindToken, Old: "gate-verdict", New: "run.verdict / docket run verdict"},
	{Row: "9", Kind: kindToken, Old: "gate-claim", New: "run.continue / docket run continue"},
	{Row: "10", Kind: kindToken, Old: "--run-epoch", New: "--run-id"},
	{Row: "10", Kind: kindGoFlag, Old: "run-epoch", New: "run-id"},
	{Row: "11", Kind: kindToken, Old: "--epoch", New: "run cancel --run-id"},
	{Row: "11", Kind: kindGoFlag, Old: "epoch", New: "run-id"},
	{Row: "12", Kind: kindToken, Old: "--gate-context", New: "change claim --run-context", Scope: changeClaimScope},
	{Row: "12", Kind: kindGoFlag, Old: "gate-context", New: "run-context", Scope: changeClaimScope},
	{Row: "13", Kind: kindToken, Old: "gate-armed", New: "run-started"},
	{Row: "13", Kind: kindToken, Old: "gate-unarmed", New: "run-untracked"},
	{Row: "13", Kind: kindJSONKey, Old: "armed", New: "started"},
	{Row: "14", Kind: kindToken, Old: "gate-retry-once", New: "run-retry-once"},
	{Row: "14", Kind: kindToken, Old: "gate-continue", New: "run-continue"},
	{Row: "14", Kind: kindToken, Old: "gate-done", New: "run-done"},
	{Row: "14", Kind: kindToken, Old: "gate-stop", New: "run-stop"},
	{Row: "14", Kind: kindToken, Old: "gate-observe", New: "run-observe"},
	{Row: "15", Kind: kindToken, Old: "gate-claimed", New: "run-continued"},
	{Row: "16", Kind: kindToken, Old: "epoch-not-found", New: "run-not-found"},
	{Row: "17", Kind: kindToken, Old: "epoch-corrupt", New: "run-record-corrupt"},
	{Row: "18", Kind: kindToken, Old: "epoch-exists", New: "run-exists"},
	{Row: "19", Kind: kindToken, Old: "epoch-not-active", New: "run-not-active"},
	{Row: "20", Kind: kindToken, Old: "epoch-mismatch", New: "run-id-mismatch"},
	{Row: "21", Kind: kindToken, Old: "epoch-not-cancelled", New: "run-not-cancelled"},
	{Row: "22", Kind: kindToken, Old: "epoch-ambiguous", New: "run-ambiguous"},
	{Row: "23", Kind: kindToken, Old: "epoch-owner-ambiguous", New: "run-owner-ambiguous"},
	{Row: "24", Kind: kindToken, Old: "epoch-owner-unresolved", New: "run-owner-unresolved"},
	{Row: "25", Kind: kindToken, Old: "epoch-io", New: "run-record-io"},
	{Row: "26", Kind: kindToken, Old: "epoch-participant-unknown", New: "run-participant-unknown"},
	{Row: "27", Kind: kindToken, Old: "epoch-state-unknown", New: "run-state-unknown"},
	{Row: "28", Kind: kindToken, Old: "epoch-unreadable", New: "run-record-unreadable"},
	{Row: "29", Kind: kindToken, Old: "incumbent-epoch-fenced", New: "incumbent-run-fenced"},
	{Row: "30", Kind: kindToken, Old: "resume-epoch-unreadable", New: "resume-run-record-unreadable"},
	{Row: "31", Kind: kindToken, Old: "stale-run-epoch", New: "stale-run-id"},
	{Row: "32", Kind: kindToken, Old: "unknown-run-epoch", New: "unknown-run-id"},
	{Row: "33", Kind: kindToken, Old: "gate-unavailable", New: "run-tracker-unavailable"},
	{Row: "34", Kind: kindToken, Old: "gate-context-invalid", New: "run-context-invalid"},
	{Row: "34", Kind: kindToken, Old: "gate-context-conflict", New: "run-context-conflict"},
	{Row: "35", Kind: kindToken, Old: "mint-epoch", New: "mint-run"},
	{Row: "35", Kind: kindToken, Old: "find-epoch", New: "find-run"},
	{Row: "35", Kind: kindToken, Old: "complete-epoch", New: "complete-run"},
	{Row: "35", Kind: kindToken, Old: "supersede-epoch", New: "supersede-run"},
	{Row: "36", Kind: kindToken, Old: "write-epoch", New: "write-run-record"},
	{Row: "36", Kind: kindToken, Old: "load-epoch", New: "load-run-record"},
	{Row: "36", Kind: kindToken, Old: "epoch-cas", New: "run-record-cas"},
	{Row: "37", Kind: kindToken, Old: "bind-epoch-change", New: "bind-run-change"},
	{Row: "37", Kind: kindToken, Old: "bind-epoch-worktree", New: "bind-run-worktree"},
	{Row: "37", Kind: kindToken, Old: "epoch-launch-gate", New: "run-launch-gate"},
	{Row: "37", Kind: kindToken, Old: "reserve-worktree-execution-epoch", New: "reserve-worktree-execution-run"},
	{Row: "37", Kind: kindToken, Old: "retire-worktree-execution-epoch", New: "retire-worktree-execution-run"},
	{Row: "37", Kind: kindGoPrefix, Old: "run epoch ", New: "error-text prefix \"run \""},
	{Row: "38", Kind: kindToken, Old: "rungate", New: "run-tracker (storage root)"},
	{Row: "38", Kind: kindToken, Old: "rungate-resume", New: "run-tracker-resume (storage root)"},
	{Row: "38", Kind: kindToken, Old: "epoch.json", New: "run.json"},
	{Row: "38", Kind: kindToken, Old: "epoch.lock", New: "run.lock"},
	{Row: "38a", Kind: kindToken, Old: "--run-gate-key", New: "--run-key"},
	{Row: "38a", Kind: kindGoFlag, Old: "run-gate-key", New: "run-key"},
	{Row: "38b", Kind: kindJSONKey, Old: "epoch", New: "run_id"},
	{Row: "38c", Kind: kindToken, Old: "gate_context", New: "run_context"},
	{Row: "38d", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_EPOCH", New: "DOCKET_AGENT_GUARDIAN_RUN_ID"},
}

// retiredHit is one seal violation.
type retiredHit struct {
	rel  string
	line int
	row  retiredToken
	text string
}

func (h retiredHit) String() string {
	return fmt.Sprintf("%s:%d: ADR-0129 row %s: retired %q — use %s: %s", h.rel, h.line, h.row.Row, h.row.Old, h.row.New, h.text)
}

var tokenReCache = map[string]*regexp.Regexp{}

// tokenRe is the bounded matcher for a kindToken row (see kindToken).
func tokenRe(old string) *regexp.Regexp {
	if re, ok := tokenReCache[old]; ok {
		return re
	}
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(old) + `([^A-Za-z0-9_-]|$)`)
	tokenReCache[old] = re
	return re
}

// scanTextLine checks one markdown/shell/generated line against every
// kindToken row.
func scanTextLine(rel string, lineNo int, line string) []retiredHit {
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		if r.Kind != kindToken {
			continue
		}
		if tokenRe(r.Old).MatchString(line) && (r.Scope == nil || r.Scope(rel, line)) {
			hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(line)})
		}
	}
	return hits
}

// scanTextContent scans a markdown/shell/config file. Shell and config lines
// have their `#` comment stripped (stripHashComment, absence_test.go); markdown
// is scanned whole, prose included.
func scanTextContent(rel, content string) []retiredHit {
	var hits []retiredHit
	md := isMarkdownSurface(rel)
	for i, raw := range strings.Split(content, "\n") {
		line := raw
		if !md {
			line = stripHashComment(raw)
		}
		hits = append(hits, scanTextLine(rel, i+1, line)...)
	}
	return hits
}

var jsonTagKeyRe = regexp.MustCompile(`json:"([^",]*)`)

// scanGoLiteral checks one Go string literal's content against every row.
func scanGoLiteral(rel string, lineNo int, lit string) []retiredHit {
	val := lit
	if unq, err := strconv.Unquote(lit); err == nil {
		val = unq
	}
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		if r.Scope != nil && !r.Scope(rel, "") {
			continue
		}
		hit := false
		switch r.Kind {
		case kindToken:
			hit = tokenRe(r.Old).MatchString(val)
		case kindGoFlag:
			hit = val == r.Old
		case kindGoPrefix:
			hit = strings.HasPrefix(val, r.Old)
		case kindJSONKey:
			for _, m := range jsonTagKeyRe.FindAllStringSubmatch(val, -1) {
				if m[1] == r.Old {
					hit = true
				}
			}
		}
		if hit {
			hits = append(hits, retiredHit{rel, lineNo, r, lit})
		}
	}
	return hits
}

// scanGoSource tokenizes one non-test Go file and scans its string literals
// (interpreted and raw, struct tags included). Comments are not scanned.
func scanGoSource(rel string, src []byte) ([]retiredHit, error) {
	fset := token.NewFileSet()
	file := fset.AddFile(rel, fset.Base(), len(src))
	var s scanner.Scanner
	var scanErr error
	s.Init(file, src, func(pos token.Position, msg string) { scanErr = fmt.Errorf("%s: %s", pos, msg) }, 0)
	var hits []retiredHit
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.STRING {
			hits = append(hits, scanGoLiteral(rel, fset.Position(pos).Line, lit)...)
		}
	}
	return hits, scanErr
}

// goSourcePop returns the maintained non-test Go files.
func goSourcePop(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range maintainedPop(t, root) {
		if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") {
			out = append(out, rel)
		}
	}
	return out
}

// TestRetiredVocabularySeal is the ADR-0129 absence seal.
func TestRetiredVocabularySeal(t *testing.T) {
	t.Run("table_integrity", testRetiredTableIntegrity)
	t.Run("non_vacuity", testRetiredNonVacuity)
	t.Run("negative_controls", testRetiredNegativeControls)
	t.Run("maintained_surfaces", testRetiredMaintainedSurfaces)
	t.Run("generator_output", testRetiredGeneratorOutput)
}

// testRetiredTableIntegrity: a malformed row would seal nothing or name no
// replacement. The floor stops a truncated table from passing vacuously.
func testRetiredTableIntegrity(t *testing.T) {
	const floor = 60
	if len(retiredVocabulary) < floor {
		t.Fatalf("retired-vocabulary table has %d rows, expected >= %d", len(retiredVocabulary), floor)
	}
	seen := map[string]bool{}
	for _, r := range retiredVocabulary {
		if r.Row == "" || r.Old == "" || r.New == "" {
			t.Errorf("incomplete row %+v", r)
		}
		key := fmt.Sprintf("%d|%s", r.Kind, r.Old)
		if seen[key] {
			t.Errorf("duplicate row for %q", r.Old)
		}
		seen[key] = true
	}
}

// testRetiredNonVacuity plants every row in each site of its kind and demands
// a hit ATTRIBUTED TO THAT ROW that names its replacement: a detector that
// stopped matching reddens here. (A token nested in a longer retired token, e.g.
// epoch-unreadable inside resume-epoch-unreadable, also hits its own shorter
// row; only the planted row's own hits are counted.)
func testRetiredNonVacuity(t *testing.T) {
	goHits := func(rel, src string) []retiredHit {
		t.Helper()
		hits, err := scanGoSource(rel, []byte(src))
		if err != nil {
			t.Fatalf("tokenize planted source: %v", err)
		}
		return hits
	}
	for _, r := range retiredVocabulary {
		var hits []retiredHit
		want := 0
		switch r.Kind {
		case kindToken:
			line := "run `" + r.Old + "` now"
			goRel := "internal/p/p.go"
			if r.Scope != nil {
				line = "docket change claim " + r.Old + " x"
				goRel = "internal/cli/change.go"
			}
			hits = append(hits, scanTextContent("skills/x/SKILL.md", line)...)
			hits = append(hits, scanTextContent("tests/test_x.sh", "docket "+line)...)
			hits = append(hits, goHits(goRel, "package p\nvar v = "+strconv.Quote("x "+r.Old+" y")+"\n")...)
			want = 3
		case kindGoFlag, kindGoPrefix:
			rel := "internal/cli/run.go"
			if r.Scope != nil {
				rel = "internal/cli/change.go"
			}
			lit := r.Old
			if r.Kind == kindGoPrefix {
				lit = r.Old + "%s"
			}
			hits = goHits(rel, "package p\nvar v = "+strconv.Quote(lit)+"\n")
			want = 1
		case kindJSONKey:
			hits = goHits("internal/p/p.go", "package p\ntype T struct {\n\tF string `json:\""+r.Old+",omitempty\"`\n}\n")
			want = 1
		}
		own := 0
		for _, h := range hits {
			if h.row.Kind != r.Kind || h.row.Old != r.Old {
				continue
			}
			own++
			if !strings.Contains(h.String(), r.New) {
				t.Errorf("row %s: failure %q does not name the replacement %q", r.Row, h, r.New)
			}
		}
		if own < want {
			t.Errorf("row %s: %q detected in %d of %d planted sites", r.Row, r.Old, own, want)
		}
	}
}

// testRetiredNegativeControls: kept namesakes must never match (spec §4 shape
// boundaries). Each is a real spelling the tree legitimately carries.
func testRetiredNegativeControls(t *testing.T) {
	cleanText := []string{
		"the claim receipt keeps gate_context_hash",
		"releasepkg --source-epoch 1700000000",
		"SOURCE_DATE_EPOCH=1700000000",
		"the suite gate-failed; a gate-scope participant",
		"gatedrive-owned drives and idempotent-suite-gate",
		"evidence reason gate-stopped",
		"docket gate drive prepare-scope --gate-context <ctx>",
		"docket gate drive start --gate-context <ctx> --run-id <id>",
		"run.start then run.verdict then run.continue",
	}
	for _, line := range cleanText {
		for _, rel := range []string{"skills/x/SKILL.md", "tests/test_x.sh"} {
			if hits := scanTextContent(rel, line); len(hits) != 0 {
				t.Errorf("negative control %q in %s matched: %v", line, rel, hits)
			}
		}
	}
	cleanGo := []struct{ rel, src string }{
		{"internal/app/change_claim.go", "package p\ntype R struct {\n\tH string `json:\"gate_context_hash\"`\n}\n"},
		{"cmd/releasepkg/main.go", "package p\nvar f = \"source-epoch\"\n"},
		{"internal/cli/gate.go", "package p\nvar f = \"gate-context\"\n"},
		{"internal/app/evidence_ops.go", "package p\nvar r = \"gate-stopped\"\n"},
		{"internal/p/p.go", "package p\n// run epoch gate-armed in a comment is a passing mention\nvar x = 1\n"},
	}
	for _, c := range cleanGo {
		hits, err := scanGoSource(c.rel, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Errorf("negative control %s matched: %v", c.rel, hits)
		}
	}
}

// testRetiredMaintainedSurfaces scans the committed tree.
func testRetiredMaintainedSurfaces(t *testing.T) {
	root := guardRoot(t)
	text := execPop(t, root)
	for _, rel := range alwaysLoadedPop(t, root) {
		if !slices.Contains(text, rel) {
			text = append(text, rel)
		}
	}
	goPop := goSourcePop(t, root)
	// Population floors FIRST: a broken walk passes every absence assert vacuously.
	if len(text) < 45 {
		t.Fatalf("population floor: text surface collapsed to %d files (expected >= 45)", len(text))
	}
	if len(goPop) < 150 {
		t.Fatalf("population floor: non-test Go surface collapsed to %d files (expected >= 150)", len(goPop))
	}
	var violations []string
	for _, rel := range text {
		if strings.HasSuffix(rel, ".go") {
			continue // Go files are scanned by literal below, never as text
		}
		for _, h := range scanTextContent(rel, readMaintained(t, root, rel)) {
			violations = append(violations, h.String())
		}
	}
	for _, rel := range goPop {
		hits, err := scanGoSource(rel, []byte(readMaintained(t, root, rel)))
		if err != nil {
			t.Fatalf("tokenize %s: %v (fail closed)", rel, err)
		}
		for _, h := range hits {
			violations = append(violations, h.String())
		}
	}
	if len(violations) != 0 {
		t.Errorf("retired ADR-0129 vocabulary survives in maintained executable surfaces (%d):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// testRetiredGeneratorOutput renders every dispatch surface docket writes into
// consumer repos — each adapter's targets, the shared dispatch interior (Codex
// variant, a superset), and the cursor rule — and scans every emitted line,
// prose included.
func testRetiredGeneratorOutput(t *testing.T) {
	cat, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	in := harness.PlanInput{
		Assets:    cat,
		Mode:      harness.ModeRelease,
		AssetsDir: "/data/versions/sha256-x/assets",
		Roots: install.UserRoots{
			Home:       "/home/u",
			DataRoot:   "/home/u/.local/share/docket",
			ConfigHome: "/home/u/.config",
			BinDir:     "/home/u/.local/bin",
		},
		Agents: crossAgentsTable(),
	}
	outputs := map[string][]byte{}
	adapters := map[string]harness.Adapter{
		"claude": claude.New(), "codex": codex.New(), "cursor": cursor.New(), "opencode": opencode.New(),
	}
	for _, name := range harness.Order {
		targets, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s Plan: %v", name, err)
		}
		for i, tg := range targets {
			outputs[fmt.Sprintf("%s/%03d-%s", name, i, filepath.Base(tg.Path))] = tg.Content
		}
	}
	payload, err := harness.RunTracker(cat)
	if err != nil {
		t.Fatalf("RunTracker: %v", err)
	}
	outputs["dispatch/codex-interior.md"] = []byte(harness.CodexDispatchInterior(payload))
	outputs["dispatch/cursor-rule.mdc"] = cursor.DispatchRuleContent(payload)
	if len(outputs) < 20 {
		t.Fatalf("generator population floor: %d outputs (expected >= 20)", len(outputs))
	}
	var violations []string
	for rel, b := range outputs {
		for i, line := range strings.Split(string(b), "\n") {
			for _, h := range scanTextLine(rel, i+1, line) {
				violations = append(violations, "[generator-output] "+h.String())
			}
		}
	}
	slices.Sort(violations)
	if len(violations) != 0 {
		t.Errorf("retired ADR-0129 vocabulary in GENERATOR OUTPUT (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
```

This code was validated at planning time against the base tree with `harness.RunGate` in place of `harness.RunTracker`: `go vet` was clean, `table_integrity`, `non_vacuity` and `negative_controls` passed, and `maintained_surfaces` / `generator_output` reported 217 base-tree violations, all of them retired tokens. Test code handed over by a plan is still unverified code (learning `plan-supplied-test-code-is-unverified`): Step 3's probes are what prove it.

- [ ] **Step 2: Run it (it should pass after Tasks 1–8)**

Run: `go test -count=1 ./internal/repoguard/ -run TestRetiredVocabularySeal -v`
Expected: all five subtests PASS. A `maintained_surfaces` or `generator_output` failure is a missed site: fix it in its owning file (regenerating the mirror, goldens and `AGENTS.md` if it is an authored root), never by weakening a row or adding an exclusion. The one exception is a fixture that must carry a retired value. It gets a bounded exclusion, mutation-tested (learning `frozen-fixture-corpus-trips-repo-wide-scans`). None is expected.

- [ ] **Step 3: Mutation probes — each must go RED, then GREEN after restore**

Each probe: back up the file, mutate it, confirm the mutation landed with a count, run `go test -count=1 ./internal/repoguard/ -run TestRetiredVocabularySeal`, restore from the backup.
1. Append the line ``Resolve the `run.gate-verdict` operation.`` to `skills/docket-implement-next/SKILL.md`. `maintained_surfaces` fails naming `run.verdict`.
2. Add `var _ = "rungate"` to `internal/app/runtracker_store.go`. `maintained_surfaces` fails naming `run-tracker (storage root)`.
3. Append ` Print gate-armed.` to the last paragraph of `CodexRootEntryClause` in `internal/harness/dispatch.go`. `generator_output` fails naming `run-started`.
4. In `tokenRe`, change the trailing class `[^A-Za-z0-9_-]` to `[^A-Za-z0-9_]`. `negative_controls` fails on `gate-stopped` (the boundary is load-bearing).
5. Delete `Scope: changeClaimScope` from the row-12 `kindToken` entry. `maintained_surfaces` fails on the gate drive's kept `--gate-context` in `skills/docket-build/SKILL.md` (the scope is load-bearing).
6. Drop the `case kindGoFlag:` arm's assignment (`hit = val == r.Old` → `hit = false`). `non_vacuity` fails for rows 10, 11, 12 and 38a.

Record every probe's before/after reading in the task report.

- [ ] **Step 4: Commit**

```bash
command git add internal/repoguard/retired_vocabulary_test.go
command git commit -m "test: retired-vocabulary table and absence seal for the run tracker (change 0471)"
```

### Task 10: Residual audit and the full suite

**Build profile:** standard

**Files:**
- Modify: only residual sites the audit finds (each fixed in its owning file)

**Interfaces:**
- Consumes: everything above.
- Produces: the classified residual list for the task report, and a green full suite.

- [ ] **Step 1: Install the shared tools, then run the whole-repo retired-phrase scan**

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"
"$T/scan.sh" "implement-next"   # positive control: must print lines
for re in "\bepochs?\b" "run[ -_]?gate" "gate[ -]facade" "gate[ -]keys?\b" "dispatch[ -_]context" "\bunarmed\b|\b(re-?)?arm(s|ed|ing)?\b" "gate-(before|verdict|claim|armed|unarmed|retry-once|continue|done|stop|observe|claimed|unavailable|context-invalid|context-conflict)\b" "--run-epoch|--run-gate-key|(?<![\w-])--epoch\b|DOCKET_AGENT_GUARDIAN_EPOCH|\bgate_context\b(?!_hash)"; do
  echo "=== $re"; "$T/scan.sh" "$re"; done'
bash -c 'cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run && command git grep -n -i -E -e "epoch|rungate|run.gate|gate.key|dispatch.context" -- internal/gatedrive/storage_reset_test.go internal/app/runtracker_storage_reset_integration_test.go'
```
Classify every printed line into exactly one bucket:
- **(a) kept name** (Global Constraints: gate drive `--gate-context`, `gate_context_hash`, `dispatch_context`, `DOCKET_AGENT_GUARDIAN_GATE_KEY`, `gate-scope`, `gatelifecycle`, suite-gate evidence reasons, Unix-epoch phrases);
- **(b) family (d) or unrelated "arm"** (auto-groom `rearm` / re-arm, "arm a stub", the suite runner's budget "armed", githubcli / workspace senses, `TestIntegration…Arm…` test names);
- **(c) storage-reset fixture** (the two storage-reset tests' retired-layout strings, and comments that explain the reset);
- **(d) test assertion that an old spelling is refused** (for example `TestRunTrackerVocabularyHardCut`, `TestAdmissionRecordPersistsRunIDKey`);
- **(e) the retired-vocabulary table itself.**

A line that fits no bucket is a missed site: fix it in its owning file (regenerating the mirror, goldens and `AGENTS.md` if it is an authored root), then re-run. Paste the classified list into the report. Point-in-time records and ADR-0129 are excluded by `fileset.sh` and are not residuals.

- [ ] **Step 2: Scope and integrity checks**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run
command git diff --name-only e68669728ed736d5b5195a0807e5734fd6e3f81e -- docs/adrs docs/changes docs/results testdata tests/fixtures internal/install/legacydata 'internal/install/legacy*.go' docs/codex/fixtures docs/reference/harness/fixtures internal/release cmd/releasepkg internal/gitcli .github '**/testdata/**' ':!internal/harness/*/testdata/golden/**'
command git diff --name-only e68669728ed736d5b5195a0807e5734fd6e3f81e -- docs/superpowers
command git diff --no-renames --name-status e68669728ed736d5b5195a0807e5734fd6e3f81e > "${TMPDIR:-/tmp}/docket-0471/name-status.txt"
grep -E -e '^[AD]' "${TMPDIR:-/tmp}/docket-0471/name-status.txt"
command git status --porcelain
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; "$T/protected.sh"'
go run ./cmd/docket capabilities --json > "${TMPDIR:-/tmp}/docket-0471/caps.json" && go run ./cmd/docket schema --json > "${TMPDIR:-/tmp}/docket-0471/schema.json"
bash -c 'T="${TMPDIR:-/tmp}/docket-0471"; grep -c -E -e "run\.gate-|\"gate_context\"|--run-epoch|--run-gate-key" "$T/caps.json" "$T/schema.json"'
```
Expected:
- The first command prints nothing. The second prints only this plan.
- The `A` / `D` list holds exactly these, and nothing else:
  - `D` the 49 old paths: the 47 in `$T/renames.txt`, plus `cursor-rules/run-gate.md` and `docs/concepts/run-gate.md`;
  - `A` their 49 new paths;
  - `A` this plan, the three Task 1 tests, `internal/cli/run_tracker_rename_test.go`, and `internal/repoguard/retired_vocabulary_test.go`.

  `--no-renames` is deliberate: git's rename detection would print only a move's destination and hide a source path (learning `diff-derived-allowlist-needs-no-renames`).
- `git status` is clean and `protected.sh` is all `ok`.
- The final grep reports `0` for both files: the golden `capabilities` / `schema` output shows only new names.

- [ ] **Step 3: Full suite (the build gate command)**

Run from `$W`: `go run ./cmd/docket development test`
Expected: the `SUITE …` summary line reports green. Read the budget report even when green, and report any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:` or `SERIAL CONFIRMED OVER BUDGET:` line. The six renamed shards carry their old budget values unchanged (learning `budget-headroom-is-spent-before-it-is-breached`). For a red result, root-cause it; never weaken a test. A golden or embedded-drift failure means a regeneration step was skipped in the task that touched that root.

- [ ] **Step 4: Commit fix-ups (only if Steps 1–3 required edits)**

```bash
command git add <each fixed path, explicitly>
command git commit -m "fix: residual run-tracker sites found in the audit (change 0471)"
```
