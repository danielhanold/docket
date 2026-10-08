<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0543 — Cursor section of the Bash upgrade guide, proven on the saved cases](../../changes/archive/2026-10-08-0543-cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved.md)**
<!-- docket:backlink:end -->

# Cursor Section of the Bash Upgrade Guide Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, one task per tier worker under
> the `docket-build-task` contract, with one whole-suite gate at the end. Steps use checkbox
> (`- [ ]`) syntax for tracking.

**Goal:** Extend `docs/release/upgrading-from-bash.md` from Claude Code only to one combined path
for Claude Code and Cursor. `TestBashUpgrade` (the `internal/bashupgrade` integration test) proves
the new path against the saved `v0.9.2` and `v0.9.3` cases.

**Architecture:** The guide's marked blocks (`<!-- upgrade-step: <name> -->`) are driven in order
by `runGuide` through `stepRegistry` actions (run / observe / mirror) in
`internal/bashupgrade/registry_test.go`. This change adds two marked steps
(`cursor-takeover-remedy`, `choose-harnesses`) and changes four (`install-binary`,
`takeover-remedy`, `leftovers`, `repo-agent-files`). It also adds the Cursor assertions to the
mirrors and to `TestIntegrationBashUpgradeGuide`. Each saved case runs once, through both
harnesses. No product code changes.

**Tech Stack:** Go tests (`testing`, `go.yaml.in/yaml/v3`), the `integration` build tag, Markdown.

**Spec:** `docs/superpowers/specs/2026-10-08-cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved-design.md`
(on the `docket` branch). Read it alongside this plan.

## Global Constraints

- The guide's install line pins `VERSION=v1.0.0-alpha.2`.
- No product code changes. If the test exposes something the **binary** does wrong, rather than
  something the guide can explain, stop. Return `BLOCKED` with the evidence. The bug gets its own
  fix change. Never patch `internal/install`, `internal/app`, `internal/reposeed` or
  `internal/harness` here. Never teach the frozen v0.9.2 floor v0.9.3's rule (ADR-0096).
- `internal/bashupgrade/bashupgrade_integration_test.go` stays `//go:build integration`. Helpers in
  `registry_test.go`, `steps_test.go` and `restore_test.go` stay untagged, so they compile in the
  default unit run.
- Every existing Claude Code step and assertion stays, including the guide-shape test and the
  finding/settings table checks. The only assertion this plan *re-keys* is the post-guide
  "CLAUDE.md still carries the Bash dispatch block" check (Task 3), and it says why.
- Guide wording describes current behavior only: no change numbers, no PR references.
- Prose the test asserts is matched whitespace-insensitively by the new `mustContainProse`
  (Task 1). Do not rewrap or edit the two existing raw-matched constants: `releaseVerifiedProse`
  (section 3, which carries an exact `\n`) and `repairPrepareOptionalProse` (section 5, one line).
  Their paragraphs stay byte-identical.
- Comments anchor on symbols or quoted clauses, never line numbers (ADR-0054,
  `TestCommentAnchorStyle`).
- Integration runs: `go test -tags integration -count=1 -timeout 30m -run '<regex>' ./internal/bashupgrade/`
  from the feature worktree root. Always pass `-count=1`, because a cached PASS proves nothing
  against a mutated tree.
- Mutation probes never edit the committed guide. Copy the guide into a templated temp dir
  (`mktemp -d "${TMPDIR:-/tmp}/bashupgrade-mut.XXXXXX"`), edit the copy, and point the test at it
  with `DOCKET_BASH_UPGRADE_GUIDE=<copy>`, the seam `guidePath` already reads. Test-code mutations
  restore from a backup copy (`cp -f`), never `git checkout --`, which would discard the
  uncommitted work under test.
- The whole-suite gate is `go run ./cmd/docket development test` (`build.test_command`). Read the
  budget report even on green. `internal/bashupgrade` gains a second harness per case, so watch
  for `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines naming it.
- Commit messages: conventional style (`test(bashupgrade): …`, `docs(release): …`). The repository
  is shared, so naming the guide and Cursor is fine.

## Review Focus

1. **A reader who uses only one harness.** The test runs only the combined path. A Cursor-only or
   Claude-only reader drops lines by following the prose, and what is left must still be a valid
   shape: `sh install.sh --harness cursor` and `--harnesses cursor`. Pinned in Task 1 by
   `TestInstallLineHarnesses` single-harness cases. Also pinned in Task 2 by the "skip this block"
   prose check, which the remedy mirror asserts.
2. **Block order inside section 4.** `cursor-takeover-remedy` must come *before* `takeover-remedy`,
   whose last line re-runs the installer. If it came after, the re-run would hit the Cursor
   conflicts. `installHandoff` already fails "install still conflicts after the guide's remedy" in
   that case. Task 2's mutation probe 1 proves it.
3. **The v0.9.2 Cursor rule.** On v0.9.2 the reader's `rm -f` deletes the rule before the
   installer could. So the test proves the installer's *view* (the rule exists and is not a
   conflict), not its act. The act is covered by `internal/install`'s
   `TestRetireCursorFileLegacyBytes` and `TestLegacyReproducer_CursorDispatchRule`. Pinned in
   Task 2.
4. **`configure-harnesses` pending-path wording.** If the binary's human output stops saying
   `review and commit the pending paths: `, `pendingPathsFrom` returns nil and `runChooseHarnesses`
   fails loudly. It never passes vacuously. Pinned in Task 1 (`TestPendingPathsFrom`) and Task 3.
5. **The repository Cursor rule stays out of commits.** `.cursor/rules/docket-dispatch.mdc` must be
   git-ignored by the block section 5 installs, or the reader commits a per-machine file. Pinned in
   Task 3 by `git check-ignore`, and by the clean `git status` after `choose-harnesses` and
   `repo-agent-files`.

---

### Task 1: Pure helpers for the combined path

Adds four untagged helpers to `registry_test.go`, with unit tests. Nothing calls them yet, so both
the default and the integration runs stay green.

**Files:**
- Modify: `internal/bashupgrade/registry_test.go` (add helpers after `mustContain`)
- Create: `internal/bashupgrade/registry_unit_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces (used by Tasks 2 and 3):
  - `func installLineHarnesses(line string) ([]string, bool)`: harness names in line order for
    `sh install.sh` followed by one or more `--harness <claude|codex|cursor|opencode>` pairs, with
    no repeats. Any other shape returns `ok=false`.
  - `func containsProse(text, want string) bool` and
    `func mustContainProse(t *testing.T, what, text, want string)`: substring match after
    collapsing every whitespace run to one space.
  - `const pendingPathsLead = "review and commit the pending paths: "` and
    `func pendingPathsFrom(out string) []string`: the comma-separated paths after the lead, up to
    end of line, sorted. Nil when the lead is absent.
  - `func sameSet(a, b []string) bool`: order-insensitive equality.
  - `var guideHarnesses = []string{"claude", "cursor"}`: the harnesses the combined path covers.

