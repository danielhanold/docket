package repoguard

import (
	"regexp"
	"strings"
	"testing"
)

// Ports tests/test_inline_role_stop_scoping.sh (change 0212). A docket-owned skill
// body that can be LOADED INTO A CALLER'S CONTEXT must scope its terminal stops and
// its second-person prohibitions to the role, because an unqualified "you" in an
// inlined body resolves to the CALLER. (On 2026-08-05 a docket-implement-next run
// read docket-build's "Then you stop — review is not yours." as its own terminal
// boundary and ended with no review and no PR.)
//
// # Positive-presence, proximity-scoped, wrap-tolerant
//
// This is deliberately NOT a negative vocabulary grep (a grep forbidding an
// unqualified "you stop" is line- and vocabulary-scoped and escapes by paraphrase).
// It is a POSITIVE-presence guard: each swept stop is a (file, anchor) SITE, and
// the two-sided scoping clause must appear within WINDOW lines AFTER the anchor —
// presence anywhere in the file is not presence AT the stop (change 0199's
// co-occurrence lesson). The window is whitespace-normalized before matching, so a
// clause whose halves straddle a hard wrap in the swept markdown still matches.
//
// # Both halves are load-bearing
//
// The discriminator is HOW THIS BODY ARRIVED, not the reader's employment status: a
// docket-implement-next fork reading docket-build inline is BOTH a dispatched
// subagent AND an inline caller. So the second person sits on the CONTINUE branch
// (inlineHalf) and the abort branch is third-person about "an agent whose entire
// assignment is this role" (dispatchHalf). A wrapper injects the same body, so both
// halves stay required; a one-sided clause must NOT satisfy the matcher.
//
// # The SITES table is hand-maintained
//
// Anchors are verbatim-quoted clauses, never line numbers (AGENTS.md / ADR-0054),
// so drift is mechanically visible: if a swept body rewords a stop, its anchor stops
// matching and the existence floor reddens — deliberately, so the table is updated
// rather than guarding nothing.

const (
	inlineHalf   = "you invoked this skill yourself"
	dispatchHalf = "only an agent whose entire assignment is this role"
	stopWindow   = 6
)

// stopSite is one swept stop: a (file, anchor, what) row. The anchor is the
// verbatim, backtick-free, mid-sentence clause on the anchor line; matching is
// case-sensitive and fixed-string.
type stopSite struct {
	file   string
	anchor string
	what   string
}

// stopSites is the hand-maintained table. A PROHIBITION site anchors on the LAST
// line of its block (the clause is appended after the block); a STOP anchors on its
// stop sentence. docket-build's anchor is the wrapped prefix `Then you stop —
// review` because the landed compression broke the full sentence across two lines.
var stopSites = []stopSite{
	{"skills/docket-build/SKILL.md", "Then you stop — review", "terminal stop"},
	{"skills/docket-build/SKILL.md", "Every halt is the same disposition", "halting stop"},
	{"skills/docket-review/SKILL.md", "One shot at the dispatched rung", "second-person prohibitions"},
	{"skills/docket-review/SKILL.md", "An unmet precondition or a blocking ambiguity is **abort-and-report**", "terminal stop"},
	{"skills/docket-status/SKILL.md", "stop rather than improvising a fix", "hard-error stop"},
	{"skills/docket-build-task/SKILL.md", "revise or replace them, but never discard them blindly", "second-person prohibitions"},
	{"skills/docket-build-task/SKILL.md", "Return exactly one of four outcomes", "terminal return"},
	{"skills/docket-brainstorm/SKILL.md", "STOP AT THE SPEC", "terminal stop (always-inlined body)"},
}

var wsRun = regexp.MustCompile(`[[:space:]]+`)

// anchorLine returns the 1-based line number of the first line containing anchor
// (fixed substring), or 0 if none. 0 means "absent", never an error and never a
// spurious number.
func anchorLine(content, anchor string) int {
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, anchor) {
			return i + 1
		}
	}
	return 0
}

// windowText returns the WINDOW+1 lines at and after the 1-based anchor line,
// joined and whitespace-normalized (runs collapsed to single spaces), so a literal
// that straddles a hard wrap still matches.
func windowText(content string, line, window int) string {
	lines := strings.Split(content, "\n")
	lo := line - 1 // 0-based
	hi := lo + window
	if hi >= len(lines) {
		hi = len(lines) - 1
	}
	if lo < 0 || lo >= len(lines) {
		return ""
	}
	joined := strings.Join(lines[lo:hi+1], "\n")
	return strings.TrimSpace(wsRun.ReplaceAllString(joined, " "))
}

// clauseNear reports whether the window starting at the anchor line carries BOTH
// halves of the two-sided scoping clause.
func clauseNear(content string, line int) bool {
	text := windowText(content, line, stopWindow)
	return strings.Contains(text, inlineHalf) && strings.Contains(text, dispatchHalf)
}

