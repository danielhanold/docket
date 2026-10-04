package repoguard

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"testing"
)

// Ports the SITE-SCAN half of tests/test_skill_handoff_precedence.sh. The
// autonomy-precedence contract: an invoked role skill's interactive hand-off must
// never halt an autonomous run, so every AUTONOMOUS invocation of a role skill
// pre-specifies its outcome with the house marker `DIRECTED to:`. What
// demonstrably lost at the moment of invocation was exactly a standing
// instruction; only a call-site direction beats a specific sub-skill instruction
// read at that same moment.
//
// The CONVENTION-clause half ("never outranks" / "DIRECTED to:" stated in the
// convention's *Skill layer*) is ported as a prose-contract row (see
// prose_contracts_test.go). This file ports the whole-skills-tree SITE SCAN, which a
// phrase stub could not express without going vacuous — the named risk.
//
// # Sites are DERIVED from a whole-tree scan, never hand-listed
//
// The sites are every line in skills/ markdown that names one of docket's fixed
// role skills (roleSkillNames) as a whole backticked token — so a tier agent such
// as `docket-build-economy` never counts. A skill is AUTONOMOUS iff a wrapper
// exists at agents/<skill>.md — the same wrapper that carries the
// abort-and-report rule. Interactive skills have no wrapper and are skipped by
// construction, never by name.
//
// # Classes, each keyed on shape
//
//   - an INVOCATION line is one whose invocation verbs ("invoke", "invoked", ...)
//     outnumber its negations ("do NOT invoke", "never invoked", "cannot be
//     invoked", "can't be invoked"); it must carry the marker. A negation is
//     read by shape (a word ending in "not", "never", or an n't contraction),
//     not by meaning: a negation outside those shapes ("unable to invoke",
//     "fails to invoke") is still read as an invocation;
//   - a MENTION names a role skill without invoking it (or only prohibits
//     invoking it) and needs no marker;
//   - docket-finalize-change's human-present close-out is the one exception (today
//     retired: zero human-present exceptions remain, and a NEW one in any skill
//     reddens).
//
// A genuine invocation line must ALSO not frame the role invocation itself as a
// dispatch ("this long build dispatch" — the observed misfire bait); the ban
// keys on that removed framing shape, not on the word dispatch.

// roleSkillNames are docket's fixed role skills (convention, *Skill layer*).
var roleSkillNames = []string{
	"superpowers:brainstorming",
	"superpowers:writing-plans",
	"docket-build",
	"docket-review",
	"superpowers:finishing-a-development-branch",
}

// roleSkillRe matches a role-skill name as a whole backticked token, so a
// tier agent such as `docket-build-economy` or `docket-review-lean` never
// matches.
var roleSkillRe = func() *regexp.Regexp {
	q := make([]string, len(roleSkillNames))
	for i, n := range roleSkillNames {
		q[i] = regexp.QuoteMeta(n)
	}
	return regexp.MustCompile("`(?:" + strings.Join(q, "|") + ")`")
}()

var (
	// invokeRe is the invocation verb the house idiom puts on every genuine
	// role invocation ("is invoked **DIRECTED to:**", "invokes").
	invokeRe = regexp.MustCompile(`(?i)\binvok(?:e|ed|es|ing)\b`)
	// negatedInvokeRe is a prohibition or inability ("do NOT invoke", "never
	// invoked", "cannot be invoked", "can't be invoked"), which pre-specifies
	// nothing and needs no marker. It keys on the negation's shape within two
	// words of the verb: a word ending in "not" (not, cannot), "never" as a
	// whole word (so "whenever" is not one), or a word ending in the n't
	// contraction with a straight or typographic apostrophe.
	negatedInvokeRe = regexp.MustCompile(`(?i)(?:\b\w*not|\bnever|\w+n['’]t)\W+(?:\w+\W+){0,2}invok(?:e|ed|es|ing)\b`)
	skillFramedRe   = regexp.MustCompile(`(?i)long [a-z-]+ dispatch`)
)

const skillMarker = "DIRECTED to:"

// wrapperNames returns the set of skills that have a generated agent wrapper
// (agents/<name>.md), i.e. the autonomous skills.
func wrapperNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	re := regexp.MustCompile(`^agents/([^/]+)\.md$`)
	names := map[string]bool{}
	for _, rel := range maintainedPop(t, root) {
		if m := re.FindStringSubmatch(rel); m != nil {
			names[m[1]] = true
		}
	}
	return names
}

// handoffSite is one discovered role-skill line.
type handoffSite struct {
	rel   string
	line  int
	text  string
	skill string // basename(dirname(rel)) — the owning skill dir
}

// discoverHandoffSites scans skills/ markdown for every line naming a role skill.
func discoverHandoffSites(t *testing.T, root string) []handoffSite {
	t.Helper()
	var sites []handoffSite
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !hasExt(rel, ".md") {
			continue
		}
		content := readMaintained(t, root, rel)
		for i, ln := range strings.Split(content, "\n") {
			if roleSkillRe.MatchString(ln) {
				sites = append(sites, handoffSite{
					rel:   rel,
					line:  i + 1,
					text:  ln,
					skill: path.Base(path.Dir(rel)),
				})
			}
		}
	}
	return sites
}

// classifyHandoffSite reports the shape of one wrapper-backed role-skill line.
type handoffClass int

const (
	handoffException  handoffClass = iota // human-present close-out
	handoffMention                        // names a role skill without invoking it
	handoffInvocation                     // a role skill is invoked here
)

