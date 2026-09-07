<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0409 — Document remote-agent session setup using a locally built Docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0409-document-remote-agent-session-setup-using-a-locally-built-do.md)**
<!-- docket:backlink:end -->

# Remote-agent session setup and connector publication

## Goal

Give an agent starting in an ephemeral container, often on a different repository, a discoverable playbook that installs Docket from its public source through the tracked development installer and establishes the capabilities needed for its requested workflow.

Support two publication paths: authenticated shell Git and an authorized repository connector. The connector contract is harness-agnostic: it names required Git capabilities and invariants, never a product's tool namespace, SDK method, credential variable, or orchestration wrapper. GitHub is the repository host in the motivating example, not a requirement that the agent run in a particular harness.

This is documentation and maintained workflow guidance. It does not implement a new Docket transport, credential bridge, service, CLI operation, or automatic hook installer.

## Decisions and approved scope

- Change 0409 was explicitly selected by the user; priority remains medium and type remains docs.
- The user explicitly selected connector publication in addition to shell Git publication.
- The user required the connector design to be harness-agnostic.
- Use the existing tracked development installer and metadata operations. Do not reproduce their rendering, allocation, schema validation, or state transitions.
- Add a first-class, narrowly scoped connector publication procedure for proposed-change creation and grooming. Do not extend this procedure to claims, gates, PR publishing, merges, or arbitrary workflow transactions.
- There are no prerequisite changes or stacked branches. Related changes 0311, 0322, 0340, 0351, 0392, 0394, and 0399 provide existing install, stamping, reconciliation, capability, and schema behavior.
- Grooming stops at a linked spec. Implementation planning and execution occur later.

## Current evidence and constraints

Source inspected at main revision `0d1a7e1b36af36c1f805952cd3836e84dbad5b6f`.

The source is a Go module. Its current go.mod declares `go 1.26.0`, `toolchain go1.26.5`, two direct dependencies, and two indirect dependencies. Maintained instructions must point to the checked-out go.mod as authority rather than treating those values as permanent requirements.

This session successfully cloned public source, obtained Go, and ran the tracked development installer with the Codex harness. The resulting binary reported a real source commit and build date. The installer correctly skipped repository reconciliation because the consuming repository lacked an explicit agent_harnesses declaration. Repository preparation succeeded with a full clone.

The native change.create operation generated the change record and BOARD.md in one commit, but publication failed because shell Git could not obtain GitHub credentials. The authorized connector published the same generated tree and transaction message as a new commit with the same parent; a non-forced branch update succeeded, and both files were read back byte-for-byte. This proves that connector access and shell Git access are distinct and that tree-preserving transport is feasible. It does not establish a general-purpose Docket connector API or validate every harness.

Relevant source seams are the tracked installer, `internal/install/devmode.go` build identity handling, `internal/cli/install.go` repository reconciliation, and `internal/repository/transaction` candidate creation, push classification, cleanup, and idempotency. The current push-failure result does not expose a supported candidate-export operation. A failed candidate may survive only as an unreachable Git commit after its temporary worktree is cleaned up. The documented recovery must acknowledge this limitation, use positive attribution, and stop if the candidate cannot be identified or has been pruned.

Readiness is evaluated for the intended task. Installing files on disk does not demonstrate that the running harness loaded them. A successful read or repository.prepare does not prove publication permission.

## Documentation surfaces

1. Add `docs/install/remote-sessions.md` as the human-facing entry point: prerequisites, manual bootstrap, an opt-in startup example, readiness checks, and troubleshooting.
2. Link it from `docs/install/README.md`, `docs/install/install.md`, `docs/install/claude-code.md`, and `docs/install/codex.md`.
3. Add one canonical maintained reference under `skills/docket-convention/references/remote-sessions.md` for agent-facing startup and connector publication rules. The user guide links to that contract instead of maintaining a second copy of its concurrency or transaction rules.
4. Add concise discovery pointers in docket-convention, docket-new-change, and docket-groom-next. Reconcile their push-failure instructions with the explicitly authorized connector path for change.create and change.groom. Preserve all other refusal and capability contracts.
5. Keep provider/harness-specific configuration examples in the install pages. The normative connector reference must not require any particular connector name, tool prefix, JavaScript runtime, host approval syntax, or environment variable.

The bootstrap path must be discoverable by a remote agent before Docket is installed: an accessible installation page or supplied repository link leads to plain repository Markdown. Do not require an already registered Docket skill as the only entry point.

## Manual session bootstrap

### Establish source and consuming-repository identity