func TestInlineRoleStopScoping(t *testing.T) {
	root := guardRoot(t)

	// Population floor: the table itself must not collapse.
	if len(stopSites) < 8 {
		t.Fatalf("population floor: only %d swept stop sites (expected >= 8)", len(stopSites))
	}

	for _, s := range stopSites {
		content := readMaintained(t, root, s.file) // fail-closed: a moved body is a read error
		if strings.TrimSpace(content) == "" {
			t.Errorf("%s: swept body is empty", s.file)
			continue
		}
		// Non-vacuity / population floor: the site anchor still matches. A reworded
		// stop reddens here instead of silently selecting an empty scope.
		ln := anchorLine(content, s.anchor)
		if ln == 0 {
			t.Errorf("%s no longer carries its %s anchor %q", s.file, s.what, s.anchor)
			continue
		}
		// The property: the two-sided clause sits AT the site, not merely somewhere
		// in the file.
		if !clauseNear(content, ln) {
			t.Errorf("%s does not scope its %s within %d lines of anchor %q\nwindow: %q",
				s.file, s.what, stopWindow, s.anchor, windowText(content, ln, stopWindow))
		}
	}

	t.Run("build_task_preload_disambiguation", func(t *testing.T) {
		// docket-build-task reaches its worker by WRAPPER PRELOAD, so for this one
		// body the inline half is readable by the worker itself — which would exempt
		// it from the metadata/never-push prohibitions the clause is the only
		// enforcement of. A positive disambiguation must say preload is not
		// self-invocation. Asserted per-site (the 0199 co-occurrence lesson).
		const preload = "Wrapper preload is not self-invocation"
		content := readMaintained(t, root, "skills/docket-build-task/SKILL.md")
		for _, anc := range []string{
			"revise or replace them, but never discard them blindly",
			"Return exactly one of four outcomes",
		} {
			ln := anchorLine(content, anc)
			if ln == 0 {
				t.Errorf("docket-build-task no longer carries anchor %q", anc)
				continue
			}
			if !strings.Contains(windowText(content, ln, stopWindow), preload) {
				t.Errorf("docket-build-task does not disambiguate preload at %q", anc)
			}
		}
	})

	t.Run("brainstorm_stop_names_planning_owner", func(t *testing.T) {
		// docket-brainstorm is the only swept body with no `context: fork`
		// frontmatter, so it is ALWAYS loaded into its caller's context. Its stop
		// must name planning's owner AT the stop (a different property from the
		// two-sided clause the SITES row checks — the artifact/stop-point boundary).
		const owner = "owned by `docket-implement-next`"
		content := readMaintained(t, root, "skills/docket-brainstorm/SKILL.md")
		ln := anchorLine(content, "STOP AT THE SPEC")
		if ln == 0 {
			t.Fatalf("docket-brainstorm no longer carries its STOP AT THE SPEC anchor")
		}
		if !strings.Contains(windowText(content, ln, stopWindow), owner) {
			t.Errorf("docket-brainstorm's stop no longer names planning's owner AT the stop")
		}
	})

	t.Run("adr_no_hazard_verdict", func(t *testing.T) {
		// docket-adr's swept verdict is still NO-HAZARD: its imperative body carries
		// no reader-directed stop and no second-person prohibition, so a future edit
		// that introduces one without scoping it reddens here. Keyed on SYNTACTIC
		// SHAPE, not on another skill's spellings (AGENTS.md).
		content := readMaintained(t, root, "skills/docket-adr/SKILL.md")
		if !strings.Contains(content, "docket-adr") {
			t.Fatalf("docket-adr no longer names itself (non-vacuity)")
		}
		sentences := adrSentences(content)
		// Non-vacuity: the stream is live and still carries the body's own text
		// (backtick-stripped), or the shape scans below pass on nothing.
		live := false
		for _, s := range sentences {
			if strings.Contains(s, "docket-adr maintains the project-wide") {
				live = true
				break
			}
		}
		if !live {
			t.Fatalf("docket-adr sentence stream is not live (non-vacuity)")
		}
		var stops, prohibitions []string
		for _, s := range sentences {
			if adrStopRe.MatchString(s) {
				stops = append(stops, strings.TrimSpace(s))
			}
			if adr2pRe.MatchString(s) && adrNegRe.MatchString(s) {
				prohibitions = append(prohibitions, strings.TrimSpace(s))
			}
		}
		if len(stops) != 0 {
			t.Errorf("docket-adr carries %d reader-directed stop(s):\n%s", len(stops), strings.Join(stops, "\n"))
		}
		if len(prohibitions) != 0 {
			t.Errorf("docket-adr carries %d second-person prohibition(s):\n%s", len(prohibitions), strings.Join(prohibitions, "\n"))
		}
	})

	t.Run("non_vacuity", func(t *testing.T) {
		// The matcher REJECTS an unscoped stop.
		probe := "Then you stop — review is not yours.\nSome other paragraph entirely."
		if clauseNear(probe, anchorLine(probe, "Then you stop — review is not yours.")) {
			t.Errorf("matcher accepted an unscoped stop")
		}
		// An absent anchor yields 0, not a spurious line number.
		if got := anchorLine(probe, "no such anchor anywhere in this probe"); got != 0 {
			t.Errorf("anchorLine returned %d for an absent anchor (expected 0)", got)
		}
		// It ACCEPTS a properly scoped stop.
		scoped := "Then you stop — review is not yours.\n\n**Scope of this stop:** If " + inlineHalf +
			", this stop ends only this role — you continue to your own next step; " + dispatchHalf + " ends its turn here."
		if !clauseNear(scoped, anchorLine(scoped, "Then you stop — review is not yours.")) {
			t.Errorf("matcher rejected a scoped stop")
		}
		// ...including one whose halves straddle a hard wrap.
		wrapped := "Then you stop — review is not yours.\n\n**Scope of this stop:** If you invoked this skill\nyourself, this stop ends only this role; only an agent whose entire\nassignment is this role ends its turn here."
		if !clauseNear(wrapped, anchorLine(wrapped, "Then you stop — review is not yours.")) {
			t.Errorf("matcher rejected a clause wrapped across lines")
		}
		// A ONE-SIDED clause must NOT satisfy it.
		oneSided := "Then you stop — review is not yours.\n\n**Scope of this stop:** If " + inlineHalf + ", you continue to your own next step."
		if clauseNear(oneSided, anchorLine(oneSided, "Then you stop — review is not yours.")) {
			t.Errorf("matcher accepted a one-sided clause")
		}
		// Presence far from the site (outside the window) must NOT satisfy it.
		far := "Then you stop — review is not yours.\nf1\nf2\nf3\nf4\nf5\nf6\nf7\nf8\n" + inlineHalf + " and " + dispatchHalf
		if clauseNear(far, anchorLine(far, "Then you stop — review is not yours.")) {
			t.Errorf("matcher accepted a clause outside the site window")
		}
		// The adr shape regexes fire on their target shapes and ignore inflected
		// third-person / artifact-directed prose.
		if !adrStopRe.MatchString("Then stop and report.") || !adrStopRe.MatchString("you stop here") {
			t.Errorf("adrStopRe missed a bare reader-directed stop")
		}
		if adrStopRe.MatchString("it simply stops being added to the ledger") {
			t.Errorf("adrStopRe wrongly flagged an inflected third-person verb")
		}
		if !(adr2pRe.MatchString("you must not push") && adrNegRe.MatchString("you must not push")) {
			t.Errorf("adr shape regexes missed a second-person prohibition")
		}
		if adr2pRe.MatchString("Never edit an Accepted ADR's body") {
			t.Errorf("adr2pRe wrongly flagged an artifact-directed imperative with no second-person pronoun")
		}
	})
}

