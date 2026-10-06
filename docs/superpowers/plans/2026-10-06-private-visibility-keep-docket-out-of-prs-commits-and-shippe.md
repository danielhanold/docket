<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0532 — Implement private visibility for PRs, commits, and shipped files](../../changes/archive/2026-10-06-0532-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md)**
<!-- docket:backlink:end -->

# Implement Private Visibility for PRs, Commits, and Shipped Files: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It routes each task to a tier agent running the `docket-build-task` contract. Each task carries its own focused test cycle and ends in one commit. The whole suite runs once at the end. Steps use checkbox (`- [ ]`) syntax.

**Goal:** In a private repository, nothing that reaches `origin` through a feature branch or a PR carries a docket or `dckt` fingerprint:
- PR text is plain authored prose;
- finalize posts no PR comments;
- merged-PR backlink repointing is skipped;
- a **blocking** leak check refuses any push or PR write that would expose a fingerprint.

**Architecture:**
- A new pure, stdlib-only package `internal/leakscan` holds the fingerprint rules.
- A new gitcli read, `ReadOutgoing`, returns what a push exposes: the commit messages in `base..head`, plus the added paths and lines of the merge-base diff `base...head`.
- One app helper, `runLeakCheck` (`internal/app/leak_check.go`), fetches the effective base from `origin` and scans. It runs only when `leakCheckApplies(layout)` holds, and always before the run-fence admission and the external effect. Its three call sites are `workspace.publish`, `pr.publish` (which also scans the title and body), and `finalize.publish`.
- In a private repository, the other mode decisions each read `pin.Layout` once: a verbatim PR body, a spec copy with no change line, no finalize-block comment, and no backlink repoint.
- Report-only additions:
  - a `metadata-on-shared-remote` warning finding;
  - a gitcli guard that makes push primitives push `refs/heads/*` only, so local `refs/docket/…` scratch refs can never reach a remote.

**Tech Stack:**
- Go.
- Real-git tests behind `//go:build integration`, sharded by name prefix. `tests/test_go_integration_contract.sh` requires each tagged test to match exactly one shard prefix.
- cobra.
- `go generate ./internal/assets/` for the embedded twins.

**Spec:** `docs/superpowers/specs/2026-10-05-private-visibility-keep-docket-out-of-prs-commits-and-shippe-design.md`. A copy is the feature branch's first commit. Read it first.

## Global Constraints

The spec's **Decisions** 1–7 are binding:
- **Private PR descriptions** are authored prose only: no backlink, artifacts, or evidence block, no `↩ Change` line, no `#issue` reference, and no change id in the title.
- **A private finalize block** is recorded on the record (`## Finalize blocked`) only. No PR comment.
- **The writing rule** (spec §2, verbatim) applies to everything that ships. Task 9 states it once in docket-convention.
- **The leak check blocks** in private repositories only, as a deliberate scoped exception. Shared repositories never invoke the scanner.
- **`dckt` is always matched.** The bare word `docket` is matched unless `leak_check.match_word: false`. That key is ordinary: any layer, normal precedence, default `true`.
- **A `docket`/`dckt` branch or a `refs/docket/…` ref on a private repository's `origin`** is reported, never blocked.
- **Docs:** only `.docket.example.yml` (and its twin), command help, and skills.
- **AGENTS.md rules:** derive sites by grep; mutation-test every guard, restoring from a **backup copy** (never `git checkout --`) with `-count=1`; template `mktemp`; anchor cross-references on symbols; no pipe into `grep -q`/`head`.

## Review Focus

Each item has a test in its owning task.
1. **Odd added paths** (a space, non-ASCII, a `"`): each added line keeps its file; Git C-quotes such patch headers (Task 3).
2. **Upstream `main` merged into the branch is not outgoing**; a fingerprint there does not block (Task 6).
3. **An unverifiable scan never reads as clean**: `leak-check-unverified`, nothing pushed (Tasks 6–7).
4. **Number-shaped non-references pass:** `#612`, `(2026)`, `change 3 files`, `ADR-0012`, and `Docket: 12-345` under `match_word: false` (Task 1).
5. **PR-body redaction:** a hit carries only the matched token and a line number (Task 7).

## Learnings applied

- **probe-error-is-not-clean-absence:** a fetch, log, diff, or `ls-remote` failure is "unverified", never "none".
- **decide-and-act-on-the-same-copy:** each site scans the exact head it publishes. `workspace.publish` re-proves it as `ExpectedHead` under the lock.
- **git-path-output-is-quoted:** read paths with `-z`, and unquote patch headers.
- **prohibition-needs-a-return-value:** the build-task rule names `BLOCKED` in the clause.
- **specified-but-unreachable:** the skill guard anchors on the producer paragraph (the implement-next halt).
- **intermediate-task-state-buildable:** Task 4 removes the spec copy's `Change 0001 — …` line before Task 6 turns the scanner on. Otherwise the private lifecycle test would be refused.
- **defaulted-param-hides-caller-wiring:** `specCopyBytes` takes the mode at every call site.

## ADRs to record

The parent records this through `docket-adr`; it is not a build task.

1. **The private-repository leak check blocks.** It is a deliberate, scoped exception to docket's report-only default. A pushed leak is irreversible and outward-facing, and the check cannot affect shared repositories.

---

### Task 1: The `internal/leakscan` package

**Build tier:** standard

**Files:**
- Create: `internal/leakscan/leakscan.go`
- Create: `internal/leakscan/leakscan_test.go` (default tag)

**Interfaces (produces):**

```go
type Rule string   // "marker" "trailer" "path" "alias" "change-ref" "word" (RuleMarker … RuleWord)
type Source string // "commit-message" "added-line" "added-path" "pr-title" "pr-body" (SourceCommitMessage …)
type Options struct { MatchWord bool; ChangeIDs map[int]bool }
type Match struct { Text string; Rule Rule }
type Hit struct { Source Source; Commit, File string; Line int; Text string; Rule Rule }
type Commit struct{ ID, Message string }
type AddedLine struct { Path string; Line int; Text string }
type PRText struct{ Title, Body string }
type Input struct { Commits []Commit; AddedPaths []string; AddedLines []AddedLine; PR *PRText }
func Line(line string, opts Options) (Match, bool)
func Text(src Source, text string, opts Options) []Hit
func Scan(in Input, opts Options) []Hit
```

- [ ] **Step 1: Write the failing tests.**
  - **`TestLineRules`** is a table over `Line` with `ids := map[int]bool{1: true, 12: true, 36: true, 612: true}`. W = `MatchWord`.

    | line | W | want |
    |---|---|---|
    | `<!-- docket:backlink:start (generated) -->` | true | marker `docket:` |
    | `<!-- docket:backlink:start -->` | false | marker `docket:` |
    | `# dckt:start` | false | marker `dckt:` |
    | `Docket: 12-345 was filed` | false | none |
    | `Docket: 12-345 was filed` | true | word `Docket` |
    | `Docket-Operation: change.claim` | false | trailer `Docket-Operation:` |
    | `  docket-request-id: abc` | false | trailer `docket-request-id:` |
    | `see .docket/docs/x.md` | false | path `.docket` |
    | `edit .docket.yml` | false | path `.docket` |
    | `cat .git/dckt/config.yml` | false | path `.git/dckt` |
    | `run dckt status` | false | alias `dckt` |
    | `the dckts list` | true | none |
    | `fix the widget (0612)` | false | change-ref `(0612)` |
    | `change 0612 lands` | false | change-ref `change 0612` |
    | `Changes #612 and more` | false | change-ref `Changes #612` |
    | `tracked as #0612` | false | change-ref `#0612` |
    | `the (2026) layout` | true | none |
    | `fixes #612` | true | none |
    | `change 3 files` | true | none |
    | `per ADR-0036 and ADR 0012` | true | none |
    | `fix (0613)` | true | none |
    | `the docket was filed` | true | word `docket` |
    | `the docket was filed` | false | none |
    | `docketBranch := x` | true | word `docket` |
    | `myDocket := x` | true | word `Docket` |
    | `court dockets` | true | none |
    | `pocketdocket` | true | none |
    | `DOCKET` | true | word `DOCKET` |
    | `docket_branch` | true | word `docket` |

  - **`TestTextLineNumbersAndCRLF`:** `Text(SourcePRBody, "ok\r\nsee .docket/x\r\n", opts)` gives one hit, `{Line: 2, Rule: path, Text: ".docket"}`.
  - **`TestScanAttributesEverySource`:** one `Input` carries:
    - commit `{ID: "c1", Message: "Add widget\n\nRefs (0612)\n"}`;
    - added path `notes/.docket-old.md`;
    - added line `{a.go, 7, "// dckt"}`;
    - `PR: &PRText{Title: "change 0012", Body: "plain\n"}`.

    Expect exactly four hits:
    - `{commit-message, Commit c1, Line 3, change-ref}`;
    - `{added-path, File notes/.docket-old.md, path}`;
    - `{added-line, a.go, 7, alias}`;
    - `{pr-title, Line 1, change-ref}`.
  - **`TestScanNilPRAndNilIDs`:** no panic, and no change-ref.