Resolve the consuming repository to an absolute path and retain its intended remote identity. Make the public Docket source available using the host's supported repository attachment or public Git mechanism where necessary; public visibility alone is not a promise that a host's Git proxy permits an unattached repository.

Clone Docket outside the consuming repository, for example at ~/docket. Use a complete source clone, retain its revision, and keep it available for the session because development-mode skills link back to that checkout.

An existing destination is not disposable. Verify that it is the expected checkout; preserve dirty or foreign directories and report them. Reuse the selected source revision during one session. Updating a reused checkout is an explicit source-selection action, not an unconditional reset or pull hidden in every prompt.

### Establish toolchain and network prerequisites

Require Git and a compatible Go bootstrap toolchain. If Go is absent, provide an actionable installation prerequisite for the host's OS and architecture using an approved package or official distribution. Do not bundle a new toolchain downloader.

Explain that automatic Go toolchain switching depends on the installed Go command and its GOTOOLCHAIN configuration. It may use a compatible toolchain already on PATH or download one. A newer selected toolchain need not equal the go.mod suggestion. Do not force a downgrade or claim automatic download with GOTOOLCHAIN=local.

Document access to the configured module proxy, checksum service, and any toolchain/download redirects as applicable. Never assume this session's network access is another host's allow-list, and do not disable verification to make installation work. Determine GitHub CLI requirements from the intended workflow; creating/grooming a change must not be made to require gh solely because other Docket workflows use it.

### Install through the tracked source installer

Preserve a concrete human-run Claude example in the user guide:

```sh
cd "$HOME/docket"
go run ./cmd/docket development install \
  --source "$HOME/docket" \
  --harness claude \
  --repo-dir /path/to/your/repo
```

Explain the corresponding Codex harness selection. Select only harnesses the host actually supports; a product's name is not sufficient evidence that it loads the standard CLI harness directories.

The installer owns building, stamping, linked skills, generated agent wrappers, and repository reconciliation. A bare go build is not a substitute. The from-source installer is the pre-binary bootstrap; after installation, maintained agent instructions use the capability catalog for runtime operations in accordance with ADR-0104.

Repository dispatch surfaces require explicit repository agent_harnesses opt-in. Document both committed .docket.yml and machine-local .docket.local.yml, preserving existing configuration and ownership checks. A --harness flag alone does not authorize repository configuration changes. Show the resulting tracked instruction/.gitignore changes to the user; do not automatically commit unrelated setup changes.

### Establish PATH and full history

Prefer ~/.local/bin on PATH. Verify resolution in the active shell and a subsequent independent tool shell. A child hook export cannot modify its parent's environment; use the host's documented environment-persistence mechanism or an existing writable executable directory selected through the installer's --bin-dir option. Do not require /usr/local/bin or root access.

For the consuming repository, check git rev-parse --is-shallow-repository. Fetch --unshallow only when true, then recheck. Confirm the default, integration, and metadata refs required by repository.prepare can be fetched. A narrow single-branch refspec may still need explicit ref fetching after unshallowing; do not blindly overwrite the user's fetch configuration.

Treat full history as a bootstrap precondition for these workflows, including Docket's full-ancestry idempotency scan. Do not claim that every shallow clone produces one universal refusal string. Preserve the actual diagnostic when history or refs cannot be obtained.

### Run readiness checks

Report three separate results in plain language:

- Binary and source assets are available: PATH resolves the intended installed binary, build identity matches the selected source, and linked assets exist.
- The requested workflow can run in this harness: required skills and named agents are actually registered, or the particular interactive workflow has its existing supported inline path.
- Metadata can be published: either authenticated shell Git is available or a connector satisfies the capability contract below.

Fetch and validate docket capabilities --json before runtime operations. Resolve version, schema, install.check when applicable, and repository.prepare from that catalog; obtain request shapes from the schema operation. Invoke repository.prepare independently, validate its protocol envelope and typed context, and honor every refusal. Init/migrate remain existing human-directed remedies.

A late install can leave the binary usable while newly generated agents remain unavailable. Require a fresh harness process when the host loads them at process start. Do not claim that clearing a conversation refreshes registration.

## Opt-in startup automation

Provide one complete, copyable consuming-repository startup example for Claude Code and host-neutral instructions for an equivalent pre-session setup step. The example reuses the manual sequence and is explicitly installed by the repository owner. It preserves existing hooks and settings, quotes paths, checks failures, and does not hide long installation behind a success exit.

Confine Claude-specific SessionStart settings and environment-persistence names to that example. The current official hooks reference describes persisting later Bash environment through CLAUDE_ENV_FILE; verify the relevant host mode and handle its absence explicitly. No such variable is part of the connector contract.

