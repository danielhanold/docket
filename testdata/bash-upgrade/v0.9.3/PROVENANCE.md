# Provenance — saved Bash docket v0.9.3 install and migrated `docket`-branch repository

This directory is a saved upgrade case: the state a user had after installing Bash docket from the
`v0.9.3` tag, running one repository in single-branch mode, converting it with the tag's
`migrate-to-docket.sh`, working on in `docket` mode, and then committing a `.docket.yml` that sets
every setting the Go binary now refuses or warns about. It was made once, by hand, in a throwaway
sandbox. The tag is the generator; no generator script is committed. The scratch helpers used are
reproduced verbatim below.

**Temporary.** This case and the test that reads it are retired when stable v1.0.0 ships.

## Source

| Item | Value |
|---|---|
| Tag | `v0.9.3` (annotated tag object `5897ac000d2ea69c36a226b9459d925067530190`) |
| Commit | `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf` |
| `install.sh` SHA-256 | `18d46c0489c638827ef56ab85ac39179acb263f2b2ee83a5c0ae72aaae785c71` (byte-identical to v0.9.2's) |
| Date made | 2026-10-04 (UTC; sandbox work began 2026-10-04T18:30:38Z) |
| Host | macOS 27.0 (build 26A428), Darwin 27.0.0 arm64 |
| Bash | GNU bash 5.3.20(1)-release (aarch64-apple-darwin27.0.0), `/opt/homebrew/bin/bash` |
| Git / Python | git 2.55.0 / Python 3.14.7 (Python only for the `home.tar` helper) |

Between the two tags, the Bash surface changed only in the agent set: v0.9.3 adds
`agents/docket-plan-writer.md` (17 agents instead of 16), its `harness-defaults.yml` rows and its
Cursor dispatch rule, plus skill text in `docket-convention` and `docket-implement-next`.
`scripts/`, `install.sh`, `link-skills.sh` and `migrate-to-docket.sh` are unchanged, and
`sync-agents.sh` differs only in one comment (`git diff --stat v0.9.2 v0.9.3`).

## Files

| File | What it is |
|---|---|
| `origin.bundle` | `git bundle create … --all` of the sandbox bare `origin`: `refs/heads/main` (`5654d4f9f09ec3fe04a375b970c41775fa291717`), `refs/heads/docket` (`7a4b3425a4872008e2179df90212803cef99248b`) and `HEAD` → `main`, complete history |
| `home.tar` | USTAR tar rooted at the sandbox `HOME`; 202 members, 48 symlinks; mtimes 0, uid/gid 0; `@@SANDBOX_HOME@@` stands for the home path in link targets and file bodies |
| `clone-config.txt` | the clone actions the restore replays (one `worktree`, one `config`, one `hooks-off`) |
| `records.txt` | the 14 record paths (9 on `docket`, 5 on `main`) the case holds |
| `clone-files.tar` | USTAR tar rooted at the sample clone: its ignored `.claude/` folder, 20 members (`.claude/`, `.claude/agents/`, the 17 per-repository `agents/docket-*.md` wrappers, `settings.local.json`); same normalization and `@@SANDBOX_HOME@@` rule as `home.tar` (no member needed it). Regenerated afterwards from the tag; see Step 10 |

## The seeded root (what this case exists to prove)

The v0.9.2 case covers the empty-orphan bootstrap. This case covers the other receiptless legacy
root, the one `migrate-to-docket.sh` writes: a root commit whose tree is a copy of the planning
directories of `main` at the moment of migration.

| Item | Value |
|---|---|
| Pre-migration `main` tip (the copied snapshot) | `3cc746b36708159334f436ec457477821070edce` (`docket: board refresh`) |
| `docket` root commit | `2e86fd7f8bce29bbf1a47435bbf9f4f092157fde` (`docket: seed metadata branch from main (migrate-to-docket.sh)`), no parents, no trailers |
| Root tree | `db3ba79d84f62347e34dec034cef959e1bba42ad` (not the empty tree `4b825dc…`) |
| `{docs/changes, docs/adrs, docs/superpowers/specs}` projection of `3cc746b` | `db3ba79d84f62347e34dec034cef959e1bba42ad` — equal to the root tree |
| Shared ancestry of `main` and `docket` | none (`git merge-base main docket` exits 1) |

The seed set was `docs/changes docs/adrs docs/superpowers/specs docs/changes/BOARD.md`; the board
sits inside `docs/changes`, so the root tree is exactly the three-directory projection the Go
binary's `verifyLegacyEquivalence` composes. The `.docket.yml` on `3cc746b` is
`metadata_branch: main` / `integration_branch: main`, so that snapshot's own config resolves the
default directories.

## Sandbox

The sandbox root `$SB` was
`/private/tmp/claude-501/-Users-homer-dev-docket/9cbfa2e1-e5f8-4f8a-9555-3ec1f4e089eb/scratchpad/bash-upgrade-v093.GeYYNm`
(a session scratchpad, removed afterwards). `HOME` was `$SB/home`. Every sandbox command below ran
through the wrapper `$SB/sbx`, written by Step 1; it clears the environment and sets exactly:

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

Where a tag script needs `DOCKET_SCRIPTS_DIR`/`DOCKET_BASH_PATH`, the commands pass the values the
tag's `ensure-docket-env.sh` wrote to `$SB/home/.zshenv` (`$HOME/dev/docket/scripts` and
`/opt/homebrew/bin/bash`), which a user's zsh exports. The sandbox shell is `sh`, which does not
read `.zshenv`.

## Commands, in order

`$SCRATCH` is the session scratchpad that contains `$SB`.

### Step 1 — sandbox

```bash
SB="$(mktemp -d "$SCRATCH/bash-upgrade-v093.XXXXXX")"
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
ls /opt/homebrew/bin/docket     # No such file or directory
```

### Step 2 — the tag, checked out into the sandbox home

```bash
"$SB/sbx" git clone --quiet --branch v0.9.3 --depth 1 file:///Users/homer/dev/docket "$SB/home/dev/docket"
"$SB/sbx" git -C "$SB/home/dev/docket" remote remove origin
git -C /Users/homer/dev/docket rev-parse 'v0.9.3^{commit}' v0.9.3   # dd742abd5e9fcdf8ffe78eb6f36a293410873bbf / 5897ac000d2ea69c36a226b9459d925067530190
"$SB/sbx" git -C "$SB/home/dev/docket" rev-parse HEAD               # dd742abd5e9fcdf8ffe78eb6f36a293410873bbf
shasum -a 256 "$SB/home/dev/docket/install.sh"                      # 18d46c04…785c71
```

### Step 3 — harness folders and the tag's installer