- [ ] **Step 2: Run `go test ./internal/leakscan/ -count=1`.** Expected: FAIL.
- [ ] **Step 3: Implement** (package doc: pure, stdlib-only; one hit per line, rules tried in a fixed order):

```go
package leakscan

import (
	"strconv"
	"strings"
)

type Rule string

const (
	RuleMarker    Rule = "marker"     // docket:/dckt: followed by a letter
	RuleTrailer   Rule = "trailer"    // Docket-<Key>: at line start
	RulePath      Rule = "path"       // .docket or .git/dckt
	RuleAlias     Rule = "alias"      // the word dckt
	RuleChangeRef Rule = "change-ref" // a reference to a backlog change id
	RuleWord      Rule = "word"       // the bare word docket (Options.MatchWord)
)

type Source string

const (
	SourceCommitMessage Source = "commit-message"
	SourceAddedLine     Source = "added-line"
	SourceAddedPath     Source = "added-path"
	SourcePRTitle       Source = "pr-title"
	SourcePRBody        Source = "pr-body"
)

// Options, Match, Hit, Commit, AddedLine, PRText, Input: exactly as in
// Interfaces. Hit.Commit is set for a commit-message hit, Hit.File for an added
// line or path; Hit.Line is 1-based (0 for a path); Hit.Text is the matched text
// only, never the line. A nil ChangeIDs matches no change reference.

// Scan, in input order: each commit through Text(SourceCommitMessage) with
// Commit set; each added path through Line (an added-path hit, File set, Line
// 0); each added line through Line (added-line, File and Line set); then, when
// PR != nil, Text(SourcePRTitle) and Text(SourcePRBody).
// Text splits on "\n", trims a trailing "\r", and numbers lines from 1.

// Line tries marker, trailer, path, alias, change-ref, word, in that order.
func Line(line string, opts Options) (Match, bool) {
	low := asciiLower(line)
	for _, w := range []string{"docket", "dckt"} {
		for _, i := range wordAt(line, low, w) {
			j := i + len(w)
			if j+1 < len(line) && line[j] == ':' && isLetter(line[j+1]) {
				return Match{line[i : j+1], RuleMarker}, true
			}
		}
	}
	if t, ok := trailerKey(line, low); ok {
		return Match{t, RuleTrailer}, true
	}
	for _, frag := range []string{".docket", ".git/dckt"} {
		if i := fragmentAt(low, frag); i >= 0 {
			return Match{line[i : i+len(frag)], RulePath}, true
		}
	}
	if is := wordAt(line, low, "dckt"); len(is) > 0 {
		return Match{line[is[0] : is[0]+4], RuleAlias}, true
	}
	if t, ok := changeRef(line, low, opts.ChangeIDs); ok {
		return Match{t, RuleChangeRef}, true
	}
	if opts.MatchWord {
		if is := wordAt(line, low, "docket"); len(is) > 0 {
			return Match{line[is[0] : is[0]+6], RuleWord}, true
		}
	}
	return Match{}, false
}

// wordAt: offsets of w in low not glued to a letter on either side, except
// across a camelCase boundary ("docketBranch", "myDocket" match; "dockets",
// "pocketdocket" do not). asciiLower keeps byte offsets.
func wordAt(line, low, w string) []int {
	var out []int
	for from := 0; ; {
		k := strings.Index(low[from:], w)
		if k < 0 {
			return out
		}
		i, j := from+k, from+k+len(w)
		prevOK := i == 0 || !isLetter(line[i-1]) || (isUpper(line[i]) && isLower(line[i-1]))
		nextOK := j == len(line) || !isLetter(line[j]) || (isUpper(line[j]) && isLower(line[j-1]))
		if prevOK && nextOK {
			out = append(out, i)
		}
		from = i + 1
	}
}

// fragmentAt: first offset of frag not followed by a letter, or -1.
func fragmentAt(low, frag string) int {
	for from := 0; ; {
		k := strings.Index(low[from:], frag)
		if k < 0 {
			return -1
		}
		i := from + k
		if j := i + len(frag); j == len(low) || !isLetter(low[j]) {
			return i
		}
		from = i + 1
	}
}

func trailerKey(line, low string) (string, bool) {
	start := len(line) - len(strings.TrimLeft(line, " \t"))
	rest, lrest := line[start:], low[start:]
	const p = "docket-"
	if !strings.HasPrefix(lrest, p) {
		return "", false
	}
	k := len(p)
	for k < len(rest) && (isLetter(rest[k]) || isDigit(rest[k]) || rest[k] == '-') {
		k++
	}
	if k == len(p) || k >= len(rest) || rest[k] != ':' {
		return "", false
	}
	return rest[:k+1], true
}

// changeRef: "change[s] #N" or "change[s] NNNN+"; "#0NNN"; "(0NNN)" — the id
// must be in ids. "(2026)" and "#612" never match (not zero-padded).
func changeRef(line, low string, ids map[int]bool) (string, bool) {
	if len(ids) == 0 {
		return "", false
	}
	for _, w := range []string{"change", "changes"} {
		for _, i := range wordAt(line, low, w) {
			j := i + len(w)
			k := j
			for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			if k == j {
				continue
			}
			hash := k < len(line) && line[k] == '#'
			if hash {
				k++
			}
			if d, end := digitsAt(line, k); d != "" && (hash || len(d) >= 4) && inBacklog(d, ids) {
				return line[i:end], true
			}
		}
	}
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '#':
			if i > 0 && (isLetter(line[i-1]) || isDigit(line[i-1])) {
				continue
			}
			if d, end := digitsAt(line, i+1); padded(d) && inBacklog(d, ids) {
				return line[i:end], true
			}
		case '(':
			if d, end := digitsAt(line, i+1); padded(d) && end < len(line) && line[end] == ')' && inBacklog(d, ids) {
				return line[i : end+1], true
			}
		}
	}
	return "", false
}

// digitsAt: 1–9 digits at i not followed by a letter; "" otherwise.
func digitsAt(line string, i int) (string, int) {
	j := i
	for j < len(line) && isDigit(line[j]) {
		j++
	}
	if j == i || j-i > 9 || (j < len(line) && isLetter(line[j])) {
		return "", i
	}
	return line[i:j], j
}

func padded(d string) bool { return len(d) >= 4 && d[0] == '0' }
func inBacklog(d string, ids map[int]bool) bool {
	n, err := strconv.Atoi(d)
	return err == nil && ids[n]
}
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if isUpper(c) {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
func isLetter(c byte) bool { return isLower(c) || isUpper(c) }
func isLower(c byte) bool  { return c >= 'a' && c <= 'z' }
func isUpper(c byte) bool  { return c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
```

  Give every exported identifier a one-line doc comment.
- [ ] **Step 4: Run the tests and confirm they pass.** The table is the spec. A row that contradicts acceptance 3 or 4 is `NEEDS_ESCALATION`; never edit a row to fit the code.
- [ ] **Step 5: Mutation-test** (backup copy in `mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX"`, `cp` restore, `-count=1`):
  - Dropping the `opts.MatchWord` check reddens the W=false word rows.
  - Removing `padded(d)` from the `(` case reddens `(2026)`.
  - Removing `isLetter(line[j+1])` reddens `Docket: 12-345` with W=false.
  - `prevOK = true` reddens `pocketdocket`.
- [ ] **Step 6: Commit.** Message: `feat(leakscan): fingerprint rules for text a private repository publishes`.

---

### Task 2: The `leak_check.match_word` config key

**Build tier:** economy

**Files:**
- Modify: `internal/config/{schema,config,defaults,resolve}.go`, `internal/app/config.go` (`effectiveLines`), and `.docket.example.yml`; regenerate `internal/assets/embedded/`.
- Tests: add a row next to every `reclaim.auto` row found by `grep -rn '"reclaim.auto"' internal/config internal/app`. Expected files: `decode_test.go`, `defaults_test.go`, `resolve_test.go` (`effectiveLeaf`), `schema_test.go` (path set, defaults), `setting_paths_test.go`, and `fixtures_test.go` (both lists).

