<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0317 — Release packaging and four-harness acceptance](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0317-release-packaging-and-four-harness-acceptance.md)**
<!-- docket:backlink:end -->

# Release packaging and four-harness acceptance

**Change:** 0317 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-20 · **Status:**
Approved design

## Purpose and boundary

This change turns the complete Go engine into verifiable release-candidate bytes and proves that
those bytes can be installed, upgraded, and invoked directly through Claude, Codex, Cursor, and
OpenCode. It owns the last release-engineering layer before Docket self-hosts and cuts over.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are governing constraints. This
spec resolves only change 0317's independently deliverable scope. It does not reopen the supported
platforms, agent-first product boundary, repository compatibility contract, direct-host topology,
or deferred/dropped capability decisions in those documents.

In particular:

- changes 0305–0316 remain the sole owners of configuration, documents, domain policy, Git,
  transactions, installation of embedded assets, harness rendering, lifecycle workflows, process
  supervision, PRs, finalize, recovery, archive, reclaim, and stacks;
- change 0322 remains the owner of the repository-root `install.sh` source-development bootstrap;
- change 0323 remains the owner of uninstall and unreferenced version-tree collection; and
- change 0318 remains the owner of final self-hosting, production Bash removal, active
  documentation replacement, tag/release publication, and hard cutover.

Change 0317 consumes those landed seams. It neither adds a workflow operation to the `docket`
binary nor publishes a public release.

## Landed foundation and independently deliverable result

Changes 0304–0316 are complete. The repository already has:

- a `CGO_ENABLED=0` Go binary that cross-builds for Darwin/Linux on amd64/arm64 and exposes injected
  version, commit, and build-date identity;
- embedded versioned assets, global-only model/effort rendering, ownership-safe release installs,
  source-linked development installs, and read-only `install check` for all four harnesses;
- the complete retained planning, implementation, finalize, recovery, and stack lifecycle;
- native exact-status process supervision on Darwin and Linux; and
- direct native agent definitions and dispatch material for Claude, Codex, Cursor, and OpenCode.

The independently reviewable deliverable of 0317 is:

1. one deterministic candidate bundle containing four versioned archives, a sorted SHA-256
   manifest, and a rendered POSIX release downloader;
2. a non-publishing GitHub Actions candidate workflow that runs the whole suite, builds the bundle
   once, and executes install/upgrade smokes natively on all four target tuples;
3. downloader ownership, checksum, failure, and interruption behavior proven without Bash,
   Python, or Perl; and
4. recorded fresh-session direct-invocation acceptance for the same read-only retained workflow in
   all four native harnesses.

These outcomes are useful before public cutover: a candidate can be inspected and accepted as one
immutable unit, and change 0318 can later publish those accepted bytes without rebuilding them.

## Chosen release-candidate architecture

### Repository-owned packaging tool

Use a small repository-owned Go packaging command backed by an internal release package. It is
development/release tooling, not a second shipped product executable and not a public Go API. It
uses the standard library plus the Go tool through explicit argument arrays; it does not invoke a
shell, import workflow packages, or teach the production binary about GitHub Releases.

The command accepts, rather than infers:

- a source root;
- a safe release version matching the repository's `vMAJOR.MINOR.PATCH` plus optional prerelease
  spelling;
- the full source commit identity; and
- a source epoch from which it derives the UTC build date and all archive timestamps.

The caller proves the source checkout is clean and that the supplied identity names it. Keeping
that Git validation at the workflow boundary avoids duplicating change 0308's Git adapter or
making the package format depend on ambient repository state.

For each approved tuple—`darwin/amd64`, `darwin/arm64`, `linux/amd64`, and `linux/arm64`—the tool
runs a `CGO_ENABLED=0`, trim-path build of `./cmd/docket` and injects the landed build-information
variables. A release binary must report the exact requested version, full commit, and derived UTC
date through `docket version --json`; a mismatch fails packaging.

### Artifact contract

The complete output set is:

```text
docket_<version>_darwin_amd64.tar.gz
docket_<version>_darwin_arm64.tar.gz
docket_<version>_linux_amd64.tar.gz
docket_<version>_linux_arm64.tar.gz
checksums.txt
install.sh
```

