package repoguard

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Ports tests/test_config_read_channel.sh (change 0120/0146, ADR-0052): the
// PROSE side of ADR-0052's config read-channel rule. A documented config key
// resolves through the config resolver / Step-0 export; no skill body may tell an
// agent to READ the config file directly. The guard classifies every occurrence
// of a config-file token across the skill markdown surface against an inline
// same-line marker, and rejects any unclassified occurrence.
//
// # Why this is a structural scan, not a phrase sentinel
//
// The reject rule is "an UNCLASSIFIED occurrence of a config filename", never "no
// line says <fixed sentence>" — an enumerated spelling misses the next phrasing,
// which is exactly how occurrence #2 shipped after ADR-0052 first stated the rule
// (AGENTS.md: shape, never a spelling). The admissible half is CLOSED and declared
// AT THE SITE with one of two same-line markers:
//
//	<!-- docket:config-read-channel: write-back -->   the line describes a WRITE to the file
//	<!-- docket:config-read-channel: negative -->     the line says the file is NOT read that way
//
// # Same-line attachment, counted per line
//
// The marker sits on the SAME line as the occurrence. Occurrences and markers are
// counted PER LINE and required equal — a line with two occurrences and one marker
// is unclassified, not admitted on the strength of the one marker it carries (the
// per-line fail-open a nearest-preceding-line attachment rule would leave). The
// occurrence test is a substring match, deliberately (change 0146): it can only
// ever OVER-report (demand a marker on a line that arguably needs none), which is
// fail-safe — it can never admit an unmarked real occurrence.
//
// # Token set and its overlap property
//
// ADR-0052's rule is about the config FILE, not one filename: docket documents
// three layers a skill could as wrongly be told to read — the repo-committed
// .docket.yml, the machine-local sibling, and the user-level global config.yml.
// The bare global token subsumes any path-qualified spelling while keeping the set
// overlap-free (no token is a substring of another, and no two co-match an
// overlapping region), so summing strings.Count across the set is exact. That
// overlap property is asserted directly below, not assumed.
//
// # State limitation
//
// The scan reads the on-disk skills/ markdown surface through MaintainedFiles, so
// an untracked scratch skill file in a dirty worktree is in-population until
// removed (the package-wide MaintainedFiles tradeoff). The two convention files
// that DEFINE the config file and its layering are categorically excluded — a
// read-channel rule cannot apply to the documentation of the file itself.

// configTokens is the closed three-member set ADR-0052 scans for. No token is a
// substring of another (proven by TestConfigReadChannelTokenOverlap), so
// strings.Count summed across the set counts each occurrence exactly once.
var configTokens = []string{".docket.yml", ".docket.local.yml", "config.yml"}

// configMarkerRe matches one admissible same-line marker, capturing its class.
// The class set is CLOSED to write-back|negative: an invented class name does not
// match, so the line's marker count stays short of its occurrence count and the
// line is reported unclassified (a future author cannot widen the guard at the
// site it is meant to constrain).
var configMarkerRe = regexp.MustCompile(`<!-- docket:config-read-channel: (write-back|negative) -->`)

// configExcluded are the two convention files that DEFINE the config file, its
// schema, and its layering; a read-channel rule cannot apply to them as written.
// (skills/docket-convention/SKILL.md and its agent-layer reference.)
var configExcluded = map[string]bool{
	"skills/docket-convention/SKILL.md":                  true,
	"skills/docket-convention/references/agent-layer.md": true,
}

// classifyConfigLine returns the token-occurrence count on one line (substring
// match, summed across the whole token set) and the ordered marker classes on the
// same line. This is the whole classifier, so the non_vacuity subtest exercises it
// directly. A line is CLASSIFIED iff len(markers) == occ.
func classifyConfigLine(line string) (occ int, markers []string) {
	for _, tok := range configTokens {
		occ += strings.Count(line, tok)
	}
	for _, m := range configMarkerRe.FindAllStringSubmatch(line, -1) {
		markers = append(markers, m[1])
	}
	return occ, markers
}

// configScan holds the per-file/per-occurrence records of one tree scan, so the
// caller can assert on the POPULATION and not only on the verdicts (an empty walk
// yields zero unclassified findings, byte-identical to a clean tree —
// marker-scoped-guard-needs-a-population-floor).
type configScan struct {
	files        []string // scanned skill files (rel paths)
	classified   []string // one class per classified occurrence
	unclassified []string // "rel:line: <line>" per unclassified line
}

// configCorpus is the maintained skills/ markdown surface minus the two excluded
// convention files.
func configCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !hasExt(rel, ".md") {
			continue
		}
		if configExcluded[rel] {
			continue
		}
		out = append(out, rel)
	}
	return out
}

// scanConfigReadChannel classifies every token occurrence across the corpus.
func scanConfigReadChannel(t *testing.T, root string, corpus []string) configScan {
	t.Helper()
	var s configScan
	for _, rel := range corpus {
		s.files = append(s.files, rel)
		content := readMaintained(t, root, rel)
		for i, line := range strings.Split(content, "\n") {
			occ, markers := classifyConfigLine(line)
			if occ == 0 && len(markers) == 0 {
				continue
			}
			if len(markers) == occ {
				s.classified = append(s.classified, markers...)
			} else {
				s.unclassified = append(s.unclassified, fmt.Sprintf("%s:%d: %s", rel, i+1, line))
			}
		}
	}
	return s
}

