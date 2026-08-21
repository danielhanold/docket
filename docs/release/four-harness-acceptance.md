# Four-harness fresh-session acceptance — the human merge-gate procedure

This is the human merge-gate checklist for change 0317. The fresh-session live acceptance is
external truth: vendor behavior and process-start loading of agent/skill registries cannot be
promoted to a pass by any in-repo test. This document is the procedure a human runs by hand against
a candidate bundle; it is **not** run by this task, and the pass/fail evidence it produces is
recorded in the change's results record (a metadata-branch artifact), not here.

Each harness section below is a **state to reproduce**, not a conclusion to confirm. Perform the
steps, observe the actual output, and record what you saw. Do not pre-fill a row and then look for
agreement.

---

## Fixture: one disposable, deterministic scenario

The same read-only status scenario is used in every harness. The acceptance fixture is a disposable
Git-backed repository with one known build-ready change and a local authoritative remote. A setup
command creates a fresh copy and records its initial refs and repository-byte digest so that the
before/after comparison at the end of each harness run is exact.

**Setup command (spelled out with `git` + the candidate binary):**

```sh
# 0. Choose a disposable location. Nothing here is shared with a real repo.
FIXTURE_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/docket-accept.XXXXXX")"
REMOTE="$FIXTURE_ROOT/authoritative.git"     # local authoritative remote
WORK="$FIXTURE_ROOT/fixture"                 # the working fixture repo
CANDIDATE=/absolute/path/to/candidate/docket # the candidate binary under test

# 1. Create the local authoritative remote (bare) and a working copy.
git init --bare "$REMOTE"
git clone "$REMOTE" "$WORK"

# 2. Seed the fixture with exactly one known build-ready change through the
#    candidate binary's own lifecycle operations, then push so the remote is
#    authoritative. (Use the candidate's create/groom/claim operations to reach
#    build-ready state for one change with a known id — record that id below as
#    the EXPECTED READY ID.)
#    ... candidate-driven seeding of one build-ready change ...
git -C "$WORK" push origin HEAD

# 3. Record the initial state that MUST be unchanged after every harness run.
git -C "$WORK" for-each-ref > "$FIXTURE_ROOT/refs.before"
git -C "$WORK" rev-parse HEAD >> "$FIXTURE_ROOT/refs.before"
# Repository-byte digest: a stable content digest over the repository bytes.
( cd "$WORK" && find . -type f -not -path './.git/objects/pack/*' -print0 \
    | sort -z | xargs -0 shasum -a 256 ) | shasum -a 256 > "$FIXTURE_ROOT/bytes.before"
```

Record, for reuse across all four harness sections:

- **EXPECTED READY ID** — the id of the one build-ready change seeded in step 2.
- **`refs.before`** — the recorded initial refs (and HEAD).
- **`bytes.before`** — the recorded initial repository-byte digest.

The scenario is deliberately read-only status. It crosses every release-specific boundary —
downloaded binary, embedded assets, installed native definition, process-start loading, direct
dispatch, PATH resolution, authoritative Git read, and protocol output — **without** mutating
lifecycle state or re-exercising behavior owned by changes 0312–0316.

---

## CLI baseline (direct candidate binary)

Before any harness, prove the candidate binary itself reads the fixture correctly. Run the direct
CLI against the candidate:

```sh
"$CANDIDATE" status --repo-dir "$WORK" --json
```

The direct CLI baseline against the candidate MUST return:

- protocol **v1** (`"protocol_version": 1`);
- operation **`status`** (`"operation": "status"`);
- result **`applied`** (`"result": "applied"`); and
- the fixture's **expected ready ID** in the ready queue (`"ready": [ <EXPECTED READY ID> ]`).

If the baseline does not match, stop: the fixture or the candidate is wrong, and no harness row can
be trusted until it is fixed.

---

## Per-harness procedure

Do the following for **each** of Claude, Codex, Cursor, and OpenCode. Each is a state to reproduce,
observed live in a genuinely fresh native session — not a golden to compare and not a parent-inline
shortcut.

### Claude

1. **Install** the candidate binary and Claude's native assets via the bundle downloader —
   `install.sh --harness claude` from the candidate bundle (which installs the candidate `docket`
   binary and then runs the landed `docket install claude` embedded-asset transaction).
2. **Fresh session.** Terminate any process that could have loaded the old agent/skill registry,
   then start a genuinely fresh native Claude session so the newly installed `docket-status`
   definition is loaded at process start.
3. **Direct native dispatch.** Directly invoke the installed `docket-status` named agent through
   Claude's own dispatch surface — never through another harness and never through a runner shim.
