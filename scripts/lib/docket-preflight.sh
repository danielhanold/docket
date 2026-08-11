#!/usr/bin/env bash
# scripts/lib/docket-preflight.sh — the shared Step-0 preflight (change 0068). Sourced by
# scripts/docket.sh and scripts/docket-status.sh; extracts the metadata-worktree sync that was
# docket-status.sh's private ensure_and_sync_worktree so there is ONE sync implementation.
#
# docket_preflight <scripts_dir>
#   1. resolve config: eval "$(${CONFIG_EXPORT_CMD:-<scripts_dir>/docket-config.sh --export})"
#      into the CALLER's scope (DOCKET_MODE, METADATA_BRANCH, METADATA_WORKTREE, BOOTSTRAP, …).
#   2. enforce the bootstrap verdict fail-closed (non-PROCEED => return 1 + stderr diagnostic).
#   3. ensure + sync the metadata worktree (docket-mode, against METADATA_BRANCH) or the primary
#      tree (main-mode, against INTEGRATION_BRANCH — required, never defaulted); disable the
#      metadata worktree's shared git hooks (best-effort, change 0063). Either way the tree must
#      already BE on that branch: preflight syncs docket's metadata branch, it never converts a
#      tree from one branch to another.
#   Returns 0 on success. Prints nothing on stdout. Honors the GIT and CONFIG_EXPORT_CMD seams.
# This file is a sourced helper: it is documented within its callers' contracts (docket.md,
# docket-status.md), not by a co-located .md (test_script_contracts_coverage.sh scopes lib/ out).

# shellcheck source=docket-root.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docket-root.sh"

# --- metadata sync: bounded, discriminating retry (change 0247) ----------------------------------
# 5 attempts with 2/4/8/8s backoff (~22s total). The RATIONALE, not just the number: the collision
# this retries is "another agent is between its edit and its push" in the SHARED .docket worktree.
# Most such windows close in seconds once that agent commits, and an autonomous caller re-running
# preflight later covers the long tail — so a longer budget buys little while blocking every
# skill's Step 0 for it. Calibrated on the live collisions observed in changes 0109/0110, on one
# machine: treat it as a starting tolerance, not a measured constant.
#
# DOCKET_SYNC_BACKOFF is a Bash array, so this file stays Bash-only. That costs nothing: both
# sourcing callers already are. docket-status.sh is `#!/usr/bin/env bash`, and docket.sh's `/bin/sh`
# prologue sources nothing — it re-execs into the configured Bash 4+ runtime BEFORE its
# `. "$SELF_DIR"/lib/docket-preflight.sh` line is ever parsed.
DOCKET_SYNC_ATTEMPTS=5
DOCKET_SYNC_BACKOFF=(2 4 8 8)

# _docket_sync_sleep SECONDS — the injectable backoff seam. DOCKET_PREFLIGHT_TEST_SLEEP_CMD, when
# set, REPLACES the real sleep: fixtures drive all five attempts at zero wall-clock cost (the
# suite's per-file budgets make ~22s of real sleeping in a test a defect, not a style choice), and
# a fixture modelling "the other agent finished" mutates its repo from inside that command — the
# only point in the loop where a second actor could have acted. DOCKET_-namespaced per ADR-0014.
# The fixture's own output is discarded on both channels: the caller's stderr is the sync's
# diagnostic channel, which the exhaustion asserts read, and fixture noise there is contamination.
_docket_sync_sleep(){
  if [ -n "${DOCKET_PREFLIGHT_TEST_SLEEP_CMD:-}" ]; then
    eval "$DOCKET_PREFLIGHT_TEST_SLEEP_CMD" >/dev/null 2>&1 || true
    return 0
  fi
  sleep "$1"
}

# _docket_tree_dirty GIT DIR — true (0) when TRACKED files are modified in DIR. Untracked-only
# files never count as dirty (ADR-0046's two-sided lesson): a stray untracked file in the shared
# worktree must never fail another agent's sync.
_docket_tree_dirty(){
  [ -n "$("$1" -C "$2" status --porcelain --untracked-files=no 2>/dev/null)" ]
}

