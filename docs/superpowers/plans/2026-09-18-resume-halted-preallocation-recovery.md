<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0368 — Recover a run halted before its workspace was allocated](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0368-resume-halted-preallocation-recovery.md)**
<!-- docket:backlink:end -->
# Recover a Run Halted Before Workspace Allocation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a human-authorized `change resume-halted` recover an in-progress change that halted before its feature workspace was ever allocated, by teaching workspace inspection to distinguish a *proven-absent* workspace (`StateAbsent`) from a foreign one, and by admitting that state into the existing resume operation only after proving the recorded remote feature branch is also cleanly absent.

**Architecture:** Three layers change. (1) `internal/workspace`: factor the local all-absent inventory checks out of `inventoryForFresh` into narrow read-only helpers with fail-closed registration semantics, and extend `Service.Inspect` to return a new `StateAbsent` kind when the manifest, local feature ref, target path, and worktree registration are all *cleanly* absent. (2) `internal/app/change_halt.go`: replace the open-default `resumeQuiescenceRefusal` with a closed admission classification; the absent state additionally requires a clean `ProbeRemoteBranch` absence of the resolved recorded feature ref before the existing `resumeHaltedOp` transaction runs unchanged. (3) Every other workspace-state consumer keeps its pre-change behavior for the newly split-out absent case (reclaim still gated by strict expiry, maintenance never reads absence as clean, repair still reports no conflict, recertify/rebase still refuse).

**Tech Stack:** Go (module `github.com/danielhanold/docket`), stdlib `testing`, real-git test harnesses already in `internal/workspace/harness_test.go` and `internal/app` (`planRepoModes`, `setupHaltedFixture`).

**Spec:** `docs/superpowers/specs/2026-09-18-resume-halted-preallocation-recovery-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary tree). The change file is `docs/changes/active/0368-resume-halted-preallocation-recovery.md`.

## Global Constraints

- No reclaim-TTL change, no new command, no bypass flag, no workspace allocation/adoption/deletion during resume, no manifest schema migration, no gate/cancellation protocol change (ADR-0118 untouched).
- `Inspect` stays local-only, read-only, and never fetches. `StateAbsent` is a current local observation, never proof a worker stopped, and grants no cleanup, launch, or cancellation authority.
- Three-outcome probes everywhere (learning `probe-error-is-not-clean-absence`): present / cleanly-absent / unknown, and *unknown never shares a branch with absent*. A dangling symlink counts as present. A stale registration counts as present. Unresolved registration identity prevents the absence result.
- Unknown workspace-state strings must not silently authorize resume (closed allowlist, not an open default).
- Every guard/conjunct added here gets mutation-tested with `go test -count=1` (learning `cached-runner-serves-a-mutated-tree`); a removed proof must redden a targeted test.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- Build gate: run the complete suite via the configured `build.test_command` (read from config — the Go runner `internal/suiterunner` is the sole channel), from this feature checkout. Read the budget report even on green.
- Commit per task; `gofmt` every touched file (finalize has previously reddened on drift).

## File Structure

- `internal/workspace/inspect.go` — new `StateAbsent` kind + absent-manifest classification arm.
- `internal/workspace/prepare.go` — `inventoryForFresh` rewired onto the shared helpers; new helpers live beside `pathPresent`/`worktreeAt` (same file; the package keeps probe primitives in `prepare.go` today, follow that).
- `internal/workspace/inspect_test.go`, `internal/workspace/prepare_test.go` — new/updated coverage.
- `internal/app/change_halt.go` — closed resume admission + remote-absence proof for `StateAbsent`; two new stable reason constants.
- `internal/app/change_halt_test.go`, `internal/app/change_integration_test.go` — fake-service admission coverage + real-service end-to-end regression.
- `internal/app/maintenance_assess.go`, `internal/app/change_repair.go` — carry the split state with pre-change behavior; their tests.
- `internal/app/change_reclaim_test.go`, `internal/app/evidence_recertify_test.go` (or nearest existing coverage) — pin that absent does not loosen reclaim/recertify.

---

### Task 1: Shared read-only absence helpers in `internal/workspace`

**Files:**
- Modify: `internal/workspace/prepare.go`
- Test: `internal/workspace/prepare_test.go`

**Interfaces:**
- Produces (used by Tasks 2 and 6):

```go
// registrationAbsence is the three-outcome classification of whether any Git
// worktree registration occupies a canonical target path or references a
// feature ref.
type registrationAbsence int

const (
	regAbsent     registrationAbsence = iota // nothing occupies the path or references the ref
	regPresent                               // a registration occupies the path (live or stale) or references the feature ref
	regUnresolved                            // a registration's path identity could not be resolved; absence is unprovable
)

// classifyRegistrationAbsence proves, fail-closed, that no registration
// occupies want (a canonical intended path) or references featureRef.
func classifyRegistrationAbsence(infos []gitcli.WorktreeInfo, want string, featureRef gitcli.RefName) registrationAbsence