- [ ] **Step 1: Write the failing unit tests**

Create `internal/bashupgrade/registry_unit_test.go`:

```go
package bashupgrade

import (
	"strings"
	"testing"
)

func TestInstallLineHarnesses(t *testing.T) {
	for line, want := range map[string][]string{
		"sh install.sh --harness claude --harness cursor": {"claude", "cursor"},
		"sh install.sh --harness cursor":                  {"cursor"},
		"sh install.sh --harness claude":                  {"claude"},
		"sh install.sh  --harness cursor   --harness claude": {"cursor", "claude"},
	} {
		got, ok := installLineHarnesses(line)
		if !ok || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q: got %v, %v; want %v", line, got, ok, want)
		}
	}
	for _, line := range []string{
		"sh install.sh",
		"sh install.sh --harness",
		"sh install.sh --harness claude extra",
		"sh install.sh --harness claude --harness claude",
		"sh install.sh --harness vim",
		"sh install.sh --harnesses claude",
		"bash install.sh --harness claude",
		"sh ./install.sh --harness claude",
	} {
		if got, ok := installLineHarnesses(line); ok {
			t.Errorf("%q was accepted as an installer line (%v)", line, got)
		}
	}
}

func TestContainsProse(t *testing.T) {
	if !containsProse("one two\n  three\tfour", "two three four") {
		t.Error("a wrapped sentence did not match its one-line form")
	}
	if !containsProse("On `v0.9.2` the installer\nremoves that rule itself.", "On `v0.9.2` the installer removes that rule itself.") {
		t.Error("a wrapped guide sentence did not match")
	}
	if containsProse("one two three", "two four") {
		t.Error("different words matched")
	}
	if containsProse("onetwo", "one two") {
		t.Error("collapsing must not delete whitespace between words")
	}
}

func TestPendingPathsFrom(t *testing.T) {
	out := "agent harnesses set to `[claude, cursor]` (needs-review); review and commit the pending paths: CLAUDE.md, .docket.yml\nnext line\n"
	if got := pendingPathsFrom(out); strings.Join(got, ",") != ".docket.yml,CLAUDE.md" {
		t.Fatalf("got %v", got)
	}
	if got := pendingPathsFrom("agent harnesses set to `[claude]` (healthy)\n"); got != nil {
		t.Fatalf("no lead must give nil, got %v", got)
	}
}

func TestSameSet(t *testing.T) {
	if !sameSet([]string{"b", "a"}, []string{"a", "b"}) {
		t.Error("order mattered")
	}
	if sameSet([]string{"a"}, []string{"a", "b"}) || sameSet([]string{"a", "a"}, []string{"a", "b"}) {
		t.Error("different sets compared equal")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 -run 'TestInstallLineHarnesses|TestContainsProse|TestPendingPathsFrom|TestSameSet' ./internal/bashupgrade/`
Expected: FAIL to compile, `undefined: installLineHarnesses` (and the other three).

- [ ] **Step 3: Implement the helpers**

In `internal/bashupgrade/registry_test.go`, add after `mustContain`:

```go
// guideHarnesses are the harnesses the guide's combined path covers; the test runs
// every saved case once, through all of them.
var guideHarnesses = []string{"claude", "cursor"}

// installLineHarnesses parses the guide's installer invocation: exactly `sh install.sh`
// followed by one or more `--harness <name>` pairs, each name one the downloader
// accepts and none repeated. It returns the names in line order.
func installLineHarnesses(line string) ([]string, bool) {
	f := strings.Fields(line)
	if len(f) < 4 || len(f)%2 != 0 || f[0] != "sh" || f[1] != "install.sh" {
		return nil, false
	}
	known := map[string]bool{"claude": true, "codex": true, "cursor": true, "opencode": true}
	seen := map[string]bool{}
	var out []string
	for i := 2; i < len(f); i += 2 {
		if f[i] != "--harness" || !known[f[i+1]] || seen[f[i+1]] {
			return nil, false
		}
		seen[f[i+1]] = true
		out = append(out, f[i+1])
	}
	return out, true
}

// containsProse reports whether text contains want once every whitespace run in both
// is collapsed to one space, so a re-flowed guide paragraph still matches the claim.
func containsProse(text, want string) bool {
	return strings.Contains(strings.Join(strings.Fields(text), " "), strings.Join(strings.Fields(want), " "))
}

func mustContainProse(t *testing.T, what, text, want string) {
	t.Helper()
	if !containsProse(text, want) {
		t.Fatalf("%s does not contain %q (whitespace collapsed):\n%s", what, want, text)
	}
}

// pendingPathsLead introduces the paths `docket repository configure-harnesses` asks
// the reader to review and commit, in its human output.
const pendingPathsLead = "review and commit the pending paths: "

// pendingPathsFrom returns, sorted, the paths after pendingPathsLead up to the end of
// that line, or nil when the output carries no such list.
func pendingPathsFrom(out string) []string {
	_, rest, ok := strings.Cut(out, pendingPathsLead)
	if !ok {
		return nil
	}
	line, _, _ := strings.Cut(rest, "\n")
	var paths []string
	for _, p := range strings.Split(line, ",") {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// sameSet reports whether a and b hold the same strings, ignoring order.
func sameSet(a, b []string) bool {
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "\x00") == strings.Join(y, "\x00") && len(x) == len(y)
}
```

