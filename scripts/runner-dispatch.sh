#!/usr/bin/env bash
# scripts/runner-dispatch.sh — the runner-neutral delegation facade (change 0079), behind
# `docket.sh runner-dispatch`. Validates arguments, anchors the repo root (ADR-0034),
# resolves the runners.<name>: config block across layers (repo-local > repo-committed >
# global; per-key), exports it as DOCKET_RUNNER_CFG_<KEY>, and CALLS the named adapter
# scripts/runners/<name>.sh in the foreground, returning with its verbatim exit code (change 0237
# replaced the original `exec` so the facade regains control at that seam). On that seam sits the
# RUN GATE: for an `implement-next` delegation only, an unfinished run gets ONE re-dispatch and a
# second strike exits 1, a halted run stops the caller with 3, and a re-dispatch that drove the run
# to completion exits 0 over a stale first code — the only three paths where the facade does not
# return the adapter's own code. That synchronous seam now serves HAND invocations: every generated
# shim launches, so the same gate follows the delegated run onto the `--observe` seam below.
# Registration IS the adapter file's existence. Unknown runner => loud nonzero (abort-and-report).
# Change 0271 added the `--launch` verb: the same validated request, but the adapter is started in
# its OWN process group with every stream redirected into a durable per-dispatch dir, and the call
# returns at once with a dispatch key instead of blocking for the child's whole run. Its other half
# is `--observe <key>`: ONE short, idempotent look at that dispatch, synthesizing 0 (complete),
# 1 (failed or unavailable), 3 (a delegated implement-next run HALTED — the synchronous gate's own
# code, reached from the seam a delegated run actually returns through) or 4 (still running — NOT a
# failure, observe again), and killing the detached PROCESS GROUP when it gives up rather than
# orphaning the adapter. It gives up on two conditions: the observation budget being SPENT, and the
# budget being UNENFORCEABLE (an unreadable clock, or a launch record whose start time cannot be
# read) on N consecutive passes — a `4` that no observation could ever leave is a caller loop with
# no exit, which is the same unbounded run this change exists to remove.
# On that seam sits the DISAGREEMENT RULE: a sentinel claiming success with no matching git evidence
# is a failure. For a `build-*` agent the facade reads verify-run's build verdict; for
# `implement-next` it runs change 0237's run gate against the before-snapshot `--launch` recorded.
# BOTH report and stop — an observation never re-dispatches: a build task may have left partial
# commits, and re-launching a detached child out of a repeated short read would race the very run
# being observed.
# Contract: scripts/runner-dispatch.md.
# Mock seams: RUNNERS_DIR, GIT, DOCKET_AGENTS_SRC.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNERS_DIR="${RUNNERS_DIR:-$SELF_DIR/runners}"
# The built-in agent sources, read at runtime for one field only: `worktree-scope:` (change 0208).
# The resolution path mirrors the adapters' own (`AGENTS_SRC="$SELF_DIR/../../agents"` in
# scripts/runners/codex.sh), with the depth adjusted because the facade sits one level shallower.
# Consumer repos run the facade out of DOCKET_SCRIPTS_DIR, so ../agents exists wherever it runs.
# The env override is a MOCK SEAM this change introduces — the adapters have no such override.
# DOCKET_-NAMESPACED per ADR-0014 ("every env var docket introduces is DOCKET_-namespaced ... to
# avoid collisions in the user's shared shell"), unlike the pre-0208 seams either side of it. The
# rule earns its keep hardest here: this value is the input BOTH delegation gates key on, and the
# un-namespaced spelling is one an unrelated tool could plausibly export — docket's own adapters and
# sync-agents.sh use exactly that name for their internal variable. A stray `AGENTS_SRC` reaching
# this seam would not merely mock it, it would decide whether the gates are armed at all. Renaming
# the neighbours is a separate change; introducing a new seam un-namespaced would not be.
DOCKET_AGENTS_SRC="${DOCKET_AGENTS_SRC:-$SELF_DIR/../agents}"
# The git seam. lib/docket-root.sh reads the same variable; naming it here makes it a seam of this
# script too, which the change-0271 launch verb uses directly for the dispatch-time SHA.
GIT="${GIT:-git}"
# Run-gate seams (change 0237): the disposition reader, and the facade used for the metadata
# re-syncs that make BOTH snapshots — "before" and "after" — reads of fresh origin state.
VERIFY_RUN="${VERIFY_RUN:-$SELF_DIR/verify-run.sh}"
DOCKET_FACADE="${DOCKET_FACADE:-$SELF_DIR/docket.sh}"
# shellcheck source=lib/docket-root.sh
. "$SELF_DIR/lib/docket-root.sh"
# shellcheck source=lib/docket-dispatch-dir.sh
. "$SELF_DIR/lib/docket-dispatch-dir.sh"
# shellcheck source=lib/docket-agent-scope.sh
# agent_worktree_scope — THE SAME reader sync-agents.sh validates with. Shared rather than
# duplicated: the value semantics cannot drift (generation rejects anything but feature/metadata)
# but the extraction can, and a spelling change made in the generator alone fails loudly there
# while leaving this probe silently reading every agent as metadata scope.
. "$SELF_DIR/lib/docket-agent-scope.sh"
# shellcheck source=lib/docket-liveness.sh
# The liveness predicate, shared with gate-run.sh (change 0284). Before this lib, the identity
# conjuncts below existed here as an inline ladder and in gate-run.sh as its own copy, and the two
# had already diverged: on an EMPTY recorded token this file SKIPPED the conjunct while gate-run.sh
# failed closed. The lib is fail-closed, and this file inherits that — see `terminate_dispatch`'s
# "IDENTITY BEFORE SIGNAL" header for why the change is behaviour-preserving on every reachable
# input. `docket_identity_of` also stands in for the private start-time reader this file used to
# carry beside the ladder, which was that same predicate's other half.
. "$SELF_DIR/lib/docket-liveness.sh"

die(){ printf 'runner-dispatch: %s\n' "$*" >&2; exit 1; }

RUNNER=""; AGENT=""; MODEL=""; EFFORT=""; WORKTREE=""; BRIEF_FILE=""
# Verb selection (change 0271). Empty = the legacy synchronous call-and-return, which stays the
# default so no currently-shipped caller changes behavior. `--observe` is parsed HERE, with
# `--launch`, so the two verbs are validated together even though its branch lands with Task 4.
VERB=""; OBSERVE_KEY=""
while [ $# -gt 0 ]; do
  case "$1" in
    --runner) [ $# -ge 2 ] || die "--runner requires a value"; RUNNER="$2"; shift 2 ;;
    --agent)  [ $# -ge 2 ] || die "--agent requires a value";  AGENT="$2";  shift 2 ;;
    --model)  [ $# -ge 2 ] || die "--model requires a value";  MODEL="$2";  shift 2 ;;
    --effort) [ $# -ge 2 ] || die "--effort requires a value"; EFFORT="$2"; shift 2 ;;
    --worktree) [ $# -ge 2 ] || die "--worktree requires a value"; WORKTREE="$2"; shift 2 ;;
    --launch)  VERB="launch"; shift ;;
    # `--observe` and `--brief-file` keep the shift-then-conditional-shift shape they were written
    # with; the arms above use the `[ $# -ge 2 ] || die` shape instead. Both are guards against the
    # same hazard — bash's `shift` FAILS rather than truncating when the flag is the last argument,
    # and this loop has no trailing shift, so an unguarded value-taking flag in final position spins
    # here forever. `--observe` additionally needs the value to stay OPTIONAL at parse time so its
    # own "--observe requires a dispatch key" refusal below is the one a caller sees.
    --observe) VERB="observe"; OBSERVE_KEY="${2:-}"; shift; [ $# -gt 0 ] && shift ;;
    # Same last-argument hazard as `--observe` above: shift the flag, then the value only if a
    # value is actually there. The refusal is inline rather than deferred to the validation block
    # below, because a value-less `--brief-file` leaves BRIEF_FILE empty and would otherwise be
    # indistinguishable from "no brief file was passed" — a silently payload-free dispatch.
    --brief-file) BRIEF_FILE="${2:-}"; [ -n "$BRIEF_FILE" ] || die "--brief-file requires a path"; shift; [ $# -gt 0 ] && shift ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$RUNNER" ] || die "--runner is required"
[ -n "$AGENT" ]  || die "--agent is required"
# `inherit` is DOCKET'S OWN no-pin sentinel (never a vendor model ID), normalized to "no model"
# HERE — the one layer every adapter is dispatched through — so no adapter re-decides it. The
# handoff below already gates the flag on `[ -n "$MODEL" ]`, which makes the sentinel
# indistinguishable from "no model supplied": exactly the model-less hand path every adapter
# contract documents. NORMALIZE, not reject — ADR-0067 already rejects the sentinel at GENERATION
# time (sync-agents.sh's runner_config_error), so a dispatch-time `inherit` is a hand invocation,
# and the hand contract is tolerant. This is sentinel normalization, NOT model-ID validation
# (ADR-0015): no vendor value is inspected and no allowlist is introduced.
case "$MODEL" in inherit) MODEL="" ;; esac
# The runner name becomes a path component below — reject anything that could traverse out
# of RUNNERS_DIR (the facade family is a finite table, never an escape hatch).
case "$RUNNER" in
  *[!A-Za-z0-9._-]*|*..*) die "invalid runner name '$RUNNER'" ;;
esac
# --- the agent's DECLARED worktree scope (change 0208) -------------------------------
# `feature` means the agent must run inside the feature worktree it serves; anything else — including
# a source file or key that cannot be read — is metadata scope. The read is TOLERANT by design and
# the loud seam is elsewhere: sync-agents.sh's validate_agent_scopes refuses to generate a wrapper
# for an undeclared agent, which is where absence is preventable. Dying here instead would shadow
# the adapter's more specific unknown-agent diagnostic and would break a probe of any agent that is
# not a built-in.
# $AGENT becomes a PATH COMPONENT below, so it earns the same shape-keyed treatment `--runner` gets
# above. It is skipped rather than fatal, for the tolerance reason just given: an off-shape name has
# no declared scope and reaches the adapter, which names it precisely.
#
# THE SOURCES DIRECTORY IS THE PROBE'S PRECONDITION, and it is LOUD — the per-file tolerance above
# stops at the file. A missing, misdirected, or unreadable $DOCKET_AGENTS_SRC is a different
# condition on the same code path: every agent resolves to metadata scope AT ONCE, so gate 1 stops
# requiring --worktree and gate 3b stops rejecting the main tree, and a feature-scoped worker — a
# child harness that may execute under an auto-approve permission grant — is handed the primary
# checkout on the integration branch with nothing printed. That is a gate green because it never
# ran. No narrower posture exists: with no sources the facade cannot tell a feature-scoped agent
# from a metadata-scoped one, so refusing only the feature-scoped ones is not a thing it can do.
# The price is that a metadata dispatch, which never needed the read, dies too — a loud install
# failure naming the one variable to fix, weighed against a silent delegation of the main tree.
# No shipped deployment pays it: the facade always runs from the docket clone's scripts/ (consumer
# repos via DOCKET_SCRIPTS_DIR, ADR-0014), whose sibling agents/ is committed beside it.
# Keyed on SHAPE — "does this directory hold docket agent sources" — never on `[ -d ]` alone, which
# a misdirected path satisfies while every scope read inside it still comes back empty, and never on
# probing one agent's file, which is the tolerated case. The glob is unquoted and `nullglob` is
# unset, so a non-matching pattern survives as its own literal and fails `-f`.
agents_src_found=0
for agent_src in "$DOCKET_AGENTS_SRC"/docket-*.md; do
  [ -f "$agent_src" ] || continue
  agents_src_found=1; break
