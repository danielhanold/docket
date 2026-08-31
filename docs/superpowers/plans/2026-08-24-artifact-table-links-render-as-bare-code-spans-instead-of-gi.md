<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0341 — Artifact-table links render as bare code spans instead of GitHub links](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-24-0341-artifact-table-links-render-as-bare-code-spans-instead-of-gi.md)**
<!-- docket:backlink:end -->
# Artifact-Table GitHub Links (change 0341) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every link block the Go `docket` binary renders (`## Artifacts` tables, `docket:backlink` blocks, PR-body links) carry `https://github.com/…/blob/…` URLs instead of bare code spans, then heal the already-broken committed files with a one-time two-branch re-render sweep.

**Architecture:** The pure render layer (`internal/render/link.go`) is already correct — it emits blob URLs iff `LinkContext.RepoWebURL` is non-empty. The defect is that all ~15 production `LinkContext` construction sites in `internal/app` set only `MetadataBranch`. Fix: a new `gitcli` remote-URL getter + a pure GitHub web-URL parser, wired once into `PinContext` (the pin every operation already threads through), and a single shared `linkContextOf(pin)` constructor replacing every inline literal so no site can omit the URL again. The sweep reuses the already-correct bash renderers: change files + spec backlinks on the `docket` metadata branch (one metadata commit), merged plan/results backlinks on this change's own feature branch (lands on `main` via its PR).

**Tech Stack:** Go 1.x (`internal/gitcli`, `internal/app`, `internal/render`), real-git Go test harnesses already in the repo, bash renderer scripts (`scripts/render-change-links.sh`, `scripts/render-artifact-backlink.sh`) for the sweep — unchanged.

