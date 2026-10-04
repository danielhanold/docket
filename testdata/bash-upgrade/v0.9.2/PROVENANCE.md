# Provenance — saved Bash docket v0.9.2 install and `docket`-branch repository

This directory is a saved upgrade case: the state a user had after installing Bash docket from the
`v0.9.2` tag and running one repository through its `docket`-branch flows. It was made once, by
hand, in a throwaway sandbox. The tag is the generator; no generator script is committed. The
scratch helpers used are reproduced verbatim below.

**Temporary.** This case and the test that reads it are retired when stable v1.0.0 ships.

## Source

| Item | Value |
|---|---|
| Tag | `v0.9.2` (annotated tag object `4e199cd0f65de048a248cf0f2f17a60ebc3e217f`) |
| Commit | `ca8251e8bc0a7944ece39dd750b1b159c978f2b3` |
| `install.sh` SHA-256 | `18d46c0489c638827ef56ab85ac39179acb263f2b2ee83a5c0ae72aaae785c71` |
| Date made | 2026-10-04 (UTC; sandbox work began 2026-10-04T18:10:19Z) |
| Host | macOS 27.0 (build 26A428), Darwin 27.0.0 arm64 |
| Bash | GNU bash 5.3.20(1)-release (aarch64-apple-darwin27.0.0), `/opt/homebrew/bin/bash` |
| Git / Python | git 2.55.0 / Python 3.14.7 (Python only for the `home.tar` helper) |

## Files

| File | What it is |
|---|---|
| `origin.bundle` | `git bundle create … --all` of the sandbox bare `origin`: `refs/heads/main` (`aea93f28d201f2e49289c3a605de1bd85739a923`), `refs/heads/docket` (`d0ea17ba29702c438b21fe17b559187e8466fc98`) and `HEAD` → `main`, complete history |
| `home.tar` | USTAR tar rooted at the sandbox `HOME`; 197 members, 48 symlinks; mtimes 0, uid/gid 0; `@@SANDBOX_HOME@@` stands for the home path in link targets and file bodies |
| `clone-config.txt` | the clone actions the restore replays (one `worktree`, one `config`, one `hooks-off`) |
| `records.txt` | the 9 records (changes, ADR, specs, learning) the case holds, all on `docket` |

## Sandbox

The sandbox root `$SB` was
`/private/tmp/claude-501/-Users-homer-dev-docket/9cbfa2e1-e5f8-4f8a-9555-3ec1f4e089eb/scratchpad/bash-upgrade-v092.apc2fV`
(a session scratchpad, removed afterwards). `HOME` was `$SB/home`. Every command below ran through
the wrapper `$SB/sbx`, written by Step 1; it clears the environment and sets exactly:

```sh
#!/bin/sh
exec env -i HOME="$SB/home" USER=fixture LOGNAME=fixture SHELL=/bin/zsh TERM=dumb LANG=en_US.UTF-8 \
  XDG_CONFIG_HOME="$SB/home/.config" XDG_DATA_HOME="$SB/home/.local/share" \
  XDG_CACHE_HOME="$SB/home/.cache" XDG_STATE_HOME="$SB/home/.local/state" \
  XDG_BIN_HOME="$SB/home/.local/bin" TMPDIR="$SB/tmp" \
  GIT_CONFIG_GLOBAL="$SB/gitconfig" GIT_CONFIG_NOSYSTEM=1 \
  PATH="$SB/stub-bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin" "$@"
```

`/opt/homebrew/bin` holds no `docket`, so it stayed on `PATH`; Bash 4+ was still always named by
its absolute path. A fail-loud `gh` stub sat first on `PATH`; it logs to `$SB/gh-calls.log` and
exits 97. That log never existed at any check, so nothing called `gh`. The real `~/.claude`,
`~/.config/docket`, `~/.zshenv` and `~/dev/docket` were checked with
`find … -newer "$SB/task-start.txt"` after the install and showed nothing newer.

## Commands, in order

`$SCRATCH` is the session scratchpad that contains `$SB`.

### Step 1 — sandbox

```bash
SB="$(mktemp -d "$SCRATCH/bash-upgrade-v092.XXXXXX")"
mkdir -p "$SB/home" "$SB/stub-bin" "$SB/home/.config" "$SB/home/.local/share" "$SB/home/.cache" "$SB/home/.local/state"
printf '#!/bin/sh\necho "gh stub: network is forbidden in the sandbox: $*" >&2\necho "$*" >> "%s/gh-calls.log"\nexit 97\n' "$SB" > "$SB/stub-bin/gh"
chmod +x "$SB/stub-bin/gh"
printf '[user]\n\tname = Bash Upgrade Fixture\n\temail = fixture@docket.invalid\n[init]\n\tdefaultBranch = main\n' > "$SB/gitconfig"
cat > "$SB/sbx" <<EOF
#!/bin/sh
exec env -i HOME="$SB/home" USER=fixture LOGNAME=fixture SHELL=/bin/zsh TERM=dumb LANG=en_US.UTF-8 \\
  XDG_CONFIG_HOME="$SB/home/.config" XDG_DATA_HOME="$SB/home/.local/share" \\
  XDG_CACHE_HOME="$SB/home/.cache" XDG_STATE_HOME="$SB/home/.local/state" \\
  XDG_BIN_HOME="$SB/home/.local/bin" TMPDIR="$SB/tmp" \\
  GIT_CONFIG_GLOBAL="$SB/gitconfig" GIT_CONFIG_NOSYSTEM=1 \\
  PATH="$SB/stub-bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin" "\$@"
EOF
chmod +x "$SB/sbx"; mkdir -p "$SB/tmp"
"$SB/sbx" sh -c 'echo "HOME=$HOME"; command -v docket || echo "no docket on PATH"; env | grep -c -e "^DOCKET_" || true'
# -> HOME=$SB/home / no docket on PATH / 0
```