- [ ] **Step 4: Run the unit tests and the package's default run**

Run: `go test -count=1 ./internal/bashupgrade/`
Expected: PASS (the new tests plus every existing default-tag test).

Run: `go vet -tags integration ./internal/bashupgrade/`
Expected: no output (the integration build still compiles).

- [ ] **Step 5: Commit**

```bash
git add internal/bashupgrade/registry_test.go internal/bashupgrade/registry_unit_test.go
git commit -m "test(bashupgrade): add install-line, prose and pending-path helpers for the Cursor guide path"
```

---

### Task 2: The machine half: install for both harnesses and take over Bash's Cursor install

This covers guide sections 1, 3, 4, 7 (the old-checkout links paragraph and the sandbox sentence)
and 8. It also changes the mirrors that drive them. Test first: change the mirrors, watch the
unchanged guide fail, then edit the guide.

**Files:**
- Modify: `internal/bashupgrade/registry_test.go`: `runState`, `stepRegistry`,
  `mirrorInstallBinary`, `installHandoff`, `mirrorLeftovers`; new `mirrorCursorTakeoverRemedy`
  and Cursor constants.
- Modify: `internal/bashupgrade/bashupgrade_integration_test.go`: `TestIntegrationBashUpgradeGuide`
  (the user-level rule is gone) and `assertCleanEndState` (install check reports both harnesses).
- Modify: `docs/release/upgrading-from-bash.md`: sections 1, 3, 4, 7 (links paragraph plus the
  sandbox sentence) and 8.

**Interfaces:**
- Consumes (Task 1): `installLineHarnesses`, `mustContainProse`, `sameSet`, `guideHarnesses`.
- Produces: `runState.InstallHarnesses []string` and `stepRegistry["cursor-takeover-remedy"]`.
  Task 3 does not depend on these.

- [ ] **Step 1: Change the mirrors (the failing test)**

In `registry_test.go`:

1. `runState`: add after `InstallLine`:

```go
	InstallHarnesses []string // the --harness names of InstallLine, in line order
```

2. `stepRegistry`: add `"cursor-takeover-remedy": mirrorCursorTakeoverRemedy,` (keep the map
   gofmt-aligned).

3. `mirrorInstallBinary`: replace the `case line == "sh install.sh --harness claude":` arm with:

```go
		case f[0] == "sh":
			hs, ok := installLineHarnesses(line)
			if !ok || !sameSet(hs, guideHarnesses) {
				t.Fatalf("install-binary: installer line %q must be `sh install.sh` with one --harness per covered harness %v", line, guideHarnesses)
			}
			st.InstallLine, st.InstallHarnesses = line, hs
```

   and change the final `t.Fatalf` text from ``run `sh install.sh --harness claude` `` to
   `run the installer for every covered harness`.

4. `installHandoff`: build the install arguments from the line instead of hard-coding claude,
   and snapshot `~/.cursor` as well as `~/.claude`:

```go
	args := []string{"install"}
	for _, h := range st.InstallHarnesses {
		args = append(args, "--harness", h)
	}
	before := harnessHomesSnapshot(t, c)
	r := c.run(t, st.DownloadDir, stage, args...)
	…
		j := c.run(t, st.DownloadDir, stage, append([]string{"--json"}, args...)...)
	…
		if after := harnessHomesSnapshot(t, c); !mapsEqual(before, after) {
			t.Fatalf("a failed install changed ~/.claude or ~/.cursor")
		}
```

   Add the helper next to `snapshotTree`:

```go
// harnessHomesSnapshot snapshots every harness folder the guide's install writes.
func harnessHomesSnapshot(t *testing.T, c *upgradeCase) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, d := range []string{".claude", ".cursor"} {
		for k, v := range snapshotTree(t, filepath.Join(c.Home, d)) {
			out[k] = v
		}
	}
	return out
}
```

   Update the doc comment on `installHandoff`: it runs the staged binary's `install` with one
   `--harness` per harness on the guide's installer line.

5. Add the Cursor constants and the new mirror after `mirrorTakeoverRemedy`:

```go
// The Cursor paths the guide's cursor-takeover-remedy block deletes, and the guide's
// claims about them.
const (
	cursorSkillsGlob            = "~/.cursor/skills/docket-*"
	cursorPlanWriter            = "~/.cursor/agents/docket-plan-writer.md"
	cursorUserRule              = "~/.cursor/rules/docket-dispatch.mdc"
	cursorRuleSelfRemovedProse  = "On `v0.9.2` the installer removes that rule itself."
	cursorRemedySkipProse       = "If you don't use Cursor, skip this block."
)

// mirrorCursorTakeoverRemedy runs the block's Cursor deletes after proving the first
// install's Cursor conflicts are exactly the ones the guide lists: every
// ~/.cursor/skills/docket-* link, plus, on v0.9.3 only, docket-plan-writer.md and Bash's
// user-level rule. On v0.9.2 the rule is present but no conflict, which is the
// installer's side of the guide's "removes that rule itself" (the block's rm -f gets
// there first; internal/install's TestRetireCursorFileLegacyBytes proves the removal).
// The installer re-run stays in takeover-remedy, so this block must come first.
func mirrorCursorTakeoverRemedy(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	lines := blockLines(body)
	if len(lines) == 0 {
		t.Fatalf("cursor-takeover-remedy block is empty")
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "rm ") {
			t.Fatalf("cursor-takeover-remedy line %q is not a delete the test mirrors; the installer re-run belongs to takeover-remedy", l)
		}
	}
	for _, p := range []string{cursorSkillsGlob, cursorPlanWriter, cursorUserRule} {
		mustContain(t, "cursor-takeover-remedy block", body, p)
	}
	mustContainProse(t, "guide", st.Guide, cursorRuleSelfRemovedProse)
	mustContainProse(t, "guide", st.Guide, cursorRemedySkipProse)
	if len(st.Conflicts) == 0 {
		t.Fatalf("cursor-takeover-remedy: the first install reported no conflict; the guide says every %s link conflicts", cursorSkillsGlob)
	}
	links, _ := filepath.Glob(c.expandHome(cursorSkillsGlob))
	if len(links) == 0 {
		t.Fatalf("population floor: the saved %s home has no %s link", c.Tag, cursorSkillsGlob)
	}
	planWriter, rule := c.expandHome(cursorPlanWriter), c.expandHome(cursorUserRule)
	want := map[string]bool{}
	for _, l := range links {
		want[l] = true
	}
	if c.Tag == "v0.9.3" {
		want[planWriter], want[rule] = true, true
	} else if _, err := os.Lstat(rule); err != nil {
		t.Errorf("guide says the installer removes Bash's rule itself on %s, but the saved home has no %s: %v", c.Tag, rule, err)
	}
	got := map[string]bool{}
	for _, p := range st.Conflicts {
		if strings.HasPrefix(p, filepath.Join(c.Home, ".cursor")+"/") {
			got[p] = true
		}
	}
	for p := range want {
		if !got[p] {
			t.Errorf("guide says %s conflicts on %s; the installer did not report it", p, c.Tag)
		}
	}
	for p := range got {
		if !want[p] {
			t.Errorf("the installer reported %s as a conflict on %s, which the guide does not list", p, c.Tag)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	runBlock(t, c, st, body)
	for p := range want {
		if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("cursor-takeover-remedy left the conflict path %s in place", p)
		}
	}
}
```

6. `mirrorLeftovers`: replace the block from the `// Guide: nothing under ~/.claude points
   into the old checkout` comment to the end of the function with:

```go
	// Guide: nothing under ~/.claude or ~/.cursor points into the old checkout; the
	// other tools' links under ~/.codex and ~/.agents still do. The guide states that
	// using docket from those tools on an upgraded repository is not supported, rather
	// than advising to keep using Bash docket there: the test observes only the links.
	mustContain(t, "guide", st.Guide, "`~/dev/docket`")
	mustContainProse(t, "guide", st.Guide, "on an upgraded repository is not supported")
	checkout := filepath.Join(c.Home, "dev", "docket")
	for _, d := range []string{".claude", ".cursor"} {
		mustContain(t, "guide", st.Guide, "`~/"+d+"`")
		if n := linksInto(t, filepath.Join(c.Home, d), checkout); n != 0 {
			t.Errorf("guide says nothing under ~/%s points into the old checkout; %d links do", d, n)
		}
	}
	for _, d := range []string{".codex", ".agents"} {
		mustContain(t, "guide", st.Guide, "`~/"+d+"`")
		if linksInto(t, filepath.Join(c.Home, d), checkout) == 0 {
			t.Errorf("guide says links under ~/%s still point into the old checkout; none do", d)
		}
	}
	// Guide: docket must run outside Cursor's sandbox, with a link to the Cursor page.
	mustContain(t, "guide", st.Guide, cursorSandboxLink)
	if _, err := os.Stat(filepath.Join(repoRoot(t), "docs", "install", "cursor.md")); err != nil {
		t.Errorf("the guide links %s, which does not exist: %v", cursorSandboxLink, err)
	}
```

   with `cursorSandboxLink = "](../install/cursor.md)"` added to the Cursor `const` block.

In `bashupgrade_integration_test.go`:

7. `TestIntegrationBashUpgradeGuide`: right after `assertCleanEndState(t, c)`, add:

```go
			if _, err := os.Lstat(filepath.Join(c.Home, ".cursor", "rules", "docket-dispatch.mdc")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("Bash's user-level Cursor rule ~/.cursor/rules/docket-dispatch.mdc is still there after the guide (%v)", err)
			}
```

   (add `errors` and `io/fs` to the imports).

8. `assertCleanEndState`: inside the loop, after the exit-code check and before the findings
   loop, add:

```go
		if args[1] == "install" {
			var doc struct {
				Harnesses []string `json:"harnesses"`
			}
			if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil {
				t.Errorf("decode install check JSON: %v\n%s", err, r.Stdout)
			}
			have := map[string]bool{}
			for _, h := range doc.Harnesses {
				have[h] = true
			}
			for _, h := range guideHarnesses {
				if !have[h] {
					t.Errorf("docket install check reports harnesses %v; the guide installs %s", doc.Harnesses, h)
				}
			}
		}
```

   Update the doc comment: `assertCleanEndState: install check (for every harness the guide
   installs), repository check and status all clean.`

- [ ] **Step 2: Run the integration test against the unchanged guide to verify it fails**

Run: `go test -tags integration -count=1 -timeout 30m -run 'TestIntegrationBashUpgrade' ./internal/bashupgrade/`
Expected: FAIL. `TestIntegrationBashUpgradeGuideShape` reports `test registry step
"cursor-takeover-remedy" has no marked block in the guide`. `TestIntegrationBashUpgradeGuide`
fails at `install-binary: installer line "sh install.sh --harness claude" must be …`.

- [ ] **Step 3: Edit the guide**

In `docs/release/upgrading-from-bash.md`:

**Section 1.** Replace the third bullet and the paragraph after the list with:

~~~~markdown
- You use docket with Claude Code, Cursor, or both.

Repositories that keep their records on the default branch (the single-branch layout) are not
covered. The guide covers Claude Code and Cursor. OpenCode follows in a later pre-release. Codex is
not supported.
~~~~

**Section 3.** Replace the intro paragraph (the one beginning `Run these commands outside any
repository.`) with:

~~~~markdown
Run these commands outside any repository. They download the installer and the checksum file into
a new folder, check the installer against the checksum, then run it for Claude Code and Cursor. If
you use only one of them, drop the other one's `--harness` flag, here and in section 4.
~~~~

