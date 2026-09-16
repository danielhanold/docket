---
id: 123
slug: 'uninstall-is-asset-and-repository-independent-proof-gated-by'
title: 'Uninstall is asset- and repository-independent, proof-gated by ownership, and retains the CLI'
status: 'Accepted'
date: '2026-09-16'
supersedes: []
reverses: []
relates_to: []
change: 323
---

## Context

Uninstall is the operation a user reaches for when the install is already in a state they do not trust — a broken config, an unreadable repository, a half-applied upgrade, or a machine whose repositories have moved or been deleted. An uninstall that loads configuration or repository state to decide what to remove would therefore be least able to run exactly when it is most needed, and its blast radius would depend on inputs the installer never created. The same applies to what it removes: the installer creates harness integrations, but the surrounding environment also contains a CLI binary a user put on their PATH, global configuration a user authored, contributor source checkouts, and repository-local setup — none of which the installer is entitled to delete, and some of which (the binary itself) the user needs in order to reinstall. A removal that also deleted the tool that performs removal and reinstallation would make the operation one-way.

## Decision

Uninstall and collect load NO configuration and NO repository state. Their sole authority is the recorded install state at `<data-root>/state/install.json` — if it is not recorded there, uninstall does not know about it and does not touch it.

Uninstall removes only the harness integrations that state records, and every removal is proof-gated by ownership: a recorded target is removed only once it is confirmed to be the thing the installer put there.

It explicitly RETAINS the CLI binary, global configuration, contributor source checkouts, and repository-local setup. There is no force mode and no purge mode: there is no supported way to ask uninstall to remove something the recorded state does not claim, or to bypass the ownership proof.

## Consequences

Removal is predictable — its blast radius is readable off one file and is the same whether the machine's repositories are healthy, moved, or gone — and it is reversible, because the retained CLI binary and global configuration are exactly what a reinstall needs. A valid but empty install state is a legitimate post-uninstall condition that a subsequent install proceeds from cleanly, so uninstall and reinstall compose.

The costs are real and intentional. Uninstall leaves residue: anything the installer did not create, or created before the current state format recorded it, survives and must be cleaned up by hand. A user who wants a total wipe has no command for it, and adding one later would reverse this decision rather than extend it. Ownership-proof failures leave targets in place rather than forcing them, so a tampered or externally replaced integration is reported, not removed.

For maintainers the binding rule is that uninstall's inputs may not grow: introducing a configuration read, a repository probe, or a `--force`/`--purge` escape hatch breaks the property that makes uninstall safe on a broken machine. Anything uninstall should be able to remove must first become part of the recorded install state.

## Alternatives considered

Read configuration and repository state to discover what to remove. Rejected: it makes uninstall fail or misbehave precisely in the broken-install case that motivates running it, and it lets removal reach things the installer never created.

Remove recorded targets without an ownership proof. Rejected: a path recorded earlier may since have been replaced by something the user owns; deleting it on the strength of the record alone destroys user data.

Remove the CLI binary along with the integrations. Rejected: it makes the operation one-way — the tool needed to reinstall is gone — and the binary's placement on PATH is a user decision, not an installer artifact.

Offer a `--force`/`--purge` mode for total removal. Rejected: it reintroduces the unbounded blast radius this decision exists to eliminate, and in practice would be reached for in exactly the confused states where its judgment is worst.

Treat an empty install state as an error. Rejected: an empty state is the correct, valid result of a completed uninstall, and a subsequent install must be able to proceed from it.
