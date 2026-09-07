package repoguard

// Change 0346 — finalize's repository-required post-merge binary rebuild.
//
// These guards pin the maintained instruction contract (skill prose +
// AGENTS.md policy), not universal obedience by every agent harness:
//   1. end-of-run ordering — the rebuild step follows step 11's integration
//      sync, structurally (heading order) and in prose;
//   2. the success-disposition gate — only sync disposition `advanced` or
//      `already-current` authorizes a rebuild, and the failure vocabulary
//      does not;
//   3. the source ancestry proof — every retained merge commit is proven an
//      ancestor via `git merge-base --is-ancestor`, with a negative answer
//      and a failed probe distinguished;
//   4. the post-install exact identity check — the installed binary's full,
//      clean commit identity must equal the verified source commit S (no
//      prefix/timestamp/label proof);
//   5. the separate incomplete-report — failures keep merged changes done
//      and are reported as "binary rebuild incomplete", never by reversing
//      a merge.
//
// Guard shape: each claim is bound to its subject with a single bounded gap
// over whitespace-collapsed section slices (learnings:
// prose-guard-binds-phrase-to-claim, phrase-grep-over-wrapped-prose). Every
// slice has a named start anchor and a named terminator, both asserted to
// exist (learnings: section-slice-needs-a-named-terminator,
// marker-scoped-guard-needs-a-population-floor). Ordering is asserted by
// index comparison, not adjacency. Embedded copies are NOT read here —
// internal/assets' drift guard owns authored-vs-embedded equality.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// collapseWS joins all whitespace runs (including newlines) into single
// spaces so a claim split across hard-wrapped lines stays matchable.
func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

// readGuarded reads one maintained file by path, fail-closed: a moved or
// renamed file is a test failure, never a silent skip.
func readGuarded(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("guarded file unreadable (moved/renamed files must repoint this guard): %v", err)
	}
	return string(raw)
}

// sliceSection cuts [start, terminator) out of content, asserting both
// anchors exist and appear in order — an absent anchor fails loudly instead
// of silently matching nothing.
func sliceSection(t *testing.T, rel, content, start, terminator string) string {
	t.Helper()
	i := strings.Index(content, start)
	if i < 0 {
		t.Fatalf("%s: section start anchor missing: %q", rel, start)
	}
	rest := content[i:]
	j := strings.Index(rest, terminator)
	if j < 0 {
		t.Fatalf("%s: section terminator missing after start: %q", rel, terminator)
	}
	return rest[:j]
}

// binding is one claim bound to its subject: a named regexp that must match
// the collapsed section slice.
type binding struct {
	name    string
	pattern string
}

func assertBindings(t *testing.T, rel string, collapsed string, bindings []binding) {
	t.Helper()
	// Population floor: an empty table would pass vacuously.
	if len(bindings) < 3 {
		t.Fatalf("%s: binding table collapsed to %d rows", rel, len(bindings))
	}
	for _, b := range bindings {
		re := regexp.MustCompile(b.pattern)
		if !re.MatchString(collapsed) {
			t.Errorf("%s: contract binding %q not found (pattern %q) — the claim was removed or severed from its subject", rel, b.name, b.pattern)
		}
	}
}

const (
	finalizeSkillRel   = "skills/docket-finalize-change/SKILL.md"
	agentsRel          = "AGENTS.md"
	syncStepAnchor     = "### 11. Integration sync"
	rebuildStepAnchor  = "### 12. Repository-required post-merge rebuild"
	rebuildTerminator  = "## Identity repair checkpoint"
	agentsRebuildStart = "## Rebuild the binary after a merge to main"
	agentsTerminator   = "<!-- docket:dispatch:start"
)