### Step 2 — the tag, checked out into the sandbox home

```bash
"$SB/sbx" git clone --quiet --branch v0.9.2 --depth 1 file:///Users/homer/dev/docket "$SB/home/dev/docket"
"$SB/sbx" git -C "$SB/home/dev/docket" remote remove origin
git -C /Users/homer/dev/docket rev-parse 'v0.9.2^{commit}'     # ca8251e8bc0a7944ece39dd750b1b159c978f2b3
"$SB/sbx" git -C "$SB/home/dev/docket" rev-parse HEAD          # ca8251e8bc0a7944ece39dd750b1b159c978f2b3
shasum -a 256 "$SB/home/dev/docket/install.sh"                 # 18d46c04…785c71
```

`~/dev/docket` is the tag's default checkout location, so the skill links look like a real user's.

### Step 3 — harness folders and the tag's installer

`link-skills.sh` (`HARNESS_SKILL_DIRS`) targets `.claude`, `.codex`, `.cursor`, `.agents`, `.kiro`
and `.windsurf` `skills/`; `sync-agents.sh` writes `.<token>/agents` for every token in
`DOCKET_GI_HARNESS_TOKENS` (`claude codex cursor opencode agents kiro windsurf`) whose parent
exists. The tag's README names `~/.agents/skills` as where OpenCode reads skills and
`.opencode/agents/` as its agent root. So the parents for Claude Code, Cursor, Codex and OpenCode
were created, and `.kiro` and `.windsurf` were not:

```bash
date -u +%Y-%m-%dT%H:%M:%SZ > "$SB/task-start.txt"     # 2026-10-04T18:10:19Z
mkdir -p "$SB/home/.claude" "$SB/home/.cursor" "$SB/home/.codex" "$SB/home/.opencode" "$SB/home/.agents"
"$SB/sbx" sh -c 'cd "$HOME/dev/docket" && /opt/homebrew/bin/bash ./install.sh' > "$SB/install.log" 2>&1   # exit 0
find ~/.claude/agents ~/.claude/skills ~/.claude/settings.json ~/.claude/CLAUDE.md ~/.zshenv ~/.config/docket -newer "$SB/task-start.txt"   # (no output)
```

One attempt; it ended `docket: install complete`. Its output, apart from the 48 `linked …` lines:

```text
==> ensure-global-config.sh (configure Bash runtime)
docket: wrote $SB/home/.config/docket/config.yml (pointer config plus managed runtime.bash)
==> link-skills.sh (install skills)

Created: 48   Skipped (already present): 0
==> sync-agents.sh (generate agent wrappers)
sync-agents: WARN agents/docket-<name>: no harness-specific model — generated unpinned; harness 'agents' will apply its own default. … (once per agent, 16 lines)
sync-agents: WARN harness 'agents' has no named emitter — its wrappers are Claude-shaped and unverified for 'agents', … (ADR-0060). …
sync-agents: done
==> ensure-docket-env.sh (export Docket runtime environment)
ensure-docket-env: wrote DOCKET_SCRIPTS_DIR and DOCKET_BASH_PATH -> $SB/home/.zshenv (zsh)
ensure-docket-env: set env.DOCKET_SCRIPTS_DIR and env.DOCKET_BASH_PATH -> .claude/settings.json
docket: install complete
```

### Step 4 — consuming repository and the empty-orphan bootstrap

```bash
"$SB/sbx" git init --quiet --bare -b main "$SB/origin.git"
"$SB/sbx" git clone --quiet "$SB/origin.git" "$SB/home/dev/sample"
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && printf "# sample\n" > README.md && git add README.md && git commit -qm "initial" && git push -q origin main && git remote set-head origin main'
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && export DOCKET_SCRIPTS_DIR="$HOME/dev/docket/scripts" DOCKET_BASH_PATH=/opt/homebrew/bin/bash && "$DOCKET_BASH_PATH" "$DOCKET_SCRIPTS_DIR/docket-config.sh" --export --bootstrap' > "$SB/bootstrap.out" 2> "$SB/bootstrap.err"   # exit 0, BOOTSTRAP=PROCEED
"$SB/sbx" git -C "$SB/home/dev/sample" rev-parse 'origin/docket^{tree}'     # 4b825dc642cb6eb9a060e54bf8d69288fbee4904
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && git add .gitignore && git commit -qm "chore: commit docket managed .gitignore block" && git push -q origin main'
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && export DOCKET_SCRIPTS_DIR="$HOME/dev/docket/scripts" DOCKET_BASH_PATH=/opt/homebrew/bin/bash && "$DOCKET_BASH_PATH" "$DOCKET_SCRIPTS_DIR/docket.sh" preflight' > "$SB/preflight.out" 2> "$SB/preflight.err"   # exit 0
```

- **Bootstrap kind:** the empty-orphan `CREATE_ORPHAN` cell. `docket-config.sh --bootstrap` made a
  root commit `c1ea581` (`docket: initialize empty orphan metadata branch`) on the empty tree,
  pushed it straight to `origin/docket`, and seeded the managed `.gitignore` block with a
  `COMMIT THIS` notice. That block was committed on `main` as above.
- **`.docket.yml`:** the fresh-repository path needs none (`metadata_branch` defaults to `docket`),
  so none was committed. `main` has no `.docket.yml`.