**Interfaces (produces):** `type LeakCheck struct { MatchWord Value[bool] \`json:"match_word"\` }` and the field `Effective.LeakCheck LeakCheck \`json:"leak_check"\``. The default is `true`.

- [ ] **Step 1: Write the failing tests.** Mirror the `reclaim.auto` rows: block `leak_check:\n  match_word: false\n`, flow `leak_check: {match_word: false}\n`, value `false`, default `true`, supported. The path follows `reclaim.auto` in the path-set list.
- [ ] **Step 2: Run the tests and watch them fail.** Run `go test ./internal/config/ ./internal/app/ -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement.**
  - **Schema row** after `reclaim.auto`, with a comment that a shared repository never runs the check:
    `{path: "leak_check.match_word", kind: kindBool, def: true, merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: boolLeaf()}`.
  - **`config.go`:** add the type and the field after `Reclaim`.
  - **`defaults.go`:** `LeakCheck: LeakCheck{MatchWord: builtinValue(true)}`.
  - **`resolve.go`:** `set(assign(&eff.LeakCheck.MatchWord, r.declared, "leak_check.match_word"))`.
  - **`app/config.go`:** `leafLine("leak_check.match_word", strconv.FormatBool(eff.LeakCheck.MatchWord.Value), eff.LeakCheck.MatchWord.Provenance)`.
  - **`.docket.example.yml`:** add a section after `reclaim`, in the file's banner style:

```yaml
# ═══ leak_check — keeping docket out of a private repository's shared surfaces ═══════════

leak_check:
  # match_word — true (default): in a private repository, a push or pull request whose commits,
  # added lines, added paths, or PR text contain the bare word "docket" is refused. false: let
  # the bare word through, for a codebase where it is an ordinary word (court or shipping
  # dockets); markers, trailers, `.docket` paths, `dckt`, and change ids are still refused.
  # A shared repository never runs the check.
  # scope: any layer
  match_word: true
```

    Then run `go generate ./internal/assets/`.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/config/ ./internal/app/ ./internal/assets/ -count=1`. Expected: PASS, including the example-correspondence and twin checks.
- [ ] **Step 5: Commit.** Message: `feat(config): add leak_check.match_word`.

---

### Task 3: gitcli `ReadOutgoing`, `ListRemoteRefs`, and branch-only pushes

**Build tier:** premium. A misread patch silently un-scans lines.

**Files:**
- Create: `internal/gitcli/outgoing.go`, `outgoing_test.go` (default tag), and `outgoing_integration_test.go` (prefix `TestIntegrationRepo`).
- Create: `internal/gitcli/push_guard_test.go` (default tag).
- Modify: `internal/gitcli/remoteheads.go` (`ListRemoteRefs`) and `push.go` (the guard).
- Extend: `remoteheads_integration_test.go`. Name the new test `TestIntegrationRepoListRemoteRefs`: prefix `TestIntegrationListRemoteRefs` would match no shard.

**Interfaces (produces):**

```go
type OutgoingCommit struct { Commit ObjectID; Message string } // raw %B
type OutgoingLine struct { Path string; Line int; Text string } // 1-based, head's version
type Outgoing struct { Commits []OutgoingCommit; AddedPaths []string; AddedLines []OutgoingLine }
func (c *Client) ReadOutgoing(ctx context.Context, repo Repository, base, head ObjectID) (Outgoing, error)
func (c *Client) ListRemoteRefs(ctx context.Context, repo Repository, remote RemoteName, patterns []string) (map[RefName]ObjectID, error)
```

`PushLease` and `PushCreateLease` refuse a ref outside `refs/heads/` with `KindInvalidRequest` before running git.

- [ ] **Step 1: Write the failing tests.**
  - **`TestParseAddedLines`** (pure). A hand-written `-U0` patch gives the exact `[]OutgoingLine`. It covers:
    - a modified file with `-`/`+` lines;
    - a new file `@@ -0,0 +1,2 @@`;
    - a deletion (`+++ /dev/null`);
    - a `\ No newline at end of file` line inside a hunk;
    - `+++ "b/we\"ird.txt"`;
    - `+++ b/has space.txt\t`;
    - an added line whose content is `++ looks like a header`.

    A hunk whose counts exceed its body is an error.
  - **`TestParseOutgoingLog`** (pure): two `-z` records `<40hex>\x01msg\n` give two commits. A record without `\x01` is an error.
  - **`TestPushRefusesNonBranchRefs`** (pure, `newRealClient`, `assertKind`): both pushes refuse `refs/docket/finalize/7/orig`, `refs/dckt/x`, and `refs/tags/v1` with `KindInvalidRequest`.
  - **`TestIntegrationRepoReadOutgoing`** (use `newMainModeRepos`, `mustDiscover`, `gitOut`):
    1. In the invocation clone, `base` = `main`.
    2. On a branch, commit (a) `Add notes\n\nBody (0007)\n` adding `notes.txt` (`a\nb\n`), `has space.txt`, `café.txt`, and `we"ird.txt`. Commit (b) `Edit notes`, setting `notes.txt` line 2 to `B` and appending `c`.
    3. A writer pushes an `upstream.txt` (`docket\n`) commit to origin `main`. Fetch it and `git merge --no-edit origin/main` into the branch; that is `head`.
    4. `ReadOutgoing(<fetched origin/main>, head)` gives:
       - `Commits` = (a), (b), and the merge (not the upstream commit);
       - `AddedPaths` = the four files;
       - `AddedLines` ⊇ `{notes.txt,2,B}`, `{notes.txt,3,c}`, plus one line each for the three odd paths, with none from `upstream.txt`.
    5. An unrelated orphan commit as `base` returns an error, never an empty result.
  - **`TestIntegrationRepoListRemoteRefs`:**
    - The writer pushes `refs/heads/docket`, `refs/heads/dckt`, `refs/docket/x/y`, and `refs/heads/feature/q`.
    - The patterns `refs/heads/docket`, `refs/heads/dckt`, and `refs/docket/*` return exactly the first three, with their oids.
    - An unreachable remote is an error.
- [ ] **Step 2: Run the tests and watch them fail.** Run `go test ./internal/gitcli/ -run 'ParseAddedLines|ParseOutgoingLog|PushRefusesNonBranchRefs' -count=1` and `go test -tags integration ./internal/gitcli/ -run '^TestIntegrationRepo(ReadOutgoing|ListRemoteRefs)' -count=1`.
- [ ] **Step 3: Implement.**
  - **`ReadOutgoing`:**
    - `validateObjectID` both ids. On failure: `KindInvalidRequest`, op `readOutgoingOp Operation = "read-outgoing"`.
    - Run three commands in `repo.PrimaryWorktree`. A non-zero exit is `KindCommandFailed` with `stderrExcerpt`; a parse error is `KindInvalidOutput`.
      1. `log -z --format=%H%x01%B <base>..<head>`, then `parseOutgoingLog`. Split on NUL, skip empty records, split each on `trailerSeparator`, and `validateObjectID` the hash.
      2. `diff --name-status -z --no-renames --diff-filter=A <base>...<head>`, then `parseAddedPaths`. The tokens alternate `A` and a path. An odd count, or a status other than `A`, is an error.
      3. `-c core.quotePath=false diff --no-renames --no-color --no-ext-diff --no-textconv --no-relative --src-prefix=a/ --dst-prefix=b/ -U0 <base>...<head>`, then `parseAddedLines`.
    - The three-dot merge-base diff is what keeps merged-in upstream lines out. Doc-comment that, and that every failure (a missing merge base included) is an error, never an empty result.