Fresh containers repeat installation. Repeated starts within one container reuse a verified source checkout, conditionally repair shallow history, and rerun the idempotent installer without discarding user changes. They do not create a Docket change, publish a candidate, or replay a previous request automatically.

Prefer a setup point before skills/agent discovery when the host provides one. If SessionStart runs too late for new registration, document the fresh-process requirement instead of promising readiness before the first prompt. Record tested harness version and mode. Codex guidance must distinguish actual runtime capabilities from assumptions about every product carrying that name.

## Harness-agnostic connector publication contract

### Eligibility and capabilities

This is an attended publication path for authorized change.create and change.groom metadata commits when ordinary shell Git publication is unavailable because its credentials/transport are missing. It does not turn an operation refusal into an approved mutation, evade host denial, or override repository protection.

Discover the connector through the current host's advertised tool capability surface. Names vary; the required semantics do not:

| Capability | Required semantics |
|---|---|
| Repository and ref reads | Resolve the intended repository and metadata branch to immutable commit and tree identities, and read committed paths/objects. |
| Object publication | Upload required blobs and trees while preserving exact bytes, modes, paths, and unchanged base entries. |
| Commit creation | Create a commit with the verified candidate tree, its original sole parent, and the original full message including transaction trailers. |
| Safe ref update | Atomically compare the expected old ref, or perform a server-enforced non-forced fast-forward update. A read followed by an unconditional update is insufficient. |
| Verification | Read the resulting commit, ancestry, tree and changed blobs from the remote to establish publication and content identity. |

An API that only replaces individual files in separate commits is insufficient for multi-file metadata publication. Missing capabilities produce a precise unavailable result before any branch update. Authentication/connection is handled through the host's advertised connector mechanism; never extract connector credentials into the shell.

Support only repositories/object formats for which local and remote tree identity can be compared faithfully. Do not silently normalize line endings, encodings, executable bits, or symlinks.

### Let Docket author the candidate

Use the installed binary and catalog-resolved operation, with its schema-defined request. For change.create, retain a stable request_id and exactly the same request on replay. For change.groom, retain the requested change's path and exact entity version from status. Do not invent a request_id field where the schema has none.

Retain the validated request, operation, starting remote metadata revision, and an inventory of existing candidate Git commits before the invocation. Limit this procedure to a task-isolated local clone without concurrent local metadata writers. Remote concurrency is handled separately.

Allow the normal operation to finish. If it succeeds, verify that success; there is no connector publication to perform. If it refuses validation, capability, topology, ownership, or entity version, surface the refusal and stop. The connector path applies only after the native operation successfully authored a commit but failed at publication because shell transport/authentication is unavailable.

Identify exactly one surviving commit produced by this invocation. Attribute it using the before/after Git object inventory, original parent, operation and transaction trailers, receipt, expected changed paths/content, and request_id/request digest when present. Parse real Git trailers, not body-text matches. The latest unreachable commit, a timestamp, a matching title, or an allocated ID alone is insufficient. If attribution is ambiguous or the object was pruned, stop and report that no recoverable candidate is available; do not reconstruct metadata manually.

Preserve the exact candidate commit object and its reachable tree/blobs in a local Git bundle or equivalent faithful checkpoint outside tracked repository content for the duration of publication. Record its SHA, original parent, tree, full message, and owned changed paths. Do not alter transaction internals or rely on a private temporary worktree still existing after cleanup.

### Publish one complete tree

1. Read the remote metadata ref through the connector and confirm it equals the candidate's original parent. If it changed, discard this publication attempt, rerun repository.prepare, reread the target through status, and have Docket produce a fresh candidate only if the requested transition is still valid. Never reparent the stale tree or hand-merge BOARD.md.
2. Upload only necessary Git objects over the original parent tree. Preserve every unchanged entry and the exact generated changed entries.
3. Require the returned complete tree identity to equal the local candidate tree. If it differs, stop before ref mutation.
4. Create the remote commit with that tree, the original sole parent, and the complete original commit message, including Docket transaction, operation, request and receipt trailers where present. Do not manufacture or rewrite a receipt. Connector-generated author/committer metadata may change the commit SHA; report the published SHA and do not claim the local and remote commit IDs are identical.
5. Update only the metadata branch using expected-old compare-and-swap when offered, otherwise an explicitly non-forced, server-validated fast-forward update. The remote commit must still be a direct child of the originally observed parent. A concurrent branch advance makes this update non-fast-forward and must be rejected. Never enable force, use an unconditional replacement, or retry with a different parent.
6. Read back the published commit/tree and each changed blob, and verify that the commit is reachable from the current metadata branch. A later legitimate descendant is acceptable; a missing or divergent candidate is not success.