In the `install-binary` block, change only two lines:

~~~~text
VERSION=v1.0.0-alpha.2
sh install.sh --harness claude --harness cursor
~~~~

Leave the rest of section 3 byte-identical. That includes the paragraph holding
`releaseVerifiedProse`, the "first run stops with `install: invalid-state`" paragraph and the
source-checkout paragraph.

**Section 4.** Replace everything from the `## 4.` heading through the line
`This time the installer reports `install: applied`.` with:

~~~~markdown
## 4. Take over the old Claude Code and Cursor install

The installer takes over the agent files Bash docket wrote under `~/.claude/agents/` by itself, as
long as they still hold exactly what the Bash installer wrote.

It takes over nothing else. Each path it will not touch is printed as a `conflict` line that tells
you to move or delete it. For a Bash `v0.9.2` or `v0.9.3` install, that list is:

- every `docket-*` link under `~/.claude/skills/`. Each one points into your old Bash checkout.
  Deleting a link leaves the checkout alone.
- on `v0.9.3` only, `~/.claude/agents/docket-plan-writer.md`.

For Cursor, the installer takes over Bash's agent files under `~/.cursor/agents/` by itself in the
same way. It reports these Cursor paths as conflicts:

- every `docket-*` link under `~/.cursor/skills/`. These also point into your old Bash checkout.
- on `v0.9.3` only, `~/.cursor/agents/docket-plan-writer.md` and Bash's user-level rule
  `~/.cursor/rules/docket-dispatch.mdc`. On `v0.9.2` the installer removes that rule itself.

Delete the Cursor paths first. If you don't use Cursor, skip this block.

<!-- upgrade-step: cursor-takeover-remedy -->
```sh
rm ~/.cursor/skills/docket-*
rm -f ~/.cursor/agents/docket-plan-writer.md ~/.cursor/rules/docket-dispatch.mdc
```

Then delete the Claude Code paths and run the installer again from the same folder, with the same
`--harness` flags as in section 3. If you don't use Claude Code, leave out the two `rm` lines. The
failed run left no `docket` command, so run `sh install.sh` again, not `docket install`.

<!-- upgrade-step: takeover-remedy -->
```sh
rm ~/.claude/skills/docket-*
rm -f ~/.claude/agents/docket-plan-writer.md
sh install.sh --harness claude --harness cursor
```

This time the installer reports `install: applied`.
~~~~

The `global-config-cleanup` and `confirm-install` parts of section 4 stay unchanged.

**Section 7.** Replace the paragraph beginning `Your old Bash checkout, usually` with:

~~~~markdown
Your old Bash checkout, usually `~/dev/docket`, is no longer used by Claude Code or Cursor: nothing
under `~/.claude` or `~/.cursor` points into it after the upgrade. The links Bash docket made for
other tools, under `~/.codex` and `~/.agents`, still do. The guide does not cover those tools.
Using docket from OpenCode on an upgraded repository is not supported until its section arrives,
and Codex is not supported at all (see section 1).

In Cursor, docket must run outside Cursor's sandbox. [Running docket under Cursor](../install/cursor.md)
shows the permission setup that allows it.
~~~~

**Section 8.** Replace the section with:

~~~~markdown
## 8. Restart Claude Code and Cursor

Quit Claude Code and Cursor and start each one again. Both load agents and skills when they start,
so clearing a conversation is not enough.
~~~~

- [ ] **Step 4: Run the integration test to verify it passes**

Run: `go test -tags integration -count=1 -timeout 30m -run 'TestIntegrationBashUpgrade' ./internal/bashupgrade/ -v 2>&1 | tail -60`
Expected: PASS for `TestIntegrationBashUpgradeOwnership`, `TestIntegrationBashUpgradeGuideShape`
and `TestIntegrationBashUpgradeGuide/v0.9.2` and `/v0.9.3`. The logs show
`guide step cursor-takeover-remedy` before `guide step takeover-remedy`.

If it fails because the binary does something the guide cannot honestly explain, stop and return
`BLOCKED` with the output. Examples: a Cursor conflict outside the spec's traced table, a dirty
`docket install check` that is not the `runtime.bash` warning the global-config step clears, or a
`~/.cursor` link into the old checkout after the re-run. Do not edit product code. A failure that
is a guide-wording mismatch is yours to fix in the guide.

Run: `go test -count=1 ./internal/bashupgrade/`
Expected: PASS (default tag).

- [ ] **Step 5: Mutation probes (each must redden, then restore)**

```bash
mut=$(mktemp -d "${TMPDIR:-/tmp}/bashupgrade-mut.XXXXXX")
cp -f docs/release/upgrading-from-bash.md "$mut/guide.md"
```

1. **The spec's acceptance mutation.** Delete the `<!-- upgrade-step: cursor-takeover-remedy -->`
   marker and its fenced block from `$mut/guide.md`. Then run
   `DOCKET_BASH_UPGRADE_GUIDE="$mut/guide.md" go test -tags integration -count=1 -timeout 30m -run 'TestIntegrationBashUpgradeGuide$' ./internal/bashupgrade/`.
   Expected: FAIL on both cases with `install still conflicts after the guide's remedy`, naming
   `~/.cursor/skills/docket-*` paths. It must not fail with only the shape test: this run
   excludes it.
2. **Exact-set check bites.** `cp -f internal/bashupgrade/registry_test.go "$mut/registry_test.go.bak"`.
   In `mirrorCursorTakeoverRemedy`, change `want[planWriter], want[rule] = true, true` to
   `want[planWriter] = true`. Run the same command without the env var. Expected: FAIL on
   `v0.9.3` with `the installer reported …/.cursor/rules/docket-dispatch.mdc as a conflict on
   v0.9.3, which the guide does not list`. Restore with
   `cp -f "$mut/registry_test.go.bak" internal/bashupgrade/registry_test.go`.
