package repoguard

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"testing"
)

// Ports the SITE-SCAN half of tests/test_skill_handoff_precedence.sh (change
// 0096/0355). The autonomy-precedence contract: an invoked role skill's interactive
// hand-off must never halt an autonomous run, so every AUTONOMOUS invocation of a
// resolved role skill pre-specifies its outcome with the house marker `DIRECTED
// to:`. What demonstrably lost at the moment of invocation (run 40) was exactly a
// standing instruction; only a call-site direction beats a specific sub-skill
// instruction read at that same moment.
//
// The CONVENTION-clause half ("never outranks" / "DIRECTED to:" stated in the
// convention's *Skill layer*) is ported as a prose-contract row (see
// prose_contracts_test.go). This file ports the whole-skills-tree SITE SCAN, which a
// phrase stub could not express without going vacuous — the named risk.
//
// # Sites are DERIVED from a whole-tree scan, never hand-listed
//
// AGENTS.md's enumerated floor: the sites are every `$SKILL_X` / `${SKILL_X}` sigil
// line across skills/ (both spellings — keying on the bare sigil alone would let a
// braced rewrite slip past discovery). A skill is AUTONOMOUS iff a wrapper exists at
// agents/<skill>.md — the same wrapper that carries the abort-and-report rule.
// Interactive skills have no wrapper and are skipped by construction, never by name.
//
// # Two classes are exempt from the marker, each keyed on shape
//
//   - a role-variable MENTION names the sigil as resolved config WITHOUT invoking a
//     role skill (the discriminator is the lowercase invocation noun "skill", which
//     the house idiom puts on every genuine invocation line and which is absent from
//     a bare-sigil enumeration — case-sensitive, so the SKILL inside the sigil does
//     not count);
//   - docket-finalize-change's human-present close-out is the one exception (today
//     retired: zero human-present exceptions remain, and a NEW one in any skill
//     reddens).
//
// A genuine invocation line must ALSO not frame the role invocation itself as a
// dispatch ("this long build dispatch" — the observed 0351 misfire bait); the ban
// keys on that removed framing shape, not on the word dispatch.

var (
	skillSigilRe  = regexp.MustCompile(`\$\{?SKILL_[A-Z]{4,}`)
	skillFramedRe = regexp.MustCompile(`(?i)long [a-z-]+ dispatch`)
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

// handoffSite is one discovered sigil line.
type handoffSite struct {
	rel   string
	line  int
	text  string
	skill string // basename(dirname(rel)) — the owning skill dir
}

// discoverHandoffSites scans skills/ markdown for every sigil line.
func discoverHandoffSites(t *testing.T, root string) []handoffSite {
	t.Helper()
	var sites []handoffSite
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !hasExt(rel, ".md") {
			continue
		}
		content := readMaintained(t, root, rel)
		for i, ln := range strings.Split(content, "\n") {
			if skillSigilRe.MatchString(ln) {
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

// classifyHandoffSite reports the shape of one wrapper-backed sigil line.
type handoffClass int

const (
	handoffException  handoffClass = iota // human-present close-out
	handoffMention                        // bare-sigil resolution, no invocation noun
	handoffInvocation                     // a role skill is invoked here
)

func classifyHandoffSite(text string) handoffClass {
	if strings.Contains(strings.ToLower(text), "human is present") {
		return handoffException
	}
	// Case-SENSITIVE lowercase invocation noun: a case-insensitive test would match
	// the SKILL inside the `$SKILL_X` sigil and classify every line as an
	// invocation.
	if !strings.Contains(text, "skill") {
		return handoffMention
	}
	return handoffInvocation
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
	// wrapper-backed sigil line; the marker-bearing invocation population is floored
	// separately so the mention branch cannot silently swallow every invocation.
	if checked < 4 {
		t.Fatalf("population floor: only %d wrapper-backed sigil lines checked (expected >= 4)", checked)
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
		const unmarked = "Run the **resolved plan skill** — `$SKILL_PLAN` from the Step-0 config export."
		const mention = "Resolve nothing new: `$SKILL_PLAN`, `$SKILL_BUILD`, learnings enablement come from the Step-0 export."
		const braced = "Run the **resolved plan skill** — `${SKILL_PLAN}` from the Step-0 config export."
		const framed = "Refresh the claim immediately before this long build dispatch — the resolved build skill `$SKILL_BUILD` is invoked **DIRECTED to:** execute the plan."

		// The marker check is non-vacuous: an unmarked invocation is caught.
		if strings.Contains(unmarked, skillMarker) {
			t.Errorf("fixture error: unmarked line unexpectedly carries the marker")
		}
		if classifyHandoffSite(unmarked) != handoffInvocation {
			t.Errorf("an invocation line was not classified as an invocation")
		}
		// A bare-sigil resolution is a mention (marker-exempt).
		if classifyHandoffSite(mention) != handoffMention {
			t.Errorf("a bare-sigil resolution mention was not classified as a mention")
		}
		// The exception classifier does not match an ordinary invocation line.
		if classifyHandoffSite(unmarked) == handoffException {
			t.Errorf("a plain line was wrongly classified as a human-present exception")
		}
		// Site discovery catches both spellings.
		if !skillSigilRe.MatchString(braced) || !skillSigilRe.MatchString(unmarked) {
			t.Errorf("site discovery missed the braced or bare sigil spelling")
		}
		// The framing ban catches the observed defect shape and permits genuine
		// nested-dispatch prose.
		if !skillFramedRe.MatchString(framed) {
			t.Errorf("framing ban missed the 0351 defect line")
		}
		if skillFramedRe.MatchString("Dispatch the selected rung wrapper by name, foreground") {
			t.Errorf("framing ban wrongly caught genuine nested-dispatch prose")
		}
	})
}
