# auto-capture — the full shared definition

The mechanics behind the convention's *Auto-capture (shared definition)* summary — read before
minting or suppressing a discovered stub. Loaded on demand from `docket-convention/SKILL.md`;
sibling files are not auto-loaded with the skill.

## Per discovery

**Per discovery** (after the materiality bar): assign exactly one type from `CHANGE_TYPES` — the
model classifies, the script never infers (ADR-0012). `AUTO_CAPTURE_ENABLED: false` ⇒ report, mint
nothing. Enabled but the type is outside `AUTO_CAPTURE_TYPES` (the literal `all`, or a subset) ⇒
mint nothing, report it as **policy-suppressed**. Enabled and admitted ⇒ `mint-stub --type`. Every
outcome keeps ADR-0045's best-effort posture. **Type filtering runs before the cap is consumed** —
a suppressed candidate must never spend a mint slot; dedup stays after admission.

## Materiality bar

Mint only for *actionable follow-up work that would be its own change / PR*
("would a human file this as a `docket-new-change`?"). A build lesson → the **learnings** harvest;
drift inside the current change → the **reconcile log**; a bare observation → the run report.

## The deterministic mint

**The mint itself is deterministic** (ADR-0012 — the model judges *what*, the script does the mint):
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mint-stub --changes-dir .docket/<changes_dir>
--title <title> --type <type> --body-file <file> --discovered-from <this change's id> --minted <n so far>` (in
`docket`-mode; in `main`-mode, `--changes-dir <changes_dir>` — the metadata worktree IS the primary
tree) — one stub per call, `--body-file` **must start with `## Why`**, contract in
`scripts/mint-stub.md`. **`<n so far>` is the running count across the whole run on a single
change, never reset per mint site** — a skill with two mint sites (`docket-implement-next`'s
reconcile and review) carries the total forward. (`docket-status`'s sweep scopes it per swept
change — see its SKILL.md.) It owns dedup, id allocation, the template write, and the CAS push;
**exit 3** = duplicate skipped, **exit 4** = cap (3) reached, **exit 1** = a real error (push
failure, malformed body, retry exhaustion). Every skip, overflow, and exit-1 failure is **surfaced
in the run report, never silently dropped** — but none is fatal: **auto-capture is best-effort and
must never abort the change being built**, because capture is a courtesy while the change is the
job. Minting is a metadata-worktree write only — it never touches the running change's own
claim/branch/PR state.