4. **Child-run reads.** Have that native child run the PATH-resolved `docket version --json` and the
   read-only `docket status --repo-dir "$WORK" --json`, with **no maintenance sweep**.
5. **Record the evidence row** (into the results record):
   - harness name (**Claude**) + exact vendor version + mode (interactive/headless/IDE);
   - host OS/architecture;
   - candidate source commit + archive SHA-256;
   - proof a fresh process was started;
   - proof the native named child ran rather than the parent executing inline;
   - observed `version`/`status` protocol fields + the expected ready ID;
   - unchanged before/after fixture refs and bytes (`refs.before`/`bytes.before` re-derived after
     the run and compared equal); and
   - pass/fail + a sanitized transcript or durable evidence location.

### Codex

1. **Install** the candidate binary and Codex's native assets via the bundle downloader
   (`install.sh --harness codex`).
2. **Fresh session.** Terminate any process holding the old registry, then start a genuinely fresh
   native Codex session.
3. **Direct native dispatch.** Directly invoke the installed `docket-status` named agent through
   Codex's own dispatch surface — never another harness, never a runner shim.
4. **Child-run reads.** Have the child run the PATH-resolved `docket version --json` and read-only
   `docket status --repo-dir "$WORK" --json`, with no maintenance sweep.
5. **Record the evidence row** — same fields as above, harness name **Codex**.

### Cursor

1. **Install** the candidate binary and Cursor's native assets via the bundle downloader
   (`install.sh --harness cursor`).
2. **Fresh session.** Terminate any process holding the old registry, then start a genuinely fresh
   native Cursor session. **Cursor acceptance runs in the IDE, never a feature-lagging CLI proxy.**
3. **Direct native dispatch.** Directly invoke the installed `docket-status` named agent through
   Cursor's own (IDE) dispatch surface — never another harness, never a runner shim.
4. **Child-run reads.** Have the child run the PATH-resolved `docket version --json` and read-only
   `docket status --repo-dir "$WORK" --json`, with no maintenance sweep.
5. **Record the evidence row** — same fields as above, harness name **Cursor**, mode **IDE**.

### OpenCode

1. **Install** the candidate binary and OpenCode's native assets via the bundle downloader
   (`install.sh --harness opencode`).
2. **Fresh session.** Terminate any process holding the old registry, then start a genuinely fresh
   native OpenCode session.
3. **Direct native dispatch.** Directly invoke the installed `docket-status` named agent through
   OpenCode's own dispatch surface — never another harness, never a runner shim.
4. **Child-run reads.** Have the child run the PATH-resolved `docket version --json` and read-only
   `docket status --repo-dir "$WORK" --json`, with no maintenance sweep.
5. **Record the evidence row** — same fields as above, harness name **OpenCode**.

---

## Evidence row (recorded once per harness)

Each harness row in change 0317's results record carries:

- harness name, exact vendor version, and interactive/headless/IDE mode;
- host OS/architecture;
- candidate source commit and archive SHA-256;
- proof a fresh process was started;
- proof the native named child ran rather than the parent executing inline;
- observed version/status protocol fields and the expected ready ID;
- unchanged fixture evidence (before/after refs and repository bytes equal); and
- pass/fail plus a sanitized transcript or durable evidence location.

---

## The gate

**All four rows are required.** Vendor behavior and process-start loading are external truth, so no
in-repo test can promote a **missing**, **stale-session**, **cross-harness**, or **ambiguous**
observation to a pass. A row is a pass only when a genuinely fresh native process, dispatched
through that harness's own surface, ran the named child and produced the expected protocol fields
and ready ID against an unchanged fixture.

A failure **blocks acceptance**. It is diagnosed against the exact recorded harness version and
mode; it does **not** widen change 0317 into a compatibility wrapper, a runner fallback, or a
harness redesign.

---

## What merely running `docket version` in the parent does NOT prove

Running `docket version` in the parent session, opening a generated file, or comparing a golden
**does not pass** this gate. None of those cross the boundaries the acceptance exists to test:

- it does not prove the candidate binary was resolved on `PATH` inside a **native child**, rather
  than the parent executing the command inline;
- it does not prove a **genuinely fresh** vendor process loaded the newly installed agent/skill
  registry at process start (a stale session may still be serving the old definitions);
- it does not prove the installed `docket-status` named agent was dispatched through **that
  harness's own** dispatch surface, rather than another harness or a runner shim;
- it does not prove the read-only `status` operation observed the authoritative Git state and
  returned the expected ready ID; and
- it does not prove the fixture's refs and repository bytes were **unchanged** by the run.

The results record's human-verify section copies these items with per-row checkboxes, so each of the
four harness rows is verified against this same non-pass list before acceptance.