done
[ "$agents_src_found" = 1 ] || die "no built-in agent sources under DOCKET_AGENTS_SRC=$DOCKET_AGENTS_SRC — every agent's worktree-scope: would read as metadata, silently disarming the --worktree requirement and the main-tree rejection; refusing every dispatch rather than admitting a feature-scoped run into the primary checkout"
AGENT_SCOPE=""
case "$AGENT" in
  *[!A-Za-z0-9._-]*|*..*) ;;
  *) case "$(agent_worktree_scope "$DOCKET_AGENTS_SRC/docket-$AGENT.md")" in
       feature) AGENT_SCOPE="feature" ;;
     esac ;;
esac
ADAPTER="$RUNNERS_DIR/$RUNNER.sh"
if [ ! -f "$ADAPTER" ]; then
  registered="$(ls "$RUNNERS_DIR" 2>/dev/null | sed -n 's/\.sh$//p' | tr '\n' ' ')"
  die "unknown runner '$RUNNER' — no adapter at $ADAPTER (registered runners: ${registered:-<none>})"
fi

# --- anchor: an explicit argument defaulting to the main worktree (change 0206) -----
# ADR-0034 is UNAMENDED. Routing --worktree through docket_anchor_path rather than using it raw
# is the whole point: a relative value joins to the MAIN worktree, so it resolves identically from
# any cwd, and the new argument inherits ADR-0034's cwd-independence instead of quietly
# reintroducing the hazard ADR-0034 was written against. Absent --worktree the expression is
# docket_anchor_path "." — the main worktree — so every currently-shipped shim is unaffected.
REPO_ROOT="$(docket_main_worktree)"
[ -n "$REPO_ROOT" ] || die "not inside a git repository"
# --- change 0277: brief-file validation, at the SAME pre-verb point as gate 1 ---------
# Both payload channels are known here — `--brief-file` from the parse loop, the trailing argv as
# the surviving positional parameters — which is why these gates sit here rather than inside a
# verb: scoping them to `--launch` would leave the legacy foreground verb, the hand-invocation
# path, free to dispatch a task-less build worker silently.
if [ -n "$BRIEF_FILE" ]; then
  # BOTH CHANNELS ⇒ REFUSE. Preferring either silently drops or duplicates the child's entire
  # input, and concatenating invents an ordering; refusal is the only shape with no
  # silent-wrong-answer mode. The adapters carry the same refusal defensively.
  [ $# -eq 0 ] || die "both --brief-file and trailing arguments after '--' were given — pass the brief in the file OR after '--', never both"
  [ -f "$BRIEF_FILE" ] && [ -r "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is not a readable file"
  [ -s "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is empty — a child launched with no task does not error, it improvises"
  # `-s` MEASURES BYTES, and the adapters measure CONTENT: they read the brief with `$(cat …)`,
  # which strips trailing newlines, so a file holding nothing but whitespace is non-empty here and
  # an EMPTY payload there — the payload block is then suppressed entirely and the child is
  # launched with no task at all, which is the very improvise defect the refusal above exists to
  # stop. Both ends must use the same predicate: content, not bytes.
  brief_body="$(cat "$BRIEF_FILE")"
  [ -n "${brief_body//[[:space:]]/}" ] || die "--brief-file '$BRIEF_FILE' holds only whitespace — it is empty as far as the child is concerned, and a child launched with no task does not error, it improvises"
fi
# Gate 1 — a FEATURE-SCOPED worker must run INSIDE the worktree it serves. Keyed on the agent's
# DECLARED scope (change 0208) rather than on a `build-*` name shape: `rebase-resolver`,
# `integration-repair` and the three `review-*` rungs are equally feature-scoped and match no build
# shape, and two of them commit. It is a RUNTIME requirement (the path is runtime data), so
# sync-agents.sh's generation-time slot cannot substitute for it. Loud, matching the facade's posture
# for an unknown --runner rather than its tolerant posture for a runners.<name>: value: that
# tolerance exists so a cosmetic config typo cannot fail a live dispatch, whereas this is a request
# the facade cannot serve correctly.
if [ "$AGENT_SCOPE" = "feature" ]; then
  [ -n "$WORKTREE" ] || die "--worktree is required for feature-scoped agents (agent '$AGENT' declares worktree-scope: feature — it must run in its feature worktree, not the main tree)"
fi
# The empty-payload refusal stays keyed on `build-*` and is NOT widened to the feature-scoped set
# (change 0277, scope confirmed at 0208's reconcile). Its reason is build-specific: a build worker
# with no task at all does not error, it invents work from whatever it can see in the worktree, and
# the dispatch still looks successful. The other feature-scoped agents legitimately dispatch
# payload-free, so widening this would refuse correct dispatches.
case "$AGENT" in
  build-*)
    # EXEMPT: `--observe`, which starts no child at all — it reads a result the matching
    # `--launch` already recorded. A payload there would have nothing to carry it to, and the
    # generated shim's observe line deliberately has no brief slot, so requiring one would refuse
    # every second half of the launch/observe pair. The gate stays pre-verb for the two verbs that
    # DO start a child: `--launch` and the legacy synchronous call.
    # CONTENT, not arity: `[ $# -gt 0 ]` counts arguments, so `-- ""` satisfied it while the
    # adapter's payload came out empty and the task context vanished. The argv channel is measured
    # by the same whitespace-stripped predicate the brief-file channel is measured by above (a
    # brief file that reached this line has already been proven to carry content).
    argv_body="$*"
    [ "$VERB" = "observe" ] || [ -n "$BRIEF_FILE" ] || [ -n "${argv_body//[[:space:]]/}" ] || die "a build-* dispatch carries no task: pass the brief with --brief-file <path> (preferred) or after '--'. A build worker launched with no task does not error — it improvises from whatever it finds in the worktree and the dispatch still looks successful" ;;
esac
# The path actually handed to the adapter. It is the caller's file on the legacy synchronous verb
# (nothing detaches, so there is no temp-file lifetime hazard); `--launch` reassigns it to the
# durable spooled copy in the dispatch dir.
BRIEF_PATH="$BRIEF_FILE"
ANCHOR="$(docket_anchor_path "${WORKTREE:-.}")"
# DURABILITY, for `--observe` ONLY: the dispatch dir lives under the git COMMON dir precisely so a
# result outlives `git worktree remove` (lib/docket-dispatch-dir.sh: "a dispatch result must outlive
# `git worktree remove`"). Gate 2 below would nevertheless refuse an observation whose anchor
# worktree has since been removed, so the recorded result would be readable by a human with a shell
# and unreadable by the facade's own reader — durable in storage and not in service.
# The root is REPO-WIDE, not worktree-scoped, so the main worktree resolves the identical path.
# Scoped tightly, deliberately: only the observe verb, and only when the anchor does not EXIST.
# A path that exists but belongs to another repository still hits gate 3 unchanged, so this widens
# nothing — a removed worktree is not a foreign one, and `--launch` keeps both gates in full.
ANCHOR_FALLBACK=0
if [ "$VERB" = "observe" ] && [ ! -d "$ANCHOR" ]; then
  printf 'runner-dispatch: observe — the anchor %s no longer exists (its worktree was removed); reading the dispatch from the repository-wide root under the main worktree %s\n' \
    "$ANCHOR" "$REPO_ROOT" >&2
  ANCHOR="$REPO_ROOT"
  # Remembered, because the RECORDED result is now readable but the child's WORK is not: the tree a
  # `build-*` verdict must inspect is exactly the one that was removed. The build leg reads this and
  # refuses to substitute the main worktree for it — measuring a different tree would manufacture a
  # verdict, which is the one thing every git-read disposition on this seam declines to do.
  ANCHOR_FALLBACK=1
fi
# Gate 2 — the resolved anchor must exist as a directory.
[ -d "$ANCHOR" ] || die "--worktree $ANCHOR is not a directory"
# Gate 3 — MEMBERSHIP, not containment. The pre-0208 test asked docket_main_worktree "$ANCHOR",
# which answers "is this path INSIDE some worktree of this repo" — true for the main worktree
# itself and for every ordinary subdirectory of it. So the one value the gate most needs to reject,
# the repo root handed to a build worker, cleared it while the diagnostic asserted a membership
# nothing had checked.
#
# One `worktree list --porcelain` capture from $ANCHOR yields BOTH facts:
#   * same repo — the FIRST `worktree` line equals $REPO_ROOT. git lists the main worktree first,
#     the exact property docket_main_worktree already rests on. NEVER an anywhere-in-list match:
#     `worktree list` retains stale records for deleted-and-recreated directories, so a FOREIGN
#     repo's list can carry a `worktree $REPO_ROOT` line for a path that is no longer its worktree,
#     and an anywhere-match would hand a delegated run a tree docket does not own — regressing the
#     very guarantee this gate provides.
#   * membership — an exact `worktree $ANCHOR` line, i.e. a worktree TOP-LEVEL rather than merely a
#     path contained in one.
# A non-repo path yields empty output and fails the first-line comparison, so the not-a-repo case
# still falls out of this same check, as it did before.
# CAPTURED INTO A VARIABLE, never piped into `grep -q`: under `pipefail` grep's early exit races
# git's SIGPIPE status (AGENTS.md, "Shell").
# `pwd -P` runs FIRST and is load-bearing on macOS: `mktemp -d` and user-supplied /tmp paths are
# symlinked (/tmp -> /private/tmp) while git prints physical paths, so without it this exact-line
# match would falsely reject valid worktrees the old containment check accepted. It runs after the
# -d gate above, so the `cd` cannot fail. $REPO_ROOT needs no normalization — it IS git's output.
ANCHOR="$(cd "$ANCHOR" && pwd -P)"
wt_list="$("$GIT" -C "$ANCHOR" worktree list --porcelain 2>/dev/null)"
[ "$(sed -n '1s/^worktree //p' <<<"$wt_list")" = "$REPO_ROOT" ] \
  || die "--worktree $ANCHOR is not a worktree of this repository"
grep -qxF -- "worktree $ANCHOR" <<<"$wt_list" \
  || die "--worktree $ANCHOR is not a worktree of this repository (it is inside one, but a run anchor must be a worktree top-level)"
# Gate 3b — and for a FEATURE-SCOPED agent it must not be the MAIN worktree. Membership alone still
# admits `$REPO_ROOT`, which is the precise wrong value this whole gate exists to reject: the primary
# checkout, sitting on the integration branch, handed to a worker that commits.
# EXEMPT when the observe anchor fallback fired. `--observe` on a dispatch whose worktree has since
# been removed deliberately reassigns ANCHOR to $REPO_ROOT so the durable record stays readable, and
# the build leg then reports `task-unverifiable worktree-removed`. Refusing here would turn that
# honest non-verdict into a failed observation — the record would be durable in storage and not in
# service, the exact failure ANCHOR_FALLBACK was added to prevent.
#
# THE DIAGNOSTIC STATES WHAT WAS MEASURED, and nothing else. What this predicate proves is PATH
# IDENTITY — the anchor IS $REPO_ROOT — so the message says that, and says which tree the agent
# must run in instead. It deliberately no longer asserts that the tree sits "on the integration
# branch": nothing here reads a branch, and a diagnostic claiming an unchecked fact is exactly the
# shape gate 3's own comment above condemns in the pre-0208 code. The branch is the HAZARD the
# main worktree normally carries, not a fact this gate establishes — see runner-dispatch.md's gate
# description for the residual that leaves (a linked worktree that happens to be on the integration
# branch is not caught) and why a branch predicate is not available: `rebase-resolver` runs
# mid-rebase on a detached HEAD, so there is no branch to compare.
if [ "$AGENT_SCOPE" = "feature" ] && [ "$ANCHOR_FALLBACK" != 1 ]; then
  [ "$ANCHOR" != "$REPO_ROOT" ] \
    || die "--worktree $ANCHOR is the main worktree; a feature-scoped agent must run in a linked feature worktree, never the primary checkout"
fi
export DOCKET_REPO_ROOT="$ANCHOR"

# --- runners.<name>: config, per-key across layers (local > committed > global) -----
# Same nested-section awk shape as sync-agents.sh's section_body (kept self-contained
# here; sync-agents.sh has the twin — tracked divergence, see LEARNINGS on twins).
yaml_section(){  # $1=key ; reads stdin -> the dedented body under <key>:, '' when absent
  awk -v key="$1" '
    function ind(s,   m){ m=match(s, /[^[:space:]]/); return (m==0 ? length(s) : m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    !inb { if (nc ~ ("^[[:space:]]*" key "[[:space:]]*:[[:space:]]*$")) { inb=1; kin=ind(nc) } next }
    nc ~ /[^[:space:]]/ && ind(nc) <= kin { exit }
    { if (!haveBase && nc ~ /[^[:space:]]/) { base=ind($0); haveBase=1 }
      if (haveBase) print substr($0, base+1); else print }
  '
}
runner_block(){  # $1=file -> the dedented body under runners.<RUNNER>., '' when absent
  [ -f "$1" ] || return 0
  yaml_section runners < "$1" | yaml_section "$RUNNER"
}

GLOBAL_CFG="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml"
seen_keys=" "
for f in "$REPO_ROOT/.docket.local.yml" "$REPO_ROOT/.docket.yml" "$GLOBAL_CFG"; do
  blk="$(runner_block "$f")"
  [ -n "$blk" ] || continue
  while IFS= read -r line; do
    k="$(sed -nE 's/^[[:space:]]*([A-Za-z0-9._-]+)[[:space:]]*:.*/\1/p' <<<"$line")"
    [ -n "$k" ] || continue
    case "$seen_keys" in *" $k "*) continue ;; esac   # first (highest-precedence) layer wins per key
    seen_keys="$seen_keys$k "                          # claim the key for THIS layer before parsing its
                                                       # value, so a malformed high-precedence value still
                                                       # masks lower layers (precedence is per-key, not per-value)
    # Value class (change 0173): rest of the line, trailing `# comment` stripped, whitespace
    # trimmed. This is a BLOCK mapping (one key per line), not sync-agents.sh's flow map, so the
    # flow-map class would be wrong here — it would admit a slash-bearing path only by luck. The
    # pre-0173 class ([A-Za-z0-9._-]+) truncated `https://host/v1` to `https` and dropped a
    # leading-slash path entirely. Comment detection requires leading whitespace, per YAML, so a
    # value containing `#` (a URL fragment) survives. A COMMENT-ONLY value needs its own leg: the
    # capture's `[[:space:]]*` is greedy and eats the space before the `#`, so the whitespace-
    # preceded strip below can never fire on it — without this the comment TEXT would be exported
    # as the value, which is worse than the truncation this change removes (change 0173 review).
    # A YAML plain scalar can never begin with `#`, so stripping a leading-`#` value is safe.
    # Posture stays TOLERANT — an unparseable value `continue`s rather than dying. This path runs
    # mid-handoff to a child process, where dying converts a cosmetic config typo into a failed
    # dispatch. sync-agents.sh is loud instead; the asymmetry is deliberate (change 0173).
    v="$(sed -nE 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*(.*)$/\1/p' <<<"$line")"
    v="${v%%$'\n'*}"
    v="$(sed -E -e 's/^#.*$//' -e 's/[[:space:]]+#.*$//' -e 's/[[:space:]]+$//' <<<"$v")"
    [ -n "$v" ] || continue
    uk="$(tr '[:lower:]' '[:upper:]' <<<"$k" | tr '.-' '__')"
    export "DOCKET_RUNNER_CFG_$uk=$v"
  done <<<"$blk"
done

# --- handoff: foreground, adapter owns everything child-specific --------------------
args=( --agent "$AGENT" )
[ -n "$MODEL" ]  && args+=( --model "$MODEL" )
[ -n "$EFFORT" ] && args+=( --effort "$EFFORT" )

# --- run-gate primitives, shared by all three verbs ---------------------------------
# Defined HERE, above every verb, because the same three reads serve all of them: the synchronous
# gate at the bottom of this file brackets its handoff with them, `--launch` captures the
# ATTRIBUTION inputs with them, and `--observe` re-reads with them. ONE reader — verify-run stays
# the single owner of frontmatter reading for the gate (its own header says so), and a second
# reader growing beside this one is exactly what that ownership exists to prevent.
in_progress_ids(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids 2>/dev/null; }
in_progress_claims(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids --with-claimed-at 2>/dev/null; }
# Best-effort metadata re-sync, used on BOTH sides of every hand-off — the synchronous gate's, and
# the launch/observe pair's. Both snapshots must be reads of FRESH ORIGIN state
# (LEARNINGS: cas-re-read-fresh-origin); an asymmetric pair attributes an ABANDONED claim from an
# earlier session to this run. A failure degrades the gate's freshness; it never fails a dispatch.
resync_metadata(){
  "$DOCKET_BASH_PATH" "$DOCKET_FACADE" preflight >/dev/null 2>&1 \
    || { printf 'runner-dispatch: run gate — metadata re-sync failed; verifying against local state\n' >&2; return 1; }
}

# --- verb: --launch (change 0271) ---------------------------------------------------
# Detach the adapter so the delegated run can OUTLIVE this call, then return at once with the
# dispatch key. The posture and its six required capabilities are cited, never restated:
# skills/docket-build/references/gate-execution.md, plus the adapter-launch verdicts in
# skills/docket-build/references/delegation-execution.md.
#
# This sits AFTER the anchor gates and the runners.<name>: resolution so `--launch` inherits every
# gate and every exported config value the legacy path already gets — a launch is the same
# validated request, differing only in how the adapter is started.
if [ "$VERB" = "launch" ]; then
  DROOT="$(docket_dispatch_root "$ANCHOR")" || die "cannot resolve the dispatch root for $ANCHOR"
  mkdir -p "$DROOT" || die "cannot create the dispatch root $DROOT"
  # RETENTION, applied at the one moment a new dir is about to be minted, so the growth and the
  # bound share a seam. It only ever removes a dispatch that is TERMINAL and whose terminal file is
  # older than the retention window — never a live one, and never one a caller could still be
  # observing inside that window (lib/docket-dispatch-dir.sh states the guarantee in full).
  # Best-effort by construction: a prune that cannot run must never fail the dispatch it precedes.
  docket_dispatch_prune "$DROOT"
  KEY="$(docket_dispatch_mint "$DROOT" "$AGENT")" || die "cannot mint a dispatch key under $DROOT"
  DDIR="$DROOT/$KEY"
  # THE BRIEF, SPOOLED (change 0277). Written atomically — a temp file BESIDE its destination then
  # `mv -f`, the same shape as `launch`, `done`, and `gate-before` — so a reader never sees a
  # half-written brief. Two things are bought: the detached child no longer depends on the
  # caller's temp file outliving this call, and the dispatch record gains its INPUT alongside its
  # output. A SUCCESSFUL spool introduces no new lifecycle: the brief sits inside the dispatch dir
  # that `docket_dispatch_prune` already bounds, and is reclaimed with it.
  # A spool that cannot be written is a hard failure, not a degrade: the brief is the child's only
  # input, so dispatching without it is the improvise defect. It is also the FIRST `die` reachable
  # after the mint, and the prune deliberately never removes a dispatch with no terminal file
  # (lib/docket-dispatch-dir.sh: "a dispatch that never went terminal is retained forever") — so
  # this path removes the dir it just minted rather than leaving one no prune can ever reclaim.
  # `rm -rf` is safe precisely here and nowhere else: `docket_dispatch_mint` refuses to reuse an
  # existing key, so this dir was created microseconds ago by this call, holds nothing but the
  # failed spool, and no child has been started against it yet.
  if [ -n "$BRIEF_FILE" ]; then
    cat "$BRIEF_FILE" > "$DDIR/brief.partial" || { rm -rf "$DDIR"; die "cannot spool the brief into $DDIR"; }
    mv -f "$DDIR/brief.partial" "$DDIR/brief" || { rm -rf "$DDIR"; die "cannot spool the brief into $DDIR"; }
    BRIEF_PATH="$DDIR/brief"
  fi
  STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  # The dispatch-time SHA: the direct analogue of DISPATCH_EPOCH, captured BEFORE the child can
  # commit anything, so a commit landing in the gap is excluded either way. Empty on a repo with
  # no commits — `verify-run --build` refuses an empty `--since` outright, so the observe leg
  # gets NO verdict at all and reports the run failed rather than guessing it succeeded.
  # (`unknown-since-sha` is the verdict for a non-empty sha git cannot resolve, not for this.)
  SINCE_SHA="$("$GIT" -C "$ANCHOR" rev-parse HEAD 2>/dev/null || true)"
  # The dispatch-time BRANCH, captured for the same reason and at the same instant: the build
  # verdict's `branch` conjunct asks whether the child ENDED where it was sent, which is only
  # answerable against a value recorded BEFORE the child could move HEAD. Read back at observation
  # time it compares the anchor's HEAD to itself and can never be unmet.
  # A DETACHED anchor records NOTHING rather than the literal `HEAD` that `--abbrev-ref` prints:
  # `HEAD` is not a branch name, and recording it would make the conjunct hold for any other
  # detached state. Absent is the honest answer, and `--observe` reports it as unverifiable.
  SINCE_BRANCH="$("$GIT" -C "$ANCHOR" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  [ "$SINCE_BRANCH" != "HEAD" ] || SINCE_BRANCH=""

  # THE RUN GATE'S ATTRIBUTION INPUTS, captured here because they are only knowable BEFORE the child
  # runs. Change 0237's gate identifies a claimant by diffing the in-progress set across the
  # hand-off; under detachment the two halves of that diff land in two different processes, so the
  # "before" half is recorded durably for `--observe` to read.
  #
  # Scoped to implement-next for exactly the reason the synchronous fence is: a build-* delegation
  # leaves its change in-progress BY DESIGN, and a build task's terminal state is a commit, read on
  # the observe seam by the build verdict family instead.
  #
  # The discipline is the synchronous gate's, unchanged: re-sync FIRST so the before-read is of
  # fresh origin state, then stamp the clock AFTER that read, so a claim landing in the gap is
  # either already in the before-set or stamped before the window and is excluded either way. The
  # observe half re-syncs again, which is what keeps the pair SYMMETRIC — an asymmetric pair
  # attributes an abandoned claim from an earlier session to this run
  # (LEARNINGS: cas-re-read-fresh-origin).
  #
  # Tolerant, like every other gate read: a failed snapshot or clock read leaves the gate UNARMED
  # and the dispatch untouched, never failed. `dispatch_epoch` in the launch record plus the
  # `gate-before` file ARE the arming signal — with either absent, `--observe` falls back to the
  # sentinel-only disposition rather than guessing at an attribution.
  GATE_BEFORE=""; GATE_EPOCH=""; GATE_ARMED=0
  if [ "$AGENT" = "implement-next" ]; then
    resync_metadata || :
    if GATE_BEFORE="$(in_progress_ids)"; then
      GATE_EPOCH="$(date -u +%s 2>/dev/null)"
      case "$GATE_EPOCH" in
        ''|*[!0-9]*) printf 'runner-dispatch: run gate disabled — could not read the clock\n' >&2 ;;
        *) GATE_ARMED=1 ;;
      esac
    else
      printf 'runner-dispatch: run gate disabled — could not read the in-progress set\n' >&2
    fi
  fi

  # DETACHMENT, measured (2026-08-09): `set -m` makes a background job a PROCESS-GROUP LEADER,
  # so it survives the harness's teardown of THIS call's process group — capability 1's stronger
  # reading. Measured with one variable changed between two arms of a single run: the set -m
  # child survived a group-directed TERM, the non-set-m child did not. `setsid` is ABSENT on
  # macOS, which is why job control rather than a new session is the mechanism.
  # Every stream is redirected to the durable dir and stdin is closed, so nothing remains
  # attached to the initiating call (capability 2).
  # THE WRAPPER WRITES THE SENTINEL, NEVER THE AGENT: "done" must not be a claim by the party
  # being judged. The write is atomic — a temp file BESIDE its destination (the one licensed
  # exception to templating temp files into TMPDIR, because the rename must be same-filesystem)
  # then `mv -f`; a reader therefore never sees a half-written sentinel.
  # `pid` is `$BASHPID` — THIS backgrounded subshell's own pid, the same value `launch` records
  # as `child_pid`. `$$` would be the facade's pid, which names a long-exited process by the time
  # a human reads the sentinel.
  set -m
  {
    # ONE channel, always — the same single-channel handoff the synchronous verb composes, here
    # over the SPOOLED copy: with a brief file the argv channel is empty by construction (the
    # pre-verb gate refused any trailing argument), so `--` is passed with nothing after it.
    if [ -n "$BRIEF_PATH" ]; then
      "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" --brief-file "$BRIEF_PATH" --
    else
      "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
    fi
    ec=$?
    printf 'exit_code=%s\nstarted_at=%s\nfinished_at=%s\npid=%s\ndispatch_key=%s\n' \
      "$ec" "$STARTED_AT" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$BASHPID" "$KEY" > "$DDIR/done.partial"
    mv -f "$DDIR/done.partial" "$DDIR/done"
  } >"$DDIR/stdout.log" 2>"$DDIR/stderr.log" </dev/null &
  CHILD_PID=$!
  set +m

  # RACE PRECONDITION (0223's measured one): the new group must be fully established before this
  # call returns, or the harness's teardown wins the race. Poll briefly and FAIL CLOSED rather
  # than return a key for a child that is still in our group and about to be reaped with us.
  MY_PGID="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  CHILD_PGID=""
  for _ in $(seq 1 50); do
    CHILD_PGID="$(ps -o pgid= -p "$CHILD_PID" 2>/dev/null | tr -d ' ')"
    # The child may already have finished — that is establishment too, and the sentinel proves it.
    [ -f "$DDIR/done" ] && break
    [ -n "$CHILD_PGID" ] && [ "$CHILD_PGID" != "$MY_PGID" ] && break
    sleep 0.1
  done
  if [ -z "$CHILD_PGID" ] && [ ! -f "$DDIR/done" ]; then
    die "launch failed — the detached child never appeared (key $KEY)"
  fi
  if [ -n "$CHILD_PGID" ] && [ "$CHILD_PGID" = "$MY_PGID" ] && [ ! -f "$DDIR/done" ]; then
    kill -TERM "$CHILD_PID" 2>/dev/null
    die "launch failed — the child did not separate into its own process group; refusing to report a dispatch that a teardown would kill (key $KEY)"
  fi

  # THE IDENTITY TOKEN for the observer's kill guard, captured from the same live process the group
  # was just measured on. `pgid` and `child_pid` are reusable names, so the observer needs one value
  # that a RECYCLED pid cannot reproduce: a pid that the OS later hands to an unrelated process
  # leading a group of the same id matches both of those and only differs here. Empty when the child
  # already finished (the `ps` above saw nothing) — and the observer then refuses to signal, because
  # an absent token is not a match (change 0284). That refusal costs nothing on any reachable input:
  # the only child whose token is empty here is one that had ALREADY finished, so the wrapper wrote
  # its `done` sentinel and the observer's sentinel read disposes long before any kill guard is asked.
  CHILD_LSTART="$(docket_identity_of "$CHILD_PID")"

  # `pgid` is what an observer must signal to reach the whole detached tree, so it is recorded from
  # the MEASUREMENT wherever the measurement exists. The fallback fires only when `ps` could not see
  # the process at all — which, given the refusals above, means the child already finished — and it
  # is sound rather than a guess: under `set -m` a background job is created as a process-group
  # LEADER, so its pgid IS its pid by construction. Note the corollary a reader must not miss: with
  # `set -m` removed the fallback would record a pid that is no group's leader, which is exactly why
  # tests/test_runner_dispatch_detach.sh reads the group from the live process rather than from this
  # record ("the child is in its OWN process group, not the test's").
  # The before-snapshot lands BEFORE the record that advertises it, so an observer that sees a
  # readable `dispatch_epoch` can rely on the file being there. It is written even when EMPTY —
  # "nothing was claimed at the handoff" is a real answer, and the arming signal is the record's
  # epoch, never this file's size. `mv -f` for the reason it is used everywhere else here: BSD `mv`
  # onto an unwritable destination with a tty prompts, self-answers `n`, and exits 0.
  if [ "$GATE_ARMED" = 1 ]; then
    printf '%s\n' "$GATE_BEFORE" > "$DDIR/gate-before.partial"
    mv -f "$DDIR/gate-before.partial" "$DDIR/gate-before" || {
      printf 'runner-dispatch: run gate disabled — could not record the before-snapshot in %s\n' "$DDIR" >&2
      GATE_EPOCH=""
    }
  fi
  printf 'pgid=%s\nchild_pid=%s\nchild_lstart=%s\nstarted_at=%s\nagent=%s\nrunner=%s\nworktree=%s\nsince_sha=%s\nbranch=%s\ndispatch_epoch=%s\n' \
    "${CHILD_PGID:-$CHILD_PID}" "$CHILD_PID" "${CHILD_LSTART:-}" "$STARTED_AT" "$AGENT" "$RUNNER" \
    "$ANCHOR" "${SINCE_SHA:-}" "${SINCE_BRANCH:-}" "${GATE_EPOCH:-}" \
    > "$DDIR/launch.partial"
  mv -f "$DDIR/launch.partial" "$DDIR/launch"

  printf '%s\n' "$KEY"
  exit 0
fi

# Read one field from a dispatch record. Deliberately NOT `sed … | sed -n 1p`: a producer piped
# into a consumer that may exit early takes SIGPIPE under `pipefail` (AGENTS.md, "Shell"), so the
# first-line trim is a parameter expansion on a captured value instead.
launch_field(){  # $1 = dispatch dir, $2 = field -> first value, empty when absent or unreadable
  local raw
  raw="$(sed -n "s/^$2=//p" "$1/launch" 2>/dev/null)"
  printf '%s' "${raw%%$'\n'*}"
}

# Test-only synchronization point, the same shape as gate-run.sh's `barrier` (change 0284). A
# TWO-WAY RENDEZVOUS: it announces its arrival (`<file>.reached`) so a fixture knows the process is
# held at exactly this point and nowhere else, then waits to be let go (`<file>.release`). That
# handshake is what makes an INTERLEAVING deterministic instead of a sleep-tuned guess — and the
# observe verb's step-1/step-3 window cannot be entered any other way.
#
# ENV-GATED AND INERT BY DEFAULT, and it is the POINT variable that arms it: with
# RUNNER_DISPATCH_TEST_BARRIER unset this is a no-op at full speed no matter what else is in the
# environment, so the hook can never itself become a hang site in production. The match is on the
# point NAME, so arming one rendezvous cannot silently hold every other call site as well.
#
# BOUNDED even when armed: a fixture that forgets to release must fail its own bounded wait and
# leave a red assert behind, never hang the suite.
barrier(){  # $1 = the point this call site names
  [ "${RUNNER_DISPATCH_TEST_BARRIER:-}" = "$1" ] || return 0
  local f="${RUNNER_DISPATCH_TEST_BARRIER_FILE:?barrier point '$1' armed without RUNNER_DISPATCH_TEST_BARRIER_FILE}"
  : >"$f.reached"
  local waited=0
  while [ ! -e "$f.release" ] && [ "$waited" -lt 300 ]; do
    sleep 0.1; waited=$(( waited + 1 ))
  done
  return 0
}

# --- verb: --observe (change 0271) --------------------------------------------------
# ONE short, idempotent look. Never a long foreground call — that ceiling is the defect this
# change removes, so re-introducing it here would defeat the whole design.
#
# LIVENESS comes from the sentinel, and from nothing else; CORRECTNESS comes from git. A sentinel
# claiming success with no matching git evidence is a FAILURE — correctness wins, so the child's
# own exit code is never the last word about the work. Keeping the two sources apart is what lets
# the facade observe a LIVE child without ever reading liveness out of git state.
if [ "$VERB" = "observe" ]; then
  [ -n "$OBSERVE_KEY" ] || die "--observe requires a dispatch key"
  # The key becomes a path component, exactly as `--runner` does above, so it earns the same
  # shape-keyed refusal. `..` is the case that matters: it names a directory that EXISTS, so
  # without this gate a typo is observed as "still running" forever — a verdict manufactured out
  # of nothing rather than read off a dispatch.
  case "$OBSERVE_KEY" in
    *[!A-Za-z0-9._-]*|*..*) die "invalid dispatch key '$OBSERVE_KEY'" ;;
  esac
  DROOT="$(docket_dispatch_root "$ANCHOR")" || die "cannot resolve the dispatch root for $ANCHOR"
  DDIR="$(docket_dispatch_dir "$DROOT" "$OBSERVE_KEY")" \
    || die "unknown dispatch key '$OBSERVE_KEY' (no result directory under $DROOT)"

  # The budget, in minutes. The caller's environment wins — that is how a shim hands one down —
  # and otherwise it is resolved once, HERE: no other verb needs it, and resolving config a verb
  # does not use is how a config failure becomes a dispatch failure.
  if [ -z "${DELEGATION_OBSERVATION_BUDGET:-}" ]; then
    _cfg="$("$DOCKET_BASH_PATH" "$SELF_DIR/docket-config.sh" --export 2>/dev/null)" \
      && eval "$(grep -E '^DELEGATION_OBSERVATION_BUDGET=' <<<"$_cfg")"
    DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-60}"
  fi
  # An unparseable budget is NOT a spent one. `[ N -lt abc ]` is a shell ERROR, whose non-zero
  # status is indistinguishable from "budget exceeded" at the `if` below — so a typo in an
  # environment variable would kill a healthy child. Normalize to the shipped default instead;
  # docket-config.sh already refuses a malformed configured value at its own boundary.
  case "$DELEGATION_OBSERVATION_BUDGET" in
    ''|*[!0-9]*) DELEGATION_OBSERVATION_BUDGET=60 ;;
  esac

  LPGID="$(launch_field "$DDIR" pgid)"
  LSTART="$(launch_field "$DDIR" started_at)"

  # THE RELAY. `--launch` redirects the adapter's stdout into `$DDIR/stdout.log`, so this function
  # is the ONLY channel by which a delegated agent's result reaches its caller: the generated shim
  # is told to "Relay that observe call's stdout as your result" (sync-agents.sh's emit_shim), an
  # instruction satisfiable only if `--observe` puts that captured stdout on ITS OWN stdout, so the
  # two are one contract and move together. The adapters call the child's stdout the relay
  # (codex.sh `cat`s the last message; opencode.sh and cursor.sh pass the formatted stdout through),
  # so it is emitted VERBATIM — never summarized, prefixed, or reformatted; a caller parses it.
  # Every diagnostic in this branch stays on stderr, which is what keeps the two from interleaving.
  #
  # Fired ONLY where the `done` sentinel exists — the child is finished, so its stdout is COMPLETE.
  # The still-running (4) path deliberately emits nothing: the shim observes repeatedly, and a
  # partial relay per pass would hand the caller the same prefix over and over. The budget-kill and
  # own-group-refusal paths emit nothing either — there the run has no result to relay (and in the
  # latter case it is still live and still writing).
  relay_child_stdout(){
    if [ -s "$DDIR/stdout.log" ]; then cat "$DDIR/stdout.log"; fi
    return 0
  }

  # THE RUN GATE ON THE OBSERVE SEAM — change 0237's disposition, reached through detachment.
  # The generated shim always launches, so the synchronous fence at the bottom of this file is
  # unreachable for every delegated implement-next run; without this leg such a run gets the
  # SENTINEL-ONLY disposition, and one that HALTED or stopped before its PR exits 0 at the adapter
  # and observes as "complete (child exited 0)". That is the prose-level failure change 0237 was
  # built to eliminate, so the disposition follows the run onto whichever seam it actually returns
  # through.
  #
  # It mirrors the synchronous gate's ATTRIBUTION exactly — the same symmetric fresh-origin
  # re-sync, the same three filters, the same more-than-one-candidate stand-down, the same posture
  # of acting only on a POSITIVE finding — and differs in exactly one respect, deliberately:
  #
  #   NO AUTO RE-DISPATCH. The synchronous gate's ONE bounded re-dispatch is not recreated here.
  #   Re-launching a detached child out of an OBSERVATION is a different lifecycle: an observation
  #   is a short, idempotent read that a shim makes repeatedly, so a re-dispatch on this path would
  #   race the very run being observed and could mint a fresh detached child per pass. A
  #   `run-incomplete` therefore reports 1 and the caller acts. This is a decision, not an omission
  #   (scripts/runner-dispatch.md records it as one).
  #
  # The verdict outranks the child's own exit code in BOTH directions, which is why this runs
  # before the exit_code split below rather than inside its zero leg: a halt is terminal whatever
  # the adapter returned, and a positively green verdict describes a run that reached its PR
  # despite a noisy adapter. Same rule as the disagreement rule — correctness outranks the
  # self-report of the party being judged.
  #
  # EXITS on a positive finding; RETURNS (falling through to the sentinel-only disposition) on
  # anything it could not establish.
  observe_implement_next(){
    local before after epoch verdict nid nclaimed
    local new_ids=()
    epoch="$(launch_field "$DDIR" dispatch_epoch)"
    # No epoch = the gate was never armed at launch (an older dispatch, a non-implement-next one,
    # or a snapshot/clock read that failed there and already said so). Silent: the launch side
    # announced it, and a second warning per observation pass would be noise in a polling loop.
    case "$epoch" in ''|*[!0-9]*) return 0 ;; esac
    # The before-snapshot and the epoch are written TOGETHER at launch, so a readable epoch with no
    # snapshot file is a half-written record: unarmed, never a before-set guessed as empty.
    [ -f "$DDIR/gate-before" ] || {
      printf 'runner-dispatch: observe %s — run gate disabled: the launch recorded no before-snapshot\n' "$OBSERVE_KEY" >&2
      return 0
    }
    before="$(cat "$DDIR/gate-before" 2>/dev/null)"
    # The AFTER read must come from FRESH ORIGIN state, symmetric with the launch-side read.
    resync_metadata || :
    after="$(in_progress_claims)" || {
      printf 'runner-dispatch: observe %s — run gate disabled: could not re-read the in-progress set\n' "$OBSERVE_KEY" >&2
      return 0
    }
    while IFS=' ' read -r nid nclaimed; do
      [ -n "$nid" ] || continue
      grep -qxF "$nid" <<<"$before" && continue
      case "${nclaimed:-}" in
        ''|*[!0-9]*)
          printf 'runner-dispatch: observe %s — run gate: ignoring change %s: no readable claimed_at\n' "$OBSERVE_KEY" "$nid" >&2
          continue ;;
      esac
      [ "$nclaimed" -ge "$epoch" ] || {
        printf 'runner-dispatch: observe %s — run gate: ignoring change %s: claimed before this dispatch started\n' "$OBSERVE_KEY" "$nid" >&2
        continue
      }
      new_ids+=("$nid")
    done <<<"$after"

    if [ "${#new_ids[@]}" -gt 1 ]; then
      printf 'runner-dispatch: observe %s — run gate disabled: %s changes were claimed during this dispatch (%s); this run claims at most one, so none can be attributed to it\n' \
        "$OBSERVE_KEY" "${#new_ids[@]}" "${new_ids[*]}" >&2
      return 0
    fi
    # No candidate at all: drained, a lost claim race, or a run that finished and left its change
    # `implemented` (which is not in-progress and so never appears in the after-set). All three are
    # no-ops here, exactly as the empty diff is on the synchronous path.
    [ "${#new_ids[@]}" -eq 1 ] || return 0
    nid="${new_ids[0]}"
    verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
    printf 'runner-dispatch: observe %s — run gate: %s\n' "$OBSERVE_KEY" "${verdict:-run-unverifiable $nid}" >&2
    case "$verdict" in
      run-halted*)
        # STOP + SURFACE with its own code, the same 3 the synchronous gate returns and the code
        # the design spec's synthesized-exit table pins normatively under detachment. Never 0: a
        # driver told 0 draws the next change, on a disposition that means a human is needed.
        printf 'runner-dispatch: observe %s — RUN HALTED (%s); the delegated implement-next run stopped and needs a human — read the change file'"'"'s "## Run halted" section\n' \
          "$OBSERVE_KEY" "$verdict" >&2
        relay_child_stdout
        exit 3 ;;
      run-incomplete*)
        printf 'runner-dispatch: observe %s — FAILED (%s); the delegated implement-next run did not reach its PR. No re-dispatch is made from an observation — the change stays in-progress with its claim intact, and board-checks'"'"' aborted-run leg remains the backstop\n' \
          "$OBSERVE_KEY" "$verdict" >&2
        relay_child_stdout
        exit 1 ;;
      run-complete*|run-unclaimed*)
        printf 'runner-dispatch: observe %s — complete (%s)\n' "$OBSERVE_KEY" "$verdict" >&2
        relay_child_stdout
        exit 0 ;;
    esac
    # Empty or unparseable: no positive finding, so the sentinel-only disposition stands.
    return 0
  }

  # THE SENTINEL DISPOSITION, in one place. A sentinel means the child is DONE, and every reader of
  # that fact must reach the same verdict, so this is a FUNCTION rather than a straight-line block:
  # besides the ordinary read below, `terminate_dispatch` re-reads the sentinel on both sides of its
  # kill and lands here when one appeared, and a second copy of the disposition would be the pair
  # that drifts. Well-formed vs malformed is the difference between a clean adapter exit and a
  # wrapper crash — the latter is `unavailable`, NEVER an exit code read out of garbage.
  # NEVER RETURNS: every leg exits, which is what makes it safe to call from inside the give-up path.
  report_done_disposition(){
    SEC="$(docket_sentinel_field "$DDIR" exit_code)"
    case "$SEC" in
      ''|*[!0-9]*)
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the sentinel is malformed; the launcher did not finish cleanly)\n' "$OBSERVE_KEY" >&2
        # The child is finished either way, so whatever it managed to say is the only evidence left;
        # relaying it costs nothing and is the difference between a diagnosable failure and a silent
        # one. The verdict still comes from the exit code, never from the relayed text.
        relay_child_stdout
        exit 1 ;;
    esac
    # The child is finished and its sentinel parses, which is the precondition for every git-read
    # disposition on this seam. implement-next's runs FIRST and for both exit-code classes — see
    # observe_implement_next's header for why the verdict outranks the child's own code in both
    # directions. It exits on a positive finding and returns otherwise, leaving the sentinel-only
    # disposition below in charge.
    case "$AGENT" in
      implement-next) observe_implement_next ;;
    esac
    if [ "$SEC" = "0" ]; then
      # LIVENESS said done; now CORRECTNESS decides. A sentinel claiming success with no matching
      # git evidence is a FAILURE — the delegated run is the party being judged, so its own exit
      # code can never be the last word. Change 0258's stranded +64 lines exited 0 at the adapter.
      GITV=""
      case "$AGENT" in
        build-*)
          LSINCE="$(launch_field "$DDIR" since_sha)"
          # THE BRANCH COMES FROM THE LAUNCH RECORD, never from the anchor now. Re-reading it here
          # compares the anchor's HEAD to itself, which made the verdict's `branch` conjunct
          # structurally vacuous on its only consumer: a child that ended on the WRONG branch or on
          # a DETACHED HEAD (a bad rebase, a stray `git checkout`) still satisfied `tip` and `tree`
          # and was reported `task-committed`.
          LBRANCH="$(launch_field "$DDIR" branch)"
          if [ "$ANCHOR_FALLBACK" = 1 ]; then
            # The dispatch RECORD survived its worktree; the child's WORK did not travel with it.
            # Verifying against the main worktree instead would answer a question nobody asked, so
            # this is no positive evidence and is surfaced as such.
            GITV="task-unverifiable worktree-removed"
          elif [ -z "$LBRANCH" ]; then
            # An older dispatch, or a detached anchor at launch. Falling back to the observation-time
            # branch would reinstate exactly the vacuity above, so this is NO POSITIVE EVIDENCE and
            # is surfaced as such — the same posture the empty-verdict case below already takes.
            GITV="task-unverifiable launch-branch-missing"
          else
            GITV="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" --build --worktree "$ANCHOR" \
                      --branch "$LBRANCH" --since "${LSINCE:-}" 2>/dev/null)"
          fi
          case "$GITV" in
            task-committed*) : ;;
            # OBSERVE-ONLY for build-*: never a re-dispatch. A build task may have left partial
            # commits, and re-running on top of them is docket-build's "never escalate onto a
            # stray commit" hazard. Report and stop. An empty verdict lands here too — a check
            # that could not run is not evidence of success.
            *) printf 'runner-dispatch: observe %s — FAILED (the child exited 0 but git disagrees: %s); work left in %s for inspection\n' \
                 "$OBSERVE_KEY" "${GITV:-no-verdict}" "$ANCHOR" >&2
               relay_child_stdout
               exit 1 ;;
          esac ;;
      esac
      printf 'runner-dispatch: observe %s — complete (child exited 0%s)\n' \
        "$OBSERVE_KEY" "${GITV:+; $GITV}" >&2
      relay_child_stdout
      exit 0
    fi
    printf 'runner-dispatch: observe %s — FAILED (child exited %s); see %s/stderr.log\n' \
      "$OBSERVE_KEY" "$SEC" "$DDIR" >&2
    relay_child_stdout
    exit 1
  }

  killed_field(){  # $1 = field -> first value from the kill marker, empty when absent
    local raw
    raw="$(sed -n "s/^$1=//p" "$DDIR/killed" 2>/dev/null)"
    printf '%s' "${raw%%$'\n'*}"
  }

  # THE GIVE-UP PATH, factored out because TWO conditions reach it: the observation budget being
  # spent (below), and the budget being UNENFORCEABLE for N consecutive passes (also below). Both
  # end the dispatch, and both must kill the detached group under the same identity check — a
  # second copy of that check is exactly the duplication the guard's own comment warns against.
  # `$1` is the CAUSE (`budget-exhausted` or `budget-unenforceable`), `$2` the diagnostic detail
  # for the latter. Never returns — it exits through the give-up verdict, or through the COMPLETED
  # disposition when the sentinel turns up inside the kill window (the two re-reads below).
  terminate_dispatch(){
    local cause="$1" detail="${2:-}" what
    case "$cause" in
      budget-unenforceable) what="the observation budget could not be enforced (${detail:-reason unrecorded})" ;;
      *)                    what="budget of ${DELEGATION_OBSERVATION_BUDGET}m exhausted" ;;
    esac
    # KILL THE WHOLE GROUP, never a single pid. A single-PID kill reaps the launcher shell and
    # ORPHANS the adapter and its children — precisely the half-dead state this change exists to
    # eliminate. Honors change 0231: no presumed-dead worker wakes to race its replacement. Partial
    # work stays in the worktree for a human.
    #
    # NEVER our own group, though. `--launch` fails closed when the child did not separate, so a
    # record it wrote can only name a foreign group — but a record can be wrong anyway (hand
    # edited, or a pgid reused after that group died), and a group-directed signal aimed at the
    # observer's own group takes down the harness that ran it. That is the one failure this facade
    # must not cause, so the impossible state is refused loudly instead of signalled.
    local my_pgid
    my_pgid="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
    if [ -n "$LPGID" ] && [ -n "$my_pgid" ] && [ "$LPGID" = "$my_pgid" ]; then
      printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (%s, but the launch record names THIS observation'"'"'s own process group (%s); refusing to signal it, so the dispatch was left running — inspect %s)\n' \
        "$OBSERVE_KEY" "$what" "$LPGID" "$DDIR" >&2
      exit 1
    fi
    # IDENTITY BEFORE SIGNAL, and the own-group refusal above is not it. A pgid is a REUSABLE
    # NAME, and this path is reached ONLY when no sentinel exists — which includes the child that
    # was killed externally, or died without the wrapper writing `done`, an hour before this
    # observation. By then the OS may have handed that id to an unrelated tree, and `kill -TERM
    # -<pgid>` reaches every process in it. Refusing to signal our OWN group defends the harness;
    # it defends nobody else. So the group is signalled only when it is provably STILL THE
    # LAUNCHED ONE, in two conjuncts against the launch record:
    #   1. the recorded `child_pid` must STILL LEAD the recorded group. The child is the group
    #      leader by construction (`set -m` creates a background job as one), so any other answer
    #      — the pid is gone, or it now sits in a different group — means the launched group is no
    #      longer reachable under that name.
    #   2. that pid's START TIME must still be the token measured at launch, because conjunct 1
    #      alone is satisfied by a RECYCLED pid that coincidentally leads a group of the same id
    #      (pid reuse is what makes the whole hazard reachable in the first place, and a recycled
    #      pid that calls setpgid(0,0) is an ordinary background job, not an exotic state).
    #      NEVER SKIPPED — an empty recorded token FAILS THE CONJUNCT rather than dropping it
    #      (change 0284: this conjunct is now scripts/lib/docket-liveness.sh's, shared with
    #      gate-run.sh, and the lib fails closed on either token being empty). The earlier spelling
    #      here skipped it, and that asymmetry against gate-run.sh's copy was the drift extracting
    #      the predicate removed. It costs nothing on any REACHABLE input: `--launch` records an
    #      empty `child_lstart` only when its `ps` saw no process at all — i.e. the child had
    #      already finished — and such a child's wrapper wrote `done`, which the observation's
    #      sentinel read disposes on before this path can be entered.
    # Failing either conjunct is NOT an error — it is the ordinary "that group is already gone"
    # outcome. The kill is skipped, the terminal `killed` marker is STILL recorded (with a reason
    # that says nothing was signalled, so the dispatch stays terminal and re-observes identically),
    # and the verdict is RESULT UNAVAILABLE either way: there is no result to report whether or not
    # a signal was sent.
    # THE RESIDUAL THIS ACCEPTS, deliberately: a group whose LEADER died while processes it spawned
    # keep running is not signalled, so those orphans outlive the budget. Killing them would mean
    # signalling a name we cannot prove is still theirs — an unrelated process group dying is the
    # worse of the two failures, and it is unrecoverable, while an orphan is visible and reapable.
    # An unreadable `ps` lands on the same side for the same reason.
    # Every one of these is initialized: `local x` leaves x UNSET, and this script runs under
    # `set -u`, so a later read on a path that skipped the assignment would abort the observation.
    local kill_reason="$cause" signal_group=0 identity_why="" lchild="" now_pgid=""
    lchild="$(launch_field "$DDIR" child_pid)"
    if [ -z "$LPGID" ]; then
      identity_why="the launch record names no process group"
    else
      case "$lchild" in
        ''|*[!0-9]*) identity_why="the launch record names no usable child pid" ;;
        *)
          # CONJUNCT 1, and it stays HERE rather than moving into the lib: it asks whether the
          # recorded CHILD still leads the recorded GROUP, which is a question about THIS file's
          # record layout (`child_pid` + `pgid`), not about liveness. The lib takes values; the
          # layout stays with its owner.
          now_pgid="$(ps -o pgid= -p "$lchild" 2>/dev/null | tr -d ' ')"
          if [ -z "$now_pgid" ]; then
            identity_why="the launched child (pid $lchild) is gone, so the group is no longer provably its own"
          elif [ "$now_pgid" != "$LPGID" ]; then
            identity_why="pid $lchild now leads group $now_pgid, not the recorded $LPGID"
          # CONJUNCT 2, now the shared predicate. It carries the reason in DOCKET_LIVENESS_WHY,
          # which is what let this call site drop its private copy of the wording rather than keep
          # a second predicate wearing the first one's answer.
          elif ! docket_group_alive_and_ours "$LPGID" "$(launch_field "$DDIR" child_lstart)"; then
            identity_why="$DOCKET_LIVENESS_WHY"
          else
            signal_group=1
          fi ;;
      esac
    fi

    if [ "$signal_group" = 1 ]; then
      # THE LAST LOOK BEFORE THE SIGNAL. The give-up path is entered off a "no sentinel" read taken
      # a `date`, a SUBPROCESS (`verify-run --iso-to-epoch`) and two `ps` calls ago — tens of
      # milliseconds in which the child can finish and write `done`. Signalling then would kill a
      # run that had already delivered its work and report it unavailable. Re-read here, where
      # nothing but the `kill` itself separates the test from the act, and hand a sentinel that
      # appeared straight to the completed disposition.
      [ -f "$DDIR/done" ] && report_done_disposition
      kill -TERM -"$LPGID" 2>/dev/null
      for _ in $(seq 1 20); do kill -0 -"$LPGID" 2>/dev/null || break; sleep 0.5; done
      kill -KILL -"$LPGID" 2>/dev/null
    else
      kill_reason="group-already-gone"
      printf 'runner-dispatch: observe %s — NOT signalling process group %s: %s. A pgid is a reusable name, so an unconfirmed one may now belong to an unrelated process group; recording the dispatch as terminal without a kill\n' \
        "$OBSERVE_KEY" "${LPGID:-<none>}" "$identity_why" >&2
    fi
    # AND AGAIN BEFORE THE MARKER. The read above closes the window up to the TERM; this one closes
    # what is left of it — the instant between the pre-signal test and the signal actually landing
    # (a wrapper whose `mv` completed as the TERM arrived), and the whole of the no-signal path,
    # where the check above never ran at all. THE CORRECTNESS ARGUMENT is the same one that orders
    # the sentinel ahead of the marker: the untrapped wrapper subshell is the only writer of `done`
    # and a group TERM/KILL reaches it, so once the kill has been delivered a sentinel can no longer
    # appear — a `done` visible HERE was therefore written BEFORE the signal, by a child that
    # completed. Recording a `killed` marker over it would mask a finished run permanently, which is
    # the defect this pair of re-reads removes.
    [ -f "$DDIR/done" ] && report_done_disposition
    # `reason` keeps its established meaning — WHETHER a signal went out — and `cause` is the new,
    # orthogonal field saying WHY the facade gave up, so the terminal re-report above can word both
    # axes without a reader ever being told a kill happened when none did.
    # `mv -f`, not `mv`: BSD `mv` onto an unwritable destination with a tty PROMPTS, self-answers
    # `n` at EOF, and exits 0 — the marker would be silently lost and the kill would re-fire on the
    # next observation.
    printf 'killed_at=%s\nreason=%s\ncause=%s\ndetail=%s\nbudget_minutes=%s\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$kill_reason" "$cause" "$detail" "$DELEGATION_OBSERVATION_BUDGET" \
      > "$DDIR/killed.partial"
    mv -f "$DDIR/killed.partial" "$DDIR/killed" || die "could not record the kill marker in $DDIR"
    if [ "$signal_group" = 1 ]; then
      printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (%s; the detached process group was terminated)\n' \
        "$OBSERVE_KEY" "$what" >&2
    else
      printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (%s; the recorded process group could not be confirmed as the launched child'"'"'s, so nothing was signalled — inspect %s)\n' \
        "$OBSERVE_KEY" "$what" "$DDIR" >&2
    fi
    exit 1
  }

  # 1. THE SENTINEL IS READ FIRST — ahead of the `killed` marker, deliberately. THE WRAPPER SUBSHELL
  #    IS UNTRAPPED and it is the only writer of `done`, so a group-directed TERM/KILL reaches it and
  #    it can never write a sentinel afterwards. A `done` sitting beside a `killed` marker therefore
  #    proves the child COMPLETED BEFORE the signal — the give-up merely raced a run that had already
  #    finished — and the completed disposition is the true one. Read the other way round (the
  #    original order), a sentinel that landed anywhere between the give-up path's "no sentinel" read
  #    and its marker write was masked FOREVER, reporting a completed run as RESULT UNAVAILABLE and
  #    sending a human to hunt for work that is in fact committed.
  #
  #    IDEMPOTENCE IS UNHARMED by the precedence: neither file is ever removed, and a sentinel that
  #    exists on one observation exists on every later one, so the verdict read from it is the same
  #    forever after. The only transition this admits is unavailable -> complete, and it can happen
  #    at most once, only on the no-signal (`reason=group-already-gone`) path where nothing was
  #    actually killed, and only on the arrival of real evidence that the work finished. Reporting a
  #    result the facade can see is not an oscillation.
  if [ -f "$DDIR/done" ]; then
    report_done_disposition
  fi

  # 2. A prior give-up is TERMINAL and re-reports identically forever (idempotence). The marker
  #    carries two orthogonal facts and both are spoken: `cause` says WHY the facade gave up (the
  #    budget ran out, or it could not be enforced at all), and `reason` says whether a group was
  #    actually terminated or left alone because it could not be confirmed as the launched child's
  #    (see the identity check above). Both are unavailable and both exit 1; collapsing them into
  #    one wording would tell a reader a kill happened when none did.
  if [ -f "$DDIR/killed" ]; then
    KREASON="$(killed_field reason)"
    KCAUSE="$(killed_field cause)"
    KDETAIL="$(killed_field detail)"
    case "$KCAUSE" in
      budget-unenforceable) KWHAT="the observation budget could not be enforced (${KDETAIL:-reason unrecorded})" ;;
      *)                    KWHAT="the budget was exhausted" ;;
    esac
    case "$KREASON" in
      group-already-gone)
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (%s and the recorded process group could not be confirmed as the launched child'"'"'s, so nothing was signalled)\n' "$OBSERVE_KEY" "$KWHAT" >&2 ;;
      *)
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (%s; the detached run was killed)\n' "$OBSERVE_KEY" "$KWHAT" >&2 ;;
    esac
    exit 1
  fi

  # 3. No sentinel: still running, unless the budget is spent. `0` is legal and buys exactly ONE
  #    observation — this one — so the comparison is `>=`, evaluated AFTER the sentinel read
  #    above, which is what makes that single observation a real one.
  NOW="$(date -u +%s 2>/dev/null)"
  START_EPOCH=""
  [ -n "$LSTART" ] && START_EPOCH="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" --iso-to-epoch "$LSTART" 2>/dev/null)"
  # Neither clock read is positive evidence that the budget is spent, so an unreadable one keeps
  # observing rather than killing a healthy child on a guess. Same posture, twice.
  #
  # BUT "do not enforce" cannot mean "never terminate". `4` is the caller's loop condition, so a
  # state that returns it unconditionally is a state the loop can never leave: an unreadable clock,
  # an unreadable `started_at`, or a launch record that is missing or unparseable (an empty
  # `started_at` field alone puts every later observation here) would each spin the caller forever
  # while the budget is never once enforced. So the unenforceable passes are COUNTED, and the Nth
  # CONSECUTIVE one converts to the same terminal give-up the spent budget takes.
  #
  # N = 3, and the choice is about which failures are transient. The only genuinely transient
  # member of this family is a clock read that fails under momentary load; a launch record that
  # cannot be parsed never repairs itself, so for it any N>1 is pure grace. Three consecutive
  # passes at the shim's paced cadence is minutes of tolerance — enough that a blip is ridden out,
  # short enough that a permanently unreadable dispatch is terminal long before a human notices the
  # loop. The counter RESETS on any enforceable pass (below), so a single bad read never
  # accumulates toward termination across an otherwise healthy run.
  #
  # THE IDEMPOTENCE SCOPE (scripts/runner-dispatch.md states the refined guarantee): the counter is
  # the one piece of mutable state an observation writes besides the terminal marker, and it is
  # reachable ONLY here — on the still-running-and-unenforceable path. Every TERMINAL state (killed,
  # done, a git verdict) is decided above, before a single byte of it is read or written, so a
  # completed, failed or killed dispatch still re-reports identically forever.
  UNENFORCEABLE_MAX=3
  note_unenforceable(){  # $1 = why the budget could not be enforced this pass — never returns
    local why="$1" n
    n="$(sed -n 1p "$DDIR/unenforceable" 2>/dev/null)"
    case "$n" in ''|*[!0-9]*) n=0 ;; esac
    n=$(( n + 1 ))
    printf '%s\n' "$n" > "$DDIR/unenforceable.partial"
    # A counter that cannot be persisted bounds nothing, and the loop would run forever on the very
    # state this exists to end — so an unwritable dispatch dir is itself terminal, immediately.
    mv -f "$DDIR/unenforceable.partial" "$DDIR/unenforceable" 2>/dev/null \
      || terminate_dispatch budget-unenforceable "$why, and the observation counter could not be recorded in $DDIR"
    if [ "$n" -ge "$UNENFORCEABLE_MAX" ]; then
      terminate_dispatch budget-unenforceable "$why, on $n consecutive observations"
    fi
    printf 'runner-dispatch: observe %s — still running (%s; budget not enforced this pass, %s of %s)\n' \
      "$OBSERVE_KEY" "$why" "$n" "$UNENFORCEABLE_MAX" >&2
    exit 4
  }
  case "${NOW:-}" in
    ''|*[!0-9]*) note_unenforceable "the clock could not be read" ;;
  esac
  case "${START_EPOCH:-}" in
    ''|*[!0-9]*) note_unenforceable "the launch record's start time is missing or unreadable" ;;
  esac
  # This pass IS enforceable, so the consecutive run ends here — a transient unreadable clock must
  # not accumulate toward termination across a healthy run.
  rm -f "$DDIR/unenforceable" 2>/dev/null
  ELAPSED_MIN=$(( (NOW - START_EPOCH) / 60 ))
  if [ "$ELAPSED_MIN" -lt "$DELEGATION_OBSERVATION_BUDGET" ]; then
    printf 'runner-dispatch: observe %s — still running (%sm of %sm budget)\n' \
      "$OBSERVE_KEY" "$ELAPSED_MIN" "$DELEGATION_OBSERVATION_BUDGET" >&2
    exit 4
  fi

  # 4. Budget exhausted: give up on the dispatch. The kill, the identity check that gates it and
  #    the terminal marker all live in `terminate_dispatch` above, shared with the
  #    budget-UNENFORCEABLE termination — one identity-checked kill path, never two copies.
  terminate_dispatch budget-exhausted