// localRefAbsent is the three-outcome local feature-ref probe: (true, nil) on
// a cleanly absent ref (KindRefUnavailable), (false, nil) when it resolves,
// and (false, err) on any other probe failure — never read as absence.
func (s *Service) localRefAbsent(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName) (bool, error)
```

- Consumes: existing `worktreeAt`, `canonicalizePath`, `pathPresent`, `gitcli.WorktreeInfo{Path, Branch, Detached, Head}`, `gitcli.AsFailure`, `gitcli.KindRefUnavailable`.

`classifyRegistrationAbsence` semantics (implement exactly; each rule is spec-mandated):
1. For each `info`: if `info.Branch == featureRef` → `regPresent` (a registration on the feature ref *anywhere* blocks absence).
2. Canonicalize `info.Path` via `canonicalizePath`. On success, `== want` → `regPresent`.
3. On canonicalization failure (typical for a *stale* registration whose directory is gone): compare lexically — `filepath.Clean(abs)` of `info.Path` equal to `want` → `regPresent` (an exact stale registration counts as present even though its directory is gone).
4. On canonicalization failure where the lexical form does *not* match `want` → remember `regUnresolved` but keep scanning (a later entry may still prove `regPresent`).
5. No entry matched, none unresolved → `regAbsent`; otherwise `regUnresolved`.

Note the deliberate divergence from `worktreeAt`, which *skips* uncanonicalizable registrations: skipping is fine for "find the worktree at X" but is insufficient proof of absence. Do not change `worktreeAt`.

- [ ] **Step 1: Write failing tests for `classifyRegistrationAbsence`**

In `internal/workspace/prepare_test.go` (table test, no git needed — it takes `[]gitcli.WorktreeInfo`):

```go
func TestClassifyRegistrationAbsence(t *testing.T) {
	want := t.TempDir() // canonical existing dir stands in for the intended path
	wantCanon, err := canonicalizePath(want)
	if err != nil {
		t.Fatal(err)
	}
	feature := gitcli.RefName("refs/heads/feat/absent-probe")
	other := gitcli.RefName("refs/heads/feat/other")
	gone := filepath.Join(t.TempDir(), "vanished") // never created: canonicalization fails

	cases := []struct {
		name  string
		infos []gitcli.WorktreeInfo
		want  registrationAbsence
	}{
		{"empty list is absent", nil, regAbsent},
		{"unrelated registration is absent", []gitcli.WorktreeInfo{{Path: t.TempDir(), Branch: other}}, regAbsent},
		{"registration at the path is present", []gitcli.WorktreeInfo{{Path: want, Branch: other}}, regPresent},
		{"registration on the feature ref elsewhere is present", []gitcli.WorktreeInfo{{Path: t.TempDir(), Branch: feature}}, regPresent},
		{"stale registration at the exact path is present", []gitcli.WorktreeInfo{{Path: filepath.Join(wantCanon, "..", filepath.Base(wantCanon)), Branch: other}}, regPresent},
		{"unresolvable unrelated registration is unresolved", []gitcli.WorktreeInfo{{Path: gone, Branch: other}}, regUnresolved},
		{"present beats unresolved", []gitcli.WorktreeInfo{{Path: gone, Branch: other}, {Path: want, Branch: other}}, regPresent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyRegistrationAbsence(tc.infos, wantCanon, feature); got != tc.want {
				t.Errorf("classifyRegistrationAbsence = %v; want %v", got, tc.want)
			}
		})
	}
}
```

Careful with the "stale at the exact path" fixture: to exercise the *lexical* arm the path must fail canonicalization yet clean to `wantCanon`. A path that exists canonicalizes fine, so instead use a nonexistent path that cleans to the want: `filepath.Join(t.TempDir(), "ws")` as `want` (do **not** create it; `wantCanon` = its parent canonicalized + `/ws`, mirroring how `intendedPath` is built for a not-yet-created worktree), and register `Path: want` — `canonicalizePath` fails (no such file), the lexical compare matches. Restructure the fixture that way; the table's intent stands.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/workspace/ -run TestClassifyRegistrationAbsence -count=1`
Expected: FAIL — `undefined: classifyRegistrationAbsence`.

- [ ] **Step 3: Implement the helpers**

At the bottom of `internal/workspace/prepare.go`, beside `worktreeAt`:

```go
// classifyRegistrationAbsence proves, fail-closed, that no worktree
// registration occupies the canonical target path or references the feature
// ref. Unlike worktreeAt — which may SKIP a registration whose path cannot be
// canonicalized, sufficient for lookup but not for an absence proof — this
// recognizes an exact stale registration lexically and reports any other
// unresolvable identity as regUnresolved, which callers must never read as
// absence (change 0368).
func classifyRegistrationAbsence(infos []gitcli.WorktreeInfo, want string, featureRef gitcli.RefName) registrationAbsence {
	verdict := regAbsent
	for _, info := range infos {
		if !info.Detached && info.Branch == featureRef {
			return regPresent
		}
		if cp, err := canonicalizePath(info.Path); err == nil {
			if cp == want {
				return regPresent
			}
			continue
		}
		// The registered path no longer resolves. An exact lexical match is a
		// stale registration at the target: present. Anything else is an
		// unresolved identity: absence is unprovable.
		if abs, aerr := filepath.Abs(info.Path); aerr == nil && filepath.Clean(abs) == want {
			return regPresent
		}
		verdict = regUnresolved
	}
	return verdict
}

// localRefAbsent is the three-outcome local ref probe shared by fresh
// allocation and the absence classification: a KindRefUnavailable failure is
// clean absence, a resolution is presence, and any other failure is a real
// probe error that never reads as absence.
func (s *Service) localRefAbsent(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName) (bool, error) {
	if _, err := s.git.ResolveRef(ctx, repo, ref); err == nil {
		return false, nil
	} else if f, ok := gitcli.AsFailure(err); !ok || f.Kind != gitcli.KindRefUnavailable {
		return false, err
	}
	return true, nil
}
```

(Decide on `info.Detached` handling for rule 1: a detached worktree has no branch, so `Branch` is zero — the `!info.Detached` guard just documents that; keep it.)

