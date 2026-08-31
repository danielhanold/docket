package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// Change 0377 Task 8: a mutation-shape structural guard proving that every
// board-authoritative change-record mutation in package app reaches the one
// shared board-inclusion path, includeBoard (spec §Derived-view ownership: "an
// operation changing a board-authoritative change record cannot land without the
// canonical board output in the same transaction"). It keys on the mutation's
// SHAPE and the operations' common ownership, never on a hand-maintained list of
// command names (AGENTS.md: "Key a guard on syntactic shape, never an enumerated
// list of spellings").
//
// # The pinned change-record-write idiom
//
// After Task 7 the BOARD.md and ADR-index (README.md) transaction.FileMutation
// literals live ONLY inside derived_views.go (includeBoard / includeADRIndex).
// Every OTHER transaction.FileMutation write-kind literal in package app writes a
// primary record (change, learning, ADR) or a derived artifact-link block. A
// board-authoritative CHANGE-record write is recognized structurally by the
// union of two shapes — neither an enumerated path spelling:
//
//   - CHANGE-PATH shape: the write's Path is gitcli.RepoPath(X.Path()) — a CALL
//     of a no-arg method named Path. In today's corpus this call form appears
//     ONLY on a domain.Change value (`c.Path()`), so it uniquely marks an
//     in-place change-record rewrite (claim, attach, implemented, reclaim,
//     reconcile, repair, halt, resume-halted, block, clear-block, stacked
//     closeout). Every non-change write instead uses a plain identifier
//     (recPath, adrRelPath, activePath, archivePath, specPath, p) or a field
//     selector (o.req.Path, p.tg.archivePath, o.req.Target.Path) — never a
//     Path() call.
//   - CANDIDATE shape: the writer reaches repository.BuildSnapshot. The
//     create/relocate change ops (create, kill, groom, lifecycle, and archive
//     closeout) write a computed placement path, not c.Path(), but they all
//     rebuild the candidate change snapshot through repository.BuildSnapshot
//     (directly or via a build*Candidate helper) — the "InputDocument feeding
//     BuildSnapshot" shape the plan names. Read-only context/status/check ops
//     also reach BuildSnapshot but write no record FileMutation, so they never
//     enter the population.
//
// A record writer that owns a DIFFERENT derived view — it reaches includeADRIndex
// — is an ADR operation (spec: "ADR operations render the index from the
// candidate snapshot"). Its incidental rewrite of a producing change's
// artifact-link block is not a board-authoritative field edit, so it is excluded
// from the board population by construction, not by name.
//
// # The invariant, enforced both directions
//
// Let CR = functions in maintained package-app source that write a change-record
// FileMutation (CHANGE-PATH or CANDIDATE shape) and do not own the ADR index.
//   - FORWARD (the teeth): every function in CR reaches includeBoard. Strip the
//     includeBoard call from any board op and the offender is named.
//   - REVERSE (learning correspondence-guard-runs-one-way): every record writer
//     that reaches includeBoard is in CR — the board is never emitted by an
//     operation that is not a change-record mutator. Together the two directions
//     assert CR == the includeBoard-reaching record writers exactly.
//   - FLOOR (learning marker-scoped-guard-needs-a-population-floor,
//     backstop-must-compute-not-reenumerate): |CR| is at least a floor computed
//     from the maintained product shape — the count of change_*.go operation
//     files — and the scan visited a non-zero file count. An empty package
//     yields an empty CR that trips the floor, so a green run can never be
//     vacuous. The floor is COMPUTED from the filesystem, never written.
//
// Residual: the guard sees transaction.FileMutation composite literals in
// maintained (non-test) package-app source. A future change-record write built
// through a different construction, or a non-change write that adopts the
// Path()-call form, is outside these shapes; TestDerivedViewsGuardIsFalsifiable
// pins the shapes so such drift is a visible test edit, and whole-branch review
// owns anything the shapes do not reach.

// dvWriteMutationKinds is the closed set of transaction.MutationKind selector
// names that WRITE a path (a create/replace/delete of the file). A FileMutation
// carrying one is a record/derived-view write; anything else is not a write.
var dvWriteMutationKinds = map[string]bool{
	"MutationCreate":  true,
	"MutationReplace": true,
	"MutationDelete":  true,
}