3. **The `~/.cursor` links assertion is live.** Back up `registry_test.go` the same way. In
   `mirrorLeftovers`, change `[]string{".claude", ".cursor"}` (the zero-links loop) to
   `[]string{".claude"}`, and change the `.codex`/`.agents` loop to
   `[]string{".cursor", ".codex", ".agents"}`. Expected: FAIL `guide says links under ~/.cursor
   still point into the old checkout; none do`. This proves the run really leaves zero links
   under `~/.cursor`. Restore from the backup.

After restoring, run `git diff --stat` and confirm it shows only this task's intended edits.
Re-run Step 4's command once more: expected PASS.

- [ ] **Step 6: Commit**

```bash
git add docs/release/upgrading-from-bash.md internal/bashupgrade/registry_test.go internal/bashupgrade/bashupgrade_integration_test.go
git commit -m "docs(release): take over Bash's Cursor install in the upgrade guide, proven on the saved cases"
```

---

### Task 3: The repository half: choose both harnesses and clear repository Cursor leftovers

This covers guide section 5 (the configure-harnesses prose and a new marked final step) and
section 7 (the `repo-agent-files` block). Test first.

Why the final step becomes a marked block: the spec requires the test to see the repository rule
written by "the final `configure-harnesses` step". Today that step is only inline prose, so the test
never runs it. A fenced block that runs `docket` must be marked, or `TestIntegrationBashUpgradeGuideShape`
refuses it.

Why one existing assertion is re-keyed: `configure-harnesses --harnesses claude,…` writes docket's
**own** dispatch block into `CLAUDE.md`, under the same `<!-- docket:dispatch:start` marker Bash
used. The post-guide check "CLAUDE.md contains no `docket:dispatch:`" would then fail on a correct
upgrade. Its intent was "Bash's block is gone". It now keys on Bash's signature, the `/docket.sh `
command that `mirrorDispatchBlock` already uses to recognize Bash's block. A new positive check
confirms docket's block is there.

**Files:**
- Modify: `internal/bashupgrade/registry_test.go`: `stepRegistry`, new `runChooseHarnesses` and
  constants, `bashDispatchSignature` (used in `mirrorDispatchBlock`), `runRepoAgentFiles`.
- Modify: `internal/bashupgrade/bashupgrade_integration_test.go`: `TestIntegrationBashUpgradeGuide`
  (CLAUDE.md re-key, repository rule present).
- Modify: `docs/release/upgrading-from-bash.md`: section 5 (`configure-harnesses` prose, the final
  paragraph that becomes the `choose-harnesses` step) and section 7 (the `repo-agent-files` prose
  and block).

**Interfaces:**
- Consumes (Task 1): `mustContainProse`, `pendingPathsFrom`, `sameSet`, `guideHarnesses`. Also the
  existing `dispatchBlockSpan`, `runBlock`, `gitRev`, `c.mustGit` and `c.run`.
- Produces: `stepRegistry["choose-harnesses"]`, `const bashDispatchSignature = "/docket.sh "`.

- [ ] **Step 1: Change the test (the failing test)**

In `registry_test.go`:

1. `stepRegistry`: add `"choose-harnesses": runChooseHarnesses,`.

2. In the dispatch-block `const` group, add:

```go
	// bashDispatchSignature is what marks a dispatch block as Bash docket's: its body
	// runs docket.sh. docket's own block shares the marker lines but never this.
	bashDispatchSignature = "/docket.sh "
```

   In `mirrorDispatchBlock`, replace the literal `"/docket.sh "` with `bashDispatchSignature`.

3. Add after `mirrorDispatchBlock`:

```go
// The guide's final section-5 step: choose the agents, then commit what the command
// lists. The Cursor rule it writes is ignored, so only the listed paths are committed.
const (
	chooseHarnessesLine = "docket repository configure-harnesses --harnesses claude,cursor"
	cursorRepoRule      = ".cursor/rules/docket-dispatch.mdc"
	cursorRepoRuleProse = "For Cursor, it writes `.cursor/rules/docket-dispatch.mdc` into the repository, which the `.gitignore` block keeps out of commits."
	claudeRepoBlockProse = "For Claude Code, it writes docket's own block into `CLAUDE.md`."
)

// runChooseHarnesses runs the block and checks the guide's claims: the git add line
// names exactly the paths configure-harnesses lists, the commit reaches origin main and
// leaves nothing behind, .docket.yml there records both harnesses, CLAUDE.md there
// carries docket's dispatch block (not Bash's), and the repository Cursor rule exists
// and is ignored by git.
func runChooseHarnesses(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	lines := blockLines(body)
	if len(lines) != 4 || lines[0] != chooseHarnessesLine || !strings.HasPrefix(lines[1], "git add ") ||
		!strings.HasPrefix(lines[2], "git commit -m ") || lines[3] != "git push" {
		t.Fatalf("choose-harnesses block must be `%s`, `git add <paths>`, `git commit -m …` and `git push`:\n%s", chooseHarnessesLine, body)
	}
	mustContainProse(t, "guide", st.Guide, cursorRepoRuleProse)
	mustContainProse(t, "guide", st.Guide, claudeRepoBlockProse)
	if st.Cwd != c.Clone {
		t.Fatalf("choose-harnesses runs in the repository; the reader is in %s", st.Cwd)
	}
	if left := strings.TrimSpace(c.mustGit(t, st.Cwd, "status", "--porcelain")); left != "" {
		t.Fatalf("choose-harnesses starts from a clean repository; git status shows:\n%s", left)
	}
	before := gitRev(t, c, "main")
	r := runBlock(t, c, st, body)
	pending := pendingPathsFrom(r.Stdout + r.Stderr)
	added := strings.Fields(strings.TrimPrefix(lines[1], "git add "))
	if len(pending) == 0 || !sameSet(pending, added) {
		t.Fatalf("the guide's git add line names %v; configure-harnesses asked to commit %v\nstdout:\n%s\nstderr:\n%s", added, pending, r.Stdout, r.Stderr)
	}
	if gitRev(t, c, "main") == before {
		t.Fatalf("choose-harnesses did not push a new commit to main")
	}
	if left := strings.TrimSpace(c.mustGit(t, st.Cwd, "status", "--porcelain")); left != "" {
		t.Fatalf("choose-harnesses left changes behind:\n%s", left)
	}
	if fi, err := os.Lstat(filepath.Join(c.Clone, filepath.FromSlash(cursorRepoRule))); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("configure-harnesses did not write %s into the repository (%v)", cursorRepoRule, err)
	}
	if ig := c.run(t, c.Clone, "git", "check-ignore", "-q", "--", cursorRepoRule); ig.Code != 0 {
		t.Errorf("guide says the .gitignore block keeps %s out of commits; git does not ignore it", cursorRepoRule)
	}
	var cfg struct {
		AgentHarnesses []string `yaml:"agent_harnesses"`
	}
	if err := yaml.Unmarshal([]byte(c.mustGit(t, c.Origin, "show", "main:.docket.yml")), &cfg); err != nil {
		t.Fatalf("parse main:.docket.yml: %v", err)
	}
	if !sameSet(cfg.AgentHarnesses, guideHarnesses) {
		t.Errorf("origin main:.docket.yml records agent_harnesses %v; the guide chooses %v", cfg.AgentHarnesses, guideHarnesses)
	}
	claudeLines := strings.Split(c.mustGit(t, c.Origin, "show", "main:CLAUDE.md"), "\n")
	start, end, err := dispatchBlockSpan(claudeLines)
	switch {
	case err != nil || start < 0:
		t.Errorf("origin main:CLAUDE.md has no docket dispatch block after choose-harnesses (%v)", err)
	case strings.Contains(strings.Join(claudeLines[start:end+1], "\n"), bashDispatchSignature):
		t.Errorf("origin main:CLAUDE.md carries Bash docket's dispatch block after choose-harnesses, not docket's")
	}
}
```