- **`.docket` worktree command:** the tag's skills create it through `docket.sh preflight`
  (`scripts/lib/docket-preflight.sh` `docket_preflight`), which ran
  `git -C <repo> worktree add .docket docket` (git's DWIM made a local `docket` tracking
  `origin/docket`). Its fallback, `git worktree add .docket origin/docket`, was not needed. It then
  ran `disable-worktree-hooks.sh --worktree .docket`, which set `extensions.worktreeConfig=true` and
  the worktree-scoped `core.hooksPath=<clone>/.git/docket/empty-hooks`.

### Step 5 — populate through the tag's flows

```bash
mkdir -p "$SB/snap-pre-populate" && cp -Rp "$SB/origin.git" "$SB/home/dev/sample" "$SB/snap-pre-populate/"
"$SB/sbx" /opt/homebrew/bin/bash "$SCRATCH/populate-v092.sh" > "$SB/populate.log" 2>&1
```

Three runs; each failed one was reset from the snapshot
(`rm -rf "$SB/origin.git" "$SB/home/dev/sample" && cp -Rp "$SB/snap-pre-populate/origin.git" "$SB/" && cp -Rp "$SB/snap-pre-populate/sample" "$SB/home/dev/"`)
so the saved history holds only the third run:

1. **Failed (exit 1)** in change 1's archive step. The merge date came from
   `TZ=UTC git show -s --date=format-local:%Y-%m-%d HEAD`, the form the tag's
   `skills/docket-convention/references/terminal-close-out.md` step 1 prints. Without `--format`
   that prints the whole commit header, so `archive-change.sh --date` got a multi-line value and
   `git mv` failed. Fix: add `--format=%ad`.
2. **Failed (exit 1)** in change 2's close-out. Re-rendering the archived file's `## Artifacts`
   block changed no bytes (a trivial change with no spec), and the follow-on `git commit` refused
   an empty commit. Fix: make that follow-on commit only when the file changed.
3. **Succeeded (exit 0)**, with no `gh` call. The script below is that final version.

Tag scripts used, all as subprocesses through `docket.sh` (counts from the final log):
`preflight` ×7, `docket-status --board-only [--must-land]` ×11, `render-change-links` ×9,
`render-artifact-backlink` ×7, `render-adr-index` ×1, `render-learnings-index` ×1, `mint-stub` ×1,
`archive-change` ×2, `terminal-publish --enabled false` ×2 (each logged `terminal_publish: false —
skipping publish onto main; the record stays on docket`) and `cleanup-feature-branch` ×2.

`$SCRATCH/populate-v092.sh`, final version, verbatim:

```bash
#!/opt/homebrew/bin/bash
# populate-v092.sh — scratch driver (never committed) that populates the v0.9.2 sample repository
# through the tag's own scripts. Run ONLY through the sandbox wrapper:
#   "$SB/sbx" /opt/homebrew/bin/bash "$SCRATCH/populate-v092.sh"
# It never sources a tag script; every tag script runs as a subprocess via docket.sh.
set -euo pipefail

export DOCKET_SCRIPTS_DIR="$HOME/dev/docket/scripts"
export DOCKET_BASH_PATH=/opt/homebrew/bin/bash
R="$HOME/dev/sample"
M="$R/.docket"
C="$M/docs/changes"
A="$M/docs/adrs"
SPECS="$M/docs/superpowers/specs"
TODAY="$(date -u +%F)"
cd "$R"

dk(){ echo "+ docket.sh $*" >&2; "$DOCKET_BASH_PATH" "$DOCKET_SCRIPTS_DIR/docket.sh" "$@"; }
ghcheck(){ if [ -e "$HOME/../gh-calls.log" ]; then echo "FATAL: gh was called:" >&2; cat "$HOME/../gh-calls.log" >&2; exit 97; fi; }
# metadata commit: explicit paths only, then push origin/docket
mcommit(){ local msg="$1"; shift; git -C "$M" add -- "$@"; git -C "$M" commit -q -m "$msg" -- "$@"; git -C "$M" push -q origin docket; ghcheck; }
board(){ dk docket-status --board-only --must-land; ghcheck; }
now(){ date -u +%Y-%m-%dT%H:%M:%SZ; }
# fm_set FILE KEY VALUE — set a frontmatter field inside the FIRST ---…--- block only
# (append before the closing --- when absent).
fm_set(){
  local f="$1" k="$2" v="$3" tmp
  tmp="$(mktemp "$(dirname "$f")/.fmset.XXXXXX")"
  awk -v k="$k" -v v="$v" '
    BEGIN{n=0; done=0}
    /^---[[:space:]]*$/ { n++; if (n==2 && !done) { print k ": " v; done=1 } print; next }
    n==1 && !done && index($0, k ":")==1 { print k ": " v; done=1; next }
    { print }
  ' "$f" > "$tmp"
  mv -f "$tmp" "$f"
}
change_links(){ dk render-change-links --change-file "$1" --adrs-dir "$A"; }

dk preflight >/dev/null
mkdir -p "$C/active" "$SPECS" "$A"

# ---------------------------------------------------------------------------------------------
# 0001 — brainstorm-mode change with a spec, later built, merged and closed out as done.
# docket-new-change Brainstorm mode (steps 1, 4, 5); spec + change body hand-authored.
# ---------------------------------------------------------------------------------------------
S1="docs/superpowers/specs/$TODAY-add-a-greeting-script-design.md"
cat > "$M/$S1" <<'EOF'
# Add a greeting script — design

## Purpose

Give the sample repository a tiny executable so later changes have code to build on.

## Design

A POSIX `sh` script, `hello.sh`, prints `hello, <name>` for its first argument and `hello, world`
when called with none. No dependencies.

## Testing

`sh hello.sh` prints `hello, world`; `sh hello.sh docket` prints `hello, docket`.
EOF
F1="$C/active/0001-add-a-greeting-script.md"
cat > "$F1" <<EOF
---
id: 1
slug: add-a-greeting-script
title: Add a greeting script
status: proposed
priority: medium
type: feat
created: $TODAY
updated: $TODAY
depends_on: []
stacked_on:
related: []
discovered_from: []
adrs: []
spec: $S1
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The sample repository has no code yet. A small greeting script gives later changes something to
build on and proves the build loop end to end.

## What changes

Add \`hello.sh\`, which greets its first argument or the world.

## Out of scope

Localisation and any configuration file.

## Reconcile log
EOF
change_links "$F1"
dk render-artifact-backlink --artifact-file "$M/$S1" --change-file "$F1"
mcommit "docket(0001): new change — add a greeting script (spec)" "docs/changes/active/0001-add-a-greeting-script.md" "$S1"
board

# ---------------------------------------------------------------------------------------------
# 0002 — trivial-mode change, later built, merged and closed out as done.
# ---------------------------------------------------------------------------------------------
F2="$C/active/0002-document-the-greeting-script.md"
cat > "$F2" <<EOF
---
id: 2
slug: document-the-greeting-script
title: Document the greeting script
status: proposed
priority: low
type: docs
created: $TODAY
updated: $TODAY
depends_on: [1]
stacked_on:
related: []
discovered_from: []
adrs: []
plan:
results:
trivial: true
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The README does not say how to run the greeting script.

## What changes

Add a short *Usage* section to \`README.md\`.

## Out of scope

Any change to the script itself.

## Reconcile log
EOF
mcommit "docket(0002): new change — document the greeting script (trivial)" "docs/changes/active/0002-document-the-greeting-script.md"
board

# ---------------------------------------------------------------------------------------------
# Build + merge + close-out helper for a done change (docket-implement-next steps 2-7, then a
# local merge standing in for the GitHub PR merge, then docket-finalize-change's close-out).
# ---------------------------------------------------------------------------------------------
build_and_close(){
  local id="$1" slug="$2" pr="$3" codefile="$4" codebody="$5" adr="$6"
  local pad; pad="$(printf '%04d' "$id")"
  local F="$C/active/$pad-$slug.md" rel="docs/changes/active/$pad-$slug.md"
  local W="$R/.worktrees/$slug"
  local plan="docs/superpowers/plans/$TODAY-$slug.md"
  local results="docs/results/$TODAY-$pad-$slug.md"

  # Step 2 — claim
  dk preflight >/dev/null
  fm_set "$F" status in-progress
  fm_set "$F" branch "feat/$slug"
  fm_set "$F" updated "$TODAY"
  fm_set "$F" claimed_at "$(now)"
  mcommit "docket($pad): claim — in-progress on feat/$slug" "$rel"
  dk docket-status --board-only || echo "best-effort board pass failed (continuing)" >&2; ghcheck

  # Step 3 — reconcile (no drift): reconciled: true + a dated reconcile-log entry
  fm_set "$F" reconciled true
  fm_set "$F" claimed_at "$(now)"
  printf '\n- %s — reconciled against main: no drift.\n' "$TODAY" >> "$F"
  mcommit "docket($pad): reconcile — no drift" "$rel"

  # Step 4 — feature worktree + plan (plan back-link stamped on the feature branch)
  git fetch -q origin
  git worktree add -q "$W" -b "feat/$slug" origin/main
  mkdir -p "$W/docs/superpowers/plans" "$W/docs/results"
  printf '# %s Implementation Plan\n\n### Task 1\n\n- [ ] Write `%s`.\n' "$(sed -n 's/^title: //p' "$F" | head -n 1)" "$codefile" > "$W/$plan"
  dk render-artifact-backlink --artifact-file "$W/$plan" --change-file "$F"
  git -C "$W" add -- "$plan"
  git -C "$W" commit -q -m "docs(plan): $slug implementation plan" -- "$plan"
  fm_set "$F" plan "$plan"
  fm_set "$F" claimed_at "$(now)"
  change_links "$F"
  mcommit "docket($pad): plan" "$rel"

  # Step 5 — build
  printf '%s' "$codebody" > "$W/$codefile"
  git -C "$W" add -- "$codefile"
  git -C "$W" commit -q -m "$( [ "$codefile" = hello.sh ] && echo feat || echo docs ): $slug" -- "$codefile"

  # Step 6 — review; optional ADR via the docket-adr flow (ADR commit, then separate index commit)
  if [ -n "$adr" ]; then
    local adrfile="docs/adrs/0001-greeting-script-is-posix-sh.md"
    cat > "$M/$adrfile" <<EOF
---
id: 1
slug: greeting-script-is-posix-sh
title: The greeting script is POSIX sh
status: Accepted
date: $TODAY
supersedes: []
reverses: []
relates_to: []
change: $id
---

## Context

The sample repository has no language runtime of its own, and the script must run anywhere.

## Decision

Write \`hello.sh\` in POSIX \`sh\`, with no Bash-only syntax.

## Consequences

It runs on any Unix-like machine. Bash conveniences such as arrays are unavailable.
EOF
    mcommit "docket(adr-0001): the greeting script is POSIX sh" "$adrfile"
    dk render-adr-index --adrs-dir "$A" > "$A/README.md"
    mcommit "docket(adr): regenerate ADR index" "docs/adrs/README.md"
    fm_set "$F" adrs "[1]"
    fm_set "$F" claimed_at "$(now)"
    change_links "$F"
    mcommit "docket($pad): adrs [1]" "$rel"
  fi

  # Step 6.5 — results file on the feature branch, with its back-link
  printf '# Results — %s\n\nBuilt as planned. Manual check: the change does what its plan says.\n' "$slug" > "$W/$results"
  dk render-artifact-backlink --artifact-file "$W/$results" --change-file "$F"
  git -C "$W" add -- "$results"
  git -C "$W" commit -q -m "docs(results): $slug" -- "$results"
  git -C "$W" push -q origin "feat/$slug"
  ghcheck

  # Step 7 — implemented + pr: (+ results:) — PR opening itself needs gh (stubbed out: gap)
  fm_set "$F" status implemented
  fm_set "$F" pr "$pr"
  fm_set "$F" results "$results"
  fm_set "$F" claimed_at "$(now)"
  change_links "$F"
  mcommit "docket($pad): implemented — PR open" "$rel"
  dk docket-status --board-only || echo "best-effort board pass failed (continuing)" >&2; ghcheck

  # Merge: a local --no-ff merge of feat/<slug> into main stands in for GitHub's PR merge.
  git checkout -q main
  git pull -q --ff-only origin main
  git merge -q --no-ff "feat/$slug" -m "Merge pull request #${pr##*/} from fixture/feat/$slug"
  git push -q origin main
  ghcheck
  local mdate; mdate="$(TZ=UTC git show -s --format=%ad --date=format-local:%Y-%m-%d HEAD)"

  # Finalize close-out (terminal-close-out.md): harvest -> archive -> re-render -> publish -> cleanup -> board
  dk preflight >/dev/null
  if [ -n "$adr" ]; then
    mkdir -p "$C/learnings"
    cat > "$C/learnings/posix-sh-means-no-arrays.md" <<EOF
---
slug: posix-sh-means-no-arrays
hook: "Choosing POSIX sh trades away arrays: plan argument handling around \"\$@\"."
topics: [shell, portability]
changes: [$id]
created: $mdate
updated: $mdate
promotion_state: retained
promoted_to:
---

## Apply
When a script is pinned to POSIX \`sh\`, handle lists with \`"\$@"\` and \`set --\`, never arrays.

## War story
- $mdate (#$id, PR #${pr##*/}) — the review caught a Bash array in the first draft of \`hello.sh\`.
EOF
    local tmp; tmp="$(mktemp "$C/learnings/.render-index.XXXXXX")"
    dk render-learnings-index --learnings-dir "$C/learnings" > "$tmp" && [ -s "$tmp" ] && mv -f "$tmp" "$C/learnings/README.md" || rm -f "$tmp"
    mcommit "docket($pad): harvest learnings" "docs/changes/learnings/posix-sh-means-no-arrays.md" "docs/changes/learnings/README.md"
  fi
  dk archive-change --changes-dir "$C" --id "$id" --outcome done --date "$mdate" --results "$results" \
    --message "docket($pad): done — archived (status done, $mdate)"
  ghcheck
  local AF="$C/archive/$mdate-$pad-$slug.md" arel="docs/changes/archive/$mdate-$pad-$slug.md"
  change_links "$AF"
  local spec; spec="$(awk '/^---[[:space:]]*$/{n++; next} n==1 && /^spec:/{sub(/^spec:[[:space:]]*/, ""); print; exit}' "$AF")"
  if [ -n "$spec" ]; then
    dk render-artifact-backlink --artifact-file "$M/$spec" --change-file "$AF"
    mcommit "docket($pad): re-render artifacts after archive" "$arel" "$spec"
  elif [ -n "$(git -C "$M" status --porcelain -- "$arel")" ]; then
    mcommit "docket($pad): re-render artifacts after archive" "$arel"
  else
    echo "re-render after archive: block unchanged for $pad — no follow-on commit" >&2
  fi
  dk terminal-publish --id "$id" --outcome done --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false --metadata-worktree "$M"
  ghcheck
  dk cleanup-feature-branch --slug "$slug"
  ghcheck
  board
}

build_and_close 1 add-a-greeting-script "https://github.com/fixture/sample/pull/1" hello.sh \
  '#!/bin/sh
# hello.sh — greet the first argument, or the world.
printf "hello, %s\n" "${1:-world}"
' adr

build_and_close 2 document-the-greeting-script "https://github.com/fixture/sample/pull/2" USAGE.md \
  '# Usage

Run `sh hello.sh [name]`. With no name it greets the world.
' ""

# ---------------------------------------------------------------------------------------------
# 0003 — proposed change with a spec (docket-new-change Brainstorm mode).
# ---------------------------------------------------------------------------------------------
dk preflight >/dev/null
S3="docs/superpowers/specs/$TODAY-add-a-farewell-script-design.md"
cat > "$M/$S3" <<'EOF'
# Add a farewell script — design

## Purpose

Pair the greeting script with a farewell, so the sample has two commands.

## Design

`bye.sh` prints `goodbye, <name>`, defaulting to `goodbye, world`. It reuses the greeting script's
POSIX `sh` rule (ADR-0001).

## Testing

`sh bye.sh` prints `goodbye, world`.
EOF
F3="$C/active/0003-add-a-farewell-script.md"
cat > "$F3" <<EOF
---
id: 3
slug: add-a-farewell-script
title: Add a farewell script
status: proposed
priority: high
type: feat
created: $TODAY
updated: $TODAY
depends_on: []
stacked_on:
related: [1]
discovered_from: []
adrs: []
spec: $S3
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

A greeting without a farewell is half a conversation.

## What changes

Add \`bye.sh\`, the counterpart of \`hello.sh\`.

## Out of scope

Merging the two scripts into one.

## Reconcile log
EOF
change_links "$F3"
dk render-artifact-backlink --artifact-file "$M/$S3" --change-file "$F3"
mcommit "docket(0003): new change — add a farewell script (spec)" "docs/changes/active/0003-add-a-farewell-script.md" "$S3"
board

# ---------------------------------------------------------------------------------------------
# 0004 — trivial proposed change (docket-new-change Trivial mode).
# ---------------------------------------------------------------------------------------------
F4="$C/active/0004-add-a-license-file.md"
cat > "$F4" <<EOF
---
id: 4
slug: add-a-license-file
title: Add a license file
status: proposed
priority: medium
type: chore
created: $TODAY
updated: $TODAY
depends_on: []
stacked_on:
related: []
discovered_from: []
adrs: []
plan:
results:
trivial: true
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The sample repository has no license.

## What changes

Add an MIT \`LICENSE\` file at the root.

## Out of scope

Per-file license headers.

## Reconcile log
EOF
mcommit "docket(0004): new change — add a license file (trivial)" "docs/changes/active/0004-add-a-license-file.md"
board

# ---------------------------------------------------------------------------------------------
# 0005 — a discovered-work stub minted by the tag's mint-stub.sh, then deferred through the
# docket-groom-next Defer exit (status: deferred + ## Why deferred + updated:).
# ---------------------------------------------------------------------------------------------
BODY="$(mktemp "$TMPDIR/stub-body.XXXXXX")"
cat > "$BODY" <<'EOF'
## Why

The greeting script could read a default name from an environment variable, which the review of
change 1 suggested but did not need.

## What changes

Read `HELLO_NAME` when no argument is given.

## Open questions

- Should an empty `HELLO_NAME` count as unset?
EOF
dk mint-stub --changes-dir "$C" --title "Read the default greeting name from the environment" --type feat \
  --body-file "$BODY" --discovered-from 1 --metadata-branch docket
ghcheck
F5="$(ls "$C"/active/0005-*.md)"
rel5="docs/changes/active/$(basename "$F5")"
dk preflight >/dev/null
fm_set "$F5" status deferred
fm_set "$F5" updated "$TODAY"
printf '\n## Why deferred\n\nRight idea, wrong time: nobody has asked for it yet. Revisit when a caller needs it.\n' >> "$F5"
mcommit "docket(0005): defer — right idea, wrong time" "$rel5"
board

echo "populate-v092: done"
```