- [ ] **Step 4: Rewire `inventoryForFresh` onto the helpers**

Replace its local-ref probe with `localRefAbsent` (mapping `(false, nil)` → blocked, error → `mapGitFailure(prepareOp, "inventory", err)`), and replace the final `registeredAt(infos, intendedPath)` check with:

```go
	switch classifyRegistrationAbsence(infos, intendedPath, target.FeatureRef) {
	case regPresent:
		w := blockedWorkspace(target, intendedPath)
		return &w, nil
	case regUnresolved:
		// Absence is unprovable: fail closed as blocked, byte-untouched — never
		// license a create on an unknown probe.
		w := blockedWorkspace(target, intendedPath)
		return &w, nil
	}
	return nil, nil
```

Keep the remote `ProbeRemoteBranch` leg and the `pathPresent` leg exactly where they are. Update the `inventoryForFresh` doc comment: the registration precondition now also blocks on a registration *on the feature ref elsewhere*, an exact stale registration, and an unresolvable registration identity (previously skip-matched via `registeredAt`). If `registeredAt` loses its last caller, delete it.

- [ ] **Step 5: Run the workspace package**

Run: `go test ./internal/workspace/ -count=1`
Expected: PASS (existing prepare tests register real worktrees, which canonicalize; behavior for them is unchanged).

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/prepare.go internal/workspace/prepare_test.go
git commit -m "feat(workspace): fail-closed shared absence probes for fresh allocation (change 0368)"
```

---

### Task 2: `StateAbsent` in `Service.Inspect`

**Files:**
- Modify: `internal/workspace/inspect.go`
- Test: `internal/workspace/inspect_test.go`

**Interfaces:**
- Produces (used by Tasks 3–6): `StateAbsent StateKind = "absent"`; `Inspect` returns `Inspection{Kind: StateAbsent, Path: intendedPath}` only when manifest, local feature ref, target path, and registration are ALL cleanly absent.
- Consumes: Task 1's `localRefAbsent`, `classifyRegistrationAbsence`; existing `pathPresent`, `mapGitFailure`, harness helpers `mainModeRepo`, `freshTarget`, `inspectOK`, `wsPathOf`.

Classification contract for the `manifestAbsent` arm (replaces the unconditional `StateForeign` return):
- Local feature ref present → `StateForeign`, `Detail: "no workspace manifest; local feature branch exists"`.
- Anything at the intended path (file, dir, or dangling symlink — `pathPresent` uses `Lstat`) → `StateForeign`, `Detail: "no workspace manifest; target path is occupied"`.
- Registration `regPresent` → `StateForeign`, `Detail: "no workspace manifest; a worktree registration occupies the target path or feature ref"`.
- Registration `regUnresolved` → `StateForeign`, `Detail: "no workspace manifest; a worktree registration's identity is unresolved"` (unresolved identity *prevents* the absence result; it is legible data, not an error).
- Genuine probe errors (a `ResolveRef` failure other than ref-unavailable, `ListWorktrees` error, `Lstat` error) → typed `*Failure{Op: inspectOp, Stage: "inventory"}` exactly like Inspect's other probes: an unknown probe is never a clean answer, in either direction.
- All cleanly absent → `StateAbsent`.

- [ ] **Step 1: Write the failing tests**

In `internal/workspace/inspect_test.go`, replace `TestInspectForeignAbsent` (it pins the conflation this change removes — the spec calls that out) with a positive absent test plus foreign-leftover coverage. Follow the file's existing fixture idioms (`mainModeRepo`, `freshTarget(t, 7)`, `inspectOK`, `wsPathOf`, `gitOut`):

```go
// TestInspectAbsent is the regression for change 0368: with no manifest, no
// local feature branch, nothing at the intended path, and no registration,
// Inspect reports the proven-absent state instead of foreign. This test MUST
// fail against the pre-0368 conflation (which returned StateForeign here).
func TestInspectAbsent(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)

	insp := inspectOK(t, svc, repo, tgt)
	if insp.Kind != StateAbsent {
		t.Errorf("Kind = %q; want absent for an all-clean missing workspace", insp.Kind)
	}
}

func TestInspectAbsentBlockedByLeftovers(t *testing.T) {
	t.Run("local-branch", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		gitOut(t, r.Primary, "branch", strings.TrimPrefix(string(tgt.FeatureRef), "refs/heads/"), "main")
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign when the local feature branch exists", insp.Kind)
		}
	})
	t.Run("occupied-path-dir", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		if err := os.MkdirAll(wsPathOf(repo), 0o755); err != nil {
			t.Fatal(err)
		}
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign when the target path is occupied", insp.Kind)
		}
	})
	t.Run("dangling-symlink", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		if err := os.MkdirAll(filepath.Dir(wsPathOf(repo)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), wsPathOf(repo)); err != nil {
			t.Fatal(err)
		}
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign for a dangling symlink at the target path", insp.Kind)
		}
	})
	t.Run("registration-on-feature-ref-elsewhere", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		elsewhere := filepath.Join(r.Primary, "..", "elsewhere-ws")
		gitOut(t, r.Primary, "worktree", "add", "-b",
			strings.TrimPrefix(string(tgt.FeatureRef), "refs/heads/"), elsewhere, "main")
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign when a registration references the feature ref", insp.Kind)
		}
	})
	t.Run("stale-registration-at-path", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		gitOut(t, r.Primary, "worktree", "add", "-b", "throwaway/stale-reg", wsPathOf(repo), "main")
		if err := os.RemoveAll(wsPathOf(repo)); err != nil { // directory gone, registration remains
			t.Fatal(err)
		}
		insp := inspectOK(t, svc, repo, tgt)
		if insp.Kind != StateForeign {
			t.Errorf("Kind = %q; want foreign for a stale registration at the target path", insp.Kind)
		}
	})
}
```

Adjust helper spellings to what `harness_test.go` actually provides (e.g. how `wsPathOf` derives the intended path, and whether `freshTarget`'s ref is `refs/heads/…`-qualified) — verify against the file, don't trust this sketch (learning `verify-the-claim`).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/workspace/ -run 'TestInspectAbsent' -count=1`
Expected: FAIL — `undefined: StateAbsent`.