// dvFuncFacts is the per-function shape read the guard needs.
type dvFuncFacts struct {
	name                  string // report name: "RecvType.Method" or "Func"
	isRecordWriter        bool   // constructs a write-kind transaction.FileMutation literal
	writesChangePathCall  bool   // some such write's Path is gitcli.RepoPath(X.Path())
	directBuildSnapshot   bool   // calls repository.BuildSnapshot
	directIncludeBoard    bool   // calls includeBoard
	directIncludeADRIndex bool   // calls includeADRIndex
	callees               []string
}

// dvScan is one scan of a source root.
type dvScan struct {
	changeRecordMutators []string // CR: board-authoritative change-record writers
	boardReachingWriters []string // record writers that reach includeBoard
	recordWriters        int
	visited              int
}

// scanDerivedViewsGuard parses every non-test .go file under root, reads each
// function's mutation shape, and classifies the board-authoritative change-record
// mutators (CR) and the record writers that reach includeBoard.
func scanDerivedViewsGuard(t *testing.T, root string) dvScan {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", root, err)
	}

	var all []*dvFuncFacts
	free := map[string]*dvFuncFacts{}
	visited := 0
	for _, pkg := range pkgs {
		for range pkg.Files {
			visited++
		}
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				facts := readFuncFacts(fd)
				all = append(all, facts)
				if fd.Recv == nil {
					free[fd.Name.Name] = facts
				}
			}
		}
	}

	reaches := func(start *dvFuncFacts, pick func(*dvFuncFacts) bool) bool {
		seen := map[string]bool{}
		var dfs func(f *dvFuncFacts) bool
		dfs = func(f *dvFuncFacts) bool {
			if pick(f) {
				return true
			}
			for _, c := range f.callees {
				cf, ok := free[c]
				if !ok || seen[c] {
					continue
				}
				seen[c] = true
				if dfs(cf) {
					return true
				}
			}
			return false
		}
		return dfs(start)
	}

	scan := dvScan{visited: visited}
	for _, f := range all {
		if !f.isRecordWriter {
			continue
		}
		scan.recordWriters++
		reachesBuildSnapshot := reaches(f, func(x *dvFuncFacts) bool { return x.directBuildSnapshot })
		reachesADRIndex := reaches(f, func(x *dvFuncFacts) bool { return x.directIncludeADRIndex })
		reachesBoard := reaches(f, func(x *dvFuncFacts) bool { return x.directIncludeBoard })

		isChangeRecordMutator := (f.writesChangePathCall || reachesBuildSnapshot) && !reachesADRIndex
		if isChangeRecordMutator {
			scan.changeRecordMutators = append(scan.changeRecordMutators, f.name)
		}
		if reachesBoard {
			scan.boardReachingWriters = append(scan.boardReachingWriters, f.name)
		}
	}
	return scan
}

// readFuncFacts reads one function's mutation shape from its AST.
func readFuncFacts(fd *ast.FuncDecl) *dvFuncFacts {
	f := &dvFuncFacts{name: dvFuncName(fd)}
	record := func(lit *ast.CompositeLit) {
		kind, path := dvFileMutationFields(lit)
		if kind == "" || !dvWriteMutationKinds[kind] {
			return
		}
		f.isRecordWriter = true
		if dvIsChangePathCall(path) {
			f.writesChangePathCall = true
		}
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			// A directly-typed literal: transaction.FileMutation{...}.
			if dvIsSelector(node.Type, "transaction", "FileMutation") {
				record(node)
				return true
			}
			// A slice literal []transaction.FileMutation{...} whose elements are
			// elided-type composite literals (their own Type is nil, so they are
			// only reachable here, through the slice's element type).
			if at, ok := node.Type.(*ast.ArrayType); ok && dvIsSelector(at.Elt, "transaction", "FileMutation") {
				for _, elt := range node.Elts {
					if lit, ok := elt.(*ast.CompositeLit); ok {
						record(lit)
					}
				}
			}
		case *ast.CallExpr:
			switch fn := node.Fun.(type) {
			case *ast.Ident:
				f.callees = append(f.callees, fn.Name)
				switch fn.Name {
				case "includeBoard":
					f.directIncludeBoard = true
				case "includeADRIndex":
					f.directIncludeADRIndex = true
				}
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); ok && x.Name == "repository" && fn.Sel.Name == "BuildSnapshot" {
					f.directBuildSnapshot = true
				}
			}
		}
		return true
	})
	return f
}