Object uploads and creation of an unattached commit do not mean the change is published. Only the safe ref update plus remote verification establishes that outcome. Preserve connector policy denials and follow the host's existing recovery mechanism; missing shell credentials and an explicit denial are different conditions.

### Lost responses, retries and handoff

After an ambiguous connector result, read the ref and ancestry before retrying. If the candidate commit is already reachable, verify its tree, message, and blobs and report success without another publication.

For change.create, preserve the stable request identity and transaction trailers so a subsequent native retry recognizes the published request. Verify that replay returns the existing change rather than allocating a second one.

For change.groom, reprepare and reread status plus the linked spec. If the intended spec and generated links are already present, report completion; do not invoke another groom with a stale entity version. A different spec or intervening state transition requires a fresh decision rather than overwriting it.

After confirmed publication, synchronize the local metadata worktree through repository.prepare, inspect status, and retain the connector-published revision as the durable result. Report the native attempt's transport failure and the separately verified connector success accurately; do not relabel the original CLI response as applied.

This is a transport recovery procedure, not an export/import product interface. If build-time validation shows candidate attribution cannot be made reliable for a supported operation, report that limitation and return the design for revision rather than silently inventing a CLI, weakening attribution, or claiming the connector path is supported.

## Error behavior

Every failure identifies the unmet prerequisite and the next supported action. Missing Go, denied download, foreign checkout, unresolved PATH, unavailable history, missing harness opt-in, unregistered agents, insufficient connector semantics, non-attributable candidate, tree mismatch, and concurrent ref movement must remain distinguishable.

No branch update occurs after a validation refusal, mismatch, ambiguous candidate, or insufficient atomicity. No silent success on an install/hook failure. No credential material in command output, files, commit messages, or documentation examples.

## Validation and acceptance

The implementation includes runnable examples and a concise validation record, not only wording assertions. Use the repo's existing validation facilities; do not add a new general-purpose bootstrap or connector framework.

- Run the documented installer from a clean source clone into a disposable home against a separate already initialized consuming repository. Verify source-linked assets and stamped identity.
- Cover missing Go, automatic toolchain selection disabled, and download failure with actionable diagnostics. Network-denial cases can use controlled fixtures rather than changing the host's permissions.
- Verify PATH in a later tool shell, an explicit --bin-dir example, and failure when the documented persistence mechanism is unavailable.
- Exercise full and shallow consuming clones, an already-unshallowed repeat, and a narrow refspec. Preserve dirty/foreign source destinations.
- Exercise missing and explicit repository harness opt-in; verify ownership and unrelated configuration survive reruns.
- Test startup example syntax and repeat behavior. Separate on-disk generation evidence from actual fresh-process registration evidence; record which harness versions/modes were tested. Require the relevant fresh-process check before claiming that route fully supported.
- For both change.create and change.groom, force shell push failure in a controlled disposable repository and demonstrate unique candidate attribution, checkpoint preservation, exact tree publication, and all generated views in the same commit.
- Use a connector implementation or test double with different tool names to demonstrate that the normative procedure depends on semantics, not one harness/tool namespace.
- Reject changed file bytes/modes, altered trailers, an ambiguous candidate, a pruned candidate, a connector lacking safe ref updates, and a remote ref that advances before publication. Confirm no target-ref mutation.
- Cover lost successful ref-update responses, already-reachable commits, create replay without duplicate allocation, and groom readback without a second write.
- Verify documentation links, catalog/schema use, consistency with the amended convention, and all existing required repository gates at implementation time.

Grooming validation is limited to source review, metadata/spec integrity, and published readiness. The above implementation acceptance work is not claimed completed by this spec.

## Non-goals

No new transport implementation, CLI candidate-export command, credential bridge, hosted service, automatic migration, unattended connector publication, automatic repository hook installation, broad mutation recovery, or extension of merge/gate/claim semantics. No manual change allocation, hand-rendered metadata, multi-commit file-by-file publication, or force push. No fixed permanent Go version or universal assertion about remote network allow-lists.

## References

- Existing Docket install and harness guides under docs/install.
- ADR-0104 for capability discovery and change 0399 for schema discovery.
- Learnings: generated-artifact-loaded-at-process-start; harness-behavior-is-mode-and-version-scoped; agent-executed-markdown-is-code; verify-the-claim.
- [Go toolchain selection](https://go.dev/doc/toolchain): automatic selection is configurable and may select an installed or downloaded compatible toolchain.
- [Claude Code hooks reference](https://code.claude.com/docs/en/hooks): source for the isolated SessionStart/environment-persistence example, to be rechecked for the tested mode at implementation.
