# Keeping docket current

**Every time you pull a new version — a `git pull` on `main` or a checked-out release tag — re-run
`install.sh`.** It is the catch-all: it applies whatever the new version needs on your machine, and
it is idempotent, so running it when nothing changed is a no-op.

```bash
cd ~/dev/docket
git fetch --tags && git pull        # or: git checkout v0.8.0
bash ~/dev/docket/install.sh        # always — not only when something looks broken
```

Pulling alone is **not** enough. Skills are symlinks, so those update the moment you pull — but the
rest of docket's on-disk footprint is generated or persisted, and only an install run refreshes it:

- **Agent wrappers are generated copies**, not symlinks — they bake in the resolved model and
  effort. A version that adds a subagent, renames one, or changes a pin lands only when the
  installer reconciles the wrappers.
- **New harness support**, and any harness you installed since last time, gets its `skills/`
  symlinks and `agents/` wrappers only on the next install run.
- **Managed global config** in `~/.config/docket/config.yml` is back-filled non-destructively by the
  same run.
- **Retired global dispatch blocks and reconciled repository surfaces** land on this run too — which
  is why the recursion-guarded wrappers you are pulling only take effect after it, in a freshly
  started harness process.

Re-running the install is **in addition to** anything the release notes call for, never a
substitute. A release may also carry a per-repo step — a `docket repository migrate` run, a
`.docket.yml` key to add, a remedy commit to land — listed in the notes for that version. Do the
machine-level `install.sh` first, then the per-repo steps.