The v0.9.3 harness lists equal v0.9.2's: `link-skills.sh` (`HARNESS_SKILL_DIRS`) targets `.claude`,
`.codex`, `.cursor`, `.agents`, `.kiro` and `.windsurf` `skills/`, and `sync-agents.sh` writes
`.<token>/agents` for every token in `DOCKET_GI_HARNESS_TOKENS`
(`claude codex cursor opencode agents kiro windsurf`) whose parent exists. As for v0.9.2, the
parents for Claude Code, Cursor, Codex and OpenCode (`.opencode`, and `.agents` for its skills) were
created, and `.kiro` and `.windsurf` were not:

```bash
date -u +%Y-%m-%dT%H:%M:%SZ > "$SB/task-start.txt"     # 2026-10-04T18:30:38Z
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
sync-agents: WARN agents/docket-<name>: no harness-specific model — generated unpinned; harness 'agents' will apply its own default. … (once per agent, 17 lines)
sync-agents: WARN harness 'agents' has no named emitter — its wrappers are Claude-shaped and unverified for 'agents', … (ADR-0060). …
sync-agents: done
==> ensure-docket-env.sh (export Docket runtime environment)
ensure-docket-env: wrote DOCKET_SCRIPTS_DIR and DOCKET_BASH_PATH -> $SB/home/.zshenv (zsh)
ensure-docket-env: set env.DOCKET_SCRIPTS_DIR and env.DOCKET_BASH_PATH -> .claude/settings.json
docket: install complete
```

### Step 4 — a single-branch repository, populated through the tag's main-mode flows

The tag's single-branch opt-out is `metadata_branch: main` with `integration_branch: main`
(`skills/docket-convention/SKILL.md`, *Configuration*); `docket-config.sh` reads the committed
`.docket.yml` from `origin/HEAD`, so it is pushed before any flow runs.

```bash
"$SB/sbx" git init --quiet --bare -b main "$SB/origin.git"
"$SB/sbx" git clone --quiet "$SB/origin.git" "$SB/home/dev/sample"
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && printf "# sample\n" > README.md && git add README.md && git commit -qm "initial" && git push -q origin main && git remote set-head origin main'
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && printf "metadata_branch: main\nintegration_branch: main\n" > .docket.yml && git add .docket.yml && git commit -qm "chore: use docket in single-branch mode" && git push -q origin main'
"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && export DOCKET_SCRIPTS_DIR="$HOME/dev/docket/scripts" DOCKET_BASH_PATH=/opt/homebrew/bin/bash && "$DOCKET_BASH_PATH" "$DOCKET_SCRIPTS_DIR/docket.sh" preflight'   # exit 0, DOCKET_MODE=main, BOOTSTRAP=PROCEED
mkdir -p "$SB/snap-pre-populate" && cp -Rp "$SB/origin.git" "$SB/home/dev/sample" "$SB/snap-pre-populate/"
"$SB/sbx" /opt/homebrew/bin/bash "$SCRATCH/populate-v093.sh" pre > "$SB/populate-pre.log" 2>&1   # exit 0
```

One run, exit 0, no `gh` call. In main mode the metadata working tree is the primary clone on
`main`, so every metadata commit went straight to `origin/main`. Tag scripts used (counts from the
log): `preflight` ×4, `docket-status --board-only [--must-land]` ×6, `render-change-links` ×6,
`render-artifact-backlink` ×5, `render-adr-index` ×1, `render-learnings-index` ×1, `archive-change`
×1, `terminal-publish` ×1 (logged `terminal-publish: main-mode (metadata-branch ==
integration-branch); no-op`) and `cleanup-feature-branch` ×1.

### Step 5 — `migrate-to-docket.sh` (the seeded-root bootstrap)

```bash
mkdir -p "$SB/snap-pre-migrate" && cp -Rp "$SB/origin.git" "$SB/home/dev/sample" "$SB/snap-pre-migrate/"
"$SB/sbx" git -C "$SB/origin.git" rev-parse main > "$SB/pre-migrate-main.txt"   # 3cc746b36708159334f436ec457477821070edce
cd "$SB/home/dev/sample" && ../../../sbx env DOCKET_SCRIPTS_DIR="$SB/home/dev/docket/scripts" DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/migrate-to-docket.sh" --yes > ../../../migrate.log 2>&1   # exit 0
```

The plan's form (`"$SB/sbx" sh -c 'cd "$HOME/dev/sample" && /opt/homebrew/bin/bash "$HOME/dev/docket/migrate-to-docket.sh" --yes'`)
was first issued combined with the snapshot command; the agent harness's safety check refused it
before anything ran (it cannot inspect removals inside a `sh -c` script, and the tag's script
removes its own `mktemp -d` directory). Nothing executed. The command above runs the same script
directly, with the two variables a user's zsh exports; without `DOCKET_BASH_PATH` the script's
`${DOCKET_BASH_PATH:?run docket/install.sh}` guard would abort it mid-seed.

One run, exit 0, no `gh` call. Its output:

```text
==> Resolved configuration
  target repo        : $SB/home/dev/sample  (the repo that will be migrated)
  integration_branch : main  (code lands here; ref origin/main)
  changes_dir        : docs/changes
  adrs_dir           : docs/adrs
  results_dir        : docs/results  (stays on integration — NOT seeded onto docket)
  specs_dir          : docs/superpowers/specs
  board              : docs/changes/BOARD.md
  (source: .docket.yml)

==> Checking preconditions
  Live planning surface present on origin/main — ok.
  origin/docket does not exist — ok to seed.

==> Seeding the orphan 'docket' branch
  Local 'docket' branch absent — creating orphan and seeding from origin/main.
Preparing worktree (new branch 'docket')
  Seeded docket with: docs/changes docs/adrs docs/superpowers/specs docs/changes/BOARD.md
  Pushing docket → origin/docket.
To $SB/origin.git
 * [new branch]      docket -> docket
branch 'docket' set up to track 'origin/docket'.

==> Pruning the live planning surface from 'main'
Preparing worktree (new branch 'migrate-prune-main')
  Removing from main: docs/changes/active docs/changes/BOARD.md
  Kept on main: docs/changes/archive/, docs/adrs/, docs/results/, docs/superpowers/plans/

==> Seeding the managed docket .gitignore block on 'main'
docket-gitignore: UPDATED $SB/tmp/migrate-to-docket.4Ald5M/prune/.gitignore managed docket block — COMMIT THIS so docket-owned files stay untracked.

==> Pushing 'main' → origin/main
To $SB/origin.git
   3cc746b..fa99478  HEAD -> main

==> Granting docket's integration-branch push permission (local Claude settings)
ensure-claude-settings: created .claude/settings.local.json and granted: Bash(git -C * push origin HEAD:main)

==> Migration complete
  (next-steps text: .docket.yml is optional; pin metadata_branch: main only to opt back out; re-run
  install.sh; the allow-rule was written to .claude/settings.local.json)
```