### Step 6 — resolved configuration (read-only)

```bash
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$HOME/dev/docket/scripts/docket-config.sh" --export' > "$SB/config-export.txt"   # exit 0
```

No config edits were made. `config-export.txt`:

```text
DOCKET_MODE=docket
DEFAULT_BRANCH=main
METADATA_BRANCH=docket
INTEGRATION_BRANCH=main
METADATA_WORKTREE=.docket
DOCKET_BASH_PATH=/opt/homebrew/bin/bash
CHANGES_DIR=docs/changes
ADRS_DIR=docs/adrs
RESULTS_DIR=docs/results
FINALIZE_GATE=local
FINALIZE_TEST_COMMAND=''
FINALIZE_REQUIRE_PR_APPROVAL=false
FINALIZE_SKIP_RESULTS_ONLY_DELTA=false
LEARNINGS_ENABLED=true
LEARNINGS_CAP=300
BOARD_SURFACES=inline
AUTO_GROOM=false
CHANGE_TYPES=chore\ docs\ feat\ fix\ refactor\ perf
AUTO_CAPTURE_ENABLED=false
AUTO_CAPTURE_TYPES=all
TERMINAL_PUBLISH=false
RECLAIM_LEASE_TTL=72
RECLAIM_AUTO=false
BUILD_CHECKPOINT=false
REVIEW_MIN_FIX_SEVERITY=minor
REVIEW_MAX_FIX_TASKS=10
GATE_OBSERVATION_BUDGET=30
DELEGATION_OBSERVATION_BUDGET=60
SKILL_BRAINSTORM=superpowers:brainstorming
SKILL_PLAN=superpowers:writing-plans
SKILL_BUILD=docket-build
SKILL_REVIEW=docket-review
SKILL_FINISH=superpowers:finishing-a-development-branch
DUMMY_MODE_ENABLED=false
DUMMY_MODE_PERSONA=A\ mid-level\ software\ engineer:\ solid\ grasp\ of\ software\ architecture\ and\ general\ engineering\ concepts\ —\ APIs\,\ testing\,\ CI\,\ version\ control\ —\ but\ only\ working-level\ fluency\ in\ any\ specific\ programming\ language\,\ so\ avoid\ language-specific\ idioms\ unless\ glossed.\ Assume\ no\ familiarity\ with\ this\ project\'s\ internal\ vocabulary\;\ introduce\ each\ project-specific\ term\ with\ a\ one-clause\ explanation.
DUMMY_MODE_SURFACES=all
BOOTSTRAP=PROCEED
```