Each archive contains exactly one regular executable named `docket`. Tar and gzip headers use
fixed source-epoch time, normalized numeric ownership, a portable `0755` mode, and deterministic
ordering. Host paths, usernames, working directories, and wall-clock time never enter the bytes.

`checksums.txt` contains one lowercase SHA-256 line for each distributable file except itself,
sorted bytewise by filename. The packager reopens every completed archive, validates its sole
entry and mode, hashes the bytes it will hand off, and refuses a missing, extra, duplicate, or
unsafe filename. Repeating the package operation with the same source, inputs, and Go toolchain
must reproduce the same archives and checksum manifest.

The authored downloader lives below a release-specific source path and is rendered into the
bundle as `install.sh` with this bundle's default version. The repository-root `install.sh` remains
byte-for-byte in its separate role as change 0322's contributor bootstrap.

## POSIX release downloader

### Interface and platform selection

The downloader is a `/bin/sh` program with three user inputs:

- `--version <vMAJOR.MINOR.PATCH[-prerelease]>`, defaulting to the version stamped into that copy;
- `--bin-dir <absolute-directory>`, defaulting to `${XDG_BIN_HOME}` or `$HOME/.local/bin`; and
- repeatable `--harness <claude|codex|cursor|opencode>`, passed unchanged as argument-vector values
  to the landed `docket install` operation.

It maps only Darwin/Linux and x86_64/amd64 or arm64/aarch64 onto the four canonical artifact names.
Every other operating system or architecture fails before a network request. The download base is
the repository's GitHub Release URL; tests and candidate acceptance may replace that base through
one narrowly documented test/mirror seam so unpublished Actions artifacts can be exercised without
creating a release.

The runtime dependency set is `/bin/sh`, `curl`, `tar`, and one supported SHA-256 provider:
`sha256sum` or OpenSSL. The script never invokes Bash, Python, Perl, `shasum`, `jq`, `eval`, or a
downloaded helper. A missing prerequisite produces an actionable failure before it changes the
destination.

### Integrity and ownership

The downloader creates a private templated scratch directory, downloads the checksum manifest and
the exact tuple archive, and requires exactly one syntactically valid manifest entry for the
archive. It verifies the archive before extraction, rejects links/special files/path traversal or
any member other than `docket`, and verifies the extracted executable before running it. A
checksum, archive, version, or tuple disagreement is a hard refusal; unverified bytes are never
executed or moved into the bin directory.

Release-binary ownership is separate from change 0311's asset ownership. After a successful first
install, the downloader atomically publishes a small Docket-owned machine record containing the
canonical destination, release version, and exact binary SHA-256. On later runs it may replace the
destination only when the current path and bytes still match that record, or when the destination
already equals the newly verified requested binary (the interrupted-completion case). An absent
record plus a different existing `docket` is an ownership conflict. The file is preserved and the
run refuses; there is no `--force` path and no attempt to execute an unknown binary to identify it.

Changing the recorded bin directory, adopting a development or Bash installation, removing an old
binary, and collecting immutable asset trees are not implicit upgrade behavior. They require an
explicitly separate operation and remain outside this change.

### Install and upgrade sequence

After all input, integrity, and ownership checks pass:

1. stage the verified executable beside its final destination so the final rename is on one
   filesystem;
2. run the staged executable's existing `docket install`, forwarding the selected harnesses as
   distinct arguments;
3. only after the embedded-asset transaction succeeds, atomically replace the owned destination
   with `mv -f`;
4. atomically publish or repair the release-binary ownership record; and
5. run the installed binary's read-only `docket install check` and return its result.

An ordinary asset-install failure leaves the prior binary untouched; change 0311's transaction
already rolls back any partial harness writes. A failure or interruption after asset publication
but before binary replacement may temporarily leave old binary/new asset state. Asset-protocol
guards fail closed if those versions are incompatible, and rerunning the same downloader converges:
the prior binary still matches its ownership record, the staged install is idempotent, and the
final rename completes. A failure after binary replacement but before ownership-record publication
also converges because the destination equals the newly verified requested binary.