func classifyHandoffSite(text string) handoffClass {
	if strings.Contains(strings.ToLower(text), "human is present") {
		return handoffException
	}
	if len(invokeRe.FindAllStringIndex(text, -1)) > len(negatedInvokeRe.FindAllStringIndex(text, -1)) {
		return handoffInvocation
	}
	return handoffMention
}

func TestSkillHandoffSites(t *testing.T) {
	root := guardRoot(t)
	wrappers := wrapperNames(t, root)
	if len(wrappers) < 10 {
		t.Fatalf("population floor: only %d agent wrappers discovered (expected >= 10)", len(wrappers))
	}
	sites := discoverHandoffSites(t, root)
	if len(sites) == 0 {
		t.Fatalf("role-skill invocation sites were not discovered (scan reached nothing)")
	}

	var checked, exceptions, mentions, invocations int
	var violations []string
	for _, s := range sites {
		if !wrappers[s.skill] {
			continue // interactive skill, no wrapper — skipped by construction
		}
		checked++
		switch classifyHandoffSite(s.text) {
		case handoffException:
			exceptions++
			// The one exception belongs to docket-finalize-change alone.
			if s.skill != "docket-finalize-change" {
				violations = append(violations, fmt.Sprintf("%s:%d human-present exception belongs to docket-finalize-change, not %s", s.rel, s.line, s.skill))
			}
		case handoffMention:
			mentions++
		case handoffInvocation:
			invocations++
			if !strings.Contains(s.text, skillMarker) {
				violations = append(violations, fmt.Sprintf("%s:%d autonomous role invocation does not pre-specify its outcome (missing %q)", s.rel, s.line, skillMarker))
			}
			if skillFramedRe.MatchString(s.text) {
				violations = append(violations, fmt.Sprintf("%s:%d role invocation is framed as a dispatch", s.rel, s.line))
			}
		}
	}

	// Floors: the classifier must not go vacuous. `checked` counts every
	// wrapper-backed role-skill line; the marker-bearing invocation population is floored
	// separately so the mention branch cannot silently swallow every invocation.
	if checked < 4 {
		t.Fatalf("population floor: only %d wrapper-backed role-skill lines checked (expected >= 4)", checked)
	}
	if invocations < 3 {
		t.Fatalf("population floor: only %d marker-checked invocation lines (expected >= 3)", invocations)
	}
	// Zero human-present exceptions is now correct (the finalize finish-role
	// exception was retired). A NEW one in any skill still trips the belongs-to
	// check above.
	if exceptions != 0 {
		t.Errorf("expected 0 human-present exceptions, found %d", exceptions)
	}
	if len(violations) != 0 {
		t.Errorf("autonomy-precedence site violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		const unmarked = "The build role `docket-build` is invoked to execute the plan."
		const marked = "`docket-build` is invoked **DIRECTED to:** execute the plan task-by-task and stop."
		const mention = "For the `docket-review` role, select the review tier from the build record."
		const prohibited = "do NOT invoke `superpowers:brainstorming` with a simulated human answerer"
		const tier = "Dispatch `docket-build-economy` by name, foreground."
		const framed = "Refresh the claim before this long build dispatch — `docket-build` is invoked **DIRECTED to:** execute the plan."
		if classifyHandoffSite(unmarked) != handoffInvocation || strings.Contains(unmarked, skillMarker) {
			t.Errorf("an unmarked invocation was not classified as a violating invocation")
		}
		if classifyHandoffSite(marked) != handoffInvocation || !strings.Contains(marked, skillMarker) {
			t.Errorf("a marked invocation was not classified as an invocation carrying the marker")
		}
		if classifyHandoffSite(mention) != handoffMention {
			t.Errorf("a role mention was classified as an invocation")
		}
		if classifyHandoffSite(prohibited) != handoffMention {
			t.Errorf("a prohibition was classified as an invocation")
		}
		// Negation is read by shape: a word ending in "not" (not, cannot), the
		// n't contraction (straight or typographic apostrophe), or "never" as a
		// whole word.
		for _, neg := range []string{
			"When `docket-review` cannot be invoked, review the branch inline.",
			"When `docket-review` can't be invoked, review the branch inline.",
			"When `docket-review` can’t be invoked, review the branch inline.",
		} {
			if classifyHandoffSite(neg) != handoffMention {
				t.Errorf("a negated invocation was classified as an invocation: %q", neg)
			}
		}
		// A line that invokes AND mentions a negated invocation still invokes
		// (2 verbs vs 1 negation), so an unmarked one must still be caught.
		const mixed = "Invoke `docket-build`; when `docket-build` cannot be invoked, run the plan inline."
		if classifyHandoffSite(mixed) != handoffInvocation || strings.Contains(mixed, skillMarker) {
			t.Errorf("an unmarked invocation beside a negated clause was not classified as a violating invocation")
		}
		// Shape boundaries: "whenever" is not "never", and a word that merely
		// contains "not" mid-word is not a negation.
		for _, inv := range []string{
			"`docket-build` is invoked whenever the plan is ready.",
			"`docket-build` runs whenever invoked by the controller.",
			"Annotate the plan, then `docket-build` is invoked to execute it.",
		} {
			if classifyHandoffSite(inv) != handoffInvocation {
				t.Errorf("an invocation was misread as a negation: %q", inv)
			}
		}
		if !roleSkillRe.MatchString(marked) || roleSkillRe.MatchString(tier) {
			t.Errorf("role-skill discovery must match a backticked role skill and never a tier agent name")
		}
		if !skillFramedRe.MatchString(framed) {
			t.Errorf("framing ban missed the framed-as-dispatch shape")
		}
		if skillFramedRe.MatchString("Dispatch the selected tier wrapper by name, foreground") {
			t.Errorf("framing ban wrongly caught genuine nested-dispatch prose")
		}
	})
}