### Step 7 — export

```bash
OUT=<worktree>/testdata/bash-upgrade/v0.9.2
"$SB/sbx" git -C "$SB/origin.git" bundle create "$OUT/origin.bundle" --all
git -C "$SB/origin.git" bundle verify "$OUT/origin.bundle"     # okay; refs/heads/docket, refs/heads/main, HEAD; complete history
{ git -C "$SB/origin.git" ls-tree -r --name-only docket | sed 's/^/docket:/'; git -C "$SB/origin.git" ls-tree -r --name-only main | sed 's/^/main:/'; } > "$SB/all-paths.txt"
grep -E -e '^(docket|main):docs/changes/(active|archive)/[^/]+\.md$' -e '^(docket|main):docs/adrs/[0-9][^/]*\.md$' -e '^(docket|main):docs/superpowers/specs/[^/]+\.md$' -e '^(docket|main):docs/changes/learnings/[^/]+\.md$' "$SB/all-paths.txt" | grep -v -e '/README\.md$' | LC_ALL=C sort > "$OUT/records.txt"
rm -rf "$SB/freshclone"; "$SB/sbx" git clone -q "$SB/origin.git" "$SB/freshclone"
"$SB/sbx" git -C "$SB/freshclone" config --local --list | sort > "$SB/cfg-fresh.txt"
"$SB/sbx" git -C "$SB/home/dev/sample" config --local --list | sort > "$SB/cfg-sample.txt"
diff "$SB/cfg-fresh.txt" "$SB/cfg-sample.txt"
#   > branch.docket.merge=refs/heads/docket
#   > branch.docket.remote=origin
#   > extensions.worktreeconfig=true
"$SB/sbx" git -C "$SB/home/dev/sample/.docket" config --worktree --list
#   core.hookspath=$SB/home/dev/sample/.git/docket/empty-hooks
python3 "$SCRATCH/make-home-tar.py" "$SB/home" "$OUT/home.tar"          # members: 197
tar -xOf "$OUT/home.tar" | grep -c -e "$SB" || true                      # 0
tar -xOf "$OUT/home.tar" | grep -c -e "/tmp/claude-501" || true          # 0 (the /tmp spelling of the same path)
tar -tvf "$OUT/home.tar" | grep -c -e '^l'                               # 48
```