// TestFinalizeRebuildContract pins the maintained finalize skill's step-12
// rebuild contract (change 0346).
func TestFinalizeRebuildContract(t *testing.T) {
	root := guardRoot(t)
	content := readGuarded(t, root, finalizeSkillRel)

	// (1) End-of-run ordering, structural: step 11's sync heading precedes
	// the rebuild heading. Both anchors are asserted to exist first, so a
	// renamed heading fails loudly rather than passing an index race.
	si := strings.Index(content, syncStepAnchor)
	ri := strings.Index(content, rebuildStepAnchor)
	if si < 0 {
		t.Fatalf("%s: integration-sync step heading missing: %q", finalizeSkillRel, syncStepAnchor)
	}
	if ri < 0 {
		t.Fatalf("%s: rebuild step heading missing: %q", finalizeSkillRel, rebuildStepAnchor)
	}
	if si > ri {
		t.Errorf("%s: rebuild step precedes the integration sync step — the rebuild must be the post-sync suffix", finalizeSkillRel)
	}

	sec := collapseWS(sliceSection(t, finalizeSkillRel, content, rebuildStepAnchor, rebuildTerminator))

	assertBindings(t, finalizeSkillRel, sec, []binding{
		// (1) ordering, in prose: bound to step 11 and to the never-per-change rule.
		{"rebuild runs after step 11's sync", "runs \\*\\*after\\*\\* step 11's integration sync"},
		{"never per-change inside merge/closeout", "never per-change inside steps 8"},
		// (2) success-disposition gate, positive and negative.
		{"gate on advanced/already-current", "authorized only when[^.]{0,160}`advanced` or `already-current`"},
		{"failure vocabulary does not authorize", "`skipped`, `refused`, or `failed` sync[^.]{0,160}leaves the rebuild not performed"},
		{"clean exit is not authorization", "clean process exit[^.]{0,80}not authorization"},
		// (3) ancestry proof, bound to its universal quantifier and its two-outcome rule.
		{"every retained merge commit proven", "every retained merge commit[^.]{0,200}merge-base --is-ancestor"},
		{"negative answer vs failed probe distinct", "negative answer and a failed probe[^.]{0,120}neither permits"},
		// merge identity comes from the merge verification, never a feature head.
		{"merge ids from step 8 verification", "merge-commit ids from step 8"},
		{"feature head is never a substitute", "original head is never a substitute"},
		// (4) post-install exact identity, equality not prefix.
		{"identity equals S exactly", "full, clean commit identity to equal `S` exactly"},
		{"prefix/timestamp/label prove nothing", "short-prefix comparison[^.]{0,120}proves nothing"},
		{"unknown/dirty/mismatch is failure", "unknown, dirty, mismatching, or unreadable identity is verification failure"},
		// (5) separate incomplete-report, bound to done-retention and no-reversal.
		{"failures keep merged changes done", "keeps every verified merged change `done`[^.]{0,160}binary rebuild incomplete"},
		{"never reverses a merge", "never reverses a merge"},
		{"no destructive source recovery", "Never stash, switch branches, reset"},
		// generic skill: no hard-coded Docket source path.
	})

	// The distributed skill body must not hard-code this repo's source path
	// (learnings: distributed-body-has-no-local-repo).
	if strings.Contains(sec, "/Users/homer/dev/docket") {
		t.Errorf("%s: the shipped skill hard-codes a repo-local source path", finalizeSkillRel)
	}
}

// TestFinalizeRebuildAgentsRule pins the concrete Docket policy in AGENTS.md:
// sync-gated, ancestry-proven, identity-verified rebuild, reported as
// incomplete on failure. CLAUDE.md is a symlink of AGENTS.md; reading
// AGENTS.md covers both.
func TestFinalizeRebuildAgentsRule(t *testing.T) {
	root := guardRoot(t)
	content := readGuarded(t, root, agentsRel)
	raw := sliceSection(t, agentsRel, content, agentsRebuildStart, agentsTerminator)
	sec := collapseWS(raw)

	// Ordering inside the rule: the sync operation is named before the
	// install operation.
	si := strings.Index(sec, "repository.sync-integration")
	ii := strings.Index(sec, "development.install")
	if si < 0 || ii < 0 {
		t.Fatalf("%s: rebuild rule must name both repository.sync-integration (%d) and development.install (%d)", agentsRel, si, ii)
	}
	if si > ii {
		t.Errorf("%s: the rebuild rule names install before sync — sync must gate the install", agentsRel)
	}

	assertBindings(t, agentsRel, sec, []binding{
		{"gate on advanced/already-current", "only on disposition `advanced` or `already-current`"},
		{"ancestry proof named", "merge-base --is-ancestor"},
		{"installed identity equality", "same full, clean commit id"},
		{"incomplete report named", "binary rebuild incomplete"},
		{"no destructive source recovery", "never stash, reset, or switch branches"},
		// The pre-existing schema-tolerance bullet survives the rewrite.
		{"schema-extension bullet retained", "extends the `\\.docket\\.yml` schema"},
	})
}