4. `runRepoAgentFiles`: the block becomes three lines, and the test observes the empty Cursor
   category. Add to the per-repository `const` group:

```go
	repoCursorAgentGlob       = "rm -f .cursor/agents/docket-*.md"
	repoCursorAgentUnproven   = "so the test does not prove the `.cursor/agents` line"
```

   Change the shape check to:

```go
	if len(lines) != 3 || lines[0] != "cd "+c.Clone || lines[1] != repoAgentGlob || lines[2] != repoCursorAgentGlob {
		t.Fatalf("repo-agent-files block must be exactly `cd <repo>`, `%s` and `%s`:\n%s", repoAgentGlob, repoCursorAgentGlob, body)
	}
```

   Switch the three existing guide-prose `mustContain` calls in this function to
   `mustContainProse`, with the same strings, so the re-flowed paragraph still matches. Then add:

```go
	mustContainProse(t, "guide", st.Guide, repoCursorAgentUnproven)
	cursorGlob := filepath.Join(c.Clone, ".cursor", "agents", "docket-*.md")
	if found, _ := filepath.Glob(cursorGlob); len(found) > 0 {
		t.Errorf("the saved %s clone carries repository Cursor agent files %v; the guide says the test does not prove the .cursor/agents line, so prove it and drop that sentence", c.Tag, found)
	}
```

   After `runBlock`, extend the leftover check:

```go
	for _, g := range []string{glob, cursorGlob} {
		if left, _ := filepath.Glob(g); len(left) > 0 {
			t.Errorf("repo-agent-files left %v behind", left)
		}
	}
```

   (replacing the single-glob check).

In `bashupgrade_integration_test.go`, `TestIntegrationBashUpgradeGuide`:

5. Replace the `CLAUDE.md` check after `assertRecordsSurvive` with:

```go
			if after, _ := os.ReadFile(filepath.Join(c.Clone, "CLAUDE.md")); strings.Contains(string(after), bashDispatchSignature) {
				t.Errorf("the repository CLAUDE.md still carries Bash docket's dispatch block (it runs docket.sh) after the guide")
			}
			if fi, err := os.Lstat(filepath.Join(c.Clone, ".cursor", "rules", "docket-dispatch.mdc")); err != nil || !fi.Mode().IsRegular() {
				t.Errorf("the repository has no .cursor/rules/docket-dispatch.mdc after the guide (%v)", err)
			}
```

- [ ] **Step 2: Run the integration test against the unchanged section 5 and 7 to verify it fails**

Run: `go test -tags integration -count=1 -timeout 30m -run 'TestIntegrationBashUpgrade' ./internal/bashupgrade/`
Expected: FAIL. The shape test reports `test registry step "choose-harnesses" has no marked block
in the guide`. The guide run fails at `repo-agent-files block must be exactly …` and at
`the repository has no .cursor/rules/docket-dispatch.mdc after the guide`.

- [ ] **Step 3: Edit the guide**

**Section 5, the `configure-harnesses` prose.** Replace the paragraph beginning
`Next, choose the coding agents docket writes instructions for.` with:

~~~~markdown
Next, choose the coding agents docket writes instructions for. On a terminal,
`docket repository configure-harnesses` shows a checklist. The command below records "none yet",
because Bash docket's `CLAUDE.md` block would get in the way. The last step of this section, after
you remove that block, chooses your agents.
~~~~

The `configure-harnesses` block itself (`--harnesses none`) stays unchanged.

**Section 5, the final paragraph.** Replace the paragraph beginning
`Last, choose your coding agents, which the configure-harnesses step left as "none yet"` with:

~~~~markdown
Last, choose your coding agents, which the configure-harnesses step left as "none yet". The command
lists the paths to review and commit. For Claude Code, it writes docket's own block into `CLAUDE.md`.
For Cursor, it writes `.cursor/rules/docket-dispatch.mdc` into the repository, which the
`.gitignore` block keeps out of commits. Drop the agent you don't use from `--harnesses`, and if you
don't use Claude Code, leave `CLAUDE.md` off the `git add` line.

<!-- upgrade-step: choose-harnesses -->
```sh
docket repository configure-harnesses --harnesses claude,cursor
git add .docket.yml CLAUDE.md
git commit -m "Choose docket's agents"
git push
```
~~~~