**Spec:** `docs/superpowers/specs/2026-08-24-artifact-table-links-render-as-bare-code-spans-instead-of-gi-design.md` (on the `docket` metadata branch; also readable at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/…`).

## Global Constraints

- **Go changes only.** The bash renderers are correct and out of scope; do not edit `scripts/render-change-links.sh` or `scripts/render-artifact-backlink.sh`.
- **Exactly three accepted remote forms**, matching the bash parsers verbatim: `git@github.com:owner/repo(.git)`, `https://github.com/owner/repo(.git)`, `ssh://git@github.com/owner/repo(.git)`; strip one trailing `.git`. Everything else (non-GitHub hosts, unparseable, empty) yields `""` — the existing bare-path fallback, unchanged.
- **The remote is `origin`, read from local config only, offline** (spec A3).
- **Mutation-test every guard** (AGENTS.md): each mutation probe below must redden the named test, and every `go test` probe/verification runs with `-count=1` — Go's result cache otherwise serves a green PASS from before your mutation.
- **Full suite at the build gate**: `scripts/run-tests.sh` (that is what `finalize.test_command` resolves to). Individual tasks run only their focused tests.
- **Stage only your own paths** — never `git add -A` in any worktree; the metadata worktree is shared with concurrent loops (`add` + `commit` in one shell invocation there).
- **Feature worktree:** `/Users/homer/dev/docket/.worktrees/artifact-table-links-render-as-bare-code-spans-instead-of-gi`, branch `feat/artifact-table-links-render-as-bare-code-spans-instead-of-gi`. **Metadata worktree:** `/Users/homer/dev/docket/.docket`, branch `docket`.
- Tasks 1–4 are strictly ordered (each consumes the previous task's symbols). Task 5 is independent of the Go fix; Task 6 needs Task 4 (parity check uses the fixed binary).

---

### Task 1: `gitcli` remote-URL getter

The Go tree fetches `remote get-url` output only to discard it (`ensureRemoteConfigured`, `internal/gitcli/refs.go`). Add a getter that returns the URL. It reads **raw local config** (`git config --get remote.<name>.url`) rather than `git remote get-url`: the two differ only when a `url.<base>.insteadOf` rewrite is configured, where the raw value is the better source for a *web* URL (insteadOf is a transport rewrite; the raw value is what the user configured the repo as) — and raw-config reads are what makes Task 3's hermetic wiring test possible. This is a deliberate, recorded divergence from the bash renderers, observable only under insteadOf configs.

**Files:**
- Modify: `internal/gitcli/refs.go` (append after `ensureRemoteConfigured`; add `remoteURLOp` to the operation-label const block at the top)
- Test: `internal/gitcli/refs_test.go`

**Interfaces:**
- Consumes: existing `Client.run`, `stdoutLines`, `newFailure`, `validateRemoteName`, failure kinds.
- Produces: `func (c *Client) RemoteURL(ctx context.Context, repo Repository, remote RemoteName) (string, error)` — Task 3 calls it from `PinContext`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/gitcli/refs_test.go`, following the file's harness idioms (`requireGit`, `newRealClient`, `newMainModeRepos`, `mustDiscover`, `gitOut`):

```go
// TestRemoteURLReturnsConfiguredURL proves RemoteURL reads the raw configured
// remote URL from local config, offline — including a GitHub spelling no
// network op could reach from this test.
func TestRemoteURLReturnsConfiguredURL(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	url, err := c.RemoteURL(ctx, repo, "origin")
	if err != nil {
		t.Fatalf("RemoteURL error: %v", err)
	}
	if url != r.Origin {
		t.Fatalf("RemoteURL = %q, want %q", url, r.Origin)
	}

	gitOut(t, r.Invocation, "remote", "set-url", "origin", "git@github.com:owner/widgets.git")
	url, err = c.RemoteURL(ctx, repo, "origin")
	if err != nil {
		t.Fatalf("RemoteURL after set-url error: %v", err)
	}
	if url != "git@github.com:owner/widgets.git" {
		t.Fatalf("RemoteURL = %q, want git@github.com:owner/widgets.git", url)
	}
}

// TestRemoteURLUnconfiguredRemoteIsRemoteUnavailable proves an unknown remote
// name is a typed remote-unavailable failure, not a command error.
func TestRemoteURLUnconfiguredRemoteIsRemoteUnavailable(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	_, err := c.RemoteURL(context.Background(), repo, "nosuch")
	f, ok := AsFailure(err)
	if !ok || f.Kind != KindRemoteUnavailable {
		t.Fatalf("RemoteURL(nosuch) = %v, want KindRemoteUnavailable failure", err)
	}
}
```

If `newMainModeRepos`'s field spellings differ (check the struct in `internal/gitcli/harness_test.go` — tests above assume `r.Invocation` / `r.Origin` as used by `TestRemoteDefaultBranchAsksRemote`), adjust to the harness's actual field names; the assertions stay the same. If the harness origin URL as configured on the clone differs in spelling from `r.Origin` (e.g. relative), assert against `gitOut(t, r.Invocation, "config", "--get", "remote.origin.url")` instead for the first check.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gitcli/ -run 'TestRemoteURL' -count=1`
Expected: FAIL to compile — `c.RemoteURL undefined`.

- [ ] **Step 3: Implement `RemoteURL`**

In `internal/gitcli/refs.go`, add `remoteURLOp Operation = "remote-url"` to the operation const block, then append:

```go
// RemoteURL returns the configured URL of a named remote, read from raw local
// config (`git config --get remote.<name>.url`); offline, no network. Raw
// config is deliberate: a url.<base>.insteadOf transport rewrite must not
// perturb the value (change 0341 derives the repository WEB URL from it). An
// unconfigured remote name is remote-unavailable.
func (c *Client) RemoteURL(ctx context.Context, repo Repository, remote RemoteName) (string, error) {
	if err := validateRemoteName(remote); err != nil {
		return "", newFailure(remoteURLOp, KindInvalidRequest, "invalid remote name", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   remoteURLOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"config", "--get", "remote." + string(remote) + ".url"},
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(remoteURLOp, KindRemoteUnavailable, "remote is not configured", nil).withExitCode(res.exitCode)
	}
	lines := stdoutLines(res.stdout)
	if len(lines) != 1 {
		return "", newFailure(remoteURLOp, KindInvalidOutput, "unexpected remote url output", nil)
	}
	return lines[0], nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gitcli/ -run 'TestRemoteURL' -count=1`
Expected: PASS. Also run the package: `go test ./internal/gitcli/ -count=1` — no regressions.

- [ ] **Step 5: Commit**

```bash
git add internal/gitcli/refs.go internal/gitcli/refs_test.go
git commit -m "feat(0341): add gitcli RemoteURL getter reading raw remote config"
```

---

### Task 2: Pure GitHub web-URL parser and the shared `linkContextOf` constructor

**Files:**
- Create: `internal/app/link_context.go`
- Test: `internal/app/link_context_test.go`

**Interfaces:**
- Consumes: `StatusPin` (Task 3 adds its `RepoWebURL` field — write this task against the field; the two tasks compile together only after Task 3's Step 3a below, see note), `metadataBranchOf(pin)` (`internal/app/change_create.go`), `render.LinkContext`.
- Produces: `func githubWebURL(remoteURL string) string` and `func linkContextOf(pin StatusPin) render.LinkContext` — Tasks 3 and 4 use both.

**Ordering note:** `linkContextOf` reads `pin.RepoWebURL`. Add the field to `StatusPin` in **this** task (it is one line, and Task 3 wires it): in `internal/app/status.go`, extend the `StatusPin` struct (after `MetadataRevision string`):

```go
	// RepoWebURL is the https web base derived from origin's configured URL
	// ("https://github.com/owner/repo"), "" for non-GitHub/unreadable remotes.
	RepoWebURL string
```

- [ ] **Step 1: Write the failing tests**

Create `internal/app/link_context_test.go`:

```go
package app

import (
	"testing"

	"github.com/danielhanold/docket/internal/render"
)

// TestGithubWebURLAcceptedForms pins the parser to exactly the three remote
// spellings the bash renderers accept (render-change-links.sh, "Derive
// OWNER/REPO" case arms), .git stripped; everything else is "" (bare-path
// fallback).
func TestGithubWebURLAcceptedForms(t *testing.T) {
	cases := []struct{ in, want string }{
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo"},
		{"git@github.com:owner/repo", "https://github.com/owner/repo"},
		{"https://github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"ssh://git@github.com/owner/repo", "https://github.com/owner/repo"},
		// Rejections: non-GitHub hosts, other schemes, path remotes, degenerate.
		{"git@gitlab.com:owner/repo.git", ""},
		{"https://gitlab.com/owner/repo", ""},
		{"ssh://git@bitbucket.org/owner/repo", ""},
		{"/tmp/fixtures/origin.git", ""},
		{"../origin.git", ""},
		{"", ""},
		{"git@github.com:", ""},
		{"https://github.com/", ""},
		// Prefix must match exactly — a lookalike host is not GitHub.
		{"git@github.com.evil.example:owner/repo.git", ""},
	}
	for _, c := range cases {
		if got := githubWebURL(c.in); got != c.want {
			t.Errorf("githubWebURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestLinkContextOfCarriesBothFields is the constructor half of the 0341
// regression guard: the sole LinkContext constructor must thread the pin's web
// URL AND metadata branch. Mutation probe: drop RepoWebURL from linkContextOf
// — this test must redden.
func TestLinkContextOfCarriesBothFields(t *testing.T) {
	pin := StatusPin{
		Mode:           metadataModeDocket,
		MetadataBranch: "docket",
		RepoWebURL:     "https://github.com/owner/repo",
	}
	got := linkContextOf(pin)
	want := render.LinkContext{RepoWebURL: "https://github.com/owner/repo", MetadataBranch: "docket"}
	if got != want {
		t.Fatalf("linkContextOf = %+v, want %+v", got, want)
	}
	if url := got.BlobURL("docs/x.md"); url != "https://github.com/owner/repo/blob/docket/docs/x.md" {
		t.Fatalf("BlobURL = %q", url)
	}

	// Main mode: the branch comes from metadataBranchOf, the URL still flows.
	pin = StatusPin{Mode: metadataModeMain, DefaultBranch: "main", RepoWebURL: "https://github.com/owner/repo"}
	if got := linkContextOf(pin); got.MetadataBranch != "main" || got.RepoWebURL != "https://github.com/owner/repo" {
		t.Fatalf("main-mode linkContextOf = %+v", got)
	}
}
```

(Check the exact spellings of `metadataModeDocket` / `metadataModeMain` consts in `internal/app` — `metadataBranchOf` in `change_create.go` uses `metadataModeMain`; use whatever the package defines.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestGithubWebURL|TestLinkContextOf' -count=1`
Expected: FAIL to compile — `githubWebURL` / `linkContextOf` / `RepoWebURL` undefined.

- [ ] **Step 3: Implement**

Add the `RepoWebURL` field to `StatusPin` (see Ordering note above), then create `internal/app/link_context.go`:

```go
package app

import (
	"strings"

	"github.com/danielhanold/docket/internal/render"
)

// This file owns the app layer's link-context derivation (change 0341). The
// render package is pure and never derives its own LinkContext; before 0341
// every operation built the context inline and every one of them forgot
// RepoWebURL, so all Go-rendered artifact tables, backlinks, and PR links came
// out as bare code spans. linkContextOf is now the SOLE constructor (a source
// guard in link_context_guard_test.go enforces that), and githubWebURL is the
// pure parser matching the bash renderers' accepted remote forms.

// githubWebURL converts a GitHub remote URL to its https web base
// ("https://github.com/owner/repo"), accepting exactly the three forms the
// bash renderers accept (scripts/render-change-links.sh, "Derive OWNER/REPO"):
// git@github.com:owner/repo(.git), https://github.com/owner/repo(.git),
// ssh://git@github.com/owner/repo(.git), one trailing ".git" stripped. Any
// other spelling — non-GitHub hosts, path remotes, empty — yields "", which
// render treats as the bare-path fallback. An empty owner/repo remainder also
// yields "" (bash would emit a broken empty-repo URL here; "" is the strictly
// safer reading of the same degenerate input).
func githubWebURL(remoteURL string) string {
	var rest string
	switch {
	case strings.HasPrefix(remoteURL, "git@github.com:"):
		rest = strings.TrimPrefix(remoteURL, "git@github.com:")
	case strings.HasPrefix(remoteURL, "https://github.com/"):
		rest = strings.TrimPrefix(remoteURL, "https://github.com/")
	case strings.HasPrefix(remoteURL, "ssh://git@github.com/"):
		rest = strings.TrimPrefix(remoteURL, "ssh://git@github.com/")
	default:
		return ""
	}
	rest = strings.TrimSuffix(rest, ".git")
	if rest == "" {
		return ""
	}
	return "https://github.com/" + rest
}

// linkContextOf is the sole constructor of the LinkContext app operations hand
// to render: the repository web URL and the metadata branch travel together,
// so no call site can silently omit the URL again — the exact defect 0341
// fixes. Companion to metadataBranchOf.
func linkContextOf(pin StatusPin) render.LinkContext {
	return render.LinkContext{
		RepoWebURL:     pin.RepoWebURL,
		MetadataBranch: metadataBranchOf(pin),
	}
}
```

Wait — check the parser tests: `git@github.com.evil.example:owner/repo.git` does **not** have prefix `git@github.com:` (the colon is part of the prefix), so it correctly returns `""`. No extra host validation is needed; the prefixes are exact. This matches the bash `case` patterns byte-for-byte (`git@github.com:*` etc.).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestGithubWebURL|TestLinkContextOf' -count=1`
Expected: PASS. Then `go build ./...` — the package must still compile (the new field is additive; nothing reads it yet).

- [ ] **Step 5: Commit**

```bash
git add internal/app/link_context.go internal/app/link_context_test.go internal/app/status.go
git commit -m "feat(0341): pure GitHub web-URL parser and shared linkContextOf constructor"
```

---

### Task 3: Wire `RepoWebURL` into `PinContext`

**Files:**
- Modify: `internal/app/status_git.go` (function `PinContext`, the `StatusPin{…}` literal and the reads before it)
- Test: `internal/app/link_context_git_test.go` (create)

**Interfaces:**
- Consumes: `Client.RemoteURL` (Task 1), `githubWebURL` (Task 2), `originRemote` const, the `StatusPin.RepoWebURL` field.
- Produces: every pin returned by the production reader carries `RepoWebURL`; Task 4's call-site swap depends on it.

- [ ] **Step 1: Write the failing wiring test**

This is the spec's mutation-tested regression guard at the wiring boundary. It is hermetic because Task 1 reads **raw** config: the fixture configures origin's URL as a GitHub spelling while a `url.<local>.insteadOf` rewrite sends every actual network access (ls-remote, fetch) to the local bare origin.

Create `internal/app/link_context_git_test.go`:

```go
package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
)

// TestPinContextDerivesGitHubRepoWebURL is 0341's wiring regression guard:
// given a GitHub origin remote, the production reader's pin carries the derived
// web base and rendered link output carries blob URLs, not bare code spans.
// Hermetic: origin's CONFIGURED url is the GitHub spelling; the insteadOf
// rewrite routes all real network traffic to the local bare origin (RemoteURL
// reads raw config, so it still sees the GitHub spelling).
// Mutation probes (each must redden this test, run with -count=1):
//   - in PinContext, drop the RemoteURL call / the RepoWebURL assignment;
//   - in linkContextOf, drop the RepoWebURL field.
func TestPinContextDerivesGitHubRepoWebURL(t *testing.T) {
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0007-widget.md": changeRecord(7, "widget", "Widget"),
	})
	runGit(t, repo.invocation, "remote", "set-url", "origin", "git@github.com:owner/widgets.git")
	runGit(t, repo.invocation, "config", "url."+repo.origin+".insteadOf", "git@github.com:owner/widgets.git")

	ctx := context.Background()
	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(ctx, repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.RepoWebURL != "https://github.com/owner/widgets" {
		t.Fatalf("pin.RepoWebURL = %q, want https://github.com/owner/widgets", pin.RepoWebURL)
	}

	link := linkContextOf(pin)
	if got := link.BlobURL("docs/x.md"); got != "https://github.com/owner/widgets/blob/main/docs/x.md" {
		t.Fatalf("BlobURL = %q", got)
	}

	// Rendered output check: a backlink through the same context is a link,
	// not a code span.
	corpus, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	c := changeByPath(t, pin, corpus, "docs/changes/active/0007-widget.md")
	block, err := render.BacklinkContent(c, link)
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	if !strings.Contains(block, "(https://github.com/owner/widgets/blob/main/docs/changes/active/0007-widget.md)") {
		t.Fatalf("backlink is not a GitHub link:\n%s", block)
	}
	if strings.Contains(block, "`docs/changes/active/0007-widget.md`") {
		t.Fatalf("backlink still renders the bare code span:\n%s", block)
	}
}

// TestPinContextNonGitHubOriginYieldsEmptyWebURL pins the fallback: a plain
// local-path origin (every other harness fixture in this package) derives "",
// and rendering stays in repo-relative mode — the pre-0341 output for
// non-GitHub remotes is unchanged.
func TestPinContextNonGitHubOriginYieldsEmptyWebURL(t *testing.T) {
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0007-widget.md": changeRecord(7, "widget", "Widget"),
	})
	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.RepoWebURL != "" {
		t.Fatalf("pin.RepoWebURL = %q, want \"\"", pin.RepoWebURL)
	}
	if url := linkContextOf(pin).BlobURL("docs/x.md"); url != "" {
		t.Fatalf("BlobURL = %q, want \"\" (bare-path mode)", url)
	}
}
```

Helpers used — `newMainModeRepo`, `changeRecord`, `runGit`, `newGitClient` (`internal/app/status_git_test.go`), `changeByPath` (`internal/app/artifact_backlink_test.go`) — all live in the same package. If `changeByPath`'s signature differs, adapt the call; its use here is only to obtain a `domain.Change` for the render call.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestPinContext.*WebURL' -count=1`
Expected: FAIL — `pin.RepoWebURL = "", want https://github.com/owner/widgets` (the field exists since Task 2 but nothing populates it).

- [ ] **Step 3: Wire the derivation into `PinContext`**

In `internal/app/status_git.go`, inside `PinContext`, after the `r.repo = repo` assignment (origin's existence is proven just below by `RemoteDefaultBranch`; the URL read is offline and cheap), add:

```go
	// 0341: derive the repository web URL once per pin, from origin's raw
	// configured URL. Bash-renderer parity (render-change-links.sh:
	// `remote get-url origin || true`): an unreadable URL degrades to
	// bare-path links, never a failed pin.
	remoteURL, uerr := r.client.RemoteURL(ctx, repo, originRemote)
	if uerr != nil {
		remoteURL = ""
	}
```

and add to the `StatusPin{…}` literal:

```go
		RepoWebURL:          githubWebURL(remoteURL),
```

- [ ] **Step 4: Run tests to verify they pass, then mutation-test the wiring**

Run: `go test ./internal/app/ -run 'TestPinContext.*WebURL' -count=1`
Expected: PASS.

Mutation probes (do each, observe RED, revert — restore from your editor buffer or `git stash`, **never** `git checkout -- <file>` after staging partial work, which restores to HEAD and destroys the edit under test):
1. Delete the `RepoWebURL: githubWebURL(remoteURL),` line → `go test ./internal/app/ -run TestPinContextDerivesGitHubRepoWebURL -count=1` must FAIL. Restore.
2. In `linkContextOf`, delete the `RepoWebURL: pin.RepoWebURL,` line → same command must FAIL (and `TestLinkContextOfCarriesBothFields` too). Restore.

Then run the whole package: `go test ./internal/app/ -count=1` — every existing fixture uses local-path origins, which parse to `""`, so all committed-golden assertions (boards, backlinks) are byte-unchanged. If anything reddens, the fixture found a real behavior change — investigate, do not adjust goldens.

- [ ] **Step 5: Commit**

```bash
git add internal/app/status_git.go internal/app/link_context_git_test.go
git commit -m "feat(0341): derive RepoWebURL from origin at pin time"
```

---

### Task 4: Swap every production `LinkContext` literal to `linkContextOf`, with a source-shape guard

**Files:**
- Modify (15 literal sites, all currently `render.LinkContext{MetadataBranch: metadataBranchOf(pin)}` — verified identical by whole-repo grep, so the consolidation flattens no caller variance):
  - `internal/app/pr_publish.go:273`
  - `internal/app/change_create.go:218`
  - `internal/app/change_groom.go:177`
  - `internal/app/change_claim.go:183` and `:236`
  - `internal/app/change_attach.go:318` and `:390`
  - `internal/app/change_implemented.go:284`
  - `internal/app/change_reconcile.go` (site near `:73` is test-only; the production file has its own — swap every non-test occurrence the grep in Step 1 lists)
  - `internal/app/change_kill.go:156`
  - `internal/app/change_lifecycle.go:191`
  - `internal/app/change_reclaim.go:242`
  - `internal/app/finalize_closeout.go:441`
  - `internal/app/artifact_backlink.go:254`
  - `internal/app/adr_ops.go:180` and `:688`
- Leave alone: the zero-value `render.LinkContext{}` error-path returns in `pr_publish.go` (they accompany an error and render nothing), and every `_test.go` literal (their pins carry no web URL, so behavior is identical; the guard scopes to non-test files).
- Test: `internal/app/link_context_guard_test.go` (create)

**Interfaces:**
- Consumes: `linkContextOf(pin)` (Task 2).
- Produces: no production site constructs a field-carrying `render.LinkContext` literal outside `link_context.go`; the guard enforces it for future sites.

- [ ] **Step 1: Derive the site list from a grep, not this plan**

Line numbers above rot; the authority is:

```bash
grep -n 'render\.LinkContext{MetadataBranch' internal/app/*.go | grep -v _test.go
```

Every hit must be swapped. Also confirm there are no other field-carrying literals: `grep -rn 'render\.LinkContext{[^}]' internal/app cmd internal | grep -v _test.go | grep -v 'internal/render/'` — expect only the same sites plus `link_context.go` after the swap.

- [ ] **Step 2: Write the failing source-shape guard**

Create `internal/app/link_context_guard_test.go`:

```go
package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLinkContextSoleConstructor is 0341's shape guard: outside
// link_context.go, no production file in this package may construct a
// field-carrying render.LinkContext literal — that inline idiom is exactly how
// all ~15 sites forgot RepoWebURL. The zero-value literal
// `render.LinkContext{}` on error paths is allowed (it renders nothing).
// A population floor rides along: the constructor must actually be in use at
// no fewer sites than the swap installed, so mass-deleting call sites cannot
// leave this guard vacuously green.
// Mutation probes (run with -count=1): (a) revert one call site to the inline
// literal -> the literal scan reddens; (b) delete linkContextOf calls below the
// floor -> the floor reddens.
func TestLinkContextSoleConstructor(t *testing.T) {
	literal := regexp.MustCompile(`render\.LinkContext\{[^}]`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	uses := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		uses += strings.Count(string(src), "linkContextOf(")
		if name == "link_context.go" {
			continue
		}
		if loc := literal.Find(src); loc != nil {
			t.Errorf("%s constructs a field-carrying render.LinkContext literal; use linkContextOf(pin) (change 0341)", name)
		}
	}
	// Floor = 15 swapped call sites + 1 definition-adjacent use; adjust ONLY
	// upward when new operations adopt the constructor.
	const floor = 15
	if uses < floor {
		t.Errorf("linkContextOf used %d times in production files, want >= %d — call sites deleted or bypassed", uses, floor)
	}
}
```

Run: `go test ./internal/app/ -run TestLinkContextSoleConstructor -count=1`
Expected: FAIL — every unswapped file is named, and the floor is unmet.

- [ ] **Step 3: Swap the sites**

At each grep hit, replace

```go
render.LinkContext{MetadataBranch: metadataBranchOf(pin)}
```

with

```go
linkContextOf(pin)
```

(The pin variable in scope is spelled `pin` at every site — confirm per site; `pr_publish.go:273` returns it as an expression: `return c, linkContextOf(pin), nil`.) Where a file's only use of `render` was the swapped literal, remove the now-unused import; `go build ./...` will name them.

- [ ] **Step 4: Run the guard and package tests; mutation-test the guard**

Run: `go test ./internal/app/ -run TestLinkContextSoleConstructor -count=1` → PASS.
Run: `go test ./internal/app/ -count=1` → PASS (pins in existing fixtures carry `RepoWebURL: ""`, so rendered goldens are unchanged).

Mutation probes:
1. Revert one site (e.g. `change_kill.go`) to the inline literal → guard test FAILS naming the file. Restore.
2. Temporarily set `const floor = 99` → guard FAILS on the floor (proves the counter counts). Restore to 15.

- [ ] **Step 5: Commit**

```bash
git add internal/app/*.go
git commit -m "feat(0341): route all link-context construction through linkContextOf"
```

(The `*.go` glob here is scoped to the swap's own package after `git status` review — confirm nothing unrelated is dirty first; list files explicitly if anything else appears.)

---

### Task 5: Metadata-branch sweep — change files and spec backlinks on `docket`

Heals the committed corpus the fixed binary will never rewrite on its own (archived changes are terminal). Uses the **bash** renderers — the proven-correct writers of these generated blocks, and the sanctioned sole writers (re-rendering an archived change's `## Artifacts` block is what the docket-status sweep already does). Idempotent: already-correct files are byte-identical no-ops.

**Files:**
- Modify (in the metadata worktree `/Users/homer/dev/docket/.docket`, branch `docket`): the `<!-- docket:artifacts:… -->` blocks of every change file under `docs/changes/active/` and `docs/changes/archive/`, and the `<!-- docket:backlink:… -->` blocks of every spec under `docs/superpowers/specs/` reachable from a change's `spec:` field. **Marker blocks only — no frontmatter, no authored prose.**
- No repo source files change in this task.

**Interfaces:**
- Consumes: `scripts/render-change-links.sh`, `scripts/render-artifact-backlink.sh`, `scripts/lib/docket-frontmatter.sh` (all read from the feature worktree checkout, unmodified), `DOCKET_BASH_PATH` (exported by docket's install; the renderers require GNU bash 4+).
- Produces: a healed metadata corpus, one commit on `docket`, pushed.

- [ ] **Step 1: Sync the metadata worktree**

```bash
git -C /Users/homer/dev/docket/.docket pull --rebase -q origin docket
git -C /Users/homer/dev/docket/.docket status --porcelain
```

The status must be clean before you start; if another loop left uncommitted files, stop and report rather than sweeping over them.

- [ ] **Step 2: Run the sweep**

From the feature worktree root, write and run this script (scratch location, e.g. `$TMPDIR`; it is throwaway per spec A6 — do not commit it):

```bash
#!/usr/bin/env bash
# 0341 one-time metadata sweep: re-render every change file's ## Artifacts
# block and every spec's docket:backlink block via the bash renderers.
set -uo pipefail
FW="$(pwd)"                                  # feature worktree (scripts source)
MW="/Users/homer/dev/docket/.docket"         # metadata worktree (files swept)
BASH_BIN="${DOCKET_BASH_PATH:?run docket/install.sh}"
source "$FW/scripts/lib/docket-frontmatter.sh"

changes=0; specs=0
for cf in "$MW"/docs/changes/active/*.md "$MW"/docs/changes/archive/*.md; do
  [ -f "$cf" ] || continue
  case "$cf" in *BOARD.md|*README.md) continue ;; esac
  "$BASH_BIN" "$FW/scripts/render-change-links.sh" \
    --change-file "$cf" --adrs-dir "$MW/docs/adrs" || { printf 'FAILED: %s\n' "$cf" >&2; exit 1; }
  changes=$((changes+1))
  spec="$(fm_field "$cf" spec)"
  if [ -n "$spec" ] && [ -f "$MW/$spec" ]; then
    "$BASH_BIN" "$FW/scripts/render-artifact-backlink.sh" \
      --artifact-file "$MW/$spec" --change-file "$cf" || { printf 'FAILED spec: %s\n' "$spec" >&2; exit 1; }
    specs=$((specs+1))
  fi
done
printf 'swept %d change files, %d spec backlinks\n' "$changes" "$specs"
[ "$changes" -gt 0 ] || { printf 'sweep iterated zero change files — glob is wrong\n' >&2; exit 1; }
```

Run it with `"${DOCKET_BASH_PATH:?run docket/install.sh}" <script>` (the loop itself uses `fm_field`, which needs GNU bash). The zero-iteration trap at the end is load-bearing: a shell sweep that matches nothing prints success otherwise. Note the counts it prints — they go in the results file.

If a change file legitimately has no spec on the metadata branch (e.g. a trivial change), the loop skips its backlink — correct, not an error.

- [ ] **Step 3: Verify the diff is marker-blocks-only**

```bash
git -C /Users/homer/dev/docket/.docket status --porcelain
git -C /Users/homer/dev/docket/.docket diff --stat
git -C /Users/homer/dev/docket/.docket diff | grep '^[+-]' | grep -v '^[+-][+-]' | grep -vE '^\+\| (Spec|Plan|Results|PR|ADRs) ' | grep -vE '^-\| (Spec|Plan|Results|PR|ADRs) ' | grep -vE '^[+-]> ↩ ' | head -20
```

Every changed path must be under `docs/changes/active/`, `docs/changes/archive/`, or `docs/superpowers/specs/`. The last command surfaces any changed line that is not an artifacts-table row or a backlink line — expect **no output** (table header/marker lines never change; if it prints anything, read the diff before committing — do not commit a sweep that touched authored content). Spot-read one previously-broken file (e.g. `docs/changes/archive/`'s 0340 entry): its rows must now be `[…](https://github.com/danielhanold/docket/blob/docket/…)` links.

- [ ] **Step 4: Commit and push on `docket` (one shell invocation — shared worktree)**

```bash
git -C /Users/homer/dev/docket/.docket add docs/changes/active docs/changes/archive docs/superpowers/specs && git -C /Users/homer/dev/docket/.docket commit -m "chore(0341): one-time re-render sweep — artifact tables and spec backlinks to GitHub links" && git -C /Users/homer/dev/docket/.docket push -q origin docket
```

If the push is rejected (another loop advanced `docket`), `git -C /Users/homer/dev/docket/.docket pull --rebase -q origin docket` and push again — the sweep's blocks re-render identically on top of any concurrent metadata edit. If the commit reports "nothing to commit" despite Step 3 showing changes, treat it as another loop having swallowed the staged files — investigate, never re-run blindly.

---

### Task 6: Integration-branch sweep — plan/results backlinks on the feature branch, plus Go/bash parity proof

Merged plan (`docs/superpowers/plans/`) and results (`docs/results/`) files live on `main`; their backlink re-stamps ride **this** feature branch to `main` through the PR. `render-artifact-backlink.sh` re-stamping a merged artifact's backlink is the convention's one permitted post-merge writer. The bash renderer resolves each done change's plan/results ref to the integration branch, so healed links point at `blob/main/…`.

**Files:**
- Modify (in the feature worktree): the `docket:backlink` marker blocks of files under `docs/superpowers/plans/` and `docs/results/`. Marker blocks only.

**Interfaces:**
- Consumes: `scripts/render-artifact-backlink.sh`, `fm_field`, the metadata worktree's change files (each change's `plan:`/`results:` frontmatter names its artifacts; the change file at its current canonical path is the renderer's required `--change-file`), and — for the parity proof — the Task-4 binary via `go run ./cmd/docket`.
- Produces: healed backlinks committed on `feat/artifact-table-links-render-as-bare-code-spans-instead-of-gi`; a recorded Go-vs-bash byte-parity observation for the results file.

- [ ] **Step 1: Run the sweep**

From the feature worktree root, another throwaway script (run under `"$DOCKET_BASH_PATH"`):

```bash
#!/usr/bin/env bash
# 0341 one-time integration-line sweep: re-stamp plan/results backlinks for
# every change whose artifacts exist in this (feature) worktree.
set -uo pipefail
FW="$(pwd)"
MW="/Users/homer/dev/docket/.docket"
BASH_BIN="${DOCKET_BASH_PATH:?run docket/install.sh}"
source "$FW/scripts/lib/docket-frontmatter.sh"

stamped=0
for cf in "$MW"/docs/changes/active/*.md "$MW"/docs/changes/archive/*.md; do
  [ -f "$cf" ] || continue
  case "$cf" in *BOARD.md|*README.md) continue ;; esac
  for key in plan results; do
    rel="$(fm_field "$cf" "$key")"
    [ -n "$rel" ] || continue
    [ -f "$FW/$rel" ] || continue
    "$BASH_BIN" "$FW/scripts/render-artifact-backlink.sh" \
      --artifact-file "$FW/$rel" --change-file "$cf" || { printf 'FAILED: %s\n' "$rel" >&2; exit 1; }
    stamped=$((stamped+1))
  done
done
printf 'stamped %d plan/results backlinks\n' "$stamped"
[ "$stamped" -gt 0 ] || { printf 'sweep stamped zero artifacts — mapping loop is wrong\n' >&2; exit 1; }
```

(Plans/results referenced by a change but absent from this tree — never merged, or metadata-only — are skipped by the `-f` test: correct.) Record the stamped count and `git status --porcelain -- docs/superpowers/plans docs/results | wc -l` (files actually modified — the idempotent majority re-render byte-identically).

- [ ] **Step 2: Verify the diff is backlink-blocks-only**

```bash
git status --porcelain -- docs/superpowers/plans docs/results
git diff -- docs/superpowers/plans docs/results | grep '^[+-]' | grep -v '^[+-][+-]' | grep -v '^[+-]> ↩ ' | head
```

Expect no output from the second command (only `> ↩ …` lines change). Changed paths must all be under the two directories. This change's **own** plan file will be among the stamped files (its backlink was written by the parent flow); its re-render must be a byte-identical no-op — if it changed, stop and look.

- [ ] **Step 3: Go/bash parity proof on a swept file**

The spec directs confirming the fixed Go renderer byte-matches bash on the same input. Pick one plan file the bash sweep just stamped (with a done/archived owning change), then run the Go verb on it and prove a no-op:

```bash
go run ./cmd/docket artifact backlink --artifact <repo-relative plan path> --change <that change's repo-relative archive path>
git diff --exit-code -- <repo-relative plan path>
```

Expected: exit 0 (byte-identical — Go and bash now agree on GitHub-link output). If it diffs, the Go and bash renderers disagree on a real input: that is a Task-2/Task-3 defect to fix, not a diff to commit. Record the file used and the outcome for the results file. (If the `artifact backlink` verb's flag spellings differ, `go run ./cmd/docket artifact backlink --help` is the authority.)

- [ ] **Step 4: Commit on the feature branch**

```bash
git add docs/superpowers/plans docs/results
git commit -m "chore(0341): one-time re-render sweep — merged plan/results backlinks to GitHub links"
```

---

### Task 7: Build gate — full suite and real-tree verification notes

- [ ] **Step 1: Run the full suite**

Run `scripts/run-tests.sh` per its own contract (background to a stable log if it approaches the foreground ceiling; key on the exit code). Expected: PASS. Act on any `SERIAL CONFIRMED OVER BUDGET:` line; note any `BUDGET WATCH:` screening findings.

- [ ] **Step 2: Real-tree verification (for the results file)**

The hermetic suite cannot see the metadata branch or the real GitHub origin (learnings: `metadata-branch-invisible-to-suite`), so record at build time, in `## Findings` of the eventual results file:
- the sweep counts from Tasks 5 and 6 (files iterated / files actually modified), and that origin's `docket` branch shows the healed blocks (`git -C /Users/homer/dev/docket/.docket log -1 --stat origin/docket` after the push; spot-read one archived change file at `origin/docket`);
- the mutation-probe outcomes from Tasks 3 and 4 (each probe red, then restored green);
- the Task 6 Step 3 parity file and its no-op result;
- the deliberate raw-config divergence from `git remote get-url` (Task 1 header) and the empty-`owner/repo` strictness note (Task 2), so review sees them as decisions, not accidents.

---

## Self-Review (performed while writing)

- **Spec coverage:** §What-changes-1 (derive once, pure parser, shared constructor, all sites) → Tasks 1–4; §2 metadata-branch sweep → Task 5; §2 integration-branch sweep → Task 6; §3 mutation-tested regression guard → Tasks 3–4 (wiring test + probes + shape guard with population floor); out-of-scope parity confirmation → Task 6 Step 3; A2/A3 exact forms and origin-only → Global Constraints + Task 2 tests; A5/A6 sweep-not-self-heal, no new public verb → Tasks 5–6 use throwaway scripts over existing renderers; A7 executable guard, not committed-file grep → Task 4's guard scans package **source** for a forbidden construction shape (a code-idiom ban, not a rendered-corpus grep — historical/non-GitHub records stay legal).
- **Known deliberate divergences from bash, both surfaced for review:** raw-config read vs `remote get-url` (Task 1); `""` for empty owner/repo remainder vs bash's degenerate URL (Task 2).
- **Type consistency:** `RemoteURL(ctx, repo Repository, remote RemoteName) (string, error)`; `githubWebURL(string) string`; `linkContextOf(StatusPin) render.LinkContext`; `StatusPin.RepoWebURL string` — spellings identical across Tasks 1–4.
