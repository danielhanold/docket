#!/usr/bin/env bash
# tests/test_release_package.sh — hermetic guards over the two authored release-candidate surfaces
# (change 0317, Task 11): the non-publishing .github/workflows/release-candidate.yml and the native
# smoke driver scripts/release-smoke.sh, plus the embed tie between the downloader and render.go.
#
# THESE ARE GREP-SHAPED GUARDS. Every assertion below is a BYTE-PATTERN scan of source text, not a
# semantic proof. It proves a spelling is present or absent in the file; it does NOT execute the
# workflow, run a smoke, or observe GitHub Actions. Each section header restates the specific
# spelling limitation it carries. The semantic proofs live elsewhere: the packager and downloader
# behavior is proven by the Go tests and the tests/test_release_downloader*.sh sandbox suites, and
# the four native tuple executions plus the four-harness live acceptance are external truth no
# in-repo test can promote (docs/release/four-harness-acceptance.md).
#
# MUTATION EVIDENCE (Task 11): unpinning one `uses:` to @v4 reddens the SHA-pin guard; adding a
# `git tag` step reddens the publishing-verb ban. Both were exercised at authoring and restored.
#
# The ok/nok helpers are the tree's canonical byte-for-byte spelling (see tests/test_release_downloader.sh);
# scripts/run-tests.sh accounts results on the `ok - ` / `NOT OK - ` markers they print.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
WF="$REPO/.github/workflows/release-candidate.yml"
SMOKE="$REPO/scripts/release-smoke.sh"
RENDER="$REPO/internal/release/render.go"
DOWNLOADER="$REPO/internal/release/downloader/install.sh"
fail=0
ok(){ printf 'ok - %s\n' "$1"; }
nok(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# ================================================================================================
# SECTION A — the workflow file exists (nothing below can scan a missing file)
# ================================================================================================
if [ -f "$WF" ]; then
  ok "release-candidate workflow exists at .github/workflows/release-candidate.yml"
else
  nok "release-candidate workflow missing at .github/workflows/release-candidate.yml"
  exit "$fail"
fi

# ================================================================================================
# SECTION B — read-only permissions. SPELLING LIMIT: this scans the text of the top-level
# `permissions:` block for a `contents: read` grant and for the absence of any `write` spelling. It
# cannot prove GitHub resolves an effective read-only token — only that this file requests one.
# ================================================================================================
# The block is `permissions:` plus the indented lines under it, up to the next column-0 line.
perm_block="$(awk '/^permissions:/{p=1;print;next} p&&/^[^[:space:]]/{p=0} p{print}' "$WF")"
if [ -n "$perm_block" ]; then
  ok "a top-level permissions: block is present"
else
  nok "no top-level permissions: block found — a candidate workflow must pin read-only permissions"
fi
if grep -Eq 'contents:[[:space:]]*read' <<<"$perm_block"; then
  ok "permissions block grants contents: read"
else
  nok "permissions block does not grant contents: read"
fi
# Any `write` spelling inside the permissions context is a write grant (contents: write, write-all,
# packages: write, …). The permissions block must carry none.
if grep -qw 'write' <<<"$perm_block"; then
  nok "a write grant appears in the permissions context (block must be read-only):"
  grep -nw 'write' <<<"$perm_block" | sed 's/^/    /'
else
  ok "no write grant appears in the permissions context"
fi

# ================================================================================================
# SECTION C — no publishing verbs. SPELLING LIMIT: this bans the literal spellings `gh release`,
# `softprops/action-gh-release`, `git tag`, and `git push` anywhere in the workflow text. A publish
# reached by some other spelling (an aliased tool, a composite action) would slip past; the
# non-publishing guarantee is ultimately the read-only token (Section B) plus review. Because this
# is a pure spelling scan it fires on a comment too — which is what makes the `git tag` mutation
# reddenable.
# ================================================================================================
pub_hits="$(grep -nE 'gh[[:space:]]+release|softprops/action-gh-release|git[[:space:]]+tag|git[[:space:]]+push' "$WF" || true)"
if [ -n "$pub_hits" ]; then
  nok "a publishing verb spelling appears in the workflow (this workflow must not publish):"
  printf '%s\n' "$pub_hits" | sed 's/^/    /'
else
  ok "no publishing verb (gh release, softprops/action-gh-release, git tag, git push) appears"
fi

# ================================================================================================
# SECTION D — every action is SHA-pinned. SPELLING LIMIT: this requires every `uses:` line to carry
# an `@` followed by exactly 40 lowercase hex — the shape of a full commit SHA. It does not verify
# the SHA resolves to the tagged release the trailing `# vN` comment claims.
# ================================================================================================
uses_lines="$(grep -nE '^[[:space:]]*(-[[:space:]]+)?uses:' "$WF" || true)"
if [ -n "$uses_lines" ]; then
  ok "the workflow has at least one uses: step to pin"
else
  nok "the workflow has no uses: step — expected the checkout/setup-go/artifact actions"
fi
unpinned="$(grep -Ev '@[0-9a-f]{40}' <<<"$uses_lines" || true)"
if [ -n "$unpinned" ]; then
  nok "a uses: line is not pinned to a full 40-hex commit SHA:"
  printf '%s\n' "$unpinned" | sed 's/^/    /'
else
  ok "every uses: line is pinned to a full 40-hex commit SHA"
fi

# ================================================================================================
# SECTION E — both workflow_dispatch inputs. SPELLING LIMIT: scans the inputs block for the `ref:`
# and `version:` keys; it does not validate their descriptions or defaults.
# ================================================================================================
inputs_block="$(awk '/^[[:space:]]+inputs:/{p=1;next} p&&/^[[:space:]]{0,4}[a-z_]+:/{if($0 !~ /^[[:space:]]{6,}/){p=0}} p{print}' "$WF")"
# Fall back to the whole workflow-dispatch region if the indent heuristic came up empty.
if [ -z "$inputs_block" ]; then
  inputs_block="$(awk '/workflow_dispatch:/{p=1} p{print}' "$WF")"
fi
if grep -Eq '^[[:space:]]+ref:' <<<"$inputs_block"; then
  ok "workflow_dispatch declares the ref input"
else
  nok "workflow_dispatch is missing the ref input"
fi
if grep -Eq '^[[:space:]]+version:' <<<"$inputs_block"; then
  ok "workflow_dispatch declares the version input"
else
  nok "workflow_dispatch is missing the version input"
fi

# ================================================================================================
# SECTION F — the smoke matrix names all four native tuples. SPELLING LIMIT: this enumerates the
# four approved runner-label spellings (the fixed target set). It cannot tell that a label still
# maps to the architecture the comment claims — that is the authoring-time runner-image
# revalidation recorded in the PR description.
# ================================================================================================
for label in 'macos-15' 'macos-15-intel' 'ubuntu-24\.04' 'ubuntu-24\.04-arm'; do
  if grep -Eq "runner:[[:space:]]*${label}[[:space:]]*$" "$WF"; then
    ok "the smoke matrix names runner ${label}"
  else
    nok "the smoke matrix does not name runner ${label}"
  fi
done

# The workflow's smoke phase must actually invoke the native smoke driver.
if grep -qF -- 'scripts/release-smoke.sh' "$WF"; then
  ok "the workflow invokes scripts/release-smoke.sh"
else
  nok "the workflow never invokes scripts/release-smoke.sh"
fi

# ================================================================================================
# SECTION G — the smoke driver's own contract. SPELLING LIMIT: scans scripts/release-smoke.sh for
# the SMOKE PASS marker the summary greps and the --base-bundle flag the workflow forwards for PRs.
# It does not run the smoke.
# ================================================================================================
if [ -x "$SMOKE" ]; then
  ok "scripts/release-smoke.sh exists and is executable"
else
  nok "scripts/release-smoke.sh is missing or not executable"
fi
if grep -qF -- 'SMOKE PASS' "$SMOKE"; then
  ok "scripts/release-smoke.sh emits the SMOKE PASS marker the summary greps"
else
  nok "scripts/release-smoke.sh does not emit a SMOKE PASS marker"
fi
# Pattern leads with '--' — declare it with -- so grep does not parse it as an option.
if grep -qF -- '--base-bundle' "$SMOKE"; then
  ok "scripts/release-smoke.sh honors the --base-bundle upgrade flag"
else
  nok "scripts/release-smoke.sh does not honor the --base-bundle flag"
fi

# ================================================================================================
# SECTION H — the downloader is embedded. SPELLING LIMIT: scans render.go for the go:embed
# directive naming downloader/install.sh and confirms that file exists. It does not compile the
# package (the Go build is the semantic proof, via go test ./...).
# ================================================================================================
if [ -f "$RENDER" ] && grep -Eq '^//go:embed[[:space:]]+downloader/install\.sh' "$RENDER"; then
  ok "internal/release/render.go embeds downloader/install.sh"
else
  nok "internal/release/render.go does not embed downloader/install.sh"
fi
if [ -f "$DOWNLOADER" ]; then
  ok "the embedded downloader source internal/release/downloader/install.sh exists"
else
  nok "the embedded downloader source internal/release/downloader/install.sh is missing"
fi

exit "$fail"