The downloader never deletes the previous embedded version tree. Change 0323 owns reclamation of
unreferenced installer trees.

## Candidate workflow

Add one `release-candidate` GitHub Actions workflow. It runs for pull requests that touch the Go,
asset, installer, harness, release, or workflow surfaces and through an explicit manual dispatch
for a named source ref/version. It uses read-only repository permissions, no publication token,
and commit-pinned action dependencies.

The workflow has four phases:

1. **Source gate.** Check out full source identity, prove the tree/ref inputs, regenerate/check
   embedded assets, and run the repository's resolved whole-suite command. Any `OVER BUDGET:` line
   remains a finding to disposition under the existing suite contract.
2. **Package once.** Run the repository-owned packager once for the head candidate. For pull
   requests, also build a base candidate with a distinct safe prerelease version so the upgrade
   smoke has a real prior Go binary. Upload the complete candidate directories only as immutable
   Actions artifacts.
3. **Native tuple smokes.** Fan the head bundle—and the base bundle on pull requests—out to native
   Darwin/Linux amd64/arm64 runners. Runner labels are revalidated against current GitHub-hosted
   inventory during implementation; emulation or a cross-compile alone does not count as an
   on-target smoke.
4. **Evidence summary.** Require every tuple job and publish one machine-readable candidate
   summary naming the source identity, package checksums, tuple verdicts, and outstanding live-host
   acceptance. The summary is an Actions artifact, not a Docket lifecycle record or release.

No trigger creates or moves a tag, opens a GitHub Release, uploads release assets, edits release
notes, changes active installation documentation, or writes a Docket metadata branch. Those are
change 0318's publication/cutover actions. Its publish step consumes the already-accepted bundle;
it does not rebuild release bytes under a different environment.

## Automated smoke and failure evidence

### Package and downloader tests

Hermetic tests cover:

- safe version grammar, all four exact filenames, normalized archive metadata, and one executable
  member per archive;
- injected version/commit/date identity and deterministic same-input output;
- sorted complete checksum correspondence in both directions;
- corrupt/missing/duplicate checksum entries, corrupt/traversing archives, unsupported tuples,
  absent tools, unsafe bin directories, and download failure all refusing before execution;
- `sha256sum` and OpenSSL verification paths, with neither Bash/Python/Perl nor `shasum` reachable;
- unknown destination, drifted owned destination, changed recorded bin directory, and malformed
  ownership state preserving every existing byte; and
- failures injected before asset install, before binary rename, before state publication, and
  before final check, followed by a rerun that either restores the prior state or converges to the
  requested version.

The script tests use executable fakes and argument capture rather than the network. Every safety
guard has a mutation that removes its premise and makes the relevant test red.

### Native tuple smoke

Each tuple job starts from isolated HOME/XDG/bin roots and exercises the actual archive and
downloader for that tuple. It must prove:

- exact `version --json` identity and `diagnostic runtime --json` reporting the native supported
  tuple;
- checksum verification and extraction before first execution;
- explicit installation of Claude, Codex, Cursor, and OpenCode assets without any vendor process
  or language-runtime setup;
- `docket install check --json` is clean and a same-version rerun is idempotent;
- base-candidate to head-candidate upgrade leaves unrelated harness bytes unchanged and reports the
  head version/asset state; and
- the interruption point between staged asset install and binary rename converges on rerun.

The tuple matrix tests packaging and installation, not live model execution. A generated native
wrapper remains fixture evidence until a fresh vendor process loads it.

## Fresh-session four-harness acceptance

### One minimal retained scenario

Use the same read-only status scenario in every harness. The acceptance fixture is a disposable
Git-backed repository with one known build-ready change and a local authoritative remote. A setup
command creates a fresh copy and records its initial refs and repository-byte digest. The direct
CLI baseline against the candidate must return protocol v1, operation `status`, result `applied`,
and the fixture's expected ready ID.

For each of Claude, Codex, Cursor, and OpenCode:

1. install the candidate binary and that harness's native assets;
2. terminate any process that could have loaded the old agent/skill registry, then start a
   genuinely fresh native session (Cursor acceptance runs in the IDE, not a feature-lagging CLI
   proxy);