fi

# --- run gate (change 0237), part 1: the "before" snapshot --------------------------
# Engages ONLY for an implement-next delegation. That scoping is load-bearing, not an
# optimization: a build-* delegation leaves its change `in-progress` BY DESIGN (the build role
# does not reach Step 7), so gating one would fire on every healthy build. status / adr /
# review-* / finalize-change / auto-groom are likewise out of scope, and an unrecognised agent is
# a no-op — never a guess.
#
# This SYNCHRONOUS path's gate stays scoped to implement-next. The build disposition is NOT bolted
# on here: it lives on the `--observe` seam (change 0271), where a detached child's terminal state
# is actually knowable and where the subject is a COMMIT on a feature branch rather than a change's
# metadata status. The two never overlap, which is why this fence never grew a build leg.
#
# The snapshot must be taken BEFORE the handoff: the gate's subject is what THIS run claimed, and
# that is only knowable as a diff across the hand-off. A snapshot that cannot be read disables the
# gate with a warning and leaves the dispatch untouched — the facade's standing tolerant posture on
# this live path (the same reason an unparseable runners.<name>: value `continue`s rather than
# dying): the gate must never convert a healthy dispatch into a failure.
#
# BOTH reads must come from FRESH ORIGIN state (LEARNINGS: cas-re-read-fresh-origin), which is why
# the re-sync below is symmetric with the one after the handoff. An asymmetric pair is not merely
# imprecise, it is actively wrong: a change that is `in-progress` on origin/docket but not yet in
# the local .docket worktree is absent from a stale BEFORE and present in the freshly-synced AFTER,
# so an ABANDONED claim from an earlier session (exactly what board-checks' `aborted-run` leg
# exists for) would be attributed to this run and spend a whole agent run being re-dispatched.
#
# THIS FENCE IS FOR THE SYNCHRONOUS PATH ONLY, and it is still reachable: a hand invocation with no
# `--launch` still blocks here. The DELEGATED path no longer passes through it at all — the
# generated shim always launches — so the same disposition is carried on the `--observe` seam by
# `observe_implement_next`, which mirrors this attribution exactly.
#
# `in_progress_ids`, `in_progress_claims` and `resync_metadata` are defined above the verbs so both
# halves share one reader.
GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1