# _docket_tree_wedged GIT DIR — true (0) when a rebase or merge is already in progress in DIR.
# It lives here, rather than inside the sync, because it is the shared shape of one hazard: a
# commit into a mid-rebase shared tree commits that rebase's staged content under the caller's
# message, which is why change 0247's Half 2 gates docket-status.sh's commit_and_push_generated on
# this same probe. The state lives under the worktree's OWN git dir — for a linked worktree that is
# <main>/.git/worktrees/<name>, never <linked>/.git, which is a gitdir pointer FILE — so resolve it
# through git rather than assuming a layout.
_docket_tree_wedged(){
  local gd
  gd="$("$1" -C "$2" rev-parse --git-dir 2>/dev/null)" || return 1
  [ -n "$gd" ] || return 1
  case "$gd" in /*) ;; *) gd="$2/$gd" ;; esac
  [ -d "$gd/rebase-merge" ] || [ -d "$gd/rebase-apply" ] || [ -f "$gd/MERGE_HEAD" ]
}

# _docket_tree_detached GIT DIR — true (0) when HEAD in DIR is not on a branch. Deliberately a
# SEPARATE predicate rather than another clause inside _docket_tree_wedged: docket-status.sh
# consumes that one by name at two sites for a class that is transient (another agent mid-sync,
# worth waiting out), and a detached HEAD is its opposite — nothing but a human clears it.
# `symbolic-ref -q HEAD` is the probe rather than `rev-parse --abbrev-ref HEAD`, whose detached
# answer is the literal string "HEAD" and so collides with a branch that could be named that.
_docket_tree_detached(){
  ! "$1" -C "$2" symbolic-ref -q HEAD >/dev/null 2>&1
}

# _docket_tree_on_other_branch GIT DIR BRANCH — true (0) when git NAMES a branch for DIR's HEAD and
# that name is not BRANCH. Ordered strictly AFTER the wedged and detached probes in the sync below,
# never before them: an interrupted rebase leaves HEAD detached at the `onto` commit, so a
# wrong-branch probe placed first would relabel every wedged and every detached tree as "wrong
# branch" and the two classes that need their own remedies would vanish behind it.
#
# Phrased as "git named a DIFFERENT branch", not as the negation of "git named THIS branch", and the
# difference is the empty answer. Real git either prints a branch and exits 0 or prints nothing and
# exits non-zero, so an empty name reaching here is not a wrong branch — it is an unreadable answer,
# and "HEAD names no branch" is the class the detached arm above already owns, decided on
# `symbolic-ref`'s EXIT STATUS, which is the thing real git actually varies. Re-deciding it here on
# weaker evidence would put two arms in a fight over one class. (It is also what keeps a caller's
# no-op GIT stub — several hermetic fixtures use one to test things that are not the sync — out of
# a class it was never answering questions about.)
#
# The comparison is against the SHORT name, because BRANCH arrives as a config value ("docket",
# "main", "trunk") rather than a full ref. A branch whose short name is ambiguous with a tag is not
# a case this distinguishes; the sync's own fetch already names the same short branch.
_docket_tree_on_other_branch(){
  local cur
  cur="$("$1" -C "$2" symbolic-ref --short -q HEAD 2>/dev/null)" || cur=""
  [ -n "$cur" ] && [ "$cur" != "$3" ]
}

# _docket_sync_metadata GIT DIR REMOTE BRANCH — the ONE metadata sync (change 0247), used by both
# branches of docket_preflight so they cannot drift apart.
#
# INVARIANT: it may report success when local metadata is already current with, or AHEAD of, the
# fetched remote; it reports success ONLY when HEAD is on BRANCH ITSELF and no git operation is in
# progress; it integrates remote changes ONLY when, additionally, the tracked tree is clean; and it
# NEVER mutates another agent's in-flight state to get there — no --autostash, no reset, no stash.
# Review any change to this function against those four clauses. The second one is stated
# separately from the third on purpose: collapsing them into "the integrate path checks it" is
# precisely the reading that shipped a fast path returning 0 on a mid-rebase shared tree.
#
# "HEAD is on BRANCH itself" is the whole of clause two, not merely "HEAD is on some branch". The
# weaker reading rebased the checked-out branch onto BRANCH's remote tip whatever that branch was —
# harmless in a worktree docket owns and on a branch it put there, and a SILENT REWRITE OF THE
# USER'S OWN HISTORY in main-mode, where DIR is the human's primary worktree and a topic branch
# checked out there is an ordinary state. Preflight is Step 0 of every docket skill, so that fired
# constantly. Both callers now pass the branch their tree is supposed to be on, which is exactly
# what makes the question answerable here; policing it in one caller instead would be the drift the
# spec's "both branches of the sync function must behave identically" clause forbids.
#
# Returns 0 on success; 1 on a terminal failure or retry exhaustion, with a stderr diagnostic that
# NAMES the last failure class (dirty / wedged / detached / wrong-branch / fetch / conflict), so the
# caller learns what blocked the sync rather than merely that attempts died.
_docket_sync_metadata(){
  local git="$1" dir="$2" remote="$3" branch="$4"
  local attempt=0 last=fetch head remote_sha nap cur
  # BRANCH is a precondition, not an optional argument, and an empty one is caught HERE rather than
  # left to fail somewhere downstream. `git fetch origin ""` does not error out: measured on git
  # 2.55 against a real remote it fetches that remote's HEAD and returns 0, so the empty string
  # travels all the way to the wrong-branch arm and the human is told to `check out ''` — a
  # diagnostic blaming their checkout for the caller's bug.
  if [ -z "$branch" ]; then
    echo "docket-preflight: metadata sync failed — no branch was named to sync $dir against. The caller must resolve one; syncing against an empty ref is not a fallback." >&2
    return 1
  fi
  while [ "$attempt" -lt "$DOCKET_SYNC_ATTEMPTS" ]; do
    attempt=$((attempt + 1))
    if ! "$git" -C "$dir" fetch "$remote" "$branch" >&2; then
      # Fetch failures retry UNDISCRIMINATED, deliberately: git's exit codes do not portably
      # separate an auth or bad-remote failure from a transient network one, and stderr-pattern
      # matching is locale- and version-fragile. Accepted limit — the diagnostic carries the class
      # and git's own stderr is already on the caller's channel.
      last=fetch
    else
      head="$("$git" -C "$dir" rev-parse HEAD 2>/dev/null)"
      # The remote tip is read from the REMOTE-TRACKING REF the fetch just advanced, not from
      # FETCH_HEAD, for BOTH callers. FETCH_HEAD is per-worktree but not per-PROCESS, and in
      # main-mode $dir is the primary worktree — shared with every other docket helper that fetches
      # there (sync-integration-branch.sh, terminal-publish.sh). One of those landing between this
      # fetch and this read would silently hand the rebase below a DIFFERENT branch's tip. The
      # remote-tracking ref names what we asked for, so a concurrent fetch of another ref cannot
      # move it, and one of the SAME ref only moves it forward.
      #
      # FETCH_HEAD stays as the fallback rather than being deleted: `git fetch <remote> <branch>`
      # only updates refs/remotes/<remote>/<branch> opportunistically, i.e. when the remote has a
      # fetch refspec configured. Every cloned or `git remote add`-ed remote does; one hand-edited
      # into having none would otherwise leave remote_sha empty and drive `git rebase ""` into the
      # terminal unknown-failure arm — a hard regression for a repo the old code served fine.
      remote_sha="$("$git" -C "$dir" rev-parse --verify -q "refs/remotes/$remote/$branch" 2>/dev/null)" || remote_sha=""
      [ -n "$remote_sha" ] || remote_sha="$("$git" -C "$dir" rev-parse FETCH_HEAD 2>/dev/null)"
      # WHAT STATE IS HEAD IN — classified BEFORE any arm may report success, because the
      # INVARIANT's "no git operation is in progress" clause binds the SUCCESS path and not merely
      # the integrate path. Getting this order wrong is not hypothetical: probing the wedge only
      # AFTER the fast path returned made preflight green-light the very state this function exists
      # to survive. A conflicted rebase leaves HEAD DETACHED at the rebase's `onto` commit, so
      # whenever the remote had not advanced past it `rev-parse HEAD` equalled `rev-parse
      # FETCH_HEAD`, the ancestor test below was trivially true, and the wedge was never consulted
      # (observed on git 2.55). An agent's correctly scoped commit then lands on that detached HEAD
      # — the branch ref never moves — and the next `rebase --abort` destroys it. Preflight is the
      # only mechanical gate the agent channel has, so it fails closed here.
      #
      # Wedged is probed before detached because a rebase in progress is ALSO detached, and the
      # more specific class wins — the same rule that puts it ahead of the dirty check, a wedged
      # tree being dirty too.
      if _docket_tree_wedged "$git" "$dir"; then
        # A rebase/merge that PREDATES this attempt is another agent mid-sync: transient, so it
        # spends budget regardless of whether the remote moved. Only the exhaustion diagnostic gets
        # to call it wedged.
        last=wedged
      elif _docket_tree_detached "$git" "$dir"; then
        # Detached with NO operation under way is a human's `git checkout <sha>`, not a window that
        # closes: no retry arm, per this file's own rule of spending budget only on classes that
        # can self-heal. It is terminal rather than merely non-fast-path because the rebase arm
        # below would otherwise move the detached HEAD and report success for it.
        #
        # Detachment gets its OWN arm ahead of the wrong-branch arm below even though a detached
        # HEAD is trivially "not on $branch": the remedies differ (a stranded commit vs. a rewritten
        # branch), and the arm below cannot name the branch the tree is on because there isn't one.
        echo "docket-preflight: metadata sync failed — HEAD in $dir is DETACHED, so no branch is checked out and a commit made there would be stranded off $branch. Nothing here self-heals, so it was not retried. This one needs a human: check out $branch." >&2
        return 1
      elif _docket_tree_on_other_branch "$git" "$dir" "$branch"; then
        # THE WRONG BRANCH IS TERMINAL, and it is refused BEFORE the fast path, not merely before
        # the rebase. Refusing only the rebase would still let a metadata commit land on a branch
        # that is not the one docket syncs and pushes — silently, whenever the remote happened not
        # to have moved. Both hazards are the same misconfiguration and both get one answer.
        #
        # No retry arm: a checked-out branch is a human's state, and this file spends budget only on
        # classes that can self-heal. Nothing is rebased, nothing is committed, nothing is stashed —
        # the tree is left exactly as its owner had it.
        cur="$("$git" -C "$dir" symbolic-ref --short -q HEAD 2>/dev/null)" || cur=""
        echo "docket-preflight: metadata sync failed — $dir is on branch '$cur', not '$branch'. Integrating $remote/$branch here would REWRITE the history of '$cur' onto it, and a metadata commit made here would land on '$cur' rather than '$branch', so nothing was attempted. Nothing here self-heals, so it was not retried. This one needs a human: check out '$branch' in $dir, or point docket's integration_branch at the branch you actually meant." >&2
        return 1
      # FAST PATH — up to date, or ahead only. The remote is an ancestor of HEAD, so there is
      # nothing to integrate and no rebase is needed. This is the single most common collision
      # (the other agent has not pushed yet), and it must succeed on a dirty tree.
      elif [ -n "$head" ] && [ -n "$remote_sha" ] \
         && "$git" -C "$dir" merge-base --is-ancestor "$remote_sha" "$head" 2>/dev/null; then
        return 0
      # From here the remote moved (or history diverged: local commits AND remote movement). Both
      # cases take one path — local commits rebase onto the fetched remote under the same
      # precondition.
      elif _docket_tree_dirty "$git" "$dir"; then
        last=dirty
      elif "$git" -C "$dir" rebase "$remote_sha" >&2; then
        return 0
      elif _docket_tree_wedged "$git" "$dir"; then
        # A content conflict raised by THIS attempt's own rebase is deterministic — it fails
        # identically on every retry — so abort (restoring the pre-attempt state) and fail now,
        # spending no further budget.
        # The abort's OWN stderr stays on stderr: a failed abort leaves the shared tree wedged for
        # the next agent, which is the one thing here a human has to be told about.
        "$git" -C "$dir" rebase --abort >&2 || true
        echo "docket-preflight: metadata sync failed — this attempt's own rebase hit a content conflict in $dir. Deterministic, so it was not retried; the tree was restored. Resolve it by hand." >&2
        return 1
      else
        # An unrecognized rebase failure is TERMINAL, never a retry arm (change 0286's fail-closed
        # doctrine): a loop whose default arm is "try again" is the shape that never terminates,
        # and item 2's own rule is to spend retries only on classes that can self-heal.
        echo "docket-preflight: metadata sync failed — the rebase in $dir failed for an unrecognized reason (git's output is above). Failing closed rather than retrying." >&2
        return 1
      fi
    fi
    # One fewer backoff than attempts, by construction: the last attempt has nothing to wait for.
    nap="${DOCKET_SYNC_BACKOFF[$((attempt - 1))]:-}"
    if [ -n "$nap" ]; then _docket_sync_sleep "$nap"; fi
  done

  case "$last" in
    dirty) echo "docket-preflight: metadata sync failed after $DOCKET_SYNC_ATTEMPTS attempts — the tracked tree in $dir stayed dirty throughout (another agent mid-write, or a human's leftover edit). Retry later, or inspect it." >&2 ;;
    wedged) echo "docket-preflight: metadata sync failed after $DOCKET_SYNC_ATTEMPTS attempts — a rebase or merge is in progress in $dir and never cleared. This one needs a human: finish or abort it." >&2 ;;
    *)     echo "docket-preflight: metadata sync failed after $DOCKET_SYNC_ATTEMPTS attempts — the last failure was fetching $branch from $remote (git's output is above)." >&2 ;;
  esac
  return 1
}

docket_preflight(){
  local scripts_dir="$1"
  local git="${GIT:-git}"
  local cfg
  if [ -n "${CONFIG_EXPORT_CMD:-}" ]; then
    cfg="$($CONFIG_EXPORT_CMD)" \
      || { echo "docket-preflight: config export failed" >&2; return 1; }
  else
    cfg="$("$DOCKET_BASH_PATH" "$scripts_dir"/docket-config.sh --export)" \
      || { echo "docket-preflight: config export failed" >&2; return 1; }
  fi
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  echo "docket-preflight: repo not migrated — run migrate-to-docket.sh" >&2; return 1 ;;
    CREATE_ORPHAN) echo "docket-preflight: fresh repo — bootstrap is opt-in; run docket.sh bootstrap (or a docket skill) to create the docket branch" >&2; return 1 ;;
    *) echo "docket-preflight: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; return 1 ;;
  esac

  # --- repo anchor (change 0075, defect D2) ------------------------------------------------
  # The eval'd SHELL format keeps METADATA_WORKTREE relative (".docket" / "."), and git — plus the
  # -d test below — would resolve that against the CALLER's CWD. Run from <repo>/.docket that
  # created a real <repo>/.docket/.docket worktree and still exited 0 (observed live, change 0073).
  # Anchor the path to the MAIN worktree before anything touches it. Not a git repo => leave the
  # value alone and let the git calls below fail exactly as they did before.
  local root
  root="$(docket_main_worktree)"
  METADATA_WORKTREE="$(docket_anchor_path "${METADATA_WORKTREE:-}")"

  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    local gitc="${root:-.}"
    if [ ! -d "$wt" ]; then
      # Fail-closed guard (change 0075): the metadata worktree must never land INSIDE a LINKED
      # worktree of this repo. The MAIN worktree legitimately contains it (<root>/.docket), so the
      # main worktree — the first entry of `worktree list` — is excluded; every other entry is a
      # linked worktree, and <repo>/.docket/.docket is never a legitimate target. Without this, a
      # caller that hands preflight a bad path silently mints debris that only `git worktree list`
      # reveals.
      if _docket_target_inside_linked_worktree "$git" "$gitc" "$wt"; then
        echo "docket-preflight: refusing to create metadata worktree at $wt — it is inside an existing worktree of this repo" >&2
        return 1
      fi
      "$git" -C "$gitc" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$git" -C "$gitc" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-preflight: cannot create metadata worktree $wt" >&2; return 1; }
    fi
    # change 0063: skip the repo's shared git hooks on the metadata worktree (idempotent;
    # self-heals existing installs). Best-effort — a failure here must not block preflight.
    "$DOCKET_BASH_PATH" "$scripts_dir"/disable-worktree-hooks.sh --worktree "$wt" >&2 \
      || echo "docket-preflight: warning — could not disable hooks on $wt (continuing)" >&2
    _docket_sync_metadata "$git" "$wt" origin "$METADATA_BRANCH" || return 1
  else
    # Main-mode takes the IDENTICAL path (spec, Half 1 item 1: "both branches of the sync function
    # must behave identically here; leaving it implicit is how they drift apart"). Naming the
    # remote and branch explicitly, rather than relying on the checked-out branch's upstream, is
    # part of that: the two branches now differ only in which directory they sync.
    #
    # The BRANCH is INTEGRATION_BRANCH, not METADATA_BRANCH. In main-mode METADATA_BRANCH is the
    # mode keyword "main" — docket-config.sh accepts only 'docket' or 'main' for it — and a repo
    # whose integration branch is named anything else (master, trunk) would have had this fetch a
    # ref that does not exist. docket-status.sh's health_checks already resolves the pair this way;
    # this is the same rule, not a second notion of it.
    #
    # An absent INTEGRATION_BRANCH therefore fails CLOSED rather than falling back to
    # METADATA_BRANCH. That fallback looks harmless only because the mode keyword and the commonest
    # branch name are spelled the same: on a repo whose integration branch is 'trunk' it resolved to
    # the literal string "main" and spent the entire retry budget on `fetch origin main`, under an
    # exhaustion diagnostic blaming the network for a config defect. A wrong ref that usually works
    # is worse than no ref at all. docket-config.sh always exports this field (resolving 'auto' to
    # the detected default branch), so an empty one means the config export itself is broken.
    if [ -z "${INTEGRATION_BRANCH:-}" ]; then
      echo "docket-preflight: main-mode metadata sync cannot start — config exported no INTEGRATION_BRANCH, and METADATA_BRANCH is the mode keyword 'main' here, not a branch name, so it is not a usable fallback. Set integration_branch in .docket.yml (or leave it 'auto' and fix why docket-config.sh could not resolve the default branch)." >&2
      return 1
    fi
    _docket_sync_metadata "$git" "${root:-.}" origin "$INTEGRATION_BRANCH" || return 1
  fi
}

# _docket_target_inside_linked_worktree <git> <repo-dir> <target> — true (0) when <target> lies at
# or inside a LINKED worktree of the repo at <repo-dir>. The MAIN worktree (the first entry of
# `git worktree list --porcelain`) is deliberately EXCLUDED: it is the one worktree that
# legitimately contains the metadata worktree. Every other entry is a linked worktree, and a
# metadata worktree inside one of those is the D2 shape.
_docket_target_inside_linked_worktree(){
  local git="$1" repo_dir="$2" target="$3" first=1 wt
  while IFS= read -r wt; do
    [ -n "$wt" ] || continue
    if [ "$first" = 1 ]; then first=0; continue; fi   # skip the main worktree
    case "$target/" in
      "$wt/"*) return 0 ;;
    esac
  done < <("$git" -C "$repo_dir" worktree list --porcelain 2>/dev/null | sed -n 's/^worktree //p')
  return 1
}