Fidelity of the seeded root (read-only; loose objects only, in a throwaway clone):

```bash
"$SB/sbx" git clone -q --bare "$SB/origin.git" "$SB/fidelity.git"
cd "$SB/fidelity.git"
"$SB/sbx" git rev-list --max-parents=0 docket                      # 2e86fd7f8bce29bbf1a47435bbf9f4f092157fde
"$SB/sbx" git cat-file -p 2e86fd7f8bce29bbf1a47435bbf9f4f092157fde # tree db3ba79d…, no parent, no trailers
"$SB/sbx" git merge-base main docket || echo "no shared ancestry"  # no shared ancestry
# (a first projection attempt with read-tree --prefix failed here: it needs a work tree)
"$SB/sbx" git clone -q --no-checkout "$SB/origin.git" "$SB/fidelity-wt"
cd "$SB/fidelity-wt"
"$SB/sbx" env GIT_INDEX_FILE="$SB/tmp/fid2.index" git read-tree --empty
for p in docs/changes docs/adrs docs/superpowers/specs; do "$SB/sbx" env GIT_INDEX_FILE="$SB/tmp/fid2.index" git read-tree --prefix="$p/" "3cc746b36708159334f436ec457477821070edce:$p"; done
"$SB/sbx" env GIT_INDEX_FILE="$SB/tmp/fid2.index" git write-tree   # db3ba79d84f62347e34dec034cef959e1bba42ad
```

### Step 6 — switch to `docket` mode and keep working there

The migration leaves `metadata_branch: main` in `.docket.yml`, which keeps the tag in single-branch
mode; the migration's own next steps say to pin it only to opt back out. The user's edit:

```bash
cd "$SB/home/dev/sample"
"$SB/sbx" git pull -q --ff-only origin main      # the clone was 2 commits behind (prune, .gitignore)
printf "metadata_branch: docket\nintegration_branch: main\n" > .docket.yml
"$SB/sbx" git add .docket.yml && "$SB/sbx" git commit -qm "chore: switch docket to docket-mode after migrate-to-docket.sh" && "$SB/sbx" git push -q origin main
"$SB/sbx" env DOCKET_SCRIPTS_DIR="$SB/home/dev/docket/scripts" DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/scripts/docket.sh" preflight > "$SB/preflight-post.out" 2> "$SB/preflight-post.err"   # exit 0, DOCKET_MODE=docket, BOOTSTRAP=PROCEED
mkdir -p "$SB/snap-post-migrate" && cp -Rp "$SB/origin.git" "$SB/home/dev/sample" "$SB/snap-post-migrate/"
"$SB/sbx" /opt/homebrew/bin/bash "$SCRATCH/populate-v093.sh" post > "$SB/populate-post.log" 2>&1   # exit 0
```

- **`.docket` worktree command:** `docket.sh preflight` (`scripts/lib/docket-preflight.sh`
  `docket_preflight`) ran `git -C <repo> worktree add .docket docket` on the local `docket` branch
  the migration had made and pushed with `-u`, then `disable-worktree-hooks.sh --worktree .docket`.
  The migration had already run `disable-worktree-hooks.sh` on its own temp worktrees, which is
  where `extensions.worktreeConfig=true` first came from.
- **Post-migration run:** one run, exit 0, no `gh` call. Tag scripts used: `preflight` ×5,
  `docket-status --board-only [--must-land]` ×5, `render-change-links` ×3,
  `render-artifact-backlink` ×2, `mint-stub` ×1, `archive-change` ×1, `terminal-publish
  --enabled false` ×1 (logged `terminal_publish: false — skipping publish onto main; the record
  stays on docket`) and `cleanup-feature-branch` ×1.

`$SCRATCH/populate-v093.sh`, verbatim (both phases):