BEFORE=""; DISPATCH_EPOCH=""
if [ "$GATE" = 1 ]; then
  resync_metadata || :
  if ! BEFORE="$(in_progress_ids)"; then
    printf 'runner-dispatch: run gate disabled — could not read the in-progress set\n' >&2
    GATE=0
  fi
  # The claim window opens here — AFTER the before-read, so a claim landing in the gap between the
  # two is either already in BEFORE or stamped before the window, and is excluded either way.
  # `date -u +%s` is UTC epoch seconds on GNU and BSD alike, and is the ONLY timestamp work done
  # here: the ISO->epoch direction (which is where the two `date` dialects actually diverge) stays
  # in verify-run, on the shared `iso_to_epoch`. The claim writer stamps UTC ISO-8601
  # (`date -u +%Y-%m-%dT%H:%M:%SZ`), so the two ends meet on the same clock.
  DISPATCH_EPOCH="$(date -u +%s 2>/dev/null)"
  case "$DISPATCH_EPOCH" in
    ''|*[!0-9]*)
      printf 'runner-dispatch: run gate disabled — could not read the clock\n' >&2
      GATE=0 ;;
  esac
fi

# CALL-AND-RETURN, not exec (change 0237). The facade must regain control so the run gate can read
# what the delegated run actually left in git — that seam is the whole point, and `exec` made it
# unreachable by replacing this process with the adapter's image. Removing `exec` shifts process
# semantics: the adapter is now a CHILD rather than a replacement image, so it gets its own pid and
# this process stays alive as its parent. Two consequences are deliberate. (1) The adapter's exit
# code is captured and propagated VERBATIM on every path where the gate takes no action, so no
# existing caller observes a behavior change.
#
# (2) SIGNAL DELIVERY, measured rather than assumed (spec Risk 2; change 0237 build). The facade
# installs NO traps, so behavior splits by how the signal is addressed:
#   - GROUP-directed (what a terminal Ctrl-C does — it signals the whole foreground process group):
#     UNCHANGED. The adapter is in that group and gets the signal directly, its own trap fires
#     immediately, and the facade dies of the same signal (130 on INT, 143 on TERM). This is the
#     interactive path, and it is the reason no trap is added here.
#   - PID-directed (a supervisor doing `kill -INT <facade pid>`, e.g. `timeout`): CHANGED, because
#     the facade is now a separate process that absorbs the signal instead of being the adapter.
#     A pid-directed INT is deferred while this shell waits on its child and is then discarded
#     (exit 0); a pid-directed TERM kills this shell at once and ORPHANS the still-running adapter.
#     Under `exec` neither was possible — there was only one process to signal. Nothing in docket
#     signals the facade by pid today; forwarding traps are deliberately NOT added here because
#     that is a design decision for the run gate's own error posture, not a silent side effect of
#     this refactor.
# ONE channel, always: the facade never hands an adapter the both-channels shape its own
# defensive gate refuses. With a brief file the argv channel is empty by construction (the gate
# above refused any trailing argument), so the `--` terminator is passed with nothing after it.
if [ -n "$BRIEF_PATH" ]; then
  "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" --brief-file "$BRIEF_PATH" --