// dvFileMutationFields returns the selector name of the Kind field's value and
// the Path field's value expression from a keyed transaction.FileMutation
// literal. Every FileMutation literal in package app is keyed (Path:/Kind:).
func dvFileMutationFields(lit *ast.CompositeLit) (kind string, path ast.Expr) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Kind":
			if sel, ok := kv.Value.(*ast.SelectorExpr); ok {
				kind = sel.Sel.Name
			}
		case "Path":
			path = kv.Value
		}
	}
	return kind, path
}

// dvIsChangePathCall reports whether a FileMutation Path expression is
// gitcli.RepoPath(X.Path()) — a call of a no-arg method named Path, the shape
// that uniquely marks an in-place change-record rewrite.
func dvIsChangePathCall(path ast.Expr) bool {
	conv, ok := path.(*ast.CallExpr)
	if !ok || !dvIsSelector(conv.Fun, "gitcli", "RepoPath") || len(conv.Args) != 1 {
		return false
	}
	inner, ok := conv.Args[0].(*ast.CallExpr)
	if !ok || len(inner.Args) != 0 {
		return false
	}
	sel, ok := inner.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Path"
}

// dvIsSelector reports whether e is the selector pkg.name.
func dvIsSelector(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// dvFuncName is the report name: "RecvType.Method" for a method, else the
// function name.
func dvFuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return dvRecvTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

func dvRecvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return dvRecvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return dvRecvTypeName(t.X)
	default:
		return "?"
	}
}

// dvChangeOpFloor counts the change_*.go operation files under root (non-test) —
// the maintained product shape the population floor is computed from. Each is at
// least one board-authoritative change-record mutator, so it is a true lower
// bound on |CR|.
func dvChangeOpFloor(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read dir %s: %v", root, err)
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "change_") && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			n++
		}
	}
	return n
}

// TestDerivedViewsGuardBoardOwnership enforces the mutation-shape invariant over
// the real package-app source.
func TestDerivedViewsGuardBoardOwnership(t *testing.T) {
	const root = "."
	scan := scanDerivedViewsGuard(t, root)
	floor := dvChangeOpFloor(t, root)

	if scan.visited == 0 {
		t.Fatalf("derived-view guard visited zero files — an unfalsifiable walker is decoration")
	}

	// FORWARD: every board-authoritative change-record mutator reaches includeBoard.
	boardSet := map[string]bool{}
	for _, n := range scan.boardReachingWriters {
		boardSet[n] = true
	}
	for _, m := range scan.changeRecordMutators {
		if !boardSet[m] {
			t.Errorf("board-authoritative change-record mutation %s does not reach includeBoard "+
				"(it writes a change record but omits the canonical board output — spec §Derived-view ownership)", m)
		}
	}

	// REVERSE: every record writer reaching includeBoard is a change-record mutator.
	mutatorSet := map[string]bool{}
	for _, n := range scan.changeRecordMutators {
		mutatorSet[n] = true
	}
	for _, b := range scan.boardReachingWriters {
		if !mutatorSet[b] {
			t.Errorf("record writer %s reaches includeBoard but is not classified as a change-record mutator "+
				"— the board is being emitted outside board ownership (correspondence-guard-runs-one-way)", b)
		}
	}

	// FLOOR: the population is non-vacuous and at least the maintained product shape.
	if floor == 0 {
		t.Fatalf("computed floor is zero — no change_*.go operation files found under %s", root)
	}
	if len(scan.changeRecordMutators) < floor {
		t.Fatalf("found %d board-authoritative change-record mutators, want >= %d (one per change_*.go op file) "+
			"— the population scan is under-counting or vacuous", len(scan.changeRecordMutators), floor)
	}
}

