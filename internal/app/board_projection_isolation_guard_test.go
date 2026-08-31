package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Change 0367 Task 9: a shape-derived static guard for the projection-isolation
// claim (spec §Projection isolation: "digest, autonomous selection, Mermaid
// graph node/edge order, and GitHub-mirror semantics receive NO board
// presentation input"). The before/after comparison tests
// (TestStatusDigestUnchangedByBoardPresentation,
// TestSelectionUnchangedByBoardPresentation,
// TestBoardMermaidBytesIdenticalAcrossPresentations) prove the behavior HOLDS
// today; this guard proves it stays structurally impossible to violate by
// accident — no consuming-package source may reference the board presentation
// leaves outside the one board-render path.
//
// # Scope and allowlist — derived, never hand-listed
//
// The walk covers the CONSUMING, non-render packages internal/app (the digest
// and the GitHub-mirror / lifecycle operations) and internal/domain (autonomous
// selection and the dependency graph the Mermaid edges derive from). It
// deliberately excludes internal/render (the renderer itself — "non-render") and
// internal/config (the PRODUCER that resolves board.section_order / board.sorting,
// not a consumer of a presentation). Every occurrence of a board-presentation
// leaf — `boardPresentation(`, `BoardPresentation`, `Board.SectionOrder`,
// `Board.Sorting` — in a walked file is a VIOLATION unless it is one of the
// board-render-path shapes below (AGENTS.md: "Key a guard on syntactic shape,
// never an enumerated list of spellings"; "Never hand-list the sites ... derive
// them from a whole-repo grep, then sort them into prose vs executable"):
//
//   - LIFT OWNER — a file that defines `func boardPresentation(` (derived_views.go).
//     It owns the single config→renderer type lift, so it alone may read the
//     config board leaves and name the render presentation type.
//   - INCLUSION ARGUMENT — a `boardPresentation(` occurrence that is an argument
//     to a board-inclusion call (`includeBoard(`, `planInlineBoard(`,
//     `renderCanonicalBoard(`). This is exactly the "op files' includeBoard
//     argument expressions" the change ops use to route the resolved presentation
//     into the renderer, and nothing else.
//   - RENDER-HELPER TYPE — a `render.BoardPresentation` type occurrence in a file
//     that IS on the board-render path (defines the lift or calls a board-inclusion
//     helper): the parameter/return types of includeBoard / planInlineBoard /
//     renderCanonicalBoard.
//   - CONFIG INSPECTION — a `Board.SectionOrder` / `Board.Sorting` read in the
//     config-inspection surface (a file that renders `leafLine("board.section_order"`).
//     `docket config` DISPLAYS every resolved leaf as text; it performs no change
//     ordering, so surfacing the board leaves' values is not a presentation input
//     to any projection.
//
// A leak — the digest sorting its change rows by `eff.Board.Sorting`, selection
// consulting a `render.BoardPresentation`, the mermaid loop reading the order —
// matches none of these shapes and is named. TestProjectionIsolationGuardIsFalsifiable
// mutation-tests every arm.
//
// FLOOR (learning marker-scoped-guard-needs-a-population-floor): the walk must
// visit a non-zero file count AND find `boardPresentation(` in the allowed set at
// least once — an empty or mis-rooted walk finds neither and cannot go vacuously
// green.

// projectionLeafPatterns is the closed set of board-presentation leaf spellings a
// consuming-package source must not carry outside the board-render path.
var projectionLeafPatterns = []string{
	"boardPresentation(",
	"BoardPresentation",
	"Board.SectionOrder",
	"Board.Sorting",
}

// boardInclusionCalls are the board-render-path sinks the resolved presentation is
// routed into. A `boardPresentation(` argument to one of these is the sanctioned
// op-file expression; a `render.BoardPresentation` type in a file that calls one
// is a render-helper signature.
var boardInclusionCalls = []string{"includeBoard(", "planInlineBoard(", "renderCanonicalBoard("}

// projScanResult is one walk of the consuming packages.
type projScanResult struct {
	violations       []string
	visited          int
	allowedBoardPres int // `boardPresentation(` occurrences classified allowed
}