- [ ] **Step 3: Implement**

In `inspect.go`: add the constant with the others:

```go
	StateAbsent StateKind = "absent" // proven cleanly absent: no manifest, local ref, path, or registration
```

Replace the `case manifestAbsent:` arm of `Inspect` with a call to a new method:

```go
	case manifestAbsent:
		return s.classifyAbsentSlot(ctx, repo, target, intendedPath)
```

```go
// classifyAbsentSlot classifies a workspace whose manifest slot is cleanly
// absent. It is StateAbsent — a current, local observation, never proof a
// worker stopped and never cleanup or launch authority — only when the local
// feature ref, the intended path, and every worktree registration are ALSO
// cleanly absent. Any leftover, and any registration whose identity cannot be
// resolved, keeps the pre-0368 StateForeign classification with a bounded
// detail; a genuine probe error is an error, never read as absence in either
// direction.
func (s *Service) classifyAbsentSlot(ctx context.Context, repo gitcli.Repository, target Target, intendedPath string) (Inspection, error) {
	foreign := func(detail string) (Inspection, error) {
		return Inspection{Kind: StateForeign, Path: intendedPath, Detail: detail}, nil
	}
	absent, err := s.localRefAbsent(ctx, repo, target.FeatureRef)
	if err != nil {
		return Inspection{}, mapGitFailure(inspectOp, "inventory", err)
	}
	if !absent {
		return foreign("no workspace manifest; local feature branch exists")
	}
	if present, perr := pathPresent(intendedPath); perr != nil {
		return Inspection{}, &Failure{Op: inspectOp, Stage: "inventory", Kind: KindExternal, Detail: "stat of target path failed", Err: perr}
	} else if present {
		return foreign("no workspace manifest; target path is occupied")
	}
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return Inspection{}, mapGitFailure(inspectOp, "inventory", err)
	}
	switch classifyRegistrationAbsence(infos, intendedPath, target.FeatureRef) {
	case regPresent:
		return foreign("no workspace manifest; a worktree registration occupies the target path or feature ref")
	case regUnresolved:
		return foreign("no workspace manifest; a worktree registration's identity is unresolved")
	}
	return Inspection{Kind: StateAbsent, Path: intendedPath}, nil
}
```

Update the `StateForeign` comment (`// absent, foreign, malformed, or unowned manifest`) to `// leftover, foreign, malformed, or unowned manifest` and the file-header sentence that says a missing manifest is StateForeign data, so the prose matches the code.

- [ ] **Step 4: Run the package**

Run: `go test ./internal/workspace/ -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-probe the conjuncts (throwaway edits, do not commit them)**

For each mutation, run `go test ./internal/workspace/ -run 'TestInspectAbsent|TestClassifyRegistrationAbsence' -count=1` and confirm a FAIL, then restore (keep a copy of your edit aside first — `git checkout --` restores to HEAD, learning `mutation-restore-needs-a-backup-copy`):
1. Make `classifyAbsentSlot` skip the local-ref check → `local-branch` subtest reddens.
2. Skip the path check → `occupied-path-dir` and `dangling-symlink` redden.
3. Treat `regUnresolved` as absent → `TestClassifyRegistrationAbsence` "unresolved" case plus `stale-registration-at-path` (if its fixture path resolves lexically) redden.
4. Return `StateForeign` unconditionally (the pre-change behavior) → `TestInspectAbsent` reddens — proving the positive regression fails under the original conflation, as the spec requires.

Record the four reddening test names for the results file later.

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/inspect.go internal/workspace/inspect_test.go
git commit -m "feat(workspace): StateAbsent — prove a pre-allocation workspace cleanly absent (change 0368)"
```

---

### Task 3: Closed resume admission + remote-absence proof in `change resume-halted`

**Files:**
- Modify: `internal/app/change_halt.go`
- Test: `internal/app/change_halt_test.go` (fake-service admission matrix), `internal/app/change_integration_test.go` (extend `TestIntegrationChangeResumeHalted`)