// TestDerivedViewsGuardIsFalsifiable mutation-tests the guard: it plants a
// change-record mutator that omits includeBoard and confirms the classifier
// flags it, confirms a compliant op is clean, and confirms an empty package
// yields an empty population that trips the floor (non-vacuity). A guard that
// cannot redden is decoration (AGENTS.md).
func TestDerivedViewsGuardIsFalsifiable(t *testing.T) {
	// Arm (a) — CHANGE-PATH shape without includeBoard: a mutator, not board-reaching.
	dirA := t.TempDir()
	writeGoFile(t, dirA, "op.go", "package p\n\n"+
		"func (o fakeOp) Plan() {\n"+
		"\tfiles := []transaction.FileMutation{{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: b}}\n"+
		"\t_ = files\n"+
		"}\n")
	scanA := scanDerivedViewsGuard(t, dirA)
	if !dvContains(scanA.changeRecordMutators, "fakeOp.Plan") {
		t.Errorf("arm (a): planted change-path writer not classified as a change-record mutator: %v", scanA.changeRecordMutators)
	}
	if dvContains(scanA.boardReachingWriters, "fakeOp.Plan") {
		t.Errorf("arm (a): planted writer omitting includeBoard was wrongly reported as board-reaching")
	}

	// Arm (b) — CANDIDATE shape (computed path + BuildSnapshot) without includeBoard.
	dirB := t.TempDir()
	writeGoFile(t, dirB, "op.go", "package p\n\n"+
		"func (o relocOp) Plan() {\n"+
		"\tsnap, _ := repository.BuildSnapshot(in)\n"+
		"\tfiles := []transaction.FileMutation{{Path: gitcli.RepoPath(newPath), Kind: transaction.MutationCreate, Bytes: b}}\n"+
		"\t_, _ = snap, files\n"+
		"}\n")
	scanB := scanDerivedViewsGuard(t, dirB)
	if !dvContains(scanB.changeRecordMutators, "relocOp.Plan") {
		t.Errorf("arm (b): planted BuildSnapshot writer not classified as a change-record mutator: %v", scanB.changeRecordMutators)
	}
	if dvContains(scanB.boardReachingWriters, "relocOp.Plan") {
		t.Errorf("arm (b): planted BuildSnapshot writer omitting includeBoard was wrongly reported as board-reaching")
	}

	// Control — a compliant op reaching includeBoard is in both sets (no violation).
	dirOK := t.TempDir()
	writeGoFile(t, dirOK, "op.go", "package p\n\n"+
		"func (o okOp) Plan() {\n"+
		"\tfiles := []transaction.FileMutation{{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: b}}\n"+
		"\t_ = includeBoard(ctx, tree, boardPath, candidate, &files)\n"+
		"}\n")
	scanOK := scanDerivedViewsGuard(t, dirOK)
	if !dvContains(scanOK.changeRecordMutators, "okOp.Plan") || !dvContains(scanOK.boardReachingWriters, "okOp.Plan") {
		t.Errorf("control: compliant op not in both sets: mutators=%v board=%v", scanOK.changeRecordMutators, scanOK.boardReachingWriters)
	}

	// Control — an ADR-index-owning writer of a change path is excluded from CR.
	dirADR := t.TempDir()
	writeGoFile(t, dirADR, "op.go", "package p\n\n"+
		"func (o adrOp) Plan() {\n"+
		"\t_ = includeADRIndex(ctx, tree, candidate, indexPath, &files)\n"+
		"\tfiles := []transaction.FileMutation{{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: b}}\n"+
		"\t_ = files\n"+
		"}\n")
	scanADR := scanDerivedViewsGuard(t, dirADR)
	if dvContains(scanADR.changeRecordMutators, "adrOp.Plan") {
		t.Errorf("control: ADR-index-owning writer wrongly classified as a board change-record mutator")
	}

	// Floor / non-vacuity — an empty package has zero mutators, below any real floor.
	dirEmpty := t.TempDir()
	scanEmpty := scanDerivedViewsGuard(t, dirEmpty)
	if len(scanEmpty.changeRecordMutators) != 0 {
		t.Errorf("empty package reported %d change-record mutators, want 0", len(scanEmpty.changeRecordMutators))
	}
	if realFloor := dvChangeOpFloor(t, "."); realFloor == 0 {
		t.Errorf("real product floor computed as zero — floor check would be vacuous")
	}
}

func dvContains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