else
  "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
fi
rc=$?

# --- run gate, part 2: the "after" snapshot, the diff, and the verdicts -------------
[ "$GATE" = 1 ] || exit "$rc"

# The "after" read must come from FRESH ORIGIN state, never from the local tree the child just
# wrote (LEARNINGS: cas-re-read-fresh-origin). Best-effort: a failed re-sync degrades the gate's
# freshness, it does not fail a dispatch that may well have succeeded.
resync_metadata || :

AFTER="$(in_progress_claims)" || {
  printf 'runner-dispatch: run gate disabled — could not re-read the in-progress set\n' >&2
  exit "$rc"
}

# ATTRIBUTION. A candidate for "this run's claim" must clear THREE filters, because the set diff
# alone does not identify a claimant — it only identifies a change of state:
#
#   1. not in BEFORE — a claim already held at the handoff is another run's (or an earlier
#      session's), and both reads are of fresh origin so BEFORE really is the whole prior set;
#   2. `claimed_at` readable — an absent or unparseable stamp is NO POSITIVE EVIDENCE of ownership,
#      and the gate acts only on a positive finding, never on a guess;
#   3. `claimed_at` at or after DISPATCH_EPOCH — a claim stamped before this run started cannot be
#      ours however it came to be visible. This is what excludes the abandoned in-progress change
#      that the pre-handoff re-sync above pulled in for the first time, even on the path where that
#      re-sync FAILED and BEFORE is stale — belt and braces, deliberately.
#
# WHAT THIS DOES NOT ESTABLISH, stated plainly so no later reader trusts more than it holds: a
# timestamp cannot separate our claim from one a CONCURRENT loop made during our run, and
# `claimed_at` is re-stamped at every phase boundary, so a foreign live run's stamp is fresh too.
# The ambiguity is therefore handled by COUNTING rather than by the clock: an implement-next run
# claims at most ONE change, so two or more candidates means at least one is not ours and none can
# be told apart — the gate disables itself rather than re-dispatching onto a change another agent
# is holding. A run that claimed nothing (drained, or contended where the CAS was lost) yields an
# empty set and the gate is a no-op.
NEW_IDS=()
while IFS=' ' read -r nid nclaimed; do
  [ -n "$nid" ] || continue
  grep -qxF "$nid" <<<"$BEFORE" && continue
  case "${nclaimed:-}" in
    ''|*[!0-9]*)
      printf 'runner-dispatch: run gate — ignoring change %s: no readable claimed_at\n' "$nid" >&2
      continue ;;
  esac
  [ "$nclaimed" -ge "$DISPATCH_EPOCH" ] || {
    printf 'runner-dispatch: run gate — ignoring change %s: claimed before this dispatch started\n' "$nid" >&2
    continue
  }
  NEW_IDS+=("$nid")