**Section 7, `repo-agent-files`.** Replace the paragraph beginning
`Bash docket also wrote agent files into a repository` and its block with:

~~~~markdown
Bash docket also wrote agent files into a repository when its `.docket.yml` has an `agents:`
setting: one `docket-*.md` file per agent under the repository's `.claude/agents/` folder, and for
Cursor under `.cursor/agents/`. Claude Code uses an agent file in the repository instead of the one
with the same name in `~/.claude/agents/`, so these old files hide the agents the installer just
wrote. Delete them in every repository. If a repository has none, the command does nothing. The
saved installs the test runs carry no repository Cursor agent files, so the test does not prove the
`.cursor/agents` line.

<!-- upgrade-step: repo-agent-files -->
```sh
cd <repo>
rm -f .claude/agents/docket-*.md
rm -f .cursor/agents/docket-*.md
```
~~~~

The next paragraph ("The `.gitignore` block from section 5 keeps these files out of commits…")
and the `.claude/settings.local.json` paragraph stay unchanged. The `.gitignore` block already
ignores `.cursor/agents/docket-*.md`.

- [ ] **Step 4: Run the integration test to verify it passes**

Run: `go test -tags integration -count=1 -timeout 30m -run 'TestIntegrationBashUpgrade' ./internal/bashupgrade/ -v 2>&1 | tail -60`
Expected: PASS on both cases. The logs show `guide step choose-harnesses` after `guide step
dispatch-block` and before `guide step leftovers`.

If `pendingPathsFrom` returns something other than `.docket.yml` and `CLAUDE.md` (for example the
ignored Cursor rule), or `configure-harnesses` refuses, stop and return `BLOCKED` with the output.
Do not adjust the binary. Do not widen the `git add` line to `git add -A` either, because that would
commit a reader's unrelated files.

Run: `go test -count=1 ./internal/bashupgrade/`
Expected: PASS.

- [ ] **Step 5: Mutation probes (each must redden, then restore)**

```bash
mut=$(mktemp -d "${TMPDIR:-/tmp}/bashupgrade-mut.XXXXXX")
cp -f docs/release/upgrading-from-bash.md "$mut/guide.md"
```

1. **The `git add` binding.** In `$mut/guide.md`, change `git add .docket.yml CLAUDE.md` to
   `git add .docket.yml`. Run
   `DOCKET_BASH_UPGRADE_GUIDE="$mut/guide.md" go test -tags integration -count=1 -timeout 30m -run 'TestIntegrationBashUpgradeGuide$' ./internal/bashupgrade/`.
   Expected: FAIL `the guide's git add line names [.docket.yml]; configure-harnesses asked to
   commit [.docket.yml CLAUDE.md]`. Then restore the copy with
   `cp -f docs/release/upgrading-from-bash.md "$mut/guide.md"`.
2. **The repository-rule end-state check.** In the copy, delete the
   `<!-- upgrade-step: choose-harnesses -->` marker and its block. Same command. Expected: FAIL
   `the repository has no .cursor/rules/docket-dispatch.mdc after the guide`.
3. **The re-keyed CLAUDE.md check is live.** Back up `bashupgrade_integration_test.go` into
   `$mut/` with `cp -f`. Temporarily change `bashDispatchSignature` in that check to the literal
   `"docket:dispatch:"`, which is the old key. Run without the env var. Expected: FAIL, because
   docket's own block now carries that marker. That is the reason for the re-key. Restore from
   the backup.

After restoring, run `git diff --stat` and confirm it shows only this task's intended edits.
Re-run Step 4's command once more: expected PASS.

- [ ] **Step 6: Run the whole suite**

Run: `go run ./cmd/docket development test`
Expected: green. Read the budget report and note any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:` or
`SERIAL CONFIRMED OVER BUDGET:` line naming `internal/bashupgrade` for the results file. Neither
fails the run by default.

- [ ] **Step 7: Commit**

```bash
git add docs/release/upgrading-from-bash.md internal/bashupgrade/registry_test.go internal/bashupgrade/bashupgrade_integration_test.go
git commit -m "docs(release): choose Claude Code and Cursor per repository in the upgrade guide, proven on the saved cases"
```

---

## Spec coverage check

| Spec item | Task |
|---|---|
| §1 Claude Code, Cursor, or both; OpenCode later; Codex unsupported | 2 |
| §3 `VERSION=v1.0.0-alpha.2`, two-harness installer line, drop-a-flag sentence | 2 (prose), 1 (single-harness shape) |
| §4 heading, Claude text kept, takeover re-run two-harness, Cursor paragraph, `cursor-takeover-remedy` before the re-run | 2 |
| §5 final `configure-harnesses` → `claude,cursor`; repository rule ignored | 3 |
| §7 no `~/.claude`/`~/.cursor` links; `~/.codex`/`~/.agents` still; OpenCode unsupported; Codex unsupported | 2 |
| §7 `repo-agent-files` adds `.cursor/agents` line, says the test does not prove it | 3 |
| §7 sandbox sentence linking `docs/install/cursor.md` | 2 |
| §8 Restart Claude Code and Cursor | 2 |
| Test: first-install Cursor conflicts (skills on both; plan-writer + rule on v0.9.3) | 2 (`mirrorCursorTakeoverRemedy`, exact set) |
| Test: `~/.cursor/rules/docket-dispatch.mdc` absent after the guide | 2 |
| Test: install check reports `cursor`, no error/warning | 2 (`assertCleanEndState`) |
| Test: zero `~/.cursor` links into the old checkout; `~/.codex`/`~/.agents` ≥ 1 | 2 |
| Test: clone has `.cursor/rules/docket-dispatch.mdc` after the final step, with the shape check | 3 |
| Existing assertions stay (shape, tables) | 2, 3 (one re-key, justified) |
| Acceptance mutation: delete `cursor-takeover-remedy` → red | 2, Step 5 probe 1 |
| Whole suite, integration tag | 3, Step 6 |