3. directly invoke the installed `docket-status` named agent through that harness's own dispatch
   surface—never through another harness or a runner shim;
4. ask the child to run the PATH-resolved `docket version --json` and read-only `docket status
   --repo-dir <fixture> --json`, with no maintenance sweep; and
5. record the native child-run evidence, candidate identity, expected ready ID, and unchanged
   before/after fixture refs and bytes.

This is the minimal scenario because it crosses every release-specific boundary—downloaded binary,
embedded assets, installed native definition, process-start loading, direct dispatch, PATH
resolution, authoritative Git read, and protocol output—without mutating lifecycle state or
re-exercising behavior owned by changes 0312–0316. Merely running `docket version` in the parent,
opening a generated file, or comparing a golden does not pass.

### Evidence and gate

Change 0317's results record carries one row per harness with:

- harness name, exact vendor version, and interactive/headless/IDE mode;
- host OS/architecture;
- candidate source commit and archive SHA-256;
- proof a fresh process was started;
- proof the native named child ran rather than the parent executing inline;
- observed version/status protocol fields and expected ready ID;
- unchanged fixture evidence; and
- pass/fail plus a sanitized transcript or durable evidence location.

All four rows are required. Vendor behavior and process-start loading are external truth, so no
in-repo test can promote a missing, stale-session, cross-harness, or ambiguous observation to a
pass. A failure blocks acceptance and is diagnosed against the exact recorded harness version and
mode; it does not widen this change into a compatibility wrapper, runner fallback, or harness
redesign.

## Alternatives considered

### GoReleaser-driven tag workflow

Rejected for this change. It reduces packaging code but adds another versioned release tool and
naturally couples build, tag, and GitHub Release publication. The program deliberately separates
candidate production/acceptance (0317) from publication/cutover (0318). A small deterministic
standard-library packager keeps that seam explicit and locally testable.

### Per-runner shell packaging

Rejected. Four independent `go build`/`tar` implementations would make archive identity depend on
runner defaults and multiply shell/platform behavior. Build once, then smoke the same bytes on each
native target.

### Treat wrapper goldens as harness acceptance

Rejected. Goldens prove renderer output only. Agent definitions and registries are loaded at vendor
process start, and direct-dispatch behavior is mode/version scoped. Fresh native sessions and
observed child runs are the only evidence this change accepts.

### Exercise a mutating lifecycle as the common live scenario

Rejected. A create/groom/claim/finalize scenario would be slower, credential-heavy, and would
retest behavior already owned by changes 0312–0316. Read-only status is sufficient to cross the
release and harness boundary while keeping the acceptance fixture deterministic and disposable.

## Explicit exclusions

Change 0317 does not:

- change configuration resolution/capabilities, document patching, domain policy, Git/repository
  transactions, renderers, installer asset semantics, workspaces, PRs, gates, lifecycle operations,
  finalize, recovery, reclaim, archive, stacks, or any skill behavior owned by changes 0305–0316;
- replace or broaden the source-development bootstrap from change 0322;
- add uninstall or immutable version-tree garbage collection from change 0323;
- select the final public version, create/move a tag, publish a GitHub Release, write release notes,
  replace active documentation, self-host Docket, capture migration learnings, remove production
  Bash/tests, or perform hard cutover from change 0318;
- add Homebrew, Windows, code signing/notarization, SBOM/provenance signing, a public Go library,
  plugins, cross-harness runners, or a Bash/Go fallback; or
- claim one harness/platform live run covers another harness or makes four-host execution a
  four-target cross product.

## Acceptance boundary

Change 0317 is designed when this spec is linked from the change. It is implemented only when the
same source identity produces one complete deterministic candidate bundle; every archive and
checksum is validated; the POSIX downloader installs and safely upgrades without Bash/Python/Perl;
all four native target smokes pass on the packaged bytes; and fresh Claude, Codex, Cursor, and
OpenCode sessions each directly execute the known-answer status scenario through their installed
named agent.

The change stops with accepted, non-public candidate bytes and recorded evidence. Public release,
self-hosting, Bash removal, documentation replacement, and hard cutover remain change 0318.
