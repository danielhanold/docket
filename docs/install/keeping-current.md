# Keeping docket current

**Installed from a release?** Updating means re-running the downloader for the new version: fetch
and verify that release's `install.sh` and `checksums.txt` as in [Installing docket](install.md),
then run `sh install.sh --version <new version> --harness <name>`.

**Running from a source checkout?** Every time you pull a new version — a `git pull` on `main` or a
checked-out release tag — re-run `install.sh`. It is the catch-all: it applies whatever the new
version needs on your machine, and it is idempotent, so running it when nothing changed is a no-op.

```bash
cd ~/dev/docket
git fetch --tags && git pull
bash ~/dev/docket/install.sh        # always — not only when something looks broken
```

Pulling alone is **not** enough. Skills are symlinks, so those update the moment you pull — but the
rest of docket's on-disk footprint is generated or persisted, and only an install run refreshes it:

- **Agent wrappers are generated copies**, not symlinks — they bake in the resolved model and
  effort. A version that adds a subagent, renames one, or changes a pin lands only when the
  installer reconciles the wrappers.
- **New harness support**, and any harness you installed since last time, gets its `skills/`
  symlinks and `agents/` wrappers only on the next install run.
- **Retired global dispatch blocks and reconciled repository surfaces** land on this run too — which
  is why the recursion-guarded wrappers you are pulling only take effect after it, in a freshly
  started harness process.

Re-running the install is **in addition to** anything the release notes call for, never a
substitute. A release may also carry a step for each repository — a `docket repository repair`
run, a `.docket.yml` key to add, a remedy commit to land — listed in the notes for that version. Do
the machine-level `install.sh` first, then the repository steps.

## Automatic cleanup of old versions

Every **successful or no-op** install also runs docket's best-effort version
collection, so the old asset trees a new version supersedes are reclaimed as you
update — you never sweep them by hand on the happy path. A cleanup warning does
**not** undo the install: the install is already recorded as successful, and if a
pass cannot finish docket reports a `collection-pending` warning naming the exact
retry command, `docket install collect`. See
[Reclaiming old version trees](install.md) for the report categories and recovery.