done <<<"$AFTER"

if [ "${#NEW_IDS[@]}" -gt 1 ]; then
  printf 'runner-dispatch: run gate disabled — %s changes were claimed during this dispatch (%s); this run claims at most one, so none can be attributed to it\n' \
    "${#NEW_IDS[@]}" "${NEW_IDS[*]}" >&2
  exit "$rc"
fi

STILL_INCOMPLETE=(); HALTED=(); GATE_VERIFIED_DONE=0
for nid in "${NEW_IDS[@]:-}"; do
  [ -n "$nid" ] || continue
  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — %s\n' "${verdict:-run-unverifiable $nid}" >&2
  case "$verdict" in
    run-incomplete*) : ;;
    # run-halted NEVER re-dispatches: a halt means a human is needed, and spending a second full
    # agent run on it is waste. But it is not a no-op either — docket-implement-next's disposition
    # table pins `halted` to STOP + SURFACE, and returning the adapter's (healthy) 0 here would tell
    # a driver to draw the next change, which is exactly the prose-level failure this change exists
    # to replace. It gets its OWN terminal code at this seam (see below).
    run-halted*) HALTED+=("$verdict"); continue ;;
    # run-complete and run-unclaimed need nothing. An empty/unparseable verdict falls here too —
    # the gate acts only on a POSITIVE finding, never on a guess.
    *) continue ;;
  esac

  # ONE bounded re-dispatch — docket-build's one-escalation-per-task rule, applied at this seam.
  # One re-dispatch is a real cost (a false `run-incomplete` spends a full agent run), which is
  # exactly why the bound is one and not a loop: the second strike stops and tells a human.
  unmet="${verdict#run-incomplete "$nid" }"
  retry_ctx="docket-implement-next $nid — the previous run left Step 7 unmet (${unmet}); resume that change and finish it: push the branch, open the PR, and write status: implemented + pr:. If it genuinely cannot proceed, write a '## Run halted' section into the change file and commit it — the heading must be bare and undated (the reader matches the whole line), so put the date inside the body."
  printf 'runner-dispatch: run gate — re-dispatching once for change %s (%s)\n' "$nid" "$unmet" >&2
  # ONE CHANNEL ON THE RETRY TOO (change 0277). The retry context used to ride as an extra trailing
  # argument, which — with a brief file in play — is exactly the both-channels shape the adapters
  # refuse ("never both"), so the facade would kill its own re-dispatch on a path no caller can see.
  # Instead the context is appended to a COMBINED brief: the original brief's bytes, then a blank
  # line, then the retry context. Never dropped, never a second channel. Templated into
  # TMPDIR per this repo's mktemp rule; removed once the re-dispatch returns.
  if [ -n "$BRIEF_PATH" ]; then
    RETRY_BRIEF="$(mktemp "${TMPDIR:-/tmp}/docket-retry-brief.XXXXXX")" || die "cannot create the re-dispatch brief"
    # EACH HALF CARRIES ITS OWN GUARD. A brace group's exit status is its LAST command's, so
    # `{ cat …; printf …; } > f || die` is blind to a failed `cat` — the re-dispatch would then run
    # on a brief holding the retry context ALONE, with the task stripped out, while this gate
    # reported an ordinary re-dispatch. On the synchronous verb `BRIEF_PATH` is the CALLER's temp
    # file, re-read here only after a full delegated run, so TMPDIR reaping or the caller's own
    # cleanup taking it away is a live path, not a theoretical one.
    cat "$BRIEF_PATH" > "$RETRY_BRIEF" \
      || die "cannot read the original brief $BRIEF_PATH into the re-dispatch brief $RETRY_BRIEF"
    # THE SEPARATOR IS UNCONDITIONAL. A brief file need not end with a newline — a caller's
    # heredoc does, a `printf` without a trailing `\n` does not — and `printf '\n%s\n'` alone
    # produces the promised BLANK line only in the first case. In the second it merely terminates
    # the brief's last line, gluing the retry context onto it: the boundary loss this whole change
    # exists to remove. So terminate the brief's last line first when it is unterminated, and only
    # then write the blank line and the context.
    if [ -n "$(tail -c 1 "$RETRY_BRIEF")" ]; then
      printf '\n' >> "$RETRY_BRIEF" || die "cannot write the re-dispatch brief $RETRY_BRIEF"
    fi
    printf '\n%s\n' "$retry_ctx" >> "$RETRY_BRIEF" \
      || die "cannot write the re-dispatch brief $RETRY_BRIEF"
    "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" --brief-file "$RETRY_BRIEF" --
    rm -f "$RETRY_BRIEF"
  else
    "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@" "$retry_ctx"
  fi

  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — after re-dispatch: %s\n' "${verdict:-run-unverifiable $nid}" >&2
  case "$verdict" in
    run-incomplete*) STILL_INCOMPLETE+=("$verdict") ;;
    # A halt discovered on the SECOND verdict is the same disposition as one discovered on the
    # first: terminal, exit 3. The re-dispatched run stopping deliberately is not a success, so it
    # never reaches the override below.
    run-halted*) HALTED+=("$verdict") ;;
    # The re-dispatch DROVE THE RUN HOME. From here the gate's own git-read verdict is a stronger
    # fact than the first adapter's status: that first code very often accompanies a run that
    # stopped short, and returning it would report a now-verified-complete run as a failure. Only a
    # POSITIVE second verdict qualifies — an empty or unparseable one falls through to `$rc`, since
    # the gate never acts on a guess.
    run-complete*|run-unclaimed*) GATE_VERIFIED_DONE=1 ;;
  esac