```bash
#!/opt/homebrew/bin/bash
# populate-v093.sh pre|post — scratch driver (never committed) that populates the v0.9.3 sample
# repository through the tag's own scripts. Run ONLY through the sandbox wrapper:
#   "$SB/sbx" /opt/homebrew/bin/bash "$SCRATCH/populate-v093.sh" pre    # single-branch (main) mode
#   "$SB/sbx" /opt/homebrew/bin/bash "$SCRATCH/populate-v093.sh" post   # docket mode, after migrate
# It never sources a tag script; every tag script runs as a subprocess via docket.sh.
set -euo pipefail
PHASE="${1:?usage: populate-v093.sh pre|post}"

export DOCKET_SCRIPTS_DIR="$HOME/dev/docket/scripts"
export DOCKET_BASH_PATH=/opt/homebrew/bin/bash
R="$HOME/dev/sample"
case "$PHASE" in
  pre)  M="$R";         MBRANCH=main ;;    # main-mode: the metadata working tree is the primary tree
  post) M="$R/.docket"; MBRANCH=docket ;;
  *) echo "unknown phase $PHASE" >&2; exit 2 ;;
esac
C="$M/docs/changes"
A="$M/docs/adrs"
SPECS="$M/docs/superpowers/specs"
TODAY="$(date -u +%F)"
cd "$R"

dk(){ echo "+ docket.sh $*" >&2; "$DOCKET_BASH_PATH" "$DOCKET_SCRIPTS_DIR/docket.sh" "$@"; }
ghcheck(){ if [ -e "$HOME/../gh-calls.log" ]; then echo "FATAL: gh was called:" >&2; cat "$HOME/../gh-calls.log" >&2; exit 97; fi; }
# metadata commit: explicit paths only, then push the metadata branch (main in main-mode)
mcommit(){ local msg="$1"; shift; git -C "$M" add -- "$@"; git -C "$M" commit -q -m "$msg" -- "$@"; git -C "$M" push -q origin "$MBRANCH"; ghcheck; }
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
  # main-mode: --metadata-branch equals --integration-branch, so terminal-publish logs its mode-guard
  # no-op (the archive move on main IS the terminal record). docket-mode: --enabled false (the
  # tag's default terminal_publish) logs the knob-guard skip.
  dk terminal-publish --id "$id" --outcome done --integration-branch main --metadata-branch "$MBRANCH" \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false --metadata-worktree "$M"
  ghcheck
  dk cleanup-feature-branch --slug "$slug"
  ghcheck
  board
}

new_change(){  # new_change ID SLUG TITLE PRIORITY TYPE DEPENDS RELATED SPEC TRIVIAL WHY WHAT OUT
  local id="$1" slug="$2" title="$3" prio="$4" type="$5" deps="$6" rel="$7" spec="$8" trivial="$9"
  local why="${10}" what="${11}" out="${12}"
  local pad; pad="$(printf '%04d' "$id")"
  local F="$C/active/$pad-$slug.md"
  {
    printf -- '---\nid: %s\nslug: %s\ntitle: %s\nstatus: proposed\npriority: %s\ntype: %s\n' "$id" "$slug" "$title" "$prio" "$type"
    printf 'created: %s\nupdated: %s\ndepends_on: %s\nstacked_on:\nrelated: %s\ndiscovered_from: []\nadrs: []\n' "$TODAY" "$TODAY" "$deps" "$rel"
    [ -n "$spec" ] && printf 'spec: %s\n' "$spec"
    printf 'plan:\nresults:\ntrivial: %s\nauto_groomable:\nbranch:\npr:\nblocked_by:\nreconciled: false\n---\n\n' "$trivial"
    printf '## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n<!-- docket:artifacts:end -->\n\n'
    printf '## Why\n\n%s\n\n## What changes\n\n%s\n\n## Out of scope\n\n%s\n\n## Reconcile log\n' "$why" "$what" "$out"
  } > "$F"
}

dk preflight >/dev/null
mkdir -p "$C/active" "$SPECS" "$A"

if [ "$PHASE" = pre ]; then
  # -------------------------------------------------------------------------------------------
  # 0001 — brainstorm-mode change with a spec, built, merged and closed out as done (main-mode).
  # -------------------------------------------------------------------------------------------
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
  new_change 1 add-a-greeting-script "Add a greeting script" medium feat "[]" "[]" "$S1" false \
    "The sample repository has no code yet. A small greeting script gives later changes something to
build on and proves the build loop end to end." \
    'Add `hello.sh`, which greets its first argument or the world.' \
    "Localisation and any configuration file."
  F1="$C/active/0001-add-a-greeting-script.md"
  change_links "$F1"
  dk render-artifact-backlink --artifact-file "$M/$S1" --change-file "$F1"
  mcommit "docket(0001): new change — add a greeting script (spec)" "docs/changes/active/0001-add-a-greeting-script.md" "$S1"
  board

  # 0002 — trivial-mode change (stays proposed until after the migration).
  new_change 2 document-the-greeting-script "Document the greeting script" low docs "[1]" "[]" "" true \
    "The repository does not say how to run the greeting script." \
    'Add a short `USAGE.md` that shows how to run it.' \
    "Any change to the script itself."
  mcommit "docket(0002): new change — document the greeting script (trivial)" "docs/changes/active/0002-document-the-greeting-script.md"
  board

  build_and_close 1 add-a-greeting-script "https://github.com/fixture/sample/pull/1" hello.sh \
    '#!/bin/sh
# hello.sh — greet the first argument, or the world.
printf "hello, %s\n" "${1:-world}"
' adr

  # 0003 — proposed change with a spec (docket-new-change Brainstorm mode).
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
  new_change 3 add-a-farewell-script "Add a farewell script" high feat "[]" "[1]" "$S3" false \
    "A greeting without a farewell is half a conversation." \
    'Add `bye.sh`, the counterpart of `hello.sh`.' \
    "Merging the two scripts into one."
  F3="$C/active/0003-add-a-farewell-script.md"
  change_links "$F3"
  dk render-artifact-backlink --artifact-file "$M/$S3" --change-file "$F3"
  mcommit "docket(0003): new change — add a farewell script (spec)" "docs/changes/active/0003-add-a-farewell-script.md" "$S3"
  board
else
  # -------------------------------------------------------------------------------------------
  # docket mode, after migrate-to-docket.sh: 0002 (seeded as a proposed trivial change) is
  # built, merged and closed out as done; 0004 is a new trivial proposed change; 0005 is minted
  # by mint-stub.sh and then deferred.
  # -------------------------------------------------------------------------------------------
  build_and_close 2 document-the-greeting-script "https://github.com/fixture/sample/pull/2" USAGE.md \
    '# Usage

Run `sh hello.sh [name]`. With no name it greets the world.
' ""

  dk preflight >/dev/null
  new_change 4 add-a-license-file "Add a license file" medium chore "[]" "[]" "" true \
    "The sample repository has no license." \
    'Add an MIT `LICENSE` file at the root.' \
    "Per-file license headers."
  mcommit "docket(0004): new change — add a license file (trivial)" "docs/changes/active/0004-add-a-license-file.md"
  board

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
fi

echo "populate-v093 $PHASE: done"
```

### Step 7 — the deliberate configuration, committed last

v0.9.3's schema (`scripts/docket-config.sh` and its `.md`, `.docket.example.yml`,
`skills/docket-convention`, and `sync-agents.sh` for `agents:`/`runners:`) accepts every key in the
spec's configuration table. Each is set below, switched **on** where the Go binary blocks only when
on. The file was first tried in a throwaway copy of the repository (its own copy of `origin`), so
the saved history holds only the accepted version:

```bash
mkdir -p "$SB/snap-pre-config" && cp -Rp "$SB/origin.git" "$SB/home/dev/sample" "$SB/snap-pre-config/"
mkdir -p "$SB/trial" && cp -Rp "$SB/origin.git" "$SB/trial/origin.git" && "$SB/sbx" git clone -q "$SB/trial/origin.git" "$SB/trial/sample" && "$SB/sbx" git -C "$SB/trial/sample" remote set-head origin main
cp "$SB/docket.yml.final" "$SB/trial/sample/.docket.yml"
cd "$SB/trial/sample" && "$SB/sbx" git add .docket.yml && "$SB/sbx" git commit -qm "chore: docket configuration" && "$SB/sbx" git push -q origin main
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/scripts/docket-config.sh" --export   # exit 0 (one runtime.bash warning)
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/sync-agents.sh" --check   # exit 1:
#   sync-agents: check: the docket dispatch block in $SB/trial/sample/CLAUDE.md is missing or stale — run: bash sync-agents.sh and commit it
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/sync-agents.sh"   # exit 0: created CLAUDE.md … COMMIT THIS
"$SB/sbx" git add CLAUDE.md && "$SB/sbx" git commit -qm "chore: commit the docket dispatch block" && "$SB/sbx" git push -q origin main
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/sync-agents.sh" --check   # exit 0
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/scripts/docket-config.sh" --export   # exit 0
```

The `agents:` block opts the repository into the tag's per-repository agent generation, and the
tag's own check then requires a committed dispatch block in the repository's `CLAUDE.md`. So the
config commit is followed by that one commit, made exactly as the tag tells the user to. Neither
reaches `gh`. The trial's `sync-agents.sh` also re-ran the user-level pass on the shared sandbox
home; a SHA-1 listing of every harness file and a listing of every symlink target, taken before the
trial and after the final run below, were identical. Then, for real:

```bash
cd "$SB/home/dev/sample"
cp "$SB/docket.yml.final" .docket.yml
"$SB/sbx" git add .docket.yml && "$SB/sbx" git commit -qm "chore: docket configuration" && "$SB/sbx" git push -q origin main
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/sync-agents.sh" > "$SB/sync-agents-repo.out" 2> "$SB/sync-agents-repo.err"   # exit 0
"$SB/sbx" git add CLAUDE.md && "$SB/sbx" git commit -qm "chore: commit the docket dispatch block" && "$SB/sbx" git push -q origin main
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/scripts/docket-config.sh" --export > "$SB/config-export.txt" 2> "$SB/config-export.err"   # exit 0
"$SB/sbx" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/sync-agents.sh" --check > "$SB/sync-check.out" 2> "$SB/sync-check.err"   # exit 0
```

`sync-agents.sh` printed the 17 unpinned-agent warnings for the `agents` harness, the
no-named-emitter warning, `created CLAUDE.md to carry the docket dispatch block — COMMIT THIS.`,
`wrote/updated the docket dispatch block in $SB/home/dev/sample/CLAUDE.md — COMMIT THIS
(machine-neutral; no model IDs).` and `done`. `config-export.err` held one line:
`docket-config: warning: committed config key runtime.bash is machine-local — set it in
.docket.local.yml or global config.yml; ignored`. `sync-check.err` was empty.

The committed `.docket.yml` (`main:.docket.yml`), verbatim:

```yaml
# Docket configuration for the sample repository (written for Bash docket v0.9.3).
metadata_branch: docket
integration_branch: main

runtime:
  bash: /opt/homebrew/bin/bash

finalize:
  gate: ci
  skip_results_only_delta: true

learnings:
  cap: 250

build:
  checkpoint: true

delegation_observation_budget: 45

board_surfaces: [inline, github]
github_project: {owner: fixture, number: 7}

terminal_publish: true
auto_groom: true

auto_capture:
  enabled: true
  types: [feat, fix]

dummy_mode:
  enabled: true
  persona: "A new contributor who knows shell scripts but not this project."
  surfaces: [dialogue, reports]

agents:
  claude:
    implement-next: { model: claude-opus-5, effort: high }
    rebase-resolver: { model: gpt-5.6-sol, effort: high, runner: codex }

runners:
  codex:
    sandbox: workspace-write
    network: true
    shim_model: inherit
    shim_effort: low
  opencode:
    permissions: ask

skills:
  brainstorm: superpowers:brainstorming
  plan: superpowers:writing-plans
  build: docket-build
  review: docket-review
  finish: superpowers:finishing-a-development-branch
```