`clone-config.txt` keeps `worktree .docket docket` and `config extensions.worktreeConfig true`, in
the order the tag made them. `branch.docket.*` comes from the worktree add itself. The
worktree-scoped `core.hooksPath` cannot be written as a `config` line (that is
`git config --local`), so it is the third line, `hooks-off .docket`: the restore creates
`<git-common-dir>/docket/empty-hooks` and sets the `.docket` worktree's `core.hooksPath` to it, as
the tag's `disable-worktree-hooks.sh` did. (This line was added with the restore harness; without
it the restored `.docket` worktree had hooks enabled, a state the tag never leaves.)

`$SCRATCH/make-home-tar.py`, verbatim:

```python
#!/usr/bin/env python3
"""make-home-tar.py SANDBOX_HOME OUT_TAR — scratch helper (never committed).

Writes an uncompressed USTAR tar rooted at SANDBOX_HOME holding only the harness folders, the
docket skills tree the skill links point at, and the shell profile the tag's ensure-docket-env.sh
wrote. Every absolute sandbox-home prefix (and its realpath / /tmp spelling) in a symlink target or
regular-file body becomes @@SANDBOX_HOME@@. mtimes are zeroed, uid/gid 0, no user/group names.
"""
import io
import os
import sys
import tarfile

home = sys.argv[1].rstrip("/")
out = sys.argv[2]
TOKEN = "@@SANDBOX_HOME@@"

INCLUDE = [".claude", ".cursor", ".codex", ".agents", ".opencode", ".config/docket",
           "dev/docket/skills", ".zshenv"]

real = os.path.realpath(home)
spellings = {home, real}
for s in list(spellings):
    if s.startswith("/private/"):
        spellings.add(s[len("/private"):])
# longest first so "/private/tmp/x" is rewritten before "/tmp/x" can match inside it
spellings = sorted(spellings, key=len, reverse=True)


def rewrite(text):
    for s in spellings:
        text = text.replace(s, TOKEN)
    return text


def norm(ti):
    ti.mtime = 0
    ti.uid = ti.gid = 0
    ti.uname = ti.gname = ""
    return ti


members = []
for top in INCLUDE:
    p = os.path.join(home, top)
    if not os.path.lexists(p):
        sys.exit("missing include: " + top)
    if os.path.isdir(p) and not os.path.islink(p):
        for root, dirs, files in os.walk(p, followlinks=False):
            dirs.sort()
            members.append(root)
            for name in sorted(files + [d for d in dirs if os.path.islink(os.path.join(root, d))]):
                members.append(os.path.join(root, name))
            # symlinked dirs must not be descended into
            dirs[:] = [d for d in dirs if not os.path.islink(os.path.join(root, d))]
    else:
        members.append(p)

# parents of nested includes (".config", "dev", "dev/docket") so extraction recreates them
parents = set()
for top in INCLUDE:
    parts = top.split("/")[:-1]
    for i in range(1, len(parts) + 1):
        parents.add("/".join(parts[:i]))

seen = set()
with tarfile.open(out, "w", format=tarfile.USTAR_FORMAT) as tf:
    for rel in sorted(parents):
        ti = norm(tf.gettarinfo(os.path.join(home, rel), arcname=rel))
        tf.addfile(ti)
    for path in members:
        rel = os.path.relpath(path, home)
        if rel in seen:
            continue
        seen.add(rel)
        ti = norm(tf.gettarinfo(path, arcname=rel))
        if ti.issym():
            ti.linkname = rewrite(ti.linkname)
            tf.addfile(ti)
        elif ti.isfile():
            with open(path, "rb") as fh:
                data = fh.read()
            try:
                data = rewrite(data.decode("utf-8")).encode("utf-8")
            except UnicodeDecodeError:
                for s in spellings:
                    data = data.replace(s.encode(), TOKEN.encode())
            ti.size = len(data)
            tf.addfile(ti, io.BytesIO(data))
        elif ti.isdir():
            tf.addfile(ti)
        else:
            sys.exit("unexpected member type: " + rel)
print("members:", len(seen) + len(parents))
```

## What `home.tar` holds