**Interfaces:**
- Consumes: `workspace.StateAbsent` (Task 2), existing `deps.Client.ProbeRemoteBranch(ctx, repo, originRemote, gitcli.RefName(insp.FeatureRef))` (`insp.FeatureRef` is the fully qualified resolved recorded ref that `WorkspaceInspect` returns — the same value `run_verify.go`'s remote probe uses; never re-mint the branch spelling), `gitcli.RemoteRefFound` / `RemoteRefAbsent`.
- Produces: two new stable reasons and a closed classifier:

```go
	// ReasonResumeRemoteBranchPresent: the workspace is proven locally absent
	// but the recorded remote feature branch still exists — existing work.
	ReasonResumeRemoteBranchPresent = "remote-branch-present"
	// ReasonResumeUnknownState: the reprobe returned a state outside the closed
	// admission set; unknown never authorizes resume.
	ReasonResumeUnknownState = "workspace-state-unknown"
```

```go
type resumeAdmission int

const (
	resumeRefused resumeAdmission = iota
	resumeAdmitted
	resumeNeedsRemoteAbsence // StateAbsent: admit only after the remote ref proves cleanly absent
)

func classifyResumeAdmission(state string) (resumeAdmission, reason, message string)
```

- [ ] **Step 1: Write the failing fake-service tests**

`fakeResumeWorkspace{kind, head}` in `change_halt_test.go` already drives `WorkspaceInspect` through a fake service; `setupHaltedFixture` builds a real repo with a real origin, so `ProbeRemoteBranch` runs against real git. The fixture's recorded branch is `feat/widget` (asserted byte-preserved in the existing quiescent test) and no such remote branch exists in the fixture — clean absence — unless a subtest pushes one. Add to `TestIntegrationChangeResumeHalted` in `change_integration_test.go`, alongside the existing subtests:

```go
			// A proven-absent workspace with a cleanly absent remote branch resumes.
			t.Run("absent-workspace-resumes", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateAbsent, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
				if got.Result != ResultApplied || got.Disposition != HaltDispResumed {
					t.Fatalf("result=%q disp=%q reason=%q", got.Result, got.Disposition, got.Reason)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if strings.Contains(rec, "## Run halted") {
					t.Errorf("marker not removed on absent-workspace resume:\n%s", rec)
				}
				if !strings.Contains(rec, "branch: feat/widget") {
					t.Errorf("recorded branch not preserved:\n%s", rec)
				}
			})

			// The same absent workspace with the recorded branch on the remote is
			// existing work: refused, marker retained.
			t.Run("absent-workspace-remote-branch-blocks", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				// Push the recorded feature branch to origin from the primary tree.
				gitRun(t, f.repo.primary, "branch", "feat/widget", "HEAD")
				gitRun(t, f.repo.primary, "push", "origin", "feat/widget")
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateAbsent, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
				if got.Result != ResultBlocked || got.Reason != ReasonResumeRemoteBranchPresent {
					t.Fatalf("result=%q reason=%q, want blocked/%s", got.Result, got.Reason, ReasonResumeRemoteBranchPresent)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if !strings.Contains(rec, "## Run halted") {
					t.Errorf("marker removed on a refused resume:\n%s", rec)
				}
			})

			// An unrecognized state string never authorizes resume.
			t.Run("unknown-state-refuses", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateKind("weird-new-state"), head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
				if got.Result != ResultBlocked || got.Reason != ReasonResumeUnknownState {
					t.Fatalf("result=%q reason=%q, want blocked/%s", got.Result, got.Reason, ReasonResumeUnknownState)
				}
			})
```

Match the fixture's actual helper names (`gitRun`/`writerAdvance`/whatever `change_integration_test.go` uses to run git in the primary and push — read `setupHaltedFixture` and its `rebaseFixture` fields first and adapt; the recorded-branch spelling `feat/widget` and how origin is wired must be taken from the fixture, not assumed). If the fake service needs its `Inspect` to carry `FeatureRef` through, note that `WorkspaceInspect` fills `FeatureRef` from the *resolved target*, not from the service result, so nothing extra is needed on the fake.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run TestIntegrationChangeResumeHalted -count=1`
Expected: FAIL — `undefined: workspace.StateAbsent` consumers aside, `ReasonResumeRemoteBranchPresent` undefined; after constants exist, `absent-workspace-resumes` fails with reason `workspace-writer-active`… actually pre-change `StateAbsent` falls into the *default admit* arm — which is exactly the open-default defect: the `unknown-state-refuses` subtest is the one that reddens against current code. Confirm both new refusal subtests fail for the right reason before implementing.

- [ ] **Step 3: Implement the closed admission**

Replace `resumeQuiescenceRefusal` with:

```go
// classifyResumeAdmission maps a reprobed workspace state onto the closed
// resume admission set. Ready, dirty-owned (uncommitted checkpoints),
// branch-missing, and cleaned are quiescent and resume as before. A proven
// locally-absent workspace (change 0368: a run halted before allocation)
// resumes only after the recorded remote feature ref also proves cleanly
// absent — the caller performs that probe. Allocating, foreign, and
// mismatched states are refused as possibly-live writers, and any state
// outside this closed set is refused: unknown never authorizes resume.
func classifyResumeAdmission(state string) (resumeAdmission, string, string) {
	switch state {
	case string(workspace.StateReady), string(workspace.StateDirty),
		string(workspace.StateBranchGone), string(workspace.StateCleaned):
		return resumeAdmitted, "", ""
	case string(workspace.StateAbsent):
		return resumeNeedsRemoteAbsence, "", ""
	case string(workspace.StateResumable):
		return resumeRefused, ReasonResumeWorkspaceActive,
			"the owned workspace is mid-allocation; its writer may still be live, so resume adopts nothing"
	case string(workspace.StateForeign), string(workspace.StateMismatch):
		return resumeRefused, ReasonResumeWorkspaceActive,
			"the owned workspace has foreign or mismatched ownership; resume never adopts a workspace whose writer may be live"
	default:
		return resumeRefused, ReasonResumeUnknownState,
			fmt.Sprintf("workspace reprobe returned unrecognized state %q; resume admits only known quiescent states", state)
	}
}
```

In `ChangeResumeHalted`, move the `deps.Client.Discover` call *above* the admission check (the remote probe needs `repo`; Discover is read-only, so reordering is safe), then replace the `resumeQuiescenceRefusal` block:

```go
	admission, reason, msg := classifyResumeAdmission(insp.State)
	switch admission {
	case resumeRefused:
		return haltRefusal(OperationChangeResumeHalted, ResultBlocked, reason, msg, req.ID)
	case resumeNeedsRemoteAbsence:
		// The workspace is proven locally absent (halted before allocation).
		// Recovery additionally requires the resolved recorded remote feature
		// ref to be cleanly absent: a remote branch is existing work; an
		// errored or unrecognized probe is unknown and blocks (never absence).
		rref, perr := deps.Client.ProbeRemoteBranch(ctx, repo, originRemote, gitcli.RefName(insp.FeatureRef))
		if perr != nil {
			return haltRefusal(OperationChangeResumeHalted, ResultBlocked, ReasonResumeWorkspaceProbe,
				fmt.Sprintf("could not probe remote feature branch %q; refusing to resume on an unknown probe", insp.FeatureRef), req.ID)
		}
		switch rref.State {
		case gitcli.RemoteRefAbsent:
			// cleanly absent: proceed to the ordinary resume transaction.
		case gitcli.RemoteRefFound:
			return haltRefusal(OperationChangeResumeHalted, ResultBlocked, ReasonResumeRemoteBranchPresent,
				fmt.Sprintf("remote feature branch %q still exists; the halted run left work behind — resume will not proceed as if none exists", insp.FeatureRef), req.ID)
		default:
			return haltRefusal(OperationChangeResumeHalted, ResultBlocked, ReasonResumeWorkspaceProbe,
				fmt.Sprintf("remote probe of %q returned an unrecognized state; refusing to resume on an unknown probe", insp.FeatureRef), req.ID)
		}
	}
```

Check `gitcli.RemoteRef`'s actual state vocabulary in `internal/gitcli/remoteref.go` before writing the switch (there may be a third `RemoteRefUnknown`-like value — handle every named value explicitly and default-refuse). `resumeHaltedOp` and everything after it stay byte-identical: no allocation, no epoch, no cancellation — ADR-0118 machinery untouched. Update the file-header comment paragraph about resume to mention the pre-allocation recovery arm.

- [ ] **Step 4: Run**

Run: `go test ./internal/app/ -run 'TestIntegrationChangeResumeHalted|TestIntegrationChangeHaltResumeCycle' -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-probe the new admission (throwaway, with a backup copy of your edits)**

1. Delete the `ProbeRemoteBranch` call and admit `StateAbsent` unconditionally → `absent-workspace-remote-branch-blocks` reddens.
2. Change the `default:` arm to admit → `unknown-state-refuses` reddens.
Both under `go test ./internal/app/ -run TestIntegrationChangeResumeHalted -count=1`. Record results.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_halt.go internal/app/change_halt_test.go internal/app/change_integration_test.go
git commit -m "feat(app): resume-halted recovers a proven pre-allocation halt behind a remote-absence proof (change 0368)"
```

---

### Task 4: Sweep every workspace-state consumer; preserve pre-change behavior for `StateAbsent`

**Files:**
- Modify: `internal/app/maintenance_assess.go`, `internal/app/change_repair.go` (plus anything the sweep finds)
- Test: nearest existing test file for each touched consumer

Do not trust this task's enumeration as closed (spec: search at implementation time; learning `verify-the-claim`). Re-derive the consumer set:

- [ ] **Step 1: Enumerate consumers**

```bash
grep -rn "StateForeign\|StateMismatch\|StateResumable\|StateCleaned\|StateBranchGone\|StateDirty\|StateReady\|StateAbsent\|insp.State\|\.Kind ==" --include="*.go" internal/ cmd/ | grep -v _test.go
```

At plan time the non-test consumers were: `change_halt.go` (Task 3), `change_reclaim.go` (`reclaimActiveWorkspaceStates`), `maintenance_assess.go` (`sweepAssessWorkspaceLeg`), `change_repair.go` (`repairWorkspaceClear`-area `insp.Kind == workspace.StateForeign`), `evidence_recertify.go` (switch on Ready/Dirty), `finalize_rebase.go` (switch on Ready/Dirty). Sort your grep output into these and any new ones; every switch/map keyed on states must be checked for what its default does to `"absent"`.

- [ ] **Step 2: `maintenance_assess.go` — absence never certifies clean**

Pre-change, an absent manifest was `StateForeign` → `markBlocked`. Preserve that:

```go
	case workspace.StateForeign, workspace.StateAbsent:
		// Absent, foreign, malformed, or unowned: never certifies a clean
		// checkout — StateAbsent is a local observation, not a cleaned
		// tombstone and not authority to delete (change 0368).
		a.markBlocked(sweepLegWorkspace, "a missing or foreign workspace manifest does not certify a clean checkout")
```

Find the existing test that pins the `StateForeign` → blocked leg (grep `sweepAssessWorkspaceLeg` / the blocked message in `internal/app/*_test.go`) and add an identical case for `StateAbsent`. Write the test first, watch it fail (absent currently falls to `default:` → `markWork()`), then apply the fix.

- [ ] **Step 3: `change_repair.go` — no owned workspace, no conflict**

Pre-change, absent inspected as `StateForeign` → `return nil` (no conflict). Preserve:

```go
	if insp.Kind == workspace.StateForeign || insp.Kind == workspace.StateAbsent {
		// No owned workspace at the recorded branch: nothing to conflict.
		return nil
	}
```

Add/extend the repair test that exercises the no-workspace path with a fake/real inspection returning `StateAbsent` (find how existing repair tests stub `deps.Workspace.Inspect`).

- [ ] **Step 4: `change_reclaim.go` — no code change, pinned by test**

`reclaimActiveWorkspaceStates` lists active states; `"absent"` is not among them, so an absent workspace does not block reclaim — same as the foreign classification did pre-change — and strict lease expiry plus the branch-absence proofs still gate every reclaim (`reclaimProveBranchesAbsent` is untouched). Add one test beside the existing reclaim workspace-gate coverage (grep `ReasonReclaimWorkspaceActive` in `change_reclaim_test.go`): an inspection reporting `StateAbsent` does not produce the workspace-active refusal (the reclaim proceeds to whatever the fixture's next gate is — assert the reason is *not* `ReasonReclaimWorkspaceActive`). Also extend the `reclaimActiveWorkspaceStates` doc comment to name absent explicitly among the non-blocking states.

- [ ] **Step 5: `evidence_recertify.go` and `finalize_rebase.go` — confirm the default refuses**

Read each switch: both accept only `StateReady`/`StateDirty` and refuse otherwise, so `"absent"` lands in the same refusal arm foreign always hit. No code change; confirm by reading the default arm and note it in the commit message. If any consumer found in Step 1 *enumerates* non-owned states instead of defaulting (learning `duplicated-gate-copies-the-whole-predicate` — a copied threshold diverges exactly on the new input), fix it to include `StateAbsent` with pre-change behavior and pin it with a test.

- [ ] **Step 6: Run and commit**

Run: `go test ./internal/app/ ./internal/workspace/ -count=1`
Expected: PASS.

```bash
git add internal/app/maintenance_assess.go internal/app/change_repair.go internal/app/change_reclaim.go internal/app/*_test.go
git commit -m "fix(app): carry StateAbsent through every workspace-state consumer with pre-change behavior (change 0368)"
```

---

### Task 5: Real claim → halt → resume regression through the actual workspace service

**Files:**
- Test: `internal/app/change_integration_test.go` (new `TestIntegrationResumeHaltedPreallocation`)

**Interfaces:**
- Consumes: `setupHaltedFixture` (real repos, halted in-progress change, real origin), `workspace.NewService(git *gitcli.Client)`, `WorkspaceDeps{Service: …}`, `WorkspacePrepare`, `ChangeResumeHalted`.

This is the spec's verification requirement 1: no fake workspace service anywhere in the pass. The fixture's change was claimed and halted with **no** `workspace.prepare` ever run, so the real service must classify `StateAbsent` and resume must apply; ordinary prepare must then succeed.

- [ ] **Step 1: Write the test**

```go
// TestIntegrationResumeHaltedPreallocation is change 0368's end-to-end
// regression: a real claim, a real halt before any workspace.prepare, then an
// acknowledged exact-version resume through the REAL workspace service — no
// fake inspection. It must fail under the pre-0368 conflation, where the
// absent workspace inspected as foreign and resume refused
// workspace-writer-active.
func TestIntegrationResumeHaltedPreallocation(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			f := setupHaltedFixture(t, m)
			svc, err := workspace.NewService(f.deps.Client) // adapt: use the same *gitcli.Client the fixture wires
			if err != nil {
				t.Fatal(err)
			}
			wdeps := WorkspaceDeps{Service: svc}

			got := ChangeResumeHalted(context.Background(), f.deps, wdeps, f.repo.invocation,
				ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
			if got.Result != ResultApplied || got.Disposition != HaltDispResumed {
				t.Fatalf("resume through the real service: result=%q disp=%q reason=%q", got.Result, got.Disposition, got.Reason)
			}

			// Marker removed, claim refreshed, status and recorded branch preserved.
			rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
			for _, want := range []string{"status: 'in-progress'", "branch: feat/widget", "claimed_at:"} {
				if !strings.Contains(rec, want) {
					t.Errorf("missing %q after resume:\n%s", want, rec)
				}
			}
			if strings.Contains(rec, "## Run halted") {
				t.Errorf("halt marker survived resume:\n%s", rec)
			}

			// Nothing was allocated by the resume itself: no feature branch, no
			// worktree path, no manifest — the workspace still inspects absent.
			insp := WorkspaceInspect(context.Background(), f.deps, wdeps, f.repo.invocation, WorkspaceIDRequest{ID: f.id})
			if insp.Result != ResultApplied || insp.State != string(workspace.StateAbsent) {
				t.Fatalf("post-resume inspect: result=%q state=%q, want applied/absent", insp.Result, insp.State)
			}

			// A subsequent ordinary prepare succeeds (the resume left allocation
			// to its normal later step). Re-read the fresh record version first.
			version := blobVersionAt(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
			prep := WorkspacePrepare(context.Background(), f.deps, wdeps, f.repo.invocation,
				WorkspaceIDRequest{ID: f.id, Version: version})
			if prep.Result != ResultApplied {
				t.Fatalf("post-resume prepare: result=%q reason=%q message=%q", prep.Result, prep.Reason, prep.Message)
			}
		})
	}
}
```

Adapt the details to reality before running (learning `verify-the-claim`): how the fixture exposes its `*gitcli.Client` (it may live on `f.deps.Client` as an interface — `workspace.NewService` wants the concrete `*gitcli.Client`; construct one the same way the fixture does if needed), whether `WorkspaceIDRequest` carries `Version` for prepare, whether the fixture's recorded branch/status spellings match, and whether the fixture repo's mode set (`planRepoModes`) needs a remote for `Prepare`'s base fetch (`setupHaltedFixture` has an origin; `Prepare` fetches the base branch from `origin`, which must exist — if a mode lacks it, scope the prepare leg to the modes where it can pass, with a comment).

- [ ] **Step 2: Run — confirm it exercises the new path**

Run: `go test ./internal/app/ -run TestIntegrationResumeHaltedPreallocation -count=1 -v`
Expected: PASS. Then prove non-vacuity: temporarily revert `classifyAbsentSlot` to return the foreign inspection unconditionally (backup copy first) and re-run — expected FAIL with reason `workspace-writer-active`. Restore.

- [ ] **Step 3: Commit**

```bash
git add internal/app/change_integration_test.go
git commit -m "test(app): end-to-end pre-allocation halt/resume regression through the real workspace service (change 0368)"
```

---

### Task 6: Refusal-shape and no-write coverage retained; prepare-side behavior notes

**Files:**
- Test: `internal/app/change_integration_test.go` or `change_halt_test.go`, `internal/workspace/prepare_test.go`

- [ ] **Step 1: Confirm the retained refusal matrix still holds**

The existing subtests already cover missing acknowledgement, version drift, live-writer, and no-marker refusals — re-read them and confirm none was weakened by Task 3's reordering of `Discover` (in particular: the not-acknowledged refusal must still fire *before* any pin, and a refused resume must still leave the marker; both are already asserted). If Task 3 moved `Discover` above the workspace reprobe, verify no test pinned the old error-ordering (run the whole `internal/app` package).

- [ ] **Step 2: Pin the prepare-side tightening**

Task 1 changed `inventoryForFresh`: a stale registration at the target path, a registration on the feature ref elsewhere, and an unresolvable registration now block a fresh allocation (previously skip-matched). Add one prepare test for the newly-blocking case that matters most:

```go
func TestPrepareFreshBlockedByStaleRegistration(t *testing.T) {
	// worktree add at the intended path on a throwaway branch, rm -rf the
	// directory (registration survives), then Prepare: disposition blocked,
	// byte-untouched — never a silent adoption or force-remove.
}
```

Follow the existing blocked-disposition prepare tests in `prepare_test.go` for fixture idioms; assert `Disposition == PrepareBlocked` and that no manifest was written.

- [ ] **Step 3: Run both packages and commit**

Run: `go test ./internal/workspace/ ./internal/app/ -count=1`
Expected: PASS.

```bash
git add internal/workspace/prepare_test.go internal/app
git commit -m "test: pin fresh-allocation fail-closed registration proof and retained resume refusals (change 0368)"
```

---

### Task 7: Documentation/schema sweep, full-suite gate

**Files:**
- Modify: whatever the sweep finds (expected: none-to-few maintained surfaces)

- [ ] **Step 1: Sweep maintained surfaces for the state vocabulary**

```bash
grep -rn --include='*.md' -e 'dirty-owned' -e 'branch-missing' -e 'workspace state' -e 'foreign' docs/ README.md | grep -v 'docs/changes/archive\|docs/results\|docs/superpowers/plans\|docs/superpowers/specs\|docs/adrs'
```

Point-in-time records (archives, results, old plans/specs, Accepted ADRs) keep their wording — do not rewrite history. Any *maintained* doc enumerating workspace states (command reference, convention doc, schema doc) gains `absent` with one sentence: proven cleanly absent pre-allocation; recoverable by acknowledged resume after a remote-absence proof. Also check `internal/app/schema_registry.go` / result schema surfaces: `WorkspaceOpResult.State` is an untagged string (no enum registry entry to extend at plan time), but confirm no `docket:"enum=…"` vocabulary or JSON-schema fixture pins the state set — if one does, extend it and its test.

- [ ] **Step 2: gofmt and vet the branch**

Run: `gofmt -l internal/ && go vet ./...`
Expected: no output from gofmt; vet clean.

- [ ] **Step 3: Full suite through the configured build gate**

Read the command from config (never a second copy):
`docket config resolve --json` (or the capability-catalog operation the build skill uses) → `build.test_command`, and run it from this feature checkout. Expected: SUITE green. Read the budget report: any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding to note; a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on (serial confirm per `tests/README.md`). The new integration test adds real-git work to `internal/app` — if its wall-clock lands near a budget row, note it for the results file rather than trimming assertions.

- [ ] **Step 4: Commit any sweep edits**

```bash
git add -u
git commit -m "docs: name the absent workspace state on maintained surfaces (change 0368)"
```

(Skip the commit if the sweep found nothing to edit.)

---

## Self-Review

- Spec coverage: absence distinction in inspection (Task 2), shared probes factored from `inventoryForFresh` with fail-closed stale-registration handling (Task 1), resume extension with remote-absence proof and closed admission (Task 3), consumers kept coherent — reclaim/maintenance/repair/recertify/rebase (Task 4), verification requirements 1–5 (Tasks 5, 6, 3, 2, 4), mutation evidence with uncached runs (Tasks 2/3/5), full-suite gate via configured command (Task 7). Alternatives section requires no work; scope exclusions are honored (no TTL, flag, allocation, migration, or gate changes anywhere).
- Type consistency: `StateAbsent` (Task 2) is what Tasks 3–6 consume; `classifyRegistrationAbsence`/`localRefAbsent` (Task 1) are what Task 2 calls; `classifyResumeAdmission` + `ReasonResumeRemoteBranchPresent`/`ReasonResumeUnknownState` are defined once in Task 3 and referenced nowhere else.
- Known adaptation points are called out explicitly (fixture helper spellings, `RemoteRef` vocabulary, `planRepoModes` remote availability) rather than asserted — the implementer must verify each against the file before coding (learning `verify-the-claim`).