```go
func parseAddedLines(patch []byte) ([]OutgoingLine, error) {
	var out []OutgoingLine
	lines := strings.Split(string(patch), "\n")
	path := ""
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case strings.HasPrefix(l, "diff --git "):
			path = ""
		case strings.HasPrefix(l, "+++ "):
			p, err := patchPath(strings.TrimPrefix(l, "+++ "))
			if err != nil {
				return nil, err
			}
			path = p
		case strings.HasPrefix(l, "@@ "):
			oldCount, newStart, newCount, err := parseHunkHeader(l)
			if err != nil {
				return nil, err
			}
			removed, added := 0, 0
			for removed < oldCount || added < newCount {
				i++
				if i >= len(lines) {
					return nil, errors.New("gitcli: patch hunk is truncated")
				}
				body := lines[i]
				switch {
				case strings.HasPrefix(body, `\`): // "\ No newline at end of file"
				case strings.HasPrefix(body, "-"):
					removed++
				case strings.HasPrefix(body, "+"):
					out = append(out, OutgoingLine{Path: path, Line: newStart + added, Text: body[1:]})
					added++
				default:
					return nil, errors.New("gitcli: unexpected line inside a patch hunk")
				}
				if removed > oldCount || added > newCount {
					return nil, errors.New("gitcli: patch hunk exceeds its header counts")
				}
			}
		}
	}
	return out, nil
}

// parseHunkHeader reads "@@ -a[,b] +c[,d] @@…"; an omitted count is 1.
func parseHunkHeader(l string) (oldCount, newStart, newCount int, err error) {
	f := strings.Fields(l)
	if len(f) < 3 || !strings.HasPrefix(f[1], "-") || !strings.HasPrefix(f[2], "+") {
		return 0, 0, 0, errors.New("gitcli: malformed hunk header")
	}
	if _, oldCount, err = hunkRange(f[1][1:]); err != nil {
		return 0, 0, 0, err
	}
	newStart, newCount, err = hunkRange(f[2][1:])
	return oldCount, newStart, newCount, err
}

// hunkRange parses "start[,count]" with strconv.Atoi (count defaults to 1); a
// non-number is an error.

// patchPath: /dev/null is ""; a C-quoted path is unquoted; b/ is required.
func patchPath(s string) (string, error) {
	s = strings.TrimSuffix(s, "\t")
	if s == "/dev/null" {
		return "", nil
	}
	if strings.HasPrefix(s, `"`) {
		u, err := strconv.Unquote(s)
		if err != nil {
			return "", fmt.Errorf("gitcli: unquoting patch path: %w", err)
		}
		s = u
	}
	p, ok := strings.CutPrefix(s, "b/")
	if !ok {
		return "", errors.New("gitcli: patch path lacks the b/ prefix")
	}
	return p, nil
}
```

    An added `++ x` line arrives as `+++ x` **inside** a hunk. The counted loop consumes it, never the header case.
  - **`ListRemoteRefs`:**
    - Run `ls-remote --refs <remote> <patterns...>` with `network: true`.
    - Validate the remote, and each pattern (non-empty, not starting with `-`).
    - Parse strictly, like `ListRemoteHeads` (`splitLsRemoteLine`, `validateObjectID`, `validateRefName`, a duplicate is invalid output), allowing any `refs/` name.
    - No match gives an empty non-nil map.
  - **`push.go`:** after `validateRefName(ref)` in both pushes, add:
    `if !strings.HasPrefix(string(ref), "refs/heads/") { return PushOutcome{}, newFailure(<that function's op>, KindInvalidRequest, "docket pushes branches only; a ref outside refs/heads/ (its local refs/docket/ scratch refs included) never leaves the clone", nil) }`.
- [ ] **Step 4: Run the tests and confirm they pass.** Run the Step 2 commands, plus `go test ./internal/gitcli/ -count=1` and `go test -tags integration ./internal/gitcli/ -run '^TestIntegrationRepo' -count=1`. If an existing caller pushes a non-branch ref, return `NEEDS_ESCALATION` naming it. Never widen the guard.
- [ ] **Step 5: Mutation-test.**
  - Dropping the over-count check reddens the truncated or overlong row.
  - Skipping `Unquote` reddens the quoted-header row.
  - A two-dot diff reddens the merge-from-main assertion.
  - Removing the push guard reddens `TestPushRefusesNonBranchRefs`.
- [ ] **Step 6: Commit.** Message: `feat(gitcli): read what a push exposes, list named remote refs, push branches only`.

---

### Task 4: Private PR text and spec copy, and the prepare context's `visibility`

**Build tier:** standard

**Files:**
- `internal/app/workspace_spec.go`, plus its callers (`grep -rn 'specCopyBytes(' internal/`).
- `internal/app/pr_publish.go`.
- `internal/app/repository_prepare.go`.
- Tests: `workspace_spec_test.go`, `repository_prepare_test.go`, and `workflow_private_integration_test.go`.

**Interfaces (produces):**

```go
func specCopyBytes(spec []byte, c domain.Change, mode layout.Mode) ([]byte, error) // private: no change line
func resolvePRChange(ctx context.Context, deps PlanningDeps, repoDir string, id int) (domain.Change, StatusPin, *PRPublishResult)
// PrepareContext gains Visibility string `json:"visibility"` ("shared" | "private")
// test helper: func privateLifecycleChecks(t *testing.T, created ChangeCreateResult) workflowEntry
```

- [ ] **Step 1: Write the failing tests.**
  - **`TestSpecCopyPrivateOmitsChangeLine`:** `specFixture()` with `layout.Private` gives the spec minus its backlink, leading newlines trimmed, with no `Change ` line. `layout.Shared` equals the existing golden. Existing callers pass `layout.Shared`.
  - **`TestBuildPrepareContextVisibility`:** a shared layout gives `"shared"`. A `layout.PrivateLayout(...)` (as `repository_prepare_test.go` builds one) gives `"private"`.
  - **Extend `TestIntegrationWorkflowLifecyclePrivateInitToImplemented`** (acceptance 1). Pass `driveClaimToImplemented` the entry `privateLifecycleChecks(t, created)`. Its callback runs `gdeps := complete("")`, then:
    1. Gets the feature branch from `WorkspaceInspect(...).FeatureRef`.
    2. Gets the repository from `gdeps.Service.DiscoverRepository(ctx, node.dir)`, then calls `FindOpenPullRequestsByHead`.
    3. Expects exactly one PR, with `Body == "Authored PR prose for the widget.\n"` byte-for-byte and `Title == "Add the widget"`.
    4. Checks that the spec copy at the feature head (`git show <head>:<specPath>`) has no `Change 0` line.

    If the fake gh omits the body, return `NEEDS_ESCALATION`; never drop the assertion.
- [ ] **Step 2: Run the tests and watch them fail.** Run `go test ./internal/app/ -run 'SpecCopy|PrepareContext' -count=1` and `go test -tags integration ./internal/app/ -run '^TestIntegrationWorkflowLifecyclePrivate' -count=1`.
- [ ] **Step 3: Implement.**
  - **`specCopyBytes`:** after removing the backlink, `text := strings.TrimLeft(string(body), "\r\n")`. When `mode == layout.Private`, return `[]byte(text), nil`. Update the `specCopyChangeLine` comment to say a private repository omits it.
  - **`WorkspaceCommitSpec`:** pass `wc.pin.Layout.Mode`.
  - **`resolvePRChange`:** return `pin`.
  - **`PRPublish` (6)–(7):**

```go
	change, pin, refusal := resolvePRChange(ctx, deps, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}
	body := []byte(req.Body) // a private repository publishes the authored prose verbatim
	if pin.Layout.Mode != layout.Private {
		link := linkContextOf(pin)
		backlink, err := render.BacklinkContent(change, link)
		if err != nil {
			return prRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
		}
		if body, err = assemblePRBody([]byte(req.Body), backlink, render.PRArtifactLinksContent(change, link)); err != nil {
			return prRefusal(ResultInvalidState, ReasonPRBodyAssemblyFailed, err.Error(), req.ID)
		}
	}
```

    Say so in the file header too.
  - **`buildPrepareContext`:** `Visibility: string(sc.layout.Mode)`.
- [ ] **Step 4: Run the tests and confirm they pass.** Run the Step 2 commands, plus `-run '^TestIntegrationChangeRuntimePRPublish'`. The shared golden is unchanged.
- [ ] **Step 5: Mutation-test.**
  - Forcing the shared branch reddens the lifecycle body assertion.
  - Passing `layout.Shared` from `WorkspaceCommitSpec` reddens the spec-copy assertion.
- [ ] **Step 6: Commit.** Message: `feat(app): private repositories publish verbatim PR prose and a line-free spec copy`.

---

### Task 5: No finalize-block comment and no backlink repoint in a private repository

**Build tier:** standard

**Files:**
- `internal/app/finalize_block.go` (`FinalizeBlock`).
- `pr_backlink_repoint.go` (add `prBacklinksApply`).
- `finalize_closeout.go` (`runCloseoutPRBacklinkLeg`).
- `finalize_cleanup.go` (`finalizeCleanupPRBacklinkRepair`).
- `maintenance.go` (`gatherSweepSharedFacts` gains `lay layout.Layout`; its caller passes `pin.Layout`).
- `repository_repair_prbacklinks.go` (`runPRBacklinkRepair`).
- `internal/cli/finalize.go` (block `Short`).
- Tests: `maintenance_assess_test.go`, the closeout-leg unit file (`grep -rln 'newFakePRBody' internal/app`), `repository_private_integration_test.go`, and `privateLifecycleChecks`.

**Interfaces (produces):** `func prBacklinksApply(lay layout.Layout) bool`, which is true unless private. It is the one predicate every repoint site consults.

- [ ] **Step 1: Write the failing tests.**
  - **`TestGatherSweepSharedFactsSkipsPRBodiesInPrivateRepository`:** mirror `TestGatherSweepSharedFactsMarksUnreadPRBodiesUnknown` with a `layout.PrivateLayout("/c", "/r", "/d", "o-r")`. Expect `!prBodiesGathered` and `batch.calls == 0`. The existing test passes the shared layout.
  - **`TestFinalizeCleanupPRBacklinkRepairSkipsPrivateRepository`:** mirror the cleanup leg of `TestLegacyArchivedPRBacklinkIsFullSweepWorkOnly` with `cc.pin.Layout` private. Expect a nil finding and no read or edit.
  - **`TestCloseoutPRBacklinkLegSkipsPrivateRepository`:** `runCloseoutPRBacklinkLeg` with a private `cc.pin.Layout` makes no read or edit and returns nil.
  - **`TestIntegrationRepoSetupPrivatePRBacklinkRepairIsNoOp`:**
    - After `init --private`, run repair with the PR-backlinks option (`grep -n 'PRBacklinks' internal/app/repository_repair*.go`).
    - Use a `RepairGitHub` fake whose every method calls `t.Fatalf`.
    - The result is `no-op`, and the human text says a private repository's pull requests carry no backlink.
  - **`privateLifecycleChecks`** (acceptance 6). After `complete("")`, call `FinalizeBlock` with:
    - `FinalizeDeps{Planning: node.deps, GitHub: gh, Workspace: svc}`, where `svc` is from `workspace.NewService(node.deps.Client)` and `gh := &fakeBlockGitHub{repo: prRepo(), commentOutcome: <the created outcome constant>, commentURL: "https://example.invalid/c/1"}`;
    - a `BlockRequest` with `ID`, `Revision` (`blobRevisionAt` on the store, branch `dckt`), `PRNumber` from the open PR, `Attempt: "a1"`, `Reason: "gate-failed"`, the feature `Head`, and `Report: "The gate failed.\n"`.

    Expect: applied, `recorded`, `gh.ensureCalls == 0`, and `CommentURL == ""`. The record at the store tip has `## Finalize blocked` and no `- Comment:` line.
- [ ] **Step 2: Run the tests and watch them fail.**
- [ ] **Step 3: Implement.**
  - **`prBacklinksApply`:** `return lay.Mode != layout.Private`. Doc comment: a private repository's PR descriptions carry no docket blocks, so nothing is repointed and no PR body is read for it.
  - **Closeout and cleanup legs:** `if !prBacklinksApply(cc.pin.Layout) { return nil }` first. Hand-built unit `closeoutContext`s have a zero `Mode` (`""`), so they stay shared.
  - **`gatherSweepSharedFacts`:** wrap only the PR-body block (from `var numbers []int` through the bodies loop, `prBodiesGathered = true` included) in `if prBacklinksApply(lay) { … }`.
  - **`runPRBacklinkRepair`:** after `repairPreflight`, `if !prBacklinksApply(sc.layout) { return prRepairPrivateNoOp(metadataTip) }`. That is a no-op whose human text is `repository repair --pr-backlinks: a private repository's pull requests carry no backlink; nothing to repoint`.
  - **`FinalizeBlock`:** `url := ""`; wrap the existing `DiscoverRepository` / `EnsureComment` / unknown handling unchanged in `if pin.Layout.Mode != layout.Private { … }`, assigning `url`. The header comment says a private repository posts no comment; the marker transaction is its only effect.
  - **Block `Short`:** `Record a blocked finalize attempt: an owned PR comment (none in a private repository), then a durable marker`.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/app/ ./internal/cli/ -count=1` and the tagged `'^TestIntegrationWorkflowLifecyclePrivate'`, `'^TestIntegrationRepoSetupPrivate'`, and `'^TestIntegrationFinalizeOps'`.
- [ ] **Step 5: Mutation-test.**
  - `prBacklinksApply` returning true reddens the three skips and the repair test.
  - Dropping the `FinalizeBlock` condition reddens `ensureCalls == 0`.
- [ ] **Step 6: Commit.** Message: `feat(finalize): no PR comment or backlink repoint in a private repository`.

---

### Task 6: The leak check, wired into `workspace.publish`

**Build tier:** premium. A false negative is an irreversible leak.

**Files:**
- Create: `internal/app/leak_check.go` and `leak_check_test.go` (default tag).
- Modify: `workspace_ops.go` (`WorkspacePublish`; `WorkspaceOpResult.Leaks`).
- Create: `leak_check_private_integration_test.go` (prefix `TestIntegrationWorkflowLifecycle`).
- Maybe modify: `tests/runtime-budgets.tsv`.

**Interfaces:**
- Consumes: Task 1's `leakscan`, Task 3's `ReadOutgoing`/`Outgoing`, and Task 2's `LeakCheck.MatchWord`.
- Produces:

```go
const (
	ReasonLeakDetected        = "leak-detected"         // blocked; nothing published
	ReasonLeakCheckUnverified = "leak-check-unverified" // external-failed; the check could not run
)
type LeakHit struct {
	Source string `json:"source"`
	Commit string `json:"commit,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Text   string `json:"text"` // matched text only
	Rule   string `json:"rule"`
}
type leakGit interface {
	FetchBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error)
	ReadOutgoing(ctx context.Context, repo gitcli.Repository, base, head gitcli.ObjectID) (gitcli.Outgoing, error)
}
func leakCheckApplies(lay layout.Layout) bool
func leakOptions(pin StatusPin, snap domain.Snapshot) leakscan.Options
func runLeakCheck(ctx context.Context, git leakGit, wc workspaceContext, target workspace.Target, head gitcli.ObjectID, pr *leakscan.PRText) ([]LeakHit, error)
func leakMessage(hits []LeakHit) string
// WorkspaceOpResult gains Leaks []LeakHit `json:"leaks,omitempty"`
```

- [ ] **Step 1: Write the failing unit tests.** `fakeLeakGit` records the fetch remote/branch and the `(base, head)` read, and returns scripted values or errors. Build `wc` from `prReader(t)`:
  - `pin`, with `Config.Effective.LeakCheck.MatchWord` set **explicitly** true;
  - `snap` via `repository.BuildSnapshot` over `parseCorpus(reader.corpus)` (change 7 exists);
  - `repo{PrimaryWorktree: "/r", CommonDir: "/r/.git"}`;
  - `target.BaseRef = "refs/heads/main"`.

  The tests:
  - **`TestRunLeakCheckScansFetchedBaseToHead`:**
    - The fetch gives `B`.
    - The read gives a commit `{c1, "Fix it (0007)\n"}` and a line `{a.go, 3, "x := \".docket/y\""}`.
    - The PR title is `change 0007`, the body `ok\n`.

    Expect a fetch of `("origin", "refs/heads/main")`, a read of `(B, head)`, and three hits: `c1` commit-message line 1 `change-ref`, `a.go:3` `path`, and `pr-title` line 1 `change-ref`.
  - **`TestRunLeakCheckFetchFailureIsUnverified`:** an error, and no read happens.
  - **`TestRunLeakCheckReadFailureIsUnverified`:** an error.
  - **`TestRunLeakCheckMatchWordOff`:** `the docket number` gives no hit; `dckt` gives one.
  - **`TestLeakCheckAppliesOnlyPrivate`:** shared and `""` give false; private gives true.
  - **`TestLeakMessageBoundsAndNamesHits`:** 7 hits name 5 and say `… and 2 more`.
- [ ] **Step 2: Write the failing integration tests** (`//go:build integration`; never `t.Parallel`, because `planningDepsFor` uses `t.Setenv`). The fixture:

```go
type leakWorkspace struct {
	node                        realNode
	wdeps                       WorkspaceDeps
	id                          int
	wp, specCommit              string // feature worktree; spec-copy commit
	origin, writer, invocation  string // bare origin; a clone with origin set; the invocation clone
}
func prepareLeakWorkspace(t *testing.T, repo *gitRepo, branch string, id int, recPath string) *leakWorkspace
func newPrivateLeakWorkspace(t *testing.T) (*leakWorkspace, layout.Layout)
func newSharedLeakWorkspace(t *testing.T) *leakWorkspace
```

  - **`prepareLeakWorkspace`** runs the first steps of `driveClaimToImplemented` in order: context, claim (`RunContext: ""`), reconcile, `WorkspacePrepare`, and `WorkspaceCommitSpec`. Each `t.Fatalf`s unless applied. Do not edit the driver.
  - **`newPrivateLeakWorkspace`** repeats the private lifecycle test's setup through groom (spec `"# Widget: design\n\nBuild the widget.\n"`, change id 1). It then uses `repo.meta` = the store and branch `dckt`.
  - **`newSharedLeakWorkspace`** uses `buildConfiguredRepoWith(t, planRepoModes()[0], …)` like `runClaimToImplemented` (id 3).
  - **Helpers:** `commitInWorkspace(t, wp, msg, files) string`, `resetWorkspace(t, wp, commit)` (`git reset -q --hard`), and `originHasBranch(t, origin, ref) bool`.

  The tests:
  - **`TestIntegrationWorkflowLifecyclePrivateLeakCheckBlocksSeededLeaks`** (acceptance 2 push half, and 8). Each seeded row starts with `resetWorkspace(specCommit)`, then calls `WorkspacePublish(..., WorkspacePublishRequest{ID, Head})`, expects `blocked`/`leak-detected` with the hit shown, and checks the feature ref is **absent** on origin:

    | subtest | seed | hit |
    |---|---|---|
    | `subject-change-id` | msg `Add the widget (0001)` | `{commit-message, Commit head, Line 1, change-ref, "(0001)"}` |
    | `added-docket-path` | `notes.txt` = `see .docket/x\n` | `{added-line, notes.txt, 1, path}` |
    | `spec-copy-word` | append `Tracked in docket.\n` to the spec copy | `{added-line, <specPath>, word}` |
    | `dckt-in-message` | msg `wire the dckt alias` | `{commit-message, alias}` |

    Then, on the same fixture:
    - **`clean-year-and-adr-publishes`** (acceptance 4): msg `Lay out the widget per ADR-0001 (2026)` and a file with `See ADR-0001 (2026).` is applied, and origin's feature ref equals the head.
    - **`merged-main-is-not-outgoing`** (Review Focus 2): the writer pushes `upstream.txt` = `docket\n` to origin `main`; in `wp`, `git fetch -q origin main && git merge -q --no-edit origin/main`. Publish is applied.
  - **`TestIntegrationWorkflowLifecyclePrivateLeakCheckMatchWordOff`** (acceptance 3). A fresh fixture; append `leak_check:\n  match_word: false\n` to `lay.ConfigPath`.
    - Msg `File the docket number` with a line `the docket entry` is applied.
    - Then, reset to that published head each time (origin holds it). Each of these is still `leak-detected` with the rule shown:
      - a line `<!-- docket:backlink:start -->` (`marker`);
      - a trailer line `Docket-Operation: x` in the message (`trailer`);
      - a line `.docket/x` (`path`);
      - a msg `dckt` (`alias`);
      - a msg `(0001)` (`change-ref`).
  - **`TestIntegrationWorkflowLifecyclePrivateLeakCheckUnverifiedPushesNothing`** (Review Focus 3).
    1. `orphan := git commit-tree <specCommit>^{tree} -m "Unrelated root"`, then `git reset -q --hard <orphan>`. The feature branch now shares no history with `origin/main`, so the merge-base diff errors.
    2. `WorkspacePublish{Head: orphan}` gives `external-failed`/`leak-check-unverified`.
    3. The feature ref is absent on origin.

    If an earlier step refuses first with another reason, return `NEEDS_ESCALATION`; never weaken the assertion to "not applied".
  - **`TestIntegrationWorkflowLifecycleSharedPublishRunsNoLeakCheck`** (acceptance 5 pin). In the shared fixture (its spec copy carries `Change 0003 — …`), msg `Add the widget (0003)` plus a line `.docket/x` is applied, and the feature ref equals the head.
- [ ] **Step 3: Run the tests and watch them fail.** Run `go test ./internal/app/ -run 'LeakCheck|LeakMessage' -count=1` and `go test -tags integration ./internal/app/ -run '^TestIntegrationWorkflowLifecycle(Private|Shared)' -count=1`.
- [ ] **Step 4: Implement `leak_check.go`.** The header comment: before a push or PR write in a private repository, everything it would expose is scanned (`internal/leakscan`), and any hit refuses with nothing published. A shared repository never runs it. A fetch or read error is `leak-check-unverified`, never a clean scan. Each caller scans the exact head it publishes.

```go
var _ leakGit = (*gitcli.Client)(nil)

func leakCheckApplies(lay layout.Layout) bool { return lay.Mode == layout.Private }

func leakOptions(pin StatusPin, snap domain.Snapshot) leakscan.Options {
	ids := map[int]bool{}
	for _, c := range snap.Changes() {
		ids[int(c.ID())] = true
	}
	return leakscan.Options{MatchWord: pin.Config.Effective.LeakCheck.MatchWord.Value, ChangeIDs: ids}
}

func runLeakCheck(ctx context.Context, git leakGit, wc workspaceContext, target workspace.Target, head gitcli.ObjectID, pr *leakscan.PRText) ([]LeakHit, error) {
	base, err := git.FetchBranch(ctx, wc.repo, originRemote, target.BaseRef)
	if err != nil {
		return nil, fmt.Errorf("fetching the base %s from origin: %w", target.BaseRef, err)
	}
	out, err := git.ReadOutgoing(ctx, wc.repo, base.Commit, head)
	if err != nil {
		return nil, fmt.Errorf("reading what %s over %s would publish: %w", shortCommit(string(head)), shortCommit(string(base.Commit)), err)
	}
	in := leakscan.Input{AddedPaths: out.AddedPaths, PR: pr}
	for _, c := range out.Commits {
		in.Commits = append(in.Commits, leakscan.Commit{ID: string(c.Commit), Message: c.Message})
	}
	for _, l := range out.AddedLines {
		in.AddedLines = append(in.AddedLines, leakscan.AddedLine{Path: l.Path, Line: l.Line, Text: l.Text})
	}
	var hits []LeakHit
	for _, h := range leakscan.Scan(in, leakOptions(wc.pin, wc.snap)) {
		hits = append(hits, LeakHit{Source: string(h.Source), Commit: h.Commit, File: h.File, Line: h.Line, Text: h.Text, Rule: string(h.Rule)})
	}
	return hits, nil
}

```

  **`leakMessage`** builds `"<n> docket fingerprint(s) would reach a shared surface; nothing was published: "` followed by the first 5 hits joined with `"; "`, then `" … and <k> more (see leaks)"` when there are more. Each hit is `<where> %q (<rule>)`, where `<where>` is:
  - `commit <shortCommit> message line <n>` for a commit;
  - `<file>:<line>` for an added line;
  - `path <file>` for an added path;
  - `<source> line <n>` otherwise.

  **`WorkspacePublish`:** after the head-mismatch refusal and **before** `admitWorkflowMutation`:

```go
	if leakCheckApplies(wc.pin.Layout) { // scan exactly req.Head, which PublishHead re-proves as ExpectedHead
		hits, lerr := runLeakCheck(ctx, deps.Client, wc, target, gitcli.ObjectID(req.Head), nil)
		if lerr != nil {
			return newWorkspaceResult(OperationWorkspacePublish, ResultExternalFailed, WorkspaceOpResult{ID: req.ID, Head: req.Head,
				Reason: ReasonLeakCheckUnverified, Message: "the private-repository leak check could not run; nothing was pushed: " + lerr.Error()})
		}
		if len(hits) > 0 {
			return newWorkspaceResult(OperationWorkspacePublish, ResultBlocked, WorkspaceOpResult{ID: req.ID, Head: req.Head,
				Reason: ReasonLeakDetected, Message: leakMessage(hits), Leaks: hits})
		}
	}
```

  If a schema describability guard reddens on the new field, follow its own remedy message.
- [ ] **Step 5: Run the tests and confirm they pass.** Run the Step 3 commands and the whole `-run '^TestIntegrationWorkflowLifecycle'` shard. The existing private lifecycle test must pass with the check on. If it is refused, return `NEEDS_ESCALATION` with the hits; never loosen a rule.
- [ ] **Step 6: Mutation-test** (acceptance 8; record each red message):
  - Deleting the `WorkspacePublish` block reddens every seeded row.
  - `leakCheckApplies` returning true reddens the shared pin.
  - `runLeakCheck` returning `nil, nil` on a read error reddens the unverified test.
- [ ] **Step 7: Check the budget.** Run the workflowlifecycle shard serially against `tests/runtime-budgets.tsv` (55s). Over budget: raise it to the next multiple of 5 plus 5s. Record the margin as a number.
- [ ] **Step 8: Commit.** Message: `feat(app): block a private repository's feature push on a docket fingerprint`.

---

### Task 7: The leak check in `pr.publish` and `finalize.publish`

**Build tier:** premium

**Files:**
- `internal/app/pr_publish.go` (`PRPublishResult.Leaks`).
- `finalize_publish.go` (`FinalizePublishResult.Leaks`).
- `leak_check_private_integration_test.go` and `privateLifecycleChecks`.
- `internal/cli/pr.go`, `workspace.go`, and `finalize.go` (`Short`).

**Interfaces:**
- Consumes: Task 6's API, and Task 4's `resolvePRChange` returning the pin.
- Produces: `Leaks []LeakHit \`json:"leaks,omitempty"\`` on both results.

- [ ] **Step 1: Write the failing tests.**
  - **`TestIntegrationWorkflowLifecyclePrivateLeakCheckGatesPR`** (acceptance 2 PR half, and 1). Use a private fixture. Commit a clean change and publish it, then use evidence from `prEvidenceBytes(t, head)`. Each row uses `gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("x")}}`:
    - Title `Add the widget for change 0001` gives `blocked`/`leak-detected` with `{pr-title, 1, change-ref}`, and `len(gh.ensureCalls) == 0`.
    - Body `Plain.\nSee .docket/notes for more.\n` gives `leak-detected` with `{pr-body, 2, path, ".docket"}`. The `json.Marshal` of the result does **not** contain `See .docket/notes for more`, and nothing was ensured.
    - Title `Add the widget`, body `Plain prose.\n` is applied, with `gh.ensureCalls[0].Body == "Plain prose.\n"` and `.Title == "Add the widget"` exactly.
  - **Extend `privateLifecycleChecks`:**
    - `leaky := git commit-tree <head>^{tree} -p <head> -m "Finalize (0001)"` (the branch does not move).
    - `FinalizePublish(ctx, FinalizeDeps{Planning: node.deps, GitHub: gh, Workspace: svc}, node.dir, FinalizePublishRequest{ID, Attempt: "a1", Head: leaky, EvidenceRecord: <evidence.Render of evidence.NewRecord("gate", leaky, now)>})` gives `blocked`/`leak-detected`, naming `leaky`.
    - The same call with the clean head gives `blocked`/`no-rebase-receipt`. That proves the scan passed and the flow reached the receipt.
- [ ] **Step 2: Run the tests and watch them fail.**
- [ ] **Step 3: Implement.**
  - **`PRPublish`**, after the body is assembled and **before** the `MutationPublication`/`admitWorkflowMutation` block:

```go
	if leakCheckApplies(pin.Layout) {
		wc, wref := loadWorkspaceContext(ctx, deps, repoDir, req.ID, OperationPRPublish)
		if wref != nil {
			return prRefusal(wref.Result, wref.Reason, wref.Message, req.ID)
		}
		target, tref := resolveWorkspaceTarget(OperationPRPublish, wc)
		if tref != nil {
			return prRefusal(tref.Result, tref.Reason, tref.Message, req.ID)
		}
		hits, lerr := runLeakCheck(ctx, deps.Client, wc, target, gitcli.ObjectID(req.Head), &leakscan.PRText{Title: req.Title, Body: string(body)})
		if lerr != nil {
			return prRefusal(ResultExternalFailed, ReasonLeakCheckUnverified,
				"the private-repository leak check could not run; no pull request was created or edited: "+lerr.Error(), req.ID)
		}
		if len(hits) > 0 {
			r := prRefusal(ResultBlocked, ReasonLeakDetected, leakMessage(hits), req.ID)
			r.Leaks = hits
			return r
		}
	}
```

    The header's "Redaction" bullet says a leak hit carries only the matched token and a line number.
  - **`FinalizePublish`**, after `metaDir := …` and **before** `ReadRebaseReceipt`: the same shape with `deps.Planning.Client`, `wc`, `target`, and `gitcli.ObjectID(req.Head)`, `pr` nil:
    - an error is `publishRefusal(ResultExternalFailed, PublishDispBlocked, ReasonLeakCheckUnverified, "…; nothing was pushed: "+err, id)`;
    - hits are `publishRefusal(ResultBlocked, PublishDispBlocked, ReasonLeakDetected, leakMessage(hits), id)` with `.Leaks = hits`.
  - **`Short` help:** append ` (leak-checked in a private repository)` to `pr publish`, `workspace publish`, and `finalize publish`. Update any `internal/cli` test pinning those strings (grep the current text).
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/app/ ./internal/cli/ -count=1` and the tagged `'^TestIntegrationWorkflowLifecycle'`, `'^TestIntegrationChangeRuntimePRPublish'`, and `'^TestIntegrationFinalizeOps'`.
- [ ] **Step 5: Mutation-test.**
  - Deleting the `PRPublish` block reddens the title and body rows.
  - Deleting the `FinalizePublish` block turns the leaky row into `no-rebase-receipt` (red).
- [ ] **Step 6: Commit.** Message: `feat(app): leak-check private PR writes and finalize pushes`.

---

### Task 8: Report docket-named refs on a private repository's `origin`

**Build tier:** standard

**Files:**
- Create: `internal/app/repository_shared_remote.go` and `_test.go`.
- Modify: `repository_check.go` (`RunRepositoryCheck`), `repository_prepare.go` (`prepareContextResult`), and `workspace_ops.go` (`WorkspacePublish`; `WorkspaceOpResult.Findings`).
- Extend: `repository_private_integration_test.go` and `leak_check_private_integration_test.go`.

**Interfaces:**
- Consumes: `ListRemoteRefs` (Task 3).
- Produces:

```go
const FindingMetadataOnSharedRemote = "metadata-on-shared-remote"
type remoteRefLister interface {
	ListRemoteRefs(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, patterns []string) (map[gitcli.RefName]gitcli.ObjectID, error)
}
func metadataRefsOnOrigin(ctx context.Context, g remoteRefLister, repo gitcli.Repository) ([]string, error) // sorted
func sharedRemoteMetadataFindings(ctx context.Context, g remoteRefLister, lay layout.Layout, repo gitcli.Repository) []reposetup.Finding
func sharedRemoteMetadataStatusFindings(ctx context.Context, g remoteRefLister, lay layout.Layout, repo gitcli.Repository) []StatusFinding
// WorkspaceOpResult gains Findings []StatusFinding `json:"findings,omitempty"`
```

- [ ] **Step 1: Write the failing tests.**
  - **`TestSharedRemoteMetadataFindings`** (fake lister):
    - Shared: nil, and the lister is never called.
    - Private, with `refs/heads/docket`, `refs/heads/dckt`, `refs/docket/finalize/7/orig`, and `refs/heads/docketeer`: three warnings, `Ref` sorted, no `Repairable`.
    - Private with a lister error: one `metadata-on-shared-remote-unverified` warning saying the refs were not proven absent.
  - **`TestIntegrationRepoSetupPrivateMetadataOnSharedRemote`** (acceptance 7):
    1. After init `--private` and configure-tests, check has no such finding.
    2. The writer pushes `refs/heads/docket` and `refs/docket/x` to origin.
    3. Check has exactly those two warnings, and `RepositoryState` is unchanged.
    4. Prepare (applied or no-op) carries the same two findings.
  - **Leak test:** after the clean publish in `…BlocksSeededLeaks`, push `refs/heads/dckt` to origin. A further clean `WorkspacePublish` is applied with one `metadata-on-shared-remote` finding.
- [ ] **Step 2: Run the tests and watch them fail.**
- [ ] **Step 3: Implement.**
  - **`metadataRefsOnOrigin`:** call `ListRemoteRefs(..., originRemote, []string{"refs/heads/" + layout.SharedName, "refs/heads/" + layout.PrivateName, "refs/" + layout.SharedName + "/*"})`. Keep exact `refs/heads/docket`, exact `refs/heads/dckt`, and the `refs/docket/` prefix, then sort.
  - **The finding** (severity warning, `Ref` = the ref):
    - Message: `origin holds <ref>, a docket-named ref, but this repository is private; everyone with access to origin can see it.`
    - Remedy: ``If nothing still needs it, delete it from origin: `git push origin --delete <ref>`.``
  - **The unverified finding:**
    - Message: `origin could not be listed for docket-named refs (unverified, not proven absent): <err>`.
    - Remedy: `Restore access to origin, then re-run docket repository check.`
  - **`sharedRemoteMetadataStatusFindings`:** the same content as `StatusFinding{Code, Severity: "warning", Path: ref, Message, Remedy}`.
  - **`RunRepositoryCheck`:** beside `privateCheckFindings(sc)` (inside the `PresencePresent` block), append `sharedRemoteMetadataFindings(ctx, d.Git, sc.layout, sc.repo)...`. The state is unaffected.
  - **`prepareContextResult`:** `out.Findings = sharedRemoteMetadataFindings(ctx, git, sc.layout, sc.repo)`. Append `- [warning] <code> <ref>` lines to `out.human`.
  - **`WorkspacePublish`:**
    - Declare `var warn []StatusFinding` before the leak-check block.
    - Inside the private branch, after the check passes, set `warn = sharedRemoteMetadataStatusFindings(ctx, deps.Client, wc.pin.Layout, wc.repo)`.
    - Set `out.Findings = warn` on both results returned after `PublishHead`. It never blocks.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/app/ -count=1` and the tagged `'^TestIntegrationRepoSetupPrivate'`, `'^TestIntegrationRepoPrepare'`, `'^TestIntegrationRepoCheck'`, and `'^TestIntegrationWorkflowLifecycle'`.
- [ ] **Step 5: Mutation-test.**
  - Dropping the exact filter reddens `docketeer`.
  - Returning nil on an error reddens the unverified row.
  - Removing the check-site append reddens acceptance 7.
- [ ] **Step 6: Commit.** Message: `feat(repository): report docket-named refs on a private repository's origin`.

---

### Task 9: Skill prose: the writing rule, the visibility hand-off, the leak-detected halt, and the guard

**Build tier:** standard

**Files:**
- `skills/docket-convention/SKILL.md`
- `skills/docket-implement-next/SKILL.md`, plus `references/edge-paths.md` and `references/fix-pass.md`
- `skills/docket-build/SKILL.md` and `skills/docket-build-task/SKILL.md`
- `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, and `skills/docket-brainstorm/SKILL.md`
- `skills/docket-finalize-change/SKILL.md`
- `agents/docket-plan-writer.md`
- `internal/assets/embedded/` (regenerated)
- Create: `internal/repoguard/private_writing_rule_test.go`
- `internal/repoguard/budgets_test.go`, plus any moved prose pin

**Interfaces:**
- Consumes: the prepare key `visibility` (Task 4); `leak-detected`, `leak-check-unverified`, and `leaks` (Tasks 6–7); `metadata-on-shared-remote` (Task 8).
- Produces: the anchor `**Private-repository writing rule.**` and the phrase `private-repository writing rule`.

- [ ] **Step 1: Write the failing guard `TestPrivateWritingRuleReachesEveryWriter`.**
  - **The anchor.** Docket-convention has `**Private-repository writing rule.**` exactly once. Within the 600 bytes after it, it has `change ids` and `metadata branch` (prose-guard-binds-phrase-to-claim).
  - **The writers.** Each of these 10 files contains `private-repository writing rule` (case-insensitive, whitespace collapsed): docket-build-task, docket-build, the docket-implement-next SKILL, `fix-pass.md`, `edge-paths.md`, docket-new-change, docket-groom-next, docket-auto-groom, docket-brainstorm, and `agents/docket-plan-writer.md`. Floor: 10.
  - **Producer anchors.** The implement-next SKILL has a blank-line paragraph containing both `leak-detected` and `change.halt`. The finalize-change SKILL has one containing `leak-detected` and `finalize.block`.

  Run it. Expected: FAIL.
- [ ] **Step 2: Write the prose.** Read each sentence as a worker in an unknown repository.
  - **docket-convention**, in *Branch model* after the private sentence, add:
    > **Private-repository writing rule.** In a private repository (`visibility: private` in the `repository.prepare` context), nothing that ships through the feature branch or the PR may contain the words "docket" or "dckt", docket marker comments, `Docket-` trailers, `.docket`/`dckt` paths, change ids or change references ("change 0612", "#0612", "(0612)"), docket ADR numbers, or references to the backlog or board. The metadata branch's own files (plan, results, records) are exempt and must not be copied from. `workspace.publish`, `pr.publish`, and `finalize.publish` enforce it with a blocking leak check (`leak-detected`); `leak_check.match_word: false` lets the bare word "docket" through. A shared repository is unaffected.

    The startup step that names the prepare-context keys also names `visibility`.
  - **docket-build-task**, *The commit*:
    > In a private repository (the dispatch payload says `Visibility: private`), every commit message, code line, comment, and test name follows the convention's private-repository writing rule; never copy a change id or docket wording from the plan. When the task cannot be done without breaking it, return `BLOCKED` naming the conflict — never commit the violation.
  - **docket-build:**
    - The tier-worker payload, inside its `docket:feature-dispatch` block (keep the `Feature worktree:` line), adds `Visibility: <shared|private>` from the prepare context.
    - One sentence: in a private repository the plan's suggested commit messages are held to the private-repository writing rule.
  - **docket-implement-next:**
    - The plan-writer payload (inside its feature-dispatch block) and the build invocation carry `Visibility`.
    - *PR-body assembly*: in a private repository, the authored title and body follow the private-repository writing rule. There is no `#<issue>` reference and no change id in the title, and `pr.publish` publishes the body verbatim.
    - Add the producer paragraph after *Publish the PR*:
      > **Leak check (private repositories).** `workspace.publish` and `pr.publish` refuse `leak-detected` (`blocked`) when anything they would expose carries a docket fingerprint; the result's `leaks` list names each hit (source, commit, file, line, rule, matched text). Nothing was pushed or opened. End the run `halted`: write the hit list into the `change.halt` report — never reword, amend, rebase, or force-push to get past it, and never retry. `leak-check-unverified` (`external-failed`) means the check could not run; treat it as any external failure. `metadata-on-shared-remote` warnings go in the run report only.
  - **edge-paths.md:** the issue-reference, back-link, and links paragraphs each add: "None in a private repository (private-repository writing rule); the body is published verbatim."
  - **fix-pass.md:** fix commits follow the private-repository writing rule; the fix-worker payload carries `Visibility`.
  - **docket-new-change, docket-groom-next, and docket-auto-groom** (where each writes the spec), and the **docket-brainstorm** compact brief:
    > In a private repository the spec ships with the PR, so its body follows the private-repository writing rule: no `Change #N, groomed …` line, no change or ADR numbers, no backlog references.
  - **docket-finalize-change:**
    - The block paragraph: "In a private repository no PR comment is posted; the marker is recorded on the change only."
    - The `finalize.publish` paragraph:
      > `leak-detected` (`blocked`): record it with `finalize.block` (reason `leak-detected`, the hit list as the report) and stop `halted`; never reword or force-push past it.
  - **agents/docket-plan-writer.md:** the payload sentence also names the repository's visibility. Add the line: "In a private repository, the commit messages, code, comments, and test names the plan prescribes follow the private-repository writing rule; the plan itself is metadata and may name the change."
- [ ] **Step 3: Move the pins.**
  - Run `grep -rn -e 'back-link line' -e '#<issue>' -e 'owned PR comment' -e 'feature-dispatch' internal/repoguard tests` and repoint any prose assert the edits moved.
  - Run `go generate ./internal/assets/`.
  - Ratchet the `skillBudgets` rows to the measured counts.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/repoguard/ ./internal/assets/ -count=1`: `TestProseContracts`, `TestAlignmentContracts`, `TestSkillSizeBudgets`, `TestEmbeddedMatchesAuthored`, any feature-dispatch guard, and the new guard.
- [ ] **Step 5: Mutation-test.** Each of these must go red:
  - delete the convention anchor;
  - delete the phrase from docket-build-task;
  - delete `change.halt` from the producer paragraph.
- [ ] **Step 6: Commit.** Message: `docs(skills): private-repository writing rule, visibility hand-off, and leak-detected halt`.

---

### Final: whole-suite gate

- [ ] **Step 1: Check the twins.** Run `go generate ./internal/assets/`, then `git status --porcelain`. Expected: empty.
- [ ] **Step 2: Run the build gate.** Run whatever `build.test_command` resolves to; read it from config. Expected: green. Act on `SERIAL CONFIRMED OVER BUDGET:`, and record `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines.
- [ ] **Step 3: Confirm the residue.** Run `grep -rn 'EnsureComment(' internal/app/*.go`. The only non-test call must sit inside `FinalizeBlock`'s shared branch; read it to confirm.
- [ ] **Step 4: Collect notes for the results file** (written by the parent):
  - every mutation and its red message;
  - the workflowlifecycle shard margin as a number;
  - the ADR above;
  - **residuals:**
    - binary file contents are unscanned (their paths are scanned);
    - the marker rule needs a letter after the colon;
    - change references match only `change[s] #N`, `change[s] NNNN+`, `#0NNN`, and `(0NNN)`;
    - the origin-ref report runs at `workspace.publish`, check, and prepare, while `finalize.publish` relies on the branch-only push guard;
    - local `refs/docket/…` scratch refs stay inside a private clone's `.git`;
  - **verified unchanged:** `finalize.publish`/`evidence.recertify` write no PR body; stacked retargeting carries no text.
