#!/usr/bin/env bash
# scripts/lib/docket-agent-scope.sh — the SINGLE reader of an agent source's declared
# `worktree-scope:` (change 0208). SOURCE this; declaring one function is its only effect — no
# writes, no git, no network, no output at source time.
#
# Provides:
#   agent_worktree_scope FILE — the declared scope (`feature` | `metadata`), empty when FILE is
#                               unreadable or declares no such key IN ITS FRONTMATTER.
#
# WHY A LIB. Two trees read this key: sync-agents.sh at GENERATION time (validate_agent_scopes,
# which refuses to write wrappers for an undeclared agent) and scripts/runner-dispatch.sh at
# DISPATCH time (the `--worktree` requirement and the main-worktree rejection). Both readings must
# be the same reading. The VALUE semantics cannot drift — sync rejects anything but feature or
# metadata — but the EXTRACTION can, and asymmetrically: a change to the key's spelling or to the
# whitespace tolerated after the colon, made in the generator alone, fails loudly there while the
# facade silently resolves every agent to metadata scope, which disarms both delegation gates
# together. One implementation removes the direction that fails quietly.
#
# ANCHORED, per AGENTS.md ("Anchor a frontmatter-field edit to the first `---…---` block, never a
# bare column-0 line match") — the read side of the same rule. It is not hypothetical here: the
# sources whose BODIES most plausibly discuss worktree scope are exactly the feature-scoped agent
# wrappers, whose prose is about where a worker must run. With an unanchored read, a source that
# LOST its frontmatter declaration does not read as absent — it reads as whatever its body prose
# says, so generation's absence refusal never fires and the facade arms or disarms its gates on a
# sentence. That is also the shape docket-frontmatter.sh's selection rule prescribes for any key
# that CAN be absent, and `worktree-scope:` is exactly such a key at the facade — a non-built-in
# agent has no source at all.
#
# WHY IT DOES NOT DELEGATE TO fm_field. docket-frontmatter.sh is the repo's canonical anchored
# reader and would otherwise be the right callee, but it is not BOOTSTRAP-COMPATIBLE: its
# `declare -gA` fails under the system Bash macOS ships (3.2), and this file is sourced by
# sync-agents.sh, which tests/test_sync_agents_defaults.sh runs under `/bin/bash` expecting rc=0
# (its "0051 rider: empty scan_dirs run succeeds under /bin/bash" assert). Under `set -e` that
# source-time error would abort the generator outright. So the awk body below is deliberately its
# own — the same anchoring rule expressed within the constraint the caller carries — and it is the
# ONLY copy of it: the thing 0208's review found duplicated, the `worktree-scope:` extraction
# itself, now exists exactly once, here.
#
# TOLERANT ON THE FILE, deliberately: an unreadable or absent source yields empty rather than an
# error, because runner-dispatch.sh's probe must not shadow the adapter's more specific
# unknown-agent diagnostic. The loud seam for absence is sync-agents.sh's validate_agent_scopes.
# The `-f`/`-r` test is what keeps that tolerance quiet — awk on a missing file would otherwise
# write to stderr on every probe of a non-built-in agent.
#
# Bootstrap-compatible by requirement (above): no associative arrays, no mapfile/readarray, no
# ${x^^}/${x,,}, no `declare -g`, no `;;&`.

agent_worktree_scope(){ # agent_worktree_scope FILE -> declared scope on stdout (empty when absent)
  [ -f "$1" ] && [ -r "$1" ] || return 0
  # n counts `---` fences: a value is taken only while n == 1, i.e. inside the FIRST block, and the
  # scan exits at the closing fence, so no body line can ever be read as a declaration.
  # `[[:space:]]` classes throughout, never a literal space — a tab after the colon is as valid as
  # a space (AGENTS.md, "Shell": indent/space classes must be [[:space:]]).
  awk '
    /^---[[:space:]]*$/ { n++; if (n >= 2) exit; next }
    n == 1 && /^worktree-scope:/ {
      sub(/^worktree-scope:[[:space:]]*/, "")
      sub(/[[:space:]]+$/, "")
      print
      exit
    }
  ' "$1"
}