func TestConfigReadChannel(t *testing.T) {
	root := guardRoot(t)
	corpus := configCorpus(t, root)

	// Population floors. A glob that matches nothing, an exclusion that swallows
	// the tree, or a reader that finds no occurrences must NOT read as green.
	if len(corpus) < 10 {
		t.Fatalf("population floor: only %d skill files scanned (expected >= 10)", len(corpus))
	}
	inCorpus := func(rel string) bool {
		for _, f := range corpus {
			if f == rel {
				return true
			}
		}
		return false
	}
	if !inCorpus("skills/docket-finalize-change/SKILL.md") {
		t.Errorf("population: the finalize skill is not in the scanned corpus")
	}
	// The exclusions are live: the two convention files exist in the tree but must
	// NOT be scanned...
	for excl := range configExcluded {
		if inCorpus(excl) {
			t.Errorf("population: excluded file %q was scanned", excl)
		}
	}
	// ...while a NON-excluded sibling under the same directory prefix IS scanned
	// (guards against an exclusion match that is accidentally a prefix match).
	if !inCorpus("skills/docket-convention/github-board-mirror.md") {
		t.Errorf("population: a non-excluded docket-convention sibling was not scanned")
	}

	s := scanConfigReadChannel(t, root, corpus)

	// At least three occurrences were reached and classified (the two docket-status
	// write-backs + the board-mirror reference — the true current count; a floor on
	// the scan reaching real occurrences, not a pin of the exact number).
	if len(s.classified) < 3 {
		t.Fatalf("population: only %d classified occurrences (expected >= 3) — scan reached too little", len(s.classified))
	}
	writeBacks := 0
	for _, c := range s.classified {
		if c == "write-back" {
			writeBacks++
		}
	}
	if writeBacks < 1 {
		t.Errorf("coverage: no write-back occurrence exists in the real tree (expected >= 1)")
	}

	// THE RULE: every occurrence in a scanned skill file is classified.
	if len(s.unclassified) != 0 {
		t.Errorf("unclassified config-file occurrences (%d) — each needs a same-line write-back/negative marker or must be removed:\n%s",
			len(s.unclassified), strings.Join(s.unclassified, "\n"))
	}

	t.Run("token_set_is_exactly_three", func(t *testing.T) {
		if len(configTokens) != 3 {
			t.Fatalf("token set has %d members (expected exactly 3)", len(configTokens))
		}
	})

	t.Run("no_two_tokens_overlap", func(t *testing.T) {
		// SEPARATED and ADJACENT probes per ordered pair: the summed count across
		// the whole set must equal exactly 2 (each token matched once, no
		// overlapping third match), or markers==occ would be satisfiable with fewer
		// markers than real occurrences.
		for _, t1 := range configTokens {
			for _, t2 := range configTokens {
				for _, probe := range []string{"x" + t1 + "y" + t2 + "z", t1 + t2} {
					occ, _ := classifyConfigLine(probe)
					if occ != 2 {
						t.Errorf("overlap: probe %q summed to %d occurrences (expected 2)", probe, occ)
					}
				}
			}
		}
	})

	t.Run("non_vacuity", func(t *testing.T) {
		// An unmarked occurrence is unclassified (markers 0 != occ 1).
		if occ, m := classifyConfigLine("merged into `<integration_branch>` (resolved from `.docket.yml`)"); !(occ == 1 && len(m) == 0) {
			t.Errorf("classifier missed an unmarked occurrence: occ=%d markers=%v", occ, m)
		}
		// A marked write-back / negative occurrence is classified, class read off
		// the marker.
		if occ, m := classifyConfigLine("writes it back into `.docket.yml` <!-- docket:config-read-channel: write-back -->"); !(occ == 1 && len(m) == 1 && m[0] == "write-back") {
			t.Errorf("classifier missed a write-back marker: occ=%d markers=%v", occ, m)
		}
		if occ, m := classifyConfigLine("never by parsing `.docket.local.yml` <!-- docket:config-read-channel: negative -->"); !(occ == 1 && len(m) == 1 && m[0] == "negative") {
			t.Errorf("classifier missed a negative marker: occ=%d markers=%v", occ, m)
		}
		// Two occurrences with only ONE marker on the line is unclassified.
		if occ, m := classifyConfigLine("resolved from `.docket.yml`, never `.docket.yml` <!-- docket:config-read-channel: negative -->"); !(occ == 2 && len(m) == 1) {
			t.Errorf("classifier miscounted two-occurrence/one-marker: occ=%d markers=%v", occ, m)
		}
		// The same two occurrences, each with its own marker, is classified.
		if occ, m := classifyConfigLine("`.docket.yml` <!-- docket:config-read-channel: negative -->, `.docket.yml` <!-- docket:config-read-channel: negative -->"); !(occ == 2 && len(m) == 2) {
			t.Errorf("classifier miscounted two-occurrence/two-marker: occ=%d markers=%v", occ, m)
		}
		// An UNKNOWN class name matches no marker, so the line stays unclassified.
		if occ, m := classifyConfigLine("`.docket.yml` <!-- docket:config-read-channel: because-i-said-so -->"); !(occ == 1 && len(m) == 0) {
			t.Errorf("classifier admitted an unknown marker class: occ=%d markers=%v", occ, m)
		}
		// A single .docket.local.yml counts ONCE, not twice (.docket.yml is not a
		// substring of it) — the overlap property applied to the real fail-open
		// change 0146 closed.
		if occ, _ := classifyConfigLine("read `.docket.local.yml` yourself"); occ != 1 {
			t.Errorf("classifier double-counted .docket.local.yml: occ=%d", occ)
		}
	})
}