done

if [ "${#STILL_INCOMPLETE[@]}" -gt 0 ]; then
  # Abort-and-report. The change stays `in-progress` with its claim intact; board-checks'
  # `aborted-run` remains the standing backstop. This is the only NEW non-zero this change
  # introduces, and it is on a path that is presently silent. The retry's own exit code is
  # deliberately NOT propagated: `$rc` (the first adapter's code) is what every no-action path
  # returns, and this abort supersedes it with 1 rather than reporting a second, different code.
  printf 'runner-dispatch: RUN GATE FAILED after one re-dispatch — a delegated implement-next run did not reach its PR:\n' >&2
  for v in "${STILL_INCOMPLETE[@]}"; do printf '  %s\n' "$v" >&2; done
  exit 1
fi

if [ "${#HALTED[@]}" -gt 0 ]; then
  # STOP + SURFACE, with its own code. 3, not 1: the two-strikes abort above is a run that FAILED
  # to finish, whereas a halt is the delegated run deliberately stopping because a human is needed —
  # two different terminal outcomes deserve two different codes, and a driver that wants to
  # distinguish them can. The consumer that exists today is the generated shim wrapper
  # (sync-agents.sh's emit_shim), whose rule is bare-non-zero: "if the dispatch exits non-zero,
  # abort-and-report its stderr diagnostic — never retry silently". That reading is CORRECT for a
  # halt — abort-and-report IS stop + surface — so the new code costs nothing there and the stderr
  # lines below carry the distinction. This is the one place the facade returns non-zero for
  # something that is not a failure of the dispatch itself; it is deliberate, and it is why the code
  # is distinct rather than folded into 1.
  printf 'runner-dispatch: RUN HALTED — a delegated implement-next run stopped and needs a human:\n' >&2
  for v in "${HALTED[@]}"; do printf '  %s\n' "$v" >&2; done
  printf 'runner-dispatch: not continuing — read the change file'"'"'s "## Run halted" section.\n' >&2
  exit 3
fi

# A re-dispatch ran and the second verdict positively showed the run finished. `$rc` is the FIRST
# adapter's code and is stale here — it describes the attempt the gate just superseded, and a
# non-zero first code is a common accompaniment to a run that stopped short. This is the one path
# where the facade drops the adapter's code without aborting; every path where NO re-dispatch ran
# still returns it verbatim, which is why the override keys on GATE_VERIFIED_DONE (set only inside
# the retry loop) rather than on the verdict alone.
[ "$GATE_VERIFIED_DONE" = 1 ] && exit 0

exit "$rc"
