# Design: migrate-to-docket.sh targets the invoking repo ($PWD)

**Status:** design (brainstormed 2026-06-04)
**Change:** 0003
**Related:** change 0002 (introduced `migrate-to-docket.sh`); ADR-0002 (docket-mode default + refuse-and-migrate bootstrap)

## 1. Context / problem

`migrate-to-docket.sh` (shipped by change 0002) opens with `cd "$SCRIPT_DIR"`, so it migrates the repo it physically lives in — the **docket repo itself**. It is also not distributed to consuming repos (the *skills* are symlinked globally by `link-skills.sh`; the script is not). So migrating any **other** repo cannot use the script as-is. Real instance: `~/dev/obsidian-wiki` was migrated to docket-mode by **manual staged git steps**, and one step (the `.gitignore` `.docket/` entry — the script's step 5) was nearly missed and only caught in verification. See memory `docket-migration-tool-not-distributed`.

## 2. Decision

Retarget the script to operate on **the git repo containing the invocation directory (`$PWD`)**, not its own location. Reachability stays **minimal**: invoke docket's single copy by absolute path from within the target repo —

```
cd <target-repo> && bash ~/dev/docket/migrate-to-docket.sh
```

Because the script now acts on **whatever repo you are standing in** — and it does branch surgery + prune + push — guard it with a **confirmation prompt** (bypassable for automation).

Rejected (out of scope, §5): global distribution (symlink / PATH install) and skill-ifying the migration. The script stays docket-repo-resident; only its **target resolution** and a **confirm guard** change.

## 3. Design

- **Target resolution.** Replace `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; cd "$SCRIPT_DIR"` with:
  ```bash
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" \
    || die "not inside a git repo — cd into the repo you want to migrate, then re-run."
  cd "$REPO_ROOT"
  ```
  `SCRIPT_DIR` / `BASH_SOURCE` are no longer used — the script's own location is irrelevant; it migrates `$PWD`'s repo.
- **Confirmation guard.** After the resolved-config banner, print `Target repo: <REPO_ROOT>` (plus the resolved `integration_branch` and `metadata_branch` target), then prompt `Migrate this repo to docket-mode? [y/N] ` reading from **`/dev/tty`** (so a piped or empty stdin cannot silently auto-confirm); abort unless the answer is `y`/`yes`. A **`--yes`/`-y`** flag, parsed at startup, skips the prompt for non-interactive use.
- **Unchanged:** all preconditions (clean tree; `require_ref` on the integration ref; abort-if-`origin/docket`-exists) and the seed → prune → `.gitignore` → idempotency logic. They already operate relative to the resolved repo root + config, so they work as-is once `cd "$REPO_ROOT"` points at the right repo.

## 4. What changes (touch-points)

- **`migrate-to-docket.sh`** — target resolution (§3); `--yes`/`-y` flag parse; confirmation prompt; update the header-comment usage block to `cd <target-repo> && bash /path/to/docket/migrate-to-docket.sh`.
- **`tests/test_docket_metadata_branch.sh`** — add assertions: resolves the target via `git rev-parse --show-toplevel`; no longer contains `cd "$SCRIPT_DIR"`; a `--yes`/`-y` bypass exists. The existing `migrate-to-docket.sh` assertions (exists, executable, creates orphan, prunes) stay green.
- **`README.md`** — migration section: the new invocation (run from *within* the target repo) + the confirm/`--yes` note; drop any wording implying the script only migrates the docket repo.

## 5. Out of scope

- **Distributing** the script (symlink/PATH install) or making migration a **skill** — declined; reachability stays "invoke by absolute path." Revisit as a separate change if it becomes painful in practice.
- **Auto-running** migration from the bootstrap guard — the guard still STOPs and points to the script; "refuse, never auto-migrate" (ADR-0002) stands.
- The **migration's behavior** (seed/prune sets, idempotency split, `ls-tree` probes) — unchanged.

## 6. Testing

TDD-for-docs/script: extend `tests/test_docket_metadata_branch.sh` with the §4 assertions; `bash -n migrate-to-docket.sh` clean; at build time, a real exercise — run it from a **subdirectory** of a throwaway repo to confirm `$PWD`→`--show-toplevel` resolution, the confirmation prompt, and the `--yes` bypass.

## 7. Open questions

None. An env-var alias for the bypass (e.g. `DOCKET_MIGRATE_YES=1`) is a build-time nicety, not a design decision.