// --- docket-adr no-hazard shape scan ---------------------------------------
//
// Shape 1 — TERMINAL STOP: the BARE (imperative / second-person) form of the
// stop-verb family. Bare form is the discriminator: "Then stop", "you stop" all
// address the reader; the inflected third-person "it stops being added to" is
// correctly ignored by the word boundary.
var adrStopRe = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])(stop|halt|abort|cease|quit)([^[:alnum:]_]|$)`)

// Shape 2 — SECOND-PERSON PROHIBITION: a sentence carrying BOTH a second-person
// pronoun AND a negative-deontic marker. Both are closed grammatical classes, so a
// newly-added "you must not push" is caught by construction while docket-adr's real
// artifact imperatives ("Never edit an Accepted ADR's body") stay out of scope.
var (
	adr2pRe  = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])(you|your|yours|yourself)([^[:alnum:]_]|$)`)
	adrNegRe = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])(never|cannot|dont|do not|does not|must not|may not|shall not|should not|can not)([^[:alnum:]_]|$)`)
)

// adrSentences turns a body into its sentence stream: markdown emphasis/code
// markers stripped (so `do **not**` reads as `do not`), then split on sentence
// terminators, with a blank line also terminating a sentence so a heading or bullet
// cannot bleed into its neighbour. The prohibition and the pronoun must co-occur in
// ONE SENTENCE, not merely one file (change 0199's co-occurrence lesson).
func adrSentences(content string) []string {
	strip := strings.NewReplacer("`", "", "*", "", "_", "")
	terminators := func(r rune) bool { return r == '.' || r == ';' || r == '!' || r == '?' }
	var out []string
	var buf string
	flush := func(s string) {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	for _, raw := range strings.Split(content, "\n") {
		line := strip.Replace(raw)
		if strings.TrimSpace(line) == "" {
			flush(buf)
			buf = ""
			continue
		}
		buf += " " + line
		for {
			idx := strings.IndexFunc(buf, terminators)
			if idx < 0 {
				break
			}
			flush(buf[:idx+1])
			buf = buf[idx+1:]
		}
	}
	flush(buf)
	return out
}