// scanProjectionIsolation walks every non-test .go file under each root, reads
// each file's board-render role from its own text, and classifies every
// board-presentation leaf occurrence as allowed or a violation.
func scanProjectionIsolation(t *testing.T, roots ...string) projScanResult {
	t.Helper()
	var res projScanResult
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				t.Fatalf("read %s: %v", p, rerr)
			}
			res.visited++
			text := string(data)
			isLiftOwner := strings.Contains(text, "func boardPresentation(")
			isInclusionCaller := isLiftOwner || containsAny(text, boardInclusionCalls)
			isConfigInspection := strings.Contains(text, `leafLine("board.section_order"`)
			base := filepath.Base(p)

			for i, line := range strings.Split(text, "\n") {
				for _, pat := range projectionLeafPatterns {
					if !strings.Contains(line, pat) {
						continue
					}
					allowed, isBoardPres := classifyLeafOccurrence(
						pat, line, isLiftOwner, isInclusionCaller, isConfigInspection)
					if allowed && isBoardPres {
						res.allowedBoardPres++
					}
					if !allowed {
						res.violations = append(res.violations, base+":"+strconv.Itoa(i+1)+
							": board presentation leaf "+pat+" reaches a consuming path outside the board-render path: "+
							strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return res
}

// classifyLeafOccurrence reports whether one leaf occurrence on a line is an
// allowed board-render-path shape, and whether the leaf is `boardPresentation(`
// (for the population floor).
func classifyLeafOccurrence(pat, line string, isLiftOwner, isInclusionCaller, isConfigInspection bool) (allowed, isBoardPres bool) {
	switch pat {
	case "boardPresentation(":
		// The lift definition itself, or an argument to a board-inclusion call.
		allowed = strings.Contains(line, "func boardPresentation(") || containsAny(line, boardInclusionCalls)
		return allowed, true
	case "BoardPresentation":
		// The render presentation type, only in a board-render-path helper.
		return isLiftOwner || isInclusionCaller, false
	case "Board.SectionOrder", "Board.Sorting":
		// The config board leaves: readable by the lift owner, or displayed by the
		// config-inspection surface.
		return isLiftOwner || isConfigInspection, false
	}
	return false, false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// projectionScanRoots are the consuming, non-render packages the guard walks:
// the app layer (digest + mirror/lifecycle ops) and the domain layer (selection
// + dependency graph). The test runs in package app, so "." is internal/app and
// "../domain" is internal/domain; internal/render (the renderer) and
// internal/config (the presentation producer) are deliberately out of scope.
var projectionScanRoots = []string{".", filepath.Join("..", "domain")}

// TestProjectionIsolationGuardHoldsOverConsumingPackages enforces the isolation
// invariant over the real consuming-package source.
func TestProjectionIsolationGuardHoldsOverConsumingPackages(t *testing.T) {
	res := scanProjectionIsolation(t, projectionScanRoots...)

	for _, v := range res.violations {
		t.Errorf("projection-isolation violation: %s", v)
	}
	if res.visited == 0 {
		t.Fatalf("projection-isolation guard visited zero files — an unfalsifiable walker is decoration")
	}
	// FLOOR: the board-render path must be present in the walk, or the guard has
	// no anchor and could pass over an empty/mis-rooted tree.
	if res.allowedBoardPres == 0 {
		t.Fatalf("guard found no allowed boardPresentation( occurrence in %v — "+
			"the walk did not reach the board-render path; the invariant would be vacuous",
			projectionScanRoots)
	}
}

// TestProjectionIsolationGuardIsFalsifiable mutation-tests the guard: it plants
// each leak shape into a temp source tree and confirms the scanner names it,
// confirms the sanctioned board-render and config-inspection shapes are clean,
// and confirms an empty tree finds no allowed anchor (tripping the floor). A
// guard that cannot redden is decoration (AGENTS.md).
func TestProjectionIsolationGuardIsFalsifiable(t *testing.T) {
	// Arm (a): the digest sorts change rows by the config board leaf.
	dirA := t.TempDir()
	writeGoFile(t, dirA, "leak.go", "package p\n\n"+
		"func assemble(eff config.Effective, rows []Change) {\n"+
		"\tsort.Slice(rows, less(eff.Board.Sorting[\"proposed\"]))\n"+
		"}\n")
	if res := scanProjectionIsolation(t, dirA); len(res.violations) == 0 {
		t.Errorf("arm (a): planted digest read of eff.Board.Sorting was not flagged")
	}

	// Arm (b): a consuming path names the render presentation type directly.
	dirB := t.TempDir()
	writeGoFile(t, dirB, "leak.go", "package p\n\n"+
		"func order(p render.BoardPresentation, rows []Change) {}\n")
	if res := scanProjectionIsolation(t, dirB); len(res.violations) == 0 {
		t.Errorf("arm (b): planted render.BoardPresentation reference was not flagged")
	}

	// Arm (c): a consuming path calls the lift outside an inclusion argument.
	dirC := t.TempDir()
	writeGoFile(t, dirC, "leak.go", "package p\n\n"+
		"func order(eff config.Effective, rows []Change) {\n"+
		"\tpres := boardPresentation(eff)\n"+
		"\tsortRowsBy(rows, pres)\n"+
		"}\n")
	if res := scanProjectionIsolation(t, dirC); len(res.violations) == 0 {
		t.Errorf("arm (c): planted boardPresentation( call outside an inclusion argument was not flagged")
	}

	// Control — a compliant op file: boardPresentation( is only ever an
	// includeBoard argument, so it is allowed and counts toward the floor.
	dirOK := t.TempDir()
	writeGoFile(t, dirOK, "op.go", "package p\n\n"+
		"func (o op) Plan() {\n"+
		"\t_ = includeBoard(ctx, tree, boardPath, candidate, boardPresentation(o.eff), &files)\n"+
		"}\n")
	resOK := scanProjectionIsolation(t, dirOK)
	if len(resOK.violations) != 0 {
		t.Errorf("control: compliant includeBoard(boardPresentation(...)) op flagged: %v", resOK.violations)
	}
	if resOK.allowedBoardPres == 0 {
		t.Errorf("control: compliant op did not count toward the boardPresentation floor")
	}

	// Control — the lift owner: it alone reads the config board leaves and names
	// the render presentation type, and is clean.
	dirLift := t.TempDir()
	writeGoFile(t, dirLift, "derived_views.go", "package p\n\n"+
		"func boardPresentation(eff config.Effective) render.BoardPresentation {\n"+
		"\t_ = eff.Board.SectionOrder.Value\n"+
		"\t_ = eff.Board.Sorting\n"+
		"\treturn render.BoardPresentation{}\n"+
		"}\n")
	if res := scanProjectionIsolation(t, dirLift); len(res.violations) != 0 {
		t.Errorf("control: lift owner flagged: %v", res.violations)
	}

	// Control — the config-inspection surface: it displays the board leaves and is clean.
	dirCfg := t.TempDir()
	writeGoFile(t, dirCfg, "config.go", "package p\n\n"+
		"func lines(eff config.Effective) {\n"+
		"\tleafLine(\"board.section_order\", listValue(eff.Board.SectionOrder.Value), eff.Board.SectionOrder.Provenance)\n"+
		"\tsrt := eff.Board.Sorting[s]\n"+
		"\t_ = srt\n"+
		"}\n")
	if res := scanProjectionIsolation(t, dirCfg); len(res.violations) != 0 {
		t.Errorf("control: config-inspection surface flagged: %v", res.violations)
	}

	// Floor / non-vacuity — an empty tree finds no allowed anchor.
	dirEmpty := t.TempDir()
	if res := scanProjectionIsolation(t, dirEmpty); res.allowedBoardPres != 0 || res.visited != 0 {
		t.Errorf("empty tree: allowedBoardPres=%d visited=%d, want 0/0", res.allowedBoardPres, res.visited)
	}
}