| Folder | Written by | Contents |
|---|---|---|
| `.claude/` | `link-skills.sh`, `sync-agents.sh`, `ensure-docket-env.sh` | `skills/docket-*` (12 symlinks), `agents/docket-*.md` (16), `settings.json` with an `env` block setting `DOCKET_SCRIPTS_DIR` and `DOCKET_BASH_PATH` |
| `.cursor/` | same | `skills/` (12 symlinks), `agents/` (16), `rules/docket-dispatch.mdc` |
| `.codex/` | same | `skills/` (12 symlinks), `agents/docket-*.toml` (16). **Saved, never asserted.** |
| `.agents/` | same | `skills/` (12 symlinks, OpenCode's skill root), `agents/` (16, Claude-shaped, unpinned) |
| `.opencode/` | `sync-agents.sh` | `agents/docket-*.md` (16) |
| `.config/docket/` | `ensure-global-config.sh` | `config.yml` with the managed `runtime.bash` block |
| `dev/docket/skills/` | the tag checkout | the tree every skill symlink points at |
| `.zshenv` | `ensure-docket-env.sh` | the managed `DOCKET_SCRIPTS_DIR` / `DOCKET_BASH_PATH` export block |

Not saved: `dev/sample` (it is the bundle plus `clone-config.txt`), `.cache`, `.local`, and the rest
of `dev/docket`. So after a restore, `DOCKET_SCRIPTS_DIR` in `.zshenv` and `settings.json` points at
`@@SANDBOX_HOME@@/dev/docket/scripts`, which does not exist.

v0.9.2 wrote **no** `~/.claude/CLAUDE.md` and no dispatch block anywhere under the home. Its only
dispatch artifact is Cursor's `rules/docket-dispatch.mdc`.

## Records

| `records.txt` entry | Exercises |
|---|---|
| `docket:docs/adrs/0001-greeting-script-is-posix-sh.md` | ADR, `Accepted`, `change: 1`, made by the `docket-adr` flow (ADR commit, then a separate index commit) |
| `docket:docs/changes/active/0003-add-a-farewell-script.md` | proposed + spec (Brainstorm mode), `related: [1]`, priority `high` |
| `docket:docs/changes/active/0004-add-a-license-file.md` | trivial proposed (`trivial: true`, no spec) |
| `docket:docs/changes/active/0005-read-the-default-greeting-name-from-the-environment.md` | deferred: minted by `mint-stub.sh` (`discovered_from: [1]`, single-quoted `title:` written by the script), then the groom Defer exit |
| `docket:docs/changes/archive/2026-10-04-0001-add-a-greeting-script.md` | done #1: spec, plan, results, `adrs: [1]`, `pr:`, `reconciled: true`, `claimed_at:` left empty by `archive-change.sh` |
| `docket:docs/changes/archive/2026-10-04-0002-document-the-greeting-script.md` | done #2: trivial, `depends_on: [1]`, plan, results, `pr:`, empty `claimed_at:` |
| `docket:docs/changes/learnings/posix-sh-means-no-arrays.md` | learning from the finalize harvest; double-quoted `hook:` with `\"` escapes |
| `docket:docs/superpowers/specs/2026-10-04-add-a-farewell-script-design.md` | spec of 0003, back-link to `active/` |
| `docket:docs/superpowers/specs/2026-10-04-add-a-greeting-script-design.md` | spec of 0001, back-link re-stamped to `archive/` |

Counts: 5 changes (3 active, 2 archived), 1 ADR, 2 specs, 1 learning. Also on `docket`, but not
records: `docs/changes/BOARD.md` (rendered by the tag's board pass), `docs/adrs/README.md` and
`docs/changes/learnings/README.md` (tag-rendered indexes). On `main`: `README.md`, `.gitignore`
(the managed block), `hello.sh`, `USAGE.md`, the two plans and the two results files. Nothing was
published to `main`, so `records.txt` has no `main:` entries.

**Empty categories** (a saved case only covers what it contains): no in-progress, implemented,
blocked or killed change; no superseded or reversed ADR; no stacked change; no `.docket.yml` and
no `.docket.local.yml`; no per-repository agent wrappers (the repo never opted in with
`agents:` or `agent_harnesses:`); no change with a `## Run halted` section; no record on `main`.

## Gaps

- **Interactive steps were hand-written** to the tag's prescribed shape: the specs and change
  bodies (`skills/docket-new-change/SKILL.md` Brainstorm and Trivial modes, steps 4–5, using
  `change-template.md`); the claim, reconcile, `plan:`, `adrs:`, `implemented` + `pr:` +
  `results:` field writes (`skills/docket-implement-next/SKILL.md` Steps 2–7 and the field-write
  rule, including the `claimed_at` re-stamp); the plan, code and results commits on
  `feat/<slug>`; the ADR (`skills/docket-adr/SKILL.md` Create, from `adr-template.md`); the defer
  (`skills/docket-groom-next/SKILL.md` Step 4 exit 4); and the learning
  (`skills/docket-finalize-change/SKILL.md` step 2.5 and `references/learnings.md`). The commit
  subjects use a `docket(<id>): …` style. The skills prescribe no exact wording for these.
- **No pull request was opened or merged on GitHub.** `gh` was stubbed out. Each `pr:` holds an
  invented `https://github.com/fixture/sample/pull/<n>` URL, and the merge is a local
  `git merge --no-ff feat/<slug>` on `main` with GitHub's merge-commit subject, pushed to `origin`.
  The close-out then ran the tag's terminal close-out sequence, with the merge commit's UTC date.
- **`terminal_publish` is `false` by default at v0.9.2**, so the tag's close-out publishes nothing
  onto `main`. The plan's "terminal-published to `main`" became "the publish step ran and skipped,
  as the tag does". This keeps the bootstrap's config, the common path.
- **The tag's merge-date command** in `terminal-close-out.md` step 1 needs `--format=%ad` to print
  only the date (run 1 above).
- **The worktree-scoped `core.hooksPath`** is not a `git config --local` setting, so
  `clone-config.txt` replays it through its `hooks-off` line (see Step 7).