| Key | Value set | Go behavior it exercises (spec table) | v0.9.3 verdict |
|---|---|---|---|
| `metadata_branch` | `docket` | warn and ignore | accepted (`METADATA_BRANCH=docket`) |
| `runtime.bash` | `/opt/homebrew/bin/bash` | warn and ignore | warned as machine-local and ignored, exit 0 (the tag's own documented handling) |
| `terminal_publish` | `true` | blocks when on | accepted (`TERMINAL_PUBLISH=true`) |
| `auto_groom` | `true` | blocks when on | accepted (`AUTO_GROOM=true`) |
| `build.checkpoint` | `true` | blocks when on | accepted (`BUILD_CHECKPOINT=true`) |
| `finalize.skip_results_only_delta` | `true` | blocks when on | accepted (`FINALIZE_SKIP_RESULTS_ONLY_DELTA=true`) |
| `auto_capture.enabled` | `true` | blocks when on | accepted (`AUTO_CAPTURE_ENABLED=true`) |
| `dummy_mode.enabled` | `true` | blocks when on | accepted (`DUMMY_MODE_ENABLED=true`) |
| `finalize.gate` | `ci` | blocks by value | accepted (`FINALIZE_GATE=ci`) |
| `board_surfaces` | `[inline, github]` | blocks by value | accepted (`BOARD_SURFACES=inline github`) |
| `skills.brainstorm`/`plan`/`build`/`review`/`finish` | the shipped defaults | blocks whenever present | accepted (`SKILL_*`) |
| `agents.claude.implement-next.model`/`effort` | `claude-opus-5` / `high` | blocks in a repository layer | accepted by `sync-agents.sh` (per-repo wrapper pinned `model: claude-opus-5`, `effort: high`) |
| `agents.claude.rebase-resolver.model`/`effort`/`runner` | `gpt-5.6-sol` / `high` / `codex` | blocks whenever present | accepted by `sync-agents.sh` (delegated wrapper, `--runner codex --model gpt-5.6-sol --effort high`) |
| `learnings.cap` | `250` | info notice | accepted (`LEARNINGS_CAP=250`) |
| `github_project` | `{owner: fixture, number: 7}` | info notice | accepted (the tag reads it nowhere) |
| `delegation_observation_budget` | `45` | info notice | accepted (`DELEGATION_OBSERVATION_BUDGET=45`) |
| companions `auto_capture.types` | `[feat, fix]` | info notice | accepted (`AUTO_CAPTURE_TYPES=feat fix`) |
| companions `dummy_mode.persona`/`surfaces` | one quoted line / `[dialogue, reports]` | info notice | accepted |
| companions `runners.codex.sandbox`/`network`/`shim_model`/`shim_effort`, `runners.opencode.permissions` | `workspace-write`/`true`/`inherit`/`low`, `ask` | info notice | accepted by `sync-agents.sh --check` (runner and shim validation gates) |

No table key was refused by v0.9.3. `integration_branch: main` is also set; it is a supported Go
key and is not part of the test.

`config-export.txt` (stdout of the final `--export`):

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
FINALIZE_GATE=ci
FINALIZE_TEST_COMMAND=''
FINALIZE_REQUIRE_PR_APPROVAL=false
FINALIZE_SKIP_RESULTS_ONLY_DELTA=true
LEARNINGS_ENABLED=true
LEARNINGS_CAP=250
BOARD_SURFACES=inline\ github
AUTO_GROOM=true
CHANGE_TYPES=chore\ docs\ feat\ fix\ refactor\ perf
AUTO_CAPTURE_ENABLED=true
AUTO_CAPTURE_TYPES=feat\ fix
TERMINAL_PUBLISH=true
RECLAIM_LEASE_TTL=72
RECLAIM_AUTO=false
BUILD_CHECKPOINT=true
REVIEW_MIN_FIX_SEVERITY=minor
REVIEW_MAX_FIX_TASKS=10
GATE_OBSERVATION_BUDGET=30
DELEGATION_OBSERVATION_BUDGET=45
SKILL_BRAINSTORM=superpowers:brainstorming
SKILL_PLAN=superpowers:writing-plans
SKILL_BUILD=docket-build
SKILL_REVIEW=docket-review
SKILL_FINISH=superpowers:finishing-a-development-branch
DUMMY_MODE_ENABLED=true
DUMMY_MODE_PERSONA=A\ new\ contributor\ who\ knows\ shell\ scripts\ but\ not\ this\ project.
DUMMY_MODE_SURFACES=dialogue\ reports
BOOTSTRAP=PROCEED
```

### Step 8 — export

```bash
OUT=<worktree>/testdata/bash-upgrade/v0.9.3
"$SB/sbx" git -C "$SB/origin.git" bundle create "$OUT/origin.bundle" --all
"$SB/sbx" git -C "$SB/origin.git" bundle verify "$OUT/origin.bundle"     # okay; refs/heads/docket, refs/heads/main, HEAD; complete history
{ "$SB/sbx" git -C "$SB/origin.git" ls-tree -r --name-only docket | sed 's/^/docket:/'; "$SB/sbx" git -C "$SB/origin.git" ls-tree -r --name-only main | sed 's/^/main:/'; } > "$SB/all-paths.txt"
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
python3 "$SCRATCH/make-home-tar.py" "$SB/home" "$OUT/home.tar"          # members: 202
tar -xOf "$OUT/home.tar" | grep -c -e "$SB" || true                      # 0
tar -xOf "$OUT/home.tar" | grep -c -e "/tmp/claude-501" || true          # 0 (the /tmp spelling of the same path)
tar -tvf "$OUT/home.tar" | grep -c -e '^l'                               # 48
du -sk "$OUT"                                                            # 676 (with this file; under 1024)
```

`clone-config.txt` has the same three lines as the v0.9.2 case: `worktree .docket docket`,
`config extensions.worktreeConfig true` and `hooks-off .docket`. Here `branch.docket.*` came from
the migration's `git push -u origin docket`; on a fresh clone, git's DWIM in
`git worktree add .docket docket` recreates the same tracking branch.

`$SCRATCH/make-home-tar.py` is byte-identical to the helper reproduced in
[`../v0.9.2/PROVENANCE.md`](../v0.9.2/PROVENANCE.md) (SHA-256
`de52b56ede3b6568b9ff17b9a291bcd97eaf67129c7e1c05b6681212ad611458`); it is repeated here verbatim:

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

### Step 9 — the ownership probe

```bash
timeout --kill-after=10s 10m go test -tags integration -count=1 -run '^TestIntegrationBashUpgradeOwnership' ./internal/bashupgrade/ -v   # exit 0; v0.9.2 and v0.9.3 PASS
```

On this case `docket repository prepare --json` returned `no-op` / `healthy` (no ownership
refusal), so the binary proved the receiptless non-empty root through the legacy-equivalence path.
`docket repository check --json` (exit 1) reported `postconditions-unmet` (error),
`committed-ignore-invalid` (error, `.gitignore`), `legacy-config-key-present` (error,
`.docket.yml`), `board-stale` (warning), `artifact-links-stale` (warning, both archived changes) and
`test-config-missing` (warning).

### Step 10 — the per-clone ignored files (`clone-files.tar`, regenerated)

Made the same day (sandbox work began 2026-10-04T19:25:55Z), on the same host and tools, after the
case above was exported. The Step 1 sandbox was gone, so a new one was built the same way, under
the same session scratchpad: `$SB2` was
`$SCRATCH/bash-upgrade-v093-clone.6we20K`. Its `sbx` wrapper is the Step 1 wrapper with `$SB2` for
`$SB`. Tag scripts run only as subprocesses; none is sourced.

```bash
/opt/homebrew/bin/bash "$SCRATCH/regen-clone-files.sh"   # exit 0
```

`$SCRATCH/regen-clone-files.sh`, verbatim (SHA-256
`a509dbbe0e62f20f6642acbd69d36f71e38346a9c65be546001dec0b5ef1d2e0`):

```bash
#!/opt/homebrew/bin/bash
# regen-clone-files.sh — scratch driver (never committed). Regenerates the per-clone ignored
# files of the saved v0.9.3 case in a throwaway sandbox. Tag scripts run only as subprocesses.
set -euo pipefail
SCRATCH="/private/tmp/claude-501/-Users-homer-dev-docket/9cbfa2e1-e5f8-4f8a-9555-3ec1f4e089eb/scratchpad"
CASE="/Users/homer/dev/docket/.worktrees/upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa/testdata/bash-upgrade/v0.9.3"
SB="$(mktemp -d "$SCRATCH/bash-upgrade-v093-clone.XXXXXX")"
echo "SB=$SB"
mkdir -p "$SB/home" "$SB/stub-bin" "$SB/home/.config" "$SB/home/.local/share" "$SB/home/.cache" "$SB/home/.local/state" "$SB/tmp"
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
chmod +x "$SB/sbx"
X="$SB/sbx"
date -u +%Y-%m-%dT%H:%M:%SZ > "$SB/task-start.txt"
cat "$SB/task-start.txt"

# Step A — the tag and its installer, exactly as PROVENANCE.md Steps 2-3.
"$X" git clone --quiet --branch v0.9.3 --depth 1 file:///Users/homer/dev/docket "$SB/home/dev/docket"
"$X" git -C "$SB/home/dev/docket" remote remove origin
"$X" git -C "$SB/home/dev/docket" rev-parse HEAD
mkdir -p "$SB/home/.claude" "$SB/home/.cursor" "$SB/home/.codex" "$SB/home/.opencode" "$SB/home/.agents"
( cd "$SB/home/dev/docket" && "$X" /opt/homebrew/bin/bash ./install.sh ) > "$SB/install.log" 2>&1
tail -n 3 "$SB/install.log"

# Step B — the saved repository, restored the way the harness restores it.
"$X" git init --quiet --bare -b main "$SB/origin.git"
"$X" git -C "$SB/origin.git" fetch --quiet "$CASE/origin.bundle" 'refs/heads/*:refs/heads/*'
"$X" git clone --quiet "$SB/origin.git" "$SB/home/dev/sample"
cd "$SB/home/dev/sample"
"$X" git remote set-head origin main
"$X" git worktree add --quiet .docket docket
"$X" git config --local extensions.worktreeConfig true
mkdir -p "$SB/home/dev/sample/.git/docket/empty-hooks"
"$X" git -C .docket config --worktree core.hooksPath "$SB/home/dev/sample/.git/docket/empty-hooks"

# Step C — migrate-to-docket.sh's step 5b, verbatim invocation shape (cwd = the repo).
"$X" env DOCKET_INTEGRATION_BRANCH=main DOCKET_BASH_PATH=/opt/homebrew/bin/bash \
  /opt/homebrew/bin/bash "$SB/home/dev/docket/scripts/ensure-claude-settings.sh" > "$SB/ensure-claude-settings.out" 2>&1
cat "$SB/ensure-claude-settings.out"

# Step D — PROVENANCE.md Step 7's real sync-agents.sh run on the committed configuration.
"$X" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/sync-agents.sh" \
  > "$SB/sync-agents-repo.out" 2> "$SB/sync-agents-repo.err"
grep -v -e 'no harness-specific model' "$SB/sync-agents-repo.err" || true
"$X" env DOCKET_BASH_PATH=/opt/homebrew/bin/bash /opt/homebrew/bin/bash "$SB/home/dev/docket/sync-agents.sh" --check \
  > "$SB/sync-check.out" 2> "$SB/sync-check.err"; echo "sync --check exit $?"
echo "--- git status (tracked changes must be none)"
"$X" git status --porcelain
echo "--- ignored"
"$X" git status --porcelain --ignored
[ -e "$SB/gh-calls.log" ] && { echo "FATAL gh called"; exit 97; }
echo "SB=$SB"
```

One run, exit 0. Results:

- Step A repeats Steps 2–3: `rev-parse HEAD` printed `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf`
  and the install ended `docket: install complete`. `make-home-tar.py` (SHA-256 `de52b56e…11458`,
  as above) run on `$SB2/home` wrote a tar **byte-identical** to the saved `home.tar` (`cmp`
  silent), so this sandbox's home, including the global `config.yml` that `sync-agents.sh` reads
  as a layer, equals the original's.
- Step B restores the bundle and replays `clone-config.txt` exactly as the test's `restoreCase`
  does.
- Step C printed `ensure-claude-settings: created .claude/settings.local.json and granted:
  Bash(git -C * push origin HEAD:main)`, the line the migration printed in Step 5.
- Step D's `sync-agents.sh` printed the 17 unpinned-agent warnings for the `agents` harness, the
  no-named-emitter warning and `done`, and nothing about `CLAUDE.md` or `.gitignore`: the committed
  dispatch block and ignore block were already what the tag writes. `sync-agents.sh --check` exited
  0. `git status --porcelain` was empty; `git status --porcelain --ignored` listed only `!! .claude/`
  and `!! .docket/`.
- The 17 wrappers: `docket-implement-next.md` is pinned `model: claude-opus-5` / `effort: high`;
  `docket-rebase-resolver.md` is the delegated wrapper (`model: inherit`, `docket.sh
  runner-dispatch … --runner codex --agent rebase-resolver --model gpt-5.6-sol --effort high`).
  No file holds a sandbox path.
- No `gh` call (`$SB2/gh-calls.log` never existed). `find ~/.claude/agents ~/.claude/skills
  ~/.claude/settings.json ~/.claude/CLAUDE.md ~/.zshenv ~/.config/docket -newer
  "$SB2/task-start.txt"` printed nothing.

Export:

```bash
OUT=<worktree>/testdata/bash-upgrade/v0.9.3
python3 "$SCRATCH/make-clone-tar.py" "$SB2/home" "$OUT/clone-files.tar"   # members: 20
tar -xOf "$OUT/clone-files.tar" | grep -c -e "$SB2" -e /tmp/claude-501 -e @@SANDBOX_HOME@@   # 0
python3 "$SCRATCH/make-clone-tar.py" "$SB2/home" "$SB2/again.tar" && cmp "$SB2/again.tar" "$OUT/clone-files.tar"   # identical
shasum -a 256 "$OUT/clone-files.tar"   # 02c9e32ac2ccfa3809629a47fbe816555c00c450dfafecdc66971d69cb914c4c
```

`$SCRATCH/make-clone-tar.py` (SHA-256
`d3aa64a8a65e7cb6d6972ffaf9924dabc17930361aa2f490d0271935763fa122`) is `make-home-tar.py` with its
root moved to the clone and its include list narrowed to `.claude`; the token rewrite still keys on
the sandbox home. Its full difference from `make-home-tar.py`:

```diff
2c2,3
< """make-home-tar.py SANDBOX_HOME OUT_TAR — scratch helper (never committed).
---
> """make-clone-tar.py SANDBOX_HOME OUT_TAR — scratch helper (never committed); make-home-tar.py with
> the root moved to the sample clone and INCLUDE narrowed to its ignored .claude/ folder.
14a16
> clone = os.path.join(home, "dev", "sample")
18,19c20
< INCLUDE = [".claude", ".cursor", ".codex", ".agents", ".opencode", ".config/docket",
<            "dev/docket/skills", ".zshenv"]
---
> INCLUDE = [".claude"]
45c46
<     p = os.path.join(home, top)
---
>     p = os.path.join(clone, top)
69c70
<         ti = norm(tf.gettarinfo(os.path.join(home, rel), arcname=rel))
---
>         ti = norm(tf.gettarinfo(os.path.join(clone, rel), arcname=rel))
72c73
<         rel = os.path.relpath(path, home)
---
>         rel = os.path.relpath(path, clone)
```

## What `home.tar` holds

| Folder | Written by | Contents |
|---|---|---|
| `.claude/` | `link-skills.sh`, `sync-agents.sh`, `ensure-docket-env.sh` | `skills/docket-*` (12 symlinks), `agents/docket-*.md` (17, including `docket-plan-writer.md`), `settings.json` with an `env` block setting `DOCKET_SCRIPTS_DIR` and `DOCKET_BASH_PATH` |
| `.cursor/` | same | `skills/` (12 symlinks), `agents/` (17), `rules/docket-dispatch.mdc` |
| `.codex/` | same | `skills/` (12 symlinks), `agents/docket-*.toml` (17). **Saved, never asserted.** |
| `.agents/` | same | `skills/` (12 symlinks, OpenCode's skill root), `agents/` (17, Claude-shaped, unpinned) |
| `.opencode/` | `sync-agents.sh` | `agents/docket-*.md` (17) |
| `.config/docket/` | `ensure-global-config.sh` | `config.yml` with the managed `runtime.bash` block |
| `dev/docket/skills/` | the tag checkout | the tree every skill symlink points at |
| `.zshenv` | `ensure-docket-env.sh` | the managed `DOCKET_SCRIPTS_DIR` / `DOCKET_BASH_PATH` export block |

Not saved: `dev/sample` (it is the bundle plus `clone-config.txt` and `clone-files.tar`), `.cache`, `.local`, and the rest
of `dev/docket`. So after a restore, `DOCKET_SCRIPTS_DIR` in `.zshenv` and `settings.json` points at
`@@SANDBOX_HOME@@/dev/docket/scripts`, which does not exist.

Like v0.9.2, v0.9.3 wrote **no** `~/.claude/CLAUDE.md` and no dispatch block anywhere under the
home. The repository, unlike the v0.9.2 one, does carry a dispatch block: `main:CLAUDE.md`, written
by `sync-agents.sh` because of the `agents:` opt-in.

## Records

| `records.txt` entry | Exercises |
|---|---|
| `docket:docs/adrs/0001-greeting-script-is-posix-sh.md` | ADR, `Accepted`, `change: 1`, made in main mode by the `docket-adr` flow, carried into the seeded root |
| `docket:docs/changes/active/0003-add-a-farewell-script.md` | proposed + spec (Brainstorm mode, made in main mode, carried by the seed), `related: [1]`, priority `high` |
| `docket:docs/changes/active/0004-add-a-license-file.md` | trivial proposed (`trivial: true`, no spec), made in docket mode |
| `docket:docs/changes/active/0005-read-the-default-greeting-name-from-the-environment.md` | deferred: minted by `mint-stub.sh` in docket mode (`discovered_from: [1]`, single-quoted `title:`), then the groom Defer exit |
| `docket:docs/changes/archive/2026-10-04-0001-add-a-greeting-script.md` | done #1, closed out in main mode before the migration: spec, plan, results, `adrs: [1]`, `pr:`, `reconciled: true`, empty `claimed_at:` |
| `docket:docs/changes/archive/2026-10-04-0002-document-the-greeting-script.md` | done #2: proposed in main mode, carried by the seed, then built and closed out in docket mode; trivial, `depends_on: [1]`, plan, results, `pr:`, empty `claimed_at:`; on `docket` only (`terminal_publish` was `false` then) |
| `docket:docs/changes/learnings/posix-sh-means-no-arrays.md` | learning from the main-mode finalize harvest; double-quoted `hook:` with `\"` escapes |
| `docket:docs/superpowers/specs/2026-10-04-add-a-farewell-script-design.md` | spec of 0003, back-link to `active/` |
| `docket:docs/superpowers/specs/2026-10-04-add-a-greeting-script-design.md` | spec of 0001, back-link re-stamped to `archive/` |
| `main:docs/adrs/0001-greeting-script-is-posix-sh.md` | the same ADR, kept on `main` by the migration (same blob as on `docket`) |
| `main:docs/changes/archive/2026-10-04-0001-add-a-greeting-script.md` | done #1, kept on `main` by the migration (same blob) |
| `main:docs/changes/learnings/posix-sh-means-no-arrays.md` | the learning, kept on `main` (same blob) |
| `main:docs/superpowers/specs/2026-10-04-add-a-farewell-script-design.md` | spec of 0003, kept on `main` (same blob) |
| `main:docs/superpowers/specs/2026-10-04-add-a-greeting-script-design.md` | spec of 0001, kept on `main` (same blob) |

Counts: on `docket`, 5 changes (3 active, 2 archived), 1 ADR, 2 specs, 1 learning; on `main`, the
5 records the migration keeps (1 archived change, 1 ADR, 2 specs, 1 learning), each the same blob
as its `docket` copy. Also on `docket`, but not records: `docs/changes/BOARD.md` (the tag's board
render) and the `docs/adrs/README.md` and `docs/changes/learnings/README.md` indexes. Also on
`main`: `README.md`, `.docket.yml`, `.gitignore` (the managed block from the migration),
`CLAUDE.md` (the dispatch block), `hello.sh`, `USAGE.md`, the ADR and learnings indexes, the two
plans and the two results files.

**Empty categories** (a saved case only covers what it contains): no in-progress, implemented,
blocked or killed change; no superseded or reversed ADR; no stacked change; no
`.docket.local.yml`; no change with a `## Run halted` section; no record published to `main` by
`terminal-publish.sh` (the `main` records are the ones the migration kept).

## Gaps

- **Interactive steps were hand-written** to the tag's prescribed shape, exactly as in the v0.9.2
  case: the specs and change bodies (`skills/docket-new-change/SKILL.md` Brainstorm and Trivial
  modes, using `change-template.md`); the claim, reconcile, `plan:`, `adrs:`, `implemented` +
  `pr:` + `results:` field writes (`skills/docket-implement-next/SKILL.md` Steps 2–7, including
  the `claimed_at` re-stamp); the plan, code and results commits on `feat/<slug>`; the ADR
  (`skills/docket-adr/SKILL.md` Create); the defer (`skills/docket-groom-next/SKILL.md` exit 4);
  the learning (`skills/docket-finalize-change/SKILL.md` step 2.5); and the two `.docket.yml`
  edits around the migration. Commit subjects use a `docket(<id>): …` style.
- **No pull request was opened or merged on GitHub.** `gh` was stubbed out. Each `pr:` holds an
  invented `https://github.com/fixture/sample/pull/<n>` URL, and the merge is a local
  `git merge --no-ff feat/<slug>` on `main` with GitHub's merge-commit subject, pushed to `origin`.
- **`terminal_publish` was `false`** while the changes closed out (the tag's default), so
  `terminal-publish.sh` published nothing. It is `true` only in the final configuration.
- **The tag's merge-date command** in `terminal-close-out.md` step 1 needs `--format=%ad` (see the
  v0.9.2 note); the driver uses that form from the start.
- **The per-clone ignored files were regenerated, not copied.** The first export (Step 8) left out
  the two ignored paths in the sandbox clone, and that sandbox was gone by the time they were
  needed: `.claude/settings.local.json` (the allow-rule written by `migrate-to-docket.sh` through
  `ensure-claude-settings.sh`) and `.claude/agents/docket-*.md` (the 17 per-repository wrappers
  `sync-agents.sh` generated because of the `agents:` opt-in, including the delegated
  `docket-rebase-resolver.md`). Step 10 rebuilt them from the tag in a new sandbox, against the
  saved bundle, by the same two tag commands. That sandbox's install reproduced `home.tar` byte for
  byte, and its `sync-agents.sh` run left every tracked file unchanged, so the inputs matched the
  original's. One difference remains: the clone the files sit in was restored from the bundle (as
  the test restores it), not carried through the original's history, and `settings.local.json`
  was written after the configuration commit rather than during the migration. Neither script
  reads the history; `ensure-claude-settings.sh` reads only the integration branch, passed
  explicitly as `main` exactly as `migrate-to-docket.sh` passes it.
- **The worktree-scoped `core.hooksPath`** is replayed through the `hooks-off` line, as in the
  v0.9.2 case.
